# Transport Abstraction API Reference

## Overview

The AG-UI Go SDK provides a comprehensive transport abstraction layer that enables reliable, bidirectional communication between agents and front-end applications. The transport layer supports multiple protocols, connection management, error handling, and advanced features like streaming, compression, and security.

## Core Interfaces

### Transport Interface

The main transport interface provides the foundation for all transport implementations:

```go
type Transport interface {
    // Connect establishes a connection to the remote endpoint
    Connect(ctx context.Context) error
    
    // Send sends an event to the remote endpoint
    Send(ctx context.Context, event TransportEvent) error
    
    // Receive returns a channel for receiving events
    Receive() <-chan events.Event
    
    // Errors returns a channel for receiving transport errors
    Errors() <-chan error
    
    // Close closes the transport and releases resources
    Close(ctx context.Context) error
    
    // IsConnected returns true if the transport is connected
    IsConnected() bool
    
    // Config returns the transport's configuration
    Config() Config
    
    // Stats returns transport statistics
    Stats() TransportStats
}
```

### Extended Interfaces

#### StreamingTransport

For real-time bidirectional communication:

```go
type StreamingTransport interface {
    Transport
    
    // StartStreaming begins streaming events in both directions
    StartStreaming(ctx context.Context) (
        send chan<- TransportEvent, 
        receive <-chan events.Event, 
        errors <-chan error, 
        err error
    )
    
    // SendBatch sends multiple events efficiently
    SendBatch(ctx context.Context, events []TransportEvent) error
    
    // SetEventHandler sets a callback for received events
    SetEventHandler(handler EventHandler)
    
    // GetStreamingStats returns streaming-specific statistics
    GetStreamingStats() StreamingStats
}
```

#### ReliableTransport

For guaranteed delivery and acknowledgments:

```go
type ReliableTransport interface {
    Transport
    
    // SendEventWithAck sends an event and waits for acknowledgment
    SendEventWithAck(ctx context.Context, event TransportEvent, timeout time.Duration) error
    
    // SetAckHandler sets a callback for handling acknowledgments
    SetAckHandler(handler AckHandler)
    
    // GetReliabilityStats returns reliability-specific statistics
    GetReliabilityStats() ReliabilityStats
}
```

## Type-Safe Capabilities

### Capabilities Configuration

The transport layer provides type-safe capability configuration using generics:

```go
// Basic capabilities
type Capabilities struct {
    Streaming        bool
    Bidirectional    bool
    Compression      []CompressionType
    Multiplexing     bool
    Reconnection     bool
    MaxMessageSize   int64
    Security         []SecurityFeature
    ProtocolVersion  string
    Features         map[string]interface{}
}

// Type-safe capabilities with feature validation
type TypedCapabilities[T FeatureSet] struct {
    Streaming        bool
    Bidirectional    bool
    Compression      []CompressionType
    Multiplexing     bool
    Reconnection     bool
    MaxMessageSize   int64
    Security         []SecurityFeature
    ProtocolVersion  string
    Features         T  // Type-safe features
}
```

### Feature Sets

#### CompressionFeatures

```go
type CompressionFeatures struct {
    SupportedAlgorithms  []CompressionType
    DefaultAlgorithm     CompressionType
    CompressionLevel     int
    MinSizeThreshold     int64
    MaxCompressionRatio  float64
}

func (cf CompressionFeatures) Validate() error {
    // Validates compression configuration
}
```

#### SecurityFeatures

```go
type SecurityFeatures struct {
    SupportedFeatures []SecurityFeature
    DefaultFeature    SecurityFeature
    TLSConfig        *TLSConfig
    JWTConfig        *JWTConfig
    APIKeyConfig     *APIKeyConfig
    OAuth2Config     *OAuth2Config
}

func (sf SecurityFeatures) Validate() error {
    // Validates security configuration
}
```

#### StreamingFeatures

```go
type StreamingFeatures struct {
    MaxConcurrentStreams  int
    StreamTimeout         time.Duration
    BufferSize           int
    FlowControlEnabled   bool
    WindowSize           int
    KeepAliveInterval    time.Duration
    CompressionPerStream bool
}

func (sf StreamingFeatures) Validate() error {
    // Validates streaming configuration
}
```

## Usage Examples

