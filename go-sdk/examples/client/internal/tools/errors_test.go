package tools

import (
	"errors"
	"testing"
	"time"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorClassification(t *testing.T) {
	classifier := NewErrorClassifier()
	
	tests := []struct {
		name        string
		err         error
		toolName    string
		toolCallID  string
		expectedCode ErrorCode
		isRetryable bool
	}{
		{
			name:        "timeout error",
			err:         errors.New("operation timed out"),
			toolName:    "http_get",
			toolCallID:  "call-1",
			expectedCode: ErrorCodeTimeout,
			isRetryable: true,
		},
		{
			name:        "deadline exceeded",
			err:         errors.New("context deadline exceeded"),
			toolName:    "http_post",
			toolCallID:  "call-2",
			expectedCode: ErrorCodeTimeout,
			isRetryable: true,
		},
		{
			name:        "network error",
			err:         errors.New("connection refused"),
			toolName:    "api_call",
			toolCallID:  "call-3",
			expectedCode: ErrorCodeNetwork,
			isRetryable: true,
		},
		{
			name:        "EOF error",
			err:         errors.New("unexpected EOF"),
			toolName:    "stream_read",
			toolCallID:  "call-4",
			expectedCode: ErrorCodeNetwork,
			isRetryable: true,
		},
		{
			name:        "rate limit error",
			err:         errors.New("rate limit exceeded"),
			toolName:    "api_call",
			toolCallID:  "call-5",
			expectedCode: ErrorCodeRateLimit,
			isRetryable: true,
		},
		{
			name:        "not found error",
			err:         errors.New("404 not found"),
			toolName:    "get_resource",
			toolCallID:  "call-6",
			expectedCode: ErrorCodeNotFound,
			isRetryable: false,
		},
		{
			name:        "permission denied",
			err:         errors.New("403 forbidden"),
			toolName:    "admin_action",
			toolCallID:  "call-7",
			expectedCode: ErrorCodePermission,
			isRetryable: false,
		},
		{
			name:        "validation error",
			err:         errors.New("invalid input parameter"),
			toolName:    "validate_data",
			toolCallID:  "call-8",
			expectedCode: ErrorCodeValidation,
			isRetryable: false,
		},
		{
			name:        "internal server error",
			err:         errors.New("500 internal server error"),
			toolName:    "api_call",
			toolCallID:  "call-9",
			expectedCode: ErrorCodeInternal,
			isRetryable: true,
		},
		{
			name:        "unknown error",
			err:         errors.New("something went wrong"),
			toolName:    "unknown_tool",
			toolCallID:  "call-10",
			expectedCode: ErrorCodeUnknown,
			isRetryable: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolErr := classifier.Classify(tt.err, tt.toolName, tt.toolCallID)
			
			assert.Equal(t, tt.toolCallID, toolErr.ToolCallID)
			assert.Equal(t, tt.toolName, toolErr.ToolName)
			assert.Equal(t, tt.expectedCode, toolErr.Code)
			assert.Equal(t, tt.isRetryable, toolErr.IsRetryable)
			assert.Equal(t, tt.err.Error(), toolErr.Message)
			assert.Equal(t, 1, toolErr.AttemptNumber)
		})
	}
}

