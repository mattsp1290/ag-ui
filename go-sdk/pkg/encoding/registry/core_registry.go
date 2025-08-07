package registry

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FormatValidator interface for validation framework integration
type FormatValidator interface {
	ValidateFormat(mimeType string, data []byte) error
	ValidateEncoding(mimeType string, data []byte) error
	ValidateDecoding(mimeType string, data []byte) error
}

// AliasProvider interface for formats that have aliases
type AliasProvider interface {
	GetAliases() []string
}

// Core interfaces needed (these will be satisfied by the main encoding package)
type Encoder interface {
	Encode(ctx context.Context, event interface{}) ([]byte, error)
	EncodeMultiple(ctx context.Context, events []interface{}) ([]byte, error)
	ContentType() string
}

type Decoder interface {
	Decode(ctx context.Context, data []byte) (interface{}, error)
	DecodeMultiple(ctx context.Context, data []byte) ([]interface{}, error)
	ContentType() string
}

type Codec interface {
	Encoder
	Decoder
}

// CoreRegistry manages encoders, decoders, and format information with cleanup capabilities
// This is implemented using sync.Map with composite keys and composed of focused components
type CoreRegistry struct {
	// Core data storage
	entries sync.Map // map[RegistryKey]*RegistryEntry

	// Focused component responsibilities
	cacheManager *CacheManager
	priorities   *PriorityManager
	lifecycle    *LifecycleManager
	metrics      *Metrics

	// Configuration
	config    *RegistryConfig
	validator FormatValidator
	closed    int32 // atomic flag for graceful shutdown
}

// NewCoreRegistry creates a new core registry with default cleanup configuration
func NewCoreRegistry() *CoreRegistry {
	return NewCoreRegistryWithConfig(DefaultRegistryConfig())
}

// NewCoreRegistryWithConfig creates a new core registry with custom configuration
// Now uses focused component composition to prevent memory leaks and reduce complexity
func NewCoreRegistryWithConfig(config *RegistryConfig) *CoreRegistry {
	if config == nil {
		config = DefaultRegistryConfig()
	}

	r := &CoreRegistry{
		cacheManager: NewCacheManager(config),
		priorities:   NewPriorityManager("application/json"), // default format
		lifecycle:    NewLifecycleManager(config),
		metrics:      NewMetrics(),
		config:       config,
	}

	// Start background cleanup if enabled
	if config.EnableBackgroundCleanup && config.CleanupInterval > 0 {
		r.lifecycle.StartBackgroundCleanup(r.cleanupCallback)
	}

	return r
}

// cleanupCallback is used by the lifecycle manager
func (r *CoreRegistry) cleanupCallback() {
	r.CleanupExpired()

	// If we're under memory pressure, do more aggressive cleanup
	currentCount := r.metrics.GetEntryCount()
	maxEntries := int64(r.config.MaxEntries)

	if maxEntries > 0 && currentCount > (maxEntries*8)/10 {
		r.CleanupByAccessTime(r.config.TTL / 2) // More aggressive cleanup
	}
}

// Helper methods for entry management

// GetEntry safely retrieves an entry from the sync.Map
func (r *CoreRegistry) GetEntry(entryType RegistryEntryType, mimeType string) (*RegistryEntry, bool) {
	if r.lifecycle.IsClosed() {
		return nil, false
	}

	// Normalize the MIME type to lowercase for consistent access
	normalizedMimeType := strings.ToLower(mimeType)
	key := RegistryKey{EntryType: entryType, MimeType: normalizedMimeType}
	if value, ok := r.entries.Load(key); ok {
		entry := value.(*RegistryEntry)
		// Update access tracking atomically
		entry.SetLastAccess(time.Now())
		atomic.AddInt64(&entry.AccessCount, 1)

		// Update LRU position if enabled
		if r.config.EnableLRU {
			r.cacheManager.UpdateLRUPosition(key)
		}

		return entry, true
	}
	return nil, false
}

