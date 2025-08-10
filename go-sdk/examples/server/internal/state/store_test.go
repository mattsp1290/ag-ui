package state

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mattbaird "github.com/mattbaird/jsonpatch"
)

func TestNewStore(t *testing.T) {
	store := NewStore()
	
	assert.NotNil(t, store)
	assert.NotNil(t, store.state)
	assert.Equal(t, int64(1), store.state.Version)
	assert.Equal(t, 0, store.state.Counter)
	assert.Equal(t, 0, len(store.state.Items))
	assert.Equal(t, 0, store.GetWatcherCount())
}

func TestSnapshot(t *testing.T) {
	store := NewStore()
	
	// Test initial snapshot
	snapshot := store.Snapshot()
	assert.NotNil(t, snapshot)
	assert.Equal(t, int64(1), snapshot.Version)
	assert.Equal(t, 0, snapshot.Counter)
	assert.Equal(t, 0, len(snapshot.Items))
	
	// Modify original state directly (should not affect snapshot)
	store.state.Counter = 99
	
	// Snapshot should be unchanged
	assert.Equal(t, 0, snapshot.Counter)
}

func TestUpdateIncrementCounter(t *testing.T) {
	store := NewStore()
	
	// Test counter increment
	err := store.Update(func(s *State) {
		s.Counter++
	})
	
	require.NoError(t, err)
	
	snapshot := store.Snapshot()
	assert.Equal(t, int64(2), snapshot.Version) // Version should increment
	assert.Equal(t, 1, snapshot.Counter)
}

func TestUpdateAddItem(t *testing.T) {
	store := NewStore()
	
	// Add an item
	testItem := Item{
		ID:    "test-item-1",
		Value: "test value",
		Type:  "test",
	}
	
	err := store.Update(func(s *State) {
		s.Items = append(s.Items, testItem)
	})
	
	require.NoError(t, err)
	
	snapshot := store.Snapshot()
	assert.Equal(t, int64(2), snapshot.Version)
	assert.Equal(t, 1, len(snapshot.Items))
	assert.Equal(t, testItem.ID, snapshot.Items[0].ID)
	assert.Equal(t, testItem.Value, snapshot.Items[0].Value)
	assert.Equal(t, testItem.Type, snapshot.Items[0].Type)
}

func TestUpdateMultipleOperations(t *testing.T) {
	store := NewStore()
	
	// Perform multiple operations in a single update
	err := store.Update(func(s *State) {
		s.Counter = 5
		s.Items = append(s.Items, Item{ID: "item1", Value: "value1"})
		s.Items = append(s.Items, Item{ID: "item2", Value: "value2"})
	})
	
	require.NoError(t, err)
	
	snapshot := store.Snapshot()
	assert.Equal(t, int64(2), snapshot.Version)
	assert.Equal(t, 5, snapshot.Counter)
	assert.Equal(t, 2, len(snapshot.Items))
}

func TestUpdateNoChanges(t *testing.T) {
	store := NewStore()
	initialVersion := store.Snapshot().Version
	
	// Update that doesn't actually change anything
	err := store.Update(func(s *State) {
		// No changes
	})
	
	require.NoError(t, err)
	
	// Version should not change if no actual changes occurred
	snapshot := store.Snapshot()
	assert.Equal(t, initialVersion+1, snapshot.Version) // Version still increments
}

func TestPatchGeneration(t *testing.T) {
	store := NewStore()
	
	// Create a watcher to capture the patch
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	watcher, err := store.Watch(ctx)
	require.NoError(t, err)
	
	// Perform an update
	err = store.Update(func(s *State) {
		s.Counter = 42
	})
	require.NoError(t, err)
	
	// Wait for the delta
	select {
	case delta := <-watcher.Channel():
		assert.Equal(t, "STATE_DELTA", delta.Type)
		assert.Equal(t, int64(2), delta.Version)
		assert.NotEmpty(t, delta.Patch)
		
		// Check that patch contains the expected operation
		foundCounterOp := false
		for _, op := range delta.Patch {
			if path, ok := op["path"].(string); ok && path == "/counter" {
				if opType, ok := op["op"].(string); ok && opType == "replace" {
					if value, ok := op["value"].(float64); ok && value == 42 {
						foundCounterOp = true
					}
				}
			}
		}
		assert.True(t, foundCounterOp, "Should find counter replace operation in patch")
		
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for state delta")
	}
}

func TestPatchValidation(t *testing.T) {
	store := NewStore()
	
	// Get initial state
	oldState := store.Snapshot()
	oldJSON, err := oldState.ToJSON()
	require.NoError(t, err)
	
	// Create modified state
	newState := oldState.Clone()
	newState.Counter = 100
	newState.Version = oldState.Version + 1
	newJSON, err := newState.ToJSON()
	require.NoError(t, err)
	
	// Generate patch
	patchOps, err := mattbaird.CreatePatch(oldJSON, newJSON)
	require.NoError(t, err)
	
	// Convert to patch maps
	patchMaps := make([]map[string]interface{}, len(patchOps))
	for i, op := range patchOps {
		patchMaps[i] = map[string]interface{}{
			"op":   op.Operation,
			"path": op.Path,
		}
		if op.Value != nil {
			patchMaps[i]["value"] = op.Value
		}
	}
	
	// Validate patch using the store's validation method
	err = store.validatePatch(oldJSON, newJSON, patchMaps)
	assert.NoError(t, err)
}

