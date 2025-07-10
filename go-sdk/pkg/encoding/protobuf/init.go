package protobuf

import (
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

func init() {
	registry := encoding.GetGlobalRegistry()
	
	// Register Protobuf format info
	formatInfo := encoding.ProtobufFormatInfo()
	if err := registry.RegisterFormat(formatInfo); err != nil {
		panic("failed to register Protobuf format: " + err.Error())
	}
	
	// Create factory
	factory := &protobufCodecFactory{}
	
	// Register the codec factory
	if err := registry.RegisterCodec("application/x-protobuf", factory); err != nil {
		panic("failed to register Protobuf codec: " + err.Error())
	}
}

// protobufCodecFactory implements encoding.CodecFactory for Protobuf
type protobufCodecFactory struct{}

// CreateEncoder creates a Protobuf encoder
func (f *protobufCodecFactory) CreateEncoder(contentType string, options *encoding.EncodingOptions) (encoding.Encoder, error) {
	return NewProtobufEncoder(options), nil
}

// CreateStreamEncoder creates a streaming Protobuf encoder
func (f *protobufCodecFactory) CreateStreamEncoder(contentType string, options *encoding.EncodingOptions) (encoding.StreamEncoder, error) {
	return NewStreamingProtobufEncoder(options), nil
}

// SupportedEncoders returns list of supported encoder types
func (f *protobufCodecFactory) SupportedEncoders() []string {
	return []string{"application/x-protobuf"}
}

// CreateDecoder creates a Protobuf decoder
func (f *protobufCodecFactory) CreateDecoder(contentType string, options *encoding.DecodingOptions) (encoding.Decoder, error) {
	return NewProtobufDecoder(options), nil
}

// CreateStreamDecoder creates a streaming Protobuf decoder
func (f *protobufCodecFactory) CreateStreamDecoder(contentType string, options *encoding.DecodingOptions) (encoding.StreamDecoder, error) {
	return NewStreamingProtobufDecoder(options), nil
}

// SupportedDecoders returns list of supported decoder types
func (f *protobufCodecFactory) SupportedDecoders() []string {
	return []string{"application/x-protobuf"}
}