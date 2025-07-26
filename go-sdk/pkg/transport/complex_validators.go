package transport

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// StructValidator provides type-safe validation for structs with constraints
type StructValidator[T any] struct {
	name       string
	validators map[string]TypedValidator[any]
	required   map[string]bool
	condition  func(T) bool
	enabled    bool
	priority   int
}

// NewStructValidator creates a new struct validator
func NewStructValidator[T any](name string) *StructValidator[T] {
	return &StructValidator[T]{
		name:       name,
		validators: make(map[string]TypedValidator[any]),
		required:   make(map[string]bool),
		enabled:    true,
		priority:   50,
	}
}

// AddFieldValidator adds a validator for a specific field
func (v *StructValidator[T]) AddFieldValidator(field string, validator TypedValidator[any]) *StructValidator[T] {
	v.validators[field] = validator
	return v
}

// SetRequired marks a field as required
func (v *StructValidator[T]) SetRequired(field string, required bool) *StructValidator[T] {
	v.required[field] = required
	return v
}

// SetCondition sets a condition that must be met for validation to apply
func (v *StructValidator[T]) SetCondition(condition func(T) bool) *StructValidator[T] {
	v.condition = condition
	return v
}

// SetEnabled enables or disables the validator
func (v *StructValidator[T]) SetEnabled(enabled bool) *StructValidator[T] {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *StructValidator[T]) SetPriority(priority int) *StructValidator[T] {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *StructValidator[T]) Name() string {
	return fmt.Sprintf("struct_%s", v.name)
}

// Validate validates a struct value
func (v *StructValidator[T]) Validate(ctx context.Context, value T) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	// Check condition if set
	if v.condition != nil && !v.condition(value) {
		return NewValidationResult(true)
	}

	result := NewValidationResult(true)
	
	// Use reflection to validate struct fields
	val := reflect.ValueOf(value)
	typ := reflect.TypeOf(value)
	
	// Dereference pointer if necessary
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			if len(v.required) > 0 {
				result.AddError(NewValidationError("struct cannot be nil when required fields are specified", nil))
				return result
			}
			return result
		}
		val = val.Elem()
		typ = typ.Elem()
	}
	
	if val.Kind() != reflect.Struct {
		result.AddError(NewValidationError("value must be a struct", nil))
		return result
	}

	// Validate each configured field
	for fieldName, validator := range v.validators {
		field := val.FieldByName(fieldName)
		if !field.IsValid() {
			if v.required[fieldName] {
				result.AddFieldError(fieldName, NewValidationError(fmt.Sprintf("required field '%s' not found", fieldName), nil))
			}
			continue
		}

		// Check if required field is zero value
		if v.required[fieldName] && field.IsZero() {
			result.AddFieldError(fieldName, NewValidationError(fmt.Sprintf("required field '%s' cannot be empty", fieldName), nil))
			continue
		}

		// Validate field value
		fieldResult := validator.Validate(ctx, field.Interface())
		if !fieldResult.IsValid() {
			for _, err := range fieldResult.Errors() {
				result.AddFieldError(fieldName, err)
			}
		}
	}

	return result
}

// IsEnabled returns whether the validator is enabled
func (v *StructValidator[T]) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *StructValidator[T]) Priority() int {
	return v.priority
}

// SliceValidator provides type-safe validation for slices with element rules
type SliceValidator[T any] struct {
	name           string
	elementValidator TypedValidator[T]
	minLength      *int
	maxLength      *int
	condition      func([]T) bool
	enabled        bool
	priority       int
}

// NewSliceValidator creates a new slice validator
func NewSliceValidator[T any](name string, elementValidator TypedValidator[T]) *SliceValidator[T] {
	return &SliceValidator[T]{
		name:             name,
		elementValidator: elementValidator,
		enabled:          true,
		priority:         50,
	}
}

// SetLengthRange sets the allowed length range for the slice
func (v *SliceValidator[T]) SetLengthRange(min, max *int) *SliceValidator[T] {
	v.minLength = min
	v.maxLength = max
	return v
}

