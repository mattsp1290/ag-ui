package state

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

// SecurityConfig defines security limits and validation rules
type SecurityConfig struct {
	MaxStateSize      int64         // Maximum state size in bytes
	MaxMetadataSize   int64         // Maximum metadata size in bytes
	MaxPatchSize      int64         // Maximum patch size in bytes
	MaxDepth          int           // Maximum nesting depth
	MaxStringLength   int           // Maximum string length
	MaxArrayLength    int           // Maximum array length
	MaxObjectKeys     int           // Maximum object keys
	AllowedOperations []JSONPatchOp // Allowed patch operations
	ForbiddenPaths    []string      // Forbidden JSON Pointer paths
}

// Security limit constants are defined in constants.go

// DefaultSecurityConfig returns safe default security settings
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		MaxStateSize:    MaxStateSizeBytes,    // 10MB
		MaxMetadataSize: MaxMetadataSizeBytes, // 1MB
		MaxPatchSize:    MaxPatchSizeBytes,    // 1MB
		MaxDepth:        MaxJSONDepth,         // 10 levels deep
		MaxStringLength: MaxStringLengthBytes, // 64KB strings
		MaxArrayLength:  MaxArrayLength,       // 10k elements
		MaxObjectKeys:   MaxObjectKeys,        // 1k keys
		AllowedOperations: []JSONPatchOp{
			JSONPatchOpAdd, JSONPatchOpRemove, JSONPatchOpReplace,
			JSONPatchOpMove, JSONPatchOpCopy, JSONPatchOpTest,
		},
		ForbiddenPaths: []string{
			"/admin", "/config", "/secrets", "/internal",
		},
	}
}

