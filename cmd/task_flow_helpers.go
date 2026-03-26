package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kagento/kagento-cli/internal/client"
)

type submitTaskOptions struct {
	Publish bool
	Draft   bool
	Resume  bool
	Stream  bool
}

type publishTaskOptions struct {
	BuildID string
	Draft   bool
	Latest  bool
	Stream  bool
}

func submitTaskDir(dir string, opts submitTaskOptions) taskActionResult {
	cfg, err := loadAndValidateTask(dir)
	if err != nil {
		return taskActionResult{TaskDir: dir, Error: err.Error()}
	}

	result := taskActionResult{
		TaskDir:         dir,
		Slug:            cfg.Slug,
		EnvironmentType: cfg.EnvironmentType,
	}

	if opts.Stream {
		fmt.Printf("Task: %s (%s)\n", cfg.Title, cfg.Slug)
	}

	if cfg.EnvironmentType == "vcluster" {
		taskID, status, err := publishVclusterTask(dir, cfg, opts.Draft)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.TaskID = taskID
		result.PublishedStatus = status
		return result
	}

	build, reused, err := ensureTaskBuild(dir, cfg, opts.Resume, opts.Stream)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.BuildID = build.ID
	result.BuildStatus = build.Status
	result.ReusedBuild = reused
	if build.Status == "failed" {
		if build.Error != "" {
			result.Error = build.Error
		} else {
			result.Error = "build failed"
		}
		return result
	}

	if opts.Publish {
		taskID, status, err := publishContainerBuild(build.ID, cfg, opts.Draft)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.TaskID = taskID
		result.PublishedStatus = status
	}

	return result
}

func publishTaskDir(dirOrSlug string, opts publishTaskOptions) taskActionResult {
	taskDir := ""
	slug := ""
	if strings.TrimSpace(dirOrSlug) != "" {
		var err error
		taskDir, slug, err = taskDirSlug(dirOrSlug)
		if err != nil {
			return taskActionResult{TaskDir: dirOrSlug, Error: err.Error()}
		}
	}

	buildID := opts.BuildID
	if buildID == "" {
		if !opts.Latest {
			return taskActionResult{TaskDir: taskDir, Slug: slug, Error: "--build-id or --latest is required"}
		}
		if slug == "" {
			return taskActionResult{TaskDir: taskDir, Error: "task path or slug is required with --latest"}
		}

		build, err := latestCompletedBuildForSlug(slug)
		if err != nil {
			return taskActionResult{TaskDir: taskDir, Slug: slug, Error: err.Error()}
		}
		if build == nil {
			return taskActionResult{TaskDir: taskDir, Slug: slug, Error: "no completed build found"}
		}
		buildID = build.ID
	}

	stdout, stderr := waitOutputWriters(opts.Stream)
	build, err := waitForBuildCompletion(cl, buildID, 3*time.Second, stdout, stderr)
	if err != nil {
		return taskActionResult{TaskDir: taskDir, Slug: slug, BuildID: buildID, Error: err.Error()}
	}
	if build.Status == "failed" {
		errText := build.Error
		if errText == "" {
			errText = "build failed"
		}
		return taskActionResult{
			TaskDir:     taskDir,
			Slug:        build.Slug,
			BuildID:     build.ID,
			BuildStatus: build.Status,
			Error:       errText,
		}
	}

	resolvedDir, cfg, autoDiscovered, err := resolvePublishTaskConfig(build.ID, build.Slug, taskDir)
	if err != nil {
		return taskActionResult{
			TaskDir:     taskDir,
			Slug:        build.Slug,
			BuildID:     build.ID,
			BuildStatus: build.Status,
			Error:       err.Error(),
		}
	}
	if opts.Stream && autoDiscovered {
		fmt.Printf("Using task metadata from %s\n", resolvedDir)
	}

	taskID, status, err := publishContainerBuild(build.ID, cfg, opts.Draft)
	if err != nil {
		return taskActionResult{
			TaskDir:     resolvedDir,
			Slug:        cfg.Slug,
			BuildID:     build.ID,
			BuildStatus: build.Status,
			Error:       err.Error(),
		}
	}

	return taskActionResult{
		TaskDir:         resolvedDir,
		Slug:            cfg.Slug,
		BuildID:         build.ID,
		BuildStatus:     build.Status,
		TaskID:          taskID,
		PublishedStatus: status,
	}
}

