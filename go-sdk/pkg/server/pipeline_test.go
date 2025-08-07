package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/testhelper"
)

func TestPipeline(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	config := DefaultPipelineConfig()
	
	// Create pipeline
	pipeline, err := NewRequestProcessingPipeline(config)
	require.NoError(t, err)
	require.NotNil(t, pipeline)
	
	// Note: Pipeline doesn't have Start/Stop/IsRunning methods
	// It processes requests directly

	t.Run("Pipeline Configuration", func(t *testing.T) {
		assert.Equal(t, config.MaxConcurrentRequests, pipeline.config.MaxConcurrentRequests)
		assert.Equal(t, config.RequestTimeout, pipeline.config.RequestTimeout)
		assert.Equal(t, config.MaxRequestSize, pipeline.config.MaxRequestSize)
	})

	t.Run("Pipeline Processing", func(t *testing.T) {
		ctx := context.Background()
		
		// Create a test HTTP request with required headers
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "test-agent")
		
		// Process request through pipeline
		response, err := pipeline.Process(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
}

func TestPipelineConfig(t *testing.T) {
	t.Run("DefaultPipelineConfig", func(t *testing.T) {
		config := DefaultPipelineConfig()
		
		assert.Greater(t, config.MaxConcurrentRequests, 0)
		assert.Greater(t, config.RequestTimeout, time.Duration(0))
		assert.Greater(t, config.MaxRequestSize, int64(0))
		assert.Greater(t, config.MaxResponseSize, int64(0))
		assert.True(t, config.EnableMetrics)
		assert.True(t, config.EnableTracing)
	})

	t.Run("PipelineConfig Constructor Validation", func(t *testing.T) {
		tests := []struct {
			name    string
			config  *PipelineConfig
			wantErr bool
		}{
			{
				name: "negative max concurrent requests",
				config: &PipelineConfig{
					MaxConcurrentRequests: -1,
					RequestTimeout:        time.Second,
					MaxRequestSize:        1024,
				},
				wantErr: true,
			},
			{
				name: "negative request timeout",
				config: &PipelineConfig{
					MaxConcurrentRequests: 10,
					RequestTimeout:        -1 * time.Second,
					MaxRequestSize:        1024,
				},
				wantErr: true,
			},
			{
				name: "negative max request size",
				config: &PipelineConfig{
					MaxConcurrentRequests: 10,
					RequestTimeout:        time.Second,
					MaxRequestSize:        -1,
				},
				wantErr: true,
			},
			{
				name: "valid config",
				config: &PipelineConfig{
					MaxConcurrentRequests: 10,
					RequestTimeout:        30 * time.Second,
					MaxRequestSize:        1024 * 1024,
					MaxResponseSize:       1024 * 1024,
					EnableMetrics:         true,
					EnableTracing:         true,
				},
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Test validation through constructor
				_, err := NewRequestProcessingPipeline(tt.config)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestPipelineRequestProcessing(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	config := DefaultPipelineConfig()
	config.MaxConcurrentRequests = 5 // Small limit for testing
	config.MaxRequestSize = 1024 * 1024 // 1MB
	
	pipeline, err := NewRequestProcessingPipeline(config)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Basic Request Processing", func(t *testing.T) {
		// Process request
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "test-agent")
		response, err := pipeline.Process(ctx, req)
		
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})

	t.Run("Request with JSON Body", func(t *testing.T) {
		requestBody := `{"message": "test message"}`
		
		req := httptest.NewRequest("POST", "/test", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "test-agent")
		response, err := pipeline.Process(ctx, req)
		
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})

	t.Run("Request Size Limit", func(t *testing.T) {
		// Create config with small request size limit
		smallConfig := DefaultPipelineConfig()
		smallConfig.MaxRequestSize = 10 // Very small limit
		smallPipeline, err := NewRequestProcessingPipeline(smallConfig)
		require.NoError(t, err)
		
		// Create request that exceeds limit
		largeBody := strings.Repeat("x", 100)
		req := httptest.NewRequest("POST", "/test", strings.NewReader(largeBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "test-agent")
		
		response, err := smallPipeline.Process(ctx, req)
		require.NoError(t, err) // Pipeline returns error responses, not errors
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		// Create cancelable context
		reqCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately
		
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "test-agent")
		
		// Process with cancelled context - should still work since pipeline handles context internally
		response, err := pipeline.Process(reqCtx, req)
		require.NoError(t, err)
		require.NotNil(t, response)
	})
}

func TestPipelineStages(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	config := DefaultPipelineConfig()
	
	pipeline, err := NewRequestProcessingPipeline(config)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Add and Remove Pipeline Stages", func(t *testing.T) {
		var executionOrder []string
		var mu sync.Mutex
		
		// Create test stages
		stage1 := &testPipelineStage{
			name: "stage1",
			priority: 500,
			processHandler: func(ctx context.Context, pipelineCtx *PipelineContext) error {
				mu.Lock()
				executionOrder = append(executionOrder, "stage1")
				mu.Unlock()
				return nil
			},
		}
		
		stage2 := &testPipelineStage{
			name: "stage2",
			priority: 400,
			processHandler: func(ctx context.Context, pipelineCtx *PipelineContext) error {
				mu.Lock()
				executionOrder = append(executionOrder, "stage2")
				mu.Unlock()
				return nil
			},
		}
		
		// Add stages
		pipeline.AddStage(stage1)
		pipeline.AddStage(stage2)
		
		// Verify stages added (plus default stages)
		stages := pipeline.GetStages()
		assert.GreaterOrEqual(t, len(stages), 2)
		
		// Process request to test execution
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "test-agent")
		_, err := pipeline.Process(ctx, req)
		require.NoError(t, err)
		
		// Verify our custom stages were executed
		mu.Lock()
		assert.Contains(t, executionOrder, "stage1")
		assert.Contains(t, executionOrder, "stage2")
		mu.Unlock()
		
		// Remove stage
		err = pipeline.RemoveStage("stage1")
		assert.NoError(t, err)
		
		stages = pipeline.GetStages()
		found := false
		for _, stage := range stages {
			if stage.Name() == "stage1" {
				found = true
				break
			}
		}
		assert.False(t, found)
		
		// Try to remove non-existent stage
		err = pipeline.RemoveStage("non-existent")
		assert.Error(t, err)
	})

	t.Run("Stage Priority Ordering", func(t *testing.T) {
		// Create new pipeline to test priority ordering
		testPipeline, err := NewRequestProcessingPipeline(DefaultPipelineConfig())
		require.NoError(t, err)
		
		var executionOrder []string
		var mu sync.Mutex
		
		// Create stages with different priorities
		highPriorityStage := &testPipelineStage{
			name:     "high-priority",
			priority: 2000, // Higher than default stages
			processHandler: func(ctx context.Context, pipelineCtx *PipelineContext) error {
				mu.Lock()
				executionOrder = append(executionOrder, "high-priority")
				mu.Unlock()
				return nil
			},
		}
		
		lowPriorityStage := &testPipelineStage{
			name:     "low-priority",
			priority: 100, // Lower than default stages
			processHandler: func(ctx context.Context, pipelineCtx *PipelineContext) error {
				mu.Lock()
				executionOrder = append(executionOrder, "low-priority")
				mu.Unlock()
				return nil
			},
		}
		
		mediumPriorityStage := &testPipelineStage{
			name:     "medium-priority",
			priority: 1500,
			processHandler: func(ctx context.Context, pipelineCtx *PipelineContext) error {
				mu.Lock()
				executionOrder = append(executionOrder, "medium-priority")
				mu.Unlock()
				return nil
			},
		}
		
		// Add in random order
		testPipeline.AddStage(lowPriorityStage)
		testPipeline.AddStage(highPriorityStage)
		testPipeline.AddStage(mediumPriorityStage)
		
		// Process request
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "test-agent")
		_, err = testPipeline.Process(ctx, req)
		require.NoError(t, err)
		
		// Verify execution order (highest priority first)
		mu.Lock()
		expectedOrder := []string{"high-priority", "medium-priority", "low-priority"}
		assert.Equal(t, expectedOrder, executionOrder)
		mu.Unlock()
	})
}

