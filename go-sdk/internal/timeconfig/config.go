package timeconfig

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TimeConfig holds all time-based configuration values
type TimeConfig struct {
	// State Management Timeouts
	DefaultShutdownTimeout         time.Duration
	DefaultUpdateTimeout           time.Duration
	DefaultRetryDelay              time.Duration
	DefaultEventRetryBackoff       time.Duration
	DefaultBatchTimeout            time.Duration
	DefaultPerformanceBatchTimeout time.Duration
	DefaultMetricsInterval         time.Duration
	DefaultCheckpointInterval      time.Duration
	DefaultContextTTL              time.Duration
	DefaultCleanupInterval         time.Duration
	DefaultSubscriptionTTL         time.Duration
	DefaultSubscriptionCleanup     time.Duration
	DefaultShutdownGracePeriod     time.Duration
	DefaultCacheTTL                time.Duration

	// Transport/WebSocket Timeouts
	DefaultDialTimeout             time.Duration
	DefaultHandshakeTimeout        time.Duration
	DefaultReadTimeout             time.Duration
	DefaultWriteTimeout            time.Duration
	DefaultPingPeriod              time.Duration
	DefaultPongTimeout             time.Duration
	DefaultInitialReconnectDelay   time.Duration
	DefaultMaxReconnectDelay       time.Duration
	DefaultAuthTimeout             time.Duration
	DefaultHeartbeatTimeout        time.Duration

	// Performance Monitoring
	DefaultProfilingInterval       time.Duration
	DefaultMaxLatency              time.Duration
	DefaultMessageBatchTimeout     time.Duration

	// Tools/HTTP Timeouts
	DefaultHTTPTimeout             time.Duration
	DefaultToolExecutionTimeout    time.Duration

	// Test/Development Timeouts
	DefaultTestTimeout             time.Duration
	DefaultMockLatency             time.Duration

	// Health and Monitoring
	DefaultHealthCheckInterval     time.Duration
	DefaultHealthCheckTimeout      time.Duration

	// Storage and I/O
	DefaultStorageTimeout          time.Duration
	DefaultIOTimeout               time.Duration
	DefaultValidationTimeout       time.Duration
	DefaultSecurityCheckTimeout    time.Duration
	DefaultAuditWriteTimeout       time.Duration
	DefaultCryptoOperationTimeout  time.Duration

	// Network Settings
	DefaultTCPKeepAlive            time.Duration
	DefaultIdleConnectionTimeout   time.Duration

	// Cleanup and Maintenance
	DefaultCleanupWorkerInterval   time.Duration
	DefaultExpiredCleanupInterval  time.Duration
	DefaultMaintenanceInterval     time.Duration

	// Circuit Breaker and Error Handling
	DefaultErrorCountWindow        time.Duration
	DefaultErrorResetInterval      time.Duration
	DefaultAlertCooldown           time.Duration
	DefaultDuplicateAlertWindow    time.Duration

	// Performance and GC
	DefaultGCMonitoringInterval    time.Duration
	DefaultResourceSampleInterval  time.Duration
	DefaultMemoryMonitoringInterval time.Duration
	DefaultPerformanceMetricsInterval time.Duration

	// Retention Policies
	DefaultMetricsRetention        time.Duration
	DefaultAuditLogRetention       time.Duration
	DefaultTraceRetention          time.Duration

	// CPU/Memory Profiling
	DefaultCPUProfileInterval      time.Duration
	DefaultMemoryProfileInterval   time.Duration
}

var (
	globalConfig *TimeConfig
	configMutex  sync.RWMutex
	once         sync.Once
)

// IsTestMode determines if we're running in test mode
func IsTestMode() bool {
	// Check for standard Go test flag
	if isGoTest() {
		return true
	}

	// Check for custom environment variables
	if val := os.Getenv("AG_SDK_TEST_MODE"); val != "" {
		if testMode, err := strconv.ParseBool(val); err == nil {
			return testMode
		}
	}

	// Check for CI environment variables
	if os.Getenv("CI") != "" || os.Getenv("AG_SDK_CI") != "" {
		return true
	}

	return false
}

