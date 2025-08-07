package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/auth"
)

// Mock auth middleware for testing
type BenchmarkAuthMiddleware struct{}

func (m *BenchmarkAuthMiddleware) Name() string                                  { return "benchmark" }
func (m *BenchmarkAuthMiddleware) Configure(config map[string]interface{}) error { return nil }
func (m *BenchmarkAuthMiddleware) Enabled() bool                                 { return true }
func (m *BenchmarkAuthMiddleware) Priority() int                                 { return 1 }

func (m *BenchmarkAuthMiddleware) Process(ctx context.Context, req *auth.Request, next auth.NextHandler) (*auth.Response, error) {
	// Simulate some processing
	time.Sleep(10 * time.Microsecond)
	return next(ctx, req)
}

// Helper function to create test request
func createTestRequest() *Request {
	return &Request{
		ID:     "test-123",
		Method: "GET",
		Path:   "/api/test",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token123",
			"User-Agent":    "test-client/1.0",
		},
		Body: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		},
		Metadata: map[string]interface{}{
			"trace_id": "trace-123",
			"user_id":  "user-456",
		},
		Timestamp: time.Now(),
	}
}

// Helper function to create test next handler
func createTestNextHandler() NextHandler {
	return func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{
			ID:         req.ID,
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"status": "success",
			},
			Metadata: map[string]interface{}{
				"processed": true,
			},
			Timestamp: time.Now(),
			Duration:  time.Millisecond,
		}, nil
	}
}

// Benchmark original auth adapter
func BenchmarkOriginalAuthAdapter(b *testing.B) {
	middleware := &BenchmarkAuthMiddleware{}
	adapter := NewAuthMiddlewareAdapter(middleware)
	req := createTestRequest()
	next := createTestNextHandler()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := adapter.Process(ctx, req, next)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark optimized auth adapter
func BenchmarkOptimizedAuthAdapter(b *testing.B) {
	middleware := &BenchmarkAuthMiddleware{}
	adapter := NewOptimizedAuthMiddlewareAdapter(middleware)
	req := createTestRequest()
	next := createTestNextHandler()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := adapter.Process(ctx, req, next)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark original conversion functions
func BenchmarkOriginalConversions(b *testing.B) {
	req := createTestRequest()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		authReq := ConvertToAuthRequest(req)
		mainReq := ConvertFromAuthRequest(authReq)
		_ = mainReq // Prevent optimization
	}
}

// Benchmark optimized conversion functions
func BenchmarkOptimizedConversions(b *testing.B) {
	req := createTestRequest()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		authReq := ConvertToAuthRequestOptimized(req)
		mainReq := ConvertFromAuthRequestOptimized(authReq)
		putAuthRequestToPool(authReq)
		putRequestToPool(mainReq)
	}
}

// Benchmark map copying - original vs optimized
func BenchmarkOriginalMapCopy(b *testing.B) {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token123",
		"User-Agent":    "test-client/1.0",
		"Accept":        "application/json",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = copyStringMap(headers)
	}
}

func BenchmarkOptimizedMapCopy(b *testing.B) {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token123",
		"User-Agent":    "test-client/1.0",
		"Accept":        "application/json",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		copied := copyStringMapOptimized(headers)
		// Return to pool to simulate real usage
		stringMapPool.Put(copied)
	}
}

// Benchmark concurrent usage
func BenchmarkConcurrentOriginal(b *testing.B) {
	middleware := &BenchmarkAuthMiddleware{}
	adapter := NewAuthMiddlewareAdapter(middleware)
	next := createTestNextHandler()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := createTestRequest()
			_, err := adapter.Process(ctx, req, next)
			if err != nil {
				b.Error(err)
			}
		}
	})
}

func BenchmarkConcurrentOptimized(b *testing.B) {
	middleware := &BenchmarkAuthMiddleware{}
	adapter := NewOptimizedAuthMiddlewareAdapter(middleware)
	next := createTestNextHandler()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := createTestRequest()
			_, err := adapter.Process(ctx, req, next)
			if err != nil {
				b.Error(err)
			}
		}
	})
}

// Memory allocation benchmarks
func BenchmarkMemoryAllocOriginal(b *testing.B) {
	req := createTestRequest()
	resp := &Response{
		ID:         "test-123",
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]interface{}{
			"status": "success",
		},
		Metadata: map[string]interface{}{
			"processed": true,
		},
		Timestamp: time.Now(),
		Duration:  time.Millisecond,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		authReq := ConvertToAuthRequest(req)
		authResp := ConvertToAuthResponse(resp)
		mainReq := ConvertFromAuthRequest(authReq)
		mainResp := ConvertFromAuthResponse(authResp)
		_ = mainReq
		_ = mainResp
	}
}

func BenchmarkMemoryAllocOptimized(b *testing.B) {
	req := createTestRequest()
	resp := &Response{
		ID:         "test-123",
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]interface{}{
			"status": "success",
		},
		Metadata: map[string]interface{}{
			"processed": true,
		},
		Timestamp: time.Now(),
		Duration:  time.Millisecond,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		authReq := ConvertToAuthRequestOptimized(req)
		authResp := ConvertToAuthResponseOptimized(resp)
		mainReq := ConvertFromAuthRequestOptimized(authReq)
		mainResp := ConvertFromAuthResponseOptimized(authResp)

		// Clean up
		putAuthRequestToPool(authReq)
		putAuthResponseToPool(authResp)
		putRequestToPool(mainReq)
		putResponseToPool(mainResp)
	}
}
