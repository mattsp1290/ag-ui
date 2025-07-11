package encoding

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	
	"github.com/ag-ui/go-sdk/pkg/errors"
)

// FormatRegistry manages encoders, decoders, and format information
type FormatRegistry struct {
	mu sync.RWMutex
	
	// Maps MIME type to format info
	formats map[string]*FormatInfo
	
	// Maps MIME type to encoder factory (concrete types)
	encoderFactories map[string]*DefaultEncoderFactory
	
	// Maps MIME type to decoder factory (concrete types)
	decoderFactories map[string]*DefaultDecoderFactory
	
	// Maps MIME type to codec factory (concrete types)
	codecFactories map[string]*DefaultCodecFactory
	
	// Legacy interface maps for backward compatibility
	legacyEncoderFactories map[string]EncoderFactory
	legacyDecoderFactories map[string]DecoderFactory
	legacyCodecFactories map[string]CodecFactory
	
	// Maps aliases to canonical MIME types
	aliases map[string]string
	
	// Priority order for format selection
	priorities []string
	
	// Default format when none specified
	defaultFormat string
	
	// Validation framework integration
	validator FormatValidator
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

// NewFormatRegistry creates a new format registry
func NewFormatRegistry() *FormatRegistry {
	return &FormatRegistry{
		formats:          make(map[string]*FormatInfo),
		encoderFactories: make(map[string]*DefaultEncoderFactory),
		decoderFactories: make(map[string]*DefaultDecoderFactory),
		codecFactories:   make(map[string]*DefaultCodecFactory),
		legacyEncoderFactories: make(map[string]EncoderFactory),
		legacyDecoderFactories: make(map[string]DecoderFactory),
		legacyCodecFactories:   make(map[string]CodecFactory),
		aliases:          make(map[string]string),
		priorities:       []string{},
		defaultFormat:    "application/json",
	}
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
	
	// Register the format
	r.formats[info.MIMEType] = info
	
	// Register aliases
	for _, alias := range info.Aliases {
		r.aliases[alias] = info.MIMEType
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
	
	r.legacyEncoderFactories[mimeType] = factory
	
	// If it's a concrete type, also register it in the concrete map
	if concreteFactory, ok := factory.(*DefaultEncoderFactory); ok {
		r.encoderFactories[mimeType] = concreteFactory
	}
	
	return nil
}

// RegisterEncoderFactory registers a concrete encoder factory (preferred method)
func (r *FormatRegistry) RegisterEncoderFactory(mimeType string, factory *DefaultEncoderFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_encoder_factory")
	}
	
	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "encoder factory cannot be nil").WithOperation("register_encoder_factory")
	}
	
	r.encoderFactories[mimeType] = factory
	r.legacyEncoderFactories[mimeType] = factory
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
	
	r.legacyDecoderFactories[mimeType] = factory
	
	// If it's a concrete type, also register it in the concrete map
	if concreteFactory, ok := factory.(*DefaultDecoderFactory); ok {
		r.decoderFactories[mimeType] = concreteFactory
	}
	
	return nil
}

// RegisterDecoderFactory registers a concrete decoder factory (preferred method)
func (r *FormatRegistry) RegisterDecoderFactory(mimeType string, factory *DefaultDecoderFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_decoder_factory")
	}
	
	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "decoder factory cannot be nil").WithOperation("register_decoder_factory")
	}
	
	r.decoderFactories[mimeType] = factory
	r.legacyDecoderFactories[mimeType] = factory
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
	
	r.legacyCodecFactories[mimeType] = factory
	
	// If it's a concrete type, also register it in the concrete map
	if concreteFactory, ok := factory.(*DefaultCodecFactory); ok {
		r.codecFactories[mimeType] = concreteFactory
		
		// Create backward compatibility factories
		r.encoderFactories[mimeType] = &DefaultEncoderFactory{DefaultCodecFactory: concreteFactory}
		r.decoderFactories[mimeType] = &DefaultDecoderFactory{DefaultCodecFactory: concreteFactory}
		
		// Register legacy interfaces
		r.legacyEncoderFactories[mimeType] = r.encoderFactories[mimeType]
		r.legacyDecoderFactories[mimeType] = r.decoderFactories[mimeType]
	}
	
	return nil
}

// RegisterCodecFactory registers a concrete codec factory (preferred method)
func (r *FormatRegistry) RegisterCodecFactory(mimeType string, factory *DefaultCodecFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return errors.NewEncodingError(errors.CodeEmptyMimeType, "MIME type cannot be empty").WithOperation("register_codec_factory")
	}
	
	if factory == nil {
		return errors.NewEncodingError(errors.CodeNilFactory, "codec factory cannot be nil").WithOperation("register_codec_factory")
	}
	
	r.codecFactories[mimeType] = factory
	
	// Create backward compatibility factories
	r.encoderFactories[mimeType] = &DefaultEncoderFactory{DefaultCodecFactory: factory}
	r.decoderFactories[mimeType] = &DefaultDecoderFactory{DefaultCodecFactory: factory}
	
	r.legacyCodecFactories[mimeType] = factory
	r.legacyEncoderFactories[mimeType] = r.encoderFactories[mimeType]
	r.legacyDecoderFactories[mimeType] = r.decoderFactories[mimeType]
	
	return nil
}

