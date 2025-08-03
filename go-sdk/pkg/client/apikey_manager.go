package client

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// APIKeyManager handles API key validation and management
type APIKeyManager struct {
	config      *APIKeyConfig
	logger      *zap.Logger
	apiKeys     map[string]*APIKeyInfo
	mu          sync.RWMutex
	rotationTicker *time.Ticker
	stopRotation   chan bool
}

// APIKeyInfo contains information about an API key
type APIKeyInfo struct {
	ID          string                 `json:"id"`
	Key         string                 `json:"key"`
	HashedKey   string                 `json:"hashed_key"`
	UserID      string                 `json:"user_id"`
	Username    string                 `json:"username"`
	Email       string                 `json:"email"`
	Roles       []string               `json:"roles"`
	Permissions []string               `json:"permissions"`
	Scopes      []string               `json:"scopes"`
	CreatedAt   time.Time              `json:"created_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time             `json:"last_used_at,omitempty"`
	IsActive    bool                   `json:"is_active"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	
	// Usage tracking
	UsageCount  int64     `json:"usage_count"`
	RateLimit   int       `json:"rate_limit,omitempty"`   // requests per minute
	QuotaLimit  int64     `json:"quota_limit,omitempty"`  // total requests allowed
	QuotaUsed   int64     `json:"quota_used"`
	QuotaReset  time.Time `json:"quota_reset,omitempty"`
}

