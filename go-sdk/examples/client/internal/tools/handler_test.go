package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockToolExecutor is a mock implementation of ToolExecutor for testing
type MockToolExecutor struct {
	mu           sync.Mutex
	calls        []ToolCallRequest
	responses    map[string]*ToolCallResponse
	errors       map[string]error
	callCount    map[string]int
	failUntil    map[string]int // Fail until Nth attempt
	delay        time.Duration
}

func NewMockToolExecutor() *MockToolExecutor {
	return &MockToolExecutor{
		responses: make(map[string]*ToolCallResponse),
		errors:    make(map[string]error),
		callCount: make(map[string]int),
		failUntil: make(map[string]int),
	}
}

func (m *MockToolExecutor) Execute(ctx context.Context, request *ToolCallRequest) (*ToolCallResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.calls = append(m.calls, *request)
	m.callCount[request.ToolCallID]++
	
	// Simulate delay if configured
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	// Check if we should fail until Nth attempt
	if failUntil, ok := m.failUntil[request.ToolCallID]; ok {
		if m.callCount[request.ToolCallID] < failUntil {
			if err, ok := m.errors[request.ToolCallID]; ok {
				return nil, err
			}
			return nil, errors.New("simulated failure")
		}
		// Success on Nth attempt - remove the error
		delete(m.errors, request.ToolCallID)
	}
	
	// Return configured error if exists
	if err, ok := m.errors[request.ToolCallID]; ok {
		return nil, err
	}
	
	// Return configured response if exists
	if resp, ok := m.responses[request.ToolCallID]; ok {
		return resp, nil
	}
	
	// Default success response
	return &ToolCallResponse{
		ToolCallID: request.ToolCallID,
		ToolName:   request.ToolName,
		Result:     map[string]interface{}{"status": "success"},
	}, nil
}

func (m *MockToolExecutor) GetCallCount(toolCallID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount[toolCallID]
}

func TestToolHandler_SuccessfulExecution(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	executor := NewMockToolExecutor()
	handler := NewToolHandler(config, executor)
	
	request := &ToolCallRequest{
		ToolCallID: "test-call-1",
		ToolName:   "test_tool",
		Arguments: map[string]interface{}{
			"param1": "value1",
		},
	}
	
	ctx := context.Background()
	response, err := handler.HandleToolCall(ctx, request)
	
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "test-call-1", response.ToolCallID)
	assert.Equal(t, "test_tool", response.ToolName)
	assert.NotNil(t, response.Result)
	assert.Nil(t, response.Error)
	assert.Greater(t, response.Duration, time.Duration(0))
	
	// Check metrics
	metrics := handler.GetMetrics()
	assert.Equal(t, int64(1), metrics["totalCalls"])
	assert.Equal(t, int64(1), metrics["successfulCalls"])
	assert.Equal(t, int64(0), metrics["failedCalls"])
}

func TestToolHandler_RetryOnFailure(t *testing.T) {
	config := RetryConfig{
		OnError:           RetryPolicyRetry,
		MaxRetries:        3,
		InitialDelay:      10 * time.Millisecond,
		BackoffMultiplier: 1.0, // No backoff for faster tests
		Logger:            logrus.New(),
	}
	
	executor := NewMockToolExecutor()
	// Fail first 2 attempts, succeed on 3rd
	executor.failUntil["retry-call"] = 3
	executor.errors["retry-call"] = errors.New("network error")
	
	handler := NewToolHandler(config, executor)
	
	request := &ToolCallRequest{
		ToolCallID: "retry-call",
		ToolName:   "retry_tool",
	}
	
	ctx := context.Background()
	response, err := handler.HandleToolCall(ctx, request)
	
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 3, executor.GetCallCount("retry-call")) // 1 initial + 2 retries
	
	// Check metrics
	metrics := handler.GetMetrics()
	assert.Equal(t, int64(1), metrics["retriedCalls"])
}

