// Package client provides configuration profiles and validation helpers to reduce
// configuration complexity and provide a more user-friendly configuration experience.
package client

import (
	"crypto/tls"
	"fmt"
	"time"
)

// ConfigProfile represents different pre-configured setups for common use cases
type ConfigProfile string

const (
	// ProfileDevelopment provides settings optimized for development and testing
	ProfileDevelopment ConfigProfile = "development"
	
	// ProfileProduction provides settings optimized for production deployment
	ProfileProduction ConfigProfile = "production"
	
	// ProfileMinimal provides basic functionality with minimal configuration
	ProfileMinimal ConfigProfile = "minimal"
)

// ConfigBuilder provides a fluent interface for building complex configurations
type ConfigBuilder struct {
	agentConfig    *AgentConfig
	httpConfig     *HttpConfig
	securityConfig *SecurityConfig
	sseConfig      *SSEClientConfig
	resilienceConfig *ResilienceConfig
	errors         []error
}

// NewConfigBuilder creates a new configuration builder
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		agentConfig:    &AgentConfig{},
		httpConfig:     &HttpConfig{},
		securityConfig: &SecurityConfig{},
		sseConfig:      &SSEClientConfig{},
		resilienceConfig: &ResilienceConfig{},
		errors:         make([]error, 0),
	}
}

// NewDevelopmentConfig creates a configuration optimized for development and testing
func NewDevelopmentConfig() *ConfigBuilder {
	builder := NewConfigBuilder()
	
	// Agent configuration
	builder.agentConfig = &AgentConfig{
		Name:        "dev-agent",
		Description: "Development agent with debug settings",
		Capabilities: AgentCapabilities{
			Tools:              []string{"basic", "debug"},
			Streaming:          true,
			StateSync:          true,
			MessageHistory:     true,
			CustomCapabilities: make(map[string]interface{}),
		},
		EventProcessing: EventProcessingConfig{
			BufferSize:       100,
			BatchSize:        10,
			Timeout:          5 * time.Second,
			EnableValidation: true,
			EnableMetrics:    true,
		},
		State: StateConfig{
			SyncInterval:       1 * time.Second,
			CacheSize:          "100MB",
			EnablePersistence:  false,
			ConflictResolution: "last_write_wins", // Using string for compatibility
		},
		Tools: ToolsConfig{
			Timeout:          30 * time.Second,
			MaxConcurrent:    5,
			EnableSandboxing: false,
			EnableCaching:    true,
		},
		History: HistoryConfig{
			MaxMessages: 1000,
			Retention:   24 * time.Hour,
		},
		Custom: make(map[string]interface{}),
	}
	
	// HTTP configuration - relaxed settings for development
	builder.httpConfig = &HttpConfig{
		ProtocolVersion:    "HTTP/1.1",
		EnableHTTP2:        false,
		ForceHTTP2:         false,
		MaxIdleConns:       10,
		MaxIdleConnsPerHost: 2,
		MaxConnsPerHost:     5,
		IdleConnTimeout:     30 * time.Second,
		KeepAlive:           30 * time.Second,
		DialTimeout:         10 * time.Second,
		RequestTimeout:      30 * time.Second,
		ResponseTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		InsecureSkipVerify:  true, // Relaxed for development
		DisableCompression:  false,
		DisableKeepAlives:   false,
		UserAgent:           "ag-ui-sdk-dev/1.0",
		MaxResponseBodySize: 50 * 1024 * 1024, // 50MB
		EnableCircuitBreaker: false, // Disabled for development
	}
	
	// Security configuration - minimal security for development
	builder.securityConfig = &SecurityConfig{
		AuthMethod:       AuthMethodAPIKey,
		EnableMultiAuth:  false,
		SupportedMethods: []AuthMethod{AuthMethodAPIKey},
		APIKey: APIKeyConfig{
			HeaderName:              "X-API-Key",
			QueryParam:              "api_key",
			Prefix:                  "",
			HashingAlgorithm:        "sha256",
			EnableKeyRotation:       false,
			KeyRotationInterval:     0,
		},
		TLS: TLSConfig{
			Enabled:               false,
			InsecureSkipVerify:    true,
			EnableSNI:             false,
		},
		SecurityHeaders: SecurityHeadersConfig{
			EnableCSP:           false,
			EnableHSTS:          false,
			EnableXFrameOptions: false,
			EnableXContentType:  false,
			CORSConfig: CORSConfig{
				Enabled:         true,
				AllowedOrigins:  []string{"*"},
				AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders:  []string{"*"},
				AllowCredentials: false,
				MaxAge:          3600,
			},
		},
		RateLimit: RateLimitConfig{
			Enabled:           false,
			RequestsPerSecond: 1000,
			BurstSize:         100,
		},
	}
	
	// SSE configuration - basic settings
	builder.sseConfig = &SSEClientConfig{
		InitialBackoff:       1 * time.Second,
		MaxBackoff:           10 * time.Second,
		BackoffMultiplier:    2.0,
		MaxReconnectAttempts: 5,
		EventBufferSize:      100,
		ReadTimeout:          0, // No timeout for development
		WriteTimeout:         10 * time.Second,
		HealthCheckInterval:  30 * time.Second,
		Headers:              make(map[string]string),
	}
	
	// Resilience configuration - minimal resilience for development
	builder.resilienceConfig = &ResilienceConfig{
		Retry: RetryConfig{
			MaxAttempts:       3,
			BaseDelay:         1 * time.Second,
			MaxDelay:          10 * time.Second,
			BackoffMultiplier: 2.0,
			JitterEnabled:     false,
			JitterMaxFactor:   0.1,
			RetryableErrors:   []string{"timeout", "connection_error"},
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:                 false,
			FailureThreshold:        5,
			SuccessThreshold:        3,
			Timeout:                 60 * time.Second,
			HalfOpenMaxCalls:        3,
			FailureRateThreshold:    0.5,
			MinimumRequestThreshold: 10,
		},
		RateLimit: RateLimitConfig{
			Enabled:           false,
			RequestsPerSecond: 100,
			BurstSize:         20,
		},
	}
	
	return builder
}

