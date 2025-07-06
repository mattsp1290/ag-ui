package state

import (
	"container/list"
	"sync"
	"time"
)

// ContextManager manages state contexts with LRU eviction to prevent memory leaks
type ContextManager struct {
	contexts map[string]*StateContext
	lru      *list.List
	index    map[string]*list.Element
	maxSize  int
	mu       sync.RWMutex
}

// contextEntry wraps StateContext for LRU tracking
type contextEntry struct {
	id  string
	ctx *StateContext
}

// NewContextManager creates a new bounded context manager
func NewContextManager(maxSize int) *ContextManager {
	if maxSize <= 0 {
		maxSize = 1000 // Default size
	}
	return &ContextManager{
		contexts: make(map[string]*StateContext),
		lru:      list.New(),
		index:    make(map[string]*list.Element),
		maxSize:  maxSize,
	}
}

// Get retrieves a context by ID and updates its access time
func (cm *ContextManager) Get(id string) (*StateContext, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if elem, ok := cm.index[id]; ok {
		cm.lru.MoveToFront(elem)
		entry := elem.Value.(*contextEntry)
		ctx := entry.ctx
		
		// Update last accessed time
		ctx.mu.Lock()
		ctx.LastAccessed = time.Now()
		ctx.mu.Unlock()
		
		return ctx, true
	}
	
	return nil, false
}

// Put adds or updates a context in the manager
func (cm *ContextManager) Put(id string, ctx *StateContext) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// Check if already exists
	if elem, ok := cm.index[id]; ok {
		cm.lru.MoveToFront(elem)
		entry := elem.Value.(*contextEntry)
		entry.ctx = ctx
		cm.contexts[id] = ctx
		return
	}
	
	// Add new entry
	entry := &contextEntry{
		id:  id,
		ctx: ctx,
	}
	elem := cm.lru.PushFront(entry)
	cm.index[id] = elem
	cm.contexts[id] = ctx
	
	// Evict if over capacity
	if cm.lru.Len() > cm.maxSize {
		oldest := cm.lru.Back()
		if oldest != nil {
			cm.removeElement(oldest)
		}
	}
}

// Delete removes a context from the manager
func (cm *ContextManager) Delete(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if elem, ok := cm.index[id]; ok {
		cm.removeElement(elem)
	}
}

// Range iterates over all contexts (for compatibility with sync.Map interface)
func (cm *ContextManager) Range(f func(key, value interface{}) bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	// Create a snapshot to avoid holding the lock during iteration
	snapshot := make(map[string]*StateContext, len(cm.contexts))
	for k, v := range cm.contexts {
		snapshot[k] = v
	}
	
	// Iterate over snapshot
	for k, v := range snapshot {
		if !f(k, v) {
			break
		}
	}
}

// Size returns the current number of contexts
func (cm *ContextManager) Size() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.contexts)
}

// Clear removes all contexts
func (cm *ContextManager) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.contexts = make(map[string]*StateContext)
	cm.lru = list.New()
	cm.index = make(map[string]*list.Element)
}

// removeElement removes an element from all internal structures
func (cm *ContextManager) removeElement(elem *list.Element) {
	cm.lru.Remove(elem)
	entry := elem.Value.(*contextEntry)
	delete(cm.index, entry.id)
	delete(cm.contexts, entry.id)
}

// GetExpiredContexts returns contexts that haven't been accessed within the TTL
func (cm *ContextManager) GetExpiredContexts(ttl time.Duration) []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	var expired []string
	cutoff := time.Now().Add(-ttl)
	
	for id, ctx := range cm.contexts {
		ctx.mu.RLock()
		lastAccessed := ctx.LastAccessed
		ctx.mu.RUnlock()
		
		if lastAccessed.Before(cutoff) {
			expired = append(expired, id)
		}
	}
	
	return expired
}

// CleanupExpired removes contexts that haven't been accessed within the TTL
func (cm *ContextManager) CleanupExpired(ttl time.Duration) int {
	expired := cm.GetExpiredContexts(ttl)
	
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	count := 0
	for _, id := range expired {
		if elem, ok := cm.index[id]; ok {
			cm.removeElement(elem)
			count++
		}
	}
	
	return count
}