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

	mutation := `
		mutation($slug: String!) {
			update_tasks(
				where: { slug: { _eq: $slug } }
				_set: { status: "draft" }
			) {
				affected_rows
			}
		}`

	data, err := cl.Query(mutation, map[string]interface{}{
		"slug": slug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating task: %v\n", err)
		os.Exit(1)
	}

	result, ok := data["update_tasks"].(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Error: unexpected response format")
		os.Exit(1)
	}

	affected, _ := result["affected_rows"].(float64)
	if affected == 0 {
		fmt.Fprintf(os.Stderr, "No task found with slug '%s'\n", slug)
		os.Exit(1)
	}

	fmt.Printf("Task '%s' set to draft.\n", slug)
}
