package transport

import (
	"fmt"
	"time"
)

// Capabilities describes what a transport can do.
type Capabilities struct {
	// Streaming indicates if the transport supports streaming
	Streaming bool

	// Bidirectional indicates if the transport supports bidirectional communication
	Bidirectional bool

	// Compression indicates if the transport supports compression
	Compression []CompressionType

	// Multiplexing indicates if the transport can multiplex multiple streams
	Multiplexing bool

	// Reconnection indicates if the transport supports automatic reconnection
	Reconnection bool

	// MaxMessageSize is the maximum message size supported (0 for unlimited)
	MaxMessageSize int64

	// Security lists supported security features
	Security []SecurityFeature

	// ProtocolVersion is the version of the transport protocol
	ProtocolVersion string

	// Features contains transport-specific feature flags
	Features map[string]interface{}
}

// FeatureSet is an interface that all feature types must implement.
// It provides a way to validate and describe feature configurations.
type FeatureSet interface {
	// Validate checks if the feature configuration is valid
	Validate() error
	// FeatureType returns the type name of this feature set
	FeatureType() string
}

// TypedCapabilities provides type-safe capabilities with generic features.
// T should be a type that implements FeatureSet.
type TypedCapabilities[T FeatureSet] struct {
	// Streaming indicates if the transport supports streaming
	Streaming bool

	// Bidirectional indicates if the transport supports bidirectional communication
	Bidirectional bool

	// Compression indicates if the transport supports compression
	Compression []CompressionType

	// Multiplexing indicates if the transport can multiplex multiple streams
	Multiplexing bool

	// Reconnection indicates if the transport supports automatic reconnection
	Reconnection bool

	// MaxMessageSize is the maximum message size supported (0 for unlimited)
	MaxMessageSize int64

	// Security lists supported security features
	Security []SecurityFeature

	// ProtocolVersion is the version of the transport protocol
	ProtocolVersion string

	// Features contains type-safe transport-specific features
	Features T
}

// CompressionFeatures contains compression-specific feature configuration.
type CompressionFeatures struct {
	// SupportedAlgorithms lists all supported compression algorithms
	SupportedAlgorithms []CompressionType
	
	// DefaultAlgorithm is the default compression algorithm to use
	DefaultAlgorithm CompressionType
	
	// CompressionLevel specifies the compression level (algorithm-specific)
	CompressionLevel int
	
	// MinSizeThreshold is the minimum message size before compression is applied
	MinSizeThreshold int64
	
	// MaxCompressionRatio is the maximum allowed compression ratio
	MaxCompressionRatio float64
}

// Validate checks if the compression features configuration is valid.
func (cf CompressionFeatures) Validate() error {
	if len(cf.SupportedAlgorithms) == 0 {
		return nil // No compression features required
	}
	
	// Validate default algorithm is in supported list
	defaultFound := false
	for _, alg := range cf.SupportedAlgorithms {
		if alg == cf.DefaultAlgorithm {
			defaultFound = true
			break
		}
	}
	if !defaultFound && cf.DefaultAlgorithm != "" {
		return fmt.Errorf("default algorithm %s not in supported algorithms", cf.DefaultAlgorithm)
	}
	
	if cf.CompressionLevel < 0 {
		return fmt.Errorf("compression level cannot be negative")
	}
	
	if cf.MinSizeThreshold < 0 {
		return fmt.Errorf("min size threshold cannot be negative")
	}
	
	if cf.MaxCompressionRatio < 0 {
		return fmt.Errorf("max compression ratio cannot be negative")
	}
	
	return nil
}

// FeatureType returns the type name of this feature set.
func (cf CompressionFeatures) FeatureType() string {
	return "compression"
}

