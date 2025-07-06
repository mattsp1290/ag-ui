# State Management Examples

This directory contains comprehensive examples demonstrating various aspects of the AG-UI state management system. Each example showcases different use cases and best practices for state synchronization, collaboration, and distributed systems.

## Examples Overview

### 1. Basic State Synchronization (`basic_state_sync.go`)

Demonstrates fundamental state management concepts including:
- Creating and managing a state store
- Subscribing to state changes at different paths
- Using transactions for atomic updates
- Creating and restoring snapshots
- Event-based synchronization with snapshots and deltas
- State history tracking and versioning
- Import/export functionality

**Key concepts:**
- State store initialization
- Path-based subscriptions
- JSON Patch operations
- Transaction management
- Event handlers

**Run:**
```bash
go run basic_state_sync.go
```

### 2. Collaborative Editing (`collaborative_editing.go`)

Shows how multiple users can edit shared state concurrently with:
- Real-time collaborative document editing
- Conflict detection and resolution strategies
- User presence tracking
- Different resolution strategies (LastWriteWins, FirstWriteWins, Merge)
- Handling network partitions during collaboration

**Key concepts:**
- Multi-user state synchronization
- Conflict resolution strategies
- Event-based collaboration
- User session management

**Run:**
```bash
go run collaborative_editing.go
```

### 3. Real-time Dashboard (`realtime_dashboard.go`)

Demonstrates high-frequency state updates for real-time monitoring:
- Multiple data sources updating at different frequencies
- Efficient batching and throttling of updates
- Performance optimization techniques
- Delta compression for bandwidth efficiency
- Time series data management
- Alert generation and activity feeds

**Key concepts:**
- High-frequency updates (10Hz+)
- Batch processing
- State streaming
- Performance monitoring
- Circular buffers for time series

**Run:**
```bash
go run realtime_dashboard.go
```

### 4. Distributed State (`distributed_state.go`)

Shows state synchronization across multiple distributed nodes:
- Multi-node cluster setup
- Leader election mechanisms
- Network partition handling
- Eventual consistency patterns
- Data locality optimization
- Rolling updates across nodes

**Key concepts:**
- Distributed consensus
- Network simulation
- Partition tolerance
- State reconciliation
- Regional data management

**Run:**
```bash
go run distributed_state.go
```

## Production Examples

### 5. Storage Backends (`storage_backends_example.go`)

Demonstrates production storage backends for state persistence:
- **File Storage**: Local file-based storage with compression and encryption
- **Redis Storage**: High-performance in-memory storage with persistence
- **PostgreSQL Storage**: Durable relational storage with versioning
- **Hybrid Storage**: Primary/fallback configuration for resilience
- Performance benchmarking and comparison

**Key features:**
- Storage backend configuration
- Compression (gzip, snappy, zstd)
- Encryption at rest
- Connection pooling and retry policies
- Performance metrics

**Run:**
```bash
go run storage_backends_example.go
```

### 6. Monitoring & Observability (`monitoring_observability_example.go`)

Comprehensive monitoring and observability features:
- **Metrics Collection**: Prometheus integration with custom metrics
- **Structured Logging**: JSON logging with field extraction
- **Distributed Tracing**: OpenTelemetry/Jaeger integration
- **Health Checks**: Liveness and readiness probes
- **Alerting**: Threshold-based alerts with multiple notifiers
- **Performance Profiling**: CPU and memory profiling
- **Real-time Dashboards**: Live metrics visualization

**Key features:**
- Prometheus metrics endpoint
- Health check HTTP endpoint
- Alert management
- Resource monitoring
- Performance analytics

**Run:**
```bash
go run monitoring_observability_example.go
```

### 7. Enhanced Event Handlers (`enhanced_event_handlers_example.go`)

Production-ready event handling with advanced features:
- **Event Compression**: Automatic compression for large events
- **Connection Resilience**: Retry logic and exponential backoff
- **Advanced Synchronization**: Cross-client coordination
- **Batch Optimization**: Configurable batching strategies
- **Event Ordering**: Sequence tracking and out-of-order handling
- **Backpressure Management**: Queue management and flow control

