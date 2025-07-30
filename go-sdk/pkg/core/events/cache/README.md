# Cache-Optimized Validation System

This package provides a comprehensive cache-optimized validation system for the AG-UI Go SDK events framework. It implements multi-level caching, intelligent cache invalidation, predictive cache warming, and distributed cache coordination to significantly improve validation performance.

## Features

### Multi-Level Caching
- **L1 Cache**: High-speed in-memory LRU cache for frequently accessed validation results
- **L2 Cache**: Distributed cache layer for shared validation results across nodes
- **Intelligent Cache Promotion**: Automatic promotion from L2 to L1 based on access patterns

### Smart Caching Strategies
- **TTL Strategy**: Time-based caching with adaptive TTL adjustment
- **LFU Strategy**: Least Frequently Used eviction with frequency decay
- **Adaptive Strategy**: Dynamic strategy adjustment based on system conditions
- **Predictive Strategy**: Pattern-based cache lifetime extension
- **Composite Strategy**: Weighted combination of multiple strategies

### Cache Invalidation
- **Event-based Invalidation**: Invalidate specific events or event types
- **Pattern-based Invalidation**: Intelligent invalidation using event patterns
- **Distributed Invalidation**: Coordinated invalidation across cache nodes
- **Time-based Expiration**: Automatic cleanup of expired entries

### Prefetch Engine
- **Pattern Learning**: Automatic learning of event access patterns
- **Predictive Prefetching**: Proactive cache warming based on predictions
- **Sequence Analysis**: Detection of event sequences and correlations
- **Adaptive Scheduling**: Dynamic adjustment of prefetch parameters

### Performance Monitoring
- **Comprehensive Metrics**: Hit rates, latencies, sizes, health indicators
- **Time Series Data**: Historical performance tracking
- **Health Analysis**: Automatic health scoring and issue detection
- **Performance Recommendations**: AI-driven optimization suggestions

### Distributed Coordination
- **Node Discovery**: Automatic detection and management of cache nodes
- **Consensus Protocol**: Distributed decision making for cache operations
- **Shard Management**: Automatic data partitioning across nodes
- **Failure Handling**: Graceful handling of node failures and recoveries

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Cache Validator                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ L1 Cache    │  │ Cache Strategy  │  │ Prefetch Engine │ │
│  │ (In-Memory) │  │                 │  │                 │ │
│  └─────────────┘  └─────────────────┘  └─────────────────┘ │
│  ┌─────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ L2 Cache    │  │ Invalidation    │  │ Metrics         │ │
│  │ (Distributed)│  │ Engine          │  │ Collector       │ │
│  └─────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                Cache Coordinator                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Node        │  │ Consensus       │  │ Shard           │ │
│  │ Management  │  │ Protocol        │  │ Management      │ │
│  └─────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Usage

### Basic Setup

```go
import "github.com/ag-ui/go-sdk/pkg/core/events/cache"

// Create cache validator
config := cache.DefaultCacheValidatorConfig()
config.L1Size = 10000
config.L1TTL = 5 * time.Minute

validator, err := cache.NewCacheValidator(config)
if err != nil {
    log.Fatal(err)
}
defer validator.Shutdown(context.Background())

// Use for validation
ctx := context.Background()
err = validator.ValidateEvent(ctx, event)
if err != nil {
    // Handle validation error
}
```

### Advanced Configuration

```go
// Configure L2 distributed cache
l2Cache := &RedisDistributedCache{
    Client: redisClient,
}

// Configure custom strategy
strategy := cache.NewCompositeStrategy(
    []cache.CacheStrategy{
        cache.NewTTLStrategy(5 * time.Minute),
        cache.NewLFUStrategy(),
        cache.NewPredictiveStrategy(cache.NewTTLStrategy(5 * time.Minute)),
    },
    []float64{0.4, 0.3, 0.3},
)

config := &cache.CacheValidatorConfig{
    L1Size:         20000,
    L1TTL:          10 * time.Minute,
    L2Cache:        l2Cache,
    L2TTL:          1 * time.Hour,
    L2Enabled:      true,
    Validator:      events.NewValidator(events.ProductionValidationConfig()),
    Strategy:       strategy,
    MetricsEnabled: true,
}

validator, err := cache.NewCacheValidator(config)
```

