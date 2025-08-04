package client

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// JWTManager handles JWT token generation, validation, and refresh
type JWTManager struct {
	config          *JWTConfig
	logger          *zap.Logger
	signingKey      interface{}
	verifyingKey    interface{}
	blacklistedTokens map[string]time.Time
	mu              sync.RWMutex
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(config *JWTConfig, logger *zap.Logger) (*JWTManager, error) {
	if config == nil {
		return nil, fmt.Errorf("JWT config cannot be nil")
	}
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	jm := &JWTManager{
		config:            config,
		logger:            logger,
		blacklistedTokens: make(map[string]time.Time),
	}
	
	// Load signing and verifying keys
	if err := jm.loadKeys(); err != nil {
		return nil, fmt.Errorf("failed to load JWT keys: %w", err)
	}
	
	// Start cleanup goroutine for blacklisted tokens
	go jm.cleanupBlacklistedTokens()
	
	return jm, nil
}

// loadKeys loads the signing and verifying keys based on the signing method
func (jm *JWTManager) loadKeys() error {
	switch jm.config.SigningMethod {
	case "HS256", "HS384", "HS512":
		// HMAC signing - use secret key
		if jm.config.SecretKey == "" {
			return fmt.Errorf("secret key is required for HMAC signing")
		}
		jm.signingKey = []byte(jm.config.SecretKey)
		jm.verifyingKey = []byte(jm.config.SecretKey)
		
	case "RS256", "RS384", "RS512":
		// RSA signing - load private and public keys
		if err := jm.loadRSAKeys(); err != nil {
			return fmt.Errorf("failed to load RSA keys: %w", err)
		}
		
	case "ES256", "ES384", "ES512":
		// ECDSA signing - load private and public keys
		return fmt.Errorf("ECDSA signing not yet implemented")
		
	default:
		return fmt.Errorf("unsupported signing method: %s", jm.config.SigningMethod)
	}
	
	return nil
}

// loadRSAKeys loads RSA private and public keys
func (jm *JWTManager) loadRSAKeys() error {
	// Load private key for signing
	if jm.config.PrivateKeyPath != "" {
		privateKeyData, err := os.ReadFile(jm.config.PrivateKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read private key file: %w", err)
		}
		
		block, _ := pem.Decode(privateKeyData)
		if block == nil {
			return fmt.Errorf("failed to decode PEM block from private key")
		}
		
		privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			// Try PKCS8 format
			pkcs8Key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err2 != nil {
				return fmt.Errorf("failed to parse private key: %v, %v", err, err2)
			}
			
			var ok bool
			privateKey, ok = pkcs8Key.(*rsa.PrivateKey)
			if !ok {
				return fmt.Errorf("private key is not RSA")
			}
		}
		
		jm.signingKey = privateKey
		jm.verifyingKey = &privateKey.PublicKey
	}
	
	// Load public key for verification (if different from private key)
	if jm.config.PublicKeyPath != "" && jm.config.PublicKeyPath != jm.config.PrivateKeyPath {
		publicKeyData, err := os.ReadFile(jm.config.PublicKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read public key file: %w", err)
		}
		
		block, _ := pem.Decode(publicKeyData)
		if block == nil {
			return fmt.Errorf("failed to decode PEM block from public key")
		}
		
		publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse public key: %w", err)
		}
		
		rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("public key is not RSA")
		}
		
		jm.verifyingKey = rsaPublicKey
	}
	
	if jm.signingKey == nil {
		return fmt.Errorf("no signing key loaded")
	}
	
	if jm.verifyingKey == nil {
		return fmt.Errorf("no verifying key loaded")
	}
	
	return nil
}