// UnregisterFormat removes a format from the registry
func (r *FormatRegistry) UnregisterFormat(mimeType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Remove format info
	info, exists := r.formats[canonical]
	if !exists {
		return errors.NewEncodingError(errors.CodeFormatNotRegistered, "format not registered").WithMimeType(mimeType).WithOperation("unregister_format")
	}
	
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
	info, exists := r.formats[canonical]
	if !exists {
		return nil, fmt.Errorf("format %s not registered", mimeType)
	}
	
	return info, nil
}

// GetEncoder creates an encoder for the specified MIME type
func (r *FormatRegistry) GetEncoder(ctx context.Context, mimeType string, options *EncodingOptions) (Encoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Try concrete factory first
	if factory, exists := r.encoderFactories[canonical]; exists {
		return factory.CreateEncoder(ctx, canonical, options)
	}
	
	// Fall back to legacy interface
	if factory, exists := r.legacyEncoderFactories[canonical]; exists {
		return factory.CreateEncoder(ctx, canonical, options)
	}
	
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no encoder registered for format").WithMimeType(mimeType).WithOperation("get_encoder")
}

// GetEncoderFactory returns the concrete encoder factory for the specified MIME type
func (r *FormatRegistry) GetEncoderFactory(mimeType string) (*DefaultEncoderFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	factory, exists := r.encoderFactories[canonical]
	if !exists {
		return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete encoder factory registered for format").WithMimeType(mimeType).WithOperation("get_encoder_factory")
	}
	
	return factory, nil
}

// GetDecoder creates a decoder for the specified MIME type
func (r *FormatRegistry) GetDecoder(ctx context.Context, mimeType string, options *DecodingOptions) (Decoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Try concrete factory first
	if factory, exists := r.decoderFactories[canonical]; exists {
		return factory.CreateDecoder(ctx, canonical, options)
	}
	
	// Fall back to legacy interface
	if factory, exists := r.legacyDecoderFactories[canonical]; exists {
		return factory.CreateDecoder(ctx, canonical, options)
	}
	
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no decoder registered for format").WithMimeType(mimeType).WithOperation("get_decoder")
}

// GetDecoderFactory returns the concrete decoder factory for the specified MIME type
func (r *FormatRegistry) GetDecoderFactory(mimeType string) (*DefaultDecoderFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	factory, exists := r.decoderFactories[canonical]
	if !exists {
		return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete decoder factory registered for format").WithMimeType(mimeType).WithOperation("get_decoder_factory")
	}
	
	return factory, nil
}

// GetCodec creates a codec for the specified MIME type
func (r *FormatRegistry) GetCodec(ctx context.Context, mimeType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Try concrete codec factory first
	if factory, exists := r.codecFactories[canonical]; exists {
		return factory.CreateCodec(ctx, canonical, encOptions, decOptions)
	}
	
	// Try legacy codec factory
	if factory, exists := r.legacyCodecFactories[canonical]; exists {
		return factory.CreateCodec(ctx, canonical, encOptions, decOptions)
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
	factory, exists := r.codecFactories[canonical]
	if !exists {
		return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete codec factory registered for format").WithMimeType(mimeType).WithOperation("get_codec_factory")
	}
	
	return factory, nil
}

// GetStreamEncoder creates a streaming encoder for the specified MIME type
func (r *FormatRegistry) GetStreamEncoder(ctx context.Context, mimeType string, options *EncodingOptions) (StreamEncoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Try concrete factory first
	if factory, exists := r.encoderFactories[canonical]; exists {
		return factory.CreateStreamEncoder(ctx, canonical, options)
	}
	
	// Fall back to legacy interface
	if factory, exists := r.legacyEncoderFactories[canonical]; exists {
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
	if factory, exists := r.decoderFactories[canonical]; exists {
		return factory.CreateStreamDecoder(ctx, canonical, options)
	}
	
	// Fall back to legacy interface
	if factory, exists := r.legacyDecoderFactories[canonical]; exists {
		return factory.CreateStreamDecoder(ctx, canonical, options)
	}
	
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no decoder registered for format").WithMimeType(mimeType).WithOperation("get_stream_decoder")
}

// ListFormats returns all registered formats
func (r *FormatRegistry) ListFormats() []*FormatInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	formats := make([]*FormatInfo, 0, len(r.formats))
	for _, info := range r.formats {
		formats = append(formats, info)
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
	info, exists := r.formats[canonical]
	if !exists {
		return nil, fmt.Errorf("format %s not registered", mimeType)
	}
	
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
		info, exists := r.formats[canonical]
		if !exists {
			continue
		}
		
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
func (r *FormatRegistry) resolveAlias(mimeType string) string {
	// Check if it's an alias
	if canonical, exists := r.aliases[strings.ToLower(mimeType)]; exists {
		return canonical
	}
	
	// Also check without parameters (e.g., "application/json; charset=utf-8" -> "application/json")
	if idx := strings.Index(mimeType, ";"); idx > 0 {
		base := strings.TrimSpace(mimeType[:idx])
		if canonical, exists := r.aliases[strings.ToLower(base)]; exists {
			return canonical
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
		pi := r.formats[priorities[i]].Priority
		pj := r.formats[priorities[j]].Priority
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