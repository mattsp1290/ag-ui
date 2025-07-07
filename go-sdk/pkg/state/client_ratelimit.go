package state

import (
	"context"
	"sync"
	"time"
	
	"golang.org/x/time/rate"
)

// ClientRateLimiterConfig defines rate limiting configuration for per-client rate limiting
type ClientRateLimiterConfig struct {
	// Rate limit per second
	RatePerSecond int
	// Burst size
	BurstSize int
	// Maximum number of tracked clients
	MaxClients int
	// TTL for inactive clients
	ClientTTL time.Duration
	// Cleanup interval
	CleanupInterval time.Duration
}

// DefaultClientRateLimiterConfig returns default rate limiter configuration
func DefaultClientRateLimiterConfig() ClientRateLimiterConfig {
	return ClientRateLimiterConfig{
		RatePerSecond:   DefaultClientRateLimit,      // Operations per second per client
		BurstSize:       DefaultClientBurstSize,      // Allow bursts up to this size
		MaxClients:      DefaultMaxClients,           // Track up to this many clients
		ClientTTL:       DefaultClientTTL,            // Remove inactive clients after this time
		CleanupInterval: DefaultClientCleanupInterval, // Run cleanup at this frequency
	}
}

// ClientRateLimiter provides per-client rate limiting
type ClientRateLimiter struct {
	limiters map[string]*clientLimiter
	mu       sync.RWMutex
	config   ClientRateLimiterConfig
	
	// Cleanup management
	lastCleanup time.Time
	cleanupMu   sync.Mutex
}

// clientLimiter tracks rate limiting for a single client
type clientLimiter struct {
	limiter      *rate.Limiter
	lastAccessed time.Time
	mu           sync.RWMutex
}

// NewClientRateLimiter creates a new rate limiter
func NewClientRateLimiter(config ClientRateLimiterConfig) *ClientRateLimiter {
	if config.MaxClients <= 0 {
		config.MaxClients = DefaultClientRateLimiterConfig().MaxClients
	}
	if config.RatePerSecond <= 0 {
		config.RatePerSecond = DefaultClientRateLimiterConfig().RatePerSecond
	}
	if config.BurstSize <= 0 {
		config.BurstSize = DefaultClientRateLimiterConfig().BurstSize
	}
	if config.ClientTTL <= 0 {
		config.ClientTTL = DefaultClientRateLimiterConfig().ClientTTL
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = DefaultClientRateLimiterConfig().CleanupInterval
	}
	
	return &ClientRateLimiter{
		limiters:    make(map[string]*clientLimiter),
		config:      config,
		lastCleanup: time.Now(),
	}
}

// Allow checks if the operation is allowed for the given client ID
func (rl *ClientRateLimiter) Allow(clientID string) bool {
	rl.mu.RLock()
	client, exists := rl.limiters[clientID]
	rl.mu.RUnlock()
	
	if exists {
		client.mu.Lock()
		client.lastAccessed = time.Now()
		client.mu.Unlock()
		return client.limiter.Allow()
	}
	
	// Create new limiter for client
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Double-check after acquiring write lock
	if client, exists = rl.limiters[clientID]; exists {
		client.mu.Lock()
		client.lastAccessed = time.Now()
		client.mu.Unlock()
		return client.limiter.Allow()
	}
	
	// Check if we need to clean up before adding new client
	if len(rl.limiters) >= rl.config.MaxClients {
		rl.cleanupLocked()
	}
	
	// Create new client limiter
	client = &clientLimiter{
		limiter:      rate.NewLimiter(rate.Limit(rl.config.RatePerSecond), rl.config.BurstSize),
		lastAccessed: time.Now(),
	}
	rl.limiters[clientID] = client
	
	// Trigger cleanup if needed
	rl.maybeCleanup()
	
	return client.limiter.Allow()
}

// AllowN checks if n operations are allowed for the given client ID
func (rl *ClientRateLimiter) AllowN(clientID string, n int) bool {
	rl.mu.RLock()
	client, exists := rl.limiters[clientID]
	rl.mu.RUnlock()
	
	if exists {
		client.mu.Lock()
		client.lastAccessed = time.Now()
		client.mu.Unlock()
		return client.limiter.AllowN(time.Now(), n)
	}
	
	// For new clients, check if n is within burst size
	if n > rl.config.BurstSize {
		return false
	}
	
	// Create new limiter and check
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Double-check after acquiring write lock
	if client, exists = rl.limiters[clientID]; exists {
		client.mu.Lock()
		client.lastAccessed = time.Now()
		client.mu.Unlock()
		return client.limiter.AllowN(time.Now(), n)
	}
	
	// Check if we need to clean up before adding new client
	if len(rl.limiters) >= rl.config.MaxClients {
		rl.cleanupLocked()
	}
	
	// Create new client limiter
	client = &clientLimiter{
		limiter:      rate.NewLimiter(rate.Limit(rl.config.RatePerSecond), rl.config.BurstSize),
		lastAccessed: time.Now(),
	}
	rl.limiters[clientID] = client
	
	// Trigger cleanup if needed
	rl.maybeCleanup()
	
	return client.limiter.AllowN(time.Now(), n)
}

