package config

import (
	"time"
)

// ValidatorConfig is the unified configuration structure for all validation components
type ValidatorConfig struct {
	// Core validation configuration
	Core *CoreValidationConfig `json:"core"`

	// Authentication configuration
	Auth *AuthValidationConfig `json:"auth,omitempty"`

	// Cache configuration
	Cache *CacheValidationConfig `json:"cache,omitempty"`

	// Distributed validation configuration
	Distributed *DistributedValidationConfig `json:"distributed,omitempty"`

	// Analytics configuration
	Analytics *AnalyticsValidationConfig `json:"analytics,omitempty"`

	// Security configuration
	Security *SecurityValidationConfig `json:"security,omitempty"`

	// Feature flags for experimental features
	Features *FeatureFlags `json:"features,omitempty"`

	// Global settings
	Global *GlobalSettings `json:"global,omitempty"`
}

// CoreValidationConfig contains core validation settings
type CoreValidationConfig struct {
	// Level defines the validation level (strict, permissive, custom)
	Level ValidationLevel `json:"level"`

	// Strict determines if validation is strict
	Strict bool `json:"strict"`

	// Skip validations
	SkipTimestampValidation bool `json:"skip_timestamp_validation"`
	SkipSequenceValidation  bool `json:"skip_sequence_validation"`
	SkipFieldValidation     bool `json:"skip_field_validation"`

	// Allow options
	AllowEmptyIDs          bool `json:"allow_empty_ids"`
	AllowUnknownEventTypes bool `json:"allow_unknown_event_types"`

	// Timeout settings
	ValidationTimeout time.Duration `json:"validation_timeout"`

	// Concurrency settings
	MaxConcurrentValidations int `json:"max_concurrent_validations"`
}

// AuthValidationConfig contains authentication validation settings
type AuthValidationConfig struct {
	// Enable authentication
	Enabled bool `json:"enabled"`

	// Require authentication for all operations
	RequireAuth bool `json:"require_auth"`

	// Allow anonymous access
	AllowAnonymous bool `json:"allow_anonymous"`

	// Token settings
	TokenExpiration   time.Duration `json:"token_expiration"`
	RefreshEnabled    bool          `json:"refresh_enabled"`
	RefreshExpiration time.Duration `json:"refresh_expiration"`

	// Provider configuration
	ProviderType   string                 `json:"provider_type"`
	ProviderConfig map[string]interface{} `json:"provider_config,omitempty"`

	// Security settings
	MaxLoginAttempts    int           `json:"max_login_attempts"`
	LockoutDuration     time.Duration `json:"lockout_duration"`
	SessionTimeout      time.Duration `json:"session_timeout"`
	RequireSecureTokens bool          `json:"require_secure_tokens"`

	// RBAC settings
	EnableRBAC    bool     `json:"enable_rbac"`
	DefaultRoles  []string `json:"default_roles"`
	RequiredRoles []string `json:"required_roles,omitempty"`
	RequiredPerms []string `json:"required_permissions,omitempty"`
}

// CacheValidationConfig contains cache validation settings
type CacheValidationConfig struct {
	// Enable caching
	Enabled bool `json:"enabled"`

	// L1 Cache (in-memory) settings
	L1Size    int           `json:"l1_size"`
	L1TTL     time.Duration `json:"l1_ttl"`
	L1Enabled bool          `json:"l1_enabled"`

	// L2 Cache (distributed) settings
	L2TTL      time.Duration          `json:"l2_ttl"`
	L2Enabled  bool                   `json:"l2_enabled"`
	L2Provider string                 `json:"l2_provider"` // "redis", "memcached", "etcd"
	L2Config   map[string]interface{} `json:"l2_config,omitempty"`

	// Compression settings
	CompressionEnabled bool   `json:"compression_enabled"`
	CompressionLevel   int    `json:"compression_level"`
	CompressionType    string `json:"compression_type"` // "gzip", "lz4", "snappy"

	// Invalidation settings
	InvalidationStrategy string        `json:"invalidation_strategy"` // "ttl", "event_driven", "manual"
	InvalidationDelay    time.Duration `json:"invalidation_delay"`

	// Coordination settings
	NodeID      string `json:"node_id,omitempty"`
	ClusterMode bool   `json:"cluster_mode"`

	// Metrics
	MetricsEnabled  bool          `json:"metrics_enabled"`
	MetricsInterval time.Duration `json:"metrics_interval"`

	// Warmup settings
	WarmupEnabled    bool     `json:"warmup_enabled"`
	WarmupEventTypes []string `json:"warmup_event_types,omitempty"`
}

