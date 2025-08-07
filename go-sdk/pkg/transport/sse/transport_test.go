package sse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport/common"
)

// TestSSETransport_NewSSETransport tests the creation of SSE transport
func TestSSETransport_NewSSETransport(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name:        "nil config uses defaults",
			config:      nil,
			expectError: false,
		},
		{
			name: "valid config",
			config: &Config{
				BaseURL:     "http://localhost:8080",
				BufferSize:  100,
				ReadTimeout: 30 * time.Second,
			},
			expectError: false,
		},
		{
			name: "missing base URL",
			config: &Config{
				BaseURL: "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := NewSSETransport(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if transport == nil {
				t.Errorf("Expected transport to be created")
				return
			}

			// Test default values
			if tt.config == nil {
				if transport.baseURL != "http://localhost:8080" {
					t.Errorf("Expected default baseURL, got %s", transport.baseURL)
				}
			}
		})
	}
}

// TestSSETransport_Send tests sending events
func TestSSETransport_Send(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events" {
			t.Errorf("Expected path /events, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL

	transport, err := NewSSETransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close(context.Background())

	// Create a test event
	event := events.NewRunStartedEvent("thread-123", "run-123")

	// Use a more generous timeout for this integration test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = transport.Send(ctx, event)
	if err != nil {
		t.Errorf("Failed to send event: %v", err)
	}
}

// TestSSETransport_Send_ValidationError tests sending invalid events
func TestSSETransport_Send_ValidationError(t *testing.T) {
	config := &Config{
		BaseURL: "http://localhost:8080",
	}

	transport, err := NewSSETransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close(context.Background())

	// Create test helper for consistent timeout handling
	helper := common.NewTestHelper(t)
	ctx, cancel := helper.TestContext()
	defer cancel()

	// Test nil event
	err = transport.Send(ctx, nil)
	if err == nil {
		t.Errorf("Expected error for nil event")
	}

	// Test invalid event (empty required field)
	event := events.NewRunStartedEvent("", "run-123") // Empty thread ID
	err = transport.Send(ctx, event)
	if err == nil {
		t.Errorf("Expected validation error for invalid event")
	}
}

// TestSSETransport_ParseEvents tests parsing various event types
func TestSSETransport_ParseEvents(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	transport, err := NewSSETransport(nil)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close(context.Background())

	tests := []struct {
		name      string
		eventType string
		data      map[string]interface{}
		expectErr bool
	}{
		{
			name:      "run started event",
			eventType: "RUN_STARTED",
			data: map[string]interface{}{
				"threadId":  "thread-123",
				"runId":     "run-123",
				"timestamp": float64(1234567890),
			},
			expectErr: false,
		},
		{
			name:      "text message start event",
			eventType: "TEXT_MESSAGE_START",
			data: map[string]interface{}{
				"messageId": "msg-123",
				"role":      "user",
				"timestamp": float64(1234567890),
			},
			expectErr: false,
		},
		{
			name:      "text message content event",
			eventType: "TEXT_MESSAGE_CONTENT",
			data: map[string]interface{}{
				"messageId": "msg-123",
				"delta":     "Hello world",
				"timestamp": float64(1234567890),
			},
			expectErr: false,
		},
		{
			name:      "tool call start event",
			eventType: "TOOL_CALL_START",
			data: map[string]interface{}{
				"toolCallId":      "tool-123",
				"toolCallName":    "calculator",
				"parentMessageId": "msg-123",
				"timestamp":       float64(1234567890),
			},
			expectErr: false,
		},
		{
			name:      "state snapshot event",
			eventType: "STATE_SNAPSHOT",
			data: map[string]interface{}{
				"snapshot": map[string]interface{}{
					"key1": "value1",
					"key2": 42,
				},
				"timestamp": float64(1234567890),
			},
			expectErr: false,
		},
		{
			name:      "custom event",
			eventType: "CUSTOM",
			data: map[string]interface{}{
				"name":      "my-custom-event",
				"value":     "custom-value",
				"timestamp": float64(1234567890),
			},
			expectErr: false,
		},
		{
			name:      "unknown event type",
			eventType: "UNKNOWN_TYPE",
			data: map[string]interface{}{
				"field": "value",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := transport.createEventFromData(tt.eventType, tt.data)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if event == nil {
				t.Errorf("Expected event to be created")
				return
			}

			// Verify the event type
			expectedType := events.EventType(tt.eventType)
			if event.Type() != expectedType {
				t.Errorf("Expected event type %s, got %s", expectedType, event.Type())
			}

			// Verify timestamp was set
			if event.Timestamp() == nil {
				t.Errorf("Expected timestamp to be set")
			}
		})
	}
}

// TestSSETransport_Receive tests SSE event reception
func TestSSETransport_Receive(t *testing.T) {
	// Create a test server that sends SSE events
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events/stream" {
			t.Errorf("Expected path /events/stream, got %s", r.URL.Path)
		}

		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Expected Accept text/event-stream, got %s", r.Header.Get("Accept"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("Streaming unsupported")
			return
		}

		// Send a test event
		event := map[string]interface{}{
			"type":      "RUN_STARTED",
			"threadId":  "thread-123",
			"runId":     "run-123",
			"timestamp": time.Now().UnixMilli(),
		}

		eventData, _ := json.Marshal(event)

		w.Write([]byte("event: RUN_STARTED\n"))
		w.Write([]byte("data: " + string(eventData) + "\n"))
		w.Write([]byte("\n"))
		flusher.Flush()

		// Send another event
		w.Write([]byte("data: " + string(eventData) + "\n"))
		w.Write([]byte("\n"))
		flusher.Flush()
	}))
	defer server.Close()

	config := &Config{
		BaseURL: server.URL,
	}

	transport, err := NewSSETransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close(context.Background())

	// Create test helper for consistent timeout handling
	helper := common.NewTestHelper(t)
	ctx, cancel := helper.ReceiveContext()
	defer cancel()

	eventChan, err := transport.Receive(ctx)
	if err != nil {
		t.Fatalf("Failed to start receiving: %v", err)
	}

	// Use test helper for event waiting with standardized timeouts
	eventCount := 0
	maxEvents := 2

	for eventCount < maxEvents {
		// Convert to interface{} channel for helper compatibility
		interfaceChan := make(chan interface{}, 1)
		go func() {
			select {
			case event := <-eventChan:
				interfaceChan <- event
			case <-ctx.Done():
				close(interfaceChan)
			}
		}()

		event := helper.WaitForEvent(interfaceChan)
		if event == nil {
			t.Errorf("Received nil event")
			continue
		}

		ssEvent, ok := event.(events.Event)
		if !ok {
			t.Errorf("Received non-event object")
			continue
		}

		if ssEvent.Type() != events.EventTypeRunStarted {
			t.Errorf("Expected RUN_STARTED event, got %s", ssEvent.Type())
		}

		eventCount++
	}

	if eventCount < maxEvents {
		t.Logf("Received %d of %d expected events", eventCount, maxEvents)
	}
}

