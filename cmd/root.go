package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overwritten at build time via -ldflags "-X jahandco/cli/cmd.version=...";
// see .goreleaser.yaml's builds.ldflags. Left as "dev" for `go run`/`go install`
// and any other build that doesn't pass that flag.
var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "jc",
	Short:   "Jah and Co Development Platform CLI",
	Long:    `jc is the developer CLI for the Jah and Co Development Platform.`,
	Version: version,
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
