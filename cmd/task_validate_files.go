package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// validateContainerTask checks Dockerfiles for common issues.
func validateContainerTask(dir string) []string {
	var errs []string

	// TASK.md must not exist in user/ — task_instructions in task.yaml is the only source.
	if _, err := os.Stat(filepath.Join(dir, "user", "TASK.md")); err == nil {
		errs = append(errs, "user/TASK.md must not exist — use task_instructions in task.yaml instead")
	}

	for _, role := range []string{"user", "test"} {
		dockerfilePath := filepath.Join(dir, role, "Dockerfile")
		issues := validateDockerfile(dockerfilePath, role)
		errs = append(errs, issues...)
	}

	return errs
}

func validateDockerfile(path, role string) []string {
	var errs []string
	prefix := fmt.Sprintf("%s/Dockerfile", role)

	f, err := os.Open(path)
	if err != nil {
		return []string{fmt.Sprintf("%s: cannot read file", prefix)}
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	var (
		hasFrom       bool
		hasUser       bool
		baseImage     string
		hasCmd        bool
		lineNum       int
		dangInstructs = []string{"--privileged", "--security-opt", "hostPath", "hostNetwork", "hostPID"}
	)

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		upper := strings.ToUpper(line)

		// Check FROM.
		if strings.HasPrefix(upper, "FROM ") {
			hasFrom = true
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				baseImage = parts[1]
			}
		}

		// Check USER instruction.
		if strings.HasPrefix(upper, "USER ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				user := parts[1]
				if user != "root" && user != "0" {
					hasUser = true
				}
			}
		}

		// Check CMD.
		if strings.HasPrefix(upper, "CMD ") || strings.HasPrefix(upper, "ENTRYPOINT ") {
			hasCmd = true
		}

		// Check for dangerous instructions.
		for _, d := range dangInstructs {
			if strings.Contains(line, d) {
				errs = append(errs, fmt.Sprintf("%s:%d: dangerous instruction: %s", prefix, lineNum, d))
			}
		}

		// TASK.md is injected by the platform, not COPYed in Dockerfile.
		if strings.HasPrefix(upper, "COPY ") && strings.Contains(line, "TASK.md") {
			errs = append(errs, fmt.Sprintf("%s:%d: do not COPY TASK.md — it is injected by the platform via task_instructions in task.yaml", prefix, lineNum))
		}
	}

	if !hasFrom {
		errs = append(errs, fmt.Sprintf("%s: missing FROM instruction", prefix))
	}

	if baseImage != "" && !isAllowedBaseImage(baseImage) {
		errs = append(errs, fmt.Sprintf("%s: base image %q not in allowed list. Allowed: %s",
			prefix, baseImage, strings.Join(allowedBaseImages, ", ")))
	}

	if role == "user" {
		if !hasUser {
			errs = append(errs, fmt.Sprintf("%s: must have a USER instruction (non-root)", prefix))
		}
		if !hasCmd {
			errs = append(errs, fmt.Sprintf("%s: must have a CMD instruction (typically CMD [\"sleep\", \"infinity\"])", prefix))
		}
	}

	if role == "test" && !hasCmd {
		errs = append(errs, fmt.Sprintf("%s: must have an ENTRYPOINT or CMD instruction", prefix))
	}

	contextDir := filepath.Join(filepath.Dir(path))
	size, _ := dirSize(contextDir)
	if size > 100*1024*1024 {
		errs = append(errs, fmt.Sprintf("%s: build context is %.0fMB (max recommended: 100MB)", prefix, float64(size)/1024/1024))
	}

	return errs
}

// allowedBaseImages is the hardcoded allowlist. Kept in sync with the DB table.
var allowedBaseImages = []string{
	"ubuntu:22.04", "ubuntu:24.04",
	"python:3.11-slim", "python:3.12-slim", "python:3.13-slim",
	"node:20-slim", "node:22-slim",
	"golang:1.22-alpine", "golang:1.23-alpine",
	"alpine:3.19", "alpine:3.20",
	"debian:bookworm-slim",
	"rust:1.82-slim",
}

func isAllowedBaseImage(image string) bool {
	for _, allowed := range allowedBaseImages {
		if image == allowed {
			return true
		}
	}
	for _, allowed := range allowedBaseImages {
		parts := strings.SplitN(allowed, ":", 2)
		if len(parts) == 2 {
			ecr := fmt.Sprintf("public.ecr.aws/docker/library/%s:%s", parts[0], parts[1])
			if image == ecr {
				return true
			}
		}
	}
	return false
}

