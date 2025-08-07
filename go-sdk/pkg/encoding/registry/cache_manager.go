package registry

import (
	"container/list"
	"sync"
)

// CacheManager handles LRU caching and memory management
type CacheManager struct {
	lruMu    sync.RWMutex
	lruList  *list.List
	lruIndex map[RegistryKey]*list.Element
	config   *RegistryConfig
}

// NewCacheManager creates a new cache manager
func NewCacheManager(config *RegistryConfig) *CacheManager {
	return &CacheManager{
		lruList:  list.New(),
		lruIndex: make(map[RegistryKey]*list.Element),
		config:   config,
	}
}

// Clear clears all LRU cache entries
func (c *CacheManager) Clear() {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()
	c.lruList = list.New()
	c.lruIndex = make(map[RegistryKey]*list.Element)
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
