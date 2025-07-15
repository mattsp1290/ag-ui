// Package pkg provides integration tests for the AG-UI Go SDK
package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/state"
	"github.com/ag-ui/go-sdk/pkg/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCoreEventsTransportIntegration verifies that core events flow correctly through the transport layer
func TestCoreEventsTransportIntegration(t *testing.T) {
	ctx := context.Background()

	// Create a simple in-memory transport for testing
	tr := transport.NewMemoryTransport(100)
	defer tr.Close(ctx)

	// Connect transport
	err := tr.Connect(ctx)
	require.NoError(t, err)

	// Create different event types
	testCases := []struct {
		name  string
		event events.Event
	}{
		{
			name: "TextMessageStartEvent",
			event: &events.TextMessageStartEvent{
				BaseEvent:  events.NewBaseEvent(events.EventTypeTextMessageStart),
				MessageID:  "msg-123",
				Role:       ptrString("assistant"),
			},
		},
		{
			name: "StateSnapshotEvent",
			event: &events.StateSnapshotEvent{
				BaseEvent: events.NewBaseEvent(events.EventTypeStateSnapshot),
				Snapshot: map[string]interface{}{
					"key1": "value1",
					"key2": 42,
				},
			},
		},
		{
			name: "ToolCallStartEvent",
			event: &events.ToolCallStartEvent{
				BaseEvent:    events.NewBaseEvent(events.EventTypeToolCallStart),
				ToolCallID:   "tool-456",
				ToolCallName: "calculator",
			},
		},
	}

	// Test sending each event type
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert to transport event
			transportEvent := &transport.SimpleTransportEvent{
				EventID:        fmt.Sprintf("event-%d", time.Now().UnixNano()),
				EventType:      string(tc.event.Type()),
				EventTimestamp: time.Now(),
				EventData:      eventToMap(tc.event),
			}

			// Send event
			err := tr.Send(ctx, transportEvent)
			assert.NoError(t, err)

			// Receive event
			select {
			case received := <-tr.Receive():
				assert.NotNil(t, received)
				assert.Equal(t, tc.event.Type(), received.Type())
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for event")
			}
		})
	}
}

// TestStateTransportIntegration tests state management with transport layer
func TestStateTransportIntegration(t *testing.T) {
	ctx := context.Background()

	// Create state manager with larger event buffer to avoid queue full errors
	opts := state.DefaultManagerOptions()
	opts.MaxHistorySize = 100
	opts.EventBufferSize = 1000
	opts.BatchSize = 100
	sm, err := state.NewStateManager(opts)
	require.NoError(t, err)
	defer sm.Close()

	// Create transport
	tr := transport.NewMemoryTransport(100)
	defer tr.Close(ctx)

	// Connect transport
	err = tr.Connect(ctx)
	require.NoError(t, err)

	// Set up state change handler that sends events through transport
	// Note: StateManager doesn't have OnStateChange directly, 
	// we would need to use event handlers, but for this test we'll send events manually

	// Test state updates
	testData := []struct {
		key   string
		value interface{}
	}{
		{"user", "john_doe"},
		{"counter", 1},
		{"settings", map[string]interface{}{"theme": "dark", "lang": "en"}},
	}

	// Apply state changes
	for _, td := range testData {
		updates := map[string]interface{}{td.key: td.value}
		_, err := sm.UpdateState(ctx, "test-context", "test-state", updates, state.UpdateOptions{})
		assert.NoError(t, err)

		// Manually create and send state delta event
		event := &events.StateDeltaEvent{
			BaseEvent: events.NewBaseEvent(events.EventTypeStateDelta),
			Delta: []events.JSONPatchOperation{
				{
					Op:    "replace",
					Path:  "/" + td.key,
					Value: td.value,
				},
			},
		}

		transportEvent := &transport.SimpleTransportEvent{
			EventID:        fmt.Sprintf("state-%d", time.Now().UnixNano()),
			EventType:      string(event.Type()),
			EventTimestamp: time.Now(),
			EventData:      eventToMap(event),
		}

		err = tr.Send(ctx, transportEvent)
		assert.NoError(t, err)

		// Verify event was sent
		select {
		case receivedEvent := <-tr.Receive():
			assert.Equal(t, events.EventTypeStateDelta, receivedEvent.Type())
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for state delta event")
		}
	}
}

