package config

import (
	"time"
)

// AuthConfigCompat provides backward compatibility for the existing AuthConfig
// This allows existing code to continue working while migrating to the unified configuration
type AuthConfigCompat struct {
	// Enabled determines if authentication is enabled
	Enabled bool `json:"enabled"`

	// RequireAuth determines if authentication is required for all operations
	RequireAuth bool `json:"require_auth"`

	// AllowAnonymous allows anonymous access for certain operations
	AllowAnonymous bool `json:"allow_anonymous"`

	// TokenExpiration is the default token expiration duration
	TokenExpiration time.Duration `json:"token_expiration"`

	// RefreshEnabled allows token refresh
	RefreshEnabled bool `json:"refresh_enabled"`

	// RefreshExpiration is the refresh token expiration duration
	RefreshExpiration time.Duration `json:"refresh_expiration"`

	// ProviderConfig contains provider-specific configuration
	ProviderConfig map[string]interface{} `json:"provider_config,omitempty"`
}

// ToUnifiedConfig converts the legacy AuthConfig to the unified AuthValidationConfig
func (ac *AuthConfigCompat) ToUnifiedConfig() *AuthValidationConfig {
	return &AuthValidationConfig{
		Enabled:           ac.Enabled,
		RequireAuth:       ac.RequireAuth,
		AllowAnonymous:    ac.AllowAnonymous,
		TokenExpiration:   ac.TokenExpiration,
		RefreshEnabled:    ac.RefreshEnabled,
		RefreshExpiration: ac.RefreshExpiration,
		ProviderType:      "basic", // Default to basic auth
		ProviderConfig:    ac.ProviderConfig,

		// Set sensible defaults for new fields
		MaxLoginAttempts:    5,
		LockoutDuration:     15 * time.Minute,
		SessionTimeout:      2 * time.Hour,
		RequireSecureTokens: false,
		EnableRBAC:          false,
		DefaultRoles:        []string{"user"},
	}
}

// FromUnifiedConfig creates a legacy AuthConfig from the unified configuration
func (ac *AuthConfigCompat) FromUnifiedConfig(unified *AuthValidationConfig) {
	ac.Enabled = unified.Enabled
	ac.RequireAuth = unified.RequireAuth
	ac.AllowAnonymous = unified.AllowAnonymous
	ac.TokenExpiration = unified.TokenExpiration
	ac.RefreshEnabled = unified.RefreshEnabled
	ac.RefreshExpiration = unified.RefreshExpiration
	ac.ProviderConfig = unified.ProviderConfig
}

// DefaultAuthConfigCompat returns the default legacy authentication configuration
func DefaultAuthConfigCompat() *AuthConfigCompat {
	return &AuthConfigCompat{
		Enabled:           true,
		RequireAuth:       false,
		AllowAnonymous:    true,
		TokenExpiration:   24 * time.Hour,
		RefreshEnabled:    true,
		RefreshExpiration: 7 * 24 * time.Hour,
		ProviderConfig:    make(map[string]interface{}),
	}
}

// CacheValidatorConfigCompat provides backward compatibility for cache configuration
type CacheValidatorConfigCompat struct {
	// L1 Cache settings
	L1Size int
	L1TTL  time.Duration

	// L2 Cache settings
	L2Cache   interface{} // DistributedCache interface
	L2TTL     time.Duration
	L2Enabled bool

	// Validation settings
	Validator interface{} // *events.Validator

	// Compression settings
	CompressionEnabled bool
	CompressionLevel   int

	// Invalidation strategies
	InvalidationStrategies []interface{} // []InvalidationStrategy

	// Coordination settings
	NodeID      string
	Coordinator interface{} // *CacheCoordinator

	// Metrics
	MetricsEnabled bool
}

// ToUnifiedConfig converts the legacy CacheValidatorConfig to the unified configuration
func (cc *CacheValidatorConfigCompat) ToUnifiedConfig() *CacheValidationConfig {
	return &CacheValidationConfig{
		Enabled:              true, // Assume enabled if config exists
		L1Size:               cc.L1Size,
		L1TTL:                cc.L1TTL,
		L1Enabled:            cc.L1Size > 0,
		L2TTL:                cc.L2TTL,
		L2Enabled:            cc.L2Enabled,
		L2Provider:           "custom", // Legacy uses custom provider
		L2Config:             make(map[string]interface{}),
		CompressionEnabled:   cc.CompressionEnabled,
		CompressionLevel:     cc.CompressionLevel,
		CompressionType:      "gzip", // Default compression type
		InvalidationStrategy: "ttl",  // Default strategy
		InvalidationDelay:    1 * time.Second,
		NodeID:               cc.NodeID,
		ClusterMode:          cc.Coordinator != nil,
		MetricsEnabled:       cc.MetricsEnabled,
		MetricsInterval:      30 * time.Second,
		WarmupEnabled:        false,
	}
}

