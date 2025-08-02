# AG-UI Go SDK Documentation

Comprehensive documentation for the AG-UI Go SDK, providing everything needed for development, deployment, and production operations. This latest release introduces type-safe transport abstractions and enhanced event systems for building AI agent applications with real-time communication capabilities.

## Documentation Overview

This documentation suite provides complete coverage of the AG-UI Go SDK from basic usage to enterprise production deployment, including the latest type-safe transport layer and event system enhancements.

### Quick Navigation

| Document | Description | Audience |
|----------|-------------|----------|
| [API Reference](api-reference.md) | Complete API documentation with examples | Developers |
| [Client APIs Guide](client-apis.md) | Client connection and communication patterns | Developers |
| [Event Validation Guide](event-validation.md) | Advanced event validation system | Developers |
| [Production Deployment](production-deployment.md) | Enterprise deployment with security hardening | DevOps/SRE |
| [OpenTelemetry Monitoring](opentelemetry-monitoring.md) | Comprehensive observability setup | DevOps/SRE |

## Documentation Structure

### 📚 API Documentation

Comprehensive reference for all public APIs in the AG-UI Go SDK:

- **[API Reference](api-reference.md)** - Complete API documentation covering:
  - Client APIs for server communication
  - Event system APIs and interfaces
  - State management with transactions
  - Tools framework and execution
  - Transport layer protocols
  - Monitoring and observability
  - Error handling patterns
  - Type definitions and utilities

- **[Transport Abstraction API](transport_abstraction_api.md)** - Complete API reference for the new transport layer:
  - Type-safe transport interfaces and configurations
  - Protocol implementations (WebSocket, HTTP, gRPC)
  - Capabilities system and feature management
  - Connection management and health checking
  - Error handling and retry strategies

- **[Type-Safe Event System API](event_system_api.md)** - Comprehensive guide to the enhanced event system:
  - AG-UI protocol event types and builders
  - Validation rules and automatic ID generation
  - JSON and Protocol Buffer serialization
  - Streaming message support with chunking

- **[Client APIs Guide](client-apis.md)** - Detailed client usage patterns:
  - Basic client setup and configuration
  - Authentication methods (Bearer, JWT, API Key)
  - Transport protocols (HTTP, WebSocket, SSE)
  - Connection management and health checks
  - Error handling and retry strategies
  - Complete working examples

- **[Event Validation Guide](event-validation.md)** - Advanced validation system:
  - Multi-level validation (structural, protocol, business)
  - Parallel validation for high throughput
  - Custom validation rules engine
  - Performance optimization techniques
  - Monitoring and metrics integration
  - Best practices and patterns

## 🚀 New Features & Enhancements

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

### 🚀 Production Deployment

Enterprise-grade deployment documentation:

- **[Production Deployment Guide](production-deployment.md)** - Complete production setup:
  - Infrastructure requirements and architecture
  - Security hardening (TLS, authentication, RBAC)
  - Container and Kubernetes deployment
  - High availability configuration
  - Configuration management and secrets
  - Security checklist and verification
  - Performance optimization

- **[OpenTelemetry Monitoring](opentelemetry-monitoring.md)** - Comprehensive observability:
  - OpenTelemetry configuration and setup
  - Distributed tracing implementation
  - Metrics collection and custom instrumentation
  - Backend integration (Jaeger, Prometheus, Grafana)
  - Performance optimization and sampling
  - Troubleshooting and debugging

## Getting Started

### For Developers

1. **Start with the [API Reference](api-reference.md)** to understand the core concepts
2. **Explore the new [Transport Abstraction API](transport_abstraction_api.md)** for modern transport usage
3. **Learn the [Type-Safe Event System API](event_system_api.md)** for structured event handling
4. **Follow the [Client APIs Guide](client-apis.md)** for practical implementation
5. **Implement validation using the [Event Validation Guide](event-validation.md)**
6. **Use the [Migration Guide](migration_guide.md)** when updating from legacy implementations

