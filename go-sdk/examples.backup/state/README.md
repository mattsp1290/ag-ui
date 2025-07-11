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

## Best Practices Demonstrated

### 1. State Organization
- Use hierarchical paths for logical organization
- Keep related data close in the state tree
- Use meaningful path names for clarity

### 2. Performance Optimization
- Batch multiple updates when possible
- Use delta events instead of full snapshots for updates
- Implement throttling for high-frequency updates
- Limit history size based on requirements

### 3. Conflict Resolution
- Choose appropriate resolution strategy for your use case
- Implement custom resolvers for complex scenarios
- Track conflict history for debugging
- Design state structure to minimize conflicts

### 4. Error Handling
- Always check errors from state operations
- Implement rollback mechanisms for failed transactions
- Use snapshots for recovery scenarios
- Monitor and log state synchronization errors

### 5. Scalability
- Use event streaming for real-time updates
- Implement proper batching for bulk operations
- Consider data locality in distributed systems
- Monitor performance metrics

## Common Patterns

### Subscription Management
```go
// Subscribe to specific path
unsubscribe := store.Subscribe("/user", func(change state.StateChange) {
    // Handle change
})
defer unsubscribe()
```

### Transaction Usage
```go
tx := store.Begin()
patch := state.JSONPatch{
    {Op: state.JSONPatchOpReplace, Path: "/field1", Value: "value1"},
    {Op: state.JSONPatchOpAdd, Path: "/field2", Value: "value2"},
}
if err := tx.Apply(patch); err != nil {
    tx.Rollback()
} else {
    tx.Commit()
}
```

### Event Handling
```go
handler := state.NewStateEventHandler(store,
    state.WithBatchSize(50),
    state.WithBatchTimeout(100*time.Millisecond),
    state.WithSnapshotCallback(func(event *events.StateSnapshotEvent) error {
        // Handle snapshot
        return nil
    }),
)
```

## Testing

Each example can be run independently and includes console output showing the operations being performed. The examples are self-contained and demonstrate both successful operations and error scenarios.

## Dependencies

All examples use the AG-UI Go SDK state management package:
```go
import (
    "github.com/ag-ui/go-sdk/pkg/state"
    "github.com/ag-ui/go-sdk/pkg/core/events"
)
```

## Further Reading

- See the main state package documentation in `/pkg/state/doc.go`
- Review the event system documentation in `/pkg/core/events/doc.go`
- Check the integration tests for more usage examples