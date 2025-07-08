package websocket

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
)

// TestPerformanceConfig tests the performance configuration
func TestPerformanceConfig(t *testing.T) {
	config := DefaultPerformanceConfig()

	assert.Equal(t, 1000, config.MaxConcurrentConnections)
	assert.Equal(t, 10, config.MessageBatchSize)
	assert.Equal(t, 5*time.Millisecond, config.MessageBatchTimeout)
	assert.Equal(t, 1000, config.BufferPoolSize)
	assert.Equal(t, 64*1024, config.MaxBufferSize)
	assert.True(t, config.EnableZeroCopy)
	assert.True(t, config.EnableMemoryPooling)
	assert.Equal(t, 50*time.Millisecond, config.MaxLatency)
	assert.Equal(t, int64(80*1024*1024), config.MaxMemoryUsage)
	assert.True(t, config.EnableMetrics)
	assert.Equal(t, OptimizedJSONSerializer, config.MessageSerializerType)
}

// TestBufferPool tests the buffer pool functionality
func TestBufferPool(t *testing.T) {
	bp := NewBufferPool(10, 1024)

	// Test basic get/put operations
	buf1 := bp.Get()
	assert.NotNil(t, buf1)
	assert.Equal(t, 0, len(buf1))
	assert.Equal(t, 1024, cap(buf1))

	// Use the buffer
	buf1 = append(buf1, []byte("test data")...)

	// Put it back
	bp.Put(buf1)

	// Get another buffer - should be reused
	buf2 := bp.Get()
	assert.NotNil(t, buf2)
	assert.Equal(t, 0, len(buf2)) // Length should be reset
	assert.Equal(t, 1024, cap(buf2))

	bp.Put(buf2)

	// Check stats
	stats := bp.GetStats()
	assert.Equal(t, int64(2), stats["gets"])
	assert.Equal(t, int64(2), stats["puts"])
}

