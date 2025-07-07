# AG-UI Event Validation System

## Overview

The AG-UI Event Validation System provides comprehensive validation capabilities for event processing in the AG-UI Go SDK. It includes advanced features such as state transition validation, protocol sequence checking, timing constraints, performance optimization, and detailed debugging support.

## Features

### 1. Enhanced Validation Rules

#### State Transition Validation
- Validates state changes according to defined state machines
- Supports rollback capabilities for invalid transitions
- Maintains state history for debugging

#### Protocol Sequence Validation  
- Ensures events follow correct protocol sequences
- Validates prerequisite events are present
- Detects out-of-order or missing events

#### Timing Constraints
- Enforces rate limiting on event streams
- Detects timeout violations
- Monitors event frequency patterns

### 2. Performance Optimization

#### Object Pooling
- Reduces memory allocations through object reuse
- Bounded pools prevent unbounded memory growth
- Configurable pool sizes based on workload

#### Batch Processing
- Processes multiple events in batches for efficiency
- Configurable batch sizes and timeouts
- Concurrent batch workers for parallelization

#### Caching
- LRU cache for validation results
- Configurable cache size and TTL
- Hot path optimization for frequently validated events

### 3. Debugging and Metrics

#### Debug Sessions
- Record validation sessions for troubleshooting
- Capture event sequences and validation results
- Export sessions for offline analysis

#### Performance Metrics
- Real-time performance monitoring
- OpenTelemetry integration for observability
- Memory usage tracking and leak detection

#### Profiling Support
- CPU and memory profiling capabilities
- Performance regression detection
- Detailed timing breakdowns

## Configuration

### Basic Configuration

```go
import "github.com/ag-ui/go-sdk/pkg/core/events"

// Create validator with default configuration
validator := events.NewEventValidator()
validator.AddDefaultRules()

// Validate an event
ctx := context.Background()
result := validator.ValidateEvent(ctx, event)
if !result.IsValid {
    for _, err := range result.Errors {
        log.Printf("Validation error: %s", err.Message)
    }
}
```

### Performance Configuration

```go
// Configure performance settings
perfConfig := &events.PerformanceConfig{
    Mode:            events.BalancedMode,
    CacheTTL:        5 * time.Minute,
    CacheSize:       10000,
    WorkerPoolSize:  runtime.NumCPU() * 2,
    BatchSize:       100,
    EnableHotPath:   true,
    EnableAsync:     true,
}

validator := events.NewEventValidatorWithConfig(&events.ValidationConfig{
    Performance: perfConfig,
})
```

### Debug Configuration

```go
// Enable debug mode
debugConfig := &events.DebugConfig{
    Level:              events.DebugLevelDetailed,
    EnableCapture:      true,
    CaptureStackTraces: true,
    MaxSessions:        100,
    SessionTimeout:     30 * time.Minute,
}

validator.EnableDebug(debugConfig)

// Start a debug session
sessionID := validator.StartDebugSession("troubleshooting-session")

// ... perform validation ...

// Export debug data
debugData := validator.ExportDebugSession(sessionID)
```

## Usage Examples

### State Transition Validation

```go
// Define state transitions
transitions := map[string][]string{
    "idle":    {"running", "error"},
    "running": {"paused", "completed", "error"},
    "paused":  {"running", "stopped", "error"},
}

rule := events.NewStateTransitionRule("workflow", transitions)
validator.AddRule(rule)

// Validate state change
event := &events.StateChangedEvent{
    OldState: "idle",
    NewState: "completed", // Invalid transition
}

result := validator.ValidateEvent(ctx, event)
// Result will contain error about invalid state transition
```

### Protocol Sequence Validation

```go
// Define protocol sequence
sequence := []string{
    "connection.request",
    "connection.established",
    "data.transfer",
    "connection.close",
}

rule := events.NewProtocolSequenceRule("tcp", sequence)
validator.AddRule(rule)

// Events must follow the defined sequence
```

### Timing Constraints

```go
// Configure rate limiting
rule := events.NewRateLimitRule("api_calls", 100, time.Second)
validator.AddRule(rule)

// Configure timeout detection
timeoutRule := events.NewTimeoutRule("request_timeout", 30*time.Second)
validator.AddRule(timeoutRule)
```

## Performance Tuning

### Memory Optimization

```go
// Configure bounded object pools
perfOpts := state.PerformanceOptions{
    EnablePooling:   true,
    MaxPoolSize:     10000,
    MaxIdleObjects:  1000,
}

// Use performance optimizer
optimizer := state.NewPerformanceOptimizer(perfOpts)
```

### Batch Processing

```go
// Configure batch processing
perfConfig := &events.PerformanceConfig{
    EnableBatching:  true,
    BatchSize:       100,
    BatchTimeout:    10 * time.Millisecond,
    MaxConcurrency:  runtime.NumCPU() * 2,
}
```

### Cache Configuration

```go
// Configure validation cache
cacheConfig := &events.CacheConfig{
    Size: 10000,
    TTL:  5 * time.Minute,
}
```

## Monitoring and Metrics

### OpenTelemetry Integration

```go
// Set up OpenTelemetry provider
meterProvider := // ... initialize meter provider

metrics := events.NewValidationPerformanceMetrics()
metrics.SetMeterProvider(meterProvider)
metrics.InitializeOpenTelemetry()

// Metrics are automatically recorded during validation
```

### Available Metrics

- `validation_events_total` - Total events processed
- `validation_duration_ms` - Validation duration histogram
- `rule_execution_duration_ms` - Individual rule execution times
- `validation_errors_total` - Total validation errors
- `memory_usage_bytes` - Current memory usage

### Performance Analysis

```go
// Get performance metrics
metrics := validator.GetMetrics()
fmt.Printf("Events processed: %d\n", metrics.EventsProcessed)
fmt.Printf("Average duration: %v\n", metrics.AverageDuration)
fmt.Printf("Cache hit rate: %.2f%%\n", metrics.CacheHitRate)
```

## Migration Guide

### From Basic Validation

If you're currently using basic validation, upgrade to enhanced validation:

```go
// Old approach
if event.Type == "" {
    return errors.New("invalid event")
}

// New approach
validator := events.NewEventValidator()
validator.AddDefaultRules()
result := validator.ValidateEvent(ctx, event)
```

### Performance Improvements

1. Enable object pooling to reduce allocations
2. Use batch processing for high-throughput scenarios
3. Configure caching for repeated validations
4. Monitor metrics to identify bottlenecks

## Best Practices

1. **Context Usage**: Always pass context to validation methods for proper cancellation
2. **Rule Selection**: Only enable rules relevant to your use case
3. **Performance Mode**: Choose appropriate performance mode based on requirements
4. **Monitoring**: Set up metrics collection in production
5. **Debug Sessions**: Use debug sessions sparingly in production
6. **Resource Limits**: Configure bounded pools and caches to prevent memory issues

## Troubleshooting

### High Memory Usage

1. Check pool sizes and reduce if necessary
2. Verify cache eviction is working properly
3. Monitor for memory leaks using debug tools

### Slow Validation

1. Enable performance profiling
2. Check for expensive validation rules
3. Consider using FastMode for less critical paths
4. Enable caching for repeated validations

### Validation Failures

1. Enable debug mode to capture detailed information
2. Export debug sessions for analysis
3. Check event sequences and state transitions
4. Verify timing constraints are reasonable

## Contributing

Please see the main AG-UI SDK documentation for contribution guidelines.