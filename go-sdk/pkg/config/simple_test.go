package config

import (
	"fmt"
	"reflect"
	"testing"
)

func TestSimpleOptimizedCopy(t *testing.T) {
	// Test the optimized copier independently
	copier := NewOptimizedCopier()
	
	original := map[string]interface{}{
		"string":  "test",
		"int":     42,
		"float":   3.14,
		"bool":    true,
		"nested": map[string]interface{}{
			"inner": "value",
		},
		"slice": []interface{}{"a", "b", "c"},
	}
	
	result := copier.DeepCopy(original)
	
	// Verify the copy is correct
	if !reflect.DeepEqual(original, result) {
		t.Errorf("Copy is not equal to original")
	}
	
	// Verify it's a different instance
	if &original == &result {
		t.Errorf("Expected different map instances")
	}
	
	// Verify modifying copy doesn't affect original
	result["new_key"] = "new_value"
	if _, exists := original["new_key"]; exists {
		t.Errorf("Modification to copy affected original")
	}
	
	// Test nested modification
	nestedResult := result["nested"].(map[string]interface{})
	nestedResult["new_nested"] = "test"
	
	nestedOriginal := original["nested"].(map[string]interface{})
	if _, exists := nestedOriginal["new_nested"]; exists {
		t.Errorf("Nested modification to copy affected original")
	}
	
	fmt.Printf("Test passed! Copy stats: %s\n", copier.GetStats().String())
}

func TestFastDeepCopyFunction(t *testing.T) {
	original := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"nested": map[string]interface{}{
			"inner": []string{"a", "b", "c"},
		},
	}
	
	result := FastDeepCopy(original)
	
	if !reflect.DeepEqual(original, result) {
		t.Errorf("FastDeepCopy failed")
	}
	
	// Test independence
	result["modified"] = true
	if _, exists := original["modified"]; exists {
		t.Errorf("FastDeepCopy didn't create independent copy")
	}
	
	fmt.Printf("FastDeepCopy test passed!\n")
}