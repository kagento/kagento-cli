package cmd

import (
	"fmt"
	"os"

	"github.com/kagento/kagento-cli/internal/auth"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of Kagento",
	Args:  cobra.NoArgs,
	Run:   runLogout,
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}

func runLogout(cmd *cobra.Command, args []string) {
	if err := auth.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Logged out")
}
