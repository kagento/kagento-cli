package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var taskInitCmd = &cobra.Command{
	Use:   "init <slug>",
	Short: "Scaffold a new task directory from template",
	Args:  cobra.ExactArgs(1),
	Run:   runTaskInit,
}

func init() {
	taskCmd.AddCommand(taskInitCmd)
}

func runTaskInit(cmd *cobra.Command, args []string) {
	slug := args[0]
	dir := filepath.Join("tasks", slug)

	if _, err := os.Stat(dir); err == nil {
		fmt.Fprintf(os.Stderr, "Error: directory %s already exists\n", dir)
		os.Exit(1)
	}

	for _, sub := range []string{"solution", "user", "test"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
			os.Exit(1)
		}
	}

	// task.yaml
	taskYAML := fmt.Sprintf(`version: 1
slug: %s
title: "%s"
short_desc: "TODO: describe your task"
difficulty: medium
container_size: small
time_limit_sec: 3600
`, slug, slug)
	writeFile(filepath.Join(dir, "task.yaml"), taskYAML)

	// user/TASK.md
	taskMD := fmt.Sprintf(`# %s

## Description

TODO: Describe the task.

## Constraints

- Your solution must be in `+"`/workspace/`"+`.

## Scoring

- Score is 0-100.

## Time Limit

3600 seconds
`, slug)
	writeFile(filepath.Join(dir, "user", "TASK.md"), taskMD)

	// user/Dockerfile
	dockerfileUser := `FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        python3 \
        vim \
        nano \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash contestant
RUN mkdir -p /workspace && chown contestant:contestant /workspace

COPY TASK.md /workspace/TASK.md
RUN chown contestant:contestant /workspace/TASK.md

USER contestant
WORKDIR /workspace

CMD ["sleep", "infinity"]
`
	writeFile(filepath.Join(dir, "user", "Dockerfile"), dockerfileUser)

	// test/run_tests.py
	testScript := `#!/usr/bin/env python3
"""Test runner. Expects workspace at /workspace. Prints JSON result."""
import json

def main():
    # TODO: implement your test logic
    print(json.dumps({
        "score": 0,
        "tests_passed": 0,
        "tests_total": 1,
    }))

if __name__ == "__main__":
    main()
`
	writeFile(filepath.Join(dir, "test", "run_tests.py"), testScript)

	// test/Dockerfile
	dockerfileTest := `FROM python:3.11-slim

RUN mkdir -p /test
COPY run_tests.py /test/run_tests.py
RUN chmod +x /test/run_tests.py
ENTRYPOINT ["python3", "/test/run_tests.py"]
`
	writeFile(filepath.Join(dir, "test", "Dockerfile"), dockerfileTest)

	// solution/solve.sh
	solveSH := `#!/bin/bash
# Reference solution — this script is executed inside the user container.
# It must reproduce the fully solved state in /workspace.

# TODO: write commands that produce the solution, e.g.:
# cat > /workspace/main.py << 'EOF'
# #!/usr/bin/env python3
# ... solution code ...
# EOF
`
	writeFile(filepath.Join(dir, "solution", "solve.sh"), solveSH)
	if err := os.Chmod(filepath.Join(dir, "solution", "solve.sh"), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting solve.sh permissions: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created task scaffold at %s/\n", dir)
	fmt.Println("  task.yaml            — task metadata (public short_desc)")
	fmt.Println("  user/Dockerfile      — contestant container image")
	fmt.Println("  user/TASK.md         — private task description (COPYed into image)")
	fmt.Println("  test/Dockerfile      — test runner image")
	fmt.Println("  test/run_tests.py    — test script (COPYed into image)")
	fmt.Println("  solution/solve.sh    — reference solution script (used by 'kagento task test')")
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
		os.Exit(1)
	}
}
