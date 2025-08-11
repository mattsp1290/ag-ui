package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"strings"

	"github.com/mattsp1290/ag-ui/go-cli/pkg/sse"
	"github.com/mattsp1290/ag-ui/go-cli/pkg/tools"
	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command-line flags
	fs := flag.NewFlagSet("ag-ui-cli", flag.ExitOnError)
	config := sse.RegisterFlags(fs)
	
	// Add help flag
	help := fs.Bool("help", false, "Show help message")
	
	// Add tool-related flags
	toolArgs := fs.String("tool-args", "", "JSON arguments for tool calls (non-interactive mode)")
	interactive := fs.Bool("interactive", true, "Enable interactive mode for tool arguments")
	
	// Parse flags
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}
	
	// Show help if requested
	if *help {
		fmt.Println("AG-UI CLI - SSE Client with Health Monitoring")
		fmt.Println()
		sse.PrintUsage()
		os.Exit(0)
	}
	
	// Load environment variables
	config.LoadFromEnv()
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		fmt.Println("\nUse --help for usage information")
		os.Exit(1)
	}
	
	// Convert to client config
	clientConfig, err := config.ToClientConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client config: %v\n", err)
		os.Exit(1)
	}
	
	// Create SSE client
	client, err := sse.NewClient(clientConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create SSE client: %v\n", err)
		os.Exit(1)
	}
	
	// Create tool registry and handler
	toolRegistry := tools.NewToolRegistry()
	toolHandler := tools.NewToolCallHandler(toolRegistry, tools.ToolHandlerConfig{
		ServerURL:   config.URL,
		Endpoint:    "/tool_based_generative_ui",
		Headers:     config.Headers,
		Interactive: *interactive,
		ToolArgs:    *toolArgs,
	}, clientConfig.Logger)
	
	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		sig := <-sigChan
		fmt.Printf("\n\nReceived signal: %v\n", sig)
		fmt.Println("Shutting down gracefully...")
		cancel()
	}()
	
	// Connect to SSE endpoint
	fmt.Printf("Connecting to SSE endpoint: %s\n", config.URL)
	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Connected successfully! Processing events...")
	if config.MetricsMode != sse.MetricsModeOff {
		fmt.Printf("Metrics reporting enabled (%s mode) with %v interval\n", 
			config.MetricsMode, config.MetricsInterval)
	}
	fmt.Println("\nPress Ctrl+C to stop")
	fmt.Println(strings.Repeat("-", 60))
	
	// Process events
	go processEvents(ctx, client, clientConfig.Logger, toolRegistry, toolHandler)
	
	// Wait for shutdown
	<-ctx.Done()
	
	// Clean shutdown
	fmt.Println("\nClosing SSE connection...")
	if err := client.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Error closing client: %v\n", err)
	}
	
	fmt.Println("Shutdown complete")
}

func processEvents(ctx context.Context, client *sse.Client, logger *logrus.Logger, registry *tools.ToolRegistry, handler *tools.ToolCallHandler) {
	eventCount := 0
	var currentThreadID, currentRunID string
	
	for event := range client.Events() {
		eventCount++
		
		// Log the event
		fields := logrus.Fields{
			"event_num":  eventCount,
			"event_id":   event.ID,
			"event_type": event.Type,
			"timestamp":  event.Timestamp,
		}
		
		// Truncate data for logging if too long
		data := event.Data
		if len(data) > 200 {
			data = data[:200] + "..."
		}
		fields["data_preview"] = data
		
		logger.WithFields(fields).Info("Event received")
		
		// Process specific event types
		switch event.Type {
		case "RUN_STARTED":
			// Parse run started event to get thread and run IDs
			var runData map[string]interface{}
			if err := json.Unmarshal([]byte(event.Data), &runData); err == nil {
				if threadID, ok := runData["threadId"].(string); ok {
					currentThreadID = threadID
				}
				if runID, ok := runData["runId"].(string); ok {
					currentRunID = runID
				}
			}
			
		case "MESSAGES_SNAPSHOT":
			// Parse messages snapshot to detect tool calls
			var snapshot map[string]interface{}
			if err := json.Unmarshal([]byte(event.Data), &snapshot); err == nil {
				if messages, ok := snapshot["messages"].([]interface{}); ok {
					// Process tool calls if any
					if err := handler.ProcessMessagesSnapshot(ctx, messages, currentThreadID, currentRunID); err != nil {
						logger.WithError(err).Error("Failed to process tool calls")
					}
				}
			}
			
		case "TOOL_CALL_START":
			// Log tool call start
			logger.WithField("data", event.Data).Info("Tool call started")
			
		case "TOOL_CALL_END":
			// Log tool call end
			logger.WithField("data", event.Data).Info("Tool call ended")
			
		case "error":
			logger.WithField("data", event.Data).Error("Error event received")
			
		case "warning":
			logger.WithField("data", event.Data).Warn("Warning event received")
			
		case "heartbeat":
			// Heartbeat events - could update a health check
			logger.Debug("Heartbeat received")
			
		default:
			// Regular event processing
			// In a real application, you would process the event data here
		}
	}
	
	logger.Info("Event channel closed")
}