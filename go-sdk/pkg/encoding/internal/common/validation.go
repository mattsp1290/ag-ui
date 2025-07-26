package common

import (
	"reflect"
	"unicode/utf8"
)

// ValidateNonNil checks if a value is not nil
func ValidateNonNil(value interface{}, fieldName string) error {
	if value == nil {
		return WrapError(ErrNilInput, "field %s cannot be nil", fieldName)
	}
	
	// Check for nil pointers, slices, maps, etc.
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return WrapError(ErrNilInput, "field %s cannot be nil", fieldName)
	}
	
	return nil
}

// ValidateSliceNotEmpty checks if a slice is not empty
func ValidateSliceNotEmpty(slice interface{}, fieldName string) error {
	if slice == nil {
		return WrapError(ErrNilInput, "slice %s cannot be nil", fieldName)
	}
	
	v := reflect.ValueOf(slice)
	if v.Kind() != reflect.Slice {
		return WrapError(ErrUnsupportedType, "field %s is not a slice", fieldName)
	}
	
	if v.Len() == 0 {
		return WrapError(ErrInvalidSize, "slice %s cannot be empty", fieldName)
	}
	
	return nil
}

// ValidateBufferSize checks if a buffer size is valid
func ValidateBufferSize(size int, fieldName string) error {
	if size < 0 {
		return WrapError(ErrInvalidSize, "buffer size %s cannot be negative: %d", fieldName, size)
	}
	
	// Check for reasonable maximum size (1GB)
	const maxBufferSize = 1 << 30
	if size > maxBufferSize {
		return WrapError(ErrInvalidSize, "buffer size %s too large: %d", fieldName, size)
	}
	
	return nil
}

// ValidateStringUTF8 checks if a string is valid UTF-8
func ValidateStringUTF8(str string, fieldName string) error {
	if !utf8.ValidString(str) {
		return WrapError(ErrInvalidFormat, "string %s is not valid UTF-8", fieldName)
	}
	return nil
}

// ValidateStringLength checks if a string length is within bounds
func ValidateStringLength(str string, minLen, maxLen int, fieldName string) error {
	length := len(str)
	if length < minLen {
		return WrapError(ErrInvalidSize, "string %s too short: %d < %d", fieldName, length, minLen)
	}
	if maxLen > 0 && length > maxLen {
		return WrapError(ErrInvalidSize, "string %s too long: %d > %d", fieldName, length, maxLen)
	}
	return nil
}

// ValidateRange checks if a value is within a specified range
func ValidateRange(value, min, max int, fieldName string) error {
	if value < min {
		return WrapError(ErrInvalidSize, "value %s too small: %d < %d", fieldName, value, min)
	}
	if value > max {
		return WrapError(ErrInvalidSize, "value %s too large: %d > %d", fieldName, value, max)
	}
	return nil
}

// ValidateMapNotEmpty checks if a map is not empty
func ValidateMapNotEmpty(m interface{}, fieldName string) error {
	if m == nil {
		return WrapError(ErrNilInput, "map %s cannot be nil", fieldName)
	}
	
	v := reflect.ValueOf(m)
	if v.Kind() != reflect.Map {
		return WrapError(ErrUnsupportedType, "field %s is not a map", fieldName)
	}
	
	if v.Len() == 0 {
		return WrapError(ErrInvalidSize, "map %s cannot be empty", fieldName)
	}
	
	return nil
}

// ValidateStructNotZero checks if a struct is not zero-valued
func ValidateStructNotZero(s interface{}, fieldName string) error {
	if s == nil {
		return WrapError(ErrNilInput, "struct %s cannot be nil", fieldName)
	}
	
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return WrapError(ErrNilInput, "struct %s cannot be nil", fieldName)
		}
		v = v.Elem()
	}
	
	if v.Kind() != reflect.Struct {
		return WrapError(ErrUnsupportedType, "field %s is not a struct", fieldName)
	}
	
	if v.IsZero() {
		return WrapError(ErrInvalidFormat, "struct %s cannot be zero-valued", fieldName)
	}
	
	return nil
}

// ValidateEnum checks if a value is within a set of valid enum values
func ValidateEnum(value interface{}, validValues []interface{}, fieldName string) error {
	for _, valid := range validValues {
		if reflect.DeepEqual(value, valid) {
			return nil
		}
	}
	
	return WrapError(ErrInvalidFormat, "field %s has invalid value: %v", fieldName, value)
}

// ValidationRule represents a validation rule
type ValidationRule struct {
	Name      string
	Validator func() error
}

// ValidateAll validates multiple rules and returns the first error encountered
func ValidateAll(rules ...ValidationRule) error {
	for _, rule := range rules {
		if err := rule.Validator(); err != nil {
			return WrapError(err, "validation failed for rule: %s", rule.Name)
		}
	}
	return nil
}

// ValidateAny validates multiple rules and returns error only if all fail
func ValidateAny(rules ...ValidationRule) error {
	var lastError error
	for _, rule := range rules {
		if err := rule.Validator(); err == nil {
			return nil
		} else {
			lastError = err
		}
	}
	
	if lastError != nil {
		return WrapError(lastError, "all validation rules failed")
	}
	
	return nil
}