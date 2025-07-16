package validation

import (
	"context"
	"strings"
	"testing"

	agerrors "github.com/ag-ui/go-sdk/pkg/errors"
)

// TestValidateNestingDepthLimit verifies that validateNestingDepth prevents stack overflow attacks
func TestValidateNestingDepthLimit(t *testing.T) {
	config := StrictSecurityConfig()
	validator := NewSecurityValidator(config)
	ctx := context.Background()

	tests := []struct {
		name      string
		data      interface{}
		maxDepth  int
		expectErr bool
		errType   string
	}{
		{
			name:      "shallow nesting within limit",
			data:      map[string]interface{}{"level1": map[string]interface{}{"level2": "value"}},
			maxDepth:  10,
			expectErr: false,
		},
		{
			name:      "deep nesting exceeds recursion limit",
			data:      createDeeplyNestedMap(100),
			maxDepth:  15,
			expectErr: true,
			errType:   "recursion_depth_limit",
		},
		{
			name:      "array nesting exceeds limit", 
			data:      createDeeplyNestedArray(100),
			maxDepth:  15,
			expectErr: true,
			errType:   "recursion_depth_limit",
		},
		{
			name:      "mixed object/array nesting exceeds limit",
			data:      createMixedNesting(60),
			maxDepth:  15,
			expectErr: true,
			errType:   "recursion_depth_limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateNestingDepthWithLimit(ctx, tt.data, 0, tt.maxDepth)
			
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				
				if secErr, ok := err.(*agerrors.SecurityError); ok {
					if violationType := secErr.ViolationType; violationType != tt.errType {
						t.Errorf("expected violation type %s, got %s", tt.errType, violationType)
					}
				} else {
					t.Errorf("expected SecurityError, got %T: %v", err, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestSanitizeValueDepthLimit verifies that sanitizeValue prevents stack overflow attacks
func TestSanitizeValueDepthLimit(t *testing.T) {
	config := StrictSecurityConfig()
	validator := NewSecurityValidator(config)

	tests := []struct {
		name      string
		data      interface{}
		maxDepth  int
		expectSafe bool
	}{
		{
			name:       "shallow nesting sanitized correctly",
			data:       map[string]interface{}{"level1": map[string]interface{}{"level2": "value"}},
			maxDepth:   10,
			expectSafe: true,
		},
		{
			name:       "deep nesting stops sanitization safely",
			data:       createDeeplyNestedMap(100),
			maxDepth:   25,
			expectSafe: true, // Should not crash, just return unsanitized at depth limit
		},
		{
			name:       "deep array nesting stops safely",
			data:       createDeeplyNestedArray(100),
			maxDepth:   25,
			expectSafe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not crash even with deep nesting
			result := validator.sanitizeValueWithDepth(tt.data, 0, tt.maxDepth)
			
			if !tt.expectSafe {
				t.Error("expected unsafe operation")
			}
			
			// Result should be non-nil (even if unsanitized at depth limit)
			if result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

// TestSecurityConfigDepthLimits verifies that security configurations include proper depth limits
func TestSecurityConfigDepthLimits(t *testing.T) {
	tests := []struct {
		name           string
		config         SecurityConfig
		expectedValDepth int
		expectedSanDepth int
	}{
		{
			name:             "default config has proper limits",
			config:           DefaultSecurityConfig(),
			expectedValDepth: DefaultMaxValidationDepth,
			expectedSanDepth: DefaultMaxSanitizationDepth,
		},
		{
			name:             "strict config has stricter limits",
			config:           StrictSecurityConfig(),
			expectedValDepth: StrictMaxValidationDepth,
			expectedSanDepth: StrictMaxSanitizationDepth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.MaxValidationDepth != tt.expectedValDepth {
				t.Errorf("expected MaxValidationDepth %d, got %d", tt.expectedValDepth, tt.config.MaxValidationDepth)
			}
			if tt.config.MaxSanitizationDepth != tt.expectedSanDepth {
				t.Errorf("expected MaxSanitizationDepth %d, got %d", tt.expectedSanDepth, tt.config.MaxSanitizationDepth)
			}
		})
	}
}

// TestDepthLimitConstantsAreReasonable verifies that depth limit constants are reasonable
func TestDepthLimitConstantsAreReasonable(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		minValue int
		maxValue int
	}{
		{"DefaultMaxValidationDepth", DefaultMaxValidationDepth, 50, 500},
		{"DefaultMaxSanitizationDepth", DefaultMaxSanitizationDepth, 25, 250},
		{"StrictMaxValidationDepth", StrictMaxValidationDepth, 25, 100},
		{"StrictMaxSanitizationDepth", StrictMaxSanitizationDepth, 10, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value < tt.minValue || tt.value > tt.maxValue {
				t.Errorf("%s value %d is outside reasonable range [%d, %d]", tt.name, tt.value, tt.minValue, tt.maxValue)
			}
		})
	}
}

// TestRecursionDepthErrorMessages verifies that depth limit errors contain useful information
func TestRecursionDepthErrorMessages(t *testing.T) {
	config := StrictSecurityConfig()
	validator := NewSecurityValidator(config)
	ctx := context.Background()

	// Create deeply nested data that will exceed limits
	deepData := createDeeplyNestedMap(100)
	
	err := validator.validateNestingDepthWithLimit(ctx, deepData, 0, 10)
	if err == nil {
		t.Fatal("expected error for deep nesting")
	}

	errMsg := err.Error()
	
	// Check that error message contains useful information
	requiredStrings := []string{
		"recursion depth",
		"exceeds maximum",
		"10", // the limit we set
	}

	for _, required := range requiredStrings {
		if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(required)) {
			t.Errorf("error message should contain '%s', got: %s", required, errMsg)
		}
	}
}

// Helper functions to create test data

func createDeeplyNestedMap(depth int) map[string]interface{} {
	if depth <= 0 {
		return map[string]interface{}{"value": "end"}
	}
	return map[string]interface{}{
		"level": createDeeplyNestedMap(depth - 1),
	}
}

func createDeeplyNestedArray(depth int) []interface{} {
	if depth <= 0 {
		return []interface{}{"end"}
	}
	return []interface{}{createDeeplyNestedArray(depth - 1)}
}

func createMixedNesting(depth int) interface{} {
	if depth <= 0 {
		return "end"
	}
	
	if depth%2 == 0 {
		return map[string]interface{}{
			"nested": createMixedNesting(depth - 1),
		}
	} else {
		return []interface{}{createMixedNesting(depth - 1)}
	}
}

// Benchmark tests to verify performance characteristics

func BenchmarkValidateNestingDepthShallow(b *testing.B) {
	config := DefaultSecurityConfig()
	validator := NewSecurityValidator(config)
	ctx := context.Background()
	data := createDeeplyNestedMap(10)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.validateNestingDepth(ctx, data, 0)
	}
}

func BenchmarkValidateNestingDepthDeep(b *testing.B) {
	config := DefaultSecurityConfig()
	validator := NewSecurityValidator(config)
	ctx := context.Background()
	data := createDeeplyNestedMap(50) // Just under default limit
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.validateNestingDepth(ctx, data, 0)
	}
}

func BenchmarkSanitizeValueShallow(b *testing.B) {
	config := DefaultSecurityConfig()
	validator := NewSecurityValidator(config)
	data := createDeeplyNestedMap(10)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.sanitizeValue(data)
	}
}

func BenchmarkSanitizeValueDeep(b *testing.B) {
	config := DefaultSecurityConfig()
	validator := NewSecurityValidator(config)
	data := createDeeplyNestedMap(25) // Just under default limit
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.sanitizeValue(data)
	}
}