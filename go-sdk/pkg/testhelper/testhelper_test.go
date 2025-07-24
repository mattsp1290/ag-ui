package testhelper

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// TestGoroutineLeakDetector tests the goroutine leak detector
func TestGoroutineLeakDetector(t *testing.T) {
	t.Run("NoLeak", func(t *testing.T) {
		defer VerifyNoGoroutineLeaks(t)

		// Simple operation that doesn't leak
		ch := make(chan int)
		go func() {
			ch <- 42
		}()
		<-ch
	})

	t.Run("WithCleanup", func(t *testing.T) {
		detector := NewGoroutineLeakDetector(t)
		detector.Start()

		// Start some goroutines
		done := make(chan struct{})
		var wg sync.WaitGroup

		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-done
			}()
		}

		// Cleanup
		close(done)
		wg.Wait()

		detector.Check()
	})
}

// TestCleanupHelpers tests cleanup utilities
func TestCleanupHelpers(t *testing.T) {
	t.Run("CleanupManager", func(t *testing.T) {
		cm := NewCleanupManager(t)

		cleaned := false
		cm.Register("test-resource", func() {
			cleaned = true
		})

		cm.Cleanup("test-resource")

		if !cleaned {
			t.Error("Resource was not cleaned up")
		}
	})

	t.Run("ChannelCleanup", func(t *testing.T) {
		cc := NewChannelCleanup(t)

		ch1 := make(chan int)
		ch2 := make(chan string)

		AddChan(cc, "int-channel", ch1)
		AddChan(cc, "string-channel", ch2)

		// Channels will be closed on test cleanup
	})

	t.Run("NetworkCleanup", func(t *testing.T) {
		nc := NewNetworkCleanup(t)

		// Create a listener
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		nc.AddListener(listener)

		// Create a connection
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		nc.AddConnection(conn)

		// Resources will be cleaned up automatically
	})

	t.Run("ResourceTracker", func(t *testing.T) {
		rt := NewResourceTracker(t)

		rt.Allocated("connection-1")
		rt.Allocated("buffer-1")

		// Simulate using resources
		time.Sleep(10 * time.Millisecond)

		rt.Cleaned("connection-1")
		rt.Cleaned("buffer-1")

		// Report will show resource lifetimes
	})
}

// TestContextHelpers tests context management utilities
func TestContextHelpers(t *testing.T) {
	t.Run("TestContext", func(t *testing.T) {
		ctx := NewTestContext(t)

		select {
		case <-ctx.Done():
			t.Error("Context should not be done yet")
		default:
			// Expected
		}

		// Context will be cancelled on test cleanup
	})

	t.Run("TestContextWithTimeout", func(t *testing.T) {
		ctx := NewTestContextWithTimeout(t, 100*time.Millisecond)

		select {
		case <-ctx.Done():
			// Expected after timeout
		case <-time.After(200 * time.Millisecond):
			t.Error("Context should have timed out")
		}
	})

	t.Run("ContextManager", func(t *testing.T) {
		cm := NewContextManager(t)

		ctx1 := cm.Create("worker-1")
		ctx2 := cm.CreateWithTimeout("worker-2", 50*time.Millisecond)

		// Use contexts
		go func() {
			<-ctx1.Done()
		}()

		go func() {
			<-ctx2.Done()
		}()

		// All contexts will be cancelled on test cleanup
	})

	t.Run("TimeoutGuard", func(t *testing.T) {
		guard := NewTimeoutGuard(t, 100*time.Millisecond)

		// Fast operation succeeds
		err := guard.Run("fast-op", func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		if err != nil {
			t.Errorf("Fast operation failed: %v", err)
		}

		// Slow operation times out - use context-aware version
		err = guard.RunWithContext("slow-op", func(ctx context.Context) error {
			select {
			case <-time.After(200 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		if err != context.DeadlineExceeded {
			t.Errorf("Expected timeout error, got: %v", err)
		}
	})
}

// TestIntegration demonstrates using multiple helpers together
func TestIntegration(t *testing.T) {
	// Set up goroutine leak detection
	defer VerifyNoGoroutineLeaks(t)

	// Set up context management
	ctx := NewTestContext(t)

	// Set up cleanup management
	cm := NewCleanupManager(t)
	cc := NewChannelCleanup(t)

	// Create resources
	resultCh := make(chan int, 10)
	errorCh := make(chan error, 1)

	AddChan(cc, "results", resultCh)
	AddChan(cc, "errors", errorCh)

	// Start workers
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			select {
			case resultCh <- id:
				t.Logf("Worker %d sent result", id)
			case <-ctx.Done():
				t.Logf("Worker %d stopped", id)
			}
		}(i)
	}

	// Register cleanup for workers
	cm.Register("workers", func() {
		ctx.Cancel()
		WaitGroupTimeout(t, &wg, 100*time.Millisecond)
	})

	// Collect some results
	for i := 0; i < 3; i++ {
		select {
		case result := <-resultCh:
			t.Logf("Got result: %d", result)
		case <-time.After(50 * time.Millisecond):
			t.Error("Timeout waiting for result")
		}
	}

	// Everything will be cleaned up automatically
}

// ExampleVerifyNoGoroutineLeaks shows basic usage
func ExampleVerifyNoGoroutineLeaks() {
	// In your test:
	// func TestSomething(t *testing.T) {
	//     defer VerifyNoGoroutineLeaks(t)
	//
	//     // Your test code here
	// }
}

// ExampleNewTestContext shows context usage
func ExampleNewTestContext() {
	// In your test:
	// func TestSomething(t *testing.T) {
	//     ctx := NewTestContext(t)
	//
	//     // Use ctx in your operations
	//     // It will be automatically cancelled when test ends
	// }
}

// ExampleNewCleanupManager shows cleanup management
func ExampleNewCleanupManager() {
	// In your test:
	// func TestSomething(t *testing.T) {
	//     cm := NewCleanupManager(t)
	//
	//     // Register cleanup operations
	//     cm.Register("database", func() {
	//         db.Close()
	//     })
	//
	//     // Cleanup happens automatically
	// }
}
