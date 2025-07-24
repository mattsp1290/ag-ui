// Package tools provides a comprehensive error handling system for tool operations.
//
// The error system provides:
//   - Structured error types with categorization (validation, execution, timeout, etc.)
//   - Consistent error codes for machine-readable error handling
//   - Rich context including tool IDs, operation details, and retry information
//   - Error wrapping to preserve original error causes
//   - Helper functions for creating common error types
//   - Circuit breaker pattern for handling repeated failures
//
// Example usage:
//
//	// Create a validation error
//	err := NewValidationError(CodeParameterMissing, "required parameter 'name' is missing", toolID)
//
//	// Create an execution error with retry information
//	err := NewExecutionError(CodeExecutionFailed, "temporary failure", toolID).
//	    WithRetry(5 * time.Second)
//
//	// Create an IO error with cause
//	err := NewIOError(CodeFileOpenFailed, "cannot open config file", "/path/to/file", originalErr)
//
package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Common error variables for tool operations.
var (
	// ErrToolNotFound indicates a requested tool doesn't exist
	ErrToolNotFound = errors.New("tool not found")

	// ErrInvalidParameters indicates the provided parameters are invalid
	ErrInvalidParameters = errors.New("invalid parameters")

	// ErrExecutionTimeout indicates tool execution exceeded timeout
	ErrExecutionTimeout = errors.New("execution timeout")

	// ErrExecutionCanceled indicates tool execution was canceled
	ErrExecutionCanceled = errors.New("execution canceled")

	// ErrRateLimitExceeded indicates rate limit was exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// ErrMaxConcurrencyReached indicates max concurrent executions reached
	ErrMaxConcurrencyReached = errors.New("maximum concurrent executions reached")

	// ErrToolPanicked indicates the tool execution panicked
	ErrToolPanicked = errors.New("tool execution panicked")

	// ErrStreamingNotSupported indicates tool doesn't support streaming
	ErrStreamingNotSupported = errors.New("streaming not supported")

	// ErrCircularDependency indicates a circular tool dependency
	ErrCircularDependency = errors.New("circular dependency detected")
)

// ToolError represents a detailed error from tool operations.
// It provides rich error information including type categorization,
// machine-readable codes, retry hints, and contextual details.
//
// ToolError implements the error interface and supports error wrapping
// for compatibility with errors.Is and errors.As.
//
// Example usage:
//
//	err := NewToolError(ErrorTypeValidation, "MISSING_FIELD", "required field 'name' is missing").
//		WithToolID("data-processor").
//		WithDetail("field", "name").
//		WithCause(originalErr)
//	
//	// Check error type
//	if errors.Is(err, ErrInvalidParameters) {
//		// Handle validation error
//	}
//	
//	// Extract ToolError
//	var toolErr *ToolError
//	if errors.As(err, &toolErr) {
//		if toolErr.Retryable {
//			// Retry after suggested duration
//		}
//	}
type ToolError struct {
	// Type categorizes the error
	Type ErrorType

	// Code is a machine-readable error code
	Code string

	// Message is a human-readable error message
	Message string

	// ToolID identifies the tool that caused the error
	ToolID string

	// Details provides additional error context
	Details map[string]interface{}

	// Cause is the underlying error, if any
	Cause error

	// Timestamp is when the error occurred
	Timestamp time.Time

	// Retryable indicates if the operation can be retried
	Retryable bool

	// RetryAfter suggests when to retry (if retryable)
	RetryAfter *time.Duration
}

// ErrorType categorizes tool errors for structured error handling.
// Each type represents a class of errors with similar characteristics
// and handling requirements.
type ErrorType string

