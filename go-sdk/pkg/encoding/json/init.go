package json

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

// Register explicitly registers the JSON codec with the global registry.
// It returns an error if registration fails. This function is idempotent
// and thread-safe - multiple calls will only register once.
func Register() error {
	registrationOnce.Do(func() {
		registrationErr = register()
	})
	return registrationErr
}

// EnsureRegistered checks if the JSON codec is registered and returns
// any error that occurred during registration. This is useful for
// applications that want to verify successful registration.
func EnsureRegistered() error {
	return Register()
}

// RegisterTo registers the JSON codec with a specific registry.
// This allows for custom registry configurations and testing.
func RegisterTo(registry *encoding.FormatRegistry) error {
	if registry == nil {
		return errors.NewEncodingError("JSON_NIL_REGISTRY", "registry cannot be nil").WithOperation("register")
	}

	// Register JSON format info
	formatInfo := encoding.JSONFormatInfo()
	if err := registry.RegisterFormat(formatInfo); err != nil {
		return errors.NewEncodingError("JSON_FORMAT_REGISTRATION_FAILED", "failed to register JSON format").WithOperation("register").WithCause(err)
	}

	// Create factory
	factory := &jsonCodecFactory{}

	// Register the codec factory (using legacy interface method for custom implementations)
	if err := registry.RegisterCodec("application/json", factory); err != nil {
		return errors.NewEncodingError("JSON_CODEC_REGISTRATION_FAILED", "failed to register JSON codec").WithOperation("register").WithCause(err)
	}

	// Also register as encoder and decoder factories for full compatibility
	if err := registry.RegisterEncoder("application/json", factory); err != nil {
		return errors.NewEncodingError("JSON_ENCODER_REGISTRATION_FAILED", "failed to register JSON encoder").WithOperation("register").WithCause(err)
	}

	if err := registry.RegisterDecoder("application/json", factory); err != nil {
		return errors.NewEncodingError("JSON_DECODER_REGISTRATION_FAILED", "failed to register JSON decoder").WithOperation("register").WithCause(err)
	}

	return nil
}

// register performs the actual registration with the global registry
func register() error {
	registry := encoding.GetGlobalRegistry()
	return RegisterTo(registry)
}

// jsonCodecFactory implements encoding.CodecFactory for JSON
// It also implements EncoderFactory and DecoderFactory for full compatibility
type jsonCodecFactory struct{}

// CreateCodec creates a JSON codec
func (f *jsonCodecFactory) CreateCodec(ctx context.Context, contentType string, encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) (encoding.Codec, error) {
	return NewJSONCodec(encOptions, decOptions), nil
}

// CreateStreamCodec creates a streaming JSON codec
func (f *jsonCodecFactory) CreateStreamCodec(ctx context.Context, contentType string, encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) (encoding.StreamCodec, error) {
	return NewJSONStreamCodec(encOptions, decOptions), nil
}

// SupportedTypes returns list of supported content types
func (f *jsonCodecFactory) SupportedTypes() []string {
	return []string{"application/json"}
}

// SupportsStreaming indicates if streaming is supported for the given content type
func (f *jsonCodecFactory) SupportsStreaming(contentType string) bool {
	return true
}

// CreateEncoder creates a JSON encoder
func (f *jsonCodecFactory) CreateEncoder(ctx context.Context, contentType string, options *encoding.EncodingOptions) (encoding.Encoder, error) {
	return NewJSONEncoder(options), nil
}

// CreateStreamEncoder creates a streaming JSON encoder
func (f *jsonCodecFactory) CreateStreamEncoder(ctx context.Context, contentType string, options *encoding.EncodingOptions) (encoding.StreamEncoder, error) {
	return NewStreamingJSONEncoder(options), nil
}

// SupportedEncoders returns list of supported encoder types
func (f *jsonCodecFactory) SupportedEncoders() []string {
	return []string{"application/json"}
}

// CreateDecoder creates a JSON decoder
func (f *jsonCodecFactory) CreateDecoder(ctx context.Context, contentType string, options *encoding.DecodingOptions) (encoding.Decoder, error) {
	return NewJSONDecoder(options), nil
}

// CreateStreamDecoder creates a streaming JSON decoder
func (f *jsonCodecFactory) CreateStreamDecoder(ctx context.Context, contentType string, options *encoding.DecodingOptions) (encoding.StreamDecoder, error) {
	return NewStreamingJSONDecoder(options), nil
}

// SupportedDecoders returns list of supported decoder types
func (f *jsonCodecFactory) SupportedDecoders() []string {
	return []string{"application/json"}
}
