package di

import "time"

// Config interfaces to avoid circular imports

// ValidatorConfigInterface defines the interface for validator configuration
type ValidatorConfigInterface interface {
	GetCore() CoreConfigInterface
	GetAuth() AuthConfigInterface  
	GetCache() CacheConfigInterface
	GetDistributed() DistributedConfigInterface
	GetAnalytics() AnalyticsConfigInterface
	GetSecurity() SecurityConfigInterface
}

// CoreConfigInterface defines the interface for core validation configuration
type CoreConfigInterface interface {
	GetLevel() string
	GetStrict() bool
	GetValidationTimeout() time.Duration
	GetMaxConcurrentValidations() int
}

// AuthConfigInterface defines the interface for authentication configuration
type AuthConfigInterface interface {
	IsEnabled() bool
	GetProviderType() string
	GetProviderConfig() map[string]interface{}
	GetTokenExpiration() time.Duration
}

// CacheConfigInterface defines the interface for cache configuration
type CacheConfigInterface interface {
	IsEnabled() bool
	GetL1Size() int
	GetL1TTL() time.Duration
	IsL2Enabled() bool
	GetL2Provider() string
	GetL2Config() map[string]interface{}
	IsCompressionEnabled() bool
	GetNodeID() string
	IsClusterMode() bool
}

// DistributedConfigInterface defines the interface for distributed configuration
type DistributedConfigInterface interface {
	IsEnabled() bool
	GetNodeID() string
	GetNodeRole() string
	GetConsensusAlgorithm() string
	GetConsensusTimeout() time.Duration
	GetMinNodes() int
	GetMaxNodes() int
	GetListenAddress() string
}

// AnalyticsConfigInterface defines the interface for analytics configuration  
type AnalyticsConfigInterface interface {
	IsEnabled() bool
	IsMetricsEnabled() bool
	GetMetricsProvider() string
	GetMetricsInterval() time.Duration
	IsTracingEnabled() bool
	GetTracingProvider() string
	GetSamplingRate() float64
	IsLoggingEnabled() bool
	GetLogLevel() string
}

// SecurityConfigInterface defines the interface for security configuration
type SecurityConfigInterface interface {
	IsEnabled() bool
	IsInputSanitizationEnabled() bool
	GetMaxContentLength() int
	IsRateLimitingEnabled() bool
	GetRateLimit() int
	GetRateLimitWindow() time.Duration
}