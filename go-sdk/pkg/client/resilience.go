// Package client provides resilience patterns for robust HTTP operations.
// This module implements comprehensive failure handling, recovery mechanisms,
// and reliability patterns to ensure high availability and fault tolerance.
package client

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	pkgerrors "github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// ResilienceConfig contains configuration for all resilience patterns
type ResilienceConfig struct {
	// RetryConfig configures retry behavior
	Retry RetryConfig `json:"retry"`
	
	// CircuitBreakerConfig configures circuit breaker behavior
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker"`
	
	// RateLimiterConfig configures rate limiting
	RateLimit RateLimitConfig `json:"rate_limit"`
	
	// TimeoutConfig configures timeout behavior
	Timeout TimeoutConfig `json:"timeout"`
	
	// BulkheadConfig configures resource isolation
	Bulkhead BulkheadConfig `json:"bulkhead"`
	
	// HealthCheckConfig configures health monitoring
	HealthCheck HealthCheckConfig `json:"health_check"`
	
	// MetricsConfig configures metrics collection
	Metrics MetricsConfig `json:"metrics"`
}

// RetryConfig configures exponential backoff retry logic
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int `json:"max_attempts"`
	
	// BaseDelay is the initial delay between retries
	BaseDelay time.Duration `json:"base_delay"`
	
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration `json:"max_delay"`
	
	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64 `json:"backoff_multiplier"`
	
	// JitterEnabled enables random jitter to prevent thundering herd
	JitterEnabled bool `json:"jitter_enabled"`
	
	// JitterMaxFactor is the maximum jitter factor (0.0 to 1.0)
	JitterMaxFactor float64 `json:"jitter_max_factor"`
	
	// RetryableErrors defines which errors should trigger retries
	RetryableErrors []string `json:"retryable_errors"`
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	// Enabled indicates if circuit breaker is active
	Enabled bool `json:"enabled"`
	
	// FailureThreshold is the number of failures before opening
	FailureThreshold int `json:"failure_threshold"`
	
	// SuccessThreshold is the number of successes needed to close
	SuccessThreshold int `json:"success_threshold"`
	
	// Timeout is how long to wait before attempting to close
	Timeout time.Duration `json:"timeout"`
	
	// HalfOpenMaxCalls is max calls allowed in half-open state
	HalfOpenMaxCalls int `json:"half_open_max_calls"`
	
	// FailureRateThreshold is the failure rate threshold (0.0 to 1.0)
	FailureRateThreshold float64 `json:"failure_rate_threshold"`
	
	// MinimumRequestThreshold is minimum requests before rate calculation
	MinimumRequestThreshold int `json:"minimum_request_threshold"`
}

// RateLimitConfig configures rate limiting and throttling
type RateLimitConfig struct {
	// Enabled indicates if rate limiting is active
	Enabled bool `json:"enabled"`
	
	// RequestsPerSecond is the maximum requests per second
	RequestsPerSecond float64 `json:"requests_per_second"`
	
	// BurstSize is the maximum burst of requests allowed
	BurstSize int `json:"burst_size"`
	
	// WindowSize is the time window for rate calculation
	WindowSize time.Duration `json:"window_size"`
	
	// ThrottleDelay is the delay when throttling is applied
	ThrottleDelay time.Duration `json:"throttle_delay"`
}

// TimeoutConfig configures timeout and deadline management
type TimeoutConfig struct {
	// OperationTimeout is the default operation timeout
	OperationTimeout time.Duration `json:"operation_timeout"`
	
	// ConnectionTimeout is the connection establishment timeout
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	
	// ReadTimeout is the read operation timeout
	ReadTimeout time.Duration `json:"read_timeout"`
	
	// WriteTimeout is the write operation timeout
	WriteTimeout time.Duration `json:"write_timeout"`
	
	// KeepAliveTimeout is the keep-alive timeout
	KeepAliveTimeout time.Duration `json:"keep_alive_timeout"`
	
	// IdleConnectionTimeout is the idle connection timeout
	IdleConnectionTimeout time.Duration `json:"idle_connection_timeout"`
}

// BulkheadConfig configures resource isolation patterns
type BulkheadConfig struct {
	// Enabled indicates if bulkhead isolation is active
	Enabled bool `json:"enabled"`
	
	// MaxConcurrentRequests is the maximum concurrent requests
	MaxConcurrentRequests int `json:"max_concurrent_requests"`
	
	// QueueSize is the size of the request queue
	QueueSize int `json:"queue_size"`
	
	// QueueTimeout is the maximum time to wait in queue
	QueueTimeout time.Duration `json:"queue_timeout"`
	
	// SemaphoreTimeout is the timeout for acquiring semaphore
	SemaphoreTimeout time.Duration `json:"semaphore_timeout"`
}

