package auth

import (
	"context"
	"crypto/rsa"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTProvider implements JWT-based authentication
type JWTProvider struct {
	config     *JWTConfig
	cache      TokenCache
	auditor    AuditLogger
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
}

// NewJWTProvider creates a new JWT authentication provider
func NewJWTProvider(config *JWTConfig, cache TokenCache, auditor AuditLogger) (*JWTProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("JWT config is required")
	}

	provider := &JWTProvider{
		config:  config,
		cache:   cache,
		auditor: auditor,
	}

	// Parse RSA keys if provided
	if config.PublicKey != "" {
		publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(config.PublicKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse RSA public key: %w", err)
		}
		provider.publicKey = publicKey
	}

	if config.PrivateKey != "" {
		privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(config.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
		}
		provider.privateKey = privateKey
	}

	return provider, nil
}

// Name returns the provider name
func (j *JWTProvider) Name() string {
	return "jwt"
}

// Authenticate validates JWT token and returns authentication context
func (j *JWTProvider) Authenticate(ctx context.Context, credentials *Credentials) (*AuthContext, error) {
	if credentials == nil || credentials.Token == "" {
		return nil, fmt.Errorf("JWT token is required")
	}

	// Check cache first
	if j.cache != nil {
		if cached, err := j.cache.Get(ctx, credentials.Token); err == nil && cached != nil {
			return j.credentialsToAuthContext(cached), nil
		}
	}

	// Validate token
	claims, err := j.ValidateToken(ctx, credentials.Token)
	if err != nil {
		if j.auditor != nil {
			j.auditor.LogAuthFailure(ctx, fmt.Sprintf("JWT validation failed: %v", err), map[string]any{
				"token_prefix": j.tokenPrefix(credentials.Token),
			})
		}
		return nil, fmt.Errorf("JWT validation failed: %w", err)
	}

	// Create auth context
	authCtx := &AuthContext{
		UserID:     claims.Subject,
		Username:   claims.Subject,
		Roles:      claims.Roles,
		Claims:     claims.Custom,
		AuthMethod: "jwt",
		Timestamp:  time.Now(),
		ExpiresAt:  claims.ExpiresAt,
	}

	// Extract permissions from roles if available
	if permissions, ok := claims.Custom["permissions"].([]string); ok {
		authCtx.Permissions = permissions
	}

	// Cache the credentials
	if j.cache != nil {
		credentials.Claims = claims.Custom
		credentials.ExpiresAt = claims.ExpiresAt
		credentials.Subject = claims.Subject
		credentials.Issuer = claims.Issuer
		credentials.Audience = claims.Audience

		ttl := time.Until(claims.ExpiresAt)
		if ttl > 0 {
			_ = j.cache.Set(ctx, credentials.Token, credentials, ttl)
		}
	}

	// Log successful authentication
	if j.auditor != nil {
		j.auditor.LogAuthSuccess(ctx, authCtx, map[string]any{
			"token_prefix": j.tokenPrefix(credentials.Token),
			"issuer":       claims.Issuer,
			"audience":     claims.Audience,
		})
	}

	return authCtx, nil
}

// Validate validates an existing authentication context
func (j *JWTProvider) Validate(ctx context.Context, authCtx *AuthContext) error {
	if authCtx == nil {
		return fmt.Errorf("authentication context is required")
	}

	if authCtx.AuthMethod != "jwt" {
		return fmt.Errorf("invalid authentication method: %s", authCtx.AuthMethod)
	}

	// Check expiration
	if !authCtx.ExpiresAt.IsZero() && time.Now().After(authCtx.ExpiresAt) {
		return fmt.Errorf("authentication context has expired")
	}

	return nil
}

