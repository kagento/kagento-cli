package cmd

import (
	"fmt"
	"os"

	"github.com/kagento/kagento-cli/internal/auth"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently logged-in user",
	Args:  cobra.NoArgs,
	Run:   runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

func runWhoami(cmd *cobra.Command, args []string) {
	// Check env override first.
	token := os.Getenv("KAGENTO_TOKEN")

	if token == "" {
		var err error
		token, err = auth.GetValidToken()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Not logged in. Run 'kagento login' first.")
			os.Exit(1)
		}
	}

	claims, err := auth.ParseJWTClaims(token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	email := claimString(claims, "email")

	// Try user_metadata for display name.
	var username string
	if um, ok := claims["user_metadata"].(map[string]interface{}); ok {
		username = claimString(um, "preferred_username")
		if username == "" {
			username = claimString(um, "user_name")
		}
		if username == "" {
			username = claimString(um, "full_name")
		}
	}

	if username != "" {
		fmt.Printf("Username: %s\n", username)
	}
	if email != "" {
		fmt.Printf("Email:    %s\n", email)
	}

	// Print role from app_metadata if present.
	if am, ok := claims["app_metadata"].(map[string]interface{}); ok {
		if role := claimString(am, "app_role"); role != "" {
			fmt.Printf("Role:     %s\n", role)
		}
	}

	if username == "" && email == "" {
		fmt.Println("Logged in (could not parse user details from token)")
	}
}
