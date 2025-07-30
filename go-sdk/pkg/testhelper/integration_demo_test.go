package testhelper

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestBasicGoroutineLeakDetection demonstrates basic leak detection
func TestBasicGoroutineLeakDetection(t *testing.T) {
	defer VerifyNoGoroutineLeaks(t)

	// This test should pass - no goroutines leak
	done := make(chan struct{})

	go func() {
		<-done
	}()

	close(done)
	time.Sleep(1 * time.Millisecond) // Allow goroutine to exit
}

// TestChannelCleanupDemo demonstrates channel cleanup
func TestChannelCleanupDemo(t *testing.T) {
	defer VerifyNoGoroutineLeaks(t)

	cc := NewChannelCleanup(t)

	ch1 := make(chan int, 5)
	ch2 := make(chan string, 5)

	AddChan(cc, "numbers", ch1)
	AddChan(cc, "strings", ch2)

	// Use channels
	ch1 <- 42
	ch2 <- "hello"

	select {
	case n := <-ch1:
		t.Logf("Received number: %d", n)
	default:
		t.Error("Should have received number")
	}

	select {
	case s := <-ch2:
		t.Logf("Received string: %s", s)
	default:
		t.Error("Should have received string")
	}

	// Channels will be cleaned up automatically
}

// TestWaitGroupTimeoutDemo demonstrates WaitGroup timeout helper
func TestWaitGroupTimeoutDemo(t *testing.T) {
	defer VerifyNoGoroutineLeaks(t)

	var wg sync.WaitGroup

	// Start some workers that complete quickly
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			time.Sleep(1 * time.Millisecond)
			t.Logf("Worker %d completed", id)
		}(i)
	}

	// Wait with timeout - should succeed
	if !WaitGroupTimeout(t, &wg, 1*time.Second) {
		t.Fatal("Workers should have completed within timeout")
	}

	t.Log("All workers completed successfully")
}

// TestCleanupManagerDemo demonstrates cleanup manager
func TestCleanupManagerDemo(t *testing.T) {
	defer VerifyNoGoroutineLeaks(t)

	cleanup := NewCleanupManager(t)

	// Simulate creating resources
	cleaned := make(map[string]bool)

	cleanup.Register("resource-1", func() {
		cleaned["resource-1"] = true
		t.Log("Cleaned up resource-1")
	})

	cleanup.Register("resource-2", func() {
		cleaned["resource-2"] = true
		t.Log("Cleaned up resource-2")
	})

	// Cleanup a specific resource early
	cleanup.Cleanup("resource-1")

	if !cleaned["resource-1"] {
		t.Error("Resource-1 should have been cleaned up")
	}

	if cleaned["resource-2"] {
		t.Error("Resource-2 should not have been cleaned up yet")
	}

	// resource-2 will be cleaned up automatically at test end
}

// TestContextManagerDemo demonstrates context management
func TestContextManagerDemo(t *testing.T) {
	defer VerifyNoGoroutineLeaks(t)

	cm := NewContextManager(t)

	// Create contexts for different operations
	fastCtx := cm.CreateWithTimeout("fast-op", 100*time.Millisecond)
	slowCtx := cm.CreateWithTimeout("slow-op", 1*time.Second)

	// Start operations
	fastDone := make(chan bool)
	go func() {
		select {
		case <-time.After(50 * time.Millisecond):
			fastDone <- true
		case <-fastCtx.Done():
			fastDone <- false
		}
	}()

	slowDone := make(chan bool)
	go func() {
		select {
		case <-time.After(200 * time.Millisecond):
			slowDone <- true
		case <-slowCtx.Done():
			slowDone <- false
		}
	}()

	// Fast operation should complete
	if !<-fastDone {
		t.Error("Fast operation should have completed")
	}

	// Slow operation should also complete (within 1s timeout)
	if !<-slowDone {
		t.Error("Slow operation should have completed")
	}

	t.Log("All operations completed successfully")
}

// TestTimeoutGuardDemo demonstrates timeout protection
func TestTimeoutGuardDemo(t *testing.T) {
	defer VerifyNoGoroutineLeaks(t)

	guard := NewTimeoutGuard(t, 50*time.Millisecond)

	// Fast operation should succeed
	err := guard.Run("fast-operation", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	if err != nil {
		t.Errorf("Fast operation should succeed: %v", err)
	}

	// Slow operation should timeout - use context-aware version
	err = guard.RunWithContext("slow-operation", func(ctx context.Context) error {
		select {
		case <-time.After(200 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	if err == nil {
		t.Error("Slow operation should have timed out")
	}

	t.Logf("Timeout guard worked correctly: %v", err)
}

// TestResourceTrackerDemo demonstrates resource tracking
func TestResourceTrackerDemo(t *testing.T) {
	defer VerifyNoGoroutineLeaks(t)

	rt := NewResourceTracker(t)

	// Allocate some resources
	rt.Allocated("connection-1")
	rt.Allocated("buffer-1")
	rt.Allocated("worker-1")

	// Simulate using resources
	time.Sleep(5 * time.Millisecond)

	// Clean up all resources
	rt.Cleaned("connection-1")
	rt.Cleaned("buffer-1")
	rt.Cleaned("worker-1")

	t.Log("Resource tracking demo completed")
}

// TestComprehensiveDemo shows multiple helpers working together
func TestComprehensiveDemo(t *testing.T) {
	defer VerifyNoGoroutineLeaks(t)

	// Set up all helpers
	cleanup := NewCleanupManager(t)
	cc := NewChannelCleanup(t)
	ctx := NewTestContextWithTimeout(t, 5*time.Second)

	// Create channels
	workCh := make(chan int, 10)
	doneCh := make(chan struct{})

	AddChan(cc, "work", workCh)
	AddChan(cc, "done", doneCh)

	// Start workers
	var wg sync.WaitGroup
	workerCount := 3

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for {
				select {
				case work := <-workCh:
					// Simulate work
					time.Sleep(1 * time.Millisecond)
					t.Logf("Worker %d processed: %d", id, work)
				case <-doneCh:
					t.Logf("Worker %d stopping", id)
					return
				case <-ctx.Done():
					t.Logf("Worker %d cancelled", id)
					return
				}
			}
		}(i)
	}

	// Register worker cleanup
	cleanup.Register("workers", func() {
		close(doneCh)
		if !WaitGroupTimeout(t, &wg, 1*time.Second) {
			t.Log("Workers did not stop gracefully")
		}
	})

	// Send some work
	for i := 0; i < 5; i++ {
		workCh <- i
	}

	// Let workers process
	time.Sleep(100 * time.Millisecond)

	// Trigger cleanup by manually calling it
	cleanup.Cleanup("workers")

	t.Log("Comprehensive demo completed successfully")
}
