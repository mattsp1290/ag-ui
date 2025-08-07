package security

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// SecretManager provides secure secret management with environment variable fallback
type SecretManager struct {
	secrets     map[string]string
	envPrefix   string
	mu          sync.RWMutex
	initialized bool
	fallbacks   map[string]string
}

// SecretConfig represents secure secret configuration
type SecretConfig struct {
	// Environment variable prefix for secrets (e.g., "AGUI_", "MYAPP_")
	EnvPrefix string `json:"env_prefix" yaml:"env_prefix"`

	// Required secrets that must be provided
	RequiredSecrets []string `json:"required_secrets" yaml:"required_secrets"`

	// Optional secrets with fallback values
	OptionalSecrets map[string]string `json:"optional_secrets" yaml:"optional_secrets"`

	// Whether to allow fallback to configuration for development
	AllowConfigFallback bool `json:"allow_config_fallback" yaml:"allow_config_fallback"`

	// Whether to validate secret strength
	ValidateSecretStrength bool `json:"validate_secret_strength" yaml:"validate_secret_strength"`

	// Minimum secret length
	MinSecretLength int `json:"min_secret_length" yaml:"min_secret_length"`
}

// NewSecretManager creates a new secure secret manager
func NewSecretManager(config *SecretConfig) (*SecretManager, error) {
	if config == nil {
		config = &SecretConfig{
			EnvPrefix:              "AGUI_",
			ValidateSecretStrength: true,
			MinSecretLength:        32,
			AllowConfigFallback:    false, // Secure by default
		}
	}

	sm := &SecretManager{
		secrets:   make(map[string]string),
		envPrefix: config.EnvPrefix,
		fallbacks: config.OptionalSecrets,
	}

	// Load secrets from environment variables
	if err := sm.loadFromEnvironment(config); err != nil {
		return nil, fmt.Errorf("failed to load secrets from environment: %w", err)
	}

	// Validate required secrets
	if err := sm.validateRequiredSecrets(config.RequiredSecrets); err != nil {
		return nil, fmt.Errorf("secret validation failed: %w", err)
	}

	// Validate secret strength if enabled
	if config.ValidateSecretStrength {
		if err := sm.validateSecretStrength(config.MinSecretLength); err != nil {
			return nil, fmt.Errorf("secret strength validation failed: %w", err)
		}
	}

	sm.initialized = true
	return sm, nil
}

// GetSecret retrieves a secret by name
func (sm *SecretManager) GetSecret(name string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.initialized {
		return "", fmt.Errorf("secret manager not initialized")
	}

	// Try to get from cached secrets
	if secret, exists := sm.secrets[name]; exists {
		return secret, nil
	}

	// Try environment variable directly (with prefix)
	envKey := sm.envPrefix + strings.ToUpper(name)
	if secret := os.Getenv(envKey); secret != "" {
		// Cache the secret
		sm.secrets[name] = secret
		return secret, nil
	}

	// Try fallback value
	if fallback, exists := sm.fallbacks[name]; exists {
		return fallback, nil
	}

	return "", fmt.Errorf("secret not found: %s", name)
}

// SetSecret sets a secret value (for runtime configuration)
func (sm *SecretManager) SetSecret(name, value string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if value == "" {
		return fmt.Errorf("secret value cannot be empty")
	}

	sm.secrets[name] = value
	return nil
}

// HasSecret checks if a secret exists
func (sm *SecretManager) HasSecret(name string) bool {
	secret, err := sm.GetSecret(name)
	return err == nil && secret != ""
}

// GenerateSecret generates a cryptographically secure random secret
func (sm *SecretManager) GenerateSecret(length int) (string, error) {
	if length < 16 {
		return "", fmt.Errorf("secret length must be at least 16 characters")
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}

	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// GenerateHexSecret generates a hex-encoded secret
func (sm *SecretManager) GenerateHexSecret(byteLength int) (string, error) {
	if byteLength < 16 {
		return "", fmt.Errorf("secret byte length must be at least 16")
	}

	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}

	return hex.EncodeToString(bytes), nil
}

// RotateSecret generates a new secret and updates the stored value
func (sm *SecretManager) RotateSecret(name string, length int) (string, error) {
	newSecret, err := sm.GenerateSecret(length)
	if err != nil {
		return "", fmt.Errorf("failed to generate new secret: %w", err)
	}

	if err := sm.SetSecret(name, newSecret); err != nil {
		return "", fmt.Errorf("failed to set new secret: %w", err)
	}

	return newSecret, nil
}

// LoadSecretsFromFile loads secrets from a secure file (for initialization only)
func (sm *SecretManager) LoadSecretsFromFile(filePath string) error {
	// This would typically load from a secure vault or encrypted file
	// For now, we'll just indicate this capability exists
	return fmt.Errorf("file-based secret loading not implemented - use environment variables")
}

// ClearSecret removes a secret from memory
func (sm *SecretManager) ClearSecret(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.secrets, name)
}

// ClearAllSecrets removes all secrets from memory
func (sm *SecretManager) ClearAllSecrets() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.secrets = make(map[string]string)
}

