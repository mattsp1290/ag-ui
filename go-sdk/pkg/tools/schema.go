package tools

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"strconv"
	"net/url"
	"net/mail"
	"hash/fnv"
	"encoding/hex"
)

// Schema validation recursion depth limits to prevent stack overflow attacks
const (
	// DefaultMaxSchemaValidationDepth is the default maximum depth for schema validation operations
	DefaultMaxSchemaValidationDepth = 100
	
	// StrictMaxSchemaValidationDepth is the strict maximum depth for security-critical schema validation
	StrictMaxSchemaValidationDepth = 50
)

// Global schema cache for compiled validators
var globalSchemaCache *SchemaCache
var globalSchemaCacheOnce sync.Once

// ResetGlobalSchemaCache resets the global schema cache for testing purposes.
// This function should only be used in test code to ensure proper test isolation.
func ResetGlobalSchemaCache() {
	globalSchemaCache = nil
	globalSchemaCacheOnce = sync.Once{}
}


// SchemaValidator provides JSON Schema validation for tool parameters.
// It supports the JSON Schema draft-07 specification with additional
// features like custom formats, type coercion, and caching.
//
// Key features:
//   - Full JSON Schema validation including composition (oneOf, anyOf, allOf)
//   - Custom format validators for domain-specific validation
//   - Type coercion for flexible parameter handling
//   - Validation result caching for performance
//   - Detailed error reporting with paths and codes
//
// Example usage:
//
//	validator := NewSchemaValidator(tool.Schema)
//	validator.SetCoercionEnabled(true)
//	validator.AddCustomFormat("phone", validatePhoneNumber)
//	
//	if err := validator.Validate(params); err != nil {
//		// Handle validation error
//	}
type SchemaValidator struct {
	// schema is the tool's parameter schema
	schema *ToolSchema

	// cache holds validation results for performance optimization
	cache *ValidationCache

	// customFormats holds custom format validators
	customFormats map[string]FormatValidator
	// mu protects customFormats from concurrent access
	mu sync.RWMutex

	// coercionEnabled determines if type coercion is enabled
	coercionEnabled bool

	// debug enables detailed validation logging
	debug bool
	
	// schemaHash is the hash of the schema for caching
	schemaHash string
	
	// globalCache is the global schema cache
	globalCache *SchemaCache
	
	// maxValidationDepth is the maximum recursion depth for validation operations
	maxValidationDepth int
}

// NewSchemaValidator creates a new schema validator for the given tool schema.
// It uses default settings with type coercion enabled and a standard cache size.
func NewSchemaValidator(schema *ToolSchema) *SchemaValidator {
	// Initialize global cache once
	globalSchemaCacheOnce.Do(func() {
		globalSchemaCache = NewSchemaCache()
	})

	// Generate schema hash for caching
	schemaHash := generateSchemaHash(schema)
	
	// Check if we have a cached validator
	if cachedValidator, exists := globalSchemaCache.Get(schemaHash); exists {
		return cachedValidator
	}

	validator := &SchemaValidator{
		schema:             schema,
		cache:              NewValidationCache(),
		customFormats:      make(map[string]FormatValidator),
		coercionEnabled:    true,
		debug:              false,
		schemaHash:         schemaHash,
		globalCache:        globalSchemaCache,
		maxValidationDepth: DefaultMaxSchemaValidationDepth,
	}

	// Cache the validator
	globalSchemaCache.Set(schemaHash, validator, schema)

	return validator
}

