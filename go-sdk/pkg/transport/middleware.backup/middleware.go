package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/transport"
)

// TransportEvent represents an event for middleware (avoiding generic types)
type TransportEvent interface {
	ID() string
	Type() string
	Timestamp() time.Time
	Data() map[string]interface{}
}

// Chain represents a middleware chain
type Chain struct {
	middlewares []transport.Middleware
}

// NewChain creates a new middleware chain
func NewChain(middlewares ...transport.Middleware) *Chain {
	return &Chain{
		middlewares: middlewares,
	}
}

// Add adds middleware to the chain
func (c *Chain) Add(middleware transport.Middleware) *Chain {
	c.middlewares = append(c.middlewares, middleware)
	return c
}

// Wrap wraps a transport with the middleware chain
func (c *Chain) Wrap(transport transport.Transport) transport.Transport {
	// Apply middleware in reverse order so the first middleware is the outermost
	result := transport
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		result = c.middlewares[i].Wrap(result)
	}
	return result
}

// LoggingMiddleware logs transport operations
type LoggingMiddleware struct {
	logger Logger
}

// Logger interface for logging
type Logger interface {
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Debug(msg string, fields ...Field)
}

// Field represents a log field
type Field struct {
	Key   string
	Value interface{}
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(logger Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

// Wrap implements the Middleware interface
func (m *LoggingMiddleware) Wrap(transport transport.Transport) transport.Transport {
	return &loggingTransport{
		Transport: transport,
		logger:    m.logger,
	}
}

// loggingTransport wraps a transport with logging
type loggingTransport struct {
	transport.Transport
	logger Logger
}

// Connect logs connection attempts
func (t *loggingTransport) Connect(ctx context.Context) error {
	t.logger.Info("transport connecting")
	
	start := time.Now()
	err := t.Transport.Connect(ctx)
	duration := time.Since(start)
	
	if err != nil {
		t.logger.Error("transport connection failed", 
			Field{Key: "error", Value: err.Error()},
			Field{Key: "duration", Value: duration})
		return err
	}
	
	t.logger.Info("transport connected successfully", 
		Field{Key: "duration", Value: duration})
	return nil
}

// Close logs connection close
func (t *loggingTransport) Close(ctx context.Context) error {
	t.logger.Info("transport closing")
	
	err := t.Transport.Close(ctx)
	if err != nil {
		t.logger.Error("transport close failed", 
			Field{Key: "error", Value: err.Error()})
		return err
	}
	
	t.logger.Info("transport closed successfully")
	return nil
}

// Send logs message sending
func (t *loggingTransport) Send(ctx context.Context, event transport.TransportEvent) error {
	t.logger.Debug("sending event", 
		Field{Key: "event_type", Value: event.Type()},
		Field{Key: "event_id", Value: event.ID()})
	
	start := time.Now()
	err := t.Transport.Send(ctx, event)
	duration := time.Since(start)
	
	if err != nil {
		t.logger.Error("send failed", 
			Field{Key: "error", Value: err.Error()},
			Field{Key: "duration", Value: duration},
			Field{Key: "event_id", Value: event.ID()})
		return err
	}
	
	t.logger.Debug("send successful", 
		Field{Key: "duration", Value: duration},
		Field{Key: "event_id", Value: event.ID()})
	return nil
}

// MetricsMiddleware collects transport metrics
type MetricsMiddleware struct {
	metrics *MetricsCollector
}

// MetricsCollector collects transport metrics
type MetricsCollector struct {
	mu                sync.RWMutex
	ConnectionAttempts uint64
	ConnectionErrors   uint64
	SendAttempts      uint64
	SendErrors        uint64
	SendDuration      time.Duration
	ReceiveCount      uint64
	ErrorCount        uint64
	LastActivity      time.Time
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware() *MetricsMiddleware {
	return &MetricsMiddleware{
		metrics: &MetricsCollector{},
	}
}

// GetMetrics returns collected metrics
func (m *MetricsMiddleware) GetMetrics() *MetricsCollector {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()
	
	// Return a copy
	return &MetricsCollector{
		ConnectionAttempts: m.metrics.ConnectionAttempts,
		ConnectionErrors:   m.metrics.ConnectionErrors,
		SendAttempts:      m.metrics.SendAttempts,
		SendErrors:        m.metrics.SendErrors,
		SendDuration:      m.metrics.SendDuration,
		ReceiveCount:      m.metrics.ReceiveCount,
		ErrorCount:        m.metrics.ErrorCount,
		LastActivity:      m.metrics.LastActivity,
	}
}

// Wrap implements the Middleware interface
func (m *MetricsMiddleware) Wrap(transport transport.Transport) transport.Transport {
	return &metricsTransport{
		Transport: transport,
		metrics:   m.metrics,
	}
}

// metricsTransport wraps a transport with metrics collection
type metricsTransport struct {
	transport.Transport
	metrics *MetricsCollector
}

// Connect collects connection metrics
func (t *metricsTransport) Connect(ctx context.Context) error {
	t.metrics.mu.Lock()
	t.metrics.ConnectionAttempts++
	t.metrics.LastActivity = time.Now()
	t.metrics.mu.Unlock()
	
	err := t.Transport.Connect(ctx)
	if err != nil {
		t.metrics.mu.Lock()
		t.metrics.ConnectionErrors++
		t.metrics.mu.Unlock()
	}
	
	return err
}

// Send collects send metrics
func (t *metricsTransport) Send(ctx context.Context, event transport.TransportEvent) error {
	start := time.Now()
	
	t.metrics.mu.Lock()
	t.metrics.SendAttempts++
	t.metrics.LastActivity = time.Now()
	t.metrics.mu.Unlock()
	
	err := t.Transport.Send(ctx, event)
	duration := time.Since(start)
	
	t.metrics.mu.Lock()
	t.metrics.SendDuration += duration
	if err != nil {
		t.metrics.SendErrors++
	}
	t.metrics.mu.Unlock()
	
	return err
}

// RetryMiddleware provides retry functionality
type RetryMiddleware struct {
	maxRetries     int
	retryDelay     time.Duration
	backoffFactor  float64
	retryableErrors []error
}

// NewRetryMiddleware creates a new retry middleware
func NewRetryMiddleware(maxRetries int, retryDelay time.Duration, backoffFactor float64) *RetryMiddleware {
	return &RetryMiddleware{
		maxRetries:    maxRetries,
		retryDelay:    retryDelay,
		backoffFactor: backoffFactor,
		retryableErrors: []error{
			transport.ErrConnectionFailed,
			transport.ErrTimeout,
			transport.ErrConnectionClosed,
		},
	}
}

// Wrap implements the Middleware interface
func (m *RetryMiddleware) Wrap(transport transport.Transport) transport.Transport {
	return &retryTransport{
		Transport:       transport,
		maxRetries:      m.maxRetries,
		retryDelay:      m.retryDelay,
		backoffFactor:   m.backoffFactor,
		retryableErrors: m.retryableErrors,
	}
}

// retryTransport wraps a transport with retry logic
type retryTransport struct {
	transport.Transport
	maxRetries      int
	retryDelay      time.Duration
	backoffFactor   float64
	retryableErrors []error
}

// Connect implements retry logic for connection
func (t *retryTransport) Connect(ctx context.Context) error {
	var lastErr error
	delay := t.retryDelay
	
	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				delay = time.Duration(float64(delay) * t.backoffFactor)
			}
		}
		
		err := t.Transport.Connect(ctx)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !t.isRetryableError(err) {
			break
		}
	}
	
	return fmt.Errorf("connect failed after %d attempts: %w", t.maxRetries+1, lastErr)
}