const (
	// ErrorTypeValidation indicates parameter validation errors
	ErrorTypeValidation ErrorType = "validation"

	// ErrorTypeExecution indicates runtime execution errors
	ErrorTypeExecution ErrorType = "execution"

	// ErrorTypeTimeout indicates timeout errors
	ErrorTypeTimeout ErrorType = "timeout"

	// ErrorTypeCancellation indicates cancellation errors
	ErrorTypeCancellation ErrorType = "cancellation"

	// ErrorTypeRateLimit indicates rate limiting errors
	ErrorTypeRateLimit ErrorType = "rate_limit"

	// ErrorTypeConcurrency indicates concurrency limit errors
	ErrorTypeConcurrency ErrorType = "concurrency"

	// ErrorTypeDependency indicates dependency resolution errors
	ErrorTypeDependency ErrorType = "dependency"

	// ErrorTypeInternal indicates internal system errors
	ErrorTypeInternal ErrorType = "internal"

	// ErrorTypeProvider indicates AI provider-specific errors
	ErrorTypeProvider ErrorType = "provider"

	// ErrorTypeResource indicates resource exhaustion errors
	ErrorTypeResource ErrorType = "resource"

	// ErrorTypeConfiguration indicates configuration errors
	ErrorTypeConfiguration ErrorType = "configuration"

	// ErrorTypeIO indicates I/O related errors
	ErrorTypeIO ErrorType = "io"

	// ErrorTypeConflict indicates conflict resolution errors
	ErrorTypeConflict ErrorType = "conflict"

	// ErrorTypeMigration indicates version migration errors
	ErrorTypeMigration ErrorType = "migration"

	// ErrorTypeNetwork indicates network-related errors
	ErrorTypeNetwork ErrorType = "network"
)

// Error implements the error interface.
// It formats the error as a human-readable string including code,
// tool ID, message, and cause if present.
func (e *ToolError) Error() string {
	var parts []string

	if e.Code != "" {
		parts = append(parts, fmt.Sprintf("[%s]", e.Code))
	}

	if e.ToolID != "" {
		parts = append(parts, fmt.Sprintf("tool %q", e.ToolID))
	}

	parts = append(parts, e.Message)

	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf("caused by: %v", e.Cause))
	}

	return strings.Join(parts, ": ")
}

