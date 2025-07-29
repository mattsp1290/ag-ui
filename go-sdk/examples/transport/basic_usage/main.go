// Package main demonstrates basic transport usage with the AG-UI Go SDK.
//
// This example shows:
// - Basic transport configuration and connection
// - Sending and receiving events
// - Error handling and connection management
// - Transport statistics and monitoring
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport"
)

// MockTransport implements the Transport interface for demonstration
type MockTransport struct {
	config          transport.Config
	connected       bool
	eventChan       chan events.Event
	errorChan       chan error
	stats           transport.TransportStats
	connectionState transport.ConnectionState
}

// MockConfig implements the Config interface
type MockConfig struct {
	transportType string
	endpoint      string
	timeout       time.Duration
	headers       map[string]string
	secure        bool
}

func (c *MockConfig) Validate() error {
	if c.endpoint == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}
	if c.timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}

func (c *MockConfig) Clone() transport.Config {
	headers := make(map[string]string)
	for k, v := range c.headers {
		headers[k] = v
	}
	return &MockConfig{
		transportType: c.transportType,
		endpoint:      c.endpoint,
		timeout:       c.timeout,
		headers:       headers,
		secure:        c.secure,
	}
}

func (c *MockConfig) GetType() string        { return c.transportType }
func (c *MockConfig) GetEndpoint() string    { return c.endpoint }
func (c *MockConfig) GetTimeout() time.Duration { return c.timeout }
func (c *MockConfig) GetHeaders() map[string]string { return c.headers }
func (c *MockConfig) IsSecure() bool         { return c.secure }

// NewMockTransport creates a new mock transport for demonstration
func NewMockTransport(config transport.Config) *MockTransport {
	return &MockTransport{
		config:          config,
		connected:       false,
		eventChan:       make(chan events.Event, 100),
		errorChan:       make(chan error, 10),
		connectionState: transport.StateDisconnected,
		stats: transport.TransportStats{
			EventsSent:     0,
			EventsReceived: 0,
			ErrorCount:     0,
		},
	}
}

func (t *MockTransport) Connect(ctx context.Context) error {
	if t.connected {
		return fmt.Errorf("transport already connected")
	}

	t.connectionState = transport.StateConnecting
	
	// Simulate connection delay
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		t.connectionState = transport.StateError
		return ctx.Err()
	}

	t.connected = true
	t.connectionState = transport.StateConnected
	t.stats.ConnectedAt = time.Now()
	
	log.Printf("Connected to %s", t.config.GetEndpoint())
	return nil
}

