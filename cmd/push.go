package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push <slug>",
	Short: "Push task + test images to registry",
	Args:  cobra.ExactArgs(1),
	Run:   runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) {
	slug := args[0]

	if cl.UserID == "" {
		fmt.Fprintln(os.Stderr, "Error: KAGENTO_USER_ID is required for push")
		os.Exit(1)
	}

	registry := cl.Registry
	base := fmt.Sprintf("%s/public/%s/%s", registry, cl.UserID, slug)

	fmt.Printf("Pushing task images for '%s'...\n", slug)

	// Check images exist
	for _, tag := range []string{"task", "test"} {
		image := fmt.Sprintf("%s:%s", slug, tag)
		if err := exec.Command("docker", "image", "inspect", image).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Image '%s' not found. Build it first.\n", image)
			os.Exit(1)
		}
	}

	// Tag and push
	for _, tag := range []string{"task", "test"} {
		src := fmt.Sprintf("%s:%s", slug, tag)
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

	fmt.Println()
	fmt.Println("Pushed:")
	fmt.Printf("   %s:task\n", base)
	fmt.Printf("   %s:test\n", base)
	fmt.Println()
	fmt.Println("Create task on the platform with these image refs.")
}

func dockerRun(args ...string) error {
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
