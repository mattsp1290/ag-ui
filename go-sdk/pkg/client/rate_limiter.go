package client

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	rate       int           // tokens per second
	burst      int           // maximum burst size
	tokens     int           // current tokens
	lastRefill time.Time     // last refill time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate, burst int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     burst,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed under the rate limit
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	
	// Calculate tokens to add based on elapsed time
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * rl.rate
	
	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.burst {
			rl.tokens = rl.burst
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

// Tokens returns the current number of available tokens
func (rl *RateLimiter) Tokens() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.tokens
}

// SetRate updates the rate limit
func (rl *RateLimiter) SetRate(rate int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.rate = rate
}

// SetBurst updates the burst size
func (rl *RateLimiter) SetBurst(burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.burst = burst
	if rl.tokens > burst {
		rl.tokens = burst
	}
}