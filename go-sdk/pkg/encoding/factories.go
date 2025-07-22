package encoding

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// DefaultCodecFactory implements the simplified CodecFactory interface
type DefaultCodecFactory struct {
	mu sync.RWMutex
	
	// Constructor functions for codecs
	codecCtors map[string]CodecConstructor
	
	// Constructor functions for stream codecs
	streamCodecCtors map[string]StreamCodecConstructor
	
	// Supported content types
	supportedTypes []string
}

// Constructor function types for the simplified interfaces
type (
	CodecConstructor       func(encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error)
	StreamCodecConstructor func(encOptions *EncodingOptions, decOptions *DecodingOptions) (StreamCodec, error)
)

// NewDefaultCodecFactory creates a new codec factory
func NewDefaultCodecFactory() *DefaultCodecFactory {
	return &DefaultCodecFactory{
		codecCtors:       make(map[string]CodecConstructor),
		streamCodecCtors: make(map[string]StreamCodecConstructor),
		supportedTypes:   []string{},
	}
}

// NewCodecFactory creates a new codec factory (returns concrete type)
func NewCodecFactory() *DefaultCodecFactory {
	return NewDefaultCodecFactory()
}

// RegisterCodec registers a codec constructor
func (f *DefaultCodecFactory) RegisterCodec(contentType string, ctor CodecConstructor) {
	if f == nil {
		return // Silently ignore nil factory to prevent panics
	}
	if contentType == "" {
		return // Silently ignore empty content type
	}
	if ctor == nil {
		return // Silently ignore nil constructor
	}
	
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.codecCtors[contentType] = ctor
	f.updateSupportedTypes()
}

// RegisterStreamCodec registers a stream codec constructor
func (f *DefaultCodecFactory) RegisterStreamCodec(contentType string, ctor StreamCodecConstructor) {
	if f == nil {
		return // Silently ignore nil factory to prevent panics
	}
	if contentType == "" {
		return // Silently ignore empty content type
	}
	if ctor == nil {
		return // Silently ignore nil constructor
	}
	
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.streamCodecCtors[contentType] = ctor
	f.updateSupportedTypes()
}

// CreateCodec creates a codec for the specified content type
func (f *DefaultCodecFactory) CreateCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	if f == nil {
		return nil, NewConfigurationError("codec_factory", "factory", "codec factory cannot be nil", nil)
	}
	if ctx == nil {
		return nil, NewConfigurationError("codec_factory", "context", "context cannot be nil", nil)
	}
	if contentType == "" {
		return nil, NewConfigurationError("codec_factory", "content_type", "content type cannot be empty", "")
	}
	
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	ctor, exists := f.codecCtors[contentType]
	if !exists {
		return nil, NewRegistryError("codec_registry", "lookup", contentType, "no codec registered for content type", nil)
	}
	if ctor == nil {
		return nil, NewConfigurationError("codec_factory", "constructor", "codec constructor is nil", contentType)
	}
	
	if encOptions == nil {
		encOptions = &EncodingOptions{}
	}
	if decOptions == nil {
		decOptions = &DecodingOptions{}
	}
	
	return ctor(encOptions, decOptions)
}

// CreateStreamCodec creates a streaming codec for the specified content type
func (f *DefaultCodecFactory) CreateStreamCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (StreamCodec, error) {
	if f == nil {
		return nil, NewConfigurationError("codec_factory", "factory", "codec factory cannot be nil", nil)
	}
	if ctx == nil {
		return nil, NewConfigurationError("codec_factory", "context", "context cannot be nil", nil)
	}
	if contentType == "" {
		return nil, NewConfigurationError("codec_factory", "content_type", "content type cannot be empty", "")
	}
	
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	ctor, exists := f.streamCodecCtors[contentType]
	if !exists {
		return nil, NewRegistryError("stream_codec_registry", "lookup", contentType, "no stream codec registered for content type", nil)
	}
	if ctor == nil {
		return nil, NewConfigurationError("stream_codec_factory", "constructor", "stream codec constructor is nil", contentType)
	}
	
	if encOptions == nil {
		encOptions = &EncodingOptions{}
	}
	if decOptions == nil {
		decOptions = &DecodingOptions{}
	}
	
	return ctor(encOptions, decOptions)
}

