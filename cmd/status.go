package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:   "status <session-name or id>",
	Short: "Check session status",
	Args:  cobra.MinimumNArgs(1),
	Run:   runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output JSON")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) {
	nameOrID := strings.Join(args, "-")

	sessionID, err := cl.ResolveSessionID(nameOrID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	session, err := cl.GetSessionStatus(sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if statusJSON {
		printJSON(session)
		return
	}

	fmt.Printf("Session: %v\n", session["id"])
	if title, ok := session["task_title"]; ok {
		fmt.Printf("Task:    %v\n", title)
	}
	fmt.Printf("Status:  %v\n", session["status"])
	if v := session["started_at"]; v != nil {
		fmt.Printf("Started: %v\n", v)
	}
	if v := session["finished_at"]; v != nil {
		fmt.Printf("Finished: %v\n", v)
	}
}
