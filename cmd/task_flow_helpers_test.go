package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/kagento/kagento-cli/internal/client"
)

func TestTaskImageMirrorCopyImageUsesSkopeoWhenAvailable(t *testing.T) {
	originalLookPath := execLookPath
	originalRunSkopeo := runSkopeoCopyCommand
	originalRunDocker := runDockerCommandFunc
	defer func() {
		execLookPath = originalLookPath
		runSkopeoCopyCommand = originalRunSkopeo
		runDockerCommandFunc = originalRunDocker
	}()

	var (
		gotAuthFile string
		gotSource   string
		gotTarget   string
	)
	execLookPath = func(file string) (string, error) {
		if file != "skopeo" {
			t.Fatalf("LookPath file = %q, want %q", file, "skopeo")
		}
		return "/usr/bin/skopeo", nil
	}
	runSkopeoCopyCommand = func(authFile, source, target string) error {
		gotAuthFile = authFile
		gotSource = source
		gotTarget = target
		return nil
	}
	runDockerCommandFunc = func(string, ...string) error {
		t.Fatal("docker fallback should not be used when skopeo is available")
		return nil
	}

	mirror := &taskImageMirror{dockerConfig: t.TempDir()}
	if err := mirror.copyImage("ghcr.io/example/app:latest", "registry.kagento.io/tasks/demo/app:latest"); err != nil {
		t.Fatalf("copyImage() error = %v", err)
	}

	if gotAuthFile != filepath.Join(mirror.dockerConfig, "config.json") {
		t.Fatalf("authFile = %q", gotAuthFile)
	}
	if gotSource != "ghcr.io/example/app:latest" || gotTarget != "registry.kagento.io/tasks/demo/app:latest" {
		t.Fatalf("skopeo copy args = (%q, %q)", gotSource, gotTarget)
	}
}

