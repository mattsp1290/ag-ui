package state

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStateStore_BasicOperations(t *testing.T) {
	store := NewStateStore()

	// Test Set
	if err := store.Set("/users/123", map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}); err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Test Get
	value, err := store.Get("/users/123")
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	user, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", value)
	}

	if user["name"] != "John Doe" {
		t.Errorf("Expected name 'John Doe', got %v", user["name"])
	}

	// Test Delete
	if err := store.Delete("/users/123"); err != nil {
		t.Fatalf("Failed to delete value: %v", err)
	}

	// Verify deletion
	_, err = store.Get("/users/123")
	if err == nil {
		t.Error("Expected error when getting deleted value")
	}
}

func TestStateStore_JSONPatch(t *testing.T) {
	store := NewStateStore()

	// Initialize state
	store.Set("/", map[string]interface{}{
		"users": map[string]interface{}{
			"123": map[string]interface{}{
				"name": "John",
				"age":  30,
			},
		},
	})

	// Apply patch
	patch := JSONPatch{
		{Op: JSONPatchOpReplace, Path: "/users/123/name", Value: "Jane"},
		{Op: JSONPatchOpAdd, Path: "/users/123/email", Value: "jane@example.com"},
		{Op: JSONPatchOpRemove, Path: "/users/123/age"},
	}

	if err := store.ApplyPatch(patch); err != nil {
		t.Fatalf("Failed to apply patch: %v", err)
	}

	// Verify changes
	user, err := store.Get("/users/123")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	userMap := user.(map[string]interface{})
	if userMap["name"] != "Jane" {
		t.Errorf("Expected name 'Jane', got %v", userMap["name"])
	}
	if userMap["email"] != "jane@example.com" {
		t.Errorf("Expected email 'jane@example.com', got %v", userMap["email"])
	}
	if _, exists := userMap["age"]; exists {
		t.Error("Expected age to be removed")
	}
}

func TestStateStore_Transactions(t *testing.T) {
	store := NewStateStore()

	// Initialize state
	store.Set("/counter", 0)

	// Start transaction
	tx := store.Begin()

	// Apply changes in transaction
	patch := JSONPatch{
		{Op: JSONPatchOpReplace, Path: "/counter", Value: 10},
	}
	if err := tx.Apply(patch); err != nil {
		t.Fatalf("Failed to apply patch in transaction: %v", err)
	}

	// Verify state hasn't changed yet
	counter, _ := store.Get("/counter")
	if counter.(float64) != 0 {
		t.Error("State changed before commit")
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify state changed after commit
	counter, _ = store.Get("/counter")
	if counter.(float64) != 10 {
		t.Errorf("Expected counter 10, got %v", counter)
	}
}

func TestStateStore_Rollback(t *testing.T) {
	store := NewStateStore()

	// Initialize state
	store.Set("/value", "initial")

	// Start transaction
	tx := store.Begin()

	// Apply changes
	patch := JSONPatch{
		{Op: JSONPatchOpReplace, Path: "/value", Value: "modified"},
	}
	tx.Apply(patch)

	// Rollback
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Failed to rollback: %v", err)
	}

	// Verify state unchanged
	value, _ := store.Get("/value")
	if value != "initial" {
		t.Errorf("Expected value 'initial', got %v", value)
	}
}

func TestStateStore_History(t *testing.T) {
	store := NewStateStore(WithMaxHistory(10))

	// Make several changes
	store.Set("/step", 1)
	store.Set("/step", 2)
	store.Set("/step", 3)

	// Get history
	history, err := store.GetHistory()
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}

	// Should have 4 versions (initial + 3 changes)
	if len(history) != 4 {
		t.Errorf("Expected 4 history entries, got %d", len(history))
	}

	// Verify versions have parent references
	for i := 1; i < len(history); i++ {
		if history[i].ParentID != history[i-1].ID {
			t.Error("Invalid parent reference in history")
		}
	}
}

func TestStateStore_Snapshot(t *testing.T) {
	store := NewStateStore()

	// Set initial state
	store.Set("/data", map[string]interface{}{
		"value": 100,
		"items": []interface{}{"a", "b", "c"},
	})

	// Create snapshot
	snapshot, err := store.CreateSnapshot()
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Modify state
	store.Set("/data/value", 200)
	store.Set("/data/items", []interface{}{"x", "y", "z"})

	// Verify state changed
	data, _ := store.Get("/data")
	dataMap := data.(map[string]interface{})
	if dataMap["value"].(float64) != 200 {
		t.Error("State not modified as expected")
	}

	// Restore snapshot
	if err := store.RestoreSnapshot(snapshot); err != nil {
		t.Fatalf("Failed to restore snapshot: %v", err)
	}

	// Verify state restored
	data, _ = store.Get("/data")
	dataMap = data.(map[string]interface{})
	if dataMap["value"].(float64) != 100 {
		t.Errorf("Expected value 100 after restore, got %v", dataMap["value"])
	}
}

