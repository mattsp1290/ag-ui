package config

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/di"
)

func TestUnifiedConfigurationBasic(t *testing.T) {
	// Test basic configuration creation
	config := DefaultValidatorConfig()

	if config.Core == nil {
		t.Error("Core configuration should not be nil")
	}

	if config.Global == nil {
		t.Error("Global configuration should not be nil")
	}

	// Test that default values are set correctly
	if config.Core.Level != ValidationLevelStrict {
		t.Errorf("Expected validation level to be strict, got %v", config.Core.Level)
	}

	if config.Global.Environment != "development" {
		t.Errorf("Expected environment to be development, got %s", config.Global.Environment)
	}
}

func TestValidatorBuilder(t *testing.T) {
	// Test builder pattern
	config, err := NewValidatorBuilder().
		WithValidationLevel(ValidationLevelPermissive).
		WithAuthenticationEnabled(true).
		WithBasicAuth().
		WithCacheEnabled(true).
		WithL1Cache(1000, 5*time.Minute).
		WithEnvironment("testing", "1.0.0", "test-app").
		Build()

	if err != nil {
		t.Fatalf("Failed to build configuration: %v", err)
	}

	// Verify configuration was set correctly
	if config.Core.Level != ValidationLevelPermissive {
		t.Errorf("Expected validation level to be permissive, got %v", config.Core.Level)
	}

	if !config.Auth.Enabled {
		t.Error("Authentication should be enabled")
	}

	if config.Auth.ProviderType != "basic" {
		t.Errorf("Expected auth provider to be basic, got %s", config.Auth.ProviderType)
	}

	if !config.Cache.Enabled {
		t.Error("Cache should be enabled")
	}

	if config.Cache.L1Size != 1000 {
		t.Errorf("Expected L1 cache size to be 1000, got %d", config.Cache.L1Size)
	}

	if config.Global.Environment != "testing" {
		t.Errorf("Expected environment to be testing, got %s", config.Global.Environment)
	}
}

func TestBuilderValidation(t *testing.T) {
	// Test validation errors
	_, err := NewValidatorBuilder().
		WithValidationTimeout(-1 * time.Second). // Invalid timeout
		Build()

	if err == nil {
		t.Error("Expected validation error for negative timeout")
	}

	// Test authentication validation
	_, err = NewValidatorBuilder().
		WithAuthenticationEnabled(true).
		WithAuthentication(func(config *AuthValidationConfig) {
			config.TokenExpiration = -1 * time.Hour // Invalid expiration
		}).
		Build()

	if err == nil {
		t.Error("Expected validation error for negative token expiration")
	}
}

func TestDependencyInjection(t *testing.T) {
	// Test basic DI container functionality
	container := di.NewContainer()

	// Register a simple service
	container.RegisterSingleton("test_service", func(ctx context.Context, c *di.Container) (interface{}, error) {
		return "test_value", nil
	})

	// Get the service
	service, err := container.Get(context.Background(), "test_service")
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if service != "test_value" {
		t.Errorf("Expected service value to be 'test_value', got %v", service)
	}

	// Test singleton behavior
	service2, err := container.Get(context.Background(), "test_service")
	if err != nil {
		t.Fatalf("Failed to get service second time: %v", err)
	}

	// Should be the same instance for singletons
	if service != service2 {
		t.Error("Singleton services should return the same instance")
	}
}

func TestDependencyInjectionWithDependencies(t *testing.T) {
	container := di.NewContainer()

	// Register dependency
	container.RegisterSingleton("dependency", func(ctx context.Context, c *di.Container) (interface{}, error) {
		return "dependency_value", nil
	})

	// Register service that depends on the dependency
	container.RegisterSingleton("main_service", func(ctx context.Context, c *di.Container) (interface{}, error) {
		dep, err := c.Get(ctx, "dependency")
		if err != nil {
			return nil, err
		}
		return "main_" + dep.(string), nil
	}).WithDependencies("dependency")

	// Get the main service
	service, err := container.Get(context.Background(), "main_service")
	if err != nil {
		t.Fatalf("Failed to get main service: %v", err)
	}

	expected := "main_dependency_value"
	if service != expected {
		t.Errorf("Expected service value to be '%s', got %v", expected, service)
	}
}

func TestCircularDependencyDetection(t *testing.T) {
	container := di.NewContainer()

	// Register services with circular dependencies
	container.RegisterSingleton("service_a", func(ctx context.Context, c *di.Container) (interface{}, error) {
		_, err := c.Get(ctx, "service_b")
		if err != nil {
			return nil, err
		}
		return "service_a", nil
	}).WithDependencies("service_b")

	container.RegisterSingleton("service_b", func(ctx context.Context, c *di.Container) (interface{}, error) {
		_, err := c.Get(ctx, "service_a")
		if err != nil {
			return nil, err
		}
		return "service_b", nil
	}).WithDependencies("service_a")

	// This should detect the circular dependency
	_, err := container.Get(context.Background(), "service_a")
	if err == nil {
		t.Error("Expected circular dependency error")
	}

	// Check that error message contains circular dependency detection
	if !strings.Contains(err.Error(), "circular dependency detected for service: service_a") {
		t.Errorf("Expected circular dependency error for service_a, got: %v", err)
	}
}

