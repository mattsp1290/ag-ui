package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// SchemaValidationError represents a JSON schema validation error with details
type SchemaValidationError struct {
	Path    string      // JSON path to the invalid field (e.g., "properties.url")
	Message string      // Human-readable error message
	Value   interface{} // The invalid value
	Constraint interface{} // The schema constraint that was violated
}

func (e *SchemaValidationError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("validation error at '%s': %s", e.Path, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// SchemaValidationErrors collects multiple validation errors
type SchemaValidationErrors struct {
	Errors []*SchemaValidationError
}

func (e *SchemaValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	
	var messages []string
	for _, err := range e.Errors {
		messages = append(messages, err.Error())
	}
	return fmt.Sprintf("multiple validation errors:\n  - %s", strings.Join(messages, "\n  - "))
}

// Add adds a validation error to the collection
func (e *SchemaValidationErrors) Add(path, message string, value, constraint interface{}) {
	e.Errors = append(e.Errors, &SchemaValidationError{
		Path:    path,
		Message: message,
		Value:   value,
		Constraint: constraint,
	})
}

// HasErrors returns true if there are any validation errors
func (e *SchemaValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// ValidateArguments validates tool arguments against a JSON schema
func ValidateArguments(args json.RawMessage, schema *ToolSchema) error {
	if schema == nil {
		// No schema means no validation required
		return nil
	}
	
	// Parse arguments into a generic structure
	var data interface{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &data); err != nil {
			return fmt.Errorf("invalid JSON arguments: %w", err)
		}
	}
	
	// Validate against schema
	errors := &SchemaValidationErrors{}
	validateToolSchema("", data, schema, errors)
	
	if errors.HasErrors() {
		return errors
	}
	
	return nil
}

// validateToolSchema validates a value against a ToolSchema (root object schema)
func validateToolSchema(path string, value interface{}, schema *ToolSchema, errors *SchemaValidationErrors) {
	if schema == nil {
		return
	}
	
	// ToolSchema is typically an object
	if schema.Type != "" {
		actualType := getJSONType(value)
		if !matchesType(actualType, schema.Type) {
			errors.Add(path, fmt.Sprintf("expected type '%s' but got '%s'", schema.Type, actualType), value, schema.Type)
			return
		}
	}
	
	// For object type, validate properties
	if schema.Type == "object" {
		obj, ok := value.(map[string]interface{})
		if !ok {
			if value != nil {
				errors.Add(path, "expected object type", value, "object")
			}
			return
		}
		
		// Check required properties
		for _, required := range schema.Required {
			if _, exists := obj[required]; !exists {
				fieldPath := joinSchemaPath(path, required)
				errors.Add(fieldPath, "required property is missing", nil, "required")
			}
		}
		
		// Validate each property
		if schema.Properties != nil {
			for propName, propValue := range obj {
				propSchema, exists := schema.Properties[propName]
				if exists {
					propPath := joinSchemaPath(path, propName)
					validateProperty(propPath, propValue, propSchema, errors)
				} else if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
					propPath := joinSchemaPath(path, propName)
					errors.Add(propPath, "additional property not allowed", propValue, false)
				}
			}
		}
	}
}

// validateProperty validates a value against a Property schema
func validateProperty(path string, value interface{}, prop *Property, errors *SchemaValidationErrors) {
	if prop == nil {
		return
	}
	
	// Check type constraint
	if prop.Type != "" {
		actualType := getJSONType(value)
		if !matchesType(actualType, prop.Type) {
			errors.Add(path, fmt.Sprintf("expected type '%s' but got '%s'", prop.Type, actualType), value, prop.Type)
			return
		}
	}
	
	// Validate based on type
	switch prop.Type {
	case "object":
		validateObjectProperty(path, value, prop, errors)
	case "array":
		validateArrayProperty(path, value, prop, errors)
	case "string":
		validateStringProperty(path, value, prop, errors)
	case "number", "integer":
		validateNumberProperty(path, value, prop, errors)
	case "boolean":
		// Boolean values don't need additional validation beyond type check
	case "null":
		if value != nil {
			errors.Add(path, "expected null value", value, nil)
		}
	}
	
	// Check enum constraint
	if len(prop.Enum) > 0 {
		if !containsValue(prop.Enum, value) {
			errors.Add(path, fmt.Sprintf("value must be one of %v", prop.Enum), value, prop.Enum)
		}
	}
}

// validateObjectProperty validates an object against a Property schema
func validateObjectProperty(path string, value interface{}, prop *Property, errors *SchemaValidationErrors) {
	obj, ok := value.(map[string]interface{})
	if !ok {
		if value != nil {
			errors.Add(path, "expected object type", value, "object")
		}
		return
	}
	
	// Check required properties
	for _, required := range prop.Required {
		if _, exists := obj[required]; !exists {
			fieldPath := joinSchemaPath(path, required)
			errors.Add(fieldPath, "required property is missing", nil, "required")
		}
	}
	
	// Validate nested properties
	if prop.Properties != nil {
		for propName, propValue := range obj {
			nestedProp, exists := prop.Properties[propName]
			if exists {
				propPath := joinSchemaPath(path, propName)
				validateProperty(propPath, propValue, nestedProp, errors)
			}
		}
	}
}

// validateArrayProperty validates an array against a Property schema
func validateArrayProperty(path string, value interface{}, prop *Property, errors *SchemaValidationErrors) {
	arr, ok := value.([]interface{})
	if !ok {
		if value != nil {
			errors.Add(path, "expected array type", value, "array")
		}
		return
	}
	
	// Validate each item against the items schema
	if prop.Items != nil {
		for i, item := range arr {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			validateProperty(itemPath, item, prop.Items, errors)
		}
	}
}

// validateStringProperty validates a string against constraints
func validateStringProperty(path string, value interface{}, prop *Property, errors *SchemaValidationErrors) {
	str, ok := value.(string)
	if !ok {
		return
	}
	
	// Check length constraints
	if prop.MinLength != nil && len(str) < *prop.MinLength {
		errors.Add(path, fmt.Sprintf("string must be at least %d characters", *prop.MinLength), len(str), *prop.MinLength)
	}
	
	if prop.MaxLength != nil && len(str) > *prop.MaxLength {
		errors.Add(path, fmt.Sprintf("string must be at most %d characters", *prop.MaxLength), len(str), *prop.MaxLength)
	}
	
	// Check pattern constraint
	if prop.Pattern != "" {
		matched, err := regexp.MatchString(prop.Pattern, str)
		if err != nil {
			errors.Add(path, fmt.Sprintf("invalid regex pattern: %v", err), str, prop.Pattern)
		} else if !matched {
			errors.Add(path, fmt.Sprintf("string does not match pattern '%s'", prop.Pattern), str, prop.Pattern)
		}
	}
	
	// Check format constraint
	if prop.Format != "" {
		validateFormat(path, str, prop.Format, errors)
	}
}

// validateNumberProperty validates a number against constraints
func validateNumberProperty(path string, value interface{}, prop *Property, errors *SchemaValidationErrors) {
	var num float64
	
	switch v := value.(type) {
	case float64:
		num = v
	case float32:
		num = float64(v)
	case int:
		num = float64(v)
	case int32:
		num = float64(v)
	case int64:
		num = float64(v)
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			errors.Add(path, "invalid number format", value, "number")
			return
		}
		num = f
	default:
		return
	}
	
	// For integer type, check if it's a whole number
	if prop.Type == "integer" {
		if num != float64(int64(num)) {
			errors.Add(path, "expected integer value", num, "integer")
			return
		}
	}
	
	// Check minimum constraint
	if prop.Minimum != nil {
		if num < *prop.Minimum {
			errors.Add(path, fmt.Sprintf("value must be at least %v", *prop.Minimum), num, *prop.Minimum)
		}
	}
	
	// Check maximum constraint
	if prop.Maximum != nil {
		if num > *prop.Maximum {
			errors.Add(path, fmt.Sprintf("value must be at most %v", *prop.Maximum), num, *prop.Maximum)
		}
	}
}

