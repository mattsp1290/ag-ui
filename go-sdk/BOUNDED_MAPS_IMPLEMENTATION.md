# Bounded Maps Implementation for Memory Leak Prevention

This document describes the comprehensive bounded maps implementation that prevents memory leaks in long-running services by limiting map sizes and providing automatic cleanup mechanisms.

## Overview

The bounded maps implementation addresses memory leak concerns in the httpagent2 codebase by replacing unbounded maps with size-limited, TTL-enabled, and automatically cleaned maps. This prevents memory growth issues in long-running services while preserving all existing functionality.

## Key Components

### 1. Core Bounded Map (`pkg/internal/bounded_map.go`)

The foundation component that provides:

- **LRU Eviction**: Least Recently Used algorithm for capacity-based eviction
- **TTL Support**: Time-to-Live for automatic expiration
- **Periodic Cleanup**: Background workers for expired entry removal
- **Thread Safety**: Full concurrency support with RWMutex
- **Metrics Collection**: OpenTelemetry metrics integration
- **Eviction Callbacks**: Customizable callbacks for cleanup operations

#### Features:
```go
type BoundedMapConfig struct {
    MaxSize         int                    // Maximum entries (0 = unlimited)
    TTL             time.Duration          // Time-to-live (0 = no TTL)
    CleanupInterval time.Duration          // Cleanup frequency
    EvictionCallback func(key, value interface{}, reason EvictionReason)
    EnableMetrics   bool                   // Metrics collection
    MetricsPrefix   string                 // Metrics naming prefix
}
```

#### Usage Example:
```go
boundedMap := internal.NewBoundedMapOptions().
    WithMaxSize(10000).
    WithTTL(30 * time.Minute).
    WithCleanupInterval(5 * time.Minute).
    WithMetrics(true).
    WithEvictionCallback(func(key, value interface{}, reason internal.EvictionReason) {
        // Custom cleanup logic
    }).
    Build()
```

### 2. Connection Pool Bounds (`pkg/http/connection_pool.go`)

**Problem Solved**: The `pools map[string]*serverPool` could grow indefinitely in long-running services as new servers were added.

**Solution**: Replaced with bounded map that:
- Limits the number of server pools (default: 1000)
- Expires unused pools after 30 minutes
- Automatically closes connections when pools are evicted
- Provides metrics for pool usage

#### Configuration:
```go
type BoundedPoolConfig struct {
    MaxServerPools      int           // Max server pools (default: 1000)
    ServerPoolTTL       time.Duration // Pool TTL (default: 30m)
    PoolCleanupInterval time.Duration // Cleanup interval (default: 5m)
    EnablePoolMetrics   bool          // Metrics (default: true)
}
```

#### Key Changes:
- `pools map[string]*serverPool` → `pools *internal.BoundedMap`
- Automatic cleanup of evicted server pools
- Connection cleanup on pool eviction
- Enhanced metrics and monitoring

### 3. Request Correlation Bounds (`pkg/client/request_manager.go`)

**Problem Solved**: The `correlationMap sync.Map` could accumulate request correlations indefinitely.

**Solution**: Replaced with bounded map that:
- Limits correlation entries (default: 10,000)
- Expires correlations after 1 hour
- Automatically ends trace spans on eviction
- Prevents correlation ID leaks

#### Configuration:
```go
type BoundedCorrelationConfig struct {
    MaxCorrelations int           // Max correlations (default: 10000)
    CorrelationTTL  time.Duration // Correlation TTL (default: 1h)
    CleanupInterval time.Duration // Cleanup interval (default: 5m)
    EnableMetrics   bool          // Metrics (default: true)
}
```

#### Key Changes:
- `correlationMap sync.Map` → `correlationMap *internal.BoundedMap`
- Automatic trace span cleanup
- Correlation metrics tracking
- Memory-bounded request tracking

### 4. Configuration Cache Bounds (`pkg/client/agent_config.go`)

**Problem Solved**: Multiple unbounded maps in the configuration manager:
- `configCache map[string]*CachedConfig`
- `watchers map[string]*ConfigWatcher`  
- `listeners map[string][]ConfigChangeListener`

**Solution**: Replaced all with bounded maps that:
- Limit cached configurations (default: 1000)
- Expire unused watchers (default: 2 hours)
- Clean up listener groups automatically
- Provide comprehensive cache statistics