// Unwrap returns the underlying error.
// This enables compatibility with errors.Is and errors.As
// for checking wrapped errors.
func (e *ToolError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches a target error.
// It supports matching against common error variables and
// comparing ToolError instances by type and code.
// This method enables using errors.Is with ToolError.
func (e *ToolError) Is(target error) bool {
	if target == nil {
		return false
	}

	// Check against common errors
	switch target {
	case ErrToolNotFound:
		return e.Type == ErrorTypeValidation && strings.Contains(e.Message, "not found")
	case ErrInvalidParameters:
		return e.Type == ErrorTypeValidation
	case ErrExecutionTimeout:
		return e.Type == ErrorTypeTimeout
	case ErrExecutionCanceled:
		return e.Type == ErrorTypeCancellation
	case ErrRateLimitExceeded:
		return e.Type == ErrorTypeRateLimit
	case ErrMaxConcurrencyReached:
		return e.Type == ErrorTypeConcurrency
	}

	// Check if target is also a ToolError
	if targetErr, ok := target.(*ToolError); ok {
		return e.Type == targetErr.Type && e.Code == targetErr.Code
	}

	return false
}

// NewToolError creates a new tool error.
// This is the primary constructor for creating structured errors
// in the tools package.
//
// Parameters:
//   - errType: Categorizes the error (validation, execution, etc.)
//   - code: Machine-readable error code for programmatic handling
//   - message: Human-readable error description
//
// The returned error can be enhanced with additional context
// using the fluent API methods (WithToolID, WithCause, etc.).
func NewToolError(errType ErrorType, code, message string) *ToolError {
	return &ToolError{
		Type:      errType,
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}
}

// WithToolID adds a tool ID to the error.
// This identifies which tool caused the error, useful for debugging
// and error reporting. Returns self for method chaining.
func (e *ToolError) WithToolID(toolID string) *ToolError {
	e.ToolID = toolID
	return e
}

// WithCause adds an underlying cause to the error.
// This preserves the error chain for debugging and enables
// errors.Is/As to work with wrapped errors.
// Returns self for method chaining.
func (e *ToolError) WithCause(cause error) *ToolError {
	e.Cause = cause
	return e
}

// WithDetail adds a detail to the error.
// Details provide additional context about the error,
// such as parameter values, field names, or state information.
// Returns self for method chaining.
//
// Example:
//
//	err.WithDetail("parameter", "timeout").
//	   WithDetail("value", "30s").
//	   WithDetail("expected", "1m-1h")
func (e *ToolError) WithDetail(key string, value interface{}) *ToolError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithRetry marks the error as retryable.
// This indicates that the operation can be safely retried
// and suggests when to retry.
// Returns self for method chaining.
//
// Example:
//
//	err.WithRetry(5 * time.Second) // Retry after 5 seconds
func (e *ToolError) WithRetry(after time.Duration) *ToolError {
	e.Retryable = true
	e.RetryAfter = &after
	return e
}

// ErrorHandler provides centralized error handling for tool operations.
// It supports error transformation, notification, and recovery strategies.
//
// Features:
//   - Error transformation for consistent formatting
//   - Error listeners for logging and monitoring
//   - Recovery strategies for automatic error recovery
//   - Context-aware error wrapping
//
// Example usage:
//
//	handler := NewErrorHandler()
//	
//	// Add error transformer
//	handler.AddTransformer(func(err *ToolError) *ToolError {
//		if err.Type == ErrorTypeTimeout {
//			err.WithRetry(30 * time.Second)
//		}
//		return err
//	})
//	
//	// Add error listener for logging
//	handler.AddListener(func(err *ToolError) {
//		log.Printf("Tool error: %v", err)
//	})
//	
//	// Set recovery strategy
//	handler.SetRecoveryStrategy(ErrorTypeNetwork, retryStrategy)
type ErrorHandler struct {
	// ErrorTransformers allow customizing error messages
	transformers []ErrorTransformer

	// ErrorListeners are notified of errors
	listeners []ErrorListener

	// RecoveryStrategies define how to recover from errors
	strategies map[ErrorType]RecoveryStrategy
}

// ErrorTransformer modifies errors before they're returned.
// Transformers can enrich errors with additional context,
// modify error messages, or add retry information.
type ErrorTransformer func(*ToolError) *ToolError

// ErrorListener is notified when errors occur.
// Listeners are useful for logging, metrics collection,
// alerting, or triggering compensating actions.
type ErrorListener func(*ToolError)

// RecoveryStrategy defines how to recover from an error.
// Strategies can implement retry logic, fallback behavior,
// circuit breaking, or other recovery mechanisms.
// Returns nil if recovery succeeded, or an error if recovery failed.
type RecoveryStrategy func(context.Context, *ToolError) error

// NewErrorHandler creates a new error handler.
// The handler starts with empty transformer and listener lists,
// and no recovery strategies. Configure it using the Add* and Set* methods.
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		transformers: []ErrorTransformer{},
		listeners:    []ErrorListener{},
		strategies:   make(map[ErrorType]RecoveryStrategy),
	}
}

// HandleError processes an error through the error handling pipeline.
// It converts errors to ToolError format, applies transformers,
// and notifies listeners.
//
// Processing steps:
//   1. Convert to ToolError if needed
//   2. Apply all transformers in order
//   3. Notify all listeners
//   4. Return the processed error
func (h *ErrorHandler) HandleError(err error, toolID string) error {
	// Handle nil error
	if err == nil {
		err = errors.New("nil error")
	}

	// Convert to ToolError if needed
	toolErr, ok := err.(*ToolError)
	if !ok {
		toolErr = h.wrapError(err, toolID)
	}

	// Apply transformers
	for _, transformer := range h.transformers {
		toolErr = transformer(toolErr)
	}

	// Notify listeners
	for _, listener := range h.listeners {
		listener(toolErr)
	}

	return toolErr
}

// Recover attempts to recover from an error.
// It looks up a recovery strategy based on the error type
// and executes it. If no strategy exists or recovery fails,
// the original error is returned.
//
// The context parameter allows recovery strategies to respect
// cancellation and timeouts.
func (h *ErrorHandler) Recover(ctx context.Context, err error) error {
	toolErr, ok := err.(*ToolError)
	if !ok {
		return err
	}

	strategy, exists := h.strategies[toolErr.Type]
	if !exists {
		return err
	}

	return strategy(ctx, toolErr)
}

// AddTransformer adds an error transformer.
// Transformers are applied in the order they were added.
// Each transformer receives the error from the previous transformer.
func (h *ErrorHandler) AddTransformer(transformer ErrorTransformer) {
	h.transformers = append(h.transformers, transformer)
}

