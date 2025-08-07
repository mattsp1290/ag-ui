package server

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
)

// BoundedMap provides a thread-safe map with LRU eviction to prevent memory exhaustion
type BoundedMap[K comparable, V any] struct {
	mu      sync.RWMutex
	data    map[K]*list.Element
	lruList *list.List
	maxSize int

	// Optional cleanup configuration
	enableTimeouts bool
	ttl            time.Duration
	lastCleanup    time.Time
	cleanupMu      sync.Mutex

	// Metrics
	hits      int64
	misses    int64
	evictions int64
	timeouts  int64
	retries   int64
}

// entry represents a key-value pair with timestamp
type entry[K comparable, V any] struct {
	key       K
	value     V
	timestamp time.Time
}

// BoundedMapConfig contains configuration options for BoundedMap
type BoundedMapConfig struct {
	MaxSize        int           `json:"max_size" yaml:"max_size"`
	EnableTimeouts bool          `json:"enable_timeouts" yaml:"enable_timeouts"`
	TTL            time.Duration `json:"ttl" yaml:"ttl"`
}

// NewBoundedMap creates a new bounded map with LRU eviction
func NewBoundedMap[K comparable, V any](config BoundedMapConfig) *BoundedMap[K, V] {
	if config.MaxSize <= 0 {
		config.MaxSize = 10000 // Default maximum size
	}
	if config.EnableTimeouts && config.TTL <= 0 {
		config.TTL = 30 * time.Minute // Default TTL
	}

	return &BoundedMap[K, V]{
		data:           make(map[K]*list.Element),
		lruList:        list.New(),
		maxSize:        config.MaxSize,
		enableTimeouts: config.EnableTimeouts,
		ttl:            config.TTL,
		lastCleanup:    time.Now(),
	}
}

// Get retrieves a value from the map
func (bm *BoundedMap[K, V]) Get(key K) (V, bool) {
	bm.mu.RLock()
	element, exists := bm.data[key]
	bm.mu.RUnlock()

	if !exists {
		atomic.AddInt64(&bm.misses, 1)
		var zero V
		return zero, false
	}

	// Check if entry has expired
	if bm.enableTimeouts {
		entry := element.Value.(*entry[K, V])
		if time.Since(entry.timestamp) > bm.ttl {
			// Entry expired, remove it
			bm.mu.Lock()
			bm.removeElement(element)
			bm.mu.Unlock()

			atomic.AddInt64(&bm.timeouts, 1)
			var zero V
			return zero, false
		}
	}

	// Move to front (most recently used)
	bm.mu.Lock()
	bm.lruList.MoveToFront(element)
	bm.mu.Unlock()

	entry := element.Value.(*entry[K, V])
	atomic.AddInt64(&bm.hits, 1)
	return entry.value, true
}

// Set adds or updates a key-value pair in the map
func (bm *BoundedMap[K, V]) Set(key K, value V) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Check if key already exists
	if element, exists := bm.data[key]; exists {
		// Update existing entry
		entry := element.Value.(*entry[K, V])
		entry.value = value
		entry.timestamp = time.Now()
		bm.lruList.MoveToFront(element)
		return
	}

	// Create new entry
	newEntry := &entry[K, V]{
		key:       key,
		value:     value,
		timestamp: time.Now(),
	}

	element := bm.lruList.PushFront(newEntry)
	bm.data[key] = element

	// Evict oldest entries if necessary
	for bm.lruList.Len() > bm.maxSize {
		oldest := bm.lruList.Back()
		if oldest != nil {
			bm.removeElement(oldest)
			atomic.AddInt64(&bm.evictions, 1)
		}
	}
}

// Delete removes a key from the map
func (bm *BoundedMap[K, V]) Delete(key K) bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	element, exists := bm.data[key]
	if !exists {
		return false
	}

	bm.removeElement(element)
	return true
}

// removeElement removes an element from both the map and the list
// Must be called with lock held
func (bm *BoundedMap[K, V]) removeElement(element *list.Element) {
	entry := element.Value.(*entry[K, V])
	delete(bm.data, entry.key)
	bm.lruList.Remove(element)
}

// Len returns the current number of entries in the map
func (bm *BoundedMap[K, V]) Len() int {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return len(bm.data)
}

// Clear removes all entries from the map
func (bm *BoundedMap[K, V]) Clear() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.data = make(map[K]*list.Element)
	bm.lruList = list.New()
}

