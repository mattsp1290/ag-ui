package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// APIValidatorExecutor implements comprehensive API request/response validation.
// This example demonstrates complex schema validation, conditional validation,
// custom validation rules, and detailed error reporting.
type APIValidatorExecutor struct {
	customValidators map[string]CustomValidator
	schemaCache      map[string]*ValidationSchema
}

// CustomValidator defines a custom validation function
type CustomValidator func(value interface{}) error

// ValidationSchema represents a comprehensive validation schema
type ValidationSchema struct {
	Type                 string                        `json:"type"`
	Properties           map[string]*PropertySchema    `json:"properties,omitempty"`
	Required             []string                      `json:"required,omitempty"`
	AdditionalProperties bool                          `json:"additionalProperties"`
	Definitions          map[string]*ValidationSchema  `json:"definitions,omitempty"`
	Conditional          *ConditionalValidation        `json:"conditional,omitempty"`
}

// PropertySchema defines validation rules for a single property
type PropertySchema struct {
	Type         string                      `json:"type"`
	Format       string                      `json:"format,omitempty"`
	Pattern      string                      `json:"pattern,omitempty"`
	Enum         []interface{}               `json:"enum,omitempty"`
	Minimum      *float64                    `json:"minimum,omitempty"`
	Maximum      *float64                    `json:"maximum,omitempty"`
	MinLength    *int                        `json:"minLength,omitempty"`
	MaxLength    *int                        `json:"maxLength,omitempty"`
	Items        *PropertySchema             `json:"items,omitempty"`
	Properties   map[string]*PropertySchema  `json:"properties,omitempty"`
	Required     []string                    `json:"required,omitempty"`
	Custom       string                      `json:"custom,omitempty"`
	Default      interface{}                 `json:"default,omitempty"`
	Description  string                      `json:"description,omitempty"`
	Example      interface{}                 `json:"example,omitempty"`
	OneOf        []*PropertySchema           `json:"oneOf,omitempty"`
	AnyOf        []*PropertySchema           `json:"anyOf,omitempty"`
	AllOf        []*PropertySchema           `json:"allOf,omitempty"`
	Not          *PropertySchema             `json:"not,omitempty"`
	If           *PropertySchema             `json:"if,omitempty"`
	Then         *PropertySchema             `json:"then,omitempty"`
	Else         *PropertySchema             `json:"else,omitempty"`
}

// ConditionalValidation defines conditional validation rules
type ConditionalValidation struct {
	Field      string                 `json:"field"`
	Value      interface{}            `json:"value"`
	Required   []string               `json:"required,omitempty"`
	Properties map[string]*PropertySchema `json:"properties,omitempty"`
}

// ValidationResult represents the result of validation
type ValidationResult struct {
	Valid      bool                     `json:"valid"`
	Errors     []ValidationError        `json:"errors,omitempty"`
	Warnings   []ValidationError        `json:"warnings,omitempty"`
	Normalized map[string]interface{}   `json:"normalized,omitempty"`
	Stats      ValidationStats          `json:"stats"`
}

// ValidationError represents a validation error with detailed context
type ValidationError struct {
	Path        string      `json:"path"`
	Field       string      `json:"field"`
	Message     string      `json:"message"`
	Code        string      `json:"code"`
	Expected    interface{} `json:"expected,omitempty"`
	Actual      interface{} `json:"actual,omitempty"`
	Severity    string      `json:"severity"`
	Rule        string      `json:"rule,omitempty"`
	Context     interface{} `json:"context,omitempty"`
}

// ValidationStats provides statistics about the validation process
type ValidationStats struct {
	FieldsValidated    int           `json:"fields_validated"`
	ErrorCount         int           `json:"error_count"`
	WarningCount       int           `json:"warning_count"`
	ValidationTime     time.Duration `json:"validation_time"`
	SchemaComplexity   int           `json:"schema_complexity"`
	CustomRulesApplied int           `json:"custom_rules_applied"`
}

// NewAPIValidatorExecutor creates a new API validator executor
func NewAPIValidatorExecutor() *APIValidatorExecutor {
	executor := &APIValidatorExecutor{
		customValidators: make(map[string]CustomValidator),
		schemaCache:      make(map[string]*ValidationSchema),
	}

	// Register built-in custom validators
	executor.registerBuiltinValidators()
	
	return executor
}

// Execute performs API validation based on the provided parameters
func (a *APIValidatorExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	startTime := time.Now()

	// Extract validation parameters
	validationType, ok := params["validation_type"].(string)
	if !ok {
		return nil, fmt.Errorf("validation_type parameter must be a string")
	}

	data, exists := params["data"]
	if !exists {
		return nil, fmt.Errorf("data parameter is required")
	}

	schemaParam, exists := params["schema"]
	if !exists {
		return nil, fmt.Errorf("schema parameter is required")
	}

	// Parse validation schema
	schema, err := a.parseSchema(schemaParam)
	if err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}

	// Extract options
	options := a.extractValidationOptions(params)

	// Perform validation based on type
	var result *ValidationResult
	switch validationType {
	case "request":
		result = a.validateRequest(data, schema, options)
	case "response":
		result = a.validateResponse(data, schema, options)
	case "schema":
		result = a.validateSchema(data, schema, options)
	case "format":
		result = a.validateFormat(data, schema, options)
	default:
		return nil, fmt.Errorf("unsupported validation type: %s", validationType)
	}

	// Update statistics
	result.Stats.ValidationTime = time.Since(startTime)
	result.Stats.SchemaComplexity = a.calculateSchemaComplexity(schema)

	// Prepare response
	responseData := map[string]interface{}{
		"validation_result": result,
		"summary": map[string]interface{}{
			"valid":           result.Valid,
			"error_count":     result.Stats.ErrorCount,
			"warning_count":   result.Stats.WarningCount,
			"fields_validated": result.Stats.FieldsValidated,
			"validation_time_ms": result.Stats.ValidationTime.Milliseconds(),
		},
	}

	if !result.Valid {
		responseData["error_details"] = a.categorizeErrors(result.Errors)
	}

	if len(result.Warnings) > 0 {
		responseData["warning_details"] = a.categorizeErrors(result.Warnings)
	}

	if options.NormalizeData && result.Normalized != nil {
		responseData["normalized_data"] = result.Normalized
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    responseData,
		Timestamp: time.Now(),
		Duration: result.Stats.ValidationTime,
		Metadata: map[string]interface{}{
			"validation_type": validationType,
			"schema_version":  schema.Type,
			"processed_at":   time.Now().Format(time.RFC3339),
			"performance": map[string]interface{}{
				"validation_time_ns": result.Stats.ValidationTime.Nanoseconds(),
				"fields_per_second":  float64(result.Stats.FieldsValidated) / result.Stats.ValidationTime.Seconds(),
			},
		},
	}, nil
}

