package config

import (
	"flag"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	config := New()

	assert.Equal(t, DefaultHost, config.Host)
	assert.Equal(t, DefaultPort, config.Port)
	assert.Equal(t, DefaultLogLevel, config.LogLevel)
	assert.Equal(t, DefaultEnableSSE, config.EnableSSE)
	assert.Equal(t, DefaultReadTimeout, config.ReadTimeout)
	assert.Equal(t, DefaultWriteTimeout, config.WriteTimeout)
	assert.Equal(t, DefaultSSEKeepAlive, config.SSEKeepAlive)
	assert.Equal(t, DefaultCORSEnabled, config.CORSEnabled)
}

func TestLoadFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
		wantErr  bool
	}{
		{
			name: "valid environment variables",
			envVars: map[string]string{
				"AGUI_HOST":          "127.0.0.1",
				"AGUI_PORT":          "9090",
				"AGUI_LOG_LEVEL":     "DEBUG",
				"AGUI_ENABLE_SSE":    "false",
				"AGUI_READ_TIMEOUT":  "60s",
				"AGUI_WRITE_TIMEOUT": "45s",
				"AGUI_SSE_KEEPALIVE": "30s",
				"AGUI_CORS_ENABLED":  "false",
			},
			expected: &Config{
				Host:         "127.0.0.1",
				Port:         9090,
				LogLevel:     "debug",
				EnableSSE:    false,
				ReadTimeout:  60 * time.Second,
				WriteTimeout: 45 * time.Second,
				SSEKeepAlive: 30 * time.Second,
				CORSEnabled:  false,
			},
			wantErr: false,
		},
		{
			name: "invalid port",
			envVars: map[string]string{
				"AGUI_PORT": "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid boolean for SSE",
			envVars: map[string]string{
				"AGUI_ENABLE_SSE": "maybe",
			},
			wantErr: true,
		},
		{
			name: "invalid duration",
			envVars: map[string]string{
				"AGUI_READ_TIMEOUT": "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment
			originalEnv := make(map[string]string)
			for key := range tt.envVars {
				originalEnv[key] = os.Getenv(key)
			}

			// Set test environment variables
			for key, value := range tt.envVars {
				if err := os.Setenv(key, value); err != nil {
					t.Fatalf("Failed to set environment variable %s: %v", key, err)
				}
			}

			// Restore environment after test
			defer func() {
				for key := range tt.envVars {
					if originalValue, exists := originalEnv[key]; exists {
						if err := os.Setenv(key, originalValue); err != nil {
							t.Errorf("Failed to restore environment variable %s: %v", key, err)
						}
					} else {
						if err := os.Unsetenv(key); err != nil {
							t.Errorf("Failed to unset environment variable %s: %v", key, err)
						}
					}
				}
			}()

			config := New()
			err := config.LoadFromEnv()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expected != nil {
					assert.Equal(t, tt.expected.Host, config.Host)
					assert.Equal(t, tt.expected.Port, config.Port)
					assert.Equal(t, tt.expected.LogLevel, config.LogLevel)
					assert.Equal(t, tt.expected.EnableSSE, config.EnableSSE)
					assert.Equal(t, tt.expected.ReadTimeout, config.ReadTimeout)
					assert.Equal(t, tt.expected.WriteTimeout, config.WriteTimeout)
					assert.Equal(t, tt.expected.SSEKeepAlive, config.SSEKeepAlive)
					assert.Equal(t, tt.expected.CORSEnabled, config.CORSEnabled)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  New(),
			wantErr: false,
		},
		{
			name: "invalid port - too low",
			config: &Config{
				Host:         DefaultHost,
				Port:         0,
				LogLevel:     DefaultLogLevel,
				EnableSSE:    DefaultEnableSSE,
				ReadTimeout:  DefaultReadTimeout,
				WriteTimeout: DefaultWriteTimeout,
				SSEKeepAlive: DefaultSSEKeepAlive,
				CORSEnabled:  DefaultCORSEnabled,
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: &Config{
				Host:         DefaultHost,
				Port:         65536,
				LogLevel:     DefaultLogLevel,
				EnableSSE:    DefaultEnableSSE,
				ReadTimeout:  DefaultReadTimeout,
				WriteTimeout: DefaultWriteTimeout,
				SSEKeepAlive: DefaultSSEKeepAlive,
				CORSEnabled:  DefaultCORSEnabled,
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			config: &Config{
				Host:         DefaultHost,
				Port:         DefaultPort,
				LogLevel:     "invalid",
				EnableSSE:    DefaultEnableSSE,
				ReadTimeout:  DefaultReadTimeout,
				WriteTimeout: DefaultWriteTimeout,
				SSEKeepAlive: DefaultSSEKeepAlive,
				CORSEnabled:  DefaultCORSEnabled,
			},
			wantErr: true,
		},
		{
			name: "negative read timeout",
			config: &Config{
				Host:         DefaultHost,
				Port:         DefaultPort,
				LogLevel:     DefaultLogLevel,
				EnableSSE:    DefaultEnableSSE,
				ReadTimeout:  -1 * time.Second,
				WriteTimeout: DefaultWriteTimeout,
				SSEKeepAlive: DefaultSSEKeepAlive,
				CORSEnabled:  DefaultCORSEnabled,
			},
			wantErr: true,
		},
		{
			name: "negative write timeout",
			config: &Config{
				Host:         DefaultHost,
				Port:         DefaultPort,
				LogLevel:     DefaultLogLevel,
				EnableSSE:    DefaultEnableSSE,
				ReadTimeout:  DefaultReadTimeout,
				WriteTimeout: -1 * time.Second,
				SSEKeepAlive: DefaultSSEKeepAlive,
				CORSEnabled:  DefaultCORSEnabled,
			},
			wantErr: true,
		},
		{
			name: "negative SSE keep-alive",
			config: &Config{
				Host:         DefaultHost,
				Port:         DefaultPort,
				LogLevel:     DefaultLogLevel,
				EnableSSE:    DefaultEnableSSE,
				ReadTimeout:  DefaultReadTimeout,
				WriteTimeout: DefaultWriteTimeout,
				SSEKeepAlive: -1 * time.Second,
				CORSEnabled:  DefaultCORSEnabled,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		expected slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"invalid level fallback", "invalid", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{LogLevel: tt.logLevel}
			level := config.GetLogLevel()
			assert.Equal(t, tt.expected, level)
		})
	}
}

func TestLoadConfig_Precedence(t *testing.T) {
	// Clear any existing flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	// Save original environment
	originalEnv := map[string]string{
		"AGUI_HOST": os.Getenv("AGUI_HOST"),
		"AGUI_PORT": os.Getenv("AGUI_PORT"),
	}

	// Restore environment after test
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				if err := os.Setenv(key, value); err != nil {
					t.Errorf("Failed to restore environment variable %s: %v", key, err)
				}
			} else {
				if err := os.Unsetenv(key); err != nil {
					t.Errorf("Failed to unset environment variable %s: %v", key, err)
				}
			}
		}
	}()

	// Set environment variables
	if err := os.Setenv("AGUI_HOST", "env-host"); err != nil {
		t.Fatalf("Failed to set AGUI_HOST: %v", err)
	}
	if err := os.Setenv("AGUI_PORT", "9999"); err != nil {
		t.Fatalf("Failed to set AGUI_PORT: %v", err)
	}

	// Set up command line arguments (flags should override env)
	oldArgs := os.Args
	os.Args = []string{"test", "-host", "flag-host", "-port", "8888"}
	defer func() { os.Args = oldArgs }()

	config, err := LoadConfig()
	require.NoError(t, err)

	// Flags should take precedence over environment variables
	assert.Equal(t, "flag-host", config.Host)
	assert.Equal(t, 8888, config.Port)
}

func TestLoadConfig_ValidationError(t *testing.T) {
	// Clear any existing flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	// Set up invalid command line arguments
	oldArgs := os.Args
	os.Args = []string{"test", "-port", "70000"} // Invalid port
	defer func() { os.Args = oldArgs }()

	config, err := LoadConfig()
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "configuration validation failed")
}
