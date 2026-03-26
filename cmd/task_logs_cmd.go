package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kagento/kagento-cli/internal/client"
	"github.com/spf13/cobra"
)

var (
	taskLogsJSON bool
)

var taskLogsCmd = &cobra.Command{
	Use:   "logs <build-id>",
	Short: "Stream build progress updates",
	Long:  "Streams build status, progress, and terminal error updates for a task build.",
	Args:  cobra.ExactArgs(1),
	Run:   runTaskLogs,
}

func init() {
	taskLogsCmd.Flags().BoolVar(&taskLogsJSON, "json", false, "Output only the final build JSON")
	taskCmd.AddCommand(taskLogsCmd)
}

func runTaskLogs(cmd *cobra.Command, args []string) {
	var (
		build *client.BuildStatus
		err   error
	)
	if taskLogsJSON {
		build, err = waitForBuildCompletion(cl, args[0], 3*time.Second, io.Discard, io.Discard)
	} else {
		build, err = watchBuildProgress(args[0], 3*time.Second)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if taskLogsJSON {
		printJSON(build)
	}
}
