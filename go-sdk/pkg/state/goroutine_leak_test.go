package state

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
	
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TestNoGoroutineLeaks verifies that all goroutines are properly cleaned up
func TestNoGoroutineLeaks(t *testing.T) {
	// Get baseline goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Run a series of operations that previously leaked goroutines
	t.Run("MonitoringSystem", func(t *testing.T) {
		config := MonitoringConfig{
			MetricsInterval:     100 * time.Millisecond,
			HealthCheckInterval: 100 * time.Millisecond,
			AlertNotifiers:      []AlertNotifier{}, // No alert notifiers to avoid nil pointer
			LogFormat:           "console",
		}
		
		ms, err := NewMonitoringSystem(config)
		if err != nil {
			t.Fatalf("Failed to create monitoring system: %v", err)
		}
		
		// Add a health check
		ms.RegisterHealthCheck(&testHealthCheck{})
		
		// Trigger some alerts by recording high latency
		ms.RecordStateOperation("test-op", 10*time.Second, nil)
		
		// Let it run briefly
		time.Sleep(200 * time.Millisecond)
		
		// Shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := ms.Shutdown(ctx); err != nil {
			t.Fatalf("Failed to shutdown monitoring system: %v", err)
		}
	})

	t.Run("StateStore", func(t *testing.T) {
		store := NewStateStore()
		
		// Subscribe to changes
		unsubscribe := store.Subscribe("/test", func(change StateChange) {
			// Handler
		})
		defer unsubscribe()
		
		// Make some changes to trigger notifications
		store.Set("/test/value", "data")
		store.Set("/test/value2", "data2")
		
		// Let notifications run
		time.Sleep(100 * time.Millisecond)
		
		// Close the store
		if err := store.Close(); err != nil {
			t.Fatalf("Failed to close store: %v", err)
		}
	})

	t.Run("StateEventStream", func(t *testing.T) {
		store := NewStateStore()
		defer store.Close()
		
		generator := NewStateEventGenerator(store)
		stream := NewStateEventStream(store, generator, WithStreamInterval(50*time.Millisecond))
		
		// Add handler
		stream.Subscribe(func(event events.Event) error {
			// Process event
			return nil
		})
		
		// Start stream
		stream.Start()
		
		// Make some changes
		store.Set("/test", "value")
		
		// Let it run
		time.Sleep(100 * time.Millisecond)
		
		// Stop stream
		stream.Stop()
	})

	t.Run("AuditManager", func(t *testing.T) {
		logger := &NoOpAuditLogger{}
		am := NewAuditManager(logger)
		
		// Log some events asynchronously
		ctx := context.Background()
		am.LogStateUpdate(ctx, "ctx1", "state1", "user1", "old", "new", AuditResultSuccess, nil)
		am.LogError(ctx, AuditActionStateUpdate, fmt.Errorf("test error"), map[string]interface{}{"test": "data"})
		
		// Let async operations run
		time.Sleep(100 * time.Millisecond)
		
		// Close audit manager
		if err := am.Close(); err != nil {
			t.Fatalf("Failed to close audit manager: %v", err)
		}
	})

	// Wait for goroutines to clean up
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	
	// Check final goroutine count
	finalCount := runtime.NumGoroutine()
	leaked := finalCount - baseline
	
	// Allow for some variance (test framework may create goroutines)
	if leaked > 5 {
		t.Errorf("Goroutine leak detected: baseline=%d, final=%d, leaked=%d", baseline, finalCount, leaked)
		
		// Print stack traces for debugging
		buf := make([]byte, 1<<20)
		stackLen := runtime.Stack(buf, true)
		t.Logf("Goroutine stack traces:\n%s", buf[:stackLen])
	}
}

// testHealthCheck is a simple health check for testing
type testHealthCheck struct{}

func (t *testHealthCheck) Name() string {
	return "test"
}

func (t *testHealthCheck) Check(ctx context.Context) error {
	return nil
}