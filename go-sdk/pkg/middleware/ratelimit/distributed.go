package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// RedisClient defines the interface for Redis operations
type RedisClient interface {
	// Get retrieves a value by key
	Get(ctx context.Context, key string) (string, error)

	// Set stores a value with expiration
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error

	// Incr increments a key by 1
	Incr(ctx context.Context, key string) (int64, error)

	// IncrBy increments a key by a specific value
	IncrBy(ctx context.Context, key string, value int64) (int64, error)

	// Expire sets expiration on a key
	Expire(ctx context.Context, key string, expiration time.Duration) error

	// TTL returns time to live for a key
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Del deletes keys
	Del(ctx context.Context, keys ...string) (int64, error)

	// ZAdd adds members to a sorted set
	ZAdd(ctx context.Context, key string, score float64, member interface{}) error

	// ZRemRangeByScore removes members from sorted set by score range
	ZRemRangeByScore(ctx context.Context, key string, min, max float64) (int64, error)

	// ZCard returns cardinality of sorted set
	ZCard(ctx context.Context, key string) (int64, error)

	// Eval executes a Lua script
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)

	// Close closes the Redis connection
	Close() error
}

// MockRedisClient provides a mock implementation for testing
type MockRedisClient struct {
	data map[string]interface{}
	exp  map[string]time.Time
	mu   sync.RWMutex
}

// NewMockRedisClient creates a new mock Redis client
func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]interface{}),
		exp:  make(map[string]time.Time),
	}
}

// Get retrieves a value by key
func (m *MockRedisClient) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check expiration
	if expTime, exists := m.exp[key]; exists && time.Now().After(expTime) {
		delete(m.data, key)
		delete(m.exp, key)
		return "", fmt.Errorf("key not found")
	}

	if value, exists := m.data[key]; exists {
		if str, ok := value.(string); ok {
			return str, nil
		}
		return fmt.Sprintf("%v", value), nil
	}

	return "", fmt.Errorf("key not found")
}

// Set stores a value with expiration
func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = value
	if expiration > 0 {
		m.exp[key] = time.Now().Add(expiration)
	}

	return nil
}

// Incr increments a key by 1
func (m *MockRedisClient) Incr(ctx context.Context, key string) (int64, error) {
	return m.IncrBy(ctx, key, 1)
}

// IncrBy increments a key by a specific value
func (m *MockRedisClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check expiration
	if expTime, exists := m.exp[key]; exists && time.Now().After(expTime) {
		delete(m.data, key)
		delete(m.exp, key)
	}

	current := int64(0)
	if existing, exists := m.data[key]; exists {
		if num, ok := existing.(int64); ok {
			current = num
		} else if str, ok := existing.(string); ok {
			if parsed, err := strconv.ParseInt(str, 10, 64); err == nil {
				current = parsed
			}
		}
	}

	newValue := current + value
	m.data[key] = newValue
	return newValue, nil
}

// Expire sets expiration on a key
func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.data[key]; exists {
		m.exp[key] = time.Now().Add(expiration)
	}

	return nil
}

// TTL returns time to live for a key
func (m *MockRedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if expTime, exists := m.exp[key]; exists {
		remaining := time.Until(expTime)
		if remaining <= 0 {
			return -2 * time.Second, nil // Key expired
		}
		return remaining, nil
	}

	if _, exists := m.data[key]; exists {
		return -1 * time.Second, nil // Key exists but no expiration
	}

	return -2 * time.Second, nil // Key doesn't exist
}

// Del deletes keys
func (m *MockRedisClient) Del(ctx context.Context, keys ...string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := int64(0)
	for _, key := range keys {
		if _, exists := m.data[key]; exists {
			delete(m.data, key)
			delete(m.exp, key)
			count++
		}
	}

	return count, nil
}

// ZAdd adds members to a sorted set
func (m *MockRedisClient) ZAdd(ctx context.Context, key string, score float64, member interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simplified sorted set implementation
	setKey := key + ":zset"
	if _, exists := m.data[setKey]; !exists {
		m.data[setKey] = make(map[string]float64)
	}

	if zset, ok := m.data[setKey].(map[string]float64); ok {
		memberStr := fmt.Sprintf("%v", member)
		zset[memberStr] = score
	}

	return nil
}

// ZRemRangeByScore removes members from sorted set by score range
func (m *MockRedisClient) ZRemRangeByScore(ctx context.Context, key string, min, max float64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	setKey := key + ":zset"
	count := int64(0)

	if zset, ok := m.data[setKey].(map[string]float64); ok {
		for member, score := range zset {
			if score >= min && score <= max {
				delete(zset, member)
				count++
			}
		}
	}

	return count, nil
}

// ZCard returns cardinality of sorted set
func (m *MockRedisClient) ZCard(ctx context.Context, key string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	setKey := key + ":zset"
	if zset, ok := m.data[setKey].(map[string]float64); ok {
		return int64(len(zset)), nil
	}

	return 0, nil
}

