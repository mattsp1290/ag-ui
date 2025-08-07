package config

import (
	"fmt"
	"time"
)

// ValidatorBuilder provides a fluent API for building ValidatorConfig
type ValidatorBuilder struct {
	config *ValidatorConfig
	errors []error
}

// NewValidatorBuilder creates a new ValidatorBuilder with default configuration
func NewValidatorBuilder() *ValidatorBuilder {
	return &ValidatorBuilder{
		config: DefaultValidatorConfig(),
		errors: make([]error, 0),
	}
}

// NewValidatorBuilderForEnvironment creates a new ValidatorBuilder for a specific environment
func NewValidatorBuilderForEnvironment(env string) *ValidatorBuilder {
	var config *ValidatorConfig

	switch env {
	case "development", "dev":
		config = DevelopmentValidatorConfig()
	case "production", "prod":
		config = ProductionValidatorConfig()
	case "testing", "test":
		config = TestingValidatorConfig()
	default:
		config = DefaultValidatorConfig()
		config.Global.Environment = env
	}

	return &ValidatorBuilder{
		config: config,
		errors: make([]error, 0),
	}
}

// Build builds the final ValidatorConfig and returns any validation errors
func (b *ValidatorBuilder) Build() (*ValidatorConfig, error) {
	if len(b.errors) > 0 {
		return nil, fmt.Errorf("configuration build failed with %d errors: %v", len(b.errors), b.errors)
	}

	// Validate the final configuration
	if err := b.validateConfig(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return b.config, nil
}

// MustBuild builds the configuration and panics on error (useful for tests)
func (b *ValidatorBuilder) MustBuild() *ValidatorConfig {
	config, err := b.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build validator config: %v", err))
	}
	return config
}

// Core validation configuration

// WithCoreValidation configures core validation settings
func (b *ValidatorBuilder) WithCoreValidation(fn func(*CoreValidationConfig)) *ValidatorBuilder {
	if b.config.Core == nil {
		b.config.Core = DefaultCoreValidationConfig()
	}
	fn(b.config.Core)
	return b
}

// WithValidationLevel sets the validation level
func (b *ValidatorBuilder) WithValidationLevel(level ValidationLevel) *ValidatorBuilder {
	if b.config.Core == nil {
		b.config.Core = DefaultCoreValidationConfig()
	}
	b.config.Core.Level = level
	return b
}

// WithStrictValidation enables or disables strict validation
func (b *ValidatorBuilder) WithStrictValidation(strict bool) *ValidatorBuilder {
	if b.config.Core == nil {
		b.config.Core = DefaultCoreValidationConfig()
	}
	b.config.Core.Strict = strict
	return b
}

// WithValidationTimeout sets the validation timeout
func (b *ValidatorBuilder) WithValidationTimeout(timeout time.Duration) *ValidatorBuilder {
	if b.config.Core == nil {
		b.config.Core = DefaultCoreValidationConfig()
	}
	b.config.Core.ValidationTimeout = timeout
	return b
}

// WithConcurrency sets the maximum concurrent validations
func (b *ValidatorBuilder) WithConcurrency(maxConcurrent int) *ValidatorBuilder {
	if b.config.Core == nil {
		b.config.Core = DefaultCoreValidationConfig()
	}
	b.config.Core.MaxConcurrentValidations = maxConcurrent
	return b
}

// Authentication configuration

// WithAuthentication configures authentication settings
func (b *ValidatorBuilder) WithAuthentication(fn func(*AuthValidationConfig)) *ValidatorBuilder {
	if b.config.Auth == nil {
		b.config.Auth = DefaultAuthValidationConfig()
	}
	fn(b.config.Auth)
	return b
}

// WithAuthenticationEnabled enables or disables authentication
func (b *ValidatorBuilder) WithAuthenticationEnabled(enabled bool) *ValidatorBuilder {
	if b.config.Auth == nil {
		b.config.Auth = DefaultAuthValidationConfig()
	}
	b.config.Auth.Enabled = enabled
	return b
}

