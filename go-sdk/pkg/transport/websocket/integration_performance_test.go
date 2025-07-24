package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
)

// TestTransportPerformanceIntegration tests integration between transport and performance manager
func TestTransportPerformanceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance integration test in short mode")
	}
	// Create transport config with performance optimizations
	config := DefaultTransportConfig()
	config.Logger = zaptest.NewLogger(t)
	config.URLs = []string{"ws://localhost:8080/ws"} // Mock URL

	// Configure performance settings
	config.PerformanceConfig.MaxConcurrentConnections = 100
	config.PerformanceConfig.MessageBatchSize = 10
	config.PerformanceConfig.MessageBatchTimeout = 5 * time.Millisecond
	config.PerformanceConfig.EnableMetrics = true
	config.PerformanceConfig.EnableMemoryPooling = true
	config.PerformanceConfig.MaxLatency = 50 * time.Millisecond
	config.PerformanceConfig.MaxMemoryUsage = 10 * 1024 * 1024 // 10MB

	// Create transport
	transport, err := NewTransport(config)
	require.NoError(t, err)
	require.NotNil(t, transport)
	require.NotNil(t, transport.performanceManager)

	// Test configuration propagation
	assert.Equal(t, 100, transport.performanceManager.config.MaxConcurrentConnections)
	assert.Equal(t, 10, transport.performanceManager.config.MessageBatchSize)
	assert.Equal(t, 5*time.Millisecond, transport.performanceManager.config.MessageBatchTimeout)
	assert.True(t, transport.performanceManager.config.EnableMetrics)
	assert.True(t, transport.performanceManager.config.EnableMemoryPooling)
}

// TestTransportPerformanceMetrics tests performance metrics collection
func TestTransportPerformanceMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance metrics test in short mode")
	}
	config := DefaultTransportConfig()
	config.Logger = zaptest.NewLogger(t)
	config.URLs = []string{"ws://localhost:8080/ws"}
	config.PerformanceConfig.EnableMetrics = true
	config.PerformanceConfig.MetricsInterval = 100 * time.Millisecond

	transport, err := NewTransport(config)
	require.NoError(t, err)

	// Test metrics access
	metrics := transport.GetPerformanceMetrics()
	require.NotNil(t, metrics)

	// Test memory usage access
	memoryUsage := transport.GetMemoryUsage()
	assert.GreaterOrEqual(t, memoryUsage, int64(0))
}

// TestTransportOptimizationMethods tests optimization methods
func TestTransportOptimizationMethods(t *testing.T) {
	config := DefaultTransportConfig()
	config.Logger = zaptest.NewLogger(t)
	config.URLs = []string{"ws://localhost:8080/ws"}

	transport, err := NewTransport(config)
	require.NoError(t, err)

	originalBatchSize := transport.performanceManager.config.MessageBatchSize

	// Test throughput optimization
	transport.OptimizeForThroughput()
	assert.GreaterOrEqual(t, transport.performanceManager.config.MessageBatchSize, originalBatchSize)

	// Test latency optimization
	transport.OptimizeForLatency()
	assert.LessOrEqual(t, transport.performanceManager.config.MessageBatchSize, 5)
	assert.LessOrEqual(t, transport.performanceManager.config.MessageBatchTimeout, 1*time.Millisecond)

	// Test memory optimization
	originalBufferPoolSize := transport.performanceManager.config.BufferPoolSize
	transport.OptimizeForMemory()
	assert.LessOrEqual(t, transport.performanceManager.config.BufferPoolSize, originalBufferPoolSize)
	assert.True(t, transport.performanceManager.config.EnableMemoryPooling)
}