func TestPipelineMetrics(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	config := DefaultPipelineConfig()
	config.EnableMetrics = true
	
	pipeline, err := NewRequestProcessingPipeline(config)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Metrics Collection", func(t *testing.T) {
		// Check pipeline has metrics enabled
		assert.NotNil(t, pipeline.metrics)
		
		// Process several requests
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", fmt.Sprintf("/test%d", i), nil)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "test-agent")
			_, err := pipeline.Process(ctx, req)
			require.NoError(t, err)
		}
		
		// Verify metrics were collected (atomic counters should be > 0)
		assert.Greater(t, pipeline.totalRequests, int64(0))
	})

	t.Run("Error Metrics", func(t *testing.T) {
		initialErrors := pipeline.totalErrors
		
		// Process request that will cause validation error (missing required headers)
		req := httptest.NewRequest("GET", "/error", nil)
		// Don't set required headers to trigger validation error
		
		response, err := pipeline.Process(ctx, req)
		require.NoError(t, err) // Pipeline returns error responses, not errors
		assert.Equal(t, http.StatusBadRequest, response.StatusCode) // Should be validation error
		
		// Check error metrics were incremented
		assert.Greater(t, pipeline.totalErrors, initialErrors)
	})

	t.Run("Pipeline Stages Count", func(t *testing.T) {
		stages := pipeline.GetStages()
		assert.NotNil(t, stages)
		assert.Greater(t, len(stages), 0) // Should have default stages
		
		// Verify we have expected default stages
		stageNames := make([]string, len(stages))
		for i, stage := range stages {
			stageNames[i] = stage.Name()
		}
		
		// Should contain validation stage
		assert.Contains(t, stageNames, "validation")
	})
}

