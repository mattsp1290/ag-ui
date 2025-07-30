package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/hashicorp/golang-lru/v2"
)

// CacheValidatorSimple provides multi-level caching for validation results
// This is a simplified version for demonstration of the interface refactoring
type CacheValidatorSimple struct {
	// L1 Cache (in-memory)
	l1Cache       *lru.Cache[string, *ValidationCacheEntry]
	l1Size        int
	l1TTL         time.Duration
	
	// L2 Cache (distributed) - now using the interface hierarchy
	l2Cache       DistributedCache // Uses the new interface hierarchy
	l2TTL         time.Duration
	l2Enabled     bool
	
	// Validation
	validator     *events.Validator
	
	// Coordination
	coordinator   *EventDrivenCoordinator // Uses event-driven coordinator
	nodeID        string
	
	// Metrics
	stats         CacheStats
	metricsEnabled bool
	
	// Synchronization
	mu            sync.RWMutex
	shutdownCh    chan struct{}
	wg            sync.WaitGroup
}

// NewCacheValidatorSimple creates a new simplified cache validator
func NewCacheValidatorSimple(l2Cache DistributedCache, nodeID string, eventBus events.EventBus) (*CacheValidatorSimple, error) {
	// Create L1 cache
	l1Cache, err := lru.New[string, *ValidationCacheEntry](10000)
	if err != nil {
		return nil, fmt.Errorf("failed to create L1 cache: %w", err)
	}
	
	// Create event-driven coordinator
	coordinator := NewEventDrivenCoordinator(nodeID, eventBus, DefaultEventDrivenConfig())
	
	cv := &CacheValidatorSimple{
		l1Cache:        l1Cache,
		l1Size:         10000,
		l1TTL:          5 * time.Minute,
		l2Cache:        l2Cache,
		l2TTL:          30 * time.Minute,
		l2Enabled:      l2Cache != nil,
		validator:      events.NewValidator(events.DefaultValidationConfig()),
		coordinator:    coordinator,
		nodeID:         nodeID,
		metricsEnabled: true,
		shutdownCh:     make(chan struct{}),
	}
	
	return cv, nil
}

// ValidateEvent validates an event with caching using the new interface hierarchy
func (cv *CacheValidatorSimple) ValidateEvent(ctx context.Context, event events.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}
	
	// Generate cache key
	key, err := cv.generateCacheKey(event)
	if err != nil {
		// Fallback to direct validation
		return cv.validator.ValidateEvent(ctx, event)
	}
	
	// Check L1 cache (BasicCache interface)
	if entry, ok := cv.getFromL1(key); ok {
		cv.recordHit("L1")
		return cv.toValidationError(entry)
	}
	
	// Check L2 cache if enabled (DistributedCache interface)
	if cv.l2Enabled {
		if entry, ok := cv.getFromL2(ctx, key); ok {
			cv.recordHit("L2")
			cv.promoteToL1(key, entry)
			return cv.toValidationError(entry)
		} else {
			// L2 miss
			atomic.AddUint64(&cv.stats.L2Misses, 1)
		}
	}
	
	// Cache miss - perform validation
	cv.recordMiss()
	startTime := time.Now()
	
	err = cv.validator.ValidateEvent(ctx, event)
	
	validationTime := time.Since(startTime)
	
	// Create cache entry
	metadata := make(map[string]interface{})
	metadata["validation_time"] = validationTime
	metadata["event_size"] = cv.estimateEventSize(event)
	
	entry := &ValidationCacheEntry{
		Key:            *key,
		Valid:          err == nil,
		Errors:         cv.extractErrors(err),
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(cv.l1TTL),
		AccessCount:    1,
		LastAccessedAt: time.Now(),
		Metadata:       metadata,
	}
	
	// Store in caches using interface hierarchy
	cv.storeInCaches(ctx, key, entry)
	
	return err
}

// InvalidateEvent invalidates cache entries and publishes event
func (cv *CacheValidatorSimple) InvalidateEvent(ctx context.Context, event events.Event) error {
	key, err := cv.generateCacheKey(event)
	if err != nil {
		return fmt.Errorf("failed to generate cache key: %w", err)
	}
	
	err = cv.invalidateKey(ctx, key)
	if err != nil {
		return err
	}
	
	// Use event-driven coordinator for invalidation
	if cv.coordinator != nil {
		return cv.coordinator.InvalidateCache(ctx, string(event.Type()), cv.cacheKeyToString(key))
	}
	
	return nil
}

// GetStats returns current cache statistics
func (cv *CacheValidatorSimple) GetStats() CacheStats {
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	
	stats := cv.stats
	stats.TotalHits = stats.L1Hits + stats.L2Hits
	stats.TotalMisses = cv.stats.L1Misses
	
	return stats
}

// InvalidateByKeys invalidates cache entries for specific keys (implements CacheValidatorInterface)
func (cv *CacheValidatorSimple) InvalidateByKeys(ctx context.Context, keys []string) error {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	
	for _, keyStr := range keys {
		// Remove from L1 cache
		cv.l1Cache.Remove(keyStr)
		
		// Remove from L2 cache if enabled
		if cv.l2Enabled {
			cv.l2Cache.Delete(ctx, keyStr)
		}
	}
	
	return nil
}

