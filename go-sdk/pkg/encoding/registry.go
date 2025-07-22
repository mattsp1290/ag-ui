package encoding

import (
	"container/list"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/errors"
)

// RegistryEntry wraps registry data with metadata for cleanup
type RegistryEntry struct {
	value       interface{}
	createdAt   time.Time
	lastAccess  time.Time
	accessCount int64
}

// RegistryConfig holds configuration for registry cleanup behavior
type RegistryConfig struct {
	// MaxEntries limits the total number of entries per map (0 = unlimited)
	MaxEntries int
	// TTL is the time-to-live for entries (0 = no TTL)
	TTL time.Duration
	// CleanupInterval is how often to run background cleanup
	CleanupInterval time.Duration
	// EnableLRU enables LRU eviction when max entries is reached
	EnableLRU bool
	// EnableBackgroundCleanup enables automatic TTL-based cleanup
	EnableBackgroundCleanup bool
}

// DefaultRegistryConfig returns sensible defaults for registry cleanup
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		MaxEntries:              1000,                // Limit to 1000 entries per map
		TTL:                     1 * time.Hour,       // 1 hour TTL
		CleanupInterval:         10 * time.Minute,    // Cleanup every 10 minutes
		EnableLRU:               true,
		EnableBackgroundCleanup: true,
	}
}

// RegistryEntryType represents the type of registry entry for composite keys
type RegistryEntryType int

const (
	EntryTypeFormat RegistryEntryType = iota
	EntryTypeEncoderFactory
	EntryTypeDecoderFactory
	EntryTypeCodecFactory
	EntryTypeLegacyEncoderFactory
	EntryTypeLegacyDecoderFactory
	EntryTypeLegacyCodecFactory
	EntryTypeAlias
)

// registryKey represents a composite key for the sync.Map
type registryKey struct {
	entryType RegistryEntryType
	mimeType  string
}

// String returns a string representation of the key for debugging
func (k registryKey) String() string {
	return fmt.Sprintf("%d:%s", k.entryType, k.mimeType)
}

// FormatRegistry manages encoders, decoders, and format information with cleanup capabilities
// This is now implemented using sync.Map with composite keys and composed of focused components
type FormatRegistry struct {
	// Core data storage
	entries sync.Map // map[registryKey]*RegistryEntry

	// Focused component responsibilities
	cacheManager  *RegistryCacheManager
	priorities    *RegistryPriorityManager
	lifecycle     *RegistryLifecycleManager
	metrics       *RegistryMetrics

	// Configuration
	config    *RegistryConfig
	validator FormatValidator
	closed    int32 // atomic flag for graceful shutdown
}

// FormatValidator interface for validation framework integration
type FormatValidator interface {
	ValidateFormat(mimeType string, data []byte) error
	ValidateEncoding(mimeType string, data []byte) error
	ValidateDecoding(mimeType string, data []byte) error
}

// ==============================================================================
// FOCUSED COMPONENTS - Registry Architecture Cleanup
// ==============================================================================

// RegistryCacheManager handles LRU caching and memory management
type RegistryCacheManager struct {
	lruMu    sync.RWMutex
	lruList  *list.List
	lruIndex map[registryKey]*list.Element
	config   *RegistryConfig
}

// Clear clears all LRU cache entries
func (c *RegistryCacheManager) Clear() {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()
	c.lruList = list.New()
	c.lruIndex = make(map[registryKey]*list.Element)
}

// Size returns the number of items in the LRU cache
func (c *RegistryCacheManager) Size() int {
	c.lruMu.RLock()
	defer c.lruMu.RUnlock()
	return c.lruList.Len()
}

// EvictOldest evicts the oldest entry from the LRU cache and returns the key
func (c *RegistryCacheManager) EvictOldest() (registryKey, bool) {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()
	
	if c.lruList.Len() == 0 {
		return registryKey{}, false
	}
	
	// Remove the oldest entry (back of the list)
	oldest := c.lruList.Back()
	if oldest != nil {
		key := oldest.Value.(registryKey)
		delete(c.lruIndex, key)
		c.lruList.Remove(oldest)
		return key, true
	}
	return registryKey{}, false
}

// NewRegistryCacheManager creates a new cache manager
func NewRegistryCacheManager(config *RegistryConfig) *RegistryCacheManager {
	return &RegistryCacheManager{
		lruList:  list.New(),
		lruIndex: make(map[registryKey]*list.Element),
		config:   config,
	}
}

// UpdateLRUPosition moves an entry to the front of the LRU list
func (cm *RegistryCacheManager) UpdateLRUPosition(key registryKey) {
	cm.lruMu.Lock()
	defer cm.lruMu.Unlock()

	if elem, exists := cm.lruIndex[key]; exists {
		cm.lruList.MoveToFront(elem)
	}
}

// AddToLRU adds a new entry to the front of the LRU list
func (cm *RegistryCacheManager) AddToLRU(key registryKey) {
	cm.lruMu.Lock()
	defer cm.lruMu.Unlock()

	elem := cm.lruList.PushFront(key)
	cm.lruIndex[key] = elem
}

// RemoveFromLRU removes an entry from the LRU list
func (cm *RegistryCacheManager) RemoveFromLRU(key registryKey) {
	cm.lruMu.Lock()
	defer cm.lruMu.Unlock()

	if elem, exists := cm.lruIndex[key]; exists {
		cm.lruList.Remove(elem)
		delete(cm.lruIndex, key)
	}
}

// GetLRUCandidate returns the least recently used entry for eviction
func (cm *RegistryCacheManager) GetLRUCandidate() *registryKey {
	cm.lruMu.Lock()
	defer cm.lruMu.Unlock()

	if cm.lruList.Len() == 0 {
		return nil
	}

	elem := cm.lruList.Back()
	if elem == nil {
		return nil
	}

	key := elem.Value.(registryKey)
	return &key
}

// RegistryPriorityManager handles format priorities and selection
type RegistryPriorityManager struct {
	priorityMu    sync.RWMutex
	priorities    []string
	defaultFormat string
}

// NewRegistryPriorityManager creates a new priority manager
func NewRegistryPriorityManager(defaultFormat string) *RegistryPriorityManager {
	return &RegistryPriorityManager{
		priorities:    []string{},
		defaultFormat: defaultFormat,
	}
}

// GetDefaultFormat returns the default format
func (pm *RegistryPriorityManager) GetDefaultFormat() string {
	pm.priorityMu.RLock()
	defer pm.priorityMu.RUnlock()
	return pm.defaultFormat
}