// SupportedTypes returns list of supported content types
func (f *DefaultCodecFactory) SupportedTypes() []string {
	if f == nil {
		return []string{} // Return empty slice instead of nil to prevent panics
	}
	
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	if f.supportedTypes == nil {
		return []string{}
	}
	
	types := make([]string, len(f.supportedTypes))
	copy(types, f.supportedTypes)
	return types
}

// SupportsStreaming indicates if streaming is supported for the given content type
func (f *DefaultCodecFactory) SupportsStreaming(contentType string) bool {
	if f == nil || contentType == "" {
		return false
	}
	
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	if f.streamCodecCtors == nil {
		return false
	}
	
	_, exists := f.streamCodecCtors[contentType]
	return exists
}

// RegisterEncoder registers an encoder constructor for backward compatibility
func (f *DefaultCodecFactory) RegisterEncoder(contentType string, ctor EncoderConstructor) {
	if f == nil {
		return // Silently ignore nil factory to prevent panics
	}
	if contentType == "" {
		return // Silently ignore empty content type
	}
	if ctor == nil {
		return // Silently ignore nil constructor
	}
	
	// Create a codec constructor that wraps the encoder
	codecCtor := func(encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
		encoder, err := ctor(encOptions)
		if err != nil {
			return nil, err
		}
		// Create an adapter for the encoder
		encoderAdapter := &encoderWithAllInterfaces{encoder: encoder}
		// Create an encoder-only codec that returns errors for decode operations
		return &encoderOnlyCodec{encoder: encoderAdapter}, nil
	}
	
	f.RegisterCodec(contentType, codecCtor)
}

// RegisterDecoder registers a decoder constructor for backward compatibility
func (f *DefaultCodecFactory) RegisterDecoder(contentType string, ctor DecoderConstructor) {
	if f == nil {
		return // Silently ignore nil factory to prevent panics
	}
	if contentType == "" {
		return // Silently ignore empty content type
	}
	if ctor == nil {
		return // Silently ignore nil constructor
	}
	
	// Create a codec constructor that wraps the decoder
	codecCtor := func(encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
		decoder, err := ctor(decOptions)
		if err != nil {
			return nil, err
		}
		// Create an adapter for the decoder
		decoderAdapter := &decoderWithAllInterfaces{decoder: decoder}
		// Create a decoder-only codec that returns errors for encode operations
		return &decoderOnlyCodec{decoder: decoderAdapter}, nil
	}
	
	f.RegisterCodec(contentType, codecCtor)
}

// RegisterStreamEncoder registers a stream encoder constructor for backward compatibility
// Note: This is a simplified implementation that doesn't fully support streaming
func (f *DefaultCodecFactory) RegisterStreamEncoder(contentType string, ctor StreamEncoderConstructor) {
	if f == nil {
		return // Silently ignore nil factory to prevent panics
	}
	if contentType == "" {
		return // Silently ignore empty content type
	}
	if ctor == nil {
		return // Silently ignore nil constructor
	}
	
	// Create an adapter that extracts Encoder interface from StreamEncoder
	encoderCtor := func(options *EncodingOptions) (Encoder, error) {
		streamEncoder, err := ctor(options)
		if err != nil {
			return nil, err
		}
		// Extract basic encoder functionality - StreamEncoder should embed Encoder
		if basicEncoder, ok := streamEncoder.(Encoder); ok {
			return basicEncoder, nil
		}
		return nil, fmt.Errorf("stream encoder does not implement basic Encoder interface")
	}
	f.RegisterEncoder(contentType, encoderCtor)
}