// validateGitTask checks that the task directory has the pieces required for
// a git-environment task: a populated template/ directory and a test image
// Dockerfile, but no user/Dockerfile or vcluster provision manifests.
func validateGitTask(dir string, cfg *TaskConfig) []string {
	var errs []string

	// test/Dockerfile is required for scoring.
	testDockerfile := filepath.Join(dir, "test", "Dockerfile")
	if !fileExists(testDockerfile) {
		errs = append(errs, "git task requires test/Dockerfile for the scoring image")
	} else {
		errs = append(errs, validateDockerfile(testDockerfile, "test")...)
	}

	// Git tasks must not carry a user image.
	if fileExists(filepath.Join(dir, "user", "Dockerfile")) {
		errs = append(errs, "git task must not contain user/Dockerfile — git tasks do not use a user container")
	}

	// Git tasks must not carry vcluster provision manifests.
	provisionDir := filepath.Join(dir, "provision", "manifests")
	if fileExists(provisionDir) {
		errs = append(errs, "git task must not contain provision/manifests — those are only for vcluster tasks")
	}
	if len(cfg.Provision.Manifests) > 0 {
		errs = append(errs, "git task must not declare provision.manifests in task.yaml")
	}

	// template/ directory is required and must be a real git repository.
	templateDir := filepath.Join(dir, "template")
	info, err := os.Stat(templateDir)
	if err != nil || !info.IsDir() {
		errs = append(errs, "git task requires a template/ directory")
		return errs
	}
	if !fileExists(filepath.Join(templateDir, ".git")) {
		errs = append(errs, "git task template/ must be a real git repository")
		return errs
	}
	errs = append(errs, validateGitTemplateRepo(templateDir)...)

	const (
		gitTemplateMaxFileSize = 1 << 20 // 1 MB per file
		gitTemplateMaxFiles    = 100
	)

	fileCount := 0
	walkErr := filepath.Walk(templateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(templateDir, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			switch {
			case info.Name() == ".git":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if info.IsDir() {
			return nil
		}
		fileCount++
		if info.Size() > gitTemplateMaxFileSize {
			errs = append(errs, fmt.Sprintf("template/%s: %.1f MB exceeds 1 MB limit", filepath.ToSlash(rel), float64(info.Size())/(1024*1024)))
		}
		return nil
	})
	if walkErr != nil {
		errs = append(errs, fmt.Sprintf("walk template/: %v", walkErr))
		return errs
	}

	if fileCount == 0 {
		errs = append(errs, "git task template/ directory must contain at least one file")
	}
	if fileCount > gitTemplateMaxFiles {
		errs = append(errs, fmt.Sprintf("template/ has %d files (max %d)", fileCount, gitTemplateMaxFiles))
	}

	// Validate scoring_type is one of the supported ones when set.
	if cfg.ScoringType != "" {
		switch cfg.ScoringType {
		case "binary", "gradient", "optimization":
		default:
			errs = append(errs, fmt.Sprintf("scoring_type %q is not supported for git tasks (use binary, gradient, or optimization)", cfg.ScoringType))
		}
	}

	return errs
}

func validateGitTemplateRepo(templateDir string) []string {
	var errs []string

	if _, err := execLookPath("git"); err != nil {
		return []string{"git task templates require the git CLI to be installed for validation"}
	}

	if err := runGitValidation(templateDir, "rev-parse", "--is-inside-work-tree"); err != nil {
		errs = append(errs, fmt.Sprintf("template/.git is present but not a valid git repository: %v", err))
		return errs
	}
	if err := runGitValidation(templateDir, "rev-parse", "--verify", "refs/heads/main"); err != nil {
		errs = append(errs, "git task template must contain a local main branch")
	}
	status, err := gitOutput(templateDir, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		errs = append(errs, fmt.Sprintf("failed to inspect template git status: %v", err))
		return errs
	}
	if strings.TrimSpace(status) != "" {
		errs = append(errs, "git task template must be clean (commit or remove untracked changes before publishing)")
	}

	return errs
}

func runGitValidation(dir string, args ...string) error {
	_, err := gitOutput(dir, args...)
	return err
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

// validateVclusterTask checks provision manifests and optionally validates a
// test image Dockerfile. Legacy checks in task.yaml are ignored.
func validateVclusterTask(dir string, cfg *TaskConfig) []string {
	var errs []string

	if len(cfg.Provision.Manifests) == 0 {
		errs = append(errs, "vcluster task must have at least one provision manifest")
	}
	for _, manifestPath := range cfg.Provision.Manifests {
		fullPath := filepath.Join(dir, manifestPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			errs = append(errs, fmt.Sprintf("provision manifest %s: %v", manifestPath, err))
			continue
		}

		errs = append(errs, validateK8sManifest(data, manifestPath)...)
	}

	testDockerfile := filepath.Join(dir, "test", "Dockerfile")
	if fileExists(testDockerfile) {
		errs = append(errs, validateDockerfile(testDockerfile, "test")...)
	} else {
		errs = append(errs, "vcluster task scoring image must be defined by test/Dockerfile")
	}

	return errs
}

// dangerousK8sFields that should not appear in provision manifests.
var dangerousK8sFields = []string{
	"hostNetwork", "hostPID", "hostIPC", "hostPath",
	"privileged", "allowPrivilegeEscalation",
}

var allowedK8sKinds = map[string]bool{
	"Pod": true, "Deployment": true, "Service": true, "ConfigMap": true,
	"Secret": true, "StatefulSet": true, "DaemonSet": true, "Job": true,
	"CronJob": true, "Ingress": true, "NetworkPolicy": true,
	"Role": true, "RoleBinding": true, "ClusterRole": true, "ClusterRoleBinding": true,
	"HorizontalPodAutoscaler": true, "PersistentVolumeClaim": true,
	"ServiceAccount": true, "Namespace": true, "LimitRange": true, "ResourceQuota": true,
	"PodDisruptionBudget": true,
}

func validateK8sManifest(data []byte, path string) []string {
	var errs []string

	var manifest map[string]interface{}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		errs = append(errs, fmt.Sprintf("%s: invalid YAML: %v", path, err))
		return errs
	}

	kind, _ := manifest["kind"].(string)
	if kind == "" {
		errs = append(errs, fmt.Sprintf("%s: missing 'kind' field", path))
	} else if !allowedK8sKinds[kind] {
		errs = append(errs, fmt.Sprintf("%s: kind %q not allowed", path, kind))
	}

	if _, ok := manifest["apiVersion"]; !ok {
		errs = append(errs, fmt.Sprintf("%s: missing 'apiVersion' field", path))
	}

	content := string(data)
	for _, field := range dangerousK8sFields {
		if strings.Contains(content, field+": true") || strings.Contains(content, field+":true") {
			errs = append(errs, fmt.Sprintf("%s: dangerous field %q set to true", path, field))
		}
	}

	return errs
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