// APIKeyStore represents stored API keys data
type APIKeyStore struct {
	Keys        map[string]*APIKeyInfo `json:"keys"`
	LastUpdated time.Time              `json:"last_updated"`
	Version     string                 `json:"version"`
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager(config *APIKeyConfig, logger *zap.Logger) (*APIKeyManager, error) {
	if config == nil {
		return nil, fmt.Errorf("API key config cannot be nil")
	}
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	akm := &APIKeyManager{
		config:       config,
		logger:       logger,
		apiKeys:      make(map[string]*APIKeyInfo),
		stopRotation: make(chan bool),
	}
	
	// Load existing API keys
	if err := akm.loadAPIKeys(); err != nil {
		return nil, fmt.Errorf("failed to load API keys: %w", err)
	}
	
	// Start key rotation if enabled
	if config.EnableKeyRotation && config.KeyRotationInterval > 0 {
		akm.startKeyRotation()
	}
	
	return akm, nil
}

// loadAPIKeys loads API keys from the configured file
func (akm *APIKeyManager) loadAPIKeys() error {
	if akm.config.KeysFile == "" {
		akm.logger.Info("No API keys file configured, starting with empty key store")
		return nil
	}
	
	// Check if file exists
	if _, err := os.Stat(akm.config.KeysFile); os.IsNotExist(err) {
		akm.logger.Info("API keys file does not exist, creating new store",
			zap.String("file", akm.config.KeysFile))
		return akm.saveAPIKeys()
	}
	
	// Read file
	data, err := os.ReadFile(akm.config.KeysFile)
	if err != nil {
		return fmt.Errorf("failed to read API keys file: %w", err)
	}
	
	// Parse JSON
	var store APIKeyStore
	if err := json.Unmarshal(data, &store); err != nil {
		return fmt.Errorf("failed to parse API keys file: %w", err)
	}
	
	// Load keys
	akm.mu.Lock()
	akm.apiKeys = store.Keys
	if akm.apiKeys == nil {
		akm.apiKeys = make(map[string]*APIKeyInfo)
	}
	akm.mu.Unlock()
	
	akm.logger.Info("Loaded API keys",
		zap.Int("count", len(store.Keys)),
		zap.String("version", store.Version),
		zap.Time("last_updated", store.LastUpdated),
	)
	
	return nil
}

// saveAPIKeys saves API keys to the configured file
func (akm *APIKeyManager) saveAPIKeys() error {
	if akm.config.KeysFile == "" {
		return nil // No file configured, don't save
	}
	
	akm.mu.RLock()
	store := APIKeyStore{
		Keys:        akm.apiKeys,
		LastUpdated: time.Now(),
		Version:     "1.0",
	}
	akm.mu.RUnlock()
	
	// Marshal to JSON
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal API keys: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(akm.config.KeysFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write API keys file: %w", err)
	}
	
	akm.logger.Debug("Saved API keys to file",
		zap.String("file", akm.config.KeysFile),
		zap.Int("count", len(store.Keys)),
	)
	
	return nil
}

// ValidateAPIKey validates an API key and returns user information
func (akm *APIKeyManager) ValidateAPIKey(apiKey string) (*UserInfo, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key cannot be empty")
	}
	
	// Find matching key
	akm.mu.RLock()
	var keyInfo *APIKeyInfo
	var keyID string
	
	for id, info := range akm.apiKeys {
		if akm.compareAPIKey(apiKey, info) {
			keyInfo = info
			keyID = id
			break
		}
	}
	akm.mu.RUnlock()
	
	if keyInfo == nil {
		akm.logger.Warn("Invalid API key attempted",
			zap.String("key_prefix", akm.getKeyPrefix(apiKey)))
		return nil, fmt.Errorf("invalid API key")
	}
	
	// Check if key is active
	if !keyInfo.IsActive {
		akm.logger.Warn("Inactive API key attempted",
			zap.String("key_id", keyID),
			zap.String("user_id", keyInfo.UserID))
		return nil, fmt.Errorf("API key is inactive")
	}
	
	// Check expiration
	if keyInfo.ExpiresAt != nil && time.Now().After(*keyInfo.ExpiresAt) {
		akm.logger.Warn("Expired API key attempted",
			zap.String("key_id", keyID),
			zap.String("user_id", keyInfo.UserID),
			zap.Time("expired_at", *keyInfo.ExpiresAt))
		return nil, fmt.Errorf("API key has expired")
	}
	
	// Check quota
	if keyInfo.QuotaLimit > 0 && keyInfo.QuotaUsed >= keyInfo.QuotaLimit {
		// Check if quota has reset
		if !keyInfo.QuotaReset.IsZero() && time.Now().After(keyInfo.QuotaReset) {
			// Reset quota
			akm.mu.Lock()
			keyInfo.QuotaUsed = 0
			keyInfo.QuotaReset = time.Now().Add(24 * time.Hour) // Daily quota reset
			akm.mu.Unlock()
		} else {
			akm.logger.Warn("API key quota exceeded",
				zap.String("key_id", keyID),
				zap.String("user_id", keyInfo.UserID),
				zap.Int64("quota_used", keyInfo.QuotaUsed),
				zap.Int64("quota_limit", keyInfo.QuotaLimit))
			return nil, fmt.Errorf("API key quota exceeded")
		}
	}
	
	// Update usage tracking
	akm.updateKeyUsage(keyID, keyInfo)
	
	// Create user info
	userInfo := &UserInfo{
		ID:          keyInfo.UserID,
		Username:    keyInfo.Username,
		Email:       keyInfo.Email,
		Roles:       keyInfo.Roles,
		Permissions: keyInfo.Permissions,
		Metadata: map[string]interface{}{
			"api_key_id":     keyID,
			"api_key_scopes": keyInfo.Scopes,
			"usage_count":    keyInfo.UsageCount,
		},
	}
	
	// Add custom metadata
	for key, value := range keyInfo.Metadata {
		userInfo.Metadata[key] = value
	}
	
	akm.logger.Debug("API key validated successfully",
		zap.String("key_id", keyID),
		zap.String("user_id", keyInfo.UserID),
		zap.Int64("usage_count", keyInfo.UsageCount))
	
	return userInfo, nil
}