// WithAuthenticationProvider sets the authentication provider
func (b *ValidatorBuilder) WithAuthenticationProvider(providerType string, config map[string]interface{}) *ValidatorBuilder {
	if b.config.Auth == nil {
		b.config.Auth = DefaultAuthValidationConfig()
	}
	b.config.Auth.ProviderType = providerType
	b.config.Auth.ProviderConfig = config
	return b
}

// WithBasicAuth configures basic authentication
func (b *ValidatorBuilder) WithBasicAuth() *ValidatorBuilder {
	return b.WithAuthenticationProvider("basic", map[string]interface{}{
		"realm": "AG-UI Validator",
	})
}

// WithJWTAuth configures JWT authentication
func (b *ValidatorBuilder) WithJWTAuth(secret string, issuer string) *ValidatorBuilder {
	return b.WithAuthenticationProvider("jwt", map[string]interface{}{
		"secret": secret,
		"issuer": issuer,
	})
}

// WithOAuthAuth configures OAuth authentication
func (b *ValidatorBuilder) WithOAuthAuth(clientID, clientSecret, authURL, tokenURL string) *ValidatorBuilder {
	return b.WithAuthenticationProvider("oauth", map[string]interface{}{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"auth_url":      authURL,
		"token_url":     tokenURL,
	})
}

// WithRBAC enables role-based access control
func (b *ValidatorBuilder) WithRBAC(enabled bool, defaultRoles []string) *ValidatorBuilder {
	if b.config.Auth == nil {
		b.config.Auth = DefaultAuthValidationConfig()
	}
	b.config.Auth.EnableRBAC = enabled
	b.config.Auth.DefaultRoles = defaultRoles
	return b
}

// Cache configuration

// WithCache configures cache settings
func (b *ValidatorBuilder) WithCache(fn func(*CacheValidationConfig)) *ValidatorBuilder {
	if b.config.Cache == nil {
		b.config.Cache = DefaultCacheValidationConfig()
	}
	fn(b.config.Cache)
	return b
}

// WithCacheEnabled enables or disables caching
func (b *ValidatorBuilder) WithCacheEnabled(enabled bool) *ValidatorBuilder {
	if b.config.Cache == nil {
		b.config.Cache = DefaultCacheValidationConfig()
	}
	b.config.Cache.Enabled = enabled
	return b
}

// WithL1Cache configures L1 (in-memory) cache
func (b *ValidatorBuilder) WithL1Cache(size int, ttl time.Duration) *ValidatorBuilder {
	if b.config.Cache == nil {
		b.config.Cache = DefaultCacheValidationConfig()
	}
	b.config.Cache.L1Enabled = true
	b.config.Cache.L1Size = size
	b.config.Cache.L1TTL = ttl
	return b
}

// WithL2Cache configures L2 (distributed) cache
func (b *ValidatorBuilder) WithL2Cache(provider string, ttl time.Duration, config map[string]interface{}) *ValidatorBuilder {
	if b.config.Cache == nil {
		b.config.Cache = DefaultCacheValidationConfig()
	}
	b.config.Cache.L2Enabled = true
	b.config.Cache.L2Provider = provider
	b.config.Cache.L2TTL = ttl
	b.config.Cache.L2Config = config
	return b
}

// WithRedisCache configures Redis as L2 cache
func (b *ValidatorBuilder) WithRedisCache(address, password string, db int, ttl time.Duration) *ValidatorBuilder {
	return b.WithL2Cache("redis", ttl, map[string]interface{}{
		"address":  address,
		"password": password,
		"db":       db,
	})
}

// WithCacheCompression enables cache compression
func (b *ValidatorBuilder) WithCacheCompression(enabled bool, compressionType string, level int) *ValidatorBuilder {
	if b.config.Cache == nil {
		b.config.Cache = DefaultCacheValidationConfig()
	}
	b.config.Cache.CompressionEnabled = enabled
	b.config.Cache.CompressionType = compressionType
	b.config.Cache.CompressionLevel = level
	return b
}

// Distributed validation configuration

