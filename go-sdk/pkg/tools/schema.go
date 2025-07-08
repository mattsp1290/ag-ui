package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"strconv"
	"net/url"
	"net/mail"
	"crypto/md5"
	"encoding/hex"
)

// SchemaValidator provides JSON Schema validation for tool parameters.
type SchemaValidator struct {
	// schema is the tool's parameter schema
	schema *ToolSchema

	// cache holds validation results for performance optimization
	cache *ValidationCache

	// customFormats holds custom format validators
	customFormats map[string]FormatValidator

	// coercionEnabled determines if type coercion is enabled
	coercionEnabled bool

	// debug enables detailed validation logging
	debug bool
}

// NewSchemaValidator creates a new schema validator for the given tool schema.
func NewSchemaValidator(schema *ToolSchema) *SchemaValidator {
	return &SchemaValidator{
		schema:          schema,
		cache:           NewValidationCache(),
		customFormats:   make(map[string]FormatValidator),
		coercionEnabled: true,
		debug:           false,
	}
}

// NewAdvancedSchemaValidator creates a new schema validator with advanced options.
func NewAdvancedSchemaValidator(schema *ToolSchema, opts *ValidatorOptions) *SchemaValidator {
	v := &SchemaValidator{
		schema:          schema,
		cache:           NewValidationCache(),
		customFormats:   make(map[string]FormatValidator),
		coercionEnabled: true, // default value
		debug:           false, // default value
	}

	if opts != nil {
		v.coercionEnabled = opts.CoercionEnabled
		v.debug = opts.Debug
		if opts.CacheSize > 0 {
			v.cache = NewValidationCacheWithSize(opts.CacheSize)
		}
		for name, validator := range opts.CustomFormats {
			v.customFormats[name] = validator
		}
	}

	return v
}

// Validate checks if the given parameters match the tool's schema.
// It returns a detailed error if validation fails.
func (v *SchemaValidator) Validate(params map[string]interface{}) error {
	if v.schema == nil {
		return nil // No schema means any parameters are valid
	}

	// Validate the top-level object
	return v.validateObject(v.schema, params, "")
}

// ValidateWithResult performs validation and returns a detailed result.
func (v *SchemaValidator) ValidateWithResult(params map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		Valid: true,
		Data:  params,
	}
	
	if v.schema == nil {
		return result
	}
	
	// Generate cache key
	cacheKey := v.generateCacheKey(params)
	if cached, exists := v.cache.Get(cacheKey); exists {
		return cached
	}
	
	// Apply type coercion if enabled
	if v.coercionEnabled {
		coercedParams, err := v.coerceTypes(params, v.schema)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, &ValidationError{
				Path:    "",
				Message: fmt.Sprintf("type coercion failed: %v", err),
				Code:    "COERCION_ERROR",
			})
			v.cache.Set(cacheKey, result)
			return result
		}
		result.Data = coercedParams
		params = coercedParams
	}
	
	// Validate the top-level object
	if err := v.validateObject(v.schema, params, ""); err != nil {
		result.Valid = false
		if validationErr, ok := err.(*ValidationError); ok {
			result.Errors = append(result.Errors, validationErr)
		} else {
			result.Errors = append(result.Errors, &ValidationError{
				Path:    "",
				Message: err.Error(),
				Code:    "VALIDATION_ERROR",
			})
		}
	}
	
	v.cache.Set(cacheKey, result)
	return result
}

// SetCoercionEnabled enables or disables type coercion.
func (v *SchemaValidator) SetCoercionEnabled(enabled bool) {
	v.coercionEnabled = enabled
}

// SetDebugMode enables or disables debug mode.
func (v *SchemaValidator) SetDebugMode(enabled bool) {
	v.debug = enabled
}

// AddCustomFormat adds a custom format validator.
func (v *SchemaValidator) AddCustomFormat(name string, validator FormatValidator) {
	v.customFormats[name] = validator
}

// RemoveCustomFormat removes a custom format validator.
func (v *SchemaValidator) RemoveCustomFormat(name string) {
	delete(v.customFormats, name)
}

// ClearCache clears the validation cache.
func (v *SchemaValidator) ClearCache() {
	v.cache.Clear()
}