func TestToolHandler_MaxRetriesExceeded(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	executor := NewMockToolExecutor()
	// Always fail
	executor.errors["fail-call"] = errors.New("network error")
	
	handler := NewToolHandler(config, executor)
	
	request := &ToolCallRequest{
		ToolCallID: "fail-call",
		ToolName:   "fail_tool",
	}
	
	ctx := context.Background()
	response, err := handler.HandleToolCall(ctx, request)
	
	require.Error(t, err)
	assert.NotNil(t, response)
	assert.NotNil(t, response.Error)
	assert.Equal(t, 3, executor.GetCallCount("fail-call")) // 1 initial + 2 retries
	
	// Check error details
	toolErr, ok := err.(*ToolError)
	require.True(t, ok)
	assert.Equal(t, ErrorCodeNetwork, toolErr.Code)
	assert.True(t, toolErr.IsRetryable)
}

func TestToolHandler_AbortPolicy(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyAbort,
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	executor := NewMockToolExecutor()
	executor.errors["abort-call"] = errors.New("network error")
	
	handler := NewToolHandler(config, executor)
	
	request := &ToolCallRequest{
		ToolCallID: "abort-call",
		ToolName:   "abort_tool",
	}
	
	ctx := context.Background()
	response, err := handler.HandleToolCall(ctx, request)
	
	require.Error(t, err)
	assert.NotNil(t, response)
	assert.NotNil(t, response.Error)
	assert.Equal(t, 1, executor.GetCallCount("abort-call")) // No retries
}

func TestToolHandler_Idempotency(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	executor := NewMockToolExecutor()
	executor.delay = 50 * time.Millisecond // Slow execution
	
	handler := NewToolHandler(config, executor)
	
	request := &ToolCallRequest{
		ToolCallID: "idempotent-call",
		ToolName:   "idempotent_tool",
	}
	
	ctx := context.Background()
	var wg sync.WaitGroup
	var responses []*ToolCallResponse
	var errors []error
	var mu sync.Mutex
	
	// Make 3 concurrent calls with same tool call ID
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := handler.HandleToolCall(ctx, request)
			mu.Lock()
			responses = append(responses, resp)
			errors = append(errors, err)
			mu.Unlock()
		}()
	}
	
	wg.Wait()
	
	// All should succeed with same response
	for _, err := range errors {
		assert.NoError(t, err)
	}
	
	// Only one actual execution should have occurred
	assert.Equal(t, 1, executor.GetCallCount("idempotent-call"))
	
	// All responses should be identical
	for i := 1; i < len(responses); i++ {
		assert.Equal(t, responses[0].ToolCallID, responses[i].ToolCallID)
		assert.Equal(t, responses[0].ToolName, responses[i].ToolName)
	}
}