// isGoTest checks if we're running under 'go test'
func isGoTest() bool {
	// Check if test.Main is available (indicates we're in a test)
	for _, arg := range os.Args {
		if strings.Contains(arg, "test") || strings.HasSuffix(arg, ".test") {
			return true
		}
	}
	
	// Check for test-specific environment variables
	if os.Getenv("GO_TEST") != "" {
		return true
	}

	return false
}

// GetConfig returns the global time configuration
func GetConfig() *TimeConfig {
	once.Do(func() {
		globalConfig = createConfig()
	})
	
	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalConfig
}

// SetConfig allows overriding the global configuration (mainly for tests)
func SetConfig(config *TimeConfig) {
	configMutex.Lock()
	defer configMutex.Unlock()
	globalConfig = config
}

// ResetConfig resets the configuration to defaults
func ResetConfig() {
	configMutex.Lock()
	defer configMutex.Unlock()
	globalConfig = createConfig()
}

// createConfig creates a new TimeConfig based on the current environment
func createConfig() *TimeConfig {
	if IsTestMode() {
		return createTestConfig()
	}
	return createProductionConfig()
}

// createProductionConfig returns production-appropriate timeouts
func createProductionConfig() *TimeConfig {
	return &TimeConfig{
		// State Management Timeouts (production values)
		DefaultShutdownTimeout:         30 * time.Second,
		DefaultUpdateTimeout:           30 * time.Second,
		DefaultRetryDelay:              100 * time.Millisecond,
		DefaultEventRetryBackoff:       time.Second,
		DefaultBatchTimeout:            100 * time.Millisecond,
		DefaultPerformanceBatchTimeout: 10 * time.Millisecond,
		DefaultMetricsInterval:         30 * time.Second,
		DefaultCheckpointInterval:      5 * time.Minute,
		DefaultContextTTL:              1 * time.Hour,
		DefaultCleanupInterval:         15 * time.Minute,
		DefaultSubscriptionTTL:         1 * time.Hour,
		DefaultSubscriptionCleanup:     10 * time.Minute,
		DefaultShutdownGracePeriod:     10 * time.Millisecond,
		DefaultCacheTTL:                5 * time.Minute,

		// Transport/WebSocket Timeouts (production values)
		DefaultDialTimeout:             10 * time.Second,
		DefaultHandshakeTimeout:        10 * time.Second,
		DefaultReadTimeout:             60 * time.Second,
		DefaultWriteTimeout:            10 * time.Second,
		DefaultPingPeriod:              30 * time.Second,
		DefaultPongTimeout:             10 * time.Second,
		DefaultInitialReconnectDelay:   time.Second,
		DefaultMaxReconnectDelay:       30 * time.Second,
		DefaultAuthTimeout:             30 * time.Second,
		DefaultHeartbeatTimeout:        3 * time.Second,

		// Performance Monitoring (production values)
		DefaultProfilingInterval:       60 * time.Second,
		DefaultMaxLatency:              50 * time.Millisecond,
		DefaultMessageBatchTimeout:     5 * time.Millisecond,

		// Tools/HTTP Timeouts (production values)
		DefaultHTTPTimeout:             60 * time.Second,
		DefaultToolExecutionTimeout:    30 * time.Second,

		// Test/Development Timeouts (not used in production, but set for consistency)
		DefaultTestTimeout:             30 * time.Second,
		DefaultMockLatency:             10 * time.Millisecond,

		// Health and Monitoring (production values)
		DefaultHealthCheckInterval:     5 * time.Minute,
		DefaultHealthCheckTimeout:      30 * time.Second,

		// Storage and I/O (production values)
		DefaultStorageTimeout:          5 * time.Second,
		DefaultIOTimeout:               30 * time.Second,
		DefaultValidationTimeout:       5 * time.Second,
		DefaultSecurityCheckTimeout:    1 * time.Second,
		DefaultAuditWriteTimeout:       10 * time.Second,
		DefaultCryptoOperationTimeout:  5 * time.Second,

		// Network Settings (production values)
		DefaultTCPKeepAlive:            30 * time.Second,
		DefaultIdleConnectionTimeout:   90 * time.Second,

		// Cleanup and Maintenance (production values)
		DefaultCleanupWorkerInterval:   10 * time.Minute,
		DefaultExpiredCleanupInterval:  30 * time.Minute,
		DefaultMaintenanceInterval:     2 * time.Hour,

		// Circuit Breaker and Error Handling (production values)
		DefaultErrorCountWindow:        5 * time.Minute,
		DefaultErrorResetInterval:      5 * time.Minute,
		DefaultAlertCooldown:           5 * time.Minute,
		DefaultDuplicateAlertWindow:    5 * time.Minute,

		// Performance and GC (production values)
		DefaultGCMonitoringInterval:    30 * time.Second,
		DefaultResourceSampleInterval:  5 * time.Minute,
		DefaultMemoryMonitoringInterval: 2 * time.Minute,
		DefaultPerformanceMetricsInterval: 2 * time.Minute,

		// Retention Policies (production values)
		DefaultMetricsRetention:        24 * time.Hour,
		DefaultAuditLogRetention:       30 * 24 * time.Hour, // 30 days
		DefaultTraceRetention:          1 * time.Hour,

		// CPU/Memory Profiling (production values)
		DefaultCPUProfileInterval:      60 * time.Second,
		DefaultMemoryProfileInterval:   60 * time.Second,
	}
}

