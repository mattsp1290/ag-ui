package config

import (
	"strings"
	"testing"
	"time"
)

func TestSchemaValidator(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":      "string",
				"minLength": float64(1),
				"maxLength": float64(100),
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
			"active": map[string]interface{}{
				"type": "boolean",
			},
		},
		"required": []interface{}{"name", "email"},
	}
	
	validator := NewSchemaValidator("test-schema", schema)
	
	// Test valid configuration
	validConfig := map[string]interface{}{
		"name":   "John Doe",
		"age":    30,
		"email":  "john@example.com",
		"active": true,
	}
	
	err := validator.Validate(validConfig)
	if err != nil {
		t.Errorf("Valid configuration should pass validation: %v", err)
	}
	
	// Test missing required field
	invalidConfig1 := map[string]interface{}{
		"age":    30,
		"active": true,
	}
	
	err = validator.Validate(invalidConfig1)
	if err == nil {
		t.Error("Configuration missing required fields should fail validation")
	}
	
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("Error should mention required field: %v", err)
	}
	
	// Test invalid type
	invalidConfig2 := map[string]interface{}{
		"name":   "John Doe",
		"age":    "thirty", // Should be integer
		"email":  "john@example.com",
		"active": true,
	}
	
	err = validator.Validate(invalidConfig2)
	if err == nil {
		t.Error("Configuration with wrong type should fail validation")
	}
	
	// Test string length validation
	invalidConfig3 := map[string]interface{}{
		"name":   "", // Too short
		"email":  "john@example.com",
		"active": true,
	}
	
	err = validator.Validate(invalidConfig3)
	if err == nil {
		t.Error("Configuration with string too short should fail validation")
	}
	
	// Test number range validation
	invalidConfig4 := map[string]interface{}{
		"name":   "John Doe",
		"age":    -5, // Below minimum
		"email":  "john@example.com",
		"active": true,
	}
	
	err = validator.Validate(invalidConfig4)
	if err == nil {
		t.Error("Configuration with number below minimum should fail validation")
	}
	
	// Test pattern validation
	invalidConfig5 := map[string]interface{}{
		"name":   "John Doe",
		"age":    30,
		"email":  "invalid-email", // Doesn't match pattern
		"active": true,
	}
	
	err = validator.Validate(invalidConfig5)
	if err == nil {
		t.Error("Configuration with invalid email pattern should fail validation")
	}
}

func TestSchemaValidatorNestedObjects(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"server": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"host": map[string]interface{}{
						"type": "string",
					},
					"port": map[string]interface{}{
						"type":    "integer",
						"minimum": float64(1),
						"maximum": float64(65535),
					},
				},
				"required": []interface{}{"host", "port"},
			},
		},
		"required": []interface{}{"server"},
	}
	
	validator := NewSchemaValidator("nested-schema", schema)
	
	// Test valid nested configuration
	validConfig := map[string]interface{}{
		"server": map[string]interface{}{
			"host": "localhost",
			"port": 8080,
		},
	}
	
	err := validator.Validate(validConfig)
	if err != nil {
		t.Errorf("Valid nested configuration should pass validation: %v", err)
	}
	
	// Test missing nested required field
	invalidConfig := map[string]interface{}{
		"server": map[string]interface{}{
			"host": "localhost",
			// Missing port
		},
	}
	
	err = validator.Validate(invalidConfig)
	if err == nil {
		t.Error("Configuration missing nested required field should fail validation")
	}
}

func TestSchemaValidatorArrays(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"tags": map[string]interface{}{
				"type":     "array",
				"minItems": float64(1),
				"maxItems": float64(5),
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}
	
	validator := NewSchemaValidator("array-schema", schema)
	
	// Test valid array
	validConfig := map[string]interface{}{
		"tags": []interface{}{"tag1", "tag2", "tag3"},
	}
	
	err := validator.Validate(validConfig)
	if err != nil {
		t.Errorf("Valid array configuration should pass validation: %v", err)
	}
	
	// Test array too short
	invalidConfig1 := map[string]interface{}{
		"tags": []interface{}{},
	}
	
	err = validator.Validate(invalidConfig1)
	if err == nil {
		t.Error("Array below minimum items should fail validation")
	}
	
	// Test array too long
	invalidConfig2 := map[string]interface{}{
		"tags": []interface{}{"tag1", "tag2", "tag3", "tag4", "tag5", "tag6"},
	}
	
	err = validator.Validate(invalidConfig2)
	if err == nil {
		t.Error("Array above maximum items should fail validation")
	}
	
	// Test invalid array item type
	invalidConfig3 := map[string]interface{}{
		"tags": []interface{}{"tag1", 123, "tag3"}, // 123 is not a string
	}
	
	err = validator.Validate(invalidConfig3)
	if err == nil {
		t.Error("Array with invalid item type should fail validation")
	}
}

