package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kagento/kagento-cli/internal/client"
	"github.com/spf13/cobra"
)

var (
	taskBuildsStatus string
	taskBuildsLimit  int
	taskBuildsJSON   bool
)

var taskBuildsCmd = &cobra.Command{
	Use:   "builds [slug-or-path]",
	Short: "List your task builds",
	Args:  cobra.MaximumNArgs(1),
	Run:   runTaskBuilds,
}

func init() {
	taskBuildsCmd.Flags().StringVar(&taskBuildsStatus, "status", "", "Filter by build status")
	taskBuildsCmd.Flags().IntVar(&taskBuildsLimit, "limit", 50, "Maximum builds to return")
	taskBuildsCmd.Flags().BoolVar(&taskBuildsJSON, "json", false, "Output JSON")
	taskCmd.AddCommand(taskBuildsCmd)
}

func runTaskBuilds(cmd *cobra.Command, args []string) {
	slug := ""
	if len(args) > 0 {
		_, resolvedSlug, err := taskDirSlug(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		slug = resolvedSlug
	}

	builds, err := cl.ListBuildsWithOptions(client.ListBuildsOptions{
		Slug:   slug,
		Status: taskBuildsStatus,
		Limit:  taskBuildsLimit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing builds: %v\n", err)
		os.Exit(1)
	}

	if taskBuildsJSON {
		printJSON(builds)
		return
	}

	if len(builds) == 0 {
		fmt.Println("No builds found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "BUILD ID\tSLUG\tSTATUS\tPROGRESS\tCREATED")
	_, _ = fmt.Fprintln(w, "--------\t----\t------\t--------\t-------")
	for _, build := range builds {
		progress := build.Progress
		if progress == "" {
			progress = "--"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", build.ID, build.Slug, build.Status, progress, build.CreatedAt)
	}
	_ = w.Flush()
}
