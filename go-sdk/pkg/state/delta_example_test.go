package state_test

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/ag-ui/go-sdk/pkg/state"
)

func ExampleDeltaComputer_ComputeDelta() {
	// Create a delta computer
	dc := state.NewDeltaComputer(state.DefaultDeltaOptions())

	// Define old and new states
	oldState := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "John Doe",
			"age":  30,
			"email": "john@example.com",
		},
		"settings": map[string]interface{}{
			"theme": "light",
			"notifications": true,
		},
	}

	newState := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "John Doe",
			"age":  31, // Changed
			"email": "john.doe@example.com", // Changed
		},
		"settings": map[string]interface{}{
			"theme": "dark", // Changed
			"notifications": true,
			"language": "en", // Added
		},
	}

	// Compute the delta
	patch, err := dc.ComputeDelta(oldState, newState)
	if err != nil {
		log.Fatal(err)
	}

	// Print the patch operations
	fmt.Printf("Delta contains %d operations:\n", len(patch))
	for _, op := range patch {
		fmt.Printf("- %s %s", op.Op, op.Path)
		if op.Value != nil {
			val, _ := json.Marshal(op.Value)
			fmt.Printf(" = %s", val)
		}
		fmt.Println()
	}

	// Apply the patch to verify it works
	result, err := patch.Apply(oldState)
	if err != nil {
		log.Fatal(err)
	}

	// Verify the result matches the new state
	resultJSON, _ := json.Marshal(result)
	newStateJSON, _ := json.Marshal(newState)
	fmt.Printf("\nPatch applied successfully: %v\n", string(resultJSON) == string(newStateJSON))

	// Output:
	// Delta contains 4 operations:
	// - add /settings/language = "en"
	// - replace /settings/theme = "dark"
	// - replace /user/age = 31
	// - replace /user/email = "john.doe@example.com"
	//
	// Patch applied successfully: true
}

func ExampleDeltaHistory() {
	// Create a delta history tracker
	dh := state.NewDeltaHistory(100)

	// Simulate some state changes
	patch1 := state.JSONPatch{
		{Op: state.JSONPatchOpAdd, Path: "/users/alice", Value: map[string]interface{}{"id": 1, "name": "Alice"}},
	}
	id1 := dh.AddDelta(patch1, map[string]interface{}{"action": "user_created", "user": "admin"})

	patch2 := state.JSONPatch{
		{Op: state.JSONPatchOpReplace, Path: "/users/alice/name", Value: "Alice Smith"},
	}
	id2 := dh.AddDelta(patch2, map[string]interface{}{"action": "user_updated", "user": "admin"})

	// Retrieve a specific delta
	entry, err := dh.GetDelta(id1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Delta %s: %d operations at %s\n", entry.ID, len(entry.Patch), entry.Timestamp.Format("15:04:05"))

	// Replay deltas on an empty state
	baseState := map[string]interface{}{
		"users": map[string]interface{}{},
	}

	finalState, err := dh.ReplayDeltas(baseState, []string{id1, id2})
	if err != nil {
		log.Fatal(err)
	}

	// Print the final state
	finalJSON, _ := json.MarshalIndent(finalState, "", "  ")
	fmt.Printf("\nFinal state after replay:\n%s\n", finalJSON)

	// Get statistics
	stats := dh.Stats()
	fmt.Printf("\nHistory stats: %d total deltas, %d operations\n", stats["total_deltas"], stats["total_operations"])
}

func ExampleDeltaComputer_OptimizePatch() {
	dc := state.NewDeltaComputer(state.DefaultDeltaOptions())

	// Create a patch with redundant operations
	unoptimized := state.JSONPatch{
		{Op: state.JSONPatchOpRemove, Path: "/temp"},
		{Op: state.JSONPatchOpAdd, Path: "/temp", Value: "new value"},
		{Op: state.JSONPatchOpAdd, Path: "/a", Value: 1},
		{Op: state.JSONPatchOpAdd, Path: "/b", Value: 2},
		{Op: state.JSONPatchOpRemove, Path: "/b"},
	}

	fmt.Printf("Unoptimized patch: %d operations\n", len(unoptimized))

	// Optimize the patch
	optimized := dc.OptimizePatch(unoptimized)

	fmt.Printf("Optimized patch: %d operations\n", len(optimized))
	for _, op := range optimized {
		fmt.Printf("- %s %s", op.Op, op.Path)
		if op.Value != nil {
			fmt.Printf(" = %v", op.Value)
		}
		fmt.Println()
	}

	// Output:
	// Unoptimized patch: 5 operations
	// Optimized patch: 3 operations
	// - replace /temp = new value
	// - add /a = 1
	// - remove /b
}