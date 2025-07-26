package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SchemaType represents the type of a schema property
type SchemaType string

const (
	SchemaTypeString  SchemaType = "string"
	SchemaTypeNumber  SchemaType = "number"
	SchemaTypeInteger SchemaType = "integer"
	SchemaTypeBoolean SchemaType = "boolean"
	SchemaTypeArray   SchemaType = "array"
	SchemaTypeObject  SchemaType = "object"
	SchemaTypeNull    SchemaType = "null"
	SchemaTypeAny     SchemaType = "any"
)

// Schema represents a validation schema
type Schema struct {
	// Type specifies the expected data type
	Type SchemaType `json:"type,omitempty"`
	
	// Title provides a human-readable title for the schema
	Title string `json:"title,omitempty"`
	
	// Description provides additional documentation
	Description string `json:"description,omitempty"`
	
	// Required specifies which properties are required (for object types)
	Required []string `json:"required,omitempty"`
	
	// Properties defines the schema for object properties
	Properties map[string]*Schema `json:"properties,omitempty"`
	
	// AdditionalProperties controls whether additional properties are allowed
	AdditionalProperties interface{} `json:"additionalProperties,omitempty"`
	
	// Items defines the schema for array items
	Items *Schema `json:"items,omitempty"`
	
	// MinItems specifies the minimum number of items in an array
	MinItems *int `json:"minItems,omitempty"`
	
	// MaxItems specifies the maximum number of items in an array
	MaxItems *int `json:"maxItems,omitempty"`
	
	// UniqueItems requires all items in an array to be unique
	UniqueItems bool `json:"uniqueItems,omitempty"`
	
	// MinLength specifies the minimum length for strings
	MinLength *int `json:"minLength,omitempty"`
	
	// MaxLength specifies the maximum length for strings
	MaxLength *int `json:"maxLength,omitempty"`
	
	// Pattern specifies a regex pattern for string validation
	Pattern string `json:"pattern,omitempty"`
	
	// Format specifies a semantic format for string validation
	Format string `json:"format,omitempty"`
	
	// Minimum specifies the minimum value for numbers
	Minimum *float64 `json:"minimum,omitempty"`
	
	// Maximum specifies the maximum value for numbers
	Maximum *float64 `json:"maximum,omitempty"`
	
	// ExclusiveMinimum specifies whether the minimum is exclusive
	ExclusiveMinimum bool `json:"exclusiveMinimum,omitempty"`
	
	// ExclusiveMaximum specifies whether the maximum is exclusive
	ExclusiveMaximum bool `json:"exclusiveMaximum,omitempty"`
	
	// MultipleOf specifies that a number must be a multiple of this value
	MultipleOf *float64 `json:"multipleOf,omitempty"`
	
	// Enum specifies a list of valid values
	Enum []interface{} `json:"enum,omitempty"`
	
	// Const specifies a single valid value
	Const interface{} `json:"const,omitempty"`
	
	// AllOf requires the value to be valid against all schemas in the array
	AllOf []*Schema `json:"allOf,omitempty"`
	
	// AnyOf requires the value to be valid against any schema in the array
	AnyOf []*Schema `json:"anyOf,omitempty"`
	
	// OneOf requires the value to be valid against exactly one schema in the array
	OneOf []*Schema `json:"oneOf,omitempty"`
	
	// Not requires the value to NOT be valid against the schema
	Not *Schema `json:"not,omitempty"`
	
	// Default provides a default value
	Default interface{} `json:"default,omitempty"`
	
	// Examples provides example values
	Examples []interface{} `json:"examples,omitempty"`
	
	// Custom validation function
	CustomValidator func(interface{}) error `json:"-"`
	
	// Version specifies the schema version for migration support
	Version string `json:"version,omitempty"`
	
	// Deprecated marks the schema as deprecated
	Deprecated bool `json:"deprecated,omitempty"`
	
	// ID provides a unique identifier for the schema
	ID string `json:"$id,omitempty"`
	
	// Ref allows referencing other schemas
	Ref string `json:"$ref,omitempty"`
	
	// Definitions allows defining reusable schema components
	Definitions map[string]*Schema `json:"definitions,omitempty"`
}

// NewSchema creates a new schema with the specified type
func NewSchema(schemaType SchemaType) *Schema {
	return &Schema{
		Type:        schemaType,
		Properties:  make(map[string]*Schema),
		Definitions: make(map[string]*Schema),
	}
}

// NewObjectSchema creates a new object schema
func NewObjectSchema() *Schema {
	return NewSchema(SchemaTypeObject)
}

