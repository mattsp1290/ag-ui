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
	eventerrors "github.com/ag-ui/go-sdk/pkg/core/events/errors"
	"github.com/hashicorp/golang-lru/v2"
)

// CacheLevel represents the cache hierarchy level
type CacheLevel int

const (
	// L1Cache is the in-memory cache level
	L1Cache CacheLevel = iota
	// L2Cache is the distributed cache level
	L2Cache
)

// ValidationCacheKey represents a cache key for validation results
type ValidationCacheKey struct {
	EventType   events.EventType
	EventHash   string
	ConfigHash  string
	ValidatorID string
}

// ValidationCacheEntry represents a cached validation result
type ValidationCacheEntry struct {
	Key              ValidationCacheKey
	Valid            bool
	Errors           []error
	Metadata         map[string]interface{}
	CreatedAt        time.Time
	ExpiresAt        time.Time
	AccessCount      uint64
	LastAccessedAt   time.Time
	CompressionRatio float64
}

// CacheStats provides detailed cache statistics
type CacheStats struct {
	L1Hits          uint64
	L1Misses        uint64
	L2Hits          uint64
	L2Misses        uint64
	TotalHits       uint64
	TotalMisses     uint64
	Evictions       uint64
	Expirations     uint64
	CompressionRate float64
	AvgHitLatency   time.Duration
	AvgMissLatency  time.Duration
}

// Note: DistributedCache interface is now defined in interfaces.go

// InvalidationStrategy defines cache invalidation strategies
type InvalidationStrategy interface {
	ShouldInvalidate(entry *ValidationCacheEntry) bool
	OnInvalidate(key ValidationCacheKey)
}

// CacheValidator provides multi-level caching for validation results
type CacheValidator struct {
	// L1 Cache (in-memory)
	l1Cache       *lru.Cache[string, *ValidationCacheEntry]
	l1Size        int
	l1TTL         time.Duration
	
	// L2 Cache (distributed)
	l2Cache       DistributedCache
	l2TTL         time.Duration
	l2Enabled     bool
	
	// Validation
	validator     *events.Validator
	
	// Compression
	compressionEnabled bool
	compressionLevel   int
	
	// Invalidation
	invalidationStrategies []InvalidationStrategy
	
	// Coordination
	coordinator   *CacheCoordinator
	nodeID        string
	
	// Metrics
	stats         CacheStats
	metricsEnabled bool
	
	// Error handling
	logger      eventerrors.Logger
	retryPolicy *eventerrors.RetryPolicy
	
	// Synchronization
	mu            sync.RWMutex
	shutdownCh    chan struct{}
	wg            sync.WaitGroup
}

// CacheValidatorConfig contains configuration for the cache validator
type CacheValidatorConfig struct {
	// L1 Cache settings
	L1Size        int
	L1TTL         time.Duration
	
	// L2 Cache settings
	L2Cache       DistributedCache
	L2TTL         time.Duration
	L2Enabled     bool
	
	// Validation settings
	Validator     *events.Validator
	
	// Compression settings
	CompressionEnabled bool
	CompressionLevel   int
	
	// Invalidation strategies
	InvalidationStrategies []InvalidationStrategy
	
	// Coordination settings
	NodeID        string
	Coordinator   *CacheCoordinator
	
	// Error handling settings
	Logger      interface{} // Simplified for refactoring demo
	RetryPolicy interface{} // Simplified for refactoring demo
	
	// Metrics
	MetricsEnabled bool
}

// DefaultCacheValidatorConfig returns a default configuration
func DefaultCacheValidatorConfig() *CacheValidatorConfig {
	return &CacheValidatorConfig{
		L1Size:             10000,
		L1TTL:              5 * time.Minute,
		L2TTL:              30 * time.Minute,
		L2Enabled:          false,
		CompressionEnabled: true,
		CompressionLevel:   6,
		Validator:          events.NewValidator(events.DefaultValidationConfig()),
		Logger:             nil, // Simplified for refactoring demo
		RetryPolicy:        nil, // Simplified for refactoring demo
		MetricsEnabled:     true,
	}
}

