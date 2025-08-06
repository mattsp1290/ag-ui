package testing

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestComprehensiveGoroutineLifecycle validates the entire goroutine management system
func TestComprehensiveGoroutineLifecycle(t *testing.T) {
	// This test verifies that our goroutine lifecycle management works correctly
	// and doesn't leak goroutines under various scenarios
	
	t.Run("BasicLeakDetection", func(t *testing.T) {
		// Test basic functionality of the enhanced leak detector
		testBasicLeakDetection(t)
	})
	
	t.Run("LifecycleManagerBasics", func(t *testing.T) {
		// Test basic lifecycle manager operations
		testLifecycleManagerBasics(t)
	})
	
	t.Run("WorkerPoolPattern", func(t *testing.T) {
		// Test worker pool implementation
		testWorkerPoolPattern(t)
	})
	
	t.Run("TickerPattern", func(t *testing.T) {
		// Test ticker-based goroutine pattern
		testTickerPattern(t)
	})
	
	t.Run("BatchProcessingPattern", func(t *testing.T) {
		// Test batch processing pattern
		testBatchProcessingPattern(t)
	})
	
	t.Run("ErrorHandlingAndRecovery", func(t *testing.T) {
		// Test panic recovery and error handling
		testErrorHandlingAndRecovery(t)
	})
	
	t.Run("ConcurrentOperations", func(t *testing.T) {
		// Test concurrent operations don't cause leaks
		testConcurrentOperations(t)
	})
	
	t.Run("StressTest", func(t *testing.T) {
		// Stress test with many goroutines
		testStressScenario(t)
	})
	
	t.Run("RealWorldScenario", func(t *testing.T) {
		// Test realistic application scenario
		testRealWorldScenario(t)
	})
}

