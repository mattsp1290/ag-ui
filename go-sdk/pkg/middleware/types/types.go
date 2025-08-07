// Package types provides shared types for the middleware system to eliminate
// code duplication across all middleware packages.
//
// This package defines the core Request, Response, and NextHandler types that
// are used consistently across all middleware implementations including:
// - auth/ (authentication and authorization middleware)
// - observability/ (logging and metrics middleware)
// - security/ (security middleware with CORS, CSRF, etc.)
// - ratelimit/ (rate limiting middleware)
// - transform/ (data transformation middleware)
//
// By centralizing these type definitions, we:
// 1. Eliminate massive code duplication (previously duplicated across 6+ packages)
// 2. Ensure type consistency across all middleware
// 3. Simplify maintenance and evolution of the middleware system
// 4. Enable better interoperability between middleware components
package types

import (
	"context"
	"time"
)

// Request represents a generic request that can be processed by any middleware
// in the system. This unified type ensures consistent request handling across
// all middleware implementations.
type Request struct {
	// ID is a unique identifier for this request, used for tracing and correlation
	ID string `json:"id"`

	// Method represents the HTTP method or operation type (GET, POST, etc.)
	Method string `json:"method"`

	// Path represents the request path or endpoint being accessed
	Path string `json:"path"`

	// Headers contains all request headers as key-value pairs
	Headers map[string]string `json:"headers"`

	// Body contains the request payload, can be any serializable type
	Body interface{} `json:"body"`

	// Metadata contains additional request metadata that middleware can use
	// for processing decisions, context sharing, and request enrichment
	Metadata map[string]interface{} `json:"metadata"`

	// Timestamp indicates when the request was created or received
	Timestamp time.Time `json:"timestamp"`
}

// Response represents a generic response that can be processed by any middleware
// in the system. This unified type ensures consistent response handling across
// all middleware implementations.
type Response struct {
	// ID correlates this response with its corresponding request
	ID string `json:"id"`

	// StatusCode represents the HTTP status code or operation result code
	StatusCode int `json:"status_code"`

	// Headers contains all response headers as key-value pairs
	Headers map[string]string `json:"headers"`

	// Body contains the response payload, can be any serializable type
	Body interface{} `json:"body"`

	// Error contains any error that occurred during request processing
	// Note: This field uses omitempty to exclude it from JSON when nil
	Error error `json:"error,omitempty"`

	// Metadata contains additional response metadata that middleware can use
	// for processing decisions, context sharing, and response enrichment
	Metadata map[string]interface{} `json:"metadata"`

	// Timestamp indicates when the response was created
	Timestamp time.Time `json:"timestamp"`

	// Duration represents the total processing time for the request
	Duration time.Duration `json:"duration"`
}

// NextHandler represents the next middleware or final handler in the middleware chain.
// This is a function type that all middleware use to continue the request processing
// chain. It receives a context and request, and returns a response and optional error.
type NextHandler func(ctx context.Context, req *Request) (*Response, error)

// RequestMetadataKeys defines standard metadata keys that middleware commonly use
// to share information in the request metadata map.
var RequestMetadataKeys = struct {
	// Authentication and authorization context
	AuthContext string
	UserID      string
	Roles       string
	Permissions string
	SessionID   string
	TokenClaims string

	// Request tracing and correlation
	TraceID       string
	SpanID        string
	CorrelationID string
	RequestSource string

	// Rate limiting and throttling
	ClientIP      string
	UserAgent     string
	RateLimitKey  string
	RateLimitTier string

	// Security and validation
	SecurityContext string
	ValidationInfo  string
	ThreatLevel     string
	SanitizedBody   string

	// Transformation and processing
	OriginalBody      string
	TransformPipeline string
	CompressionInfo   string
	ValidationSchema  string

	// Observability and monitoring
	StartTime       string
	ProcessingSteps string
	MiddlewareStack string
	MetricsContext  string
}{
	// Authentication and authorization context
	AuthContext: "auth_context",
	UserID:      "user_id",
	Roles:       "roles",
	Permissions: "permissions",
	SessionID:   "session_id",
	TokenClaims: "token_claims",

	// Request tracing and correlation
	TraceID:       "trace_id",
	SpanID:        "span_id",
	CorrelationID: "correlation_id",
	RequestSource: "request_source",

	// Rate limiting and throttling
	ClientIP:      "client_ip",
	UserAgent:     "user_agent",
	RateLimitKey:  "rate_limit_key",
	RateLimitTier: "rate_limit_tier",

	// Security and validation
	SecurityContext: "security_context",
	ValidationInfo:  "validation_info",
	ThreatLevel:     "threat_level",
	SanitizedBody:   "sanitized_body",

	// Transformation and processing
	OriginalBody:      "original_body",
	TransformPipeline: "transform_pipeline",
	CompressionInfo:   "compression_info",
	ValidationSchema:  "validation_schema",

	// Observability and monitoring
	StartTime:       "start_time",
	ProcessingSteps: "processing_steps",
	MiddlewareStack: "middleware_stack",
	MetricsContext:  "metrics_context",
}

