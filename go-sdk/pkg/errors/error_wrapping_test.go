package errors

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrorWrappingConsistencyNew(t *testing.T) {
	tests := []struct {
		name           string
		errorFunc      func() error
		wantMessage    string
		wantWrapped    error
		expectWrapping bool
	}{
		{
			name: "fmt.Errorf with %w preserves chain",
			errorFunc: func() error {
				baseErr := errors.New("base error")
				return fmt.Errorf("operation failed: %w", baseErr)
			},
			wantMessage:    "operation failed: base error",
			expectWrapping: true,
		},
		{
			name: "security error with proper context",
			errorFunc: func() error {
				return NewSecurityError(CodeXSSDetected, "XSS pattern detected").
					WithViolationType("cross_site_scripting").
					WithPattern("<script>").
					WithRiskLevel("high")
			},
			wantMessage:    "[CRITICAL] XSS_DETECTED: XSS pattern detected (violation: cross_site_scripting) (pattern: <script>) (risk: high)",
			expectWrapping: false,
		},
		{
			name: "validation error with field context",
			errorFunc: func() error {
				return NewValidationError(CodeValidationFailed, "field validation failed").
					WithField("email", "invalid@").
					WithRule("email_format")
			},
			wantMessage:    "[WARNING] VALIDATION_FAILED: field validation failed (field: email) (rule: email_format)",
			expectWrapping: false,
		},
		{
			name: "encoding error with format and operation",
			errorFunc: func() error {
				baseErr := errors.New("invalid JSON syntax")
				return NewEncodingError(CodeDecodingFailed, "JSON decoding failed").
					WithFormat("json").
					WithOperation("decode").
					WithCause(baseErr)
			},
			wantMessage:    "[ERROR] DECODING_FAILED: JSON decoding failed (format: json) (operation: decode) (caused by: invalid JSON syntax)",
			expectWrapping: true,
		},
		{
			name: "state error with state context",
			errorFunc: func() error {
				return NewStateError(CodeValidationFailed, "state validation failed").
					WithStateID("user-123").
					WithTransition("active->inactive")
			},
			wantMessage:    "[ERROR] VALIDATION_FAILED: state validation failed (state: user-123) (transition: active->inactive)",
			expectWrapping: false,
		},
		{
			name: "conflict error with resource context",
			errorFunc: func() error {
				return NewConflictError(CodeValidationFailed, "resource conflict detected").
					WithResource("user", "123").
					WithOperation("update").
					WithResolution("last_write_wins")
			},
			wantMessage:    "[ERROR] VALIDATION_FAILED: resource conflict detected (resource: user/123) (operation: update)",
			expectWrapping: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.errorFunc()

			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			// Check error message
			if err.Error() != tt.wantMessage {
				t.Errorf("Error message mismatch:\nWant: %s\nGot:  %s", tt.wantMessage, err.Error())
			}

			// Check if error wrapping works as expected
			if tt.expectWrapping && tt.wantWrapped != nil {
				if !errors.Is(err, tt.wantWrapped) {
					t.Errorf("Error wrapping not preserved: error should wrap %v", tt.wantWrapped)
				}
			}
		})
	}
}

func TestErrorConstantsUsage(t *testing.T) {
	tests := []struct {
		name     string
		errorMsg string
		constant string
	}{
		{
			name:     "base URL cannot be empty",
			errorMsg: MsgBaseURLCannotBeEmpty,
			constant: "base URL cannot be empty",
		},
		{
			name:     "agent name cannot be empty",
			errorMsg: MsgAgentNameCannotBeEmpty,
			constant: "agent name cannot be empty",
		},
		{
			name:     "event cannot be nil",
			errorMsg: MsgEventCannotBeNil,
			constant: "event cannot be nil",
		},
		{
			name:     "validation failed",
			errorMsg: MsgValidationFailed,
			constant: "validation failed",
		},
		{
			name:     "encoding failed",
			errorMsg: MsgEncodingFailed,
			constant: "encoding failed",
		},
		{
			name:     "operation timeout",
			errorMsg: MsgOperationTimeout,
			constant: "operation timeout",
		},
		{
			name:     "not implemented",
			errorMsg: MsgNotImplemented,
			constant: "not implemented",
		},
		{
			name:     "rate limit exceeded",
			errorMsg: MsgRateLimitExceeded,
			constant: "rate limit exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.errorMsg != tt.constant {
				t.Errorf("Constant value mismatch for %s:\nWant: %s\nGot:  %s",
					tt.name, tt.constant, tt.errorMsg)
			}
		})
	}
}

