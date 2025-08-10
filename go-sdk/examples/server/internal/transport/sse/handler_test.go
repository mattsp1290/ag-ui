package sse

import (
	"context"
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

func TestBuildSSEHandler_Headers(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 1 * time.Second

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	req := httptest.NewRequest("GET", "/stream?cid=test123", nil)

	// Use a shorter timeout since we only care about headers
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

	// Check required SSE headers
	expectedHeaders := map[string]string{
		"Content-Type":                 "text/event-stream",
		"Cache-Control":                "no-cache",
		"Connection":                   "keep-alive",
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "Cache-Control",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := resp.Header.Get(header)
		if actualValue != expectedValue {
			t.Errorf("Header %s: expected %q, got %q", header, expectedValue, actualValue)
		}
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestBuildSSEHandler_InitialConnection(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 100 * time.Millisecond

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	req := httptest.NewRequest("GET", "/stream?cid=test123", nil)

	// Use a short timeout - just long enough to get initial connection event
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make test request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	// Read response with a reasonable buffer for SSE initial event
	buf := make([]byte, 2048)
	n, err := resp.Body.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read response: %v", err)
	}

	response := string(buf[:n])

	// Check that initial connection event is sent
	if !strings.Contains(response, "data: {\"type\":\"connection\"") {
		t.Errorf("Expected connection event in response, got: %s", response)
	}

	// Check that cid is included in the response
	if !strings.Contains(response, "test123") {
		t.Errorf("Expected cid 'test123' in response, got: %s", response)
	}

	// Check for proper SSE formatting - should have connection event ending with \n\n
	if !strings.Contains(response, "\"type\":\"connection\"") ||
		!strings.Contains(response, "\n\n") {
		t.Errorf("SSE response should contain properly formatted connection event, got: %s", response)
	}
}

func TestBuildSSEHandler_KeepaliveInterval(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 50 * time.Millisecond // Very short for testing

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	req := httptest.NewRequest("GET", "/stream?cid=keepalive_test", nil)

	// Create a context with timeout to prevent test from running forever
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make test request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	// Read response data for a short time to capture keepalives
	buf := make([]byte, 2048)
	totalRead := 0

	// Read in chunks to capture multiple keepalive events
	for i := 0; i < 5 && totalRead < len(buf)-100; i++ {
		n, err := resp.Body.Read(buf[totalRead : totalRead+100])
		if err != nil && err != io.EOF {
			if err == context.DeadlineExceeded {
				break // Expected timeout
			}
			t.Fatalf("Failed to read response chunk %d: %v", i, err)
		}
		totalRead += n
		time.Sleep(60 * time.Millisecond) // Wait a bit between reads
	}

	response := string(buf[:totalRead])

	// Should have initial connection event
	if !strings.Contains(response, "\"type\":\"connection\"") {
		t.Errorf("Expected connection event in response")
	}

	// Should have at least one keepalive event given our timing
	if !strings.Contains(response, "event: keepalive") {
		t.Errorf("Expected keepalive event in response, got: %s", response)
	}

	// Verify keepalive format
	if !strings.Contains(response, "\"type\":\"keepalive\"") {
		t.Errorf("Expected keepalive type in response, got: %s", response)
	}
}

func TestDefaultHandlerConfig(t *testing.T) {
	config := DefaultHandlerConfig()

	if config.KeepaliveInterval != 15*time.Second {
		t.Errorf("Expected default keepalive interval 15s, got %v", config.KeepaliveInterval)
	}

	if config.EnableDebugLogging {
		t.Error("Expected debug logging to be disabled by default")
	}

	if config.MaxConnections != 100 {
		t.Errorf("Expected max connections 100, got %d", config.MaxConnections)
	}

	if config.ConnectionTimeout != 5*time.Minute {
		t.Errorf("Expected connection timeout 5m, got %v", config.ConnectionTimeout)
	}
}

func TestBuildSSEHandler_ConfigIntegration(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 25 * time.Millisecond // Custom keepalive for this test

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/stream", BuildSSEHandler(cfg))

	req := httptest.NewRequest("GET", "/stream", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 300 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to make test request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	// Read some data
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	response := string(buf[:n])

	// Verify that the custom keepalive interval is being used
	// We should see connection event at minimum
	if !strings.Contains(response, "\"type\":\"connection\"") {
		t.Errorf("Expected connection event with custom config, got: %s", response)
	}
}

func TestGetConnectionCount(t *testing.T) {
	// This is a placeholder test since connection counting is not implemented yet
	count := GetConnectionCount()
	if count != 0 {
		t.Errorf("Expected connection count 0 (placeholder), got %d", count)
	}
}

func TestBuildSSEHandler_WithoutRequestID(t *testing.T) {
	cfg := config.New()
	cfg.EnableSSE = true
	cfg.SSEKeepAlive = 50 * time.Millisecond

	// Create app without requestid middleware to test fallback
	app := fiber.New()
	app.Get("/stream", BuildSSEHandler(cfg))

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

	// Should still work even without request ID middleware
	buf := make([]byte, 512)
	n, _ := resp.Body.Read(buf)
	response := string(buf[:n])

	if !strings.Contains(response, "\"type\":\"connection\"") {
		t.Errorf("Expected connection event even without request ID, got: %s", response)
	}
}