// RegisterStreamDecoder registers a stream decoder constructor for backward compatibility
// Note: This is a simplified implementation that doesn't fully support streaming
func (f *DefaultCodecFactory) RegisterStreamDecoder(contentType string, ctor StreamDecoderConstructor) {
	if f == nil {
		return // Silently ignore nil factory to prevent panics
	}
	if contentType == "" {
		return // Silently ignore empty content type
	}
	if ctor == nil {
		return // Silently ignore nil constructor
	}
	
	// Create an adapter that extracts Decoder interface from StreamDecoder
	decoderCtor := func(options *DecodingOptions) (Decoder, error) {
		streamDecoder, err := ctor(options)
		if err != nil {
			return nil, err
		}
		// Extract basic decoder functionality - StreamDecoder should embed Decoder
		if basicDecoder, ok := streamDecoder.(Decoder); ok {
			return basicDecoder, nil
		}
		return nil, fmt.Errorf("stream decoder does not implement basic Decoder interface")
	}
	f.RegisterDecoder(contentType, decoderCtor)
}

// updateSupportedTypes updates the list of supported types
func (f *DefaultCodecFactory) updateSupportedTypes() {
	typeMap := make(map[string]bool)
	
	for contentType := range f.codecCtors {
		typeMap[contentType] = true
	}
	
	for contentType := range f.streamCodecCtors {
		typeMap[contentType] = true
	}
	
	f.supportedTypes = make([]string, 0, len(typeMap))
	for contentType := range typeMap {
		f.supportedTypes = append(f.supportedTypes, contentType)
	}
}

// PluginCodecFactory allows plugins to register codecs
type PluginCodecFactory struct {
	*DefaultCodecFactory
	plugins map[string]CodecPlugin
}

// CodecPlugin interface for codec plugins
type CodecPlugin interface {
	// Name returns the plugin name
	Name() string
	
	// ContentTypes returns supported content types
	ContentTypes() []string
	
	// CreateCodec creates a codec
	CreateCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error)
	
	// CreateStreamCodec creates a stream codec
	CreateStreamCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (StreamCodec, error)
	
	// SupportsStreaming indicates if streaming is supported for the given content type
	SupportsStreaming(contentType string) bool
}

// NewPluginCodecFactory creates a new plugin-enabled codec factory
func NewPluginCodecFactory() *PluginCodecFactory {
	return &PluginCodecFactory{
		DefaultCodecFactory: NewDefaultCodecFactory(),
		plugins:             make(map[string]CodecPlugin),
	}
}

// RegisterPlugin registers a codec plugin
func (f *PluginCodecFactory) RegisterPlugin(plugin CodecPlugin) error {
	if plugin == nil {
		return NewConfigurationError("plugin_registry", "plugin", "plugin cannot be nil", nil)
	}
	
	name := plugin.Name()
	if name == "" {
		return NewConfigurationError("plugin_registry", "name", "plugin name cannot be empty", "")
	}
	
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.plugins[name] = plugin
	
	// Register constructors for each content type
	for _, contentType := range plugin.ContentTypes() {
		ct := contentType // capture loop variable
		f.codecCtors[ct] = func(encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
			return plugin.CreateCodec(context.Background(), ct, encOptions, decOptions)
		}
		
		if plugin.SupportsStreaming(ct) {
			f.streamCodecCtors[ct] = func(encOptions *EncodingOptions, decOptions *DecodingOptions) (StreamCodec, error) {
				return plugin.CreateStreamCodec(context.Background(), ct, encOptions, decOptions)
			}
		}
	}
	
	f.updateSupportedTypes()
	return nil
}

// CachingCodecFactory wraps a factory with caching
type CachingCodecFactory struct {
	factory CodecFactory
	cache   sync.Map // map[string]Codec
}

// NewCachingCodecFactory creates a new caching codec factory
func NewCachingCodecFactory(factory CodecFactory) *CachingCodecFactory {
	if factory == nil {
		return nil
	}
	return &CachingCodecFactory{
		factory: factory,
	}
}

