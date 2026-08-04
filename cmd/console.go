package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var projectArg string

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Open the developer web console",
	RunE: func(cmd *cobra.Command, args []string) error {
		consoleURL := os.Getenv("JAHANDCO_CONSOLE_URL")
		if consoleURL == "" {
			if projectArg != "" {
				consoleURL = fmt.Sprintf("https://platforms.jahandco.dev/projects/%s/console", projectArg)
			} else {
				consoleURL = "https://platforms.jahandco.dev/dashboard"
			}
		}

		fmt.Printf("[jc] opening developer console: %s\n", consoleURL)

		opened := openInBrowser(consoleURL)
		if !opened {
			fmt.Printf("[jc] couldn't open a browser automatically -- open this URL yourself:\n\n  %s\n\n", consoleURL)
		}
		return nil
	},
}

func openInBrowser(url string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default: // linux, etc.
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start() == nil
}

func init() {
	consoleCmd.Flags().StringVarP(&projectArg, "project", "p", "", "Project name to highlights")
	rootCmd.AddCommand(consoleCmd)
}
