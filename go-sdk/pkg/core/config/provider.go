package config

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// UnifiedConfigProvider implements ConfigProvider with unified configuration access
type UnifiedConfigProvider struct {
	config    *ValidatorConfig
	watchers  map[string][]func(key string, value interface{})
	mutex     sync.RWMutex
	keyPrefix string
}

// NewUnifiedConfigProvider creates a new unified configuration provider
func NewUnifiedConfigProvider(config *ValidatorConfig) *UnifiedConfigProvider {
	return &UnifiedConfigProvider{
		config:   config,
		watchers: make(map[string][]func(key string, value interface{})),
	}
}

// Get retrieves a configuration value by key
func (p *UnifiedConfigProvider) Get(key string) (interface{}, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	value, err := p.getNestedValue(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get config key %s: %w", key, err)
	}
	
	return value, nil
}

// GetString retrieves a string configuration value
func (p *UnifiedConfigProvider) GetString(key string) (string, error) {
	value, err := p.Get(key)
	if err != nil {
		return "", err
	}
	
	if str, ok := value.(string); ok {
		return str, nil
	}
	
	return "", fmt.Errorf("config key %s is not a string", key)
}

// GetInt retrieves an integer configuration value
func (p *UnifiedConfigProvider) GetInt(key string) (int, error) {
	value, err := p.Get(key)
	if err != nil {
		return 0, err
	}
	
	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("config key %s is not an integer", key)
	}
}

// GetDuration retrieves a duration configuration value
func (p *UnifiedConfigProvider) GetDuration(key string) (time.Duration, error) {
	value, err := p.Get(key)
	if err != nil {
		return 0, err
	}
	
	if duration, ok := value.(time.Duration); ok {
		return duration, nil
	}
	
	if str, ok := value.(string); ok {
		duration, err := time.ParseDuration(str)
		if err != nil {
			return 0, fmt.Errorf("config key %s is not a valid duration: %w", key, err)
		}
		return duration, nil
	}
	
	return 0, fmt.Errorf("config key %s is not a duration", key)
}

// GetBool retrieves a boolean configuration value
func (p *UnifiedConfigProvider) GetBool(key string) (bool, error) {
	value, err := p.Get(key)
	if err != nil {
		return false, err
	}
	
	if b, ok := value.(bool); ok {
		return b, nil
	}
	
	return false, fmt.Errorf("config key %s is not a boolean", key)
}

// GetMap retrieves a map configuration value
func (p *UnifiedConfigProvider) GetMap(key string) (map[string]interface{}, error) {
	value, err := p.Get(key)
	if err != nil {
		return nil, err
	}
	
	if m, ok := value.(map[string]interface{}); ok {
		return m, nil
	}
	
	return nil, fmt.Errorf("config key %s is not a map", key)
}

// GetSlice retrieves a slice configuration value
func (p *UnifiedConfigProvider) GetSlice(key string) ([]interface{}, error) {
	value, err := p.Get(key)
	if err != nil {
		return nil, err
	}
	
	if slice, ok := value.([]interface{}); ok {
		return slice, nil
	}
	
	// Handle string slices
	if strSlice, ok := value.([]string); ok {
		result := make([]interface{}, len(strSlice))
		for i, s := range strSlice {
			result[i] = s
		}
		return result, nil
	}
	
	return nil, fmt.Errorf("config key %s is not a slice", key)
}

// Set sets a configuration value
func (p *UnifiedConfigProvider) Set(key string, value interface{}) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	err := p.setNestedValue(key, value)
	if err != nil {
		return fmt.Errorf("failed to set config key %s: %w", key, err)
	}
	
	// Notify watchers
	p.notifyWatchers(key, value)
	
	return nil
}

// Has checks if a configuration key exists
func (p *UnifiedConfigProvider) Has(key string) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	_, err := p.getNestedValue(key)
	return err == nil
}

// GetAll returns all configuration values
func (p *UnifiedConfigProvider) GetAll() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	return p.flattenConfig()
}

// Watch watches for configuration changes
func (p *UnifiedConfigProvider) Watch(callback func(key string, value interface{})) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	// Add to global watchers
	p.watchers["*"] = append(p.watchers["*"], callback)
	return nil
}

// Validate validates the configuration
func (p *UnifiedConfigProvider) Validate() error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	// Basic validation - check required fields
	if p.config == nil {
		return fmt.Errorf("configuration is nil")
	}
	
	// Validate core configuration
	if p.config.Core != nil {
		if p.config.Core.ValidationTimeout <= 0 {
			return fmt.Errorf("core validation timeout must be positive")
		}
		if p.config.Core.MaxConcurrentValidations <= 0 {
			return fmt.Errorf("max concurrent validations must be positive")
		}
	}
	
	// Validate distributed configuration
	if p.config.Distributed != nil && p.config.Distributed.Enabled {
		if p.config.Distributed.NodeID == "" {
			return fmt.Errorf("distributed node ID is required when distributed validation is enabled")
		}
		if p.config.Distributed.ConsensusTimeout <= 0 {
			return fmt.Errorf("consensus timeout must be positive")
		}
	}
	
	return nil
}

