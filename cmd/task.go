package cmd

import (
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task authoring commands (init, validate, build, test, pack, publish, unpublish, delete, list)",
}

func init() {
	rootCmd.AddCommand(taskCmd)
}
