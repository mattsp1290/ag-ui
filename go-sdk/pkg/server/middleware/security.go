// Package middleware provides secure credential handling utilities
package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

// SecureCredential represents a securely managed credential
type SecureCredential struct {
	envVar    string
	value     string
	validated bool
	logger    *zap.Logger
}

// CredentialValidator defines validation rules for credentials
type CredentialValidator struct {
	MinLength    int
	MaxLength    int
	RequireBase64 bool
	CustomValidator func(string) error
}

// DefaultJWTValidator returns default JWT secret validation rules
func DefaultJWTValidator() *CredentialValidator {
	return &CredentialValidator{
		MinLength:    32, // Minimum 256 bits
		MaxLength:    512,
		RequireBase64: false,
		CustomValidator: func(value string) error {
			if len(value) < 32 {
				return fmt.Errorf("JWT secret must be at least 32 characters (256 bits)")
			}
			return nil
		},
	}
}

// DefaultHMACValidator returns default HMAC secret validation rules
func DefaultHMACValidator() *CredentialValidator {
	return &CredentialValidator{
		MinLength:    32, // Minimum 256 bits
		MaxLength:    512,
		RequireBase64: false,
		CustomValidator: func(value string) error {
			if len(value) < 32 {
				return fmt.Errorf("HMAC secret must be at least 32 characters (256 bits)")
			}
			return nil
		},
	}
}

// DefaultPasswordValidator returns default password validation rules
func DefaultPasswordValidator() *CredentialValidator {
	return &CredentialValidator{
		MinLength:    8,
		MaxLength:    128,
		RequireBase64: false,
		CustomValidator: func(value string) error {
			if len(value) < 8 {
				return fmt.Errorf("password must be at least 8 characters")
			}
			return nil
		},
	}
}

// NewSecureCredential creates a new secure credential from environment variable
func NewSecureCredential(envVar string, validator *CredentialValidator, logger *zap.Logger) (*SecureCredential, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	if envVar == "" {
		return nil, fmt.Errorf("environment variable name cannot be empty")
	}

	value := os.Getenv(envVar)
	if value == "" {
		return nil, fmt.Errorf("environment variable %s is not set or empty", envVar)
	}

	cred := &SecureCredential{
		envVar: envVar,
		value:  value,
		logger: logger,
	}

	// Validate the credential
	if validator != nil {
		if err := cred.validate(validator); err != nil {
			return nil, fmt.Errorf("credential validation failed for %s: %w", envVar, err)
		}
	}

	cred.validated = true
	logger.Info("Secure credential loaded successfully",
		zap.String("env_var", envVar),
		zap.Int("length", len(value)),
		zap.Bool("validated", cred.validated))

	return cred, nil
}

// validate validates the credential against the provided validator
func (c *SecureCredential) validate(validator *CredentialValidator) error {
	if len(c.value) < validator.MinLength {
		return fmt.Errorf("credential length %d is below minimum %d", len(c.value), validator.MinLength)
	}

	if validator.MaxLength > 0 && len(c.value) > validator.MaxLength {
		return fmt.Errorf("credential length %d exceeds maximum %d", len(c.value), validator.MaxLength)
	}

	if validator.RequireBase64 {
		if _, err := base64.StdEncoding.DecodeString(c.value); err != nil {
			return fmt.Errorf("credential is not valid base64: %w", err)
		}
	}

	if validator.CustomValidator != nil {
		if err := validator.CustomValidator(c.value); err != nil {
			return fmt.Errorf("custom validation failed: %w", err)
		}
	}

	return nil
}

// Value returns the credential value (use sparingly and securely)
func (c *SecureCredential) Value() string {
	if !c.validated {
		c.logger.Warn("Accessing unvalidated credential", zap.String("env_var", c.envVar))
	}
	return c.value
}

// Bytes returns the credential as bytes
func (c *SecureCredential) Bytes() []byte {
	return []byte(c.value)
}

// EnvVar returns the environment variable name
func (c *SecureCredential) EnvVar() string {
	return c.envVar
}

// IsValid returns whether the credential has been validated
func (c *SecureCredential) IsValid() bool {
	return c.validated
}

// SecureCompare performs constant-time comparison of the credential with another string
func (c *SecureCredential) SecureCompare(other string) bool {
	return subtle.ConstantTimeCompare([]byte(c.value), []byte(other)) == 1
}

// Clear securely clears the credential from memory (best effort)
func (c *SecureCredential) Clear() {
	// Zero out the string memory (best effort in Go)
	if len(c.value) > 0 {
		// Create a new string of zeros
		zeros := strings.Repeat("\x00", len(c.value))
		c.value = zeros
	}
	c.validated = false
}

// CredentialManager manages multiple secure credentials
type CredentialManager struct {
	credentials map[string]*SecureCredential
	logger      *zap.Logger
}

// NewCredentialManager creates a new credential manager
func NewCredentialManager(logger *zap.Logger) *CredentialManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &CredentialManager{
		credentials: make(map[string]*SecureCredential),
		logger:      logger,
	}
}

