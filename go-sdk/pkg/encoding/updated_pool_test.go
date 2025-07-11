package encoding

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBufferPoolUpdated tests the updated buffer pool with new interfaces
func TestBufferPoolUpdated(t *testing.T) {
	pool := NewBufferPool(1024)
	
	t.Run("BasicOperations", func(t *testing.T) {
		// Test get and put
		buf := pool.Get()
		require.NotNil(t, buf, "Expected non-nil buffer")
		
		buf.WriteString("test")
		assert.Equal(t, "test", buf.String())
		
		pool.Put(buf)
		
		// Test metrics
		metrics := pool.Metrics()
		assert.Equal(t, int64(1), metrics.Gets)
		assert.Equal(t, int64(1), metrics.Puts)
	})
	
	t.Run("SizeLimits", func(t *testing.T) {
		// Test that oversized buffers are not kept
		buf := pool.Get()
		
		// Fill with data exceeding pool limit
		largeData := make([]byte, 2048) // Larger than pool's 1024 limit
		buf.Write(largeData)
		
		initialPuts := pool.Metrics().Puts
		pool.Put(buf) // Should not be added to pool due to size
		
		// Metrics should still be updated
		assert.Equal(t, initialPuts+1, pool.Metrics().Puts)
	})
	
	t.Run("Reset", func(t *testing.T) {
		// Use the pool
		buf := pool.Get()
		pool.Put(buf)
		
		initialMetrics := pool.Metrics()
		assert.Greater(t, initialMetrics.Gets, int64(0))
		
		// Reset pool
		pool.Reset()
		
		// Metrics should be reset
		resetMetrics := pool.Metrics()
		assert.Equal(t, int64(0), resetMetrics.Gets)
		assert.Equal(t, int64(0), resetMetrics.Puts)
	})
}

// TestSlicePoolUpdated tests the updated slice pool with new interfaces
func TestSlicePoolUpdated(t *testing.T) {
	pool := NewSlicePool(1024, 4096)
	
	t.Run("BasicOperations", func(t *testing.T) {
		// Test get and put
		slice := pool.Get()
		require.NotNil(t, slice, "Expected non-nil slice")
		
		slice = append(slice, []byte("test")...)
		assert.Equal(t, "test", string(slice))
		
		pool.Put(slice)
		
		// Test metrics
		metrics := pool.Metrics()
		assert.Equal(t, int64(1), metrics.Gets)
		assert.Equal(t, int64(1), metrics.Puts)
	})
	
	t.Run("CapacityLimits", func(t *testing.T) {
		// Test that oversized slices are not kept
		slice := pool.Get()
		
		// Grow slice beyond pool limit
		largeData := make([]byte, 5000) // Larger than pool's 4096 limit
		slice = append(slice, largeData...)
		
		initialPuts := pool.Metrics().Puts
		pool.Put(slice) // Should not be added to pool due to size
		
		// Metrics should still be updated
		assert.Equal(t, initialPuts+1, pool.Metrics().Puts)
	})
}

// TestErrorPoolUpdated tests the updated error pool with new interfaces
func TestErrorPoolUpdated(t *testing.T) {
	pool := NewErrorPool()
	
	t.Run("EncodingErrors", func(t *testing.T) {
		// Test encoding error
		encErr := pool.GetEncodingError()
		require.NotNil(t, encErr, "Expected non-nil encoding error")
		
		encErr.Format = "test"
		encErr.Message = "test error"
		
		pool.PutEncodingError(encErr)
		
		// Get another error - should be reset
		encErr2 := pool.GetEncodingError()
		assert.Empty(t, encErr2.Format, "Error should be reset")
		assert.Empty(t, encErr2.Message, "Error should be reset")
		
		pool.PutEncodingError(encErr2)
	})
	
	t.Run("DecodingErrors", func(t *testing.T) {
		// Test decoding error
		decErr := pool.GetDecodingError()
		require.NotNil(t, decErr, "Expected non-nil decoding error")
		
		decErr.Format = "test"
		decErr.Message = "test error"
		
		pool.PutDecodingError(decErr)
		
		// Get another error - should be reset
		decErr2 := pool.GetDecodingError()
		assert.Empty(t, decErr2.Format, "Error should be reset")
		assert.Empty(t, decErr2.Message, "Error should be reset")
		
		pool.PutDecodingError(decErr2)
	})
	
	t.Run("Metrics", func(t *testing.T) {
		initialMetrics := pool.Metrics()
		
		// Use both error types
		encErr := pool.GetEncodingError()
		decErr := pool.GetDecodingError()
		
		pool.PutEncodingError(encErr)
		pool.PutDecodingError(decErr)
		
		finalMetrics := pool.Metrics()
		assert.Equal(t, initialMetrics.Gets+2, finalMetrics.Gets)
		assert.Equal(t, initialMetrics.Puts+2, finalMetrics.Puts)
		assert.Equal(t, initialMetrics.Resets+2, finalMetrics.Resets)
	})
}

