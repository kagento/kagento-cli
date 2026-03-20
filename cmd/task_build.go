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
	Short: "Build Docker images and save as user.tar + test.tar (raw → built)",
	Args:  cobra.MaximumNArgs(1),
	Run:   runTaskBuild,
}

func init() {
	taskCmd.AddCommand(taskBuildCmd)
}

func runTaskBuild(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	if err := buildToTars(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// buildToTars validates, builds images, and saves them as tars in dir.
func buildToTars(dir string) error {
	cfg, err := loadAndValidateTask(dir)
	if err != nil {
		return err
	}

	fmt.Println("Building images...")
	if err := buildTaskImages(dir, cfg.Slug); err != nil {
		return err
	}

	// Check image sizes.
	if err := checkImageSizes(cfg.Slug); err != nil {
		return err
	}

	// Save images as tars.
	for _, spec := range []struct {
		imageTag string
		tarName  string
	}{
		{cfg.Slug + ":task", "user.tar"},
		{cfg.Slug + ":test", "test.tar"},
	} {
		tarPath := filepath.Join(dir, spec.tarName)
		fmt.Printf("  Saving %s → %s\n", spec.imageTag, spec.tarName)
		if err := dockerSave(spec.imageTag, tarPath); err != nil {
			return fmt.Errorf("save %s: %w", spec.tarName, err)
		}
	}

	fmt.Println("Built state ready.")
	return nil
}

// dockerSave runs docker save -o <path> <image>.
func dockerSave(image, path string) error {
	return dockerRun("save", "-o", path, image)
}

// dockerLoad runs docker load -i <path>.
func dockerLoad(path string) error {
	return dockerRun("load", "-i", path)
}

// checkImageSizes inspects built images and warns/errors based on size.
func checkImageSizes(slug string) error {
	fmt.Println("Checking image sizes...")
	var hasError bool

	for _, tag := range []string{"task", "test"} {
		image := fmt.Sprintf("%s:%s", slug, tag)
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

	if hasError {
		return fmt.Errorf("one or more images exceed the 1 GB size limit")
	}
	return nil
}

// ensureBuiltImages makes sure Docker images are loaded for a task.
// If built state, loads from tars. If raw state, builds from Dockerfiles.
func ensureBuiltImages(dir string, slug string) error {
	state := detectState(dir)
	switch state {
	case stateBuilt:
		fmt.Println("Loading images from tars...")
		for _, tar := range []string{"user.tar", "test.tar"} {
			tarPath := filepath.Join(dir, tar)
			fmt.Printf("  Loading %s\n", tar)
			if err := dockerLoad(tarPath); err != nil {
				return fmt.Errorf("load %s: %w", tar, err)
			}
		}
		return nil
	case stateRaw:
		fmt.Println("Building images from Dockerfiles...")
		return buildTaskImages(dir, slug)
	default:
		return fmt.Errorf("cannot determine task state in %s (need user/Dockerfile+test/Dockerfile or user.tar+test.tar)", dir)
	}
}
