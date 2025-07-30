package config

import (
	"time"
)

// ConfigProvider provides a unified interface for configuration access
type ConfigProvider interface {
	// Get retrieves a configuration value by key
	Get(key string) (interface{}, error)
	
	// GetString retrieves a string configuration value
	GetString(key string) (string, error)
	
	// GetInt retrieves an integer configuration value
	GetInt(key string) (int, error)
	
	// GetDuration retrieves a duration configuration value
	GetDuration(key string) (time.Duration, error)
	
	// GetBool retrieves a boolean configuration value
	GetBool(key string) (bool, error)
	
	// GetMap retrieves a map configuration value
	GetMap(key string) (map[string]interface{}, error)
	
	// GetSlice retrieves a slice configuration value
	GetSlice(key string) ([]interface{}, error)
	
	// Set sets a configuration value
	Set(key string, value interface{}) error
	
	// Has checks if a configuration key exists
	Has(key string) bool
	
	// GetAll returns all configuration values
	GetAll() map[string]interface{}
	
	// Watch watches for configuration changes
	Watch(callback func(key string, value interface{})) error
	
	// Validate validates the configuration
	Validate() error
}

// ModuleConfig represents configuration for a specific module
type ModuleConfig interface {
	// GetModuleName returns the name of the module
	GetModuleName() string
	
	// GetConfigKeys returns the configuration keys used by this module
	GetConfigKeys() []string
	
	// Validate validates the module configuration
	Validate() error
	
	// GetDefaults returns default configuration values for this module
	GetDefaults() map[string]interface{}
	
	// ApplyDefaults applies default values to missing configuration
	ApplyDefaults(provider ConfigProvider) error
}

// CoreConfigProvider provides core validation configuration
type CoreConfigProvider interface {
	// GetCoreConfig returns core validation configuration
	GetCoreConfig() (*CoreValidationConfig, error)
}

// AuthConfigProvider provides authentication configuration
type AuthConfigProvider interface {
	// GetAuthConfig returns authentication configuration
	GetAuthConfig() (*AuthValidationConfig, error)
}

// CacheConfigProvider provides cache configuration
type CacheConfigProvider interface {
	// GetCacheConfig returns cache configuration
	GetCacheConfig() (*CacheValidationConfig, error)
}

// DistributedConfigProvider provides distributed validation configuration
type DistributedConfigProvider interface {
	// GetDistributedConfig returns distributed validation configuration
	GetDistributedConfig() (*DistributedValidationConfig, error)
}

// AnalyticsConfigProvider provides analytics configuration
type AnalyticsConfigProvider interface {
	// GetAnalyticsConfig returns analytics configuration
	GetAnalyticsConfig() (*AnalyticsValidationConfig, error)
}

// SecurityConfigProvider provides security configuration
type SecurityConfigProvider interface {
	// GetSecurityConfig returns security configuration
	GetSecurityConfig() (*SecurityValidationConfig, error)
}

// FeatureFlagProvider provides feature flag configuration
type FeatureFlagProvider interface {
	// GetFeatureFlags returns feature flags
	GetFeatureFlags() (*FeatureFlags, error)
	
	// IsEnabled checks if a feature is enabled
	IsEnabled(feature string) bool
}

// EnvironmentProvider provides environment configuration
type EnvironmentProvider interface {
	// GetGlobalSettings returns global settings
	GetGlobalSettings() (*GlobalSettings, error)
	
	// GetEnvironment returns the current environment
	GetEnvironment() string
}

// ValidatorConfigProvider provides configuration for validation components
// Composed of focused interfaces following Interface Segregation Principle
type ValidatorConfigProvider interface {
	ConfigProvider
	CoreConfigProvider
	AuthConfigProvider
	CacheConfigProvider
	DistributedConfigProvider
	AnalyticsConfigProvider
	SecurityConfigProvider
	FeatureFlagProvider
	EnvironmentProvider
}

// ConfigRegistry manages configuration providers for different modules
type ConfigRegistry interface {
	// Register registers a configuration provider for a module
	Register(moduleName string, provider ConfigProvider) error
	
	// Unregister removes a configuration provider
	Unregister(moduleName string) error
	
	// Get retrieves a configuration provider for a module
	Get(moduleName string) (ConfigProvider, error)
	
	// GetAll returns all registered configuration providers
	GetAll() map[string]ConfigProvider
	
	// GetGlobal returns the global configuration provider
	GetGlobal() ConfigProvider
	
	// SetGlobal sets the global configuration provider
	SetGlobal(provider ConfigProvider) error
	
	// Validate validates all registered configurations
	Validate() error
}

// ConfigurationFactory creates configuration instances
type ConfigurationFactory interface {
	// CreateConfig creates a configuration instance for a module
	CreateConfig(moduleName string, options ...ConfigOption) (ConfigProvider, error)
	
	// CreateValidatorConfig creates a validator configuration
	CreateValidatorConfig(options ...ConfigOption) (ValidatorConfigProvider, error)
	
	// CreateFromFile creates configuration from a file
	CreateFromFile(filePath string, options ...ConfigOption) (ConfigProvider, error)
	
	// CreateFromMap creates configuration from a map
	CreateFromMap(data map[string]interface{}, options ...ConfigOption) (ConfigProvider, error)
}

