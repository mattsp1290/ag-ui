package state

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// StateValidator validates state data against schemas and custom rules.
type StateValidator interface {
	// Validate checks if the given state data is valid
	Validate(state map[string]interface{}) (*ValidationResult, error)

	// ValidatePath validates a specific path in the state
	ValidatePath(state map[string]interface{}, path string) (*ValidationResult, error)

	// AddRule adds a custom validation rule
	AddRule(rule ValidationRule) error

	// RemoveRule removes a validation rule by ID
	RemoveRule(ruleID string) error

	// SetSchema sets the JSON schema for validation
	SetSchema(schema *StateSchema) error
}

// ValidationResult contains detailed validation results.
type ValidationResult struct {
	// Valid indicates if the validation passed
	Valid bool `json:"valid"`

	// Errors contains validation errors
	Errors []ValidationError `json:"errors,omitempty"`

	// Warnings contains non-fatal validation issues
	Warnings []ValidationWarning `json:"warnings,omitempty"`

	// Metadata contains additional validation information
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Timestamp indicates when the validation was performed
	Timestamp time.Time `json:"timestamp"`
}

// ValidationError represents a validation error.
type ValidationError struct {
	// Path is the JSON pointer path where the error occurred
	Path string `json:"path"`

	// Message describes the error
	Message string `json:"message"`

	// Code is a machine-readable error code
	Code string `json:"code"`

	// Details contains additional error information
	Details map[string]interface{} `json:"details,omitempty"`
}

// ValidationWarning represents a non-fatal validation issue.
type ValidationWarning struct {
	// Path is the JSON pointer path where the warning occurred
	Path string `json:"path"`

	// Message describes the warning
	Message string `json:"message"`

	// Code is a machine-readable warning code
	Code string `json:"code"`

	// Severity indicates the warning severity (low, medium, high)
	Severity string `json:"severity"`
}

// StateSchema defines the JSON Schema for state validation.
type StateSchema struct {
	// Type is typically "object" for state data
	Type string `json:"type"`

	// Properties defines the schema for each state property
	Properties map[string]*SchemaProperty `json:"properties,omitempty"`

	// Required lists mandatory properties
	Required []string `json:"required,omitempty"`

	// AdditionalProperties controls whether extra properties are allowed
	AdditionalProperties *bool `json:"additionalProperties,omitempty"`

	// PatternProperties defines schemas for properties matching patterns
	PatternProperties map[string]*SchemaProperty `json:"patternProperties,omitempty"`

	// Dependencies defines property dependencies
	Dependencies map[string]interface{} `json:"dependencies,omitempty"`
}

