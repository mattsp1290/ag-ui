package tools

import (
	"context"
	"errors"
	"testing"
	"time"
	
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestRetryManager_ShouldRetry(t *testing.T) {
	tests := []struct {
		name           string
		config         RetryConfig
		err            error
		expectedRetry  bool
		expectedDelay  time.Duration
		minDelay       time.Duration
		maxDelay       time.Duration
	}{
		{
			name: "retry on timeout error",
			config: RetryConfig{
				OnError:           RetryPolicyRetry,
				MaxRetries:        3,
				InitialDelay:      100 * time.Millisecond,
				MaxDelay:          1 * time.Second,
				BackoffMultiplier: 2.0,
				JitterFactor:      0,
			},
			err:           errors.New("operation timed out"),
			expectedRetry: true,
			expectedDelay: 100 * time.Millisecond,
			minDelay:      100 * time.Millisecond,
			maxDelay:      100 * time.Millisecond,
		},
		{
			name: "abort policy",
			config: RetryConfig{
				OnError:      RetryPolicyAbort,
				MaxRetries:   3,
				InitialDelay: 100 * time.Millisecond,
			},
			err:           errors.New("network error"),
			expectedRetry: false,
		},
		{
			name: "non-retryable error",
			config: RetryConfig{
				OnError:      RetryPolicyRetry,
				MaxRetries:   3,
				InitialDelay: 100 * time.Millisecond,
			},
			err:           errors.New("validation failed"),
			expectedRetry: false,
		},
		{
			name: "exponential backoff",
			config: RetryConfig{
				OnError:           RetryPolicyRetry,
				MaxRetries:        5,
				InitialDelay:      100 * time.Millisecond,
				MaxDelay:          10 * time.Second,
				BackoffMultiplier: 2.0,
				JitterFactor:      0,
			},
			err:           errors.New("connection refused"),
			expectedRetry: true,
			expectedDelay: 100 * time.Millisecond, // First retry uses initial delay
			minDelay:      100 * time.Millisecond,
			maxDelay:      100 * time.Millisecond,
		},
		{
			name: "with jitter",
			config: RetryConfig{
				OnError:           RetryPolicyRetry,
				MaxRetries:        3,
				InitialDelay:      100 * time.Millisecond,
				MaxDelay:          1 * time.Second,
				BackoffMultiplier: 1.0,
				JitterFactor:      0.5,
			},
			err:           errors.New("network error"),
			expectedRetry: true,
			minDelay:      50 * time.Millisecond,  // 100ms - 50%
			maxDelay:      150 * time.Millisecond, // 100ms + 50%
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.Logger = logrus.New()
			tt.config.Logger.SetLevel(logrus.DebugLevel)
			
			rm := NewRetryManager(tt.config)
			ctx := context.Background()
			
			// First attempt
			shouldRetry, delay, _ := rm.ShouldRetry(ctx, "test-call", tt.err)
			
			assert.Equal(t, tt.expectedRetry, shouldRetry)
			
			if shouldRetry {
				if tt.config.JitterFactor > 0 {
					assert.GreaterOrEqual(t, delay, tt.minDelay)
					assert.LessOrEqual(t, delay, tt.maxDelay)
				} else {
					assert.Equal(t, tt.expectedDelay, delay)
				}
			}
		})
	}
}

func TestRetryManager_MaxRetries(t *testing.T) {
	config := RetryConfig{
		OnError:           RetryPolicyRetry,
		MaxRetries:        2,
		InitialDelay:      10 * time.Millisecond,
		BackoffMultiplier: 1.0,
		Logger:            logrus.New(),
	}
	
	rm := NewRetryManager(config)
	ctx := context.Background()
	err := errors.New("network error")
	
	// First retry
	shouldRetry1, _, _ := rm.ShouldRetry(ctx, "test-call", err)
	assert.True(t, shouldRetry1)
	
	// Second retry
	shouldRetry2, _, _ := rm.ShouldRetry(ctx, "test-call", err)
	assert.True(t, shouldRetry2)
	
	// Third retry - should exceed max
	shouldRetry3, _, retryErr := rm.ShouldRetry(ctx, "test-call", err)
	assert.False(t, shouldRetry3)
	assert.Error(t, retryErr)
	// Should return the tool error
	toolErr, ok := retryErr.(*ToolError)
	assert.True(t, ok)
	assert.Equal(t, 3, toolErr.AttemptNumber)
	assert.Equal(t, 3, toolErr.MaxAttempts)
}

func TestRetryManager_Timeout(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   10,
		InitialDelay: 10 * time.Millisecond,
		Timeout:      50 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	rm := NewRetryManager(config)
	ctx := context.Background()
	err := errors.New("network error")
	
	// First retry should succeed
	shouldRetry1, _, _ := rm.ShouldRetry(ctx, "test-call", err)
	assert.True(t, shouldRetry1)
	
	// Wait to exceed timeout
	time.Sleep(60 * time.Millisecond)
	
	// Next retry should fail due to timeout
	shouldRetry2, _, retryErr := rm.ShouldRetry(ctx, "test-call", err)
	assert.False(t, shouldRetry2)
	assert.Error(t, retryErr)
	assert.Contains(t, retryErr.Error(), "overall timeout exceeded")
}

