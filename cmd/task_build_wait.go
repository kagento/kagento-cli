package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kagento/kagento-cli/internal/client"
)

type buildStatusGetter interface {
	GetBuild(buildID string) (*client.BuildStatus, error)
}

func waitForBuildCompletion(getter buildStatusGetter, buildID string, pollInterval time.Duration, stdout, stderr io.Writer) (*client.BuildStatus, error) {
	lastStatus := ""
	reportedTransientError := ""

	for {
		build, err := getter.GetBuild(buildID)
		if err != nil {
			if isTransientBuildStatusError(err) {
				msg := err.Error()
				if msg != reportedTransientError {
					fmt.Fprintf(stderr, "  Build status check failed temporarily: %v\n", err)
					fmt.Fprintln(stderr, "  Retrying...")
					reportedTransientError = msg
				}
				time.Sleep(pollInterval)
				continue
			}
			return nil, err
		}

		reportedTransientError = ""
		if build.Status != lastStatus {
			fmt.Fprintf(stdout, "  Status: %s\n", build.Status)
			lastStatus = build.Status
		}

		switch build.Status {
		case "completed", "failed":
			return build, nil
		}

		time.Sleep(pollInterval)
	}
}

func isTransientBuildStatusError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	for _, fragment := range []string{
		"HTTP 502",
		"HTTP 503",
		"HTTP 504",
		"request failed:",
		"context deadline exceeded",
		"timeout awaiting response headers",
		"connection reset by peer",
		"EOF",
	} {
		if strings.Contains(msg, fragment) {
			return true
		}
	}

	return false
}

func resolvePublishTaskConfig(buildID string, buildSlug string, explicitDir string) (string, *TaskConfig, bool, error) {
	if explicitDir != "" {
		cfg, err := loadTaskConfig(explicitDir)
		if err != nil {
			return "", nil, false, err
		}
		if cfg.Slug != buildSlug {
			return "", nil, false, fmt.Errorf("task metadata slug %q does not match build slug %q", cfg.Slug, buildSlug)
		}
		return explicitDir, cfg, false, nil
	}

	if cfg, err := loadTaskConfig("."); err == nil {
		if cfg.Slug == buildSlug {
			return ".", cfg, false, nil
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", nil, false, err
	}

	dir := discoverTaskDirBySlug(wd, buildSlug)
	if dir == "" {
		return "", nil, false, fmt.Errorf("read task.yaml: no local task metadata found for build slug %q; run from the task directory or pass an explicit path like 'kagento task publish tasks/%s --build-id=%s'", buildSlug, buildSlug, buildID)
	}

	cfg, err := loadTaskConfig(dir)
	if err != nil {
		return "", nil, false, err
	}
	return dir, cfg, true, nil
}

func discoverTaskDirBySlug(startDir, slug string) string {
	cur := startDir
	for {
		candidates := []string{
			filepath.Join(cur, "tasks", slug),
			filepath.Join(cur, slug),
		}
		for _, candidate := range candidates {
			cfg, err := loadTaskConfig(candidate)
			if err == nil && cfg.Slug == slug {
				return candidate
			}
		}

		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}
