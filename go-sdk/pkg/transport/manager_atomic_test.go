package transport

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAtomicStartStopRaceConditions tests that the atomic operations prevent race conditions
func TestAtomicStartStopRaceConditions(t *testing.T) {
	const numGoroutines = 100
	const numIterations = 10

	for iteration := 0; iteration < numIterations; iteration++ {
		t.Run(fmt.Sprintf("iteration_%d", iteration), func(t *testing.T) {
			manager := NewSimpleManager()
			transport := NewRaceTestTransport()
			manager.SetTransport(transport)

			var wg sync.WaitGroup
			var startSuccesses, startErrors int64
			var stopSuccesses, stopErrors int64

			// Launch multiple goroutines trying to start concurrently
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
					defer cancel()

					err := manager.Start(ctx)
					if err == nil {
						atomic.AddInt64(&startSuccesses, 1)
					} else if errors.Is(err, ErrAlreadyConnected) {
						// This is expected - only one should succeed
						atomic.AddInt64(&startErrors, 1)
					} else {
						t.Errorf("Unexpected start error: %v", err)
					}
				}(i)
			}

			// Launch multiple goroutines trying to stop concurrently
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					// Small delay to let some starts complete
					time.Sleep(10 * time.Millisecond)

					ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
					defer cancel()

					err := manager.Stop(ctx)
					if err == nil {
						atomic.AddInt64(&stopSuccesses, 1)
					} else {
						atomic.AddInt64(&stopErrors, 1)
						t.Errorf("Unexpected stop error: %v", err)
					}
				}(i)
			}

			wg.Wait()

			// Verify exactly one start succeeded
			startSuccessCount := atomic.LoadInt64(&startSuccesses)
			if startSuccessCount != 1 {
				t.Errorf("Expected exactly 1 start success, got %d", startSuccessCount)
			}

			// Verify the rest got ErrAlreadyConnected
			startErrorCount := atomic.LoadInt64(&startErrors)
			if startSuccessCount+startErrorCount != numGoroutines {
				t.Errorf("Start operations don't add up: success=%d, errors=%d, expected=%d",
					startSuccessCount, startErrorCount, numGoroutines)
			}

			t.Logf("Iteration %d: Start successes=%d, errors=%d, Stop successes=%d, errors=%d",
				iteration, startSuccessCount, startErrorCount,
				atomic.LoadInt64(&stopSuccesses), atomic.LoadInt64(&stopErrors))
		})
	}
}

// TestAtomicStateTransitions tests atomic state transitions under failure conditions
func TestAtomicStateTransitions(t *testing.T) {
	tests := []struct {
		name           string
		setupTransport func() *RaceTestTransport
		expectSuccess  bool
	}{
		{
			name: "successful_connection",
			setupTransport: func() *RaceTestTransport {
				transport := NewRaceTestTransport()
				return transport
			},
			expectSuccess: true,
		},
		{
			name: "failed_connection",
			setupTransport: func() *RaceTestTransport {
				transport := NewRaceTestTransport()
				transport.SetShouldFailConnect(true)
				return transport
			},
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewSimpleManager()
			transport := tt.setupTransport()
			manager.SetTransport(transport)

			// Check initial state (running should be 0)
			if atomic.LoadInt32(&manager.running) != 0 {
				t.Errorf("Expected initial running state to be 0")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err := manager.Start(ctx)

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
				// Should be running
				if atomic.LoadInt32(&manager.running) != 1 {
					t.Errorf("Expected running state to be 1 after successful start")
				}
			} else {
				if err == nil {
					t.Errorf("Expected error but got success")
				}
				// Should NOT be running due to fail-safe reset
				if atomic.LoadInt32(&manager.running) != 0 {
					t.Errorf("Expected running state to be 0 after failed start")
				}
			}

			// Clean up
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			manager.Stop(stopCtx)
			stopCancel()
		})
	}
}

// TestConcurrentStateQueries tests concurrent queries of the running state
func TestConcurrentStateQueries(t *testing.T) {
	manager := NewSimpleManager()
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)

	const numReaders = 50
	const numOperations = 1000

	var wg sync.WaitGroup
	var readOperations int64
	done := make(chan struct{})

	// Start the manager
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)

	// Launch concurrent readers of the running state
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				select {
				case <-done:
					return
				default:
					// Read the atomic state
					running := atomic.LoadInt32(&manager.running)
					if running != 0 && running != 1 {
						t.Errorf("Invalid running state: %d", running)
					}
					atomic.AddInt64(&readOperations, 1)

					// Add some CPU work to increase contention
					if j%100 == 0 {
						runtime.Gosched()
					}
				}
			}
		}()
	}

	// Let them run for a bit
	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()

	totalOperations := atomic.LoadInt64(&readOperations)
	t.Logf("Completed %d concurrent state read operations", totalOperations)

	if totalOperations == 0 {
		t.Error("No read operations completed")
	}
}