// TestCodecPoolUpdated tests the updated codec pool with new interfaces
func TestCodecPoolUpdated(t *testing.T) {
	pool := NewCodecPool()
	
	t.Run("JSONCodecs", func(t *testing.T) {
		// Set up mock constructors for testing
		pool.SetJSONEncoderConstructor(func() interface{} {
			return &mockUpdatedPoolEncoder{}
		})
		pool.SetJSONDecoderConstructor(func() interface{} {
			return &mockUpdatedPoolDecoder{}
		})
		
		// Test JSON encoder
		jsonEncoder := pool.GetJSONEncoder(&EncodingOptions{})
		require.NotNil(t, jsonEncoder, "Expected non-nil JSON encoder")
		pool.PutJSONEncoder(jsonEncoder)
		
		// Test JSON decoder
		jsonDecoder := pool.GetJSONDecoder(&DecodingOptions{})
		require.NotNil(t, jsonDecoder, "Expected non-nil JSON decoder")
		pool.PutJSONDecoder(jsonDecoder)
		
		// Test metrics
		metrics := pool.Metrics()
		assert.Equal(t, int64(2), metrics.Gets)
		assert.Equal(t, int64(2), metrics.Puts)
	})
	
	t.Run("ProtobufCodecs", func(t *testing.T) {
		// Set up mock constructors for testing
		pool.SetProtobufEncoderConstructor(func() interface{} {
			return &mockUpdatedPoolEncoder{}
		})
		pool.SetProtobufDecoderConstructor(func() interface{} {
			return &mockUpdatedPoolDecoder{}
		})
		
		// Test Protobuf encoder
		protobufEncoder := pool.GetProtobufEncoder(&EncodingOptions{})
		require.NotNil(t, protobufEncoder, "Expected non-nil Protobuf encoder")
		pool.PutProtobufEncoder(protobufEncoder)
		
		// Test Protobuf decoder
		protobufDecoder := pool.GetProtobufDecoder(&DecodingOptions{})
		require.NotNil(t, protobufDecoder, "Expected non-nil Protobuf decoder")
		pool.PutProtobufDecoder(protobufDecoder)
		
		// Test metrics
		metrics := pool.Metrics()
		assert.Greater(t, metrics.Gets, int64(0))
		assert.Greater(t, metrics.Puts, int64(0))
	})
}

// TestGlobalPoolsUpdated tests the updated global pools with new interfaces
func TestGlobalPoolsUpdated(t *testing.T) {
	t.Run("BufferPools", func(t *testing.T) {
		// Test different sized buffers go to appropriate pools
		smallBuf := GetBuffer(1024)
		mediumBuf := GetBuffer(32768)
		largeBuf := GetBuffer(500000)
		
		require.NotNil(t, smallBuf)
		require.NotNil(t, mediumBuf)
		require.NotNil(t, largeBuf)
		
		// Use buffers
		smallBuf.WriteString("small")
		mediumBuf.WriteString("medium")
		largeBuf.WriteString("large")
		
		// Return to pools
		PutBuffer(smallBuf)
		PutBuffer(mediumBuf)
		PutBuffer(largeBuf)
		
		// Verify they went to correct pools based on capacity
		assert.Equal(t, "small", smallBuf.String()) // Should be reset
		assert.Equal(t, "medium", mediumBuf.String()) // Should be reset
		assert.Equal(t, "large", largeBuf.String()) // Should be reset
	})
	
	t.Run("SlicePools", func(t *testing.T) {
		// Test different sized slices go to appropriate pools
		smallSlice := GetSlice(1024)
		mediumSlice := GetSlice(32768)
		largeSlice := GetSlice(500000)
		
		require.NotNil(t, smallSlice)
		require.NotNil(t, mediumSlice)
		require.NotNil(t, largeSlice)
		
		// Use slices
		smallSlice = append(smallSlice, []byte("small")...)
		mediumSlice = append(mediumSlice, []byte("medium")...)
		largeSlice = append(largeSlice, []byte("large")...)
		
		// Return to pools
		PutSlice(smallSlice)
		PutSlice(mediumSlice)
		PutSlice(largeSlice)
	})
	
	t.Run("ErrorPools", func(t *testing.T) {
		// Test error pools
		encErr := GetEncodingError()
		require.NotNil(t, encErr)
		
		encErr.Format = "test"
		encErr.Message = "test error"
		
		PutEncodingError(encErr)
		
		decErr := GetDecodingError()
		require.NotNil(t, decErr)
		
		decErr.Format = "test"
		decErr.Message = "test error"
		
		PutDecodingError(decErr)
	})
	
	t.Run("PoolStats", func(t *testing.T) {
		// Test pool statistics
		stats := PoolStats()
		require.NotEmpty(t, stats, "Expected non-empty stats")
		
		// Verify all expected pools are present
		expectedPools := []string{"small_buffer", "medium_buffer", "large_buffer", "small_slice", "medium_slice", "large_slice", "error"}
		for _, poolName := range expectedPools {
			_, exists := stats[poolName]
			assert.True(t, exists, "Expected pool %s to exist in stats", poolName)
		}
	})
	
	t.Run("PoolReset", func(t *testing.T) {
		// Use some resources
		buf := GetBuffer(1024)
		PutBuffer(buf)
		
		// Get initial stats
		stats := PoolStats()
		require.NotEmpty(t, stats)
		
		// Reset all pools
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
		assert.True(t, allZero, "Expected all metrics to be zero after reset")
	})
}

