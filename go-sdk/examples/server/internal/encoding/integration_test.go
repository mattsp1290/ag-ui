package encoding

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// TestEncodingIntegration tests the complete encoding integration
func TestEncodingIntegration(t *testing.T) {
	// Create Fiber app with encoding middleware
	app := fiber.New()

	// Add content negotiation middleware
	app.Use(ContentNegotiationMiddleware(ContentNegotiationConfig{
		DefaultContentType: "application/json",
		SupportedTypes:     []string{"application/json", "application/vnd.ag-ui+json"},
		EnableLogging:      true,
	}))

	// Create SSE writer
	sseWriter := NewSSEWriter().WithLogger(slog.Default())

	// Add test SSE endpoint
	app.Get("/test/stream", func(c fiber.Ctx) error {
		// Set SSE headers
		c.Set("Content-Type", "text/event-stream; charset=utf-8")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		// Get negotiated content type
		eventContentType := GetEventContentType(c)

		// Create test event
		testEvent := &CustomEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventTypeTextMessageContent,
			},
		}
		testEvent.SetData(map[string]interface{}{
			"content":      "Integration test message",
			"content_type": eventContentType,
			"timestamp":    time.Now().Unix(),
		})
		testEvent.SetTimestamp(time.Now().UnixMilli())

		// Send stream response
		return c.SendStreamWriter(func(w *bufio.Writer) {
			ctx := context.Background()

			// Write the test event
			if err := sseWriter.WriteEventWithType(ctx, w, testEvent, "test-message"); err != nil {
				t.Errorf("Failed to write SSE event: %v", err)
			}
		})
	})

	tests := []struct {
		name           string
		acceptHeader   string
		expectedSSE    []string
		expectedStatus int
	}{
		{
			name:         "default JSON content type",
			acceptHeader: "",
			expectedSSE: []string{
				"event: test-message",
				"id: TEXT_MESSAGE_CONTENT_",
				"data: ",
				"Integration test message",
			},
			expectedStatus: 200,
		},
		{
			name:         "explicit JSON accept",
			acceptHeader: "application/json",
			expectedSSE: []string{
				"event: test-message",
				"data: ",
				"Integration test message",
			},
			expectedStatus: 200,
		},
		{
			name:         "AG-UI JSON accept",
			acceptHeader: "application/vnd.ag-ui+json",
			expectedSSE: []string{
				"event: test-message",
				"data: ",
				"Integration test message",
			},
			expectedStatus: 200,
		},
		{
			name:         "wildcard accept",
			acceptHeader: "*/*",
			expectedSSE: []string{
				"event: test-message",
				"data: ",
			},
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test/stream", nil)
			if tt.acceptHeader != "" {
				req.Header.Set("Accept", tt.acceptHeader)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Status = %d, expected %d", resp.StatusCode, tt.expectedStatus)
			}

			// Read response body
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			bodyStr := string(body)

			// Check SSE content
			for _, expected := range tt.expectedSSE {
				if !strings.Contains(bodyStr, expected) {
					t.Errorf("Response missing expected content %q. Full response:\n%s", expected, bodyStr)
				}
			}

			// Verify SSE format
			if !strings.HasSuffix(bodyStr, "\n\n") {
				t.Errorf("SSE response should end with double newline. Response:\n%s", bodyStr)
			}
		})
	}
}

