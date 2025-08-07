package middleware

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestMiddleware is a simple middleware for testing
type TestMiddleware struct {
	name      string
	priority  int
	enabled   bool
	processed int
	mu        sync.Mutex
}

func NewTestMiddleware(name string, priority int) *TestMiddleware {
	return &TestMiddleware{
		name:     name,
		priority: priority,
		enabled:  true,
	}
}

func (tm *TestMiddleware) Name() string {
	return tm.name
}

func (tm *TestMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	tm.mu.Lock()
	tm.processed++
	tm.mu.Unlock()

	// Add metadata to track middleware execution
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}

	executionOrder, _ := req.Metadata["execution_order"].([]string)
	executionOrder = append(executionOrder, tm.name)
	req.Metadata["execution_order"] = executionOrder

	return next(ctx, req)
}

func (tm *TestMiddleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		tm.enabled = enabled
	}
	return nil
}

func (tm *TestMiddleware) Enabled() bool {
	return tm.enabled
}

func (tm *TestMiddleware) Priority() int {
	return tm.priority
}

func (tm *TestMiddleware) GetProcessed() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.processed
}

func TestMiddlewareChain_Add(t *testing.T) {
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 200}, nil
	}

	chain := NewMiddlewareChain(handler)

	// Test adding middleware
	middleware1 := NewTestMiddleware("test1", 10)
	middleware2 := NewTestMiddleware("test2", 20)

	chain.Add(middleware1)
	chain.Add(middleware2)

	middlewareList := chain.ListMiddleware()
	if len(middlewareList) != 2 {
		t.Errorf("Expected 2 middleware, got %d", len(middlewareList))
	}

	// Higher priority should come first
	if middlewareList[0] != "test2" {
		t.Errorf("Expected test2 first due to higher priority, got %s", middlewareList[0])
	}
}

func TestMiddlewareChain_Process(t *testing.T) {
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		if req.Metadata == nil {
			req.Metadata = make(map[string]interface{})
		}
		executionOrder, _ := req.Metadata["execution_order"].([]string)
		executionOrder = append(executionOrder, "handler")
		req.Metadata["execution_order"] = executionOrder

		return &Response{
			ID:         req.ID,
			StatusCode: 200,
			Body:       map[string]interface{}{"execution_order": executionOrder},
		}, nil
	}

	chain := NewMiddlewareChain(handler)

	// Add middleware with different priorities
	chain.Add(NewTestMiddleware("middleware1", 10))
	chain.Add(NewTestMiddleware("middleware2", 30))
	chain.Add(NewTestMiddleware("middleware3", 20))

	// Create test request
	req := &Request{
		ID:     "test-123",
		Method: "GET",
		Path:   "/test",
	}

	// Process request
	resp, err := chain.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify response
	if resp.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	// Verify execution order (highest priority first)
	expectedOrder := []string{"middleware2", "middleware3", "middleware1", "handler"}
	if body, ok := resp.Body.(map[string]interface{}); ok {
		if executionOrder, ok := body["execution_order"].([]string); ok {
			if len(executionOrder) != len(expectedOrder) {
				t.Errorf("Expected %d items in execution order, got %d", len(expectedOrder), len(executionOrder))
			}

			for i, expected := range expectedOrder {
				if i >= len(executionOrder) || executionOrder[i] != expected {
					t.Errorf("Expected %s at position %d, got %s", expected, i, executionOrder[i])
				}
			}
		} else {
			t.Error("Expected execution_order in response body")
		}
	} else {
		t.Error("Expected map[string]interface{} in response body")
	}
}

func TestMiddlewareChain_Remove(t *testing.T) {
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 200}, nil
	}

	chain := NewMiddlewareChain(handler)

	// Add middleware
	chain.Add(NewTestMiddleware("test1", 10))
	chain.Add(NewTestMiddleware("test2", 20))

	// Verify middleware added
	middlewareList := chain.ListMiddleware()
	if len(middlewareList) != 2 {
		t.Errorf("Expected 2 middleware, got %d", len(middlewareList))
	}

	// Remove middleware
	removed := chain.Remove("test1")
	if !removed {
		t.Error("Expected middleware to be removed")
	}

	// Verify middleware removed
	middlewareList = chain.ListMiddleware()
	if len(middlewareList) != 1 {
		t.Errorf("Expected 1 middleware after removal, got %d", len(middlewareList))
	}

	if middlewareList[0] != "test2" {
		t.Errorf("Expected test2 to remain, got %s", middlewareList[0])
	}

	// Try to remove non-existent middleware
	removed = chain.Remove("nonexistent")
	if removed {
		t.Error("Expected removal of non-existent middleware to return false")
	}
}