// GenerateToken generates a new JWT token
func (jm *JWTManager) GenerateToken(subject string, claims jwt.MapClaims) (string, error) {
	now := time.Now()
	
	// Create standard claims
	standardClaims := jwt.MapClaims{
		"iss": jm.config.Issuer,
		"sub": subject,
		"aud": jm.config.Audience,
		"exp": now.Add(jm.config.AccessTokenTTL).Unix(),
		"nbf": now.Unix(),
		"iat": now.Unix(),
		"jti": generateJTI(),
	}
	
	// Merge with custom claims
	for key, value := range claims {
		standardClaims[key] = value
	}
	
	// Merge with configured custom claims
	for key, value := range jm.config.CustomClaims {
		if _, exists := standardClaims[key]; !exists {
			standardClaims[key] = value
		}
	}
	
	// Create token
	token := jwt.NewWithClaims(jwt.GetSigningMethod(jm.config.SigningMethod), standardClaims)
	
	// Sign token
	tokenString, err := token.SignedString(jm.signingKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	
	jm.logger.Debug("Generated JWT token",
		zap.String("subject", subject),
		zap.String("jti", standardClaims["jti"].(string)),
		zap.Time("expires_at", time.Unix(standardClaims["exp"].(int64), 0)),
	)
	
	return tokenString, nil
}

// GenerateRefreshToken generates a refresh token
func (jm *JWTManager) GenerateRefreshToken(subject string, claims jwt.MapClaims) (string, error) {
	now := time.Now()
	
	// Create refresh token claims
	refreshClaims := jwt.MapClaims{
		"iss":  jm.config.Issuer,
		"sub":  subject,
		"aud":  jm.config.Audience,
		"exp":  now.Add(jm.config.RefreshTokenTTL).Unix(),
		"nbf":  now.Unix(),
		"iat":  now.Unix(),
		"jti":  generateJTI(),
		"type": "refresh",
	}
	
	// Add selected claims from original token
	allowedRefreshClaims := []string{"username", "email", "roles"}
	for _, key := range allowedRefreshClaims {
		if value, exists := claims[key]; exists {
			refreshClaims[key] = value
		}
	}
	
	// Create token
	token := jwt.NewWithClaims(jwt.GetSigningMethod(jm.config.SigningMethod), refreshClaims)
	
	// Sign token
	tokenString, err := token.SignedString(jm.signingKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}
	
	jm.logger.Debug("Generated refresh token",
		zap.String("subject", subject),
		zap.String("jti", refreshClaims["jti"].(string)),
		zap.Time("expires_at", time.Unix(refreshClaims["exp"].(int64), 0)),
	)
	
	return tokenString, nil
}

// ValidateToken validates and parses a JWT token
func (jm *JWTManager) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	// Check if token is blacklisted
	if jm.isTokenBlacklisted(tokenString) {
		return nil, fmt.Errorf("token is blacklisted")
	}
	
	// Parse and validate token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if token.Method.Alg() != jm.config.SigningMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		
		return jm.verifyingKey, nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}
	
	// Check if token is valid
	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}
	
	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to extract claims")
	}
	
	// Additional validation
	if err := jm.validateClaims(claims); err != nil {
		return nil, fmt.Errorf("claims validation failed: %w", err)
	}
	
	jm.logger.Debug("Validated JWT token",
		zap.String("subject", claims["sub"].(string)),
		zap.Any("jti", claims["jti"]),
	)
	
	return claims, nil
}

// ValidateRefreshToken validates a refresh token
func (jm *JWTManager) ValidateRefreshToken(tokenString string) (jwt.MapClaims, error) {
	claims, err := jm.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	
	// Check if it's a refresh token
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return nil, fmt.Errorf("not a refresh token")
	}
	
	return claims, nil
}

// validateClaims performs additional claims validation
func (jm *JWTManager) validateClaims(claims jwt.MapClaims) error {
	now := time.Now()
	
	// Validate expiration with leeway
	if exp, ok := claims["exp"].(float64); ok {
		expirationTime := time.Unix(int64(exp), 0)
		if now.After(expirationTime.Add(jm.config.LeewayTime)) {
			return fmt.Errorf("token has expired")
		}
	}
	
	// Validate not before with leeway
	if nbf, ok := claims["nbf"].(float64); ok {
		notBeforeTime := time.Unix(int64(nbf), 0)
		if now.Before(notBeforeTime.Add(-jm.config.LeewayTime)) {
			return fmt.Errorf("token is not yet valid")
		}
	}
	
	// Validate issuer
	if jm.config.Issuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != jm.config.Issuer {
			return fmt.Errorf("invalid issuer")
		}
	}
	
	// Validate audience
	if len(jm.config.Audience) > 0 {
		if aud, ok := claims["aud"].([]interface{}); ok {
			// Multiple audiences
			found := false
			for _, configAud := range jm.config.Audience {
				for _, tokenAud := range aud {
					if audStr, ok := tokenAud.(string); ok && audStr == configAud {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				return fmt.Errorf("invalid audience")
			}
		} else if audStr, ok := claims["aud"].(string); ok {
			// Single audience
			found := false
			for _, configAud := range jm.config.Audience {
				if audStr == configAud {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("invalid audience")
			}
		} else {
			return fmt.Errorf("missing audience claim")
		}
	}
	
	// Validate subject
	if jm.config.Subject != "" {
		if sub, ok := claims["sub"].(string); !ok || sub != jm.config.Subject {
			return fmt.Errorf("invalid subject")
		}
	}
	
	return nil
}

// RefreshAccessToken refreshes an access token using a refresh token
func (jm *JWTManager) RefreshAccessToken(refreshTokenString string) (string, error) {
	// Validate refresh token
	claims, err := jm.ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return "", fmt.Errorf("refresh token validation failed: %w", err)
	}
	
	// Generate new access token
	subject := claims["sub"].(string)
	newToken, err := jm.GenerateToken(subject, claims)
	if err != nil {
		return "", fmt.Errorf("failed to generate new access token: %w", err)
	}
	
	jm.logger.Info("Refreshed access token",
		zap.String("subject", subject),
	)
	
	return newToken, nil
}

// BlacklistToken adds a token to the blacklist
func (jm *JWTManager) BlacklistToken(tokenString string) error {
	// Parse token to get expiration
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jm.verifyingKey, nil
	})
	
	if err != nil {
		// Even if parsing fails, we should blacklist the token
		jm.mu.Lock()
		jm.blacklistedTokens[tokenString] = time.Now().Add(24 * time.Hour)
		jm.mu.Unlock()
		return nil
	}
	
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		jm.mu.Lock()
		jm.blacklistedTokens[tokenString] = time.Now().Add(24 * time.Hour)
		jm.mu.Unlock()
		return nil
	}
	
	// Blacklist until token expiration
	var expirationTime time.Time
	if exp, ok := claims["exp"].(float64); ok {
		expirationTime = time.Unix(int64(exp), 0)
	} else {
		expirationTime = time.Now().Add(24 * time.Hour)
	}
	
	jm.mu.Lock()
	jm.blacklistedTokens[tokenString] = expirationTime
	jm.mu.Unlock()
	
	jm.logger.Info("Blacklisted JWT token",
		zap.String("jti", fmt.Sprintf("%v", claims["jti"])),
		zap.Time("expires_at", expirationTime),
	)
	
	return nil
}