// validateObject validates an object against a schema.
func (v *SchemaValidator) validateObject(schema *ToolSchema, value map[string]interface{}, path string) error {
	// Check for additional properties
	if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
		for key := range value {
			if _, defined := schema.Properties[key]; !defined {
				return newValidationError(path, fmt.Sprintf("additional property %q is not allowed", key))
			}
		}
	}

	// Check required properties
	for _, required := range schema.Required {
		if _, exists := value[required]; !exists {
			return newValidationError(joinPath(path, required), "required property is missing")
		}
	}

	// Validate each property
	for name, prop := range schema.Properties {
		propPath := joinPath(path, name)
		propValue, exists := value[name]

		if !exists {
			// Check if property is required
			for _, req := range schema.Required {
				if req == name {
					return newValidationError(propPath, "required property is missing")
				}
			}
			// Property is optional and not provided
			continue
		}

		if err := v.validateValue(prop, propValue, propPath); err != nil {
			return err
		}
	}

	return nil
}

// validateValue validates a single value against a property schema.
func (v *SchemaValidator) validateValue(prop *Property, value interface{}, path string) error {
	// Handle schema reference
	if prop.Ref != "" {
		// For now, we'll skip schema references - they would require a schema registry
		// This is a placeholder for future implementation
		return nil
	}
	
	// Handle composition schemas first
	if len(prop.OneOf) > 0 {
		return v.validateOneOf(prop.OneOf, value, path)
	}
	
	if len(prop.AnyOf) > 0 {
		return v.validateAnyOf(prop.AnyOf, value, path)
	}
	
	if len(prop.AllOf) > 0 {
		return v.validateAllOf(prop.AllOf, value, path)
	}
	
	if prop.Not != nil {
		return v.validateNot(prop.Not, value, path)
	}
	
	// Handle conditional schemas
	if prop.If != nil {
		return v.validateConditional(prop, value, path)
	}
	
	// Handle null values
	if value == nil {
		if prop.Type != "" && prop.Type != "null" {
			return newValidationError(path, "value cannot be null")
		}
		return nil
	}

	// If no type is specified, allow any type (this supports advanced composition)
	if prop.Type == "" {
		return nil
	}

	switch prop.Type {
	case "string":
		return v.validateString(prop, value, path)
	case "number":
		return v.validateNumber(prop, value, path)
	case "integer":
		return v.validateInteger(prop, value, path)
	case "boolean":
		return v.validateBoolean(prop, value, path)
	case "array":
		return v.validateArray(prop, value, path)
	case "object":
		return v.validateObjectProperty(prop, value, path)
	case "null":
		if value != nil {
			return newValidationError(path, "value must be null")
		}
		return nil
	default:
		return newValidationError(path, fmt.Sprintf("unknown type %q", prop.Type))
	}
}

// validateString validates a string value.
func (v *SchemaValidator) validateString(prop *Property, value interface{}, path string) error {
	str, ok := value.(string)
	if !ok {
		return newValidationError(path, fmt.Sprintf("expected string, got %T", value))
	}

	// Check enum
	if len(prop.Enum) > 0 {
		found := false
		for _, allowed := range prop.Enum {
			if allowedStr, ok := allowed.(string); ok && allowedStr == str {
				found = true
				break
			}
		}
		if !found {
			return newValidationError(path, fmt.Sprintf("value %q is not in enum %v", str, prop.Enum))
		}
	}

	// Check length constraints
	if prop.MinLength != nil && len(str) < *prop.MinLength {
		return newValidationError(path, fmt.Sprintf("string length %d is less than minimum %d", len(str), *prop.MinLength))
	}
	if prop.MaxLength != nil && len(str) > *prop.MaxLength {
		return newValidationError(path, fmt.Sprintf("string length %d is greater than maximum %d", len(str), *prop.MaxLength))
	}

	// Check pattern
	if prop.Pattern != "" {
		matched, err := regexp.MatchString(prop.Pattern, str)
		if err != nil {
			return newValidationError(path, fmt.Sprintf("invalid pattern: %v", err))
		}
		if !matched {
			return newValidationError(path, fmt.Sprintf("string %q does not match pattern %q", str, prop.Pattern))
		}
	}

	// Check format
	if prop.Format != "" {
		if err := v.validateFormat(prop.Format, str, path); err != nil {
			return err
		}
	}

	return nil
}

