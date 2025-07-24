// Package errors provides error handling helpers to reduce code duplication
package errors

import (
	"fmt"
	"time"
)

// WrapWithContext wraps an error with contextual information using a consistent format
func WrapWithContext(err error, operation, component string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s failed for %s: %w", operation, component, err)
}

// WrapWithContextf wraps an error with formatted contextual information
func WrapWithContextf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// NewOperationError creates a standardized error for failed operations
func NewOperationError(operation, component string, cause error) *BaseError {
	return &BaseError{
		Code:      "OPERATION_FAILED",
		Message:   fmt.Sprintf("%s failed for %s", operation, component),
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Cause:     cause,
		Details:   map[string]interface{}{
			"operation": operation,
			"component": component,
		},
	}
}

// NewValidationErrorWithField creates a validation error with field information
func NewValidationErrorWithField(field, rule, reason string, value interface{}) *ValidationError {
	return &ValidationError{
		BaseError: &BaseError{
			Code:      "VALIDATION_FAILED",
			Message:   fmt.Sprintf("validation failed for field %s: %s", field, reason),
			Severity:  SeverityWarning,
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"field":  field,
				"rule":   rule,
				"reason": reason,
				"value":  value,
			},
		},
		Field:       field,
		Value:       value,
		Rule:        rule,
		FieldErrors: make(map[string][]string),
	}
}

// NewConfigurationErrorWithField creates a configuration error with field information
func NewConfigurationErrorWithField(field, reason string, value interface{}) *BaseError {
	return &BaseError{
		Code:      "CONFIGURATION_ERROR",
		Message:   fmt.Sprintf("configuration error in field %s: %s", field, reason),
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"field":  field,
			"reason": reason,
			"value":  value,
		},
	}
}

// NewConnectionErrorWithEndpoint creates a connection error with endpoint information
func NewConnectionErrorWithEndpoint(endpoint string, cause error) *BaseError {
	return &BaseError{
		Code:      "CONNECTION_FAILED",
		Message:   fmt.Sprintf("failed to connect to %s", endpoint),
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Cause:     cause,
		Details: map[string]interface{}{
			"endpoint": endpoint,
		},
	}
}

// NewTimeoutErrorWithOperation creates a timeout error with operation information
func NewTimeoutErrorWithOperation(operation string, timeout, elapsed time.Duration) *BaseError {
	return &BaseError{
		Code:      "TIMEOUT",
		Message:   fmt.Sprintf("%s timed out after %v (elapsed: %v)", operation, timeout, elapsed),
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"operation": operation,
			"timeout":   timeout,
			"elapsed":   elapsed,
		},
	}
}

// NewInternalErrorWithComponent creates an internal error with component information
func NewInternalErrorWithComponent(component, reason string, cause error) *BaseError {
	return &BaseError{
		Code:      "INTERNAL_ERROR",
		Message:   fmt.Sprintf("internal error in %s: %s", component, reason),
		Severity:  SeverityCritical,
		Timestamp: time.Now(),
		Cause:     cause,
		Details: map[string]interface{}{
			"component": component,
			"reason":    reason,
		},
	}
}

// NewNotImplementedError creates a standardized not implemented error
func NewNotImplementedError(operation, component string) *BaseError {
	return &BaseError{
		Code:      "NOT_IMPLEMENTED",
		Message:   fmt.Sprintf("%s is not implemented for %s", operation, component),
		Severity:  SeverityWarning,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"operation": operation,
			"component": component,
		},
	}
}

// NewResourceNotFoundError creates a resource not found error
func NewResourceNotFoundError(resourceType, resourceID string) *BaseError {
	return &BaseError{
		Code:      "RESOURCE_NOT_FOUND",
		Message:   fmt.Sprintf("%s with ID %s not found", resourceType, resourceID),
		Severity:  SeverityWarning,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"resource_type": resourceType,
			"resource_id":   resourceID,
		},
	}
}

// NewResourceConflictError creates a resource conflict error
func NewResourceConflictError(resourceType, resourceID, reason string) *ConflictError {
	return &ConflictError{
		BaseError: &BaseError{
			Code:      "RESOURCE_CONFLICT",
			Message:   fmt.Sprintf("conflict with %s %s: %s", resourceType, resourceID, reason),
			Severity:  SeverityError,
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"resource_type": resourceType,
				"resource_id":   resourceID,
				"reason":        reason,
			},
		},
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
}