// ResponseMetadataKeys defines standard metadata keys that middleware commonly use
// to share information in the response metadata map.
var ResponseMetadataKeys = struct {
	// Processing information
	ProcessedBy      string
	ProcessingTime   string
	MiddlewareChain  string
	TransformApplied string

	// Security and validation
	SecurityHeaders     string
	ValidationResult    string
	SanitizationApplied string

	// Performance and monitoring
	CacheInfo        string
	CompressionRatio string
	ProcessingStats  string

	// Error and debugging information
	ErrorDetails    string
	DebugInfo       string
	WarningMessages string
}{
	// Processing information
	ProcessedBy:      "processed_by",
	ProcessingTime:   "processing_time",
	MiddlewareChain:  "middleware_chain",
	TransformApplied: "transform_applied",

	// Security and validation
	SecurityHeaders:     "security_headers",
	ValidationResult:    "validation_result",
	SanitizationApplied: "sanitization_applied",

	// Performance and monitoring
	CacheInfo:        "cache_info",
	CompressionRatio: "compression_ratio",
	ProcessingStats:  "processing_stats",

	// Error and debugging information
	ErrorDetails:    "error_details",
	DebugInfo:       "debug_info",
	WarningMessages: "warning_messages",
}

// NewRequest creates a new Request with initialized metadata and timestamp
func NewRequest(id, method, path string) *Request {
	return &Request{
		ID:        id,
		Method:    method,
		Path:      path,
		Headers:   make(map[string]string),
		Metadata:  make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// NewResponse creates a new Response with initialized metadata and timestamp
func NewResponse(id string, statusCode int) *Response {
	return &Response{
		ID:         id,
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Metadata:   make(map[string]interface{}),
		Timestamp:  time.Now(),
	}
}

// Clone creates a deep copy of the Request
func (r *Request) Clone() *Request {
	if r == nil {
		return nil
	}

	clone := &Request{
		ID:        r.ID,
		Method:    r.Method,
		Path:      r.Path,
		Body:      r.Body, // Note: shallow copy of Body
		Timestamp: r.Timestamp,
	}

	// Deep copy Headers
	if r.Headers != nil {
		clone.Headers = make(map[string]string, len(r.Headers))
		for k, v := range r.Headers {
			clone.Headers[k] = v
		}
	}

	// Deep copy Metadata (shallow copy of values)
	if r.Metadata != nil {
		clone.Metadata = make(map[string]interface{}, len(r.Metadata))
		for k, v := range r.Metadata {
			clone.Metadata[k] = v
		}
	}

	return clone
}

// Clone creates a deep copy of the Response
func (r *Response) Clone() *Response {
	if r == nil {
		return nil
	}

	clone := &Response{
		ID:         r.ID,
		StatusCode: r.StatusCode,
		Body:       r.Body, // Note: shallow copy of Body
		Error:      r.Error,
		Timestamp:  r.Timestamp,
		Duration:   r.Duration,
	}

	// Deep copy Headers
	if r.Headers != nil {
		clone.Headers = make(map[string]string, len(r.Headers))
		for k, v := range r.Headers {
			clone.Headers[k] = v
		}
	}

	// Deep copy Metadata (shallow copy of values)
	if r.Metadata != nil {
		clone.Metadata = make(map[string]interface{}, len(r.Metadata))
		for k, v := range r.Metadata {
			clone.Metadata[k] = v
		}
	}

	return clone
}

// SetMetadata sets a metadata value in the request
func (r *Request) SetMetadata(key string, value interface{}) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
}

// GetMetadata gets a metadata value from the request
func (r *Request) GetMetadata(key string) (interface{}, bool) {
	if r.Metadata == nil {
		return nil, false
	}
	value, exists := r.Metadata[key]
	return value, exists
}

// SetHeader sets a header value in the request
func (r *Request) SetHeader(key, value string) {
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}
	r.Headers[key] = value
}

// GetHeader gets a header value from the request
func (r *Request) GetHeader(key string) (string, bool) {
	if r.Headers == nil {
		return "", false
	}
	value, exists := r.Headers[key]
	return value, exists
}

// SetMetadata sets a metadata value in the response
func (r *Response) SetMetadata(key string, value interface{}) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
}

// GetMetadata gets a metadata value from the response
func (r *Response) GetMetadata(key string) (interface{}, bool) {
	if r.Metadata == nil {
		return nil, false
	}
	value, exists := r.Metadata[key]
	return value, exists
}

// SetHeader sets a header value in the response
func (r *Response) SetHeader(key, value string) {
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}
	r.Headers[key] = value
}

// GetHeader gets a header value from the response
func (r *Response) GetHeader(key string) (string, bool) {
	if r.Headers == nil {
		return "", false
	}
	value, exists := r.Headers[key]
	return value, exists
}

// IsSuccessful returns true if the response indicates success (status code 200-299)
func (r *Response) IsSuccessful() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// IsClientError returns true if the response indicates a client error (status code 400-499)
func (r *Response) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

// IsServerError returns true if the response indicates a server error (status code 500-599)
func (r *Response) IsServerError() bool {
	return r.StatusCode >= 500 && r.StatusCode < 600
}

// HasError returns true if the response contains an error
func (r *Response) HasError() bool {
	return r.Error != nil
}
