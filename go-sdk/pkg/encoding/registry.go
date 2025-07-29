package encoding

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/registry"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// Legacy type aliases for backward compatibility
type RegistryEntry = registry.RegistryEntry
type RegistryConfig = registry.RegistryConfig
type RegistryEntryType = registry.RegistryEntryType

// Legacy constants for backward compatibility
const (
	EntryTypeFormat               = registry.EntryTypeFormat
	EntryTypeEncoderFactory       = registry.EntryTypeEncoderFactory
	EntryTypeDecoderFactory       = registry.EntryTypeDecoderFactory
	EntryTypeCodecFactory         = registry.EntryTypeCodecFactory
	EntryTypeLegacyEncoderFactory = registry.EntryTypeLegacyEncoderFactory
	EntryTypeLegacyDecoderFactory = registry.EntryTypeLegacyDecoderFactory
	EntryTypeLegacyCodecFactory   = registry.EntryTypeLegacyCodecFactory
	EntryTypeAlias                = registry.EntryTypeAlias
)

// Legacy functions for backward compatibility
func DefaultRegistryConfig() *RegistryConfig {
	return registry.DefaultRegistryConfig()
}

// registryKey represents a composite key for backward compatibility
type registryKey = registry.RegistryKey

// FormatValidator interface for validation framework integration
type FormatValidator = registry.FormatValidator

// FormatRegistry manages encoders, decoders, and format information with cleanup capabilities
// This implementation uses the new modular registry components for better maintainability
type FormatRegistry struct {
	// Core registry handles the fundamental operations
	core *registry.CoreRegistry

	// Legacy compatibility - these are kept to maintain the same API surface
	validator FormatValidator
}

// NewFormatRegistry creates a new format registry with default cleanup configuration
func NewFormatRegistry() *FormatRegistry {
	return NewFormatRegistryWithConfig(DefaultRegistryConfig())
}

