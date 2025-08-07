package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// ResilienceMiddleware provides circuit breaker and retry capabilities
type ResilienceMiddleware struct {
	name           string
	enabled        bool
	priority       int
	circuitBreaker errors.CircuitBreaker
	retryConfig    *RetryConfig
	rateLimiter    *RateLimiter
	mu             sync.RWMutex
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []string // Error codes that should trigger retries
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"TIMEOUT",
			"CONNECTION_FAILED",
			"SERVICE_UNAVAILABLE",
			"RATE_LIMITED",
		},
	}
}

// RateLimiter provides rate limiting capabilities
type RateLimiter struct {
	tokensPerSecond int
	bucketSize      int
	tokens          int
	lastRefill      time.Time
	mu              sync.Mutex
}

// NewRateLimiter creates a new token bucket rate limiter
func NewRateLimiter(tokensPerSecond, bucketSize int) *RateLimiter {
	return &RateLimiter{
		tokensPerSecond: tokensPerSecond,
		bucketSize:      bucketSize,
		tokens:          bucketSize,
		lastRefill:      time.Now(),
	}
}

// Allow checks if a request is allowed based on rate limiting
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)

	// Add tokens based on elapsed time
	tokensToAdd := int(elapsed.Seconds() * float64(rl.tokensPerSecond))
	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.bucketSize {
			rl.tokens = rl.bucketSize
		}
		rl.lastRefill = now
	}

	// Check if we have tokens available
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// NewResilienceMiddleware creates a new resilience middleware
func NewResilienceMiddleware(name string, cbConfig *errors.CircuitBreakerConfig, retryConfig *RetryConfig, rateLimiter *RateLimiter) *ResilienceMiddleware {
	if cbConfig == nil {
		cbConfig = errors.DefaultCircuitBreakerConfig(name)
	}
	if retryConfig == nil {
		retryConfig = DefaultRetryConfig()
	}

	return &ResilienceMiddleware{
		name:           name,
		enabled:        true,
		priority:       100, // High priority for resilience
		circuitBreaker: errors.NewCircuitBreaker(cbConfig),
		retryConfig:    retryConfig,
		rateLimiter:    rateLimiter,
	}
}

// Name returns the middleware name
func (rm *ResilienceMiddleware) Name() string {
	return rm.name
}

// Process processes a request with resilience patterns
func (rm *ResilienceMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	rm.mu.RLock()
	enabled := rm.enabled
	rm.mu.RUnlock()

	if !enabled {
		return next(ctx, req)
	}

	// Apply rate limiting
	if rm.rateLimiter != nil && !rm.rateLimiter.Allow() {
		return &Response{
			ID:         req.ID,
			StatusCode: 429,
			Error:      fmt.Errorf("rate limit exceeded"),
			Timestamp:  time.Now(),
		}, nil
	}

	// Execute with circuit breaker and retry logic
	return rm.executeWithResilience(ctx, req, next)
}

// ProcessAsync implements AsyncMiddleware
func (rm *ResilienceMiddleware) ProcessAsync(ctx context.Context, req *Request, next NextHandler) <-chan *MiddlewareResult {
	resultChan := make(chan *MiddlewareResult, 1)

	go func() {
		defer close(resultChan)

		response, err := rm.Process(ctx, req, next)
		resultChan <- &MiddlewareResult{
			Response: response,
			Error:    err,
		}
	}()

	return resultChan
}

// executeWithResilience executes the request with circuit breaker and retry logic
func (rm *ResilienceMiddleware) executeWithResilience(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	var lastError error

	for attempt := 1; attempt <= rm.retryConfig.MaxAttempts; attempt++ {
		// Execute with circuit breaker protection
		result, err := rm.circuitBreaker.Call(ctx, func() (interface{}, error) {
			return next(ctx, req)
		})

		if err == nil && result != nil {
			if resp, ok := result.(*Response); ok {
				return resp, nil
			}
			return result.(*Response), nil
		}

		lastError = err

		// Check if we should retry
		if !rm.shouldRetry(err, attempt) {
			break
		}

		// Calculate delay for next attempt
		delay := rm.calculateDelay(attempt)

		// Wait before retry, respecting context cancellation
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return nil, fmt.Errorf("request cancelled during retry: %w", ctx.Err())
		}
	}

	// All retries exhausted, return the last error
	return &Response{
		ID:         req.ID,
		StatusCode: 500,
		Error:      fmt.Errorf("resilience middleware failed after %d attempts: %w", rm.retryConfig.MaxAttempts, lastError),
		Timestamp:  time.Now(),
	}, lastError
}

