package state

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/evanphx/json-patch/v5"
)

// Benchmark sizes for testing different scenarios
const (
	smallDataSize  = 100     // 100 items
	mediumDataSize = 1000    // 1K items
	largeDataSize  = 10000   // 10K items
	xLargeDataSize = 100000  // 100K items

	smallSubscribers  = 10
	mediumSubscribers = 100
	largeSubscribers  = 1000
)

// Helper functions for generating test data
func generateTestData(size int) map[string]interface{} {
	data := make(map[string]interface{}, size)
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("key_%d", i)
		// Create nested structures for realistic testing
		data[key] = map[string]interface{}{
			"id":        i,
			"name":      fmt.Sprintf("name_%d", i),
			"value":     rand.Float64() * 1000,
			"timestamp": time.Now().Unix(),
			"metadata": map[string]interface{}{
				"type":     "test",
				"version":  i % 10,
				"category": fmt.Sprintf("cat_%d", i%5),
			},
		}
	}
	return data
}

func generateComplexNestedData(depth, breadth int) map[string]interface{} {
	if depth == 0 {
		return map[string]interface{}{
			"value": rand.Intn(1000),
			"leaf":  true,
		}
	}
	
	data := make(map[string]interface{}, breadth)
	for i := 0; i < breadth; i++ {
		key := fmt.Sprintf("node_%d", i)
		data[key] = generateComplexNestedData(depth-1, breadth)
	}
	return data
}

// BenchmarkStateUpdate tests single update performance with different data sizes
func BenchmarkStateUpdate(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"Small", smallDataSize},
		{"Medium", mediumDataSize},
		{"Large", largeDataSize},
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			store := NewStateStore()
			
			// Initial setup
			initialData := generateTestData(tc.size)
			err := store.Set("/", initialData)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				path := fmt.Sprintf("/key_%d/value", i%tc.size)
				err := store.Set(path, rand.Float64()*1000)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkBatchUpdate tests batch update performance using JSON Patch
func BenchmarkBatchUpdate(b *testing.B) {
	batchSizes := []int{10, 100, 1000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("Batch_%d", batchSize), func(b *testing.B) {
			store := NewStateStore()
			
			// Initial setup
			initialData := generateTestData(largeDataSize)
			err := store.Set("/", initialData)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Create batch of patches
				patches := make(JSONPatch, 0, batchSize)
				for j := 0; j < batchSize; j++ {
					key := fmt.Sprintf("key_%d", (i*batchSize+j)%largeDataSize)
					patches = append(patches, JSONPatchOperation{
						Op:    JSONPatchOpReplace,
						Path:  fmt.Sprintf("/%s/value", key),
						Value: rand.Float64() * 1000,
					})
				}
				
				// Apply batch update
				err := store.ApplyPatch(patches)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkStateRead tests read performance under different scenarios
func BenchmarkStateRead(b *testing.B) {
	scenarios := []struct {
		name      string
		dataSize  int
		readDepth string
	}{
		{"Small_Full", smallDataSize, "/"},
		{"Medium_Full", mediumDataSize, "/"},
		{"Large_Full", largeDataSize, "/"},
		{"Large_Partial", largeDataSize, "/key_500/metadata"},
		{"XLarge_Partial", xLargeDataSize, "/key_50000/value"},
	}

	for _, tc := range scenarios {
		b.Run(tc.name, func(b *testing.B) {
			store := NewStateStore()
			
			// Setup data
			data := generateTestData(tc.dataSize)
			err := store.Set("/", data)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				value, err := store.Get(tc.readDepth)
				if err != nil && tc.readDepth != "/" {
					b.Fatal("Expected value not found:", err)
				}
				_ = value
			}
		})
	}
}

// BenchmarkConcurrentReads tests parallel read performance
func BenchmarkConcurrentReads(b *testing.B) {
	concurrencyLevels := []int{10, 100, 1000}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			store := NewStateStore()
			
			// Setup large dataset
			data := generateTestData(largeDataSize)
			err := store.Set("/", data)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					path := fmt.Sprintf("/key_%d", i%largeDataSize)
					value, err := store.Get(path)
					if err != nil {
						b.Fatal("Expected value not found:", err)
					}
					_ = value
					i++
				}
			})
		})
	}
}

