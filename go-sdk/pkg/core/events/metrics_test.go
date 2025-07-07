package events

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestRingBuffer tests the ring buffer implementation
func TestRingBuffer(t *testing.T) {
	t.Run("basic functionality", func(t *testing.T) {
		capacity := 5
		rb := NewRingBuffer(capacity)
		
		if rb.Size() != 0 {
			t.Errorf("Expected size 0, got %d", rb.Size())
		}
		
		if rb.Capacity() != capacity {
			t.Errorf("Expected capacity %d, got %d", capacity, rb.Capacity())
		}
		
		// Test empty buffer
		all := rb.GetAll()
		if len(all) != 0 {
			t.Errorf("Expected empty buffer, got %d elements", len(all))
		}
		
		recent := rb.GetRecent(3)
		if len(recent) != 0 {
			t.Errorf("Expected empty buffer, got %d elements", len(recent))
		}
	})
	
	t.Run("adding elements", func(t *testing.T) {
		rb := NewRingBuffer(3)
		
		// Add first element
		metric1 := MemoryUsageMetric{
			Timestamp:  time.Now(),
			AllocBytes: 1000,
		}
		rb.Add(metric1)
		
		if rb.Size() != 1 {
			t.Errorf("Expected size 1, got %d", rb.Size())
		}
		
		all := rb.GetAll()
		if len(all) != 1 || all[0].AllocBytes != 1000 {
			t.Errorf("Expected [1000], got %v", all)
		}
		
		// Add second element
		metric2 := MemoryUsageMetric{
			Timestamp:  time.Now(),
			AllocBytes: 2000,
		}
		rb.Add(metric2)
		
		if rb.Size() != 2 {
			t.Errorf("Expected size 2, got %d", rb.Size())
		}
		
		all = rb.GetAll()
		if len(all) != 2 || all[0].AllocBytes != 1000 || all[1].AllocBytes != 2000 {
			t.Errorf("Expected [1000, 2000], got %v", all)
		}
	})
	
	t.Run("ring buffer overflow", func(t *testing.T) {
		rb := NewRingBuffer(3)
		
		// Add more elements than capacity
		for i := 1; i <= 5; i++ {
			metric := MemoryUsageMetric{
				Timestamp:  time.Now(),
				AllocBytes: uint64(i * 1000),
			}
			rb.Add(metric)
		}
		
		// Should only have the last 3 elements
		if rb.Size() != 3 {
			t.Errorf("Expected size 3, got %d", rb.Size())
		}
		
		all := rb.GetAll()
		if len(all) != 3 {
			t.Errorf("Expected 3 elements, got %d", len(all))
		}
		
		// Should have elements 3, 4, 5 (oldest to newest)
		expected := []uint64{3000, 4000, 5000}
		for i, metric := range all {
			if metric.AllocBytes != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, metric.AllocBytes)
			}
		}
	})
	
	t.Run("get recent", func(t *testing.T) {
		rb := NewRingBuffer(5)
		
		// Add 4 elements
		for i := 1; i <= 4; i++ {
			metric := MemoryUsageMetric{
				Timestamp:  time.Now(),
				AllocBytes: uint64(i * 1000),
			}
			rb.Add(metric)
		}
		
		// Get recent 2
		recent := rb.GetRecent(2)
		if len(recent) != 2 {
			t.Errorf("Expected 2 recent elements, got %d", len(recent))
		}
		
		// Should be newest first: [4000, 3000]
		if recent[0].AllocBytes != 4000 || recent[1].AllocBytes != 3000 {
			t.Errorf("Expected [4000, 3000], got [%d, %d]", recent[0].AllocBytes, recent[1].AllocBytes)
		}
		
		// Get more than available
		recent = rb.GetRecent(10)
		if len(recent) != 4 {
			t.Errorf("Expected 4 elements (all available), got %d", len(recent))
		}
	})
	
	t.Run("clear", func(t *testing.T) {
		rb := NewRingBuffer(3)
		
		// Add elements
		for i := 1; i <= 3; i++ {
			metric := MemoryUsageMetric{
				Timestamp:  time.Now(),
				AllocBytes: uint64(i * 1000),
			}
			rb.Add(metric)
		}
		
		rb.Clear()
		
		if rb.Size() != 0 {
			t.Errorf("Expected size 0 after clear, got %d", rb.Size())
		}
		
		all := rb.GetAll()
		if len(all) != 0 {
			t.Errorf("Expected empty buffer after clear, got %d elements", len(all))
		}
	})
}