func publishVclusterTask(dir string, cfg *TaskConfig, draft bool) (string, string, error) {
	var manifests []json.RawMessage
	for _, manifestPath := range cfg.Provision.Manifests {
		fullPath := manifestPath
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(dir, manifestPath)
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", "", fmt.Errorf("read manifest %s: %w", manifestPath, err)
		}
		encoded, _ := json.Marshal(string(data))
		manifests = append(manifests, encoded)
	}

	var checks []json.RawMessage
	for _, check := range cfg.Checks {
		encoded, _ := json.Marshal(check)
		checks = append(checks, encoded)
	}

	body := map[string]any{
		"slug":                cfg.Slug,
		"title":               cfg.Title,
		"short_desc":          cfg.ShortDesc,
		"description":         cfg.Description,
		"task_instructions":   cfg.TaskInstructions,
		"difficulty":          defaultTaskDifficulty(cfg),
		"size":                defaultTaskSize(cfg),
		"time_limit_sec":      defaultTaskTimeLimit(cfg),
		"scoring_type":        defaultScoringType(cfg),
		"provision_manifests": manifests,
		"checks":              checks,
		"draft":               draft,
	}
	if cfg.ScoringConfig != nil {
		body["scoring_config"] = cfg.ScoringConfig
	}
	if cfg.Category != "" {
		body["category"] = cfg.Category
	}

	result, err := cl.PublishK8sTask(body)
	if err != nil {
		return "", "", err
	}
	return fmt.Sprint(result["task_id"]), fmt.Sprint(result["status"]), nil
}

func publishContainerBuild(buildID string, cfg *TaskConfig, draft bool) (string, string, error) {
	body := map[string]any{
		"title":             cfg.Title,
		"short_desc":        cfg.ShortDesc,
		"description":       cfg.Description,
		"task_instructions": cfg.TaskInstructions,
		"difficulty":        defaultTaskDifficulty(cfg),
		"size":              defaultTaskSize(cfg),
		"time_limit_sec":    defaultTaskTimeLimit(cfg),
		"scoring_type":      defaultScoringType(cfg),
		"draft":             draft,
	}
	if cfg.ScoringConfig != nil {
		body["scoring_config"] = cfg.ScoringConfig
	}
	if cfg.Category != "" {
		body["category"] = cfg.Category
	}

	result, err := cl.PublishBuild(buildID, body)
	if err != nil {
		return "", "", err
	}
	return fmt.Sprint(result["task_id"]), fmt.Sprint(result["status"]), nil
}

func ensureTaskBuild(dir string, cfg *TaskConfig, resume bool, stream bool) (*client.BuildStatus, bool, error) {
	if resume {
		build, err := latestBuildForSlug(cfg.Slug)
		if err != nil {
			return nil, false, err
		}
		if build != nil {
			if build.Status == "queued" || build.Status == "validating" || build.Status == "building" || build.Status == "scanning" || build.Status == "signing" {
				if stream {
					fmt.Printf("Resuming build %s for %s\n", build.ID, cfg.Slug)
				}
				stdout, stderr := waitOutputWriters(stream)
				waitedBuild, err := waitForBuildCompletion(cl, build.ID, 3*time.Second, stdout, stderr)
				return waitedBuild, true, err
			}
			return build, true, nil
		}
	}

	build, err := startTaskBuild(dir, cfg, stream)
	return build, false, err
}

