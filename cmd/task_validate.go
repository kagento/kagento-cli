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
	Version          int                    `yaml:"version"`
	Slug             string                 `yaml:"slug"`
	Title            string                 `yaml:"title"`
	ShortDesc        string                 `yaml:"short_desc"`
	Description      string                 `yaml:"description"`
	TaskInstructions string                 `yaml:"task_instructions"`
	Difficulty       string                 `yaml:"difficulty"`
	ContainerSize    string                 `yaml:"size"`
	TimeLimitSec     int                    `yaml:"time_limit_sec"`
	ScoringType      string                 `yaml:"scoring_type"`
	ScoringConfig    map[string]interface{} `yaml:"scoring_config"`
	Tags             []string               `yaml:"tags"`
	EnvironmentType  string                 `yaml:"environment_type"`
	Provision        struct {
		Manifests []string `yaml:"manifests"`
	} `yaml:"provision"`
	Checks []interface{} `yaml:"checks"`
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
		if err := checkImageSizes(dir, cfg.Slug); err != nil {
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
	if err := validateTaskSlugValue(cfg.Slug); err != nil {
		errs = append(errs, err.Error())
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
		errs = append(errs, fmt.Sprintf("size must be one of: nano, micro, small, medium, large (got %q)", cfg.ContainerSize))
	}
	if cfg.TimeLimitSec < 0 {
		errs = append(errs, "time_limit_sec must be non-negative")
	}

	// Environment-specific validation.
	if cfg.EnvironmentType == "vcluster" {
		errs = append(errs, validateVclusterTask(dir, &cfg)...)
	} else {
		// Container tasks: check Dockerfiles exist.
		for _, f := range []string{"user/Dockerfile", "test/Dockerfile"} {
			if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
				errs = append(errs, fmt.Sprintf("missing required file: %s", f))
			}
		}
		// Validate Dockerfile content.
		errs = append(errs, validateContainerTask(dir)...)
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

// loadTaskConfig reads task.yaml without full validation.
func loadTaskConfig(dir string) (*TaskConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, "task.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read task.yaml: %w", err)
	}

	var cfg TaskConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse task.yaml: %w", err)
	}

	if cfg.Slug == "" {
		return nil, fmt.Errorf("slug is required in task.yaml")
	}

	return &cfg, nil
}
