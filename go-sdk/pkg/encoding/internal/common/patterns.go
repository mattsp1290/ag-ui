package common

import (
	"context"
	"reflect"
	"time"
)

// EncodingResult represents the result of an encoding operation
type EncodingResult struct {
	Data     []byte
	Metadata map[string]interface{}
	Error    error
}

// DecodingResult represents the result of a decoding operation
type DecodingResult struct {
	Value    interface{}
	Metadata map[string]interface{}
	Error    error
}

// Encoder interface defines the contract for encoding operations
type Encoder interface {
	// Encode encodes a value to bytes
	Encode(value interface{}) ([]byte, error)
	
	// EncodeWithContext encodes a value with context
	EncodeWithContext(ctx context.Context, value interface{}) ([]byte, error)
}

// Decoder interface defines the contract for decoding operations
type Decoder interface {
	// Decode decodes bytes to a value
	Decode(data []byte, target interface{}) error
	
	// DecodeWithContext decodes bytes with context
	DecodeWithContext(ctx context.Context, data []byte, target interface{}) error
}

// Codec combines encoding and decoding capabilities
type Codec interface {
	Encoder
	Decoder
}

// TypedEncoder provides type-safe encoding for specific types
type TypedEncoder[T any] interface {
	Encode(value T) ([]byte, error)
	EncodeWithContext(ctx context.Context, value T) ([]byte, error)
}

// TypedDecoder provides type-safe decoding for specific types
type TypedDecoder[T any] interface {
	Decode(data []byte) (T, error)
	DecodeWithContext(ctx context.Context, data []byte) (T, error)
}

// TypedCodec combines typed encoding and decoding
type TypedCodec[T any] interface {
	TypedEncoder[T]
	TypedDecoder[T]
}

// AsyncEncoder performs encoding asynchronously
type AsyncEncoder interface {
	// EncodeAsync encodes a value asynchronously
	EncodeAsync(value interface{}) <-chan EncodingResult
	
	// EncodeAsyncWithContext encodes a value asynchronously with context
	EncodeAsyncWithContext(ctx context.Context, value interface{}) <-chan EncodingResult
}

// AsyncDecoder performs decoding asynchronously
type AsyncDecoder interface {
	// DecodeAsync decodes bytes asynchronously
	DecodeAsync(data []byte, target interface{}) <-chan DecodingResult
	
	// DecodeAsyncWithContext decodes bytes asynchronously with context
	DecodeAsyncWithContext(ctx context.Context, data []byte, target interface{}) <-chan DecodingResult
}

// StreamEncoder encodes data in streaming fashion
type StreamEncoder interface {
	// EncodeStream encodes a channel of values
	EncodeStream(values <-chan interface{}) <-chan EncodingResult
	
	// EncodeStreamWithContext encodes a channel of values with context
	EncodeStreamWithContext(ctx context.Context, values <-chan interface{}) <-chan EncodingResult
}

// StreamDecoder decodes data in streaming fashion
type StreamDecoder interface {
	// DecodeStream decodes a channel of byte slices
	DecodeStream(data <-chan []byte) <-chan DecodingResult
	
	// DecodeStreamWithContext decodes a channel of byte slices with context
	DecodeStreamWithContext(ctx context.Context, data <-chan []byte) <-chan DecodingResult
}

// ValidatingEncoder validates data before encoding
type ValidatingEncoder interface {
	Encoder
	// Validate validates a value before encoding
	Validate(value interface{}) error
}

// ValidatingDecoder validates data after decoding
type ValidatingDecoder interface {
	Decoder
	// ValidateDecoded validates a decoded value
	ValidateDecoded(value interface{}) error
}

// MetadataProvider provides metadata about encoding/decoding operations
type MetadataProvider interface {
	// GetMetadata returns metadata for the given value
	GetMetadata(value interface{}) map[string]interface{}
	
	// SetMetadata sets metadata for the given value
	SetMetadata(value interface{}, metadata map[string]interface{})
}

// VersionedEncoder supports versioned encoding
type VersionedEncoder interface {
	// EncodeVersion encodes a value with version information
	EncodeVersion(value interface{}, version int) ([]byte, error)
	
	// GetVersion returns the current version
	GetVersion() int
}

// VersionedDecoder supports versioned decoding
type VersionedDecoder interface {
	// DecodeVersion decodes versioned data
	DecodeVersion(data []byte, target interface{}) (int, error)
	
	// SupportsVersion checks if a version is supported
	SupportsVersion(version int) bool
}

// CompatibilityChecker checks compatibility between versions
type CompatibilityChecker interface {
	// IsCompatible checks if two versions are compatible
	IsCompatible(version1, version2 int) bool
	
	// GetCompatibilityInfo returns compatibility information
	GetCompatibilityInfo(version int) map[string]interface{}
}

