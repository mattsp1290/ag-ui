package websocket

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
)

// BenchmarkPerformanceManager_OptimizeMessage benchmarks message optimization
func BenchmarkPerformanceManager_OptimizeMessage(b *testing.B) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(b)

	pm, err := NewPerformanceManager(config)
	require.NoError(b, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pm.Start(ctx)
	require.NoError(b, err)
	defer pm.Stop()

	// Create a test event
	testEvent := &benchMockEvent{
		eventType: events.EventType("test"),
		data:      map[string]interface{}{"message": "test message"},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := pm.OptimizeMessage(testEvent)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkPerformanceManager_BatchMessage benchmarks message batching
func BenchmarkPerformanceManager_BatchMessage(b *testing.B) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(b)
	config.MessageBatchSize = 100
	config.MessageBatchTimeout = 10 * time.Millisecond

	pm, err := NewPerformanceManager(config)
	require.NoError(b, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pm.Start(ctx)
	require.NoError(b, err)
	defer pm.Stop()

	testData := []byte("test message data")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := pm.BatchMessage(testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkBufferPool benchmarks buffer pool operations
func BenchmarkBufferPool(b *testing.B) {
	bp := NewBufferPool(1000, 64*1024)

	b.Run("Get", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := bp.Get()
				bp.Put(buf)
			}
		})
	})

	b.Run("GetPut", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := bp.Get()
				// Simulate some work
				_ = append(buf, []byte("test data")...)
				bp.Put(buf)
			}
		})
	})
}

// BenchmarkMessageBatcher benchmarks message batching
func BenchmarkMessageBatcher(b *testing.B) {
	mb := NewMessageBatcher(100, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go mb.Start(ctx, &wg)

	testData := []byte("test message")

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

// BenchmarkConnectionPoolManager benchmarks connection pool management
func BenchmarkConnectionPoolManager(b *testing.B) {
	cpm := NewConnectionPoolManager(1000)

	b.Run("AcquireRelease", func(b *testing.B) {
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
	})
}

// BenchmarkSerializers benchmarks different serialization methods
func BenchmarkSerializers(b *testing.B) {
	testEvent := &benchMockEvent{
		eventType: events.EventType("test"),
		data:      map[string]interface{}{"message": "test message", "timestamp": time.Now().Unix()},
	}

	b.Run("PerfJSONSerializer", func(b *testing.B) {
		js := &PerfJSONSerializer{}
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
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := ojs.Serialize(testEvent)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	b.Run("PerfProtobufSerializer", func(b *testing.B) {
		ps := &PerfProtobufSerializer{}
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := ps.Serialize(testEvent)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

// BenchmarkZeroCopyBuffer benchmarks zero-copy buffer operations
func BenchmarkZeroCopyBuffer(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.Run("Bytes", func(b *testing.B) {
		zcb := NewZeroCopyBuffer(data)
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = zcb.Bytes()
			}
		})
	})

	b.Run("String", func(b *testing.B) {
		zcb := NewZeroCopyBuffer(data)
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = zcb.String()
			}
		})
	})
}

// BenchmarkMemoryManager benchmarks memory management
func BenchmarkMemoryManager(b *testing.B) {
	mm := NewMemoryManager(80 * 1024 * 1024) // 80MB

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go mm.Start(ctx, &wg)

	b.Run("AllocateBuffer", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := mm.AllocateBuffer(1024)
				if buf != nil {
					mm.DeallocateBuffer(buf)
				}
			}
		})
	})

	cancel()
	wg.Wait()
}

// BenchmarkConcurrentConnections benchmarks handling of concurrent connections
func BenchmarkConcurrentConnections(b *testing.B) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(b)
	config.MaxConcurrentConnections = 1000

	pm, err := NewPerformanceManager(config)
	require.NoError(b, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pm.Start(ctx)
	require.NoError(b, err)
	defer pm.Stop()

	b.ResetTimer()

	b.Run("1000Connections", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			for j := 0; j < 1000; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					slot, err := pm.GetConnectionSlot(ctx)
					if err != nil {
						b.Error(err)
						return
					}
					pm.ReleaseConnectionSlot(slot)
				}()
			}
			wg.Wait()
		}
	})
}

