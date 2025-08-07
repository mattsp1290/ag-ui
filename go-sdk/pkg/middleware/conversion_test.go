package middleware

import (
	"testing"
	"time"
)

// Test the conversion functions work correctly
func TestConversionFunctionality(t *testing.T) {
	req := &Request{
		ID:     "test-123",
		Method: "GET",
		Path:   "/api/test",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token123",
		},
		Body: map[string]interface{}{
			"key": "value",
		},
		Metadata: map[string]interface{}{
			"trace_id": "trace-123",
		},
		Timestamp: time.Now(),
	}

	// Test original conversion
	authReq := ConvertToAuthRequest(req)
	if authReq == nil {
		t.Fatal("ConvertToAuthRequest returned nil")
	}
	if authReq.ID != req.ID {
		t.Errorf("Expected ID %s, got %s", req.ID, authReq.ID)
	}

	mainReq := ConvertFromAuthRequest(authReq)
	if mainReq == nil {
		t.Fatal("ConvertFromAuthRequest returned nil")
	}
	if mainReq.ID != req.ID {
		t.Errorf("Expected ID %s, got %s", req.ID, mainReq.ID)
	}

	// Test optimized conversion
	authReqOpt := ConvertToAuthRequestOptimized(req)
	if authReqOpt == nil {
		t.Fatal("ConvertToAuthRequestOptimized returned nil")
	}
	if authReqOpt.ID != req.ID {
		t.Errorf("Expected ID %s, got %s", req.ID, authReqOpt.ID)
	}

	mainReqOpt := ConvertFromAuthRequestOptimized(authReqOpt)
	if mainReqOpt == nil {
		t.Fatal("ConvertFromAuthRequestOptimized returned nil")
	}
	if mainReqOpt.ID != req.ID {
		t.Errorf("Expected ID %s, got %s", req.ID, mainReqOpt.ID)
	}

	// Clean up pools
	putAuthRequestToPool(authReqOpt)
	putRequestToPool(mainReqOpt)

	t.Logf("Conversion tests passed successfully")
}

// Test map copying functions
func TestMapCopyFunctions(t *testing.T) {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token123",
	}

	// Test original
	copied1 := copyStringMap(headers)
	if len(copied1) != len(headers) {
		t.Errorf("Original copy: expected %d items, got %d", len(headers), len(copied1))
	}

	// Test optimized
	copied2 := copyStringMapOptimized(headers)
	if len(copied2) != len(headers) {
		t.Errorf("Optimized copy: expected %d items, got %d", len(headers), len(copied2))
	}

	// Verify content
	for k, v := range headers {
		if copied1[k] != v {
			t.Errorf("Original copy: expected %s=%s, got %s", k, v, copied1[k])
		}
		if copied2[k] != v {
			t.Errorf("Optimized copy: expected %s=%s, got %s", k, v, copied2[k])
		}
	}

	// Return to pool
	stringMapPool.Put(copied2)

	t.Logf("Map copy tests passed successfully")
}

func TestPoolReuse(t *testing.T) {
	// Test that pools are reusing objects
	req := &Request{
		ID:     "pool-test",
		Method: "GET",
		Path:   "/api/pool",
		Headers: map[string]string{
			"Test": "header",
		},
		Metadata: map[string]interface{}{
			"test": "metadata",
		},
		Timestamp: time.Now(),
	}

	// Create and return objects to pools multiple times
	for i := 0; i < 5; i++ {
		authReq := ConvertToAuthRequestOptimized(req)
		mainReq := ConvertFromAuthRequestOptimized(authReq)

		if authReq.ID != req.ID {
			t.Errorf("Iteration %d: ID mismatch", i)
		}

		putAuthRequestToPool(authReq)
		putRequestToPool(mainReq)
	}

	t.Logf("Pool reuse test completed successfully")
}

// Benchmark functions without external dependencies
func BenchmarkOriginalConversion(b *testing.B) {
	req := &Request{
		ID:     "bench-123",
		Method: "POST",
		Path:   "/api/bench",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token123",
			"User-Agent":    "bench/1.0",
		},
		Body: map[string]interface{}{
			"data": "benchmark",
			"num":  42,
		},
		Metadata: map[string]interface{}{
			"trace_id": "trace-bench",
			"user_id":  "user-123",
		},
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		authReq := ConvertToAuthRequest(req)
		mainReq := ConvertFromAuthRequest(authReq)
		_ = mainReq
	}
}

func BenchmarkOptimizedConversion(b *testing.B) {
	req := &Request{
		ID:     "bench-123",
		Method: "POST",
		Path:   "/api/bench",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token123",
			"User-Agent":    "bench/1.0",
		},
		Body: map[string]interface{}{
			"data": "benchmark",
			"num":  42,
		},
		Metadata: map[string]interface{}{
			"trace_id": "trace-bench",
			"user_id":  "user-123",
		},
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		authReq := ConvertToAuthRequestOptimized(req)
		mainReq := ConvertFromAuthRequestOptimized(authReq)
		putAuthRequestToPool(authReq)
		putRequestToPool(mainReq)
	}
}
