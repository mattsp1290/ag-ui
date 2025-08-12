package tools

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
	
	"github.com/sirupsen/logrus"
)

// RetryPolicy defines how to handle retries for failed tool calls
type RetryPolicy string

const (
	RetryPolicyRetry  RetryPolicy = "retry"
	RetryPolicyAbort  RetryPolicy = "abort"
	RetryPolicyPrompt RetryPolicy = "prompt"
)

// RetryConfig configures retry behavior for tool calls
type RetryConfig struct {
	// OnError defines the policy when errors occur
	OnError RetryPolicy
	
	// MaxRetries is the maximum number of retry attempts (0 = no retries)
	MaxRetries int
	
	// InitialDelay is the initial delay before first retry
	InitialDelay time.Duration
	
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	
	// BackoffMultiplier for exponential backoff (e.g., 2.0)
	BackoffMultiplier float64
	
	// JitterFactor adds randomness to delays (0.0 to 1.0)
	JitterFactor float64
	
	// Timeout is the overall timeout for all attempts
	Timeout time.Duration
	
	// PerAttemptTimeout is the timeout for each individual attempt
	PerAttemptTimeout time.Duration
	
	// ResetAfter resets the backoff after this duration of success
	ResetAfter time.Duration
	
	// Logger for retry events
	Logger *logrus.Logger
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		OnError:           RetryPolicyAbort,
		MaxRetries:        3,
		InitialDelay:      500 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.2,
		Timeout:           5 * time.Minute,
		PerAttemptTimeout: 30 * time.Second,
		ResetAfter:        60 * time.Second,
	}
}

// RetryManager manages retry logic for tool calls
type RetryManager struct {
	config     RetryConfig
	classifier *ErrorClassifier
	logger     *logrus.Logger
	
	// Track retry state per tool call
	attempts map[string]*AttemptState
	mu       sync.RWMutex
}

// AttemptState tracks the state of retry attempts for a tool call
type AttemptState struct {
	ToolCallID     string
	ToolName       string
	AttemptCount   int
	LastAttempt    time.Time
	FirstAttempt   time.Time
	LastError      *ToolError
	CurrentDelay   time.Duration
	TotalDuration  time.Duration
}

// NewRetryManager creates a new retry manager
func NewRetryManager(config RetryConfig) *RetryManager {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}
	
	return &RetryManager{
		config:     config,
		classifier: NewErrorClassifier(),
		logger:     config.Logger,
		attempts:   make(map[string]*AttemptState),
	}
}

// ShouldRetry determines if a tool call should be retried
func (rm *RetryManager) ShouldRetry(ctx context.Context, toolCallID string, err error) (bool, time.Duration, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	// Get or create attempt state
	state, exists := rm.attempts[toolCallID]
	if !exists {
		state = &AttemptState{
			ToolCallID:   toolCallID,
			FirstAttempt: time.Now(),
			CurrentDelay: rm.config.InitialDelay,
		}
		rm.attempts[toolCallID] = state
	}
	
	// Classify the error
	toolErr := rm.classifier.Classify(err, state.ToolName, toolCallID)
	state.LastError = toolErr
	state.LastAttempt = time.Now()
	state.AttemptCount++
	
	// Update error with attempt information
	toolErr.AttemptNumber = state.AttemptCount
	toolErr.MaxAttempts = rm.config.MaxRetries + 1 // +1 for initial attempt
	
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return false, 0, ctx.Err()
	default:
	}
	
	// Check policy
	switch rm.config.OnError {
	case RetryPolicyAbort:
		rm.logger.WithFields(logrus.Fields{
			"toolCallId": toolCallID,
			"error":      toolErr.Error(),
			"policy":     "abort",
		}).Info("Aborting due to policy")
		return false, 0, toolErr
		
	case RetryPolicyPrompt:
		// In non-interactive mode, fall back to retry policy
		// TODO: Implement interactive prompt
		rm.logger.WithFields(logrus.Fields{
			"toolCallId": toolCallID,
			"error":      toolErr.Error(),
			"policy":     "prompt",
		}).Info("Prompt policy not yet implemented, falling back to retry")
		// Fall through to retry logic
		
	case RetryPolicyRetry:
		// Continue with retry logic
	}
	
	// Check if error is retryable
	if !toolErr.ShouldRetry() {
		rm.logger.WithFields(logrus.Fields{
			"toolCallId": toolCallID,
			"error":      toolErr.Error(),
			"retryable":  false,
		}).Info("Error is not retryable")
		return false, 0, toolErr
	}
	
	// Check retry limits
	if rm.config.MaxRetries > 0 && state.AttemptCount > rm.config.MaxRetries {
		rm.logger.WithFields(logrus.Fields{
			"toolCallId":   toolCallID,
			"attemptCount": state.AttemptCount,
			"maxRetries":   rm.config.MaxRetries,
		}).Info("Max retry attempts exceeded")
		toolErr.IsRetryable = false
		return false, 0, toolErr
	}
	
	// Check overall timeout
	if rm.config.Timeout > 0 {
		elapsed := time.Since(state.FirstAttempt)
		if elapsed > rm.config.Timeout {
			rm.logger.WithFields(logrus.Fields{
				"toolCallId": toolCallID,
				"elapsed":    elapsed,
				"timeout":    rm.config.Timeout,
			}).Info("Overall timeout exceeded")
			return false, 0, fmt.Errorf("overall timeout exceeded: %v", rm.config.Timeout)
		}
	}
	
	// Calculate retry delay
	delay := rm.calculateDelay(state, toolErr)
	state.CurrentDelay = delay
	
	rm.logger.WithFields(logrus.Fields{
		"toolCallId":   toolCallID,
		"attemptCount": state.AttemptCount,
		"delay":        delay,
		"errorCode":    toolErr.Code,
	}).Info("Will retry after delay")
	
	return true, delay, nil
}

