package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	taskWaitJSON bool
)

var taskWaitCmd = &cobra.Command{
	Use:   "wait <build-id>",
	Short: "Wait for a build to finish",
	Args:  cobra.ExactArgs(1),
	Run:   runTaskWait,
}

func init() {
	taskWaitCmd.Flags().BoolVar(&taskWaitJSON, "json", false, "Output JSON")
	taskCmd.AddCommand(taskWaitCmd)
}

func runTaskWait(cmd *cobra.Command, args []string) {
	var stdout io.Writer = os.Stdout
	var stderr io.Writer = os.Stderr
	if taskWaitJSON {
		stdout, stderr = io.Discard, io.Discard
	}
	build, err := waitForBuildCompletion(cl, args[0], 3*time.Second, stdout, stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error waiting for build: %v\n", err)
		os.Exit(1)
	}
	if taskWaitJSON {
		printJSON(build)
		return
	}
	fmt.Println()
	printBuildSummary(build)
}
