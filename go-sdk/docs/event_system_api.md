# Type-Safe Event System API Reference

## Overview

The AG-UI Go SDK provides a comprehensive type-safe event system for modeling and handling real-time communication between agents and front-end applications. The event system ensures compile-time type safety, automatic validation, and seamless serialization across different transport protocols.

## Core Event Interface

### Event Interface

All events implement the base Event interface:

```go
type Event interface {
    // GetEventType returns the event type identifier
    GetEventType() EventType
    
    // GetTimestamp returns the event timestamp in milliseconds
    GetTimestamp() *int64
    
    // Validate validates the event structure and required fields
    Validate() error
    
    // ToJSON serializes the event to JSON
    ToJSON() ([]byte, error)
    
    // ToProtobuf converts the event to protobuf representation
    ToProtobuf() (*generated.Event, error)
}
```

### Base Event Structure

```go
type BaseEvent struct {
    EventType   EventType `json:"eventType"`
    TimestampMs *int64    `json:"timestampMs,omitempty"`
}

func NewBaseEvent(eventType EventType) *BaseEvent {
    now := time.Now().UnixMilli()
    return &BaseEvent{
        EventType:   eventType,
        TimestampMs: &now,
    }
}
```

## Event Types

### Event Type Constants

```go
type EventType string

const (
    // Run lifecycle events
    EventTypeRunStarted  EventType = "RUN_STARTED"
    EventTypeRunFinished EventType = "RUN_FINISHED"
    EventTypeRunError    EventType = "RUN_ERROR"
    
    // Step lifecycle events
    EventTypeStepStarted  EventType = "STEP_STARTED"
    EventTypeStepFinished EventType = "STEP_FINISHED"
    
    // Message events
    EventTypeTextMessageStart   EventType = "TEXT_MESSAGE_START"
    EventTypeTextMessageContent EventType = "TEXT_MESSAGE_CONTENT"
    EventTypeTextMessageEnd     EventType = "TEXT_MESSAGE_END"
    
    // Tool execution events
    EventTypeToolCallStart EventType = "TOOL_CALL_START"
    EventTypeToolCallArgs  EventType = "TOOL_CALL_ARGS"
    EventTypeToolCallEnd   EventType = "TOOL_CALL_END"
    
    // State management events
    EventTypeStateSnapshot    EventType = "STATE_SNAPSHOT"
    EventTypeStateDelta       EventType = "STATE_DELTA"
    EventTypeMessagesSnapshot EventType = "MESSAGES_SNAPSHOT"
    
    // Generic events
    EventTypeRaw    EventType = "RAW"
    EventTypeCustom EventType = "CUSTOM"
)
```

## Event Builder Pattern

### EventBuilder

The EventBuilder provides a fluent interface for creating type-safe events:

