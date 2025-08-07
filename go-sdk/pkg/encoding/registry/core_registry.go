package registry

import (
	"context"
	"fmt"
	"log"
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

// cleanupCallback is used by the lifecycle manager with enhanced memory leak prevention
func (r *CoreRegistry) cleanupCallback() {
	// Track cleanup operations for monitoring
	cleanupStart := time.Now()
	
	// Run standard TTL-based cleanup
	expiredCleaned, _ := r.CleanupExpired()
	
	// Check for memory pressure and respond accordingly
	currentCount := r.metrics.GetEntryCount()
	maxEntries := int64(r.config.MaxEntries)
	
	// Perform preventative cleanup through cache manager
	preventativeCleaned := r.cacheManager.PerformPreventativeCleanup()
	
	if maxEntries > 0 && currentCount > (maxEntries*8)/10 {
		// Under memory pressure - more aggressive cleanup
		accessCleaned, _ := r.CleanupByAccessTime(r.config.TTL / 2)
		
		if r.config.TTL > 0 {
			log.Printf("Registry memory pressure detected: cleaned %d expired, %d by access, %d preventative entries in %v",
				expiredCleaned, accessCleaned, preventativeCleaned, time.Since(cleanupStart))
		}
	} else if expiredCleaned > 0 || preventativeCleaned > 0 {
		// Log normal cleanup activity
		log.Printf("Registry maintenance: cleaned %d expired, %d preventative entries in %v",
			expiredCleaned, preventativeCleaned, time.Since(cleanupStart))
	}
	
	// Update metrics with cleanup information
	r.metrics.RecordCleanupOperation(expiredCleaned + preventativeCleaned, time.Since(cleanupStart))
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

// SetEntry safely stores an entry in the sync.Map with enhanced size limits and leak prevention
func (r *CoreRegistry) SetEntry(entryType RegistryEntryType, mimeType string, value interface{}) error {
	if r.lifecycle.IsClosed() {
		return fmt.Errorf("registry is closed")
	}

	// Normalize the MIME type to lowercase for consistent storage
	normalizedMimeType := strings.ToLower(mimeType)
	key := RegistryKey{EntryType: entryType, MimeType: normalizedMimeType}

	// Enhanced memory pressure management
	// Handle MaxEntries limit - only apply to format entries to avoid confusion
	if r.config.MaxEntries > 0 && entryType == EntryTypeFormat {
		currentFormatCount := r.countFormatEntries()
		if currentFormatCount >= r.config.MaxEntries {
			r.evictLRUFormatEntry()
		}
	}

	// Check if entry already exists and update instead of creating duplicate
	if existingValue, exists := r.entries.Load(key); exists {
		// Update existing entry to prevent accumulation
		existingEntry := existingValue.(*RegistryEntry)
		existingEntry.Value = value
		existingEntry.SetLastAccess(time.Now())
		atomic.AddInt64(&existingEntry.AccessCount, 1)
		
		// Update LRU position
		if r.config.EnableLRU {
			r.cacheManager.UpdateLRUPosition(key)
		}
		
		return nil
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

// GetRegistryStats returns comprehensive statistics about the registry with enhanced monitoring
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

	// Get enhanced metrics
	lruSize := r.cacheManager.Size()
	cleanupMetrics := r.metrics.GetHealthMetrics()
	evictionCount := r.cacheManager.GetEvictionCount()

	// Calculate memory pressure percentage
	memoryPressurePercent := 0
	if r.config.MaxEntries > 0 {
		memoryPressurePercent = int((totalEntries * 100) / int64(r.config.MaxEntries))
	}

	stats := map[string]interface{}{
		// Entry counts by type
		"formats_count":                  counts[EntryTypeFormat],
		"encoder_factories_count":        counts[EntryTypeEncoderFactory],
		"decoder_factories_count":        counts[EntryTypeDecoderFactory],
		"codec_factories_count":          counts[EntryTypeCodecFactory],
		"legacy_encoder_factories_count": counts[EntryTypeLegacyEncoderFactory],
		"legacy_decoder_factories_count": counts[EntryTypeLegacyDecoderFactory],
		"legacy_codec_factories_count":   counts[EntryTypeLegacyCodecFactory],
		"aliases_count":                  counts[EntryTypeAlias],
		
		// Total counts
		"total_entries":                  int(totalEntries),
		"atomic_entry_count":             int(r.metrics.GetEntryCount()),
		"lru_size":                       lruSize,
		"total_evictions":                evictionCount,
		
		// Configuration
		"max_entries_per_map":                r.config.MaxEntries,
		"ttl_seconds":                        r.config.TTL.Seconds(),
		"cleanup_interval_seconds":           r.config.CleanupInterval.Seconds(),
		"preventative_cleanup_interval_seconds": r.config.PreventativeCleanupInterval.Seconds(),
		"memory_pressure_threshold":          r.config.MemoryPressureThreshold,
		"batch_eviction_size":               r.config.BatchEvictionSize,
		"max_memory_pressure_level":         r.config.MaxMemoryPressureLevel,
		
		// Features
		"lru_enabled":                    r.config.EnableLRU,
		"background_cleanup_enabled":     r.config.EnableBackgroundCleanup,
		"memory_pressure_logging_enabled": r.config.EnableMemoryPressureLogging,
		
		// Status
		"is_closed":                      atomic.LoadInt32(&r.closed) != 0,
		"memory_pressure_percent":        memoryPressurePercent,
		"is_under_memory_pressure":       r.cacheManager.CheckMemoryPressure(),
	}

	// Merge cleanup metrics
	for key, value := range cleanupMetrics {
		stats[key] = value
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

// AdaptToMemoryPressure adjusts cleanup behavior with enhanced leak prevention
func (r *CoreRegistry) AdaptToMemoryPressure(pressureLevel int) error {
	if r.lifecycle.IsClosed() {
		return fmt.Errorf("registry is closed")
	}

	currentCount := r.metrics.GetEntryCount()
	cleanupStart := time.Now()
	var totalCleaned int

	log.Printf("Adapting to memory pressure level %d, current entries: %d", pressureLevel, currentCount)

	switch pressureLevel {
	case 1: // Low pressure - normal cleanup with preventative measures
		preventativeCleaned := r.cacheManager.PerformPreventativeCleanup()
		if r.config.TTL > 0 {
			expiredCleaned, _ := r.CleanupExpired()
			totalCleaned = expiredCleaned + preventativeCleaned
		} else {
			totalCleaned = preventativeCleaned
		}

	case 2: // Medium pressure - more aggressive cleanup
		if r.config.TTL > 0 {
			cleaned, _ := r.CleanupByAccessTime(r.config.TTL / 2)
			totalCleaned += cleaned
		} else {
			// If no TTL configured, use 1 hour default
			cleaned, _ := r.CleanupByAccessTime(1 * time.Hour)
			totalCleaned += cleaned
		}
		
		// Also perform batch eviction
		evicted := r.cacheManager.PerformBatchEviction(int(currentCount / 20)) // 5%
		for _, key := range evicted {
			r.entries.Delete(key)
			r.metrics.DecrementEntryCount()
		}
		totalCleaned += len(evicted)

	case 3: // High pressure - very aggressive cleanup with bounded limits
		if r.config.TTL > 0 {
			cleaned, _ := r.CleanupByAccessTime(r.config.TTL / 4)
			totalCleaned += cleaned
		} else {
			// Very aggressive - 30 minutes
			cleaned, _ := r.CleanupByAccessTime(30 * time.Minute)
			totalCleaned += cleaned
		}

		// Aggressive batch eviction to prevent unbounded growth
		maxEvictions := int(currentCount / 10) // Evict up to 10%
		if maxEvictions < 50 {
			maxEvictions = 50 // Minimum aggressive cleanup
		}
		if maxEvictions > 1000 { // Cap to prevent system overload
			maxEvictions = 1000
		}
		
		evicted := r.cacheManager.PerformBatchEviction(maxEvictions)
		for _, key := range evicted {
			r.entries.Delete(key)
			r.metrics.DecrementEntryCount()
		}
		totalCleaned += len(evicted)
		
		// If we still have too many entries, do emergency cleanup
		updatedCount := r.metrics.GetEntryCount()
		if r.config.MaxEntries > 0 && int(updatedCount) > r.config.MaxEntries*2 {
			// Emergency cleanup - remove oldest 25% of entries
			emergencyEvictions := int(updatedCount / 4)
			emergencyEvicted := r.cacheManager.PerformBatchEviction(emergencyEvictions)
			for _, key := range emergencyEvicted {
				r.entries.Delete(key)
				r.metrics.DecrementEntryCount()
			}
			totalCleaned += len(emergencyEvicted)
			log.Printf("Emergency cleanup performed: evicted %d entries", len(emergencyEvicted))
		}

	default:
		return fmt.Errorf("pressure level must be between 1 and 3")
	}

	log.Printf("Memory pressure adaptation completed: cleaned %d entries in %v, remaining: %d",
		totalCleaned, time.Since(cleanupStart), r.metrics.GetEntryCount())

	// Record the memory pressure adaptation
	r.metrics.RecordMemoryPressureAdaptation(pressureLevel, totalCleaned, time.Since(cleanupStart))

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