// NewArraySchema creates a new array schema
func NewArraySchema(itemSchema *Schema) *Schema {
	schema := NewSchema(SchemaTypeArray)
	schema.Items = itemSchema
	return schema
}

// NewStringSchema creates a new string schema
func NewStringSchema() *Schema {
	return NewSchema(SchemaTypeString)
}

// AddProperty adds a property to an object schema
func (s *Schema) AddProperty(name string, propertySchema *Schema) *Schema {
	if s.Properties == nil {
		s.Properties = make(map[string]*Schema)
	}
	s.Properties[name] = propertySchema
	return s
}

// SetRequired marks a property as required
func (s *Schema) SetRequired(property string) *Schema {
	for _, req := range s.Required {
		if req == property {
			return s // Already required
		}
	}
	s.Required = append(s.Required, property)
	return s
}

// SetTitle sets the schema title
func (s *Schema) SetTitle(title string) *Schema {
	s.Title = title
	return s
}

// SetDescription sets the schema description
func (s *Schema) SetDescription(description string) *Schema {
	s.Description = description
	return s
}

// SetPattern sets a regex pattern for string validation
func (s *Schema) SetPattern(pattern string) *Schema {
	s.Pattern = pattern
	return s
}

// SetFormat sets a semantic format for string validation
func (s *Schema) SetFormat(format string) *Schema {
	s.Format = format
	return s
}

// SetLengthRange sets min/max length for strings or arrays
func (s *Schema) SetLengthRange(min, max *int) *Schema {
	if s.Type == SchemaTypeString {
		s.MinLength = min
		s.MaxLength = max
	} else if s.Type == SchemaTypeArray {
		s.MinItems = min
		s.MaxItems = max
	}
	return s
}

// SetNumberRange sets min/max values for numbers
func (s *Schema) SetNumberRange(min, max *float64, exclusiveMin, exclusiveMax bool) *Schema {
	s.Minimum = min
	s.Maximum = max
	s.ExclusiveMinimum = exclusiveMin
	s.ExclusiveMaximum = exclusiveMax
	return s
}

// SetEnum sets allowed values
func (s *Schema) SetEnum(values ...interface{}) *Schema {
	s.Enum = values
	return s
}

// SetDefault sets a default value
func (s *Schema) SetDefault(value interface{}) *Schema {
	s.Default = value
	return s
}

// AddDefinition adds a reusable schema definition
func (s *Schema) AddDefinition(name string, definition *Schema) *Schema {
	if s.Definitions == nil {
		s.Definitions = make(map[string]*Schema)
	}
	s.Definitions[name] = definition
	return s
}

// SchemaValidator validates data against JSON-like schemas
type SchemaValidator struct {
	name               string
	schema             *Schema
	schemaRegistry     map[string]*Schema
	formatValidators   map[string]func(string) bool
	customValidators   map[string]func(interface{}) error
	strictMode         bool
	allowUndefined     bool
	validationOptions  SchemaValidationOptions
	enabled            bool
	priority           int
	mutex              sync.RWMutex
}