// NewCacheValidator creates a new cache validator
func NewCacheValidator(config *CacheValidatorConfig) (*CacheValidator, error) {
	if config == nil {
		config = DefaultCacheValidatorConfig()
	}
	
	// We'll create the L1 cache with eviction callback after creating the validator
	
	// Initialize with safe defaults
	invalidationStrategies := config.InvalidationStrategies
	if invalidationStrategies == nil {
		invalidationStrategies = []InvalidationStrategy{}
	}
	
	// Set default logger if not provided
	var logger eventerrors.Logger
	if config.Logger != nil {
		if l, ok := config.Logger.(eventerrors.Logger); ok {
			logger = l
		} else {
			logger = eventerrors.NewDefaultLogger("cache")
		}
	} else {
		logger = eventerrors.NewDefaultLogger("cache")
	}
	
	// Set default retry policy if not provided  
	var retryPolicy *eventerrors.RetryPolicy
	if config.RetryPolicy != nil {
		if rp, ok := config.RetryPolicy.(*eventerrors.RetryPolicy); ok {
			retryPolicy = rp
		} else {
			retryPolicy = eventerrors.DefaultRetryPolicy()
		}
	} else {
		retryPolicy = eventerrors.DefaultRetryPolicy()
	}

	cv := &CacheValidator{
		l1Size:                 config.L1Size,
		l1TTL:                  config.L1TTL,
		l2Cache:                config.L2Cache,
		l2TTL:                  config.L2TTL,
		l2Enabled:              config.L2Enabled && config.L2Cache != nil,
		validator:              config.Validator,
		compressionEnabled:     config.CompressionEnabled,
		compressionLevel:       config.CompressionLevel,
		invalidationStrategies: invalidationStrategies,
		coordinator:            config.Coordinator,
		nodeID:                 config.NodeID,
		logger:                 logger,
		retryPolicy:            retryPolicy,
		metricsEnabled:         config.MetricsEnabled,
		shutdownCh:             make(chan struct{}),
	}
	
	// Now create the real L1 cache with eviction callback
	l1CacheWithEvict, err := lru.NewWithEvict[string, *ValidationCacheEntry](config.L1Size, func(key string, value *ValidationCacheEntry) {
		// Track evictions when LRU automatically removes entries
		atomic.AddUint64(&cv.stats.Evictions, 1)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create L1 cache with eviction: %w", err)
	}
	cv.l1Cache = l1CacheWithEvict
	
	// Start background workers
	cv.wg.Add(2)
	go cv.expirationWorker()
	go cv.metricsWorker()
	
	return cv, nil
}

// ValidateEvent validates an event with caching
func (cv *CacheValidator) ValidateEvent(ctx context.Context, event events.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}
	
	// Generate cache key
	key, err := cv.generateCacheKey(event)
	if err != nil {
		// Fallback to direct validation
		return cv.validator.ValidateEvent(ctx, event)
	}
	
	// Check L1 cache
	if entry, ok := cv.getFromL1(key); ok {
		cv.recordHit(L1Cache)
		return cv.toValidationError(entry)
	}
	
	// Check L2 cache if enabled
	if cv.l2Enabled {
		if entry, ok := cv.getFromL2(ctx, key); ok {
			cv.recordHit(L2Cache)
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
	
	// Create cache entry with proper metadata initialization
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
	
	// Store in caches
	cv.storeInCaches(ctx, key, entry)
	
	return err
}

// ValidateSequence validates a sequence of events with caching
func (cv *CacheValidator) ValidateSequence(ctx context.Context, events []events.Event) error {
	if len(events) == 0 {
		return nil
	}
	
	// For sequences, we validate individual events with caching
	// but sequence validation itself is not cached
	for i, event := range events {
		if err := cv.ValidateEvent(ctx, event); err != nil {
			return fmt.Errorf("event %d validation failed: %w", i, err)
		}
	}
	
	// Perform sequence-specific validation without caching
	return cv.validator.ValidateSequence(ctx, events)
}

// InvalidateEvent invalidates cache entries for a specific event
func (cv *CacheValidator) InvalidateEvent(ctx context.Context, event events.Event) error {
	key, err := cv.generateCacheKey(event)
	if err != nil {
		return fmt.Errorf("failed to generate cache key: %w", err)
	}
	
	err = cv.invalidateKey(ctx, key)
	if err != nil {
		return err
	}
	
	// Broadcast invalidation if coordinator is available
	if cv.coordinator != nil {
		cv.coordinator.BroadcastInvalidation(ctx, InvalidationMessage{
			NodeID:    cv.nodeID,
			Keys:      []string{cv.cacheKeyToString(key)},
			EventType: string(event.Type()),
			Timestamp: time.Now(),
		})
	}
	
	return nil
}

// InvalidateByKeys invalidates cache entries for specific keys (used by coordinator)
func (cv *CacheValidator) InvalidateByKeys(ctx context.Context, keys []string) error {
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
	
	// Don't increment evictions here - the LRU eviction callback already does it
	// atomic.AddUint64(&cv.stats.Evictions, uint64(len(keys)))
	return nil
}

// InvalidateEventType invalidates all cache entries for a specific event type (string version for interface)
func (cv *CacheValidator) InvalidateEventType(ctx context.Context, eventType string) error {
	// This method is called by the coordinator, so we should not broadcast
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
				// Log the L2 cache deletion error with structured logging
				cacheErr := eventerrors.NewCacheError(eventerrors.CacheErrorEvictionFailed, 
					"Failed to delete key from L2 cache during event type invalidation").
					WithLevel("L2").
					WithKey(key).
					WithOperation("delete").
					WithCause(err)
				
				cv.logger.Warn(fmt.Sprintf("L2 cache deletion failed: %v", cacheErr))
				
				// Continue with other keys even if one fails
				continue
			}
		}
	}
	
	// Don't increment evictions here - the LRU eviction callback already does it
	// atomic.AddUint64(&cv.stats.Evictions, uint64(invalidatedCount))
	
	// Notify invalidation strategies
	for _, strategy := range cv.invalidationStrategies {
		strategy.OnInvalidate(ValidationCacheKey{EventType: events.EventType(eventType)})
	}
	
	// NOTE: We don't broadcast here because this method is called by the coordinator
	return nil
}

