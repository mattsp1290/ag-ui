// Package main demonstrates performance optimization features for high-scale
// state management including pooling, sharding, batching, and caching.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ag-ui/go-sdk/pkg/state"
)

// BenchmarkScenario represents a performance test scenario
type BenchmarkScenario struct {
	Name        string
	Description string
	WorkerCount int
	Operations  int
	DataSize    int
	ReadRatio   float64 // Percentage of reads vs writes
}

// BenchmarkResult stores performance metrics
type BenchmarkResult struct {
	Scenario      string
	Duration      time.Duration
	TotalOps      int64
	OpsPerSecond  float64
	AvgLatency    time.Duration
	P50Latency    time.Duration
	P95Latency    time.Duration
	P99Latency    time.Duration
	MemoryUsed    uint64
	Allocations   uint64
	GCPauses      uint32
	CPUUsage      float64
}

func main() {
	fmt.Println("=== Performance Optimization Demo ===\n")
	
	// Run demonstrations
	demonstrateObjectPooling()
	demonstrateStateSharding()
	demonstrateBatchProcessing()
	demonstrateLazyLoading()
	demonstrateMemoryOptimization()
	runComprehensiveBenchmark()
}

func demonstrateObjectPooling() {
	fmt.Println("1. Object Pooling Demo")
	fmt.Println("----------------------")
	
	// Create performance optimizer with pooling
	perfOptions := state.PerformanceOptions{
		EnablePooling:    true,
		MaxConcurrency:   runtime.NumCPU() * 2,
		EnableBatching:   true,
		BatchSize:        100,
		BatchTimeout:     10 * time.Millisecond,
	}
	
	optimizer := state.NewPerformanceOptimizer(perfOptions)
	
	// Compare with and without pooling
	fmt.Println("  Running allocation benchmark...")
	
	// Without pooling
	noPoolStore := state.NewStateStore()
	withoutPooling := benchmarkAllocations(noPoolStore, optimizer, false)
	
	// With pooling
	poolStore := state.NewStateStore(
		state.WithPerformanceOptimizer(optimizer),
	)
	withPooling := benchmarkAllocations(poolStore, optimizer, true)
	
	// Show results
	fmt.Println("\n  Object Pooling Results:")
	fmt.Printf("    Without pooling:\n")
	fmt.Printf("      Allocations: %d\n", withoutPooling.allocations)
	fmt.Printf("      Memory used: %.2f MB\n", float64(withoutPooling.memoryUsed)/1024/1024)
	fmt.Printf("      GC runs: %d\n", withoutPooling.gcRuns)
	fmt.Printf("      Time: %v\n", withoutPooling.duration)
	
	fmt.Printf("\n    With pooling:\n")
	fmt.Printf("      Allocations: %d (%.1f%% reduction)\n", 
		withPooling.allocations,
		(1-float64(withPooling.allocations)/float64(withoutPooling.allocations))*100)
	fmt.Printf("      Memory used: %.2f MB (%.1f%% reduction)\n", 
		float64(withPooling.memoryUsed)/1024/1024,
		(1-float64(withPooling.memoryUsed)/float64(withoutPooling.memoryUsed))*100)
	fmt.Printf("      GC runs: %d (%.1f%% reduction)\n", 
		withPooling.gcRuns,
		(1-float64(withPooling.gcRuns)/float64(withoutPooling.gcRuns))*100)
	fmt.Printf("      Time: %v (%.1fx faster)\n", 
		withPooling.duration,
		float64(withoutPooling.duration)/float64(withPooling.duration))
	
	// Show pool statistics
	stats := optimizer.GetStats()
	fmt.Printf("\n    Pool efficiency: %.1f%% (hits: %d, misses: %d)\n",
		stats.PoolEfficiency*100, stats.PoolHits, stats.PoolMisses)
	
	fmt.Println()
}

