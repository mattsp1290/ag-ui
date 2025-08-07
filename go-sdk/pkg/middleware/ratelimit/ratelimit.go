package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Local type definitions to avoid circular imports
type Request struct {
	ID        string                 `json:"id"`
	Method    string                 `json:"method"`
	Path      string                 `json:"path"`
	Headers   map[string]string      `json:"headers"`
	Body      interface{}            `json:"body"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

type Response struct {
	ID         string                 `json:"id"`
	StatusCode int                    `json:"status_code"`
	Headers    map[string]string      `json:"headers"`
	Body       interface{}            `json:"body"`
	Error      error                  `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata"`
	Timestamp  time.Time              `json:"timestamp"`
	Duration   time.Duration          `json:"duration"`
}

type NextHandler func(ctx context.Context, req *Request) (*Response, error)

// RateLimitAlgorithm represents different rate limiting algorithms
type RateLimitAlgorithm string

const (
	AlgorithmTokenBucket   RateLimitAlgorithm = "token_bucket"
	AlgorithmSlidingWindow RateLimitAlgorithm = "sliding_window"
	AlgorithmFixedWindow   RateLimitAlgorithm = "fixed_window"
	AlgorithmLeakyBucket   RateLimitAlgorithm = "leaky_bucket"
)

// RateLimitResult represents the result of a rate limit check
type RateLimitResult struct {
	Allowed       bool          `json:"allowed"`
	Remaining     int64         `json:"remaining"`
	RetryAfter    time.Duration `json:"retry_after,omitempty"`
	ResetTime     time.Time     `json:"reset_time"`
	TotalRequests int64         `json:"total_requests"`
}

// RateLimiter interface defines rate limiting operations
type RateLimiter interface {
	// Allow checks if a request should be allowed
	Allow(ctx context.Context, key string) (*RateLimitResult, error)

	// Reset resets the rate limiter for a given key
	Reset(ctx context.Context, key string) error

	// GetInfo returns current rate limit information for a key
	GetInfo(ctx context.Context, key string) (*RateLimitResult, error)

	// Algorithm returns the algorithm used by this rate limiter
	Algorithm() RateLimitAlgorithm
}

// TokenBucket implements token bucket rate limiting algorithm
type TokenBucket struct {
	rate     int64         // tokens per second
	capacity int64         // maximum tokens
	buckets  map[string]*bucket
	mu       sync.RWMutex
}

type bucket struct {
	tokens   int64
	lastFill time.Time
	mu       sync.Mutex
}

// NewTokenBucket creates a new token bucket rate limiter
func NewTokenBucket(rate, capacity int64) *TokenBucket {
	return &TokenBucket{
		rate:     rate,
		capacity: capacity,
		buckets:  make(map[string]*bucket),
	}
}

// Allow checks if a request should be allowed using token bucket algorithm
func (tb *TokenBucket) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	tb.mu.Lock()
	b, exists := tb.buckets[key]
	if !exists {
		b = &bucket{
			tokens:   tb.capacity,
			lastFill: time.Now(),
		}
		tb.buckets[key] = b
	}
	tb.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastFill)

	// Add tokens based on elapsed time
	tokensToAdd := int64(elapsed.Seconds()) * tb.rate
	if tokensToAdd > 0 {
		b.tokens = min(tb.capacity, b.tokens+tokensToAdd)
		b.lastFill = now
	}

	result := &RateLimitResult{
		Remaining:     b.tokens,
		ResetTime:     now.Add(time.Duration(tb.capacity/tb.rate) * time.Second),
		TotalRequests: 1,
	}

	if b.tokens > 0 {
		b.tokens--
		result.Allowed = true
		result.Remaining = b.tokens
	} else {
		result.Allowed = false
		result.RetryAfter = time.Second / time.Duration(tb.rate)
	}

	return result, nil
}

// Reset resets the token bucket for a given key
func (tb *TokenBucket) Reset(ctx context.Context, key string) error {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if b, exists := tb.buckets[key]; exists {
		b.mu.Lock()
		b.tokens = tb.capacity
		b.lastFill = time.Now()
		b.mu.Unlock()
	}

	return nil
}