#### Configuration:
```go
type BoundedConfigManagerConfig struct {
    MaxCachedConfigs int           // Max cached configs (default: 1000)
    ConfigCacheTTL   time.Duration // Config TTL (default: 1h)
    MaxWatchers      int           // Max watchers (default: 500)
    WatcherTTL       time.Duration // Watcher TTL (default: 2h)
    MaxListeners     int           // Max listener groups (default: 500)
    ListenerTTL      time.Duration // Listener TTL (default: 2h)
    CleanupInterval  time.Duration // Cleanup interval (default: 15m)
    EnableMetrics    bool          // Metrics (default: true)
}
```

## Memory Leak Prevention Strategies

### 1. Size-Based Limits
- **LRU Eviction**: When size limits are reached, least recently used entries are removed
- **Configurable Limits**: All size limits are configurable with sensible defaults
- **Graceful Degradation**: Services continue operating even when limits are hit

### 2. Time-Based Cleanup
- **TTL Expiration**: Entries automatically expire after configured time periods
- **Periodic Workers**: Background goroutines clean up expired entries
- **Immediate Cleanup**: Expired entries are removed during access

### 3. Resource Cleanup
- **Eviction Callbacks**: Custom cleanup logic runs when entries are evicted
- **Connection Cleanup**: Network connections are properly closed
- **Trace Span Cleanup**: OpenTelemetry spans are ended to prevent leaks

### 4. Monitoring and Observability
- **Comprehensive Metrics**: Size, hit/miss ratios, eviction counts, cleanup durations
- **Statistics API**: Real-time stats for debugging and monitoring
- **Structured Logging**: Detailed logs for eviction events and cleanup operations

## Configuration Examples

### Production Settings (Conservative)
```go
// Connection Pool
BoundedPoolConfig{
    MaxServerPools:      500,
    ServerPoolTTL:       1 * time.Hour,
    PoolCleanupInterval: 10 * time.Minute,
}

// Request Correlations  
BoundedCorrelationConfig{
    MaxCorrelations: 5000,
    CorrelationTTL:  30 * time.Minute,
    CleanupInterval: 5 * time.Minute,
}

// Configuration Cache
BoundedConfigManagerConfig{
    MaxCachedConfigs: 500,
    ConfigCacheTTL:   30 * time.Minute,
    MaxWatchers:      100,
    WatcherTTL:       1 * time.Hour,
    MaxListeners:     100,
    ListenerTTL:      1 * time.Hour,
    CleanupInterval:  10 * time.Minute,
}
```

### High-Throughput Settings (Aggressive)
```go
// Connection Pool
BoundedPoolConfig{
    MaxServerPools:      2000,
    ServerPoolTTL:       2 * time.Hour,
    PoolCleanupInterval: 5 * time.Minute,
}

// Request Correlations
BoundedCorrelationConfig{
    MaxCorrelations: 50000,
    CorrelationTTL:  2 * time.Hour,
    CleanupInterval: 2 * time.Minute,
}

// Configuration Cache  
BoundedConfigManagerConfig{
    MaxCachedConfigs: 5000,
    ConfigCacheTTL:   2 * time.Hour,
    MaxWatchers:      1000,
    WatcherTTL:       4 * time.Hour,
    MaxListeners:     1000,
    ListenerTTL:      4 * time.Hour,
    CleanupInterval:  5 * time.Minute,
}
```

## Metrics and Monitoring

### Available Metrics
- `bounded_map_entries`: Current number of entries
- `bounded_map_hits_total`: Total cache hits
- `bounded_map_misses_total`: Total cache misses  
- `bounded_map_evictions_total`: Total evictions (by reason)
- `bounded_map_ttl_evictions_total`: TTL-based evictions
- `bounded_map_lru_evictions_total`: LRU-based evictions
- `bounded_map_cleanup_runs_total`: Cleanup operations
- `bounded_map_cleanup_duration_seconds`: Cleanup duration histogram

### Monitoring Example
```go
// Get statistics for all bounded maps
stats := map[string]interface{}{
    "connection_pools": connectionPool.GetStats(),
    "request_correlations": requestManager.GetCorrelationMapStats(), 
    "config_caches": configManager.GetBoundedMapStats(),
}

// Log statistics periodically
ticker := time.NewTicker(1 * time.Minute)
go func() {
    for range ticker.C {
        for component, componentStats := range stats {
            logger.WithFields(logrus.Fields{
                "component": component,
                "stats": componentStats,
            }).Info("Bounded map statistics")
        }
    }
}()
```

