package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	taskSubmitAll     bool
	taskSubmitDraft   bool
	taskSubmitJobs    int
	taskSubmitJSON    bool
	taskSubmitPublish bool
	taskSubmitResume  bool
)

var taskSubmitCmd = &cobra.Command{
	Use:   "submit [path]",
	Short: "Submit a task for server-side building",
	Long: `Validates the task directory, creates a tar archive, uploads it to the
platform, and triggers a server-side build. Container tasks go through the full
build pipeline (Kaniko, SBOM, scanning, signing). vcluster tasks are published
directly from task.yaml metadata.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runTaskSubmit,
}

func init() {
	taskSubmitCmd.Flags().BoolVar(&taskSubmitAll, "all", false, "Submit every task found under the given directory")
	taskSubmitCmd.Flags().BoolVar(&taskSubmitDraft, "draft", false, "Publish submitted tasks as draft when combined with --publish")
	taskSubmitCmd.Flags().IntVar(&taskSubmitJobs, "jobs", 3, "Maximum concurrent task submits when using --all")
	taskSubmitCmd.Flags().BoolVar(&taskSubmitJSON, "json", false, "Output JSON")
	taskSubmitCmd.Flags().BoolVar(&taskSubmitPublish, "publish", false, "Publish automatically after the build completes")
	taskSubmitCmd.Flags().BoolVar(&taskSubmitResume, "resume", false, "Reuse the latest build for the task instead of starting a new one")
	taskCmd.AddCommand(taskSubmitCmd)
}

func runTaskSubmit(cmd *cobra.Command, args []string) {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	opts := submitTaskOptions{
		Publish: taskSubmitPublish,
		Draft:   taskSubmitDraft,
		Resume:  taskSubmitResume,
		Stream:  !taskSubmitJSON && !taskSubmitAll,
	}

	if taskSubmitAll {
		dirs, err := discoverTaskDirs(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error discovering tasks: %v\n", err)
			os.Exit(1)
		}
		if len(dirs) == 0 {
			fmt.Fprintln(os.Stderr, "Error: no task directories found")
			os.Exit(1)
		}

		results := runTaskBatch(dirs, taskSubmitJobs, func(dir string) taskActionResult {
			return submitTaskDir(dir, submitTaskOptions{
				Publish: opts.Publish,
				Draft:   opts.Draft,
				Resume:  opts.Resume,
				Stream:  false,
			})
		})
		if taskSubmitJSON {
			printJSON(results)
		} else {
			printTaskBatchResults("Submitted", results)
		}
		if taskBatchHasErrors(results) {
			os.Exit(1)
		}
		return
	}

	result := submitTaskDir(root, opts)
	if taskSubmitJSON {
		printJSON(result)
		if result.Error != "" {
			os.Exit(1)
		}
		return
	}

	if result.Error != "" {
		fmt.Fprintf(os.Stderr, "\nError: %s\n", result.Error)
		if result.BuildID != "" {
			fmt.Fprintf(os.Stderr, "Retry later with: kagento task wait %s\n", result.BuildID)
		}
		os.Exit(1)
	}

	fmt.Println()
	if result.EnvironmentType == "vcluster" {
		fmt.Printf("Published task: %s\n", result.Slug)
		fmt.Printf("  Task ID: %s\n", result.TaskID)
		fmt.Printf("  Status:  %s\n", result.PublishedStatus)
		return
	}

	if result.ReusedBuild {
		fmt.Printf("Reused build %s\n", result.BuildID)
	}
	if result.PublishedStatus != "" {
		fmt.Printf("Published task: %s\n", result.Slug)
		fmt.Printf("  Build ID: %s\n", result.BuildID)
		fmt.Printf("  Task ID:  %s\n", result.TaskID)
		fmt.Printf("  Status:   %s\n", result.PublishedStatus)
		return
	}

	fmt.Println("Build completed successfully!")
	fmt.Printf("  Build ID: %s\n", result.BuildID)
	fmt.Printf("  Status:   %s\n", result.BuildStatus)
	fmt.Println()
	fmt.Printf("To publish: kagento task publish %s --build-id=%s\n", root, result.BuildID)
}
