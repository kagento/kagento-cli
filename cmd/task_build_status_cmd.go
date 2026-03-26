package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	taskBuildStatusWatch bool
	taskBuildStatusJSON  bool
)

var taskBuildStatusCmd = &cobra.Command{
	Use:   "build-status <build-id>",
	Short: "Show build status for a task build",
	Args:  cobra.ExactArgs(1),
	Run:   runTaskBuildStatus,
}

func init() {
	taskBuildStatusCmd.Flags().BoolVar(&taskBuildStatusWatch, "watch", false, "Poll until the build finishes")
	taskBuildStatusCmd.Flags().BoolVar(&taskBuildStatusJSON, "json", false, "Output JSON")
	taskCmd.AddCommand(taskBuildStatusCmd)
}

func runTaskBuildStatus(cmd *cobra.Command, args []string) {
	buildID := args[0]

	if taskBuildStatusWatch {
		var stdout io.Writer = os.Stdout
		var stderr io.Writer = os.Stderr
		if taskBuildStatusJSON {
			stdout, stderr = io.Discard, io.Discard
		}
		build, err := waitForBuildCompletion(cl, buildID, 3*time.Second, stdout, stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if taskBuildStatusJSON {
			printJSON(build)
			return
		}
		fmt.Println()
		printBuildSummary(build)
		return
	}

	build, err := cl.GetBuild(buildID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if taskBuildStatusJSON {
		printJSON(build)
		return
	}
	printBuildSummary(build)
}