func demonstrateStateSharding() {
	fmt.Println("2. State Sharding Demo")
	fmt.Println("----------------------")
	
	// Create sharded state store
	shardCount := 16
	perfOptions := state.PerformanceOptions{
		EnableSharding: true,
		ShardCount:     shardCount,
		EnablePooling:  true,
	}
	
	optimizer := state.NewPerformanceOptimizer(perfOptions)
	shardedStore := state.NewShardedStateStore(shardCount,
		state.WithPerformanceOptimizer(optimizer),
	)
	
	// Create regular store for comparison
	regularStore := state.NewStateStore()
	
	fmt.Printf("  Testing concurrent access with %d shards...\n", shardCount)
	
	// Benchmark concurrent operations
	workerCount := 100
	opsPerWorker := 1000
	
	// Regular store benchmark
	fmt.Println("\n  Regular store (single lock):")
	regularResult := benchmarkConcurrentAccess(regularStore, workerCount, opsPerWorker)
	
	// Sharded store benchmark
	fmt.Println("\n  Sharded store (distributed locks):")
	shardedResult := benchmarkConcurrentAccess(shardedStore, workerCount, opsPerWorker)
	
	// Compare results
	fmt.Println("\n  Sharding Performance Comparison:")
	fmt.Printf("    Regular store:\n")
	fmt.Printf("      Duration: %v\n", regularResult.duration)
	fmt.Printf("      Throughput: %.0f ops/sec\n", regularResult.opsPerSecond)
	fmt.Printf("      Lock contention: %.1f%%\n", regularResult.contentionRate*100)
	
	fmt.Printf("\n    Sharded store:\n")
	fmt.Printf("      Duration: %v (%.1fx faster)\n", 
		shardedResult.duration,
		float64(regularResult.duration)/float64(shardedResult.duration))
	fmt.Printf("      Throughput: %.0f ops/sec (%.1fx improvement)\n", 
		shardedResult.opsPerSecond,
		shardedResult.opsPerSecond/regularResult.opsPerSecond)
	fmt.Printf("      Lock contention: %.1f%% (%.1f%% reduction)\n", 
		shardedResult.contentionRate*100,
		(regularResult.contentionRate-shardedResult.contentionRate)/regularResult.contentionRate*100)
	
	// Show shard distribution
	fmt.Println("\n  Shard Distribution:")
	distribution := shardedStore.GetShardDistribution()
	for i := 0; i < shardCount; i++ {
		load := distribution[i]
		bar := generateBar(load, 20)
		fmt.Printf("    Shard %2d: %s %.1f%%\n", i, bar, load*100)
	}
	
	fmt.Println()
}

func demonstrateBatchProcessing() {
	fmt.Println("3. Batch Processing Demo")
	fmt.Println("------------------------")
	
	// Create store with batch optimization
	perfOptions := state.PerformanceOptions{
		EnableBatching:   true,
		BatchSize:        100,
		BatchTimeout:     50 * time.Millisecond,
		EnablePooling:    true,
		MaxOpsPerSecond:  10000,
	}
	
	optimizer := state.NewPerformanceOptimizer(perfOptions)
	store := state.NewStateStore(
		state.WithPerformanceOptimizer(optimizer),
	)
	
	fmt.Println("  Comparing individual vs batch operations...")
	
	// Test 1: Individual operations
	fmt.Println("\n  Individual operations:")
	individualStart := time.Now()
	
	for i := 0; i < 10000; i++ {
		store.Set(fmt.Sprintf("/individual/item_%d", i), map[string]interface{}{
			"id":    i,
			"value": rand.Float64(),
		})
	}
	
	individualDuration := time.Since(individualStart)
	individualOpsPerSec := float64(10000) / individualDuration.Seconds()
	
	// Test 2: Batch operations
	fmt.Println("\n  Batch operations:")
	batchStart := time.Now()
	
	batcher := optimizer.CreateBatcher()
	for i := 0; i < 10000; i++ {
		batcher.Add(state.BatchOperation{
			Type: state.OpTypeSet,
			Path: fmt.Sprintf("/batch/item_%d", i),
			Value: map[string]interface{}{
				"id":    i,
				"value": rand.Float64(),
			},
		})
	}
	batcher.Flush()
	
	batchDuration := time.Since(batchStart)
	batchOpsPerSec := float64(10000) / batchDuration.Seconds()
	
	// Show results
	fmt.Println("\n  Batch Processing Results:")
	fmt.Printf("    Individual operations:\n")
	fmt.Printf("      Duration: %v\n", individualDuration)
	fmt.Printf("      Throughput: %.0f ops/sec\n", individualOpsPerSec)
	
	fmt.Printf("\n    Batch operations:\n")
	fmt.Printf("      Duration: %v (%.1fx faster)\n", 
		batchDuration,
		float64(individualDuration)/float64(batchDuration))
	fmt.Printf("      Throughput: %.0f ops/sec (%.1fx improvement)\n", 
		batchOpsPerSec,
		batchOpsPerSec/individualOpsPerSec)
	fmt.Printf("      Batches processed: %d\n", batcher.GetProcessedBatches())
	fmt.Printf("      Avg batch size: %.1f\n", 10000.0/float64(batcher.GetProcessedBatches()))
	
	fmt.Println()
}

