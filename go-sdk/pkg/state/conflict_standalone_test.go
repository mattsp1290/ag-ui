package state

import (
	"testing"
	"time"
)

// Standalone tests for conflict resolution without event handler dependencies

func TestStandaloneConflictDetection(t *testing.T) {
	detector := NewConflictDetector(DefaultConflictDetectorOptions())
	
	// Test basic conflict detection
	local := &StateChange{
		Path:      "/test/data",
		OldValue:  "original",
		NewValue:  "local-change",
		Operation: "replace",
		Timestamp: time.Now(),
	}
	
	remote := &StateChange{
		Path:      "/test/data",
		OldValue:  "original",
		NewValue:  "remote-change",
		Operation: "replace",
		Timestamp: time.Now().Add(time.Second),
	}
	
	conflict, err := detector.DetectConflict(local, remote)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if conflict == nil {
		t.Fatal("expected conflict to be detected")
	}
	
	if conflict.Path != "/test/data" {
		t.Errorf("expected path to be /test/data, got %s", conflict.Path)
	}
}

func TestStandaloneLastWriteWins(t *testing.T) {
	resolver := NewConflictResolver(LastWriteWins)
	
	// Create a conflict with clear timestamps
	earlierTime := time.Now()
	laterTime := earlierTime.Add(time.Hour)
	
	conflict := &StateConflict{
		ID:        "test-conflict",
		Timestamp: time.Now(),
		Path:      "/data",
		LocalChange: &StateChange{
			Path:      "/data",
			NewValue:  "earlier-value",
			Timestamp: earlierTime,
		},
		RemoteChange: &StateChange{
			Path:      "/data",
			NewValue:  "later-value",
			Timestamp: laterTime,
		},
	}
	
	resolution, err := resolver.Resolve(conflict)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if resolution.ResolvedValue != "later-value" {
		t.Errorf("expected later-value to win, got %v", resolution.ResolvedValue)
	}
	
	if resolution.WinningChange != "remote" {
		t.Errorf("expected remote to win, got %s", resolution.WinningChange)
	}
}

func TestStandaloneMergeStrategy(t *testing.T) {
	resolver := NewConflictResolver(MergeStrategy)
	
	// Test merging maps
	baseMap := map[string]interface{}{
		"name": "original",
		"age":  25,
	}
	
	localMap := map[string]interface{}{
		"name":  "updated-local",
		"age":   25,
		"email": "local@example.com",
	}
	
	remoteMap := map[string]interface{}{
		"name":  "updated-remote",
		"age":   26,
		"phone": "123-456-7890",
	}
	
	conflict := &StateConflict{
		ID:        "merge-test",
		Timestamp: time.Now(),
		Path:      "/user",
		LocalChange: &StateChange{
			Path:      "/user",
			OldValue:  baseMap,
			NewValue:  localMap,
			Operation: "replace",
			Timestamp: time.Now(),
		},
		RemoteChange: &StateChange{
			Path:      "/user",
			OldValue:  baseMap,
			NewValue:  remoteMap,
			Operation: "replace",
			Timestamp: time.Now(),
		},
		BaseValue: baseMap,
	}
	
	resolution, err := resolver.Resolve(conflict)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !resolution.MergedChanges {
		t.Fatal("expected changes to be merged")
	}
	
	merged, ok := resolution.ResolvedValue.(map[string]interface{})
	if !ok {
		t.Fatal("expected resolved value to be a map")
	}
	
	// Check that non-conflicting fields were preserved
	if merged["email"] != "local@example.com" {
		t.Errorf("expected email from local to be preserved")
	}
	
	if merged["phone"] != "123-456-7890" {
		t.Errorf("expected phone from remote to be preserved")
	}
	
	// For conflicting fields, remote wins in our implementation
	if merged["name"] != "updated-remote" {
		t.Errorf("expected remote name to win in conflict")
	}
}