// SchemaValidationOptions configures schema validation behavior
type SchemaValidationOptions struct {
	// ValidateFormats enables format validation for string types
	ValidateFormats bool
	
	// StrictTypes requires exact type matches
	StrictTypes bool
	
	// AllowCoercion enables type coercion (e.g., string "123" to number 123)
	AllowCoercion bool
	
	// CollectAllErrors continues validation even after finding errors
	CollectAllErrors bool
	
	// MaxErrorCount limits the number of errors to collect
	MaxErrorCount int
	
	// ValidateDefaults validates default values against their schemas
	ValidateDefaults bool
	
	// ReportDeprecated reports usage of deprecated schema elements
	ReportDeprecated bool
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator(name string, schema *Schema) *SchemaValidator {
	return &SchemaValidator{
		name:             name,
		schema:           schema,
		schemaRegistry:   make(map[string]*Schema),
		formatValidators: getDefaultFormatValidators(),
		customValidators: make(map[string]func(interface{}) error),
		strictMode:       true,
		allowUndefined:   false,
		validationOptions: SchemaValidationOptions{
			ValidateFormats:   true,
			StrictTypes:       true,
			AllowCoercion:     false,
			CollectAllErrors:  true,
			MaxErrorCount:     100,
			ValidateDefaults:  true,
			ReportDeprecated:  true,
		},
		enabled:  true,
		priority: 50,
	}
}

// SetSchema updates the schema
func (v *SchemaValidator) SetSchema(schema *Schema) *SchemaValidator {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.schema = schema
	return v
}

// RegisterSchema registers a schema in the registry for reference resolution
func (v *SchemaValidator) RegisterSchema(id string, schema *Schema) *SchemaValidator {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.schemaRegistry[id] = schema
	return v
}

// SetOptions sets validation options
func (v *SchemaValidator) SetOptions(options SchemaValidationOptions) *SchemaValidator {
	v.validationOptions = options
	return v
}

// AddFormatValidator adds a custom format validator
func (v *SchemaValidator) AddFormatValidator(format string, validator func(string) bool) *SchemaValidator {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.formatValidators[format] = validator
	return v
}

// AddCustomValidator adds a custom validation function
func (v *SchemaValidator) AddCustomValidator(name string, validator func(interface{}) error) *SchemaValidator {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.customValidators[name] = validator
	return v
}

// SetStrictMode enables or disables strict validation mode
func (v *SchemaValidator) SetStrictMode(strict bool) *SchemaValidator {
	v.strictMode = strict
	return v
}

// SetEnabled enables or disables the validator
func (v *SchemaValidator) SetEnabled(enabled bool) *SchemaValidator {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *SchemaValidator) SetPriority(priority int) *SchemaValidator {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *SchemaValidator) Name() string {
	return fmt.Sprintf("schema_%s", v.name)
}

// Validate validates a value against the schema
func (v *SchemaValidator) Validate(ctx context.Context, value interface{}) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	v.mutex.RLock()
	defer v.mutex.RUnlock()

	result := NewValidationResult(true)
	
	// Validate against the schema
	v.validateValue(ctx, value, v.schema, "", &result)
	
	return result
}

// validateValue performs the actual validation against a schema
func (v *SchemaValidator) validateValue(ctx context.Context, value interface{}, schema *Schema, path string, result *ValidationResult) {
	// Check if we've hit the error limit
	if v.validationOptions.MaxErrorCount > 0 && len(result.Issues()) >= v.validationOptions.MaxErrorCount {
		return
	}

	// Handle schema references
	if schema.Ref != "" {
		refSchema := v.resolveReference(schema.Ref)
		if refSchema != nil {
			v.validateValue(ctx, value, refSchema, path, result)
			return
		} else {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("unresolved schema reference: %s", schema.Ref), nil))
			return
		}
	}

	// Report deprecated usage
	if v.validationOptions.ReportDeprecated && schema.Deprecated {
		issue := NewValidationIssue(fmt.Sprintf("using deprecated schema at path '%s'", path), SeverityWarning)
		issue.WithField(path).WithCategory("deprecation")
		result.AddIssue(issue)
	}

	// Handle null values
	if value == nil {
		if schema.Type == SchemaTypeNull {
			return // Valid null
		}
		// Check if null is allowed through anyOf, oneOf, etc.
		if v.isNullAllowed(schema) {
			return
		}
		result.AddFieldError(path, NewValidationError("value cannot be null", nil))
		return
	}

	// Validate const
	if schema.Const != nil {
		if !v.valuesEqual(value, schema.Const) {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("value must be %v", schema.Const), nil))
			return
		}
	}

	// Validate enum
	if len(schema.Enum) > 0 {
		valid := false
		for _, enumValue := range schema.Enum {
			if v.valuesEqual(value, enumValue) {
				valid = true
				break
			}
		}
		if !valid {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("value must be one of %v", schema.Enum), nil))
			return
		}
	}

	// Type coercion if enabled
	if v.validationOptions.AllowCoercion {
		value = v.coerceValue(value, schema.Type)
	}

	// Validate type
	if schema.Type != "" && schema.Type != SchemaTypeAny {
		if !v.validateType(value, schema.Type) {
			expectedType := string(schema.Type)
			actualType := v.getValueType(value)
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("expected type %s, got %s", expectedType, actualType), nil))
			if !v.validationOptions.CollectAllErrors {
				return
			}
		}
	}

	// Type-specific validation
	switch schema.Type {
	case SchemaTypeString:
		v.validateString(value, schema, path, result)
	case SchemaTypeNumber, SchemaTypeInteger:
		v.validateNumber(value, schema, path, result)
	case SchemaTypeArray:
		v.validateArray(ctx, value, schema, path, result)
	case SchemaTypeObject:
		v.validateObject(ctx, value, schema, path, result)
	case SchemaTypeBoolean:
		// Boolean validation is handled by type validation
	}

	// Validate combinators
	v.validateCombinators(ctx, value, schema, path, result)

	// Apply custom validator if present
	if schema.CustomValidator != nil {
		if err := schema.CustomValidator(value); err != nil {
			result.AddFieldError(path, err)
		}
	}
}

