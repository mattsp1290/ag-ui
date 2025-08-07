package tools

import (
	"errors"
	"testing"
	"time"
)

// TestStructuredErrors verifies that the structured error system is working correctly
func TestStructuredErrors(t *testing.T) {
	t.Run("ValidationError", func(t *testing.T) {
		err := NewValidationError(CodeParameterMissing, "name parameter is required", "test-tool")

		// Check error type
		var toolErr *ToolError
		if !errors.As(err, &toolErr) {
			t.Fatal("Expected ToolError type")
		}

		// Verify fields
		if toolErr.Type != ErrorTypeValidation {
			t.Errorf("Expected error type %s, got %s", ErrorTypeValidation, toolErr.Type)
		}
		if toolErr.Code != CodeParameterMissing {
			t.Errorf("Expected error code %s, got %s", CodeParameterMissing, toolErr.Code)
		}
		if toolErr.ToolID != "test-tool" {
			t.Errorf("Expected tool ID 'test-tool', got %s", toolErr.ToolID)
		}
	})

	t.Run("ExecutionErrorWithRetry", func(t *testing.T) {
		retryAfter := 5 * time.Second
		err := NewExecutionError(CodeExecutionFailed, "temporary failure", "test-tool").
			WithRetry(retryAfter)

		var toolErr *ToolError
		if !errors.As(err, &toolErr) {
			t.Fatal("Expected ToolError type")
		}

		if !toolErr.Retryable {
			t.Error("Expected error to be retryable")
		}
		if toolErr.RetryAfter == nil || *toolErr.RetryAfter != retryAfter {
			t.Error("RetryAfter not set correctly")
		}
	})

	t.Run("IOErrorWithCause", func(t *testing.T) {
		originalErr := errors.New("file not found")
		err := NewIOError(CodeFileOpenFailed, "cannot open config", "/path/to/file", originalErr)

		var toolErr *ToolError
		if !errors.As(err, &toolErr) {
			t.Fatal("Expected ToolError type")
		}

		// Check cause
		if !errors.Is(toolErr, originalErr) {
			t.Error("Expected error to wrap original cause")
		}

		// Check details
		if path, ok := toolErr.Details["path"]; !ok || path != "/path/to/file" {
			t.Error("Path detail not set correctly")
		}
	})

	t.Run("ConflictError", func(t *testing.T) {
		err := NewConflictError(CodeConflictResolutionFailed, "tool already exists", "existing-tool", "new-tool")

		var toolErr *ToolError
		if !errors.As(err, &toolErr) {
			t.Fatal("Expected ToolError type")
		}

		if toolErr.Type != ErrorTypeConflict {
			t.Errorf("Expected error type %s, got %s", ErrorTypeConflict, toolErr.Type)
		}

		// Check details
		if existing, ok := toolErr.Details["existing_tool"]; !ok || existing != "existing-tool" {
			t.Error("Existing tool detail not set correctly")
		}
		if newTool, ok := toolErr.Details["new_tool"]; !ok || newTool != "new-tool" {
			t.Error("New tool detail not set correctly")
		}
	})

	t.Run("ErrorComparison", func(t *testing.T) {
		err1 := NewToolError(ErrorTypeValidation, CodeToolNotFound, "tool not found").
			WithToolID("test-tool")
		err2 := &ToolError{
			Type: ErrorTypeValidation,
			Code: CodeToolNotFound,
		}

		// Should match based on type and code
		if !errors.Is(err1, err2) {
			t.Error("Expected errors to match based on type and code")
		}

		// Should match common errors
		if !errors.Is(err1, ErrToolNotFound) {
			t.Error("Expected error to match ErrToolNotFound")
		}
	})
}

// TestErrorHandlerIntegration verifies error handler functionality
func TestErrorHandlerIntegration(t *testing.T) {
	handler := NewErrorHandler()

	// Add transformer
	handler.AddTransformer(func(err *ToolError) *ToolError {
		err.Message = "[TRANSFORMED] " + err.Message
		return err
	})

	// Add listener
	var capturedError *ToolError
	handler.AddListener(func(err *ToolError) {
		capturedError = err
	})

	// Test error handling
	originalErr := errors.New("original error")
	handledErr := handler.HandleError(originalErr, "test-tool")

	// Verify transformation
	if toolErr, ok := handledErr.(*ToolError); ok {
		if toolErr.Message != "[TRANSFORMED] original error" {
			t.Errorf("Expected transformed message, got: %s", toolErr.Message)
		}
	} else {
		t.Fatal("Expected ToolError type")
	}

	// Verify listener was called
	if capturedError == nil {
		t.Error("Listener was not called")
	}
}
