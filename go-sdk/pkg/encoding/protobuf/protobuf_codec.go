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
}

// NewStreamingProtobufCodec creates a new StreamingProtobufCodec
func NewStreamingProtobufCodec(encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) *StreamingProtobufCodec {
	return &StreamingProtobufCodec{
		ProtobufCodec: NewProtobufCodec(encOptions, decOptions),
	}
}

// GetStreamEncoder returns a new stream encoder instance
func (c *StreamingProtobufCodec) GetStreamEncoder() encoding.StreamEncoder {
	return NewStreamingProtobufEncoder(c.ProtobufEncoder.options)
}

// GetStreamDecoder returns a new stream decoder instance
func (c *StreamingProtobufCodec) GetStreamDecoder() encoding.StreamDecoder {
	return NewStreamingProtobufDecoder(c.ProtobufDecoder.options)
}