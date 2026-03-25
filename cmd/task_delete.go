package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var taskDeleteCmd = &cobra.Command{
	Use:   "delete <slug>",
	Short: "Archive a task and remove its local Docker images",
	Args:  cobra.ExactArgs(1),
	Run:   runTaskDelete,
}

func init() {
	taskCmd.AddCommand(taskDeleteCmd)
}

func runTaskDelete(cmd *cobra.Command, args []string) {
	slug := args[0]

	// Confirm before deleting.
	fmt.Printf("Delete task '%s'? [y/N] ", slug)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		return
	}

	if err := cl.ArchiveTask(slug); err != nil {
		fmt.Fprintf(os.Stderr, "Error archiving task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Task '%s' archived.\n", slug)

	// Remove local Docker images (ignore errors if not found).
	for _, tag := range []string{slug + ":task", slug + ":test"} {
		_ = exec.Command("docker", "rmi", tag).Run()
	}
	fmt.Println("Local Docker images removed.")
}
