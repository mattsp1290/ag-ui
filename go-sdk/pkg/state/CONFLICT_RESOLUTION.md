# AG-UI State Conflict Resolution System

## Overview

The AG-UI conflict resolution system provides comprehensive tools for detecting and resolving conflicts that arise when multiple clients modify the same state concurrently. The system supports multiple resolution strategies and maintains a complete history of conflicts and their resolutions.

## Core Components

### 1. ConflictDetector

Detects conflicts between concurrent state changes:

```go
detector := state.NewConflictDetector(state.DefaultConflictDetectorOptions())

conflict, err := detector.DetectConflict(localChange, remoteChange)
if conflict != nil {
    // Handle conflict
}
```

Features:
- Path-based conflict detection
- Severity classification (Low, Medium, High)
- Configurable ignore paths
- Semantic conflict detection

### 2. ConflictResolver

Resolves conflicts using various strategies:

```go
resolver := state.NewConflictResolver(state.LastWriteWins)
resolution, err := resolver.Resolve(conflict)
```

### 3. Resolution Strategies

#### Last Write Wins
Uses timestamp to determine the winner - the most recent change wins.

```go
resolver := state.NewConflictResolver(state.LastWriteWins)
```

#### First Write Wins
The first successful modification wins, preventing later changes.

```go
resolver := state.NewConflictResolver(state.FirstWriteWins)
```

#### Merge Strategy
Attempts to merge non-conflicting changes, particularly useful for object/map types.

```go
resolver := state.NewConflictResolver(state.MergeStrategy)
```

#### User Choice Strategy
Presents conflicts to the user for manual resolution.

```go
resolver := state.NewConflictResolver(state.UserChoiceStrategy)
resolver.SetUserResolver(customUserResolver)
```

#### Custom Strategy
Allows implementation of domain-specific resolution logic.

```go
resolver := state.NewConflictResolver(state.CustomStrategy)
resolver.RegisterCustomResolver("default", func(conflict *StateConflict) (*ConflictResolution, error) {
    // Custom resolution logic
    return resolution, nil
})
```

### 4. ConflictHistory

Tracks all conflicts and resolutions for analysis:

```go
history := state.NewConflictHistory(1000) // Keep last 1000 entries

// Record events
history.RecordConflict(conflict)
history.RecordResolution(resolution)

// Get statistics
stats := history.GetStatistics()
```

### 5. ConflictManager

High-level interface that combines detection and resolution:

```go
store := state.NewStateStore()
manager := state.NewConflictManager(store, state.LastWriteWins)

// Detect and resolve in one step
resolution, err := manager.ResolveConflict(localChange, remoteChange)

// Apply the resolution
err = manager.ApplyResolution(resolution)
```

## Usage Examples

### Basic Conflict Detection and Resolution

```go
// Create detector and resolver
detector := state.NewConflictDetector(state.DefaultConflictDetectorOptions())
resolver := state.NewConflictResolver(state.LastWriteWins)

// Define changes
localChange := &state.StateChange{
    Path:      "/user/profile",
    OldValue:  map[string]interface{}{"name": "Alice"},
    NewValue:  map[string]interface{}{"name": "Alice Smith"},
    Operation: "replace",
    Timestamp: time.Now(),
}

remoteChange := &state.StateChange{
    Path:      "/user/profile",
    OldValue:  map[string]interface{}{"name": "Alice"},
    NewValue:  map[string]interface{}{"name": "Alice Johnson"},
    Operation: "replace",
    Timestamp: time.Now().Add(time.Second),
}

// Detect conflict
conflict, err := detector.DetectConflict(localChange, remoteChange)
if err != nil {
    log.Fatal(err)
}

if conflict != nil {
    // Resolve conflict
    resolution, err := resolver.Resolve(conflict)
    if err != nil {
        log.Fatal(err)
    }
    
    // Apply resolution
    fmt.Printf("Resolved to: %v\n", resolution.ResolvedValue)
}
```

### Merge Strategy Example

```go
resolver := state.NewConflictResolver(state.MergeStrategy)

// Conflicting map updates
localUser := map[string]interface{}{
    "id":    "123",
    "name":  "Updated Name",
    "phone": "555-1234", // Added locally
}

remoteUser := map[string]interface{}{
    "id":      "123",
    "name":    "Different Name",
    "address": "123 Main St", // Added remotely
}

conflict := &state.StateConflict{
    LocalChange: &state.StateChange{
        NewValue: localUser,
    },
    RemoteChange: &state.StateChange{
        NewValue: remoteUser,
    },
}

resolution, _ := resolver.Resolve(conflict)
// Result will merge non-conflicting fields (phone and address)
// For conflicting fields (name), remote wins
```