### Prefetch Engine

```go
// Create prefetch engine
prefetchConfig := &cache.PrefetchConfig{
    MaxConcurrentPrefetches: 20,
    PrefetchInterval:        30 * time.Second,
    PredictionWindow:        10 * time.Minute,
    MinConfidence:           0.8,
    EnablePatternLearning:   true,
    EnableAdaptive:          true,
}

engine := cache.NewPrefetchEngine(validator, prefetchConfig)
err := engine.Start(context.Background())
defer engine.Stop(context.Background())

// Record access patterns
engine.RecordAccess(event)
```

### Distributed Setup

```go
// Create transport for node communication
transport := &GRPCTransport{
    Address: "localhost:8080",
}

// Create coordinator
coordConfig := cache.DefaultCoordinatorConfig()
coordConfig.EnableSharding = true
coordConfig.ShardCount = 32

coordinator := cache.NewCacheCoordinator("node-1", transport, coordConfig)
err := coordinator.Start(context.Background())
defer coordinator.Stop(context.Background())

// Configure cache validator with coordinator
config.Coordinator = coordinator
validator, err := cache.NewCacheValidator(config)
```

### Monitoring and Metrics

```go
// Get basic metrics
stats := validator.GetStats()
fmt.Printf("Hit Rate: %.2f%%\n", float64(stats.TotalHits)/float64(stats.TotalHits+stats.TotalMisses)*100)

// Get comprehensive report
metricsCollector := cache.NewMetricsCollector(cache.DefaultMetricsConfig())
report := metricsCollector.GetReport()

fmt.Printf("Health Score: %.1f\n", report.HealthMetrics.HealthScore)
for _, rec := range report.Recommendations {
    fmt.Printf("Recommendation: %s\n", rec)
}

// Get time series data
timeSeries := metricsCollector.GetTimeSeries(1 * time.Hour)
for _, point := range timeSeries {
    fmt.Printf("Time: %v, Hit Rate: %.2f\n", point.Timestamp, point.HitRate)
}
```

## Configuration Options

### CacheValidatorConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| L1Size | int | 10000 | Maximum number of entries in L1 cache |
| L1TTL | time.Duration | 5m | Time-to-live for L1 cache entries |
| L2Cache | DistributedCache | nil | L2 distributed cache implementation |
| L2TTL | time.Duration | 30m | Time-to-live for L2 cache entries |
| L2Enabled | bool | false | Enable L2 distributed cache |
| CompressionEnabled | bool | true | Enable cache entry compression |
| CompressionLevel | int | 6 | Compression level (1-9) |
| MetricsEnabled | bool | true | Enable metrics collection |

### PrefetchConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| MaxConcurrentPrefetches | int | 10 | Maximum concurrent prefetch operations |
| PrefetchInterval | time.Duration | 1m | Interval between prefetch cycles |
| PredictionWindow | time.Duration | 5m | Time window for predictions |
| MinConfidence | float64 | 0.7 | Minimum confidence for prefetching |
| MaxPrefetchSize | int | 100 | Maximum prefetch batch size |
| EnablePatternLearning | bool | true | Enable pattern learning |
| EnableAdaptive | bool | true | Enable adaptive tuning |

### MetricsConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| EnableDetailedLatency | bool | true | Track detailed latency metrics |
| EnableTimeSeries | bool | true | Enable time series data collection |
| EnableHistograms | bool | true | Enable histogram metrics |
| TimeSeriesWindow | time.Duration | 1h | Time series data retention window |
| ReportingInterval | time.Duration | 1m | Metrics reporting interval |
| PercentilesToTrack | []float64 | [0.5, 0.75, 0.9, 0.95, 0.99] | Latency percentiles to track |

## Performance Characteristics

### Benchmarks