// GetInfo returns current rate limit information
func (tb *TokenBucket) GetInfo(ctx context.Context, key string) (*RateLimitResult, error) {
	tb.mu.RLock()
	b, exists := tb.buckets[key]
	tb.mu.RUnlock()

	if !exists {
		return &RateLimitResult{
			Allowed:   true,
			Remaining: tb.capacity,
			ResetTime: time.Now().Add(time.Duration(tb.capacity/tb.rate) * time.Second),
		}, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	return &RateLimitResult{
		Allowed:   b.tokens > 0,
		Remaining: b.tokens,
		ResetTime: time.Now().Add(time.Duration(tb.capacity/tb.rate) * time.Second),
	}, nil
}

// Algorithm returns the algorithm name
func (tb *TokenBucket) Algorithm() RateLimitAlgorithm {
	return AlgorithmTokenBucket
}

// SlidingWindow implements sliding window rate limiting algorithm
type SlidingWindow struct {
	limit      int64
	windowSize time.Duration
	windows    map[string]*window
	mu         sync.RWMutex
}

type window struct {
	requests []time.Time
	mu       sync.Mutex
}

// NewSlidingWindow creates a new sliding window rate limiter
func NewSlidingWindow(limit int64, windowSize time.Duration) *SlidingWindow {
	return &SlidingWindow{
		limit:      limit,
		windowSize: windowSize,
		windows:    make(map[string]*window),
	}
}

// Allow checks if a request should be allowed using sliding window algorithm
func (sw *SlidingWindow) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	sw.mu.Lock()
	w, exists := sw.windows[key]
	if !exists {
		w = &window{
			requests: make([]time.Time, 0, sw.limit+1),
		}
		sw.windows[key] = w
	}
	sw.mu.Unlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-sw.windowSize)

	// Remove old requests outside the window
	validRequests := make([]time.Time, 0, len(w.requests))
	for _, reqTime := range w.requests {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}
	w.requests = validRequests

	result := &RateLimitResult{
		Remaining:     sw.limit - int64(len(w.requests)),
		ResetTime:     now.Add(sw.windowSize),
		TotalRequests: int64(len(w.requests)) + 1,
	}

	if int64(len(w.requests)) < sw.limit {
		w.requests = append(w.requests, now)
		result.Allowed = true
		result.Remaining = sw.limit - int64(len(w.requests))
	} else {
		result.Allowed = false
		// Calculate retry after based on oldest request in window
		if len(w.requests) > 0 {
			result.RetryAfter = w.requests[0].Add(sw.windowSize).Sub(now)
		}
	}

	return result, nil
}

// Reset resets the sliding window for a given key
func (sw *SlidingWindow) Reset(ctx context.Context, key string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if w, exists := sw.windows[key]; exists {
		w.mu.Lock()
		w.requests = w.requests[:0]
		w.mu.Unlock()
	}

	return nil
}

// GetInfo returns current rate limit information
func (sw *SlidingWindow) GetInfo(ctx context.Context, key string) (*RateLimitResult, error) {
	sw.mu.RLock()
	w, exists := sw.windows[key]
	sw.mu.RUnlock()

	if !exists {
		return &RateLimitResult{
			Allowed:   true,
			Remaining: sw.limit,
			ResetTime: time.Now().Add(sw.windowSize),
		}, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-sw.windowSize)

	// Count valid requests in current window
	validCount := int64(0)
	for _, reqTime := range w.requests {
		if reqTime.After(windowStart) {
			validCount++
		}
	}

	return &RateLimitResult{
		Allowed:       validCount < sw.limit,
		Remaining:     sw.limit - validCount,
		ResetTime:     now.Add(sw.windowSize),
		TotalRequests: validCount,
	}, nil
}

// Algorithm returns the algorithm name
func (sw *SlidingWindow) Algorithm() RateLimitAlgorithm {
	return AlgorithmSlidingWindow
}

// FixedWindow implements fixed window rate limiting algorithm
type FixedWindow struct {
	limit      int64
	windowSize time.Duration
	windows    map[string]*fixedWindowBucket
	mu         sync.RWMutex
}

type fixedWindowBucket struct {
	count      int64
	windowStart time.Time
	mu         sync.Mutex
}

// NewFixedWindow creates a new fixed window rate limiter
func NewFixedWindow(limit int64, windowSize time.Duration) *FixedWindow {
	return &FixedWindow{
		limit:      limit,
		windowSize: windowSize,
		windows:    make(map[string]*fixedWindowBucket),
	}
}