// Send implements retry logic for sending
func (t *retryTransport) Send(ctx context.Context, event transport.TransportEvent) error {
	var lastErr error
	delay := t.retryDelay
	
	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				delay = time.Duration(float64(delay) * t.backoffFactor)
			}
		}
		
		err := t.Transport.Send(ctx, event)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !t.isRetryableError(err) {
			break
		}
	}
	
	return fmt.Errorf("send failed after %d attempts: %w", t.maxRetries+1, lastErr)
}

// isRetryableError checks if an error is retryable
func (t *retryTransport) isRetryableError(err error) bool {
	for _, retryableErr := range t.retryableErrors {
		if err == retryableErr {
			return true
		}
	}
	
	// Check for transport errors
	if te, ok := err.(*transport.TransportError); ok {
		return te.IsRetryable()
	}
	
	return false
}

// AuthenticationMiddleware provides authentication functionality
type AuthenticationMiddleware struct {
	tokenProvider TokenProvider
}

// TokenProvider provides authentication tokens
type TokenProvider interface {
	GetToken(ctx context.Context) (string, error)
	RefreshToken(ctx context.Context) (string, error)
}

// NewAuthenticationMiddleware creates a new authentication middleware
func NewAuthenticationMiddleware(tokenProvider TokenProvider) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{
		tokenProvider: tokenProvider,
	}
}