### For DevOps/SRE Teams

1. **Review the [Production Deployment Guide](production-deployment.md)** for infrastructure setup
2. **Implement monitoring with the [OpenTelemetry Guide](opentelemetry-monitoring.md)**
3. **Use the security checklists** for hardening verification

### Examples and Quick Start

- **[Transport Basic Usage](../examples/transport/basic_usage/)** - Basic transport operations and configuration
- **[Type-Safe Events Usage](../examples/events/type_safe_usage/)** - Event creation, validation, and serialization
- **[Collaborative Editing](../examples/state/collaborative_editing/)** - Real-time state synchronization example

## Key Features Documented

### 🔐 Security & Authentication
- Comprehensive authentication patterns (JWT, API Key, Bearer Token)
- RBAC implementation with hierarchical roles
- Security hardening checklists and best practices
- TLS configuration and certificate management
- Input validation and sanitization

### 📊 Monitoring & Observability
- OpenTelemetry integration with full telemetry pipeline
- Custom metrics and business intelligence
- Distributed tracing across microservices
- Alert configuration and SLA monitoring
- Performance optimization and troubleshooting

### ⚡ Performance & Scalability
- High-performance event processing
- Parallel validation systems
- Connection pooling and management
- Caching strategies and optimization
- Load balancing and auto-scaling

### 🔧 Developer Experience
- Type-safe transport and event APIs with compile-time validation
- Comprehensive API examples and patterns
- Fluent builder patterns for complex object construction
- Error handling best practices with typed error categories
- Testing utilities and helpers with mock implementations
- Development workflow guidance
- Integration patterns and migration tools

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    AG-UI Go SDK                         │
├─────────────────────────────────────────────────────────┤
│  Type-Safe Events   │  Transport Mgr   │  State Mgmt    │
│  ┌─────────────┐   │  ┌─────────────┐ │  ┌─────────────┐│
│  │ Builder API │   │  │Load Balancer│ │  │ Store       ││
│  │ Validation  │   │  │Health Check │ │  │ Transactions││
│  │ Serializers │   │  │Circuit Break│ │  │ History     ││
│  └─────────────┘   │  └─────────────┘ │  └─────────────┘│
├─────────────────────────────────────────────────────────┤
│  Transport Layer    │  Tools Framework │  Monitoring     │
│  ┌─────────────┐   │  ┌─────────────┐ │  ┌─────────────┐│
│  │ WebSocket   │   │  │ Execution   │ │  │ OpenTel     ││
│  │ HTTP/gRPC   │   │  │ Registry    │ │  │ Prometheus  ││
│  │ Type-Safe   │   │  │ Validation  │ │  │ Jaeger      ││
│  └─────────────┘   │  └─────────────┘ │  └─────────────┘│
└─────────────────────────────────────────────────────────┘
```

## Production Deployment Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Load Balancer │    │   Web Gateway   │    │   Monitoring    │
│     (HAProxy)   │    │     (Nginx)     │    │  (Prometheus)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
    ┌────────────────────────────┼────────────────────────────┐
    │                            │                            │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   AG-UI App 1   │    │   AG-UI App 2   │    │   AG-UI App N   │
│  (Primary Pod)  │    │ (Secondary Pod) │    │   (Scale Pod)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   PostgreSQL    │    │      Redis      │    │ OpenTelemetry   │
│   (Primary)     │    │     (Cache)     │    │   Collector     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Type-Safe Architecture Details

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

## Configuration Examples

### Environment Variables

```bash
# Server Configuration
SERVER_ADDRESS=":8080"
SERVER_READ_TIMEOUT="30s"
SERVER_WRITE_TIMEOUT="30s"

# Database Configuration
DATABASE_URL="postgres://user:pass@localhost/agui"
DATABASE_MAX_OPEN_CONNS="25"
DATABASE_MAX_IDLE_CONNS="5"

# Redis Configuration
REDIS_URL="redis://localhost:6379"
REDIS_PASSWORD="secure-password"