// DistributedValidatorConfigCompat provides backward compatibility for distributed configuration
type DistributedValidatorConfigCompat struct {
	// NodeID is the unique identifier for this validator node
	NodeID string

	// ConsensusConfig contains consensus algorithm configuration
	ConsensusConfig interface{} // *ConsensusConfig

	// StateSync contains state synchronization configuration
	StateSync interface{} // *StateSyncConfig

	// LoadBalancer contains load balancing configuration
	LoadBalancer interface{} // *LoadBalancerConfig

	// PartitionHandler contains partition handling configuration
	PartitionHandler interface{} // *PartitionHandlerConfig

	// MaxNodeFailures is the maximum number of node failures to tolerate
	MaxNodeFailures int

	// ValidationTimeout is the timeout for validation operations
	ValidationTimeout time.Duration

	// HeartbeatInterval is the interval between heartbeats
	HeartbeatInterval time.Duration

	// EnableMetrics enables distributed metrics collection
	EnableMetrics bool
}

// ToUnifiedConfig converts the legacy DistributedValidatorConfig to the unified configuration
func (dc *DistributedValidatorConfigCompat) ToUnifiedConfig() *DistributedValidationConfig {
	return &DistributedValidationConfig{
		Enabled:              true, // Assume enabled if config exists
		NodeID:               dc.NodeID,
		NodeRole:             "follower", // Default role
		ConsensusAlgorithm:   "majority", // Default algorithm
		ConsensusTimeout:     dc.ValidationTimeout,
		MinNodes:             1,
		MaxNodes:             10,
		RequireUnanimous:     false,
		ListenAddress:        ":8080", // Default address
		MaxConnections:       100,
		ConnectionTimeout:    10 * time.Second,
		HeartbeatInterval:    dc.HeartbeatInterval,
		LoadBalanceStrategy:  "round_robin",
		LoadThreshold:        0.8,
		PartitionTolerance:   true,
		AllowLocalValidation: true,
		PartitionTimeout:     30 * time.Second,
		StateSyncEnabled:     true,
		StateSyncInterval:    10 * time.Second,
		StateSyncProtocol:    "gossip",
		MaxNodeFailures:      dc.MaxNodeFailures,
		FailureDetectTimeout: 5 * time.Second,
		RecoveryEnabled:      true,
		EnableTLS:            false,
		EnableMutualTLS:      false,
	}
}

// ConfigMigrator provides utilities for migrating configurations
type ConfigMigrator struct{}

// NewConfigMigrator creates a new configuration migrator
func NewConfigMigrator() *ConfigMigrator {
	return &ConfigMigrator{}
}

// MigrateFromLegacy migrates from legacy configuration patterns to unified configuration
func (cm *ConfigMigrator) MigrateFromLegacy(
	authConfig *AuthConfigCompat,
	cacheConfig *CacheValidatorConfigCompat,
	distributedConfig *DistributedValidatorConfigCompat,
) *ValidatorConfig {

	config := DefaultValidatorConfig()

	// Migrate auth configuration if provided
	if authConfig != nil {
		config.Auth = authConfig.ToUnifiedConfig()
	}

	// Migrate cache configuration if provided
	if cacheConfig != nil {
		config.Cache = cacheConfig.ToUnifiedConfig()
	}

	// Migrate distributed configuration if provided
	if distributedConfig != nil {
		config.Distributed = distributedConfig.ToUnifiedConfig()
	}

	return config
}

// MigrateToLegacy converts unified configuration back to legacy format
func (cm *ConfigMigrator) MigrateToLegacy(unified *ValidatorConfig) (
	*AuthConfigCompat,
	*CacheValidatorConfigCompat,
	*DistributedValidatorConfigCompat,
) {

	var authConfig *AuthConfigCompat
	var cacheConfig *CacheValidatorConfigCompat
	var distributedConfig *DistributedValidatorConfigCompat

	// Convert auth configuration
	if unified.Auth != nil {
		authConfig = &AuthConfigCompat{}
		authConfig.FromUnifiedConfig(unified.Auth)
	}

	// Convert cache configuration (partial, as some new fields can't be mapped back)
	if unified.Cache != nil {
		cacheConfig = &CacheValidatorConfigCompat{
			L1Size:             unified.Cache.L1Size,
			L1TTL:              unified.Cache.L1TTL,
			L2TTL:              unified.Cache.L2TTL,
			L2Enabled:          unified.Cache.L2Enabled,
			CompressionEnabled: unified.Cache.CompressionEnabled,
			CompressionLevel:   unified.Cache.CompressionLevel,
			NodeID:             unified.Cache.NodeID,
			MetricsEnabled:     unified.Cache.MetricsEnabled,
		}
	}

	// Convert distributed configuration (partial)
	if unified.Distributed != nil {
		distributedConfig = &DistributedValidatorConfigCompat{
			NodeID:            unified.Distributed.NodeID,
			MaxNodeFailures:   unified.Distributed.MaxNodeFailures,
			ValidationTimeout: unified.Distributed.ConsensusTimeout,
			HeartbeatInterval: unified.Distributed.HeartbeatInterval,
			EnableMetrics:     true, // Assume metrics are enabled
		}
	}

	return authConfig, cacheConfig, distributedConfig
}

