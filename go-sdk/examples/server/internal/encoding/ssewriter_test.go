package encoding

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// TestSSEWriter_WriteEvent tests basic SSE event writing
func TestSSEWriter_WriteEvent(t *testing.T) {
	writer := NewSSEWriter()
	ctx := context.Background()

	testEvent := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
	}
	testEvent.SetData(map[string]interface{}{
		"content": "Hello, SSE!",
	})
	testEvent.SetTimestamp(1234567890)

	tests := []struct {
		name      string
		event     events.Event
		wantError bool
	}{
		{
			name:      "valid event",
			event:     testEvent,
			wantError: false,
		},
		{
			name:      "nil event",
			event:     nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := writer.WriteEvent(ctx, &buf, tt.event)

			if tt.wantError {
				if err == nil {
					t.Errorf("WriteEvent() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("WriteEvent() error = %v", err)
				return
			}

			output := buf.String()

			// Check SSE format
			if !strings.Contains(output, "data: ") {
				t.Errorf("WriteEvent() output missing 'data: ' prefix: %s", output)
			}

			if !strings.HasSuffix(output, "\n\n") {
				t.Errorf("WriteEvent() output missing double newline suffix: %s", output)
			}
		})
	}
}

// TestSSEWriter_WriteEventWithType tests SSE event writing with event types
func TestSSEWriter_WriteEventWithType(t *testing.T) {
	writer := NewSSEWriter()
	ctx := context.Background()

	testEvent := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeCustom,
		},
	}
	testEvent.SetData(map[string]interface{}{
		"message": "Test message",
	})
	testEvent.SetTimestamp(1234567890)

	var buf bytes.Buffer
	err := writer.WriteEventWithType(ctx, &buf, testEvent, "test-event")

	if err != nil {
		t.Fatalf("WriteEventWithType() error = %v", err)
	}

	output := buf.String()

	// Check for event type line
	if !strings.Contains(output, "event: test-event\n") {
		t.Errorf("WriteEventWithType() output missing event type: %s", output)
	}

	// Check for event ID
	if !strings.Contains(output, "id: ") {
		t.Errorf("WriteEventWithType() output missing event ID: %s", output)
	}
}

// TestSSEWriter_WriteErrorEvent tests error event writing
func TestSSEWriter_WriteErrorEvent(t *testing.T) {
	writer := NewSSEWriter()
	ctx := context.Background()

	testError := fmt.Errorf("test error message")
	requestID := "test-request-123"

	var buf bytes.Buffer
	err := writer.WriteErrorEvent(ctx, &buf, testError, requestID)

	if err != nil {
		t.Fatalf("WriteErrorEvent() error = %v", err)
	}

	output := buf.String()

	// Check for error event type
	if !strings.Contains(output, "event: error\n") {
		t.Errorf("WriteErrorEvent() output missing error event type: %s", output)
	}

	// Check that error message is included
	if !strings.Contains(output, "test error message") {
		t.Errorf("WriteErrorEvent() output missing error message: %s", output)
	}

	// Check that request ID is included
	if !strings.Contains(output, requestID) {
		t.Errorf("WriteErrorEvent() output missing request ID: %s", output)
	}
}

// TestSSEWriter_createSSEFrame tests SSE frame creation
func TestSSEWriter_createSSEFrame(t *testing.T) {
	writer := NewSSEWriter()

	testEvent := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeCustom,
		},
	}
	testEvent.SetTimestamp(1234567890)

	jsonData := []byte(`{"test": "data"}`)

	tests := []struct {
		name      string
		eventType string
		wantLines []string
	}{
		{
			name:      "with event type",
			eventType: "test-event",
			wantLines: []string{
				"event: test-event",
				"id: CUSTOM_1234567890",
				"data: {\"test\": \"data\"}",
				"", // empty line at end
			},
		},
		{
			name:      "without event type",
			eventType: "",
			wantLines: []string{
				"id: CUSTOM_1234567890",
				"data: {\"test\": \"data\"}",
				"", // empty line at end
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := writer.createSSEFrame(jsonData, tt.eventType, testEvent)
			if err != nil {
				t.Fatalf("createSSEFrame() error = %v", err)
			}

			lines := strings.Split(frame, "\n")

			for _, wantLine := range tt.wantLines {
				found := false
				for _, line := range lines {
					if line == wantLine {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("createSSEFrame() missing expected line %q in output: %s", wantLine, frame)
				}
			}
		})
	}
}