// SetEntry safely stores an entry in the sync.Map with size limits and LRU eviction
func (r *CoreRegistry) SetEntry(entryType RegistryEntryType, mimeType string, value interface{}) error {
	if r.lifecycle.IsClosed() {
		return fmt.Errorf("registry is closed")
	}

	// Normalize the MIME type to lowercase for consistent storage
	normalizedMimeType := strings.ToLower(mimeType)
	key := RegistryKey{EntryType: entryType, MimeType: normalizedMimeType}

	// Check if we need to evict before adding (if max entries configured)
	// Only count format entries towards the limit, not aliases or factories
	if r.config.MaxEntries > 0 && entryType == EntryTypeFormat {
		currentFormatCount := r.countFormatEntries()
		if currentFormatCount >= r.config.MaxEntries {
			r.evictLRUFormatEntry()
		}
	}

	// Create new entry with metadata
	now := time.Now()
	entry := &RegistryEntry{
		Value:       value,
		CreatedAt:   now,
		AccessCount: 1,
	}
	entry.SetLastAccess(now)

	// Store in sync.Map
	r.entries.Store(key, entry)
	r.metrics.IncrementEntryCount()

	// Update LRU tracking if enabled
	if r.config.EnableLRU {
		r.cacheManager.AddToLRU(key)
	}

	return nil
}

// DeleteEntry safely removes an entry from the sync.Map
func (r *CoreRegistry) DeleteEntry(entryType RegistryEntryType, mimeType string) bool {
	// Normalize the MIME type to lowercase for consistent access
	normalizedMimeType := strings.ToLower(mimeType)
	key := RegistryKey{EntryType: entryType, MimeType: normalizedMimeType}

	if _, loaded := r.entries.LoadAndDelete(key); loaded {
		r.metrics.DecrementEntryCount()

		// Remove from LRU tracking
		if r.config.EnableLRU {
			r.cacheManager.RemoveFromLRU(key)
		}
		return true
	}
	return false
}

// evictLRUFormatEntry removes the least recently used format entry using cache manager
func (r *CoreRegistry) evictLRUFormatEntry() {
	// Find the least recently used format entry (not just any entry)
	oldestFormatKey := r.cacheManager.FindOldestFormatEntry()
	if oldestFormatKey == nil {
		return // No format entries to evict
	}

	mimeType := oldestFormatKey.MimeType

	// Get the FormatInfo before deleting the format entry (to extract aliases)
	if entry, exists := r.GetEntry(EntryTypeFormat, mimeType); exists {
		// Extract aliases from format info if it implements AliasProvider
		if aliasProvider, ok := entry.Value.(AliasProvider); ok {
			for _, alias := range aliasProvider.GetAliases() {
				r.DeleteEntry(EntryTypeAlias, alias)
			}
		}
	}

	// Remove all related entries for this format
	entriesToRemove := []RegistryEntryType{
		EntryTypeFormat,
		EntryTypeEncoderFactory,
		EntryTypeDecoderFactory,
		EntryTypeCodecFactory,
		EntryTypeLegacyEncoderFactory,
		EntryTypeLegacyDecoderFactory,
		EntryTypeLegacyCodecFactory,
	}

	for _, entryType := range entriesToRemove {
		if r.DeleteEntry(entryType, mimeType) {
			// Also remove from LRU tracking
			key := RegistryKey{EntryType: entryType, MimeType: mimeType}
			r.cacheManager.RemoveFromLRU(key)
		}
	}
}

// countFormatEntries counts only format entries
func (r *CoreRegistry) countFormatEntries() int {
	count := 0
	r.entries.Range(func(key, value interface{}) bool {
		k := key.(RegistryKey)
		if k.EntryType == EntryTypeFormat {
			count++
		}
		return true
	})
	return count
}

// ResolveAlias resolves an alias to canonical MIME type
func (r *CoreRegistry) ResolveAlias(mimeType string) string {
	lowerMimeType := strings.ToLower(mimeType)

	// Check if it's an alias
	if entry, exists := r.GetEntry(EntryTypeAlias, lowerMimeType); exists {
		return entry.Value.(string)
	}

	// Also check without parameters (e.g., "application/json; charset=utf-8" -> "application/json")
	if idx := strings.Index(mimeType, ";"); idx > 0 {
		base := strings.TrimSpace(mimeType[:idx])
		lowerBase := strings.ToLower(base)
		if entry, exists := r.GetEntry(EntryTypeAlias, lowerBase); exists {
			return entry.Value.(string)
		}
		return lowerBase // Return lowercase version
	}

	return lowerMimeType // Return lowercase version
}

// Cleanup Methods for Memory Management

