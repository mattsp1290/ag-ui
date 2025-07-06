package state

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// TestOptimizedDeltaCorrectness verifies that the optimized algorithm produces the same results
func TestOptimizedDeltaCorrectness(t *testing.T) {
	tests := []struct {
		name string
		old  map[string]interface{}
		new  map[string]interface{}
	}{
		{
			name: "simple addition",
			old:  map[string]interface{}{"a": 1, "b": 2},
			new:  map[string]interface{}{"a": 1, "b": 2, "c": 3},
		},
		{
			name: "simple deletion",
			old:  map[string]interface{}{"a": 1, "b": 2, "c": 3},
			new:  map[string]interface{}{"a": 1, "b": 2},
		},
		{
			name: "simple modification",
			old:  map[string]interface{}{"a": 1, "b": 2, "c": 3},
			new:  map[string]interface{}{"a": 1, "b": 20, "c": 3},
		},
		{
			name: "mixed operations",
			old:  map[string]interface{}{"a": 1, "b": 2, "c": 3, "d": 4},
			new:  map[string]interface{}{"a": 10, "b": 2, "e": 5, "f": 6},
		},
		{
			name: "nested objects",
			old: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "John",
					"age":  30,
					"address": map[string]interface{}{
						"city": "NYC",
						"zip":  "10001",
					},
				},
			},
			new: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "Jane",
					"age":  31,
					"address": map[string]interface{}{
						"city":    "NYC",
						"zip":     "10002",
						"country": "USA",
					},
				},
			},
		},
		{
			name: "large object",
			old:  generateLargeObject(100),
			new:  modifyLargeObject(generateLargeObject(100), 30),
		},
	}

	dc := NewDeltaComputer(DefaultDeltaOptions())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compute delta using the optimized algorithm
			patch, err := dc.ComputeDelta(tt.old, tt.new)
			if err != nil {
				t.Fatalf("Failed to compute delta: %v", err)
			}

			// Apply the patch to verify it transforms old to new
			result, err := patch.Apply(tt.old)
			if err != nil {
				t.Fatalf("Failed to apply patch: %v", err)
			}

			// Verify the result matches the expected new state
			if !jsonEqualOpt(result, tt.new) {
				t.Errorf("Patch did not produce expected result")
				t.Errorf("Expected: %v", tt.new)
				t.Errorf("Got: %v", result)
			}
		})
	}
}

// BenchmarkDeltaComputation compares performance of different sizes
func BenchmarkDeltaComputation(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			old := generateLargeObject(size)
			new := modifyLargeObject(generateLargeObject(size), size/3)
			dc := NewDeltaComputer(DefaultDeltaOptions())

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := dc.ComputeDelta(old, new)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkOptimizationPasses benchmarks the optimization passes
func BenchmarkOptimizationPasses(b *testing.B) {
	// Generate a patch with many operations
	patch := generateComplexPatch(1000)
	dc := NewDeltaComputer(DefaultDeltaOptions())

	b.Run("full_optimization", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = dc.OptimizePatch(patch)
		}
	})

	b.Run("batch_related_ops", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = dc.batchRelatedOps(patch)
		}
	})

	b.Run("optimize_path_ops", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = dc.optimizePathOperations(patch)
		}
	})
}

// TestHashCollisions verifies that our hash function doesn't produce false positives
func TestHashCollisions(t *testing.T) {
	dc := NewDeltaComputer(DefaultDeltaOptions())
	
	// Test different values that should have different hashes
	values := []interface{}{
		"hello",
		"world",
		123,
		123.0,
		[]interface{}{1, 2, 3},
		[]interface{}{1, 2, 3, 4},
		map[string]interface{}{"a": 1},
		map[string]interface{}{"b": 1},
		map[string]interface{}{"a": 2},
		nil,
		true,
		false,
	}

	hashes := make(map[uint64]interface{})
	for _, val := range values {
		hash := dc.hashValue(val)
		if existing, exists := hashes[hash]; exists {
			if jsonEqualOpt(existing, val) {
				// Same value, same hash is OK
				continue
			}
			t.Errorf("Hash collision detected: %v and %v have same hash %d", existing, val, hash)
		}
		hashes[hash] = val
	}
}

// Helper functions

func generateLargeObject(size int) map[string]interface{} {
	obj := make(map[string]interface{}, size)
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("key_%d", i)
		switch i % 4 {
		case 0:
			obj[key] = i
		case 1:
			obj[key] = fmt.Sprintf("value_%d", i)
		case 2:
			obj[key] = float64(i) * 1.5
		case 3:
			obj[key] = map[string]interface{}{
				"nested": i,
				"data":   fmt.Sprintf("nested_%d", i),
			}
		}
	}
	return obj
}

func modifyLargeObject(obj map[string]interface{}, modifications int) map[string]interface{} {
	result := make(map[string]interface{})
	// Copy most of the original
	for k, v := range obj {
		result[k] = v
	}

	rand.Seed(time.Now().UnixNano())
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}

	// Perform modifications
	for i := 0; i < modifications && i < len(keys); i++ {
		op := rand.Intn(3)
		switch op {
		case 0: // Delete
			delete(result, keys[i])
		case 1: // Modify
			result[keys[i]] = fmt.Sprintf("modified_%d", i)
		case 2: // Add new
			result[fmt.Sprintf("new_key_%d", i)] = fmt.Sprintf("new_value_%d", i)
		}
	}

	return result
}

func generateComplexPatch(size int) JSONPatch {
	patch := make(JSONPatch, 0, size)
	
	for i := 0; i < size; i++ {
		op := JSONPatchOperation{
			Path: fmt.Sprintf("/path/%d/item", i%10),
		}
		
		switch i % 4 {
		case 0:
			op.Op = JSONPatchOpAdd
			op.Value = fmt.Sprintf("value_%d", i)
		case 1:
			op.Op = JSONPatchOpRemove
		case 2:
			op.Op = JSONPatchOpReplace
			op.Value = i
		case 3:
			op.Op = JSONPatchOpMove
			op.From = fmt.Sprintf("/old/path/%d", i)
		}
		
		patch = append(patch, op)
	}
	
	return patch
}

func jsonEqualOpt(a, b interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	
	var aNorm, bNorm interface{}
	json.Unmarshal(aJSON, &aNorm)
	json.Unmarshal(bJSON, &bNorm)
	
	return fmt.Sprintf("%v", aNorm) == fmt.Sprintf("%v", bNorm)
}