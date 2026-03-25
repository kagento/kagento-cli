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

	if err := cl.FinishSession(sessionID); err != nil {
		fmt.Fprintf(os.Stderr, "Error finishing session: %v\n", err)
		os.Exit(1)
	}

	// Poll until status == "completed"
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Fprintln(os.Stderr, "Error: timed out waiting for session to complete")
			os.Exit(1)
		case <-ticker.C:
			session, err := cl.GetSessionStatus(sessionID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error polling session: %v\n", err)
				os.Exit(1)
			}

			status, _ := session["status"].(string)
			if status != "completed" {
				fmt.Print(".")
				continue
			}

			fmt.Println()

			// Fetch submission details
			sub, err := cl.GetSessionSubmission(sessionID)
			if err != nil {
				fmt.Println("\nSession completed!")
				fmt.Println("No submission record found.")
				return
			}

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