// compareAPIKey compares the provided API key with the stored key info
func (akm *APIKeyManager) compareAPIKey(providedKey string, keyInfo *APIKeyInfo) bool {
	switch akm.config.HashingAlgorithm {
	case "bcrypt":
		// Compare with bcrypt
		if keyInfo.HashedKey == "" {
			return false
		}
		err := bcrypt.CompareHashAndPassword([]byte(keyInfo.HashedKey), []byte(providedKey))
		return err == nil
		
	case "sha256":
		// Compare with SHA256
		if keyInfo.HashedKey == "" {
			return false
		}
		hasher := sha256.New()
		hasher.Write([]byte(providedKey))
		providedHash := hex.EncodeToString(hasher.Sum(nil))
		return subtle.ConstantTimeCompare([]byte(providedHash), []byte(keyInfo.HashedKey)) == 1
		
	case "plain":
		// Direct comparison (not recommended for production)
		return subtle.ConstantTimeCompare([]byte(providedKey), []byte(keyInfo.Key)) == 1
		
	default:
		// Default to plain comparison
		return subtle.ConstantTimeCompare([]byte(providedKey), []byte(keyInfo.Key)) == 1
	}
}

// updateKeyUsage updates the usage statistics for an API key
func (akm *APIKeyManager) updateKeyUsage(keyID string, keyInfo *APIKeyInfo) {
	akm.mu.Lock()
	defer akm.mu.Unlock()
	
	now := time.Now()
	keyInfo.LastUsedAt = &now
	keyInfo.UsageCount++
	keyInfo.QuotaUsed++
	
	// Save periodically (every 100 uses to avoid too frequent disk writes)
	if keyInfo.UsageCount%100 == 0 {
		go func() {
			if err := akm.saveAPIKeys(); err != nil {
				akm.logger.Error("Failed to save API keys after usage update", zap.Error(err))
			}
		}()
	}
}

// CreateAPIKey creates a new API key
func (akm *APIKeyManager) CreateAPIKey(userID, username, email string, roles, permissions, scopes []string, expiresIn *time.Duration) (*APIKeyInfo, error) {
	// Generate random API key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	
	apiKey := hex.EncodeToString(keyBytes)
	keyID := generateKeyID()
	
	// Hash the key if required
	var hashedKey string
	
	switch akm.config.HashingAlgorithm {
	case "bcrypt":
		hashedBytes, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash API key with bcrypt: %w", err)
		}
		hashedKey = string(hashedBytes)
		
	case "sha256":
		hasher := sha256.New()
		hasher.Write([]byte(apiKey))
		hashedKey = hex.EncodeToString(hasher.Sum(nil))
		
	case "plain":
		// No hashing for plain storage (not recommended)
		hashedKey = ""
		
	default:
		hashedKey = ""
	}
	
	// Create key info
	now := time.Now()
	keyInfo := &APIKeyInfo{
		ID:          keyID,
		Key:         apiKey,
		HashedKey:   hashedKey,
		UserID:      userID,
		Username:    username,
		Email:       email,
		Roles:       roles,
		Permissions: permissions,
		Scopes:      scopes,
		CreatedAt:   now,
		IsActive:    true,
		UsageCount:  0,
		QuotaUsed:   0,
		Metadata:    make(map[string]interface{}),
	}
	
	// Set expiration if provided
	if expiresIn != nil {
		expiresAt := now.Add(*expiresIn)
		keyInfo.ExpiresAt = &expiresAt
	}
	
	// Set quota reset time
	keyInfo.QuotaReset = now.Add(24 * time.Hour)
	
	// Store key
	akm.mu.Lock()
	akm.apiKeys[keyID] = keyInfo
	akm.mu.Unlock()
	
	// Save to file
	if err := akm.saveAPIKeys(); err != nil {
		akm.logger.Error("Failed to save API keys after creation", zap.Error(err))
	}
	
	akm.logger.Info("Created new API key",
		zap.String("key_id", keyID),
		zap.String("user_id", userID),
		zap.String("username", username),
		zap.Strings("roles", roles),
		zap.Strings("scopes", scopes))
	
	return keyInfo, nil
}

