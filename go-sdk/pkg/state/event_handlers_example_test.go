package state_test

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/state"
)

// Example demonstrates basic usage of state event handlers
func Example_basicUsage() {
	// Create a state store
	store := state.NewStateStore()

	// Create event handler
	handler := state.NewStateEventHandler(store)

	// Handle a state snapshot event
	snapshotData := map[string]interface{}{
		"users": map[string]interface{}{
			"123": map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
			},
		},
		"settings": map[string]interface{}{
			"theme": "dark",
		},
	}

	snapshotEvent := events.NewStateSnapshotEvent(snapshotData)
	if err := handler.HandleStateSnapshot(snapshotEvent); err != nil {
		log.Fatal(err)
	}

	// Handle a state delta event
	deltaOps := []events.JSONPatchOperation{
		{
			Op:    "replace",
			Path:  "/users/123/email",
			Value: "john.doe@example.com",
		},
		{
			Op:    "add",
			Path:  "/users/123/lastLogin",
			Value: time.Now().Format(time.RFC3339),
		},
	}

	deltaEvent := events.NewStateDeltaEvent(deltaOps)
	if err := handler.HandleStateDelta(deltaEvent); err != nil {
		log.Fatal(err)
	}

	// Print current state
	currentState := store.GetState()
	jsonBytes, _ := json.MarshalIndent(currentState, "", "  ")
	fmt.Println(string(jsonBytes))
}

// Example_withCallbacks demonstrates using event handlers with callbacks
func Example_withCallbacks() {
	store := state.NewStateStore()

	// Create handler with callbacks
	handler := state.NewStateEventHandler(store,
		state.WithSnapshotCallback(func(event *events.StateSnapshotEvent) error {
			fmt.Println("Snapshot received!")
			return nil
		}),
		state.WithDeltaCallback(func(event *events.StateDeltaEvent) error {
			fmt.Printf("Delta received with %d operations\n", len(event.Delta))
			return nil
		}),
		state.WithStateChangeCallback(func(change state.StateChange) {
			fmt.Printf("State changed at path: %s\n", change.Path)
		}),
	)

	// Process events
	snapshot := events.NewStateSnapshotEvent(map[string]interface{}{
		"initial": "data",
	})
	handler.HandleStateSnapshot(snapshot)

	// Output:
	// Snapshot received!
	// State changed at path: /
}

// Example_eventGeneration demonstrates generating state events
func Example_eventGeneration() {
	// Create store and generator
	store := state.NewStateStore()
	generator := state.NewStateEventGenerator(store)

	// Set some initial state
	store.Set("/users/123", map[string]interface{}{
		"name": "John Doe",
		"role": "admin",
	})

	// Generate a snapshot event
	snapshotEvent, err := generator.GenerateSnapshot()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Generated snapshot event with state: %v\n", snapshotEvent.Snapshot)

	// Make some changes
	oldState := store.GetState()
	store.Set("/users/123/role", "superadmin")
	store.Set("/users/123/lastModified", time.Now().Format(time.RFC3339))
	newState := store.GetState()

	// Generate a delta event
	deltaEvent, err := generator.GenerateDelta(oldState, newState)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Generated delta event with %d operations\n", len(deltaEvent.Delta))
	for _, op := range deltaEvent.Delta {
		fmt.Printf("  - %s %s\n", op.Op, op.Path)
	}
}

// Example_streaming demonstrates real-time state event streaming
func Example_streaming() {
	// Create components
	store := state.NewStateStore()
	generator := state.NewStateEventGenerator(store)

	// Create stream with fast updates for demo
	stream := state.NewStateEventStream(store, generator,
		state.WithStreamInterval(100*time.Millisecond),
		state.WithDeltaOnly(false),
	)

	// Subscribe to events
	unsubscribe := stream.Subscribe(func(event events.Event) error {
		switch e := event.(type) {
		case *events.StateSnapshotEvent:
			fmt.Println("Received snapshot event")
		case *events.StateDeltaEvent:
			fmt.Printf("Received delta event with %d changes\n", len(e.Delta))
		}
		return nil
	})
	defer unsubscribe()

	// Start streaming
	if err := stream.Start(); err != nil {
		log.Fatal(err)
	}
	defer stream.Stop()

	// Make some state changes
	time.Sleep(150 * time.Millisecond) // Wait for initial snapshot

	store.Set("/counter", 1)
	time.Sleep(150 * time.Millisecond)

	store.Set("/counter", 2)
	time.Sleep(150 * time.Millisecond)

	// Output:
	// Received snapshot event
	// Received delta event with 1 changes
	// Received delta event with 1 changes
}

// Example_batchProcessing demonstrates batched delta processing
func Example_batchProcessing() {
	store := state.NewStateStore()

	// Create handler with batching configuration
	handler := state.NewStateEventHandler(store,
		state.WithBatchSize(5),
		state.WithBatchTimeout(200*time.Millisecond),
		state.WithDeltaCallback(func(event *events.StateDeltaEvent) error {
			fmt.Printf("Batch processed with %d operations\n", len(event.Delta))
			return nil
		}),
	)

	// Send multiple small deltas
	for i := 0; i < 3; i++ {
		delta := events.NewStateDeltaEvent([]events.JSONPatchOperation{
			{
				Op:    "add",
				Path:  fmt.Sprintf("/item%d", i),
				Value: fmt.Sprintf("value%d", i),
			},
		})
		handler.HandleStateDelta(delta)
	}

	// Wait for batch timeout
	time.Sleep(250 * time.Millisecond)

	// Output:
	// Batch processed with 3 operations
}

