package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var watchFlag bool

var taskTestCmd = &cobra.Command{
	Use:   "test [path]",
	Short: "Run test image against solution and show score",
	Args:  cobra.MaximumNArgs(1),
	Run:   runTaskTest,
}

func init() {
	taskTestCmd.Flags().BoolVar(&watchFlag, "watch", false, "Watch for file changes and auto-retest")
	taskCmd.AddCommand(taskTestCmd)
}

func runTaskTest(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	if watchFlag {
		runTaskTestWatch(dir)
		return
	}

	if err := doTaskTest(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func doTaskTest(dir string) error {
	cfg, err := loadAndValidateTask(dir)
	if err != nil {
		return err
	}

	// Check solution/solve.sh exists.
	solvePath := filepath.Join(dir, "solution", "solve.sh")
	if _, err := os.Stat(solvePath); err != nil {
		return fmt.Errorf("solution/solve.sh not found: %w", err)
	}

	// Ensure images are available (build from raw or load from tars).
	if err := ensureBuiltImages(dir, cfg.Slug); err != nil {
		return err
	}

	slug := cfg.Slug

	workspaceDir, cleanupWorkspace, err := prepareTaskTestWorkspace(dir, cfg)
	if err != nil {
		return fmt.Errorf("preparing workspace: %w", err)
	}

	containerName := slug + "-test-user"
	defer func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
		cleanupWorkspace()
	}()

	absDir, _ := filepath.Abs(dir)
	absSolvePath := filepath.Join(absDir, "solution", "solve.sh")

	if cfg.EnvironmentType == "git" {
		// Git tasks have no user container; run solve.sh directly on the host
		// against a fresh template/ checkout. The test image is then run
		// against that workspace.
		fmt.Println("Running solve.sh against template workspace...")
		solveCmd := exec.Command("bash", absSolvePath)
		solveCmd.Dir = workspaceDir
		solveCmd.Env = append(os.Environ(), "WORKSPACE="+workspaceDir)
		solveOut, err := solveCmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n--- solve.sh output ---\n%s\n--- end output ---\n\n", string(solveOut))
			return fmt.Errorf("running solve.sh: %w", err)
		}
		if len(solveOut) > 0 {
			fmt.Print(string(solveOut))
		}
	} else {
		// Start user container with the seeded workspace mounted at /workspace.
		fmt.Println("Starting user container...")
		if err := exec.Command("docker", "run", "-d",
			"--name", containerName,
			"-v", workspaceDir+":/workspace",
			slug+":task",
		).Run(); err != nil {
			return fmt.Errorf("starting user container: %w", err)
		}

		// Copy solve.sh into the container and execute it.
		fmt.Println("Running solve.sh inside user container...")

		if err := exec.Command("docker", "cp", absSolvePath, containerName+":/tmp/solve.sh").Run(); err != nil {
			return fmt.Errorf("copying solve.sh into container: %w", err)
		}

		solveCmd := exec.Command("docker", "exec", containerName, "bash", "/tmp/solve.sh")
		solveOut, err := solveCmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n--- solve.sh output ---\n%s\n--- end output ---\n\n", string(solveOut))
			return fmt.Errorf("running solve.sh: %w", err)
		}
		// Show solve.sh output on success too.
		if len(solveOut) > 0 {
			fmt.Print(string(solveOut))
		}

		// Stop the user container (volume persists).
		_ = exec.Command("docker", "stop", containerName).Run()
	}

	// Run test image with volume mounted.
	fmt.Println("Running tests...")
	testCmd := exec.Command("docker", "run", "--rm",
		"--network", "none",
		"-v", workspaceDir+":/workspace:ro",
		slug+":test",
	)
	out, err := testCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n--- test container output ---\n%s\n--- end output ---\n\n", string(out))
		return fmt.Errorf("running tests: %w", err)
	}

	// Parse and display result.
	var result struct {
		Score       int             `json:"score"`
		TestsPassed int             `json:"tests_passed"`
		TestsTotal  int             `json:"tests_total"`
		Error       string          `json:"error,omitempty"`
		Details     json.RawMessage `json:"details,omitempty"`
	}
	jsonLine := extractLastJSONLine(out)
	if jsonLine == nil {
		return fmt.Errorf("parsing test output: no JSON found\nRaw output: %s", string(out))
	}
	if err := json.Unmarshal(jsonLine, &result); err != nil {
		return fmt.Errorf("parsing test output: %w\nRaw output: %s", err, string(out))
	}

	fmt.Println()
	fmt.Printf("Score: %d/100\n", result.Score)
	fmt.Printf("Tests: %d/%d passed\n", result.TestsPassed, result.TestsTotal)
	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
	}
	if len(result.Details) > 0 {
		pretty, _ := json.MarshalIndent(result.Details, "", "  ")
		fmt.Println(string(pretty))
	}

	if result.Score < 100 {
		fmt.Println("\nWarning: reference solution did not score 100. Fix your solution or tests.")
	}

	return nil
}

// runTaskTestWatch polls for file changes and reruns tests automatically.
func runTaskTestWatch(dir string) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Watching %s for changes (Ctrl+C to stop)...\n\n", absDir)

	// Catch Ctrl+C for clean exit.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	lastMtime := collectMaxMtime(absDir)

	// Run once immediately.
	if err := doTaskTest(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	for {
		select {
		case <-sigCh:
			fmt.Println("\nStopped watching.")
			return
		case <-time.After(2 * time.Second):
		}

		currentMtime := collectMaxMtime(absDir)
		if currentMtime.After(lastMtime) {
			lastMtime = currentMtime
			fmt.Println("\n--- Change detected, rebuilding and retesting ---")
			if err := doTaskTest(dir); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		}
	}
}

func extractLastJSONLine(output []byte) []byte {
	trimmed := bytes.TrimRight(output, "\n\r ")
	if len(trimmed) == 0 {
		return nil
	}
	idx := bytes.LastIndexByte(trimmed, '\n')
	line := bytes.TrimSpace(trimmed[idx+1:])
	if len(line) > 0 && line[0] == '{' {
		return line
	}
	return nil
}

// collectMaxMtime walks a directory and returns the most recent modification time.
func collectMaxMtime(dir string) time.Time {
	var maxTime time.Time
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs and common noise.
		if info.IsDir() && (info.Name() == ".git" || info.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if info.ModTime().After(maxTime) {
			maxTime = info.ModTime()
		}
		return nil
	})
	return maxTime
}
