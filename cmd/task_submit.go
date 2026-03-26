package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/kagento/kagento-cli/internal/client"
	"github.com/spf13/cobra"
)

var taskSubmitCmd = &cobra.Command{
	Use:   "submit [path]",
	Short: "Submit a task for server-side building",
	Long: `Validates the task directory, creates a tar archive, uploads it to the
platform, and triggers a server-side build. Container tasks go through the full
build pipeline (Kaniko, SBOM, scanning, signing). vcluster tasks are published
directly from task.yaml metadata.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runTaskSubmit,
}

func init() {
	taskCmd.AddCommand(taskSubmitCmd)
}

func runTaskSubmit(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	// Step 1: Validate.
	cfg, err := loadAndValidateTask(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Task: %s (%s)\n", cfg.Title, cfg.Slug)

	// vcluster tasks don't need a server-side build — publish directly.
	if cfg.EnvironmentType == "vcluster" {
		runVclusterSubmit(cfg, dir)
		return
	}

	// Step 2: Create tar.gz of raw task directory.
	fmt.Println("Creating source archive...")
	tarPath, err := createSourceTar(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating archive: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(tarPath)

	tarInfo, err := os.Stat(tarPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Archive size: %.1f KB\n", float64(tarInfo.Size())/1024)

	// Step 3: Get presigned upload URL.
	fmt.Println("Requesting upload URL...")
	presign, err := cl.PresignBuildUpload()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting upload URL: %v\n", err)
		os.Exit(1)
	}

	// Step 4: Upload.
	fmt.Println("Uploading source archive...")
	tarFile, err := os.Open(tarPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer tarFile.Close()

	if err := client.UploadToPresignedURL(presign.UploadURL, tarFile, tarInfo.Size()); err != nil {
		fmt.Fprintf(os.Stderr, "Error uploading: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Upload complete.")

	// Step 5: Start build.
	fmt.Println("Starting build...")
	buildID, err := cl.StartBuild(cfg.Slug, presign.SourceKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting build: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Build ID: %s\n", buildID)

	// Step 6: Poll for completion.
	fmt.Println("Waiting for build to complete...")
	build, err := waitForBuildCompletion(cl, buildID, 3*time.Second, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking build: %v\n", err)
		fmt.Fprintf(os.Stderr, "Build may still be running. Retry later with: kagento task publish %s --build-id=%s\n", dir, buildID)
		os.Exit(1)
	}

	switch build.Status {
	case "completed":
		fmt.Println()
		fmt.Println("Build completed successfully!")
		fmt.Printf("  User image: %s\n", build.UserImageDigest)
		fmt.Printf("  Test image: %s\n", build.TestImageDigest)
		fmt.Printf("  Signed:     %v\n", build.Signed)
		fmt.Println()
		fmt.Printf("To publish: kagento task publish %s --build-id=%s\n", dir, buildID)
		return
	case "failed":
		fmt.Fprintf(os.Stderr, "\nBuild failed: %s\n", build.Error)
		os.Exit(1)
	}
}

func runVclusterSubmit(cfg *TaskConfig, dir string) {
	// Read provision manifest files and encode as JSON.
	var manifests []json.RawMessage
	for _, manifestPath := range cfg.Provision.Manifests {
		fullPath := filepath.Join(dir, manifestPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading manifest %s: %v\n", manifestPath, err)
			os.Exit(1)
		}
		// Wrap YAML content as a JSON string.
		encoded, _ := json.Marshal(string(data))
		manifests = append(manifests, encoded)
	}

	// Encode checks as JSON.
	var checks []json.RawMessage
	for _, check := range cfg.Checks {
		encoded, _ := json.Marshal(check)
		checks = append(checks, encoded)
	}

	scoringType := cfg.ScoringType
	if scoringType == "" {
		scoringType = "gradient"
	}
	difficulty := cfg.Difficulty
	if difficulty == "" {
		difficulty = "medium"
	}
	containerSize := cfg.ContainerSize
	if containerSize == "" {
		containerSize = "small"
	}
	timeLimitSec := cfg.TimeLimitSec
	if timeLimitSec == 0 {
		timeLimitSec = 3600
	}

	body := map[string]interface{}{
		"slug":                cfg.Slug,
		"title":               cfg.Title,
		"short_desc":          cfg.ShortDesc,
		"description":         cfg.Description,
		"task_instructions":   cfg.TaskInstructions,
		"difficulty":          difficulty,
		"size":                containerSize,
		"time_limit_sec":      timeLimitSec,
		"scoring_type":        scoringType,
		"provision_manifests": manifests,
		"checks":              checks,
	}
	if cfg.ScoringConfig != nil {
		body["scoring_config"] = cfg.ScoringConfig
	}
	if cfg.Category != "" {
		body["category"] = cfg.Category
	}

	fmt.Println("Publishing vcluster task...")
	result, err := cl.PublishK8sTask(body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error publishing: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Published task: %s\n", cfg.Slug)
	fmt.Printf("  Task ID: %s\n", result["task_id"])
	fmt.Printf("  Status:  %s\n", result["status"])
}

// createSourceTar creates a .tar.gz of the raw task directory.
// Includes: task.yaml, user/*, test/*, solution/*
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

	// Walk the directory and add relevant files.
	dirs := []string{"user", "test", "solution"}
	files := []string{"task.yaml"}

	// Add top-level files.
	for _, f := range files {
		srcPath := filepath.Join(dir, f)
		if err := addFileToTar(tw, srcPath, f); err != nil {
			os.Remove(tmpFile.Name())
			return "", fmt.Errorf("add %s: %w", f, err)
		}
	}

	// Add subdirectories.
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
			return "", fmt.Errorf("add %s/: %w", d, err)
		}
	}

	return tmpFile.Name(), nil
}

func addFileToTar(tw *tar.Writer, srcPath, name string) error {
	fi, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name: name,
		Mode: int64(fi.Mode()),
		Size: fi.Size(),
	}

	if err := tw.WriteHeader(hdr); err != nil {
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
