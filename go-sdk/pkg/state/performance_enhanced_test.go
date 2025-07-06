package state

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestPerformanceEnhancements tests the new performance features
func TestPerformanceEnhancements(t *testing.T) {
	t.Run("ConnectionPool", testConnectionPool)
	t.Run("StateSharding", testStateSharding)
	t.Run("LazyLoading", testLazyLoading)
	t.Run("MemoryOptimization", testMemoryOptimization)
	t.Run("ConcurrentOptimization", testConcurrentOptimization)
	t.Run("DataCompression", testDataCompression)
	t.Run("LargeStateHandling", testLargeStateHandling)
	t.Run("HighConcurrency", testHighConcurrency)
}

func testConnectionPool(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()
	
	// Test getting and putting connections
	conn1, err := pool.Get()
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}
	
	if !conn1.IsValid() {
		t.Error("Connection should be valid")
	}
	
	pool.Put(conn1)
	
	// Test pool exhaustion
	var conns []Connection
	for i := 0; i < 5; i++ {
		conn, err := pool.Get()
		if err != nil {
			t.Fatalf("Failed to get connection %d: %v", i, err)
		}
		conns = append(conns, conn)
	}
	
	// This should fail as pool is exhausted
	_, err = pool.Get()
	if err == nil {
		t.Error("Expected error when pool is exhausted")
	}
	
	// Return connections to pool
	for _, conn := range conns {
		pool.Put(conn)
	}
}

func testStateSharding(t *testing.T) {
	opts := DefaultPerformanceOptions()
	opts.EnableSharding = true
	opts.ShardCount = 4
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	// Test sharding distribution
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	shardCounts := make(map[int]int)
	
	for _, key := range keys {
		shardIndex := po.GetShardForKey(key)
		shardCounts[shardIndex]++
		
		// Test shard operations
		po.ShardedSet(key, fmt.Sprintf("value-%s", key))
		
		value, found := po.ShardedGet(key)
		if !found {
			t.Errorf("Expected to find key %s in shard", key)
		}
		
		expectedValue := fmt.Sprintf("value-%s", key)
		if value != expectedValue {
			t.Errorf("Expected %s, got %s", expectedValue, value)
		}
	}
	
	// Verify distribution across shards
	if len(shardCounts) == 0 {
		t.Error("No shards were used")
	}
}

func testLazyLoading(t *testing.T) {
	opts := DefaultPerformanceOptions()
	opts.EnableLazyLoading = true
	opts.LazyCacheSize = 10
	opts.CacheExpiryTime = 100 * time.Millisecond
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	loadCount := 0
	loader := func() (interface{}, error) {
		loadCount++
		return fmt.Sprintf("loaded-value-%d", loadCount), nil
	}
	
	// First load should call loader
	value1, err := po.LazyLoadState("test-key", loader)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}
	
	if loadCount != 1 {
		t.Errorf("Expected loader to be called once, got %d", loadCount)
	}
	
	// Second load should use cache
	value2, err := po.LazyLoadState("test-key", loader)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}
	
	if loadCount != 1 {
		t.Errorf("Expected loader to be called once (cached), got %d", loadCount)
	}
	
	if value1 != value2 {
		t.Errorf("Expected cached value to match, got %s vs %s", value1, value2)
	}
	
	// Wait for cache expiry
	time.Sleep(150 * time.Millisecond)
	
	// Third load should call loader again
	_, err = po.LazyLoadState("test-key", loader)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}
	
	if loadCount != 2 {
		t.Errorf("Expected loader to be called twice (after expiry), got %d", loadCount)
	}
}

func testMemoryOptimization(t *testing.T) {
	maxMemory := int64(1024 * 1024) // 1MB
	mo := NewMemoryOptimizer(maxMemory)
	
	// Test memory usage tracking
	mo.UpdateMemoryUsage(512 * 1024) // 512KB
	
	if !mo.CheckMemoryUsage() {
		t.Error("Memory usage should be within limits")
	}
	
	usage := mo.GetMemoryUsage()
	if usage != 512*1024 {
		t.Errorf("Expected memory usage to be 512KB, got %d", usage)
	}
	
	// Test memory limit exceeded
	mo.UpdateMemoryUsage(1024 * 1024) // Another 1MB, total 1.5MB
	
	if mo.CheckMemoryUsage() {
		t.Error("Memory usage should exceed limits")
	}
}

func testConcurrentOptimization(t *testing.T) {
	maxConcurrency := 4
	co := NewConcurrentOptimizer(maxConcurrency)
	defer co.Shutdown()
	
	// Test task execution
	executed := make([]bool, 10)
	var wg sync.WaitGroup
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		idx := i
		success := co.Execute(func() {
			executed[idx] = true
			wg.Done()
		})
		
		if !success && idx < maxConcurrency*2 {
			t.Errorf("Expected task %d to be accepted", idx)
		}
	}
	
	wg.Wait()
	
	// Check that some tasks were executed
	executedCount := 0
	for _, exec := range executed {
		if exec {
			executedCount++
		}
	}
	
	if executedCount == 0 {
		t.Error("No tasks were executed")
	}
}