**Key features:**
- Compression benchmarking
- Network simulation
- Resilience testing
- Performance optimization
- Event ordering guarantees

**Run:**
```bash
go run enhanced_event_handlers_example.go
```

### 8. Performance Optimization (`performance_optimization_example.go`)

High-scale performance optimization techniques:
- **Object Pooling**: Reduce allocations with object reuse
- **State Sharding**: Distributed locking for concurrent access
- **Batch Processing**: Optimize throughput with batching
- **Lazy Loading**: On-demand loading with intelligent caching
- **Memory Optimization**: Compression and efficient data structures
- **Comprehensive Benchmarks**: Multiple workload scenarios

**Key features:**
- Allocation profiling
- Sharding strategies
- Cache management
- Memory usage optimization
- Performance benchmarking

**Run:**
```bash
go run performance_optimization_example.go
```

### 9. Enhanced Collaborative Editing (`enhanced_collaborative_editing.go`)

Production-ready collaborative editing with all new features:
- **Storage Integration**: Redis backend for persistence
- **Performance Optimization**: Pooling, sharding, and batching
- **Network Resilience**: Simulated network conditions
- **Monitoring Integration**: Real-time metrics and alerts
- **Section Locking**: Collaborative coordination features
- **Comprehensive Analytics**: Usage tracking and performance metrics

**Key features:**
- Production storage backends
- Network simulation
- Performance under load
- Conflict resolution at scale
- Session analytics

**Run:**
```bash
go run enhanced_collaborative_editing.go
```

## Best Practices Demonstrated

### 1. State Organization
- Use hierarchical paths for logical organization
- Keep related data close in the state tree
- Use meaningful path names for clarity
- Implement proper data schemas for consistency

### 2. Performance Optimization
- Enable object pooling to reduce allocations
- Use state sharding for concurrent access
- Implement batching for bulk operations
- Configure lazy loading for large datasets
- Use appropriate compression algorithms
- Monitor performance metrics continuously

### 3. Storage Configuration
- Choose storage backend based on requirements:
  - File storage for development/small deployments
  - Redis for high-performance caching
  - PostgreSQL for durability and complex queries
- Configure compression and encryption appropriately
- Implement retry policies and connection pooling
- Use hybrid storage for resilience

### 4. Monitoring & Observability
- Enable Prometheus metrics collection
- Implement structured logging
- Use distributed tracing for debugging
- Configure health checks and alerts
- Monitor resource usage and performance
- Track business metrics alongside technical metrics

### 5. Event Handling
- Configure compression for large events
- Implement retry logic with exponential backoff
- Handle out-of-order events properly
- Use batching to optimize throughput
- Implement backpressure management
- Monitor connection health

### 6. Conflict Resolution
- Choose appropriate resolution strategy for your use case
- Implement custom resolvers for complex scenarios
- Track conflict history for debugging
- Design state structure to minimize conflicts
- Use optimistic locking where appropriate

### 7. Error Handling
- Always check errors from state operations
- Implement rollback mechanisms for failed transactions
- Use snapshots for recovery scenarios
- Monitor and log state synchronization errors
- Implement circuit breakers for external dependencies

### 8. Scalability
- Use event streaming for real-time updates
- Implement proper batching for bulk operations
- Consider data locality in distributed systems
- Monitor performance metrics
- Use connection pooling effectively
- Implement gradual rollouts for changes

## Production Deployment Checklist

### Infrastructure
- [ ] Storage backend configured and tested
- [ ] Monitoring endpoints exposed
- [ ] Health check endpoints configured
- [ ] Alert notifications set up
- [ ] Backup and recovery procedures in place

### Performance
- [ ] Object pooling enabled
- [ ] Appropriate batch sizes configured
- [ ] Compression enabled for large data
- [ ] Connection pools sized correctly
- [ ] Rate limiting configured

### Reliability
- [ ] Retry policies configured
- [ ] Circuit breakers implemented
- [ ] Timeout values set appropriately
- [ ] Graceful shutdown implemented
- [ ] Error recovery procedures tested