// shouldRetry determines if an error should trigger a retry
func (rm *ResilienceMiddleware) shouldRetry(err error, attempt int) bool {
	if attempt >= rm.retryConfig.MaxAttempts {
		return false
	}

	// Check if error is retryable
	for _, retryableError := range rm.retryConfig.RetryableErrors {
		if fmt.Sprintf("%v", err) == retryableError {
			return true
		}
	}

	return false
}

// calculateDelay calculates the delay for the next retry attempt
func (rm *ResilienceMiddleware) calculateDelay(attempt int) time.Duration {
	delay := rm.retryConfig.InitialDelay

	// Apply exponential backoff
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * rm.retryConfig.BackoffFactor)
		if delay > rm.retryConfig.MaxDelay {
			delay = rm.retryConfig.MaxDelay
			break
		}
	}

	return delay
}

// Configure configures the middleware
func (rm *ResilienceMiddleware) Configure(config map[string]interface{}) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if enabled, ok := config["enabled"].(bool); ok {
		rm.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		rm.priority = priority
	}

	// Configure retry settings
	if retryConfig, ok := config["retry"].(map[string]interface{}); ok {
		if maxAttempts, ok := retryConfig["max_attempts"].(int); ok {
			rm.retryConfig.MaxAttempts = maxAttempts
		}
		if initialDelay, ok := retryConfig["initial_delay"].(string); ok {
			if delay, err := time.ParseDuration(initialDelay); err == nil {
				rm.retryConfig.InitialDelay = delay
			}
		}
		if maxDelay, ok := retryConfig["max_delay"].(string); ok {
			if delay, err := time.ParseDuration(maxDelay); err == nil {
				rm.retryConfig.MaxDelay = delay
			}
		}
		if backoffFactor, ok := retryConfig["backoff_factor"].(float64); ok {
			rm.retryConfig.BackoffFactor = backoffFactor
		}
	}

	// Configure rate limiting
	if rateLimitConfig, ok := config["rate_limit"].(map[string]interface{}); ok {
		if tokensPerSecond, ok := rateLimitConfig["tokens_per_second"].(int); ok {
			if bucketSize, ok := rateLimitConfig["bucket_size"].(int); ok {
				rm.rateLimiter = NewRateLimiter(tokensPerSecond, bucketSize)
			}
		}
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (rm *ResilienceMiddleware) Enabled() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.enabled
}

// Priority returns the middleware priority
func (rm *ResilienceMiddleware) Priority() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.priority
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (rm *ResilienceMiddleware) GetCircuitBreakerStats() ResilienceStats {
	state := rm.circuitBreaker.State()
	counts := rm.circuitBreaker.Counts()

	return ResilienceStats{
		Name:                 rm.name,
		Enabled:              rm.enabled,
		CircuitBreakerState:  state.String(),
		TotalRequests:        counts.Requests,
		SuccessfulRequests:   counts.TotalSuccesses,
		FailedRequests:       counts.TotalFailures,
		ConsecutiveSuccesses: counts.ConsecutiveSuccesses,
		ConsecutiveFailures:  counts.ConsecutiveFailures,
		RetryConfiguration:   *rm.retryConfig,
		HasRateLimiter:       rm.rateLimiter != nil,
	}
}

// ResilienceStats contains statistics for resilience middleware
type ResilienceStats struct {
	Name                 string
	Enabled              bool
	CircuitBreakerState  string
	TotalRequests        uint64
	SuccessfulRequests   uint64
	FailedRequests       uint64
	ConsecutiveSuccesses uint64
	ConsecutiveFailures  uint64
	RetryConfiguration   RetryConfig
	HasRateLimiter       bool
}

// Reset resets the circuit breaker state
func (rm *ResilienceMiddleware) Reset() {
	rm.circuitBreaker.Reset()
}

// Trip manually trips the circuit breaker
func (rm *ResilienceMiddleware) Trip() {
	rm.circuitBreaker.Trip()
}

// ResilienceMiddlewareFactory creates resilience middleware instances
type ResilienceMiddlewareFactory struct{}

// NewResilienceMiddlewareFactory creates a new factory
func NewResilienceMiddlewareFactory() *ResilienceMiddlewareFactory {
	return &ResilienceMiddlewareFactory{}
}

