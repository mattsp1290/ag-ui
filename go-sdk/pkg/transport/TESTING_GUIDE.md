# Transport Package Testing Guide

This guide provides comprehensive documentation for the test infrastructure utilities and mocks available in the transport package.

## Table of Contents

- [Overview](#overview)
- [Basic Mocks](#basic-mocks)
- [Advanced Mocks](#advanced-mocks)
- [Test Utilities](#test-utilities)
- [Test Scenarios](#test-scenarios)
- [Best Practices](#best-practices)

## Overview

The transport package provides a comprehensive test infrastructure designed to make testing transport implementations easier and more robust. The infrastructure includes:

- **Mock implementations** of Transport and Manager interfaces
- **Test utilities** for common testing patterns
- **Network simulation** capabilities
- **Chaos testing** support
- **Assertion helpers** for async operations
- **Pre-configured scenarios** for common test cases

## Basic Mocks

### MockTransport

The `MockTransport` is a highly configurable mock implementation of the Transport interface.

```go
// Create a mock transport
transport := NewMockTransport()

// Configure custom behavior
transport.SetConnectBehavior(func(ctx context.Context) error {
    // Custom connect logic
    return nil
})

transport.SetSendBehavior(func(ctx context.Context, event TransportEvent) error {
    // Custom send logic
    return nil
})

// Simulate events and errors
transport.SimulateEvent(event)
transport.SimulateError(err)

// Track calls
callCount := transport.GetCallCount("Send")
sentEvents := transport.GetSentEvents()
```

### MockManager

The `MockManager` provides a mock implementation of a transport manager.

```go
manager := NewMockManager()
manager.SetTransport(transport)

// Configure custom behavior
manager.startBehavior = func(ctx context.Context) error {
    return nil
}
```

### MockEventHandler

For testing event handling:

```go
handler := NewMockEventHandler()
handler.SetBehavior(func(ctx context.Context, event events.Event) error {
    // Custom handling logic
    return nil
})

// Get handled events
events := handler.GetHandledEvents()
```

## Advanced Mocks

### AdvancedMockTransport

Provides network simulation and state machine capabilities:

```go
transport := NewAdvancedMockTransport()

// Configure network conditions
transport.SetNetworkConditions(
    100*time.Millisecond,  // latency
    50*time.Millisecond,   // jitter
    0.05,                  // 5% packet loss
    1024*1024,            // 1MB/s bandwidth
)

// Track state changes
transport.stateCallbacks = append(transport.stateCallbacks, 
    func(state ConnectionState, err error) {
        log.Printf("State changed to: %s", state)
    })
```

### ScenarioTransport

Pre-configured transports for common scenarios:

```go
// Available scenarios:
// - "flaky-network": Simulates unreliable network
// - "slow-connection": Simulates slow network
// - "unreliable": High packet loss
// - "perfect": No delays or errors
// - "disconnecting": Disconnects after N operations
// - "reconnecting": Periodic reconnections

transport := NewScenarioTransport("flaky-network")
```

### ChaosTransport

For chaos testing with random failures:

```go
// 20% error rate
transport := NewChaosTransport(0.2)

// Customize chaos behavior
transport.delayRange = [2]time.Duration{0, 500*time.Millisecond}
transport.possibleErrors = []error{
    ErrConnectionClosed,
    ErrTimeout,
    errors.New("random error"),
}
```

### RecordingTransport

Records all operations for analysis:

```go
base := NewMockTransport()
recorder := NewRecordingTransport(base)

// Perform operations...

// Get recorded operations
ops := recorder.GetOperations()
for _, op := range ops {
    fmt.Printf("%s took %v\n", op.Type, op.Duration)
}
```

## Test Utilities

### Test Event Helpers

```go
// Create single test event
event := NewTestEvent("id-1", "test.event")

// Create test event with data
event := NewTestEventWithData("id-1", "test.event", map[string]interface{}{
    "key": "value",
})

// Generate multiple events
events := GenerateTestEvents(10, "prefix")

// Generate events with delay
events := GenerateTestEventsWithDelay(5, "prefix", 100*time.Millisecond)
```

### Assertion Helpers

For testing async operations:

```go
// Assert event is received
event := AssertEventReceived(t, eventChan, 100*time.Millisecond)

// Assert no event is received
AssertNoEvent(t, eventChan, 50*time.Millisecond)

// Assert error is received
err := AssertErrorReceived(t, errorChan, 100*time.Millisecond)

// Assert no error
AssertNoError(t, errorChan, 50*time.Millisecond)

// Assert transport state
AssertTransportConnected(t, transport)
AssertTransportNotConnected(t, transport)
```

### Timeout Helpers

```go
// Run function with timeout - fails test if timeout occurs
WithTimeout(t, 1*time.Second, func(ctx context.Context) {
    // Your test code here
})

// Run function with timeout - allows timeout as expected behavior
WithTimeoutExpected(t, 100*time.Millisecond, func(ctx context.Context) {
    // Test timeout behavior - check ctx.Err() for timeout
    err := transport.Connect(ctx)
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Errorf("Expected deadline exceeded, got %v", err)
    }
})

// Wait for condition
WaitForCondition(t, 500*time.Millisecond, func() bool {
    return transport.IsConnected()
})
```

### Test Fixtures

For common test setup:

```go
fixture := NewTestFixture(t)

// Automatically sets up:
// - MockTransport
// - MockManager
// - MockEventHandler
// - Context with cancel
// - Default configuration

// Helper methods
fixture.ConnectTransport(t)
fixture.StartManager(t)
fixture.SendEvent(t, event)
```

### Concurrent Testing

```go
ct := NewConcurrentTest()

// Run 100 concurrent operations
ct.Run(100, func(id int) error {
    event := NewTestEvent(fmt.Sprintf("concurrent-%d", id), "test")
    return transport.Send(ctx, event)
})

// Wait and check for errors
errors := ct.Wait()
```

### Error Simulation

```go
simulator := NewErrorSimulator()

// Always error
simulator.SetError("connect", errors.New("connection refused"))

// Error every 3rd call
simulator.SetError("send", errors.New("network error"))
simulator.SetErrorFrequency("send", 3)

// Check if should error
if err, shouldError := simulator.ShouldError("send"); shouldError {
    return err
}
```

## Test Scenarios

### Testing Connection Resilience

```go
func TestConnectionResilience(t *testing.T) {
    transport := NewScenarioTransport("reconnecting")
    
    // Monitor state changes
    stateChanges := 0
    transport.stateCallbacks = append(transport.stateCallbacks,
        func(state ConnectionState, err error) {
            stateChanges++
        })
    
    // Run for a period
    time.Sleep(5 * time.Second)
    
    // Verify reconnections occurred
    if stateChanges < 4 { // At least 2 reconnect cycles
        t.Error("Expected multiple reconnections")
    }
}
```

### Testing Backpressure

```go
func TestBackpressure(t *testing.T) {
    transport := NewAdvancedMockTransport()
    transport.SetNetworkConditions(
        0, 0, 0, 
        1024, // Very low bandwidth
    )
    
    // Send many events quickly
    start := time.Now()
    for i := 0; i < 100; i++ {
        event := NewTestEvent(fmt.Sprintf("bp-%d", i), "test")
        transport.Send(context.Background(), event)
    }
    duration := time.Since(start)
    
    // Should take significant time due to bandwidth limit
    if duration < 1*time.Second {
        t.Error("Backpressure not working")
    }
}
```

### Testing Error Recovery

```go
func TestErrorRecovery(t *testing.T) {
    transport := NewMockTransport()
    
    // Fail first 2 attempts, succeed on 3rd
    attempts := 0
    transport.SetConnectBehavior(func(ctx context.Context) error {
        attempts++
        if attempts < 3 {
            return errors.New("connection failed")
        }
        return nil
    })
    
    // Implement retry logic
    var err error
    for i := 0; i < 3; i++ {
        err = transport.Connect(context.Background())
        if err == nil {
            break
        }
        time.Sleep(100 * time.Millisecond)
    }
    
    if err != nil {
        t.Fatal("Failed to recover from errors")
    }
}
```

## Best Practices

### 1. Use Test Fixtures

Always use test fixtures for common setup:

```go
func TestMyFeature(t *testing.T) {
    fixture := NewTestFixture(t)
    // Automatic cleanup on test completion
}
```

### 2. Test Async Operations Properly

Use assertion helpers for async operations:

```go
// Good
event := AssertEventReceived(t, eventChan, 100*time.Millisecond)

// Bad - can cause flaky tests
select {
case event := <-eventChan:
    // ...
case <-time.After(100 * time.Millisecond):
    t.Fatal("timeout")
}
```

### 3. Simulate Real Conditions

Use network simulation for realistic testing:

```go
transport := NewAdvancedMockTransport()
transport.SetNetworkConditions(
    50*time.Millisecond,   // Typical internet latency
    10*time.Millisecond,   // Some jitter
    0.01,                  // 1% packet loss
    10*1024*1024,         // 10MB/s
)
```

### 4. Test Concurrency

Always test concurrent access:

```go
ct := NewConcurrentTest()
ct.Run(100, func(id int) error {
    return transport.Send(ctx, event)
})
errors := ct.Wait()
```

### 5. Use Chaos Testing

Include chaos testing for production readiness:

```go
func TestChaosScenarios(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping chaos tests in short mode")
    }
    
    transport := NewChaosTransport(0.1) // 10% error rate
    // Run normal operations and verify system handles errors
}
```

### 6. Record Operations for Debugging

Use recording transport when debugging issues:

```go
recorder := NewRecordingTransport(transport)
// Run failing test...

ops := recorder.GetOperations()
for _, op := range ops {
    t.Logf("%s at %v took %v, error: %v", 
        op.Type, op.Timestamp, op.Duration, op.Error)
}
```

### 7. Benchmark Performance

Include benchmarks for performance-critical paths:

```go
func BenchmarkTransportSend(b *testing.B) {
    transport := NewMockTransport()
    BenchmarkTransport(b, transport)
}

func BenchmarkConcurrentSend(b *testing.B) {
    transport := NewMockTransport()
    BenchmarkConcurrentSend(b, transport, 10) // 10 goroutines
}
```

## Examples

See `testing_demo_test.go` for comprehensive examples of all test utilities in action.