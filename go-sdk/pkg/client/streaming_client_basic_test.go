package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSSEClient_BasicFunctionality tests basic SSE client functionality
func TestSSEClient_BasicFunctionality(t *testing.T) {
	// Create a test server that sends SSE events
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Send test events
		events := []string{
			"id: 1\nevent: test\ndata: hello world",
			"id: 2\ndata: simple message",
			"data: multiline\ndata: message",
		}

		for _, event := range events {
			fmt.Fprintf(w, "%s\n\n", event)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	// Create SSE client
	config := SSEClientConfig{
		URL:             server.URL,
		EventBufferSize: 10,
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      time.Second,
	}

	client, err := NewSSEClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSE client: %v", err)
	}
	defer client.Close()

	// Connect to the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Verify connection state
	if client.State() != SSEStateConnected {
		t.Errorf("Expected state %v, got %v", SSEStateConnected, client.State())
	}

	// Collect events
	var receivedEvents []*SSEEvent
	eventChan := client.Events()

	timeout := time.After(2 * time.Second)
	for len(receivedEvents) < 3 {
		select {
		case event := <-eventChan:
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			break
		}
	}

	if len(receivedEvents) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(receivedEvents))
	}

	// Verify first event
	if len(receivedEvents) > 0 {
		event1 := receivedEvents[0]
		if event1.ID != "1" {
			t.Errorf("Expected event ID '1', got '%s'", event1.ID)
		}
		if event1.Event != "test" {
			t.Errorf("Expected event type 'test', got '%s'", event1.Event)
		}
		if event1.Data != "hello world" {
			t.Errorf("Expected data 'hello world', got '%s'", event1.Data)
		}
	}

	// Verify multiline event
	if len(receivedEvents) > 2 {
		event3 := receivedEvents[2]
		if event3.Data != "multiline\nmessage" {
			t.Errorf("Expected multiline data, got '%s'", event3.Data)
		}
	}
}

// TestSSEClient_Configuration tests configuration validation
func TestSSEClient_Configuration(t *testing.T) {
	tests := []struct {
		name        string
		config      SSEClientConfig
		expectError bool
	}{
		{
			name:        "empty URL",
			config:      SSEClientConfig{},
			expectError: true,
		},
		{
			name: "valid HTTP URL",
			config: SSEClientConfig{
				URL: "http://example.com/events",
			},
			expectError: false,
		},
		{
			name: "valid HTTPS URL",
			config: SSEClientConfig{
				URL: "https://example.com/events",
			},
			expectError: false,
		},
		{
			name: "invalid URL scheme",
			config: SSEClientConfig{
				URL: "ftp://example.com",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSSEClient(tt.config)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

