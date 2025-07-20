package errors

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

// Wrap wraps an error with additional context
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf wraps an error with formatted context
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// Is checks if an error matches a target error
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As attempts to extract a specific error type
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Cause returns the root cause of an error
func Cause(err error) error {
	for {
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		unwrapped := unwrapper.Unwrap()
		if unwrapped == nil {
			break
		}
		err = unwrapped
	}
	return err
}

// Chain creates a chain of errors
func Chain(errs ...error) error {
	var nonNil []error
	for _, err := range errs {
		if err != nil {
			nonNil = append(nonNil, err)
		}
	}

	switch len(nonNil) {
	case 0:
		return nil
	case 1:
		return nonNil[0]
	default:
		return &ChainedError{errors: nonNil}
	}
}

// ChainedError represents multiple errors
type ChainedError struct {
	errors []error
}

// Error returns the combined error message
func (e *ChainedError) Error() string {
	if len(e.errors) == 0 {
		return ""
	}

	var messages []string
	for _, err := range e.errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// Errors returns all errors in the chain
func (e *ChainedError) Errors() []error {
	return e.errors
}

// Unwrap returns the first error in the chain
func (e *ChainedError) Unwrap() error {
	if len(e.errors) > 0 {
		return e.errors[0]
	}
	return nil
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (0 = unlimited)
	MaxAttempts int

	// InitialDelay is the initial delay between retries
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Multiplier is the delay multiplier for exponential backoff
	Multiplier float64

	// Jitter adds randomness to delays (0.0 to 1.0)
	Jitter float64

	// RetryIf determines if an error should be retried
	RetryIf func(error) bool

	// OnRetry is called before each retry attempt
	OnRetry func(attempt int, err error, delay time.Duration)
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
		RetryIf:      IsRetryable,
	}
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, config *RetryConfig, fn func() error) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; config.MaxAttempts == 0 || attempt <= config.MaxAttempts; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if config.RetryIf != nil && !config.RetryIf(err) {
			return err
		}

		// Check if this is the last attempt
		if config.MaxAttempts > 0 && attempt >= config.MaxAttempts {
			break
		}

		// Calculate delay with jitter
		actualDelay := applyJitter(delay, config.Jitter)

		// Call retry callback if provided
		if config.OnRetry != nil {
			config.OnRetry(attempt, err, actualDelay)
		}

		// Wait or return if context is done
		select {
		case <-ctx.Done():
			return Chain(lastErr, ctx.Err())
		case <-time.After(actualDelay):
		}

		// Calculate next delay
		delay = calculateNextDelay(delay, config.Multiplier, config.MaxDelay)
	}

	return NewBaseError("RETRY_EXHAUSTED", "retry attempts exhausted").
		WithCause(lastErr).
		WithDetail("attempts", config.MaxAttempts)
}

// RetryWithBackoff is a convenience function for exponential backoff retry
func RetryWithBackoff(ctx context.Context, fn func() error) error {
	return Retry(ctx, DefaultRetryConfig(), fn)
}

// applyJitter adds randomness to a duration
func applyJitter(d time.Duration, jitter float64) time.Duration {
	if jitter <= 0 {
		return d
	}

	jitter = math.Min(jitter, 1.0)
	jitterRange := float64(d) * jitter
	jitterValue := (rand.Float64() * 2 * jitterRange) - jitterRange

	return time.Duration(float64(d) + jitterValue)
}

// calculateNextDelay calculates the next retry delay
func calculateNextDelay(current time.Duration, multiplier float64, max time.Duration) time.Duration {
	next := time.Duration(float64(current) * multiplier)
	if next > max {
		return max
	}
	return next
}

// Must panics if the error is not nil
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// MustValue returns the value or panics if there's an error
func MustValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

// IgnoreError executes a function and ignores any error
func IgnoreError(fn func() error) {
	_ = fn()
}

// NewBaseError creates a new base error
func NewBaseError(code, message string) *BaseError {
	return &BaseError{
		Code:      code,
		Message:   message,
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}
}

// ErrorCollector collects multiple errors
type ErrorCollector struct {
	errors []error
}

// NewErrorCollector creates a new error collector
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{
		errors: make([]error, 0),
	}
}

