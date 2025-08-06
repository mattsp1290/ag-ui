package server

import (
	"container/list"
	"sync"
	"time"
)

// BoundedMap provides a thread-safe map with LRU eviction to prevent memory exhaustion
type BoundedMap[K comparable, V any] struct {
	mu         sync.RWMutex
	data       map[K]*list.Element
	lruList    *list.List
	maxSize    int
	
	// Optional cleanup configuration
	enableTimeouts bool
	ttl            time.Duration
	lastCleanup    time.Time
	cleanupMu      sync.Mutex
	
	// Metrics
	hits         int64
	misses       int64
	evictions    int64
	timeouts     int64
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
		bm.misses++
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
			
			bm.timeouts++
			var zero V
			return zero, false
		}
	}

	// Move to front (most recently used)
	bm.mu.Lock()
	bm.lruList.MoveToFront(element)
	bm.mu.Unlock()

	entry := element.Value.(*entry[K, V])
	bm.hits++
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
			bm.evictions++
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
		bm.timeouts++
	}

	bm.lastCleanup = now
	return len(toRemove)
}

// GetOrSet retrieves a value if it exists, or sets and returns a new value
func (bm *BoundedMap[K, V]) GetOrSet(key K, factory func() V) V {
	// First try to get existing value
	if value, exists := bm.Get(key); exists {
		return value
	}

	// Create new value and set it
	value := factory()
	bm.Set(key, value)
	return value
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

	return BoundedMapStats{
		Size:       len(bm.data),
		MaxSize:    bm.maxSize,
		Hits:       bm.hits,
		Misses:     bm.misses,
		Evictions:  bm.evictions,
		Timeouts:   bm.timeouts,
		HitRate:    float64(bm.hits) / float64(bm.hits+bm.misses+1), // +1 to avoid division by zero
	}
}

// BoundedMapStats contains statistics about the bounded map
type BoundedMapStats struct {
	Size       int     `json:"size"`
	MaxSize    int     `json:"max_size"`
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	Evictions  int64   `json:"evictions"`
	Timeouts   int64   `json:"timeouts"`
	HitRate    float64 `json:"hit_rate"`
}

// DefaultBoundedMapConfig returns a default configuration for BoundedMap
func DefaultBoundedMapConfig() BoundedMapConfig {
	return BoundedMapConfig{
		MaxSize:        10000,             // Maximum 10,000 entries
		EnableTimeouts: true,              // Enable automatic expiration
		TTL:            30 * time.Minute,  // 30-minute TTL
	}
}

// RateLimiterBoundedMapConfig returns a configuration optimized for rate limiters
func RateLimiterBoundedMapConfig() BoundedMapConfig {
	return BoundedMapConfig{
		MaxSize:        50000,             // Allow more entries for rate limiting
		EnableTimeouts: true,              // Enable automatic expiration
		TTL:            10 * time.Minute,  // Shorter TTL for rate limiters
	}
}