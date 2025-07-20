# Transport Abstraction Migration Guide

This document provides comprehensive guidance for migrating from old transport patterns to the new composable transport abstraction system.

## Overview

The transport package has been refactored to provide a more modular, composable, and maintainable architecture. The new system breaks down the monolithic Transport interface into smaller, focused interfaces that can be composed together.

## Key Changes

### 1. Interface Composition

**Before:**
```go
type Transport interface {
    Send(event Event) error
    Receive() (Event, error)
    Connect() error
    Close() error
    Stats() Stats
    Config() Config
    // ... many more methods
}
```

**After:**
```go
// Core composable interfaces
type Transport interface {
    Connector
    Sender
    Receiver
    ConfigProvider
    StatsProvider
}

// Specialized transport types
type StreamingTransport interface {
    Transport
    BatchSender
    EventHandlerProvider
    StreamController
    StreamingStatsProvider
}

type ReliableTransport interface {
    Transport
    ReliableSender
    AckHandlerProvider
    ReliabilityStatsProvider
}
```

### 2. Event Handling

**Before:**
```go
func HandleEvent(event Event) error {
    // Process event
    return nil
}
```

**After:**
```go
// Use EventHandler callback type
type EventHandler func(ctx context.Context, event events.Event) error

transport.SetEventHandler(func(ctx context.Context, event events.Event) error {
    // Process event
    return nil
})
```

### 3. Batch Operations

**Before:**
```go
// Custom batch implementation
func SendBatch(transport Transport, events []Event) error {
    for _, event := range events {
        if err := transport.Send(event); err != nil {
            return err
        }
    }
    return nil
}
```

**After:**
```go
// Use BatchSender interface
if batchSender, ok := transport.(BatchSender); ok {
    return batchSender.SendBatch(ctx, events)
}
```

### 4. Streaming

**Before:**
```go
sendCh, receiveCh, errorCh := transport.StartStream()
```

**After:**
```go
sendCh, receiveCh, errorCh, err := transport.StartStreaming(ctx)
if err != nil {
    return fmt.Errorf("failed to start streaming: %w", err)
}
```

### 5. Reliability Features

**Before:**
```go
err := transport.SendWithAck(event, timeout)
```

**After:**
```go
if reliableTransport, ok := transport.(ReliableTransport); ok {
    err := reliableTransport.SendEventWithAck(ctx, event, timeout)
}
```

## Migration Steps

### Step 1: Assessment

Use the migration tool to assess your current codebase:

```go
config := &MigrationConfig{
    SourceDir:           "./pkg/transport",
    DryRun:              true,  // Don't modify files yet
    DeprecationDeadline: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
}

migrator := NewTransportMigrator(config)
report, err := migrator.Migrate()
```

### Step 2: Update Interface Implementations

1. **Split monolithic interfaces:**
   ```go
   // Before
   type MyTransport struct {
       // ... fields
   }
   
   func (t *MyTransport) Send(event Event) error { /* ... */ }
   func (t *MyTransport) Receive() (Event, error) { /* ... */ }
   // ... all methods in one struct
   
   // After
   type MyTransport struct {
       // ... fields
   }
   
   // Implement individual interfaces
   func (t *MyTransport) Send(ctx context.Context, event TransportEvent) error { /* ... */ }
   func (t *MyTransport) Channels() (<-chan events.Event, <-chan error) { /* ... */ }
   ```

2. **Use interface composition:**
   ```go
   // Ensure your transport implements the composed interface
   var _ Transport = (*MyTransport)(nil)
   
   // For specialized transports
   var _ StreamingTransport = (*MyStreamingTransport)(nil)
   ```

### Step 3: Update Usage Patterns

1. **Replace direct method calls with interface-based patterns:**
   ```go
   // Before
   stats := transport.Stats()
   
   // After
   if statsProvider, ok := transport.(StatsProvider); ok {
       stats := statsProvider.Stats()
   }
   ```

2. **Use context everywhere:**
   ```go
   // Before
   err := transport.Send(event)
   
   // After
   ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
   defer cancel()
   err := transport.Send(ctx, event)
   ```

### Step 4: Handle Deprecations

1. **Update deprecated method calls:**
   ```go
   // Deprecated: Will be removed on 2025-03-31
   data := event.Data()
   
   // Use typed methods instead
   if typedEvent, ok := event.(TypedEvent); ok {
       data := typedEvent.GetPayload()
   }
   ```

2. **Add migration timeline to your planning:**
   - **2024-12-31**: Remove deprecated Transport interface methods
   - **2025-01-31**: Remove deprecated streaming APIs
   - **2025-03-31**: Remove deprecated event data access methods