// TestRingBufferConcurrency tests thread safety of the ring buffer
func TestRingBufferConcurrency(t *testing.T) {
	rb := NewRingBuffer(100)
	numGoroutines := 10
	elementsPerGoroutine := 50
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Start multiple goroutines adding elements
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < elementsPerGoroutine; j++ {
				metric := MemoryUsageMetric{
					Timestamp:  time.Now(),
					AllocBytes: uint64(goroutineID*1000 + j),
				}
				rb.Add(metric)
				
				// Occasionally read data
				if j%10 == 0 {
					rb.GetAll()
					rb.GetRecent(5)
					rb.Size()
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify final state
	size := rb.Size()
	if size != 100 { // Should be at capacity
		t.Errorf("Expected size 100, got %d", size)
	}
	
	all := rb.GetAll()
	if len(all) != 100 {
		t.Errorf("Expected 100 elements, got %d", len(all))
	}
}

// TestOptimizedMemoryStats tests the optimized memory stats collector
func TestOptimizedMemoryStats(t *testing.T) {
	t.Run("caching behavior", func(t *testing.T) {
		oms := NewOptimizedMemoryStats(100 * time.Millisecond)
		
		// First call should read from runtime
		stats1 := oms.GetStats()
		
		// Second call immediately should return cached value
		stats2 := oms.GetStats()
		
		// Should be identical (same instance)
		if stats1.Alloc != stats2.Alloc {
			t.Errorf("Expected cached stats to be identical")
		}
		
		// Wait for cache to expire
		time.Sleep(150 * time.Millisecond)
		
		// Should read fresh stats
		stats3 := oms.GetStats()
		
		// Stats might be different due to GC activity
		_ = stats3
	})
	
	t.Run("concurrent access", func(t *testing.T) {
		oms := NewOptimizedMemoryStats(50 * time.Millisecond)
		numGoroutines := 20
		
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					stats := oms.GetStats()
					_ = stats // Use the stats to avoid optimization
					time.Sleep(time.Millisecond)
				}
			}()
		}
		
		wg.Wait()
	})
}