```go
type EventBuilder struct {
    // Internal fields for building events
}

func NewEventBuilder() *EventBuilder {
    return &EventBuilder{}
}

// Event type configuration methods
func (b *EventBuilder) RunStarted() *EventBuilder
func (b *EventBuilder) RunFinished() *EventBuilder
func (b *EventBuilder) RunError() *EventBuilder
func (b *EventBuilder) StepStarted() *EventBuilder
func (b *EventBuilder) StepFinished() *EventBuilder
func (b *EventBuilder) TextMessageStart() *EventBuilder
func (b *EventBuilder) TextMessageContent() *EventBuilder
func (b *EventBuilder) TextMessageEnd() *EventBuilder
func (b *EventBuilder) ToolCallStart() *EventBuilder
func (b *EventBuilder) ToolCallArgs() *EventBuilder
func (b *EventBuilder) ToolCallEnd() *EventBuilder
func (b *EventBuilder) StateSnapshot() *EventBuilder
func (b *EventBuilder) StateDelta() *EventBuilder
func (b *EventBuilder) MessagesSnapshot() *EventBuilder
func (b *EventBuilder) Raw() *EventBuilder
func (b *EventBuilder) Custom() *EventBuilder

// Field configuration methods
func (b *EventBuilder) WithTimestamp(timestamp int64) *EventBuilder
func (b *EventBuilder) WithCurrentTimestamp() *EventBuilder
func (b *EventBuilder) WithThreadID(threadID string) *EventBuilder
func (b *EventBuilder) WithRunID(runID string) *EventBuilder
func (b *EventBuilder) WithMessageID(messageID string) *EventBuilder
func (b *EventBuilder) WithToolCallID(toolCallID string) *EventBuilder
func (b *EventBuilder) WithStepName(stepName string) *EventBuilder
func (b *EventBuilder) WithRole(role string) *EventBuilder
func (b *EventBuilder) WithDelta(delta string) *EventBuilder
func (b *EventBuilder) WithToolCallName(toolCallName string) *EventBuilder
func (b *EventBuilder) WithParentMessageID(parentMessageID string) *EventBuilder
func (b *EventBuilder) WithErrorMessage(message string) *EventBuilder
func (b *EventBuilder) WithErrorCode(code string) *EventBuilder
func (b *EventBuilder) WithSnapshot(snapshot any) *EventBuilder
func (b *EventBuilder) WithDeltaOperations(ops []JSONPatchOperation) *EventBuilder
func (b *EventBuilder) WithMessages(messages []Message) *EventBuilder
func (b *EventBuilder) WithCustomName(name string) *EventBuilder
func (b *EventBuilder) WithCustomValue(value any) *EventBuilder
func (b *EventBuilder) WithRawEvent(event any) *EventBuilder
func (b *EventBuilder) WithRawSource(source string) *EventBuilder
func (b *EventBuilder) WithAutoGenerateIDs() *EventBuilder

// Helper methods
func (b *EventBuilder) AddDeltaOperation(op, path string, value any) *EventBuilder
func (b *EventBuilder) AddDeltaOperationWithFrom(op, path, from string) *EventBuilder
func (b *EventBuilder) AddMessage(id, role, content string) *EventBuilder

// Build the final event
func (b *EventBuilder) Build() (Event, error)
```

## Specific Event Types

### Run Events

#### RunStartedEvent

```go
type RunStartedEvent struct {
    *BaseEvent
    ThreadID string `json:"threadId"`
    RunID    string `json:"runId"`
}

func NewRunStartedEvent(threadID, runID string) *RunStartedEvent
func NewRunStartedEventWithOptions(threadID, runID string, options ...RunStartedOption) *RunStartedEvent

// Options
type RunStartedOption func(*RunStartedEvent)
func WithAutoRunID() RunStartedOption
func WithAutoThreadID() RunStartedOption
```

#### RunFinishedEvent

```go
type RunFinishedEvent struct {
    *BaseEvent
    ThreadID string `json:"threadId"`
    RunID    string `json:"runId"`
}

func NewRunFinishedEvent(threadID, runID string) *RunFinishedEvent
func NewRunFinishedEventWithOptions(threadID, runID string, options ...RunFinishedOption) *RunFinishedEvent
```

#### RunErrorEvent

```go
type RunErrorEvent struct {
    *BaseEvent
    Code    *string `json:"code,omitempty"`
    Message string  `json:"message"`
    RunID   string  `json:"runId,omitempty"`
}

func NewRunErrorEvent(message string, options ...RunErrorOption) *RunErrorEvent

// Options
type RunErrorOption func(*RunErrorEvent)
func WithErrorCode(code string) RunErrorOption
func WithRunID(runID string) RunErrorOption
func WithAutoRunIDError() RunErrorOption
```

### Step Events

#### StepStartedEvent

```go
type StepStartedEvent struct {
    *BaseEvent
    StepName string `json:"stepName"`
}

func NewStepStartedEvent(stepName string) *StepStartedEvent
func NewStepStartedEventWithOptions(stepName string, options ...StepStartedOption) *StepStartedEvent

// Options
type StepStartedOption func(*StepStartedEvent)
func WithAutoStepName() StepStartedOption
```

#### StepFinishedEvent

```go
type StepFinishedEvent struct {
    *BaseEvent
    StepName string `json:"stepName"`
}

func NewStepFinishedEvent(stepName string) *StepFinishedEvent
func NewStepFinishedEventWithOptions(stepName string, options ...StepFinishedOption) *StepFinishedEvent
```

