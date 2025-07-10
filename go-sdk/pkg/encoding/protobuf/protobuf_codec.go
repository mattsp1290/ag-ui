package protobuf

import (
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

// ProtobufCodec implements the Codec interface for Protocol Buffer encoding/decoding
type ProtobufCodec struct {
	*ProtobufEncoder
	*ProtobufDecoder
}

// NewProtobufCodec creates a new ProtobufCodec with optional configuration
func NewProtobufCodec(encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) *ProtobufCodec {
	return &ProtobufCodec{
		ProtobufEncoder: NewProtobufEncoder(encOptions),
		ProtobufDecoder: NewProtobufDecoder(decOptions),
	}
}

// StreamingProtobufCodec implements StreamCodec for Protocol Buffer format
type StreamingProtobufCodec struct {
	*ProtobufCodec
	streamEncoder *StreamingProtobufEncoder
	streamDecoder *StreamingProtobufDecoder
}

// NewStreamingProtobufCodec creates a new StreamingProtobufCodec
func NewStreamingProtobufCodec(encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) *StreamingProtobufCodec {
	return &StreamingProtobufCodec{
		ProtobufCodec: NewProtobufCodec(encOptions, decOptions),
		streamEncoder: NewStreamingProtobufEncoder(encOptions),
		streamDecoder: NewStreamingProtobufDecoder(decOptions),
	}
}

// GetStreamEncoder returns the stream encoder
func (c *StreamingProtobufCodec) GetStreamEncoder() encoding.StreamEncoder {
	return c.streamEncoder
}

// GetStreamDecoder returns the stream decoder
func (c *StreamingProtobufCodec) GetStreamDecoder() encoding.StreamDecoder {
	return c.streamDecoder
}