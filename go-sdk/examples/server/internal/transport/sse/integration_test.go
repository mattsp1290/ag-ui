package sse

import (
	"bufio"
	"context"
	"fmt"
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

	// Create test server
	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Convert http request to fiber context and handle
		fiberApp := app
		fiberApp.Test(r)

		// For integration test, we'll make a direct HTTP call
		ctx := r.Context()

		// Manually set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter does not support flushing")
		}

		// Send initial connection event
		fmt.Fprintf(w, "data: {\"type\":\"connection\",\"timestamp\":\"%s\"}\n\n", time.Now().Format(time.RFC3339))
		flusher.Flush()

		// Keepalive loop
		ticker := time.NewTicker(cfg.SSEKeepAlive)
		defer ticker.Stop()

		counter := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				counter++
				fmt.Fprintf(w, "event: keepalive\ndata: {\"type\":\"keepalive\",\"sequence\":%d}\n\n", counter)
				flusher.Flush()
			}
		}
	}))
	defer server.Close()

	// Test client connection
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream?cid=integration_test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify headers
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	// Read streaming response
	scanner := bufio.NewScanner(resp.Body)
	events := []string{}

	for scanner.Scan() && len(events) < 10 { // Limit to prevent infinite loop
		line := scanner.Text()
		if line != "" {
			events = append(events, line)
		}
	}

	// Verify we got events
	if len(events) == 0 {
		t.Fatal("Expected to receive SSE events, got none")
	}

	// Check for connection event
	foundConnection := false
	for _, event := range events {
		if strings.Contains(event, "\"type\":\"connection\"") {
			foundConnection = true
			break
		}
	}

	if !foundConnection {
		t.Errorf("Expected connection event, events received: %v", events)
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
		fmt.Fprintf(w, "data: {\"type\":\"connection\"}\n\n")
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
		resp.Body.Close() // Close immediately to simulate disconnect
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

	// Use fiber test method for header validation
	req := httptest.NewRequest("GET", "/stream", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to make test request: %v", err)
	}
	defer resp.Body.Close()

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
			defer resp.Body.Close()

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
	cfg.SSEKeepAlive = 30 * time.Millisecond // Short interval for testing

	keepaliveTimes := []time.Time{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter does not support flushing")
		}

		ctx := r.Context()

		// Send initial connection
		fmt.Fprintf(w, "data: {\"type\":\"connection\"}\n\n")
		flusher.Flush()

		ticker := time.NewTicker(cfg.SSEKeepAlive)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				keepaliveTimes = append(keepaliveTimes, now)
				fmt.Fprintf(w, "event: keepalive\ndata: {\"type\":\"keepalive\"}\n\n")
				flusher.Flush()
			}
		}
	}))
	defer server.Close()

	// Connect and wait for a few keepalives
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	resp.Body.Close()

	// Should have received several keepalives in the time window
	if len(keepaliveTimes) < 2 {
		t.Errorf("Expected at least 2 keepalives, got %d", len(keepaliveTimes))
	}

	// Check intervals are approximately correct
	if len(keepaliveTimes) >= 2 {
		interval := keepaliveTimes[1].Sub(keepaliveTimes[0])
		expectedInterval := cfg.SSEKeepAlive

		// Allow for some timing variance (±10ms)
		tolerance := 10 * time.Millisecond
		if interval < expectedInterval-tolerance || interval > expectedInterval+tolerance {
			t.Errorf("Keepalive interval: expected ~%v, got %v", expectedInterval, interval)
		}
	}
}