// SchemaProperty represents a property in the state schema.
type SchemaProperty struct {
	// Type defines the JSON type
	Type string `json:"type"`

	// Description explains the property
	Description string `json:"description,omitempty"`

	// Format provides additional type constraints
	Format string `json:"format,omitempty"`

	// Enum restricts values to a set
	Enum []interface{} `json:"enum,omitempty"`

	// Const requires an exact value
	Const interface{} `json:"const,omitempty"`

	// Default provides a default value
	Default interface{} `json:"default,omitempty"`

	// Numeric constraints
	Minimum          *float64 `json:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	MultipleOf       *float64 `json:"multipleOf,omitempty"`

	// String constraints
	MinLength *int   `json:"minLength,omitempty"`
	MaxLength *int   `json:"maxLength,omitempty"`
	Pattern   string `json:"pattern,omitempty"`

	// Array constraints
	MinItems    *int            `json:"minItems,omitempty"`
	MaxItems    *int            `json:"maxItems,omitempty"`
	UniqueItems *bool           `json:"uniqueItems,omitempty"`
	Items       *SchemaProperty `json:"items,omitempty"`

	// Object constraints
	MinProperties *int                       `json:"minProperties,omitempty"`
	MaxProperties *int                       `json:"maxProperties,omitempty"`
	Properties    map[string]*SchemaProperty `json:"properties,omitempty"`
	Required      []string                   `json:"required,omitempty"`

	// Conditional schema
	If   *SchemaProperty `json:"if,omitempty"`
	Then *SchemaProperty `json:"then,omitempty"`
	Else *SchemaProperty `json:"else,omitempty"`
}

// ValidationRule defines a custom validation rule.
type ValidationRule interface {
	// ID returns the unique identifier for this rule
	ID() string

	// Validate checks if the state meets the rule requirements
	Validate(state map[string]interface{}) []ValidationError

	// Description returns a human-readable description
	Description() string
}

// FuncValidationRule implements ValidationRule using a function.
type FuncValidationRule struct {
	id          string
	description string
	validateFn  func(map[string]interface{}) []ValidationError
}

// NewFuncValidationRule creates a new function-based validation rule.
func NewFuncValidationRule(id, description string, fn func(map[string]interface{}) []ValidationError) ValidationRule {
	return &FuncValidationRule{
		id:          id,
		description: description,
		validateFn:  fn,
	}
}

func (r *FuncValidationRule) ID() string {
	return r.id
}

func (r *FuncValidationRule) Description() string {
	return r.description
}

func (r *FuncValidationRule) Validate(state map[string]interface{}) []ValidationError {
	return r.validateFn(state)
}

// stateValidator implements StateValidator with JSON Schema and custom rules.
type stateValidator struct {
	schema *StateSchema
	rules  map[string]ValidationRule
}

// NewStateValidator creates a new state validator.
func NewStateValidator(schema *StateSchema) StateValidator {
	return &stateValidator{
		schema: schema,
		rules:  make(map[string]ValidationRule),
	}
}

// Validate validates the entire state.
func (v *stateValidator) Validate(state map[string]interface{}) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:     true,
		Errors:    []ValidationError{},
		Warnings:  []ValidationWarning{},
		Metadata:  make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	// Validate against schema
	if v.schema != nil {
		schemaErrors := v.validateAgainstSchema(state, v.schema, "")
		result.Errors = append(result.Errors, schemaErrors...)
	}

	// Apply custom rules
	for _, rule := range v.rules {
		ruleErrors := rule.Validate(state)
		result.Errors = append(result.Errors, ruleErrors...)
	}

	// Update validity
	result.Valid = len(result.Errors) == 0

	// Add metadata
	result.Metadata["schemaValidation"] = v.schema != nil
	result.Metadata["customRules"] = len(v.rules)
	result.Metadata["errorCount"] = len(result.Errors)
	result.Metadata["warningCount"] = len(result.Warnings)

	return result, nil
}

// ValidatePath validates a specific path in the state.
func (v *stateValidator) ValidatePath(state map[string]interface{}, path string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:     true,
		Errors:    []ValidationError{},
		Warnings:  []ValidationWarning{},
		Metadata:  make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	// Get value at path
	value, err := getValueAtPath(state, path)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("path not found: %v", err),
			Code:    "PATH_NOT_FOUND",
		})
		return result, nil
	}

	// Find schema for path
	if v.schema != nil {
		prop := v.findSchemaForPath(path)
		if prop != nil {
			errors := v.validateValue(value, prop, path)
			result.Errors = append(result.Errors, errors...)
		}
	}

	// Update validity
	result.Valid = len(result.Errors) == 0
	result.Metadata["path"] = path
	result.Metadata["valueType"] = fmt.Sprintf("%T", value)

	return result, nil
}

// AddRule adds a custom validation rule.
func (v *stateValidator) AddRule(rule ValidationRule) error {
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}
	if rule.ID() == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}
	v.rules[rule.ID()] = rule
	return nil
}

// RemoveRule removes a validation rule by ID.
func (v *stateValidator) RemoveRule(ruleID string) error {
	if _, exists := v.rules[ruleID]; !exists {
		return fmt.Errorf("rule with ID %s not found", ruleID)
	}
	delete(v.rules, ruleID)
	return nil
}

// SetSchema sets the JSON schema for validation.
func (v *stateValidator) SetSchema(schema *StateSchema) error {
	if schema == nil {
		return fmt.Errorf("schema cannot be nil")
	}
	v.schema = schema
	return nil
}

// validateAgainstSchema validates state against a schema.
func (v *stateValidator) validateAgainstSchema(value interface{}, schema *StateSchema, path string) []ValidationError {
	errors := []ValidationError{}

	// Type check
	if schema.Type != "" {
		if !v.checkType(value, schema.Type) {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("expected type %s, got %T", schema.Type, value),
				Code:    "TYPE_MISMATCH",
			})
			return errors
		}
	}

	// Object validation
	if obj, ok := value.(map[string]interface{}); ok {
		// Check required properties
		for _, req := range schema.Required {
			if _, exists := obj[req]; !exists {
				errors = append(errors, ValidationError{
					Path:    joinPath(path, req),
					Message: "required property missing",
					Code:    "REQUIRED_PROPERTY_MISSING",
				})
			}
		}

		// Validate properties
		for name, propValue := range obj {
			propPath := joinPath(path, name)

			// Check if property is defined in schema
			if prop, exists := schema.Properties[name]; exists {
				propErrors := v.validateValue(propValue, prop, propPath)
				errors = append(errors, propErrors...)
			} else if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
				errors = append(errors, ValidationError{
					Path:    propPath,
					Message: "additional property not allowed",
					Code:    "ADDITIONAL_PROPERTY_NOT_ALLOWED",
				})
			} else {
				// Check pattern properties
				for pattern, patternProp := range schema.PatternProperties {
					if matched, _ := regexp.MatchString(pattern, name); matched {
						propErrors := v.validateValue(propValue, patternProp, propPath)
						errors = append(errors, propErrors...)
						break
					}
				}
			}
		}

		// Check property count constraints
		propCount := len(obj)
		if schema.Properties != nil {
			if minProps := getMinProperties(schema.Properties); propCount < minProps {
				errors = append(errors, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("object has %d properties, minimum %d required", propCount, minProps),
					Code:    "MIN_PROPERTIES_VIOLATION",
				})
			}
		}
	}

	return errors
}

// validateValue validates a single value against a property schema.
func (v *stateValidator) validateValue(value interface{}, prop *SchemaProperty, path string) []ValidationError {
	errors := []ValidationError{}

	// Null check
	if value == nil {
		if prop.Type != "null" {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: "value cannot be null",
				Code:    "NULL_VALUE",
			})
		}
		return errors
	}

	// Type validation
	if !v.checkType(value, prop.Type) {
		errors = append(errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("expected type %s, got %T", prop.Type, value),
			Code:    "TYPE_MISMATCH",
		})
		return errors
	}

	// Type-specific validation
	switch prop.Type {
	case "string":
		errors = append(errors, v.validateString(value.(string), prop, path)...)
	case "number", "integer":
		errors = append(errors, v.validateNumber(value, prop, path)...)
	case "boolean":
		// Boolean has no additional constraints
	case "array":
		errors = append(errors, v.validateArray(value, prop, path)...)
	case "object":
		if obj, ok := value.(map[string]interface{}); ok {
			objSchema := &StateSchema{
				Type:       "object",
				Properties: prop.Properties,
				Required:   prop.Required,
			}
			errors = append(errors, v.validateAgainstSchema(obj, objSchema, path)...)
		}
	}

	// Enum validation
	if len(prop.Enum) > 0 {
		if !v.isInEnum(value, prop.Enum) {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("value %v not in enum %v", value, prop.Enum),
				Code:    "ENUM_VIOLATION",
			})
		}
	}

	// Const validation
	if prop.Const != nil && !reflect.DeepEqual(value, prop.Const) {
		errors = append(errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("value must be %v", prop.Const),
			Code:    "CONST_VIOLATION",
		})
	}

	// Conditional validation
	if prop.If != nil {
		ifErrors := v.validateValue(value, prop.If, path)
		if len(ifErrors) == 0 && prop.Then != nil {
			// If condition matches, apply Then schema
			errors = append(errors, v.validateValue(value, prop.Then, path)...)
		} else if len(ifErrors) > 0 && prop.Else != nil {
			// If condition doesn't match, apply Else schema
			errors = append(errors, v.validateValue(value, prop.Else, path)...)
		}
	}

	return errors
}

// validateString validates string constraints.
func (v *stateValidator) validateString(str string, prop *SchemaProperty, path string) []ValidationError {
	errors := []ValidationError{}

	// Length constraints
	if prop.MinLength != nil && len(str) < *prop.MinLength {
		errors = append(errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("string length %d is less than minimum %d", len(str), *prop.MinLength),
			Code:    "MIN_LENGTH_VIOLATION",
		})
	}
	if prop.MaxLength != nil && len(str) > *prop.MaxLength {
		errors = append(errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("string length %d exceeds maximum %d", len(str), *prop.MaxLength),
			Code:    "MAX_LENGTH_VIOLATION",
		})
	}

	// Pattern validation
	if prop.Pattern != "" {
		if matched, err := regexp.MatchString(prop.Pattern, str); err != nil || !matched {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("string does not match pattern %s", prop.Pattern),
				Code:    "PATTERN_VIOLATION",
			})
		}
	}

	// Format validation
	if prop.Format != "" {
		if err := v.validateFormat(str, prop.Format); err != nil {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: err.Error(),
				Code:    "FORMAT_VIOLATION",
			})
		}
	}

	return errors
}

// validateNumber validates numeric constraints.
func (v *stateValidator) validateNumber(value interface{}, prop *SchemaProperty, path string) []ValidationError {
	errors := []ValidationError{}

	var num float64
	switch val := value.(type) {
	case float64:
		num = val
	case float32:
		num = float64(val)
	case int:
		num = float64(val)
	case int64:
		num = float64(val)
	case json.Number:
		f, _ := val.Float64()
		num = f
	}

	// Range constraints
	if prop.Minimum != nil && num < *prop.Minimum {
		errors = append(errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("value %v is less than minimum %v", num, *prop.Minimum),
			Code:    "MIN_VALUE_VIOLATION",
		})
	}
	if prop.Maximum != nil && num > *prop.Maximum {
		errors = append(errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("value %v exceeds maximum %v", num, *prop.Maximum),
			Code:    "MAX_VALUE_VIOLATION",
		})
	}
	if prop.ExclusiveMinimum != nil && num <= *prop.ExclusiveMinimum {
		errors = append(errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("value %v must be greater than %v", num, *prop.ExclusiveMinimum),
			Code:    "EXCLUSIVE_MIN_VIOLATION",
		})
	}
	if prop.ExclusiveMaximum != nil && num >= *prop.ExclusiveMaximum {
		errors = append(errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("value %v must be less than %v", num, *prop.ExclusiveMaximum),
			Code:    "EXCLUSIVE_MAX_VIOLATION",
		})
	}

	// Multiple constraint
	if prop.MultipleOf != nil && *prop.MultipleOf != 0 {
		if remainder := num / *prop.MultipleOf; remainder != float64(int64(remainder)) {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("value %v is not a multiple of %v", num, *prop.MultipleOf),
				Code:    "MULTIPLE_OF_VIOLATION",
			})
		}
	}

	return errors
}

// validateArray validates array constraints.
func (v *stateValidator) validateArray(value interface{}, prop *SchemaProperty, path string) []ValidationError {
	errors := []ValidationError{}

	arr, ok := value.([]interface{})
	if !ok {
		return errors
	}

	// Length constraints
	if prop.MinItems != nil && len(arr) < *prop.MinItems {
		errors = append(errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("array length %d is less than minimum %d", len(arr), *prop.MinItems),
			Code:    "MIN_ITEMS_VIOLATION",
		})
	}
	if prop.MaxItems != nil && len(arr) > *prop.MaxItems {
		errors = append(errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("array length %d exceeds maximum %d", len(arr), *prop.MaxItems),
			Code:    "MAX_ITEMS_VIOLATION",
		})
	}

	// Unique items constraint
	if prop.UniqueItems != nil && *prop.UniqueItems {
		seen := make(map[string]bool)
		for i, item := range arr {
			key := fmt.Sprintf("%v", item)
			if seen[key] {
				errors = append(errors, ValidationError{
					Path:    fmt.Sprintf("%s[%d]", path, i),
					Message: "duplicate item in array",
					Code:    "UNIQUE_ITEMS_VIOLATION",
				})
			}
			seen[key] = true
		}
	}

	// Item validation
	if prop.Items != nil {
		for i, item := range arr {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			itemErrors := v.validateValue(item, prop.Items, itemPath)
			errors = append(errors, itemErrors...)
		}
	}

	return errors
}

// checkType checks if a value matches the expected type.
func (v *stateValidator) checkType(value interface{}, expectedType string) bool {
	switch expectedType {
	case "null":
		return value == nil
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "number":
		switch value.(type) {
		case float64, float32, int, int32, int64, json.Number:
			return true
		}
		return false
	case "integer":
		switch val := value.(type) {
		case int, int32, int64:
			return true
		case float64:
			return val == float64(int64(val))
		case json.Number:
			_, err := val.Int64()
			return err == nil
		}
		return false
	case "string":
		_, ok := value.(string)
		return ok
	}
	return false
}

// isInEnum checks if a value is in the enum list.
func (v *stateValidator) isInEnum(value interface{}, enum []interface{}) bool {
	for _, allowed := range enum {
		if reflect.DeepEqual(value, allowed) {
			return true
		}
	}
	return false
}

// validateFormat validates string formats.
func (v *stateValidator) validateFormat(value string, format string) error {
	switch format {
	case "date-time":
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return fmt.Errorf("invalid date-time format")
		}
	case "date":
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return fmt.Errorf("invalid date format")
		}
	case "time":
		if _, err := time.Parse("15:04:05", value); err != nil {
			return fmt.Errorf("invalid time format")
		}
	case "email":
		if !strings.Contains(value, "@") || !strings.Contains(value, ".") {
			return fmt.Errorf("invalid email format")
		}
	case "uri", "url":
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			return fmt.Errorf("invalid URI format")
		}
	case "uuid":
		if matched, _ := regexp.MatchString(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`, strings.ToLower(value)); !matched {
			return fmt.Errorf("invalid UUID format")
		}
	}
	return nil
}

