package logstreaming

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

func RunLogStreamingExample() error {
	// Create registry and register the log streaming tool
	registry := tools.NewRegistry()
	logStreamTool := CreateLogStreamingTool()

	if err := registry.Register(logStreamTool); err != nil {
		return fmt.Errorf("failed to register log streaming tool: %w", err)
	}

	// Create execution engine
	engine := tools.NewExecutionEngine(registry)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Shutdown(ctx); err != nil {
			log.Printf("Engine shutdown error: %v", err)
		}
	}()

	// Ensure temp directory and create test log file
	if err := os.MkdirAll("./temp", 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create a test log file
	testLogPath := "./temp/test.log"
	testContent := `2024-01-01T10:00:00 INFO Starting application
2024-01-01T10:00:01 INFO Loading configuration
2024-01-01T10:00:02 ERROR Failed to connect to database
2024-01-01T10:00:03 INFO Retrying database connection
2024-01-01T10:00:04 INFO Database connected successfully
2024-01-01T10:00:05 INFO Application ready
2024-01-01T10:00:06 DEBUG Processing request #1
2024-01-01T10:00:07 ERROR Invalid request format
2024-01-01T10:00:08 INFO Request processed successfully
2024-01-01T10:00:09 INFO Application running normally`

	if err := os.WriteFile(testLogPath, []byte(testContent), 0644); err != nil {
		return fmt.Errorf("failed to create test log file: %w", err)
	}

	ctx := context.Background()

	fmt.Println("=== Log Streaming Tool Example ===")
	fmt.Println("Demonstrates: Streaming interface, real-time processing, and resource management")
	fmt.Println()

	// Example 1: Tail mode
	fmt.Println("1. Streaming last 5 lines...")
	streamCh, err := engine.ExecuteStream(ctx, "log_streaming", map[string]interface{}{
		"path":  testLogPath,
		"mode":  "tail",
		"lines": 5,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		consumeLogStream(streamCh, 10) // Consume up to 10 chunks
	}
	fmt.Println()

	// Example 2: Filter for errors
	fmt.Println("2. Streaming ERROR lines only...")
	streamCh, err = engine.ExecuteStream(ctx, "log_streaming", map[string]interface{}{
		"path":   testLogPath,
		"mode":   "full",
		"filter": "ERROR",
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		consumeLogStream(streamCh, 10)
	}
	fmt.Println()

	// Example 3: Head mode
	fmt.Println("3. Streaming first 3 lines...")
	streamCh, err = engine.ExecuteStream(ctx, "log_streaming", map[string]interface{}{
		"path":  testLogPath,
		"mode":  "head",
		"lines": 3,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		consumeLogStream(streamCh, 10)
	}
	fmt.Println()

	// Example 4: Follow mode (with timeout)
	fmt.Println("4. Following log file for 3 seconds...")
	followCtx, followCancel := context.WithTimeout(ctx, 3*time.Second)
	defer followCancel()

	streamCh, err = engine.ExecuteStream(followCtx, "log_streaming", map[string]interface{}{
		"path":   testLogPath,
		"mode":   "tail",
		"lines":  2,
		"follow": true,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		// Simulate adding new content to the log file in a separate goroutine with context cancellation
		go func() {
			select {
			case <-time.After(1 * time.Second):
				newContent := "\n2024-01-01T10:00:10 INFO New log entry while following"
				file, err := os.OpenFile(testLogPath, os.O_APPEND|os.O_WRONLY, 0644)
				if err == nil {
					file.WriteString(newContent)
					file.Close()
				}
			case <-followCtx.Done():
				// Context cancelled, exit immediately
				return
			}
		}()

		consumeLogStream(streamCh, 20) // May receive historical + new entries
	}
	fmt.Println()

	// Cleanup
	fmt.Println("5. Cleaning up test file...")
	if err := os.Remove(testLogPath); err != nil {
		fmt.Printf("  Error cleaning up: %v\n", err)
	} else {
		fmt.Println("  Test file removed successfully")
	}
	
	return nil
}

// consumeLogStream consumes chunks from a stream channel for demonstration
func consumeLogStream(streamCh <-chan *tools.ToolStreamChunk, maxChunks int) {
	count := 0
	for chunk := range streamCh {
		if count >= maxChunks {
			break
		}

		switch chunk.Type {
		case "metadata":
			fmt.Printf("  [%d] Metadata: %v\n", chunk.Index, chunk.Data)
		case "data":
			data := chunk.Data.(map[string]interface{})
			content := data["content"].(string)
			fmt.Printf("  [%d] Log: %s\n", chunk.Index, content)
		case "error":
			data := chunk.Data.(map[string]interface{})
			fmt.Printf("  [%d] Error: %v\n", chunk.Index, data["error"])
		case "complete":
			fmt.Printf("  [%d] Completed: %v\n", chunk.Index, chunk.Data)
		default:
			fmt.Printf("  [%d] %s: %v\n", chunk.Index, chunk.Type, chunk.Data)
		}

		count++
	}
}

// CreateLogStreamingTool creates a streaming log reader tool
func CreateLogStreamingTool() *tools.Tool {
	return &tools.Tool{
		ID:          "log_streaming",
		Name:        "Log Streaming",
		Description: "A streaming log reader with filtering and following capabilities", 
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"path": {
					Type:        "string",
					Description: "Path to the log file",
				},
				"mode": {
					Type:        "string",
					Description: "Reading mode",
					Enum:        []interface{}{"head", "tail", "full"},
					Default:     "tail",
				},
				"lines": {
					Type:        "number",
					Description: "Number of lines to read",
					Default:     10,
				},
				"filter": {
					Type:        "string",
					Description: "Filter pattern for log lines",
				},
				"follow": {
					Type:        "boolean",
					Description: "Continue to watch for new lines",
					Default:     false,
				},
			},
			Required: []string{"path"},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  true,
			Async:      true,
			Cancelable: true,
			Timeout:    60 * time.Second,
		},
		Metadata: &tools.ToolMetadata{
			Author:   "Streaming Team",
			License:  "MIT",
			Tags:     []string{"streaming", "logs", "files", "monitoring"},
		},
		// Note: In a real implementation, you would provide a proper StreamExecutor
		// For now, this is a placeholder to allow the example to compile
		Executor: &MockLogStreamingExecutor{},
	}
}

// MockLogStreamingExecutor is a placeholder for the actual streaming executor
type MockLogStreamingExecutor struct{}

func (e *MockLogStreamingExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	return &tools.ToolExecutionResult{
		Success: true,
		Data:    "Log streaming executed successfully",
	}, nil
}