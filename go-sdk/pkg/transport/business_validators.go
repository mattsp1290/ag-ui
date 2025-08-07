package transport

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/common"
)

// RangeValidator validates that a value falls within a specified range
type RangeValidator[T comparable] struct {
	name      string
	min       *T
	max       *T
	inclusive bool
	comparer  func(a, b T) int // returns -1 if a < b, 0 if a == b, 1 if a > b
	condition func(T) bool
	enabled   bool
	priority  int
}

// NewRangeValidator creates a new range validator
func NewRangeValidator[T comparable](name string, comparer func(a, b T) int) *RangeValidator[T] {
	return &RangeValidator[T]{
		name:      name,
		comparer:  comparer,
		inclusive: true,
		enabled:   true,
		priority:  50,
	}
}

// SetRange sets the minimum and maximum values for the range
func (v *RangeValidator[T]) SetRange(min, max *T) *RangeValidator[T] {
	v.min = min
	v.max = max
	return v
}

// SetInclusive sets whether the range bounds are inclusive
func (v *RangeValidator[T]) SetInclusive(inclusive bool) *RangeValidator[T] {
	v.inclusive = inclusive
	return v
}

// SetCondition sets a condition that must be met for validation to apply
func (v *RangeValidator[T]) SetCondition(condition func(T) bool) *RangeValidator[T] {
	v.condition = condition
	return v
}

// SetEnabled enables or disables the validator
func (v *RangeValidator[T]) SetEnabled(enabled bool) *RangeValidator[T] {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *RangeValidator[T]) SetPriority(priority int) *RangeValidator[T] {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *RangeValidator[T]) Name() string {
	return fmt.Sprintf("range_%s", v.name)
}

// Validate validates that a value is within the specified range
func (v *RangeValidator[T]) Validate(ctx context.Context, value T) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	// Check condition if set
	if v.condition != nil && !v.condition(value) {
		return NewValidationResult(true)
	}

	result := NewValidationResult(true)

	// Check minimum value
	if v.min != nil {
		cmp := v.comparer(value, *v.min)
		if v.inclusive && cmp < 0 {
			result.AddError(NewValidationError(fmt.Sprintf("value %v is less than minimum %v", value, *v.min), nil))
		} else if !v.inclusive && cmp <= 0 {
			result.AddError(NewValidationError(fmt.Sprintf("value %v must be greater than %v", value, *v.min), nil))
		}
	}

	// Check maximum value
	if v.max != nil {
		cmp := v.comparer(value, *v.max)
		if v.inclusive && cmp > 0 {
			result.AddError(NewValidationError(fmt.Sprintf("value %v exceeds maximum %v", value, *v.max), nil))
		} else if !v.inclusive && cmp >= 0 {
			result.AddError(NewValidationError(fmt.Sprintf("value %v must be less than %v", value, *v.max), nil))
		}
	}

	return result
}

// IsEnabled returns whether the validator is enabled
func (v *RangeValidator[T]) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *RangeValidator[T]) Priority() int {
	return v.priority
}

// PatternValidator validates strings against complex regex patterns
type PatternValidator struct {
	name            string
	patterns        map[string]*regexp.Regexp
	required        map[string]bool
	anyMatch        bool // if true, value must match any pattern; if false, must match all
	condition       func(string) bool
	enabled         bool
	priority        int
	caseInsensitive bool
}

// NewPatternValidator creates a new pattern validator
func NewPatternValidator(name string) *PatternValidator {
	return &PatternValidator{
		name:     name,
		patterns: make(map[string]*regexp.Regexp),
		required: make(map[string]bool),
		anyMatch: false,
		enabled:  true,
		priority: 50,
	}
}

// AddPattern adds a named regex pattern
func (v *PatternValidator) AddPattern(name, pattern string, required bool) error {
	flags := ""
	if v.caseInsensitive {
		flags = "(?i)"
	}

	regex, err := regexp.Compile(flags + pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern '%s': %w", pattern, err)
	}

	v.patterns[name] = regex
	v.required[name] = required
	return nil
}