// Add adds an error to the collector
func (c *ErrorCollector) Add(err error) {
	if err != nil {
		c.errors = append(c.errors, err)
	}
}

// AddWithContext adds an error with context to the collector
func (c *ErrorCollector) AddWithContext(err error, context string) {
	if err != nil {
		c.errors = append(c.errors, Wrap(err, context))
	}
}

// HasErrors returns true if any errors have been collected
func (c *ErrorCollector) HasErrors() bool {
	return len(c.errors) > 0
}

// Count returns the number of collected errors
func (c *ErrorCollector) Count() int {
	return len(c.errors)
}

// Errors returns all collected errors
func (c *ErrorCollector) Errors() []error {
	result := make([]error, len(c.errors))
	copy(result, c.errors)
	return result
}

// Error returns the combined error or nil if no errors
func (c *ErrorCollector) Error() error {
	if len(c.errors) == 0 {
		return nil
	}
	return Chain(c.errors...)
}

// Clear removes all collected errors
func (c *ErrorCollector) Clear() {
	c.errors = c.errors[:0]
}

// First returns the first error or nil
func (c *ErrorCollector) First() error {
	if len(c.errors) > 0 {
		return c.errors[0]
	}
	return nil
}

// Last returns the last error or nil
func (c *ErrorCollector) Last() error {
	if len(c.errors) > 0 {
		return c.errors[len(c.errors)-1]
	}
	return nil
}

// Filter returns errors that match the filter function
func (c *ErrorCollector) Filter(fn func(error) bool) []error {
	var filtered []error
	for _, err := range c.errors {
		if fn(err) {
			filtered = append(filtered, err)
		}
	}
	return filtered
}

// ErrorStack provides stack-like error handling
type ErrorStack struct {
	errors []error
}

// NewErrorStack creates a new error stack
func NewErrorStack() *ErrorStack {
	return &ErrorStack{
		errors: make([]error, 0),
	}
}

// Push adds an error to the stack
func (s *ErrorStack) Push(err error) {
	if err != nil {
		s.errors = append(s.errors, err)
	}
}

// Pop removes and returns the top error
func (s *ErrorStack) Pop() error {
	if len(s.errors) == 0 {
		return nil
	}
	err := s.errors[len(s.errors)-1]
	s.errors = s.errors[:len(s.errors)-1]
	return err
}

// Peek returns the top error without removing it
func (s *ErrorStack) Peek() error {
	if len(s.errors) == 0 {
		return nil
	}
	return s.errors[len(s.errors)-1]
}

// IsEmpty returns true if the stack is empty
func (s *ErrorStack) IsEmpty() bool {
	return len(s.errors) == 0
}

// Size returns the number of errors in the stack
func (s *ErrorStack) Size() int {
	return len(s.errors)
}

// ToError converts the stack to a single error
func (s *ErrorStack) ToError() error {
	if len(s.errors) == 0 {
		return nil
	}
	return Chain(s.errors...)
}

// ErrorMatcher provides pattern matching for errors
type ErrorMatcher struct {
	matchers []func(error) bool
}

// NewErrorMatcher creates a new error matcher
func NewErrorMatcher() *ErrorMatcher {
	return &ErrorMatcher{
		matchers: make([]func(error) bool, 0),
	}
}

// WithCode matches errors with a specific code
func (m *ErrorMatcher) WithCode(code string) *ErrorMatcher {
	m.matchers = append(m.matchers, func(err error) bool {
		switch e := err.(type) {
		case *BaseError:
			return e.Code == code
		case *StateError:
			return e.BaseError.Code == code
		case *ValidationError:
			return e.BaseError.Code == code
		case *ConflictError:
			return e.BaseError.Code == code
		}
		var baseErr *BaseError
		if errors.As(err, &baseErr) {
			return baseErr.Code == code
		}
		return false
	})
	return m
}

// WithSeverity matches errors with a specific severity
func (m *ErrorMatcher) WithSeverity(severity Severity) *ErrorMatcher {
	m.matchers = append(m.matchers, func(err error) bool {
		return GetSeverity(err) == severity
	})
	return m
}

// WithType matches errors of a specific type
func (m *ErrorMatcher) WithType(target error) *ErrorMatcher {
	m.matchers = append(m.matchers, func(err error) bool {
		return errors.Is(err, target)
	})
	return m
}

