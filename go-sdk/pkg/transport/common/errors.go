package common

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"syscall"
	"time"
)

// TransportErrorType represents different types of transport errors.
type TransportErrorType string

const (
	// ErrorTypeConnection indicates connection-related errors
	ErrorTypeConnection TransportErrorType = "connection"
	// ErrorTypeTimeout indicates timeout errors
	ErrorTypeTimeout TransportErrorType = "timeout"
	// ErrorTypeAuthentication indicates authentication errors
	ErrorTypeAuthentication TransportErrorType = "authentication"
	// ErrorTypeAuthorization indicates authorization errors
	ErrorTypeAuthorization TransportErrorType = "authorization"
	// ErrorTypeProtocol indicates protocol-related errors
	ErrorTypeProtocol TransportErrorType = "protocol"
	// ErrorTypeSerialization indicates serialization errors
	ErrorTypeSerialization TransportErrorType = "serialization"
	// ErrorTypeCompression indicates compression errors
	ErrorTypeCompression TransportErrorType = "compression"
	// ErrorTypeConfiguration indicates configuration errors
	ErrorTypeConfiguration TransportErrorType = "configuration"
	// ErrorTypeValidation indicates validation errors
	ErrorTypeValidation TransportErrorType = "validation"
	// ErrorTypeNetwork indicates network-related errors
	ErrorTypeNetwork TransportErrorType = "network"
	// ErrorTypeRetry indicates retry-related errors
	ErrorTypeRetry TransportErrorType = "retry"
	// ErrorTypeCircuitBreaker indicates circuit breaker errors
	ErrorTypeCircuitBreaker TransportErrorType = "circuit_breaker"
	// ErrorTypeRateLimit indicates rate limiting errors
	ErrorTypeRateLimit TransportErrorType = "rate_limit"
	// ErrorTypeCapacity indicates capacity-related errors
	ErrorTypeCapacity TransportErrorType = "capacity"
	// ErrorTypeInternal indicates internal errors
	ErrorTypeInternal TransportErrorType = "internal"
	// ErrorTypeUnsupported indicates unsupported operation errors
	ErrorTypeUnsupported TransportErrorType = "unsupported"
	// ErrorTypeIncompatible indicates incompatible version errors
	ErrorTypeIncompatible TransportErrorType = "incompatible"
)

// TransportError is the base error type for transport-related errors.
type TransportError struct {
	Type      TransportErrorType `json:"type"`
	Message   string             `json:"message"`
	Code      string             `json:"code,omitempty"`
	Cause     error              `json:"-"`
	Timestamp time.Time          `json:"timestamp"`
	Transport string             `json:"transport,omitempty"`
	Endpoint  string             `json:"endpoint,omitempty"`
	Metadata  map[string]any     `json:"metadata,omitempty"`
	Retryable bool               `json:"retryable"`
	Temporary bool               `json:"temporary"`
}

// Error implements the error interface.
func (e *TransportError) Error() string {
	if e.Transport != "" {
		return fmt.Sprintf("%s transport error: %s", e.Transport, e.Message)
	}
	return fmt.Sprintf("transport error: %s", e.Message)
}

// Unwrap returns the underlying error.
func (e *TransportError) Unwrap() error {
	return e.Cause
}

// Is checks if the error is of a specific type.
func (e *TransportError) Is(target error) bool {
	if target == nil {
		return false
	}

	if te, ok := target.(*TransportError); ok {
		return e.Type == te.Type
	}

	return errors.Is(e.Cause, target)
}

// WithMetadata adds metadata to the error.
func (e *TransportError) WithMetadata(key string, value any) *TransportError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

// WithTransport sets the transport name.
func (e *TransportError) WithTransport(transport string) *TransportError {
	e.Transport = transport
	return e
}

// WithEndpoint sets the endpoint.
func (e *TransportError) WithEndpoint(endpoint string) *TransportError {
	e.Endpoint = endpoint
	return e
}

