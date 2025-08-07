package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
)

// ConfigurationFactoryImpl implements ConfigurationFactory
type ConfigurationFactoryImpl struct {
	registry *ConfigRegistryImpl
	mutex    sync.RWMutex
}

// NewConfigurationFactory creates a new configuration factory
func NewConfigurationFactory() *ConfigurationFactoryImpl {
	return &ConfigurationFactoryImpl{
		registry: NewConfigRegistry(),
	}
}

// CreateConfig creates a configuration instance for a module
func (f *ConfigurationFactoryImpl) CreateConfig(moduleName string, options ...ConfigOption) (ConfigProvider, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Start with default configuration
	config := DefaultValidatorConfig()

	// Apply options
	for _, option := range options {
		if err := option.Apply(config); err != nil {
			return nil, fmt.Errorf("failed to apply config option: %w", err)
		}
	}

	// Create provider
	provider := NewUnifiedConfigProvider(config)

	// Register with registry
	if err := f.registry.Register(moduleName, provider); err != nil {
		return nil, fmt.Errorf("failed to register config provider for module %s: %w", moduleName, err)
	}

	return provider, nil
}

// CreateValidatorConfig creates a validator configuration
func (f *ConfigurationFactoryImpl) CreateValidatorConfig(options ...ConfigOption) (ValidatorConfigProvider, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Start with default configuration
	config := DefaultValidatorConfig()

	// Apply options
	for _, option := range options {
		if err := option.Apply(config); err != nil {
			return nil, fmt.Errorf("failed to apply config option: %w", err)
		}
	}

	// Create provider
	provider := NewUnifiedValidatorConfigProvider(config)

	return provider, nil
}

// CreateFromFile creates configuration from a file
func (f *ConfigurationFactoryImpl) CreateFromFile(filePath string, options ...ConfigOption) (ConfigProvider, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Read file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	// Parse based on file extension
	config := &ValidatorConfig{}
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	// Apply options
	for _, option := range options {
		if err := option.Apply(config); err != nil {
			return nil, fmt.Errorf("failed to apply config option: %w", err)
		}
	}

	// Create provider
	provider := NewUnifiedConfigProvider(config)

	return provider, nil
}

// CreateFromMap creates configuration from a map
func (f *ConfigurationFactoryImpl) CreateFromMap(data map[string]interface{}, options ...ConfigOption) (ConfigProvider, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Convert map to JSON and then to struct
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config data: %w", err)
	}

	config := &ValidatorConfig{}
	if err := json.Unmarshal(jsonData, config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config data: %w", err)
	}

	// Apply options
	for _, option := range options {
		if err := option.Apply(config); err != nil {
			return nil, fmt.Errorf("failed to apply config option: %w", err)
		}
	}

	// Create provider
	provider := NewUnifiedConfigProvider(config)

	return provider, nil
}

// GetRegistry returns the configuration registry
func (f *ConfigurationFactoryImpl) GetRegistry() ConfigRegistry {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	return f.registry
}

// ConfigOption implementations

// EnvironmentOption sets the environment
type EnvironmentOption struct {
	Environment string
}

func (o *EnvironmentOption) Apply(builder interface{}) error {
	if config, ok := builder.(*ValidatorConfig); ok {
		if config.Global == nil {
			config.Global = DefaultGlobalSettings()
		}
		config.Global.Environment = o.Environment
		return nil
	}
	return fmt.Errorf("invalid builder type for environment option")
}

// NewEnvironmentOption creates a new environment option
func NewEnvironmentOption(env string) ConfigOption {
	return &EnvironmentOption{Environment: env}
}

// ValidationLevelOption sets the validation level
type ValidationLevelOption struct {
	Level ValidationLevel
}

func (o *ValidationLevelOption) Apply(builder interface{}) error {
	if config, ok := builder.(*ValidatorConfig); ok {
		if config.Core == nil {
			config.Core = DefaultCoreValidationConfig()
		}
		config.Core.Level = o.Level
		return nil
	}
	return fmt.Errorf("invalid builder type for validation level option")
}

// NewValidationLevelOption creates a new validation level option
func NewValidationLevelOption(level ValidationLevel) ConfigOption {
	return &ValidationLevelOption{Level: level}
}

// NodeIDOption sets the distributed node ID
type NodeIDOption struct {
	NodeID string
}

func (o *NodeIDOption) Apply(builder interface{}) error {
	if config, ok := builder.(*ValidatorConfig); ok {
		if config.Distributed == nil {
			config.Distributed = DefaultDistributedValidationConfig()
		}
		config.Distributed.NodeID = o.NodeID
		return nil
	}
	return fmt.Errorf("invalid builder type for node ID option")
}

// NewNodeIDOption creates a new node ID option
func NewNodeIDOption(nodeID string) ConfigOption {
	return &NodeIDOption{NodeID: nodeID}
}

// ConfigurationManager implements ConfigurationManager
type ConfigurationManagerImpl struct {
	provider  ValidatorConfigProvider
	registry  ConfigRegistry
	factory   ConfigurationFactory
	container ServiceContainer
	sources   []string
	mutex     sync.RWMutex
	started   bool
}

// NewConfigurationManager creates a new configuration manager
func NewConfigurationManager() *ConfigurationManagerImpl {
	factory := NewConfigurationFactory()
	return &ConfigurationManagerImpl{
		registry:  NewConfigRegistry(),
		factory:   factory,
		container: NewServiceRegistry(),
		sources:   make([]string, 0),
	}
}

