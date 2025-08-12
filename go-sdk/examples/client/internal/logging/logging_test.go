package logging

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestRedactingHook_Fire(t *testing.T) {
	tests := []struct {
		name           string
		fields         logrus.Fields
		expectedFields map[string]string
	}{
		{
			name: "redacts Authorization header",
			fields: logrus.Fields{
				"Authorization": "Bearer sk-1234567890abcdefghij",
				"other":         "value",
			},
			expectedFields: map[string]string{
				"Authorization": "Bearer ***ghij",
				"other":         "value",
			},
		},
		{
			name: "redacts X-API-Key header",
			fields: logrus.Fields{
				"X-API-Key": "api_key_1234567890",
				"method":    "POST",
			},
			expectedFields: map[string]string{
				"X-API-Key": "***7890",
				"method":    "POST",
			},
		},
		{
			name: "redacts lowercase headers",
			fields: logrus.Fields{
				"authorization": "Bearer token123456789",
				"x-api-key":     "key987654321",
			},
			expectedFields: map[string]string{
				"authorization": "Bearer ***6789",
				"x-api-key":     "***4321",
			},
		},
		{
			name: "redacts fields containing sensitive keywords",
			fields: logrus.Fields{
				"password":    "supersecret",
				"api_key":     "myapikey123",
				"auth_token":  "token456789",
				"secret_data": "confidential",
				"normal":      "public",
			},
			expectedFields: map[string]string{
				"password":    "***cret",
				"api_key":     "***y123",
				"auth_token":  "***6789",
				"secret_data": "***tial",
				"normal":      "public",
			},
		},
		{
			name: "redacts nested headers map with interface values",
			fields: logrus.Fields{
				"headers": map[string]interface{}{
					"Authorization": "Bearer secret123",
					"Content-Type":  "application/json",
					"X-API-Key":     "apikey456",
				},
			},
			expectedFields: map[string]string{
				"Authorization": "Bearer ***t123",
				"Content-Type":  "application/json",
				"X-API-Key":     "***y456",
			},
		},
		{
			name: "handles short sensitive values",
			fields: logrus.Fields{
				"api_key": "short",
				"token":   "",
			},
			expectedFields: map[string]string{
				"api_key": "***",
				"token":   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := &RedactingHook{}
			entry := &logrus.Entry{
				Data: tt.fields,
			}
			
			err := hook.Fire(entry)
			assert.NoError(t, err)
			
			// Check specific fields based on test case
			if tt.name == "redacts nested headers map with interface values" {
				headers, ok := entry.Data["headers"].(map[string]interface{})
				assert.True(t, ok)
				for key, expected := range tt.expectedFields {
					assert.Equal(t, expected, headers[key])
				}
			} else {
				for key, expected := range tt.expectedFields {
					actual, ok := entry.Data[key]
					assert.True(t, ok, "Field %s not found", key)
					assert.Equal(t, expected, actual, "Field %s mismatch", key)
				}
			}
		})
	}
}

func TestRedactSensitiveValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "short value",
			input:    "abc",
			expected: "***",
		},
		{
			name:     "8 character value",
			input:    "12345678",
			expected: "***",
		},
		{
			name:     "long value shows last 4",
			input:    "this-is-a-long-api-key",
			expected: "***-key",
		},
		{
			name:     "Bearer token",
			input:    "Bearer sk-1234567890abcdef",
			expected: "Bearer ***cdef",
		},
		{
			name:     "Bearer with short token",
			input:    "Bearer short",
			expected: "Bearer ***",
		},
		{
			name:     "non-string value",
			input:    123,
			expected: "[REDACTED]",
		},
		{
			name:     "nil value",
			input:    nil,
			expected: "[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSensitiveValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInitialize(t *testing.T) {
	tests := []struct {
		name         string
		opts         Options
		expectedLevel logrus.Level
		expectJSON   bool
	}{
		{
			name: "default options",
			opts: Options{},
			expectedLevel: logrus.InfoLevel,
			expectJSON:   false,
		},
		{
			name: "debug level with JSON format",
			opts: Options{
				Level:  "debug",
				Format: "json",
			},
			expectedLevel: logrus.DebugLevel,
			expectJSON:   true,
		},
		{
			name: "error level with text format",
			opts: Options{
				Level:  "error",
				Format: "text",
			},
			expectedLevel: logrus.ErrorLevel,
			expectJSON:   false,
		},
		{
			name: "invalid level defaults to info",
			opts: Options{
				Level: "invalid",
			},
			expectedLevel: logrus.InfoLevel,
			expectJSON:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.opts.Output = &buf
			
			logger := Initialize(tt.opts)
			
			assert.Equal(t, tt.expectedLevel, logger.Level)
			
			// Check if redacting hook is added
			hasRedactingHook := false
			for _, hook := range logger.Hooks[logrus.InfoLevel] {
				if _, ok := hook.(*RedactingHook); ok {
					hasRedactingHook = true
					break
				}
			}
			assert.True(t, hasRedactingHook, "RedactingHook should be added")
			
			// Test formatter type
			if tt.expectJSON {
				_, ok := logger.Formatter.(*logrus.JSONFormatter)
				assert.True(t, ok, "Should use JSON formatter")
			} else {
				_, ok := logger.Formatter.(*logrus.TextFormatter)
				assert.True(t, ok, "Should use Text formatter")
			}
		})
	}
}

func TestSensitiveHeadersList(t *testing.T) {
	// Ensure all common sensitive headers are in the list
	expectedHeaders := []string{
		"Authorization",
		"X-API-Key",
		"X-Auth-Token",
		"Cookie",
		"Set-Cookie",
	}
	
	for _, header := range expectedHeaders {
		assert.Contains(t, SensitiveHeaders, header)
	}
}

func TestRedactionInIntegration(t *testing.T) {
	// Test the full logging pipeline with redaction
	var buf bytes.Buffer
	logger := Initialize(Options{
		Level:  "info",
		Format: "json",
		Output: &buf,
	})
	
	// Log with sensitive data
	logger.WithFields(logrus.Fields{
		"api_key":       "sk-1234567890abcdefghij",
		"Authorization": "Bearer secret-token-xyz",
		"normal_field":  "public-data",
		"headers": map[string]interface{}{
			"X-API-Key":    "header-api-key-123",
			"Content-Type": "application/json",
		},
	}).Info("Test message")
	
	output := buf.String()
	
	// Check that sensitive data is redacted
	assert.NotContains(t, output, "sk-1234567890abcdefghij")
	assert.NotContains(t, output, "secret-token-xyz")
	assert.NotContains(t, output, "header-api-key-123")
	
	// Check that redacted values are present
	assert.Contains(t, output, "***ghij")
	assert.Contains(t, output, "Bearer ***-xyz")
	assert.Contains(t, output, "***-123")
	
	// Check that normal data is not redacted
	assert.Contains(t, output, "public-data")
	assert.Contains(t, output, "application/json")
}