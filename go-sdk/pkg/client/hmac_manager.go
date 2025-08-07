package client

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HMACManager handles HMAC signature-based authentication
type HMACManager struct {
	config     *HMACConfig
	logger     *zap.Logger
	nonces     map[string]time.Time
	nonceMu    sync.RWMutex
	secretKeys map[string]string // key_id -> secret_key
	keysMu     sync.RWMutex
}

// HMACSignatureInfo contains information about an HMAC signature
type HMACSignatureInfo struct {
	Signature string            `json:"signature"`
	KeyID     string            `json:"key_id"`
	Timestamp time.Time         `json:"timestamp"`
	Nonce     string            `json:"nonce"`
	Headers   map[string]string `json:"headers"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Body      string            `json:"body,omitempty"`
}

// NewHMACManager creates a new HMAC manager
func NewHMACManager(config *HMACConfig, logger *zap.Logger) (*HMACManager, error) {
	if config == nil {
		return nil, fmt.Errorf("HMAC config cannot be nil")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	hm := &HMACManager{
		config:     config,
		logger:     logger,
		nonces:     make(map[string]time.Time),
		secretKeys: make(map[string]string),
	}

	// Add default secret key
	if config.SecretKey != "" {
		hm.secretKeys["default"] = config.SecretKey
	}

	// Start nonce cleanup goroutine
	go hm.cleanupNonces()

	return hm, nil
}

// ValidateSignature validates an HMAC signature from an HTTP request
func (hm *HMACManager) ValidateSignature(req *http.Request) (*UserInfo, error) {
	// Extract signature from request
	signatureInfo, err := hm.extractSignatureInfo(req)
	if err != nil {
		return nil, fmt.Errorf("failed to extract signature info: %w", err)
	}

	// Validate timestamp
	if err := hm.validateTimestamp(signatureInfo.Timestamp); err != nil {
		return nil, fmt.Errorf("timestamp validation failed: %w", err)
	}

	// Validate nonce
	if err := hm.validateNonce(signatureInfo.Nonce); err != nil {
		return nil, fmt.Errorf("nonce validation failed: %w", err)
	}

	// Get secret key
	secretKey, err := hm.getSecretKey(signatureInfo.KeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret key: %w", err)
	}

	// Generate expected signature
	expectedSignature, err := hm.generateSignature(req, secretKey, signatureInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to generate expected signature: %w", err)
	}

	// Compare signatures
	if !hm.compareSignatures(signatureInfo.Signature, expectedSignature) {
		hm.logger.Warn("HMAC signature mismatch",
			zap.String("key_id", signatureInfo.KeyID),
			zap.String("expected", expectedSignature[:16]+"..."),
			zap.String("provided", signatureInfo.Signature[:16]+"..."))
		return nil, fmt.Errorf("signature validation failed")
	}

	// Mark nonce as used
	hm.markNonceUsed(signatureInfo.Nonce)

	// Create user info (in a real implementation, this would come from a user store)
	userInfo := &UserInfo{
		ID:       signatureInfo.KeyID,
		Username: signatureInfo.KeyID,
		Metadata: map[string]interface{}{
			"auth_method": "hmac",
			"key_id":      signatureInfo.KeyID,
			"timestamp":   signatureInfo.Timestamp,
			"nonce":       signatureInfo.Nonce,
		},
	}

	hm.logger.Debug("HMAC signature validated successfully",
		zap.String("key_id", signatureInfo.KeyID),
		zap.Time("timestamp", signatureInfo.Timestamp),
		zap.String("nonce", signatureInfo.Nonce))

	return userInfo, nil
}

// extractSignatureInfo extracts signature information from the request
func (hm *HMACManager) extractSignatureInfo(req *http.Request) (*HMACSignatureInfo, error) {
	info := &HMACSignatureInfo{
		Method:  req.Method,
		Path:    req.URL.Path,
		Headers: make(map[string]string),
	}

	// Extract signature
	signature := req.Header.Get(hm.config.HeaderName)
	if signature == "" {
		return nil, fmt.Errorf("missing signature header: %s", hm.config.HeaderName)
	}

	// Parse signature format: "algorithm=sha256,keyId=key123,signature=abc123"
	signatureParts := strings.Split(signature, ",")
	signatureMap := make(map[string]string)

	for _, part := range signatureParts {
		keyValue := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(keyValue) == 2 {
			signatureMap[keyValue[0]] = keyValue[1]
		}
	}

	// Extract signature value
	if sig, ok := signatureMap["signature"]; ok {
		info.Signature = sig
	} else {
		return nil, fmt.Errorf("missing signature value")
	}

	// Extract key ID
	if keyID, ok := signatureMap["keyId"]; ok {
		info.KeyID = keyID
	} else {
		info.KeyID = "default"
	}

	// Extract timestamp
	timestampStr := req.Header.Get(hm.config.TimestampHeader)
	if timestampStr == "" {
		return nil, fmt.Errorf("missing timestamp header: %s", hm.config.TimestampHeader)
	}

	timestampInt, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp format: %w", err)
	}
	info.Timestamp = time.Unix(timestampInt, 0)

	// Extract nonce
	info.Nonce = req.Header.Get(hm.config.NonceHeader)
	if info.Nonce == "" {
		return nil, fmt.Errorf("missing nonce header: %s", hm.config.NonceHeader)
	}

	// Extract included headers
	for _, headerName := range hm.config.IncludeHeaders {
		if value := req.Header.Get(headerName); value != "" {
			info.Headers[strings.ToLower(headerName)] = value
		}
	}

	return info, nil
}

// validateTimestamp validates the request timestamp
func (hm *HMACManager) validateTimestamp(timestamp time.Time) error {
	now := time.Now()

	// Check if timestamp is too old
	if now.Sub(timestamp) > hm.config.MaxClockSkew {
		return fmt.Errorf("timestamp is too old")
	}

	// Check if timestamp is too far in the future
	if timestamp.Sub(now) > hm.config.MaxClockSkew {
		return fmt.Errorf("timestamp is too far in the future")
	}

	return nil
}

// validateNonce validates the request nonce
func (hm *HMACManager) validateNonce(nonce string) error {
	if nonce == "" {
		return fmt.Errorf("nonce cannot be empty")
	}

	// Check if nonce has been used recently
	hm.nonceMu.RLock()
	_, used := hm.nonces[nonce]
	hm.nonceMu.RUnlock()

	if used {
		return fmt.Errorf("nonce has already been used")
	}

	return nil
}

// markNonceUsed marks a nonce as used
func (hm *HMACManager) markNonceUsed(nonce string) {
	hm.nonceMu.Lock()
	hm.nonces[nonce] = time.Now()
	hm.nonceMu.Unlock()
}

// getSecretKey retrieves the secret key for the given key ID
func (hm *HMACManager) getSecretKey(keyID string) (string, error) {
	hm.keysMu.RLock()
	defer hm.keysMu.RUnlock()

	secretKey, exists := hm.secretKeys[keyID]
	if !exists {
		return "", fmt.Errorf("secret key not found for key ID: %s", keyID)
	}

	return secretKey, nil
}

// generateSignature generates an HMAC signature for the request
func (hm *HMACManager) generateSignature(req *http.Request, secretKey string, info *HMACSignatureInfo) (string, error) {
	// Build string to sign
	stringToSign, err := hm.buildStringToSign(req, info)
	if err != nil {
		return "", fmt.Errorf("failed to build string to sign: %w", err)
	}

	// Create HMAC hasher
	var hasher hash.Hash
	switch hm.config.Algorithm {
	case "sha256":
		hasher = hmac.New(sha256.New, []byte(secretKey))
	case "sha512":
		hasher = hmac.New(sha512.New, []byte(secretKey))
	default:
		return "", fmt.Errorf("unsupported HMAC algorithm: %s", hm.config.Algorithm)
	}

	// Calculate signature
	hasher.Write([]byte(stringToSign))
	signature := hex.EncodeToString(hasher.Sum(nil))

	hm.logger.Debug("Generated HMAC signature",
		zap.String("algorithm", hm.config.Algorithm),
		zap.String("string_to_sign", stringToSign),
		zap.String("signature", signature[:16]+"..."))

	return signature, nil
}

// buildStringToSign builds the canonical string to sign
func (hm *HMACManager) buildStringToSign(req *http.Request, info *HMACSignatureInfo) (string, error) {
	var parts []string

	// Add HTTP method
	parts = append(parts, strings.ToUpper(info.Method))

	// Add path
	parts = append(parts, info.Path)

	// Add query string if present
	if req.URL.RawQuery != "" {
		parts = append(parts, req.URL.RawQuery)
	}

	// Add timestamp
	parts = append(parts, strconv.FormatInt(info.Timestamp.Unix(), 10))

	// Add nonce
	parts = append(parts, info.Nonce)

	// Add included headers in sorted order
	var headerNames []string
	for name := range info.Headers {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)

	for _, name := range headerNames {
		parts = append(parts, fmt.Sprintf("%s:%s", name, info.Headers[name]))
	}

	// Add body hash if request has a body
	if req.Body != nil && (req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH") {
		// In a real implementation, you would read and hash the body
		// For now, we'll use a placeholder
		parts = append(parts, "body:placeholder")
	}

	stringToSign := strings.Join(parts, "\n")
	return stringToSign, nil
}

// compareSignatures compares two signatures in constant time
func (hm *HMACManager) compareSignatures(provided, expected string) bool {
	// Convert to byte slices for constant-time comparison
	providedBytes := []byte(provided)
	expectedBytes := []byte(expected)

	// Ensure both slices are the same length
	if len(providedBytes) != len(expectedBytes) {
		return false
	}

	// Use constant-time comparison
	result := byte(0)
	for i := 0; i < len(providedBytes); i++ {
		result |= providedBytes[i] ^ expectedBytes[i]
	}

	return result == 0
}

// SignRequest signs an HTTP request with HMAC
func (hm *HMACManager) SignRequest(req *http.Request, keyID, secretKey string) error {
	// Generate timestamp and nonce
	timestamp := time.Now()
	nonce := generateNonce()

	// Set required headers
	req.Header.Set(hm.config.TimestampHeader, strconv.FormatInt(timestamp.Unix(), 10))
	req.Header.Set(hm.config.NonceHeader, nonce)

	// Create signature info
	info := &HMACSignatureInfo{
		KeyID:     keyID,
		Timestamp: timestamp,
		Nonce:     nonce,
		Method:    req.Method,
		Path:      req.URL.Path,
		Headers:   make(map[string]string),
	}

	// Extract included headers
	for _, headerName := range hm.config.IncludeHeaders {
		if value := req.Header.Get(headerName); value != "" {
			info.Headers[strings.ToLower(headerName)] = value
		}
	}

	// Generate signature
	signature, err := hm.generateSignature(req, secretKey, info)
	if err != nil {
		return fmt.Errorf("failed to generate signature: %w", err)
	}

	// Set signature header
	signatureHeader := fmt.Sprintf("algorithm=%s,keyId=%s,signature=%s",
		hm.config.Algorithm, keyID, signature)
	req.Header.Set(hm.config.HeaderName, signatureHeader)

	hm.logger.Debug("Signed request with HMAC",
		zap.String("key_id", keyID),
		zap.String("method", req.Method),
		zap.String("path", req.URL.Path),
		zap.Time("timestamp", timestamp),
		zap.String("nonce", nonce))

	return nil
}

// AddSecretKey adds a secret key for a given key ID
func (hm *HMACManager) AddSecretKey(keyID, secretKey string) {
	hm.keysMu.Lock()
	hm.secretKeys[keyID] = secretKey
	hm.keysMu.Unlock()

	hm.logger.Info("Added HMAC secret key",
		zap.String("key_id", keyID))
}

// RemoveSecretKey removes a secret key
func (hm *HMACManager) RemoveSecretKey(keyID string) {
	hm.keysMu.Lock()
	delete(hm.secretKeys, keyID)
	hm.keysMu.Unlock()

	hm.logger.Info("Removed HMAC secret key",
		zap.String("key_id", keyID))
}

// ListKeyIDs returns a list of configured key IDs
func (hm *HMACManager) ListKeyIDs() []string {
	hm.keysMu.RLock()
	defer hm.keysMu.RUnlock()

	var keyIDs []string
	for keyID := range hm.secretKeys {
		keyIDs = append(keyIDs, keyID)
	}

	return keyIDs
}

// cleanupNonces removes old nonces to prevent memory leaks
func (hm *HMACManager) cleanupNonces() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-2 * hm.config.MaxClockSkew)

		hm.nonceMu.Lock()
		for nonce, usedAt := range hm.nonces {
			if usedAt.Before(cutoff) {
				delete(hm.nonces, nonce)
			}
		}
		hm.nonceMu.Unlock()

		hm.logger.Debug("Cleaned up old nonces")
	}
}

// GetSignatureInfo extracts signature information from a request for debugging
func (hm *HMACManager) GetSignatureInfo(req *http.Request) (*HMACSignatureInfo, error) {
	return hm.extractSignatureInfo(req)
}

// VerifySignatureComponents verifies individual components of a signature
func (hm *HMACManager) VerifySignatureComponents(req *http.Request) map[string]interface{} {
	result := map[string]interface{}{
		"valid":      false,
		"errors":     []string{},
		"components": map[string]interface{}{},
	}

	var errors []string

	// Extract signature info
	info, err := hm.extractSignatureInfo(req)
	if err != nil {
		errors = append(errors, fmt.Sprintf("signature extraction: %v", err))
		result["errors"] = errors
		return result
	}

	// Type assert components map to avoid index expression errors
	components, ok := result["components"].(map[string]interface{})
	if !ok {
		components = make(map[string]interface{})
		result["components"] = components
	}

	components["signature_info"] = info

	// Validate timestamp
	if err := hm.validateTimestamp(info.Timestamp); err != nil {
		errors = append(errors, fmt.Sprintf("timestamp: %v", err))
	} else {
		components["timestamp_valid"] = true
	}

	// Validate nonce
	if err := hm.validateNonce(info.Nonce); err != nil {
		errors = append(errors, fmt.Sprintf("nonce: %v", err))
	} else {
		components["nonce_valid"] = true
	}

	// Check secret key
	secretKey, err := hm.getSecretKey(info.KeyID)
	if err != nil {
		errors = append(errors, fmt.Sprintf("secret key: %v", err))
	} else {
		components["secret_key_found"] = true

		// Generate expected signature
		expectedSignature, err := hm.generateSignature(req, secretKey, info)
		if err != nil {
			errors = append(errors, fmt.Sprintf("signature generation: %v", err))
		} else {
			components["expected_signature"] = expectedSignature
			components["provided_signature"] = info.Signature
			components["signatures_match"] = hm.compareSignatures(info.Signature, expectedSignature)
		}
	}

	result["errors"] = errors
	result["valid"] = len(errors) == 0

	return result
}

// Cleanup performs cleanup operations
func (hm *HMACManager) Cleanup() error {
	// Clear nonces
	hm.nonceMu.Lock()
	hm.nonces = make(map[string]time.Time)
	hm.nonceMu.Unlock()

	hm.logger.Info("HMAC manager cleanup completed")
	return nil
}

// generateNonce generates a unique nonce
func generateNonce() string {
	return fmt.Sprintf("nonce_%d_%d", time.Now().UnixNano(), time.Now().Unix())
}