// createTestConfig returns test-appropriate timeouts (much shorter)
func createTestConfig() *TimeConfig {
	return &TimeConfig{
		// State Management Timeouts (test values - much shorter)
		DefaultShutdownTimeout:         1 * time.Second,
		DefaultUpdateTimeout:           1 * time.Second,
		DefaultRetryDelay:              10 * time.Millisecond,
		DefaultEventRetryBackoff:       50 * time.Millisecond,
		DefaultBatchTimeout:            10 * time.Millisecond,
		DefaultPerformanceBatchTimeout: 1 * time.Millisecond,
		DefaultMetricsInterval:         100 * time.Millisecond,
		DefaultCheckpointInterval:      500 * time.Millisecond,
		DefaultContextTTL:              1 * time.Minute,
		DefaultCleanupInterval:         100 * time.Millisecond,
		DefaultSubscriptionTTL:         1 * time.Minute,
		DefaultSubscriptionCleanup:     100 * time.Millisecond,
		DefaultShutdownGracePeriod:     1 * time.Millisecond,
		DefaultCacheTTL:                1 * time.Minute,

		// Transport/WebSocket Timeouts (test values - much shorter)
		DefaultDialTimeout:             500 * time.Millisecond,
		DefaultHandshakeTimeout:        500 * time.Millisecond,
		DefaultReadTimeout:             1 * time.Second,
		DefaultWriteTimeout:            500 * time.Millisecond,
		DefaultPingPeriod:              100 * time.Millisecond,
		DefaultPongTimeout:             200 * time.Millisecond,
		DefaultInitialReconnectDelay:   10 * time.Millisecond,
		DefaultMaxReconnectDelay:       100 * time.Millisecond,
		DefaultAuthTimeout:             1 * time.Second,
		DefaultHeartbeatTimeout:        100 * time.Millisecond,

		// Performance Monitoring (test values - much shorter)
		DefaultProfilingInterval:       100 * time.Millisecond,
		DefaultMaxLatency:              200 * time.Millisecond, // More relaxed for tests
		DefaultMessageBatchTimeout:     1 * time.Millisecond,

		// Tools/HTTP Timeouts (test values - much shorter)
		DefaultHTTPTimeout:             1 * time.Second,
		DefaultToolExecutionTimeout:    1 * time.Second,

		// Test/Development Timeouts (test values)
		DefaultTestTimeout:             1 * time.Second,
		DefaultMockLatency:             1 * time.Millisecond,

		// Health and Monitoring (test values - much shorter)
		DefaultHealthCheckInterval:     100 * time.Millisecond,
		DefaultHealthCheckTimeout:      500 * time.Millisecond,

		// Storage and I/O (test values - much shorter)
		DefaultStorageTimeout:          500 * time.Millisecond,
		DefaultIOTimeout:               1 * time.Second,
		DefaultValidationTimeout:       100 * time.Millisecond,
		DefaultSecurityCheckTimeout:    100 * time.Millisecond,
		DefaultAuditWriteTimeout:       500 * time.Millisecond,
		DefaultCryptoOperationTimeout:  100 * time.Millisecond,

		// Network Settings (test values - much shorter)
		DefaultTCPKeepAlive:            1 * time.Second,
		DefaultIdleConnectionTimeout:   2 * time.Second,

		// Cleanup and Maintenance (test values - much shorter)
		DefaultCleanupWorkerInterval:   100 * time.Millisecond,
		DefaultExpiredCleanupInterval:  200 * time.Millisecond,
		DefaultMaintenanceInterval:     1 * time.Second,

		// Circuit Breaker and Error Handling (test values - much shorter)
		DefaultErrorCountWindow:        100 * time.Millisecond,
		DefaultErrorResetInterval:      100 * time.Millisecond,
		DefaultAlertCooldown:           100 * time.Millisecond,
		DefaultDuplicateAlertWindow:    100 * time.Millisecond,

		// Performance and GC (test values - much shorter)
		DefaultGCMonitoringInterval:    100 * time.Millisecond,
		DefaultResourceSampleInterval:  100 * time.Millisecond,
		DefaultMemoryMonitoringInterval: 100 * time.Millisecond,
		DefaultPerformanceMetricsInterval: 100 * time.Millisecond,

		// Retention Policies (test values - much shorter)
		DefaultMetricsRetention:        1 * time.Minute,
		DefaultAuditLogRetention:       5 * time.Minute,
		DefaultTraceRetention:          1 * time.Minute,

		// CPU/Memory Profiling (test values - much shorter)
		DefaultCPUProfileInterval:      100 * time.Millisecond,
		DefaultMemoryProfileInterval:   100 * time.Millisecond,
	}
}