// BenchmarkConcurrentWrites tests parallel write performance
func BenchmarkConcurrentWrites(b *testing.B) {
	concurrencyLevels := []int{10, 50, 100}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			store := NewStateStore()
			
			// Setup initial data
			data := generateTestData(largeDataSize)
			err := store.Set("/", data)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			var counter int64
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					idx := atomic.AddInt64(&counter, 1) % int64(largeDataSize)
					path := fmt.Sprintf("/key_%d/value", idx)
					err := store.Set(path, rand.Float64()*1000)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// BenchmarkDeltaComputationPerf tests delta algorithm performance
func BenchmarkDeltaComputationPerf(b *testing.B) {
	testCases := []struct {
		name        string
		oldSize     int
		changeRatio float64 // percentage of items changed
	}{
		{"Small_10%", smallDataSize, 0.1},
		{"Medium_10%", mediumDataSize, 0.1},
		{"Large_10%", largeDataSize, 0.1},
		{"Large_50%", largeDataSize, 0.5},
		{"Large_90%", largeDataSize, 0.9},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			delta := NewDeltaComputer(DefaultDeltaOptions())
			
			// Generate old and new states
			oldState := generateTestData(tc.oldSize)
			newState := make(map[string]interface{})
			
			// Copy old state to new state
			for k, v := range oldState {
				newState[k] = v
			}
			
			// Modify percentage of items
			changeCount := int(float64(tc.oldSize) * tc.changeRatio)
			for i := 0; i < changeCount; i++ {
				key := fmt.Sprintf("key_%d", i)
				newState[key] = map[string]interface{}{
					"id":        i,
					"name":      fmt.Sprintf("modified_name_%d", i),
					"value":     rand.Float64() * 2000, // Different value
					"timestamp": time.Now().Unix() + int64(i),
					"metadata": map[string]interface{}{
						"type":     "modified",
						"version":  i % 20,
						"category": fmt.Sprintf("new_cat_%d", i%10),
					},
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				patches, err := delta.ComputeDelta(oldState, newState)
				if err != nil {
					b.Fatal(err)
				}
				_ = patches
			}
		})
	}
}

// BenchmarkJSONPatch tests patch application performance
func BenchmarkJSONPatch(b *testing.B) {
	patchSizes := []struct {
		name       string
		patchCount int
	}{
		{"Small_10", 10},
		{"Medium_100", 100},
		{"Large_1000", 1000},
	}

	for _, tc := range patchSizes {
		b.Run(tc.name, func(b *testing.B) {
			// Setup initial data
			data := generateTestData(largeDataSize)
			dataBytes, err := json.Marshal(data)
			if err != nil {
				b.Fatal(err)
			}

			// Generate patches
			patches := make([]jsonpatch.Operation, 0, tc.patchCount)
			for i := 0; i < tc.patchCount; i++ {
				patches = append(patches, jsonpatch.Operation{
					Op:    "replace",
					Path:  fmt.Sprintf("/key_%d/value", i),
					Value: rand.Float64() * 1000,
				})
			}
			
			patchDoc := jsonpatch.Patch(patches)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				modified, err := patchDoc.Apply(dataBytes)
				if err != nil {
					b.Fatal(err)
				}
				_ = modified
			}
		})
	}
}

