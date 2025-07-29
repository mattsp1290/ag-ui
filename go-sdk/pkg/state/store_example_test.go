package state_test

import (
	"fmt"
	"log"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/state"
)

func ExampleStateStore() {
	// Create a new state store
	store := state.NewStateStore()
	defer store.Close()

	// Set some values
	store.Set("/users/123", map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   30,
	})

	// Get a value
	user, err := store.Get("/users/123")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("User: %+v\n", user)

	// Apply a JSON Patch
	patch := state.JSONPatch{
		{Op: state.JSONPatchOpReplace, Path: "/users/123/name", Value: "Jane Doe"},
		{Op: state.JSONPatchOpAdd, Path: "/users/123/city", Value: "New York"},
	}
	if err := store.ApplyPatch(patch); err != nil {
		log.Fatal(err)
	}

	// Subscribe to changes
	unsubscribe := store.Subscribe("/users", func(change state.StateChange) {
		fmt.Printf("Change detected - Path: %s, Operation: %s\n", change.Path, change.Operation)
	})
	defer unsubscribe()

	// Use transactions for atomic updates
	tx := store.Begin()
	tx.Apply(state.JSONPatch{
		{Op: state.JSONPatchOpAdd, Path: "/users/456", Value: map[string]interface{}{
			"name": "Alice Smith",
		}},
	})
	tx.Commit()

	// Create a snapshot
	snapshot, _ := store.CreateSnapshot()

	// Make more changes
	store.Delete("/users/123")

	// Restore from snapshot
	store.RestoreSnapshot(snapshot)

	// Get version history
	history, _ := store.GetHistory()
	fmt.Printf("Number of versions: %d\n", len(history))

	// Export state as JSON
	jsonData, _ := store.Export()
	fmt.Printf("Exported state: %s\n", string(jsonData))
}

func ExampleStateStore_transaction() {
	store := state.NewStateStore()
	defer store.Close()

	// Initialize with some data
	store.Set("/inventory", map[string]interface{}{
		"items": map[string]interface{}{
			"apple":  10,
			"orange": 5,
		},
	})

	// Start a transaction
	tx := store.Begin()

	// Apply multiple operations atomically
	patch := state.JSONPatch{
		{Op: state.JSONPatchOpReplace, Path: "/inventory/items/apple", Value: 8},
		{Op: state.JSONPatchOpReplace, Path: "/inventory/items/orange", Value: 7},
		{Op: state.JSONPatchOpAdd, Path: "/inventory/items/banana", Value: 12},
	}

	if err := tx.Apply(patch); err != nil {
		// If any operation fails, rollback
		tx.Rollback()
		log.Fatal(err)
	}

	// Commit all changes at once
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}

	// Verify the changes
	inventory, _ := store.Get("/inventory")
	fmt.Printf("Updated inventory: %+v\n", inventory)
}

func ExampleStateStore_subscriptions() {
	store := state.NewStateStore()
	defer store.Close()

	// Subscribe to specific paths
	store.Subscribe("/config/features/*", func(change state.StateChange) {
		fmt.Printf("Feature changed: %s -> %v\n", change.Path, change.NewValue)
	})

	// Enable a feature
	store.Set("/config/features/darkMode", true)
	store.Set("/config/features/notifications", false)

	// Subscribe to all user changes
	store.Subscribe("/users", func(change state.StateChange) {
		switch change.Operation {
		case "add":
			fmt.Printf("New user added at %s\n", change.Path)
		case "remove":
			fmt.Printf("User removed from %s\n", change.Path)
		case "replace":
			fmt.Printf("User updated at %s\n", change.Path)
		}
	})

	// Add and update users
	store.Set("/users/123", map[string]interface{}{"name": "John"})
	store.Set("/users/123/name", "John Doe")
	store.Delete("/users/123")
}