// DistributedValidationConfig contains distributed validation settings
type DistributedValidationConfig struct {
	// Enable distributed validation
	Enabled bool `json:"enabled"`

	// Node configuration
	NodeID   string `json:"node_id"`
	NodeRole string `json:"node_role"` // "leader", "follower", "observer"

	// Consensus configuration
	ConsensusAlgorithm string        `json:"consensus_algorithm"` // "raft", "pbft", "majority"
	ConsensusTimeout   time.Duration `json:"consensus_timeout"`
	MinNodes           int           `json:"min_nodes"`
	MaxNodes           int           `json:"max_nodes"`
	RequireUnanimous   bool          `json:"require_unanimous"`

	// Network configuration
	ListenAddress     string        `json:"listen_address"`
	AdvertiseAddress  string        `json:"advertise_address"`
	MaxConnections    int           `json:"max_connections"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`

	// Load balancing
	LoadBalanceStrategy string  `json:"load_balance_strategy"` // "round_robin", "weighted", "least_loaded"
	LoadThreshold       float64 `json:"load_threshold"`

	// Partition handling
	PartitionTolerance   bool          `json:"partition_tolerance"`
	AllowLocalValidation bool          `json:"allow_local_validation"`
	PartitionTimeout     time.Duration `json:"partition_timeout"`

	// State synchronization
	StateSyncEnabled  bool          `json:"state_sync_enabled"`
	StateSyncInterval time.Duration `json:"state_sync_interval"`
	StateSyncProtocol string        `json:"state_sync_protocol"` // "gossip", "merkle", "full"

	// Failure handling
	MaxNodeFailures      int           `json:"max_node_failures"`
	FailureDetectTimeout time.Duration `json:"failure_detect_timeout"`
	RecoveryEnabled      bool          `json:"recovery_enabled"`

	// Security
	EnableTLS       bool   `json:"enable_tls"`
	TLSCertFile     string `json:"tls_cert_file,omitempty"`
	TLSKeyFile      string `json:"tls_key_file,omitempty"`
	TLSCAFile       string `json:"tls_ca_file,omitempty"`
	EnableMutualTLS bool   `json:"enable_mutual_tls"`
}

// AnalyticsValidationConfig contains analytics and monitoring settings
type AnalyticsValidationConfig struct {
	// Enable analytics
	Enabled bool `json:"enabled"`

	// Metrics collection
	MetricsEnabled  bool                   `json:"metrics_enabled"`
	MetricsInterval time.Duration          `json:"metrics_interval"`
	MetricsProvider string                 `json:"metrics_provider"` // "prometheus", "statsd", "custom"
	MetricsConfig   map[string]interface{} `json:"metrics_config,omitempty"`

	// Tracing
	TracingEnabled  bool                   `json:"tracing_enabled"`
	TracingProvider string                 `json:"tracing_provider"` // "jaeger", "zipkin", "otlp"
	TracingConfig   map[string]interface{} `json:"tracing_config,omitempty"`
	SamplingRate    float64                `json:"sampling_rate"`

	// Logging
	LoggingEnabled bool   `json:"logging_enabled"`
	LogLevel       string `json:"log_level"`  // "debug", "info", "warn", "error"
	LogFormat      string `json:"log_format"` // "json", "text"
	LogOutput      string `json:"log_output"` // "stdout", "file", "syslog"
	LogFilePath    string `json:"log_file_path,omitempty"`

	// Performance monitoring
	PerformanceEnabled   bool          `json:"performance_enabled"`
	PerformanceThreshold time.Duration `json:"performance_threshold"`
	SlowQueryThreshold   time.Duration `json:"slow_query_threshold"`

	// Alerting
	AlertingEnabled  bool                   `json:"alerting_enabled"`
	AlertingProvider string                 `json:"alerting_provider"` // "webhook", "email", "slack"
	AlertingConfig   map[string]interface{} `json:"alerting_config,omitempty"`
	AlertThresholds  map[string]float64     `json:"alert_thresholds,omitempty"`

	// Data retention
	RetentionEnabled bool          `json:"retention_enabled"`
	RetentionPeriod  time.Duration `json:"retention_period"`

	// Export settings
	ExportEnabled     bool                   `json:"export_enabled"`
	ExportFormat      string                 `json:"export_format"`      // "json", "csv", "parquet"
	ExportDestination string                 `json:"export_destination"` // "file", "s3", "gcs"
	ExportConfig      map[string]interface{} `json:"export_config,omitempty"`
}

