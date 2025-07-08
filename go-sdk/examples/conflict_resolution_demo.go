package main

import (
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/state"
)

func main() {
	fmt.Println("AG-UI Conflict Resolution System Demo")
	fmt.Println("=====================================")

	// Demonstrate conflict detection
	demonstrateConflictDetection()

	// Demonstrate different resolution strategies
	demonstrateLastWriteWins()
	demonstrateFirstWriteWins()
	demonstrateMergeStrategy()
	demonstrateCustomStrategy()

	// Demonstrate conflict history and analytics
	demonstrateConflictHistory()
}

func demonstrateConflictDetection() {
	fmt.Println("\n1. Conflict Detection Demo")
	fmt.Println("--------------------------")

	detector := state.NewConflictDetector(state.DefaultConflictDetectorOptions())

	// Create two conflicting changes
	local := &state.StateChange{
		Path:      "/user/profile",
		OldValue:  map[string]interface{}{"name": "Alice", "age": 30},
		NewValue:  map[string]interface{}{"name": "Alice Smith", "age": 30},
		Operation: "replace",
		Timestamp: time.Now(),
	}

	remote := &state.StateChange{
		Path:      "/user/profile",
		OldValue:  map[string]interface{}{"name": "Alice", "age": 30},
		NewValue:  map[string]interface{}{"name": "Alice Johnson", "age": 31},
		Operation: "replace",
		Timestamp: time.Now().Add(time.Second),
	}

	conflict, err := detector.DetectConflict(local, remote)
	if err != nil {
		log.Fatal(err)
	}

	if conflict != nil {
		fmt.Printf("Conflict detected!\n")
		fmt.Printf("  Path: %s\n", conflict.Path)
		fmt.Printf("  Severity: %s\n", conflict.Severity.String())
		fmt.Printf("  Local change: %v\n", local.NewValue)
		fmt.Printf("  Remote change: %v\n", remote.NewValue)
	}
}

func demonstrateLastWriteWins() {
	fmt.Println("\n2. Last Write Wins Strategy")
	fmt.Println("---------------------------")

	resolver := state.NewConflictResolver(state.LastWriteWins)

	// Create a conflict where remote has later timestamp
	conflict := &state.StateConflict{
		ID:        "demo-conflict-1",
		Timestamp: time.Now(),
		Path:      "/config/setting",
		LocalChange: &state.StateChange{
			Path:      "/config/setting",
			NewValue:  "local-value",
			Timestamp: time.Now(),
		},
		RemoteChange: &state.StateChange{
			Path:      "/config/setting",
			NewValue:  "remote-value",
			Timestamp: time.Now().Add(time.Hour), // Remote is later
		},
	}

	resolution, err := resolver.Resolve(conflict)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Resolution:\n")
	fmt.Printf("  Strategy: %s\n", resolution.Strategy)
	fmt.Printf("  Winner: %s\n", resolution.WinningChange)
	fmt.Printf("  Resolved value: %v\n", resolution.ResolvedValue)
}

func demonstrateFirstWriteWins() {
	fmt.Println("\n3. First Write Wins Strategy")
	fmt.Println("----------------------------")

	resolver := state.NewConflictResolver(state.FirstWriteWins)

	// Create a conflict where local has earlier timestamp
	conflict := &state.StateConflict{
		ID:        "demo-conflict-2",
		Timestamp: time.Now(),
		Path:      "/data/item",
		LocalChange: &state.StateChange{
			Path:      "/data/item",
			NewValue:  "first-value",
			Timestamp: time.Now(), // Local is earlier
		},
		RemoteChange: &state.StateChange{
			Path:      "/data/item",
			NewValue:  "second-value",
			Timestamp: time.Now().Add(time.Hour),
		},
	}

	resolution, err := resolver.Resolve(conflict)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Resolution:\n")
	fmt.Printf("  Strategy: %s\n", resolution.Strategy)
	fmt.Printf("  Winner: %s\n", resolution.WinningChange)
	fmt.Printf("  Resolved value: %v\n", resolution.ResolvedValue)
}

func demonstrateMergeStrategy() {
	fmt.Println("\n4. Merge Strategy")
	fmt.Println("-----------------")

	resolver := state.NewConflictResolver(state.MergeStrategy)

	// Create a conflict with map values that can be merged
	baseUser := map[string]interface{}{
		"id":    "123",
		"name":  "Original Name",
		"email": "original@example.com",
	}

	localUser := map[string]interface{}{
		"id":    "123",
		"name":  "Updated Name", // Changed
		"email": "original@example.com",
		"phone": "555-1234", // Added locally
	}

	remoteUser := map[string]interface{}{
		"id":      "123",
		"name":    "Different Name",  // Also changed (conflict!)
		"email":   "new@example.com", // Changed
		"address": "123 Main St",     // Added remotely
	}

	conflict := &state.StateConflict{
		ID:        "demo-conflict-3",
		Timestamp: time.Now(),
		Path:      "/users/123",
		LocalChange: &state.StateChange{
			Path:      "/users/123",
			OldValue:  baseUser,
			NewValue:  localUser,
			Operation: "replace",
			Timestamp: time.Now(),
		},
		RemoteChange: &state.StateChange{
			Path:      "/users/123",
			OldValue:  baseUser,
			NewValue:  remoteUser,
			Operation: "replace",
			Timestamp: time.Now(),
		},
		BaseValue: baseUser,
	}

	resolution, err := resolver.Resolve(conflict)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Resolution:\n")
	fmt.Printf("  Strategy: %s\n", resolution.Strategy)
	fmt.Printf("  Merged: %v\n", resolution.MergedChanges)
	fmt.Printf("  Resolved value: %v\n", resolution.ResolvedValue)
}

