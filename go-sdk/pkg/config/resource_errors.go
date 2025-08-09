// Package config provides error types for resource management and security limits
package config

import (
	"errors"
	"fmt"
	"time"
)

// ResourceLimitError represents an error when a resource limit is exceeded
type ResourceLimitError struct {
	Resource    string `json:"resource"`     // Resource that exceeded the limit (e.g., "file_size", "memory_usage")
	Current     int64  `json:"current"`      // Current value that exceeded the limit
	Limit       int64  `json:"limit"`        // The limit that was exceeded
	Message     string `json:"message"`      // Human-readable error message
	Category    string `json:"category"`     // Error category for classification
}

// NewResourceLimitError creates a new resource limit error
func NewResourceLimitError(resource string, current, limit int64, message string) *ResourceLimitError {
	return &ResourceLimitError{
		Resource: resource,
		Current:  current,
		Limit:    limit,
		Message:  message,
		Category: "resource_limit",
	}
}

// Error implements the error interface
func (e *ResourceLimitError) Error() string {
	return fmt.Sprintf("resource limit exceeded [%s]: %s (current: %d, limit: %d)", 
		e.Resource, e.Message, e.Current, e.Limit)
}

// IsRecoverable returns true if this error might be recoverable by waiting or retrying
func (e *ResourceLimitError) IsRecoverable() bool {
	switch e.Resource {
	case "memory_usage", "key_count", "total_watchers", "key_watchers":
		return true // These might be recoverable as resources are freed
	case "file_size", "nesting_depth", "array_size", "string_length":
		return false // These are structural issues that require different data
	default:
		return false
	}
}

// GetSeverity returns the severity level of the error
func (e *ResourceLimitError) GetSeverity() string {
	switch e.Resource {
	case "file_size", "memory_usage":
		return "high" // These could indicate DoS attempts
	case "nesting_depth":
		return "high" // Could cause stack overflow
	case "total_watchers", "key_watchers":
		return "medium" // Could cause resource exhaustion but manageable
	default:
		return "low"
	}
}

// RateLimitError represents an error when a rate limit is exceeded
type RateLimitError struct {
	Operation     string        `json:"operation"`      // Operation being rate limited (e.g., "reload", "update")
	RateLimit     time.Duration `json:"rate_limit"`     // The rate limit interval
	TimeSinceLast time.Duration `json:"time_since_last"` // Time since last operation
	Message       string        `json:"message"`        // Human-readable error message
	Category      string        `json:"category"`       // Error category for classification
}

// NewRateLimitError creates a new rate limit error
func NewRateLimitError(operation string, rateLimit, timeSinceLast time.Duration, message string) *RateLimitError {
	return &RateLimitError{
		Operation:     operation,
		RateLimit:     rateLimit,
		TimeSinceLast: timeSinceLast,
		Message:       message,
		Category:      "rate_limit",
	}
}

// Error implements the error interface
func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded [%s]: %s (rate limit: %v, time since last: %v)", 
		e.Operation, e.Message, e.RateLimit, e.TimeSinceLast)
}

// IsRecoverable returns true since rate limit errors are always recoverable by waiting
func (e *RateLimitError) IsRecoverable() bool {
	return true
}

// GetRetryAfter returns the duration to wait before retrying
func (e *RateLimitError) GetRetryAfter() time.Duration {
	remaining := e.RateLimit - e.TimeSinceLast
	if remaining <= 0 {
		return 0
	}
	return remaining
}

// GetSeverity returns the severity level of the error
func (e *RateLimitError) GetSeverity() string {
	return "low" // Rate limiting is preventive, not critical
}

// StructureLimitError represents an error when configuration structure limits are exceeded
type StructureLimitError struct {
	StructureType string `json:"structure_type"` // Type of structure (e.g., "nesting_depth", "array_size")
	Current       int    `json:"current"`        // Current value that exceeded the limit
	Limit         int    `json:"limit"`          // The limit that was exceeded
	Message       string `json:"message"`        // Human-readable error message
	Path          string `json:"path,omitempty"` // Path to the problematic structure (if applicable)
	Category      string `json:"category"`       // Error category for classification
}

