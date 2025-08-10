package config

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// ExpensiveValidator simulates a validator that does heavy computation
type ExpensiveValidator struct {
	name       string
	complexity int // Simulates validation complexity (microseconds)
}

func NewExpensiveValidator(name string, complexityMicros int) *ExpensiveValidator {
	return &ExpensiveValidator{
		name:       name,
		complexity: complexityMicros,
	}
}

func (v *ExpensiveValidator) Name() string {
	return v.name
}

func (v *ExpensiveValidator) Validate(config map[string]interface{}) error {
	// Simulate expensive validation work
	time.Sleep(time.Duration(v.complexity) * time.Microsecond)
	
	// Simulate some validation logic that might fail
	if val, ok := config["fail"]; ok && val == true {
		return fmt.Errorf("validation failed as requested")
	}
	
	return nil
}

func (v *ExpensiveValidator) ValidateField(key string, value interface{}) error {
	// Simulate expensive field validation
	time.Sleep(time.Duration(v.complexity/2) * time.Microsecond)
	
	if key == "invalid" {
		return fmt.Errorf("invalid field")
	}
	
	return nil
}

func (v *ExpensiveValidator) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
			"age":  map[string]interface{}{"type": "integer"},
		},
	}
}

// Benchmark validation without caching
func BenchmarkValidation_WithoutCache(b *testing.B) {
	validator := NewExpensiveValidator("expensive-validator", 100) // 100 microseconds per validation
	
	configs := generateTestConfigs(100) // Generate 100 different configs
	
	b.ResetTimer()
	b.StartTimer()
	
	for i := 0; i < b.N; i++ {
		config := configs[i%len(configs)]
		validator.Validate(config)
	}
	
	b.StopTimer()
}

// Benchmark validation with caching
func BenchmarkValidation_WithCache(b *testing.B) {
	validator := NewExpensiveValidator("expensive-validator", 100) // 100 microseconds per validation
	
	cacheConfig := &ValidationCacheConfig{
		MaxSize:       1000,
		TTL:           5 * time.Minute,
		CleanupPeriod: time.Minute,
		Enabled:       true,
	}
	
	cachedValidator := NewCachedValidator(validator, cacheConfig)
	defer cachedValidator.Stop()
	
	configs := generateTestConfigs(100) // Generate 100 different configs
	
	b.ResetTimer()
	b.StartTimer()
	
	for i := 0; i < b.N; i++ {
		config := configs[i%len(configs)]
		cachedValidator.Validate(config)
	}
	
	b.StopTimer()
}

// Benchmark with high cache hit ratio (repeated validations)
func BenchmarkValidation_HighCacheHitRatio(b *testing.B) {
	validator := NewExpensiveValidator("expensive-validator", 100)
	
	cachedValidator := NewCachedValidator(validator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()
	
	// Only 10 different configs, so high cache hit ratio expected
	configs := generateTestConfigs(10)
	
	b.ResetTimer()
	b.StartTimer()
	
	for i := 0; i < b.N; i++ {
		config := configs[i%len(configs)]
		cachedValidator.Validate(config)
	}
	
	b.StopTimer()
}

// Benchmark with low cache hit ratio (many different configurations)
func BenchmarkValidation_LowCacheHitRatio(b *testing.B) {
	validator := NewExpensiveValidator("expensive-validator", 100)
	
	cachedValidator := NewCachedValidator(validator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()
	
	// Generate unique config for each iteration (no cache hits expected)
	b.ResetTimer()
	b.StartTimer()
	
	for i := 0; i < b.N; i++ {
		config := map[string]interface{}{
			"unique_id": i,
			"name":      fmt.Sprintf("user_%d", i),
		}
		cachedValidator.Validate(config)
	}
	
	b.StopTimer()
}

// Benchmark field validation without caching
func BenchmarkFieldValidation_WithoutCache(b *testing.B) {
	validator := NewExpensiveValidator("expensive-validator", 50) // 50 microseconds per field validation
	
	fields := []string{"name", "age", "email", "address", "phone"}
	values := []interface{}{"john", 30, "john@example.com", "123 Main St", "555-1234"}
	
	b.ResetTimer()
	b.StartTimer()
	
	for i := 0; i < b.N; i++ {
		field := fields[i%len(fields)]
		value := values[i%len(values)]
		validator.ValidateField(field, value)
	}
	
	b.StopTimer()
}

// Benchmark field validation with caching
func BenchmarkFieldValidation_WithCache(b *testing.B) {
	validator := NewExpensiveValidator("expensive-validator", 50)
	
	cachedValidator := NewCachedValidator(validator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()
	
	fields := []string{"name", "age", "email", "address", "phone"}
	values := []interface{}{"john", 30, "john@example.com", "123 Main St", "555-1234"}
	
	b.ResetTimer()
	b.StartTimer()
	
	for i := 0; i < b.N; i++ {
		field := fields[i%len(fields)]
		value := values[i%len(values)]
		cachedValidator.ValidateField(field, value)
	}
	
	b.StopTimer()
}

// Benchmark concurrent validation without caching
func BenchmarkValidation_ConcurrentWithoutCache(b *testing.B) {
	validator := NewExpensiveValidator("expensive-validator", 100)
	configs := generateTestConfigs(50)
	
	b.ResetTimer()
	b.StartTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			config := configs[i%len(configs)]
			validator.Validate(config)
			i++
		}
	})
	
	b.StopTimer()
}