// Create creates a new resilience middleware from configuration
func (rmf *ResilienceMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	// Parse circuit breaker configuration
	var cbConfig *errors.CircuitBreakerConfig
	if cbConfigMap, ok := config.Config["circuit_breaker"].(map[string]interface{}); ok {
		cbConfig = &errors.CircuitBreakerConfig{
			Name: config.Name,
		}

		if maxFailures, ok := cbConfigMap["max_failures"].(int); ok {
			cbConfig.MaxFailures = uint64(maxFailures)
		} else {
			cbConfig.MaxFailures = 5
		}

		if resetTimeout, ok := cbConfigMap["reset_timeout"].(string); ok {
			if timeout, err := time.ParseDuration(resetTimeout); err == nil {
				cbConfig.ResetTimeout = timeout
			} else {
				cbConfig.ResetTimeout = 60 * time.Second
			}
		} else {
			cbConfig.ResetTimeout = 60 * time.Second
		}

		if halfOpenMaxCalls, ok := cbConfigMap["half_open_max_calls"].(int); ok {
			cbConfig.HalfOpenMaxCalls = uint64(halfOpenMaxCalls)
		} else {
			cbConfig.HalfOpenMaxCalls = 3
		}

		if successThreshold, ok := cbConfigMap["success_threshold"].(int); ok {
			cbConfig.SuccessThreshold = uint64(successThreshold)
		} else {
			cbConfig.SuccessThreshold = 2
		}

		if timeout, ok := cbConfigMap["timeout"].(string); ok {
			if t, err := time.ParseDuration(timeout); err == nil {
				cbConfig.Timeout = t
			} else {
				cbConfig.Timeout = 10 * time.Second
			}
		} else {
			cbConfig.Timeout = 10 * time.Second
		}
	}

	// Parse retry configuration
	var retryConfig *RetryConfig
	if retryConfigMap, ok := config.Config["retry"].(map[string]interface{}); ok {
		retryConfig = &RetryConfig{}

		if maxAttempts, ok := retryConfigMap["max_attempts"].(int); ok {
			retryConfig.MaxAttempts = maxAttempts
		} else {
			retryConfig.MaxAttempts = 3
		}

		if initialDelay, ok := retryConfigMap["initial_delay"].(string); ok {
			if delay, err := time.ParseDuration(initialDelay); err == nil {
				retryConfig.InitialDelay = delay
			} else {
				retryConfig.InitialDelay = 100 * time.Millisecond
			}
		} else {
			retryConfig.InitialDelay = 100 * time.Millisecond
		}

		if maxDelay, ok := retryConfigMap["max_delay"].(string); ok {
			if delay, err := time.ParseDuration(maxDelay); err == nil {
				retryConfig.MaxDelay = delay
			} else {
				retryConfig.MaxDelay = 5 * time.Second
			}
		} else {
			retryConfig.MaxDelay = 5 * time.Second
		}

		if backoffFactor, ok := retryConfigMap["backoff_factor"].(float64); ok {
			retryConfig.BackoffFactor = backoffFactor
		} else {
			retryConfig.BackoffFactor = 2.0
		}

		if retryableErrors, ok := retryConfigMap["retryable_errors"].([]interface{}); ok {
			retryConfig.RetryableErrors = make([]string, len(retryableErrors))
			for i, err := range retryableErrors {
				if errStr, ok := err.(string); ok {
					retryConfig.RetryableErrors[i] = errStr
				}
			}
		} else {
			retryConfig.RetryableErrors = []string{
				"TIMEOUT",
				"CONNECTION_FAILED",
				"SERVICE_UNAVAILABLE",
				"RATE_LIMITED",
			}
		}
	}

	// Parse rate limiter configuration
	var rateLimiter *RateLimiter
	if rateLimitConfig, ok := config.Config["rate_limit"].(map[string]interface{}); ok {
		tokensPerSecond, hasTokens := rateLimitConfig["tokens_per_second"].(int)
		bucketSize, hasBucket := rateLimitConfig["bucket_size"].(int)

		if hasTokens && hasBucket {
			rateLimiter = NewRateLimiter(tokensPerSecond, bucketSize)
		}
	}

	middleware := NewResilienceMiddleware(config.Name, cbConfig, retryConfig, rateLimiter)

	// Apply additional configuration
	if err := middleware.Configure(config.Config); err != nil {
		return nil, fmt.Errorf("failed to configure resilience middleware: %w", err)
	}

	return middleware, nil
}

// SupportedTypes returns the types supported by this factory
func (rmf *ResilienceMiddlewareFactory) SupportedTypes() []string {
	return []string{"resilience", "circuit_breaker", "retry"}
}
