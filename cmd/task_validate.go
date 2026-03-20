package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// TaskConfig represents the task.yaml format.
type TaskConfig struct {
	Version       int    `yaml:"version"`
	Slug          string `yaml:"slug"`
	Title         string `yaml:"title"`
	ShortDesc     string `yaml:"short_desc"`
	Difficulty    string `yaml:"difficulty"`
	ContainerSize string `yaml:"container_size"`
	TimeLimitSec  int    `yaml:"time_limit_sec"`
}

var validDifficulties = map[string]bool{
	"easy": true, "medium": true, "hard": true, "extreme": true,
}

var validContainerSizes = map[string]bool{
	"nano": true, "micro": true, "small": true, "medium": true, "large": true,
}

var buildFlag bool

var taskValidateCmd = &cobra.Command{
	Use:   "validate [path]",
	Short: "Validate task.yaml and required files",
	Args:  cobra.MaximumNArgs(1),
	Run:   runTaskValidate,
}

func init() {
	taskValidateCmd.Flags().BoolVar(&buildFlag, "build", false, "Also build Docker images")
	taskCmd.AddCommand(taskValidateCmd)
}

func runTaskValidate(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	cfg, err := loadAndValidateTask(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Validated: %s (%s)\n", cfg.Title, cfg.Slug)

	if buildFlag {
		fmt.Println("Building images...")
		if err := buildTaskImages(dir, cfg.Slug); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Images built successfully.")
	}
}

func loadAndValidateTask(dir string) (*TaskConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, "task.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read task.yaml: %w", err)
	}

	var cfg TaskConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse task.yaml: %w", err)
	}

	// Validate required fields.
	var errs []string
	if cfg.Version != 1 {
		errs = append(errs, "version must be 1")
	}
	if cfg.Slug == "" {
		errs = append(errs, "slug is required")
	}
	if cfg.Title == "" {
		errs = append(errs, "title is required")
	}
	if cfg.ShortDesc == "" {
		errs = append(errs, "short_desc is required")
	}
	if cfg.Difficulty != "" && !validDifficulties[cfg.Difficulty] {
		errs = append(errs, fmt.Sprintf("difficulty must be one of: easy, medium, hard, extreme (got %q)", cfg.Difficulty))
	}
	if cfg.ContainerSize != "" && !validContainerSizes[cfg.ContainerSize] {
		errs = append(errs, fmt.Sprintf("container_size must be one of: nano, micro, small, medium, large (got %q)", cfg.ContainerSize))
	}
	if cfg.TimeLimitSec < 0 {
		errs = append(errs, "time_limit_sec must be non-negative")
	}

	// Check required files based on state.
	state := detectState(dir)
	switch state {
	case stateBuilt:
		for _, f := range []string{"user.tar", "test.tar"} {
			if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
				errs = append(errs, fmt.Sprintf("missing required file: %s", f))
			}
		}
	default:
		// Raw state (or unknown): check Dockerfiles.
		for _, f := range []string{"user/Dockerfile", "test/Dockerfile"} {
			if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
				errs = append(errs, fmt.Sprintf("missing required file: %s", f))
			}
		}
	}

	if len(errs) > 0 {
		msg := "validation errors:"
		for _, e := range errs {
			msg += "\n  - " + e
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return &cfg, nil
}

func buildTaskImages(dir, slug string) error {
	for _, spec := range []struct {
		context string
		tag     string
	}{
		{"user", slug + ":task"},
		{"test", slug + ":test"},
	} {
		ctx := filepath.Join(dir, spec.context)
		fmt.Printf("  Building %s from %s/Dockerfile...\n", spec.tag, spec.context)
		if err := dockerRun("build", "-t", spec.tag, ctx); err != nil {
			return fmt.Errorf("build %s failed (see docker output above): %w", spec.tag, err)
		}
	}
	return nil
}
