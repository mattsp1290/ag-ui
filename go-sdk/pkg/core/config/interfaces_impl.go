package config

import (
	"time"

	"github.com/ag-ui/go-sdk/pkg/di"
)

// Implement the DI interfaces to avoid circular imports

// ValidatorConfig implements ValidatorConfigInterface
func (vc *ValidatorConfig) GetCore() di.CoreConfigInterface {
	if vc.Core == nil {
		return nil
	}
	return vc.Core
}

func (vc *ValidatorConfig) GetAuth() di.AuthConfigInterface {
	if vc.Auth == nil {
		return nil
	}
	return vc.Auth
}

func (vc *ValidatorConfig) GetCache() di.CacheConfigInterface {
	if vc.Cache == nil {
		return nil
	}
	return vc.Cache
}

func (vc *ValidatorConfig) GetDistributed() di.DistributedConfigInterface {
	if vc.Distributed == nil {
		return nil
	}
	return vc.Distributed
}

func (vc *ValidatorConfig) GetAnalytics() di.AnalyticsConfigInterface {
	if vc.Analytics == nil {
		return nil
	}
	return vc.Analytics
}

func (vc *ValidatorConfig) GetSecurity() di.SecurityConfigInterface {
	if vc.Security == nil {
		return nil
	}
	return vc.Security
}

// CoreValidationConfig implements CoreConfigInterface
func (cc *CoreValidationConfig) GetLevel() string {
	return string(cc.Level)
}

func (cc *CoreValidationConfig) GetStrict() bool {
	return cc.Strict
}

func (cc *CoreValidationConfig) GetValidationTimeout() time.Duration {
	return cc.ValidationTimeout
}

func (cc *CoreValidationConfig) GetMaxConcurrentValidations() int {
	return cc.MaxConcurrentValidations
}

// AuthValidationConfig implements AuthConfigInterface
func (ac *AuthValidationConfig) IsEnabled() bool {
	return ac.Enabled
}

func (ac *AuthValidationConfig) GetProviderType() string {
	return ac.ProviderType
}

func (ac *AuthValidationConfig) GetProviderConfig() map[string]interface{} {
	return ac.ProviderConfig
}

func (ac *AuthValidationConfig) GetTokenExpiration() time.Duration {
	return ac.TokenExpiration
}

// CacheValidationConfig implements CacheConfigInterface
func (cc *CacheValidationConfig) IsEnabled() bool {
	return cc.Enabled
}

func (cc *CacheValidationConfig) GetL1Size() int {
	return cc.L1Size
}

func (cc *CacheValidationConfig) GetL1TTL() time.Duration {
	return cc.L1TTL
}

func (cc *CacheValidationConfig) IsL2Enabled() bool {
	return cc.L2Enabled
}

func (cc *CacheValidationConfig) GetL2Provider() string {
	return cc.L2Provider
}

func (cc *CacheValidationConfig) GetL2Config() map[string]interface{} {
	return cc.L2Config
}

func (cc *CacheValidationConfig) IsCompressionEnabled() bool {
	return cc.CompressionEnabled
}

func (cc *CacheValidationConfig) GetNodeID() string {
	return cc.NodeID
}

func (cc *CacheValidationConfig) IsClusterMode() bool {
	return cc.ClusterMode
}

// DistributedValidationConfig implements DistributedConfigInterface
func (dc *DistributedValidationConfig) IsEnabled() bool {
	return dc.Enabled
}

func (dc *DistributedValidationConfig) GetNodeID() string {
	return dc.NodeID
}

func (dc *DistributedValidationConfig) GetNodeRole() string {
	return dc.NodeRole
}

func (dc *DistributedValidationConfig) GetConsensusAlgorithm() string {
	return dc.ConsensusAlgorithm
}

func (dc *DistributedValidationConfig) GetConsensusTimeout() time.Duration {
	return dc.ConsensusTimeout
}

func (dc *DistributedValidationConfig) GetMinNodes() int {
	return dc.MinNodes
}

func (dc *DistributedValidationConfig) GetMaxNodes() int {
	return dc.MaxNodes
}

func (dc *DistributedValidationConfig) GetListenAddress() string {
	return dc.ListenAddress
}

// AnalyticsValidationConfig implements AnalyticsConfigInterface
func (ac *AnalyticsValidationConfig) IsEnabled() bool {
	return ac.Enabled
}

func (ac *AnalyticsValidationConfig) IsMetricsEnabled() bool {
	return ac.MetricsEnabled
}

func (ac *AnalyticsValidationConfig) GetMetricsProvider() string {
	return ac.MetricsProvider
}

func (ac *AnalyticsValidationConfig) GetMetricsInterval() time.Duration {
	return ac.MetricsInterval
}

func (ac *AnalyticsValidationConfig) IsTracingEnabled() bool {
	return ac.TracingEnabled
}

func (ac *AnalyticsValidationConfig) GetTracingProvider() string {
	return ac.TracingProvider
}

func (ac *AnalyticsValidationConfig) GetSamplingRate() float64 {
	return ac.SamplingRate
}

func (ac *AnalyticsValidationConfig) IsLoggingEnabled() bool {
	return ac.LoggingEnabled
}

func (ac *AnalyticsValidationConfig) GetLogLevel() string {
	return ac.LogLevel
}

// SecurityValidationConfig implements SecurityConfigInterface
func (sc *SecurityValidationConfig) IsEnabled() bool {
	return sc.Enabled
}

func (sc *SecurityValidationConfig) IsInputSanitizationEnabled() bool {
	return sc.EnableInputSanitization
}

func (sc *SecurityValidationConfig) GetMaxContentLength() int {
	return sc.MaxContentLength
}

func (sc *SecurityValidationConfig) IsRateLimitingEnabled() bool {
	return sc.EnableRateLimiting
}

func (sc *SecurityValidationConfig) GetRateLimit() int {
	return sc.RateLimit
}

func (sc *SecurityValidationConfig) GetRateLimitWindow() time.Duration {
	return sc.RateLimitWindow
}