// WithDistributed configures distributed validation settings
func (b *ValidatorBuilder) WithDistributed(fn func(*DistributedValidationConfig)) *ValidatorBuilder {
	if b.config.Distributed == nil {
		b.config.Distributed = DefaultDistributedValidationConfig()
	}
	fn(b.config.Distributed)
	return b
}

// WithDistributedEnabled enables or disables distributed validation
func (b *ValidatorBuilder) WithDistributedEnabled(enabled bool) *ValidatorBuilder {
	if b.config.Distributed == nil {
		b.config.Distributed = DefaultDistributedValidationConfig()
	}
	b.config.Distributed.Enabled = enabled
	return b
}

// WithDistributedNode configures the distributed node
func (b *ValidatorBuilder) WithDistributedNode(nodeID, role, listenAddr string) *ValidatorBuilder {
	if b.config.Distributed == nil {
		b.config.Distributed = DefaultDistributedValidationConfig()
	}
	b.config.Distributed.NodeID = nodeID
	b.config.Distributed.NodeRole = role
	b.config.Distributed.ListenAddress = listenAddr
	return b
}

// WithConsensus configures consensus algorithm
func (b *ValidatorBuilder) WithConsensus(algorithm string, timeout time.Duration, minNodes, maxNodes int) *ValidatorBuilder {
	if b.config.Distributed == nil {
		b.config.Distributed = DefaultDistributedValidationConfig()
	}
	b.config.Distributed.ConsensusAlgorithm = algorithm
	b.config.Distributed.ConsensusTimeout = timeout
	b.config.Distributed.MinNodes = minNodes
	b.config.Distributed.MaxNodes = maxNodes
	return b
}

// WithTLS enables TLS for distributed communication
func (b *ValidatorBuilder) WithTLS(enabled bool, certFile, keyFile, caFile string, mutualTLS bool) *ValidatorBuilder {
	if b.config.Distributed == nil {
		b.config.Distributed = DefaultDistributedValidationConfig()
	}
	b.config.Distributed.EnableTLS = enabled
	b.config.Distributed.TLSCertFile = certFile
	b.config.Distributed.TLSKeyFile = keyFile
	b.config.Distributed.TLSCAFile = caFile
	b.config.Distributed.EnableMutualTLS = mutualTLS
	return b
}

// Analytics configuration

// WithAnalytics configures analytics settings
func (b *ValidatorBuilder) WithAnalytics(fn func(*AnalyticsValidationConfig)) *ValidatorBuilder {
	if b.config.Analytics == nil {
		b.config.Analytics = DefaultAnalyticsValidationConfig()
	}
	fn(b.config.Analytics)
	return b
}

// WithMetrics configures metrics collection
func (b *ValidatorBuilder) WithMetrics(enabled bool, provider string, interval time.Duration) *ValidatorBuilder {
	if b.config.Analytics == nil {
		b.config.Analytics = DefaultAnalyticsValidationConfig()
	}
	b.config.Analytics.MetricsEnabled = enabled
	b.config.Analytics.MetricsProvider = provider
	b.config.Analytics.MetricsInterval = interval
	return b
}

// WithPrometheusMetrics configures Prometheus metrics
func (b *ValidatorBuilder) WithPrometheusMetrics(enabled bool, path string, port int) *ValidatorBuilder {
	return b.WithMetrics(enabled, "prometheus", 30*time.Second).WithAnalytics(func(config *AnalyticsValidationConfig) {
		config.MetricsConfig = map[string]interface{}{
			"path": path,
			"port": port,
		}
	})
}

// WithTracing configures distributed tracing
func (b *ValidatorBuilder) WithTracing(enabled bool, provider string, samplingRate float64) *ValidatorBuilder {
	if b.config.Analytics == nil {
		b.config.Analytics = DefaultAnalyticsValidationConfig()
	}
	b.config.Analytics.TracingEnabled = enabled
	b.config.Analytics.TracingProvider = provider
	b.config.Analytics.SamplingRate = samplingRate
	return b
}

