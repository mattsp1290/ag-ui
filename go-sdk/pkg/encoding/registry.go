package encoding

import (
	"container/list"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	
	"github.com/ag-ui/go-sdk/pkg/errors"
)

// RegistryEntry wraps registry data with metadata for cleanup
type RegistryEntry struct {
	value      interface{}
	createdAt  time.Time
	lastAccess time.Time
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
		MaxEntries:              1000,   // Limit to 1000 entries per map
		TTL:                     1 * time.Hour, // 1 hour TTL
		CleanupInterval:         10 * time.Minute, // Cleanup every 10 minutes
		EnableLRU:               true,
		EnableBackgroundCleanup: true,
	}
}

// FormatRegistry manages encoders, decoders, and format information with cleanup capabilities
type FormatRegistry struct {
	mu sync.RWMutex
	
	// Maps MIME type to format info with TTL and LRU support
	formats map[string]*RegistryEntry
	
	// Maps MIME type to encoder factory (concrete types)
	encoderFactories map[string]*RegistryEntry
	
	// Maps MIME type to decoder factory (concrete types)
	decoderFactories map[string]*RegistryEntry
	
	// Maps MIME type to codec factory (concrete types)
	codecFactories map[string]*RegistryEntry
	
	// Legacy interface maps for backward compatibility
	legacyEncoderFactories map[string]*RegistryEntry
	legacyDecoderFactories map[string]*RegistryEntry
	legacyCodecFactories map[string]*RegistryEntry
	
	// Maps aliases to canonical MIME types
	aliases map[string]*RegistryEntry
	
	// LRU tracking for each map
	formatsLRU        *list.List
	encodersLRU       *list.List
	decodersLRU       *list.List
	codecsLRU         *list.List
	legacyEncodersLRU *list.List
	legacyDecodersLRU *list.List
	legacyCodecsLRU   *list.List
	aliasesLRU        *list.List
	
	// LRU element tracking for O(1) operations
	formatsIndex        map[string]*list.Element
	encodersIndex       map[string]*list.Element
	decodersIndex       map[string]*list.Element
	codecsIndex         map[string]*list.Element
	legacyEncodersIndex map[string]*list.Element
	legacyDecodersIndex map[string]*list.Element
	legacyCodecsIndex   map[string]*list.Element
	aliasesIndex        map[string]*list.Element
	
	// Priority order for format selection
	priorities []string
	
	// Default format when none specified
	defaultFormat string
	
	// Validation framework integration
	validator FormatValidator
	
	// Registry configuration
	config *RegistryConfig
	
	// Background cleanup control
	cleanupStop chan struct{}
	cleanupOnce sync.Once
}

// FormatValidator interface for validation framework integration
type FormatValidator interface {
	ValidateFormat(mimeType string, data []byte) error
	ValidateEncoding(mimeType string, data []byte) error
	ValidateDecoding(mimeType string, data []byte) error
}

var (
	// Global registry instance
	globalRegistry *FormatRegistry
	globalOnce     sync.Once
)

// GetGlobalRegistry returns the global format registry
func GetGlobalRegistry() *FormatRegistry {
	globalOnce.Do(func() {
		globalRegistry = NewFormatRegistry()
		globalRegistry.RegisterDefaults()
	})
	return globalRegistry
}

// NewFormatRegistry creates a new format registry with default cleanup configuration
func NewFormatRegistry() *FormatRegistry {
	return NewFormatRegistryWithConfig(DefaultRegistryConfig())
}

// NewFormatRegistryWithConfig creates a new format registry with custom configuration
func NewFormatRegistryWithConfig(config *RegistryConfig) *FormatRegistry {
	if config == nil {
		config = DefaultRegistryConfig()
	}
	
	r := &FormatRegistry{
		formats:                make(map[string]*RegistryEntry),
		encoderFactories:       make(map[string]*RegistryEntry),
		decoderFactories:       make(map[string]*RegistryEntry),
		codecFactories:         make(map[string]*RegistryEntry),
		legacyEncoderFactories: make(map[string]*RegistryEntry),
		legacyDecoderFactories: make(map[string]*RegistryEntry),
		legacyCodecFactories:   make(map[string]*RegistryEntry),
		aliases:                make(map[string]*RegistryEntry),
		
		formatsLRU:        list.New(),
		encodersLRU:       list.New(),
		decodersLRU:       list.New(),
		codecsLRU:         list.New(),
		legacyEncodersLRU: list.New(),
		legacyDecodersLRU: list.New(),
		legacyCodecsLRU:   list.New(),
		aliasesLRU:        list.New(),
		
		formatsIndex:        make(map[string]*list.Element),
		encodersIndex:       make(map[string]*list.Element),
		decodersIndex:       make(map[string]*list.Element),
		codecsIndex:         make(map[string]*list.Element),
		legacyEncodersIndex: make(map[string]*list.Element),
		legacyDecodersIndex: make(map[string]*list.Element),
		legacyCodecsIndex:   make(map[string]*list.Element),
		aliasesIndex:        make(map[string]*list.Element),
		
		priorities:    []string{},
		defaultFormat: "application/json",
		config:        config,
		cleanupStop:   make(chan struct{}),
	}
	
	// Start background cleanup if enabled
	if config.EnableBackgroundCleanup && config.CleanupInterval > 0 {
		go r.backgroundCleanup()
	}
	
	return r
}

// RegisterFormat registers format information
func (r *FormatRegistry) RegisterFormat(info *FormatInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if info == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "format info cannot be nil").WithOperation("register_format")
	}
	
	if info.MIMEType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_format")
	}
	
	// Check if we need to evict entries before adding
	if r.config.MaxEntries > 0 && len(r.formats) >= r.config.MaxEntries {
		r.evictLRU(r.formatsLRU, r.formats, r.formatsIndex)
	}
	
	// Register the format with metadata
	entry := &RegistryEntry{
		value:       info,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.formats[info.MIMEType] = entry
	
	// Update LRU tracking
	if r.config.EnableLRU {
		elem := r.formatsLRU.PushFront(info.MIMEType)
		r.formatsIndex[info.MIMEType] = elem
	}
	
	// Register aliases
	for _, alias := range info.Aliases {
		// Check if we need to evict entries before adding
		if r.config.MaxEntries > 0 && len(r.aliases) >= r.config.MaxEntries {
			r.evictLRU(r.aliasesLRU, r.aliases, r.aliasesIndex)
		}
		
		aliasEntry := &RegistryEntry{
			value:       info.MIMEType,
			createdAt:   time.Now(),
			lastAccess:  time.Now(),
			accessCount: 1,
		}
		r.aliases[alias] = aliasEntry
		
		// Update LRU tracking
		if r.config.EnableLRU {
			elem := r.aliasesLRU.PushFront(alias)
			r.aliasesIndex[alias] = elem
		}
	}
	
	// Update priorities
	r.updatePriorities()
	
	return nil
}

