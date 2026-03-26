package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/kagento/kagento-cli/internal/auth"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with the Docker registry",
	Args:  cobra.NoArgs,
	Run:   runAuth,
}

func init() {
	rootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, args []string) {
	fmt.Println("Authenticating with Kagento registry...")

	// Build request
	req, err := http.NewRequest("POST", cl.BackendURL+"/api/registry/token", bytes.NewReader([]byte("{}")))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")

	if cl.AdminSecret != "" {
		req.Header.Set("Authorization", "Bearer "+cl.AdminSecret)
	} else {
		token := cl.Token
		if !cl.StaticToken {
			refreshedToken, err := auth.GetValidToken()
			if err == nil && refreshedToken != "" {
				token = refreshedToken
				cl.Token = refreshedToken
			}
		}
		if token == "" {
			fmt.Fprintln(os.Stderr, "Error: run 'kagento login' first or set KAGENTO_TOKEN / KAGENTO_ADMIN_SECRET")
			os.Exit(1)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get registry token: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: HTTP %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var result struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Registry string `json:"registry"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse response: %v\n", err)
		os.Exit(1)
	}

	// Run docker login with password on stdin
	dockerCmd := exec.Command("docker", "login", result.Registry, "-u", result.Username, "--password-stdin")
	dockerCmd.Stdin = strings.NewReader(result.Password)
	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr

	if err := dockerCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: docker login failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Logged in to %s (token expires in 15 min)\n", result.Registry)
}
