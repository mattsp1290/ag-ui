package state

import (
	"sync"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateEventHandler_HandleStateSnapshot(t *testing.T) {
	tests := []struct {
		name          string
		snapshot      interface{}
		wantErr       bool
		expectedState map[string]interface{}
	}{
		{
			name: "simple snapshot",
			snapshot: map[string]interface{}{
				"users": map[string]interface{}{
					"123": map[string]interface{}{
						"name":  "John Doe",
						"email": "john@example.com",
					},
				},
			},
			wantErr: false,
			expectedState: map[string]interface{}{
				"users": map[string]interface{}{
					"123": map[string]interface{}{
						"name":  "John Doe",
						"email": "john@example.com",
					},
				},
			},
		},
		{
			name:          "empty snapshot",
			snapshot:      map[string]interface{}{},
			wantErr:       false,
			expectedState: map[string]interface{}{},
		},
		{
			name:     "nil snapshot",
			snapshot: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create store and handler
			store := NewStateStore()
			defer store.Close()
			handler := NewStateEventHandler(store)

			// Create event
			event := events.NewStateSnapshotEvent(tt.snapshot)

			// Handle event
			err := handler.HandleStateSnapshot(event)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify state
				state := store.GetState()
				assert.Equal(t, tt.expectedState, state)
			}
		})
	}
}

func TestStateEventHandler_HandleStateDelta(t *testing.T) {
	// Create store with initial state
	baseStore := NewStateStore()
	defer baseStore.Close()
	err := baseStore.Set("/users/123", map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	})
	require.NoError(t, err)

	// Create failing store for error injection
	failingStore := NewFailingStore(baseStore, "storage", 0.1)
	store := StoreInterface(failingStore)

	tests := []struct {
		name          string
		delta         []events.JSONPatchOperation
		wantErr       bool
		expectedState map[string]interface{}
	}{
		{
			name: "update user email",
			delta: []events.JSONPatchOperation{
				{
					Op:    "replace",
					Path:  "/users/123/email",
					Value: "john.doe@example.com",
				},
			},
			wantErr: false,
			expectedState: map[string]interface{}{
				"users": map[string]interface{}{
					"123": map[string]interface{}{
						"name":  "John Doe",
						"email": "john.doe@example.com",
					},
				},
			},
		},
		{
			name: "add new field",
			delta: []events.JSONPatchOperation{
				{
					Op:    "add",
					Path:  "/users/123/age",
					Value: 30,
				},
			},
			wantErr: false,
			expectedState: map[string]interface{}{
				"users": map[string]interface{}{
					"123": map[string]interface{}{
						"name":  "John Doe",
						"email": "john.doe@example.com",
						"age":   30, // Test will accept either int or float64
					},
				},
			},
		},
		{
			name: "remove field",
			delta: []events.JSONPatchOperation{
				{
					Op:   "remove",
					Path: "/users/123/age",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid operation",
			delta: []events.JSONPatchOperation{
				{
					Op:   "invalid",
					Path: "/users/123",
				},
			},
			wantErr: true,
		},
	}

	// Create handler with small batch size for testing
	handler := NewStateEventHandler(store, WithBatchSize(1), WithBatchTimeout(10*time.Millisecond))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create event
			event := events.NewStateDeltaEvent(tt.delta)

			// Handle event
			err := handler.HandleStateDelta(event)

			// Wait for batch processing if needed
			if !tt.wantErr {
				time.Sleep(20 * time.Millisecond)
			}

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectedState != nil {
					// Verify state
					state := store.GetState()
					// Special handling for the age field test case
					if tt.name == "add new field" {
						users, ok := state["users"].(map[string]interface{})
						assert.True(t, ok)
						user123, ok := users["123"].(map[string]interface{})
						assert.True(t, ok)
						assert.Equal(t, "John Doe", user123["name"])
						assert.Equal(t, "john.doe@example.com", user123["email"])
						// Accept either int or float64 for age
						age, ok := user123["age"]
						assert.True(t, ok)
						switch v := age.(type) {
						case int:
							assert.Equal(t, 30, v)
						case float64:
							assert.Equal(t, float64(30), v)
						default:
							t.Errorf("Unexpected type for age: %T", age)
						}
					} else {
						assert.Equal(t, tt.expectedState, state)
					}
				}
			}
		})
	}
}

func TestStateEventHandler_Batching(t *testing.T) {
	// Create store and handler with batching
	store := NewStateStore()
	defer store.Close()
	handler := NewStateEventHandler(store,
		WithBatchSize(3),
		WithBatchTimeout(50*time.Millisecond),
	)

	// Send multiple delta events
	deltas := [][]events.JSONPatchOperation{
		{events.JSONPatchOperation{Op: "add", Path: "/field1", Value: "value1"}},
		{events.JSONPatchOperation{Op: "add", Path: "/field2", Value: "value2"}},
		{events.JSONPatchOperation{Op: "add", Path: "/field3", Value: "value3"}},
	}

	for _, delta := range deltas {
		event := events.NewStateDeltaEvent(delta)
		err := handler.HandleStateDelta(event)
		assert.NoError(t, err)
	}

	// Since we sent 3 events and batch size is 3, they should be processed immediately

	// Verify all changes were applied
	state := store.GetState()
	assert.Equal(t, "value1", state["field1"])
	assert.Equal(t, "value2", state["field2"])
	assert.Equal(t, "value3", state["field3"])
}

