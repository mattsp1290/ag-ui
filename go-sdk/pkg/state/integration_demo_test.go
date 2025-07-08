//go:build integration
// +build integration

package state

import (
	"context"
	"testing"
)

func TestIntegrationDemo(t *testing.T) {
	// Create state manager with default options
	opts := DefaultManagerOptions()
	manager, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	defer manager.Close()

	// Test context creation with security validation
	ctx := context.Background()
	contextID, err := manager.CreateContext(ctx, "test-state", map[string]interface{}{
		"user": "test-user",
		"data": map[string]interface{}{
			"count": 0,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	t.Logf("Created context: %s", contextID)

	// Test secure state updates
	updates := map[string]interface{}{
		"data": map[string]interface{}{
			"count":   1,
			"message": "Hello, secure world!",
		},
	}

	patch, err := manager.UpdateState(ctx, contextID, "test-state", updates, UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update state: %v", err)
	}

	t.Logf("Applied patch with %d operations", len(patch))

	// Test state retrieval
	currentState, err := manager.GetState(ctx, contextID, "test-state")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	t.Logf("Current state: %+v", currentState)

	// Test malicious input (should be rejected)
	maliciousUpdates := map[string]interface{}{
		"../../admin/secret": "hacked!",
	}

	_, err = manager.UpdateState(ctx, contextID, "test-state", maliciousUpdates, UpdateOptions{})
	if err != nil {
		t.Logf("✓ Security validation correctly rejected malicious input: %v", err)
	} else {
		t.Fatal("Security validation failed - malicious input was accepted!")
	}

	t.Log("✓ All integration tests passed! State management system is working correctly.")
}
