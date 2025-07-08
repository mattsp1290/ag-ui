package state

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"
)

// TestPerformanceOptimizerInterface tests that PerformanceOptimizerImpl implements PerformanceOptimizer
func TestPerformanceOptimizerInterface(t *testing.T) {
	opts := DefaultPerformanceOptions()
	opts.EnablePooling = true
	opts.EnableBatching = true
	opts.EnableCompression = true
	opts.EnableLazyLoading = true
	opts.EnableSharding = true

	// Test factory method
	optimizer := NewPerformanceOptimizer(opts)
	if optimizer == nil {
		t.Fatal("NewPerformanceOptimizer returned nil")
	}

	// Test that we can use it as the interface
	testPerformanceOptimizerMethods(t, optimizer)

	// Cleanup
	optimizer.Stop()
}

// testPerformanceOptimizerMethods tests all interface methods
func testPerformanceOptimizerMethods(t *testing.T, optimizer PerformanceOptimizer) {
	ctx := context.Background()

	// Test object pool operations
	t.Run("ObjectPools", func(t *testing.T) {
		// Test patch operation pooling
		patchOp := optimizer.GetPatchOperation()
		if patchOp == nil {
			t.Error("GetPatchOperation returned nil")
		}
		optimizer.PutPatchOperation(patchOp)

		// Test state change pooling
		stateChange := optimizer.GetStateChange()
		if stateChange == nil {
			t.Error("GetStateChange returned nil")
		}
		optimizer.PutStateChange(stateChange)

		// Test state event pooling
		stateEvent := optimizer.GetStateEvent()
		if stateEvent == nil {
			t.Error("GetStateEvent returned nil")
		}
		optimizer.PutStateEvent(stateEvent)

		// Test buffer pooling
		buffer := optimizer.GetBuffer()
		if buffer == nil {
			t.Error("GetBuffer returned nil")
		}
		optimizer.PutBuffer(buffer)
	})

	// Test batch operations
	t.Run("BatchOperations", func(t *testing.T) {
		operationExecuted := false
		err := optimizer.BatchOperation(ctx, func() error {
			operationExecuted = true
			return nil
		})
		if err != nil {
			t.Errorf("BatchOperation failed: %v", err)
		}

		// Give some time for async processing
		time.Sleep(100 * time.Millisecond)

		if !operationExecuted {
			t.Error("Batch operation was not executed")
		}
	})

	// Test state management operations
	t.Run("StateManagement", func(t *testing.T) {
		// Test sharded operations
		key := "test-key"
		value := "test-value"

		optimizer.ShardedSet(key, value)
		retrievedValue, found := optimizer.ShardedGet(key)
		if !found {
			t.Error("ShardedGet did not find the set value")
		}
		if retrievedValue != value {
			t.Errorf("Expected %v, got %v", value, retrievedValue)
		}

		// Test lazy loading
		lazyKey := "lazy-key"
		loadedValue := "loaded-value"
		loaderCalled := false

		result, err := optimizer.LazyLoadState(lazyKey, func() (interface{}, error) {
			loaderCalled = true
			return loadedValue, nil
		})

		if err != nil {
			t.Errorf("LazyLoadState failed: %v", err)
		}

		if !loaderCalled {
			t.Error("Loader function was not called")
		}

		if result != loadedValue {
			t.Errorf("Expected %v, got %v", loadedValue, result)
		}

		// Test that subsequent calls use cache
		loaderCalled = false
		result2, err := optimizer.LazyLoadState(lazyKey, func() (interface{}, error) {
			loaderCalled = true
			return "different-value", nil
		})

		if err != nil {
			t.Errorf("Second LazyLoadState failed: %v", err)
		}

		if loaderCalled {
			t.Error("Loader function was called again (cache not working)")
		}

		if result2 != loadedValue {
			t.Errorf("Cached value mismatch: expected %v, got %v", loadedValue, result2)
		}
	})

	// Test data compression operations
	t.Run("DataCompression", func(t *testing.T) {
		testData := []byte("This is test data for compression testing. It should be long enough to potentially benefit from compression.")

		compressed, err := optimizer.CompressData(testData)
		if err != nil {
			t.Errorf("CompressData failed: %v", err)
		}

		if compressed == nil {
			t.Error("CompressData returned nil")
		}

		decompressed, err := optimizer.DecompressData(compressed)
		if err != nil {
			t.Errorf("DecompressData failed: %v", err)
		}

		if !bytes.Equal(testData, decompressed) {
			t.Error("Decompressed data does not match original")
		}
	})

	// Test performance operations
	t.Run("PerformanceOperations", func(t *testing.T) {
		// Test OptimizeForLargeState
		largeStateSize := int64(200 * 1024 * 1024) // 200MB
		optimizer.OptimizeForLargeState(largeStateSize)

		// Test ProcessLargeStateUpdate
		updateExecuted := false
		err := optimizer.ProcessLargeStateUpdate(ctx, func() error {
			updateExecuted = true
			return nil
		})

		if err != nil {
			t.Errorf("ProcessLargeStateUpdate failed: %v", err)
		}

		if !updateExecuted {
			t.Error("Large state update was not executed")
		}
	})

	// Test metrics and monitoring
	t.Run("MetricsAndMonitoring", func(t *testing.T) {
		metrics := optimizer.GetMetrics()
		if metrics.PoolHits < 0 {
			t.Error("Invalid pool hits metric")
		}

		enhancedMetrics := optimizer.GetEnhancedMetrics()
		if enhancedMetrics.PoolHits < 0 {
			t.Error("Invalid enhanced pool hits metric")
		}
	})
}

