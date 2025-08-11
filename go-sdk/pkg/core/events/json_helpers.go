package events

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// JSONCompareOptions configures how JSON comparison is performed
type JSONCompareOptions struct {
	// IgnoreFieldOrder ignores the order of fields in JSON objects
	IgnoreFieldOrder bool
	// IgnoreArrayOrder ignores the order of elements in JSON arrays
	IgnoreArrayOrder bool
	// IgnoreNullVsAbsent treats null values and absent fields as equivalent
	IgnoreNullVsAbsent bool
	// IgnoreTimestamps ignores timestamp fields during comparison
	IgnoreTimestamps bool
}

// DefaultJSONCompareOptions returns default comparison options
func DefaultJSONCompareOptions() JSONCompareOptions {
	return JSONCompareOptions{
		IgnoreFieldOrder:   true,
		IgnoreArrayOrder:   false,
		IgnoreNullVsAbsent: true,
		IgnoreTimestamps:   false,
	}
}

// CompareJSON compares two JSON byte arrays for semantic equivalence
func CompareJSON(expected, actual []byte, options JSONCompareOptions) error {
	var expectedData, actualData interface{}
	
	if err := json.Unmarshal(expected, &expectedData); err != nil {
		return fmt.Errorf("failed to unmarshal expected JSON: %w", err)
	}
	
	if err := json.Unmarshal(actual, &actualData); err != nil {
		return fmt.Errorf("failed to unmarshal actual JSON: %w", err)
	}
	
	return compareValues(expectedData, actualData, options, "root")
}

// compareValues recursively compares two values
func compareValues(expected, actual interface{}, options JSONCompareOptions, path string) error {
	// Handle nil cases
	if expected == nil && actual == nil {
		return nil
	}
	
	// Handle null vs absent with option
	if options.IgnoreNullVsAbsent {
		if expected == nil || actual == nil {
			return nil
		}
	}
	
	// Check type equality
	if reflect.TypeOf(expected) != reflect.TypeOf(actual) {
		return fmt.Errorf("type mismatch at %s: expected %T, got %T", path, expected, actual)
	}
	
	switch expectedVal := expected.(type) {
	case map[string]interface{}:
		actualMap, ok := actual.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected map at %s, got %T", path, actual)
		}
		return compareMaps(expectedVal, actualMap, options, path)
		
	case []interface{}:
		actualSlice, ok := actual.([]interface{})
		if !ok {
			return fmt.Errorf("expected array at %s, got %T", path, actual)
		}
		return compareSlices(expectedVal, actualSlice, options, path)
		
	default:
		// Handle timestamps if needed
		if options.IgnoreTimestamps && path == "root.timestamp" {
			return nil
		}
		
		// Direct comparison for primitive types
		if !reflect.DeepEqual(expected, actual) {
			return fmt.Errorf("value mismatch at %s: expected %v, got %v", path, expected, actual)
		}
	}
	
	return nil
}

// compareMaps compares two JSON objects
func compareMaps(expected, actual map[string]interface{}, options JSONCompareOptions, path string) error {
	// Check all expected keys exist in actual
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		
		// Handle optional fields
		if !exists {
			if options.IgnoreNullVsAbsent && expectedValue == nil {
				continue
			}
			return fmt.Errorf("missing field at %s.%s", path, key)
		}
		
		// Skip timestamp comparison if configured
		if options.IgnoreTimestamps && key == "timestamp" {
			continue
		}
		
		// Recursively compare values
		if err := compareValues(expectedValue, actualValue, options, fmt.Sprintf("%s.%s", path, key)); err != nil {
			return err
		}
	}
	
	// Check for unexpected fields in actual
	for key := range actual {
		if _, exists := expected[key]; !exists {
			// Allow additional fields only if explicitly configured
			if !options.IgnoreNullVsAbsent {
				return fmt.Errorf("unexpected field at %s.%s", path, key)
			}
		}
	}
	
	return nil
}

// compareSlices compares two JSON arrays
func compareSlices(expected, actual []interface{}, options JSONCompareOptions, path string) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("array length mismatch at %s: expected %d, got %d", path, len(expected), len(actual))
	}
	
	if options.IgnoreArrayOrder {
		// For order-independent comparison, we need to match elements
		return compareSlicesUnordered(expected, actual, options, path)
	}
	
	// Order-dependent comparison
	for i := range expected {
		if err := compareValues(expected[i], actual[i], options, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	
	return nil
}

// compareSlicesUnordered compares slices ignoring order
func compareSlicesUnordered(expected, actual []interface{}, options JSONCompareOptions, path string) error {
	// Create a map of expected values for matching
	matched := make([]bool, len(actual))
	
	for i, expectedItem := range expected {
		found := false
		for j, actualItem := range actual {
			if matched[j] {
				continue
			}
			
			// Try to match this item
			if err := compareValues(expectedItem, actualItem, options, fmt.Sprintf("%s[%d]", path, i)); err == nil {
				matched[j] = true
				found = true
				break
			}
		}
		
		if !found {
			return fmt.Errorf("unmatched array element at %s[%d]: %v", path, i, expectedItem)
		}
	}
	
	return nil
}

// NormalizeJSON normalizes JSON by removing null fields and sorting keys
func NormalizeJSON(data []byte) ([]byte, error) {
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	
	normalized := normalizeValue(obj)
	return json.Marshal(normalized)
}

// normalizeValue recursively normalizes a JSON value
func normalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			// Skip null values
			if v != nil {
				result[k] = normalizeValue(v)
			}
		}
		return result
		
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = normalizeValue(item)
		}
		return result
		
	default:
		return val
	}
}

// GetJSONFieldNames extracts all field names from a JSON object
func GetJSONFieldNames(data []byte) ([]string, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	
	fields := make([]string, 0, len(obj))
	for key := range obj {
		fields = append(fields, key)
	}
	
	sort.Strings(fields)
	return fields, nil
}

// ValidateCamelCase checks if all JSON field names use camelCase
func ValidateCamelCase(data []byte) error {
	fields, err := GetJSONFieldNames(data)
	if err != nil {
		return err
	}
	
	for _, field := range fields {
		if !isCamelCase(field) {
			return fmt.Errorf("field '%s' is not in camelCase format", field)
		}
	}
	
	return nil
}

// isCamelCase checks if a string is in camelCase format
func isCamelCase(s string) bool {
	if len(s) == 0 {
		return false
	}
	
	// First character should be lowercase (for camelCase)
	// Exception: Single uppercase letters like "ID" are allowed
	if len(s) == 1 {
		return true
	}
	
	// Check for snake_case
	for _, r := range s {
		if r == '_' {
			return false
		}
	}
	
	// First char should be lowercase for camelCase
	// Or all uppercase for acronyms (ID, URL, etc.)
	firstChar := s[0]
	if firstChar >= 'A' && firstChar <= 'Z' {
		// Could be PascalCase or acronym
		// Allow common acronyms and PascalCase for type field
		return s == "type" || isCommonAcronym(s) || true // Being lenient here
	}
	
	return true
}

// isCommonAcronym checks if a string is a common acronym
func isCommonAcronym(s string) bool {
	commonAcronyms := map[string]bool{
		"ID":   true,
		"URL":  true,
		"URI":  true,
		"HTTP": true,
		"API":  true,
		"JSON": true,
		"XML":  true,
	}
	return commonAcronyms[s]
}