// validateString validates string-specific constraints
func (v *SchemaValidator) validateString(value interface{}, schema *Schema, path string, result *ValidationResult) {
	str, ok := value.(string)
	if !ok {
		return // Type validation should have caught this
	}

	// Length validation
	if schema.MinLength != nil && len(str) < *schema.MinLength {
		result.AddFieldError(path, NewValidationError(fmt.Sprintf("string length %d is less than minimum %d", len(str), *schema.MinLength), nil))
	}
	if schema.MaxLength != nil && len(str) > *schema.MaxLength {
		result.AddFieldError(path, NewValidationError(fmt.Sprintf("string length %d exceeds maximum %d", len(str), *schema.MaxLength), nil))
	}

	// Pattern validation
	if schema.Pattern != "" {
		if validator := NewPatternValidator("schema_pattern"); validator != nil {
			if err := validator.AddPattern("main", schema.Pattern, true); err == nil {
				patternResult := validator.Validate(context.Background(), str)
				if !patternResult.IsValid() {
					for _, err := range patternResult.Errors() {
						result.AddFieldError(path, err)
					}
				}
			}
		}
	}

	// Format validation
	if v.validationOptions.ValidateFormats && schema.Format != "" {
		if formatValidator, exists := v.formatValidators[schema.Format]; exists {
			if !formatValidator(str) {
				result.AddFieldError(path, NewValidationError(fmt.Sprintf("string does not match format '%s'", schema.Format), nil))
			}
		}
	}
}

// validateNumber validates number-specific constraints
func (v *SchemaValidator) validateNumber(value interface{}, schema *Schema, path string, result *ValidationResult) {
	var num float64
	var ok bool

	switch val := value.(type) {
	case float64:
		num, ok = val, true
	case float32:
		num, ok = float64(val), true
	case int:
		num, ok = float64(val), true
	case int32:
		num, ok = float64(val), true
	case int64:
		num, ok = float64(val), true
	}

	if !ok {
		return // Type validation should have caught this
	}

	// Integer type check
	if schema.Type == SchemaTypeInteger && num != float64(int64(num)) {
		result.AddFieldError(path, NewValidationError("value must be an integer", nil))
	}

	// Range validation
	if schema.Minimum != nil {
		if schema.ExclusiveMinimum && num <= *schema.Minimum {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("value %g must be greater than %g", num, *schema.Minimum), nil))
		} else if !schema.ExclusiveMinimum && num < *schema.Minimum {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("value %g is less than minimum %g", num, *schema.Minimum), nil))
		}
	}

	if schema.Maximum != nil {
		if schema.ExclusiveMaximum && num >= *schema.Maximum {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("value %g must be less than %g", num, *schema.Maximum), nil))
		} else if !schema.ExclusiveMaximum && num > *schema.Maximum {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("value %g exceeds maximum %g", num, *schema.Maximum), nil))
		}
	}

	// Multiple validation
	if schema.MultipleOf != nil && *schema.MultipleOf != 0 {
		if remainder := num / *schema.MultipleOf; remainder != float64(int64(remainder)) {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("value %g is not a multiple of %g", num, *schema.MultipleOf), nil))
		}
	}
}

// validateArray validates array-specific constraints
func (v *SchemaValidator) validateArray(ctx context.Context, value interface{}, schema *Schema, path string, result *ValidationResult) {
	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return // Type validation should have caught this
	}

	length := val.Len()

	// Length validation
	if schema.MinItems != nil && length < *schema.MinItems {
		result.AddFieldError(path, NewValidationError(fmt.Sprintf("array length %d is less than minimum %d", length, *schema.MinItems), nil))
	}
	if schema.MaxItems != nil && length > *schema.MaxItems {
		result.AddFieldError(path, NewValidationError(fmt.Sprintf("array length %d exceeds maximum %d", length, *schema.MaxItems), nil))
	}

	// Unique items validation
	if schema.UniqueItems {
		seen := make(map[interface{}]bool)
		for i := 0; i < length; i++ {
			item := val.Index(i).Interface()
			itemKey := v.getValueKey(item)
			if seen[itemKey] {
				result.AddFieldError(fmt.Sprintf("%s[%d]", path, i), NewValidationError("duplicate item in array that requires unique items", nil))
			}
			seen[itemKey] = true
		}
	}

	// Validate items
	if schema.Items != nil {
		for i := 0; i < length; i++ {
			item := val.Index(i).Interface()
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			v.validateValue(ctx, item, schema.Items, itemPath, result)
		}
	}
}

