package testhelper

import (
	"os"
	"strconv"
	"time"
)

// TimeoutConfig provides configurable timeout values for tests
type TimeoutConfig struct {
	// Short timeout for quick operations (default: 5s)
	Short time.Duration
	// Medium timeout for moderate operations (default: 10s)
	Medium time.Duration
	// Long timeout for extended operations (default: 30s, only for exceptional cases)
	Long time.Duration
	// Context timeout for context-based operations (default: 8s)
	Context time.Duration
	// Network timeout for network operations (default: 6s)
	Network time.Duration
	// Cleanup timeout for resource cleanup (default: 3s)
	Cleanup time.Duration
}

// DefaultTimeouts returns the default timeout configuration
func DefaultTimeouts() *TimeoutConfig {
	return &TimeoutConfig{
		Short:   5 * time.Second,
		Medium:  10 * time.Second,
		Long:    30 * time.Second, // Only for exceptional cases
		Context: 8 * time.Second,
		Network: 6 * time.Second,
		Cleanup: 3 * time.Second,
	}
}

// NewTimeoutConfig creates a timeout configuration with environment variable overrides
func NewTimeoutConfig() *TimeoutConfig {
	config := DefaultTimeouts()
	
	// Allow environment variable overrides
	if val := os.Getenv("TEST_TIMEOUT_SHORT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			config.Short = d
		}
	}
	
	if val := os.Getenv("TEST_TIMEOUT_MEDIUM"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			config.Medium = d
		}
	}
	
	if val := os.Getenv("TEST_TIMEOUT_LONG"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			config.Long = d
		}
	}
	
	if val := os.Getenv("TEST_TIMEOUT_CONTEXT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			config.Context = d
		}
	}
	
	if val := os.Getenv("TEST_TIMEOUT_NETWORK"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			config.Network = d
		}
	}
	
	if val := os.Getenv("TEST_TIMEOUT_CLEANUP"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			config.Cleanup = d
		}
	}
	
	// Global scale factor for CI environments
	if val := os.Getenv("TEST_TIMEOUT_SCALE"); val != "" {
		if scale, err := strconv.ParseFloat(val, 64); err == nil && scale > 0 {
			config.Short = time.Duration(float64(config.Short) * scale)
			config.Medium = time.Duration(float64(config.Medium) * scale)
			config.Long = time.Duration(float64(config.Long) * scale)
			config.Context = time.Duration(float64(config.Context) * scale)
			config.Network = time.Duration(float64(config.Network) * scale)
			config.Cleanup = time.Duration(float64(config.Cleanup) * scale)
		}
	}
	
	return config
}

// Global timeout configuration instance
var GlobalTimeouts = NewTimeoutConfig()

// SetGlobalTimeouts allows setting a custom global timeout configuration
func SetGlobalTimeouts(config *TimeoutConfig) {
	GlobalTimeouts = config
}

// FastTimeouts returns a configuration optimized for speed (shorter timeouts)
func FastTimeouts() *TimeoutConfig {
	return &TimeoutConfig{
		Short:   2 * time.Second,
		Medium:  5 * time.Second,
		Long:    10 * time.Second,
		Context: 4 * time.Second,
		Network: 3 * time.Second,
		Cleanup: 1 * time.Second,
	}
}

// SlowTimeouts returns a configuration for slower environments (longer timeouts)
func SlowTimeouts() *TimeoutConfig {
	return &TimeoutConfig{
		Short:   10 * time.Second,
		Medium:  20 * time.Second,
		Long:    60 * time.Second,
		Context: 15 * time.Second,
		Network: 12 * time.Second,
		Cleanup: 5 * time.Second,
	}
}

// GetTimeout returns appropriate timeout based on the operation type
func (tc *TimeoutConfig) GetTimeout(operation string) time.Duration {
	switch operation {
	case "short", "quick", "immediate":
		return tc.Short
	case "medium", "moderate", "normal":
		return tc.Medium
	case "long", "extended", "slow":
		return tc.Long
	case "context", "ctx":
		return tc.Context
	case "network", "net", "connection":
		return tc.Network
	case "cleanup", "close", "shutdown":
		return tc.Cleanup
	default:
		return tc.Medium // Default to medium timeout
	}
}

// WithScale returns a new TimeoutConfig scaled by the given factor
func (tc *TimeoutConfig) WithScale(scale float64) *TimeoutConfig {
	return &TimeoutConfig{
		Short:   time.Duration(float64(tc.Short) * scale),
		Medium:  time.Duration(float64(tc.Medium) * scale),
		Long:    time.Duration(float64(tc.Long) * scale),
		Context: time.Duration(float64(tc.Context) * scale),
		Network: time.Duration(float64(tc.Network) * scale),
		Cleanup: time.Duration(float64(tc.Cleanup) * scale),
	}
}

// IsCI detects if running in a CI environment
func IsCI() bool {
	return os.Getenv("CI") != "" || 
		   os.Getenv("GITHUB_ACTIONS") != "" ||
		   os.Getenv("JENKINS_URL") != "" ||
		   os.Getenv("TRAVIS") != "" ||
		   os.Getenv("CIRCLECI") != ""
}

// GetCITimeouts returns timeouts appropriate for CI environments
func GetCITimeouts() *TimeoutConfig {
	if IsCI() {
		// CI environments may be slower, scale up by 1.5x
		return GlobalTimeouts.WithScale(1.5)
	}
	return GlobalTimeouts
}

// TimeoutOption represents a function for configuring timeouts
type TimeoutOption func(*TimeoutConfig)

// WithShortTimeout sets the short timeout
func WithShortTimeout(d time.Duration) TimeoutOption {
	return func(tc *TimeoutConfig) {
		tc.Short = d
	}
}

// WithMediumTimeout sets the medium timeout
func WithMediumTimeout(d time.Duration) TimeoutOption {
	return func(tc *TimeoutConfig) {
		tc.Medium = d
	}
}

// WithNetworkTimeout sets the network timeout
func WithNetworkTimeout(d time.Duration) TimeoutOption {
	return func(tc *TimeoutConfig) {
		tc.Network = d
	}
}

// WithCleanupTimeout sets the cleanup timeout
func WithCleanupTimeout(d time.Duration) TimeoutOption {
	return func(tc *TimeoutConfig) {
		tc.Cleanup = d
	}
}

// NewCustomTimeouts creates a TimeoutConfig with custom options
func NewCustomTimeouts(options ...TimeoutOption) *TimeoutConfig {
	config := DefaultTimeouts()
	for _, opt := range options {
		opt(config)
	}
	return config
}