// Helper functions for gradual migration

// WrapLegacyAuthConfig wraps a legacy auth config for use with unified system
func WrapLegacyAuthConfig(legacy *AuthConfigCompat) *ValidatorConfig {
	if legacy == nil {
		return DefaultValidatorConfig()
	}

	config := DefaultValidatorConfig()
	config.Auth = legacy.ToUnifiedConfig()

	// Disable other features since we're only using auth
	config.Cache.Enabled = false
	config.Distributed.Enabled = false
	config.Analytics.Enabled = false
	config.Security.Enabled = false

	return config
}

// ExtractLegacyAuthConfig extracts legacy auth config from unified config
func ExtractLegacyAuthConfig(unified *ValidatorConfig) *AuthConfigCompat {
	if unified == nil || unified.Auth == nil {
		return nil
	}

	legacy := &AuthConfigCompat{}
	legacy.FromUnifiedConfig(unified.Auth)
	return legacy
}

// IsLegacyCompatible checks if a unified config can be represented in legacy format
func IsLegacyCompatible(unified *ValidatorConfig) bool {
	// Check if any new features are used that can't be represented in legacy format
	if unified.Security != nil && unified.Security.Enabled {
		return false // Security features are new
	}

	if unified.Analytics != nil && unified.Analytics.TracingEnabled {
		return false // Tracing is new
	}

	if unified.Features != nil && unified.Features.EnableExperimentalValidation {
		return false // Experimental features are new
	}

	return true
}

// Migration helpers for specific scenarios

// MigrationBuilder helps build configurations for migration scenarios
type MigrationBuilder struct {
	unified *ValidatorConfig
}

// NewMigrationBuilder creates a new migration builder
func NewMigrationBuilder() *MigrationBuilder {
	return &MigrationBuilder{
		unified: DefaultValidatorConfig(),
	}
}

// WithLegacyAuth adds legacy auth configuration
func (mb *MigrationBuilder) WithLegacyAuth(legacy *AuthConfigCompat) *MigrationBuilder {
	if legacy != nil {
		mb.unified.Auth = legacy.ToUnifiedConfig()
	}
	return mb
}

// WithLegacyCache adds legacy cache configuration
func (mb *MigrationBuilder) WithLegacyCache(legacy *CacheValidatorConfigCompat) *MigrationBuilder {
	if legacy != nil {
		mb.unified.Cache = legacy.ToUnifiedConfig()
	}
	return mb
}

// WithLegacyDistributed adds legacy distributed configuration
func (mb *MigrationBuilder) WithLegacyDistributed(legacy *DistributedValidatorConfigCompat) *MigrationBuilder {
	if legacy != nil {
		mb.unified.Distributed = legacy.ToUnifiedConfig()
	}
	return mb
}

// Build returns the unified configuration
func (mb *MigrationBuilder) Build() *ValidatorConfig {
	return mb.unified
}

// BuildForEnvironment builds and applies environment-specific overrides
func (mb *MigrationBuilder) BuildForEnvironment(env string) *ValidatorConfig {
	config := mb.unified

	// Apply environment-specific overrides
	switch env {
	case "development":
		if config.Core != nil {
			config.Core.Level = ValidationLevelDevelopment
		}
		if config.Auth != nil {
			config.Auth.RequireAuth = false
		}
	case "production":
		if config.Core != nil {
			config.Core.Level = ValidationLevelProduction
		}
		if config.Auth != nil {
			config.Auth.RequireAuth = true
			config.Auth.RequireSecureTokens = true
		}
	case "testing":
		if config.Core != nil {
			config.Core.Level = ValidationLevelTesting
		}
		if config.Auth != nil {
			config.Auth.Enabled = false
		}
	}

	return config
}