// TestContentNegotiationMiddleware tests the middleware functionality
func TestContentNegotiationMiddleware(t *testing.T) {
	app := fiber.New()

	// Add content negotiation middleware
	app.Use(ContentNegotiationMiddleware())

	// Test endpoint that returns negotiated content type
	app.Get("/test/negotiation", func(c fiber.Ctx) error {
		contentType := GetNegotiatedContentType(c)
		eventContentType := GetEventContentType(c)

		return c.JSON(fiber.Map{
			"content_type":       contentType,
			"event_content_type": eventContentType,
		})
	})

	tests := []struct {
		name                     string
		acceptHeader             string
		expectedContentType      string
		expectedEventContentType string
	}{
		{
			name:                     "default negotiation",
			acceptHeader:             "",
			expectedContentType:      "application/json",
			expectedEventContentType: "application/json",
		},
		{
			name:                     "JSON negotiation",
			acceptHeader:             "application/json",
			expectedContentType:      "application/json",
			expectedEventContentType: "application/json",
		},
		{
			name:                     "fallback negotiation",
			acceptHeader:             "application/xml",
			expectedContentType:      "application/json",
			expectedEventContentType: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test/negotiation", nil)
			if tt.acceptHeader != "" {
				req.Header.Set("Accept", tt.acceptHeader)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != 200 {
				t.Errorf("Status = %d, expected 200", resp.StatusCode)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			bodyStr := string(body)
			if !strings.Contains(bodyStr, tt.expectedContentType) {
				t.Errorf("Response should contain content type %q. Response: %s", tt.expectedContentType, bodyStr)
			}
		})
	}
}

// TestErrorHandling tests error handling in the encoding pipeline
func TestErrorHandling(t *testing.T) {
	errorHandler := NewErrorHandler(slog.Default())

	// Test encoding error handling
	encodingErr := CreateEncodingError(
		&CustomEvent{BaseEvent: events.BaseEvent{EventType: events.EventTypeCustom}},
		"test_operation",
		context.DeadlineExceeded,
		"test-request-123",
	)

	// This should not panic
	errorHandler.HandleEncodingError(encodingErr)

	// Test validation error handling
	validationErr := CreateValidationError(
		nil,
		"test_field",
		"test_value",
		"test validation message",
		"test-request-456",
	)

	// This should not panic
	errorHandler.HandleValidationError(validationErr)

	// Test negotiation error handling
	negotiationErr := CreateNegotiationError(
		"application/xml",
		[]string{"application/json"},
		"test-request-789",
	)

	// This should not panic
	errorHandler.HandleNegotiationError(negotiationErr)
}

// TestSSEWriterWithFlushableWriter tests SSE writer with a flushable writer
func TestSSEWriterWithFlushableWriter(t *testing.T) {
	writer := NewSSEWriter()
	ctx := context.Background()

	// Create a custom writer that tracks flush calls
	flushWriter := &TrackingFlushWriter{Buffer: &bytes.Buffer{}}

	testEvent := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeCustom,
		},
	}
	testEvent.SetData(map[string]interface{}{
		"test": "flush functionality",
	})
	testEvent.SetTimestamp(time.Now().UnixMilli())

	err := writer.WriteEvent(ctx, flushWriter, testEvent)
	if err != nil {
		t.Fatalf("WriteEvent() failed: %v", err)
	}

	if !flushWriter.FlushCalled {
		t.Error("Flush() should have been called on flushable writer")
	}

	if flushWriter.Buffer.Len() == 0 {
		t.Error("No data was written to the buffer")
	}
}

// TrackingFlushWriter is a test writer that tracks flush calls
type TrackingFlushWriter struct {
	Buffer      *bytes.Buffer
	FlushCalled bool
}

func (w *TrackingFlushWriter) Write(p []byte) (n int, err error) {
	return w.Buffer.Write(p)
}

func (w *TrackingFlushWriter) Flush() error {
	w.FlushCalled = true
	return nil
}

// TestEventValidation tests the SafeEventValidation function
func TestEventValidation(t *testing.T) {
	tests := []struct {
		name      string
		event     events.Event
		wantError bool
	}{
		{
			name:      "nil event",
			event:     nil,
			wantError: true,
		},
		{
			name: "valid event",
			event: func() events.Event {
				e := &CustomEvent{
					BaseEvent: events.BaseEvent{
						EventType: events.EventTypeCustom,
					},
				}
				e.SetData(map[string]interface{}{"test": "data"})
				return e
			}(),
			wantError: false,
		},
		{
			name: "event with empty type",
			event: func() events.Event {
				e := &CustomEvent{
					BaseEvent: events.BaseEvent{
						EventType: "", // Empty type
					},
				}
				e.SetData(map[string]interface{}{"test": "data"})
				return e
			}(),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SafeEventValidation(tt.event, "test-request")

			if tt.wantError && err == nil {
				t.Error("SafeEventValidation() expected error, got nil")
			}

			if !tt.wantError && err != nil {
				t.Errorf("SafeEventValidation() unexpected error: %v", err)
			}
		})
	}
}

// BenchmarkIntegration benchmarks the complete encoding pipeline
func BenchmarkIntegrationSSEWrite(b *testing.B) {
	writer := NewSSEWriter()
	ctx := context.Background()

	testEvent := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
	}
	testEvent.SetData(map[string]interface{}{
		"content":   "Benchmark integration test",
		"timestamp": time.Now().Unix(),
		"sequence":  0,
	})
	testEvent.SetTimestamp(time.Now().UnixMilli())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		err := writer.WriteEventWithType(ctx, &buf, testEvent, "benchmark")
		if err != nil {
			b.Fatalf("WriteEventWithType() failed: %v", err)
		}
	}
}
