package state

import (
	"context"
	"strings"
	"testing"
)

func TestSecurityValidator(t *testing.T) {
	validator := NewSecurityValidator(DefaultSecurityConfig())

	t.Run("validates JSON pointer path traversal", func(t *testing.T) {
		maliciousPaths := []string{
			"/../../etc/passwd",
			"/../admin/secrets",
			"/.//../sensitive",
			"//double/slash",
		}

		for _, path := range maliciousPaths {
			err := validator.ValidateJSONPointer(path)
			if err == nil {
				t.Errorf("Should have rejected malicious path: %s", path)
			}
		}
	})

	t.Run("validates resource limits", func(t *testing.T) {
		// Test oversized state
		largeState := make(map[string]interface{})
		for i := 0; i < 1000; i++ {
			largeState[string(rune(i))] = strings.Repeat("x", 1000)
		}

		err := validator.ValidateState(largeState)
		if err == nil {
			t.Error("Should have rejected oversized state")
		}
	})

	t.Run("validates patch operations", func(t *testing.T) {
		maliciousPatch := JSONPatch{
			{Op: JSONPatchOpAdd, Path: "/admin/../../secrets", Value: "hacked"},
		}

		err := validator.ValidatePatch(maliciousPatch)
		if err == nil {
			t.Error("Should have rejected malicious patch")
		}
	})
}

func TestStateManagerSecurity(t *testing.T) {
	opts := DefaultManagerOptions()
	manager, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	contextID, err := manager.CreateContext(ctx, "test", nil)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	t.Run("rejects malicious updates", func(t *testing.T) {
		maliciousUpdates := map[string]interface{}{
			"../../admin/secret": "hacked!",
		}

		_, err := manager.UpdateState(ctx, contextID, "test", maliciousUpdates, UpdateOptions{})
		if err == nil {
			t.Error("Should have rejected malicious update")
		}
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		_, err := manager.UpdateState(cancelCtx, contextID, "test", 
			map[string]interface{}{"safe": "update"}, UpdateOptions{})
		if err == nil {
			t.Error("Should have respected context cancellation")
		}
	})
}

func TestMemoryManagement(t *testing.T) {
	store := NewStateStore(WithMaxHistory(5)) // Small history for testing
	
	// Create many versions to test history trimming
	for i := 0; i < 10; i++ {
		store.Set("/counter", i)
	}
	
	history, err := store.GetHistory()
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	
	if len(history) > 5 {
		t.Errorf("History not properly trimmed: got %d versions, expected ≤ 5", len(history))
	}
}

func TestConcurrentSafety(t *testing.T) {
	store := NewStateStore()
	done := make(chan bool, 10)
	
	// Test concurrent subscriptions and cleanup
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			// Subscribe
			unsubscribe := store.Subscribe("/test", func(change StateChange) {
				// Do nothing
			})
			
			// Do some operations
			for j := 0; j < 10; j++ {
				store.Set("/test", j)
			}
			
			// Unsubscribe
			unsubscribe()
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Force subscription cleanup
	store.cleanupExpiredSubscriptions()
	
	// Test should complete without hanging or crashing
}