package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	taskRetryWait bool
	taskRetryJSON bool
)

var taskRetryCmd = &cobra.Command{
	Use:   "retry <build-id>",
	Short: "Retry a completed or failed build using the same uploaded source",
	Args:  cobra.ExactArgs(1),
	Run:   runTaskRetry,
}

func init() {
	taskRetryCmd.Flags().BoolVar(&taskRetryWait, "wait", false, "Wait for the retried build to finish")
	taskRetryCmd.Flags().BoolVar(&taskRetryJSON, "json", false, "Output JSON")
	taskCmd.AddCommand(taskRetryCmd)
}

func runTaskRetry(cmd *cobra.Command, args []string) {
	result, err := cl.RetryBuild(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrying build: %v\n", err)
		os.Exit(1)
	}

	build := result
	if taskRetryWait {
		var stdout io.Writer = os.Stdout
		var stderr io.Writer = os.Stderr
		if taskRetryJSON {
			stdout, stderr = io.Discard, io.Discard
		}
		build, err = waitForBuildCompletion(cl, result.ID, 3*time.Second, stdout, stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error waiting for build: %v\n", err)
			os.Exit(1)
		}
	}

	if taskRetryJSON {
		printJSON(build)
		return
	}
	fmt.Printf("Retried build for %s\n", result.Slug)
	printBuildSummary(build)
	if taskRetryWait {
		fmt.Println()
		fmt.Printf("To publish: kagento task publish --build-id=%s\n", build.ID)
	}
}
