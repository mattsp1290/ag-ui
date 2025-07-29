package json_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
)

func TestJSONIntegration(t *testing.T) {
	// Create a codec
	codec := json.NewDefaultJSONCodec()

	// Test 1: Single event round-trip
	t.Run("single event round-trip", func(t *testing.T) {
		original := events.NewTextMessageStartEvent("msg123", events.WithRole("assistant"))
		
		// Encode
		data, err := codec.Encode(context.Background(), original)
		if err != nil {
			t.Fatalf("Failed to encode: %v", err)
		}
		
		t.Logf("Encoded JSON: %s", string(data))
		
		// Decode
		decoded, err := codec.Decode(context.Background(), data)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		
		// Verify
		if decoded.Type() != original.Type() {
			t.Errorf("Type mismatch: expected %s, got %s", original.Type(), decoded.Type())
		}
		
		msgEvent, ok := decoded.(*events.TextMessageStartEvent)
		if !ok {
			t.Fatalf("Wrong event type: expected TextMessageStartEvent, got %T", decoded)
		}
		
		if msgEvent.MessageID != "msg123" {
			t.Errorf("MessageID mismatch: expected msg123, got %s", msgEvent.MessageID)
		}
	})

	// Test 2: Streaming round-trip
	t.Run("streaming round-trip", func(t *testing.T) {
		streamCodec := json.NewJSONStreamCodec(nil, nil)
		encoder := streamCodec.GetStreamEncoder()
		decoder := streamCodec.GetStreamDecoder()
		
		var buf bytes.Buffer
		
		// Start encoding stream
		if err := encoder.StartStream(context.Background(), &buf); err != nil {
			t.Fatalf("Failed to start encoder stream: %v", err)
		}
		
		// Write events
		testEvents := []events.Event{
			events.NewTextMessageStartEvent("msg1"),
			events.NewTextMessageContentEvent("msg1", "Hello"),
			events.NewTextMessageEndEvent("msg1"),
		}
		
		for _, event := range testEvents {
			if err := encoder.WriteEvent(context.Background(), event); err != nil {
				t.Fatalf("Failed to write event: %v", err)
			}
		}
		
		if err := encoder.EndStream(context.Background()); err != nil {
			t.Fatalf("Failed to end encoder stream: %v", err)
		}
		
		t.Logf("Streamed NDJSON:\n%s", buf.String())
		
		// Decode stream
		reader := bytes.NewReader(buf.Bytes())
		if err := decoder.StartStream(context.Background(), reader); err != nil {
			t.Fatalf("Failed to start decoder stream: %v", err)
		}
		
		var decodedEvents []events.Event
		for i := 0; i < len(testEvents); i++ {
			event, err := decoder.ReadEvent(context.Background())
			if err != nil {
				t.Fatalf("Failed to read event %d: %v", i, err)
			}
			decodedEvents = append(decodedEvents, event)
		}
		
		if err := decoder.EndStream(context.Background()); err != nil {
			t.Fatalf("Failed to end decoder stream: %v", err)
		}
		
		// Verify
		if len(decodedEvents) != len(testEvents) {
			t.Errorf("Event count mismatch: expected %d, got %d", len(testEvents), len(decodedEvents))
		}
		
		for i := range testEvents {
			if decodedEvents[i].Type() != testEvents[i].Type() {
				t.Errorf("Event %d type mismatch: expected %s, got %s", 
					i, testEvents[i].Type(), decodedEvents[i].Type())
			}
		}
	})

	// Test 3: Cross-SDK compatibility format
	t.Run("cross-sdk compatibility", func(t *testing.T) {
		codec := json.CompatibilityCodec
		
		event := events.NewToolCallStartEvent("tool123", "calculate")
		
		data, err := codec.Encode(context.Background(), event)
		if err != nil {
			t.Fatalf("Failed to encode: %v", err)
		}
		
		t.Logf("Cross-SDK compatible JSON: %s", string(data))
		
		// Verify it's valid JSON and has expected fields
		decoded, err := codec.Decode(context.Background(), data)
		if err != nil {
			t.Errorf("Failed to decode: %v", err)
		}
		
		// Verify type
		if decoded.Type() != events.EventTypeToolCallStart {
			t.Errorf("Expected type to be TOOL_CALL_START, got %s", decoded.Type())
		}
	})
}