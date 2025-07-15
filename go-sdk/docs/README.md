# AG-UI Go SDK Documentation

## Overview

The AG-UI Go SDK provides a comprehensive, type-safe framework for building AI agent applications with real-time communication capabilities. This documentation covers the major components introduced and enhanced in the latest release.

## 🚀 New Features

### Transport Abstraction Layer

A robust, type-safe transport system that enables reliable bidirectional communication between agents and front-end applications.

**Key Features:**
- Type-safe transport interfaces with comprehensive validation
- Support for multiple protocols (WebSocket, HTTP, gRPC)
- Streaming and reliable transport capabilities
- Advanced capabilities system with type-safe feature configuration
- Circuit breaker patterns and automatic reconnection
- Transport manager with load balancing and failover

### Type-Safe Event System

A comprehensive event system that ensures compile-time type safety and automatic validation for all AG-UI protocol events.

**Key Features:**
- Complete AG-UI protocol implementation with 16 event types
- Fluent builder pattern for complex event construction
- Automatic ID generation and validation
- JSON and Protocol Buffer serialization
- Streaming message support with content chunking

## 📚 Documentation

### API References

- **[Transport Abstraction API](transport_abstraction_api.md)** - Complete API reference for the transport layer
- **[Type-Safe Event System API](event_system_api.md)** - Comprehensive guide to the event system

### Migration Guide

- **[Migration Guide](migration_guide.md)** - Step-by-step guide for migrating from legacy implementations

### Examples

- **[Transport Basic Usage](../examples/transport/basic_usage/)** - Basic transport operations and configuration
- **[Type-Safe Events Usage](../examples/events/type_safe_usage/)** - Event creation, validation, and serialization
- **[Collaborative Editing](../examples/state/collaborative_editing/)** - Real-time state synchronization example

## 🏗️ Architecture

### Transport Layer Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Transport Manager                        │
├─────────────────────────────────────────────────────────────┤
│  Load Balancer  │  Health Checker  │  Circuit Breaker      │
├─────────────────────────────────────────────────────────────┤
│              Middleware Chain                               │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │   Logging   │  Metrics    │    Auth     │ Compression │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                 Transport Interface                         │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │  WebSocket  │    HTTP     │    gRPC     │    Mock     │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Event System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Event Builder                           │
├─────────────────────────────────────────────────────────────┤
│               Type-Safe Event Types                        │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │     Run     │   Message   │    Tool     │    State    │  │
│  │   Events    │   Events    │   Events    │   Events    │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
├─────────────────────────────────────────────────────────────┤
│              Validation & Serialization                    │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │ Validation  │    JSON     │  Protobuf   │ ID Utils    │  │
│  │   Rules     │ Serializer  │ Serializer  │             │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## 🚦 Quick Start

### Basic Transport Usage

```go
import (
    "context"
    "github.com/ag-ui/go-sdk/pkg/transport"
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

// Create and configure transport
config := &transport.BasicConfig{
    Type:     "websocket",
    Endpoint: "ws://localhost:8080/ws",
    Timeout:  30 * time.Second,
}

transport := transport.NewWebSocketTransport(config)

// Connect and send events
ctx := context.Background()
if err := transport.Connect(ctx); err != nil {
    log.Fatal(err)
}
defer transport.Close(ctx)

event := events.NewRunStartedEvent("thread-123", "run-456")
transport.Send(ctx, event)
```

### Type-Safe Event Creation

```go
import "github.com/ag-ui/go-sdk/pkg/core/events"

// Using constructors
event := events.NewTextMessageContentEvent("msg-123", "Hello, world!")

// Using builder pattern
event, err := events.NewEventBuilder().
    TextMessageContent().
    WithMessageID("msg-456").
    WithDelta("Built with fluent interface").
    Build()

// Auto-generation
event, err := events.NewEventBuilder().
    RunStarted().
    WithAutoGenerateIDs().
    Build()
```

## 🔧 Configuration

### Transport Capabilities

```go
// Compression configuration
compressionFeatures := transport.CompressionFeatures{
    SupportedAlgorithms: []transport.CompressionType{
        transport.CompressionGzip,
        transport.CompressionZstd,
    },
    DefaultAlgorithm: transport.CompressionGzip,
}

// Security configuration
securityFeatures := transport.SecurityFeatures{
    SupportedFeatures: []transport.SecurityFeature{
        transport.SecurityTLS,
        transport.SecurityJWT,
    },
    DefaultFeature: transport.SecurityTLS,
}

// Create typed capabilities
capabilities := transport.NewCompressionCapabilities(baseCapabilities, compressionFeatures)
```

### Event Validation

```go
// Create events with automatic validation
event := events.NewRunStartedEvent("thread-123", "run-456")
if err := event.Validate(); err != nil {
    log.Printf("Validation error: %v", err)
}

// Use builder with validation
event, err := events.NewEventBuilder().
    StateDelta().
    AddDeltaOperation("replace", "/status", "active").
    Build() // Automatically validates
```

## 🔄 Migration

### From Legacy Transport

```go
// Before (legacy)
transport := &OldTransport{
    URL: "ws://localhost:8080",
    Features: map[string]interface{}{
        "compression": true,
    },
}

// After (type-safe)
config := &transport.BasicConfig{
    Type:     "websocket",
    Endpoint: "ws://localhost:8080",
    Timeout:  30 * time.Second,
}

compressionFeatures := transport.CompressionFeatures{
    SupportedAlgorithms: []transport.CompressionType{
        transport.CompressionGzip,
    },
    DefaultAlgorithm: transport.CompressionGzip,
}

capabilities := transport.NewCompressionCapabilities(
    transport.Capabilities{Streaming: true},
    compressionFeatures,
)

newTransport := transport.NewTransportWithCapabilities(config, capabilities)
```

