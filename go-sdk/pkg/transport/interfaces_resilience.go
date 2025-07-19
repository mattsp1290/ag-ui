package transport

import (
	"context"
	"time"
)

// RetryPolicy defines retry behavior for failed operations.
type RetryPolicy interface {
	// ShouldRetry returns true if the operation should be retried.
	ShouldRetry(attempt int, err error) bool

	// NextDelay returns the delay before the next retry attempt.
	NextDelay(attempt int) time.Duration

	// MaxAttempts returns the maximum number of retry attempts.
	MaxAttempts() int

	// Reset resets the retry policy state.
	Reset()
}

// CircuitBreaker provides circuit breaker functionality for transport operations.
type CircuitBreaker interface {
	// Execute executes an operation with circuit breaker protection.
	Execute(ctx context.Context, operation func() error) error

	// IsOpen returns true if the circuit breaker is open.
	IsOpen() bool

	// Reset resets the circuit breaker state.
	Reset()

	// GetState returns the current circuit breaker state.
	GetState() CircuitBreakerState
}

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	// CircuitClosed indicates the circuit breaker is closed (normal operation).
	CircuitClosed CircuitBreakerState = iota
	// CircuitOpen indicates the circuit breaker is open (rejecting requests).
	CircuitOpen
	// CircuitHalfOpen indicates the circuit breaker is half-open (testing).
	CircuitHalfOpen
)

// String returns the string representation of the circuit breaker state.
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}