// NewFormatRegistryWithConfig creates a new format registry with custom configuration
func NewFormatRegistryWithConfig(config *RegistryConfig) *FormatRegistry {
	r := &FormatRegistry{
		core: registry.NewCoreRegistryWithConfig(config),
	}
	return r
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
	if err := r.core.SetEntry(EntryTypeFormat, info.MIMEType, info); err != nil {
		return err
	}

	// Register aliases
	for _, alias := range info.Aliases {
		if err := r.core.SetEntry(EntryTypeAlias, alias, info.MIMEType); err != nil {
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
	if err := r.core.SetEntry(EntryTypeEncoderFactory, mimeType, factory); err != nil {
		return err
	}

	// Also register in legacy map for backward compatibility
	return r.core.SetEntry(EntryTypeLegacyEncoderFactory, mimeType, factory)
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
	if err := r.core.SetEntry(EntryTypeDecoderFactory, mimeType, factory); err != nil {
		return err
	}

	// Also register in legacy map for backward compatibility
	return r.core.SetEntry(EntryTypeLegacyDecoderFactory, mimeType, factory)
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
	if err := r.core.SetEntry(EntryTypeCodecFactory, mimeType, factory); err != nil {
		return err
	}

	// Create backward compatibility factories
	encoderFactory := &DefaultEncoderFactory{DefaultCodecFactory: factory}
	decoderFactory := &DefaultDecoderFactory{DefaultCodecFactory: factory}

	// Register encoder and decoder factories
	r.core.SetEntry(EntryTypeEncoderFactory, mimeType, encoderFactory)
	r.core.SetEntry(EntryTypeDecoderFactory, mimeType, decoderFactory)

	// Register legacy factories
	r.core.SetEntry(EntryTypeLegacyCodecFactory, mimeType, factory)
	r.core.SetEntry(EntryTypeLegacyEncoderFactory, mimeType, encoderFactory)
	r.core.SetEntry(EntryTypeLegacyDecoderFactory, mimeType, decoderFactory)

	return nil
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
	if err := r.core.SetEntry(EntryTypeCodecFactory, mimeType, factory); err != nil {
		return err
	}

	// Create backward compatibility adapter for the legacy CodecFactory interface
	legacyAdapter := NewLegacyCodecFactory(factory)
	if err := r.core.SetEntry(EntryTypeLegacyCodecFactory, mimeType, legacyAdapter); err != nil {
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

	r.core.SetEntry(EntryTypeEncoderFactory, mimeType, encoderFactory)
	r.core.SetEntry(EntryTypeDecoderFactory, mimeType, decoderFactory)
	r.core.SetEntry(EntryTypeLegacyEncoderFactory, mimeType, encoderFactory)
	r.core.SetEntry(EntryTypeLegacyDecoderFactory, mimeType, decoderFactory)

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
	if err := r.core.SetEntry(EntryTypeLegacyEncoderFactory, mimeType, factory); err != nil {
		return err
	}

	// If it's a concrete type, also register it in the concrete map
	if concreteFactory, ok := factory.(*DefaultEncoderFactory); ok {
		r.core.SetEntry(EntryTypeEncoderFactory, mimeType, concreteFactory)
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
	if err := r.core.SetEntry(EntryTypeLegacyDecoderFactory, mimeType, factory); err != nil {
		return err
	}

	// If it's a concrete type, also register it in the concrete map
	if concreteFactory, ok := factory.(*DefaultDecoderFactory); ok {
		r.core.SetEntry(EntryTypeDecoderFactory, mimeType, concreteFactory)
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
	if err := r.core.SetEntry(EntryTypeLegacyCodecFactory, mimeType, factory); err != nil {
		return err
	}

	// If it's a concrete type, also register it in concrete maps and create compatibility factories
	if concreteFactory, ok := factory.(*DefaultCodecFactory); ok {
		r.core.SetEntry(EntryTypeCodecFactory, mimeType, concreteFactory)

		// Create backward compatibility factories
		encoderFactory := &DefaultEncoderFactory{DefaultCodecFactory: concreteFactory}
		decoderFactory := &DefaultDecoderFactory{DefaultCodecFactory: concreteFactory}

		r.core.SetEntry(EntryTypeEncoderFactory, mimeType, encoderFactory)
		r.core.SetEntry(EntryTypeDecoderFactory, mimeType, decoderFactory)
		r.core.SetEntry(EntryTypeLegacyEncoderFactory, mimeType, encoderFactory)
		r.core.SetEntry(EntryTypeLegacyDecoderFactory, mimeType, decoderFactory)
	} else {
		// For non-concrete types that implement both EncoderFactory and DecoderFactory,
		// register them in the legacy encoder/decoder maps as well
		if encoderFactory, ok := factory.(EncoderFactory); ok {
			r.core.SetEntry(EntryTypeLegacyEncoderFactory, mimeType, encoderFactory)
		}
		if decoderFactory, ok := factory.(DecoderFactory); ok {
			r.core.SetEntry(EntryTypeLegacyDecoderFactory, mimeType, decoderFactory)
		}
	}

	return nil
}

// UnregisterFormat removes a format from the registry
func (r *FormatRegistry) UnregisterFormat(mimeType string) error {
	canonical := r.core.ResolveAlias(mimeType)

	// Check if format exists
	entry, exists := r.core.GetEntry(EntryTypeFormat, canonical)
	if !exists {
		return errors.NewEncodingError(errors.CodeFormatNotRegistered, "format not registered").WithMimeType(mimeType).WithOperation("unregister_format")
	}

	// Get format info to find aliases
	info := entry.Value.(*FormatInfo)

	// Remove aliases
	for _, alias := range info.Aliases {
		r.core.DeleteEntry(EntryTypeAlias, alias)
	}

	// Remove all entries for this MIME type
	r.core.DeleteEntry(EntryTypeFormat, canonical)
	r.core.DeleteEntry(EntryTypeEncoderFactory, canonical)
	r.core.DeleteEntry(EntryTypeDecoderFactory, canonical)
	r.core.DeleteEntry(EntryTypeCodecFactory, canonical)
	r.core.DeleteEntry(EntryTypeLegacyEncoderFactory, canonical)
	r.core.DeleteEntry(EntryTypeLegacyDecoderFactory, canonical)
	r.core.DeleteEntry(EntryTypeLegacyCodecFactory, canonical)

	// Update priorities
	r.updatePriorities()

	return nil
}

// Getter methods

// GetFormat returns format information for a MIME type
func (r *FormatRegistry) GetFormat(mimeType string) (*FormatInfo, error) {
	canonical := r.core.ResolveAlias(mimeType)
	if entry, exists := r.core.GetEntry(EntryTypeFormat, canonical); exists {
		return entry.Value.(*FormatInfo), nil
	}
	return nil, NewRegistryError("format_registry", "lookup", mimeType, "format not registered", nil)
}

// GetEncoder creates an encoder for the specified MIME type
func (r *FormatRegistry) GetEncoder(ctx context.Context, mimeType string, options *EncodingOptions) (Encoder, error) {
	canonical := r.core.ResolveAlias(mimeType)

	// Try concrete factory first
	if entry, exists := r.core.GetEntry(EntryTypeEncoderFactory, canonical); exists {
		factory := entry.Value.(*DefaultEncoderFactory)
		return factory.CreateEncoder(ctx, canonical, options)
	}

	// Fall back to codec factory
	if entry, exists := r.core.GetEntry(EntryTypeCodecFactory, canonical); exists {
		factory := entry.Value.(*DefaultCodecFactory)
		return factory.CreateCodec(ctx, canonical, options, nil)
	}

	// Fall back to legacy encoder factories
	if entry, exists := r.core.GetEntry(EntryTypeLegacyEncoderFactory, canonical); exists {
		factory := entry.Value.(EncoderFactory)
		return factory.CreateEncoder(ctx, canonical, options)
	}

	// Fall back to legacy codec factories
	if entry, exists := r.core.GetEntry(EntryTypeLegacyCodecFactory, canonical); exists {
		factory := entry.Value.(CodecFactory)
		return factory.CreateCodec(ctx, canonical, options, nil)
	}

	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no encoder registered for format").WithMimeType(mimeType).WithOperation("get_encoder")
}

// GetDecoder creates a decoder for the specified MIME type
func (r *FormatRegistry) GetDecoder(ctx context.Context, mimeType string, options *DecodingOptions) (Decoder, error) {
	canonical := r.core.ResolveAlias(mimeType)

	// Try concrete factory first
	if entry, exists := r.core.GetEntry(EntryTypeDecoderFactory, canonical); exists {
		factory := entry.Value.(*DefaultDecoderFactory)
		return factory.CreateDecoder(ctx, canonical, options)
	}

	// Fall back to codec factory
	if entry, exists := r.core.GetEntry(EntryTypeCodecFactory, canonical); exists {
		factory := entry.Value.(*DefaultCodecFactory)
		return factory.CreateCodec(ctx, canonical, nil, options)
	}

	// Fall back to legacy decoder factories
	if entry, exists := r.core.GetEntry(EntryTypeLegacyDecoderFactory, canonical); exists {
		factory := entry.Value.(DecoderFactory)
		return factory.CreateDecoder(ctx, canonical, options)
	}

	// Fall back to legacy codec factories
	if entry, exists := r.core.GetEntry(EntryTypeLegacyCodecFactory, canonical); exists {
		factory := entry.Value.(CodecFactory)
		return factory.CreateCodec(ctx, canonical, nil, options)
	}

	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no decoder registered for format").WithMimeType(mimeType).WithOperation("get_decoder")
}

// GetCodec creates a codec for the specified MIME type (backward compatible method)
func (r *FormatRegistry) GetCodec(ctx context.Context, mimeType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	canonical := r.core.ResolveAlias(mimeType)

	// Try concrete codec factory
	if entry, exists := r.core.GetEntry(EntryTypeCodecFactory, canonical); exists {
		factory := entry.Value.(*DefaultCodecFactory)
		return factory.CreateCodec(ctx, canonical, encOptions, decOptions)
	}

	// Try legacy codec factory
	if entry, exists := r.core.GetEntry(EntryTypeLegacyCodecFactory, canonical); exists {
		factory := entry.Value.(CodecFactory)
		return factory.CreateCodec(ctx, canonical, encOptions, decOptions)
	}

	// Check if we have separate encoder/decoder factories
	var hasEncoder, hasDecoder bool

	// Check concrete factories first
	if _, exists := r.core.GetEntry(EntryTypeEncoderFactory, canonical); exists {
		hasEncoder = true
	} else if _, exists := r.core.GetEntry(EntryTypeLegacyEncoderFactory, canonical); exists {
		hasEncoder = true
	}

	if _, exists := r.core.GetEntry(EntryTypeDecoderFactory, canonical); exists {
		hasDecoder = true
	} else if _, exists := r.core.GetEntry(EntryTypeLegacyDecoderFactory, canonical); exists {
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

// Utility methods

// updatePriorities updates the priority order based on format info
func (r *FormatRegistry) updatePriorities() {
	formatMap := make(map[string]registry.FormatInfoInterface)

	// Collect all formats with adapter
	entries := r.core.ListEntries(EntryTypeFormat)
	for mimeType, value := range entries {
		info := value.(*FormatInfo)
		formatMap[mimeType] = &formatInfoAdapter{info}
	}

	// Update priorities through the priority manager
	r.core.GetPriorityManager().UpdatePriorities(formatMap)
}

// Additional methods that delegate to core registry...

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

// Delegate other methods to core registry
func (r *FormatRegistry) ListFormats() []*FormatInfo {
	entries := r.core.ListEntries(EntryTypeFormat)
	var formats []*FormatInfo

	for _, value := range entries {
		formats = append(formats, value.(*FormatInfo))
	}

	// Get priority mapping from core
	priorities, defaultPriority := r.core.GetPriorityManager().GetPriorityMap()

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

func (r *FormatRegistry) SupportsFormat(mimeType string) bool {
	canonical := r.core.ResolveAlias(mimeType)
	_, exists := r.core.GetEntry(EntryTypeFormat, canonical)
	return exists
}

func (r *FormatRegistry) SupportsEncoding(mimeType string) bool {
	canonical := r.core.ResolveAlias(mimeType)

	// Check concrete factory first
	if _, exists := r.core.GetEntry(EntryTypeEncoderFactory, canonical); exists {
		return true
	}

	// Check legacy factory
	if _, exists := r.core.GetEntry(EntryTypeLegacyEncoderFactory, canonical); exists {
		return true
	}

	return false
}

func (r *FormatRegistry) SupportsDecoding(mimeType string) bool {
	canonical := r.core.ResolveAlias(mimeType)

	// Check concrete factory first
	if _, exists := r.core.GetEntry(EntryTypeDecoderFactory, canonical); exists {
		return true
	}

	// Check legacy factory
	if _, exists := r.core.GetEntry(EntryTypeLegacyDecoderFactory, canonical); exists {
		return true
	}

	return false
}

// GetCapabilities returns the capabilities of a format
func (r *FormatRegistry) GetCapabilities(mimeType string) (*FormatCapabilities, error) {
	canonical := r.core.ResolveAlias(mimeType)
	if entry, exists := r.core.GetEntry(EntryTypeFormat, canonical); exists {
		info := entry.Value.(*FormatInfo)
		return &info.Capabilities, nil
	}
	return nil, NewRegistryError("format_registry", "lookup", mimeType, "format not registered", nil)
}

// SetDefaultFormat sets the default format
func (r *FormatRegistry) SetDefaultFormat(mimeType string) error {
	canonical := r.core.ResolveAlias(mimeType)
	if _, exists := r.core.GetEntry(EntryTypeFormat, canonical); !exists {
		return errors.NewEncodingError(errors.CodeFormatNotRegistered, "format not registered").WithMimeType(mimeType).WithOperation("set_default_format")
	}

	r.core.GetPriorityManager().SetDefaultFormat(canonical)
	return nil
}

// GetDefaultFormat returns the default format
func (r *FormatRegistry) GetDefaultFormat() string {
	return r.core.GetPriorityManager().GetDefaultFormat()
}

// SetValidator sets the format validator
func (r *FormatRegistry) SetValidator(validator FormatValidator) {
	r.validator = validator
}

// Cleanup and management methods - delegate to core
func (r *FormatRegistry) CleanupExpired() (int, error) {
	return r.core.CleanupExpired()
}

func (r *FormatRegistry) CleanupByAccessTime(maxAge time.Duration) (int, error) {
	return r.core.CleanupByAccessTime(maxAge)
}

func (r *FormatRegistry) ClearAll() error {
	return r.core.ClearAll()
}

func (r *FormatRegistry) GetRegistryStats() map[string]interface{} {
	return r.core.GetRegistryStats()
}

func (r *FormatRegistry) UpdateConfig(config *RegistryConfig) error {
	return r.core.UpdateConfig(config)
}

func (r *FormatRegistry) Close() error {
	return r.core.Close()
}

func (r *FormatRegistry) AdaptToMemoryPressure(pressureLevel int) error {
	return r.core.AdaptToMemoryPressure(pressureLevel)
}

func (r *FormatRegistry) ForceCleanup(olderThan time.Duration) (int, error) {
	return r.core.ForceCleanup(olderThan)
}

// Global registry management
var (
	// Global registry instance
	globalRegistry     *FormatRegistry
	globalOnce         sync.Once
	globalMutex        sync.RWMutex
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

	return globalRegistry
}

// GetGlobalRegistrationErrors returns any errors that occurred during global registry initialization
func GetGlobalRegistrationErrors() []error {
	// Ensure global registry is initialized
	_ = GetGlobalRegistry()
	return globalRegistrationErrors
}

// CloseGlobalRegistry closes the global registry and cleans up its resources.
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

// Adapter types for backward compatibility

// formatInfoAdapter adapts FormatInfo to the registry.FormatInfo interface
type formatInfoAdapter struct {
	*FormatInfo
}

func (a *formatInfoAdapter) GetMIMEType() string {
	return a.MIMEType
}

func (a *formatInfoAdapter) GetPriority() int {
	return a.Priority
}

// Registry-specific adapter types (from original registry.go)
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

// Additional methods that weren't included in the core registry but are needed for compatibility

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
					errorMsg.WriteString(`import _ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json" `)
				case "application/x-protobuf":
					errorMsg.WriteString(`import _ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf" `)
				}
			}
		}
		return errors.NewEncodingError(errors.CodeFormatNotRegistered, errorMsg.String()).WithOperation("ensure_registered")
	}

	return nil
}

// SelectFormat selects the best format based on priorities and capabilities
func (r *FormatRegistry) SelectFormat(acceptedFormats []string, requiredCapabilities *FormatCapabilities) (string, error) {
	// If no formats specified, use default
	if len(acceptedFormats) == 0 {
		defaultFormat := r.core.GetPriorityManager().GetDefaultFormat()
		return defaultFormat, nil
	}

	// Find the first format that matches requirements
	for _, format := range acceptedFormats {
		canonical := r.core.ResolveAlias(format)
		if entry, exists := r.core.GetEntry(EntryTypeFormat, canonical); exists {
			info := entry.Value.(*FormatInfo)

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

// Factory getters

// GetEncoderFactory returns the concrete encoder factory for the specified MIME type
func (r *FormatRegistry) GetEncoderFactory(mimeType string) (*DefaultEncoderFactory, error) {
	canonical := r.core.ResolveAlias(mimeType)
	if entry, exists := r.core.GetEntry(EntryTypeEncoderFactory, canonical); exists {
		return entry.Value.(*DefaultEncoderFactory), nil
	}
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete encoder factory registered for format").WithMimeType(mimeType).WithOperation("get_encoder_factory")
}

// GetDecoderFactory returns the concrete decoder factory for the specified MIME type
func (r *FormatRegistry) GetDecoderFactory(mimeType string) (*DefaultDecoderFactory, error) {
	canonical := r.core.ResolveAlias(mimeType)
	if entry, exists := r.core.GetEntry(EntryTypeDecoderFactory, canonical); exists {
		return entry.Value.(*DefaultDecoderFactory), nil
	}
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete decoder factory registered for format").WithMimeType(mimeType).WithOperation("get_decoder_factory")
}

// GetCodecFactory returns the concrete codec factory for the specified MIME type
func (r *FormatRegistry) GetCodecFactory(mimeType string) (*DefaultCodecFactory, error) {
	canonical := r.core.ResolveAlias(mimeType)
	if entry, exists := r.core.GetEntry(EntryTypeCodecFactory, canonical); exists {
		return entry.Value.(*DefaultCodecFactory), nil
	}
	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no concrete codec factory registered for format").WithMimeType(mimeType).WithOperation("get_codec_factory")
}

// Stream encoder/decoder methods

// GetStreamEncoder creates a streaming encoder for the specified MIME type
func (r *FormatRegistry) GetStreamEncoder(ctx context.Context, mimeType string, options *EncodingOptions) (StreamEncoder, error) {
	canonical := r.core.ResolveAlias(mimeType)

	// Try concrete factory first
	if entry, exists := r.core.GetEntry(EntryTypeEncoderFactory, canonical); exists {
		factory := entry.Value.(*DefaultEncoderFactory)
		return factory.CreateStreamEncoder(ctx, canonical, options)
	}

	// Fall back to legacy interface
	if entry, exists := r.core.GetEntry(EntryTypeLegacyEncoderFactory, canonical); exists {
		factory := entry.Value.(EncoderFactory)
		return factory.CreateStreamEncoder(ctx, canonical, options)
	}

	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no encoder registered for format").WithMimeType(mimeType).WithOperation("get_stream_encoder")
}

// GetStreamDecoder creates a streaming decoder for the specified MIME type
func (r *FormatRegistry) GetStreamDecoder(ctx context.Context, mimeType string, options *DecodingOptions) (StreamDecoder, error) {
	canonical := r.core.ResolveAlias(mimeType)

	// Try concrete factory first
	if entry, exists := r.core.GetEntry(EntryTypeDecoderFactory, canonical); exists {
		factory := entry.Value.(*DefaultDecoderFactory)
		return factory.CreateStreamDecoder(ctx, canonical, options)
	}

	// Fall back to legacy interface
	if entry, exists := r.core.GetEntry(EntryTypeLegacyDecoderFactory, canonical); exists {
		factory := entry.Value.(DecoderFactory)
		return factory.CreateStreamDecoder(ctx, canonical, options)
	}

	return nil, errors.NewEncodingError(errors.CodeFormatNotRegistered, "no decoder registered for format").WithMimeType(mimeType).WithOperation("get_stream_decoder")
}