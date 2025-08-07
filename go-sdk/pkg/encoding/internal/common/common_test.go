package common

import (
	"testing"
)

// TestPackageInitialization ensures the package can be imported and basic types work
func TestPackageInitialization(t *testing.T) {
	// Test error constants
	if ErrNilInput == nil {
		t.Error("ErrNilInput should not be nil")
	}

	if ErrInvalidSize == nil {
		t.Error("ErrInvalidSize should not be nil")
	}

	// Test error creation
	err := NewError("test error: %s", "message")
	if err == nil {
		t.Error("NewError should return an error")
	}

	// Test validation
	if err := ValidateNonNil("test", "field"); err != nil {
		t.Errorf("ValidateNonNil should not return error for non-nil value: %v", err)
	}

	if err := ValidateNonNil(nil, "field"); err == nil {
		t.Error("ValidateNonNil should return error for nil value")
	}

	// Test buffer pool
	buf := GetPooledBuffer(100)
	if buf == nil {
		t.Error("GetPooledBuffer should return a buffer")
	}

	// Test safe buffer
	safeBuf := NewSafeBuffer()
	if safeBuf == nil {
		t.Error("NewSafeBuffer should return a buffer")
	}

	// Test context
	ctx := NewContext()
	if ctx == nil {
		t.Error("NewContext should return a context")
	}

	ctx.WithValue("key", "value")
	if value, exists := ctx.GetValue("key"); !exists || value != "value" {
		t.Error("Context value should be retrievable")
	}
}

// TestErrorWrapping tests error wrapping functionality
func TestErrorWrapping(t *testing.T) {
	originalErr := NewError("original error")
	wrappedErr := WrapError(originalErr, "wrapped: %s", "context")

	if wrappedErr == nil {
		t.Error("WrapError should return an error")
	}

	if !IsNilInputError(ErrNilInput) {
		t.Error("IsNilInputError should return true for ErrNilInput")
	}

	if IsNilInputError(ErrInvalidSize) {
		t.Error("IsNilInputError should return false for ErrInvalidSize")
	}
}

// TestValidationRules tests validation rule system
func TestValidationRules(t *testing.T) {
	rules := []ValidationRule{
		{
			Name: "test rule 1",
			Validator: func() error {
				return nil // Success
			},
		},
		{
			Name: "test rule 2",
			Validator: func() error {
				return nil // Success
			},
		},
	}

	if err := ValidateAll(rules...); err != nil {
		t.Errorf("ValidateAll should not return error when all rules pass: %v", err)
	}

	failingRules := []ValidationRule{
		{
			Name: "failing rule",
			Validator: func() error {
				return NewError("test failure")
			},
		},
	}

	if err := ValidateAll(failingRules...); err == nil {
		t.Error("ValidateAll should return error when rules fail")
	}
}
