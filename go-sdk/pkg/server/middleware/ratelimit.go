package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Performance optimization pools for rate limiting objects
var (
	// Token bucket limiter pool
	tokenBucketLimiterPool = sync.Pool{
		New: func() interface{} {
			return &TokenBucketLimiter{}
		},
	}
	
	// Sliding window limiter pool
	slidingWindowLimiterPool = sync.Pool{
		New: func() interface{} {
			return &SlidingWindowLimiter{
				requests: make([]time.Time, 0, 100), // Pre-allocate capacity
			}
		},
	}
	
	// Rate limit response map pool
	rateResponsePool = sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{}, 4)
		},
	}
)

// RateLimitAlgorithm represents different rate limiting algorithms
type RateLimitAlgorithm string

const (
	// TokenBucket uses token bucket algorithm
	TokenBucket RateLimitAlgorithm = "token_bucket"
	
	// SlidingWindow uses sliding window algorithm
	SlidingWindow RateLimitAlgorithm = "sliding_window"
	
	// FixedWindow uses fixed window algorithm
	FixedWindow RateLimitAlgorithm = "fixed_window"
)

// RateLimitScope defines the scope for rate limiting
type RateLimitScope string

const (
	// ScopeGlobal applies rate limit globally
	ScopeGlobal RateLimitScope = "global"
	
	// ScopeIP applies rate limit per IP address
	ScopeIP RateLimitScope = "ip"
	
	// ScopeUser applies rate limit per authenticated user
	ScopeUser RateLimitScope = "user"
	
	// ScopeAPIKey applies rate limit per API key
	ScopeAPIKey RateLimitScope = "api_key"
	
	// ScopeEndpoint applies rate limit per endpoint
	ScopeEndpoint RateLimitScope = "endpoint"
	
	// ScopeCustom applies rate limit based on custom key extraction
	ScopeCustom RateLimitScope = "custom"
)

