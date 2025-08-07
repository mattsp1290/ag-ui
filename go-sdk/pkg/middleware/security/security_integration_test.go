package security

import (
	"context"
	"os"
	"testing"
	"time"
)

// SimpleAuditLogger for testing
type SimpleAuditLogger struct{}

func (sal *SimpleAuditLogger) LogAuthSuccess(ctx context.Context, message string, metadata map[string]any) {
}
func (sal *SimpleAuditLogger) LogAuthFailure(ctx context.Context, message string, metadata map[string]any) {
}
func (sal *SimpleAuditLogger) LogSecurityEvent(ctx context.Context, eventType, message string, metadata map[string]any) {
}

// redactSensitiveMetadata creates a copy of the metadata map with sensitive fields redacted
func (sal *SimpleAuditLogger) redactSensitiveMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}

	result := make(map[string]any)
	sensitiveFields := map[string]bool{
		"password":      true,
		"token":         true,
		"access_token":  true,
		"refresh_token": true,
		"client_secret": true,
		"secret":        true,
		"key":           true,
		"private_key":   true,
		"api_key":       true,
	}

	for k, v := range metadata {
		if sensitiveFields[k] {
			result[k] = "[REDACTED]"
		} else {
			result[k] = v
		}
	}

	return result
}

// TestSecretManagerSecureMode tests the secret manager in secure mode
func TestSecretManagerSecureMode(t *testing.T) {
	// Test environment variable loading
	os.Setenv("TEST_SECRET", "k7Hx9mPz4vBnQ8rTyW3fJ6sL2aXc1DgZ5eN0uI8oR9pY")
	defer os.Unsetenv("TEST_SECRET")

	// Verify environment variable is set
	if val := os.Getenv("TEST_SECRET"); val == "" {
		t.Fatalf("TEST_SECRET environment variable not set")
	} else {
		t.Logf("TEST_SECRET is set to: %s", val)
	}

	config := &SecretConfig{
		EnvPrefix:              "TEST_",
		ValidateSecretStrength: true,
		MinSecretLength:        32,
		RequiredSecrets:        []string{"secret"},
	}

	sm, err := NewSecretManager(config)
	if err != nil {
		t.Fatalf("Failed to create secret manager: %v", err)
	}

	// Test secret retrieval
	secret, err := sm.GetSecret("secret")
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}

	expected := "k7Hx9mPz4vBnQ8rTyW3fJ6sL2aXc1DgZ5eN0uI8oR9pY"
	if secret != expected {
		t.Errorf("Expected secret '%s', got '%s'", expected, secret)
	}
}

// TestSecretManagerWeakSecretDetection tests weak secret detection
func TestSecretManagerWeakSecretDetection(t *testing.T) {
	config := &SecretConfig{
		EnvPrefix:              "WEAK_",
		ValidateSecretStrength: true,
		MinSecretLength:        32,
	}

	sm, err := NewSecretManager(config)
	if err != nil {
		t.Fatalf("Failed to create secret manager: %v", err)
	}

	// Test setting a weak secret
	err = sm.SetSecret("weak", "password123")
	if err != nil {
		t.Fatalf("Failed to set weak secret: %v", err)
	}

	// Validate should catch the weak secret
	err = sm.validateSecretStrength(8)
	if err == nil {
		t.Error("Expected validation to fail for weak secret, but it passed")
	}
}

// TestEnhancedInputValidationSQLInjection tests SQL injection detection
func TestEnhancedInputValidationSQLInjection(t *testing.T) {
	validator, err := NewEnhancedInputValidator(nil) // Use defaults
	if err != nil {
		t.Fatalf("Failed to create enhanced input validator: %v", err)
	}

	sqlInjectionPayloads := []string{
		"'; DROP TABLE users; --",
		"' UNION SELECT * FROM passwords --",
		"' OR '1'='1",
		"admin'/**/OR/**/1=1#",
	}

	for _, payload := range sqlInjectionPayloads {
		err := validator.validateString(payload)
		if err == nil {
			t.Errorf("Expected SQL injection detection to fail for payload: %s", payload)
		} else {
			t.Logf("Correctly detected SQL injection in: %s", payload)
		}
	}
}

// TestEnhancedInputValidationXSS tests XSS detection
func TestEnhancedInputValidationXSS(t *testing.T) {
	validator, err := NewEnhancedInputValidator(nil) // Use defaults
	if err != nil {
		t.Fatalf("Failed to create enhanced input validator: %v", err)
	}

	xssPayloads := []string{
		"<script>alert('xss')</script>",
		"<img src=x onerror=alert('xss')>",
		"javascript:alert('xss')",
		"<iframe src='javascript:alert(1)'></iframe>",
	}

	for _, payload := range xssPayloads {
		err := validator.validateString(payload)
		if err == nil {
			t.Errorf("Expected XSS detection to fail for payload: %s", payload)
		} else {
			t.Logf("Correctly detected XSS in: %s", payload)
		}
	}
}

