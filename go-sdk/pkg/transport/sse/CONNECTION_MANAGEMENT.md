# SSE Connection Management System

This document describes the comprehensive connection management system for HTTP Server-Sent Events (SSE) transport in the ag-ui Go SDK.

## Overview

The connection management system provides robust, production-ready SSE connections with advanced features including:

- **Connection State Management**: Comprehensive state tracking and transitions
- **Connection Pooling**: Efficient connection reuse with load balancing
- **Automatic Reconnection**: Exponential backoff with jitter for network interruptions
- **Heartbeat/Keepalive**: Connection health monitoring with configurable intervals
- **Connection Health Monitoring**: Real-time metrics and performance tracking
- **Graceful Connection Termination**: Proper cleanup and resource management

## Architecture

### Core Components

1. **Connection**: Individual SSE connection with full lifecycle management
2. **ConnectionPool**: Pool of connections for high-throughput scenarios
3. **ConnectionMetrics**: Comprehensive metrics collection for monitoring
4. **ReconnectionPolicy**: Configurable reconnection behavior
5. **HeartbeatConfig**: Health monitoring configuration

### Connection States

The system supports the following connection states:

- `disconnected`: Initial state, not connected
- `connecting`: Attempting to establish connection
- `connected`: Successfully connected and ready
- `reconnecting`: Attempting to reconnect after failure
- `error`: Connection failed with unrecoverable error
- `closed`: Permanently closed, no further operations allowed

## Features

### 1. Connection State Management

```go
// Create a new connection
config := DefaultConfig()
conn, err := NewConnection(config, nil)
if err != nil {
    log.Fatal(err)
}

// Monitor state changes
go func() {
    for state := range conn.ReadStateChanges() {
        log.Printf("State changed to: %s", state.String())
    }
}()

// Connect
ctx := context.Background()
if err := conn.Connect(ctx); err != nil {
    log.Printf("Connection failed: %v", err)
}
```

### 2. Connection Pooling and Reuse

```go
// Create connection pool
pool, err := NewConnectionPool(config)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

// Acquire connection from pool
conn, err := pool.AcquireConnection(ctx)
if err != nil {
    log.Printf("Failed to acquire connection: %v", err)
}
defer pool.ReleaseConnection(conn)

// Use connection...
```

### 3. Automatic Reconnection

```go
// Configure reconnection policy
conn.reconnectPolicy = &ReconnectionPolicy{
    Enabled:           true,
    MaxAttempts:       10,
    InitialDelay:      100 * time.Millisecond,
    MaxDelay:          30 * time.Second,
    BackoffMultiplier: 2.0,
    JitterFactor:      0.1,
    ResetInterval:     5 * time.Minute,
}

// Reconnection happens automatically on connection failures
```

### 4. Heartbeat/Keepalive Mechanism

```go
// Configure heartbeat
conn.heartbeatConfig = &HeartbeatConfig{
    Enabled:      true,
    Interval:     30 * time.Second,
    Timeout:      5 * time.Second,
    MaxMissed:    3,
    PingEndpoint: "/ping",
}

// Heartbeat monitoring runs automatically when connected
```

### 5. Connection Health Monitoring

```go
// Get real-time metrics
metrics := conn.GetMetrics()
fmt.Printf("Connect Success Rate: %.1f%%", metrics.GetConnectSuccessRate())
fmt.Printf("Heartbeat Success Rate: %.1f%%", metrics.GetHeartbeatSuccessRate())

// Get comprehensive connection info
info := conn.GetConnectionInfo()
fmt.Printf("Uptime: %v", info["uptime"])
fmt.Printf("Reconnect Attempts: %d", info["reconnect_attempts"])
```

### 6. Graceful Connection Termination

```go
// Graceful disconnect
conn.Disconnect()

// Permanent close with cleanup
conn.Close()
```

## Configuration

### Basic Configuration

```go
config := &Config{
    BaseURL:        "https://api.example.com",
    Headers:        map[string]string{
        "Authorization": "Bearer token",
        "X-API-Key":    "api-key",
    },
    BufferSize:     1000,
    ReadTimeout:    60 * time.Second,
    WriteTimeout:   30 * time.Second,
    ReconnectDelay: 1 * time.Second,
    MaxReconnects:  5,
    Client: &http.Client{
        Timeout: 30 * time.Second,
    },
}
```

### Advanced Configuration

#### Reconnection Policy

```go
policy := &ReconnectionPolicy{
    Enabled:           true,        // Enable reconnection
    MaxAttempts:       10,          // Max attempts (0 = unlimited)
    InitialDelay:      100 * time.Millisecond,  // Initial delay
    MaxDelay:          30 * time.Second,        // Maximum delay
    BackoffMultiplier: 2.0,         // Exponential backoff multiplier
    JitterFactor:      0.1,         // Jitter factor (0.0-1.0)
    ResetInterval:     5 * time.Minute,  // Reset count after success
}
```

#### Heartbeat Configuration

