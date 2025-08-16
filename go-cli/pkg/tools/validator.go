package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// Validator validates arguments against a tool's JSON schema
type Validator struct {
	schema *ToolSchema
}

// NewValidator creates a new validator for a tool schema
func NewValidator(schema *ToolSchema) *Validator {
	return &Validator{
		schema: schema,
	}
}

// Validate validates arguments against the schema
func (v *Validator) Validate(args map[string]interface{}) error {
	if v.schema == nil {
		// No schema means any arguments are valid
		return nil
	}
	
	// Check for required properties
	for _, required := range v.schema.Required {
		if _, exists := args[required]; !exists {
			return &ValidationError{
				Field:   required,
				Message: "required field is missing",
			}
		}
	}
	
	// Check for additional properties if not allowed
	if v.schema.AdditionalProperties != nil && !*v.schema.AdditionalProperties {
		for key := range args {
			if v.schema.Properties == nil || v.schema.Properties[key] == nil {
				return &ValidationError{
					Field:   key,
					Message: "additional property not allowed",
				}
			}
		}
	}
	
	// Validate each property
	if v.schema.Properties != nil {
		for name, prop := range v.schema.Properties {
			if value, exists := args[name]; exists {
				if err := v.validateProperty(name, value, prop); err != nil {
					return err
				}
			}
		}
	}
	
	return nil
}

// validateProperty validates a single property value
func (v *Validator) validateProperty(name string, value interface{}, prop *Property) error {
	if prop == nil {
		return nil
	}
	
	// Handle null values
	if value == nil {
		if prop.Type != "null" && !v.isRequired(name) {
			// Null is allowed for optional fields
			return nil
		}
		if prop.Type != "null" {
			return &ValidationError{
				Field:   name,
				Message: fmt.Sprintf("expected type %s but got null", prop.Type),
			}
		}
		return nil
	}
	
	// Enum validation should happen first, before type validation
	if len(prop.Enum) > 0 {
		if !v.isInEnum(value, prop.Enum) {
			return &ValidationError{
				Field:   name,
				Message: fmt.Sprintf("value must be one of: %v", prop.Enum),
				Value:   value,
			}
		}
		// If enum validation passes, we're done
		return nil
	}
	
	// Type validation
	switch prop.Type {
	case "string":
		return v.validateString(name, value, prop)
	case "number":
		return v.validateNumber(name, value, prop)
	case "integer":
		return v.validateInteger(name, value, prop)
	case "boolean":
		return v.validateBoolean(name, value, prop)
	case "array":
		return v.validateArray(name, value, prop)
	case "object":
		return v.validateObject(name, value, prop)
	case "null":
		if value != nil {
			return &ValidationError{
				Field:   name,
				Message: "expected null value",
			}
		}
	default:
		// If no type specified, any type is valid
		if prop.Type != "" {
			return &ValidationError{
				Field:   name,
				Message: fmt.Sprintf("unknown type: %s", prop.Type),
			}
		}
	}
	
	return nil
}

// validateString validates a string property
func (v *Validator) validateString(name string, value interface{}, prop *Property) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("expected string but got %T", value),
			Value:   value,
		}
	}
	
	// Length constraints
	if prop.MinLength != nil && len(str) < *prop.MinLength {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("string length %d is less than minimum %d", len(str), *prop.MinLength),
			Value:   str,
		}
	}
	
	if prop.MaxLength != nil && len(str) > *prop.MaxLength {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("string length %d exceeds maximum %d", len(str), *prop.MaxLength),
			Value:   str,
		}
	}
	
	// Pattern validation
	if prop.Pattern != "" {
		re, err := regexp.Compile(prop.Pattern)
		if err != nil {
			return &ValidationError{
				Field:   name,
				Message: fmt.Sprintf("invalid regex pattern: %s", prop.Pattern),
			}
		}
		if !re.MatchString(str) {
			return &ValidationError{
				Field:   name,
				Message: fmt.Sprintf("string does not match pattern: %s", prop.Pattern),
				Value:   str,
			}
		}
	}
	
	// Format validation
	if prop.Format != "" {
		if err := v.validateFormat(str, prop.Format); err != nil {
			return &ValidationError{
				Field:   name,
				Message: fmt.Sprintf("invalid format '%s': %v", prop.Format, err),
				Value:   str,
			}
		}
	}
	
	return nil
}

