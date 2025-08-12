package tools

import (
	"encoding/json"
	"fmt"
	"time"
)

// ErrorCode represents standard error codes for tool failures
type ErrorCode string

const (
	ErrorCodeTimeout        ErrorCode = "TIMEOUT"
	ErrorCodeValidation     ErrorCode = "VALIDATION_ERROR"
	ErrorCodeNotFound       ErrorCode = "NOT_FOUND"
	ErrorCodePermission     ErrorCode = "PERMISSION_DENIED"
	ErrorCodeRateLimit      ErrorCode = "RATE_LIMIT"
	ErrorCodeNetwork        ErrorCode = "NETWORK_ERROR"
	ErrorCodeInternal       ErrorCode = "INTERNAL_ERROR"
	ErrorCodeDependency     ErrorCode = "DEPENDENCY_ERROR"
	ErrorCodeInvalidInput   ErrorCode = "INVALID_INPUT"
	ErrorCodeResourceLimit  ErrorCode = "RESOURCE_LIMIT"
	ErrorCodeUnknown        ErrorCode = "UNKNOWN_ERROR"
)

// ToolError represents a structured error from a tool invocation
type ToolError struct {
	// Core identifiers
	ToolCallID  string    `json:"toolCallId"`
	ToolName    string    `json:"toolName"`
	RequestID   string    `json:"requestId,omitempty"`
	SessionID   string    `json:"sessionId,omitempty"`
	
	// Error details
	Code        ErrorCode `json:"errorCode"`
	Message     string    `json:"errorMessage"`
	Details     string    `json:"details,omitempty"`
	
	// Retry information
	AttemptNumber int           `json:"attemptNumber"`
	MaxAttempts   int           `json:"maxAttempts"`
	IsRetryable   bool          `json:"isRetryable"`
	RetryAfter    *time.Duration `json:"retryAfter,omitempty"`
	
	// Timing
	Timestamp   time.Time     `json:"timestamp"`
	Duration    time.Duration `json:"duration,omitempty"`
	
	// Additional context
	StackTrace  string                 `json:"stackTrace,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Error implements the error interface
func (e *ToolError) Error() string {
	if e.AttemptNumber > 1 {
		return fmt.Sprintf("tool error (attempt %d/%d): %s - %s: %s", 
			e.AttemptNumber, e.MaxAttempts, e.ToolName, e.Code, e.Message)
	}
	return fmt.Sprintf("tool error: %s - %s: %s", e.ToolName, e.Code, e.Message)
}

// ShouldRetry determines if the error warrants a retry
func (e *ToolError) ShouldRetry() bool {
	if !e.IsRetryable {
		return false
	}
	
	if e.MaxAttempts > 0 && e.AttemptNumber >= e.MaxAttempts {
		return false
	}
	
	// Check error codes that are typically retryable
	switch e.Code {
	case ErrorCodeTimeout, ErrorCodeNetwork, ErrorCodeRateLimit, ErrorCodeDependency:
		return true
	case ErrorCodeValidation, ErrorCodeNotFound, ErrorCodePermission, ErrorCodeInvalidInput:
		return false
	default:
		return e.IsRetryable
	}
}

// ToJSON converts the error to JSON format
func (e *ToolError) ToJSON(pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(e, "", "  ")
	}
	return json.Marshal(e)
}

// ErrorClassifier classifies errors into standard categories
type ErrorClassifier struct {
	// Custom classification rules can be added here
	customRules []ClassificationRule
}

// ClassificationRule defines a rule for classifying errors
type ClassificationRule struct {
	Match      func(error) bool
	Code       ErrorCode
	Retryable  bool
	RetryAfter *time.Duration
}

// NewErrorClassifier creates a new error classifier with default rules
func NewErrorClassifier() *ErrorClassifier {
	return &ErrorClassifier{
		customRules: make([]ClassificationRule, 0),
	}
}

// AddRule adds a custom classification rule
func (c *ErrorClassifier) AddRule(rule ClassificationRule) {
	c.customRules = append(c.customRules, rule)
}

// Classify classifies an error into a ToolError
func (c *ErrorClassifier) Classify(err error, toolName string, toolCallID string) *ToolError {
	toolErr := &ToolError{
		ToolCallID:    toolCallID,
		ToolName:      toolName,
		Message:       err.Error(),
		Code:          ErrorCodeUnknown,
		IsRetryable:   false,
		AttemptNumber: 1,
		Timestamp:     time.Now(),
	}
	
	// Check if it's already a ToolError
	if te, ok := err.(*ToolError); ok {
		return te
	}
	
	// Apply custom rules first
	for _, rule := range c.customRules {
		if rule.Match(err) {
			toolErr.Code = rule.Code
			toolErr.IsRetryable = rule.Retryable
			toolErr.RetryAfter = rule.RetryAfter
			return toolErr
		}
	}
	
	// Default classification based on error message patterns
	errMsg := err.Error()
	switch {
	case containsAny(errMsg, "timeout", "timed out", "deadline exceeded"):
		toolErr.Code = ErrorCodeTimeout
		toolErr.IsRetryable = true
	case containsAny(errMsg, "connection", "network", "dial", "EOF"):
		toolErr.Code = ErrorCodeNetwork
		toolErr.IsRetryable = true
	case containsAny(errMsg, "rate limit", "too many requests", "throttled"):
		toolErr.Code = ErrorCodeRateLimit
		toolErr.IsRetryable = true
		toolErr.RetryAfter = durationPtr(time.Second * 30)
	case containsAny(errMsg, "not found", "404"):
		toolErr.Code = ErrorCodeNotFound
		toolErr.IsRetryable = false
	case containsAny(errMsg, "unauthorized", "forbidden", "403", "401"):
		toolErr.Code = ErrorCodePermission
		toolErr.IsRetryable = false
	case containsAny(errMsg, "invalid", "validation", "malformed"):
		toolErr.Code = ErrorCodeValidation
		toolErr.IsRetryable = false
	case containsAny(errMsg, "internal server", "500", "502", "503"):
		toolErr.Code = ErrorCodeInternal
		toolErr.IsRetryable = true
	default:
		toolErr.Code = ErrorCodeUnknown
		toolErr.IsRetryable = false
	}
	
	return toolErr
}

// Helper functions

func containsAny(s string, substrs ...string) bool {
	lowerS := fmt.Sprintf("%s", s) // Convert to lowercase for case-insensitive matching
	for _, substr := range substrs {
		if contains(lowerS, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}