package protobuf

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

func TestProtobufEncoder_Encode(t *testing.T) {
	encoder := NewProtobufEncoder(nil)

	tests := []struct {
		name    string
		event   events.Event
		wantErr bool
	}{
		{
			name:    "text message start",
			event:   events.NewTextMessageStartEvent("msg-123", events.WithRole("assistant")),
			wantErr: false,
		},
		{
			name:    "tool call start",
			event:   events.NewToolCallStartEvent("call-456", "calculate"),
			wantErr: false,
		},
		{
			name: "state snapshot",
			event: events.NewStateSnapshotEvent(map[string]interface{}{
				"counter": 42,
				"active":  true,
				"name":    "test",
			}),
			wantErr: false,
		},
		{
			name:    "nil event",
			event:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := encoder.Encode(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(data) == 0 {
				t.Error("Encode() returned empty data")
			}
		})
	}
}

func TestProtobufDecoder_Decode(t *testing.T) {
	encoder := NewProtobufEncoder(nil)
	decoder := NewProtobufDecoder(nil)

	// Test round-trip encoding/decoding
	originalEvent := events.NewTextMessageContentEvent("msg-123", "Hello, world!")

	// Encode
	data, err := encoder.Encode(originalEvent)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Decode
	decoded, err := decoder.Decode(data)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	// Verify
	decodedContent, ok := decoded.(*events.TextMessageContentEvent)
	if !ok {
		t.Fatalf("Wrong event type: got %T", decoded)
	}

	if decodedContent.Delta != originalEvent.Delta {
		t.Errorf("Delta mismatch: got %q, want %q", decodedContent.Delta, originalEvent.Delta)
	}

	if decodedContent.MessageID != originalEvent.MessageID {
		t.Errorf("MessageID mismatch: got %q, want %q", decodedContent.MessageID, originalEvent.MessageID)
	}
}

func TestProtobufCodec_MultipleEvents(t *testing.T) {
	encoder := NewProtobufEncoder(nil)
	decoder := NewProtobufDecoder(nil)

	// Create multiple events
	testEvents := []events.Event{
		events.NewRunStartedEvent("thread-123", "run-123"),
		events.NewStepStartedEvent("step-456"),
		events.NewStepFinishedEvent("step-456"),
		events.NewRunFinishedEvent("thread-123", "run-123"),
	}

	// Encode multiple
	data, err := encoder.EncodeMultiple(testEvents)
	if err != nil {
		t.Fatalf("EncodeMultiple failed: %v", err)
	}

	// Decode multiple
	decoded, err := decoder.DecodeMultiple(data)
	if err != nil {
		t.Fatalf("DecodeMultiple failed: %v", err)
	}

	// Verify count
	if len(decoded) != len(testEvents) {
		t.Fatalf("Event count mismatch: got %d, want %d", len(decoded), len(testEvents))
	}

	// Verify each event
	for i, event := range decoded {
		if event.Type() != testEvents[i].Type() {
			t.Errorf("Event %d type mismatch: got %s, want %s", i, event.Type(), testEvents[i].Type())
		}
	}
}

func TestStreamingProtobuf(t *testing.T) {
	encoder := NewStreamingProtobufEncoder(nil)
	decoder := NewStreamingProtobufDecoder(nil)

	// Create a buffer for streaming
	var buf bytes.Buffer

	// Start encoding stream
	if err := encoder.StartStream(&buf); err != nil {
		t.Fatalf("StartStream failed: %v", err)
	}

	// Write events
	eventsToStream := []events.Event{
		events.NewCustomEvent("user.action", events.WithValue(map[string]interface{}{
			"action": "click",
			"target": "button",
		})),
		events.NewRawEvent(map[string]interface{}{"cpu": 45.2, "memory": 1024}, events.WithSource("telemetry")),
	}

	for _, event := range eventsToStream {
		if err := encoder.WriteEvent(event); err != nil {
			t.Fatalf("WriteEvent failed: %v", err)
		}
	}

	if err := encoder.EndStream(); err != nil {
		t.Fatalf("EndStream failed: %v", err)
	}

	// Decode stream
	ctx := context.Background()
	eventChan := make(chan events.Event, 10)
	errChan := make(chan error, 1)

	go func() {
		errChan <- decoder.DecodeStream(ctx, &buf, eventChan)
		close(eventChan)
	}()

	// Collect decoded events
	var decoded []events.Event
	for event := range eventChan {
		decoded = append(decoded, event)
	}

	// Check for errors
	if err := <-errChan; err != nil && err != io.EOF {
		t.Fatalf("DecodeStream error: %v", err)
	}

	// Verify
	if len(decoded) != len(eventsToStream) {
		t.Fatalf("Event count mismatch: got %d, want %d", len(decoded), len(eventsToStream))
	}

	for i, event := range decoded {
		if event.Type() != eventsToStream[i].Type() {
			t.Errorf("Event %d type mismatch: got %s, want %s", i, event.Type(), eventsToStream[i].Type())
		}
	}
}

