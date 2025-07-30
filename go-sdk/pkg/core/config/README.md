# Unified Configuration + Dependency Injection System

This package implements a unified configuration system with dependency injection to resolve circular dependencies and provide a consistent configuration pattern across all validation components.

## Overview

The unified configuration system addresses several issues identified in the PR review:
- Multiple inconsistent configuration patterns (AuthConfig, CacheValidatorConfig, DistributedValidatorConfig)
- Circular dependency risks between auth, cache, and distributed packages
- Lack of a consistent builder pattern for complex configurations
- No dependency injection container to manage component lifecycles

## Key Components

### 1. Unified Configuration Structure

The `ValidatorConfig` struct serves as the single source of truth for all validation configuration:

```go
type ValidatorConfig struct {
    Core        *CoreValidationConfig         `json:"core"`
    Auth        *AuthValidationConfig         `json:"auth,omitempty"`
    Cache       *CacheValidationConfig        `json:"cache,omitempty"`
    Distributed *DistributedValidationConfig  `json:"distributed,omitempty"`
    Analytics   *AnalyticsValidationConfig    `json:"analytics,omitempty"`
    Security    *SecurityValidationConfig     `json:"security,omitempty"`
    Features    *FeatureFlags                 `json:"features,omitempty"`
    Global      *GlobalSettings               `json:"global,omitempty"`
}
```

### 2. Builder Pattern

The `ValidatorBuilder` provides a fluent API for constructing configurations:

```go
config := NewValidatorBuilder().
    WithValidationLevel(ValidationLevelStrict).
    WithAuthenticationEnabled(true).
    WithBasicAuth().
    WithCacheEnabled(true).
    WithL1Cache(10000, 5*time.Minute).
    WithDistributedEnabled(true).
    WithDistributedNode("node-1", "leader", ":8080").
    Build()
```

### 3. Dependency Injection Container

The DI container manages service creation and resolves dependencies:

```go
container := di.NewContainer()
container.RegisterSingleton("validator", factoryFunc)
validator, err := container.Get(ctx, "validator")
```

### 4. Validator Registry

The `ValidatorRegistry` uses the DI container to manage validator components and their dependencies:

```go
registry := di.NewValidatorRegistry(config)
validator, err := registry.GetMainValidator(ctx)
```

## Usage Examples

### Basic Usage

```go
// Create a simple validator with default configuration
factory, err := CreateSimpleValidator()
if err != nil {
    log.Fatal(err)
}

validator, err := factory.CreateValidator(ctx)
if err != nil {
    log.Fatal(err)
}
```

### Using the Builder Pattern

```go
// Build a custom configuration
config, err := NewValidatorBuilder().
    WithValidationLevel(ValidationLevelPermissive).
    WithAuthenticationEnabled(true).
    WithJWTAuth("secret", "issuer").
    WithCacheEnabled(true).
    WithRedisCache("localhost:6379", "", 0, 30*time.Minute).
    WithMetrics(true, "prometheus", 30*time.Second).
    WithLogging(true, "info", "json", "stdout").
    Build()

if err != nil {
    log.Fatal(err)
}

factory := NewValidatorFactory(config)
validator, err := factory.CreateValidator(ctx)
```

### Environment-Specific Configurations

```go
// Development configuration
devFactory, err := CreateDevelopmentValidator()

// Production configuration  
prodFactory, err := CreateProductionValidator()

// Testing configuration
testFactory, err := CreateTestingValidator()
```

### Using Configuration Presets

```go
// Create from preset
factory, err := CreateValidatorFromPreset("production")
if err != nil {
    log.Fatal(err)
}

// List available presets
presets := ListAvailablePresets()
fmt.Println("Available presets:", presets)
```

### Legacy Compatibility

```go
// Migrate from legacy configuration
legacyAuth := &AuthConfigCompat{
    Enabled:         true,
    RequireAuth:     false,
    TokenExpiration: 24 * time.Hour,
}

factory := NewValidatorFactoryFromLegacy(legacyAuth, nil, nil)
validator, err := factory.CreateValidator(ctx)
```

### Advanced DI Usage

```go
// Custom service registration
registry := di.NewValidatorRegistry(config)

// Add custom interceptor
registry.AddInterceptor(&LoggingInterceptor{
    Logger: logger,
})

// Get services by tag
validators, err := registry.GetServicesByTag(ctx, "validator")

// Create scoped services
scope := registry.CreateScope()
defer scope.Dispose()

scopedValidator, err := scope.Get(ctx, "validator")
```

## Configuration Reference

### Core Validation

```go
WithCoreValidation(func(config *CoreValidationConfig) {
    config.Level = ValidationLevelStrict
    config.ValidationTimeout = 30 * time.Second
    config.MaxConcurrentValidations = 100
})
```

### Authentication

```go
// Basic Auth
WithBasicAuth()

// JWT Auth
WithJWTAuth("secret", "issuer")

// OAuth
WithOAuthAuth("client_id", "client_secret", "auth_url", "token_url")

// RBAC
WithRBAC(true, []string{"user", "admin"})
```

### Caching

```go
// L1 Cache (in-memory)
WithL1Cache(10000, 5*time.Minute)

// L2 Cache (Redis)
WithRedisCache("localhost:6379", "", 0, 30*time.Minute)

// Cache compression
WithCacheCompression(true, "gzip", 6)
```

