# AG-UI State Management Performance Tuning Guide

This guide covers performance optimization techniques, benchmarking, and tuning strategies for the AG-UI state management system to achieve optimal performance in production environments.

## Table of Contents

1. [Performance Overview](#performance-overview)
2. [Benchmarking](#benchmarking)
3. [Caching Optimization](#caching-optimization)
4. [Storage Backend Optimization](#storage-backend-optimization)
5. [Batching and Concurrency](#batching-and-concurrency)
6. [Memory Management](#memory-management)
7. [Network Optimization](#network-optimization)
8. [Monitoring and Profiling](#monitoring-and-profiling)
9. [Use Case Optimizations](#use-case-optimizations)
10. [Troubleshooting Performance Issues](#troubleshooting-performance-issues)

## Performance Overview

### Key Performance Metrics

The AG-UI state management system tracks several key performance indicators:

- **Throughput**: Operations per second (read/write)
- **Latency**: Response time for operations (P50, P95, P99)
- **Memory Usage**: RAM consumption and garbage collection
- **CPU Usage**: Processing overhead
- **Storage I/O**: Disk/network operations for persistence
- **Cache Hit Rate**: Percentage of requests served from cache

### Performance Characteristics

```
Operation Type    | Typical Latency | Max Throughput | Memory Impact
------------------|-----------------|----------------|---------------
Read (cached)     | <1ms           | 50,000 ops/s   | Low
Read (uncached)   | 1-10ms         | 10,000 ops/s   | Medium
Write (batched)   | 5-15ms         | 5,000 ops/s    | Medium
Write (unbatched) | 10-50ms        | 1,000 ops/s    | High
Delta computation | 1-5ms          | 20,000 ops/s   | Low
Validation        | 0.1-1ms        | 100,000 ops/s  | Very Low
```

## Benchmarking

### Running Benchmarks

The state management system includes comprehensive benchmarks:

```bash
# Run all benchmarks
go test -bench=. -benchmem ./pkg/state/

# Run specific benchmarks
go test -bench=BenchmarkStateStore -benchmem ./pkg/state/
go test -bench=BenchmarkDelta -benchmem ./pkg/state/
go test -bench=BenchmarkConflictResolution -benchmem ./pkg/state/

# Run with CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./pkg/state/

# Run with memory profiling
go test -bench=. -memprofile=mem.prof ./pkg/state/
```

### Benchmark Results Analysis

```go
// Example benchmark output
BenchmarkStateStore_Get-8           	  500000	      2845 ns/op	     128 B/op	       3 allocs/op
BenchmarkStateStore_Set-8           	  200000	      8234 ns/op	     512 B/op	       8 allocs/op
BenchmarkDelta_Compute-8            	 1000000	      1235 ns/op	     256 B/op	       4 allocs/op
BenchmarkConflictResolution-8       	  100000	     15678 ns/op	    1024 B/op	      12 allocs/op
```

**Interpreting Results:**
- **ns/op**: Nanoseconds per operation (lower is better)
- **B/op**: Bytes allocated per operation (lower is better)
- **allocs/op**: Memory allocations per operation (lower is better)

### Custom Benchmarks

```go
func BenchmarkCustomWorkload(b *testing.B) {
    // Setup
    options := state.DefaultManagerOptions()
    options.CacheSize = 10000
    manager, _ := state.NewStateManager(options)
    defer manager.Close()
    
    contextID, _ := manager.CreateContext("bench-user", nil)
    
    // Benchmark
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        stateID := fmt.Sprintf("state-%d", i%1000)
        data := map[string]interface{}{
            "id":    stateID,
            "value": i,
            "timestamp": time.Now(),
        }
        
        _, err := manager.UpdateState(contextID, stateID, data, state.UpdateOptions{})
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## Caching Optimization

### Cache Configuration

```go
func OptimizedCacheConfig() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Cache sizing
    options.CacheSize = 50000              // Large cache for hot data
    options.CacheTTL = 30 * time.Minute    // Long TTL for stable data
    
    // Cache policies
    options.CachePolicy = state.CachePolicy{
        MaxSize:           50000,
        TTL:              30 * time.Minute,
        EvictionPolicy:   "lru",            // Least Recently Used
        WarmupEnabled:    true,             // Warm cache on startup
        WarmupSize:       5000,             // Warm 5000 entries
        WriteThrough:     true,             // Write-through caching
        RefreshAhead:     true,             // Refresh before expiry
        RefreshThreshold: 0.8,              // Refresh at 80% TTL
    }
    
    return options
}
```

### Cache Strategies

#### Read-Heavy Workloads

```go
// Optimize for read-heavy workloads
options.CachePolicy = state.CachePolicy{
    MaxSize:         100000,               // Very large cache
    TTL:            60 * time.Minute,      // Long TTL
    EvictionPolicy: "lru",                 // Keep frequently read items
    WriteThrough:   true,                  // Ensure cache consistency
    PrefetchEnabled: true,                 // Prefetch related data
    PrefetchSize:   10,                    // Prefetch 10 related items
}
```

#### Write-Heavy Workloads

```go
// Optimize for write-heavy workloads
options.CachePolicy = state.CachePolicy{
    MaxSize:         10000,                // Smaller cache
    TTL:            5 * time.Minute,       // Short TTL
    EvictionPolicy: "lfu",                 // Least Frequently Used
    WriteThrough:   false,                 // Write-behind caching
    WriteBehind:    true,
    FlushInterval:  30 * time.Second,      // Frequent flushes
}
```

### Multi-Level Caching

```go
// Implement multi-level caching
options.CacheConfig = state.CacheConfig{
    L1Cache: state.CacheLevel{
        Type:     "memory",
        MaxSize:  1000,                    // Small, fast L1 cache
        TTL:      1 * time.Minute,
    },
    L2Cache: state.CacheLevel{
        Type:     "redis",
        MaxSize:  10000,                   // Larger L2 cache
        TTL:      10 * time.Minute,
        ConnectionURL: os.Getenv("REDIS_URL"), // Example: "redis://localhost:6379"
    },
    L3Cache: state.CacheLevel{
        Type:     "file",
        MaxSize:  100000,                  // Large L3 cache
        TTL:      60 * time.Minute,
        Path:     "/var/cache/agui",
    },
}
```

## Storage Backend Optimization

### File Storage Optimization

```go
func OptimizedFileStorage() *state.StorageConfig {
    return &state.StorageConfig{
        // Basic settings
        Path:        "/fast-ssd/agui/state",   // Use fast storage
        Compression: true,                     // Enable compression
        
        // Performance settings
        WriteTimeout:    2 * time.Second,      // Fast writes
        ReadTimeout:     1 * time.Second,      // Fast reads
        SyncWrites:      false,                // Async writes for performance
        
        // Batching
        BatchWrites:     true,                 // Batch file writes
        BatchSize:       1000,                 // Large batches
        BatchTimeout:    100 * time.Millisecond,
        
        // Memory mapping
        UseMemoryMapping: true,                // Memory-mapped files
        MMapSize:        100 * 1024 * 1024,    // 100MB mmap size
        
        // Compaction
        CompactionEnabled:  true,              // Enable compaction
        CompactionInterval: 1 * time.Hour,     // Frequent compaction
        CompactionThreshold: 0.3,              // Compact at 30% fragmentation
        
        // Advanced settings
        MaxFileSize:     500 * 1024 * 1024,    // 500MB max file size
        PreallocateSize: 10 * 1024 * 1024,     // Preallocate 10MB
        IOScheduler:     "deadline",           // Use deadline scheduler
    }
}
```

### Redis Storage Optimization

```go
func OptimizedRedisStorage() *state.StorageConfig {
    return &state.StorageConfig{
        // Connection settings
        ConnectionURL: os.Getenv("REDIS_URL"), // Example: "redis://localhost:6379"
        
        // Pool optimization
        PoolSize:         50,                  // Large pool
        MinIdleConns:     25,                  // Keep connections warm
        MaxConnAge:       10 * time.Minute,    // Rotate connections
        PoolTimeout:      1 * time.Second,     // Fast pool timeout
        IdleTimeout:      2 * time.Minute,     // Quick idle timeout
        
        // Operation settings
        ReadTimeout:      500 * time.Millisecond, // Fast reads
        WriteTimeout:     1 * time.Second,        // Fast writes
        MaxRetries:       2,                      // Minimal retries
        MinRetryBackoff:  5 * time.Millisecond,   // Fast retries
        MaxRetryBackoff:  50 * time.Millisecond,
        
        // Pipelining
        PipelineEnabled:  true,                // Enable pipelining
        PipelineSize:     100,                 // Large pipelines
        PipelineTimeout:  10 * time.Millisecond,
        
        // Clustering
        ClusterEnabled:   true,                // Use Redis Cluster
        ClusterNodes: []string{
            "redis-1:6379",
            "redis-2:6379",
            "redis-3:6379",
        },
        
        // Memory optimization
        Compression:      true,                // Enable compression
        CompressionLevel: 6,                   // Balanced compression
        CompressionMinSize: 1024,              // Compress if > 1KB
    }
}
```

### PostgreSQL Storage Optimization

```go
func OptimizedPostgreSQLStorage() *state.StorageConfig {
    return &state.StorageConfig{
        // Connection settings
        ConnectionURL: os.Getenv("DATABASE_URL"), // Example: "postgres://user:pass@localhost:5432/statedb?sslmode=disable"
        
        // Pool optimization
        MaxOpenConns:     50,                  // Large pool
        MaxIdleConns:     25,                  // Keep connections warm
        ConnMaxLifetime:  30 * time.Minute,    // Long-lived connections
        ConnMaxIdleTime:  5 * time.Minute,     // Moderate idle time
        
        // Query optimization
        QueryTimeout:     5 * time.Second,     // Fast queries
        TxTimeout:        30 * time.Second,    // Reasonable tx timeout
        EnablePreparedStmts: true,             // Use prepared statements
        
        // Bulk operations
        BulkInsertSize:   1000,                // Large bulk inserts
        BulkUpdateSize:   500,                 // Large bulk updates
        BatchTimeout:     100 * time.Millisecond,
        
        // Indexing
        CreateIndexes:    true,                // Create performance indexes
        IndexColumns:     []string{"id", "updated_at", "version"},
        
        // Partitioning
        EnablePartitioning: true,              // Use table partitioning
        PartitionBy:       "created_at",       // Partition by date
        PartitionInterval: "1 month",          // Monthly partitions
        
        // Compression
        Compression:      true,                // Enable compression
        CompressionLevel: 6,                   // Balanced compression
        
        // WAL optimization
        SynchronousCommit: false,              // Async commits
        WALBuffers:        "16MB",             // Large WAL buffers
        CheckpointSegments: 32,                // More checkpoint segments
    }
}
```

## Batching and Concurrency

### Batch Processing Optimization

```go
func OptimizedBatchConfig() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Batch settings
    options.EnableBatching = true
    options.BatchSize = 500                    // Large batches
    options.BatchTimeout = 50 * time.Millisecond // Fast timeout
    
    // Advanced batching
    options.BatchConfig = state.BatchConfig{
        MaxSize:           500,
        MaxTimeout:        50 * time.Millisecond,
        MinSize:           10,                 // Minimum batch size
        MaxWaitTime:       100 * time.Millisecond,
        
        // Priority queues
        PriorityQueues:    true,
        PriorityLevels:    3,
        HighPriorityRatio: 0.3,               // 30% high priority
        
        // Adaptive batching
        AdaptiveSizing:    true,
        TargetLatency:     10 * time.Millisecond,
        SizeAdjustmentFactor: 1.5,
        
        // Parallel processing
        ParallelBatches:   true,
        MaxParallelBatches: 4,
    }
    
    return options
}
```

### Concurrency Optimization

```go
func OptimizedConcurrencyConfig() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Worker settings
    options.ProcessingWorkers = runtime.NumCPU() * 2  // 2x CPU cores
    options.EventBufferSize = 10000                   // Large buffer
    
    // Concurrency control
    options.ConcurrencyConfig = state.ConcurrencyConfig{
        MaxConcurrentOps:    1000,             // High concurrency
        MaxConcurrentReads:  500,              // Many concurrent reads
        MaxConcurrentWrites: 100,              // Limited concurrent writes
        ReadWriteRatio:      5,                // 5:1 read:write ratio
        
        // Lock optimization
        UseReadWriteLocks:   true,             // Use RW locks
        LockTimeout:         5 * time.Second,  // Lock timeout
        DeadlockTimeout:     10 * time.Second, // Deadlock detection
        
        // Goroutine pooling
        UseGoroutinePool:    true,             // Pool goroutines
        PoolSize:            100,              // Pool size
        PoolMaxIdle:         50,               // Max idle goroutines
        
        // Work stealing
        EnableWorkStealing:  true,             // Enable work stealing
        WorkStealingRatio:   0.5,              // Steal 50% of work
    }
    
    return options
}
```

## Memory Management

### Memory Optimization

```go
func OptimizedMemoryConfig() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Memory limits
    options.MemoryConfig = state.MemoryConfig{
        MaxHeapSize:        1024 * 1024 * 1024, // 1GB max heap
        MaxCacheSize:       512 * 1024 * 1024,  // 512MB max cache
        MaxBufferSize:      128 * 1024 * 1024,  // 128MB max buffers
        
        // Garbage collection
        GCTargetPercent:    50,                  // Aggressive GC
        GCForceInterval:    30 * time.Second,    // Force GC every 30s
        
        // Object pooling
        EnableObjectPools:  true,                // Use object pools
        PooledTypes:        []string{"State", "Delta", "Event"},
        
        // Memory monitoring
        MonitorMemory:      true,                // Monitor memory usage
        MemoryCheckInterval: 10 * time.Second,   // Check every 10s
        MemoryThreshold:    0.8,                 // Alert at 80% usage
        
        // Compression
        CompressLargeObjects: true,              // Compress large objects
        CompressionThreshold: 64 * 1024,         // Compress if > 64KB
    }
    
    return options
}
```

### Object Pooling

```go
// Custom object pools for frequently allocated objects
type StatePool struct {
    pool sync.Pool
}

func NewStatePool() *StatePool {
    return &StatePool{
        pool: sync.Pool{
            New: func() interface{} {
                return &state.State{
                    Data: make(map[string]interface{}),
                    Metadata: make(map[string]interface{}),
                }
            },
        },
    }
}

func (p *StatePool) Get() *state.State {
    return p.pool.Get().(*state.State)
}

func (p *StatePool) Put(s *state.State) {
    // Reset state before returning to pool
    for k := range s.Data {
        delete(s.Data, k)
    }
    for k := range s.Metadata {
        delete(s.Metadata, k)
    }
    s.Version = 0
    s.CreatedAt = time.Time{}
    s.UpdatedAt = time.Time{}
    
    p.pool.Put(s)
}
```

## Network Optimization

### Connection Pooling

```go
// HTTP transport optimization
func OptimizedHTTPTransport() *http.Transport {
    return &http.Transport{
        MaxIdleConns:        100,              // Large idle pool
        MaxIdleConnsPerHost: 10,               // Per-host idle connections
        MaxConnsPerHost:     50,               // Per-host max connections
        IdleConnTimeout:     90 * time.Second, // Idle timeout
        
        // TCP optimization
        DisableKeepAlives:   false,            // Keep connections alive
        DisableCompression:  false,            // Enable compression
        
        // Timeouts
        DialTimeout:         5 * time.Second,  // Connection timeout
        TLSHandshakeTimeout: 5 * time.Second,  // TLS timeout
        ResponseHeaderTimeout: 10 * time.Second, // Response timeout
        
        // TCP keep-alive
        KeepAlive:          30 * time.Second,  // Keep-alive interval
    }
}
```

### Protocol Optimization

```go
// gRPC optimization
func OptimizedGRPCOptions() []grpc.DialOption {
    return []grpc.DialOption{
        // Connection pooling
        grpc.WithDefaultCallOptions(
            grpc.MaxCallRecvMsgSize(4*1024*1024),    // 4MB max message
            grpc.MaxCallSendMsgSize(4*1024*1024),    // 4MB max message
        ),
        
        // Keep-alive
        grpc.WithKeepaliveParams(keepalive.ClientParameters{
            Time:                30 * time.Second,    // Send keep-alive every 30s
            Timeout:             5 * time.Second,     // Keep-alive timeout
            PermitWithoutStream: true,                // Allow keep-alive without streams
        }),
        
        // Compression
        grpc.WithCompressor(grpc.NewGZIPCompressor()),
        grpc.WithDecompressor(grpc.NewGZIPDecompressor()),
        
        // Load balancing
        grpc.WithDefaultServiceConfig(`{
            "loadBalancingPolicy": "round_robin",
            "healthCheckConfig": {
                "serviceName": "agui.state.v1.StateService"
            }
        }`),
    }
}
```

## Monitoring and Profiling

### Performance Monitoring

```go
// Enable comprehensive monitoring
func EnablePerformanceMonitoring(manager *state.StateManager) {
    // CPU profiling
    if cpuProfile := os.Getenv("CPUPROFILE"); cpuProfile != "" {
        f, err := os.Create(cpuProfile)
        if err != nil {
            log.Fatal(err)
        }
        pprof.StartCPUProfile(f)
        defer pprof.StopCPUProfile()
    }
    
    // Memory profiling
    if memProfile := os.Getenv("MEMPROFILE"); memProfile != "" {
        defer func() {
            f, err := os.Create(memProfile)
            if err != nil {
                log.Fatal(err)
            }
            pprof.WriteHeapProfile(f)
            f.Close()
        }()
    }
    
    // Performance metrics
    go func() {
        for {
            stats := manager.GetStats()
            log.Printf("Performance Stats: %+v", stats)
            time.Sleep(30 * time.Second)
        }
    }()
}
```

### Custom Metrics

```go
// Define custom performance metrics
var (
    operationLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "agui_state_operation_latency_seconds",
            Help:    "Operation latency in seconds",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
        },
        []string{"operation", "result"},
    )
    
    memoryUsage = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "agui_state_memory_usage_bytes",
            Help: "Memory usage in bytes",
        },
        []string{"component"},
    )
    
    cacheHitRate = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "agui_state_cache_hit_rate",
            Help: "Cache hit rate",
        },
        []string{"cache_type"},
    )
)