// AddListener adds an error listener.
// Listeners are called after all transformers have been applied.
// Multiple listeners can be added and are called in order.
func (h *ErrorHandler) AddListener(listener ErrorListener) {
	h.listeners = append(h.listeners, listener)
}

// SetRecoveryStrategy sets a recovery strategy for an error type.
// Only one strategy can be set per error type.
// Setting a new strategy replaces any existing strategy.
func (h *ErrorHandler) SetRecoveryStrategy(errType ErrorType, strategy RecoveryStrategy) {
	h.strategies[errType] = strategy
}

// wrapError converts a generic error to a ToolError.
func (h *ErrorHandler) wrapError(err error, toolID string) *ToolError {
	// Check for specific error types
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return NewToolError(ErrorTypeTimeout, "TIMEOUT", "execution timeout exceeded").
			WithToolID(toolID).
			WithCause(err)

	case errors.Is(err, context.Canceled):
		return NewToolError(ErrorTypeCancellation, "CANCELED", "execution was canceled").
			WithToolID(toolID).
			WithCause(err)

	default:
		return NewToolError(ErrorTypeExecution, "EXECUTION_ERROR", err.Error()).
			WithToolID(toolID).
			WithCause(err)
	}
}

// ValidationErrorBuilder helps build detailed validation errors.
// It accumulates multiple validation errors and field-specific errors
// into a single structured ToolError.
//
// This is useful for validating complex inputs where multiple
// errors should be reported at once.
//
// Example usage:
//
//	builder := NewValidationErrorBuilder()
//	
//	if name == "" {
//		builder.AddFieldError("name", "field is required")
//	}
//	
//	if age < 0 {
//		builder.AddFieldError("age", "must be non-negative")
//	}
//	
//	if builder.HasErrors() {
//		return builder.Build("user-validator")
//	}
type ValidationErrorBuilder struct {
	errors []string
	fields map[string][]string
}

// NewValidationErrorBuilder creates a new validation error builder.
// The builder starts with no errors. Use AddError and AddFieldError
// to accumulate validation errors.
func NewValidationErrorBuilder() *ValidationErrorBuilder {
	return &ValidationErrorBuilder{
		errors: []string{},
		fields: make(map[string][]string),
	}
}

// AddError adds a general validation error.
// These are validation errors that don't relate to a specific field.
// Returns self for method chaining.
func (b *ValidationErrorBuilder) AddError(message string) *ValidationErrorBuilder {
	b.errors = append(b.errors, message)
	return b
}

// AddFieldError adds a field-specific validation error.
// Multiple errors can be added for the same field.
// Returns self for method chaining.
func (b *ValidationErrorBuilder) AddFieldError(field, message string) *ValidationErrorBuilder {
	b.fields[field] = append(b.fields[field], message)
	return b
}

// Build creates a ToolError from the validation errors.
// Returns nil if no errors have been added.
// The resulting error includes all general and field-specific errors
// with field errors included in the error details.
func (b *ValidationErrorBuilder) Build(toolID string) *ToolError {
	if len(b.errors) == 0 && len(b.fields) == 0 {
		return nil
	}

	// Build error message
	var messages []string
	messages = append(messages, b.errors...)

	for field, fieldErrors := range b.fields {
		for _, err := range fieldErrors {
			messages = append(messages, fmt.Sprintf("%s: %s", field, err))
		}
	}

	err := NewToolError(
		ErrorTypeValidation,
		"VALIDATION_FAILED",
		strings.Join(messages, "; "),
	).WithToolID(toolID)

	// Add field errors as details
	if len(b.fields) > 0 {
		_ = err.WithDetail("field_errors", b.fields) // Chainable method, result not needed
	}

	return err
}

// HasErrors returns true if there are any validation errors.
// Use this to check if validation failed before calling Build.
func (b *ValidationErrorBuilder) HasErrors() bool {
	return len(b.errors) > 0 || len(b.fields) > 0
}