### From Map-Based Events

```go
// Before (map-based)
event := map[string]interface{}{
    "eventType": "RUN_STARTED",
    "threadId":  "thread-123",
    "runId":     "run-456",
}

// After (type-safe)
event := events.NewRunStartedEvent("thread-123", "run-456")
```

## 🧪 Testing

### Testing Transport

```go
func TestTransportBasics(t *testing.T) {
    config := &transport.MockConfig{
        Type:     "mock",
        Endpoint: "mock://test",
        Timeout:  5 * time.Second,
    }
    
    transport := transport.NewMockTransport(config)
    
    ctx := context.Background()
    assert.NoError(t, transport.Connect(ctx))
    defer transport.Close(ctx)
    
    event := events.NewRunStartedEvent("test-thread", "test-run")
    assert.NoError(t, transport.Send(ctx, event))
}
```

### Testing Events

```go
func TestEventCreation(t *testing.T) {
    event := events.NewRunStartedEvent("thread-123", "run-456")
    
    // Test validation
    assert.NoError(t, event.Validate())
    
    // Test serialization
    jsonData, err := event.ToJSON()
    assert.NoError(t, err)
    assert.Contains(t, string(jsonData), "RUN_STARTED")
    
    // Test protobuf
    pbEvent, err := event.ToProtobuf()
    assert.NoError(t, err)
    assert.NotNil(t, pbEvent)
}
```

## 📈 Performance

### Optimized Event Creation

```go
// For high-frequency scenarios, reuse builders
builder := events.NewEventBuilder()

for i := 0; i < 1000; i++ {
    event, err := builder.
        TextMessageContent().
        WithMessageID(fmt.Sprintf("msg-%d", i)).
        WithDelta(fmt.Sprintf("content-%d", i)).
        Build()
    
    if err != nil {
        continue
    }
    
    // Process event
}
```

### Batch Processing

```go
// Use streaming transport for batching
streamingTransport := transport.NewStreamingTransport(config)

events := []transport.TransportEvent{
    events.NewStepStartedEvent("step-1"),
    events.NewStepFinishedEvent("step-1"),
}

err := streamingTransport.SendBatch(ctx, events)
```

## 🔐 Security

### TLS Configuration

```go
tlsConfig := &transport.TLSConfig{
    MinVersion:        "1.2",
    MaxVersion:        "1.3",
    RequireClientCert: true,
}

securityFeatures := transport.SecurityFeatures{
    SupportedFeatures: []transport.SecurityFeature{
        transport.SecurityTLS,
    },
    TLSConfig: tlsConfig,
}
```

### Authentication

```go
config := &transport.BasicConfig{
    Type:     "websocket",
    Endpoint: "wss://secure.example.com/ws",
    Headers: map[string]string{
        "Authorization": "Bearer " + token,
    },
    Secure: true,
}
```

## 🐛 Error Handling

### Transport Errors

```go
// Handle transport errors
go func() {
    for err := range transport.Errors() {
        switch {
        case errors.Is(err, transport.ErrConnectionLost):
            log.Printf("Connection lost: %v", err)
            // Implement reconnection logic
        case errors.Is(err, transport.ErrSendTimeout):
            log.Printf("Send timeout: %v", err)
            // Handle timeout
        default:
            log.Printf("Transport error: %v", err)
        }
    }
}()
```

### Event Validation Errors

```go
event := events.NewRunStartedEvent("", "run-456") // Missing threadID
if err := event.Validate(); err != nil {
    var validationErr *events.ValidationError
    if errors.As(err, &validationErr) {
        log.Printf("Field: %s, Error: %s", validationErr.Field, validationErr.Message)
    }
}
```

## 🔍 Monitoring

### Transport Statistics

```go
// Monitor transport performance
stats := transport.Stats()
log.Printf("Events sent: %d", stats.EventsSent)
log.Printf("Events received: %d", stats.EventsReceived)
log.Printf("Average latency: %v", stats.AverageLatency)
log.Printf("Error count: %d", stats.ErrorCount)
```

### Health Checking

```go
// Implement health checking
if healthChecker, ok := transport.(transport.HealthChecker); ok {
    if err := healthChecker.CheckHealth(ctx); err != nil {
        log.Printf("Health check failed: %v", err)
    }
    
    status := healthChecker.GetHealthStatus()
    log.Printf("Health: %t, Latency: %v", status.Healthy, status.Latency)
}
```

## 📝 Best Practices

### Transport Usage

1. **Always use context with timeouts** for transport operations
2. **Implement proper error handling** with specific error type checking
3. **Use appropriate capabilities** for your use case
4. **Monitor transport statistics** for performance insights
5. **Implement graceful shutdown** with proper resource cleanup

### Event Creation

1. **Use specific event types** instead of generic custom events
2. **Include contextual information** (thread IDs, run IDs, etc.)
3. **Validate events before sending** to catch errors early
4. **Use auto-generation for unique IDs** to prevent conflicts
5. **Batch related events** for better performance

### Error Recovery

1. **Implement circuit breakers** for fault tolerance
2. **Use exponential backoff** for reconnection
3. **Log comprehensive error information** for debugging
4. **Provide fallback mechanisms** for critical operations

## 🤝 Contributing

See the main project documentation for contribution guidelines. When working with transport or event system:

1. Always add tests for new transport implementations
2. Ensure events follow AG-UI protocol specifications
3. Update documentation for API changes
4. Follow type safety principles throughout

## 📄 License

This project is licensed under the MIT License - see the main project documentation for details.

---

For more detailed information, see the specific API documentation files and examples provided in this repository.