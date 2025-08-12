package commands

import (
	"fmt"
	"os"

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

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "ag-ui-cli",
	Short: "AG-UI CLI - Tool-Based Generative UI Client",
	Long: `AG-UI CLI (Fang) is a command-line interface for interacting with
the AG-UI server, providing chat functionality with streaming responses,
tool call handling, and session management.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	RootCmd.PersistentFlags().StringVar(&outputFormat, "output", "pretty", "Output format (pretty|json)")
	RootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	RootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-essential output")
	RootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	RootCmd.PersistentFlags().StringVar(&sessionID, "session", "", "Session ID (uses last session if not specified)")
}