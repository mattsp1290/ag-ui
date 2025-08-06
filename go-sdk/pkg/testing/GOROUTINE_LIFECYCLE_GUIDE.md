# Goroutine Lifecycle Management Guide

This guide provides comprehensive best practices for managing goroutine lifecycles to prevent leaks and ensure proper cleanup in the AG UI Go SDK.

## Table of Contents

1. [Overview](#overview)
2. [Common Goroutine Leak Patterns](#common-goroutine-leak-patterns)
3. [Best Practices](#best-practices)
4. [Utilities Provided](#utilities-provided)
5. [Testing for Leaks](#testing-for-leaks)
6. [Implementation Examples](#implementation-examples)
7. [Migration Guide](#migration-guide)

## Overview

Goroutine leaks are one of the most common causes of memory leaks and resource exhaustion in Go applications. This guide and the accompanying utilities help prevent leaks by providing:

- **Comprehensive leak detection** with detailed stack traces
- **Lifecycle management utilities** for proper goroutine coordination
- **Testing utilities** that automatically detect leaks in tests
- **Best practice patterns** for common goroutine use cases

## Common Goroutine Leak Patterns

### 1. Infinite Loops Without Exit Conditions

```go
// PROBLEMATIC: No way to stop this goroutine
go func() {
    for {
        // Do work forever
        processWork()
        time.Sleep(time.Second)
    }
}()
```

**Fix:**
```go
// GOOD: Respects context cancellation
go func() {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            processWork()
            time.Sleep(time.Second)
        }
    }
}()
```

### 2. Blocked Channel Operations

```go
// PROBLEMATIC: Goroutine blocks forever if channel is never read
go func() {
    for data := range inputCh {
        resultCh <- process(data) // Blocks if resultCh is full
    }
}()
```

**Fix:**
```go
// GOOD: Non-blocking with context cancellation
go func() {
    for {
        select {
        case <-ctx.Done():
            return
        case data, ok := <-inputCh:
            if !ok {
                return // Channel closed
            }
            select {
            case resultCh <- process(data):
                // Sent successfully
            case <-ctx.Done():
                return
            }
        }
    }
}()
```

### 3. Missing WaitGroup Management

```go
// PROBLEMATIC: No way to wait for completion
for i := 0; i < numWorkers; i++ {
    go worker(i)
}
// Program may exit before workers finish
```

**Fix:**
```go
// GOOD: Proper WaitGroup usage
var wg sync.WaitGroup
for i := 0; i < numWorkers; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        worker(id)
    }(i)
}
wg.Wait() // Wait for all workers to complete
```

### 4. Ticker/Timer Leaks

```go
// PROBLEMATIC: Ticker not stopped, goroutine leaks
ticker := time.NewTicker(time.Second)
go func() {
    for range ticker.C {
        doPeriodicWork()
    }
}()
```

**Fix:**
```go
// GOOD: Proper ticker cleanup
ticker := time.NewTicker(time.Second)
go func() {
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            doPeriodicWork()
        }
    }
}()
```

## Best Practices

### 1. Always Use Context for Cancellation

Every long-running goroutine should accept a context and respect its cancellation:

```go
func startWorker(ctx context.Context) {
    go func() {
        for {
            select {
            case <-ctx.Done():
                return // Graceful shutdown
            default:
                // Do work
            }
        }
    }()
}
```

### 2. Use the Goroutine Lifecycle Manager

For complex goroutine management, use our `GoroutineLifecycleManager`:

```go
manager := testing.NewGoroutineLifecycleManager("my-service")
defer manager.MustShutdown()

// Start managed goroutines
manager.Go("worker-1", func(ctx context.Context) {
    // Worker logic with automatic cleanup
})

manager.GoTicker("metrics", 30*time.Second, func(ctx context.Context) {
    // Periodic metrics collection
})
```

### 3. Implement Proper Shutdown Sequences

Always implement graceful shutdown with timeouts:

```go
type Service struct {
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

func (s *Service) Start() {
    s.ctx, s.cancel = context.WithCancel(context.Background())
    
    s.wg.Add(1)
    go s.worker()
}

func (s *Service) Stop(timeout time.Duration) error {
    s.cancel() // Signal shutdown
    
    done := make(chan struct{})
    go func() {
        s.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return nil
    case <-time.After(timeout):
        return fmt.Errorf("shutdown timeout")
    }
}

func (s *Service) worker() {
    defer s.wg.Done()
    
    for {
        select {
        case <-s.ctx.Done():
            return
        default:
            // Work
        }
    }
}
```

### 4. Use Defer for Cleanup

Always use `defer` statements for cleanup to handle panics:

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Worker panicked: %v", r)
        }
    }()
    defer wg.Done()
    defer cleanup()
    
    // Worker logic
}()
```

## Utilities Provided

### EnhancedGoroutineLeakDetector

Comprehensive leak detection with detailed analysis:

```go
func TestMyService(t *testing.T) {
    detector := testing.NewEnhancedGoroutineLeakDetector(t).
        WithTolerance(2).
        WithMaxWaitTime(5*time.Second).
        WithExcludePatterns("expected-background-worker")
    defer detector.Check()
    
    // Test code that might leak goroutines
}
```

Features:
- **Stack trace analysis** for identifying leak sources
- **Configurable tolerance** for expected background goroutines
- **Pattern matching** to exclude known safe goroutines
- **Timeout handling** with graceful degradation
- **Detailed reporting** with debugging hints

### GoroutineLifecycleManager

Structured goroutine management:

```go
manager := testing.NewGoroutineLifecycleManager("service-name")
defer manager.MustShutdown()

// Various goroutine patterns
manager.Go("worker", workerFunc)
manager.GoTicker("metrics", interval, metricsFunc)
manager.GoWorker("processor", workChannel, processFunc)
manager.GoBatch("batcher", batchSize, timeout, inputCh, batchFunc)
```

Features:
- **Automatic cleanup** with context cancellation
- **Panic recovery** with logging
- **Active tracking** of running goroutines
- **Graceful shutdown** with timeout
- **Common patterns** (workers, tickers, batching)

### SafeGoroutineManager

Enhanced safety with limits and monitoring:

```go
manager := testing.NewSafeGoroutineManager("service", maxGoroutines).
    WithPanicHandler(panicHandler)
defer manager.MustShutdown()

// Goroutine creation with safety checks
err := manager.Go("task", taskFunc)
if err != nil {
    // Handle resource limits
}
```

## Testing for Leaks

### Basic Leak Detection

```go
func TestBasicFunction(t *testing.T) {
    testing.VerifyNoGoroutineLeaks(t, func() {
        // Test code that should not leak
        myFunction()
    })
}
```

### Advanced Leak Detection

```go
func TestAdvancedFunction(t *testing.T) {
    testing.VerifyNoGoroutineLeaksWithOptions(t, 
        func(detector *testing.EnhancedGoroutineLeakDetector) {
            detector.
                WithTolerance(5).
                WithMaxWaitTime(30*time.Second).
                WithExcludePatterns("background-service", "metrics-collector")
        }, func() {
            // Complex test that creates expected background goroutines
            complexService.Start()
            defer complexService.Stop()
            
            complexService.DoWork()
        })
}
```

### Manual Snapshot Comparison

```go
func TestManualDetection(t *testing.T) {
    detector := testing.NewEnhancedGoroutineLeakDetector(t)
    
    before := detector.GetCurrentSnapshot()
    
    // Operations that might leak
    performOperations()
    
    after := detector.GetCurrentSnapshot()
    
    leaked := detector.CompareSnapshots(before, after)
    if len(leaked) > 0 {
        t.Errorf("Detected %d leaked goroutine types", len(leaked))
        for _, leak := range leaked {
            t.Logf("Leaked: %s (count: %d)", leak.Signature, leak.Count)
        }
    }
}
```

## Implementation Examples

### HTTP Client with Proper Lifecycle

```go
type HTTPClient struct {
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup
    client     *http.Client
    connPool   *ConnectionPool
}

func NewHTTPClient() *HTTPClient {
    ctx, cancel := context.WithCancel(context.Background())
    return &HTTPClient{
        ctx:    ctx,
        cancel: cancel,
        client: &http.Client{},
    }
}

func (c *HTTPClient) Start() error {
    // Start connection pool maintenance
    c.wg.Add(1)
    go c.maintainConnections()
    
    // Start metrics collection
    c.wg.Add(1)
    go c.collectMetrics()
    
    return nil
}

func (c *HTTPClient) Stop(timeout time.Duration) error {
    c.cancel() // Signal shutdown
    
    done := make(chan struct{})
    go func() {
        c.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return nil
    case <-time.After(timeout):
        return fmt.Errorf("shutdown timeout")
    }
}

func (c *HTTPClient) maintainConnections() {
    defer c.wg.Done()
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-c.ctx.Done():
            return
        case <-ticker.C:
            c.connPool.Cleanup()
        }
    }
}

func (c *HTTPClient) collectMetrics() {
    defer c.wg.Done()
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-c.ctx.Done():
            return
        case <-ticker.C:
            c.updateMetrics()
        }
    }
}
```

### Event Processor with Worker Pool

```go
type EventProcessor struct {
    manager    *testing.GoroutineLifecycleManager
    inputCh    chan Event
    numWorkers int
}

