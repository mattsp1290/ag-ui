package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TokenStorage interface for storing and retrieving tokens
type TokenStorage interface {
	StoreToken(token string, info *TokenInfo) error
	GetToken(token string) (*TokenInfo, error)
	RevokeToken(token string) error
	ListTokens(userID string) ([]*TokenInfo, error)
	CleanupExpiredTokens() error
	Cleanup() error
}

// MemoryTokenStorage stores tokens in memory
type MemoryTokenStorage struct {
	config    *TokenStorageConfig
	logger    *zap.Logger
	tokens    map[string]*TokenInfo
	userTokens map[string][]string // userID -> []tokenID
	mu        sync.RWMutex
	gcTicker  *time.Ticker
	stopGC    chan bool
}

// FileTokenStorage stores tokens in encrypted files
type FileTokenStorage struct {
	config     *TokenStorageConfig
	logger     *zap.Logger
	encryptor  *TokenEncryptor
	filePath   string
	mu         sync.RWMutex
}

// TokenEncryptor handles token encryption/decryption
type TokenEncryptor struct {
	key    []byte
	gcm    cipher.AEAD
	logger *zap.Logger
}

// EncryptedTokenData represents encrypted token data
type EncryptedTokenData struct {
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// TokenStore represents the file storage format
type TokenStore struct {
	Tokens      map[string]*EncryptedTokenData `json:"tokens"`
	UserTokens  map[string][]string            `json:"user_tokens"`
	LastUpdated time.Time                      `json:"last_updated"`
	Version     string                         `json:"version"`
}

// NewTokenStorage creates a new token storage based on configuration
func NewTokenStorage(config *TokenStorageConfig, logger *zap.Logger) (TokenStorage, error) {
	if config == nil {
		return nil, fmt.Errorf("token storage config cannot be nil")
	}
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	switch config.StorageType {
	case "memory":
		return NewMemoryTokenStorage(config, logger)
	case "file":
		return NewFileTokenStorage(config, logger)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.StorageType)
	}
}

// NewMemoryTokenStorage creates a new memory-based token storage
func NewMemoryTokenStorage(config *TokenStorageConfig, logger *zap.Logger) (*MemoryTokenStorage, error) {
	mts := &MemoryTokenStorage{
		config:     config,
		logger:     logger,
		tokens:     make(map[string]*TokenInfo),
		userTokens: make(map[string][]string),
		stopGC:     make(chan bool),
	}
	
	// Start garbage collection for expired tokens
	mts.startGarbageCollection()
	
	return mts, nil
}

// StoreToken stores a token in memory
func (mts *MemoryTokenStorage) StoreToken(token string, info *TokenInfo) error {
	if token == "" || info == nil {
		return fmt.Errorf("token and info cannot be empty/nil")
	}
	
	mts.mu.Lock()
	defer mts.mu.Unlock()
	
	// Store token
	mts.tokens[token] = info
	
	// Update user token mapping
	if info.Subject != "" {
		if mts.userTokens[info.Subject] == nil {
			mts.userTokens[info.Subject] = make([]string, 0)
		}
		mts.userTokens[info.Subject] = append(mts.userTokens[info.Subject], token)
	}
	
	mts.logger.Debug("Stored token in memory",
		zap.String("token_type", string(info.TokenType)),
		zap.String("subject", info.Subject),
		zap.Time("expires_at", info.ExpiresAt))
	
	return nil
}

// GetToken retrieves a token from memory
func (mts *MemoryTokenStorage) GetToken(token string) (*TokenInfo, error) {
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}
	
	mts.mu.RLock()
	defer mts.mu.RUnlock()
	
	info, exists := mts.tokens[token]
	if !exists {
		return nil, fmt.Errorf("token not found")
	}
	
	// Check if token is expired
	if time.Now().After(info.ExpiresAt) {
		return nil, fmt.Errorf("token has expired")
	}
	
	return info, nil
}

