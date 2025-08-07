package errors

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestBaseError(t *testing.T) {
	err := NewBaseError("TEST_ERROR", "Test error message")

	if err.Code != "TEST_ERROR" {
		t.Errorf("Expected code TEST_ERROR, got %s", err.Code)
	}

	if err.Message != "Test error message" {
		t.Errorf("Expected message 'Test error message', got %s", err.Message)
	}

	if err.Severity != SeverityError {
		t.Errorf("Expected severity %s, got %s", SeverityError, err.Severity)
	}

	// Test chaining methods
	err.WithDetail("key", "value").
		WithCause(errors.New("cause")).
		WithRetry(time.Second)

	if err.Details["key"] != "value" {
		t.Error("WithDetail failed")
	}

	if err.Cause == nil {
		t.Error("WithCause failed")
	}

	if !err.Retryable || err.RetryAfter == nil {
		t.Error("WithRetry failed")
	}
}

func TestStateError(t *testing.T) {
	err := NewStateError("STATE_ERROR", "State error").
		WithStateID("state-123").
		WithTransition("active -> inactive").
		WithStates("active", "inactive")

	if err.StateID != "state-123" {
		t.Errorf("Expected state ID 'state-123', got %s", err.StateID)
	}

	if err.Transition != "active -> inactive" {
		t.Errorf("Expected transition 'active -> inactive', got %s", err.Transition)
	}

	errStr := err.Error()
	if !contains(errStr, "state-123") || !contains(errStr, "active -> inactive") {
		t.Errorf("Error string missing expected content: %s", errStr)
	}
}

func TestValidationError(t *testing.T) {
	err := NewValidationError("VALIDATION_ERROR", "Validation failed").
		WithField("email", "invalid@").
		WithRule("email_format").
		AddFieldError("email", "Invalid format").
		AddFieldError("age", "Must be positive")

	if err.Field != "email" {
		t.Errorf("Expected field 'email', got %s", err.Field)
	}

	if err.Rule != "email_format" {
		t.Errorf("Expected rule 'email_format', got %s", err.Rule)
	}

	if !err.HasFieldErrors() {
		t.Error("Expected field errors")
	}

	if len(err.FieldErrors["email"]) != 1 {
		t.Error("Expected email field error")
	}
}

func TestConflictError(t *testing.T) {
	err := NewConflictError("CONFLICT_ERROR", "Resource conflict").
		WithResource("user", "user-123").
		WithOperation("create").
		WithResolution("use update")

	if err.ResourceType != "user" || err.ResourceID != "user-123" {
		t.Error("WithResource failed")
	}

	if err.ConflictingOperation != "create" {
		t.Error("WithOperation failed")
	}

	if err.ResolutionStrategy != "use update" {
		t.Error("WithResolution failed")
	}
}

func TestSeverity(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityDebug, "DEBUG"},
		{SeverityInfo, "INFO"},
		{SeverityWarning, "WARNING"},
		{SeverityError, "ERROR"},
		{SeverityCritical, "CRITICAL"},
		{SeverityFatal, "FATAL"},
	}

	for _, tt := range tests {
		if got := tt.severity.String(); got != tt.expected {
			t.Errorf("Severity.String() = %s, want %s", got, tt.expected)
		}
	}
}

func TestIsRetryable(t *testing.T) {
	retryableErr := NewBaseError("TEMP", "Temporary error").WithRetry(time.Second)
	nonRetryableErr := NewBaseError("PERM", "Permanent error")

	if !IsRetryable(retryableErr) {
		t.Error("Expected retryable error")
	}

	if IsRetryable(nonRetryableErr) {
		t.Error("Expected non-retryable error")
	}

	if IsRetryable(nil) {
		t.Error("nil should not be retryable")
	}
}

func TestGetSeverity(t *testing.T) {
	err := NewBaseError("TEST", "Test")
	err.Severity = SeverityCritical

	if GetSeverity(err) != SeverityCritical {
		t.Error("GetSeverity failed")
	}

	if GetSeverity(errors.New("plain error")) != SeverityError {
		t.Error("GetSeverity should return default for plain errors")
	}

	if GetSeverity(nil) != SeverityInfo {
		t.Error("GetSeverity should return Info for nil")
	}
}

