package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ag-ui/go-sdk/examples/client/internal/clienttools"
	"github.com/ag-ui/go-sdk/examples/client/internal/config"
	clienterrors "github.com/ag-ui/go-sdk/examples/client/internal/errors"
	"github.com/ag-ui/go-sdk/examples/client/internal/logging"
	"github.com/ag-ui/go-sdk/examples/client/internal/prompt"
	"github.com/ag-ui/go-sdk/examples/client/internal/session"
	"github.com/ag-ui/go-sdk/examples/client/internal/spinner"
	streamingpkg "github.com/ag-ui/go-sdk/examples/client/internal/streaming"
	"github.com/ag-ui/go-sdk/examples/client/internal/ui"
	"github.com/charmbracelet/fang"
	"github.com/google/uuid"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client/sse"
	pkgtools "github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	configManager *config.Manager
	logger        *logrus.Logger
)

func main() {
	rootCmd := newRootCommand()
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	// Initialize config manager
	configManager = config.NewManager()

	// Load configuration from all sources (defaults, file, env)
	if err := configManager.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
	}

	// Get initial config
	cfg := configManager.GetConfig()

	cmd := &cobra.Command{
		Use:   "ag-ui-client",
		Short: "AG-UI Client - A tool-based UI client with SSE support",
		Long: `AG-UI Client is a command-line interface for interacting with AG-UI servers.
It provides tool-based UI capabilities with Server-Sent Events (SSE) support
for real-time communication and state synchronization.

Configuration Precedence (highest to lowest):
  1. Command-line flags
  2. Environment variables (AGUI_*)
  3. Configuration file (~/.config/ag-ui/client/config.yaml)
  4. Default values

Examples:
  # Connect to a server with an API key
  ag-ui-client --server https://api.example.com --api-key your-key
  
  # Set custom log level and output format
  ag-ui-client --log-level debug --output json
  
  # Use environment variables
  export AGUI_SERVER=https://api.example.com
  export AGUI_API_KEY=your-key
  ag-ui-client
  
  # Set persistent configuration
  ag-ui-client config set server https://api.example.com
  ag-ui-client config set api_key your-key`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Apply command-line flags to configuration
			flags := make(map[string]string)
			cmd.Flags().Visit(func(f *pflag.Flag) {
				flags[f.Name] = f.Value.String()
			})
			configManager.ApplyFlags(flags)

			// Get final config after all precedence applied
			finalCfg := configManager.GetConfig()

			// Initialize logger with final config
			logger = logging.Initialize(logging.Options{
				Level:  finalCfg.LogLevel,
				Format: finalCfg.LogFormat,
			})

			// Log initialization info with config
			logger.WithFields(logrus.Fields{
				"component": "cli",
				"version":   "0.1.0",
				"server":    finalCfg.ServerURL,
				"output":    finalCfg.Output,
			}).Info("AG-UI Client initialized")

			// Debug log the full config (without sensitive data)
			logger.WithFields(logrus.Fields{
				"server":      finalCfg.ServerURL,
				"log_level":   finalCfg.LogLevel,
				"log_format":  finalCfg.LogFormat,
				"output":      finalCfg.Output,
				"has_api_key": finalCfg.APIKey != "",
			}).Debug("Configuration loaded")
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add persistent flags for global configuration
	// Use values from loaded config as defaults
	cmd.PersistentFlags().StringP("server", "s", cfg.ServerURL,
		"AG-UI server URL (env: AGUI_SERVER)")
	cmd.PersistentFlags().StringP("api-key", "k", cfg.APIKey,
		"API key for authentication (env: AGUI_API_KEY)")
	cmd.PersistentFlags().String("auth-header", cfg.AuthHeader,
		"Authentication header name: Authorization or X-API-Key (env: AGUI_AUTH_HEADER)")
	cmd.PersistentFlags().String("auth-scheme", cfg.AuthScheme,
		"Authentication scheme for Authorization header: Bearer, Basic, etc. (env: AGUI_AUTH_SCHEME)")
	cmd.PersistentFlags().StringP("log-level", "l", cfg.LogLevel,
		"Set the logging level: debug, info, warn, error (env: AGUI_LOG_LEVEL)")
	cmd.PersistentFlags().String("log-format", cfg.LogFormat,
		"Set the logging format: json, text (env: AGUI_LOG_FORMAT)")
	cmd.PersistentFlags().StringP("output", "o", cfg.Output,
		"Set the output format: json, text (env: AGUI_OUTPUT)")

	// Add subcommands
	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newSessionCommand())
	cmd.AddCommand(newToolsCommand())
	cmd.AddCommand(newChatCommand())
	cmd.AddCommand(newUICommand())
	cmd.AddCommand(newStreamCommand())
	cmd.AddCommand(newHumanLoopCommand())
	cmd.AddCommand(newStateCommand())
	cmd.AddCommand(newPredictiveCommand())
	cmd.AddCommand(newSharedCommand())
	cmd.AddCommand(newConfigCommand())

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Run: func(cmd *cobra.Command, args []string) {
			logger.WithFields(logrus.Fields{
				"component": "version",
				"build":     "development",
			}).Info("AG-UI Client version 0.1.0")

			// Test different log levels
			logger.Debug("Debug message - build details")
			logger.Warn("This is a development build")

			// Test sensitive data redaction
			logger.WithFields(logrus.Fields{
				"Authorization": "Bearer secret-token-123",
				"X-API-Key":     "api-key-456",
				"user":          "test-user",
			}).Info("Version check completed")
		},
	}
}

func newSessionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage AG-UI sessions",
		Long: `Manage AG-UI sessions for persistent connections and state management.

Examples:
  # Open a new session
  ag-ui-client session open --name my-session
  
  # Close an existing session
  ag-ui-client session close --session-id abc123
  
  # List active sessions
  ag-ui-client session list --output json`,
	}

	// Add subcommands
	cmd.AddCommand(newSessionOpenCommand())
	cmd.AddCommand(newSessionCloseCommand())
	cmd.AddCommand(newSessionListCommand())
	cmd.AddCommand(newSessionSaveCommand())
	cmd.AddCommand(newSessionLoadCommand())
	cmd.AddCommand(newSessionExportCommand())
	cmd.AddCommand(newSessionImportCommand())
	cmd.AddCommand(newSessionHistoryCommand())
	cmd.AddCommand(newSessionResumeCommand())

	return cmd
}

func newSessionOpenCommand() *cobra.Command {
	var label string
	var metadata string

	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open a new session",
		Long: `Open a new AG-UI session for persistent connection and state management.

Examples:
  # Open a basic session
  ag-ui-client session open
  
  # Open a labeled session
  ag-ui-client session open --label development
  
  # Open a session with metadata
  ag-ui-client session open --label prod --metadata '{"env":"production"}'`,
		Run: func(cmd *cobra.Command, args []string) {
			// Create session store
			sessionStore := session.NewStore(configManager.GetConfigPath())

			// Parse metadata if provided
			var metadataMap map[string]string
			if metadata != "" && metadata != "{}" {
				if err := json.Unmarshal([]byte(metadata), &metadataMap); err != nil {
					logger.WithError(err).Error("Failed to parse metadata JSON")
					os.Exit(1)
				}
			}

			// Open new session
			sess, err := sessionStore.OpenSession(label, metadataMap)
			if err != nil {
				logger.WithError(err).Error("Failed to open session")
				os.Exit(1)
			}

			// Update config with last session ID
			configManager.Set("last_session_id", sess.ThreadID)
			if err := configManager.SaveToFile(); err != nil {
				logger.WithError(err).Warn("Failed to update config with session ID")
			}

			// Output based on format
			if configManager.GetConfig().Output == "json" {
				output := map[string]interface{}{
					"threadId": sess.ThreadID,
					"status":   "opened",
				}
				if sess.Label != "" {
					output["label"] = sess.Label
				}
				if len(sess.Metadata) > 0 {
					output["metadata"] = sess.Metadata
				}
				data, _ := json.Marshal(output)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				// Print thread ID to stdout for piping
				fmt.Fprintln(cmd.OutOrStdout(), sess.ThreadID)
			}

			// Log success with details
			logger.WithFields(logrus.Fields{
				"threadId": sess.ThreadID,
				"label":    sess.Label,
			}).Debug("Session opened successfully")
		},
	}

	cmd.Flags().StringVar(&label, "label", "", "Optional session label")
	cmd.Flags().StringVar(&metadata, "metadata", "{}", "Session metadata (JSON)")

	return cmd
}

func newSessionCloseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close",
		Short: "Close the active session",
		Long: `Close the active AG-UI session and clear persisted session context.

This command is idempotent - it's safe to call even when no active session exists.

Examples:
  # Close the active session
  ag-ui-client session close
  
  # Close session with JSON output
  ag-ui-client session close --output json`,
		Run: func(cmd *cobra.Command, args []string) {
			// Create session store
			sessionStore := session.NewStore(configManager.GetConfigPath())

			// Get current session before closing (for reporting)
			currentSession, _ := sessionStore.GetActiveSession()

			// Close session (idempotent)
			if err := sessionStore.CloseSession(); err != nil {
				logger.WithError(err).Error("Failed to close session")
				os.Exit(1)
			}

			// Clear last session ID from config
			configManager.Set("last_session_id", "")
			if err := configManager.SaveToFile(); err != nil {
				logger.WithError(err).Warn("Failed to update config")
			}

			// Output based on format
			if configManager.GetConfig().Output == "json" {
				output := map[string]interface{}{
					"status": "closed",
				}
				if currentSession != nil {
					output["threadId"] = currentSession.ThreadID
				}
				data, _ := json.Marshal(output)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				if currentSession != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "Session closed: %s\n", currentSession.ThreadID)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No active session to close")
				}
			}

			// Log success
			if currentSession != nil {
				logger.WithFields(logrus.Fields{
					"threadId": currentSession.ThreadID,
				}).Debug("Session closed successfully")
			} else {
				logger.Debug("No active session was found (idempotent close)")
			}
		},
	}

	return cmd
}

func newSessionListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show active session",
		Long: `Show the current active AG-UI session.

Examples:
  # Show active session
  ag-ui-client session list
  
  # Show session in JSON format
  ag-ui-client session list --output json`,
		Run: func(cmd *cobra.Command, args []string) {
			// Create session store
			sessionStore := session.NewStore(configManager.GetConfigPath())

			// Get active session
			activeSession, err := sessionStore.GetActiveSession()
			if err != nil {
				logger.WithError(err).Error("Failed to load session")
				os.Exit(1)
			}

			// Output based on format
			if configManager.GetConfig().Output == "json" {
				if activeSession != nil {
					output := map[string]interface{}{
						"threadId":     activeSession.ThreadID,
						"label":        activeSession.Label,
						"lastOpenedAt": activeSession.LastOpenedAt.Format(time.RFC3339),
						"status":       "active",
					}
					if len(activeSession.Metadata) > 0 {
						output["metadata"] = activeSession.Metadata
					}
					data, _ := json.MarshalIndent(output, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(data))
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
			} else {
				if activeSession != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "Active session:\n")
					fmt.Fprintf(cmd.OutOrStdout(), "  Thread ID: %s\n", activeSession.ThreadID)
					if activeSession.Label != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  Label: %s\n", activeSession.Label)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  Opened at: %s\n", activeSession.LastOpenedAt.Format(time.RFC3339))
					if len(activeSession.Metadata) > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "  Metadata:\n")
						for k, v := range activeSession.Metadata {
							fmt.Fprintf(cmd.OutOrStdout(), "    %s: %s\n", k, v)
						}
					}
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No active session")
				}
			}
		},
	}

	return cmd
}

func newSessionSaveCommand() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "save [session-id]",
		Short: "Save session to disk",
		Long: `Explicitly save the current session or a specific session to disk.
Sessions are automatically saved during chat/ui commands, but this provides manual control.

Examples:
  # Save the active session
  ag-ui-client session save
  
  # Save a specific session
  ag-ui-client session save abc123`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get session ID from args or use active session
			if len(args) > 0 {
				sessionID = args[0]
			} else {
				sessionStore := session.NewStore(configManager.GetConfigPath())
				activeSession, err := sessionStore.GetActiveSession()
				if err != nil || activeSession == nil {
					logger.Error("No active session to save")
					os.Exit(1)
				}
				sessionID = activeSession.ThreadID
			}

			// Create persistent store
			persistentStore := session.NewPersistentStore(configManager.GetConfigPath())

			// Load the session
			sessionData, err := persistentStore.LoadSession(sessionID)
			if err != nil {
				// Create new session if not found
				sessionData, err = persistentStore.CreateSession(sessionID, "")
				if err != nil {
					logger.WithError(err).Error("Failed to create session")
					os.Exit(1)
				}
			}

			// Save the session
			if err := persistentStore.SaveSession(sessionData); err != nil {
				logger.WithError(err).Error("Failed to save session")
				os.Exit(1)
			}

			if configManager.GetConfig().Output == "json" {
				output := map[string]interface{}{
					"threadId": sessionID,
					"status":   "saved",
					"messages": len(sessionData.Messages),
				}
				data, _ := json.Marshal(output)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Session saved: %s (%d messages)\n", sessionID, len(sessionData.Messages))
			}
		},
	}

	return cmd
}

func newSessionLoadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "load <session-id>",
		Short: "Load a saved session",
		Long: `Load a previously saved session from disk and make it the active session.

Examples:
  # Load a specific session
  ag-ui-client session load abc123
  
  # Load and show details
  ag-ui-client session load abc123 --output json`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sessionID := args[0]

			// Create persistent store
			persistentStore := session.NewPersistentStore(configManager.GetConfigPath())

			// Load the session
			sessionData, err := persistentStore.LoadSession(sessionID)
			if err != nil {
				logger.WithError(err).Error("Failed to load session")
				os.Exit(1)
			}

			// Make it the active session
			metadata := make(map[string]string)
			for k, v := range sessionData.Metadata {
				if str, ok := v.(string); ok {
					metadata[k] = str
				}
			}

			// Open session with the loaded thread ID
			activeSession := &session.Session{
				ThreadID:     sessionData.ThreadID,
				Label:        sessionData.Label,
				LastOpenedAt: time.Now(),
				Metadata:     metadata,
			}

			// Save as active session
			storeData := &session.StoreData{
				ActiveSession: activeSession,
			}
			path := filepath.Join(filepath.Dir(configManager.GetConfigPath()), "session.json")
			jsonData, _ := json.MarshalIndent(storeData, "", "  ")
			if err := os.WriteFile(path, jsonData, 0644); err != nil {
				logger.WithError(err).Error("Failed to set as active session")
				os.Exit(1)
			}

			// Update config
			configManager.Set("last_session_id", sessionID)
			configManager.SaveToFile()

			if configManager.GetConfig().Output == "json" {
				output := map[string]interface{}{
					"threadId":  sessionData.ThreadID,
					"label":     sessionData.Label,
					"status":    "loaded",
					"messages":  len(sessionData.Messages),
					"createdAt": sessionData.CreatedAt,
					"updatedAt": sessionData.UpdatedAt,
				}
				data, _ := json.Marshal(output)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Session loaded: %s\n", sessionID)
				fmt.Fprintf(cmd.OutOrStdout(), "  Messages: %d\n", len(sessionData.Messages))
				fmt.Fprintf(cmd.OutOrStdout(), "  Created: %s\n", sessionData.CreatedAt.Format(time.RFC3339))
				fmt.Fprintf(cmd.OutOrStdout(), "  Updated: %s\n", sessionData.UpdatedAt.Format(time.RFC3339))
			}
		},
	}

	return cmd
}

func newSessionExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export <session-id> <output-file>",
		Short: "Export a session to a file",
		Long: `Export a session to a portable JSON file that can be imported later or shared.

Examples:
  # Export active session
  ag-ui-client session export abc123 session-backup.json
  
  # Export with compression
  ag-ui-client session export abc123 session-backup.json.gz`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			sessionID := args[0]
			outputPath := args[1]

			// Create persistent store
			persistentStore := session.NewPersistentStore(configManager.GetConfigPath())

			// Export the session
			if err := persistentStore.ExportSession(sessionID, outputPath); err != nil {
				logger.WithError(err).Error("Failed to export session")
				os.Exit(1)
			}

			// Get file info for size
			info, _ := os.Stat(outputPath)

			if configManager.GetConfig().Output == "json" {
				output := map[string]interface{}{
					"threadId": sessionID,
					"status":   "exported",
					"file":     outputPath,
					"size":     info.Size(),
				}
				data, _ := json.Marshal(output)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Session exported: %s -> %s (%d bytes)\n", sessionID, outputPath, info.Size())
			}
		},
	}

	return cmd
}

func newSessionImportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <input-file>",
		Short: "Import a session from a file",
		Long: `Import a session from a JSON file that was previously exported.

Examples:
  # Import a session
  ag-ui-client session import session-backup.json
  
  # Import and make it active
  ag-ui-client session import session-backup.json --activate`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			inputPath := args[0]
			activate, _ := cmd.Flags().GetBool("activate")

			// Create persistent store
			persistentStore := session.NewPersistentStore(configManager.GetConfigPath())

			// Import the session
			sessionData, err := persistentStore.ImportSession(inputPath)
			if err != nil {
				logger.WithError(err).Error("Failed to import session")
				os.Exit(1)
			}

			// Optionally activate the session
			if activate {
				metadata := make(map[string]string)
				for k, v := range sessionData.Metadata {
					if str, ok := v.(string); ok {
						metadata[k] = str
					}
				}

				activeSession := &session.Session{
					ThreadID:     sessionData.ThreadID,
					Label:        sessionData.Label,
					LastOpenedAt: time.Now(),
					Metadata:     metadata,
				}

				storeData := &session.StoreData{
					ActiveSession: activeSession,
				}
				path := filepath.Join(filepath.Dir(configManager.GetConfigPath()), "session.json")
				jsonData, _ := json.MarshalIndent(storeData, "", "  ")
				os.WriteFile(path, jsonData, 0644)

				configManager.Set("last_session_id", sessionData.ThreadID)
				configManager.SaveToFile()
			}

			if configManager.GetConfig().Output == "json" {
				output := map[string]interface{}{
					"threadId": sessionData.ThreadID,
					"label":    sessionData.Label,
					"status":   "imported",
					"messages": len(sessionData.Messages),
					"active":   activate,
				}
				data, _ := json.Marshal(output)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Session imported: %s (%d messages)\n", sessionData.ThreadID, len(sessionData.Messages))
				if activate {
					fmt.Fprintln(cmd.OutOrStdout(), "Session is now active")
				}
			}
		},
	}

	cmd.Flags().Bool("activate", false, "Make the imported session active")

	return cmd
}

func newSessionHistoryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history [session-id]",
		Short: "Show session conversation history",
		Long: `Display the conversation history for a session.

Examples:
  # Show active session history
  ag-ui-client session history
  
  # Show specific session history
  ag-ui-client session history abc123
  
  # Show history as JSON
  ag-ui-client session history --output json`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var sessionID string

			// Get session ID from args or use active session
			if len(args) > 0 {
				sessionID = args[0]
			} else {
				sessionStore := session.NewStore(configManager.GetConfigPath())
				activeSession, err := sessionStore.GetActiveSession()
				if err != nil || activeSession == nil {
					logger.Error("No active session")
					os.Exit(1)
				}
				sessionID = activeSession.ThreadID
			}

			// Create persistent store
			persistentStore := session.NewPersistentStore(configManager.GetConfigPath())

			// Get session history
			messages, err := persistentStore.GetSessionHistory(sessionID)
			if err != nil {
				// Try to create session if not found
				_, err = persistentStore.CreateSession(sessionID, "")
				if err != nil {
					logger.WithError(err).Error("Failed to load session history")
					os.Exit(1)
				}
				messages = []session.ConversationMessage{}
			}

			if configManager.GetConfig().Output == "json" {
				output := map[string]interface{}{
					"threadId": sessionID,
					"messages": messages,
					"count":    len(messages),
				}
				data, _ := json.MarshalIndent(output, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Session History: %s\n", sessionID)
				fmt.Fprintf(cmd.OutOrStdout(), "Messages: %d\n\n", len(messages))

				for i, msg := range messages {
					fmt.Fprintf(cmd.OutOrStdout(), "[%d] %s - %s\n", i+1, msg.Role, msg.Timestamp.Format("15:04:05"))

					if msg.Content != "" {
						// Word wrap content for readability
						lines := wordWrap(msg.Content, 70)
						for _, line := range lines {
							fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", line)
						}
					}

					if len(msg.ToolCalls) > 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "    Tool Calls:")
						for _, tc := range msg.ToolCalls {
							fmt.Fprintf(cmd.OutOrStdout(), "      - %s (id: %s)\n", tc.Function.Name, tc.ID)
						}
					}

					if msg.ToolCallID != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "    Tool Result for: %s\n", msg.ToolCallID)
					}

					fmt.Fprintln(cmd.OutOrStdout())
				}
			}
		},
	}

	return cmd
}