// loadFromEnvironment loads secrets from environment variables
func (sm *SecretManager) loadFromEnvironment(config *SecretConfig) error {
	// Load required secrets
	for _, secretName := range config.RequiredSecrets {
		envKey := sm.envPrefix + strings.ToUpper(secretName)
		if secret := os.Getenv(envKey); secret != "" {
			sm.secrets[secretName] = secret
		}
	}

	// Load optional secrets
	for secretName := range config.OptionalSecrets {
		envKey := sm.envPrefix + strings.ToUpper(secretName)
		if secret := os.Getenv(envKey); secret != "" {
			sm.secrets[secretName] = secret
		}
	}

	return nil
}

// validateRequiredSecrets ensures all required secrets are available
func (sm *SecretManager) validateRequiredSecrets(required []string) error {
	var missing []string

	for _, secretName := range required {
		// Check if secret is available in cache or environment during initialization
		if _, exists := sm.secrets[secretName]; !exists {
			// Try to get from environment variable directly during validation
			envKey := sm.envPrefix + strings.ToUpper(secretName)
			if secret := os.Getenv(envKey); secret == "" {
				missing = append(missing, secretName)
			}
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required secrets: %v", missing)
	}

	return nil
}

// validateSecretStrength validates that secrets meet minimum security requirements
func (sm *SecretManager) validateSecretStrength(minLength int) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for name, secret := range sm.secrets {
		if len(secret) < minLength {
			return fmt.Errorf("secret '%s' is too short (minimum %d characters)", name, minLength)
		}

		// Check for common weak patterns
		if sm.isWeakSecret(secret) {
			return fmt.Errorf("secret '%s' appears to be weak or predictable", name)
		}
	}

	return nil
}

// isWeakSecret checks for common weak secret patterns
func (sm *SecretManager) isWeakSecret(secret string) bool {
	// Check for obviously weak secrets
	weakPatterns := []string{
		"password", "123456", "secret", "admin", "test", "demo",
		"default", "changeme", "password123", "qwerty",
	}

	lowerSecret := strings.ToLower(secret)
	for _, pattern := range weakPatterns {
		if strings.Contains(lowerSecret, pattern) {
			return true
		}
	}

	// Check for repeated characters
	if len(secret) > 1 {
		repeated := 0
		for i := 1; i < len(secret); i++ {
			if secret[i] == secret[i-1] {
				repeated++
			}
		}
		// If more than 50% of characters are repeated
		if repeated > len(secret)/2 {
			return true
		}
	}

	return false
}

// SecretReference represents a reference to a secret that should be loaded securely
type SecretReference struct {
	Name         string `json:"name" yaml:"name"`
	EnvVar       string `json:"env_var,omitempty" yaml:"env_var,omitempty"`
	DefaultValue string `json:"default_value,omitempty" yaml:"default_value,omitempty"`
	Required     bool   `json:"required" yaml:"required"`
}

// ResolveSecretReference resolves a secret reference to its actual value
func (sm *SecretManager) ResolveSecretReference(ref *SecretReference) (string, error) {
	// Try direct name lookup first
	if secret, err := sm.GetSecret(ref.Name); err == nil {
		return secret, nil
	}

	// Try custom environment variable
	if ref.EnvVar != "" {
		if secret := os.Getenv(ref.EnvVar); secret != "" {
			// Cache it for future use
			sm.SetSecret(ref.Name, secret)
			return secret, nil
		}
	}

	// Use default value if available and not required
	if !ref.Required && ref.DefaultValue != "" {
		return ref.DefaultValue, nil
	}

	if ref.Required {
		return "", fmt.Errorf("required secret '%s' not found", ref.Name)
	}

	return "", fmt.Errorf("secret '%s' not found", ref.Name)
}

// GetSecretWithFallback gets a secret with multiple fallback options
func (sm *SecretManager) GetSecretWithFallback(name string, fallbacks ...string) (string, error) {
	// Try primary secret
	if secret, err := sm.GetSecret(name); err == nil {
		return secret, nil
	}

	// Try fallbacks
	for _, fallbackName := range fallbacks {
		if secret, err := sm.GetSecret(fallbackName); err == nil {
			return secret, nil
		}
	}

	return "", fmt.Errorf("secret '%s' and fallbacks not found", name)
}

// SecretAuditLog represents an audit log entry for secret operations
type SecretAuditLog struct {
	Timestamp  time.Time `json:"timestamp"`
	Operation  string    `json:"operation"`
	SecretName string    `json:"secret_name"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
}

// AuditSecretAccess logs secret access for security auditing
func (sm *SecretManager) AuditSecretAccess(operation, secretName string, success bool, err error) {
	// In a production system, this would log to a secure audit system
	log := SecretAuditLog{
		Timestamp:  time.Now(),
		Operation:  operation,
		SecretName: secretName,
		Success:    success,
	}

	if err != nil {
		log.Error = err.Error()
	}

	// For now, just store in memory or log to stdout in debug mode
	// In production, this should go to a secure audit log
	if os.Getenv("AGUI_DEBUG_SECRET_AUDIT") == "true" {
		fmt.Printf("SECRET AUDIT: %+v\n", log)
	}
}

// GetEnv retrieves an environment variable (wrapper for testing)
func (sm *SecretManager) GetEnv(key string) string {
	return os.Getenv(key)
}
