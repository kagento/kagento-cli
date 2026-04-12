package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareGitTemplateWorkspaceChecksOutMainAndKeepsBranchHistory(t *testing.T) {
	if _, err := execLookPath("git"); err != nil {
		t.Skip("git CLI not available")
	}

	dir := t.TempDir()
	templateDir := filepath.Join(dir, "template")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(template): %v", err)
	}

	writeTaskTestFile(t, filepath.Join(templateDir, "README.md"), "# main\n")
	runGitTestCmd(t, templateDir, "init", "-b", "main")
	runGitTestCmd(t, templateDir, "config", "user.name", "Test Author")
	runGitTestCmd(t, templateDir, "config", "user.email", "author@example.com")
	runGitTestCmd(t, templateDir, "add", "README.md")
	runGitTestCmd(t, templateDir, "-c", "commit.gpgsign=false", "commit", "-m", "main")

	runGitTestCmd(t, templateDir, "checkout", "-b", "feature")
	writeTaskTestFile(t, filepath.Join(templateDir, "README.md"), "# feature\n")
	runGitTestCmd(t, templateDir, "add", "README.md")
	runGitTestCmd(t, templateDir, "-c", "commit.gpgsign=false", "commit", "-m", "feature")

	workspaceDir, cleanup, err := prepareGitTemplateWorkspace(dir, "private instructions")
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("prepareGitTemplateWorkspace() error = %v", err)
	}

	head, err := gitOutput(workspaceDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	if got := strings.TrimSpace(head); got != "main" {
		t.Fatalf("HEAD branch = %q, want %q", got, "main")
	}

	readme, err := os.ReadFile(filepath.Join(workspaceDir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile(README.md): %v", err)
	}
	if string(readme) != "# main\n" {
		t.Fatalf("README.md = %q, want main branch contents", string(readme))
	}

	branches, err := gitOutput(workspaceDir, "branch", "-a")
	if err != nil {
		t.Fatalf("git branch -a: %v", err)
	}
	if !strings.Contains(branches, "remotes/origin/feature") {
		t.Fatalf("git branch -a output = %q, want remotes/origin/feature", branches)
	}

	taskInstructions, err := os.ReadFile(filepath.Join(workspaceDir, "TASK.md"))
	if err != nil {
		t.Fatalf("ReadFile(TASK.md): %v", err)
	}
	if string(taskInstructions) != "private instructions" {
		t.Fatalf("TASK.md = %q", string(taskInstructions))
	}

	status, err := gitOutput(workspaceDir, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if strings.TrimSpace(status) != "" {
		t.Fatalf("git status output = %q, want clean workspace", status)
	}
}

func TestResolveInteractiveShellFallsBackToKnownShell(t *testing.T) {
	original := os.Getenv("SHELL")
	t.Cleanup(func() {
		if original == "" {
			_ = os.Unsetenv("SHELL")
			return
		}
		_ = os.Setenv("SHELL", original)
	})
	if err := os.Setenv("SHELL", "/definitely/missing-shell"); err != nil {
		t.Fatalf("Setenv(SHELL): %v", err)
	}

	shell := resolveInteractiveShell()
	if shell == "" {
		t.Fatal("resolveInteractiveShell() returned empty shell")
	}
	if _, err := os.Stat(shell); err != nil {
		t.Fatalf("resolved shell %q is not present: %v", shell, err)
	}
}
