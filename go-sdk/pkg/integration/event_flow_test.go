// Package integration provides comprehensive integration tests
package integration

import (
	"context"
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

// TestEventFlowIntegration tests the complete event flow from creation to transport
func TestEventFlowIntegration(t *testing.T) {
	ctx := context.Background()

	// Create transport manager with full manager for advanced features
	tm := transport.NewFullManager()
	
	// Create transport with validation
	tr, err := tm.CreateTransport(ctx, transport.TransportConfig{
		Type: "memory",
		Options: map[string]interface{}{
			"buffer_size": 100,
			"validate":    true,
		},
	})
	require.NoError(t, err)
	defer tr.Close(ctx)

	// Connect
	err = tr.Connect(ctx)
	require.NoError(t, err)

	// Test event builder
	builder := events.NewEventBuilder()
	
	// Build various events
	testEvents := []struct {
		name     string
		buildFn  func() events.Event
		validate func(t *testing.T, event events.Event)
	}{
		{
			name: "TextMessageEvent",
			buildFn: func() events.Event {
				return builder.
					WithType(events.EventTypeTextMessageStart).
					WithData("messageID", "msg-123").
					WithData("role", "assistant").
					Build()
			},
			validate: func(t *testing.T, event events.Event) {
				assert.Equal(t, events.EventTypeTextMessageStart, event.Type())
				assert.NotNil(t, event.Timestamp())
			},
		},
		{
			name: "ToolCallEvent",
			buildFn: func() events.Event {
				return builder.
					WithType(events.EventTypeToolCallStart).
					WithData("toolCallID", "tool-456").
					WithData("toolName", "search").
					WithData("arguments", map[string]interface{}{
						"query": "weather today",
					}).
					Build()
			},
			validate: func(t *testing.T, event events.Event) {
				assert.Equal(t, events.EventTypeToolCallStart, event.Type())
			},
		},
		{
			name: "CustomEvent",
			buildFn: func() events.Event {
				return events.NewTypedEvent("app.custom", map[string]interface{}{
					"action": "user_click",
					"target": "submit_button",
					"metadata": map[string]interface{}{
						"timestamp": time.Now().Unix(),
						"session":   "sess-789",
					},
				})
			},
			validate: func(t *testing.T, event events.Event) {
				typedEvent, ok := event.(*events.TypedEvent)
				require.True(t, ok)
				assert.Equal(t, "app.custom", typedEvent.EventType)
				assert.NotEmpty(t, typedEvent.ID())
			},
		},
	}

	for _, tc := range testEvents {
		t.Run(tc.name, func(t *testing.T) {
			// Build event
			event := tc.buildFn()
			require.NotNil(t, event)

			// Validate event
			err := event.Validate()
			assert.NoError(t, err)

			// Custom validation
			tc.validate(t, event)

			// Convert to transport event
			transportEvent := eventToTransportEvent(event)

			// Send through transport
			err = tr.Send(ctx, transportEvent)
			assert.NoError(t, err)

			// Receive and verify
			select {
			case received := <-tr.Receive():
				assert.NotNil(t, received)
				assert.Equal(t, event.Type(), received.Type())
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for event")
			}
		})
	}
}

// TestStateEventIntegration tests state changes triggering events
func TestStateEventIntegration(t *testing.T) {
	ctx := context.Background()

	// Create state manager
	sm := state.NewManager(state.ManagerConfig{
		MaxHistorySize: 100,
		SyncInterval:   50 * time.Millisecond,
	})
	defer sm.Shutdown(ctx)

	// Create transport
	tm := transport.NewFullManager()
	tr, err := tm.CreateTransport(ctx, transport.TransportConfig{
		Type: "memory",
		Options: map[string]interface{}{
			"buffer_size": 100,
		},
	})
	require.NoError(t, err)
	defer tr.Close(ctx)

	err = tr.Connect(ctx)
	require.NoError(t, err)

	// Channel to collect events
	stateEvents := make(chan events.Event, 10)

	// Set up state change handler
	sm.OnStateChange(func(oldState, newState map[string]interface{}) {
		// Create snapshot event
		snapshotEvent := &events.StateSnapshotEvent{
			BaseEvent: events.BaseEvent{
				EventType:      events.EventTypeStateSnapshot,
				EventTimestamp: ptr(time.Now().UnixMilli()),
			},
			StateData: newState,
		}

		// Create delta event
		deltaEvent := &events.StateDeltaEvent{
			BaseEvent: events.BaseEvent{
				EventType:      events.EventTypeStateDelta,
				EventTimestamp: ptr(time.Now().UnixMilli()),
			},
			Delta: calculateDelta(oldState, newState),
		}

		// Send both events
		for _, event := range []events.Event{snapshotEvent, deltaEvent} {
			transportEvent := eventToTransportEvent(event)
			err := tr.Send(ctx, transportEvent)
			if err == nil {
				stateEvents <- event
			}
		}
	})

	// Test state operations
	operations := []struct {
		name   string
		action func() error
		verify func()
	}{
		{
			name: "SetState",
			action: func() error {
				return sm.SetState(ctx, "user", map[string]interface{}{
					"id":   "user-123",
					"name": "John Doe",
				})
			},
			verify: func() {
				// Should receive snapshot and delta events
				for i := 0; i < 2; i++ {
					select {
					case event := <-tr.Receive():
						assert.NotNil(t, event)
						assert.Contains(t, []events.EventType{
							events.EventTypeStateSnapshot,
							events.EventTypeStateDelta,
						}, event.Type())
					case <-time.After(time.Second):
						t.Fatal("timeout waiting for state event")
					}
				}
			},
		},
		{
			name: "UpdateState",
			action: func() error {
				return sm.SetState(ctx, "counter", 1)
			},
			verify: func() {
				// Verify counter update events
				eventCount := 0
				for i := 0; i < 2; i++ {
					select {
					case <-tr.Receive():
						eventCount++
					case <-time.After(100 * time.Millisecond):
						break
					}
				}
				assert.GreaterOrEqual(t, eventCount, 1)
			},
		},
	}

	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			err := op.action()
			assert.NoError(t, err)
			op.verify()
		})
	}
}

