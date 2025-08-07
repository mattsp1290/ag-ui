package events

import (
	"fmt"
	"time"
)

// TypedEventBuilder provides a fluent interface for building typed events
type TypedEventBuilder[T EventDataType] struct {
	eventType EventType
	timestamp *int64
	data      T
	validator func(T) error

	// Auto-generation flags
	autoGenerateIDs bool
}

// NewTypedEventBuilder creates a new typed event builder
func NewTypedEventBuilder[T EventDataType]() *TypedEventBuilder[T] {
	return &TypedEventBuilder[T]{}
}

// OfType sets the event type
func (b *TypedEventBuilder[T]) OfType(eventType EventType) *TypedEventBuilder[T] {
	b.eventType = eventType
	return b
}

// WithData sets the typed event data
func (b *TypedEventBuilder[T]) WithData(data T) *TypedEventBuilder[T] {
	b.data = data
	return b
}

// WithTimestamp sets the event timestamp
func (b *TypedEventBuilder[T]) WithTimestamp(timestamp int64) *TypedEventBuilder[T] {
	b.timestamp = &timestamp
	return b
}

// WithCurrentTimestamp sets the event timestamp to the current time
func (b *TypedEventBuilder[T]) WithCurrentTimestamp() *TypedEventBuilder[T] {
	now := time.Now().UnixMilli()
	b.timestamp = &now
	return b
}

// WithValidator sets a custom validator for the event data
func (b *TypedEventBuilder[T]) WithValidator(validator func(T) error) *TypedEventBuilder[T] {
	b.validator = validator
	return b
}

// WithAutoGenerateIDs enables automatic ID generation for empty fields
func (b *TypedEventBuilder[T]) WithAutoGenerateIDs() *TypedEventBuilder[T] {
	b.autoGenerateIDs = true
	return b
}

// Build constructs the final typed event
func (b *TypedEventBuilder[T]) Build() (TypedEvent[T], error) {
	// Validate event type is set
	if b.eventType == "" {
		return nil, fmt.Errorf("event type not set: use OfType() to set the event type before calling Build()")
	}

	// Apply auto-generation if enabled
	if b.autoGenerateIDs {
		b.applyAutoGeneration()
	}

	// Set timestamp if not provided
	if b.timestamp == nil {
		now := time.Now().UnixMilli()
		b.timestamp = &now
	}

	// Create the typed event
	event := &TypedBaseEvent[T]{
		BaseEvent: &BaseEvent{
			EventType:   b.eventType,
			TimestampMs: b.timestamp,
		},
		typedData: b.data,
	}

	// Apply custom validation if provided
	if b.validator != nil {
		if err := b.validator(b.data); err != nil {
			return nil, fmt.Errorf("custom validation failed: %w", err)
		}
	}

	// Validate the constructed event
	if err := event.Validate(); err != nil {
		return nil, fmt.Errorf("built event validation failed: %w", err)
	}

	return event, nil
}

// applyAutoGeneration generates IDs for empty fields based on event type and data
func (b *TypedEventBuilder[T]) applyAutoGeneration() {
	// Type assertion to apply auto-generation based on data type
	switch data := any(&b.data).(type) {
	case *MessageEventData:
		if data.MessageID == "" {
			data.MessageID = GenerateMessageID()
		}
	case *ToolCallEventData:
		if data.ToolCallID == "" {
			data.ToolCallID = GenerateToolCallID()
		}
	case *RunEventData:
		if data.RunID == "" {
			data.RunID = GenerateRunID()
		}
		if data.ThreadID == "" {
			data.ThreadID = GenerateThreadID()
		}
	case *StepEventData:
		if data.StepName == "" {
			data.StepName = GenerateStepID()
		}
	}
}

// Specialized builders for specific event types

// MessageEventBuilder provides a fluent interface for building message events
type MessageEventBuilder struct {
	*TypedEventBuilder[MessageEventData]
	data MessageEventData
}

// NewMessageEventBuilder creates a new message event builder
func NewMessageEventBuilder() *MessageEventBuilder {
	builder := &MessageEventBuilder{
		TypedEventBuilder: NewTypedEventBuilder[MessageEventData](),
		data:              MessageEventData{},
	}
	builder.WithData(builder.data)
	return builder
}

