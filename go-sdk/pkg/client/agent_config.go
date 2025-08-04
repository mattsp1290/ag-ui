package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"gopkg.in/yaml.v3"
)

// AgentConfigManager provides comprehensive agent configuration management with
// agent-specific configuration schemas, dynamic configuration updates, validation
// with defaults, environment-based configuration loading, and configuration
// inheritance and overrides.
//
// Key features:
//   - Type-safe configuration structures
//   - Validation with clear error messages
//   - Hot-reloading capabilities
//   - Integration with global configuration system
//   - Environment variable support
//   - Configuration inheritance and composition
//   - Dynamic configuration updates
//   - Schema validation and defaults
type AgentConfigManager struct {
	// Configuration sources
	configSources []ConfigSource
	sourcesMu     sync.RWMutex
	
	// Configuration cache
	configCache   map[string]*CachedConfig
	cacheMu       sync.RWMutex
	
	// Watchers for hot-reloading
	watchers      map[string]*ConfigWatcher
	watchersMu    sync.RWMutex
	
	// Validation schemas
	schemas       map[string]*ConfigSchema
	schemasMu     sync.RWMutex
	
	// Change listeners
	listeners     map[string][]ConfigChangeListener
	listenersMu   sync.RWMutex
	
	// Lifecycle
	running       bool
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	
	// Metrics
	metrics       ConfigMetrics
	metricsMu     sync.RWMutex
}

// ConfigSource represents a source of configuration data.
type ConfigSource interface {
	// Load loads configuration data
	Load(ctx context.Context) (map[string]interface{}, error)
	
	// Watch watches for configuration changes
	Watch(ctx context.Context, callback func(map[string]interface{})) error
	
	// Name returns the name of the configuration source
	Name() string
	
	// Priority returns the priority of this source (higher = more important)
	Priority() int
}

// CachedConfig represents cached configuration data.
type CachedConfig struct {
	Data      map[string]interface{} `json:"data"`
	Source    string                 `json:"source"`
	LoadTime  time.Time              `json:"load_time"`
	ExpiresAt time.Time              `json:"expires_at"`
	Version   string                 `json:"version"`
	Checksum  string                 `json:"checksum"`
}

// ConfigWatcher watches configuration changes and triggers reloads.
type ConfigWatcher struct {
	source     ConfigSource
	callback   func(map[string]interface{})
	ctx        context.Context
	cancel     context.CancelFunc
	isActive   bool
}

// ConfigSchema defines validation rules for configuration.
type ConfigSchema struct {
	Name        string                    `json:"name" yaml:"name"`
	Version     string                    `json:"version" yaml:"version"`
	Properties  map[string]*PropertySpec  `json:"properties" yaml:"properties"`
	Required    []string                  `json:"required" yaml:"required"`
	Defaults    map[string]interface{}    `json:"defaults" yaml:"defaults"`
	Validators  []ConfigValidator         `json:"-" yaml:"-"`
}

// PropertySpec defines a configuration property specification.
type PropertySpec struct {
	Type        string                 `json:"type" yaml:"type"`
	Description string                 `json:"description" yaml:"description"`
	Default     interface{}            `json:"default" yaml:"default"`
	Required    bool                   `json:"required" yaml:"required"`
	Constraints *PropertyConstraints   `json:"constraints" yaml:"constraints"`
	EnvVar      string                 `json:"env_var" yaml:"env_var"`
	Sensitive   bool                   `json:"sensitive" yaml:"sensitive"`
}

// PropertyConstraints defines constraints for a configuration property.
type PropertyConstraints struct {
	Min         *float64               `json:"min" yaml:"min"`
	Max         *float64               `json:"max" yaml:"max"`
	MinLength   *int                   `json:"min_length" yaml:"min_length"`
	MaxLength   *int                   `json:"max_length" yaml:"max_length"`
	Pattern     string                 `json:"pattern" yaml:"pattern"`
	Enum        []interface{}          `json:"enum" yaml:"enum"`
	Format      string                 `json:"format" yaml:"format"`
}

// ConfigValidator is a function that validates configuration.
type ConfigValidator func(config map[string]interface{}) error

// ConfigChangeListener is called when configuration changes.
type ConfigChangeListener func(ConfigChangeEvent)