// CircuitBreaker provides circuit breaker pattern for tool execution.
// It prevents cascading failures by temporarily blocking calls to
// failing tools, giving them time to recover.
//
// States:
//   - Closed: Normal operation, requests pass through
//   - Open: Requests blocked due to failures exceeding threshold
//   - Half-Open: Limited requests allowed to test recovery
//
// Example usage:
//
//	cb := NewCircuitBreaker(5, 30*time.Second)
//	
//	err := cb.Call(func() error {
//		return tool.Execute(ctx, params)
//	})
//	
//	if err != nil {
//		var toolErr *ToolError
//		if errors.As(err, &toolErr) && toolErr.Code == "CIRCUIT_OPEN" {
//			// Circuit is open, retry later
//		}
//	}
type CircuitBreaker struct {
	// Configuration
	failureThreshold int
	resetTimeout     time.Duration

	// State (protected by mutex)
	mu          sync.RWMutex
	failures    int
	lastFailure time.Time
	state       CircuitState
}

// CircuitState represents the circuit breaker state.
// The circuit breaker transitions between states based on
// success and failure patterns.
type CircuitState int

const (
	// CircuitClosed allows requests through
	CircuitClosed CircuitState = iota
	// CircuitOpen blocks requests
	CircuitOpen
	// CircuitHalfOpen allows limited requests for testing
	CircuitHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker.
//
// Parameters:
//   - threshold: Number of failures before opening the circuit
//   - resetTimeout: How long to wait before testing recovery
//
// The circuit breaker starts in the Closed state.
func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: threshold,
		resetTimeout:     resetTimeout,
		state:            CircuitClosed,
	}
}

// Call executes a function with circuit breaker protection.
// If the circuit is open, it returns an error immediately.
// Otherwise, it executes the function and records the result.
//
// The function should return an error if execution fails.
func (cb *CircuitBreaker) Call(fn func() error) error {
	if err := cb.canProceed(); err != nil {
		return err
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// canProceed checks if the circuit allows the call.
func (cb *CircuitBreaker) canProceed() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = CircuitHalfOpen
			cb.failures = 0
			return nil
		}
		return NewToolError(
			ErrorTypeExecution,
			"CIRCUIT_OPEN",
			"circuit breaker is open",
		).WithRetry(cb.resetTimeout - time.Since(cb.lastFailure))

	case CircuitHalfOpen, CircuitClosed:
		return nil

	default:
		return nil
	}
}

// recordResult updates circuit breaker state based on result.
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err == nil {
		if cb.state == CircuitHalfOpen {
			cb.state = CircuitClosed
		}
		cb.failures = 0
		return
	}

	cb.failures++
	cb.lastFailure = time.Now()

	// If we're in half-open state, any failure should immediately trip to open
	if cb.state == CircuitHalfOpen || cb.failures >= cb.failureThreshold {
		cb.state = CircuitOpen
	}
}

// GetState returns the current circuit breaker state.
// This is useful for monitoring and debugging.
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset manually resets the circuit breaker.
// This closes the circuit and resets the failure count.
// Use with caution as it bypasses the normal recovery testing.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.failures = 0
}

