// Package errors provides comprehensive error handling utilities for the ag-ui Go SDK.
// It includes custom error types, severity-based handling, context management, and retry logic.
package errors

import (
	"errors"
	"fmt"
	"time"
)

// Common sentinel errors
var (
	// ErrStateInvalid indicates an invalid state transition or state data
	ErrStateInvalid = errors.New("invalid state")

	// ErrValidationFailed indicates validation of input data failed
	ErrValidationFailed = errors.New("validation failed")

	// ErrConflict indicates a conflict in concurrent operations
	ErrConflict = errors.New("operation conflict")

	// ErrRetryExhausted indicates all retry attempts have been exhausted
	ErrRetryExhausted = errors.New("retry attempts exhausted")

	// ErrContextMissing indicates required context information is missing
	ErrContextMissing = errors.New("required context missing")

	// ErrOperationNotPermitted indicates the operation is not allowed
	ErrOperationNotPermitted = errors.New("operation not permitted")
)

// Severity levels for errors
type Severity int

const (
	// SeverityDebug indicates a debug-level error (informational)
	SeverityDebug Severity = iota
	// SeverityInfo indicates an informational error
	SeverityInfo
	// SeverityWarning indicates a warning that doesn't prevent operation
	SeverityWarning
	// SeverityError indicates a recoverable error
	SeverityError
	// SeverityCritical indicates a critical error requiring immediate attention
	SeverityCritical
	// SeverityFatal indicates a fatal error that requires termination
	SeverityFatal
)