### Message Events

#### TextMessageStartEvent

```go
type TextMessageStartEvent struct {
    *BaseEvent
    MessageID string  `json:"messageId"`
    Role      *string `json:"role,omitempty"`
}

func NewTextMessageStartEvent(messageID string, options ...TextMessageStartOption) *TextMessageStartEvent

// Options
type TextMessageStartOption func(*TextMessageStartEvent)
func WithMessageRole(role string) TextMessageStartOption
func WithAutoMessageID() TextMessageStartOption
```

#### TextMessageContentEvent

```go
type TextMessageContentEvent struct {
    *BaseEvent
    MessageID string `json:"messageId"`
    Delta     string `json:"delta"`
}

func NewTextMessageContentEvent(messageID, delta string) *TextMessageContentEvent
func NewTextMessageContentEventWithOptions(messageID, delta string, options ...TextMessageContentOption) *TextMessageContentEvent
```

#### TextMessageEndEvent

```go
type TextMessageEndEvent struct {
    *BaseEvent
    MessageID string `json:"messageId"`
}

func NewTextMessageEndEvent(messageID string) *TextMessageEndEvent
func NewTextMessageEndEventWithOptions(messageID string, options ...TextMessageEndOption) *TextMessageEndEvent
```

### Tool Events

#### ToolCallStartEvent

```go
type ToolCallStartEvent struct {
    *BaseEvent
    ToolCallID      string  `json:"toolCallId"`
    ToolCallName    string  `json:"toolCallName"`
    ParentMessageID *string `json:"parentMessageId,omitempty"`
}

func NewToolCallStartEvent(toolCallID, toolCallName string, options ...ToolCallStartOption) *ToolCallStartEvent

// Options
type ToolCallStartOption func(*ToolCallStartEvent)
func WithParentMessage(parentMessageID string) ToolCallStartOption
func WithAutoToolCallID() ToolCallStartOption
```

#### ToolCallArgsEvent

```go
type ToolCallArgsEvent struct {
    *BaseEvent
    ToolCallID string `json:"toolCallId"`
    Delta      string `json:"delta"`
}

func NewToolCallArgsEvent(toolCallID, delta string) *ToolCallArgsEvent
func NewToolCallArgsEventWithOptions(toolCallID, delta string, options ...ToolCallArgsOption) *ToolCallArgsEvent
```

#### ToolCallEndEvent

```go
type ToolCallEndEvent struct {
    *BaseEvent
    ToolCallID string `json:"toolCallId"`
}

func NewToolCallEndEvent(toolCallID string) *ToolCallEndEvent
func NewToolCallEndEventWithOptions(toolCallID string, options ...ToolCallEndOption) *ToolCallEndEvent
```

### State Events

#### StateSnapshotEvent

```go
type StateSnapshotEvent struct {
    *BaseEvent
    Snapshot any `json:"snapshot"`
}

func NewStateSnapshotEvent(snapshot any) *StateSnapshotEvent
func NewStateSnapshotEventWithOptions(snapshot any, options ...StateSnapshotOption) *StateSnapshotEvent
```

#### StateDeltaEvent

```go
type StateDeltaEvent struct {
    *BaseEvent
    Delta []JSONPatchOperation `json:"delta"`
}

type JSONPatchOperation struct {
    Op    string `json:"op"`
    Path  string `json:"path"`
    Value any    `json:"value,omitempty"`
    From  string `json:"from,omitempty"`
}

func NewStateDeltaEvent(delta []JSONPatchOperation) *StateDeltaEvent
func NewStateDeltaEventWithOptions(delta []JSONPatchOperation, options ...StateDeltaOption) *StateDeltaEvent
```

#### MessagesSnapshotEvent

```go
type MessagesSnapshotEvent struct {
    *BaseEvent
    Messages []Message `json:"messages"`
}

type Message struct {
    ID      string  `json:"id"`
    Role    string  `json:"role"`
    Content *string `json:"content,omitempty"`
}

func NewMessagesSnapshotEvent(messages []Message) *MessagesSnapshotEvent
func NewMessagesSnapshotEventWithOptions(messages []Message, options ...MessagesSnapshotOption) *MessagesSnapshotEvent
```

