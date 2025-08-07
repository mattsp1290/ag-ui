package middleware

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDependencyGraph(t *testing.T) {
	t.Run("Basic Dependency Resolution", func(t *testing.T) {
		graph := NewDependencyGraph()

		// Create test middleware
		authMiddleware := &TestMiddleware{name: "auth", enabled: true, priority: 100}
		loggingMiddleware := &TestMiddleware{name: "logging", enabled: true, priority: 90}
		metricsMiddleware := &TestMiddleware{name: "metrics", enabled: true, priority: 80}

		// Add middleware with dependencies
		// metrics depends on logging, logging depends on auth
		err := graph.AddMiddleware(authMiddleware, []string{}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add auth middleware: %v", err)
		}

		err = graph.AddMiddleware(loggingMiddleware, []string{"auth"}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add logging middleware: %v", err)
		}

		err = graph.AddMiddleware(metricsMiddleware, []string{"logging"}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add metrics middleware: %v", err)
		}

		// Resolve order
		req := &Request{ID: "test", Path: "/test"}
		order, err := graph.ResolveOrder(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to resolve dependency order: %v", err)
		}

		// Should be in dependency order: auth -> logging -> metrics
		if len(order) != 3 {
			t.Fatalf("Expected 3 middleware, got %d", len(order))
		}

		if order[0].Name() != "auth" {
			t.Fatalf("Expected auth first, got %s", order[0].Name())
		}

		if order[1].Name() != "logging" {
			t.Fatalf("Expected logging second, got %s", order[1].Name())
		}

		if order[2].Name() != "metrics" {
			t.Fatalf("Expected metrics third, got %s", order[2].Name())
		}
	})

	t.Run("Circular Dependency Detection", func(t *testing.T) {
		graph := NewDependencyGraph()

		middleware1 := &TestMiddleware{name: "middleware1", enabled: true, priority: 100}
		middleware2 := &TestMiddleware{name: "middleware2", enabled: true, priority: 90}

		// Create circular dependency
		err := graph.AddMiddleware(middleware1, []string{"middleware2"}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add middleware1: %v", err)
		}

		err = graph.AddMiddleware(middleware2, []string{"middleware1"}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add middleware2: %v", err)
		}

		// Should detect circular dependency
		req := &Request{ID: "test", Path: "/test"}
		_, err = graph.ResolveOrder(context.Background(), req)
		if err == nil {
			t.Fatal("Expected circular dependency error, got nil")
		}

		if !strings.Contains(err.Error(), "circular dependency") {
			t.Fatalf("Expected circular dependency error, got: %v", err)
		}
	})

	t.Run("Optional Dependencies", func(t *testing.T) {
		graph := NewDependencyGraph()

		coreMiddleware := &TestMiddleware{name: "core", enabled: true, priority: 100}
		optionalMiddleware := &TestMiddleware{name: "optional", enabled: true, priority: 90}

		// Add core middleware
		err := graph.AddMiddleware(coreMiddleware, []string{}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add core middleware: %v", err)
		}

		// Add optional middleware that depends on non-existent middleware
		err = graph.AddMiddleware(optionalMiddleware, []string{"nonexistent"}, true, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add optional middleware: %v", err)
		}

		// Should resolve without error despite missing dependency
		req := &Request{ID: "test", Path: "/test"}
		order, err := graph.ResolveOrder(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to resolve order with optional dependency: %v", err)
		}

		// Should include both middleware
		if len(order) != 2 {
			t.Fatalf("Expected 2 middleware, got %d", len(order))
		}
	})

	t.Run("Conditional Dependencies", func(t *testing.T) {
		graph := NewDependencyGraph()

		authMiddleware := &TestMiddleware{name: "auth", enabled: true, priority: 100}
		adminMiddleware := &TestMiddleware{name: "admin", enabled: true, priority: 90}

		// Add auth middleware
		err := graph.AddMiddleware(authMiddleware, []string{}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add auth middleware: %v", err)
		}

		// Add admin middleware with path-based condition
		condition := &PathBasedDependencyCondition{
			PathPatterns: []string{"/admin/*", "/admin"},
		}
		err = graph.AddMiddleware(adminMiddleware, []string{"auth"}, false, condition)
		if err != nil {
			t.Fatalf("Failed to add admin middleware: %v", err)
		}

		// Test with non-admin path
		req := &Request{ID: "test1", Path: "/public"}
		order, err := graph.ResolveOrder(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to resolve order for public path: %v", err)
		}

		// Should only include auth middleware
		if len(order) != 1 || order[0].Name() != "auth" {
			t.Fatalf("Expected only auth middleware for public path, got %d middleware", len(order))
		}

		// Test with admin path
		req = &Request{ID: "test2", Path: "/admin/users"}
		order, err = graph.ResolveOrder(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to resolve order for admin path: %v", err)
		}

		// Should include both middleware
		if len(order) != 2 {
			t.Fatalf("Expected 2 middleware for admin path, got %d", len(order))
		}

		if order[0].Name() != "auth" || order[1].Name() != "admin" {
			t.Fatalf("Expected auth -> admin order, got %s -> %s", order[0].Name(), order[1].Name())
		}
	})

	t.Run("Graph Validation", func(t *testing.T) {
		graph := NewDependencyGraph()

		middleware1 := &TestMiddleware{name: "middleware1", enabled: true, priority: 100}

		// Add middleware with missing required dependency
		err := graph.AddMiddleware(middleware1, []string{"nonexistent"}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add middleware1: %v", err)
		}

		// Validation should detect missing dependency
		errors := graph.ValidateGraph()
		if len(errors) == 0 {
			t.Fatal("Expected validation errors, got none")
		}

		foundMissingDepError := false
		for _, err := range errors {
			if strings.Contains(err.Error(), "missing required dependency") {
				foundMissingDepError = true
				break
			}
		}

		if !foundMissingDepError {
			t.Fatal("Expected missing dependency error in validation")
		}
	})
}

