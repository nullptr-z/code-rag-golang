package cmd

import (
	"github.com/spf13/cobra"
)

var (
	DbPath string
)

// RegisterCommands adds all subcommands to the root command
func RegisterCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(analyzeCmd())
	rootCmd.AddCommand(upstreamCmd())
	rootCmd.AddCommand(downstreamCmd())
	rootCmd.AddCommand(impactCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(exportCmd())
	rootCmd.AddCommand(mcpCmd())
	rootCmd.AddCommand(watchCmd())
	rootCmd.AddCommand(viewCmd())
	rootCmd.AddCommand(implementsCmd())
	rootCmd.AddCommand(riskCmd())
}