// TestMessageBatcher tests message batching functionality
func TestMessageBatcher(t *testing.T) {
	mb := NewMessageBatcher(3, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go mb.Start(ctx, &wg)

	// Add some messages
	testData := []byte("test message")
	for i := 0; i < 5; i++ {
		err := mb.AddMessage(testData)
		assert.NoError(t, err)
	}

	// Wait a bit for batching
	time.Sleep(50 * time.Millisecond)

	// Should have at least one batch
	batch := mb.GetBatch()
	if batch != nil {
		assert.LessOrEqual(t, len(batch), 3) // Batch size should not exceed limit
	}

	cancel()
	wg.Wait()
}

// TestConnectionPoolManager tests connection pool management
func TestConnectionPoolManager(t *testing.T) {
	cpm := NewConnectionPoolManager(5)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Acquire a slot
	slot, err := cpm.AcquireSlot(ctx)
	require.NoError(t, err)
	assert.NotNil(t, slot)
	assert.True(t, slot.InUse)

	// Release the slot
	cpm.ReleaseSlot(slot)
	assert.False(t, slot.InUse)

	// Check stats
	stats := cpm.GetStats()
	assert.Equal(t, int64(1), stats["slots_acquired"])
	assert.Equal(t, int64(1), stats["slots_released"])
}

// TestSerializerFactory tests serializer factory functionality
func TestSerializerFactory(t *testing.T) {
	sf := NewSerializerFactory(OptimizedJSONSerializer)

	serializer := sf.GetSerializer()
	assert.NotNil(t, serializer)

	// Test serialization
	testEvent := &testMockEvent{
		eventType: events.EventType("test"),
		data:      map[string]interface{}{"message": "test"},
	}

	data, err := serializer.Serialize(testEvent)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Put serializer back
	sf.PutSerializer(serializer)
}

// TestZeroCopyBuffer tests zero-copy buffer operations
func TestZeroCopyBuffer(t *testing.T) {
	data := []byte("hello world test data")
	zcb := NewZeroCopyBuffer(data)

	// Test initial state
	assert.Equal(t, len(data), zcb.Len())
	assert.Equal(t, string(data), zcb.String())

	// Test advance
	zcb.Advance(6) // Skip "hello "
	assert.Equal(t, "world test data", zcb.String())
	assert.Equal(t, 15, zcb.Len())

	// Test reset
	zcb.Reset()
	assert.Equal(t, string(data), zcb.String())
	assert.Equal(t, len(data), zcb.Len())
}

// TestMetricsCollector tests metrics collection
func TestMetricsCollector(t *testing.T) {
	mc := NewMetricsCollector(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go mc.Start(ctx, &wg)

	// Track some metrics
	mc.TrackConnectionTime(10 * time.Millisecond)
	mc.TrackMessageLatency(5 * time.Millisecond)
	mc.TrackSerializationTime(1 * time.Millisecond)
	mc.TrackMessageSize(1024)
	mc.TrackError("connection")

	// Wait for metrics collection
	time.Sleep(150 * time.Millisecond)

	metrics := mc.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, int64(1), metrics.TotalConnections)
	assert.Equal(t, 10*time.Millisecond, metrics.AvgConnectionTime)
	assert.Equal(t, 5*time.Millisecond, metrics.AvgLatency)
	assert.Equal(t, 1*time.Millisecond, metrics.SerializationTime)
	assert.Equal(t, 1024.0, metrics.AvgMessageSize)
	assert.Equal(t, int64(1), metrics.TotalErrors)
	assert.Equal(t, int64(1), metrics.ConnectionErrors)
	assert.True(t, metrics.GoroutineCount > 0)
	assert.True(t, metrics.MemoryUsage > 0)

	cancel()
	wg.Wait()
}

// TestMemoryManager tests memory management
func TestMemoryManager(t *testing.T) {
	maxMemory := int64(1024 * 1024) // 1MB
	mm := NewMemoryManager(maxMemory)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go mm.Start(ctx, &wg)

	// Test allocation within limits
	buf1 := mm.AllocateBuffer(512 * 1024) // 512KB
	assert.NotNil(t, buf1)
	assert.Equal(t, 512*1024, len(buf1))

	// Test deallocation
	mm.DeallocateBuffer(buf1)

	// Check stats
	stats := mm.GetStats()
	assert.Equal(t, int64(1), stats["allocations"])
	assert.Equal(t, int64(1), stats["deallocations"])
	assert.Equal(t, maxMemory, stats["max_memory"])

	cancel()
	wg.Wait()
}

// TestDynamicMemoryMonitoring tests dynamic memory monitoring intervals
func TestDynamicMemoryMonitoring(t *testing.T) {
	// Use a larger memory limit to account for baseline process memory
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	baselineMemory := int64(memStats.Alloc)
	maxMemory := baselineMemory * 2 // Allow 2x baseline memory
	mm := NewMemoryManager(maxMemory)

	// Test initial state - should start with low pressure interval
	assert.Equal(t, 60*time.Second, mm.GetMonitoringInterval())
	assert.Equal(t, 0.0, mm.GetMemoryPressure())

	// Test different pressure levels
	testCases := []struct {
		name             string
		pressure         float64
		expectedInterval time.Duration
	}{
		{"Low pressure", 30.0, 60 * time.Second},
		{"Medium pressure", 60.0, 15 * time.Second},
		{"High pressure", 85.0, 2 * time.Second},
		{"Critical pressure", 95.0, 500 * time.Millisecond},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			interval := mm.getMonitoringInterval(tc.pressure)
			assert.Equal(t, tc.expectedInterval, interval,
				"For pressure %.1f%%, expected interval %v", tc.pressure, tc.expectedInterval)
		})
	}

	// Test dynamic adjustment during runtime
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go mm.Start(ctx, &wg)

	// Wait for initial monitoring cycle
	time.Sleep(200 * time.Millisecond)

	// Calculate how much to allocate to reach 60% of max memory
	// We need to account for current usage
	runtime.ReadMemStats(&memStats)
	currentUsage := int64(memStats.Alloc)
	targetUsage := int64(float64(maxMemory) * 0.6)
	allocSize := int(targetUsage - currentUsage)
	if allocSize <= 0 {
		allocSize = int(maxMemory / 10) // At least allocate 10% of max
	}

	// Allocate memory to increase pressure
	buf := mm.AllocateBuffer(allocSize)
	assert.NotNil(t, buf)

	// Trigger an immediate check to update the interval
	mm.TriggerCheck()

	// Wait briefly for the check to complete
	time.Sleep(100 * time.Millisecond)

	// Check that interval has been adjusted
	currentInterval := mm.GetMonitoringInterval()
	pressure := mm.GetMemoryPressure()
	t.Logf("Current pressure: %.2f%%, Current interval: %v", pressure, currentInterval)
	assert.True(t, currentInterval <= 15*time.Second,
		"Monitoring interval should be reduced with medium pressure, got %v", currentInterval)

	// Clean up
	mm.DeallocateBuffer(buf)
	cancel()
	wg.Wait()
}