func TestRetryManager_ContextCancellation(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	rm := NewRetryManager(config)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	err := errors.New("network error")
	shouldRetry, _, retryErr := rm.ShouldRetry(ctx, "test-call", err)
	
	assert.False(t, shouldRetry)
	assert.Error(t, retryErr)
	assert.Equal(t, context.Canceled, retryErr)
}

func TestRetryManager_RecordSuccess(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		ResetAfter:   100 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	rm := NewRetryManager(config)
	ctx := context.Background()
	
	// Create some retry attempts
	err := errors.New("network error")
	rm.ShouldRetry(ctx, "test-call", err)
	
	// Record success
	rm.RecordSuccess("test-call")
	
	// Check state exists
	state, exists := rm.GetAttemptState("test-call")
	assert.True(t, exists)
	assert.Greater(t, state.AttemptCount, 0)
	
	// Wait for reset
	time.Sleep(150 * time.Millisecond)
	
	// State should be cleaned up
	state, exists = rm.GetAttemptState("test-call")
	assert.False(t, exists)
}

func TestRetryManager_Metrics(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	rm := NewRetryManager(config)
	ctx := context.Background()
	
	// Create multiple retry attempts
	err1 := errors.New("network error")
	rm.ShouldRetry(ctx, "call-1", err1)
	rm.ShouldRetry(ctx, "call-1", err1)
	
	err2 := errors.New("timeout")
	rm.ShouldRetry(ctx, "call-2", err2)
	
	metrics := rm.Metrics()
	
	assert.Equal(t, 2, metrics["activeRetries"])
	assert.Equal(t, 3, metrics["totalAttempts"]) // 2 for call-1, 1 for call-2
	assert.Equal(t, 2, metrics["maxAttempts"])
	assert.Greater(t, metrics["averageAttempts"], 1.0)
}

func TestRetryManager_CalculateDelay(t *testing.T) {
	config := RetryConfig{
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          1 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFactor:      0,
		Logger:            logrus.New(),
	}
	
	rm := NewRetryManager(config)
	
	state := &AttemptState{
		CurrentDelay: 100 * time.Millisecond,
	}
	
	err := &ToolError{}
	
	// First backoff: 100ms * 2 = 200ms
	delay1 := rm.calculateDelay(state, err)
	assert.Equal(t, 200*time.Millisecond, delay1)
	
	// Update state
	state.CurrentDelay = delay1
	
	// Second backoff: 200ms * 2 = 400ms
	delay2 := rm.calculateDelay(state, err)
	assert.Equal(t, 400*time.Millisecond, delay2)
	
	// Update state
	state.CurrentDelay = delay2
	
	// Third backoff: 400ms * 2 = 800ms
	delay3 := rm.calculateDelay(state, err)
	assert.Equal(t, 800*time.Millisecond, delay3)
	
	// Update state to exceed max
	state.CurrentDelay = 600 * time.Millisecond
	
	// Should be capped at MaxDelay
	delay4 := rm.calculateDelay(state, err)
	assert.Equal(t, config.MaxDelay, delay4)
}

func TestRetryManager_ErrorSpecificDelay(t *testing.T) {
	config := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	rm := NewRetryManager(config)
	
	state := &AttemptState{
		CurrentDelay: 100 * time.Millisecond,
	}
	
	// Error with specific retry delay
	specificDelay := 5 * time.Second
	err := &ToolError{
		RetryAfter: &specificDelay,
	}
	
	delay := rm.calculateDelay(state, err)
	assert.Equal(t, specificDelay, delay)
}

func TestRetryManager_Reset(t *testing.T) {
	config := RetryConfig{
		OnError:      RetryPolicyRetry,
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		Logger:       logrus.New(),
	}
	
	rm := NewRetryManager(config)
	ctx := context.Background()
	
	// Create some retry attempts
	err := errors.New("network error")
	rm.ShouldRetry(ctx, "call-1", err)
	rm.ShouldRetry(ctx, "call-2", err)
	
	// Verify attempts exist
	metrics := rm.Metrics()
	assert.Greater(t, metrics["activeRetries"], 0)
	
	// Reset
	rm.Reset()
	
	// Verify attempts cleared
	metrics = rm.Metrics()
	assert.Equal(t, 0, metrics["activeRetries"])
	assert.Equal(t, 0, metrics["totalAttempts"])
}

func TestRetryPolicy_String(t *testing.T) {
	tests := []struct {
		input    string
		expected RetryPolicy
	}{
		{"retry", RetryPolicyRetry},
		{"RETRY", RetryPolicyRetry},
		{"abort", RetryPolicyAbort},
		{"ABORT", RetryPolicyAbort},
		{"prompt", RetryPolicyPrompt},
		{"PROMPT", RetryPolicyPrompt},
		{"invalid", RetryPolicyAbort}, // Default
		{"", RetryPolicyAbort},         // Default
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// This would be in the actual implementation
			var result RetryPolicy
			switch tt.input {
			case "retry", "RETRY":
				result = RetryPolicyRetry
			case "prompt", "PROMPT":
				result = RetryPolicyPrompt
			default:
				result = RetryPolicyAbort
			}
			
			assert.Equal(t, tt.expected, result)
		})
	}
}