func newSessionResumeCommand() *cobra.Command {
	var last bool
	var interactive bool
	var streaming bool

	cmd := &cobra.Command{
		Use:   "resume [session-id]",
		Short: "Resume a previous session with full context",
		Long: `Resume a previous conversation session with full message history and state.
The resumed session continues with all previous context, tool results, and state intact.

Examples:
  # Resume a specific session
  fang session resume abc123
  
  # Resume the last active session
  fang session resume --last
  
  # Resume in interactive mode
  fang session resume abc123 --interactive
  
  # Resume with streaming
  fang session resume abc123 --streaming`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var sessionID string

			// Determine which session to resume
			if last {
				// Resume last active session
				sessionStore := session.NewStore(configManager.GetConfigPath())
				activeSession, err := sessionStore.GetActiveSession()
				if err != nil || activeSession == nil {
					logger.Error("No active session to resume")
					os.Exit(1)
				}
				sessionID = activeSession.ThreadID
			} else if len(args) > 0 {
				// Resume specific session
				sessionID = args[0]
			} else {
				// List available sessions for user to choose
				persistentStore := session.NewPersistentStore(configManager.GetConfigPath())
				sessions, err := persistentStore.ListSessions()
				if err != nil {
					logger.WithError(err).Error("Failed to list sessions")
					os.Exit(1)
				}

				if len(sessions) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No sessions available to resume")
					os.Exit(1)
				}

				fmt.Fprintln(cmd.OutOrStdout(), "Available sessions:")
				for i, sess := range sessions {
					label := sess.Label
					if label == "" {
						label = "no label"
					}
					msgCount := len(sess.Messages)
					fmt.Fprintf(cmd.OutOrStdout(), "  [%d] %s - %s (%d messages)\n",
						i+1, sess.ThreadID, label, msgCount)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\nSpecify a session ID to resume")
				os.Exit(1)
			}

			// Load the session from persistent store
			persistentStore := session.NewPersistentStore(configManager.GetConfigPath())
			sessionData, err := persistentStore.LoadSession(sessionID)
			if err != nil {
				logger.WithError(err).Error("Failed to load session")
				os.Exit(1)
			}

			// Make it the active session
			sessionStore := session.NewStore(configManager.GetConfigPath())
			metadata := make(map[string]string)
			for k, v := range sessionData.Metadata {
				if str, ok := v.(string); ok {
					metadata[k] = str
				}
			}

			_, err = sessionStore.OpenSession(sessionData.Label, metadata)
			if err != nil {
				logger.WithError(err).Warn("Failed to set as active session")
			}

			// Display session info
			fmt.Fprintf(cmd.OutOrStdout(), "Resuming session: %s\n", sessionID)
			if sessionData.Label != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Label: %s\n", sessionData.Label)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Messages: %d\n", len(sessionData.Messages))
			fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", sessionData.CreatedAt.Format(time.RFC3339))
			fmt.Fprintf(cmd.OutOrStdout(), "Last updated: %s\n\n", sessionData.UpdatedAt.Format(time.RFC3339))

			// Show recent conversation history
			if len(sessionData.Messages) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Recent conversation:")
				start := 0
				if len(sessionData.Messages) > 3 {
					start = len(sessionData.Messages) - 3
				}
				for i := start; i < len(sessionData.Messages); i++ {
					msg := sessionData.Messages[i]
					content := msg.Content
					if len(content) > 100 {
						content = content[:97] + "..."
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", msg.Role, content)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			// If interactive mode, start chat with loaded context
			if interactive {
				fmt.Fprintln(cmd.OutOrStdout(), "Starting interactive chat with resumed session...")
				fmt.Fprintln(cmd.OutOrStdout(), "Type your message (or 'exit' to quit):")

				// Prepare the endpoint
				cfg := configManager.GetConfig()
				endpoint := strings.TrimSuffix(cfg.ServerURL, "/") + "/tool_based_generative_ui"
				if streaming {
					endpoint = strings.TrimSuffix(cfg.ServerURL, "/") + "/agentic_chat"
				}

				// Convert stored messages to AG-UI format
				var messages []map[string]interface{}
				for _, msg := range sessionData.Messages {
					agMsg := map[string]interface{}{
						"role": msg.Role,
					}

					if msg.Content != "" {
						agMsg["content"] = msg.Content
					}

					if len(msg.ToolCalls) > 0 {
						var toolCalls []map[string]interface{}
						for _, tc := range msg.ToolCalls {
							toolCall := map[string]interface{}{
								"id":   tc.ID,
								"type": tc.Type,
								"function": map[string]interface{}{
									"name":      tc.Function.Name,
									"arguments": tc.Function.Arguments,
								},
							}
							toolCalls = append(toolCalls, toolCall)
						}
						agMsg["toolCalls"] = toolCalls
					}

					if msg.ToolCallID != "" {
						agMsg["toolCallId"] = msg.ToolCallID
					}

					messages = append(messages, agMsg)
				}

				// Start interactive loop
				scanner := bufio.NewScanner(os.Stdin)
				for {
					fmt.Print("> ")
					if !scanner.Scan() {
						break
					}

					input := scanner.Text()
					if input == "exit" || input == "quit" {
						fmt.Fprintln(cmd.OutOrStdout(), "Ending resumed session.")
						break
					}

					// Add user message
					userMsg := map[string]interface{}{
						"id":      fmt.Sprintf("msg-%d", time.Now().UnixNano()),
						"role":    "user",
						"content": input,
					}
					messages = append(messages, userMsg)

					// Save user message to persistent store
					convMsg := session.ConversationMessage{
						Role:      "user",
						Content:   input,
						Timestamp: time.Now(),
					}
					if err := persistentStore.AddMessage(sessionID, convMsg); err != nil {
						logger.WithError(err).Warn("Failed to save user message")
					}

					// Prepare request
					requestBody := map[string]interface{}{
						"threadId": sessionID,
						"runId":    fmt.Sprintf("run-%d", time.Now().Unix()),
						"messages": messages,
						"state":    sessionData.State,
					}

					// Send request and process response
					jsonData, err := json.Marshal(requestBody)
					if err != nil {
						logger.WithError(err).Error("Failed to marshal request")
						continue
					}

					req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
					if err != nil {
						logger.WithError(err).Error("Failed to create request")
						continue
					}

					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Accept", "text/event-stream")

					// Add auth if configured
					if cfg.APIKey != "" {
						req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
					}

					client := &http.Client{Timeout: 30 * time.Second}
					resp, err := client.Do(req)
					if err != nil {
						logger.WithError(err).Error("Failed to send request")
						continue
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						logger.WithField("status", resp.StatusCode).Error("Request failed")
						continue
					}

					// Process SSE response
					fmt.Fprint(cmd.OutOrStdout(), "Assistant: ")
					scanner := bufio.NewScanner(resp.Body)
					var assistantContent strings.Builder

					for scanner.Scan() {
						line := scanner.Text()
						if strings.HasPrefix(line, "data: ") {
							data := strings.TrimPrefix(line, "data: ")

							var event map[string]interface{}
							if err := json.Unmarshal([]byte(data), &event); err != nil {
								continue
							}

							eventType, _ := event["type"].(string)

							switch eventType {
							case "TEXT_MESSAGE_CONTENT", "TEXT_MESSAGE_CHUNK":
								content, ok := event["delta"].(string)
								if !ok {
									content, _ = event["content"].(string)
								}
								fmt.Fprint(cmd.OutOrStdout(), content)
								assistantContent.WriteString(content)

							case "MESSAGES_SNAPSHOT":
								if messagesData, ok := event["messages"].([]interface{}); ok {
									// Update our messages array with the snapshot
									messages = nil
									for _, msgData := range messagesData {
										if msgMap, ok := msgData.(map[string]interface{}); ok {
											messages = append(messages, msgMap)

											// Save assistant messages to persistent store
											if role, _ := msgMap["role"].(string); role == "assistant" {
												content, _ := msgMap["content"].(string)
												if content != "" && content != assistantContent.String() {
													convMsg := session.ConversationMessage{
														Role:      "assistant",
														Content:   content,
														Timestamp: time.Now(),
													}
													persistentStore.AddMessage(sessionID, convMsg)
												}
											}
										}
									}
								}

							case "TOOL_CALL_RESULT":
								if name, ok := event["name"].(string); ok {
									fmt.Fprintf(cmd.OutOrStdout(), "\n🔧 Tool executed: %s\n", name)
								}

							case "RUN_FINISHED":
								fmt.Fprintln(cmd.OutOrStdout())
							}
						}
					}

					// Save assistant response if we accumulated content
					if assistantContent.Len() > 0 {
						convMsg := session.ConversationMessage{
							Role:      "assistant",
							Content:   assistantContent.String(),
							Timestamp: time.Now(),
						}
						if err := persistentStore.AddMessage(sessionID, convMsg); err != nil {
							logger.WithError(err).Warn("Failed to save assistant message")
						}
					}

					fmt.Fprintln(cmd.OutOrStdout())
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Session resumed. Use --interactive flag to continue the conversation.")
			}
		},
	}

	// Add flags
	cmd.Flags().BoolVar(&last, "last", false, "Resume the last active session")
	cmd.Flags().BoolVar(&interactive, "interactive", false, "Start interactive chat with resumed session")
	cmd.Flags().BoolVar(&streaming, "streaming", false, "Use streaming mode for responses")

	return cmd
}

func newToolsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Manage and interact with AG-UI tools",
		Long: `Manage and interact with AG-UI tools and their capabilities.

Examples:
  # List all available tools
  ag-ui-client tools list
  
  # List tools in JSON format
  ag-ui-client tools list --json
  
  # Filter tools by name
  ag-ui-client tools list --filter http
  
  # Get tool details
  ag-ui-client tools describe http_get`,
	}

	// Add subcommands
	cmd.AddCommand(newToolsListCommand())
	cmd.AddCommand(newToolsDescribeCommand())
	cmd.AddCommand(newToolsRunCommand())

	return cmd
}

func newToolsListCommand() *cobra.Command {
	var nameFilter string
	var capabilityFilter string
	var tagFilter string
	var verbose bool
	var noColor bool
	var quiet bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available tools from the server",
		Long: `List all available AG-UI tools exposed by the server.

This command fetches and displays tools that are available for Tool-Based 
Generative UI operations. It supports filtering and multiple output formats.

Examples:
  # List all tools in pretty table format
  ag-ui-client tools list
  
  # List tools in JSON format (one per line)
  ag-ui-client tools list --output json
  
  # Filter tools by name pattern
  ag-ui-client tools list --name http
  
  # Filter tools by capability
  ag-ui-client tools list --capability streaming
  
  # Show detailed schema information
  ag-ui-client tools list --verbose
  
  # Combine filters
  ag-ui-client tools list --name file --tag filesystem --verbose`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg := configManager.GetConfig()

			if cfg.ServerURL == "" {
				logger.Error("Server URL not configured. Use --server flag or set AGUI_SERVER environment variable")
				os.Exit(1)
			}

			// Fetch tools from server
			tools, err := fetchToolsFromServer(cfg)
			if err != nil {
				logger.WithError(err).Error("Failed to fetch tools from server")
				os.Exit(1)
			}

			// Apply client-side filtering
			filteredTools := filterTools(tools, nameFilter, capabilityFilter, tagFilter)

			// Render output based on format
			if cfg.Output == "json" {
				renderToolsJSON(cmd.OutOrStdout(), filteredTools, verbose)
			} else {
				renderToolsPretty(cmd.OutOrStdout(), filteredTools, verbose, noColor)
			}

			if !quiet {
				logger.WithField("count", len(filteredTools)).Debug("Tools listed successfully")
			}
		},
	}

	cmd.Flags().StringVar(&nameFilter, "name", "", "Filter tools by name (substring match)")
	cmd.Flags().StringVar(&capabilityFilter, "capability", "", "Filter tools by capability")
	cmd.Flags().StringVar(&tagFilter, "tag", "", "Filter tools by tag")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show full schema and parameter details")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress informational output")

	return cmd
}

func newToolsDescribeCommand() *cobra.Command {
	var noColor bool

	cmd := &cobra.Command{
		Use:   "describe [tool-name]",
		Short: "Describe a specific tool",
		Long: `Display detailed information about a specific tool including its parameters, schema, and usage examples.
		
Examples:
  # Describe a tool with pretty output
  ag-ui-client tools describe generate_haiku
  
  # Get tool description in JSON format
  ag-ui-client tools describe generate_haiku --output json
  
  # Disable colored output
  ag-ui-client tools describe http_get --no-color`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			toolName := args[0]
			cfg := configManager.GetConfig()

			if cfg.ServerURL == "" {
				logger.Error("Server URL not configured. Use --server flag or set AGUI_SERVER environment variable")
				os.Exit(1)
			}

			logger.WithFields(logrus.Fields{
				"action": "tools_describe",
				"tool":   toolName,
			}).Debug("Fetching tool description")

			// Fetch all tools from server
			tools, err := fetchToolsFromServer(cfg)
			if err != nil {
				logger.WithError(err).Error("Failed to fetch tools from server")
				os.Exit(1)
			}

			// Find the specific tool
			var targetTool *Tool
			for _, tool := range tools {
				if tool.Name == toolName {
					targetTool = &tool
					break
				}
			}

			if targetTool == nil {
				logger.Errorf("Tool '%s' not found", toolName)
				logger.Info("Use 'ag-ui-client tools list' to see available tools")
				os.Exit(1)
			}

			// Render output based on format
			if cfg.Output == "json" {
				renderToolDescriptionJSON(cmd.OutOrStdout(), targetTool)
			} else {
				renderToolDescriptionPretty(cmd.OutOrStdout(), targetTool, noColor)
			}
		},
	}

	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	return cmd
}

func newToolsRunCommand() *cobra.Command {
	var jsonOutput bool
	var sessionID string
	var interactive bool
	var argsJSON string
	var timeout int
	var skipValidation bool
	var clientTools bool
	var toolsDir string
	var onError string
	var maxRetries int
	var verbose bool

	cmd := &cobra.Command{
		Use:   "run [tool-names...]",
		Short: "Execute tools directly",
		Long: `Execute one or more tools directly with specified arguments.

Examples:
  # Run single tool
  ag-ui-client tools run generate_haiku
  
  # Run multiple tools
  ag-ui-client tools run generate_haiku generate_poem
  
  # Run tool with arguments (JSON string)
  ag-ui-client tools run generate_haiku --args '{"topic": "programming"}'
  
  # Run multiple tools with arguments (JSON array)
  ag-ui-client tools run generate_haiku generate_poem --args '[{"topic": "programming"}, {"style": "sonnet"}]'
  
  # Run with JSON output
  ag-ui-client tools run generate_haiku --json
  
  # Run in specific session
  ag-ui-client tools run generate_haiku --session-id abc123
  
  # Non-interactive mode (no prompts)
  ag-ui-client tools run generate_haiku --interactive=false
  
  # Skip argument validation (use with caution)
  ag-ui-client tools run experimental_tool --skip-validation --args '{"data": "raw"}'
  
  # Run tools locally (client-side execution)
  ag-ui-client tools run shell_command --client-tools --args '{"command": "ls -la"}'`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolNames := args
			cfg := configManager.GetConfig()

			if cfg.ServerURL == "" {
				logger.Error("Server URL not configured. Use --server flag or set AGUI_SERVER environment variable")
				return fmt.Errorf("server URL not configured")
			}

			// Disable interactive mode if JSON output
			if jsonOutput {
				interactive = false
			}

			// Initialize client-side tools if enabled
			var clientToolRegistry *clienttools.Registry
			if clientTools {
				clientToolRegistry = clienttools.NewRegistry()

				// Register built-in tools
				if err := clientToolRegistry.RegisterBuiltinTools(); err != nil {
					logger.WithError(err).Error("Failed to register built-in client tools")
					return fmt.Errorf("failed to register built-in client tools: %w", err)
				}

				// Load custom tools from directory if specified
				if toolsDir != "" {
					if err := clientToolRegistry.LoadToolsFromDirectory(toolsDir); err != nil {
						logger.WithError(err).Warn("Failed to load custom tools from directory")
					}
				}

				logger.WithField("toolCount", len(clientToolRegistry.ListTools())).Info("Client-side tools initialized")
			}

			// Parse tool arguments - support both single object and array
			var toolArgsList []map[string]interface{}
			if argsJSON != "" {
				// Try parsing as array first
				var argsList []map[string]interface{}
				if err := json.Unmarshal([]byte(argsJSON), &argsList); err == nil {
					toolArgsList = argsList
				} else {
					// Try parsing as single object
					var singleArgs map[string]interface{}
					if err := json.Unmarshal([]byte(argsJSON), &singleArgs); err != nil {
						// Use error handler for better error display
						errorHandler := clienterrors.NewErrorHandler(logger, os.Stderr)
						errorHandler.SetJSONOutput(jsonOutput)
						errorHandler.SetVerbose(verbose)
						errorHandler.HandleError(
							clienterrors.NewValidationError("", fmt.Sprintf("Failed to parse tool arguments JSON: %v", err)),
							"argument parsing",
						)
						return fmt.Errorf("invalid JSON arguments: %w", err)
					}
					// Use same args for all tools if single object provided
					for range toolNames {
						toolArgsList = append(toolArgsList, singleArgs)
					}
				}
			} else {
				// No arguments provided - use empty args for all tools
				for range toolNames {
					toolArgsList = append(toolArgsList, make(map[string]interface{}))
				}
			}

			// Get or create session ID
			if sessionID == "" {
				sessionStore := session.NewStore(configManager.GetConfigPath())
				activeSession, err := sessionStore.GetActiveSession()
				if err == nil && activeSession != nil {
					sessionID = activeSession.ThreadID
				} else {
					sessionID = uuid.New().String()
				}
			}

			// Validate tool arguments if validation is enabled
			if !skipValidation && len(toolArgsList) > 0 {
				// Fetch available tools from server to get schemas
				availableTools, err := fetchToolsFromServer(cfg)
				if err != nil {
					logger.WithError(err).Warn("Failed to fetch tool schemas for validation, proceeding without validation")
				} else {
					// Validate each tool's arguments
					for i, toolName := range toolNames {
						// Find tool schema
						var toolSchema *pkgtools.ToolSchema
						for _, tool := range availableTools {
							if tool.Name == toolName {
								// Extract schema from tool
								if tool.Parameters != nil {
									// Convert to ToolSchema structure
									toolSchema = convertToToolSchema(tool.Parameters)
								}
								break
							}
						}

						if toolSchema != nil && i < len(toolArgsList) {
							// Marshal arguments to JSON for validation
							argsJSON, _ := json.Marshal(toolArgsList[i])

							// Validate arguments
							if err := pkgtools.ValidateArguments(argsJSON, toolSchema); err != nil {
								if jsonOutput {
									// Output validation error as JSON
									errorResult := map[string]interface{}{
										"error":   "validation_failed",
										"tool":    toolName,
										"message": err.Error(),
									}
									jsonBytes, _ := json.MarshalIndent(errorResult, "", "  ")
									fmt.Println(string(jsonBytes))
								} else {
									// Pretty print validation error
									fmt.Fprintf(os.Stderr, "\n❌ Validation Error for tool '%s':\n", toolName)
									fmt.Fprintf(os.Stderr, "   %s\n\n", err.Error())
									fmt.Fprintf(os.Stderr, "💡 Tip: Use --skip-validation to bypass validation (use with caution)\n")
									fmt.Fprintf(os.Stderr, "💡 Tip: Use 'tools describe %s' to see the tool's schema\n\n", toolName)
								}
								return fmt.Errorf("validation error for tool '%s': %w", toolName, err)
							}
						}
					}

					if !jsonOutput && !skipValidation {
						logger.Info("✅ All tool arguments validated successfully")
					}
				}
			}

			// Create endpoint URL - Use tool_based_generative_ui for proper tool execution
			endpoint := strings.TrimSuffix(cfg.ServerURL, "/") + "/tool_based_generative_ui"

			// Create tool calls for all requested tools
			var toolCalls []map[string]interface{}
			for i, toolName := range toolNames {
				toolCallID := fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), i)

				// Get arguments for this tool
				var toolArgs map[string]interface{}
				if i < len(toolArgsList) {
					toolArgs = toolArgsList[i]
				} else {
					toolArgs = make(map[string]interface{})
				}

				// Marshal arguments to JSON string
				argsBytes, _ := json.Marshal(toolArgs)

				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   toolCallID,
					"type": "function",
					"function": map[string]interface{}{
						"name":      toolName,
						"arguments": string(argsBytes),
					},
				})
			}

			// Check if any tools should be executed locally
			var localResults []interface{}
			var remoteToolCalls []map[string]interface{}

			if clientToolRegistry != nil {
				for i, toolName := range toolNames {
					toolCallID := fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), i)

					// Check if this tool exists locally
					if _, exists := clientToolRegistry.GetTool(toolName); exists {
						// Execute tool locally
						logger.WithField("tool", toolName).Info("Executing tool locally")

						// Get arguments for this tool
						var toolArgs map[string]interface{}
						if i < len(toolArgsList) {
							toolArgs = toolArgsList[i]
						} else {
							toolArgs = make(map[string]interface{})
						}

						// Show spinner if not JSON output
						var toolSpinner *spinner.ToolExecutionSpinner
						if !jsonOutput {
							toolSpinner = spinner.NewToolExecution(os.Stdout, toolName)
							toolSpinner.Start()
						}

						// Execute the tool
						execCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
						execResult, err := clientToolRegistry.ExecuteTool(execCtx, toolName, toolArgs)
						cancel()

						// Stop spinner and show result
						if toolSpinner != nil {
							toolSpinner.CompleteWithResult(err == nil)
						}

						if err != nil {
							// Use enhanced error handler
							errorHandler := clienterrors.NewErrorHandler(logger, os.Stderr)
							errorHandler.SetJSONOutput(jsonOutput)
							errorHandler.SetVerbose(verbose)
							errorHandler.SetNoColor(false)

							toolErr := clienterrors.NewToolExecutionError(toolName, err.Error())
							errorHandler.HandleError(toolErr, "local tool execution")

							localResults = append(localResults, map[string]interface{}{
								"toolCallId": toolCallID,
								"toolName":   toolName,
								"error":      err.Error(),
								"success":    false,
							})
						} else {
							localResults = append(localResults, map[string]interface{}{
								"toolCallId": toolCallID,
								"toolName":   toolName,
								"result":     execResult,
								"success":    true,
							})

							if !jsonOutput && execResult != nil {
								fmt.Fprintf(os.Stdout, "\n📋 Local Tool Result (%s):\n", toolName)
								// Format and display the result
								if execResult.Success {
									resultJSON, _ := json.MarshalIndent(execResult.Result, "   ", "  ")
									fmt.Fprintf(os.Stdout, "   %s\n", string(resultJSON))
								} else {
									fmt.Fprintf(os.Stdout, "   Error: %s\n", execResult.Error)
								}
							}
						}
					} else {
						// Add to remote tool calls
						remoteToolCalls = append(remoteToolCalls, toolCalls[i])
					}
				}
			} else {
				// No client tools registry, all tools are remote
				remoteToolCalls = toolCalls
			}

			// If all tools were executed locally, return the results
			if len(remoteToolCalls) == 0 && len(localResults) > 0 {
				if jsonOutput {
					output := map[string]interface{}{
						"success": true,
						"results": localResults,
						"mode":    "client-side",
					}
					jsonBytes, _ := json.MarshalIndent(output, "", "  ")
					fmt.Println(string(jsonBytes))
				} else {
					fmt.Println("\n✅ All tools executed locally")
				}
				return nil
			}

			// Update tool calls to only include remote tools
			toolCalls = remoteToolCalls

			// Create request payload
			// For tools run command, we need to send a user message requesting the tool execution
			userMessage := map[string]interface{}{
				"id":      fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				"role":    "user",
				"content": fmt.Sprintf("Please execute the following tool(s): %s", strings.Join(toolNames, ", ")),
			}

			payload := map[string]interface{}{
				"thread_id":      sessionID, // Note: underscore format for this endpoint
				"run_id":         fmt.Sprintf("run-%d", time.Now().UnixNano()),
				"state":          map[string]interface{}{},
				"messages":       []interface{}{userMessage}, // User message requesting tool execution
				"tools":          []interface{}{},
				"context":        []interface{}{},
				"forwardedProps": map[string]interface{}{},
			}

			// Marshal payload
			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				logger.WithError(err).Error("Failed to marshal request payload")
				return fmt.Errorf("failed to marshal request payload: %w", err)
			}

			// Create HTTP request
			req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payloadBytes))
			if err != nil {
				logger.WithError(err).Error("Failed to create request")
				return fmt.Errorf("failed to create request: %w", err)
			}

			// Set headers
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "text/event-stream")

			// Add authentication if configured
			if cfg.APIKey != "" {
				req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
			}

			// Create HTTP client with timeout
			client := &http.Client{
				Timeout: time.Duration(timeout) * time.Second,
			}

			// Send request with retry logic
			logger.WithFields(logrus.Fields{
				"tools":   toolNames,
				"count":   len(toolNames),
				"session": sessionID,
			}).Debug("Executing tools")

			var resp *http.Response
			retryErr := executeWithRetry(func() error {
				var reqErr error
				resp, reqErr = client.Do(req)
				if reqErr != nil {
					return reqErr
				}

				// Check for server errors that should trigger retry
				if resp.StatusCode >= 500 {
					body, _ := io.ReadAll(resp.Body)
					resp.Body.Close()
					return fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
				}

				return nil
			}, 3, 2*time.Second) // Retry up to 3 times with 2-second delay

			if retryErr != nil {
				// Use enhanced error handler
				errorHandler := clienterrors.NewErrorHandler(logger, os.Stderr)
				errorHandler.SetJSONOutput(jsonOutput)
				errorHandler.SetVerbose(verbose)
				errorHandler.SetNoColor(false)

				errorHandler.HandleError(retryErr, "HTTP request")
				errorHandler.DisplaySummary()
				return fmt.Errorf("failed after retries: %w", retryErr)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				// Use enhanced error handler for HTTP errors
				errorHandler := clienterrors.NewErrorHandler(logger, os.Stderr)
				errorHandler.SetJSONOutput(jsonOutput)
				errorHandler.SetVerbose(verbose)

				httpErr := errorHandler.HandleHTTPError(resp, "tool execution request")
				if httpErr != nil {
					errorHandler.HandleError(httpErr, "server response")
					errorHandler.DisplaySummary()
				}
				return fmt.Errorf("server returned status %d", resp.StatusCode)
			}

			// Process SSE events manually
			var toolResults []interface{}
			var hasResult bool
			var toolSpinner *spinner.ToolExecutionSpinner

			// Start spinner in pretty mode
			if !jsonOutput {
				// Show spinner with all tool names
				toolNamesStr := strings.Join(toolNames, ", ")
				if len(toolNames) > 1 {
					toolNamesStr = fmt.Sprintf("%d tools (%s)", len(toolNames), toolNamesStr)
				}
				toolSpinner = spinner.NewToolExecution(os.Stdout, toolNamesStr)
				toolSpinner.Start()
			}

			// Read SSE stream
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()

				// SSE format: data: {...}
				if !strings.HasPrefix(line, "data: ") {
					continue
				}

				jsonData := strings.TrimPrefix(line, "data: ")

				// Parse event
				var event map[string]interface{}
				if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
					logger.WithField("raw", jsonData).Debug("Received non-JSON frame")
					continue
				}

				eventType, _ := event["type"].(string)
				logger.WithFields(logrus.Fields{
					"event_type": eventType,
					"event":      event,
				}).Debug("Received SSE event")

				switch eventType {
				case "RUN_STARTED":
					logger.WithField("event", "RUN_STARTED").Debug("Tool execution started")

				case "TOOL_CALL_START":
					if toolSpinner != nil {
						// Extract tool name from event if available
						if name, ok := event["toolName"].(string); ok {
							toolSpinner.UpdateMessage(fmt.Sprintf("Executing %s: initializing", name))
						} else if name, ok := event["toolCallName"].(string); ok {
							toolSpinner.UpdateMessage(fmt.Sprintf("Executing %s: initializing", name))
						} else if name, ok := event["name"].(string); ok {
							toolSpinner.UpdateMessage(fmt.Sprintf("Executing %s: initializing", name))
						}
					}

				case "TOOL_CALL_ARGS":
					if toolSpinner != nil {
						// Update spinner to show we're processing arguments
						if name, ok := event["toolName"].(string); ok {
							toolSpinner.UpdateMessage(fmt.Sprintf("Executing %s: processing arguments", name))
						} else if name, ok := event["toolCallName"].(string); ok {
							toolSpinner.UpdateMessage(fmt.Sprintf("Executing %s: processing arguments", name))
						} else if name, ok := event["name"].(string); ok {
							toolSpinner.UpdateMessage(fmt.Sprintf("Executing %s: processing arguments", name))
						} else {
							toolSpinner.UpdateMessage("Processing tool arguments...")
						}
					}

				case "TOOL_CALL_END":
					if toolSpinner != nil {
						// Update spinner to show tool completed
						if name, ok := event["toolName"].(string); ok {
							toolSpinner.UpdateMessage(fmt.Sprintf("Completed %s", name))
						} else if name, ok := event["toolCallName"].(string); ok {
							toolSpinner.UpdateMessage(fmt.Sprintf("Completed %s", name))
						} else if name, ok := event["name"].(string); ok {
							toolSpinner.UpdateMessage(fmt.Sprintf("Completed %s", name))
						}
					}
					// Extract tool result if provided directly in the event
					if result, ok := event["result"]; ok {
						toolResults = append(toolResults, result)
						hasResult = true
					}

				case "TOOL_CALL_RESULT":
					if result, ok := event["content"]; ok {
						toolResults = append(toolResults, result)
						hasResult = true
					}

				case "MESSAGES_SNAPSHOT":
					// Extract tool results from messages
					if messages, ok := event["messages"].([]interface{}); ok {
						for _, msg := range messages {
							msgMap, _ := msg.(map[string]interface{})

							// Check for assistant messages with tool calls
							// In tool_based_generative_ui, the results are in the tool call arguments
							if role, _ := msgMap["role"].(string); role == "assistant" {
								if toolCalls, ok := msgMap["toolCalls"].([]interface{}); ok {
									for _, tc := range toolCalls {
										tcMap, _ := tc.(map[string]interface{})
										if fn, ok := tcMap["function"].(map[string]interface{}); ok {
											// Extract the tool name
											toolName, _ := fn["name"].(string)

											// The arguments contain the actual tool result
											if args, ok := fn["arguments"].(string); ok && args != "" && args != "{}" {
												// Parse the arguments as JSON - this contains the result
												var parsedArgs interface{}
												if err := json.Unmarshal([]byte(args), &parsedArgs); err == nil {
													// Store both the tool name and result for proper display
													toolResults = append(toolResults, map[string]interface{}{
														"toolName": toolName,
														"result":   parsedArgs,
													})
													hasResult = true
												}
											}
										}
									}
								}
							}

							// Also check for tool messages (in case server sends them)
							if role, _ := msgMap["role"].(string); role == "tool" {
								if content, ok := msgMap["content"].(string); ok {
									// Parse the content as JSON if possible
									var parsedContent interface{}
									if err := json.Unmarshal([]byte(content), &parsedContent); err == nil {
										toolResults = append(toolResults, parsedContent)
									} else {
										toolResults = append(toolResults, content)
									}
									hasResult = true
								}
							}
						}
					}

				case "RUN_FINISHED":
					if toolSpinner != nil {
						toolSpinner.CompleteWithResult(true)
					}

					// Display results
					if hasResult {
						if jsonOutput {
							// Output JSON result
							output := map[string]interface{}{
								"tools":     toolNames,
								"sessionId": sessionID,
								"results":   toolResults,
								"success":   true,
							}
							encoder := json.NewEncoder(os.Stdout)
							encoder.SetIndent("", "  ")
							encoder.Encode(output)
						} else {
							// Pretty print results for all tools
							for i, result := range toolResults {
								// Extract tool name and result based on format
								var toolName string
								var toolResult interface{}

								// Check if result has the new format with toolName and result
								if resultMap, ok := result.(map[string]interface{}); ok {
									if name, ok := resultMap["toolName"].(string); ok {
										toolName = name
										toolResult = resultMap["result"]
									} else {
										// Fallback to old format
										if i < len(toolNames) {
											toolName = toolNames[i]
										}
										toolResult = result
									}
								} else {
									// Fallback to old format
									if i < len(toolNames) {
										toolName = toolNames[i]
									}
									toolResult = result
								}

								fmt.Fprintf(os.Stdout, "\n🔧 Tool: %s\n", toolName)
								fmt.Fprintln(os.Stdout, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

								// Special formatting for known tools
								if toolName == "generate_haiku" && toolResult != nil {
									if resultMap, ok := toolResult.(map[string]interface{}); ok {
										var japanese []string
										var english []string

										if jp, ok := resultMap["japanese"].([]interface{}); ok {
											for _, line := range jp {
												japanese = append(japanese, fmt.Sprintf("%v", line))
											}
										}
										if en, ok := resultMap["english"].([]interface{}); ok {
											for _, line := range en {
												english = append(english, fmt.Sprintf("%v", line))
											}
										}

										// Use the new box rendering (no ID in tools run context)
										fmt.Fprintln(os.Stdout)
										fmt.Fprintln(os.Stdout, ui.RenderHaikuBox(japanese, english))
									} else {
										// Generic output
										fmt.Fprintln(os.Stdout)
										fmt.Fprintln(os.Stdout, ui.RenderToolResultBox(toolName, toolResult))
									}
								} else if toolResult != nil {
									// Generic tool result with box
									fmt.Fprintln(os.Stdout)
									fmt.Fprintln(os.Stdout, ui.RenderToolResultBox(toolName, toolResult))
								} else {
									fmt.Fprintln(os.Stdout, "   (No result returned)")
								}
							}

							// Interactive prompt for apply/regenerate/cancel
							if interactive && len(toolResults) > 0 {
								p := prompt.New()
								msg := "What would you like to do with these tool results?"
								if len(toolNames) == 1 {
									msg = "What would you like to do with this tool result?"
								}
								action, err := p.AskForAction(msg)
								if err != nil {
									logger.WithError(err).Error("Failed to read user input")
									return fmt.Errorf("failed to read user input: %w", err)
								}

								switch action {
								case prompt.ActionApply:
									fmt.Fprintln(os.Stdout, "\n✅ Tool result applied")
								case prompt.ActionRegenerate:
									fmt.Fprintln(os.Stdout, "\n🔄 Regenerating...")
									// Recursively call the command with same args
									return cmd.RunE(cmd, args)
								case prompt.ActionCancel:
									fmt.Fprintln(os.Stdout, "\n❌ Cancelled")
								}
							}
						}
					} else {
						if !jsonOutput {
							fmt.Fprintln(os.Stdout, "\n⚠️  No result returned from tool")
						}
					}
					return nil

				case "RUN_ERROR":
					if toolSpinner != nil {
						toolSpinner.CompleteWithResult(false)
					}
					errorMsg, _ := event["message"].(string)
					logger.WithField("error", errorMsg).Error("Tool execution failed")
					return fmt.Errorf("tool execution failed: %s", errorMsg)
				}
			}

			// If we get here without a RUN_FINISHED, something went wrong
			if toolSpinner != nil {
				toolSpinner.StopWithMessage("⚠️  Connection lost")
			}

			if !hasResult {
				logger.Error("Tool execution did not return a result")
				return fmt.Errorf("tool execution did not return a result")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&argsJSON, "args", "", "Tool arguments as JSON string")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID for context")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results in JSON format")
	cmd.Flags().BoolVar(&interactive, "interactive", true, "Enable interactive prompts")
	cmd.Flags().IntVar(&timeout, "timeout", 30, "Request timeout in seconds")
	cmd.Flags().BoolVar(&skipValidation, "skip-validation", false, "Skip JSON schema validation of tool arguments")
	cmd.Flags().BoolVar(&clientTools, "client-tools", false, "Enable client-side tool execution")
	cmd.Flags().StringVar(&toolsDir, "tools-dir", "", "Directory containing custom tool definitions (JSON files)")
	cmd.Flags().StringVar(&onError, "on-error", "prompt", "Error handling policy: retry, skip, abort, or prompt")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 3, "Maximum number of retry attempts")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed error information")

	return cmd
}

