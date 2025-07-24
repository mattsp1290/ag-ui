package protobuf

import (
	"context"
	"fmt"
	"io"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

// ProtobufCodec implements the new Codec interface for Protocol Buffer encoding/decoding
// It properly composes the focused Encoder, Decoder, and ContentTypeProvider interfaces
type ProtobufCodec struct {
	*ProtobufEncoder
	*ProtobufDecoder
}

// Ensure ProtobufCodec implements the core interfaces
var (
	_ encoding.Encoder               = (*ProtobufCodec)(nil)
	_ encoding.Decoder               = (*ProtobufCodec)(nil)
	_ encoding.ContentTypeProvider   = (*ProtobufCodec)(nil)
	_ encoding.Codec                 = (*ProtobufCodec)(nil)
	_ encoding.LegacyEncoder         = (*ProtobufCodec)(nil)
	_ encoding.LegacyDecoder         = (*ProtobufCodec)(nil)
)

// NewProtobufCodec creates a new ProtobufCodec with optional configuration
func NewProtobufCodec(encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) *ProtobufCodec {
	return &ProtobufCodec{
		ProtobufEncoder: NewProtobufEncoder(encOptions),
		ProtobufDecoder: NewProtobufDecoder(decOptions),
	}
}

// ContentType returns the MIME type for protobuf
func (c *ProtobufCodec) ContentType() string {
	return "application/x-protobuf"
}

// SupportsStreaming indicates that protobuf codec supports streaming
func (c *ProtobufCodec) SupportsStreaming() bool {
	return true
}

// CanStream indicates that protobuf codec supports streaming (backward compatibility)
func (c *ProtobufCodec) CanStream() bool {
	return c.SupportsStreaming()
}

// StreamingProtobufCodec implements the new StreamCodec interface for Protocol Buffer format
// It composes multiple focused interfaces for complete streaming functionality
type StreamingProtobufCodec struct {
	encoder    *StreamingProtobufEncoder
	decoder    *StreamingProtobufDecoder
	encOptions *encoding.EncodingOptions
	decOptions *encoding.DecodingOptions
}

// Ensure StreamingProtobufCodec implements the streaming interfaces
var (
	_ encoding.StreamCodec                = (*StreamingProtobufCodec)(nil)
	_ encoding.LegacyStreamCodec          = (*StreamingProtobufCodec)(nil)
	_ encoding.StreamSessionManager       = (*StreamingProtobufCodec)(nil)
	_ encoding.StreamEventProcessor       = (*StreamingProtobufCodec)(nil)
)

// NewStreamingProtobufCodec creates a new StreamingProtobufCodec
func NewStreamingProtobufCodec(encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) *StreamingProtobufCodec {
	return &StreamingProtobufCodec{
		encoder:    NewStreamingProtobufEncoder(encOptions),
		decoder:    NewStreamingProtobufDecoder(decOptions),
		encOptions: encOptions,
		decOptions: decOptions,
	}
}

// ContentType returns the MIME type for protobuf streaming
func (c *StreamingProtobufCodec) ContentType() string {
	return "application/x-protobuf-stream"
}

// EncodeStream encodes events from a channel to a writer
func (c *StreamingProtobufCodec) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return c.encoder.EncodeStream(ctx, input, output)
}

// DecodeStream decodes events from a reader to a channel
func (c *StreamingProtobufCodec) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return c.decoder.DecodeStream(ctx, input, output)
}

// StartEncoding initializes a streaming encoding session
func (c *StreamingProtobufCodec) StartEncoding(ctx context.Context, w io.Writer) error {
	return c.encoder.StartStream(ctx, w)
}

// WriteEvent writes a single event to the encoding stream
func (c *StreamingProtobufCodec) WriteEvent(ctx context.Context, event events.Event) error {
	return c.encoder.WriteEvent(ctx, event)
}

// EndEncoding finalizes the streaming encoding session
func (c *StreamingProtobufCodec) EndEncoding(ctx context.Context) error {
	return c.encoder.EndStream(ctx)
}

// StartDecoding initializes a streaming decoding session
func (c *StreamingProtobufCodec) StartDecoding(ctx context.Context, r io.Reader) error {
	return c.decoder.StartStream(ctx, r)
}

// ReadEvent reads a single event from the decoding stream
func (c *StreamingProtobufCodec) ReadEvent(ctx context.Context) (events.Event, error) {
	return c.decoder.ReadEvent(ctx)
}

// EndDecoding finalizes the streaming decoding session
func (c *StreamingProtobufCodec) EndDecoding(ctx context.Context) error {
	return c.decoder.EndStream(ctx)
}


// GetStreamDecoder returns a new stream decoder instance for concurrent use
func (c *StreamingProtobufCodec) GetStreamDecoder() encoding.StreamDecoder {
	return NewStreamingProtobufDecoder(c.decOptions)
}

// GetStreamEncoder returns a new stream encoder instance for concurrent use
func (c *StreamingProtobufCodec) GetStreamEncoder() encoding.StreamEncoder {
	return NewStreamingProtobufEncoder(c.encOptions)
}


// StreamSessionManager interface implementation

// StartEncodingSession initializes a streaming encoding session
func (c *StreamingProtobufCodec) StartEncodingSession(ctx context.Context, w io.Writer) error {
	return c.StartEncoding(ctx, w)
}

// StartDecodingSession initializes a streaming decoding session
func (c *StreamingProtobufCodec) StartDecodingSession(ctx context.Context, r io.Reader) error {
	return c.StartDecoding(ctx, r)
}

// EndSession finalizes the current streaming session
func (c *StreamingProtobufCodec) EndSession(ctx context.Context) error {
	// For protobuf, we need to end both encoding and decoding sessions
	// This is a simplified implementation - in practice you'd track which session is active
	var err error
	if encErr := c.EndEncoding(ctx); encErr != nil {
		err = encErr
	}
	if decErr := c.EndDecoding(ctx); decErr != nil {
		if err != nil {
			return fmt.Errorf("multiple session end errors: %v, %v", err, decErr)
		}
		err = decErr
	}
	return err
}

// Encode implements the Codec interface - single event encoding
func (c *StreamingProtobufCodec) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return c.encoder.Encode(ctx, event)
}

// EncodeMultiple implements the Codec interface - multiple event encoding
func (c *StreamingProtobufCodec) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return c.encoder.EncodeMultiple(ctx, events)
}

// Decode implements the Codec interface - single event decoding
func (c *StreamingProtobufCodec) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return c.decoder.Decode(ctx, data)
}

// DecodeMultiple implements the Codec interface - multiple event decoding
func (c *StreamingProtobufCodec) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return c.decoder.DecodeMultiple(ctx, data)
}

// SupportsStreaming indicates if this codec has streaming capabilities
func (c *StreamingProtobufCodec) SupportsStreaming() bool {
	return true
}

// CanStream indicates if this codec supports streaming (backward compatibility)
func (c *StreamingProtobufCodec) CanStream() bool {
	return true
}