// ConfigOption represents a configuration option
type ConfigOption interface {
	// Apply applies the option to a configuration builder
	Apply(builder interface{}) error
}

// ServiceRegistryInterface manages service registration and retrieval
type ServiceRegistryInterface interface {
	// Register registers a service with the container
	Register(name string, service interface{}) error
	
	// RegisterFactory registers a factory function for a service
	RegisterFactory(name string, factory func() (interface{}, error)) error
	
	// RegisterSingleton registers a singleton service
	RegisterSingleton(name string, service interface{}) error
	
	// Get retrieves a service by name
	Get(name string) (interface{}, error)
	
	// GetTyped retrieves a service by name with type assertion
	GetTyped(name string, target interface{}) error
	
	// Has checks if a service is registered
	Has(name string) bool
	
	// Remove removes a service from the container
	Remove(name string) error
	
	// GetAll returns all registered services
	GetAll() map[string]interface{}
}

// ServiceLifecycleManager manages the lifecycle of registered services
type ServiceLifecycleManager interface {
	// Start starts all registered services that implement Startable
	Start() error
	
	// Stop stops all registered services that implement Stoppable
	Stop() error
	
	// Validate validates all registered services
	Validate() error
}

// ServiceContainer provides dependency injection for validation components
// Composed of focused interfaces following Interface Segregation Principle
type ServiceContainer interface {
	ServiceRegistryInterface
	ServiceLifecycleManager
}

// Startable interface for services that can be started
type Startable interface {
	Start() error
}

// Stoppable interface for services that can be stopped
type Stoppable interface {
	Stop() error
}

// Validatable interface for services that can be validated
type Validatable interface {
	Validate() error
}

// Configurable interface for services that can be configured
type Configurable interface {
	Configure(config ConfigProvider) error
}

// ServiceMetadata provides metadata about a service
type ServiceMetadata interface {
	// GetName returns the service name
	GetName() string
	
	// GetVersion returns the service version
	GetVersion() string
	
	// GetDependencies returns the service dependencies
	GetDependencies() []string
}

// ServiceHealth provides health check capabilities
type ServiceHealth interface {
	// IsHealthy checks if the service is healthy
	IsHealthy() bool
}

// ValidationService represents a validation service
// Composed of focused interfaces following Interface Segregation Principle
type ValidationService interface {
	Startable
	Stoppable
	Validatable
	Configurable
	ServiceMetadata
	ServiceHealth
}

// DistributedValidatorService represents the distributed validator service
type DistributedValidatorService interface {
	ValidationService
	
	// ValidateEvent validates a single event
	ValidateEvent(ctx interface{}, event interface{}) (interface{}, error)
	
	// ValidateSequence validates a sequence of events
	ValidateSequence(ctx interface{}, events []interface{}) (interface{}, error)
	
	// RegisterNode registers a validation node
	RegisterNode(nodeInfo interface{}) error
	
	// UnregisterNode removes a validation node
	UnregisterNode(nodeID string) error
	
	// GetMetrics returns validation metrics
	GetMetrics() interface{}
	
	// GetNodeInfo returns node information
	GetNodeInfo(nodeID string) (interface{}, error)
}

// ConfigurationLifecycle manages initialization and lifecycle of configuration system
type ConfigurationLifecycle interface {
	// Initialize initializes the configuration system
	Initialize() error
	
	// Start starts the configuration system
	Start() error
	
	// Stop stops the configuration system
	Stop() error
	
	// Validate validates the entire configuration system
	Validate() error
}

// ConfigurationPersistence handles loading and saving configuration
type ConfigurationPersistence interface {
	// Load loads configuration from various sources
	Load(sources ...string) error
	
	// Save saves configuration to a destination
	Save(destination string) error
	
	// Reload reloads configuration from sources
	Reload() error
}

// ConfigurationAccess provides access to configuration components
type ConfigurationAccess interface {
	// GetProvider returns the primary configuration provider
	GetProvider() ValidatorConfigProvider
	
	// GetRegistry returns the configuration registry
	GetRegistry() ConfigRegistry
	
	// GetFactory returns the configuration factory
	GetFactory() ConfigurationFactory
	
	// GetContainer returns the service container
	GetContainer() ServiceContainer
}

// ConfigurationHealth provides health monitoring for configuration system
type ConfigurationHealth interface {
	// GetHealth returns the health status of the configuration system
	GetHealth() map[string]interface{}
}

// ConfigurationManager manages the overall configuration system
// Composed of focused interfaces following Interface Segregation Principle
type ConfigurationManager interface {
	ConfigurationLifecycle
	ConfigurationPersistence
	ConfigurationAccess
	ConfigurationHealth
}