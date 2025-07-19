package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

// DataTransformerExecutor implements comprehensive data transformation with validation.
// This example demonstrates transformation pipelines, data type coercion,
// validation during transformation, and complex data manipulation.
type DataTransformerExecutor struct {
	transformers map[string]TransformFunction
	validators   map[string]ValidationFunction
}

// TransformFunction defines a data transformation function
type TransformFunction func(value interface{}, config map[string]interface{}) (interface{}, error)

// ValidationFunction defines a validation function for transformed data
type ValidationFunction func(value interface{}, config map[string]interface{}) error

// TransformationPipeline represents a series of transformations to apply
type TransformationPipeline struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	Steps        []TransformationStep     `json:"steps"`
	Validation   *ValidationConfig        `json:"validation,omitempty"`
	Options      *PipelineOptions         `json:"options,omitempty"`
}

// TransformationStep represents a single transformation step
type TransformationStep struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Condition   *ConditionConfig       `json:"condition,omitempty"`
	ErrorPolicy string                 `json:"error_policy,omitempty"` // "stop", "skip", "default"
	Default     interface{}            `json:"default,omitempty"`
}

// ValidationConfig defines validation rules for transformed data
type ValidationConfig struct {
	Rules         []ValidationRule       `json:"rules"`
	StrictMode    bool                   `json:"strict_mode"`
	FailOnFirst   bool                   `json:"fail_on_first"`
	CustomRules   map[string]interface{} `json:"custom_rules,omitempty"`
}

// ValidationRule defines a single validation rule
type ValidationRule struct {
	Field       string                 `json:"field"`
	Type        string                 `json:"type"`
	Required    bool                   `json:"required"`
	Constraints map[string]interface{} `json:"constraints,omitempty"`
	Message     string                 `json:"message,omitempty"`
}

// ConditionConfig defines when a transformation step should be applied
type ConditionConfig struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // "equals", "not_equals", "exists", "not_exists", "contains"
	Value    interface{} `json:"value,omitempty"`
}

// PipelineOptions configures pipeline behavior
type PipelineOptions struct {
	IgnoreNull        bool   `json:"ignore_null"`
	PreserveStructure bool   `json:"preserve_structure"`
	StrictTypes       bool   `json:"strict_types"`
	ErrorHandling     string `json:"error_handling"` // "strict", "lenient", "collect"
	MaxErrors         int    `json:"max_errors"`
}

// TransformationResult represents the result of a transformation pipeline
type TransformationResult struct {
	Success        bool                   `json:"success"`
	TransformedData interface{}           `json:"transformed_data,omitempty"`
	OriginalData   interface{}           `json:"original_data,omitempty"`
	Errors         []TransformationError  `json:"errors,omitempty"`
	Warnings       []TransformationError  `json:"warnings,omitempty"`
	Statistics     TransformationStats    `json:"statistics"`
	StepResults    []StepResult          `json:"step_results,omitempty"`
}

// TransformationError represents an error during transformation
type TransformationError struct {
	Step        string      `json:"step"`
	Field       string      `json:"field"`
	Message     string      `json:"message"`
	ErrorType   string      `json:"error_type"`
	Value       interface{} `json:"value,omitempty"`
	Expected    interface{} `json:"expected,omitempty"`
	Severity    string      `json:"severity"` // "error", "warning"
}

// TransformationStats provides statistics about the transformation
type TransformationStats struct {
	StepsExecuted      int           `json:"steps_executed"`
	FieldsTransformed  int           `json:"fields_transformed"`
	ErrorCount         int           `json:"error_count"`
	WarningCount       int           `json:"warning_count"`
	TransformationTime time.Duration `json:"transformation_time"`
	ValidationTime     time.Duration `json:"validation_time"`
}

