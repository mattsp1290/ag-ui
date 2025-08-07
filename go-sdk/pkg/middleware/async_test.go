package middleware

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAsyncMiddlewareChain(t *testing.T) {
	t.Run("Basic Async Processing", func(t *testing.T) {
		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "processed",
				Timestamp:  time.Now(),
			}, nil
		}

		chain := NewAsyncMiddlewareChain(handler, 5, 10*time.Second)

		req := &Request{
			ID:     "test-1",
			Method: "GET",
			Path:   "/test",
		}

		resultChan := chain.ProcessAsync(context.Background(), req)

		select {
		case result := <-resultChan:
			if result.Error != nil {
				t.Fatalf("Expected no error, got %v", result.Error)
			}
			if result.Response.StatusCode != 200 {
				t.Fatalf("Expected status code 200, got %d", result.Response.StatusCode)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Async processing timed out")
		}
	})

	t.Run("Concurrency Limits", func(t *testing.T) {
		const maxConcurrency = 2
		const totalRequests = 5

		var activeCount int64
		var maxActiveCount int64

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			active := atomic.AddInt64(&activeCount, 1)
			defer atomic.AddInt64(&activeCount, -1)

			// Track maximum concurrent requests
			for {
				current := atomic.LoadInt64(&maxActiveCount)
				if active <= current || atomic.CompareAndSwapInt64(&maxActiveCount, current, active) {
					break
				}
			}

			// Simulate work
			time.Sleep(100 * time.Millisecond)

			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "processed",
				Timestamp:  time.Now(),
			}, nil
		}

		chain := NewAsyncMiddlewareChain(handler, maxConcurrency, 5*time.Second)

		var wg sync.WaitGroup
		for i := 0; i < totalRequests; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				req := &Request{
					ID:     fmt.Sprintf("test-%d", id),
					Method: "GET",
					Path:   "/test",
				}

				resultChan := chain.ProcessAsync(context.Background(), req)
				<-resultChan // Wait for result
			}(i)
		}

		wg.Wait()

		// Verify that concurrency was limited
		if atomic.LoadInt64(&maxActiveCount) > int64(maxConcurrency) {
			t.Fatalf("Expected max concurrency %d, but saw %d", maxConcurrency, maxActiveCount)
		}
	})

	t.Run("Timeout Handling", func(t *testing.T) {
		handler := func(ctx context.Context, req *Request) (*Response, error) {
			// Simulate slow processing
			time.Sleep(2 * time.Second)
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "processed",
				Timestamp:  time.Now(),
			}, nil
		}

		chain := NewAsyncMiddlewareChain(handler, 5, 500*time.Millisecond)

		req := &Request{
			ID:     "test-timeout",
			Method: "GET",
			Path:   "/test",
		}

		resultChan := chain.ProcessAsync(context.Background(), req)

		select {
		case result := <-resultChan:
			if result.Error == nil {
				t.Fatal("Expected timeout error, got nil")
			}
			if !strings.Contains(fmt.Sprintf("%v", result.Error), "timeout") {
				t.Fatalf("Expected timeout error, got %v", result.Error)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out waiting for result")
		}
	})

	t.Run("Async Middleware Wrapper", func(t *testing.T) {
		// Create a simple middleware
		middleware := &AsyncTestMiddleware{
			name:     "test-middleware",
			enabled:  true,
			priority: 100,
		}

		// Wrap it for async processing
		asyncMiddleware := NewAsyncMiddlewareWrapper(middleware)

		if asyncMiddleware.Name() != "test-middleware" {
			t.Fatalf("Expected name 'test-middleware', got %s", asyncMiddleware.Name())
		}

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "processed",
				Timestamp:  time.Now(),
			}, nil
		}

		req := &Request{
			ID:     "test-wrapper",
			Method: "GET",
			Path:   "/test",
		}

		resultChan := asyncMiddleware.ProcessAsync(context.Background(), req, handler)

		select {
		case result := <-resultChan:
			if result.Error != nil {
				t.Fatalf("Expected no error, got %v", result.Error)
			}
			if result.Response.StatusCode != 200 {
				t.Fatalf("Expected status code 200, got %d", result.Response.StatusCode)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Async wrapper processing timed out")
		}
	})
}

