package errors

import (
	"context"
	"errors"
	"testing"
)

func TestStructuredErrorTypes(t *testing.T) {
	// Test AuthenticationError
	authErr := NewAuthenticationError(AuthErrorInvalidCredentials, "Invalid username or password").
		WithUser("testuser").
		WithTokenType("Bearer")

	if !IsAuthenticationError(authErr) {
		t.Error("Expected IsAuthenticationError to return true")
	}

	if authErr.UserID != "testuser" {
		t.Errorf("Expected UserID to be 'testuser', got %s", authErr.UserID)
	}

	if authErr.Code != AuthErrorInvalidCredentials {
		t.Errorf("Expected Code to be %s, got %s", AuthErrorInvalidCredentials, authErr.Code)
	}

	// Test CacheError
	cacheErr := NewCacheError(CacheErrorConnectionFailed, "Redis connection failed").
		WithLevel("L2").
		WithKey("test:key").
		WithOperation("get")

	if !IsCacheError(cacheErr) {
		t.Error("Expected IsCacheError to return true")
	}

	if !IsRetryable(cacheErr) {
		t.Error("Expected connection failed errors to be retryable")
	}

	if cacheErr.CacheLevel != "L2" {
		t.Errorf("Expected CacheLevel to be 'L2', got %s", cacheErr.CacheLevel)
	}

	// Test ValidationError
	validationErr := NewValidationError(ValidationErrorRequired, "Field is required").
		WithRule("REQUIRED_FIELD").
		WithEvent("event123", "run_started").
		WithField("user_id", nil, "non-empty string")

	if !IsValidationError(validationErr) {
		t.Error("Expected IsValidationError to return true")
	}

	if validationErr.RuleID != "REQUIRED_FIELD" {
		t.Errorf("Expected RuleID to be 'REQUIRED_FIELD', got %s", validationErr.RuleID)
	}

	// Test error wrapping and unwrapping
	wrappedErr := NewCacheError(CacheErrorTimeout, "Operation timed out").WithCause(context.DeadlineExceeded)
	if !errors.Is(wrappedErr, context.DeadlineExceeded) {
		t.Error("Expected wrapped error to match original cause")
	}
}

func TestErrorCodeMapping(t *testing.T) {
	// Test legacy error code conversion
	legacyErr := ConvertLegacyError("AUTH_VALIDATION", "Authentication required")
	
	if !IsAuthenticationError(legacyErr) {
		t.Error("Expected converted legacy error to be an authentication error")
	}

	code := GetErrorCode(legacyErr)
	if code != AuthErrorAuthRequired {
		t.Errorf("Expected error code to be %s, got %s", AuthErrorAuthRequired, code)
	}

	// Test unknown legacy code conversion
	unknownErr := ConvertLegacyError("UNKNOWN_RULE", "Unknown error")
	
	if !IsValidationError(unknownErr) {
		t.Error("Expected unknown legacy error to be converted to validation error")
	}
}

func TestRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()
	
	if policy.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts to be 3, got %d", policy.MaxAttempts)
	}

	// Test retry mechanism
	attempts := 0
	ctx := context.Background()
	
	err := Retry(ctx, policy, func(ctx context.Context, attempt int) error {
		attempts++
		if attempts < 2 {
			return NewCacheError(CacheErrorTimeout, "Timeout")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected retry to succeed, got error: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}

	// Test retry exhaustion
	attempts = 0
	err = Retry(ctx, policy, func(ctx context.Context, attempt int) error {
		attempts++
		return NewCacheError(CacheErrorTimeout, "Always fails")
	})

	if err == nil {
		t.Error("Expected retry to fail after exhausting attempts")
	}

	var retryErr *RetryExhaustedError
	if !errors.As(err, &retryErr) {
		t.Errorf("Expected RetryExhaustedError, got %T: %v", err, err)
	} else {
		if retryErr.Attempts != policy.MaxAttempts {
			t.Errorf("Expected %d attempts in retry error, got %d", policy.MaxAttempts, retryErr.Attempts)
		}
	}
}

func TestCacheRetryPolicy(t *testing.T) {
	policy := CacheRetryPolicy()
	
	if policy.MaxAttempts != 2 {
		t.Errorf("Expected MaxAttempts to be 2 for cache policy, got %d", policy.MaxAttempts)
	}

	// Test that cache errors are retryable
	cacheErr := NewCacheError(CacheErrorConnectionFailed, "Connection failed")
	if !policy.isRetryable(cacheErr) {
		t.Error("Expected connection failed errors to be retryable in cache policy")
	}

	// Test that non-retryable errors are not retried
	authErr := NewAuthenticationError(AuthErrorInvalidCredentials, "Invalid creds")
	if policy.isRetryable(authErr) {
		t.Error("Expected auth errors to not be retryable in cache policy")
	}
}

func TestLogger(t *testing.T) {
	logger := NewDefaultLogger("test")
	
	// Test that logger doesn't panic (basic functionality test)
	logger.Info("Test info message")
	logger.Warn("Test warning message")
	logger.Error("Test error message")
	logger.Logf("Test formatted message: %s", "test")

	// Test NoOpLogger
	noopLogger := &NoOpLogger{}
	noopLogger.Info("This should not output anything")
	noopLogger.Error("This should not output anything")
}

func TestErrorCategories(t *testing.T) {
	authErr := NewAuthenticationError(AuthErrorInvalidCredentials, "test")
	cacheErr := NewCacheError(CacheErrorTimeout, "test")
	validationErr := NewValidationError(ValidationErrorRequired, "test")

	if GetErrorCategory(authErr) != CategoryAuthentication {
		t.Errorf("Expected authentication category, got %s", GetErrorCategory(authErr))
	}

	if GetErrorCategory(cacheErr) != CategoryCache {
		t.Errorf("Expected cache category, got %s", GetErrorCategory(cacheErr))
	}

	if GetErrorCategory(validationErr) != CategoryValidation {
		t.Errorf("Expected validation category, got %s", GetErrorCategory(validationErr))
	}

	// Test HasErrorCode function
	if !HasErrorCode(authErr, AuthErrorInvalidCredentials) {
		t.Error("Expected HasErrorCode to return true for matching code")
	}

	if HasErrorCode(authErr, AuthErrorTokenExpired) {
		t.Error("Expected HasErrorCode to return false for non-matching code")
	}
}