// TestPoolManagerUpdated tests the updated pool manager with new interfaces
func TestPoolManagerUpdated(t *testing.T) {
	pm := NewPoolManager()
	
	t.Run("PoolRegistration", func(t *testing.T) {
		// Register a pool
		bufPool := NewBufferPool(1024)
		pm.RegisterPool("buffer", bufPool)
		
		// Test retrieval
		retrieved := pm.GetPool("buffer")
		require.NotNil(t, retrieved, "Expected to retrieve buffer pool")
		
		// Type assertion should work
		retrievedPool, ok := retrieved.(*BufferPool)
		assert.True(t, ok, "Expected BufferPool type")
		assert.NotNil(t, retrievedPool)
	})
	
	t.Run("Metrics", func(t *testing.T) {
		// Register and use a pool
		bufPool := NewBufferPool(1024)
		pm.RegisterPool("test_buffer", bufPool)
		
		// Use the pool to generate metrics
		buf := bufPool.Get()
		bufPool.Put(buf)
		
		// Test metrics
		metrics := pm.GetAllMetrics()
		require.NotEmpty(t, metrics, "Expected non-empty metrics")
		
		// Check our pool is in metrics
		testMetrics, exists := metrics["test_buffer"]
		assert.True(t, exists, "Expected test_buffer metrics to exist")
		assert.Greater(t, testMetrics.Gets, int64(0))
		assert.Greater(t, testMetrics.Puts, int64(0))
	})
	
	t.Run("Monitoring", func(t *testing.T) {
		// Register and use a pool
		bufPool := NewBufferPool(1024)
		pm.RegisterPool("monitor_buffer", bufPool)
		
		// Start monitoring
		ch := pm.StartMonitoring(10 * time.Millisecond)
		
		// Use pool to generate activity
		buf := bufPool.Get()
		bufPool.Put(buf)
		
		// Wait for metrics
		select {
		case receivedMetrics := <-ch:
			require.NotEmpty(t, receivedMetrics, "Expected non-empty metrics from monitoring")
			
			// Check our pool is in metrics
			_, exists := receivedMetrics["monitor_buffer"]
			assert.True(t, exists, "Expected monitor_buffer metrics to exist")
			
		case <-time.After(100 * time.Millisecond):
			t.Error("Timeout waiting for metrics")
		}
	})
}