// ConfigChangeEvent represents a configuration change event.
type ConfigChangeEvent struct {
	Source    string                 `json:"source"`
	Path      string                 `json:"path,omitempty"`
	OldValue  interface{}            `json:"old_value"`
	NewValue  interface{}            `json:"new_value"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ConfigMetrics contains metrics for configuration management.
type ConfigMetrics struct {
	ConfigLoads       int64         `json:"config_loads"`
	ConfigReloads     int64         `json:"config_reloads"`
	ValidationErrors  int64         `json:"validation_errors"`
	SourceErrors      int64         `json:"source_errors"`
	WatcherErrors     int64         `json:"watcher_errors"`
	AverageLoadTime   time.Duration `json:"average_load_time"`
	LastLoadTime      time.Time     `json:"last_load_time"`
	ActiveWatchers    int32         `json:"active_watchers"`
}

// File-based configuration source
type FileConfigSource struct {
	filePath string
	format   ConfigFormat
	priority int
}

// Environment variable configuration source
type EnvConfigSource struct {
	prefix   string
	priority int
}

// Remote configuration source (placeholder)
type RemoteConfigSource struct {
	url      string
	priority int
	headers  map[string]string
}

// ConfigFormat represents configuration file formats.
type ConfigFormat string

const (
	ConfigFormatJSON ConfigFormat = "json"
	ConfigFormatYAML ConfigFormat = "yaml"
	ConfigFormatTOML ConfigFormat = "toml"
)

// NewAgentConfigManager creates a new agent configuration manager.
func NewAgentConfigManager() *AgentConfigManager {
	return &AgentConfigManager{
		configSources: make([]ConfigSource, 0),
		configCache:   make(map[string]*CachedConfig),
		watchers:      make(map[string]*ConfigWatcher),
		schemas:       make(map[string]*ConfigSchema),
		listeners:     make(map[string][]ConfigChangeListener),
		metrics: ConfigMetrics{
			LastLoadTime: time.Now(),
		},
	}
}

// Start begins configuration management.
func (acm *AgentConfigManager) Start(ctx context.Context) error {
	if acm.running {
		return errors.NewAgentError(errors.ErrorTypeInvalidState, "agent config manager is already running", "config_manager")
	}
	
	acm.ctx, acm.cancel = context.WithCancel(ctx)
	acm.running = true
	
	// Start watchers for all sources
	for _, source := range acm.configSources {
		if err := acm.startWatcher(source); err != nil {
			return fmt.Errorf("failed to start watcher for source %s: %w", source.Name(), err)
		}
	}
	
	// Start metrics collection
	acm.wg.Add(1)
	go acm.metricsLoop()
	
	return nil
}

// Stop gracefully stops configuration management.
func (acm *AgentConfigManager) Stop(ctx context.Context) error {
	if !acm.running {
		return nil
	}
	
	acm.running = false
	acm.cancel()
	
	// Stop all watchers
	acm.watchersMu.Lock()
	for _, watcher := range acm.watchers {
		watcher.cancel()
	}
	acm.watchers = make(map[string]*ConfigWatcher)
	acm.watchersMu.Unlock()
	
	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		acm.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All goroutines finished
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for config manager to stop")
	}
	
	return nil
}

// AddSource adds a configuration source.
func (acm *AgentConfigManager) AddSource(source ConfigSource) {
	acm.sourcesMu.Lock()
	defer acm.sourcesMu.Unlock()
	
	acm.configSources = append(acm.configSources, source)
	
	// Sort sources by priority (highest first)
	acm.sortSources()
	
	// Start watcher if manager is running
	if acm.running {
		acm.startWatcher(source)
	}
}

// LoadConfig loads configuration for the specified agent.
func (acm *AgentConfigManager) LoadConfig(ctx context.Context, agentName string) (*AgentConfig, error) {
	startTime := time.Now()
	defer func() {
		loadTime := time.Since(startTime)
		acm.updateLoadTimeMetrics(loadTime)
		acm.metricsMu.Lock()
		acm.metrics.ConfigLoads++
		acm.metrics.LastLoadTime = time.Now()
		acm.metricsMu.Unlock()
	}()
	
	// Check cache first
	if cached := acm.getCachedConfig(agentName); cached != nil && !acm.isExpired(cached) {
		return acm.unmarshalConfig(cached.Data)
	}
	
	// Load from sources
	mergedConfig := make(map[string]interface{})
	
	acm.sourcesMu.RLock()
	sources := make([]ConfigSource, len(acm.configSources))
	copy(sources, acm.configSources)
	acm.sourcesMu.RUnlock()
	
	// Load from all sources in priority order
	for _, source := range sources {
		sourceConfig, err := source.Load(ctx)
		if err != nil {
			acm.metricsMu.Lock()
			acm.metrics.SourceErrors++
			acm.metricsMu.Unlock()
			continue // Skip sources with errors
		}
		
		// Merge configuration
		acm.mergeConfig(mergedConfig, sourceConfig)
	}
	
	// Apply environment variable overrides
	acm.applyEnvOverrides(mergedConfig, agentName)
	
	// Validate configuration
	if err := acm.validateConfig(agentName, mergedConfig); err != nil {
		acm.metricsMu.Lock()
		acm.metrics.ValidationErrors++
		acm.metricsMu.Unlock()
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	
	// Apply defaults
	acm.applyDefaults(agentName, mergedConfig)
	
	// Cache the configuration
	acm.cacheConfig(agentName, mergedConfig)
	
	// Convert to AgentConfig
	return acm.unmarshalConfig(mergedConfig)
}

// ReloadConfig reloads configuration for the specified agent.
func (acm *AgentConfigManager) ReloadConfig(ctx context.Context, agentName string) (*AgentConfig, error) {
	// Clear cache
	acm.cacheMu.Lock()
	delete(acm.configCache, agentName)
	acm.cacheMu.Unlock()
	
	// Load fresh configuration
	config, err := acm.LoadConfig(ctx, agentName)
	if err == nil {
		acm.metricsMu.Lock()
		acm.metrics.ConfigReloads++
		acm.metricsMu.Unlock()
		
		// Notify listeners
		acm.notifyConfigChange(ConfigChangeEvent{
			Source:    "reload",
			Timestamp: time.Now(),
			Version:   fmt.Sprintf("reload_%d", time.Now().Unix()),
		})
	}
	
	return config, err
}

// RegisterSchema registers a configuration schema for validation.
func (acm *AgentConfigManager) RegisterSchema(schema *ConfigSchema) {
	acm.schemasMu.Lock()
	defer acm.schemasMu.Unlock()
	
	acm.schemas[schema.Name] = schema
}

// AddConfigChangeListener adds a listener for configuration changes.
func (acm *AgentConfigManager) AddConfigChangeListener(agentName string, listener ConfigChangeListener) {
	acm.listenersMu.Lock()
	defer acm.listenersMu.Unlock()
	
	if acm.listeners[agentName] == nil {
		acm.listeners[agentName] = make([]ConfigChangeListener, 0)
	}
	
	acm.listeners[agentName] = append(acm.listeners[agentName], listener)
}

// UpdateConfigValue updates a specific configuration value dynamically.
func (acm *AgentConfigManager) UpdateConfigValue(ctx context.Context, agentName, path string, value interface{}) error {
	// Get current config
	currentConfig, err := acm.LoadConfig(ctx, agentName)
	if err != nil {
		return err
	}
	
	// Convert to map for manipulation
	configMap := acm.configToMap(currentConfig)
	
	// Store old value for change event
	oldValue := acm.getValueByPath(configMap, path)
	
	// Update the value
	if err := acm.setValueByPath(configMap, path, value); err != nil {
		return fmt.Errorf("failed to update config value: %w", err)
	}
	
	// Validate the updated configuration
	if err := acm.validateConfig(agentName, configMap); err != nil {
		return fmt.Errorf("configuration validation failed after update: %w", err)
	}
	
	// Update cache
	acm.cacheConfig(agentName, configMap)
	
	// Notify listeners
	acm.notifyConfigChange(ConfigChangeEvent{
		Source:    "dynamic_update",
		Path:      path,
		OldValue:  oldValue,
		NewValue:  value,
		Timestamp: time.Now(),
		Version:   fmt.Sprintf("update_%d", time.Now().Unix()),
	})
	
	return nil
}

// GetMetrics returns current configuration metrics.
func (acm *AgentConfigManager) GetMetrics() ConfigMetrics {
	acm.metricsMu.Lock()
	defer acm.metricsMu.Unlock()
	
	metrics := acm.metrics
	
	// Count active watchers
	acm.watchersMu.RLock()
	activeWatchers := int32(0)
	for _, watcher := range acm.watchers {
		if watcher.isActive {
			activeWatchers++
		}
	}
	acm.watchersMu.RUnlock()
	
	metrics.ActiveWatchers = activeWatchers
	
	return metrics
}

// Private methods

func (acm *AgentConfigManager) sortSources() {
	// Sort by priority (highest first)
	for i := 0; i < len(acm.configSources)-1; i++ {
		for j := i + 1; j < len(acm.configSources); j++ {
			if acm.configSources[i].Priority() < acm.configSources[j].Priority() {
				acm.configSources[i], acm.configSources[j] = acm.configSources[j], acm.configSources[i]
			}
		}
	}
}

func (acm *AgentConfigManager) startWatcher(source ConfigSource) error {
	watcherCtx, cancel := context.WithCancel(acm.ctx)
	
	callback := func(config map[string]interface{}) {
		// Handle configuration change
		acm.handleConfigChange(source.Name(), config)
	}
	
	watcher := &ConfigWatcher{
		source:   source,
		callback: callback,
		ctx:      watcherCtx,
		cancel:   cancel,
		isActive: true,
	}
	
	acm.watchersMu.Lock()
	acm.watchers[source.Name()] = watcher
	acm.watchersMu.Unlock()
	
	// Start watching in goroutine
	acm.wg.Add(1)
	go func() {
		defer acm.wg.Done()
		if err := source.Watch(watcherCtx, callback); err != nil {
			acm.metricsMu.Lock()
			acm.metrics.WatcherErrors++
			acm.metricsMu.Unlock()
		}
		
		acm.watchersMu.Lock()
		watcher.isActive = false
		acm.watchersMu.Unlock()
	}()
	
	return nil
}

func (acm *AgentConfigManager) handleConfigChange(sourceName string, config map[string]interface{}) {
	// Clear relevant cache entries
	acm.cacheMu.Lock()
	for key := range acm.configCache {
		delete(acm.configCache, key)
	}
	acm.cacheMu.Unlock()
	
	// Notify listeners
	acm.notifyConfigChange(ConfigChangeEvent{
		Source:    sourceName,
		Timestamp: time.Now(),
		Version:   fmt.Sprintf("watch_%d", time.Now().Unix()),
	})
}

func (acm *AgentConfigManager) getCachedConfig(agentName string) *CachedConfig {
	acm.cacheMu.RLock()
	defer acm.cacheMu.RUnlock()
	
	return acm.configCache[agentName]
}

func (acm *AgentConfigManager) isExpired(cached *CachedConfig) bool {
	return time.Now().After(cached.ExpiresAt)
}

func (acm *AgentConfigManager) cacheConfig(agentName string, config map[string]interface{}) {
	acm.cacheMu.Lock()
	defer acm.cacheMu.Unlock()
	
	checksum := acm.calculateChecksum(config)
	
	cached := &CachedConfig{
		Data:      config,
		Source:    "merged",
		LoadTime:  time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute), // 5-minute cache
		Version:   fmt.Sprintf("v_%d", time.Now().Unix()),
		Checksum:  checksum,
	}
	
	acm.configCache[agentName] = cached
}

func (acm *AgentConfigManager) mergeConfig(target, source map[string]interface{}) {
	for key, value := range source {
		if existingValue, exists := target[key]; exists {
			// If both are maps, merge recursively
			if existingMap, ok := existingValue.(map[string]interface{}); ok {
				if sourceMap, ok := value.(map[string]interface{}); ok {
					acm.mergeConfig(existingMap, sourceMap)
					continue
				}
			}
		}
		// Override existing value or add new key
		target[key] = value
	}
}

func (acm *AgentConfigManager) applyEnvOverrides(config map[string]interface{}, agentName string) {
	// Apply environment variable overrides
	prefix := fmt.Sprintf("AG_UI_%s_", strings.ToUpper(agentName))
	
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}
			
			envKey := strings.TrimPrefix(parts[0], prefix)
			envValue := parts[1]
			
			// Convert environment key to config path
			configPath := acm.envKeyToPath(envKey)
			
			// Set the value
			acm.setValueByPath(config, configPath, envValue)
		}
	}
}

func (acm *AgentConfigManager) validateConfig(agentName string, config map[string]interface{}) error {
	acm.schemasMu.RLock()
	schema, exists := acm.schemas[agentName]
	acm.schemasMu.RUnlock()
	
	if !exists {
		// No schema registered, skip validation
		return nil
	}
	
	// Validate required fields
	for _, required := range schema.Required {
		if _, exists := config[required]; !exists {
			return fmt.Errorf("required field '%s' is missing", required)
		}
	}
	
	// Validate properties
	for propName, propSpec := range schema.Properties {
		value, exists := config[propName]
		if !exists && propSpec.Required {
			return fmt.Errorf("required property '%s' is missing", propName)
		}
		
		if exists {
			if err := acm.validateProperty(propName, value, propSpec); err != nil {
				return fmt.Errorf("property '%s' validation failed: %w", propName, err)
			}
		}
	}
	
	// Run custom validators
	for _, validator := range schema.Validators {
		if err := validator(config); err != nil {
			return fmt.Errorf("custom validation failed: %w", err)
		}
	}
	
	return nil
}

func (acm *AgentConfigManager) validateProperty(name string, value interface{}, spec *PropertySpec) error {
	// Type validation
	if err := acm.validateType(value, spec.Type); err != nil {
		return err
	}
	
	// Constraint validation
	if spec.Constraints != nil {
		if err := acm.validateConstraints(value, spec.Constraints); err != nil {
			return err
		}
	}
	
	return nil
}

func (acm *AgentConfigManager) validateType(value interface{}, expectedType string) error {
	actualType := reflect.TypeOf(value).Kind().String()
	
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %s", actualType)
		}
	case "int", "integer":
		if _, ok := value.(int); !ok {
			if _, ok := value.(int64); !ok {
				return fmt.Errorf("expected integer, got %s", actualType)
			}
		}
	case "float", "number":
		if _, ok := value.(float64); !ok {
			if _, ok := value.(float32); !ok {
				return fmt.Errorf("expected number, got %s", actualType)
			}
		}
	case "bool", "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %s", actualType)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("expected object, got %s", actualType)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("expected array, got %s", actualType)
		}
	}
	
	return nil
}

func (acm *AgentConfigManager) validateConstraints(value interface{}, constraints *PropertyConstraints) error {
	// Min/Max validation for numbers
	if constraints.Min != nil || constraints.Max != nil {
		var numValue float64
		switch v := value.(type) {
		case int:
			numValue = float64(v)
		case int64:
			numValue = float64(v)
		case float32:
			numValue = float64(v)
		case float64:
			numValue = v
		default:
			return fmt.Errorf("min/max constraints only apply to numeric values")
		}
		
		if constraints.Min != nil && numValue < *constraints.Min {
			return fmt.Errorf("value %v is less than minimum %v", numValue, *constraints.Min)
		}
		
		if constraints.Max != nil && numValue > *constraints.Max {
			return fmt.Errorf("value %v is greater than maximum %v", numValue, *constraints.Max)
		}
	}
	
	// Length validation for strings and arrays
	if constraints.MinLength != nil || constraints.MaxLength != nil {
		var length int
		switch v := value.(type) {
		case string:
			length = len(v)
		case []interface{}:
			length = len(v)
		default:
			return fmt.Errorf("length constraints only apply to strings and arrays")
		}
		
		if constraints.MinLength != nil && length < *constraints.MinLength {
			return fmt.Errorf("length %d is less than minimum %d", length, *constraints.MinLength)
		}
		
		if constraints.MaxLength != nil && length > *constraints.MaxLength {
			return fmt.Errorf("length %d is greater than maximum %d", length, *constraints.MaxLength)
		}
	}
	
	// Enum validation
	if len(constraints.Enum) > 0 {
		found := false
		for _, enumValue := range constraints.Enum {
			if reflect.DeepEqual(value, enumValue) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("value %v is not in allowed enum values %v", value, constraints.Enum)
		}
	}
	
	return nil
}

func (acm *AgentConfigManager) applyDefaults(agentName string, config map[string]interface{}) {
	acm.schemasMu.RLock()
	schema, exists := acm.schemas[agentName]
	acm.schemasMu.RUnlock()
	
	if !exists {
		return
	}
	
	// Apply schema defaults
	for key, value := range schema.Defaults {
		if _, exists := config[key]; !exists {
			config[key] = value
		}
	}
	
	// Apply property defaults
	for propName, propSpec := range schema.Properties {
		if _, exists := config[propName]; !exists && propSpec.Default != nil {
			config[propName] = propSpec.Default
		}
	}
}

func (acm *AgentConfigManager) unmarshalConfig(configMap map[string]interface{}) (*AgentConfig, error) {
	// Convert map to JSON and then unmarshal to AgentConfig
	jsonBytes, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config map: %w", err)
	}
	
	var config AgentConfig
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	return &config, nil
}

func (acm *AgentConfigManager) configToMap(config *AgentConfig) map[string]interface{} {
	// Convert AgentConfig to map
	jsonBytes, _ := json.Marshal(config)
	var configMap map[string]interface{}
	json.Unmarshal(jsonBytes, &configMap)
	return configMap
}

func (acm *AgentConfigManager) getValueByPath(config map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := config
	
	for i, part := range parts {
		if i == len(parts)-1 {
			return current[part]
		}
		
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	
	return nil
}

func (acm *AgentConfigManager) setValueByPath(config map[string]interface{}, path string, value interface{}) error {
	parts := strings.Split(path, ".")
	current := config
	
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return nil
		}
		
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			// Create intermediate objects
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}
	
	return nil
}

func (acm *AgentConfigManager) envKeyToPath(envKey string) string {
	// Convert SOME_CONFIG_KEY to some.config.key
	return strings.ToLower(strings.ReplaceAll(envKey, "_", "."))
}

func (acm *AgentConfigManager) calculateChecksum(config map[string]interface{}) string {
	// Simple checksum calculation
	jsonBytes, _ := json.Marshal(config)
	return fmt.Sprintf("checksum_%d", len(jsonBytes))
}

func (acm *AgentConfigManager) notifyConfigChange(event ConfigChangeEvent) {
	acm.listenersMu.RLock()
	defer acm.listenersMu.RUnlock()
	
	for _, listeners := range acm.listeners {
		for _, listener := range listeners {
			go listener(event) // Non-blocking notification
		}
	}
}

func (acm *AgentConfigManager) updateLoadTimeMetrics(duration time.Duration) {
	acm.metricsMu.Lock()
	defer acm.metricsMu.Unlock()
	
	if acm.metrics.AverageLoadTime == 0 {
		acm.metrics.AverageLoadTime = duration
	} else {
		acm.metrics.AverageLoadTime = (acm.metrics.AverageLoadTime + duration) / 2
	}
}

func (acm *AgentConfigManager) metricsLoop() {
	defer acm.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-acm.ctx.Done():
			return
		case <-ticker.C:
			// Update metrics
			acm.updateMetrics()
		}
	}
}

func (acm *AgentConfigManager) updateMetrics() {
	// Update any calculated metrics
	// This is a placeholder for more complex metrics calculations
}

// Configuration source implementations

// NewFileConfigSource creates a file-based configuration source.
func NewFileConfigSource(filePath string, format ConfigFormat, priority int) *FileConfigSource {
	return &FileConfigSource{
		filePath: filePath,
		format:   format,
		priority: priority,
	}
}

func (fcs *FileConfigSource) Load(ctx context.Context) (map[string]interface{}, error) {
	data, err := os.ReadFile(fcs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", fcs.filePath, err)
	}
	
	var config map[string]interface{}
	
	switch fcs.format {
	case ConfigFormatJSON:
		err = json.Unmarshal(data, &config)
	case ConfigFormatYAML:
		err = yaml.Unmarshal(data, &config)
	default:
		return nil, fmt.Errorf("unsupported config format: %s", fcs.format)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", fcs.filePath, err)
	}
	
	return config, nil
}

func (fcs *FileConfigSource) Watch(ctx context.Context, callback func(map[string]interface{})) error {
	// Simplified file watching implementation
	// In a real implementation, this would use filesystem notifications
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	var lastModTime time.Time
	
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if info, err := os.Stat(fcs.filePath); err == nil {
				if !lastModTime.IsZero() && info.ModTime().After(lastModTime) {
					// File has been modified
					if config, err := fcs.Load(ctx); err == nil {
						callback(config)
					}
				}
				lastModTime = info.ModTime()
			}
		}
	}
}

func (fcs *FileConfigSource) Name() string {
	return fmt.Sprintf("file:%s", filepath.Base(fcs.filePath))
}

func (fcs *FileConfigSource) Priority() int {
	return fcs.priority
}

// NewEnvConfigSource creates an environment variable configuration source.
func NewEnvConfigSource(prefix string, priority int) *EnvConfigSource {
	return &EnvConfigSource{
		prefix:   prefix,
		priority: priority,
	}
}

func (ecs *EnvConfigSource) Load(ctx context.Context) (map[string]interface{}, error) {
	config := make(map[string]interface{})
	
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, ecs.prefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}
			
			key := strings.TrimPrefix(parts[0], ecs.prefix)
			value := parts[1]
			
			// Convert key to lowercase and use dots as separators
			configKey := strings.ToLower(strings.ReplaceAll(key, "_", "."))
			config[configKey] = value
		}
	}
	
	return config, nil
}

func (ecs *EnvConfigSource) Watch(ctx context.Context, callback func(map[string]interface{})) error {
	// Environment variables don't typically change during runtime
	// This is a no-op implementation
	<-ctx.Done()
	return nil
}

func (ecs *EnvConfigSource) Name() string {
	return fmt.Sprintf("env:%s", ecs.prefix)
}

func (ecs *EnvConfigSource) Priority() int {
	return ecs.priority
}

// Helper functions for creating configuration schemas

// NewConfigSchema creates a new configuration schema.
func NewConfigSchema(name, version string) *ConfigSchema {
	return &ConfigSchema{
		Name:       name,
		Version:    version,
		Properties: make(map[string]*PropertySpec),
		Required:   make([]string, 0),
		Defaults:   make(map[string]interface{}),
		Validators: make([]ConfigValidator, 0),
	}
}

// AddProperty adds a property specification to the schema.
func (cs *ConfigSchema) AddProperty(name string, spec *PropertySpec) *ConfigSchema {
	cs.Properties[name] = spec
	if spec.Required {
		cs.Required = append(cs.Required, name)
	}
	if spec.Default != nil {
		cs.Defaults[name] = spec.Default
	}
	return cs
}

// AddValidator adds a custom validator to the schema.
func (cs *ConfigSchema) AddValidator(validator ConfigValidator) *ConfigSchema {
	cs.Validators = append(cs.Validators, validator)
	return cs
}

// NewPropertySpec creates a new property specification.
func NewPropertySpec(propType, description string) *PropertySpec {
	return &PropertySpec{
		Type:        propType,
		Description: description,
		Constraints: &PropertyConstraints{},
	}
}

// WithDefault sets the default value for the property.
func (ps *PropertySpec) WithDefault(value interface{}) *PropertySpec {
	ps.Default = value
	return ps
}

// WithRequired marks the property as required.
func (ps *PropertySpec) WithRequired(required bool) *PropertySpec {
	ps.Required = required
	return ps
}

// WithEnvVar sets the environment variable name for the property.
func (ps *PropertySpec) WithEnvVar(envVar string) *PropertySpec {
	ps.EnvVar = envVar
	return ps
}

// WithSensitive marks the property as sensitive (for logging/debugging).
func (ps *PropertySpec) WithSensitive(sensitive bool) *PropertySpec {
	ps.Sensitive = sensitive
	return ps
}

// WithMinMax sets min/max constraints for numeric properties.
func (ps *PropertySpec) WithMinMax(min, max float64) *PropertySpec {
	if ps.Constraints == nil {
		ps.Constraints = &PropertyConstraints{}
	}
	ps.Constraints.Min = &min
	ps.Constraints.Max = &max
	return ps
}

// WithLength sets length constraints for string/array properties.
func (ps *PropertySpec) WithLength(minLength, maxLength int) *PropertySpec {
	if ps.Constraints == nil {
		ps.Constraints = &PropertyConstraints{}
	}
	ps.Constraints.MinLength = &minLength
	ps.Constraints.MaxLength = &maxLength
	return ps
}

// WithEnum sets enum constraints for the property.
func (ps *PropertySpec) WithEnum(values ...interface{}) *PropertySpec {
	if ps.Constraints == nil {
		ps.Constraints = &PropertyConstraints{}
	}
	ps.Constraints.Enum = values
	return ps
}