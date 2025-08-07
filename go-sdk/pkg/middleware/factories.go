package middleware

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/auth"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/observability"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/ratelimit"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/security"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/transform"
)

// Middleware factory implementations

// JWTMiddlewareFactory creates JWT authentication middleware with enhanced security
type JWTMiddlewareFactory struct {
	secureFactory *security.SecureMiddlewareFactory
}

func NewJWTMiddlewareFactory() (*JWTMiddlewareFactory, error) {
	secureFactory, err := security.NewSecureMiddlewareFactory()
	if err != nil {
		// Fallback to non-secure mode for backward compatibility
		return &JWTMiddlewareFactory{}, nil
	}

	return &JWTMiddlewareFactory{secureFactory: secureFactory}, nil
}

func (f *JWTMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	// Use secure factory if available and environment allows
	if f.secureFactory != nil && f.shouldUseSecureMode() {
		secureMiddleware, err := f.secureFactory.CreateSecureJWTMiddleware(config.Config)
		if err == nil {
			return NewSecureMiddlewareAdapter(secureMiddleware), nil
		}

		// Log the fallback for debugging
		if os.Getenv("AGUI_DEBUG") == "true" {
			fmt.Printf("SECURITY WARNING: Falling back to legacy JWT middleware: %v\n", err)
		}
	}

	// Legacy implementation for backward compatibility
	return f.createLegacyJWTMiddleware(config)
}

// createLegacyJWTMiddleware creates JWT middleware using the original method
func (f *JWTMiddlewareFactory) createLegacyJWTMiddleware(config *MiddlewareConfig) (Middleware, error) {
	jwtConfig := &auth.JWTConfig{
		Algorithm:  "HS256",
		Expiration: 24 * time.Hour,
	}

	// SECURITY IMPROVEMENT: Try to load secret from environment first
	secret := os.Getenv("AGUI_JWT_SECRET")
	if secret == "" {
		// Fallback to configuration (with warning)
		if configSecret, ok := config.Config["secret"].(string); ok {
			secret = configSecret
			if os.Getenv("AGUI_ENV") == "production" {
				return nil, fmt.Errorf("hard-coded JWT secret not allowed in production, use AGUI_JWT_SECRET environment variable")
			}
			if os.Getenv("AGUI_ENV") != "test" {
				fmt.Printf("SECURITY WARNING: Using hard-coded JWT secret. Set AGUI_JWT_SECRET environment variable.\n")
			}
		} else {
			return nil, fmt.Errorf("JWT secret not provided via AGUI_JWT_SECRET environment variable or config")
		}
	}
	jwtConfig.Secret = secret

	if alg, ok := config.Config["algorithm"].(string); ok {
		jwtConfig.Algorithm = alg
	}

	if issuer, ok := config.Config["issuer"].(string); ok {
		jwtConfig.Issuer = issuer
	}

	middleware, err := auth.NewJWTMiddleware(jwtConfig, nil, nil)
	if err != nil {
		return nil, err
	}

	err = middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}

	return NewAuthMiddlewareAdapter(middleware), nil
}

// shouldUseSecureMode determines if secure mode should be used
func (f *JWTMiddlewareFactory) shouldUseSecureMode() bool {
	// Use secure mode by default, unless explicitly disabled
	if os.Getenv("AGUI_DISABLE_SECURE_MODE") == "true" {
		return false
	}

	// Always use secure mode in production
	if os.Getenv("AGUI_ENV") == "production" {
		return true
	}

	// Use secure mode if JWT secret is available via environment
	return os.Getenv("AGUI_JWT_SECRET") != ""
}

func (f *JWTMiddlewareFactory) SupportedTypes() []string {
	return []string{"jwt_auth"}
}

// APIKeyMiddlewareFactory creates API key authentication middleware
type APIKeyMiddlewareFactory struct{}

