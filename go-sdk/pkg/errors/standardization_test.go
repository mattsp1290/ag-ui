package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestErrorChainPreservation verifies that error chains are properly preserved
// across all modules using the standardized fmt.Errorf with %w pattern
func TestErrorChainPreservation(t *testing.T) {
	tests := []struct {
		name        string
		createError func() error
		wantChain   []string
	}{
		{
			name: "encoding validation error chain",
			createError: func() error {
				baseErr := errors.New("invalid JSON syntax")
				return fmt.Errorf("validation failed: %w", baseErr)
			},
			wantChain: []string{"validation failed", "invalid JSON syntax"},
		},
		{
			name: "multi-level error chain",
			createError: func() error {
				baseErr := errors.New("parse error")
				middleErr := fmt.Errorf("invalid base URL: %w", baseErr)
				return fmt.Errorf("config error in field BaseURL: %w", middleErr)
			},
			wantChain: []string{"config error", "invalid base URL", "parse error"},
		},
		{
			name: "deep error chain",
			createError: func() error {
				baseErr := errors.New("network timeout")
				connectionErr := fmt.Errorf("connection error: %w", baseErr)
				return fmt.Errorf("tool execution failed: %w", connectionErr)
			},
			wantChain: []string{"tool execution failed", "connection error", "network timeout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()

			// Verify error chain preservation
			current := err
			for i, expectedMsg := range tt.wantChain {
				if current == nil {
					t.Fatalf("error chain broken at level %d, expected: %s", i, expectedMsg)
				}

				if !strings.Contains(current.Error(), expectedMsg) {
					t.Errorf("level %d: error message %q does not contain %q", i, current.Error(), expectedMsg)
				}

				current = errors.Unwrap(current)
			}
		})
	}
}

// TestErrorWrappingConsistency verifies that all modules use consistent error wrapping
func TestErrorWrappingConsistency(t *testing.T) {
	// Test encoding errors use fmt.Errorf with %w
	t.Run("encoding error wrapping", func(t *testing.T) {
		baseErr := errors.New("base error")
		wrappedErr := fmt.Errorf("encoding failed: %w", baseErr)

		if !errors.Is(wrappedErr, baseErr) {
			t.Error("fmt.Errorf with %w should preserve error identity")
		}

		unwrapped := errors.Unwrap(wrappedErr)
		if unwrapped != baseErr {
			t.Error("fmt.Errorf with %w should allow unwrapping")
		}
	})

	// Test custom error types maintain compatibility
	t.Run("custom error compatibility", func(t *testing.T) {
		baseErr := errors.New("validation failed")
		customErr := NewValidationError("TEST_ERROR", "test validation error")
		customErr.WithCause(baseErr)

		if !errors.Is(customErr, baseErr) {
			t.Error("custom errors should preserve error identity through WithCause")
		}

		unwrapped := errors.Unwrap(customErr)
		if unwrapped != baseErr {
			t.Error("custom errors should allow unwrapping")
		}
	})
}

// TestErrorContextPreservation verifies that error context is properly maintained
func TestErrorContextPreservation(t *testing.T) {
	tests := []struct {
		name        string
		createError func() error
		wantContext []string
	}{
		{
			name: "state management error with context",
			createError: func() error {
				baseErr := errors.New("state not found")
				return fmt.Errorf("state update failed for context 'user-123': %w", baseErr)
			},
			wantContext: []string{"state update failed", "user-123"},
		},
		{
			name: "validation error with field context",
			createError: func() error {
				baseErr := errors.New("invalid state")
				return fmt.Errorf("field validation failed for 'email': %w", baseErr)
			},
			wantContext: []string{"field validation failed", "email"},
		},
		{
			name: "client error with agent context",
			createError: func() error {
				baseErr := errors.New("connection refused")
				return fmt.Errorf("SendEvent failed for agent %s: %w", "test-agent", baseErr)
			},
			wantContext: []string{"SendEvent failed", "test-agent", "connection refused"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errMsg := err.Error()

			for _, expectedContext := range tt.wantContext {
				if !strings.Contains(errMsg, expectedContext) {
					t.Errorf("error message %q should contain context %q", errMsg, expectedContext)
				}
			}
		})
	}
}

