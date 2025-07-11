package encoding

import (
	"context"
	"io"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// Codec defines the core interface for encoding and decoding events
// This replaces the previous Encoder/Decoder split to simplify the interface hierarchy
type Codec interface {
	// Encode encodes a single event
	Encode(ctx context.Context, event events.Event) ([]byte, error)

	// EncodeMultiple encodes multiple events efficiently
	EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error)

	// Decode decodes a single event from raw data
	Decode(ctx context.Context, data []byte) (events.Event, error)

	// DecodeMultiple decodes multiple events from raw data
	DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error)

	// ContentType returns the MIME type for this codec
	ContentType() string

	// SupportsStreaming indicates if this codec has streaming capabilities
	SupportsStreaming() bool
	
	// CanStream indicates if this codec supports streaming (backward compatibility)
	CanStream() bool
}

// StreamCodec defines the interface for streaming event encoding/decoding
// This extends the basic Codec interface to include streaming capabilities
type StreamCodec interface {
	// Embed the basic Codec interface
	Codec

	// EncodeStream encodes events from a channel to a writer
	EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error

	// DecodeStream decodes events from a reader to a channel
	DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error

	// StartEncoding initializes a streaming encoding session
	StartEncoding(ctx context.Context, w io.Writer) error

	// WriteEvent writes a single event to the encoding stream
	WriteEvent(ctx context.Context, event events.Event) error

	// EndEncoding finalizes the streaming encoding session
	EndEncoding(ctx context.Context) error

	// StartDecoding initializes a streaming decoding session
	StartDecoding(ctx context.Context, r io.Reader) error

	// ReadEvent reads a single event from the decoding stream
	ReadEvent(ctx context.Context) (events.Event, error)

	// EndDecoding finalizes the streaming decoding session
	EndDecoding(ctx context.Context) error

	// GetStreamEncoder returns the underlying stream encoder
	GetStreamEncoder() StreamEncoder

	// GetStreamDecoder returns the underlying stream decoder
	GetStreamDecoder() StreamDecoder
}

// EncodingOptions provides options for encoding operations
type EncodingOptions struct {
	// Pretty indicates if output should be formatted for readability
	Pretty bool

	// Compression specifies compression algorithm (e.g., "gzip", "zstd")
	Compression string

	// BufferSize specifies buffer size for streaming operations
	BufferSize int

	// MaxSize specifies maximum encoded size (0 for unlimited)
	MaxSize int64

	// ValidateOutput enables output validation after encoding
	ValidateOutput bool

	// CrossSDKCompatibility ensures compatibility with other SDKs
	CrossSDKCompatibility bool
}

// DecodingOptions provides options for decoding operations
type DecodingOptions struct {
	// Strict enables strict validation during decoding
	Strict bool

	// MaxSize specifies maximum input size to process (0 for unlimited)
	MaxSize int64

	// BufferSize specifies buffer size for streaming operations
	BufferSize int

	// AllowUnknownFields allows unknown fields in the input
	AllowUnknownFields bool

	// ValidateEvents enables event validation after decoding
	ValidateEvents bool
}

// EncodingError represents an error during encoding
type EncodingError struct {
	Format  string
	Event   events.Event
	Message string
	Cause   error
}

func (e *EncodingError) Error() string {
	if e.Cause != nil {
		return "encoding error: " + e.Message + ": " + e.Cause.Error()
	}
	return "encoding error: " + e.Message
}

func (e *EncodingError) Unwrap() error {
	return e.Cause
}

// DecodingError represents an error during decoding
type DecodingError struct {
	Format  string
	Data    []byte
	Message string
	Cause   error
}

func (e *DecodingError) Error() string {
	if e.Cause != nil {
		return "decoding error: " + e.Message + ": " + e.Cause.Error()
	}
	return "decoding error: " + e.Message
}

func (e *DecodingError) Unwrap() error {
	return e.Cause
}

// ValidationError represents a validation error during encoding/decoding
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return "validation error: " + e.Message
}

// ContentNegotiator defines the interface for content type negotiation
type ContentNegotiator interface {
	// Negotiate selects the best content type based on Accept header
	Negotiate(acceptHeader string) (string, error)

	// SupportedTypes returns list of supported content types
	SupportedTypes() []string

	// PreferredType returns the preferred content type
	PreferredType() string

	// CanHandle checks if a content type can be handled
	CanHandle(contentType string) bool
}

// CodecFactory creates codecs for specific content types
type CodecFactory interface {
	// CreateCodec creates a codec for the specified content type
	CreateCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error)

	// CreateStreamCodec creates a streaming codec for the specified content type
	CreateStreamCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (StreamCodec, error)

	// SupportedTypes returns list of supported content types
	SupportedTypes() []string

	// SupportsStreaming indicates if streaming is supported for the given content type
	SupportsStreaming(contentType string) bool
}

// Backward compatibility interfaces - DEPRECATED
// These interfaces are provided for backward compatibility and will be removed in a future version

// Encoder is deprecated - use Codec instead
type Encoder interface {
	Encode(ctx context.Context, event events.Event) ([]byte, error)
	EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error)
	ContentType() string
	CanStream() bool
	SupportsStreaming() bool
}

// Decoder is deprecated - use Codec instead
type Decoder interface {
	Decode(ctx context.Context, data []byte) (events.Event, error)
	DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error)
	ContentType() string
	CanStream() bool
	SupportsStreaming() bool
}

// StreamEncoder is deprecated - use StreamCodec instead
type StreamEncoder interface {
	Encoder
	EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error
	StartStream(ctx context.Context, w io.Writer) error
	WriteEvent(ctx context.Context, event events.Event) error
	EndStream(ctx context.Context) error
}

// StreamDecoder is deprecated - use StreamCodec instead
type StreamDecoder interface {
	Decoder
	DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error
	StartStream(ctx context.Context, r io.Reader) error
	ReadEvent(ctx context.Context) (events.Event, error)
	EndStream(ctx context.Context) error
}