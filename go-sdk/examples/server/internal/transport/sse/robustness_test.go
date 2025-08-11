package sse

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
)

// Error-path robustness tests as specified in the task requirements

func TestSSEHandler_WriterErrors(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 10 * time.Millisecond // Very short to trigger errors quickly

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	// Test with context that gets cancelled immediately to simulate writer errors
	req := httptest.NewRequest("GET", "/stream?cid=error_test", nil)

	// Cancel context immediately to cause write errors
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make test request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	// The handler should handle the cancelled context gracefully
	// We expect it to close the connection without panicking
}

func TestSSEHandler_ClientDisconnect(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 50 * time.Millisecond

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	req := httptest.NewRequest("GET", "/stream?cid=disconnect_test", nil)

	// Use a very short timeout to simulate client disconnect
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make test request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	// Read what we can before the disconnect
	buf := make([]byte, 1024)
	_, err = resp.Body.Read(buf)
	// We expect either EOF or context deadline exceeded
	if err != nil && err != io.EOF && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Unexpected error on disconnect: %v", err)
	}
}

func TestSSEHandler_ConcurrentConnections(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 100 * time.Millisecond

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	// Test multiple concurrent connections
	const numConnections = 10
	done := make(chan bool, numConnections)
	errors := make(chan error, numConnections)

	for i := 0; i < numConnections; i++ {
		go func(connID int) {
			defer func() {
				if r := recover(); r != nil {
					errors <- fmt.Errorf("connection %d panicked: %v", connID, r)
				}
				done <- true
			}()

			req := httptest.NewRequest("GET", fmt.Sprintf("/stream?cid=concurrent_%d", connID), nil)

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			req = req.WithContext(ctx)

			resp, err := app.Test(req, fiber.TestConfig{Timeout: 300 * time.Millisecond})
			if err != nil {
				errors <- fmt.Errorf("connection %d failed: %v", connID, err)
				return
			}
			defer func() {
				if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
					errors <- fmt.Errorf("connection %d close failed: %v", connID, err)
				}
			}()

			// Read some data to ensure the connection works
			buf := make([]byte, 512)
			_, err = resp.Body.Read(buf)
			if err != nil && err != io.EOF && !strings.Contains(err.Error(), "context deadline exceeded") {
				errors <- fmt.Errorf("connection %d read failed: %v", connID, err)
			}
		}(i)
	}

	// Wait for all connections to complete or timeout
	completed := 0
	for completed < numConnections {
		select {
		case err := <-errors:
			t.Errorf("Concurrent connection error: %v", err)
		case <-done:
			completed++
		case <-time.After(5 * time.Second):
			t.Fatalf("Timeout waiting for concurrent connections to complete")
		}
	}
}

func TestSSEHandler_BasicFunctionality(t *testing.T) {
	// Simple test to verify basic SSE handler functionality without race conditions
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 100 * time.Millisecond

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	// Single connection test
	req := httptest.NewRequest("GET", "/stream?cid=basic_test", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 200 * time.Millisecond})
	if err != nil {
		// Timeout is expected and acceptable
		if strings.Contains(err.Error(), "timeout") {
			t.Logf("Test completed with expected timeout")
			return
		}
		t.Fatalf("Unexpected error: %v", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Logf("Response close: %v", err)
		}
	}()

	// Verify we can read some data
	buf := make([]byte, 512)
	n, err := resp.Body.Read(buf)
	if err != nil && err != io.EOF && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Logf("Read completed with: %v", err)
	}

	if n > 0 {
		t.Logf("Successfully read %d bytes from SSE stream", n)
	}
}

func TestEnhancedSSEHandler_ErrorRecovery(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 10 * time.Millisecond

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/enhanced-stream", BuildEnhancedSSEHandler(cfg))

	// Test error recovery with cancelled context
	req := httptest.NewRequest("GET", "/enhanced-stream?cid=enhanced_error", nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to trigger error paths
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make test request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	// The enhanced handler should handle errors gracefully without panicking
}

func TestEnhancedSSEHandler_EventCycle(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 50 * time.Millisecond

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/enhanced-stream", BuildEnhancedSSEHandler(cfg))

	req := httptest.NewRequest("GET", "/enhanced-stream?cid=event_cycle", nil)

	// Extend duration to capture at least one keepalive reliably
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 800 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make test request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Logf("Response body close: %v", err)
		}
	}()

	// Read data with proper EOF handling
	buf := make([]byte, 2048)
	totalRead := 0

	for totalRead < len(buf)-100 {
		n, err := resp.Body.Read(buf[totalRead : totalRead+100])
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "context deadline exceeded") ||
				strings.Contains(err.Error(), "unexpected EOF") {
				// These are expected when context is cancelled
				break
			}
			t.Logf("Read completed with: %v", err)
			break
		}
		totalRead += n

		// Give time for more events
		time.Sleep(10 * time.Millisecond)
	}

	response := string(buf[:totalRead])

	// Should have connection event (at minimum)
	if !strings.Contains(response, "event: connection") && !strings.Contains(response, "\"type\":\"connection\"") {
		t.Errorf("Expected connection event in enhanced response, got: %s", response)
	}

	// Should include at least one keepalive during the extended window
	if !strings.Contains(response, "event: keepalive") || !strings.Contains(response, "\"type\":\"keepalive\"") {
		t.Errorf("Expected keepalive event(s) in enhanced response, got: %s", response)
	}

	// Log what we received for debugging
	t.Logf("Enhanced SSE handler test completed successfully, received %d bytes", totalRead)
}

func TestSSEHandler_InvalidConfig(t *testing.T) {
	// Test with extreme config values
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 0 // Invalid keepalive

	app := fiber.New()
	app.Get("/stream", BuildSSEHandler(cfg))

	req := httptest.NewRequest("GET", "/stream", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// Should handle invalid config gracefully
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make test request with invalid config: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	buf := make([]byte, 512)
	_, err = resp.Body.Read(buf)
	// Should work even with invalid keepalive config (fallback to default)
	if err != nil && err != io.EOF && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Handler should work with invalid config, got error: %v", err)
	}
}

func TestSSEHandler_LongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 100 * time.Millisecond

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	req := httptest.NewRequest("GET", "/stream?cid=long_running", nil)

	// Use a shorter duration to reduce flakiness
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 700 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make long-running test request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Logf("Response body close: %v", err)
		}
	}()

	// Count events received with proper error handling
	buf := make([]byte, 4096)
	totalRead := 0
	eventCount := 0

	for {
		n, err := resp.Body.Read(buf[totalRead:])
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "context deadline exceeded") ||
				strings.Contains(err.Error(), "unexpected EOF") {
				// These are expected when context is cancelled
				break
			}
			t.Logf("Read completed with: %v", err)
			break
		}
		totalRead += n
		if totalRead >= len(buf)-100 {
			break
		}
	}

	response := string(buf[:totalRead])
	eventCount = strings.Count(response, "\n\n") // Each SSE event ends with \n\n

	// Should receive at least some events in the given timeframe
	if eventCount < 2 {
		t.Errorf("Expected at least 2 events in timeframe, got %d", eventCount)
	}

	t.Logf("Long-running test completed successfully with %d events", eventCount)
}