// Allow checks if a request should be allowed using fixed window algorithm
func (fw *FixedWindow) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	fw.mu.Lock()
	wb, exists := fw.windows[key]
	if !exists {
		wb = &fixedWindowBucket{
			count:       0,
			windowStart: fw.getCurrentWindow(),
		}
		fw.windows[key] = wb
	}
	fw.mu.Unlock()

	wb.mu.Lock()
	defer wb.mu.Unlock()

	currentWindow := fw.getCurrentWindow()

	// Reset if we're in a new window
	if wb.windowStart.Before(currentWindow) {
		wb.count = 0
		wb.windowStart = currentWindow
	}

	result := &RateLimitResult{
		Remaining:     fw.limit - wb.count,
		ResetTime:     currentWindow.Add(fw.windowSize),
		TotalRequests: wb.count + 1,
	}

	if wb.count < fw.limit {
		wb.count++
		result.Allowed = true
		result.Remaining = fw.limit - wb.count
	} else {
		result.Allowed = false
		result.RetryAfter = currentWindow.Add(fw.windowSize).Sub(time.Now())
	}

	return result, nil
}

// Reset resets the fixed window for a given key
func (fw *FixedWindow) Reset(ctx context.Context, key string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if wb, exists := fw.windows[key]; exists {
		wb.mu.Lock()
		wb.count = 0
		wb.windowStart = fw.getCurrentWindow()
		wb.mu.Unlock()
	}

	return nil
}

// GetInfo returns current rate limit information
func (fw *FixedWindow) GetInfo(ctx context.Context, key string) (*RateLimitResult, error) {
	fw.mu.RLock()
	wb, exists := fw.windows[key]
	fw.mu.RUnlock()

	currentWindow := fw.getCurrentWindow()

	if !exists {
		return &RateLimitResult{
			Allowed:   true,
			Remaining: fw.limit,
			ResetTime: currentWindow.Add(fw.windowSize),
		}, nil
	}

	wb.mu.Lock()
	defer wb.mu.Unlock()

	count := wb.count
	if wb.windowStart.Before(currentWindow) {
		count = 0
	}

	return &RateLimitResult{
		Allowed:       count < fw.limit,
		Remaining:     fw.limit - count,
		ResetTime:     currentWindow.Add(fw.windowSize),
		TotalRequests: count,
	}, nil
}

// Algorithm returns the algorithm name
func (fw *FixedWindow) Algorithm() RateLimitAlgorithm {
	return AlgorithmFixedWindow
}

// getCurrentWindow returns the current window start time
func (fw *FixedWindow) getCurrentWindow() time.Time {
	now := time.Now()
	windowSeconds := int64(fw.windowSize.Seconds())
	return time.Unix((now.Unix()/windowSeconds)*windowSeconds, 0)
}

// KeyGenerator generates rate limit keys from requests
type KeyGenerator interface {
	// GenerateKey generates a rate limit key for the given request
	GenerateKey(ctx context.Context, req *Request) (string, error)

	// GenerateMultipleKeys generates multiple rate limit keys (for different limits)
	GenerateMultipleKeys(ctx context.Context, req *Request) ([]string, error)
}

// IPKeyGenerator generates keys based on client IP
type IPKeyGenerator struct {
	prefix string
}

// NewIPKeyGenerator creates a new IP-based key generator
func NewIPKeyGenerator(prefix string) *IPKeyGenerator {
	return &IPKeyGenerator{
		prefix: prefix,
	}
}

// GenerateKey generates a key based on client IP
func (kg *IPKeyGenerator) GenerateKey(ctx context.Context, req *Request) (string, error) {
	ip := extractClientIP(req)
	if ip == "" {
		return "", fmt.Errorf("unable to extract client IP")
	}

	if kg.prefix != "" {
		return fmt.Sprintf("%s:%s", kg.prefix, ip), nil
	}

	return ip, nil
}

// GenerateMultipleKeys generates multiple keys for IP-based limiting
func (kg *IPKeyGenerator) GenerateMultipleKeys(ctx context.Context, req *Request) ([]string, error) {
	key, err := kg.GenerateKey(ctx, req)
	if err != nil {
		return nil, err
	}

	return []string{key}, nil
}

// UserKeyGenerator generates keys based on authenticated user
type UserKeyGenerator struct {
	prefix string
}

// NewUserKeyGenerator creates a new user-based key generator
func NewUserKeyGenerator(prefix string) *UserKeyGenerator {
	return &UserKeyGenerator{
		prefix: prefix,
	}
}

// GenerateKey generates a key based on authenticated user
func (kg *UserKeyGenerator) GenerateKey(ctx context.Context, req *Request) (string, error) {
	userID := extractUserID(req)
	if userID == "" {
		return "", fmt.Errorf("unable to extract user ID")
	}

	if kg.prefix != "" {
		return fmt.Sprintf("%s:%s", kg.prefix, userID), nil
	}

	return userID, nil
}

