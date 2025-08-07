package transport

import (
	"context"
	"fmt"
	"time"
)

// ContextConfig defines standard timeout configurations for transport operations
type ContextConfig struct {
	// DefaultTimeout is the default timeout for operations without explicit timeouts
	DefaultTimeout time.Duration

	// ConnectTimeout is the timeout for establishing connections
	ConnectTimeout time.Duration

	// SendTimeout is the timeout for send operations
	SendTimeout time.Duration

	// ReceiveTimeout is the timeout for receive operations
	ReceiveTimeout time.Duration

	// ShutdownTimeout is the timeout for graceful shutdown
	ShutdownTimeout time.Duration

	// RetryTimeout is the timeout for retry operations
	RetryTimeout time.Duration
}

// DefaultContextConfig returns the default context configuration
func DefaultContextConfig() *ContextConfig {
	return &ContextConfig{
		DefaultTimeout:  30 * time.Second,
		ConnectTimeout:  30 * time.Second,
		SendTimeout:     10 * time.Second,
		ReceiveTimeout:  60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		RetryTimeout:    5 * time.Minute,
	}
}

// WithDefaultTimeout creates a context with the default timeout
func (c *ContextConfig) WithDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.DefaultTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.DefaultTimeout)
}

// WithConnectTimeout creates a context with the connect timeout
func (c *ContextConfig) WithConnectTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.ConnectTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.ConnectTimeout)
}

// WithSendTimeout creates a context with the send timeout
func (c *ContextConfig) WithSendTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.SendTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.SendTimeout)
}

// WithReceiveTimeout creates a context with the receive timeout
func (c *ContextConfig) WithReceiveTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.ReceiveTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.ReceiveTimeout)
}

// WithShutdownTimeout creates a context with the shutdown timeout
func (c *ContextConfig) WithShutdownTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.ShutdownTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.ShutdownTimeout)
}

// WithRetryTimeout creates a context with the retry timeout
func (c *ContextConfig) WithRetryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.RetryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.RetryTimeout)
}

// Validate validates the context configuration
func (c *ContextConfig) Validate() error {
	if c.DefaultTimeout < 0 {
		return fmt.Errorf("default timeout cannot be negative")
	}
	if c.ConnectTimeout < 0 {
		return fmt.Errorf("connect timeout cannot be negative")
	}
	if c.SendTimeout < 0 {
		return fmt.Errorf("send timeout cannot be negative")
	}
	if c.ReceiveTimeout < 0 {
		return fmt.Errorf("receive timeout cannot be negative")
	}
	if c.ShutdownTimeout < 0 {
		return fmt.Errorf("shutdown timeout cannot be negative")
	}
	if c.RetryTimeout < 0 {
		return fmt.Errorf("retry timeout cannot be negative")
	}
	return nil
}

// ContextAwareSleep performs a context-aware sleep that can be cancelled
func ContextAwareSleep(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WaitWithTimeout waits for a channel with a timeout
func WaitWithTimeout[T any](ctx context.Context, ch <-chan T, timeout time.Duration) (T, bool, error) {
	var zero T

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case val, ok := <-ch:
		return val, ok, nil
	case <-timeoutCtx.Done():
		if ctx.Err() != nil {
			return zero, false, ctx.Err()
		}
		return zero, false, fmt.Errorf("operation timed out after %s", timeout)
	}
}

// RetryWithContext performs an operation with retries and context support
func RetryWithContext(ctx context.Context, maxRetries int, backoff time.Duration, operation func(context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context before each attempt
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled: %w", err)
		}

		// Add exponential backoff for retries
		if attempt > 0 {
			sleepDuration := backoff * time.Duration(1<<(attempt-1))
			if err := ContextAwareSleep(ctx, sleepDuration); err != nil {
				return fmt.Errorf("retry cancelled during backoff: %w", err)
			}
		}

		// Execute the operation
		if err := operation(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}

// PropagateDeadline propagates the deadline from parent to child context if child has no deadline or a later deadline
func PropagateDeadline(parent, child context.Context) (context.Context, context.CancelFunc) {
	parentDeadline, hasParentDeadline := parent.Deadline()
	childDeadline, hasChildDeadline := child.Deadline()

	// If parent has no deadline, return child as-is
	if !hasParentDeadline {
		return context.WithCancel(child)
	}

	// If child has no deadline or parent deadline is earlier, use parent deadline
	if !hasChildDeadline || parentDeadline.Before(childDeadline) {
		return context.WithDeadline(child, parentDeadline)
	}

	// Child deadline is earlier, keep it
	return context.WithCancel(child)
}

// EnsureTimeout ensures that a context has a timeout, adding one if necessary
func EnsureTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	deadline, hasDeadline := ctx.Deadline()

	// If context already has a deadline that's sooner than our timeout, keep it
	if hasDeadline && time.Until(deadline) < timeout {
		return context.WithCancel(ctx)
	}

	// Otherwise, add our timeout
	return context.WithTimeout(ctx, timeout)
}

// ContextBestPractices provides documentation for context handling best practices
type ContextBestPractices struct{}

// Example demonstrates best practices for context usage in transport operations
func (ContextBestPractices) Example() {
	// Example 1: Always respect context cancellation in blocking operations
	_ = func(ctx context.Context) error {
		select {
		case <-time.After(5 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Example 2: Propagate timeouts properly through operations
	_ = func(ctx context.Context) error {
		sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Use sendCtx for the operation
		_ = sendCtx
		return nil
	}

	// Example 3: Use context-aware helpers for common patterns
	_ = func(ctx context.Context) error {
		// Instead of time.Sleep(5 * time.Second)
		return ContextAwareSleep(ctx, 5*time.Second)
	}
}
