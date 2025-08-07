package middleware

import (
	"testing"
	"time"
)

// Simple benchmark test without external dependencies
func BenchmarkOriginalConversionOnly(b *testing.B) {
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

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		authReq := ConvertToAuthRequest(req)
		mainReq := ConvertFromAuthRequest(authReq)
		_ = mainReq // prevent optimization
	}
}

func BenchmarkOptimizedConversionOnly(b *testing.B) {
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

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		authReq := ConvertToAuthRequestOptimized(req)
		mainReq := ConvertFromAuthRequestOptimized(authReq)
		putAuthRequestToPool(authReq)
		putRequestToPool(mainReq)
	}
}

// Test memory allocation patterns
func BenchmarkMapCopyOriginal(b *testing.B) {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token123",
		"User-Agent":    "test/1.0",
		"Accept":        "application/json",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		copied := copyStringMap(headers)
		_ = copied
	}
}

func BenchmarkMapCopyOptimized(b *testing.B) {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token123",
		"User-Agent":    "test/1.0",
		"Accept":        "application/json",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		copied := copyStringMapOptimized(headers)
		stringMapPool.Put(copied)
	}
}

// Parallel benchmarks to test concurrency
func BenchmarkOriginalParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		req := &Request{
			ID:     "test-123",
			Method: "GET",
			Path:   "/api/test",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"key": "value",
			},
			Metadata: map[string]interface{}{
				"trace_id": "trace-123",
			},
			Timestamp: time.Now(),
		}

		for pb.Next() {
			authReq := ConvertToAuthRequest(req)
			mainReq := ConvertFromAuthRequest(authReq)
			_ = mainReq
		}
	})
}

func BenchmarkOptimizedParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		req := &Request{
			ID:     "test-123",
			Method: "GET",
			Path:   "/api/test",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"key": "value",
			},
			Metadata: map[string]interface{}{
				"trace_id": "trace-123",
			},
			Timestamp: time.Now(),
		}

		for pb.Next() {
			authReq := ConvertToAuthRequestOptimized(req)
			mainReq := ConvertFromAuthRequestOptimized(authReq)
			putAuthRequestToPool(authReq)
			putRequestToPool(mainReq)
		}
	})
}