// TestValidationPerformanceMetrics tests the main metrics struct
func TestValidationPerformanceMetrics(t *testing.T) {
	t.Run("initialization", func(t *testing.T) {
		config := DefaultMetricsConfig()
		config.RingBufferSize = 10
		config.GCStatsInterval = 100 * time.Millisecond
		
		metrics, err := NewValidationPerformanceMetrics(config)
		if err != nil {
			t.Fatalf("Failed to create metrics: %v", err)
		}
		defer metrics.Shutdown()
		
		if metrics.memoryHistory.Capacity() != 10 {
			t.Errorf("Expected ring buffer capacity 10, got %d", metrics.memoryHistory.Capacity())
		}
	})
	
	t.Run("optimized memory usage", func(t *testing.T) {
		config := DefaultMetricsConfig()
		config.GCStatsInterval = 50 * time.Millisecond
		
		metrics, err := NewValidationPerformanceMetrics(config)
		if err != nil {
			t.Fatalf("Failed to create metrics: %v", err)
		}
		defer metrics.Shutdown()
		
		// Test optimized memory usage
		memUsage1 := metrics.GetOptimizedMemoryUsage()
		memUsage2 := metrics.GetOptimizedMemoryUsage()
		
		// Should be from cache (same timestamp within interval)
		if memUsage1.Timestamp.After(memUsage2.Timestamp) {
			t.Errorf("Expected cached timestamp, but got newer timestamp")
		}
		
		// Wait for cache to expire
		time.Sleep(100 * time.Millisecond)
		
		memUsage3 := metrics.GetOptimizedMemoryUsage()
		if !memUsage3.Timestamp.After(memUsage1.Timestamp) {
			t.Errorf("Expected fresh timestamp after cache expiry")
		}
	})
	
	t.Run("memory history with ring buffer", func(t *testing.T) {
		config := DefaultMetricsConfig()
		config.RingBufferSize = 5
		config.EnableLeakDetection = false // Disable to avoid background routine
		
		metrics, err := NewValidationPerformanceMetrics(config)
		if err != nil {
			t.Fatalf("Failed to create metrics: %v", err)
		}
		defer metrics.Shutdown()
		
		// Manually add memory usage to ring buffer
		for i := 1; i <= 7; i++ {
			memUsage := MemoryUsageMetric{
				Timestamp:  time.Now(),
				AllocBytes: uint64(i * 1000),
			}
			metrics.memoryHistory.Add(memUsage)
		}
		
		// Should only have last 5 entries
		history := metrics.GetMemoryHistory()
		if len(history) != 5 {
			t.Errorf("Expected 5 entries in history, got %d", len(history))
		}
		
		// Should have entries 3, 4, 5, 6, 7
		expected := []uint64{3000, 4000, 5000, 6000, 7000}
		for i, entry := range history {
			if entry.AllocBytes != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, entry.AllocBytes)
			}
		}
	})
}

// BenchmarkRingBuffer benchmarks the ring buffer operations
func BenchmarkRingBuffer(b *testing.B) {
	b.Run("Add", func(b *testing.B) {
		rb := NewRingBuffer(1000)
		metric := MemoryUsageMetric{
			Timestamp:  time.Now(),
			AllocBytes: 1000,
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rb.Add(metric)
		}
	})
	
	b.Run("GetAll", func(b *testing.B) {
		rb := NewRingBuffer(1000)
		metric := MemoryUsageMetric{
			Timestamp:  time.Now(),
			AllocBytes: 1000,
		}
		
		// Fill the buffer
		for i := 0; i < 1000; i++ {
			rb.Add(metric)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rb.GetAll()
		}
	})
	
	b.Run("GetRecent", func(b *testing.B) {
		rb := NewRingBuffer(1000)
		metric := MemoryUsageMetric{
			Timestamp:  time.Now(),
			AllocBytes: 1000,
		}
		
		// Fill the buffer
		for i := 0; i < 1000; i++ {
			rb.Add(metric)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rb.GetRecent(10)
		}
	})
}

// BenchmarkMemoryStatsComparison compares optimized vs direct memory stats
func BenchmarkMemoryStatsComparison(b *testing.B) {
	b.Run("DirectReadMemStats", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
		}
	})
	
	b.Run("OptimizedMemoryStats", func(b *testing.B) {
		oms := NewOptimizedMemoryStats(10 * time.Millisecond)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			oms.GetStats()
		}
	})
}

// BenchmarkRingBufferVsSlice compares ring buffer vs slice operations
func BenchmarkRingBufferVsSlice(b *testing.B) {
	metric := MemoryUsageMetric{
		Timestamp:  time.Now(),
		AllocBytes: 1000,
	}
	
	b.Run("RingBufferAdd", func(b *testing.B) {
		rb := NewRingBuffer(60)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rb.Add(metric)
		}
	})
	
	b.Run("SliceAppendWithTruncation", func(b *testing.B) {
		history := make([]MemoryUsageMetric, 0, 60)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			history = append(history, metric)
			if len(history) > 60 {
				history = history[len(history)-60:]
			}
		}
	})
}