// NewProductionConfig creates a configuration optimized for production deployment
func NewProductionConfig() *ConfigBuilder {
	builder := NewConfigBuilder()
	
	// Agent configuration
	builder.agentConfig = &AgentConfig{
		Name:        "prod-agent",
		Description: "Production agent with optimized settings",
		Capabilities: AgentCapabilities{
			Tools:              []string{"production", "monitoring"},
			Streaming:          true,
			StateSync:          true,
			MessageHistory:     true,
			CustomCapabilities: make(map[string]interface{}),
		},
		EventProcessing: EventProcessingConfig{
			BufferSize:       1000,
			BatchSize:        50,
			Timeout:          30 * time.Second,
			EnableValidation: true,
			EnableMetrics:    true,
		},
		State: StateConfig{
			SyncInterval:       5 * time.Second,
			CacheSize:          "1GB",
			EnablePersistence:  true,
			ConflictResolution: "merge_with_priority",
		},
		Tools: ToolsConfig{
			Timeout:          120 * time.Second,
			MaxConcurrent:    20,
			EnableSandboxing: true,
			EnableCaching:    true,
		},
		History: HistoryConfig{
			MaxMessages: 10000,
			Retention:   7 * 24 * time.Hour, // 7 days
		},
		Custom: make(map[string]interface{}),
	}
	
	// HTTP configuration - optimized for production
	builder.httpConfig = &HttpConfig{
		ProtocolVersion:    "HTTP/2",
		EnableHTTP2:        true,
		ForceHTTP2:         false,
		MaxIdleConns:       100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     90 * time.Second,
		KeepAlive:           30 * time.Second,
		DialTimeout:         30 * time.Second,
		RequestTimeout:      120 * time.Second,
		ResponseTimeout:     120 * time.Second,
		TLSHandshakeTimeout: 30 * time.Second,
		InsecureSkipVerify:  false,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		UserAgent:           "ag-ui-sdk-prod/1.0",
		MaxResponseBodySize: 100 * 1024 * 1024, // 100MB
		EnableCircuitBreaker: true,
		CircuitBreakerConfig: &CircuitBreakerConfig{
			Enabled:                 true,
			FailureThreshold:        10,
			SuccessThreshold:        5,
			Timeout:                 60 * time.Second,
			HalfOpenMaxCalls:        5,
			FailureRateThreshold:    0.6,
			MinimumRequestThreshold: 20,
		},
	}
	
	// Security configuration - comprehensive security for production
	builder.securityConfig = &SecurityConfig{
		AuthMethod:       AuthMethodJWT,
		EnableMultiAuth:  true,
		SupportedMethods: []AuthMethod{AuthMethodJWT, AuthMethodAPIKey, AuthMethodMTLS},
		JWT: JWTConfig{
			SigningMethod:        "RS256",
			AccessTokenTTL:       15 * time.Minute,
			RefreshTokenTTL:      24 * time.Hour,
			AutoRefresh:          true,
			RefreshThreshold:     2 * time.Minute,
			LeewayTime:           30 * time.Second,
			CustomClaims:         make(map[string]interface{}),
		},
		APIKey: APIKeyConfig{
			HeaderName:              "X-API-Key",
			Prefix:                  "Bearer ",
			HashingAlgorithm:        "sha256",
			EnableKeyRotation:       true,
			KeyRotationInterval:     24 * time.Hour,
		},
		TLS: TLSConfig{
			Enabled:               true,
			ClientAuth:            tls.RequireAndVerifyClientCert,
			MinVersion:            tls.VersionTLS12,
			MaxVersion:            tls.VersionTLS13,
			InsecureSkipVerify:    false,
			EnableSNI:             true,
		},
		SecurityHeaders: SecurityHeadersConfig{
			EnableCSP:           true,
			CSPPolicy:           "default-src 'self'; script-src 'self' 'unsafe-inline'",
			EnableHSTS:          true,
			HSTSMaxAge:          31536000, // 1 year
			EnableXFrameOptions: true,
			XFrameOptions:       "DENY",
			EnableXContentType:  true,
			EnableReferrerPolicy: true,
			ReferrerPolicy:      "strict-origin-when-cross-origin",
			CustomHeaders:       map[string]string{
				"X-Content-Type-Options": "nosniff",
				"X-XSS-Protection":       "1; mode=block",
			},
			CORSConfig: CORSConfig{
				Enabled:         true,
				AllowedOrigins:  []string{}, // Must be explicitly configured
				AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE"},
				AllowedHeaders:  []string{"Authorization", "Content-Type", "X-API-Key"},
				AllowCredentials: true,
				MaxAge:          86400, // 24 hours
			},
		},
		TokenStorage: TokenStorageConfig{
			StorageType: "encrypted_memory",
			Encryption: EncryptionConfig{
				Enabled:   true,
				Algorithm: "AES-256-GCM",
			},
		},
		AuditLogging: AuditLoggingConfig{
			Enabled:    true,
			LogLevel:   "INFO",
			LogFormat:  "json",
			IncludeRequestBody:  false,
			IncludeResponseBody: false,
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 100,
			BurstSize:         20,
		},
	}
	
	// SSE configuration - robust settings for production
	builder.sseConfig = &SSEClientConfig{
		InitialBackoff:       2 * time.Second,
		MaxBackoff:           60 * time.Second,
		BackoffMultiplier:    2.0,
		MaxReconnectAttempts: 10,
		EventBufferSize:      1000,
		ReadTimeout:          5 * time.Minute,
		WriteTimeout:         30 * time.Second,
		HealthCheckInterval:  30 * time.Second,
		Headers:              make(map[string]string),
	}
	
	// Resilience configuration - comprehensive resilience for production
	builder.resilienceConfig = &ResilienceConfig{
		Retry: RetryConfig{
			MaxAttempts:       5,
			BaseDelay:         2 * time.Second,
			MaxDelay:          60 * time.Second,
			BackoffMultiplier: 2.0,
			JitterEnabled:     true,
			JitterMaxFactor:   0.2,
			RetryableErrors:   []string{"timeout", "connection_error", "service_unavailable", "rate_limit"},
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:                 true,
			FailureThreshold:        20,
			SuccessThreshold:        5,
			Timeout:                 60 * time.Second,
			HalfOpenMaxCalls:        10,
			FailureRateThreshold:    0.5,
			MinimumRequestThreshold: 50,
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 100,
			BurstSize:         20,
		},
	}
	
	return builder
}

