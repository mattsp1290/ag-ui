package encoding

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// FormatRegistry manages encoders, decoders, and format information
type FormatRegistry struct {
	mu sync.RWMutex
	
	// Maps MIME type to format info
	formats map[string]*FormatInfo
	
	// Maps MIME type to encoder factory
	encoderFactories map[string]EncoderFactory
	
	// Maps MIME type to decoder factory
	decoderFactories map[string]DecoderFactory
	
	// Maps MIME type to codec factory
	codecFactories map[string]CodecFactory
	
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
		encoderFactories: make(map[string]EncoderFactory),
		decoderFactories: make(map[string]DecoderFactory),
		codecFactories:   make(map[string]CodecFactory),
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
		return fmt.Errorf("format info cannot be nil")
	}
	
	if info.MIMEType == "" {
		return fmt.Errorf("MIME type cannot be empty")
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

// RegisterEncoder registers an encoder factory
func (r *FormatRegistry) RegisterEncoder(mimeType string, factory EncoderFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return fmt.Errorf("MIME type cannot be empty")
	}
	
	if factory == nil {
		return fmt.Errorf("encoder factory cannot be nil")
	}
	
	r.encoderFactories[mimeType] = factory
	return nil
}

// RegisterDecoder registers a decoder factory
func (r *FormatRegistry) RegisterDecoder(mimeType string, factory DecoderFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return fmt.Errorf("MIME type cannot be empty")
	}
	
	if factory == nil {
		return fmt.Errorf("decoder factory cannot be nil")
	}
	
	r.decoderFactories[mimeType] = factory
	return nil
}

// RegisterCodec registers a codec factory
func (r *FormatRegistry) RegisterCodec(mimeType string, factory CodecFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if mimeType == "" {
		return fmt.Errorf("MIME type cannot be empty")
	}
	
	if factory == nil {
		return fmt.Errorf("codec factory cannot be nil")
	}
	
	r.codecFactories[mimeType] = factory
	
	// Also register as encoder and decoder
	r.encoderFactories[mimeType] = factory
	r.decoderFactories[mimeType] = factory
	
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
		return fmt.Errorf("format %s not registered", mimeType)
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
func (r *FormatRegistry) GetEncoder(mimeType string, options *EncodingOptions) (Encoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	factory, exists := r.encoderFactories[canonical]
	if !exists {
		return nil, fmt.Errorf("no encoder registered for %s", mimeType)
	}
	
	return factory.CreateEncoder(canonical, options)
}

// GetDecoder creates a decoder for the specified MIME type
func (r *FormatRegistry) GetDecoder(mimeType string, options *DecodingOptions) (Decoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	factory, exists := r.decoderFactories[canonical]
	if !exists {
		return nil, fmt.Errorf("no decoder registered for %s", mimeType)
	}
	
	return factory.CreateDecoder(canonical, options)
}

// GetCodec creates a codec for the specified MIME type
func (r *FormatRegistry) GetCodec(mimeType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	
	// Try codec factory first
	if factory, exists := r.codecFactories[canonical]; exists {
		encoder, err := factory.CreateEncoder(canonical, encOptions)
		if err != nil {
			return nil, err
		}
		
		decoder, err := factory.CreateDecoder(canonical, decOptions)
		if err != nil {
			return nil, err
		}
		
		// Create a composite codec
		return &compositeCodec{
			encoder: encoder,
			decoder: decoder,
		}, nil
	}
	
	// Fall back to separate encoder/decoder
	encoder, err := r.GetEncoder(mimeType, encOptions)
	if err != nil {
		return nil, err
	}
	
	decoder, err := r.GetDecoder(mimeType, decOptions)
	if err != nil {
		return nil, err
	}
	
	return &compositeCodec{
		encoder: encoder,
		decoder: decoder,
	}, nil
}

// GetStreamEncoder creates a streaming encoder for the specified MIME type
func (r *FormatRegistry) GetStreamEncoder(mimeType string, options *EncodingOptions) (StreamEncoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	factory, exists := r.encoderFactories[canonical]
	if !exists {
		return nil, fmt.Errorf("no encoder registered for %s", mimeType)
	}
	
	return factory.CreateStreamEncoder(canonical, options)
}

// GetStreamDecoder creates a streaming decoder for the specified MIME type
func (r *FormatRegistry) GetStreamDecoder(mimeType string, options *DecodingOptions) (StreamDecoder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	factory, exists := r.decoderFactories[canonical]
	if !exists {
		return nil, fmt.Errorf("no decoder registered for %s", mimeType)
	}
	
	return factory.CreateStreamDecoder(canonical, options)
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
	_, exists := r.encoderFactories[canonical]
	return exists
}

// SupportsDecoding checks if decoding is supported for a format
func (r *FormatRegistry) SupportsDecoding(mimeType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	canonical := r.resolveAlias(mimeType)
	_, exists := r.decoderFactories[canonical]
	return exists
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
	
	return "", fmt.Errorf("no suitable format found")
}

// SetDefaultFormat sets the default format
func (r *FormatRegistry) SetDefaultFormat(mimeType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	canonical := r.resolveAlias(mimeType)
	if _, exists := r.formats[canonical]; !exists {
		return fmt.Errorf("format %s not registered", mimeType)
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
type compositeCodec struct {
	encoder Encoder
	decoder Decoder
}

func (c *compositeCodec) Encode(event events.Event) ([]byte, error) {
	return c.encoder.Encode(event)
}

func (c *compositeCodec) EncodeMultiple(events []events.Event) ([]byte, error) {
	return c.encoder.EncodeMultiple(events)
}

func (c *compositeCodec) Decode(data []byte) (events.Event, error) {
	return c.decoder.Decode(data)
}

func (c *compositeCodec) DecodeMultiple(data []byte) ([]events.Event, error) {
	return c.decoder.DecodeMultiple(data)
}

func (c *compositeCodec) ContentType() string {
	return c.encoder.ContentType()
}

func (c *compositeCodec) CanStream() bool {
	return c.encoder.CanStream() && c.decoder.CanStream()
}

// RegisterDefaults registers default formats (JSON and Protobuf)
func (r *FormatRegistry) RegisterDefaults() {
	// Register JSON format
	jsonInfo := JSONFormatInfo()
	r.RegisterFormat(jsonInfo)
	
	// Register Protobuf format
	protobufInfo := ProtobufFormatInfo()
	r.RegisterFormat(protobufInfo)
	
	// Note: The actual encoder/decoder factories are registered by the 
	// respective packages' init functions when they are imported
}