package errors

import (
	"errors"
	"fmt"
	"time"
)

// ErrorCategory represents the category of error
type ErrorCategory string

const (
	CategoryAuthentication ErrorCategory = "authentication"
	CategoryCache          ErrorCategory = "cache"
	CategoryValidation     ErrorCategory = "validation"
	CategoryNetwork        ErrorCategory = "network"
	CategorySerialization  ErrorCategory = "serialization"
	CategoryTimeout        ErrorCategory = "timeout"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	SeverityInfo    ErrorSeverity = "info"
	SeverityWarning ErrorSeverity = "warning"
	SeverityError   ErrorSeverity = "error"
	SeverityCritical ErrorSeverity = "critical"
)

// BaseError is the foundation for all structured errors
type BaseError struct {
	Category    ErrorCategory          `json:"category"`
	Severity    ErrorSeverity          `json:"severity"`
	Message     string                 `json:"message"`
	Code        string                 `json:"code"`
	Timestamp   time.Time              `json:"timestamp"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Cause       error                  `json:"-"`
	Retryable   bool                   `json:"retryable"`
	Suggestions []string               `json:"suggestions,omitempty"`
}

func (e *BaseError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Category, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Category, e.Code, e.Message)
}

func (e *BaseError) Unwrap() error {
	return e.Cause
}

func (e *BaseError) Is(target error) bool {
	if t, ok := target.(*BaseError); ok {
		return e.Category == t.Category && e.Code == t.Code
	}
	return false
}

// AuthenticationError represents authentication-related errors
type AuthenticationError struct {
	*BaseError
	UserID      string `json:"user_id,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// Authentication error codes
const (
	AuthErrorInvalidCredentials = "INVALID_CREDENTIALS"
	AuthErrorTokenExpired       = "TOKEN_EXPIRED"
	AuthErrorInsufficientPerms  = "INSUFFICIENT_PERMISSIONS"
	AuthErrorAuthRequired       = "AUTHENTICATION_REQUIRED"
	AuthErrorInvalidToken       = "INVALID_TOKEN"
	AuthErrorUserDisabled       = "USER_DISABLED"
	AuthErrorRateLimited        = "RATE_LIMITED"
)

func NewAuthenticationError(code, message string) *AuthenticationError {
	return &AuthenticationError{
		BaseError: &BaseError{
			Category:  CategoryAuthentication,
			Severity:  SeverityError,
			Code:      code,
			Message:   message,
			Timestamp: time.Now(),
			Retryable: false,
		},
	}
}

func (e *AuthenticationError) WithUser(userID string) *AuthenticationError {
	e.UserID = userID
	return e
}

func (e *AuthenticationError) WithTokenType(tokenType string) *AuthenticationError {
	e.TokenType = tokenType
	return e
}

func (e *AuthenticationError) WithExpiration(expiresAt time.Time) *AuthenticationError {
	e.ExpiresAt = &expiresAt
	return e
}

func (e *AuthenticationError) WithPermissions(permissions []string) *AuthenticationError {
	e.Permissions = permissions
	return e
}

func (e *AuthenticationError) IsAuthenticationError() bool {
	return true
}

// CacheError represents cache-related errors
type CacheError struct {
	*BaseError
	CacheLevel string `json:"cache_level"` // L1, L2, etc.
	Key        string `json:"key,omitempty"`
	Operation  string `json:"operation"` // get, set, delete, etc.
	Size       int64  `json:"size,omitempty"`
}

// Cache error codes
const (
	CacheErrorConnectionFailed = "CONNECTION_FAILED"
	CacheErrorTimeout          = "TIMEOUT"
	CacheErrorKeyNotFound      = "KEY_NOT_FOUND"
	CacheErrorSerializationFailed = "SERIALIZATION_FAILED"
	CacheErrorCompressionFailed = "COMPRESSION_FAILED"
	CacheErrorEvictionFailed   = "EVICTION_FAILED"
	CacheErrorL2Unavailable    = "L2_UNAVAILABLE"
	CacheErrorInvalidOperation = "INVALID_OPERATION"
)

func NewCacheError(code, message string) *CacheError {
	retryable := code == CacheErrorTimeout || code == CacheErrorConnectionFailed || code == CacheErrorL2Unavailable
	
	return &CacheError{
		BaseError: &BaseError{
			Category:  CategoryCache,
			Severity:  SeverityError,
			Code:      code,
			Message:   message,
			Timestamp: time.Now(),
			Retryable: retryable,
		},
	}
}

func (e *CacheError) WithLevel(level string) *CacheError {
	e.CacheLevel = level
	return e
}

func (e *CacheError) WithKey(key string) *CacheError {
	e.Key = key
	return e
}

func (e *CacheError) WithOperation(operation string) *CacheError {
	e.Operation = operation
	return e
}

func (e *CacheError) WithSize(size int64) *CacheError {
	e.Size = size
	return e
}

func (e *CacheError) WithCause(cause error) *CacheError {
	e.Cause = cause
	return e
}

func (e *CacheError) IsCacheError() bool {
	return true
}

// ValidationError represents validation-related errors
type ValidationError struct {
	*BaseError
	RuleID      string `json:"rule_id"`
	EventID     string `json:"event_id,omitempty"`
	EventType   string `json:"event_type,omitempty"`
	FieldPath   string `json:"field_path,omitempty"`
	ActualValue interface{} `json:"actual_value,omitempty"`
	ExpectedValue interface{} `json:"expected_value,omitempty"`
}

// Validation error codes
const (
	ValidationErrorRequired       = "REQUIRED_FIELD_MISSING"
	ValidationErrorInvalidFormat  = "INVALID_FORMAT"
	ValidationErrorOutOfRange     = "OUT_OF_RANGE"
	ValidationErrorInvalidType    = "INVALID_TYPE"
	ValidationErrorConstraintViolation = "CONSTRAINT_VIOLATION"
	ValidationErrorSequenceError = "SEQUENCE_ERROR"
	ValidationErrorDuplicateID   = "DUPLICATE_ID"
	ValidationErrorOrphanedEvent = "ORPHANED_EVENT"
)

func NewValidationError(code, message string) *ValidationError {
	return &ValidationError{
		BaseError: &BaseError{
			Category:  CategoryValidation,
			Severity:  SeverityError,
			Code:      code,
			Message:   message,
			Timestamp: time.Now(),
			Retryable: false,
		},
	}
}

func (e *ValidationError) WithRule(ruleID string) *ValidationError {
	e.RuleID = ruleID
	return e
}

func (e *ValidationError) WithEvent(eventID, eventType string) *ValidationError {
	e.EventID = eventID
	e.EventType = eventType
	return e
}

func (e *ValidationError) WithField(fieldPath string, actualValue, expectedValue interface{}) *ValidationError {
	e.FieldPath = fieldPath
	e.ActualValue = actualValue
	e.ExpectedValue = expectedValue
	return e
}

func (e *ValidationError) IsValidationError() bool {
	return true
}

// Helper functions for error detection

// IsAuthenticationError checks if an error is an authentication error
func IsAuthenticationError(err error) bool {
	var authErr *AuthenticationError
	return errors.As(err, &authErr)
}

// IsCacheError checks if an error is a cache error
func IsCacheError(err error) bool {
	var cacheErr *CacheError
	return errors.As(err, &cacheErr)
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var baseErr *BaseError
	if errors.As(err, &baseErr) {
		return baseErr.Retryable
	}
	
	// Check for specific error types
	var authErr *AuthenticationError
	if errors.As(err, &authErr) && authErr.BaseError != nil {
		return authErr.BaseError.Retryable
	}
	
	var cacheErr *CacheError
	if errors.As(err, &cacheErr) && cacheErr.BaseError != nil {
		return cacheErr.BaseError.Retryable
	}
	
	var validationErr *ValidationError
	if errors.As(err, &validationErr) && validationErr.BaseError != nil {
		return validationErr.BaseError.Retryable
	}
	
	return false
}

// GetErrorCode extracts the error code from a structured error
func GetErrorCode(err error) string {
	var baseErr *BaseError
	if errors.As(err, &baseErr) {
		return baseErr.Code
	}
	
	// Check for specific error types
	var authErr *AuthenticationError
	if errors.As(err, &authErr) && authErr.BaseError != nil {
		return authErr.BaseError.Code
	}
	
	var cacheErr *CacheError
	if errors.As(err, &cacheErr) && cacheErr.BaseError != nil {
		return cacheErr.BaseError.Code
	}
	
	var validationErr *ValidationError
	if errors.As(err, &validationErr) && validationErr.BaseError != nil {
		return validationErr.BaseError.Code
	}
	
	return ""
}

// GetErrorCategory extracts the error category from a structured error
func GetErrorCategory(err error) ErrorCategory {
	var baseErr *BaseError
	if errors.As(err, &baseErr) {
		return baseErr.Category
	}
	
	// Check for specific error types
	var authErr *AuthenticationError
	if errors.As(err, &authErr) && authErr.BaseError != nil {
		return authErr.BaseError.Category
	}
	
	var cacheErr *CacheError
	if errors.As(err, &cacheErr) && cacheErr.BaseError != nil {
		return cacheErr.BaseError.Category
	}
	
	var validationErr *ValidationError
	if errors.As(err, &validationErr) && validationErr.BaseError != nil {
		return validationErr.BaseError.Category
	}
	
	return ""
}

// HasErrorCode checks if an error has a specific error code
func HasErrorCode(err error, code string) bool {
	return GetErrorCode(err) == code
}

// Legacy error code mappings for backward compatibility
var LegacyErrorCodeMap = map[string]string{
	"AUTH_VALIDATION":     AuthErrorAuthRequired,
	"POST_AUTH_VALIDATION": AuthErrorInsufficientPerms,
}

// ConvertLegacyError converts legacy string-based errors to structured errors
func ConvertLegacyError(ruleID, message string) error {
	if code, exists := LegacyErrorCodeMap[ruleID]; exists {
		authErr := NewAuthenticationError(code, message)
		// Add rule ID to context since AuthenticationError doesn't have WithRule method
		if authErr.Context == nil {
			authErr.Context = make(map[string]interface{})
		}
		authErr.Context["rule_id"] = ruleID
		return authErr
	}
	
	// Default to validation error for unknown rule IDs
	return NewValidationError(ValidationErrorConstraintViolation, message).WithRule(ruleID)
}