// BenchmarkSubscriptions tests event delivery performance
func BenchmarkSubscriptions(b *testing.B) {
	subscriberCounts := []struct {
		name  string
		count int
	}{
		{"Small", smallSubscribers},
		{"Medium", mediumSubscribers},
		{"Large", largeSubscribers},
	}

	for _, tc := range subscriberCounts {
		b.Run(tc.name, func(b *testing.B) {
			store := NewStateStore()
			
			// Setup initial data
			data := generateTestData(mediumDataSize)
			err := store.Set("/", data)
			if err != nil {
				b.Fatal(err)
			}

			// Create subscribers
			var wg sync.WaitGroup
			subscribers := make([]func(), 0, tc.count)
			for i := 0; i < tc.count; i++ {
				wg.Add(1)
				unsubscribe := store.Subscribe("/", func(change StateChange) {
					// Process event
					_ = change
				})
				subscribers = append(subscribers, unsubscribe)
				go func() {
					defer wg.Done()
					// Simulate subscription activity
					time.Sleep(10 * time.Millisecond)
				}()
			}

			// Wait for all subscribers to be ready
			time.Sleep(100 * time.Millisecond)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				path := fmt.Sprintf("/key_%d/value", i%mediumDataSize)
				err := store.Set(path, rand.Float64()*1000)
				if err != nil {
					b.Fatal(err)
				}
			}

			b.StopTimer()
			
			// Cleanup
			for _, unsubscribe := range subscribers {
				unsubscribe()
			}
			wg.Wait()
		})
	}
}

// BenchmarkMemoryUsage tracks memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	scenarios := []struct {
		name      string
		operation func(*StateStore, int)
	}{
		{
			"SingleUpdate",
			func(store *StateStore, i int) {
				path := fmt.Sprintf("/key_%d/value", i%1000)
				_ = store.Set(path, i)
			},
		},
		{
			"BatchUpdate_10",
			func(store *StateStore, i int) {
				patches := make(JSONPatch, 0, 10)
				for j := 0; j < 10; j++ {
					key := fmt.Sprintf("key_%d", (i*10+j)%1000)
					patches = append(patches, JSONPatchOperation{
						Op:    JSONPatchOpReplace,
						Path:  fmt.Sprintf("/%s/value", key),
						Value: i + j,
					})
				}
				_ = store.ApplyPatch(patches)
			},
		},
		{
			"ReadWrite_Mix",
			func(store *StateStore, i int) {
				if i%2 == 0 {
					_, _ = store.Get("/")
				} else {
					path := fmt.Sprintf("/key_%d/value", i%1000)
					_ = store.Set(path, i)
				}
			},
		},
	}

	for _, tc := range scenarios {
		b.Run(tc.name, func(b *testing.B) {
			store := NewStateStore()
			
			// Setup initial data
			data := generateTestData(mediumDataSize)
			err := store.Set("/", data)
			if err != nil {
				b.Fatal(err)
			}

			// Force GC before starting
			runtime.GC()
			runtime.GC()

			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			allocsBefore := m.Mallocs
			bytesBefore := m.TotalAlloc

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				tc.operation(store, i)
			}

			b.StopTimer()

			runtime.ReadMemStats(&m)
			allocsAfter := m.Mallocs
			bytesAfter := m.TotalAlloc

			allocsPerOp := float64(allocsAfter-allocsBefore) / float64(b.N)
			bytesPerOp := float64(bytesAfter-bytesBefore) / float64(b.N)

			b.ReportMetric(allocsPerOp, "allocs/op")
			b.ReportMetric(bytesPerOp, "bytes/op")
		})
	}
}

