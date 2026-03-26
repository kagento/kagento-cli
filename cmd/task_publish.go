package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	draftFlag         bool
	buildIDFlag       string
	taskPublishAll    bool
	taskPublishJobs   int
	taskPublishJSON   bool
	taskPublishLatest bool
)

var taskPublishCmd = &cobra.Command{
	Use:   "publish [path-or-slug]",
	Short: "Publish a completed build as a task on the platform",
	Long: `Promotes a completed server-side build to a published (or draft) task.
Use --build-id to publish a specific build, or --latest to publish the latest
completed build for a task slug or task directory.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runTaskPublish,
}

func init() {
	taskPublishCmd.Flags().BoolVar(&draftFlag, "draft", false, "Set task status to draft instead of published")
	taskPublishCmd.Flags().StringVar(&buildIDFlag, "build-id", "", "Build ID to publish")
	taskPublishCmd.Flags().BoolVar(&taskPublishAll, "all", false, "Publish every task found under the given directory")
	taskPublishCmd.Flags().IntVar(&taskPublishJobs, "jobs", 3, "Maximum concurrent task publishes when using --all")
	taskPublishCmd.Flags().BoolVar(&taskPublishJSON, "json", false, "Output JSON")
	taskPublishCmd.Flags().BoolVar(&taskPublishLatest, "latest", false, "Publish the latest completed build for the task")
	taskCmd.AddCommand(taskPublishCmd)
}

func runTaskPublish(cmd *cobra.Command, args []string) {
	root := ""
	if len(args) > 0 {
		root = args[0]
	}

	if taskPublishAll && !taskPublishLatest && buildIDFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: --all requires --latest")
		os.Exit(1)
	}
	if taskPublishAll && buildIDFlag != "" {
		fmt.Fprintln(os.Stderr, "Error: --all cannot be combined with --build-id")
		os.Exit(1)
	}
	if buildIDFlag == "" && !taskPublishLatest {
		fmt.Fprintln(os.Stderr, "Error: --build-id or --latest is required")
		os.Exit(1)
	}
	if buildIDFlag != "" && taskPublishLatest {
		fmt.Fprintln(os.Stderr, "Error: use either --build-id or --latest, not both")
		os.Exit(1)
	}

	opts := publishTaskOptions{
		BuildID: buildIDFlag,
		Draft:   draftFlag,
		Latest:  taskPublishLatest,
		Stream:  !taskPublishJSON && !taskPublishAll,
	}

	if taskPublishAll {
		if root == "" {
			root = "."
		}
		dirs, err := discoverTaskDirs(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error discovering tasks: %v\n", err)
			os.Exit(1)
		}
		if len(dirs) == 0 {
			fmt.Fprintln(os.Stderr, "Error: no task directories found")
			os.Exit(1)
		}

		results := runTaskBatch(dirs, taskPublishJobs, func(dir string) taskActionResult {
			return publishTaskDir(dir, publishTaskOptions{
				Draft:  opts.Draft,
				Latest: true,
				Stream: false,
			})
		})
		if taskPublishJSON {
			printJSON(results)
		} else {
			printTaskBatchResults("Published", results)
		}
		if taskBatchHasErrors(results) {
			os.Exit(1)
		}
		return
	}

	result := publishTaskDir(root, opts)
	if taskPublishJSON {
		printJSON(result)
		if result.Error != "" {
			os.Exit(1)
		}
		return
	}
	if result.Error != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", result.Error)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Published task: %s\n", result.Slug)
	fmt.Printf("  Build ID: %s\n", result.BuildID)
	fmt.Printf("  Task ID:  %s\n", result.TaskID)
	fmt.Printf("  Status:   %s\n", result.PublishedStatus)
}