// WithCode sets the error code.
func (e *TransportError) WithCode(code string) *TransportError {
	e.Code = code
	return e
}

// IsRetryable returns true if the error is retryable.
func (e *TransportError) IsRetryable() bool {
	return e.Retryable
}

// IsTemporary returns true if the error is temporary.
func (e *TransportError) IsTemporary() bool {
	return e.Temporary
}

// NewTransportError creates a new transport error.
func NewTransportError(errorType TransportErrorType, message string, cause error) *TransportError {
	return &TransportError{
		Type:      errorType,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Retryable: isRetryable(errorType, cause),
		Temporary: isTemporary(errorType, cause),
	}
}

// ConnectionError represents connection-related errors.
type ConnectionError struct {
	*TransportError
	Endpoint    string        `json:"endpoint"`
	Timeout     time.Duration `json:"timeout,omitempty"`
	Attempts    int           `json:"attempts,omitempty"`
	LastAttempt time.Time     `json:"last_attempt,omitempty"`
}

// NewConnectionError creates a new connection error.
func NewConnectionError(endpoint string, cause error) *ConnectionError {
	return &ConnectionError{
		TransportError: NewTransportError(ErrorTypeConnection,
			fmt.Sprintf("failed to connect to %s", endpoint), cause),
		Endpoint: endpoint,
	}
}

// TimeoutError represents timeout errors.
type TimeoutError struct {
	*TransportError
	Operation string        `json:"operation"`
	Timeout   time.Duration `json:"timeout"`
	Elapsed   time.Duration `json:"elapsed"`
}

// NewTimeoutError creates a new timeout error.
func NewTimeoutError(operation string, timeout, elapsed time.Duration) *TimeoutError {
	return &TimeoutError{
		TransportError: NewTransportError(ErrorTypeTimeout,
			fmt.Sprintf("%s timed out after %v (elapsed: %v)", operation, timeout, elapsed), nil),
		Operation: operation,
		Timeout:   timeout,
		Elapsed:   elapsed,
	}
}

// AuthenticationError represents authentication errors.
type AuthenticationError struct {
	*TransportError
	AuthType string `json:"auth_type"`
	Reason   string `json:"reason"`
}

// NewAuthenticationError creates a new authentication error.
func NewAuthenticationError(authType, reason string) *AuthenticationError {
	return &AuthenticationError{
		TransportError: NewTransportError(ErrorTypeAuthentication,
			fmt.Sprintf("authentication failed: %s", reason), nil),
		AuthType: authType,
		Reason:   reason,
	}
}

// AuthorizationError represents authorization errors.
type AuthorizationError struct {
	*TransportError
	Resource            string   `json:"resource"`
	Action              string   `json:"action"`
	RequiredPermissions []string `json:"required_permissions,omitempty"`
}

// NewAuthorizationError creates a new authorization error.
func NewAuthorizationError(resource, action string, requiredPermissions []string) *AuthorizationError {
	return &AuthorizationError{
		TransportError: NewTransportError(ErrorTypeAuthorization,
			fmt.Sprintf("authorization failed for %s on %s", action, resource), nil),
		Resource:            resource,
		Action:              action,
		RequiredPermissions: requiredPermissions,
	}
}

// ProtocolError represents protocol-related errors.
type ProtocolError struct {
	*TransportError
	Protocol string `json:"protocol"`
	Version  string `json:"version,omitempty"`
	Details  string `json:"details,omitempty"`
}

// NewProtocolError creates a new protocol error.
func NewProtocolError(protocol, version, details string) *ProtocolError {
	message := fmt.Sprintf("protocol error in %s", protocol)
	if version != "" {
		message += fmt.Sprintf(" (version %s)", version)
	}
	if details != "" {
		message += fmt.Sprintf(": %s", details)
	}

	return &ProtocolError{
		TransportError: NewTransportError(ErrorTypeProtocol, message, nil),
		Protocol:       protocol,
		Version:        version,
		Details:        details,
	}
}

