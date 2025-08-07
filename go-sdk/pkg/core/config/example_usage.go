package config

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ExampleBasicUsage demonstrates basic usage of the unified configuration system
func ExampleBasicUsage() {
	// Create a simple validator configuration
	config, err := NewValidatorBuilder().
		WithValidationLevel(ValidationLevelPermissive).
		WithEnvironment("example", "1.0.0", "example-app").
		Build()

	if err != nil {
		log.Fatal("Failed to build configuration:", err)
	}

	// Create a validator factory
	factory := NewValidatorFactory(config)

	// Create the main validator
	ctx := context.Background()
	validator, err := factory.CreateValidator(ctx)
	if err != nil {
		log.Fatal("Failed to create validator:", err)
	}

	fmt.Printf("Created validator: %v\n", validator)

	// Output: Created validator: MainValidator(components=map[core_validator:CoreValidator(level=permissive)])
}

// ExampleWithAuthentication demonstrates usage with authentication enabled
func ExampleWithAuthentication() {
	config, err := NewValidatorBuilder().
		WithAuthenticationEnabled(true).
		WithBasicAuth().
		WithRBAC(true, []string{"user"}).
		Build()

	if err != nil {
		log.Fatal("Failed to build configuration:", err)
	}

	factory := NewValidatorFactory(config)

	ctx := context.Background()
	validator, err := factory.CreateValidator(ctx)
	if err != nil {
		log.Fatal("Failed to create validator:", err)
	}

	fmt.Printf("Created authenticated validator: %v\n", validator)

	// Also get the auth validator specifically
	authValidator, err := factory.CreateAuthValidator(ctx)
	if err != nil {
		log.Fatal("Failed to create auth validator:", err)
	}

	fmt.Printf("Auth validator: %v\n", authValidator)
}

// ExampleWithCaching demonstrates usage with caching enabled
func ExampleWithCaching() {
	config, err := NewValidatorBuilder().
		WithCacheEnabled(true).
		WithL1Cache(10000, 5*time.Minute).
		WithCacheCompression(true, "gzip", 6).
		Build()

	if err != nil {
		log.Fatal("Failed to build configuration:", err)
	}

	factory := NewValidatorFactory(config)

	ctx := context.Background()
	validator, err := factory.CreateValidator(ctx)
	if err != nil {
		log.Fatal("Failed to create validator:", err)
	}

	fmt.Printf("Created cached validator: %v\n", validator)
}

// ExampleFullConfiguration demonstrates a full configuration with all features
func ExampleFullConfiguration() {
	config, err := NewValidatorBuilder().
		WithValidationLevel(ValidationLevelStrict).
		WithStrictValidation(true).
		WithAuthenticationEnabled(true).
		WithJWTAuth("secret-key", "example-issuer").
		WithCacheEnabled(true).
		WithL1Cache(50000, 10*time.Minute).
		WithDistributedEnabled(true).
		WithDistributedNode("node-1", "leader", ":8080").
		WithMetrics(true, "prometheus", 30*time.Second).
		WithTracing(true, "jaeger", 0.1).
		WithLogging(true, "info", "json", "stdout").
		WithSecurityEnabled(true).
		WithRateLimiting(true, 1000, time.Minute, 100).
		WithEnvironment("production", "1.0.0", "example-app").
		Build()

	if err != nil {
		log.Fatal("Failed to build configuration:", err)
	}

	factory := NewValidatorFactory(config)

	ctx := context.Background()
	validator, err := factory.CreateValidator(ctx)
	if err != nil {
		log.Fatal("Failed to create validator:", err)
	}

	fmt.Printf("Created full-featured validator: %v\n", validator)

	// Get services by tag
	validators, err := factory.GetServicesByTag(ctx, "validator")
	if err != nil {
		log.Fatal("Failed to get validators:", err)
	}

	fmt.Printf("Found %d validator services\n", len(validators))
}

