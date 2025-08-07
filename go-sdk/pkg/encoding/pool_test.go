package encoding

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

func TestBufferPool(t *testing.T) {
	pool := NewBufferPool(1024)

	// Test get and put
	buf := pool.Get()
	if buf == nil {
		t.Fatal("Expected non-nil buffer")
	}

	buf.WriteString("test")
	if buf.String() != "test" {
		t.Errorf("Expected 'test', got %s", buf.String())
	}

	pool.Put(buf)

	// Test metrics
	metrics := pool.Metrics()
	if metrics.Gets != 1 {
		t.Errorf("Expected Gets=1, got %d", metrics.Gets)
	}
	if metrics.Puts != 1 {
		t.Errorf("Expected Puts=1, got %d", metrics.Puts)
	}
}

func TestSlicePool(t *testing.T) {
	pool := NewSlicePool(1024, 4096)

	// Test get and put
	slice := pool.Get()
	if slice == nil {
		t.Fatal("Expected non-nil slice")
	}

	slice = append(slice, []byte("test")...)
	if string(slice) != "test" {
		t.Errorf("Expected 'test', got %s", slice)
	}

	pool.Put(slice)

	// Test metrics
	metrics := pool.Metrics()
	if metrics.Gets != 1 {
		t.Errorf("Expected Gets=1, got %d", metrics.Gets)
	}
	if metrics.Puts != 1 {
		t.Errorf("Expected Puts=1, got %d", metrics.Puts)
	}
}

func TestErrorPool(t *testing.T) {
	pool := NewErrorPool()

	// Test encoding error
	encErr := pool.GetEncodingError()
	if encErr == nil {
		t.Fatal("Expected non-nil encoding error")
	}

	encErr.Format = "test"
	encErr.Message = "test error"

	pool.PutEncodingError(encErr)

	// Test decoding error
	decErr := pool.GetDecodingError()
	if decErr == nil {
		t.Fatal("Expected non-nil decoding error")
	}

	decErr.Format = "test"
	decErr.Message = "test error"

	pool.PutDecodingError(decErr)

	// Test metrics
	metrics := pool.Metrics()
	if metrics.Gets != 2 {
		t.Errorf("Expected Gets=2, got %d", metrics.Gets)
	}
	if metrics.Puts != 2 {
		t.Errorf("Expected Puts=2, got %d", metrics.Puts)
	}
}

func TestCodecPool(t *testing.T) {
	pool := NewCodecPool()

	// Set up mock constructors for testing
	pool.SetJSONEncoderConstructor(func() interface{} {
		return &mockPoolEncoder{}
	})
	pool.SetJSONDecoderConstructor(func() interface{} {
		return &mockPoolDecoder{}
	})

	// Test JSON encoder
	jsonEncoder := pool.GetJSONEncoder(&EncodingOptions{})
	if jsonEncoder == nil {
		t.Fatal("Expected non-nil JSON encoder")
	}
	pool.PutJSONEncoder(jsonEncoder)

	// Test JSON decoder
	jsonDecoder := pool.GetJSONDecoder(&DecodingOptions{})
	if jsonDecoder == nil {
		t.Fatal("Expected non-nil JSON decoder")
	}
	pool.PutJSONDecoder(jsonDecoder)

	// Test metrics
	metrics := pool.Metrics()
	if metrics.Gets != 2 {
		t.Errorf("Expected Gets=2, got %d", metrics.Gets)
	}
	if metrics.Puts != 2 {
		t.Errorf("Expected Puts=2, got %d", metrics.Puts)
	}
}

// Mock encoder for testing
type mockPoolEncoder struct{}

func (m *mockPoolEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return []byte("mock"), nil
}

func (m *mockPoolEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return []byte("mock"), nil
}

func (m *mockPoolEncoder) ContentType() string {
	return "application/json"
}

func (m *mockPoolEncoder) CanStream() bool {
	return true
}

func (m *mockPoolEncoder) SupportsStreaming() bool {
	return true
}

// Mock decoder for testing
type mockPoolDecoder struct{}

func (m *mockPoolDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return &events.TextMessageContentEvent{}, nil
}

func (m *mockPoolDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return []events.Event{&events.TextMessageContentEvent{}}, nil
}

func (m *mockPoolDecoder) ContentType() string {
	return "application/json"
}

func (m *mockPoolDecoder) CanStream() bool {
	return true
}

func (m *mockPoolDecoder) SupportsStreaming() bool {
	return true
}