// SetAnyMatch sets whether the value must match any pattern (true) or all patterns (false)
func (v *PatternValidator) SetAnyMatch(anyMatch bool) *PatternValidator {
	v.anyMatch = anyMatch
	return v
}

// SetCaseInsensitive sets whether pattern matching should be case insensitive
func (v *PatternValidator) SetCaseInsensitive(caseInsensitive bool) *PatternValidator {
	v.caseInsensitive = caseInsensitive
	return v
}

// SetCondition sets a condition that must be met for validation to apply
func (v *PatternValidator) SetCondition(condition func(string) bool) *PatternValidator {
	v.condition = condition
	return v
}

// SetEnabled enables or disables the validator
func (v *PatternValidator) SetEnabled(enabled bool) *PatternValidator {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *PatternValidator) SetPriority(priority int) *PatternValidator {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *PatternValidator) Name() string {
	return fmt.Sprintf("pattern_%s", v.name)
}

// Validate validates a string against the configured patterns
func (v *PatternValidator) Validate(ctx context.Context, value string) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	// Check condition if set
	if v.condition != nil && !v.condition(value) {
		return NewValidationResult(true)
	}

	result := NewValidationResult(true)

	if len(v.patterns) == 0 {
		return result
	}

	var matchedPatterns []string
	var unmatchedRequired []string

	// Check each pattern
	for name, pattern := range v.patterns {
		matches := pattern.MatchString(value)

		if matches {
			matchedPatterns = append(matchedPatterns, name)
		} else if v.required[name] {
			unmatchedRequired = append(unmatchedRequired, name)
		}
	}

	// Validate based on matching strategy
	if v.anyMatch {
		// Must match at least one pattern
		if len(matchedPatterns) == 0 {
			patternNames := make([]string, 0, len(v.patterns))
			for name := range v.patterns {
				patternNames = append(patternNames, name)
			}
			result.AddError(NewValidationError(fmt.Sprintf("value '%s' does not match any of the required patterns: %s", value, strings.Join(patternNames, ", ")), nil))
		}
	} else {
		// Must match all required patterns
		if len(unmatchedRequired) > 0 {
			result.AddError(NewValidationError(fmt.Sprintf("value '%s' does not match required patterns: %s", value, strings.Join(unmatchedRequired, ", ")), nil))
		}
	}

	return result
}

// IsEnabled returns whether the validator is enabled
func (v *PatternValidator) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *PatternValidator) Priority() int {
	return v.priority
}

// DependencyValidator validates field dependencies (field A requires field B to be present/valid)
type DependencyValidator struct {
	name         string
	dependencies map[string][]string            // field -> list of required fields
	validators   map[string]TypedValidator[any] // field -> validator for that field
	condition    func(map[string]interface{}) bool
	enabled      bool
	priority     int
}

// NewDependencyValidator creates a new dependency validator
func NewDependencyValidator(name string) *DependencyValidator {
	return &DependencyValidator{
		name:         name,
		dependencies: make(map[string][]string),
		validators:   make(map[string]TypedValidator[any]),
		enabled:      true,
		priority:     60, // Higher priority since dependencies should be checked early
	}
}

// AddDependency adds a dependency rule (if field exists, then requiredFields must also exist)
func (v *DependencyValidator) AddDependency(field string, requiredFields ...string) *DependencyValidator {
	v.dependencies[field] = append(v.dependencies[field], requiredFields...)
	return v
}

// SetFieldValidator sets a validator for a specific field
func (v *DependencyValidator) SetFieldValidator(field string, validator TypedValidator[any]) *DependencyValidator {
	v.validators[field] = validator
	return v
}

// SetCondition sets a condition that must be met for validation to apply
func (v *DependencyValidator) SetCondition(condition func(map[string]interface{}) bool) *DependencyValidator {
	v.condition = condition
	return v
}

