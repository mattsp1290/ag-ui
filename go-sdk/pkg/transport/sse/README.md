# SSE Transport for AG-UI

This package provides a comprehensive Server-Sent Events (SSE) transport implementation for the AG-UI protocol.

## Features

- **RFC-compliant SSE implementation** - Follows Server-Sent Events specification
- **Bidirectional communication** - Send events via HTTP POST, receive via SSE stream
- **Automatic reconnection** - Handles connection drops with exponential backoff
- **Event validation** - Type-safe parsing and validation of all AG-UI event types
- **Thread-safe operations** - Safe for concurrent use
- **Comprehensive error handling** - Detailed error reporting with typed errors
- **Batch operations** - Send multiple events efficiently
- **Connection monitoring** - Health checks and statistics
- **Configurable timeouts** - Customizable read/write timeouts
- **Security support** - Custom headers for authentication

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/transport/sse"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Create transport
    transport, err := sse.NewSSETransport(nil) // Uses defaults
    if err != nil {
        log.Fatal(err)
    }
    defer transport.Close()

    ctx := context.Background()
    
    // Send an event
    event := events.NewRunStartedEvent("thread-123", "run-456")
    err = transport.Send(ctx, event)
    if err != nil {
        log.Printf("Send failed: %v", err)
    }

    // Receive events
    eventChan, err := transport.Receive(ctx)
    if err != nil {
        log.Printf("Receive failed: %v", err)
    }

    for event := range eventChan {
        log.Printf("Received: %s", event.Type())
    }
}
```

## Configuration

```go
config := &sse.Config{
    BaseURL:        "https://api.example.com",
    Headers: map[string]string{
        "Authorization": "Bearer token",
    },
    BufferSize:     1000,
    ReadTimeout:    30 * time.Second,
    WriteTimeout:   10 * time.Second,
    ReconnectDelay: 5 * time.Second,
    MaxReconnects:  3,
}

transport, err := sse.NewSSETransport(config)
```

## Supported Event Types

The transport supports all AG-UI protocol events:

- **Run Events**: `RUN_STARTED`, `RUN_FINISHED`, `RUN_ERROR`
- **Step Events**: `STEP_STARTED`, `STEP_FINISHED`
- **Message Events**: `TEXT_MESSAGE_START`, `TEXT_MESSAGE_CONTENT`, `TEXT_MESSAGE_END`
- **Tool Events**: `TOOL_CALL_START`, `TOOL_CALL_ARGS`, `TOOL_CALL_END`
- **State Events**: `STATE_SNAPSHOT`, `STATE_DELTA`, `MESSAGES_SNAPSHOT`
- **Custom Events**: `RAW`, `CUSTOM`

## Error Handling

```go
err := transport.Send(ctx, event)
if err != nil {
    if messages.IsValidationError(err) {
        log.Printf("Event validation failed: %v", err)
    } else if messages.IsStreamingError(err) {
        log.Printf("Transport error: %v", err)
    }
}
```

## Batch Operations

```go
events := []events.Event{
    events.NewRunStartedEvent("thread-123", "run-456"),
    events.NewStepStartedEvent("step-1"),
    events.NewStepFinishedEvent("step-1"),
    events.NewRunFinishedEvent("thread-123", "run-456"),
}

err := transport.SendBatch(ctx, events)
```

## Connection Management

```go
// Check connection status
status := transport.GetConnectionStatus()
fmt.Printf("Status: %s\n", status)

// Test connectivity
err := transport.Ping(ctx)
if err != nil {
    log.Printf("Server unreachable: %v", err)
}

// Get statistics
stats := transport.GetStats()
fmt.Printf("Reconnects: %d\n", stats.ReconnectCount)
```

## Server Implementation

For server-side SSE implementations:

```go
// Format event as SSE
sseData, err := sse.FormatSSEEvent(event)
if err != nil {
    log.Printf("Format failed: %v", err)
}

// Write to HTTP response
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")

err = sse.WriteSSEEvent(w, event)
```

## Testing

```bash
# Run all tests
go test ./pkg/transport/sse/...

# Run specific tests
go test ./pkg/transport/sse/... -run TestSSETransport_ParseEvents

# Run benchmarks
go test ./pkg/transport/sse/... -bench=.
```

## API Endpoints

The transport expects these server endpoints:

- `POST /events` - Send individual events
- `POST /events/batch` - Send batch of events
- `GET /events/stream` - SSE event stream
- `GET /ping` - Health check

## Thread Safety

All transport methods are thread-safe and can be called from multiple goroutines:

```go
// Safe concurrent usage
go transport.Send(ctx, event1)
go transport.Send(ctx, event2)
```

## Performance

The transport is optimized for high-throughput scenarios:

- Efficient JSON parsing with minimal allocations
- Configurable buffer sizes for memory management
- Connection pooling and reuse
- Batch operations for bulk data transfer
- Streaming parser for low-latency event processing

## Limitations

- SSE is unidirectional (server to client) for event streaming
- Events are sent to server via separate HTTP POST requests
- Requires HTTP/1.1 or HTTP/2 for optimal performance
- Browser EventSource API limitations apply for web clients

## Dependencies

- `github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events` - Event types and validation
- `github.com/mattsp1290/ag-ui/go-sdk/pkg/messages` - Error types
- Standard library packages: `net/http`, `context`, `encoding/json`, etc.