### Custom Resolution Strategy

```go
resolver := state.NewConflictResolver(state.CustomStrategy)

// Register custom resolver for numeric values
resolver.RegisterCustomResolver("default", func(conflict *StateConflict) (*ConflictResolution, error) {
    // Custom logic: prefer higher numeric values
    localNum, localOk := conflict.LocalChange.NewValue.(float64)
    remoteNum, remoteOk := conflict.RemoteChange.NewValue.(float64)
    
    if localOk && remoteOk {
        if localNum > remoteNum {
            return createResolution(conflict, localNum, "local"), nil
        }
        return createResolution(conflict, remoteNum, "remote"), nil
    }
    
    // Fallback to timestamp-based resolution
    return defaultResolution(conflict), nil
})
```

### Conflict Analysis

```go
history := state.NewConflictHistory(1000)
analyzer := state.NewConflictAnalyzer(history)

// After recording conflicts and resolutions...

// Get statistics
stats := history.GetStatistics()
fmt.Printf("Total conflicts: %d\n", stats.TotalConflicts)
fmt.Printf("Resolution rate: %.2f%%\n", 
    float64(stats.TotalResolutions)/float64(stats.TotalConflicts)*100)

// Analyze patterns
analysis := analyzer.AnalyzePatterns()
hotPaths := analysis["hot_paths"].([]string)
fmt.Printf("Paths with frequent conflicts: %v\n", hotPaths)
```

## Integration with StateStore

The conflict resolution system integrates seamlessly with the StateStore:

```go
store := state.NewStateStore()
manager := state.NewConflictManager(store, state.MergeStrategy)

// When receiving remote changes
remoteChange := receiveRemoteChange()
localChange := detectLocalChange()

// Resolve any conflicts
resolution, err := manager.ResolveConflict(localChange, remoteChange)
if err != nil {
    log.Fatal(err)
}

// Apply resolution if conflict was detected
if resolution != nil {
    err = manager.ApplyResolution(resolution)
    if err != nil {
        log.Fatal(err)
    }
}
```

## Configuration Options

### ConflictDetectorOptions

```go
options := state.ConflictDetectorOptions{
    StrictMode:              true,  // Enable strict conflict detection
    IgnorePaths:             []string{"/tmp", "/cache"}, // Paths to ignore
    ConflictThreshold:       0.3,   // Threshold for severity calculation
    EnableSemanticDetection: true,  // Enable semantic conflict detection
}

detector := state.NewConflictDetector(options)
```

### ConflictHistory Configuration

```go
history := state.NewConflictHistory(5000) // Keep 5000 entries

// Get recent conflicts
recentConflicts := history.GetRecentConflicts(10)

// Get conflicts within time range
conflicts := history.GetDeltas(startTime, endTime)
```

## Best Practices

1. **Choose the Right Strategy**: Select a resolution strategy based on your use case:
   - Use `LastWriteWins` for simple overwrites
   - Use `MergeStrategy` for collaborative editing
   - Use `CustomStrategy` for domain-specific logic

2. **Monitor Conflict Patterns**: Use ConflictHistory and ConflictAnalyzer to identify problematic paths and optimize your application.

3. **Handle Resolution Failures**: Always handle cases where resolution might fail:
   ```go
   resolution, err := resolver.Resolve(conflict)
   if err != nil {
       // Fall back to a simpler strategy or notify user
   }
   ```

4. **Validate After Resolution**: Ensure the resolved state is valid:
   ```go
   if resolution != nil {
       // Validate resolved value before applying
       if isValid(resolution.ResolvedValue) {
           manager.ApplyResolution(resolution)
       }
   }
   ```

5. **Use Markers for Complex Operations**: For operations that might need rollback:
   ```go
   // Create marker before risky operation
   store.CreateMarker("before-complex-operation")
   
   // If conflict resolution fails catastrophically
   store.RollbackToMarker("before-complex-operation")
   ```

## Thread Safety

All components in the conflict resolution system are thread-safe and can be used concurrently:
- ConflictDetector
- ConflictResolver
- ConflictHistory
- ConflictManager

## Performance Considerations

1. **Conflict Detection**: O(1) for path comparison, O(n) for semantic detection
2. **Resolution**: O(1) for simple strategies, O(n) for merge strategy (where n is object size)
3. **History**: Maintains a circular buffer with configurable size
4. **Memory**: Each conflict record uses approximately 1KB (depending on state size)