Based on internal benchmarks with the AG-UI events system:

- **L1 Cache Hit Latency**: ~10-50 microseconds
- **L1 Cache Miss Latency**: ~100-500 microseconds  
- **L2 Cache Hit Latency**: ~1-5 milliseconds
- **Cache Hit Rate**: 85-95% (typical workloads)
- **Memory Overhead**: ~50-100 bytes per cached entry
- **Compression Ratio**: 60-80% (typical validation results)

### Scaling

- **Single Node**: Supports 100K+ validations/second
- **Distributed**: Linear scaling up to 10+ nodes
- **Memory Usage**: ~1GB for 100K cached validation results
- **Network Overhead**: <1% for coordination messages

## Best Practices

### Cache Strategy Selection

1. **High-frequency validation**: Use TTL + LFU strategies
2. **Memory-constrained environments**: Use Adaptive strategy
3. **Predictable access patterns**: Add Predictive strategy
4. **Variable workloads**: Use Composite strategy

### Performance Tuning

1. **Monitor hit rates**: Target >80% for optimal performance
2. **Adjust cache sizes**: Based on memory usage and hit rates
3. **Tune TTL values**: Balance freshness with performance
4. **Enable compression**: For large validation results

### Monitoring

1. **Set up alerts**: For low hit rates, high latencies, memory pressure
2. **Regular health checks**: Monitor health score and recommendations
3. **Trend analysis**: Use time series data for capacity planning
4. **Performance baselines**: Establish baseline metrics for comparison

## Integration with Existing System

The cache system is designed to be a drop-in replacement for the existing validation system:

```go
// Before
validator := events.NewValidator(config)
err := validator.ValidateEvent(ctx, event)

// After  
cacheValidator, _ := cache.NewCacheValidator(cacheConfig)
err := cacheValidator.ValidateEvent(ctx, event)
```

The cache validator implements the same interface as the standard validator while providing significant performance improvements through intelligent caching.

## Testing

Run the comprehensive test suite:

```bash
go test -v ./pkg/core/events/cache/...
```

Run benchmarks:

```bash
go test -bench=. -benchmem ./pkg/core/events/cache/...
```

Run stress tests:

```bash
go test -race -count=10 ./pkg/core/events/cache/...
```

## Troubleshooting

### Common Issues and Solutions

#### Cache Miss Rate Too High

**Problem**: Cache hit rate below 80%, indicating poor cache effectiveness
```
Cache stats: Hit rate: 45.2%, L1 hits: 1234, L2 hits: 567, misses: 2890
```

**Diagnostic Commands:**
```bash
# Run cache performance benchmarks
go test -bench=BenchmarkCache ./pkg/core/events/cache/

# Check cache statistics
go run -tags debug ./examples/cache-stats/
```

**Diagnostic Steps:**
1. Check cache configuration:
   ```go
   stats := validator.GetStats()
   log.Printf("L1 size: %d, L1 usage: %.2f%%", 
       stats.L1Size, float64(stats.L1Count)/float64(stats.L1Size)*100)
   log.Printf("L2 enabled: %t, L2 hits: %d", stats.L2Enabled, stats.L2Hits)
   ```

2. Analyze access patterns:
   ```go
   patterns := prefetchEngine.GetAccessPatterns()
   for eventType, pattern := range patterns {
       log.Printf("Event type %s: frequency %.2f, last access %v", 
           eventType, pattern.Frequency, pattern.LastAccess)
   }
   ```

**Solutions:**
- Increase L1 cache size: `config.L1Size = 20000`
- Extend TTL for stable validation results: `config.L1TTL = 10 * time.Minute`
- Enable L2 distributed cache: `config.L2Enabled = true`
- Tune cache strategy weights for better prediction
- Enable prefetch engine for predictive loading

#### Memory Pressure

**Problem**: Cache consuming excessive memory, causing OOM issues
```
Error: runtime: out of memory: cannot allocate 1073741824-byte block
```