// RevokeToken removes a token from memory
func (mts *MemoryTokenStorage) RevokeToken(token string) error {
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}
	
	mts.mu.Lock()
	defer mts.mu.Unlock()
	
	info, exists := mts.tokens[token]
	if !exists {
		return fmt.Errorf("token not found")
	}
	
	// Remove from tokens map
	delete(mts.tokens, token)
	
	// Remove from user tokens mapping
	if info.Subject != "" {
		userTokens := mts.userTokens[info.Subject]
		for i, userToken := range userTokens {
			if userToken == token {
				mts.userTokens[info.Subject] = append(userTokens[:i], userTokens[i+1:]...)
				break
			}
		}
		
		// Clean up empty user token lists
		if len(mts.userTokens[info.Subject]) == 0 {
			delete(mts.userTokens, info.Subject)
		}
	}
	
	mts.logger.Debug("Revoked token from memory",
		zap.String("subject", info.Subject),
		zap.String("token_type", string(info.TokenType)))
	
	return nil
}

// ListTokens returns all tokens for a user
func (mts *MemoryTokenStorage) ListTokens(userID string) ([]*TokenInfo, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}
	
	mts.mu.RLock()
	defer mts.mu.RUnlock()
	
	var tokens []*TokenInfo
	userTokenList, exists := mts.userTokens[userID]
	if !exists {
		return tokens, nil // Return empty list if no tokens found
	}
	
	for _, token := range userTokenList {
		if info, exists := mts.tokens[token]; exists {
			// Only return non-expired tokens
			if time.Now().Before(info.ExpiresAt) {
				tokens = append(tokens, info)
			}
		}
	}
	
	return tokens, nil
}

// CleanupExpiredTokens removes expired tokens from memory
func (mts *MemoryTokenStorage) CleanupExpiredTokens() error {
	mts.mu.Lock()
	defer mts.mu.Unlock()
	
	now := time.Now()
	var expiredTokens []string
	
	// Find expired tokens
	for token, info := range mts.tokens {
		if now.After(info.ExpiresAt) {
			expiredTokens = append(expiredTokens, token)
		}
	}
	
	// Remove expired tokens
	for _, token := range expiredTokens {
		info := mts.tokens[token]
		delete(mts.tokens, token)
		
		// Remove from user tokens mapping
		if info.Subject != "" {
			userTokens := mts.userTokens[info.Subject]
			for i, userToken := range userTokens {
				if userToken == token {
					mts.userTokens[info.Subject] = append(userTokens[:i], userTokens[i+1:]...)
					break
				}
			}
			
			// Clean up empty user token lists
			if len(mts.userTokens[info.Subject]) == 0 {
				delete(mts.userTokens, info.Subject)
			}
		}
	}
	
	if len(expiredTokens) > 0 {
		mts.logger.Info("Cleaned up expired tokens",
			zap.Int("count", len(expiredTokens)))
	}
	
	return nil
}

// startGarbageCollection starts periodic cleanup of expired tokens
func (mts *MemoryTokenStorage) startGarbageCollection() {
	mts.gcTicker = time.NewTicker(time.Hour) // Run every hour
	
	go func() {
		for {
			select {
			case <-mts.gcTicker.C:
				if err := mts.CleanupExpiredTokens(); err != nil {
					mts.logger.Error("Failed to cleanup expired tokens", zap.Error(err))
				}
			case <-mts.stopGC:
				mts.gcTicker.Stop()
				return
			}
		}
	}()
}

// Cleanup performs cleanup operations for memory storage
func (mts *MemoryTokenStorage) Cleanup() error {
	// Stop garbage collection
	close(mts.stopGC)
	if mts.gcTicker != nil {
		mts.gcTicker.Stop()
	}
	
	// Clear all tokens
	mts.mu.Lock()
	mts.tokens = make(map[string]*TokenInfo)
	mts.userTokens = make(map[string][]string)
	mts.mu.Unlock()
	
	mts.logger.Info("Memory token storage cleanup completed")
	return nil
}