// NewStructureLimitError creates a new structure limit error
func NewStructureLimitError(structureType string, current, limit int, message string) *StructureLimitError {
	return &StructureLimitError{
		StructureType: structureType,
		Current:       current,
		Limit:         limit,
		Message:       message,
		Category:      "structure_limit",
	}
}

// Error implements the error interface
func (e *StructureLimitError) Error() string {
	pathInfo := ""
	if e.Path != "" {
		pathInfo = fmt.Sprintf(" at path '%s'", e.Path)
	}
	return fmt.Sprintf("structure limit exceeded [%s]%s: %s (current: %d, limit: %d)", 
		e.StructureType, pathInfo, e.Message, e.Current, e.Limit)
}

// IsRecoverable returns false since structure limits require different data
func (e *StructureLimitError) IsRecoverable() bool {
	return false
}

// GetSeverity returns the severity level of the error
func (e *StructureLimitError) GetSeverity() string {
	switch e.StructureType {
	case "nesting_depth":
		return "high" // Could cause stack overflow
	case "key_count":
		return "high" // Could cause memory exhaustion
	case "array_size":
		return "medium" // Could cause performance issues
	case "string_length":
		return "low" // Usually just inconvenient
	default:
		return "medium"
	}
}

// TimeoutError represents an error when an operation times out
type TimeoutError struct {
	Operation string        `json:"operation"` // Operation that timed out
	Timeout   time.Duration `json:"timeout"`   // The timeout duration
	Elapsed   time.Duration `json:"elapsed"`   // Time elapsed before timeout
	Message   string        `json:"message"`   // Human-readable error message
	Category  string        `json:"category"`  // Error category for classification
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(operation string, timeout, elapsed time.Duration, message string) *TimeoutError {
	return &TimeoutError{
		Operation: operation,
		Timeout:   timeout,
		Elapsed:   elapsed,
		Message:   message,
		Category:  "timeout",
	}
}

// Error implements the error interface
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("operation timed out [%s]: %s (timeout: %v, elapsed: %v)", 
		e.Operation, e.Message, e.Timeout, e.Elapsed)
}

// IsRecoverable returns true since timeouts might be recoverable with retry
func (e *TimeoutError) IsRecoverable() bool {
	return true
}

// GetSeverity returns the severity level of the error
func (e *TimeoutError) GetSeverity() string {
	switch e.Operation {
	case "load":
		return "high" // Loading failure is critical
	case "validate":
		return "medium" // Validation timeout is concerning but not critical
	case "watcher":
		return "low" // Watcher timeout is usually not critical
	default:
		return "medium"
	}
}

// SecurityError represents a security-related error
type SecurityError struct {
	Violation string      `json:"violation"` // Type of security violation
	Details   interface{} `json:"details"`   // Additional details about the violation
	Message   string      `json:"message"`   // Human-readable error message
	Category  string      `json:"category"`  // Error category for classification
	Severity  string      `json:"severity"`  // Severity level
}

// NewSecurityError creates a new security error
func NewSecurityError(violation string, details interface{}, message string) *SecurityError {
	return &SecurityError{
		Violation: violation,
		Details:   details,
		Message:   message,
		Category:  "security",
		Severity:  "high", // Security errors are always high severity
	}
}

// Error implements the error interface
func (e *SecurityError) Error() string {
	return fmt.Sprintf("security violation [%s]: %s", e.Violation, e.Message)
}

// IsRecoverable returns false since security violations should not be retried
func (e *SecurityError) IsRecoverable() bool {
	return false
}

// GetSeverity returns the severity level of the error
func (e *SecurityError) GetSeverity() string {
	return e.Severity
}

// ResourceErrorHandler provides centralized error handling for resource-related errors
type ResourceErrorHandler struct {
	// Configuration for error handling behavior
	EnableGracefulDegradation bool
	EnableErrorRecovery       bool
	EnableMetrics            bool
	
	// Callbacks for different error types
	OnResourceLimitExceeded func(*ResourceLimitError)
	OnRateLimitExceeded     func(*RateLimitError) 
	OnStructureLimitExceeded func(*StructureLimitError)
	OnTimeoutExceeded       func(*TimeoutError)
	OnSecurityViolation     func(*SecurityError)
}

