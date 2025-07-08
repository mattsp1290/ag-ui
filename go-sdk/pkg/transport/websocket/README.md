# WebSocket Transport for AG-UI Go SDK

The WebSocket transport provides RFC 6455 compliant WebSocket communication for the AG-UI Go SDK. It offers high-performance, bidirectional streaming with comprehensive message support, connection pooling, and advanced features like compression, security, and error recovery.

## Features

- **RFC 6455 Compliance**: Full WebSocket protocol implementation using gorilla/websocket
- **Message Integration**: Native support for `messages.Message` interface types
- **Event System**: Integration with the core event system
- **Connection Pooling**: Advanced connection management with load balancing
- **Streaming Support**: Bidirectional message and event streaming
- **Multiple Serialization**: JSON and Protocol Buffer message support
- **Compression**: Per-message compression with configurable levels
- **Security**: TLS support, rate limiting, and message size constraints
- **Error Recovery**: Automatic reconnection with exponential backoff
- **Performance**: Optimized for high-throughput applications
- **Monitoring**: Comprehensive metrics and health checking

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/ag-ui/go-sdk/pkg/transport/websocket"
    "github.com/ag-ui/go-sdk/pkg/messages"
)

func main() {
    // Create transport configuration
    config := websocket.DefaultTransportConfig()
    config.URLs = []string{"ws://localhost:8080/ws"}
    
    // Create and start transport
    transport, err := websocket.NewTransport(config)
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    if err := transport.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer transport.Stop()
    
    // Create message integration
    messageIntegration := websocket.NewMessageIntegration(transport, websocket.FormatJSON)
    defer messageIntegration.Close()
    
    // Send a message
    msg := &messages.UserMessage{
        BaseMessage: messages.BaseMessage{
            Role: messages.RoleUser,
        },
        Content: stringPtr("Hello, WebSocket!"),
    }
    
    if err := messageIntegration.SendMessage(ctx, msg); err != nil {
        log.Fatal(err)
    }
}

func stringPtr(s string) *string { return &s }
```

### With Message Handlers

```go
// Set up message handling
messageIntegration.SetMessageHandler(websocket.MessageHandlerFunc(func(ctx context.Context, message messages.Message) error {
    fmt.Printf("Received message from %s: %v\n", message.GetRole(), message.GetContent())
    return nil
}))

// Set up event handling
messageIntegration.SetEventHandler(websocket.CoreEventHandlerFunc(func(ctx context.Context, event core.Event[any]) error {
    fmt.Printf("Received event: %s\n", event.Type())
    return nil
}))
```

## Configuration

### Basic Configuration

```go
config := websocket.DefaultTransportConfig()
config.URLs = []string{"ws://localhost:8080/ws"}
config.EventTimeout = 30 * time.Second
config.MaxEventSize = 1024 * 1024 // 1MB
```

### Connection Pool Configuration

```go
poolConfig := websocket.DefaultPoolConfig()
poolConfig.URLs = []string{
    "ws://server1:8080/ws",
    "ws://server2:8080/ws",
    "ws://server3:8080/ws",
}
poolConfig.MinConnections = 2
poolConfig.MaxConnections = 10
poolConfig.MaxRetries = 5
poolConfig.RetryInterval = 2 * time.Second
poolConfig.HealthCheckInterval = 30 * time.Second

config := websocket.DefaultTransportConfig()
config.PoolConfig = poolConfig
```

### Advanced Configuration

```go
poolConfig := &websocket.PoolConfig{
    URLs:                []string{"wss://secure-server:443/ws"},
    MinConnections:      3,
    MaxConnections:      15,
    ConnectionTimeout:   10 * time.Second,
    MaxRetries:          10,
    RetryInterval:       1 * time.Second,
    HealthCheckInterval: 20 * time.Second,
    IdleTimeout:         60 * time.Second,
    MaxIdleTime:         300 * time.Second,
    
    // TLS Configuration
    TLSConfig: &websocket.TLSConfig{
        InsecureSkipVerify: false,
        CertFile:          "/path/to/client.crt",
        KeyFile:           "/path/to/client.key",
        CAFile:            "/path/to/ca.crt",
    },
    
    // Compression Configuration
    CompressionConfig: &websocket.CompressionConfig{
        Enabled:              true,
        CompressionLevel:     6,
        CompressionThreshold: 1024,
        MaxCompressionRatio:  0.8,
        MaxMemoryUsage:       64 * 1024 * 1024, // 64MB
    },
    
    // Security Configuration
    SecurityConfig: &websocket.SecurityConfig{
        EnableRateLimiting:   true,
        RateLimitRequests:    100,
        RateLimitWindow:      time.Minute,
        MaxMessageSize:       5 * 1024 * 1024, // 5MB
        EnableMessageBuffer:  true,
        MessageBufferSize:    1000,
        EnableDDoSProtection: true,
        MaxConnectionsPerIP:  10,
    },
}
```

## Message Formats

### JSON Messages (Default)

```go
messageIntegration := websocket.NewMessageIntegration(transport, websocket.FormatJSON)