// validateObject validates object-specific constraints
func (v *SchemaValidator) validateObject(ctx context.Context, value interface{}, schema *Schema, path string, result *ValidationResult) {
	var objMap map[string]interface{}
	var ok bool

	// Handle different object types
	if objMap, ok = value.(map[string]interface{}); !ok {
		// Try to convert struct to map
		val := reflect.ValueOf(value)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() == reflect.Struct {
			objMap = v.structToMap(val)
		} else {
			return // Type validation should have caught this
		}
	}

	// Check required properties
	for _, required := range schema.Required {
		if _, exists := objMap[required]; !exists {
			fieldPath := v.buildPath(path, required)
			result.AddFieldError(fieldPath, NewValidationError(fmt.Sprintf("required property '%s' is missing", required), nil))
		}
	}

	// Validate properties
	if schema.Properties != nil {
		for propName, propValue := range objMap {
			fieldPath := v.buildPath(path, propName)
			
			if propSchema, exists := schema.Properties[propName]; exists {
				// Validate against defined property schema
				v.validateValue(ctx, propValue, propSchema, fieldPath, result)
			} else {
				// Handle additional properties
				v.validateAdditionalProperty(ctx, propName, propValue, schema, fieldPath, result)
			}
		}
	}

	// Validate missing properties that have defaults
	if v.validationOptions.ValidateDefaults && schema.Properties != nil {
		for propName, propSchema := range schema.Properties {
			if _, exists := objMap[propName]; !exists && propSchema.Default != nil {
				fieldPath := v.buildPath(path, propName)
				v.validateValue(ctx, propSchema.Default, propSchema, fieldPath, result)
			}
		}
	}
}

// validateAdditionalProperty handles validation of additional properties
func (v *SchemaValidator) validateAdditionalProperty(ctx context.Context, propName string, propValue interface{}, schema *Schema, path string, result *ValidationResult) {
	switch addProps := schema.AdditionalProperties.(type) {
	case bool:
		if !addProps {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("additional property '%s' is not allowed", propName), nil))
		}
	case *Schema:
		// Validate against additional properties schema
		v.validateValue(ctx, propValue, addProps, path, result)
	case map[string]interface{}:
		// Convert map to schema and validate
		if addPropsSchema, err := v.mapToSchema(addProps); err == nil {
			v.validateValue(ctx, propValue, addPropsSchema, path, result)
		}
	default:
		// Default behavior: allow additional properties
	}
}

// validateCombinators validates schema combinators (allOf, anyOf, oneOf, not)
func (v *SchemaValidator) validateCombinators(ctx context.Context, value interface{}, schema *Schema, path string, result *ValidationResult) {
	// AllOf validation
	if len(schema.AllOf) > 0 {
		for i, subSchema := range schema.AllOf {
			subResult := NewValidationResult(true)
			v.validateValue(ctx, value, subSchema, path, &subResult)
			if !subResult.IsValid() {
				for _, err := range subResult.Errors() {
					result.AddFieldError(path, NewValidationError(fmt.Sprintf("allOf[%d]: %s", i, err.Error()), nil))
				}
			}
		}
	}

	// AnyOf validation
	if len(schema.AnyOf) > 0 {
		anyValid := false
		var anyOfErrors []string
		
		for i, subSchema := range schema.AnyOf {
			subResult := NewValidationResult(true)
			v.validateValue(ctx, value, subSchema, path, &subResult)
			if subResult.IsValid() {
				anyValid = true
				break
			} else {
				// Collect error for reporting
				for _, err := range subResult.Errors() {
					anyOfErrors = append(anyOfErrors, fmt.Sprintf("anyOf[%d]: %s", i, err.Error()))
				}
			}
		}
		
		if !anyValid {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("value does not match any schema in anyOf: %s", strings.Join(anyOfErrors, "; ")), nil))
		}
	}

	// OneOf validation
	if len(schema.OneOf) > 0 {
		validCount := 0
		var oneOfErrors []string
		
		for i, subSchema := range schema.OneOf {
			subResult := NewValidationResult(true)
			v.validateValue(ctx, value, subSchema, path, &subResult)
			if subResult.IsValid() {
				validCount++
			} else {
				for _, err := range subResult.Errors() {
					oneOfErrors = append(oneOfErrors, fmt.Sprintf("oneOf[%d]: %s", i, err.Error()))
				}
			}
		}
		
		if validCount == 0 {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("value does not match any schema in oneOf: %s", strings.Join(oneOfErrors, "; ")), nil))
		} else if validCount > 1 {
			result.AddFieldError(path, NewValidationError(fmt.Sprintf("value matches %d schemas in oneOf, but exactly 1 is required", validCount), nil))
		}
	}

	// Not validation
	if schema.Not != nil {
		subResult := NewValidationResult(true)
		v.validateValue(ctx, value, schema.Not, path, &subResult)
		if subResult.IsValid() {
			result.AddFieldError(path, NewValidationError("value must not match the 'not' schema", nil))
		}
	}
}