```go
heartbeat := &HeartbeatConfig{
    Enabled:      true,             // Enable heartbeat
    Interval:     30 * time.Second, // Heartbeat interval
    Timeout:      5 * time.Second,  // Heartbeat timeout
    MaxMissed:    3,                // Max missed before dead
    PingEndpoint: "/ping",          // Ping endpoint
}
```

## Event Handling

### Reading Events

```go
// Handle incoming events
go func() {
    for event := range conn.ReadEvents() {
        switch event.Type() {
        case events.EventTypeTextMessageContent:
            // Handle text message
        case events.EventTypeStateSnapshot:
            // Handle state snapshot
        default:
            log.Printf("Unknown event: %s", event.Type())
        }
    }
}()
```

### Error Handling

```go
// Handle connection errors
go func() {
    for err := range conn.ReadErrors() {
        if messages.IsConnectionError(err) {
            log.Printf("Connection error: %v", err)
        } else if messages.IsStreamingError(err) {
            log.Printf("Streaming error: %v", err)
        }
    }
}()
```

## Metrics and Monitoring

### Connection Metrics

The system provides comprehensive metrics for monitoring:

- **Connection Lifecycle**: Connect attempts, successes, failures
- **Reconnection**: Reconnect attempts, successes, failures  
- **Duration**: Connect durations, connection uptime
- **Heartbeat**: Heartbeats sent, success/failure rates
- **Network**: Bytes sent/received, events sent/received
- **Errors**: Network errors, timeout errors, protocol errors

### Pool Metrics

For connection pools:

- **Pool Status**: Total, active, idle connections
- **Utilization**: Pool utilization percentage
- **Operations**: Acquire requests, successes, timeouts

### Accessing Metrics

```go
// Connection metrics
metrics := conn.GetMetrics()
info := conn.GetConnectionInfo()

// Pool metrics  
stats := pool.GetPoolStats()
healthyCount := pool.GetHealthyConnectionCount()
```

## Best Practices

### 1. Connection Lifecycle

- Always call `Close()` when done with connections
- Use `defer conn.Close()` for automatic cleanup
- Monitor state changes for debugging

### 2. Error Handling

- Handle both connection and streaming errors
- Implement proper retry logic for transient failures
- Log errors with appropriate context

### 3. Resource Management

- Use connection pools for high-throughput scenarios
- Configure appropriate buffer sizes for your workload
- Monitor metrics to detect issues early

### 4. Network Resilience

- Enable reconnection for production deployments
- Configure appropriate backoff and jitter
- Set reasonable heartbeat intervals

### 5. Security

- Always use HTTPS in production
- Implement proper authentication headers
- Validate server certificates

## Examples

See `connection_example.go` for comprehensive usage examples including:

- Basic connection usage
- Connection pooling
- Reconnection handling
- Heartbeat monitoring
- Advanced configuration
- Production deployment patterns

## Testing

The connection management system includes comprehensive tests:

```bash
go test ./pkg/transport/sse/connection_test.go ./pkg/transport/sse/connection.go ./pkg/transport/sse/transport.go -v
```

Tests cover:
- Connection state management
- Reconnection policies
- Heartbeat functionality
- Metrics collection
- Pool operations
- Error handling

## Integration

The connection management system integrates seamlessly with:

- **SSE Transport**: Provides the underlying connection management
- **Event System**: Handles event streaming and processing
- **Metrics Collection**: Exports metrics to monitoring systems
- **Configuration System**: Supports comprehensive configuration

## Performance Considerations

### Memory Usage

- Connections use buffered channels for event handling
- Metrics use atomic operations for thread safety
- Pool maintains connection references efficiently

### CPU Usage

- Heartbeat monitoring runs in background goroutines
- Reconnection uses exponential backoff to reduce load
- State transitions are optimized with atomic operations

### Network Usage

- Heartbeats are lightweight HTTP requests
- Reconnection includes jitter to prevent thundering herd
- Connection reuse reduces connection overhead

## Troubleshooting

### Common Issues

1. **Connection Failures**
   - Check network connectivity
   - Verify server endpoint availability
   - Review authentication credentials

2. **Reconnection Loops**
   - Check reconnection policy configuration
   - Monitor server-side connection limits
   - Review error logs for patterns

3. **High Memory Usage**
   - Monitor event buffer sizes
   - Check for connection leaks
   - Review pool configuration

4. **Poor Performance**
   - Monitor heartbeat success rates
   - Check network latency
   - Review pool utilization metrics

### Debugging

Enable detailed logging and monitoring:

```go
// Monitor all state changes
go func() {
    for state := range conn.ReadStateChanges() {
        log.Printf("State: %s", state.String())
    }
}()

// Log all errors
go func() {
    for err := range conn.ReadErrors() {
        log.Printf("Error: %v", err)
    }
}()

// Print metrics periodically
go func() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        printConnectionMetrics(conn)
    }
}()
```

This connection management system provides enterprise-grade reliability and performance for SSE-based applications while maintaining simplicity for basic use cases.