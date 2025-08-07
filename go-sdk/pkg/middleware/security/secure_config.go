package security

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
)

// SecureConfigManager manages configuration with secret protection
type SecureConfigManager struct {
	secretManager *SecretManager
	configs       map[string]interface{}
	secretFields  map[string]bool
	mu            sync.RWMutex
	initialized   bool
}

// SecureConfigOptions represents secure configuration options
type SecureConfigOptions struct {
	// Secret manager instance
	SecretManager *SecretManager

	// Fields that should be treated as secrets
	SecretFields []string

	// Environment prefix for configuration
	EnvPrefix string

	// Whether to allow configuration fallback for secrets in development
	AllowConfigFallback bool

	// Whether to redact secrets in logs and serialization
	RedactSecrets bool

	// Custom secret field detection patterns
	SecretPatterns []string
}

// NewSecureConfigManager creates a new secure configuration manager
func NewSecureConfigManager(options *SecureConfigOptions) (*SecureConfigManager, error) {
	if options == nil {
		options = &SecureConfigOptions{
			EnvPrefix:     "AGUI_",
			RedactSecrets: true,
		}
	}

	// Create secret manager if not provided
	if options.SecretManager == nil {
		secretConfig := &SecretConfig{
			EnvPrefix:              options.EnvPrefix,
			ValidateSecretStrength: true,
			MinSecretLength:        16,
			AllowConfigFallback:    options.AllowConfigFallback,
		}

		sm, err := NewSecretManager(secretConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create secret manager: %w", err)
		}
		options.SecretManager = sm
	}

	scm := &SecureConfigManager{
		secretManager: options.SecretManager,
		configs:       make(map[string]interface{}),
		secretFields:  make(map[string]bool),
	}

	// Build secret fields map
	scm.buildSecretFieldsMap(options)

	scm.initialized = true
	return scm, nil
}

// buildSecretFieldsMap builds a map of fields that should be treated as secrets
func (scm *SecureConfigManager) buildSecretFieldsMap(options *SecureConfigOptions) {
	// Default secret field names
	defaultSecretFields := []string{
		"secret", "password", "token", "key", "api_key", "apikey",
		"client_secret", "private_key", "privatekey", "jwt_secret",
		"auth_token", "access_token", "refresh_token", "session_key",
		"encryption_key", "hmac_secret", "signing_key", "master_key",
	}

	// Add default fields
	for _, field := range defaultSecretFields {
		scm.secretFields[field] = true
		scm.secretFields[strings.ToUpper(field)] = true
		scm.secretFields[strings.ToLower(field)] = true
	}

	// Add user-defined fields
	for _, field := range options.SecretFields {
		scm.secretFields[field] = true
		scm.secretFields[strings.ToUpper(field)] = true
		scm.secretFields[strings.ToLower(field)] = true
	}

	// Add fields matching patterns
	for _, pattern := range options.SecretPatterns {
		// For now, we'll store patterns as-is
		// In a more sophisticated implementation, we'd compile regex patterns
		scm.secretFields[pattern] = true
	}
}

// LoadConfig loads configuration with secure secret handling
func (scm *SecureConfigManager) LoadConfig(configName string, config interface{}) error {
	scm.mu.Lock()
	defer scm.mu.Unlock()

	if !scm.initialized {
		return fmt.Errorf("secure config manager not initialized")
	}

	// Process configuration to replace secret references
	if err := scm.processConfigSecrets(config); err != nil {
		return fmt.Errorf("failed to process config secrets: %w", err)
	}

	// Store processed config
	scm.configs[configName] = config
	return nil
}

// GetConfig retrieves configuration with secrets resolved
func (scm *SecureConfigManager) GetConfig(configName string) (interface{}, error) {
	scm.mu.RLock()
	defer scm.mu.RUnlock()

	config, exists := scm.configs[configName]
	if !exists {
		return nil, fmt.Errorf("configuration not found: %s", configName)
	}

	return config, nil
}