// HealthCheckConfig configures health monitoring and recovery
type HealthCheckConfig struct {
	// Enabled indicates if health checks are active
	Enabled bool `json:"enabled"`
	
	// Interval is the health check interval
	Interval time.Duration `json:"interval"`
	
	// Timeout is the health check timeout
	Timeout time.Duration `json:"timeout"`
	
	// FailureThreshold is failures needed to mark unhealthy
	FailureThreshold int `json:"failure_threshold"`
	
	// RecoveryThreshold is successes needed to mark healthy
	RecoveryThreshold int `json:"recovery_threshold"`
	
	// Endpoint is the health check endpoint
	Endpoint string `json:"endpoint"`
	
	// ExpectedStatusCodes are the expected healthy status codes
	ExpectedStatusCodes []int `json:"expected_status_codes"`
}

// MetricsConfig configures metrics collection and monitoring
type MetricsConfig struct {
	// Enabled indicates if metrics collection is active
	Enabled bool `json:"enabled"`
	
	// CollectionInterval is the metrics collection interval
	CollectionInterval time.Duration `json:"collection_interval"`
	
	// RetentionPeriod is how long to retain metrics
	RetentionPeriod time.Duration `json:"retention_period"`
	
	// HistogramBuckets defines histogram bucket boundaries
	HistogramBuckets []float64 `json:"histogram_buckets"`
	
	// EnableDetailedMetrics enables detailed metric collection
	EnableDetailedMetrics bool `json:"enable_detailed_metrics"`
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	// CircuitBreakerClosed indicates normal operation
	CircuitBreakerClosed CircuitBreakerState = iota
	
	// CircuitBreakerOpen indicates circuit is open (failing fast)
	CircuitBreakerOpen
	
	// CircuitBreakerHalfOpen indicates testing if service recovered
	CircuitBreakerHalfOpen
)

// String returns the string representation of circuit breaker state
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "CLOSED"
	case CircuitBreakerOpen:
		return "OPEN"
	case CircuitBreakerHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// HealthStatus represents the health status of a service
type HealthStatus int

const (
	// HealthStatusUnknown indicates health status is unknown
	HealthStatusUnknown HealthStatus = iota
	
	// HealthStatusHealthy indicates service is healthy
	HealthStatusHealthy
	
	// HealthStatusUnhealthy indicates service is unhealthy
	HealthStatusUnhealthy
	
	// HealthStatusDegraded indicates service is degraded
	HealthStatusDegraded
)

// String returns the string representation of health status
func (s HealthStatus) String() string {
	switch s {
	case HealthStatusHealthy:
		return "HEALTHY"
	case HealthStatusUnhealthy:
		return "UNHEALTHY"
	case HealthStatusDegraded:
		return "DEGRADED"
	default:
		return "UNKNOWN"
	}
}

// ResilienceManager coordinates all resilience patterns
type ResilienceManager struct {
	config       ResilienceConfig
	retryManager *RetryManager
	circuitBreaker *CircuitBreaker
	rateLimiter  *RateLimiter
	bulkhead     *Bulkhead
	healthChecker *HealthChecker
	metrics      *MetricsCollector
	mu           sync.RWMutex
}

// NewResilienceManager creates a new resilience manager with the given configuration
func NewResilienceManager(config ResilienceConfig) *ResilienceManager {
	rm := &ResilienceManager{
		config: config,
	}
	
	// Initialize components
	rm.retryManager = NewRetryManager(config.Retry)
	rm.circuitBreaker = NewCircuitBreaker(config.CircuitBreaker)
	rm.rateLimiter = NewRateLimiter(int(config.RateLimit.RequestsPerSecond), config.RateLimit.BurstSize)
	rm.bulkhead = NewBulkhead(config.Bulkhead)
	rm.healthChecker = NewHealthChecker(config.HealthCheck)
	rm.metrics = NewMetricsCollector(config.Metrics)
	
	return rm
}

