package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check <session_id>",
	Short: "Run tests without stopping session (preview score)",
	Args:  cobra.ExactArgs(1),
	Run:   runCheck,
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) {
	sessionID := args[0]
	fmt.Printf("Running tests for session %s...\n", sessionID)

	// Insert a check
	insertMutation := `
		mutation($session_id: uuid!) {
			insert_checks_one(object: {session_id: $session_id}) {
				id
				status
			}
		}`

	data, err := cl.Query(insertMutation, map[string]interface{}{
		"session_id": sessionID,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error inserting check: %v\n", err)
		os.Exit(1)
	}

	check, ok := data["insert_checks_one"].(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Error: unexpected response format")
		os.Exit(1)
	}
	checkID := check["id"].(string)

	// Poll until status != "pending"
	pollQuery := `
		query($id: uuid!) {
			checks_by_pk(id: $id) {
				id
				status
				score
				details
				completed_at
			}
		}`

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Fprintln(os.Stderr, "Error: timed out waiting for check to complete")
			os.Exit(1)
		case <-ticker.C:
			data, err := cl.Query(pollQuery, map[string]interface{}{
				"id": checkID,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error polling check: %v\n", err)
				os.Exit(1)
			}

			result, ok := data["checks_by_pk"].(map[string]interface{})
			if !ok {
				continue
			}

			status, _ := result["status"].(string)
			if status == "pending" {
				fmt.Print(".")
				continue
			}

			fmt.Println()
			if status == "error" {
				fmt.Fprintln(os.Stderr, "Error: check failed")
				if details, ok := result["details"]; ok {
					detailsJSON, _ := json.MarshalIndent(details, "", "  ")
					fmt.Fprintln(os.Stderr, string(detailsJSON))
				}
				os.Exit(1)
			}

			// Completed
			score := result["score"]
			fmt.Printf("\nScore: %v/100\n", score)
			if details, ok := result["details"]; ok && details != nil {
				detailsJSON, _ := json.MarshalIndent(details, "", "  ")
				fmt.Println(string(detailsJSON))
			}
			fmt.Printf("\nSession still running. Use 'kagento submit %s' to finalize.\n", sessionID)
			return
		}
	}
}