// TestPerformanceOptimizerTypeAssertion tests that the factory returns the expected concrete type
func TestPerformanceOptimizerTypeAssertion(t *testing.T) {
	opts := DefaultPerformanceOptions()

	optimizer := NewPerformanceOptimizer(opts)

	// Test that we can cast back to the concrete type if needed
	concrete, ok := optimizer.(*PerformanceOptimizerImpl)
	if !ok {
		t.Error("PerformanceOptimizer is not a *PerformanceOptimizerImpl")
	}

	if concrete == nil {
		t.Error("Concrete type is nil")
	}

	// Test that concrete type has all the expected fields
	if concrete.patchPool == nil {
		t.Error("Concrete type patchPool is nil")
	}

	if concrete.stateChangePool == nil {
		t.Error("Concrete type stateChangePool is nil")
	}

	optimizer.Stop()
}

// MockPerformanceOptimizerInterface is a mock implementation for testing
type MockPerformanceOptimizerInterface struct {
	getPatchOperationCalls       int
	putPatchOperationCalls       int
	getStateChangeCalls          int
	putStateChangeCalls          int
	getStateEventCalls           int
	putStateEventCalls           int
	getBufferCalls               int
	putBufferCalls               int
	batchOperationCalls          int
	shardedGetCalls              int
	shardedSetCalls              int
	lazyLoadStateCalls           int
	compressDataCalls            int
	decompressDataCalls          int
	optimizeForLargeStateCalls   int
	processLargeStateUpdateCalls int
	getMetricsCalls              int
	getEnhancedMetricsCalls      int
	stopCalls                    int

	shardedData map[string]interface{}
	lazyCache   map[string]interface{}
}

// NewMockPerformanceOptimizerInterface creates a new mock performance optimizer
func NewMockPerformanceOptimizerInterface() *MockPerformanceOptimizerInterface {
	return &MockPerformanceOptimizerInterface{
		shardedData: make(map[string]interface{}),
		lazyCache:   make(map[string]interface{}),
	}
}

func (m *MockPerformanceOptimizerInterface) GetPatchOperation() *JSONPatchOperation {
	m.getPatchOperationCalls++
	return &JSONPatchOperation{}
}

func (m *MockPerformanceOptimizerInterface) PutPatchOperation(op *JSONPatchOperation) {
	m.putPatchOperationCalls++
}

func (m *MockPerformanceOptimizerInterface) GetStateChange() *StateChange {
	m.getStateChangeCalls++
	return &StateChange{}
}

func (m *MockPerformanceOptimizerInterface) PutStateChange(sc *StateChange) {
	m.putStateChangeCalls++
}

func (m *MockPerformanceOptimizerInterface) GetStateEvent() *StateEvent {
	m.getStateEventCalls++
	return &StateEvent{}
}

func (m *MockPerformanceOptimizerInterface) PutStateEvent(se *StateEvent) {
	m.putStateEventCalls++
}

func (m *MockPerformanceOptimizerInterface) GetBuffer() *bytes.Buffer {
	m.getBufferCalls++
	return bytes.NewBuffer(nil)
}

func (m *MockPerformanceOptimizerInterface) PutBuffer(buf *bytes.Buffer) {
	m.putBufferCalls++
}

func (m *MockPerformanceOptimizerInterface) BatchOperation(ctx context.Context, operation func() error) error {
	m.batchOperationCalls++
	return operation()
}

func (m *MockPerformanceOptimizerInterface) ShardedGet(key string) (interface{}, bool) {
	m.shardedGetCalls++
	value, exists := m.shardedData[key]
	return value, exists
}

func (m *MockPerformanceOptimizerInterface) ShardedSet(key string, value interface{}) {
	m.shardedSetCalls++
	m.shardedData[key] = value
}

func (m *MockPerformanceOptimizerInterface) LazyLoadState(key string, loader func() (interface{}, error)) (interface{}, error) {
	m.lazyLoadStateCalls++

	// Check cache first
	if value, exists := m.lazyCache[key]; exists {
		return value, nil
	}

	// Load and cache
	value, err := loader()
	if err != nil {
		return nil, err
	}

	m.lazyCache[key] = value
	return value, nil
}

func (m *MockPerformanceOptimizerInterface) CompressData(data []byte) ([]byte, error) {
	m.compressDataCalls++
	// Mock compression by just returning the data
	return data, nil
}

func (m *MockPerformanceOptimizerInterface) DecompressData(data []byte) ([]byte, error) {
	m.decompressDataCalls++
	// Mock decompression by just returning the data
	return data, nil
}