// InvalidateEventTypeInternal invalidates all cache entries for a specific event type
func (cv *CacheValidator) InvalidateEventTypeInternal(ctx context.Context, eventType events.EventType) error {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	
	// Invalidate L1 entries
	keys := cv.l1Cache.Keys()
	for _, key := range keys {
		if entry, ok := cv.l1Cache.Peek(key); ok && entry != nil {
			if entry.Key.EventType == eventType {
				cv.l1Cache.Remove(key)
				// Don't increment evictions here - the LRU eviction callback already does it
				// atomic.AddUint64(&cv.stats.Evictions, 1)
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
				// Log the L2 cache deletion error with structured logging
				cacheErr := eventerrors.NewCacheError(eventerrors.CacheErrorEvictionFailed, 
					"Failed to delete key from L2 cache during internal event type invalidation").
					WithLevel("L2").
					WithKey(key).
					WithOperation("delete").
					WithCause(err)
				
				cv.logger.Warn(fmt.Sprintf("L2 cache deletion failed: %v", cacheErr))
				
				// Continue with other keys even if one fails
				continue
			}
		}
	}
	
	// Notify invalidation strategies
	for _, strategy := range cv.invalidationStrategies {
		strategy.OnInvalidate(ValidationCacheKey{EventType: eventType})
	}
	
	// Broadcast invalidation if coordinator is available
	if cv.coordinator != nil {
		cv.coordinator.BroadcastInvalidation(ctx, InvalidationMessage{
			NodeID:    cv.nodeID,
			EventType: string(eventType),
			Timestamp: time.Now(),
		})
	}
	
	return nil
}

// Warmup pre-populates the cache with validation results
func (cv *CacheValidator) Warmup(ctx context.Context, events []events.Event) error {
	for _, event := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Validate event to populate cache
			_ = cv.ValidateEvent(ctx, event)
		}
	}
	return nil
}

