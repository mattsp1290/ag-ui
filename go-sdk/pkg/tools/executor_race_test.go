package tools

import (
	"context"
	"sync"
	"testing"
	"time"
)

// Simple mock executor for testing
type mockExecutor struct{}

func (m *mockExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	// Simulate some work
	time.Sleep(10 * time.Millisecond)
	return &ToolExecutionResult{
		Success: true,
		Data:    map[string]interface{}{"result": "success"},
	}, nil
}

// Test that the executor handles concurrent execution correctly
func TestExecutorConcurrentExecution(t *testing.T) {
	// Create a registry and tool
	registry := NewRegistry()
	tool := &Tool{
		ID:          "test-tool",
		Name:        "Test Tool",
		Description: "A tool for testing",
		Version:     "1.0.0",
		Executor:    &mockExecutor{},
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type:        "string",
					Description: "Input parameter",
				},
			},
		},
	}
	
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}
	
	// Create execution engine with low concurrency limit
	engine := NewExecutionEngine(registry, WithMaxConcurrent(5))
	
	// Run many concurrent executions
	var wg sync.WaitGroup
	results := make([]*ToolExecutionResult, 20)
	errors := make([]error, 20)
	
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			results[idx], errors[idx] = engine.Execute(ctx, "test-tool", map[string]interface{}{})
		}(i)
	}
	
	wg.Wait()
	
	// Check results
	successCount := 0
	for i, result := range results {
		if errors[i] == nil && result != nil && result.Success {
			successCount++
		}
	}
	
	if successCount != 20 {
		t.Errorf("Expected 20 successful executions, got %d", successCount)
	}
	
	// Check that active count is back to 0
	activeCount := engine.GetActiveExecutions()
	if activeCount != 0 {
		t.Errorf("Expected 0 active executions, got %d", activeCount)
	}
}

// Test that metrics are updated correctly under concurrent access
func TestExecutorMetricsRaceCondition(t *testing.T) {
	registry := NewRegistry()
	tool := &Tool{
		ID:          "test-tool",
		Name:        "Test Tool",
		Description: "A tool for testing",
		Version:     "1.0.0",
		Executor:    &mockExecutor{},
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type:        "string",
					Description: "Input parameter",
				},
			},
		},
	}
	
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}
	
	engine := NewExecutionEngine(registry, WithMaxConcurrent(10))
	
	// Run concurrent executions
	var wg sync.WaitGroup
	numExecutions := 100
	
	for i := 0; i < numExecutions; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			engine.Execute(ctx, "test-tool", map[string]interface{}{})
		}()
	}
	
	wg.Wait()
	
	// Check metrics
	metrics := engine.GetMetrics()
	if metrics.totalExecutions != int64(numExecutions) {
		t.Errorf("Expected %d total executions, got %d", numExecutions, metrics.totalExecutions)
	}
}

// Test that hooks are executed safely under concurrent access
func TestExecutorHookRaceCondition(t *testing.T) {
	registry := NewRegistry()
	tool := &Tool{
		ID:          "test-tool",
		Name:        "Test Tool",
		Description: "A tool for testing",
		Version:     "1.0.0",
		Executor:    &mockExecutor{},
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type:        "string",
					Description: "Input parameter",
				},
			},
		},
	}
	
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}
	
	engine := NewExecutionEngine(registry, WithMaxConcurrent(10))
	
	// Add hooks concurrently while executing
	var wg sync.WaitGroup
	hookCount := 0
	var hookMu sync.Mutex
	
	// Hook that increments counter
	hook := func(ctx context.Context, toolID string, params map[string]interface{}) error {
		hookMu.Lock()
		hookCount++
		hookMu.Unlock()
		return nil
	}
	
	// Add hooks and execute concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				// Add hooks
				engine.AddBeforeExecuteHook(hook)
			} else {
				// Execute tool
				ctx := context.Background()
				engine.Execute(ctx, "test-tool", map[string]interface{}{})
			}
		}(i)
	}
	
	wg.Wait()
	
	// The test passes if no race conditions occurred (detected by -race flag)
	t.Logf("Hook count: %d", hookCount)
}