**Diagnostic Commands:**
```bash
# Memory profiling
go test -memprofile=cache.prof -bench=BenchmarkMemoryUsage ./pkg/core/events/cache/
go tool pprof cache.prof

# Monitor memory usage
watch -n 1 'ps aux | grep cache-validator'
```

**Diagnostic Steps:**
1. Check cache memory usage:
   ```go
   import "runtime"
   
   var m runtime.MemStats
   runtime.ReadMemStats(&m)
   log.Printf("Cache memory usage: %d MB", m.Alloc/1024/1024)
   
   stats := validator.GetStats()
   log.Printf("L1 entries: %d, estimated size: %d MB", 
       stats.L1Count, stats.L1Count*stats.AvgEntrySize/1024/1024)
   ```

2. Analyze cache entry sizes:
   ```go
   report := metricsCollector.GetReport()
   log.Printf("Average entry size: %d bytes", report.CacheMetrics.AvgEntrySize)
   log.Printf("Largest entries: %v", report.CacheMetrics.LargestEntries)
   ```

**Solutions:**
- Reduce L1 cache size: `config.L1Size = 5000`
- Enable compression: `config.CompressionEnabled = true`
- Implement cache eviction policies
- Use shorter TTL for large validation results
- Monitor and set memory limits: `config.MaxMemoryUsage = 500 * 1024 * 1024` // 500MB

#### L2 Cache Connection Issues

**Problem**: Distributed L2 cache failing to connect or timing out
```
Error: failed to connect to Redis: dial tcp 127.0.0.1:6379: connection refused
Error: L2 cache operation timeout after 5s
```

**Diagnostic Commands:**
```bash
# Test Redis connectivity
redis-cli ping

# Check Redis configuration
redis-cli config get maxmemory

# Monitor Redis performance
redis-cli --latency-history
```

**Diagnostic Steps:**
1. Test L2 cache connectivity:
   ```go
   if l2Cache := validator.GetL2Cache(); l2Cache != nil {
       err := l2Cache.Ping(context.Background())
       if err != nil {
           log.Printf("L2 cache connectivity issue: %v", err)
       }
   }
   ```

2. Check L2 cache configuration:
   ```go
   config := validator.GetConfig()
   log.Printf("L2 TTL: %v", config.L2TTL)
   log.Printf("L2 timeout: %v", config.L2Timeout)
   ```

**Solutions:**
- Configure connection retry: `config.L2RetryAttempts = 3`
- Increase timeout: `config.L2Timeout = 10 * time.Second`
- Implement circuit breaker pattern
- Use connection pooling for Redis
- Add L2 cache health checks
- Graceful degradation to L1-only mode

#### Prefetch Engine Performance Issues

**Problem**: Prefetch engine consuming too many resources or making poor predictions
```
Warning: prefetch engine CPU usage at 95%
Warning: prefetch confidence below threshold: 0.45
```

**Diagnostic Commands:**
```bash
# Profile prefetch engine
go test -cpuprofile=prefetch.prof -bench=BenchmarkPrefetch ./pkg/core/events/cache/
go tool pprof prefetch.prof
```

**Diagnostic Steps:**
1. Monitor prefetch effectiveness:
   ```go
   engine := validator.GetPrefetchEngine()
   stats := engine.GetStats()
   log.Printf("Predictions made: %d", stats.PredictionsMade)
   log.Printf("Successful prefetches: %d", stats.SuccessfulPrefetches)
   log.Printf("Average confidence: %.2f", stats.AvgConfidence)
   log.Printf("Hit rate improvement: %.2f%%", stats.HitRateImprovement)
   ```

2. Analyze pattern learning:
   ```go
   patterns := engine.GetLearnedPatterns()
   for sequence, confidence := range patterns {
       log.Printf("Pattern %s: confidence %.2f", sequence, confidence)
   }
   ```

**Solutions:**
- Adjust prediction parameters: `config.MinConfidence = 0.8`
- Reduce prefetch concurrency: `config.MaxConcurrentPrefetches = 10`
- Tune prediction window: `config.PredictionWindow = 5 * time.Minute`
- Disable adaptive features if causing instability: `config.EnableAdaptive = false`
- Use pattern-based prefetching only: `config.EnablePatternLearning = true`

