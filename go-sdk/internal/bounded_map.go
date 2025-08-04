// Package internal provides bounded map implementations with LRU eviction and TTL support
// to prevent memory leaks in long-running services.
package internal

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// BoundedMapConfig configures bounded map behavior
type BoundedMapConfig struct {
	// MaxSize is the maximum number of entries (0 = unlimited)
	MaxSize int `json:"max_size"`
	
	// TTL is the time-to-live for entries (0 = no TTL)
	TTL time.Duration `json:"ttl"`
	
	// CleanupInterval is how often to run cleanup (default: TTL/4)
	CleanupInterval time.Duration `json:"cleanup_interval"`
	
	// EvictionCallback is called when entries are evicted
	EvictionCallback func(key, value interface{}, reason EvictionReason) `json:"-"`
	
	// EnableMetrics enables metrics collection
	EnableMetrics bool `json:"enable_metrics"`
	
	// MetricsPrefix is the prefix for metrics names
	MetricsPrefix string `json:"metrics_prefix"`
}

// EvictionReason represents why an entry was evicted
type EvictionReason int

const (
	// EvictionReasonTTL indicates entry was evicted due to TTL expiry
	EvictionReasonTTL EvictionReason = iota
	
	// EvictionReasonCapacity indicates entry was evicted due to capacity limit
	EvictionReasonCapacity
	
	// EvictionReasonExplicit indicates entry was explicitly removed
	EvictionReasonExplicit
)

// String returns the string representation of eviction reason
func (r EvictionReason) String() string {
	switch r {
	case EvictionReasonTTL:
		return "ttl"
	case EvictionReasonCapacity:
		return "capacity"
	case EvictionReasonExplicit:
		return "explicit"
	default:
		return "unknown"
	}
}

// boundedMapEntry represents an entry in the bounded map
type boundedMapEntry struct {
	key        interface{}
	value      interface{}
	expiresAt  time.Time
	accessTime time.Time
	listElem   *list.Element
}

// BoundedMap is a thread-safe map with size limits, TTL, and LRU eviction
type BoundedMap struct {
	config      BoundedMapConfig
	data        map[interface{}]*boundedMapEntry
	accessList  *list.List // LRU list for access order
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	logger      *logrus.Logger
	
	// Metrics
	metrics *boundedMapMetrics
}

// boundedMapMetrics contains metrics for bounded map operations
type boundedMapMetrics struct {
	entries         metric.Int64UpDownCounter
	hits            metric.Int64Counter
	misses          metric.Int64Counter
	evictions       metric.Int64Counter
	ttlEvictions    metric.Int64Counter
	lruEvictions    metric.Int64Counter
	cleanupRuns     metric.Int64Counter
	cleanupDuration metric.Float64Histogram
}