// Helper methods

// resolveReference resolves a schema reference
func (v *SchemaValidator) resolveReference(ref string) *Schema {
	// Simple reference resolution - in a full implementation,
	// this would handle JSON Pointer references and external schemas
	if strings.HasPrefix(ref, "#/definitions/") {
		defName := strings.TrimPrefix(ref, "#/definitions/")
		if v.schema.Definitions != nil {
			return v.schema.Definitions[defName]
		}
	}
	
	// Check schema registry
	return v.schemaRegistry[ref]
}

// isNullAllowed checks if null is allowed by the schema
func (v *SchemaValidator) isNullAllowed(schema *Schema) bool {
	// Check if null is explicitly allowed in anyOf or oneOf
	for _, subSchema := range schema.AnyOf {
		if subSchema.Type == SchemaTypeNull {
			return true
		}
	}
	for _, subSchema := range schema.OneOf {
		if subSchema.Type == SchemaTypeNull {
			return true
		}
	}
	return false
}

// validateType checks if a value matches the expected schema type
func (v *SchemaValidator) validateType(value interface{}, expectedType SchemaType) bool {
	actualType := v.getValueType(value)
	
	if v.validationOptions.StrictTypes {
		return actualType == expectedType
	}
	
	// Allow some flexibility for numbers
	if expectedType == SchemaTypeNumber && (actualType == SchemaTypeInteger || actualType == SchemaTypeNumber) {
		return true
	}
	
	return actualType == expectedType
}

// getValueType determines the schema type of a value
func (v *SchemaValidator) getValueType(value interface{}) SchemaType {
	if value == nil {
		return SchemaTypeNull
	}
	
	switch value.(type) {
	case bool:
		return SchemaTypeBoolean
	case string:
		return SchemaTypeString
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return SchemaTypeInteger
	case float32, float64:
		return SchemaTypeNumber
	case []interface{}, []string, []int, []float64: // Add more slice types as needed
		return SchemaTypeArray
	case map[string]interface{}:
		return SchemaTypeObject
	default:
		// Check if it's a slice or array using reflection
		val := reflect.ValueOf(value)
		switch val.Kind() {
		case reflect.Slice, reflect.Array:
			return SchemaTypeArray
		case reflect.Map, reflect.Struct:
			return SchemaTypeObject
		default:
			return SchemaTypeAny
		}
	}
}

// coerceValue attempts to coerce a value to the expected type
func (v *SchemaValidator) coerceValue(value interface{}, expectedType SchemaType) interface{} {
	if !v.validationOptions.AllowCoercion {
		return value
	}
	
	switch expectedType {
	case SchemaTypeString:
		return fmt.Sprintf("%v", value)
	case SchemaTypeNumber:
		if str, ok := value.(string); ok {
			if num, err := strconv.ParseFloat(str, 64); err == nil {
				return num
			}
		}
	case SchemaTypeInteger:
		if str, ok := value.(string); ok {
			if num, err := strconv.ParseInt(str, 10, 64); err == nil {
				return int(num)
			}
		}
		if num, ok := value.(float64); ok {
			return int(num)
		}
	case SchemaTypeBoolean:
		if str, ok := value.(string); ok {
			if b, err := strconv.ParseBool(str); err == nil {
				return b
			}
		}
	}
	
	return value
}

// valuesEqual compares two values for equality
func (v *SchemaValidator) valuesEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}

// getValueKey generates a key for value comparison in unique items validation
func (v *SchemaValidator) getValueKey(value interface{}) interface{} {
	// For simple types, return the value itself
	// For complex types, could use JSON serialization or other methods
	return value
}

// structToMap converts a struct to a map[string]interface{}
func (v *SchemaValidator) structToMap(val reflect.Value) map[string]interface{} {
	result := make(map[string]interface{})
	typ := val.Type()
	
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)
		
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}
		
		// Use JSON tag if available, otherwise use field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			if commaIdx := strings.Index(jsonTag, ","); commaIdx > 0 {
				fieldName = jsonTag[:commaIdx]
			} else {
				fieldName = jsonTag
			}
		}
		
		result[fieldName] = fieldValue.Interface()
	}
	
	return result
}