// CreateCodec creates or retrieves a cached codec
func (f *CachingCodecFactory) CreateCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	if f == nil {
		return nil, fmt.Errorf("caching codec factory is nil")
	}
	if f.factory == nil {
		return nil, fmt.Errorf("underlying codec factory is nil")
	}
	if ctx == nil {
		return nil, NewConfigurationError("codec_factory", "context", "context cannot be nil", nil)
	}
	if contentType == "" {
		return nil, NewConfigurationError("codec_factory", "content_type", "content type cannot be empty", "")
	}
	
	// Create a cache key from content type and options
	key := f.cacheKey(contentType, encOptions, decOptions)
	
	// Check cache
	if cached, ok := f.cache.Load(key); ok {
		if codec, ok := cached.(Codec); ok {
			return codec, nil
		}
		// If cached value is not a Codec, remove it and continue
		f.cache.Delete(key)
	}
	
	// Create new codec
	codec, err := f.factory.CreateCodec(ctx, contentType, encOptions, decOptions)
	if err != nil {
		return nil, err
	}
	
	if codec == nil {
		return nil, fmt.Errorf("underlying factory returned nil codec for content type: %s", contentType)
	}
	
	// Cache it
	f.cache.Store(key, codec)
	return codec, nil
}

// CreateStreamCodec creates a streaming codec (not cached due to stateful nature)
func (f *CachingCodecFactory) CreateStreamCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (StreamCodec, error) {
	if f == nil {
		return nil, fmt.Errorf("caching codec factory is nil")
	}
	if f.factory == nil {
		return nil, fmt.Errorf("underlying codec factory is nil")
	}
	if ctx == nil {
		return nil, NewConfigurationError("codec_factory", "context", "context cannot be nil", nil)
	}
	if contentType == "" {
		return nil, NewConfigurationError("codec_factory", "content_type", "content type cannot be empty", "")
	}
	
	// Try to use as StreamCodecFactory first
	if streamFactory, ok := f.factory.(StreamCodecFactory); ok {
		return streamFactory.CreateStreamCodec(ctx, contentType, encOptions, decOptions)
	}
	
	// Fallback: try to use as FullCodecFactory
	if fullFactory, ok := f.factory.(FullCodecFactory); ok {
		return fullFactory.CreateStreamCodec(ctx, contentType, encOptions, decOptions)
	}
	
	return nil, fmt.Errorf("underlying factory does not support stream codec creation")
}

// SupportedTypes returns list of supported content types
func (f *CachingCodecFactory) SupportedTypes() []string {
	if f == nil || f.factory == nil {
		return []string{}
	}
	return f.factory.SupportedTypes()
}

// SupportsStreaming indicates if streaming is supported for the given content type
func (f *CachingCodecFactory) SupportsStreaming(contentType string) bool {
	if f == nil || f.factory == nil || contentType == "" {
		return false
	}
	
	// Try to use as StreamCodecFactory first
	if streamFactory, ok := f.factory.(StreamCodecFactory); ok {
		return streamFactory.SupportsStreaming(contentType)
	}
	
	// Fallback: try to use as FullCodecFactory
	if fullFactory, ok := f.factory.(FullCodecFactory); ok {
		return fullFactory.SupportsStreaming(contentType)
	}
	
	return false
}

// cacheKey generates a cache key from content type and options
func (f *CachingCodecFactory) cacheKey(contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) string {
	key := contentType
	
	if encOptions != nil {
		key += fmt.Sprintf(":enc:%v:%s:%d:%v:%v",
			encOptions.Pretty,
			encOptions.Compression,
			encOptions.BufferSize,
			encOptions.ValidateOutput,
			encOptions.CrossSDKCompatibility,
		)
	}
	
	if decOptions != nil {
		key += fmt.Sprintf(":dec:%v:%d:%v:%v",
			decOptions.Strict,
			decOptions.BufferSize,
			decOptions.AllowUnknownFields,
			decOptions.ValidateEvents,
		)
	}
	
	return key
}

// Backward compatibility - DEPRECATED
// These factory interfaces and implementations are provided for backward compatibility

// EncoderFactory is deprecated - use CodecFactory instead
type EncoderFactory interface {
	CreateEncoder(ctx context.Context, contentType string, options *EncodingOptions) (Encoder, error)
	CreateStreamEncoder(ctx context.Context, contentType string, options *EncodingOptions) (StreamEncoder, error)
	SupportedEncoders() []string
}

