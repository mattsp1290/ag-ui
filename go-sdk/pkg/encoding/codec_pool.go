package encoding

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// CodecPool manages pools for encoder and decoder instances
type CodecPool struct {
	jsonEncoderPool    sync.Pool
	jsonDecoderPool    sync.Pool
	protobufEncoderPool sync.Pool
	protobufDecoderPool sync.Pool
	metrics            PoolMetrics
}

// NewCodecPool creates a new codec pool
func NewCodecPool() *CodecPool {
	cp := &CodecPool{}
	
	// Initialize JSON encoder pool - will be set up by factory
	cp.jsonEncoderPool.New = func() interface{} {
		atomic.AddInt64(&cp.metrics.News, 1)
		return nil // Will be overridden by factory
	}
	
	// Initialize JSON decoder pool - will be set up by factory
	cp.jsonDecoderPool.New = func() interface{} {
		atomic.AddInt64(&cp.metrics.News, 1)
		return nil // Will be overridden by factory
	}
	
	// Initialize Protobuf encoder pool - will be set up by factory
	cp.protobufEncoderPool.New = func() interface{} {
		atomic.AddInt64(&cp.metrics.News, 1)
		return nil // Will be overridden by factory
	}
	
	// Initialize Protobuf decoder pool - will be set up by factory
	cp.protobufDecoderPool.New = func() interface{} {
		atomic.AddInt64(&cp.metrics.News, 1)
		return nil // Will be overridden by factory
	}
	
	return cp
}

// SetJSONEncoderConstructor sets the constructor for JSON encoders
func (cp *CodecPool) SetJSONEncoderConstructor(constructor func() interface{}) {
	cp.jsonEncoderPool.New = func() interface{} {
		atomic.AddInt64(&cp.metrics.News, 1)
		return constructor()
	}
}

// SetJSONDecoderConstructor sets the constructor for JSON decoders
func (cp *CodecPool) SetJSONDecoderConstructor(constructor func() interface{}) {
	cp.jsonDecoderPool.New = func() interface{} {
		atomic.AddInt64(&cp.metrics.News, 1)
		return constructor()
	}
}

// SetProtobufEncoderConstructor sets the constructor for Protobuf encoders
func (cp *CodecPool) SetProtobufEncoderConstructor(constructor func() interface{}) {
	cp.protobufEncoderPool.New = func() interface{} {
		atomic.AddInt64(&cp.metrics.News, 1)
		return constructor()
	}
}

// SetProtobufDecoderConstructor sets the constructor for Protobuf decoders
func (cp *CodecPool) SetProtobufDecoderConstructor(constructor func() interface{}) {
	cp.protobufDecoderPool.New = func() interface{} {
		atomic.AddInt64(&cp.metrics.News, 1)
		return constructor()
	}
}

// GetJSONEncoder retrieves a JSON encoder from the pool
func (cp *CodecPool) GetJSONEncoder(options *EncodingOptions) interface{} {
	atomic.AddInt64(&cp.metrics.Gets, 1)
	encoder := cp.jsonEncoderPool.Get()
	// Note: Reset method call will be handled by the caller
	return encoder
}

// PutJSONEncoder returns a JSON encoder to the pool
func (cp *CodecPool) PutJSONEncoder(encoder interface{}) {
	if encoder == nil {
		return
	}
	atomic.AddInt64(&cp.metrics.Puts, 1)
	atomic.AddInt64(&cp.metrics.Resets, 1)
	// Note: Reset method call will be handled by the caller
	cp.jsonEncoderPool.Put(encoder)
}

// GetJSONDecoder retrieves a JSON decoder from the pool
func (cp *CodecPool) GetJSONDecoder(options *DecodingOptions) interface{} {
	atomic.AddInt64(&cp.metrics.Gets, 1)
	decoder := cp.jsonDecoderPool.Get()
	// Note: Reset method call will be handled by the caller
	return decoder
}

// PutJSONDecoder returns a JSON decoder to the pool
func (cp *CodecPool) PutJSONDecoder(decoder interface{}) {
	if decoder == nil {
		return
	}
	atomic.AddInt64(&cp.metrics.Puts, 1)
	atomic.AddInt64(&cp.metrics.Resets, 1)
	// Note: Reset method call will be handled by the caller
	cp.jsonDecoderPool.Put(decoder)
}

// GetProtobufEncoder retrieves a Protobuf encoder from the pool
func (cp *CodecPool) GetProtobufEncoder(options *EncodingOptions) interface{} {
	atomic.AddInt64(&cp.metrics.Gets, 1)
	encoder := cp.protobufEncoderPool.Get()
	// Note: Reset method call will be handled by the caller
	return encoder
}

// PutProtobufEncoder returns a Protobuf encoder to the pool
func (cp *CodecPool) PutProtobufEncoder(encoder interface{}) {
	if encoder == nil {
		return
	}
	atomic.AddInt64(&cp.metrics.Puts, 1)
	atomic.AddInt64(&cp.metrics.Resets, 1)
	// Note: Reset method call will be handled by the caller
	cp.protobufEncoderPool.Put(encoder)
}

// GetProtobufDecoder retrieves a Protobuf decoder from the pool
func (cp *CodecPool) GetProtobufDecoder(options *DecodingOptions) interface{} {
	atomic.AddInt64(&cp.metrics.Gets, 1)
	decoder := cp.protobufDecoderPool.Get()
	// Note: Reset method call will be handled by the caller
	return decoder
}

// PutProtobufDecoder returns a Protobuf decoder to the pool
func (cp *CodecPool) PutProtobufDecoder(decoder interface{}) {
	if decoder == nil {
		return
	}
	atomic.AddInt64(&cp.metrics.Puts, 1)
	atomic.AddInt64(&cp.metrics.Resets, 1)
	// Note: Reset method call will be handled by the caller
	cp.protobufDecoderPool.Put(decoder)
}

