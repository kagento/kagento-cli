package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	registryTool         string
	registryPrintCommand bool
	registryPrintToken   bool
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Work with Kagento registry credentials for live sessions",
}

var registryLoginCmd = &cobra.Command{
	Use:   "login <session-name or id>",
	Short: "Log in to the Kagento session registry for a live session",
	Args:  cobra.MinimumNArgs(1),
	Run:   runRegistryLogin,
}

func init() {
	registryLoginCmd.Flags().StringVar(&registryTool, "tool", "docker", "Container tool to use (docker or podman)")
	registryLoginCmd.Flags().BoolVar(&registryPrintCommand, "print-command", false, "Print the login command instead of executing it")
	registryLoginCmd.Flags().BoolVar(&registryPrintToken, "print-token", false, "Print only the short-lived registry token")
	registryCmd.AddCommand(registryLoginCmd)
	rootCmd.AddCommand(registryCmd)
}

func runRegistryLogin(cmd *cobra.Command, args []string) {
	nameOrID := strings.Join(args, "-")

	sessionID, err := cl.ResolveSessionID(nameOrID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	creds, err := cl.GetSessionRegistryCredentials(sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	loginCommand := fmt.Sprintf(
		"printf '%%s\\n' '%s' | %s login %s -u '%s' --password-stdin",
		strings.ReplaceAll(creds.Password, "'", `'"'"'`),
		registryTool,
		creds.Registry,
		strings.ReplaceAll(creds.Username, "'", `'"'"'`),
	)

	if registryPrintToken {
		fmt.Println(creds.Password)
		return
	}
	if registryPrintCommand {
		fmt.Println(loginCommand)
		fmt.Printf("# Example push: %s build -t %s/sessions/%s/app:dev . && %s push %s/sessions/%s/app:dev\n",
			registryTool,
			creds.Registry,
			sessionID,
			registryTool,
			creds.Registry,
			sessionID,
		)
		return
	}

	login := exec.Command(registryTool, "login", creds.Registry, "-u", creds.Username, "--password-stdin")
	login.Stdin = strings.NewReader(creds.Password + "\n")
	login.Stdout = os.Stdout
	login.Stderr = os.Stderr
	if err := login.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s login failed: %v\n", registryTool, err)
		os.Exit(1)
	}

	fmt.Printf("Logged in to %s as %s\n", creds.Registry, creds.Username)
	fmt.Printf("Example build: %s build -t %s/sessions/%s/app:dev .\n", registryTool, creds.Registry, sessionID)
	fmt.Printf("Example push:  %s push %s/sessions/%s/app:dev\n", registryTool, creds.Registry, sessionID)
}
