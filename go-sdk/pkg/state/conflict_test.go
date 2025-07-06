package state

import (
	"testing"
	"time"
)

func TestConflictDetection(t *testing.T) {
	detector := NewConflictDetector(DefaultConflictDetectorOptions())
	
	// Test case 1: No conflict - different paths
	local := &StateChange{
		Path:      "/users/123",
		OldValue:  map[string]interface{}{"name": "Alice"},
		NewValue:  map[string]interface{}{"name": "Alice Smith"},
		Operation: "replace",
		Timestamp: time.Now(),
	}
	
	remote := &StateChange{
		Path:      "/users/456",
		OldValue:  map[string]interface{}{"name": "Bob"},
		NewValue:  map[string]interface{}{"name": "Bob Jones"},
		Operation: "replace",
		Timestamp: time.Now(),
	}
	
	conflict, err := detector.DetectConflict(local, remote)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflict != nil {
		t.Error("expected no conflict for different paths")
	}
	
	// Test case 2: Conflict - same path, different values
	remote2 := &StateChange{
		Path:      "/users/123",
		OldValue:  map[string]interface{}{"name": "Alice"},
		NewValue:  map[string]interface{}{"name": "Alice Johnson"},
		Operation: "replace",
		Timestamp: time.Now().Add(time.Second),
	}
	
	conflict, err = detector.DetectConflict(local, remote2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflict == nil {
		t.Error("expected conflict for same path with different values")
	}
	
	// Test case 3: No conflict - same operation and value
	remote3 := &StateChange{
		Path:      "/users/123",
		OldValue:  map[string]interface{}{"name": "Alice"},
		NewValue:  map[string]interface{}{"name": "Alice Smith"},
		Operation: "replace",
		Timestamp: time.Now().Add(time.Second),
	}
	
	conflict, err = detector.DetectConflict(local, remote3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflict != nil {
		t.Error("expected no conflict for identical changes")
	}
}

func TestLastWriteWinsStrategy(t *testing.T) {
	resolver := NewConflictResolver(LastWriteWins)
	
	localTime := time.Now()
	remoteTime := localTime.Add(time.Second)
	
	conflict := &StateConflict{
		ID:        "test-conflict-1",
		Timestamp: time.Now(),
		Path:      "/data/value",
		LocalChange: &StateChange{
			Path:      "/data/value",
			OldValue:  "original",
			NewValue:  "local-update",
			Operation: "replace",
			Timestamp: localTime,
		},
		RemoteChange: &StateChange{
			Path:      "/data/value",
			OldValue:  "original",
			NewValue:  "remote-update",
			Operation: "replace",
			Timestamp: remoteTime,
		},
		BaseValue: "original",
		Severity:  SeverityMedium,
	}
	
	resolution, err := resolver.Resolve(conflict)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if resolution.WinningChange != "remote" {
		t.Errorf("expected remote to win, got %s", resolution.WinningChange)
	}
	
	if resolution.ResolvedValue != "remote-update" {
		t.Errorf("expected resolved value to be 'remote-update', got %v", resolution.ResolvedValue)
	}
}

func TestFirstWriteWinsStrategy(t *testing.T) {
	resolver := NewConflictResolver(FirstWriteWins)
	
	localTime := time.Now()
	remoteTime := localTime.Add(time.Second)
	
	conflict := &StateConflict{
		ID:        "test-conflict-2",
		Timestamp: time.Now(),
		Path:      "/data/value",
		LocalChange: &StateChange{
			Path:      "/data/value",
			OldValue:  "original",
			NewValue:  "local-update",
			Operation: "replace",
			Timestamp: localTime,
		},
		RemoteChange: &StateChange{
			Path:      "/data/value",
			OldValue:  "original",
			NewValue:  "remote-update",
			Operation: "replace",
			Timestamp: remoteTime,
		},
		BaseValue: "original",
		Severity:  SeverityMedium,
	}
	
	resolution, err := resolver.Resolve(conflict)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if resolution.WinningChange != "local" {
		t.Errorf("expected local to win, got %s", resolution.WinningChange)
	}
	
	if resolution.ResolvedValue != "local-update" {
		t.Errorf("expected resolved value to be 'local-update', got %v", resolution.ResolvedValue)
	}
}

func TestMergeStrategy(t *testing.T) {
	resolver := NewConflictResolver(MergeStrategy)
	
	conflict := &StateConflict{
		ID:        "test-conflict-3",
		Timestamp: time.Now(),
		Path:      "/data/object",
		LocalChange: &StateChange{
			Path:     "/data/object",
			OldValue: map[string]interface{}{"a": 1, "b": 2},
			NewValue: map[string]interface{}{"a": 1, "b": 3, "c": 4},
			Operation: "replace",
			Timestamp: time.Now(),
		},
		RemoteChange: &StateChange{
			Path:      "/data/object",
			OldValue:  map[string]interface{}{"a": 1, "b": 2},
			NewValue:  map[string]interface{}{"a": 2, "b": 2, "d": 5},
			Operation: "replace",
			Timestamp: time.Now().Add(time.Second),
		},
		BaseValue: map[string]interface{}{"a": 1, "b": 2},
		Severity:  SeverityLow,
	}
	
	resolution, err := resolver.Resolve(conflict)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !resolution.MergedChanges {
		t.Error("expected changes to be merged")
	}
	
	merged, ok := resolution.ResolvedValue.(map[string]interface{})
	if !ok {
		t.Fatal("expected resolved value to be a map")
	}
	
	// Check merged values
	if merged["a"] != 2 { // Remote value wins in conflict
		t.Errorf("expected a=2, got %v", merged["a"])
	}
	if merged["c"] != 4 { // Local-only value preserved
		t.Errorf("expected c=4, got %v", merged["c"])
	}
	if merged["d"] != 5 { // Remote-only value preserved
		t.Errorf("expected d=5, got %v", merged["d"])
	}
}

func TestCustomStrategy(t *testing.T) {
	resolver := NewConflictResolver(CustomStrategy)
	
	// Register custom resolver
	customCalled := false
	resolver.RegisterCustomResolver("default", func(conflict *StateConflict) (*ConflictResolution, error) {
		customCalled = true
		return &ConflictResolution{
			ID:            generateResolutionID(),
			ConflictID:    conflict.ID,
			Timestamp:     time.Now(),
			Strategy:      CustomStrategy,
			ResolvedValue: "custom-resolved",
			ResolvedPatch: JSONPatch{{
				Op:    JSONPatchOpReplace,
				Path:  conflict.Path,
				Value: "custom-resolved",
			}},
			WinningChange: "custom",
			MergedChanges: false,
			UserIntervention: false,
		}, nil
	})
	
	conflict := &StateConflict{
		ID:        "test-conflict-4",
		Timestamp: time.Now(),
		Path:      "/data/value",
		LocalChange: &StateChange{
			Path:      "/data/value",
			NewValue:  "local",
			Timestamp: time.Now(),
		},
		RemoteChange: &StateChange{
			Path:      "/data/value",
			NewValue:  "remote",
			Timestamp: time.Now(),
		},
	}
	
	resolution, err := resolver.Resolve(conflict)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !customCalled {
		t.Error("custom resolver was not called")
	}
	
	if resolution.ResolvedValue != "custom-resolved" {
		t.Errorf("expected 'custom-resolved', got %v", resolution.ResolvedValue)
	}
}

func TestConflictHistory(t *testing.T) {
	history := NewConflictHistory(10)
	
	// Record some conflicts
	for i := 0; i < 5; i++ {
		conflict := &StateConflict{
			ID:        generateConflictID(),
			Timestamp: time.Now(),
			Path:      "/test/path",
			Severity:  SeverityMedium,
		}
		history.RecordConflict(conflict)
	}
	
	// Record some resolutions
	for i := 0; i < 3; i++ {
		resolution := &ConflictResolution{
			ID:         generateResolutionID(),
			ConflictID: "test-conflict",
			Timestamp:  time.Now(),
			Strategy:   LastWriteWins,
		}
		history.RecordResolution(resolution)
	}
	
	// Check statistics
	stats := history.GetStatistics()
	if stats.TotalConflicts != 5 {
		t.Errorf("expected 5 conflicts, got %d", stats.TotalConflicts)
	}
	if stats.TotalResolutions != 3 {
		t.Errorf("expected 3 resolutions, got %d", stats.TotalResolutions)
	}
	if stats.ResolutionsByType[LastWriteWins] != 3 {
		t.Errorf("expected 3 LastWriteWins resolutions, got %d", stats.ResolutionsByType[LastWriteWins])
	}
	
	// Check recent conflicts
	recent := history.GetRecentConflicts(3)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent conflicts, got %d", len(recent))
	}
}

func TestConflictManager(t *testing.T) {
	store := NewStateStore()
	manager := NewConflictManager(store, LastWriteWins)
	
	// Set initial state
	err := store.Set("/data/value", "initial")
	if err != nil {
		t.Fatalf("failed to set initial state: %v", err)
	}
	
	// Create conflicting changes
	local := &StateChange{
		Path:      "/data/value",
		OldValue:  "initial",
		NewValue:  "local-update",
		Operation: "replace",
		Timestamp: time.Now(),
	}
	
	remote := &StateChange{
		Path:      "/data/value",
		OldValue:  "initial",
		NewValue:  "remote-update",
		Operation: "replace",
		Timestamp: time.Now().Add(time.Second),
	}
	
	// Resolve conflict
	resolution, err := manager.ResolveConflict(local, remote)
	if err != nil {
		t.Fatalf("failed to resolve conflict: %v", err)
	}
	
	if resolution == nil {
		t.Fatal("expected resolution, got nil")
	}
	
	// Apply resolution
	err = manager.ApplyResolution(resolution)
	if err != nil {
		t.Fatalf("failed to apply resolution: %v", err)
	}
	
	// Verify state was updated
	value, err := store.Get("/data/value")
	if err != nil {
		t.Fatalf("failed to get value: %v", err)
	}
	
	if value != "remote-update" {
		t.Errorf("expected 'remote-update' (last write wins), got %v", value)
	}
}

func TestConflictSeverity(t *testing.T) {
	detector := NewConflictDetector(DefaultConflictDetectorOptions())
	
	// Test remove operation - should be high severity
	local := &StateChange{
		Path:      "/critical/data",
		Operation: "remove",
		Timestamp: time.Now(),
	}
	
	remote := &StateChange{
		Path:      "/critical/data",
		Operation: "replace",
		NewValue:  "updated",
		Timestamp: time.Now(),
	}
	
	conflict, err := detector.DetectConflict(local, remote)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if conflict.Severity != SeverityHigh {
		t.Errorf("expected high severity for remove operation, got %s", conflict.Severity.String())
	}
	
	// Test type change - should be medium severity
	local2 := &StateChange{
		Path:      "/data/field",
		OldValue:  "string-value",
		NewValue:  123, // Changed to number
		Operation: "replace",
		Timestamp: time.Now(),
	}
	
	remote2 := &StateChange{
		Path:      "/data/field",
		OldValue:  "string-value",
		NewValue:  "another-string",
		Operation: "replace",
		Timestamp: time.Now(),
	}
	
	conflict2, err := detector.DetectConflict(local2, remote2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if conflict2.Severity != SeverityMedium {
		t.Errorf("expected medium severity for type change, got %s", conflict2.Severity.String())
	}
}

func TestConflictAnalyzer(t *testing.T) {
	history := NewConflictHistory(100)
	analyzer := NewConflictAnalyzer(history)
	
	// Generate conflicts on different paths
	paths := []string{"/hot/path", "/hot/path", "/hot/path", "/normal/path", "/cold/path"}
	
	for _, path := range paths {
		conflict := &StateConflict{
			ID:        generateConflictID(),
			Timestamp: time.Now(),
			Path:      path,
		}
		history.RecordConflict(conflict)
	}
	
	// Analyze patterns
	analysis := analyzer.AnalyzePatterns()
	
	hotPaths, ok := analysis["hot_paths"].([]string)
	if !ok {
		t.Fatal("expected hot_paths to be []string")
	}
	
	// Should identify /hot/path as a hot path
	found := false
	for _, path := range hotPaths {
		if path == "/hot/path" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("expected /hot/path to be identified as a hot path")
	}
}