// SetEnabled enables or disables the validator
func (v *DependencyValidator) SetEnabled(enabled bool) *DependencyValidator {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *DependencyValidator) SetPriority(priority int) *DependencyValidator {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *DependencyValidator) Name() string {
	return fmt.Sprintf("dependency_%s", v.name)
}

// Validate validates field dependencies
func (v *DependencyValidator) Validate(ctx context.Context, value map[string]interface{}) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	// Check condition if set
	if v.condition != nil && !v.condition(value) {
		return NewValidationResult(true)
	}

	result := NewValidationResult(true)

	// Check each dependency rule
	for field, requiredFields := range v.dependencies {
		fieldValue, fieldExists := value[field]

		// If the field exists and is not nil/empty, check its dependencies
		if fieldExists && !isEmptyValue(fieldValue) {
			for _, requiredField := range requiredFields {
				requiredValue, requiredExists := value[requiredField]

				if !requiredExists || isEmptyValue(requiredValue) {
					result.AddFieldError(field, NewValidationError(fmt.Sprintf("field '%s' requires field '%s' to be present and non-empty", field, requiredField), nil))
				} else {
					// Validate the required field if we have a validator for it
					if validator, hasValidator := v.validators[requiredField]; hasValidator {
						fieldResult := validator.Validate(ctx, requiredValue)
						if !fieldResult.IsValid() {
							for _, err := range fieldResult.Errors() {
								result.AddFieldError(requiredField, NewValidationError(fmt.Sprintf("dependency validation failed: %s", err.Error()), nil))
							}
						}
					}
				}
			}
		}
	}

	return result
}

// IsEnabled returns whether the validator is enabled
func (v *DependencyValidator) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *DependencyValidator) Priority() int {
	return v.priority
}

// CrossFieldValidator validates relationships between multiple fields
type CrossFieldValidator struct {
	name      string
	rules     []CrossFieldRule
	condition func(map[string]interface{}) bool
	enabled   bool
	priority  int
}

// CrossFieldRule defines a validation rule that compares multiple fields
type CrossFieldRule struct {
	Name        string
	Fields      []string
	Validator   func(values map[string]interface{}) error
	Description string
}

// NewCrossFieldValidator creates a new cross-field validator
func NewCrossFieldValidator(name string) *CrossFieldValidator {
	return &CrossFieldValidator{
		name:     name,
		rules:    make([]CrossFieldRule, 0),
		enabled:  true,
		priority: 55,
	}
}

// AddRule adds a cross-field validation rule
func (v *CrossFieldValidator) AddRule(rule CrossFieldRule) *CrossFieldValidator {
	v.rules = append(v.rules, rule)
	return v
}

// AddComparisonRule adds a rule to compare two fields using a comparison function
func (v *CrossFieldValidator) AddComparisonRule(name, field1, field2 string, compare func(a, b interface{}) bool, errorMsg string) *CrossFieldValidator {
	rule := CrossFieldRule{
		Name:        name,
		Fields:      []string{field1, field2},
		Description: errorMsg,
		Validator: func(values map[string]interface{}) error {
			val1, exists1 := values[field1]
			val2, exists2 := values[field2]

			if !exists1 || !exists2 {
				return fmt.Errorf("both fields '%s' and '%s' must be present", field1, field2)
			}

			if !compare(val1, val2) {
				return fmt.Errorf(errorMsg)
			}

			return nil
		},
	}

	return v.AddRule(rule)
}

// SetCondition sets a condition that must be met for validation to apply
func (v *CrossFieldValidator) SetCondition(condition func(map[string]interface{}) bool) *CrossFieldValidator {
	v.condition = condition
	return v
}

// SetEnabled enables or disables the validator
func (v *CrossFieldValidator) SetEnabled(enabled bool) *CrossFieldValidator {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *CrossFieldValidator) SetPriority(priority int) *CrossFieldValidator {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *CrossFieldValidator) Name() string {
	return fmt.Sprintf("crossfield_%s", v.name)
}