// SecurityFeatures contains security-specific feature configuration.
type SecurityFeatures struct {
	// SupportedFeatures lists all supported security features
	SupportedFeatures []SecurityFeature
	
	// DefaultFeature is the default security feature to use
	DefaultFeature SecurityFeature
	
	// TLSConfig contains TLS-specific configuration
	TLSConfig *TLSConfig
	
	// JWTConfig contains JWT-specific configuration
	JWTConfig *JWTConfig
	
	// APIKeyConfig contains API key-specific configuration
	APIKeyConfig *APIKeyConfig
	
	// OAuth2Config contains OAuth2-specific configuration
	OAuth2Config *OAuth2Config
}

// TLSConfig contains TLS-specific security configuration.
type TLSConfig struct {
	MinVersion string
	MaxVersion string
	CipherSuites []string
	RequireClientCert bool
}

// JWTConfig contains JWT-specific security configuration.
type JWTConfig struct {
	Algorithm string
	Issuer string
	Audience string
	ExpirationTime time.Duration
}

// APIKeyConfig contains API key-specific security configuration.
type APIKeyConfig struct {
	HeaderName string
	QueryParam string
	RequireHTTPS bool
}

// OAuth2Config contains OAuth2-specific security configuration.
type OAuth2Config struct {
	ClientID string
	Scopes []string
	TokenURL string
	AuthURL string
}

// Validate checks if the security features configuration is valid.
func (sf SecurityFeatures) Validate() error {
	if len(sf.SupportedFeatures) == 0 {
		return nil // No security features required
	}
	
	// Validate default feature is in supported list
	defaultFound := false
	for _, feat := range sf.SupportedFeatures {
		if feat == sf.DefaultFeature {
			defaultFound = true
			break
		}
	}
	if !defaultFound && sf.DefaultFeature != "" {
		return fmt.Errorf("default security feature %s not in supported features", sf.DefaultFeature)
	}
	
	// Validate specific configurations if present
	for _, feat := range sf.SupportedFeatures {
		switch feat {
		case SecurityTLS, SecurityMTLS:
			if sf.TLSConfig != nil {
				if sf.TLSConfig.MinVersion == "" {
					return fmt.Errorf("TLS min version is required")
				}
			}
		case SecurityJWT:
			if sf.JWTConfig != nil {
				if sf.JWTConfig.Algorithm == "" {
					return fmt.Errorf("JWT algorithm is required")
				}
			}
		case SecurityAPIKey:
			if sf.APIKeyConfig != nil {
				if sf.APIKeyConfig.HeaderName == "" && sf.APIKeyConfig.QueryParam == "" {
					return fmt.Errorf("API key requires either header name or query param")
				}
			}
		case SecurityOAuth2:
			if sf.OAuth2Config != nil {
				if sf.OAuth2Config.ClientID == "" {
					return fmt.Errorf("OAuth2 client ID is required")
				}
			}
		}
	}
	
	return nil
}

// FeatureType returns the type name of this feature set.
func (sf SecurityFeatures) FeatureType() string {
	return "security"
}

// StreamingFeatures contains streaming-specific feature configuration.
type StreamingFeatures struct {
	// MaxConcurrentStreams is the maximum number of concurrent streams
	MaxConcurrentStreams int
	
	// StreamTimeout is the timeout for individual streams
	StreamTimeout time.Duration
	
	// BufferSize is the buffer size for streaming data
	BufferSize int
	
	// FlowControlEnabled indicates if flow control is enabled
	FlowControlEnabled bool
	
	// WindowSize is the flow control window size
	WindowSize int
	
	// KeepAliveInterval is the interval for keep-alive messages
	KeepAliveInterval time.Duration
	
	// CompressionPerStream indicates if compression can be applied per stream
	CompressionPerStream bool
}

// Validate checks if the streaming features configuration is valid.
func (sf StreamingFeatures) Validate() error {
	if sf.MaxConcurrentStreams < 0 {
		return fmt.Errorf("max concurrent streams cannot be negative")
	}
	
	if sf.StreamTimeout < 0 {
		return fmt.Errorf("stream timeout cannot be negative")
	}
	
	if sf.BufferSize < 0 {
		return fmt.Errorf("buffer size cannot be negative")
	}
	
	if sf.WindowSize < 0 {
		return fmt.Errorf("window size cannot be negative")
	}
	
	if sf.KeepAliveInterval < 0 {
		return fmt.Errorf("keep alive interval cannot be negative")
	}
	
	return nil
}