// Wrap implements the Middleware interface
func (m *AuthenticationMiddleware) Wrap(transport transport.Transport) transport.Transport {
	return &authTransport{
		Transport:     transport,
		tokenProvider: m.tokenProvider,
	}
}

// authTransport wraps a transport with authentication
type authTransport struct {
	transport.Transport
	tokenProvider TokenProvider
	currentToken  string
	mu           sync.RWMutex
}

// Connect adds authentication to connection
func (t *authTransport) Connect(ctx context.Context) error {
	// Get authentication token
	token, err := t.tokenProvider.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get authentication token: %w", err)
	}
	
	t.mu.Lock()
	t.currentToken = token
	t.mu.Unlock()
	
	// Connect with authentication
	return t.Transport.Connect(ctx)
}

// Send adds authentication to events
func (t *authTransport) Send(ctx context.Context, event transport.TransportEvent) error {
	t.mu.RLock()
	token := t.currentToken
	t.mu.RUnlock()
	
	if token == "" {
		return fmt.Errorf("no authentication token available")
	}
	
	// Add authentication to event (this would be implementation-specific)
	return t.Transport.Send(ctx, event)
}

// CompressionMiddleware provides compression functionality
type CompressionMiddleware struct {
	compressionType transport.CompressionType
	level          int
	minSize        int64
}

// NewCompressionMiddleware creates a new compression middleware
func NewCompressionMiddleware(compressionType transport.CompressionType, level int, minSize int64) *CompressionMiddleware {
	return &CompressionMiddleware{
		compressionType: compressionType,
		level:          level,
		minSize:        minSize,
	}
}

// Wrap implements the Middleware interface
func (m *CompressionMiddleware) Wrap(transport transport.Transport) transport.Transport {
	return &compressionTransport{
		Transport:       transport,
		compressionType: m.compressionType,
		level:          m.level,
		minSize:        m.minSize,
	}
}

// compressionTransport wraps a transport with compression
type compressionTransport struct {
	transport.Transport
	compressionType transport.CompressionType
	level          int
	minSize        int64
}

// Send compresses events before sending
func (t *compressionTransport) Send(ctx context.Context, event transport.TransportEvent) error {
	// This would implement actual compression logic
	// For now, just pass through
	return t.Transport.Send(ctx, event)
}

// RateLimitMiddleware provides rate limiting functionality
type RateLimitMiddleware struct {
	limiter RateLimiter
}

// RateLimiter interface for rate limiting
type RateLimiter interface {
	Allow() bool
	Wait(ctx context.Context) error
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(limiter RateLimiter) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter: limiter,
	}
}

// Wrap implements the Middleware interface
func (m *RateLimitMiddleware) Wrap(transport transport.Transport) transport.Transport {
	return &rateLimitTransport{
		Transport: transport,
		limiter:   m.limiter,
	}
}

// rateLimitTransport wraps a transport with rate limiting
type rateLimitTransport struct {
	transport.Transport
	limiter RateLimiter
}

// Send applies rate limiting to sends
func (t *rateLimitTransport) Send(ctx context.Context, event transport.TransportEvent) error {
	// Wait for rate limit
	if err := t.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	return t.Transport.Send(ctx, event)
}

// CircuitBreakerMiddleware provides circuit breaker functionality
type CircuitBreakerMiddleware struct {
	breaker CircuitBreaker
}

// CircuitBreaker interface for circuit breaking
type CircuitBreaker interface {
	Call(ctx context.Context, fn func() error) error
	State() CircuitBreakerState
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// NewCircuitBreakerMiddleware creates a new circuit breaker middleware
func NewCircuitBreakerMiddleware(breaker CircuitBreaker) *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{
		breaker: breaker,
	}
}

// Wrap implements the Middleware interface
func (m *CircuitBreakerMiddleware) Wrap(transport transport.Transport) transport.Transport {
	return &circuitBreakerTransport{
		Transport: transport,
		breaker:   m.breaker,
	}
}

// circuitBreakerTransport wraps a transport with circuit breaker
type circuitBreakerTransport struct {
	transport.Transport
	breaker CircuitBreaker
}

// Connect applies circuit breaker to connection
func (t *circuitBreakerTransport) Connect(ctx context.Context) error {
	return t.breaker.Call(ctx, func() error {
		return t.Transport.Connect(ctx)
	})
}

// Send applies circuit breaker to sends
func (t *circuitBreakerTransport) Send(ctx context.Context, event transport.TransportEvent) error {
	return t.breaker.Call(ctx, func() error {
		return t.Transport.Send(ctx, event)
	})
}