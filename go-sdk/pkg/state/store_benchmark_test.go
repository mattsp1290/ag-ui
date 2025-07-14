package state

import (
	"fmt"
	"runtime"
	"testing"
)

// BenchmarkStateStore_GetState tests the performance of GetState operations
func BenchmarkStateStore_GetState(b *testing.B) {
	store := NewStateStore()

	// Populate the store with test data
	testData := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		testData[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	// Set initial state
	err := store.Set("/", testData)
	if err != nil {
		b.Fatal(err)
	}

	// Measure memory allocations before
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	allocsBefore := m.Mallocs

	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		_ = store.GetState()
	}

	b.StopTimer()

	// Measure memory allocations after
	runtime.ReadMemStats(&m)
	allocsAfter := m.Mallocs
	allocsPerOp := (allocsAfter - allocsBefore) / uint64(b.N)

	b.ReportMetric(float64(allocsPerOp), "allocs/op")
}

// BenchmarkStateStore_GetStateView tests the performance of GetStateView operations
func BenchmarkStateStore_GetStateView(b *testing.B) {
	store := NewStateStore()

	// Populate the store with test data
	testData := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		testData[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	// Set initial state
	err := store.Set("/", testData)
	if err != nil {
		b.Fatal(err)
	}

	// Measure memory allocations before
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	allocsBefore := m.Mallocs

	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		view := store.GetStateView()
		view.Cleanup()
	}

	b.StopTimer()

	// Measure memory allocations after
	runtime.ReadMemStats(&m)
	allocsAfter := m.Mallocs
	allocsPerOp := (allocsAfter - allocsBefore) / uint64(b.N)

	b.ReportMetric(float64(allocsPerOp), "allocs/op")
}

// BenchmarkStateStore_ConcurrentReads tests concurrent read performance
func BenchmarkStateStore_ConcurrentReads(b *testing.B) {
	store := NewStateStore()

	// Populate the store with test data
	testData := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		testData[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	// Set initial state
	err := store.Set("/", testData)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			view := store.GetStateView()
			_ = view.Data()
			view.Cleanup()
		}
	})
}

// BenchmarkStateStore_Updates tests update performance with COW
func BenchmarkStateStore_Updates(b *testing.B) {
	store := NewStateStore()

	// Populate the store with test data
	testData := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		testData[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	// Set initial state
	err := store.Set("/", testData)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := store.Set(fmt.Sprintf("/key%d", i%1000), fmt.Sprintf("newvalue%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}