// SetCondition sets a condition that must be met for validation to apply
func (v *SliceValidator[T]) SetCondition(condition func([]T) bool) *SliceValidator[T] {
	v.condition = condition
	return v
}

// SetEnabled enables or disables the validator
func (v *SliceValidator[T]) SetEnabled(enabled bool) *SliceValidator[T] {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *SliceValidator[T]) SetPriority(priority int) *SliceValidator[T] {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *SliceValidator[T]) Name() string {
	return fmt.Sprintf("slice_%s", v.name)
}

// Validate validates a slice value
func (v *SliceValidator[T]) Validate(ctx context.Context, value []T) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	// Check condition if set
	if v.condition != nil && !v.condition(value) {
		return NewValidationResult(true)
	}

	result := NewValidationResult(true)

	// Check length constraints
	length := len(value)
	if v.minLength != nil && length < *v.minLength {
		result.AddError(NewValidationError(fmt.Sprintf("slice length %d is less than minimum %d", length, *v.minLength), nil))
	}
	if v.maxLength != nil && length > *v.maxLength {
		result.AddError(NewValidationError(fmt.Sprintf("slice length %d exceeds maximum %d", length, *v.maxLength), nil))
	}

	// Validate each element
	if v.elementValidator != nil {
		for i, element := range value {
			elementResult := v.elementValidator.Validate(ctx, element)
			if !elementResult.IsValid() {
				for _, err := range elementResult.Errors() {
					result.AddFieldError(fmt.Sprintf("[%d]", i), err)
				}
			}
		}
	}

	return result
}

// IsEnabled returns whether the validator is enabled
func (v *SliceValidator[T]) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *SliceValidator[T]) Priority() int {
	return v.priority
}

// MapValidator provides type-safe validation for maps with key and value rules
type MapValidator[K comparable, V any] struct {
	name           string
	keyValidator   TypedValidator[K]
	valueValidator TypedValidator[V]
	minSize        *int
	maxSize        *int
	requiredKeys   []K
	condition      func(map[K]V) bool
	enabled        bool
	priority       int
}

// NewMapValidator creates a new map validator
func NewMapValidator[K comparable, V any](name string) *MapValidator[K, V] {
	return &MapValidator[K, V]{
		name:     name,
		enabled:  true,
		priority: 50,
	}
}

// SetKeyValidator sets the validator for map keys
func (v *MapValidator[K, V]) SetKeyValidator(validator TypedValidator[K]) *MapValidator[K, V] {
	v.keyValidator = validator
	return v
}

// SetValueValidator sets the validator for map values
func (v *MapValidator[K, V]) SetValueValidator(validator TypedValidator[V]) *MapValidator[K, V] {
	v.valueValidator = validator
	return v
}

// SetSizeRange sets the allowed size range for the map
func (v *MapValidator[K, V]) SetSizeRange(min, max *int) *MapValidator[K, V] {
	v.minSize = min
	v.maxSize = max
	return v
}

// SetRequiredKeys sets keys that must be present in the map
func (v *MapValidator[K, V]) SetRequiredKeys(keys ...K) *MapValidator[K, V] {
	v.requiredKeys = keys
	return v
}

// SetCondition sets a condition that must be met for validation to apply
func (v *MapValidator[K, V]) SetCondition(condition func(map[K]V) bool) *MapValidator[K, V] {
	v.condition = condition
	return v
}

// SetEnabled enables or disables the validator
func (v *MapValidator[K, V]) SetEnabled(enabled bool) *MapValidator[K, V] {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *MapValidator[K, V]) SetPriority(priority int) *MapValidator[K, V] {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *MapValidator[K, V]) Name() string {
	return fmt.Sprintf("map_%s", v.name)
}