func (f *APIKeyMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	apiKeyConfig := &auth.APIKeyConfig{
		HeaderName:   "X-API-Key",
		CacheTimeout: 5 * time.Minute,
	}

	if header, ok := config.Config["header_name"].(string); ok {
		apiKeyConfig.HeaderName = header
	}

	if endpoint, ok := config.Config["validation_endpoint"].(string); ok {
		apiKeyConfig.ValidationEndpoint = endpoint
	}

	middleware := auth.NewAPIKeyMiddleware(apiKeyConfig, nil, nil)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewAuthMiddlewareAdapter(middleware), nil
}

func (f *APIKeyMiddlewareFactory) SupportedTypes() []string {
	return []string{"api_key_auth"}
}

// BasicAuthMiddlewareFactory creates basic authentication middleware
type BasicAuthMiddlewareFactory struct{}

func (f *BasicAuthMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	basicConfig := &auth.BasicAuthConfig{
		Realm:         "Restricted Area",
		HashAlgorithm: "bcrypt",
	}

	if realm, ok := config.Config["realm"].(string); ok {
		basicConfig.Realm = realm
	}

	if hash, ok := config.Config["hash_algorithm"].(string); ok {
		basicConfig.HashAlgorithm = hash
	}

	middleware := auth.NewBasicAuthMiddleware(basicConfig, nil, nil, nil)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewAuthMiddlewareAdapter(middleware), nil
}

func (f *BasicAuthMiddlewareFactory) SupportedTypes() []string {
	return []string{"basic_auth"}
}

// OAuth2MiddlewareFactory creates OAuth2 authentication middleware with enhanced security
type OAuth2MiddlewareFactory struct {
	secureFactory *security.SecureMiddlewareFactory
}

func NewOAuth2MiddlewareFactory() (*OAuth2MiddlewareFactory, error) {
	secureFactory, err := security.NewSecureMiddlewareFactory()
	if err != nil {
		// Fallback to non-secure mode for backward compatibility
		return &OAuth2MiddlewareFactory{}, nil
	}

	return &OAuth2MiddlewareFactory{secureFactory: secureFactory}, nil
}

func (f *OAuth2MiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	// Use secure factory if available and environment allows
	if f.secureFactory != nil && f.shouldUseSecureMode() {
		secureMiddleware, err := f.secureFactory.CreateSecureOAuth2Middleware(config.Config)
		if err == nil {
			return NewSecureMiddlewareAdapter(secureMiddleware), nil
		}

		// Log the fallback for debugging
		if os.Getenv("AGUI_DEBUG") == "true" {
			fmt.Printf("SECURITY WARNING: Falling back to legacy OAuth2 middleware: %v\n", err)
		}
	}

	// Legacy implementation for backward compatibility
	return f.createLegacyOAuth2Middleware(config)
}

// createLegacyOAuth2Middleware creates OAuth2 middleware using the original method
func (f *OAuth2MiddlewareFactory) createLegacyOAuth2Middleware(config *MiddlewareConfig) (Middleware, error) {
	oauth2Config := &auth.OAuth2Config{}

	if clientID, ok := config.Config["client_id"].(string); ok {
		oauth2Config.ClientID = clientID
	}

	// SECURITY IMPROVEMENT: Try to load client secret from environment first
	clientSecret := os.Getenv("AGUI_OAUTH2_CLIENT_SECRET")
	if clientSecret == "" {
		// Fallback to configuration (with warning)
		if configSecret, ok := config.Config["client_secret"].(string); ok {
			clientSecret = configSecret
			if os.Getenv("AGUI_ENV") == "production" {
				return nil, fmt.Errorf("hard-coded OAuth2 client secret not allowed in production, use AGUI_OAUTH2_CLIENT_SECRET environment variable")
			}
			if os.Getenv("AGUI_ENV") != "test" {
				fmt.Printf("SECURITY WARNING: Using hard-coded OAuth2 client secret. Set AGUI_OAUTH2_CLIENT_SECRET environment variable.\n")
			}
		} else {
			return nil, fmt.Errorf("OAuth2 client secret not provided via AGUI_OAUTH2_CLIENT_SECRET environment variable or config")
		}
	}
	oauth2Config.ClientSecret = clientSecret

	if authURL, ok := config.Config["auth_url"].(string); ok {
		oauth2Config.AuthURL = authURL
	}

	if tokenURL, ok := config.Config["token_url"].(string); ok {
		oauth2Config.TokenURL = tokenURL
	}

	if redirectURL, ok := config.Config["redirect_url"].(string); ok {
		oauth2Config.RedirectURL = redirectURL
	}

	middleware := auth.NewOAuth2Middleware(oauth2Config, nil, nil)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewAuthMiddlewareAdapter(middleware), nil
}

