package encoding

import (
	"context"
	"runtime"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// PooledEncoder wraps an encoder with pooling capabilities
type PooledEncoder struct {
	encoder     Encoder
	pool        *CodecPool
	contentType string
	putFunc     func(interface{})
}

// Encode encodes a single event
func (pe *PooledEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return pe.encoder.Encode(ctx, event)
}

// EncodeMultiple encodes multiple events efficiently
func (pe *PooledEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return pe.encoder.EncodeMultiple(ctx, events)
}

// ContentType returns the MIME type for this encoder
func (pe *PooledEncoder) ContentType() string {
	return pe.encoder.ContentType()
}

// CanStream indicates if this encoder supports streaming
func (pe *PooledEncoder) CanStream() bool {
	return pe.encoder.CanStream()
}

// SupportsStreaming indicates if this encoder supports streaming
func (pe *PooledEncoder) SupportsStreaming() bool {
	return pe.encoder.SupportsStreaming()
}

// Release returns the encoder to the pool
func (pe *PooledEncoder) Release() {
	if pe.encoder != nil && pe.putFunc != nil {
		pe.putFunc(pe.encoder)
		pe.encoder = nil
	}
}

// Finalizer automatically returns the encoder to the pool when garbage collected
func (pe *PooledEncoder) finalizer() {
	if pe.encoder != nil {
		pe.Release()
	}
}

// NewPooledEncoder creates a new pooled encoder with finalizer
func NewPooledEncoder(encoder Encoder, pool *CodecPool, contentType string, putFunc func(interface{})) *PooledEncoder {
	pe := &PooledEncoder{
		encoder:     encoder,
		pool:        pool,
		contentType: contentType,
		putFunc:     putFunc,
	}
	runtime.SetFinalizer(pe, (*PooledEncoder).finalizer)
	return pe
}

// PooledDecoder wraps a decoder with pooling capabilities
type PooledDecoder struct {
	decoder     Decoder
	pool        *CodecPool
	contentType string
	putFunc     func(interface{})
}

// Decode decodes a single event from data
func (pd *PooledDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return pd.decoder.Decode(ctx, data)
}

// DecodeMultiple decodes multiple events from data
func (pd *PooledDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return pd.decoder.DecodeMultiple(ctx, data)
}

// ContentType returns the MIME type this decoder handles
func (pd *PooledDecoder) ContentType() string {
	return pd.decoder.ContentType()
}

// CanStream indicates if this decoder supports streaming
func (pd *PooledDecoder) CanStream() bool {
	return pd.decoder.CanStream()
}

// SupportsStreaming indicates if this decoder supports streaming
func (pd *PooledDecoder) SupportsStreaming() bool {
	return pd.decoder.SupportsStreaming()
}

// Release returns the decoder to the pool
func (pd *PooledDecoder) Release() {
	if pd.decoder != nil && pd.putFunc != nil {
		pd.putFunc(pd.decoder)
		pd.decoder = nil
	}
}

// Finalizer automatically returns the decoder to the pool when garbage collected
func (pd *PooledDecoder) finalizer() {
	if pd.decoder != nil {
		pd.Release()
	}
}

// NewPooledDecoder creates a new pooled decoder with finalizer
func NewPooledDecoder(decoder Decoder, pool *CodecPool, contentType string, putFunc func(interface{})) *PooledDecoder {
	pd := &PooledDecoder{
		decoder:     decoder,
		pool:        pool,
		contentType: contentType,
		putFunc:     putFunc,
	}
	runtime.SetFinalizer(pd, (*PooledDecoder).finalizer)
	return pd
}

// ReleasableEncoder interface for encoders that can be released back to pools
type ReleasableEncoder interface {
	Encoder
	Release()
}

// ReleasableDecoder interface for decoders that can be released back to pools
type ReleasableDecoder interface {
	Decoder
	Release()
}

// AutoRelease automatically releases a releasable encoder/decoder when the function exits
func AutoRelease(obj interface{}) {
	switch v := obj.(type) {
	case ReleasableEncoder:
		runtime.SetFinalizer(v, func(re ReleasableEncoder) {
			re.Release()
		})
	case ReleasableDecoder:
		runtime.SetFinalizer(v, func(rd ReleasableDecoder) {
			rd.Release()
		})
	}
}

// WithAutoRelease wraps a function to automatically release pooled codecs
func WithAutoRelease(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			panic(r)
		}
	}()
	fn()
}