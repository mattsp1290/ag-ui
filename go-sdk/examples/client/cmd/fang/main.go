package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ag-ui/go-sdk/examples/client/internal/config"
	"github.com/ag-ui/go-sdk/examples/client/internal/logging"
	"github.com/ag-ui/go-sdk/examples/client/internal/session"
	"github.com/ag-ui/go-sdk/examples/client/internal/sse"
	"github.com/ag-ui/go-sdk/examples/client/internal/tools"
	"github.com/ag-ui/go-sdk/examples/client/internal/ui"
	"github.com/charmbracelet/fang"
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
				"server":     finalCfg.ServerURL,
				"log_level":  finalCfg.LogLevel,
				"log_format": finalCfg.LogFormat,
				"output":     finalCfg.Output,
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
	cmd.AddCommand(newStreamCommand())
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
	cmd := &cobra.Command{
		Use:   "describe [tool-name]",
		Short: "Describe a specific tool",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			toolName := args[0]
			logger.WithFields(logrus.Fields{
				"action": "tools_describe",
				"tool":   toolName,
			}).Info("Describing tool (stub)")
			
			if configManager.GetConfig().Output == "json" {
				logger.WithFields(logrus.Fields{
					"name":        toolName,
					"type":        "network",
					"description": "Make HTTP requests",
					"parameters": map[string]string{
						"url":     "string",
						"headers": "map[string]string",
					},
				}).Info("Tool details")
			} else {
				logger.Infof("Tool: %s", toolName)
				logger.Info("Type: network")
				logger.Info("Description: Make HTTP requests")
				logger.Info("Parameters:")
				logger.Info("  - url (string)")
				logger.Info("  - headers (map[string]string)")
			}
		},
	}
	
	return cmd
}

func newChatCommand() *cobra.Command {
	var message string
	var sessionID string
	var jsonOutput bool
	
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Send chat messages to AG-UI",
		Long: `Send chat messages to AG-UI server and receive responses.

Examples:
  # Send a simple message
  ag-ui-client chat --message "Hello, AG-UI!"
  
  # Send a message in a specific session
  ag-ui-client chat --message "Continue our work" --session-id abc123
  
  # Get response in JSON format
  ag-ui-client chat --message "What tools are available?" --json
  
  # Use message as argument
  ag-ui-client chat "Quick question about the API"`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Use argument as message if provided
			if len(args) > 0 && message == "" {
				message = args[0]
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
					cfg := configManager.GetConfig()
					sessionID = cfg.LastSessionID
					if sessionID != "" {
						logger.WithField("threadId", sessionID).Debug("Using last session ID from config")
					}
				}
			}
			
			logger.WithFields(logrus.Fields{
				"action":     "chat",
				"message":    message,
				"session_id": sessionID,
			}).Info("Sending chat message (stub)")
			
			if jsonOutput || configManager.GetConfig().Output == "json" {
				logger.WithFields(logrus.Fields{
					"request": map[string]string{
						"message":    message,
						"session_id": sessionID,
					},
					"response": map[string]string{
						"content": "This is a stub response to: " + message,
						"type":    "text",
					},
				}).Info("Chat response")
			} else {
				logger.Info("Chat response:")
				logger.Infof("  > %s", message)
				logger.Infof("  < This is a stub response to: %s", message)
			}
		},
	}
	
	cmd.Flags().StringVar(&message, "message", "", "Chat message to send")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID for context")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	
	return cmd
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
			
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			
			go func() {
				<-sigChan
				logger.Info("Received interrupt signal, closing stream...")
				cancel()
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
			
			renderer := ui.NewRenderer(ui.RendererConfig{
				OutputMode: rendererMode,
				NoColor:    noColor,
				Quiet:      quiet,
				Writer:     os.Stdout,
			})
			
			// Parse retry configuration
			retryDelayDuration, err := time.ParseDuration(retryDelay)
			if err != nil {
				logger.WithError(err).Error("Invalid retry delay duration")
				os.Exit(1)
			}
			
			timeoutDuration, err := time.ParseDuration(timeout)
			if err != nil {
				logger.WithError(err).Error("Invalid timeout duration")
				os.Exit(1)
			}
			
			// Helper function to parse retry policy
			parseRetryPolicy := func(s string) tools.RetryPolicy {
				switch strings.ToLower(s) {
				case "retry":
					return tools.RetryPolicyRetry
				case "prompt":
					return tools.RetryPolicyPrompt
				default:
					return tools.RetryPolicyAbort
				}
			}
			
			// Create retry configuration
			retryConfig := tools.RetryConfig{
				OnError:           parseRetryPolicy(onError),
				MaxRetries:        maxRetries,
				InitialDelay:      retryDelayDuration,
				MaxDelay:          30 * time.Second,
				BackoffMultiplier: 2.0,
				JitterFactor:      retryJitter,
				Timeout:           timeoutDuration,
				PerAttemptTimeout: 30 * time.Second,
				ResetAfter:        60 * time.Second,
				Logger:            logger,
			}
			
			// Create stream integration for tool handling
			streamIntegration := tools.NewStreamIntegration(retryConfig, renderer)
			
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
					
					eventData, _ := json.Marshal(event["data"])
					
					// Handle the event through the stream integration
					if err := streamIntegration.HandleSSEEvent(eventType, eventData); err != nil {
						logger.WithError(err).WithField("event", eventType).Warn("Failed to handle event")
					}
					
					// Check if we should exit due to tool errors
					if streamIntegration.ShouldExit() {
						streamIntegration.CleanExit()
						return
					}
					
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
					streamIntegration.CleanExit()
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
					"key":    key,
					"value":  displayValue,
					"saved":  true,
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
					"key":    key,
					"unset":  true,
					"saved":  true,
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
		"threadId": "tools-discovery-" + fmt.Sprintf("%d", time.Now().Unix()),
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
			Name:        "http_get",
			Description: "Make HTTP GET requests to external APIs",
			Tags:        []string{"network", "http", "api"},
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
			Name:        "http_post",
			Description: "Make HTTP POST requests with JSON payloads",
			Tags:        []string{"network", "http", "api"},
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
			Name:        "file_read",
			Description: "Read contents from a file",
			Tags:        []string{"filesystem", "io"},
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
			Name:        "file_write",
			Description: "Write contents to a file",
			Tags:        []string{"filesystem", "io"},
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
			Name:        "data_transform",
			Description: "Transform data between different formats",
			Tags:        []string{"data", "transformation"},
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