// Benchmark concurrent validation with caching
func BenchmarkValidation_ConcurrentWithCache(b *testing.B) {
	validator := NewExpensiveValidator("expensive-validator", 100)
	
	cachedValidator := NewCachedValidator(validator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()
	
	configs := generateTestConfigs(50)
	
	b.ResetTimer()
	b.StartTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			config := configs[i%len(configs)]
			cachedValidator.Validate(config)
			i++
		}
	})
	
	b.StopTimer()
}

// Benchmark schema validator (existing) without caching
func BenchmarkSchemaValidation_WithoutCache(b *testing.B) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":      "string",
				"minLength": float64(1),
				"maxLength": float64(100),
			},
			"age": map[string]interface{}{
				"type":    "integer",
				"minimum": float64(0),
				"maximum": float64(150),
			},
			"email": map[string]interface{}{
				"type":    "string",
				"pattern": `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			},
			"tags": map[string]interface{}{
				"type":     "array",
				"minItems": float64(0),
				"maxItems": float64(10),
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []interface{}{"name", "email"},
	}
	
	validator := NewSchemaValidator("test-schema", schema)
	configs := generateComplexTestConfigs(100)
	
	b.ResetTimer()
	b.StartTimer()
	
	for i := 0; i < b.N; i++ {
		config := configs[i%len(configs)]
		validator.Validate(config)
	}
	
	b.StopTimer()
}

// Benchmark schema validator with caching
func BenchmarkSchemaValidation_WithCache(b *testing.B) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":      "string",
				"minLength": float64(1),
				"maxLength": float64(100),
			},
			"age": map[string]interface{}{
				"type":    "integer",
				"minimum": float64(0),
				"maximum": float64(150),
			},
			"email": map[string]interface{}{
				"type":    "string",
				"pattern": `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			},
			"tags": map[string]interface{}{
				"type":     "array",
				"minItems": float64(0),
				"maxItems": float64(10),
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []interface{}{"name", "email"},
	}
	
	validator := NewSchemaValidator("test-schema", schema)
	cachedValidator := NewCachedValidator(validator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()
	
	configs := generateComplexTestConfigs(100)
	
	b.ResetTimer()
	b.StartTimer()
	
	for i := 0; i < b.N; i++ {
		config := configs[i%len(configs)]
		cachedValidator.Validate(config)
	}
	
	b.StopTimer()
}

// Helper function to generate test configurations
func generateTestConfigs(count int) []map[string]interface{} {
	configs := make([]map[string]interface{}, count)
	
	for i := 0; i < count; i++ {
		configs[i] = map[string]interface{}{
			"name":    fmt.Sprintf("user_%d", i%10), // Some repetition for cache hits
			"age":     20 + (i % 50),
			"active":  i%2 == 0,
			"score":   rand.Float64() * 100,
			"tags":    []string{fmt.Sprintf("tag_%d", i%5)},
		}
	}
	
	return configs
}