func TestMiddlewareChain_Clear(t *testing.T) {
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 200}, nil
	}

	chain := NewMiddlewareChain(handler)

	// Add middleware
	chain.Add(NewTestMiddleware("test1", 10))
	chain.Add(NewTestMiddleware("test2", 20))

	// Clear all middleware
	chain.Clear()

	// Verify all middleware removed
	middlewareList := chain.ListMiddleware()
	if len(middlewareList) != 0 {
		t.Errorf("Expected 0 middleware after clear, got %d", len(middlewareList))
	}
}

func TestMiddlewareChain_DisabledMiddleware(t *testing.T) {
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		if req.Metadata == nil {
			req.Metadata = make(map[string]interface{})
		}
		executionOrder, _ := req.Metadata["execution_order"].([]string)
		executionOrder = append(executionOrder, "handler")
		req.Metadata["execution_order"] = executionOrder

		return &Response{
			ID:         req.ID,
			StatusCode: 200,
			Body:       map[string]interface{}{"execution_order": executionOrder},
		}, nil
	}

	chain := NewMiddlewareChain(handler)

	// Add middleware
	middleware1 := NewTestMiddleware("enabled", 10)
	middleware2 := NewTestMiddleware("disabled", 20)
	middleware2.Configure(map[string]interface{}{"enabled": false})

	chain.Add(middleware1)
	chain.Add(middleware2)

	// Create test request
	req := &Request{
		ID:     "test-123",
		Method: "GET",
		Path:   "/test",
	}

	// Process request
	resp, err := chain.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify only enabled middleware executed
	if body, ok := resp.Body.(map[string]interface{}); ok {
		if executionOrder, ok := body["execution_order"].([]string); ok {
			expectedOrder := []string{"enabled", "handler"}
			if len(executionOrder) != len(expectedOrder) {
				t.Errorf("Expected %d items in execution order, got %d", len(expectedOrder), len(executionOrder))
			}

			for i, expected := range expectedOrder {
				if i >= len(executionOrder) || executionOrder[i] != expected {
					t.Errorf("Expected %s at position %d, got %s", expected, i, executionOrder[i])
				}
			}
		}
	}
}

func TestMiddlewareChain_NilRequest(t *testing.T) {
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 200}, nil
	}

	chain := NewMiddlewareChain(handler)

	// Process nil request
	resp, err := chain.Process(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil request")
	}

	if resp != nil {
		t.Error("Expected nil response for nil request")
	}
}

func TestMiddlewareContext(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-123"

	middlewareCtx := NewMiddlewareContext(ctx, requestID)

	if middlewareCtx.RequestID != requestID {
		t.Errorf("Expected request ID %s, got %s", requestID, middlewareCtx.RequestID)
	}

	// Test metadata operations
	middlewareCtx.SetMetadata("key1", "value1")

	value, exists := middlewareCtx.GetMetadata("key1")
	if !exists {
		t.Error("Expected metadata key1 to exist")
	}

	if value != "value1" {
		t.Errorf("Expected metadata value 'value1', got %v", value)
	}

	// Test elapsed time
	time.Sleep(10 * time.Millisecond)
	elapsed := middlewareCtx.Elapsed()
	if elapsed < 10*time.Millisecond {
		t.Errorf("Expected elapsed time >= 10ms, got %v", elapsed)
	}
}

func TestRequest_TimestampSet(t *testing.T) {
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 200}, nil
	}

	chain := NewMiddlewareChain(handler)

	// Create request without timestamp
	req := &Request{
		ID:     "test-123",
		Method: "GET",
		Path:   "/test",
	}

	// Verify timestamp is zero
	if !req.Timestamp.IsZero() {
		t.Error("Expected request timestamp to be zero initially")
	}

	// Process request
	_, err := chain.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify timestamp was set
	if req.Timestamp.IsZero() {
		t.Error("Expected request timestamp to be set after processing")
	}
}

// Benchmark middleware chain processing
func BenchmarkMiddlewareChain_Process(b *testing.B) {
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 200}, nil
	}

	chain := NewMiddlewareChain(handler)

	// Add multiple middleware
	for i := 0; i < 10; i++ {
		chain.Add(NewTestMiddleware(fmt.Sprintf("middleware_%d", i), i))
	}

	req := &Request{
		ID:     "bench-test",
		Method: "GET",
		Path:   "/test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chain.Process(context.Background(), req)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}