// TestTypedEventsTransportIntegration tests typed events through transport
func TestTypedEventsTransportIntegration(t *testing.T) {
	ctx := context.Background()

	// Create transport
	tr := transport.NewMemoryTransport(100)
	defer tr.Close(ctx)

	err := tr.Connect(ctx)
	require.NoError(t, err)

	// Test typed event with validation
	// Create a custom event using BaseEvent
	typedEvent := &events.BaseEvent{
		EventType: events.EventType("user.login"),
	}
	typedEvent.SetTimestamp(time.Now().UnixMilli())

	// Validate event
	err = typedEvent.Validate()
	assert.NoError(t, err)

	// Convert to transport event
	transportEvent := &transport.SimpleTransportEvent{
		EventID:        fmt.Sprintf("typed-event-%d", time.Now().UnixNano()),
		EventType:      string(typedEvent.Type()),
		EventTimestamp: time.Now(),
		EventData: map[string]interface{}{
			"username":  "testuser",
			"ip":        "192.168.1.1",
			"timestamp": time.Now().Unix(),
		},
	}

	// Send through transport
	err = tr.Send(ctx, transportEvent)
	assert.NoError(t, err)

	// Receive and verify
	select {
	case received := <-tr.Receive():
		assert.NotNil(t, received)
		// Verify data integrity
		baseEvent, ok := received.(*events.BaseEvent)
		require.True(t, ok)
		assert.Equal(t, events.EventType("user.login"), baseEvent.Type())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for typed event")
	}
}

// TestConcurrentEventFlow tests concurrent event processing
func TestConcurrentEventFlow(t *testing.T) {
	ctx := context.Background()

	// Create components
	sm, err := state.NewStateManager(state.ManagerOptions{
		MaxHistorySize: 1000,
	})
	require.NoError(t, err)
	defer sm.Close()

	tr := transport.NewMemoryTransport(1000)
	defer tr.Close(ctx)

	err = tr.Connect(ctx)
	require.NoError(t, err)

	// Concurrent event producers
	numProducers := 10
	eventsPerProducer := 100
	var wg sync.WaitGroup

	// Event counter
	receivedEvents := make(map[string]int)
	var mu sync.Mutex

	// Start receiver
	go func() {
		for event := range tr.Receive() {
			mu.Lock()
			receivedEvents[string(event.Type())]++
			mu.Unlock()
		}
	}()

	// Start producers
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			
			for j := 0; j < eventsPerProducer; j++ {
				var event events.Event
				
				switch j % 3 {
				case 0:
					event = &events.TextMessageContentEvent{
						BaseEvent: events.NewBaseEvent(events.EventTypeTextMessageContent),
						MessageID: fmt.Sprintf("msg-%d-%d", producerID, j),
				Delta:     fmt.Sprintf("Message from producer %d, event %d", producerID, j),
					}
				case 1:
					event = &events.StateSnapshotEvent{
						BaseEvent: events.NewBaseEvent(events.EventTypeStateSnapshot),
						Snapshot: map[string]interface{}{
							"producer": producerID,
							"event":    j,
						},
					}
				case 2:
					event = &events.ToolCallArgsEvent{
						BaseEvent:  events.NewBaseEvent(events.EventTypeToolCallArgs),
						ToolCallID: fmt.Sprintf("tool-%d-%d", producerID, j),
						Delta:      fmt.Sprintf(`{"producer": %d, "event": %d}`, producerID, j),
					}
				}

				transportEvent := &transport.SimpleTransportEvent{
					EventID:        fmt.Sprintf("event-%d-%d", producerID, j),
					EventType:      string(event.Type()),
					EventTimestamp: time.Now(),
					EventData:      eventToMap(event),
				}

				err := tr.Send(ctx, transportEvent)
				if err != nil {
					t.Errorf("Failed to send event: %v", err)
				}
			}
		}(i)
	}

	// Wait for all producers
	wg.Wait()

	// Give receiver time to process
	time.Sleep(500 * time.Millisecond)

	// Verify all events were received
	mu.Lock()
	defer mu.Unlock()
	
	totalExpected := numProducers * eventsPerProducer
	totalReceived := 0
	for _, count := range receivedEvents {
		totalReceived += count
	}
	
	assert.Equal(t, totalExpected, totalReceived, "Not all events were received")
}

// TestErrorPropagation tests error handling across packages
func TestErrorPropagation(t *testing.T) {
	ctx := context.Background()

	// Create transport with small buffer to trigger backpressure
	tr := transport.NewMemoryTransport(1)
	defer tr.Close(ctx)

	err := tr.Connect(ctx)
	require.NoError(t, err)

	// Send events rapidly to trigger backpressure
	errors := make([]error, 0)
	for i := 0; i < 10; i++ {
		event := &transport.SimpleTransportEvent{
			EventID:        fmt.Sprintf("event-%d", i),
			EventType:      "test",
			EventTimestamp: time.Now(),
			EventData:      map[string]interface{}{"index": i},
		}
		
		err := tr.Send(ctx, event)
		if err != nil {
			errors = append(errors, err)
		}
	}

	// Should have some backpressure errors
	assert.NotEmpty(t, errors, "Expected backpressure errors")
	
	// Check error channel
	select {
	case err := <-tr.Errors():
		assert.NotNil(t, err)
	case <-time.After(100 * time.Millisecond):
		// May not have errors in error channel
	}
}

// Helper functions

func ptrString(s string) *string {
	return &s
}

func eventToMap(event events.Event) map[string]interface{} {
	// Simple conversion - in real implementation would use proper serialization
	data := make(map[string]interface{})
	
	// Marshal event to JSON and back to map
	jsonData, _ := json.Marshal(event)
	json.Unmarshal(jsonData, &data)
	
	return data
}