// NewUnsupportedOperationError creates an unsupported operation error
func NewUnsupportedOperationError(operation, component, reason string) *BaseError {
	return &BaseError{
		Code:      "UNSUPPORTED_OPERATION",
		Message:   fmt.Sprintf("operation %s is not supported by %s: %s", operation, component, reason),
		Severity:  SeverityWarning,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"operation": operation,
			"component": component,
			"reason":    reason,
		},
	}
}

// NewSerializationError creates a serialization error
func NewSerializationError(format, operation string, cause error) *BaseError {
	return &BaseError{
		Code:      "SERIALIZATION_ERROR",
		Message:   fmt.Sprintf("%s serialization failed for format %s", operation, format),
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Cause:     cause,
		Details: map[string]interface{}{
			"format":    format,
			"operation": operation,
		},
	}
}

// NewNetworkError creates a network error
func NewNetworkError(operation, network, address string, cause error) *BaseError {
	return &BaseError{
		Code:      "NETWORK_ERROR",
		Message:   fmt.Sprintf("network error during %s to %s://%s", operation, network, address),
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Cause:     cause,
		Details: map[string]interface{}{
			"operation": operation,
			"network":   network,
			"address":   address,
		},
	}
}

// NewAuthenticationError creates an authentication error
func NewAuthenticationError(authType, reason string) *BaseError {
	return &BaseError{
		Code:      "AUTHENTICATION_FAILED",
		Message:   fmt.Sprintf("authentication failed: %s", reason),
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"auth_type": authType,
			"reason":    reason,
		},
	}
}

// NewAuthorizationError creates an authorization error
func NewAuthorizationError(resource, action string, requiredPermissions []string) *BaseError {
	return &BaseError{
		Code:      "AUTHORIZATION_FAILED",
		Message:   fmt.Sprintf("authorization failed for %s on %s", action, resource),
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"resource":             resource,
			"action":               action,
			"required_permissions": requiredPermissions,
		},
	}
}

// ErrorHelpers provides common error handling utilities
type ErrorHelpers struct{}

// NewErrorHelpers creates a new instance of ErrorHelpers
func NewErrorHelpers() *ErrorHelpers {
	return &ErrorHelpers{}
}

// IsNilOrEmpty checks if an error is nil or has empty message
func (h *ErrorHelpers) IsNilOrEmpty(err error) bool {
	return err == nil || err.Error() == ""
}

// ExtractRootCause extracts the root cause from a chain of errors
func (h *ErrorHelpers) ExtractRootCause(err error) error {
	if err == nil {
		return nil
	}
	
	for {
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			if unwrapped := unwrapper.Unwrap(); unwrapped != nil {
				err = unwrapped
				continue
			}
		}
		break
	}
	
	return err
}

// CreateErrorMessage creates a standardized error message format
func (h *ErrorHelpers) CreateErrorMessage(operation, component, reason string) string {
	if reason == "" {
		return fmt.Sprintf("%s failed for %s", operation, component)
	}
	return fmt.Sprintf("%s failed for %s: %s", operation, component, reason)
}

// CreateTimeoutMessage creates a standardized timeout message
func (h *ErrorHelpers) CreateTimeoutMessage(operation string, timeout, elapsed time.Duration) string {
	return fmt.Sprintf("%s timed out after %v (elapsed: %v)", operation, timeout, elapsed)
}

// CreateConnectionMessage creates a standardized connection error message
func (h *ErrorHelpers) CreateConnectionMessage(endpoint string) string {
	return fmt.Sprintf("failed to connect to %s", endpoint)
}

// CreateValidationMessage creates a standardized validation error message
func (h *ErrorHelpers) CreateValidationMessage(field, reason string) string {
	return fmt.Sprintf("validation failed for field %s: %s", field, reason)
}

// CreateConfigurationMessage creates a standardized configuration error message
func (h *ErrorHelpers) CreateConfigurationMessage(field, reason string) string {
	return fmt.Sprintf("configuration error in field %s: %s", field, reason)
}

// CreateNotImplementedMessage creates a standardized not implemented message
func (h *ErrorHelpers) CreateNotImplementedMessage(operation, component string) string {
	return fmt.Sprintf("%s is not implemented for %s", operation, component)
}

// CreateResourceNotFoundMessage creates a standardized resource not found message
func (h *ErrorHelpers) CreateResourceNotFoundMessage(resourceType, resourceID string) string {
	return fmt.Sprintf("%s with ID %s not found", resourceType, resourceID)
}

// CreateUnsupportedOperationMessage creates a standardized unsupported operation message
func (h *ErrorHelpers) CreateUnsupportedOperationMessage(operation, component, reason string) string {
	return fmt.Sprintf("operation %s is not supported by %s: %s", operation, component, reason)
}