### Generic Events

#### CustomEvent

```go
type CustomEvent struct {
    *BaseEvent
    Name  string `json:"name"`
    Value any    `json:"value"`
}

func NewCustomEvent(name string, value any) *CustomEvent
func NewCustomEventWithOptions(name string, value any, options ...CustomOption) *CustomEvent
```

#### RawEvent

```go
type RawEvent struct {
    *BaseEvent
    Event  any     `json:"event"`
    Source *string `json:"source,omitempty"`
}

func NewRawEvent(event any, options ...RawOption) *RawEvent

// Options
type RawOption func(*RawEvent)
func WithRawEventSource(source string) RawOption
```

## Usage Examples

### Basic Event Creation

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Create a run started event
    runEvent := events.NewRunStartedEvent("thread-123", "run-456")
    
    // Validate the event
    if err := runEvent.Validate(); err != nil {
        log.Fatalf("Invalid event: %v", err)
    }
    
    // Serialize to JSON
    jsonData, err := runEvent.ToJSON()
    if err != nil {
        log.Fatalf("Failed to serialize: %v", err)
    }
    
    fmt.Printf("Event JSON: %s\n", string(jsonData))
    
    // Convert to protobuf
    pbEvent, err := runEvent.ToProtobuf()
    if err != nil {
        log.Fatalf("Failed to convert to protobuf: %v", err)
    }
    
    fmt.Printf("Protobuf event type: %T\n", pbEvent.Event)
}
```

### Using EventBuilder

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Create a complex event using the builder
    event, err := events.NewEventBuilder().
        TextMessageContent().
        WithMessageID("msg-789").
        WithDelta("Hello, world!").
        WithCurrentTimestamp().
        Build()
    
    if err != nil {
        log.Fatalf("Failed to build event: %v", err)
    }
    
    fmt.Printf("Built event: %+v\n", event)
    
    // Create a state delta event with operations
    deltaEvent, err := events.NewEventBuilder().
        StateDelta().
        AddDeltaOperation("replace", "/title", "New Document Title").
        AddDeltaOperation("add", "/tags/-", "new-tag").
        WithAutoGenerateIDs().
        Build()
    
    if err != nil {
        log.Fatalf("Failed to build delta event: %v", err)
    }
    
    fmt.Printf("Delta event: %+v\n", deltaEvent)
}
```

### Auto-ID Generation

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Create event with auto-generated IDs
    event, err := events.NewEventBuilder().
        RunStarted().
        WithAutoGenerateIDs().
        Build()
    
    if err != nil {
        log.Fatalf("Failed to build event: %v", err)
    }
    
    runEvent := event.(*events.RunStartedEvent)
    fmt.Printf("Auto-generated Thread ID: %s\n", runEvent.ThreadID)
    fmt.Printf("Auto-generated Run ID: %s\n", runEvent.RunID)
    
    // Using constructor options for auto-generation
    runEvent2 := events.NewRunStartedEventWithOptions("", "", 
        events.WithAutoThreadID(),
        events.WithAutoRunID(),
    )
    
    fmt.Printf("Option-generated Thread ID: %s\n", runEvent2.ThreadID)
    fmt.Printf("Option-generated Run ID: %s\n", runEvent2.RunID)
}
```

### Complex State Events

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Create a messages snapshot
    messages := []events.Message{
        {
            ID:      "msg-1",
            Role:    "user",
            Content: stringPtr("Hello, how can I help you?"),
        },
        {
            ID:      "msg-2",
            Role:    "assistant",
            Content: stringPtr("I can help you with various tasks."),
        },
    }
    
    snapshotEvent := events.NewMessagesSnapshotEvent(messages)
    
    if err := snapshotEvent.Validate(); err != nil {
        log.Fatalf("Invalid snapshot event: %v", err)
    }
    
    fmt.Printf("Messages snapshot: %+v\n", snapshotEvent)
    
    // Create a state delta with multiple operations
    deltaOps := []events.JSONPatchOperation{
        {
            Op:    "replace",
            Path:  "/status",
            Value: "active",
        },
        {
            Op:    "add",
            Path:  "/metadata/lastUpdated",
            Value: time.Now().Unix(),
        },
        {
            Op:   "move",
            Path: "/data/newLocation",
            From: "/data/oldLocation",
        },
    }
    
    deltaEvent := events.NewStateDeltaEvent(deltaOps)
    
    if err := deltaEvent.Validate(); err != nil {
        log.Fatalf("Invalid delta event: %v", err)
    }
    
    fmt.Printf("State delta: %+v\n", deltaEvent)
}

func stringPtr(s string) *string {
    return &s
}
```