func testBasicLeakDetection(t *testing.T) {
	t.Log("Testing basic leak detection capabilities")
	
	// Test 1: No leaks
	VerifyNoGoroutineLeaks(t, func() {
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
	
	// Test 2: Leak detection with custom settings
	VerifyNoGoroutineLeaksWithOptions(t, func(detector *EnhancedGoroutineLeakDetector) {
		detector.WithTolerance(1).WithMaxWaitTime(2 * time.Second)
	}, func() {
		// This should pass with tolerance
		done := make(chan struct{})
		defer close(done)
		
		// Temporary goroutine that finishes quickly
		go func() {
			select {
			case <-done:
			case <-time.After(100 * time.Millisecond):
			}
		}()
		
		time.Sleep(50 * time.Millisecond)
	})
}

func testLifecycleManagerBasics(t *testing.T) {
	t.Log("Testing lifecycle manager basic operations")
	
	VerifyNoGoroutineLeaks(t, func() {
		manager := NewGoroutineLifecycleManager("test-basic-ops")
		
		// Test starting and stopping goroutines
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
		
		// Let it run briefly
		time.Sleep(100 * time.Millisecond)
		
		// Should have active goroutines
		if manager.GetActiveCount() == 0 {
			t.Error("Expected active goroutines")
		}
		
		// Should have incremented counter
		if atomic.LoadInt64(&counter) == 0 {
			t.Error("Counter should have been incremented")
		}
		
		// Graceful shutdown
		if err := manager.ShutdownWithTimeout(); err != nil {
			t.Fatalf("Failed to shutdown: %v", err)
		}
		
		// Should have no active goroutines
		if manager.GetActiveCount() != 0 {
			t.Errorf("Expected 0 active goroutines, got %d", manager.GetActiveCount())
		}
	})
}

func testWorkerPoolPattern(t *testing.T) {
	t.Log("Testing worker pool pattern")
	
	VerifyNoGoroutineLeaksWithOptions(t, func(detector *EnhancedGoroutineLeakDetector) {
		detector.WithTolerance(2).WithMaxWaitTime(5 * time.Second)
	}, func() {
		manager := NewGoroutineLifecycleManager("worker-pool")
		defer manager.MustShutdown()
		
		// Create work channel
		workCh := make(chan interface{}, 100)
		var processed int64
		
		// Start worker pool
		numWorkers := 3
		for i := 0; i < numWorkers; i++ {
			workerID := fmt.Sprintf("worker-%d", i)
			err := manager.GoWorker(workerID, workCh, func(ctx context.Context, work interface{}) {
				// Simulate work
				time.Sleep(5 * time.Millisecond)
				atomic.AddInt64(&processed, 1)
			})
			
			if err != nil {
				t.Fatalf("Failed to start worker %s: %v", workerID, err)
			}
		}
		
		// Send work items
		workItems := 20
		for i := 0; i < workItems; i++ {
			workCh <- i
		}
		
		// Close channel to signal completion
		close(workCh)
		
		// Wait for processing
		maxWait := time.Now().Add(2 * time.Second)
		for atomic.LoadInt64(&processed) < int64(workItems) && time.Now().Before(maxWait) {
			time.Sleep(10 * time.Millisecond)
		}
		
		processedCount := atomic.LoadInt64(&processed)
		if processedCount != int64(workItems) {
			t.Errorf("Expected %d processed items, got %d", workItems, processedCount)
		}
		
		// Workers should exit after channel close
		time.Sleep(100 * time.Millisecond)
		if manager.GetActiveCount() != 0 {
			t.Errorf("Expected 0 active goroutines after channel close, got %d", manager.GetActiveCount())
		}
	})
}

func testTickerPattern(t *testing.T) {
	t.Log("Testing ticker pattern")
	
	VerifyNoGoroutineLeaks(t, func() {
		manager := NewGoroutineLifecycleManager("ticker-test")
		defer manager.MustShutdown()
		
		var ticks int64
		err := manager.GoTicker("ticker", 20*time.Millisecond, func(ctx context.Context) {
			atomic.AddInt64(&ticks, 1)
		})
		
		if err != nil {
			t.Fatalf("Failed to start ticker: %v", err)
		}
		
		// Let it tick several times
		time.Sleep(150 * time.Millisecond)
		
		tickCount := atomic.LoadInt64(&ticks)
		if tickCount < 5 {
			t.Errorf("Expected at least 5 ticks, got %d", tickCount)
		}
	})
}

func testBatchProcessingPattern(t *testing.T) {
	t.Log("Testing batch processing pattern")
	
	VerifyNoGoroutineLeaks(t, func() {
		manager := NewGoroutineLifecycleManager("batch-test")
		defer manager.MustShutdown()
		
		inputCh := make(chan interface{}, 100)
		var batches, items int64
		
		err := manager.GoBatch("batcher", 5, 50*time.Millisecond, inputCh,
			func(ctx context.Context, batch []interface{}) {
				atomic.AddInt64(&batches, 1)
				atomic.AddInt64(&items, int64(len(batch)))
			})
		
		if err != nil {
			t.Fatalf("Failed to start batch processor: %v", err)
		}
		
		// Send items
		totalItems := 12
		for i := 0; i < totalItems; i++ {
			inputCh <- i
		}
		
		// Wait for processing
		time.Sleep(200 * time.Millisecond)
		
		batchCount := atomic.LoadInt64(&batches)
		itemCount := atomic.LoadInt64(&items)
		
		if batchCount < 2 {
			t.Errorf("Expected at least 2 batches, got %d", batchCount)
		}
		if itemCount != int64(totalItems) {
			t.Errorf("Expected %d items processed, got %d", totalItems, itemCount)
		}
	})
}

func testErrorHandlingAndRecovery(t *testing.T) {
	t.Log("Testing error handling and panic recovery")
	
	VerifyNoGoroutineLeaks(t, func() {
		manager := NewGoroutineLifecycleManager("error-test")
		defer manager.MustShutdown()
		
		var panicRecovered bool
		err := manager.GoWithRecovery("panic-worker", func(ctx context.Context) {
			panic("test panic")
		}, func(r interface{}) {
			if r == "test panic" {
				panicRecovered = true
			}
		})
		
		if err != nil {
			t.Fatalf("Failed to start panic worker: %v", err)
		}
		
		// Wait for panic and recovery
		time.Sleep(100 * time.Millisecond)
		
		if !panicRecovered {
			t.Error("Panic was not properly recovered")
		}
		
		// Goroutine should have cleaned up
		if manager.GetActiveCount() != 0 {
			t.Errorf("Expected 0 active goroutines after panic, got %d", manager.GetActiveCount())
		}
	})
}

func testConcurrentOperations(t *testing.T) {
	t.Log("Testing concurrent operations")
	
	VerifyNoGoroutineLeaksWithOptions(t, func(detector *EnhancedGoroutineLeakDetector) {
		detector.WithTolerance(5).WithMaxWaitTime(10 * time.Second)
	}, func() {
		manager := NewGoroutineLifecycleManager("concurrent-test")
		defer manager.MustShutdown()
		
		var wg sync.WaitGroup
		numConcurrent := 10
		
		// Start multiple concurrent operations
		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Each goroutine starts its own managed goroutine
				workerID := fmt.Sprintf("concurrent-worker-%d", id)
				manager.Go(workerID, func(ctx context.Context) {
					// Simulate longer-running work so goroutines are still active when we check
					for j := 0; j < 200; j++ {
						select {
						case <-ctx.Done():
							return
						default:
							time.Sleep(time.Millisecond)
						}
					}
				})
			}(i)
		}
		
		wg.Wait()
		
		// Check immediately after starting - workers should be active
		// Small sleep to ensure goroutines have started
		time.Sleep(50 * time.Millisecond)
		
		// Should have started workers
		if manager.GetActiveCount() == 0 {
			t.Error("Expected some active goroutines")
		}
	})
}

func testStressScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	
	t.Log("Testing stress scenario with many goroutines")
	
	VerifyNoGoroutineLeaksWithOptions(t, func(detector *EnhancedGoroutineLeakDetector) {
		detector.WithTolerance(10).WithMaxWaitTime(15 * time.Second)
	}, func() {
		manager := NewGoroutineLifecycleManager("stress-test")
		defer manager.MustShutdown()
		
		numGoroutines := 50
		var completedWork int64
		
		// Start many short-lived goroutines
		for i := 0; i < numGoroutines; i++ {
			workerID := fmt.Sprintf("stress-worker-%d", i)
			manager.Go(workerID, func(ctx context.Context) {
				defer func() {
					atomic.AddInt64(&completedWork, 1)
				}()
				
				// Simulate work with random duration
				workTime := time.Duration(i%10+1) * time.Millisecond
				select {
				case <-ctx.Done():
					return
				case <-time.After(workTime):
					// Work completed
				}
			})
		}
		
		// Wait for work to complete
		maxWait := time.Now().Add(5 * time.Second)
		for atomic.LoadInt64(&completedWork) < int64(numGoroutines) && time.Now().Before(maxWait) {
			time.Sleep(10 * time.Millisecond)
		}
		
		completed := atomic.LoadInt64(&completedWork)
		if completed != int64(numGoroutines) {
			t.Errorf("Expected %d completed work items, got %d", numGoroutines, completed)
		}
	})
}

func testRealWorldScenario(t *testing.T) {
	t.Log("Testing realistic application scenario")
	
	VerifyNoGoroutineLeaksWithOptions(t, func(detector *EnhancedGoroutineLeakDetector) {
		detector.WithTolerance(5).WithMaxWaitTime(10 * time.Second)
	}, func() {
		// Simulate a realistic service with multiple components
		service := NewTestService()
		defer service.Stop()
		
		if err := service.Start(); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}
		
		// Simulate load
		service.SimulateLoad(100)
		
		// Let it run
		time.Sleep(500 * time.Millisecond)
		
		// Check health
		if !service.IsHealthy() {
			t.Error("Service should be healthy")
		}
		
		// Check metrics
		metrics := service.GetMetrics()
		if metrics.ProcessedItems == 0 {
			t.Error("Service should have processed some items")
		}
		if metrics.ActiveWorkers == 0 {
			t.Error("Service should have active workers")
		}
	})
}