// NewBoundedMap creates a new bounded map with the given configuration
func NewBoundedMap(config BoundedMapConfig) *BoundedMap {
	// Set default values
	if config.CleanupInterval == 0 && config.TTL > 0 {
		config.CleanupInterval = config.TTL / 4
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 5 * time.Minute // Default cleanup interval
	}
	if config.MetricsPrefix == "" {
		config.MetricsPrefix = "bounded_map"
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	bm := &BoundedMap{
		config:     config,
		data:       make(map[interface{}]*boundedMapEntry),
		accessList: list.New(),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logrus.New(),
	}

	// Initialize metrics if enabled
	if config.EnableMetrics {
		bm.initializeMetrics()
	}

	// Start cleanup worker
	bm.wg.Add(1)
	go bm.cleanupWorker()

	return bm
}

// initializeMetrics initializes OpenTelemetry metrics
func (bm *BoundedMap) initializeMetrics() {
	meter := otel.Meter("ag-ui-bounded-map")
	prefix := bm.config.MetricsPrefix

	var err error
	bm.metrics = &boundedMapMetrics{}

	bm.metrics.entries, err = meter.Int64UpDownCounter(
		prefix+"_entries",
		metric.WithDescription("Number of entries in bounded map"),
	)
	if err != nil {
		bm.logger.WithError(err).Warn("Failed to create entries metric")
	}

	bm.metrics.hits, err = meter.Int64Counter(
		prefix+"_hits_total",
		metric.WithDescription("Total number of cache hits"),
	)
	if err != nil {
		bm.logger.WithError(err).Warn("Failed to create hits metric")
	}

	bm.metrics.misses, err = meter.Int64Counter(
		prefix+"_misses_total",
		metric.WithDescription("Total number of cache misses"),
	)
	if err != nil {
		bm.logger.WithError(err).Warn("Failed to create misses metric")
	}

	bm.metrics.evictions, err = meter.Int64Counter(
		prefix+"_evictions_total",
		metric.WithDescription("Total number of evictions"),
	)
	if err != nil {
		bm.logger.WithError(err).Warn("Failed to create evictions metric")
	}

	bm.metrics.ttlEvictions, err = meter.Int64Counter(
		prefix+"_ttl_evictions_total",
		metric.WithDescription("Total number of TTL evictions"),
	)
	if err != nil {
		bm.logger.WithError(err).Warn("Failed to create ttl_evictions metric")
	}

	bm.metrics.lruEvictions, err = meter.Int64Counter(
		prefix+"_lru_evictions_total",
		metric.WithDescription("Total number of LRU evictions"),
	)
	if err != nil {
		bm.logger.WithError(err).Warn("Failed to create lru_evictions metric")
	}

	bm.metrics.cleanupRuns, err = meter.Int64Counter(
		prefix+"_cleanup_runs_total",
		metric.WithDescription("Total number of cleanup runs"),
	)
	if err != nil {
		bm.logger.WithError(err).Warn("Failed to create cleanup_runs metric")
	}

	bm.metrics.cleanupDuration, err = meter.Float64Histogram(
		prefix+"_cleanup_duration_seconds",
		metric.WithDescription("Duration of cleanup operations in seconds"),
	)
	if err != nil {
		bm.logger.WithError(err).Warn("Failed to create cleanup_duration metric")
	}
}

// Get retrieves a value from the map
func (bm *BoundedMap) Get(key interface{}) (interface{}, bool) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	entry, exists := bm.data[key]
	if !exists {
		if bm.metrics != nil {
			bm.metrics.misses.Add(bm.ctx, 1)
		}
		return nil, false
	}

	// Check if entry has expired
	if bm.config.TTL > 0 && time.Now().After(entry.expiresAt) {
		bm.removeEntryUnsafe(key, entry, EvictionReasonTTL)
		if bm.metrics != nil {
			bm.metrics.misses.Add(bm.ctx, 1)
		}
		return nil, false
	}

	// Update access time and move to front of LRU list
	entry.accessTime = time.Now()
	bm.accessList.MoveToFront(entry.listElem)

	if bm.metrics != nil {
		bm.metrics.hits.Add(bm.ctx, 1)
	}

	return entry.value, true
}

// Set stores a key-value pair in the map
func (bm *BoundedMap) Set(key, value interface{}) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	
	// Check if key already exists
	if entry, exists := bm.data[key]; exists {
		// Update existing entry
		entry.value = value
		entry.accessTime = now
		if bm.config.TTL > 0 {
			entry.expiresAt = now.Add(bm.config.TTL)
		}
		bm.accessList.MoveToFront(entry.listElem)
		return
	}

	// Check if we need to evict entries due to capacity
	if bm.config.MaxSize > 0 && len(bm.data) >= bm.config.MaxSize {
		bm.evictLRUUnsafe()
	}

	// Create new entry
	entry := &boundedMapEntry{
		key:        key,
		value:      value,
		accessTime: now,
	}
	
	if bm.config.TTL > 0 {
		entry.expiresAt = now.Add(bm.config.TTL)
	}

	// Add to data map and LRU list
	entry.listElem = bm.accessList.PushFront(entry)
	bm.data[key] = entry

	if bm.metrics != nil {
		bm.metrics.entries.Add(bm.ctx, 1)
	}
}

// Delete removes a key from the map
func (bm *BoundedMap) Delete(key interface{}) bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	entry, exists := bm.data[key]
	if !exists {
		return false
	}

	bm.removeEntryUnsafe(key, entry, EvictionReasonExplicit)
	return true
}

// Len returns the current number of entries in the map
func (bm *BoundedMap) Len() int {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return len(bm.data)
}

// Clear removes all entries from the map
func (bm *BoundedMap) Clear() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for key, entry := range bm.data {
		bm.removeEntryUnsafe(key, entry, EvictionReasonExplicit)
	}
}

// Keys returns all keys in the map
func (bm *BoundedMap) Keys() []interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	keys := make([]interface{}, 0, len(bm.data))
	for key := range bm.data {
		keys = append(keys, key)
	}
	return keys
}

// GetStats returns statistics about the map
func (bm *BoundedMap) GetStats() map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	stats := map[string]interface{}{
		"size":          len(bm.data),
		"max_size":      bm.config.MaxSize,
		"ttl_seconds":   bm.config.TTL.Seconds(),
		"cleanup_interval_seconds": bm.config.CleanupInterval.Seconds(),
	}

	// Add expired entry count
	now := time.Now()
	expiredCount := 0
	if bm.config.TTL > 0 {
		for _, entry := range bm.data {
			if now.After(entry.expiresAt) {
				expiredCount++
			}
		}
	}
	stats["expired_entries"] = expiredCount

	return stats
}

// Close stops the cleanup worker and releases resources
func (bm *BoundedMap) Close() error {
	bm.cancel()
	bm.wg.Wait()
	return nil
}

