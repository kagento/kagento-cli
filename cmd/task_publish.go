package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	draftFlag   bool
	buildIDFlag string
)

var taskPublishCmd = &cobra.Command{
	Use:   "publish [path]",
	Short: "Publish a completed build as a task on the platform",
	Long: `Promotes a completed server-side build to a published (or draft) task.
Requires a --build-id from a successful 'kagento task submit'.`,
	Run: runTaskPublish,
}

func init() {
	taskPublishCmd.Flags().BoolVar(&draftFlag, "draft", false, "Set task status to draft instead of published")
	taskPublishCmd.Flags().StringVar(&buildIDFlag, "build-id", "", "Build ID to publish (required)")
	taskPublishCmd.MarkFlagRequired("build-id")
	taskCmd.AddCommand(taskPublishCmd)
}

func runTaskPublish(cmd *cobra.Command, args []string) {
	dir := ""
	if len(args) > 0 {
		dir = args[0]
	}

	build, err := waitForBuildCompletion(cl, buildIDFlag, 3*time.Second, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking build: %v\n", err)
		os.Exit(1)
	}
	if build.Status == "failed" {
		fmt.Fprintf(os.Stderr, "Build failed: %s\n", build.Error)
		os.Exit(1)
	}

	resolvedDir, cfg, autoDiscovered, err := resolvePublishTaskConfig(buildIDFlag, build.Slug, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if autoDiscovered {
		fmt.Printf("Using task metadata from %s\n", resolvedDir)
	}

	status := "published"
	if draftFlag {
		status = "draft"
	}

	timeLimitSec := cfg.TimeLimitSec
	if timeLimitSec == 0 {
		timeLimitSec = 3600
	}
	difficulty := cfg.Difficulty
	if difficulty == "" {
		difficulty = "medium"
	}
	containerSize := cfg.ContainerSize
	if containerSize == "" {
		containerSize = "small"
	}

	scoringType := cfg.ScoringType
	if scoringType == "" {
		scoringType = "gradient"
	}
	category := cfg.Category

	fmt.Printf("Publishing build %s as '%s' (%s)...\n", buildIDFlag, cfg.Slug, status)

	body := map[string]interface{}{
		"title":             cfg.Title,
		"short_desc":        cfg.ShortDesc,
		"description":       cfg.Description,
		"task_instructions": cfg.TaskInstructions,
		"difficulty":        difficulty,
		"size":              containerSize,
		"time_limit_sec":    timeLimitSec,
		"scoring_type":      scoringType,
		"draft":             draftFlag,
	}
	if cfg.ScoringConfig != nil {
		body["scoring_config"] = cfg.ScoringConfig
	}
	if category != "" {
		body["category"] = category
	}

	result, err := cl.PublishBuild(buildIDFlag, body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error publishing: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Published task: %s\n", cfg.Slug)
	fmt.Printf("  Task ID: %s\n", result["task_id"])
	fmt.Printf("  Status:  %s\n", result["status"])
}
