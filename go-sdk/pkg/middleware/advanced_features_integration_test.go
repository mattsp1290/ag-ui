package middleware

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// Integration test that verifies all advanced features work together
func TestAdvancedFeaturesIntegration(t *testing.T) {
	t.Run("Complete Integration Test", func(t *testing.T) {
		// Create enhanced middleware manager
		manager := NewEnhancedMiddlewareManager()

		// Create a handler that simulates real processing
		var requestCount int64
		handler := func(ctx context.Context, req *Request) (*Response, error) {
			atomic.AddInt64(&requestCount, 1)

			// Simulate processing time
			time.Sleep(10 * time.Millisecond)

			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       fmt.Sprintf("processed request %s", req.ID),
				Timestamp:  time.Now(),
			}, nil
		}

		// Create async chain
		asyncChain := manager.CreateAsyncChain("integration-async", handler, 5, 2*time.Second)
		if asyncChain == nil {
			t.Fatal("Failed to create async chain")
		}

		// Create dependency-aware chain
		depChain := manager.CreateDependencyAwareChain("integration-dependency", handler)
		if depChain == nil {
			t.Fatal("Failed to create dependency chain")
		}

		// Add resilience middleware
		resilienceMiddleware := NewResilienceMiddleware(
			"integration-resilience",
			&errors.CircuitBreakerConfig{
				Name:             "integration-cb",
				MaxFailures:      3,
				ResetTimeout:     1 * time.Second,
				HalfOpenMaxCalls: 1,
				SuccessThreshold: 1,
				Timeout:          5 * time.Second,
			},
			&RetryConfig{
				MaxAttempts:     2,
				InitialDelay:    50 * time.Millisecond,
				MaxDelay:        500 * time.Millisecond,
				BackoffFactor:   2.0,
				RetryableErrors: []string{"TIMEOUT"},
			},
			NewRateLimiter(10, 10),
		)

		// Add to async chain
		asyncChain.Add(resilienceMiddleware)

		// Add to dependency chain with dependencies
		authMiddleware := &TestMiddleware{name: "auth", enabled: true, priority: 100}
		err := depChain.AddMiddlewareWithDependencies(authMiddleware, []string{}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add auth middleware: %v", err)
		}

		err = depChain.AddMiddlewareWithDependencies(resilienceMiddleware, []string{"auth"}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add resilience middleware: %v", err)
		}

		// Test async processing
		t.Run("Async Processing", func(t *testing.T) {
			req := &Request{
				ID:     "async-integration",
				Method: "GET",
				Path:   "/test",
			}

			resultChan := manager.ProcessAsync(context.Background(), "integration-async", req)

			select {
			case result := <-resultChan:
				if result.Error != nil {
					t.Fatalf("Async processing failed: %v", result.Error)
				}
				if result.Response.StatusCode != 200 {
					t.Fatalf("Expected status code 200, got %d", result.Response.StatusCode)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Async processing timed out")
			}
		})

		// Test dependency processing
		t.Run("Dependency Processing", func(t *testing.T) {
			req := &Request{
				ID:     "dependency-integration",
				Method: "GET",
				Path:   "/test",
			}

			resp, err := manager.ProcessWithDependencies(context.Background(), "integration-dependency", req)
			if err != nil {
				t.Fatalf("Dependency processing failed: %v", err)
			}
			if resp.StatusCode != 200 {
				t.Fatalf("Expected status code 200, got %d", resp.StatusCode)
			}
		})

		// Test batch processing
		t.Run("Batch Processing", func(t *testing.T) {
			requests := make([]*Request, 3)
			for i := 0; i < 3; i++ {
				requests[i] = &Request{
					ID:     fmt.Sprintf("batch-%d", i),
					Method: "GET",
					Path:   "/test",
				}
			}

			results, err := manager.ProcessBatch(context.Background(), "integration-async", requests)
			if err != nil {
				t.Fatalf("Batch processing failed: %v", err)
			}

			if len(results) != 3 {
				t.Fatalf("Expected 3 results, got %d", len(results))
			}

			for i, result := range results {
				if result.Error != nil {
					t.Fatalf("Batch item %d failed: %v", i, result.Error)
				}
				if result.Response.StatusCode != 200 {
					t.Fatalf("Batch item %d: expected status code 200, got %d", i, result.Response.StatusCode)
				}
			}
		})

		// Test performance metrics
		t.Run("Performance Metrics", func(t *testing.T) {
			metrics := manager.GetPerformanceMetrics()

			// Should have some requests processed by now
			if metrics.TotalRequests == 0 {
				t.Fatal("Expected some requests to be processed")
			}

			if metrics.SuccessfulRequests == 0 {
				t.Fatal("Expected some successful requests")
			}
		})

		// Test resilience stats
		t.Run("Resilience Stats", func(t *testing.T) {
			stats := manager.GetResilienceStats()

			// Should have stats from our resilience middleware
			if len(stats) == 0 {
				t.Fatal("Expected resilience statistics")
			}
		})

		// Test health check
		t.Run("Health Check", func(t *testing.T) {
			health := manager.HealthCheck()

			if !health.Healthy {
				t.Fatal("Expected system to be healthy")
			}

			if health.AsyncChains == 0 {
				t.Fatal("Expected async chains to be tracked")
			}

			if health.DependencyChains == 0 {
				t.Fatal("Expected dependency chains to be tracked")
			}
		})

		// Test dependency validation
		t.Run("Dependency Validation", func(t *testing.T) {
			validationErrors := manager.ValidateAllDependencies()

			// Should have no validation errors for our simple setup
			if len(validationErrors) > 0 {
				t.Fatalf("Unexpected validation errors: %v", validationErrors)
			}
		})

		// Test graceful shutdown
		t.Run("Graceful Shutdown", func(t *testing.T) {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := manager.Shutdown(shutdownCtx)
			if err != nil {
				t.Fatalf("Shutdown failed: %v", err)
			}
		})
	})
}

