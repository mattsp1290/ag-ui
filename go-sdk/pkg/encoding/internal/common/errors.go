// Package common provides internal utilities shared across the encoding package.
// This package is internal and not exported to external consumers.
package common

import (
	"errors"
	"fmt"
)

// Common error variables used across encoding implementations
var (
	// ErrNilInput indicates that a nil input was provided where a non-nil value was expected
	ErrNilInput = errors.New("nil input provided")

	// ErrInvalidSize indicates that a size value is invalid (e.g., negative or too large)
	ErrInvalidSize = errors.New("invalid size")

	// ErrBufferTooSmall indicates that a buffer is too small for the operation
	ErrBufferTooSmall = errors.New("buffer too small")

	// ErrInvalidFormat indicates that data has an invalid format
	ErrInvalidFormat = errors.New("invalid format")

	// ErrUnsupportedType indicates that a type is not supported
	ErrUnsupportedType = errors.New("unsupported type")

	// ErrDataCorrupted indicates that data appears to be corrupted
	ErrDataCorrupted = errors.New("data corrupted")
)

// WrapError wraps an error with additional context
func WrapError(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// NewError creates a new error with formatted message
func NewError(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// IsNilInputError checks if an error is a nil input error
func IsNilInputError(err error) bool {
	return errors.Is(err, ErrNilInput)
}

// IsInvalidSizeError checks if an error is an invalid size error
func IsInvalidSizeError(err error) bool {
	return errors.Is(err, ErrInvalidSize)
}

// IsBufferTooSmallError checks if an error is a buffer too small error
func IsBufferTooSmallError(err error) bool {
	return errors.Is(err, ErrBufferTooSmall)
}

// IsInvalidFormatError checks if an error is an invalid format error
func IsInvalidFormatError(err error) bool {
	return errors.Is(err, ErrInvalidFormat)
}

// IsUnsupportedTypeError checks if an error is an unsupported type error
func IsUnsupportedTypeError(err error) bool {
	return errors.Is(err, ErrUnsupportedType)
}

// IsDataCorruptedError checks if an error is a data corrupted error
func IsDataCorruptedError(err error) bool {
	return errors.Is(err, ErrDataCorrupted)
}

// ErrorWithCode represents an error with an associated error code
type ErrorWithCode struct {
	Code    string
	Message string
	Err     error
}

// Error implements the error interface
func (e *ErrorWithCode) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error
func (e *ErrorWithCode) Unwrap() error {
	return e.Err
}

// NewErrorWithCode creates a new error with an error code
func NewErrorWithCode(code, message string, err error) *ErrorWithCode {
	return &ErrorWithCode{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