func demonstrateLazyLoading() {
	fmt.Println("4. Lazy Loading & Caching Demo")
	fmt.Println("------------------------------")
	
	// Create store with lazy loading
	perfOptions := state.PerformanceOptions{
		EnableLazyLoading: true,
		LazyCacheSize:     1000,
		CacheExpiryTime:   5 * time.Minute,
		EnablePooling:     true,
	}
	
	optimizer := state.NewPerformanceOptimizer(perfOptions)
	store := state.NewStateStore(
		state.WithPerformanceOptimizer(optimizer),
		state.WithLazyLoading(true),
	)
	
	// Populate test data
	fmt.Println("  Populating test data...")
	testDataSize := 10000
	
	for i := 0; i < testDataSize; i++ {
		store.Set(fmt.Sprintf("/lazy/data_%d", i), map[string]interface{}{
			"id":          i,
			"largeField":  generateLargeData(10 * 1024), // 10KB per entry
			"metadata":    map[string]interface{}{"created": time.Now()},
		})
	}
	
	// Simulate access patterns
	fmt.Println("\n  Simulating access patterns...")
	
	// Pattern 1: Hot data (20% of data accessed 80% of time)
	hotDataSize := testDataSize / 5
	accessCount := 10000
	
	var cacheHits, cacheMisses int64
	var totalLatency time.Duration
	
	for i := 0; i < accessCount; i++ {
		var key string
		if rand.Float64() < 0.8 {
			// Access hot data
			key = fmt.Sprintf("/lazy/data_%d", rand.Intn(hotDataSize))
		} else {
			// Access cold data
			key = fmt.Sprintf("/lazy/data_%d", hotDataSize+rand.Intn(testDataSize-hotDataSize))
		}
		
		start := time.Now()
		_, cached := store.GetCached(key)
		latency := time.Since(start)
		totalLatency += latency
		
		if cached {
			atomic.AddInt64(&cacheHits, 1)
		} else {
			atomic.AddInt64(&cacheMisses, 1)
		}
	}
	
	// Calculate metrics
	hitRate := float64(cacheHits) / float64(accessCount) * 100
	avgLatency := totalLatency / time.Duration(accessCount)
	
	fmt.Println("\n  Lazy Loading Results:")
	fmt.Printf("    Total accesses: %d\n", accessCount)
	fmt.Printf("    Cache hits: %d (%.1f%%)\n", cacheHits, hitRate)
	fmt.Printf("    Cache misses: %d (%.1f%%)\n", cacheMisses, 100-hitRate)
	fmt.Printf("    Average latency: %v\n", avgLatency)
	
	// Show cache statistics
	cacheStats := optimizer.GetCacheStats()
	fmt.Printf("\n    Cache Statistics:\n")
	fmt.Printf("      Size: %d/%d entries\n", cacheStats.CurrentSize, cacheStats.MaxSize)
	fmt.Printf("      Memory usage: %.2f MB\n", float64(cacheStats.MemoryUsage)/1024/1024)
	fmt.Printf("      Evictions: %d\n", cacheStats.Evictions)
	fmt.Printf("      Hit rate: %.1f%%\n", cacheStats.HitRate*100)
	
	fmt.Println()
}

