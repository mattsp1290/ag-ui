package protobuf

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
)

func TestProtobufIntegration_CompleteWorkflow(t *testing.T) {
	// Create codec
	codec := NewProtobufCodec(nil, nil)

	// Simulate a complete workflow
	workflow := []events.Event{
		// Run starts
		events.NewRunStartedEvent("thread-001", "workflow-run-001"),
		// Step 1: User message
		events.NewStepStartedEvent("user-input"),
		events.NewTextMessageStartEvent("msg-001", events.WithRole("user")),
		events.NewTextMessageContentEvent("msg-001", "Calculate the sum of 15 and 27"),
		events.NewTextMessageEndEvent("msg-001"),
		events.NewStepFinishedEvent("user-input"),
		// Step 2: Tool call
		events.NewStepStartedEvent("tool-execution"),
		events.NewToolCallStartEvent("calc-001", "calculator"),
		events.NewToolCallArgsEvent("calc-001", `{"operation": "add", "a": 15, "b": 27}`),
		events.NewToolCallEndEvent("calc-001"),
		events.NewStepFinishedEvent("tool-execution"),
		// State update
		events.NewStateSnapshotEvent(map[string]interface{}{
			"last_calculation":   42,
			"total_calculations": 1,
		}),
		// Run completes
		events.NewRunFinishedEvent("thread-001", "workflow-run-001"),
	}

	// Test 1: Single event encoding/decoding
	for _, event := range workflow {
		data, err := codec.Encode(context.Background(), event)
		if err != nil {
			t.Fatalf("Failed to encode %s: %v", event.Type(), err)
		}

		decoded, err := codec.Decode(context.Background(), data)
		if err != nil {
			t.Fatalf("Failed to decode %s: %v", event.Type(), err)
		}

		if decoded.Type() != event.Type() {
			t.Errorf("Type mismatch: got %s, want %s", decoded.Type(), event.Type())
		}
	}

	// Test 2: Batch encoding/decoding
	batchData, err := codec.EncodeMultiple(context.Background(), workflow)
	if err != nil {
		t.Fatalf("Failed to encode batch: %v", err)
	}

	decodedBatch, err := codec.DecodeMultiple(context.Background(), batchData)
	if err != nil {
		t.Fatalf("Failed to decode batch: %v", err)
	}

	if len(decodedBatch) != len(workflow) {
		t.Fatalf("Batch size mismatch: got %d, want %d", len(decodedBatch), len(workflow))
	}

	// Test 3: Streaming
	streamCodec := NewStreamingProtobufCodec(nil, nil)
	var buf bytes.Buffer

	// Stream encode
	streamEncoder := streamCodec.GetStreamEncoder()
	if err := streamEncoder.StartStream(context.Background(), &buf); err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}

	for _, event := range workflow {
		if err := streamEncoder.WriteEvent(context.Background(), event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := streamEncoder.EndStream(context.Background()); err != nil {
		t.Fatalf("Failed to end stream: %v", err)
	}

	// Stream decode
	eventChan := make(chan events.Event, len(workflow))

	streamDecoder := streamCodec.GetStreamDecoder()
	if err := streamDecoder.StartStream(context.Background(), &buf); err != nil {
		t.Fatalf("Failed to start decode stream: %v", err)
	}

	go func() {
		for i := 0; i < len(workflow); i++ {
			event, err := streamDecoder.ReadEvent(context.Background())
			if err != nil {
				break
			}
			eventChan <- event
		}
		close(eventChan)
	}()

	// Verify streamed events
	var streamedEvents []events.Event
	for event := range eventChan {
		streamedEvents = append(streamedEvents, event)
	}

	if len(streamedEvents) != len(workflow) {
		t.Fatalf("Streamed event count mismatch: got %d, want %d", len(streamedEvents), len(workflow))
	}
}

func TestProtobufIntegration_ConcurrentStreaming(t *testing.T) {
	// Test concurrent streaming operations
	streamCodec := NewStreamingProtobufCodec(nil, nil)

	numGoroutines := 10
	eventsPerGoroutine := 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			var buf bytes.Buffer
			encoder := streamCodec.GetStreamEncoder()
			decoder := streamCodec.GetStreamDecoder()

			// Encode events
			if err := encoder.StartStream(context.Background(), &buf); err != nil {
				errors <- err
				return
			}

			for j := 0; j < eventsPerGoroutine; j++ {
				event := events.NewCustomEvent(
					"concurrent.test",
					events.WithValue(map[string]interface{}{
						"goroutine": id,
						"index":     j,
					}),
				)

				if err := encoder.WriteEvent(context.Background(), event); err != nil {
					errors <- err
					return
				}
			}

			if err := encoder.EndStream(context.Background()); err != nil {
				errors <- err
				return
			}

			// Decode events
			if err := decoder.StartStream(context.Background(), &buf); err != nil {
				errors <- err
				return
			}

			for j := 0; j < eventsPerGoroutine; j++ {
				event, err := decoder.ReadEvent(context.Background())
				if err != nil {
					errors <- err
					return
				}

				if event.Type() != events.EventTypeCustom {
					errors <- fmt.Errorf("unexpected event type: %s", event.Type())
					return
				}
			}

			if err := decoder.EndStream(context.Background()); err != nil {
				errors <- err
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			t.Fatalf("Concurrent operation failed: %v", err)
		}
	}
}

func TestProtobufIntegration_ErrorHandling(t *testing.T) {
	// Test various error conditions

	t.Run("invalid data", func(t *testing.T) {
		decoder := NewProtobufDecoder(nil)

		// Random invalid data
		_, err := decoder.Decode(context.Background(), []byte("invalid protobuf data"))
		if err == nil {
			t.Error("Expected error for invalid data")
		}
	})

	t.Run("size limits", func(t *testing.T) {
		encoder := NewProtobufEncoder(&encoding.EncodingOptions{
			MaxSize: 50, // Very small limit
		})

		// Create large event
		event := events.NewCustomEvent(
			"large.event",
			events.WithValue(map[string]interface{}{
				"data": "This is a very large piece of data that should exceed our size limit",
			}),
		)

		_, err := encoder.Encode(context.Background(), event)
		if err == nil {
			t.Error("Expected error for size limit exceeded")
		}
	})

	t.Run("validation", func(t *testing.T) {
		decoder := NewProtobufDecoder(&encoding.DecodingOptions{
			ValidateEvents: true,
		})

		// Create invalid event (will fail validation)
		encoder := NewProtobufEncoder(nil)
		event := events.NewRunStartedEvent("", "") // Invalid: empty thread and run IDs

		data, _ := encoder.Encode(context.Background(), event)
		_, err := decoder.Decode(context.Background(), data)
		if err == nil {
			t.Error("Expected validation error")
		}
	})
}