// NewResourceErrorHandler creates a new resource error handler with default settings
func NewResourceErrorHandler() *ResourceErrorHandler {
	return &ResourceErrorHandler{
		EnableGracefulDegradation: true,
		EnableErrorRecovery:       true,
		EnableMetrics:            true,
	}
}

// HandleError provides centralized error handling for resource-related errors
func (h *ResourceErrorHandler) HandleError(err error) error {
	switch e := err.(type) {
	case *ResourceLimitError:
		if h.OnResourceLimitExceeded != nil {
			h.OnResourceLimitExceeded(e)
		}
		return h.handleResourceLimitError(e)
		
	case *RateLimitError:
		if h.OnRateLimitExceeded != nil {
			h.OnRateLimitExceeded(e)
		}
		return h.handleRateLimitError(e)
		
	case *StructureLimitError:
		if h.OnStructureLimitExceeded != nil {
			h.OnStructureLimitExceeded(e)
		}
		return h.handleStructureLimitError(e)
		
	case *TimeoutError:
		if h.OnTimeoutExceeded != nil {
			h.OnTimeoutExceeded(e)
		}
		return h.handleTimeoutError(e)
		
	case *SecurityError:
		if h.OnSecurityViolation != nil {
			h.OnSecurityViolation(e)
		}
		return h.handleSecurityError(e)
		
	default:
		// Not a resource-related error, pass through
		return err
	}
}

// handleResourceLimitError handles resource limit exceeded errors
func (h *ResourceErrorHandler) handleResourceLimitError(err *ResourceLimitError) error {
	if h.EnableGracefulDegradation && err.IsRecoverable() {
		// For recoverable errors, we could potentially wait or provide fallback
		// For now, just return the error but mark it as handled
		return fmt.Errorf("resource limit exceeded (handled): %w", err)
	}
	
	// Non-recoverable or degradation disabled, return original error
	return err
}

// handleRateLimitError handles rate limit exceeded errors
func (h *ResourceErrorHandler) handleRateLimitError(err *RateLimitError) error {
	if h.EnableErrorRecovery {
		// Could implement automatic retry with backoff
		retryAfter := err.GetRetryAfter()
		return fmt.Errorf("rate limited (retry after %v): %w", retryAfter, err)
	}
	
	return err
}

// handleStructureLimitError handles structure limit exceeded errors
func (h *ResourceErrorHandler) handleStructureLimitError(err *StructureLimitError) error {
	if h.EnableGracefulDegradation {
		// Structure limits are usually not recoverable, but we could provide
		// guidance on how to fix the configuration
		return fmt.Errorf("configuration structure invalid: %w", err)
	}
	
	return err
}

// handleTimeoutError handles timeout errors
func (h *ResourceErrorHandler) handleTimeoutError(err *TimeoutError) error {
	if h.EnableErrorRecovery && err.IsRecoverable() {
		// Could implement retry logic
		return fmt.Errorf("operation timed out (recoverable): %w", err)
	}
	
	return err
}

// handleSecurityError handles security violation errors
func (h *ResourceErrorHandler) handleSecurityError(err *SecurityError) error {
	// Security errors should never be recovered from or degraded
	// Always return them as-is for proper handling
	return err
}

// IsResourceError checks if an error is a resource-related error
func IsResourceError(err error) bool {
	switch err.(type) {
	case *ResourceLimitError, *RateLimitError, *StructureLimitError, *TimeoutError, *SecurityError:
		return true
	default:
		// Check if it's a wrapped resource error (handled error)
		var resourceErr *ResourceLimitError
		var rateErr *RateLimitError  
		var structErr *StructureLimitError
		var timeoutErr *TimeoutError
		var secErr *SecurityError
		if errors.As(err, &resourceErr) || errors.As(err, &rateErr) || 
		   errors.As(err, &structErr) || errors.As(err, &timeoutErr) || errors.As(err, &secErr) {
			return true
		}
		return false
	}
}

// GetErrorSeverity returns the severity of a resource error
func GetErrorSeverity(err error) string {
	switch e := err.(type) {
	case *ResourceLimitError:
		return e.GetSeverity()
	case *RateLimitError:
		return e.GetSeverity()
	case *StructureLimitError:
		return e.GetSeverity()
	case *TimeoutError:
		return e.GetSeverity()
	case *SecurityError:
		return e.GetSeverity()
	default:
		return "unknown"
	}
}