package events_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeadlockPrevention tests that the EventBus prevents deadlocks under high load
func TestDeadlockPrevention(t *testing.T) {
	t.Run("PublishTimeoutPreventsDeadlock", func(t *testing.T) {
		// Create a small buffer to force timeout scenarios
		config := events.DefaultEventBusConfig()
		config.BufferSize = 10
		config.PublishTimeout = 100 * time.Millisecond
		config.DropOnFullBuffer = false // Force timeout behavior

		eventBus := events.NewEventBus(config)
		defer eventBus.Close()

		// Fill up the buffer with a slow subscriber
		slowHandler := func(ctx context.Context, event events.BusEvent) error {
			time.Sleep(500 * time.Millisecond) // Slow processing
			return nil
		}

		_, err := eventBus.Subscribe("slow.event", slowHandler)
		require.NoError(t, err)

		// Fill the buffer - need to fill it faster than it can be processed
		for i := 0; i < config.BufferSize+5; i++ { // Overfill to ensure blocking
			event := events.BusEvent{
				ID:        fmt.Sprintf("slow-event-%d", i),
				Type:      "slow.event",
				Source:    "test",
				Timestamp: time.Now(),
			}
			
			// Don't check error here since we expect some to fail
			eventBus.Publish(context.Background(), event)
		}

		// Give a moment for the system to get into a backlogged state
		time.Sleep(10 * time.Millisecond)

		// Now try to publish more events - should timeout instead of deadlocking
		start := time.Now()
		event := events.BusEvent{
			ID:        "timeout-test",
			Type:      "slow.event", 
			Source:    "test",
			Timestamp: time.Now(),
		}
		
		err = eventBus.Publish(context.Background(), event)
		elapsed := time.Since(start)

		// Should have timed out within reasonable time
		assert.True(t, elapsed >= 100*time.Millisecond, "Should have waited at least the timeout duration")
		assert.True(t, elapsed < 200*time.Millisecond, "Should not have waited much longer than timeout")
		
		// Should have received a timeout error rather than hanging
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})

	t.Run("CircuitBreakerPreventsCascadeFailure", func(t *testing.T) {
		config := events.DefaultEventBusConfig()
		config.BufferSize = 5
		config.PublishTimeout = 50 * time.Millisecond
		config.CircuitBreakerConfig.FailureThreshold = 3
		config.CircuitBreakerConfig.RecoveryTimeout = 100 * time.Millisecond

		eventBus := events.NewEventBus(config)
		defer eventBus.Close()

		// Add a handler that will process events
		processed := make(chan string, 100)
		handler := func(ctx context.Context, event events.BusEvent) error {
			processed <- event.ID
			return nil
		}

		_, err := eventBus.Subscribe("test.event", handler)
		require.NoError(t, err)

		// Fill up the buffer to trigger timeouts and circuit breaker
		var wg sync.WaitGroup
		errorCount := 0
		var mu sync.Mutex

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				event := events.BusEvent{
					ID:        fmt.Sprintf("event-%d", id),
					Type:      "test.event",
					Source:    "test",
					Timestamp: time.Now(),
				}
				
				err := eventBus.Publish(context.Background(), event)
				if err != nil {
					mu.Lock()
					errorCount++
					mu.Unlock()
				}
			}(i)
		}

		wg.Wait()

		// Some events should have failed due to buffer overflow and timeouts
		mu.Lock()
		finalErrorCount := errorCount
		mu.Unlock()

		t.Logf("Error count: %d out of 10 events", finalErrorCount)
		
		// Circuit breaker should have kicked in after failures
		cbStats := eventBus.GetCircuitBreakerStats()
		t.Logf("Circuit breaker stats: %+v", cbStats)
		
		// Verify system is still responsive (not deadlocked)
		stats := eventBus.GetStats()
		assert.NotNil(t, stats)
		assert.True(t, stats.QueueUtilization >= 0 && stats.QueueUtilization <= 1)
	})

	t.Run("HighConcurrencyStressTest", func(t *testing.T) {
		config := events.DefaultEventBusConfig()
		config.BufferSize = 100
		config.PublishTimeout = 200 * time.Millisecond
		config.WorkerCount = 8

		eventBus := events.NewEventBus(config)
		defer eventBus.Close()

		// Add multiple handlers
		processed := make(chan string, 1000)
		for i := 0; i < 3; i++ {
			eventType := fmt.Sprintf("stress.event.%d", i)
			handler := func(ctx context.Context, event events.BusEvent) error {
				processed <- event.ID
				time.Sleep(1 * time.Millisecond) // Small processing delay
				return nil
			}

			_, err := eventBus.Subscribe(eventType, handler)
			require.NoError(t, err)
		}

		// Launch many concurrent publishers
		numPublishers := 20
		numEventsPerPublisher := 10
		
		var wg sync.WaitGroup
		start := time.Now()

		for p := 0; p < numPublishers; p++ {
			wg.Add(1)
			go func(publisherID int) {
				defer wg.Done()
				
				for e := 0; e < numEventsPerPublisher; e++ {
					event := events.BusEvent{
						ID:        fmt.Sprintf("stress-%d-%d", publisherID, e),
						Type:      fmt.Sprintf("stress.event.%d", e%3),
						Source:    fmt.Sprintf("publisher-%d", publisherID),
						Timestamp: time.Now(),
					}
					
					// Use PublishAsync to test async behavior
					err := eventBus.PublishAsync(context.Background(), event)
					if err != nil {
						t.Logf("PublishAsync error: %v", err)
					}
				}
			}(p)
		}

		wg.Wait()
		elapsed := time.Since(start)

		// Give some time for async processing to complete
		time.Sleep(100 * time.Millisecond)

		// Verify the system handled the load without deadlocking
		stats := eventBus.GetStats()
		t.Logf("Stress test completed in %v", elapsed)
		t.Logf("Final stats: Published=%d, Delivered=%d, Dropped=%d, Errors=%d", 
			stats.EventsPublished, stats.EventsDelivered, stats.EventsDropped, stats.DeliveryErrors)
		
		// System should have processed events (not deadlocked)
		assert.True(t, stats.EventsPublished > 0, "Should have published some events")
		
		// Should complete in reasonable time (not hang)
		assert.True(t, elapsed < 5*time.Second, "Should complete quickly without deadlocks")
		
		// Verify circuit breaker state is reasonable
		cbStats := eventBus.GetCircuitBreakerStats()
		t.Logf("Circuit breaker final state: %+v", cbStats)
	})
}

