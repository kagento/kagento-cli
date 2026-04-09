package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kagento/kagento-cli/internal/client"
)

type fakeBuildGetter struct {
	responses []fakeBuildResponse
	index     int
}

type fakeBuildResponse struct {
	build *client.BuildStatus
	err   error
}

func (f *fakeBuildGetter) GetBuild(string) (*client.BuildStatus, error) {
	if f.index >= len(f.responses) {
		return nil, fmt.Errorf("unexpected extra GetBuild call")
	}
	resp := f.responses[f.index]
	f.index++
	return resp.build, resp.err
}

func TestWaitForBuildCompletionRetriesTransientErrors(t *testing.T) {
	getter := &fakeBuildGetter{
		responses: []fakeBuildResponse{
			{err: fmt.Errorf("HTTP 504: gateway timeout")},
			{build: &client.BuildStatus{ID: "b1", Slug: "demo", Status: "building"}},
			{build: &client.BuildStatus{ID: "b1", Slug: "demo", Status: "completed", UserImageDigest: "u", TestImageDigest: "t", Signed: true}},
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	build, err := waitForBuildCompletion(getter, "b1", 0*time.Second, &stdout, &stderr)
	if err != nil {
		t.Fatalf("waitForBuildCompletion returned error: %v", err)
	}
	if build.Status != "completed" {
		t.Fatalf("build status = %q, want completed", build.Status)
	}
	if !strings.Contains(stderr.String(), "HTTP 504") {
		t.Fatalf("stderr %q does not mention transient 504", stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Status: building") || !strings.Contains(got, "Status: completed") {
		t.Fatalf("stdout %q does not include expected statuses", got)
	}
}

func TestResolvePublishTaskConfigFindsTasksDirFromRepoRoot(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	taskDir := filepath.Join(repoRoot, "tasks", "demo-task")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir task dir: %v", err)
	}
	taskYAML := "version: 1\nslug: demo-task\ntitle: Demo Task\nshort_desc: Demo\n"
	if err := os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0o644); err != nil {
		t.Fatalf("write task.yaml: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir repo root: %v", err)
	}

	dir, cfg, auto, err := resolvePublishTaskConfig("build-123", "demo-task", "")
	if err != nil {
		t.Fatalf("resolvePublishTaskConfig returned error: %v", err)
	}
	if !auto {
		t.Fatalf("auto = false, want true")
	}
	if cfg.Slug != "demo-task" {
		t.Fatalf("cfg.Slug = %q, want demo-task", cfg.Slug)
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		resolvedDir = filepath.Clean(dir)
	}
	resolvedTaskDir, err := filepath.EvalSymlinks(taskDir)
	if err != nil {
		resolvedTaskDir = filepath.Clean(taskDir)
	}
	if resolvedDir != resolvedTaskDir {
		t.Fatalf("resolved dir = %q, want %q", resolvedDir, resolvedTaskDir)
	}
}

func TestIsTransientBuildStatusError(t *testing.T) {
	for _, err := range []error{
		fmt.Errorf("HTTP 502: bad gateway"),
		fmt.Errorf("HTTP 503: unavailable"),
		fmt.Errorf("HTTP 504: timeout"),
		fmt.Errorf("request failed: dial tcp timeout"),
		fmt.Errorf("context deadline exceeded"),
	} {
		if !isTransientBuildStatusError(err) {
			t.Fatalf("expected transient error for %q", err)
		}
	}

	if isTransientBuildStatusError(fmt.Errorf("HTTP 400: build not found")) {
		t.Fatal("unexpected transient classification for HTTP 400")
	}
}

func TestLatestBuildForSlugSkipsFailedBuilds(t *testing.T) {
	originalClient := cl
	originalHTTPClient := http.DefaultClient
	defer func() {
		cl = originalClient
		http.DefaultClient = originalHTTPClient
	}()

	http.DefaultClient = &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.String(), "https://backend.test/api/builds?") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, []map[string]any{
			{"id": "failed-build", "slug": "demo", "status": "failed"},
			{"id": "completed-build", "slug": "demo", "status": "completed"},
		}), nil
	})}

	cl = &client.Client{
		BackendURL:  "https://backend.test",
		Token:       "test-token",
		StaticToken: true,
	}

	build, err := latestBuildForSlug("demo")
	if err != nil {
		t.Fatalf("latestBuildForSlug() error = %v", err)
	}
	if build == nil {
		t.Fatal("latestBuildForSlug() returned nil")
	}
	if build.ID != "completed-build" {
		t.Fatalf("build.ID = %q, want completed-build", build.ID)
	}
}

func TestLatestBuildForSlugReturnsNilWhenOnlyFailedBuildsRemain(t *testing.T) {
	originalClient := cl
	originalHTTPClient := http.DefaultClient
	defer func() {
		cl = originalClient
		http.DefaultClient = originalHTTPClient
	}()

	http.DefaultClient = &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.String(), "https://backend.test/api/builds?") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		body, _ := json.Marshal([]map[string]any{
			{"id": "failed-build", "slug": "demo", "status": "failed"},
		})
		return stringHTTPResponse(http.StatusOK, string(body)), nil
	})}

	cl = &client.Client{
		BackendURL:  "https://backend.test",
		Token:       "test-token",
		StaticToken: true,
	}

	build, err := latestBuildForSlug("demo")
	if err != nil {
		t.Fatalf("latestBuildForSlug() error = %v", err)
	}
	if build != nil {
		t.Fatalf("build = %#v, want nil", build)
	}
}
