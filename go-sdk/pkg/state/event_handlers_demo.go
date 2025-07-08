//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/state"
)

func main() {
	// Create a state store
	store := state.NewStateStore()

	// Create event handler with callbacks
	handler := state.NewStateEventHandler(store,
		state.WithBatchSize(5),
		state.WithBatchTimeout(100*time.Millisecond),
		state.WithSnapshotCallback(func(event *events.StateSnapshotEvent) error {
			fmt.Println("📸 Snapshot received")
			return nil
		}),
		state.WithDeltaCallback(func(event *events.StateDeltaEvent) error {
			fmt.Printf("🔄 Delta batch with %d operations\n", len(event.Delta))
			return nil
		}),
		state.WithStateChangeCallback(func(change state.StateChange) {
			fmt.Printf("📝 State changed: %s (%s)\n", change.Path, change.Operation)
		}),
	)

	// Create event generator
	generator := state.NewStateEventGenerator(store)

	fmt.Println("=== State Event Handler Demo ===\n")

	// 1. Apply initial state via snapshot
	fmt.Println("1️⃣ Applying initial snapshot...")
	initialState := map[string]interface{}{
		"users": map[string]interface{}{
			"user1": map[string]interface{}{
				"name":   "Alice",
				"role":   "admin",
				"active": true,
			},
			"user2": map[string]interface{}{
				"name":   "Bob",
				"role":   "user",
				"active": true,
			},
		},
		"config": map[string]interface{}{
			"theme":    "dark",
			"language": "en",
		},
	}

	snapshotEvent := events.NewStateSnapshotEvent(initialState)
	if err := handler.HandleStateSnapshot(snapshotEvent); err != nil {
		log.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond) // Let callbacks process
	printState(store)

	// 2. Apply incremental changes via deltas
	fmt.Println("\n2️⃣ Applying delta changes...")

	// Single delta
	delta1 := events.NewStateDeltaEvent([]events.JSONPatchOperation{
		{Op: "replace", Path: "/users/user1/role", Value: "superadmin"},
	})
	handler.HandleStateDelta(delta1)

	// Multiple deltas (will batch)
	for i := 0; i < 3; i++ {
		delta := events.NewStateDeltaEvent([]events.JSONPatchOperation{
			{Op: "add", Path: fmt.Sprintf("/logs/entry%d", i), Value: fmt.Sprintf("Log entry %d", i)},
		})
		handler.HandleStateDelta(delta)
	}

	time.Sleep(200 * time.Millisecond) // Wait for batch processing
	printState(store)

	// 3. Generate events from state changes
	fmt.Println("\n3️⃣ Generating events from state changes...")

	// Capture current state
	beforeState := store.GetState()

	// Make direct changes to store
	store.Set("/users/user3", map[string]interface{}{
		"name":   "Charlie",
		"role":   "user",
		"active": false,
	})
	store.Set("/config/notifications", true)

	// Generate delta event for the changes
	afterState := store.GetState()
	deltaEvent, err := generator.GenerateDelta(beforeState, afterState)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Generated delta with %d operations:\n", len(deltaEvent.Delta))
	for _, op := range deltaEvent.Delta {
		fmt.Printf("  - %s %s\n", op.Op, op.Path)
	}

	// 4. Real-time streaming
	fmt.Println("\n4️⃣ Starting real-time event stream...")

	stream := state.NewStateEventStream(store, generator,
		state.WithStreamInterval(500*time.Millisecond),
		state.WithDeltaOnly(true),
	)

	// Subscribe to stream
	unsubscribe := stream.Subscribe(func(event events.Event) error {
		switch e := event.(type) {
		case *events.StateDeltaEvent:
			if len(e.Delta) > 0 {
				fmt.Printf("📡 Stream: %d changes detected\n", len(e.Delta))
			}
		}
		return nil
	})
	defer unsubscribe()

	// Start streaming
	stream.Start()
	defer stream.Stop()

	// Make some changes while streaming
	time.Sleep(600 * time.Millisecond)
	store.Set("/streaming/test1", "value1")

	time.Sleep(600 * time.Millisecond)
	store.Set("/streaming/test2", "value2")

	time.Sleep(600 * time.Millisecond)

	// 5. Show final state
	fmt.Println("\n5️⃣ Final state:")
	printState(store)
}

func printState(store *state.StateStore) {
	state := store.GetState()
	jsonBytes, _ := json.MarshalIndent(state, "  ", "  ")
	fmt.Printf("  %s\n", string(jsonBytes))
}
