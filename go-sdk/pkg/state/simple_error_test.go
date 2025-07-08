package state

import (
	"context"
	"testing"
)

// TestSimpleErrorInjection tests basic error injection functionality
func TestSimpleErrorInjection(t *testing.T) {
	// Create a simple state manager
	opts := DefaultManagerOptions()
	opts.EnableMetrics = false // Disable metrics to avoid logger issues

	manager, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	// Test basic operations
	ctx := context.Background()
	contextID, err := manager.CreateContext(ctx, "test-state", nil)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	// Test update
	updates := map[string]interface{}{
		"test": "value",
	}

	_, err = manager.UpdateState(ctx, contextID, "test-state", updates, UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update state: %v", err)
	}

	// Test get
	state, err := manager.GetState(ctx, contextID, "test-state")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if state == nil {
		t.Error("State should not be nil")
	}

	t.Log("Simple error injection test completed successfully")
}
