package config

import (
	"context"
	"fmt"

	"github.com/ag-ui/go-sdk/pkg/di"
)

// ValidatorFactory provides a high-level API for creating validators with unified configuration
// It serves as the main entry point for the new configuration system while maintaining
// backward compatibility with existing APIs
type ValidatorFactory struct {
	registry *di.SimpleValidatorRegistry
}

// NewValidatorFactory creates a new validator factory with the given configuration
func NewValidatorFactory(config *ValidatorConfig) *ValidatorFactory {
	registry := di.NewSimpleValidatorRegistry(config)
	return &ValidatorFactory{
		registry: registry,
	}
}

// NewValidatorFactoryFromBuilder creates a validator factory from a configuration builder
func NewValidatorFactoryFromBuilder(builder *ValidatorBuilder) (*ValidatorFactory, error) {
	config, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build configuration: %w", err)
	}
	
	return NewValidatorFactory(config), nil
}

// NewValidatorFactoryFromLegacy creates a validator factory from legacy configurations
func NewValidatorFactoryFromLegacy(
	authConfig *AuthConfigCompat,
	cacheConfig *CacheValidatorConfigCompat,
	distributedConfig *DistributedValidatorConfigCompat,
) *ValidatorFactory {
	migrator := NewConfigMigrator()
	config := migrator.MigrateFromLegacy(authConfig, cacheConfig, distributedConfig)
	return NewValidatorFactory(config)
}

// CreateValidator creates the main composed validator
func (vf *ValidatorFactory) CreateValidator(ctx context.Context) (interface{}, error) {
	return vf.registry.GetMainValidator(ctx)
}

// CreateCoreValidator creates just the core validator
func (vf *ValidatorFactory) CreateCoreValidator(ctx context.Context) (interface{}, error) {
	return vf.registry.GetCoreValidator(ctx)
}

// CreateAuthValidator creates an authenticated validator (for backward compatibility)
func (vf *ValidatorFactory) CreateAuthValidator(ctx context.Context) (interface{}, error) {
	return vf.registry.GetService(ctx, "auth_validator")
}

// CreateCacheValidator creates a cache validator (for backward compatibility)
func (vf *ValidatorFactory) CreateCacheValidator(ctx context.Context) (interface{}, error) {
	return vf.registry.GetService(ctx, "cache_validator")
}

// CreateDistributedValidator creates a distributed validator (for backward compatibility)
func (vf *ValidatorFactory) CreateDistributedValidator(ctx context.Context) (interface{}, error) {
	return vf.registry.GetService(ctx, "distributed_validator")
}

// GetService returns a specific service by name
func (vf *ValidatorFactory) GetService(ctx context.Context, name string) (interface{}, error) {
	return vf.registry.GetService(ctx, name)
}

// GetServicesByTag returns all services with a specific tag
func (vf *ValidatorFactory) GetServicesByTag(ctx context.Context, tag string) ([]interface{}, error) {
	return vf.registry.GetServicesByTag(ctx, tag)
}

// AddInterceptor adds an interceptor to monitor service creation
func (vf *ValidatorFactory) AddInterceptor(interceptor di.Interceptor) {
	vf.registry.AddInterceptor(interceptor)
}

// CreateScope creates a new scope for scoped services
func (vf *ValidatorFactory) CreateScope() *di.Scope {
	return vf.registry.CreateScope()
}

// Validate validates the factory configuration
func (vf *ValidatorFactory) Validate() error {
	return vf.registry.Validate()
}

// GetConfiguration returns the underlying configuration
func (vf *ValidatorFactory) GetConfiguration() *ValidatorConfig {
	config, _ := vf.registry.GetService(context.Background(), "config")
	return config.(*ValidatorConfig)
}

// GetServiceNames returns all registered service names
func (vf *ValidatorFactory) GetServiceNames() []string {
	return vf.registry.GetServiceNames()
}

// Convenience factory methods for common scenarios

// CreateSimpleValidator creates a simple validator with minimal configuration
func CreateSimpleValidator() (*ValidatorFactory, error) {
	config := NewValidatorBuilder().
		WithValidationLevel(ValidationLevelPermissive).
		WithAuthenticationEnabled(false).
		WithCacheEnabled(false).
		WithDistributedEnabled(false).
		WithSecurityEnabled(false).
		MustBuild()
	
	return NewValidatorFactory(config), nil
}

// CreateDevelopmentValidator creates a validator configured for development
func CreateDevelopmentValidator() (*ValidatorFactory, error) {
	config := NewValidatorBuilder().
		ForDevelopment().
		MustBuild()
	
	return NewValidatorFactory(config), nil
}

// CreateProductionValidator creates a validator configured for production
func CreateProductionValidator() (*ValidatorFactory, error) {
	config := NewValidatorBuilder().
		ForProduction().
		MustBuild()
	
	return NewValidatorFactory(config), nil
}