// validateNumber validates a number property
func (v *Validator) validateNumber(name string, value interface{}, prop *Property) error {
	num, ok := toFloat64(value)
	if !ok {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("expected number but got %T", value),
			Value:   value,
		}
	}
	
	// Range constraints
	if prop.Minimum != nil && num < *prop.Minimum {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("value %v is less than minimum %v", num, *prop.Minimum),
			Value:   num,
		}
	}
	
	if prop.Maximum != nil && num > *prop.Maximum {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("value %v exceeds maximum %v", num, *prop.Maximum),
			Value:   num,
		}
	}
	
	if prop.ExclusiveMin != nil && num <= *prop.ExclusiveMin {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("value %v must be greater than %v", num, *prop.ExclusiveMin),
			Value:   num,
		}
	}
	
	if prop.ExclusiveMax != nil && num >= *prop.ExclusiveMax {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("value %v must be less than %v", num, *prop.ExclusiveMax),
			Value:   num,
		}
	}
	
	if prop.MultipleOf != nil && *prop.MultipleOf != 0 {
		if remainder := num / *prop.MultipleOf; remainder != float64(int64(remainder)) {
			return &ValidationError{
				Field:   name,
				Message: fmt.Sprintf("value %v is not a multiple of %v", num, *prop.MultipleOf),
				Value:   num,
			}
		}
	}
	
	return nil
}

// validateInteger validates an integer property
func (v *Validator) validateInteger(name string, value interface{}, prop *Property) error {
	// First check if it's a valid number
	num, ok := toFloat64(value)
	if !ok {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("expected integer but got %T", value),
			Value:   value,
		}
	}
	
	// Check if it's actually an integer
	if num != float64(int64(num)) {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("expected integer but got float: %v", num),
			Value:   value,
		}
	}
	
	// Apply same constraints as number
	return v.validateNumber(name, value, prop)
}

// validateBoolean validates a boolean property
func (v *Validator) validateBoolean(name string, value interface{}, prop *Property) error {
	_, ok := value.(bool)
	if !ok {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("expected boolean but got %T", value),
			Value:   value,
		}
	}
	return nil
}