// Refresh attempts to refresh JWT token
func (j *JWTProvider) Refresh(ctx context.Context, credentials *Credentials) (*Credentials, error) {
	if credentials == nil || credentials.Token == "" {
		return nil, fmt.Errorf("JWT token is required for refresh")
	}

	// Parse existing token without validation to get claims
	token, _, err := new(jwt.Parser).ParseUnverified(credentials.Token, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token for refresh: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Check if token is within refresh window
	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid expiration claim")
	}

	expTime := time.Unix(int64(exp), 0)
	refreshWindow := j.config.RefreshWindow
	if refreshWindow == 0 {
		refreshWindow = time.Hour // Default refresh window
	}

	if time.Until(expTime) > refreshWindow {
		return nil, fmt.Errorf("token not yet eligible for refresh")
	}

	// Create new token with extended expiration
	newClaims := jwt.MapClaims{}
	for k, v := range claims {
		newClaims[k] = v
	}

	newClaims["exp"] = time.Now().Add(j.config.Expiration).Unix()
	newClaims["iat"] = time.Now().Unix()

	var newToken string
	switch j.config.Algorithm {
	case "RS256", "RS384", "RS512":
		if j.privateKey == nil {
			return nil, fmt.Errorf("RSA private key required for signing")
		}

		method := jwt.GetSigningMethod(j.config.Algorithm)
		tokenObj := jwt.NewWithClaims(method, newClaims)
		newToken, err = tokenObj.SignedString(j.privateKey)
	default:
		if j.config.Secret == "" {
			return nil, fmt.Errorf("secret required for HMAC signing")
		}

		method := jwt.GetSigningMethod("HS256")
		if j.config.Algorithm != "" {
			method = jwt.GetSigningMethod(j.config.Algorithm)
		}

		tokenObj := jwt.NewWithClaims(method, newClaims)
		newToken, err = tokenObj.SignedString([]byte(j.config.Secret))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to sign new token: %w", err)
	}

	// Remove old token from cache
	if j.cache != nil {
		_ = j.cache.Delete(ctx, credentials.Token)
	}

	refreshedCredentials := &Credentials{
		Type:      "jwt",
		Token:     newToken,
		Claims:    newClaims,
		ExpiresAt: time.Unix(int64(newClaims["exp"].(float64)), 0),
		IssuedAt:  time.Now(),
		Subject:   fmt.Sprintf("%v", claims["sub"]),
		Issuer:    fmt.Sprintf("%v", claims["iss"]),
	}

	// Log token refresh
	if j.auditor != nil {
		authCtx := &AuthContext{
			UserID:     refreshedCredentials.Subject,
			AuthMethod: "jwt",
			Timestamp:  time.Now(),
		}
		j.auditor.LogTokenRefresh(ctx, authCtx)
	}

	return refreshedCredentials, nil
}

// Revoke revokes JWT token by removing it from cache
func (j *JWTProvider) Revoke(ctx context.Context, credentials *Credentials) error {
	if credentials == nil || credentials.Token == "" {
		return fmt.Errorf("JWT token is required")
	}

	if j.cache != nil {
		return j.cache.Delete(ctx, credentials.Token)
	}

	return nil
}

// SupportedTypes returns supported credential types
func (j *JWTProvider) SupportedTypes() []string {
	return []string{"jwt", "bearer"}
}