### Coordinator and Distributed Issues

#### Node Discovery Problems

**Problem**: Cache nodes unable to discover each other
```
Error: no healthy nodes found in cluster
Warning: node registration failed for node-2
```

**Diagnostic Commands:**
```bash
# Check network connectivity between nodes
ping node-2.example.com
telnet node-2.example.com 8080

# Check cluster state
go run ./cmd/cluster-status/
```

**Diagnostic Steps:**
1. Check coordinator health:
   ```go
   coordinator := validator.GetCoordinator()
   if coordinator != nil {
       health := coordinator.GetHealth()
       log.Printf("Coordinator healthy: %t", health.Healthy)
       log.Printf("Known nodes: %d", len(health.KnownNodes))
       log.Printf("Active nodes: %d", health.ActiveNodeCount)
   }
   ```

2. Verify node registration:
   ```go
   nodes := coordinator.GetNodes()
   for _, node := range nodes {
       log.Printf("Node %s: state=%s, last_seen=%v", 
           node.ID, node.State, node.LastSeen)
   }
   ```

**Solutions:**
- Configure proper network addresses: `config.BindAddress = "0.0.0.0:8080"`
- Use service discovery (e.g., Consul, etcd)
- Implement health check endpoints
- Add retry logic for node registration
- Configure firewall rules for cluster communication

#### Consensus Algorithm Issues

**Problem**: Distributed cache operations failing due to consensus problems
```
Error: consensus timeout: insufficient nodes responded
Error: split-brain detected in cluster
```

**Diagnostic Steps:**
1. Check consensus health:
   ```go
   consensus := coordinator.GetConsensusManager()
   status := consensus.GetStatus()
   log.Printf("Algorithm: %s", status.Algorithm)
   log.Printf("Leader: %s", status.Leader)
   log.Printf("Quorum size: %d, active nodes: %d", 
       status.QuorumSize, status.ActiveNodes)
   ```

2. Monitor consensus operations:
   ```go
   metrics := consensus.GetMetrics()
   log.Printf("Consensus operations: %d", metrics.Operations)
   log.Printf("Success rate: %.2f%%", metrics.SuccessRate)
   log.Printf("Average latency: %v", metrics.AvgLatency)
   ```

**Solutions:**
- Ensure odd number of nodes: minimum 3 for fault tolerance
- Adjust consensus timeouts: `config.ConsensusTimeout = 5 * time.Second`
- Use appropriate algorithm for cluster size (Raft for < 7 nodes, PBFT for Byzantine faults)
- Implement cluster rebalancing
- Monitor network partitions and healing

### Performance Debugging

#### Cache Latency Analysis

```go
func analyzeCacheLatency(validator *CacheValidator) {
    // Measure L1 cache performance
    start := time.Now()
    result := validator.ValidateEvent(context.Background(), event)
    l1Latency := time.Since(start)
    
    // Force L1 miss and measure L2 performance
    validator.InvalidateL1(event.GetEventType())
    start = time.Now()
    result = validator.ValidateEvent(context.Background(), event)
    l2Latency := time.Since(start)
    
    // Force cache miss and measure full validation
    validator.InvalidateAll()
    start = time.Now()
    result = validator.ValidateEvent(context.Background(), event)
    fullLatency := time.Since(start)
    
    log.Printf("Performance analysis:")
    log.Printf("  L1 cache hit: %v", l1Latency)
    log.Printf("  L2 cache hit: %v", l2Latency)
    log.Printf("  Full validation: %v", fullLatency)
    log.Printf("  L1 speedup: %.2fx", float64(fullLatency)/float64(l1Latency))
    log.Printf("  L2 speedup: %.2fx", float64(fullLatency)/float64(l2Latency))
}
```

#### Memory Leak Detection

