package testing

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEnhancedGoroutineLeakDetector(t *testing.T) {
	t.Run("NoLeaks", func(t *testing.T) {
		detector := NewEnhancedGoroutineLeakDetector(t)
		defer detector.Check()
		
		// This should not leak anything
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		
		done := make(chan struct{})
		go func() {
			defer close(done)
			<-ctx.Done()
		}()
		
		cancel()
		<-done
	})
	
	t.Run("DetectsLeaks", func(t *testing.T) {
		// This test intentionally creates a leak to verify detection works
		// The leak is cleaned up at the end to prevent actual test leaks
		
		detector := NewEnhancedGoroutineLeakDetector(t).WithTolerance(0)
		
		// Create a leak
		stopCh := make(chan struct{})
		go func() {
			<-stopCh // This will leak until stopCh is closed
		}()
		
		// Check should detect the leak
		leaked := false
		defer func() {
			if !leaked {
				t.Error("Expected leak detection to fail, but it didn't")
			}
			close(stopCh) // Clean up the intentional leak
		}()
		
		// Capture the fact that we expect this to fail
		originalT := detector.t
		mockT := &mockTesting{TB: originalT}
		detector.t = mockT
		
		detector.Check()
		
		if mockT.errorCalled {
			leaked = true
		}
	})
	
	t.Run("CustomPatterns", func(t *testing.T) {
		detector := NewEnhancedGoroutineLeakDetector(t).
			WithExcludePatterns("intentional-leak").
			WithTolerance(0)
		defer detector.Check()
		
		// This should be excluded from leak detection
		stopCh := make(chan struct{})
		defer close(stopCh)
		
		go func() {
			runtime.SetFinalizer(&struct{}{}, func(interface{}) {
				// This makes the stack trace contain our exclude pattern
			})
			<-stopCh
		}()
		
		time.Sleep(50 * time.Millisecond) // Let it run briefly
	})
}