// SetDefaultFormat sets the default format
func (pm *RegistryPriorityManager) SetDefaultFormat(format string) {
	pm.priorityMu.Lock()
	defer pm.priorityMu.Unlock()
	pm.defaultFormat = format
}

// UpdatePriorities updates the priority order based on format info
func (pm *RegistryPriorityManager) UpdatePriorities(formatMap map[string]*FormatInfo) {
	var priorities []string

	for mimeType := range formatMap {
		priorities = append(priorities, mimeType)
	}

	// Sort by priority value (lower is higher priority)
	sort.Slice(priorities, func(i, j int) bool {
		pi := formatMap[priorities[i]].Priority
		pj := formatMap[priorities[j]].Priority
		if pi == pj {
			// Secondary sort by name for stability
			return priorities[i] < priorities[j]
		}
		return pi < pj
	})

	pm.priorityMu.Lock()
	pm.priorities = priorities
	pm.priorityMu.Unlock()
}

// GetPriorityMap returns priority mapping
func (pm *RegistryPriorityManager) GetPriorityMap() (map[string]int, int) {
	pm.priorityMu.RLock()
	defer pm.priorityMu.RUnlock()

	priorityMap := make(map[string]int)
	for i, mimeType := range pm.priorities {
		priorityMap[mimeType] = i
	}
	return priorityMap, len(pm.priorities)
}

// RegistryLifecycleManager handles cleanup and resource management
type RegistryLifecycleManager struct {
	config      *RegistryConfig
	cleanupStop chan struct{}
	cleanupOnce sync.Once
	closed      int32
}

// NewRegistryLifecycleManager creates a new lifecycle manager
func NewRegistryLifecycleManager(config *RegistryConfig) *RegistryLifecycleManager {
	return &RegistryLifecycleManager{
		config:      config,
		cleanupStop: make(chan struct{}),
	}
}

// StartBackgroundCleanup starts background cleanup process
func (lm *RegistryLifecycleManager) StartBackgroundCleanup(cleanupCallback func()) {
	go lm.backgroundCleanup(cleanupCallback)
}

// backgroundCleanup runs periodic cleanup
func (lm *RegistryLifecycleManager) backgroundCleanup(cleanupCallback func()) {
	ticker := time.NewTicker(lm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		// Check if we're closed before entering blocking select
		if atomic.LoadInt32(&lm.closed) != 0 {
			return
		}
		
		select {
		case <-ticker.C:
			// Double-check if we're closed before running cleanup
			if atomic.LoadInt32(&lm.closed) != 0 {
				return
			}
			cleanupCallback()
		case <-lm.cleanupStop:
			return
		}
	}
}

// Close stops background cleanup
func (lm *RegistryLifecycleManager) Close() error {
	if !atomic.CompareAndSwapInt32(&lm.closed, 0, 1) {
		return nil // Already closed
	}

	// Close the stop channel to signal the goroutine to exit
	lm.cleanupOnce.Do(func() {
		close(lm.cleanupStop)
	})

	return nil
}

// IsClosed returns whether the lifecycle is closed
func (lm *RegistryLifecycleManager) IsClosed() bool {
	return atomic.LoadInt32(&lm.closed) != 0
}

// RegistryMetrics handles statistics and monitoring
type RegistryMetrics struct {
	entryCount int64 // atomic counter for better memory pressure detection
}

// NewRegistryMetrics creates a new metrics manager
func NewRegistryMetrics() *RegistryMetrics {
	return &RegistryMetrics{}
}

// IncrementEntryCount atomically increments the entry count
func (m *RegistryMetrics) IncrementEntryCount() {
	atomic.AddInt64(&m.entryCount, 1)
}

// DecrementEntryCount atomically decrements the entry count
func (m *RegistryMetrics) DecrementEntryCount() {
	atomic.AddInt64(&m.entryCount, -1)
}

// GetEntryCount returns the current entry count
func (m *RegistryMetrics) GetEntryCount() int64 {
	return atomic.LoadInt64(&m.entryCount)
}

// ResetEntryCount resets the entry count to zero
func (m *RegistryMetrics) ResetEntryCount() {
	atomic.StoreInt64(&m.entryCount, 0)
}

// Reset resets all metrics
func (m *RegistryMetrics) Reset() {
	atomic.StoreInt64(&m.entryCount, 0)
}

var (
	// Global registry instance
	globalRegistry *FormatRegistry
	globalOnce     sync.Once
	globalMutex    sync.RWMutex
	// Track registration attempts to provide better error handling
	globalRegistrationErrors []error
)

// GetGlobalRegistry returns the global format registry with built-in codecs registered
func GetGlobalRegistry() *FormatRegistry {
	globalMutex.RLock()
	if globalRegistry != nil {
		defer globalMutex.RUnlock()
		return globalRegistry
	}
	globalMutex.RUnlock()
	
	// Need to create registry, use write lock
	globalMutex.Lock()
	defer globalMutex.Unlock()
	
	// Double-check after acquiring write lock
	if globalRegistry != nil {
		return globalRegistry
	}
	
	globalRegistry = NewFormatRegistry()
	if err := globalRegistry.RegisterDefaults(); err != nil {
		globalRegistrationErrors = append(globalRegistrationErrors, err)
	}

	// The codec registration happens automatically via init() functions
	// when the json/protobuf packages are imported. No additional action needed here.
	return globalRegistry
}

// NewFormatRegistry creates a new format registry with default cleanup configuration
func NewFormatRegistry() *FormatRegistry {
	return NewFormatRegistryWithConfig(DefaultRegistryConfig())
}

// NewFormatRegistryWithConfig creates a new format registry with custom configuration
// Now uses focused component composition to prevent memory leaks and reduce complexity
func NewFormatRegistryWithConfig(config *RegistryConfig) *FormatRegistry {
	if config == nil {
		config = DefaultRegistryConfig()
	}

	r := &FormatRegistry{
		cacheManager:  NewRegistryCacheManager(config),
		priorities:    NewRegistryPriorityManager("application/json"), // default format
		lifecycle:     NewRegistryLifecycleManager(config),
		metrics:       NewRegistryMetrics(),
		config:        config,
	}

	// Start background cleanup if enabled
	if config.EnableBackgroundCleanup && config.CleanupInterval > 0 {
		r.lifecycle.StartBackgroundCleanup(r.cleanupCallback)
	}

	return r
}

// cleanupCallback is used by the lifecycle manager
func (r *FormatRegistry) cleanupCallback() {
	r.CleanupExpired()

	// If we're under memory pressure, do more aggressive cleanup
	currentCount := r.metrics.GetEntryCount()
	maxEntries := int64(r.config.MaxEntries)

	if maxEntries > 0 && currentCount > (maxEntries*8)/10 {
		r.CleanupByAccessTime(r.config.TTL / 2) // More aggressive cleanup
	}
}

