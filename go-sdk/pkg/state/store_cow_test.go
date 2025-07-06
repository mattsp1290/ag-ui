package state

import (
	"testing"
)

// TestStateStore_COW verifies copy-on-write behavior
func TestStateStore_COW(t *testing.T) {
	store := NewStateStore()

	// Set initial state
	initialData := map[string]interface{}{
		"key1": "value1",
		"key2": map[string]interface{}{
			"nested": "value2",
		},
	}
	err := store.Set("/", initialData)
	if err != nil {
		t.Fatal(err)
	}

	// Get a view of the state
	view1 := store.GetStateView()
	defer view1.Cleanup()

	// Update the state
	err = store.Set("/key1", "updated_value1")
	if err != nil {
		t.Fatal(err)
	}

	// Get another view after update
	view2 := store.GetStateView()
	defer view2.Cleanup()

	// Original view should still see old value
	data1 := view1.Data()
	if data1["key1"] != "value1" {
		t.Errorf("View1 should see original value, got %v", data1["key1"])
	}

	// New view should see updated value
	data2 := view2.Data()
	if data2["key1"] != "updated_value1" {
		t.Errorf("View2 should see updated value, got %v", data2["key1"])
	}

	// Both views should see the same nested data (shared reference)
	nested1 := data1["key2"].(map[string]interface{})
	nested2 := data2["key2"].(map[string]interface{})
	if nested1["nested"] != nested2["nested"] {
		t.Error("Nested values should be the same")
	}
}

// TestStateStore_ReferenceCount verifies reference counting
func TestStateStore_ReferenceCount(t *testing.T) {
	store := NewStateStore()

	// Set initial state
	err := store.Set("/", map[string]interface{}{"key": "value"})
	if err != nil {
		t.Fatal(err)
	}

	// Initial reference count should be 0
	refs := store.GetReferenceCount()
	if refs != 0 {
		t.Errorf("Initial reference count should be 0, got %d", refs)
	}

	// Get a view - should increment reference count
	view := store.GetStateView()
	refs = store.GetReferenceCount()
	if refs != 1 {
		t.Errorf("Reference count should be 1 after GetStateView, got %d", refs)
	}

	// Cleanup should decrement reference count
	view.Cleanup()
	refs = store.GetReferenceCount()
	if refs != 0 {
		t.Errorf("Reference count should be 0 after Cleanup, got %d", refs)
	}
}

// TestStateStore_ConcurrentCOW verifies COW under concurrent access
func TestStateStore_ConcurrentCOW(t *testing.T) {
	store := NewStateStore()

	// Set initial state
	err := store.Set("/", map[string]interface{}{
		"counter": 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Run concurrent readers and writers
	done := make(chan bool)
	errors := make(chan error, 100)

	// Writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				err := store.Set("/counter", id*100+j)
				if err != nil {
					errors <- err
				}
			}
			done <- true
		}(i)
	}

	// Readers
	for i := 0; i < 20; i++ {
		go func() {
			for j := 0; j < 200; j++ {
				view := store.GetStateView()
				_ = view.Data()
				view.Cleanup()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 30; i++ {
		<-done
	}

	// Check for errors
	select {
	case err := <-errors:
		t.Fatal(err)
	default:
		// No errors
	}

	// Final reference count should be 0
	refs := store.GetReferenceCount()
	if refs != 0 {
		t.Errorf("Final reference count should be 0, got %d", refs)
	}
}