func TestSchemaValidatorEnum(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"environment": map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"development", "staging", "production"},
			},
		},
	}
	
	validator := NewSchemaValidator("enum-schema", schema)
	
	// Test valid enum value
	validConfig := map[string]interface{}{
		"environment": "production",
	}
	
	err := validator.Validate(validConfig)
	if err != nil {
		t.Errorf("Valid enum value should pass validation: %v", err)
	}
	
	// Test invalid enum value
	invalidConfig := map[string]interface{}{
		"environment": "invalid-env",
	}
	
	err = validator.Validate(invalidConfig)
	if err == nil {
		t.Error("Invalid enum value should fail validation")
	}
}

func TestCustomValidator(t *testing.T) {
	validator := NewCustomValidator("test-validator")
	
	// Add custom rules
	validator.AddRule("name", RequiredRule)
	validator.AddRule("email", EmailRule)
	validator.AddRule("port", PortRule)
	validator.AddRule("url", URLRule)
	validator.AddRule("duration", DurationRule)
	
	// Add cross-reference rule
	validator.AddCrossReferenceRule(func(config map[string]interface{}) error {
		if config["debug"] == true && config["log_level"] != "debug" {
			return &ValidationErrors{Errors: []ValidationError{
				{
					Field:   "log_level",
					Value:   config["log_level"],
					Rule:    "cross-reference",
					Message: "log_level should be 'debug' when debug is true",
				},
			}}
		}
		return nil
	})
	
	// Test valid configuration
	validConfig := map[string]interface{}{
		"name":      "Test Service",
		"email":     "admin@example.com",
		"port":      8080,
		"url":       "https://api.example.com",
		"duration":  "30s",
		"debug":     true,
		"log_level": "debug",
	}
	
	err := validator.Validate(validConfig)
	if err != nil {
		t.Errorf("Valid configuration should pass validation: %v", err)
	}
	
	// Test invalid configuration
	invalidConfig := map[string]interface{}{
		"name":      "", // Should fail RequiredRule
		"email":     "invalid-email", // Should fail EmailRule
		"port":      70000, // Should fail PortRule
		"url":       "not-a-url", // Should fail URLRule
		"duration":  "invalid-duration", // Should fail DurationRule
		"debug":     true,
		"log_level": "info", // Should fail cross-reference rule
	}
	
	err = validator.Validate(invalidConfig)
	if err == nil {
		t.Error("Invalid configuration should fail validation")
	}
	
	// Check that we got multiple validation errors
	if validationErrors, ok := err.(*ValidationErrors); ok {
		if len(validationErrors.Errors) < 5 {
			t.Errorf("Expected at least 5 validation errors, got %d", len(validationErrors.Errors))
		}
	} else {
		t.Error("Expected ValidationErrors type")
	}
}

