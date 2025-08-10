package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ValidationCacheEntry represents a cached validation result
type ValidationCacheEntry struct {
	Result    error     // nil means validation passed
	ExpiresAt time.Time
	CreatedAt time.Time
	HitCount  int64
}

// ValidationCacheMetrics contains cache performance metrics
type ValidationCacheMetrics struct {
	Hits           int64
	Misses         int64
	Evictions      int64
	Invalidations  int64
	Size           int64
	MaxSize        int64
	TTL            time.Duration
	LastCleanup    time.Time
	TotalChecks    int64
}

// ValidationCacheConfig contains configuration for the validation cache
type ValidationCacheConfig struct {
	MaxSize       int           // Maximum number of entries
	TTL           time.Duration // Time-to-live for cache entries
	CleanupPeriod time.Duration // How often to run cleanup
	Enabled       bool          // Whether caching is enabled
}

// DefaultValidationCacheConfig returns a sensible default cache configuration
func DefaultValidationCacheConfig() *ValidationCacheConfig {
	return &ValidationCacheConfig{
		MaxSize:       1000,
		TTL:           5 * time.Minute,
		CleanupPeriod: time.Minute,
		Enabled:       true,
	}
}

// ValidationCache provides thread-safe caching of validation results
type ValidationCache struct {
	mu          sync.RWMutex
	entries     map[string]*ValidationCacheEntry
	config      *ValidationCacheConfig
	metrics     *ValidationCacheMetrics
	cleanupDone chan struct{}
	stopped     int32
}

// NewValidationCache creates a new validation cache with the given configuration
func NewValidationCache(config *ValidationCacheConfig) *ValidationCache {
	if config == nil {
		config = DefaultValidationCacheConfig()
	}

	cache := &ValidationCache{
		entries: make(map[string]*ValidationCacheEntry),
		config:  config,
		metrics: &ValidationCacheMetrics{
			MaxSize: int64(config.MaxSize),
			TTL:     config.TTL,
		},
		cleanupDone: make(chan struct{}),
	}

	// Start background cleanup goroutine
	if config.Enabled {
		go cache.cleanupRoutine()
	}

	return cache
}

// generateCacheKey generates a deterministic hash key for the configuration data
func (c *ValidationCache) generateCacheKey(validatorName string, data map[string]interface{}) string {
	// Create a consistent JSON representation
	jsonData, err := json.Marshal(map[string]interface{}{
		"validator": validatorName,
		"data":      data,
	})
	if err != nil {
		// Fallback to a simple string representation if JSON marshaling fails
		return fmt.Sprintf("%s:%v", validatorName, data)
	}

	// Generate SHA256 hash
	hasher := sha256.New()
	hasher.Write(jsonData)
	return hex.EncodeToString(hasher.Sum(nil))
}

// Get retrieves a cached validation result
func (c *ValidationCache) Get(validatorName string, data map[string]interface{}) (error, bool) {
	if !c.config.Enabled {
		return nil, false
	}

	key := c.generateCacheKey(validatorName, data)

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	atomic.AddInt64(&c.metrics.TotalChecks, 1)

	if !exists {
		atomic.AddInt64(&c.metrics.Misses, 1)
		return nil, false
	}

	// Check if entry has expired
	if time.Now().After(entry.ExpiresAt) {
		// Remove expired entry
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		atomic.AddInt64(&c.metrics.Misses, 1)
		atomic.AddInt64(&c.metrics.Evictions, 1)
		return nil, false
	}

	// Update hit count and metrics
	atomic.AddInt64(&entry.HitCount, 1)
	atomic.AddInt64(&c.metrics.Hits, 1)

	return entry.Result, true
}

// Put stores a validation result in the cache
func (c *ValidationCache) Put(validatorName string, data map[string]interface{}, result error) {
	if !c.config.Enabled {
		return
	}

	key := c.generateCacheKey(validatorName, data)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict entries to make room
	if len(c.entries) >= c.config.MaxSize {
		c.evictLRU()
	}

	// Create new cache entry
	entry := &ValidationCacheEntry{
		Result:    result,
		ExpiresAt: time.Now().Add(c.config.TTL),
		CreatedAt: time.Now(),
		HitCount:  0,
	}

	c.entries[key] = entry
	atomic.StoreInt64(&c.metrics.Size, int64(len(c.entries)))
}