### Security
- [ ] Encryption at rest enabled
- [ ] TLS for network communication
- [ ] Access controls implemented
- [ ] Audit logging enabled
- [ ] Sensitive data handling reviewed

### Monitoring
- [ ] Metrics collection enabled
- [ ] Log aggregation configured
- [ ] Distributed tracing set up
- [ ] Alerts configured with appropriate thresholds
- [ ] Dashboards created for key metrics

## Common Patterns

### Storage Backend Configuration
```go
storageConfig := &state.StorageConfig{
    Type:              state.StorageTypeRedis,
    ConnectionURL:     "redis://localhost:6379/0",
    MaxConnections:    50,
    ConnectionTimeout: 10 * time.Second,
    Compression: state.CompressionConfig{
        Enabled:       true,
        Algorithm:     "snappy",
        Level:         1,
        MinSizeBytes:  512,
    },
}
storage, err := state.NewRedisBackend(storageConfig, logger)
```

### Performance Optimization
```go
perfOptions := state.PerformanceOptions{
    EnablePooling:      true,
    EnableBatching:     true,
    EnableCompression:  true,
    EnableLazyLoading:  true,
    EnableSharding:     true,
    BatchSize:          100,
    BatchTimeout:       10 * time.Millisecond,
    ShardCount:         16,
    MaxConcurrency:     runtime.NumCPU() * 2,
}
optimizer := state.NewPerformanceOptimizer(perfOptions)
```

### Monitoring Setup
```go
monitoringConfig := &state.MonitoringConfig{
    EnablePrometheus:    true,
    MetricsEnabled:      true,
    EnableHealthChecks:  true,
    EnableTracing:       true,
    TracingServiceName:  "my-service",
    AlertThresholds: state.AlertThresholds{
        ErrorRate:        5.0,
        P95LatencyMs:     100,
        MemoryUsagePercent: 80,
    },
}
monitor := state.NewStateMonitor(store, monitoringConfig)
```

### Resilient Event Handler
```go
handler := state.NewStateEventHandler(store,
    state.WithCompressionThreshold(1024),
    state.WithMaxRetries(3),
    state.WithRetryDelay(100*time.Millisecond),
    state.WithBatchSize(50),
    state.WithBatchTimeout(100*time.Millisecond),
    state.WithConnectionHealth(health),
)
```

## Performance Benchmarks

Based on the examples, typical performance characteristics:

- **Sequential Operations**: 10,000-50,000 ops/sec
- **Concurrent Operations**: 50,000-200,000 ops/sec with sharding
- **Network Latency**: <1ms local, 10-100ms distributed
- **Compression Ratios**: 40-80% reduction for typical JSON data
- **Memory Usage**: 50-80% reduction with pooling
- **Cache Hit Rates**: 80-95% with proper configuration

## Testing

Each example can be run independently and includes:
- Console output showing operations
- Performance metrics
- Error scenarios
- Recovery demonstrations

To run all examples:
```bash
# Basic examples
go run basic_state_sync.go
go run collaborative_editing.go
go run realtime_dashboard.go
go run distributed_state.go

# Production examples
go run storage_backends_example.go
go run monitoring_observability_example.go
go run enhanced_event_handlers_example.go
go run performance_optimization_example.go
go run enhanced_collaborative_editing.go
```

## Dependencies

All examples use the AG-UI Go SDK state management package:
```go
import (
    "github.com/ag-ui/go-sdk/pkg/state"
    "github.com/ag-ui/go-sdk/pkg/core/events"
)
```

Additional dependencies for production features:
- Prometheus client for metrics
- Structured logging (zap)
- OpenTelemetry for tracing

## Further Reading

- See the main state package documentation in `/pkg/state/doc.go`
- Review the event system documentation in `/pkg/core/events/doc.go`
- Check the integration tests for more usage examples
- Review the performance benchmarks in `/pkg/state/benchmark_test.go`
- See monitoring configuration in `/pkg/state/monitoring_config.go`