// SecurityValidationConfig contains security validation settings
type SecurityValidationConfig struct {
	// Enable security validation
	Enabled bool `json:"enabled"`

	// Input sanitization
	EnableInputSanitization bool     `json:"enable_input_sanitization"`
	MaxContentLength        int      `json:"max_content_length"`
	AllowedHTMLTags         []string `json:"allowed_html_tags,omitempty"`

	// SQL injection protection
	EnableSQLInjectionProtection bool `json:"enable_sql_injection_protection"`

	// XSS protection
	EnableXSSProtection bool `json:"enable_xss_protection"`

	// CSRF protection
	EnableCSRFProtection bool   `json:"enable_csrf_protection"`
	CSRFTokenName        string `json:"csrf_token_name"`

	// Rate limiting
	EnableRateLimiting bool          `json:"enable_rate_limiting"`
	RateLimit          int           `json:"rate_limit"` // requests per window
	RateLimitWindow    time.Duration `json:"rate_limit_window"`
	RateLimitBurstSize int           `json:"rate_limit_burst_size"`

	// IP filtering
	EnableIPFiltering bool     `json:"enable_ip_filtering"`
	AllowedIPs        []string `json:"allowed_ips,omitempty"`
	BlockedIPs        []string `json:"blocked_ips,omitempty"`

	// Content validation
	EnableContentValidation bool     `json:"enable_content_validation"`
	AllowedContentTypes     []string `json:"allowed_content_types,omitempty"`
	MaxUploadSize           int64    `json:"max_upload_size"`

	// Encryption
	EnableEncryption    bool   `json:"enable_encryption"`
	EncryptionAlgorithm string `json:"encryption_algorithm"` // "AES256", "ChaCha20"
	EncryptionKey       string `json:"encryption_key,omitempty"`

	// Audit settings
	EnableAuditLogging  bool                   `json:"enable_audit_logging"`
	AuditLogDestination string                 `json:"audit_log_destination"` // "file", "database", "syslog"
	AuditLogConfig      map[string]interface{} `json:"audit_log_config,omitempty"`
}

// FeatureFlags contains feature flag settings for experimental features
type FeatureFlags struct {
	// Experimental features
	EnableExperimentalValidation bool `json:"enable_experimental_validation"`
	EnableAsyncValidation        bool `json:"enable_async_validation"`
	EnableBatchValidation        bool `json:"enable_batch_validation"`
	EnableStreamValidation       bool `json:"enable_stream_validation"`

	// Performance features
	EnableLazyLoading     bool `json:"enable_lazy_loading"`
	EnablePrefetching     bool `json:"enable_prefetching"`
	EnableParallelization bool `json:"enable_parallelization"`
	EnableCompression     bool `json:"enable_compression"`

	// Debug features
	EnableDebugMode      bool `json:"enable_debug_mode"`
	EnableVerboseLogging bool `json:"enable_verbose_logging"`
	EnableProfiling      bool `json:"enable_profiling"`
	EnableTraceMode      bool `json:"enable_trace_mode"`
}

// GlobalSettings contains global configuration settings
type GlobalSettings struct {
	// Application settings
	Environment   string `json:"environment"` // "development", "staging", "production"
	Version       string `json:"version"`
	ApplicationID string `json:"application_id"`

	// Resource limits
	MaxMemoryUsage int64         `json:"max_memory_usage"` // in bytes
	MaxCPUUsage    float64       `json:"max_cpu_usage"`    // percentage
	MaxDiskUsage   int64         `json:"max_disk_usage"`   // in bytes
	DefaultTimeout time.Duration `json:"default_timeout"`

	// Networking
	MaxConnections    int           `json:"max_connections"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`

	// Worker settings
	MaxWorkers      int           `json:"max_workers"`
	WorkerQueueSize int           `json:"worker_queue_size"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`

	// Health check settings
	HealthCheckEnabled  bool          `json:"health_check_enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`

	// Graceful shutdown
	GracefulShutdownEnabled bool          `json:"graceful_shutdown_enabled"`
	GracefulShutdownTimeout time.Duration `json:"graceful_shutdown_timeout"`
}

// ValidationLevel defines the level of validation to apply
type ValidationLevel string

