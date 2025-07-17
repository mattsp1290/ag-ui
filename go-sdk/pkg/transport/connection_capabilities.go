package transport

import "time"

// ConnectionCapabilities represents negotiated connection capabilities
type ConnectionCapabilities struct {
	// Compression capabilities
	Compression CompressionCapability `json:"compression,omitempty"`
	
	// Streaming capabilities
	Streaming StreamingCapability `json:"streaming,omitempty"`
	
	// Security capabilities
	Security SecurityCapability `json:"security,omitempty"`
	
	// Protocol-specific features
	ProtocolFeatures ProtocolFeatures `json:"protocol_features,omitempty"`
	
	// Maximum message size supported
	MaxMessageSize int64 `json:"max_message_size,omitempty"`
	
	// Heartbeat configuration
	Heartbeat HeartbeatCapability `json:"heartbeat,omitempty"`
	
	// Authentication methods supported
	AuthMethods []string `json:"auth_methods,omitempty"`
	
	// Custom extension capabilities
	Extensions map[string]string `json:"extensions,omitempty"`
}

// CompressionCapability represents compression support
type CompressionCapability struct {
	Supported   bool     `json:"supported"`
	Algorithms  []string `json:"algorithms,omitempty"`
	MinSize     int      `json:"min_size,omitempty"`
	DefaultAlgo string   `json:"default_algorithm,omitempty"`
}

// StreamingCapability represents streaming support
type StreamingCapability struct {
	Supported         bool  `json:"supported"`
	MaxConcurrentStreams int32 `json:"max_concurrent_streams,omitempty"`
	FlowControl       bool  `json:"flow_control,omitempty"`
	Multiplexing      bool  `json:"multiplexing,omitempty"`
}

// SecurityCapability represents security features
type SecurityCapability struct {
	TLSSupported     bool     `json:"tls_supported"`
	TLSVersions      []string `json:"tls_versions,omitempty"`
	CertValidation   bool     `json:"cert_validation"`
	MutualTLS        bool     `json:"mutual_tls,omitempty"`
	TokenAuth        bool     `json:"token_auth,omitempty"`
}

// ProtocolFeatures represents protocol-specific capabilities
type ProtocolFeatures struct {
	WebSocket WebSocketFeatures `json:"websocket,omitempty"`
	HTTP      HTTPFeatures      `json:"http,omitempty"`
	GRPC      GRPCFeatures      `json:"grpc,omitempty"`
}

// WebSocketFeatures represents WebSocket-specific features
type WebSocketFeatures struct {
	Extensions  []string `json:"extensions,omitempty"`
	SubProtocols []string `json:"sub_protocols,omitempty"`
	PingPong    bool     `json:"ping_pong,omitempty"`
}

// HTTPFeatures represents HTTP-specific features
type HTTPFeatures struct {
	Version     string            `json:"version,omitempty"`
	Methods     []string          `json:"methods,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Chunked     bool              `json:"chunked,omitempty"`
	KeepAlive   bool              `json:"keep_alive,omitempty"`
}

// GRPCFeatures represents gRPC-specific features
type GRPCFeatures struct {
	Version         string   `json:"version,omitempty"`
	Reflection      bool     `json:"reflection,omitempty"`
	HealthCheck     bool     `json:"health_check,omitempty"`
	Services        []string `json:"services,omitempty"`
	MaxMessageSize  int64    `json:"max_message_size,omitempty"`
}

// HeartbeatCapability represents heartbeat configuration
type HeartbeatCapability struct {
	Supported bool          `json:"supported"`
	Interval  time.Duration `json:"interval,omitempty"`
	Timeout   time.Duration `json:"timeout,omitempty"`
	Required  bool          `json:"required,omitempty"`
}

// ErrorDetails represents structured error information
type ErrorDetails struct {
	// Error code or identifier
	Code string `json:"code,omitempty"`
	
	// Error category (network, protocol, auth, etc.)
	Category string `json:"category,omitempty"`
	
	// HTTP status code if applicable
	HTTPStatus int `json:"http_status,omitempty"`
	
	// Retry information
	Retry ErrorRetryInfo `json:"retry,omitempty"`
	
	// Context information
	Context ErrorContext `json:"context,omitempty"`
	
	// Related errors or causes
	Causes []ErrorCause `json:"causes,omitempty"`
	
	// Custom fields for specific error types
	Custom map[string]string `json:"custom,omitempty"`
}

// ErrorRetryInfo provides retry guidance
type ErrorRetryInfo struct {
	Retryable    bool          `json:"retryable"`
	RetryAfter   time.Duration `json:"retry_after,omitempty"`
	MaxRetries   int           `json:"max_retries,omitempty"`
	BackoffType  string        `json:"backoff_type,omitempty"`
}

// ErrorContext provides contextual information about an error
type ErrorContext struct {
	RequestID   string            `json:"request_id,omitempty"`
	Operation   string            `json:"operation,omitempty"`
	Resource    string            `json:"resource,omitempty"`
	UserAgent   string            `json:"user_agent,omitempty"`
	RemoteAddr  string            `json:"remote_addr,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"query_params,omitempty"`
}

