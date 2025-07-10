package encoding

import (
	"fmt"
	"sync"
)

// DefaultEncoderFactory implements EncoderFactory with plugin support
type DefaultEncoderFactory struct {
	mu sync.RWMutex
	
	// Constructor functions for encoders
	encoderCtors map[string]EncoderConstructor
	
	// Constructor functions for stream encoders
	streamEncoderCtors map[string]StreamEncoderConstructor
	
	// Supported content types
	supportedTypes []string
}

// DefaultDecoderFactory implements DecoderFactory with plugin support
type DefaultDecoderFactory struct {
	mu sync.RWMutex
	
	// Constructor functions for decoders
	decoderCtors map[string]DecoderConstructor
	
	// Constructor functions for stream decoders
	streamDecoderCtors map[string]StreamDecoderConstructor
	
	// Supported content types
	supportedTypes []string
}

// DefaultCodecFactory implements CodecFactory
type DefaultCodecFactory struct {
	encoderFactory *DefaultEncoderFactory
	decoderFactory *DefaultDecoderFactory
}

// Constructor function types
type (
	EncoderConstructor       func(options *EncodingOptions) (Encoder, error)
	StreamEncoderConstructor func(options *EncodingOptions) (StreamEncoder, error)
	DecoderConstructor       func(options *DecodingOptions) (Decoder, error)
	StreamDecoderConstructor func(options *DecodingOptions) (StreamDecoder, error)
)

// NewDefaultEncoderFactory creates a new encoder factory
func NewDefaultEncoderFactory() *DefaultEncoderFactory {
	return &DefaultEncoderFactory{
		encoderCtors:       make(map[string]EncoderConstructor),
		streamEncoderCtors: make(map[string]StreamEncoderConstructor),
		supportedTypes:     []string{},
	}
}

// RegisterEncoder registers an encoder constructor
func (f *DefaultEncoderFactory) RegisterEncoder(contentType string, ctor EncoderConstructor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.encoderCtors[contentType] = ctor
	f.updateSupportedTypes()
}

// RegisterStreamEncoder registers a stream encoder constructor
func (f *DefaultEncoderFactory) RegisterStreamEncoder(contentType string, ctor StreamEncoderConstructor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.streamEncoderCtors[contentType] = ctor
	f.updateSupportedTypes()
}

// CreateEncoder creates an encoder for the specified content type
func (f *DefaultEncoderFactory) CreateEncoder(contentType string, options *EncodingOptions) (Encoder, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	ctor, exists := f.encoderCtors[contentType]
	if !exists {
		return nil, fmt.Errorf("no encoder registered for content type: %s", contentType)
	}
	
	if options == nil {
		options = &EncodingOptions{}
	}
	
	return ctor(options)
}

// CreateStreamEncoder creates a streaming encoder
func (f *DefaultEncoderFactory) CreateStreamEncoder(contentType string, options *EncodingOptions) (StreamEncoder, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	ctor, exists := f.streamEncoderCtors[contentType]
	if !exists {
		return nil, fmt.Errorf("no stream encoder registered for content type: %s", contentType)
	}
	
	if options == nil {
		options = &EncodingOptions{}
	}
	
	return ctor(options)
}

// SupportedEncoders returns list of supported encoder types
func (f *DefaultEncoderFactory) SupportedEncoders() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	types := make([]string, len(f.supportedTypes))
	copy(types, f.supportedTypes)
	return types
}

// updateSupportedTypes updates the list of supported types
func (f *DefaultEncoderFactory) updateSupportedTypes() {
	typeMap := make(map[string]bool)
	
	for contentType := range f.encoderCtors {
		typeMap[contentType] = true
	}
	
	for contentType := range f.streamEncoderCtors {
		typeMap[contentType] = true
	}
	
	f.supportedTypes = make([]string, 0, len(typeMap))
	for contentType := range typeMap {
		f.supportedTypes = append(f.supportedTypes, contentType)
	}
}

// NewDefaultDecoderFactory creates a new decoder factory
func NewDefaultDecoderFactory() *DefaultDecoderFactory {
	return &DefaultDecoderFactory{
		decoderCtors:       make(map[string]DecoderConstructor),
		streamDecoderCtors: make(map[string]StreamDecoderConstructor),
		supportedTypes:     []string{},
	}
}

// RegisterDecoder registers a decoder constructor
func (f *DefaultDecoderFactory) RegisterDecoder(contentType string, ctor DecoderConstructor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.decoderCtors[contentType] = ctor
	f.updateSupportedTypes()
}

// RegisterStreamDecoder registers a stream decoder constructor
func (f *DefaultDecoderFactory) RegisterStreamDecoder(contentType string, ctor StreamDecoderConstructor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.streamDecoderCtors[contentType] = ctor
	f.updateSupportedTypes()
}