// ValidationOptions configures validation behavior
type ValidationOptions struct {
	StrictMode       bool `json:"strict_mode"`
	NormalizeData    bool `json:"normalize_data"`
	IncludeWarnings  bool `json:"include_warnings"`
	MaxErrors        int  `json:"max_errors"`
	CustomValidation bool `json:"custom_validation"`
}

// parseSchema converts the schema parameter to a ValidationSchema
func (a *APIValidatorExecutor) parseSchema(schemaParam interface{}) (*ValidationSchema, error) {
	schemaBytes, err := json.Marshal(schemaParam)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	var schema ValidationSchema
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	return &schema, nil
}

// extractValidationOptions extracts validation options from parameters
func (a *APIValidatorExecutor) extractValidationOptions(params map[string]interface{}) ValidationOptions {
	options := ValidationOptions{
		StrictMode:       false,
		NormalizeData:    false,
		IncludeWarnings:  true,
		MaxErrors:        100,
		CustomValidation: true,
	}

	if opts, exists := params["options"]; exists {
		if optsMap, ok := opts.(map[string]interface{}); ok {
			if strict, exists := optsMap["strict_mode"]; exists {
				if strictBool, ok := strict.(bool); ok {
					options.StrictMode = strictBool
				}
			}
			if normalize, exists := optsMap["normalize_data"]; exists {
				if normalizeBool, ok := normalize.(bool); ok {
					options.NormalizeData = normalizeBool
				}
			}
			if warnings, exists := optsMap["include_warnings"]; exists {
				if warningsBool, ok := warnings.(bool); ok {
					options.IncludeWarnings = warningsBool
				}
			}
			if maxErrors, exists := optsMap["max_errors"]; exists {
				if maxErrorsFloat, ok := maxErrors.(float64); ok {
					options.MaxErrors = int(maxErrorsFloat)
				}
			}
		}
	}

	return options
}

// validateRequest validates an API request
func (a *APIValidatorExecutor) validateRequest(data interface{}, schema *ValidationSchema, options ValidationOptions) *ValidationResult {
	result := &ValidationResult{
		Valid:      true,
		Errors:     []ValidationError{},
		Warnings:   []ValidationError{},
		Normalized: make(map[string]interface{}),
		Stats:      ValidationStats{},
	}

	// Convert data to map for validation
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:     "",
			Field:    "root",
			Message:  "request data must be an object",
			Code:     "INVALID_TYPE",
			Expected: "object",
			Actual:   fmt.Sprintf("%T", data),
			Severity: "error",
			Rule:     "type_validation",
		})
		result.Stats.ErrorCount = 1
		return result
	}

	// Validate against schema
	a.validateObject(dataMap, schema, "", result, options)

	// Apply custom validation rules if enabled
	if options.CustomValidation {
		a.applyCustomValidation(dataMap, result, options)
	}

	// Normalize data if requested
	if options.NormalizeData {
		result.Normalized = a.normalizeData(dataMap, schema)
	}

	return result
}

// validateResponse validates an API response
func (a *APIValidatorExecutor) validateResponse(data interface{}, schema *ValidationSchema, options ValidationOptions) *ValidationResult {
	// For responses, we might be more lenient about missing fields
	relaxedOptions := options
	relaxedOptions.StrictMode = false
	
	return a.validateRequest(data, schema, relaxedOptions)
}

// validateSchema validates data against a schema without specific API context
func (a *APIValidatorExecutor) validateSchema(data interface{}, schema *ValidationSchema, options ValidationOptions) *ValidationResult {
	return a.validateRequest(data, schema, options)
}

// validateFormat validates specific format requirements
func (a *APIValidatorExecutor) validateFormat(data interface{}, schema *ValidationSchema, options ValidationOptions) *ValidationResult {
	result := &ValidationResult{
		Valid:      true,
		Errors:     []ValidationError{},
		Warnings:   []ValidationError{},
		Normalized: make(map[string]interface{}),
		Stats:      ValidationStats{},
	}

	// Focus on format validation
	if dataStr, ok := data.(string); ok {
		if schema.Properties != nil {
			for fieldName, prop := range schema.Properties {
				if prop.Format != "" {
					if err := a.validateFormatConstraint(dataStr, prop.Format); err != nil {
						result.Valid = false
						result.Errors = append(result.Errors, ValidationError{
							Path:     fieldName,
							Field:    fieldName,
							Message:  err.Error(),
							Code:     "FORMAT_VALIDATION_FAILED",
							Expected: prop.Format,
							Actual:   dataStr,
							Severity: "error",
							Rule:     "format_validation",
						})
						result.Stats.ErrorCount++
					}
				}
				result.Stats.FieldsValidated++
			}
		}
	}

	return result
}