// validateNumber validates a numeric value.
func (v *SchemaValidator) validateNumber(prop *Property, value interface{}, path string) error {
	var num float64

	switch val := value.(type) {
	case float64:
		num = val
	case float32:
		num = float64(val)
	case int:
		num = float64(val)
	case int32:
		num = float64(val)
	case int64:
		num = float64(val)
	case json.Number:
		f, err := val.Float64()
		if err != nil {
			return newValidationError(path, fmt.Sprintf("invalid number: %v", err))
		}
		num = f
	default:
		return newValidationError(path, fmt.Sprintf("expected number, got %T", value))
	}

	// Check enum
	if len(prop.Enum) > 0 {
		found := false
		for _, allowed := range prop.Enum {
			if allowedNum, ok := toFloat64(allowed); ok && allowedNum == num {
				found = true
				break
			}
		}
		if !found {
			return newValidationError(path, fmt.Sprintf("value %v is not in enum %v", num, prop.Enum))
		}
	}

	// Check range constraints
	if prop.Minimum != nil && num < *prop.Minimum {
		return newValidationError(path, fmt.Sprintf("value %v is less than minimum %v", num, *prop.Minimum))
	}
	if prop.Maximum != nil && num > *prop.Maximum {
		return newValidationError(path, fmt.Sprintf("value %v is greater than maximum %v", num, *prop.Maximum))
	}

	return nil
}

// validateInteger validates an integer value.
func (v *SchemaValidator) validateInteger(prop *Property, value interface{}, path string) error {
	var num int64

	switch val := value.(type) {
	case int:
		num = int64(val)
	case int32:
		num = int64(val)
	case int64:
		num = val
	case float64:
		if val != float64(int64(val)) {
			return newValidationError(path, fmt.Sprintf("expected integer, got float %v", val))
		}
		num = int64(val)
	case json.Number:
		i, err := val.Int64()
		if err != nil {
			return newValidationError(path, fmt.Sprintf("invalid integer: %v", err))
		}
		num = i
	default:
		return newValidationError(path, fmt.Sprintf("expected integer, got %T", value))
	}

	// Check enum
	if len(prop.Enum) > 0 {
		found := false
		for _, allowed := range prop.Enum {
			if allowedInt, ok := toInt64(allowed); ok && allowedInt == num {
				found = true
				break
			}
		}
		if !found {
			return newValidationError(path, fmt.Sprintf("value %v is not in enum %v", num, prop.Enum))
		}
	}

	// Check range constraints
	if prop.Minimum != nil && float64(num) < *prop.Minimum {
		return newValidationError(path, fmt.Sprintf("value %v is less than minimum %v", num, *prop.Minimum))
	}
	if prop.Maximum != nil && float64(num) > *prop.Maximum {
		return newValidationError(path, fmt.Sprintf("value %v is greater than maximum %v", num, *prop.Maximum))
	}

	return nil
}

// validateBoolean validates a boolean value.
func (v *SchemaValidator) validateBoolean(prop *Property, value interface{}, path string) error {
	_, ok := value.(bool)
	if !ok {
		return newValidationError(path, fmt.Sprintf("expected boolean, got %T", value))
	}
	return nil
}

