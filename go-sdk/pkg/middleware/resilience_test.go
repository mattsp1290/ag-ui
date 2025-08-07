package middleware

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

func TestResilienceMiddleware(t *testing.T) {
	t.Run("Circuit Breaker Integration", func(t *testing.T) {
		cbConfig := &errors.CircuitBreakerConfig{
			Name:             "test-circuit-breaker",
			MaxFailures:      3,
			ResetTimeout:     1 * time.Second,
			HalfOpenMaxCalls: 1,
			SuccessThreshold: 1,
			Timeout:          5 * time.Second,
		}

		retryConfig := &RetryConfig{
			MaxAttempts:     2,
			InitialDelay:    100 * time.Millisecond,
			MaxDelay:        1 * time.Second,
			BackoffFactor:   2.0,
			RetryableErrors: []string{"TIMEOUT", "SERVICE_UNAVAILABLE"},
		}

		middleware := NewResilienceMiddleware("test-resilience", cbConfig, retryConfig, nil)

		var failureCount int64
		handler := func(ctx context.Context, req *Request) (*Response, error) {
			count := atomic.AddInt64(&failureCount, 1)
			if count <= 3 {
				return nil, fmt.Errorf("SERVICE_UNAVAILABLE")
			}
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "success",
				Timestamp:  time.Now(),
			}, nil
		}

		req := &Request{
			ID:     "test-cb",
			Method: "GET",
			Path:   "/test",
		}

		// First few requests should fail and eventually trip the circuit breaker
		for i := 0; i < 5; i++ {
			_, err := middleware.Process(context.Background(), req, handler)

			if i < 3 {
				// Should retry and eventually fail
				if err == nil {
					t.Fatalf("Expected error on request %d, got success", i)
				}
			} else if i == 3 {
				// Circuit breaker should be open now
				if err == nil {
					t.Fatalf("Expected circuit breaker to be open on request %d", i)
				}
				if !strings.Contains(strings.ToLower(err.Error()), "circuit breaker") {
					t.Fatalf("Expected circuit breaker error, got %v", err)
				}
			}
		}

		// Get stats to verify circuit breaker state
		stats := middleware.GetCircuitBreakerStats()
		if stats.CircuitBreakerState != "OPEN" {
			t.Fatalf("Expected circuit breaker to be OPEN, got %s", stats.CircuitBreakerState)
		}
	})

	t.Run("Retry Logic", func(t *testing.T) {
		retryConfig := &RetryConfig{
			MaxAttempts:     3,
			InitialDelay:    50 * time.Millisecond,
			MaxDelay:        1 * time.Second,
			BackoffFactor:   2.0,
			RetryableErrors: []string{"TIMEOUT", "SERVICE_UNAVAILABLE"},
		}

		middleware := NewResilienceMiddleware("test-retry", nil, retryConfig, nil)

		var attemptCount int64
		handler := func(ctx context.Context, req *Request) (*Response, error) {
			count := atomic.AddInt64(&attemptCount, 1)
			if count < 3 {
				return nil, fmt.Errorf("TIMEOUT")
			}
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "success after retries",
				Timestamp:  time.Now(),
			}, nil
		}

		req := &Request{
			ID:     "test-retry",
			Method: "GET",
			Path:   "/test",
		}

		start := time.Now()
		resp, err := middleware.Process(context.Background(), req, handler)
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Expected success after retries, got %v", err)
		}

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status code 200, got %d", resp.StatusCode)
		}

		// Should have made 3 attempts
		if atomic.LoadInt64(&attemptCount) != 3 {
			t.Fatalf("Expected 3 attempts, got %d", attemptCount)
		}

		// Should have taken at least the retry delays
		expectedMinDuration := 50*time.Millisecond + 100*time.Millisecond // First retry + second retry
		if duration < expectedMinDuration {
			t.Fatalf("Expected duration >= %v, got %v", expectedMinDuration, duration)
		}
	})

	t.Run("Rate Limiting", func(t *testing.T) {
		rateLimiter := NewRateLimiter(2, 2) // 2 tokens per second, bucket size 2
		middleware := NewResilienceMiddleware("test-rate-limit", nil, nil, rateLimiter)

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "success",
				Timestamp:  time.Now(),
			}, nil
		}

		// First two requests should succeed
		for i := 0; i < 2; i++ {
			req := &Request{
				ID:     fmt.Sprintf("test-rl-%d", i),
				Method: "GET",
				Path:   "/test",
			}

			resp, err := middleware.Process(context.Background(), req, handler)
			if err != nil {
				t.Fatalf("Request %d should succeed, got error: %v", i, err)
			}
			if resp.StatusCode != 200 {
				t.Fatalf("Request %d: expected status code 200, got %d", i, resp.StatusCode)
			}
		}

		// Third request should be rate limited
		req := &Request{
			ID:     "test-rl-3",
			Method: "GET",
			Path:   "/test",
		}

		resp, err := middleware.Process(context.Background(), req, handler)
		if err != nil {
			t.Fatalf("Rate limited request returned error: %v", err)
		}
		if resp.StatusCode != 429 {
			t.Fatalf("Expected status code 429 (rate limited), got %d", resp.StatusCode)
		}
	})

	t.Run("Async Processing", func(t *testing.T) {
		middleware := NewResilienceMiddleware("test-async", nil, nil, nil)

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "async success",
				Timestamp:  time.Now(),
			}, nil
		}

		req := &Request{
			ID:     "test-async",
			Method: "GET",
			Path:   "/test",
		}

		resultChan := middleware.ProcessAsync(context.Background(), req, handler)

		select {
		case result := <-resultChan:
			if result.Error != nil {
				t.Fatalf("Expected no error, got %v", result.Error)
			}
			if result.Response.StatusCode != 200 {
				t.Fatalf("Expected status code 200, got %d", result.Response.StatusCode)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Async processing timed out")
		}
	})

	t.Run("Configuration", func(t *testing.T) {
		middleware := NewResilienceMiddleware("test-config", nil, nil, nil)

		config := map[string]interface{}{
			"enabled":  false,
			"priority": 200,
			"retry": map[string]interface{}{
				"max_attempts":     5,
				"initial_delay":    "200ms",
				"max_delay":        "10s",
				"backoff_factor":   3.0,
				"retryable_errors": []interface{}{"TIMEOUT", "CONNECTION_FAILED"},
			},
			"rate_limit": map[string]interface{}{
				"tokens_per_second": 10,
				"bucket_size":       20,
			},
		}

		err := middleware.Configure(config)
		if err != nil {
			t.Fatalf("Configuration failed: %v", err)
		}

		if middleware.Enabled() {
			t.Fatal("Expected middleware to be disabled")
		}

		if middleware.Priority() != 200 {
			t.Fatalf("Expected priority 200, got %d", middleware.Priority())
		}

		if middleware.retryConfig.MaxAttempts != 5 {
			t.Fatalf("Expected max attempts 5, got %d", middleware.retryConfig.MaxAttempts)
		}

		if middleware.retryConfig.InitialDelay != 200*time.Millisecond {
			t.Fatalf("Expected initial delay 200ms, got %v", middleware.retryConfig.InitialDelay)
		}

		if middleware.rateLimiter == nil {
			t.Fatal("Expected rate limiter to be configured")
		}
	})
}

