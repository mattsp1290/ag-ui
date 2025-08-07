package json

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
)

func TestJSONEncoder_Encode(t *testing.T) {
	encoder := NewJSONEncoder(nil)

	t.Run("encode text message start event", func(t *testing.T) {
		event := events.NewTextMessageStartEvent("msg123", events.WithRole("assistant"))
		data, err := encoder.Encode(context.Background(), event)
		if err != nil {
			t.Fatalf("failed to encode event: %v", err)
		}

		// Verify JSON structure
		var decoded map[string]interface{}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("failed to unmarshal encoded data: %v", err)
		}

		if decoded["type"] != "TEXT_MESSAGE_START" {
			t.Errorf("expected type TEXT_MESSAGE_START, got %v", decoded["type"])
		}
		if decoded["messageId"] != "msg123" {
			t.Errorf("expected messageId msg123, got %v", decoded["messageId"])
		}
		if decoded["role"] != "assistant" {
			t.Errorf("expected role assistant, got %v", decoded["role"])
		}
	})

	t.Run("encode with validation", func(t *testing.T) {
		encoder := NewJSONEncoder(&encoding.EncodingOptions{
			ValidateOutput: true,
		})

		// Create an invalid event (missing messageId)
		event := &events.TextMessageStartEvent{
			BaseEvent: events.NewBaseEvent(events.EventTypeTextMessageStart),
			MessageID: "", // Invalid: empty messageId
		}

		_, err := encoder.Encode(context.Background(), event)
		if err == nil {
			t.Error("expected validation error, got nil")
		}
	})

	t.Run("encode with size limit", func(t *testing.T) {
		encoder := NewJSONEncoder(&encoding.EncodingOptions{
			MaxSize: 50, // Very small size limit
		})

		event := events.NewTextMessageStartEvent("msg123")
		_, err := encoder.Encode(context.Background(), event)
		if err == nil {
			t.Error("expected size limit error, got nil")
		}
	})

	t.Run("encode nil event", func(t *testing.T) {
		_, err := encoder.Encode(context.Background(), nil)
		if err == nil {
			t.Error("expected error for nil event, got nil")
		}
	})
}

func TestJSONEncoder_EncodeMultiple(t *testing.T) {
	encoder := NewJSONEncoder(nil)

	testEvents := []events.Event{
		events.NewTextMessageStartEvent("msg1"),
		events.NewTextMessageContentEvent("msg1", "Hello"),
		events.NewTextMessageEndEvent("msg1"),
	}

	data, err := encoder.EncodeMultiple(context.Background(), testEvents)
	if err != nil {
		t.Fatalf("failed to encode multiple events: %v", err)
	}

	// Verify it's a valid JSON array
	var decoded []map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal encoded data: %v", err)
	}

	if len(decoded) != 3 {
		t.Errorf("expected 3 events, got %d", len(decoded))
	}
}

func TestJSONDecoder_Decode(t *testing.T) {
	decoder := NewJSONDecoder(nil)

	t.Run("decode text message start event", func(t *testing.T) {
		jsonData := `{"type":"TEXT_MESSAGE_START","messageId":"msg123","role":"assistant","timestamp":1234567890}`

		event, err := decoder.Decode(context.Background(), []byte(jsonData))
		if err != nil {
			t.Fatalf("failed to decode event: %v", err)
		}

		msgEvent, ok := event.(*events.TextMessageStartEvent)
		if !ok {
			t.Fatalf("expected TextMessageStartEvent, got %T", event)
		}

		if msgEvent.MessageID != "msg123" {
			t.Errorf("expected messageId msg123, got %s", msgEvent.MessageID)
		}
		if msgEvent.Role == nil || *msgEvent.Role != "assistant" {
			t.Errorf("expected role assistant, got %v", msgEvent.Role)
		}
	})

	t.Run("decode with validation", func(t *testing.T) {
		decoder := NewJSONDecoder(&encoding.DecodingOptions{
			ValidateEvents: true,
		})

		// Invalid event (missing messageId)
		jsonData := `{"type":"TEXT_MESSAGE_START","timestamp":1234567890}`

		_, err := decoder.Decode(context.Background(), []byte(jsonData))
		if err == nil {
			t.Error("expected validation error, got nil")
		}
	})

	t.Run("decode unknown event type", func(t *testing.T) {
		jsonData := `{"type":"UNKNOWN_EVENT_TYPE"}`

		_, err := decoder.Decode(context.Background(), []byte(jsonData))
		if err == nil {
			t.Error("expected error for unknown event type, got nil")
		}
	})

	t.Run("decode empty data", func(t *testing.T) {
		_, err := decoder.Decode(context.Background(), []byte{})
		if err == nil {
			t.Error("expected error for empty data, got nil")
		}
	})

	t.Run("decode invalid JSON", func(t *testing.T) {
		_, err := decoder.Decode(context.Background(), []byte("not json"))
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})
}

