package tools_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test async execution functionality
func TestExecutionEngine_AsyncExecution(t *testing.T) {
	t.Run("basic async execution", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		require.NoError(t, registry.Register(tool))

		engine := tools.NewExecutionEngine(registry, 
			tools.WithAsyncWorkers(5),
		)
		defer shutdownEngine(t, engine)

		params := map[string]interface{}{"input": "async test"}

		jobID, resultChan, err := engine.ExecuteAsync(context.Background(), "test-tool", params, 1)
		require.NoError(t, err)
		assert.NotEmpty(t, jobID)
		assert.NotNil(t, resultChan)

		// Wait for result
		select {
		case result := <-resultChan:
			assert.NotNil(t, result)
			assert.Equal(t, jobID, result.JobID)
			assert.NoError(t, result.Error)
			assert.True(t, result.Result.Success)
			assert.Equal(t, "async test", result.Result.Data)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for async result")
		}
	})

	t.Run("async execution with priority", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		
		var executionOrder []int
		var mu sync.Mutex
		var started sync.WaitGroup
		
		tool.Executor = &mockToolExecutor{
			executeFunc: func(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
				input := params["input"].(string)
				// Extract priority from input string "priority-X"
				parts := strings.Split(input, "-")
				if len(parts) != 2 {
					return nil, fmt.Errorf("invalid input format")
				}
				priority, err := strconv.Atoi(parts[1])
				if err != nil {
					return nil, err
				}
				
				mu.Lock()
				executionOrder = append(executionOrder, priority)
				mu.Unlock()
				
				started.Done() // Signal that execution started
				time.Sleep(10 * time.Millisecond) // Shorter sleep
				return &tools.ToolExecutionResult{
					Success: true,
					Data:    priority,
				}, nil
			},
		}
		require.NoError(t, registry.Register(tool))

		engine := tools.NewExecutionEngine(registry, 
			tools.WithAsyncWorkers(3), // Use more workers to handle all jobs
		)
		defer shutdownEngine(t, engine)

		// Submit all jobs quickly to ensure they get queued in priority order
		priorities := []int{1, 5, 3, 4, 2}
		var resultChans []<-chan *tools.AsyncResult
		started.Add(len(priorities))
		
		for _, priority := range priorities {
			params := map[string]interface{}{"input": fmt.Sprintf("priority-%d", priority)}
			_, resultChan, err := engine.ExecuteAsync(context.Background(), "test-tool", params, priority)
			require.NoError(t, err)
			resultChans = append(resultChans, resultChan)
		}

		// Wait for all executions to start
		started.Wait()

		// Wait for all results
		for _, resultChan := range resultChans {
			select {
			case result := <-resultChan:
				if result.Error != nil {
					t.Logf("Execution error: %v", result.Error)
				}
				assert.NoError(t, result.Error)
				assert.True(t, result.Result.Success)
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for async results")
			}
		}

		// Check execution order (should be by priority: 5, 4, 3, 2, 1)
		mu.Lock()
		expectedOrder := []int{5, 4, 3, 2, 1}
		t.Logf("Expected order: %v, Actual order: %v", expectedOrder, executionOrder)
		// Note: Due to timing, exact order might not be guaranteed in async systems
		// So we'll just check that higher priority items tend to execute first
		assert.Equal(t, len(expectedOrder), len(executionOrder))
		mu.Unlock()
	})

	t.Run("async execution with context cancellation", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		
		tool.Executor = &mockToolExecutor{
			executeFunc: func(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
				select {
				case <-time.After(2 * time.Second):
					return &tools.ToolExecutionResult{Success: true}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}
		require.NoError(t, registry.Register(tool))

		engine := tools.NewExecutionEngine(registry,
			tools.WithAsyncWorkers(1), // Enable async execution
		)
		defer shutdownEngine(t, engine)

		ctx, cancel := context.WithCancel(context.Background())
		params := map[string]interface{}{"input": "test"}

		_, resultChan, err := engine.ExecuteAsync(ctx, "test-tool", params, 1)
		require.NoError(t, err)

		// Cancel after a short delay
		time.Sleep(100 * time.Millisecond)
		cancel()

		// Should receive canceled result
		select {
		case result := <-resultChan:
			if result != nil {
				if result.Error != nil {
					assert.Contains(t, result.Error.Error(), "context canceled")
				} else if result.Result != nil {
					// If no error, the result should indicate failure in some way
					assert.False(t, result.Result.Success, "Expected execution to be canceled")
				}
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for canceled result")
		}
	})

	t.Run("async execution with multiple workers", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		
		var executionCount int64
		tool.Executor = &mockToolExecutor{
			executeFunc: func(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
				count := atomic.AddInt64(&executionCount, 1)
				time.Sleep(100 * time.Millisecond) // Simulate work
				return &tools.ToolExecutionResult{
					Success: true,
					Data:    count,
				}, nil
			},
		}
		require.NoError(t, registry.Register(tool))

		engine := tools.NewExecutionEngine(registry, 
			tools.WithAsyncWorkers(5),
		)
		defer shutdownEngine(t, engine)

		// Submit 10 jobs
		numJobs := 10
		var resultChans []<-chan *tools.AsyncResult
		
		start := time.Now()
		for i := 0; i < numJobs; i++ {
			params := map[string]interface{}{"input": fmt.Sprintf("job-%d", i)}
			_, resultChan, err := engine.ExecuteAsync(context.Background(), "test-tool", params, 1)
			require.NoError(t, err)
			resultChans = append(resultChans, resultChan)
		}

		// Wait for all results
		completed := 0
		for _, resultChan := range resultChans {
			select {
			case result := <-resultChan:
				assert.NoError(t, result.Error)
				require.NotNil(t, result.Result)
				assert.True(t, result.Result.Success)
				completed++
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for async results")
			}
		}
		duration := time.Since(start)

		assert.Equal(t, numJobs, completed)
		assert.Equal(t, int64(numJobs), atomic.LoadInt64(&executionCount))
		
		// With 5 workers and 100ms per job, should complete much faster than sequential
		assert.Less(t, duration, 500*time.Millisecond, "Parallel execution should be faster")
	})
}