// CreateTestingValidator creates a validator configured for testing
func CreateTestingValidator() (*ValidatorFactory, error) {
	config := NewValidatorBuilder().
		ForTesting().
		MustBuild()
	
	return NewValidatorFactory(config), nil
}

// CreateValidatorWithAuth creates a validator with authentication enabled
func CreateValidatorWithAuth(providerType string, providerConfig map[string]interface{}) (*ValidatorFactory, error) {
	config := NewValidatorBuilder().
		WithAuthenticationEnabled(true).
		WithAuthenticationProvider(providerType, providerConfig).
		WithRBAC(true, []string{"user"}).
		MustBuild()
	
	return NewValidatorFactory(config), nil
}

// CreateValidatorWithCache creates a validator with caching enabled
func CreateValidatorWithCache(l1Size int, l1TTL, l2TTL string) (*ValidatorFactory, error) {
	builder := NewValidatorBuilder().
		WithCacheEnabled(true)
	
	// Parse TTL strings and configure cache
	// This is a simplified example - would need proper parsing
	if l1TTL != "" {
		// builder = builder.WithL1Cache(l1Size, parseTTL(l1TTL))
	}
	
	config := builder.MustBuild()
	return NewValidatorFactory(config), nil
}

// CreateValidatorWithDistributed creates a validator with distributed validation
func CreateValidatorWithDistributed(nodeID, role, listenAddr string) (*ValidatorFactory, error) {
	config := NewValidatorBuilder().
		WithDistributedEnabled(true).
		WithDistributedNode(nodeID, role, listenAddr).
		MustBuild()
	
	return NewValidatorFactory(config), nil
}

// Legacy compatibility functions

// LegacyValidatorFactory provides backward compatibility with existing factory patterns
type LegacyValidatorFactory struct {
	factory *ValidatorFactory
}

// NewLegacyValidatorFactory creates a legacy-compatible factory
func NewLegacyValidatorFactory() *LegacyValidatorFactory {
	factory, _ := CreateSimpleValidator()
	return &LegacyValidatorFactory{factory: factory}
}

// WithAuthConfig configures authentication using legacy config
func (lvf *LegacyValidatorFactory) WithAuthConfig(authConfig *AuthConfigCompat) *LegacyValidatorFactory {
	if authConfig != nil {
		// Create new factory with auth configuration
		unified := WrapLegacyAuthConfig(authConfig)
		lvf.factory = NewValidatorFactory(unified)
	}
	return lvf
}

// WithCacheConfig configures caching using legacy config
func (lvf *LegacyValidatorFactory) WithCacheConfig(cacheConfig *CacheValidatorConfigCompat) *LegacyValidatorFactory {
	if cacheConfig != nil {
		// Merge cache config with existing configuration
		// This is simplified - would need proper merging logic
		config := lvf.factory.GetConfiguration()
		config.Cache = cacheConfig.ToUnifiedConfig()
		lvf.factory = NewValidatorFactory(config)
	}
	return lvf
}

// WithDistributedConfig configures distributed validation using legacy config
func (lvf *LegacyValidatorFactory) WithDistributedConfig(distributedConfig *DistributedValidatorConfigCompat) *LegacyValidatorFactory {
	if distributedConfig != nil {
		// Merge distributed config with existing configuration
		config := lvf.factory.GetConfiguration()
		config.Distributed = distributedConfig.ToUnifiedConfig()
		lvf.factory = NewValidatorFactory(config)
	}
	return lvf
}

// Build creates the validator using the legacy factory
func (lvf *LegacyValidatorFactory) Build(ctx context.Context) (interface{}, error) {
	return lvf.factory.CreateValidator(ctx)
}

// BuildAuth creates an authenticated validator using the legacy factory
func (lvf *LegacyValidatorFactory) BuildAuth(ctx context.Context) (interface{}, error) {
	return lvf.factory.CreateAuthValidator(ctx)
}

// BuildCache creates a cache validator using the legacy factory
func (lvf *LegacyValidatorFactory) BuildCache(ctx context.Context) (interface{}, error) {
	return lvf.factory.CreateCacheValidator(ctx)
}

// BuildDistributed creates a distributed validator using the legacy factory
func (lvf *LegacyValidatorFactory) BuildDistributed(ctx context.Context) (interface{}, error) {
	return lvf.factory.CreateDistributedValidator(ctx)
}

// Configuration presets for common scenarios

// ConfigurationPreset represents a pre-configured setup
type ConfigurationPreset struct {
	Name        string
	Description string
	Config      *ValidatorConfig
}

