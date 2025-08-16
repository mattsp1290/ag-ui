package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-cli/pkg/client"
	"github.com/mattsp1290/ag-ui/go-cli/pkg/config"
	"github.com/mattsp1290/ag-ui/go-cli/pkg/sse"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// Tool run flags
	toolName    string
	argsJSON    string
	argPairs    []string
	timeout     time.Duration
)

// toolsCmd represents the tools command group
var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Manage and interact with tools",
	Long: `Tools command group for listing, discovering, and running tools directly.
	
The tools group provides commands for:
- Listing available tools from the server
- Running tools directly with provided arguments
- Managing tool execution and results`,
}

// toolsRunCmd represents the run subcommand
var toolsRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a tool directly with provided arguments",
	Long: `Run a tool directly with provided arguments and stream the results.

This command executes a specified tool with the given arguments and streams
the resulting events in real-time. It's optimized for scripting and automation.

Examples:
  # Run a tool with JSON arguments
  ag-ui-cli tools run --tool weather --args-json '{"location": "New York"}'
  
  # Run a tool with key=value arguments
  ag-ui-cli tools run --tool calculator --arg operation=add --arg x=5 --arg y=3
  
  # Run with specific session and output format
  ag-ui-cli tools run --tool search --arg query="golang tutorials" --session abc123 --output json
  
  # Run with timeout
  ag-ui-cli tools run --tool long-running --timeout 30s --args-json '{}'`,
	RunE: runToolCommand,
}

func init() {
	// Add tools command to root
	RootCmd.AddCommand(toolsCmd)
	
	// Add run subcommand to tools
	toolsCmd.AddCommand(toolsRunCmd)
	
	// Add list subcommand to tools
	toolsCmd.AddCommand(toolsListCmd)
	
	// Tool run flags
	toolsRunCmd.Flags().StringVar(&toolName, "tool", "", "Name of the tool to run (required)")
	toolsRunCmd.Flags().StringVar(&argsJSON, "args-json", "", "Tool arguments as a JSON object")
	toolsRunCmd.Flags().StringArrayVar(&argPairs, "arg", []string{}, "Tool arguments as key=value pairs (can be repeated)")
	toolsRunCmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Timeout for tool execution")
	
	// Mark tool name as required
	toolsRunCmd.MarkFlagRequired("tool")
}

// toolsListCmd represents the list subcommand
var toolsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available tools from the server",
	Long:  `List all available tools from the AG-UI server with their descriptions and parameters.`,
	RunE:  listToolsCommand,
}

