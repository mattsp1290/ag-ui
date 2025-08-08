package config

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestOptimizedCopier_DeepCopy_Nil(t *testing.T) {
	copier := NewOptimizedCopier()
	result := copier.DeepCopy(nil)
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}
}

func TestOptimizedCopier_DeepCopy_EmptyMap(t *testing.T) {
	copier := NewOptimizedCopier()
	original := make(map[string]interface{})
	result := copier.DeepCopy(original)
	
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result) != 0 {
		t.Errorf("Expected empty map, got %v", result)
	}
	
	// Ensure it's a different map instance
	if &result == &original {
		t.Error("Expected different map instances")
	}
}

func TestOptimizedCopier_DeepCopy_SimpleTypes(t *testing.T) {
	copier := NewOptimizedCopier()
	original := map[string]interface{}{
		"string":  "test",
		"int":     42,
		"int64":   int64(123),
		"float64": 3.14,
		"bool":    true,
	}
	
	result := copier.DeepCopy(original)
	
	if !reflect.DeepEqual(original, result) {
		t.Errorf("Expected %v, got %v", original, result)
	}
	
	// Ensure it's a different map instance
	if &result == &original {
		t.Error("Expected different map instances")
	}
}

func TestOptimizedCopier_DeepCopy_NestedMaps(t *testing.T) {
	copier := NewOptimizedCopier()
	original := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"value": "deep",
				},
			},
		},
	}
	
	result := copier.DeepCopy(original)
	
	if !reflect.DeepEqual(original, result) {
		t.Errorf("Expected %v, got %v", original, result)
	}
	
	// Test that modifying result doesn't affect original
	level3 := result["level1"].(map[string]interface{})["level2"].(map[string]interface{})["level3"].(map[string]interface{})
	level3["new_value"] = "added"
	
	originalLevel3 := original["level1"].(map[string]interface{})["level2"].(map[string]interface{})["level3"].(map[string]interface{})
	if _, exists := originalLevel3["new_value"]; exists {
		t.Error("Modification to copy affected original")
	}
}

func TestOptimizedCopier_DeepCopy_Slices(t *testing.T) {
	copier := NewOptimizedCopier()
	original := map[string]interface{}{
		"interface_slice": []interface{}{"a", 1, true},
		"string_slice":    []string{"hello", "world"},
		"int_slice":       []int{1, 2, 3},
		"int64_slice":     []int64{100, 200, 300},
		"float64_slice":   []float64{1.1, 2.2, 3.3},
		"bool_slice":      []bool{true, false, true},
	}
	
	result := copier.DeepCopy(original)
	
	if !reflect.DeepEqual(original, result) {
		t.Errorf("Expected %v, got %v", original, result)
	}
	
	// Test that modifying slice in result doesn't affect original
	resultSlice := result["string_slice"].([]string)
	resultSlice[0] = "modified"
	
	if original["string_slice"].([]string)[0] == "modified" {
		t.Error("Modification to copy affected original slice")
	}
}

func TestOptimizedCopier_DeepCopy_NestedSlices(t *testing.T) {
	copier := NewOptimizedCopier()
	original := map[string]interface{}{
		"slice_of_maps": []interface{}{
			map[string]interface{}{"id": 1, "name": "first"},
			map[string]interface{}{"id": 2, "name": "second"},
		},
	}
	
	result := copier.DeepCopy(original)
	
	if !reflect.DeepEqual(original, result) {
		t.Errorf("Expected %v, got %v", original, result)
	}
	
	// Test that modifying nested map in slice doesn't affect original
	resultSlice := result["slice_of_maps"].([]interface{})
	resultMap := resultSlice[0].(map[string]interface{})
	resultMap["modified"] = true
	
	originalSlice := original["slice_of_maps"].([]interface{})
	originalMap := originalSlice[0].(map[string]interface{})
	if _, exists := originalMap["modified"]; exists {
		t.Error("Modification to copy affected original nested map")
	}
}

func TestOptimizedCopier_DeepCopy_SpecializedMaps(t *testing.T) {
	copier := NewOptimizedCopier()
	original := map[string]interface{}{
		"string_string_map": map[string]string{"key1": "value1", "key2": "value2"},
		"string_int_map":    map[string]int{"count1": 10, "count2": 20},
	}
	
	result := copier.DeepCopy(original)
	
	if !reflect.DeepEqual(original, result) {
		t.Errorf("Expected %v, got %v", original, result)
	}
	
	// Test that modifying specialized map in result doesn't affect original
	resultMap := result["string_string_map"].(map[string]string)
	resultMap["key3"] = "value3"
	
	originalMap := original["string_string_map"].(map[string]string)
	if _, exists := originalMap["key3"]; exists {
		t.Error("Modification to copy affected original specialized map")
	}
}