// mapToSchema converts a map to a schema (basic implementation)
func (v *SchemaValidator) mapToSchema(m map[string]interface{}) (*Schema, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	
	var schema Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	
	return &schema, nil
}

// buildPath builds a field path for error reporting
func (v *SchemaValidator) buildPath(basePath, field string) string {
	if basePath == "" {
		return field
	}
	return basePath + "." + field
}

// IsEnabled returns whether the validator is enabled
func (v *SchemaValidator) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *SchemaValidator) Priority() int {
	return v.priority
}

// getDefaultFormatValidators returns a map of default format validators
func getDefaultFormatValidators() map[string]func(string) bool {
	return map[string]func(string) bool{
		"email": func(s string) bool {
			// Simple email validation - in practice, use a proper email validation library
			return strings.Contains(s, "@") && strings.Contains(s, ".")
		},
		"uri": func(s string) bool {
			// Simple URI validation
			return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
		},
		"date": func(s string) bool {
			// ISO 8601 date format
			_, err := time.Parse("2006-01-02", s)
			return err == nil
		},
		"time": func(s string) bool {
			// ISO 8601 time format
			_, err := time.Parse("15:04:05", s)
			return err == nil
		},
		"date-time": func(s string) bool {
			// ISO 8601 date-time format
			_, err := time.Parse(time.RFC3339, s)
			return err == nil
		},
		"ipv4": func(s string) bool {
			// Simple IPv4 validation
			parts := strings.Split(s, ".")
			if len(parts) != 4 {
				return false
			}
			for _, part := range parts {
				if num, err := strconv.Atoi(part); err != nil || num < 0 || num > 255 {
					return false
				}
			}
			return true
		},
		"uuid": func(s string) bool {
			// Simple UUID validation
			return len(s) == 36 && strings.Count(s, "-") == 4
		},
	}
}

// Schema composition and inheritance

// SchemaComposer helps compose complex schemas from simpler ones
type SchemaComposer struct {
	baseSchema  *Schema
	mixins      []*Schema
	overrides   *Schema
}

// NewSchemaComposer creates a new schema composer
func NewSchemaComposer(baseSchema *Schema) *SchemaComposer {
	return &SchemaComposer{
		baseSchema: baseSchema,
		mixins:     make([]*Schema, 0),
	}
}

// AddMixin adds a schema mixin
func (c *SchemaComposer) AddMixin(mixin *Schema) *SchemaComposer {
	c.mixins = append(c.mixins, mixin)
	return c
}

// SetOverrides sets schema overrides
func (c *SchemaComposer) SetOverrides(overrides *Schema) *SchemaComposer {
	c.overrides = overrides
	return c
}

// Compose creates the final composed schema
func (c *SchemaComposer) Compose() *Schema {
	result := c.deepCopySchema(c.baseSchema)
	
	// Apply mixins
	for _, mixin := range c.mixins {
		c.mergeSchema(result, mixin)
	}
	
	// Apply overrides
	if c.overrides != nil {
		c.mergeSchema(result, c.overrides)
	}
	
	return result
}

// deepCopySchema creates a deep copy of a schema
func (c *SchemaComposer) deepCopySchema(schema *Schema) *Schema {
	// Simple implementation - in practice, might use a proper deep copy library
	data, _ := json.Marshal(schema)
	var copy Schema
	json.Unmarshal(data, &copy)
	return &copy
}

// mergeSchema merges source schema into target schema
func (c *SchemaComposer) mergeSchema(target, source *Schema) {
	// Merge properties
	if source.Properties != nil {
		if target.Properties == nil {
			target.Properties = make(map[string]*Schema)
		}
		for key, value := range source.Properties {
			target.Properties[key] = value
		}
	}
	
	// Merge required fields
	target.Required = append(target.Required, source.Required...)
	
	// Override scalar fields
	if source.Type != "" {
		target.Type = source.Type
	}
	if source.Title != "" {
		target.Title = source.Title
	}
	if source.Description != "" {
		target.Description = source.Description
	}
	
	// Add more merge logic as needed
}

// Dynamic schema validation for runtime schemas

// DynamicSchemaValidator validates against schemas loaded at runtime
type DynamicSchemaValidator struct {
	name            string
	schemaProvider  func() (*Schema, error)
	cacheTimeout    time.Duration
	cachedSchema    *Schema
	cacheTime       time.Time
	enabled         bool
	priority        int
	mutex           sync.RWMutex
}

