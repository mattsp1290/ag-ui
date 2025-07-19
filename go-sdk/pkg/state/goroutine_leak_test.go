package state

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// TestGoroutineLeakFixes tests that goroutine leaks are fixed
func TestGoroutineLeakFixes(t *testing.T) {
	// Record initial goroutine count
	initialGoroutines := runtime.NumGoroutine()
	
	// Test LazyCache
	t.Run("LazyCache", func(t *testing.T) {
		cache := NewLazyCache(10, time.Minute)
		cache.Set("key", "value")
		
		// Give some time for cleanup goroutine to start
		time.Sleep(100 * time.Millisecond)
		
		// Close the cache
		cache.Close()
		
		// Give time for cleanup goroutine to exit
		time.Sleep(100 * time.Millisecond)
	})
	
	// Test RateLimiter
	t.Run("RateLimiter", func(t *testing.T) {
		limiter := NewRateLimiter(10)
		
		// Give some time for generate goroutine to start
		time.Sleep(100 * time.Millisecond)
		
		// Stop the limiter
		limiter.Stop()
		
		// Give time for generate goroutine to exit
		time.Sleep(100 * time.Millisecond)
	})
	
	// Test StateStore
	t.Run("StateStore", func(t *testing.T) {
		store := NewStateStore()
		
		// Subscribe to trigger cleanup goroutines
		unsubscribe := store.Subscribe("/test", func(change StateChange) {})
		defer unsubscribe()
		
		// Set some data and trigger cleanup
		store.Set("/test", "value")
		
		// Give some time for cleanup goroutines to be created
		time.Sleep(100 * time.Millisecond)
		
		// Close the store
		store.Close()
		
		// Give time for cleanup goroutines to exit
		time.Sleep(100 * time.Millisecond)
	})
	
	// Test StateManager
	t.Run("StateManager", func(t *testing.T) {
		opts := DefaultManagerOptions()
		opts.EnableMetrics = true
		opts.AutoCheckpoint = true
		
		sm, err := NewStateManager(opts)
		if err != nil {
			t.Fatalf("Failed to create StateManager: %v", err)
		}
		
		// Give some time for background goroutines to start
		time.Sleep(200 * time.Millisecond)
		
		// Close the state manager
		err = sm.Close()
		if err != nil {
			t.Fatalf("Failed to close StateManager: %v", err)
		}
		
		// Give time for background goroutines to exit
		time.Sleep(200 * time.Millisecond)
	})
	
	// Test PerformanceOptimizer
	t.Run("PerformanceOptimizer", func(t *testing.T) {
		opts := DefaultPerformanceOptions()
		opts.EnableBatching = true
		opts.EnableLazyLoading = true
		
		po := NewPerformanceOptimizerImpl(opts)
		
		// Give some time for background goroutines to start
		time.Sleep(200 * time.Millisecond)
		
		// Stop the optimizer
		po.Stop()
		
		// Give time for background goroutines to exit
		time.Sleep(200 * time.Millisecond)
	})
	
	// Allow some time for all goroutines to finish
	time.Sleep(500 * time.Millisecond)
	
	// Force garbage collection to clean up any lingering goroutines
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	// Check final goroutine count
	finalGoroutines := runtime.NumGoroutine()
	
	// Allow some leeway for test goroutines and framework goroutines
	leeway := 5
	if finalGoroutines > initialGoroutines+leeway {
		t.Logf("Initial goroutines: %d", initialGoroutines)
		t.Logf("Final goroutines: %d", finalGoroutines)
		t.Errorf("Potential goroutine leak detected: %d goroutines leaked", finalGoroutines-initialGoroutines)
	} else {
		t.Logf("No goroutine leaks detected. Initial: %d, Final: %d", initialGoroutines, finalGoroutines)
	}
}

// TestStateManagerGoroutineCleanup specifically tests StateManager cleanup
func TestStateManagerGoroutineCleanup(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()
	
	for i := 0; i < 5; i++ {
		func() {
			opts := DefaultManagerOptions()
			opts.EnableMetrics = true
			opts.AutoCheckpoint = true
			
			sm, err := NewStateManager(opts)
			if err != nil {
				t.Fatalf("Failed to create StateManager: %v", err)
			}
			
			// Create some state
			ctx := context.Background()
			contextID, err := sm.CreateContext(ctx, "test-state", nil)
			if err != nil {
				t.Fatalf("Failed to create context: %v", err)
			}
			
			// Update some state
			updates := map[string]interface{}{
				"test": "value",
			}
			_, err = sm.UpdateState(ctx, contextID, "test-state", updates, UpdateOptions{})
			if err != nil {
				t.Fatalf("Failed to update state: %v", err)
			}
			
			// Give some time for operations to complete
			time.Sleep(50 * time.Millisecond)
			
			// Close the state manager
			err = sm.Close()
			if err != nil {
				t.Fatalf("Failed to close StateManager: %v", err)
			}
		}()
	}
	
	// Allow time for cleanup
	time.Sleep(300 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	finalGoroutines := runtime.NumGoroutine()
	
	// Be more strict here as we're creating and destroying multiple managers
	leeway := 3
	if finalGoroutines > initialGoroutines+leeway {
		t.Logf("Initial goroutines: %d", initialGoroutines)
		t.Logf("Final goroutines: %d", finalGoroutines)
		t.Errorf("StateManager goroutine leak detected: %d goroutines leaked", finalGoroutines-initialGoroutines)
	} else {
		t.Logf("StateManager cleanup successful. Initial: %d, Final: %d", initialGoroutines, finalGoroutines)
	}
}