// DecoderFactory is deprecated - use CodecFactory instead
type DecoderFactory interface {
	CreateDecoder(ctx context.Context, contentType string, options *DecodingOptions) (Decoder, error)
	CreateStreamDecoder(ctx context.Context, contentType string, options *DecodingOptions) (StreamDecoder, error)
	SupportedDecoders() []string
}

// Constructor function types for backward compatibility
type (
	EncoderConstructor       func(options *EncodingOptions) (Encoder, error)
	StreamEncoderConstructor func(options *EncodingOptions) (StreamEncoder, error)
	DecoderConstructor       func(options *DecodingOptions) (Decoder, error)
	StreamDecoderConstructor func(options *DecodingOptions) (StreamDecoder, error)
)

// Adapter types for backward compatibility
type encoderAdapter struct {
	codec Codec
}

func (a *encoderAdapter) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return a.codec.Encode(ctx, event)
}

func (a *encoderAdapter) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return a.codec.EncodeMultiple(ctx, events)
}

func (a *encoderAdapter) ContentType() string {
	return a.codec.ContentType()
}

func (a *encoderAdapter) CanStream() bool {
	return a.codec.SupportsStreaming()
}

func (a *encoderAdapter) SupportsStreaming() bool {
	return a.codec.SupportsStreaming()
}

type decoderAdapter struct {
	codec Codec
}

func (a *decoderAdapter) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return a.codec.Decode(ctx, data)
}

func (a *decoderAdapter) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return a.codec.DecodeMultiple(ctx, data)
}

func (a *decoderAdapter) ContentType() string {
	return a.codec.ContentType()
}

func (a *decoderAdapter) CanStream() bool {
	return a.codec.SupportsStreaming()
}

func (a *decoderAdapter) SupportsStreaming() bool {
	return a.codec.SupportsStreaming()
}

type streamEncoderAdapter struct {
	codec    StreamCodec
	session  StreamSessionManager
	processor StreamEventProcessor
}

func (a *streamEncoderAdapter) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	// StreamCodec doesn't have simple Encode method, so this is a placeholder
	return nil, fmt.Errorf("direct encode not supported by stream codec")
}

func (a *streamEncoderAdapter) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	// StreamCodec doesn't have simple EncodeMultiple method, so this is a placeholder
	return nil, fmt.Errorf("direct encode multiple not supported by stream codec")
}

func (a *streamEncoderAdapter) ContentType() string {
	return a.codec.ContentType()
}

func (a *streamEncoderAdapter) CanStream() bool {
	return true
}

func (a *streamEncoderAdapter) SupportsStreaming() bool {
	return true
}

func (a *streamEncoderAdapter) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return a.codec.EncodeStream(ctx, input, output)
}

func (a *streamEncoderAdapter) StartStream(ctx context.Context, w io.Writer) error {
	if a.session != nil {
		return a.session.StartEncodingSession(ctx, w)
	}
	return fmt.Errorf("session management not available")
}

func (a *streamEncoderAdapter) WriteEvent(ctx context.Context, event events.Event) error {
	if a.processor != nil {
		return a.processor.WriteEvent(ctx, event)
	}
	return fmt.Errorf("event processing not available")
}

func (a *streamEncoderAdapter) EndStream(ctx context.Context) error {
	if a.session != nil {
		return a.session.EndSession(ctx)
	}
	return fmt.Errorf("session management not available")
}

type streamDecoderAdapter struct {
	codec    StreamCodec
	session  StreamSessionManager
	processor StreamEventProcessor
}

func (a *streamDecoderAdapter) Decode(ctx context.Context, data []byte) (events.Event, error) {
	// StreamCodec doesn't have simple Decode method, so this is a placeholder
	return nil, fmt.Errorf("direct decode not supported by stream codec")
}

func (a *streamDecoderAdapter) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	// StreamCodec doesn't have simple DecodeMultiple method, so this is a placeholder
	return nil, fmt.Errorf("direct decode multiple not supported by stream codec")
}