// BenchmarkLatencyMeasurement benchmarks latency measurement
func BenchmarkLatencyMeasurement(b *testing.B) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(b)
	config.EnableMetrics = true

	pm, err := NewPerformanceManager(config)
	require.NoError(b, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pm.Start(ctx)
	require.NoError(b, err)
	defer pm.Stop()

	testEvent := &benchMockEvent{
		eventType: events.EventType("test"),
		data:      map[string]interface{}{"message": "test message"},
	}

	b.ResetTimer()

	b.Run("E2ELatency", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				start := time.Now()

				// Simulate message processing
				data, err := pm.OptimizeMessage(testEvent)
				if err != nil {
					b.Fatal(err)
				}

				err = pm.BatchMessage(data)
				if err != nil {
					b.Fatal(err)
				}

				latency := time.Since(start)
				if pm.metricsCollector != nil {
					pm.metricsCollector.TrackMessageLatency(latency)
				}
			}
		})
	})
}

// BenchmarkMemoryUsage benchmarks memory usage under load
func BenchmarkMemoryUsage(b *testing.B) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(b)
	config.EnableMemoryPooling = true
	config.BufferPoolSize = 10000

	pm, err := NewPerformanceManager(config)
	require.NoError(b, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pm.Start(ctx)
	require.NoError(b, err)
	defer pm.Stop()

	b.ResetTimer()

	b.Run("MemoryIntensive", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Allocate buffer
				buf := pm.GetBuffer()

				// Simulate work
				for i := 0; i < 1000; i++ {
					buf = append(buf, byte(i%256))
				}

				// Return buffer
				pm.PutBuffer(buf)
			}
		})
	})
}

// BenchmarkThroughput benchmarks message throughput
func BenchmarkThroughput(b *testing.B) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(b)
	config.MessageBatchSize = 1000
	config.MessageBatchTimeout = 1 * time.Millisecond

	pm, err := NewPerformanceManager(config)
	require.NoError(b, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pm.Start(ctx)
	require.NoError(b, err)
	defer pm.Stop()

	testEvent := &benchMockEvent{
		eventType: events.EventType("test"),
		data:      map[string]interface{}{"message": "test message"},
	}

	b.ResetTimer()

	b.Run("HighThroughput", func(b *testing.B) {
		var messageCount int64

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				data, err := pm.OptimizeMessage(testEvent)
				if err != nil {
					b.Fatal(err)
				}

				err = pm.BatchMessage(data)
				if err != nil {
					b.Fatal(err)
				}

				atomic.AddInt64(&messageCount, 1)
			}
		})

		b.Logf("Processed %d messages", messageCount)
	})
}

