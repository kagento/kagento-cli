package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <task-slug>",
	Short: "Start a new session for a task",
	Args:  cobra.ExactArgs(1),
	Run:   runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) {
	taskSlug := args[0]

	fmt.Printf("Starting session for task %q...\n", taskSlug)

	result, err := cl.StartSession(taskSlug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	sessionID, _ := result["session_id"].(string)
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "Error: no session_id in response")
		os.Exit(1)
	}

	if existing, _ := result["existing"].(bool); existing {
		status, _ := result["status"].(string)
		name, _ := result["ssh_session_name"].(string)
		fmt.Printf("Reusing existing session: %s\n", sessionID)
		if name != "" {
			fmt.Printf("Session name: %s\n", name)
		}
		if status == "ready" || status == "running" {
			downloadAndPrintKubeconfig(sessionID)
			return
		}
		fmt.Printf("Status: %s\n", status)
		// Fall through to polling if not yet ready.
	} else {
		if title, _ := result["task_title"].(string); title != "" {
			fmt.Printf("Task: %s\n", title)
		}
		fmt.Printf("Session: %s\n", sessionID)
	}

	// Poll until session is ready or running.
	fmt.Print("Waiting for session to be ready")
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Fprintln(os.Stderr, "\nError: timed out waiting for session to be ready")
			os.Exit(1)
		case <-ticker.C:
			session, err := cl.GetSessionStatus(sessionID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nError polling session: %v\n", err)
				os.Exit(1)
			}

			status, _ := session["status"].(string)
			switch status {
			case "ready", "running":
				fmt.Println()
				downloadAndPrintKubeconfig(sessionID)
				return
			case "failed":
				fmt.Fprintln(os.Stderr, "\nError: session failed to start")
				os.Exit(1)
			default:
				fmt.Print(".")
			}
		}
	}
}

func downloadAndPrintKubeconfig(sessionID string) {
	data, err := cl.GetKubeconfig(sessionID)
	if err != nil {
		fmt.Printf("Kubeconfig not yet available: %v\n", err)
		fmt.Printf("You can download it later: kagento kubeconfig %s\n", sessionID)
		return
	}

	kcPath := filepath.Join(".", "kubeconfig.yaml")
	if err := os.WriteFile(kcPath, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing kubeconfig: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Kubeconfig saved to %s\n", kcPath)
	fmt.Printf("Run: export KUBECONFIG=./%s\n", kcPath)
}