// NewAdvancedSchemaValidator creates a new schema validator with advanced options.
// This allows fine-grained control over validation behavior, caching, and custom formats.
//
// Example:
//
//	opts := &ValidatorOptions{
//		CoercionEnabled:  true,
//		Debug:           true,
//		CacheSize:       500,
//		CustomFormats: map[string]FormatValidator{
//			"ssn": validateSSN,
//		},
//	}
//	validator := NewAdvancedSchemaValidator(schema, opts)
func NewAdvancedSchemaValidator(schema *ToolSchema, opts *ValidatorOptions) *SchemaValidator {
	v := &SchemaValidator{
		schema:             schema,
		cache:              NewValidationCache(),
		customFormats:      make(map[string]FormatValidator),
		coercionEnabled:    true,                               // default value
		debug:              false,                              // default value
		maxValidationDepth: DefaultMaxSchemaValidationDepth,    // default value
	}

	if opts != nil {
		v.coercionEnabled = opts.CoercionEnabled
		v.debug = opts.Debug
		if opts.CacheSize > 0 {
			v.cache = NewValidationCacheWithSize(opts.CacheSize)
		}
		if opts.MaxValidationDepth > 0 {
			v.maxValidationDepth = opts.MaxValidationDepth
		}
		for name, validator := range opts.CustomFormats {
			v.mu.Lock()
			v.customFormats[name] = validator
			v.mu.Unlock()
		}
	}

	return v
}

// Validate checks if the given parameters match the tool's schema.
// It returns a detailed error if validation fails.
//
// The validation process includes:
//   - Type checking against schema definitions
//   - Required field validation
//   - Format validation (email, URL, date-time, etc.)
//   - Constraint validation (min/max, pattern, enum)
//   - Nested object and array validation
//
// Returns nil if validation succeeds, or a ValidationError with details.
func (v *SchemaValidator) Validate(params map[string]interface{}) error {
	if v.schema == nil {
		return nil // No schema means any parameters are valid
	}

	// Validate the top-level object
	return v.validateObject(v.schema, params, "")
}

// ValidateWithResult performs validation and returns a detailed result.
// Unlike Validate, this method returns a structured result containing
// all validation errors, warnings, and the processed data (with type coercion applied).
//
// This is useful when you need:
//   - Multiple validation errors at once
//   - Access to coerced/normalized data
//   - Validation warnings (future feature)
//
// Example:
//
//	result := validator.ValidateWithResult(params)
//	if !result.Valid {
//		for _, err := range result.Errors {
//			log.Printf("Error at %s: %s", err.Path, err.Message)
//		}
//	} else {
//		// Use result.Data which has been coerced to correct types
//	}
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
// When enabled, the validator attempts to convert parameter types
// to match the schema (e.g., string "123" to integer 123).
// This provides flexibility when receiving parameters from various sources.
func (v *SchemaValidator) SetCoercionEnabled(enabled bool) {
	v.coercionEnabled = enabled
}

// SetDebugMode enables or disables debug mode.
// In debug mode, the validator provides more detailed logging
// and validation traces for troubleshooting.
func (v *SchemaValidator) SetDebugMode(enabled bool) {
	v.debug = enabled
}

// AddCustomFormat adds a custom format validator.
// Custom formats extend the built-in format validation with
// domain-specific rules.
//
// Example:
//
//	validator.AddCustomFormat("credit-card", func(value string) error {
//		if !isValidCreditCard(value) {
//			return fmt.Errorf("invalid credit card number")
//		}
//		return nil
//	})
func (v *SchemaValidator) AddCustomFormat(name string, validator FormatValidator) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.customFormats[name] = validator
}

// RemoveCustomFormat removes a custom format validator.
// This restores default behavior for the specified format.
func (v *SchemaValidator) RemoveCustomFormat(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.customFormats, name)
}

// ClearCache clears the validation cache.
// This forces revalidation of previously cached parameter sets.
// Useful after schema changes or when memory usage is a concern.
func (v *SchemaValidator) ClearCache() {
	v.cache.Clear()
}

// validateObject validates an object against a schema.
func (v *SchemaValidator) validateObject(schema *ToolSchema, value map[string]interface{}, path string) error {
	return v.validateObjectWithDepth(schema, value, path, 0)
}

