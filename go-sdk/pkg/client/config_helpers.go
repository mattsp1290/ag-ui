// Package client provides helper functions for common configuration patterns
// and convenience methods to simplify complex configuration scenarios.
package client

import (
	"crypto/tls"
	"fmt"
	"time"
)

// =============================================================================
// Configuration Helper Functions
// =============================================================================

// QuickHTTPConfig creates a simple HTTP configuration with sensible defaults
func QuickHTTPConfig(timeout time.Duration) *HttpConfig {
	return &HttpConfig{
		ProtocolVersion:     "HTTP/1.1",
		EnableHTTP2:         false,
		ForceHTTP2:          false,
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 2,
		MaxConnsPerHost:     10,
		IdleConnTimeout:     30 * time.Second,
		KeepAlive:           30 * time.Second,
		DialTimeout:         timeout,
		RequestTimeout:      timeout,
		ResponseTimeout:     timeout,
		TLSHandshakeTimeout: 10 * time.Second,
		InsecureSkipVerify:  false,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		UserAgent:           "ag-ui-sdk/1.0",
		MaxResponseBodySize: 50 * 1024 * 1024, // 50MB
		EnableCircuitBreaker: false,
	}
}

// QuickJWTConfig creates a simple JWT configuration with HMAC-SHA256
func QuickJWTConfig(secretKey string, accessTTL, refreshTTL time.Duration) *JWTConfig {
	return &JWTConfig{
		SigningMethod:    "HS256",
		SecretKey:        secretKey,
		AccessTokenTTL:   accessTTL,
		RefreshTokenTTL:  refreshTTL,
		AutoRefresh:      true,
		RefreshThreshold: accessTTL / 4, // Refresh when 25% of TTL remains
		LeewayTime:       30 * time.Second,
		CustomClaims:     make(map[string]interface{}),
	}
}

// QuickAPIKeyConfig creates a simple API key configuration
func QuickAPIKeyConfig(headerName, prefix string) *APIKeyConfig {
	return &APIKeyConfig{
		HeaderName:              headerName,
		QueryParam:              "api_key",
		Prefix:                  prefix,
		HashingAlgorithm:        "sha256",
		EnableKeyRotation:       false,
		KeyRotationInterval:     0,
	}
}

// QuickTLSConfig creates a TLS configuration with secure defaults
func QuickTLSConfig(certFile, keyFile string) *TLSConfig {
	return &TLSConfig{
		Enabled:            true,
		CertFile:           certFile,
		KeyFile:            keyFile,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: false,
		EnableSNI:          true,
	}
}

// QuickCORSConfig creates a permissive CORS configuration for development
func QuickCORSConfig(allowedOrigins []string) *CORSConfig {
	return &CORSConfig{
		Enabled:         true,
		AllowedOrigins:  allowedOrigins,
		AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:  []string{"Authorization", "Content-Type", "X-API-Key"},
		AllowCredentials: len(allowedOrigins) > 0 && allowedOrigins[0] != "*",
		MaxAge:          3600, // 1 hour
	}
}

// QuickRateLimitConfig creates a rate limiting configuration
func QuickRateLimitConfig(requestsPerSecond float64, burstSize int) *RateLimitConfig {
	return &RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: requestsPerSecond,
		BurstSize:         burstSize,
	}
}

// QuickRetryConfig creates a retry configuration with exponential backoff
func QuickRetryConfig(maxAttempts int, baseDelay time.Duration) *RetryConfig {
	return &RetryConfig{
		MaxAttempts:       maxAttempts,
		BaseDelay:         baseDelay,
		MaxDelay:          baseDelay * 16, // Max delay is 16x base delay
		BackoffMultiplier: 2.0,
		JitterEnabled:     true,
		JitterMaxFactor:   0.1,
		RetryableErrors:   []string{"timeout", "connection_error", "service_unavailable"},
	}
}

// QuickCircuitBreakerConfig creates a circuit breaker configuration
func QuickCircuitBreakerConfig(failureThreshold int, timeout time.Duration) *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Enabled:                 true,
		FailureThreshold:        failureThreshold,
		SuccessThreshold:        failureThreshold / 2, // Half of failure threshold
		Timeout:                 timeout,
		HalfOpenMaxCalls:        3,
		FailureRateThreshold:    0.5,
		MinimumRequestThreshold: failureThreshold,
	}
}

// =============================================================================
// Configuration Pattern Helpers
// =============================================================================

// MicroserviceConfig creates a configuration suitable for microservice communication
func MicroserviceConfig(serviceName, baseURL string) *ConfigBuilder {
	return NewConfigBuilder().
		WithAgentName(serviceName).
		WithAgentDescription(fmt.Sprintf("Microservice agent for %s", serviceName)).
		WithHTTPTimeouts(5*time.Second, 30*time.Second, 30*time.Second).
		WithHTTPConnectionLimits(50, 10, 25).
		WithHTTP2(true, false).
		WithRetry(3, 1*time.Second, 10*time.Second).
		WithCircuitBreaker(true, 10, 3, 60*time.Second)
}