// SerializationError represents serialization errors.
type SerializationError struct {
	*TransportError
	Format string `json:"format"`
	Data   string `json:"data,omitempty"`
}

// NewSerializationError creates a new serialization error.
func NewSerializationError(format string, data string, cause error) *SerializationError {
	return &SerializationError{
		TransportError: NewTransportError(ErrorTypeSerialization,
			fmt.Sprintf("serialization failed for format %s", format), cause),
		Format: format,
		Data:   data,
	}
}

// CompressionError represents compression errors.
type CompressionError struct {
	*TransportError
	Algorithm string `json:"algorithm"`
	Operation string `json:"operation"` // "compress" or "decompress"
	DataSize  int64  `json:"data_size,omitempty"`
}

// NewCompressionError creates a new compression error.
func NewCompressionError(algorithm, operation string, dataSize int64, cause error) *CompressionError {
	return &CompressionError{
		TransportError: NewTransportError(ErrorTypeCompression,
			fmt.Sprintf("%s %s failed", algorithm, operation), cause),
		Algorithm: algorithm,
		Operation: operation,
		DataSize:  dataSize,
	}
}

// ConfigurationError represents configuration errors.
type ConfigurationError struct {
	*TransportError
	Field  string `json:"field"`
	Value  any    `json:"value,omitempty"`
	Reason string `json:"reason"`
}

// NewConfigurationError creates a new configuration error.
func NewConfigurationError(field string, value any, reason string) *ConfigurationError {
	return &ConfigurationError{
		TransportError: NewTransportError(ErrorTypeConfiguration,
			fmt.Sprintf("configuration error in field %s: %s", field, reason), nil),
		Field:  field,
		Value:  value,
		Reason: reason,
	}
}

// ValidationError represents validation errors.
type ValidationError struct {
	*TransportError
	Field  string `json:"field"`
	Value  any    `json:"value,omitempty"`
	Rule   string `json:"rule"`
	Reason string `json:"reason"`
}

// NewValidationError creates a new validation error.
func NewValidationError(field, rule, reason string, value any) *ValidationError {
	return &ValidationError{
		TransportError: NewTransportError(ErrorTypeValidation,
			fmt.Sprintf("validation failed for field %s: %s", field, reason), nil),
		Field:  field,
		Value:  value,
		Rule:   rule,
		Reason: reason,
	}
}

// NetworkError represents network-related errors.
type NetworkError struct {
	*TransportError
	Operation string `json:"operation"`
	Address   string `json:"address"`
	Network   string `json:"network"`
}

// NewNetworkError creates a new network error.
func NewNetworkError(operation, network, address string, cause error) *NetworkError {
	return &NetworkError{
		TransportError: NewTransportError(ErrorTypeNetwork,
			fmt.Sprintf("network error during %s to %s://%s", operation, network, address), cause),
		Operation: operation,
		Address:   address,
		Network:   network,
	}
}

// RetryError represents retry-related errors.
type RetryError struct {
	*TransportError
	Attempts    int           `json:"attempts"`
	MaxAttempts int           `json:"max_attempts"`
	LastError   error         `json:"-"`
	Duration    time.Duration `json:"duration"`
}

// NewRetryError creates a new retry error.
func NewRetryError(attempts, maxAttempts int, lastError error, duration time.Duration) *RetryError {
	return &RetryError{
		TransportError: NewTransportError(ErrorTypeRetry,
			fmt.Sprintf("retry failed after %d attempts (max %d)", attempts, maxAttempts), lastError),
		Attempts:    attempts,
		MaxAttempts: maxAttempts,
		LastError:   lastError,
		Duration:    duration,
	}
}

// CircuitBreakerError represents circuit breaker errors.
type CircuitBreakerError struct {
	*TransportError
	State       string        `json:"state"`
	Failures    int           `json:"failures"`
	LastFailure time.Time     `json:"last_failure"`
	NextRetry   time.Time     `json:"next_retry"`
	Timeout     time.Duration `json:"timeout"`
}

