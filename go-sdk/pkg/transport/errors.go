package transport

import (
	"errors"
	"fmt"
	"sync"
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

	// ErrAlreadyStarted is returned when trying to start an already started component
	ErrAlreadyStarted = errors.New("already started")

	// ErrInvalidTaskName is returned when a task name is invalid
	ErrInvalidTaskName = errors.New("invalid task name")

	// ErrInvalidCleanupFunc is returned when a cleanup function is invalid
	ErrInvalidCleanupFunc = errors.New("invalid cleanup function")

	// ErrTaskNotFound is returned when a cleanup task is not found
	ErrTaskNotFound = errors.New("task not found")
)

// TransportError represents a transport-specific error with additional context
type TransportError struct {
	// mu protects the mutable fields
	mu sync.RWMutex

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
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Temporary
}

// IsRetryable returns whether the operation can be retried
func (e *TransportError) IsRetryable() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Retryable
}

// SetTemporary sets whether the error is temporary (thread-safe)
func (e *TransportError) SetTemporary(temporary bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Temporary = temporary
}

// SetRetryable sets whether the operation can be retried (thread-safe)
func (e *TransportError) SetRetryable(retryable bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Retryable = retryable
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

// ErrorValue defines the interface for allowed error context types
type ErrorValue interface {
	// ErrorString returns a string representation of the value for error messages
	ErrorString() string
}

// Common error value types that implement ErrorValue
type (
	// StringValue wraps a string value
	StringValue struct{ Value string }
	// IntValue wraps an integer value
	IntValue struct{ Value int }
	// BoolValue wraps a boolean value
	BoolValue struct{ Value bool }
	// FloatValue wraps a float value
	FloatValue struct{ Value float64 }
	// NilValue represents a nil/missing value
	NilValue struct{}
	// GenericValue wraps any value (for backward compatibility)
	GenericValue struct{ Value interface{} }
)

// ErrorString implementations for common types
func (v StringValue) ErrorString() string  { return v.Value }
func (v IntValue) ErrorString() string     { return fmt.Sprintf("%d", v.Value) }
func (v BoolValue) ErrorString() string    { return fmt.Sprintf("%t", v.Value) }
func (v FloatValue) ErrorString() string   { return fmt.Sprintf("%g", v.Value) }
func (v NilValue) ErrorString() string     { return "<nil>" }
func (v GenericValue) ErrorString() string { return fmt.Sprintf("%v", v.Value) }

// ConfigurationError represents a type-safe configuration-related error
type ConfigurationError[T ErrorValue] struct {
	Field   string
	Value   T
	Message string
}

func (e *ConfigurationError[T]) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("configuration error for field %s (value: %s): %s", e.Field, e.Value.ErrorString(), e.Message)
	}
	return fmt.Sprintf("configuration error: %s", e.Message)
}

// Legacy ConfigurationError for backward compatibility
// This maintains the original interface{} behavior
type LegacyConfigurationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *LegacyConfigurationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("configuration error for field %s (value: %v): %s", e.Field, e.Value, e.Message)
	}
	return fmt.Sprintf("configuration error: %s", e.Message)
}

// Type-safe error creation functions