// Initialize initializes the configuration system
func (m *ConfigurationManagerImpl) Initialize() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Create default provider if none exists
	if m.provider == nil {
		provider, err := m.factory.CreateValidatorConfig()
		if err != nil {
			return fmt.Errorf("failed to create default config provider: %w", err)
		}
		m.provider = provider
	}

	// Register core services
	if err := m.registerCoreServices(); err != nil {
		return fmt.Errorf("failed to register core services: %w", err)
	}

	return nil
}

// Load loads configuration from various sources
func (m *ConfigurationManagerImpl) Load(sources ...string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.sources = sources

	// Load from each source
	for _, source := range sources {
		if err := m.loadFromSource(source); err != nil {
			return fmt.Errorf("failed to load config from source %s: %w", source, err)
		}
	}

	return nil
}

// Save saves configuration to a destination
func (m *ConfigurationManagerImpl) Save(destination string) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.provider == nil {
		return fmt.Errorf("no configuration provider available")
	}

	// Get all config data
	data := m.provider.GetAll()

	// Convert to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config data: %w", err)
	}

	// Write to file
	if err := ioutil.WriteFile(destination, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Reload reloads configuration from sources
func (m *ConfigurationManagerImpl) Reload() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Reload from all sources
	for _, source := range m.sources {
		if err := m.loadFromSource(source); err != nil {
			return fmt.Errorf("failed to reload config from source %s: %w", source, err)
		}
	}

	return nil
}

// GetProvider returns the primary configuration provider
func (m *ConfigurationManagerImpl) GetProvider() ValidatorConfigProvider {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.provider
}

// GetRegistry returns the configuration registry
func (m *ConfigurationManagerImpl) GetRegistry() ConfigRegistry {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.registry
}

// GetFactory returns the configuration factory
func (m *ConfigurationManagerImpl) GetFactory() ConfigurationFactory {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.factory
}

// GetContainer returns the service container
func (m *ConfigurationManagerImpl) GetContainer() ServiceContainer {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.container
}

// Validate validates the entire configuration system
func (m *ConfigurationManagerImpl) Validate() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var errors []error

	// Validate provider
	if m.provider != nil {
		if err := m.provider.Validate(); err != nil {
			errors = append(errors, fmt.Errorf("provider validation failed: %w", err))
		}
	}

	// Validate registry
	if err := m.registry.Validate(); err != nil {
		errors = append(errors, fmt.Errorf("registry validation failed: %w", err))
	}

	// Validate container
	if err := m.container.Validate(); err != nil {
		errors = append(errors, fmt.Errorf("container validation failed: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration system validation failed: %v", errors)
	}

	return nil
}

// Start starts the configuration system
func (m *ConfigurationManagerImpl) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.started {
		return fmt.Errorf("configuration manager already started")
	}

	// Start the service container
	if err := m.container.Start(); err != nil {
		return fmt.Errorf("failed to start service container: %w", err)
	}

	m.started = true
	return nil
}

// Stop stops the configuration system
func (m *ConfigurationManagerImpl) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.started {
		return nil
	}

	// Stop the service container
	if err := m.container.Stop(); err != nil {
		return fmt.Errorf("failed to stop service container: %w", err)
	}

	m.started = false
	return nil
}

// GetHealth returns the health status of the configuration system
func (m *ConfigurationManagerImpl) GetHealth() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	health := map[string]interface{}{
		"started":            m.started,
		"provider_available": m.provider != nil,
		"sources_count":      len(m.sources),
		"registry_providers": len(m.registry.GetAll()),
		"container_services": len(m.container.GetAll()),
	}

	// Check if validation passes
	if err := m.Validate(); err != nil {
		health["validation_error"] = err.Error()
		health["healthy"] = false
	} else {
		health["healthy"] = true
	}

	return health
}

// loadFromSource loads configuration from a specific source
func (m *ConfigurationManagerImpl) loadFromSource(source string) error {
	// Check if source is a file
	if strings.HasSuffix(source, ".json") {
		provider, err := m.factory.CreateFromFile(source)
		if err != nil {
			return fmt.Errorf("failed to load config from file: %w", err)
		}

		// Update main provider
		if validatorProvider, ok := provider.(*UnifiedConfigProvider); ok {
			m.provider = &UnifiedValidatorConfigProvider{
				UnifiedConfigProvider: validatorProvider,
			}
		}

		return nil
	}

	// Add support for other sources (environment variables, etc.)
	return fmt.Errorf("unsupported config source: %s", source)
}

// registerCoreServices registers core services with the container
func (m *ConfigurationManagerImpl) registerCoreServices() error {
	// Register configuration provider
	if err := m.container.RegisterSingleton("config.provider", m.provider); err != nil {
		return fmt.Errorf("failed to register config provider: %w", err)
	}

	// Register configuration registry
	if err := m.container.RegisterSingleton("config.registry", m.registry); err != nil {
		return fmt.Errorf("failed to register config registry: %w", err)
	}

	// Register configuration factory
	if err := m.container.RegisterSingleton("config.factory", m.factory); err != nil {
		return fmt.Errorf("failed to register config factory: %w", err)
	}

	return nil
}