func TestGlobalPools(t *testing.T) {
	// Test buffer pools
	buf := GetBuffer(1024)
	if buf == nil {
		t.Fatal("Expected non-nil buffer")
	}
	PutBuffer(buf)

	// Test slice pools
	slice := GetSlice(1024)
	if slice == nil {
		t.Fatal("Expected non-nil slice")
	}
	PutSlice(slice)

	// Test error pools
	encErr := GetEncodingError()
	if encErr == nil {
		t.Fatal("Expected non-nil encoding error")
	}
	PutEncodingError(encErr)

	decErr := GetDecodingError()
	if decErr == nil {
		t.Fatal("Expected non-nil decoding error")
	}
	PutDecodingError(decErr)

	// Test stats
	stats := PoolStats()
	if len(stats) == 0 {
		t.Error("Expected non-empty stats")
	}
}

func TestPoolManager(t *testing.T) {
	pm := NewPoolManager()

	// Register a pool
	bufPool := NewBufferPool(1024)
	pm.RegisterPool("buffer", bufPool)

	// Test retrieval
	retrieved := pm.GetPool("buffer")
	if retrieved == nil {
		t.Error("Expected to retrieve buffer pool")
	}

	// Use the pool to generate metrics
	buf := bufPool.Get()
	bufPool.Put(buf)

	// Test metrics
	metrics := pm.GetAllMetrics()
	if len(metrics) == 0 {
		t.Error("Expected non-empty metrics")
	}

	// Test monitoring
	ch := pm.StartMonitoring(10 * time.Millisecond)

	// Use pool again
	buf = bufPool.Get()
	bufPool.Put(buf)

	// Wait for metrics
	select {
	case receivedMetrics := <-ch:
		if len(receivedMetrics) == 0 {
			t.Error("Expected non-empty metrics from monitoring")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for metrics")
	}
}

func TestPoolReset(t *testing.T) {
	// Use some resources
	buf := GetBuffer(1024)
	PutBuffer(buf)

	// Get initial stats
	stats := PoolStats()
	if len(stats) == 0 {
		t.Error("Expected non-empty stats")
	}

	// Reset pools
	ResetAllPools()

	// Check stats are reset
	stats = PoolStats()
	allZero := true
	for _, metrics := range stats {
		if metrics.Gets != 0 || metrics.Puts != 0 {
			allZero = false
			break
		}
	}
	if !allZero {
		t.Error("Expected all metrics to be zero after reset")
	}
}

func TestBufferPoolResetActiveCounters(t *testing.T) {
	pool := NewBufferPoolWithCapacity(1024, 10)

	// Get multiple buffers to increase activeBuffers counter
	buffers := make([]*bytes.Buffer, 5)
	for i := 0; i < 5; i++ {
		buffers[i] = pool.Get()
		if buffers[i] == nil {
			t.Fatalf("Failed to get buffer %d", i)
		}
	}

	// Reset the pool
	pool.Reset()

	// After reset, we should be able to get buffers again
	// This verifies that activeBuffers was properly reset
	newBuf := pool.Get()
	if newBuf == nil {
		t.Error("Failed to get buffer after reset - activeBuffers counter not properly reset")
	}

	// Clean up
	pool.Put(newBuf)
}

func TestSlicePoolResetActiveCounters(t *testing.T) {
	pool := NewSlicePoolWithCapacity(1024, 4096, 10)

	// Get multiple slices to increase activeSlices counter
	slices := make([][]byte, 5)
	for i := 0; i < 5; i++ {
		slices[i] = pool.Get()
		if slices[i] == nil {
			t.Fatalf("Failed to get slice %d", i)
		}
	}

	// Reset the pool
	pool.Reset()

	// After reset, we should be able to get slices again
	// This verifies that activeSlices was properly reset
	newSlice := pool.Get()
	if newSlice == nil {
		t.Error("Failed to get slice after reset - activeSlices counter not properly reset")
	}

	// Clean up
	pool.Put(newSlice)
}

func TestPooledFactory(t *testing.T) {
	ctx := context.Background()
	factory := NewPooledCodecFactory()

	// Set up mock constructors
	factory.codecPool.SetJSONEncoderConstructor(func() interface{} {
		return &mockPoolEncoder{}
	})
	factory.codecPool.SetJSONDecoderConstructor(func() interface{} {
		return &mockPoolDecoder{}
	})

	// Test JSON codec creation
	codec, err := factory.CreateCodec(ctx, "application/json", &EncodingOptions{}, &DecodingOptions{})
	if err != nil {
		t.Fatalf("Failed to create JSON codec: %v", err)
	}

	// Test interface compliance
	if codec == nil {
		t.Error("Expected non-nil codec")
	}

	// Test content type
	if codec.ContentType() != "application/json" {
		t.Errorf("Expected application/json, got %s", codec.ContentType())
	}

	// Test release (if codec implements release functionality)
	if releasable, ok := codec.(interface{ Release() }); ok {
		releasable.Release()
	}

	// Test metrics
	pool := factory.GetCodecPool()
	metrics := pool.Metrics()
	if metrics.Gets == 0 {
		t.Error("Expected Gets > 0")
	}
}