// NewCircuitBreakerError creates a new circuit breaker error.
func NewCircuitBreakerError(state string, failures int, lastFailure time.Time, timeout time.Duration) *CircuitBreakerError {
	nextRetry := lastFailure.Add(timeout)
	return &CircuitBreakerError{
		TransportError: NewTransportError(ErrorTypeCircuitBreaker,
			fmt.Sprintf("circuit breaker is %s (failures: %d)", state, failures), nil),
		State:       state,
		Failures:    failures,
		LastFailure: lastFailure,
		NextRetry:   nextRetry,
		Timeout:     timeout,
	}
}

// RateLimitError represents rate limiting errors.
type RateLimitError struct {
	*TransportError
	Limit     int           `json:"limit"`
	Window    time.Duration `json:"window"`
	Remaining int           `json:"remaining"`
	ResetAt   time.Time     `json:"reset_at"`
}

// NewRateLimitError creates a new rate limit error.
func NewRateLimitError(limit int, window time.Duration, remaining int, resetAt time.Time) *RateLimitError {
	return &RateLimitError{
		TransportError: NewTransportError(ErrorTypeRateLimit,
			fmt.Sprintf("rate limit exceeded: %d requests per %v", limit, window), nil),
		Limit:     limit,
		Window:    window,
		Remaining: remaining,
		ResetAt:   resetAt,
	}
}

// CapacityError represents capacity-related errors.
type CapacityError struct {
	*TransportError
	Resource string `json:"resource"`
	Current  int64  `json:"current"`
	Maximum  int64  `json:"maximum"`
	Unit     string `json:"unit"`
}

// NewCapacityError creates a new capacity error.
func NewCapacityError(resource string, current, maximum int64, unit string) *CapacityError {
	return &CapacityError{
		TransportError: NewTransportError(ErrorTypeCapacity,
			fmt.Sprintf("capacity exceeded for %s: %d/%d %s", resource, current, maximum, unit), nil),
		Resource: resource,
		Current:  current,
		Maximum:  maximum,
		Unit:     unit,
	}
}

// UnsupportedError represents unsupported operation errors.
type UnsupportedError struct {
	*TransportError
	Operation string `json:"operation"`
	Transport string `json:"transport"`
	Reason    string `json:"reason"`
}

// NewUnsupportedError creates a new unsupported error.
func NewUnsupportedError(operation, transport, reason string) *UnsupportedError {
	return &UnsupportedError{
		TransportError: NewTransportError(ErrorTypeUnsupported,
			fmt.Sprintf("operation %s is not supported by %s transport: %s", operation, transport, reason), nil),
		Operation: operation,
		Transport: transport,
		Reason:    reason,
	}
}

// IncompatibleError represents incompatible version errors.
type IncompatibleError struct {
	*TransportError
	RequiredVersion string `json:"required_version"`
	CurrentVersion  string `json:"current_version"`
	Component       string `json:"component"`
}

// NewIncompatibleError creates a new incompatible error.
func NewIncompatibleError(component, requiredVersion, currentVersion string) *IncompatibleError {
	return &IncompatibleError{
		TransportError: NewTransportError(ErrorTypeIncompatible,
			fmt.Sprintf("incompatible %s version: required %s, current %s", component, requiredVersion, currentVersion), nil),
		RequiredVersion: requiredVersion,
		CurrentVersion:  currentVersion,
		Component:       component,
	}
}

// Helper functions for error classification

