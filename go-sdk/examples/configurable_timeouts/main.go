package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/state"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport/websocket"
)

func main() {
	fmt.Println("=== Configurable Timeouts Example ===")
	fmt.Println()

	// Demonstrate state management timeouts
	fmt.Println("State Management Timeouts:")
	fmt.Printf("  Shutdown timeout: %v\n", state.GetDefaultShutdownTimeout())
	fmt.Printf("  Update timeout: %v\n", state.GetDefaultUpdateTimeout())
	fmt.Printf("  Retry delay: %v\n", state.GetDefaultRetryDelay())
	fmt.Printf("  Batch timeout: %v\n", state.GetDefaultBatchTimeout())
	fmt.Println()

	// Demonstrate WebSocket timeouts
	fmt.Println("WebSocket Connection Timeouts:")
	wsConfig := websocket.DefaultConnectionConfig()
	fmt.Printf("  Dial timeout: %v\n", wsConfig.DialTimeout)
	fmt.Printf("  Handshake timeout: %v\n", wsConfig.HandshakeTimeout)
	fmt.Printf("  Read timeout: %v\n", wsConfig.ReadTimeout)
	fmt.Printf("  Write timeout: %v\n", wsConfig.WriteTimeout)
	fmt.Printf("  Ping period: %v\n", wsConfig.PingPeriod)
	fmt.Printf("  Pong timeout: %v\n", wsConfig.PongWait)
	fmt.Println()

	// Demonstrate Tools timeouts
	fmt.Println("Tools Timeouts:")
	// We can't access internal timeconfig directly, but we can see the effects
	// through the configured tools and connections
	fmt.Printf("  I/O timeout: %v\n", state.GetDefaultIOTimeout())
	fmt.Printf("  Validation timeout: %v\n", state.GetDefaultValidationTimeout())
	fmt.Println()

	// Example: Using configurable timeouts in practice
	fmt.Println("=== Practical Usage Examples ===")
	fmt.Println()

	// Example 1: HTTP request with configurable timeout
	fmt.Println("1. HTTP Request Example:")
	registry := tools.NewRegistry()
	err := tools.RegisterBuiltinTools(registry)
	if err != nil {
		log.Printf("Failed to register builtin tools: %v", err)
	} else {
		httpTool, err := registry.Get("http_get")
		if err == nil {
			fmt.Printf("   HTTP GET tool timeout: %v\n", httpTool.Capabilities.Timeout)
		}
	}

	// Example 2: WebSocket connection with configurable timeouts
	fmt.Println("2. WebSocket Connection Example:")
	config := websocket.DefaultConnectionConfig()
	config.URL = "wss://echo.websocket.org"
	
	// In production: these would be longer timeouts (30s, 10s, etc.)
	// In test mode: these are much shorter (500ms, 100ms, etc.)
	fmt.Printf("   Connection will use dial timeout: %v\n", config.DialTimeout)
	fmt.Printf("   Connection will use handshake timeout: %v\n", config.HandshakeTimeout)

	// Example 3: Context with configurable timeout
	fmt.Println("3. Context Timeout Example:")
	shutdownTimeout := state.GetDefaultShutdownTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	
	startTime := time.Now()
	select {
	case <-ctx.Done():
		fmt.Printf("   Context timed out after: %v (configured: %v)\n", 
			time.Since(startTime), shutdownTimeout)
	case <-time.After(10 * time.Millisecond):
		fmt.Printf("   Context still active after 10ms (timeout: %v)\n", 
			shutdownTimeout)
	}
	fmt.Println()

	fmt.Println("=== Configuration Details ===")
	// In test mode, these will be much shorter
	// In production mode, these will be the full timeouts
	fmt.Printf("Sample timeouts from current config:\n")
	fmt.Printf("  - Shutdown: %v\n", state.GetDefaultShutdownTimeout())
	fmt.Printf("  - Update: %v\n", state.GetDefaultUpdateTimeout()) 
	fmt.Printf("  - WebSocket Dial: %v\n", wsConfig.DialTimeout)
	fmt.Printf("  - WebSocket Read: %v\n", wsConfig.ReadTimeout)
	fmt.Printf("  - I/O Operations: %v\n", state.GetDefaultIOTimeout())
	
	fmt.Println()
	fmt.Println("Note: When running under 'go test', these timeouts are automatically")
	fmt.Println("reduced to speed up test execution. In production, they use full values.")
}