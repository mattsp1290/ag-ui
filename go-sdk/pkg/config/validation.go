package config

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Rule    string
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError
}

// Error implements the error interface
func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}
	
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	
	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	
	return fmt.Sprintf("multiple validation errors: %s", strings.Join(msgs, "; "))
}

// Add adds a validation error
func (e *ValidationErrors) Add(err ValidationError) {
	e.Errors = append(e.Errors, err)
}

// HasErrors returns true if there are validation errors
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// SchemaValidator validates configuration against a JSON Schema
type SchemaValidator struct {
	name   string
	schema map[string]interface{}
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator(name string, schema map[string]interface{}) *SchemaValidator {
	return &SchemaValidator{
		name:   name,
		schema: schema,
	}
}

// Name returns the validator name
func (v *SchemaValidator) Name() string {
	return v.name
}

// Validate validates the entire configuration
func (v *SchemaValidator) Validate(config map[string]interface{}) error {
	return v.validateObject(config, v.schema, "")
}

// ValidateField validates a specific field
func (v *SchemaValidator) ValidateField(key string, value interface{}) error {
	// Navigate to the field schema
	fieldSchema := v.getFieldSchema(v.schema, key)
	if fieldSchema == nil {
		return nil // No schema for this field
	}
	
	return v.validateValue(value, fieldSchema, key)
}

// GetSchema returns the validation schema
func (v *SchemaValidator) GetSchema() map[string]interface{} {
	return v.schema
}

// validateObject validates an object against a schema
func (v *SchemaValidator) validateObject(obj map[string]interface{}, schema map[string]interface{}, path string) error {
	validationErrors := &ValidationErrors{}
	
	// Get properties schema
	properties, hasProperties := schema["properties"].(map[string]interface{})
	required, hasRequired := schema["required"].([]interface{})
	
	// Validate required fields
	if hasRequired {
		for _, req := range required {
			if reqField, ok := req.(string); ok {
				if _, exists := obj[reqField]; !exists {
					validationErrors.Add(ValidationError{
						Field:   v.joinPath(path, reqField),
						Value:   nil,
						Rule:    "required",
						Message: "field is required",
					})
				}
			}
		}
	}
	
	// Validate existing fields
	for field, value := range obj {
		fieldPath := v.joinPath(path, field)
		
		if hasProperties {
			if fieldSchema, ok := properties[field].(map[string]interface{}); ok {
				if err := v.validateValue(value, fieldSchema, fieldPath); err != nil {
					if valErr, ok := err.(*ValidationErrors); ok {
						validationErrors.Errors = append(validationErrors.Errors, valErr.Errors...)
					} else {
						validationErrors.Add(ValidationError{
							Field:   fieldPath,
							Value:   value,
							Rule:    "schema",
							Message: err.Error(),
						})
					}
				}
			}
		}
	}
	
	if validationErrors.HasErrors() {
		return validationErrors
	}
	
	return nil
}

// validateValue validates a single value against a schema
func (v *SchemaValidator) validateValue(value interface{}, schema map[string]interface{}, path string) error {
	validationErrors := &ValidationErrors{}
	
	// Type validation
	if schemaType, ok := schema["type"].(string); ok {
		if !v.validateType(value, schemaType) {
			validationErrors.Add(ValidationError{
				Field:   path,
				Value:   value,
				Rule:    "type",
				Message: fmt.Sprintf("expected type %s, got %T", schemaType, value),
			})
		}
	}
	
	// String validations
	if v.isStringType(value) {
		str := value.(string)
		
		// MinLength validation
		if minLength, ok := schema["minLength"].(float64); ok {
			if len(str) < int(minLength) {
				validationErrors.Add(ValidationError{
					Field:   path,
					Value:   value,
					Rule:    "minLength",
					Message: fmt.Sprintf("string length %d is less than minimum %d", len(str), int(minLength)),
				})
			}
		}
		
		// MaxLength validation
		if maxLength, ok := schema["maxLength"].(float64); ok {
			if len(str) > int(maxLength) {
				validationErrors.Add(ValidationError{
					Field:   path,
					Value:   value,
					Rule:    "maxLength",
					Message: fmt.Sprintf("string length %d exceeds maximum %d", len(str), int(maxLength)),
				})
			}
		}
		
		// Pattern validation
		if pattern, ok := schema["pattern"].(string); ok {
			if regex, err := regexp.Compile(pattern); err == nil {
				if !regex.MatchString(str) {
					validationErrors.Add(ValidationError{
						Field:   path,
						Value:   value,
						Rule:    "pattern",
						Message: fmt.Sprintf("string does not match pattern %s", pattern),
					})
				}
			}
		}
		
		// Enum validation
		if enum, ok := schema["enum"].([]interface{}); ok {
			valid := false
			for _, enumValue := range enum {
				if str == fmt.Sprintf("%v", enumValue) {
					valid = true
					break
				}
			}
			if !valid {
				validationErrors.Add(ValidationError{
					Field:   path,
					Value:   value,
					Rule:    "enum",
					Message: fmt.Sprintf("value must be one of %v", enum),
				})
			}
		}
	}
	
	// Number validations
	if v.isNumericType(value) {
		var num float64
		switch v := value.(type) {
		case int:
			num = float64(v)
		case int64:
			num = float64(v)
		case float64:
			num = v
		}
		
		// Minimum validation
		if minimum, ok := schema["minimum"].(float64); ok {
			if num < minimum {
				validationErrors.Add(ValidationError{
					Field:   path,
					Value:   value,
					Rule:    "minimum",
					Message: fmt.Sprintf("value %v is less than minimum %v", num, minimum),
				})
			}
		}
		
		// Maximum validation
		if maximum, ok := schema["maximum"].(float64); ok {
			if num > maximum {
				validationErrors.Add(ValidationError{
					Field:   path,
					Value:   value,
					Rule:    "maximum",
					Message: fmt.Sprintf("value %v exceeds maximum %v", num, maximum),
				})
			}
		}
	}
	
	// Array validations
	if v.isArrayType(value) {
		arr := value.([]interface{})
		
		// MinItems validation
		if minItems, ok := schema["minItems"].(float64); ok {
			if len(arr) < int(minItems) {
				validationErrors.Add(ValidationError{
					Field:   path,
					Value:   value,
					Rule:    "minItems",
					Message: fmt.Sprintf("array length %d is less than minimum %d", len(arr), int(minItems)),
				})
			}
		}
		
		// MaxItems validation
		if maxItems, ok := schema["maxItems"].(float64); ok {
			if len(arr) > int(maxItems) {
				validationErrors.Add(ValidationError{
					Field:   path,
					Value:   value,
					Rule:    "maxItems",
					Message: fmt.Sprintf("array length %d exceeds maximum %d", len(arr), int(maxItems)),
				})
			}
		}
		
		// Items validation
		if itemsSchema, ok := schema["items"].(map[string]interface{}); ok {
			for i, item := range arr {
				itemPath := fmt.Sprintf("%s[%d]", path, i)
				if err := v.validateValue(item, itemsSchema, itemPath); err != nil {
					if valErr, ok := err.(*ValidationErrors); ok {
						validationErrors.Errors = append(validationErrors.Errors, valErr.Errors...)
					} else {
						validationErrors.Add(ValidationError{
							Field:   itemPath,
							Value:   item,
							Rule:    "items",
							Message: err.Error(),
						})
					}
				}
			}
		}
	}
	
	// Object validation
	if v.isObjectType(value) {
		if objMap, ok := value.(map[string]interface{}); ok {
			if err := v.validateObject(objMap, schema, path); err != nil {
				if valErr, ok := err.(*ValidationErrors); ok {
					validationErrors.Errors = append(validationErrors.Errors, valErr.Errors...)
				} else {
					validationErrors.Add(ValidationError{
						Field:   path,
						Value:   value,
						Rule:    "object",
						Message: err.Error(),
					})
				}
			}
		}
	}
	
	if validationErrors.HasErrors() {
		return validationErrors
	}
	
	return nil
}

// validateType checks if a value matches the expected type
func (v *SchemaValidator) validateType(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		return v.isStringType(value)
	case "number":
		return v.isNumericType(value)
	case "integer":
		return v.isIntegerType(value)
	case "boolean":
		return v.isBooleanType(value)
	case "array":
		return v.isArrayType(value)
	case "object":
		return v.isObjectType(value)
	case "null":
		return value == nil
	default:
		return true // Unknown type, assume valid
	}
}

