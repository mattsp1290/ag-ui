package encoding

import (
	"testing"
	"time"
)

func TestSimpleBufferPool(t *testing.T) {
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

func TestSimpleSlicePool(t *testing.T) {
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

func TestSimpleErrorPool(t *testing.T) {
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

func TestSimpleGlobalPools(t *testing.T) {
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

func TestSimplePoolManager(t *testing.T) {
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

func TestSimplePoolReset(t *testing.T) {
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