// String returns the string representation of severity
func (s Severity) String() string {
	switch s {
	case SeverityDebug:
		return "DEBUG"
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	case SeverityCritical:
		return "CRITICAL"
	case SeverityFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// BaseError provides common fields for all custom error types
type BaseError struct {
	// Code is a machine-readable error code
	Code string

	// Message is a human-readable error message
	Message string

	// Severity indicates the error severity
	Severity Severity

	// Timestamp is when the error occurred
	Timestamp time.Time

	// Details provides additional error context
	Details map[string]interface{}

	// Cause is the underlying error, if any
	Cause error

	// Retryable indicates if the operation can be retried
	Retryable bool

	// RetryAfter suggests when to retry (if retryable)
	RetryAfter *time.Duration
}

// Error implements the error interface
func (e *BaseError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %s (caused by: %v)", e.Severity, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Severity, e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *BaseError) Unwrap() error {
	return e.Cause
}

// WithDetail adds a detail to the error
func (e *BaseError) WithDetail(key string, value interface{}) *BaseError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithCause adds an underlying cause to the error
func (e *BaseError) WithCause(cause error) *BaseError {
	e.Cause = cause
	return e
}

// WithRetry marks the error as retryable with a suggested retry time
func (e *BaseError) WithRetry(after time.Duration) *BaseError {
	e.Retryable = true
	e.RetryAfter = &after
	return e
}

// StateError represents errors related to state management
type StateError struct {
	*BaseError

	// StateID identifies the state that caused the error
	StateID string

	// CurrentState is the current state value (if available)
	CurrentState interface{}

	// ExpectedState is the expected state value (if applicable)
	ExpectedState interface{}

	// Transition describes the attempted state transition
	Transition string
}

// NewStateError creates a new state error
func NewStateError(code, message string) *StateError {
	return &StateError{
		BaseError: &BaseError{
			Code:      code,
			Message:   message,
			Severity:  SeverityError,
			Timestamp: time.Now(),
			Details:   make(map[string]interface{}),
		},
	}
}

// Error implements the error interface with state-specific details
func (e *StateError) Error() string {
	base := e.BaseError.Error()
	if e.StateID != "" {
		base = fmt.Sprintf("%s (state: %s)", base, e.StateID)
	}
	if e.Transition != "" {
		base = fmt.Sprintf("%s (transition: %s)", base, e.Transition)
	}
	return base
}

// WithStateID sets the state ID
func (e *StateError) WithStateID(id string) *StateError {
	e.StateID = id
	return e
}

// WithStates sets the current and expected states
func (e *StateError) WithStates(current, expected interface{}) *StateError {
	e.CurrentState = current
	e.ExpectedState = expected
	return e
}

// WithTransition sets the attempted transition
func (e *StateError) WithTransition(transition string) *StateError {
	e.Transition = transition
	return e
}

// ValidationError represents validation-related errors
type ValidationError struct {
	*BaseError

	// Field identifies the field that failed validation
	Field string

	// Value is the invalid value
	Value interface{}

	// Rule is the validation rule that failed
	Rule string

	// FieldErrors contains field-specific validation errors
	FieldErrors map[string][]string
}

// NewValidationError creates a new validation error
func NewValidationError(code, message string) *ValidationError {
	return &ValidationError{
		BaseError: &BaseError{
			Code:      code,
			Message:   message,
			Severity:  SeverityWarning,
			Timestamp: time.Now(),
			Details:   make(map[string]interface{}),
		},
		FieldErrors: make(map[string][]string),
	}
}

// Error implements the error interface with validation-specific details
func (e *ValidationError) Error() string {
	base := e.BaseError.Error()
	if e.Field != "" {
		base = fmt.Sprintf("%s (field: %s)", base, e.Field)
	}
	if e.Rule != "" {
		base = fmt.Sprintf("%s (rule: %s)", base, e.Rule)
	}
	return base
}

// WithField sets the field that failed validation
func (e *ValidationError) WithField(field string, value interface{}) *ValidationError {
	e.Field = field
	e.Value = value
	return e
}

// WithRule sets the validation rule that failed
func (e *ValidationError) WithRule(rule string) *ValidationError {
	e.Rule = rule
	return e
}

// AddFieldError adds a field-specific error
func (e *ValidationError) AddFieldError(field, message string) *ValidationError {
	e.FieldErrors[field] = append(e.FieldErrors[field], message)
	return e
}

// HasFieldErrors returns true if there are field-specific errors
func (e *ValidationError) HasFieldErrors() bool {
	return len(e.FieldErrors) > 0
}

// ConflictError represents conflict-related errors
type ConflictError struct {
	*BaseError

	// ResourceID identifies the resource in conflict
	ResourceID string

	// ResourceType describes the type of resource
	ResourceType string

	// ConflictingOperation describes the conflicting operation
	ConflictingOperation string

	// ResolutionStrategy suggests how to resolve the conflict
	ResolutionStrategy string
}

// NewConflictError creates a new conflict error
func NewConflictError(code, message string) *ConflictError {
	return &ConflictError{
		BaseError: &BaseError{
			Code:      code,
			Message:   message,
			Severity:  SeverityError,
			Timestamp: time.Now(),
			Details:   make(map[string]interface{}),
		},
	}
}

// Error implements the error interface with conflict-specific details
func (e *ConflictError) Error() string {
	base := e.BaseError.Error()
	if e.ResourceType != "" && e.ResourceID != "" {
		base = fmt.Sprintf("%s (resource: %s/%s)", base, e.ResourceType, e.ResourceID)
	}
	if e.ConflictingOperation != "" {
		base = fmt.Sprintf("%s (operation: %s)", base, e.ConflictingOperation)
	}
	return base
}

// WithResource sets the conflicting resource details
func (e *ConflictError) WithResource(resourceType, resourceID string) *ConflictError {
	e.ResourceType = resourceType
	e.ResourceID = resourceID
	return e
}

// WithOperation sets the conflicting operation
func (e *ConflictError) WithOperation(operation string) *ConflictError {
	e.ConflictingOperation = operation
	return e
}

// WithResolution sets the suggested resolution strategy
func (e *ConflictError) WithResolution(strategy string) *ConflictError {
	e.ResolutionStrategy = strategy
	return e
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's one of our custom errors
	switch e := err.(type) {
	case *BaseError:
		return e.Retryable
	case *StateError:
		return e.BaseError.Retryable
	case *ValidationError:
		return e.BaseError.Retryable
	case *ConflictError:
		return e.BaseError.Retryable
	}

	// Check wrapped errors
	var base *BaseError
	if errors.As(err, &base) {
		return base.Retryable
	}

	return false
}

// GetSeverity extracts the severity from an error
func GetSeverity(err error) Severity {
	if err == nil {
		return SeverityInfo
	}

	// Check if it's one of our custom errors
	switch e := err.(type) {
	case *BaseError:
		return e.Severity
	case *StateError:
		return e.BaseError.Severity
	case *ValidationError:
		return e.BaseError.Severity
	case *ConflictError:
		return e.BaseError.Severity
	}

	// Check wrapped errors
	var base *BaseError
	if errors.As(err, &base) {
		return base.Severity
	}

	// Default severity for unknown errors
	return SeverityError
}

// GetRetryAfter extracts the retry after duration from an error
func GetRetryAfter(err error) *time.Duration {
	if err == nil {
		return nil
	}

	// Check if it's one of our custom errors
	switch e := err.(type) {
	case *BaseError:
		return e.RetryAfter
	case *StateError:
		return e.BaseError.RetryAfter
	case *ValidationError:
		return e.BaseError.RetryAfter
	case *ConflictError:
		return e.BaseError.RetryAfter
	}

	// Check wrapped errors
	var base *BaseError
	if errors.As(err, &base) {
		return base.RetryAfter
	}

	return nil
}