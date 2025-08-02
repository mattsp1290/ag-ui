// Package integration provides type safety integration tests
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTypeSafetyAcrossPackages verifies type safety between events and transport
func TestTypeSafetyAcrossPackages(t *testing.T) {
	ctx := context.Background()

	// Create transport
	tr := transport.NewMemoryTransport(100)
	defer tr.Close(ctx)

	err := tr.Connect(ctx)
	require.NoError(t, err)

	// Test various typed events
	testCases := []struct {
		name        string
		createEvent func() events.Event
		validateFn  func(t *testing.T, original, received events.Event)
	}{
		{
			name: "TypedEvent with validation",
			createEvent: func() events.Event {
				event := events.NewTypedCustomEvent("user.action", map[string]interface{}{
					"action": "click",
					"target": "button",
					"metadata": map[string]interface{}{
						"x": 100,
						"y": 200,
					},
				})
				return event.ToLegacyEvent()
			},
			validateFn: func(t *testing.T, original, received events.Event) {
				assert.Equal(t, events.EventTypeCustom, received.Type())
				assert.NotNil(t, received.Timestamp())
			},
		},
		{
			name: "TextMessageEvent with content",
			createEvent: func() events.Event {
				return &events.TextMessageContentEvent{
					BaseEvent: &events.BaseEvent{
						EventType:   events.EventTypeTextMessageContent,
						TimestampMs: ptrInt64(time.Now().UnixMilli()),
					},
					MessageID: "msg-123",
					Delta:     "Hello, this is a test message",
				}
			},
			validateFn: func(t *testing.T, original, received events.Event) {
				origMsg, ok := original.(*events.TextMessageContentEvent)
				require.True(t, ok)

				assert.Equal(t, origMsg.Type(), received.Type())
				assert.Equal(t, origMsg.Delta, "Hello, this is a test message")
			},
		},
		{
			name: "ToolCallEvent with arguments",
			createEvent: func() events.Event {
				return &events.ToolCallArgsEvent{
					BaseEvent: &events.BaseEvent{
						EventType:   events.EventTypeToolCallArgs,
						TimestampMs: ptrInt64(time.Now().UnixMilli()),
					},
					ToolCallID: "tool-123",
					Delta:      `{"query": "weather", "location": "San Francisco"}`,
				}
			},
			validateFn: func(t *testing.T, original, received events.Event) {
				origTool, ok := original.(*events.ToolCallArgsEvent)
				require.True(t, ok)

				assert.Equal(t, origTool.Type(), received.Type())
				assert.NotEmpty(t, origTool.Delta)
			},
		},
		{
			name: "StateSnapshotEvent with nested data",
			createEvent: func() events.Event {
				return &events.StateSnapshotEvent{
					BaseEvent: &events.BaseEvent{
						EventType:   events.EventTypeStateSnapshot,
						TimestampMs: ptrInt64(time.Now().UnixMilli()),
					},
					Snapshot: map[string]interface{}{
						"user": map[string]interface{}{
							"id":   "123",
							"name": "Test User",
							"preferences": map[string]interface{}{
								"theme": "dark",
								"lang":  "en",
							},
						},
						"session": map[string]interface{}{
							"id":        "sess-456",
							"startTime": time.Now().Unix(),
						},
					},
				}
			},
			validateFn: func(t *testing.T, original, received events.Event) {
				origState, ok := original.(*events.StateSnapshotEvent)
				require.True(t, ok)

				assert.Equal(t, origState.Type(), received.Type())
				assert.NotNil(t, origState.Snapshot)

				// Verify nested structure
				stateData, ok := origState.Snapshot.(map[string]interface{})
				require.True(t, ok)
				userData, ok := stateData["user"].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "123", userData["id"])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create event
			event := tc.createEvent()
			require.NotNil(t, event)

			// Validate event before sending
			err := event.Validate()
			assert.NoError(t, err)

			// Convert to transport event
			transportEvent := eventToTransportEvent(event)

			// Send through transport
			err = tr.Send(ctx, transportEvent)
			assert.NoError(t, err)

			// Receive event
			select {
			case received := <-tr.Receive():
				assert.NotNil(t, received)
				tc.validateFn(t, event, received)
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for event")
			}
		})
	}
}