// SecurityValidator validates inputs for security compliance
type SecurityValidator struct {
	config SecurityConfig
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator(config SecurityConfig) *SecurityValidator {
	return &SecurityValidator{config: config}
}

// ValidateJSONPointer validates a JSON Pointer path for security
func (sv *SecurityValidator) ValidateJSONPointer(pointer string) error {
	// Basic validation
	if err := validateJSONPointer(pointer); err != nil {
		return fmt.Errorf("invalid JSON pointer: %w", err)
	}

	// Check forbidden paths
	for _, forbidden := range sv.config.ForbiddenPaths {
		if strings.HasPrefix(pointer, forbidden) {
			return fmt.Errorf("access to path %s is forbidden", pointer)
		}
	}

	// Check for suspicious patterns
	if strings.Contains(pointer, "..") {
		return fmt.Errorf("path traversal patterns not allowed")
	}

	if strings.Contains(pointer, "//") {
		return fmt.Errorf("empty path segments not allowed")
	}

	// Check for control characters and non-printable characters
	for i, r := range pointer {
		if !unicode.IsPrint(r) && r != '/' {
			return fmt.Errorf("non-printable character at position %d", i)
		}
	}

	return nil
}

// ValidatePatch validates a JSON Patch for security
func (sv *SecurityValidator) ValidatePatch(patch JSONPatch) error {
	if patch == nil {
		return fmt.Errorf("patch cannot be nil")
	}

	// Check patch size
	patchSize, err := sv.calculateSize(patch)
	if err != nil {
		return fmt.Errorf("failed to calculate patch size: %w", err)
	}

	if patchSize > sv.config.MaxPatchSize {
		return ErrPatchTooLarge
	}

	// Check JSON depth of the entire patch
	if err := sv.ValidateJSONDepth(patch); err != nil {
		return err
	}

	// Validate each operation
	for i, op := range patch {
		if err := sv.validatePatchOperation(op); err != nil {
			return fmt.Errorf("invalid operation at index %d: %w", i, err)
		}
	}

	return nil
}

// validatePatchOperation validates a single patch operation
func (sv *SecurityValidator) validatePatchOperation(op JSONPatchOperation) error {
	// Check if operation is allowed
	allowed := false
	for _, allowedOp := range sv.config.AllowedOperations {
		if op.Op == allowedOp {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("operation %s is not allowed", op.Op)
	}

	// Validate path
	if err := sv.ValidateJSONPointer(op.Path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Validate 'from' path for move/copy operations
	if op.Op == JSONPatchOpMove || op.Op == JSONPatchOpCopy {
		if err := sv.ValidateJSONPointer(op.From); err != nil {
			return fmt.Errorf("invalid from path: %w", err)
		}
	}

	// Validate value size and structure
	if op.Op == JSONPatchOpAdd || op.Op == JSONPatchOpReplace || op.Op == JSONPatchOpTest {
		if err := sv.validateValue(op.Value, 0); err != nil {
			return fmt.Errorf("invalid value: %w", err)
		}
	}

	return nil
}

// ValidateState validates a complete state object
func (sv *SecurityValidator) ValidateState(state interface{}) error {
	if state == nil {
		return nil // nil state is allowed
	}

	// Check state size
	stateSize, err := sv.calculateSize(state)
	if err != nil {
		return fmt.Errorf("failed to calculate state size: %w", err)
	}

	if stateSize > sv.config.MaxStateSize {
		return fmt.Errorf("state size %d exceeds limit %d", stateSize, sv.config.MaxStateSize)
	}

	// Validate structure
	return sv.validateValue(state, 0)
}

// ValidateMetadata validates metadata object
func (sv *SecurityValidator) ValidateMetadata(metadata map[string]interface{}) error {
	if metadata == nil {
		return nil
	}

	// Check metadata size
	metadataSize, err := sv.calculateSize(metadata)
	if err != nil {
		return fmt.Errorf("failed to calculate metadata size: %w", err)
	}

	if metadataSize > sv.config.MaxMetadataSize {
		return fmt.Errorf("metadata size %d exceeds limit %d", metadataSize, sv.config.MaxMetadataSize)
	}

	// Validate structure
	return sv.validateValue(metadata, 0)
}

// validateValue validates a value recursively
func (sv *SecurityValidator) validateValue(value interface{}, depth int) error {
	if depth > sv.config.MaxDepth {
		return fmt.Errorf("nesting depth %d exceeds limit %d", depth, sv.config.MaxDepth)
	}

	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		if len(v) > sv.config.MaxStringLength {
			return ErrStringTooLong
		}

		// Check for suspicious patterns in strings
		if strings.Contains(v, "<script") || strings.Contains(v, "javascript:") {
			return fmt.Errorf("potentially malicious content detected in string")
		}

	case []interface{}:
		if len(v) > sv.config.MaxArrayLength {
			return ErrArrayTooLong
		}

		for i, item := range v {
			if err := sv.validateValue(item, depth+1); err != nil {
				return fmt.Errorf("invalid array item at index %d: %w", i, err)
			}
		}

	case map[string]interface{}:
		if len(v) > sv.config.MaxObjectKeys {
			return ErrTooManyKeys
		}

		for key, val := range v {
			// Validate key
			if len(key) > sv.config.MaxStringLength {
				return ErrStringTooLong
			}

			// Check for suspicious key patterns
			if strings.Contains(key, "..") || strings.Contains(key, "/") {
				return fmt.Errorf("suspicious pattern in object key: %s", key)
			}

			// Validate value
			if err := sv.validateValue(val, depth+1); err != nil {
				return fmt.Errorf("invalid object value for key %s: %w", key, err)
			}
		}

	case float64, int, int32, int64, bool:
		// Basic types are always allowed

	default:
		// Check if it's a valid JSON type using reflection
		rv := reflect.ValueOf(value)
		if !sv.isJSONCompatible(rv) {
			return fmt.Errorf("unsupported value type: %T", value)
		}
	}

	return nil
}

// isJSONCompatible checks if a value is JSON-compatible
func (sv *SecurityValidator) isJSONCompatible(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return true
	case reflect.Slice, reflect.Array:
		return true // Will be validated recursively
	case reflect.Map:
		return v.Type().Key().Kind() == reflect.String
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return true
		}
		return sv.isJSONCompatible(v.Elem())
	default:
		return false
	}
}

