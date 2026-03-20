package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <session_id>",
	Short: "Check session status",
	Args:  cobra.ExactArgs(1),
	Run:   runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) {
	sessionID := args[0]

	query := `
		query($id: uuid!) {
			sessions_by_pk(id: $id) {
				id
				status
				started_at
				finished_at
				task {
					title
				}
			}
		}`

	data, err := cl.Query(query, map[string]interface{}{
		"id": sessionID,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	session, ok := data["sessions_by_pk"].(map[string]interface{})
	if !ok || session == nil {
		fmt.Fprintln(os.Stderr, "Session not found")
		os.Exit(1)
	}

	fmt.Printf("Session: %v\n", session["id"])
	if task, ok := session["task"].(map[string]interface{}); ok {
		fmt.Printf("Task:    %v\n", task["title"])
	}
	fmt.Printf("Status:  %v\n", session["status"])
	if v := session["started_at"]; v != nil {
		fmt.Printf("Started: %v\n", v)
	}
	if v := session["finished_at"]; v != nil {
		fmt.Printf("Finished: %v\n", v)
	}
}