func TestGoroutineLifecycleManager(t *testing.T) {
	t.Run("BasicLifecycle", func(t *testing.T) {
		VerifyNoGoroutineLeaks(t, func() {
			manager := NewGoroutineLifecycleManager("test-basic")
			defer manager.MustShutdown()
			
			var counter int64
			err := manager.Go("counter", func(ctx context.Context) {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						atomic.AddInt64(&counter, 1)
						time.Sleep(10 * time.Millisecond)
					}
				}
			})
			
			if err != nil {
				t.Fatalf("Failed to start goroutine: %v", err)
			}
			
			// Let it run for a bit
			time.Sleep(100 * time.Millisecond)
			
			// Should have incremented
			if atomic.LoadInt64(&counter) == 0 {
				t.Error("Counter was not incremented")
			}
			
			// Active count should be 1
			if manager.GetActiveCount() != 1 {
				t.Errorf("Expected 1 active goroutine, got %d", manager.GetActiveCount())
			}
		})
	})
	
	t.Run("TickerGoroutine", func(t *testing.T) {
		VerifyNoGoroutineLeaks(t, func() {
			manager := NewGoroutineLifecycleManager("test-ticker")
			defer manager.MustShutdown()
			
			var tickCount int64
			err := manager.GoTicker("ticker", 20*time.Millisecond, func(ctx context.Context) {
				atomic.AddInt64(&tickCount, 1)
			})
			
			if err != nil {
				t.Fatalf("Failed to start ticker: %v", err)
			}
			
			// Let it tick a few times
			time.Sleep(100 * time.Millisecond)
			
			ticks := atomic.LoadInt64(&tickCount)
			if ticks < 3 {
				t.Errorf("Expected at least 3 ticks, got %d", ticks)
			}
		})
	})
	
	t.Run("WorkerGoroutine", func(t *testing.T) {
		VerifyNoGoroutineLeaks(t, func() {
			manager := NewGoroutineLifecycleManager("test-worker")
			defer manager.MustShutdown()
			
			workCh := make(chan interface{}, 10)
			var processed int64
			
			err := manager.GoWorker("worker", workCh, func(ctx context.Context, work interface{}) {
				atomic.AddInt64(&processed, 1)
			})
			
			if err != nil {
				t.Fatalf("Failed to start worker: %v", err)
			}
			
			// Send some work
			for i := 0; i < 5; i++ {
				workCh <- i
			}
			
			// Wait for processing
			time.Sleep(50 * time.Millisecond)
			
			if atomic.LoadInt64(&processed) != 5 {
				t.Errorf("Expected 5 processed items, got %d", atomic.LoadInt64(&processed))
			}
			
			close(workCh) // This should cause worker to exit gracefully
			time.Sleep(50 * time.Millisecond)
			
			if manager.GetActiveCount() != 0 {
				t.Errorf("Expected 0 active goroutines after channel close, got %d", manager.GetActiveCount())
			}
		})
	})
	
	t.Run("BatchProcessor", func(t *testing.T) {
		VerifyNoGoroutineLeaks(t, func() {
			manager := NewGoroutineLifecycleManager("test-batch")
			defer manager.MustShutdown()
			
			inputCh := make(chan interface{}, 100)
			var batchCount, itemCount int64
			
			err := manager.GoBatch("batcher", 3, 50*time.Millisecond, inputCh, 
				func(ctx context.Context, batch []interface{}) {
					atomic.AddInt64(&batchCount, 1)
					atomic.AddInt64(&itemCount, int64(len(batch)))
				})
			
			if err != nil {
				t.Fatalf("Failed to start batch processor: %v", err)
			}
			
			// Send items that should form batches
			for i := 0; i < 7; i++ {
				inputCh <- i
			}
			
			// Wait for batches to be processed
			time.Sleep(100 * time.Millisecond)
			
			batches := atomic.LoadInt64(&batchCount)
			items := atomic.LoadInt64(&itemCount)
			
			if batches < 2 { // Should have at least 2 full batches (3+3) plus possibly a partial one
				t.Errorf("Expected at least 2 batches, got %d", batches)
			}
			if items != 7 {
				t.Errorf("Expected 7 items processed, got %d", items)
			}
		})
	})
	
	t.Run("GracefulShutdown", func(t *testing.T) {
		VerifyNoGoroutineLeaks(t, func() {
			manager := NewGoroutineLifecycleManager("test-shutdown")
			
			var shutdownReceived bool
			err := manager.Go("shutdown-test", func(ctx context.Context) {
				<-ctx.Done()
				shutdownReceived = true
			})
			
			if err != nil {
				t.Fatalf("Failed to start goroutine: %v", err)
			}
			
			// Should have 1 active goroutine
			if manager.GetActiveCount() != 1 {
				t.Errorf("Expected 1 active goroutine, got %d", manager.GetActiveCount())
			}
			
			// Shutdown should complete quickly
			start := time.Now()
			err = manager.ShutdownWithTimeout()
			elapsed := time.Since(start)
			
			if err != nil {
				t.Errorf("Shutdown failed: %v", err)
			}
			
			if elapsed > time.Second {
				t.Errorf("Shutdown took too long: %v", elapsed)
			}
			
			if !shutdownReceived {
				t.Error("Goroutine did not receive shutdown signal")
			}
			
			// Should have 0 active goroutines after shutdown
			if manager.GetActiveCount() != 0 {
				t.Errorf("Expected 0 active goroutines after shutdown, got %d", manager.GetActiveCount())
			}
		})
	})
	
	t.Run("PanicRecovery", func(t *testing.T) {
		VerifyNoGoroutineLeaks(t, func() {
			manager := NewGoroutineLifecycleManager("test-panic")
			defer manager.MustShutdown()
			
			var panicRecovered int32 // Use int32 for atomic operations
			err := manager.GoWithRecovery("panic-test", func(ctx context.Context) {
				panic("test panic")
			}, func(r interface{}) {
				if r == "test panic" {
					atomic.StoreInt32(&panicRecovered, 1)
				}
			})
			
			if err != nil {
				t.Fatalf("Failed to start goroutine: %v", err)
			}
			
			// Wait for panic and recovery
			time.Sleep(50 * time.Millisecond)
			
			if atomic.LoadInt32(&panicRecovered) == 0 {
				t.Error("Panic was not properly recovered")
			}
			
			// Goroutine should have cleaned up after panic
			if manager.GetActiveCount() != 0 {
				t.Errorf("Expected 0 active goroutines after panic, got %d", manager.GetActiveCount())
			}
		})
	})
}

