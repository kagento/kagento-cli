package cmd

import (
	"fmt"
	"os"

	"github.com/kagento/kagento-cli/internal/auth"
	"github.com/kagento/kagento-cli/internal/client"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	cl      *client.Client
)

var rootCmd = &cobra.Command{
	Use:     "kagento",
	Short:   "Kagento CLI — competitive AI agent challenge platform",
	Version: version,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initClient)
}

func initClient() {
	cl = client.NewClientFromEnv()

	// If KAGENTO_TOKEN is already set, use it (backward compat).
	if cl.Token != "" {
		return
	}

	// If KAGENTO_ADMIN_SECRET is set, use it (backward compat).
	if cl.AdminSecret != "" {
		return
	}

	// Try to load saved credentials from ~/.kagento/credentials.json.
	token, err := auth.GetValidToken()
	if err == nil && token != "" {
		cl.Token = token
	}
}