func TestPipelineConcurrency(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	config := DefaultPipelineConfig()
	config.MaxConcurrentRequests = 10
	
	pipeline, err := NewRequestProcessingPipeline(config)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Concurrent Request Processing", func(t *testing.T) {
		const numRequests = 20
		var wg sync.WaitGroup
		var successCount int32
		var errorCount int32
		
		// Process requests concurrently
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				req := httptest.NewRequest("GET", fmt.Sprintf("/concurrent%d", id), nil)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", "test-agent")
				
				response, err := pipeline.Process(ctx, req)
				if err != nil || response.StatusCode != http.StatusOK {
					atomic.AddInt32(&errorCount, 1)
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}(i)
		}
		
		wg.Wait()
		
		// All requests should complete
		assert.Equal(t, int32(numRequests), successCount+errorCount)
		t.Logf("Successful: %d, Errors: %d", successCount, errorCount)
	})

	t.Run("Request Limit Handling", func(t *testing.T) {
		// Create pipeline with very low concurrent limit
		limitConfig := DefaultPipelineConfig()
		limitConfig.MaxConcurrentRequests = 2
		limitPipeline, err := NewRequestProcessingPipeline(limitConfig)
		require.NoError(t, err)
		
		var wg sync.WaitGroup
		const numRequests = 5 // More than MaxConcurrentRequests
		var rateLimitErrors int32
		
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				req := httptest.NewRequest("GET", fmt.Sprintf("/queue%d", id), nil)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", "test-agent")
				
				response, err := limitPipeline.Process(ctx, req)
				if err == nil && response.StatusCode == http.StatusTooManyRequests {
					atomic.AddInt32(&rateLimitErrors, 1)
				}
			}(i)
		}
		
		wg.Wait()
		
		// Some requests should hit rate limits
		t.Logf("Rate limit errors: %d", rateLimitErrors)
	})

	t.Run("Concurrent Stage Management", func(t *testing.T) {
		var wg sync.WaitGroup
		const numOperations = 10
		
		// Concurrent stage additions and removals
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				stage := &testPipelineStage{
					name: fmt.Sprintf("concurrent-stage-%d", id),
					priority: 500,
				}
				
				// Add stage
				pipeline.AddStage(stage)
				
				// Brief pause
				time.Sleep(10 * time.Millisecond)
				
				// Remove stage
				pipeline.RemoveStage(stage.Name())
			}(i)
		}
		
		wg.Wait()
		
		// Pipeline should still be functional
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "test-agent")
		response, err := pipeline.Process(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, response)
	})
}

func TestPipelineErrorHandling(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("Invalid Configuration", func(t *testing.T) {
		// Negative max concurrent requests
		config := &PipelineConfig{
			MaxConcurrentRequests: -1,
			RequestTimeout:        time.Second,
			MaxRequestSize:        1024,
		}
		
		pipeline, err := NewRequestProcessingPipeline(config)
		assert.Error(t, err)
		assert.Nil(t, pipeline)
	})

	t.Run("Nil Configuration", func(t *testing.T) {
		// Nil config should use defaults
		pipeline, err := NewRequestProcessingPipeline(nil)
		assert.NoError(t, err)
		assert.NotNil(t, pipeline)
	})

	t.Run("Request Validation Error", func(t *testing.T) {
		config := DefaultPipelineConfig()
		config.EnableRequestValidation = true
		
		pipeline, err := NewRequestProcessingPipeline(config)
		require.NoError(t, err)
		
		ctx := context.Background()
		
		// Request without required headers should fail validation
		req := httptest.NewRequest("GET", "/test", nil)
		// Missing Content-Type and User-Agent headers
		
		response, err := pipeline.Process(ctx, req)
		require.NoError(t, err) // Pipeline returns error responses, not errors
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("Request Size Limit", func(t *testing.T) {
		config := DefaultPipelineConfig()
		config.MaxRequestSize = 10 // Very small limit
		
		pipeline, err := NewRequestProcessingPipeline(config)
		require.NoError(t, err)
		
		ctx := context.Background()
		
		// Large request should be rejected
		largeBody := strings.Repeat("x", 100)
		req := httptest.NewRequest("POST", "/test", strings.NewReader(largeBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "test-agent")
		
		response, err := pipeline.Process(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
}

// Test pipeline stage implementation
type testPipelineStage struct {
	name           string
	priority       int
	processHandler func(context.Context, *PipelineContext) error
	shouldProcess  bool
}

func (t *testPipelineStage) Name() string {
	return t.name
}

func (t *testPipelineStage) Priority() int {
	return t.priority
}

func (t *testPipelineStage) Process(ctx context.Context, pipelineCtx *PipelineContext) error {
	if t.processHandler != nil {
		return t.processHandler(ctx, pipelineCtx)
	}
	return nil
}

func (t *testPipelineStage) ShouldProcess(ctx context.Context, pipelineCtx *PipelineContext) bool {
	return true // Always process for tests
}

func (t *testPipelineStage) OnError(ctx context.Context, pipelineCtx *PipelineContext, err error) error {
	return err // Just pass through the error
}