package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var submitCmd = &cobra.Command{
	Use:   "submit <session_id>",
	Short: "Final submit (stops session, records score)",
	Args:  cobra.ExactArgs(1),
	Run:   runSubmit,
}

func init() {
	rootCmd.AddCommand(submitCmd)
}

func runSubmit(cmd *cobra.Command, args []string) {
	sessionID := args[0]
	fmt.Printf("Finishing session %s...\n", sessionID)

	// Set session status to "finishing"
	updateMutation := `
		mutation($id: uuid!) {
			update_sessions(where: {id: {_eq: $id}, status: {_in: ["ready", "running"]}}, _set: {status: "finishing"}) {
				affected_rows
				returning {
					id
					status
				}
			}
		}`

	data, err := cl.Query(updateMutation, map[string]interface{}{
		"id": sessionID,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating session: %v\n", err)
		os.Exit(1)
	}

	result, ok := data["update_sessions"].(map[string]interface{})
	if ok && result != nil {
		if affected, ok := result["affected_rows"].(float64); ok && affected > 0 {
			goto poll
		}
	}

	// Poll until status == "completed"
poll:
	pollQuery := `
		query($id: uuid!) {
			sessions_by_pk(id: $id) {
				id
				status
			}
		}`

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Fprintln(os.Stderr, "Error: timed out waiting for session to complete")
			os.Exit(1)
		case <-ticker.C:
			data, err := cl.Query(pollQuery, map[string]interface{}{
				"id": sessionID,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error polling session: %v\n", err)
				os.Exit(1)
			}

			result, ok := data["sessions_by_pk"].(map[string]interface{})
			if !ok {
				continue
			}

			status, _ := result["status"].(string)
			if status != "completed" {
				fmt.Print(".")
				continue
			}

			fmt.Println()

			// Fetch submission details
			subQuery := `
				query($session_id: uuid!) {
					submissions(
						where: {session_id: {_eq: $session_id}}
						limit: 1
						order_by: {created_at: desc}
					) {
						id
						score
						duration_sec
						details
						created_at
					}
				}`

			subData, err := cl.Query(subQuery, map[string]interface{}{
				"session_id": sessionID,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error fetching submission: %v\n", err)
				os.Exit(1)
			}

			submissions, ok := subData["submissions"].([]interface{})
			if !ok || len(submissions) == 0 {
				fmt.Println("\nSession completed!")
				fmt.Println("No submission record found.")
				return
			}

			sub := submissions[0].(map[string]interface{})
			fmt.Println("\nSession complete!")
			fmt.Printf("Score: %v/100\n", sub["score"])

			if dur, ok := sub["duration_sec"].(float64); ok {
				minutes := int(dur) / 60
				seconds := int(dur) % 60
				fmt.Printf("Duration: %dm %ds\n", minutes, seconds)
			}

			if id, ok := sub["id"]; ok {
				fmt.Printf("Submission: %v\n", id)
			}

			if details, ok := sub["details"]; ok && details != nil {
				detailsJSON, _ := json.MarshalIndent(details, "", "  ")
				fmt.Println(string(detailsJSON))
			}
			return
		}
	}
}
