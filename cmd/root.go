package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jc",
	Short: "Jah and Co Development Platform CLI",
	Long:  `jc is the developer CLI for the Jah and Co Development Platform.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Root-level options/flags can be added here if needed
}
