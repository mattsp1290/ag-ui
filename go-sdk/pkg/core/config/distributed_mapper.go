package config

import (
	"fmt"
	"time"
)

// DistributedConfigMapper maps unified configuration to distributed validator configuration
type DistributedConfigMapper struct {
	provider ValidatorConfigProvider
}

// NewDistributedConfigMapper creates a new distributed configuration mapper
func NewDistributedConfigMapper(provider ValidatorConfigProvider) *DistributedConfigMapper {
	return &DistributedConfigMapper{
		provider: provider,
	}
}

// MapToDistributedConfig maps the unified configuration to distributed validator configuration
func (m *DistributedConfigMapper) MapToDistributedConfig() (*DistributedValidatorConfigAdapter, error) {
	// Get all configuration sections
	distributedConfig, err := m.provider.GetDistributedConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get distributed config: %w", err)
	}

	coreConfig, err := m.provider.GetCoreConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get core config: %w", err)
	}

	_, err = m.provider.GetSecurityConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get security config: %w", err)
	}

	globalSettings, err := m.provider.GetGlobalSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get global settings: %w", err)
	}

	// Create the adapted configuration
	adapter := &DistributedValidatorConfigAdapter{
		NodeID:                        distributedConfig.NodeID,
		MaxNodeFailures:               distributedConfig.MaxNodeFailures,
		ValidationTimeout:             coreConfig.ValidationTimeout,
		HeartbeatInterval:             distributedConfig.HeartbeatInterval,
		EnableMetrics:                 m.provider.IsEnabled("metrics"),
		ConsensusConfig:               m.mapConsensusConfig(distributedConfig),
		StateSync:                     m.mapStateSyncConfig(distributedConfig),
		LoadBalancer:                  m.mapLoadBalancerConfig(distributedConfig),
		PartitionHandler:              m.mapPartitionHandlerConfig(distributedConfig),
		ConsensusCircuitBreakerConfig: m.mapCircuitBreakerConfig("consensus", distributedConfig),
		StateSyncCircuitBreakerConfig: m.mapCircuitBreakerConfig("state-sync", distributedConfig),
		HeartbeatCircuitBreakerConfig: m.mapCircuitBreakerConfig("heartbeat", distributedConfig),
		GoroutineRestartPolicy:        m.mapGoroutineRestartPolicy(globalSettings),
	}

	return adapter, nil
}

// mapConsensusConfig maps consensus configuration
func (m *DistributedConfigMapper) mapConsensusConfig(config *DistributedValidationConfig) *ConsensusConfigAdapter {
	return &ConsensusConfigAdapter{
		Algorithm:        config.ConsensusAlgorithm,
		MinNodes:         config.MinNodes,
		MaxNodes:         config.MaxNodes,
		Timeout:          config.ConsensusTimeout,
		RequireUnanimous: config.RequireUnanimous,
		EnableTLS:        config.EnableTLS,
		TLSCertFile:      config.TLSCertFile,
		TLSKeyFile:       config.TLSKeyFile,
		TLSCAFile:        config.TLSCAFile,
		EnableMutualTLS:  config.EnableMutualTLS,
	}
}

// mapStateSyncConfig maps state synchronization configuration
func (m *DistributedConfigMapper) mapStateSyncConfig(config *DistributedValidationConfig) *StateSyncConfigAdapter {
	return &StateSyncConfigAdapter{
		Enabled:      config.StateSyncEnabled,
		Interval:     config.StateSyncInterval,
		Protocol:     config.StateSyncProtocol,
		Timeout:      config.FailureDetectTimeout,
		MaxRetries:   3,               // Default value
		RetryBackoff: 1 * time.Second, // Default value
		BatchSize:    100,             // Default value
		EnableTLS:    config.EnableTLS,
		TLSCertFile:  config.TLSCertFile,
		TLSKeyFile:   config.TLSKeyFile,
		TLSCAFile:    config.TLSCAFile,
	}
}

