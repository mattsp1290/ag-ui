package middleware

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestEnhancedMiddlewareManager(t *testing.T) {
	t.Run("Enhanced Manager Creation", func(t *testing.T) {
		manager := NewEnhancedMiddlewareManager()

		if manager.MiddlewareManager == nil {
			t.Fatal("Base middleware manager not initialized")
		}

		if manager.dependencyManager == nil {
			t.Fatal("Dependency manager not initialized")
		}

		if manager.performanceMetrics == nil {
			t.Fatal("Performance metrics not initialized")
		}
	})

	t.Run("Async Chain Management", func(t *testing.T) {
		manager := NewEnhancedMiddlewareManager()

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "async processed",
				Timestamp:  time.Now(),
			}, nil
		}

		// Create async chain
		chain := manager.CreateAsyncChain("test-async", handler, 10, 5*time.Second)
		if chain == nil {
			t.Fatal("Failed to create async chain")
		}

		// Retrieve async chain
		retrievedChain := manager.GetAsyncChain("test-async")
		if retrievedChain != chain {
			t.Fatal("Failed to retrieve async chain")
		}

		// Test async processing
		req := &Request{
			ID:     "async-test",
			Method: "GET",
			Path:   "/test",
		}

		resultChan := manager.ProcessAsync(context.Background(), "test-async", req)

		select {
		case result := <-resultChan:
			if result.Error != nil {
				t.Fatalf("Async processing failed: %v", result.Error)
			}
			if result.Response.StatusCode != 200 {
				t.Fatalf("Expected status code 200, got %d", result.Response.StatusCode)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Async processing timed out")
		}
	})

	t.Run("Dependency-Aware Chain Management", func(t *testing.T) {
		manager := NewEnhancedMiddlewareManager()

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "dependency processed",
				Timestamp:  time.Now(),
			}, nil
		}

		// Create dependency-aware chain
		chain := manager.CreateDependencyAwareChain("test-dependency", handler)
		if chain == nil {
			t.Fatal("Failed to create dependency-aware chain")
		}

		// Add middleware with dependencies
		auth := &TestMiddleware{name: "auth", enabled: true, priority: 100}
		logging := &TestMiddleware{name: "logging", enabled: true, priority: 90}

		err := chain.AddMiddlewareWithDependencies(auth, []string{}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add auth middleware: %v", err)
		}

		err = chain.AddMiddlewareWithDependencies(logging, []string{"auth"}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add logging middleware: %v", err)
		}

		// Test processing with dependencies
		req := &Request{
			ID:     "dependency-test",
			Method: "GET",
			Path:   "/test",
		}

		resp, err := manager.ProcessWithDependencies(context.Background(), "test-dependency", req)
		if err != nil {
			t.Fatalf("Dependency processing failed: %v", err)
		}

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status code 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Batch Processing", func(t *testing.T) {
		manager := NewEnhancedMiddlewareManager()

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       fmt.Sprintf("batch processed %s", req.ID),
				Timestamp:  time.Now(),
			}, nil
		}

		// Create async chain for batch processing
		manager.CreateAsyncChain("batch-test", handler, 20, 10*time.Second)

		// Create batch requests
		requests := make([]*Request, 5)
		for i := 0; i < 5; i++ {
			requests[i] = &Request{
				ID:     fmt.Sprintf("batch-%d", i),
				Method: "GET",
				Path:   "/test",
			}
		}

		// Process batch
		results, err := manager.ProcessBatch(context.Background(), "batch-test", requests)
		if err != nil {
			t.Fatalf("Batch processing failed: %v", err)
		}

		if len(results) != len(requests) {
			t.Fatalf("Expected %d results, got %d", len(requests), len(results))
		}

		// Verify all results
		for i, result := range results {
			if result.Error != nil {
				t.Fatalf("Batch item %d failed: %v", i, result.Error)
			}
			if result.Response.StatusCode != 200 {
				t.Fatalf("Batch item %d: expected status code 200, got %d", i, result.Response.StatusCode)
			}
		}
	})

	t.Run("Performance Metrics", func(t *testing.T) {
		manager := NewEnhancedMiddlewareManager()

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "processed",
				Timestamp:  time.Now(),
			}, nil
		}

		_ = manager.CreateDependencyAwareChain("metrics-test", handler)

		// Process several requests
		for i := 0; i < 10; i++ {
			req := &Request{
				ID:     fmt.Sprintf("metrics-%d", i),
				Method: "GET",
				Path:   "/test",
			}

			_, err := manager.ProcessWithDependencies(context.Background(), "metrics-test", req)
			if err != nil {
				t.Fatalf("Request %d failed: %v", i, err)
			}
		}

		// Check performance metrics
		metrics := manager.GetPerformanceMetrics()

		if metrics.TotalRequests != 10 {
			t.Fatalf("Expected 10 total requests, got %d", metrics.TotalRequests)
		}

		if metrics.SuccessfulRequests != 10 {
			t.Fatalf("Expected 10 successful requests, got %d", metrics.SuccessfulRequests)
		}

		if metrics.FailedRequests != 0 {
			t.Fatalf("Expected 0 failed requests, got %d", metrics.FailedRequests)
		}

		if metrics.AverageLatency < 10*time.Millisecond {
			t.Fatalf("Expected average latency >= 10ms, got %v", metrics.AverageLatency)
		}
	})

	t.Run("Resilience Integration", func(t *testing.T) {
		manager := NewEnhancedMiddlewareManager()

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "processed",
				Timestamp:  time.Now(),
			}, nil
		}

		// Create chain
		manager.CreateChain("resilience-test", handler)

		// Add resilience middleware
		resilienceMiddleware := NewResilienceMiddleware("test-resilience", nil, nil, nil)
		err := manager.AddResilienceMiddleware("resilience-test", resilienceMiddleware)
		if err != nil {
			t.Fatalf("Failed to add resilience middleware: %v", err)
		}

		// Get resilience stats
		stats := manager.GetResilienceStats()
		if len(stats) == 0 {
			t.Fatal("Expected resilience stats, got none")
		}

		// Verify stats contain our middleware
		found := false
		for key := range stats {
			if key == "resilience-test:test-resilience" {
				found = true
				break
			}
		}

		if !found {
			t.Fatal("Expected to find resilience middleware in stats")
		}
	})

	t.Run("Health Check", func(t *testing.T) {
		manager := NewEnhancedMiddlewareManager()

		// Create various types of chains
		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{ID: req.ID, StatusCode: 200, Body: "OK"}, nil
		}

		manager.CreateChain("regular", handler)
		manager.CreateAsyncChain("async", handler, 10, 5*time.Second)
		manager.CreateDependencyAwareChain("dependency", handler)

		// Add some middleware
		auth := &TestMiddleware{name: "auth", enabled: true, priority: 100}
		manager.GetChain("regular").Add(auth)

		// Get health status
		health := manager.HealthCheck()

		if !health.Healthy {
			t.Fatal("Expected healthy status")
		}

		if health.RegularChains != 2 { // regular + default
			t.Fatalf("Expected 2 regular chains, got %d", health.RegularChains)
		}

		if health.AsyncChains != 1 {
			t.Fatalf("Expected 1 async chain, got %d", health.AsyncChains)
		}

		if health.DependencyChains != 1 {
			t.Fatalf("Expected 1 dependency chain, got %d", health.DependencyChains)
		}

		if health.ActiveMiddleware < 1 {
			t.Fatalf("Expected at least 1 active middleware, got %d", health.ActiveMiddleware)
		}
	})

	t.Run("Async Stats", func(t *testing.T) {
		manager := NewEnhancedMiddlewareManager()

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{ID: req.ID, StatusCode: 200, Body: "processed"}, nil
		}

		// Create async chains with different configurations
		manager.CreateAsyncChain("async1", handler, 5, 10*time.Second)
		manager.CreateAsyncChain("async2", handler, 10, 20*time.Second)

		stats := manager.GetAsyncStats()

		if len(stats) != 2 {
			t.Fatalf("Expected 2 async chain stats, got %d", len(stats))
		}

		if stats["async1"].MaxConcurrency != 5 {
			t.Fatalf("Expected async1 max concurrency 5, got %d", stats["async1"].MaxConcurrency)
		}

		if stats["async2"].MaxConcurrency != 10 {
			t.Fatalf("Expected async2 max concurrency 10, got %d", stats["async2"].MaxConcurrency)
		}

		if stats["async1"].Timeout != 10*time.Second {
			t.Fatalf("Expected async1 timeout 10s, got %v", stats["async1"].Timeout)
		}

		if stats["async2"].Timeout != 20*time.Second {
			t.Fatalf("Expected async2 timeout 20s, got %v", stats["async2"].Timeout)
		}
	})

	t.Run("Enhanced Configuration", func(t *testing.T) {
		manager := NewEnhancedMiddlewareManager()

		config := &EnhancedMiddlewareConfiguration{
			MiddlewareConfiguration: &MiddlewareConfiguration{
				DefaultChain: "enhanced",
				Chains: []ChainConfiguration{
					{
						Name:    "enhanced",
						Enabled: true,
						Handler: HandlerConfiguration{
							Type: "echo",
						},
						Middleware: []MiddlewareConfig{},
					},
				},
			},
			AsyncChains: []AsyncChainConfiguration{
				{
					Name:           "async-enhanced",
					MaxConcurrency: 15,
					Timeout:        "30s",
					Handler: HandlerConfiguration{
						Type: "status",
						Config: map[string]interface{}{
							"status_code": 201,
							"message":     "async created",
						},
					},
					Middleware: []MiddlewareConfig{},
				},
			},
			DependencyChains: []DependencyChainConfiguration{
				{
					Name: "dependency-enhanced",
					Handler: HandlerConfiguration{
						Type: "echo",
					},
					Middleware: []DependencyMiddlewareConfig{},
				},
			},
		}

		err := manager.ApplyEnhancedConfiguration(config)
		if err != nil {
			t.Fatalf("Failed to apply enhanced configuration: %v", err)
		}

		// Verify async chain was created
		asyncChain := manager.GetAsyncChain("async-enhanced")
		if asyncChain == nil {
			t.Fatal("Expected async chain to be created from configuration")
		}

		stats := asyncChain.GetConcurrencyStats()
		if stats.MaxConcurrency != 15 {
			t.Fatalf("Expected max concurrency 15, got %d", stats.MaxConcurrency)
		}

		if stats.Timeout != 30*time.Second {
			t.Fatalf("Expected timeout 30s, got %v", stats.Timeout)
		}

		// Verify dependency chain was created
		depChain := manager.GetDependencyAwareChain("dependency-enhanced")
		if depChain == nil {
			t.Fatal("Expected dependency chain to be created from configuration")
		}
	})

	t.Run("Graceful Shutdown", func(t *testing.T) {
		manager := NewEnhancedMiddlewareManager()

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			time.Sleep(100 * time.Millisecond) // Simulate work
			return &Response{ID: req.ID, StatusCode: 200, Body: "processed"}, nil
		}

		// Create chains with ongoing work
		manager.CreateAsyncChain("shutdown-test", handler, 5, 10*time.Second)

		// Start some async work
		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				req := &Request{
					ID:     fmt.Sprintf("shutdown-%d", id),
					Method: "GET",
					Path:   "/test",
				}
				resultChan := manager.ProcessAsync(context.Background(), "shutdown-test", req)
				<-resultChan
			}(i)
		}

		// Give work time to start
		time.Sleep(50 * time.Millisecond)

		// Shutdown with reasonable timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := manager.Shutdown(shutdownCtx)
		if err != nil {
			t.Fatalf("Shutdown failed: %v", err)
		}

		// Wait for work to complete
		wg.Wait()
	})
}

