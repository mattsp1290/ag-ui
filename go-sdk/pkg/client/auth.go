package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"go.uber.org/zap"
)

// AuthMethod represents the type of authentication method
type AuthMethod string

const (
	// AuthMethodJWT uses JSON Web Tokens for authentication
	AuthMethodJWT AuthMethod = "jwt"
	
	// AuthMethodAPIKey uses API key-based authentication
	AuthMethodAPIKey AuthMethod = "api_key"
	
	// AuthMethodBasic uses HTTP Basic authentication
	AuthMethodBasic AuthMethod = "basic"
	
	// AuthMethodBearer uses Bearer token authentication
	AuthMethodBearer AuthMethod = "bearer"
	
	// AuthMethodMTLS uses mutual TLS authentication
	AuthMethodMTLS AuthMethod = "mtls"
	
	// AuthMethodHMAC uses HMAC signature-based authentication
	AuthMethodHMAC AuthMethod = "hmac"
)

// TokenType represents different types of tokens
type TokenType string

const (
	// TokenTypeAccess is used for accessing resources
	TokenTypeAccess TokenType = "access"
	
	// TokenTypeRefresh is used for refreshing access tokens
	TokenTypeRefresh TokenType = "refresh"
	
	// TokenTypeID contains user identity information
	TokenTypeID TokenType = "id"
)

// SecurityConfig contains comprehensive security configuration
type SecurityConfig struct {
	// Authentication settings
	AuthMethod              AuthMethod           `json:"auth_method" yaml:"auth_method"`
	EnableMultiAuth         bool                 `json:"enable_multi_auth" yaml:"enable_multi_auth"`
	SupportedMethods        []AuthMethod         `json:"supported_methods" yaml:"supported_methods"`
	
	// JWT settings
	JWT                     JWTConfig            `json:"jwt" yaml:"jwt"`
	
	// API Key settings
	APIKey                  APIKeyConfig         `json:"api_key" yaml:"api_key"`
	
	// Basic Auth settings
	BasicAuth               BasicAuthConfig      `json:"basic_auth" yaml:"basic_auth"`
	
	// TLS/SSL settings
	TLS                     TLSConfig            `json:"tls" yaml:"tls"`
	
	// HMAC settings
	HMAC                    HMACConfig           `json:"hmac" yaml:"hmac"`
	
	// Security headers
	SecurityHeaders         SecurityHeadersConfig `json:"security_headers" yaml:"security_headers"`
	
	// Token storage
	TokenStorage            TokenStorageConfig   `json:"token_storage" yaml:"token_storage"`
	
	// Audit logging
	AuditLogging            AuditLoggingConfig   `json:"audit_logging" yaml:"audit_logging"`
	
	// Rate limiting and protection
	RateLimit               RateLimitConfig      `json:"rate_limit" yaml:"rate_limit"`
	
	// Session management
	SessionConfig           SessionConfig        `json:"session" yaml:"session"`
}

// JWTConfig contains JWT-specific configuration
type JWTConfig struct {
	// Signing configuration
	SigningMethod           string               `json:"signing_method" yaml:"signing_method"`
	SecretKey               string               `json:"secret_key,omitempty" yaml:"secret_key,omitempty"`
	PrivateKeyPath          string               `json:"private_key_path,omitempty" yaml:"private_key_path,omitempty"`
	PublicKeyPath           string               `json:"public_key_path,omitempty" yaml:"public_key_path,omitempty"`
	
	// Token lifecycle
	AccessTokenTTL          time.Duration        `json:"access_token_ttl" yaml:"access_token_ttl"`
	RefreshTokenTTL         time.Duration        `json:"refresh_token_ttl" yaml:"refresh_token_ttl"`
	AutoRefresh             bool                 `json:"auto_refresh" yaml:"auto_refresh"`
	RefreshThreshold        time.Duration        `json:"refresh_threshold" yaml:"refresh_threshold"`
	
	// Validation settings
	Issuer                  string               `json:"issuer" yaml:"issuer"`
	Audience                []string             `json:"audience" yaml:"audience"`
	Subject                 string               `json:"subject" yaml:"subject"`
	LeewayTime              time.Duration        `json:"leeway_time" yaml:"leeway_time"`
	
	// Claims configuration
	CustomClaims            map[string]interface{} `json:"custom_claims,omitempty" yaml:"custom_claims,omitempty"`
}

// APIKeyConfig contains API key authentication configuration
type APIKeyConfig struct {
	HeaderName              string               `json:"header_name" yaml:"header_name"`
	QueryParam              string               `json:"query_param" yaml:"query_param"`
	Prefix                  string               `json:"prefix" yaml:"prefix"`
	KeysFile                string               `json:"keys_file" yaml:"keys_file"`
	HashingAlgorithm        string               `json:"hashing_algorithm" yaml:"hashing_algorithm"`
	EnableKeyRotation       bool                 `json:"enable_key_rotation" yaml:"enable_key_rotation"`
	KeyRotationInterval     time.Duration        `json:"key_rotation_interval" yaml:"key_rotation_interval"`
}

