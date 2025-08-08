// Package config provides a comprehensive configuration management system
// supporting multiple sources, validation, hot-reloading, and environment-specific profiles
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

// Config represents the main configuration interface
type Config interface {
	Get(key string) interface{}
	GetString(key string) string
	GetInt(key string) int
	GetInt64(key string) int64
	GetFloat64(key string) float64
	GetBool(key string) bool
	GetDuration(key string) time.Duration
	GetSlice(key string) []interface{}
	GetStringSlice(key string) []string
	GetStringMap(key string) map[string]interface{}
	GetStringMapString(key string) map[string]string
	Set(key string, value interface{}) error
	IsSet(key string) bool
	AllKeys() []string
	AllSettings() map[string]interface{}
	Watch(key string, callback func(interface{})) error
	UnWatch(key string, callback func(interface{})) error
	Validate() error
	Clone() Config
	Merge(other Config) error
	GetProfile() string
	SetProfile(profile string) error
}

// Source represents a configuration source
type Source interface {
	Name() string
	Priority() int
	Load(ctx context.Context) (map[string]interface{}, error)
	Watch(ctx context.Context, callback func(map[string]interface{})) error
	CanWatch() bool
	LastModified() time.Time
}

// Validator represents a configuration validator
type Validator interface {
	Name() string
	Validate(config map[string]interface{}) error
	ValidateField(key string, value interface{}) error
	GetSchema() map[string]interface{}
}

// Merger handles configuration merging strategies
type Merger interface {
	Merge(base, override map[string]interface{}) map[string]interface{}
	Strategy() MergeStrategy
}

// MergeStrategy defines how configurations are merged
type MergeStrategy int

const (
	MergeStrategyOverride MergeStrategy = iota
	MergeStrategyAppend
	MergeStrategyDeepMerge
	MergeStrategyPreferBase
)

// Metadata contains configuration metadata
type Metadata struct {
	Name        string            `json:"name" yaml:"name"`
	Version     string            `json:"version" yaml:"version"`
	Environment string            `json:"environment" yaml:"environment"`
	Profile     string            `json:"profile,omitempty" yaml:"profile,omitempty"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Properties  map[string]string `json:"properties,omitempty" yaml:"properties,omitempty"`
	CreatedAt   time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" yaml:"updated_at"`
}

// ConfigImpl is the default implementation of Config
type ConfigImpl struct {
	mu           sync.RWMutex
	data         map[string]interface{}
	profile      string
	sources      []Source
	validators   []Validator
	watchers     map[string][]func(interface{})
	metadata     *Metadata
	defaults     map[string]interface{}
	keyDelimiter string
	caseMapping  bool
	envPrefix    string
}

