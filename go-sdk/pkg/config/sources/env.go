package sources

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// EnvSource loads configuration from environment variables
type EnvSource struct {
	prefix      string
	separator   string
	keyMapping  map[string]string
	transformer func(string) string
	priority    int
}

// EnvSourceOptions configures environment source behavior
type EnvSourceOptions struct {
	Prefix      string
	Separator   string
	KeyMapping  map[string]string
	Transformer func(string) string
	Priority    int
}

// NewEnvSource creates a new environment variable source
func NewEnvSource(prefix string) *EnvSource {
	return NewEnvSourceWithOptions(&EnvSourceOptions{
		Prefix:    prefix,
		Separator: "_",
		Priority:  10,
	})
}

// NewEnvSourceWithOptions creates a new environment variable source with options
func NewEnvSourceWithOptions(options *EnvSourceOptions) *EnvSource {
	if options == nil {
		options = &EnvSourceOptions{}
	}
	
	return &EnvSource{
		prefix:      options.Prefix,
		separator:   options.Separator,
		keyMapping:  options.KeyMapping,
		transformer: options.Transformer,
		priority:    options.Priority,
	}
}

// Name returns the source name
func (e *EnvSource) Name() string {
	if e.prefix != "" {
		return fmt.Sprintf("env:%s", e.prefix)
	}
	return "env"
}

// Priority returns the source priority
func (e *EnvSource) Priority() int {
	return e.priority
}

// Load loads configuration from environment variables
func (e *EnvSource) Load(ctx context.Context) (map[string]interface{}, error) {
	config := make(map[string]interface{})
	
	// Get all environment variables
	env := os.Environ()
	
	for _, pair := range env {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key, value := parts[0], parts[1]
		
		// Filter by prefix if specified
		if e.prefix != "" && !strings.HasPrefix(key, e.prefix+e.separator) {
			continue
		}
		
		// Remove prefix
		configKey := key
		if e.prefix != "" {
			configKey = strings.TrimPrefix(key, e.prefix+e.separator)
		}
		
		// Apply key mapping
		if e.keyMapping != nil {
			if mapped, ok := e.keyMapping[configKey]; ok {
				configKey = mapped
			}
		}
		
		// Apply transformer
		if e.transformer != nil {
			configKey = e.transformer(configKey)
		} else {
			// Default transformation: lowercase with dots
			configKey = strings.ToLower(configKey)
			configKey = strings.ReplaceAll(configKey, e.separator, ".")
		}
		
		// Parse value
		parsedValue := e.parseValue(value)
		
		// Set nested value
		if err := e.setNestedValue(config, configKey, parsedValue); err != nil {
			continue // Skip invalid keys
		}
	}
	
	return config, nil
}

// Watch starts watching for environment variable changes
// Note: Environment variables don't typically change during runtime,
// so this implementation returns immediately
func (e *EnvSource) Watch(ctx context.Context, callback func(map[string]interface{})) error {
	// Environment variables don't change during runtime in most cases
	// This could be extended to poll for changes if needed
	return nil
}

// CanWatch returns whether this source supports watching
func (e *EnvSource) CanWatch() bool {
	return false
}

// LastModified returns when the source was last modified
func (e *EnvSource) LastModified() time.Time {
	// Environment variables don't have modification times
	return time.Now()
}

// parseValue attempts to parse a string value into appropriate type
func (e *EnvSource) parseValue(value string) interface{} {
	// Handle empty values
	if value == "" {
		return ""
	}
	
	// Try boolean
	if lower := strings.ToLower(value); lower == "true" || lower == "false" {
		return lower == "true"
	}
	
	// Try integer
	if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
		// Return int if it fits in int32 range, otherwise int64
		if intVal >= -2147483648 && intVal <= 2147483647 {
			return int(intVal)
		}
		return intVal
	}
	
	// Try float
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal
	}
	
	// Try duration
	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}
	
	// Handle comma-separated lists
	if strings.Contains(value, ",") {
		parts := strings.Split(value, ",")
		slice := make([]string, len(parts))
		for i, part := range parts {
			slice[i] = strings.TrimSpace(part)
		}
		return slice
	}
	
	// Return as string
	return value
}

// setNestedValue sets a value in a nested map using dot notation
func (e *EnvSource) setNestedValue(config map[string]interface{}, key string, value interface{}) error {
	keys := strings.Split(key, ".")
	current := config
	
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
			return fmt.Errorf("key conflict at %s", k)
		}
	}
	
	return nil
}