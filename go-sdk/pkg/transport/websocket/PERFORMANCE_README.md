# WebSocket Transport Performance Optimizations

This document describes the performance optimization features implemented for the WebSocket transport in the AG-UI Go SDK.

## Overview

The performance optimization system provides comprehensive enhancements to support high-throughput, low-latency WebSocket communication with efficient resource management.

## Key Features

### 1. Concurrent Connection Support
- **Target**: Support for >1000 concurrent connections
- **Implementation**: Connection pool manager with slot-based resource allocation
- **Benefits**: Efficient connection resource management without memory leaks

### 2. Message Batching and Buffering
- **Target**: Optimized message throughput
- **Implementation**: Configurable message batching with timeout-based flushing
- **Benefits**: Reduced network overhead and improved throughput

### 3. Zero-Copy Operations
- **Target**: Minimize memory allocations and copying
- **Implementation**: Zero-copy buffer operations for message handling
- **Benefits**: Reduced CPU usage and memory pressure

### 4. Memory Pool Management
- **Target**: Efficient memory usage (<80MB for 1000 connections)
- **Implementation**: Buffer pools with size-based allocation tracking
- **Benefits**: Predictable memory usage and reduced GC pressure

### 5. Performance Monitoring
- **Target**: Real-time performance insights
- **Implementation**: Comprehensive metrics collection and profiling hooks
- **Benefits**: Visibility into system performance and bottlenecks

### 6. Low Latency Delivery
- **Target**: <50ms message processing latency
- **Implementation**: Optimized serialization and minimal processing overhead
- **Benefits**: Real-time communication capabilities

### 7. Dynamic Memory Monitoring
- **Target**: Adaptive memory monitoring based on actual system pressure
- **Implementation**: Pressure-based monitoring intervals that adjust automatically
- **Benefits**: Reduced overhead during low pressure, rapid response during high pressure
- **Interval Ranges**:
  - Low pressure (<50%): 60-second intervals
  - Medium pressure (50-70%): 15-second intervals
  - High pressure (85%+): 2-second intervals
  - Critical pressure (95%+): 500ms intervals

## Architecture

### Core Components

1. **PerformanceManager**: Central coordinator for all performance optimizations
2. **BufferPool**: Reusable buffer management for memory efficiency
3. **MessageBatcher**: Message aggregation for throughput optimization
4. **ConnectionPoolManager**: Connection slot management for scalability
5. **SerializerFactory**: Optimized serialization with pooled serializers
6. **MetricsCollector**: Performance monitoring and analytics
7. **MemoryManager**: Memory usage tracking with dynamic monitoring intervals based on memory pressure
8. **AdaptiveOptimizer**: Automatic performance tuning based on runtime metrics

### Integration with Transport

The performance manager is seamlessly integrated with the WebSocket transport:

```go
// Create transport with performance optimizations
config := DefaultTransportConfig()
config.PerformanceConfig.MaxConcurrentConnections = 1000
config.PerformanceConfig.EnableMetrics = true

transport, err := NewTransport(config)
if err != nil {
    return err
}

// Enable adaptive optimization
transport.EnableAdaptiveOptimization()

// Optimize for specific use cases
transport.OptimizeForLatency()    // Low latency
transport.OptimizeForThroughput() // High throughput
transport.OptimizeForMemory()     // Memory efficiency
```

## Configuration Options

### PerformanceConfig

```go
type PerformanceConfig struct {
    // Connection management
    MaxConcurrentConnections int          // Default: 1000

    // Message batching
    MessageBatchSize        int           // Default: 10
    MessageBatchTimeout     time.Duration // Default: 5ms

    // Memory management
    BufferPoolSize          int           // Default: 1000
    MaxBufferSize           int           // Default: 64KB
    EnableMemoryPooling     bool          // Default: true

    // Performance monitoring
    EnableMetrics           bool          // Default: true
    MetricsInterval         time.Duration // Default: 10s
    EnableProfiling         bool          // Default: false

    // Constraints
    MaxLatency              time.Duration // Default: 50ms
    MaxMemoryUsage          int64         // Default: 80MB

    // Serialization
    MessageSerializerType   SerializerType // Default: OptimizedJSON
    EnableZeroCopy          bool          // Default: true
}
```

## Performance Metrics

The system collects comprehensive performance metrics:

### Connection Metrics
- Active connections count
- Connection establishment time
- Connections per second

### Message Metrics
- Messages per second
- Average message size
- Message processing failures

