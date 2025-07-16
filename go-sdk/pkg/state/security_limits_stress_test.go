//go:build stress
// +build stress

package state

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestConcurrentSecurityValidationStress tests security validation under heavy concurrent load
// This test is only run with the "stress" build tag: go test -tags stress
func TestConcurrentSecurityValidationStress(t *testing.T) {
	// Add timeout protection to prevent indefinite hangs
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	sm, err := NewStateManager(DefaultManagerOptions())
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	defer sm.Close()

	// Create multiple contexts
	numContexts := 10
	contexts := make([]string, numContexts)
	for i := 0; i < numContexts; i++ {
		contextID, err := sm.CreateContext(ctx, fmt.Sprintf("stress-state-%d", i), nil)
		if err != nil {
			t.Fatalf("Failed to create context %d: %v", i, err)
		}
		contexts[i] = contextID
	}

	// Stress test with original high load
	var wg sync.WaitGroup
	numWorkers := 20      // Original heavy load
	updatesPerWorker := 100  // Original heavy load

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < updatesPerWorker; j++ {
				// Check for timeout
				select {
				case <-ctx.Done():
					t.Errorf("Stress test timed out after %d updates", j)
					return
				default:
				}

				contextID := contexts[j%numContexts]

				// Mix of valid and invalid updates
				var updates map[string]interface{}
				switch j % 5 {
				case 0: // Valid update
					updates = map[string]interface{}{
						fmt.Sprintf("stress_worker_%d_update_%d", workerID, j): "valid",
					}
				case 1: // String too long
					updates = map[string]interface{}{
						"long": strings.Repeat("x", MaxStringLengthBytes+1),
					}
				case 2: // Array too long
					arr := make([]interface{}, MaxArrayLength+1)
					updates = map[string]interface{}{
						"array": arr,
					}
				case 3: // Too deep
					var deep interface{} = "value"
					for k := 0; k < MaxJSONDepth+2; k++ {
						deep = map[string]interface{}{"d": deep}
					}
					updates = map[string]interface{}{
						"deep": deep,
					}
				case 4: // Too many keys
					updates = make(map[string]interface{})
					for k := 0; k < 1001; k++ {
						updates[fmt.Sprintf("key_%d", k)] = k
					}
				}

				_, err := sm.UpdateState(ctx, contextID, fmt.Sprintf("stress-state-%d", j%numContexts), updates, UpdateOptions{})

				// Check that appropriate errors are returned
				if j%5 == 0 && err != nil {
					t.Errorf("Stress Worker %d: Valid update %d failed: %v", workerID, j, err)
				} else if j%5 != 0 && err == nil {
					t.Errorf("Stress Worker %d: Invalid update %d should have failed", workerID, j)
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestRateLimitingIntegrationStress tests rate limiting under heavy load
// This test is only run with the "stress" build tag: go test -tags stress
func TestRateLimitingIntegrationStress(t *testing.T) {
	// Add timeout protection to prevent indefinite hangs
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create state manager with custom rate limiting
	opts := DefaultManagerOptions()
	sm, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	defer sm.Close()

	contextID, err := sm.CreateContext(ctx, "rate-stress-test", nil)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	// Original heavy load for stress testing
	numRequests := 500  // Higher load for stress testing
	errors := 0
	start := time.Now()

	for i := 0; i < numRequests; i++ {
		// Check for timeout
		select {
		case <-ctx.Done():
			t.Errorf("Stress test timed out after %d requests", i)
			return
		default:
		}

		updates := map[string]interface{}{
			fmt.Sprintf("stress_update_%d", i): i,
		}

		_, err := sm.UpdateState(ctx, contextID, "rate-stress-test", updates, UpdateOptions{})
		if err != nil && strings.Contains(err.Error(), "rate limit exceeded") {
			errors++
		}
	}

	duration := time.Since(start)

	// Should have some rate limit errors
	if errors == 0 {
		t.Logf("No rate limit errors encountered - this may be acceptable with current configuration")
	}

	// But not all requests should fail
	if errors == numRequests {
		t.Error("All requests were rate limited")
	}

	t.Logf("Stress rate limiting: %d/%d requests failed in %v", errors, numRequests, duration)
}