func (m *MockPerformanceOptimizerInterface) OptimizeForLargeState(stateSize int64) {
	m.optimizeForLargeStateCalls++
}

func (m *MockPerformanceOptimizerInterface) ProcessLargeStateUpdate(ctx context.Context, update func() error) error {
	m.processLargeStateUpdateCalls++
	return update()
}

func (m *MockPerformanceOptimizerInterface) GetMetrics() PerformanceMetrics {
	m.getMetricsCalls++
	return PerformanceMetrics{
		PoolHits:    int64(m.getPatchOperationCalls + m.getStateChangeCalls + m.getStateEventCalls + m.getBufferCalls),
		PoolMisses:  0,
		CacheHits:   int64(len(m.lazyCache)),
		CacheMisses: 0,
	}
}

func (m *MockPerformanceOptimizerInterface) GetEnhancedMetrics() PerformanceMetrics {
	m.getEnhancedMetricsCalls++
	return m.GetMetrics()
}

func (m *MockPerformanceOptimizerInterface) Stop() {
	m.stopCalls++
}

// TestMockPerformanceOptimizer tests that the mock implements the interface correctly
func TestMockPerformanceOptimizer(t *testing.T) {
	mock := NewMockPerformanceOptimizerInterface()

	// Test that mock implements the interface
	var optimizer PerformanceOptimizer = mock

	// Test interface methods
	testPerformanceOptimizerMethods(t, optimizer)

	// Test mock-specific behavior
	if mock.getPatchOperationCalls == 0 {
		t.Error("GetPatchOperation was not called")
	}

	if mock.putPatchOperationCalls == 0 {
		t.Error("PutPatchOperation was not called")
	}

	if mock.batchOperationCalls == 0 {
		t.Error("BatchOperation was not called")
	}

	if mock.shardedSetCalls == 0 {
		t.Error("ShardedSet was not called")
	}

	if mock.shardedGetCalls == 0 {
		t.Error("ShardedGet was not called")
	}

	if mock.lazyLoadStateCalls == 0 {
		t.Error("LazyLoadState was not called")
	}

	if mock.compressDataCalls == 0 {
		t.Error("CompressData was not called")
	}

	if mock.decompressDataCalls == 0 {
		t.Error("DecompressData was not called")
	}

	if mock.stopCalls == 0 {
		t.Error("Stop was not called")
	}
}

// TestInterfaceCompatibility tests interface compatibility
func TestInterfaceCompatibility(t *testing.T) {
	// Test that both real and mock implementations work with the same interface
	implementations := []PerformanceOptimizer{
		NewMockPerformanceOptimizerInterface(),
	}

	// Add real implementation
	opts := DefaultPerformanceOptions()
	real := NewPerformanceOptimizer(opts)
	implementations = append(implementations, real)

	for i, impl := range implementations {
		t.Run(fmt.Sprintf("Implementation%d", i), func(t *testing.T) {
			ctx := context.Background()

			// Test basic interface usage
			patchOp := impl.GetPatchOperation()
			if patchOp == nil {
				t.Error("GetPatchOperation returned nil")
			}
			impl.PutPatchOperation(patchOp)

			err := impl.BatchOperation(ctx, func() error {
				return nil
			})
			if err != nil {
				t.Errorf("BatchOperation failed: %v", err)
			}

			impl.ShardedSet("test", "value")
			value, found := impl.ShardedGet("test")
			if !found {
				t.Error("ShardedGet did not find value")
			}
			if value != "value" {
				t.Errorf("Expected 'value', got %v", value)
			}

			// Test metrics
			metrics := impl.GetMetrics()
			if metrics.PoolHits < 0 {
				t.Error("Invalid metrics")
			}

			// Cleanup
			impl.Stop()
		})
	}
}

// BenchmarkPerformanceOptimizerPooling benchmarks object pooling performance
func BenchmarkPerformanceOptimizerPooling(b *testing.B) {
	opts := DefaultPerformanceOptions()
	opts.EnablePooling = true
	optimizer := NewPerformanceOptimizer(opts)
	defer optimizer.Stop()

	b.Run("PatchOperationPooling", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			op := optimizer.GetPatchOperation()
			optimizer.PutPatchOperation(op)
		}
	})

	b.Run("StateChangePooling", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sc := optimizer.GetStateChange()
			optimizer.PutStateChange(sc)
		}
	})

	b.Run("BufferPooling", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := optimizer.GetBuffer()
			optimizer.PutBuffer(buf)
		}
	})
}

// BenchmarkPerformanceOptimizerBatching benchmarks batch operation performance
func BenchmarkPerformanceOptimizerBatching(b *testing.B) {
	opts := DefaultPerformanceOptions()
	opts.EnableBatching = true
	optimizer := NewPerformanceOptimizer(opts)
	defer optimizer.Stop()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := optimizer.BatchOperation(ctx, func() error {
			return nil
		})
		if err != nil {
			b.Errorf("BatchOperation failed: %v", err)
		}
	}
}