// GenerateMultipleKeys generates multiple keys for user-based limiting
func (kg *UserKeyGenerator) GenerateMultipleKeys(ctx context.Context, req *Request) ([]string, error) {
	key, err := kg.GenerateKey(ctx, req)
	if err != nil {
		return nil, err
	}

	return []string{key}, nil
}

// EndpointKeyGenerator generates keys based on endpoint
type EndpointKeyGenerator struct {
	prefix string
}

// NewEndpointKeyGenerator creates a new endpoint-based key generator
func NewEndpointKeyGenerator(prefix string) *EndpointKeyGenerator {
	return &EndpointKeyGenerator{
		prefix: prefix,
	}
}

// GenerateKey generates a key based on endpoint
func (kg *EndpointKeyGenerator) GenerateKey(ctx context.Context, req *Request) (string, error) {
	endpoint := fmt.Sprintf("%s:%s", req.Method, req.Path)

	if kg.prefix != "" {
		return fmt.Sprintf("%s:%s", kg.prefix, endpoint), nil
	}

	return endpoint, nil
}

// GenerateMultipleKeys generates multiple keys for endpoint-based limiting
func (kg *EndpointKeyGenerator) GenerateMultipleKeys(ctx context.Context, req *Request) ([]string, error) {
	key, err := kg.GenerateKey(ctx, req)
	if err != nil {
		return nil, err
	}

	return []string{key}, nil
}

// CompositeKeyGenerator combines multiple key generators
type CompositeKeyGenerator struct {
	generators []KeyGenerator
	separator  string
}

// NewCompositeKeyGenerator creates a new composite key generator
func NewCompositeKeyGenerator(separator string, generators ...KeyGenerator) *CompositeKeyGenerator {
	if separator == "" {
		separator = ":"
	}

	return &CompositeKeyGenerator{
		generators: generators,
		separator:  separator,
	}
}

// GenerateKey generates a composite key from multiple generators
func (kg *CompositeKeyGenerator) GenerateKey(ctx context.Context, req *Request) (string, error) {
	var parts []string

	for _, generator := range kg.generators {
		key, err := generator.GenerateKey(ctx, req)
		if err != nil {
			return "", fmt.Errorf("failed to generate key part: %w", err)
		}
		parts = append(parts, key)
	}

	return fmt.Sprintf("%s", parts), nil
}

// GenerateMultipleKeys generates multiple composite keys
func (kg *CompositeKeyGenerator) GenerateMultipleKeys(ctx context.Context, req *Request) ([]string, error) {
	key, err := kg.GenerateKey(ctx, req)
	if err != nil {
		return nil, err
	}

	return []string{key}, nil
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Algorithm       RateLimitAlgorithm `json:"algorithm" yaml:"algorithm"`
	RequestsPerUnit int64              `json:"requests_per_unit" yaml:"requests_per_unit"`
	Unit            time.Duration      `json:"unit" yaml:"unit"`
	Burst           int64              `json:"burst" yaml:"burst"`
	KeyGenerator    string             `json:"key_generator" yaml:"key_generator"`
	SkipPaths       []string           `json:"skip_paths" yaml:"skip_paths"`
	SkipHealthCheck bool               `json:"skip_health_check" yaml:"skip_health_check"`
	Distributed     bool               `json:"distributed" yaml:"distributed"`
	RedisURL        string             `json:"redis_url" yaml:"redis_url"`
}