// Test the new channel-based semaphore under extreme concurrent load (1000+ operations)
func TestExecutorChannelSemaphoreStressTest(t *testing.T) {
	registry := NewRegistry()
	tool := &Tool{
		ID:          "stress-test-tool",
		Name:        "Stress Test Tool",
		Description: "A tool for stress testing",
		Version:     "1.0.0",
		Executor:    &mockExecutor{},
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type:        "string",
					Description: "Input parameter",
				},
			},
		},
	}
	
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}
	
	// Create execution engine with limited concurrency to stress test semaphore
	maxConcurrent := 10
	engine := NewExecutionEngine(registry, WithMaxConcurrent(maxConcurrent))
	
	// Run 1000+ concurrent executions to stress test the semaphore
	var wg sync.WaitGroup
	numExecutions := 1500
	results := make([]*ToolExecutionResult, numExecutions)
	errors := make([]error, numExecutions)
	
	startTime := time.Now()
	
	for i := 0; i < numExecutions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			results[idx], errors[idx] = engine.Execute(ctx, "stress-test-tool", map[string]interface{}{
				"iteration": idx,
			})
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(startTime)
	
	// Count different result types
	successCount := 0
	cancelledCount := 0
	otherErrorCount := 0
	
	for i, result := range results {
		if errors[i] != nil {
			// Check if it's a context cancellation (expected during shutdown)
			if errors[i] == context.Canceled || errors[i] == context.DeadlineExceeded {
				cancelledCount++
			} else {
				otherErrorCount++
			}
		} else if result != nil && result.Success {
			successCount++
		}
	}
	
	t.Logf("Stress test completed in %v", duration)
	t.Logf("Successful executions: %d, Cancelled: %d, Other errors: %d", 
		successCount, cancelledCount, otherErrorCount)
	
	// With blocking semaphore, all executions should eventually succeed
	// (unless context was cancelled during shutdown)
	if successCount < numExecutions-cancelledCount {
		t.Errorf("Expected %d successful executions, got %d (cancelled: %d)", 
			numExecutions-cancelledCount, successCount, cancelledCount)
	}
	
	// Should have no other types of errors
	if otherErrorCount > 0 {
		t.Errorf("Unexpected other errors: %d", otherErrorCount)
	}
	
	// Verify active count is back to 0
	activeCount := engine.GetActiveExecutions()
	if activeCount != 0 {
		t.Errorf("Expected 0 active executions after completion, got %d", activeCount)
	}
	
	// Verify semaphore capacity is correct
	if cap(engine.concurrencySemaphore) != maxConcurrent {
		t.Errorf("Expected semaphore capacity %d, got %d", maxConcurrent, cap(engine.concurrencySemaphore))
	}
}