// getNestedValue retrieves a value using dot notation
func (p *UnifiedConfigProvider) getNestedValue(key string) (interface{}, error) {
	parts := strings.Split(key, ".")
	current := reflect.ValueOf(p.config)
	
	for _, part := range parts {
		if !current.IsValid() {
			return nil, fmt.Errorf("invalid path at %s", part)
		}
		
		// Dereference pointer if needed
		if current.Kind() == reflect.Ptr {
			if current.IsNil() {
				return nil, fmt.Errorf("nil pointer at %s", part)
			}
			current = current.Elem()
		}
		
		// Handle struct fields
		if current.Kind() == reflect.Struct {
			current = current.FieldByName(p.fieldNameFromKey(part))
			if !current.IsValid() {
				return nil, fmt.Errorf("field %s not found", part)
			}
		} else {
			return nil, fmt.Errorf("cannot access field %s on non-struct", part)
		}
	}
	
	if !current.IsValid() {
		return nil, fmt.Errorf("invalid value")
	}
	
	return current.Interface(), nil
}

// setNestedValue sets a value using dot notation
func (p *UnifiedConfigProvider) setNestedValue(key string, value interface{}) error {
	parts := strings.Split(key, ".")
	current := reflect.ValueOf(p.config)
	
	// Navigate to the parent of the target field
	for i, part := range parts[:len(parts)-1] {
		if !current.IsValid() {
			return fmt.Errorf("invalid path at %s", part)
		}
		
		// Dereference pointer if needed
		if current.Kind() == reflect.Ptr {
			if current.IsNil() {
				return fmt.Errorf("nil pointer at %s", part)
			}
			current = current.Elem()
		}
		
		// Handle struct fields
		if current.Kind() == reflect.Struct {
			fieldName := p.fieldNameFromKey(part)
			field := current.FieldByName(fieldName)
			if !field.IsValid() {
				return fmt.Errorf("field %s not found", part)
			}
			
			// If the field is a pointer and nil, initialize it
			if field.Kind() == reflect.Ptr && field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			
			current = field
		} else {
			return fmt.Errorf("cannot access field %s on non-struct at step %d", part, i)
		}
	}
	
	// Set the final field
	lastPart := parts[len(parts)-1]
	if current.Kind() == reflect.Ptr {
		if current.IsNil() {
			return fmt.Errorf("nil pointer at final field %s", lastPart)
		}
		current = current.Elem()
	}
	
	if current.Kind() == reflect.Struct {
		fieldName := p.fieldNameFromKey(lastPart)
		field := current.FieldByName(fieldName)
		if !field.IsValid() {
			return fmt.Errorf("field %s not found", lastPart)
		}
		
		if !field.CanSet() {
			return fmt.Errorf("field %s is not settable", lastPart)
		}
		
		valueReflect := reflect.ValueOf(value)
		if !valueReflect.Type().AssignableTo(field.Type()) {
			return fmt.Errorf("value type %s is not assignable to field type %s", valueReflect.Type(), field.Type())
		}
		
		field.Set(valueReflect)
		return nil
	}
	
	return fmt.Errorf("cannot set field %s on non-struct", lastPart)
}

// fieldNameFromKey converts a key to a field name (e.g., "node_id" -> "NodeID", "NodeID" -> "NodeID")
func (p *UnifiedConfigProvider) fieldNameFromKey(key string) string {
	// Handle already PascalCase fields
	if key == "NodeID" {
		return "NodeID"
	}
	
	// Convert snake_case to PascalCase
	parts := strings.Split(key, "_")
	result := ""
	for _, part := range parts {
		if len(part) > 0 {
			result += strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	
	// Handle special cases
	if result == "NodeId" {
		return "NodeID"
	}
	
	return result
}

// flattenConfig flattens the configuration to a flat map
func (p *UnifiedConfigProvider) flattenConfig() map[string]interface{} {
	result := make(map[string]interface{})
	p.flattenStruct(reflect.ValueOf(p.config), "", result)
	return result
}

// flattenStruct recursively flattens a struct
func (p *UnifiedConfigProvider) flattenStruct(v reflect.Value, prefix string, result map[string]interface{}) {
	if !v.IsValid() {
		return
	}
	
	// Dereference pointer if needed
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	
	if v.Kind() != reflect.Struct {
		if prefix != "" {
			result[prefix] = v.Interface()
		}
		return
	}
	
	vType := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := vType.Field(i)
		
		// Skip unexported fields
		if !fieldType.IsExported() {
			continue
		}
		
		fieldName := p.keyFromFieldName(fieldType.Name)
		fullKey := fieldName
		if prefix != "" {
			fullKey = prefix + "." + fieldName
		}
		
		if field.Kind() == reflect.Struct || (field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct) {
			p.flattenStruct(field, fullKey, result)
		} else {
			result[fullKey] = field.Interface()
		}
	}
}

// keyFromFieldName converts a field name to a key (e.g., "NodeID" -> "node_id")
func (p *UnifiedConfigProvider) keyFromFieldName(fieldName string) string {
	result := ""
	for i, r := range fieldName {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result += "_"
		}
		result += strings.ToLower(string(r))
	}
	return result
}