// validateObject validates an object against a schema
func (a *APIValidatorExecutor) validateObject(data map[string]interface{}, schema *ValidationSchema, path string, result *ValidationResult, options ValidationOptions) {
	// Check required fields
	for _, required := range schema.Required {
		if _, exists := data[required]; !exists {
			result.Valid = false
			fieldPath := a.buildPath(path, required)
			result.Errors = append(result.Errors, ValidationError{
				Path:     fieldPath,
				Field:    required,
				Message:  "required field is missing",
				Code:     "MISSING_REQUIRED_FIELD",
				Expected: "present",
				Actual:   "missing",
				Severity: "error",
				Rule:     "required_validation",
			})
			result.Stats.ErrorCount++

			if result.Stats.ErrorCount >= options.MaxErrors {
				return
			}
		}
	}

	// Validate each field
	for fieldName, fieldValue := range data {
		fieldPath := a.buildPath(path, fieldName)
		
		if propSchema, exists := schema.Properties[fieldName]; exists {
			a.validateProperty(fieldValue, propSchema, fieldPath, fieldName, result, options)
		} else if !schema.AdditionalProperties && options.StrictMode {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Path:     fieldPath,
				Field:    fieldName,
				Message:  "additional property not allowed",
				Code:     "ADDITIONAL_PROPERTY_NOT_ALLOWED",
				Expected: "defined property",
				Actual:   fieldName,
				Severity: "error",
				Rule:     "additional_properties",
			})
			result.Stats.ErrorCount++
		} else if !schema.AdditionalProperties {
			// Add warning for additional properties in non-strict mode
			if options.IncludeWarnings {
				result.Warnings = append(result.Warnings, ValidationError{
					Path:     fieldPath,
					Field:    fieldName,
					Message:  "additional property found",
					Code:     "ADDITIONAL_PROPERTY_WARNING",
					Expected: "defined property",
					Actual:   fieldName,
					Severity: "warning",
					Rule:     "additional_properties",
				})
				result.Stats.WarningCount++
			}
		}

		result.Stats.FieldsValidated++

		if result.Stats.ErrorCount >= options.MaxErrors {
			return
		}
	}

	// Apply conditional validation
	if schema.Conditional != nil {
		a.applyConditionalValidation(data, schema.Conditional, path, result, options)
	}
}

// validateProperty validates a single property
func (a *APIValidatorExecutor) validateProperty(value interface{}, schema *PropertySchema, path, fieldName string, result *ValidationResult, options ValidationOptions) {
	// Type validation
	if !a.validateType(value, schema.Type) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:     path,
			Field:    fieldName,
			Message:  fmt.Sprintf("expected type %s", schema.Type),
			Code:     "TYPE_MISMATCH",
			Expected: schema.Type,
			Actual:   fmt.Sprintf("%T", value),
			Severity: "error",
			Rule:     "type_validation",
		})
		result.Stats.ErrorCount++
		return
	}

	// String-specific validations
	if schema.Type == "string" && value != nil {
		if strValue, ok := value.(string); ok {
			a.validateStringConstraints(strValue, schema, path, fieldName, result, options)
		}
	}

	// Number-specific validations
	if (schema.Type == "number" || schema.Type == "integer") && value != nil {
		if numValue, ok := a.toFloat64(value); ok {
			a.validateNumberConstraints(numValue, schema, path, fieldName, result, options)
		}
	}

	// Array-specific validations
	if schema.Type == "array" && value != nil {
		if arrValue, ok := value.([]interface{}); ok {
			a.validateArrayConstraints(arrValue, schema, path, fieldName, result, options)
		}
	}

	// Object-specific validations
	if schema.Type == "object" && value != nil {
		if objValue, ok := value.(map[string]interface{}); ok {
			objSchema := &ValidationSchema{
				Type:                 "object",
				Properties:           schema.Properties,
				Required:             schema.Required,
				AdditionalProperties: true, // Default for nested objects
			}
			a.validateObject(objValue, objSchema, path, result, options)
		}
	}

	// Enum validation
	if len(schema.Enum) > 0 {
		a.validateEnum(value, schema.Enum, path, fieldName, result)
	}

	// Custom validation
	if schema.Custom != "" && options.CustomValidation {
		a.applyCustomPropertyValidation(value, schema.Custom, path, fieldName, result)
	}

	// Composition validation (oneOf, anyOf, allOf, not)
	a.validateComposition(value, schema, path, fieldName, result, options)

	// Conditional validation (if/then/else)
	a.validateConditionalProperty(value, schema, path, fieldName, result, options)
}