// shouldUseSecureMode determines if secure mode should be used for OAuth2
func (f *OAuth2MiddlewareFactory) shouldUseSecureMode() bool {
	// Use secure mode by default, unless explicitly disabled
	if os.Getenv("AGUI_DISABLE_SECURE_MODE") == "true" {
		return false
	}

	// Always use secure mode in production
	if os.Getenv("AGUI_ENV") == "production" {
		return true
	}

	// Use secure mode if client secret is available via environment
	return os.Getenv("AGUI_OAUTH2_CLIENT_SECRET") != ""
}

func (f *OAuth2MiddlewareFactory) SupportedTypes() []string {
	return []string{"oauth2_auth"}
}

// LoggingMiddlewareFactory creates logging middleware
type LoggingMiddlewareFactory struct{}

func (f *LoggingMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	loggingConfig := &observability.LoggingConfig{
		Level:             observability.LogLevelInfo,
		Format:            observability.LogFormatJSON,
		EnableCorrelation: true,
	}

	if level, ok := config.Config["level"].(string); ok {
		switch strings.ToUpper(level) {
		case "DEBUG":
			loggingConfig.Level = observability.LogLevelDebug
		case "INFO":
			loggingConfig.Level = observability.LogLevelInfo
		case "WARN":
			loggingConfig.Level = observability.LogLevelWarn
		case "ERROR":
			loggingConfig.Level = observability.LogLevelError
		}
	}

	middleware := observability.NewLoggingMiddleware(loggingConfig)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewObservabilityMiddlewareAdapter(middleware), nil
}

func (f *LoggingMiddlewareFactory) SupportedTypes() []string {
	return []string{"logging"}
}

// MetricsMiddlewareFactory creates metrics middleware
type MetricsMiddlewareFactory struct{}

func (f *MetricsMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	metricsConfig := &observability.MetricsConfig{
		EnableRequestCount:    true,
		EnableRequestDuration: true,
		EnableActiveRequests:  true,
	}

	middleware := observability.NewMetricsMiddleware(metricsConfig, nil)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewObservabilityMiddlewareAdapter(middleware), nil
}

func (f *MetricsMiddlewareFactory) SupportedTypes() []string {
	return []string{"metrics"}
}

// CorrelationIDMiddlewareFactory creates correlation ID middleware
type CorrelationIDMiddlewareFactory struct{}

func (f *CorrelationIDMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	headerName := "X-Correlation-ID"
	if header, ok := config.Config["header_name"].(string); ok {
		headerName = header
	}

	middleware := observability.NewCorrelationIDMiddleware(headerName)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewObservabilityMiddlewareAdapter(middleware), nil
}

func (f *CorrelationIDMiddlewareFactory) SupportedTypes() []string {
	return []string{"correlation_id"}
}

// RateLimitMiddlewareFactory creates rate limiting middleware
type RateLimitMiddlewareFactory struct{}

