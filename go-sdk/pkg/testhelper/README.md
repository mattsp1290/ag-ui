# Test Helper Package

The `testhelper` package provides utilities for robust testing, including goroutine leak detection, resource cleanup helpers, and context management utilities.

## Features

### Goroutine Leak Detection

Automatically detect and report goroutine leaks in tests:

```go
func TestExample(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    // Your test code here
    // If any goroutines leak, the test will fail with detailed information
}
```

#### Advanced Leak Detection

```go
func TestWithCustomDetection(t *testing.T) {
    detector := testhelper.NewGoroutineLeakDetector(t).
        WithThreshold(10).                    // Allow up to 10 extra goroutines
        WithCheckDelay(200*time.Millisecond)  // Wait 200ms before checking
    
    detector.Start()
    
    // Your test code
    
    detector.Check()
}
```

#### Monitoring Goroutines

```go
func TestWithMonitoring(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    // Log goroutine counts every second
    stopMonitor := testhelper.MonitorGoroutines(t, 1*time.Second)
    defer stopMonitor()
    
    // Your test code
}
```

### Resource Cleanup

#### Cleanup Manager

Manages cleanup of multiple resources:

```go
func TestWithCleanup(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    cleanup := testhelper.NewCleanupManager(t)
    
    // Register cleanup operations
    cleanup.Register("database", func() {
        db.Close()
    })
    
    cleanup.Register("server", func() {
        server.Stop()
    })
    
    // Cleanup happens automatically on test end
}
```

#### Channel Cleanup

Safely close channels:

```go
func TestChannels(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    cc := testhelper.NewChannelCleanup(t)
    
    ch1 := make(chan int)
    ch2 := make(chan string)
    
    testhelper.AddChan(cc, "numbers", ch1)
    testhelper.AddChan(cc, "strings", ch2)
    
    // Channels are closed automatically
}
```

#### Network Cleanup

Clean up network resources:

```go
func TestNetwork(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    nc := testhelper.NewNetworkCleanup(t)
    
    listener, _ := net.Listen("tcp", ":0")
    nc.AddListener(listener)
    
    conn, _ := net.Dial("tcp", "example.com:80")
    nc.AddConnection(conn)
    
    // All network resources closed automatically
}
```

### Context Management

#### Test Contexts

Automatically cancelled contexts:

```go
func TestWithContext(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    // Auto-cancelled at test end
    ctx := testhelper.NewTestContext(t)
    
    // With timeout
    ctx2 := testhelper.NewTestContextWithTimeout(t, 5*time.Second)
    
    // Use contexts in your operations
}
```

#### Context Manager

Manage multiple named contexts:

```go
func TestMultipleContexts(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    cm := testhelper.NewContextManager(t)
    
    workerCtx := cm.Create("worker")
    managerCtx := cm.CreateWithTimeout("manager", 10*time.Second)
    
    // Cancel specific contexts
    cm.Cancel("worker")
    
    // All contexts cancelled automatically at test end
}
```

#### Parallel Contexts

For testing parallel components:

```go
func TestParallel(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    pc := testhelper.NewParallelContexts(t)
    
    ctx1 := pc.Create("component-1")
    ctx2 := pc.Create("component-2")
    
    // Start parallel operations
    go func() { <-ctx1.Done() }()
    go func() { <-ctx2.Done() }()
    
    // Stop components
    pc.Cancel("component-1")
    pc.Cancel("component-2")
}
```

### Synchronization Helpers

#### WaitGroup with Timeout

```go
func TestConcurrent(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    var wg sync.WaitGroup
    
    // Start workers
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            // Work here
        }()
    }
    
    // Wait with timeout
    if !testhelper.WaitGroupTimeout(t, &wg, 5*time.Second) {
        t.Fatal("Workers didn't complete in time")
    }
}
```

#### Timeout Guard

Protect operations with timeouts:

```go
func TestWithTimeout(t *testing.T) {
    guard := testhelper.NewTimeoutGuard(t, 2*time.Second)
    
    err := guard.Run("slow-operation", func() error {
        // Potentially slow operation
        time.Sleep(1 * time.Second)
        return nil
    })
    
    if err != nil {
        t.Errorf("Operation failed: %v", err)
    }
}
```

### Resource Tracking

Track resource allocation and cleanup:

```go
func TestResourceTracking(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    rt := testhelper.NewResourceTracker(t)
    
    // Mark allocation
    rt.Allocated("connection-1")
    rt.Allocated("buffer-1")
    
    // Use resources...
    
    // Mark cleanup
    rt.Cleaned("connection-1")
    rt.Cleaned("buffer-1")
    
    // Report shows resource lifetimes and any leaks
}
```

### Utility Functions

#### Safe Channel Operations

```go
// Close channels safely
testhelper.CloseChannel(t, ch, "my-channel")

// Drain then close
testhelper.DrainChannel(t, ch, "my-channel", 1*time.Second)
```

#### Connection Cleanup

```go
// Close any io.Closer
testhelper.CloseConnection(t, conn, "database")
```

#### Context Assertions

```go
// Verify context was cancelled
testhelper.AssertContextCancelled(t, ctx, 1*time.Second)

// Verify context timed out
testhelper.AssertContextTimeout(t, ctx, 1*time.Second)
```

## Best Practices

### 1. Always Use Goroutine Leak Detection

Add this to every test that might create goroutines:

```go
func TestExample(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    // rest of test
}
```

### 2. Use Test Contexts

Replace manual context creation:

```go
// Instead of:
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// Use:
ctx := testhelper.NewTestContextWithTimeout(t, 10*time.Second)
```

### 3. Register Cleanup Early

Set up cleanup as soon as resources are created:

```go
func TestExample(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    cleanup := testhelper.NewCleanupManager(t)
    
    server := startServer()
    cleanup.Register("server", func() {
        server.Stop()
    })
    
    // Continue with test
}
```

### 4. Use Timeouts for Synchronization

Never use bare WaitGroup.Wait():

```go
// Instead of:
wg.Wait()

// Use:
if !testhelper.WaitGroupTimeout(t, &wg, 5*time.Second) {
    t.Fatal("Timeout waiting for workers")
}
```

### 5. Monitor Long-Running Tests

For tests that run for a while:

```go
func TestLongRunning(t *testing.T) {
    defer testhelper.VerifyNoGoroutineLeaks(t)
    
    stopMonitor := testhelper.MonitorGoroutines(t, 5*time.Second)
    defer stopMonitor()
    
    // Long-running test code
}
```

## Integration Examples

See `examples_test.go` for comprehensive examples showing how to use multiple helpers together.

## Common Issues and Solutions

### Goroutine Leaks

If you see goroutine leaks:

1. Check for unclosed channels
2. Verify context cancellation
3. Look for infinite loops without exit conditions
4. Ensure network connections are closed

### Channel Deadlocks

Use the channel cleanup helpers and drain channels before closing:

```go
testhelper.DrainChannel(t, ch, "work-queue", 1*time.Second)
```

### Context Not Cancelled

Make sure you're using test contexts that auto-cancel:

```go
ctx := testhelper.NewTestContext(t)  // Auto-cancelled
```

### Resource Leaks

Use the resource tracker to identify what's not being cleaned up:

```go
rt := testhelper.NewResourceTracker(t)
rt.Allocated("resource-name")
// ... use resource ...
rt.Cleaned("resource-name")
```