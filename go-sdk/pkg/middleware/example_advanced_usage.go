package middleware

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// ExampleAdvancedMiddlewareUsage demonstrates how to use all the advanced middleware features
func ExampleAdvancedMiddlewareUsage() {
	fmt.Println("=== Advanced Middleware Features Example ===")

	// Create an enhanced middleware manager
	manager := NewEnhancedMiddlewareManager()

	// Define a sample handler
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		// Simulate processing time
		time.Sleep(10 * time.Millisecond)

		return &Response{
			ID:         req.ID,
			StatusCode: 200,
			Body:       fmt.Sprintf("Processed request: %s", req.Path),
			Timestamp:  time.Now(),
		}, nil
	}

	// 1. Create an Async Chain for high-throughput scenarios
	fmt.Println("\n1. Creating Async Chain...")
	asyncChain := manager.CreateAsyncChain(
		"high-throughput",
		handler,
		20,             // Max 20 concurrent requests
		30*time.Second, // 30 second timeout
	)

	// Add resilience middleware to async chain
	resilienceConfig := &errors.CircuitBreakerConfig{
		Name:             "api-circuit-breaker",
		MaxFailures:      5,
		ResetTimeout:     60 * time.Second,
		HalfOpenMaxCalls: 3,
		SuccessThreshold: 2,
		Timeout:          10 * time.Second,
	}

	retryConfig := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"TIMEOUT",
			"CONNECTION_FAILED",
			"SERVICE_UNAVAILABLE",
		},
	}

	rateLimiter := NewRateLimiter(100, 200) // 100 requests/sec, bucket size 200

	resilienceMiddleware := NewResilienceMiddleware(
		"api-resilience",
		resilienceConfig,
		retryConfig,
		rateLimiter,
	)

	asyncChain.Add(resilienceMiddleware)

	// 2. Create a Dependency-Aware Chain for complex processing
	fmt.Println("2. Creating Dependency-Aware Chain...")
	depChain := manager.CreateDependencyAwareChain("complex-processing", handler)

	// Create middleware with dependencies
	authMiddleware := &ExampleAuthMiddleware{name: "auth", enabled: true, priority: 100}
	loggingMiddleware := &ExampleLoggingMiddleware{name: "logging", enabled: true, priority: 90}
	metricsMiddleware := &ExampleMetricsMiddleware{name: "metrics", enabled: true, priority: 80}
	validationMiddleware := &ExampleValidationMiddleware{name: "validation", enabled: true, priority: 85}

	// Set up dependencies:
	// - logging depends on auth (need user context for logging)
	// - metrics depends on logging (need request info)
	// - validation depends on auth (need to validate permissions)
	depChain.AddMiddlewareWithDependencies(authMiddleware, []string{}, false, &AlwaysDependencyCondition{})
	depChain.AddMiddlewareWithDependencies(loggingMiddleware, []string{"auth"}, false, &AlwaysDependencyCondition{})
	depChain.AddMiddlewareWithDependencies(validationMiddleware, []string{"auth"}, false, &AlwaysDependencyCondition{})
	depChain.AddMiddlewareWithDependencies(metricsMiddleware, []string{"logging"}, false, &AlwaysDependencyCondition{})

	// 3. Create Admin Chain with Path-Based Dependencies
	fmt.Println("3. Creating Admin Chain with Conditional Dependencies...")
	adminChain := manager.CreateDependencyAwareChain("admin-processing", handler)

	adminAuthMiddleware := &ExampleAuthMiddleware{name: "admin-auth", enabled: true, priority: 100}
	adminOnlyMiddleware := &ExampleAdminOnlyMiddleware{name: "admin-only", enabled: true, priority: 90}

	// Admin middleware only applies to admin paths
	adminPathCondition := &PathBasedDependencyCondition{
		PathPatterns: []string{"/admin/*", "/management/*"},
	}

	adminChain.AddMiddlewareWithDependencies(adminAuthMiddleware, []string{}, false, &AlwaysDependencyCondition{})
	adminChain.AddMiddlewareWithDependencies(adminOnlyMiddleware, []string{"admin-auth"}, false, adminPathCondition)

	// 4. Demonstrate Processing Different Types of Requests
	fmt.Println("\n4. Processing Requests...")

	// Async processing example
	fmt.Println("   a) Async Processing:")
	asyncReq := &Request{
		ID:     "async-001",
		Method: "GET",
		Path:   "/api/data",
		Headers: map[string]string{
			"Authorization": "Bearer token123",
		},
	}

	resultChan := manager.ProcessAsync(context.Background(), "high-throughput", asyncReq)
	select {
	case result := <-resultChan:
		if result.Error != nil {
			fmt.Printf("      Error: %v\n", result.Error)
		} else {
			fmt.Printf("      Success: %s (Status: %d)\n", result.Response.Body, result.Response.StatusCode)
		}
	case <-time.After(5 * time.Second):
		fmt.Println("      Timeout waiting for async result")
	}

	// Dependency-aware processing example
	fmt.Println("   b) Dependency-Aware Processing:")
	depReq := &Request{
		ID:     "dep-001",
		Method: "POST",
		Path:   "/api/users",
		Headers: map[string]string{
			"Authorization": "Bearer token123",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
		},
	}

	resp, err := manager.ProcessWithDependencies(context.Background(), "complex-processing", depReq)
	if err != nil {
		fmt.Printf("      Error: %v\n", err)
	} else {
		fmt.Printf("      Success: %s (Status: %d)\n", resp.Body, resp.StatusCode)
	}

	// Admin processing example
	fmt.Println("   c) Admin Processing (public path):")
	publicReq := &Request{
		ID:     "admin-001",
		Method: "GET",
		Path:   "/public/info",
	}

	resp, err = manager.ProcessWithDependencies(context.Background(), "admin-processing", publicReq)
	if err != nil {
		fmt.Printf("      Error: %v\n", err)
	} else {
		fmt.Printf("      Success: %s (Status: %d)\n", resp.Body, resp.StatusCode)
	}

	fmt.Println("   d) Admin Processing (admin path):")
	adminReq := &Request{
		ID:     "admin-002",
		Method: "GET",
		Path:   "/admin/users",
		Headers: map[string]string{
			"Authorization": "Bearer admin-token",
		},
	}

	resp, err = manager.ProcessWithDependencies(context.Background(), "admin-processing", adminReq)
	if err != nil {
		fmt.Printf("      Error: %v\n", err)
	} else {
		fmt.Printf("      Success: %s (Status: %d)\n", resp.Body, resp.StatusCode)
	}

	// Batch processing example
	fmt.Println("   e) Batch Processing:")
	batchReqs := []*Request{
		{ID: "batch-1", Method: "GET", Path: "/api/item/1"},
		{ID: "batch-2", Method: "GET", Path: "/api/item/2"},
		{ID: "batch-3", Method: "GET", Path: "/api/item/3"},
	}

	batchResults, err := manager.ProcessBatch(context.Background(), "high-throughput", batchReqs)
	if err != nil {
		fmt.Printf("      Batch Error: %v\n", err)
	} else {
		fmt.Printf("      Batch completed: %d requests processed\n", len(batchResults))
		for i, result := range batchResults {
			if result.Error != nil {
				fmt.Printf("        Request %d failed: %v\n", i+1, result.Error)
			} else {
				fmt.Printf("        Request %d success: Status %d\n", i+1, result.Response.StatusCode)
			}
		}
	}

	// 5. Display System Statistics
	fmt.Println("\n5. System Statistics:")

	// Performance metrics
	perfMetrics := manager.GetPerformanceMetrics()
	fmt.Printf("   Performance: %d total, %d successful, %d failed\n",
		perfMetrics.TotalRequests, perfMetrics.SuccessfulRequests, perfMetrics.FailedRequests)
	fmt.Printf("   Average latency: %v\n", perfMetrics.AverageLatency)

	// Resilience statistics
	resilienceStats := manager.GetResilienceStats()
	fmt.Printf("   Resilience Middleware Count: %d\n", len(resilienceStats))
	for name, stats := range resilienceStats {
		fmt.Printf("     %s: State=%s, Requests=%d, Failures=%d\n",
			name, stats.CircuitBreakerState, stats.TotalRequests, stats.FailedRequests)
	}

	// Async statistics
	asyncStats := manager.GetAsyncStats()
	fmt.Printf("   Async Chains: %d\n", len(asyncStats))
	for name, stats := range asyncStats {
		fmt.Printf("     %s: Max Concurrency=%d, Active=%d, Timeout=%v\n",
			name, stats.MaxConcurrency, stats.CurrentActive, stats.Timeout)
	}

	// Dependency report
	depReport := manager.GetDependencyReport()
	fmt.Printf("   Dependency Chains: %d\n", len(depReport.Chains))
	for chainName, chainInfo := range depReport.Chains {
		fmt.Printf("     %s: %d middleware\n", chainName, chainInfo.MiddlewareCount)
		for mwName, deps := range chainInfo.Dependencies {
			if len(deps) > 0 {
				fmt.Printf("       %s depends on: %v\n", mwName, deps)
			}
		}
	}

	// Health check
	health := manager.HealthCheck()
	fmt.Printf("   System Health: %v\n", health.Healthy)
	fmt.Printf("   Chains: %d regular, %d async, %d dependency-aware\n",
		health.RegularChains, health.AsyncChains, health.DependencyChains)

	// 6. Graceful Shutdown
	fmt.Println("\n6. Graceful Shutdown...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = manager.Shutdown(shutdownCtx)
	if err != nil {
		fmt.Printf("   Shutdown error: %v\n", err)
	} else {
		fmt.Println("   Shutdown completed successfully")
	}

	fmt.Println("\n=== Advanced Middleware Features Example Complete ===")
}