func TestStateEventGenerator_GenerateSnapshot(t *testing.T) {
	// Create store with state
	store := NewStateStore()
	defer store.Close()
	err := store.Set("/users/123", map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	})
	require.NoError(t, err)

	// Create generator
	generator := NewStateEventGenerator(store)

	// Generate snapshot
	event, err := generator.GenerateSnapshot()
	assert.NoError(t, err)
	assert.NotNil(t, event)

	// Verify snapshot content
	expectedSnapshot := map[string]interface{}{
		"users": map[string]interface{}{
			"123": map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
			},
		},
	}
	assert.Equal(t, expectedSnapshot, event.Snapshot)
}

func TestStateEventGenerator_GenerateDelta(t *testing.T) {
	store := NewStateStore()
	defer store.Close()
	generator := NewStateEventGenerator(store)

	oldState := map[string]interface{}{
		"users": map[string]interface{}{
			"123": map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
			},
		},
	}

	newState := map[string]interface{}{
		"users": map[string]interface{}{
			"123": map[string]interface{}{
				"name":  "John Doe",
				"email": "john.doe@example.com",
				"age":   30,
			},
		},
	}

	// Generate delta
	event, err := generator.GenerateDelta(oldState, newState)
	assert.NoError(t, err)
	assert.NotNil(t, event)

	// Verify delta operations
	assert.Len(t, event.Delta, 2)

	// Find the operations
	var hasEmailReplace, hasAgeAdd bool
	for _, op := range event.Delta {
		if op.Path == "/users/123/email" && op.Op == "replace" {
			hasEmailReplace = true
			assert.Equal(t, "john.doe@example.com", op.Value)
		}
		if op.Path == "/users/123/age" && op.Op == "add" {
			hasAgeAdd = true
			assert.Equal(t, float64(30), op.Value)
		}
	}

	assert.True(t, hasEmailReplace, "Expected email replace operation")
	assert.True(t, hasAgeAdd, "Expected age add operation")
}

func TestStateEventStream(t *testing.T) {
	// Create store and generator
	store := NewStateStore()
	defer store.Close()
	generator := NewStateEventGenerator(store)

	// Create stream with fast interval for testing
	stream := NewStateEventStream(store, generator,
		WithStreamInterval(50*time.Millisecond),
		WithDeltaOnly(false),
	)

	// Track received events
	var receivedEvents []events.Event
	var mu sync.Mutex

	// Subscribe to events
	unsubscribe := stream.Subscribe(func(event events.Event) error {
		mu.Lock()
		defer mu.Unlock()
		receivedEvents = append(receivedEvents, event)
		return nil
	})
	defer unsubscribe()

	// Start streaming
	err := stream.Start()
	assert.NoError(t, err)
	defer stream.Stop()

	// Wait for initial snapshot
	time.Sleep(100 * time.Millisecond)

	// Verify initial snapshot was received
	mu.Lock()
	assert.Len(t, receivedEvents, 1)
	_, ok := receivedEvents[0].(*events.StateSnapshotEvent)
	assert.True(t, ok, "First event should be a snapshot")
	mu.Unlock()

	// Make a state change
	err = store.Set("/test", "value")
	assert.NoError(t, err)

	// Wait for delta event
	time.Sleep(150 * time.Millisecond)

	// Verify delta event was received
	mu.Lock()
	assert.GreaterOrEqual(t, len(receivedEvents), 2)
	if len(receivedEvents) >= 2 {
		deltaEvent, ok := receivedEvents[1].(*events.StateDeltaEvent)
		assert.True(t, ok, "Second event should be a delta")
		if ok {
			assert.Len(t, deltaEvent.Delta, 1)
			assert.Equal(t, "add", deltaEvent.Delta[0].Op)
			assert.Equal(t, "/test", deltaEvent.Delta[0].Path)
			assert.Equal(t, "value", deltaEvent.Delta[0].Value)
		}
	}
	mu.Unlock()
}