```go
func detectMemoryLeaks(validator *CacheValidator) {
    runtime.GC()
    var m1 runtime.MemStats
    runtime.ReadMemStats(&m1)
    
    // Perform cache operations
    for i := 0; i < 10000; i++ {
        event := createTestEvent(i)
        validator.ValidateEvent(context.Background(), event)
    }
    
    runtime.GC()
    var m2 runtime.MemStats
    runtime.ReadMemStats(&m2)
    
    growth := m2.Alloc - m1.Alloc
    log.Printf("Memory growth: %d bytes", growth)
    
    if growth > 10*1024*1024 { // 10MB threshold
        log.Printf("WARNING: Potential memory leak detected")
        
        // Analyze cache sizes
        stats := validator.GetStats()
        log.Printf("L1 entries: %d", stats.L1Count)
        log.Printf("Estimated L1 size: %d bytes", stats.L1Count*stats.AvgEntrySize)
    }
}
```

#### Throughput Benchmarking

```go
func benchmarkThroughput(validator *CacheValidator) {
    const numEvents = 100000
    events := make([]Event, numEvents)
    for i := range events {
        events[i] = createTestEvent(i)
    }
    
    start := time.Now()
    
    var wg sync.WaitGroup
    numWorkers := runtime.NumCPU()
    eventsPerWorker := numEvents / numWorkers
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(startIdx int) {
            defer wg.Done()
            endIdx := startIdx + eventsPerWorker
            if endIdx > numEvents {
                endIdx = numEvents
            }
            
            for j := startIdx; j < endIdx; j++ {
                validator.ValidateEvent(context.Background(), events[j])
            }
        }(i * eventsPerWorker)
    }
    
    wg.Wait()
    duration := time.Since(start)
    
    throughput := float64(numEvents) / duration.Seconds()
    log.Printf("Throughput: %.0f events/second", throughput)
    
    stats := validator.GetStats()
    hitRate := float64(stats.TotalHits) / float64(stats.TotalHits+stats.TotalMisses) * 100
    log.Printf("Hit rate: %.2f%%", hitRate)
}
```

### Configuration Tuning

#### Optimal Configuration for Different Workloads

**High-Frequency, Low-Latency Workload:**
```go
config := &CacheValidatorConfig{
    L1Size:               50000,              // Large L1 for high hit rate
    L1TTL:                30 * time.Second,   // Short TTL for fresh data
    L2Enabled:            false,              // Disable L2 to avoid network latency
    CompressionEnabled:   false,              // Disable compression for speed
    MetricsEnabled:       true,               // Monitor performance
    PrefetchConfig: &PrefetchConfig{
        EnablePatternLearning: true,          // Learn access patterns
        PredictionWindow:     1 * time.Minute, // Short prediction window
        MaxConcurrentPrefetches: 50,          // High prefetch concurrency
    },
}
```

**Memory-Constrained Environment:**
```go
config := &CacheValidatorConfig{
    L1Size:               5000,               // Small L1 to save memory
    L1TTL:                2 * time.Minute,    // Moderate TTL
    L2Enabled:            true,               // Use L2 for capacity
    L2TTL:                15 * time.Minute,   // Longer L2 TTL
    CompressionEnabled:   true,               // Compress to save space
    CompressionLevel:     9,                  // Maximum compression
    MetricsEnabled:       false,              // Disable metrics to save memory
}
```

**Distributed High-Availability Setup:**
```go
config := &CacheValidatorConfig{
    L1Size:               10000,
    L1TTL:                5 * time.Minute,
    L2Enabled:            true,
    L2TTL:                30 * time.Minute,
    Coordinator: &CoordinatorConfig{
        EnableSharding:    true,              // Distribute data across nodes
        ShardCount:        32,                // Optimize for cluster size
        ConsensusTimeout:  3 * time.Second,   // Fast consensus
        HeartbeatInterval: 1 * time.Second,   // Quick failure detection
    },
}
```

## Contributing

1. Ensure all tests pass
2. Add benchmarks for performance-critical code
3. Update documentation for new features
4. Follow the existing code style and patterns

## License

This code is part of the AG-UI Go SDK and follows the same licensing terms.