// TestPerformanceOptimizer tests performance optimization strategies
func TestPerformanceOptimizer(t *testing.T) {
	config := DefaultPerformanceConfig()
	pm, err := NewPerformanceManager(config)
	require.NoError(t, err)

	po := NewPerformanceOptimizer(pm)

	// Test throughput optimization
	originalBatchSize := config.MessageBatchSize
	po.OptimizeForThroughput()
	assert.GreaterOrEqual(t, config.MessageBatchSize, originalBatchSize)

	// Test latency optimization
	po.OptimizeForLatency()
	assert.LessOrEqual(t, config.MessageBatchSize, 5)
	assert.LessOrEqual(t, config.MessageBatchTimeout, 1*time.Millisecond)

	// Test memory optimization
	originalBufferPoolSize := config.BufferPoolSize
	po.OptimizeForMemory()
	assert.LessOrEqual(t, config.BufferPoolSize, originalBufferPoolSize)
	assert.True(t, config.EnableMemoryPooling)
}

// BenchmarkBufferPoolPerformance benchmarks buffer pool performance
func BenchmarkBufferPoolPerformance(b *testing.B) {
	bp := NewBufferPool(1000, 64*1024)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := bp.Get()
			// Simulate work
			buf = append(buf, []byte("test data")...)
			bp.Put(buf)
		}
	})
}

// BenchmarkMessageBatcherPerformance benchmarks message batcher performance
func BenchmarkMessageBatcherPerformance(b *testing.B) {
	mb := NewMessageBatcher(100, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go mb.Start(ctx, &wg)

	testData := []byte("test message data")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := mb.AddMessage(testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	cancel()
	wg.Wait()
}

// BenchmarkSerializationPerformance benchmarks serialization performance
func BenchmarkSerializationPerformance(b *testing.B) {
	testEvent := &testMockEvent{
		eventType: events.EventType("test"),
		data:      map[string]interface{}{"message": "test message", "data": "some data"},
	}

	b.Run("PerfJSONSerializer", func(b *testing.B) {
		js := &PerfJSONSerializer{}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := js.Serialize(testEvent)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	b.Run("PerfOptimizedJSONSerializer", func(b *testing.B) {
		ojs := &PerfOptimizedJSONSerializer{}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := ojs.Serialize(testEvent)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

// BenchmarkZeroCopyOperations benchmarks zero-copy operations
func BenchmarkZeroCopyOperations(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.Run("Bytes", func(b *testing.B) {
		zcb := NewZeroCopyBuffer(data)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = zcb.Bytes()
			}
		})
	})

	b.Run("String", func(b *testing.B) {
		zcb := NewZeroCopyBuffer(data)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = zcb.String()
			}
		})
	})
}

// BenchmarkConcurrentConnectionManagement benchmarks concurrent connection management
func BenchmarkConcurrentConnectionManagement(b *testing.B) {
	cpm := NewConnectionPoolManager(1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			slot, err := cpm.AcquireSlot(ctx)
			cancel()
			if err != nil {
				b.Fatal(err)
			}
			cpm.ReleaseSlot(slot)
		}
	})
}

