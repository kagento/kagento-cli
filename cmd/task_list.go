package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks from the platform",
	Run:   runTaskList,
}

func init() {
	taskCmd.AddCommand(taskListCmd)
}

type taskListItem struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	Difficulty      string `json:"difficulty"`
	Size            string `json:"size"`
	Category        string `json:"category"`
	EnvironmentType string `json:"environment_type"`
	ScoringType     string `json:"scoring_type"`
}

func runTaskList(cmd *cobra.Command, args []string) {
	resp, err := cl.SupabaseGet(
		"/rest/v1/tasks?status=eq.published" +
			"&select=slug,title,difficulty,size,category,environment_type,scoring_type" +
			"&order=created_at.desc",
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing tasks: %v\n", err)
		os.Exit(1)
	}

	var tasks []taskListItem
	if err := json.Unmarshal(resp, &tasks); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tTITLE\tDIFFICULTY\tTYPE\tCATEGORY")
	fmt.Fprintln(w, "----\t-----\t----------\t----\t--------")

	for _, t := range tasks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			t.Slug, t.Title, t.Difficulty, t.EnvironmentType, t.Category,
		)
	}
	w.Flush()
}