// GetConfigurationPresets returns available configuration presets
func GetConfigurationPresets() []*ConfigurationPreset {
	return []*ConfigurationPreset{
		{
			Name:        "minimal",
			Description: "Minimal configuration with only core validation",
			Config: NewValidatorBuilder().
				WithValidationLevel(ValidationLevelPermissive).
				WithAuthenticationEnabled(false).
				WithCacheEnabled(false).
				WithDistributedEnabled(false).
				WithSecurityEnabled(false).
				WithAnalytics(func(config *AnalyticsValidationConfig) {
					config.Enabled = false
				}).
				MustBuild(),
		},
		{
			Name:        "development",
			Description: "Development-friendly configuration",
			Config:      DevelopmentValidatorConfig(),
		},
		{
			Name:        "production",
			Description: "Production-ready configuration with all features enabled",
			Config:      ProductionValidatorConfig(),
		},
		{
			Name:        "testing",
			Description: "Testing configuration with fast execution",
			Config:      TestingValidatorConfig(),
		},
		{
			Name:        "secure",
			Description: "Security-focused configuration",
			Config: NewValidatorBuilder().
				WithValidationLevel(ValidationLevelStrict).
				WithAuthenticationEnabled(true).
				WithRBAC(true, []string{"user"}).
				WithSecurityEnabled(true).
				WithRateLimiting(true, 100, 60, 10).
				WithEncryption(true, "AES256", "").
				MustBuild(),
		},
		{
			Name:        "performance",
			Description: "Performance-optimized configuration",
			Config: NewValidatorBuilder().
				WithCacheEnabled(true).
				WithL1Cache(100000, 300).
				WithPerformanceFeatures(true).
				WithConcurrency(1000).
				MustBuild(),
		},
		{
			Name:        "distributed",
			Description: "Distributed validation configuration",
			Config: NewValidatorBuilder().
				WithDistributedEnabled(true).
				WithDistributedNode("node-1", "leader", ":8080").
				WithConsensus("raft", 5, 3, 10).
				WithTLS(true, "", "", "", false).
				MustBuild(),
		},
	}
}

// CreateValidatorFromPreset creates a validator factory from a preset
func CreateValidatorFromPreset(presetName string) (*ValidatorFactory, error) {
	presets := GetConfigurationPresets()
	
	for _, preset := range presets {
		if preset.Name == presetName {
			return NewValidatorFactory(preset.Config), nil
		}
	}
	
	return nil, fmt.Errorf("unknown configuration preset: %s", presetName)
}

// ListAvailablePresets returns the names of available presets
func ListAvailablePresets() []string {
	presets := GetConfigurationPresets()
	names := make([]string, len(presets))
	
	for i, preset := range presets {
		names[i] = preset.Name
	}
	
	return names
}

// ValidatorFactoryRegistry manages multiple validator factories
type ValidatorFactoryRegistry struct {
	factories map[string]*ValidatorFactory
}

// NewValidatorFactoryRegistry creates a new factory registry
func NewValidatorFactoryRegistry() *ValidatorFactoryRegistry {
	return &ValidatorFactoryRegistry{
		factories: make(map[string]*ValidatorFactory),
	}
}

// Register registers a validator factory with a name
func (vfr *ValidatorFactoryRegistry) Register(name string, factory *ValidatorFactory) {
	vfr.factories[name] = factory
}

// Get retrieves a validator factory by name
func (vfr *ValidatorFactoryRegistry) Get(name string) (*ValidatorFactory, bool) {
	factory, exists := vfr.factories[name]
	return factory, exists
}

// CreateValidator creates a validator from a registered factory
func (vfr *ValidatorFactoryRegistry) CreateValidator(ctx context.Context, factoryName string) (interface{}, error) {
	factory, exists := vfr.factories[factoryName]
	if !exists {
		return nil, fmt.Errorf("validator factory not found: %s", factoryName)
	}
	
	return factory.CreateValidator(ctx)
}

// List returns all registered factory names
func (vfr *ValidatorFactoryRegistry) List() []string {
	names := make([]string, 0, len(vfr.factories))
	for name := range vfr.factories {
		names = append(names, name)
	}
	return names
}

// Global factory registry for convenience
var globalFactoryRegistry = NewValidatorFactoryRegistry()

// RegisterGlobalFactory registers a factory globally
func RegisterGlobalFactory(name string, factory *ValidatorFactory) {
	globalFactoryRegistry.Register(name, factory)
}

// GetGlobalFactory retrieves a globally registered factory
func GetGlobalFactory(name string) (*ValidatorFactory, bool) {
	return globalFactoryRegistry.Get(name)
}

// CreateGlobalValidator creates a validator from a globally registered factory
func CreateGlobalValidator(ctx context.Context, factoryName string) (interface{}, error) {
	return globalFactoryRegistry.CreateValidator(ctx, factoryName)
}

// Initialize common global factories
func init() {
	// Register common presets as global factories
	presets := GetConfigurationPresets()
	for _, preset := range presets {
		factory := NewValidatorFactory(preset.Config)
		RegisterGlobalFactory(preset.Name, factory)
	}
}