// RegisterEncoder registers an encoder factory (legacy method - accepts interface)
func (r *FormatRegistry) RegisterEncoder(mimeType string, factory EncoderFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_encoder")
	}
	
	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "encoder factory cannot be nil").WithOperation("register_encoder")
	}
	
	// Check if we need to evict entries before adding
	if r.config.MaxEntries > 0 && len(r.legacyEncoderFactories) >= r.config.MaxEntries {
		r.evictLRU(r.legacyEncodersLRU, r.legacyEncoderFactories, r.legacyEncodersIndex)
	}
	
	// Register the legacy factory with metadata
	entry := &RegistryEntry{
		value:       factory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.legacyEncoderFactories[mimeType] = entry
	
	// Update LRU tracking
	if r.config.EnableLRU {
		elem := r.legacyEncodersLRU.PushFront(mimeType)
		r.legacyEncodersIndex[mimeType] = elem
	}
	
	// If it's a concrete type, also register it in the concrete map
	if concreteFactory, ok := factory.(*DefaultEncoderFactory); ok {
		// Check if we need to evict entries before adding
		if r.config.MaxEntries > 0 && len(r.encoderFactories) >= r.config.MaxEntries {
			r.evictLRU(r.encodersLRU, r.encoderFactories, r.encodersIndex)
		}
		
		concreteEntry := &RegistryEntry{
			value:       concreteFactory,
			createdAt:   time.Now(),
			lastAccess:  time.Now(),
			accessCount: 1,
		}
		r.encoderFactories[mimeType] = concreteEntry
		
		// Update LRU tracking
		if r.config.EnableLRU {
			elem := r.encodersLRU.PushFront(mimeType)
			r.encodersIndex[mimeType] = elem
		}
	}
	
	return nil
}

// RegisterEncoderFactory registers a concrete encoder factory (preferred method)
func (r *FormatRegistry) RegisterEncoderFactory(mimeType string, factory *DefaultEncoderFactory) error {
	if r == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "format registry is nil").WithOperation("register_encoder_factory")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_encoder_factory")
	}
	
	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "encoder factory cannot be nil").WithOperation("register_encoder_factory")
	}
	
	// Check if we need to evict entries before adding
	if r.config.MaxEntries > 0 && len(r.encoderFactories) >= r.config.MaxEntries {
		r.evictLRU(r.encodersLRU, r.encoderFactories, r.encodersIndex)
	}
	
	// Register the encoder factory with metadata
	entry := &RegistryEntry{
		value:       factory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.encoderFactories[mimeType] = entry
	
	// Update LRU tracking
	if r.config.EnableLRU {
		elem := r.encodersLRU.PushFront(mimeType)
		r.encodersIndex[mimeType] = elem
	}
	
	// Also register in legacy map for backward compatibility
	if r.config.MaxEntries > 0 && len(r.legacyEncoderFactories) >= r.config.MaxEntries {
		r.evictLRU(r.legacyEncodersLRU, r.legacyEncoderFactories, r.legacyEncodersIndex)
	}
	
	legacyEntry := &RegistryEntry{
		value:       factory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.legacyEncoderFactories[mimeType] = legacyEntry
	
	// Update LRU tracking for legacy
	if r.config.EnableLRU {
		elem := r.legacyEncodersLRU.PushFront(mimeType)
		r.legacyEncodersIndex[mimeType] = elem
	}
	
	return nil
}

// RegisterDecoder registers a decoder factory (legacy method - accepts interface)
func (r *FormatRegistry) RegisterDecoder(mimeType string, factory DecoderFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_decoder")
	}
	
	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "decoder factory cannot be nil").WithOperation("register_decoder")
	}
	
	// Check if we need to evict entries before adding
	if r.config.MaxEntries > 0 && len(r.legacyDecoderFactories) >= r.config.MaxEntries {
		r.evictLRU(r.legacyDecodersLRU, r.legacyDecoderFactories, r.legacyDecodersIndex)
	}
	
	// Register the legacy factory with metadata
	entry := &RegistryEntry{
		value:       factory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.legacyDecoderFactories[mimeType] = entry
	
	// Update LRU tracking
	if r.config.EnableLRU {
		elem := r.legacyDecodersLRU.PushFront(mimeType)
		r.legacyDecodersIndex[mimeType] = elem
	}
	
	// If it's a concrete type, also register it in the concrete map
	if concreteFactory, ok := factory.(*DefaultDecoderFactory); ok {
		// Check if we need to evict entries before adding
		if r.config.MaxEntries > 0 && len(r.decoderFactories) >= r.config.MaxEntries {
			r.evictLRU(r.decodersLRU, r.decoderFactories, r.decodersIndex)
		}
		
		concreteEntry := &RegistryEntry{
			value:       concreteFactory,
			createdAt:   time.Now(),
			lastAccess:  time.Now(),
			accessCount: 1,
		}
		r.decoderFactories[mimeType] = concreteEntry
		
		// Update LRU tracking
		if r.config.EnableLRU {
			elem := r.decodersLRU.PushFront(mimeType)
			r.decodersIndex[mimeType] = elem
		}
	}
	
	return nil
}