func TestTaskImageMirrorCopyImageFallsBackToDockerWhenSkopeoUnavailable(t *testing.T) {
	originalLookPath := execLookPath
	originalRunSkopeo := runSkopeoCopyCommand
	originalRunDocker := runDockerCommandFunc
	originalStderr := os.Stderr
	defer func() {
		execLookPath = originalLookPath
		runSkopeoCopyCommand = originalRunSkopeo
		runDockerCommandFunc = originalRunDocker
		os.Stderr = originalStderr
	}()

	execLookPath = func(string) (string, error) {
		return "", exec.ErrNotFound
	}
	runSkopeoCopyCommand = func(string, string, string) error {
		t.Fatal("skopeo should not be used when it is unavailable")
		return nil
	}

	var got [][]string
	runDockerCommandFunc = func(_ string, args ...string) error {
		got = append(got, append([]string(nil), args...))
		return nil
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = writer

	mirror := &taskImageMirror{dockerConfig: t.TempDir()}
	if err := mirror.copyImage("ghcr.io/example/app:latest", "registry.kagento.io/tasks/demo/app:latest"); err != nil {
		t.Fatalf("copyImage() error = %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	warning, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	want := [][]string{
		{"pull", "ghcr.io/example/app:latest"},
		{"tag", "ghcr.io/example/app:latest", "registry.kagento.io/tasks/demo/app:latest"},
		{"push", "registry.kagento.io/tasks/demo/app:latest"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("docker args = %#v, want %#v", got, want)
	}
	if len(warning) == 0 {
		t.Fatal("expected fallback warning on stderr")
	}
}

func TestSubmitTaskDirBuildsAndPublishesVclusterTask(t *testing.T) {
	originalClient := cl
	originalHTTPClient := http.DefaultClient
	defer func() {
		cl = originalClient
		http.DefaultClient = originalHTTPClient
	}()

	const (
		buildID   = "11111111-1111-1111-1111-111111111111"
		testImage = "registry.kagento.io/builds/vcluster-demo@sha256:deadbeef"
	)

	var uploadedTar []byte
	var publishedBody map[string]any
	http.DefaultClient = &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost && r.URL.String() == "https://backend.test/api/builds/start":
			if err := r.ParseMultipartForm(8 << 20); err != nil {
				t.Fatalf("parse start build multipart form: %v", err)
			}
			if got := r.FormValue("slug"); got != "vcluster-demo" {
				t.Fatalf("start build slug = %q, want %q", got, "vcluster-demo")
			}
			file, _, err := r.FormFile("source")
			if err != nil {
				t.Fatalf("FormFile(source): %v", err)
			}
			defer func() {
				_ = file.Close()
			}()
			uploadedTar, err = io.ReadAll(file)
			if err != nil {
				t.Fatalf("ReadAll(source): %v", err)
			}
			return jsonHTTPResponse(http.StatusOK, map[string]string{"build_id": buildID}), nil
		case r.Method == http.MethodGet && r.URL.String() == "https://backend.test/api/builds/"+buildID:
			return jsonHTTPResponse(http.StatusOK, map[string]any{
				"id":                buildID,
				"slug":              "vcluster-demo",
				"status":            "completed",
				"test_image_digest": testImage,
			}), nil
		case r.Method == http.MethodPost && r.URL.String() == "https://backend.test/api/tasks/publish-k8s":
			if err := json.NewDecoder(r.Body).Decode(&publishedBody); err != nil {
				t.Fatalf("decode publish body: %v", err)
			}
			return jsonHTTPResponse(http.StatusOK, map[string]string{
				"task_id": "task-123",
				"status":  "draft",
			}), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})}

	cl = &client.Client{
		BackendURL:  "https://backend.test",
		Registry:    "registry.kagento.io",
		Token:       "test-token",
		StaticToken: true,
	}

	dir := t.TempDir()
	writeTaskTestFile(t, filepath.Join(dir, "task.yaml"), strings.TrimSpace(`
version: 1
slug: vcluster-demo
title: VCluster Demo
short_desc: Demo vcluster task
environment_type: vcluster
provision:
  manifests:
    - provision/configmap.yaml
`)+"\n")
	writeTaskTestFile(t, filepath.Join(dir, "provision", "configmap.yaml"), strings.TrimSpace(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
data:
  hello: world
`)+"\n")
	writeTaskTestFile(t, filepath.Join(dir, "test", "Dockerfile"), "FROM python:3.11-slim\nCMD [\"python3\", \"-V\"]\n")
	writeTaskTestFile(t, filepath.Join(dir, "solution", "solve.sh"), "#!/bin/sh\necho solved\n")

	result := submitTaskDir(dir, submitTaskOptions{Draft: true})
	if result.Error != "" {
		t.Fatalf("submitTaskDir() error = %q", result.Error)
	}
	if result.EnvironmentType != "vcluster" {
		t.Fatalf("environment_type = %q, want %q", result.EnvironmentType, "vcluster")
	}
	if result.BuildID != buildID {
		t.Fatalf("build_id = %q, want %q", result.BuildID, buildID)
	}
	if result.BuildStatus != "completed" {
		t.Fatalf("build_status = %q, want %q", result.BuildStatus, "completed")
	}
	if result.TaskID != "task-123" {
		t.Fatalf("task_id = %q, want %q", result.TaskID, "task-123")
	}
	if result.PublishedStatus != "draft" {
		t.Fatalf("published_status = %q, want %q", result.PublishedStatus, "draft")
	}

	if got := publishedBody["test_image"]; got != testImage {
		t.Fatalf("publish test_image = %#v, want %q", got, testImage)
	}
	if got := publishedBody["draft"]; got != true {
		t.Fatalf("publish draft = %#v, want true", got)
	}

	manifestPayload, ok := publishedBody["provision_manifests"].([]any)
	if !ok || len(manifestPayload) != 1 {
		t.Fatalf("provision_manifests = %#v, want one manifest", publishedBody["provision_manifests"])
	}
	if manifest, ok := manifestPayload[0].(string); !ok || !strings.Contains(manifest, "kind: ConfigMap") {
		t.Fatalf("manifest payload = %#v, want configmap yaml", manifestPayload[0])
	}

	entries := tarEntryNames(t, uploadedTar)
	for _, required := range []string{"task.yaml", "provision/configmap.yaml", "test/Dockerfile", "solution/solve.sh"} {
		if !slices.Contains(entries, required) {
			t.Fatalf("archive entries = %#v, missing %q", entries, required)
		}
	}
}

func TestSubmitTaskDirUploadsGitTaskTemplate(t *testing.T) {
	originalClient := cl
	originalHTTPClient := http.DefaultClient
	defer func() {
		cl = originalClient
		http.DefaultClient = originalHTTPClient
	}()

	const (
		buildID = "22222222-2222-2222-2222-222222222222"
	)

	var uploadedTar []byte
	http.DefaultClient = &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost && r.URL.String() == "https://backend.test/api/builds/start":
			if err := r.ParseMultipartForm(8 << 20); err != nil {
				t.Fatalf("parse start build multipart form: %v", err)
			}
			if got := r.FormValue("slug"); got != "git-demo" {
				t.Fatalf("start build slug = %q, want %q", got, "git-demo")
			}
			file, _, err := r.FormFile("source")
			if err != nil {
				t.Fatalf("FormFile(source): %v", err)
			}
			defer func() {
				_ = file.Close()
			}()
			uploadedTar, err = io.ReadAll(file)
			if err != nil {
				t.Fatalf("ReadAll(source): %v", err)
			}
			return jsonHTTPResponse(http.StatusOK, map[string]string{"build_id": buildID}), nil
		case r.Method == http.MethodGet && r.URL.String() == "https://backend.test/api/builds/"+buildID:
			return jsonHTTPResponse(http.StatusOK, map[string]any{
				"id":     buildID,
				"slug":   "git-demo",
				"status": "completed",
			}), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})}

	cl = &client.Client{
		BackendURL:  "https://backend.test",
		Registry:    "registry.kagento.io",
		Token:       "test-token",
		StaticToken: true,
	}

	dir := t.TempDir()
	writeTaskTestFile(t, filepath.Join(dir, "task.yaml"), strings.TrimSpace(`
version: 1
slug: git-demo
title: Git Demo
short_desc: Demo git task
environment_type: git
scoring_type: gradient
`)+"\n")
	writeTaskTestFile(t, filepath.Join(dir, "template", "README.md"), "# Git Demo\n")
	writeTaskTestFile(t, filepath.Join(dir, "template", "main.py"), "print('hi')\n")
	writeTaskTestFile(t, filepath.Join(dir, "test", "Dockerfile"), "FROM python:3.12-slim\nCMD [\"python3\", \"-V\"]\n")
	writeTaskTestFile(t, filepath.Join(dir, "solution", "solve.sh"), "#!/bin/sh\necho solved\n")

	result := submitTaskDir(dir, submitTaskOptions{})
	if result.Error != "" {
		t.Fatalf("submitTaskDir() error = %q", result.Error)
	}
	if result.EnvironmentType != "git" {
		t.Fatalf("environment_type = %q, want %q", result.EnvironmentType, "git")
	}
	if result.BuildID != buildID {
		t.Fatalf("build_id = %q, want %q", result.BuildID, buildID)
	}

	entries := tarEntryNames(t, uploadedTar)
	for _, required := range []string{"task.yaml", "template/README.md", "template/main.py", "test/Dockerfile", "solution/solve.sh"} {
		if !slices.Contains(entries, required) {
			t.Fatalf("archive entries = %#v, missing %q", entries, required)
		}
	}
	for _, forbidden := range []string{"user/Dockerfile"} {
		if slices.Contains(entries, forbidden) {
			t.Fatalf("archive entries = %#v, contains forbidden %q", entries, forbidden)
		}
	}
}

func TestLoadAndValidateTaskRejectsGitTaskWithUserDockerfile(t *testing.T) {
	dir := t.TempDir()
	writeTaskTestFile(t, filepath.Join(dir, "task.yaml"), strings.TrimSpace(`
version: 1
slug: git-bad
title: Git Bad
short_desc: Demo git task
environment_type: git
`)+"\n")
	writeTaskTestFile(t, filepath.Join(dir, "template", "README.md"), "# hi\n")
	writeTaskTestFile(t, filepath.Join(dir, "test", "Dockerfile"), "FROM python:3.12-slim\nCMD [\"python3\", \"-V\"]\n")
	writeTaskTestFile(t, filepath.Join(dir, "user", "Dockerfile"), "FROM python:3.12-slim\n")

	_, err := loadAndValidateTask(dir)
	if err == nil {
		t.Fatal("loadAndValidateTask() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must not contain user/Dockerfile") {
		t.Fatalf("error = %q, want user/Dockerfile rejection", err.Error())
	}
}

func TestLoadAndValidateTaskRejectsGitTaskWithEmptyTemplate(t *testing.T) {
	dir := t.TempDir()
	writeTaskTestFile(t, filepath.Join(dir, "task.yaml"), strings.TrimSpace(`
version: 1
slug: git-empty
title: Git Empty
short_desc: Demo git task
environment_type: git
`)+"\n")
	if err := os.MkdirAll(filepath.Join(dir, "template"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeTaskTestFile(t, filepath.Join(dir, "test", "Dockerfile"), "FROM python:3.12-slim\nCMD [\"python3\", \"-V\"]\n")

	_, err := loadAndValidateTask(dir)
	if err == nil {
		t.Fatal("loadAndValidateTask() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "template/ directory must contain at least one file") {
		t.Fatalf("error = %q, want empty template rejection", err.Error())
	}
}

func TestSubmitTaskDirRejectsVclusterTaskWithoutTestDockerfile(t *testing.T) {
	dir := t.TempDir()
	writeTaskTestFile(t, filepath.Join(dir, "task.yaml"), strings.TrimSpace(`
version: 1
slug: missing-test-image
title: Missing Test Image
short_desc: Demo vcluster task
environment_type: vcluster
provision:
  manifests:
    - provision/configmap.yaml
`)+"\n")
	writeTaskTestFile(t, filepath.Join(dir, "provision", "configmap.yaml"), strings.TrimSpace(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
`)+"\n")

	result := submitTaskDir(dir, submitTaskOptions{})
	if !strings.Contains(result.Error, "vcluster task scoring image must be defined by test/Dockerfile") {
		t.Fatalf("error = %q, want missing test/Dockerfile validation", result.Error)
	}
}

func writeTaskTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func tarEntryNames(t *testing.T, archive []byte) []string {
	t.Helper()

	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("gzip.NewReader(): %v", err)
	}
	defer func() {
		_ = gz.Close()
	}()

	tr := tar.NewReader(gz)
	var names []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next(): %v", err)
		}
		names = append(names, header.Name)
	}
	return names
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func jsonHTTPResponse(statusCode int, payload any) *http.Response {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	resp := stringHTTPResponse(statusCode, string(body))
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

func stringHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