// CreateDecoder creates a decoder for the specified content type
func (f *DefaultDecoderFactory) CreateDecoder(contentType string, options *DecodingOptions) (Decoder, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	ctor, exists := f.decoderCtors[contentType]
	if !exists {
		return nil, fmt.Errorf("no decoder registered for content type: %s", contentType)
	}
	
	if options == nil {
		options = &DecodingOptions{}
	}
	
	return ctor(options)
}

// CreateStreamDecoder creates a streaming decoder
func (f *DefaultDecoderFactory) CreateStreamDecoder(contentType string, options *DecodingOptions) (StreamDecoder, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	ctor, exists := f.streamDecoderCtors[contentType]
	if !exists {
		return nil, fmt.Errorf("no stream decoder registered for content type: %s", contentType)
	}
	
	if options == nil {
		options = &DecodingOptions{}
	}
	
	return ctor(options)
}

// SupportedDecoders returns list of supported decoder types
func (f *DefaultDecoderFactory) SupportedDecoders() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	types := make([]string, len(f.supportedTypes))
	copy(types, f.supportedTypes)
	return types
}

// updateSupportedTypes updates the list of supported types
func (f *DefaultDecoderFactory) updateSupportedTypes() {
	typeMap := make(map[string]bool)
	
	for contentType := range f.decoderCtors {
		typeMap[contentType] = true
	}
	
	for contentType := range f.streamDecoderCtors {
		typeMap[contentType] = true
	}
	
	f.supportedTypes = make([]string, 0, len(typeMap))
	for contentType := range typeMap {
		f.supportedTypes = append(f.supportedTypes, contentType)
	}
}

// NewDefaultCodecFactory creates a new codec factory
func NewDefaultCodecFactory() *DefaultCodecFactory {
	return &DefaultCodecFactory{
		encoderFactory: NewDefaultEncoderFactory(),
		decoderFactory: NewDefaultDecoderFactory(),
	}
}

// CreateEncoder creates an encoder for the specified content type
func (f *DefaultCodecFactory) CreateEncoder(contentType string, options *EncodingOptions) (Encoder, error) {
	return f.encoderFactory.CreateEncoder(contentType, options)
}

// CreateStreamEncoder creates a streaming encoder
func (f *DefaultCodecFactory) CreateStreamEncoder(contentType string, options *EncodingOptions) (StreamEncoder, error) {
	return f.encoderFactory.CreateStreamEncoder(contentType, options)
}

// SupportedEncoders returns list of supported encoder types
func (f *DefaultCodecFactory) SupportedEncoders() []string {
	return f.encoderFactory.SupportedEncoders()
}

// CreateDecoder creates a decoder for the specified content type
func (f *DefaultCodecFactory) CreateDecoder(contentType string, options *DecodingOptions) (Decoder, error) {
	return f.decoderFactory.CreateDecoder(contentType, options)
}

// CreateStreamDecoder creates a streaming decoder
func (f *DefaultCodecFactory) CreateStreamDecoder(contentType string, options *DecodingOptions) (StreamDecoder, error) {
	return f.decoderFactory.CreateStreamDecoder(contentType, options)
}

// SupportedDecoders returns list of supported decoder types
func (f *DefaultCodecFactory) SupportedDecoders() []string {
	return f.decoderFactory.SupportedDecoders()
}

// RegisterCodec registers both encoder and decoder constructors
func (f *DefaultCodecFactory) RegisterCodec(
	contentType string,
	encoderCtor EncoderConstructor,
	decoderCtor DecoderConstructor,
	streamEncoderCtor StreamEncoderConstructor,
	streamDecoderCtor StreamDecoderConstructor,
) {
	f.encoderFactory.RegisterEncoder(contentType, encoderCtor)
	f.decoderFactory.RegisterDecoder(contentType, decoderCtor)
	
	if streamEncoderCtor != nil {
		f.encoderFactory.RegisterStreamEncoder(contentType, streamEncoderCtor)
	}
	
	if streamDecoderCtor != nil {
		f.decoderFactory.RegisterStreamDecoder(contentType, streamDecoderCtor)
	}
}

// PluginEncoderFactory allows plugins to register encoders
type PluginEncoderFactory struct {
	*DefaultEncoderFactory
	plugins map[string]EncoderPlugin
}

// PluginDecoderFactory allows plugins to register decoders
type PluginDecoderFactory struct {
	*DefaultDecoderFactory
	plugins map[string]DecoderPlugin
}

// EncoderPlugin interface for encoder plugins
type EncoderPlugin interface {
	// Name returns the plugin name
	Name() string
	
	// ContentTypes returns supported content types
	ContentTypes() []string
	
	// CreateEncoder creates an encoder
	CreateEncoder(contentType string, options *EncodingOptions) (Encoder, error)
	
	// CreateStreamEncoder creates a stream encoder
	CreateStreamEncoder(contentType string, options *EncodingOptions) (StreamEncoder, error)
}

