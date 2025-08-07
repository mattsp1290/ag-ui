package security

import (
	"fmt"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/auth"
)

// SecureMiddlewareFactory provides secure middleware creation with proper secret handling
type SecureMiddlewareFactory struct {
	configManager *SecureConfigManager
	secretManager *SecretManager
	validator     *EnhancedInputValidator
}

// NewSecureMiddlewareFactory creates a new secure middleware factory
func NewSecureMiddlewareFactory() (*SecureMiddlewareFactory, error) {
	// Initialize secret manager with secure defaults
	secretConfig := &SecretConfig{
		EnvPrefix:              "AGUI_",
		ValidateSecretStrength: true,
		MinSecretLength:        32,
		AllowConfigFallback:    false, // Secure by default - no config fallback
		RequiredSecrets: []string{
			"jwt_secret", "oauth2_client_secret", "encryption_key",
		},
	}

	secretManager, err := NewSecretManager(secretConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secret manager: %w", err)
	}

	// Initialize secure config manager
	configOptions := &SecureConfigOptions{
		SecretManager: secretManager,
		EnvPrefix:     "AGUI_",
		RedactSecrets: true,
		SecretFields: []string{
			"secret", "client_secret", "private_key", "jwt_secret",
			"api_key", "auth_token", "password", "token",
		},
	}

	configManager, err := NewSecureConfigManager(configOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize config manager: %w", err)
	}

	// Initialize enhanced input validator
	validator, err := NewEnhancedInputValidator(nil) // Use secure defaults
	if err != nil {
		return nil, fmt.Errorf("failed to initialize input validator: %w", err)
	}

	return &SecureMiddlewareFactory{
		configManager: configManager,
		secretManager: secretManager,
		validator:     validator,
	}, nil
}

// CreateSecureJWTMiddleware creates JWT middleware with secure secret handling
func (smf *SecureMiddlewareFactory) CreateSecureJWTMiddleware(config map[string]interface{}) (*SecureJWTMiddleware, error) {
	// Create JWT configuration with secure defaults
	jwtConfig := &auth.JWTConfig{
		Algorithm:        "HS256",
		Expiration:       24 * time.Hour,
		RefreshWindow:    time.Hour,
		ClaimsValidation: true,
	}

	// Securely load JWT secret
	secret, err := smf.secretManager.GetSecretWithFallback(
		"jwt_secret",
		"jwt_signing_key",
		"auth_secret",
	)
	if err != nil {
		// Generate a secure secret if none exists (development only)
		if smf.isAllowedToGenerateSecrets() {
			secret, err = smf.secretManager.GenerateSecret(64)
			if err != nil {
				return nil, fmt.Errorf("failed to generate JWT secret: %w", err)
			}
			smf.secretManager.SetSecret("jwt_secret", secret)
		} else {
			return nil, fmt.Errorf("JWT secret not found and secret generation disabled: %w", err)
		}
	}
	jwtConfig.Secret = secret

	// Process other configuration securely
	if err := smf.processJWTConfig(jwtConfig, config); err != nil {
		return nil, fmt.Errorf("failed to process JWT config: %w", err)
	}

	// Create the middleware
	middleware, err := auth.NewJWTMiddleware(jwtConfig, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT middleware: %w", err)
	}

	// Create secure config
	secureConfig := &SecureMiddlewareConfig{
		EnableInputValidation: true,
		EnableSecurityHeaders: true,
		EnableAuditLogging:    true,
		MaxRequestSize:        1024 * 1024, // 1MB
		AllowedContentTypes:   []string{"application/json", "application/x-www-form-urlencoded"},
	}

	return NewSecureJWTMiddleware(*middleware, secureConfig), nil
}

// CreateSecureOAuth2Middleware creates OAuth2 middleware with secure secret handling
func (smf *SecureMiddlewareFactory) CreateSecureOAuth2Middleware(config map[string]interface{}) (*SecureOAuth2Middleware, error) {
	oauth2Config := &auth.OAuth2Config{
		RefreshTokens: true,
	}

	// Securely load client secret
	clientSecret, err := smf.secretManager.GetSecretWithFallback(
		"oauth2_client_secret",
		"client_secret",
		"oauth_client_secret",
	)
	if err != nil {
		return nil, fmt.Errorf("OAuth2 client secret not found: %w", err)
	}
	oauth2Config.ClientSecret = clientSecret

	// Process other configuration securely
	if err := smf.processOAuth2Config(oauth2Config, config); err != nil {
		return nil, fmt.Errorf("failed to process OAuth2 config: %w", err)
	}

	// Validate configuration
	if err := smf.validateOAuth2Config(oauth2Config); err != nil {
		return nil, fmt.Errorf("OAuth2 configuration validation failed: %w", err)
	}

	middleware := auth.NewOAuth2Middleware(oauth2Config, nil, nil)

	// Create secure config
	secureConfig := &SecureMiddlewareConfig{
		EnableInputValidation: true,
		EnableSecurityHeaders: true,
		EnableAuditLogging:    true,
		MaxRequestSize:        1024 * 1024, // 1MB
		AllowedContentTypes:   []string{"application/json", "application/x-www-form-urlencoded"},
	}

	return NewSecureOAuth2Middleware(*middleware, secureConfig), nil
}

