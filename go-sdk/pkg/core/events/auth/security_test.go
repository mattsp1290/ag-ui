package auth

import (
	"net/http"
	"testing"
	"time"
)

func TestInputValidation(t *testing.T) {
	// Test basic credentials validation
	t.Run("ValidBasicCredentials", func(t *testing.T) {
		creds := &BasicCredentials{
			Username: "testuser",
			Password: "SecurePass123!",
		}

		err := creds.Validate()
		if err != nil {
			t.Errorf("Expected valid credentials to pass validation, got error: %v", err)
		}
	})

	t.Run("InvalidUsername", func(t *testing.T) {
		creds := &BasicCredentials{
			Username: "ab", // Too short
			Password: "SecurePass123!",
		}

		err := creds.Validate()
		if err == nil {
			t.Error("Expected validation to fail for short username")
		}
	})

	t.Run("InvalidPassword", func(t *testing.T) {
		creds := &BasicCredentials{
			Username: "testuser",
			Password: "weak", // Too weak
		}

		err := creds.Validate()
		if err == nil {
			t.Error("Expected validation to fail for weak password")
		}
	})

	t.Run("InvalidPasswordComplexity", func(t *testing.T) {
		creds := &BasicCredentials{
			Username: "testuser",
			Password: "nouppercaseorspecial123",
		}

		err := creds.Validate()
		if err == nil {
			t.Error("Expected validation to fail for password without complexity")
		}
	})

	// Test token credentials validation
	t.Run("ValidTokenCredentials", func(t *testing.T) {
		creds := &TokenCredentials{
			Token: "dGVzdC10b2tlbi0xMjM0NTY3ODkwYWJjZGVmZw==", // Valid base64-like token
		}

		err := creds.Validate()
		if err != nil {
			t.Errorf("Expected valid token credentials to pass validation, got error: %v", err)
		}
	})

	t.Run("InvalidTokenLength", func(t *testing.T) {
		creds := &TokenCredentials{
			Token: "short", // Too short
		}

		err := creds.Validate()
		if err == nil {
			t.Error("Expected validation to fail for short token")
		}
	})

	// Test API key credentials validation
	t.Run("ValidAPIKeyCredentials", func(t *testing.T) {
		creds := &APIKeyCredentials{
			APIKey: "api-key-1234567890abcdef1234567890abcdef",
		}

		err := creds.Validate()
		if err != nil {
			t.Errorf("Expected valid API key credentials to pass validation, got error: %v", err)
		}
	})

	t.Run("InvalidAPIKeyLength", func(t *testing.T) {
		creds := &APIKeyCredentials{
			APIKey: "short-key",
		}

		err := creds.Validate()
		if err == nil {
			t.Error("Expected validation to fail for short API key")
		}
	})
}

func TestPasswordComplexity(t *testing.T) {
	tests := []struct {
		password string
		expected bool
	}{
		{"SecurePass123!", true},
		{"Password123!", true},
		{"password123!", false}, // No uppercase
		{"PASSWORD123!", false}, // No lowercase
		{"Password!!", false},   // No digit
		{"Password123", false},  // No special char
		{"Pass123!", true},      // Actually meets all criteria
		{"A1b!", true},          // Actually meets all criteria
		{"ValidPassword123@", true},
	}

	for _, test := range tests {
		t.Run(test.password, func(t *testing.T) {
			result := isPasswordComplex(test.password)
			if result != test.expected {
				t.Errorf("Password %s: expected %v, got %v", test.password, test.expected, result)
			}
		})
	}
}

func TestTokenGeneration(t *testing.T) {
	// Test that tokens have increased entropy (256-bit)
	token1, err := generateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	token2, err := generateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Tokens should be different
	if token1 == token2 {
		t.Error("Generated tokens should be unique")
	}

	// Tokens should be longer due to 256-bit entropy
	// Format: "token-" + 64 hex chars (32 bytes * 2)
	expectedLength := len("token-") + 64
	if len(token1) != expectedLength {
		t.Errorf("Expected token length %d, got %d", expectedLength, len(token1))
	}
}

func TestCSRFToken(t *testing.T) {
	config := DefaultCSRFConfig()
	manager, err := NewCSRFManager(config)
	if err != nil {
		t.Fatalf("Failed to create CSRF manager: %v", err)
	}

	userID := "test-user"

	// Generate token
	token, err := manager.GenerateToken(userID)
	if err != nil {
		t.Fatalf("Failed to generate CSRF token: %v", err)
	}

	// Validate token
	err = manager.ValidateToken(token, userID)
	if err != nil {
		t.Errorf("Failed to validate CSRF token: %v", err)
	}

	// Validate with wrong user should fail
	err = manager.ValidateToken(token, "wrong-user")
	if err == nil {
		t.Error("Expected CSRF validation to fail for wrong user")
	}

	// Invalid token should fail
	err = manager.ValidateToken("invalid-token", userID)
	if err == nil {
		t.Error("Expected CSRF validation to fail for invalid token")
	}
}

