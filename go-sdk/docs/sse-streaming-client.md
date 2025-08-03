# Server-Sent Events (SSE) Streaming Client

## Overview

The SSE Streaming Client provides a robust, RFC-compliant implementation for consuming Server-Sent Events from HTTP servers. It includes comprehensive features for production use including automatic reconnection, backpressure handling, event ordering, and integration with the AG-UI event system.

## Features

### Core Functionality
- ✅ **RFC-compliant SSE parsing** - Fully compliant with the Server-Sent Events specification
- ✅ **Event parsing and validation** - Proper parsing of SSE event fields (id, event, data, retry)
- ✅ **Automatic reconnection** - Exponential backoff with configurable parameters
- ✅ **Event ordering and sequence tracking** - Maintains event sequence and ordering
- ✅ **Connection health monitoring** - Detects and handles connection issues
- ✅ **Context integration** - Full support for Go context cancellation

### Advanced Features
- ✅ **Backpressure handling** - Flow control for high-frequency event streams
- ✅ **Event buffer management** - Configurable buffering with overflow protection
- ✅ **TLS/SSL support** - Secure connections with configurable TLS settings
- ✅ **Event filtering** - Client-side filtering of event types
- ✅ **Custom headers** - Support for authentication and custom headers
- ✅ **Callback system** - Configurable callbacks for connection events

### Integration
- ✅ **AG-UI Event conversion** - Seamless conversion between SSE and AG-UI events
- ✅ **Error handling** - Integration with the AG-UI error handling system
- ✅ **Concurrent access** - Thread-safe operations for concurrent usage

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
)

func main() {
    // Configure the SSE client
    config := client.SSEClientConfig{
        URL: "https://api.example.com/events",
        Headers: map[string]string{
            "Authorization": "Bearer your-token",
        },
    }

    // Create the client
    sseClient, err := client.NewSSEClient(config)
    if err != nil {
        log.Fatal(err)
    }
    defer sseClient.Close()

    // Connect to the stream
    ctx := context.Background()
    if err := sseClient.Connect(ctx); err != nil {
        log.Fatal(err)
    }

    // Process events
    for event := range sseClient.Events() {
        fmt.Printf("Received: %s - %s\n", event.Event, event.Data)
    }
}
```

### Advanced Configuration

```go
config := client.SSEClientConfig{
    URL: "https://api.example.com/events",
    
    // Reconnection settings
    InitialBackoff:       time.Second,
    MaxBackoff:          30 * time.Second,
    BackoffMultiplier:   2.0,
    MaxReconnectAttempts: 0, // Unlimited
    
    // Buffer and flow control
    EventBufferSize:      1000,
    FlowControlEnabled:   true,
    FlowControlThreshold: 0.8,
    
    // Timeouts
    ReadTimeout:         30 * time.Second,
    HealthCheckInterval: 30 * time.Second,
    
    // Event filtering
    EventFilter: func(eventType string) bool {
        return eventType == "message" || eventType == "notification"
    },
    
    // Callbacks
    OnConnect: func() {
        fmt.Println("Connected to stream")
    },
    OnDisconnect: func(err error) {
        fmt.Printf("Disconnected: %v\n", err)
    },
    OnReconnect: func(attempt int) {
        fmt.Printf("Reconnecting (attempt %d)\n", attempt)
    },
}
```

## Configuration Options

### Connection Settings

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `URL` | `string` | **required** | SSE endpoint URL |
| `Headers` | `map[string]string` | `{}` | Custom HTTP headers |
| `TLSConfig` | `*tls.Config` | `nil` | TLS configuration |
| `SkipTLSVerify` | `bool` | `false` | Skip TLS certificate verification |
| `UserAgent` | `string` | `"ag-ui-go-sdk-sse/1.0.0"` | User-Agent header |

### Reconnection Settings

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `InitialBackoff` | `time.Duration` | `1s` | Initial backoff duration |
| `MaxBackoff` | `time.Duration` | `30s` | Maximum backoff duration |
| `BackoffMultiplier` | `float64` | `2.0` | Exponential backoff multiplier |
| `MaxReconnectAttempts` | `int` | `0` | Max reconnect attempts (0 = unlimited) |

### Event Handling

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `EventBufferSize` | `int` | `1000` | Event buffer size |
| `FlowControlEnabled` | `bool` | `false` | Enable backpressure handling |
| `FlowControlThreshold` | `float64` | `0.8` | Buffer threshold for flow control |
| `EventFilter` | `func(string) bool` | `nil` | Event type filter function |

### Timeouts and Health

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ReadTimeout` | `time.Duration` | `0` | Read timeout (0 = no timeout) |
| `WriteTimeout` | `time.Duration` | `10s` | Write timeout |
| `HealthCheckInterval` | `time.Duration` | `30s` | Health check interval |