const (
	// ValidationLevelStrict applies all validation rules strictly
	ValidationLevelStrict ValidationLevel = "strict"

	// ValidationLevelPermissive applies minimal validation rules
	ValidationLevelPermissive ValidationLevel = "permissive"

	// ValidationLevelCustom allows custom validation rules
	ValidationLevelCustom ValidationLevel = "custom"

	// ValidationLevelDevelopment is optimized for development
	ValidationLevelDevelopment ValidationLevel = "development"

	// ValidationLevelProduction is optimized for production
	ValidationLevelProduction ValidationLevel = "production"

	// ValidationLevelTesting is optimized for testing
	ValidationLevelTesting ValidationLevel = "testing"
)

// Default configurations

// DefaultValidatorConfig returns a default validator configuration
func DefaultValidatorConfig() *ValidatorConfig {
	return &ValidatorConfig{
		Core:        DefaultCoreValidationConfig(),
		Auth:        DefaultAuthValidationConfig(),
		Cache:       DefaultCacheValidationConfig(),
		Distributed: DefaultDistributedValidationConfig(),
		Analytics:   DefaultAnalyticsValidationConfig(),
		Security:    DefaultSecurityValidationConfig(),
		Features:    DefaultFeatureFlags(),
		Global:      DefaultGlobalSettings(),
	}
}

// DefaultCoreValidationConfig returns default core validation configuration
func DefaultCoreValidationConfig() *CoreValidationConfig {
	return &CoreValidationConfig{
		Level:                    ValidationLevelStrict,
		Strict:                   true,
		SkipTimestampValidation:  false,
		SkipSequenceValidation:   false,
		SkipFieldValidation:      false,
		AllowEmptyIDs:            false,
		AllowUnknownEventTypes:   false,
		ValidationTimeout:        30 * time.Second,
		MaxConcurrentValidations: 100,
	}
}

// DefaultAuthValidationConfig returns default auth validation configuration
func DefaultAuthValidationConfig() *AuthValidationConfig {
	return &AuthValidationConfig{
		Enabled:             false, // Disabled by default for backward compatibility
		RequireAuth:         false,
		AllowAnonymous:      true,
		TokenExpiration:     24 * time.Hour,
		RefreshEnabled:      true,
		RefreshExpiration:   7 * 24 * time.Hour,
		ProviderType:        "basic",
		ProviderConfig:      make(map[string]interface{}),
		MaxLoginAttempts:    5,
		LockoutDuration:     15 * time.Minute,
		SessionTimeout:      2 * time.Hour,
		RequireSecureTokens: false,
		EnableRBAC:          false,
		DefaultRoles:        []string{"user"},
	}
}

// DefaultCacheValidationConfig returns default cache validation configuration
func DefaultCacheValidationConfig() *CacheValidationConfig {
	return &CacheValidationConfig{
		Enabled:              false, // Disabled by default
		L1Size:               10000,
		L1TTL:                5 * time.Minute,
		L1Enabled:            true,
		L2TTL:                30 * time.Minute,
		L2Enabled:            false,
		L2Provider:           "memory",
		L2Config:             make(map[string]interface{}),
		CompressionEnabled:   true,
		CompressionLevel:     6,
		CompressionType:      "gzip",
		InvalidationStrategy: "ttl",
		InvalidationDelay:    1 * time.Second,
		ClusterMode:          false,
		MetricsEnabled:       true,
		MetricsInterval:      30 * time.Second,
		WarmupEnabled:        false,
	}
}

// DefaultDistributedValidationConfig returns default distributed validation configuration
func DefaultDistributedValidationConfig() *DistributedValidationConfig {
	return &DistributedValidationConfig{
		Enabled:              false, // Disabled by default
		NodeID:               "default-node-id",
		NodeRole:             "follower",
		ConsensusAlgorithm:   "majority",
		ConsensusTimeout:     5 * time.Second,
		MinNodes:             1,
		MaxNodes:             10,
		RequireUnanimous:     false,
		ListenAddress:        ":8080",
		MaxConnections:       100,
		ConnectionTimeout:    10 * time.Second,
		HeartbeatInterval:    1 * time.Second,
		LoadBalanceStrategy:  "round_robin",
		LoadThreshold:        0.8,
		PartitionTolerance:   true,
		AllowLocalValidation: true,
		PartitionTimeout:     30 * time.Second,
		StateSyncEnabled:     true,
		StateSyncInterval:    10 * time.Second,
		StateSyncProtocol:    "gossip",
		MaxNodeFailures:      2,
		FailureDetectTimeout: 5 * time.Second,
		RecoveryEnabled:      true,
		EnableTLS:            false,
		EnableMutualTLS:      false,
	}
}