// DatabaseAgentConfig creates a configuration suitable for database operations
func DatabaseAgentConfig(dbName string) *ConfigBuilder {
	return NewConfigBuilder().
		WithAgentName(fmt.Sprintf("%s-db-agent", dbName)).
		WithAgentDescription(fmt.Sprintf("Database agent for %s", dbName)).
		WithHTTPTimeouts(10*time.Second, 60*time.Second, 60*time.Second).
		WithHTTPConnectionLimits(20, 5, 10).
		WithRetry(5, 2*time.Second, 30*time.Second).
		WithCircuitBreaker(true, 5, 2, 30*time.Second)
}

// APIGatewayConfig creates a configuration suitable for API gateway scenarios
func APIGatewayConfig() *ConfigBuilder {
	return NewConfigBuilder().
		WithAgentName("api-gateway-agent").
		WithAgentDescription("API Gateway agent for request routing").
		WithHTTPTimeouts(2*time.Second, 15*time.Second, 15*time.Second).
		WithHTTPConnectionLimits(100, 20, 50).
		WithHTTP2(true, false).
		WithRateLimit(1000, 100). // High rate limit for gateway
		WithRetry(2, 500*time.Millisecond, 2*time.Second). // Fast retries
		WithCircuitBreaker(true, 20, 5, 30*time.Second)
}

// StreamingConfig creates a configuration optimized for streaming operations
func StreamingConfig(streamName string) *ConfigBuilder {
	return NewConfigBuilder().
		WithAgentName(fmt.Sprintf("%s-streaming-agent", streamName)).
		WithAgentDescription(fmt.Sprintf("Streaming agent for %s", streamName)).
		WithHTTPTimeouts(5*time.Second, 0, 0). // No timeout for streaming
		WithHTTPConnectionLimits(20, 5, 10).
		WithSSEBackoff(1*time.Second, 30*time.Second, 2.0).
		WithRetry(3, 2*time.Second, 10*time.Second)
}

// BatchProcessingConfig creates a configuration for batch processing
func BatchProcessingConfig(batchName string) *ConfigBuilder {
	return NewConfigBuilder().
		WithAgentName(fmt.Sprintf("%s-batch-agent", batchName)).
		WithAgentDescription(fmt.Sprintf("Batch processing agent for %s", batchName)).
		WithHTTPTimeouts(30*time.Second, 5*time.Minute, 5*time.Minute). // Long timeouts for batch
		WithHTTPConnectionLimits(5, 2, 3). // Few connections for batch
		WithRetry(2, 5*time.Second, 30*time.Second). // Longer delays for batch
		WithCircuitBreaker(false, 0, 0, 0) // Disable circuit breaker for batch
}

// =============================================================================
// Security Configuration Helpers
// =============================================================================

// DevSecurityConfig creates a relaxed security configuration for development
func DevSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		AuthMethod:       AuthMethodAPIKey,
		EnableMultiAuth:  false,
		SupportedMethods: []AuthMethod{AuthMethodAPIKey},
		APIKey: APIKeyConfig{
			HeaderName:              "X-API-Key",
			QueryParam:              "api_key",
			Prefix:                  "",
			HashingAlgorithm:        "sha256",
			EnableKeyRotation:       false,
		},
		TLS: TLSConfig{
			Enabled:               false,
			InsecureSkipVerify:    true,
		},
		SecurityHeaders: SecurityHeadersConfig{
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
			Enabled: false,
		},
	}
}