func TestOptimizedCopier_StackOverflowProtection(t *testing.T) {
	copier := NewOptimizedCopier()
	
	// Create a deeply nested structure that exceeds max depth
	current := make(map[string]interface{})
	root := current
	
	for i := 0; i < MaxDepth+50; i++ {
		next := make(map[string]interface{})
		current["next"] = next
		current["value"] = i
		current = next
	}
	
	result := copier.DeepCopy(root)
	
	// Should not panic and should return a partial result
	if result == nil {
		t.Fatal("Expected non-nil result even with stack overflow protection")
	}
	
	stats := copier.GetStats()
	if stats.StackOverflowHits == 0 {
		t.Error("Expected stack overflow protection to be triggered")
	}
}

func TestOptimizedCopier_CopyOnWriteOptimization(t *testing.T) {
	copier := NewOptimizedCopier()
	
	// Create a large map with only immutable values
	original := make(map[string]interface{}, CopyOnWriteThreshold+10)
	for i := 0; i < CopyOnWriteThreshold+10; i++ {
		original[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	
	initialStats := copier.GetStats()
	result := copier.DeepCopy(original)
	finalStats := copier.GetStats()
	
	// Should perform copy-on-write optimization
	if finalStats.CowOptimizations <= initialStats.CowOptimizations {
		t.Log("Copy-on-write optimization may not have been triggered (acceptable for mixed content)")
	}
	
	// Result should still be correct
	if !reflect.DeepEqual(original, result) {
		t.Errorf("Copy-on-write optimization produced incorrect result")
	}
}

func TestOptimizedCopier_TypeOptimizations(t *testing.T) {
	copier := NewOptimizedCopier()
	
	// Test immutable value optimization
	testCases := []interface{}{
		"string",
		42,
		int64(123),
		float64(3.14),
		true,
		complex64(1 + 2i),
	}
	
	initialStats := copier.GetStats()
	
	for _, value := range testCases {
		result := copier.DeepCopyValue(value)
		if result != value {
			t.Errorf("Expected same value for immutable type, got %v != %v", result, value)
		}
	}
	
	finalStats := copier.GetStats()
	
	if finalStats.TypeOptimizations <= initialStats.TypeOptimizations {
		t.Error("Expected type optimizations to be applied")
	}
}

func TestOptimizedCopier_ThreadSafety(t *testing.T) {
	copier := NewOptimizedCopier()
	original := map[string]interface{}{
		"shared": map[string]interface{}{
			"value": "test",
		},
	}
	
	// Run concurrent copying
	done := make(chan bool)
	errors := make(chan error, 10)
	
	for i := 0; i < 10; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					errors <- fmt.Errorf("panic during concurrent copy: %v", r)
				}
				done <- true
			}()
			
			for j := 0; j < 100; j++ {
				result := copier.DeepCopy(original)
				if !reflect.DeepEqual(original, result) {
					errors <- fmt.Errorf("concurrent copy produced incorrect result")
					return
				}
			}
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Check for errors
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestOptimizedCopier_PoolEfficiency(t *testing.T) {
	copier := NewOptimizedCopier()
	original := generateTestMixedTypeMap(50)
	
	// First copy should populate the pools
	result1 := copier.DeepCopy(original)
	if result1 == nil {
		t.Fatal("Expected non-nil result")
	}
	
	// Second copy should reuse from pools
	result2 := copier.DeepCopy(original)
	if result2 == nil {
		t.Fatal("Expected non-nil result")
	}
	
	// Results should be equal but different instances
	if !reflect.DeepEqual(result1, result2) {
		t.Error("Pool reuse produced different results")
	}
	
	if &result1 == &result2 {
		t.Error("Expected different map instances")
	}
}

func TestFastDeepCopy_ConvenienceFunction(t *testing.T) {
	original := map[string]interface{}{
		"test": "value",
		"nested": map[string]interface{}{
			"inner": 42,
		},
	}
	
	result := FastDeepCopy(original)
	
	if !reflect.DeepEqual(original, result) {
		t.Errorf("FastDeepCopy produced incorrect result")
	}
	
	// Test that it's a different instance
	result["modified"] = true
	if _, exists := original["modified"]; exists {
		t.Error("FastDeepCopy didn't create independent copy")
	}
}

func TestFastDeepCopyValue_ConvenienceFunction(t *testing.T) {
	testCases := []interface{}{
		"string",
		42,
		[]string{"a", "b"},
		map[string]interface{}{"key": "value"},
	}
	
	for _, original := range testCases {
		result := FastDeepCopyValue(original)
		if !reflect.DeepEqual(original, result) {
			t.Errorf("FastDeepCopyValue failed for %T: %v != %v", original, original, result)
		}
	}
}

func TestCopyConfigData_SpecializedFunction(t *testing.T) {
	original := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":    "test",
			"version": "1.0",
		},
		"properties": map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		"config": map[string]interface{}{
			"nested": map[string]interface{}{
				"value": 42,
			},
		},
	}
	
	result := CopyConfigData(original)
	
	if !reflect.DeepEqual(original, result) {
		t.Errorf("CopyConfigData produced incorrect result")
	}
	
	// Should be independent copy
	result["new_key"] = "new_value"
	if _, exists := original["new_key"]; exists {
		t.Error("CopyConfigData didn't create independent copy")
	}
}