// NewFileTokenStorage creates a new file-based token storage
func NewFileTokenStorage(config *TokenStorageConfig, logger *zap.Logger) (*FileTokenStorage, error) {
	if config.FilePath == "" {
		return nil, fmt.Errorf("file path must be specified for file storage")
	}
	
	var encryptor *TokenEncryptor
	if config.Encryption.Enabled {
		var err error
		encryptor, err = NewTokenEncryptor(&config.Encryption, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create token encryptor: %w", err)
		}
	}
	
	fts := &FileTokenStorage{
		config:    config,
		logger:    logger,
		encryptor: encryptor,
		filePath:  config.FilePath,
	}
	
	return fts, nil
}

// StoreToken stores a token in file
func (fts *FileTokenStorage) StoreToken(token string, info *TokenInfo) error {
	if token == "" || info == nil {
		return fmt.Errorf("token and info cannot be empty/nil")
	}
	
	fts.mu.Lock()
	defer fts.mu.Unlock()
	
	// Load existing store
	store, err := fts.loadStore()
	if err != nil {
		return fmt.Errorf("failed to load token store: %w", err)
	}
	
	// Encrypt token info if encryption is enabled
	var tokenData *EncryptedTokenData
	if fts.encryptor != nil {
		tokenData, err = fts.encryptor.Encrypt(info)
		if err != nil {
			return fmt.Errorf("failed to encrypt token: %w", err)
		}
	} else {
		// Store as plaintext (not recommended for production)
		infoBytes, _ := json.Marshal(info)
		tokenData = &EncryptedTokenData{
			Ciphertext: infoBytes,
		}
	}
	
	// Store token
	store.Tokens[token] = tokenData
	
	// Update user token mapping
	if info.Subject != "" {
		if store.UserTokens[info.Subject] == nil {
			store.UserTokens[info.Subject] = make([]string, 0)
		}
		store.UserTokens[info.Subject] = append(store.UserTokens[info.Subject], token)
	}
	
	store.LastUpdated = time.Now()
	
	// Save store
	if err := fts.saveStore(store); err != nil {
		return fmt.Errorf("failed to save token store: %w", err)
	}
	
	fts.logger.Debug("Stored token in file",
		zap.String("token_type", string(info.TokenType)),
		zap.String("subject", info.Subject),
		zap.String("file", fts.filePath))
	
	return nil
}

// GetToken retrieves a token from file
func (fts *FileTokenStorage) GetToken(token string) (*TokenInfo, error) {
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}
	
	fts.mu.RLock()
	defer fts.mu.RUnlock()
	
	// Load store
	store, err := fts.loadStore()
	if err != nil {
		return nil, fmt.Errorf("failed to load token store: %w", err)
	}
	
	// Get encrypted token data
	tokenData, exists := store.Tokens[token]
	if !exists {
		return nil, fmt.Errorf("token not found")
	}
	
	// Decrypt token info
	var info *TokenInfo
	if fts.encryptor != nil {
		info, err = fts.encryptor.Decrypt(tokenData)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt token: %w", err)
		}
	} else {
		// Plaintext storage
		info = &TokenInfo{}
		if err := json.Unmarshal(tokenData.Ciphertext, info); err != nil {
			return nil, fmt.Errorf("failed to unmarshal token info: %w", err)
		}
	}
	
	// Check if token is expired
	if time.Now().After(info.ExpiresAt) {
		return nil, fmt.Errorf("token has expired")
	}
	
	return info, nil
}