// Validate validates cross-field relationships
func (v *CrossFieldValidator) Validate(ctx context.Context, value map[string]interface{}) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	// Check condition if set
	if v.condition != nil && !v.condition(value) {
		return NewValidationResult(true)
	}

	result := NewValidationResult(true)

	// Apply each cross-field rule
	for _, rule := range v.rules {
		// Extract values for the fields involved in this rule
		ruleValues := make(map[string]interface{})
		for _, field := range rule.Fields {
			if val, exists := value[field]; exists {
				ruleValues[field] = val
			}
		}

		// Apply the rule validator
		if err := rule.Validator(ruleValues); err != nil {
			result.AddError(NewValidationError(fmt.Sprintf("cross-field rule '%s' failed: %s", rule.Name, err.Error()), nil))
		}
	}

	return result
}

// IsEnabled returns whether the validator is enabled
func (v *CrossFieldValidator) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *CrossFieldValidator) Priority() int {
	return v.priority
}

// ConditionalValidator applies validation rules based on conditions
type ConditionalValidator struct {
	name       string
	conditions []ConditionalRule
	enabled    bool
	priority   int
}

// ConditionalRule defines a condition and the validators to apply when it's met
type ConditionalRule struct {
	Name        string
	Condition   func(interface{}) bool
	Validator   TypedValidator[any]
	Description string
}

// NewConditionalValidator creates a new conditional validator
func NewConditionalValidator(name string) *ConditionalValidator {
	return &ConditionalValidator{
		name:       name,
		conditions: make([]ConditionalRule, 0),
		enabled:    true,
		priority:   40, // Lower priority so other validators run first
	}
}

// AddRule adds a conditional validation rule
func (v *ConditionalValidator) AddRule(rule ConditionalRule) *ConditionalValidator {
	v.conditions = append(v.conditions, rule)
	return v
}

// AddFieldCondition adds a rule that applies a validator when a specific field meets a condition
func (v *ConditionalValidator) AddFieldCondition(name, field string, condition func(interface{}) bool, validator TypedValidator[any], description string) *ConditionalValidator {
	rule := ConditionalRule{
		Name:        name,
		Description: description,
		Validator:   validator,
		Condition: func(value interface{}) bool {
			if data, ok := value.(map[string]interface{}); ok {
				if fieldValue, exists := data[field]; exists {
					return condition(fieldValue)
				}
			}
			return false
		},
	}

	return v.AddRule(rule)
}

// SetEnabled enables or disables the validator
func (v *ConditionalValidator) SetEnabled(enabled bool) *ConditionalValidator {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *ConditionalValidator) SetPriority(priority int) *ConditionalValidator {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *ConditionalValidator) Name() string {
	return fmt.Sprintf("conditional_%s", v.name)
}

// Validate applies conditional validation rules
func (v *ConditionalValidator) Validate(ctx context.Context, value interface{}) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	result := NewValidationResult(true)

	// Apply each conditional rule
	for _, rule := range v.conditions {
		if rule.Condition(value) {
			// Condition is met, apply the validator
			ruleResult := rule.Validator.Validate(ctx, value)
			if !ruleResult.IsValid() {
				result.SetValid(false)
				for _, err := range ruleResult.Errors() {
					result.AddError(NewValidationError(fmt.Sprintf("conditional rule '%s': %s", rule.Name, err.Error()), nil))
				}
				// Merge field errors
				for field, errors := range ruleResult.FieldErrors() {
					for _, err := range errors {
						result.AddFieldError(field, err)
					}
				}
			}
		}
	}

	return result
}

// IsEnabled returns whether the validator is enabled
func (v *ConditionalValidator) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *ConditionalValidator) Priority() int {
	return v.priority
}

// Helper functions for common business validation scenarios