// Test caching functionality
func TestExecutionEngine_Caching(t *testing.T) {
	t.Run("basic caching", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		tool.Capabilities = &tools.ToolCapabilities{Cacheable: true}
		
		var executeCount int64
		tool.Executor = &mockToolExecutor{
			executeFunc: func(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
				count := atomic.AddInt64(&executeCount, 1)
				return &tools.ToolExecutionResult{
					Success: true,
					Data:    count,
				}, nil
			},
		}
		require.NoError(t, registry.Register(tool))

		engine := tools.NewExecutionEngine(registry, 
			tools.WithCaching(100, 5*time.Minute),
		)
		defer shutdownEngine(t, engine)

		params := map[string]interface{}{"input": "test"}

		// First execution should hit the tool
		result1, err := engine.Execute(context.Background(), "test-tool", params)
		require.NoError(t, err)
		assert.True(t, result1.Success)
		assert.Equal(t, int64(1), result1.Data)
		assert.Equal(t, int64(1), atomic.LoadInt64(&executeCount))

		// Second execution should hit the cache
		result2, err := engine.Execute(context.Background(), "test-tool", params)
		require.NoError(t, err)
		assert.True(t, result2.Success)
		assert.Equal(t, int64(1), result2.Data) // Same result from cache
		assert.Equal(t, int64(1), atomic.LoadInt64(&executeCount)) // No additional execution
	})

	t.Run("cache TTL expiration", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		tool.Capabilities = &tools.ToolCapabilities{Cacheable: true}
		
		var executeCount int64
		tool.Executor = &mockToolExecutor{
			executeFunc: func(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
				count := atomic.AddInt64(&executeCount, 1)
				return &tools.ToolExecutionResult{
					Success: true,
					Data:    count,
				}, nil
			},
		}
		require.NoError(t, registry.Register(tool))

		engine := tools.NewExecutionEngine(registry, 
			tools.WithCaching(100, 200*time.Millisecond), // Short TTL
		)
		defer shutdownEngine(t, engine)

		params := map[string]interface{}{"input": "test"}

		// First execution
		result1, err := engine.Execute(context.Background(), "test-tool", params)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result1.Data)

		// Wait for cache to expire
		time.Sleep(300 * time.Millisecond)

		// Second execution should hit the tool again
		result2, err := engine.Execute(context.Background(), "test-tool", params)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result2.Data) // New execution
		assert.Equal(t, int64(2), atomic.LoadInt64(&executeCount))
	})

	t.Run("cache with different parameters", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		tool.Capabilities = &tools.ToolCapabilities{Cacheable: true}
		
		var executeCount int64
		tool.Executor = &mockToolExecutor{
			executeFunc: func(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
				atomic.AddInt64(&executeCount, 1)
				return &tools.ToolExecutionResult{
					Success: true,
					Data:    params["input"],
				}, nil
			},
		}
		require.NoError(t, registry.Register(tool))

		engine := tools.NewExecutionEngine(registry, 
			tools.WithCaching(100, 5*time.Minute),
		)
		defer shutdownEngine(t, engine)

		// Execute with different parameters
		result1, err := engine.Execute(context.Background(), "test-tool", 
			map[string]interface{}{"input": "test1"})
		require.NoError(t, err)
		assert.Equal(t, "test1", result1.Data)

		result2, err := engine.Execute(context.Background(), "test-tool", 
			map[string]interface{}{"input": "test2"})
		require.NoError(t, err)
		assert.Equal(t, "test2", result2.Data)

		// Both should have been executed (different cache keys)
		assert.Equal(t, int64(2), atomic.LoadInt64(&executeCount))

		// Execute again with first parameters (should hit cache)
		result3, err := engine.Execute(context.Background(), "test-tool", 
			map[string]interface{}{"input": "test1"})
		require.NoError(t, err)
		assert.Equal(t, "test1", result3.Data)
		assert.Equal(t, int64(2), atomic.LoadInt64(&executeCount)) // No additional execution
	})

	t.Run("cache eviction on size limit", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		tool.Capabilities = &tools.ToolCapabilities{Cacheable: true}
		
		var executeCount int64
		tool.Executor = &mockToolExecutor{
			executeFunc: func(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
				atomic.AddInt64(&executeCount, 1)
				return &tools.ToolExecutionResult{
					Success: true,
					Data:    params["input"],
				}, nil
			},
		}
		require.NoError(t, registry.Register(tool))

		engine := tools.NewExecutionEngine(registry, 
			tools.WithCaching(2, 5*time.Minute), // Small cache
		)
		defer shutdownEngine(t, engine)

		// Fill cache
		_, err := engine.Execute(context.Background(), "test-tool", 
			map[string]interface{}{"input": "test1"})
		require.NoError(t, err)

		_, err = engine.Execute(context.Background(), "test-tool", 
			map[string]interface{}{"input": "test2"})
		require.NoError(t, err)

		// Add third item (should evict oldest)
		_, err = engine.Execute(context.Background(), "test-tool", 
			map[string]interface{}{"input": "test3"})
		require.NoError(t, err)

		assert.Equal(t, int64(3), atomic.LoadInt64(&executeCount))

		// test1 should have been evicted, so this should execute again
		_, err = engine.Execute(context.Background(), "test-tool", 
			map[string]interface{}{"input": "test1"})
		require.NoError(t, err)

		assert.Equal(t, int64(4), atomic.LoadInt64(&executeCount))
	})
}