// TestSSETransport_ConnectionManagement tests connection lifecycle
func TestSSETransport_ConnectionManagement(t *testing.T) {
	transport, err := NewSSETransport(nil)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	// Test initial state
	if transport.GetConnectionStatus() != ConnectionDisconnected {
		t.Errorf("Expected initial state to be disconnected")
	}

	// Test closing
	err = transport.Close(context.Background())
	if err != nil {
		t.Errorf("Failed to close transport: %v", err)
	}

	if transport.GetConnectionStatus() != ConnectionClosed {
		t.Errorf("Expected state to be closed after Close()")
	}

	// Test operations on closed transport
	helper := common.NewTestHelper(t)
	ctx, cancel := helper.TestContext()
	defer cancel()
	err = transport.Send(ctx, events.NewRunStartedEvent("thread-123", "run-123"))
	if err == nil {
		t.Errorf("Expected error when sending on closed transport")
	}

	_, err = transport.Receive(ctx)
	if err == nil {
		t.Errorf("Expected error when receiving on closed transport")
	}
}

// TestSSETransport_SetHeader tests custom header functionality
func TestSSETransport_SetHeader(t *testing.T) {
	transport, err := NewSSETransport(nil)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close(context.Background())

	// Set custom headers
	transport.SetHeader("Authorization", "Bearer token123")
	transport.SetHeader("X-Custom-Header", "custom-value")

	// Verify headers are set
	if transport.headers["Authorization"] != "Bearer token123" {
		t.Errorf("Authorization header not set correctly")
	}

	if transport.headers["X-Custom-Header"] != "custom-value" {
		t.Errorf("Custom header not set correctly")
	}

	// Verify default SSE headers are preserved
	if transport.headers["Accept"] != "text/event-stream" {
		t.Errorf("Default Accept header should be preserved")
	}
}

// BenchmarkSSETransport_CreateEventFromData benchmarks event parsing
func BenchmarkSSETransport_CreateEventFromData(b *testing.B) {
	transport, err := NewSSETransport(nil)
	if err != nil {
		b.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close(context.Background())

	data := map[string]interface{}{
		"threadId":  "thread-123",
		"runId":     "run-123",
		"timestamp": float64(1234567890),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := transport.createEventFromData("RUN_STARTED", data)
		if err != nil {
			b.Fatalf("Failed to create event: %v", err)
		}
	}
}

// TestSSETransport_ErrorHandling tests error scenarios
func TestSSETransport_ErrorHandling(t *testing.T) {
	// Test server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "error") {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL + "/error"

	transport, err := NewSSETransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close(context.Background())

	// Create test helper for consistent timeout handling
	helper := common.NewTestHelper(t)
	ctx, cancel := helper.TestContext()
	defer cancel()
	event := events.NewRunStartedEvent("thread-123", "run-123")

	err = transport.Send(ctx, event)
	if err == nil {
		t.Errorf("Expected error from server")
	}
}