func TestJSONDecoder_DecodeMultiple(t *testing.T) {
	decoder := NewJSONDecoder(nil)

	jsonData := `[
		{"type":"TEXT_MESSAGE_START","messageId":"msg1","timestamp":1234567890},
		{"type":"TEXT_MESSAGE_CONTENT","messageId":"msg1","delta":"Hello","timestamp":1234567891},
		{"type":"TEXT_MESSAGE_END","messageId":"msg1","timestamp":1234567892}
	]`

	decodedEvents, err := decoder.DecodeMultiple(context.Background(), []byte(jsonData))
	if err != nil {
		t.Fatalf("failed to decode multiple events: %v", err)
	}

	if len(decodedEvents) != 3 {
		t.Errorf("expected 3 events, got %d", len(decodedEvents))
	}

	// Verify event types
	if decodedEvents[0].Type() != events.EventTypeTextMessageStart {
		t.Errorf("expected first event to be TEXT_MESSAGE_START, got %s", decodedEvents[0].Type())
	}
	if decodedEvents[1].Type() != events.EventTypeTextMessageContent {
		t.Errorf("expected second event to be TEXT_MESSAGE_CONTENT, got %s", decodedEvents[1].Type())
	}
	if decodedEvents[2].Type() != events.EventTypeTextMessageEnd {
		t.Errorf("expected third event to be TEXT_MESSAGE_END, got %s", decodedEvents[2].Type())
	}
}

func TestStreamingJSONEncoder(t *testing.T) {
	t.Run("basic streaming", func(t *testing.T) {
		encoder := NewStreamingJSONEncoder(nil)
		var buf bytes.Buffer

		if err := encoder.StartStream(context.Background(), &buf); err != nil {
			t.Fatalf("failed to start stream: %v", err)
		}

		testEvents := []events.Event{
			events.NewTextMessageStartEvent("msg1"),
			events.NewTextMessageContentEvent("msg1", "Hello"),
			events.NewTextMessageEndEvent("msg1"),
		}

		for _, event := range testEvents {
			if err := encoder.WriteEvent(context.Background(), event); err != nil {
				t.Fatalf("failed to write event: %v", err)
			}
		}

		if err := encoder.EndStream(context.Background()); err != nil {
			t.Fatalf("failed to end stream: %v", err)
		}

		// Verify NDJSON format (each event on its own line)
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 3 {
			t.Errorf("expected 3 lines, got %d", len(lines))
		}

		// Verify each line is valid JSON
		for i, line := range lines {
			var decoded map[string]interface{}
			if err := json.Unmarshal([]byte(line), &decoded); err != nil {
				t.Errorf("line %d is not valid JSON: %v", i, err)
			}
		}
	})

	t.Run("stream not started error", func(t *testing.T) {
		encoder := NewStreamingJSONEncoder(nil)
		event := events.NewTextMessageStartEvent("msg1")

		err := encoder.WriteEvent(context.Background(), event)
		if err == nil {
			t.Error("expected error when writing to unstarted stream")
		}
	})

	t.Run("concurrent streaming", func(t *testing.T) {
		encoder := NewStreamingJSONEncoder(nil)
		ctx := context.Background()
		eventChan := make(chan events.Event)
		var buf bytes.Buffer

		// Start streaming in a goroutine
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := encoder.EncodeStream(ctx, eventChan, &buf); err != nil {
				t.Errorf("EncodeStream failed: %v", err)
			}
		}()

		// Send events
		go func() {
			for i := 0; i < 5; i++ {
				eventChan <- events.NewTextMessageContentEvent("msg1", "Line")
			}
			close(eventChan)
		}()

		wg.Wait()

		// Verify output
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 5 {
			t.Errorf("expected 5 lines, got %d", len(lines))
		}
	})
}