// RevokeToken removes a token from file
func (fts *FileTokenStorage) RevokeToken(token string) error {
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}
	
	fts.mu.Lock()
	defer fts.mu.Unlock()
	
	// Load store
	store, err := fts.loadStore()
	if err != nil {
		return fmt.Errorf("failed to load token store: %w", err)
	}
	
	// Get token info for user mapping cleanup
	tokenData, exists := store.Tokens[token]
	if !exists {
		return fmt.Errorf("token not found")
	}
	
	var info *TokenInfo
	if fts.encryptor != nil {
		info, err = fts.encryptor.Decrypt(tokenData)
		if err != nil {
			return fmt.Errorf("failed to decrypt token: %w", err)
		}
	} else {
		info = &TokenInfo{}
		if err := json.Unmarshal(tokenData.Ciphertext, info); err != nil {
			return fmt.Errorf("failed to unmarshal token info: %w", err)
		}
	}
	
	// Remove token
	delete(store.Tokens, token)
	
	// Remove from user tokens mapping
	if info.Subject != "" {
		userTokens := store.UserTokens[info.Subject]
		for i, userToken := range userTokens {
			if userToken == token {
				store.UserTokens[info.Subject] = append(userTokens[:i], userTokens[i+1:]...)
				break
			}
		}
		
		// Clean up empty user token lists
		if len(store.UserTokens[info.Subject]) == 0 {
			delete(store.UserTokens, info.Subject)
		}
	}
	
	store.LastUpdated = time.Now()
	
	// Save store
	if err := fts.saveStore(store); err != nil {
		return fmt.Errorf("failed to save token store: %w", err)
	}
	
	fts.logger.Debug("Revoked token from file",
		zap.String("subject", info.Subject),
		zap.String("file", fts.filePath))
	
	return nil
}

// ListTokens returns all tokens for a user from file
func (fts *FileTokenStorage) ListTokens(userID string) ([]*TokenInfo, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}
	
	fts.mu.RLock()
	defer fts.mu.RUnlock()
	
	// Load store
	store, err := fts.loadStore()
	if err != nil {
		return nil, fmt.Errorf("failed to load token store: %w", err)
	}
	
	var tokens []*TokenInfo
	userTokenList, exists := store.UserTokens[userID]
	if !exists {
		return tokens, nil // Return empty list if no tokens found
	}
	
	for _, token := range userTokenList {
		if tokenData, exists := store.Tokens[token]; exists {
			var info *TokenInfo
			if fts.encryptor != nil {
				info, err = fts.encryptor.Decrypt(tokenData)
				if err != nil {
					fts.logger.Warn("Failed to decrypt token", zap.Error(err))
					continue
				}
			} else {
				info = &TokenInfo{}
				if err := json.Unmarshal(tokenData.Ciphertext, info); err != nil {
					fts.logger.Warn("Failed to unmarshal token info", zap.Error(err))
					continue
				}
			}
			
			// Only return non-expired tokens
			if time.Now().Before(info.ExpiresAt) {
				tokens = append(tokens, info)
			}
		}
	}
	
	return tokens, nil
}

// CleanupExpiredTokens removes expired tokens from file
func (fts *FileTokenStorage) CleanupExpiredTokens() error {
	fts.mu.Lock()
	defer fts.mu.Unlock()
	
	// Load store
	store, err := fts.loadStore()
	if err != nil {
		return fmt.Errorf("failed to load token store: %w", err)
	}
	
	now := time.Now()
	var expiredTokens []string
	
	// Find expired tokens
	for token, tokenData := range store.Tokens {
		var info *TokenInfo
		if fts.encryptor != nil {
			info, err = fts.encryptor.Decrypt(tokenData)
			if err != nil {
				fts.logger.Warn("Failed to decrypt token during cleanup", zap.Error(err))
				continue
			}
		} else {
			info = &TokenInfo{}
			if err := json.Unmarshal(tokenData.Ciphertext, info); err != nil {
				fts.logger.Warn("Failed to unmarshal token info during cleanup", zap.Error(err))
				continue
			}
		}
		
		if now.After(info.ExpiresAt) {
			expiredTokens = append(expiredTokens, token)
		}
	}
	
	// Remove expired tokens
	for _, token := range expiredTokens {
		tokenData := store.Tokens[token]
		delete(store.Tokens, token)
		
		// Get user ID for mapping cleanup
		var info *TokenInfo
		if fts.encryptor != nil {
			info, _ = fts.encryptor.Decrypt(tokenData)
		} else {
			info = &TokenInfo{}
			json.Unmarshal(tokenData.Ciphertext, info)
		}
		
		// Remove from user tokens mapping
		if info != nil && info.Subject != "" {
			userTokens := store.UserTokens[info.Subject]
			for i, userToken := range userTokens {
				if userToken == token {
					store.UserTokens[info.Subject] = append(userTokens[:i], userTokens[i+1:]...)
					break
				}
			}
			
			// Clean up empty user token lists
			if len(store.UserTokens[info.Subject]) == 0 {
				delete(store.UserTokens, info.Subject)
			}
		}
	}
	
	if len(expiredTokens) > 0 {
		store.LastUpdated = time.Now()
		if err := fts.saveStore(store); err != nil {
			return fmt.Errorf("failed to save store after cleanup: %w", err)
		}
		
		fts.logger.Info("Cleaned up expired tokens",
			zap.Int("count", len(expiredTokens)))
	}
	
	return nil
}