// Helper functions for accessing common timeouts

// ShutdownTimeout returns the configured shutdown timeout
func ShutdownTimeout() time.Duration {
	return GetConfig().DefaultShutdownTimeout
}

// UpdateTimeout returns the configured update timeout
func UpdateTimeout() time.Duration {
	return GetConfig().DefaultUpdateTimeout
}

// HTTPTimeout returns the configured HTTP timeout
func HTTPTimeout() time.Duration {
	return GetConfig().DefaultHTTPTimeout
}

// ToolExecutionTimeout returns the configured tool execution timeout
func ToolExecutionTimeout() time.Duration {
	return GetConfig().DefaultToolExecutionTimeout
}

// DialTimeout returns the configured dial timeout
func DialTimeout() time.Duration {
	return GetConfig().DefaultDialTimeout
}

// HandshakeTimeout returns the configured handshake timeout
func HandshakeTimeout() time.Duration {
	return GetConfig().DefaultHandshakeTimeout
}

// ReadTimeout returns the configured read timeout
func ReadTimeout() time.Duration {
	return GetConfig().DefaultReadTimeout
}

// WriteTimeout returns the configured write timeout
func WriteTimeout() time.Duration {
	return GetConfig().DefaultWriteTimeout
}

// PingPeriod returns the configured ping period
func PingPeriod() time.Duration {
	return GetConfig().DefaultPingPeriod
}

// PongTimeout returns the configured pong timeout
func PongTimeout() time.Duration {
	return GetConfig().DefaultPongTimeout
}

