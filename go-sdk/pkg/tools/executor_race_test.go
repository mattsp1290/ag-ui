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

// Test the race condition fix with atomic counters under extreme concurrent load
func TestExecutorAtomicCounterRaceFix(t *testing.T) {
	registry := NewRegistry()
	tool := &Tool{
		ID:          "atomic-test-tool",
		Name:        "Atomic Counter Test Tool",
		Description: "A tool for testing atomic counter race fix",
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
	
	// Create execution engine with moderate concurrency limit
	maxConcurrent := 50
	engine := NewExecutionEngine(registry, WithMaxConcurrent(maxConcurrent))
	
	// Run many concurrent executions to stress test the atomic counter
	var wg sync.WaitGroup
	numGoroutines := 200 // More than max concurrent to ensure queuing
	results := make([]*ToolExecutionResult, numGoroutines)
	errors := make([]error, numGoroutines)
	
	// Track GetActiveExecutions() calls during concurrent execution
	var activeCountChecks []int
	var activeCountMu sync.Mutex
	
	// Start a goroutine to continuously check active executions
	checkDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-checkDone:
				return
			case <-ticker.C:
				activeCount := engine.GetActiveExecutions()
				activeCountMu.Lock()
				activeCountChecks = append(activeCountChecks, activeCount)
				activeCountMu.Unlock()
			}
		}
	}()
	
	startTime := time.Now()
	
	// Launch all goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			results[idx], errors[idx] = engine.Execute(ctx, "atomic-test-tool", map[string]interface{}{
				"iteration": idx,
			})
		}(i)
	}
	
	wg.Wait()
	close(checkDone)
	duration := time.Since(startTime)
	
	// Verify all executions completed successfully
	successCount := 0
	for i, result := range results {
		if errors[i] == nil && result != nil && result.Success {
			successCount++
		} else if errors[i] != nil {
			t.Errorf("Execution %d failed with error: %v", i, errors[i])
		}
	}
	
	// Verify final state
	finalActiveCount := engine.GetActiveExecutions()
	
	// Get count checks safely
	activeCountMu.Lock()
	checkCount := len(activeCountChecks)
	activeCountMu.Unlock()
	
	t.Logf("Atomic counter race test completed in %v", duration)
	t.Logf("Successful executions: %d/%d", successCount, numGoroutines)
	t.Logf("Final active count: %d", finalActiveCount)
	t.Logf("Active count checks performed: %d", checkCount)
	
	// All executions should complete successfully
	if successCount != numGoroutines {
		t.Errorf("Expected %d successful executions, got %d", numGoroutines, successCount)
	}
	
	// Final active count should be 0 (this is the main test for the race condition fix)
	if finalActiveCount != 0 {
		t.Errorf("RACE CONDITION NOT FIXED: Expected 0 active executions after completion, got %d", finalActiveCount)
	}
	
	// Verify active count never exceeded the maximum concurrent limit
	activeCountMu.Lock()
	maxObservedActive := 0
	for _, count := range activeCountChecks {
		if count > maxObservedActive {
			maxObservedActive = count
		}
		if count > maxConcurrent {
			t.Errorf("Active execution count %d exceeded max concurrent limit %d", count, maxConcurrent)
		}
	}
	activeCountMu.Unlock()
	
	t.Logf("Maximum observed active executions: %d (limit: %d)", maxObservedActive, maxConcurrent)
	
	// Ensure we actually had some concurrent executions
	if maxObservedActive == 0 {
		t.Error("Test may be invalid: no concurrent executions were observed")
	}
}

// Test atomic counter consistency across Execute and ExecuteStream methods
func TestExecutorAtomicCounterConsistency(t *testing.T) {
	registry := NewRegistry()
	
	// Create both regular and streaming executors
	streamingTool := &Tool{
		ID:          "streaming-atomic-tool",
		Name:        "Streaming Atomic Test Tool",
		Description: "A streaming tool for testing atomic counter consistency",
		Version:     "1.0.0",
		Executor:    &streamingMockExecutor{},
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
	
	regularTool := &Tool{
		ID:          "regular-atomic-tool",
		Name:        "Regular Atomic Test Tool",
		Description: "A regular tool for testing atomic counter consistency",
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
	
	err := registry.Register(streamingTool)
	if err != nil {
		t.Fatalf("Failed to register streaming tool: %v", err)
	}
	
	err = registry.Register(regularTool)
	if err != nil {
		t.Fatalf("Failed to register regular tool: %v", err)
	}
	
	engine := NewExecutionEngine(registry, WithMaxConcurrent(25))
	
	var wg sync.WaitGroup
	numExecutions := 100
	
	// Mix of Execute and ExecuteStream calls
	for i := 0; i < numExecutions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			
			if idx%2 == 0 {
				// Regular execution
				_, err := engine.Execute(ctx, "regular-atomic-tool", map[string]interface{}{
					"iteration": idx,
				})
				if err != nil {
					t.Errorf("Regular execution %d failed: %v", idx, err)
				}
			} else {
				// Streaming execution
				stream, err := engine.ExecuteStream(ctx, "streaming-atomic-tool", map[string]interface{}{
					"iteration": idx,
				})
				if err != nil {
					t.Errorf("Streaming execution %d failed: %v", idx, err)
					return
				}
				
				// Consume the stream
				for range stream {
					// Just consume chunks
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify final active count is 0
	finalActiveCount := engine.GetActiveExecutions()
	if finalActiveCount != 0 {
		t.Errorf("RACE CONDITION: Mixed Execute/ExecuteStream left %d active executions", finalActiveCount)
	}
}

// Mock streaming executor for atomic counter testing
type streamingMockExecutor struct{}

func (s *streamingMockExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	time.Sleep(5 * time.Millisecond)
	return &ToolExecutionResult{
		Success: true,
		Data:    map[string]interface{}{"result": "success"},
	}, nil
}

func (s *streamingMockExecutor) ExecuteStream(ctx context.Context, params map[string]interface{}) (<-chan *ToolStreamChunk, error) {
	ch := make(chan *ToolStreamChunk, 3)
	
	go func() {
		defer close(ch)
		
		// Send a few chunks with small delays
		for i := 0; i < 3; i++ {
			select {
			case <-ctx.Done():
				return
			case ch <- &ToolStreamChunk{
				Type:  "data",
				Data:  map[string]interface{}{"chunk": i},
				Index: i,
			}:
				time.Sleep(2 * time.Millisecond)
			}
		}
		
		// Send completion chunk
		select {
		case <-ctx.Done():
		case ch <- &ToolStreamChunk{
			Type:  "complete",
			Index: 3,
		}:
		}
	}()
	
	return ch, nil
}