// FeatureType returns the type name of this feature set.
func (sf StreamingFeatures) FeatureType() string {
	return "streaming"
}

// CustomFeatures provides backward compatibility for transport-specific features.
// It wraps the original map[string]interface{} for custom feature configurations.
type CustomFeatures struct {
	// Features contains transport-specific feature flags
	Features map[string]interface{}
}

// Validate checks if the custom features configuration is valid.
// Since custom features are transport-specific, this always returns nil.
func (cf CustomFeatures) Validate() error {
	return nil // Custom features validation is transport-specific
}

// FeatureType returns the type name of this feature set.
func (cf CustomFeatures) FeatureType() string {
	return "custom"
}

// CompressionType represents supported compression algorithms.
type CompressionType string

const (
	CompressionNone   CompressionType = "none"
	CompressionGzip   CompressionType = "gzip"
	CompressionZstd   CompressionType = "zstd"
	CompressionSnappy CompressionType = "snappy"
	CompressionBrotli CompressionType = "brotli"
)

// SecurityFeature represents supported security features.
type SecurityFeature string

const (
	SecurityTLS      SecurityFeature = "tls"
	SecurityMTLS     SecurityFeature = "mtls"
	SecurityJWT      SecurityFeature = "jwt"
	SecurityAPIKey   SecurityFeature = "api-key"
	SecurityOAuth2   SecurityFeature = "oauth2"
	SecurityCustom   SecurityFeature = "custom"
)

// Metrics contains performance metrics for a transport.
type Metrics struct {
	// ConnectionUptime is how long the connection has been established
	ConnectionUptime time.Duration

	// MessagesSent is the total number of messages sent
	MessagesSent uint64

	// MessagesReceived is the total number of messages received
	MessagesReceived uint64

	// BytesSent is the total number of bytes sent
	BytesSent uint64

	// BytesReceived is the total number of bytes received
	BytesReceived uint64

	// ErrorCount is the total number of errors encountered
	ErrorCount uint64

	// AverageLatency is the average message latency
	AverageLatency time.Duration

	// CurrentThroughput is the current throughput in messages per second
	CurrentThroughput float64

	// ReconnectCount is the number of reconnection attempts
	ReconnectCount uint64

	// LastError contains the last error encountered
	LastError error

	// LastErrorTime is when the last error occurred
	LastErrorTime time.Time
}

// ReconnectStrategy defines how reconnection should be handled.
type ReconnectStrategy struct {
	// MaxAttempts is the maximum number of reconnection attempts (0 for infinite)
	MaxAttempts int

	// InitialDelay is the initial delay between reconnection attempts
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between reconnection attempts
	MaxDelay time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64

	// Jitter adds randomness to reconnection delays
	Jitter bool
}

// Conversion utilities between old and new types

// ToTypedCapabilities converts a legacy Capabilities struct to TypedCapabilities with CustomFeatures.
func ToTypedCapabilities(cap Capabilities) TypedCapabilities[CustomFeatures] {
	return TypedCapabilities[CustomFeatures]{
		Streaming:        cap.Streaming,
		Bidirectional:    cap.Bidirectional,
		Compression:      cap.Compression,
		Multiplexing:     cap.Multiplexing,
		Reconnection:     cap.Reconnection,
		MaxMessageSize:   cap.MaxMessageSize,
		Security:         cap.Security,
		ProtocolVersion:  cap.ProtocolVersion,
		Features: CustomFeatures{
			Features: cap.Features,
		},
	}
}