// TestPerformanceManagerComponents tests individual performance manager components
func TestPerformanceManagerComponents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance manager components test in short mode")
	}
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(t)
	config.EnableMetrics = true
	config.EnableMemoryPooling = true

	pm, err := NewPerformanceManager(config)
	require.NoError(t, err)
	require.NotNil(t, pm)

	// Test buffer pool
	assert.NotNil(t, pm.bufferPool)
	buf := pm.GetBuffer()
	assert.NotNil(t, buf)
	assert.Equal(t, 0, len(buf))
	pm.PutBuffer(buf)

	// Test message batcher
	assert.NotNil(t, pm.messageBatcher)
	testData := []byte("test message")
	err = pm.BatchMessage(testData)
	assert.NoError(t, err)

	// Test connection pool manager
	assert.NotNil(t, pm.connectionPoolManager)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	slot, err := pm.GetConnectionSlot(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, slot)
	pm.ReleaseConnectionSlot(slot)

	// Test serializer factory
	assert.NotNil(t, pm.serializerFactory)
	serializer := pm.serializerFactory.GetSerializer()
	assert.NotNil(t, serializer)

	// Test metrics collector
	if pm.metricsCollector != nil {
		metrics := pm.GetMetrics()
		assert.NotNil(t, metrics)
	}

	// Test memory manager
	if pm.memoryManager != nil {
		usage := pm.GetMemoryUsage()
		assert.GreaterOrEqual(t, usage, int64(0))
	}
}

// TestMessageOptimization tests message optimization functionality
func TestMessageOptimization(t *testing.T) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(t)
	config.MessageSerializerType = OptimizedJSONSerializer

	pm, err := NewPerformanceManager(config)
	require.NoError(t, err)

	testEvent := &integrationMockEvent{
		eventType: events.EventType("test"),
		data:      map[string]interface{}{"message": "test data", "value": 42},
	}

	// Test message optimization
	data, err := pm.OptimizeMessage(testEvent)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify the serialized data contains expected content
	dataStr := string(data)
	assert.Contains(t, dataStr, "test")
	assert.Contains(t, dataStr, "test data")
}

// BenchmarkTransportWithPerformanceManager benchmarks transport with performance optimizations
func BenchmarkTransportWithPerformanceManager(b *testing.B) {
	config := DefaultTransportConfig()
	config.Logger = zaptest.NewLogger(b)
	config.URLs = []string{"ws://localhost:8080/ws"}
	config.PerformanceConfig.EnableMetrics = true
	config.PerformanceConfig.MessageBatchSize = 50
	config.PerformanceConfig.MessageBatchTimeout = 1 * time.Millisecond

	transport, err := NewTransport(config)
	require.NoError(b, err)

	testEvent := &integrationMockEvent{
		eventType: events.EventType("benchmark"),
		data:      map[string]interface{}{"message": "benchmark test"},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Test message optimization only (not full send since we don't have a real server)
			_, err := transport.performanceManager.OptimizeMessage(testEvent)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkPerformanceManagerOverhead benchmarks performance manager overhead
func BenchmarkPerformanceManagerOverhead(b *testing.B) {
	config := DefaultPerformanceConfig()
	config.Logger = zaptest.NewLogger(b)

	pm, err := NewPerformanceManager(config)
	require.NoError(b, err)

	testEvent := &integrationMockEvent{
		eventType: events.EventType("overhead"),
		data:      map[string]interface{}{"test": "data"},
	}

	b.Run("WithOptimization", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, err := pm.OptimizeMessage(testEvent)
			if err != nil {
				b.Fatal(err)
			}
			_ = data
		}
	})

	b.Run("WithoutOptimization", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, err := testEvent.ToJSON()
			if err != nil {
				b.Fatal(err)
			}
			_ = data
		}
	})
}


// integrationMockEvent is a mock event for integration testing
type integrationMockEvent struct {
	eventType events.EventType
	data      map[string]interface{}
}

func (m *integrationMockEvent) Type() events.EventType {
	return m.eventType
}

func (m *integrationMockEvent) Timestamp() *int64 {
	return nil
}

func (m *integrationMockEvent) SetTimestamp(int64) {}

func (m *integrationMockEvent) Validate() error {
	return nil
}

func (m *integrationMockEvent) ToJSON() ([]byte, error) {
	dataJSON, err := json.Marshal(m.data)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`{"type":"%s","data":%s}`, m.eventType, dataJSON)), nil
}

func (m *integrationMockEvent) ToProtobuf() (*generated.Event, error) {
	return nil, nil
}

func (m *integrationMockEvent) GetBaseEvent() *events.BaseEvent {
	return nil
}

func (m *integrationMockEvent) ThreadID() string {
	return ""
}

func (m *integrationMockEvent) RunID() string {
	return ""
}