// ValidateToken validates a JWT token and returns claims
func (j *JWTProvider) ValidateToken(ctx context.Context, tokenString string) (*TokenClaims, error) {
	var keyFunc jwt.Keyfunc

	switch j.config.Algorithm {
	case "RS256", "RS384", "RS512":
		if j.publicKey == nil {
			return nil, fmt.Errorf("RSA public key required for validation")
		}
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return j.publicKey, nil
		}
	default:
		if j.config.Secret == "" {
			return nil, fmt.Errorf("secret required for HMAC validation")
		}
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(j.config.Secret), nil
		}
	}

	token, err := jwt.Parse(tokenString, keyFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate standard claims
	if j.config.ClaimsValidation {
		if err := j.validateStandardClaims(claims); err != nil {
			return nil, fmt.Errorf("claims validation failed: %w", err)
		}
	}

	// Convert to TokenClaims
	tokenClaims := &TokenClaims{
		Custom: make(map[string]any),
	}

	if sub, ok := claims["sub"].(string); ok {
		tokenClaims.Subject = sub
	}

	if iss, ok := claims["iss"].(string); ok {
		tokenClaims.Issuer = iss
	}

	if aud, ok := claims["aud"].([]interface{}); ok {
		tokenClaims.Audience = make([]string, len(aud))
		for i, a := range aud {
			if audStr, ok := a.(string); ok {
				tokenClaims.Audience[i] = audStr
			}
		}
	}

	if exp, ok := claims["exp"].(float64); ok {
		tokenClaims.ExpiresAt = time.Unix(int64(exp), 0)
	}

	if iat, ok := claims["iat"].(float64); ok {
		tokenClaims.IssuedAt = time.Unix(int64(iat), 0)
	}

	if nbf, ok := claims["nbf"].(float64); ok {
		tokenClaims.NotBefore = time.Unix(int64(nbf), 0)
	}

	if jti, ok := claims["jti"].(string); ok {
		tokenClaims.JTI = jti
	}

	if scopes, ok := claims["scopes"].([]interface{}); ok {
		tokenClaims.Scopes = make([]string, len(scopes))
		for i, s := range scopes {
			if scopeStr, ok := s.(string); ok {
				tokenClaims.Scopes[i] = scopeStr
			}
		}
	}

	if roles, ok := claims["roles"].([]interface{}); ok {
		tokenClaims.Roles = make([]string, len(roles))
		for i, r := range roles {
			if roleStr, ok := r.(string); ok {
				tokenClaims.Roles[i] = roleStr
			}
		}
	}

	// Copy custom claims
	standardClaims := map[string]bool{
		"sub": true, "iss": true, "aud": true, "exp": true,
		"iat": true, "nbf": true, "jti": true, "scopes": true, "roles": true,
	}

	for k, v := range claims {
		if !standardClaims[k] {
			tokenClaims.Custom[k] = v
		}
	}

	return tokenClaims, nil
}

// RefreshToken refreshes a JWT token
func (j *JWTProvider) RefreshToken(ctx context.Context, token string) (string, error) {
	credentials := &Credentials{
		Type:  "jwt",
		Token: token,
	}

	refreshed, err := j.Refresh(ctx, credentials)
	if err != nil {
		return "", err
	}

	return refreshed.Token, nil
}

// RevokeToken revokes a JWT token
func (j *JWTProvider) RevokeToken(ctx context.Context, token string) error {
	credentials := &Credentials{
		Type:  "jwt",
		Token: token,
	}

	return j.Revoke(ctx, credentials)
}

// validateStandardClaims validates standard JWT claims
func (j *JWTProvider) validateStandardClaims(claims jwt.MapClaims) error {
	now := time.Now()

	// Validate issuer
	if j.config.Issuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != j.config.Issuer {
			return fmt.Errorf("invalid issuer")
		}
	}

	// Validate audience
	if len(j.config.Audience) > 0 {
		aud, ok := claims["aud"].([]interface{})
		if !ok {
			return fmt.Errorf("audience claim missing or invalid")
		}

		audMap := make(map[string]bool)
		for _, a := range aud {
			if audStr, ok := a.(string); ok {
				audMap[audStr] = true
			}
		}

		found := false
		for _, expectedAud := range j.config.Audience {
			if audMap[expectedAud] {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("token audience does not match expected audience")
		}
	}

	// Validate not before
	if nbf, ok := claims["nbf"].(float64); ok {
		if now.Before(time.Unix(int64(nbf), 0)) {
			return fmt.Errorf("token not yet valid")
		}
	}

	return nil
}