// LoadCredential loads and validates a credential from environment variable
func (cm *CredentialManager) LoadCredential(name, envVar string, validator *CredentialValidator) error {
	cred, err := NewSecureCredential(envVar, validator, cm.logger)
	if err != nil {
		return fmt.Errorf("failed to load credential %s: %w", name, err)
	}

	cm.credentials[name] = cred
	return nil
}

// GetCredential retrieves a managed credential
func (cm *CredentialManager) GetCredential(name string) (*SecureCredential, bool) {
	cred, exists := cm.credentials[name]
	return cred, exists
}

// ListCredentials returns the names of all managed credentials
func (cm *CredentialManager) ListCredentials() []string {
	names := make([]string, 0, len(cm.credentials))
	for name := range cm.credentials {
		names = append(names, name)
	}
	return names
}

// Cleanup securely clears all managed credentials
func (cm *CredentialManager) Cleanup() {
	for name, cred := range cm.credentials {
		cred.Clear()
		cm.logger.Debug("Cleared credential", zap.String("name", name))
	}
	cm.credentials = make(map[string]*SecureCredential)
}

// SecureJWTConfig contains secure JWT configuration
type SecureJWTConfig struct {
	// Signing configuration - NO PLAINTEXT CREDENTIALS
	SigningMethod string `json:"signing_method" yaml:"signing_method"`
	SecretKeyEnv  string `json:"secret_key_env" yaml:"secret_key_env"`   // Environment variable name
	PublicKeyEnv  string `json:"public_key_env" yaml:"public_key_env"`   // Environment variable name
	PrivateKeyEnv string `json:"private_key_env" yaml:"private_key_env"` // Environment variable name
	
	// Token validation
	Issuer       string        `json:"issuer" yaml:"issuer"`
	Audience     []string      `json:"audience" yaml:"audience"`
	LeewayTime   time.Duration `json:"leeway_time" yaml:"leeway_time"`
	
	// Token extraction
	TokenHeader string `json:"token_header" yaml:"token_header"`
	TokenPrefix string `json:"token_prefix" yaml:"token_prefix"`
	QueryParam  string `json:"query_param" yaml:"query_param"`
	CookieName  string `json:"cookie_name" yaml:"cookie_name"`
	
	// Runtime credentials (populated from env vars)
	secretKey  *SecureCredential
	publicKey  *SecureCredential
	privateKey *SecureCredential
}

// LoadCredentials loads JWT credentials from environment variables
func (c *SecureJWTConfig) LoadCredentials(logger *zap.Logger) error {
	if logger == nil {
		logger = zap.NewNop()
	}

	var err error

	// Load secret key for HMAC methods
	if c.SecretKeyEnv != "" {
		c.secretKey, err = NewSecureCredential(c.SecretKeyEnv, DefaultJWTValidator(), logger)
		if err != nil {
			return fmt.Errorf("failed to load JWT secret key: %w", err)
		}
	}

	// Load public key for RSA/ECDSA verification
	if c.PublicKeyEnv != "" {
		c.publicKey, err = NewSecureCredential(c.PublicKeyEnv, &CredentialValidator{MinLength: 100}, logger)
		if err != nil {
			return fmt.Errorf("failed to load JWT public key: %w", err)
		}
	}

	// Load private key for RSA/ECDSA signing
	if c.PrivateKeyEnv != "" {
		c.privateKey, err = NewSecureCredential(c.PrivateKeyEnv, &CredentialValidator{MinLength: 100}, logger)
		if err != nil {
			return fmt.Errorf("failed to load JWT private key: %w", err)
		}
	}

	return nil
}

// GetSecretKey returns the secret key credential
func (c *SecureJWTConfig) GetSecretKey() *SecureCredential {
	return c.secretKey
}

// GetPublicKey returns the public key credential
func (c *SecureJWTConfig) GetPublicKey() *SecureCredential {
	return c.publicKey
}

// GetPrivateKey returns the private key credential
func (c *SecureJWTConfig) GetPrivateKey() *SecureCredential {
	return c.privateKey
}

// Cleanup securely clears all credentials
func (c *SecureJWTConfig) Cleanup() {
	if c.secretKey != nil {
		c.secretKey.Clear()
	}
	if c.publicKey != nil {
		c.publicKey.Clear()
	}
	if c.privateKey != nil {
		c.privateKey.Clear()
	}
}

// SecureHMACConfig contains secure HMAC configuration
type SecureHMACConfig struct {
	// Signature configuration - NO PLAINTEXT CREDENTIALS
	SecretKeyEnv string `json:"secret_key_env" yaml:"secret_key_env"` // Environment variable name
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
	
	// Runtime credentials
	secretKey *SecureCredential
}

// LoadCredentials loads HMAC credentials from environment variables
func (c *SecureHMACConfig) LoadCredentials(logger *zap.Logger) error {
	if c.SecretKeyEnv == "" {
		return fmt.Errorf("HMAC secret key environment variable not specified")
	}

	var err error
	c.secretKey, err = NewSecureCredential(c.SecretKeyEnv, DefaultHMACValidator(), logger)
	if err != nil {
		return fmt.Errorf("failed to load HMAC secret key: %w", err)
	}

	return nil
}