func BenchmarkEnhancedMiddlewareManager(b *testing.B) {
	b.Run("Regular Processing", func(b *testing.B) {
		manager := NewEnhancedMiddlewareManager()

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{ID: req.ID, StatusCode: 200, Body: "processed"}, nil
		}

		chain := manager.CreateChain("bench-regular", handler)
		auth := &TestMiddleware{name: "auth", enabled: true, priority: 100}
		chain.Add(auth)

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				req := &Request{
					ID:     fmt.Sprintf("bench-%d", i),
					Method: "GET",
					Path:   "/test",
				}

				chain.Process(context.Background(), req)
				i++
			}
		})
	})

	b.Run("Async Processing", func(b *testing.B) {
		manager := NewEnhancedMiddlewareManager()

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{ID: req.ID, StatusCode: 200, Body: "processed"}, nil
		}

		manager.CreateAsyncChain("bench-async", handler, 100, 30*time.Second)

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				req := &Request{
					ID:     fmt.Sprintf("bench-async-%d", i),
					Method: "GET",
					Path:   "/test",
				}

				resultChan := manager.ProcessAsync(context.Background(), "bench-async", req)
				<-resultChan
				i++
			}
		})
	})

	b.Run("Dependency Processing", func(b *testing.B) {
		manager := NewEnhancedMiddlewareManager()

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{ID: req.ID, StatusCode: 200, Body: "processed"}, nil
		}

		chain := manager.CreateDependencyAwareChain("bench-dependency", handler)

		// Add middleware with dependencies
		auth := &TestMiddleware{name: "auth", enabled: true, priority: 100}
		logging := &TestMiddleware{name: "logging", enabled: true, priority: 90}

		chain.AddMiddlewareWithDependencies(auth, []string{}, false, &AlwaysDependencyCondition{})
		chain.AddMiddlewareWithDependencies(logging, []string{"auth"}, false, &AlwaysDependencyCondition{})

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				req := &Request{
					ID:     fmt.Sprintf("bench-dep-%d", i),
					Method: "GET",
					Path:   "/test",
				}

				manager.ProcessWithDependencies(context.Background(), "bench-dependency", req)
				i++
			}
		})
	})
}