// Metrics returns pool metrics
func (cp *CodecPool) Metrics() PoolMetrics {
	return PoolMetrics{
		Gets:   atomic.LoadInt64(&cp.metrics.Gets),
		Puts:   atomic.LoadInt64(&cp.metrics.Puts),
		News:   atomic.LoadInt64(&cp.metrics.News),
		Resets: atomic.LoadInt64(&cp.metrics.Resets),
	}
}

// Reset clears the pool
func (cp *CodecPool) Reset() {
	cp.jsonEncoderPool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&cp.metrics.News, 1)
			return nil // Will be overridden by factory
		},
	}
	cp.jsonDecoderPool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&cp.metrics.News, 1)
			return nil // Will be overridden by factory
		},
	}
	cp.protobufEncoderPool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&cp.metrics.News, 1)
			return nil // Will be overridden by factory
		},
	}
	cp.protobufDecoderPool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&cp.metrics.News, 1)
			return nil // Will be overridden by factory
		},
	}
	atomic.StoreInt64(&cp.metrics.Gets, 0)
	atomic.StoreInt64(&cp.metrics.Puts, 0)
	atomic.StoreInt64(&cp.metrics.News, 0)
	atomic.StoreInt64(&cp.metrics.Resets, 0)
}

// PooledCodecFactory wraps a codec factory with pooling capabilities
type PooledCodecFactory struct {
	factory   *DefaultCodecFactory
	codecPool *CodecPool
}

// NewPooledCodecFactory creates a new pooled codec factory
func NewPooledCodecFactory() *PooledCodecFactory {
	return &PooledCodecFactory{
		factory:   NewDefaultCodecFactory(),
		codecPool: NewCodecPool(),
	}
}

// CreateCodec creates a pooled codec
func (pcf *PooledCodecFactory) CreateCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	switch contentType {
	case "application/json":
		// Try to get cached encoder/decoder components
		encoderInterface := pcf.codecPool.GetJSONEncoder(encOptions)
		decoderInterface := pcf.codecPool.GetJSONDecoder(decOptions)
		
		if encoderInterface != nil && decoderInterface != nil {
			// Create a composite codec from cached components
			encoder := encoderInterface.(Encoder)
			decoder := decoderInterface.(Decoder)
			return &compositeCodec{
				encoder: encoder,
				decoder: decoder,
			}, nil
		}
		
		// Fall back to factory
		return pcf.factory.CreateCodec(ctx, contentType, encOptions, decOptions)
	
	case "application/x-protobuf":
		// Try to get cached encoder/decoder components
		encoderInterface := pcf.codecPool.GetProtobufEncoder(encOptions)
		decoderInterface := pcf.codecPool.GetProtobufDecoder(decOptions)
		
		if encoderInterface != nil && decoderInterface != nil {
			// Create a composite codec from cached components
			encoder := encoderInterface.(Encoder)
			decoder := decoderInterface.(Decoder)
			return &compositeCodec{
				encoder: encoder,
				decoder: decoder,
			}, nil
		}
		
		// Fall back to factory
		return pcf.factory.CreateCodec(ctx, contentType, encOptions, decOptions)
	
	default:
		return pcf.factory.CreateCodec(ctx, contentType, encOptions, decOptions)
	}
}

// CreateStreamCodec creates a streaming codec
func (pcf *PooledCodecFactory) CreateStreamCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (StreamCodec, error) {
	// Streaming codecs are not pooled as they maintain state
	return pcf.factory.CreateStreamCodec(ctx, contentType, encOptions, decOptions)
}

// SupportedTypes returns supported content types
func (pcf *PooledCodecFactory) SupportedTypes() []string {
	return pcf.factory.SupportedTypes()
}

// SupportsStreaming indicates if streaming is supported for the given content type
func (pcf *PooledCodecFactory) SupportsStreaming(contentType string) bool {
	return pcf.factory.SupportsStreaming(contentType)
}

// compositeCodec combines cached encoder and decoder components
type compositeCodec struct {
	encoder Encoder
	decoder Decoder
}

func (c *compositeCodec) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return c.encoder.Encode(ctx, event)
}

func (c *compositeCodec) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return c.encoder.EncodeMultiple(ctx, events)
}

func (c *compositeCodec) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return c.decoder.Decode(ctx, data)
}

func (c *compositeCodec) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return c.decoder.DecodeMultiple(ctx, data)
}

func (c *compositeCodec) ContentType() string {
	return c.encoder.ContentType()
}

func (c *compositeCodec) SupportsStreaming() bool {
	return c.encoder.CanStream() && c.decoder.CanStream()
}

func (c *compositeCodec) CanStream() bool {
	return c.SupportsStreaming()
}

// RegisterCodec registers a codec with the factory
func (pcf *PooledCodecFactory) RegisterCodec(contentType string, codecCtor CodecConstructor) {
	pcf.factory.RegisterCodec(contentType, codecCtor)
}

// RegisterStreamCodec registers a stream codec with the factory
func (pcf *PooledCodecFactory) RegisterStreamCodec(contentType string, streamCodecCtor StreamCodecConstructor) {
	pcf.factory.RegisterStreamCodec(contentType, streamCodecCtor)
}

// GetCodecPool returns the codec pool for metrics
func (pcf *PooledCodecFactory) GetCodecPool() *CodecPool {
	return pcf.codecPool
}

// Global pooled codec factory
var globalCodecPool = NewCodecPool()

// GetGlobalCodecPool returns the global codec pool
func GetGlobalCodecPool() *CodecPool {
	return globalCodecPool
}