func newChatCommand() *cobra.Command {
	var message string
	var sessionID string
	var jsonOutput bool
	var noColor bool
	var systemPrompt string
	var model string
	var temperature float64
	var maxTokens int
	var interactive bool
	var streaming bool
	var humanLoop bool
	var stateMode bool
	var predictiveMode bool
	var sharedStateMode bool
	var optimizedStreaming bool
	var bufferSize string
	var showMetrics bool
	var clientTools bool
	var toolsDir string
	var resumeSession bool

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Send chat messages to AG-UI",
		Long: `Send chat messages to AG-UI server and receive responses.

Examples:
  # Send a simple message
  ag-ui-client chat --message "Hello, AG-UI!"
  
  # Send a message with streaming text output
  ag-ui-client chat --message "Tell me a story" --streaming
  
  # Use human-in-the-loop for tool approval
  ag-ui-client chat --message "Generate a haiku" --human-loop
  
  # Use state management endpoint
  ag-ui-client chat --message "Update my preferences" --state-mode
  
  # Use predictive updates endpoint
  ag-ui-client chat --message "Predict next steps" --predictive
  
  # Use shared state synchronization
  ag-ui-client chat --message "Sync state" --shared-state
  
  # Use client-side tool execution
  ag-ui-client chat --message "Read the README file" --client-tools
  
  # Load custom tools from directory
  ag-ui-client chat --message "Run my custom tool" --client-tools --tools-dir ./my-tools
  
  # Resume a previous conversation with context
  ag-ui-client chat --message "What did we discuss earlier?" --resume --session my-session-id
  
  # Send a message in a specific session
  ag-ui-client chat --message "Continue our work" --session-id abc123
  
  # Get response in JSON format
  ag-ui-client chat --message "What tools are available?" --json
  
  # Use message as argument
  ag-ui-client chat "Quick question about the API"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use argument as message if provided
			if len(args) > 0 && message == "" {
				message = args[0]
			}

			// Require a message
			if message == "" {
				return clienterrors.NewValidationError("chat", "Message is required. Use --message flag or provide as argument")
			}

			cfg := configManager.GetConfig()

			if cfg.ServerURL == "" {
				return clienterrors.NewToolError(clienterrors.CategoryValidation, clienterrors.SeverityError, "Server URL not configured. Use --server flag or set AGUI_SERVER environment variable")
			}

			// Create session store
			sessionStore := session.NewStore(configManager.GetConfigPath())

			// If no session ID provided, try to use the active session
			if sessionID == "" {
				activeSession, err := sessionStore.GetActiveSession()
				if err == nil && activeSession != nil {
					sessionID = activeSession.ThreadID
					logger.WithField("threadId", sessionID).Debug("Using active session")
				} else {
					// Generate a new session ID if none exists
					sessionID = uuid.New().String()
					logger.WithField("threadId", sessionID).Debug("Generated new session ID")
					// Set this as the active session
					if err := sessionStore.SetActiveSession(sessionID, "chat-session"); err != nil {
						logger.WithError(err).Warn("Failed to set active session")
					}
				}
			} else {
				// Session ID was provided, set it as active
				if err := sessionStore.SetActiveSession(sessionID, "chat-session"); err != nil {
					logger.WithError(err).Warn("Failed to set active session")
				}
			}

			// Create persistent store for session history
			persistentStore := session.NewPersistentStore(configManager.GetConfigPath())

			// Load existing session history if available
			existingMessages, err := persistentStore.GetSessionHistory(sessionID)
			if err != nil {
				// Create new session if not found
				_, err = persistentStore.CreateSession(sessionID, "chat-session")
				if err != nil {
					logger.WithError(err).Warn("Failed to create session")
				}
				existingMessages = []session.ConversationMessage{}
			}

			// Initialize prompt handler for interactive mode
			var promptHandler *prompt.InteractivePrompt
			if interactive && !jsonOutput {
				promptHandler = prompt.New()
			}

			// Initialize client-side tools if enabled
			var clientToolRegistry *clienttools.Registry
			if clientTools {
				clientToolRegistry = clienttools.NewRegistry()

				// Register built-in tools
				if err := clientToolRegistry.RegisterBuiltinTools(); err != nil {
					logger.WithError(err).Error("Failed to register built-in client tools")
					return clienterrors.NewToolError(clienterrors.CategoryTool, clienterrors.SeverityError, "Failed to register built-in client tools")
				}

				// Load custom tools from directory if specified
				if toolsDir != "" {
					if err := clientToolRegistry.LoadToolsFromDirectory(toolsDir); err != nil {
						logger.WithError(err).Warn("Failed to load custom tools from directory")
					}
				}

				logger.WithField("toolCount", len(clientToolRegistry.ListTools())).Info("Client-side tools initialized")
			}

			// Track tool results for interactive mode
			var lastToolResults []map[string]interface{}
			var currentAssistantMessage map[string]interface{}

			// Build conversation history
			conversationHistory := []map[string]interface{}{}

			// Include existing messages if resuming session
			if resumeSession && len(existingMessages) > 0 {
				logger.WithField("messageCount", len(existingMessages)).Info("Resuming session with previous messages")
				if !jsonOutput {
					fmt.Printf("📚 Resuming session with %d previous messages\n", len(existingMessages))
				}

				// Convert existing messages to API format
				for _, msg := range existingMessages {
					apiMsg := map[string]interface{}{
						"id":      msg.ID,
						"role":    msg.Role,
						"content": msg.Content,
					}

					// Add tool calls if present
					if len(msg.ToolCalls) > 0 {
						toolCalls := []map[string]interface{}{}
						for _, tc := range msg.ToolCalls {
							toolCall := map[string]interface{}{
								"id":   tc.ID,
								"type": tc.Type,
								"function": map[string]interface{}{
									"name":      tc.Function.Name,
									"arguments": tc.Function.Arguments,
								},
							}
							toolCalls = append(toolCalls, toolCall)
						}
						apiMsg["toolCalls"] = toolCalls
					}

					// Add tool call ID for tool messages
					if msg.ToolCallID != "" {
						apiMsg["toolCallId"] = msg.ToolCallID
					}

					conversationHistory = append(conversationHistory, apiMsg)
				}
			}

			// Add the new user message
			// Note: human_in_the_loop endpoint requires id field for UserMessage
			userMessage := map[string]interface{}{
				"id":      fmt.Sprintf("msg-%d", time.Now().UnixNano()),
				"role":    "user",
				"content": message,
			}
			conversationHistory = append(conversationHistory, userMessage)

			// Save original interactive flag value for tool detection
			interactiveFlag := interactive

			// Main interaction loop for regeneration support
		regenerateLoop:
			for {
				// Build the endpoint URL based on mode flags
				var endpoint string
				if streaming {
					endpoint = strings.TrimSuffix(cfg.ServerURL, "/") + "/agentic_chat"
					// Note: agentic_chat now supports tools via streaming events
					// Interactive mode will be enabled if tools are detected
				} else if humanLoop {
					endpoint = strings.TrimSuffix(cfg.ServerURL, "/") + "/human_in_the_loop"
				} else if stateMode {
					endpoint = strings.TrimSuffix(cfg.ServerURL, "/") + "/agentic_generative_ui"
				} else if predictiveMode {
					endpoint = strings.TrimSuffix(cfg.ServerURL, "/") + "/predictive_state_updates"
				} else if sharedStateMode {
					endpoint = strings.TrimSuffix(cfg.ServerURL, "/") + "/shared_state"
				} else {
					endpoint = strings.TrimSuffix(cfg.ServerURL, "/") + "/tool_based_generative_ui"
				}

				// Initialize metrics if requested
				var streamMetrics *streamingpkg.StreamMetrics
				if showMetrics {
					streamMetrics = streamingpkg.NewStreamMetrics()
					defer func() {
						if streamMetrics != nil {
							snapshot := streamMetrics.GetSnapshot()
							fmt.Fprintln(os.Stderr, streamingpkg.FormatMetrics(snapshot))
						}
					}()
				}

				// Configure SSE client
				authHeader := cfg.AuthHeader
				if authHeader == "" {
					authHeader = "Authorization"
				}
				authScheme := cfg.AuthScheme
				if authScheme == "" && authHeader == "Authorization" {
					authScheme = "Bearer"
				}

				// Build the request payload first
				runID := fmt.Sprintf("run-%d", time.Now().UnixNano())

				// Include state data for state-related endpoints
				state := map[string]interface{}{}
				if stateMode || sharedStateMode {
					state = map[string]interface{}{
						"preferences": map[string]interface{}{
							"theme":    "dark",
							"language": "en",
						},
						"counter": 0,
					}
				}

				payload := map[string]interface{}{
					"threadId":       sessionID,
					"runId":          runID,
					"messages":       conversationHistory,
					"state":          state,
					"tools":          []interface{}{},
					"context":        []interface{}{},
					"forwardedProps": map[string]interface{}{},
				}

				// Add optional parameters
				if systemPrompt != "" {
					payload["systemPrompt"] = systemPrompt
				}
				if model != "" {
					payload["model"] = model
				}
				if temperature > 0 {
					payload["temperature"] = temperature
				}
				if maxTokens > 0 {
					payload["maxTokens"] = maxTokens
				}

				// Create context with timeout
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				// Handle interrupts gracefully
				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
				defer signal.Stop(sigChan) // Clean up signal handler

				go func() {
					select {
					case <-sigChan:
						logger.Debug("Received interrupt signal, closing stream...")
						cancel()
					case <-ctx.Done():
						return // Exit goroutine when context is done
					}
				}()

				// Configure output renderer first (needed for spinners)
				outputMode := ui.OutputModePretty
				if jsonOutput || cfg.Output == "json" {
					outputMode = ui.OutputModeJSON
				}

				// Track visual feedback spinners
				var connectionSpinner *spinner.ConnectionSpinner

				// Choose between optimized and standard streaming
				var frames <-chan sse.Frame
				var errors <-chan error

				if optimizedStreaming {
					// Use optimized streaming with configurable buffer
					bufSize := streamingpkg.MediumBuffer
					switch bufferSize {
					case "small":
						bufSize = streamingpkg.SmallBuffer
					case "medium":
						bufSize = streamingpkg.MediumBuffer
					case "large":
						bufSize = streamingpkg.LargeBuffer
					case "max":
						bufSize = streamingpkg.MaxBuffer
					}

					optimizedConfig := &streamingpkg.StreamConfig{
						BufferSize:        bufSize,
						MaxRetries:        3,
						RetryDelay:        time.Second,
						MaxRetryDelay:     30 * time.Second,
						ConnectionTimeout: 30 * time.Second,
						ReadTimeout:       5 * time.Minute,
						KeepAliveInterval: 30 * time.Second,
						Logger:            logger,
					}

					optimizedStream := streamingpkg.NewOptimizedStream(optimizedConfig)
					defer optimizedStream.Close()

					// Create HTTP request
					reqBody, _ := json.Marshal(payload)
					req, err := http.NewRequest("POST", endpoint, bytes.NewReader(reqBody))
					if err != nil {
						logger.WithError(err).Error("Failed to create request")
						return clienterrors.NewToolError(clienterrors.CategoryNetwork, clienterrors.SeverityError, "Failed to create request")
					}

					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Accept", "text/event-stream")
					if cfg.APIKey != "" {
						if authScheme != "" {
							req.Header.Set(authHeader, fmt.Sprintf("%s %s", authScheme, cfg.APIKey))
						} else {
							req.Header.Set(authHeader, cfg.APIKey)
						}
					}

					// Connect with optimized streaming
					eventChan, errorChan, err := optimizedStream.Connect(ctx, req)
					if err != nil {
						logger.WithError(err).Error("Failed to establish optimized SSE connection")
						return clienterrors.NewNetworkError("Failed to establish optimized SSE connection", true)
					}

					// Convert optimized events to standard frames
					frameChan := make(chan sse.Frame, 100)
					go func() {
						defer close(frameChan)
						for event := range eventChan {
							if streamMetrics != nil {
								streamMetrics.RecordEvent(int64(len(event.Data)))
								// Record latency (simplified - just measure processing time)
								start := time.Now()
								defer func() {
									streamMetrics.RecordLatency(time.Since(start).Microseconds())
								}()
							}

							frameChan <- sse.Frame{
								Data:      event.Data,
								Timestamp: time.Now(),
							}
						}
					}()

					frames = frameChan
					errors = errorChan

					if streamMetrics != nil {
						streamMetrics.RecordConnectionOpened()
					}

					logger.WithFields(logrus.Fields{
						"buffer_size": bufSize,
						"optimized":   true,
					}).Info("Using optimized SSE streaming")

				} else {
					// Use standard SSE client
					sseConfig := sse.Config{
						Endpoint:       endpoint,
						APIKey:         cfg.APIKey,
						AuthHeader:     authHeader,
						AuthScheme:     authScheme,
						ConnectTimeout: 30 * time.Second,
						ReadTimeout:    5 * time.Minute,
						BufferSize:     100,
						Logger:         logger,
					}

					// Show connection spinner for visual feedback
					if outputMode == ui.OutputModePretty && !jsonOutput {
						connectionSpinner = spinner.NewConnection(cmd.OutOrStdout(), endpoint)
						connectionSpinner.SetConnecting()
					}

					client := sse.NewClient(sseConfig)
					defer func() {
						client.Close()
						if connectionSpinner != nil {
							connectionSpinner.CompleteConnection(true)
						}
					}()

					logger.WithFields(logrus.Fields{
						"endpoint":   endpoint,
						"session_id": sessionID,
						"run_id":     runID,
						"streaming":  streaming,
					}).Debug("Connecting to SSE stream")

					// Start the SSE stream
					var err error
					frames, errors, err = client.Stream(sse.StreamOptions{
						Context: ctx,
						Payload: payload,
					})

					if err != nil {
						if connectionSpinner != nil {
							connectionSpinner.SetError(err)
							connectionSpinner.CompleteConnection(false)
						}
						logger.WithError(err).Error("Failed to establish SSE connection")
						return clienterrors.NewNetworkError("Failed to establish SSE connection", true)
					}

					// Mark connection as established
					if connectionSpinner != nil {
						connectionSpinner.SetConnected()
						connectionSpinner.CompleteConnection(true)
					}
				}

				// Configure output renderer (outputMode already configured above)
				renderer := ui.NewRenderer(ui.RendererConfig{
					OutputMode: outputMode,
					NoColor:    noColor,
					Quiet:      false,
					Writer:     cmd.OutOrStdout(),
				})

				// Track if we've received any content
				hasContent := false
				assistantMessage := strings.Builder{}
				lastToolResults = []map[string]interface{}{}
				currentAssistantMessage = nil

				// Track tool execution state
				var toolSpinner *spinner.ToolExecutionSpinner
				var thinkingSpinner *spinner.Spinner
				var streamingSpinner *spinner.StreamingSpinner
				var multiToolSpinner *spinner.MultiToolSpinner
				toolStartTime := time.Time{}
				var currentToolName string
				streamChunkCount := 0
				streamByteCount := int64(0)

				// Process SSE events
				for {
					select {
					case frame, ok := <-frames:
						if !ok {
							// Stream closed
							if !hasContent && outputMode == ui.OutputModePretty {
								fmt.Fprintln(os.Stdout, "\nNo response received from server")
							}
							return nil
						}

						// Parse the SSE event
						var event map[string]interface{}
						if err := json.Unmarshal(frame.Data, &event); err != nil {
							logger.WithField("raw", string(frame.Data)).Debug("Received non-JSON frame")
							continue
						}

						// Extract event type - the server sends it as "type" field directly
						eventType, _ := event["type"].(string)
						// The event data is the event object itself
						eventData := event

						// Handle specific events for chat display
						switch eventType {
						case "RUN_STARTED":
							if outputMode == ui.OutputModePretty {
								fmt.Fprintln(os.Stdout, "")
							} else if outputMode == ui.OutputModeJSON {
								// Pass to renderer for JSON output
								eventDataBytes, _ := json.Marshal(eventData)
								renderer.HandleEvent(eventType, eventDataBytes)
							}

						case "TEXT_MESSAGE_START":
							hasContent = true
							if outputMode == ui.OutputModePretty {
								role, _ := eventData["role"].(string)
								if role == "assistant" {
									fmt.Fprint(os.Stdout, "Assistant: ")
									// Start streaming spinner for text generation
									if streamingSpinner == nil && !jsonOutput {
										streamingSpinner = spinner.NewStreaming(cmd.OutOrStdout(), "Generating response")
										streamingSpinner.StartStreaming()
									}
								}
							}

						case "TEXT_MESSAGE_CONTENT", "TEXT_MESSAGE_CHUNK":
							// Handle both regular content and chunked text messages
							// TEXT_MESSAGE_CHUNK is used for progressive text streaming
							// TEXT_MESSAGE_CONTENT is used for complete text segments

							// Try "delta" field first (used by agentic_chat and chunks), then "content"
							content, ok := eventData["delta"].(string)
							if !ok {
								content, _ = eventData["content"].(string)
							}

							// For TEXT_MESSAGE_CHUNK specifically, we may also have messageId and role
							if eventType == "TEXT_MESSAGE_CHUNK" {
								messageId, _ := eventData["messageId"].(string)
								role, _ := eventData["role"].(string)

								// Log chunk details for debugging
								if messageId != "" || role != "" {
									logger.WithFields(logrus.Fields{
										"eventType":   "TEXT_MESSAGE_CHUNK",
										"messageId":   messageId,
										"role":        role,
										"deltaLength": len(content),
									}).Debug("Received text message chunk")
								}
							}

							// Accumulate content and display progressively
							assistantMessage.WriteString(content)
							if outputMode == ui.OutputModePretty {
								fmt.Fprint(cmd.OutOrStdout(), content)
								// Update streaming spinner with progress
								if streamingSpinner != nil && content != "" {
									streamChunkCount++
									streamByteCount += int64(len(content))
									streamingSpinner.UpdateProgress(int64(len(content)), 1)
								}
							}

						case "TEXT_MESSAGE_END":
							if outputMode == ui.OutputModePretty {
								fmt.Fprintln(cmd.OutOrStdout(), "")
								// Complete streaming spinner
								if streamingSpinner != nil {
									streamingSpinner.CompleteStreaming(true)
									streamingSpinner = nil
								}
							}

						case "TOOL_CALL_START":
							hasContent = true
							// Enable interactive mode when tools are detected in streaming
							if streaming && !interactive && interactiveFlag {
								interactive = true
								logger.Debug("Enabling interactive mode - tools detected in streaming")
							}

							if outputMode == ui.OutputModePretty {
								// Try both field names for compatibility with different endpoints
								toolName, _ := eventData["toolName"].(string)
								if toolName == "" {
									toolName, _ = eventData["toolCallName"].(string)
								}
								if toolName == "" {
									toolName, _ = eventData["name"].(string)
								}
								currentToolName = toolName
								toolStartTime = time.Now()

								// Update MultiToolSpinner if active, otherwise use regular tool spinner
								if multiToolSpinner != nil {
									multiToolSpinner.StartTool(toolName)
								} else {
									// Create and start spinner for tool execution
									toolSpinner = spinner.NewToolExecution(cmd.OutOrStdout(), toolName)
									toolSpinner.StartWithPhase("initializing")
								}
							}

							// Track tool call for results
							toolCallId, _ := eventData["toolCallId"].(string)
							if toolCallId == "" {
								toolCallId, _ = eventData["id"].(string)
							}

							// Initialize tool call tracking
							currentToolCall := map[string]interface{}{
								"id":   toolCallId,
								"type": "function",
								"function": map[string]interface{}{
									"name":      currentToolName,
									"arguments": "{}",
								},
							}
							lastToolResults = append(lastToolResults, currentToolCall)

						case "TOOL_CALL_ARGS":
							if outputMode == ui.OutputModePretty && toolSpinner != nil {
								// Update with current tool name
								toolSpinner.Spinner.UpdateMessage(fmt.Sprintf("Executing %s: processing arguments", currentToolName))
							}

							// Accumulate streaming arguments
							if len(lastToolResults) > 0 {
								lastTool := lastToolResults[len(lastToolResults)-1]
								if fn, ok := lastTool["function"].(map[string]interface{}); ok {
									// Handle both delta and chunk fields for arguments
									delta, hasDelta := eventData["delta"].(string)
									chunk, hasChunk := eventData["chunk"].(string)
									args, hasArgs := eventData["args"].(string)

									currentArgs, _ := fn["arguments"].(string)
									if hasDelta {
										currentArgs += delta
									} else if hasChunk {
										currentArgs += chunk
									} else if hasArgs {
										currentArgs = args
									}
									fn["arguments"] = currentArgs
								}
							}

						case "TOOL_CALL_CHUNK":
							// Handle chunked tool call data (progressive streaming of tool arguments)
							if outputMode == ui.OutputModePretty {
								// Extract relevant fields from the chunk event
								toolCallId, _ := eventData["toolCallId"].(string)
								toolCallName, _ := eventData["toolCallName"].(string)
								delta, _ := eventData["delta"].(string)

								// If we have a delta, it's progressive argument data
								if delta != "" {
									// Update spinner message to show we're receiving chunks
									if toolSpinner != nil && toolCallName != "" {
										toolSpinner.Spinner.UpdateMessage(fmt.Sprintf("Executing %s: streaming arguments...", toolCallName))
									} else if toolSpinner != nil && currentToolName != "" {
										toolSpinner.Spinner.UpdateMessage(fmt.Sprintf("Executing %s: streaming arguments...", currentToolName))
									}

									// In a production implementation, you would accumulate these chunks
									// to build the complete tool arguments progressively
									logger.WithFields(logrus.Fields{
										"toolCallId":   toolCallId,
										"toolCallName": toolCallName,
										"deltaLength":  len(delta),
									}).Debug("Received tool call chunk")
								}
							}

						case "TOOL_CALL_END":
							if outputMode == ui.OutputModePretty {
								// Extract tool name if available
								toolName, _ := eventData["toolName"].(string)
								if toolName == "" {
									toolName, _ = eventData["toolCallName"].(string)
								}
								if toolName == "" {
									toolName = currentToolName
								}

								// Update appropriate spinner
								if multiToolSpinner != nil {
									multiToolSpinner.CompleteTool(toolName, true, nil)
								} else if toolSpinner != nil {
									// Stop spinner and show completion
									toolSpinner.CompleteWithResult(true)
									toolSpinner = nil
								}
							}

							// Mark tool as completed for interactive mode
							if len(lastToolResults) > 0 {
								// Create assistant message with tool calls for history
								if streaming && currentAssistantMessage == nil {
									currentAssistantMessage = map[string]interface{}{
										"role":      "assistant",
										"toolCalls": lastToolResults,
									}
								}
							}

						case "TOOL_CALL_RESULT":
							hasContent = true
							if outputMode == ui.OutputModePretty {
								// Stop spinner and show completion
								if toolSpinner != nil {
									toolSpinner.CompleteWithResult(true)
									toolSpinner = nil

									// Show execution time
									if !toolStartTime.IsZero() {
										elapsed := time.Since(toolStartTime)
										fmt.Fprintf(os.Stdout, "   ⏱️  Completed in %.2fs\n", elapsed.Seconds())
									}
								}

								// Extract tool name if available
								toolName, _ := eventData["toolName"].(string)
								if toolName == "" {
									// Try "name" field (alternative field name)
									toolName, _ = eventData["name"].(string)
								}
								if toolName == "" {
									// Try to extract from current tool name
									toolName = currentToolName
								}

								// Extract tool call ID if available
								toolCallID, _ := eventData["toolCallId"].(string)

								// Display the result with proper formatting
								if result, ok := eventData["result"]; ok {
									// Check if this is a haiku result
									if toolName == "generate_haiku" {
										if resultStr, ok := result.(string); ok {
											var resultObj map[string]interface{}
											if err := json.Unmarshal([]byte(resultStr), &resultObj); err == nil {
												result = resultObj
											}
										}

										if resultMap, ok := result.(map[string]interface{}); ok {
											var japanese []string
											var english []string

											if jp, ok := resultMap["japanese"].([]interface{}); ok {
												for _, line := range jp {
													japanese = append(japanese, fmt.Sprintf("%v", line))
												}
											}
											if en, ok := resultMap["english"].([]interface{}); ok {
												for _, line := range en {
													english = append(english, fmt.Sprintf("%v", line))
												}
											}

											// Use the new box rendering with ID
											fmt.Fprintln(cmd.OutOrStdout())
											fmt.Fprintln(cmd.OutOrStdout(), ui.RenderHaikuBoxWithID(japanese, english, toolCallID))
										} else {
											// Fallback for unexpected format
											fmt.Fprintln(cmd.OutOrStdout())
											fmt.Fprintln(cmd.OutOrStdout(), ui.RenderToolResultBox(toolName, result))
										}
									} else {
										// Generic tool result
										fmt.Fprintln(cmd.OutOrStdout())
										fmt.Fprintln(cmd.OutOrStdout(), ui.RenderToolResultBox(toolName, result))
									}
								} else if content, ok := eventData["content"]; ok {
									// Alternative field name for result
									fmt.Fprintln(cmd.OutOrStdout())
									fmt.Fprintln(cmd.OutOrStdout(), ui.RenderToolResultBox(toolName, content))
								}
							}
							if outputMode == ui.OutputModeJSON {
								jsonData, _ := json.Marshal(eventData)
								fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
							}

						case "MESSAGES_SNAPSHOT":
							// Handle complete messages snapshot
							hasContent = true
							if messages, ok := eventData["messages"].([]interface{}); ok {
								// First, scan for all tool calls to detect multiple tools
								var allToolNames []string
								for _, msg := range messages {
									if msgMap, ok := msg.(map[string]interface{}); ok {
										if role, _ := msgMap["role"].(string); role == "assistant" {
											if toolCalls, ok := msgMap["toolCalls"].([]interface{}); ok {
												for _, tc := range toolCalls {
													if tcMap, ok := tc.(map[string]interface{}); ok {
														if fn, ok := tcMap["function"].(map[string]interface{}); ok {
															if name, ok := fn["name"].(string); ok {
																allToolNames = append(allToolNames, name)
															}
														}
													}
												}
											}
										}
									}
								}

								// Initialize MultiToolSpinner if multiple tools detected
								if len(allToolNames) > 1 && multiToolSpinner == nil && outputMode == ui.OutputModePretty && !jsonOutput {
									multiToolSpinner = spinner.NewMultiTool(cmd.OutOrStdout(), allToolNames)
									multiToolSpinner.Start()
								}

								// Check for backend-executed tools
								// Process tool messages for display (not just in streaming mode)
								for _, msg := range messages {
									if msgMap, ok := msg.(map[string]interface{}); ok {
										role, _ := msgMap["role"].(string)

										// Detect assistant messages with tool calls (backend execution)
										if role == "assistant" {
											if toolCalls, ok := msgMap["toolCalls"].([]interface{}); ok && len(toolCalls) > 0 {
												// Enable interactive mode for backend tools
												if !interactive && interactiveFlag {
													interactive = true
													logger.Debug("Enabling interactive mode - backend tools detected in streaming")
												}

												// Store tool results for interactive mode
												currentAssistantMessage = msgMap
												for _, tc := range toolCalls {
													if tcMap, ok := tc.(map[string]interface{}); ok {
														lastToolResults = append(lastToolResults, tcMap)
													}
												}

												// Display tool information
												if outputMode == ui.OutputModePretty {
													fmt.Fprintln(cmd.OutOrStdout(), "\n🔧 Backend tool execution detected:")
													for _, tc := range toolCalls {
														if tcMap, ok := tc.(map[string]interface{}); ok {
															if fn, ok := tcMap["function"].(map[string]interface{}); ok {
																name, _ := fn["name"].(string)
																fmt.Fprintf(cmd.OutOrStdout(), "   - %s\n", name)
															}
														}
													}
												}
											}
										}

										// Detect tool result messages
										if role == "tool" {
											if outputMode == ui.OutputModePretty {
												content, _ := msgMap["content"].(string)
												toolCallId, _ := msgMap["toolCallId"].(string)
												fmt.Fprintf(cmd.OutOrStdout(), "\n📋 Tool Result (ID: %s):\n", toolCallId)

												// Try to parse and display JSON content nicely
												var result interface{}
												if err := json.Unmarshal([]byte(content), &result); err == nil {
													if resultMap, ok := result.(map[string]interface{}); ok {
														// Special handling for haiku tool
														var japanese []string
														var english []string

														if jp, ok := resultMap["japanese"].([]interface{}); ok {
															for _, line := range jp {
																japanese = append(japanese, fmt.Sprintf("%v", line))
															}
														}
														if en, ok := resultMap["english"].([]interface{}); ok {
															for _, line := range en {
																english = append(english, fmt.Sprintf("%v", line))
															}
														}

														if len(japanese) > 0 || len(english) > 0 {
															// Use the new box rendering for haiku
															fmt.Fprintln(cmd.OutOrStdout())
															fmt.Fprintln(cmd.OutOrStdout(), ui.RenderHaikuBox(japanese, english))
														} else {
															// Generic display
															formatted, _ := json.MarshalIndent(result, "   ", "  ")
															fmt.Fprintln(cmd.OutOrStdout(), string(formatted))
														}
													} else {
														// Generic display
														formatted, _ := json.MarshalIndent(result, "   ", "  ")
														fmt.Fprintln(cmd.OutOrStdout(), string(formatted))
													}
												} else {
													// Plain text result
													fmt.Fprintf(cmd.OutOrStdout(), "   %s\n", content)
												}
											}
										}
									}
								}

								// Save messages to persistent store
								for _, msg := range messages {
									if msgMap, ok := msg.(map[string]interface{}); ok {
										// Convert to ConversationMessage
										convMsg := session.ConversationMessage{
											ID:        fmt.Sprintf("%v", msgMap["id"]),
											Role:      fmt.Sprintf("%v", msgMap["role"]),
											Timestamp: time.Now(),
										}

										// Add content if present
										if content, ok := msgMap["content"].(string); ok {
											convMsg.Content = content
										}

										// Add tool calls if present
										if toolCalls, ok := msgMap["toolCalls"].([]interface{}); ok {
											for _, tc := range toolCalls {
												if tcMap, ok := tc.(map[string]interface{}); ok {
													toolCall := session.ToolCall{
														ID:   fmt.Sprintf("%v", tcMap["id"]),
														Type: fmt.Sprintf("%v", tcMap["type"]),
													}
													if fn, ok := tcMap["function"].(map[string]interface{}); ok {
														toolCall.Function = session.FunctionCall{
															Name:      fmt.Sprintf("%v", fn["name"]),
															Arguments: fmt.Sprintf("%v", fn["arguments"]),
														}
													}
													convMsg.ToolCalls = append(convMsg.ToolCalls, toolCall)
												}
											}
										}

										// Add tool call ID if present (for tool results)
										if toolCallID, ok := msgMap["toolCallId"].(string); ok {
											convMsg.ToolCallID = toolCallID
										}

										// Check if message is already in history before saving
										alreadyExists := false
										for _, existing := range existingMessages {
											if existing.ID == convMsg.ID {
												alreadyExists = true
												break
											}
										}

										if !alreadyExists {
											// Save message to persistent store
											err := persistentStore.AddMessage(sessionID, convMsg)
											if err != nil {
												logger.WithError(err).Warn("Failed to save message to session history")
											} else {
												// Add to existing messages to avoid duplicates
												existingMessages = append(existingMessages, convMsg)
											}
										}
									}
								}

								// Create a map to track tool calls by ID for linking with results
								toolCallMap := make(map[string]map[string]interface{})

								// First pass: collect all tool calls
								for _, msg := range messages {
									if msgMap, ok := msg.(map[string]interface{}); ok {
										role, _ := msgMap["role"].(string)
										if role == "assistant" {
											if toolCalls, ok := msgMap["toolCalls"].([]interface{}); ok {
												for _, tc := range toolCalls {
													if tcMap, ok := tc.(map[string]interface{}); ok {
														if id, ok := tcMap["id"].(string); ok {
															toolCallMap[id] = tcMap
														}
													}
												}
											}
										}
									}
								}

								// Second pass: process messages
								for _, msg := range messages {
									if msgMap, ok := msg.(map[string]interface{}); ok {
										role, _ := msgMap["role"].(string)

										switch role {
										case "assistant":
											// Check if it has content
											if content, ok := msgMap["content"].(string); ok && content != "" {
												assistantMessage.WriteString(content)
												if outputMode == ui.OutputModePretty {
													fmt.Fprintf(os.Stdout, "\nAssistant: %s\n", content)
												}
											}
											// Check if it has tool calls
											if toolCalls, ok := msgMap["toolCalls"].([]interface{}); ok && len(toolCalls) > 0 {
												currentAssistantMessage = msgMap

												// Validate tool calls if in pretty mode (not JSON output)
												if outputMode == ui.OutputModePretty {
													// Try to fetch tool schemas for validation (best effort)
													if availableTools, err := fetchToolsFromServer(cfg); err == nil {
														for _, tc := range toolCalls {
															if tcMap, ok := tc.(map[string]interface{}); ok {
																if fn, ok := tcMap["function"].(map[string]interface{}); ok {
																	toolName, _ := fn["name"].(string)
																	argsStr, _ := fn["arguments"].(string)

																	// Find tool schema
																	for _, tool := range availableTools {
																		if tool.Name == toolName {
																			if tool.Parameters != nil {
																				toolSchema := convertToToolSchema(tool.Parameters)
																				// Validate arguments
																				if err := pkgtools.ValidateArguments(json.RawMessage(argsStr), toolSchema); err != nil {
																					logger.WithFields(logrus.Fields{
																						"tool":  toolName,
																						"error": err.Error(),
																					}).Warn("Tool argument validation failed")
																				}
																			}
																			break
																		}
																	}
																}
															}
														}
													}
												}

												// Store tool results for interactive mode
												for _, tc := range toolCalls {
													if tcMap, ok := tc.(map[string]interface{}); ok {
														lastToolResults = append(lastToolResults, tcMap)
													}
												}

												if outputMode == ui.OutputModePretty {
													fmt.Fprintln(os.Stdout, "\nAssistant is using tools:")

													// Process each tool call with visual feedback
													for _, tc := range toolCalls {
														if tcMap, ok := tc.(map[string]interface{}); ok {
															toolCallId, _ := tcMap["id"].(string)
															if function, ok := tcMap["function"].(map[string]interface{}); ok {
																name, _ := function["name"].(string)
																args, _ := function["arguments"].(string)

																// Check if this tool should be executed locally
																if clientToolRegistry != nil {
																	if _, exists := clientToolRegistry.GetTool(name); exists {
																		// Execute tool locally
																		logger.WithField("tool", name).Debug("Executing tool locally")

																		// Parse arguments
																		var argsMap map[string]interface{}
																		if err := json.Unmarshal([]byte(args), &argsMap); err != nil {
																			logger.WithError(err).Error("Failed to parse tool arguments")
																			continue
																		}

																		// Show spinner for local tool execution
																		toolSpinner := spinner.NewToolExecution(os.Stdout, name)
																		toolSpinner.Start()

																		// Execute the tool
																		execCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
																		execResult, err := clientToolRegistry.ExecuteTool(execCtx, name, argsMap)
																		cancel()

																		// Stop spinner and show result
																		if err != nil {
																			toolSpinner.CompleteWithResult(false)
																			fmt.Fprintf(os.Stdout, "   ❌ Local tool execution failed: %v\n", err)
																		} else {
																			toolSpinner.CompleteWithResult(true)

																			// Display the result
																			if execResult != nil {
																				fmt.Fprintf(os.Stdout, "\n   📋 Local Tool Result (ID: %s):\n", toolCallId)

																				// Format and display the result
																				if execResult.Result != nil {
																					resultJSON, _ := json.MarshalIndent(execResult.Result, "      ", "  ")
																					fmt.Fprintf(os.Stdout, "      %s\n", string(resultJSON))
																				}

																				// Show execution time
																				fmt.Fprintf(os.Stdout, "   ⏱️  Completed in %.2fs\n\n", execResult.Duration.Seconds())

																				// TODO: Send result back to server as tool response message
																				// This would require injecting a new message into the conversation
																			}
																		}

																		// Skip server-side execution for this tool
																		continue
																	}
																}

																// Show spinner for tool execution (simulated since server processes instantly)
																toolSpinner := spinner.NewToolExecution(os.Stdout, name)
																toolSpinner.Start()

																// Simulate brief processing time for visual feedback
																time.Sleep(500 * time.Millisecond)
																toolSpinner.UpdateMessage(fmt.Sprintf("Executing %s: processing", name))
																time.Sleep(300 * time.Millisecond)

																// Stop spinner and show success
																toolSpinner.CompleteWithResult(true)

																// Try to parse and pretty-print the arguments
																var argsObj map[string]interface{}
																if err := json.Unmarshal([]byte(args), &argsObj); err == nil {
																	fmt.Fprintf(os.Stdout, "  🔧 %s", name)
																	if toolCallId != "" {
																		fmt.Fprintf(os.Stdout, " (ID: %s)", toolCallId)
																	}
																	fmt.Fprintln(os.Stdout, ":")
																	// Display the haiku if it's a generate_haiku call
																	if name == "generate_haiku" {
																		var japanese []string
																		var english []string

																		if jp, ok := argsObj["japanese"].([]interface{}); ok {
																			for _, line := range jp {
																				japanese = append(japanese, fmt.Sprintf("%v", line))
																			}
																		}
																		if en, ok := argsObj["english"].([]interface{}); ok {
																			for _, line := range en {
																				english = append(english, fmt.Sprintf("%v", line))
																			}
																		}

																		// Use the new box rendering
																		fmt.Fprintln(os.Stdout)
																		fmt.Fprintln(os.Stdout, ui.RenderHaikuBox(japanese, english))
																	} else {
																		// Generic pretty print for other tools
																		argsJSON, _ := json.MarshalIndent(argsObj, "      ", "  ")
																		fmt.Fprintf(os.Stdout, "      %s\n", string(argsJSON))
																	}
																} else {
																	fmt.Fprintf(os.Stdout, "  🔧 %s: %s\n", name, args)
																}
															}
														}
													}
												}
											}

										case "tool":
											// Handle tool result messages
											if outputMode == ui.OutputModePretty {
												toolCallId, _ := msgMap["toolCallId"].(string)
												content, _ := msgMap["content"].(string)

												// Check for error status
												isError := false
												if errorField, ok := msgMap["error"]; ok {
													isError = errorField != nil && errorField != false
												}

												// Find the associated tool call
												var toolName string
												if toolCallId != "" {
													if toolCall, ok := toolCallMap[toolCallId]; ok {
														if function, ok := toolCall["function"].(map[string]interface{}); ok {
															toolName, _ = function["name"].(string)
														}
													}
												}

												// Display with appropriate status indicator
												if isError {
													fmt.Fprintf(os.Stdout, "\n❌ Tool Result (Error)")
												} else {
													fmt.Fprintf(os.Stdout, "\n✅ Tool Result")
												}

												if toolName != "" {
													fmt.Fprintf(os.Stdout, " (%s)", toolName)
												}
												if toolCallId != "" {
													fmt.Fprintf(os.Stdout, " [ID: %s]", toolCallId)
												}
												fmt.Fprintln(os.Stdout, ":")

												// Try to parse and pretty print the content if it's JSON
												var contentObj interface{}
												if err := json.Unmarshal([]byte(content), &contentObj); err == nil {
													contentJSON, _ := json.MarshalIndent(contentObj, "   ", "  ")
													fmt.Fprintf(os.Stdout, "   %s\n", string(contentJSON))
												} else {
													// Display as plain text if not JSON
													lines := strings.Split(content, "\n")
													for _, line := range lines {
														fmt.Fprintf(os.Stdout, "   %s\n", line)
													}
												}
											}
										}
									}
								}
							}
							if outputMode == ui.OutputModeJSON {
								jsonData, _ := json.Marshal(event)
								fmt.Fprintln(os.Stdout, string(jsonData))
							}

						case "STATE_SNAPSHOT":
							// Handle full state snapshot
							hasContent = true
							if outputMode == ui.OutputModePretty {
								fmt.Fprintln(os.Stdout, "\n📊 State Snapshot:")
								// The snapshot data might be at the top level or in a "snapshot" field
								if snapshot, ok := eventData["snapshot"].(map[string]interface{}); ok {
									stateJSON, _ := json.MarshalIndent(snapshot, "  ", "  ")
									fmt.Fprintf(os.Stdout, "  %s\n", string(stateJSON))
								} else if state, ok := eventData["state"].(map[string]interface{}); ok {
									stateJSON, _ := json.MarshalIndent(state, "  ", "  ")
									fmt.Fprintf(os.Stdout, "  %s\n", string(stateJSON))
								}
							}
							if outputMode == ui.OutputModeJSON {
								jsonData, _ := json.Marshal(event)
								fmt.Fprintln(os.Stdout, string(jsonData))
							}

						case "STATE_DELTA":
							// Handle state delta (JSON Patch operations)
							hasContent = true
							if outputMode == ui.OutputModePretty {
								fmt.Fprintln(os.Stdout, "\n🔄 State Update:")
								// The delta operations might be at the top level as "delta" or "operations"
								var ops []interface{}
								if delta, ok := eventData["delta"].([]interface{}); ok {
									ops = delta
								} else if operations, ok := eventData["operations"].([]interface{}); ok {
									ops = operations
								}

								for _, op := range ops {
									if opMap, ok := op.(map[string]interface{}); ok {
										opType, _ := opMap["op"].(string)
										path, _ := opMap["path"].(string)
										value := opMap["value"]

										switch opType {
										case "add":
											fmt.Fprintf(os.Stdout, "  ➕ Add: %s = %v\n", path, value)
										case "replace":
											fmt.Fprintf(os.Stdout, "  🔄 Replace: %s = %v\n", path, value)
										case "remove":
											fmt.Fprintf(os.Stdout, "  ➖ Remove: %s\n", path)
										default:
											fmt.Fprintf(os.Stdout, "  ❓ %s: %s = %v\n", opType, path, value)
										}
									}
								}
							}
							if outputMode == ui.OutputModeJSON {
								jsonData, _ := json.Marshal(event)
								fmt.Fprintln(os.Stdout, string(jsonData))
							}

						case "CUSTOM":
							// Handle custom events (used by predictive_state_updates)
							hasContent = true
							if outputMode == ui.OutputModePretty {
								fmt.Fprintln(os.Stdout, "\n🔮 Custom Event:")
								if customType, ok := eventData["customType"].(string); ok {
									fmt.Fprintf(os.Stdout, "  Type: %s\n", customType)
								}
								if data, ok := eventData["data"].(map[string]interface{}); ok {
									dataJSON, _ := json.MarshalIndent(data, "  ", "  ")
									fmt.Fprintf(os.Stdout, "  Data: %s\n", string(dataJSON))
								}
							}
							if outputMode == ui.OutputModeJSON {
								jsonData, _ := json.Marshal(event)
								fmt.Fprintln(os.Stdout, string(jsonData))
							}

						case "UI_UPDATE":
							// Handle UI update events for dynamic interface changes
							hasContent = true
							if outputMode == ui.OutputModePretty {
								// Extract update type and content
								updateType, _ := eventData["updateType"].(string)

								switch updateType {
								case "progress":
									// Progress bar update
									if progress, ok := eventData["progress"].(map[string]interface{}); ok {
										current, _ := progress["current"].(float64)
										total, _ := progress["total"].(float64)
										message, _ := progress["message"].(string)

										if total > 0 {
											percentage := int((current / total) * 100)
											barLength := 30
											filled := int(float64(barLength) * (current / total))

											// Create progress bar
											bar := "["
											for i := 0; i < barLength; i++ {
												if i < filled {
													bar += "█"
												} else {
													bar += "░"
												}
											}
											bar += "]"

											fmt.Fprintf(os.Stdout, "\r📊 Progress: %s %d%% - %s", bar, percentage, message)
											if current >= total {
												fmt.Fprintln(os.Stdout) // New line when complete
											}
										}
									}

								case "status":
									// Status message update
									if status, ok := eventData["status"].(string); ok {
										icon := "ℹ️"
										if severity, ok := eventData["severity"].(string); ok {
											switch severity {
											case "success":
												icon = "✅"
											case "warning":
												icon = "⚠️"
											case "error":
												icon = "❌"
											case "info":
												icon = "ℹ️"
											}
										}
										fmt.Fprintf(os.Stdout, "\n%s %s\n", icon, status)
									}

								case "component":
									// Component update (e.g., form fields, buttons)
									if component, ok := eventData["component"].(map[string]interface{}); ok {
										componentType, _ := component["type"].(string)
										componentId, _ := component["id"].(string)

										fmt.Fprintf(os.Stdout, "\n🔧 UI Component Update:\n")
										fmt.Fprintf(os.Stdout, "  Type: %s\n", componentType)
										fmt.Fprintf(os.Stdout, "  ID: %s\n", componentId)

										if props, ok := component["props"].(map[string]interface{}); ok {
											propsJSON, _ := json.MarshalIndent(props, "  ", "  ")
											fmt.Fprintf(os.Stdout, "  Properties:\n  %s\n", string(propsJSON))
										}
									}

								default:
									// Generic UI update
									fmt.Fprintln(os.Stdout, "\n🎨 UI Update:")
									if updateType != "" {
										fmt.Fprintf(os.Stdout, "  Type: %s\n", updateType)
									}
									// Display any additional data
									for key, value := range eventData {
										if key != "type" && key != "updateType" {
											fmt.Fprintf(os.Stdout, "  %s: %v\n", key, value)
										}
									}
								}
							}
							if outputMode == ui.OutputModeJSON {
								jsonData, _ := json.Marshal(event)
								fmt.Fprintln(os.Stdout, string(jsonData))
							}

						case "THINKING_START":
							// Handle thinking start event - assistant is processing
							if outputMode == ui.OutputModePretty {
								fmt.Fprintln(cmd.OutOrStdout(), "\n🤔 Assistant is thinking...")
								if thinkingSpinner == nil {
									thinkingSpinner = spinner.New(spinner.Config{
										Writer:  cmd.OutOrStdout(),
										Message: "Processing request",
										Style:   spinner.StyleDots,
									})
									thinkingSpinner.Start()
								}
							}
							if outputMode == ui.OutputModeJSON {
								jsonData, _ := json.Marshal(event)
								fmt.Fprintln(os.Stdout, string(jsonData))
							}

						case "THINKING_DELTA", "THINKING_CONTENT":
							// Handle thinking content updates
							if outputMode == ui.OutputModePretty {
								// Try both "delta" and "content" fields for compatibility
								content, ok := eventData["delta"].(string)
								if !ok {
									content, _ = eventData["content"].(string)
								}

								if content != "" && thinkingSpinner != nil {
									// Update spinner message with thinking content preview
									if len(content) > 50 {
										content = content[:50] + "..."
									}
									thinkingSpinner.UpdateMessage(fmt.Sprintf("Thinking: %s", content))
								}
							}
							if outputMode == ui.OutputModeJSON {
								jsonData, _ := json.Marshal(event)
								fmt.Fprintln(os.Stdout, string(jsonData))
							}

						case "THINKING_END":
							// Handle thinking end event
							if outputMode == ui.OutputModePretty {
								if thinkingSpinner != nil {
									thinkingSpinner.StopWithMessage("✅ Thinking complete")
									thinkingSpinner = nil
								}
							}
							if outputMode == ui.OutputModeJSON {
								jsonData, _ := json.Marshal(event)
								fmt.Fprintln(os.Stdout, string(jsonData))
							}

						case "RUN_FINISHED":
							// Complete any active spinners
							if multiToolSpinner != nil {
								multiToolSpinner.CompleteAll()
								multiToolSpinner = nil
							}
							if streamingSpinner != nil {
								streamingSpinner.CompleteStreaming(true)
								streamingSpinner = nil
							}

							// Run completed
							if outputMode == ui.OutputModeJSON {
								// Pass to renderer for JSON output
								eventDataBytes, _ := json.Marshal(eventData)
								renderer.HandleEvent(eventType, eventDataBytes)

								if assistantMessage.Len() > 0 {
									output := map[string]interface{}{
										"role":    "assistant",
										"content": assistantMessage.String(),
									}
									jsonData, _ := json.Marshal(output)
									fmt.Fprintln(os.Stdout, string(jsonData))
								}
								return nil
							}

							// Handle interactive mode
							if interactive && len(lastToolResults) > 0 && promptHandler != nil {
								action, err := promptHandler.AskForAction("")
								if err != nil {
									logger.WithError(err).Error("Failed to read user input")
									return nil
								}

								switch action {
								case prompt.ActionApply:
									// Apply the results - add assistant message to history
									if currentAssistantMessage != nil {
										conversationHistory = append(conversationHistory, currentAssistantMessage)
									}
									fmt.Println("\n✅ Applied! The tool results have been accepted.")
									return nil

								case prompt.ActionRegenerate:
									// Regenerate - keep the same user message, try again
									fmt.Println("\n🔄 Regenerating response...")
									continue regenerateLoop

								case prompt.ActionCancel:
									// Cancel - exit without applying
									fmt.Println("\n❌ Cancelled. No changes were applied.")
									return nil
								}
							}
							return nil

						default:
							// Pass other events to the renderer
							if eventData != nil {
								eventDataBytes, _ := json.Marshal(eventData)
								renderer.HandleEvent(eventType, eventDataBytes)
							}
						}

					case err, ok := <-errors:
						if !ok {
							break regenerateLoop
						}
						if err != nil {
							logger.WithError(err).Error("SSE stream error")
							break regenerateLoop
						}

					case <-ctx.Done():
						logger.Debug("Context cancelled, closing stream")
						break regenerateLoop
					}
				}
			} // End of regenerateLoop

			return nil
		},
	}

	cmd.Flags().StringVar(&message, "message", "", "Chat message to send")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID for context")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.Flags().BoolVar(&interactive, "interactive", true, "Enable interactive prompts for tool results")
	cmd.Flags().BoolVar(&streaming, "streaming", false, "Use streaming endpoint for real-time text generation")
	cmd.Flags().BoolVar(&humanLoop, "human-loop", false, "Use human-in-the-loop endpoint for tool approval workflows")
	cmd.Flags().BoolVar(&stateMode, "state-mode", false, "Use state management endpoint for STATE_SNAPSHOT/DELTA events")
	cmd.Flags().BoolVar(&predictiveMode, "predictive", false, "Use predictive updates endpoint for advanced features")
	cmd.Flags().BoolVar(&sharedStateMode, "shared-state", false, "Use shared state endpoint for state synchronization")
	cmd.Flags().BoolVar(&optimizedStreaming, "optimized-streaming", false, "Use optimized SSE streaming with better buffer management")
	cmd.Flags().StringVar(&bufferSize, "buffer-size", "medium", "Buffer size for optimized streaming (small/medium/large/max)")
	cmd.Flags().BoolVar(&showMetrics, "show-metrics", false, "Show streaming performance metrics after completion")
	cmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "System prompt for the agent")
	cmd.Flags().StringVar(&model, "model", "", "Model to use for generation")
	cmd.Flags().Float64Var(&temperature, "temperature", 0, "Temperature for generation (0 for default)")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "Maximum tokens for generation (0 for default)")
	cmd.Flags().BoolVar(&clientTools, "client-tools", false, "Enable client-side tool execution")
	cmd.Flags().StringVar(&toolsDir, "tools-dir", "", "Directory containing custom tool definitions (JSON files)")
	cmd.Flags().BoolVar(&resumeSession, "resume", false, "Resume previous conversation in the session (includes message history)")

	return cmd
}

func newUICommand() *cobra.Command {
	var sessionID string
	var statePath string
	var showState bool
	var showHistory bool
	var interactive bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Interactive tool-based UI for state management",
		Long: `Interactive tool-based UI with enhanced state management and rendering.
		
This command provides a dedicated interface for tool-based generative UI workflows with:
- State persistence across sessions
- Enhanced tool result rendering
- Conversation history tracking
- State viewer and management

Examples:
  # Start UI with new state
  ag-ui-client ui
  
  # Resume with existing state file
  ag-ui-client ui --state-file ./my-state.json
  
  # View current state
  ag-ui-client ui --show-state
  
  # View conversation history
  ag-ui-client ui --show-history
  
  # Non-interactive mode for automation
  ag-ui-client ui --interactive=false --message "Generate content"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := configManager.GetConfig()

			// Check for JSON output mode
			jsonOutput = cfg.Output == "json" || jsonOutput

			// Initialize state manager
			stateManager := NewUIStateManager(statePath)
			if err := stateManager.Load(); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to load state: %w", err)
			}

			// Handle state viewing
			if showState {
				return displayState(stateManager, jsonOutput)
			}

			// Handle history viewing
			if showHistory {
				return displayHistory(stateManager, jsonOutput)
			}

			// Get or create session
			if sessionID == "" {
				sessionStore := session.NewStore(configManager.GetConfigPath())
				activeSession, err := sessionStore.GetActiveSession()
				if err == nil && activeSession != nil {
					sessionID = activeSession.ThreadID
					logger.WithField("session_id", sessionID).Debug("Using active session")
				} else {
					sessionID = uuid.New().String()
					logger.WithField("session_id", sessionID).Debug("Created new session")
				}
			}

			// Create persistent store and integrate with UI state manager
			persistentStore := session.NewPersistentStore(configManager.GetConfigPath())

			// Load existing session history if available
			existingMessages, err := persistentStore.GetSessionHistory(sessionID)
			if err != nil {
				// Create new session if not found
				_, err = persistentStore.CreateSession(sessionID, "ui-session")
				if err != nil {
					logger.WithError(err).Warn("Failed to create session")
				}
			} else {
				// Sync existing messages to UI state manager
				for _, msg := range existingMessages {
					stateManager.AddMessage(msg.Role, msg.Content)
					for _, tc := range msg.ToolCalls {
						stateManager.AddToolCall(tc.Function.Name, tc.Function.Arguments, tc.Result)
					}
				}
			}

			// Load existing state from persistent store
			if existingState, err := persistentStore.GetSessionState(sessionID); err == nil && existingState != nil {
				stateManager.state = existingState
			}

			// Start interactive UI loop with persistent store
			return runInteractiveUIWithPersistence(cfg, sessionID, stateManager, persistentStore, interactive, jsonOutput)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID for context")
	cmd.Flags().StringVar(&statePath, "state-file", "", "Path to state persistence file")
	cmd.Flags().BoolVar(&showState, "show-state", false, "Display current state and exit")
	cmd.Flags().BoolVar(&showHistory, "show-history", false, "Display conversation history and exit")
	cmd.Flags().BoolVar(&interactive, "interactive", true, "Enable interactive mode")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