// findSchemaForPath finds the schema property for a given path.
func (v *stateValidator) findSchemaForPath(path string) *SchemaProperty {
	if v.schema == nil || path == "" || path == "/" {
		return nil
	}

	tokens := parseJSONPointer(path)
	current := v.schema.Properties

	for i, token := range tokens {
		if current == nil {
			return nil
		}

		prop, exists := current[token]
		if !exists {
			return nil
		}

		if i == len(tokens)-1 {
			return prop
		}

		current = prop.Properties
	}

	return nil
}

// getMinProperties calculates minimum required properties.
func getMinProperties(props map[string]*SchemaProperty) int {
	count := 0
	for _, prop := range props {
		if prop.MinProperties != nil {
			count += *prop.MinProperties
		}
	}
	return count
}

// joinPath joins path segments.
func joinPath(base, segment string) string {
	if base == "" || base == "/" {
		return "/" + segment
	}
	return base + "/" + segment
}

// Common validation rules

// NewRequiredFieldsRule creates a rule that ensures required fields exist.
func NewRequiredFieldsRule(fields []string) ValidationRule {
	return NewFuncValidationRule(
		"required-fields",
		"Ensures required fields exist in state",
		func(state map[string]interface{}) []ValidationError {
			errors := []ValidationError{}
			for _, field := range fields {
				if _, err := getValueAtPath(state, field); err != nil {
					errors = append(errors, ValidationError{
						Path:    field,
						Message: "required field missing",
						Code:    "REQUIRED_FIELD_MISSING",
					})
				}
			}
			return errors
		},
	)
}

// NewMaxDepthRule creates a rule that limits state nesting depth.
func NewMaxDepthRule(maxDepth int) ValidationRule {
	return NewFuncValidationRule(
		"max-depth",
		fmt.Sprintf("Limits state nesting to %d levels", maxDepth),
		func(state map[string]interface{}) []ValidationError {
			errors := []ValidationError{}
			checkDepth(state, "", 0, maxDepth, &errors)
			return errors
		},
	)
}

// checkDepth recursively checks nesting depth.
func checkDepth(value interface{}, path string, currentDepth, maxDepth int, errors *[]ValidationError) {
	if currentDepth > maxDepth {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("nesting depth %d exceeds maximum %d", currentDepth, maxDepth),
			Code:    "MAX_DEPTH_EXCEEDED",
		})
		return
	}

	switch v := value.(type) {
	case map[string]interface{}:
		for key, val := range v {
			checkDepth(val, joinPath(path, key), currentDepth+1, maxDepth, errors)
		}
	case []interface{}:
		for i, val := range v {
			checkDepth(val, fmt.Sprintf("%s[%d]", path, i), currentDepth+1, maxDepth, errors)
		}
	}
}