// GetStats returns current cache statistics
func (cv *CacheValidator) GetStats() CacheStats {
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	
	stats := cv.stats
	stats.TotalHits = stats.L1Hits + stats.L2Hits
	stats.TotalMisses = cv.stats.L1Misses
	
	if stats.TotalHits > 0 {
		hitRate := float64(stats.TotalHits) / float64(stats.TotalHits + stats.TotalMisses)
		stats.CompressionRate = hitRate
	}
	
	return stats
}

// Shutdown gracefully shuts down the cache validator
func (cv *CacheValidator) Shutdown(ctx context.Context) error {
	close(cv.shutdownCh)
	
	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		cv.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Private methods

func (cv *CacheValidator) generateCacheKey(event events.Event) (*ValidationCacheKey, error) {
	// Serialize event for hashing
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Generate hash
	hash := sha256.Sum256(data)
	
	// Get validator config hash
	configData, err := json.Marshal(cv.validator)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal validator config: %w", err)
	}
	configHash := sha256.Sum256(configData)
	
	return &ValidationCacheKey{
		EventType:   event.Type(),
		EventHash:   hex.EncodeToString(hash[:]),
		ConfigHash:  hex.EncodeToString(configHash[:]),
		ValidatorID: "", // Empty for shared L2 cache across nodes
	}, nil
}

func (cv *CacheValidator) getFromL1(key *ValidationCacheKey) (*ValidationCacheEntry, bool) {
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
	
	// Update access stats and refresh TTL
	atomic.AddUint64(&entry.AccessCount, 1)
	entry.LastAccessedAt = time.Now()
	entry.ExpiresAt = time.Now().Add(cv.l1TTL) // Refresh TTL on access
	
	return entry, true
}

func (cv *CacheValidator) getFromL2(ctx context.Context, key *ValidationCacheKey) (*ValidationCacheEntry, bool) {
	if !cv.l2Enabled {
		return nil, false
	}
	
	keyStr := cv.cacheKeyToString(key)
	data, err := cv.l2Cache.Get(ctx, keyStr)
	if err != nil {
		// Log L2 cache get error but don't fail the operation
		{
			getErr := eventerrors.NewCacheError(eventerrors.CacheErrorKeyNotFound, 
				"Failed to retrieve entry from L2 cache").
				WithLevel("L2").
				WithKey(keyStr).
				WithOperation("get").
				WithCause(err)
			cv.logger.Warn(fmt.Sprintf("L2 cache get failed: %v", getErr))
		}
		return nil, false
	}
	
	// Decompress if needed
	if cv.compressionEnabled {
		data, err = cv.decompress(data)
		if err != nil {
			// Log decompression error
			{
				decompErr := eventerrors.NewCacheError(eventerrors.CacheErrorCompressionFailed, 
					"Failed to decompress entry from L2 cache").
					WithLevel("L2").
					WithKey(keyStr).
					WithOperation("decompress").
					WithCause(err)
				cv.logger.Error(fmt.Sprintf("L2 cache decompression failed: %v", decompErr))
			}
			return nil, false
		}
	}
	
	// Deserialize
	var entry ValidationCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		// Log deserialization error
		{
			deserErr := eventerrors.NewCacheError(eventerrors.CacheErrorSerializationFailed, 
				"Failed to deserialize entry from L2 cache").
				WithLevel("L2").
				WithKey(keyStr).
				WithOperation("deserialize").
				WithCause(err)
			cv.logger.Error(fmt.Sprintf("L2 cache deserialization failed: %v", deserErr))
		}
		return nil, false
	}
	
	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		// Clean up expired entry
		if delErr := cv.l2Cache.Delete(ctx, keyStr); delErr != nil {
			// Log deletion error but don't fail
			{
				delCacheErr := eventerrors.NewCacheError(eventerrors.CacheErrorEvictionFailed, 
					"Failed to delete expired entry from L2 cache").
					WithLevel("L2").
					WithKey(keyStr).
					WithOperation("delete").
					WithCause(delErr)
				cv.logger.Warn(fmt.Sprintf("L2 cache expired entry deletion failed: %v", delCacheErr))
			}
		}
		atomic.AddUint64(&cv.stats.Expirations, 1)
		return nil, false
	}
	
	return &entry, true
}

