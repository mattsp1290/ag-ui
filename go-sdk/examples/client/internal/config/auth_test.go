package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredentialResolver_Resolve(t *testing.T) {
	tests := []struct {
		name           string
		setupConfig    func(*Manager)
		setupEnv       func()
		cleanupEnv     func()
		expectedAPIKey string
		expectedHeader string
		expectedScheme string
	}{
		{
			name: "default values when nothing set",
			setupConfig: func(m *Manager) {
				m.config.APIKey = ""
			},
			setupEnv:       func() {},
			cleanupEnv:     func() {},
			expectedAPIKey: "",
			expectedHeader: "Authorization",
			expectedScheme: "Bearer",
		},
		{
			name: "config file values",
			setupConfig: func(m *Manager) {
				m.config.APIKey = "config-key-123"
				m.config.AuthHeader = "X-API-Key"
				m.config.AuthScheme = ""
			},
			setupEnv:       func() {},
			cleanupEnv:     func() {},
			expectedAPIKey: "config-key-123",
			expectedHeader: "X-API-Key",
			expectedScheme: "",
		},
		{
			name: "environment overrides config",
			setupConfig: func(m *Manager) {
				m.config.APIKey = "config-key-123"
				m.config.AuthHeader = "X-API-Key"
			},
			setupEnv: func() {
				os.Setenv("AGUI_API_KEY", "env-key-456")
				os.Setenv("AGUI_AUTH_HEADER", "Authorization")
				os.Setenv("AGUI_AUTH_SCHEME", "Token")
			},
			cleanupEnv: func() {
				os.Unsetenv("AGUI_API_KEY")
				os.Unsetenv("AGUI_AUTH_HEADER")
				os.Unsetenv("AGUI_AUTH_SCHEME")
			},
			expectedAPIKey: "env-key-456",
			expectedHeader: "Authorization",
			expectedScheme: "Token",
		},
		{
			name: "flags override everything",
			setupConfig: func(m *Manager) {
				m.config.APIKey = "config-key-123"
				m.config.AuthHeader = "X-API-Key"
			},
			setupEnv: func() {
				os.Setenv("AGUI_API_KEY", "env-key-456")
			},
			cleanupEnv: func() {
				os.Unsetenv("AGUI_API_KEY")
			},
			expectedAPIKey: "flag-key-789",
			expectedHeader: "Authorization",
			expectedScheme: "Basic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			manager := NewManager()
			manager.setDefaults()
			
			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			if tt.cleanupEnv != nil {
				defer tt.cleanupEnv()
			}
			
			if tt.setupConfig != nil {
				tt.setupConfig(manager)
			}
			
			// Apply environment if needed
			if tt.name == "environment overrides config" || tt.name == "flags override everything" {
				manager.applyEnvironment()
			}
			
			// Apply flags for the flags test case
			if tt.name == "flags override everything" {
				manager.ApplyFlags(map[string]string{
					"api-key":     "flag-key-789",
					"auth-header": "Authorization",
					"auth-scheme": "Basic",
				})
			}
			
			// Test
			resolver := NewCredentialResolver(manager)
			authConfig, err := resolver.Resolve()
			
			require.NoError(t, err)
			assert.Equal(t, tt.expectedAPIKey, authConfig.APIKey)
			assert.Equal(t, tt.expectedHeader, authConfig.AuthHeader)
			assert.Equal(t, tt.expectedScheme, authConfig.AuthScheme)
		})
	}
}

