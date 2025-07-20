package sse

// This file contains examples of how to use the comprehensive SSE configuration system
// NOTE: These example functions intentionally panic on validation failures for demonstration purposes.
// In production code, proper error handling should be used instead of panicking.

import (
	"fmt"
	"time"

	"go.uber.org/zap/zapcore"
)

// ExampleBasicUsage demonstrates basic usage of the configuration system
func ExampleBasicUsage() {
	// Create a default configuration
	config := DefaultComprehensiveConfig()

	// Validate the configuration
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("Configuration validation failed: %v", err))
	}

	fmt.Printf("Default config created with base URL: %s\n", config.Connection.BaseURL)
}

// ExampleBuilderPattern demonstrates the builder pattern for configuration
func ExampleBuilderPattern() {
	config := NewConfigBuilder().
		WithBaseURL("https://api.myservice.com").
		WithEndpoint("/stream").
		WithBearerToken("my-secret-token").
		WithMaxRetries(5).
		WithCompression(CompressionAlgorithmGzip, 6).
		WithMetrics(true, 30*time.Second).
		WithEnvironment(EnvironmentProduction).
		Build()

	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("Builder config validation failed: %v", err))
	}

	fmt.Printf("Builder config created with URL: %s%s\n",
		config.Connection.BaseURL, config.Connection.Endpoint)
}

// ExampleEnvironmentConfigs demonstrates environment-specific configurations
func ExampleEnvironmentConfigs() {
	// Development configuration
	devConfig := DevelopmentConfig()
	fmt.Printf("Development - Debug mode: %v, Trace sampling: %.2f\n",
		devConfig.Features.DebugMode, devConfig.Monitoring.Tracing.SamplingRate)

	// Production configuration
	prodConfig := ProductionConfig()
	fmt.Printf("Production - Compression: %v, Rate limiting: %v\n",
		prodConfig.Performance.Compression.Enabled, prodConfig.Security.RateLimit.Enabled)

	// Staging configuration
	stagingConfig := StagingConfig()
	fmt.Printf("Staging - Debug mode: %v, Detailed metrics: %v\n",
		stagingConfig.Features.DebugMode, stagingConfig.Features.DetailedMetrics)
}

// ExampleConfigFromEnvironment demonstrates loading config from environment variables
func ExampleConfigFromEnvironment() {
	// This would typically read from actual environment variables
	// For demonstration, we show what variables would be read

	loader := NewConfigLoader()
	config := loader.LoadFromEnv()

	fmt.Printf("Environment config loaded with base URL: %s\n", config.Connection.BaseURL)

	// Example environment variables that would be read:
	fmt.Println("Example environment variables:")
	fmt.Println("  SSE_BASE_URL=https://api.example.com")
	fmt.Println("  SSE_AUTH_TYPE=bearer")
	fmt.Println("  SSE_AUTH_BEARER_TOKEN=my-token")
	fmt.Println("  SSE_RETRY_MAX_RETRIES=5")
	fmt.Println("  SSE_COMPRESSION_ENABLED=true")
	fmt.Println("  SSE_MONITORING_ENABLED=true")
}

// ExampleSecurityConfiguration demonstrates security configuration options
func ExampleSecurityConfiguration() {
	config := NewConfigBuilder().
		WithBaseURL("https://secure-api.example.com").
		WithAuth(AuthConfig{
			Type:        AuthTypeBearer,
			BearerToken: "secure-token-here",
		}).
		WithSecurity(SecurityConfig{
			Auth: AuthConfig{
				Type:        AuthTypeBearer,
				BearerToken: "secure-token-here",
			},
			RateLimit: RateLimitConfig{
				Enabled:           true,
				RequestsPerSecond: 100,
				BurstSize:         200,
			},
			Validation: ValidationConfig{
				Enabled:        true,
				MaxRequestSize: 10 * 1024 * 1024, // 10MB
			},
		}).
		Build()

	fmt.Printf("Security config - Auth type: %s, Rate limiting: %v\n",
		config.Security.Auth.Type, config.Security.RateLimit.Enabled)
}