// NewMinimalConfig creates a configuration with basic functionality only
func NewMinimalConfig() *ConfigBuilder {
	builder := NewConfigBuilder()
	
	// Agent configuration - minimal setup
	builder.agentConfig = &AgentConfig{
		Name:        "minimal-agent",
		Description: "Minimal agent configuration",
		Capabilities: AgentCapabilities{
			Tools:              []string{"basic"},
			Streaming:          false,
			StateSync:          false,
			MessageHistory:     false,
			CustomCapabilities: make(map[string]interface{}),
		},
		EventProcessing: EventProcessingConfig{
			BufferSize:       10,
			BatchSize:        1,
			Timeout:          10 * time.Second,
			EnableValidation: false,
			EnableMetrics:    false,
		},
		State: StateConfig{
			SyncInterval:       0, // Disabled
			CacheSize:          "10MB",
			EnablePersistence:  false,
			ConflictResolution: "last_write_wins",
		},
		Tools: ToolsConfig{
			Timeout:          30 * time.Second,
			MaxConcurrent:    1,
			EnableSandboxing: false,
			EnableCaching:    false,
		},
		History: HistoryConfig{
			MaxMessages: 0, // Disabled
			Retention:   0, // Disabled
		},
		Custom: make(map[string]interface{}),
	}
	
	// HTTP configuration - basic settings
	builder.httpConfig = &HttpConfig{
		ProtocolVersion:    "HTTP/1.1",
		EnableHTTP2:        false,
		ForceHTTP2:         false,
		MaxIdleConns:       2,
		MaxIdleConnsPerHost: 1,
		MaxConnsPerHost:     2,
		IdleConnTimeout:     30 * time.Second,
		KeepAlive:           0, // Disabled
		DialTimeout:         10 * time.Second,
		RequestTimeout:      30 * time.Second,
		ResponseTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		InsecureSkipVerify:  false,
		DisableCompression:  true,
		DisableKeepAlives:   true,
		UserAgent:           "ag-ui-sdk-minimal/1.0",
		MaxResponseBodySize: 10 * 1024 * 1024, // 10MB
		EnableCircuitBreaker: false,
	}
	
	// Security configuration - basic security
	builder.securityConfig = &SecurityConfig{
		AuthMethod:       AuthMethodBasic,
		EnableMultiAuth:  false,
		SupportedMethods: []AuthMethod{AuthMethodBasic},
		BasicAuth: BasicAuthConfig{
			Realm:                   "Minimal Auth",
			HashingAlgorithm:        "bcrypt",
			EnablePasswordPolicy:    false,
		},
		TLS: TLSConfig{
			Enabled:               false,
			InsecureSkipVerify:    false,
			EnableSNI:             false,
		},
		SecurityHeaders: SecurityHeadersConfig{
			EnableCSP:           false,
			EnableHSTS:          false,
			EnableXFrameOptions: false,
			EnableXContentType:  false,
			CORSConfig: CORSConfig{
				Enabled:         false,
			},
		},
		RateLimit: RateLimitConfig{
			Enabled:           false,
		},
	}
	
	// SSE configuration - disabled
	builder.sseConfig = &SSEClientConfig{
		InitialBackoff:       1 * time.Second,
		MaxBackoff:           5 * time.Second,
		BackoffMultiplier:    1.5,
		MaxReconnectAttempts: 1,
		EventBufferSize:      10,
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         10 * time.Second,
		HealthCheckInterval:  0, // Disabled
		Headers:              make(map[string]string),
	}
	
	// Resilience configuration - minimal resilience
	builder.resilienceConfig = &ResilienceConfig{
		Retry: RetryConfig{
			MaxAttempts:       1, // No retries
			BaseDelay:         0,
			MaxDelay:          0,
			BackoffMultiplier: 1.0,
			JitterEnabled:     false,
			JitterMaxFactor:   0,
			RetryableErrors:   []string{},
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		RateLimit: RateLimitConfig{
			Enabled: false,
		},
	}
	
	return builder
}

// =============================================================================
// Configuration Builder Methods
// =============================================================================

// WithAgent sets the agent configuration
func (cb *ConfigBuilder) WithAgent(config *AgentConfig) *ConfigBuilder {
	if config != nil {
		cb.agentConfig = config
	}
	return cb
}

// WithAgentName sets the agent name
func (cb *ConfigBuilder) WithAgentName(name string) *ConfigBuilder {
	if cb.agentConfig == nil {
		cb.agentConfig = &AgentConfig{}
	}
	cb.agentConfig.Name = name
	return cb
}

// WithAgentDescription sets the agent description
func (cb *ConfigBuilder) WithAgentDescription(description string) *ConfigBuilder {
	if cb.agentConfig == nil {
		cb.agentConfig = &AgentConfig{}
	}
	cb.agentConfig.Description = description
	return cb
}

// WithHTTP sets the HTTP configuration
func (cb *ConfigBuilder) WithHTTP(config *HttpConfig) *ConfigBuilder {
	if config != nil {
		cb.httpConfig = config
	}
	return cb
}

// WithHTTPTimeouts sets common HTTP timeout values
func (cb *ConfigBuilder) WithHTTPTimeouts(dial, request, response time.Duration) *ConfigBuilder {
	if cb.httpConfig == nil {
		cb.httpConfig = &HttpConfig{}
	}
	cb.httpConfig.DialTimeout = dial
	cb.httpConfig.RequestTimeout = request
	cb.httpConfig.ResponseTimeout = response
	return cb
}

// WithHTTPConnectionLimits sets HTTP connection limits
func (cb *ConfigBuilder) WithHTTPConnectionLimits(maxIdle, maxIdlePerHost, maxPerHost int) *ConfigBuilder {
	if cb.httpConfig == nil {
		cb.httpConfig = &HttpConfig{}
	}
	cb.httpConfig.MaxIdleConns = maxIdle
	cb.httpConfig.MaxIdleConnsPerHost = maxIdlePerHost
	cb.httpConfig.MaxConnsPerHost = maxPerHost
	return cb
}

// WithHTTP2 enables or disables HTTP/2
func (cb *ConfigBuilder) WithHTTP2(enabled, force bool) *ConfigBuilder {
	if cb.httpConfig == nil {
		cb.httpConfig = &HttpConfig{}
	}
	cb.httpConfig.EnableHTTP2 = enabled
	cb.httpConfig.ForceHTTP2 = force
	if enabled {
		cb.httpConfig.ProtocolVersion = "HTTP/2"
	} else {
		cb.httpConfig.ProtocolVersion = "HTTP/1.1"
	}
	return cb
}

// WithSecurity sets the security configuration
func (cb *ConfigBuilder) WithSecurity(config *SecurityConfig) *ConfigBuilder {
	if config != nil {
		cb.securityConfig = config
	}
	return cb
}

// WithAuthMethod sets the primary authentication method
func (cb *ConfigBuilder) WithAuthMethod(method AuthMethod) *ConfigBuilder {
	if cb.securityConfig == nil {
		cb.securityConfig = &SecurityConfig{}
	}
	cb.securityConfig.AuthMethod = method
	return cb
}

// WithJWTAuth configures JWT authentication
func (cb *ConfigBuilder) WithJWTAuth(signingMethod, secretKey string, accessTTL, refreshTTL time.Duration) *ConfigBuilder {
	if cb.securityConfig == nil {
		cb.securityConfig = &SecurityConfig{}
	}
	cb.securityConfig.AuthMethod = AuthMethodJWT
	cb.securityConfig.JWT = JWTConfig{
		SigningMethod:   signingMethod,
		SecretKey:       secretKey,
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		AutoRefresh:     true,
		RefreshThreshold: accessTTL / 4, // Refresh when 25% of TTL remains
		LeewayTime:      30 * time.Second,
		CustomClaims:    make(map[string]interface{}),
	}
	return cb
}

// WithAPIKeyAuth configures API key authentication
func (cb *ConfigBuilder) WithAPIKeyAuth(headerName, prefix string) *ConfigBuilder {
	if cb.securityConfig == nil {
		cb.securityConfig = &SecurityConfig{}
	}
	cb.securityConfig.AuthMethod = AuthMethodAPIKey
	cb.securityConfig.APIKey = APIKeyConfig{
		HeaderName:              headerName,
		Prefix:                  prefix,
		HashingAlgorithm:        "sha256",
		EnableKeyRotation:       false,
		KeyRotationInterval:     0,
	}
	return cb
}

// WithTLS configures TLS settings
func (cb *ConfigBuilder) WithTLS(enabled bool, certFile, keyFile, caFile string) *ConfigBuilder {
	if cb.securityConfig == nil {
		cb.securityConfig = &SecurityConfig{}
	}
	cb.securityConfig.TLS = TLSConfig{
		Enabled:               enabled,
		CertFile:              certFile,
		KeyFile:               keyFile,
		CAFile:                caFile,
		MinVersion:            tls.VersionTLS12,
		MaxVersion:            tls.VersionTLS13,
		InsecureSkipVerify:    false,
		EnableSNI:             true,
	}
	return cb
}

// WithInsecureTLS enables insecure TLS (skip verification)
func (cb *ConfigBuilder) WithInsecureTLS() *ConfigBuilder {
	if cb.securityConfig == nil {
		cb.securityConfig = &SecurityConfig{}
	}
	if cb.httpConfig == nil {
		cb.httpConfig = &HttpConfig{}
	}
	cb.securityConfig.TLS.InsecureSkipVerify = true
	cb.httpConfig.InsecureSkipVerify = true
	return cb
}

// WithCORS configures CORS settings
func (cb *ConfigBuilder) WithCORS(allowedOrigins, allowedMethods, allowedHeaders []string) *ConfigBuilder {
	if cb.securityConfig == nil {
		cb.securityConfig = &SecurityConfig{}
	}
	cb.securityConfig.SecurityHeaders.CORSConfig = CORSConfig{
		Enabled:         true,
		AllowedOrigins:  allowedOrigins,
		AllowedMethods:  allowedMethods,
		AllowedHeaders:  allowedHeaders,
		AllowCredentials: len(allowedOrigins) > 0 && allowedOrigins[0] != "*",
		MaxAge:          86400, // 24 hours
	}
	return cb
}

// WithRateLimit configures rate limiting
func (cb *ConfigBuilder) WithRateLimit(requestsPerSecond float64, burstSize int) *ConfigBuilder {
	if cb.securityConfig == nil {
		cb.securityConfig = &SecurityConfig{}
	}
	cb.securityConfig.RateLimit = RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: requestsPerSecond,
		BurstSize:         burstSize,
	}
	return cb
}