// Helper methods for entry management

// getEntry safely retrieves an entry from the sync.Map
func (r *FormatRegistry) getEntry(entryType RegistryEntryType, mimeType string) (*RegistryEntry, bool) {
	if r.lifecycle.IsClosed() {
		return nil, false
	}

	// Normalize the MIME type to lowercase for consistent access
	normalizedMimeType := strings.ToLower(mimeType)
	key := registryKey{entryType: entryType, mimeType: normalizedMimeType}
	if value, ok := r.entries.Load(key); ok {
		entry := value.(*RegistryEntry)
		// Update access tracking atomically
		entry.lastAccess = time.Now()
		atomic.AddInt64(&entry.accessCount, 1)

		// Update LRU position if enabled
		if r.config.EnableLRU {
			r.cacheManager.UpdateLRUPosition(key)
		}

		return entry, true
	}
	return nil, false
}

// setEntry safely stores an entry in the sync.Map with size limits and LRU eviction
func (r *FormatRegistry) setEntry(entryType RegistryEntryType, mimeType string, value interface{}) error {
	if r.lifecycle.IsClosed() {
		return NewOperationError("register", "registry", "registry is closed", nil)
	}

	// Normalize the MIME type to lowercase for consistent storage
	normalizedMimeType := strings.ToLower(mimeType)
	key := registryKey{entryType: entryType, mimeType: normalizedMimeType}

	// Check if we need to evict before adding (if max entries configured)
	// Only count format entries towards the limit, not aliases or factories
	if r.config.MaxEntries > 0 && entryType == EntryTypeFormat {
		currentFormatCount := r.countFormatEntries()
		if currentFormatCount >= r.config.MaxEntries {
			r.evictLRUFormatEntry()
		}
	}

	// Create new entry with metadata
	entry := &RegistryEntry{
		value:       value,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}

	// Store in sync.Map
	r.entries.Store(key, entry)
	r.metrics.IncrementEntryCount()

	// Update LRU tracking if enabled
	if r.config.EnableLRU {
		r.cacheManager.AddToLRU(key)
	}

	return nil
}

// deleteEntry safely removes an entry from the sync.Map
func (r *FormatRegistry) deleteEntry(entryType RegistryEntryType, mimeType string) bool {
	// Normalize the MIME type to lowercase for consistent access
	normalizedMimeType := strings.ToLower(mimeType)
	key := registryKey{entryType: entryType, mimeType: normalizedMimeType}

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

// LRU management methods





// evictLRUFormatEntry removes the least recently used format entry using cache manager
func (r *FormatRegistry) evictLRUFormatEntry() {
	// Find the least recently used format entry (not just any entry)
	var oldestFormatKey *registryKey
	
	r.cacheManager.lruMu.Lock()
	// Walk from back (oldest) to front to find the oldest format entry
	for elem := r.cacheManager.lruList.Back(); elem != nil; elem = elem.Prev() {
		key := elem.Value.(registryKey)
		if key.entryType == EntryTypeFormat {
			oldestFormatKey = &key
			break
		}
	}
	r.cacheManager.lruMu.Unlock()
	
	if oldestFormatKey == nil {
		return // No format entries to evict
	}
	
	// Get format info before removal to access aliases
	if entry, exists := r.getEntry(EntryTypeFormat, oldestFormatKey.mimeType); exists {
		info := entry.value.(*FormatInfo)
		
		// Remove aliases first
		for _, alias := range info.Aliases {
			if r.deleteEntry(EntryTypeAlias, alias) {
				// Also remove alias from LRU tracking
				aliasKey := registryKey{entryType: EntryTypeAlias, mimeType: strings.ToLower(alias)}
				r.cacheManager.RemoveFromLRU(aliasKey)
			}
		}
	}
	
	// Remove all related entries for this format
	mimeType := oldestFormatKey.mimeType
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
		if r.deleteEntry(entryType, mimeType) {
			// Also remove from LRU tracking
			key := registryKey{entryType: entryType, mimeType: mimeType}
			r.cacheManager.RemoveFromLRU(key)
		}
	}
}

// countFormatEntries counts only format entries
func (r *FormatRegistry) countFormatEntries() int {
	count := 0
	r.entries.Range(func(key, value interface{}) bool {
		k := key.(registryKey)
		if k.entryType == EntryTypeFormat {
			count++
		}
		return true
	})
	return count
}

// RegisterFormat registers format information
func (r *FormatRegistry) RegisterFormat(info *FormatInfo) error {
	if info == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "format info cannot be nil").WithOperation("register_format")
	}

	if info.MIMEType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_format")
	}

	// Register the format
	if err := r.setEntry(EntryTypeFormat, info.MIMEType, info); err != nil {
		return err
	}

	// Register aliases
	for _, alias := range info.Aliases {
		if err := r.setEntry(EntryTypeAlias, alias, info.MIMEType); err != nil {
			// Continue with other aliases even if one fails
			continue
		}
	}

	// Update priorities
	r.updatePriorities()

	return nil
}

// RegisterEncoderFactory registers a concrete encoder factory (preferred method)
func (r *FormatRegistry) RegisterEncoderFactory(mimeType string, factory *DefaultEncoderFactory) error {
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_encoder_factory")
	}

	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "encoder factory cannot be nil").WithOperation("register_encoder_factory")
	}

	// Register in concrete factory map
	if err := r.setEntry(EntryTypeEncoderFactory, mimeType, factory); err != nil {
		return err
	}

	// Also register in legacy map for backward compatibility
	return r.setEntry(EntryTypeLegacyEncoderFactory, mimeType, factory)
}

// RegisterDecoderFactory registers a concrete decoder factory (preferred method)
func (r *FormatRegistry) RegisterDecoderFactory(mimeType string, factory *DefaultDecoderFactory) error {
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_decoder_factory")
	}

	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "decoder factory cannot be nil").WithOperation("register_decoder_factory")
	}

	// Register in concrete factory map
	if err := r.setEntry(EntryTypeDecoderFactory, mimeType, factory); err != nil {
		return err
	}

	// Also register in legacy map for backward compatibility
	return r.setEntry(EntryTypeLegacyDecoderFactory, mimeType, factory)
}