// NewIntRangeValidator creates a range validator for integers
func NewIntRangeValidator(name string) *RangeValidator[int] {
	return NewRangeValidator(name, func(a, b int) int {
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0
	})
}

// NewFloat64RangeValidator creates a range validator for float64 values
func NewFloat64RangeValidator(name string) *RangeValidator[float64] {
	return NewRangeValidator(name, func(a, b float64) int {
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0
	})
}

// NewTimeRangeValidator creates a range validator for time values
func NewTimeRangeValidator(name string) *RangeValidator[time.Time] {
	return NewRangeValidator(name, func(a, b time.Time) int {
		if a.Before(b) {
			return -1
		} else if a.After(b) {
			return 1
		}
		return 0
	})
}

// NewStringLengthRangeValidator creates a range validator for string lengths
func NewStringLengthRangeValidator(name string) *RangeValidator[string] {
	return NewRangeValidator(name, func(a, b string) int {
		lenA, lenB := len(a), len(b)
		if lenA < lenB {
			return -1
		} else if lenA > lenB {
			return 1
		}
		return 0
	})
}

// NewEmailPatternValidator creates a pattern validator for email addresses
func NewEmailPatternValidator() (*PatternValidator, error) {
	validator := NewPatternValidator("email")
	// Basic email pattern - can be enhanced for more sophisticated validation
	err := validator.AddPattern("email", `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create email validator: %w", err)
	}
	return validator, nil
}

// NewURLPatternValidator creates a pattern validator for URLs
func NewURLPatternValidator() (*PatternValidator, error) {
	validator := NewPatternValidator("url")
	// More comprehensive URL pattern that validates structure
	// This pattern checks for:
	// - Valid scheme (http/https)
	// - Valid hostname or IP
	// - Optional port
	// - Optional path, query, and fragment
	err := validator.AddPattern("url", `^https?://([a-zA-Z0-9-]+\.)*[a-zA-Z0-9-]+(:[0-9]+)?(/[^?\s]*)?(\?[^#\s]*)?(#[^\s]*)?$`, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create URL validator: %w", err)
	}
	return validator, nil
}

// NewPhonePatternValidator creates a pattern validator for phone numbers
func NewPhonePatternValidator() (*PatternValidator, error) {
	validator := NewPatternValidator("phone")
	// Basic phone pattern - supports various formats
	err := validator.AddPattern("phone", `^[\+]?[\d\s\-\(\)]{10,}$`, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create phone validator: %w", err)
	}
	return validator, nil
}

// MustNewEmailPatternValidator creates a pattern validator for email addresses and panics on error.
// This function is provided for backward compatibility where panicking is desired.
// For production code, prefer using NewEmailPatternValidator() which returns an error.
func MustNewEmailPatternValidator() *PatternValidator {
	validator, err := NewEmailPatternValidator()
	if err != nil {
		panic(fmt.Sprintf("failed to create email validator: %v", err))
	}
	return validator
}

// MustNewURLPatternValidator creates a pattern validator for URLs and panics on error.
// This function is provided for backward compatibility where panicking is desired.
// For production code, prefer using NewURLPatternValidator() which returns an error.
func MustNewURLPatternValidator() *PatternValidator {
	validator, err := NewURLPatternValidator()
	if err != nil {
		panic(fmt.Sprintf("failed to create URL validator: %v", err))
	}
	return validator
}

// MustNewPhonePatternValidator creates a pattern validator for phone numbers and panics on error.
// This function is provided for backward compatibility where panicking is desired.
// For production code, prefer using NewPhonePatternValidator() which returns an error.
func MustNewPhonePatternValidator() *PatternValidator {
	validator, err := NewPhonePatternValidator()
	if err != nil {
		panic(fmt.Sprintf("failed to create phone validator: %v", err))
	}
	return validator
}

// URLValidator provides comprehensive URL validation with security checks
type URLValidator struct {
	name     string
	options  common.URLValidationOptions
	enabled  bool
	priority int
}

