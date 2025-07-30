package state

import (
	"context"
	"testing"
	"time"
)

// ExampleTestWithProperCleanup demonstrates how to properly clean up resources
// to prevent "write error: write /dev/stdout: file already closed" errors
func TestExampleWithProperCleanup(t *testing.T) {
	// Skip this example in short mode
	if testing.Short() {
		t.Skip("Skipping example test in short mode")
	}
	
	// Create test cleanup helper
	cleanup := NewTestCleanup(t)
	
	// Create monitoring system
	monitoringConfig := NewTestSafeMonitoringConfig()
	monitoringConfig.EnableHealthChecks = true
	monitoringConfig.HealthCheckInterval = 5 * time.Second
	
	monitoring, err := NewMonitoringSystem(monitoringConfig)
	if err != nil {
		t.Fatalf("Failed to create monitoring: %v", err)
	}
	cleanup.SetMonitoring(monitoring)
	
	// Create state manager with audit logging enabled
	opts := DefaultManagerOptions()
	opts.EnableAudit = true
	
	manager, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	cleanup.SetManager(manager)
	
	// Add any additional cleanup functions
	cleanup.AddCleanup(func() {
		t.Log("Running custom cleanup")
	})
	
	// Perform test operations
	ctx := context.Background()
	contextID, err := manager.CreateContext(ctx, "test", map[string]interface{}{
		"data": "test",
	})
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	
	// Update state (this triggers audit logging)
	_, err = manager.UpdateState(ctx, contextID, "test", map[string]interface{}{
		"data": "updated",
	}, UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update state: %v", err)
	}
	
	// Send an alert (this spawns goroutines)
	monitoring.sendAlert(Alert{
		Level:       AlertLevelInfo,
		Title:       "Test Alert", 
		Description: "Test alert from example",
		Timestamp:   time.Now(),
		Component:   "test",
	})
	
	// The cleanup will automatically run when the test ends
	// It will:
	// 1. Shut down monitoring system (waiting for all goroutines)
	// 2. Close state manager (waiting for audit log goroutines)
	// 3. Run custom cleanup functions
	// 4. Redirect output to devnull to suppress late writes
}

// TestExampleConcurrentOperations demonstrates cleanup with concurrent operations
func TestExampleConcurrentOperations(t *testing.T) {
	cleanup := NewTestCleanup(t)
	
	// Create manager
	manager, err := NewStateManager(DefaultManagerOptions())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	cleanup.SetManager(manager)
	
	ctx := context.Background()
	contextID, err := manager.CreateContext(ctx, "test", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	
	// Start concurrent operations
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				select {
				case <-done:
					return
				default:
					manager.UpdateState(ctx, contextID, "test", map[string]interface{}{
						"worker": id,
						"count":  j,
					}, UpdateOptions{})
					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}
	
	// Let operations run
	time.Sleep(100 * time.Millisecond)
	
	// Signal workers to stop
	close(done)
	
	// Give workers time to finish before cleanup
	time.Sleep(50 * time.Millisecond)
	
	// Cleanup will ensure all goroutines are properly terminated
}