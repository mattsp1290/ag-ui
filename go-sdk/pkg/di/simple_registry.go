package di

import (
	"context"
	"fmt"
	"reflect"
)

// SimpleValidatorRegistry provides a simplified validator registry without circular dependencies
type SimpleValidatorRegistry struct {
	container *Container
	config    ValidatorConfigInterface
}

// NewSimpleValidatorRegistry creates a new simplified validator registry
func NewSimpleValidatorRegistry(cfg ValidatorConfigInterface) *SimpleValidatorRegistry {
	registry := &SimpleValidatorRegistry{
		container: NewContainer(),
		config:    cfg,
	}
	
	// Register core services
	registry.registerCoreServices()
	
	return registry
}

// registerCoreServices registers the core validator services
func (vr *SimpleValidatorRegistry) registerCoreServices() {
	// Register configuration as singleton
	vr.container.RegisterInstance("config", vr.config)
	
	// Register core validation service
	vr.container.RegisterSingleton("core_validator", func(ctx context.Context, c *Container) (interface{}, error) {
		config, err := c.Get(ctx, "config")
		if err != nil {
			return nil, err
		}
		
		cfg := config.(ValidatorConfigInterface)
		return vr.createCoreValidator(ctx, cfg.GetCore())
	}).WithTags("validator", "core")
	
	// Register authentication service if enabled
	if vr.config.GetAuth() != nil && vr.config.GetAuth().IsEnabled() {
		vr.registerAuthServices()
	}
	
	// Register cache service if enabled
	if vr.config.GetCache() != nil && vr.config.GetCache().IsEnabled() {
		vr.registerCacheServices()
	}
	
	// Register distributed service if enabled
	if vr.config.GetDistributed() != nil && vr.config.GetDistributed().IsEnabled() {
		vr.registerDistributedServices()
	}
	
	// Register analytics service if enabled
	if vr.config.GetAnalytics() != nil && vr.config.GetAnalytics().IsEnabled() {
		vr.registerAnalyticsServices()
	}
	
	// Register security service if enabled
	if vr.config.GetSecurity() != nil && vr.config.GetSecurity().IsEnabled() {
		vr.registerSecurityServices()
	}
	
	// Register main validator service that composes all components
	vr.registerMainValidator()
}

// registerAuthServices registers authentication-related services
func (vr *SimpleValidatorRegistry) registerAuthServices() {
	// Register auth provider
	vr.container.RegisterSingleton("auth_provider", func(ctx context.Context, c *Container) (interface{}, error) {
		config, err := c.Get(ctx, "config")
		if err != nil {
			return nil, err
		}
		
		cfg := config.(ValidatorConfigInterface)
		return vr.createAuthProvider(ctx, cfg.GetAuth())
	}).WithTags("auth", "provider")
	
	// Register auth validator
	vr.container.RegisterSingleton("auth_validator", func(ctx context.Context, c *Container) (interface{}, error) {
		coreValidator, err := c.Get(ctx, "core_validator")
		if err != nil {
			return nil, err
		}
		
		authProvider, err := c.Get(ctx, "auth_provider")
		if err != nil {
			return nil, err
		}
		
		config, err := c.Get(ctx, "config")
		if err != nil {
			return nil, err
		}
		
		cfg := config.(ValidatorConfigInterface)
		return vr.createAuthValidator(ctx, coreValidator, authProvider, cfg.GetAuth())
	}).WithDependencies("core_validator", "auth_provider").WithTags("validator", "auth")
}

// registerCacheServices registers cache-related services
func (vr *SimpleValidatorRegistry) registerCacheServices() {
	// Register cache validator
	deps := []string{"core_validator"}
	
	vr.container.RegisterSingleton("cache_validator", func(ctx context.Context, c *Container) (interface{}, error) {
		coreValidator, err := c.Get(ctx, "core_validator")
		if err != nil {
			return nil, err
		}
		
		config, err := c.Get(ctx, "config")
		if err != nil {
			return nil, err
		}
		
		cfg := config.(ValidatorConfigInterface)
		return vr.createCacheValidator(ctx, coreValidator, cfg.GetCache())
	}).WithDependencies(deps...).WithTags("validator", "cache")
}