// UIStateManager manages state persistence for the UI command
type UIStateManager struct {
	filePath string
	state    map[string]interface{}
	history  []UIHistoryEntry
}

type UIHistoryEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"` // "message", "tool_call", "state_update"
	Content   map[string]interface{} `json:"content"`
}

func NewUIStateManager(filePath string) *UIStateManager {
	if filePath == "" {
		home, _ := os.UserHomeDir()
		filePath = fmt.Sprintf("%s/.config/ag-ui/client/ui-state.json", home)
	}
	return &UIStateManager{
		filePath: filePath,
		state:    make(map[string]interface{}),
		history:  []UIHistoryEntry{},
	}
}

func (m *UIStateManager) Load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	var saved struct {
		State   map[string]interface{} `json:"state"`
		History []UIHistoryEntry       `json:"history"`
	}

	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}

	m.state = saved.State
	m.history = saved.History
	return nil
}

func (m *UIStateManager) Save() error {
	// Ensure directory exists
	dir := strings.TrimSuffix(m.filePath, "/ui-state.json")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	saved := struct {
		State   map[string]interface{} `json:"state"`
		History []UIHistoryEntry       `json:"history"`
	}{
		State:   m.state,
		History: m.history,
	}

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.filePath, data, 0644)
}

func (m *UIStateManager) UpdateState(key string, value interface{}) {
	m.state[key] = value
	m.history = append(m.history, UIHistoryEntry{
		Timestamp: time.Now(),
		Type:      "state_update",
		Content: map[string]interface{}{
			"key":   key,
			"value": value,
		},
	})
}