func demonstrateCustomStrategy() {
	fmt.Println("\n5. Custom Strategy")
	fmt.Println("------------------")

	resolver := state.NewConflictResolver(state.CustomStrategy)

	// Register a custom resolver that prefers higher numeric values
	resolver.RegisterCustomResolver("default", func(conflict *state.StateConflict) (*state.ConflictResolution, error) {
		// Custom logic: choose the higher value if both are numbers
		localNum, localOk := conflict.LocalChange.NewValue.(float64)
		remoteNum, remoteOk := conflict.RemoteChange.NewValue.(float64)

		var resolvedValue interface{}
		var winner string

		if localOk && remoteOk {
			if localNum > remoteNum {
				resolvedValue = localNum
				winner = "local"
			} else {
				resolvedValue = remoteNum
				winner = "remote"
			}
		} else {
			// Fallback to last write wins
			if conflict.LocalChange.Timestamp.After(conflict.RemoteChange.Timestamp) {
				resolvedValue = conflict.LocalChange.NewValue
				winner = "local"
			} else {
				resolvedValue = conflict.RemoteChange.NewValue
				winner = "remote"
			}
		}

		return &state.ConflictResolution{
			ID:            fmt.Sprintf("resolution-%d", time.Now().UnixNano()),
			ConflictID:    conflict.ID,
			Timestamp:     time.Now(),
			Strategy:      state.CustomStrategy,
			ResolvedValue: resolvedValue,
			ResolvedPatch: state.JSONPatch{state.JSONPatchOperation{
				Op:    state.JSONPatchOpReplace,
				Path:  conflict.Path,
				Value: resolvedValue,
			}},
			WinningChange: winner,
			MergedChanges: false,
		}, nil
	})

	// Create a conflict with numeric values
	conflict := &state.StateConflict{
		ID:        "demo-conflict-4",
		Timestamp: time.Now(),
		Path:      "/metrics/score",
		LocalChange: &state.StateChange{
			Path:      "/metrics/score",
			NewValue:  float64(85),
			Timestamp: time.Now(),
		},
		RemoteChange: &state.StateChange{
			Path:      "/metrics/score",
			NewValue:  float64(92),
			Timestamp: time.Now(),
		},
	}

	resolution, err := resolver.Resolve(conflict)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Resolution (custom - higher value wins):\n")
	fmt.Printf("  Strategy: %s\n", resolution.Strategy)
	fmt.Printf("  Winner: %s\n", resolution.WinningChange)
	fmt.Printf("  Resolved value: %v\n", resolution.ResolvedValue)
}

func demonstrateConflictHistory() {
	fmt.Println("\n6. Conflict History and Analytics")
	fmt.Println("---------------------------------")

	history := state.NewConflictHistory(100)
	analyzer := state.NewConflictAnalyzer(history)

	// Simulate some conflicts
	paths := []string{
		"/hot/path", "/hot/path", "/hot/path", // This path has many conflicts
		"/normal/path", "/normal/path",
		"/cold/path",
	}

	for i, path := range paths {
		conflict := &state.StateConflict{
			ID:        fmt.Sprintf("history-conflict-%d", i),
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Path:      path,
			Severity:  state.SeverityMedium,
		}
		history.RecordConflict(conflict)

		// Simulate resolution
		resolution := &state.ConflictResolution{
			ID:         fmt.Sprintf("history-resolution-%d", i),
			ConflictID: conflict.ID,
			Timestamp:  time.Now().Add(time.Duration(i)*time.Minute + 30*time.Second),
			Strategy:   state.LastWriteWins,
		}
		history.RecordResolution(resolution)
	}

	// Get statistics
	stats := history.GetStatistics()
	fmt.Printf("Conflict Statistics:\n")
	fmt.Printf("  Total conflicts: %d\n", stats.TotalConflicts)
	fmt.Printf("  Total resolutions: %d\n", stats.TotalResolutions)
	fmt.Printf("  Conflicts by path:\n")
	for path, count := range stats.ConflictsByPath {
		fmt.Printf("    %s: %d conflicts\n", path, count)
	}

	// Analyze patterns
	analysis := analyzer.AnalyzePatterns()
	fmt.Printf("\nConflict Analysis:\n")
	fmt.Printf("  Resolution rate: %.2f\n", analysis["resolution_rate"])
	fmt.Printf("  Hot paths (above average conflicts):\n")
	if hotPaths, ok := analysis["hot_paths"].([]string); ok {
		for _, path := range hotPaths {
			fmt.Printf("    - %s\n", path)
		}
	}
}