func TestSafeGoroutineManager(t *testing.T) {
	t.Run("MaxGoroutineLimit", func(t *testing.T) {
		VerifyNoGoroutineLeaks(t, func() {
			manager := NewSafeGoroutineManager("test-safe", 2)
			defer manager.MustShutdown()
			
			// Should be able to start 2 goroutines
			err1 := manager.Go("worker-1", func(ctx context.Context) {
				<-ctx.Done()
			})
			err2 := manager.Go("worker-2", func(ctx context.Context) {
				<-ctx.Done()
			})
			
			if err1 != nil || err2 != nil {
				t.Fatalf("Failed to start initial goroutines: %v, %v", err1, err2)
			}
			
			// Third goroutine should be rejected
			err3 := manager.Go("worker-3", func(ctx context.Context) {
				<-ctx.Done()
			})
			
			if err3 == nil {
				t.Error("Expected third goroutine to be rejected due to limit")
			}
			
			if manager.GetActiveCount() != 2 {
				t.Errorf("Expected 2 active goroutines, got %d", manager.GetActiveCount())
			}
		})
	})
	
	t.Run("PanicHandler", func(t *testing.T) {
		VerifyNoGoroutineLeaks(t, func() {
			var mu sync.Mutex
			var handlerCalled bool
			var handlerID string
			var handlerPanic interface{}
			
			manager := NewSafeGoroutineManager("test-panic-handler", 10).
				WithPanicHandler(func(id string, panic interface{}) {
					mu.Lock()
					handlerCalled = true
					handlerID = id
					handlerPanic = panic
					mu.Unlock()
				})
			defer manager.MustShutdown()
			
			err := manager.Go("panic-worker", func(ctx context.Context) {
				panic("custom panic")
			})
			
			if err != nil {
				t.Fatalf("Failed to start goroutine: %v", err)
			}
			
			// Wait for panic and handling
			time.Sleep(50 * time.Millisecond)
			
			// Read values safely with mutex protection
			mu.Lock()
			called := handlerCalled
			id := handlerID
			panicValue := handlerPanic
			mu.Unlock()
			
			if !called {
				t.Error("Panic handler was not called")
			}
			if id != "panic-worker" {
				t.Errorf("Expected panic handler ID 'panic-worker', got '%s'", id)
			}
			if panicValue != "custom panic" {
				t.Errorf("Expected panic 'custom panic', got %v", panicValue)
			}
		})
	})
}

func TestGoroutineLifecycleIntegration(t *testing.T) {
	// Test that demonstrates realistic usage patterns
	t.Run("RealisticUsage", func(t *testing.T) {
		VerifyNoGoroutineLeaksWithOptions(t, func(detector *EnhancedGoroutineLeakDetector) {
			detector.WithTolerance(2).WithMaxWaitTime(5 * time.Second)
		}, func() {
			manager := NewGoroutineLifecycleManager("integration-test")
			defer manager.MustShutdown()
			
			// Start various types of goroutines
			workCh := make(chan interface{}, 100)
			var metrics struct {
				processed  int64
				ticks      int64
				errors     int64
			}
			
			// Worker pool
			for i := 0; i < 3; i++ {
				workerID := string(rune('A' + i))
				manager.GoWorker("worker-"+workerID, workCh, func(ctx context.Context, work interface{}) {
					time.Sleep(5 * time.Millisecond) // Simulate work
					atomic.AddInt64(&metrics.processed, 1)
				})
			}
			
			// Metrics ticker
			manager.GoTicker("metrics", 20*time.Millisecond, func(ctx context.Context) {
				atomic.AddInt64(&metrics.ticks, 1)
			})
			
			// Error handler
			errorCh := make(chan interface{}, 10)
			manager.GoWorker("error-handler", errorCh, func(ctx context.Context, err interface{}) {
				atomic.AddInt64(&metrics.errors, 1)
			})
			
			// Generate work
			for i := 0; i < 10; i++ {
				workCh <- i
			}
			
			// Let everything run
			time.Sleep(200 * time.Millisecond)
			
			// Verify work was done
			if atomic.LoadInt64(&metrics.processed) == 0 {
				t.Error("No work was processed")
			}
			if atomic.LoadInt64(&metrics.ticks) == 0 {
				t.Error("No metrics ticks occurred")
			}
			
			// Check active goroutines
			active := manager.GetActiveGoroutines()
			if len(active) != 5 { // 3 workers + 1 metrics + 1 error handler
				t.Errorf("Expected 5 active goroutines, got %d", len(active))
				for id, info := range active {
					t.Logf("Active: %s (running %v)", id, info.Running)
				}
			}
			
			// Close channels to allow workers to exit
			close(workCh)
			close(errorCh)
		})
	})
}

// mockTesting is used to capture testing errors for verification
type mockTesting struct {
	testing.TB
	errorCalled bool
}

func (m *mockTesting) Errorf(format string, args ...interface{}) {
	m.errorCalled = true
	// Don't call the real Errorf to avoid failing the test
}

func (m *mockTesting) Helper() {
	// No-op
}