// TestService simulates a realistic service with multiple goroutine patterns
type TestService struct {
	manager     *GoroutineLifecycleManager
	inputCh     chan interface{}
	started     bool
	mu          sync.RWMutex
	metrics     ServiceMetrics
}

type ServiceMetrics struct {
	ProcessedItems int64
	ActiveWorkers  int64
	Uptime         time.Duration
	StartTime      time.Time
}

func NewTestService() *TestService {
	return &TestService{
		manager: NewGoroutineLifecycleManager("test-service"),
		inputCh: make(chan interface{}, 1000),
	}
}

func (s *TestService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.started {
		return fmt.Errorf("service already started")
	}
	
	s.metrics.StartTime = time.Now()
	
	// Start worker pool
	for i := 0; i < 3; i++ {
		workerID := fmt.Sprintf("worker-%d", i)
		if err := s.manager.GoWorker(workerID, s.inputCh, s.processItem); err != nil {
			return fmt.Errorf("failed to start worker: %w", err)
		}
		atomic.AddInt64(&s.metrics.ActiveWorkers, 1)
	}
	
	// Start metrics collector
	if err := s.manager.GoTicker("metrics", 100*time.Millisecond, s.collectMetrics); err != nil {
		return fmt.Errorf("failed to start metrics collector: %w", err)
	}
	
	// Start health checker
	if err := s.manager.GoTicker("health", 50*time.Millisecond, s.healthCheck); err != nil {
		return fmt.Errorf("failed to start health checker: %w", err)
	}
	
	s.started = true
	return nil
}

func (s *TestService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.started {
		return nil
	}
	
	close(s.inputCh)
	s.started = false
	
	return s.manager.ShutdownWithTimeout()
}

func (s *TestService) processItem(ctx context.Context, item interface{}) {
	// Simulate processing
	time.Sleep(time.Millisecond)
	atomic.AddInt64(&s.metrics.ProcessedItems, 1)
}

func (s *TestService) collectMetrics(ctx context.Context) {
	s.mu.Lock()
	s.metrics.Uptime = time.Since(s.metrics.StartTime)
	s.mu.Unlock()
}

func (s *TestService) healthCheck(ctx context.Context) {
	// Simulate health check
	runtime.GC() // Force GC for testing
}

func (s *TestService) SimulateLoad(items int) {
	for i := 0; i < items; i++ {
		select {
		case s.inputCh <- i:
		default:
			// Channel full, skip
		}
	}
}

func (s *TestService) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.started && s.manager.GetActiveCount() > 0
}

func (s *TestService) GetMetrics() ServiceMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	metrics := s.metrics
	metrics.ActiveWorkers = s.manager.GetActiveCount()
	return metrics
}

// Benchmark goroutine lifecycle performance
func BenchmarkGoroutineLifecycleManager(b *testing.B) {
	manager := NewGoroutineLifecycleManager("benchmark")
	defer manager.MustShutdown()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		workerID := fmt.Sprintf("bench-worker-%d", i)
		manager.Go(workerID, func(ctx context.Context) {
			// Minimal work
			select {
			case <-ctx.Done():
				return
			default:
				runtime.Gosched()
			}
		})
	}
}

// Benchmark leak detection performance
func BenchmarkLeakDetection(b *testing.B) {
	for i := 0; i < b.N; i++ {
		detector := NewEnhancedGoroutineLeakDetector(b)
		detector.captureBaseline()
		
		// Simulate some goroutines
		done := make(chan struct{})
		for j := 0; j < 10; j++ {
			go func() {
				<-done
			}()
		}
		
		snapshot := detector.captureSnapshot()
		detector.analyzeLeak(snapshot)
		
		close(done)
		time.Sleep(time.Millisecond) // Let goroutines exit
	}
}

