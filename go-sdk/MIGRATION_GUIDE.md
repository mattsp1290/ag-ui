# Event Validation System Migration Guide

This guide provides comprehensive instructions for migrating from the base event validation system to the enhanced event validation system in the AG-UI Go SDK.

## Table of Contents

1. [Overview](#overview)
2. [Breaking Changes](#breaking-changes)
3. [Step-by-Step Migration](#step-by-step-migration)
4. [Rollback Procedures](#rollback-procedures)
5. [Performance Considerations](#performance-considerations)
6. [Configuration Migration Examples](#configuration-migration-examples)

## Overview

The enhanced event validation system introduces several enterprise-grade features:

- **Distributed Validation**: Multi-node consensus-based validation
- **Performance Optimization**: Parallel validation, caching, and hot-path optimization
- **Enterprise Monitoring**: Prometheus, OpenTelemetry, and Grafana integration
- **Advanced Security**: Encryption validation and authentication support
- **Multi-level Caching**: L1 (in-memory) and L2 (distributed) caching

### Feature Comparison

| Feature | Base System | Enhanced System |
|---------|------------|-----------------|
| Validation Rules | Basic rules | Extensible rule engine with severity levels |
| Performance | Sequential | Parallel with configurable worker pools |
| Caching | None | Multi-level with TTL and invalidation |
| Monitoring | Basic metrics | Full observability stack |
| Distribution | Single node | Multi-node with consensus |
| Security | Basic validation | Encryption and authentication |

## Breaking Changes

### 1. API Changes

#### Validator Creation
```go
// Old
validator := events.NewValidator(config)

// New - EventValidator with enhanced features
validator := events.NewEventValidator(config)
```

#### Validation Result Structure
```go
// Old
err := validator.ValidateEvent(ctx, event)
if err != nil {
    // Handle error
}

// New - Returns structured ValidationResult
result := validator.ValidateEvent(ctx, event)
if result.HasErrors() {
    for _, err := range result.Errors {
        // Handle validation error with severity and context
    }
}
```

#### Configuration Changes
```go
// Old
config := &events.ValidationConfig{
    Level: events.ValidationStrict,
    SkipTimestampValidation: false,
}

// New - Extended configuration options
config := &events.ValidationConfig{
    Level: events.ValidationStrict,
    SkipTimestampValidation: false,
    CustomValidators: []events.CustomValidator{
        // Add custom validators
    },
}

// Additional performance configuration
perfConfig := &events.PerformanceConfig{
    Mode: events.BalancedMode,
    CacheTTL: 5 * time.Minute,
    EnableParallelExecution: true,
    MaxGoroutines: runtime.NumCPU() * 2,
}
```

### 2. Import Changes

```go
// Old
import "github.com/ag-ui/go-sdk/pkg/core/events"

// New - Additional imports for enhanced features
import (
    "github.com/ag-ui/go-sdk/pkg/core/events"
    "github.com/ag-ui/go-sdk/pkg/core/events/distributed"
    "github.com/ag-ui/go-sdk/pkg/core/events/monitoring"
    "github.com/ag-ui/go-sdk/pkg/core/events/cache"
    "github.com/ag-ui/go-sdk/pkg/core/events/security"
)
```

### 3. Interface Changes

The `ValidationRule` interface now includes severity and enable/disable functionality:

```go
// Old - Simple validation function
type CustomValidator func(ctx context.Context, event Event) error

// New - Full ValidationRule interface
type ValidationRule interface {
    ID() string
    Description() string
    Validate(event Event, context *ValidationContext) *ValidationResult
    IsEnabled() bool
    SetEnabled(enabled bool)
    GetSeverity() ValidationSeverity
    SetSeverity(severity ValidationSeverity)
}
```

## Step-by-Step Migration

### Step 1: Update Dependencies

```bash
# Update to the latest version
go get -u github.com/ag-ui/go-sdk@enhanced-event-validation

# Update dependencies
go mod tidy
```

### Step 2: Update Validator Creation

Replace all instances of `NewValidator` with `NewEventValidator`:

```go
// Before
validator := events.NewValidator(events.DefaultValidationConfig())

// After
validator := events.NewEventValidator(events.DefaultValidationConfig())
```

### Step 3: Update Validation Calls

Update validation calls to handle the new result structure:

```go
// Before
err := validator.ValidateEvent(ctx, event)
if err != nil {
    log.Printf("Validation failed: %v", err)
    return err
}

// After
result := validator.ValidateEvent(ctx, event)
if !result.IsValid {
    for _, err := range result.Errors {
        log.Printf("[%s] %s: %s", err.Severity, err.RuleID, err.Message)
        
        // Use suggestions for remediation
        for _, suggestion := range err.Suggestions {
            log.Printf("  Suggestion: %s", suggestion)
        }
    }
    
    // Handle warnings separately if needed
    for _, warning := range result.Warnings {
        log.Printf("Warning: %s", warning.Message)
    }
    
    return fmt.Errorf("validation failed with %d errors", len(result.Errors))
}
```

### Step 4: Migrate Custom Validators

Convert custom validators to the new ValidationRule interface:

```go
// Before
customValidator := func(ctx context.Context, event events.Event) error {
    // Validation logic
    if invalid {
        return fmt.Errorf("custom validation failed")
    }
    return nil
}

config.CustomValidators = append(config.CustomValidators, customValidator)

// After
type MyCustomRule struct {
    *events.BaseValidationRule
}

func NewMyCustomRule() *MyCustomRule {
    return &MyCustomRule{
        BaseValidationRule: events.NewBaseValidationRule(
            "MY_CUSTOM_RULE",
            "Validates custom business logic",
            events.ValidationSeverityError,
        ),
    }
}

func (r *MyCustomRule) Validate(event events.Event, ctx *events.ValidationContext) *events.ValidationResult {
    result := &events.ValidationResult{
        IsValid: true,
        Timestamp: time.Now(),
    }
    
    // Validation logic
    if invalid {
        result.AddError(r.CreateError(
            event,
            "Custom validation failed",
            map[string]interface{}{
                "reason": "specific reason",
            },
            []string{"Check X", "Try Y"},
        ))
    }
    
    return result
}

// Add to validator
validator.AddRule(NewMyCustomRule())
```

### Step 5: Enable Enhanced Features (Optional)

#### Enable Parallel Validation
```go
// Configure parallel validation
parallelConfig := &events.ParallelValidationConfig{
    EnableParallelExecution: true,
    MaxGoroutines: runtime.NumCPU() * 2,
    MinRulesForParallel: 3,
}
validator.SetParallelConfig(parallelConfig)

// Use parallel validation
result := validator.ValidateEventParallel(ctx, event)
```

#### Enable Caching
```go
// Create cache-enabled validator
cacheConfig := &cache.CacheConfig{
    L1Size: 10000,
    L1TTL: 5 * time.Minute,
    EnableL2Cache: true,
    L2TTL: 1 * time.Hour,
}

cacheValidator, err := cache.NewCacheValidator(validator, cacheConfig)
if err != nil {
    log.Fatal(err)
}

// Use cached validation
result := cacheValidator.ValidateWithCache(ctx, event)
```

#### Enable Distributed Validation
```go
// Configure distributed validation
distConfig := distributed.DefaultDistributedValidatorConfig("node-1")
distValidator, err := distributed.NewDistributedValidator(distConfig, validator)
if err != nil {
    log.Fatal(err)
}

// Start distributed validator
ctx := context.Background()
if err := distValidator.Start(ctx); err != nil {
    log.Fatal(err)
}
defer distValidator.Stop()

// Use distributed validation
result := distValidator.ValidateEvent(ctx, event)
```

#### Enable Monitoring
```go
// Configure monitoring integration
monitoringConfig := &monitoring.Config{
    PrometheusPort: 9090,
    EnableTracing: true,
    ServiceName: "my-validation-service",
    OTLPEndpoint: "localhost:4317",
}

monitor, err := monitoring.NewMonitoringIntegration(monitoringConfig, validator)
if err != nil {
    log.Fatal(err)
}

// Start monitoring
if err := monitor.Start(); err != nil {
    log.Fatal(err)
}
defer monitor.Stop()
```

### Step 6: Update Tests

Update tests to work with the new validation result structure:

```go
// Before
func TestValidation(t *testing.T) {
    validator := events.NewValidator(events.TestingValidationConfig())
    err := validator.ValidateEvent(context.Background(), event)
    assert.NoError(t, err)
}

// After
func TestValidation(t *testing.T) {
    validator := events.NewEventValidator(events.TestingValidationConfig())
    result := validator.ValidateEvent(context.Background(), event)
    
    assert.True(t, result.IsValid)
    assert.Empty(t, result.Errors)
    assert.Empty(t, result.Warnings)
}
```

## Rollback Procedures

If you need to rollback to the base system:

### 1. Feature Flag Approach (Recommended)

```go
type ValidatorWrapper struct {
    useEnhanced bool
    baseValidator *events.Validator
    enhancedValidator *events.EventValidator
}

func (w *ValidatorWrapper) ValidateEvent(ctx context.Context, event events.Event) error {
    if w.useEnhanced {
        result := w.enhancedValidator.ValidateEvent(ctx, event)
        if !result.IsValid {
            return fmt.Errorf("validation failed: %v", result.Errors[0].Message)
        }
        return nil
    }
    return w.baseValidator.ValidateEvent(ctx, event)
}
```

### 2. Version Pinning

```bash
# Pin to previous version
go get github.com/ag-ui/go-sdk@v1.2.3

# Update dependencies
go mod tidy
```

### 3. Code Rollback

Keep the old validation code in a separate branch or tag:

```bash
# Create rollback branch before migration
git checkout -b pre-enhanced-validation

# If rollback needed
git checkout pre-enhanced-validation
```

## Performance Considerations

### 1. Memory Usage

The enhanced system uses more memory due to:
- Validation state tracking
- Rule execution metrics
- Cache storage
- Parallel execution buffers

**Mitigation:**
```go
// Configure memory limits
perfConfig := &events.PerformanceConfig{
    CacheSize: 5000,        // Reduce cache size
    WorkerPoolSize: 4,      // Limit worker pool
    MemoryPoolSize: 500,    // Smaller memory pool
}

// Enable cleanup routine
validator.StartCleanupRoutine(ctx, 5*time.Minute, 1*time.Hour)
```

### 2. CPU Usage

Parallel validation can increase CPU usage:

**Mitigation:**
```go
// Limit parallelism
parallelConfig := &events.ParallelValidationConfig{
    MaxGoroutines: 2,              // Limit goroutines
    MinRulesForParallel: 10,       // Only parallelize for many rules
}
```

### 3. Latency

Initial requests may be slower due to:
- Cache warming
- Rule initialization
- Distributed consensus

**Mitigation:**
```go
// Warm up cache
validator.WarmupCache(ctx, commonEvents)

// Use fast mode for time-sensitive validation
perfConfig.Mode = events.FastMode
```

## Configuration Migration Examples

### Example 1: Basic to Enhanced

```go
// Old configuration
oldConfig := &events.ValidationConfig{
    Level: events.ValidationStrict,
    SkipTimestampValidation: false,
    AllowEmptyIDs: false,
}

// Enhanced configuration with backward compatibility
enhancedConfig := &events.ValidationConfig{
    Level: events.ValidationStrict,
    SkipTimestampValidation: false,
    AllowEmptyIDs: false,
    
    // New features (optional)
    CustomValidators: []events.CustomValidator{
        events.NewTimestampValidator(minTime, maxTime),
        events.NewEventTypeValidator(allowedTypes...),
    },
}

// Performance configuration
perfConfig := events.DefaultPerformanceConfig()

// Create validator
validator := events.NewEventValidator(enhancedConfig)
validator.SetPerformanceConfig(perfConfig)
```

### Example 2: Adding Custom Rules

```go
// Define custom rule for business logic
type BusinessLogicRule struct {
    *events.BaseValidationRule
    businessService BusinessService
}

func (r *BusinessLogicRule) Validate(event events.Event, ctx *events.ValidationContext) *events.ValidationResult {
    result := &events.ValidationResult{IsValid: true, Timestamp: time.Now()}
    
    // Custom validation logic
    if err := r.businessService.ValidateEvent(event); err != nil {
        result.AddError(r.CreateError(
            event,
            fmt.Sprintf("Business validation failed: %v", err),
            map[string]interface{}{
                "business_rule": "XYZ",
            },
            []string{
                "Ensure business rule XYZ is satisfied",
                "Contact support if unclear",
            },
        ))
    }
    
    return result
}

// Add to validator
validator.AddRule(&BusinessLogicRule{
    BaseValidationRule: events.NewBaseValidationRule(
        "BUSINESS_LOGIC",
        "Validates business-specific rules",
        events.ValidationSeverityError,
    ),
    businessService: myBusinessService,
})
```

### Example 3: Distributed Setup

```go
// Configure for distributed environment
nodeID := distributed.NodeID(os.Getenv("NODE_ID"))
distConfig := &distributed.DistributedValidatorConfig{
    NodeID: nodeID,
    ConsensusConfig: &distributed.ConsensusConfig{
        Algorithm: distributed.RaftConsensus,
        MinNodes: 3,
        Timeout: 5 * time.Second,
    },
    MaxNodeFailures: 1,
    ValidationTimeout: 10 * time.Second,
}

// Create distributed validator
baseValidator := events.NewEventValidator(validationConfig)
distValidator, err := distributed.NewDistributedValidator(distConfig, baseValidator)
if err != nil {
    log.Fatal(err)
}

// Register with cluster
ctx := context.Background()
if err := distValidator.Start(ctx); err != nil {
    log.Fatal(err)
}
```

## Migration Checklist

- [ ] Update dependencies to latest version
- [ ] Replace `NewValidator` with `NewEventValidator`
- [ ] Update validation error handling to use `ValidationResult`
- [ ] Migrate custom validators to `ValidationRule` interface
- [ ] Update tests for new result structure
- [ ] Configure performance settings
- [ ] Enable desired enhanced features (caching, parallel, distributed)
- [ ] Set up monitoring if needed
- [ ] Test rollback procedures
- [ ] Performance test under load
- [ ] Update documentation
- [ ] Train team on new features

## Support

For migration assistance:
- Review the [examples](./examples) directory
- Check [API documentation](./docs/api.md)
- Open an issue on GitHub
- Contact support for enterprise customers

## Version Compatibility

| SDK Version | Base System | Enhanced System |
|-------------|-------------|-----------------|
| < 1.3.0     | ✓           | ✗               |
| 1.3.0-1.4.x | ✓           | Beta            |
| >= 1.5.0    | Deprecated  | ✓               |

Plan your migration according to your current version and support requirements.