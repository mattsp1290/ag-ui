package logging

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected logrus.Level
	}{
		{
			name:     "debug level",
			config:   Config{Level: "debug", EnableCaller: true},
			expected: logrus.DebugLevel,
		},
		{
			name:     "info level",
			config:   Config{Level: "info", EnableCaller: false},
			expected: logrus.InfoLevel,
		},
		{
			name:     "warn level",
			config:   Config{Level: "warn", EnableCaller: false},
			expected: logrus.WarnLevel,
		},
		{
			name:     "error level",
			config:   Config{Level: "error", EnableCaller: false},
			expected: logrus.ErrorLevel,
		},
		{
			name:     "invalid level defaults to info",
			config:   Config{Level: "invalid", EnableCaller: false},
			expected: logrus.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := Init(tt.config)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, logger.Level)

			// Check if formatter is JSON
			_, ok := logger.Formatter.(*logrus.JSONFormatter)
			assert.True(t, ok, "Expected JSON formatter")

			// Check output is stdout
			assert.Equal(t, os.Stdout, logger.Out)
		})
	}
}

func TestJSONLogging(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer

	logger, err := Init(Config{Level: "info", EnableCaller: false})
	require.NoError(t, err)
	logger.SetOutput(&buf)

	// Log a test message
	logger.WithFields(logrus.Fields{
		"request_id": "test-123",
		"method":     "GET",
		"path":       "/test",
	}).Info("Test log message")

	// Parse JSON output
	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	// Verify structure
	assert.Equal(t, "info", logEntry["level"])
	assert.Equal(t, "Test log message", logEntry["msg"])
	assert.Equal(t, "test-123", logEntry["request_id"])
	assert.Equal(t, "GET", logEntry["method"])
	assert.Equal(t, "/test", logEntry["path"])
	assert.Contains(t, logEntry, "time")
}

func TestWithRequestFields(t *testing.T) {
	logger, err := Init(Config{Level: "info", EnableCaller: false})
	require.NoError(t, err)

	// Capture log output
	var buf bytes.Buffer
	logger.SetOutput(&buf)

	// Create entry with request fields
	entry := WithRequestFields(logger, "req-456", "POST", "/api/test", "192.168.1.1", "test-agent/1.0")
	entry.Info("Request processed")

	// Parse JSON output
	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	// Verify request fields
	assert.Equal(t, "req-456", logEntry["request_id"])
	assert.Equal(t, "POST", logEntry["method"])
	assert.Equal(t, "/api/test", logEntry["path"])
	assert.Equal(t, "192.168.1.1", logEntry["remote_ip"])
	assert.Equal(t, "test-agent/1.0", logEntry["user_agent"])
}
