package cmd

import (
	"fmt"
	"os"

	"github.com/kagento/kagento-cli/internal/client"
	"github.com/spf13/cobra"
)

var (
	taskGetMine bool
	taskGetJSON bool
)

var taskGetCmd = &cobra.Command{
	Use:   "get <slug>",
	Short: "Show task metadata",
	Args:  cobra.ExactArgs(1),
	Run:   runTaskGet,
}

func init() {
	taskGetCmd.Flags().BoolVar(&taskGetMine, "mine", false, "Read through the authored-task API (required for drafts/private metadata)")
	taskGetCmd.Flags().BoolVar(&taskGetJSON, "json", false, "Output JSON")
	taskCmd.AddCommand(taskGetCmd)
}

func runTaskGet(cmd *cobra.Command, args []string) {
	slug := args[0]
	var (
		task *client.TaskRecord
		err  error
	)
	if taskGetMine {
		task, err = cl.GetOwnTask(slug)
	} else {
		task, err = cl.GetTask(slug)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if taskGetJSON {
		printJSON(task)
		return
	}

	fmt.Printf("Slug:            %s\n", task.Slug)
	fmt.Printf("Title:           %s\n", task.Title)
	fmt.Printf("Short Desc:      %s\n", task.ShortDesc)
	if task.Status != "" {
		fmt.Printf("Status:          %s\n", task.Status)
	}
	if task.Difficulty != "" {
		fmt.Printf("Difficulty:      %s\n", task.Difficulty)
	}
	if task.Size != "" {
		fmt.Printf("Size:            %s\n", task.Size)
	}
	if task.EnvironmentType != "" {
		fmt.Printf("Environment:     %s\n", task.EnvironmentType)
	}
	if task.TimeLimitSec > 0 {
		fmt.Printf("Time Limit:      %ds\n", task.TimeLimitSec)
	}
	if task.Category != "" {
		fmt.Printf("Category:        %s\n", task.Category)
	}
	if task.ScoringType != "" {
		fmt.Printf("Scoring Type:    %s\n", task.ScoringType)
	}
	if task.ReviewFeedback != "" {
		fmt.Printf("Review Feedback: %s\n", task.ReviewFeedback)
	}
	if task.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", task.Description)
	}
	if task.TaskInstructions != "" {
		fmt.Printf("\nTask Instructions:\n%s\n", task.TaskInstructions)
	}
}
