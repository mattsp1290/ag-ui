package protobuf

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

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
			name: "text message start",
			event: &events.TextMessageStartEvent{
				BaseEvent: events.BaseEvent{
					EventType: events.TextMessageStart,
					ID:        "msg-123",
					Timestamp: time.Now().Unix(),
					SessionID: "session-456",
				},
				Role:   "assistant",
				Sender: "ai-agent",
			},
			wantErr: false,
		},
		{
			name: "tool call start",
			event: &events.ToolCallStartEvent{
				BaseEvent: events.BaseEvent{
					EventType: events.ToolCallStart,
					ID:        "tool-123",
				},
				ToolCallID: "call-456",
				ToolName:   "calculate",
			},
			wantErr: false,
		},
		{
			name: "state snapshot",
			event: &events.StateSnapshotEvent{
				BaseEvent: events.BaseEvent{
					EventType: events.StateSnapshot,
					ID:        "state-123",
				},
				State: map[string]interface{}{
					"counter": 42,
					"active":  true,
					"name":    "test",
				},
			},
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
	originalEvent := &events.TextMessageContentEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.TextMessageContent,
			ID:        "content-123",
			Timestamp: time.Now().Unix(),
		},
		Content: "Hello, world!",
	}

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

	if decodedContent.Content != originalEvent.Content {
		t.Errorf("Content mismatch: got %q, want %q", decodedContent.Content, originalEvent.Content)
	}

	if decodedContent.ID != originalEvent.ID {
		t.Errorf("ID mismatch: got %q, want %q", decodedContent.ID, originalEvent.ID)
	}
}