// WithSSE sets the SSE configuration
func (cb *ConfigBuilder) WithSSE(config *SSEClientConfig) *ConfigBuilder {
	if config != nil {
		cb.sseConfig = config
	}
	return cb
}

// WithSSEBackoff configures SSE reconnection backoff
func (cb *ConfigBuilder) WithSSEBackoff(initial, max time.Duration, multiplier float64) *ConfigBuilder {
	if cb.sseConfig == nil {
		cb.sseConfig = &SSEClientConfig{}
	}
	cb.sseConfig.InitialBackoff = initial
	cb.sseConfig.MaxBackoff = max
	cb.sseConfig.BackoffMultiplier = multiplier
	return cb
}

// WithResilience sets the resilience configuration
func (cb *ConfigBuilder) WithResilience(config *ResilienceConfig) *ConfigBuilder {
	if config != nil {
		cb.resilienceConfig = config
	}
	return cb
}

// WithRetry configures retry behavior
func (cb *ConfigBuilder) WithRetry(maxAttempts int, baseDelay, maxDelay time.Duration) *ConfigBuilder {
	if cb.resilienceConfig == nil {
		cb.resilienceConfig = &ResilienceConfig{}
	}
	cb.resilienceConfig.Retry = RetryConfig{
		MaxAttempts:       maxAttempts,
		BaseDelay:         baseDelay,
		MaxDelay:          maxDelay,
		BackoffMultiplier: 2.0,
		JitterEnabled:     true,
		JitterMaxFactor:   0.1,
		RetryableErrors:   []string{"timeout", "connection_error", "service_unavailable"},
	}
	return cb
}

