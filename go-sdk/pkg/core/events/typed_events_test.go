package events

import (
	"testing"
	"time"
)

// Helper function to create string pointer
func stringPtrTyped(s string) *string {
	return &s
}

func TestTypedEventDataInterface(t *testing.T) {
	// Test that all event data types implement EventDataType interface
	var eventDataTypes []EventDataType

	// Test MessageEventData
	messageData := MessageEventData{
		MessageID: "test-message-id",
		Role:      stringPtrTyped("user"),
		Delta:     "hello world",
	}
	eventDataTypes = append(eventDataTypes, messageData)

	// Test ToolCallEventData
	toolCallData := ToolCallEventData{
		ToolCallID:      "test-tool-id",
		ToolCallName:    "test-tool",
		ParentMessageID: stringPtrTyped("parent-msg-id"),
		Delta:           "tool delta",
	}
	eventDataTypes = append(eventDataTypes, toolCallData)

	// Test RunEventData
	runData := RunEventData{
		RunID:    "test-run-id",
		ThreadID: "test-thread-id",
		Message:  "test message",
		Code:     stringPtrTyped("ERROR_CODE"),
	}
	eventDataTypes = append(eventDataTypes, runData)

	// Test StepEventData
	stepData := StepEventData{
		StepName: "test-step",
	}
	eventDataTypes = append(eventDataTypes, stepData)

	// Test StateSnapshotEventData
	stateSnapshotData := StateSnapshotEventData{
		Snapshot: map[string]interface{}{"key": "value"},
	}
	eventDataTypes = append(eventDataTypes, stateSnapshotData)

	// Test StateDeltaEventData
	stateDeltaData := StateDeltaEventData{
		Delta: []JSONPatchOperation{
			{
				Op:    "add",
				Path:  "/test",
				Value: "test-value",
			},
		},
	}
	eventDataTypes = append(eventDataTypes, stateDeltaData)

	// Test MessagesSnapshotEventData
	messagesSnapshotData := MessagesSnapshotEventData{
		Messages: []Message{
			{
				ID:   "msg-1",
				Role: "user",
				Content: stringPtrTyped("Hello"),
			},
		},
	}
	eventDataTypes = append(eventDataTypes, messagesSnapshotData)

	// Test RawEventData
	rawData := RawEventData{
		Event:  map[string]interface{}{"raw": "data"},
		Source: stringPtrTyped("external"),
	}
	eventDataTypes = append(eventDataTypes, rawData)

	// Test CustomEventData
	customData := CustomEventData{
		Name:  "custom-event",
		Value: map[string]interface{}{"custom": "value"},
	}
	eventDataTypes = append(eventDataTypes, customData)

	// Test that all data types implement the interface correctly
	for i, data := range eventDataTypes {
		t.Run(data.DataType(), func(t *testing.T) {
			// Test Validate method
			if err := data.Validate(); err != nil {
				t.Errorf("Event data %d (%s) failed validation: %v", i, data.DataType(), err)
			}

			// Test ToMap method
			mapData := data.ToMap()
			if mapData == nil {
				t.Errorf("Event data %d (%s) ToMap returned nil", i, data.DataType())
			}

			// Test DataType method
			dataType := data.DataType()
			if dataType == "" {
				t.Errorf("Event data %d DataType returned empty string", i)
			}
		})
	}
}

func TestTypedBaseEventImplementsInterface(t *testing.T) {
	// Test that TypedBaseEvent implements TypedEvent interface
	messageData := MessageEventData{
		MessageID: "test-message-id",
		Role:      stringPtrTyped("user"),
		Delta:     "hello world",
	}

	now := time.Now().UnixMilli()
	typedEvent := &TypedBaseEvent[MessageEventData]{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeTextMessageStart,
			TimestampMs: &now,
		},
		typedData: messageData,
	}

	// Test that it implements TypedEvent interface
	var _ TypedEvent[MessageEventData] = typedEvent

	// Test interface methods
	if typedEvent.ID() == "" {
		t.Error("ID() should return a non-empty string")
	}

	if typedEvent.Type() != EventTypeTextMessageStart {
		t.Errorf("Type() should return %s, got %s", EventTypeTextMessageStart, typedEvent.Type())
	}

	if typedEvent.Timestamp() != &now {
		t.Error("Timestamp() should return the correct timestamp")
	}

	newTimestamp := time.Now().UnixMilli()
	typedEvent.SetTimestamp(newTimestamp)
	if *typedEvent.Timestamp() != newTimestamp {
		t.Error("SetTimestamp() should update the timestamp")
	}

	if typedEvent.TypedData() != messageData {
		t.Error("TypedData() should return the correct typed data")
	}

	if err := typedEvent.Validate(); err != nil {
		t.Errorf("Validate() should not return error: %v", err)
	}

	jsonData, err := typedEvent.ToJSON()
	if err != nil {
		t.Errorf("ToJSON() should not return error: %v", err)
	}
	if len(jsonData) == 0 {
		t.Error("ToJSON() should return non-empty JSON")
	}

	baseEvent := typedEvent.GetBaseEvent()
	if baseEvent == nil {
		t.Error("GetBaseEvent() should return a non-nil BaseEvent")
	}

	legacyEvent := typedEvent.ToLegacyEvent()
	if legacyEvent == nil {
		t.Error("ToLegacyEvent() should return a non-nil legacy event")
	}
}

