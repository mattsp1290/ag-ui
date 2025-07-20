package common

import (
	"context"
	"time"
)

// TimeoutConfig defines standard timeout configurations for testing
type TimeoutConfig struct {
	// Short timeout for simple operations (connection establishment, sends)
	Short time.Duration
	// Medium timeout for operations that may take time (event processing)
	Medium time.Duration
	// Long timeout for complex operations (load tests, integration tests)
	Long time.Duration
}

// DefaultTimeoutConfig returns optimized timeouts for different environments
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Short:  2 * time.Second,  // For quick operations
		Medium: 5 * time.Second,  // For normal operations
		Long:   15 * time.Second, // For complex operations
	}
}

// TestTimeoutConfig returns faster timeouts for unit tests
func TestTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Short:  1 * time.Second,   // For quick operations
		Medium: 3 * time.Second,   // For normal operations
		Long:   8 * time.Second,   // For complex operations
	}
}

// IntegrationTimeoutConfig returns appropriate timeouts for integration tests
func IntegrationTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Short:  3 * time.Second,   // For quick operations
		Medium: 8 * time.Second,   // For normal operations
		Long:   20 * time.Second,  // For complex operations
	}
}

// WithShortTimeout creates a context with a short timeout
func WithShortTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	config := DefaultTimeoutConfig()
	return context.WithTimeout(parent, config.Short)
}

// WithMediumTimeout creates a context with a medium timeout
func WithMediumTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	config := DefaultTimeoutConfig()
	return context.WithTimeout(parent, config.Medium)
}

// WithLongTimeout creates a context with a long timeout
func WithLongTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	config := DefaultTimeoutConfig()
	return context.WithTimeout(parent, config.Long)
}

// WithCustomTimeout creates a context with a custom timeout based on operation type
func WithCustomTimeout(parent context.Context, operation string) (context.Context, context.CancelFunc) {
	config := DefaultTimeoutConfig()
	
	switch operation {
	case "connect", "send", "close":
		return context.WithTimeout(parent, config.Short)
	case "receive", "process", "validate":
		return context.WithTimeout(parent, config.Medium)
	case "integration", "load", "benchmark":
		return context.WithTimeout(parent, config.Long)
	default:
		return context.WithTimeout(parent, config.Medium)
	}
}

// TimeoutHandler provides graceful timeout handling with cleanup
type TimeoutHandler struct {
	operation string
	timeout   time.Duration
	cleanup   func()
}

// NewTimeoutHandler creates a new timeout handler
func NewTimeoutHandler(operation string, timeout time.Duration, cleanup func()) *TimeoutHandler {
	return &TimeoutHandler{
		operation: operation,
		timeout:   timeout,
		cleanup:   cleanup,
	}
}

// Execute runs the given function with timeout handling and cleanup
func (th *TimeoutHandler) Execute(ctx context.Context, fn func(context.Context) error) error {
	// Create a context with the specified timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, th.timeout)
	defer cancel()
	
	// Channel to receive the result
	resultChan := make(chan error, 1)
	
	// Run the function in a goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- NewTransportError(ErrorTypeInternal, 
					"panic during operation", nil).WithMetadata("panic", r)
			}
		}()
		
		resultChan <- fn(timeoutCtx)
	}()
	
	// Wait for either completion or timeout
	select {
	case err := <-resultChan:
		return err
	case <-timeoutCtx.Done():
		// Timeout occurred, run cleanup if provided
		if th.cleanup != nil {
			th.cleanup()
		}
		
		return NewTimeoutError(th.operation, th.timeout, th.timeout)
	}
}

// RetryableTimeoutHandler provides timeout handling with retry logic
type RetryableTimeoutHandler struct {
	*TimeoutHandler
	maxRetries int
	retryDelay time.Duration
}

// NewRetryableTimeoutHandler creates a new retryable timeout handler
func NewRetryableTimeoutHandler(operation string, timeout time.Duration, maxRetries int, retryDelay time.Duration, cleanup func()) *RetryableTimeoutHandler {
	return &RetryableTimeoutHandler{
		TimeoutHandler: NewTimeoutHandler(operation, timeout, cleanup),
		maxRetries:     maxRetries,
		retryDelay:     retryDelay,
	}
}

// Execute runs the function with retry logic on timeout
func (rth *RetryableTimeoutHandler) Execute(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error
	
	for attempt := 0; attempt <= rth.maxRetries; attempt++ {
		// Execute with timeout
		err := rth.TimeoutHandler.Execute(ctx, fn)
		
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if this is a timeout error and we have retries left
		if IsTimeoutError(err) && attempt < rth.maxRetries {
			// Wait before retrying
			select {
			case <-time.After(rth.retryDelay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		
		// If not a timeout error or no retries left, return the error
		break
	}
	
	return lastErr
}

// Note: IsTimeoutError is defined in errors.go to avoid duplication

// WaitWithTimeout waits for a channel with timeout and proper cleanup
func WaitWithTimeout[T any](ctx context.Context, ch <-chan T, timeout time.Duration, operation string) (T, error) {
	var zero T
	
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	select {
	case result := <-ch:
		return result, nil
	case <-timeoutCtx.Done():
		return zero, NewTimeoutError(operation, timeout, timeout)
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

// WaitWithTimeoutAndCleanup waits for a channel with timeout and runs cleanup on timeout
func WaitWithTimeoutAndCleanup[T any](ctx context.Context, ch <-chan T, timeout time.Duration, operation string, cleanup func()) (T, error) {
	var zero T
	
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	select {
	case result := <-ch:
		return result, nil
	case <-timeoutCtx.Done():
		if cleanup != nil {
			cleanup()
		}
		return zero, NewTimeoutError(operation, timeout, timeout)
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}