// validateArray validates an array value.
func (v *SchemaValidator) validateArray(prop *Property, value interface{}, path string) error {
	arr, ok := value.([]interface{})
	if !ok {
		return newValidationError(path, fmt.Sprintf("expected array, got %T", value))
	}

	// Check length constraints
	if prop.MinLength != nil && len(arr) < *prop.MinLength {
		return newValidationError(path, fmt.Sprintf("array length %d is less than minimum %d", len(arr), *prop.MinLength))
	}
	if prop.MaxLength != nil && len(arr) > *prop.MaxLength {
		return newValidationError(path, fmt.Sprintf("array length %d is greater than maximum %d", len(arr), *prop.MaxLength))
	}

	// Validate items
	if prop.Items != nil {
		for i, item := range arr {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			if err := v.validateValue(prop.Items, item, itemPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateObjectProperty validates an object property value.
func (v *SchemaValidator) validateObjectProperty(prop *Property, value interface{}, path string) error {
	obj, ok := value.(map[string]interface{})
	if !ok {
		return newValidationError(path, fmt.Sprintf("expected object, got %T", value))
	}

	// Create a temporary schema for the nested object
	tempSchema := &ToolSchema{
		Type:       "object",
		Properties: prop.Properties,
		Required:   prop.Required,
	}

	return v.validateObject(tempSchema, obj, path)
}


// ValidationError represents a schema validation error.
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (e *ValidationError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ValidationResult represents the result of schema validation.
type ValidationResult struct {
	Valid    bool               `json:"valid"`
	Errors   []*ValidationError `json:"errors,omitempty"`
	Warnings []*ValidationError `json:"warnings,omitempty"`
	Data     interface{}        `json:"data,omitempty"`
}

// ValidatorOptions configures advanced schema validation behavior.
type ValidatorOptions struct {
	// CoercionEnabled enables automatic type coercion
	CoercionEnabled bool
	
	// Debug enables detailed validation logging
	Debug bool
	
	// CacheSize sets the validation cache size (0 = default)
	CacheSize int
	
	// CustomFormats provides custom format validators
	CustomFormats map[string]FormatValidator
	
	// DefaultInjection enables default value injection
	DefaultInjection bool
	
	// StrictMode enables strict validation (no coercion)
	StrictMode bool
}

// FormatValidator defines a custom format validation function.
type FormatValidator func(value string) error

// ValidationCache caches validation results for performance optimization.
type ValidationCache struct {
	cache map[string]*ValidationResult
	mutex sync.RWMutex
	size  int
	maxSize int
}

// NewValidationCache creates a new validation cache with default size.
func NewValidationCache() *ValidationCache {
	return &ValidationCache{
		cache:   make(map[string]*ValidationResult),
		maxSize: 1000,
	}
}

// NewValidationCacheWithSize creates a new validation cache with specified size.
func NewValidationCacheWithSize(size int) *ValidationCache {
	return &ValidationCache{
		cache:   make(map[string]*ValidationResult),
		maxSize: size,
	}
}

// Get retrieves a cached validation result.
func (c *ValidationCache) Get(key string) (*ValidationResult, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	result, exists := c.cache[key]
	return result, exists
}

// Set stores a validation result in the cache.
func (c *ValidationCache) Set(key string, result *ValidationResult) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	if c.size >= c.maxSize {
		// Simple eviction: remove oldest entry
		for k := range c.cache {
			delete(c.cache, k)
			c.size--
			break
		}
	}
	
	c.cache[key] = result
	c.size++
}

// Clear empties the validation cache.
func (c *ValidationCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache = make(map[string]*ValidationResult)
	c.size = 0
}

// SchemaComposition represents schema composition patterns.
type SchemaComposition struct {
	// OneOf specifies that the value must match exactly one of the schemas
	OneOf []*Property `json:"oneOf,omitempty"`
	
	// AnyOf specifies that the value must match at least one of the schemas
	AnyOf []*Property `json:"anyOf,omitempty"`
	
	// AllOf specifies that the value must match all of the schemas
	AllOf []*Property `json:"allOf,omitempty"`
	
	// Not specifies that the value must not match the schema
	Not *Property `json:"not,omitempty"`
}

// ConditionalSchema represents conditional validation logic.
type ConditionalSchema struct {
	// If specifies the condition schema
	If *Property `json:"if,omitempty"`
	
	// Then specifies the schema to apply if the condition is true
	Then *Property `json:"then,omitempty"`
	
	// Else specifies the schema to apply if the condition is false
	Else *Property `json:"else,omitempty"`
}

// SchemaReference represents a JSON Schema reference.
type SchemaReference struct {
	// Ref is the schema reference URI
	Ref string `json:"$ref,omitempty"`
	
	// Resolved is the resolved schema (populated during validation)
	Resolved *Property `json:"-"`
}

// PropertyTransformation defines type transformation rules.
type PropertyTransformation struct {
	// CoercionRules define how to coerce types
	CoercionRules map[string][]string `json:"coercionRules,omitempty"`
	
	// NormalizationRules define how to normalize values
	NormalizationRules map[string]string `json:"normalizationRules,omitempty"`
	
	// DefaultValueRules define default value injection
	DefaultValueRules map[string]interface{} `json:"defaultValueRules,omitempty"`
}

// newValidationError creates a new validation error.
func newValidationError(path, message string) error {
	return &ValidationError{
		Path:    path,
		Message: message,
	}
}

// joinPath joins path segments for error reporting.
func joinPath(base, segment string) string {
	if base == "" {
		return segment
	}
	return base + "." + segment
}

// Helper functions for type conversion
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}

func toInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int32:
		return int64(val), true
	case int64:
		return val, true
	case float64:
		if val == float64(int64(val)) {
			return int64(val), true
		}
		return 0, false
	default:
		return 0, false
	}
}

// Format validation helpers
func isValidEmail(email string) bool {
	// Simple email validation
	parts := strings.Split(email, "@")
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}
	return strings.Contains(parts[1], ".")
}