func (m *UIStateManager) AddMessage(role string, content string) {
	m.history = append(m.history, UIHistoryEntry{
		Timestamp: time.Now(),
		Type:      "message",
		Content: map[string]interface{}{
			"role":    role,
			"content": content,
		},
	})
}

func (m *UIStateManager) AddToolCall(name string, args interface{}, result interface{}) {
	m.history = append(m.history, UIHistoryEntry{
		Timestamp: time.Now(),
		Type:      "tool_call",
		Content: map[string]interface{}{
			"name":   name,
			"args":   args,
			"result": result,
		},
	})
}

func displayState(manager *UIStateManager, jsonOutput bool) error {
	if jsonOutput {
		data, err := json.MarshalIndent(manager.state, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		fmt.Println("╭─────────────────────────────────────────╮")
		fmt.Println("│            Current State                │")
		fmt.Println("├─────────────────────────────────────────┤")

		if len(manager.state) == 0 {
			fmt.Println("│ (empty)                                 │")
		} else {
			for key, value := range manager.state {
				valueStr := fmt.Sprintf("%v", value)
				if len(valueStr) > 30 {
					valueStr = valueStr[:27] + "..."
				}
				fmt.Printf("│ %-15s: %-23s │\n", key, valueStr)
			}
		}

		fmt.Println("╰─────────────────────────────────────────╯")
	}
	return nil
}

func displayHistory(manager *UIStateManager, jsonOutput bool) error {
	if jsonOutput {
		data, err := json.MarshalIndent(manager.history, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		fmt.Println("╭─────────────────────────────────────────╮")
		fmt.Println("│          Conversation History           │")
		fmt.Println("├─────────────────────────────────────────┤")

		if len(manager.history) == 0 {
			fmt.Println("│ (no history)                            │")
		} else {
			for _, entry := range manager.history {
				timestamp := entry.Timestamp.Format("15:04:05")
				switch entry.Type {
				case "message":
					role := entry.Content["role"].(string)
					content := entry.Content["content"].(string)
					if len(content) > 25 {
						content = content[:22] + "..."
					}
					fmt.Printf("│ %s [%s]: %-20s │\n", timestamp, role, content)
				case "tool_call":
					name := entry.Content["name"].(string)
					fmt.Printf("│ %s 🔧 %s                    │\n", timestamp, name)
				case "state_update":
					key := entry.Content["key"].(string)
					fmt.Printf("│ %s 📊 Updated: %-15s │\n", timestamp, key)
				}
			}
		}

		fmt.Println("╰─────────────────────────────────────────╯")
	}
	return nil
}

func runInteractiveUIWithPersistence(cfg *config.Config, sessionID string, stateManager *UIStateManager, persistentStore *session.PersistentStore, interactive bool, jsonOutput bool) error {
	// Save state and messages to persistent store periodically
	saveToStore := func() {
		// Save state to persistent store
		for key, value := range stateManager.state {
			if err := persistentStore.UpdateState(sessionID, key, value); err != nil {
				logger.WithError(err).Warn("Failed to save state to persistent store")
			}
		}
	}

	// Auto-save on exit
	defer saveToStore()

	// Continue with existing implementation
	return runInteractiveUI(cfg, sessionID, stateManager, interactive, jsonOutput)
}

func runInteractiveUI(cfg *config.Config, sessionID string, stateManager *UIStateManager, interactive bool, jsonOutput bool) error {
	// Create enhanced UI renderer
	outputMode := ui.OutputModePretty
	if jsonOutput {
		outputMode = ui.OutputModeJSON
	}
	renderer := ui.NewRenderer(ui.RendererConfig{
		OutputMode: outputMode,
		NoColor:    false,
		Quiet:      false,
		Writer:     os.Stdout,
	})

	fmt.Println("╭─────────────────────────────────────────╮")
	fmt.Println("│      AG-UI Interactive Interface        │")
	fmt.Println("├─────────────────────────────────────────┤")
	fmt.Println("│ Commands:                               │")
	fmt.Println("│   /state    - Show current state        │")
	fmt.Println("│   /history  - Show conversation history │")
	fmt.Println("│   /clear    - Clear state and history   │")
	fmt.Println("│   /save     - Save state to file        │")
	fmt.Println("│   /exit     - Exit UI mode              │")
	fmt.Println("│                                         │")
	fmt.Println("│ Or type a message to send to AG-UI      │")
	fmt.Println("╰─────────────────────────────────────────╯")
	fmt.Println()

	// Interactive loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("🎨 UI> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle commands
		switch input {
		case "/state":
			displayState(stateManager, false)
			continue
		case "/history":
			displayHistory(stateManager, false)
			continue
		case "/clear":
			stateManager.state = make(map[string]interface{})
			stateManager.history = []UIHistoryEntry{}
			fmt.Println("✅ State and history cleared")
			continue
		case "/save":
			if err := stateManager.Save(); err != nil {
				fmt.Printf("❌ Failed to save: %v\n", err)
			} else {
				fmt.Printf("✅ State saved to %s\n", stateManager.filePath)
			}
			continue
		case "/exit":
			// Auto-save on exit
			if err := stateManager.Save(); err != nil {
				fmt.Printf("Warning: Failed to save state: %v\n", err)
			}
			fmt.Println("👋 Goodbye!")
			return nil
		}

		// Send message to AG-UI
		stateManager.AddMessage("user", input)

		// Prepare request with state
		endpoint := strings.TrimSuffix(cfg.ServerURL, "/") + "/tool_based_generative_ui"

		requestBody := map[string]interface{}{
			"threadId": sessionID,
			"runId":    fmt.Sprintf("ui-run-%d", time.Now().UnixNano()),
			"messages": []map[string]interface{}{
				{
					"id":      fmt.Sprintf("msg-%d", time.Now().UnixNano()),
					"role":    "user",
					"content": input,
				},
			},
			"state":          stateManager.state,
			"tools":          []interface{}{},
			"context":        []interface{}{},
			"forwardedProps": map[string]interface{}{},
		}

		// Send request and process response
		if err := processUIRequest(endpoint, requestBody, cfg, stateManager, renderer, interactive, jsonOutput, os.Stdout); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
		}
	}

	return nil
}

func renderToolResult(name string, args map[string]interface{}) {
	// Enhanced rendering for known tool types
	fmt.Println("╭─────────────────────────────────────────╮")
	fmt.Printf("│ 🔧 %-37s │\n", name)
	fmt.Println("├─────────────────────────────────────────┤")

	switch name {
	case "generate_haiku":
		// Special rendering for haiku
		if japanese, ok := args["japanese"].([]interface{}); ok {
			for _, line := range japanese {
				fmt.Printf("│ %-39s │\n", line)
			}
		}
		if english, ok := args["english"].([]interface{}); ok {
			fmt.Println("├─────────────────────────────────────────┤")
			for _, line := range english {
				fmt.Printf("│ %-39s │\n", line)
			}
		}
	default:
		// Generic rendering for other tools
		for key, value := range args {
			valueStr := fmt.Sprintf("%v", value)
			if len(valueStr) > 30 {
				valueStr = valueStr[:27] + "..."
			}
			fmt.Printf("│ %-15s: %-23s │\n", key, valueStr)
		}
	}

	fmt.Println("╰─────────────────────────────────────────╯")
}

func processUIRequest(endpoint string, requestBody map[string]interface{}, cfg *config.Config, stateManager *UIStateManager, renderer *ui.Renderer, interactive bool, jsonOutput bool, output io.Writer) error {
	// Create request
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Add authentication if configured
	if cfg.APIKey != "" {
		if cfg.AuthHeader == "X-API-Key" {
			req.Header.Set("X-API-Key", cfg.APIKey)
		} else {
			req.Header.Set("Authorization", fmt.Sprintf("%s %s", cfg.AuthScheme, cfg.APIKey))
		}
	}

	// Send request
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	// Process SSE events
	scanner := bufio.NewScanner(resp.Body)
	var toolResults []map[string]interface{}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)

		switch eventType {
		case "MESSAGES_SNAPSHOT":
			messages, _ := event["messages"].([]interface{})
			for _, msg := range messages {
				msgMap := msg.(map[string]interface{})
				role, _ := msgMap["role"].(string)

				if role == "assistant" {
					if toolCalls, ok := msgMap["toolCalls"].([]interface{}); ok && len(toolCalls) > 0 {
						for _, tc := range toolCalls {
							toolCall := tc.(map[string]interface{})
							function := toolCall["function"].(map[string]interface{})
							name := function["name"].(string)
							argsStr := function["arguments"].(string)

							var args map[string]interface{}
							json.Unmarshal([]byte(argsStr), &args)

							// Display tool result with enhanced rendering
							fmt.Println("\n🎨 Tool Result:")
							renderToolResult(name, args)

							// Track tool call
							stateManager.AddToolCall(name, args, nil)
							toolResults = append(toolResults, args)
						}
					}

					if content, ok := msgMap["content"].(string); ok && content != "" {
						fmt.Fprintf(output, "\n💬 Assistant: %s\n", content)
						stateManager.AddMessage("assistant", content)
					}
				}
			}

		case "STATE_SNAPSHOT":
			if state, ok := event["state"].(map[string]interface{}); ok {
				// Update state manager
				for key, value := range state {
					stateManager.UpdateState(key, value)
				}

				if !jsonOutput {
					fmt.Println("\n📊 State Updated:")
					displayState(stateManager, false)
				}
			}
		}
	}

	// Interactive prompts for tool results
	if interactive && len(toolResults) > 0 {
		prompter := prompt.New()
		action, err := prompter.AskForAction("Apply tool results?")
		if err == nil {
			switch action {
			case prompt.ActionApply:
				// Apply results to state
				for _, result := range toolResults {
					for key, value := range result {
						stateManager.UpdateState(key, value)
					}
				}
				fmt.Println("✅ Results applied to state")
				stateManager.Save()
			case prompt.ActionRegenerate:
				// Re-run the same request
				return processUIRequest(endpoint, requestBody, cfg, stateManager, renderer, interactive, jsonOutput, output)
			case prompt.ActionCancel:
				fmt.Println("❌ Results discarded")
			}
		}
	}

	return nil
}