## Thread Safety

All bounded map operations are thread-safe:
- **Read Operations**: Protected by RWMutex read locks
- **Write Operations**: Protected by RWMutex write locks  
- **Cleanup Operations**: Coordinated with main operations
- **Eviction Callbacks**: Run in separate goroutines to prevent blocking

## Performance Characteristics

### Time Complexity
- **Get**: O(1) average case
- **Set**: O(1) average case  
- **Delete**: O(1) average case
- **LRU Update**: O(1) using doubly-linked list
- **Cleanup**: O(n) where n is number of expired entries

### Memory Overhead
- **Per Entry**: ~100-200 bytes (key, value, metadata, list pointers)
- **Base Structure**: ~1-2 KB (maps, lists, mutexes, metrics)
- **Total**: Bounded by MaxSize × per-entry overhead

### Cleanup Performance
- **Background Workers**: Non-blocking cleanup operations
- **Batch Processing**: Multiple expired entries cleaned in single pass
- **Timeout Protection**: Cleanup operations have time limits to prevent blocking

## Migration Guide

### Before (Unbounded)
```go
type ConnectionPool struct {
    pools map[string]*serverPool
    mu    sync.RWMutex
}

func (cp *ConnectionPool) getPool(host string) *serverPool {
    cp.mu.RLock()
    pool, exists := cp.pools[host]
    cp.mu.RUnlock()
    
    if !exists {
        cp.mu.Lock()
        pool = newServerPool(host)
        cp.pools[host] = pool
        cp.mu.Unlock()
    }
    
    return pool
}
```

### After (Bounded)
```go
type ConnectionPool struct {
    pools *internal.BoundedMap
    mu    sync.RWMutex
}

func (cp *ConnectionPool) getPool(host string) *serverPool {
    if poolInterface, exists := cp.pools.Get(host); exists {
        if pool, ok := poolInterface.(*serverPool); ok {
            return pool
        }
    }
    
    cp.mu.Lock()
    defer cp.mu.Unlock()
    
    // Double-check pattern
    if poolInterface, exists := cp.pools.Get(host); exists {
        if pool, ok := poolInterface.(*serverPool); ok {
            return pool
        }
    }
    
    pool := newServerPool(host)
    cp.pools.Set(host, pool)
    return pool
}
```

## Testing and Validation

### Unit Tests
- Bounded map functionality (size limits, TTL, cleanup)
- LRU eviction behavior
- Thread safety under concurrent access
- Metrics accuracy
- Eviction callback execution

### Integration Tests  
- Connection pool behavior with bounded server pools
- Request correlation lifecycle management
- Configuration cache effectiveness
- End-to-end memory leak prevention

### Performance Tests
- Memory usage over time
- Cleanup operation performance
- Concurrent access benchmarks
- Large-scale eviction scenarios

## Best Practices

### 1. Size Limits
- Set limits based on expected usage patterns
- Monitor metrics to adjust limits over time
- Consider memory constraints of deployment environment

### 2. TTL Configuration
- Use longer TTLs for frequently accessed data
- Use shorter TTLs for transient data
- Balance between cache effectiveness and memory usage

### 3. Cleanup Intervals
- Set cleanup intervals to 1/4 of TTL for efficiency
- More frequent cleanup for high-churn scenarios
- Less frequent cleanup for stable workloads

### 4. Monitoring
- Set up alerts for high eviction rates
- Monitor memory usage trends
- Track cache hit/miss ratios for optimization

### 5. Eviction Callbacks
- Keep callbacks lightweight and non-blocking
- Perform resource cleanup (close connections, cancel contexts)
- Log important eviction events for debugging

## Conclusion

The bounded maps implementation provides a comprehensive solution to prevent memory leaks in the httpagent2 codebase while maintaining all existing functionality. The solution is:

- **Production-Ready**: Thoroughly tested and configurable
- **Observable**: Comprehensive metrics and logging
- **Performant**: Minimal overhead with efficient algorithms
- **Thread-Safe**: Full concurrency support
- **Flexible**: Highly configurable for different use cases

This implementation ensures that long-running services can operate indefinitely without memory growth issues, while providing the monitoring and debugging capabilities needed for production deployments.