func testDataCompression(t *testing.T) {
	opts := DefaultPerformanceOptions()
	opts.EnableCompression = true
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	// Test data compression
	originalData := []byte("This is a test string for compression. " +
		"It should compress well because it has repetitive patterns. " +
		"Compression is important for performance optimization.")
	
	compressedData, err := po.CompressData(originalData)
	if err != nil {
		t.Fatalf("Failed to compress data: %v", err)
	}
	
	if len(compressedData) >= len(originalData) {
		t.Error("Compressed data should be smaller than original")
	}
	
	// Test decompression
	decompressedData, err := po.DecompressData(compressedData)
	if err != nil {
		t.Fatalf("Failed to decompress data: %v", err)
	}
	
	if string(decompressedData) != string(originalData) {
		t.Error("Decompressed data should match original")
	}
}

func testLargeStateHandling(t *testing.T) {
	opts := DefaultPerformanceOptions()
	opts.EnableSharding = true
	opts.EnableLazyLoading = true
	opts.EnableCompression = true
	opts.MaxMemoryUsage = 10 * 1024 * 1024 // 10MB
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	// Simulate large state
	largeStateSize := int64(150 * 1024 * 1024) // 150MB
	po.OptimizeForLargeState(largeStateSize)
	
	// Test that optimizations are enabled
	if !po.enableCompression {
		t.Error("Compression should be enabled for large states")
	}
	
	if !po.enableSharding {
		t.Error("Sharding should be enabled for large states")
	}
	
	if !po.enableLazyLoading {
		t.Error("Lazy loading should be enabled for large states")
	}
}

func testHighConcurrency(t *testing.T) {
	opts := DefaultPerformanceOptions()
	opts.MaxConcurrency = 100
	opts.MaxOpsPerSecond = 10000
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	// Test high concurrency operations
	numGoroutines := 1000
	numOpsPerGoroutine := 10
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOpsPerGoroutine)
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numOpsPerGoroutine; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				
				err := po.ProcessLargeStateUpdate(ctx, func() error {
					// Simulate some work
					time.Sleep(time.Microsecond)
					return nil
				})
				
				cancel()
				
				if err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}
}

// BenchmarkPerformanceEnhancements benchmarks the performance enhancements
func BenchmarkPerformanceEnhancements(b *testing.B) {
	b.Run("ConnectionPool", benchmarkConnectionPool)
	b.Run("StateSharding", benchmarkStateSharding)
	b.Run("LazyLoading", benchmarkLazyLoading)
	b.Run("DataCompression", benchmarkDataCompression)
	b.Run("ConcurrentAccess", benchmarkConcurrentAccess)
}

func benchmarkConnectionPool(b *testing.B) {
	pool := NewConnectionPool(10)
	defer pool.Close()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := pool.Get()
			if err != nil {
				b.Fatal(err)
			}
			pool.Put(conn)
		}
	})
}

func benchmarkStateSharding(b *testing.B) {
	opts := DefaultPerformanceOptions()
	opts.EnableSharding = true
	opts.ShardCount = 16
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i)
			po.ShardedSet(key, fmt.Sprintf("value-%d", i))
			po.ShardedGet(key)
			i++
		}
	})
}

func benchmarkLazyLoading(b *testing.B) {
	opts := DefaultPerformanceOptions()
	opts.EnableLazyLoading = true
	opts.LazyCacheSize = 1000
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%100) // Reuse keys for cache hits
			po.LazyLoadState(key, func() (interface{}, error) {
				return fmt.Sprintf("value-%d", i), nil
			})
			i++
		}
	})
}

func benchmarkDataCompression(b *testing.B) {
	opts := DefaultPerformanceOptions()
	opts.EnableCompression = true
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	data := make([]byte, 1024) // 1KB of data
	for i := range data {
		data[i] = byte(i % 256)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			compressed, err := po.CompressData(data)
			if err != nil {
				b.Fatal(err)
			}
			_, err = po.DecompressData(compressed)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func benchmarkConcurrentAccess(b *testing.B) {
	opts := DefaultPerformanceOptions()
	opts.MaxConcurrency = runtime.NumCPU()
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			po.ProcessLargeStateUpdate(ctx, func() error {
				// Simulate some work
				runtime.Gosched()
				return nil
			})
		}
	})
}

// TestPerformanceTargets tests that the system meets performance targets
func TestPerformanceTargets(t *testing.T) {
	t.Run("HighConcurrency", testHighConcurrencyTarget)
	t.Run("LargeStateSize", testLargeStateSizeTarget)
	t.Run("LowLatency", testLowLatencyTarget)
	t.Run("MemoryEfficiency", testMemoryEfficiencyTarget)
}