func isValidURL(url string) bool {
	// Simple URL validation
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func isValidDateTime(dt string) bool {
	// ISO 8601 date-time format validation
	matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z?$`, dt)
	return matched
}

func isValidDate(date string) bool {
	// ISO 8601 date format validation
	matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, date)
	return matched
}

func isValidTime(time string) bool {
	// ISO 8601 time format validation
	matched, _ := regexp.MatchString(`^\d{2}:\d{2}:\d{2}`, time)
	return matched
}

func isValidUUID(uuid string) bool {
	// UUID format validation
	matched, _ := regexp.MatchString(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`, strings.ToLower(uuid))
	return matched
}

// Advanced JSON Schema validation methods

// validateOneOf validates that the value matches exactly one of the provided schemas.
func (v *SchemaValidator) validateOneOf(schemas []*Property, value interface{}, path string) error {
	matchCount := 0
	
	for _, schema := range schemas {
		if err := v.validateValue(schema, value, path); err == nil {
			matchCount++
		}
	}
	
	if matchCount == 0 {
		return newValidationErrorWithCode(path, "value does not match any of the oneOf schemas", "ONEOF_NO_MATCH")
	}
	
	if matchCount > 1 {
		return newValidationErrorWithCode(path, fmt.Sprintf("value matches %d schemas, but oneOf requires exactly one match", matchCount), "ONEOF_MULTIPLE_MATCH")
	}
	
	return nil
}

// validateAnyOf validates that the value matches at least one of the provided schemas.
func (v *SchemaValidator) validateAnyOf(schemas []*Property, value interface{}, path string) error {
	for _, schema := range schemas {
		if err := v.validateValue(schema, value, path); err == nil {
			return nil
		}
	}
	
	return newValidationErrorWithCode(path, "value does not match any of the anyOf schemas", "ANYOF_NO_MATCH")
}

// validateAllOf validates that the value matches all of the provided schemas.
func (v *SchemaValidator) validateAllOf(schemas []*Property, value interface{}, path string) error {
	for i, schema := range schemas {
		if err := v.validateValue(schema, value, path); err != nil {
			return newValidationErrorWithCode(path, fmt.Sprintf("value fails allOf schema at index %d: %v", i, err), "ALLOF_FAILED")
		}
	}
	
	return nil
}

// validateNot validates that the value does not match the provided schema.
func (v *SchemaValidator) validateNot(schema *Property, value interface{}, path string) error {
	if err := v.validateValue(schema, value, path); err == nil {
		return newValidationErrorWithCode(path, "value matches the not schema, but it should not", "NOT_MATCHED")
	}
	
	return nil
}

// validateConditional validates conditional schemas (if/then/else).
func (v *SchemaValidator) validateConditional(prop *Property, value interface{}, path string) error {
	if prop.If == nil {
		return nil
	}
	
	// First, validate the base schema (if it has a type)
	if prop.Type != "" {
		switch prop.Type {
		case "string":
			if err := v.validateString(prop, value, path); err != nil {
				return err
			}
		case "number":
			if err := v.validateNumber(prop, value, path); err != nil {
				return err
			}
		case "integer":
			if err := v.validateInteger(prop, value, path); err != nil {
				return err
			}
		case "boolean":
			if err := v.validateBoolean(prop, value, path); err != nil {
				return err
			}
		case "array":
			if err := v.validateArray(prop, value, path); err != nil {
				return err
			}
		case "object":
			if err := v.validateObjectProperty(prop, value, path); err != nil {
				return err
			}
		}
	}
	
	// Test the condition
	conditionMatches := v.validateValue(prop.If, value, path) == nil
	
	if conditionMatches && prop.Then != nil {
		// Apply the "then" schema
		return v.validateValue(prop.Then, value, path)
	}
	
	if !conditionMatches && prop.Else != nil {
		// Apply the "else" schema
		return v.validateValue(prop.Else, value, path)
	}
	
	return nil
}