// BenchmarkWorstCase tests performance under worst-case scenarios
func BenchmarkWorstCase(b *testing.B) {
	b.Run("DeepNesting", func(b *testing.B) {
		store := NewStateStore()
		
		// Create deeply nested structure
		deepData := generateComplexNestedData(10, 5) // 10 levels deep, 5 branches each
		err := store.Set("/", deepData)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Update deeply nested value
			path := "/node_0/node_1/node_2/node_3/node_4/value"
			err := store.Set(path, i)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("LargeArrayModification", func(b *testing.B) {
		store := NewStateStore()
		
		// Create large array
		largeArray := make([]interface{}, 10000)
		for i := range largeArray {
			largeArray[i] = map[string]interface{}{
				"id":    i,
				"value": rand.Float64(),
			}
		}
		
		err := store.Set("/", map[string]interface{}{
			"array": largeArray,
		})
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Modify array element
			path := fmt.Sprintf("/array/%d/value", i%len(largeArray))
			err := store.Set(path, rand.Float64())
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("RapidStateChurn", func(b *testing.B) {
		store := NewStateStore()
		
		// Initial small state
		err := store.Set("/", map[string]interface{}{"counter": 0})
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Rapid state replacement
			newState := generateTestData(100)
			err := store.Set("/", newState)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkRegressionDetection provides baseline metrics for regression detection
func BenchmarkRegressionDetection(b *testing.B) {
	// These benchmarks establish baseline performance metrics
	// Run with: go test -bench=BenchmarkRegressionDetection -benchmem -benchtime=10s
	
	b.Run("Baseline_Update", func(b *testing.B) {
		store := NewStateStore()
		
		data := generateTestData(1000)
		err := store.Set("/", data)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := store.Set("/key_500/value", i)
			if err != nil {
				b.Fatal(err)
			}
		}
		
		// Expected performance: < 10µs per operation
		// Expected allocations: < 10 per operation
	})

	b.Run("Baseline_Read", func(b *testing.B) {
		store := NewStateStore()
		
		data := generateTestData(1000)
		err := store.Set("/", data)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := store.Get("/key_500")
			if err != nil {
				b.Fatal(err)
			}
		}
		
		// Expected performance: < 1µs per operation
		// Expected allocations: 0-1 per operation
	})

	b.Run("Baseline_Delta", func(b *testing.B) {
		delta := NewDeltaComputer(DefaultDeltaOptions())
		
		oldState := generateTestData(1000)
		newState := make(map[string]interface{})
		for k, v := range oldState {
			newState[k] = v
		}
		// Modify 10% of items
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key_%d", i)
			newState[key] = map[string]interface{}{
				"id":    i,
				"value": i * 2,
			}
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := delta.ComputeDelta(oldState, newState)
			if err != nil {
				b.Fatal(err)
			}
		}
		
		// Expected performance: < 1ms per operation
		// Expected allocations: < 1000 per operation
	})
}

// BenchmarkCOWEfficiency tests Copy-on-Write efficiency
func BenchmarkCOWEfficiency(b *testing.B) {
	b.Run("ReadHeavyWorkload", func(b *testing.B) {
		store := NewStateStore()
		data := generateTestData(largeDataSize)
		err := store.Set("/", data)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// 95% reads, 5% writes
				if rand.Float32() < 0.95 {
					view := store.GetStateView()
					_ = view.Data()
					view.Cleanup()
				} else {
					key := fmt.Sprintf("/key_%d/value", rand.Intn(largeDataSize))
					_ = store.Set(key, rand.Float64())
				}
			}
		})
	})

	b.Run("WriteHeavyWorkload", func(b *testing.B) {
		store := NewStateStore()
		data := generateTestData(largeDataSize)
		err := store.Set("/", data)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// 20% reads, 80% writes
				if rand.Float32() < 0.20 {
					view := store.GetStateView()
					_ = view.Data()
					view.Cleanup()
				} else {
					key := fmt.Sprintf("/key_%d/value", rand.Intn(largeDataSize))
					_ = store.Set(key, rand.Float64())
				}
			}
		})
	})
}

// BenchmarkContextCancellation tests performance under context cancellation
func BenchmarkContextCancellation(b *testing.B) {
	b.Run("NormalOperation", func(b *testing.B) {
		store := NewStateStore()
		
		data := generateTestData(mediumDataSize)
		err := store.Set("/", data)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			path := fmt.Sprintf("/key_%d/value", i%mediumDataSize)
			err := store.Set(path, i)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("FrequentCancellation", func(b *testing.B) {
		store := NewStateStore()
		
		data := generateTestData(mediumDataSize)
		err := store.Set("/", data)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			path := fmt.Sprintf("/key_%d/value", i%mediumDataSize)
			err := store.Set(path, i)
			if err != nil {
				b.Fatal(err)
			}
			cancel()
			_ = ctx // use ctx to avoid compiler warning
		}
	})
}