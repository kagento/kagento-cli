package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listAllFlag bool

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks from the platform",
	Run:   runTaskList,
}

func init() {
	taskListCmd.Flags().BoolVar(&listAllFlag, "all", false, "Include draft tasks (default: only published)")
	taskCmd.AddCommand(taskListCmd)
}

func runTaskList(cmd *cobra.Command, args []string) {
	var where string
	if listAllFlag {
		where = "{}"
	} else {
		where = `{status: {_eq: "published"}}`
	}

	query := fmt.Sprintf(`
		query {
			tasks(where: %s, order_by: {created_at: desc}) {
				slug
				title
				difficulty
				size
				status
			}
		}`, where)

	data, err := cl.Query(query, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying tasks: %v\n", err)
		os.Exit(1)
	}

	tasksRaw, ok := data["tasks"].([]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Error: unexpected response format")
		os.Exit(1)
	}

	if len(tasksRaw) == 0 {
		fmt.Println("No tasks found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tTITLE\tDIFFICULTY\tSIZE\tSTATUS")
	fmt.Fprintln(w, "----\t-----\t----------\t----\t------")

	for _, t := range tasksRaw {
		task, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			strVal(task, "slug"),
			strVal(task, "title"),
			strVal(task, "difficulty"),
			strVal(task, "size"),
			strVal(task, "status"),
		)
	}
	w.Flush()
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}