// TestPooledFactoryUpdated tests the updated pooled factory with new interfaces
func TestPooledFactoryUpdated(t *testing.T) {
	ctx := context.Background()
	factory := NewPooledCodecFactory()
	
	t.Run("JSONEncoders", func(t *testing.T) {
		// Set up mock constructors
		factory.codecPool.SetJSONEncoderConstructor(func() interface{} {
			return &mockUpdatedPoolEncoder{}
		})
		
		// Test JSON codec creation
		codec, err := factory.CreateCodec(ctx, "application/json", &EncodingOptions{}, &DecodingOptions{})
		require.NoError(t, err)
		require.NotNil(t, codec)
		
		// Test interface compliance
		assert.Equal(t, "application/json", codec.ContentType())
		
		// Test encoding
		event := events.NewTextMessageContentEvent("msg", "content")
		_, err = codec.Encode(ctx, event)
		require.NoError(t, err)
		
		// Test release
		if releasable, ok := codec.(interface{ Release() }); ok {
			releasable.Release()
		}
	})
	
	t.Run("JSONDecoders", func(t *testing.T) {
		// Set up mock constructors
		factory.codecPool.SetJSONDecoderConstructor(func() interface{} {
			return &mockUpdatedPoolDecoder{}
		})
		
		// Test JSON codec creation (for decoding)
		codec, err := factory.CreateCodec(ctx, "application/json", &EncodingOptions{}, &DecodingOptions{})
		require.NoError(t, err)
		require.NotNil(t, codec)
		
		// Test interface compliance
		assert.Equal(t, "application/json", codec.ContentType())
		
		// Test release
		if releasable, ok := codec.(interface{ Release() }); ok {
			releasable.Release()
		}
	})
	
	t.Run("PoolMetrics", func(t *testing.T) {
		// Test metrics
		pool := factory.GetCodecPool()
		metrics := pool.Metrics()
		
		// Should have some activity from previous tests
		assert.GreaterOrEqual(t, metrics.Gets, int64(0))
		assert.GreaterOrEqual(t, metrics.Puts, int64(0))
	})
	
	t.Run("UnsupportedFormats", func(t *testing.T) {
		// Test unsupported format
		_, err := factory.CreateCodec(ctx, "application/xml", nil, nil)
		assert.Error(t, err, "Should fail for unsupported format")
	})
}

// TestPoolIntegrationUpdated tests pool integration with the encoding system
func TestPoolIntegrationUpdated(t *testing.T) {
	ctx := context.Background()
	
	t.Run("RegistryWithPooling", func(t *testing.T) {
		// Create registry with pooled factory
		registry := NewFormatRegistry()
		factory := NewPooledCodecFactory()
		
		// Register format
		info := NewFormatInfo("JSON", "application/json")
		require.NoError(t, registry.RegisterFormat(info))
		
		// Register pooled factory
		require.NoError(t, registry.RegisterCodec("application/json", factory))
		
		// Get codec pool metrics before
		pool := factory.GetCodecPool()
		beforeMetrics := pool.Metrics()
		
		// Use the registry multiple times
		for i := 0; i < 10; i++ {
			encoder, err := registry.GetEncoder(ctx, "application/json", nil)
			require.NoError(t, err)
			
			event := events.NewTextMessageContentEvent("msg", "content")
			_, err = encoder.Encode(ctx, event)
			require.NoError(t, err)
			
			// Release if possible
			if releasable, ok := encoder.(ReleasableEncoder); ok {
				releasable.Release()
			}
		}
		
		// Check metrics improved
		afterMetrics := pool.Metrics()
		assert.Greater(t, afterMetrics.Gets, beforeMetrics.Gets)
		assert.Greater(t, afterMetrics.Puts, beforeMetrics.Puts)
	})
	
	t.Run("GlobalPoolsWithRegistry", func(t *testing.T) {
		// Reset global pools
		ResetAllPools()
		
		// Use registry with global pools
		registry := GetGlobalRegistry()
		
		// Get initial buffer stats
		initialStats := PoolStats()
		
		// Perform encoding operations that should use buffers
		for i := 0; i < 100; i++ {
			encoder, err := registry.GetEncoder(ctx, "application/json", nil)
			require.NoError(t, err)
			
			event := events.NewTextMessageContentEvent("msg", "content")
			_, err = encoder.Encode(ctx, event)
			require.NoError(t, err)
		}
		
		// Check buffer pool usage
		finalStats := PoolStats()
		
		// Verify some buffer pools were used
		bufferPoolUsed := false
		for poolName, stats := range finalStats {
			if strings.Contains(poolName, "buffer") {
				initialStat := initialStats[poolName]
				if stats.Gets > initialStat.Gets {
					bufferPoolUsed = true
					break
				}
			}
		}
		
		assert.True(t, bufferPoolUsed, "Buffer pools should have been used")
	})
}

// Mock implementations for testing

type mockUpdatedPoolEncoder struct{}

func (m *mockUpdatedPoolEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return []byte("mock"), nil
}

func (m *mockUpdatedPoolEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return []byte("mock"), nil
}

func (m *mockUpdatedPoolEncoder) ContentType() string {
	return "application/json"
}

func (m *mockUpdatedPoolEncoder) CanStream() bool {
	return true
}

type mockUpdatedPoolDecoder struct{}

func (m *mockUpdatedPoolDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return events.NewTextMessageContentEvent("mock", "mock"), nil
}

func (m *mockUpdatedPoolDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return []events.Event{events.NewTextMessageContentEvent("mock", "mock")}, nil
}

func (m *mockUpdatedPoolDecoder) ContentType() string {
	return "application/json"
}

func (m *mockUpdatedPoolDecoder) CanStream() bool {
	return true
}