func startTaskBuild(dir string, cfg *TaskConfig, stream bool) (*client.BuildStatus, error) {
	if stream {
		fmt.Println("Creating source archive...")
	}
	tarPath, err := createSourceTar(dir)
	if err != nil {
		return nil, fmt.Errorf("create archive: %w", err)
	}
	defer os.Remove(tarPath)

	tarInfo, err := os.Stat(tarPath)
	if err != nil {
		return nil, err
	}
	if stream {
		fmt.Printf("  Archive size: %.1f KB\n", float64(tarInfo.Size())/1024)
		fmt.Println("Requesting upload URL...")
	}
	presign, err := cl.PresignBuildUpload()
	if err != nil {
		return nil, err
	}

	if stream {
		fmt.Println("Uploading source archive...")
	}
	tarFile, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer tarFile.Close()

	if err := client.UploadToPresignedURL(presign.UploadURL, tarFile, tarInfo.Size()); err != nil {
		return nil, err
	}
	if stream {
		fmt.Println("  Upload complete.")
		fmt.Println("Starting build...")
	}
	buildID, err := cl.StartBuild(cfg.Slug, presign.SourceKey)
	if err != nil {
		return nil, err
	}
	if stream {
		fmt.Printf("  Build ID: %s\n", buildID)
		fmt.Println("Waiting for build to complete...")
	}

	stdout, stderr := waitOutputWriters(stream)
	return waitForBuildCompletion(cl, buildID, 3*time.Second, stdout, stderr)
}

func waitOutputWriters(stream bool) (io.Writer, io.Writer) {
	if stream {
		return os.Stdout, os.Stderr
	}
	return io.Discard, io.Discard
}

func defaultTaskDifficulty(cfg *TaskConfig) string {
	if cfg.Difficulty != "" {
		return cfg.Difficulty
	}
	return "medium"
}

func defaultTaskSize(cfg *TaskConfig) string {
	if cfg.ContainerSize != "" {
		return cfg.ContainerSize
	}
	return "small"
}

func defaultTaskTimeLimit(cfg *TaskConfig) int {
	if cfg.TimeLimitSec > 0 {
		return cfg.TimeLimitSec
	}
	return 3600
}

func defaultScoringType(cfg *TaskConfig) string {
	if cfg.ScoringType != "" {
		return cfg.ScoringType
	}
	return "gradient"
}

func createSourceTar(dir string) (string, error) {
	tmpFile, err := os.CreateTemp("", "kagento-submit-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	gw := gzip.NewWriter(tmpFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	dirs := []string{"user", "test", "solution"}
	files := []string{"task.yaml"}

	for _, f := range files {
		srcPath := filepath.Join(dir, f)
		if err := addFileToTar(tw, srcPath, f); err != nil {
			os.Remove(tmpFile.Name())
			return "", fmt.Errorf("add %s: %w", f, err)
		}
	}

	for _, d := range dirs {
		subDir := filepath.Join(dir, d)
		if _, err := os.Stat(subDir); os.IsNotExist(err) {
			continue
		}
		err := filepath.Walk(subDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}

			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = relPath

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(tw, f)
			return err
		})
		if err != nil {
			os.Remove(tmpFile.Name())
			return "", err
		}
	}

	return tmpFile.Name(), nil
}

func addFileToTar(tw *tar.Writer, srcPath, tarName string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = tarName
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}

func discoverTaskDirs(root string) ([]string, error) {
	if root == "" {
		root = "."
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	if fileExists(filepath.Join(root, "task.yaml")) {
		return []string{root}, nil
	}

	var dirs []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}
		if fileExists(filepath.Join(path, "task.yaml")) {
			dirs = append(dirs, path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(dirs)
	return dirs, nil
}

func runTaskBatch(dirs []string, jobs int, fn func(string) taskActionResult) []taskActionResult {
	if len(dirs) == 0 {
		return nil
	}
	if jobs <= 0 {
		jobs = 1
	}
	if jobs > len(dirs) {
		jobs = len(dirs)
	}

	type job struct {
		index int
		dir   string
	}
	type result struct {
		index  int
		action taskActionResult
	}

	jobsCh := make(chan job)
	resultsCh := make(chan result, len(dirs))

	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobsCh {
				resultsCh <- result{
					index:  job.index,
					action: fn(job.dir),
				}
			}
		}()
	}

	go func() {
		for i, dir := range dirs {
			jobsCh <- job{index: i, dir: dir}
		}
		close(jobsCh)
		wg.Wait()
		close(resultsCh)
	}()

	results := make([]taskActionResult, len(dirs))
	for result := range resultsCh {
		results[result.index] = result.action
	}
	return results
}