### Step 5: Test and Validate

1. **Use migration test helpers:**
   ```go
   func TestMigration(t *testing.T) {
       suite := NewMigrationTestSuite(t)
       defer suite.Cleanup()
       
       // Test interface composition
       suite.TestInterfaceComposition()
       
       // Test backward compatibility
       suite.TestBackwardCompatibility(oldCode, newCode)
   }
   ```

2. **Validate with mock implementations:**
   ```go
   transport := NewMockTransport()
   
   // Test all interface methods
   ctx := context.Background()
   err := transport.Connect(ctx)
   // ... test other methods
   ```

## Automated Migration Tool

The migration tool can automatically transform most patterns:

```bash
# Analyze codebase
go run migration_tool.go -source ./src -dry-run

# Apply transformations
go run migration_tool.go -source ./src -backup

# Generate deprecation annotations
go run migration_tool.go -source ./src -deprecate
```

### Migration Tool Features

1. **AST-based transformations**
2. **Deprecation detection and annotation**
3. **Backward compatibility validation**
4. **Comprehensive reporting**
5. **Dry-run mode for safe analysis**

## Common Migration Patterns

### Pattern 1: Simple Transport Usage

**Before:**
```go
func useTransport(transport Transport) error {
    if err := transport.Connect(); err != nil {
        return err
    }
    defer transport.Close()
    
    event := createEvent("test")
    return transport.Send(event)
}
```

**After:**
```go
func useTransport(transport Transport) error {
    ctx := context.Background()
    
    if err := transport.Connect(ctx); err != nil {
        return err
    }
    defer transport.Close(ctx)
    
    event := createEvent("test")
    return transport.Send(ctx, event)
}
```

### Pattern 2: Event Processing

**Before:**
```go
for {
    event, err := transport.Receive()
    if err != nil {
        log.Printf("Error: %v", err)
        continue
    }
    processEvent(event)
}
```

**After:**
```go
eventCh, errorCh := transport.Channels()

for {
    select {
    case event := <-eventCh:
        processEvent(event)
    case err := <-errorCh:
        log.Printf("Error: %v", err)
    }
}
```

### Pattern 3: Streaming Operations

**Before:**
```go
sendCh, receiveCh, errorCh := transport.StartStream()
```

**After:**
```go
if streamingTransport, ok := transport.(StreamingTransport); ok {
    sendCh, receiveCh, errorCh, err := streamingTransport.StartStreaming(ctx)
    if err != nil {
        return fmt.Errorf("streaming failed: %w", err)
    }
    // Use channels...
}
```

## Best Practices

### 1. Interface Segregation

- Implement only the interfaces you need
- Use composition to combine functionality
- Keep interfaces focused and cohesive

### 2. Context Usage

- Always pass context to methods
- Set appropriate timeouts
- Handle context cancellation

### 3. Error Handling

- Check interface implementations before casting
- Provide fallback behavior for optional interfaces
- Log errors with sufficient context

### 4. Testing

- Use mock implementations for testing
- Test interface composition
- Validate backward compatibility

### 5. Documentation

- Document interface implementations
- Provide usage examples
- Maintain migration guides

## Troubleshooting

### Common Issues

1. **Interface Not Implemented**
   ```
   Error: interface conversion: *MyTransport does not implement BatchSender
   ```
   
   **Solution:** Check if your type implements all required methods or use interface checks:
   ```go
   if batchSender, ok := transport.(BatchSender); ok {
       // Use batch functionality
   }
   ```

2. **Context Timeout**
   ```
   Error: context deadline exceeded
   ```
   
   **Solution:** Increase timeout or handle context cancellation:
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   ```

3. **Deprecated Method Usage**
   ```
   Warning: method HandleEvent is deprecated
   ```
   
   **Solution:** Replace with new patterns:
   ```go
   // Instead of HandleEvent method
   transport.SetEventHandler(func(ctx context.Context, event events.Event) error {
       return handleEvent(event)
   })
   ```

## Support and Resources

- **Documentation:** See `doc_generator.go` for API documentation
- **Examples:** Check interface files for comprehensive examples
- **Testing:** Use `migration_test_helpers.go` for validation
- **Migration Tool:** Use `migration_tool.go` for automated transformations

## Timeline

| Date | Milestone |
|------|-----------|
| 2024-11-30 | Migration tool available |
| 2024-12-31 | Old Transport interface deprecated |
| 2025-01-31 | Streaming APIs migration complete |
| 2025-03-31 | All deprecated methods removed |

For additional support, please refer to the inline documentation and examples provided in the interface files.