// InitialReconnectDelay returns the configured initial reconnect delay
func InitialReconnectDelay() time.Duration {
	return GetConfig().DefaultInitialReconnectDelay
}

// MaxReconnectDelay returns the configured max reconnect delay
func MaxReconnectDelay() time.Duration {
	return GetConfig().DefaultMaxReconnectDelay
}

// AuthTimeout returns the configured auth timeout
func AuthTimeout() time.Duration {
	return GetConfig().DefaultAuthTimeout
}

// HeartbeatTimeout returns the configured heartbeat timeout
func HeartbeatTimeout() time.Duration {
	return GetConfig().DefaultHeartbeatTimeout
}

// ProfilingInterval returns the configured profiling interval
func ProfilingInterval() time.Duration {
	return GetConfig().DefaultProfilingInterval
}

// MaxLatency returns the configured max latency
func MaxLatency() time.Duration {
	return GetConfig().DefaultMaxLatency
}

// MessageBatchTimeout returns the configured message batch timeout
func MessageBatchTimeout() time.Duration {
	return GetConfig().DefaultMessageBatchTimeout
}

// TestTimeout returns the configured test timeout
func TestTimeout() time.Duration {
	return GetConfig().DefaultTestTimeout
}

// Override functions for specific use cases

// OverrideForTest allows tests to override specific timeouts
func OverrideForTest(overrides map[string]time.Duration) func() {
	configMutex.Lock()
	defer configMutex.Unlock()
	
	// Save current config
	oldConfig := *globalConfig
	
	// Apply overrides
	newConfig := *globalConfig
	for key, value := range overrides {
		switch key {
		case "shutdown":
			newConfig.DefaultShutdownTimeout = value
		case "update":
			newConfig.DefaultUpdateTimeout = value
		case "http":
			newConfig.DefaultHTTPTimeout = value
		case "tool_execution":
			newConfig.DefaultToolExecutionTimeout = value
		case "dial":
			newConfig.DefaultDialTimeout = value
		case "handshake":
			newConfig.DefaultHandshakeTimeout = value
		case "read":
			newConfig.DefaultReadTimeout = value
		case "write":
			newConfig.DefaultWriteTimeout = value
		case "ping_period":
			newConfig.DefaultPingPeriod = value
		case "pong_timeout":
			newConfig.DefaultPongTimeout = value
		case "initial_reconnect_delay":
			newConfig.DefaultInitialReconnectDelay = value
		case "max_reconnect_delay":
			newConfig.DefaultMaxReconnectDelay = value
		case "auth":
			newConfig.DefaultAuthTimeout = value
		case "heartbeat":
			newConfig.DefaultHeartbeatTimeout = value
		case "profiling_interval":
			newConfig.DefaultProfilingInterval = value
		case "max_latency":
			newConfig.DefaultMaxLatency = value
		case "message_batch":
			newConfig.DefaultMessageBatchTimeout = value
		case "test":
			newConfig.DefaultTestTimeout = value
		}
	}
	
	globalConfig = &newConfig
	
	// Return cleanup function
	return func() {
		configMutex.Lock()
		defer configMutex.Unlock()
		globalConfig = &oldConfig
	}
}