// Cleanup removes expired entries (if timeouts are enabled)
func (bm *BoundedMap[K, V]) Cleanup() int {
	if !bm.enableTimeouts {
		return 0
	}

	bm.cleanupMu.Lock()
	defer bm.cleanupMu.Unlock()

	// Limit cleanup frequency
	if time.Since(bm.lastCleanup) < time.Minute {
		return 0
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	var toRemove []*list.Element
	now := time.Now()

	// Walk backwards through the list (oldest entries first)
	for element := bm.lruList.Back(); element != nil; element = element.Prev() {
		entry := element.Value.(*entry[K, V])
		if now.Sub(entry.timestamp) > bm.ttl {
			toRemove = append(toRemove, element)
		} else {
			// Since list is ordered by access time, no need to check further
			break
		}
	}

	// Remove expired entries
	for _, element := range toRemove {
		bm.removeElement(element)
		atomic.AddInt64(&bm.timeouts, 1)
	}

	bm.lastCleanup = now
	return len(toRemove)
}

// GetOrSet retrieves a value if it exists, or sets and returns a new value
// This operation is atomic and race-condition safe
func (bm *BoundedMap[K, V]) GetOrSet(key K, factory func() V) V {
	return bm.getOrSetWithRetry(key, factory, 3) // Allow up to 3 retries for eviction scenarios
}

// getOrSetWithRetry implements the atomic GetOrSet operation with retry logic for eviction race conditions
func (bm *BoundedMap[K, V]) getOrSetWithRetry(key K, factory func() V, maxRetries int) V {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Try to get the value atomically
		if value, found := bm.atomicGetOrCreate(key, factory); found {
			return value
		}
		
		// If we reached max retries, perform the operation anyway
		if attempt == maxRetries {
			break
		}
		
		// Track retry attempt
		atomic.AddInt64(&bm.retries, 1)
		
		// Brief backoff before retry to avoid thundering herd
		time.Sleep(time.Microsecond * time.Duration(1<<attempt))
	}
	
	// Fallback: perform the operation one final time
	value, _ := bm.atomicGetOrCreate(key, factory)
	return value
}

// atomicGetOrCreate performs an atomic get-or-create operation
func (bm *BoundedMap[K, V]) atomicGetOrCreate(key K, factory func() V) (V, bool) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	// Double-check if the key exists after acquiring the lock
	if element, exists := bm.data[key]; exists {
		// Check if entry has expired
		if bm.enableTimeouts {
			entry := element.Value.(*entry[K, V])
			if time.Since(entry.timestamp) > bm.ttl {
				// Entry expired, remove it and create new one
				bm.removeElement(element)
				atomic.AddInt64(&bm.timeouts, 1)
				// Continue to create new entry below
			} else {
				// Entry is valid, move to front and return it
				bm.lruList.MoveToFront(element)
				atomic.AddInt64(&bm.hits, 1)
				return entry.value, true
			}
		} else {
			// No timeout check needed, move to front and return
			entry := element.Value.(*entry[K, V])
			bm.lruList.MoveToFront(element)
			atomic.AddInt64(&bm.hits, 1)
			return entry.value, true
		}
	}
	
	// Key doesn't exist or was expired, create new value
	atomic.AddInt64(&bm.misses, 1)
	value := factory()
	
	// Create new entry
	newEntry := &entry[K, V]{
		key:       key,
		value:     value,
		timestamp: time.Now(),
	}
	
	element := bm.lruList.PushFront(newEntry)
	bm.data[key] = element
	
	// Evict oldest entries if necessary
	for bm.lruList.Len() > bm.maxSize {
		oldest := bm.lruList.Back()
		if oldest != nil {
			bm.removeElement(oldest)
			atomic.AddInt64(&bm.evictions, 1)
		}
	}
	
	return value, true
}

// Keys returns a slice of all keys in the map
func (bm *BoundedMap[K, V]) Keys() []K {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	keys := make([]K, 0, len(bm.data))
	for key := range bm.data {
		keys = append(keys, key)
	}
	return keys
}

// Stats returns statistics about the bounded map
func (bm *BoundedMap[K, V]) Stats() BoundedMapStats {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	hits := atomic.LoadInt64(&bm.hits)
	misses := atomic.LoadInt64(&bm.misses)
	evictions := atomic.LoadInt64(&bm.evictions)
	timeouts := atomic.LoadInt64(&bm.timeouts)

	retries := atomic.LoadInt64(&bm.retries)
	
	return BoundedMapStats{
		Size:      len(bm.data),
		MaxSize:   bm.maxSize,
		Hits:      hits,
		Misses:    misses,
		Evictions: evictions,
		Timeouts:  timeouts,
		Retries:   retries,
		HitRate:   float64(hits) / float64(hits+misses+1), // +1 to avoid division by zero
	}
}

// BoundedMapStats contains statistics about the bounded map
type BoundedMapStats struct {
	Size      int     `json:"size"`
	MaxSize   int     `json:"max_size"`
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	Evictions int64   `json:"evictions"`
	Timeouts  int64   `json:"timeouts"`
	Retries   int64   `json:"retries"`
	HitRate   float64 `json:"hit_rate"`
}

// DefaultBoundedMapConfig returns a default configuration for BoundedMap
func DefaultBoundedMapConfig() BoundedMapConfig {
	return BoundedMapConfig{
		MaxSize:        10000,            // Maximum 10,000 entries
		EnableTimeouts: true,             // Enable automatic expiration
		TTL:            30 * time.Minute, // 30-minute TTL
	}
}

// RateLimiterBoundedMapConfig returns a configuration optimized for rate limiters
func RateLimiterBoundedMapConfig() BoundedMapConfig {
	return BoundedMapConfig{
		MaxSize:        50000,            // Allow more entries for rate limiting
		EnableTimeouts: true,             // Enable automatic expiration
		TTL:            10 * time.Minute, // Shorter TTL for rate limiters
	}
}