// Common error codes for consistent error handling.
// These codes provide machine-readable identifiers for specific error conditions.
// Use these constants instead of string literals for consistency.
const (
	// Validation error codes
	CodeNilTool              = "NIL_TOOL"
	CodeToolNotFound         = "TOOL_NOT_FOUND"
	CodeValidationFailed     = "VALIDATION_FAILED"
	CodeCustomValidationFailed = "CUSTOM_VALIDATION_FAILED"
	CodeNameConflict         = "NAME_CONFLICT"
	CodeParameterMissing     = "PARAMETER_MISSING"
	CodeParameterInvalid     = "PARAMETER_INVALID"
	CodeTypeCoercionFailed   = "TYPE_COERCION_FAILED"
	
	// Execution error codes
	CodeExecutionFailed      = "EXECUTION_FAILED"
	CodeExecutionPanic       = "EXECUTION_PANIC"
	CodeStreamingFailed      = "STREAMING_FAILED"
	CodeStreamingNotSupported = "STREAMING_NOT_SUPPORTED"
	CodeHookFailed           = "HOOK_FAILED"
	CodeAsyncNotEnabled      = "ASYNC_NOT_ENABLED"
	
	// Resource error codes
	CodeRateLimitExceeded    = "RATE_LIMIT_EXCEEDED"
	CodeQueueFull            = "QUEUE_FULL"
	CodeResourceExhausted    = "RESOURCE_EXHAUSTED"
	
	// Conflict resolution error codes
	CodeConflictResolutionFailed = "CONFLICT_RESOLUTION_FAILED"
	CodeUnknownConflictStrategy  = "UNKNOWN_CONFLICT_STRATEGY"
	CodeVersionComparisonFailed  = "VERSION_COMPARISON_FAILED"
	
	// Migration error codes
	CodeMigrationFailed      = "MIGRATION_FAILED"
	CodeMigrationCompatibilityFailed = "MIGRATION_COMPATIBILITY_FAILED"
	CodeParameterRemoved     = "PARAMETER_REMOVED"
	
	// IO error codes
	CodeFileOpenFailed       = "FILE_OPEN_FAILED"
	CodeDecodeFailed         = "DECODE_FAILED"
	CodeRegistrationFailed   = "REGISTRATION_FAILED"
	
	// Network error codes
	CodeRequestCreationFailed = "REQUEST_CREATION_FAILED"
	CodeHTTPRequestFailed    = "HTTP_REQUEST_FAILED"
	CodeHTTPError            = "HTTP_ERROR"
	
	// Configuration error codes
	CodeHotReloadingDisabled = "HOT_RELOADING_DISABLED"
	CodeFileAlreadyWatched   = "FILE_ALREADY_WATCHED"
	CodeFileNotWatched       = "FILE_NOT_WATCHED"
	
	// Dependency error codes
	CodeDependencyNotFound   = "DEPENDENCY_NOT_FOUND"
	CodeCircularDependency   = "CIRCULAR_DEPENDENCY"
	CodeDependencyDepthExceeded = "DEPENDENCY_DEPTH_EXCEEDED"
	CodeVersionConstraintFailed = "VERSION_CONSTRAINT_FAILED"
)

// Helper functions for creating common error types.
// These functions provide convenient shortcuts for creating
// properly typed and formatted errors.

// NewValidationError creates a validation error with proper context.
// This is a convenience function for creating validation errors
// with consistent type and tool ID.
func NewValidationError(code, message, toolID string) *ToolError {
	return NewToolError(ErrorTypeValidation, code, message).
		WithToolID(toolID)
}

// NewExecutionError creates an execution error with proper context.
// This is a convenience function for creating execution errors
// with consistent type and tool ID.
func NewExecutionError(code, message, toolID string) *ToolError {
	return NewToolError(ErrorTypeExecution, code, message).
		WithToolID(toolID)
}

// NewDependencyError creates a dependency error with proper context.
// This is a convenience function for creating dependency resolution errors
// with consistent type and tool ID.
func NewDependencyError(code, message, toolID string) *ToolError {
	return NewToolError(ErrorTypeDependency, code, message).
		WithToolID(toolID)
}

// NewIOError creates an IO error with proper context.
// Use this for file system and I/O related errors.
// The path parameter should contain the file or resource path.
func NewIOError(code, message string, path string, cause error) *ToolError {
	return NewToolError(ErrorTypeIO, code, message).
		WithDetail("path", path).
		WithCause(cause)
}

// NewNetworkError creates a network error with proper context.
// Use this for HTTP requests, API calls, and network-related errors.
// The url parameter should contain the target URL or endpoint.
func NewNetworkError(code, message string, url string, cause error) *ToolError {
	return NewToolError(ErrorTypeNetwork, code, message).
		WithDetail("url", url).
		WithCause(cause)
}

// NewConflictError creates a conflict error with proper context.
// Use this when tool registration conflicts occur.
// Both tool IDs are included in the error details for debugging.
func NewConflictError(code, message string, existingToolID, newToolID string) *ToolError {
	return NewToolError(ErrorTypeConflict, code, message).
		WithDetail("existing_tool", existingToolID).
		WithDetail("new_tool", newToolID)
}

// NewMigrationError creates a migration error with proper context.
// Use this for version migration failures.
// Both versions are included in the error details for debugging.
func NewMigrationError(code, message string, fromVersion, toVersion string) *ToolError {
	return NewToolError(ErrorTypeMigration, code, message).
		WithDetail("from_version", fromVersion).
		WithDetail("to_version", toVersion)
}