// RevokeAPIKey revokes an API key
func (akm *APIKeyManager) RevokeAPIKey(keyID string) error {
	akm.mu.Lock()
	defer akm.mu.Unlock()
	
	keyInfo, exists := akm.apiKeys[keyID]
	if !exists {
		return fmt.Errorf("API key not found: %s", keyID)
	}
	
	// Mark as inactive instead of deleting to preserve audit trail
	keyInfo.IsActive = false
	
	// Save to file
	if err := akm.saveAPIKeys(); err != nil {
		akm.logger.Error("Failed to save API keys after revocation", zap.Error(err))
	}
	
	akm.logger.Info("Revoked API key",
		zap.String("key_id", keyID),
		zap.String("user_id", keyInfo.UserID))
	
	return nil
}

// UpdateAPIKey updates an existing API key
func (akm *APIKeyManager) UpdateAPIKey(keyID string, updates map[string]interface{}) error {
	akm.mu.Lock()
	defer akm.mu.Unlock()
	
	keyInfo, exists := akm.apiKeys[keyID]
	if !exists {
		return fmt.Errorf("API key not found: %s", keyID)
	}
	
	// Apply updates
	for field, value := range updates {
		switch field {
		case "is_active":
			if active, ok := value.(bool); ok {
				keyInfo.IsActive = active
			}
		case "roles":
			if roles, ok := value.([]string); ok {
				keyInfo.Roles = roles
			}
		case "permissions":
			if permissions, ok := value.([]string); ok {
				keyInfo.Permissions = permissions
			}
		case "scopes":
			if scopes, ok := value.([]string); ok {
				keyInfo.Scopes = scopes
			}
		case "rate_limit":
			if rateLimit, ok := value.(int); ok {
				keyInfo.RateLimit = rateLimit
			}
		case "quota_limit":
			if quotaLimit, ok := value.(int64); ok {
				keyInfo.QuotaLimit = quotaLimit
			}
		case "expires_at":
			if expiresAt, ok := value.(time.Time); ok {
				keyInfo.ExpiresAt = &expiresAt
			}
		default:
			// Add to metadata
			if keyInfo.Metadata == nil {
				keyInfo.Metadata = make(map[string]interface{})
			}
			keyInfo.Metadata[field] = value
		}
	}
	
	// Save to file
	if err := akm.saveAPIKeys(); err != nil {
		akm.logger.Error("Failed to save API keys after update", zap.Error(err))
	}
	
	akm.logger.Info("Updated API key",
		zap.String("key_id", keyID),
		zap.String("user_id", keyInfo.UserID),
		zap.Any("updates", updates))
	
	return nil
}

// ListAPIKeys returns a list of API keys for a user
func (akm *APIKeyManager) ListAPIKeys(userID string) ([]*APIKeyInfo, error) {
	akm.mu.RLock()
	defer akm.mu.RUnlock()
	
	var userKeys []*APIKeyInfo
	for _, keyInfo := range akm.apiKeys {
		if keyInfo.UserID == userID {
			// Create a copy without the actual key for security
			keyCopy := *keyInfo
			keyCopy.Key = "" // Don't expose the actual key
			userKeys = append(userKeys, &keyCopy)
		}
	}
	
	return userKeys, nil
}

// GetAPIKeyInfo returns information about an API key
func (akm *APIKeyManager) GetAPIKeyInfo(keyID string) (*APIKeyInfo, error) {
	akm.mu.RLock()
	defer akm.mu.RUnlock()
	
	keyInfo, exists := akm.apiKeys[keyID]
	if !exists {
		return nil, fmt.Errorf("API key not found: %s", keyID)
	}
	
	// Create a copy without the actual key for security
	keyCopy := *keyInfo
	keyCopy.Key = "" // Don't expose the actual key
	
	return &keyCopy, nil
}

