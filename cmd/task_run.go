package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var taskRunCmd = &cobra.Command{
	Use:   "run [path]",
	Short: "Run task locally — start user container with interactive shell",
	Args:  cobra.MaximumNArgs(1),
	Run:   runTaskRun,
}

func init() {
	taskCmd.AddCommand(taskRunCmd)
}

func runTaskRun(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	cfg, err := loadTaskConfig(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := ensureBuiltImages(dir, cfg.Slug); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	slug := cfg.Slug
	containerName := slug + "-run"
	workspaceDir, cleanupWorkspace, err := prepareLocalWorkspace(slug, cfg.TaskInstructions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error preparing workspace: %v\n", err)
		os.Exit(1)
	}

	// Clean up on exit.
	defer func() {
		fmt.Println("\nCleaning up...")
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
		cleanupWorkspace()
	}()

	// Run user container interactively.
	fmt.Printf("Starting %s — interactive shell\n", slug)
	fmt.Println("Read /workspace/TASK.md for instructions. Exit the shell when done.")
	fmt.Println()

	dockerRun := exec.Command("docker", "run", "-it", "--rm",
		"--name", containerName,
		"-v", workspaceDir+":/workspace",
		slug+":task",
		"/bin/bash",
	)
	dockerRun.Stdin = os.Stdin
	dockerRun.Stdout = os.Stdout
	dockerRun.Stderr = os.Stderr

	if err := dockerRun.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 0 {
			fmt.Fprintf(os.Stderr, "Error running task container: %v\n", err)
			os.Exit(1)
		}
	}

	// Ask if they want to run tests.
	fmt.Print("\nRun tests against your solution? [Y/n] ")
	var answer string
	_, _ = fmt.Scanln(&answer)
	if answer != "" && answer != "y" && answer != "Y" {
		return
	}

	// Run test image.
	fmt.Println("Running tests...")
	out, err := exec.Command("docker", "run", "--rm",
		"--network", "none",
		"-v", workspaceDir+":/workspace:ro",
		slug+":test",
	).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running tests: %v\n", err)
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			fmt.Fprintln(os.Stderr, string(exitErr.Stderr))
		}
		os.Exit(1)
	}

	fmt.Println(string(out))
}
