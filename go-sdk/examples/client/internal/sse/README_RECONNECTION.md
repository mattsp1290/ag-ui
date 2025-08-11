# SSE Client Reconnection Feature

## Overview

The SSE client now includes robust automatic reconnection capabilities with exponential backoff, jitter, and comprehensive error handling. This ensures reliable streaming even across network failures and server restarts.

## Features

### Core Capabilities
- **Exponential Backoff with Jitter**: Prevents thundering herd and respects server resources
- **Error Classification**: Intelligent retry decisions based on error types
- **HTTP 429 Support**: Respects rate limiting with Retry-After header parsing
- **Context Cancellation**: Clean shutdown at all wait points and I/O boundaries
- **Idle Timeout Detection**: Reconnects when no data received for configured duration
- **Connection Statistics**: Monitor reconnection attempts and success metrics
- **Resource Management**: No goroutine or file descriptor leaks
- **Last-Event-ID Support**: Optional event resumption (when server supports it)

### Configuration Options

```go
type ReconnectionConfig struct {
    Enabled           bool          // Enable/disable reconnection
    InitialDelay      time.Duration // First retry delay (default: 250ms)
    MaxDelay          time.Duration // Maximum retry delay (default: 30s)
    BackoffMultiplier float64       // Exponential growth factor (default: 2.0)
    JitterFactor      float64       // Random jitter ±percentage (default: 0.2)
    MaxRetries        int           // Max attempts, 0=unlimited (default: 0)
    MaxElapsedTime    time.Duration // Total time limit, 0=unlimited (default: 0)
    ResetInterval     time.Duration // Reset backoff after stable connection (default: 60s)
    IdleTimeout       time.Duration // Reconnect if idle for this duration (default: 5m)
}
```

## Usage

### Basic Usage with Defaults

```go
config := sse.Config{
    Endpoint: "https://api.example.com/events",
    APIKey:   "your-api-key",
    Logger:   logrus.New(),
}

reconnectConfig := sse.DefaultReconnectionConfig()
client := sse.NewReconnectingClient(config, reconnectConfig)

frames, errors, err := client.StreamWithReconnect(sse.StreamOptions{
    Context: ctx,
})
```

### Custom Configuration

```go
reconnectConfig := sse.ReconnectionConfig{
    Enabled:           true,
    InitialDelay:      100 * time.Millisecond,
    MaxDelay:          10 * time.Second,
    BackoffMultiplier: 1.5,
    JitterFactor:      0.3,
    MaxRetries:        50,
    MaxElapsedTime:    1 * time.Hour,
    ResetInterval:     30 * time.Second,
    IdleTimeout:       1 * time.Minute,
}
```

## Error Classification

The client intelligently classifies errors to determine retry behavior:

### Retryable Errors
- Network failures (connection refused, reset, timeout)
- EOF and unexpected EOF
- HTTP 5xx server errors
- HTTP 408 (Request Timeout)
- HTTP 429 (Too Many Requests)
- HTTP 502, 503, 504 (Gateway errors)

### Non-Retryable Errors
- HTTP 401 (Unauthorized)
- HTTP 403 (Forbidden)
- HTTP 404 (Not Found)
- Other 4xx client errors

## Backoff Schedule

With default configuration (initial=250ms, multiplier=2.0, max=30s):

| Attempt | Base Delay | With Jitter (±20%) |
|---------|------------|-------------------|
| 1       | 250ms      | 200-300ms        |
| 2       | 500ms      | 400-600ms        |
| 3       | 1s         | 800ms-1.2s       |
| 4       | 2s         | 1.6-2.4s         |
| 5       | 4s         | 3.2-4.8s         |
| 6       | 8s         | 6.4-9.6s         |
| 7       | 16s        | 12.8-19.2s       |
| 8+      | 30s        | 24-36s           |

## Monitoring

Get connection statistics:

```go
stats := client.GetStats()
// Returns:
// {
//   "attempt_count": 3,
//   "last_success": "2024-01-10T10:30:00Z",
//   "start_time": "2024-01-10T10:00:00Z",
//   "elapsed": "30m0s",
//   "last_event_id": "event-123"
// }
```

## Testing

Comprehensive test coverage includes:
- Exponential backoff calculation
- Error classification logic
- Connection drop recovery
- Context cancellation
- Max retries enforcement
- Max elapsed time limits
- Idle timeout detection
- Goroutine leak prevention
- Last-Event-ID propagation

Run tests:
```bash
go test -v ./internal/sse -run TestReconnector
```

## Implementation Details

### State Management
- Tracks connection attempts and success time
- Resets backoff after stable connection period
- Maintains Last-Event-ID for resumption

### Resource Safety
- Proper cleanup on context cancellation
- No goroutine leaks across reconnections
- Bounded channels prevent memory issues
- Clean shutdown of all resources

### Logging
Structured logging at key lifecycle points:
- Connection attempts with attempt number
- Backoff delays and elapsed time
- Error classification decisions
- Idle timeout triggers
- Connection recovery events

## Migration from Basic Client

Existing code using the basic `Stream()` method:

```go
// Old approach
frames, errors, err := client.Stream(opts)
```

Can be easily upgraded to use reconnection:

```go
// New approach with reconnection
reconnectConfig := sse.DefaultReconnectionConfig()
frames, errors, err := client.StreamWithReconnect(opts, reconnectConfig)
```

Or use the `ReconnectingClient` directly:

```go
rc := sse.NewReconnectingClient(config, reconnectConfig)
frames, errors, err := rc.StreamWithReconnect(opts)
```

## Performance Considerations

- Jitter prevents thundering herd when multiple clients reconnect
- Exponential backoff reduces server load during outages
- Idle timeout detection prevents hanging connections
- Resource cleanup ensures no memory leaks over time

## Future Enhancements

- [ ] Metrics hooks for monitoring systems
- [ ] Circuit breaker pattern integration
- [ ] Adaptive backoff based on error patterns
- [ ] Connection pooling for multiple streams
- [ ] Health check endpoint support