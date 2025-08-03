package security

import (
	"fmt"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// RateLimiter implements rate limiting for events
type RateLimiter struct {
	config        *SecurityConfig
	globalBucket  *TokenBucket
	eventBuckets  map[events.EventType]*TokenBucket
	sourceBuckets map[string]*TokenBucket
	mutex         sync.RWMutex
}

// TokenBucket implements a token bucket algorithm
type TokenBucket struct {
	capacity     int
	tokens       int
	refillRate   int
	lastRefill   time.Time
	mutex        sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *SecurityConfig) *RateLimiter {
	rl := &RateLimiter{
		config:        config,
		eventBuckets:  make(map[events.EventType]*TokenBucket),
		sourceBuckets: make(map[string]*TokenBucket),
	}
	
	// Initialize global bucket
	rl.globalBucket = NewTokenBucket(config.RateLimitPerMinute, config.RateLimitPerMinute)
	
	// Initialize per-event-type buckets
	for eventType, limit := range config.RateLimitPerEventType {
		rl.eventBuckets[eventType] = NewTokenBucket(limit, limit)
	}
	
	return rl
}

// CheckLimit checks if an event is within rate limits
func (rl *RateLimiter) CheckLimit(event events.Event) error {
	if !rl.config.EnableRateLimiting {
		return nil
	}
	
	// Check global rate limit
	if !rl.globalBucket.TryConsume(1) {
		return fmt.Errorf("global rate limit exceeded")
	}
	
	// Check event-type specific limit
	rl.mutex.RLock()
	eventBucket, exists := rl.eventBuckets[event.Type()]
	rl.mutex.RUnlock()
	
	if exists && !eventBucket.TryConsume(1) {
		return fmt.Errorf("rate limit exceeded for event type %s", event.Type())
	}
	
	// Check source-specific limit (if we can extract source)
	// This is a placeholder - in real implementation, extract source from event context
	
	return nil
}

// UpdateConfig updates the rate limiter configuration
func (rl *RateLimiter) UpdateConfig(config *SecurityConfig) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	
	rl.config = config
	
	// Update global bucket
	rl.globalBucket = NewTokenBucket(config.RateLimitPerMinute, config.RateLimitPerMinute)
	
	// Update event type buckets
	rl.eventBuckets = make(map[events.EventType]*TokenBucket)
	for eventType, limit := range config.RateLimitPerEventType {
		rl.eventBuckets[eventType] = NewTokenBucket(limit, limit)
	}
}

// GetCurrentLimits returns current token counts
func (rl *RateLimiter) GetCurrentLimits() map[string]int {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()
	
	limits := make(map[string]int)
	limits["global"] = rl.globalBucket.GetTokens()
	
	for eventType, bucket := range rl.eventBuckets {
		limits[string(eventType)] = bucket.GetTokens()
	}
	
	return limits
}

// NewTokenBucket creates a new token bucket
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// TryConsume attempts to consume tokens
func (tb *TokenBucket) TryConsume(tokens int) bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	
	tb.refill()
	
	if tb.tokens >= tokens {
		tb.tokens -= tokens
		return true
	}
	
	return false
}

// refill adds tokens based on elapsed time
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	
	// Calculate tokens to add (rate is per minute)
	tokensToAdd := int(elapsed.Seconds() * float64(tb.refillRate) / 60.0)
	
	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefill = now
	}
}

// GetTokens returns current token count
func (tb *TokenBucket) GetTokens() int {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	
	tb.refill()
	return tb.tokens
}