func (cv *CacheValidator) storeInCaches(ctx context.Context, key *ValidationCacheKey, entry *ValidationCacheEntry) {
	keyStr := cv.cacheKeyToString(key)
	
	// Store in L1
	cv.mu.Lock()
	cv.l1Cache.Add(keyStr, entry)
	cv.mu.Unlock()
	
	// Store in L2 if enabled
	if cv.l2Enabled {
		// Store synchronously in tests to avoid timing issues
		if ctx.Value("test_mode") != nil {
			cv.storeInL2(ctx, keyStr, entry)
		} else {
			go cv.storeInL2(ctx, keyStr, entry)
		}
	}
	
	// Notify coordinator if available
	if cv.coordinator != nil {
		cv.coordinator.NotifyCacheUpdate(ctx, CacheUpdateMessage{
			NodeID:    cv.nodeID,
			Key:       keyStr,
			EventType: string(entry.Key.EventType),
			Timestamp: time.Now(),
		})
	}
}

func (cv *CacheValidator) storeInL2(ctx context.Context, key string, entry *ValidationCacheEntry) {
	// Use retry mechanism for L2 cache operations
	err := eventerrors.RetryWithLogging(ctx, cv.retryPolicy, func(ctx context.Context, attempt int) error {
		// Serialize
		data, err := json.Marshal(entry)
		if err != nil {
			// Create structured error for serialization failure
			serErr := eventerrors.NewCacheError(eventerrors.CacheErrorSerializationFailed, 
				"Failed to serialize cache entry for L2 storage").
				WithLevel("L2").
				WithKey(key).
				WithOperation("serialize").
				WithCause(err)
			
			{
				cv.logger.Error(fmt.Sprintf("Cache serialization failed: %v", serErr))
			}
			return serErr
		}
		
		// Compress if enabled
		if cv.compressionEnabled {
			compressed, ratio := cv.compress(data)
			if compressed == nil {
				compErr := eventerrors.NewCacheError(eventerrors.CacheErrorCompressionFailed, 
					"Failed to compress cache entry for L2 storage").
					WithLevel("L2").
					WithKey(key).
					WithOperation("compress")
				
				{
					cv.logger.Error(fmt.Sprintf("Cache compression failed: %v", compErr))
				}
				return compErr
			}
			data = compressed
			entry.CompressionRatio = ratio
		}
		
		// Store with TTL
		if err := cv.l2Cache.Set(ctx, key, data, cv.l2TTL); err != nil {
			// Create structured error for L2 cache set failure
			setErr := eventerrors.NewCacheError(eventerrors.CacheErrorConnectionFailed, 
				"Failed to store entry in L2 cache").
				WithLevel("L2").
				WithKey(key).
				WithOperation("set").
				WithSize(int64(len(data))).
				WithCause(err)
			
			{
				cv.logger.Error(fmt.Sprintf("L2 cache set failed: %v", setErr))
			}
			return setErr
		}
		
		return nil
	}, cv.logger)
	
	// If all retries failed, log the final error but don't crash the system
	if err != nil {
		{
			cv.logger.Error(fmt.Sprintf("L2 cache store operation failed after retries: %v", err))
		}
	}
}

func (cv *CacheValidator) promoteToL1(key *ValidationCacheKey, entry *ValidationCacheEntry) {
	keyStr := cv.cacheKeyToString(key)
	cv.mu.Lock()
	cv.l1Cache.Add(keyStr, entry)
	cv.mu.Unlock()
}

