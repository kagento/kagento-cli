package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// prepareLocalWorkspace seeds a writable local workspace from the built task
// image and injects TASK.md from task_instructions so local CLI flows behave
// like real sessions.
func prepareLocalWorkspace(slug, taskInstructions string) (string, func(), error) {
	workspaceDir, err := os.MkdirTemp("", "kagento-workspace-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp workspace: %w", err)
	}
	if err := os.Chmod(workspaceDir, 0o777); err != nil {
		_ = os.RemoveAll(workspaceDir)
		return "", nil, fmt.Errorf("chmod temp workspace: %w", err)
	}

	seedContainer := fmt.Sprintf("%s-seed-%d", slug, time.Now().UnixNano())
	cleanup := func() {
		_ = exec.Command("docker", "rm", "-f", seedContainer).Run()
		_ = os.RemoveAll(workspaceDir)
	}

	if err := exec.Command("docker", "create", "--name", seedContainer, slug+":task").Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("create seed container: %w", err)
	}
	if err := exec.Command("docker", "cp", seedContainer+":/workspace/.", workspaceDir).Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("copy seed workspace: %w", err)
	}

	if taskInstructions != "" {
		taskPath := filepath.Join(workspaceDir, "TASK.md")
		if err := os.WriteFile(taskPath, []byte(taskInstructions), 0o644); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("write TASK.md: %w", err)
		}
	}

	_ = exec.Command("docker", "rm", "-f", seedContainer).Run()
	return workspaceDir, cleanup, nil
}
