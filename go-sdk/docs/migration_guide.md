# Migration Guide: AG-UI Go SDK Transport Abstraction and Type-Safe Events

## Overview

This migration guide helps you transition from legacy transport and event handling to the new type-safe interfaces introduced in the AG-UI Go SDK. The new system provides better type safety, comprehensive validation, and improved error handling while maintaining backward compatibility where possible.

## Table of Contents

1. [Transport Abstraction Migration](#transport-abstraction-migration)
2. [Type-Safe Event System Migration](#type-safe-event-system-migration)
3. [Breaking Changes](#breaking-changes)
4. [Backward Compatibility](#backward-compatibility)
5. [Step-by-Step Migration](#step-by-step-migration)
6. [Best Practices](#best-practices)
7. [Troubleshooting](#troubleshooting)

## Transport Abstraction Migration

### From Legacy Transport to New Interface

#### Old Approach (Legacy)

```go
// Legacy transport configuration
type OldTransportConfig struct {
    URL      string
    Type     string
    Options  map[string]interface{}
    Features map[string]interface{}
}

// Legacy transport usage
transport := &OldTransport{
    Config: OldTransportConfig{
        URL:  "ws://localhost:8080",
        Type: "websocket",
        Options: map[string]interface{}{
            "timeout": 30,
            "secure":  true,
        },
        Features: map[string]interface{}{
            "compression": true,
            "streaming":   true,
        },
    },
}

// Legacy event sending
event := map[string]interface{}{
    "type":      "message",
    "content":   "Hello world",
    "timestamp": time.Now().Unix(),
}
transport.Send(event)
```

#### New Approach (Type-Safe)

```go
import (
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/transport"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// New type-safe configuration
config := &transport.BasicConfig{
    Type:     "websocket",
    Endpoint: "ws://localhost:8080",
    Timeout:  30 * time.Second,
    Secure:   true,
    Headers: map[string]string{
        "Authorization": "Bearer token",
    },
}

// Type-safe capabilities with validation
compressionFeatures := transport.CompressionFeatures{
    SupportedAlgorithms: []transport.CompressionType{
        transport.CompressionGzip,
    },
    DefaultAlgorithm: transport.CompressionGzip,
}

capabilities := transport.NewCompressionCapabilities(
    transport.Capabilities{
        Streaming:     true,
        Bidirectional: true,
        Compression:   compressionFeatures.SupportedAlgorithms,
    },
    compressionFeatures,
)

// Create transport with validated configuration
newTransport := transport.NewTransportWithCapabilities(config, capabilities)

// Type-safe event creation and sending
event := events.NewTextMessageContentEvent("msg-123", "Hello world")
if err := event.Validate(); err != nil {
    log.Fatalf("Invalid event: %v", err)
}

ctx := context.Background()
if err := newTransport.Send(ctx, event); err != nil {
    log.Printf("Failed to send event: %v", err)
}
```

### Migration Steps for Transport

1. **Replace configuration structures**:
   ```go
   // Before
   config := OldTransportConfig{
       URL:  "ws://localhost:8080",
       Type: "websocket",
   }
   
   // After
   config := &transport.BasicConfig{
       Type:     "websocket",
       Endpoint: "ws://localhost:8080",
       Timeout:  30 * time.Second,
   }
   ```

2. **Update feature configuration**:
   ```go
   // Before
   features := map[string]interface{}{
       "compression": true,
       "security":    "tls",
   }
   
   // After
   compressionFeatures := transport.CompressionFeatures{
       SupportedAlgorithms: []transport.CompressionType{
           transport.CompressionGzip,
       },
       DefaultAlgorithm: transport.CompressionGzip,
   }
   
   securityFeatures := transport.SecurityFeatures{
       SupportedFeatures: []transport.SecurityFeature{
           transport.SecurityTLS,
       },
       DefaultFeature: transport.SecurityTLS,
   }
   ```

3. **Update transport creation**:
   ```go
   // Before
   transport := NewOldTransport(config)
   
   // After
   transport := transport.NewWebSocketTransport(config)
   // or with capabilities
   transport := transport.NewTransportWithCapabilities(config, capabilities)
   ```

4. **Update connection handling**:
   ```go
   // Before
   if err := transport.Connect(); err != nil {
       log.Fatal(err)
   }
   
   // After
   ctx := context.Background()
   if err := transport.Connect(ctx); err != nil {
       log.Fatal(err)
   }
   defer transport.Close(ctx)
   ```

## Type-Safe Event System Migration

### From Map-Based Events to Type-Safe Events

#### Old Approach (Map-Based)

```go
// Legacy event creation
event := map[string]interface{}{
    "eventType":   "RUN_STARTED",
    "threadId":    "thread-123",
    "runId":       "run-456",
    "timestampMs": time.Now().UnixMilli(),
}

// Legacy validation (manual)
if event["threadId"] == "" {
    return errors.New("threadId is required")
}

// Legacy serialization
jsonData, _ := json.Marshal(event)
```

#### New Approach (Type-Safe)

```go
// Type-safe event creation
event := events.NewRunStartedEvent("thread-123", "run-456")

// Automatic validation
if err := event.Validate(); err != nil {
    return fmt.Errorf("invalid event: %w", err)
}

// Type-safe serialization
jsonData, err := event.ToJSON()
if err != nil {
    return fmt.Errorf("serialization failed: %w", err)
}

// Protobuf support
pbEvent, err := event.ToProtobuf()
if err != nil {
    return fmt.Errorf("protobuf conversion failed: %w", err)
}
```

### Migration Steps for Events

1. **Replace map-based event creation**:
   ```go
   // Before
   runEvent := map[string]interface{}{
       "eventType": "RUN_STARTED",
       "threadId":  "thread-123",
       "runId":     "run-456",
   }
   
   // After
   runEvent := events.NewRunStartedEvent("thread-123", "run-456")
   ```

2. **Update message events**:
   ```go
   // Before
   messageEvent := map[string]interface{}{
       "eventType": "TEXT_MESSAGE_CONTENT",
       "messageId": "msg-123",
       "delta":     "Hello world",
   }
   
   // After
   messageEvent := events.NewTextMessageContentEvent("msg-123", "Hello world")
   ```

3. **Replace tool events**:
   ```go
   // Before
   toolEvent := map[string]interface{}{
       "eventType":    "TOOL_CALL_START",
       "toolCallId":   "tool-123",
       "toolCallName": "calculator",
   }
   
   // After
   toolEvent := events.NewToolCallStartEvent("tool-123", "calculator")
   ```

4. **Update state events**:
   ```go
   // Before
   stateEvent := map[string]interface{}{
       "eventType": "STATE_DELTA",
       "delta": []map[string]interface{}{
           {
               "op":    "replace",
               "path":  "/status",
               "value": "active",
           },
       },
   }
   
   // After
   stateEvent := events.NewStateDeltaEvent([]events.JSONPatchOperation{
       {
           Op:    "replace",
           Path:  "/status",
           Value: "active",
       },
   })
   ```

### Using the Event Builder

For complex event creation, use the new EventBuilder:

```go
// Before (complex manual construction)
event := map[string]interface{}{
    "eventType":   "TEXT_MESSAGE_CONTENT",
    "messageId":   generateMessageID(),
    "delta":       "Content chunk",
    "timestampMs": time.Now().UnixMilli(),
}

// After (using builder)
event, err := events.NewEventBuilder().
    TextMessageContent().
    WithMessageID(events.GenerateMessageID()).
    WithDelta("Content chunk").
    WithCurrentTimestamp().
    Build()

if err != nil {
    log.Fatalf("Failed to build event: %v", err)
}
```

## Breaking Changes

### Transport Interface Changes

1. **Method Signatures**:
   - `Connect()` → `Connect(ctx context.Context)`
   - `Send(event interface{})` → `Send(ctx context.Context, event TransportEvent)`
   - `Close()` → `Close(ctx context.Context)`

2. **Configuration Structure**:
   - `Config` map replaced with structured `Config` interface
   - Features moved to type-safe feature sets

3. **Event Types**:
   - Events must implement `TransportEvent` interface
   - Map-based events no longer accepted directly

### Event System Changes

1. **Event Creation**:
   - Map-based events deprecated in favor of typed events
   - Manual timestamp setting replaced with automatic generation

2. **Validation**:
   - Manual validation replaced with automatic validation
   - Validation errors now provide structured information

3. **Serialization**:
   - Direct JSON marshaling replaced with `ToJSON()` method
   - Added protobuf support via `ToProtobuf()` method

## Backward Compatibility

### Compatibility Helpers

The SDK provides compatibility helpers for gradual migration:

```go
// Convert legacy capabilities to new format
legacyCapabilities := transport.Capabilities{
    Streaming:     true,
    Bidirectional: true,
    Features: map[string]interface{}{
        "compression": true,
    },
}

// Convert to type-safe format
typedCapabilities := transport.ToTypedCapabilities(legacyCapabilities)

// Convert back if needed
backToLegacy := transport.ToCapabilities(typedCapabilities)
```

### Legacy Event Support

```go
// Wrap legacy events in RawEvent for transport
legacyEvent := map[string]interface{}{
    "type": "legacy_event",
    "data": "some data",
}

rawEvent := events.NewRawEvent(legacyEvent,
    events.WithRawEventSource("legacy-system"),
)
```

## Step-by-Step Migration

### Phase 1: Configuration Migration

1. Update transport configuration structures
2. Replace feature maps with type-safe feature sets
3. Add proper validation and error handling

### Phase 2: Transport Interface Migration

1. Update method calls to include context
2. Replace direct event sending with type-safe events
3. Update connection and error handling

### Phase 3: Event System Migration

1. Replace map-based events with typed events
2. Update event creation to use constructors or builders
3. Add proper event validation

### Phase 4: Advanced Features

1. Implement streaming transport if needed
2. Add reliability features (acknowledgments, retries)
3. Configure middleware and monitoring

### Phase 5: Optimization

1. Use protobuf serialization for performance
2. Implement batching for high-throughput scenarios
3. Add comprehensive monitoring and metrics

## Best Practices

### Migration Best Practices

1. **Gradual Migration**:
   ```go
   // Start with compatibility wrappers
   func migrateGradually() {
       // Use legacy format initially
       legacyEvent := map[string]interface{}{
           "type": "message",
           "content": "data",
       }
       
       // Wrap in new format
       wrappedEvent := events.NewRawEvent(legacyEvent)
       
       // Eventually replace with typed events
       typedEvent := events.NewTextMessageContentEvent("msg-123", "data")
   }
   ```

2. **Validation Strategy**:
   ```go
   // Always validate after creation
   func createValidatedEvent() error {
       event := events.NewRunStartedEvent("thread-123", "run-456")
       
       if err := event.Validate(); err != nil {
           return fmt.Errorf("event validation failed: %w", err)
       }
       
       return nil
   }
   ```

3. **Error Handling**:
   ```go
   // Implement comprehensive error handling
   func handleTransportErrors(transport transport.Transport) {
       go func() {
           for err := range transport.Errors() {
               switch {
               case errors.Is(err, transport.ErrConnectionLost):
                   // Handle connection loss
                   log.Printf("Connection lost: %v", err)
               case errors.Is(err, transport.ErrSendTimeout):
                   // Handle send timeout
                   log.Printf("Send timeout: %v", err)
               default:
                   log.Printf("Transport error: %v", err)
               }
           }
       }()
   }
   ```

### Code Organization

1. **Separate concerns**:
   ```go
   // events/factory.go
   package events
   
   func CreateRunLifecycleEvents(threadID, runID string) []events.Event {
       return []events.Event{
           events.NewRunStartedEvent(threadID, runID),
           // ... other events
       }
   }
   
   // transport/manager.go
   package transport
   
   func SetupTransportManager() *transport.TransportManager {
       manager := transport.NewTransportManager()
       // ... configuration
       return manager
   }
   ```

2. **Use dependency injection**:
   ```go
   type EventService struct {
       transport transport.Transport
   }
   
   func NewEventService(t transport.Transport) *EventService {
       return &EventService{transport: t}
   }
   
   func (s *EventService) SendEvent(ctx context.Context, event events.Event) error {
       if err := event.Validate(); err != nil {
           return err
       }
       return s.transport.Send(ctx, event)
   }
   ```

## Troubleshooting

### Common Issues and Solutions

1. **Validation Errors**:
   ```go
   // Problem: Event validation fails
   event := events.NewRunStartedEvent("", "run-123") // Empty threadID
   err := event.Validate() // Returns validation error
   
   // Solution: Use auto-generation or provide valid values
   event := events.NewRunStartedEventWithOptions("", "run-123",
       events.WithAutoThreadID(),
   )
   ```

2. **Context Cancellation**:
   ```go
   // Problem: Operations timeout
   ctx := context.Background()
   err := transport.Send(ctx, event) // May timeout
   
   // Solution: Use appropriate timeout
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   err := transport.Send(ctx, event)
   ```

3. **Feature Configuration**:
   ```go
   // Problem: Feature validation fails
   features := transport.CompressionFeatures{
       DefaultAlgorithm: transport.CompressionZstd, // Not in supported list
       SupportedAlgorithms: []transport.CompressionType{
           transport.CompressionGzip,
       },
   }
   
   // Solution: Ensure consistency
   features := transport.CompressionFeatures{
       SupportedAlgorithms: []transport.CompressionType{
           transport.CompressionGzip,
           transport.CompressionZstd,
       },
       DefaultAlgorithm: transport.CompressionZstd, // Now valid
   }
   ```

4. **Event Builder Issues**:
   ```go
   // Problem: Builder fails without event type
   event, err := events.NewEventBuilder().
       WithMessageID("msg-123").
       Build() // Fails - no event type set
   
   // Solution: Always set event type first
   event, err := events.NewEventBuilder().
       TextMessageContent(). // Set event type first
       WithMessageID("msg-123").
       WithDelta("content").
       Build()
   ```

### Performance Considerations

1. **Event Creation Performance**:
   ```go
   // For high-throughput scenarios, reuse builders
   builder := events.NewEventBuilder()
   
   for i := 0; i < 1000; i++ {
       event, err := builder.
           TextMessageContent().
           WithMessageID(fmt.Sprintf("msg-%d", i)).
           WithDelta(fmt.Sprintf("content-%d", i)).
           Build()
       
       if err != nil {
           log.Printf("Failed to build event %d: %v", i, err)
           continue
       }
       
       // Send event
   }
   ```

2. **Batch Processing**:
   ```go
   // Use streaming transport for batching
   streamingTransport := transport.NewStreamingTransport(config)
   
   events := []transport.TransportEvent{
       events.NewRunStartedEvent("thread-123", "run-456"),
       events.NewStepStartedEvent("step-1"),
       events.NewStepFinishedEvent("step-1"),
   }
   
   err := streamingTransport.SendBatch(ctx, events)
   ```

### Debugging Tips

1. **Enable detailed logging**:
   ```go
   // Configure transport with logging middleware
   logger := log.New(os.Stdout, "[TRANSPORT] ", log.LstdFlags)
   loggingMiddleware := transport.NewLoggingMiddleware(logger)
   
   wrappedTransport := loggingMiddleware.Wrap(baseTransport)
   ```

2. **Monitor statistics**:
   ```go
   // Regularly check transport statistics
   go func() {
       ticker := time.NewTicker(30 * time.Second)
       defer ticker.Stop()
       
       for range ticker.C {
           stats := transport.Stats()
           log.Printf("Transport stats: %+v", stats)
       }
   }()
   ```

3. **Validate configuration**:
   ```go
   // Always validate configuration before use
   config := &transport.BasicConfig{
       Type:     "websocket",
       Endpoint: "ws://localhost:8080",
       Timeout:  30 * time.Second,
   }
   
   if err := config.Validate(); err != nil {
       log.Fatalf("Invalid configuration: %v", err)
   }
   ```

This migration guide provides a comprehensive path from legacy implementations to the new type-safe transport abstraction and event system. Follow the phases gradually and use the troubleshooting section to resolve common issues during migration.