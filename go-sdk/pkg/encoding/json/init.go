package json

import (
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

func init() {
	registry := encoding.GetGlobalRegistry()
	
	// Register JSON format info
	formatInfo := encoding.JSONFormatInfo()
	if err := registry.RegisterFormat(formatInfo); err != nil {
		panic("failed to register JSON format: " + err.Error())
	}
	
	// Create factory
	factory := &jsonCodecFactory{}
	
	// Register the codec factory
	if err := registry.RegisterCodec("application/json", factory); err != nil {
		panic("failed to register JSON codec: " + err.Error())
	}
}

// jsonCodecFactory implements encoding.CodecFactory for JSON
type jsonCodecFactory struct{}

// CreateEncoder creates a JSON encoder
func (f *jsonCodecFactory) CreateEncoder(contentType string, options *encoding.EncodingOptions) (encoding.Encoder, error) {
	return NewJSONEncoder(options), nil
}

// CreateStreamEncoder creates a streaming JSON encoder
func (f *jsonCodecFactory) CreateStreamEncoder(contentType string, options *encoding.EncodingOptions) (encoding.StreamEncoder, error) {
	return NewStreamingJSONEncoder(options), nil
}

// SupportedEncoders returns list of supported encoder types
func (f *jsonCodecFactory) SupportedEncoders() []string {
	return []string{"application/json"}
}

// CreateDecoder creates a JSON decoder
func (f *jsonCodecFactory) CreateDecoder(contentType string, options *encoding.DecodingOptions) (encoding.Decoder, error) {
	return NewJSONDecoder(options), nil
}

// CreateStreamDecoder creates a streaming JSON decoder
func (f *jsonCodecFactory) CreateStreamDecoder(contentType string, options *encoding.DecodingOptions) (encoding.StreamDecoder, error) {
	return NewStreamingJSONDecoder(options), nil
}

// SupportedDecoders returns list of supported decoder types
func (f *jsonCodecFactory) SupportedDecoders() []string {
	return []string{"application/json"}
}