// DecoderPlugin interface for decoder plugins
type DecoderPlugin interface {
	// Name returns the plugin name
	Name() string
	
	// ContentTypes returns supported content types
	ContentTypes() []string
	
	// CreateDecoder creates a decoder
	CreateDecoder(contentType string, options *DecodingOptions) (Decoder, error)
	
	// CreateStreamDecoder creates a stream decoder
	CreateStreamDecoder(contentType string, options *DecodingOptions) (StreamDecoder, error)
}

// NewPluginEncoderFactory creates a new plugin-enabled encoder factory
func NewPluginEncoderFactory() *PluginEncoderFactory {
	return &PluginEncoderFactory{
		DefaultEncoderFactory: NewDefaultEncoderFactory(),
		plugins:              make(map[string]EncoderPlugin),
	}
}

// RegisterPlugin registers an encoder plugin
func (f *PluginEncoderFactory) RegisterPlugin(plugin EncoderPlugin) error {
	if plugin == nil {
		return fmt.Errorf("plugin cannot be nil")
	}
	
	name := plugin.Name()
	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.plugins[name] = plugin
	
	// Register constructors for each content type
	for _, contentType := range plugin.ContentTypes() {
		f.encoderCtors[contentType] = func(options *EncodingOptions) (Encoder, error) {
			return plugin.CreateEncoder(contentType, options)
		}
		
		f.streamEncoderCtors[contentType] = func(options *EncodingOptions) (StreamEncoder, error) {
			return plugin.CreateStreamEncoder(contentType, options)
		}
	}
	
	f.updateSupportedTypes()
	return nil
}

// NewPluginDecoderFactory creates a new plugin-enabled decoder factory
func NewPluginDecoderFactory() *PluginDecoderFactory {
	return &PluginDecoderFactory{
		DefaultDecoderFactory: NewDefaultDecoderFactory(),
		plugins:              make(map[string]DecoderPlugin),
	}
}

// RegisterPlugin registers a decoder plugin
func (f *PluginDecoderFactory) RegisterPlugin(plugin DecoderPlugin) error {
	if plugin == nil {
		return fmt.Errorf("plugin cannot be nil")
	}
	
	name := plugin.Name()
	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.plugins[name] = plugin
	
	// Register constructors for each content type
	for _, contentType := range plugin.ContentTypes() {
		f.decoderCtors[contentType] = func(options *DecodingOptions) (Decoder, error) {
			return plugin.CreateDecoder(contentType, options)
		}
		
		f.streamDecoderCtors[contentType] = func(options *DecodingOptions) (StreamDecoder, error) {
			return plugin.CreateStreamDecoder(contentType, options)
		}
	}
	
	f.updateSupportedTypes()
	return nil
}

// CachingEncoderFactory wraps a factory with caching
type CachingEncoderFactory struct {
	factory EncoderFactory
	cache   sync.Map // map[string]Encoder
}

// NewCachingEncoderFactory creates a new caching encoder factory
func NewCachingEncoderFactory(factory EncoderFactory) *CachingEncoderFactory {
	return &CachingEncoderFactory{
		factory: factory,
	}
}

// CreateEncoder creates or retrieves a cached encoder
func (f *CachingEncoderFactory) CreateEncoder(contentType string, options *EncodingOptions) (Encoder, error) {
	// Create a cache key from content type and options
	key := f.cacheKey(contentType, options)
	
	// Check cache
	if cached, ok := f.cache.Load(key); ok {
		return cached.(Encoder), nil
	}
	
	// Create new encoder
	encoder, err := f.factory.CreateEncoder(contentType, options)
	if err != nil {
		return nil, err
	}
	
	// Cache it
	f.cache.Store(key, encoder)
	return encoder, nil
}

// CreateStreamEncoder creates a streaming encoder (not cached)
func (f *CachingEncoderFactory) CreateStreamEncoder(contentType string, options *EncodingOptions) (StreamEncoder, error) {
	return f.factory.CreateStreamEncoder(contentType, options)
}

// SupportedEncoders returns list of supported encoder types
func (f *CachingEncoderFactory) SupportedEncoders() []string {
	return f.factory.SupportedEncoders()
}

// cacheKey generates a cache key from content type and options
func (f *CachingEncoderFactory) cacheKey(contentType string, options *EncodingOptions) string {
	if options == nil {
		return contentType
	}
	
	// Include relevant options in the key
	return fmt.Sprintf("%s:%v:%s:%d:%v:%v",
		contentType,
		options.Pretty,
		options.Compression,
		options.BufferSize,
		options.ValidateOutput,
		options.CrossSDKCompatibility,
	)
}