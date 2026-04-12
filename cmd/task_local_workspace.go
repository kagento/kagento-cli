package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// prepareTaskTestWorkspace selects the right workspace seeding strategy for the
// task's environment type. Container tasks seed from the built user image; git
// tasks use a fresh local clone checked out on main.
func prepareTaskTestWorkspace(dir string, cfg *TaskConfig) (string, func(), error) {
	if cfg.EnvironmentType == "git" {
		return prepareGitTemplateWorkspace(dir, cfg.TaskInstructions)
	}
	return prepareLocalWorkspace(cfg.Slug, cfg.TaskInstructions)
}

// prepareGitTemplateWorkspace creates a fresh clone of template/ checked out on
// main so local flows match the repo state contestants get when the task is
// published.
func prepareGitTemplateWorkspace(dir, taskInstructions string) (string, func(), error) {
	templateDir := filepath.Join(dir, "template")
	info, err := os.Stat(templateDir)
	if err != nil || !info.IsDir() {
		return "", nil, fmt.Errorf("template/ directory not found at %s", templateDir)
	}
	if _, err := execLookPath("git"); err != nil {
		return "", nil, fmt.Errorf("git CLI is required to prepare git task workspaces: %w", err)
	}

	workspaceDir, cleanup, err := createWritableWorkspace("kagento-git-workspace-*")
	if err != nil {
		return "", nil, err
	}

	if err := cloneGitWorkspace(templateDir, workspaceDir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("clone template repo: %w", err)
	}

	if taskInstructions != "" {
		if err := writeGitWorkspaceTaskInstructions(workspaceDir, taskInstructions); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("write TASK.md: %w", err)
		}
	}

	return workspaceDir, cleanup, nil
}

func cloneGitWorkspace(templateDir, workspaceDir string) error {
	if err := runGitWorkspaceCommand("", "clone", "--quiet", "--origin", "origin", templateDir, workspaceDir); err != nil {
		return err
	}
	if err := runGitWorkspaceCommand(workspaceDir, "checkout", "--quiet", "main"); err != nil {
		return err
	}
	if err := runGitWorkspaceCommand(workspaceDir, "reset", "--quiet", "--hard", "origin/main"); err != nil {
		return err
	}
	return nil
}

func writeGitWorkspaceTaskInstructions(workspaceDir, taskInstructions string) error {
	taskPath := filepath.Join(workspaceDir, "TASK.md")
	if err := os.WriteFile(taskPath, []byte(taskInstructions), 0o644); err != nil {
		return err
	}

	excludePath := filepath.Join(workspaceDir, ".git", "info", "exclude")
	data, err := os.ReadFile(excludePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	marker := "\nTASK.md\n"
	current := "\n" + string(data)
	if !strings.Contains(current, marker) {
		f, err := os.OpenFile(excludePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer func() {
			_ = f.Close()
		}()
		if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}
		if _, err := f.WriteString("TASK.md\n"); err != nil {
			return err
		}
	}
	return nil
}

// prepareLocalWorkspace seeds a writable local workspace from the built task
// image and injects TASK.md from task_instructions so local CLI flows behave
// like real sessions.
func prepareLocalWorkspace(slug, taskInstructions string) (string, func(), error) {
	workspaceDir, cleanup, err := createWritableWorkspace("kagento-workspace-*")
	if err != nil {
		return "", nil, err
	}

	seedContainer := fmt.Sprintf("%s-seed-%d", slug, time.Now().UnixNano())
	cleanupWithContainer := func() {
		_ = exec.Command("docker", "rm", "-f", seedContainer).Run()
		cleanup()
	}

	if err := exec.Command("docker", "create", "--name", seedContainer, slug+":task").Run(); err != nil {
		cleanupWithContainer()
		return "", nil, fmt.Errorf("create seed container: %w", err)
	}
	if err := exec.Command("docker", "cp", seedContainer+":/workspace/.", workspaceDir).Run(); err != nil {
		cleanupWithContainer()
		return "", nil, fmt.Errorf("copy seed workspace: %w", err)
	}

	if taskInstructions != "" {
		taskPath := filepath.Join(workspaceDir, "TASK.md")
		if err := os.WriteFile(taskPath, []byte(taskInstructions), 0o644); err != nil {
			cleanupWithContainer()
			return "", nil, fmt.Errorf("write TASK.md: %w", err)
		}
	}

	_ = exec.Command("docker", "rm", "-f", seedContainer).Run()
	return workspaceDir, cleanupWithContainer, nil
}

func createWritableWorkspace(pattern string) (string, func(), error) {
	workspaceDir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return "", nil, fmt.Errorf("create temp workspace: %w", err)
	}
	if err := os.Chmod(workspaceDir, 0o777); err != nil {
		_ = os.RemoveAll(workspaceDir)
		return "", nil, fmt.Errorf("chmod temp workspace: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(workspaceDir)
	}
	return workspaceDir, cleanup, nil
}

func runGitWorkspaceCommand(dir string, args ...string) error {
	cmdArgs := args
	if dir != "" {
		cmdArgs = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