// RegisterFullCodecFactory registers a full codec factory supporting both basic and streaming operations
func (r *FormatRegistry) RegisterFullCodecFactory(mimeType string, factory FullCodecFactory) error {
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_full_codec_factory")
	}

	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "codec factory cannot be nil").WithOperation("register_full_codec_factory")
	}

	// Register both basic and streaming factory capabilities
	if err := r.setEntry(EntryTypeCodecFactory, mimeType, factory); err != nil {
		return err
	}

	// Create backward compatibility adapter for the legacy CodecFactory interface
	legacyAdapter := NewLegacyCodecFactory(factory)
	if err := r.setEntry(EntryTypeLegacyCodecFactory, mimeType, legacyAdapter); err != nil {
		return err
	}

	// Create encoder/decoder factory adapters for backward compatibility
	encoderFactory := &DefaultEncoderFactory{}
	if defaultFactory, ok := factory.(*DefaultCodecFactory); ok {
		encoderFactory.DefaultCodecFactory = defaultFactory
	}
	decoderFactory := &DefaultDecoderFactory{}
	if defaultFactory, ok := factory.(*DefaultCodecFactory); ok {
		decoderFactory.DefaultCodecFactory = defaultFactory
	}

	r.setEntry(EntryTypeEncoderFactory, mimeType, encoderFactory)
	r.setEntry(EntryTypeDecoderFactory, mimeType, decoderFactory)
	r.setEntry(EntryTypeLegacyEncoderFactory, mimeType, encoderFactory)
	r.setEntry(EntryTypeLegacyDecoderFactory, mimeType, decoderFactory)

	return nil
}

// RegisterCodecFactory registers a concrete codec factory (preferred method)
func (r *FormatRegistry) RegisterCodecFactory(mimeType string, factory *DefaultCodecFactory) error {
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_codec_factory")
	}

	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "codec factory cannot be nil").WithOperation("register_codec_factory")
	}

	// Register the codec factory
	if err := r.setEntry(EntryTypeCodecFactory, mimeType, factory); err != nil {
		return err
	}

	// Create backward compatibility factories
	encoderFactory := &DefaultEncoderFactory{DefaultCodecFactory: factory}
	decoderFactory := &DefaultDecoderFactory{DefaultCodecFactory: factory}

	// Register encoder and decoder factories
	r.setEntry(EntryTypeEncoderFactory, mimeType, encoderFactory)
	r.setEntry(EntryTypeDecoderFactory, mimeType, decoderFactory)

	// Register legacy factories
	r.setEntry(EntryTypeLegacyCodecFactory, mimeType, factory)
	r.setEntry(EntryTypeLegacyEncoderFactory, mimeType, encoderFactory)
	r.setEntry(EntryTypeLegacyDecoderFactory, mimeType, decoderFactory)

	return nil
}

// Legacy registration methods for backward compatibility

// RegisterEncoder registers an encoder factory (legacy method - accepts interface)
func (r *FormatRegistry) RegisterEncoder(mimeType string, factory EncoderFactory) error {
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_encoder")
	}

	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "encoder factory cannot be nil").WithOperation("register_encoder")
	}

	// Register the legacy factory
	if err := r.setEntry(EntryTypeLegacyEncoderFactory, mimeType, factory); err != nil {
		return err
	}

	// If it's a concrete type, also register it in the concrete map
	if concreteFactory, ok := factory.(*DefaultEncoderFactory); ok {
		r.setEntry(EntryTypeEncoderFactory, mimeType, concreteFactory)
	}

	return nil
}

// RegisterDecoder registers a decoder factory (legacy method - accepts interface)
func (r *FormatRegistry) RegisterDecoder(mimeType string, factory DecoderFactory) error {
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_decoder")
	}

	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "decoder factory cannot be nil").WithOperation("register_decoder")
	}

	// Register the legacy factory
	if err := r.setEntry(EntryTypeLegacyDecoderFactory, mimeType, factory); err != nil {
		return err
	}

	// If it's a concrete type, also register it in the concrete map
	if concreteFactory, ok := factory.(*DefaultDecoderFactory); ok {
		r.setEntry(EntryTypeDecoderFactory, mimeType, concreteFactory)
	}

	return nil
}

// RegisterCodec registers a codec factory (legacy method - accepts interface)
func (r *FormatRegistry) RegisterCodec(mimeType string, factory CodecFactory) error {
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_codec")
	}

	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "codec factory cannot be nil").WithOperation("register_codec")
	}

	// Register the legacy factory
	if err := r.setEntry(EntryTypeLegacyCodecFactory, mimeType, factory); err != nil {
		return err
	}

	// If it's a concrete type, also register it in concrete maps and create compatibility factories
	if concreteFactory, ok := factory.(*DefaultCodecFactory); ok {
		r.setEntry(EntryTypeCodecFactory, mimeType, concreteFactory)

		// Create backward compatibility factories
		encoderFactory := &DefaultEncoderFactory{DefaultCodecFactory: concreteFactory}
		decoderFactory := &DefaultDecoderFactory{DefaultCodecFactory: concreteFactory}

		r.setEntry(EntryTypeEncoderFactory, mimeType, encoderFactory)
		r.setEntry(EntryTypeDecoderFactory, mimeType, decoderFactory)
		r.setEntry(EntryTypeLegacyEncoderFactory, mimeType, encoderFactory)
		r.setEntry(EntryTypeLegacyDecoderFactory, mimeType, decoderFactory)
	} else {
		// For non-concrete types that implement both EncoderFactory and DecoderFactory,
		// register them in the legacy encoder/decoder maps as well
		if encoderFactory, ok := factory.(EncoderFactory); ok {
			r.setEntry(EntryTypeLegacyEncoderFactory, mimeType, encoderFactory)
		}
		if decoderFactory, ok := factory.(DecoderFactory); ok {
			r.setEntry(EntryTypeLegacyDecoderFactory, mimeType, decoderFactory)
		}
	}

	return nil
}

// UnregisterFormat removes a format from the registry
func (r *FormatRegistry) UnregisterFormat(mimeType string) error {
	canonical := r.resolveAlias(mimeType)

	// Check if format exists
	entry, exists := r.getEntry(EntryTypeFormat, canonical)
	if !exists {
		return errors.NewEncodingError(errors.CodeFormatNotRegistered, "format not registered").WithMimeType(mimeType).WithOperation("unregister_format")
	}

	// Get format info to find aliases
	info := entry.value.(*FormatInfo)

	// Remove aliases
	for _, alias := range info.Aliases {
		r.deleteEntry(EntryTypeAlias, alias)
	}

	// Remove all entries for this MIME type
	r.deleteEntry(EntryTypeFormat, canonical)
	r.deleteEntry(EntryTypeEncoderFactory, canonical)
	r.deleteEntry(EntryTypeDecoderFactory, canonical)
	r.deleteEntry(EntryTypeCodecFactory, canonical)
	r.deleteEntry(EntryTypeLegacyEncoderFactory, canonical)
	r.deleteEntry(EntryTypeLegacyDecoderFactory, canonical)
	r.deleteEntry(EntryTypeLegacyCodecFactory, canonical)

	// Update priorities
	r.updatePriorities()

	return nil
}