// GetSecretKey returns the secret key credential
func (c *SecureHMACConfig) GetSecretKey() *SecureCredential {
	return c.secretKey
}

// Cleanup securely clears all credentials
func (c *SecureHMACConfig) Cleanup() {
	if c.secretKey != nil {
		c.secretKey.Clear()
	}
}

// SecureRedisConfig contains secure Redis configuration
type SecureRedisConfig struct {
	Address     string `json:"address" yaml:"address"`
	PasswordEnv string `json:"password_env" yaml:"password_env"` // Environment variable name
	DB          int    `json:"db" yaml:"db"`
	KeyPrefix   string `json:"key_prefix" yaml:"key_prefix"`
	PoolSize    int    `json:"pool_size" yaml:"pool_size"`
	MaxRetries  int    `json:"max_retries" yaml:"max_retries"`
	EnableTLS   bool   `json:"enable_tls" yaml:"enable_tls"`
	
	// Runtime credentials
	password *SecureCredential
}

// LoadCredentials loads Redis credentials from environment variables
func (c *SecureRedisConfig) LoadCredentials(logger *zap.Logger) error {
	if c.PasswordEnv != "" {
		var err error
		c.password, err = NewSecureCredential(c.PasswordEnv, DefaultPasswordValidator(), logger)
		if err != nil {
			return fmt.Errorf("failed to load Redis password: %w", err)
		}
	}
	return nil
}

// GetPassword returns the password credential
func (c *SecureRedisConfig) GetPassword() *SecureCredential {
	return c.password
}

// Cleanup securely clears all credentials
func (c *SecureRedisConfig) Cleanup() {
	if c.password != nil {
		c.password.Clear()
	}
}

// SecureDatabaseConfig contains secure database configuration
type SecureDatabaseConfig struct {
	Driver              string `json:"driver" yaml:"driver"`
	ConnectionStringEnv string `json:"connection_string_env" yaml:"connection_string_env"` // Environment variable name
	TableName           string `json:"table_name" yaml:"table_name"`
	MaxConnections      int    `json:"max_connections" yaml:"max_connections"`
	EnableSSL           bool   `json:"enable_ssl" yaml:"enable_ssl"`
	
	// Runtime credentials
	connectionString *SecureCredential
}

// LoadCredentials loads database credentials from environment variables
func (c *SecureDatabaseConfig) LoadCredentials(logger *zap.Logger) error {
	if c.ConnectionStringEnv == "" {
		return fmt.Errorf("database connection string environment variable not specified")
	}

	var err error
	c.connectionString, err = NewSecureCredential(c.ConnectionStringEnv, &CredentialValidator{MinLength: 10}, logger)
	if err != nil {
		return fmt.Errorf("failed to load database connection string: %w", err)
	}

	return nil
}

// GetConnectionString returns the connection string credential
func (c *SecureDatabaseConfig) GetConnectionString() *SecureCredential {
	return c.connectionString
}

// Cleanup securely clears all credentials
func (c *SecureDatabaseConfig) Cleanup() {
	if c.connectionString != nil {
		c.connectionString.Clear()
	}
}

// CredentialAuditor provides credential security auditing
type CredentialAuditor struct {
	logger *zap.Logger
}

// NewCredentialAuditor creates a new credential auditor
func NewCredentialAuditor(logger *zap.Logger) *CredentialAuditor {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CredentialAuditor{logger: logger}
}

// AuditCredentialAccess logs credential access events
func (ca *CredentialAuditor) AuditCredentialAccess(credentialName, operation string, success bool) {
	ca.logger.Info("Credential access audit",
		zap.String("credential", credentialName),
		zap.String("operation", operation),
		zap.Bool("success", success),
		zap.Time("timestamp", time.Now()),
	)
}

// AuditCredentialValidation logs credential validation events
func (ca *CredentialAuditor) AuditCredentialValidation(credentialName string, success bool, error error) {
	fields := []zap.Field{
		zap.String("credential", credentialName),
		zap.Bool("success", success),
		zap.Time("timestamp", time.Now()),
	}
	
	if error != nil {
		fields = append(fields, zap.Error(error))
	}
	
	if success {
		ca.logger.Info("Credential validation audit", fields...)
	} else {
		ca.logger.Warn("Credential validation failed", fields...)
	}
}

// ValidateEnvironmentSetup validates that all required environment variables are set
func ValidateEnvironmentSetup(requiredEnvVars []string, logger *zap.Logger) error {
	var missing []string
	
	for _, envVar := range requiredEnvVars {
		if value := os.Getenv(envVar); value == "" {
			missing = append(missing, envVar)
		}
	}
	
	if len(missing) > 0 {
		err := fmt.Errorf("missing required environment variables: %v", missing)
		if logger != nil {
			logger.Error("Environment validation failed", zap.Error(err))
		}
		return err
	}
	
	if logger != nil {
		logger.Info("Environment validation successful", 
			zap.Strings("validated_vars", requiredEnvVars))
	}
	
	return nil
}