func TestDependencyAwareMiddlewareChain(t *testing.T) {
	t.Run("Dependency-Based Execution", func(t *testing.T) {
		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       "processed",
				Timestamp:  time.Now(),
			}, nil
		}

		chain := NewDependencyAwareMiddlewareChain(handler)

		// Create middleware that track execution order
		var executionOrder []string
		var mu sync.Mutex

		createTrackingMiddleware := func(name string) Middleware {
			return &TrackingMiddleware{
				name:     name,
				enabled:  true,
				priority: 100,
				onProcess: func() {
					mu.Lock()
					executionOrder = append(executionOrder, name)
					mu.Unlock()
				},
			}
		}

		authMiddleware := createTrackingMiddleware("auth")
		loggingMiddleware := createTrackingMiddleware("logging")
		metricsMiddleware := createTrackingMiddleware("metrics")

		// Add middleware with dependencies
		err := chain.AddMiddlewareWithDependencies(authMiddleware, []string{}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add auth middleware: %v", err)
		}

		err = chain.AddMiddlewareWithDependencies(loggingMiddleware, []string{"auth"}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add logging middleware: %v", err)
		}

		err = chain.AddMiddlewareWithDependencies(metricsMiddleware, []string{"logging"}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add metrics middleware: %v", err)
		}

		// Process request
		req := &Request{ID: "test", Path: "/test"}
		resp, err := chain.Process(context.Background(), req)
		if err != nil {
			t.Fatalf("Chain processing failed: %v", err)
		}

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status code 200, got %d", resp.StatusCode)
		}

		// Verify execution order
		mu.Lock()
		defer mu.Unlock()

		expectedOrder := []string{"auth", "logging", "metrics"}
		if len(executionOrder) != len(expectedOrder) {
			t.Fatalf("Expected %d middleware executions, got %d", len(expectedOrder), len(executionOrder))
		}

		for i, expected := range expectedOrder {
			if executionOrder[i] != expected {
				t.Fatalf("Expected execution order %v, got %v", expectedOrder, executionOrder)
			}
		}
	})

	t.Run("Dependency Validation", func(t *testing.T) {
		chain := NewDependencyAwareMiddlewareChain(nil)

		middleware1 := &TestMiddleware{name: "middleware1", enabled: true, priority: 100}

		// Add middleware with missing dependency
		err := chain.AddMiddlewareWithDependencies(middleware1, []string{"missing"}, false, &AlwaysDependencyCondition{})
		if err != nil {
			t.Fatalf("Failed to add middleware: %v", err)
		}

		// Validation should detect issues
		errors := chain.ValidateDependencies()
		if len(errors) == 0 {
			t.Fatal("Expected validation errors, got none")
		}
	})
}

