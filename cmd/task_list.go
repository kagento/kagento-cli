package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/kagento/kagento-cli/internal/client"
	"github.com/spf13/cobra"
)

var (
	taskListMine   bool
	taskListStatus string
	taskListLimit  int
	taskListJSON   bool
)

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks from the platform",
	Run:   runTaskList,
}

func init() {
	taskListCmd.Flags().BoolVar(&taskListMine, "mine", false, "List your authored tasks instead of public published tasks")
	taskListCmd.Flags().StringVar(&taskListStatus, "status", "", "Filter by task status")
	taskListCmd.Flags().IntVar(&taskListLimit, "limit", 100, "Maximum tasks to return")
	taskListCmd.Flags().BoolVar(&taskListJSON, "json", false, "Output JSON")
	taskCmd.AddCommand(taskListCmd)
}

func runTaskList(cmd *cobra.Command, args []string) {
	tasks, err := cl.ListTasks(client.ListTasksOptions{
		Mine:   taskListMine,
		Status: taskListStatus,
		Limit:  taskListLimit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing tasks: %v\n", err)
		os.Exit(1)
	}

	if taskListJSON {
		printJSON(tasks)
		return
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tTITLE\tSTATUS\tDIFFICULTY\tTYPE\tTAGS")
	fmt.Fprintln(w, "----\t-----\t------\t----------\t----\t----")
	for _, t := range tasks {
		status := t.Status
		if status == "" {
			status = "--"
		}
		difficulty := t.Difficulty
		if difficulty == "" {
			difficulty = "--"
		}
		envType := t.EnvironmentType
		if envType == "" {
			envType = "--"
		}
		tags := "--"
		if len(t.Tags) > 0 {
			tags = strings.Join(t.Tags, ",")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			t.Slug, t.Title, status, difficulty, envType, tags,
		)
	}
	w.Flush()
}