// removeEntryUnsafe removes an entry without locking (caller must hold lock)
func (bm *BoundedMap) removeEntryUnsafe(key interface{}, entry *boundedMapEntry, reason EvictionReason) {
	// Remove from data map
	delete(bm.data, key)
	
	// Remove from LRU list
	bm.accessList.Remove(entry.listElem)

	// Call eviction callback if configured
	if bm.config.EvictionCallback != nil {
		go bm.config.EvictionCallback(key, entry.value, reason)
	}

	// Update metrics
	if bm.metrics != nil {
		bm.metrics.entries.Add(bm.ctx, -1)
		bm.metrics.evictions.Add(bm.ctx, 1,
			metric.WithAttributes(attribute.String("reason", reason.String())))
		
		switch reason {
		case EvictionReasonTTL:
			bm.metrics.ttlEvictions.Add(bm.ctx, 1)
		case EvictionReasonCapacity:
			bm.metrics.lruEvictions.Add(bm.ctx, 1)
		}
	}
}

// evictLRUUnsafe evicts the least recently used entry (caller must hold lock)
func (bm *BoundedMap) evictLRUUnsafe() {
	if bm.accessList.Len() == 0 {
		return
	}

	// Get least recently used entry (back of list)
	elem := bm.accessList.Back()
	if elem == nil {
		return
	}

	entry := elem.Value.(*boundedMapEntry)
	bm.removeEntryUnsafe(entry.key, entry, EvictionReasonCapacity)
}

// cleanupWorker runs periodically to clean up expired entries
func (bm *BoundedMap) cleanupWorker() {
	defer bm.wg.Done()

	ticker := time.NewTicker(bm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bm.ctx.Done():
			return
		case <-ticker.C:
			bm.cleanupExpired()
		}
	}
}

// cleanupExpired removes expired entries
func (bm *BoundedMap) cleanupExpired() {
	if bm.config.TTL == 0 {
		return // No TTL configured
	}

	start := time.Now()
	
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]interface{}, 0)

	// Find expired entries
	for key, entry := range bm.data {
		if now.After(entry.expiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// Remove expired entries
	for _, key := range expiredKeys {
		if entry, exists := bm.data[key]; exists {
			bm.removeEntryUnsafe(key, entry, EvictionReasonTTL)
		}
	}

	// Update metrics
	if bm.metrics != nil {
		bm.metrics.cleanupRuns.Add(bm.ctx, 1)
		bm.metrics.cleanupDuration.Record(bm.ctx, time.Since(start).Seconds())
	}

	if len(expiredKeys) > 0 {
		bm.logger.WithFields(logrus.Fields{
			"expired_count": len(expiredKeys),
			"duration_ms":   time.Since(start).Milliseconds(),
		}).Debug("Cleaned up expired entries")
	}
}

// DefaultBoundedMapConfig returns a default configuration for bounded maps
func DefaultBoundedMapConfig() BoundedMapConfig {
	return BoundedMapConfig{
		MaxSize:         10000,
		TTL:             30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
		EnableMetrics:   true,
		MetricsPrefix:   "bounded_map",
	}
}

// BoundedMapOptions provides a fluent interface for configuring bounded maps
type BoundedMapOptions struct {
	config BoundedMapConfig
}

// NewBoundedMapOptions creates a new options builder
func NewBoundedMapOptions() *BoundedMapOptions {
	return &BoundedMapOptions{
		config: DefaultBoundedMapConfig(),
	}
}

// WithMaxSize sets the maximum size
func (o *BoundedMapOptions) WithMaxSize(size int) *BoundedMapOptions {
	o.config.MaxSize = size
	return o
}

// WithTTL sets the time-to-live
func (o *BoundedMapOptions) WithTTL(ttl time.Duration) *BoundedMapOptions {
	o.config.TTL = ttl
	return o
}

// WithCleanupInterval sets the cleanup interval
func (o *BoundedMapOptions) WithCleanupInterval(interval time.Duration) *BoundedMapOptions {
	o.config.CleanupInterval = interval
	return o
}

// WithEvictionCallback sets the eviction callback
func (o *BoundedMapOptions) WithEvictionCallback(callback func(key, value interface{}, reason EvictionReason)) *BoundedMapOptions {
	o.config.EvictionCallback = callback
	return o
}

// WithMetrics enables or disables metrics
func (o *BoundedMapOptions) WithMetrics(enabled bool) *BoundedMapOptions {
	o.config.EnableMetrics = enabled
	return o
}

// WithMetricsPrefix sets the metrics prefix
func (o *BoundedMapOptions) WithMetricsPrefix(prefix string) *BoundedMapOptions {
	o.config.MetricsPrefix = prefix
	return o
}

// Build creates a new bounded map with the configured options
func (o *BoundedMapOptions) Build() *BoundedMap {
	return NewBoundedMap(o.config)
}