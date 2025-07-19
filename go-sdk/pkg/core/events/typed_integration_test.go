package events

import (
	"context"
	"testing"
)

func TestTypedEventSystemIntegration(t *testing.T) {
	// Test full integration of typed event system
	t.Run("MessageEventLifecycle", func(t *testing.T) {
		// Create a complete message event lifecycle
		startEvent, err := NewMessageEventBuilder().
			MessageStart().
			WithMessageID("msg-123").
			WithRole("user").
			WithAutoGenerateIDs().
			Build()
		if err != nil {
			t.Fatalf("Failed to create message start event: %v", err)
		}

		contentEvent, err := NewMessageEventBuilder().
			MessageContent().
			WithMessageID("msg-123").
			WithDelta("Hello").
			Build()
		if err != nil {
			t.Fatalf("Failed to create message content event: %v", err)
		}

		endEvent, err := NewMessageEventBuilder().
			MessageEnd().
			WithMessageID("msg-123").
			Build()
		if err != nil {
			t.Fatalf("Failed to create message end event: %v", err)
		}

		// Test that all events are properly typed
		var _ TypedEvent[MessageEventData] = startEvent
		var _ TypedEvent[MessageEventData] = contentEvent
		var _ TypedEvent[MessageEventData] = endEvent

		// Test legacy conversion
		legacyStart := startEvent.ToLegacyEvent()
		if legacyStart.Type() != EventTypeTextMessageStart {
			t.Errorf("Legacy conversion failed for start event")
		}

		legacyContent := contentEvent.ToLegacyEvent()
		if legacyContent.Type() != EventTypeTextMessageContent {
			t.Errorf("Legacy conversion failed for content event")
		}

		legacyEnd := endEvent.ToLegacyEvent()
		if legacyEnd.Type() != EventTypeTextMessageEnd {
			t.Errorf("Legacy conversion failed for end event")
		}

		// Test JSON serialization
		jsonData, err := startEvent.ToJSON()
		if err != nil {
			t.Errorf("JSON serialization failed: %v", err)
		}
		if len(jsonData) == 0 {
			t.Error("JSON serialization returned empty data")
		}
	})

	t.Run("ToolCallEventLifecycle", func(t *testing.T) {
		// Create a complete tool call event lifecycle
		startEvent, err := NewToolCallEventBuilder().
			ToolCallStart().
			WithToolCallID("tool-123").
			WithToolCallName("calculator").
			WithParentMessageID("msg-456").
			Build()
		if err != nil {
			t.Fatalf("Failed to create tool call start event: %v", err)
		}

		argsEvent, err := NewToolCallEventBuilder().
			ToolCallArgs().
			WithToolCallID("tool-123").
			WithDelta("{\"operation\": \"add\"}").
			Build()
		if err != nil {
			t.Fatalf("Failed to create tool call args event: %v", err)
		}

		endEvent, err := NewToolCallEventBuilder().
			ToolCallEnd().
			WithToolCallID("tool-123").
			Build()
		if err != nil {
			t.Fatalf("Failed to create tool call end event: %v", err)
		}

		// Test that all events are properly typed
		var _ TypedEvent[ToolCallEventData] = startEvent
		var _ TypedEvent[ToolCallEventData] = argsEvent
		var _ TypedEvent[ToolCallEventData] = endEvent

		// Test typed data access
		startData := startEvent.TypedData()
		if startData.ToolCallID != "tool-123" {
			t.Errorf("Expected tool call ID 'tool-123', got '%s'", startData.ToolCallID)
		}
		if startData.ToolCallName != "calculator" {
			t.Errorf("Expected tool call name 'calculator', got '%s'", startData.ToolCallName)
		}
	})

	t.Run("RunEventLifecycle", func(t *testing.T) {
		// Create a complete run event lifecycle
		startEvent, err := NewRunEventBuilder().
			RunStarted().
			WithRunID("run-789").
			WithThreadID("thread-123").
			Build()
		if err != nil {
			t.Fatalf("Failed to create run start event: %v", err)
		}

		finishEvent, err := NewRunEventBuilder().
			RunFinished().
			WithRunID("run-789").
			WithThreadID("thread-123").
			Build()
		if err != nil {
			t.Fatalf("Failed to create run finish event: %v", err)
		}

		errorEvent, err := NewRunEventBuilder().
			RunError().
			WithRunID("run-error-789").
			WithMessage("Something went wrong").
			WithCode("INTERNAL_ERROR").
			Build()
		if err != nil {
			t.Fatalf("Failed to create run error event: %v", err)
		}

		// Test that all events are properly typed
		var _ TypedEvent[RunEventData] = startEvent
		var _ TypedEvent[RunEventData] = finishEvent
		var _ TypedEvent[RunEventData] = errorEvent

		// Test typed data access
		errorData := errorEvent.TypedData()
		if errorData.Message != "Something went wrong" {
			t.Errorf("Expected error message 'Something went wrong', got '%s'", errorData.Message)
		}
		if errorData.Code == nil || *errorData.Code != "INTERNAL_ERROR" {
			t.Errorf("Expected error code 'INTERNAL_ERROR', got %v", errorData.Code)
		}
	})

	t.Run("StepEventLifecycle", func(t *testing.T) {
		// Create step events
		startEvent, err := NewStepEventBuilder().
			StepStarted().
			WithStepName("parse_input").
			Build()
		if err != nil {
			t.Fatalf("Failed to create step start event: %v", err)
		}

		finishEvent, err := NewStepEventBuilder().
			StepFinished().
			WithStepName("parse_input").
			Build()
		if err != nil {
			t.Fatalf("Failed to create step finish event: %v", err)
		}

		// Test that events are properly typed
		var _ TypedEvent[StepEventData] = startEvent
		var _ TypedEvent[StepEventData] = finishEvent

		// Test typed data access
		startData := startEvent.TypedData()
		if startData.StepName != "parse_input" {
			t.Errorf("Expected step name 'parse_input', got '%s'", startData.StepName)
		}
	})

	t.Run("StateEventLifecycle", func(t *testing.T) {
		// Create state snapshot event
		snapshot := map[string]interface{}{
			"current_user": "alice",
			"session_id":   "sess-123",
			"step_count":   42,
		}

		snapshotEvent, err := NewStateSnapshotEventBuilder().
			WithData(StateSnapshotEventData{Snapshot: snapshot}).
			Build()
		if err != nil {
			t.Fatalf("Failed to create state snapshot event: %v", err)
		}

		// Create state delta event
		delta := []JSONPatchOperation{
			{
				Op:    "replace",
				Path:  "/step_count",
				Value: 43,
			},
		}

		deltaEvent, err := NewStateDeltaEventBuilder().
			WithData(StateDeltaEventData{Delta: delta}).
			Build()
		if err != nil {
			t.Fatalf("Failed to create state delta event: %v", err)
		}

		// Test that events are properly typed
		var _ TypedEvent[StateSnapshotEventData] = snapshotEvent
		var _ TypedEvent[StateDeltaEventData] = deltaEvent

		// Test typed data access
		snapshotData := snapshotEvent.TypedData()
		if snapshotData.Snapshot == nil {
			t.Error("Expected non-nil snapshot data")
		}

		deltaData := deltaEvent.TypedData()
		if len(deltaData.Delta) != 1 {
			t.Errorf("Expected 1 delta operation, got %d", len(deltaData.Delta))
		}
	})

	t.Run("TypedValidatorIntegration", func(t *testing.T) {
		// Create typed validator
		validator := NewTypedEventValidator[MessageEventData](nil)

		// Create test event
		event, err := NewMessageEventBuilder().
			MessageStart().
			WithMessageID("test-msg").
			WithRole("user").
			Build()
		if err != nil {
			t.Fatalf("Failed to create test event: %v", err)
		}

		// Validate the event
		ctx := context.Background()
		result := validator.ValidateTypedEvent(ctx, event)

		if !result.IsValid {
			t.Errorf("Event validation failed: %v", result.Errors)
		}

		if result.EventCount != 1 {
			t.Errorf("Expected event count 1, got %d", result.EventCount)
		}

		// Test sequence validation
		events := []TypedEvent[MessageEventData]{event}
		seqResult := validator.ValidateTypedSequence(ctx, events)

		if !seqResult.IsValid {
			t.Errorf("Sequence validation failed: %v", seqResult.Errors)
		}
	})

	t.Run("EventAdapterIntegration", func(t *testing.T) {
		adapter := &EventAdapter{}

		// Create a typed event
		typedEvent, err := NewMessageEventBuilder().
			MessageStart().
			WithMessageID("adapter-test").
			WithRole("assistant").
			Build()
		if err != nil {
			t.Fatalf("Failed to create typed event: %v", err)
		}

		// Convert to legacy event
		legacyEvent := typedEvent.ToLegacyEvent()

		// Convert back to typed event
		convertedTyped, err := adapter.ToTypedEvent(legacyEvent)
		if err != nil {
			t.Fatalf("Failed to convert back to typed event: %v", err)
		}

		// Verify the conversion
		if convertedTyped == nil {
			t.Error("Converted typed event should not be nil")
		}

		// Type assertion to verify correct type
		if convertedEvent, ok := convertedTyped.(TypedEvent[MessageEventData]); ok {
			convertedData := convertedEvent.TypedData()
			originalData := typedEvent.TypedData()

			if convertedData.MessageID != originalData.MessageID {
				t.Errorf("Message ID mismatch after conversion: expected %s, got %s",
					originalData.MessageID, convertedData.MessageID)
			}

			if (convertedData.Role == nil) != (originalData.Role == nil) {
				t.Error("Role pointer mismatch after conversion")
			}

			if convertedData.Role != nil && originalData.Role != nil &&
				*convertedData.Role != *originalData.Role {
				t.Errorf("Role value mismatch after conversion: expected %s, got %s",
					*originalData.Role, *convertedData.Role)
			}
		} else {
			t.Error("Failed to type assert converted event")
		}
	})
}

