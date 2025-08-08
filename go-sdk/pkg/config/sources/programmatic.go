package sources

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ProgrammaticSource allows setting configuration values programmatically
type ProgrammaticSource struct {
	mu       sync.RWMutex
	data     map[string]interface{}
	priority int
	name     string
	modTime  time.Time
}

// ProgrammaticSourceOptions configures programmatic source behavior
type ProgrammaticSourceOptions struct {
	Name     string
	Priority int
	InitialData map[string]interface{}
}

// NewProgrammaticSource creates a new programmatic configuration source
func NewProgrammaticSource() *ProgrammaticSource {
	return NewProgrammaticSourceWithOptions(&ProgrammaticSourceOptions{
		Name:     "programmatic",
		Priority: 40,
	})
}

// NewProgrammaticSourceWithOptions creates a new programmatic source with options
func NewProgrammaticSourceWithOptions(options *ProgrammaticSourceOptions) *ProgrammaticSource {
	if options == nil {
		options = &ProgrammaticSourceOptions{}
	}
	
	if options.Name == "" {
		options.Name = "programmatic"
	}
	
	data := make(map[string]interface{})
	if options.InitialData != nil {
		for k, v := range options.InitialData {
			data[k] = v
		}
	}
	
	return &ProgrammaticSource{
		data:     data,
		priority: options.Priority,
		name:     options.Name,
		modTime:  time.Now(),
	}
}

// Name returns the source name
func (p *ProgrammaticSource) Name() string {
	return fmt.Sprintf("programmatic:%s", p.name)
}

// Priority returns the source priority
func (p *ProgrammaticSource) Priority() int {
	return p.priority
}

// Load loads configuration from the programmatic data
func (p *ProgrammaticSource) Load(ctx context.Context) (map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	// Deep copy the data to prevent external modifications
	result := p.deepCopy(p.data)
	return result, nil
}

// Watch starts watching for programmatic changes
func (p *ProgrammaticSource) Watch(ctx context.Context, callback func(map[string]interface{})) error {
	// For programmatic sources, we could implement a notification system
	// For now, return nil as changes are made via Set methods
	return nil
}

// CanWatch returns whether this source supports watching
func (p *ProgrammaticSource) CanWatch() bool {
	return false // Could be true if we implement a notification system
}

// LastModified returns when the source was last modified
func (p *ProgrammaticSource) LastModified() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.modTime
}

// Set sets a configuration value programmatically
func (p *ProgrammaticSource) Set(key string, value interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if err := p.setNestedValue(p.data, key, value); err != nil {
		return err
	}
	
	p.modTime = time.Now()
	return nil
}

// Get gets a configuration value
func (p *ProgrammaticSource) Get(key string) interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return p.getNestedValue(p.data, key)
}

// Delete removes a configuration value
func (p *ProgrammaticSource) Delete(key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if err := p.deleteNestedValue(p.data, key); err != nil {
		return err
	}
	
	p.modTime = time.Now()
	return nil
}

// Clear clears all configuration data
func (p *ProgrammaticSource) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.data = make(map[string]interface{})
	p.modTime = time.Now()
}

// SetAll replaces all configuration data
func (p *ProgrammaticSource) SetAll(data map[string]interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.data = p.deepCopy(data)
	p.modTime = time.Now()
}

// GetAll returns all configuration data
func (p *ProgrammaticSource) GetAll() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return p.deepCopy(p.data)
}

// setNestedValue sets a value in a nested map using dot notation
func (p *ProgrammaticSource) setNestedValue(data map[string]interface{}, key string, value interface{}) error {
	keys := p.splitKey(key)
	current := data
	
	for i, k := range keys {
		if i == len(keys)-1 {
			// Last key, set the value
			current[k] = value
			return nil
		}
		
		// Intermediate key, ensure it's a map
		if _, ok := current[k]; !ok {
			current[k] = make(map[string]interface{})
		}
		
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			// Can't traverse further, key conflicts with existing value
			return fmt.Errorf("key conflict at %s: existing value is not a map", k)
		}
	}
	
	return nil
}

// getNestedValue gets a value from a nested map using dot notation
func (p *ProgrammaticSource) getNestedValue(data map[string]interface{}, key string) interface{} {
	keys := p.splitKey(key)
	current := data
	
	for i, k := range keys {
		if i == len(keys)-1 {
			// Last key, return the value
			return current[k]
		}
		
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			// Can't traverse further
			return nil
		}
	}
	
	return nil
}

// deleteNestedValue deletes a value from a nested map using dot notation
func (p *ProgrammaticSource) deleteNestedValue(data map[string]interface{}, key string) error {
	keys := p.splitKey(key)
	if len(keys) == 0 {
		return fmt.Errorf("empty key")
	}
	
	if len(keys) == 1 {
		// Simple key, delete directly
		delete(data, keys[0])
		return nil
	}
	
	// Navigate to parent
	current := data
	for i := 0; i < len(keys)-1; i++ {
		k := keys[i]
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			// Key doesn't exist or is not a map
			return nil
		}
	}
	
	// Delete the final key
	delete(current, keys[len(keys)-1])
	return nil
}

// splitKey splits a configuration key by dots
func (p *ProgrammaticSource) splitKey(key string) []string {
	if key == "" {
		return []string{}
	}
	
	// Simple split by dots
	parts := []string{}
	current := ""
	
	for _, char := range key {
		if char == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	
	if current != "" {
		parts = append(parts, current)
	}
	
	return parts
}

// deepCopy creates a deep copy of a map using optimized type-specific copying
func (p *ProgrammaticSource) deepCopy(original map[string]interface{}) map[string]interface{} {
	if original == nil {
		return nil
	}
	
	// Use pre-allocated capacity for better performance
	result := make(map[string]interface{}, len(original))
	
	for key, value := range original {
		result[key] = p.deepCopyValue(value)
	}
	
	return result
}

// deepCopyValue creates an optimized deep copy of individual values
func (p *ProgrammaticSource) deepCopyValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	
	// Type-specific optimizations without reflection
	switch v := value.(type) {
	case map[string]interface{}:
		return p.deepCopy(v)
	case []interface{}:
		newSlice := make([]interface{}, len(v))
		for i, item := range v {
			newSlice[i] = p.deepCopyValue(item)
		}
		return newSlice
	case []string:
		newSlice := make([]string, len(v))
		copy(newSlice, v)
		return newSlice
	case []int:
		newSlice := make([]int, len(v))
		copy(newSlice, v)
		return newSlice
	case []int64:
		newSlice := make([]int64, len(v))
		copy(newSlice, v)
		return newSlice
	case []float64:
		newSlice := make([]float64, len(v))
		copy(newSlice, v)
		return newSlice
	case []bool:
		newSlice := make([]bool, len(v))
		copy(newSlice, v)
		return newSlice
	case map[string]string:
		newMap := make(map[string]string, len(v))
		for k, val := range v {
			newMap[k] = val
		}
		return newMap
	case map[string]int:
		newMap := make(map[string]int, len(v))
		for k, val := range v {
			newMap[k] = val
		}
		return newMap
	case string, int, int8, int16, int32, int64,
		 uint, uint8, uint16, uint32, uint64,
		 float32, float64, bool, complex64, complex128:
		// Immutable types - return as-is
		return value
	default:
		// For unknown types, return as-is (assume immutable)
		return value
	}
}