// WithJaegerTracing configures Jaeger tracing
func (b *ValidatorBuilder) WithJaegerTracing(enabled bool, endpoint string, samplingRate float64) *ValidatorBuilder {
	return b.WithTracing(enabled, "jaeger", samplingRate).WithAnalytics(func(config *AnalyticsValidationConfig) {
		config.TracingConfig = map[string]interface{}{
			"endpoint": endpoint,
		}
	})
}

// WithLogging configures logging
func (b *ValidatorBuilder) WithLogging(enabled bool, level, format, output string) *ValidatorBuilder {
	if b.config.Analytics == nil {
		b.config.Analytics = DefaultAnalyticsValidationConfig()
	}
	b.config.Analytics.LoggingEnabled = enabled
	b.config.Analytics.LogLevel = level
	b.config.Analytics.LogFormat = format
	b.config.Analytics.LogOutput = output
	return b
}

// Security configuration

// WithSecurity configures security settings
func (b *ValidatorBuilder) WithSecurity(fn func(*SecurityValidationConfig)) *ValidatorBuilder {
	if b.config.Security == nil {
		b.config.Security = DefaultSecurityValidationConfig()
	}
	fn(b.config.Security)
	return b
}

// WithSecurityEnabled enables or disables security validation
func (b *ValidatorBuilder) WithSecurityEnabled(enabled bool) *ValidatorBuilder {
	if b.config.Security == nil {
		b.config.Security = DefaultSecurityValidationConfig()
	}
	b.config.Security.Enabled = enabled
	return b
}

// WithInputSanitization configures input sanitization
func (b *ValidatorBuilder) WithInputSanitization(enabled bool, maxLength int, allowedTags []string) *ValidatorBuilder {
	if b.config.Security == nil {
		b.config.Security = DefaultSecurityValidationConfig()
	}
	b.config.Security.EnableInputSanitization = enabled
	b.config.Security.MaxContentLength = maxLength
	b.config.Security.AllowedHTMLTags = allowedTags
	return b
}

// WithRateLimiting configures rate limiting
func (b *ValidatorBuilder) WithRateLimiting(enabled bool, limit int, window time.Duration, burstSize int) *ValidatorBuilder {
	if b.config.Security == nil {
		b.config.Security = DefaultSecurityValidationConfig()
	}
	b.config.Security.EnableRateLimiting = enabled
	b.config.Security.RateLimit = limit
	b.config.Security.RateLimitWindow = window
	b.config.Security.RateLimitBurstSize = burstSize
	return b
}

// WithEncryption configures encryption
func (b *ValidatorBuilder) WithEncryption(enabled bool, algorithm, key string) *ValidatorBuilder {
	if b.config.Security == nil {
		b.config.Security = DefaultSecurityValidationConfig()
	}
	b.config.Security.EnableEncryption = enabled
	b.config.Security.EncryptionAlgorithm = algorithm
	b.config.Security.EncryptionKey = key
	return b
}

// Feature flags configuration

// WithFeatures configures feature flags
func (b *ValidatorBuilder) WithFeatures(fn func(*FeatureFlags)) *ValidatorBuilder {
	if b.config.Features == nil {
		b.config.Features = DefaultFeatureFlags()
	}
	fn(b.config.Features)
	return b
}

// WithExperimentalFeatures enables experimental features
func (b *ValidatorBuilder) WithExperimentalFeatures(enabled bool) *ValidatorBuilder {
	if b.config.Features == nil {
		b.config.Features = DefaultFeatureFlags()
	}
	b.config.Features.EnableExperimentalValidation = enabled
	b.config.Features.EnableAsyncValidation = enabled
	b.config.Features.EnableStreamValidation = enabled
	return b
}

// WithPerformanceFeatures enables performance features
func (b *ValidatorBuilder) WithPerformanceFeatures(enabled bool) *ValidatorBuilder {
	if b.config.Features == nil {
		b.config.Features = DefaultFeatureFlags()
	}
	b.config.Features.EnableLazyLoading = enabled
	b.config.Features.EnablePrefetching = enabled
	b.config.Features.EnableParallelization = enabled
	b.config.Features.EnableCompression = enabled
	return b
}

