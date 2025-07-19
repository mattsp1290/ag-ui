package transport

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestMinimalRaceCondition is a minimal test to verify race detection works
func TestMinimalRaceCondition(t *testing.T) {
	// This test verifies that our race testing infrastructure works
	// It tests the most basic concurrent operations
	
	manager := NewSimpleManager()
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	
	var wg sync.WaitGroup
	
	// Start operation
	wg.Add(1)
	go func() {
		defer wg.Done()
		manager.Start(ctx)
	}()
	
	// Concurrent send operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			event := &DemoEvent{
				id:        "test-event",
				eventType: "test",
				timestamp: time.Now(),
			}
			
			sendCtx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
			manager.Send(sendCtx, event)
			cancel()
		}(i)
	}
	
	// Wait for operations to complete
	wg.Wait()
	
	// Stop operation
	stopCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	manager.Stop(stopCtx)
	cancel()
}

// TestRaceDetectorEnabled verifies the race detector is working
func TestRaceDetectorEnabled(t *testing.T) {
	// This test intentionally creates a data race when run with -race
	// to verify the race detector is enabled and working
	
	t.Skip("Skipping intentional race test - uncomment to verify race detector")
	
	/*
	var counter int
	var wg sync.WaitGroup
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Intentional race condition
			counter++
		}()
	}
	
	wg.Wait()
	t.Logf("Counter value: %d", counter)
	*/
}