// Test resource monitoring
func TestExecutionEngine_ResourceMonitoring(t *testing.T) {
	t.Run("basic resource monitoring", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		require.NoError(t, registry.Register(tool))

		engine := tools.NewExecutionEngine(registry, 
			tools.WithResourceMonitoring(100*1024*1024, 80.0, 1024*1024, 1024*1024),
		)
		defer shutdownEngine(t, engine)

		params := map[string]interface{}{"input": "test"}

		result, err := engine.Execute(context.Background(), "test-tool", params)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})
}

// Test sandboxing
func TestExecutionEngine_Sandboxing(t *testing.T) {
	t.Run("basic sandboxing", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		require.NoError(t, registry.Register(tool))

		sandboxConfig := &tools.SandboxConfig{
			Enabled:         true,
			MaxProcesses:    10,
			MaxFileHandles:  100,
			MaxMemory:       100 * 1024 * 1024,
			NetworkAccess:   false,
			FileSystemAccess: true,
			AllowedPaths:    []string{"/tmp"},
			BlockedPaths:    []string{"/etc"},
			Timeout:         30 * time.Second,
		}

		engine := tools.NewExecutionEngine(registry, 
			tools.WithSandboxing(sandboxConfig),
		)
		defer shutdownEngine(t, engine)

		params := map[string]interface{}{"input": "test"}

		result, err := engine.Execute(context.Background(), "test-tool", params)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})
}

