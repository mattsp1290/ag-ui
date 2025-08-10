package config

import (
	"fmt"
	"log"
	"time"
)

// ExampleValidationCache demonstrates how to use validation result caching
// to improve performance for repeated validations
func ExampleValidationCache() {
	// Create a complex schema validator
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"user": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":      "string",
						"minLength": float64(1),
						"maxLength": float64(100),
					},
					"email": map[string]interface{}{
						"type":    "string",
						"pattern": `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
					},
					"age": map[string]interface{}{
						"type":    "integer",
						"minimum": float64(0),
						"maximum": float64(150),
					},
				},
				"required": []interface{}{"name", "email"},
			},
		},
		"required": []interface{}{"user"},
	}

	// Create the original validator
	originalValidator := NewSchemaValidator("user-schema", schema)

	// Wrap it with caching for performance
	cacheConfig := &ValidationCacheConfig{
		MaxSize:       100,               // Store up to 100 validation results
		TTL:           5 * time.Minute,   // Cache for 5 minutes
		CleanupPeriod: time.Minute,      // Clean up expired entries every minute
		Enabled:       true,
	}
	
	cachedValidator := NewCachedValidator(originalValidator, cacheConfig)
	defer cachedValidator.Stop() // Important: stop the cache cleanup goroutine

	// Test configuration
	config := map[string]interface{}{
		"user": map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
			"age":   30,
		},
	}

	// First validation (cache miss)
	start := time.Now()
	err1 := cachedValidator.Validate(config)
	duration1 := time.Since(start)
	
	fmt.Printf("First validation: %v (took %v)\n", err1 == nil, duration1)

	// Second validation (cache hit - much faster)
	start = time.Now()
	err2 := cachedValidator.Validate(config)
	duration2 := time.Since(start)
	
	fmt.Printf("Second validation: %v (took %v)\n", err2 == nil, duration2)

	// Get cache metrics
	metrics := cachedValidator.GetMetrics()
	fmt.Printf("Cache hits: %d, misses: %d, hit ratio: %.1f%%\n", 
		metrics.Hits, metrics.Misses, cachedValidator.GetHitRatio())

}

// Example_cachedValidatorIntegration shows how to integrate
// validation caching into the configuration system
func Example_cachedValidatorIntegration() {
	// Create a schema validator
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"database": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"host": map[string]interface{}{"type": "string"},
					"port": map[string]interface{}{"type": "integer"},
				},
				"required": []interface{}{"host", "port"},
			},
		},
		"required": []interface{}{"database"},
	}
	validator := NewSchemaValidator("app-config", schema)

	// Configure validation caching
	cacheConfig := &ValidationCacheConfig{
		MaxSize:       500,               // Large cache for complex validations
		TTL:           10 * time.Minute,  // Cache for 10 minutes
		CleanupPeriod: 2 * time.Minute,  // Clean up every 2 minutes
		Enabled:       true,
	}

	// Build configuration with caching enabled
	config, err := NewConfigBuilder().
		AddValidator(validator).
		WithOptions(&BuilderOptions{
			EnableHotReload:       true,
			ValidateOnBuild:       true,
			ValidationCacheConfig: cacheConfig,
		}).
		Build()

	if err != nil {
		log.Printf("Failed to build config: %v", err)
		return
	}
	defer config.(*ConfigImpl).Shutdown()

	// Set some configuration values
	config.Set("database.host", "localhost")
	config.Set("database.port", 5432)

	// Validation happens automatically and is cached
	err = config.Validate()
	if err != nil {
		log.Printf("Validation failed: %v", err)
		return
	}

	// Get cache performance metrics
	metrics := config.(*ConfigImpl).GetValidationCacheMetrics()
	fmt.Printf("Validation cache performance:\n")
	fmt.Printf("  Total checks: %d\n", metrics.TotalChecks)
	fmt.Printf("  Cache hits: %d\n", metrics.Hits)
	fmt.Printf("  Cache misses: %d\n", metrics.Misses)
	fmt.Printf("  Cache size: %d/%d\n", metrics.Size, metrics.MaxSize)
	fmt.Printf("  TTL: %v\n", metrics.TTL)

	fmt.Println("Configuration system with validation caching setup complete")

}

// Example_validationCacheManager demonstrates managing multiple cached validators
func Example_validationCacheManager() {
	// Create a validation cache manager with shared cache
	cacheConfig := &ValidationCacheConfig{
		MaxSize:       200,
		TTL:           15 * time.Minute,
		CleanupPeriod: 5 * time.Minute,
		Enabled:       true,
	}
	
	manager := NewCachedValidatorManager(cacheConfig)
	defer manager.Stop()

	// Create different validators for different schemas
	userSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":  map[string]interface{}{"type": "string"},
			"email": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"name", "email"},
	}

	productSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":  map[string]interface{}{"type": "string"},
			"price": map[string]interface{}{"type": "number", "minimum": float64(0)},
		},
		"required": []interface{}{"name", "price"},
	}

	// Create original validators
	userValidator := NewSchemaValidator("user-schema", userSchema)
	productValidator := NewSchemaValidator("product-schema", productSchema)

	// Wrap them with the shared cache
	cachedUserValidator := manager.WrapValidator(userValidator)
	cachedProductValidator := manager.WrapValidator(productValidator)

	// Use the validators
	userConfig := map[string]interface{}{
		"name":  "Alice",
		"email": "alice@example.com",
	}

	productConfig := map[string]interface{}{
		"name":  "Widget",
		"price": 29.99,
	}

	// Validate (results will be cached in shared cache)
	userErr := cachedUserValidator.Validate(userConfig)
	productErr := cachedProductValidator.Validate(productConfig)

	fmt.Printf("User validation: %v\n", userErr == nil)
	fmt.Printf("Product validation: %v\n", productErr == nil)

	// Get aggregated metrics from shared cache
	metrics := manager.GetAggregatedMetrics()
	fmt.Printf("Shared cache metrics: %d total checks, %d size\n", 
		metrics.TotalChecks, metrics.Size)

}

// Example_performanceComparison demonstrates the performance benefits of caching
func Example_performanceComparison() {
	// Create an expensive validator (simulates complex validation logic)
	expensiveValidator := NewExpensiveValidator("complex-validator", 1000) // 1ms per validation

	// Test without caching
	config := map[string]interface{}{
		"data": "test-value",
	}

	start := time.Now()
	for i := 0; i < 100; i++ {
		expensiveValidator.Validate(config)
	}
	uncachedDuration := time.Since(start)

	// Test with caching
	cachedValidator := NewCachedValidator(expensiveValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	start = time.Now()
	for i := 0; i < 100; i++ {
		cachedValidator.Validate(config) // Only first call is expensive, rest are cached
	}
	cachedDuration := time.Since(start)

	improvement := float64(uncachedDuration.Nanoseconds()) / float64(cachedDuration.Nanoseconds())
	
	fmt.Printf("Performance comparison for 100 validations:\n")
	fmt.Printf("  Without cache: %v\n", uncachedDuration)
	fmt.Printf("  With cache: %v\n", cachedDuration)
	fmt.Printf("  Performance improvement: %.1fx faster\n", improvement)

	// Get final metrics
	fmt.Printf("Final cache stats: %.1f%% hit ratio\n", cachedValidator.GetHitRatio())

}

// Example_cacheInvalidationOnConfigChange demonstrates how cache invalidation
// works when configuration changes
func Example_cacheInvalidationOnConfigChange() {
	// Create validator and cached version
	validator := NewCustomValidator("test-validator")
	validator.AddRule("name", RequiredRule)

	cachedValidator := NewCachedValidator(validator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	config := map[string]interface{}{
		"name": "test",
	}

	// First validation (populates cache)
	err1 := cachedValidator.Validate(config)
	fmt.Printf("First validation passed: %v\n", err1 == nil)

	// Second validation (uses cache)
	err2 := cachedValidator.Validate(config)
	fmt.Printf("Second validation passed: %v\n", err2 == nil)

	metrics := cachedValidator.GetMetrics()
	fmt.Printf("Before invalidation - Hits: %d, Size: %d\n", metrics.Hits, metrics.Size)

	// Simulate configuration change by invalidating cache
	cachedValidator.InvalidateCache()

	// Check metrics after invalidation
	metrics = cachedValidator.GetMetrics()
	fmt.Printf("After invalidation - Hits: %d, Size: %d, Invalidations: %d\n", 
		metrics.Hits, metrics.Size, metrics.Invalidations)

	// Next validation will re-populate cache
	err3 := cachedValidator.Validate(config)
	fmt.Printf("Post-invalidation validation passed: %v\n", err3 == nil)

}