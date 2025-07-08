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
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/state"
	"github.com/ag-ui/go-sdk/pkg/tools"
)

// ApplicationState represents our example application state
type ApplicationState struct {
	User        UserInfo               `json:"user"`
	Settings    Settings               `json:"settings"`
	LastUpdated time.Time              `json:"lastUpdated"`
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
	Theme         string `json:"theme"`
	Language      string `json:"language"`
	Notifications bool   `json:"notifications"`
}

func main() {
	// Create a context with timeout for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create error handler for standardized error handling
	errorHandler := tools.NewErrorHandler()
	
	// Add error listener for logging
	errorHandler.AddListener(func(err *tools.ToolError) {
		log.Printf("[%s] %s: %v", err.Type, err.Code, err.Message)
		if err.Details != nil && len(err.Details) > 0 {
			log.Printf("  Details: %+v", err.Details)
		}
	})

	// Add recovery strategy for timeout errors
	errorHandler.SetRecoveryStrategy(tools.ErrorTypeTimeout, func(ctx context.Context, err *tools.ToolError) error {
		log.Printf("Timeout occurred: %v - attempting to recover", err)
		// In a real application, you might want to save state or clean up resources
		return err
	})

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
			Theme:         "dark",
			Language:      "en",
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
		toolErr := errorHandler.HandleError(err, "state-sync-example")
		log.Fatalf("Failed to set initial state: %v", toolErr)
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
	eventHandler := createStateEventHandler(store, errorHandler)

	// Simulate state changes
	fmt.Println("\n=== Simulating State Changes ===")

	// Update user name
	fmt.Println("\n1. Updating user name...")
	// Check context before operation
	if err := ctx.Err(); err != nil {
		errorHandler.HandleError(err, "state-sync-example")
		return
	}
	
	if err := store.Set("/user/name", "Jane Smith"); err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"STATE_UPDATE_FAILED",
			"failed to update user name",
		).WithToolID("state-sync-example").
			WithCause(err).
			WithDetail("path", "/user/name").
			WithDetail("value", "Jane Smith")
		
		errorHandler.HandleError(toolErr, "state-sync-example")
	}
	time.Sleep(100 * time.Millisecond)

	// Update settings
	fmt.Println("\n2. Updating theme setting...")
	if err := store.Set("/settings/theme", "light"); err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"STATE_UPDATE_FAILED",
			"failed to update theme setting",
		).WithToolID("state-sync-example").
			WithCause(err).
			WithDetail("path", "/settings/theme").
			WithDetail("value", "light")
		
		errorHandler.HandleError(toolErr, "state-sync-example")
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
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"TRANSACTION_APPLY_FAILED",
			"failed to apply patch in transaction",
		).WithToolID("state-sync-example").
			WithCause(err).
			WithDetail("patch_operations", len(patch))
		
		errorHandler.HandleError(toolErr, "state-sync-example")
		
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			rollbackToolErr := tools.NewToolError(
				tools.ErrorTypeExecution,
				"TRANSACTION_ROLLBACK_FAILED",
				"failed to rollback transaction",
			).WithToolID("state-sync-example").
				WithCause(rollbackErr)
			
			errorHandler.HandleError(rollbackToolErr, "state-sync-example")
		}
	} else {
		if err := tx.Commit(); err != nil {
			toolErr := tools.NewToolError(
				tools.ErrorTypeExecution,
				"TRANSACTION_COMMIT_FAILED",
				"failed to commit transaction",
			).WithToolID("state-sync-example").
				WithCause(err)
			
			errorHandler.HandleError(toolErr, "state-sync-example")
		}
	}
	time.Sleep(100 * time.Millisecond)

	// Demonstrate snapshot and restore
	fmt.Println("\n=== Snapshot and Restore Demo ===")

	// Create a snapshot
	snapshot, err := store.CreateSnapshot()
	if err != nil {
		toolErr := errorHandler.HandleError(err, "state-sync-example")
		log.Fatalf("Failed to create snapshot: %v", toolErr)
	}
	fmt.Printf("Created snapshot: %s\n", snapshot.ID)

	// Make more changes
	fmt.Println("\nMaking additional changes...")
	if err := store.Set("/user/isActive", false); err != nil {
		errorHandler.HandleError(err, "state-sync-example")
	}
	if err := store.Set("/settings/language", "es"); err != nil {
		errorHandler.HandleError(err, "state-sync-example")
	}
	time.Sleep(100 * time.Millisecond)

	// Show current state
	currentState, err := store.Get("/")
	if err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"STATE_GET_FAILED",
			"failed to get current state",
		).WithToolID("state-sync-example").
			WithCause(err).
			WithDetail("path", "/")
		
		errorHandler.HandleError(toolErr, "state-sync-example")
	} else {
		fmt.Println("\nCurrent state after changes:")
		printJSON(currentState)
	}

	// Restore from snapshot
	fmt.Println("\nRestoring from snapshot...")
	if err := store.RestoreSnapshot(snapshot); err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"SNAPSHOT_RESTORE_FAILED",
			"failed to restore snapshot",
		).WithToolID("state-sync-example").
			WithCause(err).
			WithDetail("snapshot_id", snapshot.ID)
		
		errorHandler.HandleError(toolErr, "state-sync-example")
	}
	time.Sleep(100 * time.Millisecond)

	// Show restored state
	restoredState, err := store.Get("/")
	if err != nil {
		errorHandler.HandleError(err, "state-sync-example")
	} else {
		fmt.Println("\nState after restore:")
		printJSON(restoredState)
	}

	// Demonstrate event-based synchronization
	fmt.Println("\n=== Event-Based Synchronization Demo ===")

	// Generate and handle a state snapshot event
	generator := state.NewStateEventGenerator(store)

	snapshotEvent, err := generator.GenerateSnapshot()
	if err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"SNAPSHOT_EVENT_GENERATION_FAILED",
			"failed to generate snapshot event",
		).WithToolID("state-sync-example").
			WithCause(err)
		
		errorHandler.HandleError(toolErr, "state-sync-example")
	} else {
		fmt.Println("\nGenerated snapshot event:")
		fmt.Printf("Event Type: %s\n", snapshotEvent.Type())
		if ts := snapshotEvent.Timestamp(); ts != nil {
			fmt.Printf("Timestamp: %s\n", time.UnixMilli(*ts).Format(time.RFC3339))
		}

		// Handle the snapshot event (simulate receiving it)
		if err := eventHandler.HandleStateSnapshot(snapshotEvent); err != nil {
			toolErr := tools.NewToolError(
				tools.ErrorTypeExecution,
				"SNAPSHOT_EVENT_HANDLE_FAILED",
				"failed to handle snapshot event",
			).WithToolID("state-sync-example").
				WithCause(err).
				WithDetail("event_type", snapshotEvent.Type())
			
			errorHandler.HandleError(toolErr, "state-sync-example")
		}
	}

	// Make a change and generate delta event
	fmt.Println("\nMaking change for delta event...")
	oldState := store.GetState()
	if err := store.Set("/user/name", "John Updated"); err != nil {
		errorHandler.HandleError(err, "state-sync-example")
	}
	newState := store.GetState()

	deltaEvent, err := generator.GenerateDelta(oldState, newState)
	if err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"DELTA_EVENT_GENERATION_FAILED",
			"failed to generate delta event",
		).WithToolID("state-sync-example").
			WithCause(err)
		
		errorHandler.HandleError(toolErr, "state-sync-example")
	} else {
		fmt.Println("\nGenerated delta event:")
		fmt.Printf("Event Type: %s\n", deltaEvent.Type())
		fmt.Printf("Delta operations: %d\n", len(deltaEvent.Delta))
		for _, op := range deltaEvent.Delta {
			fmt.Printf("  - %s %s\n", op.Op, op.Path)
		}
	}

	// Show state history
	fmt.Println("\n=== State History ===")
	history, err := store.GetHistory()
	if err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"HISTORY_GET_FAILED",
			"failed to get state history",
		).WithToolID("state-sync-example").
			WithCause(err)
		
		errorHandler.HandleError(toolErr, "state-sync-example")
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
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"STATE_EXPORT_FAILED",
			"failed to export state",
		).WithToolID("state-sync-example").
			WithCause(err)
		
		errorHandler.HandleError(toolErr, "state-sync-example")
	} else {
		fmt.Println("Exported state:")
		fmt.Println(string(exported))
		
		// Clear store and import
		fmt.Println("\nClearing store and importing...")
		store.Clear()

		if err := store.Import(exported); err != nil {
			toolErr := tools.NewToolError(
				tools.ErrorTypeExecution,
				"STATE_IMPORT_FAILED",
				"failed to import state",
			).WithToolID("state-sync-example").
				WithCause(err).
				WithDetail("export_size", len(exported))
			
			errorHandler.HandleError(toolErr, "state-sync-example")
		} else {
			fmt.Println("Successfully imported state")
		}
	}

	// Final state
	finalState, err := store.Get("/")
	if err != nil {
		errorHandler.HandleError(err, "state-sync-example")
	} else {
		fmt.Println("\nFinal imported state:")
		printJSON(finalState)
	}

	// Show performance stats
	fmt.Println("\n=== Performance Summary ===")
	fmt.Printf("Current version: %d\n", store.GetVersion())
	if history != nil {
		fmt.Printf("History entries: %d\n", len(history))
	}
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
func createStateEventHandler(store *state.StateStore, errorHandler *tools.ErrorHandler) *state.StateEventHandler {
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
		// For helper functions, we'll use simple error handling
		log.Printf("Failed to marshal JSON: %v", err)
		return
	}
	fmt.Println(string(data))
}
