package protobuf

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

func TestProtobufIntegration_CompleteWorkflow(t *testing.T) {
	// Create codec
	codec := NewProtobufCodec(nil, nil)

	// Simulate a complete workflow
	workflow := []events.Event{
		// Run starts
		&events.RunStartedEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.RunStarted,
				ID:        "run-001",
				Timestamp: time.Now().Unix(),
				SessionID: "session-123",
			},
			RunID: "workflow-run-001",
		},
		// Step 1: User message
		&events.StepStartedEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.StepStarted,
				ID:        "step-001",
			},
			StepID:   "user-input",
			StepType: "message",
		},
		&events.TextMessageStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.TextMessageStart,
				ID:        "msg-001",
			},
			Role:   "user",
			Sender: "user-123",
		},
		&events.TextMessageContentEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.TextMessageContent,
				ID:        "content-001",
			},
			Content: "Calculate the sum of 15 and 27",
		},
		&events.TextMessageEndEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.TextMessageEnd,
				ID:        "msg-end-001",
			},
		},
		&events.StepFinishedEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.StepFinished,
				ID:        "step-end-001",
			},
			StepID: "user-input",
		},
		// Step 2: Tool call
		&events.StepStartedEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.StepStarted,
				ID:        "step-002",
			},
			StepID:   "tool-execution",
			StepType: "tool",
		},
		&events.ToolCallStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.ToolCallStart,
				ID:        "tool-001",
			},
			ToolCallID: "calc-001",
			ToolName:   "calculator",
		},
		&events.ToolCallArgsEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.ToolCallArgs,
				ID:        "args-001",
			},
			ToolCallID: "calc-001",
			Arguments:  `{"operation": "add", "a": 15, "b": 27}`,
			Complete:   true,
		},
		&events.ToolCallEndEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.ToolCallEnd,
				ID:        "tool-end-001",
			},
			ToolCallID: "calc-001",
			Output:     "42",
		},
		&events.StepFinishedEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.StepFinished,
				ID:        "step-end-002",
			},
			StepID: "tool-execution",
		},
		// State update
		&events.StateSnapshotEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.StateSnapshot,
				ID:        "state-001",
			},
			State: map[string]interface{}{
				"last_calculation": 42,
				"total_calculations": 1,
			},
		},
		// Run completes
		&events.RunFinishedEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.RunFinished,
				ID:        "run-end-001",
			},
			RunID: "workflow-run-001",
		},
	}

	// Test 1: Single event encoding/decoding
	for _, event := range workflow {
		data, err := codec.Encode(event)
		if err != nil {
			t.Fatalf("Failed to encode %s: %v", event.Type(), err)
		}

		decoded, err := codec.Decode(data)
		if err != nil {
			t.Fatalf("Failed to decode %s: %v", event.Type(), err)
		}

		if decoded.Type() != event.Type() {
			t.Errorf("Type mismatch: got %s, want %s", decoded.Type(), event.Type())
		}
	}

	// Test 2: Batch encoding/decoding
	batchData, err := codec.EncodeMultiple(workflow)
	if err != nil {
		t.Fatalf("Failed to encode batch: %v", err)
	}

	decodedBatch, err := codec.DecodeMultiple(batchData)
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
	if err := streamEncoder.StartStream(&buf); err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}

	for _, event := range workflow {
		if err := streamEncoder.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := streamEncoder.EndStream(); err != nil {
		t.Fatalf("Failed to end stream: %v", err)
	}

	// Stream decode
	ctx := context.Background()
	eventChan := make(chan events.Event, len(workflow))
	
	streamDecoder := streamCodec.GetStreamDecoder()
	if err := streamDecoder.StartStream(&buf); err != nil {
		t.Fatalf("Failed to start decode stream: %v", err)
	}

	go func() {
		for i := 0; i < len(workflow); i++ {
			event, err := streamDecoder.ReadEvent()
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
			if err := encoder.StartStream(&buf); err != nil {
				errors <- err
				return
			}

			for j := 0; j < eventsPerGoroutine; j++ {
				event := &events.CustomEvent{
					BaseEvent: events.BaseEvent{
						EventType: events.Custom,
						ID:        string(rune(id*1000 + j)),
					},
					EventName: "concurrent.test",
					Data: map[string]interface{}{
						"goroutine": id,
						"index":     j,
					},
				}

				if err := encoder.WriteEvent(event); err != nil {
					errors <- err
					return
				}
			}

			if err := encoder.EndStream(); err != nil {
				errors <- err
				return
			}

			// Decode events
			if err := decoder.StartStream(&buf); err != nil {
				errors <- err
				return
			}

			for j := 0; j < eventsPerGoroutine; j++ {
				event, err := decoder.ReadEvent()
				if err != nil {
					errors <- err
					return
				}

				if event.Type() != events.Custom.String() {
					errors <- fmt.Errorf("unexpected event type: %s", event.Type())
					return
				}
			}

			if err := decoder.EndStream(); err != nil {
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
		_, err := decoder.Decode([]byte("invalid protobuf data"))
		if err == nil {
			t.Error("Expected error for invalid data")
		}
	})

	t.Run("size limits", func(t *testing.T) {
		encoder := NewProtobufEncoder(&encoding.EncodingOptions{
			MaxSize: 50, // Very small limit
		})

		// Create large event
		event := &events.CustomEvent{
			BaseEvent: events.BaseEvent{EventType: events.Custom, ID: "large"},
			EventName: "large.event",
			Data: map[string]interface{}{
				"data": "This is a very large piece of data that should exceed our size limit",
			},
		}

		_, err := encoder.Encode(event)
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
		event := &events.RunStartedEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.RunStarted,
				ID:        "", // Invalid: empty ID
			},
			RunID: "", // Invalid: empty RunID
		}

		data, _ := encoder.Encode(event)
		_, err := decoder.Decode(data)
		if err == nil {
			t.Error("Expected validation error")
		}
	})
}