// Helper function to generate complex test configurations for schema validation
func generateComplexTestConfigs(count int) []map[string]interface{} {
	configs := make([]map[string]interface{}, count)
	names := []string{"Alice", "Bob", "Charlie", "Diana", "Eve"}
	domains := []string{"example.com", "test.org", "demo.net"}
	
	for i := 0; i < count; i++ {
		name := names[i%len(names)]
		domain := domains[i%len(domains)]
		
		configs[i] = map[string]interface{}{
			"name":  name,
			"age":   18 + (i % 60),
			"email": fmt.Sprintf("%s%d@%s", name, i%10, domain),
			"tags":  []interface{}{"user", fmt.Sprintf("group_%d", i%3)},
		}
	}
	
	return configs
}

// Example benchmark that demonstrates real-world configuration validation performance
func BenchmarkRealWorldConfig_WithoutCache(b *testing.B) {
	// Create a complex schema similar to real application configs
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"database": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"host":     map[string]interface{}{"type": "string"},
					"port":     map[string]interface{}{"type": "integer", "minimum": float64(1), "maximum": float64(65535)},
					"username": map[string]interface{}{"type": "string"},
					"password": map[string]interface{}{"type": "string"},
					"database": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"host", "port", "database"},
			},
			"server": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"host":    map[string]interface{}{"type": "string"},
					"port":    map[string]interface{}{"type": "integer", "minimum": float64(1), "maximum": float64(65535)},
					"timeout": map[string]interface{}{"type": "integer", "minimum": float64(1)},
				},
				"required": []interface{}{"host", "port"},
			},
			"logging": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"level":  map[string]interface{}{"type": "string", "enum": []interface{}{"debug", "info", "warn", "error"}},
					"format": map[string]interface{}{"type": "string"},
					"output": map[string]interface{}{"type": "string"},
				},
			},
		},
		"required": []interface{}{"database", "server"},
	}
	
	validator := NewSchemaValidator("app-config", schema)
	configs := generateRealWorldConfigs(50)
	
	b.ResetTimer()
	b.StartTimer()
	
	for i := 0; i < b.N; i++ {
		config := configs[i%len(configs)]
		validator.Validate(config)
	}
	
	b.StopTimer()
}

func BenchmarkRealWorldConfig_WithCache(b *testing.B) {
	// Same schema as above
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"database": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"host":     map[string]interface{}{"type": "string"},
					"port":     map[string]interface{}{"type": "integer", "minimum": float64(1), "maximum": float64(65535)},
					"username": map[string]interface{}{"type": "string"},
					"password": map[string]interface{}{"type": "string"},
					"database": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"host", "port", "database"},
			},
			"server": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"host":    map[string]interface{}{"type": "string"},
					"port":    map[string]interface{}{"type": "integer", "minimum": float64(1), "maximum": float64(65535)},
					"timeout": map[string]interface{}{"type": "integer", "minimum": float64(1)},
				},
				"required": []interface{}{"host", "port"},
			},
			"logging": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"level":  map[string]interface{}{"type": "string", "enum": []interface{}{"debug", "info", "warn", "error"}},
					"format": map[string]interface{}{"type": "string"},
					"output": map[string]interface{}{"type": "string"},
				},
			},
		},
		"required": []interface{}{"database", "server"},
	}
	
	validator := NewSchemaValidator("app-config", schema)
	cachedValidator := NewCachedValidator(validator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()
	
	configs := generateRealWorldConfigs(50)
	
	b.ResetTimer()
	b.StartTimer()
	
	for i := 0; i < b.N; i++ {
		config := configs[i%len(configs)]
		cachedValidator.Validate(config)
	}
	
	b.StopTimer()
}

// Generate realistic application configurations
func generateRealWorldConfigs(count int) []map[string]interface{} {
	configs := make([]map[string]interface{}, count)
	
	environments := []string{"dev", "staging", "prod"}
	logLevels := []string{"debug", "info", "warn", "error"}
	
	for i := 0; i < count; i++ {
		env := environments[i%len(environments)]
		
		configs[i] = map[string]interface{}{
			"database": map[string]interface{}{
				"host":     fmt.Sprintf("db-%s.example.com", env),
				"port":     5432,
				"username": fmt.Sprintf("app_%s", env),
				"password": "secret123",
				"database": fmt.Sprintf("appdb_%s", env),
			},
			"server": map[string]interface{}{
				"host":    "0.0.0.0",
				"port":    8080 + (i % 10), // Some variation
				"timeout": 30,
			},
			"logging": map[string]interface{}{
				"level":  logLevels[i%len(logLevels)],
				"format": "json",
				"output": "stdout",
			},
		}
	}
	
	return configs
}