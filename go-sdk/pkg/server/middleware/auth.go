package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Performance optimization pools for frequently allocated objects
var (
	// AuthUserPool for reusing user objects - exported for benchmark access
	AuthUserPool = sync.Pool{
		New: func() interface{} {
			return &AuthUser{
				Roles:       make([]string, 0, 4),
				Permissions: make([]string, 0, 4),
				Metadata:    make(map[string]interface{}),
			}
		},
	}
	
	// Claims map pool for JWT processing
	claimsMapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{}, 8)
		},
	}
	
	// String slice pool for roles/permissions
	stringSlicePool = sync.Pool{
		New: func() interface{} {
			return make([]string, 0, 4)
		},
	}
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
	
	// AuthMethodHMAC uses HMAC signature-based authentication
	AuthMethodHMAC AuthMethod = "hmac"
	
	// AuthMethodBearer uses Bearer token authentication
	AuthMethodBearer AuthMethod = "bearer"
)

// BCrypt security constants for enforcing minimum security standards
const (
	// BCryptMinCost is the minimum secure cost (2^12 = 4096 iterations)
	// Industry standard for security as of 2024
	BCryptMinCost = 12
	
	// BCryptMaxCost is the maximum cost to prevent DoS attacks (2^15 = 32768 iterations)  
	// Higher costs can cause excessive computation time and potential DoS
	BCryptMaxCost = 15
	
	// BCryptDefaultCost is the secure default cost when none specified
	// Uses minimum secure cost as default
	BCryptDefaultCost = BCryptMinCost
)

// AuthUser represents an authenticated user
type AuthUser struct {
	ID          string                 `json:"id"`
	Username    string                 `json:"username"`
	Email       string                 `json:"email"`
	Roles       []string               `json:"roles"`
	Permissions []string               `json:"permissions"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// HasRole checks if the user has a specific role
func (u *AuthUser) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the user has any of the specified roles
func (u *AuthUser) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if u.HasRole(role) {
			return true
		}
	}
	return false
}

// HasPermission checks if the user has a specific permission
func (u *AuthUser) HasPermission(permission string) bool {
	for _, p := range u.Permissions {
		if p == permission || p == "*" {
			return true
		}
	}
	return false
}

// BCrypt cost validation functions

// ValidateBCryptCost validates that a BCrypt cost parameter meets security requirements
func ValidateBCryptCost(cost int) error {
	if cost < BCryptMinCost {
		return fmt.Errorf("bcrypt cost %d is below minimum secure cost %d (2^%d iterations)", cost, BCryptMinCost, BCryptMinCost)
	}
	if cost > BCryptMaxCost {
		return fmt.Errorf("bcrypt cost %d exceeds maximum allowed cost %d (2^%d iterations) to prevent DoS", cost, BCryptMaxCost, BCryptMaxCost)
	}
	return nil
}

// NormalizeBCryptCost ensures BCrypt cost is within acceptable bounds, using default if invalid
func NormalizeBCryptCost(cost int) int {
	if cost < BCryptMinCost || cost > BCryptMaxCost {
		return BCryptDefaultCost
	}
	return cost
}

// GetSecureBCryptCost returns a secure BCrypt cost, validating the input and providing defaults
func GetSecureBCryptCost(cost int) (int, error) {
	if cost == 0 {
		return BCryptDefaultCost, nil
	}
	
	if err := ValidateBCryptCost(cost); err != nil {
		return 0, fmt.Errorf("invalid bcrypt cost: %w", err)
	}
	
	return cost, nil
}

// AuthConfig contains authentication middleware configuration
type AuthConfig struct {
	BaseConfig `json:",inline" yaml:",inline"`
	
	// Method specifies the authentication method to use
	Method AuthMethod `json:"method" yaml:"method"`
	
	// Global BCrypt cost for password/API key hashing (12-15, default 12)
	// Controls computational cost - higher values are more secure but slower
	// This can be overridden by method-specific configurations
	BCryptCost int `json:"bcrypt_cost" yaml:"bcrypt_cost"`
	
	// JWT configuration
	JWT JWTConfig `json:"jwt" yaml:"jwt"`
	
	// API Key configuration
	APIKey APIKeyConfig `json:"api_key" yaml:"api_key"`
	
	// Basic Auth configuration
	BasicAuth BasicAuthConfig `json:"basic_auth" yaml:"basic_auth"`
	
	// HMAC configuration
	HMAC HMACConfig `json:"hmac" yaml:"hmac"`
	
	// Bearer token configuration
	Bearer BearerConfig `json:"bearer" yaml:"bearer"`
	
	// Authorization configuration
	RequiredRoles       []string `json:"required_roles" yaml:"required_roles"`
	RequiredPermissions []string `json:"required_permissions" yaml:"required_permissions"`
	
	// Error handling
	SecureErrorMode bool `json:"secure_error_mode" yaml:"secure_error_mode"`
	
	// Optional paths that don't require authentication
	OptionalPaths []string `json:"optional_paths" yaml:"optional_paths"`
	
	// Excluded paths that bypass authentication entirely
	ExcludedPaths []string `json:"excluded_paths" yaml:"excluded_paths"`
}

// JWTConfig contains JWT-specific configuration with secure credential handling
type JWTConfig struct {
	// Signing configuration - SECURE: Uses environment variables instead of plaintext
	SigningMethod string `json:"signing_method" yaml:"signing_method"`
	SecretKeyEnv  string `json:"secret_key_env" yaml:"secret_key_env"`   // Environment variable name for HMAC secret
	PublicKeyEnv  string `json:"public_key_env" yaml:"public_key_env"`   // Environment variable name for RSA/ECDSA public key
	PrivateKeyEnv string `json:"private_key_env" yaml:"private_key_env"` // Environment variable name for RSA/ECDSA private key
	
	// Token validation
	Issuer       string        `json:"issuer" yaml:"issuer"`
	Audience     []string      `json:"audience" yaml:"audience"`
	LeewayTime   time.Duration `json:"leeway_time" yaml:"leeway_time"`
	
	// Token extraction
	TokenHeader string `json:"token_header" yaml:"token_header"`
	TokenPrefix string `json:"token_prefix" yaml:"token_prefix"`
	QueryParam  string `json:"query_param" yaml:"query_param"`
	CookieName  string `json:"cookie_name" yaml:"cookie_name"`

	// Runtime secure credentials (populated from environment variables)
	secretKey  *SecureCredential
	publicKey  *SecureCredential
	privateKey *SecureCredential
}

// APIKeyConfig contains API key authentication configuration
type APIKeyConfig struct {
	// Header configuration
	HeaderName string `json:"header_name" yaml:"header_name"`
	Prefix     string `json:"prefix" yaml:"prefix"`
	
	// Query parameter configuration
	QueryParam string `json:"query_param" yaml:"query_param"`
	
	// Validation
	Keys           map[string]*APIKeyInfo `json:"keys" yaml:"keys"`
	ValidateLength bool                   `json:"validate_length" yaml:"validate_length"`
	MinLength      int                    `json:"min_length" yaml:"min_length"`
	MaxLength      int                    `json:"max_length" yaml:"max_length"`
}

// APIKeyInfo contains information about an API key
type APIKeyInfo struct {
	UserID      string                 `json:"user_id"`
	Username    string                 `json:"username"`
	Roles       []string               `json:"roles"`
	Permissions []string               `json:"permissions"`
	Metadata    map[string]interface{} `json:"metadata"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	LastUsedAt  *time.Time             `json:"last_used_at,omitempty"`
}