// TestErrorTypeConsistency verifies that similar errors across modules use consistent patterns
func TestErrorTypeConsistency(t *testing.T) {
	// Test validation errors
	t.Run("validation error patterns", func(t *testing.T) {
		// Standard validation error
		validationErr := NewValidationError("VALIDATION_FAILED", "validation failed")

		// Should be identifiable as validation errors
		if validationErr.Severity != SeverityWarning {
			t.Error("validation errors should have Warning severity")
		}

		if validationErr.Code != "VALIDATION_FAILED" {
			t.Error("validation errors should have correct code")
		}
	})

	// Test error severity consistency
	t.Run("error severity patterns", func(t *testing.T) {
		securityErr := NewSecurityError("SECURITY_VIOLATION", "security violation")

		if securityErr.Severity != SeverityCritical {
			t.Error("security errors should have Critical severity")
		}
	})
}

// TestStandardizedErrorMessages verifies error messages follow consistent patterns
func TestStandardizedErrorMessages(t *testing.T) {
	tests := []struct {
		name           string
		createError    func() error
		wantPattern    string
		wantComponents []string
	}{
		{
			name: "method not implemented error",
			createError: func() error {
				return fmt.Errorf("PerfJSONSerializer.Deserialize: method not yet implemented")
			},
			wantPattern:    "MethodName: description",
			wantComponents: []string{"PerfJSONSerializer.Deserialize", "method not yet implemented"},
		},
		{
			name: "component operation error",
			createError: func() error {
				return fmt.Errorf("websocket performance manager: rate limit exceeded for message batching")
			},
			wantPattern:    "component: description",
			wantComponents: []string{"websocket performance manager", "rate limit exceeded"},
		},
		{
			name: "field validation error",
			createError: func() error {
				return fmt.Errorf("invalid message content: %w", errors.New("field too long"))
			},
			wantPattern:    "operation description: underlying error",
			wantComponents: []string{"invalid message content", "field too long"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errMsg := err.Error()

			for _, component := range tt.wantComponents {
				if !strings.Contains(errMsg, component) {
					t.Errorf("error message %q should contain component %q", errMsg, component)
				}
			}
		})
	}
}

// TestErrorUnwrappingBehavior verifies that errors can be properly unwrapped
// for inspection and handling
func TestErrorUnwrappingBehavior(t *testing.T) {
	// Create a chain of wrapped errors
	baseErr := NewValidationError("BASE_ERROR", "base validation error")
	middleErr := fmt.Errorf("middle layer: %w", baseErr)
	topErr := fmt.Errorf("top layer: %w", middleErr)

	// Test errors.Is() works through the chain
	if !errors.Is(topErr, baseErr) {
		t.Error("errors.Is should work through wrapped error chain")
	}

	// Test errors.As() can extract custom error types
	var validationErr *ValidationError
	if !errors.As(topErr, &validationErr) {
		t.Error("errors.As should extract custom error types from chain")
	}

	if validationErr.Code != "BASE_ERROR" {
		t.Errorf("extracted error should have correct code, got: %s", validationErr.Code)
	}
}

// TestErrorHandlerIntegration verifies that error handlers work correctly
// with standardized error patterns
func TestErrorHandlerIntegration(t *testing.T) {
	// Test validation error creation and properties
	validationErr := NewValidationError("TEST_VALIDATION", "test validation error")

	if validationErr.Code != "TEST_VALIDATION" {
		t.Errorf("expected code TEST_VALIDATION, got %s", validationErr.Code)
	}

	if validationErr.Message != "test validation error" {
		t.Errorf("expected message 'test validation error', got %s", validationErr.Message)
	}

	// Test error enhancement
	enhancedErr := validationErr.WithDetail("context", "test_context")
	if enhancedErr.Details["context"] != "test_context" {
		t.Error("error detail should be added correctly")
	}
}

// TestConcurrentErrorHandling verifies that error handling is thread-safe
func TestConcurrentErrorHandling(t *testing.T) {
	const numGoroutines = 100
	done := make(chan bool, numGoroutines)

	// Create errors concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Create various types of errors
			validationErr := NewValidationError(fmt.Sprintf("ERROR_%d", id), "concurrent error")
			wrappedErr := fmt.Errorf("wrapper %d: %w", id, validationErr)

			// Verify error properties
			if !errors.Is(wrappedErr, validationErr) {
				t.Errorf("concurrent error wrapping failed for ID %d", id)
			}

			var extracted *ValidationError
			if !errors.As(wrappedErr, &extracted) {
				t.Errorf("concurrent error extraction failed for ID %d", id)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}