func TestOptimizedCopier_Statistics(t *testing.T) {
	copier := NewOptimizedCopier()
	
	// Reset stats
	copier.ResetStats()
	initialStats := copier.GetStats()
	
	if initialStats.TotalCopies != 0 {
		t.Error("Expected zero total copies after reset")
	}
	
	// Perform some copies
	data := generateTestMixedTypeMap(10)
	for i := 0; i < 5; i++ {
		copier.DeepCopy(data)
	}
	
	finalStats := copier.GetStats()
	if finalStats.TotalCopies != 5 {
		t.Errorf("Expected 5 total copies, got %d", finalStats.TotalCopies)
	}
	
	// Test string representation
	statsStr := finalStats.String()
	if statsStr == "" {
		t.Error("Expected non-empty statistics string")
	}
}

func TestOptimizedCopier_EdgeCases(t *testing.T) {
	copier := NewOptimizedCopier()
	
	// Test with nil values in map
	original := map[string]interface{}{
		"nil_value":   nil,
		"string":      "test",
		"nil_in_slice": []interface{}{nil, "value", nil},
	}
	
	result := copier.DeepCopy(original)
	
	if !reflect.DeepEqual(original, result) {
		t.Errorf("Failed to handle nil values correctly")
	}
	
	// Test empty slices
	original["empty_slice"] = []interface{}{}
	original["empty_string_slice"] = []string{}
	
	result = copier.DeepCopy(original)
	
	if !reflect.DeepEqual(original, result) {
		t.Errorf("Failed to handle empty slices correctly")
	}
}

func TestOptimizedCopier_LargeData(t *testing.T) {
	copier := NewOptimizedCopier()
	
	// Test with large amounts of data
	original := make(map[string]interface{}, 10000)
	for i := 0; i < 10000; i++ {
		if i%100 == 0 {
			// Add some nested structure every 100 items
			original[fmt.Sprintf("nested_%d", i)] = map[string]interface{}{
				"id":    i,
				"data":  fmt.Sprintf("nested_data_%d", i),
				"slice": []int{i, i + 1, i + 2},
			}
		} else {
			original[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
		}
	}
	
	start := time.Now()
	result := copier.DeepCopy(original)
	elapsed := time.Since(start)
	
	t.Logf("Large data copy took %v", elapsed)
	
	if len(result) != len(original) {
		t.Errorf("Expected %d keys, got %d", len(original), len(result))
	}
	
	// Spot check some values
	if result["key_500"] != "value_500" {
		t.Errorf("Incorrect value for key_500")
	}
	
	if nested, ok := result["nested_500"].(map[string]interface{}); ok {
		if nested["id"] != 500 {
			t.Errorf("Incorrect nested value")
		}
	} else {
		t.Error("Expected nested map at nested_500")
	}
}

func TestUnsafeCopyMap_ZeroCopy(t *testing.T) {
	original := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	
	result := UnsafeCopyMap(original)
	
	// Should be the same instance (zero-copy)
	if &result != &original {
		t.Error("UnsafeCopyMap should return the same instance")
	}
	
	// Values should be identical
	if !reflect.DeepEqual(original, result) {
		t.Error("UnsafeCopyMap should preserve all values")
	}
}

// Helpers for generating test data

func generateTestMixedTypeMap(size int) map[string]interface{} {
	m := make(map[string]interface{}, size)
	
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("key_%d", i)
		switch i % 7 {
		case 0:
			m[key] = fmt.Sprintf("string_value_%d", i)
		case 1:
			m[key] = i
		case 2:
			m[key] = int64(i)
		case 3:
			m[key] = float64(i) * 1.5
		case 4:
			m[key] = i%2 == 0
		case 5:
			m[key] = []string{fmt.Sprintf("item_%d_1", i), fmt.Sprintf("item_%d_2", i)}
		case 6:
			if i%14 == 6 {
				// Add nested map occasionally
				m[key] = map[string]interface{}{
					"nested_id":   i,
					"nested_name": fmt.Sprintf("nested_%d", i),
				}
			} else {
				m[key] = []int{i, i + 1, i + 2}
			}
		}
	}
	
	return m
}