// validateStringConstraints validates string-specific constraints
func (a *APIValidatorExecutor) validateStringConstraints(value string, schema *PropertySchema, path, fieldName string, result *ValidationResult, options ValidationOptions) {
	// Length constraints
	if schema.MinLength != nil && len(value) < *schema.MinLength {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:     path,
			Field:    fieldName,
			Message:  fmt.Sprintf("string length %d is less than minimum %d", len(value), *schema.MinLength),
			Code:     "STRING_TOO_SHORT",
			Expected: fmt.Sprintf("length >= %d", *schema.MinLength),
			Actual:   len(value),
			Severity: "error",
			Rule:     "length_validation",
		})
		result.Stats.ErrorCount++
	}

	if schema.MaxLength != nil && len(value) > *schema.MaxLength {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:     path,
			Field:    fieldName,
			Message:  fmt.Sprintf("string length %d exceeds maximum %d", len(value), *schema.MaxLength),
			Code:     "STRING_TOO_LONG",
			Expected: fmt.Sprintf("length <= %d", *schema.MaxLength),
			Actual:   len(value),
			Severity: "error",
			Rule:     "length_validation",
		})
		result.Stats.ErrorCount++
	}

	// Pattern validation
	if schema.Pattern != "" {
		matched, err := regexp.MatchString(schema.Pattern, value)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Path:     path,
				Field:    fieldName,
				Message:  fmt.Sprintf("invalid pattern: %v", err),
				Code:     "INVALID_PATTERN",
				Expected: "valid regex pattern",
				Actual:   schema.Pattern,
				Severity: "error",
				Rule:     "pattern_validation",
			})
			result.Stats.ErrorCount++
		} else if !matched {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Path:     path,
				Field:    fieldName,
				Message:  fmt.Sprintf("string does not match pattern %s", schema.Pattern),
				Code:     "PATTERN_MISMATCH",
				Expected: schema.Pattern,
				Actual:   value,
				Severity: "error",
				Rule:     "pattern_validation",
			})
			result.Stats.ErrorCount++
		}
	}

	// Format validation
	if schema.Format != "" {
		if err := a.validateFormatConstraint(value, schema.Format); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Path:     path,
				Field:    fieldName,
				Message:  err.Error(),
				Code:     "FORMAT_VALIDATION_FAILED",
				Expected: schema.Format,
				Actual:   value,
				Severity: "error",
				Rule:     "format_validation",
			})
			result.Stats.ErrorCount++
		}
	}
}

// validateNumberConstraints validates number-specific constraints
func (a *APIValidatorExecutor) validateNumberConstraints(value float64, schema *PropertySchema, path, fieldName string, result *ValidationResult, options ValidationOptions) {
	if schema.Minimum != nil && value < *schema.Minimum {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:     path,
			Field:    fieldName,
			Message:  fmt.Sprintf("value %v is less than minimum %v", value, *schema.Minimum),
			Code:     "VALUE_TOO_SMALL",
			Expected: fmt.Sprintf(">= %v", *schema.Minimum),
			Actual:   value,
			Severity: "error",
			Rule:     "range_validation",
		})
		result.Stats.ErrorCount++
	}

	if schema.Maximum != nil && value > *schema.Maximum {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:     path,
			Field:    fieldName,
			Message:  fmt.Sprintf("value %v exceeds maximum %v", value, *schema.Maximum),
			Code:     "VALUE_TOO_LARGE",
			Expected: fmt.Sprintf("<= %v", *schema.Maximum),
			Actual:   value,
			Severity: "error",
			Rule:     "range_validation",
		})
		result.Stats.ErrorCount++
	}
}

// validateArrayConstraints validates array-specific constraints
func (a *APIValidatorExecutor) validateArrayConstraints(value []interface{}, schema *PropertySchema, path, fieldName string, result *ValidationResult, options ValidationOptions) {
	// Length constraints (reusing MinLength/MaxLength for arrays)
	if schema.MinLength != nil && len(value) < *schema.MinLength {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:     path,
			Field:    fieldName,
			Message:  fmt.Sprintf("array length %d is less than minimum %d", len(value), *schema.MinLength),
			Code:     "ARRAY_TOO_SHORT",
			Expected: fmt.Sprintf("length >= %d", *schema.MinLength),
			Actual:   len(value),
			Severity: "error",
			Rule:     "length_validation",
		})
		result.Stats.ErrorCount++
	}

	if schema.MaxLength != nil && len(value) > *schema.MaxLength {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:     path,
			Field:    fieldName,
			Message:  fmt.Sprintf("array length %d exceeds maximum %d", len(value), *schema.MaxLength),
			Code:     "ARRAY_TOO_LONG",
			Expected: fmt.Sprintf("length <= %d", *schema.MaxLength),
			Actual:   len(value),
			Severity: "error",
			Rule:     "length_validation",
		})
		result.Stats.ErrorCount++
	}

	// Validate array items
	if schema.Items != nil {
		for i, item := range value {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			a.validateProperty(item, schema.Items, itemPath, fmt.Sprintf("%s[%d]", fieldName, i), result, options)
		}
	}
}

// Helper methods for validation

func (a *APIValidatorExecutor) validateType(value interface{}, expectedType string) bool {
	if value == nil {
		return expectedType == "null"
	}

	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := a.toFloat64(value)
		return ok
	case "integer":
		if f, ok := a.toFloat64(value); ok {
			return f == float64(int64(f))
		}
		return false
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	case "null":
		return value == nil
	default:
		return true // Unknown types are considered valid
	}
}