// credentialsToAuthContext converts credentials to auth context
func (j *JWTProvider) credentialsToAuthContext(credentials *Credentials) *AuthContext {
	authCtx := &AuthContext{
		UserID:     credentials.Subject,
		Username:   credentials.Subject,
		Claims:     credentials.Claims,
		AuthMethod: "jwt",
		Timestamp:  time.Now(),
		ExpiresAt:  credentials.ExpiresAt,
	}

	if roles, ok := credentials.Claims["roles"].([]string); ok {
		authCtx.Roles = roles
	}

	if permissions, ok := credentials.Claims["permissions"].([]string); ok {
		authCtx.Permissions = permissions
	}

	return authCtx
}

// tokenPrefix returns the first few characters of token for logging
func (j *JWTProvider) tokenPrefix(token string) string {
	if len(token) > 16 {
		return token[:16] + "..."
	}
	return token
}

// JWTMiddleware implements JWT authentication middleware
type JWTMiddleware struct {
	provider  *JWTProvider
	extractor CredentialExtractor
	enabled   bool
	priority  int
}

// NewJWTMiddleware creates new JWT authentication middleware
func NewJWTMiddleware(config *JWTConfig, cache TokenCache, auditor AuditLogger) (*JWTMiddleware, error) {
	provider, err := NewJWTProvider(config, cache, auditor)
	if err != nil {
		return nil, err
	}

	return &JWTMiddleware{
		provider:  provider,
		extractor: NewBearerTokenExtractor(),
		enabled:   true,
		priority:  100, // High priority for authentication
	}, nil
}

// Name returns middleware name
func (j *JWTMiddleware) Name() string {
	return "jwt_auth"
}

// Process processes the request through JWT authentication
func (j *JWTMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Extract credentials
	credentials, err := j.extractor.Extract(ctx, req.Headers, req.Body)
	if err != nil || credentials == nil {
		return &Response{
			ID:         req.ID,
			StatusCode: 401,
			Headers:    map[string]string{"WWW-Authenticate": "Bearer"},
			Error:      fmt.Errorf("authentication required: %w", err),
			Timestamp:  time.Now(),
		}, nil
	}

	// Authenticate
	authCtx, err := j.provider.Authenticate(ctx, credentials)
	if err != nil {
		return &Response{
			ID:         req.ID,
			StatusCode: 401,
			Headers:    map[string]string{"WWW-Authenticate": "Bearer"},
			Error:      fmt.Errorf("authentication failed: %w", err),
			Timestamp:  time.Now(),
		}, nil
	}

	// Add auth context to request metadata
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}
	req.Metadata["auth_context"] = authCtx

	// Continue to next middleware
	return next(ctx, req)
}

// Configure configures the middleware
func (j *JWTMiddleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		j.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		j.priority = priority
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (j *JWTMiddleware) Enabled() bool {
	return j.enabled
}

// Priority returns the middleware priority
func (j *JWTMiddleware) Priority() int {
	return j.priority
}

// BearerTokenExtractor extracts bearer tokens from Authorization header
type BearerTokenExtractor struct{}

// NewBearerTokenExtractor creates a new bearer token extractor
func NewBearerTokenExtractor() *BearerTokenExtractor {
	return &BearerTokenExtractor{}
}

// Extract extracts bearer token from Authorization header
func (b *BearerTokenExtractor) Extract(ctx context.Context, headers map[string]string, body any) (*Credentials, error) {
	authHeader := ""

	// Look for Authorization header (case-insensitive)
	for k, v := range headers {
		if strings.ToLower(k) == "authorization" {
			authHeader = v
			break
		}
	}

	if authHeader == "" {
		return nil, fmt.Errorf("authorization header not found")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	return &Credentials{
		Type:  "jwt",
		Token: parts[1],
	}, nil
}

// SupportedMethods returns supported authentication methods
func (b *BearerTokenExtractor) SupportedMethods() []string {
	return []string{"bearer", "jwt"}
}