// Wait blocks until the operation is allowed for the given client ID
func (rl *ClientRateLimiter) Wait(clientID string) error {
	limiter := rl.getOrCreateLimiter(clientID)
	return limiter.Wait(context.Background())
}

// WaitN blocks until n operations are allowed for the given client ID
func (rl *ClientRateLimiter) WaitN(clientID string, n int) error {
	limiter := rl.getOrCreateLimiter(clientID)
	return limiter.WaitN(context.Background(), n)
}

// Reserve returns a reservation for future use
func (rl *ClientRateLimiter) Reserve(clientID string) *rate.Reservation {
	limiter := rl.getOrCreateLimiter(clientID)
	return limiter.Reserve()
}

// ReserveN returns a reservation for n future uses
func (rl *ClientRateLimiter) ReserveN(clientID string, n int) *rate.Reservation {
	limiter := rl.getOrCreateLimiter(clientID)
	return limiter.ReserveN(time.Now(), n)
}

// getOrCreateLimiter gets or creates a rate limiter for the client
func (rl *ClientRateLimiter) getOrCreateLimiter(clientID string) *rate.Limiter {
	rl.mu.RLock()
	client, exists := rl.limiters[clientID]
	rl.mu.RUnlock()
	
	if exists {
		client.mu.Lock()
		client.lastAccessed = time.Now()
		client.mu.Unlock()
		return client.limiter
	}
	
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Double-check after acquiring write lock
	if client, exists = rl.limiters[clientID]; exists {
		client.mu.Lock()
		client.lastAccessed = time.Now()
		client.mu.Unlock()
		return client.limiter
	}
	
	// Check if we need to clean up before adding new client
	if len(rl.limiters) >= rl.config.MaxClients {
		rl.cleanupLocked()
	}
	
	// Create new client limiter
	client = &clientLimiter{
		limiter:      rate.NewLimiter(rate.Limit(rl.config.RatePerSecond), rl.config.BurstSize),
		lastAccessed: time.Now(),
	}
	rl.limiters[clientID] = client
	
	// Trigger cleanup if needed
	rl.maybeCleanup()
	
	return client.limiter
}

// maybeCleanup checks if cleanup should be triggered
func (rl *ClientRateLimiter) maybeCleanup() {
	rl.cleanupMu.Lock()
	defer rl.cleanupMu.Unlock()
	
	now := time.Now()
	if now.Sub(rl.lastCleanup) >= rl.config.CleanupInterval {
		rl.lastCleanup = now
		go rl.cleanup()
	}
}

// cleanup removes expired client limiters
func (rl *ClientRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.cleanupLocked()
}

// cleanupLocked performs cleanup while holding the lock
func (rl *ClientRateLimiter) cleanupLocked() {
	now := time.Now()
	cutoff := now.Add(-rl.config.ClientTTL)
	
	// Find expired clients
	toDelete := make([]string, 0)
	for clientID, client := range rl.limiters {
		client.mu.RLock()
		expired := client.lastAccessed.Before(cutoff)
		client.mu.RUnlock()
		if expired {
			toDelete = append(toDelete, clientID)
		}
	}
	
	// Remove expired clients
	for _, clientID := range toDelete {
		delete(rl.limiters, clientID)
	}
	
	// If still over limit, remove oldest entries
	if len(rl.limiters) >= rl.config.MaxClients {
		// Find oldest entries
		type clientAge struct {
			id           string
			lastAccessed time.Time
		}
		
		clients := make([]clientAge, 0, len(rl.limiters))
		for id, client := range rl.limiters {
			client.mu.RLock()
			lastAccessed := client.lastAccessed
			client.mu.RUnlock()
			clients = append(clients, clientAge{id: id, lastAccessed: lastAccessed})
		}
		
		// Sort by last accessed (oldest first)
		// Simple bubble sort for small datasets
		for i := 0; i < len(clients)-1; i++ {
			for j := 0; j < len(clients)-i-1; j++ {
				if clients[j].lastAccessed.After(clients[j+1].lastAccessed) {
					clients[j], clients[j+1] = clients[j+1], clients[j]
				}
			}
		}
		
		// Remove oldest entries based on LRU eviction percentage
		toRemove := len(rl.limiters) / DefaultLRUEvictionPercent
		if toRemove < 1 {
			toRemove = 1
		}
		
		for i := 0; i < toRemove && i < len(clients); i++ {
			delete(rl.limiters, clients[i].id)
		}
	}
}

// GetClientCount returns the current number of tracked clients
func (rl *ClientRateLimiter) GetClientCount() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return len(rl.limiters)
}

// Reset removes all client limiters
func (rl *ClientRateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.limiters = make(map[string]*clientLimiter)
	rl.lastCleanup = time.Now()
}