func (a *APIValidatorExecutor) toFloat64(value interface{}) (float64, bool) {
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
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func (a *APIValidatorExecutor) validateEnum(value interface{}, enum []interface{}, path, fieldName string, result *ValidationResult) {
	for _, allowed := range enum {
		if value == allowed {
			return
		}
	}

	result.Valid = false
	result.Errors = append(result.Errors, ValidationError{
		Path:     path,
		Field:    fieldName,
		Message:  fmt.Sprintf("value is not in allowed enum values"),
		Code:     "ENUM_VIOLATION",
		Expected: enum,
		Actual:   value,
		Severity: "error",
		Rule:     "enum_validation",
	})
	result.Stats.ErrorCount++
}

func (a *APIValidatorExecutor) validateFormatConstraint(value, format string) error {
	switch format {
	case "email":
		if _, err := mail.ParseAddress(value); err != nil {
			return fmt.Errorf("invalid email format: %v", err)
		}
	case "uri", "url":
		if _, err := url.Parse(value); err != nil {
			return fmt.Errorf("invalid URL format: %v", err)
		}
	case "date":
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return fmt.Errorf("invalid date format (expected YYYY-MM-DD): %v", err)
		}
	case "date-time":
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return fmt.Errorf("invalid date-time format (expected RFC3339): %v", err)
		}
	case "uuid":
		uuidPattern := `^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
		matched, _ := regexp.MatchString(uuidPattern, strings.ToLower(value))
		if !matched {
			return fmt.Errorf("invalid UUID format")
		}
	case "ipv4":
		ipPattern := `^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`
		matched, _ := regexp.MatchString(ipPattern, value)
		if !matched {
			return fmt.Errorf("invalid IPv4 format")
		}
	case "phone":
		phonePattern := `^\+?[1-9]\d{1,14}$`
		matched, _ := regexp.MatchString(phonePattern, value)
		if !matched {
			return fmt.Errorf("invalid phone number format")
		}
	}
	return nil
}

func (a *APIValidatorExecutor) buildPath(basePath, field string) string {
	if basePath == "" {
		return field
	}
	return fmt.Sprintf("%s.%s", basePath, field)
}

// Composition validation methods

func (a *APIValidatorExecutor) validateComposition(value interface{}, schema *PropertySchema, path, fieldName string, result *ValidationResult, options ValidationOptions) {
	if len(schema.OneOf) > 0 {
		a.validateOneOf(value, schema.OneOf, path, fieldName, result, options)
	}
	if len(schema.AnyOf) > 0 {
		a.validateAnyOf(value, schema.AnyOf, path, fieldName, result, options)
	}
	if len(schema.AllOf) > 0 {
		a.validateAllOf(value, schema.AllOf, path, fieldName, result, options)
	}
	if schema.Not != nil {
		a.validateNot(value, schema.Not, path, fieldName, result, options)
	}
}

func (a *APIValidatorExecutor) validateOneOf(value interface{}, schemas []*PropertySchema, path, fieldName string, result *ValidationResult, options ValidationOptions) {
	matchCount := 0
	
	for _, schema := range schemas {
		tempResult := &ValidationResult{Valid: true, Stats: ValidationStats{}}
		a.validateProperty(value, schema, path, fieldName, tempResult, options)
		if tempResult.Valid {
			matchCount++
		}
	}
	
	if matchCount != 1 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:     path,
			Field:    fieldName,
			Message:  fmt.Sprintf("oneOf validation failed: matched %d schemas, expected exactly 1", matchCount),
			Code:     "ONEOF_VALIDATION_FAILED",
			Expected: "exactly 1 match",
			Actual:   matchCount,
			Severity: "error",
			Rule:     "composition_validation",
		})
		result.Stats.ErrorCount++
	}
}

func (a *APIValidatorExecutor) validateAnyOf(value interface{}, schemas []*PropertySchema, path, fieldName string, result *ValidationResult, options ValidationOptions) {
	for _, schema := range schemas {
		tempResult := &ValidationResult{Valid: true, Stats: ValidationStats{}}
		a.validateProperty(value, schema, path, fieldName, tempResult, options)
		if tempResult.Valid {
			return // At least one match found
		}
	}
	
	result.Valid = false
	result.Errors = append(result.Errors, ValidationError{
		Path:     path,
		Field:    fieldName,
		Message:  "anyOf validation failed: no schemas matched",
		Code:     "ANYOF_VALIDATION_FAILED",
		Expected: "at least 1 match",
		Actual:   0,
		Severity: "error",
		Rule:     "composition_validation",
	})
	result.Stats.ErrorCount++
}

func (a *APIValidatorExecutor) validateAllOf(value interface{}, schemas []*PropertySchema, path, fieldName string, result *ValidationResult, options ValidationOptions) {
	for i, schema := range schemas {
		tempResult := &ValidationResult{Valid: true, Stats: ValidationStats{}}
		a.validateProperty(value, schema, path, fieldName, tempResult, options)
		if !tempResult.Valid {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Path:     path,
				Field:    fieldName,
				Message:  fmt.Sprintf("allOf validation failed at schema index %d", i),
				Code:     "ALLOF_VALIDATION_FAILED",
				Expected: "all schemas to match",
				Actual:   fmt.Sprintf("schema %d failed", i),
				Severity: "error",
				Rule:     "composition_validation",
			})
			result.Stats.ErrorCount++
			return
		}
	}
}

func (a *APIValidatorExecutor) validateNot(value interface{}, schema *PropertySchema, path, fieldName string, result *ValidationResult, options ValidationOptions) {
	tempResult := &ValidationResult{Valid: true, Stats: ValidationStats{}}
	a.validateProperty(value, schema, path, fieldName, tempResult, options)
	
	if tempResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:     path,
			Field:    fieldName,
			Message:  "not validation failed: value matches the negated schema",
			Code:     "NOT_VALIDATION_FAILED",
			Expected: "schema not to match",
			Actual:   "schema matched",
			Severity: "error",
			Rule:     "composition_validation",
		})
		result.Stats.ErrorCount++
	}
}

// Conditional validation methods

func (a *APIValidatorExecutor) validateConditionalProperty(value interface{}, schema *PropertySchema, path, fieldName string, result *ValidationResult, options ValidationOptions) {
	if schema.If == nil {
		return
	}
	
	// Check if condition matches
	tempResult := &ValidationResult{Valid: true, Stats: ValidationStats{}}
	a.validateProperty(value, schema.If, path, fieldName, tempResult, options)
	
	if tempResult.Valid && schema.Then != nil {
		// Condition matched, apply "then" schema
		a.validateProperty(value, schema.Then, path, fieldName, result, options)
	} else if !tempResult.Valid && schema.Else != nil {
		// Condition didn't match, apply "else" schema
		a.validateProperty(value, schema.Else, path, fieldName, result, options)
	}
}

func (a *APIValidatorExecutor) applyConditionalValidation(data map[string]interface{}, conditional *ConditionalValidation, path string, result *ValidationResult, options ValidationOptions) {
	if fieldValue, exists := data[conditional.Field]; exists {
		if fieldValue == conditional.Value {
			// Condition met, apply conditional rules
			for _, required := range conditional.Required {
				if _, exists := data[required]; !exists {
					result.Valid = false
					fieldPath := a.buildPath(path, required)
					result.Errors = append(result.Errors, ValidationError{
						Path:     fieldPath,
						Field:    required,
						Message:  fmt.Sprintf("conditionally required field is missing (when %s = %v)", conditional.Field, conditional.Value),
						Code:     "CONDITIONAL_REQUIRED_FIELD_MISSING",
						Expected: "present",
						Actual:   "missing",
						Severity: "error",
						Rule:     "conditional_validation",
						Context: map[string]interface{}{
							"condition_field": conditional.Field,
							"condition_value": conditional.Value,
						},
					})
					result.Stats.ErrorCount++
				}
			}
			
			// Validate conditional properties
			for propName, propSchema := range conditional.Properties {
				if propValue, exists := data[propName]; exists {
					propPath := a.buildPath(path, propName)
					a.validateProperty(propValue, propSchema, propPath, propName, result, options)
				}
			}
		}
	}
}

// Custom validation methods

func (a *APIValidatorExecutor) registerBuiltinValidators() {
	a.customValidators["credit_card"] = func(value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("credit card number must be a string")
		}
		
		// Simple Luhn algorithm check
		return a.validateCreditCard(str)
	}
	
	a.customValidators["strong_password"] = func(value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("password must be a string")
		}
		
		return a.validateStrongPassword(str)
	}
	
	a.customValidators["business_email"] = func(value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("email must be a string")
		}
		
		return a.validateBusinessEmail(str)
	}
}

func (a *APIValidatorExecutor) applyCustomValidation(data map[string]interface{}, result *ValidationResult, options ValidationOptions) {
	// Apply global custom validation rules
	result.Stats.CustomRulesApplied++
}

func (a *APIValidatorExecutor) applyCustomPropertyValidation(value interface{}, customRule string, path, fieldName string, result *ValidationResult) {
	if validator, exists := a.customValidators[customRule]; exists {
		if err := validator(value); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Path:     path,
				Field:    fieldName,
				Message:  err.Error(),
				Code:     "CUSTOM_VALIDATION_FAILED",
				Expected: customRule,
				Actual:   value,
				Severity: "error",
				Rule:     customRule,
			})
			result.Stats.ErrorCount++
		}
		result.Stats.CustomRulesApplied++
	}
}

// Custom validator implementations

func (a *APIValidatorExecutor) validateCreditCard(number string) error {
	// Remove spaces and dashes
	number = strings.ReplaceAll(strings.ReplaceAll(number, " ", ""), "-", "")
	
	if len(number) < 13 || len(number) > 19 {
		return fmt.Errorf("credit card number must be 13-19 digits")
	}
	
	// Simple Luhn algorithm
	sum := 0
	alternate := false
	
	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0')
		if digit < 0 || digit > 9 {
			return fmt.Errorf("credit card number contains invalid characters")
		}
		
		if alternate {
			digit *= 2
			if digit > 9 {
				digit = digit%10 + digit/10
			}
		}
		
		sum += digit
		alternate = !alternate
	}
	
	if sum%10 != 0 {
		return fmt.Errorf("invalid credit card number (Luhn check failed)")
	}
	
	return nil
}

func (a *APIValidatorExecutor) validateStrongPassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}
	
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`\d`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[^a-zA-Z\d]`).MatchString(password)
	
	if !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("password must contain at least one digit")
	}
	if !hasSpecial {
		return fmt.Errorf("password must contain at least one special character")
	}
	
	return nil
}

