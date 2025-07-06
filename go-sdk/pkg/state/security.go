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
	MaxStateSize      int64 // Maximum state size in bytes
	MaxMetadataSize   int64 // Maximum metadata size in bytes  
	MaxPatchSize      int64 // Maximum patch size in bytes
	MaxDepth          int   // Maximum nesting depth
	MaxStringLength   int   // Maximum string length
	MaxArrayLength    int   // Maximum array length
	MaxObjectKeys     int   // Maximum object keys
	AllowedOperations []JSONPatchOp // Allowed patch operations
	ForbiddenPaths    []string // Forbidden JSON Pointer paths
}

// DefaultSecurityConfig returns safe default security settings
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		MaxStateSize:      10 * 1024 * 1024, // 10MB
		MaxMetadataSize:   1024 * 1024,      // 1MB
		MaxPatchSize:      1024 * 1024,      // 1MB
		MaxDepth:          10,               // 10 levels deep
		MaxStringLength:   1024 * 1024,      // 1MB strings
		MaxArrayLength:    10000,            // 10k elements
		MaxObjectKeys:     1000,             // 1k keys
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
		return fmt.Errorf("patch size %d exceeds limit %d", patchSize, sv.config.MaxPatchSize)
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
			return fmt.Errorf("string length %d exceeds limit %d", len(v), sv.config.MaxStringLength)
		}
		
		// Check for suspicious patterns in strings
		if strings.Contains(v, "<script") || strings.Contains(v, "javascript:") {
			return fmt.Errorf("potentially malicious content detected in string")
		}
		
	case []interface{}:
		if len(v) > sv.config.MaxArrayLength {
			return fmt.Errorf("array length %d exceeds limit %d", len(v), sv.config.MaxArrayLength)
		}
		
		for i, item := range v {
			if err := sv.validateValue(item, depth+1); err != nil {
				return fmt.Errorf("invalid array item at index %d: %w", i, err)
			}
		}
		
	case map[string]interface{}:
		if len(v) > sv.config.MaxObjectKeys {
			return fmt.Errorf("object key count %d exceeds limit %d", len(v), sv.config.MaxObjectKeys)
		}
		
		for key, val := range v {
			// Validate key
			if len(key) > sv.config.MaxStringLength {
				return fmt.Errorf("object key length %d exceeds limit %d", len(key), sv.config.MaxStringLength)
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