func TestDependencyManager(t *testing.T) {
	t.Run("Multiple Chain Management", func(t *testing.T) {
		manager := NewDependencyManager()

		handler1 := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{ID: req.ID, StatusCode: 200, Body: "chain1"}, nil
		}

		handler2 := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{ID: req.ID, StatusCode: 200, Body: "chain2"}, nil
		}

		// Create chains
		chain1 := manager.CreateChain("chain1", handler1)
		chain2 := manager.CreateChain("chain2", handler2)

		if chain1 == nil {
			t.Fatal("Failed to create chain1")
		}

		if chain2 == nil {
			t.Fatal("Failed to create chain2")
		}

		// Verify retrieval
		retrievedChain1 := manager.GetChain("chain1")
		if retrievedChain1 != chain1 {
			t.Fatal("Failed to retrieve chain1")
		}

		retrievedChain2 := manager.GetChain("chain2")
		if retrievedChain2 != chain2 {
			t.Fatal("Failed to retrieve chain2")
		}
	})

	t.Run("Dependency Report", func(t *testing.T) {
		manager := NewDependencyManager()

		handler := func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{ID: req.ID, StatusCode: 200, Body: "processed"}, nil
		}

		chain := manager.CreateChain("test-chain", handler)

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

		// Get dependency report
		report := manager.GetDependencyReport()

		if len(report.Chains) != 1 {
			t.Fatalf("Expected 1 chain in report, got %d", len(report.Chains))
		}

		chainInfo, exists := report.Chains["test-chain"]
		if !exists {
			t.Fatal("Expected test-chain in report")
		}

		if chainInfo.MiddlewareCount != 2 {
			t.Fatalf("Expected 2 middleware in chain, got %d", chainInfo.MiddlewareCount)
		}

		// Verify dependencies
		authDeps, exists := chainInfo.Dependencies["auth"]
		if !exists {
			t.Fatal("Expected auth dependencies in report")
		}
		if len(authDeps) != 0 {
			t.Fatalf("Expected auth to have no dependencies, got %d", len(authDeps))
		}

		loggingDeps, exists := chainInfo.Dependencies["logging"]
		if !exists {
			t.Fatal("Expected logging dependencies in report")
		}
		if len(loggingDeps) != 1 || loggingDeps[0] != "auth" {
			t.Fatalf("Expected logging to depend on auth, got %v", loggingDeps)
		}
	})
}

func TestPriorityBasedDependency(t *testing.T) {
	t.Run("Automatic Priority Resolution", func(t *testing.T) {
		pbd := NewPriorityBasedDependency()

		// Add middleware with different priorities
		high := &TestMiddleware{name: "high", enabled: true, priority: 100}
		medium := &TestMiddleware{name: "medium", enabled: true, priority: 50}
		low := &TestMiddleware{name: "low", enabled: true, priority: 10}

		// Add in random order
		err := pbd.AddMiddlewareWithPriority(medium)
		if err != nil {
			t.Fatalf("Failed to add medium priority middleware: %v", err)
		}

		err = pbd.AddMiddlewareWithPriority(high)
		if err != nil {
			t.Fatalf("Failed to add high priority middleware: %v", err)
		}

		err = pbd.AddMiddlewareWithPriority(low)
		if err != nil {
			t.Fatalf("Failed to add low priority middleware: %v", err)
		}

		// Resolve order - should be high -> medium -> low
		req := &Request{ID: "test", Path: "/test"}
		order, err := pbd.ResolveOrder(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to resolve priority order: %v", err)
		}

		if len(order) != 3 {
			t.Fatalf("Expected 3 middleware, got %d", len(order))
		}

		if order[0].Name() != "high" {
			t.Fatalf("Expected high priority first, got %s", order[0].Name())
		}

		if order[1].Name() != "medium" {
			t.Fatalf("Expected medium priority second, got %s", order[1].Name())
		}

		if order[2].Name() != "low" {
			t.Fatalf("Expected low priority third, got %s", order[2].Name())
		}
	})
}

// TrackingMiddleware for testing execution order
type TrackingMiddleware struct {
	name      string
	enabled   bool
	priority  int
	onProcess func()
}

func (tm *TrackingMiddleware) Name() string {
	return tm.name
}

func (tm *TrackingMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	if tm.onProcess != nil {
		tm.onProcess()
	}
	return next(ctx, req)
}

func (tm *TrackingMiddleware) Configure(config map[string]interface{}) error {
	return nil
}

func (tm *TrackingMiddleware) Enabled() bool {
	return tm.enabled
}

func (tm *TrackingMiddleware) Priority() int {
	return tm.priority
}

func BenchmarkDependencyResolution(b *testing.B) {
	graph := NewDependencyGraph()

	// Create middleware chain with dependencies
	for i := 0; i < 10; i++ {
		middleware := &TestMiddleware{
			name:     fmt.Sprintf("middleware-%d", i),
			enabled:  true,
			priority: 100 - i,
		}

		var deps []string
		if i > 0 {
			deps = []string{fmt.Sprintf("middleware-%d", i-1)}
		}

		graph.AddMiddleware(middleware, deps, false, &AlwaysDependencyCondition{})
	}

	req := &Request{ID: "bench", Path: "/test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := graph.ResolveOrder(context.Background(), req)
		if err != nil {
			b.Fatalf("Resolution failed: %v", err)
		}
	}
}
