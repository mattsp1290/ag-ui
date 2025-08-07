package protobuf

import (
	"context"
	"sync"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

var (
	// registrationOnce ensures registration happens only once
	registrationOnce sync.Once
	// registrationErr stores any error that occurred during registration
	registrationErr error
)

// init still exists but now calls the explicit registration function
// This maintains backward compatibility for existing code
func init() {
	// Attempt registration but don't panic on failure
	// Users can check the error using EnsureRegistered()
	_ = Register()
}

// Register explicitly registers the Protobuf codec with the global registry.
// It returns an error if registration fails. This function is idempotent
// and thread-safe - multiple calls will only register once.
func Register() error {
	registrationOnce.Do(func() {
		registrationErr = register()
	})
	return registrationErr
}

// EnsureRegistered checks if the Protobuf codec is registered and returns
// any error that occurred during registration. This is useful for
// applications that want to verify successful registration.
func EnsureRegistered() error {
	return Register()
}

// RegisterTo registers the Protobuf codec with a specific registry.
// This allows for custom registry configurations and testing.
func RegisterTo(registry *encoding.FormatRegistry) error {
	if registry == nil {
		return errors.NewEncodingError("PROTOBUF_NIL_REGISTRY", "registry cannot be nil").WithOperation("register")
	}

	// Register Protobuf format info
	formatInfo := encoding.ProtobufFormatInfo()
	if err := registry.RegisterFormat(formatInfo); err != nil {
		return errors.NewEncodingError("PROTOBUF_FORMAT_REGISTRATION_FAILED", "failed to register Protobuf format").WithOperation("register").WithCause(err)
	}

	// Create factory
	factory := &protobufCodecFactory{}

	// Register the codec factory
	if err := registry.RegisterCodec("application/x-protobuf", factory); err != nil {
		return errors.NewEncodingError("PROTOBUF_CODEC_REGISTRATION_FAILED", "failed to register Protobuf codec").WithOperation("register").WithCause(err)
	}

	return nil
}

// register performs the actual registration with the global registry
func register() error {
	registry := encoding.GetGlobalRegistry()
	return RegisterTo(registry)
}

// protobufCodecFactory implements encoding.CodecFactory for Protobuf
type protobufCodecFactory struct{}

// CreateCodec creates a Protobuf codec
func (f *protobufCodecFactory) CreateCodec(ctx context.Context, contentType string, encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) (encoding.Codec, error) {
	return NewProtobufCodec(encOptions, decOptions), nil
}

// CreateStreamCodec creates a streaming Protobuf codec
func (f *protobufCodecFactory) CreateStreamCodec(ctx context.Context, contentType string, encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) (encoding.StreamCodec, error) {
	return NewStreamingProtobufCodec(encOptions, decOptions), nil
}

// SupportedTypes returns list of supported content types
func (f *protobufCodecFactory) SupportedTypes() []string {
	return []string{"application/x-protobuf"}
}

// SupportsStreaming indicates if streaming is supported for the given content type
func (f *protobufCodecFactory) SupportsStreaming(contentType string) bool {
	return true
}

// CreateEncoder creates a Protobuf encoder
func (f *protobufCodecFactory) CreateEncoder(ctx context.Context, contentType string, options *encoding.EncodingOptions) (encoding.Encoder, error) {
	return NewProtobufEncoder(options), nil
}

// CreateStreamEncoder creates a streaming Protobuf encoder
func (f *protobufCodecFactory) CreateStreamEncoder(ctx context.Context, contentType string, options *encoding.EncodingOptions) (encoding.StreamEncoder, error) {
	return NewStreamingProtobufEncoder(options), nil
}

// SupportedEncoders returns list of supported encoder types
func (f *protobufCodecFactory) SupportedEncoders() []string {
	return []string{"application/x-protobuf"}
}

// CreateDecoder creates a Protobuf decoder
func (f *protobufCodecFactory) CreateDecoder(ctx context.Context, contentType string, options *encoding.DecodingOptions) (encoding.Decoder, error) {
	return NewProtobufDecoder(options), nil
}

// CreateStreamDecoder creates a streaming Protobuf decoder
func (f *protobufCodecFactory) CreateStreamDecoder(ctx context.Context, contentType string, options *encoding.DecodingOptions) (encoding.StreamDecoder, error) {
	return NewStreamingProtobufDecoder(options), nil
}

// SupportedDecoders returns list of supported decoder types
func (f *protobufCodecFactory) SupportedDecoders() []string {
	return []string{"application/x-protobuf"}
}