// Validate validates a map value
func (v *MapValidator[K, V]) Validate(ctx context.Context, value map[K]V) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	// Check condition if set
	if v.condition != nil && !v.condition(value) {
		return NewValidationResult(true)
	}

	result := NewValidationResult(true)

	// Check size constraints
	size := len(value)
	if v.minSize != nil && size < *v.minSize {
		result.AddError(NewValidationError(fmt.Sprintf("map size %d is less than minimum %d", size, *v.minSize), nil))
	}
	if v.maxSize != nil && size > *v.maxSize {
		result.AddError(NewValidationError(fmt.Sprintf("map size %d exceeds maximum %d", size, *v.maxSize), nil))
	}

	// Check required keys
	for _, reqKey := range v.requiredKeys {
		if _, exists := value[reqKey]; !exists {
			result.AddError(NewValidationError(fmt.Sprintf("required key '%v' not found", reqKey), nil))
		}
	}

	// Validate keys and values
	for key, val := range value {
		keyStr := fmt.Sprintf("%v", key)
		
		if v.keyValidator != nil {
			keyResult := v.keyValidator.Validate(ctx, key)
			if !keyResult.IsValid() {
				for _, err := range keyResult.Errors() {
					result.AddFieldError(fmt.Sprintf("key[%s]", keyStr), err)
				}
			}
		}

		if v.valueValidator != nil {
			valueResult := v.valueValidator.Validate(ctx, val)
			if !valueResult.IsValid() {
				for _, err := range valueResult.Errors() {
					result.AddFieldError(fmt.Sprintf("value[%s]", keyStr), err)
				}
			}
		}
	}

	return result
}

// IsEnabled returns whether the validator is enabled
func (v *MapValidator[K, V]) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *MapValidator[K, V]) Priority() int {
	return v.priority
}

// UnionValidator validates union types (one of several possible types)
type UnionValidator[T any] struct {
	name       string
	validators []TypedValidator[any]
	condition  func(T) bool
	enabled    bool
	priority   int
}

// NewUnionValidator creates a new union validator
func NewUnionValidator[T any](name string) *UnionValidator[T] {
	return &UnionValidator[T]{
		name:       name,
		validators: make([]TypedValidator[any], 0),
		enabled:    true,
		priority:   50,
	}
}

// AddValidator adds a validator for one of the union types
func (v *UnionValidator[T]) AddValidator(validator TypedValidator[any]) *UnionValidator[T] {
	v.validators = append(v.validators, validator)
	return v
}

// SetCondition sets a condition that must be met for validation to apply
func (v *UnionValidator[T]) SetCondition(condition func(T) bool) *UnionValidator[T] {
	v.condition = condition
	return v
}

// SetEnabled enables or disables the validator
func (v *UnionValidator[T]) SetEnabled(enabled bool) *UnionValidator[T] {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *UnionValidator[T]) SetPriority(priority int) *UnionValidator[T] {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *UnionValidator[T]) Name() string {
	return fmt.Sprintf("union_%s", v.name)
}

// Validate validates a union value (must match at least one validator)
func (v *UnionValidator[T]) Validate(ctx context.Context, value T) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	// Check condition if set
	if v.condition != nil && !v.condition(value) {
		return NewValidationResult(true)
	}

	if len(v.validators) == 0 {
		return NewValidationResult(true)
	}

	// Try each validator until one succeeds
	var allErrors []error
	for i, validator := range v.validators {
		result := validator.Validate(ctx, value)
		if result.IsValid() {
			return result // Success with one of the validators
		}
		
		// Collect errors for reporting
		for _, err := range result.Errors() {
			wrappedErr := NewValidationError(fmt.Sprintf("union option %d: %s", i+1, err.Error()), nil)
			allErrors = append(allErrors, wrappedErr)
		}
	}

	// None of the validators succeeded
	finalResult := NewValidationResult(false)
	unionErr := NewValidationError("value does not match any of the union types", allErrors)
	finalResult.AddError(unionErr)
	
	return finalResult
}

// IsEnabled returns whether the validator is enabled
func (v *UnionValidator[T]) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *UnionValidator[T]) Priority() int {
	return v.priority
}

// RecursiveValidator handles recursive data structures with cycle detection
type RecursiveValidator[T any] struct {
	name      string
	validator TypedValidator[T]
	maxDepth  int
	condition func(T) bool
	enabled   bool
	priority  int
}

// NewRecursiveValidator creates a new recursive validator
func NewRecursiveValidator[T any](name string, validator TypedValidator[T]) *RecursiveValidator[T] {
	return &RecursiveValidator[T]{
		name:      name,
		validator: validator,
		maxDepth:  100, // Default max depth to prevent infinite recursion
		enabled:   true,
		priority:  50,
	}
}

