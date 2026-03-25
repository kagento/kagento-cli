package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var taskUnpublishCmd = &cobra.Command{
	Use:   "unpublish <slug>",
	Short: "Set a task's status back to draft",
	Args:  cobra.ExactArgs(1),
	Run:   runTaskUnpublish,
}

func init() {
	taskCmd.AddCommand(taskUnpublishCmd)
}

func runTaskUnpublish(cmd *cobra.Command, args []string) {
	slug := args[0]

	if err := cl.UnpublishTask(slug); err != nil {
		fmt.Fprintf(os.Stderr, "Error unpublishing task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Task '%s' set to draft.\n", slug)
}