### Callbacks

| Field | Type | Description |
|-------|------|-------------|
| `OnConnect` | `func()` | Called when connection is established |
| `OnDisconnect` | `func(error)` | Called when connection is lost |
| `OnReconnect` | `func(int)` | Called when reconnection starts |
| `OnError` | `func(error)` | Called when an error occurs |

## Event Structure

### SSE Event

```go
type SSEEvent struct {
    ID        string            // Event ID
    Event     string            // Event type
    Data      string            // Event data
    Retry     *time.Duration    // Retry interval
    Raw       string            // Raw event text
    Headers   map[string]string // Custom headers
    Timestamp time.Time         // Reception timestamp
    Sequence  uint64            // Sequence number
}
```

### Event Conversion

Convert SSE events to AG-UI events:

```go
sseEvent := &client.SSEEvent{
    Event: "TEXT_MESSAGE_CONTENT",
    Data:  `{"message": "hello"}`,
}

agEvent, err := client.ConvertSSEToEvent(sseEvent)
if err != nil {
    log.Fatal(err)
}

// Process AG-UI event
fmt.Printf("Event type: %s\n", agEvent.Type())
```

Convert AG-UI events to SSE events:

```go
agEvent := events.NewBaseEvent(events.EventTypeTextMessageStart)
agEvent.RawEvent = map[string]interface{}{
    "message": "Hello world",
}

sseEvent, err := client.ConvertEventToSSE(agEvent)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("SSE data: %s\n", sseEvent.Data)
```

## Connection States

The client maintains connection state through the `SSEConnectionState` enum:

- `SSEStateDisconnected` - Not connected
- `SSEStateConnecting` - Connection in progress
- `SSEStateConnected` - Successfully connected
- `SSEStateReconnecting` - Reconnection in progress
- `SSEStateClosed` - Client has been closed

Monitor connection state:

```go
fmt.Printf("Current state: %s\n", sseClient.State())
fmt.Printf("Reconnect count: %d\n", sseClient.ReconnectCount())
fmt.Printf("Last event ID: %s\n", sseClient.LastEventID())
```

## Error Handling

The SSE client integrates with the AG-UI error handling system:

```go
config := client.SSEClientConfig{
    URL: "https://api.example.com/events",
    OnError: func(err error) {
        // Handle errors based on type
        switch {
        case errors.Is(err, context.Canceled):
            log.Println("Connection canceled")
        case errors.Is(err, io.EOF):
            log.Println("Stream ended")
        default:
            log.Printf("SSE error: %v", err)
        }
    },
}
```

## Flow Control and Backpressure

For high-frequency event streams, enable flow control to prevent memory issues:

```go
config := client.SSEClientConfig{
    URL:                   "https://api.example.com/events",
    FlowControlEnabled:    true,
    FlowControlThreshold:  0.8,  // Activate backpressure at 80% buffer capacity
    EventBufferSize:       10000, // Larger buffer for high-frequency streams
}

sseClient, _ := client.NewSSEClient(config)

// Monitor backpressure status
if sseClient.IsBackpressureActive() {
    fmt.Println("Backpressure is active - processing slower than reception")
}
```

## Resumable Connections

Use event IDs to resume connections from the last received event:

```go
// First connection
config := client.SSEClientConfig{
    URL: "https://api.example.com/events",
}

sseClient, _ := client.NewSSEClient(config)
sseClient.Connect(ctx)

// Process some events...
lastEventID := sseClient.LastEventID()

// Later, resume from last event
config.LastEventID = lastEventID
resumeClient, _ := client.NewSSEClient(config)
resumeClient.Connect(ctx) // Will send "Last-Event-ID" header
```

## Security Considerations

### TLS Configuration

```go
config := client.SSEClientConfig{
    URL: "https://api.example.com/events",
    TLSConfig: &tls.Config{
        InsecureSkipVerify: false, // Always validate certificates in production
        MinVersion:         tls.VersionTLS12,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
        },
    },
}
```

### Authentication

```go
config := client.SSEClientConfig{
    URL: "https://api.example.com/events",
    Headers: map[string]string{
        "Authorization": "Bearer " + authToken,
        "X-API-Key":     apiKey,
    },
}
```

