# Race Conditions Analysis for go-sdk/pkg/tools

## Critical Race Conditions Found

### 1. Registry.go

#### Race Condition 1: TOCTOU in RegisterWithContext
**Location**: Lines 244-285
**Issue**: Time-of-check to time-of-use race between checking for existing tools and registering new ones.
```go
// Lines 244-251: RLock to check existence
r.mu.RLock()
existingTool, idExists := r.tools[tool.ID]
existingID, nameExists := r.nameIndex[tool.Name]
var existingByName *Tool
if nameExists && existingID != tool.ID {
    existingByName = r.tools[existingID]
}
r.mu.RUnlock()

// Lines 253-283: Conflict resolution without lock
// Another goroutine could register a tool with the same ID/name here

// Lines 285-323: Write lock to register
r.mu.Lock()
defer r.mu.Unlock()
// But the tool might already exist now!
```

#### Race Condition 2: Concurrent Map Access in checkConcurrencyLimit
**Location**: Lines 338-358 in executor.go
**Issue**: Unlocking and re-locking mutex while checking activeCount
```go
if e.activeCount >= e.maxConcurrent {
    for e.activeCount >= e.maxConcurrent {
        e.mu.Unlock()  // Releases lock
        select {
        case <-ctx.Done():
            e.mu.Lock()  // Re-acquires lock
            return ctx.Err()
        case <-time.After(100 * time.Millisecond):
            e.mu.Lock()  // Re-acquires lock
        }
    }
}
```

#### Race Condition 3: FileWatcher Stop/Callback Race
**Location**: Lines 909-941, 979-1001
**Issue**: Multiple race conditions in FileWatcher:
1. Stop channel can be closed multiple times
2. Callback can be called after stop
3. WaitGroup not properly tracked

#### Race Condition 4: Dependency Graph Cache
**Location**: Lines 1192-1212
**Issue**: Reading from cache without proper locking in ResolveDependencies
```go
// Check cache first
if resolved, exists := dg.resolved[toolID]; exists {
    return resolved, nil
}
// Another goroutine could modify cache here
```

#### Race Condition 5: Category Tree Operations
**Location**: Lines 1048-1088
**Issue**: Non-atomic operations on category tree that could lead to inconsistent state

### 2. Executor.go

#### Race Condition 1: Async Results Map
**Location**: Lines 594-596
**Issue**: Non-atomic map operations
```go
e.mu.Lock()
e.asyncResults[jobID] = resultChan
e.mu.Unlock()
// Map could be accessed/modified between operations
```

#### Race Condition 2: Metrics Updates
**Location**: Lines 382-411
**Issue**: Non-atomic counter updates that could lead to lost updates

#### Race Condition 3: Hook Arrays
**Location**: Lines 451-458
**Issue**: Hook arrays are modified without synchronization
```go
func (e *ExecutionEngine) AddBeforeExecuteHook(hook ExecutionHook) {
    e.beforeExecute = append(e.beforeExecute, hook) // No lock!
}
```

## Recommended Fixes

### Fix 1: Atomic Registration in Registry
- Use double-checked locking pattern
- Or use a single write lock for the entire registration process

### Fix 2: Proper Concurrency Control in Executor
- Use atomic operations for counters
- Use sync.Map for concurrent access patterns
- Implement proper condition variables for waiting

### Fix 3: FileWatcher Lifecycle Management
- Use sync.Once for stop operations
- Properly coordinate goroutine lifecycle with WaitGroup
- Add mutex protection for all shared state

### Fix 4: Thread-Safe Cache Implementation
- Use sync.Map or properly locked cache
- Implement cache with atomic operations

### Fix 5: Hook Management
- Use sync.RWMutex to protect hook slices
- Or use atomic.Value for hook arrays

## Implementation Summary

I've created the following files to address the race conditions:

1. **registry_race_fixes.go**: Contains `RaceFixedRegistry` with fixes for:
   - TOCTOU race in RegisterWithContext (holds write lock for entire operation)
   - FileWatcher lifecycle management using sync.Once and sync.Map
   - Dependency graph cache using sync.Map
   - Hook protection with RWMutex

2. **executor_race_fixes.go**: Contains `RaceFixedExecutionEngine` with fixes for:
   - Atomic operations for activeCount
   - sync.Map for concurrent execution tracking
   - Condition variables for proper waiting on concurrency limits
   - Protected hook arrays with RWMutex
   - Atomic metrics updates

3. **race_test.go**: Comprehensive test suite that:
   - Tests for specific race conditions
   - Includes tests designed to run with Go's race detector (`go test -race`)
   - Verifies the fixes prevent data races

## Key Improvements

1. **Eliminated TOCTOU Races**: By holding locks for entire critical sections
2. **Used sync.Map**: For concurrent map access patterns (watchers, async results, cache)
3. **Atomic Operations**: For counters and metrics that are frequently updated
4. **Proper Synchronization**: Using condition variables instead of polling
5. **Protected Shared State**: All mutable shared state is now properly synchronized

## Testing

Run the race tests with:
```bash
go test -race -v ./pkg/tools -run TestRace
```

This will use Go's race detector to verify that the implementations are free from data races.