// WithCircuitBreaker configures circuit breaker behavior
func (cb *ConfigBuilder) WithCircuitBreaker(enabled bool, failureThreshold, successThreshold int, timeout time.Duration) *ConfigBuilder {
	if cb.resilienceConfig == nil {
		cb.resilienceConfig = &ResilienceConfig{}
	}
	cb.resilienceConfig.CircuitBreaker = CircuitBreakerConfig{
		Enabled:                 enabled,
		FailureThreshold:        failureThreshold,
		SuccessThreshold:        successThreshold,
		Timeout:                 timeout,
		HalfOpenMaxCalls:        successThreshold,
		FailureRateThreshold:    0.5,
		MinimumRequestThreshold: failureThreshold,
	}
	return cb
}

// =============================================================================
// Common Configuration Combinations
// =============================================================================

// ForDevelopment applies development-friendly settings to existing configuration
func (cb *ConfigBuilder) ForDevelopment() *ConfigBuilder {
	// Relax security settings
	cb.WithInsecureTLS()
	cb.WithCORS([]string{"*"}, []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, []string{"*"})
	
	// Disable advanced features that might complicate development
	if cb.httpConfig != nil {
		cb.httpConfig.EnableCircuitBreaker = false
	}
	if cb.resilienceConfig != nil {
		cb.resilienceConfig.CircuitBreaker.Enabled = false
		cb.resilienceConfig.RateLimit.Enabled = false
	}
	if cb.securityConfig != nil {
		cb.securityConfig.RateLimit.Enabled = false
	}
	
	return cb
}