// ExamplePerformanceConfiguration demonstrates performance tuning options
func ExamplePerformanceConfiguration() {
	config := NewConfigBuilder().
		WithPerformance(PerformanceConfig{
			Buffering: BufferingConfig{
				Enabled:         true,
				ReadBufferSize:  16384,
				WriteBufferSize: 16384,
				EventBufferSize: 2000,
			},
			Compression: CompressionConfig{
				Enabled:   true,
				Algorithm: CompressionAlgorithmGzip,
				Level:     6,
				MinSize:   1024,
			},
			Batching: BatchingConfig{
				Enabled:      true,
				BatchSize:    100,
				BatchTimeout: 100 * time.Millisecond,
			},
			Caching: CachingConfig{
				Enabled:        true,
				CacheSize:      1000,
				TTL:            5 * time.Minute,
				EvictionPolicy: EvictionPolicyLRU,
			},
		}).
		Build()

	fmt.Printf("Performance config - Compression: %v, Batching: %v, Caching: %v\n",
		config.Performance.Compression.Enabled,
		config.Performance.Batching.Enabled,
		config.Performance.Caching.Enabled)
}

// ExampleMonitoringConfiguration demonstrates monitoring and observability setup
func ExampleMonitoringConfiguration() {
	config := NewConfigBuilder().
		WithMonitoring(MonitoringConfig{
			Enabled: true,
			Metrics: MetricsConfig{
				Enabled:  true,
				Interval: 30 * time.Second,
				Prometheus: PrometheusConfig{
					Enabled:   true,
					Namespace: "myapp",
					Subsystem: "sse_transport",
				},
			},
			Logging: LoggingConfig{
				Enabled:    true,
				Level:      zapcore.InfoLevel,
				Format:     "json",
				Structured: true,
			},
			Tracing: TracingConfig{
				Enabled:      true,
				Provider:     "jaeger",
				ServiceName:  "sse-transport",
				SamplingRate: 0.1,
			},
			HealthChecks: HealthChecksConfig{
				Enabled:  true,
				Interval: 30 * time.Second,
				Timeout:  5 * time.Second,
			},
		}).
		Build()

	fmt.Printf("Monitoring config - Metrics: %v, Tracing: %v, Health checks: %v\n",
		config.Monitoring.Metrics.Enabled,
		config.Monitoring.Tracing.Enabled,
		config.Monitoring.HealthChecks.Enabled)
}

// ExampleBackwardCompatibility demonstrates compatibility with existing code
func ExampleBackwardCompatibility() {
	// Create a comprehensive config
	comprehensive := NewConfigBuilder().
		WithBaseURL("https://api.example.com").
		WithBearerToken("token").
		Build()

	// Convert to simple config for use with existing transport code
	simple := comprehensive.ToSimpleConfig()

	fmt.Printf("Simple config - BaseURL: %s, BufferSize: %d\n",
		simple.BaseURL, simple.BufferSize)

	// Convert back to comprehensive config
	backToComprehensive := FromSimpleConfig(simple)

	fmt.Printf("Back to comprehensive - BaseURL: %s, Auth type: %s\n",
		backToComprehensive.Connection.BaseURL, backToComprehensive.Security.Auth.Type)
}

// ExampleConfigPersistence demonstrates saving and loading configuration
func ExampleConfigPersistence() {
	// Create a configuration
	config := NewConfigBuilder().
		WithBaseURL("https://api.example.com").
		WithBearerToken("my-token").
		WithEnvironment(EnvironmentProduction).
		Build()

	// Save to file (in real usage)
	// config.SaveToFile("/path/to/config.json")

	// Load from file (in real usage)
	// loader := NewConfigLoader()
	// loadedConfig, err := loader.LoadFromFile("/path/to/config.json")

	fmt.Printf("Config can be saved to and loaded from JSON files\n")
	fmt.Printf("Config JSON representation:\n%s\n", config.String())
}