### Custom Events

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Create a custom event for application-specific data
    customData := map[string]interface{}{
        "userId":     "user-123",
        "action":     "file_upload",
        "fileName":   "document.pdf",
        "fileSize":   1024000,
        "uploadTime": time.Now().Unix(),
    }
    
    customEvent := events.NewCustomEvent("file_uploaded", customData)
    
    if err := customEvent.Validate(); err != nil {
        log.Fatalf("Invalid custom event: %v", err)
    }
    
    fmt.Printf("Custom event: %+v\n", customEvent)
    
    // Create a raw event for wrapping external event systems
    externalEvent := struct {
        Type      string `json:"type"`
        Source    string `json:"source"`
        Data      any    `json:"data"`
        Timestamp int64  `json:"timestamp"`
    }{
        Type:      "external.notification",
        Source:    "external-service",
        Data:      map[string]string{"message": "External event occurred"},
        Timestamp: time.Now().Unix(),
    }
    
    rawEvent := events.NewRawEvent(externalEvent, 
        events.WithRawEventSource("external-service"),
    )
    
    if err := rawEvent.Validate(); err != nil {
        log.Fatalf("Invalid raw event: %v", err)
    }
    
    fmt.Printf("Raw event: %+v\n", rawEvent)
}
```

### Event Streaming

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Simulate streaming message content
    messageID := events.GenerateMessageID()
    
    // Start message
    startEvent := events.NewTextMessageStartEvent(messageID, 
        events.WithMessageRole("assistant"),
    )
    
    if err := startEvent.Validate(); err != nil {
        log.Fatalf("Invalid start event: %v", err)
    }
    
    fmt.Printf("Start: %+v\n", startEvent)
    
    // Stream content chunks
    content := "This is a streaming response that will be sent in chunks."
    chunkSize := 10
    
    for i := 0; i < len(content); i += chunkSize {
        end := i + chunkSize
        if end > len(content) {
            end = len(content)
        }
        
        chunk := content[i:end]
        contentEvent := events.NewTextMessageContentEvent(messageID, chunk)
        
        if err := contentEvent.Validate(); err != nil {
            log.Printf("Invalid content event: %v", err)
            continue
        }
        
        fmt.Printf("Content chunk: %+v\n", contentEvent)
        time.Sleep(100 * time.Millisecond) // Simulate streaming delay
    }
    
    // End message
    endEvent := events.NewTextMessageEndEvent(messageID)
    
    if err := endEvent.Validate(); err != nil {
        log.Fatalf("Invalid end event: %v", err)
    }
    
    fmt.Printf("End: %+v\n", endEvent)
}
```

## ID Generation Utilities

### ID Generators

```go
// Generate unique IDs for various event components
func GenerateThreadID() string
func GenerateRunID() string
func GenerateMessageID() string
func GenerateToolCallID() string
func GenerateStepID() string

// Example usage
threadID := events.GenerateThreadID() // Returns "thread_01HQ7..."
runID := events.GenerateRunID()       // Returns "run_01HQ7..."
messageID := events.GenerateMessageID() // Returns "msg_01HQ7..."
```

## Validation

### Event Validation

Each event type implements comprehensive validation:

```go
// Example validation for RunStartedEvent
func (e *RunStartedEvent) Validate() error {
    if err := e.BaseEvent.Validate(); err != nil {
        return err
    }
    
    if e.ThreadID == "" {
        return fmt.Errorf("RunStartedEvent validation failed: threadId field is required")
    }
    
    if e.RunID == "" {
        return fmt.Errorf("RunStartedEvent validation failed: runId field is required")
    }
    
    return nil
}
```

### Custom Validation