// Example middleware implementations for demonstration

type ExampleAuthMiddleware struct {
	name     string
	enabled  bool
	priority int
}

func (m *ExampleAuthMiddleware) Name() string                                  { return m.name }
func (m *ExampleAuthMiddleware) Enabled() bool                                 { return m.enabled }
func (m *ExampleAuthMiddleware) Priority() int                                 { return m.priority }
func (m *ExampleAuthMiddleware) Configure(config map[string]interface{}) error { return nil }

func (m *ExampleAuthMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Simulate authentication
	if req.Headers["Authorization"] == "" {
		return &Response{
			ID:         req.ID,
			StatusCode: 401,
			Body:       "Unauthorized",
			Timestamp:  time.Now(),
		}, nil
	}

	// Add user context
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}
	req.Metadata["user_id"] = "user123"
	req.Metadata["authenticated"] = true

	return next(ctx, req)
}

type ExampleLoggingMiddleware struct {
	name     string
	enabled  bool
	priority int
}

func (m *ExampleLoggingMiddleware) Name() string                                  { return m.name }
func (m *ExampleLoggingMiddleware) Enabled() bool                                 { return m.enabled }
func (m *ExampleLoggingMiddleware) Priority() int                                 { return m.priority }
func (m *ExampleLoggingMiddleware) Configure(config map[string]interface{}) error { return nil }