// RegisterDecoderFactory registers a concrete decoder factory (preferred method)
func (r *FormatRegistry) RegisterDecoderFactory(mimeType string, factory *DefaultDecoderFactory) error {
	if r == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "format registry is nil").WithOperation("register_decoder_factory")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_decoder_factory")
	}
	
	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "decoder factory cannot be nil").WithOperation("register_decoder_factory")
	}
	
	// Check if we need to evict entries before adding
	if r.config.MaxEntries > 0 && len(r.decoderFactories) >= r.config.MaxEntries {
		r.evictLRU(r.decodersLRU, r.decoderFactories, r.decodersIndex)
	}
	
	// Register the decoder factory with metadata
	entry := &RegistryEntry{
		value:       factory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.decoderFactories[mimeType] = entry
	
	// Update LRU tracking
	if r.config.EnableLRU {
		elem := r.decodersLRU.PushFront(mimeType)
		r.decodersIndex[mimeType] = elem
	}
	
	// Also register in legacy map for backward compatibility
	if r.config.MaxEntries > 0 && len(r.legacyDecoderFactories) >= r.config.MaxEntries {
		r.evictLRU(r.legacyDecodersLRU, r.legacyDecoderFactories, r.legacyDecodersIndex)
	}
	
	legacyEntry := &RegistryEntry{
		value:       factory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.legacyDecoderFactories[mimeType] = legacyEntry
	
	// Update LRU tracking for legacy
	if r.config.EnableLRU {
		elem := r.legacyDecodersLRU.PushFront(mimeType)
		r.legacyDecodersIndex[mimeType] = elem
	}
	
	return nil
}

// RegisterCodec registers a codec factory (legacy method - accepts interface)
func (r *FormatRegistry) RegisterCodec(mimeType string, factory CodecFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_codec")
	}
	
	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "codec factory cannot be nil").WithOperation("register_codec")
	}
	
	// Check if we need to evict entries before adding
	if r.config.MaxEntries > 0 && len(r.legacyCodecFactories) >= r.config.MaxEntries {
		r.evictLRU(r.legacyCodecsLRU, r.legacyCodecFactories, r.legacyCodecsIndex)
	}
	
	// Register the legacy factory with metadata
	legacyEntry := &RegistryEntry{
		value:       factory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.legacyCodecFactories[mimeType] = legacyEntry
	
	// Update LRU tracking
	if r.config.EnableLRU {
		elem := r.legacyCodecsLRU.PushFront(mimeType)
		r.legacyCodecsIndex[mimeType] = elem
	}
	
	// If it's a concrete type, also register it in the concrete map
	if concreteFactory, ok := factory.(*DefaultCodecFactory); ok {
		// Check if we need to evict entries before adding
		if r.config.MaxEntries > 0 && len(r.codecFactories) >= r.config.MaxEntries {
			r.evictLRU(r.codecsLRU, r.codecFactories, r.codecsIndex)
		}
		
		concreteEntry := &RegistryEntry{
			value:       concreteFactory,
			createdAt:   time.Now(),
			lastAccess:  time.Now(),
			accessCount: 1,
		}
		r.codecFactories[mimeType] = concreteEntry
		
		// Update LRU tracking
		if r.config.EnableLRU {
			elem := r.codecsLRU.PushFront(mimeType)
			r.codecsIndex[mimeType] = elem
		}
		
		// Create backward compatibility factories
		encoderFactory := &DefaultEncoderFactory{DefaultCodecFactory: concreteFactory}
		decoderFactory := &DefaultDecoderFactory{DefaultCodecFactory: concreteFactory}
		
		// Check if we need to evict entries before adding encoders
		if r.config.MaxEntries > 0 && len(r.encoderFactories) >= r.config.MaxEntries {
			r.evictLRU(r.encodersLRU, r.encoderFactories, r.encodersIndex)
		}
		
		encoderEntry := &RegistryEntry{
			value:       encoderFactory,
			createdAt:   time.Now(),
			lastAccess:  time.Now(),
			accessCount: 1,
		}
		r.encoderFactories[mimeType] = encoderEntry
		
		// Update LRU tracking for encoders
		if r.config.EnableLRU {
			elem := r.encodersLRU.PushFront(mimeType)
			r.encodersIndex[mimeType] = elem
		}
		
		// Check if we need to evict entries before adding decoders
		if r.config.MaxEntries > 0 && len(r.decoderFactories) >= r.config.MaxEntries {
			r.evictLRU(r.decodersLRU, r.decoderFactories, r.decodersIndex)
		}
		
		decoderEntry := &RegistryEntry{
			value:       decoderFactory,
			createdAt:   time.Now(),
			lastAccess:  time.Now(),
			accessCount: 1,
		}
		r.decoderFactories[mimeType] = decoderEntry
		
		// Update LRU tracking for decoders
		if r.config.EnableLRU {
			elem := r.decodersLRU.PushFront(mimeType)
			r.decodersIndex[mimeType] = elem
		}
		
		// Register legacy interfaces
		// Check if we need to evict entries before adding legacy encoders
		if r.config.MaxEntries > 0 && len(r.legacyEncoderFactories) >= r.config.MaxEntries {
			r.evictLRU(r.legacyEncodersLRU, r.legacyEncoderFactories, r.legacyEncodersIndex)
		}
		
		legacyEncoderEntry := &RegistryEntry{
			value:       encoderFactory,
			createdAt:   time.Now(),
			lastAccess:  time.Now(),
			accessCount: 1,
		}
		r.legacyEncoderFactories[mimeType] = legacyEncoderEntry
		
		// Update LRU tracking for legacy encoders
		if r.config.EnableLRU {
			elem := r.legacyEncodersLRU.PushFront(mimeType)
			r.legacyEncodersIndex[mimeType] = elem
		}
		
		// Check if we need to evict entries before adding legacy decoders
		if r.config.MaxEntries > 0 && len(r.legacyDecoderFactories) >= r.config.MaxEntries {
			r.evictLRU(r.legacyDecodersLRU, r.legacyDecoderFactories, r.legacyDecodersIndex)
		}
		
		legacyDecoderEntry := &RegistryEntry{
			value:       decoderFactory,
			createdAt:   time.Now(),
			lastAccess:  time.Now(),
			accessCount: 1,
		}
		r.legacyDecoderFactories[mimeType] = legacyDecoderEntry
		
		// Update LRU tracking for legacy decoders
		if r.config.EnableLRU {
			elem := r.legacyDecodersLRU.PushFront(mimeType)
			r.legacyDecodersIndex[mimeType] = elem
		}
	}
	
	return nil
}

