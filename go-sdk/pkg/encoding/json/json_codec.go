package json

import (
	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

// JSONCodec implements the Codec interface combining JSON encoder and decoder
type JSONCodec struct {
	*JSONEncoder
	*JSONDecoder
}

// NewJSONCodec creates a new JSON codec with the given options
func NewJSONCodec(encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) *JSONCodec {
	return &JSONCodec{
		JSONEncoder: NewJSONEncoder(encOptions),
		JSONDecoder: NewJSONDecoder(decOptions),
	}
}

// NewDefaultJSONCodec creates a new JSON codec with default options
func NewDefaultJSONCodec() *JSONCodec {
	return NewJSONCodec(
		&encoding.EncodingOptions{
			CrossSDKCompatibility: true,
			ValidateOutput:        true,
		},
		&encoding.DecodingOptions{
			Strict:         true,
			ValidateEvents: true,
		},
	)
}

// Encode delegates to the encoder
func (c *JSONCodec) Encode(event events.Event) ([]byte, error) {
	return c.JSONEncoder.Encode(event)
}

// EncodeMultiple delegates to the encoder
func (c *JSONCodec) EncodeMultiple(events []events.Event) ([]byte, error) {
	return c.JSONEncoder.EncodeMultiple(events)
}

// Decode delegates to the decoder
func (c *JSONCodec) Decode(data []byte) (events.Event, error) {
	return c.JSONDecoder.Decode(data)
}

// DecodeMultiple delegates to the decoder
func (c *JSONCodec) DecodeMultiple(data []byte) ([]events.Event, error) {
	return c.JSONDecoder.DecodeMultiple(data)
}

// ContentType returns the MIME type for JSON
func (c *JSONCodec) ContentType() string {
	return "application/json"
}

// CanStream indicates that JSON codec supports streaming
func (c *JSONCodec) CanStream() bool {
	return true
}

// CodecOptions provides combined options for JSON codec
type CodecOptions struct {
	EncodingOptions *encoding.EncodingOptions
	DecodingOptions *encoding.DecodingOptions
}

// DefaultCodecOptions returns default codec options
func DefaultCodecOptions() *CodecOptions {
	return &CodecOptions{
		EncodingOptions: &encoding.EncodingOptions{
			CrossSDKCompatibility: true,
			ValidateOutput:        true,
			Pretty:                false,
			BufferSize:            4096,
		},
		DecodingOptions: &encoding.DecodingOptions{
			Strict:             true,
			ValidateEvents:     true,
			AllowUnknownFields: false,
			BufferSize:         4096,
		},
	}
}

// PrettyCodecOptions returns codec options for pretty-printed JSON
func PrettyCodecOptions() *CodecOptions {
	opts := DefaultCodecOptions()
	opts.EncodingOptions.Pretty = true
	return opts
}

// CompatibilityCodecOptions returns codec options optimized for cross-SDK compatibility
func CompatibilityCodecOptions() *CodecOptions {
	return &CodecOptions{
		EncodingOptions: &encoding.EncodingOptions{
			CrossSDKCompatibility: true,
			ValidateOutput:        true,
			Pretty:                false,
			BufferSize:            8192,
		},
		DecodingOptions: &encoding.DecodingOptions{
			Strict:             false,
			ValidateEvents:     true,
			AllowUnknownFields: true,
			BufferSize:         8192,
		},
	}
}

// StreamingCodecOptions returns codec options optimized for streaming
func StreamingCodecOptions() *CodecOptions {
	return &CodecOptions{
		EncodingOptions: &encoding.EncodingOptions{
			CrossSDKCompatibility: true,
			ValidateOutput:        false, // Skip validation for performance
			Pretty:                false,
			BufferSize:            16384, // Larger buffer for streaming
		},
		DecodingOptions: &encoding.DecodingOptions{
			Strict:             false,
			ValidateEvents:     false, // Skip validation for performance
			AllowUnknownFields: true,
			BufferSize:         16384, // Larger buffer for streaming
		},
	}
}