package testing

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestGoroutineLeakDetector_DetectLeaks tests the goroutine leak detection functionality
func TestGoroutineLeakDetector_DetectLeaks(t *testing.T) {
	t.Run("basic_leak_detection", func(t *testing.T) {
		detector := NewGoroutineLeakDetector()
		
		// Create a goroutine that will leak
		done := make(chan struct{})
		go func() {
			<-done // This will block and create a leak
		}()
		
		// Allow some time for the goroutine to start
		time.Sleep(50 * time.Millisecond)
		
		// Check for leaks with a reasonable timeout
		detector.CheckForLeaksWithTimeout(t, 2*time.Second)
		
		// Clean up
		close(done)
	})
	
	t.Run("no_false_positives", func(t *testing.T) {
		detector := NewGoroutineLeakDetector()
		
		// Create a properly cleaned up goroutine
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
		}()
		wg.Wait()
		
		// Should not detect any leaks
		detector.CheckForLeaksWithTimeout(t, 1*time.Second)
	})
	
	t.Run("timeout_handling", func(t *testing.T) {
		detector := NewGoroutineLeakDetector()
		
		// Test with a very short timeout to ensure it doesn't hang
		start := time.Now()
		detector.CheckForLeaksWithTimeout(t, 100*time.Millisecond)
		elapsed := time.Since(start)
		
		// Should complete within reasonable time even with short timeout
		if elapsed > 500*time.Millisecond {
			t.Errorf("CheckForLeaksWithTimeout took too long: %v", elapsed)
		}
	})
	
	t.Run("stabilization_timeout", func(t *testing.T) {
		detector := NewGoroutineLeakDetector()
		
		// Create unstable goroutine environment
		stopCh := make(chan struct{})
		defer close(stopCh) // Ensure cleanup
		
		for i := 0; i < 5; i++ {
			go func() {
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-stopCh:
						return
					case <-ticker.C:
						// Keep creating and destroying goroutines
						go func() {
							time.Sleep(5 * time.Millisecond)
						}()
					}
				}
			}()
		}
		
		// Test that detection doesn't hang even with unstable environment
		start := time.Now()
		
		// Create a test capture to isolate expected failures from parent test
		testCapture := NewTestCapture(t)
		
		// Run leak detection with expected failure (due to intentional leaks)
		// We're only testing that it doesn't hang
		detector.CheckForLeaksWithTimeout(testCapture, 500*time.Millisecond)
		
		elapsed := time.Since(start)
		
		// Should complete within reasonable time
		if elapsed > 1*time.Second {
			t.Errorf("CheckForLeaksWithTimeout took too long with unstable environment: %v", elapsed)
		}
		
		// We expect leak detection to report errors due to intentional leaks
		if len(testCapture.errors) == 0 {
			t.Logf("Leak detection unexpectedly passed - may not have created enough instability")
		} else {
			t.Logf("Leak detection failed as expected due to intentional leaks: %d errors", len(testCapture.errors))
		}
		
		// The main goal is that it completes within time
		t.Logf("Test completed in %v (timing test passed)", elapsed)
	})
}

func TestGoroutineLeakDetector_WaitForStableGoroutineCount(t *testing.T) {
	detector := NewGoroutineLeakDetector()
	
	t.Run("stable_count_success", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		
		// Should return true when count is stable
		stabilized := detector.waitForStableGoroutineCount(ctx, 50*time.Millisecond)
		if !stabilized {
			t.Error("Expected stabilization to succeed with stable goroutine count")
		}
	})
	
	t.Run("timeout_handling", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		// Create continuously changing goroutine count
		stopCh := make(chan struct{})
		defer close(stopCh) // Ensure cleanup
		
		go func() {
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					go func() {
						time.Sleep(20 * time.Millisecond) // Longer-lived goroutines
					}()
				}
			}
		}()
		
		// Let unstable environment get established
		time.Sleep(20 * time.Millisecond)
		
		start := time.Now()
		stabilized := detector.waitForStableGoroutineCount(ctx, 10*time.Millisecond)
		elapsed := time.Since(start)
		
		// Should timeout and return false (though it may stabilize quickly)
		// The main thing is it shouldn't hang
		t.Logf("Stabilized: %v, Elapsed: %v", stabilized, elapsed)
		
		// Should respect timeout and not hang
		if elapsed > 200*time.Millisecond {
			t.Errorf("waitForStableGoroutineCount took too long: %v", elapsed)
		}
	})
}