// Example_errorHandling demonstrates error handling and recovery
func Example_errorHandling() {
	store := state.NewStateStore()

	// Set initial state
	store.Set("/important", "data")

	// Create handler with error callbacks
	handler := state.NewStateEventHandler(store,
		state.WithSnapshotCallback(func(event *events.StateSnapshotEvent) error {
			// Simulate an error condition
			if event.Snapshot == nil {
				return fmt.Errorf("invalid snapshot")
			}
			return nil
		}),
	)

	// Try to apply an invalid snapshot
	invalidEvent := &events.StateSnapshotEvent{
		BaseEvent: events.NewBaseEvent(events.EventTypeStateSnapshot),
		Snapshot:  nil,
	}

	err := handler.HandleStateSnapshot(invalidEvent)
	if err != nil {
		fmt.Printf("Error handled: %v\n", err)
	}

	// Verify state was preserved
	value, _ := store.Get("/important")
	fmt.Printf("State preserved: %v\n", value)

	// Output:
	// Error handled: invalid snapshot event: StateSnapshotEvent validation failed: snapshot field is required
	// State preserved: data
}

// Example_metrics demonstrates using state metrics
func Example_metrics() {
	store := state.NewStateStore()
	handler := state.NewStateEventHandler(store,
		state.WithBatchSize(1),
		state.WithBatchTimeout(10*time.Millisecond),
	)

	// Process some events
	for i := 0; i < 3; i++ {
		snapshot := events.NewStateSnapshotEvent(map[string]interface{}{
			"iteration": i,
		})
		handler.HandleStateSnapshot(snapshot)
	}

	// Process some deltas
	for i := 0; i < 5; i++ {
		delta := events.NewStateDeltaEvent([]events.JSONPatchOperation{
			{Op: "add", Path: fmt.Sprintf("/delta%d", i), Value: i},
		})
		handler.HandleStateDelta(delta)
		time.Sleep(15 * time.Millisecond) // Ensure processing
	}

	// Get metrics (would need to expose handler.metrics in real usage)
	// This is just for demonstration
	fmt.Println("Events processed: snapshots=3, deltas=5")
}

// Example_transactionalUpdates demonstrates coordinated state updates
func Example_transactionalUpdates() {
	store := state.NewStateStore()
	generator := state.NewStateEventGenerator(store)

	// Initial state
	store.Set("/balance", 100)
	store.Set("/transactions", []interface{}{})

	// Simulate a transaction
	tx := store.Begin()

	// Get current values
	balance, _ := store.Get("/balance")
	transactions, _ := store.Get("/transactions")

	// Update balance
	newBalance := balance.(float64) - 25
	txList := transactions.([]interface{})
	txList = append(txList, map[string]interface{}{
		"amount": -25,
		"time":   time.Now().Format(time.RFC3339),
	})

	// Apply changes
	tx.Apply(state.JSONPatch{
		{Op: state.JSONPatchOpReplace, Path: "/balance", Value: newBalance},
		{Op: state.JSONPatchOpReplace, Path: "/transactions", Value: txList},
	})

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}

	// Generate delta event for the changes
	deltaEvent, _ := generator.GenerateDeltaFromCurrent()
	fmt.Printf("Transaction completed with %d changes\n", len(deltaEvent.Delta))
}

// Example_complexStateSync demonstrates synchronizing complex state structures
func Example_complexStateSync() {
	// Source and target stores
	sourceStore := state.NewStateStore()
	targetStore := state.NewStateStore()

	// Generator for source
	generator := state.NewStateEventGenerator(sourceStore)

	// Handler for target with sync callback
	syncHandler := state.NewStateEventHandler(targetStore,
		state.WithSnapshotCallback(func(event *events.StateSnapshotEvent) error {
			fmt.Println("Target synchronized with snapshot")
			return nil
		}),
		state.WithDeltaCallback(func(event *events.StateDeltaEvent) error {
			fmt.Printf("Target synchronized with %d changes\n", len(event.Delta))
			return nil
		}),
		state.WithBatchSize(1),
		state.WithBatchTimeout(10*time.Millisecond),
	)

	// Build complex state in source
	sourceStore.Set("/app/config", map[string]interface{}{
		"version": "1.0.0",
		"features": map[string]interface{}{
			"auth":     true,
			"payments": false,
		},
	})
	sourceStore.Set("/app/users", []interface{}{
		map[string]interface{}{"id": "1", "name": "Alice"},
		map[string]interface{}{"id": "2", "name": "Bob"},
	})

	// Generate and apply snapshot to sync initial state
	snapshot, _ := generator.GenerateSnapshot()
	syncHandler.HandleStateSnapshot(snapshot)

	// Make changes in source
	oldState := sourceStore.GetState()
	sourceStore.Set("/app/config/features/payments", true)
	sourceStore.Set("/app/users/2/name", "Robert")
	newState := sourceStore.GetState()

	// Generate and apply delta
	delta, _ := generator.GenerateDelta(oldState, newState)
	syncHandler.HandleStateDelta(delta)

	// Wait for processing
	time.Sleep(20 * time.Millisecond)

	// Verify synchronization
	sourceJSON, _ := sourceStore.Export()
	targetJSON, _ := targetStore.Export()

	fmt.Printf("States synchronized: %v\n", string(sourceJSON) == string(targetJSON))

	// Output:
	// Target synchronized with snapshot
	// Target synchronized with 2 changes
	// States synchronized: true
}
