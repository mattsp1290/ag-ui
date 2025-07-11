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
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.codecCtors[contentType] = ctor
	f.updateSupportedTypes()
}

// RegisterStreamCodec registers a stream codec constructor
func (f *DefaultCodecFactory) RegisterStreamCodec(contentType string, ctor StreamCodecConstructor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.streamCodecCtors[contentType] = ctor
	f.updateSupportedTypes()
}

// CreateCodec creates a codec for the specified content type
func (f *DefaultCodecFactory) CreateCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	ctor, exists := f.codecCtors[contentType]
	if !exists {
		return nil, fmt.Errorf("no codec registered for content type: %s", contentType)
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
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	ctor, exists := f.streamCodecCtors[contentType]
	if !exists {
		return nil, fmt.Errorf("no stream codec registered for content type: %s", contentType)
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
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	types := make([]string, len(f.supportedTypes))
	copy(types, f.supportedTypes)
	return types
}

// SupportsStreaming indicates if streaming is supported for the given content type
func (f *DefaultCodecFactory) SupportsStreaming(contentType string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	_, exists := f.streamCodecCtors[contentType]
	return exists
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
	return &CachingCodecFactory{
		factory: factory,
	}
}

// CreateCodec creates or retrieves a cached codec
func (f *CachingCodecFactory) CreateCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	// Create a cache key from content type and options
	key := f.cacheKey(contentType, encOptions, decOptions)
	
	// Check cache
	if cached, ok := f.cache.Load(key); ok {
		return cached.(Codec), nil
	}
	
	// Create new codec
	codec, err := f.factory.CreateCodec(ctx, contentType, encOptions, decOptions)
	if err != nil {
		return nil, err
	}
	
	// Cache it
	f.cache.Store(key, codec)
	return codec, nil
}

// CreateStreamCodec creates a streaming codec (not cached due to stateful nature)
func (f *CachingCodecFactory) CreateStreamCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (StreamCodec, error) {
	return f.factory.CreateStreamCodec(ctx, contentType, encOptions, decOptions)
}

// SupportedTypes returns list of supported content types
func (f *CachingCodecFactory) SupportedTypes() []string {
	return f.factory.SupportedTypes()
}

// SupportsStreaming indicates if streaming is supported for the given content type
func (f *CachingCodecFactory) SupportsStreaming(contentType string) bool {
	return f.factory.SupportsStreaming(contentType)
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
	codec StreamCodec
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
	return a.codec.StartEncoding(ctx, w)
}

func (a *streamEncoderAdapter) WriteEvent(ctx context.Context, event events.Event) error {
	return a.codec.WriteEvent(ctx, event)
}

func (a *streamEncoderAdapter) EndStream(ctx context.Context) error {
	return a.codec.EndEncoding(ctx)
}

type streamDecoderAdapter struct {
	codec StreamCodec
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
	return a.codec.StartDecoding(ctx, r)
}

func (a *streamDecoderAdapter) ReadEvent(ctx context.Context) (events.Event, error) {
	return a.codec.ReadEvent(ctx)
}

func (a *streamDecoderAdapter) EndStream(ctx context.Context) error {
	return a.codec.EndDecoding(ctx)
}

// DefaultEncoderFactory is deprecated - use DefaultCodecFactory instead
type DefaultEncoderFactory struct {
	*DefaultCodecFactory
}

// DefaultDecoderFactory is deprecated - use DefaultCodecFactory instead
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
	return &streamEncoderAdapter{streamCodec}, nil
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
	return &streamDecoderAdapter{streamCodec}, nil
}

// SupportedDecoders returns supported decoders (backward compatibility)
func (f *DefaultDecoderFactory) SupportedDecoders() []string {
	return f.SupportedTypes()
}

// Note: Adapter types are defined earlier in the file to avoid duplication