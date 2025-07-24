package state

import (
	"context"
	"testing"
	"time"
)

// TestStore creates a new StateStore that will be automatically closed
// when the test completes. This ensures proper resource cleanup.
func TestStore(t *testing.T) *StateStore {
	t.Helper()
	store := NewStateStore()
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

// TestMonitoringSystem creates a new MonitoringSystem that will be automatically
// shut down when the test completes. This ensures proper resource cleanup.
func TestMonitoringSystem(t *testing.T, config MonitoringConfig) (*MonitoringSystem, error) {
	t.Helper()
	ms, err := NewMonitoringSystem(config)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	})
	return ms, nil
}

// TestStoreWithInitialState creates a new StateStore with initial state
// that will be automatically closed when the test completes.
func TestStoreWithInitialState(t *testing.T, initialState map[string]interface{}) *StateStore {
	t.Helper()
	store := TestStore(t)
	if initialState != nil {
		if err := store.Set("/", initialState); err != nil {
			t.Fatalf("Failed to set initial state: %v", err)
		}
	}
	return store
}

// TestStoreWithPath creates a new StateStore with a value at a specific path
// that will be automatically closed when the test completes.
func TestStoreWithPath(t *testing.T, path string, value interface{}) *StateStore {
	t.Helper()
	store := TestStore(t)
	if err := store.Set(path, value); err != nil {
		t.Fatalf("Failed to set value at path %s: %v", path, err)
	}
	return store
}


// TestGenerator creates a new StateEventGenerator for testing.
func TestGenerator(t *testing.T, store *StateStore) *StateEventGenerator {
	t.Helper()
	return NewStateEventGenerator(store)
}

// TestHealthCheck creates a custom health check that will be automatically
// cleaned up when the test completes (if it has cleanup).
func TestHealthCheck(t *testing.T, name string, checkFunc func(context.Context) error) HealthCheck {
	t.Helper()
	return NewCustomHealthCheck(name, checkFunc)
}

// AssertEventuallyTrue asserts that a condition becomes true within the given timeout.
// This is useful for testing async operations.
func AssertEventuallyTrue(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	interval := timeout / 20 // Check 20 times
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Errorf("Condition not met within %v: %s", timeout, msg)
}

// AssertNoPanic asserts that the given function does not panic.
func AssertNoPanic(t *testing.T, f func(), msg string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Function panicked: %v. %s", r, msg)
		}
	}()
	f()
}

// WaitForBatchProcessing waits for batch processing to complete.
// This is useful when testing batched operations.
func WaitForBatchProcessing(timeout time.Duration) {
	time.Sleep(timeout)
}