// Test that verifies async middleware wrapper works correctly
func TestAsyncMiddlewareWrapper(t *testing.T) {
	// Create a simple synchronous middleware
	syncMiddleware := &TestMiddleware{
		name:     "sync-test",
		enabled:  true,
		priority: 100,
	}

	// Wrap it for async processing
	asyncWrapper := NewAsyncMiddlewareWrapper(syncMiddleware)

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{
			ID:         req.ID,
			StatusCode: 200,
			Body:       "async wrapped",
			Timestamp:  time.Now(),
		}, nil
	}

	req := &Request{
		ID:     "wrapper-test",
		Method: "GET",
		Path:   "/test",
	}

	resultChan := asyncWrapper.ProcessAsync(context.Background(), req, handler)

	select {
	case result := <-resultChan:
		if result.Error != nil {
			t.Fatalf("Async wrapper failed: %v", result.Error)
		}
		if result.Response.StatusCode != 200 {
			t.Fatalf("Expected status code 200, got %d", result.Response.StatusCode)
		}

		// Verify that the original middleware was processed
		if syncMiddleware.GetProcessed() != 1 {
			t.Fatalf("Expected middleware to be processed once, got %d", syncMiddleware.GetProcessed())
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Async wrapper timed out")
	}
}

// Test rate limiter functionality
func TestRateLimiterIntegration(t *testing.T) {
	rateLimiter := NewRateLimiter(2, 2) // 2 tokens per second, bucket size 2

	// First two requests should succeed
	if !rateLimiter.Allow() {
		t.Fatal("First request should be allowed")
	}
	if !rateLimiter.Allow() {
		t.Fatal("Second request should be allowed")
	}

	// Third request should be denied (bucket empty)
	if rateLimiter.Allow() {
		t.Fatal("Third request should be denied")
	}

	// Wait for token refill and try again
	time.Sleep(600 * time.Millisecond) // Should get 1 token

	if !rateLimiter.Allow() {
		t.Fatal("Request after refill should be allowed")
	}

	// Should be denied again
	if rateLimiter.Allow() {
		t.Fatal("Request should be denied after consuming refilled token")
	}
}

// Test dependency condition functionality
func TestDependencyConditions(t *testing.T) {
	t.Run("Always Condition", func(t *testing.T) {
		condition := &AlwaysDependencyCondition{}
		req := &Request{ID: "test", Path: "/any"}

		if !condition.ShouldApply(context.Background(), req) {
			t.Fatal("Always condition should always return true")
		}
	})

	t.Run("Path-Based Condition", func(t *testing.T) {
		condition := &PathBasedDependencyCondition{
			PathPatterns: []string{"/admin/*", "/api/v1/*"},
		}

		// Test matching paths
		testCases := []struct {
			path     string
			expected bool
		}{
			{"/admin/users", true},
			{"/admin/settings", true},
			{"/api/v1/data", true},
			{"/public/info", false},
			{"/api/v2/data", false},
		}

		for _, tc := range testCases {
			req := &Request{ID: "test", Path: tc.path}
			result := condition.ShouldApply(context.Background(), req)
			if result != tc.expected {
				t.Fatalf("Path %s: expected %v, got %v", tc.path, tc.expected, result)
			}
		}
	})

	t.Run("Conditional Function", func(t *testing.T) {
		condition := &ConditionalDependencyCondition{
			Condition: func(ctx context.Context, req *Request) bool {
				return req.Method == "POST"
			},
		}

		// Test POST request
		postReq := &Request{ID: "test", Method: "POST", Path: "/test"}
		if !condition.ShouldApply(context.Background(), postReq) {
			t.Fatal("Condition should apply for POST request")
		}

		// Test GET request
		getReq := &Request{ID: "test", Method: "GET", Path: "/test"}
		if condition.ShouldApply(context.Background(), getReq) {
			t.Fatal("Condition should not apply for GET request")
		}
	})
}