// PerformanceTracker tracks encoding/decoding performance
type PerformanceTracker interface {
	// RecordEncoding records encoding performance metrics
	RecordEncoding(size int, duration time.Duration, success bool)
	
	// RecordDecoding records decoding performance metrics
	RecordDecoding(size int, duration time.Duration, success bool)
	
	// GetStats returns performance statistics
	GetStats() map[string]interface{}
}

// ConfigurableCodec allows runtime configuration
type ConfigurableCodec interface {
	Codec
	// Configure applies configuration options
	Configure(options map[string]interface{}) error
	
	// GetConfiguration returns current configuration
	GetConfiguration() map[string]interface{}
}

// CompressionCodec provides compression capabilities
type CompressionCodec interface {
	Codec
	// Compress compresses data
	Compress(data []byte) ([]byte, error)
	
	// Decompress decompresses data
	Decompress(data []byte) ([]byte, error)
	
	// GetCompressionRatio returns the compression ratio
	GetCompressionRatio() float64
}

// EncryptionCodec provides encryption capabilities
type EncryptionCodec interface {
	Codec
	// Encrypt encrypts data
	Encrypt(data []byte) ([]byte, error)
	
	// Decrypt decrypts data
	Decrypt(data []byte) ([]byte, error)
	
	// SetKey sets the encryption key
	SetKey(key []byte) error
}

// Registry manages codec registration and discovery
type Registry interface {
	// RegisterCodec registers a codec for a type
	RegisterCodec(codecType reflect.Type, codec Codec) error
	
	// GetCodec retrieves a codec for a type
	GetCodec(codecType reflect.Type) (Codec, error)
	
	// ListCodecs lists all registered codecs
	ListCodecs() []reflect.Type
	
	// UnregisterCodec removes a codec registration
	UnregisterCodec(codecType reflect.Type) error
}

// Factory creates codec instances
type Factory interface {
	// CreateEncoder creates an encoder for a type
	CreateEncoder(codecType reflect.Type) (Encoder, error)
	
	// CreateDecoder creates a decoder for a type
	CreateDecoder(codecType reflect.Type) (Decoder, error)
	
	// CreateCodec creates a codec for a type
	CreateCodec(codecType reflect.Type) (Codec, error)
}

// Builder helps build complex codecs
type Builder interface {
	// WithCompression adds compression capability
	WithCompression(algorithm string) Builder
	
	// WithEncryption adds encryption capability
	WithEncryption(algorithm string) Builder
	
	// WithValidation adds validation capability
	WithValidation(rules []ValidationRule) Builder
	
	// WithMetadata adds metadata support
	WithMetadata(provider MetadataProvider) Builder
	
	// Build creates the final codec
	Build() (Codec, error)
}

// Middleware allows chaining of encoding/decoding operations
type Middleware interface {
	// ProcessEncode processes encoding operation
	ProcessEncode(next Encoder) Encoder
	
	// ProcessDecode processes decoding operation
	ProcessDecode(next Decoder) Decoder
}

// Chain manages middleware chain
type Chain interface {
	// Add adds middleware to the chain
	Add(middleware Middleware) Chain
	
	// Build builds the final codec with all middleware applied
	Build(base Codec) Codec
}

// Context provides shared context for encoding/decoding operations
type Context struct {
	Values   map[string]interface{}
	Metadata map[string]interface{}
	Timeout  time.Duration
}

// NewContext creates a new context
func NewContext() *Context {
	return &Context{
		Values:   make(map[string]interface{}),
		Metadata: make(map[string]interface{}),
		Timeout:  30 * time.Second,
	}
}

// WithValue sets a value in the context
func (c *Context) WithValue(key string, value interface{}) *Context {
	c.Values[key] = value
	return c
}

// GetValue retrieves a value from the context
func (c *Context) GetValue(key string) (interface{}, bool) {
	value, exists := c.Values[key]
	return value, exists
}

// WithMetadata sets metadata in the context
func (c *Context) WithMetadata(key string, value interface{}) *Context {
	c.Metadata[key] = value
	return c
}

// GetMetadata retrieves metadata from the context
func (c *Context) GetMetadata(key string) (interface{}, bool) {
	value, exists := c.Metadata[key]
	return value, exists
}

// WithTimeout sets the timeout for operations
func (c *Context) WithTimeout(timeout time.Duration) *Context {
	c.Timeout = timeout
	return c
}

// GetTimeout returns the operation timeout
func (c *Context) GetTimeout() time.Duration {
	return c.Timeout
}