// Eval executes a Lua script
func (m *MockRedisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	// Simplified Lua script execution for common rate limiting patterns
	// In a real implementation, this would execute actual Lua scripts

	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys provided")
	}

	key := keys[0]

	// Token bucket script simulation
	if len(args) >= 4 {
		capacity, _ := strconv.ParseInt(fmt.Sprintf("%v", args[0]), 10, 64)
		rate, _ := strconv.ParseInt(fmt.Sprintf("%v", args[1]), 10, 64)
		now, _ := strconv.ParseInt(fmt.Sprintf("%v", args[2]), 10, 64)
		requested, _ := strconv.ParseInt(fmt.Sprintf("%v", args[3]), 10, 64)

		// Get current bucket state
		bucketData, _ := m.Get(ctx, key)

		bucket := struct {
			Tokens   int64 `json:"tokens"`
			LastFill int64 `json:"last_fill"`
		}{
			Tokens:   capacity,
			LastFill: now,
		}

		if bucketData != "" {
			json.Unmarshal([]byte(bucketData), &bucket)
		}

		// Calculate tokens to add
		elapsed := now - bucket.LastFill
		tokensToAdd := elapsed * rate / 1000 // Assuming rate is per second and elapsed in milliseconds
		bucket.Tokens = min(capacity, bucket.Tokens+tokensToAdd)
		bucket.LastFill = now

		allowed := bucket.Tokens >= requested
		if allowed {
			bucket.Tokens -= requested
		}

		// Save bucket state
		bucketJSON, _ := json.Marshal(bucket)
		m.Set(ctx, key, string(bucketJSON), time.Hour) // TTL of 1 hour

		return map[string]interface{}{
			"allowed": allowed,
			"tokens":  bucket.Tokens,
			"reset":   bucket.LastFill + (capacity * 1000 / rate),
		}, nil
	}

	return nil, fmt.Errorf("unsupported script")
}

// Close closes the Redis connection
func (m *MockRedisClient) Close() error {
	return nil
}

// DistributedTokenBucket implements distributed token bucket using Redis
type DistributedTokenBucket struct {
	redis    RedisClient
	rate     int64
	capacity int64
	script   string
}

// Lua script for atomic token bucket operations
const tokenBucketScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local bucket = redis.call('HMGET', key, 'tokens', 'last_fill')
local tokens = tonumber(bucket[1]) or capacity
local last_fill = tonumber(bucket[2]) or now

-- Calculate tokens to add based on elapsed time
local elapsed = math.max(0, now - last_fill)
local tokens_to_add = math.floor(elapsed * rate / 1000) -- rate per second, elapsed in ms
tokens = math.min(capacity, tokens + tokens_to_add)

-- Check if request can be allowed
local allowed = tokens >= requested
if allowed then
    tokens = tokens - requested
end

-- Update bucket state
redis.call('HMSET', key, 'tokens', tokens, 'last_fill', now)
redis.call('EXPIRE', key, 3600) -- Expire in 1 hour

