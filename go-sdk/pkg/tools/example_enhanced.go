package tools

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ExampleEnhancedExecutionUsage demonstrates the new features of the enhanced execution engine.
func ExampleEnhancedExecutionUsage() {
	// Create a registry and register a tool
	registry := NewRegistry()

	// Create a sample tool with caching capability
	tool := &Tool{
		ID:          "example-tool",
		Name:        "Example Tool",
		Description: "A tool that demonstrates enhanced execution features",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type:        "string",
					Description: "Input data for processing",
				},
			},
			Required: []string{"input"},
		},
		Executor: &ExampleExecutor{},
		Capabilities: &ToolCapabilities{
			Async:      true,
			Cancelable: true,
			Cacheable:  true,
			Streaming:  false,
			Timeout:    30 * time.Second,
		},
	}

	if err := registry.Register(tool); err != nil {
		log.Fatal("Failed to register tool:", err)
	}

	// Create enhanced execution engine with all features enabled
	engine := NewExecutionEngine(registry,
		WithCaching(100, 5*time.Minute),                   // Cache up to 100 results for 5 minutes
		WithAsyncWorkers(5),                               // 5 async workers
		WithResourceMonitoring(100*1024*1024, 80.0, 0, 0), // 100MB memory, 80% CPU limits
		WithSandboxing(&SandboxConfig{ // Enable sandboxing
			Enabled:          true,
			MaxMemory:        50 * 1024 * 1024, // 50MB limit
			NetworkAccess:    false,
			FileSystemAccess: true,
			AllowedPaths:     []string{"/tmp"},
			Timeout:          30 * time.Second,
		}),
	)

	ctx := context.Background()

	// Example 1: Synchronous execution with caching
	fmt.Println("=== Synchronous Execution with Caching ===")
	params := map[string]interface{}{"input": "test data"}

	// First execution - will hit the tool
	result1, err := engine.Execute(ctx, "example-tool", params)
	if err != nil {
		log.Printf("Execution error: %v", err)
		return
	}
	fmt.Printf("First execution result: %+v\n", result1)

	// Second execution - will hit the cache
	result2, err := engine.Execute(ctx, "example-tool", params)
	if err != nil {
		log.Printf("Execution error: %v", err)
		return
	}
	fmt.Printf("Second execution result (cached): %+v\n", result2)

	// Example 2: Asynchronous execution with priority
	fmt.Println("\n=== Asynchronous Execution with Priority ===")

	// Submit high priority job
	jobID1, resultChan1, err := engine.ExecuteAsync(ctx, "example-tool",
		map[string]interface{}{"input": "high priority task"}, 10)
	if err != nil {
		log.Printf("Async execution error: %v", err)
		return
	}
	fmt.Printf("Submitted high priority job: %s\n", jobID1)

	// Submit low priority job
	jobID2, resultChan2, err := engine.ExecuteAsync(ctx, "example-tool",
		map[string]interface{}{"input": "low priority task"}, 1)
	if err != nil {
		log.Printf("Async execution error: %v", err)
		return
	}
	fmt.Printf("Submitted low priority job: %s\n", jobID2)

	// Wait for results
	go func() {
		select {
		case result := <-resultChan1:
			fmt.Printf("High priority result: %+v\n", result)
		case <-time.After(10 * time.Second):
			fmt.Println("High priority job timed out")
		}
	}()

	go func() {
		select {
		case result := <-resultChan2:
			fmt.Printf("Low priority result: %+v\n", result)
		case <-time.After(10 * time.Second):
			fmt.Println("Low priority job timed out")
		}
	}()

	// Example 3: View metrics
	fmt.Println("\n=== Execution Metrics ===")
	time.Sleep(100 * time.Millisecond) // Allow async jobs to start

	metrics := engine.GetMetrics()
	fmt.Printf("Total executions: %d\n", metrics.totalExecutions)
	fmt.Printf("Cache hits: %d\n", metrics.cacheHits)
	fmt.Printf("Cache misses: %d\n", metrics.cacheMisses)
	fmt.Printf("Async executions: %d\n", metrics.asyncExecutions)

	cacheMetrics := engine.GetCacheMetrics()
	if cacheMetrics != nil {
		fmt.Printf("Cache size: %v\n", cacheMetrics["size"])
		fmt.Printf("Cache hit ratio: %.2f\n", cacheMetrics["hitRatio"])
	}

	resourceMetrics := engine.GetResourceMetrics()
	if resourceMetrics != nil {
		fmt.Printf("Resource violations: %v\n", resourceMetrics["violations"])
	}

	jobQueueMetrics := engine.GetJobQueueMetrics()
	if jobQueueMetrics != nil {
		fmt.Printf("Job queue size: %v\n", jobQueueMetrics["queueSize"])
		fmt.Printf("Worker count: %v\n", jobQueueMetrics["workers"])
	}

	// Example 4: Graceful shutdown
	fmt.Println("\n=== Graceful Shutdown ===")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := engine.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	} else {
		fmt.Println("Engine shut down gracefully")
	}
}

// ExampleExecutor is a simple tool executor for demonstration.
type ExampleExecutor struct{}

func (e *ExampleExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	input, ok := params["input"].(string)
	if !ok {
		return &ToolExecutionResult{
			Success: false,
			Error:   "input parameter must be a string",
		}, nil
	}

	// Simulate some processing time
	time.Sleep(50 * time.Millisecond)

	return &ToolExecutionResult{
		Success:   true,
		Data:      fmt.Sprintf("Processed: %s", input),
		Timestamp: time.Now(),
	}, nil
}
