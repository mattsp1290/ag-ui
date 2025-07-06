package state

import (
	"encoding/json"
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

	// Convert document to a modifiable structure
	var doc interface{}
	jsonBytes, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal document: %w", err)
	}

	// Apply each operation
	for i, op := range p {
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
		return value, nil
	}

	tokens := parseJSONPointer(path)
	if len(tokens) == 0 {
		return value, nil
	}

	parent, lastToken, err := getParent(document, tokens)
	if err != nil {
		return nil, err
	}

	switch p := parent.(type) {
	case map[string]interface{}:
		p[lastToken] = value
	case []interface{}:
		idx, isAppend, err := parseArrayIndex(lastToken, len(p))
		if err != nil {
			return nil, err
		}
		if isAppend {
			parent = append(p, value)
		} else {
			// Insert at index
			parent = append(p[:idx], append([]interface{}{value}, p[idx:]...)...)
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
		return value, nil
	}

	// First check if the path exists
	if _, err := getValueAtPath(document, path); err != nil {
		return nil, fmt.Errorf("path %s does not exist", path)
	}

	// Remove then add
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

// deepCopy creates a deep copy of a value
func deepCopy(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	// Use JSON marshal/unmarshal for deep copy
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}

	var copy interface{}
	if err := json.Unmarshal(data, &copy); err != nil {
		return v
	}

	return copy
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
		return nil, "", err
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
	current := document

	for _, token := range tokens {
		switch c := current.(type) {
		case map[string]interface{}:
			val, exists := c[token]
			if !exists {
				return nil, fmt.Errorf("key %s not found", token)
			}
			current = val
		case []interface{}:
			idx, _, err := parseArrayIndex(token, len(c))
			if err != nil {
				return nil, err
			}
			if idx < 0 || idx >= len(c) {
				return nil, fmt.Errorf("array index out of bounds: %d", idx)
			}
			current = c[idx]
		default:
			return nil, fmt.Errorf("cannot index into %T", current)
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

// validateJSONPointer validates a JSON Pointer
func validateJSONPointer(pointer string) error {
	if pointer == "" {
		return nil // Empty pointer is valid (refers to root)
	}

	if !strings.HasPrefix(pointer, "/") {
		return fmt.Errorf("JSON Pointer must start with '/' or be empty")
	}

	return nil
}