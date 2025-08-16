package errors

import (
	"fmt"
)

// ErrorCategory represents the category of an error
type ErrorCategory string

const (
	// CategoryNetwork represents network-related errors
	CategoryNetwork ErrorCategory = "network"
	
	// CategoryValidation represents validation errors
	CategoryValidation ErrorCategory = "validation"
	
	// CategoryServer represents server-side errors
	CategoryServer ErrorCategory = "server"
	
	// CategoryTool represents tool execution errors
	CategoryTool ErrorCategory = "tool"
	
	// CategoryPermission represents permission/authorization errors
	CategoryPermission ErrorCategory = "permission"
	
	// CategoryTimeout represents timeout errors
	CategoryTimeout ErrorCategory = "timeout"
	
	// CategoryUnknown represents uncategorized errors
	CategoryUnknown ErrorCategory = "unknown"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	// SeverityCritical means the operation cannot continue
	SeverityCritical ErrorSeverity = "critical"
	
	// SeverityError means the operation failed but may be retried
	SeverityError ErrorSeverity = "error"
	
	// SeverityWarning means the operation succeeded with issues
	SeverityWarning ErrorSeverity = "warning"
	
	// SeverityInfo means informational message only
	SeverityInfo ErrorSeverity = "info"
)

// ToolError represents a structured error for tool operations
type ToolError struct {
	// Core error information
	Category    ErrorCategory  `json:"category"`
	Severity    ErrorSeverity  `json:"severity"`
	Code        string         `json:"code,omitempty"`
	Message     string         `json:"message"`
	
	// Context information
	ToolName    string         `json:"tool_name,omitempty"`
	Operation   string         `json:"operation,omitempty"`
	Details     string         `json:"details,omitempty"`
	
	// Recovery information
	Retryable   bool           `json:"retryable"`
	Suggestion  string         `json:"suggestion,omitempty"`
	
	// Original error for debugging
	Original    error          `json:"-"`
}

// Error implements the error interface
func (e *ToolError) Error() string {
	if e.ToolName != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Category, e.ToolName, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Category, e.Message)
}

// IsRetryable returns whether the error can be retried
func (e *ToolError) IsRetryable() bool {
	return e.Retryable
}

// WithSuggestion adds a suggestion to the error
func (e *ToolError) WithSuggestion(suggestion string) *ToolError {
	e.Suggestion = suggestion
	return e
}

// WithDetails adds details to the error
func (e *ToolError) WithDetails(details string) *ToolError {
	e.Details = details
	return e
}

// NewToolError creates a new tool error
func NewToolError(category ErrorCategory, severity ErrorSeverity, message string) *ToolError {
	return &ToolError{
		Category:  category,
		Severity:  severity,
		Message:   message,
		Retryable: isRetryableCategory(category),
	}
}

// NewNetworkError creates a network-related error
func NewNetworkError(message string, retryable bool) *ToolError {
	return &ToolError{
		Category:  CategoryNetwork,
		Severity:  SeverityError,
		Message:   message,
		Retryable: retryable,
		Suggestion: "Check your network connection and server URL",
	}
}

// NewValidationError creates a validation error
func NewValidationError(toolName, message string) *ToolError {
	return &ToolError{
		Category:  CategoryValidation,
		Severity:  SeverityError,
		ToolName:  toolName,
		Message:   message,
		Retryable: false,
		Suggestion: "Check the tool arguments against the schema",
	}
}

// NewServerError creates a server-side error
func NewServerError(code int, message string) *ToolError {
	retryable := code >= 500 && code != 501 // 5xx errors except Not Implemented
	severity := SeverityError
	if code >= 500 {
		severity = SeverityCritical
	}
	
	return &ToolError{
		Category:  CategoryServer,
		Severity:  severity,
		Code:      fmt.Sprintf("HTTP_%d", code),
		Message:   message,
		Retryable: retryable,
		Suggestion: getServerErrorSuggestion(code),
	}
}

// NewToolExecutionError creates a tool execution error
func NewToolExecutionError(toolName, message string) *ToolError {
	return &ToolError{
		Category:  CategoryTool,
		Severity:  SeverityError,
		ToolName:  toolName,
		Message:   message,
		Retryable: true,
		Suggestion: "The tool failed to execute. Check the tool logs for details",
	}
}

// NewPermissionError creates a permission error
func NewPermissionError(message string) *ToolError {
	return &ToolError{
		Category:  CategoryPermission,
		Severity:  SeverityCritical,
		Message:   message,
		Retryable: false,
		Suggestion: "Check your authentication credentials and permissions",
	}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(operation string, duration string) *ToolError {
	return &ToolError{
		Category:  CategoryTimeout,
		Severity:  SeverityError,
		Operation: operation,
		Message:   fmt.Sprintf("Operation timed out after %s", duration),
		Retryable: true,
		Suggestion: "The operation took too long. Try again or increase the timeout",
	}
}

// isRetryableCategory determines if errors in a category are generally retryable
func isRetryableCategory(category ErrorCategory) bool {
	switch category {
	case CategoryNetwork, CategoryServer, CategoryTimeout, CategoryTool:
		return true
	case CategoryValidation, CategoryPermission:
		return false
	default:
		return false
	}
}

// getServerErrorSuggestion provides suggestions for HTTP error codes
func getServerErrorSuggestion(code int) string {
	switch code {
	case 400:
		return "The request was invalid. Check your input parameters"
	case 401:
		return "Authentication required. Check your API credentials"
	case 403:
		return "Permission denied. You don't have access to this resource"
	case 404:
		return "Resource not found. Check the endpoint URL"
	case 429:
		return "Rate limit exceeded. Wait a moment before retrying"
	case 500:
		return "Server error. The server encountered an internal error"
	case 502:
		return "Bad gateway. The server received an invalid response from upstream"
	case 503:
		return "Service unavailable. The server is temporarily overloaded or down"
	case 504:
		return "Gateway timeout. The server didn't respond in time"
	default:
		if code >= 400 && code < 500 {
			return "Client error. Check your request parameters"
		} else if code >= 500 {
			return "Server error. Try again later or contact support"
		}
		return "Unexpected error. Check the server logs for details"
	}
}

// GetErrorEmoji returns an appropriate emoji for the error category
func GetErrorEmoji(category ErrorCategory) string {
	switch category {
	case CategoryNetwork:
		return "🌐"
	case CategoryValidation:
		return "⚠️"
	case CategoryServer:
		return "🔥"
	case CategoryTool:
		return "🔧"
	case CategoryPermission:
		return "🔒"
	case CategoryTimeout:
		return "⏱️"
	default:
		return "❌"
	}
}

// GetSeverityColor returns ANSI color code for severity
func GetSeverityColor(severity ErrorSeverity) string {
	switch severity {
	case SeverityCritical:
		return "\033[91m" // Bright Red
	case SeverityError:
		return "\033[31m" // Red
	case SeverityWarning:
		return "\033[33m" // Yellow
	case SeverityInfo:
		return "\033[36m" // Cyan
	default:
		return "\033[0m"  // Reset
	}
}