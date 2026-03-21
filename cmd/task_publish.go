package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var draftFlag bool

var taskPublishCmd = &cobra.Command{
	Use:   "publish [path]",
	Short: "Build, push, and upsert a task into the platform (creates or updates)",
	Long: `Publish a task to the platform. This command validates, builds, pushes
Docker images, and upserts the task in the database.

If the task slug already exists, it will be updated (upsert).
By default the task status is set to "published". Use --draft to keep
it as a draft.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runTaskPublish,
}

func init() {
	taskPublishCmd.Flags().BoolVar(&draftFlag, "draft", false, "Set task status to draft instead of published")
	taskCmd.AddCommand(taskPublishCmd)
}

func runTaskPublish(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	// Support portable state: extract .tar.gz first.
	if detectState(dir) == statePortable {
		extracted, err := extractPortableArchive(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error extracting archive: %v\n", err)
			os.Exit(1)
		}
		dir = extracted
		defer os.RemoveAll(extracted)
	}

	// Step 1: load or build images.
	cfg, err := loadTaskConfig(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Task: %s (%s)\n", cfg.Title, cfg.Slug)

	if err := ensureBuiltImages(dir, cfg.Slug); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Step 2: push images (reuse push.go logic).
	if cl.Registry == "" {
		fmt.Fprintln(os.Stderr, "Error: KAGENTO_REGISTRY is required")
		os.Exit(1)
	}
	if cl.UserID == "" {
		fmt.Fprintln(os.Stderr, "Error: KAGENTO_USER_ID is required for publish")
		os.Exit(1)
	}

	registry := cl.Registry
	base := fmt.Sprintf("%s/public/%s/%s", registry, cl.UserID, cfg.Slug)

	// Auto-login to registry using token from backend
	fmt.Println("Authenticating with registry...")
	token, err := cl.GetRegistryToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting registry token: %v\n", err)
		os.Exit(1)
	}
	if err := dockerLogin(registry, cl.UserID, token); err != nil {
		fmt.Fprintf(os.Stderr, "Error logging into registry: %v\n", err)
		os.Exit(1)
	}

	for _, tag := range []string{"task", "test"} {
		src := fmt.Sprintf("%s:%s", cfg.Slug, tag)
		dst := fmt.Sprintf("%s:%s", base, tag)

		if err := dockerRun("tag", src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "Error tagging %s: %v\n", src, err)
			os.Exit(1)
		}

		fmt.Printf("Pushing %s...\n", dst)
		if err := dockerRun("push", dst); err != nil {
			fmt.Fprintf(os.Stderr, "Error pushing %s: %v\n", dst, err)
			os.Exit(1)
		}
	}

	userImage := fmt.Sprintf("%s:task", base)
	testImage := fmt.Sprintf("%s:test", base)

	// Step 3: insert task into DB via GraphQL.
	mutation := `
		mutation($slug: String!, $title: String!, $short_desc: String!, $difficulty: String!, $container_size: String!, $time_limit_sec: Int!, $user_image: String!, $test_image: String!, $status: String!) {
			insert_tasks_one(
				object: {
					slug: $slug
					title: $title
					short_desc: $short_desc
					difficulty: $difficulty
					container_size: $container_size
					time_limit_sec: $time_limit_sec
					user_image: $user_image
					test_image: $test_image
					status: $status
				}
				on_conflict: {
					constraint: tasks_slug_key
					update_columns: [title, short_desc, difficulty, container_size, time_limit_sec, user_image, test_image, status]
				}
			) {
				id
				slug
				status
			}
		}`

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

	status := "published"
	if draftFlag {
		status = "draft"
	}

	data, err := cl.Query(mutation, map[string]interface{}{
		"slug":           cfg.Slug,
		"title":          cfg.Title,
		"short_desc":     cfg.ShortDesc,
		"difficulty":     difficulty,
		"container_size": containerSize,
		"time_limit_sec": timeLimitSec,
		"user_image":     userImage,
		"test_image":     testImage,
		"status":         status,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error inserting task: %v\n", err)
		os.Exit(1)
	}

	task, ok := data["insert_tasks_one"].(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Error: unexpected response format")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Published task: %s\n", task["slug"])
	fmt.Printf("  ID:     %s\n", task["id"])
	fmt.Printf("  Status: %s\n", task["status"])
	fmt.Printf("  Images: %s:task, %s:test\n", base, base)
}
