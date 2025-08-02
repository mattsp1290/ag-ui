package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/proto/generated"
)

// TypedEvent interface for core events with type safety
// This bridges the gap between core types and events package
type TypedEvent[T EventDataType] interface {
	// ID returns the unique identifier for this event
	ID() string
	
	// Type returns the event type
	Type() EventType
	
	// Timestamp returns the event timestamp (Unix milliseconds)
	Timestamp() *int64
	
	// SetTimestamp sets the event timestamp
	SetTimestamp(timestamp int64)
	
	// TypedData returns the strongly-typed event data
	TypedData() T
	
	// Validate validates the event structure and content
	Validate() error
	
	// ToJSON serializes the event to JSON for cross-SDK compatibility
	ToJSON() ([]byte, error)
	
	// ToProtobuf converts the event to its protobuf representation
	ToProtobuf() (*generated.Event, error)
	
	// GetBaseEvent returns the underlying base event
	GetBaseEvent() *BaseEvent
	
	// ToLegacyEvent converts to the legacy Event interface for compatibility
	ToLegacyEvent() Event
}

// TypedBaseEvent provides common typed event implementation
type TypedBaseEvent[T EventDataType] struct {
	*BaseEvent
	typedData T
}

// NewTypedEvent creates a new typed event
func NewTypedEvent[T EventDataType](eventType EventType, data T) TypedEvent[T] {
	now := time.Now().UnixMilli()
	return &TypedBaseEvent[T]{
		BaseEvent: &BaseEvent{
			EventType:   eventType,
			TimestampMs: &now,
		},
		typedData: data,
	}
}

// TypedData returns the strongly-typed event data
func (e *TypedBaseEvent[T]) TypedData() T {
	return e.typedData
}

// Validate validates the typed event
func (e *TypedBaseEvent[T]) Validate() error {
	// Validate base event
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}
	
	// Validate typed data
	return e.typedData.Validate()
}

// ToJSON serializes the typed event to JSON
func (e *TypedBaseEvent[T]) ToJSON() ([]byte, error) {
	eventData := map[string]interface{}{
		"type": e.EventType,
		"data": e.typedData.ToMap(),
	}
	
	if e.TimestampMs != nil {
		eventData["timestamp"] = *e.TimestampMs
	}
	
	return json.Marshal(eventData)
}

// ID returns the unique identifier for this event
func (e *TypedBaseEvent[T]) ID() string {
	if e.BaseEvent == nil {
		return ""
	}
	return e.BaseEvent.ID()
}

// Type returns the event type
func (e *TypedBaseEvent[T]) Type() EventType {
	return e.EventType
}

// Timestamp returns the event timestamp
func (e *TypedBaseEvent[T]) Timestamp() *int64 {
	return e.TimestampMs
}

// SetTimestamp sets the event timestamp
func (e *TypedBaseEvent[T]) SetTimestamp(timestamp int64) {
	e.TimestampMs = &timestamp
}

// GetBaseEvent returns the underlying base event
func (e *TypedBaseEvent[T]) GetBaseEvent() *BaseEvent {
	return e.BaseEvent
}

// ThreadID returns the thread ID (delegate to base event's default implementation)
func (e *TypedBaseEvent[T]) ThreadID() string {
	if e.BaseEvent != nil {
		return e.BaseEvent.ThreadID()
	}
	return ""
}

// RunID returns the run ID (delegate to base event's default implementation)
func (e *TypedBaseEvent[T]) RunID() string {
	if e.BaseEvent != nil {
		return e.BaseEvent.RunID()
	}
	return ""
}