// StepResult represents the result of a single transformation step
type StepResult struct {
	StepName    string      `json:"step_name"`
	Success     bool        `json:"success"`
	Input       interface{} `json:"input,omitempty"`
	Output      interface{} `json:"output,omitempty"`
	Error       string      `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
	Applied     bool        `json:"applied"` // Whether the step was actually applied (conditions)
}

// NewDataTransformerExecutor creates a new data transformer executor
func NewDataTransformerExecutor() *DataTransformerExecutor {
	executor := &DataTransformerExecutor{
		transformers: make(map[string]TransformFunction),
		validators:   make(map[string]ValidationFunction),
	}

	// Register built-in transformers
	executor.registerBuiltinTransformers()
	
	// Register built-in validators
	executor.registerBuiltinValidators()
	
	return executor
}

// Execute performs data transformation based on the provided parameters
func (d *DataTransformerExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	startTime := time.Now()

	// Extract parameters
	data, exists := params["data"]
	if !exists {
		return nil, fmt.Errorf("data parameter is required")
	}

	pipelineParam, exists := params["pipeline"]
	if !exists {
		return nil, fmt.Errorf("pipeline parameter is required")
	}

	// Parse pipeline configuration
	pipeline, err := d.parsePipeline(pipelineParam)
	if err != nil {
		return nil, fmt.Errorf("invalid pipeline configuration: %w", err)
	}

	// Execute transformation pipeline
	result := d.executePipeline(ctx, data, pipeline)
	result.Statistics.TransformationTime = time.Since(startTime)

	// Prepare response
	responseData := map[string]interface{}{
		"transformation_result": result,
		"summary": map[string]interface{}{
			"success":              result.Success,
			"steps_executed":       result.Statistics.StepsExecuted,
			"fields_transformed":   result.Statistics.FieldsTransformed,
			"error_count":          result.Statistics.ErrorCount,
			"warning_count":        result.Statistics.WarningCount,
			"transformation_time_ms": result.Statistics.TransformationTime.Milliseconds(),
		},
	}

	if result.TransformedData != nil {
		responseData["transformed_data"] = result.TransformedData
	}

	if len(result.Errors) > 0 {
		responseData["error_details"] = d.categorizeErrors(result.Errors)
	}

	if len(result.Warnings) > 0 {
		responseData["warning_details"] = d.categorizeErrors(result.Warnings)
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    responseData,
		Timestamp: time.Now(),
		Duration: time.Since(startTime),
		Metadata: map[string]interface{}{
			"pipeline_name":    pipeline.Name,
			"pipeline_steps":   len(pipeline.Steps),
			"processed_at":     time.Now().Format(time.RFC3339),
			"performance": map[string]interface{}{
				"transformation_time_ns": result.Statistics.TransformationTime.Nanoseconds(),
				"fields_per_second":      float64(result.Statistics.FieldsTransformed) / result.Statistics.TransformationTime.Seconds(),
			},
		},
	}, nil
}

// parsePipeline converts the pipeline parameter to a TransformationPipeline
func (d *DataTransformerExecutor) parsePipeline(pipelineParam interface{}) (*TransformationPipeline, error) {
	pipelineBytes, err := json.Marshal(pipelineParam)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pipeline: %w", err)
	}

	var pipeline TransformationPipeline
	if err := json.Unmarshal(pipelineBytes, &pipeline); err != nil {
		return nil, fmt.Errorf("failed to parse pipeline: %w", err)
	}

	// Set default options
	if pipeline.Options == nil {
		pipeline.Options = &PipelineOptions{
			IgnoreNull:        false,
			PreserveStructure: true,
			StrictTypes:       false,
			ErrorHandling:     "lenient",
			MaxErrors:         100,
		}
	}

	return &pipeline, nil
}

// executePipeline executes the transformation pipeline
func (d *DataTransformerExecutor) executePipeline(ctx context.Context, data interface{}, pipeline *TransformationPipeline) *TransformationResult {
	result := &TransformationResult{
		Success:        true,
		OriginalData:   data,
		TransformedData: data,
		Errors:         []TransformationError{},
		Warnings:       []TransformationError{},
		Statistics:     TransformationStats{},
		StepResults:    []StepResult{},
	}

	currentData := data

	// Execute each step in the pipeline
	for _, step := range pipeline.Steps {
		stepStart := time.Now()
		stepResult := StepResult{
			StepName: step.Name,
			Input:    currentData,
		}

		// Check if step should be applied based on condition
		if step.Condition != nil {
			if !d.evaluateCondition(currentData, step.Condition) {
				stepResult.Success = true
				stepResult.Applied = false
				stepResult.Output = currentData
				stepResult.Duration = time.Since(stepStart)
				result.StepResults = append(result.StepResults, stepResult)
				continue
			}
		}

		stepResult.Applied = true

		// Apply transformation
		transformedData, err := d.applyTransformation(currentData, &step, pipeline.Options)
		stepResult.Duration = time.Since(stepStart)
		
		if err != nil {
			stepResult.Success = false
			stepResult.Error = err.Error()
			
			// Handle error based on policy
			switch step.ErrorPolicy {
			case "stop":
				result.Success = false
				d.addError(result, step.Name, "", err.Error(), "TRANSFORMATION_ERROR", currentData, nil, "error")
				result.StepResults = append(result.StepResults, stepResult)
				return result
			case "skip":
				d.addError(result, step.Name, "", err.Error(), "TRANSFORMATION_SKIPPED", currentData, nil, "warning")
				stepResult.Output = currentData
			case "default":
				if step.Default != nil {
					transformedData = step.Default
					stepResult.Success = true
					stepResult.Output = transformedData
					d.addError(result, step.Name, "", "used default value due to error: "+err.Error(), "DEFAULT_VALUE_USED", currentData, step.Default, "warning")
				} else {
					stepResult.Output = currentData
					d.addError(result, step.Name, "", err.Error(), "TRANSFORMATION_ERROR", currentData, nil, "error")
				}
			default: // Continue with original data
				stepResult.Output = currentData
				d.addError(result, step.Name, "", err.Error(), "TRANSFORMATION_ERROR", currentData, nil, "error")
			}
		} else {
			stepResult.Success = true
			stepResult.Output = transformedData
			currentData = transformedData
			result.Statistics.FieldsTransformed++
		}

		result.StepResults = append(result.StepResults, stepResult)
		result.Statistics.StepsExecuted++

		// Check error limits
		if pipeline.Options.ErrorHandling == "strict" && len(result.Errors) > 0 {
			result.Success = false
			return result
		}

		if len(result.Errors) >= pipeline.Options.MaxErrors {
			result.Success = false
			d.addError(result, "pipeline", "", "maximum error count exceeded", "MAX_ERRORS_EXCEEDED", nil, pipeline.Options.MaxErrors, "error")
			return result
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			result.Success = false
			d.addError(result, "pipeline", "", "transformation cancelled", "CANCELLED", nil, nil, "error")
			return result
		default:
		}
	}

	result.TransformedData = currentData

	// Perform validation if configured
	if pipeline.Validation != nil {
		validationStart := time.Now()
		d.validateTransformedData(currentData, pipeline.Validation, result)
		result.Statistics.ValidationTime = time.Since(validationStart)
	}

	// Determine overall success
	if pipeline.Options.ErrorHandling == "strict" && len(result.Errors) > 0 {
		result.Success = false
	}

	return result
}

// applyTransformation applies a single transformation step
func (d *DataTransformerExecutor) applyTransformation(data interface{}, step *TransformationStep, options *PipelineOptions) (interface{}, error) {
	transformer, exists := d.transformers[step.Type]
	if !exists {
		return nil, fmt.Errorf("unknown transformation type: %s", step.Type)
	}

	return transformer(data, step.Config)
}

// evaluateCondition evaluates whether a condition is met
func (d *DataTransformerExecutor) evaluateCondition(data interface{}, condition *ConditionConfig) bool {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return false
	}

	fieldValue, exists := dataMap[condition.Field]

	switch condition.Operator {
	case "exists":
		return exists
	case "not_exists":
		return !exists
	case "equals":
		return exists && fieldValue == condition.Value
	case "not_equals":
		return !exists || fieldValue != condition.Value
	case "contains":
		if str, ok := fieldValue.(string); ok {
			if searchStr, ok := condition.Value.(string); ok {
				return strings.Contains(str, searchStr)
			}
		}
		return false
	default:
		return false
	}
}

// validateTransformedData validates the final transformed data
func (d *DataTransformerExecutor) validateTransformedData(data interface{}, validation *ValidationConfig, result *TransformationResult) {
	for _, rule := range validation.Rules {
		if err := d.validateField(data, &rule); err != nil {
			severity := "error"
			if !validation.StrictMode {
				severity = "warning"
			}
			
			d.addError(result, "validation", rule.Field, err.Error(), "VALIDATION_ERROR", data, nil, severity)
			
			if validation.FailOnFirst {
				result.Success = false
				return
			}
		}
	}
}

// validateField validates a single field according to a validation rule
func (d *DataTransformerExecutor) validateField(data interface{}, rule *ValidationRule) error {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("data is not an object")
	}

	fieldValue, exists := dataMap[rule.Field]

	if rule.Required && !exists {
		return fmt.Errorf("required field '%s' is missing", rule.Field)
	}

	if !exists {
		return nil // Optional field not present
	}

	// Type validation
	if !d.validateFieldType(fieldValue, rule.Type) {
		return fmt.Errorf("field '%s' has incorrect type, expected %s", rule.Field, rule.Type)
	}

	// Constraint validation
	if err := d.validateFieldConstraints(fieldValue, rule.Constraints); err != nil {
		return fmt.Errorf("field '%s' constraint validation failed: %w", rule.Field, err)
	}

	return nil
}

// validateFieldType validates the type of a field value
func (d *DataTransformerExecutor) validateFieldType(value interface{}, expectedType string) bool {
	if value == nil {
		return expectedType == "null"
	}

	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		return d.isNumber(value)
	case "integer":
		return d.isInteger(value)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	default:
		return true // Unknown types are considered valid
	}
}

// validateFieldConstraints validates field constraints
func (d *DataTransformerExecutor) validateFieldConstraints(value interface{}, constraints map[string]interface{}) error {
	if constraints == nil {
		return nil
	}

	// Length constraints for strings and arrays
	if minLen, exists := constraints["minLength"]; exists {
		if minLenFloat, ok := minLen.(float64); ok {
			length := d.getLength(value)
			if length < int(minLenFloat) {
				return fmt.Errorf("length %d is less than minimum %d", length, int(minLenFloat))
			}
		}
	}

	if maxLen, exists := constraints["maxLength"]; exists {
		if maxLenFloat, ok := maxLen.(float64); ok {
			length := d.getLength(value)
			if length > int(maxLenFloat) {
				return fmt.Errorf("length %d exceeds maximum %d", length, int(maxLenFloat))
			}
		}
	}

	// Range constraints for numbers
	if min, exists := constraints["minimum"]; exists {
		if minFloat, ok := min.(float64); ok {
			if valueFloat, ok := d.toFloat64(value); ok {
				if valueFloat < minFloat {
					return fmt.Errorf("value %v is less than minimum %v", valueFloat, minFloat)
				}
			}
		}
	}

	if max, exists := constraints["maximum"]; exists {
		if maxFloat, ok := max.(float64); ok {
			if valueFloat, ok := d.toFloat64(value); ok {
				if valueFloat > maxFloat {
					return fmt.Errorf("value %v exceeds maximum %v", valueFloat, maxFloat)
				}
			}
		}
	}

	return nil
}

// addError adds an error to the transformation result
func (d *DataTransformerExecutor) addError(result *TransformationResult, step, field, message, errorType string, value, expected interface{}, severity string) {
	error := TransformationError{
		Step:      step,
		Field:     field,
		Message:   message,
		ErrorType: errorType,
		Value:     value,
		Expected:  expected,
		Severity:  severity,
	}

	if severity == "error" {
		result.Errors = append(result.Errors, error)
		result.Statistics.ErrorCount++
	} else {
		result.Warnings = append(result.Warnings, error)
		result.Statistics.WarningCount++
	}
}

// Helper methods

func (d *DataTransformerExecutor) isNumber(value interface{}) bool {
	_, ok := d.toFloat64(value)
	return ok
}

func (d *DataTransformerExecutor) isInteger(value interface{}) bool {
	if f, ok := d.toFloat64(value); ok {
		return f == float64(int64(f))
	}
	return false
}

func (d *DataTransformerExecutor) toFloat64(value interface{}) (float64, bool) {
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

func (d *DataTransformerExecutor) getLength(value interface{}) int {
	switch v := value.(type) {
	case string:
		return len(v)
	case []interface{}:
		return len(v)
	case map[string]interface{}:
		return len(v)
	default:
		return 0
	}
}

func (d *DataTransformerExecutor) categorizeErrors(errors []TransformationError) map[string][]TransformationError {
	categories := make(map[string][]TransformationError)
	
	for _, err := range errors {
		category := err.ErrorType
		if category == "" {
			category = "general"
		}
		categories[category] = append(categories[category], err)
	}
	
	return categories
}

// Built-in transformers

func (d *DataTransformerExecutor) registerBuiltinTransformers() {
	// String transformers
	d.transformers["lowercase"] = d.transformLowercase
	d.transformers["uppercase"] = d.transformUppercase
	d.transformers["trim"] = d.transformTrim
	d.transformers["replace"] = d.transformReplace
	d.transformers["substring"] = d.transformSubstring
	d.transformers["split"] = d.transformSplit
	d.transformers["join"] = d.transformJoin

	// Type conversion transformers
	d.transformers["to_string"] = d.transformToString
	d.transformers["to_number"] = d.transformToNumber
	d.transformers["to_integer"] = d.transformToInteger
	d.transformers["to_boolean"] = d.transformToBoolean
	d.transformers["to_array"] = d.transformToArray

	// Date/time transformers
	d.transformers["parse_date"] = d.transformParseDate
	d.transformers["format_date"] = d.transformFormatDate
	d.transformers["add_time"] = d.transformAddTime

	// Object transformers
	d.transformers["rename_field"] = d.transformRenameField
	d.transformers["remove_field"] = d.transformRemoveField
	d.transformers["add_field"] = d.transformAddField
	d.transformers["merge_objects"] = d.transformMergeObjects
	d.transformers["pick_fields"] = d.transformPickFields
	d.transformers["omit_fields"] = d.transformOmitFields

	// Array transformers
	d.transformers["map_array"] = d.transformMapArray
	d.transformers["filter_array"] = d.transformFilterArray
	d.transformers["sort_array"] = d.transformSortArray
	d.transformers["unique_array"] = d.transformUniqueArray

	// Mathematical transformers
	d.transformers["add"] = d.transformAdd
	d.transformers["subtract"] = d.transformSubtract
	d.transformers["multiply"] = d.transformMultiply
	d.transformers["divide"] = d.transformDivide
	d.transformers["round"] = d.transformRound

	// Conditional transformers
	d.transformers["if_then_else"] = d.transformIfThenElse
	d.transformers["coalesce"] = d.transformCoalesce
	d.transformers["default_value"] = d.transformDefaultValue
}

// String transformation implementations
func (d *DataTransformerExecutor) transformLowercase(value interface{}, config map[string]interface{}) (interface{}, error) {
	str, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("value must be a string")
	}
	return strings.ToLower(str), nil
}

func (d *DataTransformerExecutor) transformUppercase(value interface{}, config map[string]interface{}) (interface{}, error) {
	str, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("value must be a string")
	}
	return strings.ToUpper(str), nil
}

func (d *DataTransformerExecutor) transformTrim(value interface{}, config map[string]interface{}) (interface{}, error) {
	str, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("value must be a string")
	}
	
	cutset := " \t\n\r"
	if customCutset, exists := config["cutset"]; exists {
		if cutsetStr, ok := customCutset.(string); ok {
			cutset = cutsetStr
		}
	}
	
	return strings.Trim(str, cutset), nil
}

func (d *DataTransformerExecutor) transformReplace(value interface{}, config map[string]interface{}) (interface{}, error) {
	str, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("value must be a string")
	}

	old, exists := config["old"]
	if !exists {
		return nil, fmt.Errorf("'old' parameter is required")
	}
	oldStr, ok := old.(string)
	if !ok {
		return nil, fmt.Errorf("'old' parameter must be a string")
	}

	new, exists := config["new"]
	if !exists {
		return nil, fmt.Errorf("'new' parameter is required")
	}
	newStr, ok := new.(string)
	if !ok {
		return nil, fmt.Errorf("'new' parameter must be a string")
	}

	count := -1 // Replace all by default
	if countParam, exists := config["count"]; exists {
		if countFloat, ok := countParam.(float64); ok {
			count = int(countFloat)
		}
	}

	return strings.Replace(str, oldStr, newStr, count), nil
}

// Type conversion implementations
func (d *DataTransformerExecutor) transformToString(value interface{}, config map[string]interface{}) (interface{}, error) {
	return fmt.Sprintf("%v", value), nil
}

func (d *DataTransformerExecutor) transformToNumber(value interface{}, config map[string]interface{}) (interface{}, error) {
	if f, ok := d.toFloat64(value); ok {
		return f, nil
	}
	return nil, fmt.Errorf("cannot convert %v to number", value)
}

func (d *DataTransformerExecutor) transformToInteger(value interface{}, config map[string]interface{}) (interface{}, error) {
	if f, ok := d.toFloat64(value); ok {
		return int64(f), nil
	}
	return nil, fmt.Errorf("cannot convert %v to integer", value)
}

func (d *DataTransformerExecutor) transformToBoolean(value interface{}, config map[string]interface{}) (interface{}, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		lowerStr := strings.ToLower(v)
		return lowerStr == "true" || lowerStr == "1" || lowerStr == "yes" || lowerStr == "on", nil
	case float64:
		return v != 0, nil
	case int:
		return v != 0, nil
	default:
		return false, nil
	}
}

func (d *DataTransformerExecutor) transformToArray(value interface{}, config map[string]interface{}) (interface{}, error) {
	if arr, ok := value.([]interface{}); ok {
		return arr, nil
	}
	return []interface{}{value}, nil
}

// Object transformation implementations
func (d *DataTransformerExecutor) transformRenameField(value interface{}, config map[string]interface{}) (interface{}, error) {
	obj, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value must be an object")
	}

	from, exists := config["from"]
	if !exists {
		return nil, fmt.Errorf("'from' parameter is required")
	}
	fromStr, ok := from.(string)
	if !ok {
		return nil, fmt.Errorf("'from' parameter must be a string")
	}

	to, exists := config["to"]
	if !exists {
		return nil, fmt.Errorf("'to' parameter is required")
	}
	toStr, ok := to.(string)
	if !ok {
		return nil, fmt.Errorf("'to' parameter must be a string")
	}

	result := make(map[string]interface{})
	for k, v := range obj {
		if k == fromStr {
			result[toStr] = v
		} else {
			result[k] = v
		}
	}

	return result, nil
}

func (d *DataTransformerExecutor) transformAddField(value interface{}, config map[string]interface{}) (interface{}, error) {
	obj, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value must be an object")
	}

	field, exists := config["field"]
	if !exists {
		return nil, fmt.Errorf("'field' parameter is required")
	}
	fieldStr, ok := field.(string)
	if !ok {
		return nil, fmt.Errorf("'field' parameter must be a string")
	}

	fieldValue, exists := config["value"]
	if !exists {
		return nil, fmt.Errorf("'value' parameter is required")
	}

	result := make(map[string]interface{})
	for k, v := range obj {
		result[k] = v
	}
	result[fieldStr] = fieldValue

	return result, nil
}

// Mathematical transformation implementations
func (d *DataTransformerExecutor) transformAdd(value interface{}, config map[string]interface{}) (interface{}, error) {
	num, ok := d.toFloat64(value)
	if !ok {
		return nil, fmt.Errorf("value must be a number")
	}

	addend, exists := config["value"]
	if !exists {
		return nil, fmt.Errorf("'value' parameter is required")
	}

	addendNum, ok := d.toFloat64(addend)
	if !ok {
		return nil, fmt.Errorf("'value' parameter must be a number")
	}

	return num + addendNum, nil
}

func (d *DataTransformerExecutor) transformRound(value interface{}, config map[string]interface{}) (interface{}, error) {
	num, ok := d.toFloat64(value)
	if !ok {
		return nil, fmt.Errorf("value must be a number")
	}

	precision := 0
	if precisionParam, exists := config["precision"]; exists {
		if precisionFloat, ok := precisionParam.(float64); ok {
			precision = int(precisionFloat)
		}
	}

	multiplier := 1.0
	for i := 0; i < precision; i++ {
		multiplier *= 10
	}

	return float64(int(num*multiplier+0.5)) / multiplier, nil
}

// Conditional transformation implementations
func (d *DataTransformerExecutor) transformIfThenElse(value interface{}, config map[string]interface{}) (interface{}, error) {
	condition, exists := config["condition"]
	if !exists {
		return nil, fmt.Errorf("'condition' parameter is required")
	}

	thenValue, exists := config["then"]
	if !exists {
		return nil, fmt.Errorf("'then' parameter is required")
	}

	elseValue, exists := config["else"]
	if !exists {
		elseValue = value
	}

	// Simple condition evaluation (value equals condition)
	if reflect.DeepEqual(value, condition) {
		return thenValue, nil
	}

	return elseValue, nil
}

func (d *DataTransformerExecutor) transformDefaultValue(value interface{}, config map[string]interface{}) (interface{}, error) {
	if value == nil {
		defaultVal, exists := config["value"]
		if !exists {
			return nil, fmt.Errorf("'value' parameter is required")
		}
		return defaultVal, nil
	}
	return value, nil
}

// Built-in validators

func (d *DataTransformerExecutor) registerBuiltinValidators() {
	d.validators["not_null"] = func(value interface{}, config map[string]interface{}) error {
		if value == nil {
			return fmt.Errorf("value cannot be null")
		}
		return nil
	}

	d.validators["range"] = func(value interface{}, config map[string]interface{}) error {
		num, ok := d.toFloat64(value)
		if !ok {
			return fmt.Errorf("value must be a number")
		}

		if min, exists := config["min"]; exists {
			if minFloat, ok := d.toFloat64(min); ok && num < minFloat {
				return fmt.Errorf("value %v is less than minimum %v", num, minFloat)
			}
		}

		if max, exists := config["max"]; exists {
			if maxFloat, ok := d.toFloat64(max); ok && num > maxFloat {
				return fmt.Errorf("value %v exceeds maximum %v", num, maxFloat)
			}
		}

		return nil
	}
}

// Placeholder implementations for remaining transformers
func (d *DataTransformerExecutor) transformSubstring(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would extract substring based on start/end parameters
	return value, nil
}

func (d *DataTransformerExecutor) transformSplit(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would split string into array
	return value, nil
}

func (d *DataTransformerExecutor) transformJoin(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would join array into string
	return value, nil
}

func (d *DataTransformerExecutor) transformParseDate(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would parse date string
	return value, nil
}

func (d *DataTransformerExecutor) transformFormatDate(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would format date
	return value, nil
}

func (d *DataTransformerExecutor) transformAddTime(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would add time duration
	return value, nil
}

func (d *DataTransformerExecutor) transformRemoveField(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would remove field from object
	return value, nil
}

func (d *DataTransformerExecutor) transformMergeObjects(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would merge objects
	return value, nil
}

func (d *DataTransformerExecutor) transformPickFields(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would pick specific fields
	return value, nil
}

func (d *DataTransformerExecutor) transformOmitFields(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would omit specific fields
	return value, nil
}

func (d *DataTransformerExecutor) transformMapArray(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would map array elements
	return value, nil
}

func (d *DataTransformerExecutor) transformFilterArray(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would filter array elements
	return value, nil
}

func (d *DataTransformerExecutor) transformSortArray(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would sort array
	return value, nil
}

func (d *DataTransformerExecutor) transformUniqueArray(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would remove duplicates from array
	return value, nil
}

func (d *DataTransformerExecutor) transformSubtract(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would subtract numbers
	return value, nil
}

func (d *DataTransformerExecutor) transformMultiply(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would multiply numbers
	return value, nil
}

func (d *DataTransformerExecutor) transformDivide(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would divide numbers
	return value, nil
}

func (d *DataTransformerExecutor) transformCoalesce(value interface{}, config map[string]interface{}) (interface{}, error) {
	// Implementation would return first non-null value
	return value, nil
}

// CreateDataTransformerTool creates and configures the data transformer tool
func CreateDataTransformerTool() *tools.Tool {
	return &tools.Tool{
		ID:          "data_transformer",
		Name:        "Advanced Data Transformer",
		Description: "Comprehensive data transformation tool with validation, pipelines, and extensive transformation functions",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"data": {
					Type:        "object",
					Description: "Data to transform",
				},
				"pipeline": {
					Type:        "object",
					Description: "Transformation pipeline configuration",
					Properties: map[string]*tools.Property{
						"name": {
							Type:        "string",
							Description: "Pipeline name",
						},
						"description": {
							Type:        "string",
							Description: "Pipeline description",
						},
						"steps": {
							Type:        "array",
							Description: "Transformation steps",
							Items: &tools.Property{
								Type: "object",
								Properties: map[string]*tools.Property{
									"name": {Type: "string"},
									"type": {Type: "string"},
									"config": {Type: "object"},
									"condition": {Type: "object"},
									"error_policy": {
										Type: "string",
										Enum: []interface{}{"stop", "skip", "default"},
									},
									"default": {Type: "object"},
								},
								Required: []string{"name", "type"},
							},
						},
						"validation": {
							Type:        "object",
							Description: "Validation configuration",
							Properties: map[string]*tools.Property{
								"rules": {
									Type: "array",
									Items: &tools.Property{
										Type: "object",
										Properties: map[string]*tools.Property{
											"field": {Type: "string"},
											"type": {Type: "string"},
											"required": {Type: "boolean"},
											"constraints": {Type: "object"},
											"message": {Type: "string"},
										},
										Required: []string{"field", "type"},
									},
								},
								"strict_mode": {Type: "boolean"},
								"fail_on_first": {Type: "boolean"},
							},
						},
						"options": {
							Type:        "object",
							Description: "Pipeline options",
							Properties: map[string]*tools.Property{
								"ignore_null": {Type: "boolean"},
								"preserve_structure": {Type: "boolean"},
								"strict_types": {Type: "boolean"},
								"error_handling": {
									Type: "string",
									Enum: []interface{}{"strict", "lenient", "collect"},
								},
								"max_errors": {Type: "number"},
							},
						},
					},
					Required: []string{"name", "steps"},
				},
			},
			Required: []string{"data", "pipeline"},
		},
		Metadata: &tools.ToolMetadata{
			Author:        "AG-UI SDK Examples",
			License:       "MIT",
			Documentation: "https://github.com/mattsp1290/ag-ui/blob/main/go-sdk/examples/tools/validation/README.md",
			Tags:          []string{"transformation", "validation", "data-processing", "pipeline"},
			Examples: []tools.ToolExample{
				{
					Name:        "String Transformation Pipeline",
					Description: "Transform user input with validation",
					Input: map[string]interface{}{
						"data": map[string]interface{}{
							"firstName": "  JOHN  ",
							"lastName":  "  DOE  ",
							"email":     "JOHN.DOE@EXAMPLE.COM",
							"age":       "25",
						},
						"pipeline": map[string]interface{}{
							"name": "user_data_cleanup",
							"steps": []interface{}{
								map[string]interface{}{
									"name": "trim_first_name",
									"type": "trim",
									"config": map[string]interface{}{
										"field": "firstName",
									},
								},
								map[string]interface{}{
									"name": "lowercase_email",
									"type": "lowercase",
									"config": map[string]interface{}{
										"field": "email",
									},
								},
								map[string]interface{}{
									"name": "convert_age",
									"type": "to_integer",
									"config": map[string]interface{}{
										"field": "age",
									},
								},
							},
							"validation": map[string]interface{}{
								"rules": []interface{}{
									map[string]interface{}{
										"field":    "firstName",
										"type":     "string",
										"required": true,
										"constraints": map[string]interface{}{
											"minLength": 1,
											"maxLength": 50,
										},
									},
									map[string]interface{}{
										"field":    "age",
										"type":     "integer",
										"required": true,
										"constraints": map[string]interface{}{
											"minimum": 18,
											"maximum": 120,
										},
									},
								},
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
			Cacheable:  false, // Transformation results should not be cached by default
			Timeout:    60 * time.Second,
		},
		Executor: NewDataTransformerExecutor(),
	}
}

func main() {
	// Create registry and register the data transformer tool
	registry := tools.NewRegistry()
	dataTransformerTool := CreateDataTransformerTool()

	if err := registry.Register(dataTransformerTool); err != nil {
		log.Fatalf("Failed to register data transformer tool: %v", err)
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

	fmt.Println("=== Advanced Data Transformer Tool Example ===")
	fmt.Println("Demonstrates: Complex transformation pipelines, validation, and error handling")
	fmt.Println()

	// Example 1: String transformation pipeline
	fmt.Println("1. String transformation and validation...")
	result, err := engine.Execute(ctx, "data_transformer", map[string]interface{}{
		"data": map[string]interface{}{
			"firstName": "  JOHN  ",
			"lastName":  "  DOE  ",
			"email":     "JOHN.DOE@EXAMPLE.COM",
			"age":       "25",
		},
		"pipeline": map[string]interface{}{
			"name":        "user_data_cleanup",
			"description": "Clean and validate user registration data",
			"steps": []interface{}{
				map[string]interface{}{
					"name": "trim_first_name",
					"type": "trim",
				},
				map[string]interface{}{
					"name": "trim_last_name",
					"type": "trim",
				},
				map[string]interface{}{
					"name": "lowercase_email",
					"type": "lowercase",
				},
				map[string]interface{}{
					"name": "convert_age",
					"type": "to_integer",
				},
			},
			"validation": map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{
						"field":    "firstName",
						"type":     "string",
						"required": true,
						"constraints": map[string]interface{}{
							"minLength": 1,
							"maxLength": 50,
						},
					},
					map[string]interface{}{
						"field":    "age",
						"type":     "integer",
						"required": true,
						"constraints": map[string]interface{}{
							"minimum": 18,
							"maximum": 120,
						},
					},
				},
			},
		},
	})

	printTransformationResult(result, err, "String transformation")

	// Example 2: Mathematical transformations
	fmt.Println("2. Mathematical transformations...")
	result, err = engine.Execute(ctx, "data_transformer", map[string]interface{}{
		"data": map[string]interface{}{
			"price":    100.456,
			"quantity": 3,
			"discount": 0.1,
		},
		"pipeline": map[string]interface{}{
			"name": "order_calculation",
			"steps": []interface{}{
				map[string]interface{}{
					"name": "round_price",
					"type": "round",
					"config": map[string]interface{}{
						"precision": 2,
					},
				},
				map[string]interface{}{
					"name": "calculate_total",
					"type": "add_field",
					"config": map[string]interface{}{
						"field": "total",
						"value": 301.37, // This would be calculated in a real implementation
					},
				},
			},
			"options": map[string]interface{}{
				"error_handling": "lenient",
			},
		},
	})

	printTransformationResult(result, err, "Mathematical transformations")

	// Example 3: Conditional transformations with errors
	fmt.Println("3. Conditional transformations with error handling...")
	result, err = engine.Execute(ctx, "data_transformer", map[string]interface{}{
		"data": map[string]interface{}{
			"status": "premium",
			"score":  "invalid_number", // This will cause an error
		},
		"pipeline": map[string]interface{}{
			"name": "conditional_processing",
			"steps": []interface{}{
				map[string]interface{}{
					"name": "convert_score",
					"type": "to_number",
					"condition": map[string]interface{}{
						"field":    "status",
						"operator": "equals",
						"value":    "premium",
					},
					"error_policy": "default",
					"default":      0,
				},
				map[string]interface{}{
					"name": "add_bonus",
					"type": "add",
					"config": map[string]interface{}{
						"value": 10,
					},
				},
			},
			"options": map[string]interface{}{
				"error_handling": "collect",
				"max_errors":     5,
			},
		},
	})

	printTransformationResult(result, err, "Conditional transformations")

	// Example 4: Complex object transformation
	fmt.Println("4. Complex object transformation...")
	result, err = engine.Execute(ctx, "data_transformer", map[string]interface{}{
		"data": map[string]interface{}{
			"user_name":  "john_doe",
			"user_email": "john@example.com",
			"temp_field": "should_be_removed",
		},
		"pipeline": map[string]interface{}{
			"name": "object_restructuring",
			"steps": []interface{}{
				map[string]interface{}{
					"name": "rename_username",
					"type": "rename_field",
					"config": map[string]interface{}{
						"from": "user_name",
						"to":   "username",
					},
				},
				map[string]interface{}{
					"name": "rename_email",
					"type": "rename_field",
					"config": map[string]interface{}{
						"from": "user_email",
						"to":   "email",
					},
				},
				map[string]interface{}{
					"name": "add_timestamp",
					"type": "add_field",
					"config": map[string]interface{}{
						"field": "created_at",
						"value": time.Now().Format(time.RFC3339),
					},
				},
			},
			"validation": map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{
						"field":    "username",
						"type":     "string",
						"required": true,
					},
					map[string]interface{}{
						"field":    "email",
						"type":     "string",
						"required": true,
					},
				},
				"strict_mode": true,
			},
		},
	})

	printTransformationResult(result, err, "Complex object transformation")
}

func printTransformationResult(result *tools.ToolExecutionResult, err error, title string) {
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
	
	fmt.Printf("  Success: %v\n", summary["success"])
	fmt.Printf("  Steps executed: %v\n", summary["steps_executed"])
	fmt.Printf("  Fields transformed: %v\n", summary["fields_transformed"])
	fmt.Printf("  Errors: %v\n", summary["error_count"])
	fmt.Printf("  Warnings: %v\n", summary["warning_count"])
	fmt.Printf("  Transformation time: %vms\n", summary["transformation_time_ms"])

	if transformedData, exists := data["transformed_data"]; exists {
		fmt.Printf("  Transformed data: %v\n", transformedData)
	}

	if errorDetails, exists := data["error_details"]; exists {
		fmt.Printf("  Error details:\n")
		errorMap := errorDetails.(map[string]interface{})
		for category, errors := range errorMap {
			errorList := errors.([]interface{})
			fmt.Printf("    %s: %d errors\n", category, len(errorList))
		}
	}

	if warningDetails, exists := data["warning_details"]; exists {
		fmt.Printf("  Warning details:\n")
		warningMap := warningDetails.(map[string]interface{})
		for category, warnings := range warningMap {
			warningList := warnings.([]interface{})
			fmt.Printf("    %s: %d warnings\n", category, len(warningList))
		}
	}

	fmt.Printf("  Duration: %v\n", result.Duration)
	fmt.Println()
}