// MessageStart configures the builder for a TEXT_MESSAGE_START event
func (b *MessageEventBuilder) MessageStart() *MessageEventBuilder {
	b.OfType(EventTypeTextMessageStart)
	return b
}

// MessageContent configures the builder for a TEXT_MESSAGE_CONTENT event
func (b *MessageEventBuilder) MessageContent() *MessageEventBuilder {
	b.OfType(EventTypeTextMessageContent)
	return b
}

// MessageEnd configures the builder for a TEXT_MESSAGE_END event
func (b *MessageEventBuilder) MessageEnd() *MessageEventBuilder {
	b.OfType(EventTypeTextMessageEnd)
	return b
}

// WithMessageID sets the message ID
func (b *MessageEventBuilder) WithMessageID(messageID string) *MessageEventBuilder {
	b.data.MessageID = messageID
	b.WithData(b.data)
	return b
}

// WithRole sets the message role
func (b *MessageEventBuilder) WithRole(role string) *MessageEventBuilder {
	b.data.Role = &role
	b.WithData(b.data)
	return b
}

// WithDelta sets the delta content
func (b *MessageEventBuilder) WithDelta(delta string) *MessageEventBuilder {
	b.data.Delta = delta
	b.WithData(b.data)
	return b
}

// ToolCallEventBuilder provides a fluent interface for building tool call events
type ToolCallEventBuilder struct {
	*TypedEventBuilder[ToolCallEventData]
	data ToolCallEventData
}

// NewToolCallEventBuilder creates a new tool call event builder
func NewToolCallEventBuilder() *ToolCallEventBuilder {
	builder := &ToolCallEventBuilder{
		TypedEventBuilder: NewTypedEventBuilder[ToolCallEventData](),
		data:              ToolCallEventData{},
	}
	builder.WithData(builder.data)
	return builder
}

// ToolCallStart configures the builder for a TOOL_CALL_START event
func (b *ToolCallEventBuilder) ToolCallStart() *ToolCallEventBuilder {
	b.OfType(EventTypeToolCallStart)
	return b
}

// ToolCallArgs configures the builder for a TOOL_CALL_ARGS event
func (b *ToolCallEventBuilder) ToolCallArgs() *ToolCallEventBuilder {
	b.OfType(EventTypeToolCallArgs)
	return b
}

// ToolCallEnd configures the builder for a TOOL_CALL_END event
func (b *ToolCallEventBuilder) ToolCallEnd() *ToolCallEventBuilder {
	b.OfType(EventTypeToolCallEnd)
	return b
}

// WithToolCallID sets the tool call ID
func (b *ToolCallEventBuilder) WithToolCallID(toolCallID string) *ToolCallEventBuilder {
	b.data.ToolCallID = toolCallID
	b.WithData(b.data)
	return b
}

// WithToolCallName sets the tool call name
func (b *ToolCallEventBuilder) WithToolCallName(toolCallName string) *ToolCallEventBuilder {
	b.data.ToolCallName = toolCallName
	b.WithData(b.data)
	return b
}

// WithParentMessageID sets the parent message ID
func (b *ToolCallEventBuilder) WithParentMessageID(parentMessageID string) *ToolCallEventBuilder {
	b.data.ParentMessageID = &parentMessageID
	b.WithData(b.data)
	return b
}

// WithDelta sets the delta content
func (b *ToolCallEventBuilder) WithDelta(delta string) *ToolCallEventBuilder {
	b.data.Delta = delta
	b.WithData(b.data)
	return b
}

// RunEventBuilder provides a fluent interface for building run events
type RunEventBuilder struct {
	*TypedEventBuilder[RunEventData]
	data RunEventData
}

// NewRunEventBuilder creates a new run event builder
func NewRunEventBuilder() *RunEventBuilder {
	builder := &RunEventBuilder{
		TypedEventBuilder: NewTypedEventBuilder[RunEventData](),
		data:              RunEventData{},
	}
	builder.WithData(builder.data)
	return builder
}

// RunStarted configures the builder for a RUN_STARTED event
func (b *RunEventBuilder) RunStarted() *RunEventBuilder {
	b.OfType(EventTypeRunStarted)
	return b
}

// RunFinished configures the builder for a RUN_FINISHED event
func (b *RunEventBuilder) RunFinished() *RunEventBuilder {
	b.OfType(EventTypeRunFinished)
	return b
}

