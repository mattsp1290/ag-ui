package state

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// BoundedResolverRegistry manages custom resolvers with size limits to prevent memory leaks
type BoundedResolverRegistry struct {
	resolvers  map[string]CustomResolverFunc
	lru        *list.List
	index      map[string]*list.Element
	maxSize    int
	mu         sync.RWMutex
	accessTime map[string]time.Time
}

// resolverEntry wraps a resolver for LRU tracking
type resolverEntry struct {
	name     string
	resolver CustomResolverFunc
}

// NewBoundedResolverRegistry creates a new bounded resolver registry
func NewBoundedResolverRegistry(maxSize int) *BoundedResolverRegistry {
	if maxSize <= 0 {
		maxSize = 100 // Default size
	}
	return &BoundedResolverRegistry{
		resolvers:  make(map[string]CustomResolverFunc),
		lru:        list.New(),
		index:      make(map[string]*list.Element),
		maxSize:    maxSize,
		accessTime: make(map[string]time.Time),
	}
}

// Register adds a custom resolver with the given name
func (br *BoundedResolverRegistry) Register(name string, resolver CustomResolverFunc) error {
	if name == "" {
		return fmt.Errorf("resolver name cannot be empty")
	}
	if resolver == nil {
		return fmt.Errorf("resolver function cannot be nil")
	}

	br.mu.Lock()
	defer br.mu.Unlock()

	// Check if already exists
	if elem, ok := br.index[name]; ok {
		// Update existing resolver
		br.lru.MoveToFront(elem)
		entry := elem.Value.(*resolverEntry)
		entry.resolver = resolver
		br.resolvers[name] = resolver
		br.accessTime[name] = time.Now()
		return nil
	}

	// Add new resolver
	entry := &resolverEntry{
		name:     name,
		resolver: resolver,
	}
	elem := br.lru.PushFront(entry)
	br.index[name] = elem
	br.resolvers[name] = resolver
	br.accessTime[name] = time.Now()

	// Evict if over capacity
	if br.lru.Len() > br.maxSize {
		oldest := br.lru.Back()
		if oldest != nil {
			br.removeElement(oldest)
		}
	}

	return nil
}

// Get retrieves a resolver by name
func (br *BoundedResolverRegistry) Get(name string) (CustomResolverFunc, bool) {
	br.mu.Lock()
	defer br.mu.Unlock()

	if elem, ok := br.index[name]; ok {
		br.lru.MoveToFront(elem)
		entry := elem.Value.(*resolverEntry)
		br.accessTime[name] = time.Now()
		return entry.resolver, true
	}

	return nil, false
}

// Delete removes a resolver from the registry
func (br *BoundedResolverRegistry) Delete(name string) {
	br.mu.Lock()
	defer br.mu.Unlock()

	if elem, ok := br.index[name]; ok {
		br.removeElement(elem)
	}
}

// Size returns the current number of resolvers
func (br *BoundedResolverRegistry) Size() int {
	br.mu.RLock()
	defer br.mu.RUnlock()
	return len(br.resolvers)
}

// Clear removes all resolvers
func (br *BoundedResolverRegistry) Clear() {
	br.mu.Lock()
	defer br.mu.Unlock()

	br.resolvers = make(map[string]CustomResolverFunc)
	br.lru = list.New()
	br.index = make(map[string]*list.Element)
	br.accessTime = make(map[string]time.Time)
}

// GetNames returns all resolver names in LRU order (most recently used first)
func (br *BoundedResolverRegistry) GetNames() []string {
	br.mu.RLock()
	defer br.mu.RUnlock()

	names := make([]string, 0, br.lru.Len())
	for elem := br.lru.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*resolverEntry)
		names = append(names, entry.name)
	}

	return names
}

// removeElement removes an element from all internal structures
func (br *BoundedResolverRegistry) removeElement(elem *list.Element) {
	br.lru.Remove(elem)
	entry := elem.Value.(*resolverEntry)
	delete(br.index, entry.name)
	delete(br.resolvers, entry.name)
	delete(br.accessTime, entry.name)
}

// CleanupOldResolvers removes resolvers that haven't been accessed within the given duration
func (br *BoundedResolverRegistry) CleanupOldResolvers(maxAge time.Duration) int {
	br.mu.Lock()
	defer br.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	count := 0

	// Collect resolvers to remove
	var toRemove []*list.Element
	for elem := br.lru.Back(); elem != nil; elem = elem.Prev() {
		entry := elem.Value.(*resolverEntry)
		if accessTime, ok := br.accessTime[entry.name]; ok {
			if accessTime.Before(cutoff) {
				toRemove = append(toRemove, elem)
			} else {
				// Since we're iterating from back (oldest), we can stop when we find a non-expired entry
				break
			}
		}
	}

	// Remove collected resolvers
	for _, elem := range toRemove {
		br.removeElement(elem)
		count++
	}

	return count
}

// GetStatistics returns statistics about the resolver registry
func (br *BoundedResolverRegistry) GetStatistics() map[string]interface{} {
	br.mu.RLock()
	defer br.mu.RUnlock()

	stats := map[string]interface{}{
		"current_size": len(br.resolvers),
		"max_size":     br.maxSize,
		"capacity":     float64(len(br.resolvers)) / float64(br.maxSize),
	}

	// Find oldest access time
	var oldestAccess time.Time
	for _, t := range br.accessTime {
		if oldestAccess.IsZero() || t.Before(oldestAccess) {
			oldestAccess = t
		}
	}

	if !oldestAccess.IsZero() {
		stats["oldest_access"] = oldestAccess
		stats["oldest_age"] = time.Since(oldestAccess)
	}

	return stats
}