func (a *APIValidatorExecutor) validateBusinessEmail(email string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("invalid email format: %v", err)
	}
	
	// Check for common personal email domains
	personalDomains := []string{"gmail.com", "yahoo.com", "hotmail.com", "outlook.com", "aol.com"}
	emailLower := strings.ToLower(email)
	
	for _, domain := range personalDomains {
		if strings.HasSuffix(emailLower, "@"+domain) {
			return fmt.Errorf("personal email domains are not allowed for business accounts")
		}
	}
	
	return nil
}

// Utility methods

func (a *APIValidatorExecutor) calculateSchemaComplexity(schema *ValidationSchema) int {
	complexity := 1 // Base complexity
	
	complexity += len(schema.Properties)
	complexity += len(schema.Required)
	
	for _, prop := range schema.Properties {
		complexity += a.calculatePropertyComplexity(prop)
	}
	
	return complexity
}

func (a *APIValidatorExecutor) calculatePropertyComplexity(prop *PropertySchema) int {
	complexity := 1
	
	if len(prop.Enum) > 0 {
		complexity++
	}
	if prop.Pattern != "" {
		complexity++
	}
	if prop.Format != "" {
		complexity++
	}
	if prop.Custom != "" {
		complexity += 2
	}
	
	complexity += len(prop.OneOf) + len(prop.AnyOf) + len(prop.AllOf)
	
	if prop.Not != nil {
		complexity++
	}
	
	return complexity
}

func (a *APIValidatorExecutor) categorizeErrors(errors []ValidationError) map[string][]ValidationError {
	categories := make(map[string][]ValidationError)
	
	for _, err := range errors {
		category := err.Rule
		if category == "" {
			category = "general"
		}
		categories[category] = append(categories[category], err)
	}
	
	return categories
}

