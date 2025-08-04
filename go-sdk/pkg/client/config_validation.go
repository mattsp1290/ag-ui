// Package client provides configuration validation methods to ensure
// configuration integrity and provide clear error messages.
package client

import (
	"fmt"
	"net/url"
	"strings"
	"crypto/tls"
)

// =============================================================================
// AgentConfig Validation
// =============================================================================

// Validate validates the AgentConfig and returns detailed errors
func (ac *AgentConfig) Validate() error {
	var errors []string
	
	// Validate name
	if strings.TrimSpace(ac.Name) == "" {
		errors = append(errors, "agent name cannot be empty")
	}
	
	// Validate event processing
	if ac.EventProcessing.BufferSize < 0 {
		errors = append(errors, "event processing buffer size cannot be negative")
	}
	if ac.EventProcessing.BatchSize < 0 {
		errors = append(errors, "event processing batch size cannot be negative")
	}
	if ac.EventProcessing.BatchSize > ac.EventProcessing.BufferSize {
		errors = append(errors, "event processing batch size cannot exceed buffer size")
	}
	if ac.EventProcessing.Timeout < 0 {
		errors = append(errors, "event processing timeout cannot be negative")
	}
	
	// Validate state configuration
	if ac.State.SyncInterval < 0 {
		errors = append(errors, "state sync interval cannot be negative")
	}
	if !isValidCacheSize(ac.State.CacheSize) {
		errors = append(errors, "invalid cache size format, use formats like '100MB', '1GB'")
	}
	
	// Validate tools configuration
	if ac.Tools.Timeout < 0 {
		errors = append(errors, "tools timeout cannot be negative")
	}
	if ac.Tools.MaxConcurrent < 0 {
		errors = append(errors, "tools max concurrent cannot be negative")
	}
	
	// Validate history configuration
	if ac.History.MaxMessages < 0 {
		errors = append(errors, "history max messages cannot be negative")
	}
	if ac.History.Retention < 0 {
		errors = append(errors, "history retention cannot be negative")
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("agent config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// =============================================================================
// HttpConfig Validation
// =============================================================================

// Validate validates the HttpConfig and returns detailed errors
func (hc *HttpConfig) Validate() error {
	var errors []string
	
	// Validate protocol version
	if hc.ProtocolVersion != "" && hc.ProtocolVersion != "HTTP/1.1" && hc.ProtocolVersion != "HTTP/2" {
		errors = append(errors, "protocol version must be 'HTTP/1.1' or 'HTTP/2'")
	}
	
	// Validate connection limits
	if hc.MaxIdleConns < 0 {
		errors = append(errors, "max idle connections cannot be negative")
	}
	if hc.MaxIdleConnsPerHost < 0 {
		errors = append(errors, "max idle connections per host cannot be negative")
	}
	if hc.MaxConnsPerHost < 0 {
		errors = append(errors, "max connections per host cannot be negative")
	}
	if hc.MaxIdleConnsPerHost > hc.MaxIdleConns && hc.MaxIdleConns > 0 {
		errors = append(errors, "max idle connections per host cannot exceed max idle connections")
	}
	
	// Validate timeouts
	if hc.DialTimeout < 0 {
		errors = append(errors, "dial timeout cannot be negative")
	}
	if hc.RequestTimeout < 0 {
		errors = append(errors, "request timeout cannot be negative")
	}
	if hc.ResponseTimeout < 0 {
		errors = append(errors, "response timeout cannot be negative")
	}
	if hc.TLSHandshakeTimeout < 0 {
		errors = append(errors, "TLS handshake timeout cannot be negative")
	}
	if hc.IdleConnTimeout < 0 {
		errors = append(errors, "idle connection timeout cannot be negative")
	}
	if hc.KeepAlive < 0 {
		errors = append(errors, "keep alive cannot be negative")
	}
	
	// Validate response body size
	if hc.MaxResponseBodySize < 0 {
		errors = append(errors, "max response body size cannot be negative")
	}
	
	// Validate HTTP/2 settings
	if hc.ForceHTTP2 && !hc.EnableHTTP2 {
		errors = append(errors, "cannot force HTTP/2 when HTTP/2 is disabled")
	}
	
	// Validate user agent
	if strings.TrimSpace(hc.UserAgent) == "" {
		errors = append(errors, "user agent cannot be empty")
	}
	
	// Validate circuit breaker config if enabled
	if hc.EnableCircuitBreaker && hc.CircuitBreakerConfig != nil {
		if err := hc.CircuitBreakerConfig.Validate(); err != nil {
			errors = append(errors, fmt.Sprintf("circuit breaker config: %v", err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("http config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// =============================================================================
// SecurityConfig Validation
// =============================================================================

// Validate validates the SecurityConfig and returns detailed errors
func (sc *SecurityConfig) Validate() error {
	var errors []string
	
	// Validate auth method
	if sc.AuthMethod == "" {
		errors = append(errors, "authentication method cannot be empty")
	} else if !isValidAuthMethod(sc.AuthMethod) {
		errors = append(errors, fmt.Sprintf("invalid authentication method: %s", sc.AuthMethod))
	}
	
	// Validate supported methods if multi-auth is enabled
	if sc.EnableMultiAuth {
		if len(sc.SupportedMethods) == 0 {
			errors = append(errors, "supported methods cannot be empty when multi-auth is enabled")
		}
		for _, method := range sc.SupportedMethods {
			if !isValidAuthMethod(method) {
				errors = append(errors, fmt.Sprintf("invalid supported auth method: %s", method))
			}
		}
	}
	
	// Validate JWT config if JWT auth is used
	if sc.AuthMethod == AuthMethodJWT || containsAuthMethod(sc.SupportedMethods, AuthMethodJWT) {
		if err := sc.JWT.Validate(); err != nil {
			errors = append(errors, fmt.Sprintf("JWT config: %v", err))
		}
	}
	
	// Validate API Key config if API Key auth is used
	if sc.AuthMethod == AuthMethodAPIKey || containsAuthMethod(sc.SupportedMethods, AuthMethodAPIKey) {
		if err := sc.APIKey.Validate(); err != nil {
			errors = append(errors, fmt.Sprintf("API Key config: %v", err))
		}
	}
	
	// Validate Basic Auth config if Basic auth is used
	if sc.AuthMethod == AuthMethodBasic || containsAuthMethod(sc.SupportedMethods, AuthMethodBasic) {
		if err := sc.BasicAuth.Validate(); err != nil {
			errors = append(errors, fmt.Sprintf("Basic Auth config: %v", err))
		}
	}
	
	// Validate TLS config
	if err := sc.TLS.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("TLS config: %v", err))
	}
	
	// Validate HMAC config if HMAC auth is used
	if sc.AuthMethod == AuthMethodHMAC || containsAuthMethod(sc.SupportedMethods, AuthMethodHMAC) {
		if err := sc.HMAC.Validate(); err != nil {
			errors = append(errors, fmt.Sprintf("HMAC config: %v", err))
		}
	}
	
	// Validate security headers config
	if err := sc.SecurityHeaders.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("security headers config: %v", err))
	}
	
	// Validate token storage config
	if err := sc.TokenStorage.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("token storage config: %v", err))
	}
	
	// Validate audit logging config
	if err := sc.AuditLogging.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("audit logging config: %v", err))
	}
	
	// Validate rate limit config
	if err := sc.RateLimit.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("rate limit config: %v", err))
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("security config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Validate validates JWTConfig
func (jc *JWTConfig) Validate() error {
	var errors []string
	
	if strings.TrimSpace(jc.SigningMethod) == "" {
		errors = append(errors, "signing method cannot be empty")
	}
	
	// Validate signing method and corresponding key requirements
	switch strings.ToUpper(jc.SigningMethod) {
	case "HS256", "HS384", "HS512":
		if strings.TrimSpace(jc.SecretKey) == "" {
			errors = append(errors, "secret key required for HMAC signing methods")
		}
	case "RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512":
		if strings.TrimSpace(jc.PrivateKeyPath) == "" && strings.TrimSpace(jc.PublicKeyPath) == "" {
			errors = append(errors, "private/public key paths required for asymmetric signing methods")
		}
	default:
		errors = append(errors, fmt.Sprintf("unsupported signing method: %s", jc.SigningMethod))
	}
	
	if jc.AccessTokenTTL <= 0 {
		errors = append(errors, "access token TTL must be positive")
	}
	
	if jc.RefreshTokenTTL <= 0 {
		errors = append(errors, "refresh token TTL must be positive")
	}
	
	if jc.RefreshTokenTTL <= jc.AccessTokenTTL {
		errors = append(errors, "refresh token TTL should be longer than access token TTL")
	}
	
	if jc.AutoRefresh && jc.RefreshThreshold >= jc.AccessTokenTTL {
		errors = append(errors, "refresh threshold must be less than access token TTL")
	}
	
	if jc.LeewayTime < 0 {
		errors = append(errors, "leeway time cannot be negative")
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("JWT config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Validate validates APIKeyConfig
func (akc *APIKeyConfig) Validate() error {
	var errors []string
	
	if strings.TrimSpace(akc.HeaderName) == "" && strings.TrimSpace(akc.QueryParam) == "" {
		errors = append(errors, "either header name or query param must be specified")
	}
	
	if strings.TrimSpace(akc.HashingAlgorithm) == "" {
		errors = append(errors, "hashing algorithm cannot be empty")
	} else if !isValidHashingAlgorithm(akc.HashingAlgorithm) {
		errors = append(errors, fmt.Sprintf("unsupported hashing algorithm: %s", akc.HashingAlgorithm))
	}
	
	if akc.EnableKeyRotation && akc.KeyRotationInterval <= 0 {
		errors = append(errors, "key rotation interval must be positive when key rotation is enabled")
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("API key config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Validate validates BasicAuthConfig
func (bac *BasicAuthConfig) Validate() error {
	var errors []string
	
	if strings.TrimSpace(bac.Realm) == "" {
		errors = append(errors, "realm cannot be empty")
	}
	
	if strings.TrimSpace(bac.HashingAlgorithm) == "" {
		errors = append(errors, "hashing algorithm cannot be empty")
	} else if !isValidHashingAlgorithm(bac.HashingAlgorithm) {
		errors = append(errors, fmt.Sprintf("unsupported hashing algorithm: %s", bac.HashingAlgorithm))
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("basic auth config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Validate validates TLSConfig
func (tc *TLSConfig) Validate() error {
	var errors []string
	
	if tc.Enabled {
		if strings.TrimSpace(tc.CertFile) == "" {
			errors = append(errors, "certificate file path cannot be empty when TLS is enabled")
		}
		if strings.TrimSpace(tc.KeyFile) == "" {
			errors = append(errors, "key file path cannot be empty when TLS is enabled")
		}
		
		// Validate TLS version range
		if tc.MinVersion > tc.MaxVersion && tc.MaxVersion != 0 {
			errors = append(errors, "minimum TLS version cannot be higher than maximum TLS version")
		}
		
		// Validate minimum TLS version is secure
		if tc.MinVersion != 0 && tc.MinVersion < tls.VersionTLS12 {
			errors = append(errors, "minimum TLS version should be at least TLS 1.2 for security")
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("TLS config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Validate validates HMACConfig
func (hc *HMACConfig) Validate() error {
	var errors []string
	
	if strings.TrimSpace(hc.SecretKey) == "" {
		errors = append(errors, "secret key cannot be empty")
	}
	
	if strings.TrimSpace(hc.Algorithm) == "" {
		errors = append(errors, "algorithm cannot be empty")
	} else if !isValidHMACAlgorithm(hc.Algorithm) {
		errors = append(errors, fmt.Sprintf("unsupported HMAC algorithm: %s", hc.Algorithm))
	}
	
	if strings.TrimSpace(hc.HeaderName) == "" {
		errors = append(errors, "header name cannot be empty")
	}
	
	if hc.MaxClockSkew < 0 {
		errors = append(errors, "max clock skew cannot be negative")
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("HMAC config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Validate validates SecurityHeadersConfig
func (shc *SecurityHeadersConfig) Validate() error {
	var errors []string
	
	// Validate CSP policy if CSP is enabled
	if shc.EnableCSP && strings.TrimSpace(shc.CSPPolicy) == "" {
		errors = append(errors, "CSP policy cannot be empty when CSP is enabled")
	}
	
	// Validate HSTS settings
	if shc.EnableHSTS && shc.HSTSMaxAge <= 0 {
		errors = append(errors, "HSTS max age must be positive when HSTS is enabled")
	}
	
	// Validate X-Frame-Options
	if shc.EnableXFrameOptions && strings.TrimSpace(shc.XFrameOptions) == "" {
		errors = append(errors, "X-Frame-Options value cannot be empty when enabled")
	} else if shc.EnableXFrameOptions && !isValidXFrameOptions(shc.XFrameOptions) {
		errors = append(errors, fmt.Sprintf("invalid X-Frame-Options value: %s", shc.XFrameOptions))
	}
	
	// Validate Referrer Policy
	if shc.EnableReferrerPolicy && strings.TrimSpace(shc.ReferrerPolicy) == "" {
		errors = append(errors, "referrer policy cannot be empty when enabled")
	}
	
	// Validate CORS config
	if err := shc.CORSConfig.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("CORS config: %v", err))
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("security headers config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Validate validates CORSConfig
func (cc *CORSConfig) Validate() error {
	var errors []string
	
	if cc.Enabled {
		if len(cc.AllowedOrigins) == 0 {
			errors = append(errors, "allowed origins cannot be empty when CORS is enabled")
		}
		
		// Validate origins format
		for _, origin := range cc.AllowedOrigins {
			if origin != "*" {
				if _, err := url.Parse(origin); err != nil {
					errors = append(errors, fmt.Sprintf("invalid origin URL: %s", origin))
				}
			}
		}
		
		if len(cc.AllowedMethods) == 0 {
			errors = append(errors, "allowed methods cannot be empty when CORS is enabled")
		}
		
		if cc.MaxAge < 0 {
			errors = append(errors, "CORS max age cannot be negative")
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("CORS config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Validate validates TokenStorageConfig
func (tsc *TokenStorageConfig) Validate() error {
	var errors []string
	
	if strings.TrimSpace(tsc.StorageType) == "" {
		errors = append(errors, "storage type cannot be empty")
	} else if !isValidStorageType(tsc.StorageType) {
		errors = append(errors, fmt.Sprintf("unsupported storage type: %s", tsc.StorageType))
	}
	
	// Validate encryption config
	if err := tsc.Encryption.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("encryption config: %v", err))
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("token storage config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Validate validates AuditLoggingConfig (stub implementation)
func (alc *AuditLoggingConfig) Validate() error {
	// Basic validation - can be expanded based on actual AuditLoggingConfig structure
	return nil
}

// Validate validates RateLimitConfig
func (rlc *RateLimitConfig) Validate() error {
	var errors []string
	
	if rlc.Enabled {
		if rlc.RequestsPerSecond <= 0 {
			errors = append(errors, "requests per second must be positive when rate limiting is enabled")
		}
		
		if rlc.BurstSize <= 0 {
			errors = append(errors, "burst size must be positive when rate limiting is enabled")
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("rate limit config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// =============================================================================
// SSEClientConfig Validation
// =============================================================================

// Validate validates the SSEClientConfig and returns detailed errors
func (sc *SSEClientConfig) Validate() error {
	var errors []string
	
	if strings.TrimSpace(sc.URL) != "" {
		if _, err := url.Parse(sc.URL); err != nil {
			errors = append(errors, fmt.Sprintf("invalid SSE URL: %v", err))
		}
	}
	
	if sc.InitialBackoff < 0 {
		errors = append(errors, "initial backoff cannot be negative")
	}
	
	if sc.MaxBackoff < 0 {
		errors = append(errors, "max backoff cannot be negative")
	}
	
	if sc.MaxBackoff < sc.InitialBackoff {
		errors = append(errors, "max backoff cannot be less than initial backoff")
	}
	
	if sc.BackoffMultiplier <= 1.0 {
		errors = append(errors, "backoff multiplier must be greater than 1.0")
	}
	
	if sc.MaxReconnectAttempts < 0 {
		errors = append(errors, "max reconnect attempts cannot be negative")
	}
	
	if sc.EventBufferSize < 0 {
		errors = append(errors, "event buffer size cannot be negative")
	}
	
	if sc.ReadTimeout < 0 {
		errors = append(errors, "read timeout cannot be negative")
	}
	
	if sc.WriteTimeout < 0 {
		errors = append(errors, "write timeout cannot be negative")
	}
	
	if sc.HealthCheckInterval < 0 {
		errors = append(errors, "health check interval cannot be negative")
	}
	
	if sc.MaxStreamLifetime < 0 {
		errors = append(errors, "max stream lifetime cannot be negative")
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("SSE client config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// =============================================================================
// ResilienceConfig Validation
// =============================================================================

// Validate validates the ResilienceConfig and returns detailed errors
func (rc *ResilienceConfig) Validate() error {
	var errors []string
	
	// Validate retry config
	if err := rc.Retry.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("retry config: %v", err))
	}
	
	// Validate circuit breaker config
	if err := rc.CircuitBreaker.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("circuit breaker config: %v", err))
	}
	
	// Validate rate limit config
	if err := rc.RateLimit.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("rate limit config: %v", err))
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("resilience config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Validate validates RetryConfig
func (rc *RetryConfig) Validate() error {
	var errors []string
	
	if rc.MaxAttempts < 0 {
		errors = append(errors, "max attempts cannot be negative")
	}
	
	if rc.BaseDelay < 0 {
		errors = append(errors, "base delay cannot be negative")
	}
	
	if rc.MaxDelay < 0 {
		errors = append(errors, "max delay cannot be negative")
	}
	
	if rc.MaxDelay < rc.BaseDelay {
		errors = append(errors, "max delay cannot be less than base delay")
	}
	
	if rc.BackoffMultiplier <= 1.0 {
		errors = append(errors, "backoff multiplier must be greater than 1.0")
	}
	
	if rc.JitterMaxFactor < 0 || rc.JitterMaxFactor > 1.0 {
		errors = append(errors, "jitter max factor must be between 0.0 and 1.0")
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("retry config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Validate validates CircuitBreakerConfig
func (cbc *CircuitBreakerConfig) Validate() error {
	var errors []string
	
	if cbc.Enabled {
		if cbc.FailureThreshold <= 0 {
			errors = append(errors, "failure threshold must be positive when circuit breaker is enabled")
		}
		
		if cbc.SuccessThreshold <= 0 {
			errors = append(errors, "success threshold must be positive when circuit breaker is enabled")
		}
		
		if cbc.Timeout <= 0 {
			errors = append(errors, "timeout must be positive when circuit breaker is enabled")
		}
		
		if cbc.HalfOpenMaxCalls <= 0 {
			errors = append(errors, "half open max calls must be positive when circuit breaker is enabled")
		}
		
		if cbc.FailureRateThreshold <= 0 || cbc.FailureRateThreshold > 1.0 {
			errors = append(errors, "failure rate threshold must be between 0.0 and 1.0")
		}
		
		if cbc.MinimumRequestThreshold <= 0 {
			errors = append(errors, "minimum request threshold must be positive when circuit breaker is enabled")
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("circuit breaker config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// =============================================================================
// Helper Functions for Validation
// =============================================================================

// isValidCacheSize validates cache size format (e.g., "100MB", "1GB")
func isValidCacheSize(size string) bool {
	if strings.TrimSpace(size) == "" {
		return false
	}
	
	// Simple validation for common formats
	validSuffixes := []string{"B", "KB", "MB", "GB", "TB"}
	for _, suffix := range validSuffixes {
		if strings.HasSuffix(strings.ToUpper(size), suffix) {
			return true
		}
	}
	
	return false
}

// isValidAuthMethod validates authentication method
func isValidAuthMethod(method AuthMethod) bool {
	validMethods := []AuthMethod{AuthMethodJWT, AuthMethodAPIKey, AuthMethodBasic, AuthMethodBearer, AuthMethodMTLS, AuthMethodHMAC}
	for _, valid := range validMethods {
		if method == valid {
			return true
		}
	}
	return false
}

// isValidHashingAlgorithm validates hashing algorithm
func isValidHashingAlgorithm(algorithm string) bool {
	validAlgorithms := []string{"sha256", "sha512", "bcrypt", "argon2", "scrypt"}
	for _, valid := range validAlgorithms {
		if strings.ToLower(algorithm) == valid {
			return true
		}
	}
	return false
}

// isValidHMACAlgorithm validates HMAC algorithm
func isValidHMACAlgorithm(algorithm string) bool {
	validAlgorithms := []string{"hmac-sha256", "hmac-sha512", "sha256", "sha512"}
	for _, valid := range validAlgorithms {
		if strings.ToLower(algorithm) == valid {
			return true
		}
	}
	return false
}

// isValidXFrameOptions validates X-Frame-Options values
func isValidXFrameOptions(value string) bool {
	validOptions := []string{"DENY", "SAMEORIGIN", "ALLOW-FROM"}
	upper := strings.ToUpper(value)
	for _, valid := range validOptions {
		if upper == valid || strings.HasPrefix(upper, "ALLOW-FROM ") {
			return true
		}
	}
	return false
}

// isValidStorageType validates token storage type
func isValidStorageType(storageType string) bool {
	validTypes := []string{"memory", "encrypted_memory", "file", "redis", "database"}
	for _, valid := range validTypes {
		if strings.ToLower(storageType) == valid {
			return true
		}
	}
	return false
}

// containsAuthMethod checks if a slice contains a specific AuthMethod value
func containsAuthMethod(slice []AuthMethod, item AuthMethod) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// =============================================================================
// Additional Configuration Types (placeholders for missing types)
// =============================================================================

// EncryptionConfig is defined in auth.go

// Validate validates EncryptionConfig
func (ec *EncryptionConfig) Validate() error {
	var errors []string
	
	if ec.Enabled {
		if strings.TrimSpace(ec.Algorithm) == "" {
			errors = append(errors, "encryption algorithm cannot be empty when encryption is enabled")
		} else if !isValidEncryptionAlgorithm(ec.Algorithm) {
			errors = append(errors, fmt.Sprintf("unsupported encryption algorithm: %s", ec.Algorithm))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("encryption config validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// isValidEncryptionAlgorithm validates encryption algorithm
func isValidEncryptionAlgorithm(algorithm string) bool {
	validAlgorithms := []string{"AES-128-GCM", "AES-256-GCM", "ChaCha20-Poly1305"}
	for _, valid := range validAlgorithms {
		if strings.ToUpper(algorithm) == valid {
			return true
		}
	}
	return false
}