func TestStateMetrics(t *testing.T) {
	metrics := NewStateMetrics()

	// Record some events
	metrics.IncrementEvents("snapshot")
	metrics.IncrementEvents("snapshot")
	metrics.IncrementEvents("delta")

	// Record some errors
	metrics.IncrementErrors("validation")

	// Record processing times
	metrics.RecordEventProcessing("snapshot", 10*time.Millisecond)
	metrics.RecordEventProcessing("snapshot", 20*time.Millisecond)
	metrics.RecordEventProcessing("delta", 5*time.Millisecond)

	// Get stats
	stats := metrics.GetStats()

	// Verify counters
	eventsProcessed := stats["events_processed"].(map[string]int64)
	assert.Equal(t, int64(2), eventsProcessed["snapshot"])
	assert.Equal(t, int64(1), eventsProcessed["delta"])

	errors := stats["errors"].(map[string]int64)
	assert.Equal(t, int64(1), errors["validation"])

	// Verify average processing times
	avgTimes := stats["avg_processing_times_ms"].(map[string]float64)
	assert.Equal(t, float64(15), avgTimes["snapshot"]) // (10+20)/2
	assert.Equal(t, float64(5), avgTimes["delta"])
}

func TestStateEventHandler_Callbacks(t *testing.T) {
	store := TestStore(t)

	// Track callback invocations
	var snapshotCalled, deltaCalled, stateChangeCalled bool
	var callbackMu sync.Mutex

	handler := NewStateEventHandler(store,
		WithSnapshotCallback(func(event *events.StateSnapshotEvent) error {
			snapshotCalled = true
			return nil
		}),
		WithDeltaCallback(func(event *events.StateDeltaEvent) error {
			deltaCalled = true
			return nil
		}),
		WithStateChangeCallback(func(change StateChange) {
			callbackMu.Lock()
			stateChangeCalled = true
			callbackMu.Unlock()
		}),
		WithBatchSize(1),
		WithBatchTimeout(10*time.Millisecond),
	)

	// Test snapshot callback
	snapshotEvent := events.NewStateSnapshotEvent(map[string]interface{}{"test": "snapshot"})
	err := handler.HandleStateSnapshot(snapshotEvent)
	assert.NoError(t, err)
	assert.True(t, snapshotCalled)

	// Wait for async state change callback
	AssertEventuallyTrue(t, func() bool {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		return stateChangeCalled
	}, 200*time.Millisecond, "state change callback should be called")

	// Reset flags
	deltaCalled = false
	callbackMu.Lock()
	stateChangeCalled = false
	callbackMu.Unlock()

	// Test delta callback
	deltaEvent := events.NewStateDeltaEvent([]events.JSONPatchOperation{
		{Op: "add", Path: "/new", Value: "value"},
	})
	err = handler.HandleStateDelta(deltaEvent)
	assert.NoError(t, err)

	// Wait for batch processing
	time.Sleep(50 * time.Millisecond)

	assert.True(t, deltaCalled)

	// Wait for async state change callback
	AssertEventuallyTrue(t, func() bool {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		return stateChangeCalled
	}, 200*time.Millisecond, "state change callback should be called after delta")
}

func TestStateEventHandler_ErrorRecovery(t *testing.T) {
	store := NewStateStore()
	defer store.Close()
	handler := NewStateEventHandler(store)

	// Set initial state
	initialState := map[string]interface{}{"initial": "state"}
	err := store.Set("/", initialState)
	require.NoError(t, err)

	// Create an invalid snapshot event that will fail validation
	event := events.NewStateSnapshotEvent(nil) // nil snapshot will fail validation

	// Handle event - should fail but preserve state
	err = handler.HandleStateSnapshot(event)
	assert.Error(t, err)

	// Verify state was not changed
	state := store.GetState()
	assert.Equal(t, initialState, state)
}

func TestConcurrentEventHandling(t *testing.T) {
	store := NewStateStore()
	defer store.Close()
	handler := NewStateEventHandler(store,
		WithBatchSize(10),
		WithBatchTimeout(50*time.Millisecond),
	)

	// Concurrent snapshot and delta handling
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Handle a single snapshot
	go func() {
		defer wg.Done()
		snapshot := map[string]interface{}{
			"counter": 0,
			"base":    "value",
		}
		event := events.NewStateSnapshotEvent(snapshot)
		err := handler.HandleStateSnapshot(event)
		assert.NoError(t, err)
	}()

	// Goroutine 2: Handle deltas
	go func() {
		defer wg.Done()
		// Wait a bit to ensure snapshot is applied first
		time.Sleep(20 * time.Millisecond)

		for i := 0; i < 5; i++ {
			delta := []events.JSONPatchOperation{
				{Op: "add", Path: "/delta" + string(rune('0'+i)), Value: i},
			}
			event := events.NewStateDeltaEvent(delta)
			err := handler.HandleStateDelta(event)
			assert.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Wait for final batch processing
	time.Sleep(100 * time.Millisecond)

	// Verify final state has content from both operations
	state := store.GetState()
	assert.NotNil(t, state["counter"])

	// Check that at least some deltas were applied
	deltaCount := 0
	for i := 0; i < 5; i++ {
		if _, exists := state["delta"+string(rune('0'+i))]; exists {
			deltaCount++
		}
	}
	assert.Greater(t, deltaCount, 0)
}
