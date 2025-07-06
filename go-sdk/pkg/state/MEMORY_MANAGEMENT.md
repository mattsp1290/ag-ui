# Memory Management in State Package

This document describes the memory management strategies implemented in the state management system to prevent memory leaks and ensure bounded resource usage.

## Overview

The state management system implements several bounded data structures to prevent unbounded memory growth:

1. **ContextManager** - Manages state contexts with LRU eviction
2. **BoundedResolverRegistry** - Manages custom conflict resolvers with size limits

## ContextManager

The `ContextManager` replaces the previous `sync.Map` implementation for managing active contexts. It provides:

### Features
- **Bounded Size**: Enforces a maximum number of contexts (default: 1000)
- **LRU Eviction**: Automatically evicts least recently used contexts when at capacity
- **Thread-Safe**: All operations are protected by mutex locks
- **TTL Support**: Can clean up contexts that haven't been accessed within a TTL

### Usage
```go
// Create a context manager with max 1000 contexts
cm := NewContextManager(1000)

// Add a context
ctx := &StateContext{
    ID: "ctx-123",
    StateID: "state-456",
    Created: time.Now(),
    LastAccessed: time.Now(),
}
cm.Put(ctx.ID, ctx)

// Get a context (updates last accessed time)
ctx, exists := cm.Get("ctx-123")

// Remove expired contexts (older than 1 hour)
expired := cm.CleanupExpired(1 * time.Hour)
```

### Configuration
The maximum number of contexts is configured via `ManagerOptions.CacheSize`:

```go
opts := DefaultManagerOptions()
opts.CacheSize = 5000 // Allow up to 5000 contexts
sm, _ := NewStateManager(opts)
```

## BoundedResolverRegistry

The `BoundedResolverRegistry` manages custom conflict resolvers with size limits to prevent memory exhaustion from unbounded resolver registration.

### Features
- **Bounded Size**: Enforces a maximum number of resolvers (default: 100)
- **LRU Eviction**: Automatically evicts least recently used resolvers
- **Thread-Safe**: All operations are protected by mutex locks
- **Access Tracking**: Tracks last access time for each resolver

### Usage
```go
// Create a registry with max 100 resolvers
br := NewBoundedResolverRegistry(100)

// Register a resolver
err := br.Register("my-resolver", func(conflict *StateConflict) (*ConflictResolution, error) {
    // Custom resolution logic
    return nil, nil
})

// Get a resolver
resolver, exists := br.Get("my-resolver")

// Get statistics
stats := br.GetStatistics()
// stats["current_size"] = 42
// stats["max_size"] = 100
// stats["capacity"] = 0.42
```

## Memory Leak Prevention

### Previous Issues
1. **Unbounded sync.Map**: The `activeContexts` sync.Map could grow indefinitely
2. **Unbounded resolver map**: The `customResolvers` map had no size limits

### Solutions Implemented
1. **Bounded Collections**: Both contexts and resolvers now use bounded LRU caches
2. **Automatic Eviction**: Least recently used items are automatically evicted at capacity
3. **Periodic Cleanup**: Background goroutine cleans up expired contexts based on TTL
4. **Graceful Shutdown**: Proper cleanup during manager shutdown

### Best Practices
1. **Set Appropriate Limits**: Configure `CacheSize` based on expected load
2. **Monitor Metrics**: Use `GetMetrics()` to monitor active context count
3. **Handle Eviction**: Design your application to handle context eviction gracefully
4. **Clean Shutdown**: Always call `Close()` on the StateManager when done

## Performance Considerations

### LRU Cache Overhead
- **Time Complexity**: O(1) for Get/Put operations
- **Space Complexity**: O(n) where n is the max size
- **Lock Contention**: Minimal due to efficient locking strategy

### Benchmarks
Run benchmarks to verify performance:
```bash
go test -bench=BenchmarkContextManager -benchmem ./pkg/state
```

### Tuning Recommendations
1. **Context Cache Size**: Set based on concurrent users/sessions
2. **Resolver Registry Size**: Set based on number of custom resolver types
3. **Cleanup Interval**: Balance between memory usage and cleanup overhead
4. **Context TTL**: Set based on typical session duration

## Monitoring

Monitor these metrics to ensure healthy memory usage:

```go
metrics := sm.GetMetrics()
// Check active contexts
activeContexts := metrics["active_contexts"]

// Check if approaching limits
if activeContexts.(int) > int(0.8 * opts.CacheSize) {
    // Consider increasing cache size or investigating why contexts aren't being released
}
```

## Testing

The memory leak fixes are tested in `memory_leak_test.go`:

- `TestContextManagerBoundedSize`: Verifies size limits are enforced
- `TestBoundedResolverRegistrySize`: Verifies resolver limits
- `TestStateManagerMemoryLeakPrevention`: End-to-end memory leak test
- Concurrent access tests ensure thread safety

Run memory leak tests:
```bash
go test ./pkg/state -run MemoryLeak -v
```