// NewConfigurationError creates a new type-safe configuration error
func NewConfigurationError[T ErrorValue](field string, value T, message string) *ConfigurationError[T] {
	return &ConfigurationError[T]{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// NewLegacyConfigurationError creates a new legacy configuration error for backward compatibility
func NewLegacyConfigurationError(field string, value interface{}, message string) *LegacyConfigurationError {
	return &LegacyConfigurationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// Specific constructors for common configuration error types

// NewStringConfigError creates a configuration error for string values
func NewStringConfigError(field, value, message string) *ConfigurationError[StringValue] {
	return NewConfigurationError(field, StringValue{Value: value}, message)
}

// NewIntConfigError creates a configuration error for integer values
func NewIntConfigError(field string, value int, message string) *ConfigurationError[IntValue] {
	return NewConfigurationError(field, IntValue{Value: value}, message)
}

// NewBoolConfigError creates a configuration error for boolean values
func NewBoolConfigError(field string, value bool, message string) *ConfigurationError[BoolValue] {
	return NewConfigurationError(field, BoolValue{Value: value}, message)
}

// NewFloatConfigError creates a configuration error for float values
func NewFloatConfigError(field string, value float64, message string) *ConfigurationError[FloatValue] {
	return NewConfigurationError(field, FloatValue{Value: value}, message)
}

// NewNilConfigError creates a configuration error for nil/missing values
func NewNilConfigError(field, message string) *ConfigurationError[NilValue] {
	return NewConfigurationError(field, NilValue{}, message)
}

// NewGenericConfigError creates a configuration error for any value type (fallback)
func NewGenericConfigError(field string, value interface{}, message string) *ConfigurationError[GenericValue] {
	return NewConfigurationError(field, GenericValue{Value: value}, message)
}

// Validation functions for error context values

// ValidateErrorValue validates an error value and returns a standardized ErrorValue
func ValidateErrorValue(value interface{}) ErrorValue {
	if value == nil {
		return NilValue{}
	}

	switch v := value.(type) {
	case string:
		return StringValue{Value: v}
	case int:
		return IntValue{Value: v}
	case int32:
		return IntValue{Value: int(v)}
	case int64:
		return IntValue{Value: int(v)}
	case bool:
		return BoolValue{Value: v}
	case float32:
		return FloatValue{Value: float64(v)}
	case float64:
		return FloatValue{Value: v}
	default:
		return GenericValue{Value: value}
	}
}

// CreateTypedConfigError creates a type-safe configuration error from an interface{} value
func CreateTypedConfigError(field string, value interface{}, message string) error {
	typedValue := ValidateErrorValue(value)
	return &ConfigurationError[ErrorValue]{
		Field:   field,
		Value:   typedValue,
		Message: message,
	}
}

// Backward compatibility functions and type aliases

// ConfigError is an alias for LegacyConfigurationError to maintain compatibility
type ConfigError = LegacyConfigurationError

// NewConfigError creates a legacy configuration error (backward compatibility)
func NewConfigError(field string, value interface{}, message string) *LegacyConfigurationError {
	return NewLegacyConfigurationError(field, value, message)
}

// IsConfigurationError checks if an error is any type of configuration error
func IsConfigurationError(err error) bool {
	// Check for new generic configuration error
	var genericErr *ConfigurationError[ErrorValue]
	if errors.As(err, &genericErr) {
		return true
	}

	// Check for legacy configuration error
	var legacyErr *LegacyConfigurationError
	if errors.As(err, &legacyErr) {
		return true
	}

	// Check for specific typed configuration errors
	var stringErr *ConfigurationError[StringValue]
	var intErr *ConfigurationError[IntValue]
	var boolErr *ConfigurationError[BoolValue]
	var floatErr *ConfigurationError[FloatValue]
	var nilErr *ConfigurationError[NilValue]
	var genericValueErr *ConfigurationError[GenericValue]

	return errors.As(err, &stringErr) ||
		errors.As(err, &intErr) ||
		errors.As(err, &boolErr) ||
		errors.As(err, &floatErr) ||
		errors.As(err, &nilErr) ||
		errors.As(err, &genericValueErr)
}

// GetConfigurationErrorField extracts the field name from any configuration error type
func GetConfigurationErrorField(err error) string {
	// Try legacy first for better performance on existing code
	if legacyErr, ok := err.(*LegacyConfigurationError); ok {
		return legacyErr.Field
	}

	// Try generic configuration error
	if genericErr, ok := err.(*ConfigurationError[ErrorValue]); ok {
		return genericErr.Field
	}

	// Try specific typed errors
	if stringErr, ok := err.(*ConfigurationError[StringValue]); ok {
		return stringErr.Field
	}
	if intErr, ok := err.(*ConfigurationError[IntValue]); ok {
		return intErr.Field
	}
	if boolErr, ok := err.(*ConfigurationError[BoolValue]); ok {
		return boolErr.Field
	}
	if floatErr, ok := err.(*ConfigurationError[FloatValue]); ok {
		return floatErr.Field
	}
	if nilErr, ok := err.(*ConfigurationError[NilValue]); ok {
		return nilErr.Field
	}
	if genericValueErr, ok := err.(*ConfigurationError[GenericValue]); ok {
		return genericValueErr.Field
	}

	return ""
}

// GetConfigurationErrorValue extracts the value from any configuration error type as interface{}
func GetConfigurationErrorValue(err error) interface{} {
	// Try legacy first for better performance on existing code
	if legacyErr, ok := err.(*LegacyConfigurationError); ok {
		return legacyErr.Value
	}

	// Try generic configuration error
	if genericErr, ok := err.(*ConfigurationError[ErrorValue]); ok {
		if genericValue, ok := genericErr.Value.(GenericValue); ok {
			return genericValue.Value
		}
		// For other ErrorValue types, return the underlying value
		switch v := genericErr.Value.(type) {
		case StringValue:
			return v.Value
		case IntValue:
			return v.Value
		case BoolValue:
			return v.Value
		case FloatValue:
			return v.Value
		case NilValue:
			return nil
		default:
			return v
		}
	}

	// Try specific typed errors
	if stringErr, ok := err.(*ConfigurationError[StringValue]); ok {
		return stringErr.Value.Value
	}
	if intErr, ok := err.(*ConfigurationError[IntValue]); ok {
		return intErr.Value.Value
	}
	if boolErr, ok := err.(*ConfigurationError[BoolValue]); ok {
		return boolErr.Value.Value
	}
	if floatErr, ok := err.(*ConfigurationError[FloatValue]); ok {
		return floatErr.Value.Value
	}
	if _, ok := err.(*ConfigurationError[NilValue]); ok {
		return nil
	}
	if genericValueErr, ok := err.(*ConfigurationError[GenericValue]); ok {
		return genericValueErr.Value.Value
	}

	return nil
}