// CreateNetworkTestConfig creates a configuration optimized for network simulation tests
// These tests need longer timeouts to handle simulated network conditions gracefully
func CreateNetworkTestConfig() *TimeConfig {
	return &TimeConfig{
		// State Management Timeouts (network test values - balanced for stability)
		DefaultShutdownTimeout:         5 * time.Second,
		DefaultUpdateTimeout:           5 * time.Second,
		DefaultRetryDelay:              50 * time.Millisecond,
		DefaultEventRetryBackoff:       200 * time.Millisecond,
		DefaultBatchTimeout:            50 * time.Millisecond,
		DefaultPerformanceBatchTimeout: 10 * time.Millisecond,
		DefaultMetricsInterval:         500 * time.Millisecond,
		DefaultCheckpointInterval:      2 * time.Second,
		DefaultContextTTL:              5 * time.Minute,
		DefaultCleanupInterval:         500 * time.Millisecond,
		DefaultSubscriptionTTL:         5 * time.Minute,
		DefaultSubscriptionCleanup:     500 * time.Millisecond,
		DefaultShutdownGracePeriod:     10 * time.Millisecond,
		DefaultCacheTTL:                5 * time.Minute,

		// Transport/WebSocket Timeouts (network test values - more tolerant of network issues)
		DefaultDialTimeout:             3 * time.Second,
		DefaultHandshakeTimeout:        3 * time.Second,
		DefaultReadTimeout:             10 * time.Second,  // Longer to handle packet loss
		DefaultWriteTimeout:            5 * time.Second,   // Longer to handle delays
		DefaultPingPeriod:              2 * time.Second,   // Less frequent pings
		DefaultPongTimeout:             5 * time.Second,   // More time for pong responses
		DefaultInitialReconnectDelay:   200 * time.Millisecond, // Slower initial reconnect
		DefaultMaxReconnectDelay:       2 * time.Second,   // Reasonable max delay
		DefaultAuthTimeout:             5 * time.Second,
		DefaultHeartbeatTimeout:        1 * time.Second,   // More tolerance for heartbeat delays

		// Performance Monitoring (network test values)
		DefaultProfilingInterval:       500 * time.Millisecond,
		DefaultMaxLatency:              1 * time.Second,   // Much more relaxed for network tests
		DefaultMessageBatchTimeout:     10 * time.Millisecond,

		// Tools/HTTP Timeouts (network test values)
		DefaultHTTPTimeout:             5 * time.Second,
		DefaultToolExecutionTimeout:    5 * time.Second,

		// Test/Development Timeouts (network test values)
		DefaultTestTimeout:             10 * time.Second,  // Much longer for network tests
		DefaultMockLatency:             10 * time.Millisecond,

		// Health and Monitoring (network test values)
		DefaultHealthCheckInterval:     500 * time.Millisecond,
		DefaultHealthCheckTimeout:      2 * time.Second,

		// Storage and I/O (network test values)
		DefaultStorageTimeout:          2 * time.Second,
		DefaultIOTimeout:               5 * time.Second,
		DefaultValidationTimeout:       1 * time.Second,
		DefaultSecurityCheckTimeout:    500 * time.Millisecond,
		DefaultAuditWriteTimeout:       2 * time.Second,
		DefaultCryptoOperationTimeout:  1 * time.Second,

		// Network Settings (network test values)
		DefaultTCPKeepAlive:            5 * time.Second,
		DefaultIdleConnectionTimeout:   15 * time.Second,

		// Cleanup and Maintenance (network test values)
		DefaultCleanupWorkerInterval:   500 * time.Millisecond,
		DefaultExpiredCleanupInterval:  1 * time.Second,
		DefaultMaintenanceInterval:     10 * time.Second,

		// Circuit Breaker and Error Handling (network test values)
		DefaultErrorCountWindow:        2 * time.Second,
		DefaultErrorResetInterval:      2 * time.Second,
		DefaultAlertCooldown:           1 * time.Second,
		DefaultDuplicateAlertWindow:    1 * time.Second,

		// Performance and GC (network test values)
		DefaultGCMonitoringInterval:    500 * time.Millisecond,
		DefaultResourceSampleInterval:  1 * time.Second,
		DefaultMemoryMonitoringInterval: 500 * time.Millisecond,
		DefaultPerformanceMetricsInterval: 500 * time.Millisecond,

		// Retention Policies (network test values)
		DefaultMetricsRetention:        5 * time.Minute,
		DefaultAuditLogRetention:       15 * time.Minute,
		DefaultTraceRetention:          5 * time.Minute,

		// CPU/Memory Profiling (network test values)
		DefaultCPUProfileInterval:      1 * time.Second,
		DefaultMemoryProfileInterval:   1 * time.Second,
	}
}