// validateArray validates an array property
func (v *Validator) validateArray(name string, value interface{}, prop *Property) error {
	arr, ok := toArray(value)
	if !ok {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("expected array but got %T", value),
			Value:   value,
		}
	}
	
	// Length constraints
	if prop.MinItems != nil && len(arr) < *prop.MinItems {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("array length %d is less than minimum %d", len(arr), *prop.MinItems),
			Value:   arr,
		}
	}
	
	if prop.MaxItems != nil && len(arr) > *prop.MaxItems {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("array length %d exceeds maximum %d", len(arr), *prop.MaxItems),
			Value:   arr,
		}
	}
	
	// Unique items
	if prop.UniqueItems != nil && *prop.UniqueItems {
		if !hasUniqueItems(arr) {
			return &ValidationError{
				Field:   name,
				Message: "array items must be unique",
				Value:   arr,
			}
		}
	}
	
	// Validate each item
	if prop.Items != nil {
		for i, item := range arr {
			itemName := fmt.Sprintf("%s[%d]", name, i)
			if err := v.validateProperty(itemName, item, prop.Items); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// validateObject validates an object property
func (v *Validator) validateObject(name string, value interface{}, prop *Property) error {
	obj, ok := value.(map[string]interface{})
	if !ok {
		return &ValidationError{
			Field:   name,
			Message: fmt.Sprintf("expected object but got %T", value),
			Value:   value,
		}
	}
	
	// Check for required properties
	for _, required := range prop.Required {
		if _, exists := obj[required]; !exists {
			return &ValidationError{
				Field:   fmt.Sprintf("%s.%s", name, required),
				Message: "required field is missing",
			}
		}
	}
	
	// Check for additional properties if not allowed
	if prop.AdditionalProperties != nil && !*prop.AdditionalProperties {
		for key := range obj {
			if prop.Properties == nil || prop.Properties[key] == nil {
				return &ValidationError{
					Field:   fmt.Sprintf("%s.%s", name, key),
					Message: "additional property not allowed",
				}
			}
		}
	}
	
	// Validate nested properties
	if prop.Properties != nil {
		for propName, propSchema := range prop.Properties {
			if val, exists := obj[propName]; exists {
				nestedName := fmt.Sprintf("%s.%s", name, propName)
				if err := v.validateProperty(nestedName, val, propSchema); err != nil {
					return err
				}
			}
		}
	}
	
	return nil
}

// validateFormat validates common string formats
func (v *Validator) validateFormat(value string, format string) error {
	switch format {
	case "email":
		if !strings.Contains(value, "@") || !strings.Contains(value, ".") {
			return fmt.Errorf("invalid email format")
		}
	case "uri", "url":
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") &&
			!strings.HasPrefix(value, "ftp://") && !strings.HasPrefix(value, "file://") {
			return fmt.Errorf("invalid URI format")
		}
	case "date":
		// Simple date validation (YYYY-MM-DD)
		dateRegex := regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$`)
		if !dateRegex.MatchString(value) {
			return fmt.Errorf("invalid date format (expected YYYY-MM-DD)")
		}
	case "time":
		// Simple time validation (HH:MM:SS)
		timeRegex := regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)
		if !timeRegex.MatchString(value) {
			return fmt.Errorf("invalid time format (expected HH:MM:SS)")
		}
	case "date-time":
		// Simple datetime validation (ISO 8601)
		if !strings.Contains(value, "T") {
			return fmt.Errorf("invalid datetime format")
		}
	case "uuid":
		uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
		if !uuidRegex.MatchString(strings.ToLower(value)) {
			return fmt.Errorf("invalid UUID format")
		}
	default:
		// Unknown format, skip validation
	}
	return nil
}

// isRequired checks if a field is required
func (v *Validator) isRequired(name string) bool {
	for _, req := range v.schema.Required {
		if req == name {
			return true
		}
	}
	return false
}

// isInEnum checks if a value is in the enum list
func (v *Validator) isInEnum(value interface{}, enum []interface{}) bool {
	for _, enumVal := range enum {
		if reflect.DeepEqual(value, enumVal) {
			return true
		}
		// Try JSON marshaling for complex types
		v1, _ := json.Marshal(value)
		v2, _ := json.Marshal(enumVal)
		if string(v1) == string(v2) {
			return true
		}
	}
	return false
}

// Helper functions

// toFloat64 converts various number types to float64
func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// toArray converts various slice types to []interface{}
func toArray(value interface{}) ([]interface{}, bool) {
	if arr, ok := value.([]interface{}); ok {
		return arr, true
	}
	
	// Use reflection for other slice types
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice {
		return nil, false
	}
	
	arr := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		arr[i] = v.Index(i).Interface()
	}
	return arr, true
}

// hasUniqueItems checks if all items in an array are unique
func hasUniqueItems(arr []interface{}) bool {
	seen := make(map[string]bool)
	for _, item := range arr {
		// Use JSON encoding as a simple way to compare complex types
		key, _ := json.Marshal(item)
		keyStr := string(key)
		if seen[keyStr] {
			return false
		}
		seen[keyStr] = true
	}
	return true
}

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("validation error for field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}