// Collect metrics
func CollectMetrics(manager *state.StateManager) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            stats := manager.GetStats()
            
            // Update memory usage
            memoryUsage.WithLabelValues("total").Set(float64(stats.MemoryUsage))
            memoryUsage.WithLabelValues("cache").Set(float64(stats.CacheMemoryUsage))
            
            // Update cache hit rate
            cacheHitRate.WithLabelValues("l1").Set(stats.CacheHitRate)
            
            // Update other metrics...
        }
    }
}
```

## Use Case Optimizations

### Real-time Applications

```go
func RealTimeOptimization() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Minimal latency settings
    options.CacheSize = 10000
    options.CacheTTL = 30 * time.Second
    
    // Fast batching
    options.BatchSize = 50
    options.BatchTimeout = 10 * time.Millisecond
    
    // In-memory storage for speed
    options.StorageBackend = "memory"
    
    // Many workers for parallel processing
    options.ProcessingWorkers = runtime.NumCPU() * 4
    
    // Minimal validation for speed
    options.StrictMode = false
    
    // Disable heavy features
    options.EnableCompression = false
    options.AutoCheckpoint = false
    
    return options
}
```

### High-Throughput Applications

```go
func HighThroughputOptimization() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Large batches
    options.BatchSize = 1000
    options.BatchTimeout = 100 * time.Millisecond
    
    // Large cache
    options.CacheSize = 100000
    options.CacheTTL = 60 * time.Minute
    
    // Redis for high throughput
    options.StorageBackend = "redis"
    options.StorageConfig = OptimizedRedisStorage()
    
    // Many workers
    options.ProcessingWorkers = runtime.NumCPU() * 2
    
    // Aggressive compression
    options.EnableCompression = true
    
    // Minimal history
    options.MaxHistorySize = 10
    
    return options
}
```

### Memory-Constrained Environments

```go
func MemoryConstrainedOptimization() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Small cache
    options.CacheSize = 100
    options.CacheTTL = 1 * time.Minute
    
    // Small batches
    options.BatchSize = 10
    options.BatchTimeout = 1 * time.Second
    
    // Aggressive compression
    options.EnableCompression = true
    
    // File storage with compression
    options.StorageBackend = "file"
    options.StorageConfig = &state.StorageConfig{
        Compression: true,
        CompressionLevel: 9,
    }
    
    // Minimal history
    options.MaxHistorySize = 5
    options.MaxCheckpoints = 2
    
    // Few workers
    options.ProcessingWorkers = 1
    
    return options
}
```

## Troubleshooting Performance Issues

### Common Performance Issues

#### High Latency

```go
// Diagnose high latency
func DiagnoseLatency(manager *state.StateManager) {
    stats := manager.GetStats()
    
    if stats.AvgLatency > 100*time.Millisecond {
        log.Printf("High latency detected: %v", stats.AvgLatency)
        
        // Check cache hit rate
        if stats.CacheHitRate < 0.8 {
            log.Printf("Low cache hit rate: %f", stats.CacheHitRate)
            // Increase cache size or TTL
        }
        
        // Check queue sizes
        if stats.QueueSize > 1000 {
            log.Printf("High queue size: %d", stats.QueueSize)
            // Increase workers or batch size
        }
        
        // Check storage backend
        if stats.StorageLatency > 50*time.Millisecond {
            log.Printf("High storage latency: %v", stats.StorageLatency)
            // Optimize storage backend
        }
    }
}
```

#### Memory Usage Issues

```go
// Monitor memory usage
func MonitorMemoryUsage(manager *state.StateManager) {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    if m.Alloc > 1024*1024*1024 { // 1GB
        log.Printf("High memory usage: %d MB", m.Alloc/(1024*1024))
        
        // Force garbage collection
        runtime.GC()
        
        // Check cache size
        stats := manager.GetStats()
        if stats.CacheSize > 50000 {
            log.Printf("Large cache size: %d", stats.CacheSize)
            // Reduce cache size
        }
        
        // Check for memory leaks
        if m.NumGC > 100 && m.Alloc > m.TotalAlloc/10 {
            log.Printf("Potential memory leak detected")
        }
    }
}
```

### Performance Tuning Checklist

1. **Cache Configuration**
   - [ ] Appropriate cache size for workload
   - [ ] Optimal TTL settings
   - [ ] Correct eviction policy
   - [ ] Cache warming enabled

2. **Storage Backend**
   - [ ] Appropriate backend for use case
   - [ ] Connection pooling optimized
   - [ ] Compression enabled where beneficial
   - [ ] Indexing configured

3. **Batching Settings**
   - [ ] Batch size appropriate for workload
   - [ ] Batch timeout optimized
   - [ ] Parallel batching enabled

4. **Concurrency**
   - [ ] Worker count matches CPU cores
   - [ ] Queue sizes appropriate
   - [ ] Lock contention minimized

5. **Memory Management**
   - [ ] Object pooling enabled
   - [ ] Garbage collection tuned
   - [ ] Memory limits set

6. **Monitoring**
   - [ ] Performance metrics enabled
   - [ ] Alerts configured
   - [ ] Profiling available

### Load Testing

```go
func LoadTest(manager *state.StateManager) {
    var wg sync.WaitGroup
    numWorkers := 100
    operationsPerWorker := 1000
    
    start := time.Now()
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            contextID, _ := manager.CreateContext(fmt.Sprintf("worker-%d", workerID), nil)
            
            for j := 0; j < operationsPerWorker; j++ {
                stateID := fmt.Sprintf("state-%d-%d", workerID, j)
                data := map[string]interface{}{
                    "id":    stateID,
                    "value": j,
                    "worker": workerID,
                }
                
                _, err := manager.UpdateState(contextID, stateID, data, state.UpdateOptions{})
                if err != nil {
                    log.Printf("Error: %v", err)
                }
            }
        }(i)
    }
    
    wg.Wait()
    duration := time.Since(start)
    
    totalOps := numWorkers * operationsPerWorker
    throughput := float64(totalOps) / duration.Seconds()
    
    log.Printf("Load test completed:")
    log.Printf("  Total operations: %d", totalOps)
    log.Printf("  Duration: %v", duration)
    log.Printf("  Throughput: %.2f ops/sec", throughput)
    log.Printf("  Average latency: %v", duration/time.Duration(totalOps))
}
```

This performance tuning guide provides comprehensive strategies for optimizing the AG-UI state management system across different deployment scenarios and workload patterns. Regular monitoring and profiling are essential for maintaining optimal performance in production environments.