# Security Configuration
JWT_SECRET="your-jwt-secret-key"
ENCRYPTION_KEY="your-encryption-key"
SECURITY_REQUIRE_TLS="true"

# Monitoring Configuration
OTEL_SERVICE_NAME="ag-ui-server"
OTEL_EXPORTER_OTLP_ENDPOINT="http://jaeger:4317"
MONITORING_ENABLED="true"
MONITORING_PROMETHEUS_PORT="9090"

# Logging Configuration
LOG_LEVEL="info"
LOG_FORMAT="json"
```

### Kubernetes Resources

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ag-ui-production
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ag-ui-server
  namespace: ag-ui-production
spec:
  replicas: 3
  selector:
    matchLabels:
      app: ag-ui-server
  template:
    metadata:
      labels:
        app: ag-ui-server
    spec:
      containers:
      - name: ag-ui-server
        image: your-registry/ag-ui-server:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        env:
        - name: ENVIRONMENT
          value: "production"
        envFrom:
        - secretRef:
            name: ag-ui-secrets
```

## Security Checklist

### Pre-deployment Checklist
- [ ] TLS 1.2+ enabled with strong cipher suites
- [ ] JWT secrets securely stored and rotated
- [ ] RBAC policies defined and tested
- [ ] Network policies configured
- [ ] Container security contexts applied
- [ ] Input validation implemented
- [ ] Security headers configured
- [ ] Rate limiting enabled

### Monitoring Checklist
- [ ] OpenTelemetry configured and exporting
- [ ] Prometheus metrics collection active
- [ ] Grafana dashboards deployed
- [ ] Alert rules configured
- [ ] Log aggregation working
- [ ] Trace sampling optimized
- [ ] SLA targets defined

## Performance Benchmarks

### Expected Performance Metrics

| Metric | Target | Monitoring |
|--------|--------|------------|
| Response Time (P95) | < 100ms | Prometheus |
| Throughput | > 1000 req/s | Grafana |
| Error Rate | < 1% | Alertmanager |
| Memory Usage | < 512MB | Kubernetes |
| CPU Usage | < 500m | cAdvisor |

### Optimization Recommendations

1. **Enable Connection Pooling**: Configure appropriate pool sizes
2. **Use Caching**: Implement Redis for frequently accessed data
3. **Optimize Batch Sizes**: Tune OpenTelemetry batch configurations
4. **Monitor Resource Usage**: Set appropriate resource limits
5. **Use Horizontal Scaling**: Configure HPA for automatic scaling

## 🚦 Quick Start Examples

### Basic Transport Usage

```go
import (
    "context"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/transport"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
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
import "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"

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

### Migration from Legacy Code

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

## Support and Troubleshooting

### Common Issues

1. **High Memory Usage**: Check [OpenTelemetry Guide](opentelemetry-monitoring.md#troubleshooting)
2. **Authentication Failures**: Review [Security Configuration](production-deployment.md#authentication-and-authorization)
3. **Performance Issues**: See [Performance Optimization](production-deployment.md#performance-optimization)
4. **Monitoring Problems**: Follow [Troubleshooting Guide](opentelemetry-monitoring.md#troubleshooting)

### Getting Help

- Review the appropriate documentation section
- Check the security and deployment checklists
- Verify configuration against provided examples
- Monitor metrics and logs for diagnostic information

## Contributing to Documentation

### Documentation Standards

- Use clear, concise language
- Provide working code examples
- Include security considerations
- Add monitoring and observability guidance
- Test all code examples
- Update configuration examples

### Maintenance

This documentation is maintained alongside the AG-UI Go SDK codebase. Updates should be made when:

- New APIs are added
- Security practices change
- Deployment patterns evolve
- Monitoring capabilities expand
- Performance optimizations are discovered

---

**Last Updated**: July 2025  
**Version**: 1.0.0  
**Maintained by**: AG-UI Development Team

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
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/transport"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
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
import "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"

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
>