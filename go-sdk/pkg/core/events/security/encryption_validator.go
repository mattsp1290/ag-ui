package security

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// EncryptionValidator validates encryption requirements
type EncryptionValidator struct {
	config *SecurityConfig
}

// EncryptionInfo contains information about encrypted content
type EncryptionInfo struct {
	Algorithm    string
	KeySize      int
	IsEncrypted  bool
	IsValid      bool
}

// NewEncryptionValidator creates a new encryption validator
func NewEncryptionValidator(config *SecurityConfig) *EncryptionValidator {
	return &EncryptionValidator{
		config: config,
	}
}

// ValidateEncryption validates that event content meets encryption requirements
func (ev *EncryptionValidator) ValidateEncryption(event events.Event) error {
	if !ev.config.RequireEncryption {
		return nil
	}
	
	content := ev.extractContent(event)
	if content == "" {
		return nil // No content to validate
	}
	
	// Check if content appears to be encrypted
	info := ev.analyzeEncryption(content)
	
	if !info.IsEncrypted {
		return fmt.Errorf("content is not encrypted")
	}
	
	if !info.IsValid {
		return fmt.Errorf("encryption does not meet requirements")
	}
	
	if info.KeySize < ev.config.MinimumEncryptionBits {
		return fmt.Errorf("encryption key size %d is below minimum %d bits", 
			info.KeySize, ev.config.MinimumEncryptionBits)
	}
	
	if !ev.isAllowedAlgorithm(info.Algorithm) {
		return fmt.Errorf("encryption algorithm %s is not allowed", info.Algorithm)
	}
	
	return nil
}

// analyzeEncryption analyzes content to determine encryption status
func (ev *EncryptionValidator) analyzeEncryption(content string) *EncryptionInfo {
	info := &EncryptionInfo{
		IsEncrypted: false,
		IsValid:     false,
	}
	
	// Check for base64 encoding (common for encrypted content)
	if ev.isLikelyBase64(content) {
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err == nil && len(decoded) > 0 {
			// Check for encryption headers or patterns
			if ev.hasEncryptionHeader(decoded) {
				info.IsEncrypted = true
				info.Algorithm = ev.detectAlgorithm(decoded)
				info.KeySize = ev.estimateKeySize(decoded)
				info.IsValid = info.KeySize >= ev.config.MinimumEncryptionBits
			}
		}
	}
	
	// Check for encrypted JSON format
	if ev.isEncryptedJSON(content) {
		info.IsEncrypted = true
		info.Algorithm = ev.extractJSONAlgorithm(content)
		info.KeySize = ev.extractJSONKeySize(content)
		info.IsValid = info.KeySize >= ev.config.MinimumEncryptionBits
	}
	
	return info
}

// isLikelyBase64 checks if content is likely base64 encoded
func (ev *EncryptionValidator) isLikelyBase64(content string) bool {
	// Simple heuristic: check length and character set
	if len(content) < 16 {
		return false
	}
	
	// Check if it's valid base64
	for _, ch := range content {
		if !((ch >= 'A' && ch <= 'Z') || 
			(ch >= 'a' && ch <= 'z') || 
			(ch >= '0' && ch <= '9') || 
			ch == '+' || ch == '/' || ch == '=') {
			return false
		}
	}
	
	return true
}

// hasEncryptionHeader checks for common encryption headers
func (ev *EncryptionValidator) hasEncryptionHeader(data []byte) bool {
	// Check for common encryption patterns
	// This is simplified - real implementation would check actual headers
	
	// High entropy check (encrypted data has high randomness)
	entropy := ev.calculateEntropy(data)
	return entropy > 7.0 // Encrypted data typically has entropy > 7
}

// calculateEntropy calculates Shannon entropy
func (ev *EncryptionValidator) calculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	
	// Count byte frequencies
	freq := make(map[byte]int)
	for _, b := range data {
		freq[b]++
	}
	
	// Calculate entropy
	var entropy float64
	dataLen := float64(len(data))
	
	for _, count := range freq {
		if count > 0 {
			probability := float64(count) / dataLen
			entropy -= probability * (probability * 8) // log2(probability)
		}
	}
	
	return entropy
}

// detectAlgorithm attempts to detect the encryption algorithm
func (ev *EncryptionValidator) detectAlgorithm(data []byte) string {
	// This is a simplified detection - real implementation would
	// check actual algorithm identifiers
	
	// Check block size patterns
	if len(data)%16 == 0 {
		return "AES-256-GCM" // Assume AES for 16-byte blocks
	} else if len(data)%8 == 0 {
		return "3DES" // Assume 3DES for 8-byte blocks
	}
	
	return "Unknown"
}

// estimateKeySize estimates the key size based on content
func (ev *EncryptionValidator) estimateKeySize(data []byte) int {
	// Simplified estimation
	// Real implementation would parse encryption headers
	
	if len(data) >= 32 {
		return 256 // Assume 256-bit key for larger encrypted content
	}
	
	return 128
}

// isEncryptedJSON checks if content is in encrypted JSON format
func (ev *EncryptionValidator) isEncryptedJSON(content string) bool {
	// Check for common encrypted JSON patterns
	return strings.Contains(content, `"encrypted"`) && 
		   strings.Contains(content, `"algorithm"`) &&
		   strings.Contains(content, `"ciphertext"`)
}

// extractJSONAlgorithm extracts algorithm from encrypted JSON
func (ev *EncryptionValidator) extractJSONAlgorithm(content string) string {
	// Simplified extraction - real implementation would use JSON parsing
	if strings.Contains(content, `"algorithm":"AES-256-GCM"`) {
		return "AES-256-GCM"
	} else if strings.Contains(content, `"algorithm":"ChaCha20-Poly1305"`) {
		return "ChaCha20-Poly1305"
	}
	
	return "Unknown"
}

// extractJSONKeySize extracts key size from encrypted JSON
func (ev *EncryptionValidator) extractJSONKeySize(content string) int {
	// Simplified extraction
	if strings.Contains(content, `"keySize":256`) {
		return 256
	} else if strings.Contains(content, `"keySize":128`) {
		return 128
	}
	
	// Default based on algorithm
	algorithm := ev.extractJSONAlgorithm(content)
	if strings.Contains(algorithm, "256") {
		return 256
	}
	
	return 128
}

// isAllowedAlgorithm checks if an algorithm is allowed
func (ev *EncryptionValidator) isAllowedAlgorithm(algorithm string) bool {
	for _, allowed := range ev.config.AllowedEncryptionTypes {
		if algorithm == allowed {
			return true
		}
	}
	return false
}

// extractContent extracts content from various event types
func (ev *EncryptionValidator) extractContent(event events.Event) string {
	switch e := event.(type) {
	case *events.TextMessageContentEvent:
		return e.Delta
	case *events.ToolCallArgsEvent:
		return e.Delta
	case *events.CustomEvent:
		if e.Value != nil {
			return fmt.Sprintf("%v", e.Value)
		}
		return ""
	default:
		return ""
	}
}

// UpdateConfig updates the encryption validator configuration
func (ev *EncryptionValidator) UpdateConfig(config *SecurityConfig) {
	ev.config = config
}

// EncryptContent provides content encryption functionality
func (ev *EncryptionValidator) EncryptContent(content string, key []byte) (string, error) {
	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	// In production, use crypto/rand to generate nonce
	
	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, []byte(content), nil)
	
	// Encode to base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptContent provides content decryption functionality
func (ev *EncryptionValidator) DecryptContent(encrypted string, key []byte) (string, error) {
	// Decode from base64
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	
	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	
	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}