// All message types are supported
userMsg := &messages.UserMessage{...}
assistantMsg := &messages.AssistantMessage{...}
systemMsg := &messages.SystemMessage{...}
toolMsg := &messages.ToolMessage{...}
devMsg := &messages.DeveloperMessage{...}
```

### Protocol Buffer Messages

```go
messageIntegration := websocket.NewMessageIntegration(transport, websocket.FormatProtobuf)

// Messages must implement proto.Message interface
// Custom protobuf definitions required
```

## Streaming

### Message Streaming

```go
// Get message stream
messagesChan, err := messageIntegration.ReceiveMessages(ctx)
if err != nil {
    log.Fatal(err)
}

// Create stream wrapper
messageStream := websocket.NewMessageStreamWrapper(messagesChan, ctx)
defer messageStream.Close()

// Process streaming messages
for {
    event, err := messageStream.Next(ctx)
    if err != nil {
        log.Printf("Stream error: %v", err)
        break
    }
    
    if event == nil {
        break // Stream ended
    }
    
    fmt.Printf("Stream message: %v\n", event.Message)
}
```

### Event Streaming

```go
// Get event stream
eventsChan, err := messageIntegration.ReceiveEvents(ctx)
if err != nil {
    log.Fatal(err)
}

// Process streaming events
for event := range eventsChan {
    fmt.Printf("Event: %s - %v\n", event.Type(), event.Data())
}
```

## Error Handling

### Automatic Reconnection

```go
poolConfig := websocket.DefaultPoolConfig()
poolConfig.MaxRetries = 10
poolConfig.RetryInterval = 2 * time.Second
poolConfig.HealthCheckInterval = 30 * time.Second

// Transport will automatically reconnect on connection failures
```

### Error Recovery in Handlers

```go
messageIntegration.SetMessageHandler(websocket.MessageHandlerFunc(func(ctx context.Context, message messages.Message) error {
    // Handle message processing errors gracefully
    if err := processMessage(message); err != nil {
        log.Printf("Failed to process message: %v", err)
        return err // Will be logged by the transport
    }
    return nil
}))
```

### Circuit Breaker Pattern

```go
// The transport includes built-in circuit breaker functionality
// Configure thresholds in SecurityConfig
securityConfig.EnableCircuitBreaker = true
securityConfig.CircuitBreakerThreshold = 10
securityConfig.CircuitBreakerTimeout = 30 * time.Second
```

## Performance Optimization

### High-Throughput Configuration

```go
poolConfig := websocket.DefaultPoolConfig()
poolConfig.MinConnections = 10        // Pre-allocate connections
poolConfig.MaxConnections = 50        // Allow high concurrency
poolConfig.MessageBufferSize = 10000  // Large message buffer
poolConfig.HealthCheckInterval = 60 * time.Second // Reduce overhead

// Fast compression for large messages
poolConfig.CompressionConfig = &websocket.CompressionConfig{
    Enabled:              true,
    CompressionLevel:     1, // Fast compression
    CompressionThreshold: 1024,
}
```

### Memory Management

```go
// Configure memory limits
poolConfig.CompressionConfig.MaxMemoryUsage = 128 * 1024 * 1024 // 128MB
poolConfig.SecurityConfig.MaxMessageSize = 10 * 1024 * 1024     // 10MB per message
poolConfig.SecurityConfig.MessageBufferSize = 5000              // Limit buffered messages
```

## Monitoring and Metrics

### Transport Statistics

```go
stats := transport.GetStats()
fmt.Printf("Events sent: %d\n", stats.EventsSent)
fmt.Printf("Events received: %d\n", stats.EventsReceived)
fmt.Printf("Bytes transferred: %d\n", stats.BytesTransferred)
fmt.Printf("Average latency: %v\n", stats.AverageLatency)
```

### Connection Pool Health

```go
poolStats := transport.pool.GetStats()
fmt.Printf("Active connections: %d\n", poolStats.ActiveConnections)
fmt.Printf("Total connections: %d\n", poolStats.TotalConnections)
fmt.Printf("Failed connections: %d\n", poolStats.FailedConnections)
```

### Health Checks

```go
if transport.pool.IsHealthy() {
    fmt.Println("Transport is healthy")
} else {
    fmt.Println("Transport has issues")
}