// ToLegacyEvent converts to the legacy Event interface
func (e *TypedBaseEvent[T]) ToLegacyEvent() Event {
	// This requires mapping to the appropriate legacy event type
	// Implementation depends on the specific event type
	switch e.EventType {
	case EventTypeTextMessageStart:
		if msgData, ok := any(e.typedData).(MessageEventData); ok {
			return &TextMessageStartEvent{
				BaseEvent: e.BaseEvent,
				MessageID: msgData.MessageID,
				Role:      msgData.Role,
			}
		}
	case EventTypeTextMessageContent:
		if msgData, ok := any(e.typedData).(MessageEventData); ok {
			return &TextMessageContentEvent{
				BaseEvent: e.BaseEvent,
				MessageID: msgData.MessageID,
				Delta:     msgData.Delta,
			}
		}
	case EventTypeTextMessageEnd:
		if msgData, ok := any(e.typedData).(MessageEventData); ok {
			return &TextMessageEndEvent{
				BaseEvent: e.BaseEvent,
				MessageID: msgData.MessageID,
			}
		}
	case EventTypeToolCallStart:
		if toolData, ok := any(e.typedData).(ToolCallEventData); ok {
			return &ToolCallStartEvent{
				BaseEvent:       e.BaseEvent,
				ToolCallID:      toolData.ToolCallID,
				ToolCallName:    toolData.ToolCallName,
				ParentMessageID: toolData.ParentMessageID,
			}
		}
	case EventTypeToolCallArgs:
		if toolData, ok := any(e.typedData).(ToolCallEventData); ok {
			return &ToolCallArgsEvent{
				BaseEvent:  e.BaseEvent,
				ToolCallID: toolData.ToolCallID,
				Delta:      toolData.Delta,
			}
		}
	case EventTypeToolCallEnd:
		if toolData, ok := any(e.typedData).(ToolCallEventData); ok {
			return &ToolCallEndEvent{
				BaseEvent:  e.BaseEvent,
				ToolCallID: toolData.ToolCallID,
			}
		}
	case EventTypeRunStarted:
		if runData, ok := any(e.typedData).(RunEventData); ok {
			return &RunStartedEvent{
				BaseEvent:     e.BaseEvent,
				ThreadIDValue: runData.ThreadID,
				RunIDValue:    runData.RunID,
			}
		}
	case EventTypeRunFinished:
		if runData, ok := any(e.typedData).(RunEventData); ok {
			return &RunFinishedEvent{
				BaseEvent:     e.BaseEvent,
				ThreadIDValue: runData.ThreadID,
				RunIDValue:    runData.RunID,
			}
		}
	case EventTypeRunError:
		if runData, ok := any(e.typedData).(RunEventData); ok {
			return &RunErrorEvent{
				BaseEvent:  e.BaseEvent,
				Message:    runData.Message,
				Code:       runData.Code,
				RunIDValue: runData.RunID,
			}
		}
	case EventTypeStepStarted:
		if stepData, ok := any(e.typedData).(StepEventData); ok {
			return &StepStartedEvent{
				BaseEvent: e.BaseEvent,
				StepName:  stepData.StepName,
			}
		}
	case EventTypeStepFinished:
		if stepData, ok := any(e.typedData).(StepEventData); ok {
			return &StepFinishedEvent{
				BaseEvent: e.BaseEvent,
				StepName:  stepData.StepName,
			}
		}
	case EventTypeStateSnapshot:
		if stateData, ok := any(e.typedData).(StateSnapshotEventData); ok {
			return &StateSnapshotEvent{
				BaseEvent: e.BaseEvent,
				Snapshot:  stateData.Snapshot,
			}
		}
	case EventTypeStateDelta:
		if deltaData, ok := any(e.typedData).(StateDeltaEventData); ok {
			return &StateDeltaEvent{
				BaseEvent: e.BaseEvent,
				Delta:     deltaData.Delta,
			}
		}
	case EventTypeMessagesSnapshot:
		if msgsData, ok := any(e.typedData).(MessagesSnapshotEventData); ok {
			return &MessagesSnapshotEvent{
				BaseEvent: e.BaseEvent,
				Messages:  msgsData.Messages,
			}
		}
	case EventTypeRaw:
		if rawData, ok := any(e.typedData).(RawEventData); ok {
			return &RawEvent{
				BaseEvent: e.BaseEvent,
				Event:     rawData.Event,
				Source:    rawData.Source,
			}
		}
	case EventTypeCustom:
		if customData, ok := any(e.typedData).(CustomEventData); ok {
			return &CustomEvent{
				BaseEvent: e.BaseEvent,
				Name:      customData.Name,
				Value:     customData.Value,
			}
		}
	}
	
	// Fallback: create a generic legacy event
	return &BaseEvent{
		EventType:   e.EventType,
		TimestampMs: e.TimestampMs,
		RawEvent:    e.typedData.ToMap(),
	}
}