func TestAuthConfig_GetAuthorizationHeader(t *testing.T) {
	tests := []struct {
		name           string
		authConfig     AuthConfig
		expectedHeader string
		expectedValue  string
	}{
		{
			name: "no API key returns empty",
			authConfig: AuthConfig{
				APIKey:     "",
				AuthHeader: "Authorization",
				AuthScheme: "Bearer",
			},
			expectedHeader: "",
			expectedValue:  "",
		},
		{
			name: "Authorization with Bearer",
			authConfig: AuthConfig{
				APIKey:     "test-key-123",
				AuthHeader: "Authorization",
				AuthScheme: "Bearer",
			},
			expectedHeader: "Authorization",
			expectedValue:  "Bearer test-key-123",
		},
		{
			name: "Authorization with Basic",
			authConfig: AuthConfig{
				APIKey:     "dXNlcjpwYXNz",
				AuthHeader: "Authorization",
				AuthScheme: "Basic",
			},
			expectedHeader: "Authorization",
			expectedValue:  "Basic dXNlcjpwYXNz",
		},
		{
			name: "X-API-Key header",
			authConfig: AuthConfig{
				APIKey:     "api-key-456",
				AuthHeader: "X-API-Key",
				AuthScheme: "Bearer", // Should be ignored for X-API-Key
			},
			expectedHeader: "X-API-Key",
			expectedValue:  "api-key-456",
		},
		{
			name: "default header when empty",
			authConfig: AuthConfig{
				APIKey:     "test-key-789",
				AuthHeader: "",
				AuthScheme: "Bearer",
			},
			expectedHeader: "Authorization",
			expectedValue:  "Bearer test-key-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header, value := tt.authConfig.GetAuthorizationHeader()
			assert.Equal(t, tt.expectedHeader, header)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestRedactAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{
			name:     "empty key",
			apiKey:   "",
			expected: "",
		},
		{
			name:     "short key",
			apiKey:   "abc",
			expected: "***",
		},
		{
			name:     "8 char key",
			apiKey:   "12345678",
			expected: "***",
		},
		{
			name:     "long key shows last 4",
			apiKey:   "sk-1234567890abcdefghij",
			expected: "***ghij",
		},
		{
			name:     "UUID format",
			apiKey:   "550e8400-e29b-41d4-a716-446655440000",
			expected: "***0000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactAPIKey(tt.apiKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateAuthConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      AuthConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid Authorization header",
			config: AuthConfig{
				APIKey:     "test-key",
				AuthHeader: "Authorization",
				AuthScheme: "Bearer",
			},
			expectError: false,
		},
		{
			name: "valid X-API-Key header",
			config: AuthConfig{
				APIKey:     "test-key",
				AuthHeader: "X-API-Key",
				AuthScheme: "",
			},
			expectError: false,
		},
		{
			name: "header with whitespace",
			config: AuthConfig{
				APIKey:     "test-key",
				AuthHeader: "Auth Header",
				AuthScheme: "Bearer",
			},
			expectError: true,
			errorMsg:    "invalid auth header name: contains whitespace",
		},
		{
			name: "header with colon",
			config: AuthConfig{
				APIKey:     "test-key",
				AuthHeader: "Auth:Header",
				AuthScheme: "Bearer",
			},
			expectError: true,
			errorMsg:    "invalid auth header name: contains colon",
		},
		{
			name: "Authorization without scheme gets default",
			config: AuthConfig{
				APIKey:     "test-key",
				AuthHeader: "Authorization",
				AuthScheme: "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.config // Make a copy
			err := ValidateAuthConfig(&config)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Equal(t, tt.errorMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
				// Check if default scheme was applied
				if tt.config.AuthHeader == "Authorization" && tt.config.AuthScheme == "" && tt.config.APIKey != "" {
					assert.Equal(t, "Bearer", config.AuthScheme)
				}
			}
		})
	}
}

func TestAuthConfigPrecedence(t *testing.T) {
	// Save original env vars
	originalAPIKey := os.Getenv("AGUI_API_KEY")
	originalAuthHeader := os.Getenv("AGUI_AUTH_HEADER")
	defer func() {
		if originalAPIKey != "" {
			os.Setenv("AGUI_API_KEY", originalAPIKey)
		} else {
			os.Unsetenv("AGUI_API_KEY")
		}
		if originalAuthHeader != "" {
			os.Setenv("AGUI_AUTH_HEADER", originalAuthHeader)
		} else {
			os.Unsetenv("AGUI_AUTH_HEADER")
		}
	}()

	// Test precedence: flag > env > config > default
	manager := NewManager()
	
	// 1. Start with defaults
	manager.setDefaults()
	assert.Equal(t, "Authorization", manager.config.AuthHeader)
	assert.Equal(t, "Bearer", manager.config.AuthScheme)
	
	// 2. Config file values (simulated)
	manager.config.APIKey = "config-key"
	manager.config.AuthHeader = "X-API-Key"
	assert.Equal(t, "config-key", manager.config.APIKey)
	assert.Equal(t, "X-API-Key", manager.config.AuthHeader)
	
	// 3. Environment overrides config
	os.Setenv("AGUI_API_KEY", "env-key")
	os.Setenv("AGUI_AUTH_HEADER", "Authorization")
	manager.applyEnvironment()
	assert.Equal(t, "env-key", manager.config.APIKey)
	assert.Equal(t, "Authorization", manager.config.AuthHeader)
	
	// 4. Flags override everything
	manager.ApplyFlags(map[string]string{
		"api-key":     "flag-key",
		"auth-header": "X-Custom-Auth",
	})
	assert.Equal(t, "flag-key", manager.config.APIKey)
	assert.Equal(t, "X-Custom-Auth", manager.config.AuthHeader)
}