// Execute executes an operation with all resilience patterns applied
func (rm *ResilienceManager) Execute(ctx context.Context, operation func(ctx context.Context) error) error {
	start := time.Now()
	
	// Check if circuit breaker allows the call
	if !rm.circuitBreaker.AllowRequest() {
		rm.metrics.RecordCircuitBreakerReject()
		return pkgerrors.NewOperationError("Execute", "circuit_breaker", 
			fmt.Errorf("circuit breaker is open"))
	}
	
	// Apply rate limiting
	if !rm.rateLimiter.Allow() {
		rm.metrics.RecordRateLimitReject()
		return pkgerrors.NewOperationError("Execute", "rate_limiter", fmt.Errorf("rate limit exceeded"))
	}
	
	// Acquire bulkhead semaphore
	if err := rm.bulkhead.Acquire(ctx); err != nil {
		rm.metrics.RecordBulkheadReject()
		return pkgerrors.NewOperationError("Execute", "bulkhead", err)
	}
	defer rm.bulkhead.Release()
	
	// Execute with retry logic
	err := rm.retryManager.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		// Apply timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, rm.config.Timeout.OperationTimeout)
		defer cancel()
		
		return operation(timeoutCtx)
	})
	
	// Record circuit breaker result
	if err != nil {
		rm.circuitBreaker.RecordFailure()
		rm.metrics.RecordFailure(time.Since(start))
	} else {
		rm.circuitBreaker.RecordSuccess()
		rm.metrics.RecordSuccess(time.Since(start))
	}
	
	return err
}

// RetryManager handles exponential backoff retry logic with jitter
type RetryManager struct {
	config RetryConfig
	rand   *rand.Rand
	mu     sync.Mutex
}

// NewRetryManager creates a new retry manager
func NewRetryManager(config RetryConfig) *RetryManager {
	return &RetryManager{
		config: config,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ExecuteWithRetry executes an operation with retry logic
func (rm *RetryManager) ExecuteWithRetry(ctx context.Context, operation func(ctx context.Context) error) error {
	var lastErr error
	
	for attempt := 0; attempt <= rm.config.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := rm.calculateDelay(attempt)
			
			select {
			case <-ctx.Done():
				return pkgerrors.NewTimeoutErrorWithOperation("ExecuteWithRetry", 
					time.Duration(0), delay).WithCause(ctx.Err())
			case <-time.After(delay):
				// Continue to retry
			}
		}
		
		err := operation(ctx)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !rm.isRetryable(err) {
			break
		}
		
		// Check if we've exhausted retries
		if attempt >= rm.config.MaxAttempts {
			break
		}
	}
	
	return pkgerrors.NewOperationError("ExecuteWithRetry", "retry_exhausted", 
		pkgerrors.ErrRetryExhausted).WithCause(lastErr)
}

// calculateDelay calculates the delay for the given attempt with exponential backoff and jitter
func (rm *RetryManager) calculateDelay(attempt int) time.Duration {
	// Calculate exponential backoff
	delay := float64(rm.config.BaseDelay) * math.Pow(rm.config.BackoffMultiplier, float64(attempt-1))
	
	// Apply maximum delay limit
	if maxDelay := float64(rm.config.MaxDelay); delay > maxDelay {
		delay = maxDelay
	}
	
	// Apply jitter if enabled
	if rm.config.JitterEnabled {
		rm.mu.Lock()
		jitter := rm.rand.Float64() * rm.config.JitterMaxFactor * delay
		rm.mu.Unlock()
		
		delay += jitter
	}
	
	return time.Duration(delay)
}

// isRetryable determines if an error should trigger a retry
func (rm *RetryManager) isRetryable(err error) bool {
	if err == nil {
		return false
	}
	
	// Check if it's marked as retryable in our error system
	if pkgerrors.IsRetryable(err) {
		return true
	}
	
	// Check against configured retryable error patterns
	errStr := err.Error()
	for _, pattern := range rm.config.RetryableErrors {
		if pattern == errStr {
			return true
		}
	}
	
	return false
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config          CircuitBreakerConfig
	state          CircuitBreakerState
	failures       int64
	successes      int64
	requests       int64
	lastFailTime   time.Time
	stateChangedAt time.Time
	mu             sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:         config,
		state:          CircuitBreakerClosed,
		stateChangedAt: time.Now(),
	}
}

