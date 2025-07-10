package encoding

import (
	"context"
	"io"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// Encoder defines the interface for encoding events into various formats
type Encoder interface {
	// Encode encodes a single event
	Encode(event events.Event) ([]byte, error)

	// EncodeMultiple encodes multiple events efficiently
	EncodeMultiple(events []events.Event) ([]byte, error)

	// ContentType returns the MIME type for this encoder
	ContentType() string

	// CanStream indicates if this encoder supports streaming
	CanStream() bool
}

// Decoder defines the interface for decoding events from various formats
type Decoder interface {
	// Decode decodes a single event from raw data
	Decode(data []byte) (events.Event, error)

	// DecodeMultiple decodes multiple events from raw data
	DecodeMultiple(data []byte) ([]events.Event, error)

	// ContentType returns the MIME type this decoder handles
	ContentType() string

	// CanStream indicates if this decoder supports streaming
	CanStream() bool
}

// StreamEncoder defines the interface for streaming event encoding
type StreamEncoder interface {
	Encoder

	// EncodeStream encodes events from a channel to a writer
	EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error

	// StartStream initializes a streaming session
	StartStream(w io.Writer) error

	// WriteEvent writes a single event to the stream
	WriteEvent(event events.Event) error

	// EndStream finalizes the streaming session
	EndStream() error
}

// StreamDecoder defines the interface for streaming event decoding
type StreamDecoder interface {
	Decoder

	// DecodeStream decodes events from a reader to a channel
	DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error

	// StartStream initializes a streaming session
	StartStream(r io.Reader) error

	// ReadEvent reads a single event from the stream
	ReadEvent() (events.Event, error)

	// EndStream finalizes the streaming session
	EndStream() error
}

// Codec combines both encoder and decoder interfaces
type Codec interface {
	Encoder
	Decoder
}

// StreamCodec combines codec with streaming capabilities
// Note: Does not embed StreamEncoder/StreamDecoder due to method conflicts
type StreamCodec interface {
	Codec
	
	// GetStreamEncoder returns the stream encoder
	GetStreamEncoder() StreamEncoder
	
	// GetStreamDecoder returns the stream decoder  
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

// EncoderFactory creates encoders for specific content types
type EncoderFactory interface {
	// CreateEncoder creates an encoder for the specified content type
	CreateEncoder(contentType string, options *EncodingOptions) (Encoder, error)

	// CreateStreamEncoder creates a streaming encoder
	CreateStreamEncoder(contentType string, options *EncodingOptions) (StreamEncoder, error)

	// SupportedEncoders returns list of supported encoder types
	SupportedEncoders() []string
}

// DecoderFactory creates decoders for specific content types
type DecoderFactory interface {
	// CreateDecoder creates a decoder for the specified content type
	CreateDecoder(contentType string, options *DecodingOptions) (Decoder, error)

	// CreateStreamDecoder creates a streaming decoder
	CreateStreamDecoder(contentType string, options *DecodingOptions) (StreamDecoder, error)

	// SupportedDecoders returns list of supported decoder types
	SupportedDecoders() []string
}

// CodecFactory combines encoder and decoder factories
type CodecFactory interface {
	EncoderFactory
	DecoderFactory
}