// CreateSecureAPIKeyMiddleware creates API key middleware with enhanced validation
func (smf *SecureMiddlewareFactory) CreateSecureAPIKeyMiddleware(config map[string]interface{}) (*SecureAPIKeyMiddleware, error) {
	apiKeyConfig := &auth.APIKeyConfig{
		HeaderName:      "X-API-Key",
		CacheTimeout:    5 * time.Minute,
		RateLimitPerKey: 1000,
	}

	// Process configuration securely
	if err := smf.processAPIKeyConfig(apiKeyConfig, config); err != nil {
		return nil, fmt.Errorf("failed to process API key config: %w", err)
	}

	middleware := auth.NewAPIKeyMiddleware(apiKeyConfig, nil, nil)

	// Create secure config
	secureConfig := &SecureMiddlewareConfig{
		EnableInputValidation: true,
		EnableSecurityHeaders: true,
		EnableAuditLogging:    true,
		MaxRequestSize:        1024 * 1024, // 1MB
		AllowedContentTypes:   []string{"application/json", "application/x-www-form-urlencoded"},
	}

	return NewSecureAPIKeyMiddleware(*middleware, secureConfig), nil
}

// processJWTConfig processes JWT configuration with security validation
func (smf *SecureMiddlewareFactory) processJWTConfig(jwtConfig *auth.JWTConfig, config map[string]interface{}) error {
	// Safely process algorithm
	if alg, ok := config["algorithm"].(string); ok {
		if err := smf.validateJWTAlgorithm(alg); err != nil {
			return fmt.Errorf("invalid JWT algorithm: %w", err)
		}
		jwtConfig.Algorithm = alg
	}

	// Safely process issuer
	if issuer, ok := config["issuer"].(string); ok {
		if err := smf.validateIssuer(issuer); err != nil {
			return fmt.Errorf("invalid issuer: %w", err)
		}
		jwtConfig.Issuer = issuer
	}

	// Safely process audience
	if audience, ok := config["audience"].([]string); ok {
		for _, aud := range audience {
			if err := smf.validateAudience(aud); err != nil {
				return fmt.Errorf("invalid audience: %w", err)
			}
		}
		jwtConfig.Audience = audience
	}

	// Process expiration with limits
	if exp, ok := config["expiration"].(string); ok {
		duration, err := time.ParseDuration(exp)
		if err != nil {
			return fmt.Errorf("invalid expiration duration: %w", err)
		}
		if err := smf.validateTokenExpiration(duration); err != nil {
			return fmt.Errorf("invalid token expiration: %w", err)
		}
		jwtConfig.Expiration = duration
	}

	return nil
}

// processOAuth2Config processes OAuth2 configuration with security validation
func (smf *SecureMiddlewareFactory) processOAuth2Config(oauth2Config *auth.OAuth2Config, config map[string]interface{}) error {
	// Validate and process client ID
	if clientID, ok := config["client_id"].(string); ok {
		if err := smf.validateClientID(clientID); err != nil {
			return fmt.Errorf("invalid client ID: %w", err)
		}
		oauth2Config.ClientID = clientID
	}

	// Validate and process URLs
	if authURL, ok := config["auth_url"].(string); ok {
		if err := smf.validateURL(authURL); err != nil {
			return fmt.Errorf("invalid auth URL: %w", err)
		}
		oauth2Config.AuthURL = authURL
	}

	if tokenURL, ok := config["token_url"].(string); ok {
		if err := smf.validateURL(tokenURL); err != nil {
			return fmt.Errorf("invalid token URL: %w", err)
		}
		oauth2Config.TokenURL = tokenURL
	}

	if redirectURL, ok := config["redirect_url"].(string); ok {
		if err := smf.validateRedirectURL(redirectURL); err != nil {
			return fmt.Errorf("invalid redirect URL: %w", err)
		}
		oauth2Config.RedirectURL = redirectURL
	}

	// Process scopes with validation
	if scopes, ok := config["scopes"].([]string); ok {
		for _, scope := range scopes {
			if err := smf.validateScope(scope); err != nil {
				return fmt.Errorf("invalid scope '%s': %w", scope, err)
			}
		}
		oauth2Config.Scopes = scopes
	}

	return nil
}

// processAPIKeyConfig processes API key configuration with security validation
func (smf *SecureMiddlewareFactory) processAPIKeyConfig(apiKeyConfig *auth.APIKeyConfig, config map[string]interface{}) error {
	// Validate header name
	if header, ok := config["header_name"].(string); ok {
		if err := smf.validateHeaderName(header); err != nil {
			return fmt.Errorf("invalid header name: %w", err)
		}
		apiKeyConfig.HeaderName = header
	}

	// Validate validation endpoint
	if endpoint, ok := config["validation_endpoint"].(string); ok {
		if err := smf.validateURL(endpoint); err != nil {
			return fmt.Errorf("invalid validation endpoint: %w", err)
		}
		apiKeyConfig.ValidationEndpoint = endpoint
	}

	// Process rate limits with bounds checking
	if rateLimit, ok := config["rate_limit_per_key"].(int); ok {
		if err := smf.validateRateLimit(rateLimit); err != nil {
			return fmt.Errorf("invalid rate limit: %w", err)
		}
		apiKeyConfig.RateLimitPerKey = rateLimit
	}

	return nil
}