### Basic Transport Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/transport"
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Create a basic transport configuration
    config := &BasicConfig{
        Type:     "websocket",
        Endpoint: "ws://localhost:8080/ws",
        Timeout:  30 * time.Second,
        Headers: map[string]string{
            "Authorization": "Bearer token123",
        },
    }
    
    // Create transport instance
    t := NewWebSocketTransport(config)
    
    // Connect
    ctx := context.Background()
    if err := t.Connect(ctx); err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer t.Close(ctx)
    
    // Send an event
    event := events.NewRunStartedEvent("thread-123", "run-456")
    if err := t.Send(ctx, event); err != nil {
        log.Printf("Failed to send event: %v", err)
    }
    
    // Listen for events
    go func() {
        for event := range t.Receive() {
            fmt.Printf("Received event: %+v\n", event)
        }
    }()
    
    // Listen for errors
    go func() {
        for err := range t.Errors() {
            log.Printf("Transport error: %v", err)
        }
    }()
    
    // Keep running
    time.Sleep(10 * time.Second)
}
```

### Type-Safe Capabilities Example

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/ag-ui/go-sdk/pkg/transport"
)

func main() {
    // Create compression-focused capabilities
    compressionFeatures := transport.CompressionFeatures{
        SupportedAlgorithms: []transport.CompressionType{
            transport.CompressionGzip,
            transport.CompressionZstd,
        },
        DefaultAlgorithm:     transport.CompressionGzip,
        CompressionLevel:     6,
        MinSizeThreshold:     1024,
        MaxCompressionRatio:  0.9,
    }
    
    // Validate features
    if err := compressionFeatures.Validate(); err != nil {
        log.Fatalf("Invalid compression features: %v", err)
    }
    
    // Create typed capabilities
    capabilities := transport.NewCompressionCapabilities(
        transport.Capabilities{
            Streaming:       true,
            Bidirectional:   true,
            Compression:     compressionFeatures.SupportedAlgorithms,
            Multiplexing:    true,
            MaxMessageSize:  1024 * 1024, // 1MB
            ProtocolVersion: "1.0",
        },
        compressionFeatures,
    )
    
    // Validate the complete configuration
    if err := transport.ValidateCapabilities(capabilities); err != nil {
        log.Fatalf("Invalid capabilities: %v", err)
    }
    
    fmt.Printf("Capabilities configured successfully: %+v\n", capabilities)
}
```

### Streaming Transport Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/transport"
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Create streaming transport
    config := &StreamingConfig{
        BasicConfig: BasicConfig{
            Type:     "grpc",
            Endpoint: "localhost:9090",
            Timeout:  30 * time.Second,
        },
        MaxConcurrentStreams: 10,
        BufferSize:          1024,
        KeepAliveInterval:   30 * time.Second,
    }
    
    streamingTransport := NewGRPCStreamingTransport(config)
    
    ctx := context.Background()
    if err := streamingTransport.Connect(ctx); err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer streamingTransport.Close(ctx)
    
    // Start streaming
    send, receive, errors, err := streamingTransport.StartStreaming(ctx)
    if err != nil {
        log.Fatalf("Failed to start streaming: %v", err)
    }
    
    // Send events
    go func() {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                event := events.NewTextMessageContentEvent("msg-123", "Hello from streaming transport")
                select {
                case send <- event:
                    fmt.Println("Sent streaming event")
                case <-ctx.Done():
                    return
                }
            case <-ctx.Done():
                return
            }
        }
    }()
    
    // Receive events
    go func() {
        for {
            select {
            case event := <-receive:
                fmt.Printf("Received streaming event: %+v\n", event)
            case <-ctx.Done():
                return
            }
        }
    }()
    
    // Handle errors
    go func() {
        for {
            select {
            case err := <-errors:
                log.Printf("Streaming error: %v", err)
            case <-ctx.Done():
                return
            }
        }
    }()
    
    // Batch sending example
    batchEvents := []transport.TransportEvent{
        events.NewStepStartedEvent("step-1"),
        events.NewStepStartedEvent("step-2"),
        events.NewStepFinishedEvent("step-1"),
    }
    
    if err := streamingTransport.SendBatch(ctx, batchEvents); err != nil {
        log.Printf("Failed to send batch: %v", err)
    }
    
    time.Sleep(30 * time.Second)
}
```

### Transport Manager Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/transport"
)

func main() {
    // Create transport manager
    manager := transport.NewTransportManager()
    
    // Add multiple transports for redundancy
    wsTransport := NewWebSocketTransport(&BasicConfig{
        Type:     "websocket",
        Endpoint: "ws://primary.example.com/ws",
        Timeout:  30 * time.Second,
    })
    
    grpcTransport := NewGRPCTransport(&BasicConfig{
        Type:     "grpc",
        Endpoint: "backup.example.com:9090",
        Timeout:  30 * time.Second,
    })
    
    if err := manager.AddTransport("primary", wsTransport); err != nil {
        log.Fatalf("Failed to add primary transport: %v", err)
    }
    
    if err := manager.AddTransport("backup", grpcTransport); err != nil {
        log.Fatalf("Failed to add backup transport: %v", err)
    }
    
    // Set load balancer (round-robin, failover, etc.)
    manager.SetLoadBalancer(transport.NewFailoverLoadBalancer())
    
    ctx := context.Background()
    
    // Send event using best available transport
    event := events.NewRunStartedEvent("thread-123", "run-456")
    if err := manager.SendEvent(ctx, event); err != nil {
        log.Printf("Failed to send event: %v", err)
    }
    
    // Receive events from all transports
    eventChan, err := manager.ReceiveEvents(ctx)
    if err != nil {
        log.Fatalf("Failed to start receiving: %v", err)
    }
    
    go func() {
        for event := range eventChan {
            fmt.Printf("Received event from any transport: %+v\n", event)
        }
    }()
    
    // Show statistics from all transports
    stats := manager.GetStats()
    for name, stat := range stats {
        fmt.Printf("Transport %s stats: %+v\n", name, stat)
    }
    
    defer manager.Close()
    time.Sleep(30 * time.Second)
}
```