// TestSecureConfigManagerSecretDetection tests secret field detection
func TestSecureConfigManagerSecretDetection(t *testing.T) {
	options := &SecureConfigOptions{
		EnvPrefix:     "TEST_",
		RedactSecrets: true,
	}

	scm, err := NewSecureConfigManager(options)
	if err != nil {
		t.Fatalf("Failed to create secure config manager: %v", err)
	}

	// Test secret field detection
	testCases := []struct {
		fieldName string
		expected  bool
	}{
		{"secret", true},
		{"password", true},
		{"token", true},
		{"jwt_secret", true},
		{"client_secret", true},
		{"username", false},
		{"email", false},
		{"timeout", false},
	}

	for _, tc := range testCases {
		result := scm.IsSecretField(tc.fieldName)
		if result != tc.expected {
			t.Errorf("Expected IsSecretField('%s') = %v, got %v", tc.fieldName, tc.expected, result)
		}
	}
}

// TestSecureMiddlewareFactoryCreation tests secure middleware factory
func TestSecureMiddlewareFactoryCreation(t *testing.T) {
	// Set up required environment variables
	os.Setenv("AGUI_JWT_SECRET", "A9bF3xK7rP2qL8mW5jR0tY4uI1oE6zN3vS9cH8dG2kX")
	os.Setenv("AGUI_OAUTH2_CLIENT_SECRET", "M4nB7xQ1pL9rE2wT8yU5iO3kH6jF0vC1zX9mS4aD7gN")
	os.Setenv("AGUI_ENCRYPTION_KEY", "P8qK2rM9xB4fT1yW7uR5hL3jF6vC0zN8dG1aS9eX4mQ")
	defer func() {
		os.Unsetenv("AGUI_JWT_SECRET")
		os.Unsetenv("AGUI_OAUTH2_CLIENT_SECRET")
		os.Unsetenv("AGUI_ENCRYPTION_KEY")
	}()

	factory, err := NewSecureMiddlewareFactory()
	if err != nil {
		t.Fatalf("Failed to create secure middleware factory: %v", err)
	}

	// Test JWT middleware creation
	config := map[string]interface{}{
		"algorithm":  "HS256",
		"issuer":     "test-issuer",
		"expiration": "1h",
	}

	middleware, err := factory.CreateSecureJWTMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create secure JWT middleware: %v", err)
	}

	if middleware == nil {
		t.Error("Expected middleware to be created, got nil")
	}

	// Test middleware name (should be "secure_jwt" as returned by the factory)
	expectedName := "secure_jwt"
	if middleware.Name() != expectedName {
		t.Errorf("Expected middleware name '%s', got '%s'", expectedName, middleware.Name())
	}
}

// TestRequestValidationIntegration tests the complete request validation flow
func TestRequestValidationIntegration(t *testing.T) {
	validator, err := NewEnhancedInputValidator(nil)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	ctx := context.Background()

	// Test valid request
	validReq := &Request{
		ID:     "test-123",
		Method: "POST",
		Path:   "/api/users",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   "test-client/1.0",
		},
		Body: map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
		},
		Timestamp: time.Now(),
	}

	err = validator.ValidateRequest(ctx, validReq)
	if err != nil {
		t.Errorf("Valid request should pass validation, got error: %v", err)
	}

	// Test malicious request
	maliciousReq := &Request{
		ID:     "test-456",
		Method: "POST",
		Path:   "/api/users/../../../etc/passwd",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]interface{}{
			"query": "'; DROP TABLE users; --",
		},
		Timestamp: time.Now(),
	}

	err = validator.ValidateRequest(ctx, maliciousReq)
	if err == nil {
		t.Error("Malicious request should fail validation")
	} else {
		t.Logf("Correctly rejected malicious request: %v", err)
	}
}

// TestSecurityAuditLogger tests the secure audit logger
func TestSecurityAuditLogger(t *testing.T) {
	logger := &SimpleAuditLogger{}

	// Test metadata redaction
	metadata := map[string]any{
		"username":      "testuser",
		"password":      "secret123",
		"token":         "bearer-token-123",
		"client_id":     "test-client",
		"client_secret": "super-secret",
	}

	safeMetadata := logger.redactSensitiveMetadata(metadata)

	// Check that sensitive fields are redacted
	if safeMetadata["password"] != "[REDACTED]" {
		t.Error("Expected password to be redacted")
	}

	if safeMetadata["token"] != "[REDACTED]" {
		t.Error("Expected token to be redacted")
	}

	if safeMetadata["client_secret"] != "[REDACTED]" {
		t.Error("Expected client_secret to be redacted")
	}

	// Check that non-sensitive fields are preserved
	if safeMetadata["username"] != "testuser" {
		t.Error("Expected username to be preserved")
	}

	if safeMetadata["client_id"] != "test-client" {
		t.Error("Expected client_id to be preserved")
	}
}

// BenchmarkEnhancedInputValidation benchmarks the validation performance
func BenchmarkEnhancedInputValidation(b *testing.B) {
	validator, err := NewEnhancedInputValidator(nil)
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	testString := "This is a test string with normal content"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.validateString(testString)
	}
}

// BenchmarkSecretManagerGetSecret benchmarks secret retrieval
func BenchmarkSecretManagerGetSecret(b *testing.B) {
	os.Setenv("BENCH_SECRET", "benchmark-secret-value-for-testing-performance")
	defer os.Unsetenv("BENCH_SECRET")

	config := &SecretConfig{
		EnvPrefix:       "BENCH_",
		MinSecretLength: 16,
	}

	sm, err := NewSecretManager(config)
	if err != nil {
		b.Fatalf("Failed to create secret manager: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sm.GetSecret("secret")
	}
}