// DefaultAnalyticsValidationConfig returns default analytics validation configuration
func DefaultAnalyticsValidationConfig() *AnalyticsValidationConfig {
	return &AnalyticsValidationConfig{
		Enabled:              true,
		MetricsEnabled:       true,
		MetricsInterval:      30 * time.Second,
		MetricsProvider:      "prometheus",
		MetricsConfig:        make(map[string]interface{}),
		TracingEnabled:       false,
		TracingProvider:      "jaeger",
		TracingConfig:        make(map[string]interface{}),
		SamplingRate:         0.1,
		LoggingEnabled:       true,
		LogLevel:             "info",
		LogFormat:            "json",
		LogOutput:            "stdout",
		PerformanceEnabled:   true,
		PerformanceThreshold: 1 * time.Second,
		SlowQueryThreshold:   500 * time.Millisecond,
		AlertingEnabled:      false,
		AlertingProvider:     "webhook",
		AlertingConfig:       make(map[string]interface{}),
		RetentionEnabled:     false,
		RetentionPeriod:      7 * 24 * time.Hour,
		ExportEnabled:        false,
		ExportFormat:         "json",
		ExportDestination:    "file",
		ExportConfig:         make(map[string]interface{}),
	}
}

// DefaultSecurityValidationConfig returns default security validation configuration
func DefaultSecurityValidationConfig() *SecurityValidationConfig {
	return &SecurityValidationConfig{
		Enabled:                      true,
		EnableInputSanitization:      true,
		MaxContentLength:             1024 * 1024, // 1MB
		AllowedHTMLTags:              []string{},
		EnableSQLInjectionProtection: true,
		EnableXSSProtection:          true,
		EnableCSRFProtection:         false,
		CSRFTokenName:                "_csrf_token",
		EnableRateLimiting:           false,
		RateLimit:                    1000,
		RateLimitWindow:              time.Minute,
		RateLimitBurstSize:           100,
		EnableIPFiltering:            false,
		EnableContentValidation:      true,
		MaxUploadSize:                10 * 1024 * 1024, // 10MB
		EnableEncryption:             false,
		EncryptionAlgorithm:          "AES256",
		EnableAuditLogging:           false,
		AuditLogDestination:          "file",
		AuditLogConfig:               make(map[string]interface{}),
	}
}

// DefaultFeatureFlags returns default feature flags
func DefaultFeatureFlags() *FeatureFlags {
	return &FeatureFlags{
		EnableExperimentalValidation: false,
		EnableAsyncValidation:        false,
		EnableBatchValidation:        true,
		EnableStreamValidation:       false,
		EnableLazyLoading:            false,
		EnablePrefetching:            false,
		EnableParallelization:        true,
		EnableCompression:            true,
		EnableDebugMode:              false,
		EnableVerboseLogging:         false,
		EnableProfiling:              false,
		EnableTraceMode:              false,
	}
}

// DefaultGlobalSettings returns default global settings
func DefaultGlobalSettings() *GlobalSettings {
	return &GlobalSettings{
		Environment:             "development",
		Version:                 "1.0.0",
		MaxMemoryUsage:          1024 * 1024 * 1024,      // 1GB
		MaxCPUUsage:             0.8,                     // 80%
		MaxDiskUsage:            10 * 1024 * 1024 * 1024, // 10GB
		DefaultTimeout:          30 * time.Second,
		MaxConnections:          1000,
		ConnectionTimeout:       10 * time.Second,
		ReadTimeout:             30 * time.Second,
		WriteTimeout:            30 * time.Second,
		MaxWorkers:              10,
		WorkerQueueSize:         1000,
		ShutdownTimeout:         30 * time.Second,
		HealthCheckEnabled:      true,
		HealthCheckInterval:     30 * time.Second,
		HealthCheckTimeout:      5 * time.Second,
		GracefulShutdownEnabled: true,
		GracefulShutdownTimeout: 30 * time.Second,
	}
}

// Environment-specific configurations