// Get detailed health status
healthStatus := transport.pool.GetHealthStatus()
for url, status := range healthStatus {
    fmt.Printf("Server %s: %v\n", url, status.Healthy)
}
```

## Security

### TLS Configuration

```go
tlsConfig := &websocket.TLSConfig{
    InsecureSkipVerify: false,
    CertFile:          "/path/to/client.crt",
    KeyFile:           "/path/to/client.key",
    CAFile:            "/path/to/ca.crt",
    ServerName:        "secure-server.example.com",
}

poolConfig.TLSConfig = tlsConfig
```

### Rate Limiting

```go
securityConfig := &websocket.SecurityConfig{
    EnableRateLimiting: true,
    RateLimitRequests:  100,              // 100 requests
    RateLimitWindow:    time.Minute,      // per minute
    MaxMessageSize:     1024 * 1024,      // 1MB max message
    MaxConnectionsPerIP: 5,               // Per IP limit
}

poolConfig.SecurityConfig = securityConfig
```

### DDoS Protection

```go
securityConfig.EnableDDoSProtection = true
securityConfig.DDoSDetectionThreshold = 1000    // requests per window
securityConfig.DDoSBlockDuration = 10 * time.Minute
```

## Best Practices

### Connection Management

1. **Pre-allocate Connections**: Set `MinConnections` to your expected base load
2. **Reasonable Limits**: Don't set `MaxConnections` too high to avoid resource exhaustion
3. **Health Checks**: Enable regular health checks for early problem detection
4. **Graceful Shutdown**: Always call `transport.Stop()` and `messageIntegration.Close()`

### Message Handling

1. **Handle Errors Gracefully**: Always return appropriate errors from handlers
2. **Avoid Blocking**: Keep message handlers fast and non-blocking
3. **Use Channels**: Prefer channel-based communication for async processing
4. **Size Limits**: Configure appropriate message size limits

### Performance

1. **Buffer Sizes**: Tune buffer sizes based on your message volume
2. **Compression**: Enable compression for large messages
3. **Connection Pooling**: Use multiple connections for high throughput
4. **Monitoring**: Monitor transport statistics regularly

### Security

1. **TLS**: Always use TLS in production (`wss://` URLs)
2. **Rate Limiting**: Configure appropriate rate limits
3. **Message Validation**: Validate message content and size
4. **Authentication**: Implement proper authentication mechanisms

## Troubleshooting

### Common Issues

1. **Connection Failures**: Check network connectivity and server availability
2. **High Latency**: Monitor network conditions and server performance
3. **Memory Usage**: Check message sizes and buffer configurations
4. **Rate Limiting**: Verify rate limit settings match your usage patterns

### Debug Logging

```go
import "go.uber.org/zap"

// Enable debug logging
logger, _ := zap.NewDevelopment()
config.Logger = logger

// Transport will log detailed information about operations
```

### Metrics Collection

```go
// Implement custom metrics collection
type CustomMetricsCollector struct{}

func (c *CustomMetricsCollector) RecordEvent(eventType string, size int64, latency time.Duration) {
    // Send metrics to your monitoring system
}

// Set custom metrics collector
transport.SetMetricsCollector(&CustomMetricsCollector{})
```

## Integration Examples

See the `examples.go` file for comprehensive examples including:

- Basic usage patterns
- Connection pooling
- Streaming messages
- Error handling
- Advanced configuration
- Performance optimization

## API Reference

### Core Types

- `Transport`: Main WebSocket transport implementation
- `MessageIntegration`: Message layer integration
- `TransportConfig`: Transport configuration
- `PoolConfig`: Connection pool configuration
- `MessageFormat`: Message serialization format

### Interfaces

- `MessageHandler`: Handles incoming messages
- `CoreEventHandler`: Handles incoming events
- `MessageStream`: Streaming message interface

For complete API documentation, see the generated Go documentation.