func TestToolError_ShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		toolErr  *ToolError
		expected bool
	}{
		{
			name: "retryable error with attempts remaining",
			toolErr: &ToolError{
				Code:          ErrorCodeTimeout,
				IsRetryable:   true,
				AttemptNumber: 2,
				MaxAttempts:   5,
			},
			expected: true,
		},
		{
			name: "non-retryable error",
			toolErr: &ToolError{
				Code:          ErrorCodeValidation,
				IsRetryable:   false,
				AttemptNumber: 1,
				MaxAttempts:   5,
			},
			expected: false,
		},
		{
			name: "max attempts reached",
			toolErr: &ToolError{
				Code:          ErrorCodeTimeout,
				IsRetryable:   true,
				AttemptNumber: 5,
				MaxAttempts:   5,
			},
			expected: false,
		},
		{
			name: "retryable network error",
			toolErr: &ToolError{
				Code:          ErrorCodeNetwork,
				IsRetryable:   true,
				AttemptNumber: 1,
				MaxAttempts:   3,
			},
			expected: true,
		},
		{
			name: "non-retryable permission error",
			toolErr: &ToolError{
				Code:          ErrorCodePermission,
				IsRetryable:   true, // Even if marked retryable, permission errors should not retry
				AttemptNumber: 1,
				MaxAttempts:   3,
			},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.toolErr.ShouldRetry()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToolError_ToJSON(t *testing.T) {
	toolErr := &ToolError{
		ToolCallID:    "call-123",
		ToolName:      "test_tool",
		Code:          ErrorCodeTimeout,
		Message:       "operation timed out",
		Details:       "connection timeout after 30s",
		AttemptNumber: 2,
		MaxAttempts:   5,
		IsRetryable:   true,
		RetryAfter:    durationPtr(5 * time.Second),
		Timestamp:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Duration:      30 * time.Second,
	}
	
	// Test regular JSON
	jsonData, err := toolErr.ToJSON(false)
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), `"toolCallId":"call-123"`)
	assert.Contains(t, string(jsonData), `"errorCode":"TIMEOUT"`)
	assert.Contains(t, string(jsonData), `"attemptNumber":2`)
	
	// Test pretty JSON
	prettyJSON, err := toolErr.ToJSON(true)
	require.NoError(t, err)
	assert.Contains(t, string(prettyJSON), `"toolCallId": "call-123"`)
	assert.Contains(t, string(prettyJSON), "\n")
}

func TestErrorClassifier_CustomRules(t *testing.T) {
	classifier := NewErrorClassifier()
	
	// Add custom rule for specific error message
	classifier.AddRule(ClassificationRule{
		Match: func(err error) bool {
			return err.Error() == "custom database error"
		},
		Code:       ErrorCodeDependency,
		Retryable:  true,
		RetryAfter: durationPtr(10 * time.Second),
	})
	
	// Test custom rule
	err := errors.New("custom database error")
	toolErr := classifier.Classify(err, "db_query", "call-custom")
	
	assert.Equal(t, ErrorCodeDependency, toolErr.Code)
	assert.True(t, toolErr.IsRetryable)
	assert.NotNil(t, toolErr.RetryAfter)
	assert.Equal(t, 10*time.Second, *toolErr.RetryAfter)
}

func TestToolError_ErrorString(t *testing.T) {
	tests := []struct {
		name     string
		toolErr  *ToolError
		expected string
	}{
		{
			name: "first attempt",
			toolErr: &ToolError{
				ToolName:      "http_get",
				Code:          ErrorCodeTimeout,
				Message:       "request timed out",
				AttemptNumber: 1,
				MaxAttempts:   3,
			},
			expected: "tool error: http_get - TIMEOUT: request timed out",
		},
		{
			name: "retry attempt",
			toolErr: &ToolError{
				ToolName:      "api_call",
				Code:          ErrorCodeNetwork,
				Message:       "connection refused",
				AttemptNumber: 2,
				MaxAttempts:   5,
			},
			expected: "tool error (attempt 2/5): api_call - NETWORK_ERROR: connection refused",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.toolErr.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		substrs  []string
		expected bool
	}{
		{
			name:     "contains timeout",
			input:    "operation timed out",
			substrs:  []string{"timeout", "timed out"},
			expected: true,
		},
		{
			name:     "contains none",
			input:    "success",
			substrs:  []string{"error", "fail"},
			expected: false,
		},
		{
			name:     "empty substrs",
			input:    "test",
			substrs:  []string{},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.input, tt.substrs...)
			assert.Equal(t, tt.expected, result)
		})
	}
}