// Type coercion methods

// coerceTypes performs type coercion on the input parameters.
func (v *SchemaValidator) coerceTypes(params map[string]interface{}, schema *ToolSchema) (map[string]interface{}, error) {
	if schema == nil || schema.Properties == nil {
		return params, nil
	}
	
	coerced := make(map[string]interface{})
	
	// Copy all existing parameters
	for k, v := range params {
		coerced[k] = v
	}
	
	// Apply coercion rules for each property
	for name, prop := range schema.Properties {
		if value, exists := coerced[name]; exists {
			coercedValue, err := v.coerceValue(value, prop)
			if err != nil {
				return nil, fmt.Errorf("failed to coerce property %q: %w", name, err)
			}
			coerced[name] = coercedValue
		} else if prop.Default != nil {
			// Inject default value
			coerced[name] = prop.Default
		}
	}
	
	return coerced, nil
}

// coerceValue performs type coercion on a single value.
func (v *SchemaValidator) coerceValue(value interface{}, prop *Property) (interface{}, error) {
	if prop.Type == "" {
		return value, nil
	}
	
	switch prop.Type {
	case "string":
		return v.coerceToString(value), nil
	case "number":
		return v.coerceToNumber(value)
	case "integer":
		return v.coerceToInteger(value)
	case "boolean":
		return v.coerceToBoolean(value), nil
	case "array":
		return v.coerceToArray(value), nil
	default:
		return value, nil
	}
}

// coerceToString converts a value to string.
func (v *SchemaValidator) coerceToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// coerceToNumber converts a value to float64.
func (v *SchemaValidator) coerceToNumber(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, nil
		}
		return 0, fmt.Errorf("cannot convert %q to number", v)
	default:
		return 0, fmt.Errorf("cannot convert %T to number", v)
	}
}

// coerceToInteger converts a value to int64.
func (v *SchemaValidator) coerceToInteger(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		if v == float64(int64(v)) {
			return int64(v), nil
		}
		return 0, fmt.Errorf("cannot convert %f to integer", v)
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i, nil
		}
		return 0, fmt.Errorf("cannot convert %q to integer", v)
	default:
		return 0, fmt.Errorf("cannot convert %T to integer", v)
	}
}