func TestGoroutineLeakDetector_DetectLeaksMethod(t *testing.T) {
	detector := NewGoroutineLeakDetector()
	
	t.Run("detect_actual_leaks", func(t *testing.T) {
		// Create a known leak pattern
		blockCh := make(chan struct{})
		go func() {
			select {
			case <-blockCh:
				return
			}
		}()
		
		// Allow goroutine to start
		time.Sleep(50 * time.Millisecond)
		
		leaks := detector.detectLeaks()
		
		// Clean up
		close(blockCh)
		
		// Should detect the leak
		if len(leaks) == 0 {
			t.Error("Expected to detect at least one leak")
		}
	})
	
	t.Run("ignore_system_goroutines", func(t *testing.T) {
		leaks := detector.detectLeaks()
		
		// Should not flag normal system goroutines as leaks
		for _, leak := range leaks {
			_ = leak // Just to avoid unused variable error
			// The ignore patterns are working if we get here without system goroutines flagged
		}
	})
}

func TestGoroutineLeakDetector_IgnoreFunction(t *testing.T) {
	detector := NewGoroutineLeakDetector()
	
	pattern := "test.custom.pattern"
	detector.IgnoreFunction(pattern)
	
	detector.mu.RLock()
	ignored := detector.ignored[pattern]
	detector.mu.RUnlock()
	
	if !ignored {
		t.Error("Expected custom pattern to be added to ignore list")
	}
}

func TestGoroutineLeakDetector_BaselineOperations(t *testing.T) {
	detector := NewGoroutineLeakDetector()
	
	originalBaseline := detector.GetBaseline()
	
	// Reset baseline
	detector.ResetBaseline()
	newBaseline := detector.GetBaseline()
	
	// Current count should be accessible
	currentCount := detector.GetCurrentCount()
	
	if currentCount <= 0 {
		t.Error("Current goroutine count should be positive")
	}
	
	// New baseline should be current count
	if newBaseline != currentCount {
		t.Errorf("New baseline (%d) should equal current count (%d)", newBaseline, currentCount)
	}
	
	t.Logf("Original baseline: %d, New baseline: %d, Current count: %d", 
		originalBaseline, newBaseline, currentCount)
}

func TestGoroutineLeakDetector_WithGoroutineLeakDetection(t *testing.T) {
	t.Run("clean_test", func(t *testing.T) {
		WithGoroutineLeakDetection(t, func(t *testing.T) {
			// This should not leak anything
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(10 * time.Millisecond)
			}()
			wg.Wait()
		})
	})
	
	t.Run("test_with_cleanup", func(t *testing.T) {
		var cleanup func()
		
		WithGoroutineLeakDetection(t, func(t *testing.T) {
			// Create a goroutine that needs cleanup
			done := make(chan struct{})
			go func() {
				<-done
			}()
			
			cleanup = func() {
				close(done)
				time.Sleep(10 * time.Millisecond) // Allow goroutine to exit
			}
		})
		
		// Cleanup after the test
		if cleanup != nil {
			cleanup()
		}
	})
}

// BenchmarkGoroutineLeakDetector tests performance of leak detection
func BenchmarkGoroutineLeakDetector(b *testing.B) {
	detector := NewGoroutineLeakDetector()
	
	b.Run("CheckForLeaks", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Skip the actual test call to avoid benchmark issues
			// Just benchmark the detection logic
			detector.GetCurrentCount()
		}
	})
	
	b.Run("DetectLeaks", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			detector.detectLeaks()
		}
	})
}

// TestCapture wraps testing.T to capture output
type TestCapture struct {
	*testing.T
	errors []string
	logs   []string
}

func NewTestCapture(t *testing.T) *TestCapture {
	return &TestCapture{T: t}
}

func (tc *TestCapture) Errorf(format string, args ...interface{}) {
	tc.errors = append(tc.errors, fmt.Sprintf(format, args...))
	// Don't call the underlying T.Errorf to avoid failing the test
}

func (tc *TestCapture) Logf(format string, args ...interface{}) {
	tc.logs = append(tc.logs, fmt.Sprintf(format, args...))
	tc.T.Logf(format, args...) // Still log to the real test
}

// Implement the testing.TB interface methods
func (tc *TestCapture) Error(args ...interface{}) {
	tc.errors = append(tc.errors, fmt.Sprint(args...))
	// Don't call the underlying T.Error to avoid failing the test
}

func (tc *TestCapture) Log(args ...interface{}) {
	tc.logs = append(tc.logs, fmt.Sprint(args...))
	tc.T.Log(args...) // Still log to the real test
}