// ExampleValidationAndErrorHandling demonstrates validation and error handling
func ExampleValidationAndErrorHandling() {
	// Create an invalid configuration
	config := ComprehensiveConfig{}

	// Validate and handle errors
	if err := config.Validate(); err != nil {
		fmt.Printf("Validation error: %v\n", err)
	}

	// Create a valid configuration
	validConfig := DefaultComprehensiveConfig()
	if err := validConfig.Validate(); err != nil {
		fmt.Printf("Unexpected validation error: %v\n", err)
	} else {
		fmt.Printf("Valid configuration created successfully\n")
	}
}

// ExampleCustomConfiguration demonstrates creating custom configurations
func ExampleCustomConfiguration() {
	// Create a custom configuration for a specific use case
	config := NewConfigBuilder().
		// High-throughput scenario
		WithConnection(ConnectionConfig{
			BaseURL:        "https://high-throughput-api.example.com",
			Endpoint:       "/events/stream",
			ConnectTimeout: 10 * time.Second,
			ReadTimeout:    60 * time.Second,
			WriteTimeout:   10 * time.Second,
			ConnectionPool: ConnectionPoolConfig{
				MaxConnections:     50,
				MaxIdleConnections: 10,
				ConnectionLifetime: 10 * time.Minute,
			},
		}).
		// Aggressive retry policy
		WithRetry(RetryConfig{
			Enabled:           true,
			MaxRetries:        10,
			InitialDelay:      50 * time.Millisecond,
			MaxDelay:          5 * time.Second,
			BackoffStrategy:   BackoffStrategyExponential,
			BackoffMultiplier: 1.5,
			JitterFactor:      0.2,
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 10,
				SuccessThreshold: 5,
				Timeout:          30 * time.Second,
			},
		}).
		// Performance optimizations
		WithPerformance(PerformanceConfig{
			Buffering: BufferingConfig{
				Enabled:         true,
				ReadBufferSize:  32768,
				WriteBufferSize: 32768,
				EventBufferSize: 5000,
				FlushInterval:   50 * time.Millisecond,
			},
			Compression: CompressionConfig{
				Enabled:   true,
				Algorithm: CompressionAlgorithmGzip,
				Level:     9, // Maximum compression
				MinSize:   512,
			},
			Batching: BatchingConfig{
				Enabled:      true,
				BatchSize:    200,
				BatchTimeout: 50 * time.Millisecond,
				MaxBatchSize: 1000,
			},
		}).
		Build()

	fmt.Printf("Custom high-throughput config created\n")
	fmt.Printf("Max connections: %d, Max retries: %d, Batch size: %d\n",
		config.Connection.ConnectionPool.MaxConnections,
		config.Retry.MaxRetries,
		config.Performance.Batching.BatchSize)
}

// RunAllExamples runs all configuration examples
func RunAllExamples() {
	fmt.Println("=== SSE Transport Configuration Examples ===")

	fmt.Println("1. Basic Usage:")
	ExampleBasicUsage()
	fmt.Println()

	fmt.Println("2. Builder Pattern:")
	ExampleBuilderPattern()
	fmt.Println()

	fmt.Println("3. Environment Configs:")
	ExampleEnvironmentConfigs()
	fmt.Println()

	fmt.Println("4. Environment Variables:")
	ExampleConfigFromEnvironment()
	fmt.Println()

	fmt.Println("5. Security Configuration:")
	ExampleSecurityConfiguration()
	fmt.Println()

	fmt.Println("6. Performance Configuration:")
	ExamplePerformanceConfiguration()
	fmt.Println()

	fmt.Println("7. Monitoring Configuration:")
	ExampleMonitoringConfiguration()
	fmt.Println()

	fmt.Println("8. Backward Compatibility:")
	ExampleBackwardCompatibility()
	fmt.Println()

	fmt.Println("9. Config Persistence:")
	ExampleConfigPersistence()
	fmt.Println()

	fmt.Println("10. Validation and Error Handling:")
	ExampleValidationAndErrorHandling()
	fmt.Println()

	fmt.Println("11. Custom Configuration:")
	ExampleCustomConfiguration()
	fmt.Println()
}