// ErrorCause represents a related error or cause
type ErrorCause struct {
	Type        string `json:"type"`
	Message     string `json:"message"`
	Source      string `json:"source,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

// ToMap converts ConnectionCapabilities to map[string]interface{} for backward compatibility
func (c ConnectionCapabilities) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	
	if c.MaxMessageSize > 0 {
		result["max_message_size"] = c.MaxMessageSize
	}
	
	if c.Compression.Supported {
		result["compression"] = map[string]interface{}{
			"supported":   c.Compression.Supported,
			"algorithms":  c.Compression.Algorithms,
			"min_size":    c.Compression.MinSize,
			"default":     c.Compression.DefaultAlgo,
		}
	}
	
	if c.Streaming.Supported {
		result["streaming"] = map[string]interface{}{
			"supported":              c.Streaming.Supported,
			"max_concurrent_streams": c.Streaming.MaxConcurrentStreams,
			"flow_control":           c.Streaming.FlowControl,
			"multiplexing":           c.Streaming.Multiplexing,
		}
	}
	
	if len(c.AuthMethods) > 0 {
		result["auth_methods"] = c.AuthMethods
	}
	
	if len(c.Extensions) > 0 {
		result["extensions"] = c.Extensions
	}
	
	return result
}

// ToMap converts ErrorDetails to map[string]interface{} for backward compatibility
func (e ErrorDetails) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	
	if e.Code != "" {
		result["code"] = e.Code
	}
	if e.Category != "" {
		result["category"] = e.Category
	}
	if e.HTTPStatus > 0 {
		result["http_status"] = e.HTTPStatus
	}
	
	if e.Retry.Retryable {
		result["retry"] = map[string]interface{}{
			"retryable":    e.Retry.Retryable,
			"retry_after":  e.Retry.RetryAfter.String(),
			"max_retries":  e.Retry.MaxRetries,
			"backoff_type": e.Retry.BackoffType,
		}
	}
	
	if len(e.Custom) > 0 {
		result["custom"] = e.Custom
	}
	
	return result
}

// IsEmpty returns true if the ConnectionCapabilities struct has no set values
func (c ConnectionCapabilities) IsEmpty() bool {
	return !c.Compression.Supported &&
		!c.Streaming.Supported &&
		!c.Security.TLSSupported &&
		c.MaxMessageSize == 0 &&
		len(c.AuthMethods) == 0 &&
		len(c.Extensions) == 0
}

// IsEmpty returns true if the ErrorDetails struct has no set values  
func (e ErrorDetails) IsEmpty() bool {
	return e.Code == "" &&
		e.Category == "" &&
		e.HTTPStatus == 0 &&
		!e.Retry.Retryable &&
		len(e.Custom) == 0
}