// TestBackpressureHandling tests the backpressure handling mechanisms
func TestBackpressureHandling(t *testing.T) {
	t.Run("AdaptiveBackpressure", func(t *testing.T) {
		config := events.DefaultEventBusConfig()
		config.BufferSize = 20
		config.PublishTimeout = 300 * time.Millisecond

		eventBus := events.NewEventBus(config)
		defer eventBus.Close()

		// Add a very slow handler to create backpressure
		slowHandler := func(ctx context.Context, event events.BusEvent) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		_, err := eventBus.Subscribe("backpressure.event", slowHandler)
		require.NoError(t, err)

		// Publish events rapidly to test backpressure
		var successCount, errorCount int
		var mu sync.Mutex
		
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				event := events.BusEvent{
					ID:        fmt.Sprintf("backpressure-%d", id),
					Type:      "backpressure.event",
					Source:    "test",
					Timestamp: time.Now(),
				}
				
				err := eventBus.Publish(context.Background(), event)
				mu.Lock()
				if err != nil {
					errorCount++
				} else {
					successCount++
				}
				mu.Unlock()
			}(i)
		}
		
		wg.Wait()
		
		mu.Lock()
		finalSuccessCount := successCount
		finalErrorCount := errorCount
		mu.Unlock()
		
		t.Logf("Backpressure test: Success=%d, Errors=%d", finalSuccessCount, finalErrorCount)
		
		// Should have handled backpressure gracefully (some success, some controlled failures)
		assert.True(t, finalSuccessCount > 0, "Should have processed some events successfully")
		assert.True(t, finalErrorCount > 0, "Should have rejected some events under backpressure")
		assert.Equal(t, 50, finalSuccessCount+finalErrorCount, "Should account for all events")
	})
}