// WithDebugFeatures enables debug features
func (b *ValidatorBuilder) WithDebugFeatures(enabled bool) *ValidatorBuilder {
	if b.config.Features == nil {
		b.config.Features = DefaultFeatureFlags()
	}
	b.config.Features.EnableDebugMode = enabled
	b.config.Features.EnableVerboseLogging = enabled
	b.config.Features.EnableProfiling = enabled
	b.config.Features.EnableTraceMode = enabled
	return b
}

// Global settings configuration

// WithGlobal configures global settings
func (b *ValidatorBuilder) WithGlobal(fn func(*GlobalSettings)) *ValidatorBuilder {
	if b.config.Global == nil {
		b.config.Global = DefaultGlobalSettings()
	}
	fn(b.config.Global)
	return b
}

// WithEnvironment sets the environment
func (b *ValidatorBuilder) WithEnvironment(env, version, appID string) *ValidatorBuilder {
	if b.config.Global == nil {
		b.config.Global = DefaultGlobalSettings()
	}
	b.config.Global.Environment = env
	b.config.Global.Version = version
	b.config.Global.ApplicationID = appID
	return b
}

// WithResourceLimits configures resource limits
func (b *ValidatorBuilder) WithResourceLimits(maxMemory int64, maxCPU float64, maxDisk int64) *ValidatorBuilder {
	if b.config.Global == nil {
		b.config.Global = DefaultGlobalSettings()
	}
	b.config.Global.MaxMemoryUsage = maxMemory
	b.config.Global.MaxCPUUsage = maxCPU
	b.config.Global.MaxDiskUsage = maxDisk
	return b
}

// WithWorkerSettings configures worker settings
func (b *ValidatorBuilder) WithWorkerSettings(maxWorkers, queueSize int, shutdownTimeout time.Duration) *ValidatorBuilder {
	if b.config.Global == nil {
		b.config.Global = DefaultGlobalSettings()
	}
	b.config.Global.MaxWorkers = maxWorkers
	b.config.Global.WorkerQueueSize = queueSize
	b.config.Global.ShutdownTimeout = shutdownTimeout
	return b
}

// WithHealthCheck configures health check settings
func (b *ValidatorBuilder) WithHealthCheck(enabled bool, interval, timeout time.Duration) *ValidatorBuilder {
	if b.config.Global == nil {
		b.config.Global = DefaultGlobalSettings()
	}
	b.config.Global.HealthCheckEnabled = enabled
	b.config.Global.HealthCheckInterval = interval
	b.config.Global.HealthCheckTimeout = timeout
	return b
}

// Convenience methods for common configurations

// ForDevelopment configures the builder for development environment
func (b *ValidatorBuilder) ForDevelopment() *ValidatorBuilder {
	return b.
		WithEnvironment("development", "dev", "ag-ui-dev").
		WithValidationLevel(ValidationLevelDevelopment).
		WithAuthenticationEnabled(false).
		WithCacheEnabled(true).
		WithL1Cache(1000, 1*time.Minute).
		WithDistributedEnabled(false).
		WithLogging(true, "debug", "json", "stdout").
		WithDebugFeatures(true).
		WithSecurityEnabled(false)
}

// ForProduction configures the builder for production environment
func (b *ValidatorBuilder) ForProduction() *ValidatorBuilder {
	return b.
		WithEnvironment("production", "1.0.0", "ag-ui-prod").
		WithValidationLevel(ValidationLevelProduction).
		WithStrictValidation(true).
		WithAuthenticationEnabled(true).
		WithRBAC(true, []string{"user"}).
		WithCacheEnabled(true).
		WithL1Cache(50000, 5*time.Minute).
		WithDistributedEnabled(true).
		WithMetrics(true, "prometheus", 30*time.Second).
		WithTracing(true, "jaeger", 0.01).
		WithLogging(true, "warn", "json", "stdout").
		WithSecurityEnabled(true).
		WithRateLimiting(true, 1000, time.Minute, 100).
		WithPerformanceFeatures(true)
}