func newStreamCommand() *cobra.Command {
	var sessionID string
	var follow bool
	var messages []string
	var systemPrompt string
	var model string
	var temperature float64
	var maxTokens int
	var outputMode string
	var noColor bool
	var quiet bool

	// Retry configuration flags
	var onError string
	var maxRetries int
	var retryDelay string
	var retryJitter float64
	var timeout string

	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Stream events from AG-UI sessions",
		Long: `Stream Server-Sent Events (SSE) from AG-UI sessions for real-time updates.

Examples:
  # Stream events from a specific session
  ag-ui-client stream --session-id abc123
  
  # Follow a session continuously
  ag-ui-client stream --session-id abc123 --follow
  
  # Stream with messages
  ag-ui-client stream --session-id abc123 --message "Hello, AG-UI!"
  
  # Stream with custom model and parameters
  ag-ui-client stream --session-id abc123 --model gpt-4 --temperature 0.7 --max-tokens 1000`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg := configManager.GetConfig()

			if cfg.ServerURL == "" {
				logger.Error("Server URL not configured. Use --server flag or set AGUI_SERVER environment variable")
				os.Exit(1)
			}

			// If no session ID provided, try to use the active session
			if sessionID == "" {
				sessionStore := session.NewStore(configManager.GetConfigPath())
				activeSession, err := sessionStore.GetActiveSession()
				if err == nil && activeSession != nil {
					sessionID = activeSession.ThreadID
					logger.WithField("threadId", sessionID).Debug("Using active session")
				} else {
					// Fall back to last session ID from config
					sessionID = cfg.LastSessionID
					if sessionID != "" {
						logger.WithField("threadId", sessionID).Debug("Using last session ID from config")
					}
				}
			}

			endpoint := strings.TrimSuffix(cfg.ServerURL, "/") + "/tool_based_generative_ui"

			// Use auth configuration from config manager
			authHeader := cfg.AuthHeader
			if authHeader == "" {
				authHeader = "Authorization"
			}
			authScheme := cfg.AuthScheme
			if authScheme == "" && authHeader == "Authorization" {
				authScheme = "Bearer"
			}

			sseConfig := sse.Config{
				Endpoint:       endpoint,
				APIKey:         cfg.APIKey,
				AuthHeader:     authHeader,
				AuthScheme:     authScheme,
				ConnectTimeout: 30 * time.Second,
				ReadTimeout:    5 * time.Minute,
				BufferSize:     100,
				Logger:         logger,
			}

			client := sse.NewClient(sseConfig)
			defer client.Close()

			payload := sse.RunAgentInput{
				SessionID:    sessionID,
				Stream:       true,
				Model:        model,
				SystemPrompt: systemPrompt,
			}

			if temperature > 0 {
				payload.Temperature = &temperature
			}

			if maxTokens > 0 {
				payload.MaxTokens = &maxTokens
			}

			for _, msg := range messages {
				payload.Messages = append(payload.Messages, sse.Message{
					Role:    "user",
					Content: msg,
				})
			}

			ctx := context.Background()
			if !follow {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigChan) // Clean up signal handler

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			go func() {
				select {
				case <-sigChan:
					logger.Info("Received interrupt signal, closing stream...")
					cancel()
				case <-ctx.Done():
					return // Exit goroutine when context is done
				}
			}()

			logger.WithFields(logrus.Fields{
				"endpoint":   endpoint,
				"session_id": sessionID,
				"follow":     follow,
			}).Info("Connecting to SSE stream")

			frames, errors, err := client.Stream(sse.StreamOptions{
				Context: ctx,
				Payload: payload,
			})

			if err != nil {
				logger.WithError(err).Error("Failed to establish SSE connection")
				os.Exit(1)
			}

			// Override output mode from flags if specified
			if outputMode != "" {
				cfg.Output = outputMode
			}

			// Create UI renderer
			var rendererMode ui.OutputMode
			if cfg.Output == "json" {
				rendererMode = ui.OutputModeJSON
			} else {
				rendererMode = ui.OutputModePretty
			}

			_ = ui.NewRenderer(ui.RendererConfig{
				OutputMode: rendererMode,
				NoColor:    noColor,
				Quiet:      quiet,
				Writer:     os.Stdout,
			})

			// Parse retry configuration
			_, err = time.ParseDuration(retryDelay)
			if err != nil {
				logger.WithError(err).Error("Invalid retry delay duration")
				os.Exit(1)
			}

			_, err = time.ParseDuration(timeout)
			if err != nil {
				logger.WithError(err).Error("Invalid timeout duration")
				os.Exit(1)
			}

			// Helper function to parse retry policy
			// parseRetryPolicy := func(s string) tools.RetryPolicy {
			// 	switch strings.ToLower(s) {
			// 	case "retry":
			// 		return tools.RetryPolicyRetry
			// 	case "prompt":
			// 		return tools.RetryPolicyPrompt
			// 	default:
			// 		return tools.RetryPolicyAbort
			// 	}
			// }

			// Create retry configuration
			// retryConfig := tools.RetryConfig{
			// 	OnError:           parseRetryPolicy(onError),
			// 	MaxRetries:        maxRetries,
			// 	InitialDelay:      retryDelayDuration,
			// 	MaxDelay:          30 * time.Second,
			// 	BackoffMultiplier: 2.0,
			// 	JitterFactor:      retryJitter,
			// 	Timeout:           timeoutDuration,
			// 	PerAttemptTimeout: 30 * time.Second,
			// 	ResetAfter:        60 * time.Second,
			// 	Logger:            logger,
			// }

			// Create stream integration for tool handling
			// streamIntegration := tools.NewStreamIntegration(retryConfig, renderer)

			frameCount := 0
			startTime := time.Now()

			for {
				select {
				case frame, ok := <-frames:
					if !ok {
						if !quiet {
							logger.WithFields(logrus.Fields{
								"frames":   frameCount,
								"duration": time.Since(startTime),
							}).Info("SSE stream closed")
						}
						return
					}

					frameCount++

					// Parse the SSE event
					var event map[string]interface{}
					if err := json.Unmarshal(frame.Data, &event); err != nil {
						if !quiet {
							logger.WithFields(logrus.Fields{
								"frame":     frameCount,
								"raw":       string(frame.Data),
								"timestamp": frame.Timestamp,
							}).Warn("Received non-JSON frame")
						}
						continue
					}

					// Extract event type and data
					eventType, _ := event["event"].(string)
					if eventType == "" {
						continue
					}

					_, _ = json.Marshal(event["data"])

					// Handle the event through the stream integration
					// if err := streamIntegration.HandleSSEEvent(eventType, eventData); err != nil {
					// 	logger.WithError(err).WithField("event", eventType).Warn("Failed to handle event")
					// }

					// Check if we should exit due to tool errors
					// if streamIntegration.ShouldExit() {
					// 	streamIntegration.CleanExit()
					// 	return
					// }

				case err, ok := <-errors:
					if !ok {
						return
					}
					if err != nil {
						logger.WithError(err).Error("SSE stream error")
						return
					}

				case <-ctx.Done():
					logger.Info("Context cancelled, closing stream")
					// Perform clean exit with metrics
					// streamIntegration.CleanExit()
					return
				}
			}
		},
	}

	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID to stream")
	cmd.Flags().BoolVar(&follow, "follow", false, "Follow stream continuously")
	cmd.Flags().StringArrayVar(&messages, "message", nil, "Messages to send (can be specified multiple times)")
	cmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "System prompt for the agent")
	cmd.Flags().StringVar(&model, "model", "", "Model to use for generation")
	cmd.Flags().Float64Var(&temperature, "temperature", 0, "Temperature for generation (0 for default)")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "Maximum tokens for generation (0 for default)")
	cmd.Flags().StringVar(&outputMode, "output", "pretty", "Output mode: pretty or json")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress all output except errors")

	// Retry configuration flags
	cmd.Flags().StringVar(&onError, "on-error", "abort", "Error handling policy: retry, abort, or prompt")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 3, "Maximum number of retry attempts (0 = no retries)")
	cmd.Flags().StringVar(&retryDelay, "retry-delay", "500ms", "Initial delay before first retry")
	cmd.Flags().Float64Var(&retryJitter, "retry-jitter", 0.2, "Jitter factor for retry delays (0.0 to 1.0)")
	cmd.Flags().StringVar(&timeout, "timeout", "5m", "Overall timeout for all retry attempts")

	return cmd
}

func newHumanLoopCommand() *cobra.Command {
	var message string
	var sessionID string
	var runID string
	var jsonOutput bool
	var noColor bool
	var showMetrics bool
	var tools []string
	var approvalMode string // auto, manual, reject
	var timeout string

	cmd := &cobra.Command{
		Use:   "human-loop",
		Short: "Execute tools with human-in-the-loop approval workflow",
		Long: `Execute tools using the /human_in_the_loop endpoint which provides detailed 
tool execution events including TOOL_CALL_START, TOOL_CALL_ARGS, and TOOL_CALL_END.

This endpoint is specifically designed for workflows that require human approval or 
monitoring of tool executions with granular event tracking.

Examples:
  # Execute with automatic approval
  ag-ui-client human-loop --message "Generate a haiku" --approval auto
  
  # Execute with manual approval (interactive)
  ag-ui-client human-loop --message "Make API calls" --approval manual
  
  # Execute with specific tools allowed
  ag-ui-client human-loop --message "Fetch weather" --tools weather_api,http_get
  
  # Get detailed execution events in JSON
  ag-ui-client human-loop --message "Process data" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if message == "" {
				logger.Error("Message is required. Use --message flag")
				return fmt.Errorf("message is required")
			}

			cfg := configManager.GetConfig()
			if cfg.ServerURL == "" {
				logger.Error("Server URL not configured")
				return fmt.Errorf("server URL not configured")
			}

			// If no session ID provided, generate one
			if sessionID == "" {
				sessionID = uuid.New().String()
			}
			if runID == "" {
				runID = fmt.Sprintf("run-%d", time.Now().UnixNano())
			}

			// Build endpoint URL
			endpoint := strings.TrimSuffix(cfg.ServerURL, "/") + "/human_in_the_loop"

			// Prepare request payload
			payload := map[string]interface{}{
				"thread_id": sessionID,
				"run_id":    runID,
				"messages": []map[string]interface{}{
					{
						"id":      fmt.Sprintf("msg-%d", time.Now().UnixNano()),
						"role":    "user",
						"content": message,
					},
				},
				"state":   map[string]interface{}{},
				"tools":   []string{}, // Server will provide available tools
				"context": []interface{}{},
				"forwardedProps": map[string]interface{}{
					"approval_mode": approvalMode,
				},
			}

			// Add tool filter if specified
			if len(tools) > 0 {
				payload["tools"] = tools
			}

			// Initialize metrics if requested
			var streamMetrics *streamingpkg.StreamMetrics
			if showMetrics {
				streamMetrics = streamingpkg.NewStreamMetrics()
				defer func() {
					if streamMetrics != nil {
						snapshot := streamMetrics.GetSnapshot()
						fmt.Fprintln(os.Stderr, streamingpkg.FormatMetrics(snapshot))
					}
				}()
			}

			// Setup output mode
			outputMode := ui.OutputModePretty
			if jsonOutput {
				outputMode = ui.OutputModeJSON
			}

			// Marshal request
			requestBody, err := json.Marshal(payload)
			if err != nil {
				logger.WithError(err).Error("Failed to marshal request")
				return fmt.Errorf("failed to marshal request: %w", err)
			}

			// Create HTTP request
			req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestBody))
			if err != nil {
				logger.WithError(err).Error("Failed to create request")
				return fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "text/event-stream")

			// Add authentication if configured
			if cfg.APIKey != "" {
				authHeader := cfg.AuthHeader
				if authHeader == "" {
					authHeader = "Authorization"
				}
				if authHeader == "Authorization" && cfg.AuthScheme != "" {
					req.Header.Set(authHeader, fmt.Sprintf("%s %s", cfg.AuthScheme, cfg.APIKey))
				} else {
					req.Header.Set(authHeader, cfg.APIKey)
				}
			}

			// Create context with timeout or cancellation
			var ctx context.Context
			var cancel context.CancelFunc

			if timeout != "" {
				duration, err := time.ParseDuration(timeout)
				if err != nil {
					logger.WithError(err).Error("Invalid timeout duration")
					return fmt.Errorf("invalid timeout duration: %w", err)
				}
				ctx, cancel = context.WithTimeout(context.Background(), duration)
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()

			// Handle interrupt signals
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigChan) // Clean up signal handler

			go func() {
				select {
				case <-sigChan:
					logger.Debug("Received interrupt signal")
					cancel()
				case <-ctx.Done():
					return // Exit goroutine when context is done
				}
			}()

			// Make the request
			client := &http.Client{Timeout: 0} // No timeout for SSE
			resp, err := client.Do(req.WithContext(ctx))
			if err != nil {
				logger.WithError(err).Error("Failed to connect to server")
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				logger.WithFields(logrus.Fields{
					"status": resp.StatusCode,
					"body":   string(body),
				}).Error("Server returned error")
				return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
			}

			// Parse SSE stream
			reader := bufio.NewReader(resp.Body)

			// Start parsing in the background
			frames := make(chan sse.Frame)
			errors := make(chan error)

			go func() {
				defer close(frames)
				defer close(errors)

				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							errors <- err
						}
						return
					}

					if strings.HasPrefix(line, "data: ") {
						data := strings.TrimPrefix(line, "data: ")
						data = strings.TrimSpace(data)
						if data != "" && data != "[DONE]" {
							frames <- sse.Frame{Data: []byte(data)}
						}
					}
				}
			}()

			// Track tool execution state
			var currentToolCall map[string]interface{}
			var toolApprovalPending bool
			toolExecutions := []map[string]interface{}{}

			// Process events
			logger.Debug("Starting to process SSE events from human-in-the-loop endpoint")

			for {
				select {
				case <-ctx.Done():
					return nil

				case err := <-errors:
					if err != nil && err != io.EOF {
						logger.WithError(err).Error("SSE stream error")
					}
					return nil

				case frame := <-frames:
					if frame.Data == nil || len(frame.Data) == 0 {
						continue
					}

					// Update metrics
					if streamMetrics != nil {
						streamMetrics.RecordEvent(int64(len(frame.Data)))
					}

					// Parse event
					var event map[string]interface{}
					if err := json.Unmarshal(frame.Data, &event); err != nil {
						logger.WithField("raw", string(frame.Data)).Debug("Received non-JSON frame")
						continue
					}

					eventType, _ := event["type"].(string)

					// Log all events in debug mode
					logger.WithFields(logrus.Fields{
						"type": eventType,
						"data": event,
					}).Debug("Received event")

					switch eventType {
					case "RUN_STARTED":
						if outputMode == ui.OutputModePretty {
							fmt.Fprintln(cmd.OutOrStdout(), "\n🔄 Human-in-the-loop workflow started")
						}

					case "TOOL_CALL_START":
						// Extract tool details, try multiple field names
						toolName, _ := event["toolName"].(string)
						if toolName == "" {
							toolName, _ = event["toolCallName"].(string)
						}
						if toolName == "" {
							toolName, _ = event["name"].(string)
						}
						toolCall := map[string]interface{}{
							"id":        event["toolCallId"],
							"tool_name": toolName,
							"started":   time.Now(),
						}
						currentToolCall = toolCall

						if outputMode == ui.OutputModePretty {
							fmt.Fprintf(cmd.OutOrStdout(), "\n🔧 Tool execution started: %s\n", toolName)

							// Check approval mode
							if approvalMode == "manual" {
								toolApprovalPending = true
								fmt.Fprintln(cmd.OutOrStdout(), "⏸️  Awaiting approval...")
								fmt.Fprintln(cmd.OutOrStdout(), "   [A]pprove  [R]eject  [S]kip")

								// Get user input
								reader := bufio.NewReader(cmd.InOrStdin())
								input, _ := reader.ReadString('\n')
								input = strings.TrimSpace(strings.ToLower(input))

								switch input {
								case "a", "approve":
									fmt.Fprintln(cmd.OutOrStdout(), "✅ Tool approved")
									toolApprovalPending = false
								case "r", "reject":
									fmt.Fprintln(cmd.OutOrStdout(), "❌ Tool rejected")
									// In real implementation, would send rejection to server
									return nil
								case "s", "skip":
									fmt.Fprintln(cmd.OutOrStdout(), "⏭️  Tool skipped")
									currentToolCall = nil
									continue
								}
							}
						} else if outputMode == ui.OutputModeJSON {
							output, _ := json.Marshal(event)
							fmt.Fprintln(cmd.OutOrStdout(), string(output))
						}

					case "TOOL_CALL_ARGS":
						// Accumulate tool arguments
						if currentToolCall != nil {
							if args, ok := event["args"]; ok {
								currentToolCall["args"] = args
							}
							if chunk, ok := event["chunk"]; ok {
								// Handle streaming arguments
								if existingArgs, exists := currentToolCall["args_chunks"]; exists {
									chunks := existingArgs.([]interface{})
									currentToolCall["args_chunks"] = append(chunks, chunk)
								} else {
									currentToolCall["args_chunks"] = []interface{}{chunk}
								}
							}
						}

						if outputMode == ui.OutputModePretty && !toolApprovalPending {
							fmt.Fprintln(cmd.OutOrStdout(), "   📝 Receiving arguments...")
						}

					case "TOOL_CALL_END":
						// Complete tool execution
						if currentToolCall != nil {
							currentToolCall["ended"] = time.Now()
							if result, ok := event["result"]; ok {
								currentToolCall["result"] = result
							}
							toolExecutions = append(toolExecutions, currentToolCall)

							if outputMode == ui.OutputModePretty {
								toolName, _ := currentToolCall["tool_name"].(string)
								fmt.Fprintf(cmd.OutOrStdout(), "   ✅ Tool completed: %s\n", toolName)

								// Display result if available
								if result, ok := currentToolCall["result"]; ok {
									resultStr, _ := json.MarshalIndent(result, "      ", "  ")
									fmt.Fprintf(cmd.OutOrStdout(), "      Result: %s\n", resultStr)
								}
							}

							currentToolCall = nil
							toolApprovalPending = false
						}

					case "TOOL_CALL_RESULT":
						// Alternative result event
						if outputMode == ui.OutputModePretty {
							if toolCallId, ok := event["toolCallId"]; ok {
								fmt.Fprintf(cmd.OutOrStdout(), "   📊 Result for tool %s received\n", toolCallId)
							}
						}

					case "TEXT_MESSAGE_START", "TEXT_MESSAGE_CONTENT", "TEXT_MESSAGE_END":
						// Handle text streaming
						if outputMode == ui.OutputModePretty {
							if content, ok := event["content"].(string); ok && content != "" {
								fmt.Fprint(cmd.OutOrStdout(), content)
							}
						}

					case "MESSAGES_SNAPSHOT":
						// Handle complete messages
						if messages, ok := event["messages"].([]interface{}); ok {
							if outputMode == ui.OutputModePretty {
								for _, msg := range messages {
									if msgMap, ok := msg.(map[string]interface{}); ok {
										role, _ := msgMap["role"].(string)
										content, _ := msgMap["content"].(string)

										if content != "" {
											switch role {
											case "assistant":
												fmt.Fprintf(cmd.OutOrStdout(), "\n🤖 Assistant: %s\n", content)
											case "tool":
												fmt.Fprintf(cmd.OutOrStdout(), "\n🔧 Tool Result: %s\n", content)
											}
										}
									}
								}
							} else if outputMode == ui.OutputModeJSON {
								// Output the full event in JSON mode
								output, _ := json.Marshal(event)
								fmt.Fprintln(cmd.OutOrStdout(), string(output))
							}
						}

					case "RUN_FINISHED":
						if outputMode == ui.OutputModePretty {
							fmt.Fprintln(cmd.OutOrStdout(), "\n✨ Workflow completed")

							// Summary
							if len(toolExecutions) > 0 {
								fmt.Fprintf(cmd.OutOrStdout(), "\n📊 Summary: %d tool(s) executed\n", len(toolExecutions))
								for _, tool := range toolExecutions {
									name, _ := tool["tool_name"].(string)
									fmt.Fprintf(cmd.OutOrStdout(), "   - %s\n", name)
								}
							}
						}
						return nil

					default:
						// Log unknown events
						if outputMode == ui.OutputModeJSON {
							output, _ := json.Marshal(event)
							fmt.Fprintln(cmd.OutOrStdout(), string(output))
						}
					}
				}
			}
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID for the conversation")
	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID for this execution")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON events")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.Flags().BoolVar(&showMetrics, "show-metrics", false, "Show streaming metrics")
	cmd.Flags().StringSliceVar(&tools, "tools", []string{}, "Comma-separated list of allowed tools")
	cmd.Flags().StringVar(&approvalMode, "approval", "auto", "Approval mode: auto, manual, reject")
	cmd.Flags().StringVar(&timeout, "timeout", "30s", "Request timeout duration (e.g., 30s, 5m)")

	return cmd
}

func newStateCommand() *cobra.Command {
	var message string
	var sessionID string
	var runID string
	var jsonOutput bool
	var noColor bool
	var showMetrics bool
	var initialState string
	var watchMode bool

	cmd := &cobra.Command{
		Use:   "state",
		Short: "Manage state with STATE_SNAPSHOT and STATE_DELTA events",
		Long: `Connect to the /agentic_generative_ui endpoint for advanced state management.

This endpoint provides STATE_SNAPSHOT (full state) and STATE_DELTA (incremental updates)
events, enabling sophisticated state synchronization and management workflows.

Examples:
  # Send message with state tracking
  ag-ui-client state --message "Update preferences" --watch
  
  # Initialize with custom state
  ag-ui-client state --message "Process data" --initial-state '{"key": "value"}'
  
  # Get state events in JSON format
  ag-ui-client state --message "Sync state" --json`,
		Run: func(cmd *cobra.Command, args []string) {
			if message == "" {
				logger.Error("Message is required. Use --message flag")
				os.Exit(1)
			}

			cfg := configManager.GetConfig()
			if cfg.ServerURL == "" {
				logger.Error("Server URL not configured")
				os.Exit(1)
			}

			// If no session ID provided, generate one
			if sessionID == "" {
				sessionID = uuid.New().String()
			}
			if runID == "" {
				runID = fmt.Sprintf("run-%d", time.Now().UnixNano())
			}

			// Build endpoint URL
			endpoint := strings.TrimSuffix(cfg.ServerURL, "/") + "/agentic_generative_ui"

			// Parse initial state if provided
			state := map[string]interface{}{}
			if initialState != "" {
				if err := json.Unmarshal([]byte(initialState), &state); err != nil {
					logger.WithError(err).Error("Failed to parse initial state")
					os.Exit(1)
				}
			}

			// Prepare request payload
			payload := map[string]interface{}{
				"thread_id": sessionID,
				"run_id":    runID,
				"messages": []map[string]interface{}{
					{
						"id":      fmt.Sprintf("msg-%d", time.Now().UnixNano()),
						"role":    "user",
						"content": message,
					},
				},
				"state":          state,
				"tools":          []string{},
				"context":        []interface{}{},
				"forwardedProps": map[string]interface{}{},
			}

			// Initialize metrics if requested
			var streamMetrics *streamingpkg.StreamMetrics
			if showMetrics {
				streamMetrics = streamingpkg.NewStreamMetrics()
				defer func() {
					if streamMetrics != nil {
						snapshot := streamMetrics.GetSnapshot()
						fmt.Fprintln(os.Stderr, streamingpkg.FormatMetrics(snapshot))
					}
				}()
			}

			// Setup output mode
			outputMode := ui.OutputModePretty
			if jsonOutput {
				outputMode = ui.OutputModeJSON
			}

			// Marshal request
			requestBody, err := json.Marshal(payload)
			if err != nil {
				logger.WithError(err).Error("Failed to marshal request")
				os.Exit(1)
			}

			// Create HTTP request
			req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestBody))
			if err != nil {
				logger.WithError(err).Error("Failed to create request")
				os.Exit(1)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "text/event-stream")

			// Add authentication if configured
			if cfg.APIKey != "" {
				authHeader := cfg.AuthHeader
				if authHeader == "" {
					authHeader = "Authorization"
				}
				if authHeader == "Authorization" && cfg.AuthScheme != "" {
					req.Header.Set(authHeader, fmt.Sprintf("%s %s", cfg.AuthScheme, cfg.APIKey))
				} else {
					req.Header.Set(authHeader, cfg.APIKey)
				}
			}

			// Create context with cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle interrupt signals
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigChan) // Clean up signal handler

			go func() {
				select {
				case <-sigChan:
					logger.Debug("Received interrupt signal")
					cancel()
				case <-ctx.Done():
					return // Exit goroutine when context is done
				}
			}()

			// Make the request
			client := &http.Client{Timeout: 0} // No timeout for SSE
			resp, err := client.Do(req.WithContext(ctx))
			if err != nil {
				logger.WithError(err).Error("Failed to connect to server")
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				logger.WithFields(logrus.Fields{
					"status": resp.StatusCode,
					"body":   string(body),
				}).Error("Server returned error")
				os.Exit(1)
			}

			// Parse SSE stream
			reader := bufio.NewReader(resp.Body)

			// Start parsing in the background
			frames := make(chan sse.Frame)
			errors := make(chan error)

			go func() {
				defer close(frames)
				defer close(errors)

				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							errors <- err
						}
						return
					}

					if strings.HasPrefix(line, "data: ") {
						data := strings.TrimPrefix(line, "data: ")
						data = strings.TrimSpace(data)
						if data != "" && data != "[DONE]" {
							frames <- sse.Frame{Data: []byte(data)}
						}
					}
				}
			}()

			// Track state
			currentState := make(map[string]interface{})
			stateVersions := []map[string]interface{}{}

			// Process events
			logger.Debug("Starting to process SSE events from state management endpoint")

			for {
				select {
				case <-ctx.Done():
					return

				case err := <-errors:
					if err != nil && err != io.EOF {
						logger.WithError(err).Error("SSE stream error")
					}
					return

				case frame := <-frames:
					if frame.Data == nil || len(frame.Data) == 0 {
						continue
					}

					// Update metrics
					if streamMetrics != nil {
						streamMetrics.RecordEvent(int64(len(frame.Data)))
					}

					// Parse event
					var event map[string]interface{}
					if err := json.Unmarshal(frame.Data, &event); err != nil {
						logger.WithField("raw", string(frame.Data)).Debug("Received non-JSON frame")
						continue
					}

					eventType, _ := event["type"].(string)

					// Log all events in debug mode
					logger.WithFields(logrus.Fields{
						"type": eventType,
						"data": event,
					}).Debug("Received event")

					switch eventType {
					case "RUN_STARTED":
						if outputMode == ui.OutputModePretty {
							fmt.Fprintln(os.Stdout, "\n🔄 State management session started")
							if watchMode {
								fmt.Fprintln(os.Stdout, "👁️  Watching for state changes...")
							}
						}

					case "STATE_SNAPSHOT":
						// Full state update
						if stateData, ok := event["snapshot"].(map[string]interface{}); ok {
							currentState = stateData
							stateVersions = append(stateVersions, map[string]interface{}{
								"type":      "snapshot",
								"state":     stateData,
								"timestamp": time.Now(),
							})

							if outputMode == ui.OutputModePretty {
								fmt.Fprintln(os.Stdout, "\n📸 State Snapshot received:")
								stateJSON, _ := json.MarshalIndent(stateData, "   ", "  ")
								fmt.Fprintf(os.Stdout, "   %s\n", stateJSON)
							} else if outputMode == ui.OutputModeJSON {
								output, _ := json.Marshal(event)
								fmt.Fprintln(cmd.OutOrStdout(), string(output))
							}
						}

					case "STATE_DELTA":
						// Incremental state update - handle both object and JSON Patch formats
						if deltaObj, ok := event["delta"].(map[string]interface{}); ok {
							// Simple object delta format
							stateVersions = append(stateVersions, map[string]interface{}{
								"type":      "delta",
								"delta":     deltaObj,
								"timestamp": time.Now(),
							})

							// Apply delta to current state
							for key, value := range deltaObj {
								currentState[key] = value
							}

							if outputMode == ui.OutputModePretty {
								fmt.Fprintln(os.Stdout, "\n🔄 State Delta received:")
								deltaJSON, _ := json.MarshalIndent(deltaObj, "   ", "  ")
								fmt.Fprintf(os.Stdout, "   %s\n", deltaJSON)

								// Show updated state
								fmt.Fprintln(os.Stdout, "\n📊 Current State:")
								stateJSON, _ := json.MarshalIndent(currentState, "   ", "  ")
								fmt.Fprintf(os.Stdout, "   %s\n", stateJSON)
							} else if outputMode == ui.OutputModeJSON {
								output, _ := json.Marshal(event)
								fmt.Fprintln(cmd.OutOrStdout(), string(output))
							}
						} else if operations, ok := event["delta"].([]interface{}); ok {
							stateVersions = append(stateVersions, map[string]interface{}{
								"type":       "delta",
								"operations": operations,
								"timestamp":  time.Now(),
							})

							if outputMode == ui.OutputModePretty {
								fmt.Fprintln(os.Stdout, "\n🔄 State Delta received:")
								for _, op := range operations {
									if opMap, ok := op.(map[string]interface{}); ok {
										opType, _ := opMap["op"].(string)
										path, _ := opMap["path"].(string)
										value := opMap["value"]

										fmt.Fprintf(os.Stdout, "   %s: %s", opType, path)
										if value != nil {
											valueJSON, _ := json.Marshal(value)
											fmt.Fprintf(os.Stdout, " = %s", valueJSON)
										}
										fmt.Fprintln(os.Stdout)

										// Apply patch to current state (simplified)
										switch opType {
										case "add", "replace":
											// In real implementation, would use JSON Patch library
											pathParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
											if len(pathParts) > 0 {
												// Simplified: just update top-level keys
												currentState[pathParts[0]] = value
											}
										case "remove":
											pathParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
											if len(pathParts) > 0 {
												delete(currentState, pathParts[0])
											}
										}
									}
								}

								// Show updated state
								fmt.Fprintln(os.Stdout, "\n📊 Current State:")
								stateJSON, _ := json.MarshalIndent(currentState, "   ", "  ")
								fmt.Fprintf(os.Stdout, "   %s\n", stateJSON)
							} else if outputMode == ui.OutputModeJSON {
								output, _ := json.Marshal(event)
								fmt.Fprintln(cmd.OutOrStdout(), string(output))
							}
						}

					case "TEXT_MESSAGE_START", "TEXT_MESSAGE_CONTENT", "TEXT_MESSAGE_END":
						// Handle text streaming
						if outputMode == ui.OutputModePretty {
							if content, ok := event["content"].(string); ok && content != "" {
								fmt.Fprint(os.Stdout, content)
							}
						}

					case "MESSAGES_SNAPSHOT":
						// Handle complete messages
						if messages, ok := event["messages"].([]interface{}); ok {
							for _, msg := range messages {
								if msgMap, ok := msg.(map[string]interface{}); ok {
									role, _ := msgMap["role"].(string)
									content, _ := msgMap["content"].(string)

									if outputMode == ui.OutputModePretty && content != "" {
										switch role {
										case "assistant":
											fmt.Fprintf(os.Stdout, "\n🤖 Assistant: %s\n", content)
										}
									}
								}
							}
						}

					case "RUN_FINISHED":
						if outputMode == ui.OutputModePretty {
							fmt.Fprintln(os.Stdout, "\n✨ State management session completed")

							// Summary
							if len(stateVersions) > 0 {
								fmt.Fprintf(os.Stdout, "\n📊 Summary:\n")
								fmt.Fprintf(os.Stdout, "   - State versions: %d\n", len(stateVersions))

								snapshotCount := 0
								deltaCount := 0
								for _, v := range stateVersions {
									if vType, ok := v["type"].(string); ok {
										if vType == "snapshot" {
											snapshotCount++
										} else if vType == "delta" {
											deltaCount++
										}
									}
								}
								fmt.Fprintf(os.Stdout, "   - Snapshots: %d\n", snapshotCount)
								fmt.Fprintf(os.Stdout, "   - Deltas: %d\n", deltaCount)

								// Final state
								fmt.Fprintln(os.Stdout, "\n📋 Final State:")
								stateJSON, _ := json.MarshalIndent(currentState, "   ", "  ")
								fmt.Fprintf(os.Stdout, "   %s\n", stateJSON)
							}
						}

						if !watchMode {
							return
						}

					default:
						// Log unknown events
						if outputMode == ui.OutputModeJSON {
							output, _ := json.Marshal(event)
							fmt.Println(string(output))
						}
					}
				}
			}
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID for the conversation")
	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID for this execution")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON events")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.Flags().BoolVar(&showMetrics, "show-metrics", false, "Show streaming metrics")
	cmd.Flags().StringVar(&initialState, "initial-state", "", "Initial state as JSON string")
	cmd.Flags().BoolVar(&watchMode, "watch", false, "Continue watching for state changes after completion")

	return cmd
}

func newPredictiveCommand() *cobra.Command {
	var message string
	var sessionID string
	var runID string
	var jsonOutput bool
	var noColor bool
	var showMetrics bool
	var watchMode bool
	var interactive bool

	cmd := &cobra.Command{
		Use:   "predictive",
		Short: "Interact with predictive state updates for real-time document generation",
		Long: `Connect to the /predictive_state_updates endpoint for advanced predictive workflows.

This endpoint provides:
- PredictState custom events for UI predictions
- Incremental tool argument streaming for real-time document updates
- Progressive document generation with word-by-word display
- Two-phase tool execution (write_document_local + confirm_changes)

Examples:
  # Generate a document with predictive updates
  ag-ui-client predictive --message "Write a story"
  
  # Watch for continuous updates
  ag-ui-client predictive --message "Create content" --watch
  
  # Non-interactive mode for automation
  ag-ui-client predictive --message "Generate text" --interactive=false
  
  # Get raw events in JSON format
  ag-ui-client predictive --message "Process data" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if message == "" {
				logger.Error("Message is required. Use --message flag")
				return fmt.Errorf("message is required")
			}

			cfg := configManager.GetConfig()
			if cfg.ServerURL == "" {
				logger.Error("Server URL not configured")
				return fmt.Errorf("server URL not configured")
			}

			// If no session ID provided, generate one
			if sessionID == "" {
				sessionID = uuid.New().String()
			}
			if runID == "" {
				runID = fmt.Sprintf("run-%d", time.Now().UnixNano())
			}

			// Build endpoint URL
			endpoint := strings.TrimSuffix(cfg.ServerURL, "/") + "/predictive_state_updates"

			// Prepare request payload with proper structure
			payload := map[string]interface{}{
				"threadId": sessionID, // Note: using camelCase for consistency
				"runId":    runID,
				"messages": []map[string]interface{}{
					{
						"id":      fmt.Sprintf("msg-%d", time.Now().UnixNano()),
						"role":    "user",
						"content": message,
					},
				},
				"state":          map[string]interface{}{},
				"tools":          []string{},
				"context":        []interface{}{},
				"forwardedProps": map[string]interface{}{},
			}

			// Marshal request
			requestBody, err := json.Marshal(payload)
			if err != nil {
				logger.WithError(err).Error("Failed to marshal request")
				os.Exit(1)
			}

			// Initialize metrics if requested
			var streamMetrics *streamingpkg.StreamMetrics
			if showMetrics {
				streamMetrics = streamingpkg.NewStreamMetrics()
				defer func() {
					if streamMetrics != nil {
						snapshot := streamMetrics.GetSnapshot()
						fmt.Fprintln(os.Stderr, streamingpkg.FormatMetrics(snapshot))
					}
				}()
			}

			// Setup output mode
			outputMode := ui.OutputModePretty
			if jsonOutput {
				outputMode = ui.OutputModeJSON
			}

			// Create HTTP request
			req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestBody))
			if err != nil {
				logger.WithError(err).Error("Failed to create request")
				os.Exit(1)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "text/event-stream")

			// Add authentication if configured
			if cfg.APIKey != "" {
				authHeader := cfg.AuthHeader
				if authHeader == "" {
					authHeader = "Authorization"
				}
				if authHeader == "Authorization" && cfg.AuthScheme != "" {
					req.Header.Set(authHeader, fmt.Sprintf("%s %s", cfg.AuthScheme, cfg.APIKey))
				} else {
					req.Header.Set(authHeader, cfg.APIKey)
				}
			}

			// Create context with cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle interrupt signals
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigChan) // Clean up signal handler

			go func() {
				select {
				case <-sigChan:
					logger.Debug("Received interrupt signal")
					cancel()
				case <-ctx.Done():
					return // Exit goroutine when context is done
				}
			}()

			// Make the request
			client := &http.Client{Timeout: 0} // No timeout for SSE
			resp, err := client.Do(req.WithContext(ctx))
			if err != nil {
				logger.WithError(err).Error("Failed to connect to server")
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				logger.WithFields(logrus.Fields{
					"status": resp.StatusCode,
					"body":   string(body),
				}).Error("Server returned error")
				os.Exit(1)
			}

			// Track state for document building
			var documentBuffer strings.Builder
			var toolCallInProgress bool
			var toolResults []interface{}

			// Create a scanner to read SSE events
			scanner := bufio.NewScanner(resp.Body)
			var eventData []byte

			// Handle events
			for scanner.Scan() {
				line := scanner.Text()

				// SSE format: data: {json}
				if strings.HasPrefix(line, "data: ") {
					eventData = []byte(strings.TrimPrefix(line, "data: "))

					// Update metrics
					if streamMetrics != nil {
						streamMetrics.RecordEvent(int64(len(eventData)))
					}

					// Parse event
					var event map[string]interface{}
					if err := json.Unmarshal(eventData, &event); err != nil {
						logger.WithField("raw", string(eventData)).Debug("Received non-JSON frame")
						continue
					}

					// Extract event type
					eventType, _ := event["type"].(string)

					// Handle based on output mode
					if outputMode == ui.OutputModeJSON {
						// In JSON mode, just output the raw event
						if output, err := json.Marshal(event); err == nil {
							fmt.Println(string(output))
						}
					} else {
						// Pretty mode with specialized rendering
						switch eventType {
						case "RUN_STARTED":
							if !noColor {
								fmt.Println("\n🚀 Predictive generation started")
							}

						case "CUSTOM":
							// Check if this is a PredictState event
							if name, ok := event["name"].(string); ok && name == "PredictState" {
								if !noColor {
									fmt.Println("\n📊 Predictive State Configuration:")
									fmt.Println("   ╭─────────────────────────────────────────╮")
								}

								// Parse and display the prediction details
								if value, ok := event["value"].([]interface{}); ok && len(value) > 0 {
									if prediction, ok := value[0].(map[string]interface{}); ok {
										if !noColor {
											if stateKey, ok := prediction["state_key"].(string); ok {
												fmt.Printf("   │ State Key:     %-24s │\n", stateKey)
											}
											if tool, ok := prediction["tool"].(string); ok {
												fmt.Printf("   │ Tool:          %-24s │\n", tool)
											}
											if toolArg, ok := prediction["tool_argument"].(string); ok {
												fmt.Printf("   │ Tool Argument: %-24s │\n", toolArg)
											}
											fmt.Println("   ╰─────────────────────────────────────────╯")
										}
									}
								}
							}

						case "TOOL_CALL_START":
							toolCallInProgress = true
							documentBuffer.Reset()
							// Check for both camelCase and snake_case
							toolName, ok := event["toolCallName"].(string)
							if !ok {
								toolName, ok = event["tool_call_name"].(string)
							}
							if !ok {
								toolName, ok = event["toolName"].(string)
							}
							if !ok {
								toolName, ok = event["name"].(string)
							}
							if ok && !noColor {
								fmt.Printf("\n🔧 Executing tool: %s\n", toolName)
								if toolName == "write_document_local" {
									fmt.Println("\n📝 Document content (streaming):")
									fmt.Println("   ╭─────────────────────────────────────────╮")
									fmt.Print("   │ ")
								}
							}

						case "TOOL_CALL_ARGS":
							if toolCallInProgress {
								if delta, ok := event["delta"].(string); ok {
									// Parse the incremental JSON to extract document text
									if strings.Contains(delta, `{"document":"`) {
										// Start of document
										delta = strings.TrimPrefix(delta, `{"document":"`)
									} else if strings.Contains(delta, `"}`) {
										// End of document - remove the closing quote and brace
										delta = strings.TrimSuffix(delta, `"}`)
										delta = strings.TrimSuffix(delta, `"`)
									}

									// Add to buffer and display incrementally
									documentBuffer.WriteString(delta)
									if !noColor && delta != "" && delta != " " {
										// Display with word-wrapping
										words := strings.Fields(delta)
										for _, word := range words {
											fmt.Print(word + " ")
											time.Sleep(50 * time.Millisecond) // Small delay for visual effect
										}
									}
								}
							}

						case "TOOL_CALL_END":
							if toolCallInProgress {
								toolCallInProgress = false
								if !noColor && documentBuffer.Len() > 0 {
									fmt.Println()
									fmt.Println("   ╰─────────────────────────────────────────╯")

									// Store the result
									toolResults = append(toolResults, map[string]interface{}{
										"type":    "document",
										"content": documentBuffer.String(),
									})
								}
							}

						case "TEXT_MESSAGE_START":
							if !noColor {
								if role, ok := event["role"].(string); ok && role == "assistant" {
									fmt.Println("\n💬 Assistant response:")
								}
							}

						case "TEXT_MESSAGE_CONTENT":
							if delta, ok := event["delta"].(string); ok {
								if !noColor {
									fmt.Print(delta)
								}
							}

						case "TEXT_MESSAGE_END":
							if !noColor {
								fmt.Println()
							}

						case "RUN_FINISHED":
							if !noColor {
								fmt.Println("\n✅ Predictive generation complete")
							}

							// Interactive prompt if enabled and we have results
							if interactive && len(toolResults) > 0 {
								p := prompt.New()
								action, err := p.AskForAction("Apply document changes?")
								if err != nil {
									logger.WithError(err).Debug("Failed to get user action")
									return nil
								}

								switch action {
								case prompt.ActionApply:
									fmt.Println("\n✅ Document applied successfully")
									return nil
								case prompt.ActionRegenerate:
									fmt.Println("\n🔄 Regenerating document...")
									// Would need to implement regeneration logic here
									return nil
								case prompt.ActionCancel:
									fmt.Println("\n❌ Operation cancelled")
									return nil
								}
							}

							if !watchMode {
								return nil
							}
						}
					}
				}
			}

			// Check for scanner errors
			if err := scanner.Err(); err != nil {
				logger.WithError(err).Error("Error reading stream")
				return fmt.Errorf("error reading stream: %w", err)
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID for the conversation")
	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID for this execution")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON events")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.Flags().BoolVar(&showMetrics, "show-metrics", false, "Show streaming metrics")
	cmd.Flags().BoolVar(&watchMode, "watch", false, "Continue watching for updates after completion")
	cmd.Flags().BoolVar(&interactive, "interactive", true, "Enable interactive prompts")

	return cmd
}