// isRetryable determines if an error is retryable based on its type and cause.
func isRetryable(errorType TransportErrorType, cause error) bool {
	switch errorType {
	case ErrorTypeConnection, ErrorTypeTimeout, ErrorTypeNetwork:
		return true
	case ErrorTypeAuthentication, ErrorTypeAuthorization:
		return false
	case ErrorTypeProtocol, ErrorTypeSerialization, ErrorTypeCompression:
		return false
	case ErrorTypeConfiguration, ErrorTypeValidation:
		return false
	case ErrorTypeRetry, ErrorTypeCircuitBreaker:
		return false
	case ErrorTypeRateLimit:
		return true
	case ErrorTypeCapacity:
		return true
	case ErrorTypeInternal:
		return true
	case ErrorTypeUnsupported, ErrorTypeIncompatible:
		return false
	default:
		return isRetryableFromCause(cause)
	}
}

// isTemporary determines if an error is temporary based on its type and cause.
func isTemporary(errorType TransportErrorType, cause error) bool {
	switch errorType {
	case ErrorTypeConnection, ErrorTypeTimeout, ErrorTypeNetwork:
		return true
	case ErrorTypeAuthentication, ErrorTypeAuthorization:
		return false
	case ErrorTypeProtocol, ErrorTypeSerialization, ErrorTypeCompression:
		return false
	case ErrorTypeConfiguration, ErrorTypeValidation:
		return false
	case ErrorTypeRetry, ErrorTypeCircuitBreaker:
		return true
	case ErrorTypeRateLimit:
		return true
	case ErrorTypeCapacity:
		return true
	case ErrorTypeInternal:
		return true
	case ErrorTypeUnsupported, ErrorTypeIncompatible:
		return false
	default:
		return isTemporaryFromCause(cause)
	}
}

// isRetryableFromCause determines if an error is retryable based on its underlying cause.
func isRetryableFromCause(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types
	if netErr, ok := err.(net.Error); ok {
		return netErr.Temporary() || netErr.Timeout()
	}

	if urlErr, ok := err.(*url.Error); ok {
		return isRetryableFromCause(urlErr.Err)
	}

	// Check for syscall errors
	if opErr, ok := err.(*net.OpError); ok {
		return isRetryableFromCause(opErr.Err)
	}

	// Check for specific syscall errors
	if errno, ok := err.(syscall.Errno); ok {
		switch errno {
		case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.ECONNABORTED:
			return true
		case syscall.ENETUNREACH, syscall.EHOSTUNREACH:
			return true
		case syscall.ETIMEDOUT:
			return true
		default:
			return false
		}
	}

	return false
}

// isTemporaryFromCause determines if an error is temporary based on its underlying cause.
func isTemporaryFromCause(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.Error interface
	if netErr, ok := err.(net.Error); ok {
		return netErr.Temporary()
	}

	// Check for url.Error
	if urlErr, ok := err.(*url.Error); ok {
		return isTemporaryFromCause(urlErr.Err)
	}

	// Check for net.OpError
	if opErr, ok := err.(*net.OpError); ok {
		return isTemporaryFromCause(opErr.Err)
	}

	return false
}

// Error classification functions

// IsConnectionError checks if an error is a connection error.
func IsConnectionError(err error) bool {
	return isTransportErrorType(err, ErrorTypeConnection)
}

// IsTimeoutError checks if an error is a timeout error.
func IsTimeoutError(err error) bool {
	return isTransportErrorType(err, ErrorTypeTimeout)
}

// IsAuthenticationError checks if an error is an authentication error.
func IsAuthenticationError(err error) bool {
	return isTransportErrorType(err, ErrorTypeAuthentication)
}

// IsAuthorizationError checks if an error is an authorization error.
func IsAuthorizationError(err error) bool {
	return isTransportErrorType(err, ErrorTypeAuthorization)
}

// IsProtocolError checks if an error is a protocol error.
func IsProtocolError(err error) bool {
	return isTransportErrorType(err, ErrorTypeProtocol)
}

// IsSerializationError checks if an error is a serialization error.
func IsSerializationError(err error) bool {
	return isTransportErrorType(err, ErrorTypeSerialization)
}

// IsCompressionError checks if an error is a compression error.
func IsCompressionError(err error) bool {
	return isTransportErrorType(err, ErrorTypeCompression)
}