func TestAsyncBatchProcessor(t *testing.T) {
	t.Run("Batch Processing", func(t *testing.T) {
		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       fmt.Sprintf("processed-%s", req.ID),
				Timestamp:  time.Now(),
			}, nil
		}

		chain := NewAsyncMiddlewareChain(handler, 10, 5*time.Second)
		processor := NewAsyncBatchProcessor(chain, 3) // Batch size of 3

		// Create test requests
		requests := make([]*Request, 5)
		for i := 0; i < 5; i++ {
			requests[i] = &Request{
				ID:     fmt.Sprintf("batch-%d", i),
				Method: "GET",
				Path:   "/test",
			}
		}

		results, err := processor.ProcessBatch(context.Background(), requests)
		if err != nil {
			t.Fatalf("Batch processing failed: %v", err)
		}

		if len(results) != len(requests) {
			t.Fatalf("Expected %d results, got %d", len(requests), len(results))
		}

		// Verify all requests were processed
		for i, result := range results {
			if result.Error != nil {
				t.Fatalf("Request %d failed: %v", i, result.Error)
			}
			if result.Response.StatusCode != 200 {
				t.Fatalf("Request %d: expected status code 200, got %d", i, result.Response.StatusCode)
			}
		}
	})

	t.Run("Batch Processing with Timeout", func(t *testing.T) {
		handler := func(ctx context.Context, req *Request) (*Response, error) {
			// Some requests take longer
			if req.ID == "batch-1" {
				time.Sleep(2 * time.Second)
			}
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       fmt.Sprintf("processed-%s", req.ID),
				Timestamp:  time.Now(),
			}, nil
		}

		chain := NewAsyncMiddlewareChain(handler, 10, 5*time.Second)
		processor := NewAsyncBatchProcessor(chain, 2)

		requests := []*Request{
			{ID: "batch-0", Method: "GET", Path: "/test"},
			{ID: "batch-1", Method: "GET", Path: "/test"}, // This one will timeout
			{ID: "batch-2", Method: "GET", Path: "/test"},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		results, err := processor.ProcessBatch(ctx, requests)

		// Should get timeout error
		if err == nil {
			t.Fatal("Expected timeout error, got nil")
		}

		if len(results) != len(requests) {
			t.Fatalf("Expected %d results, got %d", len(requests), len(results))
		}
	})
}

func TestAsyncMiddlewarePool(t *testing.T) {
	t.Run("Pool Operations", func(t *testing.T) {
		factory := func() AsyncMiddleware {
			return NewAsyncMiddlewareWrapper(&AsyncTestMiddleware{
				name:     "pooled-middleware",
				enabled:  true,
				priority: 100,
			})
		}

		pool := NewAsyncMiddlewarePool(3, factory)
		defer pool.Close()

		// Get instances from pool
		instances := make([]AsyncMiddleware, 5)
		for i := 0; i < 5; i++ {
			instances[i] = pool.Get()
			if instances[i] == nil {
				t.Fatalf("Failed to get instance %d from pool", i)
			}
		}

		// Return instances to pool
		for _, instance := range instances {
			pool.Put(instance)
		}
	})
}

// AsyncTestMiddleware for async testing purposes (renamed to avoid conflict)
type AsyncTestMiddleware struct {
	name      string
	enabled   bool
	priority  int
	processed int
	mu        sync.Mutex
}

func (tm *AsyncTestMiddleware) Name() string {
	return tm.name
}

func (tm *AsyncTestMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	tm.mu.Lock()
	tm.processed++
	tm.mu.Unlock()

	// Add some test metadata
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}
	req.Metadata[tm.name] = "processed"

	return next(ctx, req)
}

func (tm *AsyncTestMiddleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		tm.enabled = enabled
	}
	if priority, ok := config["priority"].(int); ok {
		tm.priority = priority
	}
	return nil
}

func (tm *AsyncTestMiddleware) Enabled() bool {
	return tm.enabled
}

func (tm *AsyncTestMiddleware) Priority() int {
	return tm.priority
}

func (tm *AsyncTestMiddleware) GetProcessed() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.processed
}

func BenchmarkAsyncMiddleware(b *testing.B) {
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{
			ID:         req.ID,
			StatusCode: 200,
			Body:       "processed",
			Timestamp:  time.Now(),
		}, nil
	}

	chain := NewAsyncMiddlewareChain(handler, 100, 30*time.Second)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := &Request{
				ID:     fmt.Sprintf("bench-%d", i),
				Method: "GET",
				Path:   "/test",
			}

			resultChan := chain.ProcessAsync(context.Background(), req)
			<-resultChan
			i++
		}
	})
}
