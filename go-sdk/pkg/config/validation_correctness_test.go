package config

import (
	"fmt"
	"testing"
)

// TestValidationCorrectness_CacheDoesNotAffectResults ensures that caching
// doesn't change validation results compared to non-cached validation
func TestValidationCorrectness_CacheDoesNotAffectResults(t *testing.T) {
	// Create a complex schema validator
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":      "string",
				"minLength": float64(1),
				"maxLength": float64(50),
			},
			"age": map[string]interface{}{
				"type":    "integer",
				"minimum": float64(0),
				"maximum": float64(150),
			},
			"email": map[string]interface{}{
				"type":    "string",
				"pattern": `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			},
			"tags": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"minItems": float64(0),
				"maxItems": float64(5),
			},
		},
		"required": []interface{}{"name", "email"},
	}

	// Create original validator and cached validator
	originalValidator := NewSchemaValidator("test-schema", schema)
	cachedValidator := NewCachedValidator(originalValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	// Test cases with expected results
	testCases := []struct {
		name           string
		config         map[string]interface{}
		shouldValidate bool
	}{
		{
			name: "valid configuration",
			config: map[string]interface{}{
				"name":  "John Doe",
				"age":   30,
				"email": "john@example.com",
				"tags":  []interface{}{"user", "admin"},
			},
			shouldValidate: true,
		},
		{
			name: "missing required field",
			config: map[string]interface{}{
				"name": "John Doe",
				"age":  30,
				// missing email
			},
			shouldValidate: false,
		},
		{
			name: "invalid email format",
			config: map[string]interface{}{
				"name":  "John Doe",
				"age":   30,
				"email": "invalid-email",
			},
			shouldValidate: false,
		},
		{
			name: "age out of range",
			config: map[string]interface{}{
				"name":  "John Doe",
				"age":   200, // too old
				"email": "john@example.com",
			},
			shouldValidate: false,
		},
		{
			name: "name too long",
			config: map[string]interface{}{
				"name":  "This is a very long name that exceeds the maximum length limit",
				"age":   30,
				"email": "john@example.com",
			},
			shouldValidate: false,
		},
		{
			name: "too many tags",
			config: map[string]interface{}{
				"name":  "John Doe",
				"age":   30,
				"email": "john@example.com",
				"tags":  []interface{}{"tag1", "tag2", "tag3", "tag4", "tag5", "tag6"}, // 6 tags, max is 5
			},
			shouldValidate: false,
		},
		{
			name: "empty config",
			config: map[string]interface{}{},
			shouldValidate: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test with original validator
			originalErr := originalValidator.Validate(tc.config)
			originalPassed := originalErr == nil

			// Test with cached validator (first time - cache miss)
			cachedErr1 := cachedValidator.Validate(tc.config)
			cachedPassed1 := cachedErr1 == nil

			// Test with cached validator (second time - cache hit)
			cachedErr2 := cachedValidator.Validate(tc.config)
			cachedPassed2 := cachedErr2 == nil

			// Verify results match expected behavior
			if originalPassed != tc.shouldValidate {
				t.Errorf("Original validator result doesn't match expected: got %v, want %v", originalPassed, tc.shouldValidate)
			}

			// Verify cached results match original results
			if originalPassed != cachedPassed1 {
				t.Errorf("First cached validation doesn't match original: original=%v, cached=%v", originalPassed, cachedPassed1)
			}

			if originalPassed != cachedPassed2 {
				t.Errorf("Second cached validation doesn't match original: original=%v, cached=%v", originalPassed, cachedPassed2)
			}

			if cachedPassed1 != cachedPassed2 {
				t.Errorf("Cached results inconsistent: first=%v, second=%v", cachedPassed1, cachedPassed2)
			}

			// For failed validations, verify error messages are consistent
			if originalErr != nil && cachedErr1 != nil && cachedErr2 != nil {
				if originalErr.Error() != cachedErr1.Error() {
					t.Errorf("Error messages don't match between original and first cached: original='%s', cached='%s'", 
						originalErr.Error(), cachedErr1.Error())
				}
				if cachedErr1.Error() != cachedErr2.Error() {
					t.Errorf("Error messages don't match between cached calls: first='%s', second='%s'", 
						cachedErr1.Error(), cachedErr2.Error())
				}
			}
		})
	}
}

// TestValidationCorrectness_FieldValidation ensures field validation correctness with caching
func TestValidationCorrectness_FieldValidation(t *testing.T) {
	// Create a custom validator for field testing
	customValidator := NewCustomValidator("test-custom")
	
	// Add some validation rules
	customValidator.AddRule("name", RequiredRule)
	customValidator.AddRule("email", EmailRule)
	customValidator.AddRule("port", PortRule)
	customValidator.AddRule("age", RangeRule(0, 150))

	// Create cached version
	cachedValidator := NewCachedValidator(customValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	// Test cases for field validation
	fieldTestCases := []struct {
		field          string
		value          interface{}
		shouldValidate bool
	}{
		{"name", "John Doe", true},
		{"name", "", false},          // empty string should fail RequiredRule
		{"name", nil, false},         // nil should fail RequiredRule
		
		{"email", "john@example.com", true},
		{"email", "invalid-email", false},
		{"email", "", false},         // EmailRule fails on empty string (regex doesn't match)
		{"email", nil, true},         // EmailRule allows nil
		
		{"port", 8080, true},
		{"port", 0, false},           // port 0 is invalid
		{"port", 70000, false},       // port too high
		{"port", "8080", true},       // string port is valid
		{"port", "invalid", false},   // invalid string port
		
		{"age", 25, true},
		{"age", -5, false},           // negative age
		{"age", 200, false},          // age too high
		{"age", 0, true},             // zero age is valid
		{"age", 150, true},           // max age is valid
	}

	for _, tc := range fieldTestCases {
		t.Run(fmt.Sprintf("%s_%v", tc.field, tc.value), func(t *testing.T) {
			// Test with original validator
			originalErr := customValidator.ValidateField(tc.field, tc.value)
			originalPassed := originalErr == nil

			// Test with cached validator (first time - cache miss)
			cachedErr1 := cachedValidator.ValidateField(tc.field, tc.value)
			cachedPassed1 := cachedErr1 == nil

			// Test with cached validator (second time - cache hit)
			cachedErr2 := cachedValidator.ValidateField(tc.field, tc.value)
			cachedPassed2 := cachedErr2 == nil

			// Verify results match expected behavior
			if originalPassed != tc.shouldValidate {
				t.Errorf("Original validator result doesn't match expected: got %v, want %v, error: %v", 
					originalPassed, tc.shouldValidate, originalErr)
			}

			// Verify cached results match original results
			if originalPassed != cachedPassed1 {
				t.Errorf("First cached field validation doesn't match original: original=%v, cached=%v", 
					originalPassed, cachedPassed1)
			}

			if originalPassed != cachedPassed2 {
				t.Errorf("Second cached field validation doesn't match original: original=%v, cached=%v", 
					originalPassed, cachedPassed2)
			}

			if cachedPassed1 != cachedPassed2 {
				t.Errorf("Cached field results inconsistent: first=%v, second=%v", cachedPassed1, cachedPassed2)
			}

			// For failed validations, verify error messages are consistent
			if originalErr != nil && cachedErr1 != nil && cachedErr2 != nil {
				if originalErr.Error() != cachedErr1.Error() {
					t.Errorf("Field error messages don't match: original='%s', cached='%s'", 
						originalErr.Error(), cachedErr1.Error())
				}
			}
		})
	}
}

// TestValidationCorrectness_ComplexScenarios tests complex validation scenarios
func TestValidationCorrectness_ComplexScenarios(t *testing.T) {
	// Create a validator with cross-reference rules
	customValidator := NewCustomValidator("complex-validator")
	
	// Add a cross-reference rule: if type is "admin", email domain must be "company.com"
	customValidator.AddCrossReferenceRule(func(config map[string]interface{}) error {
		userType, hasType := config["type"].(string)
		email, hasEmail := config["email"].(string)
		
		if hasType && userType == "admin" && hasEmail {
			if len(email) > 11 && email[len(email)-11:] != "company.com" {
				return fmt.Errorf("admin users must have company.com email")
			}
		}
		return nil
	})

	// Create cached version
	cachedValidator := NewCachedValidator(customValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	// Test cases
	testCases := []struct {
		name   string
		config map[string]interface{}
		shouldValidate bool
	}{
		{
			name: "admin with company email",
			config: map[string]interface{}{
				"type":  "admin",
				"email": "admin@company.com",
			},
			shouldValidate: true,
		},
		{
			name: "admin with external email",
			config: map[string]interface{}{
				"type":  "admin",
				"email": "admin@external.com",
			},
			shouldValidate: false,
		},
		{
			name: "user with external email",
			config: map[string]interface{}{
				"type":  "user",
				"email": "user@external.com",
			},
			shouldValidate: true,
		},
		{
			name: "admin without email",
			config: map[string]interface{}{
				"type": "admin",
			},
			shouldValidate: true, // No email to validate
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test multiple times to ensure caching doesn't affect cross-reference rules
			for i := 0; i < 3; i++ {
				originalErr := customValidator.Validate(tc.config)
				cachedErr := cachedValidator.Validate(tc.config)
				
				originalPassed := originalErr == nil
				cachedPassed := cachedErr == nil
				
				if originalPassed != tc.shouldValidate {
					t.Errorf("Iteration %d: Original validator result doesn't match expected: got %v, want %v", 
						i, originalPassed, tc.shouldValidate)
				}
				
				if originalPassed != cachedPassed {
					t.Errorf("Iteration %d: Cached result doesn't match original: original=%v, cached=%v", 
						i, originalPassed, cachedPassed)
				}
				
				if originalErr != nil && cachedErr != nil {
					if originalErr.Error() != cachedErr.Error() {
						t.Errorf("Iteration %d: Error messages don't match: original='%s', cached='%s'", 
							i, originalErr.Error(), cachedErr.Error())
					}
				}
			}
		})
	}
}

// TestValidationCorrectness_ConfigurationChanges tests that cache invalidation 
// works correctly when configuration changes
func TestValidationCorrectness_ConfigurationChanges(t *testing.T) {
	// Create a validator that we can modify
	customValidator := NewCustomValidator("changeable-validator")
	
	// Initially, only require name
	customValidator.AddRule("name", RequiredRule)

	cachedValidator := NewCachedValidator(customValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	config := map[string]interface{}{
		"name": "John Doe",
	}

	// First validation should pass
	err1 := cachedValidator.Validate(config)
	if err1 != nil {
		t.Errorf("First validation should pass: %v", err1)
	}

	// Second validation should be cached and pass
	err2 := cachedValidator.Validate(config)
	if err2 != nil {
		t.Errorf("Second validation should pass (cached): %v", err2)
	}

	// Now add a rule that makes this config invalid
	customValidator.AddRule("email", RequiredRule)

	// Invalidate cache to simulate configuration change
	cachedValidator.InvalidateCache()

	// Third validation should now fail because email is required but missing
	err3 := cachedValidator.Validate(config)
	if err3 == nil {
		t.Error("Third validation should fail after adding email requirement")
	}

	// Fourth validation should be cached and still fail
	err4 := cachedValidator.Validate(config)
	if err4 == nil {
		t.Error("Fourth validation should fail (cached)")
	}

	// Add email to make it valid again
	configWithEmail := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}

	err5 := cachedValidator.Validate(configWithEmail)
	if err5 != nil {
		t.Errorf("Fifth validation should pass with email: %v", err5)
	}

	// Verify it's cached
	err6 := cachedValidator.Validate(configWithEmail)
	if err6 != nil {
		t.Errorf("Sixth validation should pass (cached): %v", err6)
	}
}

// TestValidationCorrectness_ErrorTypes ensures that error types are preserved through caching
func TestValidationCorrectness_ErrorTypes(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []interface{}{"name"},
	}

	originalValidator := NewSchemaValidator("test-schema", schema)
	cachedValidator := NewCachedValidator(originalValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	invalidConfig := map[string]interface{}{
		"age": 30, // missing required "name" field
	}

	// Get original error
	originalErr := originalValidator.Validate(invalidConfig)
	if originalErr == nil {
		t.Fatal("Expected validation error from original validator")
	}

	// Get cached error (first call)
	cachedErr1 := cachedValidator.Validate(invalidConfig)
	if cachedErr1 == nil {
		t.Fatal("Expected validation error from cached validator (first call)")
	}

	// Get cached error (second call)
	cachedErr2 := cachedValidator.Validate(invalidConfig)
	if cachedErr2 == nil {
		t.Fatal("Expected validation error from cached validator (second call)")
	}

	// Check that error types match
	_, originalIsValidationErrors := originalErr.(*ValidationErrors)
	_, cached1IsValidationErrors := cachedErr1.(*ValidationErrors)
	_, cached2IsValidationErrors := cachedErr2.(*ValidationErrors)

	if originalIsValidationErrors != cached1IsValidationErrors {
		t.Error("Error type mismatch between original and first cached")
	}
	if cached1IsValidationErrors != cached2IsValidationErrors {
		t.Error("Error type mismatch between first and second cached")
	}

	// Check error details
	if originalIsValidationErrors {
		originalVE := originalErr.(*ValidationErrors)
		cached1VE := cachedErr1.(*ValidationErrors)
		cached2VE := cachedErr2.(*ValidationErrors)

		if len(originalVE.Errors) != len(cached1VE.Errors) {
			t.Error("Different number of validation errors between original and first cached")
		}
		if len(cached1VE.Errors) != len(cached2VE.Errors) {
			t.Error("Different number of validation errors between first and second cached")
		}

		// Check first error details (if any)
		if len(originalVE.Errors) > 0 && len(cached1VE.Errors) > 0 {
			if originalVE.Errors[0].Rule != cached1VE.Errors[0].Rule {
				t.Error("Validation error rules don't match")
			}
			if originalVE.Errors[0].Field != cached1VE.Errors[0].Field {
				t.Error("Validation error fields don't match")
			}
		}
	}
}