func newSharedCommand() *cobra.Command {
	var message string
	var sessionID string
	var runID string
	var jsonOutput bool
	var noColor bool
	var showMetrics bool
	var initialState string
	var watchMode bool
	var interactive bool

	cmd := &cobra.Command{
		Use:   "shared",
		Short: "Connect to shared state endpoint for multi-client synchronization",
		Long: `Connect to the /shared_state endpoint for state synchronization between multiple clients.

This endpoint enables real-time state sharing across multiple connected clients,
with STATE_SNAPSHOT for full state updates and STATE_DELTA for incremental changes.

Features:
  • Real-time state synchronization
  • JSON Patch (RFC 6902) delta updates
  • Multi-client state sharing
  • Visual state viewer

Examples:
  # Connect and view shared state
  ag-ui-client shared --message "Join session" --watch
  
  # Initialize with custom state
  ag-ui-client shared --message "Update counter" --initial-state '{"counter": 0}'
  
  # Get state updates in JSON format
  ag-ui-client shared --message "Sync" --json`,
		Run: func(cmd *cobra.Command, args []string) {
			if message == "" {
				logger.Error("Message is required. Use --message flag")
				os.Exit(1)
			}

			cfg := configManager.GetConfig()
			if cfg.ServerURL == "" {
				logger.Error("Server URL not configured")
				os.Exit(1)
			}

			// If no session ID provided, generate one
			if sessionID == "" {
				sessionID = uuid.New().String()
			}
			if runID == "" {
				runID = fmt.Sprintf("run-%d", time.Now().UnixNano())
			}

			// Build endpoint URL
			endpoint := strings.TrimSuffix(cfg.ServerURL, "/") + "/shared_state"

			// Parse initial state if provided
			state := map[string]interface{}{}
			if initialState != "" {
				if err := json.Unmarshal([]byte(initialState), &state); err != nil {
					logger.WithError(err).Error("Failed to parse initial state")
					os.Exit(1)
				}
			}

			// Prepare request payload
			payload := map[string]interface{}{
				"thread_id": sessionID,
				"run_id":    runID,
				"messages": []map[string]interface{}{
					{
						"id":      fmt.Sprintf("msg-%d", time.Now().UnixNano()),
						"role":    "user",
						"content": message,
					},
				},
				"state":          state,
				"tools":          []string{},
				"context":        []interface{}{},
				"forwardedProps": map[string]interface{}{},
			}

			// Initialize metrics if requested
			var streamMetrics *streamingpkg.StreamMetrics
			if showMetrics {
				streamMetrics = streamingpkg.NewStreamMetrics()
				defer func() {
					if streamMetrics != nil {
						snapshot := streamMetrics.GetSnapshot()
						fmt.Fprintln(os.Stderr, streamingpkg.FormatMetrics(snapshot))
					}
				}()
			}

			// Setup output mode
			outputMode := ui.OutputModePretty
			if jsonOutput {
				outputMode = ui.OutputModeJSON
			}

			// Marshal request
			requestBody, err := json.Marshal(payload)
			if err != nil {
				logger.WithError(err).Error("Failed to marshal request")
				os.Exit(1)
			}

			// Create HTTP request
			req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestBody))
			if err != nil {
				logger.WithError(err).Error("Failed to create request")
				os.Exit(1)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "text/event-stream")

			// Add authentication if configured
			if cfg.APIKey != "" {
				authHeader := cfg.AuthHeader
				if authHeader == "" {
					authHeader = "Authorization"
				}
				if authHeader == "Authorization" && cfg.AuthScheme != "" {
					req.Header.Set(authHeader, fmt.Sprintf("%s %s", cfg.AuthScheme, cfg.APIKey))
				} else {
					req.Header.Set(authHeader, cfg.APIKey)
				}
			}

			// Create context with cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle interrupt signals
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigChan) // Clean up signal handler

			go func() {
				select {
				case <-sigChan:
					logger.Debug("Received interrupt signal")
					cancel()
				case <-ctx.Done():
					return // Exit goroutine when context is done
				}
			}()

			// Make the request
			client := &http.Client{Timeout: 0} // No timeout for SSE
			resp, err := client.Do(req.WithContext(ctx))
			if err != nil {
				logger.WithError(err).Error("Failed to connect to server")
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				logger.WithFields(logrus.Fields{
					"status": resp.StatusCode,
					"body":   string(body),
				}).Error("Server returned error")
				os.Exit(1)
			}

			// State tracking
			var currentState map[string]interface{}

			// Process SSE stream
			logger.Debug("Connected to shared state endpoint, processing events...")
			scanner := bufio.NewScanner(resp.Body)
			scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

			var buffer bytes.Buffer
			eventCount := 0

			for scanner.Scan() {
				line := scanner.Text()

				if strings.HasPrefix(line, "data: ") {
					data := strings.TrimPrefix(line, "data: ")
					buffer.WriteString(data)
				} else if line == "" && buffer.Len() > 0 {
					// Process complete event
					eventData := buffer.String()
					buffer.Reset()

					// Parse event
					var event map[string]interface{}
					if err := json.Unmarshal([]byte(eventData), &event); err != nil {
						logger.WithField("raw", eventData).Debug("Failed to parse event")
						continue
					}

					eventCount++
					eventType, _ := event["type"].(string)

					// Track metrics
					if streamMetrics != nil {
						streamMetrics.RecordEvent(int64(len(eventData)))
					}

					// Handle different event types
					switch eventType {
					case "RUN_STARTED":
						if outputMode == ui.OutputModePretty {
							fmt.Println("🔄 Shared state session started")
							threadID, _ := event["threadId"].(string)
							runID, _ := event["runId"].(string)
							fmt.Printf("   Session: %s\n   Run: %s\n\n", threadID, runID)
						} else {
							fmt.Println(eventData)
						}

					case "STATE_SNAPSHOT":
						// Full state update
						if snapshot, ok := event["snapshot"].(map[string]interface{}); ok {
							currentState = snapshot
							if outputMode == ui.OutputModePretty {
								fmt.Println("📊 State Snapshot:")
								displaySharedState(currentState)
								fmt.Println()
							} else {
								fmt.Println(eventData)
							}
						}

					case "STATE_DELTA":
						// Incremental state update using JSON Patch
						if delta, ok := event["delta"].([]interface{}); ok {
							if outputMode == ui.OutputModePretty {
								fmt.Println("🔧 State Delta (JSON Patch operations):")
								for _, op := range delta {
									if opMap, ok := op.(map[string]interface{}); ok {
										operation, _ := opMap["op"].(string)
										path, _ := opMap["path"].(string)
										value := opMap["value"]

										// Apply the patch to current state
										applyJSONPatch(&currentState, operation, path, value)

										// Display the operation
										fmt.Printf("   %s %s", operation, path)
										if value != nil {
											valueJSON, _ := json.Marshal(value)
											fmt.Printf(" = %s", string(valueJSON))
										}
										fmt.Println()
									}
								}

								// Show updated state
								fmt.Println("\n📊 Updated State:")
								displaySharedState(currentState)
								fmt.Println()
							} else {
								fmt.Println(eventData)
							}
						}

					case "MESSAGES_SNAPSHOT":
						if outputMode == ui.OutputModePretty {
							if messages, ok := event["messages"].([]interface{}); ok {
								for _, msg := range messages {
									if msgMap, ok := msg.(map[string]interface{}); ok {
										role, _ := msgMap["role"].(string)
										content, _ := msgMap["content"].(string)

										if role == "assistant" && content != "" {
											fmt.Printf("Assistant: %s\n\n", content)
										}
									}
								}
							}
						} else {
							fmt.Println(eventData)
						}

					case "TEXT_MESSAGE_START":
						if outputMode == ui.OutputModePretty {
							fmt.Print("Assistant: ")
						} else {
							fmt.Println(eventData)
						}

					case "TEXT_MESSAGE_CONTENT":
						if outputMode == ui.OutputModePretty {
							if delta, ok := event["delta"].(string); ok {
								fmt.Print(delta)
							}
						} else {
							fmt.Println(eventData)
						}

					case "TEXT_MESSAGE_END":
						if outputMode == ui.OutputModePretty {
							fmt.Println()
						} else {
							fmt.Println(eventData)
						}

					case "RUN_FINISHED":
						if outputMode == ui.OutputModePretty {
							fmt.Println("✅ Shared state session completed")

							// Display final state if available
							if len(currentState) > 0 {
								fmt.Println("\n📊 Final Shared State:")
								displaySharedState(currentState)
							}

							if showMetrics {
								fmt.Printf("\n📈 Processed %d events\n", eventCount)
							}
						} else {
							fmt.Println(eventData)
						}

						if !watchMode {
							return
						}

					case "RUN_ERROR":
						errorMsg, _ := event["message"].(string)
						logger.WithField("error", errorMsg).Error("Run error")
						if outputMode == ui.OutputModePretty {
							fmt.Printf("❌ Error: %s\n", errorMsg)
						} else {
							fmt.Println(eventData)
						}
						os.Exit(1)

					default:
						if outputMode == ui.OutputModeJSON {
							fmt.Println(eventData)
						}
					}
				}
			}

			if err := scanner.Err(); err != nil {
				logger.WithError(err).Error("Error reading stream")
				os.Exit(1)
			}
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send (required)")
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID (auto-generated if not provided)")
	cmd.Flags().StringVar(&runID, "run", "", "Run ID (auto-generated if not provided)")
	cmd.Flags().StringVar(&initialState, "initial-state", "", "Initial state as JSON")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON events")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.Flags().BoolVar(&showMetrics, "show-metrics", false, "Show streaming metrics")
	cmd.Flags().BoolVar(&watchMode, "watch", false, "Continue watching for state updates")
	cmd.Flags().BoolVar(&interactive, "interactive", true, "Enable interactive features")

	return cmd
}

// displaySharedState displays the shared state in a formatted way
func displaySharedState(state map[string]interface{}) {
	if len(state) == 0 {
		fmt.Println("  (empty state)")
		return
	}

	// Pretty print the state as JSON
	stateJSON, err := json.MarshalIndent(state, "  ", "  ")
	if err != nil {
		fmt.Printf("  Error formatting state: %v\n", err)
		return
	}
	fmt.Println(string(stateJSON))
}

