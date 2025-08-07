package transport

import "time"

// ConnectionCapabilities represents basic connection information.
// Simplified from the previous complex capability negotiation system.
type ConnectionCapabilities struct {
	// Maximum message size supported
	MaxMessageSize int64 `json:"max_message_size,omitempty"`

	// Protocol version
	ProtocolVersion string `json:"protocol_version,omitempty"`

	// Custom extension capabilities for backward compatibility
	Extensions map[string]string `json:"extensions,omitempty"`
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
	Retryable   bool          `json:"retryable"`
	RetryAfter  time.Duration `json:"retry_after,omitempty"`
	MaxRetries  int           `json:"max_retries,omitempty"`
	BackoffType string        `json:"backoff_type,omitempty"`
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
	Type      string `json:"type"`
	Message   string `json:"message"`
	Source    string `json:"source,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// ToMap converts ConnectionCapabilities to map[string]interface{} for backward compatibility
func (c ConnectionCapabilities) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if c.MaxMessageSize > 0 {
		result["max_message_size"] = c.MaxMessageSize
	}

	if c.ProtocolVersion != "" {
		result["protocol_version"] = c.ProtocolVersion
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
	return c.MaxMessageSize == 0 &&
		c.ProtocolVersion == "" &&
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