func TestProtobufOptions(t *testing.T) {
	t.Run("max size enforcement", func(t *testing.T) {
		encoder := NewProtobufEncoder(&encoding.EncodingOptions{
			MaxSize: 100, // Very small limit
		})

		// Create a large event
		event := events.NewStateSnapshotEvent(map[string]interface{}{
			"data": "this is a very long string that will exceed the size limit when encoded to protobuf format",
		})

		_, err := encoder.Encode(event)
		if err == nil {
			t.Error("Expected error for exceeding max size")
		}
	})

	t.Run("output validation", func(t *testing.T) {
		encoder := NewProtobufEncoder(&encoding.EncodingOptions{
			ValidateOutput: true,
		})

		event := events.NewToolCallEndEvent("call-123")

		data, err := encoder.Encode(event)
		if err != nil {
			t.Errorf("Encode with validation failed: %v", err)
		}
		if len(data) == 0 {
			t.Error("Encoded data is empty")
		}
	})

	t.Run("event validation", func(t *testing.T) {
		decoder := NewProtobufDecoder(&encoding.DecodingOptions{
			ValidateEvents: true,
		})

		// Create an encoder to generate test data
		encoder := NewProtobufEncoder(nil)
		
		// Create an invalid event (empty error message)
		event := events.NewRunErrorEvent("") // Invalid: empty error message

		data, _ := encoder.Encode(event)
		
		_, err := decoder.Decode(data)
		if err == nil {
			t.Error("Expected validation error for invalid event")
		}
	})
}

func TestProtobufAllEventTypes(t *testing.T) {
	codec := NewProtobufCodec(nil, nil)

	// Test all event types
	testEvents := []events.Event{
		// Message events
		events.NewTextMessageStartEvent("msg-1", events.WithRole("user")),
		events.NewTextMessageContentEvent("msg-1", "test content"),
		events.NewTextMessageEndEvent("msg-1"),
		// Tool events
		events.NewToolCallStartEvent("tool-1", "calculator"),
		events.NewToolCallArgsEvent("tool-1", `{"a": 1, "b": 2}`),
		events.NewToolCallEndEvent("tool-1"),
		// State events
		events.NewStateSnapshotEvent(map[string]interface{}{"key": "value"}),
		events.NewStateDeltaEvent([]events.JSONPatchOperation{
			{Op: "add", Path: "/key", Value: "value"},
		}),
		events.NewMessagesSnapshotEvent([]events.Message{
			{Role: "user", Content: func(s string) *string { return &s }("hi")},
		}),
		// Custom events
		events.NewRawEvent(map[string]interface{}{"test": true}, events.WithSource("custom")),
		events.NewCustomEvent("test.event", events.WithValue(map[string]interface{}{"value": 42})),
		// Run events
		events.NewRunStartedEvent("thread-1", "run-1"),
		events.NewRunFinishedEvent("thread-1", "run-1"),
		events.NewRunErrorEvent("test error", events.WithRunID("run-1")),
		events.NewStepStartedEvent("step-1"),
		events.NewStepFinishedEvent("step-1"),
	}

	for _, originalEvent := range testEvents {
		t.Run(string(originalEvent.Type()), func(t *testing.T) {
			// Encode
			data, err := codec.Encode(originalEvent)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Decode
			decoded, err := codec.Decode(data)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Verify type
			if decoded.Type() != originalEvent.Type() {
				t.Errorf("Type mismatch: got %s, want %s", decoded.Type(), originalEvent.Type())
			}
		})
	}
}