// ExamplePresets demonstrates using configuration presets
func ExamplePresets() {
	// List available presets
	presets := ListAvailablePresets()
	fmt.Printf("Available presets: %v\n", presets)

	// Create validator from development preset
	devFactory, err := CreateValidatorFromPreset("development")
	if err != nil {
		log.Fatal("Failed to create development validator:", err)
	}

	ctx := context.Background()
	devValidator, err := devFactory.CreateValidator(ctx)
	if err != nil {
		log.Fatal("Failed to create dev validator:", err)
	}

	fmt.Printf("Development validator: %v\n", devValidator)

	// Create validator from production preset
	prodFactory, err := CreateValidatorFromPreset("production")
	if err != nil {
		log.Fatal("Failed to create production validator:", err)
	}

	prodValidator, err := prodFactory.CreateValidator(ctx)
	if err != nil {
		log.Fatal("Failed to create prod validator:", err)
	}

	fmt.Printf("Production validator: %v\n", prodValidator)
}

// ExampleLegacyMigration demonstrates migrating from legacy configurations
func ExampleLegacyMigration() {
	// Legacy auth configuration
	legacyAuth := &AuthConfigCompat{
		Enabled:           true,
		RequireAuth:       false,
		AllowAnonymous:    true,
		TokenExpiration:   24 * time.Hour,
		RefreshEnabled:    true,
		RefreshExpiration: 7 * 24 * time.Hour,
		ProviderConfig:    make(map[string]interface{}),
	}

	// Legacy cache configuration
	legacyCache := &CacheValidatorConfigCompat{
		L1Size:             10000,
		L1TTL:              5 * time.Minute,
		L2TTL:              30 * time.Minute,
		L2Enabled:          false,
		CompressionEnabled: true,
		CompressionLevel:   6,
		MetricsEnabled:     true,
	}

	// Migrate to unified configuration
	factory := NewValidatorFactoryFromLegacy(legacyAuth, legacyCache, nil)

	ctx := context.Background()
	validator, err := factory.CreateValidator(ctx)
	if err != nil {
		log.Fatal("Failed to create migrated validator:", err)
	}

	fmt.Printf("Migrated validator: %v\n", validator)

	// Show that we can still extract legacy config
	config := factory.GetConfiguration()
	extractedAuth := ExtractLegacyAuthConfig(config)

	fmt.Printf("Extracted auth config enabled: %t\n", extractedAuth.Enabled)
	fmt.Printf("Extracted auth config token expiration: %v\n", extractedAuth.TokenExpiration)
}

// ExampleDependencyInjection demonstrates advanced DI usage
func ExampleDependencyInjection() {
	config, err := NewValidatorBuilder().
		WithAuthenticationEnabled(true).
		WithCacheEnabled(true).
		Build()

	if err != nil {
		log.Fatal("Failed to build configuration:", err)
	}

	factory := NewValidatorFactory(config)

	// Add interceptor to monitor service creation
	factory.AddInterceptor(&LoggingInterceptor{})

	ctx := context.Background()

	// Get core validator
	coreValidator, err := factory.CreateCoreValidator(ctx)
	if err != nil {
		log.Fatal("Failed to create core validator:", err)
	}

	fmt.Printf("Core validator: %v\n", coreValidator)

	// Get auth validator
	authValidator, err := factory.CreateAuthValidator(ctx)
	if err != nil {
		log.Fatal("Failed to create auth validator:", err)
	}

	fmt.Printf("Auth validator: %v\n", authValidator)

	// Get all validators
	validators, err := factory.GetServicesByTag(ctx, "validator")
	if err != nil {
		log.Fatal("Failed to get validators:", err)
	}

	fmt.Printf("Total validators: %d\n", len(validators))

	// Get service names
	serviceNames := factory.GetServiceNames()
	fmt.Printf("Available services: %v\n", serviceNames)
}

// Simple logging interceptor for demonstration
type LoggingInterceptor struct{}

func (li *LoggingInterceptor) BeforeCreate(ctx context.Context, serviceName string) error {
	fmt.Printf("Creating service: %s\n", serviceName)
	return nil
}

func (li *LoggingInterceptor) AfterCreate(ctx context.Context, serviceName string, instance interface{}) error {
	fmt.Printf("Created service: %s\n", serviceName)
	return nil
}
