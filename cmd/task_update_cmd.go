package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	taskUpdateFrom             string
	taskUpdateTitle            string
	taskUpdateShortDesc        string
	taskUpdateDescription      string
	taskUpdateDifficulty       string
	taskUpdateStatus           string
	taskUpdateSize             string
	taskUpdateTimeLimitSec     int
	taskUpdateTaskInstructions string
	taskUpdateScoringType      string
	taskUpdateCategory         string
	taskUpdateJSON             bool
)

var taskUpdateCmd = &cobra.Command{
	Use:   "update <slug-or-path>",
	Short: "Update task metadata without republishing images",
	Args:  cobra.ExactArgs(1),
	Run:   runTaskUpdate,
}

func init() {
	taskUpdateCmd.Flags().StringVar(&taskUpdateFrom, "from", "", "Load metadata fields from a task directory")
	taskUpdateCmd.Flags().StringVar(&taskUpdateTitle, "title", "", "Override title")
	taskUpdateCmd.Flags().StringVar(&taskUpdateShortDesc, "short-desc", "", "Override short description")
	taskUpdateCmd.Flags().StringVar(&taskUpdateDescription, "description", "", "Override public description")
	taskUpdateCmd.Flags().StringVar(&taskUpdateDifficulty, "difficulty", "", "Override difficulty")
	taskUpdateCmd.Flags().StringVar(&taskUpdateStatus, "status", "", "Set status (draft, pending, or published)")
	taskUpdateCmd.Flags().StringVar(&taskUpdateSize, "size", "", "Override container size")
	taskUpdateCmd.Flags().IntVar(&taskUpdateTimeLimitSec, "time-limit-sec", 0, "Override time limit seconds")
	taskUpdateCmd.Flags().StringVar(&taskUpdateTaskInstructions, "task-instructions", "", "Override private task instructions")
	taskUpdateCmd.Flags().StringVar(&taskUpdateScoringType, "scoring-type", "", "Override scoring type")
	taskUpdateCmd.Flags().StringVar(&taskUpdateCategory, "category", "", "Override category")
	taskUpdateCmd.Flags().BoolVar(&taskUpdateJSON, "json", false, "Output JSON")
	taskCmd.AddCommand(taskUpdateCmd)
}

func runTaskUpdate(cmd *cobra.Command, args []string) {
	arg := args[0]
	dir, slug, err := taskDirSlug(arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fromDir := taskUpdateFrom
	if fromDir == "" && dir != "" {
		fromDir = dir
	}

	payload := map[string]any{}
	if fromDir != "" {
		cfg, err := loadTaskConfig(fromDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", fromDir, err)
			os.Exit(1)
		}
		slug = cfg.Slug
		payload["title"] = cfg.Title
		payload["short_desc"] = cfg.ShortDesc
		payload["description"] = cfg.Description
		payload["task_instructions"] = cfg.TaskInstructions
		payload["difficulty"] = cfg.Difficulty
		payload["size"] = cfg.ContainerSize
		payload["time_limit_sec"] = cfg.TimeLimitSec
		payload["scoring_type"] = cfg.ScoringType
		payload["scoring_config"] = cfg.ScoringConfig
		payload["category"] = cfg.Category
	}

	overrideString := func(key, value string) {
		if value != "" {
			payload[key] = value
		}
	}

	overrideString("title", taskUpdateTitle)
	overrideString("short_desc", taskUpdateShortDesc)
	overrideString("description", taskUpdateDescription)
	overrideString("difficulty", taskUpdateDifficulty)
	overrideString("status", taskUpdateStatus)
	overrideString("size", taskUpdateSize)
	overrideString("task_instructions", taskUpdateTaskInstructions)
	overrideString("scoring_type", taskUpdateScoringType)
	overrideString("category", taskUpdateCategory)
	if taskUpdateTimeLimitSec > 0 {
		payload["time_limit_sec"] = taskUpdateTimeLimitSec
	}

	if len(payload) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no update fields provided")
		os.Exit(1)
	}

	task, err := cl.UpdateTask(slug, payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating task: %v\n", err)
		os.Exit(1)
	}

	if taskUpdateJSON {
		printJSON(task)
		return
	}

	fmt.Printf("Updated task %s\n", slug)
	fmt.Printf("  Title:       %s\n", task.Title)
	fmt.Printf("  Status:      %s\n", task.Status)
	if task.Difficulty != "" {
		fmt.Printf("  Difficulty:  %s\n", task.Difficulty)
	}
	if task.Size != "" {
		fmt.Printf("  Size:        %s\n", task.Size)
	}
	if task.TimeLimitSec > 0 {
		fmt.Printf("  Time limit:  %ss\n", strconv.Itoa(task.TimeLimitSec))
	}
}
