package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time via ldflags
	Version = "dev"
	// Commit is set at build time via ldflags
	Commit = "none"
	// Date is set at build time via ldflags
	Date = "unknown"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "hlw",
	Short: "Launch AI coding agents via configurable harness endpoints",
	Long: `hlw is a CLI tool that launches AI coding agents
(like Claude, Codex, OpenCode) via configurable harness endpoints.

It reads configuration from ~/.config/hlw/config.json,
fetches available models from the endpoint, and launches the selected agent
with the appropriate environment variables.`,
	// Errors are reported once by Execute; cobra should not also print them
	// alongside the usage text on a runtime failure.
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(
		fmt.Sprintf("hlw %s (commit %s, built %s)\n", Version, Commit, Date),
	)
}