// RegisterCodecFactory registers a concrete codec factory (preferred method)
func (r *FormatRegistry) RegisterCodecFactory(mimeType string, factory *DefaultCodecFactory) error {
	if r == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "format registry is nil").WithOperation("register_codec_factory")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_codec_factory")
	}
	
	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "codec factory cannot be nil").WithOperation("register_codec_factory")
	}
	
	// Check if we need to evict entries before adding
	if r.config.MaxEntries > 0 && len(r.codecFactories) >= r.config.MaxEntries {
		r.evictLRU(r.codecsLRU, r.codecFactories, r.codecsIndex)
	}
	
	// Register the codec factory with metadata
	codecEntry := &RegistryEntry{
		value:       factory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.codecFactories[mimeType] = codecEntry
	
	// Update LRU tracking
	if r.config.EnableLRU {
		elem := r.codecsLRU.PushFront(mimeType)
		r.codecsIndex[mimeType] = elem
	}
	
	// Create backward compatibility factories
	encoderFactory := &DefaultEncoderFactory{DefaultCodecFactory: factory}
	decoderFactory := &DefaultDecoderFactory{DefaultCodecFactory: factory}
	
	// Check if we need to evict entries before adding encoders
	if r.config.MaxEntries > 0 && len(r.encoderFactories) >= r.config.MaxEntries {
		r.evictLRU(r.encodersLRU, r.encoderFactories, r.encodersIndex)
	}
	
	encoderEntry := &RegistryEntry{
		value:       encoderFactory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.encoderFactories[mimeType] = encoderEntry
	
	// Update LRU tracking for encoders
	if r.config.EnableLRU {
		elem := r.encodersLRU.PushFront(mimeType)
		r.encodersIndex[mimeType] = elem
	}
	
	// Check if we need to evict entries before adding decoders
	if r.config.MaxEntries > 0 && len(r.decoderFactories) >= r.config.MaxEntries {
		r.evictLRU(r.decodersLRU, r.decoderFactories, r.decodersIndex)
	}
	
	decoderEntry := &RegistryEntry{
		value:       decoderFactory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.decoderFactories[mimeType] = decoderEntry
	
	// Update LRU tracking for decoders
	if r.config.EnableLRU {
		elem := r.decodersLRU.PushFront(mimeType)
		r.decodersIndex[mimeType] = elem
	}
	
	// Register legacy factories
	// Check if we need to evict entries before adding legacy codecs
	if r.config.MaxEntries > 0 && len(r.legacyCodecFactories) >= r.config.MaxEntries {
		r.evictLRU(r.legacyCodecsLRU, r.legacyCodecFactories, r.legacyCodecsIndex)
	}
	
	legacyCodecEntry := &RegistryEntry{
		value:       factory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.legacyCodecFactories[mimeType] = legacyCodecEntry
	
	// Update LRU tracking for legacy codecs
	if r.config.EnableLRU {
		elem := r.legacyCodecsLRU.PushFront(mimeType)
		r.legacyCodecsIndex[mimeType] = elem
	}
	
	// Check if we need to evict entries before adding legacy encoders
	if r.config.MaxEntries > 0 && len(r.legacyEncoderFactories) >= r.config.MaxEntries {
		r.evictLRU(r.legacyEncodersLRU, r.legacyEncoderFactories, r.legacyEncodersIndex)
	}
	
	legacyEncoderEntry := &RegistryEntry{
		value:       encoderFactory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.legacyEncoderFactories[mimeType] = legacyEncoderEntry
	
	// Update LRU tracking for legacy encoders
	if r.config.EnableLRU {
		elem := r.legacyEncodersLRU.PushFront(mimeType)
		r.legacyEncodersIndex[mimeType] = elem
	}
	
	// Check if we need to evict entries before adding legacy decoders
	if r.config.MaxEntries > 0 && len(r.legacyDecoderFactories) >= r.config.MaxEntries {
		r.evictLRU(r.legacyDecodersLRU, r.legacyDecoderFactories, r.legacyDecodersIndex)
	}
	
	legacyDecoderEntry := &RegistryEntry{
		value:       decoderFactory,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.legacyDecoderFactories[mimeType] = legacyDecoderEntry
	
	// Update LRU tracking for legacy decoders
	if r.config.EnableLRU {
		elem := r.legacyDecodersLRU.PushFront(mimeType)
		r.legacyDecodersIndex[mimeType] = elem
	}
	
	return nil
}

// UnregisterFormat removes a format from the registry
func (r *FormatRegistry) UnregisterFormat(mimeType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Remove format info
	entry, exists := r.formats[canonical]
	if !exists {
		return errors.NewEncodingError(errors.CodeFormatNotRegistered, "format not registered").WithMimeType(mimeType).WithOperation("unregister_format")
	}
	
	info := entry.value.(*FormatInfo)
	delete(r.formats, canonical)
	
	// Remove aliases
	for _, alias := range info.Aliases {
		delete(r.aliases, alias)
	}
	
	// Remove factories
	delete(r.encoderFactories, canonical)
	delete(r.decoderFactories, canonical)
	delete(r.codecFactories, canonical)
	delete(r.legacyEncoderFactories, canonical)
	delete(r.legacyDecoderFactories, canonical)
	delete(r.legacyCodecFactories, canonical)
	
	// Update priorities
	r.updatePriorities()
	
	return nil
}

// GetFormat returns format information for a MIME type
func (r *FormatRegistry) GetFormat(mimeType string) (*FormatInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	entry, exists := r.formats[canonical]
	if !exists {
		return nil, fmt.Errorf("format %s not registered", mimeType)
	}
	
	// Update access tracking
	entry.lastAccess = time.Now()
	entry.accessCount++
	
	// Update LRU position if enabled
	if r.config.EnableLRU {
		if elem, found := r.formatsIndex[canonical]; found {
			r.formatsLRU.MoveToFront(elem)
		}
	}
	
	return entry.value.(*FormatInfo), nil
}

// GetEncoder creates an encoder for the specified MIME type
func (r *FormatRegistry) GetEncoder(ctx context.Context, mimeType string, options *EncodingOptions) (Encoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Try concrete factory first
	if entry, exists := r.encoderFactories[canonical]; exists {
		// Update access tracking
		entry.lastAccess = time.Now()
		entry.accessCount++
		
		// Update LRU position if enabled
		if r.config.EnableLRU {
			if elem, found := r.encodersIndex[canonical]; found {
				r.encodersLRU.MoveToFront(elem)
			}
		}
		
		factory := entry.value.(*DefaultEncoderFactory)
		return factory.CreateEncoder(ctx, canonical, options)
	}
	
	// Fall back to legacy interface
	if entry, exists := r.legacyEncoderFactories[canonical]; exists {
		// Update access tracking
		entry.lastAccess = time.Now()
		entry.accessCount++
		
		// Update LRU position if enabled
		if r.config.EnableLRU {
			if elem, found := r.legacyEncodersIndex[canonical]; found {
				r.legacyEncodersLRU.MoveToFront(elem)
			}
		}
		
		factory := entry.value.(EncoderFactory)
		return factory.CreateEncoder(ctx, canonical, options)
	}
	
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no encoder registered for format").WithMimeType(mimeType).WithOperation("get_encoder")
}

// GetEncoderFactory returns the concrete encoder factory for the specified MIME type
func (r *FormatRegistry) GetEncoderFactory(mimeType string) (*DefaultEncoderFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	entry, exists := r.encoderFactories[canonical]
	if !exists {
		return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete encoder factory registered for format").WithMimeType(mimeType).WithOperation("get_encoder_factory")
	}
	
	// Update access tracking
	entry.lastAccess = time.Now()
	entry.accessCount++
	
	// Update LRU position if enabled
	if r.config.EnableLRU {
		if elem, found := r.encodersIndex[canonical]; found {
			r.encodersLRU.MoveToFront(elem)
		}
	}
	
	return entry.value.(*DefaultEncoderFactory), nil
}

// GetDecoder creates a decoder for the specified MIME type
func (r *FormatRegistry) GetDecoder(ctx context.Context, mimeType string, options *DecodingOptions) (Decoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Try concrete factory first
	if entry, exists := r.decoderFactories[canonical]; exists {
		// Update access tracking
		entry.lastAccess = time.Now()
		entry.accessCount++
		
		// Update LRU position if enabled
		if r.config.EnableLRU {
			if elem, found := r.decodersIndex[canonical]; found {
				r.decodersLRU.MoveToFront(elem)
			}
		}
		
		factory := entry.value.(*DefaultDecoderFactory)
		return factory.CreateDecoder(ctx, canonical, options)
	}
	
	// Fall back to legacy interface
	if entry, exists := r.legacyDecoderFactories[canonical]; exists {
		// Update access tracking
		entry.lastAccess = time.Now()
		entry.accessCount++
		
		// Update LRU position if enabled
		if r.config.EnableLRU {
			if elem, found := r.legacyDecodersIndex[canonical]; found {
				r.legacyDecodersLRU.MoveToFront(elem)
			}
		}
		
		factory := entry.value.(DecoderFactory)
		return factory.CreateDecoder(ctx, canonical, options)
	}
	
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no decoder registered for format").WithMimeType(mimeType).WithOperation("get_decoder")
}

// GetDecoderFactory returns the concrete decoder factory for the specified MIME type
func (r *FormatRegistry) GetDecoderFactory(mimeType string) (*DefaultDecoderFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	entry, exists := r.decoderFactories[canonical]
	if !exists {
		return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete decoder factory registered for format").WithMimeType(mimeType).WithOperation("get_decoder_factory")
	}
	
	// Update access tracking
	entry.lastAccess = time.Now()
	entry.accessCount++
	
	// Update LRU position if enabled
	if r.config.EnableLRU {
		if elem, found := r.decodersIndex[canonical]; found {
			r.decodersLRU.MoveToFront(elem)
		}
	}
	
	return entry.value.(*DefaultDecoderFactory), nil
}

// GetCodec creates a codec for the specified MIME type
func (r *FormatRegistry) GetCodec(ctx context.Context, mimeType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	r.mu.RLock()
	canonical := r.resolveAlias(mimeType)
	
	// Try concrete codec factory first
	if entry, exists := r.codecFactories[canonical]; exists {
		// Update access tracking
		entry.lastAccess = time.Now()
		entry.accessCount++
		
		// Update LRU position if enabled
		if r.config.EnableLRU {
			if elem, found := r.codecsIndex[canonical]; found {
				r.codecsLRU.MoveToFront(elem)
			}
		}
		
		factory := entry.value.(*DefaultCodecFactory)
		codec, err := factory.CreateCodec(ctx, canonical, encOptions, decOptions)
		r.mu.RUnlock()
		return codec, err
	}
	
	// Try legacy codec factory
	if entry, exists := r.legacyCodecFactories[canonical]; exists {
		// Update access tracking
		entry.lastAccess = time.Now()
		entry.accessCount++
		
		// Update LRU position if enabled
		if r.config.EnableLRU {
			if elem, found := r.legacyCodecsIndex[canonical]; found {
				r.legacyCodecsLRU.MoveToFront(elem)
			}
		}
		
		factory := entry.value.(CodecFactory)
		codec, err := factory.CreateCodec(ctx, canonical, encOptions, decOptions)
		r.mu.RUnlock()
		return codec, err
	}
	
	// Need to create composite codec - release lock before calling GetEncoder/GetDecoder
	r.mu.RUnlock()
	
	// Fall back to separate encoder/decoder
	encoder, err := r.GetEncoder(ctx, mimeType, encOptions)
	if err != nil {
		return nil, err
	}
	
	decoder, err := r.GetDecoder(ctx, mimeType, decOptions)
	if err != nil {
		return nil, err
	}
	
	return &compositeCodec{
		encoder: encoder,
		decoder: decoder,
	}, nil
}

// GetCodecFactory returns the concrete codec factory for the specified MIME type
func (r *FormatRegistry) GetCodecFactory(mimeType string) (*DefaultCodecFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	entry, exists := r.codecFactories[canonical]
	if !exists {
		return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete codec factory registered for format").WithMimeType(mimeType).WithOperation("get_codec_factory")
	}
	
	// Update access tracking
	entry.lastAccess = time.Now()
	entry.accessCount++
	
	// Update LRU position if enabled
	if r.config.EnableLRU {
		if elem, found := r.codecsIndex[canonical]; found {
			r.codecsLRU.MoveToFront(elem)
		}
	}
	
	return entry.value.(*DefaultCodecFactory), nil
}

// GetStreamEncoder creates a streaming encoder for the specified MIME type
func (r *FormatRegistry) GetStreamEncoder(ctx context.Context, mimeType string, options *EncodingOptions) (StreamEncoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Try concrete factory first
	if entry, exists := r.encoderFactories[canonical]; exists {
		// Update access tracking
		entry.lastAccess = time.Now()
		entry.accessCount++
		
		// Update LRU position if enabled
		if r.config.EnableLRU {
			if elem, found := r.encodersIndex[canonical]; found {
				r.encodersLRU.MoveToFront(elem)
			}
		}
		
		factory := entry.value.(*DefaultEncoderFactory)
		return factory.CreateStreamEncoder(ctx, canonical, options)
	}
	
	// Fall back to legacy interface
	if entry, exists := r.legacyEncoderFactories[canonical]; exists {
		// Update access tracking
		entry.lastAccess = time.Now()
		entry.accessCount++
		
		// Update LRU position if enabled
		if r.config.EnableLRU {
			if elem, found := r.legacyEncodersIndex[canonical]; found {
				r.legacyEncodersLRU.MoveToFront(elem)
			}
		}
		
		factory := entry.value.(EncoderFactory)
		return factory.CreateStreamEncoder(ctx, canonical, options)
	}
	
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no encoder registered for format").WithMimeType(mimeType).WithOperation("get_stream_encoder")
}

// GetStreamDecoder creates a streaming decoder for the specified MIME type
func (r *FormatRegistry) GetStreamDecoder(ctx context.Context, mimeType string, options *DecodingOptions) (StreamDecoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Try concrete factory first
	if entry, exists := r.decoderFactories[canonical]; exists {
		// Update access tracking
		entry.lastAccess = time.Now()
		entry.accessCount++
		
		// Update LRU position if enabled
		if r.config.EnableLRU {
			if elem, found := r.decodersIndex[canonical]; found {
				r.decodersLRU.MoveToFront(elem)
			}
		}
		
		factory := entry.value.(*DefaultDecoderFactory)
		return factory.CreateStreamDecoder(ctx, canonical, options)
	}
	
	// Fall back to legacy interface
	if entry, exists := r.legacyDecoderFactories[canonical]; exists {
		// Update access tracking
		entry.lastAccess = time.Now()
		entry.accessCount++
		
		// Update LRU position if enabled
		if r.config.EnableLRU {
			if elem, found := r.legacyDecodersIndex[canonical]; found {
				r.legacyDecodersLRU.MoveToFront(elem)
			}
		}
		
		factory := entry.value.(DecoderFactory)
		return factory.CreateStreamDecoder(ctx, canonical, options)
	}
	
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no decoder registered for format").WithMimeType(mimeType).WithOperation("get_stream_decoder")
}

// ListFormats returns all registered formats
func (r *FormatRegistry) ListFormats() []*FormatInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	formats := make([]*FormatInfo, 0, len(r.formats))
	for _, entry := range r.formats {
		formats = append(formats, entry.value.(*FormatInfo))
	}
	
	// Sort by priority
	sort.Slice(formats, func(i, j int) bool {
		return r.getPriority(formats[i].MIMEType) < r.getPriority(formats[j].MIMEType)
	})
	
	return formats
}

// SupportsFormat checks if a format is supported
func (r *FormatRegistry) SupportsFormat(mimeType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	_, exists := r.formats[canonical]
	return exists
}

// SupportsEncoding checks if encoding is supported for a format
func (r *FormatRegistry) SupportsEncoding(mimeType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Check concrete factory first
	if _, exists := r.encoderFactories[canonical]; exists {
		return true
	}
	
	// Check legacy factory
	if _, exists := r.legacyEncoderFactories[canonical]; exists {
		return true
	}
	
	return false
}

// SupportsDecoding checks if decoding is supported for a format
func (r *FormatRegistry) SupportsDecoding(mimeType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Check concrete factory first
	if _, exists := r.decoderFactories[canonical]; exists {
		return true
	}
	
	// Check legacy factory
	if _, exists := r.legacyDecoderFactories[canonical]; exists {
		return true
	}
	
	return false
}

// GetCapabilities returns the capabilities of a format
func (r *FormatRegistry) GetCapabilities(mimeType string) (*FormatCapabilities, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	entry, exists := r.formats[canonical]
	if !exists {
		return nil, fmt.Errorf("format %s not registered", mimeType)
	}
	
	info := entry.value.(*FormatInfo)
	return &info.Capabilities, nil
}

// SelectFormat selects the best format based on priorities and capabilities
func (r *FormatRegistry) SelectFormat(acceptedFormats []string, requiredCapabilities *FormatCapabilities) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// If no formats specified, use default
	if len(acceptedFormats) == 0 {
		return r.defaultFormat, nil
	}
	
	// Find the first format that matches requirements
	for _, format := range acceptedFormats {
		canonical := r.resolveAlias(format)
		entry, exists := r.formats[canonical]
		if !exists {
			continue
		}
		
		info := entry.value.(*FormatInfo)
		
		// Check if capabilities match
		if requiredCapabilities != nil {
			if !r.matchesCapabilities(&info.Capabilities, requiredCapabilities) {
				continue
			}
		}
		
		return canonical, nil
	}
	
	return "", errors.NewEncodingError(errors.CodeNoSuitableFormat, "no suitable format found").WithOperation("select_format")
}

// SetDefaultFormat sets the default format
func (r *FormatRegistry) SetDefaultFormat(mimeType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	canonical := r.resolveAlias(mimeType)
	if _, exists := r.formats[canonical]; !exists {
		return errors.NewEncodingError(errors.CodeFormatNotRegistered, "format not registered").WithMimeType(mimeType).WithOperation("set_default_format")
	}
	
	r.defaultFormat = canonical
	return nil
}

// GetDefaultFormat returns the default format
func (r *FormatRegistry) GetDefaultFormat() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	return r.defaultFormat
}

// SetValidator sets the format validator
func (r *FormatRegistry) SetValidator(validator FormatValidator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.validator = validator
}

// resolveAlias resolves an alias to canonical MIME type
// IMPORTANT: This method accesses the aliases map and MUST be called with at least a read lock held
func (r *FormatRegistry) resolveAlias(mimeType string) string {
	// Check if it's an alias
	if entry, exists := r.aliases[strings.ToLower(mimeType)]; exists {
		// Update access tracking
		entry.lastAccess = time.Now()
		entry.accessCount++
		
		// Update LRU position if enabled
		if r.config.EnableLRU {
			if elem, found := r.aliasesIndex[strings.ToLower(mimeType)]; found {
				r.aliasesLRU.MoveToFront(elem)
			}
		}
		
		return entry.value.(string)
	}
	
	// Also check without parameters (e.g., "application/json; charset=utf-8" -> "application/json")
	if idx := strings.Index(mimeType, ";"); idx > 0 {
		base := strings.TrimSpace(mimeType[:idx])
		if entry, exists := r.aliases[strings.ToLower(base)]; exists {
			// Update access tracking
			entry.lastAccess = time.Now()
			entry.accessCount++
			
			// Update LRU position if enabled
			if r.config.EnableLRU {
				if elem, found := r.aliasesIndex[strings.ToLower(base)]; found {
					r.aliasesLRU.MoveToFront(elem)
				}
			}
			
			return entry.value.(string)
		}
		return base
	}
	
	return mimeType
}

// updatePriorities updates the priority order based on format info
func (r *FormatRegistry) updatePriorities() {
	priorities := make([]string, 0, len(r.formats))
	for mimeType := range r.formats {
		priorities = append(priorities, mimeType)
	}
	
	// Sort by priority value (lower is higher priority)
	sort.Slice(priorities, func(i, j int) bool {
		pi := r.formats[priorities[i]].value.(*FormatInfo).Priority
		pj := r.formats[priorities[j]].value.(*FormatInfo).Priority
		if pi == pj {
			// Secondary sort by name for stability
			return priorities[i] < priorities[j]
		}
		return pi < pj
	})
	
	r.priorities = priorities
}

// getPriority returns the priority index for a format
func (r *FormatRegistry) getPriority(mimeType string) int {
	for i, mt := range r.priorities {
		if mt == mimeType {
			return i
		}
	}
	return len(r.priorities)
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

// compositeCodec combines separate encoder and decoder into a codec
// compositeCodec is defined in codec_pool.go to avoid duplication

// CanStream method is defined in codec_pool.go

// RegisterDefaults registers default formats (JSON and Protobuf)
// Deprecated: This method only registers format info, not the actual codecs.
// Import the specific codec packages (json, protobuf) to register their codecs.
func (r *FormatRegistry) RegisterDefaults() {
	// Register JSON format info only
	jsonInfo := JSONFormatInfo()
	_ = r.RegisterFormat(jsonInfo)
	
	// Register Protobuf format info only
	protobufInfo := ProtobufFormatInfo()
	_ = r.RegisterFormat(protobufInfo)
	
	// Note: The actual encoder/decoder factories must be registered by
	// importing the respective packages:
	//   import _ "github.com/ag-ui/go-sdk/pkg/encoding/json"
	//   import _ "github.com/ag-ui/go-sdk/pkg/encoding/protobuf"
	// Or by explicitly calling their Register() functions.
}

// Cleanup Methods for Memory Management

// evictLRU removes the least recently used entry from a map and its tracking structures
func (r *FormatRegistry) evictLRU(lru *list.List, entryMap map[string]*RegistryEntry, index map[string]*list.Element) {
	if lru.Len() == 0 {
		return
	}
	
	// Get the least recently used entry
	elem := lru.Back()
	if elem == nil {
		return
	}
	
	key := elem.Value.(string)
	
	// Remove from all structures
	lru.Remove(elem)
	delete(entryMap, key)
	delete(index, key)
}

// backgroundCleanup runs periodic cleanup of expired entries
func (r *FormatRegistry) backgroundCleanup() {
	ticker := time.NewTicker(r.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			r.CleanupExpired()
		case <-r.cleanupStop:
			return
		}
	}
}

// CleanupExpired removes all expired entries based on TTL
func (r *FormatRegistry) CleanupExpired() (int, error) {
	if r.config.TTL <= 0 {
		return 0, nil // TTL not configured
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	cutoff := time.Now().Add(-r.config.TTL)
	totalCleaned := 0
	
	// Clean expired formats
	totalCleaned += r.cleanupExpiredFromMap(r.formats, r.formatsLRU, r.formatsIndex, cutoff)
	
	// Clean expired encoder factories
	totalCleaned += r.cleanupExpiredFromMap(r.encoderFactories, r.encodersLRU, r.encodersIndex, cutoff)
	
	// Clean expired decoder factories
	totalCleaned += r.cleanupExpiredFromMap(r.decoderFactories, r.decodersLRU, r.decodersIndex, cutoff)
	
	// Clean expired codec factories
	totalCleaned += r.cleanupExpiredFromMap(r.codecFactories, r.codecsLRU, r.codecsIndex, cutoff)
	
	// Clean expired legacy factories
	totalCleaned += r.cleanupExpiredFromMap(r.legacyEncoderFactories, r.legacyEncodersLRU, r.legacyEncodersIndex, cutoff)
	totalCleaned += r.cleanupExpiredFromMap(r.legacyDecoderFactories, r.legacyDecodersLRU, r.legacyDecodersIndex, cutoff)
	totalCleaned += r.cleanupExpiredFromMap(r.legacyCodecFactories, r.legacyCodecsLRU, r.legacyCodecsIndex, cutoff)
	
	// Clean expired aliases
	totalCleaned += r.cleanupExpiredFromMap(r.aliases, r.aliasesLRU, r.aliasesIndex, cutoff)
	
	// Update priorities after cleanup
	r.updatePriorities()
	
	return totalCleaned, nil
}

// cleanupExpiredFromMap removes expired entries from a specific map
func (r *FormatRegistry) cleanupExpiredFromMap(entryMap map[string]*RegistryEntry, lru *list.List, index map[string]*list.Element, cutoff time.Time) int {
	cleaned := 0
	var toRemove []string
	
	// Collect expired keys
	for key, entry := range entryMap {
		if entry.createdAt.Before(cutoff) {
			toRemove = append(toRemove, key)
		}
	}
	
	// Remove expired entries
	for _, key := range toRemove {
		delete(entryMap, key)
		if elem, exists := index[key]; exists {
			lru.Remove(elem)
			delete(index, key)
		}
		cleaned++
	}
	
	return cleaned
}

// CleanupByAccessTime removes entries that haven't been accessed within the given duration
func (r *FormatRegistry) CleanupByAccessTime(maxAge time.Duration) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	cutoff := time.Now().Add(-maxAge)
	totalCleaned := 0
	
	// Clean by access time from all maps
	totalCleaned += r.cleanupByAccessTimeFromMap(r.formats, r.formatsLRU, r.formatsIndex, cutoff)
	totalCleaned += r.cleanupByAccessTimeFromMap(r.encoderFactories, r.encodersLRU, r.encodersIndex, cutoff)
	totalCleaned += r.cleanupByAccessTimeFromMap(r.decoderFactories, r.decodersLRU, r.decodersIndex, cutoff)
	totalCleaned += r.cleanupByAccessTimeFromMap(r.codecFactories, r.codecsLRU, r.codecsIndex, cutoff)
	totalCleaned += r.cleanupByAccessTimeFromMap(r.legacyEncoderFactories, r.legacyEncodersLRU, r.legacyEncodersIndex, cutoff)
	totalCleaned += r.cleanupByAccessTimeFromMap(r.legacyDecoderFactories, r.legacyDecodersLRU, r.legacyDecodersIndex, cutoff)
	totalCleaned += r.cleanupByAccessTimeFromMap(r.legacyCodecFactories, r.legacyCodecsLRU, r.legacyCodecsIndex, cutoff)
	totalCleaned += r.cleanupByAccessTimeFromMap(r.aliases, r.aliasesLRU, r.aliasesIndex, cutoff)
	
	// Update priorities after cleanup
	r.updatePriorities()
	
	return totalCleaned, nil
}

// cleanupByAccessTimeFromMap removes entries that haven't been accessed recently
func (r *FormatRegistry) cleanupByAccessTimeFromMap(entryMap map[string]*RegistryEntry, lru *list.List, index map[string]*list.Element, cutoff time.Time) int {
	cleaned := 0
	var toRemove []string
	
	// Collect old entries
	for key, entry := range entryMap {
		if entry.lastAccess.Before(cutoff) {
			toRemove = append(toRemove, key)
		}
	}
	
	// Remove old entries
	for _, key := range toRemove {
		delete(entryMap, key)
		if elem, exists := index[key]; exists {
			lru.Remove(elem)
			delete(index, key)
		}
		cleaned++
	}
	
	return cleaned
}

// ClearAll removes all entries from the registry
func (r *FormatRegistry) ClearAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Clear all maps
	r.formats = make(map[string]*RegistryEntry)
	r.encoderFactories = make(map[string]*RegistryEntry)
	r.decoderFactories = make(map[string]*RegistryEntry)
	r.codecFactories = make(map[string]*RegistryEntry)
	r.legacyEncoderFactories = make(map[string]*RegistryEntry)
	r.legacyDecoderFactories = make(map[string]*RegistryEntry)
	r.legacyCodecFactories = make(map[string]*RegistryEntry)
	r.aliases = make(map[string]*RegistryEntry)
	
	// Clear all LRU lists
	r.formatsLRU = list.New()
	r.encodersLRU = list.New()
	r.decodersLRU = list.New()
	r.codecsLRU = list.New()
	r.legacyEncodersLRU = list.New()
	r.legacyDecodersLRU = list.New()
	r.legacyCodecsLRU = list.New()
	r.aliasesLRU = list.New()
	
	// Clear all indexes
	r.formatsIndex = make(map[string]*list.Element)
	r.encodersIndex = make(map[string]*list.Element)
	r.decodersIndex = make(map[string]*list.Element)
	r.codecsIndex = make(map[string]*list.Element)
	r.legacyEncodersIndex = make(map[string]*list.Element)
	r.legacyDecodersIndex = make(map[string]*list.Element)
	r.legacyCodecsIndex = make(map[string]*list.Element)
	r.aliasesIndex = make(map[string]*list.Element)
	
	// Clear priorities
	r.priorities = []string{}
	
	return nil
}

// GetRegistryStats returns statistics about the registry
func (r *FormatRegistry) GetRegistryStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	stats := map[string]interface{}{
		"formats_count":               len(r.formats),
		"encoder_factories_count":     len(r.encoderFactories),
		"decoder_factories_count":     len(r.decoderFactories),
		"codec_factories_count":       len(r.codecFactories),
		"legacy_encoder_factories_count": len(r.legacyEncoderFactories),
		"legacy_decoder_factories_count": len(r.legacyDecoderFactories),
		"legacy_codec_factories_count":   len(r.legacyCodecFactories),
		"aliases_count":               len(r.aliases),
		"total_entries":               len(r.formats) + len(r.encoderFactories) + len(r.decoderFactories) + len(r.codecFactories) + len(r.legacyEncoderFactories) + len(r.legacyDecoderFactories) + len(r.legacyCodecFactories) + len(r.aliases),
		"max_entries_per_map":         r.config.MaxEntries,
		"ttl_seconds":                 r.config.TTL.Seconds(),
		"cleanup_interval_seconds":    r.config.CleanupInterval.Seconds(),
		"lru_enabled":                 r.config.EnableLRU,
		"background_cleanup_enabled":  r.config.EnableBackgroundCleanup,
	}
	
	return stats
}

// UpdateConfig updates the registry configuration
func (r *FormatRegistry) UpdateConfig(config *RegistryConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	oldConfig := r.config
	r.config = config
	
	// If background cleanup settings changed, restart background cleanup
	if oldConfig.EnableBackgroundCleanup != config.EnableBackgroundCleanup ||
		oldConfig.CleanupInterval != config.CleanupInterval {
		
		// Stop existing cleanup
		r.cleanupOnce.Do(func() {
			close(r.cleanupStop)
		})
		
		// Start new cleanup if enabled
		if config.EnableBackgroundCleanup && config.CleanupInterval > 0 {
			r.cleanupStop = make(chan struct{})
			r.cleanupOnce = sync.Once{}
			go r.backgroundCleanup()
		}
	}
	
	return nil
}

// Close stops background cleanup and releases resources
func (r *FormatRegistry) Close() error {
	r.cleanupOnce.Do(func() {
		close(r.cleanupStop)
	})
	return nil
}