// ToProtobuf converts the typed event to protobuf
func (e *TypedBaseEvent[T]) ToProtobuf() (*generated.Event, error) {
	// Delegate to the legacy event's ToProtobuf method
	legacyEvent := e.ToLegacyEvent()
	return legacyEvent.ToProtobuf()
}

// Specific typed event constructors with type safety

// NewTypedTextMessageStartEvent creates a typed text message start event
func NewTypedTextMessageStartEvent(messageID string, options ...TextMessageStartOption) TypedEvent[MessageEventData] {
	data := MessageEventData{
		MessageID: messageID,
	}
	
	event := NewTypedEvent(EventTypeTextMessageStart, data)
	
	// Apply options by converting to legacy event, applying options, then converting back
	legacyEvent := &TextMessageStartEvent{
		BaseEvent: event.GetBaseEvent(),
		MessageID: messageID,
	}
	
	for _, opt := range options {
		opt(legacyEvent)
	}
	
	// Update typed data from legacy event
	if legacyEvent.Role != nil {
		data.Role = legacyEvent.Role
	}
	
	return &TypedBaseEvent[MessageEventData]{
		BaseEvent: legacyEvent.BaseEvent,
		typedData: data,
	}
}

// NewTypedTextMessageContentEvent creates a typed text message content event
func NewTypedTextMessageContentEvent(messageID, delta string) TypedEvent[MessageEventData] {
	data := MessageEventData{
		MessageID: messageID,
		Delta:     delta,
	}
	
	return NewTypedEvent(EventTypeTextMessageContent, data)
}

// NewTypedTextMessageEndEvent creates a typed text message end event
func NewTypedTextMessageEndEvent(messageID string) TypedEvent[MessageEventData] {
	data := MessageEventData{
		MessageID: messageID,
	}
	
	return NewTypedEvent(EventTypeTextMessageEnd, data)
}

// NewTypedToolCallStartEvent creates a typed tool call start event
func NewTypedToolCallStartEvent(toolCallID, toolCallName string, parentMessageID *string) TypedEvent[ToolCallEventData] {
	data := ToolCallEventData{
		ToolCallID:      toolCallID,
		ToolCallName:    toolCallName,
		ParentMessageID: parentMessageID,
	}
	
	return NewTypedEvent(EventTypeToolCallStart, data)
}

// NewTypedToolCallArgsEvent creates a typed tool call args event
func NewTypedToolCallArgsEvent(toolCallID, delta string) TypedEvent[ToolCallEventData] {
	data := ToolCallEventData{
		ToolCallID: toolCallID,
		Delta:      delta,
	}
	
	return NewTypedEvent(EventTypeToolCallArgs, data)
}

// NewTypedToolCallEndEvent creates a typed tool call end event
func NewTypedToolCallEndEvent(toolCallID string) TypedEvent[ToolCallEventData] {
	data := ToolCallEventData{
		ToolCallID: toolCallID,
	}
	
	return NewTypedEvent(EventTypeToolCallEnd, data)
}

// NewTypedRunStartedEvent creates a typed run started event
func NewTypedRunStartedEvent(runID, threadID string) TypedEvent[RunEventData] {
	data := RunEventData{
		RunID:    runID,
		ThreadID: threadID,
	}
	
	return NewTypedEvent(EventTypeRunStarted, data)
}

// NewTypedRunFinishedEvent creates a typed run finished event
func NewTypedRunFinishedEvent(runID, threadID string) TypedEvent[RunEventData] {
	data := RunEventData{
		RunID:    runID,
		ThreadID: threadID,
	}
	
	return NewTypedEvent(EventTypeRunFinished, data)
}