// ToCapabilities converts TypedCapabilities back to legacy Capabilities struct.
func ToCapabilities[T FeatureSet](typedCap TypedCapabilities[T]) Capabilities {
	var features map[string]interface{}
	
	// Extract features based on type
	switch f := any(typedCap.Features).(type) {
	case CustomFeatures:
		features = f.Features
	case CompressionFeatures:
		features = map[string]interface{}{
			"compression_supported_algorithms": f.SupportedAlgorithms,
			"compression_default_algorithm":    f.DefaultAlgorithm,
			"compression_level":                f.CompressionLevel,
			"compression_min_size_threshold":   f.MinSizeThreshold,
			"compression_max_ratio":            f.MaxCompressionRatio,
		}
	case SecurityFeatures:
		features = map[string]interface{}{
			"security_supported_features": f.SupportedFeatures,
			"security_default_feature":    f.DefaultFeature,
			"security_tls_config":         f.TLSConfig,
			"security_jwt_config":         f.JWTConfig,
			"security_apikey_config":      f.APIKeyConfig,
			"security_oauth2_config":      f.OAuth2Config,
		}
	case StreamingFeatures:
		features = map[string]interface{}{
			"streaming_max_concurrent_streams": f.MaxConcurrentStreams,
			"streaming_timeout":                f.StreamTimeout,
			"streaming_buffer_size":            f.BufferSize,
			"streaming_flow_control_enabled":   f.FlowControlEnabled,
			"streaming_window_size":            f.WindowSize,
			"streaming_keep_alive_interval":    f.KeepAliveInterval,
			"streaming_compression_per_stream": f.CompressionPerStream,
		}
	default:
		features = make(map[string]interface{})
	}
	
	return Capabilities{
		Streaming:        typedCap.Streaming,
		Bidirectional:    typedCap.Bidirectional,
		Compression:      typedCap.Compression,
		Multiplexing:     typedCap.Multiplexing,
		Reconnection:     typedCap.Reconnection,
		MaxMessageSize:   typedCap.MaxMessageSize,
		Security:         typedCap.Security,
		ProtocolVersion:  typedCap.ProtocolVersion,
		Features:         features,
	}
}

// NewCompressionCapabilities creates TypedCapabilities with CompressionFeatures.
func NewCompressionCapabilities(base Capabilities, compressionFeatures CompressionFeatures) TypedCapabilities[CompressionFeatures] {
	return TypedCapabilities[CompressionFeatures]{
		Streaming:       base.Streaming,
		Bidirectional:   base.Bidirectional,
		Compression:     base.Compression,
		Multiplexing:    base.Multiplexing,
		Reconnection:    base.Reconnection,
		MaxMessageSize:  base.MaxMessageSize,
		Security:        base.Security,
		ProtocolVersion: base.ProtocolVersion,
		Features:        compressionFeatures,
	}
}

// NewSecurityCapabilities creates TypedCapabilities with SecurityFeatures.
func NewSecurityCapabilities(base Capabilities, securityFeatures SecurityFeatures) TypedCapabilities[SecurityFeatures] {
	return TypedCapabilities[SecurityFeatures]{
		Streaming:       base.Streaming,
		Bidirectional:   base.Bidirectional,
		Compression:     base.Compression,
		Multiplexing:    base.Multiplexing,
		Reconnection:    base.Reconnection,
		MaxMessageSize:  base.MaxMessageSize,
		Security:        base.Security,
		ProtocolVersion: base.ProtocolVersion,
		Features:        securityFeatures,
	}
}

// NewStreamingCapabilities creates TypedCapabilities with StreamingFeatures.
func NewStreamingCapabilities(base Capabilities, streamingFeatures StreamingFeatures) TypedCapabilities[StreamingFeatures] {
	return TypedCapabilities[StreamingFeatures]{
		Streaming:       base.Streaming,
		Bidirectional:   base.Bidirectional,
		Compression:     base.Compression,
		Multiplexing:    base.Multiplexing,
		Reconnection:    base.Reconnection,
		MaxMessageSize:  base.MaxMessageSize,
		Security:        base.Security,
		ProtocolVersion: base.ProtocolVersion,
		Features:        streamingFeatures,
	}
}

// ValidateCapabilities validates TypedCapabilities by calling the Validate method on Features.
func ValidateCapabilities[T FeatureSet](cap TypedCapabilities[T]) error {
	return cap.Features.Validate()
}