// BenchmarkMemoryUsageMinimal benchmarks memory usage patterns
func BenchmarkMemoryUsageMinimal(b *testing.B) {
	mm := NewMemoryManager(80 * 1024 * 1024) // 80MB

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go mm.Start(ctx, &wg)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := mm.AllocateBuffer(1024)
			if buf != nil {
				mm.DeallocateBuffer(buf)
			}
		}
	})

	cancel()
	wg.Wait()
}

// TestPerformanceConstraintsValidation validates performance constraints
func TestPerformanceConstraintsValidation(t *testing.T) {
	// Test that we can handle 1000+ concurrent connection slots
	t.Run("1000ConcurrentConnections", func(t *testing.T) {
		cpm := NewConnectionPoolManager(1000)

		var wg sync.WaitGroup
		slots := make([]*ConnectionSlot, 1000)

		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()

				slot, err := cpm.AcquireSlot(ctx)
				assert.NoError(t, err)
				slots[index] = slot
			}(i)
		}

		wg.Wait()

		// Verify all slots were acquired
		for i, slot := range slots {
			assert.NotNil(t, slot, "Slot %d should not be nil", i)
			if slot != nil {
				cpm.ReleaseSlot(slot)
			}
		}
	})

	// Test latency constraint
	t.Run("LatencyConstraint", func(t *testing.T) {
		bp := NewBufferPool(1000, 64*1024)

		for i := 0; i < 1000; i++ {
			start := time.Now()

			buf := bp.Get()
			buf = append(buf, []byte("test data")...)
			bp.Put(buf)

			latency := time.Since(start)
			assert.LessOrEqual(t, latency, 50*time.Millisecond, "Operation latency should be under 50ms")
		}
	})

	// Test memory usage constraint
	t.Run("MemoryUsageConstraint", func(t *testing.T) {
		var beforeMem, afterMem runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&beforeMem)

		// Simulate 1000 connections with typical buffer usage
		bp := NewBufferPool(1000, 64*1024)
		buffers := make([][]byte, 1000)

		for i := 0; i < 1000; i++ {
			buffers[i] = bp.Get()
			// Simulate typical message size
			buffers[i] = append(buffers[i], make([]byte, 1024)...)
		}

		runtime.GC()
		runtime.ReadMemStats(&afterMem)

		memoryUsed := int64(afterMem.Alloc - beforeMem.Alloc)

		// Should use less than 80MB for 1000 connections
		assert.LessOrEqual(t, memoryUsed, int64(80*1024*1024),
			"Memory usage for 1000 connections should be under 80MB, got %d bytes", memoryUsed)

		// Clean up
		for i := 0; i < 1000; i++ {
			bp.Put(buffers[i])
		}
	})
}

// testMockEvent is a simple mock event for testing
type testMockEvent struct {
	eventType events.EventType
	data      map[string]interface{}
}

func (m *testMockEvent) Type() events.EventType {
	return m.eventType
}

func (m *testMockEvent) Timestamp() *int64 {
	return nil
}

func (m *testMockEvent) SetTimestamp(int64) {}

func (m *testMockEvent) Validate() error {
	return nil
}

func (m *testMockEvent) ToJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"type":"%s","data":%v}`, m.eventType, m.data)), nil
}

func (m *testMockEvent) ToProtobuf() (*generated.Event, error) {
	return nil, nil
}

func (m *testMockEvent) GetBaseEvent() *events.BaseEvent {
	return nil
}

// Helper function to measure memory usage
func measureMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}
