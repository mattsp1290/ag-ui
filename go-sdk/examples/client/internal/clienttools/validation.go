package clienttools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// ValidationError represents a detailed validation error
type ValidationError struct {
	ToolName   string
	Parameter  string
	Value      interface{}
	Expected   string
	Message    string
	Suggestion string
}

func (e *ValidationError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Validation error for tool '%s'", e.ToolName))
	if e.Parameter != "" {
		sb.WriteString(fmt.Sprintf(", parameter '%s'", e.Parameter))
	}
	sb.WriteString(": ")
	sb.WriteString(e.Message)
	if e.Expected != "" {
		sb.WriteString(fmt.Sprintf(" (expected: %s", e.Expected))
		if e.Value != nil {
			sb.WriteString(fmt.Sprintf(", got: %v", e.Value))
		}
		sb.WriteString(")")
	}
	if e.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("\n  💡 Suggestion: %s", e.Suggestion))
	}
	return sb.String()
}

// ValidateToolArguments validates tool arguments against the tool's parameter schema
func ValidateToolArguments(toolName string, params *ToolParameters, args map[string]interface{}) error {
	if params == nil {
		// No parameters defined means any arguments are accepted
		return nil
	}

	// Check for required parameters
	for _, required := range params.Required {
		if _, exists := args[required]; !exists {
			return &ValidationError{
				ToolName:   toolName,
				Parameter:  required,
				Message:    "required parameter is missing",
				Suggestion: fmt.Sprintf("Add the '%s' parameter to your arguments", required),
			}
		}
	}

	// Validate each provided argument
	for paramName, value := range args {
		paramDef, exists := params.Properties[paramName]
		if !exists {
			// Extra parameters might be allowed depending on the tool
			continue
		}

		// Validate parameter type
		if err := validateParameterType(toolName, paramName, value, paramDef); err != nil {
			return err
		}

		// Validate enum values if specified
		if len(paramDef.Enum) > 0 {
			if err := validateEnumValue(toolName, paramName, value, paramDef.Enum); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateParameterType validates that a parameter value matches its expected type
func validateParameterType(toolName, paramName string, value interface{}, paramDef ParameterDef) error {
	expectedType := paramDef.Type
	actualType := getJSONType(value)

	// Check if types match
	if !isTypeCompatible(actualType, expectedType) {
		return &ValidationError{
			ToolName:   toolName,
			Parameter:  paramName,
			Value:      value,
			Expected:   expectedType,
			Message:    fmt.Sprintf("parameter has wrong type: expected %s, got %s", expectedType, actualType),
			Suggestion: fmt.Sprintf("Convert the value to %s type", expectedType),
		}
	}

	// Additional validation for specific types
	switch expectedType {
	case "integer":
		if !isInteger(value) {
			return &ValidationError{
				ToolName:   toolName,
				Parameter:  paramName,
				Value:      value,
				Expected:   "integer",
				Message:    "value must be a whole number",
				Suggestion: "Use a whole number without decimal points",
			}
		}
	case "string":
		if str, ok := value.(string); ok && str == "" && paramDef.Default == nil {
			return &ValidationError{
				ToolName:   toolName,
				Parameter:  paramName,
				Value:      value,
				Expected:   "non-empty string",
				Message:    "string parameter cannot be empty",
				Suggestion: "Provide a non-empty value for this parameter",
			}
		}
	}

	return nil
}

// validateEnumValue validates that a value is one of the allowed enum values
func validateEnumValue(toolName, paramName string, value interface{}, enumValues []string) error {
	valueStr := fmt.Sprintf("%v", value)
	for _, allowed := range enumValues {
		if valueStr == allowed {
			return nil
		}
	}

	return &ValidationError{
		ToolName:   toolName,
		Parameter:  paramName,
		Value:      value,
		Expected:   fmt.Sprintf("one of: %s", strings.Join(enumValues, ", ")),
		Message:    "value is not in the allowed list",
		Suggestion: fmt.Sprintf("Use one of the allowed values: %s", strings.Join(enumValues, ", ")),
	}
}

// ConvertToToolSchema converts ToolParameters to tools.ToolSchema for validation
func ConvertToToolSchema(params *ToolParameters) *tools.ToolSchema {
	if params == nil {
		return nil
	}

	schema := &tools.ToolSchema{
		Type:     params.Type,
		Required: params.Required,
	}

	if params.Properties != nil {
		schema.Properties = make(map[string]*tools.Property)
		for name, def := range params.Properties {
			prop := &tools.Property{
				Type:        def.Type,
				Description: def.Description,
			}
			if def.Default != nil {
				prop.Default = def.Default
			}
			if len(def.Enum) > 0 {
				prop.Enum = make([]interface{}, len(def.Enum))
				for i, v := range def.Enum {
					prop.Enum[i] = v
				}
			}
			schema.Properties[name] = prop
		}
	}

	return schema
}

// ValidateWithSDK validates arguments using the SDK's validation functionality
func ValidateWithSDK(toolName string, params *ToolParameters, args map[string]interface{}) error {
	if params == nil {
		return nil
	}

	// Convert to SDK schema format
	schema := ConvertToToolSchema(params)

	// Marshal arguments to JSON for SDK validation
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return &ValidationError{
			ToolName:   toolName,
			Message:    fmt.Sprintf("failed to marshal arguments: %v", err),
			Suggestion: "Ensure arguments are valid JSON-serializable values",
		}
	}

	// Use SDK validation
	if err := tools.ValidateArguments(argsJSON, schema); err != nil {
		// Wrap SDK error with our error type for consistency
		return &ValidationError{
			ToolName:   toolName,
			Message:    err.Error(),
			Suggestion: "Check the tool's parameter requirements and adjust your arguments",
		}
	}

	return nil
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
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		// Use reflection for other types
		rt := reflect.TypeOf(value)
		switch rt.Kind() {
		case reflect.Slice, reflect.Array:
			return "array"
		case reflect.Map, reflect.Struct:
			return "object"
		case reflect.Bool:
			return "boolean"
		case reflect.String:
			return "string"
		case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return "number"
		default:
			return "unknown"
		}
	}
}

// isTypeCompatible checks if an actual type is compatible with expected type
func isTypeCompatible(actualType, expectedType string) bool {
	if actualType == expectedType {
		return true
	}

	// Allow number types to match integer requirement (will validate separately)
	if expectedType == "integer" && actualType == "number" {
		return true
	}

	// Allow null for optional parameters
	if actualType == "null" {
		return true
	}

	return false
}

// isInteger checks if a value is an integer
func isInteger(value interface{}) bool {
	switch v := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return v == float32(int(v))
	case float64:
		return v == float64(int(v))
	default:
		return false
	}
}

// SuggestCorrection provides helpful suggestions for common validation errors
func SuggestCorrection(err error) string {
	if validErr, ok := err.(*ValidationError); ok {
		if validErr.Suggestion != "" {
			return validErr.Suggestion
		}
	}

	// Provide generic suggestions based on error message
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "required"):
		return "Check the tool documentation for required parameters"
	case strings.Contains(errMsg, "type"):
		return "Ensure parameter types match the expected format"
	case strings.Contains(errMsg, "enum"):
		return "Use one of the predefined allowed values"
	default:
		return "Review the tool's parameter requirements"
	}
}