// calculateSize estimates the size of a value in bytes
func (sv *SecurityValidator) calculateSize(value interface{}) (int64, error) {
	// Use JSON marshaling to get approximate size
	data, err := json.Marshal(value)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal value: %w", err)
	}

	return int64(len(data)), nil
}

// SanitizeString removes potentially dangerous content from strings
func (sv *SecurityValidator) SanitizeString(s string) string {
	// Remove control characters except tab, newline, carriage return
	var result strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) || r == '\t' || r == '\n' || r == '\r' {
			result.WriteRune(r)
		}
	}

	sanitized := result.String()

	// Remove potentially dangerous patterns
	sanitized = strings.ReplaceAll(sanitized, "<script", "&lt;script")
	sanitized = strings.ReplaceAll(sanitized, "javascript:", "")
	sanitized = strings.ReplaceAll(sanitized, "data:text/html", "")

	return sanitized
}

// GetJSONDepth calculates the maximum depth of a JSON structure
func (sv *SecurityValidator) GetJSONDepth(data interface{}) (int, error) {
	if data == nil {
		return 0, nil
	}

	// Marshal to JSON to ensure valid structure
	jsonData, err := json.Marshal(data)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal data: %w", err)
	}

	// Parse and calculate depth
	var parsed interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		return 0, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return sv.calculateDepth(parsed, 0), nil
}

// calculateDepth recursively calculates the depth of a value
func (sv *SecurityValidator) calculateDepth(value interface{}, currentDepth int) int {
	if value == nil {
		return currentDepth
	}

	maxDepth := currentDepth

	switch v := value.(type) {
	case map[string]interface{}:
		for _, val := range v {
			depth := sv.calculateDepth(val, currentDepth+1)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	case []interface{}:
		for _, item := range v {
			depth := sv.calculateDepth(item, currentDepth+1)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	default:
		// Primitive type, no increase in depth
		maxDepth = currentDepth
	}

	return maxDepth
}

// ValidateJSONDepth checks if the JSON depth exceeds the configured limit
func (sv *SecurityValidator) ValidateJSONDepth(data interface{}) error {
	depth, err := sv.GetJSONDepth(data)
	if err != nil {
		return fmt.Errorf("failed to calculate JSON depth: %w", err)
	}

	if depth > sv.config.MaxDepth {
		return fmt.Errorf("JSON depth %d exceeds maximum allowed depth %d", depth, sv.config.MaxDepth)
	}

	return nil
}

// EstimateSize provides a more accurate size estimation for complex objects
func (sv *SecurityValidator) EstimateSize(value interface{}) (int64, error) {
	// For nil values
	if value == nil {
		return 4, nil // "null"
	}

	switch v := value.(type) {
	case string:
		// Account for JSON string encoding overhead
		return int64(len(v) + 2), nil // quotes
	case []interface{}:
		size := int64(2) // "[]"
		for i, item := range v {
			itemSize, err := sv.EstimateSize(item)
			if err != nil {
				return 0, err
			}
			size += itemSize
			if i < len(v)-1 {
				size++ // comma
			}
		}
		return size, nil
	case map[string]interface{}:
		size := int64(2) // "{}"
		i := 0
		for key, val := range v {
			// Key size with quotes and colon
			size += int64(len(key) + 3) // "key":

			valSize, err := sv.EstimateSize(val)
			if err != nil {
				return 0, err
			}
			size += valSize

			if i < len(v)-1 {
				size++ // comma
			}
			i++
		}
		return size, nil
	default:
		// Use JSON marshaling for other types
		data, err := json.Marshal(v)
		if err != nil {
			return 0, fmt.Errorf("failed to estimate size: %w", err)
		}
		return int64(len(data)), nil
	}
}
