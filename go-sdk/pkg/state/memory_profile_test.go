package state

import (
	"fmt"
	"runtime"
	"testing"
)

// TestMemoryProfile compares memory usage between GetState and GetStateView
func TestMemoryProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory profile test in short mode")
	}
	
	store := NewStateStore()

	// Create a large state
	largeData := make(map[string]interface{})
	for i := 0; i < 10000; i++ {
		largeData[fmt.Sprintf("key%d", i)] = map[string]interface{}{
			"value":       fmt.Sprintf("value%d", i),
			"description": fmt.Sprintf("This is a longer description for key %d", i),
			"metadata": map[string]interface{}{
				"created": "2024-01-01",
				"updated": "2024-01-02",
				"tags":    []string{"tag1", "tag2", "tag3"},
			},
		}
	}

	err := store.Set("/", largeData)
	if err != nil {
		t.Fatal(err)
	}

	// Force GC to get clean baseline
	runtime.GC()
	runtime.GC()

	// Measure GetState (deep copy)
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Perform 100 GetState operations
	for i := 0; i < 100; i++ {
		_ = store.GetState()
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	getStateAllocBytes := m2.TotalAlloc - m1.TotalAlloc
	getStateAllocs := m2.Mallocs - m1.Mallocs

	// Measure GetStateView (COW)
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Perform 100 GetStateView operations
	views := make([]*StateView, 100)
	for i := 0; i < 100; i++ {
		views[i] = store.GetStateView()
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	getStateViewAllocBytes := m2.TotalAlloc - m1.TotalAlloc
	getStateViewAllocs := m2.Mallocs - m1.Mallocs

	// Cleanup views
	for _, view := range views {
		view.Cleanup()
	}

	// Report results
	t.Logf("Memory Usage Comparison (100 operations on 10k keys):")
	t.Logf("GetState:")
	t.Logf("  Total Bytes Allocated: %d", getStateAllocBytes)
	t.Logf("  Number of Allocations: %d", getStateAllocs)
	t.Logf("  Bytes per Operation: %d", getStateAllocBytes/100)
	t.Logf("  Allocs per Operation: %d", getStateAllocs/100)

	t.Logf("\nGetStateView:")
	t.Logf("  Total Bytes Allocated: %d", getStateViewAllocBytes)
	t.Logf("  Number of Allocations: %d", getStateViewAllocs)
	t.Logf("  Bytes per Operation: %d", getStateViewAllocBytes/100)
	t.Logf("  Allocs per Operation: %d", getStateViewAllocs/100)

	t.Logf("\nImprovement:")
	t.Logf("  Memory Reduction: %.2fx", float64(getStateAllocBytes)/float64(getStateViewAllocBytes))
	t.Logf("  Allocation Reduction: %.2fx", float64(getStateAllocs)/float64(getStateViewAllocs))

	// Verify significant improvement
	if getStateViewAllocBytes >= getStateAllocBytes {
		t.Error("GetStateView should allocate less memory than GetState")
	}
}
