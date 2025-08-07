package registry

import (
	"container/list"
	"sync"
	"time"
)

// CacheManager handles LRU caching and memory management with enhanced leak prevention
type CacheManager struct {
	lruMu         sync.RWMutex
	lruList       *list.List
	lruIndex      map[RegistryKey]*list.Element
	config        *RegistryConfig
	lastCleanup   time.Time
	cleanupMu     sync.Mutex
	evictionCount int64 // Track evictions for monitoring
}

// NewCacheManager creates a new cache manager with enhanced leak prevention
func NewCacheManager(config *RegistryConfig) *CacheManager {
	return &CacheManager{
		lruList:     list.New(),
		lruIndex:    make(map[RegistryKey]*list.Element),
		config:      config,
		lastCleanup: time.Now(),
	}
}

// Clear clears all LRU cache entries and prevents memory leaks
func (c *CacheManager) Clear() {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()
	
	// Explicitly remove all elements to help GC
	for c.lruList.Len() > 0 {
		back := c.lruList.Back()
		if back != nil {
			c.lruList.Remove(back)
		}
	}
	
	// Create new structures to ensure clean state
	c.lruList = list.New()
	c.lruIndex = make(map[RegistryKey]*list.Element)
	c.lastCleanup = time.Now()
	c.evictionCount = 0
}

// Size returns the number of items in the LRU cache
func (c *CacheManager) Size() int {
	c.lruMu.RLock()
	defer c.lruMu.RUnlock()
	return c.lruList.Len()
}

// EvictOldest evicts the oldest entry from the LRU cache and returns the key
func (c *CacheManager) EvictOldest() (RegistryKey, bool) {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()

	if c.lruList.Len() == 0 {
		return RegistryKey{}, false
	}

	// Remove the oldest entry (back of the list)
	oldest := c.lruList.Back()
	if oldest != nil {
		key := oldest.Value.(RegistryKey)
		delete(c.lruIndex, key)
		c.lruList.Remove(oldest)
		return key, true
	}
	return RegistryKey{}, false
}

// UpdateLRUPosition moves an entry to the front of the LRU list
func (c *CacheManager) UpdateLRUPosition(key RegistryKey) {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()

	if elem, exists := c.lruIndex[key]; exists {
		c.lruList.MoveToFront(elem)
	}
}

// AddToLRU adds a new entry to the front of the LRU list
func (c *CacheManager) AddToLRU(key RegistryKey) {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()

	elem := c.lruList.PushFront(key)
	c.lruIndex[key] = elem
}

// RemoveFromLRU removes an entry from the LRU list
func (c *CacheManager) RemoveFromLRU(key RegistryKey) {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()

	if elem, exists := c.lruIndex[key]; exists {
		c.lruList.Remove(elem)
		delete(c.lruIndex, key)
	}
}

// GetLRUCandidate returns the least recently used entry for eviction
func (c *CacheManager) GetLRUCandidate() *RegistryKey {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()

	if c.lruList.Len() == 0 {
		return nil
	}

	elem := c.lruList.Back()
	if elem == nil {
		return nil
	}

	key := elem.Value.(RegistryKey)
	return &key
}

// FindOldestFormatEntry walks the LRU list to find the oldest format entry
func (c *CacheManager) FindOldestFormatEntry() *RegistryKey {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()

	// Walk from back (oldest) to front to find the oldest format entry
	for elem := c.lruList.Back(); elem != nil; elem = elem.Prev() {
		key := elem.Value.(RegistryKey)
		if key.EntryType == EntryTypeFormat {
			return &key
		}
	}
	return nil
}

// PerformBatchEviction performs efficient batch eviction to prevent memory accumulation
func (c *CacheManager) PerformBatchEviction(maxEvictions int) []RegistryKey {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()
	
	var evicted []RegistryKey
	for i := 0; i < maxEvictions && c.lruList.Len() > 0; i++ {
		oldest := c.lruList.Back()
		if oldest == nil {
			break
		}
		
		key := oldest.Value.(RegistryKey)
		delete(c.lruIndex, key)
		c.lruList.Remove(oldest)
		evicted = append(evicted, key)
		c.evictionCount++
	}
	
	return evicted
}

// GetEvictionCount returns the total number of evictions performed
func (c *CacheManager) GetEvictionCount() int64 {
	c.lruMu.RLock()
	defer c.lruMu.RUnlock()
	return c.evictionCount
}

// CheckMemoryPressure evaluates if the cache is under memory pressure
func (c *CacheManager) CheckMemoryPressure() bool {
	c.lruMu.RLock()
	defer c.lruMu.RUnlock()
	
	if c.config.MaxEntries <= 0 {
		return false // No limit configured
	}
	
	// Consider pressure if we're at 80% of capacity
	currentSize := c.lruList.Len()
	threshold := (c.config.MaxEntries * 4) / 5 // 80%
	return currentSize >= threshold
}

// PerformPreventativeCleanup runs cleanup to prevent memory accumulation
func (c *CacheManager) PerformPreventativeCleanup() int {
	c.cleanupMu.Lock()
	defer c.cleanupMu.Unlock()
	
	now := time.Now()
	// Only run cleanup if enough time has passed since last cleanup
	if now.Sub(c.lastCleanup) < time.Minute {
		return 0
	}
	
	c.lastCleanup = now
	
	// If we're under memory pressure, perform batch eviction
	if c.CheckMemoryPressure() {
		// Evict 10% of entries to free up space
		c.lruMu.RLock()
		currentSize := c.lruList.Len()
		c.lruMu.RUnlock()
		
		maxEvictions := currentSize / 10
		if maxEvictions < 1 {
			maxEvictions = 1
		}
		if maxEvictions > 50 { // Cap at 50 to prevent too aggressive eviction
			maxEvictions = 50
		}
		
		evicted := c.PerformBatchEviction(maxEvictions)
		return len(evicted)
	}
	
	return 0
}