func (a *APIValidatorExecutor) normalizeData(data map[string]interface{}, schema *ValidationSchema) map[string]interface{} {
	normalized := make(map[string]interface{})
	
	for key, value := range data {
		if propSchema, exists := schema.Properties[key]; exists {
			// Apply normalization based on property schema
			normalized[key] = a.normalizeValue(value, propSchema)
		} else {
			normalized[key] = value
		}
	}
	
	// Add default values for missing properties
	for propName, propSchema := range schema.Properties {
		if _, exists := normalized[propName]; !exists && propSchema.Default != nil {
			normalized[propName] = propSchema.Default
		}
	}
	
	return normalized
}

func (a *APIValidatorExecutor) normalizeValue(value interface{}, schema *PropertySchema) interface{} {
	// Simple normalization examples
	if schema.Type == "string" && value != nil {
		if strValue, ok := value.(string); ok {
			// Trim whitespace
			return strings.TrimSpace(strValue)
		}
	}
	
	return value
}

// CreateAPIValidatorTool creates and configures the API validator tool
func CreateAPIValidatorTool() *tools.Tool {
	return &tools.Tool{
		ID:          "api_validator",
		Name:        "Advanced API Validator",
		Description: "Comprehensive API request/response validation with complex schema support, conditional validation, and custom rules",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"validation_type": {
					Type:        "string",
					Description: "Type of validation to perform",
					Enum: []interface{}{
						"request", "response", "schema", "format",
					},
				},
				"data": {
					Type:        "object",
					Description: "Data to validate",
				},
				"schema": {
					Type:        "object",
					Description: "Validation schema definition",
					Properties: map[string]*tools.Property{
						"type": {
							Type: "string",
							Enum: []interface{}{"object"},
						},
						"properties": {
							Type: "object",
							AdditionalProperties: &[]bool{true}[0],
						},
						"required": {
							Type: "array",
							Items: &tools.Property{Type: "string"},
						},
						"additionalProperties": {
							Type:    "boolean",
							Default: true,
						},
					},
					Required: []string{"type"},
				},
				"options": {
					Type:        "object",
					Description: "Validation options",
					Properties: map[string]*tools.Property{
						"strict_mode": {
							Type:        "boolean",
							Description: "Enable strict validation mode",
							Default:     false,
						},
						"normalize_data": {
							Type:        "boolean",
							Description: "Return normalized data",
							Default:     false,
						},
						"include_warnings": {
							Type:        "boolean",
							Description: "Include validation warnings",
							Default:     true,
						},
						"max_errors": {
							Type:        "number",
							Description: "Maximum number of errors to collect",
							Minimum:     &[]float64{1}[0],
							Maximum:     &[]float64{1000}[0],
							Default:     100,
						},
						"custom_validation": {
							Type:        "boolean",
							Description: "Enable custom validation rules",
							Default:     true,
						},
					},
				},
			},
			Required: []string{"validation_type", "data", "schema"},
		},
		Metadata: &tools.ToolMetadata{
			Author:        "AG-UI SDK Examples",
			License:       "MIT",
			Documentation: "https://github.com/mattsp1290/ag-ui/blob/main/go-sdk/examples/tools/validation/README.md",
			Tags:          []string{"validation", "api", "schema", "json", "complex"},
			Examples: []tools.ToolExample{
				{
					Name:        "Basic Request Validation",
					Description: "Validate a simple API request",
					Input: map[string]interface{}{
						"validation_type": "request",
						"data": map[string]interface{}{
							"username": "john_doe",
							"email":    "john@example.com",
							"age":      25,
						},
						"schema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"username": map[string]interface{}{
									"type":      "string",
									"minLength": 3,
									"maxLength": 20,
									"pattern":   "^[a-zA-Z0-9_]+$",
								},
								"email": map[string]interface{}{
									"type":   "string",
									"format": "email",
								},
								"age": map[string]interface{}{
									"type":    "integer",
									"minimum": 18,
									"maximum": 120,
								},
							},
							"required": []string{"username", "email"},
						},
					},
				},
				{
					Name:        "Complex Conditional Validation",
					Description: "Validate with conditional requirements",
					Input: map[string]interface{}{
						"validation_type": "request",
						"data": map[string]interface{}{
							"account_type": "premium",
							"credit_limit": 10000,
						},
						"schema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"account_type": map[string]interface{}{
									"type": "string",
									"enum": []interface{}{"basic", "premium", "enterprise"},
								},
								"credit_limit": map[string]interface{}{
									"type":    "number",
									"minimum": 0,
								},
							},
							"conditional": map[string]interface{}{
								"field":    "account_type",
								"value":    "premium",
								"required": []string{"credit_limit"},
							},
						},
					},
				},
			},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      false,
			Cancelable: true,
			Retryable:  true,
			Cacheable:  true,
			Timeout:    30 * time.Second,
		},
		Executor: NewAPIValidatorExecutor(),
	}
}