// Getter methods

// GetFormat returns format information for a MIME type
func (r *FormatRegistry) GetFormat(mimeType string) (*FormatInfo, error) {
	canonical := r.resolveAlias(mimeType)
	if entry, exists := r.getEntry(EntryTypeFormat, canonical); exists {
		return entry.value.(*FormatInfo), nil
	}
	return nil, NewRegistryError("format_registry", "lookup", mimeType, "format not registered", nil)
}

// GetEncoder creates an encoder for the specified MIME type
func (r *FormatRegistry) GetEncoder(ctx context.Context, mimeType string, options *EncodingOptions) (Encoder, error) {
	canonical := r.resolveAlias(mimeType)

	// Try concrete factory first
	if entry, exists := r.getEntry(EntryTypeEncoderFactory, canonical); exists {
		factory := entry.value.(*DefaultEncoderFactory)
		return factory.CreateEncoder(ctx, canonical, options)
	}

	// Fall back to codec factory
	if entry, exists := r.getEntry(EntryTypeCodecFactory, canonical); exists {
		factory := entry.value.(*DefaultCodecFactory)
		return factory.CreateCodec(ctx, canonical, options, nil)
	}

	// Fall back to legacy encoder factories
	if entry, exists := r.getEntry(EntryTypeLegacyEncoderFactory, canonical); exists {
		factory := entry.value.(EncoderFactory)
		return factory.CreateEncoder(ctx, canonical, options)
	}

	// Fall back to legacy codec factories
	if entry, exists := r.getEntry(EntryTypeLegacyCodecFactory, canonical); exists {
		factory := entry.value.(CodecFactory)
		return factory.CreateCodec(ctx, canonical, options, nil)
	}

	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no encoder registered for format").WithMimeType(mimeType).WithOperation("get_encoder")
}

// GetDecoder creates a decoder for the specified MIME type
func (r *FormatRegistry) GetDecoder(ctx context.Context, mimeType string, options *DecodingOptions) (Decoder, error) {
	canonical := r.resolveAlias(mimeType)

	// Try concrete factory first
	if entry, exists := r.getEntry(EntryTypeDecoderFactory, canonical); exists {
		factory := entry.value.(*DefaultDecoderFactory)
		return factory.CreateDecoder(ctx, canonical, options)
	}

	// Fall back to codec factory
	if entry, exists := r.getEntry(EntryTypeCodecFactory, canonical); exists {
		factory := entry.value.(*DefaultCodecFactory)
		return factory.CreateCodec(ctx, canonical, nil, options)
	}

	// Fall back to legacy decoder factories
	if entry, exists := r.getEntry(EntryTypeLegacyDecoderFactory, canonical); exists {
		factory := entry.value.(DecoderFactory)
		return factory.CreateDecoder(ctx, canonical, options)
	}

	// Fall back to legacy codec factories
	if entry, exists := r.getEntry(EntryTypeLegacyCodecFactory, canonical); exists {
		factory := entry.value.(CodecFactory)
		return factory.CreateCodec(ctx, canonical, nil, options)
	}

	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no decoder registered for format").WithMimeType(mimeType).WithOperation("get_decoder")
}

// GetFocusedCodec creates a codec using the new focused interface approach
func (r *FormatRegistry) GetFocusedCodec(ctx context.Context, mimeType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	canonical := r.resolveAlias(mimeType)

	// Try full codec factory first (supports both CodecFactory and StreamCodecFactory)
	if entry, exists := r.getEntry(EntryTypeCodecFactory, canonical); exists {
		factory := entry.value.(CodecFactory)
		return factory.CreateCodec(ctx, canonical, encOptions, decOptions)
	}

	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no codec registered for format").WithMimeType(mimeType).WithOperation("get_focused_codec")
}