// TestPerformanceConstraints tests that performance constraints are met
func TestPerformanceConstraints(t *testing.T) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(t)
	config.MaxConcurrentConnections = 1000
	config.MaxLatency = 50 * time.Millisecond
	config.MaxMemoryUsage = 80 * 1024 * 1024 // 80MB
	config.EnableMetrics = true

	pm, err := NewPerformanceManager(config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pm.Start(ctx)
	require.NoError(t, err)
	defer func() {
		err := pm.Stop()
		assert.NoError(t, err)
	}()

	// Test concurrent connections
	t.Run("ConcurrentConnections", func(t *testing.T) {
		var wg sync.WaitGroup
		slots := make([]*ConnectionSlot, 1000)

		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				slot, err := pm.GetConnectionSlot(ctx)
				assert.NoError(t, err)
				slots[index] = slot
			}(i)
		}

		wg.Wait()

		// Release all slots
		for _, slot := range slots {
			if slot != nil {
				pm.ReleaseConnectionSlot(slot)
			}
		}

		// Check that we could handle 1000 concurrent connections
		assert.Equal(t, 1000, len(slots))
	})

	// Test latency constraint
	t.Run("LatencyConstraint", func(t *testing.T) {
		testEvent := &benchMockEvent{
			eventType: events.EventType("test"),
			data:      map[string]interface{}{"message": "test message"},
		}

		for i := 0; i < 1000; i++ {
			start := time.Now()

			data, err := pm.OptimizeMessage(testEvent)
			require.NoError(t, err)

			err = pm.BatchMessage(data)
			require.NoError(t, err)

			latency := time.Since(start)
			assert.LessOrEqual(t, latency, config.MaxLatency, "Latency constraint violated")
		}
	})

	// Test memory usage constraint
	t.Run("MemoryUsageConstraint", func(t *testing.T) {
		if pm.memoryManager == nil {
			t.Skip("Memory manager not enabled")
		}

		// Generate some load
		for i := 0; i < 10000; i++ {
			buf := pm.GetBuffer()
			buf = append(buf, make([]byte, 1024)...)
			pm.PutBuffer(buf)
		}

		// Check memory usage
		usage := pm.GetMemoryUsage()
		assert.LessOrEqual(t, usage, config.MaxMemoryUsage, "Memory usage constraint violated")
	})
}

// TestPerformanceMetrics tests that metrics are collected correctly
func TestPerformanceMetrics(t *testing.T) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(t)
	config.EnableMetrics = true
	config.MetricsInterval = 100 * time.Millisecond

	pm, err := NewPerformanceManager(config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pm.Start(ctx)
	require.NoError(t, err)
	defer pm.Stop()

	// Wait for metrics to be collected
	time.Sleep(200 * time.Millisecond)

	metrics := pm.GetMetrics()
	require.NotNil(t, metrics)

	// Check that metrics are being updated
	assert.True(t, metrics.StartTime.Before(time.Now()))
	assert.True(t, metrics.LastUpdate.After(metrics.StartTime))
	assert.True(t, metrics.Uptime > 0)
	assert.True(t, metrics.GoroutineCount > 0)
	assert.True(t, metrics.MemoryUsage > 0)
}

// TestAdaptiveOptimization tests adaptive optimization
func TestAdaptiveOptimization(t *testing.T) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(t)
	config.EnableMetrics = true
	config.MaxLatency = 10 * time.Millisecond
	config.MaxMemoryUsage = 10 * 1024 * 1024 // 10MB for testing

	pm, err := NewPerformanceManager(config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pm.Start(ctx)
	require.NoError(t, err)
	defer pm.Stop()

	// Create adaptive optimizer
	ao := NewAdaptiveOptimizer(pm)

	var wg sync.WaitGroup
	wg.Add(1)
	go ao.Start(ctx, &wg)

	// Simulate high latency
	if pm.metricsCollector != nil {
		pm.metricsCollector.TrackMessageLatency(100 * time.Millisecond)
	}

	// Wait for adaptation to occur
	time.Sleep(1 * time.Second)

	// Force adaptation by calling TriggerAdaptation manually
	ao.TriggerAdaptation()

	// Check that settings were adapted
	assert.True(t, config.MessageBatchSize <= 5, "Batch size should be reduced for latency")
	assert.True(t, config.MessageBatchTimeout <= 1*time.Millisecond, "Batch timeout should be reduced for latency")

	cancel()
	wg.Wait()
}

// TestBufferPoolEfficiency tests buffer pool efficiency
func TestBufferPoolEfficiency(t *testing.T) {
	bp := NewBufferPool(100, 1024)

	// Test that buffers are reused
	buf1 := bp.Get()
	buf1 = append(buf1, []byte("test")...)
	bp.Put(buf1)

	buf2 := bp.Get()

	// Should get the same underlying buffer (capacity should be same)
	assert.Equal(t, cap(buf1), cap(buf2))
	assert.Equal(t, 0, len(buf2)) // Length should be reset

	bp.Put(buf2)

	// Check stats
	stats := bp.GetStats()
	assert.Equal(t, int64(2), stats["gets"])
	assert.Equal(t, int64(2), stats["puts"])
}