func TestProtobufCodec_MultipleEvents(t *testing.T) {
	encoder := NewProtobufEncoder(nil)
	decoder := NewProtobufDecoder(nil)

	// Create multiple events
	events := []events.Event{
		&events.RunStartedEvent{
			BaseEvent: events.BaseEvent{EventType: events.RunStarted},
			RunID:     "run-123",
		},
		&events.StepStartedEvent{
			BaseEvent: events.BaseEvent{EventType: events.StepStarted},
			StepID:    "step-456",
			StepType:  "process",
		},
		&events.StepFinishedEvent{
			BaseEvent: events.BaseEvent{EventType: events.StepFinished},
			StepID:    "step-456",
		},
		&events.RunFinishedEvent{
			BaseEvent: events.BaseEvent{EventType: events.RunFinished},
			RunID:     "run-123",
		},
	}

	// Encode multiple
	data, err := encoder.EncodeMultiple(events)
	if err != nil {
		t.Fatalf("EncodeMultiple failed: %v", err)
	}

	// Decode multiple
	decoded, err := decoder.DecodeMultiple(data)
	if err != nil {
		t.Fatalf("DecodeMultiple failed: %v", err)
	}

	// Verify count
	if len(decoded) != len(events) {
		t.Fatalf("Event count mismatch: got %d, want %d", len(decoded), len(events))
	}

	// Verify each event
	for i, event := range decoded {
		if event.Type() != events[i].Type() {
			t.Errorf("Event %d type mismatch: got %s, want %s", i, event.Type(), events[i].Type())
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
	events := []events.Event{
		&events.CustomEvent{
			BaseEvent: events.BaseEvent{EventType: events.Custom},
			EventName: "user.action",
			Data: map[string]interface{}{
				"action": "click",
				"target": "button",
			},
		},
		&events.RawEvent{
			BaseEvent: events.BaseEvent{EventType: events.Raw},
			Type:      "telemetry",
			Data:      map[string]interface{}{"cpu": 45.2, "memory": 1024},
		},
	}

	for _, event := range events {
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
	if len(decoded) != len(events) {
		t.Fatalf("Event count mismatch: got %d, want %d", len(decoded), len(events))
	}

	for i, event := range decoded {
		if event.Type() != events[i].Type() {
			t.Errorf("Event %d type mismatch: got %s, want %s", i, event.Type(), events[i].Type())
		}
	}
}

func TestProtobufOptions(t *testing.T) {
	t.Run("max size enforcement", func(t *testing.T) {
		encoder := NewProtobufEncoder(&encoding.EncodingOptions{
			MaxSize: 100, // Very small limit
		})

		// Create a large event
		event := &events.StateSnapshotEvent{
			BaseEvent: events.BaseEvent{EventType: events.StateSnapshot},
			State: map[string]interface{}{
				"data": "this is a very long string that will exceed the size limit when encoded to protobuf format",
			},
		}

		_, err := encoder.Encode(event)
		if err == nil {
			t.Error("Expected error for exceeding max size")
		}
	})

	t.Run("output validation", func(t *testing.T) {
		encoder := NewProtobufEncoder(&encoding.EncodingOptions{
			ValidateOutput: true,
		})

		event := &events.ToolCallEndEvent{
			BaseEvent:  events.BaseEvent{EventType: events.ToolCallEnd},
			ToolCallID: "call-123",
			Output:     "result",
		}

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
		
		// Create an invalid event (empty ID)
		event := &events.RunErrorEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.RunError,
				ID:        "", // Invalid: empty ID
			},
			RunID: "run-123",
			Error: "test error",
		}

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
		&events.TextMessageStartEvent{
			BaseEvent: events.BaseEvent{EventType: events.TextMessageStart, ID: "1"},
			Role:      "user",
			Sender:    "test",
		},
		&events.TextMessageContentEvent{
			BaseEvent: events.BaseEvent{EventType: events.TextMessageContent, ID: "2"},
			Content:   "test content",
		},
		&events.TextMessageEndEvent{
			BaseEvent: events.BaseEvent{EventType: events.TextMessageEnd, ID: "3"},
		},
		// Tool events
		&events.ToolCallStartEvent{
			BaseEvent:  events.BaseEvent{EventType: events.ToolCallStart, ID: "4"},
			ToolCallID: "tool-1",
			ToolName:   "calculator",
		},
		&events.ToolCallArgsEvent{
			BaseEvent:  events.BaseEvent{EventType: events.ToolCallArgs, ID: "5"},
			ToolCallID: "tool-1",
			Arguments:  `{"a": 1, "b": 2}`,
			Complete:   true,
		},
		&events.ToolCallEndEvent{
			BaseEvent:  events.BaseEvent{EventType: events.ToolCallEnd, ID: "6"},
			ToolCallID: "tool-1",
			Output:     "3",
		},
		// State events
		&events.StateSnapshotEvent{
			BaseEvent: events.BaseEvent{EventType: events.StateSnapshot, ID: "7"},
			State:     map[string]interface{}{"key": "value"},
		},
		&events.StateDeltaEvent{
			BaseEvent: events.BaseEvent{EventType: events.StateDelta, ID: "8"},
			Patches: []*events.JSONPatch{
				{Op: "add", Path: "/key", Value: "value"},
			},
		},
		&events.MessagesSnapshotEvent{
			BaseEvent: events.BaseEvent{EventType: events.MessagesSnapshot, ID: "9"},
			Messages:  []interface{}{map[string]interface{}{"role": "user", "content": "hi"}},
		},
		// Custom events
		&events.RawEvent{
			BaseEvent: events.BaseEvent{EventType: events.Raw, ID: "10"},
			Type:      "custom",
			Data:      map[string]interface{}{"test": true},
		},
		&events.CustomEvent{
			BaseEvent: events.BaseEvent{EventType: events.Custom, ID: "11"},
			EventName: "test.event",
			Data:      map[string]interface{}{"value": 42},
		},
		// Run events
		&events.RunStartedEvent{
			BaseEvent: events.BaseEvent{EventType: events.RunStarted, ID: "12"},
			RunID:     "run-1",
		},
		&events.RunFinishedEvent{
			BaseEvent: events.BaseEvent{EventType: events.RunFinished, ID: "13"},
			RunID:     "run-1",
		},
		&events.RunErrorEvent{
			BaseEvent: events.BaseEvent{EventType: events.RunError, ID: "14"},
			RunID:     "run-1",
			Error:     "test error",
		},
		&events.StepStartedEvent{
			BaseEvent: events.BaseEvent{EventType: events.StepStarted, ID: "15"},
			StepID:    "step-1",
			StepType:  "process",
		},
		&events.StepFinishedEvent{
			BaseEvent: events.BaseEvent{EventType: events.StepFinished, ID: "16"},
			StepID:    "step-1",
		},
	}

	for _, originalEvent := range testEvents {
		t.Run(originalEvent.Type(), func(t *testing.T) {
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

			// Verify ID
			if decoded.GetID() != originalEvent.GetID() {
				t.Errorf("ID mismatch: got %s, want %s", decoded.GetID(), originalEvent.GetID())
			}
		})
	}
}