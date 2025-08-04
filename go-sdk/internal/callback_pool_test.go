package internal

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewCallbackPool(t *testing.T) {
	// Test with explicit worker count
	pool := NewCallbackPool(4)
	if pool.WorkerCount() != 4 {
		t.Errorf("Expected 4 workers, got %d", pool.WorkerCount())
	}
	if pool.QueueCapacity() != 8 { // workers * 2
		t.Errorf("Expected queue capacity of 8, got %d", pool.QueueCapacity())
	}
	pool.Stop()

	// Test with zero workers (should default to NumCPU)
	pool = NewCallbackPool(0)
	expectedWorkers := runtime.NumCPU()
	if pool.WorkerCount() != expectedWorkers {
		t.Errorf("Expected %d workers, got %d", expectedWorkers, pool.WorkerCount())
	}
	pool.Stop()
}

func TestCallbackPoolSubmit(t *testing.T) {
	pool := NewCallbackPool(2)
	defer pool.Stop()

	var counter int64
	var wg sync.WaitGroup

	// Submit multiple jobs
	numJobs := 10
	wg.Add(numJobs)
	
	for i := 0; i < numJobs; i++ {
		pool.Submit(func() {
			defer wg.Done()
			atomic.AddInt64(&counter, 1)
		})
	}

	// Wait for all jobs to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for jobs to complete")
	}

	if atomic.LoadInt64(&counter) != int64(numJobs) {
		t.Errorf("Expected %d jobs executed, got %d", numJobs, counter)
	}
}

func TestCallbackPoolAutoStart(t *testing.T) {
	pool := NewCallbackPool(1)
	defer pool.Stop()

	if pool.IsStarted() {
		t.Error("Pool should not be started initially")
	}

	var executed bool
	var wg sync.WaitGroup
	wg.Add(1)

	pool.Submit(func() {
		defer wg.Done()
		executed = true
	})

	if !pool.IsStarted() {
		t.Error("Pool should be started after first Submit")
	}

	wg.Wait()
	if !executed {
		t.Error("Job should have been executed")
	}
}

func TestCallbackPoolFallbackOnFullQueue(t *testing.T) {
	// Create a pool with small capacity
	pool := NewCallbackPool(1)
	defer pool.Stop()

	var counter int64
	var wg sync.WaitGroup

	// Fill up the queue and add more jobs to test fallback
	numJobs := 20
	wg.Add(numJobs)

	for i := 0; i < numJobs; i++ {
		pool.Submit(func() {
			defer wg.Done()
			atomic.AddInt64(&counter, 1)
			// Add small delay to simulate work
			time.Sleep(1 * time.Millisecond)
		})
	}

	// Wait for all jobs to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for jobs to complete")
	}

	if atomic.LoadInt64(&counter) != int64(numJobs) {
		t.Errorf("Expected %d jobs executed, got %d", numJobs, counter)
	}
}

func TestCallbackPoolPanicRecovery(t *testing.T) {
	pool := NewCallbackPool(1)
	defer pool.Stop()

	var wg sync.WaitGroup
	var normalJobExecuted bool

	// Submit a job that panics
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		panic("test panic")
	})

	// Submit a normal job after the panic
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		normalJobExecuted = true
	})

	wg.Wait()

	if !normalJobExecuted {
		t.Error("Normal job should have executed even after panic")
	}
}

func TestCallbackPoolNilFunction(t *testing.T) {
	pool := NewCallbackPool(1)
	defer pool.Stop()

	// Should not panic with nil function
	pool.Submit(nil)
	
	// Verify pool still works
	var executed bool
	var wg sync.WaitGroup
	wg.Add(1)

	pool.Submit(func() {
		defer wg.Done()
		executed = true
	})

	wg.Wait()
	if !executed {
		t.Error("Job should have been executed after nil submission")
	}
}

func TestCallbackPoolGracefulShutdown(t *testing.T) {
	pool := NewCallbackPool(2)
	
	var counter int64
	var wg sync.WaitGroup

	// Submit several jobs
	numJobs := 5
	wg.Add(numJobs)
	
	for i := 0; i < numJobs; i++ {
		pool.Submit(func() {
			defer wg.Done()
			atomic.AddInt64(&counter, 1)
			time.Sleep(10 * time.Millisecond)
		})
	}

	// Wait for jobs to complete, then stop the pool
	wg.Wait()
	pool.Stop()

	if atomic.LoadInt64(&counter) != int64(numJobs) {
		t.Errorf("Expected %d jobs completed during shutdown, got %d", numJobs, counter)
	}
}

func TestCallbackPoolConcurrentSubmit(t *testing.T) {
	pool := NewCallbackPool(4)
	defer pool.Stop()

	var counter int64
	var wg sync.WaitGroup

	// Launch multiple goroutines submitting jobs concurrently
	numSubmitters := 10
	jobsPerSubmitter := 10
	totalJobs := numSubmitters * jobsPerSubmitter

	wg.Add(totalJobs)

	for i := 0; i < numSubmitters; i++ {
		go func() {
			for j := 0; j < jobsPerSubmitter; j++ {
				pool.Submit(func() {
					defer wg.Done()
					atomic.AddInt64(&counter, 1)
				})
			}
		}()
	}

	// Wait for all jobs to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for concurrent jobs to complete")
	}

	if atomic.LoadInt64(&counter) != int64(totalJobs) {
		t.Errorf("Expected %d jobs executed, got %d", totalJobs, counter)
	}
}

// Benchmark to compare goroutine creation vs pool usage
func BenchmarkDirectGoroutines(b *testing.B) {
	var wg sync.WaitGroup
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Simulate small callback work
			_ = 1 + 1
		}()
	}
	wg.Wait()
}

func BenchmarkCallbackPool(b *testing.B) {
	pool := NewCallbackPool(runtime.NumCPU())
	defer pool.Stop()
	
	var wg sync.WaitGroup
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			// Simulate small callback work
			_ = 1 + 1
		})
	}
	wg.Wait()
}

// More realistic benchmark with concurrent callback execution
func BenchmarkDirectGoroutinesHighLoad(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		// Simulate 100 concurrent callbacks (like SSE events)
		for j := 0; j < 100; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Simulate some callback work
				for k := 0; k < 100; k++ {
					_ = k * k
				}
			}()
		}
		wg.Wait()
	}
}

func BenchmarkCallbackPoolHighLoad(b *testing.B) {
	pool := NewCallbackPool(runtime.NumCPU())
	defer pool.Stop()
	
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		// Simulate 100 concurrent callbacks (like SSE events)
		for j := 0; j < 100; j++ {
			wg.Add(1)
			pool.Submit(func() {
				defer wg.Done()
				// Simulate some callback work
				for k := 0; k < 100; k++ {
					_ = k * k
				}
			})
		}
		wg.Wait()
	}
}