// ForProduction applies production-ready settings to existing configuration
func (cb *ConfigBuilder) ForProduction() *ConfigBuilder {
	// Enable security features
	if cb.securityConfig != nil {
		cb.securityConfig.SecurityHeaders.EnableCSP = true
		cb.securityConfig.SecurityHeaders.EnableHSTS = true
		cb.securityConfig.SecurityHeaders.EnableXFrameOptions = true
		cb.securityConfig.SecurityHeaders.EnableXContentType = true
		cb.securityConfig.AuditLogging.Enabled = true
		cb.securityConfig.RateLimit.Enabled = true
	}
	
	// Enable resilience features
	if cb.httpConfig != nil {
		cb.httpConfig.EnableCircuitBreaker = true
	}
	if cb.resilienceConfig != nil {
		cb.resilienceConfig.CircuitBreaker.Enabled = true
		cb.resilienceConfig.RateLimit.Enabled = true
	}
	
	// Enable HTTP/2 for better performance
	cb.WithHTTP2(true, false)
	
	return cb
}

// ForHighPerformance applies performance-optimized settings
func (cb *ConfigBuilder) ForHighPerformance() *ConfigBuilder {
	// Optimize HTTP settings
	cb.WithHTTP2(true, false)
	cb.WithHTTPConnectionLimits(100, 20, 100)
	cb.WithHTTPTimeouts(10*time.Second, 60*time.Second, 60*time.Second)
	
	// Optimize agent settings
	if cb.agentConfig != nil {
		cb.agentConfig.EventProcessing.BufferSize = 1000
		cb.agentConfig.EventProcessing.BatchSize = 100
		cb.agentConfig.Tools.MaxConcurrent = 50
		cb.agentConfig.State.CacheSize = "1GB"
	}
	
	return cb
}