// notifyWatchers notifies all watchers about a configuration change
func (p *UnifiedConfigProvider) notifyWatchers(key string, value interface{}) {
	// Notify global watchers
	for _, callback := range p.watchers["*"] {
		go callback(key, value)
	}
	
	// Notify key-specific watchers
	if watchers, exists := p.watchers[key]; exists {
		for _, callback := range watchers {
			go callback(key, value)
		}
	}
}

// UnifiedValidatorConfigProvider implements ValidatorConfigProvider
type UnifiedValidatorConfigProvider struct {
	*UnifiedConfigProvider
}

// NewUnifiedValidatorConfigProvider creates a new unified validator configuration provider
func NewUnifiedValidatorConfigProvider(config *ValidatorConfig) *UnifiedValidatorConfigProvider {
	return &UnifiedValidatorConfigProvider{
		UnifiedConfigProvider: NewUnifiedConfigProvider(config),
	}
}

// GetCoreConfig returns core validation configuration
func (p *UnifiedValidatorConfigProvider) GetCoreConfig() (*CoreValidationConfig, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if p.config.Core == nil {
		return DefaultCoreValidationConfig(), nil
	}
	
	return p.config.Core, nil
}

// GetAuthConfig returns authentication configuration
func (p *UnifiedValidatorConfigProvider) GetAuthConfig() (*AuthValidationConfig, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if p.config.Auth == nil {
		return DefaultAuthValidationConfig(), nil
	}
	
	return p.config.Auth, nil
}

// GetCacheConfig returns cache configuration
func (p *UnifiedValidatorConfigProvider) GetCacheConfig() (*CacheValidationConfig, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if p.config.Cache == nil {
		return DefaultCacheValidationConfig(), nil
	}
	
	return p.config.Cache, nil
}

// GetDistributedConfig returns distributed validation configuration
func (p *UnifiedValidatorConfigProvider) GetDistributedConfig() (*DistributedValidationConfig, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if p.config.Distributed == nil {
		return DefaultDistributedValidationConfig(), nil
	}
	
	return p.config.Distributed, nil
}

// GetAnalyticsConfig returns analytics configuration
func (p *UnifiedValidatorConfigProvider) GetAnalyticsConfig() (*AnalyticsValidationConfig, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if p.config.Analytics == nil {
		return DefaultAnalyticsValidationConfig(), nil
	}
	
	return p.config.Analytics, nil
}

// GetSecurityConfig returns security configuration
func (p *UnifiedValidatorConfigProvider) GetSecurityConfig() (*SecurityValidationConfig, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if p.config.Security == nil {
		return DefaultSecurityValidationConfig(), nil
	}
	
	return p.config.Security, nil
}

// GetFeatureFlags returns feature flags
func (p *UnifiedValidatorConfigProvider) GetFeatureFlags() (*FeatureFlags, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if p.config.Features == nil {
		return DefaultFeatureFlags(), nil
	}
	
	return p.config.Features, nil
}

// GetGlobalSettings returns global settings
func (p *UnifiedValidatorConfigProvider) GetGlobalSettings() (*GlobalSettings, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if p.config.Global == nil {
		return DefaultGlobalSettings(), nil
	}
	
	return p.config.Global, nil
}

// GetEnvironment returns the current environment
func (p *UnifiedValidatorConfigProvider) GetEnvironment() string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if p.config.Global != nil {
		return p.config.Global.Environment
	}
	
	return "development"
}

// IsEnabled checks if a feature is enabled
func (p *UnifiedValidatorConfigProvider) IsEnabled(feature string) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if p.config.Features == nil {
		return false
	}
	
	switch feature {
	case "experimental_validation":
		return p.config.Features.EnableExperimentalValidation
	case "async_validation":
		return p.config.Features.EnableAsyncValidation
	case "batch_validation":
		return p.config.Features.EnableBatchValidation
	case "stream_validation":
		return p.config.Features.EnableStreamValidation
	case "lazy_loading":
		return p.config.Features.EnableLazyLoading
	case "prefetching":
		return p.config.Features.EnablePrefetching
	case "parallelization":
		return p.config.Features.EnableParallelization
	case "compression":
		return p.config.Features.EnableCompression
	case "debug_mode":
		return p.config.Features.EnableDebugMode
	case "verbose_logging":
		return p.config.Features.EnableVerboseLogging
	case "profiling":
		return p.config.Features.EnableProfiling
	case "trace_mode":
		return p.config.Features.EnableTraceMode
	default:
		return false
	}
}