// Golden test for SSE frame format
func TestSSEWriter_GoldenFrameFormat(t *testing.T) {
	writer := NewSSEWriter()
	ctx := context.Background()

	// Create a predictable event
	testEvent := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
	}
	testEvent.SetData(map[string]interface{}{
		"content":   "Hello, World!",
		"timestamp": int64(1234567890),
	})
	testEvent.SetTimestamp(1234567890)

	var buf bytes.Buffer
	err := writer.WriteEventWithType(ctx, &buf, testEvent, "message")

	if err != nil {
		t.Fatalf("WriteEventWithType() error = %v", err)
	}

	output := buf.String()

	// Golden test - check exact format
	expectedParts := []string{
		"event: message\n",
		"id: TEXT_MESSAGE_CONTENT_1234567890\n",
		"data: ",
		"\n\n", // Double newline at end
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Golden test failed - missing expected part %q in output:\n%s", part, output)
		}
	}

	// Verify the data line contains valid JSON
	lines := strings.Split(output, "\n")
	var dataLine string
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimPrefix(line, "data: ")
			break
		}
	}

	if dataLine == "" {
		t.Error("Golden test failed - no data line found")
	}

	// Verify JSON is valid
	if !isValidJSON([]byte(dataLine)) {
		t.Errorf("Golden test failed - invalid JSON in data line: %s", dataLine)
	}
}

// Test newline escaping in JSON data
func TestSSEWriter_NewlineEscaping(t *testing.T) {
	writer := NewSSEWriter()
	ctx := context.Background()

	// Create event with content that has newlines
	testEvent := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
	}
	testEvent.SetData(map[string]interface{}{
		"content": "Line 1\nLine 2\r\nLine 3",
	})
	testEvent.SetTimestamp(1234567890)

	var buf bytes.Buffer
	err := writer.WriteEvent(ctx, &buf, testEvent)

	if err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	output := buf.String()

	// Extract data line
	lines := strings.Split(output, "\n")
	var dataLine string
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimPrefix(line, "data: ")
			break
		}
	}

	// Newlines should be escaped in the data line
	if strings.Contains(dataLine, "\n") || strings.Contains(dataLine, "\r") {
		t.Errorf("Newlines not properly escaped in data line: %s", dataLine)
	}

	// But escaped newlines should be present
	if !strings.Contains(dataLine, "\\n") {
		t.Errorf("Expected escaped newlines in data line: %s", dataLine)
	}
}

// Benchmark tests
func BenchmarkSSEWriter_WriteEvent(b *testing.B) {
	writer := NewSSEWriter()
	ctx := context.Background()

	testEvent := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
	}
	testEvent.SetData(map[string]interface{}{
		"content": "Benchmark test content",
		"number":  42,
	})
	testEvent.SetTimestamp(1234567890)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		err := writer.WriteEvent(ctx, &buf, testEvent)
		if err != nil {
			b.Fatalf("WriteEvent() error = %v", err)
		}
	}
}

// MockFlushWriter is a test writer that implements the flusher interface
type MockFlushWriter struct {
	bytes.Buffer
	flushCalled bool
}

func (m *MockFlushWriter) Flush() error {
	m.flushCalled = true
	return nil
}

// Test that flush is called when writer supports it
func TestSSEWriter_FlushCalled(t *testing.T) {
	writer := NewSSEWriter()
	ctx := context.Background()

	testEvent := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeCustom,
		},
	}
	testEvent.SetData(map[string]interface{}{"test": "data"})
	testEvent.SetTimestamp(1234567890)

	mockWriter := &MockFlushWriter{}
	err := writer.WriteEvent(ctx, mockWriter, testEvent)

	if err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	if !mockWriter.flushCalled {
		t.Error("Flush() was not called on writer that supports flushing")
	}
}