// IsExpired checks if the API key is expired
func (info *APIKeyInfo) IsExpired() bool {
	return info.ExpiresAt != nil && time.Now().After(*info.ExpiresAt)
}

// BasicAuthConfig contains Basic authentication configuration
type BasicAuthConfig struct {
	// Realm for Basic auth challenge
	Realm string `json:"realm" yaml:"realm"`
	
	// BCrypt cost for password hashing (12-15, default 12)
	// Controls computational cost - higher values are more secure but slower
	BCryptCost int `json:"bcrypt_cost" yaml:"bcrypt_cost"`
	
	// User credentials (username -> hashed password)
	Users map[string]*BasicAuthUser `json:"users" yaml:"users"`
}

// BasicAuthUser contains Basic auth user information
type BasicAuthUser struct {
	PasswordHash string                 `json:"password_hash"`
	UserID       string                 `json:"user_id"`
	Roles        []string               `json:"roles"`
	Permissions  []string               `json:"permissions"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// HMACConfig contains HMAC signature authentication configuration with secure credential handling
type HMACConfig struct {
	// Signature configuration - SECURE: Uses environment variables instead of plaintext
	SecretKeyEnv string `json:"secret_key_env" yaml:"secret_key_env"` // Environment variable name for HMAC secret
	Algorithm    string `json:"algorithm" yaml:"algorithm"`
	
	// Header configuration
	SignatureHeader string   `json:"signature_header" yaml:"signature_header"`
	TimestampHeader string   `json:"timestamp_header" yaml:"timestamp_header"`
	NonceHeader     string   `json:"nonce_header" yaml:"nonce_header"`
	IncludeHeaders  []string `json:"include_headers" yaml:"include_headers"`
	
	// Validation
	MaxClockSkew time.Duration `json:"max_clock_skew" yaml:"max_clock_skew"`
	RequireNonce bool          `json:"require_nonce" yaml:"require_nonce"`
	
	// User mapping (signature -> user info)
	Users map[string]*HMACUser `json:"users" yaml:"users"`

	// Runtime secure credentials (populated from environment variables)
	secretKey *SecureCredential
}

// HMACUser contains HMAC user information
type HMACUser struct {
	UserID      string                 `json:"user_id"`
	Username    string                 `json:"username"`
	Roles       []string               `json:"roles"`
	Permissions []string               `json:"permissions"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// BearerConfig contains Bearer token configuration
type BearerConfig struct {
	// Token validation
	Tokens map[string]*BearerTokenInfo `json:"tokens" yaml:"tokens"`
	
	// Header configuration
	HeaderName string `json:"header_name" yaml:"header_name"`
	Prefix     string `json:"prefix" yaml:"prefix"`
}

// BearerTokenInfo contains Bearer token information
type BearerTokenInfo struct {
	UserID      string                 `json:"user_id"`
	Username    string                 `json:"username"`
	Roles       []string               `json:"roles"`
	Permissions []string               `json:"permissions"`
	Metadata    map[string]interface{} `json:"metadata"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// IsExpired checks if the bearer token is expired
func (info *BearerTokenInfo) IsExpired() bool {
	return info.ExpiresAt != nil && time.Now().After(*info.ExpiresAt)
}

// AuthMiddleware implements authentication middleware with secure credential management
type AuthMiddleware struct {
	config    *AuthConfig
	logger    *zap.Logger
	mu        sync.RWMutex
	
	// JWT parser and validator
	jwtParser *jwt.Parser
	
	// Used nonces for HMAC replay protection
	usedNonces map[string]time.Time

	// Secure credential management
	credentialManager *CredentialManager
	auditor          *CredentialAuditor
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(config *AuthConfig, logger *zap.Logger) (*AuthMiddleware, error) {
	if config == nil {
		return nil, fmt.Errorf("auth config cannot be nil")
	}
	
	if err := ValidateBaseConfig(&config.BaseConfig); err != nil {
		return nil, fmt.Errorf("invalid base config: %w", err)
	}
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	// Set defaults
	if config.Name == "" {
		config.Name = "auth"
	}
	if config.Priority == 0 {
		config.Priority = 100 // High priority for auth
	}
	
	// Validate and set BCrypt cost defaults
	if config.BCryptCost == 0 {
		config.BCryptCost = BCryptDefaultCost
	} else {
		if err := ValidateBCryptCost(config.BCryptCost); err != nil {
			return nil, fmt.Errorf("invalid global bcrypt cost: %w", err)
		}
	}
	
	// Validate Basic Auth BCrypt cost and existing password hashes
	if config.Method == AuthMethodBasic {
		if config.BasicAuth.BCryptCost == 0 {
			config.BasicAuth.BCryptCost = config.BCryptCost // Use global default
		} else {
			if err := ValidateBCryptCost(config.BasicAuth.BCryptCost); err != nil {
				return nil, fmt.Errorf("invalid basic auth bcrypt cost: %w", err)
			}
		}
		
		// Validate existing user password hashes meet security requirements
		for username, user := range config.BasicAuth.Users {
			if user.PasswordHash != "" {
				if err := ValidatePasswordHash(user.PasswordHash); err != nil {
					logger.Warn("User password hash does not meet security requirements",
						zap.String("username", username),
						zap.Error(err),
					)
					// Don't fail startup but log warning - allow graceful migration
				}
			}
		}
	}
	
	if config.JWT.TokenHeader == "" {
		config.JWT.TokenHeader = "Authorization"
	}
	if config.JWT.TokenPrefix == "" {
		config.JWT.TokenPrefix = "Bearer "
	}
	if config.JWT.SigningMethod == "" {
		config.JWT.SigningMethod = "HS256"
	}
	if config.APIKey.HeaderName == "" {
		config.APIKey.HeaderName = "X-API-Key"
	}
	if config.BasicAuth.Realm == "" {
		config.BasicAuth.Realm = "AG-UI API"
	}
	if config.HMAC.Algorithm == "" {
		config.HMAC.Algorithm = "sha256"
	}
	if config.HMAC.SignatureHeader == "" {
		config.HMAC.SignatureHeader = "X-Signature"
	}
	if config.HMAC.TimestampHeader == "" {
		config.HMAC.TimestampHeader = "X-Timestamp"
	}
	if config.HMAC.MaxClockSkew == 0 {
		config.HMAC.MaxClockSkew = 5 * time.Minute
	}
	if config.Bearer.HeaderName == "" {
		config.Bearer.HeaderName = "Authorization"
	}
	if config.Bearer.Prefix == "" {
		config.Bearer.Prefix = "Bearer "
	}
	
	middleware := &AuthMiddleware{
		config:            config,
		logger:            logger,
		usedNonces:        make(map[string]time.Time),
		credentialManager: NewCredentialManager(logger),
		auditor:          NewCredentialAuditor(logger),
	}

	// Load secure credentials from environment variables
	if err := middleware.loadSecureCredentials(); err != nil {
		return nil, fmt.Errorf("failed to load secure credentials: %w", err)
	}
	
	// Initialize JWT parser
	if config.Method == AuthMethodJWT {
		middleware.jwtParser = jwt.NewParser(
			jwt.WithValidMethods([]string{config.JWT.SigningMethod}),
		)
		if config.JWT.LeewayTime > 0 {
			middleware.jwtParser = jwt.NewParser(
				jwt.WithValidMethods([]string{config.JWT.SigningMethod}),
				jwt.WithLeeway(config.JWT.LeewayTime),
			)
		}
	}
	
	return middleware, nil
}

// loadSecureCredentials loads credentials from environment variables
func (am *AuthMiddleware) loadSecureCredentials() error {
	// Load JWT credentials if JWT method is enabled
	if am.config.Method == AuthMethodJWT {
		if err := am.loadJWTCredentials(); err != nil {
			return fmt.Errorf("failed to load JWT credentials: %w", err)
		}
	}

	// Load HMAC credentials if HMAC method is enabled
	if am.config.Method == AuthMethodHMAC {
		if err := am.loadHMACCredentials(); err != nil {
			return fmt.Errorf("failed to load HMAC credentials: %w", err)
		}
	}

	return nil
}

// loadJWTCredentials loads JWT signing credentials from environment variables
func (am *AuthMiddleware) loadJWTCredentials() error {
	// Determine which credentials are needed based on signing method
	switch am.config.JWT.SigningMethod {
	case "HS256", "HS384", "HS512":
		// HMAC methods require secret key
		if am.config.JWT.SecretKeyEnv == "" {
			return fmt.Errorf("JWT secret key environment variable not specified for HMAC method")
		}
		
		if err := am.credentialManager.LoadCredential("jwt_secret", am.config.JWT.SecretKeyEnv, DefaultJWTValidator()); err != nil {
			return fmt.Errorf("failed to load JWT secret key: %w", err)
		}
		
		cred, _ := am.credentialManager.GetCredential("jwt_secret")
		am.config.JWT.secretKey = cred
		am.auditor.AuditCredentialValidation("jwt_secret", true, nil)

	case "RS256", "RS384", "RS512", "ES256", "ES384", "ES512":
		// RSA/ECDSA methods require public/private keys
		if am.config.JWT.PublicKeyEnv != "" {
			if err := am.credentialManager.LoadCredential("jwt_public", am.config.JWT.PublicKeyEnv, &CredentialValidator{MinLength: 100}); err != nil {
				return fmt.Errorf("failed to load JWT public key: %w", err)
			}
			
			cred, _ := am.credentialManager.GetCredential("jwt_public")
			am.config.JWT.publicKey = cred
			am.auditor.AuditCredentialValidation("jwt_public", true, nil)
		}

		if am.config.JWT.PrivateKeyEnv != "" {
			if err := am.credentialManager.LoadCredential("jwt_private", am.config.JWT.PrivateKeyEnv, &CredentialValidator{MinLength: 100}); err != nil {
				return fmt.Errorf("failed to load JWT private key: %w", err)
			}
			
			cred, _ := am.credentialManager.GetCredential("jwt_private")
			am.config.JWT.privateKey = cred
			am.auditor.AuditCredentialValidation("jwt_private", true, nil)
		}

	default:
		return fmt.Errorf("unsupported JWT signing method: %s", am.config.JWT.SigningMethod)
	}

	return nil
}

// loadHMACCredentials loads HMAC signing credentials from environment variables
func (am *AuthMiddleware) loadHMACCredentials() error {
	if am.config.HMAC.SecretKeyEnv == "" {
		return fmt.Errorf("HMAC secret key environment variable not specified")
	}

	if err := am.credentialManager.LoadCredential("hmac_secret", am.config.HMAC.SecretKeyEnv, DefaultHMACValidator()); err != nil {
		return fmt.Errorf("failed to load HMAC secret key: %w", err)
	}
	
	cred, _ := am.credentialManager.GetCredential("hmac_secret")
	am.config.HMAC.secretKey = cred
	am.auditor.AuditCredentialValidation("hmac_secret", true, nil)

	return nil
}

// Handler implements the Middleware interface
func (am *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !am.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		
		// Check if path is excluded
		if am.isExcludedPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		
		// Perform authentication
		user, err := am.authenticate(r)
		if err != nil {
			am.handleAuthError(w, r, err)
			return
		}
		
		// Check if authentication is optional for this path
		if user == nil && am.isOptionalPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		
		// Require authentication for non-optional paths
		if user == nil {
			am.handleAuthError(w, r, fmt.Errorf("authentication required"))
			return
		}
		
		// Perform authorization
		if err := am.authorize(user); err != nil {
			am.handleAuthError(w, r, err)
			return
		}
		
		// Add user to context
		ctx := context.WithValue(r.Context(), AuthContextKey, user)
		ctx = SetUserID(ctx, user.ID)
		
		// Continue with authenticated request
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Name returns the middleware name
func (am *AuthMiddleware) Name() string {
	return am.config.Name
}

// Priority returns the middleware priority
func (am *AuthMiddleware) Priority() int {
	return am.config.Priority
}

// Config returns the middleware configuration
func (am *AuthMiddleware) Config() interface{} {
	return am.config
}

// Cleanup performs cleanup including secure credential cleanup
func (am *AuthMiddleware) Cleanup() error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	// Clear used nonces
	am.usedNonces = make(map[string]time.Time)
	
	// Securely cleanup credentials
	if am.credentialManager != nil {
		am.credentialManager.Cleanup()
	}
	
	// Clear credential references in configs
	if am.config.JWT.secretKey != nil {
		am.config.JWT.secretKey = nil
	}
	if am.config.JWT.publicKey != nil {
		am.config.JWT.publicKey = nil
	}
	if am.config.JWT.privateKey != nil {
		am.config.JWT.privateKey = nil
	}
	if am.config.HMAC.secretKey != nil {
		am.config.HMAC.secretKey = nil
	}
	
	am.logger.Info("AuthMiddleware cleanup completed with secure credential clearing")
	
	return nil
}

// authenticate performs authentication based on the configured method
func (am *AuthMiddleware) authenticate(r *http.Request) (*AuthUser, error) {
	switch am.config.Method {
	case AuthMethodJWT:
		return am.authenticateJWT(r)
	case AuthMethodAPIKey:
		return am.authenticateAPIKey(r)
	case AuthMethodBasic:
		return am.authenticateBasic(r)
	case AuthMethodHMAC:
		return am.authenticateHMAC(r)
	case AuthMethodBearer:
		return am.authenticateBearer(r)
	default:
		return nil, fmt.Errorf("unsupported authentication method: %s", am.config.Method)
	}
}

// authenticateJWT performs JWT authentication
func (am *AuthMiddleware) authenticateJWT(r *http.Request) (*AuthUser, error) {
	// Extract token
	tokenString := am.extractJWTToken(r)
	if tokenString == "" {
		return nil, fmt.Errorf("no JWT token found")
	}
	
	// Parse and validate token
	token, err := am.jwtParser.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if token.Method.Alg() != am.config.JWT.SigningMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		
		// Return appropriate key based on signing method - SECURE VERSION
		switch am.config.JWT.SigningMethod {
		case "HS256", "HS384", "HS512":
			if am.config.JWT.secretKey == nil {
				return nil, fmt.Errorf("JWT secret key not loaded")
			}
			am.auditor.AuditCredentialAccess("jwt_secret", "signing", true)
			return am.config.JWT.secretKey.Bytes(), nil
		case "RS256", "RS384", "RS512":
			// For verification, use public key
			if am.config.JWT.publicKey == nil {
				return nil, fmt.Errorf("JWT public key not loaded")
			}
			am.auditor.AuditCredentialAccess("jwt_public", "verification", true)
			return am.config.JWT.publicKey.Bytes(), nil
		default:
			return nil, fmt.Errorf("unsupported signing method: %s", am.config.JWT.SigningMethod)
		}
	})
	
	if err != nil {
		return nil, fmt.Errorf("JWT validation failed: %w", err)
	}
	
	if !token.Valid {
		return nil, fmt.Errorf("invalid JWT token")
	}
	
	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid JWT claims")
	}
	
	// Validate issuer if configured
	if am.config.JWT.Issuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != am.config.JWT.Issuer {
			return nil, fmt.Errorf("invalid issuer")
		}
	}
	
	// Validate audience if configured
	if len(am.config.JWT.Audience) > 0 {
		if aud, ok := claims["aud"].([]interface{}); ok {
			validAudience := false
			for _, configAud := range am.config.JWT.Audience {
				for _, tokenAud := range aud {
					if audStr, ok := tokenAud.(string); ok && audStr == configAud {
						validAudience = true
						break
					}
				}
				if validAudience {
					break
				}
			}
			if !validAudience {
				return nil, fmt.Errorf("invalid audience")
			}
		}
	}
	
	// Create user from claims using object pool
	user := AuthUserPool.Get().(*AuthUser)
	ResetAuthUser(user) // Clear previous data
	
	if sub, ok := claims["sub"].(string); ok {
		user.ID = sub
	}
	if username, ok := claims["username"].(string); ok {
		user.Username = username
	}
	if email, ok := claims["email"].(string); ok {
		user.Email = email
	}
	if roles, ok := claims["roles"].([]interface{}); ok {
		user.Roles = make([]string, len(roles))
		for i, role := range roles {
			if roleStr, ok := role.(string); ok {
				user.Roles[i] = roleStr
			}
		}
	}
	if permissions, ok := claims["permissions"].([]interface{}); ok {
		user.Permissions = make([]string, len(permissions))
		for i, perm := range permissions {
			if permStr, ok := perm.(string); ok {
				user.Permissions[i] = permStr
			}
		}
	}
	
	// Copy other claims to metadata
	for key, value := range claims {
		if key != "sub" && key != "username" && key != "email" && key != "roles" && key != "permissions" {
			user.Metadata[key] = value
		}
	}
	
	return user, nil
}

// ResetAuthUser clears an AuthUser object for reuse - exported for benchmark access
func ResetAuthUser(user *AuthUser) {
	user.ID = ""
	user.Username = ""
	user.Email = ""
	user.Roles = user.Roles[:0]        // Keep capacity, reset length
	user.Permissions = user.Permissions[:0] // Keep capacity, reset length
	// Clear map efficiently
	for k := range user.Metadata {
		delete(user.Metadata, k)
	}
}

// ReleaseAuthUser returns an AuthUser to the pool - exported for benchmark access
func ReleaseAuthUser(user *AuthUser) {
	if user != nil {
		ResetAuthUser(user)
		AuthUserPool.Put(user)
	}
}

// Private compatibility functions for internal use
func resetAuthUser(user *AuthUser) {
	ResetAuthUser(user)
}

func releaseAuthUser(user *AuthUser) {
	ReleaseAuthUser(user)
}

var authUserPool = &AuthUserPool

// authenticateAPIKey performs API key authentication
func (am *AuthMiddleware) authenticateAPIKey(r *http.Request) (*AuthUser, error) {
	// Extract API key
	apiKey := am.extractAPIKey(r)
	if apiKey == "" {
		return nil, fmt.Errorf("no API key found")
	}
	
	// Validate key length if configured
	if am.config.APIKey.ValidateLength {
		if len(apiKey) < am.config.APIKey.MinLength || len(apiKey) > am.config.APIKey.MaxLength {
			return nil, fmt.Errorf("invalid API key length")
		}
	}
	
	// Look up key info
	keyInfo, exists := am.config.APIKey.Keys[apiKey]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}
	
	// Check expiration
	if keyInfo.IsExpired() {
		return nil, fmt.Errorf("API key expired")
	}
	
	// Update last used time
	now := time.Now()
	keyInfo.LastUsedAt = &now
	
	// Create user from key info using object pool
	user := AuthUserPool.Get().(*AuthUser)
	ResetAuthUser(user)
	user.ID = keyInfo.UserID
	user.Username = keyInfo.Username
	user.Roles = append(user.Roles[:0], keyInfo.Roles...)
	user.Permissions = append(user.Permissions[:0], keyInfo.Permissions...)
	for k, v := range keyInfo.Metadata {
		user.Metadata[k] = v
	}
	
	return user, nil
}

// authenticateBasic performs Basic authentication
func (am *AuthMiddleware) authenticateBasic(r *http.Request) (*AuthUser, error) {
	// Extract credentials
	username, password, ok := r.BasicAuth()
	if !ok {
		return nil, fmt.Errorf("no Basic auth credentials found")
	}
	
	// Look up user
	userInfo, exists := am.config.BasicAuth.Users[username]
	if !exists {
		return nil, fmt.Errorf("invalid credentials")
	}
	
	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(userInfo.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	
	// Create user from info using object pool
	user := AuthUserPool.Get().(*AuthUser)
	ResetAuthUser(user)
	user.ID = userInfo.UserID
	user.Username = username
	user.Roles = append(user.Roles[:0], userInfo.Roles...)
	user.Permissions = append(user.Permissions[:0], userInfo.Permissions...)
	for k, v := range userInfo.Metadata {
		user.Metadata[k] = v
	}
	
	return user, nil
}

// authenticateHMAC performs HMAC signature authentication
func (am *AuthMiddleware) authenticateHMAC(r *http.Request) (*AuthUser, error) {
	// Extract signature
	signature := r.Header.Get(am.config.HMAC.SignatureHeader)
	if signature == "" {
		return nil, fmt.Errorf("no HMAC signature found")
	}
	
	// Extract timestamp
	timestampStr := r.Header.Get(am.config.HMAC.TimestampHeader)
	if timestampStr == "" {
		return nil, fmt.Errorf("no timestamp found")
	}
	
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp format")
	}
	
	// Check clock skew
	now := time.Now().Unix()
	if abs(now-timestamp) > int64(am.config.HMAC.MaxClockSkew.Seconds()) {
		return nil, fmt.Errorf("timestamp outside allowed clock skew")
	}
	
	// Extract nonce if required
	var nonce string
	if am.config.HMAC.RequireNonce {
		nonce = r.Header.Get(am.config.HMAC.NonceHeader)
		if nonce == "" {
			return nil, fmt.Errorf("no nonce found")
		}
		
		// Check for replay attack
		am.mu.Lock()
		if _, used := am.usedNonces[nonce]; used {
			am.mu.Unlock()
			return nil, fmt.Errorf("nonce already used")
		}
		am.usedNonces[nonce] = time.Now()
		am.mu.Unlock()
	}
	
	// Build signature string
	sigString := am.buildSignatureString(r, timestampStr, nonce)
	
	// Calculate expected signature
	expectedSig := am.calculateHMAC(sigString)
	
	// Compare signatures
	if !am.constantTimeCompare(signature, expectedSig) {
		return nil, fmt.Errorf("invalid HMAC signature")
	}
	
	// For HMAC, we need a way to identify the user
	// This is a simplified approach - in practice, you might include user ID in the signature
	// or have a separate header for user identification
	userID := r.Header.Get("X-User-ID")
	if userInfo, exists := am.config.HMAC.Users[userID]; exists {
		user := AuthUserPool.Get().(*AuthUser)
		ResetAuthUser(user)
		user.ID = userInfo.UserID
		user.Username = userInfo.Username
		user.Roles = append(user.Roles[:0], userInfo.Roles...)
		user.Permissions = append(user.Permissions[:0], userInfo.Permissions...)
		for k, v := range userInfo.Metadata {
			user.Metadata[k] = v
		}
		return user, nil
	}
	
	// Default user for valid HMAC signature using object pool
	user := AuthUserPool.Get().(*AuthUser)
	ResetAuthUser(user)
	user.ID = "hmac-user"
	user.Username = "hmac-user"
	return user, nil
}

// authenticateBearer performs Bearer token authentication
func (am *AuthMiddleware) authenticateBearer(r *http.Request) (*AuthUser, error) {
	// Extract token
	token := am.extractBearerToken(r)
	if token == "" {
		return nil, fmt.Errorf("no Bearer token found")
	}
	
	// Look up token info
	tokenInfo, exists := am.config.Bearer.Tokens[token]
	if !exists {
		return nil, fmt.Errorf("invalid Bearer token")
	}
	
	// Check expiration
	if tokenInfo.IsExpired() {
		return nil, fmt.Errorf("Bearer token expired")
	}
	
	// Create user from token info using object pool
	user := AuthUserPool.Get().(*AuthUser)
	ResetAuthUser(user)
	user.ID = tokenInfo.UserID
	user.Username = tokenInfo.Username
	user.Roles = append(user.Roles[:0], tokenInfo.Roles...)
	user.Permissions = append(user.Permissions[:0], tokenInfo.Permissions...)
	for k, v := range tokenInfo.Metadata {
		user.Metadata[k] = v
	}
	
	return user, nil
}

// authorize performs authorization checks
func (am *AuthMiddleware) authorize(user *AuthUser) error {
	// Check required roles
	if len(am.config.RequiredRoles) > 0 {
		if !user.HasAnyRole(am.config.RequiredRoles...) {
			return fmt.Errorf("insufficient roles")
		}
	}
	
	// Check required permissions
	for _, perm := range am.config.RequiredPermissions {
		if !user.HasPermission(perm) {
			return fmt.Errorf("insufficient permissions")
		}
	}
	
	return nil
}

// Token extraction methods

func (am *AuthMiddleware) extractJWTToken(r *http.Request) string {
	// Try Authorization header
	authHeader := r.Header.Get(am.config.JWT.TokenHeader)
	if strings.HasPrefix(authHeader, am.config.JWT.TokenPrefix) {
		return strings.TrimPrefix(authHeader, am.config.JWT.TokenPrefix)
	}
	
	// Try query parameter
	if am.config.JWT.QueryParam != "" {
		if token := r.URL.Query().Get(am.config.JWT.QueryParam); token != "" {
			return token
		}
	}
	
	// Try cookie
	if am.config.JWT.CookieName != "" {
		if cookie, err := r.Cookie(am.config.JWT.CookieName); err == nil {
			return cookie.Value
		}
	}
	
	return ""
}

func (am *AuthMiddleware) extractAPIKey(r *http.Request) string {
	// Try configured header
	if key := r.Header.Get(am.config.APIKey.HeaderName); key != "" {
		// Remove prefix if configured
		if am.config.APIKey.Prefix != "" {
			return strings.TrimPrefix(key, am.config.APIKey.Prefix)
		}
		return key
	}
	
	// Try query parameter
	if am.config.APIKey.QueryParam != "" {
		if key := r.URL.Query().Get(am.config.APIKey.QueryParam); key != "" {
			return key
		}
	}
	
	return ""
}

func (am *AuthMiddleware) extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get(am.config.Bearer.HeaderName)
	if strings.HasPrefix(authHeader, am.config.Bearer.Prefix) {
		return strings.TrimPrefix(authHeader, am.config.Bearer.Prefix)
	}
	return ""
}

// HMAC utility methods

func (am *AuthMiddleware) buildSignatureString(r *http.Request, timestamp, nonce string) string {
	var parts []string
	
	// Add method and path
	parts = append(parts, r.Method, r.URL.Path)
	
	// Add timestamp
	parts = append(parts, timestamp)
	
	// Add nonce if present
	if nonce != "" {
		parts = append(parts, nonce)
	}
	
	// Add included headers
	for _, headerName := range am.config.HMAC.IncludeHeaders {
		if value := r.Header.Get(headerName); value != "" {
			parts = append(parts, headerName+":"+value)
		}
	}
	
	// Add body hash for POST/PUT requests
	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
		// In a real implementation, you'd read and hash the body
		// This is simplified for demonstration
		parts = append(parts, "body-hash:placeholder")
	}
	
	return strings.Join(parts, "\n")
}

func (am *AuthMiddleware) calculateHMAC(data string) string {
	if am.config.HMAC.secretKey == nil {
		am.logger.Error("HMAC secret key not loaded")
		return ""
	}

	var mac hash.Hash
	
	switch am.config.HMAC.Algorithm {
	case "sha256":
		mac = hmac.New(sha256.New, am.config.HMAC.secretKey.Bytes())
	default:
		mac = hmac.New(sha256.New, am.config.HMAC.secretKey.Bytes())
	}
	
	am.auditor.AuditCredentialAccess("hmac_secret", "signing", true)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func (am *AuthMiddleware) constantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Path checking methods

func (am *AuthMiddleware) isExcludedPath(path string) bool {
	for _, excludedPath := range am.config.ExcludedPaths {
		if path == excludedPath || strings.HasPrefix(path, excludedPath) {
			return true
		}
	}
	return false
}

func (am *AuthMiddleware) isOptionalPath(path string) bool {
	for _, optionalPath := range am.config.OptionalPaths {
		if path == optionalPath || strings.HasPrefix(path, optionalPath) {
			return true
		}
	}
	return false
}

// Error handling

func (am *AuthMiddleware) handleAuthError(w http.ResponseWriter, r *http.Request, err error) {
	statusCode := http.StatusUnauthorized
	message := "Authentication required"
	
	if !am.config.SecureErrorMode {
		message = err.Error()
	}
	
	// Set appropriate authentication challenge header
	switch am.config.Method {
	case AuthMethodBasic:
		w.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=\"%s\"", am.config.BasicAuth.Realm))
	case AuthMethodBearer, AuthMethodJWT:
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	
	am.logger.Debug("Authentication failed",
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.String("remote_addr", r.RemoteAddr),
		zap.Error(err),
	)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := map[string]interface{}{
		"error":     message,
		"timestamp": time.Now().Unix(),
	}
	
	json.NewEncoder(w).Encode(response)
}

// Utility functions

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// GetAuthUser extracts the authenticated user from the request context
func GetAuthUser(ctx context.Context) (*AuthUser, bool) {
	user, ok := ctx.Value(AuthContextKey).(*AuthUser)
	return user, ok
}

// RequireAuth is a helper function to create auth middleware with default config
func RequireAuth(method AuthMethod, logger *zap.Logger) (*AuthMiddleware, error) {
	config := &AuthConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Priority: 100,
			Name:     "auth",
		},
		Method:          method,
		SecureErrorMode: true,
	}
	
	return NewAuthMiddleware(config, logger)
}

// CreateAPIKeyHash creates a bcrypt hash for API key storage using secure defaults
func CreateAPIKeyHash(key string) (string, error) {
	return CreateAPIKeyHashWithCost(key, BCryptDefaultCost)
}

// CreateAPIKeyHashWithCost creates a bcrypt hash for API key storage with specified cost
func CreateAPIKeyHashWithCost(key string, cost int) (string, error) {
	if key == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}
	
	// Validate cost parameter
	if err := ValidateBCryptCost(cost); err != nil {
		return "", fmt.Errorf("invalid bcrypt cost for API key hashing: %w", err)
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(key), cost)
	if err != nil {
		return "", fmt.Errorf("failed to generate bcrypt hash: %w", err)
	}
	return string(hash), nil
}

// GenerateAPIKey generates a random API key
func GenerateAPIKey() string {
	// Generate 32 random bytes and encode as base64
	bytes := make([]byte, 32)
	// In a real implementation, use crypto/rand
	for i := range bytes {
		bytes[i] = byte(time.Now().UnixNano() % 256)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

// Password hashing utilities with BCrypt cost validation

// HashPassword creates a secure bcrypt hash of a password using secure defaults
func HashPassword(password string) (string, error) {
	return HashPasswordWithCost(password, BCryptDefaultCost)
}

// HashPasswordWithCost creates a secure bcrypt hash of a password with specified cost
func HashPasswordWithCost(password string, cost int) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}
	
	// Validate cost parameter
	if err := ValidateBCryptCost(cost); err != nil {
		return "", fmt.Errorf("invalid bcrypt cost for password hashing: %w", err)
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("failed to generate bcrypt hash: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword verifies a password against its bcrypt hash
func VerifyPassword(password, hash string) error {
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}
	if hash == "" {
		return fmt.Errorf("hash cannot be empty")
	}
	
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return fmt.Errorf("password verification failed: %w", err)
	}
	return nil
}

// GetBCryptCostFromHash extracts the cost parameter from a bcrypt hash
func GetBCryptCostFromHash(hash string) (int, error) {
	if hash == "" {
		return 0, fmt.Errorf("hash cannot be empty")
	}
	
	// BCrypt hash format: $2a$cost$salt+hash
	// Extract cost from hash
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return 0, fmt.Errorf("failed to extract cost from bcrypt hash: %w", err)
	}
	
	return cost, nil
}

// ValidatePasswordHash validates that a password hash meets security requirements
func ValidatePasswordHash(hash string) error {
	if hash == "" {
		return fmt.Errorf("password hash cannot be empty")
	}
	
	// Extract and validate cost
	cost, err := GetBCryptCostFromHash(hash)
	if err != nil {
		return fmt.Errorf("invalid bcrypt hash format: %w", err)
	}
	
	// Validate cost meets security requirements
	if err := ValidateBCryptCost(cost); err != nil {
		return fmt.Errorf("password hash uses insecure bcrypt cost: %w", err)
	}
	
	return nil
}

// ValidateAuthConfig performs comprehensive validation of auth configuration for security compliance
func ValidateAuthConfig(config *AuthConfig) error {
	if config == nil {
		return fmt.Errorf("auth config cannot be nil")
	}
	
	// Validate global BCrypt cost
	if config.BCryptCost != 0 {
		if err := ValidateBCryptCost(config.BCryptCost); err != nil {
			return fmt.Errorf("invalid global bcrypt cost: %w", err)
		}
	}
	
	// Method-specific validation
	switch config.Method {
	case AuthMethodBasic:
		// Validate Basic Auth BCrypt cost
		if config.BasicAuth.BCryptCost != 0 {
			if err := ValidateBCryptCost(config.BasicAuth.BCryptCost); err != nil {
				return fmt.Errorf("invalid basic auth bcrypt cost: %w", err)
			}
		}
		
		// Count users with weak password hashes
		weakHashCount := 0
		for username, user := range config.BasicAuth.Users {
			if user.PasswordHash != "" {
				if err := ValidatePasswordHash(user.PasswordHash); err != nil {
					weakHashCount++
					// Log individual issues but continue counting
					fmt.Printf("WARNING: User '%s' has weak password hash: %v\n", username, err)
				}
			}
		}
		
		// If all users have weak hashes and there are users, that's a critical security issue
		if weakHashCount > 0 && weakHashCount == len(config.BasicAuth.Users) && len(config.BasicAuth.Users) > 0 {
			return fmt.Errorf("all %d basic auth users have insecure password hashes below minimum cost %d", weakHashCount, BCryptMinCost)
		}
	}
	
	return nil
}

// GetEffectiveBCryptCost returns the effective BCrypt cost for a given configuration
func GetEffectiveBCryptCost(config *AuthConfig, method AuthMethod) int {
	switch method {
	case AuthMethodBasic:
		if config.BasicAuth.BCryptCost > 0 {
			return config.BasicAuth.BCryptCost
		}
	}
	
	if config.BCryptCost > 0 {
		return config.BCryptCost
	}
	
	return BCryptDefaultCost
}