func (cv *CacheValidator) invalidateKey(ctx context.Context, key *ValidationCacheKey) error {
	keyStr := cv.cacheKeyToString(key)
	
	// Remove from L1
	cv.mu.Lock()
	cv.l1Cache.Remove(keyStr)
	cv.mu.Unlock()
	
	// Remove from L2 if enabled with retry mechanism
	if cv.l2Enabled {
		err := eventerrors.RetryWithLogging(ctx, cv.retryPolicy, func(ctx context.Context, attempt int) error {
			if err := cv.l2Cache.Delete(ctx, keyStr); err != nil {
				// Create structured error for L2 cache deletion failure
				delErr := eventerrors.NewCacheError(eventerrors.CacheErrorEvictionFailed, 
					"Failed to delete key from L2 cache during invalidation").
					WithLevel("L2").
					WithKey(keyStr).
					WithOperation("delete").
					WithCause(err)
				
				return delErr
			}
			return nil
		}, cv.logger)
		
		if err != nil {
			// Log final error but don't fail the invalidation completely
			{
				cv.logger.Error(fmt.Sprintf("L2 cache invalidation failed after retries: %v", err))
			}
			// Return the error for this specific case since invalidation is critical
			return fmt.Errorf("failed to delete from L2 cache: %w", err)
		}
	}
	
	// Notify invalidation strategies
	for _, strategy := range cv.invalidationStrategies {
		strategy.OnInvalidate(*key)
	}
	
	// Don't increment evictions here - the LRU eviction callback already does it
	// atomic.AddUint64(&cv.stats.Evictions, 1)
	
	return nil
}

func (cv *CacheValidator) cacheKeyToString(key *ValidationCacheKey) string {
	return fmt.Sprintf("validation:%s:%s:%s:%s", 
		key.EventType, 
		key.EventHash[:8], 
		key.ConfigHash[:8],
		key.ValidatorID)
}

func (cv *CacheValidator) toValidationError(entry *ValidationCacheEntry) error {
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

func (cv *CacheValidator) extractErrors(err error) []error {
	if err == nil {
		return nil
	}
	
	// TODO: Implement error extraction logic
	return []error{err}
}

func (cv *CacheValidator) recordHit(level CacheLevel) {
	if !cv.metricsEnabled {
		return
	}
	
	switch level {
	case L1Cache:
		atomic.AddUint64(&cv.stats.L1Hits, 1)
	case L2Cache:
		atomic.AddUint64(&cv.stats.L2Hits, 1)
	}
}

func (cv *CacheValidator) recordMiss() {
	if !cv.metricsEnabled {
		return
	}
	
	atomic.AddUint64(&cv.stats.L1Misses, 1)
}

func (cv *CacheValidator) estimateEventSize(event events.Event) int {
	// Simple size estimation
	data, err := json.Marshal(event)
	if err != nil {
		return 0
	}
	return len(data)
}

func (cv *CacheValidator) compress(data []byte) ([]byte, float64) {
	// TODO: Implement compression
	return data, 1.0
}

func (cv *CacheValidator) decompress(data []byte) ([]byte, error) {
	// TODO: Implement decompression
	return data, nil
}

// Background workers

func (cv *CacheValidator) expirationWorker() {
	defer cv.wg.Done()
	
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-cv.shutdownCh:
			return
		case <-ticker.C:
			cv.cleanupExpired()
		}
	}
}

func (cv *CacheValidator) metricsWorker() {
	defer cv.wg.Done()
	
	if !cv.metricsEnabled {
		return
	}
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-cv.shutdownCh:
			return
		case <-ticker.C:
			cv.updateMetrics()
		}
	}
}

func (cv *CacheValidator) cleanupExpired() {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	
	now := time.Now()
	keys := cv.l1Cache.Keys()
	
	for _, key := range keys {
		if entry, ok := cv.l1Cache.Peek(key); ok && entry != nil {
			if now.After(entry.ExpiresAt) {
				cv.l1Cache.Remove(key)
				atomic.AddUint64(&cv.stats.Expirations, 1)
			}
		}
	}
}

func (cv *CacheValidator) updateMetrics() {
	// Update hit ratios and other metrics
	stats := cv.GetStats()
	if cv.coordinator != nil {
		cv.coordinator.ReportMetrics(context.Background(), MetricsReport{
			NodeID:    cv.nodeID,
			Stats:     stats,
			Timestamp: time.Now(),
		})
	}
}