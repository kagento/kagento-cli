package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var taskInitEnv string

var taskInitCmd = &cobra.Command{
	Use:   "init <slug>",
	Short: "Scaffold a new task directory from template",
	Args:  cobra.ExactArgs(1),
	Run:   runTaskInit,
}

func init() {
	taskInitCmd.Flags().StringVar(&taskInitEnv, "env", "container", "Task environment: container, git, or vcluster")
	taskCmd.AddCommand(taskInitCmd)
}

func runTaskInit(cmd *cobra.Command, args []string) {
	slug := args[0]
	if err := validateTaskSlugValue(slug); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	dir := filepath.Join("tasks", slug)

	if _, err := os.Stat(dir); err == nil {
		fmt.Fprintf(os.Stderr, "Error: directory %s already exists\n", dir)
		os.Exit(1)
	}

	switch taskInitEnv {
	case "git":
		scaffoldGitTask(dir, slug)
		return
	case "container", "":
		// fall through to default container scaffold below
	default:
		fmt.Fprintf(os.Stderr, "Error: --env %q is not supported by init (supported: container, git)\n", taskInitEnv)
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
short_desc: "TODO: one-liner for task cards"
description: |
  ## Overview

  TODO: public description shown on the task detail page.
  Leave empty or remove if you want to keep details private until SSH.
task_instructions: |
  # %s

  ## Description

  TODO: Describe the task.

  ## Constraints

  - Your solution must be in `+"`/workspace/`"+`.

  ## Scoring

  - Score is 0-100.
difficulty: medium
size: small
time_limit_sec: 3600
`, slug, slug, slug)
	writeFile(filepath.Join(dir, "task.yaml"), taskYAML)

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
	fmt.Println("  task.yaml            — task metadata (including private task_instructions)")
	fmt.Println("  user/Dockerfile      — contestant container image")
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

func scaffoldGitTask(dir, slug string) {
	for _, sub := range []string{"solution", "template", "test"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
			os.Exit(1)
		}
	}

	taskYAML := fmt.Sprintf(`version: 1
slug: %s
title: "%s"
short_desc: "TODO: one-liner for task cards"
description: |
  ## Overview

  TODO: public description shown on the task detail page.
task_instructions: |
  # %s

  ## Description

  TODO: Describe the task. Contestants will clone the template repo, commit
  changes, and push. The test image runs against the pushed repo state.

  ## Scoring

  - Score is 0-100.
difficulty: medium
size: small
time_limit_sec: 3600
scoring_type: gradient
environment_type: git
`, slug, slug, slug)
	writeFile(filepath.Join(dir, "task.yaml"), taskYAML)

	// template/README.md
	templateReadme := fmt.Sprintf(`# %s

Edit the files in this repo to solve the task, then commit and push.

Run `+"`"+`git push`+"`"+` to trigger scoring.
`, slug)
	writeFile(filepath.Join(dir, "template", "README.md"), templateReadme)

	// template/main.py (stub file)
	templateMain := `#!/usr/bin/env python3
"""Starter code. Replace with your solution."""


def solve():
    # TODO: implement your solution
    return None


if __name__ == "__main__":
    print(solve())
`
	writeFile(filepath.Join(dir, "template", "main.py"), templateMain)

	// test/run_tests.py
	testScript := `#!/usr/bin/env python3
"""Test runner. Expects the contestant's repo checked out at /workspace. Prints JSON result."""
import json
import os


def main():
    workspace = "/workspace"
    has_main = os.path.exists(os.path.join(workspace, "main.py"))

    # TODO: implement your test logic
    print(json.dumps({
        "score": 100 if has_main else 0,
        "tests_passed": 1 if has_main else 0,
        "tests_total": 1,
    }))


if __name__ == "__main__":
    main()
`
	writeFile(filepath.Join(dir, "test", "run_tests.py"), testScript)

	// test/Dockerfile
	dockerfileTest := `FROM python:3.12-slim

RUN mkdir -p /test
COPY run_tests.py /test/run_tests.py
RUN chmod +x /test/run_tests.py
ENTRYPOINT ["python3", "/test/run_tests.py"]
`
	writeFile(filepath.Join(dir, "test", "Dockerfile"), dockerfileTest)

	// solution/solve.sh (placeholder)
	solveSH := `#!/bin/bash
# Reference solution — this script is executed against a fresh checkout of
# the template/ directory at /workspace when running 'kagento task test'.
#
# TODO: write commands that produce the fully solved repo state, e.g.:
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

	fmt.Printf("Created git task scaffold at %s/\n", dir)
	fmt.Println("  task.yaml             — task metadata (environment_type: git)")
	fmt.Println("  template/             — initial repo contents pushed to Gitea")
	fmt.Println("  template/README.md    — boilerplate README for contestants")
	fmt.Println("  template/main.py      — stub file contestants edit")
	fmt.Println("  test/Dockerfile       — scoring image")
	fmt.Println("  test/run_tests.py     — test script (COPYed into image)")
	fmt.Println("  solution/solve.sh     — reference solution script (used by 'kagento task test')")
}