func TestValidationRules(t *testing.T) {
	// Test RequiredRule
	err := RequiredRule(nil)
	if err == nil {
		t.Error("RequiredRule should fail for nil value")
	}
	
	err = RequiredRule("")
	if err == nil {
		t.Error("RequiredRule should fail for empty string")
	}
	
	err = RequiredRule("valid")
	if err != nil {
		t.Errorf("RequiredRule should pass for non-empty string: %v", err)
	}
	
	// Test URLRule
	err = URLRule("https://example.com")
	if err != nil {
		t.Errorf("URLRule should pass for valid HTTPS URL: %v", err)
	}
	
	err = URLRule("http://example.com")
	if err != nil {
		t.Errorf("URLRule should pass for valid HTTP URL: %v", err)
	}
	
	err = URLRule("not-a-url")
	if err == nil {
		t.Error("URLRule should fail for invalid URL")
	}
	
	// Test PortRule
	err = PortRule(8080)
	if err != nil {
		t.Errorf("PortRule should pass for valid port: %v", err)
	}
	
	err = PortRule("8080")
	if err != nil {
		t.Errorf("PortRule should pass for valid port string: %v", err)
	}
	
	err = PortRule(0)
	if err == nil {
		t.Error("PortRule should fail for port 0")
	}
	
	err = PortRule(70000)
	if err == nil {
		t.Error("PortRule should fail for port > 65535")
	}
	
	// Test DurationRule
	err = DurationRule("30s")
	if err != nil {
		t.Errorf("DurationRule should pass for valid duration string: %v", err)
	}
	
	err = DurationRule(time.Minute * 5)
	if err != nil {
		t.Errorf("DurationRule should pass for duration value: %v", err)
	}
	
	err = DurationRule("invalid-duration")
	if err == nil {
		t.Error("DurationRule should fail for invalid duration string")
	}
	
	// Test EmailRule
	err = EmailRule("user@example.com")
	if err != nil {
		t.Errorf("EmailRule should pass for valid email: %v", err)
	}
	
	err = EmailRule("invalid-email")
	if err == nil {
		t.Error("EmailRule should fail for invalid email")
	}
	
	err = EmailRule("user@")
	if err == nil {
		t.Error("EmailRule should fail for incomplete email")
	}
	
	// Test RangeRule
	rangeRule := RangeRule(1, 10)
	
	err = rangeRule(5)
	if err != nil {
		t.Errorf("RangeRule should pass for value in range: %v", err)
	}
	
	err = rangeRule(0)
	if err == nil {
		t.Error("RangeRule should fail for value below range")
	}
	
	err = rangeRule(11)
	if err == nil {
		t.Error("RangeRule should fail for value above range")
	}
	
	// Test OneOfRule
	oneOfRule := OneOfRule("option1", "option2", "option3")
	
	err = oneOfRule("option2")
	if err != nil {
		t.Errorf("OneOfRule should pass for valid option: %v", err)
	}
	
	err = oneOfRule("invalid-option")
	if err == nil {
		t.Error("OneOfRule should fail for invalid option")
	}
}

func TestValidationErrors(t *testing.T) {
	// Test single error
	err := &ValidationError{
		Field:   "test.field",
		Value:   "test_value",
		Rule:    "test_rule",
		Message: "test error message",
	}
	
	expectedMsg := "validation failed for field 'test.field' (rule: test_rule): test error message"
	if err.Error() != expectedMsg {
		t.Errorf("Expected '%s', got '%s'", expectedMsg, err.Error())
	}
	
	// Test multiple errors
	errors := &ValidationErrors{}
	errors.Add(ValidationError{
		Field:   "field1",
		Message: "error1",
	})
	errors.Add(ValidationError{
		Field:   "field2",
		Message: "error2",
	})
	
	if !errors.HasErrors() {
		t.Error("HasErrors should return true")
	}
	
	errMsg := errors.Error()
	if !strings.Contains(errMsg, "field1") || !strings.Contains(errMsg, "field2") {
		t.Errorf("Multiple errors message should contain all field names: %s", errMsg)
	}
	
	// Test no errors
	noErrors := &ValidationErrors{}
	if noErrors.HasErrors() {
		t.Error("HasErrors should return false for empty errors")
	}
}

func TestSchemaValidatorFieldValidation(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":      "string",
				"minLength": float64(1),
			},
			"nested": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"value": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}
	
	validator := NewSchemaValidator("field-schema", schema)
	
	// Test field validation
	err := validator.ValidateField("name", "valid_name")
	if err != nil {
		t.Errorf("Field validation should pass for valid value: %v", err)
	}
	
	err = validator.ValidateField("name", "")
	if err == nil {
		t.Error("Field validation should fail for empty string")
	}
	
	// Test non-existent field
	err = validator.ValidateField("nonexistent", "value")
	if err != nil {
		t.Errorf("Field validation should pass for non-existent field: %v", err)
	}
}

func BenchmarkSchemaValidator(b *testing.B) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":      "string",
				"minLength": float64(1),
				"maxLength": float64(100),
			},
			"age": map[string]interface{}{
				"type":    "integer",
				"minimum": float64(0),
				"maximum": float64(150),
			},
		},
		"required": []interface{}{"name"},
	}
	
	validator := NewSchemaValidator("benchmark-schema", schema)
	
	config := map[string]interface{}{
		"name": "John Doe",
		"age":  30,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.Validate(config)
	}
}

func BenchmarkCustomValidator(b *testing.B) {
	validator := NewCustomValidator("benchmark-validator")
	validator.AddRule("name", RequiredRule)
	validator.AddRule("email", EmailRule)
	validator.AddRule("port", PortRule)
	
	config := map[string]interface{}{
		"name":  "Test Service",
		"email": "admin@example.com",
		"port":  8080,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.Validate(config)
	}
}