// Test concurrent execution with context cancellation and timeouts
func TestExecutorConcurrentCancellation(t *testing.T) {
	registry := NewRegistry()
	
	// Create a slow executor for timeout/cancellation testing
	slowExecutor := &slowMockExecutor{delay: 100 * time.Millisecond}
	tool := &Tool{
		ID:          "slow-tool",
		Name:        "Slow Tool",
		Description: "A slow tool for timeout testing",
		Version:     "1.0.0",
		Executor:    slowExecutor,
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type:        "string",
					Description: "Input parameter",
				},
			},
		},
	}
	
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}
	
	engine := NewExecutionEngine(registry, WithMaxConcurrent(5))
	
	var wg sync.WaitGroup
	numExecutions := 20
	results := make([]*ToolExecutionResult, numExecutions)
	errors := make([]error, numExecutions)
	
	// Start executions with various contexts
	for i := 0; i < numExecutions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			
			// Create contexts with different cancellation patterns
			var ctx context.Context
			var cancel context.CancelFunc
			
			switch idx % 3 {
			case 0:
				// Normal context
				ctx = context.Background()
			case 1:
				// Context with timeout
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
			case 2:
				// Context that gets cancelled immediately
				ctx, cancel = context.WithCancel(context.Background())
				cancel() // Cancel immediately
			}
			
			if cancel != nil {
				defer cancel()
			}
			
			results[idx], errors[idx] = engine.Execute(ctx, "slow-tool", map[string]interface{}{
				"iteration": idx,
			})
		}(i)
	}
	
	wg.Wait()
	
	// Count different result types
	successCount := 0
	cancelledCount := 0
	otherErrorCount := 0
	
	for i, result := range results {
		if errors[i] != nil {
			if errors[i] == context.Canceled || errors[i] == context.DeadlineExceeded {
				cancelledCount++
			} else {
				otherErrorCount++
			}
		} else if result != nil && result.Success {
			successCount++
		}
	}
	
	t.Logf("Success: %d, Cancelled: %d, Other errors: %d", successCount, cancelledCount, otherErrorCount)
	
	// Verify active count is back to 0
	activeCount := engine.GetActiveExecutions()
	if activeCount != 0 {
		t.Errorf("Expected 0 active executions after cancellation test, got %d", activeCount)
	}
}

// Test that the semaphore correctly limits concurrent executions
func TestExecutorSemaphoreLimit(t *testing.T) {
	registry := NewRegistry()
	
	// Create an executor that tracks concurrent executions
	concurrentExecutor := &concurrencyTrackingExecutor{}
	tool := &Tool{
		ID:          "concurrency-tool",
		Name:        "Concurrency Tool",
		Description: "A tool that tracks concurrent executions",
		Version:     "1.0.0",
		Executor:    concurrentExecutor,
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type:        "string",
					Description: "Input parameter",
				},
			},
		},
	}
	
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}
	
	maxConcurrent := 3
	engine := NewExecutionEngine(registry, WithMaxConcurrent(maxConcurrent))
	
	var wg sync.WaitGroup
	numExecutions := 20
	
	for i := 0; i < numExecutions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			engine.Execute(ctx, "concurrency-tool", map[string]interface{}{
				"iteration": idx,
			})
		}(i)
	}
	
	wg.Wait()
	
	// Verify the maximum concurrent executions never exceeded the limit
	if concurrentExecutor.maxConcurrent > maxConcurrent {
		t.Errorf("Semaphore failed: max concurrent executions was %d, limit was %d", 
			concurrentExecutor.maxConcurrent, maxConcurrent)
	}
	
	// Verify active count is back to 0
	activeCount := engine.GetActiveExecutions()
	if activeCount != 0 {
		t.Errorf("Expected 0 active executions, got %d", activeCount)
	}
}

// Slow executor for timeout testing
type slowMockExecutor struct {
	delay time.Duration
}

func (s *slowMockExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	select {
	case <-time.After(s.delay):
		return &ToolExecutionResult{
			Success: true,
			Data:    map[string]interface{}{"result": "success"},
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Executor that tracks concurrent executions to verify semaphore is working
type concurrencyTrackingExecutor struct {
	currentConcurrent int32
	maxConcurrent     int
	mu                sync.Mutex
}

func (c *concurrencyTrackingExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	// Increment and track concurrent executions
	c.mu.Lock()
	c.currentConcurrent++
	if int(c.currentConcurrent) > c.maxConcurrent {
		c.maxConcurrent = int(c.currentConcurrent)
	}
	c.mu.Unlock()
	
	// Simulate work
	time.Sleep(50 * time.Millisecond)
	
	// Decrement concurrent executions
	c.mu.Lock()
	c.currentConcurrent--
	c.mu.Unlock()
	
	return &ToolExecutionResult{
		Success: true,
		Data:    map[string]interface{}{"result": "success"},
	}, nil
}