// IsConfigurationError checks if an error is a configuration error.
func IsConfigurationError(err error) bool {
	return isTransportErrorType(err, ErrorTypeConfiguration)
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	return isTransportErrorType(err, ErrorTypeValidation)
}

// IsNetworkError checks if an error is a network error.
func IsNetworkError(err error) bool {
	return isTransportErrorType(err, ErrorTypeNetwork)
}

// IsRetryError checks if an error is a retry error.
func IsRetryError(err error) bool {
	return isTransportErrorType(err, ErrorTypeRetry)
}

// IsCircuitBreakerError checks if an error is a circuit breaker error.
func IsCircuitBreakerError(err error) bool {
	return isTransportErrorType(err, ErrorTypeCircuitBreaker)
}

// IsRateLimitError checks if an error is a rate limit error.
func IsRateLimitError(err error) bool {
	return isTransportErrorType(err, ErrorTypeRateLimit)
}

// IsCapacityError checks if an error is a capacity error.
func IsCapacityError(err error) bool {
	return isTransportErrorType(err, ErrorTypeCapacity)
}

// IsUnsupportedError checks if an error is an unsupported error.
func IsUnsupportedError(err error) bool {
	return isTransportErrorType(err, ErrorTypeUnsupported)
}

// IsIncompatibleError checks if an error is an incompatible error.
func IsIncompatibleError(err error) bool {
	return isTransportErrorType(err, ErrorTypeIncompatible)
}

// IsRetryableError checks if an error is retryable.
func IsRetryableError(err error) bool {
	if transportErr, ok := err.(*TransportError); ok {
		return transportErr.IsRetryable()
	}
	return isRetryableFromCause(err)
}

// IsTemporaryError checks if an error is temporary.
func IsTemporaryError(err error) bool {
	if transportErr, ok := err.(*TransportError); ok {
		return transportErr.IsTemporary()
	}
	return isTemporaryFromCause(err)
}

// isTransportErrorType checks if an error is of a specific transport error type.
func isTransportErrorType(err error, errorType TransportErrorType) bool {
	if transportErr, ok := err.(*TransportError); ok {
		return transportErr.Type == errorType
	}

	// Check wrapped errors
	var transportErr *TransportError
	if errors.As(err, &transportErr) {
		return transportErr.Type == errorType
	}

	return false
}

// WrapError wraps an error in a transport error of the specified type.
func WrapError(errorType TransportErrorType, message string, cause error) *TransportError {
	return NewTransportError(errorType, message, cause)
}

// WrapWithMetadata wraps an error with metadata.
func WrapWithMetadata(errorType TransportErrorType, message string, cause error, metadata map[string]any) *TransportError {
	err := NewTransportError(errorType, message, cause)
	err.Metadata = metadata
	return err
}

// ErrorCode generates a unique error code for an error.
func ErrorCode(err error) string {
	if transportErr, ok := err.(*TransportError); ok {
		if transportErr.Code != "" {
			return transportErr.Code
		}
		return fmt.Sprintf("T%s", transportErr.Type)
	}
	return "TUNKNOWN"
}

// ErrorSeverity determines the severity of an error.
type ErrorSeverity int