// calculateDelay calculates the retry delay with exponential backoff and jitter
func (rm *RetryManager) calculateDelay(state *AttemptState, err *ToolError) time.Duration {
	// Use error-specific retry delay if available
	if err.RetryAfter != nil {
		return *err.RetryAfter
	}
	
	// For first retry, use initial delay without backoff
	if state.AttemptCount == 1 {
		delay := rm.config.InitialDelay
		// Apply jitter even on first retry
		if rm.config.JitterFactor > 0 {
			jitter := rm.config.JitterFactor * float64(delay)
			minDelay := float64(delay) - jitter
			maxDelay := float64(delay) + jitter
			
			// Ensure minimum delay is positive
			if minDelay < 0 {
				minDelay = 0
			}
			
			// Random delay between min and max
			delay = time.Duration(minDelay + rand.Float64()*(maxDelay-minDelay))
		}
		return delay
	}
	
	// Calculate exponential backoff for subsequent retries
	delay := state.CurrentDelay
	
	// Apply exponential backoff
	if rm.config.BackoffMultiplier > 1 {
		delay = time.Duration(float64(delay) * rm.config.BackoffMultiplier)
	}
	
	// Cap at max delay
	if delay > rm.config.MaxDelay {
		delay = rm.config.MaxDelay
	}
	
	// Apply jitter
	if rm.config.JitterFactor > 0 {
		jitter := rm.config.JitterFactor * float64(delay)
		minDelay := float64(delay) - jitter
		maxDelay := float64(delay) + jitter
		
		// Ensure minimum delay is positive
		if minDelay < 0 {
			minDelay = 0
		}
		
		// Random delay between min and max
		delay = time.Duration(minDelay + rand.Float64()*(maxDelay-minDelay))
	}
	
	return delay
}

// RecordSuccess records a successful tool call
func (rm *RetryManager) RecordSuccess(toolCallID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	if state, exists := rm.attempts[toolCallID]; exists {
		state.TotalDuration = time.Since(state.FirstAttempt)
		
		rm.logger.WithFields(logrus.Fields{
			"toolCallId":    toolCallID,
			"attemptCount":  state.AttemptCount,
			"totalDuration": state.TotalDuration,
		}).Info("Tool call succeeded")
		
		// Clean up after success
		// Keep state for a short time for telemetry
		go func() {
			time.Sleep(rm.config.ResetAfter)
			rm.mu.Lock()
			delete(rm.attempts, toolCallID)
			rm.mu.Unlock()
		}()
	}
}

// GetAttemptState returns the current attempt state for a tool call
func (rm *RetryManager) GetAttemptState(toolCallID string) (*AttemptState, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	state, exists := rm.attempts[toolCallID]
	if exists {
		// Return a copy to prevent external modification
		stateCopy := *state
		return &stateCopy, true
	}
	return nil, false
}

// Reset clears all retry state
func (rm *RetryManager) Reset() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	rm.attempts = make(map[string]*AttemptState)
	rm.logger.Info("Retry manager state reset")
}

// Metrics returns current retry metrics
func (rm *RetryManager) Metrics() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	activeRetries := 0
	totalAttempts := 0
	maxAttempts := 0
	
	for _, state := range rm.attempts {
		activeRetries++
		totalAttempts += state.AttemptCount
		if state.AttemptCount > maxAttempts {
			maxAttempts = state.AttemptCount
		}
	}
	
	avgAttempts := 0.0
	if activeRetries > 0 {
		avgAttempts = float64(totalAttempts) / float64(activeRetries)
	}
	
	return map[string]interface{}{
		"activeRetries":   activeRetries,
		"totalAttempts":   totalAttempts,
		"maxAttempts":     maxAttempts,
		"averageAttempts": math.Round(avgAttempts*100) / 100,
	}
}