func (t *MockTransport) Send(ctx context.Context, event transport.TransportEvent) error {
	if !t.connected {
		return fmt.Errorf("transport not connected")
	}

	// Simulate sending delay
	select {
	case <-time.After(10 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}

	t.stats.EventsSent++
	t.stats.LastEventSentAt = time.Now()
	
	log.Printf("Sent event: %s (ID: %s)", event.Type(), event.ID())
	return nil
}

func (t *MockTransport) Receive() <-chan events.Event {
	return t.eventChan
}

func (t *MockTransport) Errors() <-chan error {
	return t.errorChan
}

func (t *MockTransport) Channels() (<-chan events.Event, <-chan error) {
	return t.eventChan, t.errorChan
}

func (t *MockTransport) Close(ctx context.Context) error {
	if !t.connected {
		return nil
	}

	t.connectionState = transport.StateClosing
	
	// Close channels
	close(t.eventChan)
	close(t.errorChan)
	
	t.connected = false
	t.connectionState = transport.StateClosed
	
	log.Printf("Disconnected from %s", t.config.GetEndpoint())
	return nil
}

func (t *MockTransport) IsConnected() bool {
	return t.connected
}

func (t *MockTransport) Config() transport.Config {
	return t.config
}

func (t *MockTransport) Stats() transport.TransportStats {
	t.stats.Uptime = time.Since(t.stats.ConnectedAt)
	return t.stats
}

// MockTransportEvent implements the TransportEvent interface
type MockTransportEvent struct {
	id        string
	eventType string
	timestamp time.Time
	data      map[string]interface{}
}

func (e *MockTransportEvent) ID() string                    { return e.id }
func (e *MockTransportEvent) Type() string                  { return e.eventType }
func (e *MockTransportEvent) Timestamp() time.Time          { return e.timestamp }
func (e *MockTransportEvent) Data() map[string]interface{} { return e.data }

func NewMockTransportEvent(eventType string, data map[string]interface{}) *MockTransportEvent {
	return &MockTransportEvent{
		id:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		eventType: eventType,
		timestamp: time.Now(),
		data:      data,
	}
}

func main() {
	fmt.Println("=== AG-UI Transport Basic Usage Example ===")
	fmt.Println("Demonstrates: Basic transport operations, event handling, and connection management")
	fmt.Println()

	// Create transport configuration
	config := &MockConfig{
		transportType: "websocket",
		endpoint:      "ws://localhost:8080/ws",
		timeout:       30 * time.Second,
		headers: map[string]string{
			"Authorization": "Bearer example-token",
			"User-Agent":    "AG-UI-Go-SDK/1.0",
		},
		secure: true,
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create transport
	transport := NewMockTransport(config)

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down gracefully...")
		cancel()
	}()

	// Connect to transport
	fmt.Printf("Connecting to %s...\n", config.GetEndpoint())
	if err := transport.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		if err := transport.Close(ctx); err != nil {
			log.Printf("Error closing transport: %v", err)
		}
	}()

	// Start error handling goroutine
	go func() {
		for err := range transport.Errors() {
			log.Printf("Transport error: %v", err)
		}
	}()

	// Start event receiving goroutine
	go func() {
		for event := range transport.Receive() {
			fmt.Printf("Received event: %s (Type: %s)\n", 
				"[event received]", 
				fmt.Sprintf("%T", event))
		}
	}()

	// Demonstrate sending various types of events
	fmt.Println("\n=== Sending Events ===")

	// 1. Send a run started event
	runEvent := NewMockTransportEvent("RUN_STARTED", map[string]interface{}{
		"threadId": "thread-123",
		"runId":    "run-456",
	})

	if err := transport.Send(ctx, runEvent); err != nil {
		log.Printf("Failed to send run event: %v", err)
	}

	// 2. Send a text message event
	messageEvent := NewMockTransportEvent("TEXT_MESSAGE_CONTENT", map[string]interface{}{
		"messageId": "msg-789",
		"delta":     "Hello from the transport example!",
	})

	if err := transport.Send(ctx, messageEvent); err != nil {
		log.Printf("Failed to send message event: %v", err)
	}

	// 3. Send a tool call event
	toolEvent := NewMockTransportEvent("TOOL_CALL_START", map[string]interface{}{
		"toolCallId":   "tool-call-123",
		"toolCallName": "calculator",
	})

	if err := transport.Send(ctx, toolEvent); err != nil {
		log.Printf("Failed to send tool event: %v", err)
	}

	// 4. Send a state delta event
	stateEvent := NewMockTransportEvent("STATE_DELTA", map[string]interface{}{
		"delta": []map[string]interface{}{
			{
				"op":    "replace",
				"path":  "/status",
				"value": "processing",
			},
		},
	})

	if err := transport.Send(ctx, stateEvent); err != nil {
		log.Printf("Failed to send state event: %v", err)
	}

	// 5. Send a custom event
	customEvent := NewMockTransportEvent("CUSTOM", map[string]interface{}{
		"name": "user_action",
		"value": map[string]interface{}{
			"action":    "button_click",
			"elementId": "submit-btn",
			"timestamp": time.Now().Unix(),
		},
	})

	if err := transport.Send(ctx, customEvent); err != nil {
		log.Printf("Failed to send custom event: %v", err)
	}

	// Wait a moment for events to be processed
	time.Sleep(500 * time.Millisecond)

	// Show transport statistics
	fmt.Println("\n=== Transport Statistics ===")
	stats := transport.Stats()
	fmt.Printf("Connection uptime: %v\n", stats.Uptime)
	fmt.Printf("Events sent: %d\n", stats.EventsSent)
	fmt.Printf("Events received: %d\n", stats.EventsReceived)
	fmt.Printf("Error count: %d\n", stats.ErrorCount)
	fmt.Printf("Last event sent at: %v\n", stats.LastEventSentAt)

	// Demonstrate connection status checking
	fmt.Println("\n=== Connection Status ===")
	fmt.Printf("Transport connected: %t\n", transport.IsConnected())
	fmt.Printf("Transport type: %s\n", transport.Config().GetType())
	fmt.Printf("Transport endpoint: %s\n", transport.Config().GetEndpoint())
	fmt.Printf("Transport secure: %t\n", transport.Config().IsSecure())

	// Demonstrate configuration cloning
	fmt.Println("\n=== Configuration Management ===")
	clonedConfig := transport.Config().Clone()
	fmt.Printf("Original config endpoint: %s\n", transport.Config().GetEndpoint())
	fmt.Printf("Cloned config endpoint: %s\n", clonedConfig.GetEndpoint())

	// Demonstrate timeout handling
	fmt.Println("\n=== Timeout Handling ===")
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer timeoutCancel()

	slowEvent := NewMockTransportEvent("SLOW_EVENT", map[string]interface{}{
		"delay": "100ms",
	})

	if err := transport.Send(timeoutCtx, slowEvent); err != nil {
		if err == context.DeadlineExceeded {
			fmt.Println("Event send timed out as expected")
		} else {
			log.Printf("Unexpected error: %v", err)
		}
	}

	// Demonstrate error scenarios
	fmt.Println("\n=== Error Handling Scenarios ===")
	
	// Close transport and try to send (should fail)
	transport.Close(ctx)
	
	failEvent := NewMockTransportEvent("FAIL_EVENT", map[string]interface{}{
		"shouldFail": true,
	})

	if err := transport.Send(ctx, failEvent); err != nil {
		fmt.Printf("Expected error when sending to closed transport: %v\n", err)
	}

	// Wait for graceful shutdown or timeout
	select {
	case <-ctx.Done():
		fmt.Println("\nExample completed successfully!")
	case <-time.After(10 * time.Second):
		fmt.Println("\nExample timed out")
	}
}