// Type checking helpers
func (v *SchemaValidator) isStringType(value interface{}) bool {
	_, ok := value.(string)
	return ok
}

func (v *SchemaValidator) isNumericType(value interface{}) bool {
	switch value.(type) {
	case int, int64, float64:
		return true
	default:
		return false
	}
}

func (v *SchemaValidator) isIntegerType(value interface{}) bool {
	switch value.(type) {
	case int, int64:
		return true
	default:
		return false
	}
}

func (v *SchemaValidator) isBooleanType(value interface{}) bool {
	_, ok := value.(bool)
	return ok
}

func (v *SchemaValidator) isArrayType(value interface{}) bool {
	_, ok := value.([]interface{})
	return ok
}

func (v *SchemaValidator) isObjectType(value interface{}) bool {
	_, ok := value.(map[string]interface{})
	return ok
}

// getFieldSchema navigates to a field's schema using dot notation
func (v *SchemaValidator) getFieldSchema(schema map[string]interface{}, key string) map[string]interface{} {
	keys := strings.Split(key, ".")
	current := schema
	
	for _, k := range keys {
		if properties, ok := current["properties"].(map[string]interface{}); ok {
			if fieldSchema, ok := properties[k].(map[string]interface{}); ok {
				current = fieldSchema
			} else {
				return nil
			}
		} else {
			return nil
		}
	}
	
	return current
}

// joinPath joins path segments with dots
func (v *SchemaValidator) joinPath(base, field string) string {
	if base == "" {
		return field
	}
	return base + "." + field
}

// CustomValidator allows custom validation rules
type CustomValidator struct {
	name      string
	rules     map[string]ValidationRule
	crossRefs []CrossReferenceRule
}

