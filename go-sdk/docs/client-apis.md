# Client APIs Guide

Comprehensive guide to using the AG-UI Go SDK client APIs for connecting to servers and managing communication.

## Table of Contents

- [Overview](#overview)
- [Client Configuration](#client-configuration)
- [Basic Usage](#basic-usage)
- [Advanced Features](#advanced-features)
- [Error Handling](#error-handling)
- [Best Practices](#best-practices)
- [Examples](#examples)

## Overview

The AG-UI Go SDK client APIs provide a robust foundation for communicating with AG-UI servers. The client supports multiple transport protocols, authentication methods, and advanced features like streaming and event management.

### Key Features

- **Multiple Transport Protocols**: HTTP/REST, WebSocket, Server-Sent Events (SSE)
- **Authentication Support**: Bearer tokens, API keys, JWT validation
- **Event Streaming**: Real-time bidirectional communication
- **Connection Management**: Automatic reconnection, health checks
- **Error Recovery**: Automatic retry with exponential backoff
- **Context Support**: Proper cancellation and timeout handling

## Client Configuration

### Basic Configuration

```go
package main

import (
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
)

func main() {
    // Minimal configuration
    config := client.Config{
        BaseURL: "https://api.example.com/ag-ui",
    }
    
    client, err := client.New(config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
}
```

### Advanced Configuration

```go
// Production configuration with all options
config := client.Config{
    // Connection settings
    BaseURL:        "https://api.example.com/ag-ui",
    Timeout:        30 * time.Second,
    MaxRetries:     3,
    RetryBackoff:   time.Second,
    
    // Authentication
    AuthMethod:     client.AuthMethodBearer,
    Token:          os.Getenv("API_TOKEN"),
    TokenRefresher: tokenRefresher,
    
    // Transport options
    Transport:      client.TransportWebSocket,
    KeepAlive:      true,
    BufferSize:     1024,
    
    // TLS configuration
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
    },
    
    // Event handling
    EventHandlers: map[string]client.EventHandler{
        "message": handleMessage,
        "error":   handleError,
    },
    
    // Monitoring
    MetricsEnabled: true,
    Logger:         logger,
}

client, err := client.New(config)
if err != nil {
    return fmt.Errorf("failed to create client: %w", err)
}
```

### Configuration Options

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `BaseURL` | `string` | Server base URL (required) | - |
| `Timeout` | `time.Duration` | Request timeout | `30s` |
| `MaxRetries` | `int` | Maximum retry attempts | `3` |
| `RetryBackoff` | `time.Duration` | Initial retry delay | `1s` |
| `AuthMethod` | `AuthMethod` | Authentication method | `None` |
| `Token` | `string` | Authentication token | - |
| `Transport` | `Transport` | Transport protocol | `HTTP` |
| `KeepAlive` | `bool` | Enable connection keep-alive | `true` |
| `BufferSize` | `int` | Event buffer size | `256` |
| `TLSConfig` | `*tls.Config` | TLS configuration | `nil` |

## Basic Usage

### Creating a Client

```go
// Create client with minimal configuration
client, err := client.New(client.Config{
    BaseURL: "https://api.example.com/ag-ui",
})
if err != nil {
    return fmt.Errorf("failed to create client: %w", err)
}
defer client.Close()
```

### Sending Events

```go
// Send a simple message event
ctx := context.Background()
event := map[string]interface{}{
    "type":    "message",
    "content": "Hello, agent!",
    "metadata": map[string]interface{}{
        "user_id": "user-123",
        "session_id": "session-456",
    },
}

responses, err := client.SendEvent(ctx, "my-agent", event)
if err != nil {
    return fmt.Errorf("failed to send event: %w", err)
}

for _, response := range responses {
    fmt.Printf("Received response: %v\n", response)
}
```

### Streaming Events

```go
// Open streaming connection
ctx := context.Background()
eventStream, err := client.Stream(ctx, "my-agent")
if err != nil {
    return fmt.Errorf("failed to open stream: %w", err)
}

// Process streaming events
for event := range eventStream {
    switch e := event.(type) {
    case *client.MessageEvent:
        fmt.Printf("Message: %s\n", e.Content)
    case *client.ErrorEvent:
        fmt.Printf("Error: %s\n", e.Message)
    default:
        fmt.Printf("Unknown event: %v\n", event)
    }
}
```

## Advanced Features

### Authentication

#### Bearer Token Authentication

```go
config := client.Config{
    BaseURL:    "https://api.example.com/ag-ui",
    AuthMethod: client.AuthMethodBearer,
    Token:      "your-bearer-token",
}
```

#### JWT Authentication with Refresh

```go
tokenRefresher := &client.JWTRefresher{
    RefreshURL:    "https://auth.example.com/refresh",
    RefreshToken:  "your-refresh-token",
    ClientID:      "your-client-id",
    ClientSecret:  "your-client-secret",
}

config := client.Config{
    BaseURL:        "https://api.example.com/ag-ui",
    AuthMethod:     client.AuthMethodJWT,
    Token:          "your-jwt-token",
    TokenRefresher: tokenRefresher,
}
```

#### API Key Authentication

```go
config := client.Config{
    BaseURL:    "https://api.example.com/ag-ui",
    AuthMethod: client.AuthMethodAPIKey,
    APIKey:     "your-api-key",
    APIKeyHeader: "X-API-Key", // Optional, defaults to "X-API-Key"
}
```

### Transport Protocols

#### WebSocket Transport

```go
config := client.Config{
    BaseURL:   "wss://api.example.com/ag-ui/ws",
    Transport: client.TransportWebSocket,
    WebSocketConfig: &client.WebSocketConfig{
        PingInterval:    30 * time.Second,
        PongTimeout:     10 * time.Second,
        MaxMessageSize:  1024 * 1024, // 1MB
        Subprotocols:    []string{"ag-ui-v1"},
        EnableGzip:      true,
    },
}
```

#### Server-Sent Events Transport

```go
config := client.Config{
    BaseURL:   "https://api.example.com/ag-ui/sse",
    Transport: client.TransportSSE,
    SSEConfig: &client.SSEConfig{
        RetryInterval:   5 * time.Second,
        MaxRetries:      10,
        BufferSize:      512,
        EnableGzip:      true,
    },
}
```

### Connection Management

#### Health Checks

```go
// Configure health checks
config := client.Config{
    BaseURL:           "https://api.example.com/ag-ui",
    HealthCheckEnabled: true,
    HealthCheckInterval: 30 * time.Second,
    HealthCheckPath:     "/health",
    HealthCheckHandler: func(healthy bool, err error) {
        if !healthy {
            log.Printf("Health check failed: %v", err)
        }
    },
}
```

#### Automatic Reconnection

```go
// Configure reconnection behavior
config := client.Config{
    BaseURL:              "https://api.example.com/ag-ui",
    AutoReconnect:        true,
    ReconnectInterval:    5 * time.Second,
    MaxReconnectAttempts: 10,
    ReconnectBackoff:     client.ExponentialBackoff,
    ReconnectHandler: func(attempt int, err error) {
        log.Printf("Reconnection attempt %d failed: %v", attempt, err)
    },
}
```

### Event Handling

#### Registering Event Handlers

```go
// Create client with event handlers
config := client.Config{
    BaseURL: "https://api.example.com/ag-ui",
    EventHandlers: map[string]client.EventHandler{
        "message":     handleMessage,
        "state_delta": handleStateDelta,
        "tool_call":   handleToolCall,
        "error":       handleError,
    },
}

func handleMessage(ctx context.Context, event *client.MessageEvent) error {
    fmt.Printf("Received message: %s\n", event.Content)
    return nil
}

func handleStateDelta(ctx context.Context, event *client.StateDeltaEvent) error {
    fmt.Printf("State changed: %v\n", event.Delta)
    return nil
}

func handleToolCall(ctx context.Context, event *client.ToolCallEvent) error {
    fmt.Printf("Tool called: %s with args %v\n", event.ToolName, event.Arguments)
    return nil
}

func handleError(ctx context.Context, event *client.ErrorEvent) error {
    fmt.Printf("Error received: %s\n", event.Message)
    return nil
}
```

#### Middleware

```go
// Add middleware for logging and metrics
client.Use(client.LoggingMiddleware(logger))
client.Use(client.MetricsMiddleware(metricsCollector))
client.Use(client.RetryMiddleware(retryConfig))

// Custom middleware
client.Use(func(next client.Handler) client.Handler {
    return client.HandlerFunc(func(ctx context.Context, event client.Event) error {
        // Pre-processing
        start := time.Now()
        
        // Call next handler
        err := next.Handle(ctx, event)
        
        // Post-processing
        duration := time.Since(start)
        log.Printf("Event processed in %v", duration)
        
        return err
    })
})
```

## Error Handling

### Error Types

```go
// Client-specific errors
var (
    ErrConnectionClosed  = errors.New("connection closed")
    ErrInvalidResponse   = errors.New("invalid response")
    ErrAuthenticationFailed = errors.New("authentication failed")
    ErrRateLimited      = errors.New("rate limited")
    ErrTimeout          = errors.New("request timeout")
)
```

### Handling Different Error Types

```go
func handleClientError(err error) {
    switch {
    case errors.Is(err, client.ErrConnectionClosed):
        // Handle connection closure
        log.Println("Connection closed, attempting reconnect...")
        
    case errors.Is(err, client.ErrAuthenticationFailed):
        // Handle authentication failure
        log.Println("Authentication failed, refreshing token...")
        
    case errors.Is(err, client.ErrRateLimited):
        // Handle rate limiting
        log.Println("Rate limited, backing off...")
        
    case errors.Is(err, context.DeadlineExceeded):
        // Handle timeout
        log.Println("Request timed out, retrying...")
        
    default:
        // Handle other errors
        log.Printf("Unexpected error: %v", err)
    }
}
```

### Retry Configuration

```go
retryConfig := &client.RetryConfig{
    MaxAttempts:     5,
    InitialBackoff:  time.Second,
    MaxBackoff:      30 * time.Second,
    BackoffStrategy: client.ExponentialBackoff,
    RetryableErrors: []error{
        client.ErrConnectionClosed,
        client.ErrTimeout,
        context.DeadlineExceeded,
    },
}

config := client.Config{
    BaseURL:     "https://api.example.com/ag-ui",
    RetryConfig: retryConfig,
}
```

## Best Practices

### 1. Always Use Context

```go
// Good: Use context for cancellation and timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

response, err := client.SendEvent(ctx, "agent", event)
```

### 2. Handle Errors Appropriately

```go
// Good: Handle specific error types
_, err := client.SendEvent(ctx, "agent", event)
if err != nil {
    switch {
    case errors.Is(err, client.ErrAuthenticationFailed):
        // Refresh token and retry
        if err := refreshToken(); err == nil {
            return client.SendEvent(ctx, "agent", event)
        }
    case errors.Is(err, client.ErrRateLimited):
        // Wait and retry
        time.Sleep(time.Second)
        return client.SendEvent(ctx, "agent", event)
    default:
        return fmt.Errorf("failed to send event: %w", err)
    }
}
```

### 3. Use Connection Pooling

```go
// Good: Reuse client connections
var (
    clientOnce sync.Once
    sharedClient *client.Client
)

func getClient() (*client.Client, error) {
    clientOnce.Do(func() {
        config := client.Config{
            BaseURL:     "https://api.example.com/ag-ui",
            MaxPoolSize: 10,
            PoolTimeout: 30 * time.Second,
        }
        sharedClient, _ = client.New(config)
    })
    return sharedClient, nil
}
```

### 4. Implement Graceful Shutdown

```go
func main() {
    client, err := client.New(config)
    if err != nil {
        log.Fatal(err)
    }
    
    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        log.Println("Shutting down...")
        
        // Graceful shutdown with timeout
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        if err := client.Shutdown(ctx); err != nil {
            log.Printf("Shutdown error: %v", err)
        }
    }()
    
    // Run application
    runApplication(client)
}
```

### 5. Monitor Client Health

```go
// Set up monitoring
config := client.Config{
    BaseURL:           "https://api.example.com/ag-ui",
    MetricsEnabled:    true,
    HealthCheckEnabled: true,
    HealthCheckHandler: func(healthy bool, err error) {
        if !healthy {
            metrics.IncrementCounter("client.health_check.failed")
            log.Printf("Client health check failed: %v", err)
        }
    },
}
```

## Examples

### Complete Example: Chat Client

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"
    
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
)

func main() {
    // Create client
    client, err := createClient()
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // Start message processing
    ctx := context.Background()
    if err := processMessages(ctx, client); err != nil {
        log.Fatal(err)
    }
}

func createClient() (*client.Client, error) {
    config := client.Config{
        BaseURL:    os.Getenv("AG_UI_URL"),
        AuthMethod: client.AuthMethodBearer,
        Token:      os.Getenv("AUTH_TOKEN"),
        Transport:  client.TransportWebSocket,
        
        // Configure retry behavior
        MaxRetries:   3,
        RetryBackoff: time.Second,
        
        // Enable health checks
        HealthCheckEnabled:  true,
        HealthCheckInterval: 30 * time.Second,
        
        // Set up event handlers
        EventHandlers: map[string]client.EventHandler{
            "message": handleMessage,
            "error":   handleError,
        },
        
        // Configure WebSocket
        WebSocketConfig: &client.WebSocketConfig{
            PingInterval:   30 * time.Second,
            PongTimeout:    10 * time.Second,
            MaxMessageSize: 1024 * 1024,
        },
    }
    
    return client.New(config)
}

func processMessages(ctx context.Context, client *client.Client) error {
    // Open streaming connection
    stream, err := client.Stream(ctx, "chat-agent")
    if err != nil {
        return fmt.Errorf("failed to open stream: %w", err)
    }
    
    // Send initial message
    event := map[string]interface{}{
        "type":    "message",
        "content": "Hello, I'm ready to chat!",
    }
    
    if _, err := client.SendEvent(ctx, "chat-agent", event); err != nil {
        return fmt.Errorf("failed to send initial message: %w", err)
    }
    
    // Process incoming events
    for event := range stream {
        if err := processEvent(ctx, client, event); err != nil {
            log.Printf("Error processing event: %v", err)
        }
    }
    
    return nil
}

func processEvent(ctx context.Context, client *client.Client, event interface{}) error {
    switch e := event.(type) {
    case *client.MessageEvent:
        return handleMessageEvent(ctx, client, e)
    case *client.ToolCallEvent:
        return handleToolCallEvent(ctx, client, e)
    default:
        log.Printf("Unknown event type: %T", event)
        return nil
    }
}

func handleMessageEvent(ctx context.Context, client *client.Client, event *client.MessageEvent) error {
    fmt.Printf("Received message: %s\n", event.Content)
    
    // Send acknowledgment
    ackEvent := map[string]interface{}{
        "type":       "acknowledgment",
        "message_id": event.ID,
        "timestamp":  time.Now().Unix(),
    }
    
    _, err := client.SendEvent(ctx, "chat-agent", ackEvent)
    return err
}

func handleToolCallEvent(ctx context.Context, client *client.Client, event *client.ToolCallEvent) error {
    fmt.Printf("Tool call: %s with args %v\n", event.ToolName, event.Arguments)
    
    // Execute tool and send result
    result, err := executeTool(event.ToolName, event.Arguments)
    if err != nil {
        return fmt.Errorf("tool execution failed: %w", err)
    }
    
    resultEvent := map[string]interface{}{
        "type":        "tool_result",
        "tool_call_id": event.ID,
        "result":      result,
    }
    
    _, err = client.SendEvent(ctx, "chat-agent", resultEvent)
    return err
}

func executeTool(toolName string, args map[string]interface{}) (interface{}, error) {
    // Tool execution logic
    switch toolName {
    case "get_time":
        return time.Now().Format(time.RFC3339), nil
    case "calculate":
        // Simple calculator
        a, ok1 := args["a"].(float64)
        b, ok2 := args["b"].(float64)
        op, ok3 := args["operation"].(string)
        
        if !ok1 || !ok2 || !ok3 {
            return nil, fmt.Errorf("invalid arguments")
        }
        
        switch op {
        case "add":
            return a + b, nil
        case "subtract":
            return a - b, nil
        case "multiply":
            return a * b, nil
        case "divide":
            if b == 0 {
                return nil, fmt.Errorf("division by zero")
            }
            return a / b, nil
        default:
            return nil, fmt.Errorf("unknown operation: %s", op)
        }
    default:
        return nil, fmt.Errorf("unknown tool: %s", toolName)
    }
}

func handleMessage(ctx context.Context, event *client.MessageEvent) error {
    fmt.Printf("Handler: Received message: %s\n", event.Content)
    return nil
}

func handleError(ctx context.Context, event *client.ErrorEvent) error {
    log.Printf("Handler: Error received: %s\n", event.Message)
    return nil
}
```

This comprehensive client APIs guide provides everything needed to effectively use the AG-UI Go SDK client functionality, from basic usage to advanced features and best practices.