// validateObjectWithDepth validates an object against a schema with depth tracking.
func (v *SchemaValidator) validateObjectWithDepth(schema *ToolSchema, value map[string]interface{}, path string, depth int) error {
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

		if err := v.validateValueWithDepth(prop, propValue, propPath, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// validateValue validates a single value against a property schema.
func (v *SchemaValidator) validateValue(prop *Property, value interface{}, path string) error {
	return v.validateValueWithDepth(prop, value, path, 0)
}

// validateValueWithDepth validates a single value against a property schema with depth tracking.
func (v *SchemaValidator) validateValueWithDepth(prop *Property, value interface{}, path string, depth int) error {
	// Check recursion depth limit to prevent stack overflow attacks
	if depth > v.maxValidationDepth {
		return newValidationErrorWithCode(path, fmt.Sprintf("validation recursion depth %d exceeds maximum %d", depth, v.maxValidationDepth), "RECURSION_DEPTH_EXCEEDED")
	}
	// Handle schema reference
	if prop.Ref != "" {
		// For now, we'll skip schema references - they would require a schema registry
		// This is a placeholder for future implementation
		return nil
	}
	
	// Handle composition schemas first
	if len(prop.OneOf) > 0 {
		return v.validateOneOfWithDepth(prop.OneOf, value, path, depth)
	}
	
	if len(prop.AnyOf) > 0 {
		return v.validateAnyOfWithDepth(prop.AnyOf, value, path, depth)
	}
	
	if len(prop.AllOf) > 0 {
		return v.validateAllOfWithDepth(prop.AllOf, value, path, depth)
	}
	
	if prop.Not != nil {
		return v.validateNotWithDepth(prop.Not, value, path, depth)
	}
	
	// Handle conditional schemas
	if prop.If != nil {
		return v.validateConditionalWithDepth(prop, value, path, depth)
	}
	
	// Handle null values
	if value == nil {
		if prop.Type != "" && prop.Type != "null" {
			return newValidationError(path, "value cannot be null")
		}
		return nil
	}

	// If no type is specified, try to infer from constraints and validate accordingly
	if prop.Type == "" {
		// If we have string-specific constraints, validate as string
		if prop.MinLength != nil || prop.MaxLength != nil || prop.Pattern != "" || prop.Format != "" {
			return v.validateString(prop, value, path)
		}
		// If we have number-specific constraints, validate as number
		if prop.Minimum != nil || prop.Maximum != nil || prop.ExclusiveMinimum != nil || prop.ExclusiveMaximum != nil || prop.MultipleOf != nil {
			return v.validateNumber(prop, value, path)
		}
		// If we have array-specific constraints, validate as array
		if prop.Items != nil || prop.MinItems != nil || prop.MaxItems != nil || prop.UniqueItems != nil {
			return v.validateArray(prop, value, path)
		}
		// If we have object-specific constraints, validate as object
		if prop.Properties != nil || prop.Required != nil || prop.MinProperties != nil || prop.MaxProperties != nil || prop.AdditionalProperties != nil {
			return v.validateObjectProperty(prop, value, path)
		}
		// If we have enum, validate enum without type checking
		if len(prop.Enum) > 0 {
			return v.validateEnum(prop, value, path)
		}
		// No type and no constraints - allow any type
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
		return v.validateArrayWithDepth(prop, value, path, depth)
	case "object":
		return v.validateObjectPropertyWithDepth(prop, value, path, depth)
	case "null":
		if value != nil {
			return newValidationError(path, "value must be null")
		}
		return nil
	default:
		return newValidationError(path, fmt.Sprintf("unknown type %q", prop.Type))
	}
}

// validateEnum validates a value against an enum constraint without type checking
func (v *SchemaValidator) validateEnum(prop *Property, value interface{}, path string) error {
	if len(prop.Enum) == 0 {
		return nil
	}
	
	for _, allowed := range prop.Enum {
		if allowed == value {
			return nil
		}
	}
	
	return newValidationError(path, fmt.Sprintf("value %v is not in enum %v", value, prop.Enum))
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
	return v.validateArrayWithDepth(prop, value, path, 0)
}

// validateArrayWithDepth validates an array value with depth tracking.
func (v *SchemaValidator) validateArrayWithDepth(prop *Property, value interface{}, path string, depth int) error {
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
			if err := v.validateValueWithDepth(prop.Items, item, itemPath, depth+1); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateObjectProperty validates an object property value.
func (v *SchemaValidator) validateObjectProperty(prop *Property, value interface{}, path string) error {
	return v.validateObjectPropertyWithDepth(prop, value, path, 0)
}

// validateObjectPropertyWithDepth validates an object property value with depth tracking.
func (v *SchemaValidator) validateObjectPropertyWithDepth(prop *Property, value interface{}, path string, depth int) error {
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

	return v.validateObjectWithDepth(tempSchema, obj, path, depth)
}


// ValidationError represents a schema validation error.
// It provides detailed information about what failed and where.
//
// Fields:
//   - Path: JSON path to the invalid value (e.g., "user.email")
//   - Message: Human-readable error description
//   - Code: Machine-readable error code for programmatic handling
//   - Details: Additional context about the error
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
// It provides a complete view of the validation outcome including
// all errors, warnings, and the processed data.
//
// Example usage:
//
//	if result.Valid {
//		// Use result.Data safely
//	} else {
//		// Handle result.Errors
//	}
type ValidationResult struct {
	Valid    bool               `json:"valid"`
	Errors   []*ValidationError `json:"errors,omitempty"`
	Warnings []*ValidationError `json:"warnings,omitempty"`
	Data     interface{}        `json:"data,omitempty"`
}

// ValidatorOptions configures advanced schema validation behavior.
// It provides fine-grained control over validation features and performance.
//
// Options:
//   - CoercionEnabled: Automatically convert compatible types
//   - Debug: Enable detailed validation logging
//   - CacheSize: Number of validation results to cache (0 = default)
//   - CustomFormats: Additional format validators
//   - DefaultInjection: Inject default values for missing fields
//   - StrictMode: Disable all coercion and flexibility features
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
	
	// MaxValidationDepth sets the maximum recursion depth for validation operations
	MaxValidationDepth int
}

// FormatValidator defines a custom format validation function.
// It receives a string value and returns an error if the format is invalid.
// Format validators are used to extend the built-in format validation
// with application-specific rules.
type FormatValidator func(value string) error

// ValidationCache caches validation results for performance optimization.
// It uses an LRU (Least Recently Used) eviction strategy to maintain
// a bounded size while keeping frequently validated parameter sets in memory.
//
// The cache significantly improves performance for:
//   - Repeated validations of the same parameters
//   - High-frequency tool executions
//   - Complex schemas with expensive validation logic
type ValidationCache struct {
	cache map[string]*ValidationResult
	mutex sync.RWMutex
	size  int
	maxSize int
	// LRU tracking
	accessOrder []string
}

// NewValidationCache creates a new validation cache with default size.
// The default size is 1000 entries, suitable for most applications.
func NewValidationCache() *ValidationCache {
	return &ValidationCache{
		cache:       make(map[string]*ValidationResult),
		maxSize:     1000,
		accessOrder: make([]string, 0),
	}
}

// NewValidationCacheWithSize creates a new validation cache with specified size.
// Use a larger size for applications with many unique parameter combinations,
// or a smaller size to reduce memory usage.
func NewValidationCacheWithSize(size int) *ValidationCache {
	return &ValidationCache{
		cache:       make(map[string]*ValidationResult),
		maxSize:     size,
		accessOrder: make([]string, 0),
	}
}

// Get retrieves a cached validation result.
// Returns the cached result and true if found, or nil and false if not cached.
// Accessing a cached entry updates its position in the LRU order.
func (c *ValidationCache) Get(key string) (*ValidationResult, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	result, exists := c.cache[key]
	if exists {
		// Move to end of access order (most recently used)
		c.moveToEnd(key)
	}
	return result, exists
}

// Set stores a validation result in the cache.
// If the cache is at capacity, the least recently used entry is evicted.
// If the key already exists, it's updated and moved to the most recent position.
func (c *ValidationCache) Set(key string, result *ValidationResult) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// If key already exists, update and move to end
	if _, exists := c.cache[key]; exists {
		c.cache[key] = result
		c.moveToEnd(key)
		return
	}
	
	// If cache is full, evict least recently used
	if c.size >= c.maxSize && c.maxSize > 0 {
		c.evictLRU()
	}
	
	// Add new entry
	c.cache[key] = result
	c.accessOrder = append(c.accessOrder, key)
	c.size++
}

// Clear empties the validation cache.
// All cached validation results are removed, forcing fresh validation
// for all subsequent requests.
func (c *ValidationCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache = make(map[string]*ValidationResult)
	c.accessOrder = make([]string, 0)
	c.size = 0
}

// moveToEnd moves a key to the end of the access order (most recently used)
func (c *ValidationCache) moveToEnd(key string) {
	// Find and remove the key from its current position
	for i, k := range c.accessOrder {
		if k == key {
			c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
			break
		}
	}
	// Add to end
	c.accessOrder = append(c.accessOrder, key)
}

// evictLRU removes the least recently used entry
func (c *ValidationCache) evictLRU() {
	if len(c.accessOrder) == 0 {
		return
	}
	
	// Remove the first entry (least recently used)
	lruKey := c.accessOrder[0]
	c.accessOrder = c.accessOrder[1:]
	delete(c.cache, lruKey)
	c.size--
}

// SchemaComposition represents schema composition patterns.
// These patterns enable complex validation logic by combining multiple schemas.
//
// Composition types:
//   - OneOf: Exactly one schema must match
//   - AnyOf: At least one schema must match
//   - AllOf: All schemas must match
//   - Not: The schema must not match
//
// Example:
//
//	property := &Property{
//		OneOf: []*Property{
//			{Type: "string", Pattern: "^https://"},
//			{Type: "string", Pattern: "^file://"},
//		},
//	}
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
// It enables if-then-else validation patterns based on data values.
//
// Example:
//
//	property := &Property{
//		If:   &Property{Properties: map[string]*Property{"type": {Enum: []interface{}{"premium"}}}},
//		Then: &Property{Required: []string{"creditCard"}},
//		Else: &Property{Required: []string{"email"}},
//	}
type ConditionalSchema struct {
	// If specifies the condition schema
	If *Property `json:"if,omitempty"`
	
	// Then specifies the schema to apply if the condition is true
	Then *Property `json:"then,omitempty"`
	
	// Else specifies the schema to apply if the condition is false
	Else *Property `json:"else,omitempty"`
}

// SchemaReference represents a JSON Schema reference.
// References enable schema reuse and modular schema design.
// The $ref property contains a URI pointing to another schema definition.
type SchemaReference struct {
	// Ref is the schema reference URI
	Ref string `json:"$ref,omitempty"`
	
	// Resolved is the resolved schema (populated during validation)
	Resolved *Property `json:"-"`
}

// PropertyTransformation defines type transformation rules.
// These rules control how values are coerced, normalized, and defaulted
// during validation.
//
// Features:
//   - CoercionRules: Type conversion mappings
//   - NormalizationRules: Value standardization (e.g., lowercase emails)
//   - DefaultValueRules: Automatic default value injection
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
	return v.validateOneOfWithDepth(schemas, value, path, 0)
}