func TestSecurityLogger(t *testing.T) {
	config := DefaultSecurityLoggerConfig()
	config.LogFile = "" // Disable file logging for test

	logger, err := NewSecurityLogger(config)
	if err != nil {
		t.Fatalf("Failed to create security logger: %v", err)
	}
	defer logger.Close()

	// Test logging various security events
	logger.LogAuthFailure("user123", "testuser", "127.0.0.1", "test-agent", "invalid password")
	logger.LogAuthSuccess("user123", "testuser", "127.0.0.1", "test-agent")
	logger.LogCSRFAttempt("user123", "127.0.0.1", "test-agent", "https://evil.com")
	logger.LogRateLimitExceeded("user123", "127.0.0.1", "test-agent", "/api/test")

	// Check events were logged
	events := logger.GetEvents(nil)
	if len(events) != 4 {
		t.Errorf("Expected 4 events, got %d", len(events))
	}

	// Test filtering
	filter := &SecurityEventFilter{
		EventTypes: []SecurityEventType{SecurityEventAuthFailure},
	}

	filteredEvents := logger.GetEvents(filter)
	if len(filteredEvents) != 1 {
		t.Errorf("Expected 1 filtered event, got %d", len(filteredEvents))
	}

	if filteredEvents[0].EventType != SecurityEventAuthFailure {
		t.Error("Expected filtered event to be auth failure")
	}
}

func TestSecurityLoggerMiddleware(t *testing.T) {
	config := DefaultSecurityLoggerConfig()
	config.LogFile = "" // Disable file logging for test

	logger, err := NewSecurityLogger(config)
	if err != nil {
		t.Fatalf("Failed to create security logger: %v", err)
	}
	defer logger.Close()

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	// Wrap with security logger middleware
	middleware := SecurityLoggerMiddleware(logger)
	wrappedHandler := middleware(handler)

	// Create test request
	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Create response recorder
	rr := &responseWrapper{
		ResponseWriter: &mockResponseWriter{},
		statusCode:     200,
	}

	// Execute request
	wrappedHandler.ServeHTTP(rr, req)

	// Wait a bit for async logging
	time.Sleep(100 * time.Millisecond)

	// Check if security event was logged
	events := logger.GetEvents(nil)
	if len(events) == 0 {
		t.Error("Expected security event to be logged")
	}
}

// Mock response writer for testing
type mockResponseWriter struct {
	headers http.Header
	body    []byte
	code    int
}

func (m *mockResponseWriter) Header() http.Header {
	if m.headers == nil {
		m.headers = make(http.Header)
	}
	return m.headers
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	m.body = append(m.body, data...)
	return len(data), nil
}

func (m *mockResponseWriter) WriteHeader(code int) {
	m.code = code
}

func TestUsernameValidation(t *testing.T) {
	tests := []struct {
		username string
		expected bool
	}{
		{"validuser", true},
		{"valid_user", true},
		{"valid-user", true},
		{"valid.user", true},
		{"valid123", true},
		{"user@domain", false}, // @ not allowed
		{"user space", false},  // Space not allowed
		{"user#tag", false},    // # not allowed
		{"", false},            // Empty not allowed
	}

	for _, test := range tests {
		t.Run(test.username, func(t *testing.T) {
			result := isValidUsername(test.username)
			if result != test.expected {
				t.Errorf("Username %s: expected %v, got %v", test.username, test.expected, result)
			}
		})
	}
}

func TestTokenValidation(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{"dGVzdC10b2tlbi0xMjM0NTY3ODkwYWJjZGVmZw==", true}, // Valid base64
		{"test-token-1234567890abcdef", true},              // Valid hex-like
		{"token_with_underscores", true},                   // Valid with underscores
		{"token-with-hyphens", true},                       // Valid with hyphens
		{"token.with.dots", true},                          // Valid with dots
		{"token with spaces", false},                       // Spaces not allowed
		{"token#with#hash", false},                         // Hash not allowed
		{"token@with@at", false},                           // @ not allowed
		{"", false},                                        // Empty not allowed
	}

	for _, test := range tests {
		t.Run(test.token, func(t *testing.T) {
			result := isValidToken(test.token)
			if result != test.expected {
				t.Errorf("Token %s: expected %v, got %v", test.token, test.expected, result)
			}
		})
	}
}

func TestAPIKeyValidation(t *testing.T) {
	tests := []struct {
		apiKey   string
		expected bool
	}{
		{"api-key-1234567890abcdef", true}, // Valid API key
		{"apikey123", true},                // Valid simple API key
		{"api_key_with_underscores", true}, // Valid with underscores
		{"api.key.with.dots", true},        // Valid with dots
		{"api key with spaces", false},     // Spaces not allowed
		{"api@key", false},                 // @ not allowed
		{"api#key", false},                 // # not allowed
		{"", false},                        // Empty not allowed
	}

	for _, test := range tests {
		t.Run(test.apiKey, func(t *testing.T) {
			result := isValidAPIKey(test.apiKey)
			if result != test.expected {
				t.Errorf("API Key %s: expected %v, got %v", test.apiKey, test.expected, result)
			}
		})
	}
}