// mapLoadBalancerConfig maps load balancer configuration
func (m *DistributedConfigMapper) mapLoadBalancerConfig(config *DistributedValidationConfig) *LoadBalancerConfigAdapter {
	return &LoadBalancerConfigAdapter{
		Strategy:             config.LoadBalanceStrategy,
		HealthCheckEnabled:   true,             // Default value
		HealthCheckInterval:  30 * time.Second, // Default value
		HealthCheckTimeout:   5 * time.Second,  // Default value
		MaxRetries:           3,                // Default value
		RetryBackoff:         1 * time.Second,  // Default value
		LoadThreshold:        config.LoadThreshold,
		EnableStickySessions: false,            // Default value
		SessionTimeout:       30 * time.Minute, // Default value
	}
}

// mapPartitionHandlerConfig maps partition handler configuration
func (m *DistributedConfigMapper) mapPartitionHandlerConfig(config *DistributedValidationConfig) *PartitionHandlerConfigAdapter {
	return &PartitionHandlerConfigAdapter{
		PartitionTolerance:         config.PartitionTolerance,
		AllowLocalValidation:       config.AllowLocalValidation,
		PartitionTimeout:           config.PartitionTimeout,
		RecoveryEnabled:            config.RecoveryEnabled,
		RecoveryInterval:           30 * time.Second, // Default value
		MaxPartitionTime:           5 * time.Minute,  // Default value
		QuorumSize:                 config.MinNodes,
		EnableSplitBrainProtection: true,            // Default value
		SplitBrainTimeout:          1 * time.Minute, // Default value
	}
}

// mapCircuitBreakerConfig maps circuit breaker configuration
func (m *DistributedConfigMapper) mapCircuitBreakerConfig(name string, config *DistributedValidationConfig) *CircuitBreakerConfigAdapter {
	return &CircuitBreakerConfigAdapter{
		Name:             name,
		MaxRequests:      100,
		Interval:         60 * time.Second,
		Timeout:          30 * time.Second,
		MaxRetries:       3,
		RetryBackoff:     1 * time.Second,
		FailureThreshold: 5,
		SuccessThreshold: 3,
		HalfOpenRequests: 10,
		EnableMetrics:    true,
		MetricsInterval:  30 * time.Second,
	}
}

// mapGoroutineRestartPolicy maps goroutine restart policy
func (m *DistributedConfigMapper) mapGoroutineRestartPolicy(config *GlobalSettings) *GoroutineRestartPolicyAdapter {
	return &GoroutineRestartPolicyAdapter{
		MaxRestarts:       10,
		RestartWindow:     5 * time.Minute,
		BaseBackoff:       100 * time.Millisecond,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		EnableJitter:      true,
		JitterFactor:      0.1,
		EnableMetrics:     true,
		MetricsInterval:   30 * time.Second,
	}
}

// Configuration adapters that decouple the distributed validator from internal config structures

// DistributedValidatorConfigAdapter adapts unified configuration for distributed validator
type DistributedValidatorConfigAdapter struct {
	NodeID                        string
	MaxNodeFailures               int
	ValidationTimeout             time.Duration
	HeartbeatInterval             time.Duration
	EnableMetrics                 bool
	ConsensusConfig               *ConsensusConfigAdapter
	StateSync                     *StateSyncConfigAdapter
	LoadBalancer                  *LoadBalancerConfigAdapter
	PartitionHandler              *PartitionHandlerConfigAdapter
	ConsensusCircuitBreakerConfig *CircuitBreakerConfigAdapter
	StateSyncCircuitBreakerConfig *CircuitBreakerConfigAdapter
	HeartbeatCircuitBreakerConfig *CircuitBreakerConfigAdapter
	GoroutineRestartPolicy        *GoroutineRestartPolicyAdapter
}

// ConsensusConfigAdapter adapts consensus configuration
type ConsensusConfigAdapter struct {
	Algorithm        string
	MinNodes         int
	MaxNodes         int
	Timeout          time.Duration
	RequireUnanimous bool
	EnableTLS        bool
	TLSCertFile      string
	TLSKeyFile       string
	TLSCAFile        string
	EnableMutualTLS  bool
}