// processConfigSecrets processes configuration to resolve secret references
func (scm *SecureConfigManager) processConfigSecrets(config interface{}) error {
	value := reflect.ValueOf(config)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	return scm.processValue(value, "")
}

// processValue recursively processes configuration values
func (scm *SecureConfigManager) processValue(value reflect.Value, path string) error {
	if !value.IsValid() || !value.CanSet() {
		return nil
	}

	switch value.Kind() {
	case reflect.Struct:
		return scm.processStruct(value, path)
	case reflect.Map:
		return scm.processMap(value, path)
	case reflect.Slice, reflect.Array:
		return scm.processSlice(value, path)
	case reflect.String:
		return scm.processString(value, path)
	}

	return nil
}

// processStruct processes struct fields
func (scm *SecureConfigManager) processStruct(value reflect.Value, basePath string) error {
	valueType := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := valueType.Field(i)
		fieldName := fieldType.Name

		// Build field path
		fieldPath := fieldName
		if basePath != "" {
			fieldPath = basePath + "." + fieldName
		}

		// Check if field is a secret field
		if scm.isSecretField(fieldName, fieldPath) && field.Kind() == reflect.String {
			if err := scm.processSecretField(field, fieldPath); err != nil {
				return fmt.Errorf("failed to process secret field %s: %w", fieldPath, err)
			}
		} else {
			// Recursively process non-secret fields
			if err := scm.processValue(field, fieldPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// processMap processes map values
func (scm *SecureConfigManager) processMap(value reflect.Value, basePath string) error {
	if value.IsNil() {
		return nil
	}

	for _, key := range value.MapKeys() {
		mapValue := value.MapIndex(key)
		keyStr := fmt.Sprintf("%v", key.Interface())

		// Build path
		fieldPath := keyStr
		if basePath != "" {
			fieldPath = basePath + "." + keyStr
		}

		// Check if this is a secret field
		if scm.isSecretField(keyStr, fieldPath) && mapValue.Kind() == reflect.String {
			// Create a new string value that we can modify
			newValue := reflect.New(mapValue.Type()).Elem()
			newValue.SetString(mapValue.String())

			if err := scm.processSecretField(newValue, fieldPath); err != nil {
				return fmt.Errorf("failed to process secret field %s: %w", fieldPath, err)
			}

			// Update the map with the processed value
			value.SetMapIndex(key, newValue)
		} else {
			// Recursively process if it's a complex type
			if mapValue.Kind() == reflect.Interface {
				actualValue := mapValue.Elem()
				if err := scm.processValue(actualValue, fieldPath); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// processSlice processes slice/array elements
func (scm *SecureConfigManager) processSlice(value reflect.Value, basePath string) error {
	for i := 0; i < value.Len(); i++ {
		element := value.Index(i)
		elementPath := fmt.Sprintf("%s[%d]", basePath, i)

		if err := scm.processValue(element, elementPath); err != nil {
			return err
		}
	}

	return nil
}

// processString processes string values for secret references
func (scm *SecureConfigManager) processString(value reflect.Value, path string) error {
	strValue := value.String()

	// Check if this looks like a secret reference (e.g., "${SECRET_NAME}")
	if strings.HasPrefix(strValue, "${") && strings.HasSuffix(strValue, "}") {
		secretName := strings.TrimSuffix(strings.TrimPrefix(strValue, "${"), "}")

		// Resolve the secret
		secret, err := scm.secretManager.GetSecret(secretName)
		if err != nil {
			return fmt.Errorf("failed to resolve secret reference %s: %w", secretName, err)
		}

		// Update the field with the resolved secret
		value.SetString(secret)
	}

	return nil
}

// processSecretField processes a field identified as containing a secret
func (scm *SecureConfigManager) processSecretField(value reflect.Value, fieldPath string) error {
	currentValue := value.String()

	// If the value is empty, try to load from secret manager
	if currentValue == "" {
		// Try to get secret from manager using field path as secret name
		secretName := strings.ToLower(strings.ReplaceAll(fieldPath, ".", "_"))
		if secret, err := scm.secretManager.GetSecret(secretName); err == nil {
			value.SetString(secret)
			return nil
		}
	}

	// Check if current value is a secret reference
	if strings.HasPrefix(currentValue, "${") && strings.HasSuffix(currentValue, "}") {
		secretName := strings.TrimSuffix(strings.TrimPrefix(currentValue, "${"), "}")

		secret, err := scm.secretManager.GetSecret(secretName)
		if err != nil {
			return fmt.Errorf("failed to resolve secret reference %s: %w", secretName, err)
		}

		value.SetString(secret)
	} else if currentValue != "" {
		// Current value might be a direct secret - validate it if needed
		// For now, we'll keep it as-is but log a warning in development
		if os.Getenv("AGUI_ENV") == "development" {
			fmt.Printf("WARNING: Direct secret value detected for field %s. Consider using environment variables.\n", fieldPath)
		}
	}

	return nil
}

// isSecretField checks if a field should be treated as a secret
func (scm *SecureConfigManager) isSecretField(fieldName, fieldPath string) bool {
	// Check direct field name
	if scm.secretFields[strings.ToLower(fieldName)] {
		return true
	}

	// Check full field path
	if scm.secretFields[strings.ToLower(fieldPath)] {
		return true
	}

	// Check for common secret field patterns
	lowerFieldName := strings.ToLower(fieldName)
	secretIndicators := []string{"secret", "password", "token", "key"}

	for _, indicator := range secretIndicators {
		if strings.Contains(lowerFieldName, indicator) {
			return true
		}
	}

	return false
}

// SafeSerialize serializes configuration with secrets redacted
func (scm *SecureConfigManager) SafeSerialize(configName string) ([]byte, error) {
	scm.mu.RLock()
	config, exists := scm.configs[configName]
	scm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("configuration not found: %s", configName)
	}

	// Create a deep copy and redact secrets
	safeCopy := scm.createSafeCopy(config)

	return json.MarshalIndent(safeCopy, "", "  ")
}

// createSafeCopy creates a copy of configuration with secrets redacted
func (scm *SecureConfigManager) createSafeCopy(original interface{}) interface{} {
	return scm.redactSecrets(original, "")
}

// redactSecrets recursively redacts secret values in a configuration
func (scm *SecureConfigManager) redactSecrets(value interface{}, path string) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			fieldPath := key
			if path != "" {
				fieldPath = path + "." + key
			}

			if scm.isSecretField(key, fieldPath) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = scm.redactSecrets(val, fieldPath)
			}
		}
		return result

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			elementPath := fmt.Sprintf("%s[%d]", path, i)
			result[i] = scm.redactSecrets(val, elementPath)
		}
		return result

	default:
		// For structs and other complex types, use reflection
		return scm.redactSecretsReflection(value, path)
	}
}

// redactSecretsReflection uses reflection to redact secrets in structs
func (scm *SecureConfigManager) redactSecretsReflection(original interface{}, basePath string) interface{} {
	value := reflect.ValueOf(original)

	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		// Create a map representation for safe serialization
		result := make(map[string]interface{})
		valueType := value.Type()

		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			fieldType := valueType.Field(i)
			fieldName := fieldType.Name

			// Skip unexported fields
			if !fieldType.IsExported() {
				continue
			}

			fieldPath := fieldName
			if basePath != "" {
				fieldPath = basePath + "." + fieldName
			}

			if scm.isSecretField(fieldName, fieldPath) {
				result[fieldName] = "[REDACTED]"
			} else {
				result[fieldName] = scm.redactSecrets(field.Interface(), fieldPath)
			}
		}
		return result

	default:
		return original
	}
}