// GetStreamCodec creates a streaming codec using the new focused interface approach
func (r *FormatRegistry) GetFocusedStreamCodec(ctx context.Context, mimeType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (StreamCodec, error) {
	canonical := r.resolveAlias(mimeType)

	// Try stream codec factory
	if entry, exists := r.getEntry(EntryTypeCodecFactory, canonical); exists {
		factory := entry.value
		if streamFactory, ok := factory.(StreamCodecFactory); ok {
			return streamFactory.CreateStreamCodec(ctx, canonical, encOptions, decOptions)
		}
	}

	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no stream codec registered for format").WithMimeType(mimeType).WithOperation("get_focused_stream_codec")
}

// GetCodec creates a codec for the specified MIME type (backward compatible method)
func (r *FormatRegistry) GetCodec(ctx context.Context, mimeType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	canonical := r.resolveAlias(mimeType)

	// Try focused approach first
	if codec, err := r.GetFocusedCodec(ctx, mimeType, encOptions, decOptions); err == nil {
		return codec, nil
	}

	// Try concrete codec factory
	if entry, exists := r.getEntry(EntryTypeCodecFactory, canonical); exists {
		factory := entry.value.(*DefaultCodecFactory)
		return factory.CreateCodec(ctx, canonical, encOptions, decOptions)
	}

	// Try legacy codec factory
	if entry, exists := r.getEntry(EntryTypeLegacyCodecFactory, canonical); exists {
		factory := entry.value.(CodecFactory)
		return factory.CreateCodec(ctx, canonical, encOptions, decOptions)
	}

	// Check if we have separate encoder/decoder factories
	var hasEncoder, hasDecoder bool

	// Check concrete factories first
	if _, exists := r.getEntry(EntryTypeEncoderFactory, canonical); exists {
		hasEncoder = true
	} else if _, exists := r.getEntry(EntryTypeLegacyEncoderFactory, canonical); exists {
		hasEncoder = true
	}

	if _, exists := r.getEntry(EntryTypeDecoderFactory, canonical); exists {
		hasDecoder = true
	} else if _, exists := r.getEntry(EntryTypeLegacyDecoderFactory, canonical); exists {
		hasDecoder = true
	}

	if !hasEncoder || !hasDecoder {
		return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no codec registered for format").WithMimeType(mimeType).WithOperation("get_codec")
	}

	// Fall back to separate encoder/decoder
	encoder, err := r.GetEncoder(ctx, mimeType, encOptions)
	if err != nil {
		return nil, err
	}

	decoder, err := r.GetDecoder(ctx, mimeType, decOptions)
	if err != nil {
		return nil, err
	}

	// Create adapters that implement the required interfaces
	encoderAdapter := &registryEncoderAdapter{encoder: encoder}
	decoderAdapter := &registryDecoderAdapter{decoder: decoder}
	
	return &compositeCodec{
		encoder: encoderAdapter,
		decoder: decoderAdapter,
	}, nil
}

// Factory getters

// GetEncoderFactory returns the concrete encoder factory for the specified MIME type
func (r *FormatRegistry) GetEncoderFactory(mimeType string) (*DefaultEncoderFactory, error) {
	canonical := r.resolveAlias(mimeType)
	if entry, exists := r.getEntry(EntryTypeEncoderFactory, canonical); exists {
		return entry.value.(*DefaultEncoderFactory), nil
	}
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete encoder factory registered for format").WithMimeType(mimeType).WithOperation("get_encoder_factory")
}

// GetDecoderFactory returns the concrete decoder factory for the specified MIME type
func (r *FormatRegistry) GetDecoderFactory(mimeType string) (*DefaultDecoderFactory, error) {
	canonical := r.resolveAlias(mimeType)
	if entry, exists := r.getEntry(EntryTypeDecoderFactory, canonical); exists {
		return entry.value.(*DefaultDecoderFactory), nil
	}
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete decoder factory registered for format").WithMimeType(mimeType).WithOperation("get_decoder_factory")
}

// GetCodecFactory returns the concrete codec factory for the specified MIME type
func (r *FormatRegistry) GetCodecFactory(mimeType string) (*DefaultCodecFactory, error) {
	canonical := r.resolveAlias(mimeType)
	if entry, exists := r.getEntry(EntryTypeCodecFactory, canonical); exists {
		return entry.value.(*DefaultCodecFactory), nil
	}
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete codec factory registered for format").WithMimeType(mimeType).WithOperation("get_codec_factory")
}

// Stream encoder/decoder methods

// GetStreamEncoder creates a streaming encoder for the specified MIME type
func (r *FormatRegistry) GetStreamEncoder(ctx context.Context, mimeType string, options *EncodingOptions) (StreamEncoder, error) {
	canonical := r.resolveAlias(mimeType)

	// Try concrete factory first
	if entry, exists := r.getEntry(EntryTypeEncoderFactory, canonical); exists {
		factory := entry.value.(*DefaultEncoderFactory)
		return factory.CreateStreamEncoder(ctx, canonical, options)
	}

	// Fall back to legacy interface
	if entry, exists := r.getEntry(EntryTypeLegacyEncoderFactory, canonical); exists {
		factory := entry.value.(EncoderFactory)
		return factory.CreateStreamEncoder(ctx, canonical, options)
	}

	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no encoder registered for format").WithMimeType(mimeType).WithOperation("get_stream_encoder")
}

// GetStreamDecoder creates a streaming decoder for the specified MIME type
func (r *FormatRegistry) GetStreamDecoder(ctx context.Context, mimeType string, options *DecodingOptions) (StreamDecoder, error) {
	canonical := r.resolveAlias(mimeType)

	// Try concrete factory first
	if entry, exists := r.getEntry(EntryTypeDecoderFactory, canonical); exists {
		factory := entry.value.(*DefaultDecoderFactory)
		return factory.CreateStreamDecoder(ctx, canonical, options)
	}

	// Fall back to legacy interface
	if entry, exists := r.getEntry(EntryTypeLegacyDecoderFactory, canonical); exists {
		factory := entry.value.(DecoderFactory)
		return factory.CreateStreamDecoder(ctx, canonical, options)
	}

	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no decoder registered for format").WithMimeType(mimeType).WithOperation("get_stream_decoder")
}

// Query methods

// ListFormats returns all registered formats
func (r *FormatRegistry) ListFormats() []*FormatInfo {
	var formats []*FormatInfo
	var priorities map[string]int

	// Collect all formats
	r.entries.Range(func(key, value interface{}) bool {
		k := key.(registryKey)
		if k.entryType == EntryTypeFormat {
			entry := value.(*RegistryEntry)
			formats = append(formats, entry.value.(*FormatInfo))
		}
		return true
	})

	// Get priority mapping
	priorities, defaultPriority := r.priorities.GetPriorityMap()

	// Sort by priority
	sort.Slice(formats, func(i, j int) bool {
		priorityI, exists := priorities[formats[i].MIMEType]
		if !exists {
			priorityI = defaultPriority
		}
		priorityJ, exists := priorities[formats[j].MIMEType]
		if !exists {
			priorityJ = defaultPriority
		}
		return priorityI < priorityJ
	})

	return formats
}

// SupportsFormat checks if a format is supported
func (r *FormatRegistry) SupportsFormat(mimeType string) bool {
	canonical := r.resolveAlias(mimeType)
	_, exists := r.getEntry(EntryTypeFormat, canonical)
	return exists
}

// SupportsEncoding checks if encoding is supported for a format
func (r *FormatRegistry) SupportsEncoding(mimeType string) bool {
	canonical := r.resolveAlias(mimeType)

	// Check concrete factory first
	if _, exists := r.getEntry(EntryTypeEncoderFactory, canonical); exists {
		return true
	}

	// Check legacy factory
	if _, exists := r.getEntry(EntryTypeLegacyEncoderFactory, canonical); exists {
		return true
	}

	return false
}

// SupportsDecoding checks if decoding is supported for a format
func (r *FormatRegistry) SupportsDecoding(mimeType string) bool {
	canonical := r.resolveAlias(mimeType)

	// Check concrete factory first
	if _, exists := r.getEntry(EntryTypeDecoderFactory, canonical); exists {
		return true
	}

	// Check legacy factory
	if _, exists := r.getEntry(EntryTypeLegacyDecoderFactory, canonical); exists {
		return true
	}

	return false
}

// GetCapabilities returns the capabilities of a format
func (r *FormatRegistry) GetCapabilities(mimeType string) (*FormatCapabilities, error) {
	canonical := r.resolveAlias(mimeType)
	if entry, exists := r.getEntry(EntryTypeFormat, canonical); exists {
		info := entry.value.(*FormatInfo)
		return &info.Capabilities, nil
	}
	return nil, NewRegistryError("format_registry", "lookup", mimeType, "format not registered", nil)
}

// SelectFormat selects the best format based on priorities and capabilities
func (r *FormatRegistry) SelectFormat(acceptedFormats []string, requiredCapabilities *FormatCapabilities) (string, error) {
	// If no formats specified, use default
	if len(acceptedFormats) == 0 {
		defaultFormat := r.priorities.GetDefaultFormat()
		return defaultFormat, nil
	}

	// Find the first format that matches requirements
	for _, format := range acceptedFormats {
		canonical := r.resolveAlias(format)
		if entry, exists := r.getEntry(EntryTypeFormat, canonical); exists {
			info := entry.value.(*FormatInfo)

			// Check if capabilities match
			if requiredCapabilities != nil {
				if !r.matchesCapabilities(&info.Capabilities, requiredCapabilities) {
					continue
				}
			}

			return canonical, nil
		}
	}

	return "", errors.NewEncodingError(errors.CodeNoSuitableFormat, "no suitable format found").WithOperation("select_format")
}

// Configuration methods

// SetDefaultFormat sets the default format
func (r *FormatRegistry) SetDefaultFormat(mimeType string) error {
	canonical := r.resolveAlias(mimeType)
	if _, exists := r.getEntry(EntryTypeFormat, canonical); !exists {
		return errors.NewEncodingError(errors.CodeFormatNotRegistered, "format not registered").WithMimeType(mimeType).WithOperation("set_default_format")
	}

	r.priorities.SetDefaultFormat(canonical)
	return nil
}

// GetDefaultFormat returns the default format
func (r *FormatRegistry) GetDefaultFormat() string {
	return r.priorities.GetDefaultFormat()
}

// SetValidator sets the format validator
func (r *FormatRegistry) SetValidator(validator FormatValidator) {
	// Note: No mutex needed for simple assignment to interface field
	r.validator = validator
}

// Utility methods

// resolveAlias resolves an alias to canonical MIME type
func (r *FormatRegistry) resolveAlias(mimeType string) string {
	lowerMimeType := strings.ToLower(mimeType)

	// Check if it's an alias
	if entry, exists := r.getEntry(EntryTypeAlias, lowerMimeType); exists {
		return entry.value.(string)
	}

	// Also check without parameters (e.g., "application/json; charset=utf-8" -> "application/json")
	if idx := strings.Index(mimeType, ";"); idx > 0 {
		base := strings.TrimSpace(mimeType[:idx])
		lowerBase := strings.ToLower(base)
		if entry, exists := r.getEntry(EntryTypeAlias, lowerBase); exists {
			return entry.value.(string)
		}
		return lowerBase // Return lowercase version
	}

	return lowerMimeType // Return lowercase version
}

// updatePriorities updates the priority order based on format info
func (r *FormatRegistry) updatePriorities() {
	formatMap := make(map[string]*FormatInfo)

	// Collect all formats
	r.entries.Range(func(key, value interface{}) bool {
		k := key.(registryKey)
		if k.entryType == EntryTypeFormat {
			entry := value.(*RegistryEntry)
			info := entry.value.(*FormatInfo)
			formatMap[info.MIMEType] = info
		}
		return true
	})

	// Update priorities through the priority manager
	r.priorities.UpdatePriorities(formatMap)
}

// matchesCapabilities checks if format capabilities match requirements
func (r *FormatRegistry) matchesCapabilities(formatCaps, requiredCaps *FormatCapabilities) bool {
	if requiredCaps.Streaming && !formatCaps.Streaming {
		return false
	}

	if requiredCaps.Compression && !formatCaps.Compression {
		return false
	}

	if requiredCaps.SchemaValidation && !formatCaps.SchemaValidation {
		return false
	}

	if requiredCaps.BinaryEfficient && !formatCaps.BinaryEfficient {
		return false
	}

	if requiredCaps.HumanReadable && !formatCaps.HumanReadable {
		return false
	}

	if requiredCaps.SelfDescribing && !formatCaps.SelfDescribing {
		return false
	}

	if requiredCaps.Versionable && !formatCaps.Versionable {
		return false
	}

	return true
}

// RegisterDefaults registers default formats (JSON and Protobuf)
func (r *FormatRegistry) RegisterDefaults() error {
	var errors []error

	// Register JSON format info
	jsonInfo := JSONFormatInfo()
	if err := r.RegisterFormat(jsonInfo); err != nil {
		errors = append(errors, NewRegistryError("format_registry", "register", "application/json", "failed to register JSON format", err))
	}

	// Register Protobuf format info
	protobufInfo := ProtobufFormatInfo()
	if err := r.RegisterFormat(protobufInfo); err != nil {
		errors = append(errors, NewRegistryError("format_registry", "register", "application/protobuf", "failed to register Protobuf format", err))
	}

	// Return the first error if any occurred, but don't fail completely
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// EnsureRegistered checks that critical formats are registered
func (r *FormatRegistry) EnsureRegistered(mimeTypes ...string) error {
	var missing []string
	var notSupported []string

	for _, mimeType := range mimeTypes {
		if !r.SupportsFormat(mimeType) {
			missing = append(missing, mimeType)
		} else if !r.SupportsEncoding(mimeType) {
			notSupported = append(notSupported, mimeType)
		}
	}

	if len(missing) > 0 || len(notSupported) > 0 {
		var errorMsg strings.Builder
		if len(missing) > 0 {
			errorMsg.WriteString(fmt.Sprintf("Missing format registration for: %v. ", missing))
		}
		if len(notSupported) > 0 {
			errorMsg.WriteString(fmt.Sprintf("Missing codec registration for: %v. ", notSupported))
			errorMsg.WriteString("Import the appropriate codec packages: ")
			for _, mt := range notSupported {
				switch mt {
				case "application/json":
					errorMsg.WriteString(`import _ "github.com/ag-ui/go-sdk/pkg/encoding/json" `)
				case "application/x-protobuf":
					errorMsg.WriteString(`import _ "github.com/ag-ui/go-sdk/pkg/encoding/protobuf" `)
				}
			}
		}
		return errors.NewEncodingError(errors.CodeFormatNotRegistered, errorMsg.String()).WithOperation("ensure_registered")
	}

	return nil
}

// Cleanup Methods for Memory Management


// CleanupExpired removes all expired entries based on TTL
func (r *FormatRegistry) CleanupExpired() (int, error) {
	if r.config.TTL <= 0 {
		return 0, nil // TTL not configured
	}

	if atomic.LoadInt32(&r.closed) != 0 {
		return 0, NewOperationError("count", "registry", "registry is closed", nil)
	}

	cutoff := time.Now().Add(-r.config.TTL)
	totalCleaned := int64(0)
	var keysToDelete []registryKey

	// Collect expired entries
	r.entries.Range(func(key, value interface{}) bool {
		k := key.(registryKey)
		entry := value.(*RegistryEntry)

		if entry.createdAt.Before(cutoff) {
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

	// Update priorities after cleanup if formats were removed
	needsPriorityUpdate := false
	for _, key := range keysToDelete {
		if key.entryType == EntryTypeFormat {
			needsPriorityUpdate = true
			break
		}
	}

	if needsPriorityUpdate {
		r.updatePriorities()
	}

	return int(totalCleaned), nil
}

// CleanupByAccessTime removes entries that haven't been accessed within the given duration
func (r *FormatRegistry) CleanupByAccessTime(maxAge time.Duration) (int, error) {
	if atomic.LoadInt32(&r.closed) != 0 {
		return 0, NewOperationError("count", "registry", "registry is closed", nil)
	}

	cutoff := time.Now().Add(-maxAge)
	totalCleaned := int64(0)
	var keysToDelete []registryKey

	// Collect old entries
	r.entries.Range(func(key, value interface{}) bool {
		k := key.(registryKey)
		entry := value.(*RegistryEntry)

		if entry.lastAccess.Before(cutoff) {
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

	// Update priorities after cleanup if formats were removed
	needsPriorityUpdate := false
	for _, key := range keysToDelete {
		if key.entryType == EntryTypeFormat {
			needsPriorityUpdate = true
			break
		}
	}

	if needsPriorityUpdate {
		r.updatePriorities()
	}

	return int(totalCleaned), nil
}

// ClearAll removes all entries from the registry
func (r *FormatRegistry) ClearAll() error {
	if atomic.LoadInt32(&r.closed) != 0 {
		return NewOperationError("register", "registry", "registry is closed", nil)
	}

	// Clear all entries
	r.entries.Range(func(key, value interface{}) bool {
		r.entries.Delete(key)
		return true
	})

	// Clear LRU structures through cache manager
	r.cacheManager.Clear()

	// Clear priorities through the priority manager
	r.priorities.UpdatePriorities(make(map[string]*FormatInfo))

	// Reset metrics
	r.metrics.Reset()

	return nil
}

// GetRegistryStats returns statistics about the registry
func (r *FormatRegistry) GetRegistryStats() map[string]interface{} {
	// Count entries by type
	counts := make(map[RegistryEntryType]int)
	totalEntries := int64(0)

	r.entries.Range(func(key, value interface{}) bool {
		k := key.(registryKey)
		counts[k.entryType]++
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
		"total_entries":                  int(totalEntries), // Convert to int for backward compatibility
		"atomic_entry_count":             int(r.metrics.GetEntryCount()), // Convert to int for backward compatibility
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
func (r *FormatRegistry) UpdateConfig(config *RegistryConfig) error {
	if config == nil {
		return NewConfigurationError("registry", "config", "configuration cannot be nil", nil)
	}

	if r.lifecycle.IsClosed() {
		return NewOperationError("register", "registry", "registry is closed", nil)
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
		r.lifecycle = NewRegistryLifecycleManager(config)

		// Start new cleanup if enabled
		if config.EnableBackgroundCleanup && config.CleanupInterval > 0 {
			r.lifecycle.StartBackgroundCleanup(r.cleanupCallback)
		}
	}

	return nil
}

// Close stops background cleanup and releases resources
func (r *FormatRegistry) Close() error {
	// Set the registry as closed
	atomic.StoreInt32(&r.closed, 1)
	
	// Close the lifecycle manager (stops background cleanup)
	return r.lifecycle.Close()
}

// Memory pressure adaptation methods

// AdaptToMemoryPressure adjusts cleanup behavior based on current memory usage
func (r *FormatRegistry) AdaptToMemoryPressure(pressureLevel int) error {
	if r.lifecycle.IsClosed() {
		return NewOperationError("register", "registry", "registry is closed", nil)
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
				// Remove the 100 eviction limit under high pressure
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
		return NewValidationError("registry", "pressure_level", "range", "pressure level must be between 0 and 100", pressureLevel)
	}

	return nil
}

// ForceCleanup immediately removes entries that haven't been accessed recently
// This is useful for testing and emergency memory cleanup
func (r *FormatRegistry) ForceCleanup(olderThan time.Duration) (int, error) {
	if r.lifecycle.IsClosed() {
		return 0, NewOperationError("count", "registry", "registry is closed", nil)
	}

	return r.CleanupByAccessTime(olderThan)
}

// GetGlobalRegistrationErrors returns any errors that occurred during global registry initialization
func GetGlobalRegistrationErrors() []error {
	// Ensure global registry is initialized
	_ = GetGlobalRegistry()
	return globalRegistrationErrors
}

// CloseGlobalRegistry closes the global registry and cleans up its resources.
// This is primarily intended for testing and application shutdown.
// After calling this, the next call to GetGlobalRegistry() will create a new instance.
func CloseGlobalRegistry() error {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	
	if globalRegistry != nil {
		err := globalRegistry.Close()
		globalRegistry = nil
		
		// Reset the sync.Once so a new registry can be created
		globalOnce = sync.Once{}
		globalRegistrationErrors = nil
		
		return err
	}
	return nil
}

// Note: compositeCodec is already defined in codec_pool.go

// Registry-specific adapter types
type registryEncoderAdapter struct {
	encoder Encoder
}

func (e *registryEncoderAdapter) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return e.encoder.Encode(ctx, event)
}

func (e *registryEncoderAdapter) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return e.encoder.EncodeMultiple(ctx, events)
}

func (e *registryEncoderAdapter) ContentType() string {
	// Try to get content type from encoder if it supports it
	if provider, ok := e.encoder.(ContentTypeProvider); ok {
		return provider.ContentType()
	}
	return "application/octet-stream" // Default fallback
}

func (e *registryEncoderAdapter) SupportsStreaming() bool {
	// Try to get streaming support from encoder if it supports it
	if provider, ok := e.encoder.(StreamingCapabilityProvider); ok {
		return provider.SupportsStreaming()
	}
	return false // Default fallback
}

type registryDecoderAdapter struct {
	decoder Decoder
}

func (d *registryDecoderAdapter) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return d.decoder.Decode(ctx, data)
}

func (d *registryDecoderAdapter) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return d.decoder.DecodeMultiple(ctx, data)
}

func (d *registryDecoderAdapter) SupportsStreaming() bool {
	// Try to get streaming support from decoder if it supports it
	if provider, ok := d.decoder.(StreamingCapabilityProvider); ok {
		return provider.SupportsStreaming()
	}
	return false // Default fallback
}

func (d *registryDecoderAdapter) ContentType() string {
	return d.decoder.ContentType()
}