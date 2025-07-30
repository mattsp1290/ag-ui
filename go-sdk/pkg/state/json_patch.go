package state

import (
	"fmt"
	"reflect"
	"strings"
)

// JSONPatchOp represents the JSON Patch operation type
type JSONPatchOp string

const (
	// JSONPatchOpAdd adds a value at the specified location
	JSONPatchOpAdd JSONPatchOp = "add"
	// JSONPatchOpRemove removes the value at the specified location
	JSONPatchOpRemove JSONPatchOp = "remove"
	// JSONPatchOpReplace replaces the value at the specified location
	JSONPatchOpReplace JSONPatchOp = "replace"
	// JSONPatchOpMove moves the value from one location to another
	JSONPatchOpMove JSONPatchOp = "move"
	// JSONPatchOpCopy copies the value from one location to another
	JSONPatchOpCopy JSONPatchOp = "copy"
	// JSONPatchOpTest tests that the value at the specified location equals the provided value
	JSONPatchOpTest JSONPatchOp = "test"
)

// JSONPatchOperation represents a single JSON Patch operation (RFC 6902)
type JSONPatchOperation struct {
	Op    JSONPatchOp `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
	From  string      `json:"from,omitempty"`
}

// JSONPatch represents a collection of JSON Patch operations
type JSONPatch []JSONPatchOperation

// Validate validates the JSON Patch operations
func (p JSONPatch) Validate() error {
	for i, op := range p {
		if err := op.Validate(); err != nil {
			return fmt.Errorf("invalid operation at index %d: %w", i, err)
		}
	}
	return nil
}

// Validate validates a single JSON Patch operation
func (op JSONPatchOperation) Validate() error {
	// Validate operation type
	switch op.Op {
	case JSONPatchOpAdd, JSONPatchOpRemove, JSONPatchOpReplace,
		JSONPatchOpMove, JSONPatchOpCopy, JSONPatchOpTest:
		// Valid operation
	default:
		return fmt.Errorf("invalid operation type: %s", op.Op)
	}

	// Validate path
	if err := validateJSONPointer(op.Path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Validate operation-specific requirements
	switch op.Op {
	case JSONPatchOpAdd, JSONPatchOpReplace, JSONPatchOpTest:
		// These operations can have any value, including null
	case JSONPatchOpMove, JSONPatchOpCopy:
		if op.From == "" {
			return fmt.Errorf("from field is required for %s operation", op.Op)
		}
		if err := validateJSONPointer(op.From); err != nil {
			return fmt.Errorf("invalid from path: %w", err)
		}
		if op.Op == JSONPatchOpMove && op.Path == op.From {
			return fmt.Errorf("move operation cannot have the same path and from")
		}
	}

	return nil
}

// Apply applies the JSON Patch to the given document
func (p JSONPatch) Apply(document interface{}) (interface{}, error) {
	// Validate the patch first
	if err := p.Validate(); err != nil {
		return nil, err
	}

	// Create a deep copy of the document to avoid modifying the original
	// and ensure thread safety for concurrent operations
	doc := deepCopy(document)

	// Apply each operation
	for i, op := range p {
		var err error
		doc, err = op.Apply(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to apply operation at index %d: %w", i, err)
		}
	}

	return doc, nil
}

// Apply applies a single JSON Patch operation to the document
func (op JSONPatchOperation) Apply(document interface{}) (interface{}, error) {
	switch op.Op {
	case JSONPatchOpAdd:
		return applyAdd(document, op.Path, op.Value)
	case JSONPatchOpRemove:
		return applyRemove(document, op.Path)
	case JSONPatchOpReplace:
		return applyReplace(document, op.Path, op.Value)
	case JSONPatchOpMove:
		return applyMove(document, op.From, op.Path)
	case JSONPatchOpCopy:
		return applyCopy(document, op.From, op.Path)
	case JSONPatchOpTest:
		return applyTest(document, op.Path, op.Value)
	default:
		return nil, fmt.Errorf("unknown operation: %s", op.Op)
	}
}

// applyAdd applies an add operation
func applyAdd(document interface{}, path string, value interface{}) (interface{}, error) {
	if path == "" || path == "/" {
		// Replace the entire document
		return normalizeJSONValue(value), nil
	}

	tokens := parseJSONPointer(path)
	if len(tokens) == 0 {
		return normalizeJSONValue(value), nil
	}

	parent, lastToken, err := getParent(document, tokens)
	if err != nil {
		return nil, err
	}

	switch p := parent.(type) {
	case map[string]interface{}:
		p[lastToken] = normalizeJSONValue(value)
	case []interface{}:
		idx, isAppend, err := parseArrayIndex(lastToken, len(p))
		if err != nil {
			return nil, err
		}
		if isAppend {
			parent = append(p, normalizeJSONValue(value))
		} else {
			// Insert at index
			parent = append(p[:idx], append([]interface{}{normalizeJSONValue(value)}, p[idx:]...)...)
		}
		// Update the parent in the document
		if len(tokens) > 1 {
			if err := setValueAtPath(document, tokens[:len(tokens)-1], parent); err != nil {
				return nil, err
			}
		} else {
			document = parent
		}
	default:
		return nil, fmt.Errorf("cannot add to non-object/non-array at path %s", path)
	}

	return document, nil
}

// applyRemove applies a remove operation
func applyRemove(document interface{}, path string) (interface{}, error) {
	if path == "" || path == "/" {
		return nil, fmt.Errorf("cannot remove root document")
	}

	tokens := parseJSONPointer(path)
	parent, lastToken, err := getParent(document, tokens)
	if err != nil {
		return nil, err
	}

	switch p := parent.(type) {
	case map[string]interface{}:
		if _, exists := p[lastToken]; !exists {
			return nil, fmt.Errorf("path %s does not exist", path)
		}
		delete(p, lastToken)
	case []interface{}:
		idx, _, err := parseArrayIndex(lastToken, len(p))
		if err != nil {
			return nil, err
		}
		if idx < 0 || idx >= len(p) {
			return nil, fmt.Errorf("array index out of bounds: %d", idx)
		}
		parent = append(p[:idx], p[idx+1:]...)
		// Update the parent in the document
		if len(tokens) > 1 {
			if err := setValueAtPath(document, tokens[:len(tokens)-1], parent); err != nil {
				return nil, err
			}
		} else {
			document = parent
		}
	default:
		return nil, fmt.Errorf("cannot remove from non-object/non-array")
	}

	return document, nil
}

// applyReplace applies a replace operation
func applyReplace(document interface{}, path string, value interface{}) (interface{}, error) {
	if path == "" || path == "/" {
		return normalizeJSONValue(value), nil
	}

	// First check if the path exists
	if _, err := getValueAtPath(document, path); err != nil {
		return nil, fmt.Errorf("path %s does not exist", path)
	}

	// Remove then add (applyAdd already normalizes)
	doc, err := applyRemove(document, path)
	if err != nil {
		return nil, err
	}
	return applyAdd(doc, path, value)
}

// applyMove applies a move operation
func applyMove(document interface{}, from, path string) (interface{}, error) {
	// Get the value to move
	value, err := getValueAtPath(document, from)
	if err != nil {
		return nil, fmt.Errorf("source path %s does not exist", from)
	}

	// Make a deep copy of the value
	valueCopy := deepCopy(value)

	// Remove from source
	doc, err := applyRemove(document, from)
	if err != nil {
		return nil, err
	}

	// Add to destination
	return applyAdd(doc, path, valueCopy)
}

// applyCopy applies a copy operation
func applyCopy(document interface{}, from, path string) (interface{}, error) {
	// Get the value to copy
	value, err := getValueAtPath(document, from)
	if err != nil {
		return nil, fmt.Errorf("source path %s does not exist", from)
	}

	// Make a deep copy of the value
	valueCopy := deepCopy(value)

	// Add to destination
	return applyAdd(document, path, valueCopy)
}

// applyTest applies a test operation
func applyTest(document interface{}, path string, expected interface{}) (interface{}, error) {
	actual, err := getValueAtPath(document, path)
	if err != nil {
		return nil, fmt.Errorf("test failed: %w", err)
	}

	if !reflect.DeepEqual(actual, expected) {
		return nil, fmt.Errorf("test failed: expected %v, got %v", expected, actual)
	}

	return document, nil
}

// deepCopy creates a deep copy of a value using reflection for better performance
func deepCopy(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	return deepCopyValue(reflect.ValueOf(v)).Interface()
}

// deepCopyValue performs deep copy using reflection
func deepCopyValue(original reflect.Value) reflect.Value {
	if !original.IsValid() {
		return reflect.Value{}
	}

	switch original.Kind() {
	case reflect.Interface:
		if original.IsNil() {
			return original
		}
		return deepCopyValue(original.Elem())

	case reflect.Ptr:
		if original.IsNil() {
			return original
		}
		copy := reflect.New(original.Type().Elem())
		copy.Elem().Set(deepCopyValue(original.Elem()))
		return copy

	case reflect.Slice:
		if original.IsNil() {
			return original
		}
		copy := reflect.MakeSlice(original.Type(), original.Len(), original.Cap())
		for i := 0; i < original.Len(); i++ {
			copy.Index(i).Set(deepCopyValue(original.Index(i)))
		}
		return copy

	case reflect.Map:
		if original.IsNil() {
			return original
		}
		copy := reflect.MakeMap(original.Type())
		for _, key := range original.MapKeys() {
			copy.SetMapIndex(key, deepCopyValue(original.MapIndex(key)))
		}
		return copy

	case reflect.Struct:
		copy := reflect.New(original.Type()).Elem()
		for i := 0; i < original.NumField(); i++ {
			if original.Type().Field(i).PkgPath == "" { // exported field
				copy.Field(i).Set(deepCopyValue(original.Field(i)))
			}
		}
		return copy

	case reflect.Array:
		copy := reflect.New(original.Type()).Elem()
		for i := 0; i < original.Len(); i++ {
			copy.Index(i).Set(deepCopyValue(original.Index(i)))
		}
		return copy

	default:
		// For basic types (string, int, float, bool, etc.), just return the value
		return original
	}
}

// getParent gets the parent container and the last token of a path
func getParent(document interface{}, tokens []string) (interface{}, string, error) {
	if len(tokens) == 0 {
		return nil, "", fmt.Errorf("empty path")
	}

	if len(tokens) == 1 {
		return document, tokens[0], nil
	}

	parent, err := getValueAtPathTokens(document, tokens[:len(tokens)-1])
	if err != nil {
		return nil, "", fmt.Errorf("failed to get parent at path: %w", err)
	}

	if parent == nil {
		return nil, "", fmt.Errorf("parent is nil")
	}

	return parent, tokens[len(tokens)-1], nil
}

// getValueAtPath gets the value at the specified JSON Pointer path
func getValueAtPath(document interface{}, path string) (interface{}, error) {
	if path == "" || path == "/" {
		return document, nil
	}

	tokens := parseJSONPointer(path)
	return getValueAtPathTokens(document, tokens)
}

// getValueAtPathTokens gets the value at the specified path tokens
func getValueAtPathTokens(document interface{}, tokens []string) (interface{}, error) {
	if document == nil {
		return nil, fmt.Errorf("document is nil")
	}

	current := document

	for i, token := range tokens {
		if current == nil {
			return nil, fmt.Errorf("encountered nil value at path segment %d", i)
		}

		switch c := current.(type) {
		case map[string]interface{}:
			if c == nil {
				return nil, fmt.Errorf("map is nil at path segment %d", i)
			}
			val, exists := c[token]
			if !exists {
				return nil, fmt.Errorf("key %s not found at path segment %d", token, i)
			}
			current = val
		case []interface{}:
			if c == nil {
				return nil, fmt.Errorf("slice is nil at path segment %d", i)
			}
			idx, _, err := parseArrayIndex(token, len(c))
			if err != nil {
				return nil, fmt.Errorf("invalid array index at path segment %d: %w", i, err)
			}
			if idx < 0 || idx >= len(c) {
				return nil, fmt.Errorf("array index %d out of bounds [0,%d) at path segment %d", idx, len(c), i)
			}
			current = c[idx]
		default:
			return nil, fmt.Errorf("cannot index into %T at path segment %d (token: %s)", current, i, token)
		}
	}

	return current, nil
}

// setValueAtPath sets a value at the specified path
func setValueAtPath(document interface{}, tokens []string, value interface{}) error {
	if len(tokens) == 0 {
		return fmt.Errorf("cannot set root document")
	}

	parent, err := getValueAtPathTokens(document, tokens[:len(tokens)-1])
	if err != nil {
		return err
	}

	lastToken := tokens[len(tokens)-1]

	switch p := parent.(type) {
	case map[string]interface{}:
		p[lastToken] = value
	case []interface{}:
		idx, _, err := parseArrayIndex(lastToken, len(p))
		if err != nil {
			return err
		}
		if idx < 0 || idx >= len(p) {
			return fmt.Errorf("array index out of bounds: %d", idx)
		}
		p[idx] = value
	default:
		return fmt.Errorf("cannot set value in %T", parent)
	}

	return nil
}

// parseArrayIndex parses an array index token
func parseArrayIndex(token string, length int) (int, bool, error) {
	if token == "-" {
		return length, true, nil // Append
	}

	// Parse as integer
	var idx int
	if _, err := fmt.Sscanf(token, "%d", &idx); err != nil {
		return 0, false, fmt.Errorf("invalid array index: %s", token)
	}

	// Negative indices are not allowed in JSON Patch
	if idx < 0 {
		return 0, false, fmt.Errorf("negative array index not allowed: %s", token)
	}

	return idx, false, nil
}

// parseJSONPointer parses a JSON Pointer into tokens
func parseJSONPointer(pointer string) []string {
	if pointer == "" {
		return []string{}
	}

	if !strings.HasPrefix(pointer, "/") {
		return []string{}
	}

	// Remove leading slash and split
	pointer = pointer[1:]
	if pointer == "" {
		return []string{""}
	}

	tokens := strings.Split(pointer, "/")

	// Unescape tokens
	for i, token := range tokens {
		tokens[i] = unescapeJSONPointer(token)
	}

	return tokens
}

// unescapeJSONPointer unescapes a JSON Pointer token
func unescapeJSONPointer(token string) string {
	// Replace ~1 with / and ~0 with ~
	token = strings.ReplaceAll(token, "~1", "/")
	token = strings.ReplaceAll(token, "~0", "~")
	return token
}

// normalizeJSONValue normalizes a value to match JSON conventions
// This ensures all numeric types are converted to float64 as JSON doesn't distinguish between int and float
func normalizeJSONValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case map[string]interface{}:
		normalized := make(map[string]interface{}, len(v))
		for key, val := range v {
			normalized[key] = normalizeJSONValue(val)
		}
		return normalized
	case []interface{}:
		normalized := make([]interface{}, len(v))
		for i, val := range v {
			normalized[i] = normalizeJSONValue(val)
		}
		return normalized
	case int:
		// Convert to float64 for JSON consistency
		return float64(v)
	case int8:
		return float64(v) // Convert to float64 for JSON consistency
	case int16:
		return float64(v) // Convert to float64 for JSON consistency
	case int32:
		return float64(v) // Convert to float64 for JSON consistency
	case int64:
		return float64(v) // Convert to float64 for JSON consistency
	case uint:
		return float64(v) // Convert to float64 for JSON consistency
	case uint8:
		return float64(v) // Convert to float64 for JSON consistency
	case uint16:
		return float64(v) // Convert to float64 for JSON consistency
	case uint32:
		return float64(v) // Convert to float64 for JSON consistency
	case uint64:
		return float64(v) // Convert to float64 for JSON consistency
	default:
		return v
	}
}

// validateJSONPointer validates a JSON Pointer with comprehensive checks
func validateJSONPointer(pointer string) error {
	if pointer == "" {
		return nil // Empty pointer is valid (refers to root)
	}

	if !strings.HasPrefix(pointer, "/") {
		return fmt.Errorf("JSON Pointer must start with '/' or be empty")
	}

	// Check for dangerous patterns
	if strings.Contains(pointer, "..") {
		return fmt.Errorf("JSON Pointer contains path traversal pattern (..)")
	}

	if strings.Contains(pointer, "//") {
		return fmt.Errorf("JSON Pointer contains empty path segments")
	}

	// Check for control characters
	for i, r := range pointer {
		if r < 32 && r != 9 && r != 10 && r != 13 { // Allow tab, LF, CR
			return fmt.Errorf("JSON Pointer contains control character at position %d", i)
		}
	}

	return nil
}