func NewEventProcessor(numWorkers int) *EventProcessor {
    return &EventProcessor{
        manager:    testing.NewGoroutineLifecycleManager("event-processor"),
        inputCh:    make(chan Event, 1000),
        numWorkers: numWorkers,
    }
}

func (ep *EventProcessor) Start() error {
    // Start worker pool
    for i := 0; i < ep.numWorkers; i++ {
        workerID := fmt.Sprintf("worker-%d", i)
        if err := ep.manager.GoWorker(workerID, ep.inputCh, ep.processEvent); err != nil {
            return fmt.Errorf("failed to start worker %s: %w", workerID, err)
        }
    }
    
    // Start metrics collection
    return ep.manager.GoTicker("metrics", 30*time.Second, ep.collectMetrics)
}

func (ep *EventProcessor) Stop() error {
    close(ep.inputCh)
    return ep.manager.ShutdownWithTimeout()
}

func (ep *EventProcessor) processEvent(ctx context.Context, event interface{}) {
    e := event.(Event)
    // Process the event
    processEventLogic(e)
}

func (ep *EventProcessor) collectMetrics(ctx context.Context) {
    // Collect processing metrics
}
```

## Migration Guide

### Step 1: Identify Existing Goroutines

1. Search for `go func()` and `go someFunction()` patterns
2. Identify goroutines without proper lifecycle management
3. Look for missing context usage and WaitGroup coordination

### Step 2: Add Leak Detection to Tests

```go
// Add to existing tests
func TestExistingFunction(t *testing.T) {
    testing.VerifyNoGoroutineLeaks(t, func() {
        // Existing test code
        existingFunction()
    })
}
```

### Step 3: Implement Proper Lifecycle Management

1. Add context support to long-running goroutines
2. Implement graceful shutdown with timeouts
3. Use WaitGroups for coordination
4. Add proper cleanup in defer statements

### Step 4: Use Lifecycle Manager for New Code

```go
// Instead of raw goroutines
go func() {
    // Work
}()

// Use lifecycle manager
manager := testing.NewGoroutineLifecycleManager("service")
defer manager.MustShutdown()
manager.Go("worker", func(ctx context.Context) {
    // Work with automatic cleanup
})
```

### Step 5: Validate with Enhanced Detection

Run tests with enhanced leak detection to ensure no regressions:

```bash
go test -v ./... -run TestLeak
```

## Monitoring and Alerts

### Runtime Monitoring

```go
// Add to your monitoring system
func monitorGoroutines() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        count := runtime.NumGoroutine()
        if count > 1000 { // Threshold
            log.Printf("High goroutine count: %d", count)
            // Alert or dump stack traces
        }
    }
}
```

### Debugging High Goroutine Counts

```go
func debugGoroutines() {
    buf := make([]byte, 2<<20) // 2MB buffer
    n := runtime.Stack(buf, true)
    log.Printf("Goroutine dump:\n%s", buf[:n])
}
```

This comprehensive guide and the accompanying utilities provide a robust foundation for preventing goroutine leaks in the AG UI Go SDK. Use these patterns consistently to ensure reliable, leak-free applications.