// validateFormat validates string formats
func validateFormat(path, value, format string, errors *SchemaValidationErrors) {
	var valid bool
	var message string
	
	switch format {
	case "date-time":
		// RFC3339 datetime format
		pattern := `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})$`
		valid, _ = regexp.MatchString(pattern, value)
		message = "invalid date-time format (expected RFC3339)"
		
	case "date":
		// Date format: YYYY-MM-DD
		pattern := `^\d{4}-\d{2}-\d{2}$`
		valid, _ = regexp.MatchString(pattern, value)
		message = "invalid date format (expected YYYY-MM-DD)"
		
	case "time":
		// Time format: HH:MM:SS
		pattern := `^\d{2}:\d{2}:\d{2}$`
		valid, _ = regexp.MatchString(pattern, value)
		message = "invalid time format (expected HH:MM:SS)"
		
	case "email":
		// Simple email validation
		pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
		valid, _ = regexp.MatchString(pattern, value)
		message = "invalid email format"
		
	case "uri", "url":
		// Simple URI validation
		pattern := `^(https?|ftp)://[^\s/$.?#].[^\s]*$`
		valid, _ = regexp.MatchString(pattern, value)
		message = "invalid URI format"
		
	case "uuid":
		// UUID v4 format
		pattern := `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
		valid, _ = regexp.MatchString(pattern, strings.ToLower(value))
		message = "invalid UUID format"
		
	case "ipv4":
		pattern := `^(\d{1,3}\.){3}\d{1,3}$`
		valid, _ = regexp.MatchString(pattern, value)
		if valid {
			// Additional check for valid IP ranges
			parts := strings.Split(value, ".")
			for _, part := range parts {
				var num int
				fmt.Sscanf(part, "%d", &num)
				if num > 255 {
					valid = false
					break
				}
			}
		}
		message = "invalid IPv4 address"
		
	case "ipv6":
		pattern := `^([0-9a-fA-F]{0,4}:){7}[0-9a-fA-F]{0,4}$`
		valid, _ = regexp.MatchString(pattern, value)
		message = "invalid IPv6 address"
		
	default:
		// Unknown format, skip validation
		valid = true
	}
	
	if !valid {
		errors.Add(path, message, value, format)
	}
}

// Helper functions

// getJSONType returns the JSON type of a value
func getJSONType(value interface{}) string {
	if value == nil {
		return "null"
	}
	
	switch value.(type) {
	case bool:
		return "boolean"
	case float64, float32, int, int32, int64, json.Number:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}

// matchesType checks if an actual type matches an expected type
func matchesType(actual, expected string) bool {
	if actual == expected {
		return true
	}
	// Special case: integer is a subset of number
	if expected == "integer" && actual == "number" {
		return true
	}
	return false
}

// containsValue checks if a value is in a slice
func containsValue(slice []interface{}, value interface{}) bool {
	for _, v := range slice {
		if reflect.DeepEqual(v, value) {
			return true
		}
	}
	return false
}

// joinSchemaPath joins path segments for schema validation
func joinSchemaPath(base, segment string) string {
	if base == "" {
		return segment
	}
	return base + "." + segment
}

// ValidateToolCall validates a tool call's arguments against the tool's schema
func ValidateToolCall(toolName string, args json.RawMessage, registry *Registry) error {
	if registry == nil {
		return fmt.Errorf("tool registry is nil")
	}
	
	// Get tool from registry
	tool, err := registry.Get(toolName)
	if err != nil || tool == nil {
		return fmt.Errorf("tool '%s' not found: %w", toolName, err)
	}
	
	// Validate arguments against tool schema
	if err := ValidateArguments(args, tool.Schema); err != nil {
		return fmt.Errorf("invalid arguments for tool '%s': %w", toolName, err)
	}
	
	return nil
}