func demonstrateMemoryOptimization() {
	fmt.Println("5. Memory Optimization Demo")
	fmt.Println("---------------------------")
	
	// Create memory-optimized store
	perfOptions := state.PerformanceOptions{
		EnablePooling:     true,
		EnableCompression: true,
		CompressionLevel:  6,
		MaxMemoryUsage:    100 * 1024 * 1024, // 100MB limit
		EnableSharding:    true,
		ShardCount:        8,
	}
	
	optimizer := state.NewPerformanceOptimizer(perfOptions)
	store := state.NewStateStore(
		state.WithPerformanceOptimizer(optimizer),
		state.WithMaxHistory(10), // Limit history
	)
	
	fmt.Println("  Testing memory usage with different data patterns...")
	
	// Track memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	startMem := memStats.Alloc
	
	// Pattern 1: Many small objects
	fmt.Println("\n  Pattern 1: Many small objects")
	for i := 0; i < 50000; i++ {
		store.Set(fmt.Sprintf("/small/obj_%d", i), map[string]interface{}{
			"id": i,
			"v":  rand.Intn(100),
		})
	}
	
	runtime.ReadMemStats(&memStats)
	smallObjMem := memStats.Alloc - startMem
	
	// Clear and test pattern 2
	store.Clear()
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	startMem = memStats.Alloc
	
	// Pattern 2: Few large objects
	fmt.Println("  Pattern 2: Few large objects")
	for i := 0; i < 100; i++ {
		store.Set(fmt.Sprintf("/large/obj_%d", i), map[string]interface{}{
			"id":   i,
			"data": generateLargeData(500 * 1024), // 500KB each
		})
	}
	
	runtime.ReadMemStats(&memStats)
	largeObjMem := memStats.Alloc - startMem
	
	// Show memory optimization results
	fmt.Println("\n  Memory Usage Comparison:")
	fmt.Printf("    Small objects (50k items):\n")
	fmt.Printf("      Memory used: %.2f MB\n", float64(smallObjMem)/1024/1024)
	fmt.Printf("      Per object: %.2f KB\n", float64(smallObjMem)/50000/1024)
	
	fmt.Printf("\n    Large objects (100 items):\n")
	fmt.Printf("      Memory used: %.2f MB\n", float64(largeObjMem)/1024/1024)
	fmt.Printf("      Per object: %.2f KB\n", float64(largeObjMem)/100/1024)
	fmt.Printf("      Compression ratio: %.1f%%\n", 
		float64(largeObjMem)/(100*500*1024)*100)
	
	// Show memory optimization techniques
	memStats2 := optimizer.GetMemoryStats()
	fmt.Printf("\n    Memory Optimization Stats:\n")
	fmt.Printf("      Objects pooled: %d\n", memStats2.PooledObjects)
	fmt.Printf("      Memory saved by pooling: %.2f MB\n", 
		float64(memStats2.PoolMemorySaved)/1024/1024)
	fmt.Printf("      Compressed objects: %d\n", memStats2.CompressedObjects)
	fmt.Printf("      Memory saved by compression: %.2f MB\n", 
		float64(memStats2.CompressionMemorySaved)/1024/1024)
	
	fmt.Println()
}