// AllowRequest determines if a request should be allowed
func (cb *CircuitBreaker) AllowRequest() bool {
	if !cb.config.Enabled {
		return true
	}
	
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	switch cb.state {
	case CircuitBreakerClosed:
		return true
		
	case CircuitBreakerOpen:
		// Check if timeout has elapsed
		if time.Since(cb.stateChangedAt) >= cb.config.Timeout {
			cb.state = CircuitBreakerHalfOpen
			cb.stateChangedAt = time.Now()
			cb.successes = 0
			return true
		}
		return false
		
	case CircuitBreakerHalfOpen:
		// Allow limited requests in half-open state
		return cb.requests < int64(cb.config.HalfOpenMaxCalls)
		
	default:
		return false
	}
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	if !cb.config.Enabled {
		return
	}
	
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	atomic.AddInt64(&cb.requests, 1)
	atomic.AddInt64(&cb.successes, 1)
	
	switch cb.state {
	case CircuitBreakerHalfOpen:
		if cb.successes >= int64(cb.config.SuccessThreshold) {
			cb.state = CircuitBreakerClosed
			cb.stateChangedAt = time.Now()
			cb.reset()
		}
		
	case CircuitBreakerOpen:
		// Should not happen, but reset if it does
		cb.reset()
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	if !cb.config.Enabled {
		return
	}
	
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	atomic.AddInt64(&cb.requests, 1)
	atomic.AddInt64(&cb.failures, 1)
	cb.lastFailTime = time.Now()
	
	if cb.shouldTrip() {
		cb.state = CircuitBreakerOpen
		cb.stateChangedAt = time.Now()
	}
}

// shouldTrip determines if the circuit breaker should trip to open state
func (cb *CircuitBreaker) shouldTrip() bool {
	// Check minimum request threshold
	if cb.requests < int64(cb.config.MinimumRequestThreshold) {
		return false
	}
	
	// Check failure threshold
	if cb.failures >= int64(cb.config.FailureThreshold) {
		return true
	}
	
	// Check failure rate threshold
	failureRate := float64(cb.failures) / float64(cb.requests)
	return failureRate >= cb.config.FailureRateThreshold
}

// reset resets the circuit breaker counters
func (cb *CircuitBreaker) reset() {
	atomic.StoreInt64(&cb.failures, 0)
	atomic.StoreInt64(&cb.successes, 0)
	atomic.StoreInt64(&cb.requests, 0)
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetMetrics returns circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	return map[string]interface{}{
		"state":            cb.state.String(),
		"failures":         atomic.LoadInt64(&cb.failures),
		"successes":        atomic.LoadInt64(&cb.successes),
		"requests":         atomic.LoadInt64(&cb.requests),
		"last_fail_time":   cb.lastFailTime,
		"state_changed_at": cb.stateChangedAt,
	}
}

// Note: RateLimiter is implemented in rate_limiter.go

// Note: NewRateLimiter and RateLimiter methods are implemented in rate_limiter.go

// Bulkhead implements resource isolation using semaphores
type Bulkhead struct {
	config    BulkheadConfig
	semaphore chan struct{}
	queue     chan request
	metrics   bulkheadMetrics
}

type request struct {
	ctx      context.Context
	response chan error
}

type bulkheadMetrics struct {
	activeRequests   int64
	queuedRequests   int64
	rejectedRequests int64
	timeouts         int64
}

// NewBulkhead creates a new bulkhead
func NewBulkhead(config BulkheadConfig) *Bulkhead {
	b := &Bulkhead{
		config: config,
	}
	
	if config.Enabled {
		b.semaphore = make(chan struct{}, config.MaxConcurrentRequests)
		b.queue = make(chan request, config.QueueSize)
		
		// Start request processor
		go b.processRequests()
	}
	
	return b
}

// Acquire acquires a permit from the bulkhead
func (b *Bulkhead) Acquire(ctx context.Context) error {
	if !b.config.Enabled {
		return nil
	}
	
	req := request{
		ctx:      ctx,
		response: make(chan error, 1),
	}
	
	select {
	case b.queue <- req:
		atomic.AddInt64(&b.metrics.queuedRequests, 1)
		
		select {
		case err := <-req.response:
			atomic.AddInt64(&b.metrics.queuedRequests, -1)
			return err
		case <-ctx.Done():
			atomic.AddInt64(&b.metrics.queuedRequests, -1)
			atomic.AddInt64(&b.metrics.timeouts, 1)
			return pkgerrors.NewTimeoutErrorWithOperation("Bulkhead.Acquire", 
				b.config.QueueTimeout, b.config.QueueTimeout).WithCause(ctx.Err())
		}
		
	default:
		atomic.AddInt64(&b.metrics.rejectedRequests, 1)
		return pkgerrors.NewOperationError("Bulkhead.Acquire", "queue_full", 
			fmt.Errorf("bulkhead queue is full"))
	}
}

// Release releases a permit back to the bulkhead
func (b *Bulkhead) Release() {
	if !b.config.Enabled {
		return
	}
	
	select {
	case <-b.semaphore:
		atomic.AddInt64(&b.metrics.activeRequests, -1)
	default:
		// Should not happen
	}
}

// processRequests processes queued requests
func (b *Bulkhead) processRequests() {
	for req := range b.queue {
		select {
		case b.semaphore <- struct{}{}:
			atomic.AddInt64(&b.metrics.activeRequests, 1)
			req.response <- nil
		case <-time.After(b.config.SemaphoreTimeout):
			req.response <- pkgerrors.NewTimeoutErrorWithOperation("Bulkhead.processRequests", 
				b.config.SemaphoreTimeout, b.config.SemaphoreTimeout)
		case <-req.ctx.Done():
			req.response <- pkgerrors.NewTimeoutErrorWithOperation("Bulkhead.processRequests", 
				b.config.SemaphoreTimeout, b.config.SemaphoreTimeout).WithCause(req.ctx.Err())
		}
	}
}

// GetMetrics returns bulkhead metrics
func (b *Bulkhead) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"active_requests":   atomic.LoadInt64(&b.metrics.activeRequests),
		"queued_requests":   atomic.LoadInt64(&b.metrics.queuedRequests),
		"rejected_requests": atomic.LoadInt64(&b.metrics.rejectedRequests),
		"timeouts":          atomic.LoadInt64(&b.metrics.timeouts),
	}
}

