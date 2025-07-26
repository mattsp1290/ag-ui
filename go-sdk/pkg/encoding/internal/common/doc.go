// Package common provides internal utilities shared across the encoding package.
//
// This package contains common utilities, patterns, and helper functions that are
// used internally by the encoding package implementations. It is not intended for
// external use and its API may change without notice.
//
// # Package Structure
//
// The package is organized into the following modules:
//
//   - errors.go: Common error types and error handling utilities
//   - validation.go: Shared validation functions for input validation
//   - buffers.go: Buffer management utilities including pooling and safe access
//   - patterns.go: Common interfaces and patterns used across encoding implementations
//
// # Error Handling
//
// The package provides a consistent error handling approach with predefined error
// types and helper functions for error creation and checking:
//
//	if err := common.ValidateNonNil(value, "input"); err != nil {
//		return common.WrapError(err, "validation failed")
//	}
//
// # Validation
//
// Common validation functions are provided for typical input validation scenarios:
//
//	// Validate multiple conditions
//	err := common.ValidateAll(
//		common.ValidationRule{
//			Name: "non-nil check",
//			Validator: func() error {
//				return common.ValidateNonNil(input, "input")
//			},
//		},
//		common.ValidationRule{
//			Name: "size check",
//			Validator: func() error {
//				return common.ValidateBufferSize(len(data), "data")
//			},
//		},
//	)
//
// # Buffer Management
//
// Efficient buffer management with pooling and thread-safe operations:
//
//	// Use pooled buffers for temporary operations
//	buf := common.GetPooledBuffer(1024)
//	defer common.ReturnPooledBuffer(buf)
//
//	// Use thread-safe buffers for concurrent access
//	safeBuf := common.NewSafeBuffer()
//	safeBuf.WriteString("data")
//
// # Common Patterns
//
// The package defines common interfaces and patterns used across encoding
// implementations:
//
//	type MyEncoder struct{}
//
//	func (e *MyEncoder) Encode(value interface{}) ([]byte, error) {
//		// Implementation
//	}
//
//	func (e *MyEncoder) EncodeWithContext(ctx context.Context, value interface{}) ([]byte, error) {
//		// Implementation with context
//	}
//
// # Internal Use Only
//
// This package is internal to the encoding package and should not be imported
// or used by external code. The APIs provided here may change without notice
// as part of internal refactoring and improvements.
//
// # Thread Safety
//
// Unless explicitly documented otherwise, the utilities in this package are
// not thread-safe and should be used with appropriate synchronization when
// accessed from multiple goroutines. The SafeBuffer type is an exception
// and provides thread-safe operations.
package common