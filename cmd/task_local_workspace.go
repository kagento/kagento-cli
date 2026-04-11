package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// prepareTaskTestWorkspace selects the right workspace seeding strategy for the
// task's environment type. Container tasks seed from the built user image; git
// tasks copy the template/ directory as-is.
func prepareTaskTestWorkspace(dir string, cfg *TaskConfig) (string, func(), error) {
	if cfg.EnvironmentType == "git" {
		return prepareGitTemplateWorkspace(dir, cfg.TaskInstructions)
	}
	return prepareLocalWorkspace(cfg.Slug, cfg.TaskInstructions)
}

// prepareGitTemplateWorkspace copies template/ into a fresh writable temp
// directory so solve.sh and the test image run against a clean clone-like
// state.
func prepareGitTemplateWorkspace(dir, taskInstructions string) (string, func(), error) {
	templateDir := filepath.Join(dir, "template")
	info, err := os.Stat(templateDir)
	if err != nil || !info.IsDir() {
		return "", nil, fmt.Errorf("template/ directory not found at %s", templateDir)
	}

	workspaceDir, err := os.MkdirTemp("", "kagento-git-workspace-*")
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

	if err := copyTreeContents(templateDir, workspaceDir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("copy template: %w", err)
	}

	if taskInstructions != "" {
		if err := os.WriteFile(filepath.Join(workspaceDir, "TASK.md"), []byte(taskInstructions), 0o644); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("write TASK.md: %w", err)
		}
	}

	return workspaceDir, cleanup, nil
}

// copyTreeContents copies every file under src into dst, preserving the
// relative layout. It does not follow symlinks.
func copyTreeContents(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Skip symlinks — template files must be regular.
			return nil
		}
		return copyRegularFile(path, target, info.Mode())
	})
}

func copyRegularFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()
	_, err = io.Copy(out, in)
	return err
}

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
