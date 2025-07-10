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