func (a *streamDecoderAdapter) ContentType() string {
	return a.codec.ContentType()
}

func (a *streamDecoderAdapter) CanStream() bool {
	return true
}

func (a *streamDecoderAdapter) SupportsStreaming() bool {
	return true
}

func (a *streamDecoderAdapter) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return a.codec.DecodeStream(ctx, input, output)
}

func (a *streamDecoderAdapter) StartStream(ctx context.Context, r io.Reader) error {
	if a.session != nil {
		return a.session.StartDecodingSession(ctx, r)
	}
	return fmt.Errorf("session management not available")
}

func (a *streamDecoderAdapter) ReadEvent(ctx context.Context) (events.Event, error) {
	if a.processor != nil {
		return a.processor.ReadEvent(ctx)
	}
	return nil, fmt.Errorf("event processing not available")
}

func (a *streamDecoderAdapter) EndStream(ctx context.Context) error {
	if a.session != nil {
		return a.session.EndSession(ctx)
	}
	return fmt.Errorf("session management not available")
}

// ==============================================================================
// SIMPLIFIED FACTORY TYPES - Removed Duplication
// ==============================================================================

// DefaultEncoderFactory is deprecated - use DefaultCodecFactory instead
// Simplified to embed DefaultCodecFactory with minimal wrapper
type DefaultEncoderFactory struct {
	*DefaultCodecFactory
}

// DefaultDecoderFactory is deprecated - use DefaultCodecFactory instead  
// Simplified to embed DefaultCodecFactory with minimal wrapper
type DefaultDecoderFactory struct {
	*DefaultCodecFactory
}

// NewDefaultEncoderFactory creates a backward compatibility encoder factory
func NewDefaultEncoderFactory() *DefaultEncoderFactory {
	return &DefaultEncoderFactory{
		DefaultCodecFactory: NewDefaultCodecFactory(),
	}
}

// NewDefaultDecoderFactory creates a backward compatibility decoder factory
func NewDefaultDecoderFactory() *DefaultDecoderFactory {
	return &DefaultDecoderFactory{
		DefaultCodecFactory: NewDefaultCodecFactory(),
	}
}

// NewEncoderFactory creates a new encoder factory (convenience function)
func NewEncoderFactory() *DefaultEncoderFactory {
	return NewDefaultEncoderFactory()
}

// NewDecoderFactory creates a new decoder factory (convenience function)
func NewDecoderFactory() *DefaultDecoderFactory {
	return NewDefaultDecoderFactory()
}

// PluginEncoderFactory is a plugin-based encoder factory for backward compatibility
// Simplified to embed PluginCodecFactory with minimal wrapper
type PluginEncoderFactory struct {
	*PluginCodecFactory
}

// PluginDecoderFactory is a plugin-based decoder factory for backward compatibility
// Simplified to embed PluginCodecFactory with minimal wrapper
type PluginDecoderFactory struct {
	*PluginCodecFactory
}

// NewPluginBasedEncoderFactory creates a new plugin-based encoder factory
func NewPluginBasedEncoderFactory() *PluginEncoderFactory {
	return &PluginEncoderFactory{
		PluginCodecFactory: NewPluginCodecFactory(),
	}
}

// NewPluginBasedDecoderFactory creates a new plugin-based decoder factory
func NewPluginBasedDecoderFactory() *PluginDecoderFactory {
	return &PluginDecoderFactory{
		PluginCodecFactory: NewPluginCodecFactory(),
	}
}

// SupportedEncoders returns supported encoders (implements EncoderFactory interface)
func (f *PluginEncoderFactory) SupportedEncoders() []string {
	return f.SupportedTypes()
}

// SupportedDecoders returns supported decoders (implements DecoderFactory interface)
func (f *PluginDecoderFactory) SupportedDecoders() []string {
	return f.SupportedTypes()
}

// CachingEncoderFactory is a caching encoder factory for backward compatibility
// Simplified to embed CachingCodecFactory with minimal wrapper
type CachingEncoderFactory struct {
	*CachingCodecFactory
}