// NewConfig creates a new configuration instance
func NewConfig() *ConfigImpl {
	return &ConfigImpl{
		data:         make(map[string]interface{}),
		watchers:     make(map[string][]func(interface{})),
		defaults:     make(map[string]interface{}),
		keyDelimiter: ".",
		caseMapping:  true,
		metadata: &Metadata{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// ConfigBuilder provides a fluent interface for building configurations
type ConfigBuilder struct {
	config     *ConfigImpl
	sources    []Source
	validators []Validator
	profile    string
	metadata   *Metadata
	options    *BuilderOptions
}

// BuilderOptions contains configuration builder options
type BuilderOptions struct {
	EnableHotReload   bool
	ValidateOnBuild   bool
	MergeStrategy     MergeStrategy
	CaseSensitive     bool
	KeyDelimiter      string
	EnvPrefix         string
	AllowEmptyValues  bool
	StrictValidation  bool
	CacheSize         int
	RefreshInterval   time.Duration
	Timeout           time.Duration
}

// NewConfigBuilder creates a new configuration builder
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config:  NewConfig(),
		options: &BuilderOptions{
			EnableHotReload:   false,
			ValidateOnBuild:   true,
			MergeStrategy:     MergeStrategyDeepMerge,
			CaseSensitive:     true,
			KeyDelimiter:      ".",
			AllowEmptyValues:  true,
			StrictValidation:  false,
			CacheSize:         1000,
			RefreshInterval:   time.Minute * 5,
			Timeout:           time.Second * 30,
		},
	}
}

// AddSource adds a configuration source to the builder
func (b *ConfigBuilder) AddSource(source Source) *ConfigBuilder {
	b.sources = append(b.sources, source)
	return b
}

// AddValidator adds a configuration validator to the builder
func (b *ConfigBuilder) AddValidator(validator Validator) *ConfigBuilder {
	b.validators = append(b.validators, validator)
	return b
}

// WithProfile sets the configuration profile
func (b *ConfigBuilder) WithProfile(profile string) *ConfigBuilder {
	b.profile = profile
	return b
}

// WithMetadata sets the configuration metadata
func (b *ConfigBuilder) WithMetadata(metadata *Metadata) *ConfigBuilder {
	b.metadata = metadata
	return b
}

// WithOptions sets the builder options
func (b *ConfigBuilder) WithOptions(options *BuilderOptions) *ConfigBuilder {
	b.options = options
	return b
}

// EnableHotReload enables hot-reloading for the configuration
func (b *ConfigBuilder) EnableHotReload() *ConfigBuilder {
	b.options.EnableHotReload = true
	return b
}

// DisableValidation disables validation during build
func (b *ConfigBuilder) DisableValidation() *ConfigBuilder {
	b.options.ValidateOnBuild = false
	return b
}

// WithMergeStrategy sets the merge strategy
func (b *ConfigBuilder) WithMergeStrategy(strategy MergeStrategy) *ConfigBuilder {
	b.options.MergeStrategy = strategy
	return b
}

// Build builds the configuration from all sources
func (b *ConfigBuilder) Build() (Config, error) {
	return b.BuildWithContext(context.Background())
}

// BuildWithContext builds the configuration with a context
func (b *ConfigBuilder) BuildWithContext(ctx context.Context) (Config, error) {
	// Apply builder options to config
	b.config.profile = b.profile
	b.config.keyDelimiter = b.options.KeyDelimiter
	b.config.caseMapping = !b.options.CaseSensitive
	b.config.envPrefix = b.options.EnvPrefix

	if b.metadata != nil {
		b.config.metadata = b.metadata
		b.config.metadata.Profile = b.profile
	}

	// Sort sources by priority
	sort.Slice(b.sources, func(i, j int) bool {
		return b.sources[i].Priority() < b.sources[j].Priority()
	})

	b.config.sources = b.sources
	b.config.validators = b.validators

	// Load configuration from all sources
	if err := b.loadFromSources(ctx); err != nil {
		return nil, fmt.Errorf("failed to load configuration from sources: %w", err)
	}

	// Validate configuration if enabled
	if b.options.ValidateOnBuild {
		if err := b.config.Validate(); err != nil {
			return nil, fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	// Start hot-reloading if enabled
	if b.options.EnableHotReload {
		if err := b.startHotReload(ctx); err != nil {
			return nil, fmt.Errorf("failed to start hot-reloading: %w", err)
		}
	}

	b.config.metadata.UpdatedAt = time.Now()
	return b.config, nil
}

// loadFromSources loads configuration from all sources
func (b *ConfigBuilder) loadFromSources(ctx context.Context) error {
	merger := NewMerger(b.options.MergeStrategy)
	
	for _, source := range b.sources {
		loadCtx, cancel := context.WithTimeout(ctx, b.options.Timeout)
		data, err := source.Load(loadCtx)
		cancel()
		
		if err != nil {
			return fmt.Errorf("failed to load from source %s: %w", source.Name(), err)
		}

		// Apply profile filtering if data contains profiles
		if b.profile != "" {
			if profiles, ok := data["profiles"]; ok {
				if profileMap, ok := profiles.(map[string]interface{}); ok {
					if profileData, ok := profileMap[b.profile]; ok {
						if profileDataMap, ok := profileData.(map[string]interface{}); ok {
							data = merger.Merge(data, profileDataMap)
						}
					}
				}
			}
		}

		// Merge with existing configuration
		b.config.mu.Lock()
		b.config.data = merger.Merge(b.config.data, data)
		b.config.mu.Unlock()
	}

	return nil
}

// startHotReload starts hot-reloading for watchable sources
func (b *ConfigBuilder) startHotReload(ctx context.Context) error {
	for _, source := range b.sources {
		if source.CanWatch() {
			go func(s Source) {
				s.Watch(ctx, func(data map[string]interface{}) {
					b.config.mu.Lock()
					defer b.config.mu.Unlock()
					
					merger := NewMerger(b.options.MergeStrategy)
					
					// Apply profile filtering
					if b.profile != "" {
						if profiles, ok := data["profiles"]; ok {
							if profileMap, ok := profiles.(map[string]interface{}); ok {
								if profileData, ok := profileMap[b.profile]; ok {
									if profileDataMap, ok := profileData.(map[string]interface{}); ok {
										data = merger.Merge(data, profileDataMap)
									}
								}
							}
						}
					}

					oldData := b.config.data
					b.config.data = merger.Merge(b.config.data, data)
					
					// Trigger watchers for changed keys
					b.triggerWatchers(oldData, b.config.data)
					
					b.config.metadata.UpdatedAt = time.Now()
				})
			}(source)
		}
	}
	return nil
}

// triggerWatchers triggers watchers for changed configuration values
func (b *ConfigBuilder) triggerWatchers(oldData, newData map[string]interface{}) {
	for key, callbacks := range b.config.watchers {
		oldVal := b.getNestedValue(oldData, key)
		newVal := b.getNestedValue(newData, key)
		
		if !reflect.DeepEqual(oldVal, newVal) {
			for _, callback := range callbacks {
				go callback(newVal)
			}
		}
	}
}

// getNestedValue retrieves a nested value from a map using dot notation
func (b *ConfigBuilder) getNestedValue(data map[string]interface{}, key string) interface{} {
	keys := b.splitKey(key)
	current := data
	
	for i, k := range keys {
		if i == len(keys)-1 {
			return current[k]
		}
		
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	
	return nil
}

// splitKey splits a configuration key by delimiter
func (b *ConfigBuilder) splitKey(key string) []string {
	if b.config.keyDelimiter == "" {
		return []string{key}
	}
	
	// Simple split for now, could be enhanced with escaping
	result := []string{}
	current := ""
	
	for _, char := range key {
		if string(char) == b.config.keyDelimiter {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	
	if current != "" {
		result = append(result, current)
	}
	
	return result
}

// Implementation of Config interface methods

// Get retrieves a configuration value
func (c *ConfigImpl) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.getNestedValue(c.data, key)
}

// GetString retrieves a string configuration value
func (c *ConfigImpl) GetString(key string) string {
	if val := c.Get(key); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", val)
	}
	return ""
}

// GetInt retrieves an int configuration value
func (c *ConfigImpl) GetInt(key string) int {
	if val := c.Get(key); val != nil {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case string:
			// Could add string parsing here
		}
	}
	return 0
}

// GetInt64 retrieves an int64 configuration value
func (c *ConfigImpl) GetInt64(key string) int64 {
	if val := c.Get(key); val != nil {
		switch v := val.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

// GetFloat64 retrieves a float64 configuration value
func (c *ConfigImpl) GetFloat64(key string) float64 {
	if val := c.Get(key); val != nil {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return 0.0
}

// GetBool retrieves a boolean configuration value
func (c *ConfigImpl) GetBool(key string) bool {
	if val := c.Get(key); val != nil {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// GetDuration retrieves a duration configuration value
func (c *ConfigImpl) GetDuration(key string) time.Duration {
	if val := c.Get(key); val != nil {
		switch v := val.(type) {
		case time.Duration:
			return v
		case string:
			if d, err := time.ParseDuration(v); err == nil {
				return d
			}
		case int64:
			return time.Duration(v)
		case int:
			return time.Duration(v)
		}
	}
	return 0
}

// GetSlice retrieves a slice configuration value
func (c *ConfigImpl) GetSlice(key string) []interface{} {
	if val := c.Get(key); val != nil {
		if slice, ok := val.([]interface{}); ok {
			return slice
		}
	}
	return nil
}

// GetStringSlice retrieves a string slice configuration value
func (c *ConfigImpl) GetStringSlice(key string) []string {
	if val := c.Get(key); val != nil {
		if slice, ok := val.([]string); ok {
			return slice
		}
		if slice, ok := val.([]interface{}); ok {
			result := make([]string, len(slice))
			for i, item := range slice {
				result[i] = fmt.Sprintf("%v", item)
			}
			return result
		}
	}
	return nil
}

// GetStringMap retrieves a string map configuration value
func (c *ConfigImpl) GetStringMap(key string) map[string]interface{} {
	if val := c.Get(key); val != nil {
		if m, ok := val.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// GetStringMapString retrieves a string-to-string map configuration value
func (c *ConfigImpl) GetStringMapString(key string) map[string]string {
	if val := c.Get(key); val != nil {
		if m, ok := val.(map[string]string); ok {
			return m
		}
		if m, ok := val.(map[string]interface{}); ok {
			result := make(map[string]string)
			for k, v := range m {
				result[k] = fmt.Sprintf("%v", v)
			}
			return result
		}
	}
	return nil
}

// Set sets a configuration value
func (c *ConfigImpl) Set(key string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if err := c.setNestedValue(c.data, key, value); err != nil {
		return err
	}
	
	// Trigger watchers
	if callbacks, ok := c.watchers[key]; ok {
		for _, callback := range callbacks {
			go callback(value)
		}
	}
	
	c.metadata.UpdatedAt = time.Now()
	return nil
}

// IsSet checks if a configuration key is set
func (c *ConfigImpl) IsSet(key string) bool {
	return c.Get(key) != nil
}

// AllKeys returns all configuration keys
func (c *ConfigImpl) AllKeys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.flattenKeys(c.data, "")
}

// AllSettings returns all configuration settings
func (c *ConfigImpl) AllSettings() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Deep copy the data
	return c.deepCopy(c.data)
}

// Watch adds a watcher for a configuration key
func (c *ConfigImpl) Watch(key string, callback func(interface{})) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.watchers[key] == nil {
		c.watchers[key] = []func(interface{}){}
	}
	c.watchers[key] = append(c.watchers[key], callback)
	
	return nil
}

// UnWatch removes a watcher for a configuration key
func (c *ConfigImpl) UnWatch(key string, callback func(interface{})) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if callbacks, ok := c.watchers[key]; ok {
		// Find and remove the callback
		for i, cb := range callbacks {
			// This is a simplification - in practice, we'd need a better way
			// to identify and remove specific callbacks
			if fmt.Sprintf("%p", cb) == fmt.Sprintf("%p", callback) {
				c.watchers[key] = append(callbacks[:i], callbacks[i+1:]...)
				break
			}
		}
		
		// Clean up empty watcher list
		if len(c.watchers[key]) == 0 {
			delete(c.watchers, key)
		}
	}
	
	return nil
}

// Validate validates the configuration using all validators
func (c *ConfigImpl) Validate() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	for _, validator := range c.validators {
		if err := validator.Validate(c.data); err != nil {
			return fmt.Errorf("validation failed for validator %s: %w", validator.Name(), err)
		}
	}
	
	return nil
}

// Clone creates a deep copy of the configuration
func (c *ConfigImpl) Clone() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	clone := NewConfig()
	clone.data = c.deepCopy(c.data)
	clone.profile = c.profile
	clone.defaults = c.deepCopy(c.defaults)
	clone.keyDelimiter = c.keyDelimiter
	clone.caseMapping = c.caseMapping
	clone.envPrefix = c.envPrefix
	
	// Clone metadata
	if c.metadata != nil {
		metadata := *c.metadata
		clone.metadata = &metadata
	}
	
	return clone
}

// Merge merges another configuration into this one
func (c *ConfigImpl) Merge(other Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	otherSettings := other.AllSettings()
	merger := NewMerger(MergeStrategyDeepMerge)
	c.data = merger.Merge(c.data, otherSettings)
	
	c.metadata.UpdatedAt = time.Now()
	return nil
}

// GetProfile returns the current profile
func (c *ConfigImpl) GetProfile() string {
	return c.profile
}

// SetProfile sets the current profile
func (c *ConfigImpl) SetProfile(profile string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.profile = profile
	if c.metadata != nil {
		c.metadata.Profile = profile
		c.metadata.UpdatedAt = time.Now()
	}
	
	return nil
}

// Helper methods

// getNestedValue retrieves a nested value from the configuration data
func (c *ConfigImpl) getNestedValue(data map[string]interface{}, key string) interface{} {
	keys := c.splitKey(key)
	current := data
	
	for i, k := range keys {
		if c.caseMapping {
			k = c.findCaseInsensitiveKey(current, k)
		}
		
		if i == len(keys)-1 {
			if val, ok := current[k]; ok {
				return val
			}
			// Check defaults
			if defaultVal, ok := c.defaults[key]; ok {
				return defaultVal
			}
			return nil
		}
		
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	
	return nil
}

// setNestedValue sets a nested value in the configuration data
func (c *ConfigImpl) setNestedValue(data map[string]interface{}, key string, value interface{}) error {
	keys := c.splitKey(key)
	current := data
	
	for i, k := range keys {
		if i == len(keys)-1 {
			current[k] = value
			return nil
		}
		
		if _, ok := current[k]; !ok {
			current[k] = make(map[string]interface{})
		}
		
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return fmt.Errorf("cannot set value at key %s: intermediate key %s is not a map", key, k)
		}
	}
	
	return nil
}

// splitKey splits a configuration key by delimiter
func (c *ConfigImpl) splitKey(key string) []string {
	if c.keyDelimiter == "" {
		return []string{key}
	}
	
	// Simple split implementation
	result := []string{}
	current := ""
	
	for _, char := range key {
		if string(char) == c.keyDelimiter {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	
	if current != "" {
		result = append(result, current)
	}
	
	return result
}

// findCaseInsensitiveKey finds a key in a case-insensitive manner
func (c *ConfigImpl) findCaseInsensitiveKey(data map[string]interface{}, key string) string {
	if !c.caseMapping {
		return key
	}
	
	lowerKey := strings.ToLower(key)
	for k := range data {
		if strings.ToLower(k) == lowerKey {
			return k
		}
	}
	
	return key
}

// flattenKeys returns all keys in flattened dot notation
func (c *ConfigImpl) flattenKeys(data map[string]interface{}, prefix string) []string {
	keys := []string{}
	
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + c.keyDelimiter + key
		}
		
		if nestedMap, ok := value.(map[string]interface{}); ok {
			keys = append(keys, c.flattenKeys(nestedMap, fullKey)...)
		} else {
			keys = append(keys, fullKey)
		}
	}
	
	sort.Strings(keys)
	return keys
}

// deepCopy creates a deep copy of a map
func (c *ConfigImpl) deepCopy(original map[string]interface{}) map[string]interface{} {
	copy := make(map[string]interface{})
	
	for key, value := range original {
		switch v := value.(type) {
		case map[string]interface{}:
			copy[key] = c.deepCopy(v)
		case []interface{}:
			newSlice := make([]interface{}, len(v))
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					newSlice[i] = c.deepCopy(itemMap)
				} else {
					newSlice[i] = item
				}
			}
			copy[key] = newSlice
		default:
			copy[key] = value
		}
	}
	
	return copy
}

// String provides a string representation of the configuration
func (c *ConfigImpl) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	data, _ := json.MarshalIndent(c.data, "", "  ")
	return string(data)
}