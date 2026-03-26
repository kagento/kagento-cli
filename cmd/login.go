package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/kagento/kagento-cli/internal/auth"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Kagento via browser (OAuth)",
	Args:  cobra.NoArgs,
	Run:   runLogin,
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) {
	supabaseURL := envOr("SUPABASE_URL", "https://bkhmyatdwfaydhcmgkte.supabase.co")

	// Find a free port for the callback server.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not start local server: %v\n", err)
		os.Exit(1)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// Build the Supabase OAuth URL — use GitHub as provider.
	// Supabase manages its own state/PKCE internally.
	authURL := fmt.Sprintf(
		"%s/auth/v1/authorize?provider=github&redirect_to=%s",
		supabaseURL,
		url.QueryEscape(redirectURI),
	)

	fmt.Printf("\nOpening browser to log in...\n")
	fmt.Printf("If the browser doesn't open, visit:\n  %s\n\n", authURL)

	openBrowser(authURL)

	// Start local server to capture the callback.
	resultCh := make(chan *callbackResult, 1)

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/callback" {
				http.NotFound(w, r)
				return
			}

			// Supabase sends tokens as hash fragment, but with PKCE flow
			// it can also send an auth code as query param.
			code := r.URL.Query().Get("code")
			errMsg := r.URL.Query().Get("error")
			errDesc := r.URL.Query().Get("error_description")

			if errMsg != "" {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, "<html><body><h2>Login failed</h2><p>%s: %s</p><p>You can close this tab.</p></body></html>", errMsg, errDesc)
				resultCh <- &callbackResult{err: fmt.Errorf("%s: %s", errMsg, errDesc)}
				return
			}

			if code == "" {
				// Supabase might send tokens in the hash fragment.
				// Serve a page that extracts them and sends to our server.
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, `<html><body><script>
const hash = window.location.hash.substring(1);
const params = new URLSearchParams(hash);
const access_token = params.get('access_token');
const refresh_token = params.get('refresh_token');
const expires_in = params.get('expires_in');
if (access_token) {
	fetch('/callback/token?' + new URLSearchParams({access_token, refresh_token: refresh_token || '', expires_in: expires_in || '3600'}))
	.then(() => { document.body.innerHTML = '<h2>Logged in!</h2><p>You can close this tab.</p>'; });
} else {
	document.body.innerHTML = '<h2>Login failed</h2><p>No tokens received. You can close this tab.</p>';
}
</script><p>Processing login...</p></body></html>`)
				return
			}

			// Exchange code for tokens.
			tokens, err := exchangeCode(supabaseURL, code, redirectURI)
			if err != nil {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, "<html><body><h2>Login failed</h2><p>%v</p><p>You can close this tab.</p></body></html>", err)
				resultCh <- &callbackResult{err: err}
				return
			}

			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body><h2>Logged in!</h2><p>You can close this tab.</p></body></html>")
			resultCh <- &callbackResult{tokens: tokens}
		}),
	}

	// Also handle the token extraction from hash fragment.
	origHandler := srv.Handler
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/callback/token" {
			accessToken := r.URL.Query().Get("access_token")
			refreshToken := r.URL.Query().Get("refresh_token")
			expiresIn := r.URL.Query().Get("expires_in")
			if accessToken != "" {
				var expIn int64 = 3600
				fmt.Sscanf(expiresIn, "%d", &expIn)
				resultCh <- &callbackResult{tokens: &tokenResult{
					AccessToken:  accessToken,
					RefreshToken: refreshToken,
					ExpiresIn:    expIn,
				}}
				w.WriteHeader(http.StatusOK)
				return
			}
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}
		origHandler.ServeHTTP(w, r)
	})

	go func() {
		_ = srv.Serve(listener)
	}()

	fmt.Println("Waiting for login...")

	// Wait for callback or timeout.
	select {
	case result := <-resultCh:
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)

		if result.err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", result.err)
			os.Exit(1)
		}

		creds := &auth.Credentials{
			AccessToken:  result.tokens.AccessToken,
			RefreshToken: result.tokens.RefreshToken,
			ExpiresAt:    time.Now().Unix() + result.tokens.ExpiresIn,
			ServerURL:    supabaseURL,
		}
		if err := auth.Save(creds); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to save credentials: %v\n", err)
			os.Exit(1)
		}

		claims, err := auth.ParseJWTClaims(result.tokens.AccessToken)
		if err == nil {
			if email := claimString(claims, "email"); email != "" {
				fmt.Printf("Logged in as %s\n", email)
				return
			}
		}
		fmt.Println("Logged in successfully!")

	case <-time.After(5 * time.Minute):
		_ = srv.Close()
		fmt.Fprintln(os.Stderr, "Error: login timed out. Please try again.")
		os.Exit(1)
	}
}

type callbackResult struct {
	tokens *tokenResult
	err    error
}

type tokenResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func exchangeCode(supabaseURL, code, redirectURI string) (*tokenResult, error) {
	tokenURL := supabaseURL + "/auth/v1/token?grant_type=pkce"

	form := url.Values{
		"auth_code":     {code},
		"code_verifier": {""},
	}

	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("%s: %s", result.Error, result.ErrorDesc)
	}

	return &tokenResult{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
	}, nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}

func claimString(claims map[string]interface{}, key string) string {
	v, ok := claims[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