func TestFormatHelperFunctions(t *testing.T) {
	tests := []struct {
		name     string
		function func() string
		expected string
	}{
		{
			name: "FormatComponentError",
			function: func() string {
				return FormatComponentError("websocket", "connection", "timeout exceeded")
			},
			expected: "websocket connection: timeout exceeded",
		},
		{
			name: "FormatFieldError",
			function: func() string {
				return FormatFieldError("email", "is required")
			},
			expected: "field 'email' is required",
		},
		{
			name: "FormatOperationError",
			function: func() string {
				return FormatOperationError("encoding", "invalid JSON format")
			},
			expected: "encoding failed: invalid JSON format",
		},
		{
			name: "FormatResourceError with ID",
			function: func() string {
				return FormatResourceError("user", "123", "not found")
			},
			expected: "user '123' not found",
		},
		{
			name: "FormatResourceError without ID",
			function: func() string {
				return FormatResourceError("session", "", "expired")
			},
			expected: "session expired",
		},
		{
			name: "FormatSecurityError with location",
			function: func() string {
				return FormatSecurityError("xss", "/api/user", "malicious script detected")
			},
			expected: "security violation (xss) at /api/user: malicious script detected",
		},
		{
			name: "FormatSecurityError without location",
			function: func() string {
				return FormatSecurityError("injection", "", "SQL injection attempt")
			},
			expected: "security violation (injection): SQL injection attempt",
		},
		{
			name: "FormatNotImplementedError with component",
			function: func() string {
				return FormatNotImplementedError("JSONEncoder", "Compress")
			},
			expected: "JSONEncoder.Compress: method not implemented",
		},
		{
			name: "FormatNotImplementedError without component",
			function: func() string {
				return FormatNotImplementedError("", "StreamData")
			},
			expected: "StreamData: method not implemented",
		},
		{
			name: "FormatTimeoutError",
			function: func() string {
				return FormatTimeoutError("connection", "30s")
			},
			expected: "connection operation timeout after 30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function()
			if result != tt.expected {
				t.Errorf("Format function output mismatch:\nWant: %s\nGot:  %s", tt.expected, result)
			}
		})
	}
}

func TestErrorChainPreservationNew(t *testing.T) {
	tests := []struct {
		name        string
		createError func() error
		checkChain  []string
	}{
		{
			name: "three level error chain",
			createError: func() error {
				baseErr := errors.New("database connection failed")
				middleErr := fmt.Errorf("user lookup failed: %w", baseErr)
				return fmt.Errorf("authentication failed: %w", middleErr)
			},
			checkChain: []string{
				"authentication failed",
				"user lookup failed",
				"database connection failed",
			},
		},
		{
			name: "custom error with cause chain",
			createError: func() error {
				baseErr := errors.New("network unreachable")
				customErr := NewEncodingError(CodeDecodingFailed, "remote data fetch failed").WithCause(baseErr)
				return fmt.Errorf("operation failed: %w", customErr)
			},
			checkChain: []string{
				"operation failed",
				"remote data fetch failed",
				"network unreachable",
			},
		},
		{
			name: "validation error with field context",
			createError: func() error {
				baseErr := errors.New("invalid email format")
				validationErr := NewValidationError(CodeValidationFailed, "field validation failed").
					WithField("email", "user@invalid").
					WithCause(baseErr)
				return fmt.Errorf("user registration failed: %w", validationErr)
			},
			checkChain: []string{
				"user registration failed",
				"field validation failed",
				"invalid email format",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()

			// Check that each level in the chain is preserved
			for _, expectedMsg := range tt.checkChain {
				if !strings.Contains(err.Error(), expectedMsg) {
					t.Errorf("Error chain missing expected message: %s\nFull error: %s",
						expectedMsg, err.Error())
				}
			}

			// Test error unwrapping
			currentErr := err
			for i := 0; i < len(tt.checkChain)-1; i++ {
				unwrapped := errors.Unwrap(currentErr)
				if unwrapped == nil {
					t.Errorf("Error chain broken at level %d", i+1)
					break
				}
				currentErr = unwrapped
			}
		})
	}
}

func TestContextCancellationErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() (context.Context, context.CancelFunc)
		operation func(ctx context.Context) error
		wantError bool
		checkMsg  string
	}{
		{
			name: "context cancellation is properly wrapped",
			setupFunc: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			operation: func(ctx context.Context) error {
				// Simulate an operation that checks context
				select {
				case <-ctx.Done():
					return fmt.Errorf("operation cancelled: %w", ctx.Err())
				default:
					return nil
				}
			},
			wantError: true,
			checkMsg:  "operation cancelled",
		},
		{
			name: "timeout context is properly handled",
			setupFunc: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background()) // Will manually cancel for test
			},
			operation: func(ctx context.Context) error {
				if ctx.Err() != nil {
					return NewEncodingError(CodeEncodingFailed, "encoding operation cancelled").
						WithOperation("encode").
						WithCause(ctx.Err())
				}
				return nil
			},
			wantError: true,
			checkMsg:  "encoding operation cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.setupFunc()
			cancel() // Cancel immediately for testing

			err := tt.operation(ctx)

			if tt.wantError && err == nil {
				t.Fatal("Expected error, got nil")
			}

			if !tt.wantError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.wantError {
				if !strings.Contains(err.Error(), tt.checkMsg) {
					t.Errorf("Error message should contain %q, got: %s", tt.checkMsg, err.Error())
				}

				// Check that context error is preserved in the chain
				if !errors.Is(err, context.Canceled) {
					t.Error("Context cancellation error should be preserved in error chain")
				}
			}
		})
	}
}

func TestSecurityErrorClassification(t *testing.T) {
	tests := []struct {
		name            string
		error           error
		isSecurityError bool
		violationType   string
		riskLevel       string
	}{
		{
			name:            "XSS security error",
			error:           NewXSSError("script injection detected", "<script>alert('xss')</script>"),
			isSecurityError: true,
			violationType:   "cross_site_scripting",
			riskLevel:       "high",
		},
		{
			name:            "SQL injection security error",
			error:           NewSQLInjectionError("SQL injection detected", "' OR '1'='1"),
			isSecurityError: true,
			violationType:   "sql_injection",
			riskLevel:       "critical",
		},
		{
			name:            "Path traversal security error",
			error:           NewPathTraversalError("path traversal attempt", "../../../etc/passwd"),
			isSecurityError: true,
			violationType:   "path_traversal",
			riskLevel:       "high",
		},
		{
			name:            "Regular validation error",
			error:           NewValidationError(CodeValidationFailed, "invalid input"),
			isSecurityError: false,
			violationType:   "",
			riskLevel:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if error is classified as security error
			if IsSecurityError(tt.error) != tt.isSecurityError {
				t.Errorf("IsSecurityError() = %v, want %v", IsSecurityError(tt.error), tt.isSecurityError)
			}

			// For security errors, check specific attributes
			if tt.isSecurityError {
				var secErr *SecurityError
				if !errors.As(tt.error, &secErr) {
					t.Fatal("Should be able to extract SecurityError")
				}

				if secErr.ViolationType != tt.violationType {
					t.Errorf("ViolationType = %s, want %s", secErr.ViolationType, tt.violationType)
				}

				if secErr.RiskLevel != tt.riskLevel {
					t.Errorf("RiskLevel = %s, want %s", secErr.RiskLevel, tt.riskLevel)
				}
			}
		})
	}
}