// NewTypedRunErrorEvent creates a typed run error event
func NewTypedRunErrorEvent(runID, message string, code *string) TypedEvent[RunEventData] {
	data := RunEventData{
		RunID:   runID,
		Message: message,
		Code:    code,
	}
	
	return NewTypedEvent(EventTypeRunError, data)
}

// NewTypedStepStartedEvent creates a typed step started event
func NewTypedStepStartedEvent(stepName string) TypedEvent[StepEventData] {
	data := StepEventData{
		StepName: stepName,
	}
	
	return NewTypedEvent(EventTypeStepStarted, data)
}

// NewTypedStepFinishedEvent creates a typed step finished event
func NewTypedStepFinishedEvent(stepName string) TypedEvent[StepEventData] {
	data := StepEventData{
		StepName: stepName,
	}
	
	return NewTypedEvent(EventTypeStepFinished, data)
}

// NewTypedStateSnapshotEvent creates a typed state snapshot event
func NewTypedStateSnapshotEvent(snapshot interface{}) TypedEvent[StateSnapshotEventData] {
	data := StateSnapshotEventData{
		Snapshot: snapshot,
	}
	
	return NewTypedEvent(EventTypeStateSnapshot, data)
}

// NewTypedStateDeltaEvent creates a typed state delta event
func NewTypedStateDeltaEvent(delta []JSONPatchOperation) TypedEvent[StateDeltaEventData] {
	data := StateDeltaEventData{
		Delta: delta,
	}
	
	return NewTypedEvent(EventTypeStateDelta, data)
}

// NewTypedMessagesSnapshotEvent creates a typed messages snapshot event
func NewTypedMessagesSnapshotEvent(messages []Message) TypedEvent[MessagesSnapshotEventData] {
	data := MessagesSnapshotEventData{
		Messages: messages,
	}
	
	return NewTypedEvent(EventTypeMessagesSnapshot, data)
}

// NewTypedRawEvent creates a typed raw event
func NewTypedRawEvent(event interface{}, source *string) TypedEvent[RawEventData] {
	data := RawEventData{
		Event:  event,
		Source: source,
	}
	
	return NewTypedEvent(EventTypeRaw, data)
}

// NewTypedCustomEvent creates a typed custom event
func NewTypedCustomEvent(name string, value interface{}) TypedEvent[CustomEventData] {
	data := CustomEventData{
		Name:  name,
		Value: value,
	}
	
	return NewTypedEvent(EventTypeCustom, data)
}

// EventAdapter provides conversion utilities between typed and legacy events
type EventAdapter struct{}

