package cmd

import (
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task authoring commands (init, validate, build, submit, publish, builds, logs, update, list)",
}

func init() {
	rootCmd.AddCommand(taskCmd)
}
