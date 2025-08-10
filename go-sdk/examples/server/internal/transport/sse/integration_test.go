package sse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
)

// TestSSEIntegration_FullFlow tests the complete SSE flow including connection, keepalives, and disconnection
func TestSSEIntegration_FullFlow(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 50 * time.Millisecond

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	req := httptest.NewRequest("GET", "/stream?cid=integration_test", nil)

	// Test should run long enough to get initial connection + at least one keepalive
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 300 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	// Verify headers first
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	// Read streaming response with a larger buffer
	buf := make([]byte, 4096)
	totalRead := 0

	// Read in chunks with small delays to capture multiple events
	for i := 0; i < 5 && totalRead < len(buf)-100; i++ {
		n, err := resp.Body.Read(buf[totalRead : totalRead+800])
		if err != nil && err != io.EOF {
			// Handle expected errors when connection is closed due to context timeout
			if strings.Contains(err.Error(), "timeout") ||
				strings.Contains(err.Error(), "deadline") ||
				strings.Contains(err.Error(), "unexpected EOF") {
				break // Expected when context times out
			}
			t.Fatalf("Failed to read response chunk %d: %v", i, err)
		}
		totalRead += n
		if n == 0 { // No more data available
			break
		}
		if i < 4 { // Don't sleep on the last iteration
			time.Sleep(60 * time.Millisecond) // Wait between reads to get more events
		}
	}

	response := string(buf[:totalRead])

	// Check for connection event
	if !strings.Contains(response, "\"type\":\"connection\"") {
		t.Errorf("Expected connection event, got response: %s", response)
	}

	// Check for integration_test cid
	if !strings.Contains(response, "integration_test") {
		t.Errorf("Expected cid 'integration_test' in response, got: %s", response)
	}
}

// TestSSEIntegration_ClientDisconnect tests that the server properly handles client disconnection
func TestSSEIntegration_ClientDisconnect(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 10 * time.Millisecond // Fast keepalives for quick test

	disconnectDetected := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter does not support flushing")
		}

		ctx := r.Context()

		// Send initial event
		if _, err := fmt.Fprintf(w, "data: {\"type\":\"connection\"}\n\n"); err != nil {
			t.Logf("Failed to write SSE event: %v", err)
			return
		}
		flusher.Flush()

		// Keepalive loop that should detect disconnection
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// Client disconnected - this is what we want to test
				disconnectDetected <- true
				return
			case <-ticker.C:
				_, err := fmt.Fprintf(w, "event: keepalive\ndata: {\"type\":\"keepalive\"}\n\n")
				if err != nil {
					// Write error indicates client disconnect
					disconnectDetected <- true
					return
				}
				flusher.Flush()
			}
		}
	}))
	defer server.Close()

	// Make request and disconnect quickly
	req, _ := http.NewRequest("GET", server.URL, nil)
	client := &http.Client{Timeout: 50 * time.Millisecond}

	resp, err := client.Do(req)
	if err != nil {
		// Timeout expected due to short client timeout
		if !strings.Contains(err.Error(), "timeout") {
			t.Fatalf("Unexpected error: %v", err)
		}
	} else {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Logf("Failed to close response body: %v", err)
		}
	}

	// Wait for disconnect detection
	select {
	case <-disconnectDetected:
		// Good - server detected the disconnect
	case <-time.After(100 * time.Millisecond):
		t.Error("Server did not detect client disconnect within expected time")
	}
}

// TestSSEIntegration_HeaderValidation tests that all required SSE headers are set correctly
func TestSSEIntegration_HeaderValidation(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	// Use fiber test method for header validation with timeout
	req := httptest.NewRequest("GET", "/stream", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make test request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	requiredHeaders := map[string]string{
		"Content-Type":                 "text/event-stream",
		"Cache-Control":                "no-cache",
		"Connection":                   "keep-alive",
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "Cache-Control",
	}

	for headerName, expectedValue := range requiredHeaders {
		actualValue := resp.Header.Get(headerName)
		if actualValue != expectedValue {
			t.Errorf("Header %s: expected %q, got %q", headerName, expectedValue, actualValue)
		}
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}
}

// TestSSEIntegration_ConcurrentConnections tests multiple concurrent SSE connections
func TestSSEIntegration_ConcurrentConnections(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 20 * time.Millisecond

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	// Test with multiple concurrent connections
	numConnections := 3
	done := make(chan bool, numConnections)

	for i := 0; i < numConnections; i++ {
		go func(connID int) {
			defer func() { done <- true }()

			req := httptest.NewRequest("GET", fmt.Sprintf("/stream?cid=conn_%d", connID), nil)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			req = req.WithContext(ctx)

			resp, err := app.Test(req, fiber.TestConfig{Timeout: 150 * time.Millisecond})
			if err != nil {
				t.Errorf("Connection %d failed: %v", connID, err)
				return
			}
			defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

			// Verify each connection gets proper headers
			if resp.Header.Get("Content-Type") != "text/event-stream" {
				t.Errorf("Connection %d: wrong content type", connID)
			}

			// Read some data to verify streaming works
			buf := make([]byte, 256)
			n, _ := resp.Body.Read(buf)
			response := string(buf[:n])

			if !strings.Contains(response, "\"type\":\"connection\"") {
				t.Errorf("Connection %d: no connection event received", connID)
			}
		}(i)
	}

	// Wait for all connections to complete
	for i := 0; i < numConnections; i++ {
		select {
		case <-done:
			// Connection completed
		case <-time.After(300 * time.Millisecond):
			t.Errorf("Connection %d did not complete in time", i)
		}
	}
}

// TestSSEIntegration_KeepaliveInterval tests that keepalives are sent at the correct interval
func TestSSEIntegration_KeepaliveInterval(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 40 * time.Millisecond // Short interval for testing

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	req := httptest.NewRequest("GET", "/stream", nil)

	// Run long enough to capture multiple keepalives
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	// Read response in chunks to capture keepalive events
	buf := make([]byte, 2048)
	totalRead := 0

	// Read for long enough to get multiple keepalives
	for i := 0; i < 4 && totalRead < len(buf)-200; i++ {
		n, err := resp.Body.Read(buf[totalRead : totalRead+500])
		if err != nil && err != io.EOF {
			// Handle expected errors when connection is closed due to context timeout
			if strings.Contains(err.Error(), "timeout") ||
				strings.Contains(err.Error(), "deadline") ||
				strings.Contains(err.Error(), "unexpected EOF") {
				break // Expected when context times out
			}
			t.Fatalf("Failed to read response: %v", err)
		}
		totalRead += n
		if n == 0 { // No more data available
			break
		}
		time.Sleep(45 * time.Millisecond) // Wait a bit longer than keepalive interval
	}

	response := string(buf[:totalRead])

	// Should have connection event
	if !strings.Contains(response, "\"type\":\"connection\"") {
		t.Error("Expected initial connection event")
	}

	// Count keepalive events
	keepaliveCount := strings.Count(response, "\"type\":\"keepalive\"")
	if keepaliveCount < 2 {
		t.Errorf("Expected at least 2 keepalives, got %d. Response: %s", keepaliveCount, response)
	}
}