// BasicAuthConfig contains Basic authentication configuration
type BasicAuthConfig struct {
	Realm                   string               `json:"realm" yaml:"realm"`
	UsersFile               string               `json:"users_file" yaml:"users_file"`
	HashingAlgorithm        string               `json:"hashing_algorithm" yaml:"hashing_algorithm"`
	EnablePasswordPolicy    bool                 `json:"enable_password_policy" yaml:"enable_password_policy"`
	PasswordPolicy          PasswordPolicy       `json:"password_policy" yaml:"password_policy"`
}

// TLSConfig contains TLS/SSL configuration
type TLSConfig struct {
	Enabled                 bool                 `json:"enabled" yaml:"enabled"`
	CertFile                string               `json:"cert_file" yaml:"cert_file"`
	KeyFile                 string               `json:"key_file" yaml:"key_file"`
	CAFile                  string               `json:"ca_file" yaml:"ca_file"`
	ClientAuth              tls.ClientAuthType   `json:"client_auth" yaml:"client_auth"`
	MinVersion              uint16               `json:"min_version" yaml:"min_version"`
	MaxVersion              uint16               `json:"max_version" yaml:"max_version"`
	CipherSuites            []uint16             `json:"cipher_suites" yaml:"cipher_suites"`
	CurvePreferences        []tls.CurveID        `json:"curve_preferences" yaml:"curve_preferences"`
	EnableSNI               bool                 `json:"enable_sni" yaml:"enable_sni"`
	InsecureSkipVerify      bool                 `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	CertificateValidation   CertValidationConfig `json:"certificate_validation" yaml:"certificate_validation"`
}

// HMACConfig contains HMAC signature configuration
type HMACConfig struct {
	SecretKey               string               `json:"secret_key" yaml:"secret_key"`
	Algorithm               string               `json:"algorithm" yaml:"algorithm"`
	HeaderName              string               `json:"header_name" yaml:"header_name"`
	TimestampHeader         string               `json:"timestamp_header" yaml:"timestamp_header"`
	NonceHeader             string               `json:"nonce_header" yaml:"nonce_header"`
	MaxClockSkew            time.Duration        `json:"max_clock_skew" yaml:"max_clock_skew"`
	IncludeHeaders          []string             `json:"include_headers" yaml:"include_headers"`
}

// SecurityHeadersConfig contains security headers configuration
type SecurityHeadersConfig struct {
	EnableCSP               bool                 `json:"enable_csp" yaml:"enable_csp"`
	CSPPolicy               string               `json:"csp_policy" yaml:"csp_policy"`
	EnableHSTS              bool                 `json:"enable_hsts" yaml:"enable_hsts"`
	HSTSMaxAge              int                  `json:"hsts_max_age" yaml:"hsts_max_age"`
	EnableXFrameOptions     bool                 `json:"enable_x_frame_options" yaml:"enable_x_frame_options"`
	XFrameOptions           string               `json:"x_frame_options" yaml:"x_frame_options"`
	EnableXContentType      bool                 `json:"enable_x_content_type" yaml:"enable_x_content_type"`
	EnableReferrerPolicy    bool                 `json:"enable_referrer_policy" yaml:"enable_referrer_policy"`
	ReferrerPolicy          string               `json:"referrer_policy" yaml:"referrer_policy"`
	CustomHeaders           map[string]string    `json:"custom_headers" yaml:"custom_headers"`
	CORSConfig              CORSConfig           `json:"cors" yaml:"cors"`
}

// CORSConfig contains CORS configuration
type CORSConfig struct {
	Enabled                 bool                 `json:"enabled" yaml:"enabled"`
	AllowedOrigins          []string             `json:"allowed_origins" yaml:"allowed_origins"`
	AllowedMethods          []string             `json:"allowed_methods" yaml:"allowed_methods"`
	AllowedHeaders          []string             `json:"allowed_headers" yaml:"allowed_headers"`
	ExposedHeaders          []string             `json:"exposed_headers" yaml:"exposed_headers"`
	AllowCredentials        bool                 `json:"allow_credentials" yaml:"allow_credentials"`
	MaxAge                  int                  `json:"max_age" yaml:"max_age"`
}

// TokenStorageConfig contains token storage configuration
type TokenStorageConfig struct {
	StorageType             string               `json:"storage_type" yaml:"storage_type"`
	Encryption              EncryptionConfig     `json:"encryption" yaml:"encryption"`
	FilePath                string               `json:"file_path,omitempty" yaml:"file_path,omitempty"`
	RedisConfig             RedisConfig          `json:"redis,omitempty" yaml:"redis,omitempty"`
	DatabaseConfig          DatabaseConfig       `json:"database,omitempty" yaml:"database,omitempty"`
}

// AuditLoggingConfig contains audit logging configuration
type AuditLoggingConfig struct {
	Enabled                 bool                 `json:"enabled" yaml:"enabled"`
	LogLevel                string               `json:"log_level" yaml:"log_level"`
	LogFile                 string               `json:"log_file" yaml:"log_file"`
	LogFormat               string               `json:"log_format" yaml:"log_format"`
	RotateSize              int64                `json:"rotate_size" yaml:"rotate_size"`
	RotateCount             int                  `json:"rotate_count" yaml:"rotate_count"`
	LogSensitiveData        bool                 `json:"log_sensitive_data" yaml:"log_sensitive_data"`
	EventTypes              []string             `json:"event_types" yaml:"event_types"`
	IncludeRequestBody      bool                 `json:"include_request_body" yaml:"include_request_body"`
	IncludeResponseBody     bool                 `json:"include_response_body" yaml:"include_response_body"`
}

// Supporting configuration types
type PasswordPolicy struct {
	MinLength               int                  `json:"min_length" yaml:"min_length"`
	RequireUppercase        bool                 `json:"require_uppercase" yaml:"require_uppercase"`
	RequireLowercase        bool                 `json:"require_lowercase" yaml:"require_lowercase"`
	RequireNumbers          bool                 `json:"require_numbers" yaml:"require_numbers"`
	RequireSpecialChars     bool                 `json:"require_special_chars" yaml:"require_special_chars"`
	MaxAge                  time.Duration        `json:"max_age" yaml:"max_age"`
}

type CertValidationConfig struct {
	ValidateCertChain       bool                 `json:"validate_cert_chain" yaml:"validate_cert_chain"`
	ValidateHostname        bool                 `json:"validate_hostname" yaml:"validate_hostname"`
	CustomCAPool            bool                 `json:"custom_ca_pool" yaml:"custom_ca_pool"`
	CRLCheckEnabled         bool                 `json:"crl_check_enabled" yaml:"crl_check_enabled"`
	OCSPCheckEnabled        bool                 `json:"ocsp_check_enabled" yaml:"ocsp_check_enabled"`
}

type EncryptionConfig struct {
	Enabled                 bool                 `json:"enabled" yaml:"enabled"`
	Algorithm               string               `json:"algorithm" yaml:"algorithm"`
	KeyFile                 string               `json:"key_file" yaml:"key_file"`
	KeyRotationInterval     time.Duration        `json:"key_rotation_interval" yaml:"key_rotation_interval"`
}

type RedisConfig struct {
	Address                 string               `json:"address" yaml:"address"`
	Password                string               `json:"password" yaml:"password"`
	Database                int                  `json:"database" yaml:"database"`
	PoolSize                int                  `json:"pool_size" yaml:"pool_size"`
}

type DatabaseConfig struct {
	Driver                  string               `json:"driver" yaml:"driver"`
	ConnectionString        string               `json:"connection_string" yaml:"connection_string"`
	TableName               string               `json:"table_name" yaml:"table_name"`
}

// Note: RateLimitConfig is defined in resilience.go with more comprehensive fields

type SessionConfig struct {
	Enabled                 bool                 `json:"enabled" yaml:"enabled"`
	CookieName              string               `json:"cookie_name" yaml:"cookie_name"`
	CookieSecure            bool                 `json:"cookie_secure" yaml:"cookie_secure"`
	CookieHTTPOnly          bool                 `json:"cookie_http_only" yaml:"cookie_http_only"`
	CookieSameSite          string               `json:"cookie_same_site" yaml:"cookie_same_site"`
	SessionTimeout          time.Duration        `json:"session_timeout" yaml:"session_timeout"`
}

// SecurityManager manages authentication and security features
type SecurityManager struct {
	config                  *SecurityConfig
	logger                  *zap.Logger
	
	// Authentication providers
	jwtManager              *JWTManager
	apiKeyManager           *APIKeyManager
	basicAuthManager        *BasicAuthManager
	hmacManager             *HMACManager
	
	// TLS management
	tlsConfig               *tls.Config
	certManager             *CertificateManager
	
	// Token storage
	tokenStorage            TokenStorage
	
	// Security components
	headerManager           *SecurityHeaderManager
	auditLogger             *AuditLogger
	
	// Synchronization
	mu                      sync.RWMutex
	
	// Runtime state
	activeTokens            map[string]*TokenInfo
	activeSessions          map[string]*SessionInfo
	rateLimiters            map[string]*RateLimiter
}

// TokenInfo contains information about an active token
type TokenInfo struct {
	Token                   string               `json:"token"`
	TokenType               TokenType            `json:"token_type"`
	ExpiresAt               time.Time            `json:"expires_at"`
	IssuedAt                time.Time            `json:"issued_at"`
	Subject                 string               `json:"subject"`
	Scopes                  []string             `json:"scopes"`
	RefreshToken            string               `json:"refresh_token,omitempty"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
}