func runComprehensiveBenchmark() {
	fmt.Println("6. Comprehensive Performance Benchmark")
	fmt.Println("--------------------------------------")
	
	// Define benchmark scenarios
	scenarios := []BenchmarkScenario{
		{
			Name:        "Read Heavy",
			Description: "80% reads, 20% writes",
			WorkerCount: 50,
			Operations:  100000,
			DataSize:    1024,
			ReadRatio:   0.8,
		},
		{
			Name:        "Write Heavy",
			Description: "20% reads, 80% writes",
			WorkerCount: 50,
			Operations:  100000,
			DataSize:    1024,
			ReadRatio:   0.2,
		},
		{
			Name:        "Mixed Workload",
			Description: "50% reads, 50% writes",
			WorkerCount: 100,
			Operations:  200000,
			DataSize:    2048,
			ReadRatio:   0.5,
		},
		{
			Name:        "High Concurrency",
			Description: "Many workers, small operations",
			WorkerCount: 200,
			Operations:  50000,
			DataSize:    512,
			ReadRatio:   0.6,
		},
		{
			Name:        "Large Data",
			Description: "Few workers, large data",
			WorkerCount: 10,
			Operations:  10000,
			DataSize:    10240,
			ReadRatio:   0.7,
		},
	}
	
	// Run benchmarks with different configurations
	configs := []struct {
		name string
		opts state.PerformanceOptions
	}{
		{
			name: "Baseline",
			opts: state.PerformanceOptions{},
		},
		{
			name: "Optimized",
			opts: state.PerformanceOptions{
				EnablePooling:     true,
				EnableBatching:    true,
				EnableCompression: true,
				EnableLazyLoading: true,
				EnableSharding:    true,
				BatchSize:         100,
				BatchTimeout:      10 * time.Millisecond,
				ShardCount:        16,
				MaxConcurrency:    runtime.NumCPU() * 2,
			},
		},
	}
	
	results := make(map[string][]BenchmarkResult)
	
	for _, config := range configs {
		fmt.Printf("\n  Running benchmarks with %s configuration...\n", config.name)
		
		for _, scenario := range scenarios {
			result := runBenchmark(scenario, config.opts)
			results[config.name] = append(results[config.name], result)
			
			fmt.Printf("    %s: %.0f ops/sec, %.2fms avg latency\n",
				scenario.Name, result.OpsPerSecond, float64(result.AvgLatency)/float64(time.Millisecond))
		}
	}
	
	// Show comparison
	fmt.Println("\n  Performance Comparison (Optimized vs Baseline):")
	fmt.Println("  " + strings.Repeat("-", 80))
	fmt.Printf("  %-20s | %-15s | %-15s | %-10s\n", 
		"Scenario", "Baseline (ops/s)", "Optimized (ops/s)", "Improvement")
	fmt.Println("  " + strings.Repeat("-", 80))
	
	for i, scenario := range scenarios {
		baseline := results["Baseline"][i]
		optimized := results["Optimized"][i]
		improvement := (optimized.OpsPerSecond / baseline.OpsPerSecond - 1) * 100
		
		fmt.Printf("  %-20s | %15.0f | %15.0f | %9.1f%%\n",
			scenario.Name, baseline.OpsPerSecond, optimized.OpsPerSecond, improvement)
	}
	
	fmt.Println("  " + strings.Repeat("-", 80))
	
	// Show detailed results for best performing scenario
	var bestImprovement float64
	var bestScenario string
	for i, scenario := range scenarios {
		baseline := results["Baseline"][i]
		optimized := results["Optimized"][i]
		improvement := optimized.OpsPerSecond / baseline.OpsPerSecond
		
		if improvement > bestImprovement {
			bestImprovement = improvement
			bestScenario = scenario.Name
		}
	}
	
	fmt.Printf("\n  Best improvement: %s (%.1fx faster)\n", 
		bestScenario, bestImprovement)
}

// Helper functions

type allocationResult struct {
	allocations uint64
	memoryUsed  uint64
	gcRuns      uint32
	duration    time.Duration
}

func benchmarkAllocations(store *state.StateStore, optimizer *state.PerformanceOptimizer, usePooling bool) allocationResult {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	startAllocs := memStats.Mallocs
	startMem := memStats.Alloc
	startGC := memStats.NumGC
	
	start := time.Now()
	
	// Perform many operations that allocate objects
	for i := 0; i < 10000; i++ {
		// Create JSON patch operations
		patch := state.JSONPatch{
			{Op: state.JSONPatchOpAdd, Path: fmt.Sprintf("/alloc/item_%d", i), Value: i},
			{Op: state.JSONPatchOpReplace, Path: fmt.Sprintf("/alloc/item_%d", i), Value: i * 2},
		}
		
		// Apply in transaction
		tx := store.Begin()
		tx.Apply(patch)
		tx.Commit()
		
		// Create state changes
		store.Set(fmt.Sprintf("/alloc/data_%d", i), map[string]interface{}{
			"id":        i,
			"timestamp": time.Now(),
			"data":      make([]byte, 1024),
		})
	}
	
	duration := time.Since(start)
	
	runtime.ReadMemStats(&memStats)
	return allocationResult{
		allocations: memStats.Mallocs - startAllocs,
		memoryUsed:  memStats.Alloc - startMem,
		gcRuns:      memStats.NumGC - startGC,
		duration:    duration,
	}
}

type concurrentResult struct {
	duration       time.Duration
	opsPerSecond   float64
	contentionRate float64
}