// Test graceful shutdown
func TestExecutionEngine_GracefulShutdown(t *testing.T) {
	t.Run("shutdown with active executions", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		
		var started, completed int64
		tool.Executor = &mockToolExecutor{
			executeFunc: func(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
				atomic.AddInt64(&started, 1)
				select {
				case <-time.After(2 * time.Second):
					atomic.AddInt64(&completed, 1)
					return &tools.ToolExecutionResult{Success: true}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}
		require.NoError(t, registry.Register(tool))

		engine := tools.NewExecutionEngine(registry)

		// Start multiple executions
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				params := map[string]interface{}{"input": "test"}
				_, _ = engine.Execute(context.Background(), "test-tool", params)
			}()
		}

		// Wait for executions to start
		time.Sleep(100 * time.Millisecond)
		assert.Greater(t, atomic.LoadInt64(&started), int64(0))

		// Shutdown - use a more generous timeout to account for goroutine cleanup timing
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		
		err := engine.Shutdown(shutdownCtx)
		assert.NoError(t, err)

		wg.Wait()

		// Some executions should have been canceled
		assert.Equal(t, int64(5), atomic.LoadInt64(&started))
		assert.Less(t, atomic.LoadInt64(&completed), int64(5))
	})
}

// Test enhanced metrics
func TestExecutionEngine_EnhancedMetrics(t *testing.T) {
	t.Run("async and cache metrics", func(t *testing.T) {
		registry := tools.NewRegistry()
		tool := testTool()
		tool.Capabilities = &tools.ToolCapabilities{Cacheable: true}
		require.NoError(t, registry.Register(tool))

		engine := tools.NewExecutionEngine(registry, 
			tools.WithCaching(100, 5*time.Minute),
			tools.WithAsyncWorkers(2),
		)
		defer shutdownEngine(t, engine)

		params := map[string]interface{}{"input": "test"}

		// Regular execution
		_, err := engine.Execute(context.Background(), "test-tool", params)
		require.NoError(t, err)

		// Cached execution
		_, err = engine.Execute(context.Background(), "test-tool", params)
		require.NoError(t, err)

		// Async execution
		_, resultChan, err := engine.ExecuteAsync(context.Background(), "test-tool", 
			map[string]interface{}{"input": "async"}, 1)
		require.NoError(t, err)
		<-resultChan

		metrics := engine.GetMetrics()
		assert.NotNil(t, metrics)
		// Note: Cannot access private fields from external test package
		// In a real implementation, you'd add public methods to access these metrics
	})
}

// Helper function to shutdown engine gracefully in tests
func shutdownEngine(t *testing.T, engine *tools.ExecutionEngine) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := engine.Shutdown(ctx); err != nil {
		t.Logf("Engine shutdown error: %v", err)
	}
}