package transport

import (
	"errors"
	"fmt"
)

// Common transport errors
var (
	// ErrNotConnected is returned when an operation is attempted on a disconnected transport
	ErrNotConnected = errors.New("transport not connected")

	// ErrAlreadyConnected is returned when Connect is called on an already connected transport
	ErrAlreadyConnected = errors.New("transport already connected")

	// ErrConnectionFailed is returned when a connection attempt fails
	ErrConnectionFailed = errors.New("failed to establish connection")

	// ErrConnectionClosed is returned when the connection is closed unexpectedly
	ErrConnectionClosed = errors.New("connection closed")

	// ErrTimeout is returned when an operation times out
	ErrTimeout = errors.New("operation timed out")

	// ErrMessageTooLarge is returned when a message exceeds the transport's size limit
	ErrMessageTooLarge = errors.New("message too large")

	// ErrUnsupportedCapability is returned when a requested capability is not supported
	ErrUnsupportedCapability = errors.New("unsupported capability")

	// ErrTransportNotFound is returned when a requested transport is not registered
	ErrTransportNotFound = errors.New("transport not found")

	// ErrInvalidConfiguration is returned when transport configuration is invalid
	ErrInvalidConfiguration = errors.New("invalid configuration")

	// ErrStreamNotFound is returned when a requested stream does not exist
	ErrStreamNotFound = errors.New("stream not found")

	// ErrStreamClosed is returned when an operation is attempted on a closed stream
	ErrStreamClosed = errors.New("stream closed")

	// ErrReconnectFailed is returned when all reconnection attempts fail
	ErrReconnectFailed = errors.New("reconnection failed")

	// ErrHealthCheckFailed is returned when a health check fails
	ErrHealthCheckFailed = errors.New("health check failed")

	// ErrBackpressureActive is returned when backpressure is active and blocking operations
	ErrBackpressureActive = errors.New("backpressure active")

	// ErrBackpressureTimeout is returned when backpressure timeout is exceeded
	ErrBackpressureTimeout = errors.New("backpressure timeout exceeded")

	// ErrValidationFailed is returned when message validation fails
	ErrValidationFailed = errors.New("message validation failed")

	// ErrInvalidMessageSize is returned when message size exceeds limits
	ErrInvalidMessageSize = errors.New("message size exceeds limits")

	// ErrMissingRequiredFields is returned when required fields are missing
	ErrMissingRequiredFields = errors.New("missing required fields")

	// ErrInvalidEventType is returned when event type is not allowed
	ErrInvalidEventType = errors.New("invalid event type")

	// ErrInvalidDataFormat is returned when data format is invalid
	ErrInvalidDataFormat = errors.New("invalid data format")

	// ErrFieldValidationFailed is returned when field validation fails
	ErrFieldValidationFailed = errors.New("field validation failed")

	// ErrPatternValidationFailed is returned when pattern validation fails
	ErrPatternValidationFailed = errors.New("pattern validation failed")
)

// TransportError represents a transport-specific error with additional context
type TransportError struct {
	// Transport is the name of the transport that generated the error
	Transport string

	// Op is the operation that caused the error
	Op string

	// Err is the underlying error
	Err error

	// Temporary indicates if the error is temporary and may be retried
	Temporary bool

	// Retryable indicates if the operation can be retried
	Retryable bool
}

// Error implements the error interface
func (e *TransportError) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("%s %s: %v", e.Transport, e.Op, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Transport, e.Err)
}

// Unwrap returns the underlying error
func (e *TransportError) Unwrap() error {
	return e.Err
}

// IsTemporary returns whether the error is temporary
func (e *TransportError) IsTemporary() bool {
	return e.Temporary
}

// IsRetryable returns whether the operation can be retried
func (e *TransportError) IsRetryable() bool {
	return e.Retryable
}

// NewTransportError creates a new TransportError
func NewTransportError(transport, op string, err error) *TransportError {
	return &TransportError{
		Transport: transport,
		Op:        op,
		Err:       err,
		Temporary: false,
		Retryable: false,
	}
}

// NewTemporaryError creates a new temporary TransportError
func NewTemporaryError(transport, op string, err error) *TransportError {
	return &TransportError{
		Transport: transport,
		Op:        op,
		Err:       err,
		Temporary: true,
		Retryable: true,
	}
}

// IsTransportError checks if an error is a TransportError
func IsTransportError(err error) bool {
	var te *TransportError
	return errors.As(err, &te)
}

// ConnectionError represents a connection-related error
type ConnectionError struct {
	Endpoint string
	Cause    error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("connection error to %s: %v", e.Endpoint, e.Cause)
}

func (e *ConnectionError) Unwrap() error {
	return e.Cause
}

// ConfigurationError represents a configuration-related error
type ConfigurationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ConfigurationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("configuration error for field %s (value: %v): %s", e.Field, e.Value, e.Message)
	}
	return fmt.Sprintf("configuration error: %s", e.Message)
}