// TestConcurrentIntegration tests concurrent operations across packages
func TestConcurrentIntegration(t *testing.T) {
	ctx := context.Background()

	// Create components
	sm := state.NewManager(state.ManagerConfig{
		MaxHistorySize: 1000,
	})
	defer sm.Shutdown(ctx)

	tm := transport.NewFullManager()
	
	// Create multiple transports
	numTransports := 3
	transports := make([]transport.Transport, numTransports)
	
	for i := 0; i < numTransports; i++ {
		tr, err := tm.CreateTransport(ctx, transport.TransportConfig{
			Type: "memory",
			Options: map[string]interface{}{
				"buffer_size": 100,
				"id":          fmt.Sprintf("transport-%d", i),
			},
		})
		require.NoError(t, err)
		err = tr.Connect(ctx)
		require.NoError(t, err)
		transports[i] = tr
		defer tr.Close(ctx)
	}

	// Metrics
	var sentCount, receivedCount int64
	var mu sync.Mutex

	// Start receivers
	var wg sync.WaitGroup
	for i, tr := range transports {
		wg.Add(1)
		go func(transportID int, transport transport.Transport) {
			defer wg.Done()
			for event := range transport.Receive() {
				mu.Lock()
				receivedCount++
				mu.Unlock()
				
				// Validate event
				assert.NotNil(t, event)
				assert.NoError(t, event.Validate())
			}
		}(i, tr)
	}

	// Concurrent senders
	numSenders := 5
	eventsPerSender := 20
	
	for i := 0; i < numSenders; i++ {
		wg.Add(1)
		go func(senderID int) {
			defer wg.Done()
			
			for j := 0; j < eventsPerSender; j++ {
				// Create different event types
				var event events.Event
				switch j % 4 {
				case 0:
					event = &events.TextMessageContentEvent{
						BaseEvent: events.BaseEvent{
							EventType:      events.EventTypeTextMessageContent,
							EventTimestamp: ptr(time.Now().UnixMilli()),
						},
						Content: fmt.Sprintf("Message %d from sender %d", j, senderID),
					}
				case 1:
					event = events.NewTypedEvent("test.event", map[string]interface{}{
						"sender": senderID,
						"index":  j,
					})
				case 2:
					event = &events.RunStartedEvent{
						BaseEvent: events.BaseEvent{
							EventType:      events.EventTypeRunStarted,
							EventTimestamp: ptr(time.Now().UnixMilli()),
						},
						RunID: fmt.Sprintf("run-%d-%d", senderID, j),
					}
				case 3:
					// State update that triggers events
					sm.SetState(ctx, fmt.Sprintf("key-%d", senderID), j)
					continue
				}

				// Send to random transport
				transportEvent := eventToTransportEvent(event)
				tr := transports[j%numTransports]
				
				err := tr.Send(ctx, transportEvent)
				if err == nil {
					mu.Lock()
					sentCount++
					mu.Unlock()
				}
			}
		}(i)
	}

	// Wait for senders to complete
	wg.Wait()

	// Give receivers time to process
	time.Sleep(500 * time.Millisecond)

	// Log results
	t.Logf("Sent: %d, Received: %d", sentCount, receivedCount)
}

// Helper functions

func ptr(v int64) *int64 {
	return &v
}

func eventToTransportEvent(event events.Event) transport.TransportEvent {
	// Convert events.Event to transport.TransportEvent
	var data map[string]interface{}
	
	// Type-specific data extraction
	switch e := event.(type) {
	case *events.TypedEvent:
		data = e.Data()
		data["id"] = e.ID()
	case *events.StateSnapshotEvent:
		data = map[string]interface{}{
			"state": e.StateData,
		}
	case *events.StateDeltaEvent:
		data = map[string]interface{}{
			"delta": e.Delta,
		}
	default:
		// Generic extraction
		data = map[string]interface{}{
			"type": string(event.Type()),
		}
	}

	return &transport.SimpleTransportEvent{
		EventID:        fmt.Sprintf("event-%d", time.Now().UnixNano()),
		EventType:      string(event.Type()),
		EventTimestamp: time.Now(),
		EventData:      data,
	}
}

func calculateDelta(oldState, newState map[string]interface{}) map[string]interface{} {
	delta := make(map[string]interface{})
	
	// Added or modified keys
	for key, newValue := range newState {
		if oldValue, exists := oldState[key]; !exists {
			delta[key] = map[string]interface{}{
				"op":    "add",
				"value": newValue,
			}
		} else if oldValue != newValue {
			delta[key] = map[string]interface{}{
				"op":       "replace",
				"oldValue": oldValue,
				"newValue": newValue,
			}
		}
	}
	
	// Removed keys
	for key := range oldState {
		if _, exists := newState[key]; !exists {
			delta[key] = map[string]interface{}{
				"op": "remove",
			}
		}
	}
	
	return delta
}