package cmd

import (
	"fmt"
	"os"
	"strings"

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
		serverURL := envOr("KAGENTO_URL", "https://kagento.io")
		var err error
		token, err = auth.GetValidToken(serverURL, "contest-web")
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

	username := claimString(claims, "preferred_username")
	email := claimString(claims, "email")

	if username != "" {
		fmt.Printf("Username: %s\n", username)
	}
	if email != "" {
		fmt.Printf("Email:    %s\n", email)
	}

	// Print realm roles if present.
	if ra, ok := claims["realm_access"]; ok {
		if raMap, ok := ra.(map[string]interface{}); ok {
			if roles, ok := raMap["roles"]; ok {
				if roleList, ok := roles.([]interface{}); ok {
					var names []string
					for _, r := range roleList {
						if s, ok := r.(string); ok {
							names = append(names, s)
						}
					}
					if len(names) > 0 {
						fmt.Printf("Roles:    %s\n", strings.Join(names, ", "))
					}
				}
			}
		}
	}

	if username == "" && email == "" {
		fmt.Println("Logged in (could not parse user details from token)")
	}
}
