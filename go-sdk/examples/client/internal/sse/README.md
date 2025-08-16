# SSE Client Package

This package provides a robust Server-Sent Events (SSE) client implementation for the AG-UI Tool-Based Generative UI endpoint.

## Features

- **Streaming HTTP Client**: Optimized transport settings for long-lived connections
- **Automatic Frame Parsing**: Handles SSE `data:` frames with proper line buffering
- **Context Support**: Full context cancellation and timeout support
- **Backpressure Management**: Buffered channels prevent unbounded memory usage
- **Authentication**: Support for API key authentication via configurable headers
- **Comprehensive Logging**: Debug-level logging for connection lifecycle and data flow
- **Error Handling**: Graceful error propagation through dedicated error channel

## Architecture

### Core Components

1. **Client**: Main SSE client struct with configuration and HTTP client
2. **Config**: Configuration for endpoint, auth, timeouts, and buffer sizes
3. **Frame**: Individual SSE frame with data and timestamp
4. **StreamOptions**: Options for establishing an SSE stream

### Transport Configuration

The HTTP transport is specifically tuned for SSE streaming:
- `DisableCompression: true` - SSE data is typically already efficient
- `ExpectContinueTimeout: 0` - No expect-continue for streaming
- `DisableKeepAlives: false` - Allows connection reuse
- `MaxIdleConns: 1` - Single connection per client
- Response header timeout for initial connection
- No overall client timeout to support long streams

## Usage

### Basic Example

```go
import (
    "context"
    "github.com/ag-ui/go-sdk/examples/client/internal/sse"
    "github.com/sirupsen/logrus"
)

// Configure client
config := sse.Config{
    Endpoint:       "http://localhost:8080/tool_based_generative_ui",
    APIKey:         "your-api-key",
    ConnectTimeout: 30 * time.Second,
    ReadTimeout:    5 * time.Minute,
    BufferSize:     100,
    Logger:         logrus.New(),
}

// Create client
client := sse.NewClient(config)
defer client.Close()

// Prepare payload
payload := sse.RunAgentInput{
    SessionID: "my-session",
    Messages: []sse.Message{
        {Role: "user", Content: "Hello"},
    },
    Stream: true,
}

// Start streaming
ctx := context.Background()
frames, errors, err := client.Stream(sse.StreamOptions{
    Context: ctx,
    Payload: payload,
})

if err != nil {
    log.Fatal(err)
}

// Process frames
for {
    select {
    case frame, ok := <-frames:
        if !ok {
            return // Stream closed
        }
        // Process frame.Data
        
    case err := <-errors:
        if err != nil {
            log.Error(err)
            return
        }
        
    case <-ctx.Done():
        return // Context cancelled
    }
}
```

### With Context Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Handle interrupt signals
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

go func() {
    <-sigChan
    cancel()
}()

frames, errors, err := client.Stream(sse.StreamOptions{
    Context: ctx,
    Payload: payload,
})
```

## SSE Protocol

The client expects SSE frames in the standard format:
```
data: {"event":"start","data":{"session_id":"123"}}

data: {"event":"message","data":{"content":"Hello"}}

data: {"event":"end"}

```

Each frame:
- Starts with `data: ` prefix
- Contains JSON payload
- Ends with double newline (`\n\n`)
- Multi-line data is supported (multiple `data:` lines before blank line)

## Error Handling

Errors are reported through the error channel:
- Connection errors during initial setup return immediately
- Read errors during streaming are sent to the error channel
- Context cancellation gracefully closes the stream
- Non-200 status codes return detailed error with response body

## Testing

The package includes comprehensive tests:
- Basic connectivity and frame parsing
- Authentication (Bearer token and custom headers)
- Multi-line frame handling
- Context cancellation
- Error scenarios (non-200, wrong content-type)

Run tests:
```bash
go test ./internal/sse/... -v
```

## Logging

The client uses structured logging via logrus:
- **Debug**: Connection setup, headers, periodic progress
- **Info**: Connection established/closed, stream statistics
- **Warn/Error**: Connection failures, read errors

## Performance Considerations

1. **Buffer Size**: Configure based on expected event rate
2. **Read Timeout**: Balance between detecting stale connections and allowing quiet periods
3. **Context**: Always provide context for proper resource cleanup
4. **Goroutine Safety**: Channels are safe for concurrent use

## Future Enhancements

- [ ] Automatic reconnection with exponential backoff
- [ ] Metrics collection (events/sec, bytes transferred)
- [ ] Event type filtering
- [ ] Compression support for large payloads
- [ ] Connection pooling for multiple concurrent streams