func (m *ExampleLoggingMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	start := time.Now()

	userID := "anonymous"
	if req.Metadata != nil && req.Metadata["user_id"] != nil {
		userID = req.Metadata["user_id"].(string)
	}

	log.Printf("Request started: %s %s (User: %s)", req.Method, req.Path, userID)

	resp, err := next(ctx, req)

	duration := time.Since(start)
	status := 500
	if err == nil && resp != nil {
		status = resp.StatusCode
	}

	log.Printf("Request completed: %s %s -> %d (%v)", req.Method, req.Path, status, duration)

	return resp, err
}

type ExampleMetricsMiddleware struct {
	name     string
	enabled  bool
	priority int
}

func (m *ExampleMetricsMiddleware) Name() string                                  { return m.name }
func (m *ExampleMetricsMiddleware) Enabled() bool                                 { return m.enabled }
func (m *ExampleMetricsMiddleware) Priority() int                                 { return m.priority }
func (m *ExampleMetricsMiddleware) Configure(config map[string]interface{}) error { return nil }

func (m *ExampleMetricsMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Simulate metrics collection
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}
	req.Metadata["metrics_collected"] = true
	req.Metadata["metric_timestamp"] = time.Now()

	return next(ctx, req)
}

type ExampleValidationMiddleware struct {
	name     string
	enabled  bool
	priority int
}

func (m *ExampleValidationMiddleware) Name() string                                  { return m.name }
func (m *ExampleValidationMiddleware) Enabled() bool                                 { return m.enabled }
func (m *ExampleValidationMiddleware) Priority() int                                 { return m.priority }
func (m *ExampleValidationMiddleware) Configure(config map[string]interface{}) error { return nil }

func (m *ExampleValidationMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Simulate validation based on user permissions
	if req.Method == "POST" || req.Method == "PUT" || req.Method == "DELETE" {
		if req.Metadata == nil || req.Metadata["authenticated"] != true {
			return &Response{
				ID:         req.ID,
				StatusCode: 403,
				Body:       "Forbidden - Authentication required for modifying operations",
				Timestamp:  time.Now(),
			}, nil
		}
	}

	return next(ctx, req)
}

type ExampleAdminOnlyMiddleware struct {
	name     string
	enabled  bool
	priority int
}

func (m *ExampleAdminOnlyMiddleware) Name() string                                  { return m.name }
func (m *ExampleAdminOnlyMiddleware) Enabled() bool                                 { return m.enabled }
func (m *ExampleAdminOnlyMiddleware) Priority() int                                 { return m.priority }
func (m *ExampleAdminOnlyMiddleware) Configure(config map[string]interface{}) error { return nil }

func (m *ExampleAdminOnlyMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Check for admin token
	authHeader := req.Headers["Authorization"]
	if authHeader != "Bearer admin-token" {
		return &Response{
			ID:         req.ID,
			StatusCode: 403,
			Body:       "Forbidden - Admin access required",
			Timestamp:  time.Now(),
		}, nil
	}

	// Add admin context
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}
	req.Metadata["admin"] = true

	return next(ctx, req)
}
