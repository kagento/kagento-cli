package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// validateContainerTask checks Dockerfiles for common issues.
func validateContainerTask(dir string) []string {
	var errs []string

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
	defer f.Close()

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
				img := parts[1]
				// Handle "FROM image AS stage"
				baseImage = img
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
	}

	if !hasFrom {
		errs = append(errs, fmt.Sprintf("%s: missing FROM instruction", prefix))
	}

	// Validate base image against allowlist.
	if baseImage != "" {
		if !isAllowedBaseImage(baseImage) {
			errs = append(errs, fmt.Sprintf("%s: base image %q not in allowed list. Allowed: %s",
				prefix, baseImage, strings.Join(allowedBaseImages, ", ")))
		}
	}

	// User image specific checks.
	if role == "user" {
		if !hasUser {
			errs = append(errs, fmt.Sprintf("%s: must have a USER instruction (non-root)", prefix))
		}
		if !hasCmd {
			errs = append(errs, fmt.Sprintf("%s: must have a CMD instruction (typically CMD [\"sleep\", \"infinity\"])", prefix))
		}
	}

	// Test image specific checks.
	if role == "test" {
		if !hasCmd {
			errs = append(errs, fmt.Sprintf("%s: must have an ENTRYPOINT or CMD instruction", prefix))
		}
	}

	// Check build context size.
	contextDir := filepath.Join(filepath.Dir(path))
	size, _ := dirSize(contextDir)
	if size > 100*1024*1024 { // 100MB
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
	// Also check public.ecr.aws mirror format.
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

// validateVclusterTask checks provision manifests and check definitions.
func validateVclusterTask(dir string, cfg *TaskConfig) []string {
	var errs []string

	// Validate provision manifests exist and are valid YAML.
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

		// Parse YAML and validate.
		issues := validateK8sManifest(data, manifestPath)
		errs = append(errs, issues...)
	}

	// Validate checks.
	if len(cfg.Checks) == 0 {
		errs = append(errs, "vcluster task must have at least one check")
	}
	for i, check := range cfg.Checks {
		issues := validateCheck(check, i)
		errs = append(errs, issues...)
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
}

func validateK8sManifest(data []byte, path string) []string {
	var errs []string

	// Parse as YAML to check structure.
	var manifest map[string]interface{}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		errs = append(errs, fmt.Sprintf("%s: invalid YAML: %v", path, err))
		return errs
	}

	// Check kind.
	kind, _ := manifest["kind"].(string)
	if kind == "" {
		errs = append(errs, fmt.Sprintf("%s: missing 'kind' field", path))
	} else if !allowedK8sKinds[kind] {
		errs = append(errs, fmt.Sprintf("%s: kind %q not allowed", path, kind))
	}

	// Check apiVersion.
	if _, ok := manifest["apiVersion"]; !ok {
		errs = append(errs, fmt.Sprintf("%s: missing 'apiVersion' field", path))
	}

	// Scan for dangerous fields.
	content := string(data)
	for _, field := range dangerousK8sFields {
		if strings.Contains(content, field+": true") || strings.Contains(content, field+":true") {
			errs = append(errs, fmt.Sprintf("%s: dangerous field %q set to true", path, field))
		}
	}

	return errs
}

var resourcePattern = regexp.MustCompile(`^[a-z]+/[a-zA-Z0-9._-]+$`)

var validCheckResources = map[string]bool{
	"pod": true, "service": true, "configmap": true, "secret": true,
	"deployment": true, "statefulset": true, "daemonset": true,
	"job": true, "cronjob": true, "ingress": true, "networkpolicy": true,
	"role": true, "rolebinding": true, "clusterrole": true, "clusterrolebinding": true,
	"hpa": true, "pvc": true, "serviceaccount": true, "namespace": true,
}

func validateCheck(raw interface{}, index int) []string {
	var errs []string
	prefix := fmt.Sprintf("check[%d]", index)

	check, ok := raw.(map[string]interface{})
	if !ok {
		return []string{fmt.Sprintf("%s: must be a map", prefix)}
	}

	// Name is required.
	name, _ := check["name"].(string)
	if name == "" {
		errs = append(errs, fmt.Sprintf("%s: 'name' is required", prefix))
	} else {
		prefix = fmt.Sprintf("check[%s]", name)
	}

	// Must have either resource+conditions or script.
	_, hasScript := check["script"]
	resource, _ := check["resource"].(string)

	if !hasScript && resource == "" {
		errs = append(errs, fmt.Sprintf("%s: must have 'resource' with 'conditions', or 'script'", prefix))
		return errs
	}

	if hasScript {
		return errs // Script checks don't need further validation.
	}

	// Validate resource format: kind/name.
	if !resourcePattern.MatchString(resource) {
		errs = append(errs, fmt.Sprintf("%s: resource must be 'kind/name' (got %q)", prefix, resource))
	} else {
		kind := strings.SplitN(resource, "/", 2)[0]
		if !validCheckResources[kind] {
			errs = append(errs, fmt.Sprintf("%s: unknown resource kind %q", prefix, kind))
		}
	}

	// Validate conditions.
	conditions, _ := check["conditions"].([]interface{})
	if len(conditions) == 0 {
		errs = append(errs, fmt.Sprintf("%s: must have at least one condition", prefix))
	}
	for j, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			errs = append(errs, fmt.Sprintf("%s.conditions[%d]: must be a map", prefix, j))
			continue
		}
		if _, ok := condMap["field"]; !ok {
			errs = append(errs, fmt.Sprintf("%s.conditions[%d]: 'field' is required", prefix, j))
		}
		_, hasEquals := condMap["equals"]
		_, hasMatches := condMap["matches"]
		_, hasNotMatches := condMap["not_matches"]
		if !hasEquals && !hasMatches && !hasNotMatches {
			errs = append(errs, fmt.Sprintf("%s.conditions[%d]: must have 'equals', 'matches', or 'not_matches'", prefix, j))
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