// StateSyncConfigAdapter adapts state synchronization configuration
type StateSyncConfigAdapter struct {
	Enabled      bool
	Interval     time.Duration
	Protocol     string
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
	BatchSize    int
	EnableTLS    bool
	TLSCertFile  string
	TLSKeyFile   string
	TLSCAFile    string
}

// LoadBalancerConfigAdapter adapts load balancer configuration
type LoadBalancerConfigAdapter struct {
	Strategy             string
	HealthCheckEnabled   bool
	HealthCheckInterval  time.Duration
	HealthCheckTimeout   time.Duration
	MaxRetries           int
	RetryBackoff         time.Duration
	LoadThreshold        float64
	EnableStickySessions bool
	SessionTimeout       time.Duration
}

// PartitionHandlerConfigAdapter adapts partition handler configuration
type PartitionHandlerConfigAdapter struct {
	PartitionTolerance         bool
	AllowLocalValidation       bool
	PartitionTimeout           time.Duration
	RecoveryEnabled            bool
	RecoveryInterval           time.Duration
	MaxPartitionTime           time.Duration
	QuorumSize                 int
	EnableSplitBrainProtection bool
	SplitBrainTimeout          time.Duration
}

// CircuitBreakerConfigAdapter adapts circuit breaker configuration
type CircuitBreakerConfigAdapter struct {
	Name             string
	MaxRequests      int
	Interval         time.Duration
	Timeout          time.Duration
	MaxRetries       int
	RetryBackoff     time.Duration
	FailureThreshold int
	SuccessThreshold int
	HalfOpenRequests int
	EnableMetrics    bool
	MetricsInterval  time.Duration
}

// GoroutineRestartPolicyAdapter adapts goroutine restart policy
type GoroutineRestartPolicyAdapter struct {
	MaxRestarts       int
	RestartWindow     time.Duration
	BaseBackoff       time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
	EnableJitter      bool
	JitterFactor      float64
	EnableMetrics     bool
	MetricsInterval   time.Duration
}

// Conversion methods to convert adapters to original types (if needed)

// ToDistributedValidatorConfig converts adapter to original distributed validator config
func (a *DistributedValidatorConfigAdapter) ToDistributedValidatorConfig() interface{} {
	// This would convert to the original distributed validator config type
	// For now, we return the adapter itself as it provides the necessary interface
	return a
}

// GetNodeID returns the node ID
func (a *DistributedValidatorConfigAdapter) GetNodeID() string {
	return a.NodeID
}

// GetMaxNodeFailures returns the maximum node failures
func (a *DistributedValidatorConfigAdapter) GetMaxNodeFailures() int {
	return a.MaxNodeFailures
}

// GetValidationTimeout returns the validation timeout
func (a *DistributedValidatorConfigAdapter) GetValidationTimeout() time.Duration {
	return a.ValidationTimeout
}

// GetHeartbeatInterval returns the heartbeat interval
func (a *DistributedValidatorConfigAdapter) GetHeartbeatInterval() time.Duration {
	return a.HeartbeatInterval
}

// IsMetricsEnabled returns whether metrics are enabled
func (a *DistributedValidatorConfigAdapter) IsMetricsEnabled() bool {
	return a.EnableMetrics
}

// GetConsensusConfig returns the consensus configuration
func (a *DistributedValidatorConfigAdapter) GetConsensusConfig() *ConsensusConfigAdapter {
	return a.ConsensusConfig
}

// GetStateSyncConfig returns the state sync configuration
func (a *DistributedValidatorConfigAdapter) GetStateSyncConfig() *StateSyncConfigAdapter {
	return a.StateSync
}

// GetLoadBalancerConfig returns the load balancer configuration
func (a *DistributedValidatorConfigAdapter) GetLoadBalancerConfig() *LoadBalancerConfigAdapter {
	return a.LoadBalancer
}