return {
    allowed and 1 or 0,
    tokens,
    math.ceil((capacity - tokens) * 1000 / rate) + now
}
`

// NewDistributedTokenBucket creates a new distributed token bucket rate limiter
func NewDistributedTokenBucket(redis RedisClient, rate, capacity int64) *DistributedTokenBucket {
	return &DistributedTokenBucket{
		redis:    redis,
		rate:     rate,
		capacity: capacity,
		script:   tokenBucketScript,
	}
}

// Allow checks if a request should be allowed using distributed token bucket
func (dtb *DistributedTokenBucket) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	now := time.Now().UnixMilli()

	result, err := dtb.redis.Eval(ctx, dtb.script, []string{key}, dtb.capacity, dtb.rate, now, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to execute rate limit script: %w", err)
	}

	// Parse script result
	if resultArray, ok := result.([]interface{}); ok && len(resultArray) >= 3 {
		allowed := false
		if allowedVal, ok := resultArray[0].(int64); ok && allowedVal == 1 {
			allowed = true
		}

		remaining, _ := resultArray[1].(int64)
		resetTime, _ := resultArray[2].(int64)

		return &RateLimitResult{
			Allowed:   allowed,
			Remaining: remaining,
			ResetTime: time.UnixMilli(resetTime),
		}, nil
	}

	return nil, fmt.Errorf("unexpected script result format")
}

// Reset resets the distributed token bucket for a given key
func (dtb *DistributedTokenBucket) Reset(ctx context.Context, key string) error {
	_, err := dtb.redis.Del(ctx, key)
	return err
}

// GetInfo returns current rate limit information
func (dtb *DistributedTokenBucket) GetInfo(ctx context.Context, key string) (*RateLimitResult, error) {
	tokens, err := dtb.redis.Get(ctx, key+":tokens")
	if err != nil {
		// Key doesn't exist, return full capacity
		return &RateLimitResult{
			Allowed:   true,
			Remaining: dtb.capacity,
			ResetTime: time.Now().Add(time.Duration(dtb.capacity/dtb.rate) * time.Second),
		}, nil
	}

	remaining, _ := strconv.ParseInt(tokens, 10, 64)

	return &RateLimitResult{
		Allowed:   remaining > 0,
		Remaining: remaining,
		ResetTime: time.Now().Add(time.Duration(dtb.capacity/dtb.rate) * time.Second),
	}, nil
}

// Algorithm returns the algorithm name
func (dtb *DistributedTokenBucket) Algorithm() RateLimitAlgorithm {
	return AlgorithmTokenBucket
}

// DistributedSlidingWindow implements distributed sliding window using Redis
type DistributedSlidingWindow struct {
	redis      RedisClient
	limit      int64
	windowSize time.Duration
}

// NewDistributedSlidingWindow creates a new distributed sliding window rate limiter
func NewDistributedSlidingWindow(redis RedisClient, limit int64, windowSize time.Duration) *DistributedSlidingWindow {
	return &DistributedSlidingWindow{
		redis:      redis,
		limit:      limit,
		windowSize: windowSize,
	}
}

// Allow checks if a request should be allowed using distributed sliding window
func (dsw *DistributedSlidingWindow) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-dsw.windowSize)

	// Remove expired entries
	_, err := dsw.redis.ZRemRangeByScore(ctx, key, 0, float64(windowStart.UnixMilli()))
	if err != nil {
		return nil, fmt.Errorf("failed to clean expired entries: %w", err)
	}

	// Count current requests in window
	count, err := dsw.redis.ZCard(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to count requests: %w", err)
	}

	result := &RateLimitResult{
		Remaining:     dsw.limit - count,
		ResetTime:     now.Add(dsw.windowSize),
		TotalRequests: count + 1,
	}

	if count < dsw.limit {
		// Add current request to window
		err = dsw.redis.ZAdd(ctx, key, float64(now.UnixMilli()), now.UnixMilli())
		if err != nil {
			return nil, fmt.Errorf("failed to add request to window: %w", err)
		}

		// Set expiration for the key
		err = dsw.redis.Expire(ctx, key, dsw.windowSize+time.Minute)
		if err != nil {
			return nil, fmt.Errorf("failed to set expiration: %w", err)
		}

		result.Allowed = true
		result.Remaining = dsw.limit - count - 1
	} else {
		result.Allowed = false
		// Calculate retry after based on oldest request in window
		result.RetryAfter = dsw.windowSize / time.Duration(dsw.limit)
	}

	return result, nil
}

// Reset resets the distributed sliding window for a given key
func (dsw *DistributedSlidingWindow) Reset(ctx context.Context, key string) error {
	_, err := dsw.redis.Del(ctx, key)
	return err
}

// GetInfo returns current rate limit information
func (dsw *DistributedSlidingWindow) GetInfo(ctx context.Context, key string) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-dsw.windowSize)

	// Remove expired entries
	_, err := dsw.redis.ZRemRangeByScore(ctx, key, 0, float64(windowStart.UnixMilli()))
	if err != nil {
		return nil, fmt.Errorf("failed to clean expired entries: %w", err)
	}

	// Count current requests in window
	count, err := dsw.redis.ZCard(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to count requests: %w", err)
	}

	return &RateLimitResult{
		Allowed:       count < dsw.limit,
		Remaining:     dsw.limit - count,
		ResetTime:     now.Add(dsw.windowSize),
		TotalRequests: count,
	}, nil
}

// Algorithm returns the algorithm name
func (dsw *DistributedSlidingWindow) Algorithm() RateLimitAlgorithm {
	return AlgorithmSlidingWindow
}

// DistributedRateLimitMiddleware extends RateLimitMiddleware with distributed support
type DistributedRateLimitMiddleware struct {
	*RateLimitMiddleware
	redisClient RedisClient
}

// NewDistributedRateLimitMiddleware creates a new distributed rate limiting middleware
func NewDistributedRateLimitMiddleware(config *RateLimitConfig, redisClient RedisClient) (*DistributedRateLimitMiddleware, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if redisClient == nil {
		redisClient = NewMockRedisClient()
	}

	// Create distributed rate limiter based on algorithm
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
		rateLimiter = NewDistributedTokenBucket(redisClient, rate, burst)
	case AlgorithmSlidingWindow:
		rateLimiter = NewDistributedSlidingWindow(redisClient, config.RequestsPerUnit, config.Unit)
	default:
		return nil, fmt.Errorf("unsupported distributed rate limit algorithm: %s", config.Algorithm)
	}

	// Create base middleware
	baseMiddleware, err := NewRateLimitMiddleware(config)
	if err != nil {
		return nil, err
	}

	// Replace with distributed rate limiter
	baseMiddleware.rateLimiter = rateLimiter

	return &DistributedRateLimitMiddleware{
		RateLimitMiddleware: baseMiddleware,
		redisClient:         redisClient,
	}, nil
}

// Name returns middleware name
func (drl *DistributedRateLimitMiddleware) Name() string {
	return "distributed_rate_limit"
}

// Close closes the Redis connection
func (drl *DistributedRateLimitMiddleware) Close() error {
	return drl.redisClient.Close()
}
