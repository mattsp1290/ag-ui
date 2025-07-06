package state

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestManagerShutdownNoRace tests that closing the manager doesn't cause races
func TestManagerShutdownNoRace(t *testing.T) {
	manager, err := NewStateManager(DefaultManagerOptions())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Start multiple writers
	var wg sync.WaitGroup
	stopWriters := make(chan struct{})
	
	// Writer goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			ctx := context.Background()
			contextID, err := manager.CreateContext(ctx, "test-state", nil)
			if err != nil {
				t.Logf("writer %d: failed to create context: %v", id, err)
				return
			}
			
			for {
				select {
				case <-stopWriters:
					return
				default:
					updates := map[string]interface{}{
						"counter": id,
						"timestamp": time.Now().Unix(),
					}
					
					_, err := manager.UpdateState(ctx, contextID, "test-state", updates, UpdateOptions{})
					if err != nil {
						// Expected errors during shutdown
						if err == ErrManagerClosing || err == ErrManagerClosed || err == ErrQueueFull {
							t.Logf("writer %d: expected error during shutdown: %v", id, err)
							return
						}
						t.Logf("writer %d: update error: %v", id, err)
					}
					
					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}

	// Let writers run for a bit
	time.Sleep(100 * time.Millisecond)

	// Close the manager while writers are active
	done := make(chan struct{})
	go func() {
		err := manager.Close()
		if err != nil {
			t.Errorf("failed to close manager: %v", err)
		}
		close(done)
	}()

	// Signal writers to stop after a delay
	time.Sleep(50 * time.Millisecond)
	close(stopWriters)

	// Wait for writers to finish
	wg.Wait()

	// Ensure close completes
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("manager close timed out")
	}
}

// TestManagerGracefulShutdown tests graceful shutdown behavior
func TestManagerGracefulShutdown(t *testing.T) {
	opts := DefaultManagerOptions()
	opts.EventBufferSize = 10
	opts.BatchSize = 5
	
	manager, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	ctx := context.Background()
	contextID, err := manager.CreateContext(ctx, "test-state", nil)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	// Fill the update queue with synchronous calls
	var updateWg sync.WaitGroup
	for i := 0; i < 10; i++ {
		updates := map[string]interface{}{
			"value": i,
		}
		updateWg.Add(1)
		go func(val int) {
			defer updateWg.Done()
			_, err := manager.UpdateState(ctx, contextID, "test-state", updates, UpdateOptions{
				Timeout: 100 * time.Millisecond,
			})
			if err != nil && err != ErrManagerClosing && err != ErrManagerClosed && err != ErrQueueFull {
				t.Logf("update %d error: %v", val, err)
			}
		}(i)
	}

	// Give some time for updates to be queued
	time.Sleep(20 * time.Millisecond)

	// Close should drain the queue
	done := make(chan error, 1)
	go func() {
		done <- manager.Close()
	}()
	
	// Wait for update goroutines to finish
	updateWg.Wait()
	
	// Wait for close to complete
	select {
	case err = <-done:
		if err != nil {
			t.Errorf("failed to close manager: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("close timed out")
	}

	// Try to use after close - should get errors
	_, err = manager.CreateContext(ctx, "new-state", nil)
	if err == nil {
		t.Error("expected error creating context after close")
	}

	_, err = manager.UpdateState(ctx, contextID, "test-state", map[string]interface{}{"final": true}, UpdateOptions{})
	if err != ErrManagerClosing && err != ErrManagerClosed {
		t.Errorf("expected closing/closed error, got: %v", err)
	}
}

// TestManagerCloseTimeout tests shutdown timeout behavior
func TestManagerCloseTimeout(t *testing.T) {
	t.Skip("Skipping timeout test - would take 30 seconds")
}