func TestValidatorRegistry(t *testing.T) {
	// Test validator registry functionality
	config := NewValidatorBuilder().
		WithAuthenticationEnabled(true).
		WithCacheEnabled(true).
		MustBuild()

	registry := di.NewSimpleValidatorRegistry(config)

	// Test that services are registered
	serviceNames := registry.GetServiceNames()
	expectedServices := []string{"config", "core_validator", "auth_provider", "auth_validator", "cache_validator", "main_validator"}

	for _, expected := range expectedServices {
		found := false
		for _, name := range serviceNames {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected service %s to be registered", expected)
		}
	}

	// Test getting the main validator
	validator, err := registry.GetMainValidator(context.Background())
	if err != nil {
		t.Fatalf("Failed to get main validator: %v", err)
	}

	if validator == nil {
		t.Error("Main validator should not be nil")
	}
}

func TestValidatorFactory(t *testing.T) {
	// Test validator factory
	config := NewValidatorBuilder().
		WithValidationLevel(ValidationLevelTesting).
		WithAuthenticationEnabled(false).
		MustBuild()

	factory := NewValidatorFactory(config)

	// Test creating a validator
	validator, err := factory.CreateValidator(context.Background())
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	if validator == nil {
		t.Error("Validator should not be nil")
	}

	// Test getting core validator
	coreValidator, err := factory.CreateCoreValidator(context.Background())
	if err != nil {
		t.Fatalf("Failed to create core validator: %v", err)
	}

	if coreValidator == nil {
		t.Error("Core validator should not be nil")
	}
}

func TestLegacyCompatibility(t *testing.T) {
	// Test backward compatibility with legacy configurations
	legacyAuth := &AuthConfigCompat{
		Enabled:           true,
		RequireAuth:       false,
		AllowAnonymous:    true,
		TokenExpiration:   24 * time.Hour,
		RefreshEnabled:    true,
		RefreshExpiration: 7 * 24 * time.Hour,
		ProviderConfig:    make(map[string]interface{}),
	}

	// Convert to unified config
	unifiedAuth := legacyAuth.ToUnifiedConfig()

	if !unifiedAuth.Enabled {
		t.Error("Unified auth config should be enabled")
	}

	if unifiedAuth.TokenExpiration != 24*time.Hour {
		t.Errorf("Expected token expiration to be 24h, got %v", unifiedAuth.TokenExpiration)
	}

	// Test migration
	migrator := NewConfigMigrator()
	config := migrator.MigrateFromLegacy(legacyAuth, nil, nil)

	if config.Auth == nil {
		t.Error("Migrated config should have auth configuration")
	}

	if !config.Auth.Enabled {
		t.Error("Migrated auth config should be enabled")
	}

	// Test round-trip conversion
	legacyBack := &AuthConfigCompat{}
	legacyBack.FromUnifiedConfig(unifiedAuth)

	if legacyBack.Enabled != legacyAuth.Enabled {
		t.Error("Round-trip conversion should preserve enabled flag")
	}

	if legacyBack.TokenExpiration != legacyAuth.TokenExpiration {
		t.Error("Round-trip conversion should preserve token expiration")
	}
}

func TestConfigurationPresets(t *testing.T) {
	// Test configuration presets
	presets := GetConfigurationPresets()

	if len(presets) == 0 {
		t.Error("Should have at least one configuration preset")
	}

	// Test specific presets
	presetNames := ListAvailablePresets()
	expectedPresets := []string{"minimal", "development", "production", "testing", "secure", "performance", "distributed"}

	for _, expected := range expectedPresets {
		found := false
		for _, name := range presetNames {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected preset %s to be available", expected)
		}
	}

	// Test creating validator from preset
	factory, err := CreateValidatorFromPreset("development")
	if err != nil {
		t.Fatalf("Failed to create validator from development preset: %v", err)
	}

	if factory == nil {
		t.Error("Factory should not be nil")
	}

	// Test the development preset configuration
	config := factory.GetConfiguration()
	if config.Core.Level != ValidationLevelDevelopment {
		t.Errorf("Development preset should use development validation level, got %v", config.Core.Level)
	}
}

