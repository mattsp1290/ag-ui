# AG-UI Server Framework

This document describes the core server framework implementation for AG-UI compatible endpoints located in `framework.go`.

## Overview

The AG-UI Server Framework provides a production-ready, framework-agnostic foundation for building AG-UI compatible servers. It implements lifecycle management, request routing, agent endpoint management, and graceful shutdown capabilities.

## Key Features

- **Framework-Agnostic Design**: Works independently without external web framework dependencies
- **Production-Ready Lifecycle Management**: Complete initialization, start, stop, and shutdown cycle
- **Agent Registration and Discovery**: Plugin-style agent registration with discovery capabilities
- **Request Routing**: Configurable routing with middleware support
- **Graceful Shutdown**: Proper resource cleanup and connection handling
- **Health Checks**: Comprehensive health monitoring and status reporting
- **Configuration Management**: Flexible YAML/JSON configuration with validation
- **Middleware System**: Composable middleware chain with CORS, logging, metrics support
- **Type Safety**: Strongly-typed interfaces and error handling

## Core Components

### ServerFramework Interface

The main interface provides:
- `Initialize(ctx, config)` - Framework initialization
- `Start(ctx)` - Start the server
- `Stop(ctx)` - Graceful stop
- `Shutdown(ctx)` - Complete shutdown with cleanup
- `RegisterAgent(agent)` - Register AG-UI agents
- `RegisterHandler(pattern, handler)` - Register custom request handlers
- `RegisterMiddleware(middleware)` - Add middleware to the chain

### Configuration

```go
config := server.DefaultFrameworkConfig()
config.Name = "My AG-UI Server"
config.HTTP.Port = 8080
config.HTTP.Host = "0.0.0.0"

// Enable TLS
config.HTTP.TLS.Enabled = true
config.HTTP.TLS.CertFile = "/path/to/cert.pem"
config.HTTP.TLS.KeyFile = "/path/to/key.pem"

// Configure CORS
config.HTTP.CORS.Enabled = true
config.HTTP.CORS.AllowOrigins = []string{"https://myapp.com"}

// Agent management
config.Agents.MaxAgents = 50
config.Agents.HealthCheckEnabled = true
```

### Agent Registration

```go
// Implement the core.Agent interface
type MyAgent struct {
    name string
}

func (a *MyAgent) HandleEvent(ctx context.Context, event any) ([]any, error) {
    // Process the event and return responses
    return []any{"response"}, nil
}

func (a *MyAgent) Name() string { return a.name }
func (a *MyAgent) Description() string { return "My custom agent" }

// Register with framework
agent := &MyAgent{name: "my-agent"}
framework.RegisterAgent(agent)
```

### Custom Handlers

```go
type CustomHandler struct{}

func (h *CustomHandler) Handle(ctx context.Context, req *server.Request, resp server.ResponseWriter) error {
    return resp.WriteJSON(map[string]string{"message": "Hello World"})
}

func (h *CustomHandler) Pattern() string { return "/api/custom" }
func (h *CustomHandler) Methods() []string { return []string{"GET", "POST"} }

framework.RegisterHandler("/api/custom", &CustomHandler{})
```

### Middleware

```go
type LoggingMiddleware struct{}

func (m *LoggingMiddleware) Process(ctx context.Context, req *server.Request, resp server.ResponseWriter, next server.NextHandler) error {
    start := time.Now()
    err := next(ctx, req, resp)
    log.Printf("%s %s - %v", req.Method, req.URL.Path, time.Since(start))
    return err
}

func (m *LoggingMiddleware) Name() string { return "logging" }
func (m *LoggingMiddleware) Priority() int { return 100 }

framework.RegisterMiddleware(&LoggingMiddleware{})
```

## Usage Example

```go
package main

import (
    "context"
    "log"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/server"
)

func main() {
    // Create and configure framework
    config := server.DefaultFrameworkConfig()
    config.HTTP.Port = 8080
    
    framework := server.NewFramework()
    if err := framework.Initialize(context.Background(), config); err != nil {
        log.Fatal(err)
    }

    // Register agents
    agent := server.NewExampleAgent("echo", "Echo agent")
    framework.RegisterAgent(agent)

    // Start server
    if err := framework.Start(context.Background()); err != nil {
        log.Fatal(err)
    }

    // Server is now running
    log.Println("Server started on port 8080")
    
    // Graceful shutdown (in production, use signal handling)
    // framework.Stop(context.Background())
    // framework.Shutdown(context.Background())
}
```

## Built-in Endpoints

The framework automatically provides:

- `GET /health` - Health check endpoint
- `GET /status` - Framework status and metrics
- `GET /agents` - List registered agents

## Default Middleware

- **Logging Middleware**: Request/response logging
- **Metrics Middleware**: Request metrics collection
- **CORS Middleware**: Cross-Origin Resource Sharing support

## Configuration File Example

```yaml
name: "Production AG-UI Server"
version: "1.0.0"
description: "Production server"

http:
  host: "0.0.0.0"
  port: 443
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"
  tls:
    enabled: true
    cert_file: "/etc/ssl/certs/server.crt"
    key_file: "/etc/ssl/private/server.key"
  cors:
    enabled: true
    allow_origins: ["https://myapp.com"]
    allow_methods: ["GET", "POST", "PUT", "DELETE"]
    allow_headers: ["Content-Type", "Authorization"]

agents:
  max_agents: 100
  discovery_enabled: true
  health_check_enabled: true

middleware:
  enable_logging: true
  enable_metrics: true
  enable_rate_limit: false

security:
  enable_https: true
  rate_limit_per_min: 1000
  max_request_size: 10485760  # 10MB
```

## Testing

The framework includes comprehensive tests:

```bash
go test ./pkg/server/framework.go ./pkg/server/framework_test.go ./pkg/server/example_usage.go -v
```

## Integration with Existing AG-UI Components

The framework integrates with:
- **Transport Layer** (`pkg/transport/`) - For protocol abstraction
- **Encoding System** (`pkg/encoding/`) - For message serialization
- **Error Handling** (`pkg/errors/`) - For structured error management
- **Event System** (`pkg/core/events/`) - For AG-UI event processing

## Architecture Notes

- **Thread-Safe**: All operations are safe for concurrent use
- **Atomic State Management**: Uses atomic operations for state transitions
- **Graceful Shutdown**: Proper cleanup of resources and connections
- **Production Ready**: Includes timeouts, limits, and error handling
- **Extensible**: Plugin architecture for agents and middleware
- **Framework Agnostic**: No external web framework dependencies

## Error Handling

The framework uses the AG-UI error system with:
- Structured error types with context
- Retry logic for transient errors
- Proper error propagation and logging
- Health check integration

This framework provides a solid foundation for building production AG-UI servers with proper lifecycle management and extensibility.