func runToolCommand(cmd *cobra.Command, args []string) error {
	// Setup logger
	logger := setupLogger()
	
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	// Get or create session
	sessionID := sessionID // Use global flag
	if sessionID == "" {
		// Try to get last session from state
		state, err := config.LoadState()
		if err == nil && state.LastSessionID != "" {
			sessionID = state.LastSessionID
			logger.WithField("session", sessionID).Debug("Using last session")
		} else {
			// Create new session
			httpClient := client.NewHTTPClient(cfg.ServerURL, cfg.APIKey)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			sessionID, err = httpClient.CreateSession(ctx)
			if err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}
			logger.WithField("session", sessionID).Debug("Created new session")
			
			// Save session ID for future use
			if state == nil {
				state = &config.State{}
			}
			state.LastSessionID = sessionID
			if err := config.SaveState(state); err != nil {
				logger.WithError(err).Warn("Failed to save session state")
			}
		}
	}
	
	// Parse and merge arguments
	toolArgs, err := parseToolArguments(argsJSON, argPairs)
	if err != nil {
		return fmt.Errorf("failed to parse tool arguments: %w", err)
	}
	
	// Create HTTP client
	httpClient := client.NewHTTPClient(cfg.ServerURL, cfg.APIKey)
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	// Create tool invocation via InvokeTool method
	toolCallID := fmt.Sprintf("tool_%d", time.Now().UnixNano())
	if err := httpClient.InvokeTool(ctx, sessionID, toolName, string(toolArgs), toolCallID); err != nil {
		return fmt.Errorf("failed to invoke tool: %w", err)
	}
	
	logger.WithFields(logrus.Fields{
		"tool": toolName,
		"id":   toolCallID,
	}).Debug("Tool invocation sent")
	
	// Connect to SSE stream for results
	sseConfig := sse.ClientConfig{
		URL: fmt.Sprintf("%s/tool_based_generative_ui", cfg.ServerURL),
		Headers: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", cfg.APIKey),
		},
		EnableReconnect: false, // Single execution, no reconnect
		Logger:          logger,
	}
	
	sseClient, err := sse.NewClient(sseConfig)
	if err != nil {
		return fmt.Errorf("failed to create SSE client: %w", err)
	}
	defer sseClient.Close()
	
	// Start SSE connection
	if err := sseClient.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to SSE stream: %w", err)
	}
	
	// Get event channel
	eventChan := sseClient.Events()
	
	// Stream and render events
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("tool execution timed out")
			
		case event, ok := <-eventChan:
			if !ok {
				// Stream closed
				return nil
			}
			
			// Handle different event types (Python server format)
			switch event.Type {
			case "RUN_STARTED":
				logger.WithField("data", event.Data).Debug("Run started")
				if outputFormat == "json" {
					fmt.Println(event.Data)
				}
				
			case "RUN_FINISHED":
				logger.Debug("Run finished")
				if outputFormat == "json" {
					fmt.Println(event.Data)
				}
				return nil
				
			case "RUN_ERROR":
				var errData struct {
					Message string `json:"message"`
					Code    string `json:"code,omitempty"`
				}
				if err := json.Unmarshal([]byte(event.Data), &errData); err == nil {
					return fmt.Errorf("tool execution error: %s", errData.Message)
				}
				return fmt.Errorf("tool execution error: %s", event.Data)
				
			case "MESSAGES_SNAPSHOT":
				// Contains the full messages including tool results
				if outputFormat == "json" {
					fmt.Println(event.Data)
				} else {
					// Pretty print messages
					var snapshot struct {
						Messages []json.RawMessage `json:"messages"`
					}
					if err := json.Unmarshal([]byte(event.Data), &snapshot); err == nil {
						for _, msg := range snapshot.Messages {
							var message map[string]interface{}
							if err := json.Unmarshal(msg, &message); err == nil {
								if role, ok := message["role"].(string); ok && role == "tool" {
									// This is a tool result
									if content, ok := message["content"].(string); ok {
										fmt.Printf("\n📦 Tool Result:\n%s\n", content)
									}
								}
							}
						}
					}
				}
				
			case "TOOL_CALL_START":
				var data struct {
					ToolCallID   string `json:"toolCallId"`
					ToolCallName string `json:"toolCallName"`
				}
				if err := json.Unmarshal([]byte(event.Data), &data); err == nil {
					if !quiet {
						fmt.Printf("⚡ Executing tool: %s (ID: %s)\n", data.ToolCallName, data.ToolCallID)
					}
				}
				
			case "TOOL_CALL_ARGS":
				// Stream tool arguments if verbose
				if verbose {
					logger.WithField("data", event.Data).Debug("Tool arguments")
				}
				
			case "TOOL_CALL_END":
				if !quiet {
					fmt.Println("✅ Tool execution completed")
				}
				
			case "TOOL_CALL_RESULT":
				var result struct {
					MessageID  string `json:"messageId"`
					ToolCallID string `json:"toolCallId"`
					Content    string `json:"content"`
				}
				if err := json.Unmarshal([]byte(event.Data), &result); err == nil {
					if outputFormat == "json" {
						fmt.Println(event.Data)
					} else {
						fmt.Printf("\n📦 Tool Result (ID: %s):\n%s\n", result.ToolCallID, result.Content)
					}
				}
				
			case "TEXT_MESSAGE_START", "TEXT_MESSAGE_CONTENT", "TEXT_MESSAGE_END":
				// Handle text streaming if tool generates text output
				if outputFormat == "json" {
					fmt.Println(event.Data)
				}
				
			case "STATE_SNAPSHOT", "STATE_DELTA":
				// Handle state updates
				if outputFormat == "json" {
					fmt.Println(event.Data)
				}
				
			default:
				logger.WithField("type", event.Type).Debug("Unhandled event type")
				if verbose || outputFormat == "json" {
					fmt.Printf("data: %s\n\n", event.Data)
				}
			}
		}
	}
}

func listToolsCommand(cmd *cobra.Command, args []string) error {
	// Setup logger
	logger := setupLogger()
	
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	// Create HTTP client
	httpClient := client.NewHTTPClient(cfg.ServerURL, cfg.APIKey)
	
	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Get tools list from server
	toolsList, err := httpClient.GetTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tools: %w", err)
	}
	
	// Render based on output format
	switch outputFormat {
	case "json":
		// Output as JSON
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(toolsList); err != nil {
			return fmt.Errorf("failed to encode tools: %w", err)
		}
		
	default:
		// Pretty print
		if len(toolsList) == 0 {
			fmt.Println("No tools available")
			return nil
		}
		
		fmt.Printf("Available Tools (%d):\n\n", len(toolsList))
		for _, tool := range toolsList {
			fmt.Printf("  %s\n", tool.Name)
			if tool.Description != "" {
				fmt.Printf("    %s\n", tool.Description)
			}
			if tool.Parameters != nil && len(tool.Parameters.Properties) > 0 {
				fmt.Printf("    Parameters:\n")
				for name, prop := range tool.Parameters.Properties {
					required := ""
					if contains(tool.Parameters.Required, name) {
						required = " (required)"
					}
					fmt.Printf("      - %s: %s%s\n", name, prop.Type, required)
					if prop.Description != "" {
						fmt.Printf("        %s\n", prop.Description)
					}
				}
			}
			fmt.Println()
		}
	}
	
	logger.WithField("count", len(toolsList)).Debug("Listed tools")
	return nil
}

func parseToolArguments(argsJSON string, argPairs []string) ([]byte, error) {
	args := make(map[string]interface{})
	
	// First, parse JSON arguments if provided
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nil, fmt.Errorf("invalid JSON arguments: %w", err)
		}
	}
	
	// Then, parse and merge key=value pairs (these take precedence)
	for _, pair := range argPairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid argument format: %s (expected key=value)", pair)
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Try to parse value as JSON first (for arrays, objects, booleans, numbers)
		var parsedValue interface{}
		if err := json.Unmarshal([]byte(value), &parsedValue); err != nil {
			// If not valid JSON, treat as string
			parsedValue = value
		}
		
		args[key] = parsedValue
	}
	
	// Marshal the merged arguments
	return json.Marshal(args)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}