// InvalidateEventType invalidates all cache entries for a specific event type (implements CacheValidatorInterface)
func (cv *CacheValidatorSimple) InvalidateEventType(ctx context.Context, eventType string) error {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	
	// Invalidate L1 entries
	keys := cv.l1Cache.Keys()
	for _, key := range keys {
		if entry, ok := cv.l1Cache.Peek(key); ok && entry != nil {
			if entry.Key.EventType == events.EventType(eventType) {
				cv.l1Cache.Remove(key)
			}
		}
	}
	
	// Invalidate L2 entries if enabled
	if cv.l2Enabled {
		pattern := fmt.Sprintf("validation:%s:*", eventType)
		keys, err := cv.l2Cache.Scan(ctx, pattern)
		if err != nil {
			return fmt.Errorf("failed to scan L2 cache: %w", err)
		}
		
		for _, key := range keys {
			if err := cv.l2Cache.Delete(ctx, key); err != nil {
				// Log error but continue
				continue
			}
		}
	}
	
	return nil
}

// Private helper methods (simplified)

func (cv *CacheValidatorSimple) generateCacheKey(event events.Event) (*ValidationCacheKey, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}
	
	hash := sha256.Sum256(data)
	
	return &ValidationCacheKey{
		EventType:   event.Type(),
		EventHash:   hex.EncodeToString(hash[:]),
		ConfigHash:  "simplified",
		ValidatorID: "",
	}, nil
}

func (cv *CacheValidatorSimple) getFromL1(key *ValidationCacheKey) (*ValidationCacheEntry, bool) {
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	
	keyStr := cv.cacheKeyToString(key)
	entry, ok := cv.l1Cache.Get(keyStr)
	if !ok {
		return nil, false
	}
	
	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		cv.l1Cache.Remove(keyStr)
		atomic.AddUint64(&cv.stats.Expirations, 1)
		return nil, false
	}
	
	// Update access stats
	atomic.AddUint64(&entry.AccessCount, 1)
	entry.LastAccessedAt = time.Now()
	
	return entry, true
}

func (cv *CacheValidatorSimple) getFromL2(ctx context.Context, key *ValidationCacheKey) (*ValidationCacheEntry, bool) {
	if !cv.l2Enabled {
		return nil, false
	}
	
	keyStr := cv.cacheKeyToString(key)
	data, err := cv.l2Cache.Get(ctx, keyStr)
	if err != nil {
		return nil, false
	}
	
	// Deserialize
	var entry ValidationCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}
	
	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		cv.l2Cache.Delete(ctx, keyStr)
		atomic.AddUint64(&cv.stats.Expirations, 1)
		return nil, false
	}
	
	return &entry, true
}

func (cv *CacheValidatorSimple) storeInCaches(ctx context.Context, key *ValidationCacheKey, entry *ValidationCacheEntry) {
	keyStr := cv.cacheKeyToString(key)
	
	// Store in L1 (BasicCache interface)
	cv.mu.Lock()
	cv.l1Cache.Add(keyStr, entry)
	cv.mu.Unlock()
	
	// Store in L2 if enabled (DistributedCache interface)
	if cv.l2Enabled {
		go cv.storeInL2(ctx, keyStr, entry)
	}
}

func (cv *CacheValidatorSimple) storeInL2(ctx context.Context, key string, entry *ValidationCacheEntry) {
	// Serialize
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	
	// Store with TTL using DistributedCache interface
	cv.l2Cache.Set(ctx, key, data, cv.l2TTL)
}

func (cv *CacheValidatorSimple) promoteToL1(key *ValidationCacheKey, entry *ValidationCacheEntry) {
	keyStr := cv.cacheKeyToString(key)
	cv.mu.Lock()
	cv.l1Cache.Add(keyStr, entry)
	cv.mu.Unlock()
}

func (cv *CacheValidatorSimple) invalidateKey(ctx context.Context, key *ValidationCacheKey) error {
	keyStr := cv.cacheKeyToString(key)
	
	// Remove from L1
	cv.mu.Lock()
	cv.l1Cache.Remove(keyStr)
	cv.mu.Unlock()
	
	// Remove from L2 if enabled
	if cv.l2Enabled {
		if err := cv.l2Cache.Delete(ctx, keyStr); err != nil {
			return fmt.Errorf("failed to delete from L2 cache: %w", err)
		}
	}
	
	return nil
}

func (cv *CacheValidatorSimple) cacheKeyToString(key *ValidationCacheKey) string {
	return fmt.Sprintf("validation:%s:%s:%s:%s", 
		key.EventType, 
		key.EventHash[:8], 
		key.ConfigHash[:8],
		key.ValidatorID)
}

func (cv *CacheValidatorSimple) toValidationError(entry *ValidationCacheEntry) error {
	if entry == nil {
		return fmt.Errorf("nil cache entry")
	}
	
	if entry.Valid {
		return nil
	}
	
	if len(entry.Errors) == 0 {
		return fmt.Errorf("validation failed")
	}
	
	if len(entry.Errors) == 1 {
		return entry.Errors[0]
	}
	
	// Combine multiple errors
	errStr := "validation failed with multiple errors:"
	for _, err := range entry.Errors {
		if err != nil {
			errStr += fmt.Sprintf("\n  - %v", err)
		}
	}
	return fmt.Errorf("%s", errStr)
}

func (cv *CacheValidatorSimple) extractErrors(err error) []error {
	if err == nil {
		return nil
	}
	return []error{err}
}

func (cv *CacheValidatorSimple) recordHit(level string) {
	if !cv.metricsEnabled {
		return
	}
	
	switch level {
	case "L1":
		atomic.AddUint64(&cv.stats.L1Hits, 1)
	case "L2":
		atomic.AddUint64(&cv.stats.L2Hits, 1)
	}
}

func (cv *CacheValidatorSimple) recordMiss() {
	if !cv.metricsEnabled {
		return
	}
	
	atomic.AddUint64(&cv.stats.L1Misses, 1)
}

func (cv *CacheValidatorSimple) estimateEventSize(event events.Event) int {
	data, err := json.Marshal(event)
	if err != nil {
		return 0
	}
	return len(data)
}