// RunError configures the builder for a RUN_ERROR event
func (b *RunEventBuilder) RunError() *RunEventBuilder {
	b.OfType(EventTypeRunError)
	return b
}

// WithRunID sets the run ID
func (b *RunEventBuilder) WithRunID(runID string) *RunEventBuilder {
	b.data.RunID = runID
	b.WithData(b.data)
	return b
}

// WithThreadID sets the thread ID
func (b *RunEventBuilder) WithThreadID(threadID string) *RunEventBuilder {
	b.data.ThreadID = threadID
	b.WithData(b.data)
	return b
}

// WithMessage sets the error message (for error events)
func (b *RunEventBuilder) WithMessage(message string) *RunEventBuilder {
	b.data.Message = message
	b.WithData(b.data)
	return b
}

// WithCode sets the error code (for error events)
func (b *RunEventBuilder) WithCode(code string) *RunEventBuilder {
	b.data.Code = &code
	b.WithData(b.data)
	return b
}

// StepEventBuilder provides a fluent interface for building step events
type StepEventBuilder struct {
	*TypedEventBuilder[StepEventData]
	data StepEventData
}

// NewStepEventBuilder creates a new step event builder
func NewStepEventBuilder() *StepEventBuilder {
	builder := &StepEventBuilder{
		TypedEventBuilder: NewTypedEventBuilder[StepEventData](),
		data:              StepEventData{},
	}
	builder.WithData(builder.data)
	return builder
}

// StepStarted configures the builder for a STEP_STARTED event
func (b *StepEventBuilder) StepStarted() *StepEventBuilder {
	b.OfType(EventTypeStepStarted)
	return b
}

// StepFinished configures the builder for a STEP_FINISHED event
func (b *StepEventBuilder) StepFinished() *StepEventBuilder {
	b.OfType(EventTypeStepFinished)
	return b
}

// WithStepName sets the step name
func (b *StepEventBuilder) WithStepName(stepName string) *StepEventBuilder {
	b.data.StepName = stepName
	b.WithData(b.data)
	return b
}

// StateEventBuilder provides a fluent interface for building state events
type StateEventBuilder[T EventDataType] struct {
	*TypedEventBuilder[T]
}

// NewStateSnapshotEventBuilder creates a builder for state snapshot events
func NewStateSnapshotEventBuilder() *TypedEventBuilder[StateSnapshotEventData] {
	return NewTypedEventBuilder[StateSnapshotEventData]().OfType(EventTypeStateSnapshot)
}

// NewStateDeltaEventBuilder creates a builder for state delta events
func NewStateDeltaEventBuilder() *TypedEventBuilder[StateDeltaEventData] {
	return NewTypedEventBuilder[StateDeltaEventData]().OfType(EventTypeStateDelta)
}

// NewMessagesSnapshotEventBuilder creates a builder for messages snapshot events
func NewMessagesSnapshotEventBuilder() *TypedEventBuilder[MessagesSnapshotEventData] {
	return NewTypedEventBuilder[MessagesSnapshotEventData]().OfType(EventTypeMessagesSnapshot)
}

// NewRawEventBuilder creates a builder for raw events
func NewRawEventBuilder() *TypedEventBuilder[RawEventData] {
	return NewTypedEventBuilder[RawEventData]().OfType(EventTypeRaw)
}

// NewCustomEventBuilder creates a builder for custom events
func NewCustomEventBuilder() *TypedEventBuilder[CustomEventData] {
	return NewTypedEventBuilder[CustomEventData]().OfType(EventTypeCustom)
}

// Convenience functions for quick event creation

// QuickMessageStart creates a message start event with minimal configuration
func QuickMessageStart(messageID string, role *string) (TypedEvent[MessageEventData], error) {
	builder := NewMessageEventBuilder().
		MessageStart().
		WithMessageID(messageID)

	if role != nil {
		builder = builder.WithRole(*role)
	}

	return builder.Build()
}

// QuickMessageContent creates a message content event with minimal configuration
func QuickMessageContent(messageID, delta string) (TypedEvent[MessageEventData], error) {
	return NewMessageEventBuilder().
		MessageContent().
		WithMessageID(messageID).
		WithDelta(delta).
		Build()
}

