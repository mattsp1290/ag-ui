package commands

import (
	"github.com/spf13/cobra"
)

var (
	// Global flags
	outputFormat string
	noColor      bool
	quiet        bool
	verbose      bool
	sessionID    string
)

// Version information for the CLI
var (
	Version = "0.1.0"
	Commit  = "dev"
	Date    = "unknown"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:     "ag-ui-cli",
	Short:   "AG-UI CLI - Tool-Based Generative UI Client",
	Version: Version,
	Long: `AG-UI CLI (Fang) is a command-line interface for interacting with
the AG-UI server, providing chat functionality with streaming responses,
tool call handling, and session management.

This CLI uses the Charmbracelet Fang framework for enhanced terminal UX.`,
}

// Removed Execute() function - Fang handles execution in main.go

func init() {
	// Global flags
	RootCmd.PersistentFlags().StringVar(&outputFormat, "output", "pretty", "Output format (pretty|json)")
	RootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	RootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-essential output")
	RootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	RootCmd.PersistentFlags().StringVar(&sessionID, "session", "", "Session ID (uses last session if not specified)")
}