// coerceToBoolean converts a value to bool.
func (v *SchemaValidator) coerceToBoolean(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

// coerceToArray converts a value to []interface{}.
func (v *SchemaValidator) coerceToArray(value interface{}) []interface{} {
	switch v := value.(type) {
	case []interface{}:
		return v
	case []string:
		arr := make([]interface{}, len(v))
		for i, s := range v {
			arr[i] = s
		}
		return arr
	case []int:
		arr := make([]interface{}, len(v))
		for i, n := range v {
			arr[i] = n
		}
		return arr
	default:
		return []interface{}{value}
	}
}

// Utility methods

// generateCacheKey generates a cache key for the given parameters.
func (v *SchemaValidator) generateCacheKey(params map[string]interface{}) string {
	data, _ := json.Marshal(params)
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// newValidationErrorWithCode creates a new validation error with a specific code.
func newValidationErrorWithCode(path, message, code string) error {
	return &ValidationError{
		Path:    path,
		Message: message,
		Code:    code,
	}
}

// Enhanced format validation

// validateFormat validates string format constraints with custom formats.
func (v *SchemaValidator) validateFormat(format, value, path string) error {
	// Check custom formats first
	if validator, exists := v.customFormats[format]; exists {
		if err := validator(value); err != nil {
			return newValidationErrorWithCode(path, fmt.Sprintf("format %q validation failed: %v", format, err), "FORMAT_CUSTOM_FAILED")
		}
		return nil
	}
	
	// Built-in formats
	switch format {
	case "email":
		if !isValidEmail(value) {
			return newValidationErrorWithCode(path, fmt.Sprintf("%q is not a valid email address", value), "FORMAT_EMAIL_INVALID")
		}
	case "uri", "url":
		if !isValidURLRFC3986(value) {
			return newValidationErrorWithCode(path, fmt.Sprintf("%q is not a valid URL", value), "FORMAT_URL_INVALID")
		}
	case "date-time":
		if !isValidDateTimeRFC3339(value) {
			return newValidationErrorWithCode(path, fmt.Sprintf("%q is not a valid date-time", value), "FORMAT_DATETIME_INVALID")
		}
	case "date":
		if !isValidDateRFC3339(value) {
			return newValidationErrorWithCode(path, fmt.Sprintf("%q is not a valid date", value), "FORMAT_DATE_INVALID")
		}
	case "time":
		if !isValidTimeRFC3339(value) {
			return newValidationErrorWithCode(path, fmt.Sprintf("%q is not a valid time", value), "FORMAT_TIME_INVALID")
		}
	case "uuid":
		if !isValidUUIDRFC4122(value) {
			return newValidationErrorWithCode(path, fmt.Sprintf("%q is not a valid UUID", value), "FORMAT_UUID_INVALID")
		}
	case "ipv4":
		if !isValidIPv4(value) {
			return newValidationErrorWithCode(path, fmt.Sprintf("%q is not a valid IPv4 address", value), "FORMAT_IPV4_INVALID")
		}
	case "ipv6":
		if !isValidIPv6(value) {
			return newValidationErrorWithCode(path, fmt.Sprintf("%q is not a valid IPv6 address", value), "FORMAT_IPV6_INVALID")
		}
	case "hostname":
		if !isValidHostname(value) {
			return newValidationErrorWithCode(path, fmt.Sprintf("%q is not a valid hostname", value), "FORMAT_HOSTNAME_INVALID")
		}
	case "json-pointer":
		if !isValidJSONPointer(value) {
			return newValidationErrorWithCode(path, fmt.Sprintf("%q is not a valid JSON pointer", value), "FORMAT_JSONPOINTER_INVALID")
		}
	case "regex":
		if !isValidRegex(value) {
			return newValidationErrorWithCode(path, fmt.Sprintf("%q is not a valid regex", value), "FORMAT_REGEX_INVALID")
		}
	}
	return nil
}

// Enhanced format validation helpers

// isValidEmailRFC5322 validates email addresses according to RFC 5322.
func isValidEmailRFC5322(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// isValidURLRFC3986 validates URLs according to RFC 3986.
func isValidURLRFC3986(urlStr string) bool {
	u, err := url.Parse(urlStr)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// isValidDateTimeRFC3339 validates date-time strings according to RFC 3339.
func isValidDateTimeRFC3339(dt string) bool {
	_, err := time.Parse(time.RFC3339, dt)
	return err == nil
}

// isValidDateRFC3339 validates date strings according to RFC 3339.
func isValidDateRFC3339(date string) bool {
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}

// isValidTimeRFC3339 validates time strings according to RFC 3339.
func isValidTimeRFC3339(timeStr string) bool {
	_, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		_, err = time.Parse("15:04:05.999999999", timeStr)
	}
	return err == nil
}

// isValidUUIDRFC4122 validates UUID strings according to RFC 4122.
func isValidUUIDRFC4122(uuid string) bool {
	matched, _ := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, strings.ToLower(uuid))
	return matched
}

// isValidIPv4 validates IPv4 addresses.
func isValidIPv4(ip string) bool {
	matched, _ := regexp.MatchString(`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`, ip)
	return matched
}

// isValidIPv6 validates IPv6 addresses.
func isValidIPv6(ip string) bool {
	matched, _ := regexp.MatchString(`^(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$`, ip)
	if !matched {
		// Check compressed format
		matched, _ = regexp.MatchString(`^::1$|^::$|^([0-9a-fA-F]{1,4}:){1,7}:$|^:([0-9a-fA-F]{1,4}:){1,6}[0-9a-fA-F]{1,4}$|^([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}$|^([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}$|^([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}$|^([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}$|^([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}$|^[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})$`, ip)
	}
	return matched
}

// isValidHostname validates hostnames.
func isValidHostname(hostname string) bool {
	if len(hostname) > 253 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`, hostname)
	return matched
}

// isValidJSONPointer validates JSON pointers.
func isValidJSONPointer(pointer string) bool {
	if pointer == "" {
		return true
	}
	if !strings.HasPrefix(pointer, "/") {
		return false
	}
	// Basic validation - could be more comprehensive
	return !strings.Contains(pointer, "//")
}

// isValidRegex validates regular expressions.
func isValidRegex(pattern string) bool {
	_, err := regexp.Compile(pattern)
	return err == nil
}
