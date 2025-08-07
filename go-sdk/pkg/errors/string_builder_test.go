package errors

import (
	"testing"
)

// TestStringBuilderCompatibility tests that string builder optimizations
// produce the same results as the original string concatenation
func TestStringBuilderCompatibility(t *testing.T) {
	// Test cases for error message formatting functions
	testCases := []struct {
		name     string
		testFunc func() (string, string) // returns (optimized, original)
	}{
		{
			name: "FormatComponentError_with_component",
			testFunc: func() (string, string) {
				optimized := FormatComponentError("auth", "login", "invalid credentials")
				// Simulate original implementation
				original := "auth" + " " + "login" + ": " + "invalid credentials"
				return optimized, original
			},
		},
		{
			name: "FormatComponentError_without_component",
			testFunc: func() (string, string) {
				optimized := FormatComponentError("", "login", "invalid credentials")
				// Simulate original implementation
				original := "login" + ": " + "invalid credentials"
				return optimized, original
			},
		},
		{
			name: "FormatFieldError",
			testFunc: func() (string, string) {
				optimized := FormatFieldError("email", "is required")
				// Simulate original implementation
				original := "field '" + "email" + "' " + "is required"
				return optimized, original
			},
		},
		{
			name: "FormatOperationError",
			testFunc: func() (string, string) {
				optimized := FormatOperationError("save", "database connection failed")
				// Simulate original implementation
				original := "save" + " " + SuffixFailed + ": " + "database connection failed"
				return optimized, original
			},
		},
		{
			name: "FormatResourceError_with_id",
			testFunc: func() (string, string) {
				optimized := FormatResourceError("user", "123", "not found")
				// Simulate original implementation
				original := "user" + " '" + "123" + "' " + "not found"
				return optimized, original
			},
		},
		{
			name: "FormatResourceError_without_id",
			testFunc: func() (string, string) {
				optimized := FormatResourceError("user", "", "not found")
				// Simulate original implementation
				original := "user" + " " + "not found"
				return optimized, original
			},
		},
		{
			name: "FormatSecurityError_with_location",
			testFunc: func() (string, string) {
				optimized := FormatSecurityError("XSS", "input field", "malicious script detected")
				// Simulate original implementation
				original := MsgSecurityViolation + " (" + "XSS" + ") at " + "input field" + ": " + "malicious script detected"
				return optimized, original
			},
		},
		{
			name: "FormatSecurityError_without_location",
			testFunc: func() (string, string) {
				optimized := FormatSecurityError("XSS", "", "malicious script detected")
				// Simulate original implementation
				original := MsgSecurityViolation + " (" + "XSS" + "): " + "malicious script detected"
				return optimized, original
			},
		},
		{
			name: "FormatNotImplementedError_with_component",
			testFunc: func() (string, string) {
				optimized := FormatNotImplementedError("auth", "validateToken")
				// Simulate original implementation
				original := "auth" + "." + "validateToken" + ": " + MsgMethodNotImplemented
				return optimized, original
			},
		},
		{
			name: "FormatNotImplementedError_without_component",
			testFunc: func() (string, string) {
				optimized := FormatNotImplementedError("", "validateToken")
				// Simulate original implementation
				original := "validateToken" + ": " + MsgMethodNotImplemented
				return optimized, original
			},
		},
		{
			name: "FormatTimeoutError",
			testFunc: func() (string, string) {
				optimized := FormatTimeoutError("database query", "30s")
				// Simulate original implementation
				original := "database query" + " " + MsgOperationTimeout + " after " + "30s"
				return optimized, original
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			optimized, original := tc.testFunc()
			if optimized != original {
				t.Errorf("String builder optimization changed output:\nOptimized: %q\nOriginal:  %q",
					optimized, original)
			}
		})
	}
}

// BenchmarkStringBuilderPerformance compares string concatenation vs strings.Builder
func BenchmarkStringBuilderPerformance(b *testing.B) {
	b.Run("FormatComponentError_Optimized", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = FormatComponentError("authentication", "login", "invalid credentials provided")
		}
	})

	b.Run("FormatComponentError_Original", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate original string concatenation
			component := "authentication"
			operation := "login"
			message := "invalid credentials provided"
			_ = component + " " + operation + ": " + message
		}
	})

	b.Run("FormatSecurityError_Optimized", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = FormatSecurityError("SQL_INJECTION", "user_input", "malicious query detected")
		}
	})

	b.Run("FormatSecurityError_Original", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate original string concatenation
			violationType := "SQL_INJECTION"
			location := "user_input"
			message := "malicious query detected"
			_ = MsgSecurityViolation + " (" + violationType + ") at " + location + ": " + message
		}
	})
}