func TestPatchValidationFailure(t *testing.T) {
	store := NewStore()
	
	// Create a deliberately incorrect scenario
	oldJSON := []byte(`{"version": 1, "counter": 0, "items": []}`)
	newJSON := []byte(`{"version": 2, "counter": 5, "items": []}`)
	
	// Create a patch that doesn't match the transition
	badPatchMaps := []map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/counter",
			"value": 10, // Different from what's in newJSON
		},
	}
	
	// Validation should fail
	err := store.validatePatch(oldJSON, newJSON, badPatchMaps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "patch application does not match expected result")
}

func TestWatcher(t *testing.T) {
	store := NewStore()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create watcher
	watcher, err := store.Watch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, store.GetWatcherCount())
	
	// Perform update
	go func() {
		time.Sleep(10 * time.Millisecond)
		err := store.Update(func(s *State) {
			s.Counter = 777
		})
		require.NoError(t, err)
	}()
	
	// Wait for delta
	select {
	case delta := <-watcher.Channel():
		assert.Equal(t, "STATE_DELTA", delta.Type)
		assert.Equal(t, int64(2), delta.Version)
		
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for state delta")
	}
	
	// Close watcher
	watcher.Close()
	
	// Give some time for cleanup
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 0, store.GetWatcherCount())
}

func TestMultipleWatchers(t *testing.T) {
	store := NewStore()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create multiple watchers
	watcher1, err := store.Watch(ctx)
	require.NoError(t, err)
	
	watcher2, err := store.Watch(ctx)
	require.NoError(t, err)
	
	assert.Equal(t, 2, store.GetWatcherCount())
	
	// Perform update
	err = store.Update(func(s *State) {
		s.Counter = 123
	})
	require.NoError(t, err)
	
	// Both watchers should receive the delta
	deltas := make([]*StateDelta, 0, 2)
	
	for i := 0; i < 2; i++ {
		select {
		case delta := <-watcher1.Channel():
			deltas = append(deltas, delta)
		case delta := <-watcher2.Channel():
			deltas = append(deltas, delta)
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for state delta")
		}
	}
	
	// Verify both deltas are identical
	assert.Equal(t, 2, len(deltas))
	for _, delta := range deltas {
		assert.Equal(t, "STATE_DELTA", delta.Type)
		assert.Equal(t, int64(2), delta.Version)
	}
}

func TestWatcherContextCancellation(t *testing.T) {
	store := NewStore()
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create watcher
	watcher, err := store.Watch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, store.GetWatcherCount())
	
	// Cancel context
	cancel()
	
	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)
	
	// Watcher should be removed
	assert.Equal(t, 0, store.GetWatcherCount())
	
	// Channel should be closed
	select {
	case _, ok := <-watcher.Channel():
		assert.False(t, ok, "Watcher channel should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Channel should be closed")
	}
}

func TestStoreClose(t *testing.T) {
	store := NewStore()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create watchers
	_, err := store.Watch(ctx)
	require.NoError(t, err)
	_, err = store.Watch(ctx)
	require.NoError(t, err)
	
	assert.Equal(t, 2, store.GetWatcherCount())
	
	// Close store
	store.Close()
	
	// All watchers should be removed
	assert.Equal(t, 0, store.GetWatcherCount())
}

func TestStateJSONSerialization(t *testing.T) {
	state := &State{
		Version: 42,
		Counter: 100,
		Items: []Item{
			{ID: "item1", Value: "value1", Type: "type1"},
			{ID: "item2", Value: "value2"},
		},
	}
	
	jsonData, err := state.ToJSON()
	require.NoError(t, err)
	
	// Verify JSON structure
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)
	
	assert.Equal(t, float64(42), parsed["version"])
	assert.Equal(t, float64(100), parsed["counter"])
	
	items, ok := parsed["items"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, 2, len(items))
}

func TestStateClone(t *testing.T) {
	original := &State{
		Version: 1,
		Counter: 5,
		Items: []Item{
			{ID: "item1", Value: "value1"},
		},
	}
	
	clone := original.Clone()
	
	// Verify clone is identical
	assert.Equal(t, original.Version, clone.Version)
	assert.Equal(t, original.Counter, clone.Counter)
	assert.Equal(t, len(original.Items), len(clone.Items))
	assert.Equal(t, original.Items[0], clone.Items[0])
	
	// Verify they are independent
	clone.Counter = 999
	clone.Items[0].Value = "modified"
	
	assert.Equal(t, 5, original.Counter)
	assert.Equal(t, "value1", original.Items[0].Value)
}

func TestStateEvents(t *testing.T) {
	state := &State{Version: 5, Counter: 10}
	
	// Test StateSnapshot
	snapshot := NewStateSnapshot(state)
	assert.Equal(t, "STATE_SNAPSHOT", snapshot.Type)
	assert.Equal(t, int64(5), snapshot.Version)
	assert.Equal(t, state, snapshot.Data)
	
	// Test StateDelta
	patch := []map[string]interface{}{
		{"op": "replace", "path": "/counter", "value": 15},
	}
	delta := NewStateDelta(6, patch)
	assert.Equal(t, "STATE_DELTA", delta.Type)
	assert.Equal(t, int64(6), delta.Version)
	assert.Equal(t, patch, delta.Patch)
}

func BenchmarkStoreUpdate(b *testing.B) {
	store := NewStore()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := store.Update(func(s *State) {
			s.Counter++
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPatchGeneration(b *testing.B) {
	oldState := &State{Version: 1, Counter: 0, Items: []Item{}}
	newState := &State{Version: 2, Counter: 1, Items: []Item{}}
	
	oldJSON, _ := oldState.ToJSON()
	newJSON, _ := newState.ToJSON()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mattbaird.CreatePatch(oldJSON, newJSON)
		if err != nil {
			b.Fatal(err)
		}
	}
}