// ProdSecurityConfig creates a secure configuration for production
func ProdSecurityConfig(jwtSecret string) *SecurityConfig {
	return &SecurityConfig{
		AuthMethod:       AuthMethodJWT,
		EnableMultiAuth:  true,
		SupportedMethods: []AuthMethod{AuthMethodJWT, AuthMethodAPIKey, AuthMethodMTLS},
		JWT: JWTConfig{
			SigningMethod:    "HS256",
			SecretKey:        jwtSecret,
			AccessTokenTTL:   15 * time.Minute,
			RefreshTokenTTL:  24 * time.Hour,
			AutoRefresh:      true,
			RefreshThreshold: 2 * time.Minute,
			LeewayTime:       30 * time.Second,
			CustomClaims:     make(map[string]interface{}),
		},
		APIKey: APIKeyConfig{
			HeaderName:              "X-API-Key",
			Prefix:                  "Bearer ",
			HashingAlgorithm:        "sha256",
			EnableKeyRotation:       true,
			KeyRotationInterval:     24 * time.Hour,
		},
		TLS: TLSConfig{
			Enabled:            true,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS13,
			InsecureSkipVerify: false,
			EnableSNI:          true,
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
			CustomHeaders: map[string]string{
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
}

// =============================================================================
// Environment-Specific Helpers
// =============================================================================

// LoadBalancerConfig creates configuration for load balancer scenarios
func LoadBalancerConfig(lbName string) *ConfigBuilder {
	return NewConfigBuilder().
		WithAgentName(fmt.Sprintf("%s-lb-agent", lbName)).
		WithAgentDescription(fmt.Sprintf("Load balancer agent for %s", lbName)).
		WithHTTPTimeouts(1*time.Second, 10*time.Second, 10*time.Second). // Fast timeouts
		WithHTTPConnectionLimits(200, 50, 100). // High connection limits
		WithHTTP2(true, false).
		WithRateLimit(5000, 500). // Very high rate limits
		WithRetry(1, 100*time.Millisecond, 500*time.Millisecond). // Very fast retries
		WithCircuitBreaker(true, 50, 10, 10*time.Second) // Sensitive circuit breaker
}

// EdgeComputingConfig creates configuration for edge computing scenarios
func EdgeComputingConfig(edgeName string) *ConfigBuilder {
	return NewConfigBuilder().
		WithAgentName(fmt.Sprintf("%s-edge-agent", edgeName)).
		WithAgentDescription(fmt.Sprintf("Edge computing agent for %s", edgeName)).
		WithHTTPTimeouts(3*time.Second, 20*time.Second, 20*time.Second).
		WithHTTPConnectionLimits(10, 3, 5). // Limited resources at edge
		WithRetry(5, 1*time.Second, 30*time.Second). // More retries for unreliable networks
		WithCircuitBreaker(true, 3, 1, 60*time.Second) // Sensitive to failures
}

// TestingConfig creates configuration optimized for testing
func TestingConfig(testName string) *ConfigBuilder {
	return NewConfigBuilder().
		WithAgentName(fmt.Sprintf("%s-test-agent", testName)).
		WithAgentDescription(fmt.Sprintf("Test agent for %s", testName)).
		WithHTTPTimeouts(1*time.Second, 5*time.Second, 5*time.Second). // Fast timeouts for tests
		WithHTTPConnectionLimits(5, 2, 3). // Limited connections for tests
		WithInsecureTLS(). // Allow insecure connections for testing
		WithRetry(1, 100*time.Millisecond, 500*time.Millisecond). // Minimal retries
		WithCircuitBreaker(false, 0, 0, 0) // Disable circuit breaker for tests
}

// =============================================================================
// Configuration Templates
// =============================================================================

// ConfigTemplate represents a reusable configuration template
type ConfigTemplate struct {
	Name        string
	Description string
	Builder     func() *ConfigBuilder
}

// GetConfigTemplates returns a list of predefined configuration templates
func GetConfigTemplates() []ConfigTemplate {
	return []ConfigTemplate{
		{
			Name:        "development",
			Description: "Development-friendly configuration with relaxed security",
			Builder:     func() *ConfigBuilder { return NewDevelopmentConfig() },
		},
		{
			Name:        "production",
			Description: "Production-ready configuration with full security",
			Builder:     func() *ConfigBuilder { return NewProductionConfig() },
		},
		{
			Name:        "minimal",
			Description: "Minimal configuration with basic functionality",
			Builder:     func() *ConfigBuilder { return NewMinimalConfig() },
		},
		{
			Name:        "microservice",
			Description: "Optimized for microservice communication",
			Builder:     func() *ConfigBuilder { return MicroserviceConfig("default", "http://localhost:8080") },
		},
		{
			Name:        "api-gateway",
			Description: "Optimized for API gateway scenarios",
			Builder:     func() *ConfigBuilder { return APIGatewayConfig() },
		},
		{
			Name:        "streaming",
			Description: "Optimized for streaming operations",
			Builder:     func() *ConfigBuilder { return StreamingConfig("default") },
		},
		{
			Name:        "batch-processing",
			Description: "Optimized for batch processing",
			Builder:     func() *ConfigBuilder { return BatchProcessingConfig("default") },
		},
		{
			Name:        "load-balancer",
			Description: "Optimized for load balancer scenarios",
			Builder:     func() *ConfigBuilder { return LoadBalancerConfig("default") },
		},
		{
			Name:        "edge-computing",
			Description: "Optimized for edge computing scenarios",
			Builder:     func() *ConfigBuilder { return EdgeComputingConfig("default") },
		},
		{
			Name:        "testing",
			Description: "Optimized for testing scenarios",
			Builder:     func() *ConfigBuilder { return TestingConfig("default") },
		},
	}
}

// GetConfigTemplate returns a specific configuration template by name
func GetConfigTemplate(name string) (*ConfigTemplate, error) {
	templates := GetConfigTemplates()
	for _, template := range templates {
		if template.Name == name {
			return &template, nil
		}
	}
	return nil, fmt.Errorf("configuration template '%s' not found", name)
}