// isTokenBlacklisted checks if a token is blacklisted
func (jm *JWTManager) isTokenBlacklisted(tokenString string) bool {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	
	_, exists := jm.blacklistedTokens[tokenString]
	return exists
}

// cleanupBlacklistedTokens removes expired entries from the blacklist
func (jm *JWTManager) cleanupBlacklistedTokens() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		now := time.Now()
		
		jm.mu.Lock()
		for token, expirationTime := range jm.blacklistedTokens {
			if now.After(expirationTime) {
				delete(jm.blacklistedTokens, token)
			}
		}
		jm.mu.Unlock()
		
		jm.logger.Debug("Cleaned up expired blacklisted tokens")
	}
}

// GetTokenInfo extracts information from a token without validating it
func (jm *JWTManager) GetTokenInfo(tokenString string) (*TokenInfo, error) {
	// Parse token without validation
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}
	
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to extract claims")
	}
	
	tokenInfo := &TokenInfo{
		Token: tokenString,
	}
	
	// Extract standard claims
	if sub, ok := claims["sub"].(string); ok {
		tokenInfo.Subject = sub
	}
	
	if iat, ok := claims["iat"].(float64); ok {
		tokenInfo.IssuedAt = time.Unix(int64(iat), 0)
	}
	
	if exp, ok := claims["exp"].(float64); ok {
		tokenInfo.ExpiresAt = time.Unix(int64(exp), 0)
	}
	
	// Determine token type
	if tokenType, ok := claims["type"].(string); ok && tokenType == "refresh" {
		tokenInfo.TokenType = TokenTypeRefresh
	} else {
		tokenInfo.TokenType = TokenTypeAccess
	}
	
	// Extract scopes if present
	if scopes, ok := claims["scopes"].([]interface{}); ok {
		tokenInfo.Scopes = make([]string, len(scopes))
		for i, scope := range scopes {
			if scopeStr, ok := scope.(string); ok {
				tokenInfo.Scopes[i] = scopeStr
			}
		}
	}
	
	return tokenInfo, nil
}

// ShouldRefresh checks if a token should be refreshed based on configuration
func (jm *JWTManager) ShouldRefresh(tokenInfo *TokenInfo) bool {
	if !jm.config.AutoRefresh {
		return false
	}
	
	if tokenInfo.TokenType != TokenTypeAccess {
		return false
	}
	
	timeUntilExpiration := time.Until(tokenInfo.ExpiresAt)
	return timeUntilExpiration <= jm.config.RefreshThreshold
}

// Cleanup performs cleanup operations
func (jm *JWTManager) Cleanup() error {
	// Clear blacklisted tokens
	jm.mu.Lock()
	jm.blacklistedTokens = make(map[string]time.Time)
	jm.mu.Unlock()
	
	jm.logger.Info("JWT manager cleanup completed")
	return nil
}

// generateJTI generates a unique JWT ID
func generateJTI() string {
	return fmt.Sprintf("jwt_%d_%d", time.Now().UnixNano(), time.Now().Unix())
}