func testHighConcurrencyTarget(t *testing.T) {
	// Target: Support >1000 concurrent clients
	opts := DefaultPerformanceOptions()
	opts.MaxConcurrency = 1000
	opts.MaxOpsPerSecond = 100000
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	numClients := 1200
	var wg sync.WaitGroup
	errors := make(chan error, numClients)
	
	start := time.Now()
	
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			err := po.ProcessLargeStateUpdate(ctx, func() error {
				// Simulate client work
				time.Sleep(time.Millisecond)
				return nil
			})
			
			if err != nil {
				errors <- err
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	duration := time.Since(start)
	
	// Check for errors
	errorCount := 0
	for range errors {
		errorCount++
	}
	
	if errorCount > numClients/10 { // Allow up to 10% errors
		t.Errorf("Too many errors: %d/%d", errorCount, numClients)
	}
	
	t.Logf("Processed %d concurrent clients in %v", numClients, duration)
}

func testLargeStateSizeTarget(t *testing.T) {
	// Target: Handle state sizes >100MB efficiently
	opts := DefaultPerformanceOptions()
	opts.EnableSharding = true
	opts.EnableCompression = true
	opts.MaxMemoryUsage = 200 * 1024 * 1024 // 200MB
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	// Simulate 150MB state
	largeStateSize := int64(150 * 1024 * 1024)
	
	start := time.Now()
	po.OptimizeForLargeState(largeStateSize)
	duration := time.Since(start)
	
	// Should complete quickly even for large states
	if duration > 100*time.Millisecond {
		t.Errorf("Large state optimization took too long: %v", duration)
	}
	
	t.Logf("Optimized %dMB state in %v", largeStateSize/(1024*1024), duration)
}

func testLowLatencyTarget(t *testing.T) {
	// Target: Maintain <10ms state update latency
	opts := DefaultPerformanceOptions()
	opts.EnableBatching = true
	opts.BatchSize = 10
	opts.BatchTimeout = 1 * time.Millisecond
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	numOperations := 100
	latencies := make([]time.Duration, numOperations)
	
	for i := 0; i < numOperations; i++ {
		start := time.Now()
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		
		err := po.BatchOperation(ctx, func() error {
			// Simulate state update
			time.Sleep(time.Microsecond)
			return nil
		})
		
		cancel()
		
		if err != nil {
			t.Fatalf("Batch operation failed: %v", err)
		}
		
		latencies[i] = time.Since(start)
	}
	
	// Calculate average latency
	var totalLatency time.Duration
	for _, latency := range latencies {
		totalLatency += latency
	}
	avgLatency := totalLatency / time.Duration(numOperations)
	
	// Target: <10ms average latency
	if avgLatency > 10*time.Millisecond {
		t.Errorf("Average latency too high: %v (target: <10ms)", avgLatency)
	}
	
	t.Logf("Average latency: %v", avgLatency)
}

func testMemoryEfficiencyTarget(t *testing.T) {
	// Target: Optimize memory usage for large states
	opts := DefaultPerformanceOptions()
	opts.EnablePooling = true
	opts.MaxMemoryUsage = 50 * 1024 * 1024 // 50MB
	
	po := NewPerformanceOptimizer(opts)
	defer po.Stop()
	
	// Measure memory usage before operations
	var memStatsBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsBefore)
	
	// Perform many operations that would normally allocate memory
	numOperations := 10000
	for i := 0; i < numOperations; i++ {
		// Use object pools
		patch := po.GetPatchOperation()
		patch.Op = JSONPatchOpAdd
		patch.Path = fmt.Sprintf("/test-%d", i)
		patch.Value = fmt.Sprintf("value-%d", i)
		po.PutPatchOperation(patch)
		
		stateChange := po.GetStateChange()
		stateChange.Path = fmt.Sprintf("/test-%d", i)
		stateChange.Operation = "add"
		stateChange.NewValue = fmt.Sprintf("value-%d", i)
		po.PutStateChange(stateChange)
		
		event := po.GetStateEvent()
		event.Type = "change"
		event.Path = fmt.Sprintf("/test-%d", i)
		event.Value = fmt.Sprintf("value-%d", i)
		po.PutStateEvent(event)
	}
	
	// Measure memory usage after operations
	var memStatsAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsAfter)
	
	// Calculate memory efficiency
	memoryIncrease := memStatsAfter.Alloc - memStatsBefore.Alloc
	metrics := po.GetMetrics()
	
	t.Logf("Memory increase: %d bytes", memoryIncrease)
	t.Logf("Pool efficiency: %.2f%%", metrics.PoolEfficiency)
	t.Logf("Pool hits: %d, misses: %d", metrics.PoolHits, metrics.PoolMisses)
	
	// Pool efficiency should be high (>80%)
	if metrics.PoolEfficiency < 80 {
		t.Errorf("Pool efficiency too low: %.2f%% (target: >80%%)", metrics.PoolEfficiency)
	}
}