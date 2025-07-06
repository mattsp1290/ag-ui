# AG-UI State Management Package

The `state` package provides a comprehensive, thread-safe state management solution for the AG-UI Go SDK. It handles complex state operations including storage, validation, conflict resolution, rollback capabilities, and event integration.

## Architecture Overview

The state management system consists of several integrated components:

```
┌─────────────────────────────────────────────────────────────┐
│                      StateManager                           │
│  (Main entry point - coordinates all components)            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐  ┌─────────────────┐  ┌───────────────┐│
│  │ StateStore   │  │ DeltaComputer   │  │StateValidator ││
│  │              │  │                 │  │               ││
│  │ - Storage    │  │ - Change        │  │ - Validation  ││
│  │ - History    │  │   tracking      │  │ - Rules       ││
│  │ - Caching    │  │ - Diff calc     │  │ - Schemas     ││
│  └──────────────┘  └─────────────────┘  └───────────────┘│
│                                                             │
│  ┌──────────────┐  ┌─────────────────┐  ┌───────────────┐│
│  │Conflict      │  │RollbackManager  │  │StateEvent     ││
│  │Resolver      │  │                 │  │Handler        ││
│  │              │  │ - Checkpoints   │  │               ││
│  │ - Strategies │  │ - Restoration   │  │ - AG-UI       ││
│  │ - Resolution │  │ - Compression   │  │   events      ││
│  └──────────────┘  └─────────────────┘  └───────────────┘│
└─────────────────────────────────────────────────────────────┘
```

## Features

### Core Features
- **Thread-safe state management** with concurrent access control
- **Automatic change tracking** with delta computation
- **Conflict resolution** for concurrent modifications
- **State validation** with custom rules and schemas
- **Rollback capabilities** with checkpoint management
- **Event integration** with AG-UI protocol
- **Performance optimizations** including caching and batching

### Advanced Features
- **Pluggable storage backends** (in-memory, file, distributed)
- **Compression support** for storage and checkpoints
- **Metrics collection** for monitoring
- **Transaction support** with atomic operations
- **State migration** utilities
- **Query capabilities** with filtering

## Quick Start

### Basic Usage

```go
package main

import (
    "log"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/state"
)

func main() {
    // Create state manager with default options
    manager, err := state.NewStateManager(state.DefaultManagerOptions())
    if err != nil {
        log.Fatal(err)
    }
    defer manager.Close()

    // Create a context for state operations
    contextID, err := manager.CreateContext("user-123", map[string]interface{}{
        "session": "abc123",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Update state
    delta, err := manager.UpdateState(contextID, "user-123", map[string]interface{}{
        "name": "John Doe",
        "email": "john@example.com",
        "preferences": map[string]interface{}{
            "theme": "dark",
            "language": "en",
        },
    }, state.UpdateOptions{})
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("State updated with %d changes", len(delta.Changes))

    // Retrieve state
    currentState, err := manager.GetState(contextID, "user-123")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Current state: %+v", currentState)
}
```

### Advanced Configuration

```go
// Create manager with custom options
options := state.ManagerOptions{
    // Storage configuration
    StoreOptions: state.StoreOptions{
        Backend: state.FileBackend,
        Path:    "/var/lib/agui/state",
        Compression: true,
    },
    
    // Conflict resolution
    ConflictStrategy: state.CustomMerge,
    MaxRetries: 5,
    
    // Validation
    StrictMode: true,
    ValidationRules: []state.ValidationRule{
        state.RequiredFields("id", "type"),
        state.TypeValidation("age", "number"),
        state.RangeValidation("score", 0, 100),
    },
    
    // Performance
    EnableCaching: true,
    CacheSize: 10000,
    EnableBatching: true,
    BatchSize: 100,
    
    // Monitoring
    EnableMetrics: true,
    EnableTracing: true,
}

manager, err := state.NewStateManager(options)
```

## Component Details

### StateStore
Handles persistent storage of state data with support for:
- Multiple backend implementations
- Automatic versioning
- History tracking
- Caching layer
- Compression

### DeltaComputer
Computes differences between state versions:
- Efficient diff algorithms
- Deep object comparison
- Change categorization
- Path-based updates

### ConflictResolver
Resolves conflicts in concurrent updates:
- Multiple resolution strategies (LastWriteWins, FirstWriteWins, CustomMerge)
- Automatic retry with backoff
- Conflict detection and reporting
- Custom resolution functions

### StateValidator
Validates state against defined rules:
- Schema-based validation
- Custom validation functions
- Type checking
- Range and constraint validation
- Conditional rules

### RollbackManager
Manages state checkpoints and rollback:
- Automatic checkpoint creation
- Named checkpoints
- Checkpoint compression
- Point-in-time recovery
- Checkpoint cleanup

### StateEventHandler
Integrates with AG-UI event system:
- State change events
- Event filtering and routing
- Async event processing
- Event persistence
- Retry mechanisms

## Usage Patterns

### 1. User Session Management

