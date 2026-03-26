package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	taskCancelJSON bool
)

var taskCancelCmd = &cobra.Command{
	Use:   "cancel <build-id>",
	Short: "Cancel an in-flight build",
	Args:  cobra.ExactArgs(1),
	Run:   runTaskCancel,
}

func init() {
	taskCancelCmd.Flags().BoolVar(&taskCancelJSON, "json", false, "Output JSON")
	taskCmd.AddCommand(taskCancelCmd)
}

func runTaskCancel(cmd *cobra.Command, args []string) {
	result, err := cl.CancelBuild(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error cancelling build: %v\n", err)
		os.Exit(1)
	}
	if taskCancelJSON {
		printJSON(result)
		return
	}
	fmt.Printf("Cancelled build %s\n", args[0])
}