// RateLimitMiddleware implements rate limiting middleware
type RateLimitMiddleware struct {
	config       *RateLimitConfig
	rateLimiter  RateLimiter
	keyGenerator KeyGenerator
	enabled      bool
	priority     int
	skipMap      map[string]bool
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(config *RateLimitConfig) (*RateLimitMiddleware, error) {
	if config == nil {
		config = &RateLimitConfig{
			Algorithm:       AlgorithmTokenBucket,
			RequestsPerUnit: 100,
			Unit:            time.Minute,
			Burst:           10,
			KeyGenerator:    "ip",
			SkipHealthCheck: true,
		}
	}

	// Create rate limiter based on algorithm
	var rateLimiter RateLimiter
	switch config.Algorithm {
	case AlgorithmTokenBucket:
		rate := config.RequestsPerUnit / int64(config.Unit.Seconds())
		if rate == 0 {
			rate = 1
		}
		burst := config.Burst
		if burst == 0 {
			burst = config.RequestsPerUnit
		}
		rateLimiter = NewTokenBucket(rate, burst)
	case AlgorithmSlidingWindow:
		rateLimiter = NewSlidingWindow(config.RequestsPerUnit, config.Unit)
	case AlgorithmFixedWindow:
		rateLimiter = NewFixedWindow(config.RequestsPerUnit, config.Unit)
	default:
		return nil, fmt.Errorf("unsupported rate limit algorithm: %s", config.Algorithm)
	}

	// Create key generator
	var keyGenerator KeyGenerator
	switch config.KeyGenerator {
	case "ip":
		keyGenerator = NewIPKeyGenerator("ip")
	case "user":
		keyGenerator = NewUserKeyGenerator("user")
	case "endpoint":
		keyGenerator = NewEndpointKeyGenerator("endpoint")
	default:
		keyGenerator = NewIPKeyGenerator("ip")
	}

	skipMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipMap[path] = true
	}

	// Add common health check paths
	if config.SkipHealthCheck {
		skipMap["/health"] = true
		skipMap["/healthz"] = true
		skipMap["/ping"] = true
		skipMap["/ready"] = true
		skipMap["/live"] = true
	}

	return &RateLimitMiddleware{
		config:       config,
		rateLimiter:  rateLimiter,
		keyGenerator: keyGenerator,
		enabled:      true,
		priority:     50, // Medium priority
		skipMap:      skipMap,
	}, nil
}

// Name returns middleware name
func (rl *RateLimitMiddleware) Name() string {
	return "rate_limit"
}

// Process processes the request through rate limiting middleware
func (rl *RateLimitMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Skip rate limiting for configured paths
	if rl.skipMap[req.Path] {
		return next(ctx, req)
	}

	// Generate rate limit key
	key, err := rl.keyGenerator.GenerateKey(ctx, req)
	if err != nil {
		// If key generation fails, log error but allow request
		return next(ctx, req)
	}

	// Check rate limit
	result, err := rl.rateLimiter.Allow(ctx, key)
	if err != nil {
		// If rate limit check fails, log error but allow request
		return next(ctx, req)
	}

	// Add rate limit headers to response
	headers := map[string]string{
		"X-RateLimit-Limit":     fmt.Sprintf("%d", rl.config.RequestsPerUnit),
		"X-RateLimit-Remaining": fmt.Sprintf("%d", result.Remaining),
		"X-RateLimit-Reset":     fmt.Sprintf("%d", result.ResetTime.Unix()),
	}

	if !result.Allowed {
		// Rate limit exceeded
		if result.RetryAfter > 0 {
			headers["Retry-After"] = fmt.Sprintf("%d", int(result.RetryAfter.Seconds()))
		}

		return &Response{
			ID:         req.ID,
			StatusCode: 429, // Too Many Requests
			Headers:    headers,
			Body: map[string]interface{}{
				"error":   "Rate limit exceeded",
				"message": fmt.Sprintf("Too many requests. Limit: %d per %s", rl.config.RequestsPerUnit, rl.config.Unit),
			},
			Timestamp: time.Now(),
		}, nil
	}

	// Process request through next middleware
	resp, err := next(ctx, req)

	// Add rate limit headers to successful response
	if resp != nil {
		if resp.Headers == nil {
			resp.Headers = make(map[string]string)
		}
		for k, v := range headers {
			resp.Headers[k] = v
		}
	}

	return resp, err
}

// Configure configures the middleware
func (rl *RateLimitMiddleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		rl.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		rl.priority = priority
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (rl *RateLimitMiddleware) Enabled() bool {
	return rl.enabled
}

// Priority returns the middleware priority
func (rl *RateLimitMiddleware) Priority() int {
	return rl.priority
}

// Helper functions

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func extractClientIP(req *Request) string {
	// Check common headers for client IP
	if ip := req.Headers["X-Forwarded-For"]; ip != "" {
		return ip
	}
	if ip := req.Headers["X-Real-IP"]; ip != "" {
		return ip
	}
	if ip := req.Headers["X-Client-IP"]; ip != "" {
		return ip
	}
	
	// Fallback to a default IP if none found
	return "unknown"
}

func extractUserID(req *Request) string {
	// Extract user ID from auth context
	if authCtx, ok := req.Metadata["auth_context"]; ok {
		if auth, ok := authCtx.(map[string]interface{}); ok {
			if userID, ok := auth["user_id"].(string); ok {
				return userID
			}
		}
	}

	return ""
}