// ForSecure applies security-focused settings
func (cb *ConfigBuilder) ForSecure() *ConfigBuilder {
	// Enable all security headers
	if cb.securityConfig != nil {
		cb.securityConfig.SecurityHeaders.EnableCSP = true
		cb.securityConfig.SecurityHeaders.EnableHSTS = true
		cb.securityConfig.SecurityHeaders.EnableXFrameOptions = true
		cb.securityConfig.SecurityHeaders.EnableXContentType = true
		cb.securityConfig.SecurityHeaders.EnableReferrerPolicy = true
		
		// Enable audit logging
		cb.securityConfig.AuditLogging.Enabled = true
		cb.securityConfig.AuditLogging.LogLevel = "DEBUG"
		
		// Enable rate limiting
		cb.securityConfig.RateLimit.Enabled = true
		if cb.securityConfig.RateLimit.RequestsPerSecond == 0 {
			cb.securityConfig.RateLimit.RequestsPerSecond = 100
			cb.securityConfig.RateLimit.BurstSize = 20
		}
	}
	
	// Ensure TLS is properly configured
	if cb.securityConfig.TLS.Enabled {
		cb.securityConfig.TLS.MinVersion = tls.VersionTLS12
		cb.securityConfig.TLS.InsecureSkipVerify = false
	}
	
	return cb
}