// CachingDecoderFactory is a caching decoder factory for backward compatibility
// Simplified to embed CachingCodecFactory with minimal wrapper
type CachingDecoderFactory struct {
	*CachingCodecFactory
}

// NewCachingEncoderFactoryWithConcrete creates a caching encoder factory
func NewCachingEncoderFactoryWithConcrete(baseFactory *DefaultEncoderFactory) *CachingEncoderFactory {
	if baseFactory == nil {
		return nil
	}
	return &CachingEncoderFactory{
		CachingCodecFactory: NewCachingCodecFactory(baseFactory.DefaultCodecFactory),
	}
}

// NewCachingDecoderFactoryWithConcrete creates a caching decoder factory
func NewCachingDecoderFactoryWithConcrete(baseFactory *DefaultDecoderFactory) *CachingDecoderFactory {
	if baseFactory == nil {
		return nil
	}
	return &CachingDecoderFactory{
		CachingCodecFactory: NewCachingCodecFactory(baseFactory.DefaultCodecFactory),
	}
}

// CreateEncoder creates an encoder using the caching mechanism
func (f *CachingEncoderFactory) CreateEncoder(ctx context.Context, contentType string, options *EncodingOptions) (Encoder, error) {
	codec, err := f.CreateCodec(ctx, contentType, options, nil)
	if err != nil {
		return nil, err
	}
	return &encoderAdapter{codec}, nil
}

// SupportedEncoders returns supported encoders (implements EncoderFactory interface)
func (f *CachingEncoderFactory) SupportedEncoders() []string {
	return f.SupportedTypes()
}

// SupportedDecoders returns supported decoders (implements DecoderFactory interface)
func (f *CachingDecoderFactory) SupportedDecoders() []string {
	return f.SupportedTypes()
}

// CreateEncoder creates an encoder (backward compatibility)
func (f *DefaultEncoderFactory) CreateEncoder(ctx context.Context, contentType string, options *EncodingOptions) (Encoder, error) {
	codec, err := f.CreateCodec(ctx, contentType, options, nil)
	if err != nil {
		return nil, err
	}
	return &encoderAdapter{codec}, nil
}

// CreateStreamEncoder creates a stream encoder (backward compatibility)
func (f *DefaultEncoderFactory) CreateStreamEncoder(ctx context.Context, contentType string, options *EncodingOptions) (StreamEncoder, error) {
	streamCodec, err := f.CreateStreamCodec(ctx, contentType, options, nil)
	if err != nil {
		return nil, err
	}
	return &streamEncoderAdapter{
		codec:     streamCodec,
		session:   nil, // Will be nil unless streamCodec also implements StreamSessionManager
		processor: nil, // Will be nil unless streamCodec also implements StreamEventProcessor
	}, nil
}

// SupportedEncoders returns supported encoders (backward compatibility)
func (f *DefaultEncoderFactory) SupportedEncoders() []string {
	return f.SupportedTypes()
}

// CreateDecoder creates a decoder (backward compatibility)
func (f *DefaultDecoderFactory) CreateDecoder(ctx context.Context, contentType string, options *DecodingOptions) (Decoder, error) {
	codec, err := f.CreateCodec(ctx, contentType, nil, options)
	if err != nil {
		return nil, err
	}
	return &decoderAdapter{codec}, nil
}

// CreateStreamDecoder creates a stream decoder (backward compatibility)
func (f *DefaultDecoderFactory) CreateStreamDecoder(ctx context.Context, contentType string, options *DecodingOptions) (StreamDecoder, error) {
	streamCodec, err := f.CreateStreamCodec(ctx, contentType, nil, options)
	if err != nil {
		return nil, err
	}
	return &streamDecoderAdapter{
		codec:     streamCodec,
		session:   nil, // Will be nil unless streamCodec also implements StreamSessionManager
		processor: nil, // Will be nil unless streamCodec also implements StreamEventProcessor
	}, nil
}

// SupportedDecoders returns supported decoders (backward compatibility)
func (f *DefaultDecoderFactory) SupportedDecoders() []string {
	return f.SupportedTypes()
}