// WithMessage matches errors containing a message
func (m *ErrorMatcher) WithMessage(substring string) *ErrorMatcher {
	m.matchers = append(m.matchers, func(err error) bool {
		return strings.Contains(err.Error(), substring)
	})
	return m
}

// Matches checks if an error matches all conditions
func (m *ErrorMatcher) Matches(err error) bool {
	for _, matcher := range m.matchers {
		if !matcher(err) {
			return false
		}
	}
	return true
}

// AnyMatch checks if any error in a slice matches
func (m *ErrorMatcher) AnyMatch(errors []error) bool {
	for _, err := range errors {
		if m.Matches(err) {
			return true
		}
	}
	return false
}

// AllMatch checks if all errors in a slice match
func (m *ErrorMatcher) AllMatch(errors []error) bool {
	if len(errors) == 0 {
		return false
	}
	for _, err := range errors {
		if !m.Matches(err) {
			return false
		}
	}
	return true
}

// RetryableFunc wraps a function to make it retryable
type RetryableFunc func() error

// WithRetry wraps a function with retry logic
func WithRetry(fn func() error, config *RetryConfig) RetryableFunc {
	return func() error {
		return Retry(context.Background(), config, fn)
	}
}

// WithTimeout wraps a function with a timeout
func WithTimeout(fn func(context.Context) error, timeout time.Duration) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return fn(ctx)
	}
}

// WithPanic wraps a function to recover from panics
func WithPanic(fn func() error) func() error {
	return func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = NewBaseError("PANIC", fmt.Sprintf("panic: %v", r)).
					WithDetail("panic_value", r)
			}
		}()
		return fn()
	}
}

// Encoding-specific error creation functions

// NewEncodingErrorWithFormat creates an encoding error with format details
func NewEncodingErrorWithFormat(code, message, format string) *EncodingError {
	return NewEncodingError(code, message).WithFormat(format)
}

// NewDecodingError creates a decoding error
func NewDecodingError(code, message string) *EncodingError {
	return NewEncodingError(code, message).WithOperation("decode")
}

// NewEncodingErrorWithMime creates an encoding error with MIME type
func NewEncodingErrorWithMime(code, message, mimeType string) *EncodingError {
	return NewEncodingError(code, message).WithMimeType(mimeType)
}

// NewSerializationError creates a serialization error
func NewSerializationError(code, message string) *EncodingError {
	return NewEncodingError(code, message).WithOperation("serialize")
}

// NewDeserializationError creates a deserialization error
func NewDeserializationError(code, message string) *EncodingError {
	return NewEncodingError(code, message).WithOperation("deserialize")
}

// NewStreamingError creates a streaming error
func NewStreamingError(code, message string) *EncodingError {
	return NewEncodingError(code, message).WithOperation("stream")
}

// NewCompressionError creates a compression error
func NewCompressionError(code, message string) *EncodingError {
	return NewEncodingError(code, message).WithOperation("compress")
}

// NewSecurityViolationError creates a security violation error
func NewSecurityViolationError(code, message, violationType string) *SecurityError {
	return NewSecurityError(code, message).WithViolationType(violationType)
}

// NewXSSError creates an XSS detection error
func NewXSSError(message, pattern string) *SecurityError {
	return NewSecurityError("XSS_DETECTED", message).
		WithViolationType("cross_site_scripting").
		WithPattern(pattern).
		WithRiskLevel("high")
}

// NewSQLInjectionError creates a SQL injection detection error
func NewSQLInjectionError(message, pattern string) *SecurityError {
	return NewSecurityError("SQL_INJECTION_DETECTED", message).
		WithViolationType("sql_injection").
		WithPattern(pattern).
		WithRiskLevel("critical")
}

// NewScriptInjectionError creates a script injection detection error
func NewScriptInjectionError(message, pattern string) *SecurityError {
	return NewSecurityError("SCRIPT_INJECTION_DETECTED", message).
		WithViolationType("script_injection").
		WithPattern(pattern).
		WithRiskLevel("high")
}

// NewDOSError creates a denial of service detection error
func NewDOSError(message, location string) *SecurityError {
	return NewSecurityError("DOS_ATTACK_DETECTED", message).
		WithViolationType("denial_of_service").
		WithLocation(location).
		WithRiskLevel("medium")
}