func TestStreamingJSONDecoder(t *testing.T) {
	t.Run("basic streaming", func(t *testing.T) {
		decoder := NewStreamingJSONDecoder(nil)

		ndjson := `{"type":"TEXT_MESSAGE_START","messageId":"msg1","timestamp":1234567890}
{"type":"TEXT_MESSAGE_CONTENT","messageId":"msg1","delta":"Hello","timestamp":1234567891}
{"type":"TEXT_MESSAGE_END","messageId":"msg1","timestamp":1234567892}
`
		reader := strings.NewReader(ndjson)

		if err := decoder.StartStream(context.Background(), reader); err != nil {
			t.Fatalf("failed to start stream: %v", err)
		}

		// Read all events
		var decodedEvents []events.Event
		for {
			event, err := decoder.ReadEvent(context.Background())
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("failed to read event: %v", err)
			}
			decodedEvents = append(decodedEvents, event)
		}

		if err := decoder.EndStream(context.Background()); err != nil {
			t.Fatalf("failed to end stream: %v", err)
		}

		if len(decodedEvents) != 3 {
			t.Errorf("expected 3 events, got %d", len(decodedEvents))
		}
	})

	t.Run("skip empty lines", func(t *testing.T) {
		decoder := NewStreamingJSONDecoder(nil)

		ndjson := `{"type":"TEXT_MESSAGE_START","messageId":"msg1","timestamp":1234567890}

{"type":"TEXT_MESSAGE_END","messageId":"msg1","timestamp":1234567892}
`
		reader := strings.NewReader(ndjson)

		if err := decoder.StartStream(context.Background(), reader); err != nil {
			t.Fatalf("failed to start stream: %v", err)
		}

		// Read all events
		var decodedEvents []events.Event
		for {
			event, err := decoder.ReadEvent(context.Background())
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("failed to read event: %v", err)
			}
			decodedEvents = append(decodedEvents, event)
		}

		if len(decodedEvents) != 2 {
			t.Errorf("expected 2 events, got %d", len(decodedEvents))
		}
	})

	t.Run("concurrent decoding", func(t *testing.T) {
		decoder := NewStreamingJSONDecoder(nil)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		ndjson := `{"type":"TEXT_MESSAGE_START","messageId":"msg1","timestamp":1234567890}
{"type":"TEXT_MESSAGE_CONTENT","messageId":"msg1","delta":"Hello","timestamp":1234567891}
{"type":"TEXT_MESSAGE_END","messageId":"msg1","timestamp":1234567892}
`
		reader := strings.NewReader(ndjson)
		eventChan := make(chan events.Event, 10)

		// Decode in a goroutine
		go func() {
			if err := decoder.DecodeStream(ctx, reader, eventChan); err != nil && err != context.DeadlineExceeded {
				t.Errorf("DecodeStream failed: %v", err)
			}
		}()

		// Collect events
		var collectedEvents []events.Event
		for event := range eventChan {
			collectedEvents = append(collectedEvents, event)
		}

		if len(collectedEvents) != 3 {
			t.Errorf("expected 3 events, got %d", len(collectedEvents))
		}
	})
}

func TestJSONCodec(t *testing.T) {
	codec := NewDefaultJSONCodec()

	t.Run("round trip single event", func(t *testing.T) {
		original := events.NewTextMessageStartEvent("msg123", events.WithRole("user"))

		// Encode
		data, err := codec.Encode(context.Background(), original)
		if err != nil {
			t.Fatalf("failed to encode: %v", err)
		}

		// Decode
		decoded, err := codec.Decode(context.Background(), data)
		if err != nil {
			t.Fatalf("failed to decode: %v", err)
		}

		// Verify
		decodedMsg, ok := decoded.(*events.TextMessageStartEvent)
		if !ok {
			t.Fatalf("expected TextMessageStartEvent, got %T", decoded)
		}

		if decodedMsg.MessageID != original.MessageID {
			t.Errorf("messageId mismatch: expected %s, got %s", original.MessageID, decodedMsg.MessageID)
		}
	})

	t.Run("round trip multiple events", func(t *testing.T) {
		originals := []events.Event{
			events.NewTextMessageStartEvent("msg1"),
			events.NewTextMessageContentEvent("msg1", "Hello"),
			events.NewTextMessageEndEvent("msg1"),
		}

		// Encode
		data, err := codec.EncodeMultiple(context.Background(), originals)
		if err != nil {
			t.Fatalf("failed to encode: %v", err)
		}

		// Decode
		decoded, err := codec.DecodeMultiple(context.Background(), data)
		if err != nil {
			t.Fatalf("failed to decode: %v", err)
		}

		// Verify
		if len(decoded) != len(originals) {
			t.Errorf("event count mismatch: expected %d, got %d", len(originals), len(decoded))
		}

		for i := range originals {
			if decoded[i].Type() != originals[i].Type() {
				t.Errorf("event %d type mismatch: expected %s, got %s", i, originals[i].Type(), decoded[i].Type())
			}
		}
	})
}

func TestThreadSafety(t *testing.T) {
	encoder := NewJSONEncoder(nil)
	decoder := NewJSONDecoder(nil)

	// Test concurrent encoding
	t.Run("concurrent encoding", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				event := events.NewTextMessageContentEvent("msg1", "content")
				_, err := encoder.Encode(context.Background(), event)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("concurrent encoding error: %v", err)
		}
	})

	// Test concurrent decoding
	t.Run("concurrent decoding", func(t *testing.T) {
		jsonData := `{"type":"TEXT_MESSAGE_CONTENT","messageId":"msg1","delta":"test","timestamp":1234567890}`
		var wg sync.WaitGroup
		errors := make(chan error, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				_, err := decoder.Decode(context.Background(), []byte(jsonData))
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("concurrent decoding error: %v", err)
		}
	})
}