func TestTypedEventBuilder(t *testing.T) {
	// Test that TypedEventBuilder creates valid events
	event, err := NewTypedEventBuilder[MessageEventData]().
		OfType(EventTypeTextMessageStart).
		WithData(MessageEventData{
			MessageID: "test-message-id",
			Role:      stringPtrTyped("user"),
			Delta:     "hello world",
		}).
		Build()

	if err != nil {
		t.Errorf("Builder should not return error: %v", err)
	}

	if event == nil {
		t.Error("Builder should return non-nil event")
	}

	// Test validation
	if err := event.Validate(); err != nil {
		t.Errorf("Built event should validate successfully: %v", err)
	}

	// Test interface compliance
	var _ TypedEvent[MessageEventData] = event

	// Test specialized builders
	msgEvent, err := NewMessageEventBuilder().
		MessageStart().
		WithMessageID("test-msg-id").
		WithRole("user").
		Build()

	if err != nil {
		t.Errorf("Message builder should not return error: %v", err)
	}

	if msgEvent == nil {
		t.Error("Message builder should return non-nil event")
	}

	if msgEvent.Type() != EventTypeTextMessageStart {
		t.Errorf("Message event should have type %s, got %s", EventTypeTextMessageStart, msgEvent.Type())
	}
}

func TestTypedEventValidation(t *testing.T) {
	// Test that typed events validate correctly
	tests := []struct {
		name      string
		eventType EventType
		data      EventDataType
		wantError bool
	}{
		{
			name:      "Valid MessageEventData",
			eventType: EventTypeTextMessageStart,
			data: MessageEventData{
				MessageID: "test-message-id",
				Role:      stringPtrTyped("user"),
			},
			wantError: false,
		},
		{
			name:      "Invalid MessageEventData - missing ID",
			eventType: EventTypeTextMessageStart,
			data: MessageEventData{
				Role: stringPtrTyped("user"),
			},
			wantError: true,
		},
		{
			name:      "Valid ToolCallEventData",
			eventType: EventTypeToolCallStart,
			data: ToolCallEventData{
				ToolCallID:   "test-tool-id",
				ToolCallName: "test-tool",
			},
			wantError: false,
		},
		{
			name:      "Invalid ToolCallEventData - missing ID",
			eventType: EventTypeToolCallStart,
			data: ToolCallEventData{
				ToolCallName: "test-tool",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now().UnixMilli()
			event := &TypedBaseEvent[EventDataType]{
				BaseEvent: &BaseEvent{
					EventType:   tt.eventType,
					TimestampMs: &now,
				},
				typedData: tt.data,
			}

			err := event.Validate()
			if tt.wantError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestEventDataTypePointerVsValueReceivers(t *testing.T) {
	// Test that all EventDataType methods use consistent receiver types
	messageData := MessageEventData{
		MessageID: "test-message-id",
		Role:      stringPtrTyped("user"),
		Delta:     "hello world",
	}

	// Test value receiver methods
	if err := messageData.Validate(); err != nil {
		t.Errorf("MessageEventData.Validate() with value receiver failed: %v", err)
	}

	mapData := messageData.ToMap()
	if mapData == nil {
		t.Error("MessageEventData.ToMap() with value receiver returned nil")
	}

	dataType := messageData.DataType()
	if dataType == "" {
		t.Error("MessageEventData.DataType() with value receiver returned empty string")
	}

	// Test pointer receiver methods (for mutable interface)
	messageDataPtr := &MessageEventData{}
	testMap := map[string]interface{}{
		"messageId": "test-msg-id",
		"role":      "user",
		"delta":     "test delta",
	}

	if err := messageDataPtr.FromMap(testMap); err != nil {
		t.Errorf("MessageEventData.FromMap() with pointer receiver failed: %v", err)
	}

	if messageDataPtr.MessageID != "test-msg-id" {
		t.Error("FromMap should have set MessageID correctly")
	}

	if messageDataPtr.Role == nil || *messageDataPtr.Role != "user" {
		t.Error("FromMap should have set Role correctly")
	}

	if messageDataPtr.Delta != "test delta" {
		t.Error("FromMap should have set Delta correctly")
	}
}

