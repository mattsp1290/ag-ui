# Comprehensive Testing Guide

This guide provides comprehensive testing patterns and best practices for the AG-UI Go SDK enhanced event validation system.

## Table of Contents

1. [Testing Overview](#testing-overview)
2. [Unit Testing Patterns](#unit-testing-patterns)
3. [Integration Testing](#integration-testing)
4. [Performance Testing](#performance-testing)
5. [Mock and Stub Usage](#mock-and-stub-usage)
6. [Testing Enhanced Validation Features](#testing-enhanced-validation-features)
7. [Distributed Testing Scenarios](#distributed-testing-scenarios)
8. [Common Testing Utilities](#common-testing-utilities)
9. [Test Organization](#test-organization)
10. [Best Practices](#best-practices)

## Testing Overview

The AG-UI Go SDK uses a comprehensive testing strategy that includes:

- **Unit Tests**: Test individual components in isolation
- **Integration Tests**: Test component interactions
- **Performance Tests**: Benchmark performance characteristics
- **Property-Based Tests**: Test invariants using rapid testing
- **Concurrent Tests**: Test thread safety and race conditions
- **Memory Tests**: Test for memory leaks and resource management

### Test Categories

| Category | Purpose | Files | Command |
|----------|---------|-------|---------|
| Unit | Component isolation | `*_test.go` | `go test ./pkg/...` |
| Integration | Component interaction | `*_integration_test.go` | `go test -tags=integration ./pkg/...` |
| Performance | Benchmarking | `*_benchmark_test.go` | `go test -bench=. ./pkg/...` |
| Memory | Leak detection | `*_memory_test.go` | `go test -race -count=10 ./pkg/...` |
| Property | Invariant testing | `*_property_test.go` | `go test ./pkg/... -timeout=30s` |

## Unit Testing Patterns

### Basic Event Validation Testing

```go
func TestEventValidator_ValidateEvent(t *testing.T) {
    tests := []struct {
        name          string
        event         Event
        expectedValid bool
        expectedError string
        setup         func(*EventValidator)
    }{
        {
            name:          "nil event",
            event:         nil,
            expectedValid: false,
            expectedError: "Event cannot be nil",
        },
        {
            name: "valid run started event",
            event: &RunStartedEvent{
                BaseEvent: &BaseEvent{
                    EventType:   EventTypeRunStarted,
                    TimestampMs: timePtr(time.Now().UnixMilli()),
                },
                RunIDValue:    "run-123",
                ThreadIDValue: "thread-456",
            },
            expectedValid: true,
        },
        {
            name: "with custom validation rules",
            event: &RunStartedEvent{
                BaseEvent: &BaseEvent{
                    EventType:   EventTypeRunStarted,
                    TimestampMs: timePtr(time.Now().UnixMilli()),
                },
                RunIDValue:    "run-123",
                ThreadIDValue: "thread-456",
            },
            expectedValid: true,
            setup: func(v *EventValidator) {
                v.AddValidationRule(&CustomValidationRule{
                    ID: "custom-rule",
                    Validate: func(ctx context.Context, event Event) error {
                        // Custom validation logic
                        return nil
                    },
                })
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            validator := NewEventValidator(DefaultValidationConfig())
            
            if tt.setup != nil {
                tt.setup(validator)
            }
            
            result := validator.ValidateEvent(context.Background(), tt.event)

            assert.Equal(t, tt.expectedValid, result.IsValid)
            
            if !tt.expectedValid && tt.expectedError != "" {
                assert.Contains(t, getErrorMessages(result.Errors), tt.expectedError)
            }
        })
    }
}
```

### Testing with Context and Timeouts

```go
func TestEventValidator_WithTimeout(t *testing.T) {
    validator := NewEventValidator(DefaultValidationConfig())
    
    // Test with timeout context
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()
    
    event := &RunStartedEvent{
        BaseEvent: &BaseEvent{
            EventType:   EventTypeRunStarted,
            TimestampMs: timePtr(time.Now().UnixMilli()),
        },
        RunIDValue:    "run-123",
        ThreadIDValue: "thread-456",
    }
    
    result := validator.ValidateEvent(ctx, event)
    assert.True(t, result.IsValid)
    
    // Test context cancellation
    ctx, cancel = context.WithCancel(context.Background())
    cancel() // Cancel immediately
    
    result = validator.ValidateEvent(ctx, event)
    // Should handle cancellation gracefully
}
```

### Enhanced Validator Testing

```go
func TestEnhancedValidator_AdvancedFeatures(t *testing.T) {
    config := EnhancedValidationConfig()
    config.EnableRuleChaining = true
    config.EnableAsyncValidation = true
    config.EnableMetricsCollection = true
    
    validator := NewEnhancedEventValidator(config)
    
    // Test rule chaining
    t.Run("rule chaining", func(t *testing.T) {
        validator.AddValidationRule(&TimingConstraintRule{})
        validator.AddValidationRule(&ContentValidationRule{})
        
        event := createTestEvent()
        result := validator.ValidateEvent(context.Background(), event)
        
        assert.True(t, result.IsValid)
        assert.True(t, len(result.AppliedRules) >= 2)
    })
    
    // Test async validation
    t.Run("async validation", func(t *testing.T) {
        events := make([]Event, 100)
        for i := range events {
            events[i] = createTestEvent()
        }
        
        results := validator.ValidateSequenceAsync(context.Background(), events)
        
        for result := range results {
            assert.True(t, result.IsValid)
        }
    })
    
    // Test metrics collection
    t.Run("metrics collection", func(t *testing.T) {
        event := createTestEvent()
        validator.ValidateEvent(context.Background(), event)
        
        metrics := validator.GetMetrics()
        assert.Greater(t, metrics.ValidationCount, int64(0))
        assert.GreaterOrEqual(t, metrics.SuccessRate, 0.0)
    })
}
```

## Integration Testing

### Component Integration Testing

```go
func TestAuthenticatedValidator_Integration(t *testing.T) {
    // Setup auth provider
    authProvider := auth.NewBasicAuthProvider(nil)
    authProvider.AddUser(&auth.User{
        Username:    "validator",
        Roles:       []string{"validator"},
        Permissions: []string{"event:validate"},
        Active:      true,
    })
    authProvider.SetUserPassword("validator", "secret")
    
    // Setup cache
    cache := cache.NewCacheValidator(cache.DefaultCacheValidatorConfig())
    defer cache.Shutdown(context.Background())
    
    // Setup validator with auth and cache
    validator := auth.NewAuthenticatedValidator(
        events.DefaultValidationConfig(),
        authProvider,
        auth.DefaultAuthConfig(),
    )
    
    event := &events.RunStartedEvent{
        BaseEvent: &events.BaseEvent{
            EventType:   events.EventTypeRunStarted,
            TimestampMs: timePtr(time.Now().UnixMilli()),
        },
        RunIDValue:    "run-123",
        ThreadIDValue: "thread-456",
    }
    
    // Test authenticated validation
    result := validator.ValidateWithBasicAuth(
        context.Background(), 
        event, 
        "validator", 
        "secret",
    )
    
    assert.True(t, result.IsValid)
    assert.NotNil(t, result.AuthContext)
    
    // Test caching
    start := time.Now()
    result1 := validator.ValidateWithBasicAuth(context.Background(), event, "validator", "secret")
    firstDuration := time.Since(start)
    
    start = time.Now()
    result2 := validator.ValidateWithBasicAuth(context.Background(), event, "validator", "secret")
    secondDuration := time.Since(start)
    
    assert.True(t, result1.IsValid)
    assert.True(t, result2.IsValid)
    assert.Less(t, secondDuration, firstDuration) // Cache hit should be faster
}
```

### Distributed Integration Testing

```go
func TestDistributedValidator_Integration(t *testing.T) {
    // Setup multiple nodes
    nodes := make([]*distributed.DistributedValidator, 3)
    
    for i := 0; i < 3; i++ {
        config := distributed.DefaultDistributedValidatorConfig(fmt.Sprintf("node-%d", i))
        config.ConsensusConfig.MinNodes = 3
        config.ConsensusConfig.QuorumSize = 2
        
        localValidator := events.NewEventValidator(events.DefaultValidationConfig())
        dv, err := distributed.NewDistributedValidator(config, localValidator)
        require.NoError(t, err)
        
        require.NoError(t, dv.Start(context.Background()))
        nodes[i] = dv
        
        defer dv.Stop()
    }
    
    // Register nodes with each other
    for i, node := range nodes {
        for j, other := range nodes {
            if i != j {
                otherInfo := &distributed.NodeInfo{
                    ID:      fmt.Sprintf("node-%d", j),
                    Address: fmt.Sprintf("localhost:%d", 8080+j),
                    State:   distributed.NodeStateActive,
                }
                node.RegisterNode(otherInfo)
            }
        }
    }
    
    // Wait for consensus
    time.Sleep(2 * time.Second)
    
    // Test distributed validation
    event := &events.RunStartedEvent{
        BaseEvent: &events.BaseEvent{
            EventType:   events.EventTypeRunStarted,
            TimestampMs: timePtr(time.Now().UnixMilli()),
        },
        RunIDValue:    "run-123",
        ThreadIDValue: "thread-456",
    }
    
    // Validate on first node
    result := nodes[0].ValidateEvent(context.Background(), event)
    assert.True(t, result.IsValid)
    assert.Equal(t, distributed.ConsensusMajority, result.ConsensusType)
    assert.GreaterOrEqual(t, result.NodeResponses, 2)
}
```

## Performance Testing

### Benchmark Testing Patterns

```go
func BenchmarkEventValidator_ValidateEvent(b *testing.B) {
    validator := NewEventValidator(DefaultValidationConfig())
    event := &RunStartedEvent{
        BaseEvent: &BaseEvent{
            EventType:   EventTypeRunStarted,
            TimestampMs: timePtr(time.Now().UnixMilli()),
        },
        RunIDValue:    "run-123",
        ThreadIDValue: "thread-456",
    }
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        result := validator.ValidateEvent(context.Background(), event)
        if !result.IsValid {
            b.Fatal("Validation failed")
        }
    }
}

func BenchmarkEventValidator_ConcurrentValidation(b *testing.B) {
    validator := NewEventValidator(DefaultValidationConfig())
    event := &RunStartedEvent{
        BaseEvent: &BaseEvent{
            EventType:   EventTypeRunStarted,
            TimestampMs: timePtr(time.Now().UnixMilli()),
        },
        RunIDValue:    "run-123",
        ThreadIDValue: "thread-456",
    }
    
    b.ResetTimer()
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            result := validator.ValidateEvent(context.Background(), event)
            if !result.IsValid {
                b.Fatal("Validation failed")
            }
        }
    })
}
```

### Load Testing

```go
func TestEventValidator_LoadTest(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping load test in short mode")
    }
    
    validator := NewEventValidator(DefaultValidationConfig())
    
    const (
        numGoroutines = 100
        eventsPerGoroutine = 1000
        totalEvents = numGoroutines * eventsPerGoroutines
    )
    
    var wg sync.WaitGroup
    var successCount int64
    var errorCount int64
    
    start := time.Now()
    
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(routineID int) {
            defer wg.Done()
            
            for j := 0; j < eventsPerGoroutine; j++ {
                event := &RunStartedEvent{
                    BaseEvent: &BaseEvent{
                        EventType:   EventTypeRunStarted,
                        TimestampMs: timePtr(time.Now().UnixMilli()),
                    },
                    RunIDValue:    fmt.Sprintf("run-%d-%d", routineID, j),
                    ThreadIDValue: fmt.Sprintf("thread-%d", routineID),
                }
                
                result := validator.ValidateEvent(context.Background(), event)
                if result.IsValid {
                    atomic.AddInt64(&successCount, 1)
                } else {
                    atomic.AddInt64(&errorCount, 1)
                }
            }
        }(i)
    }
    
    wg.Wait()
    duration := time.Since(start)
    
    successRate := float64(successCount) / float64(totalEvents) * 100
    throughput := float64(totalEvents) / duration.Seconds()
    
    t.Logf("Load test results:")
    t.Logf("  Total events: %d", totalEvents)
    t.Logf("  Success rate: %.2f%%", successRate)
    t.Logf("  Throughput: %.0f events/second", throughput)
    t.Logf("  Duration: %v", duration)
    
    assert.Greater(t, successRate, 95.0, "Success rate should be > 95%")
    assert.Greater(t, throughput, 10000.0, "Throughput should be > 10,000 events/second")
}
```

## Mock and Stub Usage

### Mock Authentication Provider

```go
type MockAuthProvider struct {
    users map[string]*auth.User
    calls map[string]int
}

func NewMockAuthProvider() *MockAuthProvider {
    return &MockAuthProvider{
        users: make(map[string]*auth.User),
        calls: make(map[string]int),
    }
}

func (m *MockAuthProvider) Authenticate(ctx context.Context, credentials auth.Credentials) (*auth.AuthContext, error) {
    m.calls["Authenticate"]++
    
    if basicCreds, ok := credentials.(*auth.BasicCredentials); ok {
        if user, exists := m.users[basicCreds.Username]; exists {
            return &auth.AuthContext{
                UserID:      user.ID,
                Username:    user.Username,
                Roles:       user.Roles,
                Permissions: user.Permissions,
            }, nil
        }
    }
    
    return nil, auth.ErrInvalidCredentials
}

func (m *MockAuthProvider) AddUser(username string, user *auth.User) {
    m.users[username] = user
}

func (m *MockAuthProvider) GetCallCount(method string) int {
    return m.calls[method]
}

// Usage in tests
func TestWithMockAuth(t *testing.T) {
    mockAuth := NewMockAuthProvider()
    mockAuth.AddUser("testuser", &auth.User{
        ID:          "user-1",
        Username:    "testuser",
        Roles:       []string{"validator"},
        Permissions: []string{"event:validate"},
    })
    
    validator := auth.NewAuthenticatedValidator(
        events.DefaultValidationConfig(),
        mockAuth,
        auth.DefaultAuthConfig(),
    )
    
    event := createTestEvent()
    result := validator.ValidateWithBasicAuth(
        context.Background(),
        event,
        "testuser",
        "password",
    )
    
    assert.True(t, result.IsValid)
    assert.Equal(t, 1, mockAuth.GetCallCount("Authenticate"))
}
```

### Stub Transport for Distributed Testing

```go
type StubTransport struct {
    messages []distributed.Message
    delay    time.Duration
    failRate float64
}

func NewStubTransport() *StubTransport {
    return &StubTransport{
        messages: make([]distributed.Message, 0),
        delay:    10 * time.Millisecond,
        failRate: 0.0,
    }
}

func (s *StubTransport) Send(ctx context.Context, nodeID string, message distributed.Message) error {
    // Simulate network delay
    time.Sleep(s.delay)
    
    // Simulate failures
    if rand.Float64() < s.failRate {
        return errors.New("simulated network failure")
    }
    
    s.messages = append(s.messages, message)
    return nil
}

func (s *StubTransport) SetDelay(delay time.Duration) {
    s.delay = delay
}

func (s *StubTransport) SetFailureRate(rate float64) {
    s.failRate = rate
}

func (s *StubTransport) GetMessages() []distributed.Message {
    return s.messages
}

// Usage in distributed tests
func TestDistributedWithStubTransport(t *testing.T) {
    transport := NewStubTransport()
    transport.SetDelay(5 * time.Millisecond)
    transport.SetFailureRate(0.1) // 10% failure rate
    
    config := distributed.DefaultDistributedValidatorConfig("test-node")
    config.Transport = transport
    
    validator, err := distributed.NewDistributedValidator(config, nil)
    require.NoError(t, err)
    
    // Test with simulated network conditions
    event := createTestEvent()
    result := validator.ValidateEvent(context.Background(), event)
    
    // Verify behavior under network stress
    messages := transport.GetMessages()
    assert.Greater(t, len(messages), 0)
}
```

## Testing Enhanced Validation Features

### Rule Chaining Tests

```go
func TestRuleChaining(t *testing.T) {
    config := EnhancedValidationConfig()
    config.EnableRuleChaining = true
    
    validator := NewEnhancedEventValidator(config)
    
    // Add multiple rules
    rule1 := &TimingConstraintRule{ID: "timing-1"}
    rule2 := &ContentValidationRule{ID: "content-1"}
    rule3 := &SequenceValidationRule{ID: "sequence-1"}
    
    validator.AddValidationRule(rule1)
    validator.AddValidationRule(rule2)
    validator.AddValidationRule(rule3)
    
    event := &TextMessageStartEvent{
        BaseEvent: &BaseEvent{
            EventType:   EventTypeTextMessageStart,
            TimestampMs: timePtr(time.Now().UnixMilli()),
        },
        MessageID: "msg-123",
    }
    
    result := validator.ValidateEvent(context.Background(), event)
    
    // Verify all rules were applied
    assert.Equal(t, 3, len(result.AppliedRules))
    assert.Contains(t, result.AppliedRules, "timing-1")
    assert.Contains(t, result.AppliedRules, "content-1")
    assert.Contains(t, result.AppliedRules, "sequence-1")
}
```

### Async Validation Tests

```go
func TestAsyncValidation(t *testing.T) {
    config := EnhancedValidationConfig()
    config.EnableAsyncValidation = true
    config.AsyncWorkerCount = 5
    
    validator := NewEnhancedEventValidator(config)
    
    events := make([]Event, 100)
    for i := range events {
        events[i] = &RunStartedEvent{
            BaseEvent: &BaseEvent{
                EventType:   EventTypeRunStarted,
                TimestampMs: timePtr(time.Now().UnixMilli()),
            },
            RunIDValue:    fmt.Sprintf("run-%d", i),
            ThreadIDValue: "thread-1",
        }
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    results := validator.ValidateSequenceAsync(ctx, events)
    
    validCount := 0
    for result := range results {
        if result.IsValid {
            validCount++
        }
    }
    
    assert.Equal(t, len(events), validCount)
}
```

## Distributed Testing Scenarios

### Partition Tolerance Testing

```go
func TestNetworkPartitionTolerance(t *testing.T) {
    // Create 5-node cluster
    nodes := createTestCluster(5)
    defer cleanupCluster(nodes)
    
    // Wait for cluster formation
    waitForConsensus(nodes)
    
    // Simulate network partition (split into 3-2)
    partition1 := nodes[:3]  // Majority partition
    partition2 := nodes[3:]  // Minority partition
    
    // Block communication between partitions
    blockCommunication(partition1, partition2)
    
    event := createTestEvent()
    
    // Test validation in majority partition
    result1 := partition1[0].ValidateEvent(context.Background(), event)
    assert.True(t, result1.IsValid, "Majority partition should continue validating")
    
    // Test validation in minority partition
    result2 := partition2[0].ValidateEvent(context.Background(), event)
    // Behavior depends on configuration - might fail or use local validation
    
    // Heal partition
    allowCommunication(partition1, partition2)
    
    // Wait for reconciliation
    time.Sleep(5 * time.Second)
    
    // Verify all nodes are synchronized
    for _, node := range nodes {
        assert.True(t, node.IsHealthy())
    }
}
```

### Byzantine Fault Tolerance Testing

```go
func TestByzantineFaultTolerance(t *testing.T) {
    // Create 4-node cluster (can tolerate 1 Byzantine failure)
    nodes := createTestCluster(4)
    defer cleanupCluster(nodes)
    
    // Configure PBFT consensus
    for _, node := range nodes {
        node.SetConsensusAlgorithm(distributed.ConsensusPBFT)
    }
    
    waitForConsensus(nodes)
    
    // Make one node Byzantine (always returns invalid)
    byzantineNode := nodes[0]
    byzantineNode.SetByzantineMode(true)
    
    event := createTestEvent()
    
    // Test validation - should succeed despite Byzantine node
    for i := 1; i < len(nodes); i++ {
        result := nodes[i].ValidateEvent(context.Background(), event)
        assert.True(t, result.IsValid, "Validation should succeed despite Byzantine node")
    }
}
```

## Common Testing Utilities

### Test Helper Functions

```go
// testhelper/events.go
package testhelper

import (
    "time"
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func CreateTestEvent(eventType events.EventType) events.Event {
    switch eventType {
    case events.EventTypeRunStarted:
        return &events.RunStartedEvent{
            BaseEvent: &events.BaseEvent{
                EventType:   eventType,
                TimestampMs: timePtr(time.Now().UnixMilli()),
            },
            RunIDValue:    "test-run-123",
            ThreadIDValue: "test-thread-456",
        }
    case events.EventTypeTextMessageStart:
        return &events.TextMessageStartEvent{
            BaseEvent: &events.BaseEvent{
                EventType:   eventType,
                TimestampMs: timePtr(time.Now().UnixMilli()),
            },
            MessageID: "test-msg-123",
        }
    // Add more event types as needed
    default:
        return nil
    }
}

func CreateEventSequence() []events.Event {
    return []events.Event{
        CreateTestEvent(events.EventTypeRunStarted),
        CreateTestEvent(events.EventTypeTextMessageStart),
        &events.TextMessageContentEvent{
            BaseEvent: &events.BaseEvent{
                EventType:   events.EventTypeTextMessageContent,
                TimestampMs: timePtr(time.Now().UnixMilli()),
            },
            MessageID: "test-msg-123",
            Delta:     "Hello, world!",
        },
        &events.TextMessageEndEvent{
            BaseEvent: &events.BaseEvent{
                EventType:   events.EventTypeTextMessageEnd,
                TimestampMs: timePtr(time.Now().UnixMilli()),
            },
            MessageID: "test-msg-123",
        },
        &events.RunFinishedEvent{
            BaseEvent: &events.BaseEvent{
                EventType:   events.EventTypeRunFinished,
                TimestampMs: timePtr(time.Now().UnixMilli()),
            },
            RunIDValue: "test-run-123",
        },
    }
}

func timePtr(t int64) *int64 {
    return &t
}
```

### Assertion Helpers

```go
// testhelper/assertions.go
package testhelper

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func AssertValidationResult(t *testing.T, result *events.ValidationResult, expectedValid bool) {
    t.Helper()
    
    assert.Equal(t, expectedValid, result.IsValid)
    
    if !expectedValid {
        assert.NotEmpty(t, result.Errors, "Invalid result should have errors")
    }
    
    assert.NotNil(t, result.Timestamp)
    assert.GreaterOrEqual(t, result.ProcessingTime, int64(0))
}

func AssertErrorContains(t *testing.T, errors []events.ValidationError, expectedMessage string) {
    t.Helper()
    
    found := false
    for _, err := range errors {
        if assert.Contains(t, err.Message, expectedMessage) {
            found = true
            break
        }
    }
    
    assert.True(t, found, "Expected error message not found: %s", expectedMessage)
}

func AssertMetrics(t *testing.T, metrics *events.Metrics) {
    t.Helper()
    
    assert.GreaterOrEqual(t, metrics.ValidationCount, int64(0))
    assert.GreaterOrEqual(t, metrics.SuccessCount, int64(0))
    assert.GreaterOrEqual(t, metrics.ErrorCount, int64(0))
    assert.GreaterOrEqual(t, metrics.SuccessRate, 0.0)
    assert.LessOrEqual(t, metrics.SuccessRate, 1.0)
}
```

## Test Organization

### Directory Structure

```
pkg/
├── core/
│   └── events/
│       ├── events_test.go              # Basic event tests
│       ├── validator_test.go           # Validator unit tests
│       ├── validator_integration_test.go # Integration tests
│       ├── validator_benchmark_test.go # Performance tests
│       ├── validator_memory_test.go    # Memory leak tests
│       ├── validator_concurrency_test.go # Race condition tests
│       └── auth/
│           ├── auth_test.go           # Auth unit tests
│           ├── example_test.go        # Example tests
│           └── integration_test.go    # Auth integration tests
├── testhelper/                        # Common test utilities
│   ├── events.go                     # Event creation helpers
│   ├── assertions.go                 # Custom assertions
│   ├── mocks.go                      # Mock implementations
│   └── fixtures.go                   # Test data fixtures
└── integration/                       # System integration tests
    ├── auth_integration_test.go
    ├── cache_integration_test.go
    └── distributed_integration_test.go
```

### Test Tags and Categories

Use build tags to categorize tests:

```go
//go:build integration
// +build integration

package events_test

// Integration tests that require external dependencies
```

```go
//go:build performance
// +build performance

package events_test

// Performance tests that take longer to run
```

### Running Different Test Categories

```bash
# Run all tests
go test ./pkg/...

# Run only unit tests (default)
go test -tags=unit ./pkg/...

# Run integration tests
go test -tags=integration ./pkg/...

# Run performance tests
go test -tags=performance -bench=. ./pkg/...

# Run with race detection
go test -race ./pkg/...

# Run with coverage
go test -cover ./pkg/...

# Run specific package
go test ./pkg/core/events/...

# Run with verbose output
go test -v ./pkg/...

# Run tests multiple times to catch flaky tests
go test -count=10 ./pkg/...
```

## Best Practices

### 1. Test Naming

```go
// Good: Descriptive test names
func TestEventValidator_ValidateEvent_WithNilEvent_ShouldReturnError(t *testing.T)
func TestCacheValidator_ValidateEvent_WithCacheHit_ShouldReturnFaster(t *testing.T)
func TestDistributedValidator_ValidateEvent_WithPartition_ShouldUseMajority(t *testing.T)

// Avoid: Generic test names
func TestValidation(t *testing.T)
func TestCache(t *testing.T)
```

### 2. Test Structure

Follow the Arrange-Act-Assert pattern:

```go
func TestEventValidator_ValidateEvent(t *testing.T) {
    // Arrange
    validator := NewEventValidator(DefaultValidationConfig())
    event := &RunStartedEvent{
        // ... event setup
    }
    
    // Act
    result := validator.ValidateEvent(context.Background(), event)
    
    // Assert
    assert.True(t, result.IsValid)
    assert.Empty(t, result.Errors)
}
```

### 3. Use Table-Driven Tests

```go
func TestEventValidator_ValidateEvent(t *testing.T) {
    tests := []struct {
        name          string
        event         Event
        expectedValid bool
        expectedError string
    }{
        // Test cases here
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### 4. Clean Up Resources

```go
func TestWithCleanup(t *testing.T) {
    validator := NewEventValidator(DefaultValidationConfig())
    defer validator.Shutdown() // Always clean up
    
    // If using t.Cleanup
    t.Cleanup(func() {
        validator.Shutdown()
    })
}
```

### 5. Use Context for Timeouts

```go
func TestWithTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    result := validator.ValidateEvent(ctx, event)
    // Test should complete within timeout
}
```

### 6. Test Error Conditions

```go
func TestErrorConditions(t *testing.T) {
    // Test nil inputs
    result := validator.ValidateEvent(context.Background(), nil)
    assert.False(t, result.IsValid)
    
    // Test invalid contexts
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel before use
    
    result = validator.ValidateEvent(ctx, event)
    // Handle cancellation appropriately
}
```

### 7. Use Mocks Sparingly

- Use mocks for external dependencies
- Prefer real implementations when possible
- Keep mocks simple and focused

### 8. Test Concurrency

```go
func TestConcurrency(t *testing.T) {
    validator := NewEventValidator(DefaultValidationConfig())
    
    var wg sync.WaitGroup
    numGoroutines := 100
    
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            
            event := createTestEvent()
            result := validator.ValidateEvent(context.Background(), event)
            assert.True(t, result.IsValid)
        }()
    }
    
    wg.Wait()
}
```

### 9. Test Memory Management

```go
func TestMemoryLeaks(t *testing.T) {
    validator := NewEventValidator(DefaultValidationConfig())
    
    // Get initial memory stats
    var m1, m2 runtime.MemStats
    runtime.GC()
    runtime.ReadMemStats(&m1)
    
    // Perform operations
    for i := 0; i < 10000; i++ {
        event := createTestEvent()
        validator.ValidateEvent(context.Background(), event)
    }
    
    // Force garbage collection and check memory
    runtime.GC()
    runtime.ReadMemStats(&m2)
    
    // Memory should not grow significantly
    growth := m2.Alloc - m1.Alloc
    assert.Less(t, growth, uint64(10*1024*1024), "Memory growth should be < 10MB")
}
```

### 10. Performance Baseline Testing

```go
func TestPerformanceBaseline(t *testing.T) {
    validator := NewEventValidator(DefaultValidationConfig())
    event := createTestEvent()
    
    // Warm up
    for i := 0; i < 100; i++ {
        validator.ValidateEvent(context.Background(), event)
    }
    
    // Measure performance
    start := time.Now()
    iterations := 10000
    
    for i := 0; i < iterations; i++ {
        result := validator.ValidateEvent(context.Background(), event)
        assert.True(t, result.IsValid)
    }
    
    duration := time.Since(start)
    avgLatency := duration / time.Duration(iterations)
    
    // Performance assertions
    assert.Less(t, avgLatency, 100*time.Microsecond, "Average latency should be < 100μs")
    
    throughput := float64(iterations) / duration.Seconds()
    assert.Greater(t, throughput, 50000.0, "Throughput should be > 50K events/second")
}
```

This comprehensive testing guide provides patterns and practices for thoroughly testing the AG-UI Go SDK enhanced event validation system across all levels from unit tests to distributed integration scenarios.