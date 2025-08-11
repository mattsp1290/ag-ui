package main

import (
	"context"
	"os"

	"github.com/ag-ui/go-sdk/examples/client/internal/logging"
	"github.com/charmbracelet/fang"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Config holds the global CLI configuration
type Config struct {
	ServerURL string
	APIKey    string
	LogLevel  string
	LogFormat string
	Output    string // json or text
}

var (
	config Config
	logger *logrus.Logger
)

func main() {
	rootCmd := newRootCommand()
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ag-ui-client",
		Short: "AG-UI Client - A tool-based UI client with SSE support",
		Long: `AG-UI Client is a command-line interface for interacting with AG-UI servers.
It provides tool-based UI capabilities with Server-Sent Events (SSE) support
for real-time communication and state synchronization.

Examples:
  # Connect to a server with an API key
  ag-ui-client --server https://api.example.com --api-key your-key
  
  # Set custom log level and output format
  ag-ui-client --log-level debug --output json
  
  # Use environment variables
  export AG_UI_SERVER=https://api.example.com
  export AG_UI_API_KEY=your-key
  ag-ui-client`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Initialize logger with command flags
			logger = logging.Initialize(logging.Options{
				Level:  config.LogLevel,
				Format: config.LogFormat,
			})
			
			// Log initialization info with config
			logger.WithFields(logrus.Fields{
				"component": "cli",
				"version":   "0.1.0",
				"server":    config.ServerURL,
				"output":    config.Output,
			}).Info("AG-UI Client initialized")
			
			// Debug log the full config (without sensitive data)
			logger.WithFields(logrus.Fields{
				"server":     config.ServerURL,
				"log_level":  config.LogLevel,
				"log_format": config.LogFormat,
				"output":     config.Output,
				"has_api_key": config.APIKey != "",
			}).Debug("Configuration loaded")
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	
	// Add persistent flags for global configuration
	cmd.PersistentFlags().StringVar(&config.ServerURL, "server", getEnvOrDefault("AG_UI_SERVER", "http://localhost:8080"), 
		"AG-UI server URL")
	cmd.PersistentFlags().StringVar(&config.APIKey, "api-key", getEnvOrDefault("AG_UI_API_KEY", ""), 
		"API key for authentication")
	cmd.PersistentFlags().StringVar(&config.LogLevel, "log-level", getEnvOrDefault("AG_UI_LOG_LEVEL", "info"), 
		"Set the logging level (debug, info, warn, error)")
	cmd.PersistentFlags().StringVar(&config.LogFormat, "log-format", getEnvOrDefault("AG_UI_LOG_FORMAT", "text"), 
		"Set the logging format (json, text)")
	cmd.PersistentFlags().StringVar(&config.Output, "output", getEnvOrDefault("AG_UI_OUTPUT", "text"), 
		"Set the output format (json, text)")
	
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
	var sessionName string
	var metadata string
	
	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open a new session",
		Long: `Open a new AG-UI session for persistent connection and state management.

Examples:
  # Open a basic session
  ag-ui-client session open
  
  # Open a named session
  ag-ui-client session open --name development
  
  # Open a session with metadata
  ag-ui-client session open --name prod --metadata '{"env":"production"}'`,
		Run: func(cmd *cobra.Command, args []string) {
			logger.WithFields(logrus.Fields{
				"action":   "session_open",
				"name":     sessionName,
				"metadata": metadata,
				"server":   config.ServerURL,
			}).Info("Opening new session (stub)")
			
			if config.Output == "json" {
				logger.WithFields(logrus.Fields{
					"session_id": "stub-session-" + sessionName,
					"status":     "opened",
					"name":       sessionName,
				}).Info("Session opened successfully")
			} else {
				logger.Infof("Session opened: stub-session-%s", sessionName)
			}
		},
	}
	
	cmd.Flags().StringVar(&sessionName, "name", "default", "Session name")
	cmd.Flags().StringVar(&metadata, "metadata", "{}", "Session metadata (JSON)")
	
	return cmd
}

func newSessionCloseCommand() *cobra.Command {
	var sessionID string
	
	cmd := &cobra.Command{
		Use:   "close",
		Short: "Close an existing session",
		Long: `Close an active AG-UI session.

Examples:
  # Close a session by ID
  ag-ui-client session close --session-id abc123
  
  # Force close without cleanup
  ag-ui-client session close --session-id abc123 --force`,
		Run: func(cmd *cobra.Command, args []string) {
			logger.WithFields(logrus.Fields{
				"action":     "session_close",
				"session_id": sessionID,
			}).Info("Closing session (stub)")
			
			if config.Output == "json" {
				logger.WithFields(logrus.Fields{
					"session_id": sessionID,
					"status":     "closed",
				}).Info("Session closed successfully")
			} else {
				logger.Infof("Session closed: %s", sessionID)
			}
		},
	}
	
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID to close")
	cmd.MarkFlagRequired("session-id")
	
	return cmd
}

func newSessionListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active sessions",
		Long: `List all active AG-UI sessions.

Examples:
  # List all sessions
  ag-ui-client session list
  
  # List sessions in JSON format
  ag-ui-client session list --output json`,
		Run: func(cmd *cobra.Command, args []string) {
			logger.WithFields(logrus.Fields{
				"action": "session_list",
			}).Info("Listing sessions (stub)")
			
			if config.Output == "json" {
				logger.WithFields(logrus.Fields{
					"sessions": []map[string]string{
						{"id": "stub-session-1", "name": "default", "status": "active"},
						{"id": "stub-session-2", "name": "development", "status": "active"},
					},
				}).Info("Active sessions")
			} else {
				logger.Info("Active sessions:")
				logger.Info("  stub-session-1 (default) - active")
				logger.Info("  stub-session-2 (development) - active")
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
	var jsonOutput bool
	var filter string
	
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available tools",
		Long: `List all available AG-UI tools with filtering options.

Examples:
  # List all tools
  ag-ui-client tools list
  
  # List tools in JSON format
  ag-ui-client tools list --json
  
  # Filter tools by substring
  ag-ui-client tools list --filter http`,
		Run: func(cmd *cobra.Command, args []string) {
			logger.WithFields(logrus.Fields{
				"action": "tools_list",
				"filter": filter,
				"json":   jsonOutput,
			}).Info("Listing tools (stub)")
			
			tools := []map[string]string{
				{"name": "http_get", "type": "network", "description": "Make HTTP GET requests"},
				{"name": "http_post", "type": "network", "description": "Make HTTP POST requests"},
				{"name": "file_read", "type": "filesystem", "description": "Read file contents"},
				{"name": "file_write", "type": "filesystem", "description": "Write file contents"},
			}
			
			if jsonOutput || config.Output == "json" {
				logger.WithFields(logrus.Fields{
					"tools": tools,
					"count": len(tools),
				}).Info("Available tools")
			} else {
				logger.Info("Available tools:")
				for _, tool := range tools {
					if filter == "" || containsSubstring(tool["name"], filter) {
						logger.Infof("  %s (%s) - %s", tool["name"], tool["type"], tool["description"])
					}
				}
			}
		},
	}
	
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVar(&filter, "filter", "", "Filter tools by substring")
	
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
			
			if config.Output == "json" {
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
			
			logger.WithFields(logrus.Fields{
				"action":     "chat",
				"message":    message,
				"session_id": sessionID,
			}).Info("Sending chat message (stub)")
			
			if jsonOutput || config.Output == "json" {
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
	
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Stream events from AG-UI sessions",
		Long: `Stream Server-Sent Events (SSE) from AG-UI sessions for real-time updates.

Examples:
  # Stream events from a specific session
  ag-ui-client stream --session-id abc123
  
  # Follow a session continuously
  ag-ui-client stream --session-id abc123 --follow
  
  # Stream all events (requires appropriate permissions)
  ag-ui-client stream --all`,
		Run: func(cmd *cobra.Command, args []string) {
			logger.WithFields(logrus.Fields{
				"action":     "stream",
				"session_id": sessionID,
				"follow":     follow,
			}).Info("Starting event stream (stub)")
			
			if config.Output == "json" {
				logger.WithFields(logrus.Fields{
					"event": map[string]string{
						"type":       "connection",
						"session_id": sessionID,
						"status":     "connected",
					},
				}).Info("Stream event")
				logger.WithFields(logrus.Fields{
					"event": map[string]string{
						"type":    "message",
						"content": "Stub SSE event",
					},
				}).Info("Stream event")
			} else {
				logger.Infof("Streaming events for session: %s", sessionID)
				logger.Info("Event: connection established")
				logger.Info("Event: stub SSE message received")
				if !follow {
					logger.Info("Stream ended (use --follow to continue)")
				}
			}
		},
	}
	
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID to stream")
	cmd.Flags().BoolVar(&follow, "follow", false, "Follow stream continuously")
	cmd.MarkFlagRequired("session-id")
	
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
			if config.Output == "json" {
				logger.WithFields(logrus.Fields{
					"server":      config.ServerURL,
					"has_api_key": config.APIKey != "",
					"log_level":   config.LogLevel,
					"log_format":  config.LogFormat,
					"output":      config.Output,
				}).Info("Current configuration")
			} else {
				logger.Info("Current Configuration:")
				logger.Infof("  Server URL: %s", config.ServerURL)
				logger.Infof("  API Key: %s", maskAPIKey(config.APIKey))
				logger.Infof("  Log Level: %s", config.LogLevel)
				logger.Infof("  Log Format: %s", config.LogFormat)
				logger.Infof("  Output Format: %s", config.Output)
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
			paths := []string{
				"$HOME/.config/ag-ui/config.yaml",
				"$HOME/.ag-ui/config.yaml",
				"./ag-ui.yaml",
				"./.ag-ui.yaml",
			}
			
			if config.Output == "json" {
				logger.WithFields(logrus.Fields{
					"config_paths": paths,
					"env_vars": []string{
						"AG_UI_SERVER",
						"AG_UI_API_KEY",
						"AG_UI_LOG_LEVEL",
						"AG_UI_LOG_FORMAT",
						"AG_UI_OUTPUT",
					},
				}).Info("Configuration paths")
			} else {
				logger.Info("Configuration search paths:")
				for _, path := range paths {
					logger.Infof("  - %s", path)
				}
				logger.Info("Environment variables:")
				logger.Info("  - AG_UI_SERVER")
				logger.Info("  - AG_UI_API_KEY")
				logger.Info("  - AG_UI_LOG_LEVEL")
				logger.Info("  - AG_UI_LOG_FORMAT")
				logger.Info("  - AG_UI_OUTPUT")
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

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}