func TestTypedEventQuickBuilders(t *testing.T) {
	// Test all quick builder functions
	t.Run("QuickMessageEvents", func(t *testing.T) {
		role := "user"
		
		startEvent, err := QuickMessageStart("msg-123", &role)
		if err != nil {
			t.Errorf("QuickMessageStart failed: %v", err)
		}
		if startEvent.TypedData().MessageID != "msg-123" {
			t.Error("QuickMessageStart: incorrect message ID")
		}

		contentEvent, err := QuickMessageContent("msg-123", "Hello")
		if err != nil {
			t.Errorf("QuickMessageContent failed: %v", err)
		}
		if contentEvent.TypedData().Delta != "Hello" {
			t.Error("QuickMessageContent: incorrect delta")
		}

		endEvent, err := QuickMessageEnd("msg-123")
		if err != nil {
			t.Errorf("QuickMessageEnd failed: %v", err)
		}
		if endEvent.TypedData().MessageID != "msg-123" {
			t.Error("QuickMessageEnd: incorrect message ID")
		}
	})

	t.Run("QuickToolCallEvents", func(t *testing.T) {
		parentMsgID := "parent-msg-123"
		
		startEvent, err := QuickToolCallStart("tool-123", "calculator", &parentMsgID)
		if err != nil {
			t.Errorf("QuickToolCallStart failed: %v", err)
		}
		
		argsEvent, err := QuickToolCallArgs("tool-123", "{\"op\":\"add\"}")
		if err != nil {
			t.Errorf("QuickToolCallArgs failed: %v", err)
		}
		
		endEvent, err := QuickToolCallEnd("tool-123")
		if err != nil {
			t.Errorf("QuickToolCallEnd failed: %v", err)
		}

		// Verify data integrity
		if startEvent.TypedData().ToolCallID != "tool-123" {
			t.Error("QuickToolCallStart: incorrect tool call ID")
		}
		if argsEvent.TypedData().Delta != "{\"op\":\"add\"}" {
			t.Error("QuickToolCallArgs: incorrect delta")
		}
		if endEvent.TypedData().ToolCallID != "tool-123" {
			t.Error("QuickToolCallEnd: incorrect tool call ID")
		}
	})

	t.Run("QuickRunEvents", func(t *testing.T) {
		startEvent, err := QuickRunStarted("run-123", "thread-456")
		if err != nil {
			t.Errorf("QuickRunStarted failed: %v", err)
		}

		finishEvent, err := QuickRunFinished("run-123", "thread-456")
		if err != nil {
			t.Errorf("QuickRunFinished failed: %v", err)
		}

		code := "ERROR_CODE"
		errorEvent, err := QuickRunError("run-123", "Error message", &code)
		if err != nil {
			t.Errorf("QuickRunError failed: %v", err)
		}

		// Verify data integrity
		if startEvent.TypedData().RunID != "run-123" {
			t.Error("QuickRunStarted: incorrect run ID")
		}
		if finishEvent.TypedData().ThreadID != "thread-456" {
			t.Error("QuickRunFinished: incorrect thread ID")
		}
		if errorEvent.TypedData().Message != "Error message" {
			t.Error("QuickRunError: incorrect error message")
		}
	})

	t.Run("QuickStepEvents", func(t *testing.T) {
		startEvent, err := QuickStepStarted("process_input")
		if err != nil {
			t.Errorf("QuickStepStarted failed: %v", err)
		}

		finishEvent, err := QuickStepFinished("process_input")
		if err != nil {
			t.Errorf("QuickStepFinished failed: %v", err)
		}

		// Verify data integrity
		if startEvent.TypedData().StepName != "process_input" {
			t.Error("QuickStepStarted: incorrect step name")
		}
		if finishEvent.TypedData().StepName != "process_input" {
			t.Error("QuickStepFinished: incorrect step name")
		}
	})

	t.Run("QuickStateEvents", func(t *testing.T) {
		snapshot := map[string]interface{}{"key": "value"}
		snapshotEvent, err := QuickStateSnapshot(snapshot)
		if err != nil {
			t.Errorf("QuickStateSnapshot failed: %v", err)
		}

		delta := []JSONPatchOperation{{Op: "add", Path: "/test", Value: "value"}}
		deltaEvent, err := QuickStateDelta(delta)
		if err != nil {
			t.Errorf("QuickStateDelta failed: %v", err)
		}

		// Verify data integrity
		if snapshotEvent.TypedData().Snapshot == nil {
			t.Error("QuickStateSnapshot: nil snapshot")
		}
		if len(deltaEvent.TypedData().Delta) != 1 {
			t.Error("QuickStateDelta: incorrect delta length")
		}
	})
}