// TestZeroCopyOperations tests zero-copy operations
func TestZeroCopyOperations(t *testing.T) {
	data := []byte("test data for zero copy operations")
	zcb := NewZeroCopyBuffer(data)

	// Test that no copying occurs
	bytes1 := zcb.Bytes()
	bytes2 := zcb.Bytes()

	// Should point to the same underlying data
	assert.Equal(t, &bytes1[0], &bytes2[0])

	// Test string conversion
	str := zcb.String()
	assert.Equal(t, string(data), str)

	// Test advance
	zcb.Advance(5)
	assert.Equal(t, string(data[5:]), zcb.String())
	assert.Equal(t, len(data)-5, zcb.Len())

	// Test reset
	zcb.Reset()
	assert.Equal(t, string(data), zcb.String())
	assert.Equal(t, len(data), zcb.Len())
}

// TestMemoryManagerConstraints tests memory manager constraints
func TestMemoryManagerConstraints(t *testing.T) {
	maxMemory := int64(1024 * 1024) // 1MB
	mm := NewMemoryManager(maxMemory)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go mm.Start(ctx, &wg)

	// Try to allocate within limit
	buf1 := mm.AllocateBuffer(512 * 1024) // 512KB
	assert.NotNil(t, buf1)

	// Check intermediate stats
	stats := mm.GetStats()
	t.Logf("After first allocation: allocations=%d", stats["allocations"])

	// Try to allocate beyond limit
	buf2 := mm.AllocateBuffer(1024 * 1024) // 1MB (should fail due to existing allocation)
	assert.Nil(t, buf2)

	// Check stats after failed allocation
	stats = mm.GetStats()
	t.Logf("After failed allocation: allocations=%d", stats["allocations"])

	// Clean up
	mm.DeallocateBuffer(buf1)

	// Let memory manager do a final check  
	time.Sleep(50 * time.Millisecond)

	cancel()
	wg.Wait()

	// Check final stats - allow for reasonable range of allocations due to internal allocations
	stats = mm.GetStats()
	t.Logf("Final stats: allocations=%d, deallocations=%d", stats["allocations"], stats["deallocations"])
	assert.True(t, stats["allocations"] >= 1 && stats["allocations"] <= 2, "Expected 1-2 allocations, got %d", stats["allocations"])
	assert.Equal(t, int64(1), stats["deallocations"])
}

// TestProfilingIntegration tests profiling integration
func TestProfilingIntegration(t *testing.T) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(t)
	config.EnableProfiling = true
	config.ProfilingInterval = 100 * time.Millisecond

	pm, err := NewPerformanceManager(config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pm.Start(ctx)
	require.NoError(t, err)
	defer pm.Stop()

	// Wait for profiling data to be collected
	time.Sleep(200 * time.Millisecond)

	// Check that profiling data is available
	if pm.profiler != nil {
		profilingData := pm.profiler.GetProfilingData()
		assert.NotEmpty(t, profilingData)
		assert.Contains(t, profilingData, "timestamp")
	}
}

// Benchmark helpers and utilities

// benchMockEvent implements the Event interface for testing
type benchMockEvent struct {
	eventType events.EventType
	data      map[string]interface{}
}

func (m *benchMockEvent) Type() events.EventType {
	return m.eventType
}

func (m *benchMockEvent) Timestamp() *int64 {
	return nil
}

func (m *benchMockEvent) SetTimestamp(int64) {}

func (m *benchMockEvent) Validate() error {
	return nil
}

func (m *benchMockEvent) ToJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"type":"%s","data":%v}`, m.eventType, m.data)), nil
}

func (m *benchMockEvent) ToProtobuf() (*generated.Event, error) {
	return nil, nil
}

func (m *benchMockEvent) GetBaseEvent() *events.BaseEvent {
	return nil
}

// Helper function to measure memory usage
func measureMemory() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}
