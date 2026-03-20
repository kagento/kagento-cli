package cmd

import (
	"encoding/json"
	"fmt"
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
	Short: "Log in to Kagento via browser (OAuth 2.0 Device Flow)",
	Args:  cobra.NoArgs,
	Run:   runLogin,
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

type deviceAuthResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func runLogin(cmd *cobra.Command, args []string) {
	serverURL := envOr("KAGENTO_URL", "https://kagento.io")
	clientID := "contest-web"
	keycloakURL := serverURL
	deviceAuthURL := keycloakURL + "/realms/contest/protocol/openid-connect/auth/device"
	tokenURL := keycloakURL + "/realms/contest/protocol/openid-connect/token"

	// Step 1: Request device authorization.
	resp, err := http.PostForm(deviceAuthURL, url.Values{
		"client_id": {clientID},
		"scope":     {"openid"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to contact auth server: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: device auth request failed with HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var deviceResp deviceAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse device auth response: %v\n", err)
		os.Exit(1)
	}

	// Step 2: Show instructions to user.
	fmt.Printf("\nOpen %s and enter code: %s\n\n", deviceResp.VerificationURI, deviceResp.UserCode)

	// Step 3: Try to open browser automatically.
	openBrowser(deviceResp.VerificationURI)

	fmt.Println("Waiting for login...")

	// Step 4: Poll token endpoint.
	interval := deviceResp.Interval
	if interval < 1 {
		interval = 5
	}

	deadline := time.Now().Add(time.Duration(deviceResp.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(interval) * time.Second)

		tokenResp, err := pollToken(tokenURL, clientID, deviceResp.DeviceCode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		switch tokenResp.Error {
		case "":
			// Success — save credentials.
			creds := &auth.Credentials{
				AccessToken:  tokenResp.AccessToken,
				RefreshToken: tokenResp.RefreshToken,
				ExpiresAt:    time.Now().Unix() + tokenResp.ExpiresIn,
				ServerURL:    serverURL,
			}
			if err := auth.Save(creds); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to save credentials: %v\n", err)
				os.Exit(1)
			}

			// Print who we logged in as.
			claims, err := auth.ParseJWTClaims(tokenResp.AccessToken)
			if err == nil {
				name := claimString(claims, "preferred_username")
				if name == "" {
					name = claimString(claims, "email")
				}
				if name != "" {
					fmt.Printf("Logged in as %s\n", name)
					return
				}
			}
			fmt.Println("Logged in successfully!")
			return

		case "authorization_pending":
			// Keep polling.
			continue

		case "slow_down":
			interval += 5
			continue

		case "expired_token":
			fmt.Fprintln(os.Stderr, "Error: login timed out. Please try again.")
			os.Exit(1)

		default:
			fmt.Fprintf(os.Stderr, "Error: %s — %s\n", tokenResp.Error, tokenResp.ErrorDesc)
			os.Exit(1)
		}
	}

	fmt.Fprintln(os.Stderr, "Error: login timed out. Please try again.")
	os.Exit(1)
}

func pollToken(tokenURL, clientID, deviceCode string) (*tokenResponse, error) {
	resp, err := http.PostForm(tokenURL, url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {deviceCode},
		"client_id":   {clientID},
	})
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	return &tokenResp, nil
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