// Validation methods for secure configuration

func (smf *SecureMiddlewareFactory) validateJWTAlgorithm(algorithm string) error {
	allowedAlgorithms := map[string]bool{
		"HS256": true, "HS384": true, "HS512": true,
		"RS256": true, "RS384": true, "RS512": true,
		"ES256": true, "ES384": true, "ES512": true,
	}

	if !allowedAlgorithms[algorithm] {
		return fmt.Errorf("algorithm '%s' not allowed", algorithm)
	}

	return nil
}

func (smf *SecureMiddlewareFactory) validateIssuer(issuer string) error {
	if len(issuer) == 0 {
		return fmt.Errorf("issuer cannot be empty")
	}

	if len(issuer) > 255 {
		return fmt.Errorf("issuer too long")
	}

	return smf.validator.validateString(issuer)
}

func (smf *SecureMiddlewareFactory) validateAudience(audience string) error {
	if len(audience) == 0 {
		return fmt.Errorf("audience cannot be empty")
	}

	if len(audience) > 255 {
		return fmt.Errorf("audience too long")
	}

	return smf.validator.validateString(audience)
}

func (smf *SecureMiddlewareFactory) validateTokenExpiration(duration time.Duration) error {
	minExp := 5 * time.Minute
	maxExp := 30 * 24 * time.Hour // 30 days

	if duration < minExp {
		return fmt.Errorf("token expiration too short (minimum %v)", minExp)
	}

	if duration > maxExp {
		return fmt.Errorf("token expiration too long (maximum %v)", maxExp)
	}

	return nil
}

func (smf *SecureMiddlewareFactory) validateClientID(clientID string) error {
	if len(clientID) == 0 {
		return fmt.Errorf("client ID cannot be empty")
	}

	if len(clientID) > 128 {
		return fmt.Errorf("client ID too long")
	}

	return smf.validator.validateString(clientID)
}

func (smf *SecureMiddlewareFactory) validateURL(urlStr string) error {
	if len(urlStr) == 0 {
		return fmt.Errorf("URL cannot be empty")
	}

	if len(urlStr) > 2048 {
		return fmt.Errorf("URL too long")
	}

	// Basic URL validation would go here
	// For now, just validate the string content
	return smf.validator.validateString(urlStr)
}

func (smf *SecureMiddlewareFactory) validateRedirectURL(redirectURL string) error {
	if err := smf.validateURL(redirectURL); err != nil {
		return err
	}

	// Additional redirect URL security checks could go here
	// e.g., checking against allowed domains

	return nil
}

func (smf *SecureMiddlewareFactory) validateScope(scope string) error {
	if len(scope) == 0 {
		return fmt.Errorf("scope cannot be empty")
	}

	if len(scope) > 64 {
		return fmt.Errorf("scope too long")
	}

	return smf.validator.validateString(scope)
}

func (smf *SecureMiddlewareFactory) validateHeaderName(headerName string) error {
	if len(headerName) == 0 {
		return fmt.Errorf("header name cannot be empty")
	}

	// HTTP header names should only contain token characters
	for _, char := range headerName {
		if !isTokenChar(char) {
			return fmt.Errorf("invalid character in header name: %c", char)
		}
	}

	return nil
}

func (smf *SecureMiddlewareFactory) validateRateLimit(rateLimit int) error {
	if rateLimit <= 0 {
		return fmt.Errorf("rate limit must be positive")
	}

	if rateLimit > 10000 {
		return fmt.Errorf("rate limit too high (maximum 10000)")
	}

	return nil
}

func (smf *SecureMiddlewareFactory) validateOAuth2Config(config *auth.OAuth2Config) error {
	if config.ClientID == "" {
		return fmt.Errorf("client ID is required")
	}

	if config.ClientSecret == "" {
		return fmt.Errorf("client secret is required")
	}

	if config.AuthURL == "" {
		return fmt.Errorf("auth URL is required")
	}

	if config.TokenURL == "" {
		return fmt.Errorf("token URL is required")
	}

	return nil
}

func (smf *SecureMiddlewareFactory) isAllowedToGenerateSecrets() bool {
	// Only allow secret generation in development environments
	env := smf.secretManager.GetEnv("AGUI_ENV")
	return env == "development" || env == "test"
}

// GetSecretManager returns the secret manager for advanced usage
func (smf *SecureMiddlewareFactory) GetSecretManager() *SecretManager {
	return smf.secretManager
}

// GetConfigManager returns the config manager for advanced usage
func (smf *SecureMiddlewareFactory) GetConfigManager() *SecureConfigManager {
	return smf.configManager
}

// GetValidator returns the input validator for advanced usage
func (smf *SecureMiddlewareFactory) GetValidator() *EnhancedInputValidator {
	return smf.validator
}
