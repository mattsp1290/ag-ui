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

	// Create memory transport directly
	tr := transport.NewMemoryTransport(100)
	defer tr.Close(ctx)

	// Connect
	err := tr.Connect(ctx)
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
				event, _ := builder.
					TextMessageStart().
					WithMessageID("msg-123").
					WithRole("assistant").
					Build()
				return event
			},
			validate: func(t *testing.T, event events.Event) {
				assert.Equal(t, events.EventTypeTextMessageStart, event.Type())
				assert.NotNil(t, event.Timestamp())
			},
		},
		{
			name: "ToolCallEvent",
			buildFn: func() events.Event {
				event, _ := builder.
					ToolCallStart().
					WithToolCallID("tool-456").
					WithToolCallName("search").
					Build()
				return event
			},
			validate: func(t *testing.T, event events.Event) {
				assert.Equal(t, events.EventTypeToolCallStart, event.Type())
			},
		},
		{
			name: "CustomEvent",
			buildFn: func() events.Event {
				event, _ := builder.
					Custom().
					WithCustomName("app.custom").
					WithCustomValue(map[string]interface{}{
						"action": "user_click",
						"target": "submit_button",
						"metadata": map[string]interface{}{
							"timestamp": time.Now().Unix(),
							"session":   "sess-789",
						},
					}).
					Build()
				return event
			},
			validate: func(t *testing.T, event events.Event) {
				assert.Equal(t, events.EventTypeCustom, event.Type())
				assert.NotNil(t, event.Timestamp())
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
			receiveChan, errChan := tr.Channels()
			select {
			case received := <-receiveChan:
				assert.NotNil(t, received)
				assert.Equal(t, event.Type(), received.Type())
			case err := <-errChan:
				t.Fatalf("received error: %v", err)
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
	opts := state.DefaultManagerOptions()
	opts.MaxHistorySize = 100
	opts.EnableMetrics = true
	sm, err := state.NewStateManager(opts)
	require.NoError(t, err)
	defer sm.Close()

	// Create transport
	tr := transport.NewMemoryTransport(100)
	defer tr.Close(ctx)

	err = tr.Connect(ctx)
	require.NoError(t, err)

	// Channel to collect events
	stateEvents := make(chan events.Event, 10)

	// Set up state change handler
	unsubscribe := sm.Subscribe("/", func(change state.StateChange) {
		// Convert StateChange to old/new state maps for compatibility
		oldState := make(map[string]interface{})
		newState := make(map[string]interface{})
		oldState[change.Path] = change.OldValue
		newState[change.Path] = change.NewValue
		// Create snapshot event
		snapshotEvent := &events.StateSnapshotEvent{
			BaseEvent: &events.BaseEvent{
				EventType:   events.EventTypeStateSnapshot,
				TimestampMs: ptr(time.Now().UnixMilli()),
			},
			Snapshot: newState,
		}

		// Create delta event
		deltaEvent := &events.StateDeltaEvent{
			BaseEvent: &events.BaseEvent{
				EventType:   events.EventTypeStateDelta,
				TimestampMs: ptr(time.Now().UnixMilli()),
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
	defer unsubscribe()

	// Test state operations
	operations := []struct {
		name   string
		action func() error
		verify func()
	}{
		{
			name: "SetState",
			action: func() error {
				contextID, err := sm.CreateContext(ctx, "test-state", nil)
				if err != nil {
					return err
				}
				updates := map[string]interface{}{
					"user": map[string]interface{}{
						"id":   "user-123",
						"name": "John Doe",
					},
				}
				_, err = sm.UpdateState(ctx, contextID, "test-state", updates, state.UpdateOptions{})
				return err
			},
			verify: func() {
				// Should receive snapshot and delta events
				receiveChan, errChan := tr.Channels()
				for i := 0; i < 2; i++ {
					select {
					case event := <-receiveChan:
						assert.NotNil(t, event)
						assert.Contains(t, []events.EventType{
							events.EventTypeStateSnapshot,
							events.EventTypeStateDelta,
						}, event.Type())
					case err := <-errChan:
						t.Fatalf("received error: %v", err)
					case <-time.After(time.Second):
						t.Fatal("timeout waiting for state event")
					}
				}
			},
		},
		{
			name: "UpdateState",
			action: func() error {
				contextID, err := sm.CreateContext(ctx, "test-state-2", nil)
				if err != nil {
					return err
				}
				updates := map[string]interface{}{
					"counter": 1,
				}
				_, err = sm.UpdateState(ctx, contextID, "test-state-2", updates, state.UpdateOptions{})
				return err
			},
			verify: func() {
				// Verify counter update events
				receiveChan, errChan := tr.Channels()
				eventCount := 0
				for i := 0; i < 2; i++ {
					select {
					case <-receiveChan:
						eventCount++
					case <-errChan:
						// Ignore errors for this test
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
	opts := state.DefaultManagerOptions()
	opts.MaxHistorySize = 1000
	opts.EnableMetrics = true
	sm, err := state.NewStateManager(opts)
	require.NoError(t, err)
	defer sm.Close()

	// Create multiple transports
	numTransports := 3
	transports := make([]transport.Transport, numTransports)
	
	for i := 0; i < numTransports; i++ {
		tr := transport.NewMemoryTransport(100)
		err := tr.Connect(ctx)
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
			receiveChan, errChan := transport.Channels()
			for {
				select {
				case event, ok := <-receiveChan:
					if !ok {
						return
					}
					mu.Lock()
					receivedCount++
					mu.Unlock()
					
					// Validate event
					assert.NotNil(t, event)
					assert.NoError(t, event.Validate())
				case <-errChan:
					// Ignore errors for this test
				case <-time.After(time.Second):
					return
				}
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
						BaseEvent: &events.BaseEvent{
							EventType:   events.EventTypeTextMessageContent,
							TimestampMs: ptr(time.Now().UnixMilli()),
						},
						MessageID: fmt.Sprintf("msg-%d-%d", senderID, j),
						Delta:     fmt.Sprintf("Message %d from sender %d", j, senderID),
					}
				case 1:
					event, _ = events.NewEventBuilder().
						Custom().
						WithCustomName("test.event").
						WithCustomValue(map[string]interface{}{
							"sender": senderID,
							"index":  j,
						}).
						Build()
				case 2:
					event = &events.RunStartedEvent{
						BaseEvent: &events.BaseEvent{
							EventType:   events.EventTypeRunStarted,
							TimestampMs: ptr(time.Now().UnixMilli()),
						},
						ThreadID: fmt.Sprintf("thread-%d", senderID),
						RunID:    fmt.Sprintf("run-%d-%d", senderID, j),
					}
				case 3:
					// State update that triggers events
					contextID, err := sm.CreateContext(ctx, fmt.Sprintf("state-%d", senderID), nil)
					if err == nil {
						updates := map[string]interface{}{
							fmt.Sprintf("key-%d", senderID): j,
						}
						sm.UpdateState(ctx, contextID, fmt.Sprintf("state-%d", senderID), updates, state.UpdateOptions{})
					}
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
	case *events.StateSnapshotEvent:
		data = map[string]interface{}{
			"snapshot": e.Snapshot,
		}
	case *events.StateDeltaEvent:
		data = map[string]interface{}{
			"delta": e.Delta,
		}
	case *events.TextMessageContentEvent:
		data = map[string]interface{}{
			"messageId": e.MessageID,
			"delta":     e.Delta,
		}
	case *events.RunStartedEvent:
		data = map[string]interface{}{
			"threadId": e.ThreadID,
			"runId":    e.RunID,
		}
	default:
		// Generic extraction - try to get any available data
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

func calculateDelta(oldState, newState map[string]interface{}) []events.JSONPatchOperation {
	var delta []events.JSONPatchOperation
	
	// Added or modified keys
	for key, newValue := range newState {
		if oldValue, exists := oldState[key]; !exists {
			delta = append(delta, events.JSONPatchOperation{
				Op:    "add",
				Path:  "/" + key,
				Value: newValue,
			})
		} else if oldValue != newValue {
			delta = append(delta, events.JSONPatchOperation{
				Op:    "replace",
				Path:  "/" + key,
				Value: newValue,
			})
		}
	}
	
	// Removed keys
	for key := range oldState {
		if _, exists := newState[key]; !exists {
			delta = append(delta, events.JSONPatchOperation{
				Op:   "remove",
				Path: "/" + key,
			})
		}
	}
	
	return delta
}