// TestEventInterfaceCompatibility tests that all event types implement the interface correctly
func TestEventInterfaceCompatibility(t *testing.T) {
	// Test all event types implement the Event interface
	eventTypes := []events.Event{
		&events.BaseEvent{EventType: events.EventTypeCustom},
		&events.TextMessageStartEvent{BaseEvent: &events.BaseEvent{EventType: events.EventTypeTextMessageStart}},
		&events.TextMessageContentEvent{BaseEvent: &events.BaseEvent{EventType: events.EventTypeTextMessageContent}},
		&events.TextMessageEndEvent{BaseEvent: &events.BaseEvent{EventType: events.EventTypeTextMessageEnd}},
		&events.ToolCallStartEvent{BaseEvent: &events.BaseEvent{EventType: events.EventTypeToolCallStart}},
		&events.ToolCallArgsEvent{BaseEvent: &events.BaseEvent{EventType: events.EventTypeToolCallArgs}},
		&events.ToolCallEndEvent{BaseEvent: &events.BaseEvent{EventType: events.EventTypeToolCallEnd}},
		&events.StateSnapshotEvent{BaseEvent: &events.BaseEvent{EventType: events.EventTypeStateSnapshot}},
		&events.StateDeltaEvent{BaseEvent: &events.BaseEvent{EventType: events.EventTypeStateDelta}},
		&events.RunStartedEvent{BaseEvent: &events.BaseEvent{EventType: events.EventTypeRunStarted}},
		&events.RunFinishedEvent{BaseEvent: &events.BaseEvent{EventType: events.EventTypeRunFinished}},
		&events.RunErrorEvent{BaseEvent: &events.BaseEvent{EventType: events.EventTypeRunError}},
		events.NewTypedCustomEvent("test", nil).ToLegacyEvent(),
	}

	for _, event := range eventTypes {
		t.Run(string(event.Type()), func(t *testing.T) {
			// Verify interface methods
			assert.NotNil(t, event.Type())
			_ = event.Timestamp() // May be nil

			// Set timestamp
			event.SetTimestamp(time.Now().UnixMilli())
			assert.NotNil(t, event.Timestamp())

			// Validate
			err := event.Validate()
			// Some events may have validation errors without required fields
			// but the method should not panic
			_ = err
		})
	}
}

// TestTransportEventCompatibility tests transport event interface implementation
func TestTransportEventCompatibility(t *testing.T) {
	// Test transport event types
	transportEvents := []transport.TransportEvent{
		&transport.SimpleTransportEvent{
			EventID:        "test-1",
			EventType:      "test.event",
			EventTimestamp: time.Now(),
			EventData:      map[string]interface{}{"key": "value"},
		},
		&transport.CompositeEvent{
			BaseEvent: transport.SimpleTransportEvent{
				EventID:        "composite-1",
				EventType:      "composite.event",
				EventTimestamp: time.Now(),
			},
			Events: []transport.TransportEvent{
				&transport.SimpleTransportEvent{
					EventID:        "child-1",
					EventType:      "child.event",
					EventTimestamp: time.Now(),
				},
			},
		},
	}

	for _, event := range transportEvents {
		t.Run(event.Type(), func(t *testing.T) {
			// Verify interface methods
			assert.NotEmpty(t, event.ID())
			assert.NotEmpty(t, event.Type())
			assert.NotZero(t, event.Timestamp())
			assert.NotNil(t, event.Data())
		})
	}
}

// TestEventValidationIntegration tests event validation across packages
func TestEventValidationIntegration(t *testing.T) {
	ctx := context.Background()

	// Create transport with validation
	tr := transport.NewMemoryTransport(100)
	defer tr.Close(ctx)

	err := tr.Connect(ctx)
	require.NoError(t, err)

	// Test invalid events
	invalidEvents := []struct {
		name  string
		event events.Event
		error string
	}{
		{
			name: "Event without type",
			event: &events.BaseEvent{
				TimestampMs: ptrInt64(time.Now().UnixMilli()),
			},
			error: "type field is required",
		},
		{
			name: "TextMessage without ID",
			event: &events.TextMessageStartEvent{
				BaseEvent: &events.BaseEvent{
					EventType:   events.EventTypeTextMessageStart,
					TimestampMs: ptrInt64(time.Now().UnixMilli()),
				},
				Role: ptrString("assistant"),
				// Missing MessageID
			},
			error: "messageId field is required",
		},
		{
			name: "ToolCall without name",
			event: &events.ToolCallStartEvent{
				BaseEvent: &events.BaseEvent{
					EventType:   events.EventTypeToolCallStart,
					TimestampMs: ptrInt64(time.Now().UnixMilli()),
				},
				ToolCallID: "tool-123",
				// Missing ToolName
			},
			error: "toolCallName field is required",
		},
	}

	for _, tc := range invalidEvents {
		t.Run(tc.name, func(t *testing.T) {
			// Validate should fail
			err := tc.event.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.error)
		})
	}
}

// TestCrossPackageErrorHandling tests error propagation across packages
func TestCrossPackageErrorHandling(t *testing.T) {
	ctx := context.Background()

	// Create transport with small buffer to trigger errors
	tr := transport.NewMemoryTransport(1) // Small buffer to trigger backpressure
	defer tr.Close(ctx)

	err := tr.Connect(ctx)
	require.NoError(t, err)

	// Monitor errors
	errorCount := 0
	go func() {
		for err := range tr.Errors() {
			assert.NotNil(t, err)
			errorCount++
		}
	}()

	// Send many events to trigger backpressure
	for i := 0; i < 10; i++ {
		event := events.NewTypedCustomEvent("test.event", map[string]interface{}{
			"index": i,
		}).ToLegacyEvent()

		transportEvent := eventToTransportEvent(event)
		err := tr.Send(ctx, transportEvent)
		// Some sends may fail due to backpressure
		_ = err
	}

	// Give time for error processing
	time.Sleep(100 * time.Millisecond)

	// Should have received some errors
	assert.Greater(t, errorCount, 0, "Expected some errors due to backpressure")
}

func ptrInt64(v int64) *int64 {
	return &v
}

func ptrString(v string) *string {
	return &v
}