// CleanupExpired removes all expired entries based on TTL
func (r *CoreRegistry) CleanupExpired() (int, error) {
	if r.config.TTL <= 0 {
		return 0, nil // TTL not configured
	}

	if atomic.LoadInt32(&r.closed) != 0 {
		return 0, fmt.Errorf("registry is closed")
	}

	cutoff := time.Now().Add(-r.config.TTL)
	totalCleaned := int64(0)
	var keysToDelete []RegistryKey

	// Collect expired entries
	r.entries.Range(func(key, value interface{}) bool {
		k := key.(RegistryKey)
		entry := value.(*RegistryEntry)

		if entry.CreatedAt.Before(cutoff) {
			keysToDelete = append(keysToDelete, k)
		}
		return true
	})

	// Remove expired entries
	for _, key := range keysToDelete {
		if _, loaded := r.entries.LoadAndDelete(key); loaded {
			atomic.AddInt64(&totalCleaned, 1)
			r.metrics.DecrementEntryCount()

			// Remove from LRU tracking
			if r.config.EnableLRU {
				r.cacheManager.RemoveFromLRU(key)
			}
		}
	}

	return int(totalCleaned), nil
}

// CleanupByAccessTime removes entries that haven't been accessed within the given duration
func (r *CoreRegistry) CleanupByAccessTime(maxAge time.Duration) (int, error) {
	if atomic.LoadInt32(&r.closed) != 0 {
		return 0, fmt.Errorf("registry is closed")
	}

	cutoff := time.Now().Add(-maxAge)
	totalCleaned := int64(0)
	var keysToDelete []RegistryKey

	// Collect old entries
	r.entries.Range(func(key, value interface{}) bool {
		k := key.(RegistryKey)
		entry := value.(*RegistryEntry)

		if entry.GetLastAccess().Before(cutoff) {
			keysToDelete = append(keysToDelete, k)
		}
		return true
	})

	// Remove old entries
	for _, key := range keysToDelete {
		if _, loaded := r.entries.LoadAndDelete(key); loaded {
			atomic.AddInt64(&totalCleaned, 1)
			r.metrics.DecrementEntryCount()

			// Remove from LRU tracking
			if r.config.EnableLRU {
				r.cacheManager.RemoveFromLRU(key)
			}
		}
	}

	return int(totalCleaned), nil
}

// ClearAll removes all entries from the registry
func (r *CoreRegistry) ClearAll() error {
	if atomic.LoadInt32(&r.closed) != 0 {
		return fmt.Errorf("registry is closed")
	}

	// Clear all entries
	r.entries.Range(func(key, value interface{}) bool {
		r.entries.Delete(key)
		return true
	})

	// Clear LRU structures through cache manager
	r.cacheManager.Clear()

	// Clear priorities through the priority manager
	r.priorities.UpdatePriorities(make(map[string]FormatInfoInterface))

	// Reset metrics
	r.metrics.Reset()

	return nil
}

// GetRegistryStats returns statistics about the registry
func (r *CoreRegistry) GetRegistryStats() map[string]interface{} {
	// Count entries by type
	counts := make(map[RegistryEntryType]int)
	totalEntries := int64(0)

	r.entries.Range(func(key, value interface{}) bool {
		k := key.(RegistryKey)
		counts[k.EntryType]++
		totalEntries++
		return true
	})

	// Get LRU size from cache manager
	lruSize := r.cacheManager.Size()

	stats := map[string]interface{}{
		"formats_count":                  counts[EntryTypeFormat],
		"encoder_factories_count":        counts[EntryTypeEncoderFactory],
		"decoder_factories_count":        counts[EntryTypeDecoderFactory],
		"codec_factories_count":          counts[EntryTypeCodecFactory],
		"legacy_encoder_factories_count": counts[EntryTypeLegacyEncoderFactory],
		"legacy_decoder_factories_count": counts[EntryTypeLegacyDecoderFactory],
		"legacy_codec_factories_count":   counts[EntryTypeLegacyCodecFactory],
		"aliases_count":                  counts[EntryTypeAlias],
		"total_entries":                  int(totalEntries),
		"atomic_entry_count":             int(r.metrics.GetEntryCount()),
		"lru_size":                       lruSize,
		"max_entries_per_map":            r.config.MaxEntries,
		"ttl_seconds":                    r.config.TTL.Seconds(),
		"cleanup_interval_seconds":       r.config.CleanupInterval.Seconds(),
		"lru_enabled":                    r.config.EnableLRU,
		"background_cleanup_enabled":     r.config.EnableBackgroundCleanup,
		"is_closed":                      atomic.LoadInt32(&r.closed) != 0,
	}

	return stats
}

