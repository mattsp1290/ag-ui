package config

import (
	"reflect"
)

// MergerImpl implements the Merger interface
type MergerImpl struct {
	strategy MergeStrategy
}

// NewMerger creates a new configuration merger
func NewMerger(strategy MergeStrategy) Merger {
	return &MergerImpl{strategy: strategy}
}

// Merge merges two configuration maps according to the strategy
func (m *MergerImpl) Merge(base, override map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = make(map[string]interface{})
	}
	if override == nil {
		return base
	}

	switch m.strategy {
	case MergeStrategyOverride:
		return m.mergeOverride(base, override)
	case MergeStrategyAppend:
		return m.mergeAppend(base, override)
	case MergeStrategyDeepMerge:
		return m.mergeDeep(base, override)
	case MergeStrategyPreferBase:
		return m.mergePreferBase(base, override)
	default:
		return m.mergeDeep(base, override)
	}
}

// Strategy returns the current merge strategy
func (m *MergerImpl) Strategy() MergeStrategy {
	return m.strategy
}

// mergeOverride completely replaces base with override
func (m *MergerImpl) mergeOverride(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Copy base first
	for k, v := range base {
		result[k] = v
	}
	
	// Override with new values
	for k, v := range override {
		result[k] = v
	}
	
	return result
}

// mergeAppend appends values where possible, otherwise overrides
func (m *MergerImpl) mergeAppend(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Copy base first
	for k, v := range base {
		result[k] = v
	}
	
	// Merge override values
	for k, v := range override {
		if existing, exists := result[k]; exists {
			// Try to append if both are slices
			if existingSlice, ok := existing.([]interface{}); ok {
				if overrideSlice, ok := v.([]interface{}); ok {
					result[k] = append(existingSlice, overrideSlice...)
					continue
				}
			}
			// Try to append if both are string slices
			if existingSlice, ok := existing.([]string); ok {
				if overrideSlice, ok := v.([]string); ok {
					result[k] = append(existingSlice, overrideSlice...)
					continue
				}
			}
		}
		
		// Default to override behavior
		result[k] = v
	}
	
	return result
}

// mergeDeep performs deep merging of nested maps and intelligent merging of slices
func (m *MergerImpl) mergeDeep(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Copy all keys from base
	for k, v := range base {
		result[k] = m.deepCopyValue(v)
	}
	
	// Merge override values
	for k, v := range override {
		if existing, exists := result[k]; exists {
			result[k] = m.mergeValues(existing, v)
		} else {
			result[k] = m.deepCopyValue(v)
		}
	}
	
	return result
}

// mergePreferBase only adds keys from override that don't exist in base
func (m *MergerImpl) mergePreferBase(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Copy base first (base takes precedence)
	for k, v := range base {
		result[k] = m.deepCopyValue(v)
	}
	
	// Only add keys from override that don't exist in base
	for k, v := range override {
		if _, exists := result[k]; !exists {
			result[k] = m.deepCopyValue(v)
		}
	}
	
	return result
}

// mergeValues merges two individual values intelligently
func (m *MergerImpl) mergeValues(existing, new interface{}) interface{} {
	// If both are maps, merge them recursively
	if existingMap, ok := existing.(map[string]interface{}); ok {
		if newMap, ok := new.(map[string]interface{}); ok {
			return m.mergeDeep(existingMap, newMap)
		}
	}
	
	// If both are slices, decide how to merge based on content
	if existingSlice, ok := existing.([]interface{}); ok {
		if newSlice, ok := new.([]interface{}); ok {
			return m.mergeSlices(existingSlice, newSlice)
		}
	}
	
	// If both are string slices
	if existingSlice, ok := existing.([]string); ok {
		if newSlice, ok := new.([]string); ok {
			return append(existingSlice, newSlice...)
		}
	}
	
	// Default: new value replaces existing
	return m.deepCopyValue(new)
}

// mergeSlices merges two slices intelligently
func (m *MergerImpl) mergeSlices(existing, new []interface{}) []interface{} {
	// For slices of primitives, append
	if m.isPrimitiveSlice(existing) && m.isPrimitiveSlice(new) {
		return append(existing, new...)
	}
	
	// For slices of maps, try to merge by key if they have an "id" or "name" field
	if m.isMapSlice(existing) && m.isMapSlice(new) {
		return m.mergeMapSlices(existing, new)
	}
	
	// Default: append
	return append(existing, new...)
}

// isPrimitiveSlice checks if a slice contains only primitive types
func (m *MergerImpl) isPrimitiveSlice(slice []interface{}) bool {
	for _, item := range slice {
		switch item.(type) {
		case string, int, int64, float64, bool:
			continue
		default:
			return false
		}
	}
	return true
}

// isMapSlice checks if a slice contains only maps
func (m *MergerImpl) isMapSlice(slice []interface{}) bool {
	for _, item := range slice {
		if _, ok := item.(map[string]interface{}); !ok {
			return false
		}
	}
	return true
}

// mergeMapSlices merges two slices of maps by matching keys
func (m *MergerImpl) mergeMapSlices(existing, new []interface{}) []interface{} {
	result := make([]interface{}, 0, len(existing)+len(new))
	
	// Copy existing items
	existingByKey := make(map[string]map[string]interface{})
	for _, item := range existing {
		if itemMap, ok := item.(map[string]interface{}); ok {
			key := m.getMapKey(itemMap)
			if key != "" {
				existingByKey[key] = itemMap
				result = append(result, m.deepCopyValue(itemMap))
			} else {
				result = append(result, m.deepCopyValue(itemMap))
			}
		}
	}
	
	// Merge or add new items
	for _, item := range new {
		if itemMap, ok := item.(map[string]interface{}); ok {
			key := m.getMapKey(itemMap)
			if key != "" && existingByKey[key] != nil {
				// Merge with existing item
				for i, existingItem := range result {
					if existingMap, ok := existingItem.(map[string]interface{}); ok {
						if m.getMapKey(existingMap) == key {
							result[i] = m.mergeDeep(existingMap, itemMap)
							break
						}
					}
				}
			} else {
				// Add new item
				result = append(result, m.deepCopyValue(itemMap))
			}
		}
	}
	
	return result
}

// getMapKey extracts a key from a map (tries "id", "name", then first string key)
func (m *MergerImpl) getMapKey(itemMap map[string]interface{}) string {
	// Try common key fields
	for _, keyField := range []string{"id", "name", "key"} {
		if val, ok := itemMap[keyField]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}
	
	// Fall back to first string value
	for _, val := range itemMap {
		if str, ok := val.(string); ok {
			return str
		}
	}
	
	return ""
}

// deepCopyValue creates a deep copy of a value
func (m *MergerImpl) deepCopyValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	
	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.Map:
		if val.Type().Key().Kind() == reflect.String {
			original := value.(map[string]interface{})
			copy := make(map[string]interface{})
			for k, v := range original {
				copy[k] = m.deepCopyValue(v)
			}
			return copy
		}
	case reflect.Slice:
		original, ok := value.([]interface{})
		if ok {
			copy := make([]interface{}, len(original))
			for i, v := range original {
				copy[i] = m.deepCopyValue(v)
			}
			return copy
		}
		// Handle string slices
		if stringSlice, ok := value.([]string); ok {
			copy := make([]string, len(stringSlice))
			copy = append(copy, stringSlice...)
			return copy
		}
	}
	
	// For primitive types, return as-is
	return value
}