// RateLimitConfig contains rate limiting middleware configuration
type RateLimitConfig struct {
	BaseConfig `json:",inline" yaml:",inline"`
	
	// Algorithm specifies the rate limiting algorithm to use
	Algorithm RateLimitAlgorithm `json:"algorithm" yaml:"algorithm"`
	
	// Scope defines how rate limits are scoped
	Scope RateLimitScope `json:"scope" yaml:"scope"`
	
	// Rate limit settings
	RequestsPerSecond float64       `json:"requests_per_second" yaml:"requests_per_second"`
	RequestsPerMinute int           `json:"requests_per_minute" yaml:"requests_per_minute"`
	RequestsPerHour   int           `json:"requests_per_hour" yaml:"requests_per_hour"`
	BurstSize         int           `json:"burst_size" yaml:"burst_size"`
	WindowSize        time.Duration `json:"window_size" yaml:"window_size"`
	
	// Custom key extraction (for ScopeCustom)
	CustomKeyExtractor string `json:"custom_key_extractor" yaml:"custom_key_extractor"`
	CustomKeyHeader    string `json:"custom_key_header" yaml:"custom_key_header"`
	
	// Per-endpoint rate limits
	EndpointLimits map[string]*EndpointRateLimit `json:"endpoint_limits" yaml:"endpoint_limits"`
	
	// Per-user rate limits
	UserLimits map[string]*UserRateLimit `json:"user_limits" yaml:"user_limits"`
	
	// IP whitelist/blacklist
	WhitelistedIPs []string `json:"whitelisted_ips" yaml:"whitelisted_ips"`
	BlacklistedIPs []string `json:"blacklisted_ips" yaml:"blacklisted_ips"`
	
	// Headers to include in rate limit response
	IncludeHeaders bool `json:"include_headers" yaml:"include_headers"`
	
	// Storage settings
	CleanupInterval    time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`
	LimiterTTL         time.Duration `json:"limiter_ttl" yaml:"limiter_ttl"`
	// Memory protection settings
	MaxLimiters        int           `json:"max_limiters" yaml:"max_limiters"`
	EnableMemoryBounds bool          `json:"enable_memory_bounds" yaml:"enable_memory_bounds"`
	
	// Error handling
	RetryAfterHeader   bool          `json:"retry_after_header" yaml:"retry_after_header"`
	CustomErrorMessage string        `json:"custom_error_message" yaml:"custom_error_message"`
	
	// Skip rate limiting for certain conditions
	SkipSuccessfulAuth bool     `json:"skip_successful_auth" yaml:"skip_successful_auth"`
	SkipPaths          []string `json:"skip_paths" yaml:"skip_paths"`
	SkipMethods        []string `json:"skip_methods" yaml:"skip_methods"`
	SkipUserAgents     []string `json:"skip_user_agents" yaml:"skip_user_agents"`
}

// EndpointRateLimit defines rate limits for specific endpoints
type EndpointRateLimit struct {
	Path              string        `json:"path" yaml:"path"`
	Method            string        `json:"method" yaml:"method"`
	RequestsPerSecond float64       `json:"requests_per_second" yaml:"requests_per_second"`
	RequestsPerMinute int           `json:"requests_per_minute" yaml:"requests_per_minute"`
	BurstSize         int           `json:"burst_size" yaml:"burst_size"`
	WindowSize        time.Duration `json:"window_size" yaml:"window_size"`
}

// UserRateLimit defines rate limits for specific users
type UserRateLimit struct {
	UserID            string        `json:"user_id" yaml:"user_id"`
	RequestsPerSecond float64       `json:"requests_per_second" yaml:"requests_per_second"`
	RequestsPerMinute int           `json:"requests_per_minute" yaml:"requests_per_minute"`
	BurstSize         int           `json:"burst_size" yaml:"burst_size"`
	WindowSize        time.Duration `json:"window_size" yaml:"window_size"`
}

// RateLimiter interface defines rate limiting operations
type RateLimiter interface {
	// Allow checks if a request is allowed
	Allow() bool
	
	// AllowN checks if N requests are allowed
	AllowN(n int) bool
	
	// Reserve reserves a token and returns a reservation
	Reserve() Reservation
	
	// ReserveN reserves N tokens and returns a reservation
	ReserveN(n int) Reservation
	
	// Limit returns the current rate limit
	Limit() rate.Limit
	
	// Burst returns the current burst size
	Burst() int
	
	// Tokens returns the number of tokens available
	Tokens() float64
}

// Reservation represents a rate limit reservation
type Reservation interface {
	// OK returns whether the reservation is valid
	OK() bool
	
	// Cancel cancels the reservation
	Cancel()
	
	// Delay returns how long to wait before the reservation becomes valid
	Delay() time.Duration
	
	// DelayFrom returns how long to wait from a specific time
	DelayFrom(time.Time) time.Duration
}

// TokenBucketLimiter implements token bucket rate limiting
type TokenBucketLimiter struct {
	limiter *rate.Limiter
	mu      sync.Mutex
}

// NewTokenBucketLimiter creates a new token bucket rate limiter using object pool
func NewTokenBucketLimiter(requestsPerSecond float64, burstSize int) *TokenBucketLimiter {
	tbl := tokenBucketLimiterPool.Get().(*TokenBucketLimiter)
	tbl.limiter = rate.NewLimiter(rate.Limit(requestsPerSecond), burstSize)
	return tbl
}

// Release returns a TokenBucketLimiter to the pool
func (tbl *TokenBucketLimiter) Release() {
	if tbl != nil {
		tbl.limiter = nil
		tokenBucketLimiterPool.Put(tbl)
	}
}

// Allow checks if a request is allowed
func (tbl *TokenBucketLimiter) Allow() bool {
	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	return tbl.limiter.Allow()
}

// AllowN checks if N requests are allowed
func (tbl *TokenBucketLimiter) AllowN(n int) bool {
	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	return tbl.limiter.AllowN(time.Now(), n)
}

// Reserve reserves a token
func (tbl *TokenBucketLimiter) Reserve() Reservation {
	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	return &tokenBucketReservation{
		reservation: tbl.limiter.Reserve(),
	}
}

// ReserveN reserves N tokens
func (tbl *TokenBucketLimiter) ReserveN(n int) Reservation {
	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	return &tokenBucketReservation{
		reservation: tbl.limiter.ReserveN(time.Now(), n),
	}
}

// Limit returns the current rate limit
func (tbl *TokenBucketLimiter) Limit() rate.Limit {
	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	return tbl.limiter.Limit()
}

// Burst returns the current burst size
func (tbl *TokenBucketLimiter) Burst() int {
	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	return tbl.limiter.Burst()
}

// Tokens returns the number of available tokens
func (tbl *TokenBucketLimiter) Tokens() float64 {
	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	return tbl.limiter.TokensAt(time.Now())
}

// tokenBucketReservation wraps rate.Reservation
type tokenBucketReservation struct {
	reservation *rate.Reservation
}

func (tbr *tokenBucketReservation) OK() bool {
	return tbr.reservation.OK()
}

func (tbr *tokenBucketReservation) Cancel() {
	tbr.reservation.Cancel()
}

func (tbr *tokenBucketReservation) Delay() time.Duration {
	return tbr.reservation.Delay()
}

func (tbr *tokenBucketReservation) DelayFrom(t time.Time) time.Duration {
	return tbr.reservation.DelayFrom(t)
}

// SlidingWindowLimiter implements sliding window rate limiting
type SlidingWindowLimiter struct {
	limit      int
	windowSize time.Duration
	requests   []time.Time
	mu         sync.Mutex
}

// NewSlidingWindowLimiter creates a new sliding window rate limiter using object pool
func NewSlidingWindowLimiter(limit int, windowSize time.Duration) *SlidingWindowLimiter {
	swl := slidingWindowLimiterPool.Get().(*SlidingWindowLimiter)
	swl.limit = limit
	swl.windowSize = windowSize
	swl.requests = swl.requests[:0] // Reset slice but keep capacity
	return swl
}

// Release returns a SlidingWindowLimiter to the pool
func (swl *SlidingWindowLimiter) Release() {
	if swl != nil {
		swl.requests = swl.requests[:0] // Clear slice
		swl.limit = 0
		swl.windowSize = 0
		slidingWindowLimiterPool.Put(swl)
	}
}

// Allow checks if a request is allowed
func (swl *SlidingWindowLimiter) Allow() bool {
	swl.mu.Lock()
	defer swl.mu.Unlock()
	
	now := time.Now()
	
	// Clean old requests
	swl.cleanOldRequests(now)
	
	// Check if we can allow the request
	if len(swl.requests) >= swl.limit {
		return false
	}
	
	// Add the request
	swl.requests = append(swl.requests, now)
	return true
}

// AllowN checks if N requests are allowed
func (swl *SlidingWindowLimiter) AllowN(n int) bool {
	swl.mu.Lock()
	defer swl.mu.Unlock()
	
	now := time.Now()
	
	// Clean old requests
	swl.cleanOldRequests(now)
	
	// Check if we can allow N requests
	if len(swl.requests)+n > swl.limit {
		return false
	}
	
	// Add N requests
	for i := 0; i < n; i++ {
		swl.requests = append(swl.requests, now)
	}
	return true
}

// Reserve reserves a token (simplified implementation)
func (swl *SlidingWindowLimiter) Reserve() Reservation {
	return &slidingWindowReservation{
		limiter: swl,
		ok:      swl.Allow(),
	}
}

// ReserveN reserves N tokens (simplified implementation)
func (swl *SlidingWindowLimiter) ReserveN(n int) Reservation {
	return &slidingWindowReservation{
		limiter: swl,
		ok:      swl.AllowN(n),
	}
}

// Limit returns the rate limit as requests per second
func (swl *SlidingWindowLimiter) Limit() rate.Limit {
	return rate.Limit(float64(swl.limit) / swl.windowSize.Seconds())
}

// Burst returns the burst size (same as limit for sliding window)
func (swl *SlidingWindowLimiter) Burst() int {
	return swl.limit
}

// Tokens returns available tokens
func (swl *SlidingWindowLimiter) Tokens() float64 {
	swl.mu.Lock()
	defer swl.mu.Unlock()
	
	swl.cleanOldRequests(time.Now())
	return float64(swl.limit - len(swl.requests))
}

func (swl *SlidingWindowLimiter) cleanOldRequests(now time.Time) {
	cutoff := now.Add(-swl.windowSize)
	var validRequests []time.Time
	
	for _, reqTime := range swl.requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}
	
	swl.requests = validRequests
}

// slidingWindowReservation implements Reservation for sliding window
type slidingWindowReservation struct {
	limiter *SlidingWindowLimiter
	ok      bool
}

func (swr *slidingWindowReservation) OK() bool {
	return swr.ok
}

func (swr *slidingWindowReservation) Cancel() {
	// For sliding window, we can't easily cancel, so this is a no-op
}

func (swr *slidingWindowReservation) Delay() time.Duration {
	if swr.ok {
		return 0
	}
	// Simplified: return window size as delay
	return swr.limiter.windowSize
}

func (swr *slidingWindowReservation) DelayFrom(t time.Time) time.Duration {
	return swr.Delay()
}

// RateLimitMiddleware implements rate limiting middleware
type RateLimitMiddleware struct {
	config *RateLimitConfig
	logger *zap.Logger
	
	// Limiter storage - either bounded or unbounded based on configuration
	limiters         map[string]RateLimiter
	boundedLimiters  *BoundedMap[string, RateLimiter]
	limitersMu       sync.RWMutex
	lastCleanup      time.Time
	
	// Precomputed maps for performance
	whitelistedIPMap map[string]bool
	blacklistedIPMap map[string]bool
	skipPathMap      map[string]bool
	skipMethodMap    map[string]bool
	skipUserAgentMap map[string]bool
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(config *RateLimitConfig, logger *zap.Logger) (*RateLimitMiddleware, error) {
	if config == nil {
		return nil, fmt.Errorf("rate limit config cannot be nil")
	}
	
	if err := ValidateBaseConfig(&config.BaseConfig); err != nil {
		return nil, fmt.Errorf("invalid base config: %w", err)
	}
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	// Set defaults
	if config.Name == "" {
		config.Name = "ratelimit"
	}
	if config.Priority == 0 {
		config.Priority = 80 // High priority, before auth but after CORS
	}
	if config.Algorithm == "" {
		config.Algorithm = TokenBucket
	}
	if config.Scope == "" {
		config.Scope = ScopeIP
	}
	if config.RequestsPerSecond == 0 && config.RequestsPerMinute == 0 {
		config.RequestsPerMinute = 60 // Default to 60 requests per minute
	}
	if config.BurstSize == 0 {
		if config.RequestsPerSecond > 0 {
			config.BurstSize = int(config.RequestsPerSecond * 2) // 2x rate as burst
		} else {
			config.BurstSize = config.RequestsPerMinute / 6 // 10 second burst
		}
	}
	if config.WindowSize == 0 {
		config.WindowSize = time.Minute
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 5 * time.Minute
	}
	if config.LimiterTTL == 0 {
		config.LimiterTTL = 1 * time.Hour
	}
	// Set defaults for memory bounds
	if config.MaxLimiters <= 0 {
		config.MaxLimiters = 50000
	}
	
	middleware := &RateLimitMiddleware{
		config:           config,
		logger:           logger,
		lastCleanup:      time.Now(),
		whitelistedIPMap: make(map[string]bool),
		blacklistedIPMap: make(map[string]bool),
		skipPathMap:      make(map[string]bool),
		skipMethodMap:    make(map[string]bool),
		skipUserAgentMap: make(map[string]bool),
	}
	
	// Initialize limiter storage based on memory bounds configuration
	if config.EnableMemoryBounds {
		boundedConfig := BoundedMapConfig{
			MaxSize:        config.MaxLimiters,
			EnableTimeouts: true,
			TTL:            config.LimiterTTL,
		}
		middleware.boundedLimiters = NewBoundedMap[string, RateLimiter](boundedConfig)
	} else {
		middleware.limiters = make(map[string]RateLimiter)
	}
	
	// Build maps for performance
	middleware.buildMaps()
	
	return middleware, nil
}

// Handler implements the Middleware interface
func (rlm *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rlm.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		
		// Check if request should skip rate limiting
		if rlm.shouldSkipRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		
		// Check blacklisted IPs
		clientIP := GetClientIP(r)
		if rlm.blacklistedIPMap[clientIP] {
			rlm.handleRateLimitExceeded(w, r, "IP blacklisted")
			return
		}
		
		// Check whitelisted IPs
		if rlm.whitelistedIPMap[clientIP] {
			next.ServeHTTP(w, r)
			return
		}
		
		// Get or create rate limiter
		limiter, key := rlm.getLimiter(r)
		if limiter == nil {
			// If we can't get a limiter, allow the request
			next.ServeHTTP(w, r)
			return
		}
		
		// Check rate limit
		if !limiter.Allow() {
			rlm.logger.Debug("Rate limit exceeded",
				zap.String("key", key),
				zap.String("client_ip", clientIP),
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
			)
			
			rlm.handleRateLimitExceeded(w, r, "Rate limit exceeded")
			return
		}
		
		// Add rate limit headers if enabled
		if rlm.config.IncludeHeaders {
			rlm.addRateLimitHeaders(w, limiter)
		}
		
		// Cleanup old limiters periodically
		rlm.periodicCleanup()
		
		next.ServeHTTP(w, r)
	})
}

// Name returns the middleware name
func (rlm *RateLimitMiddleware) Name() string {
	return rlm.config.Name
}

// Priority returns the middleware priority
func (rlm *RateLimitMiddleware) Priority() int {
	return rlm.config.Priority
}

// Config returns the middleware configuration
func (rlm *RateLimitMiddleware) Config() interface{} {
	return rlm.config
}

// Cleanup performs cleanup
func (rlm *RateLimitMiddleware) Cleanup() error {
	if rlm.config.EnableMemoryBounds && rlm.boundedLimiters != nil {
		// Clear bounded map
		rlm.boundedLimiters.Clear()
	} else {
		// Clear unbounded map
		rlm.limitersMu.Lock()
		rlm.limiters = make(map[string]RateLimiter)
		rlm.limitersMu.Unlock()
	}
	
	return nil
}

// shouldSkipRequest checks if the request should skip rate limiting
func (rlm *RateLimitMiddleware) shouldSkipRequest(r *http.Request) bool {
	// Check skip paths
	path := r.URL.Path
	if rlm.skipPathMap[path] {
		return true
	}
	
	// Check path prefixes
	for skipPath := range rlm.skipPathMap {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}
	
	// Check skip methods
	if rlm.skipMethodMap[r.Method] {
		return true
	}
	
	// Check skip user agents
	userAgent := r.Header.Get("User-Agent")
	if userAgent != "" && rlm.skipUserAgentMap[userAgent] {
		return true
	}
	
	// Check if we should skip for successful auth
	if rlm.config.SkipSuccessfulAuth {
		if user, ok := GetAuthUser(r.Context()); ok && user != nil {
			return true
		}
	}
	
	return false
}

// getLimiter gets or creates a rate limiter for the request
func (rlm *RateLimitMiddleware) getLimiter(r *http.Request) (RateLimiter, string) {
	key := rlm.extractKey(r)
	if key == "" {
		return nil, ""
	}
	
	// Use bounded map if memory bounds are enabled
	if rlm.config.EnableMemoryBounds && rlm.boundedLimiters != nil {
		return rlm.getBoundedLimiter(r, key)
	}
	
	// Use unbounded map (legacy behavior)
	rlm.limitersMu.RLock()
	limiter, exists := rlm.limiters[key]
	rlm.limitersMu.RUnlock()
	
	if exists {
		return limiter, key
	}
	
	// Create new limiter
	rlm.limitersMu.Lock()
	defer rlm.limitersMu.Unlock()
	
	// Double-check after acquiring write lock
	if limiter, exists := rlm.limiters[key]; exists {
		return limiter, key
	}
	
	limiter = rlm.createLimiter(r, key)
	if limiter != nil {
		rlm.limiters[key] = limiter
	}
	
	return limiter, key
}

// getBoundedLimiter gets or creates a rate limiter using the bounded map
func (rlm *RateLimitMiddleware) getBoundedLimiter(r *http.Request, key string) (RateLimiter, string) {
	limiter := rlm.boundedLimiters.GetOrSet(key, func() RateLimiter {
		return rlm.createLimiter(r, key)
	})
	return limiter, key
}

// createLimiter creates a new rate limiter based on the configuration
func (rlm *RateLimitMiddleware) createLimiter(r *http.Request, key string) RateLimiter {
	// Determine rate limit settings for this key
	rps, rpm, burst, windowSize := rlm.getRateLimitSettings(r, key)
	
	// Create limiter based on algorithm
	var limiter RateLimiter
	switch rlm.config.Algorithm {
	case TokenBucket:
		if rps > 0 {
			limiter = NewTokenBucketLimiter(rps, burst)
		} else if rpm > 0 {
			limiter = NewTokenBucketLimiter(float64(rpm)/60.0, burst)
		}
	case SlidingWindow:
		if rpm > 0 {
			limiter = NewSlidingWindowLimiter(rpm, windowSize)
		} else if rps > 0 {
			limiter = NewSlidingWindowLimiter(int(rps*windowSize.Seconds()), windowSize)
		}
	case FixedWindow:
		// For simplicity, use sliding window as fixed window
		if rpm > 0 {
			limiter = NewSlidingWindowLimiter(rpm, windowSize)
		} else if rps > 0 {
			limiter = NewSlidingWindowLimiter(int(rps*windowSize.Seconds()), windowSize)
		}
	}
	
	return limiter
}

// extractKey extracts the rate limiting key from the request
func (rlm *RateLimitMiddleware) extractKey(r *http.Request) string {
	switch rlm.config.Scope {
	case ScopeGlobal:
		return "global"
	case ScopeIP:
		return "ip:" + GetClientIP(r)
	case ScopeUser:
		if userID := GetUserID(r.Context()); userID != "" {
			return "user:" + userID
		}
		// Fall back to IP if no user
		return "ip:" + GetClientIP(r)
	case ScopeAPIKey:
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			return "apikey:" + apiKey
		}
		// Fall back to IP if no API key
		return "ip:" + GetClientIP(r)
	case ScopeEndpoint:
		return fmt.Sprintf("endpoint:%s:%s", r.Method, r.URL.Path)
	case ScopeCustom:
		if rlm.config.CustomKeyHeader != "" {
			if value := r.Header.Get(rlm.config.CustomKeyHeader); value != "" {
				return "custom:" + value
			}
		}
		// Custom key extraction logic could be implemented here
		return "ip:" + GetClientIP(r)
	default:
		return "ip:" + GetClientIP(r)
	}
}

// getRateLimitSettings gets rate limit settings for a specific key
func (rlm *RateLimitMiddleware) getRateLimitSettings(r *http.Request, key string) (float64, int, int, time.Duration) {
	// Check for user-specific limits
	if userID := GetUserID(r.Context()); userID != "" {
		if userLimit, exists := rlm.config.UserLimits[userID]; exists {
			return userLimit.RequestsPerSecond, userLimit.RequestsPerMinute, 
				   userLimit.BurstSize, userLimit.WindowSize
		}
	}
	
	// Check for endpoint-specific limits
	for _, endpointLimit := range rlm.config.EndpointLimits {
		if (endpointLimit.Method == "" || endpointLimit.Method == r.Method) &&
		   (endpointLimit.Path == "" || strings.HasPrefix(r.URL.Path, endpointLimit.Path)) {
			return endpointLimit.RequestsPerSecond, endpointLimit.RequestsPerMinute,
				   endpointLimit.BurstSize, endpointLimit.WindowSize
		}
	}
	
	// Use default config
	return rlm.config.RequestsPerSecond, rlm.config.RequestsPerMinute,
		   rlm.config.BurstSize, rlm.config.WindowSize
}

// addRateLimitHeaders adds rate limit headers to the response
func (rlm *RateLimitMiddleware) addRateLimitHeaders(w http.ResponseWriter, limiter RateLimiter) {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", float64(limiter.Limit())))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%.0f", limiter.Tokens()))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))
}

// handleRateLimitExceeded handles rate limit exceeded responses
func (rlm *RateLimitMiddleware) handleRateLimitExceeded(w http.ResponseWriter, r *http.Request, message string) {
	statusCode := http.StatusTooManyRequests
	
	// Set retry-after header if enabled
	if rlm.config.RetryAfterHeader {
		w.Header().Set("Retry-After", "60") // 60 seconds
	}
	
	// Use custom error message if provided
	if rlm.config.CustomErrorMessage != "" {
		message = rlm.config.CustomErrorMessage
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	// Use pooled response map for better performance
	response := rateResponsePool.Get().(map[string]interface{})
	defer func() {
		// Clear and return to pool
		for k := range response {
			delete(response, k)
		}
		rateResponsePool.Put(response)
	}()
	
	response["error"] = message
	response["timestamp"] = time.Now().Unix()
	response["retry_after"] = 60
	
	json.NewEncoder(w).Encode(response)
}

// periodicCleanup performs periodic cleanup of old limiters
func (rlm *RateLimitMiddleware) periodicCleanup() {
	now := time.Now()
	if now.Sub(rlm.lastCleanup) < rlm.config.CleanupInterval {
		return
	}
	
	if rlm.config.EnableMemoryBounds && rlm.boundedLimiters != nil {
		// Bounded map handles cleanup automatically
		cleanedCount := rlm.boundedLimiters.Cleanup()
		if cleanedCount > 0 {
			rlm.logger.Debug("Rate limiter cleanup completed",
				zap.Int("cleaned_count", cleanedCount),
				zap.Time("timestamp", now))
		}
	} else {
		// Manual cleanup for unbounded map
		rlm.limitersMu.Lock()
		defer rlm.limitersMu.Unlock()
		
		// Simple cleanup - limit the number of stored limiters
		if len(rlm.limiters) > 10000 {
			// Clear half the limiters randomly
			count := 0
			for key := range rlm.limiters {
				if count%2 == 0 {
					delete(rlm.limiters, key)
				}
				count++
			}
			rlm.logger.Warn("Rate limiter map cleanup performed due to size limit",
				zap.Int("remaining_count", len(rlm.limiters)),
				zap.Time("timestamp", now))
		}
	}
	
	rlm.lastCleanup = now
}

// GetLimiterStats returns statistics about the rate limiter storage
func (rlm *RateLimitMiddleware) GetLimiterStats() interface{} {
	if rlm.config.EnableMemoryBounds && rlm.boundedLimiters != nil {
		return rlm.boundedLimiters.Stats()
	}
	
	rlm.limitersMu.RLock()
	defer rlm.limitersMu.RUnlock()
	
	return map[string]interface{}{
		"type":         "unbounded",
		"size":         len(rlm.limiters),
		"last_cleanup": rlm.lastCleanup,
	}
}

// CleanupExpiredLimiters manually triggers cleanup of expired limiters
func (rlm *RateLimitMiddleware) CleanupExpiredLimiters() int {
	if rlm.config.EnableMemoryBounds && rlm.boundedLimiters != nil {
		return rlm.boundedLimiters.Cleanup()
	}
	
	// For unbounded maps, we don't have TTL tracking, so return 0
	return 0
}

// buildMaps precomputes maps for performance
func (rlm *RateLimitMiddleware) buildMaps() {
	// Build whitelisted IP map
	for _, ip := range rlm.config.WhitelistedIPs {
		rlm.whitelistedIPMap[ip] = true
	}
	
	// Build blacklisted IP map
	for _, ip := range rlm.config.BlacklistedIPs {
		rlm.blacklistedIPMap[ip] = true
	}
	
	// Build skip path map
	for _, path := range rlm.config.SkipPaths {
		rlm.skipPathMap[path] = true
	}
	
	// Build skip method map
	for _, method := range rlm.config.SkipMethods {
		rlm.skipMethodMap[strings.ToUpper(method)] = true
	}
	
	// Build skip user agent map
	for _, userAgent := range rlm.config.SkipUserAgents {
		rlm.skipUserAgentMap[userAgent] = true
	}
}


// Default configurations

// DefaultRateLimitConfig returns a default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Priority: 80,
			Name:     "ratelimit",
		},
		Algorithm:         TokenBucket,
		Scope:             ScopeIP,
		RequestsPerMinute: 1000,
		BurstSize:         100,
		WindowSize:        time.Minute,
		IncludeHeaders:      true,
		CleanupInterval:     5 * time.Minute,
		LimiterTTL:         1 * time.Hour,
		EnableMemoryBounds: true,      // Enable memory bounds by default
		MaxLimiters:        50000,     // Allow up to 50K rate limiters
		RetryAfterHeader:   true,
		SkipPaths:          []string{"/health", "/metrics"},
		EndpointLimits:     make(map[string]*EndpointRateLimit),
		UserLimits:         make(map[string]*UserRateLimit),
	}
}

// StrictRateLimitConfig returns a strict rate limit configuration
func StrictRateLimitConfig() *RateLimitConfig {
	config := DefaultRateLimitConfig()
	config.RequestsPerMinute = 100
	config.BurstSize = 10
	config.Algorithm = SlidingWindow
	return config
}

// PermissiveRateLimitConfig returns a permissive rate limit configuration
func PermissiveRateLimitConfig() *RateLimitConfig {
	config := DefaultRateLimitConfig()
	config.RequestsPerMinute = 10000
	config.BurstSize = 1000
	return config
}

// IPRateLimitConfig creates an IP-based rate limit configuration
func IPRateLimitConfig(requestsPerMinute int, burstSize int) *RateLimitConfig {
	config := DefaultRateLimitConfig()
	config.Scope = ScopeIP
	config.RequestsPerMinute = requestsPerMinute
	config.BurstSize = burstSize
	return config
}

// UserRateLimitConfig creates a user-based rate limit configuration
func UserRateLimitConfig(requestsPerMinute int, burstSize int) *RateLimitConfig {
	config := DefaultRateLimitConfig()
	config.Scope = ScopeUser
	config.RequestsPerMinute = requestsPerMinute
	config.BurstSize = burstSize
	return config
}

// GlobalRateLimitConfig creates a global rate limit configuration
func GlobalRateLimitConfig(requestsPerSecond float64, burstSize int) *RateLimitConfig {
	config := DefaultRateLimitConfig()
	config.Scope = ScopeGlobal
	config.RequestsPerSecond = requestsPerSecond
	config.BurstSize = burstSize
	return config
}