func TestStateStore_Subscriptions(t *testing.T) {
	store := NewStateStore()

	var wg sync.WaitGroup
	changes := make([]StateChange, 0)
	var mu sync.Mutex

	// Subscribe to changes
	unsubscribe := store.Subscribe("/users", func(change StateChange) {
		mu.Lock()
		changes = append(changes, change)
		mu.Unlock()
		wg.Done()
	})
	defer unsubscribe()

	// Make changes
	wg.Add(1)
	store.Set("/users/123", map[string]interface{}{"name": "John"})
	
	wg.Add(1)
	store.Set("/users/456", map[string]interface{}{"name": "Jane"})

	// Wait for notifications
	wg.Wait()

	// Verify notifications received
	if len(changes) != 2 {
		t.Errorf("Expected 2 change notifications, got %d", len(changes))
	}
}

func TestStateStore_ConcurrentAccess(t *testing.T) {
	store := NewStateStore()

	// Initialize counters
	for i := 0; i < 10; i++ {
		store.Set(fmt.Sprintf("/counter/%d", i), 0)
	}

	// Concurrent updates
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				path := fmt.Sprintf("/counter/%d", id)
				val, _ := store.Get(path)
				count := int(val.(float64))
				store.Set(path, count+1)
			}
		}(i)
	}

	wg.Wait()

	// Verify all counters
	for i := 0; i < 10; i++ {
		val, _ := store.Get(fmt.Sprintf("/counter/%d", i))
		if val.(float64) != 100 {
			t.Errorf("Counter %d has value %v, expected 100", i, val)
		}
	}
}

func TestStateStore_ImportExport(t *testing.T) {
	store1 := NewStateStore()

	// Set some state
	store1.Set("/config", map[string]interface{}{
		"debug":   true,
		"timeout": 30,
		"servers": []interface{}{"server1", "server2"},
	})

	// Export
	data, err := store1.Export()
	if err != nil {
		t.Fatalf("Failed to export: %v", err)
	}

	// Import into new store
	store2 := NewStateStore()
	if err := store2.Import(data); err != nil {
		t.Fatalf("Failed to import: %v", err)
	}

	// Verify states match
	config1, _ := store1.Get("/config")
	config2, _ := store2.Get("/config")

	json1, _ := json.Marshal(config1)
	json2, _ := json.Marshal(config2)

	if string(json1) != string(json2) {
		t.Error("Exported and imported states don't match")
	}
}

func TestStateStore_VersionTracking(t *testing.T) {
	store := NewStateStore()

	initialVersion := store.GetVersion()
	if initialVersion != 0 {
		t.Errorf("Expected initial version 0, got %d", initialVersion)
	}

	// Make changes
	store.Set("/test", "value1")
	if store.GetVersion() != 1 {
		t.Error("Version not incremented after Set")
	}

	store.Delete("/test")
	if store.GetVersion() != 2 {
		t.Error("Version not incremented after Delete")
	}

	patch := JSONPatch{{Op: JSONPatchOpAdd, Path: "/new", Value: "value"}}
	store.ApplyPatch(patch)
	if store.GetVersion() != 3 {
		t.Error("Version not incremented after ApplyPatch")
	}
}

func TestStateStore_ComplexPaths(t *testing.T) {
	store := NewStateStore()

	// Set nested structure
	store.Set("/", map[string]interface{}{
		"deeply": map[string]interface{}{
			"nested": map[string]interface{}{
				"structure": map[string]interface{}{
					"with": map[string]interface{}{
						"data": []interface{}{1, 2, 3},
					},
				},
			},
		},
	})

	// Access deep path
	value, err := store.Get("/deeply/nested/structure/with/data")
	if err != nil {
		t.Fatalf("Failed to get deep path: %v", err)
	}

	arr, ok := value.([]interface{})
	if !ok || len(arr) != 3 {
		t.Error("Failed to retrieve nested array")
	}

	// Update deep path
	store.Set("/deeply/nested/structure/with/data", []interface{}{4, 5, 6})

	// Verify update
	value, _ = store.Get("/deeply/nested/structure/with/data")
	arr = value.([]interface{})
	if arr[0].(float64) != 4 {
		t.Error("Failed to update nested array")
	}
}

func TestStateStore_SubscriptionPatterns(t *testing.T) {
	store := NewStateStore()
	received := make(map[string]int)
	var mu sync.Mutex

	// Subscribe with wildcard
	store.Subscribe("/items/*", func(change StateChange) {
		mu.Lock()
		received[change.Path]++
		mu.Unlock()
	})

	// Make changes
	store.Set("/items/1", "value1")
	store.Set("/items/2", "value2")
	store.Set("/other/1", "value3") // Should not trigger

	// Allow time for async notifications
	time.Sleep(100 * time.Millisecond)

	// Verify
	mu.Lock()
	defer mu.Unlock()
	if received["/items/1"] != 1 {
		t.Error("Did not receive notification for /items/1")
	}
	if received["/items/2"] != 1 {
		t.Error("Did not receive notification for /items/2")
	}
	if received["/other/1"] > 0 {
		t.Error("Received unexpected notification for /other/1")
	}
}