// startKeyRotation starts the automatic key rotation process
func (akm *APIKeyManager) startKeyRotation() {
	akm.rotationTicker = time.NewTicker(akm.config.KeyRotationInterval)
	
	go func() {
		for {
			select {
			case <-akm.rotationTicker.C:
				akm.performKeyRotation()
			case <-akm.stopRotation:
				akm.rotationTicker.Stop()
				return
			}
		}
	}()
	
	akm.logger.Info("Started API key rotation",
		zap.Duration("interval", akm.config.KeyRotationInterval))
}

// performKeyRotation performs automatic key rotation for expired keys
func (akm *APIKeyManager) performKeyRotation() {
	akm.mu.Lock()
	defer akm.mu.Unlock()
	
	now := time.Now()
	rotatedCount := 0
	
	for keyID, keyInfo := range akm.apiKeys {
		// Check if key should be rotated (close to expiration)
		if keyInfo.ExpiresAt != nil {
			timeUntilExpiration := keyInfo.ExpiresAt.Sub(now)
			if timeUntilExpiration <= 7*24*time.Hour && timeUntilExpiration > 0 {
				// Generate new key
				keyBytes := make([]byte, 32)
				if _, err := rand.Read(keyBytes); err != nil {
					akm.logger.Error("Failed to generate new key during rotation",
						zap.String("key_id", keyID),
						zap.Error(err))
					continue
				}
				
				newAPIKey := hex.EncodeToString(keyBytes)
				
				// Hash the new key if required
				var hashedKey string
				switch akm.config.HashingAlgorithm {
				case "bcrypt":
					hashedBytes, err := bcrypt.GenerateFromPassword([]byte(newAPIKey), bcrypt.DefaultCost)
					if err != nil {
						akm.logger.Error("Failed to hash new key during rotation",
							zap.String("key_id", keyID),
							zap.Error(err))
						continue
					}
					hashedKey = string(hashedBytes)
					
				case "sha256":
					hasher := sha256.New()
					hasher.Write([]byte(newAPIKey))
					hashedKey = hex.EncodeToString(hasher.Sum(nil))
				}
				
				// Update key info
				keyInfo.Key = newAPIKey
				keyInfo.HashedKey = hashedKey
				newExpiresAt := now.Add(akm.config.KeyRotationInterval)
				keyInfo.ExpiresAt = &newExpiresAt
				
				rotatedCount++
				
				akm.logger.Info("Rotated API key",
					zap.String("key_id", keyID),
					zap.String("user_id", keyInfo.UserID),
					zap.Time("new_expires_at", newExpiresAt))
			}
		}
	}
	
	if rotatedCount > 0 {
		// Save rotated keys
		go func() {
			if err := akm.saveAPIKeys(); err != nil {
				akm.logger.Error("Failed to save API keys after rotation", zap.Error(err))
			}
		}()
		
		akm.logger.Info("Completed key rotation",
			zap.Int("rotated_count", rotatedCount))
	}
}

// getKeyPrefix returns the first few characters of an API key for logging
func (akm *APIKeyManager) getKeyPrefix(apiKey string) string {
	if len(apiKey) <= 8 {
		return "***"
	}
	return apiKey[:8] + "***"
}

// Cleanup performs cleanup operations
func (akm *APIKeyManager) Cleanup() error {
	// Stop key rotation if running
	if akm.rotationTicker != nil {
		close(akm.stopRotation)
		akm.rotationTicker.Stop()
	}
	
	// Save any pending changes
	if err := akm.saveAPIKeys(); err != nil {
		return fmt.Errorf("failed to save API keys during cleanup: %w", err)
	}
	
	akm.logger.Info("API key manager cleanup completed")
	return nil
}

// generateKeyID generates a unique key ID
func generateKeyID() string {
	return fmt.Sprintf("ak_%d_%d", time.Now().UnixNano(), time.Now().Unix())
}