const (
	SeverityLow ErrorSeverity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// GetErrorSeverity returns the severity of an error.
func GetErrorSeverity(err error) ErrorSeverity {
	if transportErr, ok := err.(*TransportError); ok {
		switch transportErr.Type {
		case ErrorTypeConfiguration, ErrorTypeValidation:
			return SeverityCritical
		case ErrorTypeAuthentication, ErrorTypeAuthorization:
			return SeverityHigh
		case ErrorTypeProtocol, ErrorTypeIncompatible:
			return SeverityHigh
		case ErrorTypeConnection, ErrorTypeNetwork:
			return SeverityMedium
		case ErrorTypeTimeout, ErrorTypeRetry:
			return SeverityMedium
		case ErrorTypeCircuitBreaker, ErrorTypeRateLimit:
			return SeverityMedium
		case ErrorTypeCapacity:
			return SeverityMedium
		case ErrorTypeSerialization, ErrorTypeCompression:
			return SeverityLow
		case ErrorTypeUnsupported:
			return SeverityLow
		case ErrorTypeInternal:
			return SeverityHigh
		default:
			return SeverityMedium
		}
	}
	return SeverityMedium
}

// ErrorGroup represents a collection of related errors.
type ErrorGroup struct {
	Errors []error `json:"errors"`
}

// Error implements the error interface for ErrorGroup.
func (g *ErrorGroup) Error() string {
	if len(g.Errors) == 0 {
		return "no errors"
	}

	if len(g.Errors) == 1 {
		return g.Errors[0].Error()
	}

	return fmt.Sprintf("multiple errors occurred: %d errors", len(g.Errors))
}

// Unwrap returns the combined error using errors.Join
func (g *ErrorGroup) Unwrap() error {
	if len(g.Errors) == 0 {
		return nil
	}
	return errors.Join(g.Errors...)
}

// Add adds an error to the group.
func (g *ErrorGroup) Add(err error) {
	if err != nil {
		g.Errors = append(g.Errors, err)
	}
}

// HasErrors returns true if the group contains any errors.
func (g *ErrorGroup) HasErrors() bool {
	return len(g.Errors) > 0
}

// First returns the first error in the group.
func (g *ErrorGroup) First() error {
	if len(g.Errors) == 0 {
		return nil
	}
	return g.Errors[0]
}

// Last returns the last error in the group.
func (g *ErrorGroup) Last() error {
	if len(g.Errors) == 0 {
		return nil
	}
	return g.Errors[len(g.Errors)-1]
}

// NewErrorGroup creates a new error group.
func NewErrorGroup() *ErrorGroup {
	return &ErrorGroup{
		Errors: make([]error, 0),
	}
}

// CombineErrors combines multiple errors into a single error.
// It uses errors.Join for proper error wrapping and unwrapping support.
func CombineErrors(errs ...error) error {
	// Filter out nil errors
	var nonNilErrors []error
	for _, err := range errs {
		if err != nil {
			nonNilErrors = append(nonNilErrors, err)
		}
	}

	if len(nonNilErrors) == 0 {
		return nil
	}

	// Use errors.Join for proper error chaining
	return errors.Join(nonNilErrors...)
}

// BatchError represents errors that occurred during batch operations.
type BatchError struct {
	Operation string
	Errors    map[int]error // Index -> Error mapping
	Total     int
	Failed    int
}

// Error implements the error interface for BatchError.
func (e *BatchError) Error() string {
	if e.Failed == 0 {
		return fmt.Sprintf("%s: all %d operations succeeded", e.Operation, e.Total)
	}
	return fmt.Sprintf("%s: %d of %d operations failed", e.Operation, e.Failed, e.Total)
}

// Unwrap returns the combined error using errors.Join
func (e *BatchError) Unwrap() error {
	if len(e.Errors) == 0 {
		return nil
	}

	// Convert map values to slice for errors.Join
	errs := make([]error, 0, len(e.Errors))
	for _, err := range e.Errors {
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// NewBatchError creates a new batch error.
func NewBatchError(operation string, total int) *BatchError {
	return &BatchError{
		Operation: operation,
		Errors:    make(map[int]error),
		Total:     total,
		Failed:    0,
	}
}

// AddError adds an error for a specific index in the batch.
func (e *BatchError) AddError(index int, err error) {
	if err != nil {
		e.Errors[index] = err
		e.Failed++
	}
}

// HasErrors returns true if any errors occurred.
func (e *BatchError) HasErrors() bool {
	return e.Failed > 0
}

// GetError returns the error for a specific index.
func (e *BatchError) GetError(index int) error {
	return e.Errors[index]
}