// applyJSONPatch applies a JSON Patch operation to the state
func applyJSONPatch(state *map[string]interface{}, op string, path string, value interface{}) {
	// Remove leading slash from path
	path = strings.TrimPrefix(path, "/")
	keys := strings.Split(path, "/")

	switch op {
	case "add", "replace":
		// Navigate to the parent and set the value
		current := *state
		for i := 0; i < len(keys)-1; i++ {
			key := keys[i]
			if _, exists := current[key]; !exists {
				current[key] = make(map[string]interface{})
			}
			if next, ok := current[key].(map[string]interface{}); ok {
				current = next
			}
		}
		if len(keys) > 0 {
			current[keys[len(keys)-1]] = value
		}

	case "remove":
		// Navigate to parent and remove the key
		current := *state
		for i := 0; i < len(keys)-1; i++ {
			key := keys[i]
			if next, ok := current[key].(map[string]interface{}); ok {
				current = next
			} else {
				return
			}
		}
		if len(keys) > 0 {
			delete(current, keys[len(keys)-1])
		}

	case "copy", "move":
		// These operations would need more complex handling
		// For now, we'll just log them
		logger.WithFields(logrus.Fields{
			"op":   op,
			"path": path,
		}).Debug("Complex patch operation not fully implemented")
	}
}

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage AG-UI client configuration",
		Long: `Manage and display AG-UI client configuration settings.

Examples:
  # Show current configuration
  ag-ui-client config show
  
  # Show configuration file paths
  ag-ui-client config paths
  
  # Show effective configuration in JSON
  ag-ui-client config show --output json`,
	}

	// Add subcommands
	cmd.AddCommand(newConfigShowCommand())
	cmd.AddCommand(newConfigPathsCommand())
	cmd.AddCommand(newConfigGetCommand())
	cmd.AddCommand(newConfigSetCommand())
	cmd.AddCommand(newConfigUnsetCommand())

	return cmd
}

func newConfigShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show effective configuration",
		Long: `Display the current effective configuration including environment variables and defaults.

Examples:
  # Show configuration
  ag-ui-client config show
  
  # Show configuration in JSON format
  ag-ui-client config show --output json`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg := configManager.GetConfig()
			if cfg.Output == "json" {
				jsonStr, _ := configManager.ToJSON(true)
				fmt.Println(jsonStr)
			} else {
				logger.Info("Current Configuration:")
				logger.Infof("  Server URL: %s", cfg.ServerURL)
				logger.Infof("  API Key: %s", maskAPIKey(cfg.APIKey))
				logger.Infof("  Log Level: %s", cfg.LogLevel)
				logger.Infof("  Log Format: %s", cfg.LogFormat)
				logger.Infof("  Output Format: %s", cfg.Output)
				if cfg.LastSessionID != "" {
					logger.Infof("  Last Session ID: %s", cfg.LastSessionID)
				}
			}
		},
	}

	return cmd
}

func newConfigPathsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "paths",
		Short: "Show configuration file paths",
		Long: `Display the paths where configuration files are searched and loaded.

Examples:
  # Show config paths
  ag-ui-client config paths
  
  # Show paths in JSON format
  ag-ui-client config paths --output json`,
		Run: func(cmd *cobra.Command, args []string) {
			paths := configManager.GetConfigPaths()
			envVars := configManager.GetEnvironmentVariables()

			if configManager.GetConfig().Output == "json" {
				output := map[string]interface{}{
					"config_file":  configManager.GetConfigPath(),
					"search_paths": paths,
					"env_vars":     envVars,
				}
				data, _ := json.MarshalIndent(output, "", "  ")
				fmt.Println(string(data))
			} else {
				logger.Info("Configuration file location:")
				logger.Infof("  %s", configManager.GetConfigPath())
				logger.Info("Configuration search paths:")
				for _, path := range paths {
					logger.Infof("  - %s", path)
				}
				logger.Info("Environment variables:")
				for _, env := range envVars {
					logger.Infof("  - %s", env)
				}
			}
		},
	}

	return cmd
}

func newConfigGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long: `Get a specific configuration value by key.

Available keys:
  - server, serverurl
  - apikey, api_key, api-key
  - loglevel, log_level, log-level
  - logformat, log_format, log-format
  - output
  - lastsessionid, last_session_id, last-session-id

Examples:
  # Get server URL
  ag-ui-client config get server
  
  # Get API key (will be redacted)
  ag-ui-client config get api_key
  
  # Get output format in JSON
  ag-ui-client config get output --output json`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value, err := configManager.Get(key)

			// Redact API key if requested
			if (key == "apikey" || key == "api_key" || key == "api-key") && value != "" {
				value = maskAPIKey(value)
			}

			if configManager.GetConfig().Output == "json" {
				output := map[string]interface{}{
					"key":   key,
					"value": value,
				}
				if err != nil {
					output["error"] = err.Error()
				}
				data, _ := json.MarshalIndent(output, "", "  ")
				fmt.Println(string(data))
			} else {
				if err != nil {
					logger.Errorf("Error: %v", err)
					os.Exit(1)
				}
				fmt.Println(value)
			}
		},
	}

	return cmd
}

func newConfigSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value and persist it to the config file.

Available keys:
  - server, serverurl
  - apikey, api_key, api-key
  - loglevel, log_level, log-level
  - logformat, log_format, log-format
  - output
  - lastsessionid, last_session_id, last-session-id
  - Any custom key (stored in extras)

Examples:
  # Set server URL
  ag-ui-client config set server https://api.example.com
  
  # Set API key
  ag-ui-client config set api_key your-secret-key
  
  # Set log level
  ag-ui-client config set log_level debug
  
  # Set custom value
  ag-ui-client config set custom_setting value123`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]

			// Set the value
			if err := configManager.Set(key, value); err != nil {
				logger.Errorf("Failed to set config value: %v", err)
				os.Exit(1)
			}

			// Save to file
			if err := configManager.SaveToFile(); err != nil {
				logger.Errorf("Failed to save config file: %v", err)
				os.Exit(1)
			}

			// Mask API key in output
			displayValue := value
			if key == "apikey" || key == "api_key" || key == "api-key" {
				displayValue = maskAPIKey(value)
			}

			if configManager.GetConfig().Output == "json" {
				output := map[string]interface{}{
					"key":         key,
					"value":       displayValue,
					"saved":       true,
					"config_file": configManager.GetConfigPath(),
				}
				data, _ := json.MarshalIndent(output, "", "  ")
				fmt.Println(string(data))
			} else {
				logger.Infof("Set %s = %s", key, displayValue)
				logger.Infof("Configuration saved to: %s", configManager.GetConfigPath())
			}
		},
	}

	return cmd
}

func newConfigUnsetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "Unset a configuration value",
		Long: `Remove a configuration value from the config file.

Available keys:
  - server, serverurl
  - apikey, api_key, api-key
  - loglevel, log_level, log-level
  - logformat, log_format, log-format
  - output
  - lastsessionid, last_session_id, last-session-id
  - Any custom key in extras

Examples:
  # Unset API key
  ag-ui-client config unset api_key
  
  # Unset last session ID
  ag-ui-client config unset last_session_id
  
  # Unset custom value
  ag-ui-client config unset custom_setting`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]

			// Unset the value
			if err := configManager.Unset(key); err != nil {
				logger.Errorf("Failed to unset config value: %v", err)
				os.Exit(1)
			}

			// Save to file
			if err := configManager.SaveToFile(); err != nil {
				logger.Errorf("Failed to save config file: %v", err)
				os.Exit(1)
			}

			if configManager.GetConfig().Output == "json" {
				output := map[string]interface{}{
					"key":         key,
					"unset":       true,
					"saved":       true,
					"config_file": configManager.GetConfigPath(),
				}
				data, _ := json.MarshalIndent(output, "", "  ")
				fmt.Println(string(data))
			} else {
				logger.Infof("Unset %s", key)
				logger.Infof("Configuration saved to: %s", configManager.GetConfigPath())
			}
		},
	}

	return cmd
}

func containsSubstring(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && containsSubstring(s[1:], substr))
}

func maskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// Tool represents a tool definition from the server
type Tool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Capabilities []string               `json:"capabilities,omitempty"`
}

// fetchToolsFromServer retrieves available tools from the AG-UI server
func fetchToolsFromServer(cfg *config.Config) ([]Tool, error) {
	// Create a minimal request to discover tools
	// The server will include available tools in the response
	endpoint := strings.TrimSuffix(cfg.ServerURL, "/") + "/tool_based_generative_ui"

	// Create request payload conforming to RunAgentInput
	payload := map[string]interface{}{
		"threadId": uuid.New().String(),
		"runId":    "run-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"state":    map[string]interface{}{},
		"messages": []map[string]interface{}{
			{
				"id":      "msg-1",
				"role":    "user",
				"content": "",
			},
		},
		"tools":          []interface{}{}, // Request tool discovery
		"context":        []interface{}{},
		"forwardedProps": map[string]interface{}{},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add authentication if configured
	if cfg.APIKey != "" {
		authHeader := cfg.AuthHeader
		if authHeader == "" {
			authHeader = "Authorization"
		}

		if authHeader == "Authorization" {
			authScheme := cfg.AuthScheme
			if authScheme == "" {
				authScheme = "Bearer"
			}
			req.Header.Set(authHeader, fmt.Sprintf("%s %s", authScheme, cfg.APIKey))
		} else {
			req.Header.Set(authHeader, cfg.APIKey)
		}
	}

	// Execute request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	// For now, return sample tools as the server doesn't have a dedicated discovery endpoint
	// In a real implementation, this would parse the server response
	sampleTools := []Tool{
		{
			Name:         "http_get",
			Description:  "Make HTTP GET requests to external APIs",
			Tags:         []string{"network", "http", "api"},
			Capabilities: []string{"async", "retry"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL to fetch",
						"format":      "uri",
					},
					"headers": map[string]interface{}{
						"type":        "object",
						"description": "Optional HTTP headers",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:         "http_post",
			Description:  "Make HTTP POST requests with JSON payloads",
			Tags:         []string{"network", "http", "api"},
			Capabilities: []string{"async", "retry", "streaming"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL to post to",
						"format":      "uri",
					},
					"body": map[string]interface{}{
						"type":        "object",
						"description": "JSON body to send",
					},
					"headers": map[string]interface{}{
						"type":        "object",
						"description": "Optional HTTP headers",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:         "file_read",
			Description:  "Read contents from a file",
			Tags:         []string{"filesystem", "io"},
			Capabilities: []string{"local"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
					"encoding": map[string]interface{}{
						"type":        "string",
						"description": "File encoding (default: utf-8)",
						"enum":        []string{"utf-8", "ascii", "base64"},
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:         "file_write",
			Description:  "Write contents to a file",
			Tags:         []string{"filesystem", "io"},
			Capabilities: []string{"local"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to write",
					},
					"encoding": map[string]interface{}{
						"type":        "string",
						"description": "File encoding (default: utf-8)",
						"enum":        []string{"utf-8", "ascii", "base64"},
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:         "data_transform",
			Description:  "Transform data between different formats",
			Tags:         []string{"data", "transformation"},
			Capabilities: []string{"streaming"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input": map[string]interface{}{
						"type":        "string",
						"description": "Input data",
					},
					"from_format": map[string]interface{}{
						"type":        "string",
						"description": "Source format",
						"enum":        []string{"json", "xml", "csv", "yaml"},
					},
					"to_format": map[string]interface{}{
						"type":        "string",
						"description": "Target format",
						"enum":        []string{"json", "xml", "csv", "yaml"},
					},
				},
				"required": []string{"input", "from_format", "to_format"},
			},
		},
	}

	return sampleTools, nil
}

// filterTools applies client-side filtering to the tools list
func filterTools(tools []Tool, nameFilter, capabilityFilter, tagFilter string) []Tool {
	var filtered []Tool

	for _, tool := range tools {
		// Check name filter
		if nameFilter != "" && !strings.Contains(strings.ToLower(tool.Name), strings.ToLower(nameFilter)) {
			continue
		}

		// Check capability filter
		if capabilityFilter != "" {
			hasCapability := false
			for _, cap := range tool.Capabilities {
				if strings.Contains(strings.ToLower(cap), strings.ToLower(capabilityFilter)) {
					hasCapability = true
					break
				}
			}
			if !hasCapability {
				continue
			}
		}

		// Check tag filter
		if tagFilter != "" {
			hasTag := false
			for _, tag := range tool.Tags {
				if strings.Contains(strings.ToLower(tag), strings.ToLower(tagFilter)) {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		filtered = append(filtered, tool)
	}

	return filtered
}

// renderToolsJSON outputs tools in JSON format (one per line for scripting)
func renderToolsJSON(w io.Writer, tools []Tool, verbose bool) {
	for _, tool := range tools {
		output := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
		}

		if len(tool.Tags) > 0 {
			output["tags"] = tool.Tags
		}

		if len(tool.Capabilities) > 0 {
			output["capabilities"] = tool.Capabilities
		}

		if verbose && tool.Parameters != nil {
			output["schema"] = tool.Parameters
		}

		data, _ := json.Marshal(output)
		fmt.Fprintln(w, string(data))
	}
}

// renderToolsPretty outputs tools in a human-readable table format
func renderToolsPretty(w io.Writer, tools []Tool, verbose, noColor bool) {
	if len(tools) == 0 {
		fmt.Fprintln(w, "No tools found")
		return
	}

	// Print header
	fmt.Fprintln(w, "Available Tools:")
	fmt.Fprintln(w, strings.Repeat("-", 80))

	for i, tool := range tools {
		// Tool name and description
		if !noColor {
			fmt.Fprintf(w, "\033[1;34m%s\033[0m\n", tool.Name)
		} else {
			fmt.Fprintf(w, "%s\n", tool.Name)
		}
		fmt.Fprintf(w, "  %s\n", tool.Description)

		// Tags
		if len(tool.Tags) > 0 {
			fmt.Fprintf(w, "  Tags: %s\n", strings.Join(tool.Tags, ", "))
		}

		// Capabilities
		if len(tool.Capabilities) > 0 {
			fmt.Fprintf(w, "  Capabilities: %s\n", strings.Join(tool.Capabilities, ", "))
		}

		// Parameters (if verbose)
		if verbose && tool.Parameters != nil {
			fmt.Fprintln(w, "  Parameters:")
			renderParameterSchema(w, tool.Parameters, "    ")
		}

		// Add separator between tools (except for last one)
		if i < len(tools)-1 {
			fmt.Fprintln(w, "")
		}
	}

	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintf(w, "Total: %d tools\n", len(tools))
}

// renderParameterSchema recursively renders parameter schema
func renderParameterSchema(w io.Writer, schema map[string]interface{}, indent string) {
	if schemaType, ok := schema["type"].(string); ok {
		fmt.Fprintf(w, "%sType: %s\n", indent, schemaType)
	}

	if props, ok := schema["properties"].(map[string]interface{}); ok {
		fmt.Fprintf(w, "%sProperties:\n", indent)
		for name, prop := range props {
			if propMap, ok := prop.(map[string]interface{}); ok {
				fmt.Fprintf(w, "%s  %s:\n", indent, name)

				if propType, ok := propMap["type"].(string); ok {
					fmt.Fprintf(w, "%s    type: %s\n", indent, propType)
				}

				if desc, ok := propMap["description"].(string); ok {
					fmt.Fprintf(w, "%s    description: %s\n", indent, desc)
				}

				if format, ok := propMap["format"].(string); ok {
					fmt.Fprintf(w, "%s    format: %s\n", indent, format)
				}

				if enum, ok := propMap["enum"].([]interface{}); ok {
					enumStrs := make([]string, len(enum))
					for i, e := range enum {
						enumStrs[i] = fmt.Sprint(e)
					}
					fmt.Fprintf(w, "%s    enum: [%s]\n", indent, strings.Join(enumStrs, ", "))
				}
			}
		}
	}

	// Handle required fields - they might be []interface{} or []string
	if required, ok := schema["required"].([]interface{}); ok {
		reqStrs := make([]string, len(required))
		for i, r := range required {
			reqStrs[i] = fmt.Sprint(r)
		}
		fmt.Fprintf(w, "%sRequired: [%s]\n", indent, strings.Join(reqStrs, ", "))
	} else if required, ok := schema["required"].([]string); ok {
		fmt.Fprintf(w, "%sRequired: [%s]\n", indent, strings.Join(required, ", "))
	}
}

// renderToolDescriptionJSON outputs tool description in JSON format
func renderToolDescriptionJSON(w io.Writer, tool *Tool) {
	output := map[string]interface{}{
		"name":        tool.Name,
		"description": tool.Description,
	}

	if tool.Parameters != nil && len(tool.Parameters) > 0 {
		output["parameters"] = tool.Parameters
	}

	if len(tool.Tags) > 0 {
		output["tags"] = tool.Tags
	}

	if len(tool.Capabilities) > 0 {
		output["capabilities"] = tool.Capabilities
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		logger.WithError(err).Error("Failed to marshal tool description")
		return
	}

	fmt.Fprintln(w, string(jsonBytes))
}

// renderToolDescriptionPretty outputs tool description in a human-readable format
func renderToolDescriptionPretty(w io.Writer, tool *Tool, noColor bool) {
	// Tool name header
	if !noColor {
		fmt.Fprintf(w, "\n\033[1;36m🔧 %s\033[0m\n", tool.Name)
	} else {
		fmt.Fprintf(w, "\n🔧 %s\n", tool.Name)
	}

	// Draw separator line
	fmt.Fprintln(w, strings.Repeat("─", 60))

	// Description
	if !noColor {
		fmt.Fprintf(w, "\n\033[1mDescription:\033[0m\n")
	} else {
		fmt.Fprintln(w, "\nDescription:")
	}

	// Word wrap description for better readability
	wrapped := wordWrap(tool.Description, 60)
	for _, line := range wrapped {
		fmt.Fprintf(w, "  %s\n", line)
	}

	// Tags
	if len(tool.Tags) > 0 {
		if !noColor {
			fmt.Fprintf(w, "\n\033[1mTags:\033[0m\n")
		} else {
			fmt.Fprintln(w, "\nTags:")
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(tool.Tags, ", "))
	}

	// Capabilities
	if len(tool.Capabilities) > 0 {
		if !noColor {
			fmt.Fprintf(w, "\n\033[1mCapabilities:\033[0m\n")
		} else {
			fmt.Fprintln(w, "\nCapabilities:")
		}
		for _, cap := range tool.Capabilities {
			fmt.Fprintf(w, "  • %s\n", cap)
		}
	}

	// Parameters
	if tool.Parameters != nil && len(tool.Parameters) > 0 {
		if !noColor {
			fmt.Fprintf(w, "\n\033[1mParameters:\033[0m\n")
		} else {
			fmt.Fprintln(w, "\nParameters:")
		}
		renderParameterSchema(w, tool.Parameters, "  ")
	}

	// Usage example
	if !noColor {
		fmt.Fprintf(w, "\n\033[1mUsage Example:\033[0m\n")
	} else {
		fmt.Fprintln(w, "\nUsage Example:")
	}

	// Generate example based on tool name
	if tool.Name == "generate_haiku" {
		fmt.Fprintln(w, "  ag-ui-client tools run generate_haiku")
		fmt.Fprintln(w, "  ag-ui-client chat --message \"Generate a haiku about coding\"")
	} else {
		// Generic example
		argsExample := "{}"
		if props, ok := tool.Parameters["properties"].(map[string]interface{}); ok && len(props) > 0 {
			exampleArgs := make(map[string]string)
			for name, prop := range props {
				if propMap, ok := prop.(map[string]interface{}); ok {
					if propType, ok := propMap["type"].(string); ok {
						switch propType {
						case "string":
							exampleArgs[name] = "value"
						case "number", "integer":
							exampleArgs[name] = "123"
						case "boolean":
							exampleArgs[name] = "true"
						case "array":
							exampleArgs[name] = "[]"
						case "object":
							exampleArgs[name] = "{}"
						}
					}
				}
			}
			if len(exampleArgs) > 0 {
				if argsBytes, err := json.Marshal(exampleArgs); err == nil {
					argsExample = string(argsBytes)
				}
			}
		}
		fmt.Fprintf(w, "  ag-ui-client tools run %s --args '%s'\n", tool.Name, argsExample)
	}

	fmt.Fprintln(w)
}

// wordWrap breaks a long string into lines of specified width
func wordWrap(text string, width int) []string {
	if len(text) <= width {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
		} else if currentLine.Len()+1+len(word) <= width {
			currentLine.WriteString(" ")
			currentLine.WriteString(word)
		} else {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

// executeWithRetry executes a function with retry logic
func executeWithRetry(fn func() error, maxRetries int, delay time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logger.WithFields(logrus.Fields{
				"attempt":     attempt,
				"max_retries": maxRetries,
			}).Info("Retrying after failure")
			time.Sleep(delay)
		}

		if err := fn(); err != nil {
			lastErr = err

			// Check if error is retryable
			if !isRetryableError(err) {
				return err
			}

			logger.WithFields(logrus.Fields{
				"error":       err.Error(),
				"attempt":     attempt + 1,
				"max_retries": maxRetries + 1,
			}).Warn("Execution failed, will retry")
		} else {
			// Success
			return nil
		}
	}

	return fmt.Errorf("execution failed after %d retries: %w", maxRetries+1, lastErr)
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network-related errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "EOF") {
		return true
	}

	// HTTP status codes that are retryable
	if strings.Contains(errStr, "502") || // Bad Gateway
		strings.Contains(errStr, "503") || // Service Unavailable
		strings.Contains(errStr, "504") || // Gateway Timeout
		strings.Contains(errStr, "429") { // Too Many Requests
		return true
	}

	// Don't retry on validation errors or client errors
	if strings.Contains(errStr, "validation") ||
		strings.Contains(errStr, "400") || // Bad Request
		strings.Contains(errStr, "401") || // Unauthorized
		strings.Contains(errStr, "403") || // Forbidden
		strings.Contains(errStr, "404") { // Not Found
		return false
	}

	// Default to not retrying unknown errors
	return false
}

// convertToToolSchema converts a raw schema map to a ToolSchema structure
func convertToToolSchema(rawSchema map[string]interface{}) *pkgtools.ToolSchema {
	schema := &pkgtools.ToolSchema{}

	// Extract type
	if typeVal, ok := rawSchema["type"].(string); ok {
		schema.Type = typeVal
	}

	// Extract description
	if desc, ok := rawSchema["description"].(string); ok {
		schema.Description = desc
	}

	// Extract required fields
	if required, ok := rawSchema["required"].([]interface{}); ok {
		for _, req := range required {
			if reqStr, ok := req.(string); ok {
				schema.Required = append(schema.Required, reqStr)
			}
		}
	}

	// Extract additionalProperties
	if addProps, ok := rawSchema["additionalProperties"].(bool); ok {
		schema.AdditionalProperties = &addProps
	}

	// Extract and convert properties
	if props, ok := rawSchema["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*pkgtools.Property)
		for propName, propValue := range props {
			if propMap, ok := propValue.(map[string]interface{}); ok {
				schema.Properties[propName] = convertToProperty(propMap)
			}
		}
	}

	return schema
}

// convertToProperty converts a raw property map to a Property structure
func convertToProperty(rawProp map[string]interface{}) *pkgtools.Property {
	prop := &pkgtools.Property{}

	// Extract type
	if typeVal, ok := rawProp["type"].(string); ok {
		prop.Type = typeVal
	}

	// Extract description
	if desc, ok := rawProp["description"].(string); ok {
		prop.Description = desc
	}

	// Extract format
	if format, ok := rawProp["format"].(string); ok {
		prop.Format = format
	}

	// Extract pattern
	if pattern, ok := rawProp["pattern"].(string); ok {
		prop.Pattern = pattern
	}

	// Extract enum values
	if enum, ok := rawProp["enum"].([]interface{}); ok {
		prop.Enum = enum
	}

	// Extract default value
	if defaultVal, ok := rawProp["default"]; ok {
		prop.Default = defaultVal
	}

	// Extract numeric constraints
	if min, ok := rawProp["minimum"].(float64); ok {
		prop.Minimum = &min
	}
	if max, ok := rawProp["maximum"].(float64); ok {
		prop.Maximum = &max
	}

	// Extract string constraints
	if minLen, ok := rawProp["minLength"].(float64); ok {
		minLenInt := int(minLen)
		prop.MinLength = &minLenInt
	}
	if maxLen, ok := rawProp["maxLength"].(float64); ok {
		maxLenInt := int(maxLen)
		prop.MaxLength = &maxLenInt
	}

	// Extract array items schema
	if items, ok := rawProp["items"].(map[string]interface{}); ok {
		prop.Items = convertToProperty(items)
	}

	// Extract nested properties for objects
	if props, ok := rawProp["properties"].(map[string]interface{}); ok {
		prop.Properties = make(map[string]*pkgtools.Property)
		for propName, propValue := range props {
			if propMap, ok := propValue.(map[string]interface{}); ok {
				prop.Properties[propName] = convertToProperty(propMap)
			}
		}
	}

	// Extract required fields for nested objects
	if required, ok := rawProp["required"].([]interface{}); ok {
		for _, req := range required {
			if reqStr, ok := req.(string); ok {
				prop.Required = append(prop.Required, reqStr)
			}
		}
	}

	return prop
}