// loadStore loads the token store from file
func (fts *FileTokenStorage) loadStore() (*TokenStore, error) {
	// Check if file exists
	if _, err := os.Stat(fts.filePath); os.IsNotExist(err) {
		// Create new empty store
		return &TokenStore{
			Tokens:      make(map[string]*EncryptedTokenData),
			UserTokens:  make(map[string][]string),
			LastUpdated: time.Now(),
			Version:     "1.0",
		}, nil
	}
	
	// Read file
	data, err := os.ReadFile(fts.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	// Parse JSON
	var store TokenStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}
	
	// Initialize maps if nil
	if store.Tokens == nil {
		store.Tokens = make(map[string]*EncryptedTokenData)
	}
	if store.UserTokens == nil {
		store.UserTokens = make(map[string][]string)
	}
	
	return &store, nil
}

// saveStore saves the token store to file
func (fts *FileTokenStorage) saveStore(store *TokenStore) error {
	// Marshal to JSON
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal store: %w", err)
	}
	
	// Write to file with secure permissions
	if err := os.WriteFile(fts.filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	return nil
}

// Cleanup performs cleanup operations for file storage
func (fts *FileTokenStorage) Cleanup() error {
	// Clean up expired tokens
	if err := fts.CleanupExpiredTokens(); err != nil {
		fts.logger.Error("Failed to cleanup expired tokens", zap.Error(err))
	}
	
	fts.logger.Info("File token storage cleanup completed")
	return nil
}

// NewTokenEncryptor creates a new token encryptor
func NewTokenEncryptor(config *EncryptionConfig, logger *zap.Logger) (*TokenEncryptor, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("encryption is not enabled")
	}
	
	// Load encryption key
	var key []byte
	var err error
	
	if config.KeyFile != "" {
		key, err = os.ReadFile(config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read encryption key file: %w", err)
		}
	} else {
		// Generate a random key (not recommended for production)
		key = make([]byte, 32) // AES-256
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("failed to generate encryption key: %w", err)
		}
		logger.Warn("Using randomly generated encryption key - tokens will not persist across restarts")
	}
	
	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM mode: %w", err)
	}
	
	return &TokenEncryptor{
		key:    key,
		gcm:    gcm,
		logger: logger,
	}, nil
}

// Encrypt encrypts token info
func (te *TokenEncryptor) Encrypt(info *TokenInfo) (*EncryptedTokenData, error) {
	// Marshal token info
	plaintext, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token info: %w", err)
	}
	
	// Generate nonce
	nonce := make([]byte, te.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	// Encrypt
	ciphertext := te.gcm.Seal(nil, nonce, plaintext, nil)
	
	return &EncryptedTokenData{
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, nil
}

// Decrypt decrypts token info
func (te *TokenEncryptor) Decrypt(data *EncryptedTokenData) (*TokenInfo, error) {
	// Decrypt
	plaintext, err := te.gcm.Open(nil, data.Nonce, data.Ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	
	// Unmarshal token info
	var info TokenInfo
	if err := json.Unmarshal(plaintext, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token info: %w", err)
	}
	
	return &info, nil
}