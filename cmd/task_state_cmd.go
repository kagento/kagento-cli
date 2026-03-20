package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var taskStateCmd = &cobra.Command{
	Use:   "state [path]",
	Short: "Show current state of a task (raw, built, or portable)",
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
		fmt.Fprintln(os.Stderr, "Unknown task state: no Dockerfiles, tars, or .tar.gz found")
		os.Exit(1)
	}

	fmt.Printf("State: %s\n", state)

	switch state {
	case stateRaw:
		fmt.Println("  user/Dockerfile  ✓")
		fmt.Println("  test/Dockerfile  ✓")
	case stateBuilt:
		fmt.Println("  user.tar  ✓")
		fmt.Println("  test.tar  ✓")
	case statePortable:
		fmt.Printf("  archive: %s\n", dir)
	}

	// Try to load and show task.yaml info.
	taskDir := dir
	if state == statePortable {
		fmt.Println("\nExtract with 'kagento task publish' or 'tar xzf' to inspect further.")
		return
	}

	cfg, err := loadTaskConfig(taskDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n  task.yaml: %v\n", err)
		return
	}

	fmt.Printf("\n  slug:           %s\n", cfg.Slug)
	fmt.Printf("  title:          %s\n", cfg.Title)
	fmt.Printf("  difficulty:     %s\n", cfg.Difficulty)
	fmt.Printf("  container_size: %s\n", cfg.ContainerSize)
	fmt.Printf("  time_limit:     %ds\n", cfg.TimeLimitSec)

	if fileExists(taskDir + "/solution/solve.sh") {
		fmt.Println("  solution:       ✓")
	} else {
		fmt.Println("  solution:       ✗ (no solve.sh)")
	}
}