// ValidateConfiguration validates configuration for security best practices
func (scm *SecureConfigManager) ValidateConfiguration(configName string) error {
	scm.mu.RLock()
	config, exists := scm.configs[configName]
	scm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("configuration not found: %s", configName)
	}

	return scm.validateConfigSecrets(config, "")
}

// validateConfigSecrets validates that secrets are properly handled
func (scm *SecureConfigManager) validateConfigSecrets(config interface{}, path string) error {
	value := reflect.ValueOf(config)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		return scm.validateStructSecrets(value, path)
	case reflect.Map:
		return scm.validateMapSecrets(value, path)
	case reflect.Slice, reflect.Array:
		return scm.validateSliceSecrets(value, path)
	}

	return nil
}

// validateStructSecrets validates struct fields for proper secret handling
func (scm *SecureConfigManager) validateStructSecrets(value reflect.Value, basePath string) error {
	valueType := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := valueType.Field(i)
		fieldName := fieldType.Name

		fieldPath := fieldName
		if basePath != "" {
			fieldPath = basePath + "." + fieldName
		}

		if scm.isSecretField(fieldName, fieldPath) && field.Kind() == reflect.String {
			secretValue := field.String()

			// Check if secret looks like it might be hard-coded
			if err := scm.validateSecretValue(secretValue, fieldPath); err != nil {
				return err
			}
		} else {
			// Recursively validate non-secret fields
			if err := scm.validateConfigSecrets(field.Interface(), fieldPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateMapSecrets validates map values for proper secret handling
func (scm *SecureConfigManager) validateMapSecrets(value reflect.Value, basePath string) error {
	if value.IsNil() {
		return nil
	}

	for _, key := range value.MapKeys() {
		keyStr := fmt.Sprintf("%v", key.Interface())
		mapValue := value.MapIndex(key)

		fieldPath := keyStr
		if basePath != "" {
			fieldPath = basePath + "." + keyStr
		}

		if scm.isSecretField(keyStr, fieldPath) && mapValue.Kind() == reflect.String {
			secretValue := mapValue.String()
			if err := scm.validateSecretValue(secretValue, fieldPath); err != nil {
				return err
			}
		} else {
			if err := scm.validateConfigSecrets(mapValue.Interface(), fieldPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateSliceSecrets validates slice elements for proper secret handling
func (scm *SecureConfigManager) validateSliceSecrets(value reflect.Value, basePath string) error {
	for i := 0; i < value.Len(); i++ {
		element := value.Index(i)
		elementPath := fmt.Sprintf("%s[%d]", basePath, i)

		if err := scm.validateConfigSecrets(element.Interface(), elementPath); err != nil {
			return err
		}
	}

	return nil
}

// validateSecretValue validates that a secret value is properly secured
func (scm *SecureConfigManager) validateSecretValue(secretValue, fieldPath string) error {
	// Empty secrets are handled by secret manager
	if secretValue == "" {
		return nil
	}

	// Secret references are OK
	if strings.HasPrefix(secretValue, "${") && strings.HasSuffix(secretValue, "}") {
		return nil
	}

	// In production, we should never have hard-coded secrets
	if os.Getenv("AGUI_ENV") == "production" {
		return fmt.Errorf("hard-coded secret detected in production for field %s", fieldPath)
	}

	// In development, warn about hard-coded secrets
	if os.Getenv("AGUI_ENV") == "development" {
		fmt.Printf("WARNING: Hard-coded secret detected for field %s. Use environment variables in production.\n", fieldPath)
	}

	return nil
}

// GetSecretManager returns the underlying secret manager
func (scm *SecureConfigManager) GetSecretManager() *SecretManager {
	return scm.secretManager
}

// IsSecretField checks if a field name/path should be treated as a secret
func (scm *SecureConfigManager) IsSecretField(fieldName string) bool {
	return scm.isSecretField(fieldName, fieldName)
}
