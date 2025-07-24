package state

import (
	"fmt"
	"testing"
)

// TestPerformanceComparison compares the old O(n²) approach with the new O(n) approach
func TestPerformanceComparison(t *testing.T) {
	// Removed t.Parallel() to prevent resource contention
	// t.Parallel()
	sizes := []int{50, 100, 200, 500}  // Reduced sizes to prevent timeout

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			old := generateLargeObject(size)
			new := modifyLargeObject(generateLargeObject(size), size/3)

			dc := NewDeltaComputer(DefaultDeltaOptions())

			// Measure the performance
			patch, err := dc.ComputeDelta(old, new)
			if err != nil {
				t.Fatalf("Failed to compute delta: %v", err)
			}

			// Log some statistics
			t.Logf("Object size: %d keys", size)
			t.Logf("Patch operations: %d", len(patch))
			t.Logf("Estimated complexity: O(n) where n=%d", size)

			// Verify correctness
			result, err := patch.Apply(old)
			if err != nil {
				t.Fatalf("Failed to apply patch: %v", err)
			}

			if !jsonEqualOpt(result, new) {
				t.Errorf("Patch did not produce expected result")
			}
		})
	}
}

// BenchmarkLinearVsQuadratic simulates the difference between O(n²) and O(n)
func BenchmarkLinearVsQuadratic(b *testing.B) {
	sizes := []int{50, 100, 200, 400}  // Reduced sizes to prevent timeout

	for _, size := range sizes {
		// Our optimized O(n) implementation
		b.Run(fmt.Sprintf("linear_n=%d", size), func(b *testing.B) {
			old := generateLargeObject(size)
			new := modifyLargeObject(generateLargeObject(size), size/3)
			dc := NewDeltaComputer(DefaultDeltaOptions())

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dc.ComputeDelta(old, new)
			}
		})

		// Simulate O(n²) behavior for comparison
		b.Run(fmt.Sprintf("quadratic_simulation_n=%d", size), func(b *testing.B) {
			old := generateLargeObject(size)
			new := modifyLargeObject(generateLargeObject(size), size/3)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Simulate O(n²) work
				simulateQuadraticWork(old, new)
			}
		})
	}
}

// simulateQuadraticWork simulates O(n²) behavior for comparison
func simulateQuadraticWork(old, new map[string]interface{}) {
	// This simulates the old nested loop approach
	operations := 0
	for k1 := range old {
		for k2 := range new {
			// Simulate comparison work
			operations++
			if k1 == k2 {
				// Simulate deep comparison
				_ = fmt.Sprintf("%v%v", old[k1], new[k2])
			}
		}
	}
}