// QuickMessageEnd creates a message end event with minimal configuration
func QuickMessageEnd(messageID string) (TypedEvent[MessageEventData], error) {
	return NewMessageEventBuilder().
		MessageEnd().
		WithMessageID(messageID).
		Build()
}

// QuickToolCallStart creates a tool call start event with minimal configuration
func QuickToolCallStart(toolCallID, toolCallName string, parentMessageID *string) (TypedEvent[ToolCallEventData], error) {
	builder := NewToolCallEventBuilder().
		ToolCallStart().
		WithToolCallID(toolCallID).
		WithToolCallName(toolCallName)

	if parentMessageID != nil {
		builder = builder.WithParentMessageID(*parentMessageID)
	}

	return builder.Build()
}

// QuickToolCallArgs creates a tool call args event with minimal configuration
func QuickToolCallArgs(toolCallID, delta string) (TypedEvent[ToolCallEventData], error) {
	return NewToolCallEventBuilder().
		ToolCallArgs().
		WithToolCallID(toolCallID).
		WithDelta(delta).
		Build()
}

// QuickToolCallEnd creates a tool call end event with minimal configuration
func QuickToolCallEnd(toolCallID string) (TypedEvent[ToolCallEventData], error) {
	return NewToolCallEventBuilder().
		ToolCallEnd().
		WithToolCallID(toolCallID).
		Build()
}

// QuickRunStarted creates a run started event with minimal configuration
func QuickRunStarted(runID, threadID string) (TypedEvent[RunEventData], error) {
	return NewRunEventBuilder().
		RunStarted().
		WithRunID(runID).
		WithThreadID(threadID).
		Build()
}

// QuickRunFinished creates a run finished event with minimal configuration
func QuickRunFinished(runID, threadID string) (TypedEvent[RunEventData], error) {
	return NewRunEventBuilder().
		RunFinished().
		WithRunID(runID).
		WithThreadID(threadID).
		Build()
}

// QuickRunError creates a run error event with minimal configuration
func QuickRunError(runID, message string, code *string) (TypedEvent[RunEventData], error) {
	builder := NewRunEventBuilder().
		RunError().
		WithRunID(runID).
		WithMessage(message)

	if code != nil {
		builder = builder.WithCode(*code)
	}

	return builder.Build()
}

// QuickStepStarted creates a step started event with minimal configuration
func QuickStepStarted(stepName string) (TypedEvent[StepEventData], error) {
	return NewStepEventBuilder().
		StepStarted().
		WithStepName(stepName).
		Build()
}

// QuickStepFinished creates a step finished event with minimal configuration
func QuickStepFinished(stepName string) (TypedEvent[StepEventData], error) {
	return NewStepEventBuilder().
		StepFinished().
		WithStepName(stepName).
		Build()
}

// QuickStateSnapshot creates a state snapshot event with minimal configuration
func QuickStateSnapshot(snapshot interface{}) (TypedEvent[StateSnapshotEventData], error) {
	data := StateSnapshotEventData{Snapshot: snapshot}
	return NewStateSnapshotEventBuilder().WithData(data).Build()
}

// QuickStateDelta creates a state delta event with minimal configuration
func QuickStateDelta(delta []JSONPatchOperation) (TypedEvent[StateDeltaEventData], error) {
	data := StateDeltaEventData{Delta: delta}
	return NewStateDeltaEventBuilder().WithData(data).Build()
}

// QuickMessagesSnapshot creates a messages snapshot event with minimal configuration
func QuickMessagesSnapshot(messages []Message) (TypedEvent[MessagesSnapshotEventData], error) {
	data := MessagesSnapshotEventData{Messages: messages}
	return NewMessagesSnapshotEventBuilder().WithData(data).Build()
}

// QuickRawEvent creates a raw event with minimal configuration
func QuickRawEvent(event interface{}, source *string) (TypedEvent[RawEventData], error) {
	data := RawEventData{Event: event, Source: source}
	return NewRawEventBuilder().WithData(data).Build()
}

// QuickCustomEvent creates a custom event with minimal configuration
func QuickCustomEvent(name string, value interface{}) (TypedEvent[CustomEventData], error) {
	data := CustomEventData{Name: name, Value: value}
	return NewCustomEventBuilder().WithData(data).Build()
}
