package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var taskStateCmd = &cobra.Command{
	Use:   "state [path]",
	Short: "Show current state of a task directory",
	Args:  cobra.MaximumNArgs(1),
	Run:   runTaskState,
}

func init() {
	taskCmd.AddCommand(taskStateCmd)
}

func runTaskState(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	state := detectState(dir)
	if state == stateUnknown {
		fmt.Fprintln(os.Stderr, "Unknown task state: no Dockerfiles found")
		os.Exit(1)
	}

	fmt.Printf("State: %s\n", state)
	fmt.Println("  user/Dockerfile  ✓")
	fmt.Println("  test/Dockerfile  ✓")

	cfg, err := loadTaskConfig(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n  task.yaml: %v\n", err)
		return
	}

	fmt.Printf("\n  slug:           %s\n", cfg.Slug)
	fmt.Printf("  title:          %s\n", cfg.Title)
	fmt.Printf("  difficulty:     %s\n", cfg.Difficulty)
	fmt.Printf("  size: %s\n", cfg.ContainerSize)
	fmt.Printf("  time_limit:     %ds\n", cfg.TimeLimitSec)

	if fileExists(dir + "/solution/solve.sh") {
		fmt.Println("  solution:       ✓")
	} else {
		fmt.Println("  solution:       ✗ (no solve.sh)")
	}
}