func TestToolHandler_ContextTimeout(t *testing.T) {
	config := RetryConfig{
		OnError:           RetryPolicyRetry,
		MaxRetries:        3,
		InitialDelay:      10 * time.Millisecond,
		PerAttemptTimeout: 50 * time.Millisecond,
		Logger:            logrus.New(),
	}
	
	executor := NewMockToolExecutor()
	executor.delay = 100 * time.Millisecond // Longer than timeout
	
	handler := NewToolHandler(config, executor)
	
	request := &ToolCallRequest{
		ToolCallID: "timeout-call",
		ToolName:   "timeout_tool",
	}
	
	ctx := context.Background()
	_, err := handler.HandleToolCall(ctx, request)
	
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

func TestToolHandler_NonRetryableError(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	executor := NewMockToolExecutor()
	executor.errors["validation-call"] = errors.New("validation failed: invalid input")
	
	handler := NewToolHandler(config, executor)
	
	request := &ToolCallRequest{
		ToolCallID: "validation-call",
		ToolName:   "validation_tool",
	}
	
	ctx := context.Background()
	response, err := handler.HandleToolCall(ctx, request)
	
	require.Error(t, err)
	assert.NotNil(t, response)
	assert.NotNil(t, response.Error)
	assert.Equal(t, 1, executor.GetCallCount("validation-call")) // No retries
	
	// Check error classification
	toolErr, ok := err.(*ToolError)
	require.True(t, ok)
	assert.Equal(t, ErrorCodeValidation, toolErr.Code)
	assert.False(t, toolErr.IsRetryable)
}

func TestToolHandler_CancelToolCall(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	executor := NewMockToolExecutor()
	executor.delay = 100 * time.Millisecond
	
	handler := NewToolHandler(config, executor)
	
	request := &ToolCallRequest{
		ToolCallID: "cancel-call",
		ToolName:   "cancel_tool",
	}
	
	// Start execution in background
	ctx := context.Background()
	done := make(chan bool)
	
	go func() {
		_, _ = handler.HandleToolCall(ctx, request)
		done <- true
	}()
	
	// Give it time to start
	time.Sleep(20 * time.Millisecond)
	
	// Cancel the call
	cancelErr := handler.CancelToolCall("cancel-call")
	assert.NoError(t, cancelErr)
	
	// Wait for completion
	<-done
	
	// Check that call was marked as cancelled
	handler.mu.RLock()
	state, exists := handler.activeCalls["cancel-call"]
	handler.mu.RUnlock()
	
	assert.True(t, exists)
	assert.True(t, state.IsCancelled)
}

func TestToolHandler_Metrics(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	executor := NewMockToolExecutor()
	handler := NewToolHandler(config, executor)
	
	// Successful call
	request1 := &ToolCallRequest{
		ToolCallID: "success-1",
		ToolName:   "tool1",
	}
	handler.HandleToolCall(context.Background(), request1)
	
	// Failed call (non-retryable)
	executor.errors["fail-1"] = errors.New("validation error")
	request2 := &ToolCallRequest{
		ToolCallID: "fail-1",
		ToolName:   "tool2",
	}
	handler.HandleToolCall(context.Background(), request2)
	
	// Retried call
	executor.failUntil["retry-1"] = 2
	executor.errors["retry-1"] = errors.New("network error")
	request3 := &ToolCallRequest{
		ToolCallID: "retry-1",
		ToolName:   "tool3",
	}
	handler.HandleToolCall(context.Background(), request3)
	
	// Check metrics
	metrics := handler.GetMetrics()
	assert.Equal(t, int64(3), metrics["totalCalls"])
	assert.Equal(t, int64(2), metrics["successfulCalls"])
	assert.Equal(t, int64(1), metrics["failedCalls"])
	assert.Equal(t, int64(1), metrics["retriedCalls"])
	
	// Test JSON export
	jsonData, err := handler.MetricsJSON(false)
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), `"totalCalls":3`)
}

func TestToolHandler_Reset(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	executor := NewMockToolExecutor()
	handler := NewToolHandler(config, executor)
	
	// Make some calls
	request := &ToolCallRequest{
		ToolCallID: "test-call",
		ToolName:   "test_tool",
	}
	handler.HandleToolCall(context.Background(), request)
	
	// Verify metrics exist
	metrics := handler.GetMetrics()
	assert.Greater(t, metrics["totalCalls"], int64(0))
	
	// Reset
	handler.Reset()
	
	// Verify metrics cleared
	metrics = handler.GetMetrics()
	assert.Equal(t, int64(0), metrics["totalCalls"])
	assert.Equal(t, int64(0), metrics["successfulCalls"])
	assert.Equal(t, int64(0), metrics["failedCalls"])
}

func TestToolHandler_ConcurrentExecution(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	executor := NewMockToolExecutor()
	handler := NewToolHandler(config, executor)
	
	// Run many concurrent tool calls
	var wg sync.WaitGroup
	numCalls := 50
	successCount := int64(0)
	
	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			request := &ToolCallRequest{
				ToolCallID: fmt.Sprintf("concurrent-%d", id),
				ToolName:   "concurrent_tool",
			}
			_, err := handler.HandleToolCall(context.Background(), request)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	// All should succeed
	assert.Equal(t, int64(numCalls), successCount)
	
	// Check metrics
	metrics := handler.GetMetrics()
	assert.Equal(t, int64(numCalls), metrics["totalCalls"])
	assert.Equal(t, int64(numCalls), metrics["successfulCalls"])
}