func (f *RateLimitMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	rateLimitConfig := &ratelimit.RateLimitConfig{
		Algorithm:       ratelimit.AlgorithmTokenBucket,
		RequestsPerUnit: 100,
		Unit:            time.Minute,
		Burst:           10,
		KeyGenerator:    "ip",
	}

	if alg, ok := config.Config["algorithm"].(string); ok {
		rateLimitConfig.Algorithm = ratelimit.RateLimitAlgorithm(alg)
	}

	if requests, ok := config.Config["requests_per_unit"].(int); ok {
		rateLimitConfig.RequestsPerUnit = int64(requests)
	}

	if unit, ok := config.Config["unit"].(string); ok {
		if duration, err := time.ParseDuration(unit); err == nil {
			rateLimitConfig.Unit = duration
		}
	}

	middleware, err := ratelimit.NewRateLimitMiddleware(rateLimitConfig)
	if err != nil {
		return nil, err
	}

	err = middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewRateLimitMiddlewareAdapter(middleware), nil
}

func (f *RateLimitMiddlewareFactory) SupportedTypes() []string {
	return []string{"rate_limit"}
}

// DistributedRateLimitMiddlewareFactory creates distributed rate limiting middleware
type DistributedRateLimitMiddlewareFactory struct{}

func (f *DistributedRateLimitMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	rateLimitConfig := &ratelimit.RateLimitConfig{
		Algorithm:       ratelimit.AlgorithmTokenBucket,
		RequestsPerUnit: 100,
		Unit:            time.Minute,
		Burst:           10,
		KeyGenerator:    "ip",
		Distributed:     true,
	}

	if redisURL, ok := config.Config["redis_url"].(string); ok {
		rateLimitConfig.RedisURL = redisURL
	}

	// Use mock Redis client for now
	redisClient := ratelimit.NewMockRedisClient()

	middleware, err := ratelimit.NewDistributedRateLimitMiddleware(rateLimitConfig, redisClient)
	if err != nil {
		return nil, err
	}

	err = middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewRateLimitMiddlewareAdapter(middleware), nil
}

func (f *DistributedRateLimitMiddlewareFactory) SupportedTypes() []string {
	return []string{"distributed_rate_limit"}
}

// TransformationMiddlewareFactory creates transformation middleware
type TransformationMiddlewareFactory struct{}

func (f *TransformationMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	transformConfig := &transform.TransformationConfig{
		DefaultPipeline: "default",
		Pipelines: []transform.PipelineConfig{
			{
				Name:    "default",
				Enabled: true,
				Transformers: []transform.TransformerConfig{
					{
						Type:    "sanitization",
						Name:    "default_sanitization",
						Enabled: true,
						Config: map[string]interface{}{
							"sensitive_fields": []string{"password", "token", "secret"},
							"replacement":      "[REDACTED]",
						},
					},
				},
			},
		},
	}

	middleware, err := transform.NewTransformationMiddleware(transformConfig)
	if err != nil {
		return nil, err
	}

	err = middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewTransformMiddlewareAdapter(middleware), nil
}

func (f *TransformationMiddlewareFactory) SupportedTypes() []string {
	return []string{"transformation"}
}

// SecurityMiddlewareFactory creates security middleware
type SecurityMiddlewareFactory struct{}

func (f *SecurityMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	securityConfig := &security.SecurityConfig{
		CORS: &security.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		},
		Headers: &security.SecurityHeadersConfig{
			Enabled:             true,
			XFrameOptions:       "DENY",
			XContentTypeOptions: "nosniff",
			XXSSProtection:      "1; mode=block",
		},
		ThreatDetection: &security.ThreatDetectionConfig{
			Enabled:      true,
			SQLInjection: true,
			XSSDetection: true,
			LogThreats:   true,
		},
	}

	middleware, err := security.NewSecurityMiddleware(securityConfig)
	if err != nil {
		return nil, err
	}

	err = middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewSecurityMiddlewareAdapter(middleware), nil
}

func (f *SecurityMiddlewareFactory) SupportedTypes() []string {
	return []string{"security"}
}