// TestFailSafeStateManagement tests that failed operations don't leave inconsistent state
func TestFailSafeStateManagement(t *testing.T) {
	const numGoroutines = 20

	var wg sync.WaitGroup
	var inconsistentStates int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			manager := NewSimpleManager()

			// Create a transport that will fail sometimes
			transport := NewRaceTestTransport()
			if id%3 == 0 {
				transport.SetShouldFailConnect(true)
			}
			manager.SetTransport(transport)

			// Try to start
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			err := manager.Start(ctx)
			cancel()

			// Check state consistency
			runningState := atomic.LoadInt32(&manager.running)

			if err == nil {
				// Success: should be running
				if runningState != 1 {
					atomic.AddInt64(&inconsistentStates, 1)
					t.Errorf("Success but not running: state=%d", runningState)
				}
			} else {
				// Failure: should NOT be running (fail-safe)
				if runningState != 0 {
					atomic.AddInt64(&inconsistentStates, 1)
					t.Errorf("Failed but still running: state=%d, error=%v", runningState, err)
				}
			}

			// Clean up
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			manager.Stop(stopCtx)
			stopCancel()
		}(i)
	}

	wg.Wait()

	if atomic.LoadInt64(&inconsistentStates) > 0 {
		t.Errorf("Detected %d inconsistent states", inconsistentStates)
	}
}

// TestAtomicCompareAndSwapBehavior tests the specific CAS behavior
func TestAtomicCompareAndSwapBehavior(t *testing.T) {
	manager := NewSimpleManager()
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)

	// Test initial CAS from 0->1 (should succeed)
	if !atomic.CompareAndSwapInt32(&manager.running, 0, 1) {
		t.Error("Expected CAS 0->1 to succeed initially")
	}

	// Test second CAS from 0->1 (should fail)
	if atomic.CompareAndSwapInt32(&manager.running, 0, 1) {
		t.Error("Expected second CAS 0->1 to fail")
	}

	// Test CAS from 1->0 (should succeed)
	if !atomic.CompareAndSwapInt32(&manager.running, 1, 0) {
		t.Error("Expected CAS 1->0 to succeed")
	}

	// Test second CAS from 1->0 (should fail)
	if atomic.CompareAndSwapInt32(&manager.running, 1, 0) {
		t.Error("Expected second CAS 1->0 to fail")
	}
}

// BenchmarkAtomicOperations benchmarks the atomic operations
func BenchmarkAtomicOperations(b *testing.B) {
	manager := NewSimpleManager()

	b.Run("atomic_load", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			atomic.LoadInt32(&manager.running)
		}
	})

	b.Run("atomic_store", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			atomic.StoreInt32(&manager.running, int32(i%2))
		}
	})

	b.Run("atomic_cas", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			old := i % 2
			new := (i + 1) % 2
			atomic.CompareAndSwapInt32(&manager.running, int32(old), int32(new))
		}
	})

	b.Run("concurrent_cas", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Try to flip between 0 and 1
				for {
					current := atomic.LoadInt32(&manager.running)
					next := (current + 1) % 2
					if atomic.CompareAndSwapInt32(&manager.running, current, next) {
						break
					}
				}
			}
		})
	})
}

// TestMemoryConsistency tests memory consistency of atomic operations
func TestMemoryConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory consistency test in short mode")
	}

	const numGoroutines = 100
	const iterations = 1000

	manager := NewSimpleManager()
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)

	var wg sync.WaitGroup
	var inconsistencies int64

	// Writer goroutines that flip the state
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Flip from 0->1 or 1->0
				for {
					current := atomic.LoadInt32(&manager.running)
					next := (current + 1) % 2
					if atomic.CompareAndSwapInt32(&manager.running, current, next) {
						break
					}
				}
				runtime.Gosched()
			}
		}()
	}

	// Reader goroutines that verify consistency
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				val1 := atomic.LoadInt32(&manager.running)
				val2 := atomic.LoadInt32(&manager.running)

				// Values should be 0 or 1, and two consecutive reads
				// should see a consistent state (unless changed between reads)
				if val1 < 0 || val1 > 1 || val2 < 0 || val2 > 1 {
					atomic.AddInt64(&inconsistencies, 1)
				}
				runtime.Gosched()
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt64(&inconsistencies) > 0 {
		t.Errorf("Detected %d memory consistency issues", inconsistencies)
	}
}

// TestIsRunningMethod tests the IsRunning method for thread safety
func TestIsRunningMethod(t *testing.T) {
	manager := NewSimpleManager()
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)

	// Initially should not be running
	if manager.IsRunning() {
		t.Error("Manager should not be running initially")
	}

	// Start the manager
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Should now be running
	if !manager.IsRunning() {
		t.Error("Manager should be running after start")
	}

	// Test concurrent reads of IsRunning
	const numGoroutines = 100
	var wg sync.WaitGroup
	var inconsistencies int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				running1 := manager.IsRunning()
				running2 := manager.IsRunning()

				// Both reads should be consistent (true in this case)
				if running1 != running2 {
					atomic.AddInt64(&inconsistencies, 1)
				}

				// Values should be boolean-consistent
				if running1 != true && running1 != false {
					atomic.AddInt64(&inconsistencies, 1)
				}
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt64(&inconsistencies) > 0 {
		t.Errorf("Detected %d inconsistencies in IsRunning method", inconsistencies)
	}

	// Stop the manager
	if err := manager.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}

	// Should no longer be running
	if manager.IsRunning() {
		t.Error("Manager should not be running after stop")
	}
}