func TestResilienceMiddlewareFactory(t *testing.T) {
	factory := NewResilienceMiddlewareFactory()

	t.Run("Create Resilience Middleware", func(t *testing.T) {
		config := &MiddlewareConfig{
			Name:     "test-factory",
			Type:     "resilience",
			Enabled:  true,
			Priority: 100,
			Config: map[string]interface{}{
				"circuit_breaker": map[string]interface{}{
					"max_failures":        5,
					"reset_timeout":       "30s",
					"half_open_max_calls": 2,
					"success_threshold":   3,
					"timeout":             "15s",
				},
				"retry": map[string]interface{}{
					"max_attempts":     4,
					"initial_delay":    "500ms",
					"max_delay":        "30s",
					"backoff_factor":   2.5,
					"retryable_errors": []interface{}{"TIMEOUT", "SERVICE_UNAVAILABLE"},
				},
				"rate_limit": map[string]interface{}{
					"tokens_per_second": 100,
					"bucket_size":       200,
				},
			},
		}

		middleware, err := factory.Create(config)
		if err != nil {
			t.Fatalf("Factory creation failed: %v", err)
		}

		resilienceMiddleware, ok := middleware.(*ResilienceMiddleware)
		if !ok {
			t.Fatal("Expected ResilienceMiddleware type")
		}

		if resilienceMiddleware.Name() != "test-factory" {
			t.Fatalf("Expected name 'test-factory', got %s", resilienceMiddleware.Name())
		}

		if !resilienceMiddleware.Enabled() {
			t.Fatal("Expected middleware to be enabled")
		}

		if resilienceMiddleware.Priority() != 100 {
			t.Fatalf("Expected priority 100, got %d", resilienceMiddleware.Priority())
		}
	})

	t.Run("Supported Types", func(t *testing.T) {
		supportedTypes := factory.SupportedTypes()
		expectedTypes := []string{"resilience", "circuit_breaker", "retry"}

		if len(supportedTypes) != len(expectedTypes) {
			t.Fatalf("Expected %d supported types, got %d", len(expectedTypes), len(supportedTypes))
		}

		for _, expectedType := range expectedTypes {
			found := false
			for _, supportedType := range supportedTypes {
				if supportedType == expectedType {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("Expected type %s not found in supported types", expectedType)
			}
		}
	})
}

func TestRateLimiter(t *testing.T) {
	t.Run("Token Bucket Algorithm", func(t *testing.T) {
		limiter := NewRateLimiter(2, 5) // 2 tokens per second, bucket size 5

		// Should initially have full bucket
		for i := 0; i < 5; i++ {
			if !limiter.Allow() {
				t.Fatalf("Request %d should be allowed", i)
			}
		}

		// Next request should be denied
		if limiter.Allow() {
			t.Fatal("Request should be denied when bucket is empty")
		}

		// Wait for token refill
		time.Sleep(600 * time.Millisecond) // Should get 1 token

		if !limiter.Allow() {
			t.Fatal("Request should be allowed after token refill")
		}

		// Should be denied again
		if limiter.Allow() {
			t.Fatal("Request should be denied after consuming refilled token")
		}
	})

	t.Run("Concurrent Access", func(t *testing.T) {
		limiter := NewRateLimiter(10, 10)

		var allowed, denied int64
		var wg sync.WaitGroup

		// Launch multiple goroutines
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if limiter.Allow() {
					atomic.AddInt64(&allowed, 1)
				} else {
					atomic.AddInt64(&denied, 1)
				}
			}()
		}

		wg.Wait()

		// Should have allowed around 10 requests and denied the rest
		if atomic.LoadInt64(&allowed) > 10 {
			t.Fatalf("Too many requests allowed: %d", allowed)
		}

		if atomic.LoadInt64(&denied) < 90 {
			t.Fatalf("Too few requests denied: %d", denied)
		}
	})
}

func BenchmarkResilienceMiddleware(b *testing.B) {
	middleware := NewResilienceMiddleware("bench-resilience", nil, nil, nil)

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{
			ID:         req.ID,
			StatusCode: 200,
			Body:       "success",
			Timestamp:  time.Now(),
		}, nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := &Request{
				ID:     fmt.Sprintf("bench-%d", i),
				Method: "GET",
				Path:   "/test",
			}

			middleware.Process(context.Background(), req, handler)
			i++
		}
	})
}

func BenchmarkRateLimiter(b *testing.B) {
	limiter := NewRateLimiter(1000, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}