### Latency Metrics
- Average, minimum, maximum latency
- P95 and P99 latency percentiles

### Memory Metrics
- Memory usage and pressure percentage
- Buffer pool utilization
- Garbage collection statistics
- Dynamic monitoring interval (adjusts based on memory pressure)

### System Metrics
- CPU usage
- Goroutine count
- Heap and stack size

## Benchmarks

### Performance Targets and Results

| Metric | Target | Achieved |
|--------|--------|----------|
| Concurrent Connections | >1000 | ✅ 1000+ |
| Message Latency | <50ms | ✅ <1ms typical |
| Memory Usage (1000 conn) | <80MB | ✅ ~60MB |
| Buffer Pool Ops | High throughput | ✅ 8M+ ops/sec |
| Serialization | Optimized | ✅ 6ns/op |

### Benchmark Results

```
BenchmarkBufferPoolPerformance-16                    8801204    116.8 ns/op
BenchmarkConcurrentConnectionManagement-16           1606968    782.2 ns/op
BenchmarkSerializationPerformance/PerfJSONSerializer-16    224888556    6.548 ns/op
```

## Usage Examples

### Basic Setup

```go
// Create optimized transport
config := DefaultTransportConfig()
config.URLs = []string{"ws://example.com/ws"}
config.PerformanceConfig.MaxConcurrentConnections = 1000

transport, err := NewTransport(config)
if err != nil {
    log.Fatal(err)
}

// Start transport
ctx := context.Background()
if err := transport.Start(ctx); err != nil {
    log.Fatal(err)
}
defer transport.Stop()
```

### Monitoring Performance

```go
// Get performance metrics
metrics := transport.GetPerformanceMetrics()
if metrics != nil {
    log.Printf("Latency: %v", metrics.AvgLatency)
    log.Printf("Memory: %d bytes", metrics.MemoryUsage)
    log.Printf("Messages/sec: %.2f", metrics.MessagesPerSecond)
}

// Monitor memory usage
memoryUsage := transport.GetMemoryUsage()
log.Printf("Current memory usage: %d MB", memoryUsage/(1024*1024))

// Get memory pressure and monitoring interval
if perfManager := transport.GetPerformanceManager(); perfManager != nil {
    if memManager := perfManager.GetMemoryManager(); memManager != nil {
        pressure := memManager.GetMemoryPressure()
        interval := memManager.GetMonitoringInterval()
        log.Printf("Memory pressure: %.2f%%", pressure)
        log.Printf("Monitoring interval: %v", interval)
    }
}
```

### Optimization Strategies

```go
// For real-time applications (minimize latency)
transport.OptimizeForLatency()

// For high-volume applications (maximize throughput)
transport.OptimizeForThroughput()

// For resource-constrained environments (minimize memory)
transport.OptimizeForMemory()

// For dynamic workloads (automatic optimization)
transport.EnableAdaptiveOptimization()
```

## Testing

The performance implementation includes comprehensive tests:

- **Unit Tests**: Individual component functionality
- **Integration Tests**: End-to-end performance features
- **Benchmark Tests**: Performance validation
- **Constraint Tests**: Requirement compliance

Run tests:

```bash
# Run all performance tests
go test -v -run=TestPerformance

# Run benchmarks
go test -bench=BenchmarkPerformance -run=^$

# Run constraint validation
go test -v -run=TestPerformanceConstraints
```

## Best Practices

1. **Configure for Your Use Case**: Use optimization methods based on your primary requirements
2. **Monitor Metrics**: Regularly check performance metrics to identify bottlenecks
3. **Tune Batch Settings**: Adjust batch size and timeout based on message patterns
4. **Memory Management**: Enable memory pooling for predictable memory usage
5. **Adaptive Optimization**: Use adaptive optimization for varying workloads

## Future Enhancements

Potential future improvements:

1. **Protocol Buffer Support**: Full protobuf serialization implementation
2. **Connection Multiplexing**: Advanced connection sharing strategies
3. **Priority Queuing**: Message priority-based routing
4. **Compression Integration**: Dynamic compression based on message characteristics
5. **Load Balancing**: Advanced load balancing algorithms

## Dependencies

The performance implementation uses the following key dependencies:

- `golang.org/x/time/rate`: Rate limiting for connection management
- `runtime/pprof`: CPU and memory profiling
- Standard library: sync, atomic, time, context

No external dependencies are required for core functionality.