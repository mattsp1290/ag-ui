package transform

import (
	"context"
	"fmt"
	"reflect"
)

// Validator interface for data validation
type Validator interface {
	// Validate validates data and returns error if invalid
	Validate(ctx context.Context, data interface{}) error

	// Name returns validator name
	Name() string
}

// ValidationTransformer validates request/response data
type ValidationTransformer struct {
	*BaseTransformer
	requestValidators  []Validator
	responseValidators []Validator
}

// NewValidationTransformer creates a new validation transformer
func NewValidationTransformer() *ValidationTransformer {
	return &ValidationTransformer{
		BaseTransformer:    NewBaseTransformer("validation", TransformationBoth),
		requestValidators:  make([]Validator, 0),
		responseValidators: make([]Validator, 0),
	}
}

// AddRequestValidator adds a request validator
func (vt *ValidationTransformer) AddRequestValidator(validator Validator) {
	vt.requestValidators = append(vt.requestValidators, validator)
}

// AddResponseValidator adds a response validator
func (vt *ValidationTransformer) AddResponseValidator(validator Validator) {
	vt.responseValidators = append(vt.responseValidators, validator)
}

// TransformRequest validates request data
func (vt *ValidationTransformer) TransformRequest(ctx context.Context, req *Request) error {
	if !vt.Enabled() {
		return nil
	}

	for _, validator := range vt.requestValidators {
		if err := validator.Validate(ctx, req.Body); err != nil {
			return fmt.Errorf("request validation failed in %s: %w", validator.Name(), err)
		}
	}

	return nil
}

// TransformResponse validates response data
func (vt *ValidationTransformer) TransformResponse(ctx context.Context, resp *Response) error {
	if !vt.Enabled() {
		return nil
	}

	for _, validator := range vt.responseValidators {
		if err := validator.Validate(ctx, resp.Body); err != nil {
			return fmt.Errorf("response validation failed in %s: %w", validator.Name(), err)
		}
	}

	return nil
}

// JSONSchemaValidator validates data against JSON schema (simplified)
type JSONSchemaValidator struct {
	name   string
	schema map[string]interface{}
}

// NewJSONSchemaValidator creates a new JSON schema validator
func NewJSONSchemaValidator(name string, schema map[string]interface{}) *JSONSchemaValidator {
	return &JSONSchemaValidator{
		name:   name,
		schema: schema,
	}
}

// Name returns validator name
func (jsv *JSONSchemaValidator) Name() string {
	return jsv.name
}

// Validate validates data against schema (simplified implementation)
func (jsv *JSONSchemaValidator) Validate(ctx context.Context, data interface{}) error {
	// This is a simplified validation - in production, use a proper JSON schema library
	if data == nil {
		return fmt.Errorf("data cannot be nil")
	}

	// Basic type validation
	if expectedType, ok := jsv.schema["type"].(string); ok {
		actualType := reflect.TypeOf(data).Kind().String()
		if actualType != expectedType && !jsv.isCompatibleType(expectedType, actualType) {
			return fmt.Errorf("expected type %s, got %s", expectedType, actualType)
		}
	}

	// Required fields validation
	if requiredFields, ok := jsv.schema["required"].([]interface{}); ok {
		if dataMap, ok := data.(map[string]interface{}); ok {
			for _, field := range requiredFields {
				if fieldName, ok := field.(string); ok {
					if _, exists := dataMap[fieldName]; !exists {
						return fmt.Errorf("required field '%s' is missing", fieldName)
					}
				}
			}
		}
	}

	return nil
}

// isCompatibleType checks if types are compatible
func (jsv *JSONSchemaValidator) isCompatibleType(expected, actual string) bool {
	compatibleTypes := map[string][]string{
		"object":  {"map"},
		"array":   {"slice"},
		"string":  {"string"},
		"number":  {"int", "int64", "float64", "float32"},
		"boolean": {"bool"},
	}

	if compatible, ok := compatibleTypes[expected]; ok {
		for _, t := range compatible {
			if t == actual {
				return true
			}
		}
	}

	return false
}
