package internal

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestGoroutineUsageComparison demonstrates the reduction in goroutine count
func TestGoroutineUsageComparison(t *testing.T) {
	const numCallbacks = 1000

	// Test direct goroutines
	t.Run("DirectGoroutines", func(t *testing.T) {
		runtime.GC()
		runtime.GC() // Force GC to get accurate count
		startGoroutines := runtime.NumGoroutine()
		
		var wg sync.WaitGroup
		wg.Add(numCallbacks)
		
		startTime := time.Now()
		for i := 0; i < numCallbacks; i++ {
			go func() {
				defer wg.Done()
				// Simulate callback work
				time.Sleep(1 * time.Millisecond)
			}()
		}
		
		// Check peak goroutine count
		peakGoroutines := runtime.NumGoroutine()
		
		wg.Wait()
		duration := time.Since(startTime)
		
		runtime.GC()
		runtime.GC()
		endGoroutines := runtime.NumGoroutine()
		
		t.Logf("Direct Goroutines:")
		t.Logf("  Start goroutines: %d", startGoroutines)
		t.Logf("  Peak goroutines: %d (increase: %d)", peakGoroutines, peakGoroutines-startGoroutines)
		t.Logf("  End goroutines: %d", endGoroutines)
		t.Logf("  Duration: %v", duration)
	})

	// Test callback pool
	t.Run("CallbackPool", func(t *testing.T) {
		runtime.GC()
		runtime.GC() // Force GC to get accurate count
		startGoroutines := runtime.NumGoroutine()
		
		pool := NewCallbackPool(runtime.NumCPU())
		defer pool.Stop()
		
		afterPoolStart := runtime.NumGoroutine()
		
		var wg sync.WaitGroup
		wg.Add(numCallbacks)
		
		startTime := time.Now()
		for i := 0; i < numCallbacks; i++ {
			pool.Submit(func() {
				defer wg.Done()
				// Simulate callback work
				time.Sleep(1 * time.Millisecond)
			})
		}
		
		// Check peak goroutine count
		peakGoroutines := runtime.NumGoroutine()
		
		wg.Wait()
		duration := time.Since(startTime)
		
		runtime.GC()
		runtime.GC()
		endGoroutines := runtime.NumGoroutine()
		
		t.Logf("Callback Pool:")
		t.Logf("  Start goroutines: %d", startGoroutines)
		t.Logf("  After pool start: %d (increase: %d)", afterPoolStart, afterPoolStart-startGoroutines)
		t.Logf("  Peak goroutines: %d (increase from start: %d)", peakGoroutines, peakGoroutines-startGoroutines)
		t.Logf("  End goroutines: %d", endGoroutines)
		t.Logf("  Duration: %v", duration)
	})
}

// TestMemoryUsageComparison demonstrates the reduction in memory usage
func TestMemoryUsageComparison(t *testing.T) {
	const numCallbacks = 10000

	// Test direct goroutines
	t.Run("DirectGoroutines", func(t *testing.T) {
		runtime.GC()
		var startMem, peakMem, endMem runtime.MemStats
		runtime.ReadMemStats(&startMem)
		
		var wg sync.WaitGroup
		wg.Add(numCallbacks)
		
		for i := 0; i < numCallbacks; i++ {
			go func() {
				defer wg.Done()
				// Simulate some work
				data := make([]byte, 100)
				_ = data
			}()
		}
		
		runtime.ReadMemStats(&peakMem)
		wg.Wait()
		
		runtime.GC()
		runtime.ReadMemStats(&endMem)
		
		t.Logf("Direct Goroutines Memory Usage:")
		t.Logf("  Start: %d KB", startMem.Alloc/1024)
		t.Logf("  Peak: %d KB (increase: %d KB)", peakMem.Alloc/1024, (peakMem.Alloc-startMem.Alloc)/1024)
		t.Logf("  End: %d KB", endMem.Alloc/1024)
		t.Logf("  Total allocations: %d", peakMem.TotalAlloc-startMem.TotalAlloc)
	})

	// Test callback pool
	t.Run("CallbackPool", func(t *testing.T) {
		runtime.GC()
		var startMem, peakMem, endMem runtime.MemStats
		runtime.ReadMemStats(&startMem)
		
		pool := NewCallbackPool(runtime.NumCPU())
		defer pool.Stop()
		
		var wg sync.WaitGroup
		wg.Add(numCallbacks)
		
		for i := 0; i < numCallbacks; i++ {
			pool.Submit(func() {
				defer wg.Done()
				// Simulate some work
				data := make([]byte, 100)
				_ = data
			})
		}
		
		runtime.ReadMemStats(&peakMem)
		wg.Wait()
		
		runtime.GC()
		runtime.ReadMemStats(&endMem)
		
		t.Logf("Callback Pool Memory Usage:")
		t.Logf("  Start: %d KB", startMem.Alloc/1024)
		t.Logf("  Peak: %d KB (increase: %d KB)", peakMem.Alloc/1024, (peakMem.Alloc-startMem.Alloc)/1024)
		t.Logf("  End: %d KB", endMem.Alloc/1024)
		t.Logf("  Total allocations: %d", peakMem.TotalAlloc-startMem.TotalAlloc)
	})
}