## Configuration

### Transport Configuration

```go
type Config interface {
    Validate() error
    Clone() Config
    GetType() string
    GetEndpoint() string
    GetTimeout() time.Duration
    GetHeaders() map[string]string
    IsSecure() bool
}

// Basic implementation
type BasicConfig struct {
    Type     string
    Endpoint string
    Timeout  time.Duration
    Headers  map[string]string
    Secure   bool
}
```

### Reconnection Strategy

```go
type ReconnectStrategy struct {
    MaxAttempts       int           // 0 for infinite
    InitialDelay      time.Duration
    MaxDelay          time.Duration
    BackoffMultiplier float64
    Jitter            bool
}

// Example usage
strategy := &transport.ReconnectStrategy{
    MaxAttempts:       5,
    InitialDelay:      1 * time.Second,
    MaxDelay:          30 * time.Second,
    BackoffMultiplier: 2.0,
    Jitter:           true,
}
```

## Error Handling

### Transport Errors

```go
// Connection state tracking
type ConnectionState int

const (
    StateDisconnected ConnectionState = iota
    StateConnecting
    StateConnected
    StateReconnecting
    StateClosing
    StateClosed
    StateError
)

// Error handling callback
type ConnectionCallback func(state ConnectionState, err error)
```

### Error Recovery

```go
// Circuit breaker for fault tolerance
type CircuitBreaker interface {
    Execute(ctx context.Context, operation func() error) error
    IsOpen() bool
    Reset()
    GetState() CircuitBreakerState
}

// Retry policy for failed operations
type RetryPolicy interface {
    ShouldRetry(attempt int, err error) bool
    NextDelay(attempt int) time.Duration
    MaxAttempts() int
    Reset()
}
```

## Middleware and Interceptors

### Middleware Interface

```go
type Middleware interface {
    ProcessOutgoing(ctx context.Context, event TransportEvent) (TransportEvent, error)
    ProcessIncoming(ctx context.Context, event events.Event) (events.Event, error)
    Name() string
    Wrap(transport Transport) Transport
}

// Middleware chain for multiple processors
type MiddlewareChain interface {
    Add(middleware Middleware)
    ProcessOutgoing(ctx context.Context, event TransportEvent) (TransportEvent, error)
    ProcessIncoming(ctx context.Context, event events.Event) (events.Event, error)
    Clear()
}
```

### Example Middleware

```go
// Logging middleware
type LoggingMiddleware struct {
    logger *log.Logger
}

func (m *LoggingMiddleware) ProcessOutgoing(ctx context.Context, event TransportEvent) (TransportEvent, error) {
    m.logger.Printf("Outgoing event: %s", event.Type())
    return event, nil
}

func (m *LoggingMiddleware) ProcessIncoming(ctx context.Context, event events.Event) (events.Event, error) {
    m.logger.Printf("Incoming event: %s", event.GetEventType())
    return event, nil
}

func (m *LoggingMiddleware) Name() string {
    return "logging"
}
```