// NewDynamicSchemaValidator creates a new dynamic schema validator
func NewDynamicSchemaValidator(name string, provider func() (*Schema, error)) *DynamicSchemaValidator {
	return &DynamicSchemaValidator{
		name:           name,
		schemaProvider: provider,
		cacheTimeout:   5 * time.Minute, // Default cache timeout
		enabled:        true,
		priority:       50,
	}
}

// SetCacheTimeout sets the schema cache timeout
func (v *DynamicSchemaValidator) SetCacheTimeout(timeout time.Duration) *DynamicSchemaValidator {
	v.cacheTimeout = timeout
	return v
}

// Name returns the validator name
func (v *DynamicSchemaValidator) Name() string {
	return fmt.Sprintf("dynamic_schema_%s", v.name)
}

// Validate validates against the dynamic schema
func (v *DynamicSchemaValidator) Validate(ctx context.Context, value interface{}) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	schema, err := v.getSchema()
	if err != nil {
		result := NewValidationResult(false)
		result.AddError(NewValidationError(fmt.Sprintf("failed to load schema: %v", err), nil))
		return result
	}

	validator := NewSchemaValidator(v.name, schema)
	return validator.Validate(ctx, value)
}

// getSchema gets the schema, using cache if valid
func (v *DynamicSchemaValidator) getSchema() (*Schema, error) {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	// Check cache
	if v.cachedSchema != nil && time.Since(v.cacheTime) < v.cacheTimeout {
		return v.cachedSchema, nil
	}

	// Load new schema
	schema, err := v.schemaProvider()
	if err != nil {
		return nil, err
	}

	// Update cache
	v.cachedSchema = schema
	v.cacheTime = time.Now()

	return schema, nil
}

// InvalidateCache invalidates the schema cache
func (v *DynamicSchemaValidator) InvalidateCache() {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.cachedSchema = nil
}

// IsEnabled returns whether the validator is enabled
func (v *DynamicSchemaValidator) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *DynamicSchemaValidator) Priority() int {
	return v.priority
}

// Schema versioning and migration support

// VersionedSchema represents a schema with version information
type VersionedSchema struct {
	Schema   *Schema            `json:"schema"`
	Version  string             `json:"version"`
	Previous string             `json:"previous,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SchemaMigrator handles schema version migrations
type SchemaMigrator struct {
	schemas    map[string]*VersionedSchema
	migrations map[string]func(interface{}) (interface{}, error)
	mutex      sync.RWMutex
}

// NewSchemaMigrator creates a new schema migrator
func NewSchemaMigrator() *SchemaMigrator {
	return &SchemaMigrator{
		schemas:    make(map[string]*VersionedSchema),
		migrations: make(map[string]func(interface{}) (interface{}, error)),
	}
}

// RegisterSchema registers a versioned schema
func (m *SchemaMigrator) RegisterSchema(version string, schema *VersionedSchema) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.schemas[version] = schema
}

// RegisterMigration registers a migration function
func (m *SchemaMigrator) RegisterMigration(fromVersion, toVersion string, migrator func(interface{}) (interface{}, error)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	key := fmt.Sprintf("%s->%s", fromVersion, toVersion)
	m.migrations[key] = migrator
}

// MigrateAndValidate migrates data to the target version and validates it
func (m *SchemaMigrator) MigrateAndValidate(data interface{}, currentVersion, targetVersion string) ValidationResult {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := NewValidationResult(true)

	// Migrate data if needed
	migratedData := data
	if currentVersion != targetVersion {
		var err error
		migratedData, err = m.migrate(data, currentVersion, targetVersion)
		if err != nil {
			result.AddError(NewValidationError(fmt.Sprintf("migration failed: %v", err), nil))
			return result
		}
	}

	// Validate against target schema
	if targetSchema, exists := m.schemas[targetVersion]; exists {
		validator := NewSchemaValidator("migrated", targetSchema.Schema)
		return validator.Validate(context.Background(), migratedData)
	}

	result.AddError(NewValidationError(fmt.Sprintf("schema version %s not found", targetVersion), nil))
	return result
}

// migrate performs the actual migration
func (m *SchemaMigrator) migrate(data interface{}, from, to string) (interface{}, error) {
	if from == to {
		return data, nil
	}

	// Simple direct migration
	key := fmt.Sprintf("%s->%s", from, to)
	if migrator, exists := m.migrations[key]; exists {
		return migrator(data)
	}

	// Could implement multi-step migration here
	return nil, fmt.Errorf("no migration path from %s to %s", from, to)
}