// =============================================================================
// Build and Validation Methods
// =============================================================================

// Build finalizes the configuration and returns the complete configuration objects
func (cb *ConfigBuilder) Build() (*AgentConfig, *HttpConfig, *SecurityConfig, *SSEClientConfig, *ResilienceConfig, error) {
	// Validate all configurations
	if err := cb.Validate(); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	
	return cb.agentConfig, cb.httpConfig, cb.securityConfig, cb.sseConfig, cb.resilienceConfig, nil
}

// BuildAgentConfig returns just the agent configuration
func (cb *ConfigBuilder) BuildAgentConfig() (*AgentConfig, error) {
	if err := cb.agentConfig.Validate(); err != nil {
		return nil, err
	}
	return cb.agentConfig, nil
}

// BuildHttpConfig returns just the HTTP configuration
func (cb *ConfigBuilder) BuildHttpConfig() (*HttpConfig, error) {
	if err := cb.httpConfig.Validate(); err != nil {
		return nil, err
	}
	return cb.httpConfig, nil
}

// BuildSecurityConfig returns just the security configuration
func (cb *ConfigBuilder) BuildSecurityConfig() (*SecurityConfig, error) {
	if err := cb.securityConfig.Validate(); err != nil {
		return nil, err
	}
	return cb.securityConfig, nil
}

// Validate validates all configurations in the builder
func (cb *ConfigBuilder) Validate() error {
	var allErrors []error
	
	if cb.agentConfig != nil {
		if err := cb.agentConfig.Validate(); err != nil {
			allErrors = append(allErrors, fmt.Errorf("agent config validation failed: %w", err))
		}
	}
	
	if cb.httpConfig != nil {
		if err := cb.httpConfig.Validate(); err != nil {
			allErrors = append(allErrors, fmt.Errorf("http config validation failed: %w", err))
		}
	}
	
	if cb.securityConfig != nil {
		if err := cb.securityConfig.Validate(); err != nil {
			allErrors = append(allErrors, fmt.Errorf("security config validation failed: %w", err))
		}
	}
	
	if cb.sseConfig != nil {
		if err := cb.sseConfig.Validate(); err != nil {
			allErrors = append(allErrors, fmt.Errorf("sse config validation failed: %w", err))
		}
	}
	
	if cb.resilienceConfig != nil {
		if err := cb.resilienceConfig.Validate(); err != nil {
			allErrors = append(allErrors, fmt.Errorf("resilience config validation failed: %w", err))
		}
	}
	
	if len(allErrors) > 0 {
		return fmt.Errorf("configuration validation errors: %v", allErrors)
	}
	
	return nil
}