## Statistics and Monitoring

### Transport Statistics

```go
type TransportStats struct {
    ConnectedAt      time.Time
    ReconnectCount   int
    LastError        error
    Uptime           time.Duration
    EventsSent       int64
    EventsReceived   int64
    BytesSent        int64
    BytesReceived    int64
    AverageLatency   time.Duration
    ErrorCount       int64
    LastEventSentAt  time.Time
    LastEventRecvAt  time.Time
}

// Streaming-specific statistics
type StreamingStats struct {
    TransportStats
    StreamsActive          int
    StreamsTotal          int
    BufferUtilization     float64
    BackpressureEvents    int64
    DroppedEvents         int64
    AverageEventSize      int64
    ThroughputEventsPerSec float64
    ThroughputBytesPerSec  float64
}

// Reliability-specific statistics
type ReliabilityStats struct {
    TransportStats
    EventsAcknowledged   int64
    EventsUnacknowledged int64
    EventsRetried        int64
    EventsTimedOut       int64
    AverageAckTime       time.Duration
    DuplicateEvents      int64
    OutOfOrderEvents     int64
    RedeliveryRate       float64
}
```

### Health Checking

```go
type HealthChecker interface {
    CheckHealth(ctx context.Context) error
    IsHealthy() bool
    GetHealthStatus() HealthStatus
}

type HealthStatus struct {
    Healthy   bool
    Timestamp time.Time
    Latency   time.Duration
    Error     string
    Metadata  map[string]any
}
```

## Security

### Authentication

```go
type AuthProvider interface {
    GetCredentials(ctx context.Context) (map[string]string, error)
    RefreshCredentials(ctx context.Context) error
    IsValid() bool
    ExpiresAt() time.Time
}

// Security features
type SecurityFeature string

const (
    SecurityTLS      SecurityFeature = "tls"
    SecurityMTLS     SecurityFeature = "mtls"
    SecurityJWT      SecurityFeature = "jwt"
    SecurityAPIKey   SecurityFeature = "api-key"
    SecurityOAuth2   SecurityFeature = "oauth2"
    SecurityCustom   SecurityFeature = "custom"
)
```

### TLS Configuration

```go
type TLSConfig struct {
    MinVersion        string
    MaxVersion        string
    CipherSuites      []string
    RequireClientCert bool
}
```

## Best Practices

### Connection Management

1. **Always use context for timeouts**: Pass context with appropriate timeouts to all transport operations
2. **Handle reconnection gracefully**: Implement proper reconnection logic with exponential backoff
3. **Monitor connection health**: Use health checkers to detect and handle connection issues

### Error Handling

1. **Implement circuit breakers**: Prevent cascading failures in distributed systems
2. **Use appropriate retry policies**: Configure retry behavior based on error types
3. **Log transport events**: Implement comprehensive logging for debugging and monitoring

### Performance Optimization

1. **Use batching for high throughput**: Batch multiple events for better performance
2. **Configure appropriate buffer sizes**: Size buffers based on expected load
3. **Enable compression**: Use compression for large messages to reduce bandwidth

### Security

1. **Always use secure connections**: Prefer TLS/mTLS over plain connections
2. **Implement proper authentication**: Use appropriate auth mechanisms for your use case
3. **Validate certificates**: Properly validate server certificates in production

## Migration Guide

### From Legacy Transport

```go
// Old approach
legacyTransport := &OldTransport{
    Endpoint: "ws://example.com",
    Features: map[string]interface{}{
        "compression": true,
        "security":    "tls",
    },
}

// New approach with type safety
compressionFeatures := transport.CompressionFeatures{
    SupportedAlgorithms: []transport.CompressionType{transport.CompressionGzip},
    DefaultAlgorithm:    transport.CompressionGzip,
}

newCapabilities := transport.NewCompressionCapabilities(
    transport.Capabilities{
        Streaming:     true,
        Bidirectional: true,
        Security:      []transport.SecurityFeature{transport.SecurityTLS},
    },
    compressionFeatures,
)

newTransport := transport.NewTransportWithCapabilities(config, newCapabilities)
```

This comprehensive API reference provides detailed information about all transport abstraction features, usage patterns, and best practices for the AG-UI Go SDK.