func benchmarkConcurrentAccess(store state.Store, workers, opsPerWorker int) concurrentResult {
	var wg sync.WaitGroup
	var totalOps int64
	var contentions int64
	
	start := time.Now()
	
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for i := 0; i < opsPerWorker; i++ {
				key := fmt.Sprintf("/concurrent/w%d_i%d", workerID, i)
				
				// Mix of operations
				switch i % 3 {
				case 0: // Write
					if err := store.Set(key, i); err != nil {
						atomic.AddInt64(&contentions, 1)
					}
				case 1: // Read
					if _, err := store.Get(key); err != nil {
						atomic.AddInt64(&contentions, 1)
					}
				case 2: // Update
					store.Set(key, i*2)
				}
				
				atomic.AddInt64(&totalOps, 1)
			}
		}(w)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	return concurrentResult{
		duration:       duration,
		opsPerSecond:   float64(totalOps) / duration.Seconds(),
		contentionRate: float64(contentions) / float64(totalOps),
	}
}

func runBenchmark(scenario BenchmarkScenario, opts state.PerformanceOptions) BenchmarkResult {
	// Create optimizer and store
	optimizer := state.NewPerformanceOptimizer(opts)
	store := state.NewStateStore(
		state.WithPerformanceOptimizer(optimizer),
	)
	
	// Pre-populate some data
	for i := 0; i < 1000; i++ {
		store.Set(fmt.Sprintf("/bench/initial_%d", i), generateTestData(scenario.DataSize))
	}
	
	// Run benchmark
	var wg sync.WaitGroup
	var totalOps int64
	var totalLatency int64
	latencies := make([]time.Duration, 0, scenario.Operations)
	latenciesMu := sync.Mutex{}
	
	start := time.Now()
	
	opsPerWorker := scenario.Operations / scenario.WorkerCount
	
	for w := 0; w < scenario.WorkerCount; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for i := 0; i < opsPerWorker; i++ {
				opStart := time.Now()
				
				if rand.Float64() < scenario.ReadRatio {
					// Read operation
					key := fmt.Sprintf("/bench/initial_%d", rand.Intn(1000))
					store.Get(key)
				} else {
					// Write operation
					key := fmt.Sprintf("/bench/w%d_i%d", workerID, i)
					store.Set(key, generateTestData(scenario.DataSize))
				}
				
				latency := time.Since(opStart)
				atomic.AddInt64(&totalOps, 1)
				atomic.AddInt64(&totalLatency, int64(latency))
				
				latenciesMu.Lock()
				latencies = append(latencies, latency)
				latenciesMu.Unlock()
			}
		}(w)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	// Calculate percentiles
	p50, p95, p99 := calculatePercentiles(latencies)
	
	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return BenchmarkResult{
		Scenario:     scenario.Name,
		Duration:     duration,
		TotalOps:     totalOps,
		OpsPerSecond: float64(totalOps) / duration.Seconds(),
		AvgLatency:   time.Duration(totalLatency / totalOps),
		P50Latency:   p50,
		P95Latency:   p95,
		P99Latency:   p99,
		MemoryUsed:   memStats.Alloc,
		Allocations:  memStats.Mallocs,
		GCPauses:     memStats.NumGC,
		CPUUsage:     0, // Would need more sophisticated measurement
	}
}

func generateTestData(size int) map[string]interface{} {
	return map[string]interface{}{
		"id":        rand.Int63(),
		"timestamp": time.Now().Unix(),
		"data":      make([]byte, size),
		"metadata": map[string]interface{}{
			"version": 1,
			"source":  "benchmark",
		},
	}
}

func generateLargeData(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

func generateBar(value float64, width int) string {
	filled := int(value * float64(width))
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return bar
}

func calculatePercentiles(latencies []time.Duration) (p50, p95, p99 time.Duration) {
	if len(latencies) == 0 {
		return
	}
	
	// Sort latencies
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	p50 = sorted[len(sorted)*50/100]
	p95 = sorted[len(sorted)*95/100]
	p99 = sorted[len(sorted)*99/100]
	
	return
}

var strings = struct {
	Repeat func(string, int) string
}{
	Repeat: func(s string, n int) string {
		result := ""
		for i := 0; i < n; i++ {
			result += s
		}
		return result
	},
}