// registerDistributedServices registers distributed validation services
func (vr *SimpleValidatorRegistry) registerDistributedServices() {
	// Register distributed validator
	vr.container.RegisterSingleton("distributed_validator", func(ctx context.Context, c *Container) (interface{}, error) {
		coreValidator, err := c.Get(ctx, "core_validator")
		if err != nil {
			return nil, err
		}
		
		config, err := c.Get(ctx, "config")
		if err != nil {
			return nil, err
		}
		
		cfg := config.(ValidatorConfigInterface)
		return vr.createDistributedValidator(ctx, coreValidator, cfg.GetDistributed())
	}).WithDependencies("core_validator").WithTags("validator", "distributed")
}

// registerAnalyticsServices registers analytics and monitoring services
func (vr *SimpleValidatorRegistry) registerAnalyticsServices() {
	// Register analytics service
	vr.container.RegisterSingleton("analytics_service", func(ctx context.Context, c *Container) (interface{}, error) {
		config, err := c.Get(ctx, "config")
		if err != nil {
			return nil, err
		}
		
		cfg := config.(ValidatorConfigInterface)
		return vr.createAnalyticsService(ctx, cfg.GetAnalytics())
	}).WithTags("analytics", "service")
}

// registerSecurityServices registers security-related services
func (vr *SimpleValidatorRegistry) registerSecurityServices() {
	// Register security validator
	vr.container.RegisterSingleton("security_validator", func(ctx context.Context, c *Container) (interface{}, error) {
		config, err := c.Get(ctx, "config")
		if err != nil {
			return nil, err
		}
		
		cfg := config.(ValidatorConfigInterface)
		return vr.createSecurityValidator(ctx, cfg.GetSecurity())
	}).WithTags("validator", "security")
}

// registerMainValidator registers the main validator that composes all components
func (vr *SimpleValidatorRegistry) registerMainValidator() {
	deps := []string{"core_validator"}
	
	if vr.config.GetAuth() != nil && vr.config.GetAuth().IsEnabled() {
		deps = append(deps, "auth_validator")
	}
	if vr.config.GetCache() != nil && vr.config.GetCache().IsEnabled() {
		deps = append(deps, "cache_validator")
	}
	if vr.config.GetDistributed() != nil && vr.config.GetDistributed().IsEnabled() {
		deps = append(deps, "distributed_validator")
	}
	if vr.config.GetAnalytics() != nil && vr.config.GetAnalytics().IsEnabled() {
		deps = append(deps, "analytics_service")
	}
	if vr.config.GetSecurity() != nil && vr.config.GetSecurity().IsEnabled() {
		deps = append(deps, "security_validator")
	}
	
	vr.container.RegisterSingleton("main_validator", func(ctx context.Context, c *Container) (interface{}, error) {
		// Get all the components
		components := make(map[string]interface{})
		
		for _, dep := range deps {
			component, err := c.Get(ctx, dep)
			if err != nil {
				return nil, fmt.Errorf("failed to get component %s: %w", dep, err)
			}
			components[dep] = component
		}
		
		// Create the main validator that composes all components
		return vr.createMainValidator(ctx, components)
	}).WithDependencies(deps...).WithTags("validator", "main")
}

// Factory methods for creating services (simplified versions)

func (vr *SimpleValidatorRegistry) createCoreValidator(ctx context.Context, cfg CoreConfigInterface) (interface{}, error) {
	return fmt.Sprintf("CoreValidator(level=%s)", cfg.GetLevel()), nil
}

