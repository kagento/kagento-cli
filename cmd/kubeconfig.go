package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var kubeconfigOutput string

var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig <session-name or id>",
	Short: "Download kubeconfig for a session",
	Args:  cobra.MinimumNArgs(1),
	Run:   runKubeconfig,
}

func init() {
	kubeconfigCmd.Flags().StringVarP(&kubeconfigOutput, "output", "o", "kubeconfig.yaml", "Output file path")
	rootCmd.AddCommand(kubeconfigCmd)
}

func runKubeconfig(cmd *cobra.Command, args []string) {
	nameOrID := strings.Join(args, "-")

	sessionID, err := cl.ResolveSessionID(nameOrID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	data, err := cl.GetKubeconfig(sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(kubeconfigOutput, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Kubeconfig saved to %s\n", kubeconfigOutput)
	fmt.Printf("Run: export KUBECONFIG=./%s\n", kubeconfigOutput)
}