// NewSecureURLValidator creates a URL validator with security checks
func NewSecureURLValidator(name string) *URLValidator {
	return &URLValidator{
		name:     name,
		options:  common.DefaultHTTPValidationOptions(),
		enabled:  true,
		priority: 60, // Higher priority for security validations
	}
}

// NewWebhookURLValidator creates a URL validator specifically for webhooks
func NewWebhookURLValidator(name string) *URLValidator {
	return &URLValidator{
		name:     name,
		options:  common.DefaultWebhookValidationOptions(),
		enabled:  true,
		priority: 60,
	}
}

// SetOptions sets custom validation options
func (v *URLValidator) SetOptions(opts common.URLValidationOptions) *URLValidator {
	v.options = opts
	return v
}

// SetEnabled enables or disables the validator
func (v *URLValidator) SetEnabled(enabled bool) *URLValidator {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *URLValidator) SetPriority(priority int) *URLValidator {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *URLValidator) Name() string {
	return fmt.Sprintf("url_%s", v.name)
}

// Validate validates a URL string
func (v *URLValidator) Validate(ctx context.Context, value string) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	result := NewValidationResult(true)

	if err := common.ValidateURL(value, v.options); err != nil {
		result.AddError(NewValidationError(err.Error(), nil))
	}

	return result
}

// IsEnabled returns whether the validator is enabled
func (v *URLValidator) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *URLValidator) Priority() int {
	return v.priority
}

// Common cross-field validation helpers

// CreateDateRangeValidator creates a cross-field validator for date ranges
func CreateDateRangeValidator(startField, endField string) *CrossFieldValidator {
	validator := NewCrossFieldValidator("date_range")

	validator.AddComparisonRule(
		"start_before_end",
		startField,
		endField,
		func(start, end interface{}) bool {
			startTime, ok1 := start.(time.Time)
			endTime, ok2 := end.(time.Time)
			if !ok1 || !ok2 {
				return false // Type validation should be done elsewhere
			}
			return startTime.Before(endTime) || startTime.Equal(endTime)
		},
		fmt.Sprintf("'%s' must be before or equal to '%s'", startField, endField),
	)

	return validator
}

// CreatePasswordConfirmationValidator creates a cross-field validator for password confirmation
func CreatePasswordConfirmationValidator(passwordField, confirmField string) *CrossFieldValidator {
	validator := NewCrossFieldValidator("password_confirmation")

	validator.AddComparisonRule(
		"passwords_match",
		passwordField,
		confirmField,
		func(password, confirm interface{}) bool {
			pwd, ok1 := password.(string)
			conf, ok2 := confirm.(string)
			if !ok1 || !ok2 {
				return false
			}
			return pwd == conf
		},
		"passwords do not match",
	)

	return validator
}

// Utility functions

// isEmptyValue checks if a value is considered empty
func isEmptyValue(value interface{}) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Invalid:
		return true
	default:
		return false
	}
}

// compareValues provides a generic comparison function for common types
func compareValues(a, b interface{}) int {
	// Handle nil values
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	// Try to compare as common types
	switch va := a.(type) {
	case int:
		if vb, ok := b.(int); ok {
			if va < vb {
				return -1
			} else if va > vb {
				return 1
			}
			return 0
		}
	case float64:
		if vb, ok := b.(float64); ok {
			if va < vb {
				return -1
			} else if va > vb {
				return 1
			}
			return 0
		}
	case string:
		if vb, ok := b.(string); ok {
			return strings.Compare(va, vb)
		}
	case time.Time:
		if vb, ok := b.(time.Time); ok {
			if va.Before(vb) {
				return -1
			} else if va.After(vb) {
				return 1
			}
			return 0
		}
	}

	// Fallback to string comparison
	return strings.Compare(fmt.Sprintf("%v", a), fmt.Sprintf("%v", b))
}
