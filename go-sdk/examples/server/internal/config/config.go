package config

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all server configuration values
type Config struct {
	// Server settings
	Host string
	Port int

	// Logging
	LogLevel string

	// Transport settings
	EnableSSE bool

	// Timeout settings
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	SSEKeepAlive time.Duration

	// CORS settings
	CORSEnabled        bool
	CORSAllowedOrigins []string
}

// Default configuration values
const (
	DefaultHost         = "0.0.0.0"
	DefaultPort         = 8080
	DefaultLogLevel     = "info"
	DefaultEnableSSE    = true
	DefaultReadTimeout  = 30 * time.Second
	DefaultWriteTimeout = 30 * time.Second
	DefaultSSEKeepAlive = 15 * time.Second
	DefaultCORSEnabled  = true
)

// Default CORS allowed origins (secure by default)
var DefaultCORSAllowedOrigins = []string{"http://localhost:3000", "http://localhost:8080"}

// Valid log levels
var ValidLogLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// New creates a new Config with default values
func New() *Config {
	return &Config{
		Host:               DefaultHost,
		Port:               DefaultPort,
		LogLevel:           DefaultLogLevel,
		EnableSSE:          DefaultEnableSSE,
		ReadTimeout:        DefaultReadTimeout,
		WriteTimeout:       DefaultWriteTimeout,
		SSEKeepAlive:       DefaultSSEKeepAlive,
		CORSEnabled:        DefaultCORSEnabled,
		CORSAllowedOrigins: DefaultCORSAllowedOrigins,
	}
}

// LoadFromEnv loads configuration from environment variables with AGUI_ prefix
func (c *Config) LoadFromEnv() error {
	if host := os.Getenv("AGUI_HOST"); host != "" {
		c.Host = host
	}

	if portStr := os.Getenv("AGUI_PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid AGUI_PORT value '%s': %w", portStr, err)
		}
		c.Port = port
	}

	if logLevel := os.Getenv("AGUI_LOG_LEVEL"); logLevel != "" {
		c.LogLevel = strings.ToLower(logLevel)
	}

	if enableSSEStr := os.Getenv("AGUI_ENABLE_SSE"); enableSSEStr != "" {
		enableSSE, err := strconv.ParseBool(enableSSEStr)
		if err != nil {
			return fmt.Errorf("invalid AGUI_ENABLE_SSE value '%s': %w", enableSSEStr, err)
		}
		c.EnableSSE = enableSSE
	}

	if readTimeoutStr := os.Getenv("AGUI_READ_TIMEOUT"); readTimeoutStr != "" {
		readTimeout, err := time.ParseDuration(readTimeoutStr)
		if err != nil {
			return fmt.Errorf("invalid AGUI_READ_TIMEOUT value '%s': %w", readTimeoutStr, err)
		}
		c.ReadTimeout = readTimeout
	}

	if writeTimeoutStr := os.Getenv("AGUI_WRITE_TIMEOUT"); writeTimeoutStr != "" {
		writeTimeout, err := time.ParseDuration(writeTimeoutStr)
		if err != nil {
			return fmt.Errorf("invalid AGUI_WRITE_TIMEOUT value '%s': %w", writeTimeoutStr, err)
		}
		c.WriteTimeout = writeTimeout
	}

	if sseKeepAliveStr := os.Getenv("AGUI_SSE_KEEPALIVE"); sseKeepAliveStr != "" {
		sseKeepAlive, err := time.ParseDuration(sseKeepAliveStr)
		if err != nil {
			return fmt.Errorf("invalid AGUI_SSE_KEEPALIVE value '%s': %w", sseKeepAliveStr, err)
		}
		c.SSEKeepAlive = sseKeepAlive
	}

	if corsEnabledStr := os.Getenv("AGUI_CORS_ENABLED"); corsEnabledStr != "" {
		corsEnabled, err := strconv.ParseBool(corsEnabledStr)
		if err != nil {
			return fmt.Errorf("invalid AGUI_CORS_ENABLED value '%s': %w", corsEnabledStr, err)
		}
		c.CORSEnabled = corsEnabled
	}

	if corsOriginsStr := os.Getenv("AGUI_CORS_ORIGINS"); corsOriginsStr != "" {
		origins := strings.Split(corsOriginsStr, ",")
		for i, origin := range origins {
			origins[i] = strings.TrimSpace(origin)
		}
		c.CORSAllowedOrigins = origins
	}

	return nil
}