// ToTypedEvent converts a legacy event to a typed event
func (a *EventAdapter) ToTypedEvent(legacyEvent Event) (interface{}, error) {
	switch e := legacyEvent.(type) {
	case *TextMessageStartEvent:
		data := MessageEventData{
			MessageID: e.MessageID,
			Role:      e.Role,
		}
		return &TypedBaseEvent[MessageEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *TextMessageContentEvent:
		data := MessageEventData{
			MessageID: e.MessageID,
			Delta:     e.Delta,
		}
		return &TypedBaseEvent[MessageEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *TextMessageEndEvent:
		data := MessageEventData{
			MessageID: e.MessageID,
		}
		return &TypedBaseEvent[MessageEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *ToolCallStartEvent:
		data := ToolCallEventData{
			ToolCallID:      e.ToolCallID,
			ToolCallName:    e.ToolCallName,
			ParentMessageID: e.ParentMessageID,
		}
		return &TypedBaseEvent[ToolCallEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *ToolCallArgsEvent:
		data := ToolCallEventData{
			ToolCallID: e.ToolCallID,
			Delta:      e.Delta,
		}
		return &TypedBaseEvent[ToolCallEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *ToolCallEndEvent:
		data := ToolCallEventData{
			ToolCallID: e.ToolCallID,
		}
		return &TypedBaseEvent[ToolCallEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *RunStartedEvent:
		data := RunEventData{
			RunID:    e.RunID(),
			ThreadID: e.ThreadID(),
		}
		return &TypedBaseEvent[RunEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *RunFinishedEvent:
		data := RunEventData{
			RunID:    e.RunID(),
			ThreadID: e.ThreadID(),
		}
		return &TypedBaseEvent[RunEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *RunErrorEvent:
		data := RunEventData{
			RunID:   e.RunID(),
			Message: e.Message,
			Code:    e.Code,
		}
		return &TypedBaseEvent[RunEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *StepStartedEvent:
		data := StepEventData{
			StepName: e.StepName,
		}
		return &TypedBaseEvent[StepEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *StepFinishedEvent:
		data := StepEventData{
			StepName: e.StepName,
		}
		return &TypedBaseEvent[StepEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *StateSnapshotEvent:
		data := StateSnapshotEventData{
			Snapshot: e.Snapshot,
		}
		return &TypedBaseEvent[StateSnapshotEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *StateDeltaEvent:
		data := StateDeltaEventData{
			Delta: e.Delta,
		}
		return &TypedBaseEvent[StateDeltaEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *MessagesSnapshotEvent:
		data := MessagesSnapshotEventData{
			Messages: e.Messages,
		}
		return &TypedBaseEvent[MessagesSnapshotEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *RawEvent:
		data := RawEventData{
			Event:  e.Event,
			Source: e.Source,
		}
		return &TypedBaseEvent[RawEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	case *CustomEvent:
		data := CustomEventData{
			Name:  e.Name,
			Value: e.Value,
		}
		return &TypedBaseEvent[CustomEventData]{
			BaseEvent: e.BaseEvent,
			typedData: data,
		}, nil
		
	default:
		return nil, fmt.Errorf("unsupported event type: %T", legacyEvent)
	}
}

// FromTypedEvent converts a typed event to a legacy event (already implemented in ToLegacyEvent)
func (a *EventAdapter) FromTypedEvent(typedEvent interface{}) (Event, error) {
	switch e := typedEvent.(type) {
	case TypedEvent[MessageEventData]:
		return e.ToLegacyEvent(), nil
	case TypedEvent[ToolCallEventData]:
		return e.ToLegacyEvent(), nil
	case TypedEvent[RunEventData]:
		return e.ToLegacyEvent(), nil
	case TypedEvent[StepEventData]:
		return e.ToLegacyEvent(), nil
	case TypedEvent[StateSnapshotEventData]:
		return e.ToLegacyEvent(), nil
	case TypedEvent[StateDeltaEventData]:
		return e.ToLegacyEvent(), nil
	case TypedEvent[MessagesSnapshotEventData]:
		return e.ToLegacyEvent(), nil
	case TypedEvent[RawEventData]:
		return e.ToLegacyEvent(), nil
	case TypedEvent[CustomEventData]:
		return e.ToLegacyEvent(), nil
	default:
		return nil, fmt.Errorf("unsupported typed event type: %T", typedEvent)
	}
}

// Integration with core package types

// ToCoreTypedEvent converts to core package TypedEvent
func ToCoreTypedEvent[T EventDataType](event TypedEvent[T]) (core.Event[map[string]interface{}], error) {
	// Convert EventDataType to core.EventData format
	eventData := event.TypedData()
	
	// Use the ToMap method to convert to map format
	mapData := eventData.ToMap()
	
	// Create a core event with map data
	coreEvent := core.NewEvent[map[string]interface{}]("", string(event.Type()), mapData)
	
	return coreEvent, nil
}

// FromCoreEvent converts from core package Event
func FromCoreEvent(coreEvent core.Event[map[string]interface{}]) (Event, error) {
	// Convert map data back to a typed event
	eventData := coreEvent.Data()
	
	// Parse the event type string to EventType
	eventType := EventType(coreEvent.Type())
	
	// Create a BaseEvent with the raw data
	timestamp := coreEvent.Timestamp().UnixMilli()
	baseEvent := &BaseEvent{
		EventType:   eventType,
		TimestampMs: &timestamp,
		RawEvent:    eventData,
	}
	
	return baseEvent, nil
}