// HealthChecker monitors service health and implements recovery mechanisms
type HealthChecker struct {
	config      HealthCheckConfig
	status      HealthStatus
	failures    int
	successes   int
	lastCheck   time.Time
	mu          sync.RWMutex
	stopCh      chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(config HealthCheckConfig) *HealthChecker {
	hc := &HealthChecker{
		config:    config,
		status:    HealthStatusUnknown,
		lastCheck: time.Now(),
		stopCh:    make(chan struct{}),
	}
	
	if config.Enabled {
		go hc.startHealthChecks()
	}
	
	return hc
}

// GetStatus returns the current health status
func (hc *HealthChecker) GetStatus() HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.status
}

// IsHealthy returns true if the service is healthy
func (hc *HealthChecker) IsHealthy() bool {
	return hc.GetStatus() == HealthStatusHealthy
}

// startHealthChecks starts the health check routine
func (hc *HealthChecker) startHealthChecks() {
	ticker := time.NewTicker(hc.config.Interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			hc.performHealthCheck()
		case <-hc.stopCh:
			return
		}
	}
}

// performHealthCheck performs a single health check
func (hc *HealthChecker) performHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()
	
	err := hc.checkHealth(ctx)
	
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.lastCheck = time.Now()
	
	if err != nil {
		hc.failures++
		hc.successes = 0
		
		if hc.failures >= hc.config.FailureThreshold {
			hc.status = HealthStatusUnhealthy
		}
	} else {
		hc.successes++
		hc.failures = 0
		
		if hc.successes >= hc.config.RecoveryThreshold {
			hc.status = HealthStatusHealthy
		}
	}
}

// checkHealth performs the actual health check
func (hc *HealthChecker) checkHealth(ctx context.Context) error {
	// For now, this is a placeholder for actual health check logic
	// In a real implementation, this would make HTTP requests to health endpoints
	// or perform other health verification operations
	return nil
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// GetMetrics returns health check metrics
func (hc *HealthChecker) GetMetrics() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	return map[string]interface{}{
		"status":      hc.status.String(),
		"failures":    hc.failures,
		"successes":   hc.successes,
		"last_check":  hc.lastCheck,
	}
}