// Validate validates the configuration values
func (c *Config) Validate() error {
	var errs []error

	// Validate port range
	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("port must be between 1 and 65535, got %d", c.Port))
	}

	// Validate log level
	if _, ok := ValidLogLevels[c.LogLevel]; !ok {
		validLevels := make([]string, 0, len(ValidLogLevels))
		for level := range ValidLogLevels {
			validLevels = append(validLevels, level)
		}
		errs = append(errs, fmt.Errorf("invalid log level '%s', must be one of: %s", c.LogLevel, strings.Join(validLevels, ", ")))
	}

	// Validate timeout durations are non-negative
	if c.ReadTimeout < 0 {
		errs = append(errs, fmt.Errorf("read timeout must be non-negative, got %v", c.ReadTimeout))
	}

	if c.WriteTimeout < 0 {
		errs = append(errs, fmt.Errorf("write timeout must be non-negative, got %v", c.WriteTimeout))
	}

	if c.SSEKeepAlive < 0 {
		errs = append(errs, fmt.Errorf("SSE keep-alive must be non-negative, got %v", c.SSEKeepAlive))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// LogLevel returns the slog.Level for the configured log level
func (c *Config) GetLogLevel() slog.Level {
	level, ok := ValidLogLevels[c.LogLevel]
	if !ok {
		return slog.LevelInfo // fallback to info if invalid
	}
	return level
}

// LoadFromFlags loads configuration from command line flags with precedence over env vars
func (c *Config) LoadFromFlags() error {
	var (
		host         = flag.String("host", c.Host, "Server host address")
		port         = flag.Int("port", c.Port, "Server port (1-65535)")
		logLevel     = flag.String("log-level", c.LogLevel, "Log level (debug, info, warn, error)")
		enableSSE    = flag.Bool("enable-sse", c.EnableSSE, "Enable Server-Sent Events")
		readTimeout  = flag.Duration("read-timeout", c.ReadTimeout, "Read timeout duration")
		writeTimeout = flag.Duration("write-timeout", c.WriteTimeout, "Write timeout duration")
		sseKeepAlive = flag.Duration("sse-keepalive", c.SSEKeepAlive, "SSE keep-alive duration")
		corsEnabled  = flag.Bool("cors-enabled", c.CORSEnabled, "Enable CORS")
	)

	flag.Parse()

	// Apply flag values with precedence over env vars
	c.Host = *host
	c.Port = *port
	c.LogLevel = strings.ToLower(*logLevel)
	c.EnableSSE = *enableSSE
	c.ReadTimeout = *readTimeout
	c.WriteTimeout = *writeTimeout
	c.SSEKeepAlive = *sseKeepAlive
	c.CORSEnabled = *corsEnabled

	return nil
}

// LoadConfig creates and loads configuration with proper precedence: flags > env > defaults
func LoadConfig() (*Config, error) {
	// Start with defaults
	config := New()

	// Load environment variables (override defaults)
	if err := config.LoadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Load command line flags (override env vars)
	if err := config.LoadFromFlags(); err != nil {
		return nil, fmt.Errorf("failed to load command line flags: %w", err)
	}

	// Validate the final configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// LogSafeConfig logs the configuration without sensitive information
func (c *Config) LogSafeConfig(logger *slog.Logger) {
	logger.Info("Server configuration loaded",
		"host", c.Host,
		"port", c.Port,
		"log_level", c.LogLevel,
		"enable_sse", c.EnableSSE,
		"read_timeout", c.ReadTimeout,
		"write_timeout", c.WriteTimeout,
		"sse_keepalive", c.SSEKeepAlive,
		"cors_enabled", c.CORSEnabled,
	)
}