// NewPathTraversalError creates a path traversal detection error
func NewPathTraversalError(message, pattern string) *SecurityError {
	return NewSecurityError("PATH_TRAVERSAL_DETECTED", message).
		WithViolationType("path_traversal").
		WithPattern(pattern).
		WithRiskLevel("high")
}

// Error discrimination functions

// IsEncodingError checks if an error is an encoding error
func IsEncodingError(err error) bool {
	if err == nil {
		return false
	}
	var encodingErr *EncodingError
	return errors.As(err, &encodingErr)
}

// IsSecurityError checks if an error is a security error
func IsSecurityError(err error) bool {
	if err == nil {
		return false
	}
	var securityErr *SecurityError
	return errors.As(err, &securityErr)
}

// IsValidationError checks if an error is a validation error
func IsValidationErrorType(err error) bool {
	if err == nil {
		return false
	}
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// IsCancelError checks if an error is a cancellation error
func IsCancelError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.Canceled)
}

// IsConflictError checks if an error is a conflict error
func IsConflictErrorType(err error) bool {
	if err == nil {
		return false
	}
	var conflictErr *ConflictError
	return errors.As(err, &conflictErr)
}

// IsStateError checks if an error is a state error
func IsStateErrorType(err error) bool {
	if err == nil {
		return false
	}
	var stateErr *StateError
	return errors.As(err, &stateErr)
}

// IsFormatError checks if an error is related to format registration
func IsFormatError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrFormatNotRegistered) || errors.Is(err, ErrEncodingNotSupported)
}

// IsSecurityViolation checks if an error is a security violation
func IsSecurityViolation(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrSecurityViolation) || IsSecurityError(err)
}

// Common error codes for encoding system
const (
	// Registry error codes
	CodeFormatNotRegistered = "FORMAT_NOT_REGISTERED"
	CodeInvalidMimeType     = "INVALID_MIME_TYPE"
	CodeNilFactory          = "NIL_FACTORY"
	CodeEmptyMimeType       = "EMPTY_MIME_TYPE"
	
	// Encoding/Decoding error codes
	CodeEncodingFailed      = "ENCODING_FAILED"
	CodeDecodingFailed      = "DECODING_FAILED"
	CodeSerializationFailed = "SERIALIZATION_FAILED"
	CodeCompressionFailed   = "COMPRESSION_FAILED"
	CodeStreamingFailed     = "STREAMING_FAILED"
	CodeChunkingFailed      = "CHUNKING_FAILED"
	
	// Security error codes
	CodeSecurityViolation   = "SECURITY_VIOLATION"
	CodeXSSDetected         = "XSS_DETECTED"
	CodeSQLInjectionDetected = "SQL_INJECTION_DETECTED"
	CodeScriptInjectionDetected = "SCRIPT_INJECTION_DETECTED"
	CodeDOSDetected         = "DOS_ATTACK_DETECTED"
	CodeInvalidData         = "INVALID_DATA"
	CodeSizeExceeded        = "SIZE_EXCEEDED"
	CodeDepthExceeded       = "DEPTH_EXCEEDED"
	CodeNullByteDetected    = "NULL_BYTE_DETECTED"
	CodeInvalidUTF8         = "INVALID_UTF8"
	CodeHTMLNotAllowed      = "HTML_NOT_ALLOWED"
	CodeEntityExpansion     = "ENTITY_EXPANSION"
	CodeZipBomb             = "ZIP_BOMB"
	CodeExcessiveRepetition = "EXCESSIVE_REPETITION"
	
	// Validation error codes
	CodeValidationFailed    = "VALIDATION_FAILED"
	CodeMissingEvent        = "MISSING_EVENT"
	CodeMissingEventType    = "MISSING_EVENT_TYPE"
	CodeNegativeTimestamp   = "NEGATIVE_TIMESTAMP"
	CodeIDTooLong          = "ID_TOO_LONG"
	
	// Negotiation error codes
	CodeNegotiationFailed   = "NEGOTIATION_FAILED"
	CodeNoSuitableFormat    = "NO_SUITABLE_FORMAT"
	CodeUnsupportedFormat   = "UNSUPPORTED_FORMAT"
)