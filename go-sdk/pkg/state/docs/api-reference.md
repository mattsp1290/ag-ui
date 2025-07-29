# AG-UI State Management API Reference

This document provides comprehensive API documentation for the AG-UI state management system, including all public interfaces, methods, and usage examples.

## Table of Contents

1. [StateManager](#statemanager)
2. [Configuration](#configuration)
3. [Core Operations](#core-operations)
4. [Storage Backends](#storage-backends)
5. [Validation System](#validation-system)
6. [Event System](#event-system)
7. [Monitoring & Metrics](#monitoring--metrics)
8. [Error Handling](#error-handling)
9. [Types & Interfaces](#types--interfaces)
10. [Examples](#examples)

## StateManager

The `StateManager` is the main entry point for all state operations.

### Constructor

#### `NewStateManager(options ManagerOptions) (*StateManager, error)`

Creates a new state manager instance with the specified configuration.

**Parameters:**
- `options` - Configuration options for the state manager

**Returns:**
- `*StateManager` - The state manager instance
- `error` - Any initialization error

**Example:**
```go
options := state.DefaultManagerOptions()
options.EnableCaching = true
options.CacheSize = 5000

manager, err := state.NewStateManager(options)
if err != nil {
    log.Fatal("Failed to create state manager:", err)
}
defer manager.Close()
```

### Core Methods

#### `CreateContext(userID string, metadata map[string]interface{}) (string, error)`

Creates a new context for state operations.

**Parameters:**
- `userID` - User identifier for the context
- `metadata` - Additional context metadata

**Returns:**
- `string` - Context ID
- `error` - Any creation error

**Example:**
```go
contextID, err := manager.CreateContext("user-123", map[string]interface{}{
    "session": "sess-456",
    "ip": "192.168.1.1",
    "userAgent": "Mozilla/5.0...",
})
```

#### `UpdateState(contextID, stateID string, data map[string]interface{}, options UpdateOptions) (*Delta, error)`

Updates state data and returns the computed delta.

**Parameters:**
- `contextID` - Context for the operation
- `stateID` - Unique identifier for the state
- `data` - State data to update
- `options` - Update configuration options

**Returns:**
- `*Delta` - Computed changes
- `error` - Any update error

**Example:**
```go
delta, err := manager.UpdateState(contextID, "user-profile", map[string]interface{}{
    "name": "John Doe",
    "email": "john@example.com",
    "preferences": map[string]interface{}{
        "theme": "dark",
        "notifications": true,
    },
}, state.UpdateOptions{
    CreateCheckpoint: true,
    CheckpointName: "profile-update",
    ConflictStrategy: state.LastWriteWins,
})
```

#### `GetState(contextID, stateID string) (map[string]interface{}, error)`

Retrieves current state data.

**Parameters:**
- `contextID` - Context for the operation
- `stateID` - State identifier

**Returns:**
- `map[string]interface{}` - Current state data
- `error` - Any retrieval error

**Example:**
```go
currentState, err := manager.GetState(contextID, "user-profile")
if err != nil {
    log.Printf("Failed to get state: %v", err)
    return
}

name, ok := currentState["name"].(string)
if ok {
    log.Printf("User name: %s", name)
}
```

#### `DeleteState(contextID, stateID string) error`

Deletes state data and its history.

**Parameters:**
- `contextID` - Context for the operation
- `stateID` - State identifier

**Returns:**
- `error` - Any deletion error

**Example:**
```go
err := manager.DeleteState(contextID, "temp-session")
if err != nil {
    log.Printf("Failed to delete state: %v", err)
}
```

#### `GetStateHistory(contextID, stateID string, limit int) ([]*StateVersion, error)`

Retrieves version history for a state.

**Parameters:**
- `contextID` - Context for the operation
- `stateID` - State identifier
- `limit` - Maximum number of versions to return

**Returns:**
- `[]*StateVersion` - Version history
- `error` - Any retrieval error

**Example:**
```go
history, err := manager.GetStateHistory(contextID, "user-profile", 10)
if err != nil {
    log.Printf("Failed to get history: %v", err)
    return
}

for _, version := range history {
    log.Printf("Version %d created at %s", version.Version, version.CreatedAt)
}
```

#### `CreateCheckpoint(contextID, stateID, name string) error`

Creates a named checkpoint for rollback purposes.

**Parameters:**
- `contextID` - Context for the operation
- `stateID` - State identifier
- `name` - Checkpoint name

**Returns:**
- `error` - Any creation error

**Example:**
```go
err := manager.CreateCheckpoint(contextID, "user-profile", "before-migration")
if err != nil {
    log.Printf("Failed to create checkpoint: %v", err)
}
```

#### `RollbackToCheckpoint(contextID, stateID, checkpointName string) error`

Rolls back state to a specific checkpoint.

**Parameters:**
- `contextID` - Context for the operation
- `stateID` - State identifier
- `checkpointName` - Name of the checkpoint

**Returns:**
- `error` - Any rollback error

**Example:**
```go
err := manager.RollbackToCheckpoint(contextID, "user-profile", "before-migration")
if err != nil {
    log.Printf("Failed to rollback: %v", err)
}
```

#### `Subscribe(handler EventHandler, filter EventFilter) error`

Subscribes to state change events.

**Parameters:**
- `handler` - Event handler function
- `filter` - Event filtering criteria

**Returns:**
- `error` - Any subscription error

**Example:**
```go
err := manager.Subscribe(func(event *Event) error {
    log.Printf("State changed: %s", event.StateID)
    return nil
}, state.EventFilter{
    Types: []string{"state.updated"},
    StateIDs: []string{"user-*"},
})
```

#### `Close() error`

Gracefully shuts down the state manager.

**Returns:**
- `error` - Any shutdown error

**Example:**
```go
err := manager.Close()
if err != nil {
    log.Printf("Error during shutdown: %v", err)
}
```

## Configuration

### ManagerOptions

Configuration structure for the StateManager.

```go
type ManagerOptions struct {
    // Storage configuration
    MaxHistorySize int                    // Maximum versions to keep
    EnableCaching  bool                   // Enable in-memory caching
    
    // Conflict resolution
    ConflictStrategy ConflictResolutionStrategy // Resolution strategy
    MaxRetries       int                        // Maximum retry attempts
    RetryDelay       time.Duration             // Delay between retries
    
    // Validation
    ValidationRules []ValidationRule // Custom validation rules
    StrictMode      bool            // Enable strict validation
    
    // Rollback configuration
    MaxCheckpoints       int           // Maximum checkpoints to keep
    CheckpointInterval   time.Duration // Automatic checkpoint interval
    AutoCheckpoint       bool          // Enable automatic checkpoints
    CompressCheckpoints  bool          // Enable checkpoint compression
    
    // Event handling
    EventBufferSize      int           // Event buffer size
    ProcessingWorkers    int           // Number of event workers
    EventRetryBackoff    time.Duration // Event retry backoff
    
    // Performance
    CacheSize          int           // Cache size limit
    CacheTTL           time.Duration // Cache TTL
    EnableCompression  bool          // Enable data compression
    EnableBatching     bool          // Enable batch processing
    BatchSize          int           // Batch size limit
    BatchTimeout       time.Duration // Batch timeout
    
    // Monitoring
    EnableMetrics      bool          // Enable metrics collection
    MetricsInterval    time.Duration // Metrics collection interval
    EnableTracing      bool          // Enable distributed tracing
    
    // Audit
    EnableAudit        bool          // Enable audit logging
    AuditLogger        AuditLogger   // Custom audit logger
}
```

### UpdateOptions

Options for state update operations.

```go
type UpdateOptions struct {
    CreateCheckpoint    bool                        // Create checkpoint before update
    CheckpointName      string                      // Custom checkpoint name
    ConflictStrategy    ConflictResolutionStrategy  // Override conflict strategy
    ValidateOnly        bool                        // Only validate, don't update
    SkipValidation      bool                        // Skip validation
    SkipEvents          bool                        // Skip event emission
    Metadata            map[string]interface{}      // Additional metadata
}
```

### Default Options

```go
func DefaultManagerOptions() ManagerOptions {
    return ManagerOptions{
        MaxHistorySize:       100,
        EnableCaching:        true,
        ConflictStrategy:     LastWriteWins,
        MaxRetries:           3,
        RetryDelay:           100 * time.Millisecond,
        StrictMode:           true,
        MaxCheckpoints:       10,
        CheckpointInterval:   5 * time.Minute,
        AutoCheckpoint:       true,
        CompressCheckpoints:  true,
        EventBufferSize:      1000,
        ProcessingWorkers:    4,
        EventRetryBackoff:    time.Second,
        CacheSize:            1000,
        CacheTTL:             5 * time.Minute,
        EnableCompression:    true,
        EnableBatching:       true,
        BatchSize:            100,
        BatchTimeout:         100 * time.Millisecond,
        EnableMetrics:        true,
        MetricsInterval:      30 * time.Second,
        EnableTracing:        false,
        EnableAudit:          true,
    }
}
```

## Core Operations

### State Management

#### Basic State Operations

```go
// Create context
contextID, err := manager.CreateContext("user-123", map[string]interface{}{
    "session": "sess-456",
})

// Update state
delta, err := manager.UpdateState(contextID, "user-profile", map[string]interface{}{
    "name": "John Doe",
    "age": 30,
}, state.UpdateOptions{})

// Get current state
currentState, err := manager.GetState(contextID, "user-profile")

// Delete state
err = manager.DeleteState(contextID, "user-profile")
```

#### Batch Operations

```go
// Batch multiple updates
batch := []state.BatchOperation{
    {
        Type:    "update",
        StateID: "user-profile",
        Data:    map[string]interface{}{"name": "John"},
    },
    {
        Type:    "update",
        StateID: "user-settings",
        Data:    map[string]interface{}{"theme": "dark"},
    },
}

results, err := manager.BatchOperations(contextID, batch)
```

#### Transactional Operations

```go
// Begin transaction
tx, err := manager.BeginTransaction(contextID)
if err != nil {
    log.Fatal(err)
}

// Perform operations
err = tx.UpdateState("user-profile", userData)
if err != nil {
    tx.Rollback()
    log.Fatal(err)
}

err = tx.UpdateState("user-settings", settingsData)
if err != nil {
    tx.Rollback()
    log.Fatal(err)
}

// Commit transaction
err = tx.Commit()
if err != nil {
    log.Fatal(err)
}
```

### Version Management

#### Working with Versions

```go
// Get specific version
version, err := manager.GetStateVersion(contextID, "user-profile", 5)

// Get version history
history, err := manager.GetStateHistory(contextID, "user-profile", 10)

// Compare versions
delta, err := manager.CompareVersions(contextID, "user-profile", 3, 5)

// Rollback to version
err = manager.RollbackToVersion(contextID, "user-profile", 3)
```

#### Checkpoint Management

```go
// Create named checkpoint
err := manager.CreateCheckpoint(contextID, "user-profile", "before-update")

// List checkpoints
checkpoints, err := manager.ListCheckpoints(contextID, "user-profile")

// Rollback to checkpoint
err = manager.RollbackToCheckpoint(contextID, "user-profile", "before-update")

// Delete checkpoint
err = manager.DeleteCheckpoint(contextID, "user-profile", "before-update")
```

## Storage Backends

### Backend Configuration

```go
// File backend
options := state.DefaultManagerOptions()
options.StorageBackend = "file"
options.StorageConfig = &state.StorageConfig{
    Path:        "/var/lib/agui/state",
    Compression: true,
    BackupInterval: 1 * time.Hour,
}

// Redis backend
options.StorageBackend = "redis"
options.StorageConfig = &state.StorageConfig{
    ConnectionURL: os.Getenv("REDIS_URL"), // Example: "redis://localhost:6379"
    Database:      0,
    MaxRetries:    3,
    PoolSize:      10,
}

// PostgreSQL backend
options.StorageBackend = "postgres"
options.StorageConfig = &state.StorageConfig{
    ConnectionURL: os.Getenv("DATABASE_URL"), // Example: "postgres://user:pass@localhost/statedb"
    MaxOpenConns:  25,
    MaxIdleConns:  5,
    ConnMaxLifetime: 5 * time.Minute,
}
```

### Custom Storage Backend

```go
// Implement StorageBackend interface
type MyStorageBackend struct {
    config *state.StorageConfig
}

func (b *MyStorageBackend) GetState(ctx context.Context, stateID string) (map[string]interface{}, error) {
    // Custom implementation
}

func (b *MyStorageBackend) SetState(ctx context.Context, stateID string, data map[string]interface{}) error {
    // Custom implementation
}

// Register backend
state.RegisterStorageBackend("custom", func(config *state.StorageConfig) (state.StorageBackend, error) {
    return &MyStorageBackend{config: config}, nil
})

// Use custom backend
options.StorageBackend = "custom"
```

## Validation System

### Built-in Validation Rules

```go
// Required fields
rule := state.RequiredFields("name", "email", "age")

// Type validation
rule = state.TypeValidation("age", "number")
rule = state.TypeValidation("email", "string")

// Range validation
rule = state.RangeValidation("age", 0, 120)
rule = state.RangeValidation("score", 0.0, 100.0)

// Pattern validation
rule = state.PatternValidation("email", `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Custom validation
rule = state.CustomRule("valid-phone", func(state *state.State) error {
    phone, ok := state.Data["phone"].(string)
    if !ok {
        return errors.New("phone must be a string")
    }
    if !isValidPhone(phone) {
        return errors.New("invalid phone format")
    }
    return nil
})
```

### Validation Configuration

```go
options := state.DefaultManagerOptions()
options.ValidationRules = []state.ValidationRule{
    state.RequiredFields("id", "type"),
    state.TypeValidation("age", "number"),
    state.RangeValidation("score", 0, 100),
    state.CustomRule("business-logic", businessValidation),
}
options.StrictMode = true // Fail on validation errors
```

### Conditional Validation

```go
// Validate email only if user type is "premium"
rule := state.ConditionalRule(
    func(state *state.State) bool {
        userType, _ := state.Data["type"].(string)
        return userType == "premium"
    },
    state.RequiredFields("email"),
)
```

## Event System

### Event Types

```go
const (
    EventStateCreated       = "state.created"
    EventStateUpdated       = "state.updated"
    EventStateDeleted       = "state.deleted"
    EventStateConflict      = "state.conflict"
    EventValidationFailed   = "state.validation_failed"
    EventCheckpointCreated  = "state.checkpoint_created"
    EventRollbackPerformed  = "state.rollback_performed"
)
```

### Event Subscription

```go
// Subscribe to all state updates
err := manager.Subscribe(func(event *state.Event) error {
    log.Printf("State event: %s for %s", event.Type, event.StateID)
    return nil
}, state.EventFilter{
    Types: []string{state.EventStateUpdated},
})

// Subscribe to specific state IDs
err = manager.Subscribe(func(event *state.Event) error {
    // Handle user profile updates
    return nil
}, state.EventFilter{
    Types: []string{state.EventStateUpdated},
    StateIDs: []string{"user-profile"},
})

// Subscribe with pattern matching
err = manager.Subscribe(func(event *state.Event) error {
    // Handle all user-related states
    return nil
}, state.EventFilter{
    Types: []string{state.EventStateUpdated},
    StateIDPattern: "user-*",
})
```

### Event Handler Examples

```go
// Audit logging handler
auditHandler := func(event *state.Event) error {
    auditLog := map[string]interface{}{
        "timestamp": time.Now(),
        "event_type": event.Type,
        "state_id": event.StateID,
        "user_id": event.UserID,
        "changes": event.Delta.Changes,
    }
    
    return auditLogger.Log(auditLog)
}

// Notification handler
notificationHandler := func(event *state.Event) error {
    if event.Type == state.EventStateUpdated {
        return notificationService.SendUpdate(event.StateID, event.Delta)
    }
    return nil
}

// Cache invalidation handler
cacheHandler := func(event *state.Event) error {
    return cache.Invalidate(event.StateID)
}
```

## Monitoring & Metrics

### Metrics Collection

```go
// Enable metrics
options := state.DefaultManagerOptions()
options.EnableMetrics = true
options.MetricsInterval = 30 * time.Second

// Custom metrics
metrics := state.NewMetrics("my-app", "state-mgr")
options.MetricsCollector = metrics

// Prometheus integration
registry := prometheus.NewRegistry()
options.PrometheusRegistry = registry
```

### Available Metrics

```go
// Operation metrics
agui_state_operations_total{operation="update",result="success"} 1234
agui_state_operations_total{operation="update",result="error"} 5

// Performance metrics
agui_state_operation_duration_seconds{operation="update",quantile="0.5"} 0.015
agui_state_operation_duration_seconds{operation="update",quantile="0.95"} 0.045

// System metrics
agui_state_active_contexts 25
agui_state_cache_hits_total 5678
agui_state_cache_misses_total 123
agui_state_queue_size 0

// Business metrics
agui_state_conflicts_total{strategy="last_write_wins"} 3
agui_state_validation_failures_total{rule="required_fields"} 2
```

### Health Checks

```go
// Health check endpoints
health := state.NewHealthChecker(manager)

// Check overall health
status := health.Check()
if status.Status != "healthy" {
    log.Printf("Health check failed: %s", status.Message)
}

// Check specific components
storageHealth := health.CheckStorage()
eventHealth := health.CheckEvents()
```

## Error Handling

### Error Types

```go
// Common error types
var (
    ErrStateNotFound        = errors.New("state not found")
    ErrContextNotFound      = errors.New("context not found")
    ErrValidationFailed     = errors.New("validation failed")
    ErrConflictResolution   = errors.New("conflict resolution failed")
    ErrStorageBackend       = errors.New("storage backend error")
    ErrEventProcessing      = errors.New("event processing failed")
    ErrManagerClosed        = errors.New("state manager is closed")
)

// Wrapped errors
type StateError struct {
    Op       string // Operation that failed
    StateID  string // State ID if relevant
    Err      error  // Underlying error
}

func (e *StateError) Error() string {
    return fmt.Sprintf("state operation %s failed for %s: %v", e.Op, e.StateID, e.Err)
}
```

### Error Handling Patterns

```go
// Check for specific error types
delta, err := manager.UpdateState(contextID, stateID, data, options)
if err != nil {
    var stateErr *state.StateError
    if errors.As(err, &stateErr) {
        switch stateErr.Op {
        case "validation":
            log.Printf("Validation failed: %v", stateErr.Err)
        case "conflict":
            log.Printf("Conflict resolution failed: %v", stateErr.Err)
        case "storage":
            log.Printf("Storage error: %v", stateErr.Err)
        }
        return
    }
    
    // Handle other errors
    log.Printf("Unexpected error: %v", err)
}

// Retry with backoff
func updateWithRetry(manager *state.StateManager, contextID, stateID string, data map[string]interface{}) error {
    backoff := time.Millisecond * 100
    maxRetries := 3
    
    for i := 0; i < maxRetries; i++ {
        _, err := manager.UpdateState(contextID, stateID, data, state.UpdateOptions{})
        if err == nil {
            return nil
        }
        
        // Check if error is retryable
        if !isRetryableError(err) {
            return err
        }
        
        time.Sleep(backoff)
        backoff *= 2
    }
    
    return fmt.Errorf("max retries exceeded")
}
```

## Types & Interfaces

### Core Types

```go
// State represents a state object
type State struct {
    ID        string                 `json:"id"`
    Version   int                    `json:"version"`
    Data      map[string]interface{} `json:"data"`
    Metadata  map[string]interface{} `json:"metadata"`
    CreatedAt time.Time             `json:"created_at"`
    UpdatedAt time.Time             `json:"updated_at"`
}

// Delta represents changes between states
type Delta struct {
    Changes   []Change               `json:"changes"`
    Metadata  map[string]interface{} `json:"metadata"`
    CreatedAt time.Time             `json:"created_at"`
}

// Change represents a single state change
type Change struct {
    Type     string      `json:"type"`     // "added", "modified", "deleted"
    Path     string      `json:"path"`     // JSONPath to changed field
    OldValue interface{} `json:"old_value,omitempty"`
    NewValue interface{} `json:"new_value,omitempty"`
}

// StateVersion represents a versioned state
type StateVersion struct {
    Version   int                    `json:"version"`
    Data      map[string]interface{} `json:"data"`
    Delta     *Delta                 `json:"delta,omitempty"`
    CreatedAt time.Time             `json:"created_at"`
}

// Event represents a state change event
type Event struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    StateID   string                 `json:"state_id"`
    UserID    string                 `json:"user_id"`
    Delta     *Delta                 `json:"delta,omitempty"`
    Metadata  map[string]interface{} `json:"metadata"`
    CreatedAt time.Time             `json:"created_at"`
}
```

### Interfaces

```go
// ValidationRule interface for custom validation
type ValidationRule interface {
    Validate(state *State) error
    Description() string
}

// EventHandler function type
type EventHandler func(event *Event) error

// StorageBackend interface for custom storage
type StorageBackend interface {
    GetState(ctx context.Context, stateID string) (map[string]interface{}, error)
    SetState(ctx context.Context, stateID string, state map[string]interface{}) error
    DeleteState(ctx context.Context, stateID string) error
    GetVersion(ctx context.Context, stateID string, version int) (*StateVersion, error)
    SaveVersion(ctx context.Context, stateID string, version *StateVersion) error
    GetVersionHistory(ctx context.Context, stateID string, limit int) ([]*StateVersion, error)
    Close() error
}

// AuditLogger interface for custom audit logging
type AuditLogger interface {
    Log(event *AuditEvent) error
    Query(filter AuditFilter) ([]*AuditEvent, error)
}
```

## Examples

### Complete Application Example

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/mattsp1290/ag-ui/go-sdk/pkg/state"
)

func main() {
    // Configure state manager
    options := state.DefaultManagerOptions()
    options.StorageBackend = "file"
    options.StorageConfig = &state.StorageConfig{
        Path: "/tmp/app-state",
        Compression: true,
    }
    
    // Add validation rules
    options.ValidationRules = []state.ValidationRule{
        state.RequiredFields("id", "type"),
        state.TypeValidation("age", "number"),
    }
    
    // Create manager
    manager, err := state.NewStateManager(options)
    if err != nil {
        log.Fatal("Failed to create state manager:", err)
    }
    defer manager.Close()
    
    // Create context
    contextID, err := manager.CreateContext("user-123", map[string]interface{}{
        "session": "sess-456",
        "ip": "192.168.1.1",
    })
    if err != nil {
        log.Fatal("Failed to create context:", err)
    }
    
    // Subscribe to events
    err = manager.Subscribe(func(event *state.Event) error {
        log.Printf("State event: %s for %s", event.Type, event.StateID)
        return nil
    }, state.EventFilter{
        Types: []string{state.EventStateUpdated},
    })
    if err != nil {
        log.Fatal("Failed to subscribe to events:", err)
    }
    
    // Update state
    userData := map[string]interface{}{
        "id":   "user-123",
        "type": "premium",
        "name": "John Doe",
        "age":  30,
        "preferences": map[string]interface{}{
            "theme": "dark",
            "notifications": true,
        },
    }
    
    delta, err := manager.UpdateState(contextID, "user-profile", userData, state.UpdateOptions{
        CreateCheckpoint: true,
        CheckpointName:   "initial-profile",
    })
    if err != nil {
        log.Fatal("Failed to update state:", err)
    }
    
    log.Printf("State updated with %d changes", len(delta.Changes))
    
    // Get current state
    currentState, err := manager.GetState(contextID, "user-profile")
    if err != nil {
        log.Fatal("Failed to get state:", err)
    }
    
    log.Printf("Current state: %+v", currentState)
    
    // Update preferences
    preferencesUpdate := map[string]interface{}{
        "preferences": map[string]interface{}{
            "theme": "light",
            "language": "en",
        },
    }
    
    _, err = manager.UpdateState(contextID, "user-profile", preferencesUpdate, state.UpdateOptions{})
    if err != nil {
        log.Fatal("Failed to update preferences:", err)
    }
    
    // Get version history
    history, err := manager.GetStateHistory(contextID, "user-profile", 10)
    if err != nil {
        log.Fatal("Failed to get history:", err)
    }
    
    log.Printf("State has %d versions", len(history))
    
    // Rollback to checkpoint
    time.Sleep(time.Second) // Wait for events to process
    
    err = manager.RollbackToCheckpoint(contextID, "user-profile", "initial-profile")
    if err != nil {
        log.Fatal("Failed to rollback:", err)
    }
    
    log.Println("Successfully rolled back to checkpoint")
}
```

### Advanced Usage Examples

#### Custom Validation Rule

```go
// Email validation rule
type EmailValidationRule struct {
    field string
}

func (r *EmailValidationRule) Validate(state *state.State) error {
    if email, ok := state.Data[r.field].(string); ok {
        if !isValidEmail(email) {
            return fmt.Errorf("invalid email format: %s", email)
        }
    }
    return nil
}

func (r *EmailValidationRule) Description() string {
    return fmt.Sprintf("validates email format for field: %s", r.field)
}

// Usage
options.ValidationRules = append(options.ValidationRules, &EmailValidationRule{
    field: "email",
})
```

#### Event-Driven Cache Updates

```go
// Cache invalidation on state updates
err := manager.Subscribe(func(event *state.Event) error {
    if event.Type == state.EventStateUpdated {
        // Invalidate cache for updated state
        return cache.Delete(event.StateID)
    }
    return nil
}, state.EventFilter{
    Types: []string{state.EventStateUpdated},
})
```

#### Conflict Resolution with Custom Merge

```go
// Custom merge function for user profiles
func mergeUserProfile(base, local, remote map[string]interface{}) (map[string]interface{}, error) {
    result := make(map[string]interface{})
    
    // Copy base
    for k, v := range base {
        result[k] = v
    }
    
    // Apply local changes
    for k, v := range local {
        result[k] = v
    }
    
    // Apply remote changes, with special handling for preferences
    for k, v := range remote {
        if k == "preferences" {
            // Merge preferences instead of replacing
            if basePrefs, ok := base["preferences"].(map[string]interface{}); ok {
                if remotePrefs, ok := v.(map[string]interface{}); ok {
                    mergedPrefs := make(map[string]interface{})
                    for pk, pv := range basePrefs {
                        mergedPrefs[pk] = pv
                    }
                    for pk, pv := range remotePrefs {
                        mergedPrefs[pk] = pv
                    }
                    result[k] = mergedPrefs
                    continue
                }
            }
        }
        result[k] = v
    }
    
    return result, nil
}

// Register the merge function
state.RegisterMergeFunction("user-profile", mergeUserProfile)

// Use custom merge strategy
delta, err := manager.UpdateState(contextID, "user-profile", data, state.UpdateOptions{
    ConflictStrategy: state.CustomMerge,
})
```

This API reference provides comprehensive coverage of the AG-UI state management system's public interface, with practical examples for common use cases and advanced scenarios.