```go
// Add custom validation for specific use cases
type ValidatedEvent interface {
    Event
    ValidateForContext(ctx context.Context) error
}

// Example implementation
func (e *CustomEvent) ValidateForContext(ctx context.Context) error {
    if err := e.Validate(); err != nil {
        return err
    }
    
    // Add context-specific validation
    if userID := ctx.Value("userID"); userID == nil {
        return fmt.Errorf("user context required for custom events")
    }
    
    return nil
}
```

## Serialization

### JSON Serialization

All events provide JSON serialization:

```go
event := events.NewRunStartedEvent("thread-123", "run-456")
jsonData, err := event.ToJSON()
if err != nil {
    log.Fatalf("Serialization failed: %v", err)
}

// JSON output:
// {
//   "eventType": "RUN_STARTED",
//   "timestampMs": 1699123456789,
//   "threadId": "thread-123",
//   "runId": "run-456"
// }
```

### Protobuf Serialization

Events can be converted to protobuf for efficient transport:

```go
event := events.NewTextMessageContentEvent("msg-123", "Hello")
pbEvent, err := event.ToProtobuf()
if err != nil {
    log.Fatalf("Protobuf conversion failed: %v", err)
}

// Use pbEvent for efficient serialization
data, err := proto.Marshal(pbEvent)
if err != nil {
    log.Fatalf("Protobuf serialization failed: %v", err)
}
```

## Error Handling

### Event Errors

```go
// Event validation errors
type EventValidationError struct {
    EventType EventType
    Field     string
    Message   string
}

func (e *EventValidationError) Error() string {
    return fmt.Sprintf("validation error for %s.%s: %s", e.EventType, e.Field, e.Message)
}

// Event building errors
type EventBuildError struct {
    Message string
    Cause   error
}

func (e *EventBuildError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("event build error: %s (caused by: %v)", e.Message, e.Cause)
    }
    return fmt.Sprintf("event build error: %s", e.Message)
}
```

### Error Recovery

```go
// Gracefully handle event creation errors
func CreateEventSafely(builder *events.EventBuilder) (events.Event, error) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Panic during event creation: %v", r)
        }
    }()
    
    event, err := builder.Build()
    if err != nil {
        return nil, fmt.Errorf("failed to create event: %w", err)
    }
    
    return event, nil
}
```

## Best Practices

### Event Design

1. **Use specific event types**: Prefer specific event types over generic custom events
2. **Include relevant context**: Add thread IDs, run IDs, and other contextual information
3. **Validate early**: Always validate events before sending
4. **Use auto-generation**: Leverage automatic ID generation to prevent conflicts

### Performance

1. **Batch similar events**: Group related events for efficient processing
2. **Use protobuf for high-throughput**: Prefer protobuf serialization for performance-critical applications
3. **Stream large content**: Use content streaming for large messages

### Error Handling

1. **Handle validation errors**: Always check validation results
2. **Log event errors**: Implement comprehensive logging for debugging
3. **Provide fallbacks**: Handle event creation failures gracefully

### Security

1. **Validate event sources**: Verify event authenticity in distributed systems
2. **Sanitize event data**: Clean user-provided data in custom events
3. **Use secure serialization**: Validate deserialized event data

## Migration Guide

### From Untyped Events

```go
// Old approach with maps
eventData := map[string]interface{}{
    "eventType": "RUN_STARTED",
    "threadId":  "thread-123",
    "runId":     "run-456",
    "timestamp": time.Now().UnixMilli(),
}

// New approach with type safety
event := events.NewRunStartedEvent("thread-123", "run-456")
```

### From Manual JSON Construction

```go
// Old approach with manual JSON
jsonStr := `{
    "eventType": "TEXT_MESSAGE_CONTENT",
    "messageId": "msg-123",
    "delta": "Hello",
    "timestampMs": ` + fmt.Sprintf("%d", time.Now().UnixMilli()) + `
}`

// New approach with type safety
event := events.NewTextMessageContentEvent("msg-123", "Hello")
jsonData, _ := event.ToJSON()
```

This comprehensive API reference provides all the information needed to effectively use the type-safe event system in the AG-UI Go SDK.