// validateOneOfWithDepth validates that the value matches exactly one of the provided schemas with depth tracking.
func (v *SchemaValidator) validateOneOfWithDepth(schemas []*Property, value interface{}, path string, depth int) error {
	matchCount := 0
	
	for _, schema := range schemas {
		if err := v.validateValueWithDepth(schema, value, path, depth+1); err == nil {
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
	return v.validateAnyOfWithDepth(schemas, value, path, 0)
}

// validateAnyOfWithDepth validates that the value matches at least one of the provided schemas with depth tracking.
func (v *SchemaValidator) validateAnyOfWithDepth(schemas []*Property, value interface{}, path string, depth int) error {
	for _, schema := range schemas {
		if err := v.validateValueWithDepth(schema, value, path, depth+1); err == nil {
			return nil
		}
	}
	
	return newValidationErrorWithCode(path, "value does not match any of the anyOf schemas", "ANYOF_NO_MATCH")
}

// validateAllOf validates that the value matches all of the provided schemas.
func (v *SchemaValidator) validateAllOf(schemas []*Property, value interface{}, path string) error {
	return v.validateAllOfWithDepth(schemas, value, path, 0)
}

// validateAllOfWithDepth validates that the value matches all of the provided schemas with depth tracking.
func (v *SchemaValidator) validateAllOfWithDepth(schemas []*Property, value interface{}, path string, depth int) error {
	for i, schema := range schemas {
		if err := v.validateValueWithDepth(schema, value, path, depth+1); err != nil {
			return newValidationErrorWithCode(path, fmt.Sprintf("value fails allOf schema at index %d: %v", i, err), "ALLOF_FAILED")
		}
	}
	
	return nil
}

// validateNot validates that the value does not match the provided schema.
func (v *SchemaValidator) validateNot(schema *Property, value interface{}, path string) error {
	return v.validateNotWithDepth(schema, value, path, 0)
}

// validateNotWithDepth validates that the value does not match the provided schema with depth tracking.
func (v *SchemaValidator) validateNotWithDepth(schema *Property, value interface{}, path string, depth int) error {
	if err := v.validateValueWithDepth(schema, value, path, depth+1); err == nil {
		return newValidationErrorWithCode(path, "value matches the not schema, but it should not", "NOT_MATCHED")
	}
	
	return nil
}

// validateConditional validates conditional schemas (if/then/else).
func (v *SchemaValidator) validateConditional(prop *Property, value interface{}, path string) error {
	return v.validateConditionalWithDepth(prop, value, path, 0)
}

// validateConditionalWithDepth validates conditional schemas (if/then/else) with depth tracking.
func (v *SchemaValidator) validateConditionalWithDepth(prop *Property, value interface{}, path string, depth int) error {
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
	conditionMatches := v.validateValueWithDepth(prop.If, value, path, depth+1) == nil
	
	if conditionMatches && prop.Then != nil {
		// Apply the "then" schema
		return v.validateValueWithDepth(prop.Then, value, path, depth+1)
	}
	
	if !conditionMatches && prop.Else != nil {
		// Apply the "else" schema
		return v.validateValueWithDepth(prop.Else, value, path, depth+1)
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
	// Include validation settings in cache key to ensure correct caching
	cacheData := struct {
		Params          map[string]interface{} `json:"params"`
		CoercionEnabled bool                   `json:"coercionEnabled"`
		Debug           bool                   `json:"debug"`
	}{
		Params:          params,
		CoercionEnabled: v.coercionEnabled,
		Debug:           v.debug,
	}
	
	data, _ := json.Marshal(cacheData)
	hash := fnv.New64a()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
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
	v.mu.RLock()
	validator, exists := v.customFormats[format]
	v.mu.RUnlock()
	
	if exists {
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

// generateSchemaHash generates a SHA-256 hash of the schema for caching.
func generateSchemaHash(schema *ToolSchema) string {
	if schema == nil {
		return ""
	}

	// Serialize the schema to JSON for hashing
	data, err := json.Marshal(schema)
	if err != nil {
		// Fallback to a default hash if serialization fails
		return "default"
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