// UpdateConfig updates the registry configuration
func (r *CoreRegistry) UpdateConfig(config *RegistryConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	if r.lifecycle.IsClosed() {
		return fmt.Errorf("registry is closed")
	}

	oldConfig := r.config

	// Update config
	r.config = config
	r.cacheManager.config = config

	// If background cleanup settings changed, restart background cleanup
	if oldConfig.EnableBackgroundCleanup != config.EnableBackgroundCleanup ||
		oldConfig.CleanupInterval != config.CleanupInterval {

		// Close and recreate lifecycle manager
		r.lifecycle.Close()
		r.lifecycle = NewLifecycleManager(config)

		// Start new cleanup if enabled
		if config.EnableBackgroundCleanup && config.CleanupInterval > 0 {
			r.lifecycle.StartBackgroundCleanup(r.cleanupCallback)
		}
	}

	return nil
}

// Close stops background cleanup and releases resources
func (r *CoreRegistry) Close() error {
	// Set the registry as closed
	atomic.StoreInt32(&r.closed, 1)

	// Close the lifecycle manager (stops background cleanup)
	return r.lifecycle.Close()
}

// Memory pressure adaptation methods

// AdaptToMemoryPressure adjusts cleanup behavior based on current memory usage
func (r *CoreRegistry) AdaptToMemoryPressure(pressureLevel int) error {
	if r.lifecycle.IsClosed() {
		return fmt.Errorf("registry is closed")
	}

	currentCount := r.metrics.GetEntryCount()

	switch pressureLevel {
	case 1: // Low pressure - normal cleanup
		if r.config.TTL > 0 {
			return nil // Let normal cleanup handle it
		}

	case 2: // Medium pressure - more aggressive TTL
		if r.config.TTL > 0 {
			r.CleanupByAccessTime(r.config.TTL / 2)
		} else {
			// If no TTL configured, use 1 hour default
			r.CleanupByAccessTime(1 * time.Hour)
		}

	case 3: // High pressure - very aggressive cleanup
		if r.config.TTL > 0 {
			r.CleanupByAccessTime(r.config.TTL / 4)
		} else {
			// Very aggressive - 30 minutes
			r.CleanupByAccessTime(30 * time.Minute)
		}

		// Also force LRU eviction if we have too many entries
		if r.config.MaxEntries > 0 && int(currentCount) > r.config.MaxEntries {
			// Under high memory pressure, be more aggressive with eviction
			targetEntries := r.config.MaxEntries * 2 // Target double the max entries
			excessEntries := int(currentCount) - targetEntries
			if excessEntries > 0 {
				maxEvictions := excessEntries
				if maxEvictions > 500 { // Set a reasonable upper bound
					maxEvictions = 500
				}
				for i := 0; i < maxEvictions; i++ {
					if key, ok := r.cacheManager.EvictOldest(); ok {
						// Actually remove the entry from the sync.Map
						r.entries.Delete(key)
						r.metrics.DecrementEntryCount()
					} else {
						break // No more entries to evict
					}
				}
			}
		}

	default:
		return fmt.Errorf("pressure level must be between 1 and 3")
	}

	return nil
}

// ForceCleanup immediately removes entries that haven't been accessed recently
// This is useful for testing and emergency memory cleanup
func (r *CoreRegistry) ForceCleanup(olderThan time.Duration) (int, error) {
	if r.lifecycle.IsClosed() {
		return 0, fmt.Errorf("registry is closed")
	}

	return r.CleanupByAccessTime(olderThan)
}

// ListEntries returns all entries of a specific type for inspection
func (r *CoreRegistry) ListEntries(entryType RegistryEntryType) map[string]interface{} {
	entries := make(map[string]interface{})

	r.entries.Range(func(key, value interface{}) bool {
		k := key.(RegistryKey)
		if k.EntryType == entryType {
			entry := value.(*RegistryEntry)
			entries[k.MimeType] = entry.Value
		}
		return true
	})

	return entries
}

// GetCacheManager returns the cache manager for advanced operations
func (r *CoreRegistry) GetCacheManager() *CacheManager {
	return r.cacheManager
}

// GetPriorityManager returns the priority manager for advanced operations
func (r *CoreRegistry) GetPriorityManager() *PriorityManager {
	return r.priorities
}

// GetLifecycleManager returns the lifecycle manager for advanced operations
func (r *CoreRegistry) GetLifecycleManager() *LifecycleManager {
	return r.lifecycle
}

// GetMetrics returns the metrics manager for advanced operations
func (r *CoreRegistry) GetMetrics() *Metrics {
	return r.metrics
}