func main() {
	// Create registry and register the API validator tool
	registry := tools.NewRegistry()
	apiValidatorTool := CreateAPIValidatorTool()

	if err := registry.Register(apiValidatorTool); err != nil {
		log.Fatalf("Failed to register API validator tool: %v", err)
	}

	// Create execution engine
	engine := tools.NewExecutionEngine(registry)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Shutdown(ctx); err != nil {
			log.Printf("Engine shutdown error: %v", err)
		}
	}()

	ctx := context.Background()

	fmt.Println("=== Advanced API Validator Tool Example ===")
	fmt.Println("Demonstrates: Complex schema validation, conditional logic, and custom validation rules")
	fmt.Println()

	// Example 1: Basic validation
	fmt.Println("1. Basic user registration validation...")
	result, err := engine.Execute(ctx, "api_validator", map[string]interface{}{
		"validation_type": "request",
		"data": map[string]interface{}{
			"username": "john_doe",
			"email":    "john@example.com",
			"age":      25,
			"password": "SecurePass123!",
		},
		"schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"username": map[string]interface{}{
					"type":      "string",
					"minLength": 3,
					"maxLength": 20,
					"pattern":   "^[a-zA-Z0-9_]+$",
				},
				"email": map[string]interface{}{
					"type":   "string",
					"format": "email",
				},
				"age": map[string]interface{}{
					"type":    "integer",
					"minimum": 18,
					"maximum": 120,
				},
				"password": map[string]interface{}{
					"type":   "string",
					"custom": "strong_password",
				},
			},
			"required":             []string{"username", "email", "password"},
			"additionalProperties": false,
		},
		"options": map[string]interface{}{
			"strict_mode": true,
		},
	})

	printValidationResult(result, err, "Basic validation")

	// Example 2: Validation with errors
	fmt.Println("2. Validation with multiple errors...")
	result, err = engine.Execute(ctx, "api_validator", map[string]interface{}{
		"validation_type": "request",
		"data": map[string]interface{}{
			"username": "jo", // Too short
			"email":    "invalid-email",
			"age":      15, // Too young
			// Missing required password
		},
		"schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"username": map[string]interface{}{
					"type":      "string",
					"minLength": 3,
					"maxLength": 20,
				},
				"email": map[string]interface{}{
					"type":   "string",
					"format": "email",
				},
				"age": map[string]interface{}{
					"type":    "integer",
					"minimum": 18,
				},
				"password": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"username", "email", "password"},
		},
	})

	printValidationResult(result, err, "Validation with errors")

	// Example 3: Conditional validation
	fmt.Println("3. Conditional validation example...")
	result, err = engine.Execute(ctx, "api_validator", map[string]interface{}{
		"validation_type": "request",
		"data": map[string]interface{}{
			"account_type": "premium",
			"credit_limit": 10000,
			"annual_fee":   199.99,
		},
		"schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"account_type": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"basic", "premium", "enterprise"},
				},
				"credit_limit": map[string]interface{}{
					"type":    "number",
					"minimum": 0,
				},
				"annual_fee": map[string]interface{}{
					"type":    "number",
					"minimum": 0,
				},
			},
			"conditional": map[string]interface{}{
				"field":    "account_type",
				"value":    "premium",
				"required": []string{"credit_limit", "annual_fee"},
			},
		},
	})

	printValidationResult(result, err, "Conditional validation")

	// Example 4: Complex composition validation
	fmt.Println("4. Complex composition validation (oneOf)...")
	result, err = engine.Execute(ctx, "api_validator", map[string]interface{}{
		"validation_type": "request",
		"data": map[string]interface{}{
			"payment_method": map[string]interface{}{
				"type":   "credit_card",
				"number": "4111111111111111",
				"expiry": "12/25",
				"cvv":    "123",
			},
		},
		"schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"payment_method": map[string]interface{}{
					"type": "object",
					"oneOf": []interface{}{
						map[string]interface{}{
							"properties": map[string]interface{}{
								"type": map[string]interface{}{
									"enum": []interface{}{"credit_card"},
								},
								"number": map[string]interface{}{
									"type":   "string",
									"custom": "credit_card",
								},
								"expiry": map[string]interface{}{
									"type":    "string",
									"pattern": "^(0[1-9]|1[0-2])/([0-9]{2})$",
								},
								"cvv": map[string]interface{}{
									"type":    "string",
									"pattern": "^[0-9]{3,4}$",
								},
							},
							"required": []string{"type", "number", "expiry", "cvv"},
						},
						map[string]interface{}{
							"properties": map[string]interface{}{
								"type": map[string]interface{}{
									"enum": []interface{}{"bank_transfer"},
								},
								"account_number": map[string]interface{}{
									"type": "string",
								},
								"routing_number": map[string]interface{}{
									"type": "string",
								},
							},
							"required": []string{"type", "account_number", "routing_number"},
						},
					},
				},
			},
			"required": []string{"payment_method"},
		},
	})

	printValidationResult(result, err, "Complex composition validation")
}

func printValidationResult(result *tools.ToolExecutionResult, err error, title string) {
	fmt.Printf("=== %s ===\n", title)
	
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		fmt.Println()
		return
	}

	if !result.Success {
		fmt.Printf("  Failed: %s\n", result.Error)
		fmt.Println()
		return
	}

	data := result.Data.(map[string]interface{})
	summary := data["summary"].(map[string]interface{})
	
	fmt.Printf("  Valid: %v\n", summary["valid"])
	fmt.Printf("  Errors: %v\n", summary["error_count"])
	fmt.Printf("  Warnings: %v\n", summary["warning_count"])
	fmt.Printf("  Fields validated: %v\n", summary["fields_validated"])
	fmt.Printf("  Validation time: %vms\n", summary["validation_time_ms"])

	if errorDetails, exists := data["error_details"]; exists {
		fmt.Printf("  Error details:\n")
		errorMap := errorDetails.(map[string]interface{})
		for category, errors := range errorMap {
			errorList := errors.([]interface{})
			fmt.Printf("    %s: %d errors\n", category, len(errorList))
			for i, errInterface := range errorList {
				if i < 3 { // Show first 3 errors
					errMap := errInterface.(map[string]interface{})
					fmt.Printf("      - %s: %s\n", errMap["field"], errMap["message"])
				}
			}
			if len(errorList) > 3 {
				fmt.Printf("      ... and %d more\n", len(errorList)-3)
			}
		}
	}

	fmt.Printf("  Duration: %v\n", result.Duration)
	fmt.Println()
}