```go
// Initialize user session
contextID, _ := manager.CreateContext("user-123", map[string]interface{}{
    "ip": "192.168.1.1",
    "userAgent": "Mozilla/5.0...",
})

// Track user activity
manager.UpdateState(contextID, "user-123", map[string]interface{}{
    "lastActivity": time.Now(),
    "pageViews": 5,
}, state.UpdateOptions{
    CreateCheckpoint: true,
    CheckpointName: "session-start",
})

// Subscribe to changes
manager.Subscribe(func(event *protocols.Event) error {
    log.Printf("User state changed: %v", event.Data)
    return nil
}, state.EventFilter{
    Types: []string{"state.updated"},
    StateIDs: []string{"user-123"},
})
```

### 2. Configuration Management

```go
// Define validation rules for config
rules := []state.ValidationRule{
    state.RequiredFields("version", "services"),
    state.CustomRule("valid-ports", func(s *state.State) error {
        // Validate port numbers
        return nil
    }),
}

// Create manager with validation
options := state.DefaultManagerOptions()
options.ValidationRules = rules
manager, _ := state.NewStateManager(options)

// Update configuration with validation
_, err := manager.UpdateState(contextID, "app-config", map[string]interface{}{
    "version": "2.0",
    "services": map[string]interface{}{
        "api": map[string]interface{}{
            "port": 8080,
            "enabled": true,
        },
    },
}, state.UpdateOptions{})
```

### 3. Collaborative Editing

```go
// Configure for collaborative scenarios
options := state.ManagerOptions{
    ConflictStrategy: state.CustomMerge,
    EnableBatching: true,
    BatchTimeout: 50 * time.Millisecond,
}
manager, _ := state.NewStateManager(options)

// Define custom merge function
state.RegisterMergeFunction("document", func(base, local, remote map[string]interface{}) (map[string]interface{}, error) {
    // Implement operational transformation
    return merged, nil
})

// Handle concurrent edits
delta1, _ := manager.UpdateState(context1, "doc-1", edits1, state.UpdateOptions{
    ConflictStrategy: state.CustomMerge,
})

delta2, _ := manager.UpdateState(context2, "doc-1", edits2, state.UpdateOptions{
    ConflictStrategy: state.CustomMerge,
})
```

### 4. State Migration

```go
// Define migration function
migrator := state.NewMigrator()
migrator.AddMigration("1.0", "2.0", func(oldState map[string]interface{}) (map[string]interface{}, error) {
    // Transform state structure
    newState := make(map[string]interface{})
    newState["version"] = "2.0"
    newState["data"] = oldState
    return newState, nil
})

// Apply migration
newState, err := migrator.Migrate(oldState, "1.0", "2.0")
```

## Performance Considerations

### Caching
- Enable caching for frequently accessed states
- Configure cache size based on memory constraints
- Use TTL to prevent stale data

### Batching
- Enable batching for high-throughput scenarios
- Adjust batch size and timeout for optimal performance
- Monitor queue sizes to prevent overload

### Compression
- Enable compression for large state objects
- Use checkpoint compression for storage efficiency
- Consider CPU vs storage tradeoffs

### Metrics
- Monitor active contexts and queue sizes
- Track update latency and throughput
- Set up alerts for performance degradation

## Best Practices

1. **Context Management**
   - Create contexts for logical operation groups
   - Include relevant metadata in contexts
   - Clean up contexts when no longer needed

2. **Validation**
   - Define validation rules early
   - Use strict mode in production
   - Implement custom rules for business logic

3. **Error Handling**
   - Always check errors from state operations
   - Implement retry logic for transient failures
   - Log errors with context information

4. **Event Handling**
   - Subscribe to relevant events only
   - Process events asynchronously
   - Implement idempotent handlers

5. **Checkpointing**
   - Use automatic checkpoints for safety
   - Create named checkpoints before risky operations
   - Regularly clean up old checkpoints

## Troubleshooting

### Common Issues

1. **Validation Failures**
   - Check validation rules match state structure
   - Verify required fields are present
   - Enable debug logging for validation details

2. **Conflict Resolution**
   - Review conflict strategy selection
   - Implement custom merge for complex cases
   - Monitor retry attempts

3. **Performance Issues**
   - Check cache hit rates
   - Monitor queue sizes
   - Adjust batching parameters
   - Enable metrics collection

4. **Memory Usage**
   - Limit cache size
   - Enable checkpoint compression
   - Implement state cleanup policies

## Examples

Additional examples can be found in the `examples/` directory:
- `basic/` - Simple state management
- `validation/` - Custom validation rules
- `events/` - Event handling patterns
- `distributed/` - Distributed state scenarios
- `migration/` - State migration examples

## Contributing

See the main SDK contributing guide for general guidelines. For state package specific contributions:

1. Add tests for new features
2. Update documentation
3. Follow existing patterns
4. Benchmark performance impacts
5. Consider backward compatibility

## License

Part of the AG-UI Go SDK. See LICENSE file in the repository root.