// MetricsCollector collects and manages resilience metrics
type MetricsCollector struct {
	config                  MetricsConfig
	successCount            int64
	failureCount            int64
	circuitBreakerRejects   int64
	rateLimitRejects        int64
	bulkheadRejects         int64
	responseTimeHistogram   []int64
	mu                      sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(config MetricsConfig) *MetricsCollector {
	return &MetricsCollector{
		config:                config,
		responseTimeHistogram: make([]int64, len(config.HistogramBuckets)),
	}
}

// RecordSuccess records a successful operation
func (mc *MetricsCollector) RecordSuccess(duration time.Duration) {
	if !mc.config.Enabled {
		return
	}
	
	atomic.AddInt64(&mc.successCount, 1)
	mc.recordResponseTime(duration)
}

// RecordFailure records a failed operation
func (mc *MetricsCollector) RecordFailure(duration time.Duration) {
	if !mc.config.Enabled {
		return
	}
	
	atomic.AddInt64(&mc.failureCount, 1)
	mc.recordResponseTime(duration)
}

// RecordCircuitBreakerReject records a circuit breaker rejection
func (mc *MetricsCollector) RecordCircuitBreakerReject() {
	if mc.config.Enabled {
		atomic.AddInt64(&mc.circuitBreakerRejects, 1)
	}
}

// RecordRateLimitReject records a rate limit rejection
func (mc *MetricsCollector) RecordRateLimitReject() {
	if mc.config.Enabled {
		atomic.AddInt64(&mc.rateLimitRejects, 1)
	}
}

// RecordBulkheadReject records a bulkhead rejection
func (mc *MetricsCollector) RecordBulkheadReject() {
	if mc.config.Enabled {
		atomic.AddInt64(&mc.bulkheadRejects, 1)
	}
}

// recordResponseTime records response time in histogram
func (mc *MetricsCollector) recordResponseTime(duration time.Duration) {
	if !mc.config.EnableDetailedMetrics {
		return
	}
	
	durationMs := float64(duration.Nanoseconds()) / 1e6
	
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	for i, bucket := range mc.config.HistogramBuckets {
		if durationMs <= bucket {
			mc.responseTimeHistogram[i]++
			break
		}
	}
}

// GetMetrics returns all collected metrics
func (mc *MetricsCollector) GetMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	metrics := map[string]interface{}{
		"success_count":              atomic.LoadInt64(&mc.successCount),
		"failure_count":              atomic.LoadInt64(&mc.failureCount),
		"circuit_breaker_rejects":    atomic.LoadInt64(&mc.circuitBreakerRejects),
		"rate_limit_rejects":         atomic.LoadInt64(&mc.rateLimitRejects),
		"bulkhead_rejects":           atomic.LoadInt64(&mc.bulkheadRejects),
	}
	
	if mc.config.EnableDetailedMetrics {
		metrics["response_time_histogram"] = mc.responseTimeHistogram
		metrics["histogram_buckets"] = mc.config.HistogramBuckets
	}
	
	return metrics
}

// DefaultResilienceConfig returns a default resilience configuration
func DefaultResilienceConfig() ResilienceConfig {
	return ResilienceConfig{
		Retry: RetryConfig{
			MaxAttempts:       3,
			BaseDelay:         100 * time.Millisecond,
			MaxDelay:          30 * time.Second,
			BackoffMultiplier: 2.0,
			JitterEnabled:     true,
			JitterMaxFactor:   0.1,
			RetryableErrors:   []string{"timeout", "connection refused", "temporary failure"},
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:                 true,
			FailureThreshold:        5,
			SuccessThreshold:        3,
			Timeout:                 60 * time.Second,
			HalfOpenMaxCalls:        3,
			FailureRateThreshold:    0.5,
			MinimumRequestThreshold: 10,
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 100.0,
			BurstSize:         10,
			WindowSize:        time.Second,
			ThrottleDelay:     100 * time.Millisecond,
		},
		Timeout: TimeoutConfig{
			OperationTimeout:      30 * time.Second,
			ConnectionTimeout:     10 * time.Second,
			ReadTimeout:           10 * time.Second,
			WriteTimeout:          10 * time.Second,
			KeepAliveTimeout:      30 * time.Second,
			IdleConnectionTimeout: 90 * time.Second,
		},
		Bulkhead: BulkheadConfig{
			Enabled:               true,
			MaxConcurrentRequests: 100,
			QueueSize:             50,
			QueueTimeout:          5 * time.Second,
			SemaphoreTimeout:      1 * time.Second,
		},
		HealthCheck: HealthCheckConfig{
			Enabled:             true,
			Interval:            30 * time.Second,
			Timeout:             5 * time.Second,
			FailureThreshold:    3,
			RecoveryThreshold:   2,
			Endpoint:            "/health",
			ExpectedStatusCodes: []int{200, 201, 202},
		},
		Metrics: MetricsConfig{
			Enabled:               true,
			CollectionInterval:    60 * time.Second,
			RetentionPeriod:       24 * time.Hour,
			HistogramBuckets:      []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
			EnableDetailedMetrics: true,
		},
	}
}