// DevelopmentValidatorConfig returns configuration optimized for development
func DevelopmentValidatorConfig() *ValidatorConfig {
	config := DefaultValidatorConfig()

	// More permissive core validation
	config.Core.Level = ValidationLevelDevelopment
	config.Core.SkipTimestampValidation = true
	config.Core.AllowEmptyIDs = true
	config.Core.ValidationTimeout = 60 * time.Second

	// Disable authentication by default in development
	config.Auth.Enabled = false

	// Enable caching with shorter TTLs
	config.Cache.Enabled = true
	config.Cache.L1TTL = 1 * time.Minute
	config.Cache.L2TTL = 5 * time.Minute

	// Disable distributed validation in development
	config.Distributed.Enabled = false

	// Enable verbose logging and debugging
	config.Analytics.LogLevel = "debug"
	config.Analytics.TracingEnabled = true
	config.Analytics.SamplingRate = 1.0 // 100% sampling in dev

	// Relaxed security
	config.Security.EnableRateLimiting = false
	config.Security.MaxContentLength = 10 * 1024 * 1024 // 10MB

	// Enable debug features
	config.Features.EnableDebugMode = true
	config.Features.EnableVerboseLogging = true
	config.Features.EnableTraceMode = true

	// Development global settings
	config.Global.Environment = "development"
	config.Global.HealthCheckInterval = 10 * time.Second

	return config
}

// ProductionValidatorConfig returns configuration optimized for production
func ProductionValidatorConfig() *ValidatorConfig {
	config := DefaultValidatorConfig()

	// Strict core validation
	config.Core.Level = ValidationLevelProduction
	config.Core.Strict = true
	config.Core.ValidationTimeout = 10 * time.Second
	config.Core.MaxConcurrentValidations = 1000

	// Enable authentication
	config.Auth.Enabled = true
	config.Auth.RequireAuth = true
	config.Auth.AllowAnonymous = false
	config.Auth.RequireSecureTokens = true
	config.Auth.EnableRBAC = true

	// Enable caching with production TTLs
	config.Cache.Enabled = true
	config.Cache.L1Size = 50000
	config.Cache.L2Enabled = true
	config.Cache.ClusterMode = true
	config.Cache.WarmupEnabled = true

	// Enable distributed validation
	config.Distributed.Enabled = true
	config.Distributed.NodeRole = "leader"
	config.Distributed.EnableTLS = true
	config.Distributed.EnableMutualTLS = true

	// Production analytics
	config.Analytics.TracingEnabled = true
	config.Analytics.SamplingRate = 0.01 // 1% sampling in production
	config.Analytics.LogLevel = "warn"
	config.Analytics.AlertingEnabled = true
	config.Analytics.RetentionEnabled = true
	config.Analytics.ExportEnabled = true

	// Enhanced security
	config.Security.EnableRateLimiting = true
	config.Security.EnableIPFiltering = true
	config.Security.EnableEncryption = true
	config.Security.EnableAuditLogging = true

	// Disable debug features
	config.Features.EnableDebugMode = false
	config.Features.EnableVerboseLogging = false
	config.Features.EnableTraceMode = false

	// Production global settings
	config.Global.Environment = "production"
	config.Global.MaxWorkers = 50
	config.Global.WorkerQueueSize = 10000

	return config
}

// TestingValidatorConfig returns configuration optimized for testing
func TestingValidatorConfig() *ValidatorConfig {
	config := DefaultValidatorConfig()

	// Testing-friendly core validation
	config.Core.Level = ValidationLevelTesting
	config.Core.SkipTimestampValidation = true
	config.Core.SkipSequenceValidation = true
	config.Core.AllowEmptyIDs = true
	config.Core.ValidationTimeout = 5 * time.Second

	// Disable authentication in tests
	config.Auth.Enabled = false

	// Fast cache with short TTLs for testing
	config.Cache.Enabled = true
	config.Cache.L1Size = 1000
	config.Cache.L1TTL = 10 * time.Second
	config.Cache.L2Enabled = false

	// Disable distributed validation in tests
	config.Distributed.Enabled = false

	// Minimal analytics
	config.Analytics.MetricsEnabled = false
	config.Analytics.TracingEnabled = false
	config.Analytics.LogLevel = "error"
	config.Analytics.LogOutput = "stdout"

	// Relaxed security for testing
	config.Security.EnableRateLimiting = false
	config.Security.EnableAuditLogging = false

	// Enable test-friendly features
	config.Features.EnableDebugMode = true
	config.Features.EnableVerboseLogging = false // Keep logs clean in tests

	// Testing global settings
	config.Global.Environment = "testing"
	config.Global.DefaultTimeout = 5 * time.Second
	config.Global.HealthCheckEnabled = false

	return config
}