// ForTesting configures the builder for testing environment
func (b *ValidatorBuilder) ForTesting() *ValidatorBuilder {
	return b.
		WithEnvironment("testing", "test", "ag-ui-test").
		WithValidationLevel(ValidationLevelTesting).
		WithAuthenticationEnabled(false).
		WithCacheEnabled(false).
		WithDistributedEnabled(false).
		WithMetrics(false, "", 0).
		WithLogging(true, "error", "json", "stdout").
		WithSecurityEnabled(false).
		WithHealthCheck(false, 0, 0)
}

// Validation methods

// validateConfig validates the final configuration
func (b *ValidatorBuilder) validateConfig() error {
	config := b.config

	// Validate core configuration
	if config.Core != nil {
		if config.Core.ValidationTimeout <= 0 {
			return fmt.Errorf("core.validation_timeout must be positive")
		}
		if config.Core.MaxConcurrentValidations <= 0 {
			return fmt.Errorf("core.max_concurrent_validations must be positive")
		}
	}

	// Validate authentication configuration
	if config.Auth != nil && config.Auth.Enabled {
		if config.Auth.TokenExpiration <= 0 {
			return fmt.Errorf("auth.token_expiration must be positive when auth is enabled")
		}
		if config.Auth.RefreshEnabled && config.Auth.RefreshExpiration <= 0 {
			return fmt.Errorf("auth.refresh_expiration must be positive when refresh is enabled")
		}
	}

	// Validate cache configuration
	if config.Cache != nil && config.Cache.Enabled {
		if config.Cache.L1Enabled && config.Cache.L1Size <= 0 {
			return fmt.Errorf("cache.l1_size must be positive when L1 cache is enabled")
		}
		if config.Cache.L1Enabled && config.Cache.L1TTL <= 0 {
			return fmt.Errorf("cache.l1_ttl must be positive when L1 cache is enabled")
		}
		if config.Cache.L2Enabled && config.Cache.L2TTL <= 0 {
			return fmt.Errorf("cache.l2_ttl must be positive when L2 cache is enabled")
		}
	}

	// Validate distributed configuration
	if config.Distributed != nil && config.Distributed.Enabled {
		if config.Distributed.NodeID == "" {
			return fmt.Errorf("distributed.node_id is required when distributed validation is enabled")
		}
		if config.Distributed.ConsensusTimeout <= 0 {
			return fmt.Errorf("distributed.consensus_timeout must be positive")
		}
		if config.Distributed.MinNodes <= 0 {
			return fmt.Errorf("distributed.min_nodes must be positive")
		}
		if config.Distributed.MaxNodes < config.Distributed.MinNodes {
			return fmt.Errorf("distributed.max_nodes must be >= min_nodes")
		}
	}

	// Validate analytics configuration
	if config.Analytics != nil {
		if config.Analytics.TracingEnabled && (config.Analytics.SamplingRate < 0 || config.Analytics.SamplingRate > 1) {
			return fmt.Errorf("analytics.sampling_rate must be between 0 and 1")
		}
	}

	// Validate security configuration
	if config.Security != nil && config.Security.Enabled {
		if config.Security.EnableRateLimiting && config.Security.RateLimit <= 0 {
			return fmt.Errorf("security.rate_limit must be positive when rate limiting is enabled")
		}
		if config.Security.MaxContentLength <= 0 {
			return fmt.Errorf("security.max_content_length must be positive")
		}
	}

	// Validate global settings
	if config.Global != nil {
		if config.Global.MaxWorkers <= 0 {
			return fmt.Errorf("global.max_workers must be positive")
		}
		if config.Global.WorkerQueueSize <= 0 {
			return fmt.Errorf("global.worker_queue_size must be positive")
		}
	}

	return nil
}

// addError adds an error to the builder's error list
func (b *ValidatorBuilder) addError(err error) {
	b.errors = append(b.errors, err)
}

// Clone creates a copy of the builder
func (b *ValidatorBuilder) Clone() *ValidatorBuilder {
	// Note: This is a shallow clone. For deep cloning, we'd need to implement
	// proper deep copy for the nested structs
	return &ValidatorBuilder{
		config: b.config,
		errors: make([]error, len(b.errors)),
	}
}
