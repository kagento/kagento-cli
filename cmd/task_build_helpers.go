package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kagento/kagento-cli/internal/client"
)

type taskActionResult struct {
	TaskDir         string `json:"task_dir,omitempty"`
	Slug            string `json:"slug,omitempty"`
	BuildID         string `json:"build_id,omitempty"`
	BuildStatus     string `json:"build_status,omitempty"`
	TaskID          string `json:"task_id,omitempty"`
	PublishedStatus string `json:"published_status,omitempty"`
	EnvironmentType string `json:"environment_type,omitempty"`
	ReusedBuild     bool   `json:"reused_build,omitempty"`
	Error           string `json:"error,omitempty"`
}

func latestBuildForSlug(slug string) (*client.BuildStatus, error) {
	builds, err := cl.ListBuildsWithOptions(client.ListBuildsOptions{Slug: slug, Limit: 50})
	if err != nil {
		return nil, err
	}
	for _, build := range builds {
		if isReusableBuildStatus(build.Status) {
			candidate := build
			return &candidate, nil
		}
	}
	return nil, nil
}

func latestCompletedBuildForSlug(slug string) (*client.BuildStatus, error) {
	builds, err := cl.ListBuildsWithOptions(client.ListBuildsOptions{Slug: slug, Limit: 50})
	if err != nil {
		return nil, err
	}
	for _, build := range builds {
		if build.Status == "completed" {
			candidate := build
			return &candidate, nil
		}
	}
	return nil, nil
}

func isReusableBuildStatus(status string) bool {
	switch status {
	case "queued", "validating", "building", "scanning", "signing", "completed":
		return true
	default:
		return false
	}
}

func printBuildSummary(build *client.BuildStatus) {
	fmt.Printf("  Build ID: %s\n", build.ID)
	fmt.Printf("  Status:   %s\n", build.Status)
	if build.Progress != "" {
		fmt.Printf("  Progress: %s\n", build.Progress)
	}
	if build.UserImageDigest != "" {
		fmt.Printf("  User:     %s\n", build.UserImageDigest)
	}
	if build.TestImageDigest != "" {
		fmt.Printf("  Test:     %s\n", build.TestImageDigest)
	}
	if build.Error != "" {
		fmt.Printf("  Error:    %s\n", build.Error)
	}
}

func watchBuildProgress(buildID string, pollInterval time.Duration) (*client.BuildStatus, error) {
	var (
		lastStatus   string
		lastProgress string
		lastError    string
	)

	for {
		build, err := cl.GetBuild(buildID)
		if err != nil {
			if isTransientBuildStatusError(err) {
				time.Sleep(pollInterval)
				continue
			}
			return nil, err
		}

		if build.Status != lastStatus {
			fmt.Printf("status=%s\n", build.Status)
			lastStatus = build.Status
		}
		if build.Progress != "" && build.Progress != lastProgress {
			fmt.Println(build.Progress)
			lastProgress = build.Progress
		}
		if build.Error != "" && build.Error != lastError {
			fmt.Fprintln(os.Stderr, build.Error)
			lastError = build.Error
		}

		if build.Status == "completed" || build.Status == "failed" {
			return build, nil
		}
		time.Sleep(pollInterval)
	}
}

func taskDirSlug(arg string) (string, string, error) {
	if strings.TrimSpace(arg) == "" {
		return "", "", fmt.Errorf("task path or slug is required")
	}
	if info, err := os.Stat(arg); err == nil && info.IsDir() {
		cfg, err := loadTaskConfig(arg)
		if err != nil {
			return "", "", err
		}
		return arg, cfg.Slug, nil
	}
	return "", arg, nil
}

func printTaskBatchResults(verb string, results []taskActionResult) {
	successes := 0
	for _, result := range results {
		target := result.Slug
		if target == "" {
			target = result.TaskDir
		}
		if result.Error != "" {
			fmt.Printf("%s %s: ERROR: %s\n", verb, target, result.Error)
			continue
		}

		successes++
		parts := []string{}
		if result.BuildID != "" {
			parts = append(parts, "build="+result.BuildID)
		}
		if result.BuildStatus != "" {
			parts = append(parts, "status="+result.BuildStatus)
		}
		if result.ReusedBuild {
			parts = append(parts, "reused=true")
		}
		if result.TaskID != "" {
			parts = append(parts, "task_id="+result.TaskID)
		}
		if result.PublishedStatus != "" {
			parts = append(parts, "published="+result.PublishedStatus)
		}
		if len(parts) > 0 {
			fmt.Printf("%s %s: %s\n", verb, target, strings.Join(parts, " "))
		} else {
			fmt.Printf("%s %s\n", verb, target)
		}
	}

	fmt.Println()
	fmt.Printf("Completed %d/%d tasks successfully.\n", successes, len(results))
}

func taskBatchHasErrors(results []taskActionResult) bool {
	for _, result := range results {
		if result.Error != "" {
			return true
		}
	}
	return false
}