// GetPartitionHandlerConfig returns the partition handler configuration
func (a *DistributedValidatorConfigAdapter) GetPartitionHandlerConfig() *PartitionHandlerConfigAdapter {
	return a.PartitionHandler
}

// GetGoroutineRestartPolicy returns the goroutine restart policy
func (a *DistributedValidatorConfigAdapter) GetGoroutineRestartPolicy() *GoroutineRestartPolicyAdapter {
	return a.GoroutineRestartPolicy
}

// Validation methods

// Validate validates the distributed validator configuration
func (a *DistributedValidatorConfigAdapter) Validate() error {
	if a.NodeID == "" {
		return fmt.Errorf("node ID cannot be empty")
	}

	if a.MaxNodeFailures < 0 {
		return fmt.Errorf("max node failures cannot be negative")
	}

	if a.ValidationTimeout <= 0 {
		return fmt.Errorf("validation timeout must be positive")
	}

	if a.HeartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat interval must be positive")
	}

	// Validate nested configs
	if a.ConsensusConfig != nil {
		if err := a.ConsensusConfig.Validate(); err != nil {
			return fmt.Errorf("consensus config validation failed: %w", err)
		}
	}

	if a.StateSync != nil {
		if err := a.StateSync.Validate(); err != nil {
			return fmt.Errorf("state sync config validation failed: %w", err)
		}
	}

	if a.LoadBalancer != nil {
		if err := a.LoadBalancer.Validate(); err != nil {
			return fmt.Errorf("load balancer config validation failed: %w", err)
		}
	}

	if a.PartitionHandler != nil {
		if err := a.PartitionHandler.Validate(); err != nil {
			return fmt.Errorf("partition handler config validation failed: %w", err)
		}
	}

	return nil
}

// Validate validates the consensus configuration
func (c *ConsensusConfigAdapter) Validate() error {
	if c.Algorithm == "" {
		return fmt.Errorf("consensus algorithm cannot be empty")
	}

	if c.MinNodes <= 0 {
		return fmt.Errorf("min nodes must be positive")
	}

	if c.MaxNodes < c.MinNodes {
		return fmt.Errorf("max nodes must be >= min nodes")
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	return nil
}

// Validate validates the state sync configuration
func (s *StateSyncConfigAdapter) Validate() error {
	if s.Enabled {
		if s.Interval <= 0 {
			return fmt.Errorf("sync interval must be positive when enabled")
		}

		if s.Protocol == "" {
			return fmt.Errorf("sync protocol cannot be empty when enabled")
		}

		if s.Timeout <= 0 {
			return fmt.Errorf("sync timeout must be positive")
		}

		if s.MaxRetries < 0 {
			return fmt.Errorf("max retries cannot be negative")
		}

		if s.BatchSize <= 0 {
			return fmt.Errorf("batch size must be positive")
		}
	}

	return nil
}

// Validate validates the load balancer configuration
func (l *LoadBalancerConfigAdapter) Validate() error {
	if l.Strategy == "" {
		return fmt.Errorf("load balance strategy cannot be empty")
	}

	if l.LoadThreshold < 0 || l.LoadThreshold > 1 {
		return fmt.Errorf("load threshold must be between 0 and 1")
	}

	if l.HealthCheckEnabled {
		if l.HealthCheckInterval <= 0 {
			return fmt.Errorf("health check interval must be positive when enabled")
		}

		if l.HealthCheckTimeout <= 0 {
			return fmt.Errorf("health check timeout must be positive when enabled")
		}
	}

	return nil
}

// Validate validates the partition handler configuration
func (p *PartitionHandlerConfigAdapter) Validate() error {
	if p.PartitionTimeout <= 0 {
		return fmt.Errorf("partition timeout must be positive")
	}

	if p.RecoveryEnabled && p.RecoveryInterval <= 0 {
		return fmt.Errorf("recovery interval must be positive when recovery is enabled")
	}

	if p.MaxPartitionTime <= 0 {
		return fmt.Errorf("max partition time must be positive")
	}

	if p.QuorumSize <= 0 {
		return fmt.Errorf("quorum size must be positive")
	}

	return nil
}