// Additional adapter types for backward compatibility

type encoderWithAllInterfaces struct {
	encoder Encoder
}

func (e *encoderWithAllInterfaces) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return e.encoder.Encode(ctx, event)
}

func (e *encoderWithAllInterfaces) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return e.encoder.EncodeMultiple(ctx, events)
}

func (e *encoderWithAllInterfaces) ContentType() string {
	// Try to get content type from encoder if it implements the interface
	if provider, ok := e.encoder.(ContentTypeProvider); ok {
		return provider.ContentType()
	}
	return "application/octet-stream" // Default
}

func (e *encoderWithAllInterfaces) SupportsStreaming() bool {
	// Try to get streaming capability from encoder if it implements the interface
	if provider, ok := e.encoder.(StreamingCapabilityProvider); ok {
		return provider.SupportsStreaming()
	}
	return false // Default
}

type decoderWithAllInterfaces struct {
	decoder Decoder
}

func (d *decoderWithAllInterfaces) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return d.decoder.Decode(ctx, data)
}

func (d *decoderWithAllInterfaces) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return d.decoder.DecodeMultiple(ctx, data)
}

func (d *decoderWithAllInterfaces) ContentType() string {
	// Try to get content type from decoder if it implements the interface
	if provider, ok := d.decoder.(ContentTypeProvider); ok {
		return provider.ContentType()
	}
	return "application/octet-stream" // Default
}

func (d *decoderWithAllInterfaces) SupportsStreaming() bool {
	// Try to get streaming capability from decoder if it implements the interface
	if provider, ok := d.decoder.(StreamingCapabilityProvider); ok {
		return provider.SupportsStreaming()
	}
	return false // Default
}

// encoderOnlyCodec implements Codec with only encoding support
type encoderOnlyCodec struct {
	encoder interface {
		Encoder
		ContentTypeProvider
		StreamingCapabilityProvider
	}
}

func (c *encoderOnlyCodec) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	if c.encoder == nil {
		return nil, fmt.Errorf("encoder not available")
	}
	return c.encoder.Encode(ctx, event)
}

func (c *encoderOnlyCodec) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	if c.encoder == nil {
		return nil, fmt.Errorf("encoder not available")
	}
	return c.encoder.EncodeMultiple(ctx, events)
}

func (c *encoderOnlyCodec) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return nil, fmt.Errorf("decode operation not supported by encoder-only codec")
}

func (c *encoderOnlyCodec) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return nil, fmt.Errorf("decode operation not supported by encoder-only codec")
}

func (c *encoderOnlyCodec) ContentType() string {
	if c.encoder == nil {
		return ""
	}
	return c.encoder.ContentType()
}

func (c *encoderOnlyCodec) SupportsStreaming() bool {
	if c.encoder == nil {
		return false
	}
	return c.encoder.SupportsStreaming()
}

// decoderOnlyCodec implements Codec with only decoding support
type decoderOnlyCodec struct {
	decoder interface {
		Decoder
		ContentTypeProvider
		StreamingCapabilityProvider
	}
}

func (c *decoderOnlyCodec) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return nil, fmt.Errorf("encode operation not supported by decoder-only codec")
}

func (c *decoderOnlyCodec) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return nil, fmt.Errorf("encode operation not supported by decoder-only codec")
}

func (c *decoderOnlyCodec) Decode(ctx context.Context, data []byte) (events.Event, error) {
	if c.decoder == nil {
		return nil, fmt.Errorf("decoder not available")
	}
	return c.decoder.Decode(ctx, data)
}

func (c *decoderOnlyCodec) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	if c.decoder == nil {
		return nil, fmt.Errorf("decoder not available")
	}
	return c.decoder.DecodeMultiple(ctx, data)
}

func (c *decoderOnlyCodec) ContentType() string {
	if c.decoder == nil {
		return ""
	}
	return c.decoder.ContentType()
}

func (c *decoderOnlyCodec) SupportsStreaming() bool {
	if c.decoder == nil {
		return false
	}
	return c.decoder.SupportsStreaming()
}