func TestErrorContext(t *testing.T) {
	ec := NewErrorContext("TestOperation").
		WithUserID("user-123").
		WithRequestID("req-456").
		WithTracing("trace-789", "span-012").
		SetTag("env", "test").
		SetMetadata("version", "1.0.0")

	if ec.OperationName != "TestOperation" {
		t.Error("Operation name not set")
	}

	if ec.UserID != "user-123" {
		t.Error("User ID not set")
	}

	// Record some errors
	ec.RecordError(errors.New("error 1"))
	ec.RecordError(errors.New("error 2"))

	if !ec.HasErrors() {
		t.Error("HasErrors should return true")
	}

	if ec.ErrorCount() != 2 {
		t.Errorf("Expected 2 errors, got %d", ec.ErrorCount())
	}

	// Test context integration
	ctx := ec.ToContext(context.Background())
	retrieved, ok := FromContext(ctx)
	if !ok || retrieved.ID != ec.ID {
		t.Error("Context integration failed")
	}

	// Complete and check duration
	ec.Complete()
	if ec.EndTime == nil {
		t.Error("EndTime not set")
	}

	duration := ec.Duration()
	if duration <= 0 {
		t.Error("Invalid duration")
	}
}

func TestRetry(t *testing.T) {
	attempts := 0
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		RetryIf: func(err error) bool {
			return true
		},
	}

	// Test successful retry
	err := Retry(context.Background(), config, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	// Test exhausted retries
	attempts = 0
	err = Retry(context.Background(), config, func() error {
		attempts++
		return errors.New("permanent error")
	})

	if err == nil {
		t.Error("Expected error after exhausted retries")
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestErrorCollector(t *testing.T) {
	collector := NewErrorCollector()

	if collector.HasErrors() {
		t.Error("New collector should have no errors")
	}

	// Add errors
	collector.Add(errors.New("error 1"))
	collector.AddWithContext(errors.New("error 2"), "processing item")
	collector.Add(nil) // Should be ignored

	if collector.Count() != 2 {
		t.Errorf("Expected 2 errors, got %d", collector.Count())
	}

	// Test First and Last
	first := collector.First()
	if first == nil || first.Error() != "error 1" {
		t.Error("First() failed")
	}

	last := collector.Last()
	if last == nil || !contains(last.Error(), "error 2") {
		t.Error("Last() failed")
	}

	// Test combined error
	combined := collector.Error()
	if combined == nil {
		t.Error("Expected combined error")
	}

	// Test Clear
	collector.Clear()
	if collector.HasErrors() {
		t.Error("Clear() failed")
	}
}

func TestErrorMatcher(t *testing.T) {
	// Create test errors
	err1 := NewValidationError("FIELD_REQUIRED", "Field is required")
	err1.BaseError.Severity = SeverityWarning

	err2 := NewStateError("STATE_INVALID", "Invalid state")
	err2.BaseError.Severity = SeverityCritical

	// Test matchers
	matcher := NewErrorMatcher().
		WithCode("FIELD_REQUIRED").
		WithSeverity(SeverityWarning)

	if !matcher.Matches(err1) {
		t.Error("Matcher should match err1")
	}

	if matcher.Matches(err2) {
		t.Error("Matcher should not match err2")
	}

	// Test message matcher
	msgMatcher := NewErrorMatcher().WithMessage("state")
	if !msgMatcher.Matches(err2) {
		t.Error("Message matcher should match err2")
	}
}

func TestChainedError(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")

	chained := Chain(err1, err2, err3)
	if chained == nil {
		t.Fatal("Chain should return non-nil error")
	}

	chainedErr, ok := chained.(*ChainedError)
	if !ok {
		t.Fatal("Expected ChainedError type")
	}

	if len(chainedErr.Errors()) != 3 {
		t.Errorf("Expected 3 errors in chain, got %d", len(chainedErr.Errors()))
	}

	// Test Unwrap
	if chainedErr.Unwrap() != err1 {
		t.Error("Unwrap should return first error")
	}

	// Test with nil errors
	nilChain := Chain(nil, nil)
	if nilChain != nil {
		t.Error("Chain of nil errors should return nil")
	}

	// Test with single error
	single := Chain(err1)
	if single != err1 {
		t.Error("Chain with single error should return that error")
	}
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")

	// Test Wrap
	wrapped := Wrap(originalErr, "additional context")
	if wrapped == nil {
		t.Fatal("Wrap should return non-nil error")
	}

	if !errors.Is(wrapped, originalErr) {
		t.Error("Wrapped error should match original")
	}

	// Test Wrapf
	wrapped2 := Wrapf(originalErr, "context with %s", "formatting")
	if !contains(wrapped2.Error(), "formatting") {
		t.Error("Wrapf formatting failed")
	}

	// Test with nil
	if Wrap(nil, "context") != nil {
		t.Error("Wrap of nil should return nil")
	}
}

// Helper function
func contains(s, substr string) bool {
	return fmt.Sprintf("%v", s) != "" && fmt.Sprintf("%v", s) != "<nil>" &&
		len(s) > 0 && len(substr) > 0 &&
		(s == substr || (len(s) >= len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