func TestFactoryRegistry(t *testing.T) {
	// Test factory registry
	registry := NewValidatorFactoryRegistry()

	config := NewValidatorBuilder().
		WithValidationLevel(ValidationLevelTesting).
		MustBuild()

	factory := NewValidatorFactory(config)

	// Register factory
	registry.Register("test_factory", factory)

	// Retrieve factory
	retrievedFactory, exists := registry.Get("test_factory")
	if !exists {
		t.Error("Factory should exist in registry")
	}

	if retrievedFactory != factory {
		t.Error("Retrieved factory should be the same instance")
	}

	// Test creating validator from registry
	validator, err := registry.CreateValidator(context.Background(), "test_factory")
	if err != nil {
		t.Fatalf("Failed to create validator from registry: %v", err)
	}

	if validator == nil {
		t.Error("Validator should not be nil")
	}

	// Test listing factories
	names := registry.List()
	if len(names) != 1 || names[0] != "test_factory" {
		t.Errorf("Expected registry to contain 'test_factory', got %v", names)
	}
}

func TestInterceptors(t *testing.T) {
	// Test interceptors
	container := di.NewContainer()

	// Track interceptor calls
	beforeCalls := make([]string, 0)
	afterCalls := make([]string, 0)

	interceptor := &testInterceptor{
		beforeFunc: func(serviceName string) {
			beforeCalls = append(beforeCalls, serviceName)
		},
		afterFunc: func(serviceName string, instance interface{}) {
			afterCalls = append(afterCalls, serviceName)
		},
	}

	container.AddInterceptor(interceptor)

	// Register a service
	container.RegisterSingleton("test_service", func(ctx context.Context, c *di.Container) (interface{}, error) {
		return "test_value", nil
	})

	// Get the service (should trigger interceptors)
	_, err := container.Get(context.Background(), "test_service")
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	// Check that interceptors were called
	if len(beforeCalls) != 1 || beforeCalls[0] != "test_service" {
		t.Errorf("Expected one before call for 'test_service', got %v", beforeCalls)
	}

	if len(afterCalls) != 1 || afterCalls[0] != "test_service" {
		t.Errorf("Expected one after call for 'test_service', got %v", afterCalls)
	}
}

func TestScopes(t *testing.T) {
	// Test scoped services
	container := di.NewContainer()

	// Register a scoped service
	container.RegisterScoped("scoped_service", func(ctx context.Context, c *di.Container) (interface{}, error) {
		return "scoped_value", nil
	})

	// Create two scopes
	scope1 := container.CreateScope()
	scope2 := container.CreateScope()

	// Get service from both scopes
	service1, err := scope1.Get(context.Background(), "scoped_service")
	if err != nil {
		t.Fatalf("Failed to get service from scope1: %v", err)
	}

	service2, err := scope2.Get(context.Background(), "scoped_service")
	if err != nil {
		t.Fatalf("Failed to get service from scope2: %v", err)
	}

	// Services should have the same value but be different instances (in a real implementation)
	if service1 != "scoped_value" || service2 != "scoped_value" {
		t.Error("Scoped services should have the correct value")
	}

	// Test scope disposal
	scope1.Dispose()
	scope2.Dispose()
}

// Test helper types

type testInterceptor struct {
	beforeFunc func(serviceName string)
	afterFunc  func(serviceName string, instance interface{})
}

func (ti *testInterceptor) BeforeCreate(ctx context.Context, serviceName string) error {
	if ti.beforeFunc != nil {
		ti.beforeFunc(serviceName)
	}
	return nil
}

func (ti *testInterceptor) AfterCreate(ctx context.Context, serviceName string, instance interface{}) error {
	if ti.afterFunc != nil {
		ti.afterFunc(serviceName, instance)
	}
	return nil
}

// Benchmark tests

func BenchmarkValidatorCreation(b *testing.B) {
	config := NewValidatorBuilder().
		WithValidationLevel(ValidationLevelPermissive).
		MustBuild()

	factory := NewValidatorFactory(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := factory.CreateValidator(context.Background())
		if err != nil {
			b.Fatalf("Failed to create validator: %v", err)
		}
	}
}

func BenchmarkConfigurationBuilding(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewValidatorBuilder().
			WithValidationLevel(ValidationLevelPermissive).
			WithAuthenticationEnabled(true).
			WithCacheEnabled(true).
			WithDistributedEnabled(false).
			Build()
		if err != nil {
			b.Fatalf("Failed to build configuration: %v", err)
		}
	}
}

func BenchmarkDependencyInjection(b *testing.B) {
	container := di.NewContainer()

	// Register multiple services
	for i := 0; i < 10; i++ {
		serviceName := "service_" + string(rune(i))
		container.RegisterSingleton(serviceName, func(ctx context.Context, c *di.Container) (interface{}, error) {
			return serviceName + "_value", nil
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := container.Get(context.Background(), "service_5")
		if err != nil {
			b.Fatalf("Failed to get service: %v", err)
		}
	}
}