// evictLRU evicts the least recently used entry (based on creation time and hit count)
func (c *ValidationCache) evictLRU() {
	if len(c.entries) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	var lowestHitCount int64 = -1

	// Find the entry with the oldest creation time and lowest hit count
	for key, entry := range c.entries {
		if lowestHitCount == -1 || entry.HitCount < lowestHitCount || 
		   (entry.HitCount == lowestHitCount && entry.CreatedAt.Before(oldestTime)) {
			oldestKey = key
			oldestTime = entry.CreatedAt
			lowestHitCount = entry.HitCount
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		atomic.AddInt64(&c.metrics.Evictions, 1)
	}
}

// Invalidate removes all cached entries (useful when configuration sources change)
func (c *ValidationCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	entriesCount := len(c.entries)
	c.entries = make(map[string]*ValidationCacheEntry)
	atomic.StoreInt64(&c.metrics.Size, 0)
	atomic.AddInt64(&c.metrics.Invalidations, int64(entriesCount))
}

// InvalidateValidator removes all cached entries for a specific validator
func (c *ValidationCache) InvalidateValidator(validatorName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var keysToDelete []string
	for key := range c.entries {
		// Since our cache key includes the validator name, we can check if it matches
		// We need to be careful here as the key is a hash, so we'll need a different approach
		// For now, we'll invalidate all entries - a more sophisticated approach would 
		// track validator-specific keys separately
		keysToDelete = append(keysToDelete, key)
	}

	for _, key := range keysToDelete {
		delete(c.entries, key)
		atomic.AddInt64(&c.metrics.Invalidations, 1)
	}

	atomic.StoreInt64(&c.metrics.Size, int64(len(c.entries)))
}

// GetMetrics returns a copy of the current cache metrics
func (c *ValidationCache) GetMetrics() ValidationCacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return ValidationCacheMetrics{
		Hits:           atomic.LoadInt64(&c.metrics.Hits),
		Misses:         atomic.LoadInt64(&c.metrics.Misses),
		Evictions:      atomic.LoadInt64(&c.metrics.Evictions),
		Invalidations:  atomic.LoadInt64(&c.metrics.Invalidations),
		Size:           atomic.LoadInt64(&c.metrics.Size),
		MaxSize:        c.metrics.MaxSize,
		TTL:            c.metrics.TTL,
		LastCleanup:    c.metrics.LastCleanup,
		TotalChecks:    atomic.LoadInt64(&c.metrics.TotalChecks),
	}
}

// GetHitRatio returns the cache hit ratio as a percentage
func (c *ValidationCache) GetHitRatio() float64 {
	hits := atomic.LoadInt64(&c.metrics.Hits)
	totalChecks := atomic.LoadInt64(&c.metrics.TotalChecks)

	if totalChecks == 0 {
		return 0.0
	}

	return float64(hits) / float64(totalChecks) * 100.0
}

// cleanupRoutine runs periodic cleanup of expired entries
func (c *ValidationCache) cleanupRoutine() {
	ticker := time.NewTicker(c.config.CleanupPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt32(&c.stopped) == 1 {
				return
			}
			c.cleanup()
		case <-c.cleanupDone:
			return
		}
	}
}

// cleanup removes expired entries from the cache
func (c *ValidationCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var keysToDelete []string

	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(c.entries, key)
		atomic.AddInt64(&c.metrics.Evictions, 1)
	}

	atomic.StoreInt64(&c.metrics.Size, int64(len(c.entries)))
	c.metrics.LastCleanup = now
}

// Stop gracefully stops the cache and its cleanup routine
func (c *ValidationCache) Stop() {
	if atomic.CompareAndSwapInt32(&c.stopped, 0, 1) {
		close(c.cleanupDone)
	}
}

// Size returns the current number of entries in the cache
func (c *ValidationCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Clear removes all entries from the cache
func (c *ValidationCache) Clear() {
	c.Invalidate()
}

// SetTTL updates the time-to-live for new cache entries
func (c *ValidationCache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.TTL = ttl
	c.metrics.TTL = ttl
}

// SetMaxSize updates the maximum cache size
func (c *ValidationCache) SetMaxSize(maxSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.MaxSize = maxSize
	c.metrics.MaxSize = int64(maxSize)

	// If current size exceeds new max size, evict entries
	for len(c.entries) > maxSize {
		c.evictLRU()
	}
	atomic.StoreInt64(&c.metrics.Size, int64(len(c.entries)))
}