func (vr *SimpleValidatorRegistry) createAuthProvider(ctx context.Context, cfg AuthConfigInterface) (interface{}, error) {
	return fmt.Sprintf("AuthProvider(type=%s)", cfg.GetProviderType()), nil
}

func (vr *SimpleValidatorRegistry) createAuthValidator(ctx context.Context, coreValidator, authProvider interface{}, cfg AuthConfigInterface) (interface{}, error) {
	return fmt.Sprintf("AuthValidator(core=%v, provider=%v)", coreValidator, authProvider), nil
}

func (vr *SimpleValidatorRegistry) createCacheValidator(ctx context.Context, coreValidator interface{}, cfg CacheConfigInterface) (interface{}, error) {
	return fmt.Sprintf("CacheValidator(core=%v, l1_size=%d)", coreValidator, cfg.GetL1Size()), nil
}

func (vr *SimpleValidatorRegistry) createDistributedValidator(ctx context.Context, coreValidator interface{}, cfg DistributedConfigInterface) (interface{}, error) {
	return fmt.Sprintf("DistributedValidator(core=%v, node=%s)", coreValidator, cfg.GetNodeID()), nil
}

func (vr *SimpleValidatorRegistry) createAnalyticsService(ctx context.Context, cfg AnalyticsConfigInterface) (interface{}, error) {
	return fmt.Sprintf("AnalyticsService(metrics=%t, tracing=%t)", cfg.IsMetricsEnabled(), cfg.IsTracingEnabled()), nil
}

func (vr *SimpleValidatorRegistry) createSecurityValidator(ctx context.Context, cfg SecurityConfigInterface) (interface{}, error) {
	return fmt.Sprintf("SecurityValidator(sanitization=%t, rate_limiting=%t)", cfg.IsInputSanitizationEnabled(), cfg.IsRateLimitingEnabled()), nil
}

func (vr *SimpleValidatorRegistry) createMainValidator(ctx context.Context, components map[string]interface{}) (interface{}, error) {
	return fmt.Sprintf("MainValidator(components=%v)", components), nil
}

// Public methods for accessing services

// GetMainValidator returns the main composed validator
func (vr *SimpleValidatorRegistry) GetMainValidator(ctx context.Context) (interface{}, error) {
	return vr.container.Get(ctx, "main_validator")
}

// GetCoreValidator returns the core validator
func (vr *SimpleValidatorRegistry) GetCoreValidator(ctx context.Context) (interface{}, error) {
	return vr.container.Get(ctx, "core_validator")
}

// GetService returns a specific service by name
func (vr *SimpleValidatorRegistry) GetService(ctx context.Context, name string) (interface{}, error) {
	return vr.container.Get(ctx, name)
}

// GetServicesByTag returns all services with a specific tag
func (vr *SimpleValidatorRegistry) GetServicesByTag(ctx context.Context, tag string) ([]interface{}, error) {
	return vr.container.GetByTag(ctx, tag)
}

// GetServicesByInterface returns all services implementing a specific interface
func (vr *SimpleValidatorRegistry) GetServicesByInterface(ctx context.Context, interfaceType reflect.Type) ([]interface{}, error) {
	return vr.container.GetByInterface(ctx, interfaceType)
}

// Validate validates the registry configuration
func (vr *SimpleValidatorRegistry) Validate() error {
	return vr.container.Validate()
}

// GetServiceNames returns all registered service names
func (vr *SimpleValidatorRegistry) GetServiceNames() []string {
	return vr.container.GetServiceNames()
}

// AddInterceptor adds an interceptor to the container
func (vr *SimpleValidatorRegistry) AddInterceptor(interceptor Interceptor) {
	vr.container.AddInterceptor(interceptor)
}

// CreateScope creates a new scope for scoped services
func (vr *SimpleValidatorRegistry) CreateScope() *Scope {
	return vr.container.CreateScope()
}

// Clear clears all cached instances
func (vr *SimpleValidatorRegistry) Clear() {
	vr.container.Clear()
}