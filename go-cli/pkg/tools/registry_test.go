package tools

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestToolRegistry(t *testing.T) {
	t.Run("Register and Get", func(t *testing.T) {
		registry := NewToolRegistry()
		
		tool := &Tool{
			Name:        "test_tool",
			Description: "Test tool",
		}
		
		// Register tool
		err := registry.Register(tool)
		if err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
		
		// Get tool
		retrieved, exists := registry.Get("test_tool")
		if !exists {
			t.Fatal("Tool not found after registration")
		}
		
		if retrieved.Name != tool.Name {
			t.Errorf("Retrieved tool name = %v, want %v", retrieved.Name, tool.Name)
		}
		
		// Verify it's a clone
		retrieved.Description = "Modified"
		original, _ := registry.Get("test_tool")
		if original.Description == "Modified" {
			t.Error("Registry returned reference instead of clone")
		}
	})
	
	t.Run("Register invalid tool", func(t *testing.T) {
		registry := NewToolRegistry()
		
		// Try to register nil
		err := registry.Register(nil)
		if err == nil {
			t.Error("Expected error registering nil tool")
		}
		
		// Try to register tool without name
		err = registry.Register(&Tool{Description: "No name"})
		if err == nil {
			t.Error("Expected error registering tool without name")
		}
	})
	
	t.Run("Has method", func(t *testing.T) {
		registry := NewToolRegistry()
		
		tool := &Tool{Name: "exists"}
		registry.Register(tool)
		
		if !registry.Has("exists") {
			t.Error("Has() returned false for existing tool")
		}
		
		if registry.Has("not_exists") {
			t.Error("Has() returned true for non-existing tool")
		}
	})
	
	t.Run("List tools", func(t *testing.T) {
		registry := NewToolRegistry()
		
		tools := []*Tool{
			{Name: "tool_c", Description: "C"},
			{Name: "tool_a", Description: "A"},
			{Name: "tool_b", Description: "B"},
		}
		
		for _, tool := range tools {
			registry.Register(tool)
		}
		
		list := registry.List()
		if len(list) != 3 {
			t.Errorf("List() returned %d tools, want 3", len(list))
		}
		
		// Check sorting
		if list[0].Name != "tool_a" || list[1].Name != "tool_b" || list[2].Name != "tool_c" {
			t.Error("List() did not return sorted tools")
		}
	})
	
	t.Run("ListNames", func(t *testing.T) {
		registry := NewToolRegistry()
		
		tools := []*Tool{
			{Name: "zebra"},
			{Name: "alpha"},
			{Name: "beta"},
		}
		
		for _, tool := range tools {
			registry.Register(tool)
		}
		
		names := registry.ListNames()
		if len(names) != 3 {
			t.Errorf("ListNames() returned %d names, want 3", len(names))
		}
		
		// Check sorting
		expected := []string{"alpha", "beta", "zebra"}
		for i, name := range names {
			if name != expected[i] {
				t.Errorf("ListNames()[%d] = %v, want %v", i, name, expected[i])
			}
		}
	})
	
	t.Run("Remove tool", func(t *testing.T) {
		registry := NewToolRegistry()
		
		tool := &Tool{Name: "removable"}
		registry.Register(tool)
		
		if !registry.Has("removable") {
			t.Fatal("Tool not registered")
		}
		
		removed := registry.Remove("removable")
		if !removed {
			t.Error("Remove() returned false for existing tool")
		}
		
		if registry.Has("removable") {
			t.Error("Tool still exists after removal")
		}
		
		// Try to remove non-existing
		removed = registry.Remove("not_exists")
		if removed {
			t.Error("Remove() returned true for non-existing tool")
		}
	})
	
	t.Run("Clear registry", func(t *testing.T) {
		registry := NewToolRegistry()
		
		// Add some tools
		registry.Register(&Tool{Name: "tool1"})
		registry.Register(&Tool{Name: "tool2"})
		
		if registry.Count() != 2 {
			t.Errorf("Count() = %d, want 2", registry.Count())
		}
		
		registry.Clear()
		
		if registry.Count() != 0 {
			t.Errorf("Count() = %d after Clear(), want 0", registry.Count())
		}
	})
	
	t.Run("ValidateArgs", func(t *testing.T) {
		registry := NewToolRegistry()
		
		tool := &Tool{
			Name: "validator_test",
			Parameters: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"required_field": {Type: "string"},
					"optional_field": {Type: "number"},
				},
				Required: []string{"required_field"},
			},
		}
		
		registry.Register(tool)
		
		// Valid arguments
		validArgs := json.RawMessage(`{"required_field": "value", "optional_field": 42}`)
		err := registry.ValidateArgs("validator_test", validArgs)
		if err != nil {
			t.Errorf("ValidateArgs() failed for valid args: %v", err)
		}
		
		// Missing required field
		invalidArgs := json.RawMessage(`{"optional_field": 42}`)
		err = registry.ValidateArgs("validator_test", invalidArgs)
		if err == nil {
			t.Error("ValidateArgs() did not fail for missing required field")
		}
		
		// Wrong type
		wrongTypeArgs := json.RawMessage(`{"required_field": 123}`)
		err = registry.ValidateArgs("validator_test", wrongTypeArgs)
		if err == nil {
			t.Error("ValidateArgs() did not fail for wrong type")
		}
		
		// Non-existing tool
		err = registry.ValidateArgs("not_exists", validArgs)
		if err == nil {
			t.Error("ValidateArgs() did not fail for non-existing tool")
		}
	})
	
	t.Run("JSON export/import", func(t *testing.T) {
		registry := NewToolRegistry()
		
		// Add tools
		tools := []*Tool{
			{
				Name:        "tool1",
				Description: "First tool",
				Parameters: &ToolSchema{
					Type: "object",
					Properties: map[string]*Property{
						"field": {Type: "string"},
					},
				},
			},
			{
				Name:        "tool2",
				Description: "Second tool",
			},
		}
		
		for _, tool := range tools {
			registry.Register(tool)
		}
		
		// Export to JSON
		data, err := registry.ToJSON()
		if err != nil {
			t.Fatalf("ToJSON() failed: %v", err)
		}
		
		// Import into new registry
		newRegistry := NewToolRegistry()
		err = newRegistry.FromJSON(data)
		if err != nil {
			t.Fatalf("FromJSON() failed: %v", err)
		}
		
		// Verify tools
		if newRegistry.Count() != 2 {
			t.Errorf("Imported registry has %d tools, want 2", newRegistry.Count())
		}
		
		tool1, exists := newRegistry.Get("tool1")
		if !exists {
			t.Error("tool1 not found in imported registry")
		} else if tool1.Description != "First tool" {
			t.Errorf("tool1 description = %v, want 'First tool'", tool1.Description)
		}
	})
	
	t.Run("GetSummary", func(t *testing.T) {
		registry := NewToolRegistry()
		
		tool := &Tool{
			Name:        "summary_test",
			Description: "Tool for summary test",
			Parameters: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"param1": {Type: "string"},
					"param2": {Type: "number"},
					"param3": {Type: "boolean"},
				},
				Required: []string{"param1", "param3"},
			},
		}
		
		registry.Register(tool)
		
		summary, err := registry.GetSummary("summary_test")
		if err != nil {
			t.Fatalf("GetSummary() failed: %v", err)
		}
		
		if summary.Name != "summary_test" {
			t.Errorf("Summary name = %v, want summary_test", summary.Name)
		}
		
		if len(summary.Parameters) != 3 {
			t.Errorf("Summary has %d parameters, want 3", len(summary.Parameters))
		}
		
		if len(summary.Required) != 2 {
			t.Errorf("Summary has %d required, want 2", len(summary.Required))
		}
		
		// Check sorting
		expectedParams := []string{"param1", "param2", "param3"}
		for i, param := range summary.Parameters {
			if param != expectedParams[i] {
				t.Errorf("Parameter[%d] = %v, want %v", i, param, expectedParams[i])
			}
		}
	})
	
	t.Run("Concurrent access", func(t *testing.T) {
		registry := NewToolRegistry()
		done := make(chan bool)
		
		// Writer goroutine
		go func() {
			for i := 0; i < 100; i++ {
				tool := &Tool{
					Name: fmt.Sprintf("tool_%d", i),
				}
				registry.Register(tool)
			}
			done <- true
		}()
		
		// Reader goroutine
		go func() {
			for i := 0; i < 100; i++ {
				registry.List()
				registry.Count()
			}
			done <- true
		}()
		
		// Wait for both
		<-done
		<-done
		
		// Verify final state
		if registry.Count() != 100 {
			t.Errorf("Final count = %d, want 100", registry.Count())
		}
	})
}

func TestRegistrySnapshot(t *testing.T) {
	registry := NewToolRegistry()
	
	// Add tools
	tools := []*Tool{
		{Name: "tool1", Description: "First"},
		{Name: "tool2", Description: "Second"},
	}
	
	for _, tool := range tools {
		registry.Register(tool)
	}
	
	// Create snapshot
	snapshot := registry.Snapshot()
	
	if snapshot.Count != 2 {
		t.Errorf("Snapshot count = %d, want 2", snapshot.Count)
	}
	
	if len(snapshot.Tools) != 2 {
		t.Errorf("Snapshot has %d tools, want 2", len(snapshot.Tools))
	}
	
	// Modify registry
	registry.Clear()
	registry.Register(&Tool{Name: "new_tool"})
	
	// Restore from snapshot
	err := registry.Restore(snapshot)
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}
	
	// Verify restoration
	if registry.Count() != 2 {
		t.Errorf("Restored registry has %d tools, want 2", registry.Count())
	}
	
	if !registry.Has("tool1") || !registry.Has("tool2") {
		t.Error("Original tools not restored")
	}
	
	if registry.Has("new_tool") {
		t.Error("New tool still exists after restore")
	}
}