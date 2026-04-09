package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	imageSizeLimit = 1 << 30 // 1 GB hard limit
)

var taskBuildCmd = &cobra.Command{
	Use:   "build [path]",
	Short: "Build Docker images locally (for local testing only)",
	Long: `Builds user and test Docker images locally. These images are used
by 'kagento task test' and 'kagento task run' for local development.
To submit for server-side building, use 'kagento task submit' instead.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runTaskBuild,
}

func init() {
	taskCmd.AddCommand(taskBuildCmd)
}

func runTaskBuild(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	cfg, err := loadAndValidateTask(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Building images...")
	if err := buildTaskImages(dir, cfg.Slug); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := checkImageSizes(dir, cfg.Slug); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Images built successfully.")
}

// checkImageSizes inspects built images and warns/errors based on size.
func checkImageSizes(dir, slug string) error {
	fmt.Println("Checking image sizes...")
	var hasError bool

	imageSpecs := []struct {
		tag  string
		path string
	}{
		{tag: "task", path: filepath.Join(dir, "user", "Dockerfile")},
		{tag: "test", path: filepath.Join(dir, "test", "Dockerfile")},
	}
	inspected := 0
	for _, spec := range imageSpecs {
		if !fileExists(spec.path) {
			continue
		}
		inspected++
		image := fmt.Sprintf("%s:%s", slug, spec.tag)
		out, err := exec.Command("docker", "image", "inspect", "--format", "{{.Size}}", image).Output()
		if err != nil {
			return fmt.Errorf("inspect %s: %w", image, err)
		}

		sizeStr := strings.TrimSpace(string(out))
		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			return fmt.Errorf("parse size for %s: %w", image, err)
		}

		sizeMB := float64(size) / (1024 * 1024)
		if size >= imageSizeLimit {
			fmt.Fprintf(os.Stderr, "  ERROR: %s is %.0f MB (exceeds 1 GB limit)\n", image, sizeMB)
			hasError = true
		} else {
			fmt.Printf("  %s: %.0f MB\n", image, sizeMB)
		}
	}
	if inspected == 0 {
		return fmt.Errorf("no buildable images found in %s", dir)
	}

	if hasError {
		return fmt.Errorf("one or more images exceed the 1 GB size limit")
	}
	return nil
}

// ensureBuiltImages makes sure Docker images are available locally.
func ensureBuiltImages(dir string, slug string) error {
	state := detectState(dir)
	if state == stateRaw {
		fmt.Println("Building images from Dockerfiles...")
		return buildTaskImages(dir, slug)
	}
	return fmt.Errorf("cannot determine task state in %s (need user/Dockerfile+test/Dockerfile)", dir)
}

func buildTaskImages(dir, slug string) error {
	built := 0
	for _, spec := range []struct {
		context string
		tag     string
	}{
		{"user", slug + ":task"},
		{"test", slug + ":test"},
	} {
		ctx := filepath.Join(dir, spec.context)
		dockerfile := filepath.Join(ctx, "Dockerfile")
		if !fileExists(dockerfile) {
			continue
		}
		built++
		fmt.Printf("  Building %s from %s/Dockerfile...\n", spec.tag, spec.context)
		if err := dockerRun("build", "-t", spec.tag, ctx); err != nil {
			return fmt.Errorf("build %s failed (see docker output above): %w", spec.tag, err)
		}
	}
	if built == 0 {
		return fmt.Errorf("no Dockerfiles found to build in %s", dir)
	}
	return nil
}
