package tools

import (
	"strings"
	"testing"
)

// TestSchemaValidatorDepthLimits verifies that schema validation prevents stack overflow attacks
func TestSchemaValidatorDepthLimits(t *testing.T) {
	// Create a schema that allows deep nesting through recursive definition
	nestedProperty := &Property{
		Type: "object",
		Properties: map[string]*Property{
			"value": {Type: "string"},
		},
	}

	// Create self-referencing property structure
	nestedProperty.Properties["nested"] = nestedProperty

	schema := &ToolSchema{
		Type: "object",
		Properties: map[string]*Property{
			"nested": nestedProperty,
		},
	}

	tests := []struct {
		name            string
		maxDepth        int
		data            map[string]interface{}
		expectError     bool
		expectedErrCode string
	}{
		{
			name:        "shallow nesting within limits",
			maxDepth:    100,
			data:        createNestedObject(5),
			expectError: false,
		},
		{
			name:            "deep nesting exceeds limits",
			maxDepth:        10,
			data:            createNestedObject(50),
			expectError:     true,
			expectedErrCode: "RECURSION_DEPTH_EXCEEDED",
		},
		{
			name:        "exactly at limit should pass",
			maxDepth:    21,
			data:        createNestedObject(20),
			expectError: false,
		},
		{
			name:            "one beyond limit should fail",
			maxDepth:        20,
			data:            createNestedObject(20),
			expectError:     true,
			expectedErrCode: "RECURSION_DEPTH_EXCEEDED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &ValidatorOptions{
				MaxValidationDepth: tt.maxDepth,
			}
			validator := NewAdvancedSchemaValidator(schema, opts)

			err := validator.Validate(tt.data)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}

				// Check if it's a ValidationError with the expected code
				if validationErr, ok := err.(*ValidationError); ok {
					if validationErr.Code != tt.expectedErrCode {
						t.Errorf("expected error code '%s', got: %s (error: %s)", tt.expectedErrCode, validationErr.Code, err.Error())
					}
				} else {
					// Fallback: check if error message contains the code
					errMsg := err.Error()
					if !strings.Contains(errMsg, tt.expectedErrCode) {
						t.Errorf("expected error containing '%s', got: %s", tt.expectedErrCode, errMsg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestSchemaCompositionDepthLimits tests that composition schemas (oneOf, anyOf, etc.) respect depth limits
func TestSchemaCompositionDepthLimits(t *testing.T) {
	// Create a schema with deep composition nesting
	schema := &ToolSchema{
		Type: "object",
		Properties: map[string]*Property{
			"data": {
				OneOf: []*Property{
					{
						Type: "object",
						Properties: map[string]*Property{
							"nested": {
								OneOf: []*Property{
									{Type: "string"},
									{Type: "number"},
								},
							},
						},
					},
					{Type: "string"},
				},
			},
		},
	}

	tests := []struct {
		name        string
		maxDepth    int
		data        map[string]interface{}
		expectError bool
	}{
		{
			name:        "composition within limits",
			maxDepth:    50,
			data:        map[string]interface{}{"data": map[string]interface{}{"nested": "value"}},
			expectError: false,
		},
		{
			name:        "deep composition nesting",
			maxDepth:    5,
			data:        createDeeplyNestedComposition(10),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &ValidatorOptions{
				MaxValidationDepth: tt.maxDepth,
			}
			validator := NewAdvancedSchemaValidator(schema, opts)

			err := validator.Validate(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestArrayValidationDepthLimits tests that array validation respects depth limits
func TestArrayValidationDepthLimits(t *testing.T) {
	schema := &ToolSchema{
		Type: "object",
		Properties: map[string]*Property{
			"items": {
				Type: "array",
				Items: &Property{
					Type: "array",
					Items: &Property{
						Type: "string",
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		maxDepth    int
		data        map[string]interface{}
		expectError bool
	}{
		{
			name:        "shallow array nesting",
			maxDepth:    50,
			data:        map[string]interface{}{"items": []interface{}{[]interface{}{"value"}}},
			expectError: false,
		},
		{
			name:        "deep array nesting exceeds limit",
			maxDepth:    5,
			data:        map[string]interface{}{"items": createDeeplyNestedArrayData(10)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &ValidatorOptions{
				MaxValidationDepth: tt.maxDepth,
			}
			validator := NewAdvancedSchemaValidator(schema, opts)

			err := validator.Validate(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestDepthLimitDefaults verifies that default depth limits are set correctly
func TestDepthLimitDefaults(t *testing.T) {
	schema := &ToolSchema{Type: "object"}

	// Test default validator
	defaultValidator := NewSchemaValidator(schema)
	if defaultValidator.maxValidationDepth != DefaultMaxSchemaValidationDepth {
		t.Errorf("expected default depth %d, got %d", DefaultMaxSchemaValidationDepth, defaultValidator.maxValidationDepth)
	}

	// Test advanced validator with no options
	advancedValidator := NewAdvancedSchemaValidator(schema, nil)
	if advancedValidator.maxValidationDepth != DefaultMaxSchemaValidationDepth {
		t.Errorf("expected default depth %d, got %d", DefaultMaxSchemaValidationDepth, advancedValidator.maxValidationDepth)
	}

	// Test advanced validator with custom depth
	customDepth := 200
	customValidator := NewAdvancedSchemaValidator(schema, &ValidatorOptions{
		MaxValidationDepth: customDepth,
	})
	if customValidator.maxValidationDepth != customDepth {
		t.Errorf("expected custom depth %d, got %d", customDepth, customValidator.maxValidationDepth)
	}
}

// TestConditionalSchemaDepthLimits tests that conditional schemas (if/then/else) respect depth limits
func TestConditionalSchemaDepthLimits(t *testing.T) {
	// Create a recursive object schema that uses conditionals for validation
	// This creates a scenario where conditional validation must traverse deeply
	objectWithConditionalValidation := &Property{
		Type: "object",
		Properties: map[string]*Property{
			"type":  {Type: "string"},
			"value": {Type: "string"},
		},
		// Add conditional validation that must be applied at each level
		If: &Property{
			Type: "object",
			Properties: map[string]*Property{
				"type": {Enum: []interface{}{"test"}},
			},
		},
		Then: &Property{
			Type: "object",
			Properties: map[string]*Property{
				"value": {Type: "string", MinLength: intPtr2(1)}, // Additional validation
			},
		},
	}

	// Add recursive nested property
	objectWithConditionalValidation.Properties["nested"] = objectWithConditionalValidation

	schema := &ToolSchema{
		Type: "object",
		Properties: map[string]*Property{
			"conditional": objectWithConditionalValidation,
		},
	}

	tests := []struct {
		name        string
		maxDepth    int
		data        map[string]interface{}
		expectError bool
	}{
		{
			name:     "conditional within limits",
			maxDepth: 50,
			data: map[string]interface{}{
				"conditional": map[string]interface{}{
					"type":  "test",
					"value": "valid",
					"nested": map[string]interface{}{
						"type":  "test",
						"value": "also_valid",
					},
				},
			},
			expectError: false,
		},
		{
			name:     "conditional exceeds depth limit",
			maxDepth: 3,
			data: map[string]interface{}{
				"conditional": createDeeplyNestedConditional(5),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &ValidatorOptions{
				MaxValidationDepth: tt.maxDepth,
			}
			validator := NewAdvancedSchemaValidator(schema, opts)

			err := validator.Validate(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestLegitimateDeepNesting verifies that reasonable deep nesting still works
func TestLegitimateDeepNesting(t *testing.T) {
	// Create a schema that represents a realistic deeply nested structure
	schema := &ToolSchema{
		Type: "object",
		Properties: map[string]*Property{
			"config": {
				Type: "object",
				Properties: map[string]*Property{
					"database": {
						Type: "object",
						Properties: map[string]*Property{
							"connection": {
								Type: "object",
								Properties: map[string]*Property{
									"settings": {
										Type: "object",
										Properties: map[string]*Property{
											"advanced": {
												Type: "object",
												Properties: map[string]*Property{
													"ssl": {
														Type: "object",
														Properties: map[string]*Property{
															"certificate": {
																Type: "object",
																Properties: map[string]*Property{
																	"path": {Type: "string"},
																	"key":  {Type: "string"},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Test data that represents a legitimate deeply nested configuration
	legitimateData := map[string]interface{}{
		"config": map[string]interface{}{
			"database": map[string]interface{}{
				"connection": map[string]interface{}{
					"settings": map[string]interface{}{
						"advanced": map[string]interface{}{
							"ssl": map[string]interface{}{
								"certificate": map[string]interface{}{
									"path": "/path/to/cert.pem",
									"key":  "/path/to/key.pem",
								},
							},
						},
					},
				},
			},
		},
	}

	validator := NewSchemaValidator(schema)
	err := validator.Validate(legitimateData)

	if err != nil {
		t.Errorf("legitimate deep nesting should not fail validation: %v", err)
	}
}

// Helper functions

func createNestedObject(depth int) map[string]interface{} {
	if depth <= 0 {
		return map[string]interface{}{"value": "end"}
	}
	return map[string]interface{}{
		"nested": createNestedObject(depth - 1),
	}
}

func createDeeplyNestedComposition(depth int) map[string]interface{} {
	if depth <= 0 {
		return map[string]interface{}{"data": "end"}
	}
	return map[string]interface{}{
		"data": map[string]interface{}{
			"nested": createDeeplyNestedComposition(depth - 1),
		},
	}
}

func createDeeplyNestedArrayData(depth int) []interface{} {
	if depth <= 0 {
		return []interface{}{"end"}
	}
	return []interface{}{createDeeplyNestedArrayData(depth - 1)}
}

func createDeeplyNestedConditional(depth int) map[string]interface{} {
	if depth <= 0 {
		return map[string]interface{}{
			"type":  "test",
			"value": "end",
		}
	}
	return map[string]interface{}{
		"type":   "test",
		"nested": createDeeplyNestedConditional(depth - 1),
	}
}

// Benchmark tests

func BenchmarkSchemaValidationShallow(b *testing.B) {
	schema := &ToolSchema{
		Type: "object",
		Properties: map[string]*Property{
			"nested": {Type: "object"},
		},
	}
	validator := NewSchemaValidator(schema)
	data := createNestedObject(5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(data)
	}
}

func BenchmarkSchemaValidationDeep(b *testing.B) {
	schema := &ToolSchema{
		Type: "object",
		Properties: map[string]*Property{
			"nested": {Type: "object"},
		},
	}
	validator := NewSchemaValidator(schema)
	data := createNestedObject(50) // Near default limit

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(data)
	}
}

// Helper function for creating int pointers
func intPtr2(i int) *int {
	return &i
}