// SessionInfo contains information about an active session
type SessionInfo struct {
	SessionID               string               `json:"session_id"`
	UserID                  string               `json:"user_id"`
	CreatedAt               time.Time            `json:"created_at"`
	LastAccessedAt          time.Time            `json:"last_accessed_at"`
	ExpiresAt               time.Time            `json:"expires_at"`
	IPAddress               string               `json:"ip_address"`
	UserAgent               string               `json:"user_agent"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
}

// AuthenticationResult contains the result of an authentication attempt
type AuthenticationResult struct {
	Success                 bool                 `json:"success"`
	User                    *UserInfo            `json:"user,omitempty"`
	Token                   *TokenInfo           `json:"token,omitempty"`
	Session                 *SessionInfo         `json:"session,omitempty"`
	Error                   error                `json:"error,omitempty"`
	RequiredScopes          []string             `json:"required_scopes,omitempty"`
	GrantedScopes           []string             `json:"granted_scopes,omitempty"`
}

// UserInfo contains information about an authenticated user
type UserInfo struct {
	ID                      string               `json:"id"`
	Username                string               `json:"username"`
	Email                   string               `json:"email"`
	Roles                   []string             `json:"roles"`
	Permissions             []string             `json:"permissions"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
}

// SecurityEvent represents a security-related event for audit logging
type SecurityEvent struct {
	Type                    string               `json:"type"`
	Timestamp               time.Time            `json:"timestamp"`
	UserID                  string               `json:"user_id,omitempty"`
	SessionID               string               `json:"session_id,omitempty"`
	IPAddress               string               `json:"ip_address,omitempty"`
	UserAgent               string               `json:"user_agent,omitempty"`
	Resource                string               `json:"resource,omitempty"`
	Action                  string               `json:"action,omitempty"`
	Result                  string               `json:"result"`
	Error                   string               `json:"error,omitempty"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
}

// NewSecurityManager creates a new security manager with the given configuration
func NewSecurityManager(config *SecurityConfig, logger *zap.Logger) (*SecurityManager, error) {
	if config == nil {
		return nil, errors.NewValidationError("invalid_configuration", "security config cannot be nil")
	}
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	sm := &SecurityManager{
		config:         config,
		logger:         logger,
		activeTokens:   make(map[string]*TokenInfo),
		activeSessions: make(map[string]*SessionInfo),
		rateLimiters:   make(map[string]*RateLimiter),
	}
	
	// Initialize authentication providers
	if err := sm.initializeAuthProviders(); err != nil {
		return nil, fmt.Errorf("failed to initialize auth providers: %w", err)
	}
	
	// Initialize TLS configuration
	if err := sm.initializeTLS(); err != nil {
		return nil, fmt.Errorf("failed to initialize TLS: %w", err)
	}
	
	// Initialize token storage
	if err := sm.initializeTokenStorage(); err != nil {
		return nil, fmt.Errorf("failed to initialize token storage: %w", err)
	}
	
	// Initialize security components
	if err := sm.initializeSecurityComponents(); err != nil {
		return nil, fmt.Errorf("failed to initialize security components: %w", err)
	}
	
	return sm, nil
}

// Initialize authentication providers
func (sm *SecurityManager) initializeAuthProviders() error {
	// Initialize JWT manager
	if sm.isMethodSupported(AuthMethodJWT) {
		var err error
		sm.jwtManager, err = NewJWTManager(&sm.config.JWT, sm.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize JWT manager: %w", err)
		}
	}
	
	// Initialize API key manager
	if sm.isMethodSupported(AuthMethodAPIKey) {
		var err error
		sm.apiKeyManager, err = NewAPIKeyManager(&sm.config.APIKey, sm.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize API key manager: %w", err)
		}
	}
	
	// Initialize Basic Auth manager
	if sm.isMethodSupported(AuthMethodBasic) {
		var err error
		sm.basicAuthManager, err = NewBasicAuthManager(&sm.config.BasicAuth, sm.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize Basic Auth manager: %w", err)
		}
	}
	
	// Initialize HMAC manager
	if sm.isMethodSupported(AuthMethodHMAC) {
		var err error
		sm.hmacManager, err = NewHMACManager(&sm.config.HMAC, sm.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize HMAC manager: %w", err)
		}
	}
	
	return nil
}

// Initialize TLS configuration
func (sm *SecurityManager) initializeTLS() error {
	if !sm.config.TLS.Enabled {
		return nil
	}
	
	var err error
	sm.certManager, err = NewCertificateManager(&sm.config.TLS, sm.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize certificate manager: %w", err)
	}
	
	sm.tlsConfig, err = sm.certManager.GetTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to get TLS config: %w", err)
	}
	
	return nil
}

// Initialize token storage
func (sm *SecurityManager) initializeTokenStorage() error {
	var err error
	sm.tokenStorage, err = NewTokenStorage(&sm.config.TokenStorage, sm.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize token storage: %w", err)
	}
	
	return nil
}

// Initialize security components
func (sm *SecurityManager) initializeSecurityComponents() error {
	// Initialize security header manager
	var err error
	sm.headerManager, err = NewSecurityHeaderManager(&sm.config.SecurityHeaders, sm.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize security header manager: %w", err)
	}
	
	// Initialize audit logger
	sm.auditLogger, err = NewAuditLogger(&sm.config.AuditLogging, sm.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize audit logger: %w", err)
	}
	
	return nil
}

// Authenticate performs authentication based on the configured method
func (sm *SecurityManager) Authenticate(ctx context.Context, req *http.Request) (*AuthenticationResult, error) {
	startTime := time.Now()
	
	defer func() {
		// Log authentication attempt
		duration := time.Since(startTime)
		sm.logger.Debug("Authentication attempt completed",
			zap.Duration("duration", duration),
			zap.String("method", string(sm.config.AuthMethod)),
			zap.String("remote_addr", req.RemoteAddr),
		)
	}()
	
	// Check rate limiting
	if sm.config.RateLimit.Enabled {
		if !sm.checkRateLimit(req) {
			sm.logSecurityEvent("rate_limit_exceeded", req, "BLOCKED", nil)
			return &AuthenticationResult{
				Success: false,
				Error:   fmt.Errorf("rate limit exceeded"),
			}, nil
		}
	}
	
	// If multi-auth is enabled, try all supported methods
	if sm.config.EnableMultiAuth {
		return sm.authenticateMultiMethod(ctx, req)
	}
	
	// Single authentication method
	return sm.authenticateSingleMethod(ctx, req, sm.config.AuthMethod)
}

// authenticateMultiMethod tries multiple authentication methods
func (sm *SecurityManager) authenticateMultiMethod(ctx context.Context, req *http.Request) (*AuthenticationResult, error) {
	var lastError error
	
	// Try each supported method in order
	for _, method := range sm.config.SupportedMethods {
		result, err := sm.authenticateSingleMethod(ctx, req, method)
		if err != nil {
			lastError = err
			continue
		}
		
		if result.Success {
			sm.logSecurityEvent("auth_success", req, "SUCCESS", result.User)
			return result, nil
		}
		
		if result.Error != nil {
			lastError = result.Error
		}
	}
	
	sm.logSecurityEvent("auth_failure", req, "FAILURE", nil)
	return &AuthenticationResult{
		Success: false,
		Error:   fmt.Errorf("authentication failed for all methods: %w", lastError),
	}, nil
}

// authenticateSingleMethod performs authentication with a specific method
func (sm *SecurityManager) authenticateSingleMethod(ctx context.Context, req *http.Request, method AuthMethod) (*AuthenticationResult, error) {
	switch method {
	case AuthMethodJWT:
		return sm.authenticateJWT(ctx, req)
	case AuthMethodAPIKey:
		return sm.authenticateAPIKey(ctx, req)
	case AuthMethodBasic:
		return sm.authenticateBasic(ctx, req)
	case AuthMethodHMAC:
		return sm.authenticateHMAC(ctx, req)
	case AuthMethodBearer:
		return sm.authenticateBearer(ctx, req)
	case AuthMethodMTLS:
		return sm.authenticateMTLS(ctx, req)
	default:
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("unsupported authentication method: %s", method),
		}, nil
	}
}

// JWT Authentication
func (sm *SecurityManager) authenticateJWT(ctx context.Context, req *http.Request) (*AuthenticationResult, error) {
	if sm.jwtManager == nil {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("JWT manager not initialized"),
		}, nil
	}
	
	// Extract token from request
	tokenString := sm.extractJWTToken(req)
	if tokenString == "" {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("no JWT token found"),
		}, nil
	}
	
	// Validate and parse token
	claims, err := sm.jwtManager.ValidateToken(tokenString)
	if err != nil {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("JWT validation failed: %w", err),
		}, nil
	}
	
	// Create user info from claims
	userInfo := sm.extractUserInfoFromJWTClaims(claims)
	
	// Extract claims safely
	var issuedAt, expiresAt time.Time
	if iat, ok := claims["iat"].(float64); ok {
		issuedAt = time.Unix(int64(iat), 0)
	}
	if exp, ok := claims["exp"].(float64); ok {
		expiresAt = time.Unix(int64(exp), 0)
	}
	
	// Create token info
	tokenInfo := &TokenInfo{
		Token:     tokenString,
		TokenType: TokenTypeAccess,
		Subject:   userInfo.ID,
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
	}
	
	// Store active token
	sm.mu.Lock()
	sm.activeTokens[tokenString] = tokenInfo
	sm.mu.Unlock()
	
	return &AuthenticationResult{
		Success: true,
		User:    userInfo,
		Token:   tokenInfo,
	}, nil
}

// API Key Authentication
func (sm *SecurityManager) authenticateAPIKey(ctx context.Context, req *http.Request) (*AuthenticationResult, error) {
	if sm.apiKeyManager == nil {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("API key manager not initialized"),
		}, nil
	}
	
	// Extract API key from request
	apiKey := sm.extractAPIKey(req)
	if apiKey == "" {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("no API key found"),
		}, nil
	}
	
	// Validate API key
	userInfo, err := sm.apiKeyManager.ValidateAPIKey(apiKey)
	if err != nil {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("API key validation failed: %w", err),
		}, nil
	}
	
	return &AuthenticationResult{
		Success: true,
		User:    userInfo,
	}, nil
}

// Basic Authentication
func (sm *SecurityManager) authenticateBasic(ctx context.Context, req *http.Request) (*AuthenticationResult, error) {
	if sm.basicAuthManager == nil {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("Basic Auth manager not initialized"),
		}, nil
	}
	
	// Extract credentials from request
	username, password, ok := req.BasicAuth()
	if !ok {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("no basic auth credentials found"),
		}, nil
	}
	
	// Validate credentials
	userInfo, err := sm.basicAuthManager.ValidateCredentials(username, password)
	if err != nil {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("basic auth validation failed: %w", err),
		}, nil
	}
	
	return &AuthenticationResult{
		Success: true,
		User:    userInfo,
	}, nil
}

// HMAC Authentication
func (sm *SecurityManager) authenticateHMAC(ctx context.Context, req *http.Request) (*AuthenticationResult, error) {
	if sm.hmacManager == nil {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("HMAC manager not initialized"),
		}, nil
	}
	
	// Validate HMAC signature
	userInfo, err := sm.hmacManager.ValidateSignature(req)
	if err != nil {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("HMAC validation failed: %w", err),
		}, nil
	}
	
	return &AuthenticationResult{
		Success: true,
		User:    userInfo,
	}, nil
}

// Bearer Authentication (similar to JWT but more generic)
func (sm *SecurityManager) authenticateBearer(ctx context.Context, req *http.Request) (*AuthenticationResult, error) {
	// Extract bearer token
	authHeader := req.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("no bearer token found"),
		}, nil
	}
	
	token := strings.TrimPrefix(authHeader, "Bearer ")
	
	// Check if it's a stored token
	sm.mu.RLock()
	tokenInfo, exists := sm.activeTokens[token]
	sm.mu.RUnlock()
	
	if !exists {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("invalid bearer token"),
		}, nil
	}
	
	// Check if token is expired
	if time.Now().After(tokenInfo.ExpiresAt) {
		sm.mu.Lock()
		delete(sm.activeTokens, token)
		sm.mu.Unlock()
		
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("bearer token expired"),
		}, nil
	}
	
	// Create user info from token
	userInfo := &UserInfo{
		ID: tokenInfo.Subject,
	}
	
	return &AuthenticationResult{
		Success: true,
		User:    userInfo,
		Token:   tokenInfo,
	}, nil
}

// Mutual TLS Authentication
func (sm *SecurityManager) authenticateMTLS(ctx context.Context, req *http.Request) (*AuthenticationResult, error) {
	if req.TLS == nil {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("no TLS connection"),
		}, nil
	}
	
	if len(req.TLS.PeerCertificates) == 0 {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("no client certificate"),
		}, nil
	}
	
	clientCert := req.TLS.PeerCertificates[0]
	
	// Validate client certificate
	if err := sm.validateClientCertificate(clientCert); err != nil {
		return &AuthenticationResult{
			Success: false,
			Error:   fmt.Errorf("client certificate validation failed: %w", err),
		}, nil
	}
	
	// Extract user info from certificate
	userInfo := sm.extractUserInfoFromCertificate(clientCert)
	
	return &AuthenticationResult{
		Success: true,
		User:    userInfo,
	}, nil
}

// Token management methods
func (sm *SecurityManager) RefreshToken(ctx context.Context, refreshToken string) (*TokenInfo, error) {
	if sm.jwtManager == nil {
		return nil, fmt.Errorf("JWT manager not initialized")
	}
	
	// Validate refresh token
	claims, err := sm.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("refresh token validation failed: %w", err)
	}
	
	// Extract subject from claims
	subject, _ := claims["sub"].(string)
	
	// Generate new access token
	newToken, err := sm.jwtManager.GenerateToken(subject, claims)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new token: %w", err)
	}
	
	// Create token info
	tokenInfo := &TokenInfo{
		Token:        newToken,
		TokenType:    TokenTypeAccess,
		Subject:      subject,
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(sm.config.JWT.AccessTokenTTL),
		RefreshToken: refreshToken,
	}
	
	// Store new token
	sm.mu.Lock()
	sm.activeTokens[newToken] = tokenInfo
	sm.mu.Unlock()
	
	// Store token in persistent storage
	if err := sm.tokenStorage.StoreToken(newToken, tokenInfo); err != nil {
		sm.logger.Warn("Failed to store token in persistent storage", zap.Error(err))
	}
	
	sm.logSecurityEvent("token_refresh", nil, "SUCCESS", &UserInfo{ID: subject})
	
	return tokenInfo, nil
}

// RevokeToken revokes a token
func (sm *SecurityManager) RevokeToken(ctx context.Context, token string) error {
	// Remove from active tokens
	sm.mu.Lock()
	tokenInfo, exists := sm.activeTokens[token]
	if exists {
		delete(sm.activeTokens, token)
	}
	sm.mu.Unlock()
	
	// Remove from persistent storage
	if err := sm.tokenStorage.RevokeToken(token); err != nil {
		return fmt.Errorf("failed to revoke token from storage: %w", err)
	}
	
	if exists && tokenInfo != nil {
		sm.logSecurityEvent("token_revoke", nil, "SUCCESS", &UserInfo{ID: tokenInfo.Subject})
	}
	
	return nil
}

// Middleware integration
func (sm *SecurityManager) AuthenticationMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Apply security headers
			sm.headerManager.ApplySecurityHeaders(w, r)
			
			// Perform authentication
			result, err := sm.Authenticate(r.Context(), r)
			if err != nil {
				sm.logger.Error("Authentication error", zap.Error(err))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			
			if !result.Success {
				sm.logger.Debug("Authentication failed", zap.Error(result.Error))
				http.Error(w, "Authentication failed", http.StatusUnauthorized)
				return
			}
			
			// Add user info to context
			ctx := context.WithValue(r.Context(), "user", result.User)
			ctx = context.WithValue(ctx, "token", result.Token)
			ctx = context.WithValue(ctx, "session", result.Session)
			
			// Continue with authenticated request
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Authorization methods
func (sm *SecurityManager) AuthorizeRequest(ctx context.Context, user *UserInfo, resource string, action string) error {
	// Simple role-based authorization - can be extended
	if user == nil {
		return fmt.Errorf("no user context")
	}
	
	// Check if user has required permissions
	requiredPermission := fmt.Sprintf("%s:%s", resource, action)
	for _, permission := range user.Permissions {
		if permission == requiredPermission || permission == "*" {
			return nil
		}
	}
	
	return fmt.Errorf("insufficient permissions")
}

// Utility methods
func (sm *SecurityManager) isMethodSupported(method AuthMethod) bool {
	if sm.config.AuthMethod == method {
		return true
	}
	
	for _, supported := range sm.config.SupportedMethods {
		if supported == method {
			return true
		}
	}
	
	return false
}

func (sm *SecurityManager) extractJWTToken(req *http.Request) string {
	// Try Authorization header first
	authHeader := req.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	
	// Try query parameter
	if token := req.URL.Query().Get("token"); token != "" {
		return token
	}
	
	// Try cookie
	if cookie, err := req.Cookie("jwt_token"); err == nil {
		return cookie.Value
	}
	
	return ""
}

func (sm *SecurityManager) extractAPIKey(req *http.Request) string {
	// Try configured header
	if sm.config.APIKey.HeaderName != "" {
		if key := req.Header.Get(sm.config.APIKey.HeaderName); key != "" {
			// Remove prefix if configured
			if sm.config.APIKey.Prefix != "" {
				return strings.TrimPrefix(key, sm.config.APIKey.Prefix+" ")
			}
			return key
		}
	}
	
	// Try configured query parameter
	if sm.config.APIKey.QueryParam != "" {
		if key := req.URL.Query().Get(sm.config.APIKey.QueryParam); key != "" {
			return key
		}
	}
	
	// Default to X-API-Key header
	return req.Header.Get("X-API-Key")
}

func (sm *SecurityManager) extractUserInfoFromJWTClaims(claims jwt.MapClaims) *UserInfo {
	userInfo := &UserInfo{
		Metadata: make(map[string]interface{}),
	}
	
	if sub, ok := claims["sub"].(string); ok {
		userInfo.ID = sub
	}
	
	if username, ok := claims["username"].(string); ok {
		userInfo.Username = username
	}
	
	if email, ok := claims["email"].(string); ok {
		userInfo.Email = email
	}
	
	if roles, ok := claims["roles"].([]interface{}); ok {
		userInfo.Roles = make([]string, len(roles))
		for i, role := range roles {
			if roleStr, ok := role.(string); ok {
				userInfo.Roles[i] = roleStr
			}
		}
	}
	
	if permissions, ok := claims["permissions"].([]interface{}); ok {
		userInfo.Permissions = make([]string, len(permissions))
		for i, perm := range permissions {
			if permStr, ok := perm.(string); ok {
				userInfo.Permissions[i] = permStr
			}
		}
	}
	
	// Copy other claims to metadata
	for key, value := range claims {
		if key != "sub" && key != "username" && key != "email" && key != "roles" && key != "permissions" {
			userInfo.Metadata[key] = value
		}
	}
	
	return userInfo
}

func (sm *SecurityManager) extractUserInfoFromCertificate(cert *x509.Certificate) *UserInfo {
	return &UserInfo{
		ID:       cert.Subject.CommonName,
		Username: cert.Subject.CommonName,
		Email:    strings.Join(cert.EmailAddresses, ","),
		Metadata: map[string]interface{}{
			"serial_number": cert.SerialNumber.String(),
			"issuer":        cert.Issuer.String(),
			"not_before":    cert.NotBefore,
			"not_after":     cert.NotAfter,
		},
	}
}

func (sm *SecurityManager) validateClientCertificate(cert *x509.Certificate) error {
	// Check if certificate is expired
	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf("client certificate has expired")
	}
	
	if time.Now().Before(cert.NotBefore) {
		return fmt.Errorf("client certificate is not yet valid")
	}
	
	// Additional validation can be added here
	// e.g., check against CRL, OCSP, custom validation rules
	
	return nil
}

func (sm *SecurityManager) checkRateLimit(req *http.Request) bool {
	// Simple IP-based rate limiting
	clientIP := sm.getClientIP(req)
	
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	limiter, exists := sm.rateLimiters[clientIP]
	if !exists {
		limiter = NewRateLimiter(int(sm.config.RateLimit.RequestsPerSecond), sm.config.RateLimit.BurstSize)
		sm.rateLimiters[clientIP] = limiter
	}
	
	return limiter.Allow()
}

func (sm *SecurityManager) getClientIP(req *http.Request) string {
	// Check for forwarded IP headers
	if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}
	
	if realIP := req.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	
	// Default to remote address
	return req.RemoteAddr
}

func (sm *SecurityManager) logSecurityEvent(eventType string, req *http.Request, result string, user *UserInfo) {
	if sm.auditLogger == nil {
		return
	}
	
	event := &SecurityEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		Result:    result,
	}
	
	if req != nil {
		event.IPAddress = sm.getClientIP(req)
		event.UserAgent = req.Header.Get("User-Agent")
		event.Resource = req.URL.Path
		event.Action = req.Method
	}
	
	if user != nil {
		event.UserID = user.ID
	}
	
	sm.auditLogger.LogEvent(event)
}

// Cleanup method
func (sm *SecurityManager) Cleanup() error {
	var errs []error
	
	// Cleanup JWT manager
	if sm.jwtManager != nil {
		if err := sm.jwtManager.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("JWT manager cleanup failed: %w", err))
		}
	}
	
	// Cleanup other managers
	if sm.apiKeyManager != nil {
		if err := sm.apiKeyManager.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("API key manager cleanup failed: %w", err))
		}
	}
	
	if sm.basicAuthManager != nil {
		if err := sm.basicAuthManager.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("Basic auth manager cleanup failed: %w", err))
		}
	}
	
	if sm.hmacManager != nil {
		if err := sm.hmacManager.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("HMAC manager cleanup failed: %w", err))
		}
	}
	
	if sm.certManager != nil {
		if err := sm.certManager.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("Certificate manager cleanup failed: %w", err))
		}
	}
	
	if sm.tokenStorage != nil {
		if err := sm.tokenStorage.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("Token storage cleanup failed: %w", err))
		}
	}
	
	if sm.auditLogger != nil {
		if err := sm.auditLogger.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("Audit logger cleanup failed: %w", err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	
	return nil
}

// Helper function to create default security configuration
func NewDefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		AuthMethod:       AuthMethodJWT,
		EnableMultiAuth:  false,
		SupportedMethods: []AuthMethod{AuthMethodJWT, AuthMethodAPIKey},
		JWT: JWTConfig{
			SigningMethod:     "HS256",
			AccessTokenTTL:    time.Hour,
			RefreshTokenTTL:   24 * time.Hour,
			AutoRefresh:       true,
			RefreshThreshold:  15 * time.Minute,
			LeewayTime:        1 * time.Minute,
		},
		APIKey: APIKeyConfig{
			HeaderName:         "X-API-Key",
			QueryParam:         "api_key",
			HashingAlgorithm:   "sha256",
			EnableKeyRotation:  false,
			KeyRotationInterval: 30 * 24 * time.Hour,
		},
		BasicAuth: BasicAuthConfig{
			Realm:            "AG-UI API",
			HashingAlgorithm: "bcrypt",
			PasswordPolicy: PasswordPolicy{
				MinLength:        8,
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumbers:   true,
				MaxAge:           90 * 24 * time.Hour,
			},
		},
		TLS: TLSConfig{
			Enabled:               true,
			ClientAuth:            tls.RequestClientCert,
			MinVersion:            tls.VersionTLS12,
			MaxVersion:            tls.VersionTLS13,
			EnableSNI:             true,
			InsecureSkipVerify:    false,
			CertificateValidation: CertValidationConfig{
				ValidateCertChain: true,
				ValidateHostname:  true,
				CustomCAPool:      false,
			},
		},
		HMAC: HMACConfig{
			Algorithm:        "sha256",
			HeaderName:       "X-Signature",
			TimestampHeader:  "X-Timestamp",
			NonceHeader:      "X-Nonce",
			MaxClockSkew:     5 * time.Minute,
			IncludeHeaders:   []string{"Content-Type", "Date"},
		},
		SecurityHeaders: SecurityHeadersConfig{
			EnableCSP:           true,
			CSPPolicy:           "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline';",
			EnableHSTS:          true,
			HSTSMaxAge:          31536000, // 1 year
			EnableXFrameOptions: true,
			XFrameOptions:       "DENY",
			EnableXContentType:  true,
			EnableReferrerPolicy: true,
			ReferrerPolicy:      "strict-origin-when-cross-origin",
			CustomHeaders:       make(map[string]string),
			CORSConfig: CORSConfig{
				Enabled:          true,
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"Content-Type", "Authorization", "X-API-Key"},
				AllowCredentials: true,
				MaxAge:           86400, // 24 hours
			},
		},
		TokenStorage: TokenStorageConfig{
			StorageType: "memory",
			Encryption: EncryptionConfig{
				Enabled:   true,
				Algorithm: "aes-256-gcm",
			},
		},
		AuditLogging: AuditLoggingConfig{
			Enabled:          true,
			LogLevel:         "info",
			LogFormat:        "json",
			LogSensitiveData: false,
			EventTypes: []string{
				"auth_success", "auth_failure", "token_refresh",
				"token_revoke", "rate_limit_exceeded",
			},
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 100,
			BurstSize:         200,
			WindowSize:        time.Minute,
		},
		SessionConfig: SessionConfig{
			Enabled:        true,
			CookieName:     "ag_ui_session",
			CookieSecure:   true,
			CookieHTTPOnly: true,
			CookieSameSite: "Strict",
			SessionTimeout: 24 * time.Hour,
		},
	}
}