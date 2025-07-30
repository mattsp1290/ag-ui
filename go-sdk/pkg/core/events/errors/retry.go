package errors

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryPolicy defines the retry behavior
type RetryPolicy struct {
	MaxAttempts     int           `json:"max_attempts"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor"`
	Jitter          bool          `json:"jitter"`
	RetryableErrors []string      `json:"retryable_errors"`
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		RetryableErrors: []string{
			CacheErrorConnectionFailed,
			CacheErrorTimeout,
			CacheErrorL2Unavailable,
		},
	}
}

// CacheRetryPolicy returns a retry policy optimized for cache operations
func CacheRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   2,
		InitialDelay:  50 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 1.5,
		Jitter:        true,
		RetryableErrors: []string{
			CacheErrorConnectionFailed,
			CacheErrorTimeout,
			CacheErrorL2Unavailable,
			CacheErrorSerializationFailed,
		},
	}
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func(ctx context.Context, attempt int) error

// Retry executes the operation with the given retry policy
func Retry(ctx context.Context, policy *RetryPolicy, operation RetryableOperation) error {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}

	var lastErr error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		err := operation(ctx, attempt)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !policy.isRetryable(err) {
			return err
		}

		// Don't sleep after the last attempt
		if attempt == policy.MaxAttempts {
			break
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Calculate delay
		delay := policy.calculateDelay(attempt)
		
		// Wait before retry
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	// Return the last error wrapped with retry information
	return &RetryExhaustedError{
		Attempts:  policy.MaxAttempts,
		LastError: lastErr,
		Operation: "retry operation",
	}
}

// RetryExhaustedError is returned when all retry attempts are exhausted
type RetryExhaustedError struct {
	Attempts  int   `json:"attempts"`
	LastError error `json:"-"`
	Operation string `json:"operation"`
}

func (e *RetryExhaustedError) Error() string {
	return fmt.Sprintf("retry exhausted after %d attempts in %s: %v", e.Attempts, e.Operation, e.LastError)
}

func (e *RetryExhaustedError) Unwrap() error {
	return e.LastError
}

// isRetryable checks if an error should be retried according to the policy
func (p *RetryPolicy) isRetryable(err error) bool {
	// First check if the error itself is marked as retryable
	if IsRetryable(err) {
		return true
	}

	// Check if the error code is in the retryable list
	errorCode := GetErrorCode(err)
	if errorCode != "" {
		for _, retryableCode := range p.RetryableErrors {
			if errorCode == retryableCode {
				return true
			}
		}
	}

	return false
}

// calculateDelay calculates the delay for the given attempt
func (p *RetryPolicy) calculateDelay(attempt int) time.Duration {
	delay := float64(p.InitialDelay) * math.Pow(p.BackoffFactor, float64(attempt-1))
	
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}

	if p.Jitter {
		// Add up to 10% jitter using simple random
		jitterRange := delay * 0.1
		jitter := rand.Float64()*jitterRange - jitterRange/2
		delay += jitter
	}

	return time.Duration(delay)
}

// RetryWithLogging is like Retry but includes logging for each attempt
func RetryWithLogging(ctx context.Context, policy *RetryPolicy, operation RetryableOperation, logger Logger) error {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}

	var lastErr error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if logger != nil && attempt > 1 {
			logger.Logf("Retrying operation (attempt %d/%d)", attempt, policy.MaxAttempts)
		}

		err := operation(ctx, attempt)
		if err == nil {
			if logger != nil && attempt > 1 {
				logger.Logf("Operation succeeded on attempt %d", attempt)
			}
			return nil
		}

		lastErr = err

		// Log the error
		if logger != nil {
			logger.Logf("Operation failed on attempt %d: %v", attempt, err)
		}

		// Check if error is retryable
		if !policy.isRetryable(err) {
			if logger != nil {
				logger.Logf("Error is not retryable, aborting: %v", err)
			}
			return err
		}

		// Don't sleep after the last attempt
		if attempt == policy.MaxAttempts {
			break
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Calculate delay
		delay := policy.calculateDelay(attempt)
		
		if logger != nil {
			logger.Logf("Waiting %v before retry", delay)
		}

		// Wait before retry
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	// Return the last error wrapped with retry information
	retryErr := &RetryExhaustedError{
		Attempts:  policy.MaxAttempts,
		LastError: lastErr,
		Operation: "logged retry operation",
	}

	if logger != nil {
		logger.Logf("All retry attempts exhausted: %v", retryErr)
	}

	return retryErr
}