### Distributed Validation

```go
// Basic distributed setup
WithDistributedNode("node-1", "leader", ":8080")

// Consensus configuration
WithConsensus("raft", 5*time.Second, 3, 10)

// TLS configuration
WithTLS(true, "cert.pem", "key.pem", "ca.pem", true)
```

### Analytics & Monitoring

```go
// Prometheus metrics
WithPrometheusMetrics(true, "/metrics", 9090)

// Jaeger tracing
WithJaegerTracing(true, "http://jaeger:14268/api/traces", 0.1)

// Logging
WithLogging(true, "info", "json", "stdout")
```

### Security

```go
// Input sanitization
WithInputSanitization(true, 1024*1024, []string{"p", "br"})

// Rate limiting
WithRateLimiting(true, 1000, time.Minute, 100)

// Encryption
WithEncryption(true, "AES256", "secret-key")
```

## Migration Guide

### From Legacy AuthConfig

```go
// Old way
authConfig := &auth.AuthConfig{
    Enabled:         true,
    RequireAuth:     false,
    TokenExpiration: 24 * time.Hour,
}

// New way
config := NewValidatorBuilder().
    WithAuthenticationEnabled(true).
    WithAuthentication(func(auth *AuthValidationConfig) {
        auth.RequireAuth = false
        auth.TokenExpiration = 24 * time.Hour
    }).
    Build()
```

### From Legacy CacheValidatorConfig

```go
// Old way
cacheConfig := &cache.CacheValidatorConfig{
    L1Size: 10000,
    L1TTL:  5 * time.Minute,
    L2Enabled: true,
}

// New way
config := NewValidatorBuilder().
    WithCacheEnabled(true).
    WithL1Cache(10000, 5*time.Minute).
    WithL2Cache("redis", 30*time.Minute, redisConfig).
    Build()
```

### From Legacy DistributedValidatorConfig

```go
// Old way
distributedConfig := &distributed.DistributedValidatorConfig{
    NodeID:            "node-1",
    ValidationTimeout: 5 * time.Second,
    HeartbeatInterval: 1 * time.Second,
}

// New way
config := NewValidatorBuilder().
    WithDistributedEnabled(true).
    WithDistributedNode("node-1", "leader", ":8080").
    WithConsensus("majority", 5*time.Second, 1, 10).
    Build()
```

## Best Practices

### 1. Use Environment-Specific Configurations

```go
var factory *ValidatorFactory
switch os.Getenv("ENVIRONMENT") {
case "production":
    factory, _ = CreateProductionValidator()
case "development":
    factory, _ = CreateDevelopmentValidator()
case "testing":
    factory, _ = CreateTestingValidator()
default:
    factory, _ = CreateSimpleValidator()
}
```

### 2. Validate Configurations Early

```go
config, err := NewValidatorBuilder().
    WithAuthenticationEnabled(true).
    WithCacheEnabled(true).
    Build()

if err != nil {
    log.Fatalf("Configuration validation failed: %v", err)
}
```

### 3. Use Dependency Injection for Testing

```go
func TestValidator(t *testing.T) {
    // Create test configuration
    config := NewValidatorBuilder().ForTesting().MustBuild()
    factory := NewValidatorFactory(config)
    
    // Create scope for test
    scope := factory.CreateScope()
    defer scope.Dispose()
    
    validator, err := scope.Get(ctx, "main_validator")
    // ... test validator
}
```

### 4. Monitor Service Creation

```go
factory := NewValidatorFactory(config)

// Add logging interceptor
factory.AddInterceptor(&di.LoggingInterceptor{
    Logger: logger,
})

// Add timing interceptor
factory.AddInterceptor(&di.TimingInterceptor{
    OnTiming: func(service string, duration int64) {
        metrics.RecordServiceCreationTime(service, duration)
    },
})
```

## Architecture Benefits

### 1. Circular Dependency Resolution

The DI container automatically resolves dependencies in the correct order and detects circular dependencies at registration time.

### 2. Consistent Configuration Pattern

All components use the same configuration structure and builder pattern, making the codebase more maintainable.

### 3. Backward Compatibility

Legacy configurations are automatically migrated to the new format, ensuring existing code continues to work.

### 4. Testability

The DI container makes it easy to inject mock dependencies for testing.

### 5. Flexibility

The builder pattern and presets make it easy to create different configurations for different environments.

## Performance Considerations

- Services are created lazily when first requested
- Singletons are cached for subsequent requests
- Scoped services are cleaned up automatically
- Configuration validation happens at build time, not runtime

## Future Enhancements

1. **Configuration Hot Reloading**: Support for updating configuration without restarting
2. **Configuration Validation**: Enhanced validation with detailed error messages
3. **Configuration Templating**: Support for configuration templates and inheritance
4. **Metrics Integration**: Built-in metrics for configuration usage and performance
5. **Configuration Discovery**: Automatic discovery of available configuration options

## Contributing

When adding new configuration options:

1. Add fields to the appropriate config struct
2. Update the builder with corresponding methods
3. Update the default configurations
4. Add validation rules
5. Update tests and documentation
6. Consider backward compatibility impact