// ValidationRule represents a single validation rule
type ValidationRule func(value interface{}) error

// CrossReferenceRule represents a rule that validates across multiple fields
type CrossReferenceRule func(config map[string]interface{}) error

// NewCustomValidator creates a new custom validator
func NewCustomValidator(name string) *CustomValidator {
	return &CustomValidator{
		name:  name,
		rules: make(map[string]ValidationRule),
	}
}

// Name returns the validator name
func (v *CustomValidator) Name() string {
	return v.name
}

// AddRule adds a validation rule for a specific field
func (v *CustomValidator) AddRule(field string, rule ValidationRule) {
	v.rules[field] = rule
}

// AddCrossReferenceRule adds a cross-reference validation rule
func (v *CustomValidator) AddCrossReferenceRule(rule CrossReferenceRule) {
	v.crossRefs = append(v.crossRefs, rule)
}

// Validate validates the entire configuration
func (v *CustomValidator) Validate(config map[string]interface{}) error {
	validationErrors := &ValidationErrors{}
	
	// Validate individual field rules
	for field, rule := range v.rules {
		value := v.getNestedValue(config, field)
		if err := rule(value); err != nil {
			validationErrors.Add(ValidationError{
				Field:   field,
				Value:   value,
				Rule:    "custom",
				Message: err.Error(),
			})
		}
	}
	
	// Validate cross-reference rules
	for _, rule := range v.crossRefs {
		if err := rule(config); err != nil {
			validationErrors.Add(ValidationError{
				Field:   "cross-reference",
				Value:   nil,
				Rule:    "cross-reference",
				Message: err.Error(),
			})
		}
	}
	
	if validationErrors.HasErrors() {
		return validationErrors
	}
	
	return nil
}

// ValidateField validates a specific field
func (v *CustomValidator) ValidateField(key string, value interface{}) error {
	if rule, ok := v.rules[key]; ok {
		return rule(value)
	}
	return nil
}

// GetSchema returns the validation schema (empty for custom validators)
func (v *CustomValidator) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"description": fmt.Sprintf("Custom validation rules for %s", v.name),
	}
}

// getNestedValue retrieves a nested value using dot notation
func (v *CustomValidator) getNestedValue(config map[string]interface{}, key string) interface{} {
	keys := strings.Split(key, ".")
	current := config
	
	for i, k := range keys {
		if i == len(keys)-1 {
			return current[k]
		}
		
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	
	return nil
}

// Common validation rules

// RequiredRule validates that a field is present and not empty
func RequiredRule(value interface{}) error {
	if value == nil {
		return fmt.Errorf("field is required")
	}
	
	if str, ok := value.(string); ok && str == "" {
		return fmt.Errorf("field cannot be empty")
	}
	
	return nil
}

// URLRule validates that a field is a valid URL
func URLRule(value interface{}) error {
	if value == nil {
		return nil
	}
	
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string")
	}
	
	// Basic URL validation
	if !strings.HasPrefix(str, "http://") && !strings.HasPrefix(str, "https://") {
		return fmt.Errorf("value must be a valid URL starting with http:// or https://")
	}
	
	return nil
}

// PortRule validates that a field is a valid port number
func PortRule(value interface{}) error {
	if value == nil {
		return nil
	}
	
	var port int
	switch v := value.(type) {
	case int:
		port = v
	case int64:
		port = int(v)
	case string:
		if p, err := strconv.Atoi(v); err != nil {
			return fmt.Errorf("port must be a number")
		} else {
			port = p
		}
	default:
		return fmt.Errorf("port must be a number")
	}
	
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	
	return nil
}

// DurationRule validates that a field is a valid duration string
func DurationRule(value interface{}) error {
	if value == nil {
		return nil
	}
	
	switch v := value.(type) {
	case string:
		if _, err := time.ParseDuration(v); err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}
	case time.Duration:
		// Already a duration, valid
	default:
		return fmt.Errorf("duration must be a string or time.Duration")
	}
	
	return nil
}

// EmailRule validates that a field is a valid email address
func EmailRule(value interface{}) error {
	if value == nil {
		return nil
	}
	
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("email must be a string")
	}
	
	// Simple email validation regex
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(str) {
		return fmt.Errorf("invalid email format")
	}
	
	return nil
}

// RangeRule validates that a numeric field is within a specified range
func RangeRule(min, max float64) ValidationRule {
	return func(value interface{}) error {
		if value == nil {
			return nil
		}
		
		var num float64
		switch v := value.(type) {
		case int:
			num = float64(v)
		case int64:
			num = float64(v)
		case float64:
			num = v
		default:
			return fmt.Errorf("value must be a number")
		}
		
		if num < min || num > max {
			return fmt.Errorf("value %v must be between %v and %v", num, min, max)
		}
		
		return nil
	}
}

// OneOfRule validates that a field is one of the specified values
func OneOfRule(validValues ...interface{}) ValidationRule {
	return func(value interface{}) error {
		if value == nil {
			return nil
		}
		
		for _, validValue := range validValues {
			if reflect.DeepEqual(value, validValue) {
				return nil
			}
		}
		
		return fmt.Errorf("value must be one of %v", validValues)
	}
}