## Performance Considerations

### High-Frequency Streams

For streams with high event frequency:

1. **Enable flow control**: Prevents memory exhaustion
2. **Use larger buffers**: Increase `EventBufferSize`
3. **Filter events**: Use `EventFilter` to process only relevant events
4. **Optimize processing**: Process events in batches

```go
config := client.SSEClientConfig{
    URL:                  "https://api.example.com/events",
    EventBufferSize:      10000,
    FlowControlEnabled:   true,
    FlowControlThreshold: 0.9,
    EventFilter: func(eventType string) bool {
        // Only process critical events
        return eventType == "alert" || eventType == "critical"
    },
}
```

### Memory Management

The client automatically manages memory through:

- **Buffer rotation**: Removes oldest events when buffer is full
- **Backpressure**: Slows down reception when processing is behind
- **Resource cleanup**: Properly closes connections and releases resources

## Testing

### Unit Tests

Run the comprehensive test suite:

```bash
go test ./pkg/client -v -run TestSSEClient
```

### Integration Testing

Test with a real SSE server:

```go
func TestWithRealServer(t *testing.T) {
    config := client.SSEClientConfig{
        URL: "https://your-sse-endpoint.com/events",
        Headers: map[string]string{
            "Authorization": "Bearer test-token",
        },
    }

    sseClient, err := client.NewSSEClient(config)
    if err != nil {
        t.Fatal(err)
    }
    defer sseClient.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := sseClient.Connect(ctx); err != nil {
        t.Fatal(err)
    }

    // Test event reception...
}
```

## Example Applications

### Real-time Chat Application

```go
func chatApplication() {
    config := client.SSEClientConfig{
        URL: "https://chat-api.example.com/events",
        EventFilter: func(eventType string) bool {
            return eventType == "message" || eventType == "user_joined" || eventType == "user_left"
        },
    }

    sseClient, _ := client.NewSSEClient(config)
    defer sseClient.Close()

    sseClient.Connect(context.Background())

    for event := range sseClient.Events() {
        switch event.Event {
        case "message":
            handleChatMessage(event.Data)
        case "user_joined":
            handleUserJoined(event.Data)
        case "user_left":
            handleUserLeft(event.Data)
        }
    }
}
```

### Monitoring Dashboard

```go
func monitoringDashboard() {
    config := client.SSEClientConfig{
        URL: "https://monitoring-api.example.com/events",
        FlowControlEnabled: true,
        EventFilter: func(eventType string) bool {
            return eventType == "metric" || eventType == "alert"
        },
    }

    sseClient, _ := client.NewSSEClient(config)
    defer sseClient.Close()

    sseClient.Connect(context.Background())

    for event := range sseClient.Events() {
        if event.Event == "metric" {
            updateDashboard(event.Data)
        } else if event.Event == "alert" {
            triggerAlert(event.Data)
        }
    }
}
```

## Best Practices

1. **Always handle errors**: Implement proper error handling for production use
2. **Use contexts**: Always provide contexts for cancellation support
3. **Monitor connection health**: Use callbacks to track connection status
4. **Enable flow control**: For high-frequency streams to prevent memory issues
5. **Filter events**: Process only relevant events to improve performance
6. **Secure connections**: Use proper TLS configuration and authentication
7. **Test thoroughly**: Test with various network conditions and server behaviors
8. **Log appropriately**: Use structured logging for debugging and monitoring

## Troubleshooting

### Common Issues

1. **Connection timeouts**: Increase health check interval or read timeout
2. **Memory issues**: Enable flow control and reduce buffer size
3. **Authentication failures**: Verify headers and token validity
4. **TLS errors**: Check certificate validity and TLS configuration
5. **Event parsing errors**: Verify server sends RFC-compliant SSE format

### Debug Logging

Enable debug logging to troubleshoot issues:

```go
config := client.SSEClientConfig{
    URL: "https://api.example.com/events",
    OnConnect: func() {
        log.Println("DEBUG: Connected to SSE stream")
    },
    OnDisconnect: func(err error) {
        log.Printf("DEBUG: Disconnected: %v", err)
    },
    OnError: func(err error) {
        log.Printf("DEBUG: Error: %v", err)
    },
}
```

## Conclusion

The SSE Streaming Client provides a production-ready, feature-complete solution for consuming Server-Sent Events in Go applications. It handles all the complexities of SSE connections while providing a clean, intuitive API that integrates seamlessly with the AG-UI event system.