// SetMaxDepth sets the maximum recursion depth
func (v *RecursiveValidator[T]) SetMaxDepth(maxDepth int) *RecursiveValidator[T] {
	v.maxDepth = maxDepth
	return v
}

// SetCondition sets a condition that must be met for validation to apply
func (v *RecursiveValidator[T]) SetCondition(condition func(T) bool) *RecursiveValidator[T] {
	v.condition = condition
	return v
}

// SetEnabled enables or disables the validator
func (v *RecursiveValidator[T]) SetEnabled(enabled bool) *RecursiveValidator[T] {
	v.enabled = enabled
	return v
}

// SetPriority sets the validator priority
func (v *RecursiveValidator[T]) SetPriority(priority int) *RecursiveValidator[T] {
	v.priority = priority
	return v
}

// Name returns the validator name
func (v *RecursiveValidator[T]) Name() string {
	return fmt.Sprintf("recursive_%s", v.name)
}

// Validate validates a recursive structure with cycle detection
func (v *RecursiveValidator[T]) Validate(ctx context.Context, value T) ValidationResult {
	return v.validateWithDepth(ctx, value, 0, make(map[uintptr]bool))
}

// validateWithDepth performs validation with depth tracking and cycle detection
func (v *RecursiveValidator[T]) validateWithDepth(ctx context.Context, value T, depth int, visited map[uintptr]bool) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	// Check condition if set
	if v.condition != nil && !v.condition(value) {
		return NewValidationResult(true)
	}

	// Check maximum depth
	if depth > v.maxDepth {
		result := NewValidationResult(false)
		result.AddError(NewValidationError(fmt.Sprintf("maximum recursion depth %d exceeded", v.maxDepth), nil))
		return result
	}

	// Cycle detection for pointer types
	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Ptr || val.Kind() == reflect.Slice || val.Kind() == reflect.Map {
		if val.IsValid() && !val.IsNil() {
			ptr := val.Pointer()
			if visited[ptr] {
				result := NewValidationResult(false)
				result.AddError(NewValidationError("circular reference detected", nil))
				return result
			}
			visited[ptr] = true
			defer delete(visited, ptr)
		}
	}

	// Create context with depth information for the validator
	type depthKey struct{}
	ctxWithDepth := context.WithValue(ctx, depthKey{}, depth)

	// Delegate to the inner validator
	return v.validator.Validate(ctxWithDepth, value)
}

// IsEnabled returns whether the validator is enabled
func (v *RecursiveValidator[T]) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *RecursiveValidator[T]) Priority() int {
	return v.priority
}

// TypedValidator interface for type-safe validators
type TypedValidator[T any] interface {
	// Name returns the validator name
	Name() string
	
	// Validate validates a value and returns a validation result
	Validate(ctx context.Context, value T) ValidationResult
	
	// IsEnabled returns whether the validator is enabled
	IsEnabled() bool
	
	// Priority returns the validator priority
	Priority() int
}

// Complex validator composition helpers

// AllOfValidator requires all validators to pass
type AllOfValidator[T any] struct {
	name       string
	validators []TypedValidator[T]
	enabled    bool
	priority   int
}

// NewAllOfValidator creates a validator that requires all sub-validators to pass
func NewAllOfValidator[T any](name string, validators ...TypedValidator[T]) *AllOfValidator[T] {
	return &AllOfValidator[T]{
		name:       name,
		validators: validators,
		enabled:    true,
		priority:   50,
	}
}

// Name returns the validator name
func (v *AllOfValidator[T]) Name() string {
	return fmt.Sprintf("allof_%s", v.name)
}

// Validate validates that all sub-validators pass
func (v *AllOfValidator[T]) Validate(ctx context.Context, value T) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	result := NewValidationResult(true)
	
	for _, validator := range v.validators {
		subResult := validator.Validate(ctx, value)
		if !subResult.IsValid() {
			result.SetValid(false)
			for _, err := range subResult.Errors() {
				result.AddError(err)
			}
			// Merge field errors
			for field, errors := range subResult.FieldErrors() {
				for _, err := range errors {
					result.AddFieldError(field, err)
				}
			}
		}
	}

	return result
}

