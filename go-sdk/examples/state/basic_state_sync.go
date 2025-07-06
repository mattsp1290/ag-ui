// Package main demonstrates basic state synchronization between agent and client
// using the AG-UI state management system.
//
// This example shows:
// - Creating and managing a state store
// - Subscribing to state changes
// - Handling state events (snapshots and deltas)
// - Basic synchronization patterns
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/state"
)

// ApplicationState represents our example application state
type ApplicationState struct {
	User        UserInfo              `json:"user"`
	Settings    Settings              `json:"settings"`
	LastUpdated time.Time             `json:"lastUpdated"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// UserInfo contains user information
type UserInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	IsActive bool   `json:"isActive"`
}

// Settings contains application settings
type Settings struct {
	Theme        string `json:"theme"`
	Language     string `json:"language"`
	Notifications bool   `json:"notifications"`
}

func main() {
	// Create a state store with history tracking
	store := state.NewStateStore(
		state.WithMaxHistory(100), // Keep last 100 state versions
	)

	// Initialize application state
	initialState := ApplicationState{
		User: UserInfo{
			ID:       "user-123",
			Name:     "John Doe",
			Email:    "john@example.com",
			IsActive: true,
		},
		Settings: Settings{
			Theme:        "dark",
			Language:     "en",
			Notifications: true,
		},
		LastUpdated: time.Now(),
		Metadata: map[string]interface{}{
			"version": "1.0.0",
			"source":  "client",
		},
	}

	// Set initial state
	if err := setApplicationState(store, initialState); err != nil {
		log.Fatal("Failed to set initial state:", err)
	}

	// Subscribe to all state changes
	fmt.Println("=== Subscribing to State Changes ===")
	unsubscribe := store.Subscribe("/", func(change state.StateChange) {
		fmt.Printf("State changed at path: %s\n", change.Path)
		fmt.Printf("  Operation: %s\n", change.Operation)
		fmt.Printf("  Old Value: %v\n", change.OldValue)
		fmt.Printf("  New Value: %v\n", change.NewValue)
		fmt.Printf("  Timestamp: %s\n\n", change.Timestamp.Format(time.RFC3339))
	})
	defer unsubscribe()

	// Subscribe to specific path (user settings)
	unsubSettings := store.Subscribe("/settings", func(change state.StateChange) {
		fmt.Printf("Settings changed: %+v\n", change.NewValue)
	})
	defer unsubSettings()

	// Create state event handlers for synchronization
	eventHandler := createStateEventHandler(store)

	// Simulate state changes
	fmt.Println("\n=== Simulating State Changes ===")
	
	// Update user name
	fmt.Println("\n1. Updating user name...")
	if err := store.Set("/user/name", "Jane Smith"); err != nil {
		log.Printf("Failed to update user name: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Update settings
	fmt.Println("\n2. Updating theme setting...")
	if err := store.Set("/settings/theme", "light"); err != nil {
		log.Printf("Failed to update theme: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Apply a batch of changes using transaction
	fmt.Println("\n3. Applying batch changes using transaction...")
	tx := store.Begin()
	
	patch := state.JSONPatch{
		{Op: state.JSONPatchOpReplace, Path: "/user/email", Value: "jane@example.com"},
		{Op: state.JSONPatchOpReplace, Path: "/settings/notifications", Value: false},
		{Op: state.JSONPatchOpAdd, Path: "/metadata/lastSync", Value: time.Now().Unix()},
	}
	
	if err := tx.Apply(patch); err != nil {
		log.Printf("Failed to apply patch in transaction: %v", err)
		tx.Rollback()
	} else {
		if err := tx.Commit(); err != nil {
			log.Printf("Failed to commit transaction: %v", err)
		}
	}
	time.Sleep(100 * time.Millisecond)

	// Demonstrate snapshot and restore
	fmt.Println("\n=== Snapshot and Restore Demo ===")
	
	// Create a snapshot
	snapshot, err := store.CreateSnapshot()
	if err != nil {
		log.Fatal("Failed to create snapshot:", err)
	}
	fmt.Printf("Created snapshot: %s\n", snapshot.ID)

	// Make more changes
	fmt.Println("\nMaking additional changes...")
	store.Set("/user/isActive", false)
	store.Set("/settings/language", "es")
	time.Sleep(100 * time.Millisecond)

	// Show current state
	currentState, _ := store.Get("/")
	fmt.Println("\nCurrent state after changes:")
	printJSON(currentState)

	// Restore from snapshot
	fmt.Println("\nRestoring from snapshot...")
	if err := store.RestoreSnapshot(snapshot); err != nil {
		log.Printf("Failed to restore snapshot: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Show restored state
	restoredState, _ := store.Get("/")
	fmt.Println("\nState after restore:")
	printJSON(restoredState)

	// Demonstrate event-based synchronization
	fmt.Println("\n=== Event-Based Synchronization Demo ===")
	
	// Generate and handle a state snapshot event
	generator := state.NewStateEventGenerator(store)
	
	snapshotEvent, err := generator.GenerateSnapshot()
	if err != nil {
		log.Printf("Failed to generate snapshot event: %v", err)
	} else {
		fmt.Println("\nGenerated snapshot event:")
		fmt.Printf("Event ID: %s\n", snapshotEvent.ID)
		fmt.Printf("Timestamp: %s\n", snapshotEvent.Timestamp.Format(time.RFC3339))
		
		// Handle the snapshot event (simulate receiving it)
		if err := eventHandler.HandleStateSnapshot(snapshotEvent); err != nil {
			log.Printf("Failed to handle snapshot event: %v", err)
		}
	}

	// Make a change and generate delta event
	fmt.Println("\nMaking change for delta event...")
	oldState := store.GetState()
	store.Set("/user/name", "John Updated")
	newState := store.GetState()
	
	deltaEvent, err := generator.GenerateDelta(oldState, newState)
	if err != nil {
		log.Printf("Failed to generate delta event: %v", err)
	} else {
		fmt.Println("\nGenerated delta event:")
		fmt.Printf("Event ID: %s\n", deltaEvent.ID)
		fmt.Printf("Delta operations: %d\n", len(deltaEvent.Delta))
		for _, op := range deltaEvent.Delta {
			fmt.Printf("  - %s %s\n", op.Op, op.Path)
		}
	}

	// Show state history
	fmt.Println("\n=== State History ===")
	history, err := store.GetHistory()
	if err != nil {
		log.Printf("Failed to get history: %v", err)
	} else {
		fmt.Printf("Total versions: %d\n", len(history))
		// Show last 5 versions
		start := len(history) - 5
		if start < 0 {
			start = 0
		}
		for i := start; i < len(history); i++ {
			version := history[i]
			fmt.Printf("\nVersion %d (ID: %s):\n", i+1, version.ID[:8])
			fmt.Printf("  Timestamp: %s\n", version.Timestamp.Format(time.RFC3339))
			if version.Delta != nil && len(version.Delta) > 0 {
				fmt.Printf("  Changes: %d operations\n", len(version.Delta))
			}
		}
	}

	// Export and import state
	fmt.Println("\n=== Export/Import Demo ===")
	
	// Export current state
	exported, err := store.Export()
	if err != nil {
		log.Printf("Failed to export state: %v", err)
	} else {
		fmt.Println("Exported state:")
		fmt.Println(string(exported))
	}

	// Clear store and import
	fmt.Println("\nClearing store and importing...")
	store.Clear()
	
	if err := store.Import(exported); err != nil {
		log.Printf("Failed to import state: %v", err)
	} else {
		fmt.Println("Successfully imported state")
	}

	// Final state
	finalState, _ := store.Get("/")
	fmt.Println("\nFinal imported state:")
	printJSON(finalState)

	// Show performance stats
	fmt.Println("\n=== Performance Summary ===")
	fmt.Printf("Current version: %d\n", store.GetVersion())
	fmt.Printf("History entries: %d\n", len(history))
}

// Helper function to set application state
func setApplicationState(store *state.StateStore, appState ApplicationState) error {
	// Convert to map for storage
	data, err := json.Marshal(appState)
	if err != nil {
		return err
	}

	var stateMap map[string]interface{}
	if err := json.Unmarshal(data, &stateMap); err != nil {
		return err
	}

	// Set each top-level key
	for key, value := range stateMap {
		if err := store.Set("/"+key, value); err != nil {
			return err
		}
	}

	return nil
}

// Create state event handler for synchronization
func createStateEventHandler(store *state.StateStore) *state.StateEventHandler {
	return state.NewStateEventHandler(
		store,
		state.WithBatchSize(10),
		state.WithBatchTimeout(50*time.Millisecond),
		state.WithSnapshotCallback(func(event *events.StateSnapshotEvent) error {
			fmt.Println("Snapshot event received - state synchronized")
			return nil
		}),
		state.WithDeltaCallback(func(event *events.StateDeltaEvent) error {
			fmt.Printf("Delta event received - %d changes applied\n", len(event.Delta))
			return nil
		}),
		state.WithStateChangeCallback(func(change state.StateChange) {
			// This could send the change to a remote system
			fmt.Printf("State change callback: %s changed\n", change.Path)
		}),
	)
}

// Helper function to pretty print JSON
func printJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}