// IsEnabled returns whether the validator is enabled
func (v *AllOfValidator[T]) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *AllOfValidator[T]) Priority() int {
	return v.priority
}

// AnyOfValidator requires at least one validator to pass
type AnyOfValidator[T any] struct {
	name       string
	validators []TypedValidator[T]
	enabled    bool
	priority   int
}

// NewAnyOfValidator creates a validator that requires at least one sub-validator to pass
func NewAnyOfValidator[T any](name string, validators ...TypedValidator[T]) *AnyOfValidator[T] {
	return &AnyOfValidator[T]{
		name:       name,
		validators: validators,
		enabled:    true,
		priority:   50,
	}
}

// Name returns the validator name
func (v *AnyOfValidator[T]) Name() string {
	return fmt.Sprintf("anyof_%s", v.name)
}

// Validate validates that at least one sub-validator passes
func (v *AnyOfValidator[T]) Validate(ctx context.Context, value T) ValidationResult {
	if !v.enabled {
		return NewValidationResult(true)
	}

	if len(v.validators) == 0 {
		return NewValidationResult(true)
	}

	var allErrors []error
	for _, validator := range v.validators {
		subResult := validator.Validate(ctx, value)
		if subResult.IsValid() {
			return subResult // At least one passed
		}
		
		// Collect errors for reporting
		for _, err := range subResult.Errors() {
			allErrors = append(allErrors, err)
		}
	}

	// None passed
	result := NewValidationResult(false)
	anyOfErr := NewValidationError("none of the validators passed", allErrors)
	result.AddError(anyOfErr)
	
	return result
}

// IsEnabled returns whether the validator is enabled
func (v *AnyOfValidator[T]) IsEnabled() bool {
	return v.enabled
}

// Priority returns the validator priority
func (v *AnyOfValidator[T]) Priority() int {
	return v.priority
}

// Helper functions for creating common validators

// CreateStringStructValidator creates a struct validator for string fields
func CreateStringStructValidator(name string) *StructValidator[map[string]string] {
	return NewStructValidator[map[string]string](name)
}

// CreateIntSliceValidator creates a slice validator for integers
func CreateIntSliceValidator(name string, elementValidator TypedValidator[int]) *SliceValidator[int] {
	return NewSliceValidator(name, elementValidator)
}

// CreateStringMapValidator creates a map validator for string keys and values
func CreateStringMapValidator(name string) *MapValidator[string, string] {
	return NewMapValidator[string, string](name)
}

// ValidationContext keys for passing data through validation chains
type ValidationContextKey string

const (
	ValidationDepthKey ValidationContextKey = "validation_depth"
	ValidationPathKey  ValidationContextKey = "validation_path"
	ValidationRootKey  ValidationContextKey = "validation_root"
)

// GetValidationDepth extracts validation depth from context
func GetValidationDepth(ctx context.Context) int {
	if depth, ok := ctx.Value(ValidationDepthKey).(int); ok {
		return depth
	}
	return 0
}

// GetValidationPath extracts validation path from context
func GetValidationPath(ctx context.Context) string {
	if path, ok := ctx.Value(ValidationPathKey).(string); ok {
		return path
	}
	return ""
}

// WithValidationPath adds a path component to the validation context
func WithValidationPath(ctx context.Context, path string) context.Context {
	currentPath := GetValidationPath(ctx)
	if currentPath == "" {
		return context.WithValue(ctx, ValidationPathKey, path)
	}
	
	// Build hierarchical path
	var fullPath string
	if strings.HasPrefix(path, "[") {
		fullPath = currentPath + path
	} else {
		fullPath = currentPath + "." + path
	}
	
	return context.WithValue(ctx, ValidationPathKey, fullPath)
}

// WithValidationDepth increments validation depth in context
func WithValidationDepth(ctx context.Context) context.Context {
	depth := GetValidationDepth(ctx) + 1
	return context.WithValue(ctx, ValidationDepthKey, depth)
}