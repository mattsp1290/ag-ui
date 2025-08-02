package state

import (
	"time"
	
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/internal/timeconfig"
)

// Default configuration constants for state management
const (
	// History and Storage Limits
	DefaultMaxHistorySize         = 100  // Maximum number of state versions to retain
	DefaultMaxHistorySizeSharding = 1000 // Maximum history size for sharded operations
	DefaultMaxContexts            = 1000 // Default maximum number of active contexts
	DefaultShardCount             = 16   // Default number of shards for better concurrency

	// Buffer Sizes
	DefaultEventBufferSize  = 1000 // Default size for event processing buffer
	DefaultUpdateQueueSize  = 2000 // Update queue size for high-concurrency scenarios
	DefaultErrorChannelSize = 100  // Buffer size for error propagation channel
	BufferPoolSize          = 1024 // Buffer pool default size in bytes

	// Batch Processing
	DefaultBatchSize    = 100  // Default batch size for processing operations
	DefaultMaxBatchSize = 1000 // Maximum allowed batch size

	// Timeouts and Intervals - Now configurable based on test/production mode

	// Worker and Concurrency Settings
	DefaultProcessingWorkers = 4  // Default number of processing workers
	DefaultMaxConcurrency    = 16 // Maximum concurrent operations (typically runtime.NumCPU() * 2)
	DefaultMaxRetries        = 3  // Maximum number of operation retries
	DefaultMaxCheckpoints    = 10 // Maximum number of checkpoints to retain

	// Cache and Memory Settings
	DefaultCacheSize           = 1000              // Default cache size
	DefaultMaxMemoryUsage      = 100 * 1024 * 1024 // 100MB maximum memory usage
	DefaultLazyCacheSize       = 1000              // Lazy cache default size
	DefaultCacheExpiryTime     = 30 * time.Minute  // Cache entry expiration
	DefaultLazyCacheExpiryTime = 30 * time.Minute  // Lazy cache expiration
	DefaultMaxCacheEntries     = 1000              // Maximum cache entries for LRU eviction

	// Rate Limiting
	DefaultGlobalRateLimit       = 1000             // Default global rate limit (operations per second)
	DefaultClientRateLimit       = 100              // Default per-client rate limit
	DefaultClientBurstSize       = 200              // Default burst size for client rate limiting
	DefaultMaxClients            = 10000            // Maximum number of tracked clients
	DefaultClientTTL             = 30 * time.Minute // Client rate limiter TTL
	DefaultClientCleanupInterval = 5 * time.Minute  // Client cleanup interval
	DefaultMaxOpsPerSecond       = 10000            // Performance optimizer max ops per second

	// Connection and Pool Settings
	DefaultConnectionPoolSize = 10              // Default connection pool size
	DefaultConnectionTTL      = 5 * time.Minute // Connection validity timeout
	DefaultMaxConnections     = 100             // Maximum connections

	// Performance and Monitoring
	DefaultGCMonitoringInterval       = 30 * time.Second       // GC monitoring frequency
	DefaultResourceSampleInterval     = 5 * time.Minute        // Resource monitoring interval
	DefaultMemoryMonitoringInterval   = 2 * time.Minute        // Memory monitoring frequency
	DefaultPerformanceMetricsInterval = 2 * time.Minute        // Performance metrics collection
	DefaultCompressionLevel           = 6                      // Default compression level
	DefaultCompressionThreshold       = 50 * 1024 * 1024       // 50MB threshold for compression

	// Circuit Breaker and Error Handling
	DefaultErrorCountWindow         = 5 * time.Minute // Error count tracking window
	DefaultErrorResetInterval       = 5 * time.Minute // Error count reset interval
	DefaultMaxErrorCount            = 20              // Maximum errors before circuit breaker
	DefaultUpdateErrorThreshold     = 10              // Update error threshold
	DefaultCheckpointErrorThreshold = 5               // Checkpoint error threshold
	DefaultAlertCooldown            = 5 * time.Minute // Alert cooldown period
	DefaultDuplicateAlertWindow     = 5 * time.Minute // Duplicate alert prevention window
	DefaultLRUEvictionPercent       = 10              // LRU eviction percentage (10%)
)

// Security Constants
const (
	// Size Limits (from security.go)
	MaxPatchSizeBytes    = 1 << 20     // 1MB maximum patch size
	MaxStateSizeBytes    = 10 << 20    // 10MB maximum state size
	MaxMetadataSizeBytes = 1024 * 1024 // 1MB maximum metadata size
	MaxStringLengthBytes = 1 << 16     // 64KB maximum string length
	MaxArrayLength       = 10000       // Maximum array length
	MaxObjectKeys        = 1000        // Maximum object keys
	MaxJSONDepth         = 10          // Maximum JSON nesting depth
	DefaultMaxKeys       = 1000        // Default maximum keys

	// Buffer and ID Generation
	RandomIDBytes            = 16 // Random ID generation bytes
	AutoCheckpointNameLength = 15 // Auto checkpoint name format length ("20060102-150405")
)

// Health Check and Monitoring Constants
const (
	// Health Check Settings
	DefaultHealthCheckInterval = 5 * time.Minute // Health check frequency
	DefaultHealthCheckTimeout  = 30 * time.Second // Health check timeout

	// Alert Thresholds
	DefaultErrorRateThreshold       = 5.0                    // 5% error rate threshold
	DefaultErrorRateWindow          = 5 * time.Minute        // Error rate calculation window
	DefaultP95LatencyThreshold      = 100                    // 100ms P95 latency threshold
	DefaultP99LatencyThreshold      = 500                    // 500ms P99 latency threshold
	DefaultMemoryUsageThreshold     = 80                     // 80% memory usage threshold
	DefaultGCPauseThreshold         = 50                     // 50ms GC pause threshold
	DefaultConnectionPoolThreshold  = 85                     // 85% connection pool utilization
	DefaultConnectionErrorThreshold = 10                     // 10 connection errors threshold
	DefaultQueueDepthThreshold      = 1000                   // Queue depth threshold
	DefaultQueueLatencyThreshold    = 100                    // 100ms queue latency threshold
	DefaultRateLimitRejectThreshold = 100                    // Rate limit reject threshold
	DefaultRateLimitUtilThreshold   = 90                     // 90% rate limit utilization
	DefaultSlowOperationThreshold   = 100 * time.Millisecond // Slow operation threshold

	// Sampling and Profiling
	DefaultTraceSampleRate       = 0.1              // 10% trace sampling rate
	DefaultLogSamplingInitial    = 100              // Log sampling initial count
	DefaultLogSamplingThereafter = 100              // Log sampling thereafter count
	DefaultCPUProfileInterval    = 60 * time.Second // CPU profiling interval
	DefaultMemoryProfileInterval = 60 * time.Second // Memory profiling interval

	// Alert History and Management
	DefaultMaxAlertHistory   = 1000 // Maximum alert history entries
	DefaultLatencySampleSize = 1000 // Latency tracker sample size
)

// Storage and Backend Constants
const (
	// Storage Key Prefixes (for organized storage)
	StateKeyPrefix    = "state:"
	VersionKeyPrefix  = "version:"
	SnapshotKeyPrefix = "snapshot:"

	// Storage Limits
	DefaultStorageRetries = 3       // Storage operation retries
	DefaultStorageMaxKeys = 1000000 // Maximum storage keys

	// Transaction Settings
	DefaultMaxTransactionOps = 1000 // Maximum operations per transaction
)

// Performance Optimizer Constants
const (
	// Object Pool Settings
	DefaultPoolBufferSize = 1024  // Default object pool buffer size
	DefaultPoolMaxObjects = 10000 // Maximum objects in pool

	// Garbage Collection
	DefaultGCThresholdPercent = 50          // GC trigger at 50% memory usage
	DefaultGCInterval         = time.Minute // Minimum GC interval

	// Sharding
	DefaultShardHashSeed = 42 // Hash seed for consistent sharding

	// Compression
	DefaultCompressionMinSize = 1024 // Minimum size for compression

	// Task Queue
	DefaultTaskQueueMultiplier = 2 // Task queue size multiplier
)

// Cleanup and Maintenance Constants
const (
	// Cleanup Frequencies
	DefaultCleanupWorkerInterval  = 10 * time.Minute // Cleanup worker run frequency
	DefaultExpiredCleanupInterval = 30 * time.Minute // Expired entries cleanup
	DefaultMaintenanceInterval    = 2 * time.Hour    // General maintenance interval

	// Retention Policies
	DefaultMetricsRetention  = 24 * time.Hour      // Metrics retention period
	DefaultAuditLogRetention = 30 * 24 * time.Hour // 30 days audit retention
	DefaultTraceRetention    = 1 * time.Hour       // Trace data retention
)

// Network and I/O Constants
const (
	// I/O Settings
	DefaultReadBufferSize  = 4096             // Read buffer size
	DefaultWriteBufferSize = 4096             // Write buffer size
	DefaultIOTimeout       = 30 * time.Second // I/O operation timeout

	// Network Settings
	DefaultTCPKeepAlive          = 30 * time.Second // TCP keep-alive interval
	DefaultDialTimeout           = 10 * time.Second // Connection dial timeout
	DefaultMaxIdleConnections    = 100              // Maximum idle connections
	DefaultIdleConnectionTimeout = 90 * time.Second // Idle connection timeout
)

// Validation and Security Timeouts
const (
	// Validation Timeouts
	DefaultValidationTimeout      = 5 * time.Second  // Validation operation timeout
	DefaultSecurityCheckTimeout   = 1 * time.Second  // Security check timeout
	DefaultAuditWriteTimeout      = 10 * time.Second // Audit log write timeout
	DefaultCryptoOperationTimeout = 5 * time.Second  // Cryptographic operation timeout
)

// Development and Testing Constants
const (
	// Test Settings
	DefaultTestRetries = 3 // Test retry attempts

	// Debug Settings
	DefaultDebugSampleRate = 1.0  // 100% debug sampling in dev mode
	DefaultVerboseLogging  = true // Enable verbose logging in development
)

// Configurable timeout functions - use these instead of constants for time-based values
// These automatically adapt to test/production environments

// GetDefaultRetryDelay returns the configured retry delay
func GetDefaultRetryDelay() time.Duration {
	return timeconfig.GetConfig().DefaultRetryDelay
}

// GetDefaultEventRetryBackoff returns the configured event retry backoff
func GetDefaultEventRetryBackoff() time.Duration {
	return timeconfig.GetConfig().DefaultEventRetryBackoff
}

// GetDefaultBatchTimeout returns the configured batch timeout
func GetDefaultBatchTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultBatchTimeout
}

// GetDefaultPerformanceBatchTimeout returns the configured performance batch timeout
func GetDefaultPerformanceBatchTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultPerformanceBatchTimeout
}

// GetDefaultMetricsInterval returns the configured metrics interval
func GetDefaultMetricsInterval() time.Duration {
	return timeconfig.GetConfig().DefaultMetricsInterval
}

// GetDefaultCheckpointInterval returns the configured checkpoint interval
func GetDefaultCheckpointInterval() time.Duration {
	return timeconfig.GetConfig().DefaultCheckpointInterval
}

// GetDefaultContextTTL returns the configured context TTL
func GetDefaultContextTTL() time.Duration {
	return timeconfig.GetConfig().DefaultContextTTL
}

// GetDefaultCleanupInterval returns the configured cleanup interval
func GetDefaultCleanupInterval() time.Duration {
	return timeconfig.GetConfig().DefaultCleanupInterval
}

// GetDefaultSubscriptionTTL returns the configured subscription TTL
func GetDefaultSubscriptionTTL() time.Duration {
	return timeconfig.GetConfig().DefaultSubscriptionTTL
}

// GetDefaultSubscriptionCleanup returns the configured subscription cleanup interval
func GetDefaultSubscriptionCleanup() time.Duration {
	return timeconfig.GetConfig().DefaultSubscriptionCleanup
}

// GetDefaultShutdownTimeout returns the configured shutdown timeout
func GetDefaultShutdownTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultShutdownTimeout
}

// GetDefaultUpdateTimeout returns the configured update timeout
func GetDefaultUpdateTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultUpdateTimeout
}

// GetDefaultShutdownGracePeriod returns the configured shutdown grace period
func GetDefaultShutdownGracePeriod() time.Duration {
	return timeconfig.GetConfig().DefaultShutdownGracePeriod
}

// GetDefaultCacheTTL returns the configured cache TTL
func GetDefaultCacheTTL() time.Duration {
	return timeconfig.GetConfig().DefaultCacheTTL
}

// GetDefaultStorageTimeout returns the configured storage timeout
func GetDefaultStorageTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultStorageTimeout
}

// GetDefaultTransactionTimeout returns the configured transaction timeout
func GetDefaultTransactionTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultUpdateTimeout // Reuse update timeout for transactions
}

// GetDefaultTestTimeout returns the configured test timeout
func GetDefaultTestTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultTestTimeout
}

// GetDefaultMockLatency returns the configured mock latency
func GetDefaultMockLatency() time.Duration {
	return timeconfig.GetConfig().DefaultMockLatency
}

// GetDefaultHealthCheckInterval returns the configured health check interval
func GetDefaultHealthCheckInterval() time.Duration {
	return timeconfig.GetConfig().DefaultHealthCheckInterval
}

// GetDefaultHealthCheckTimeout returns the configured health check timeout
func GetDefaultHealthCheckTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultHealthCheckTimeout
}

// GetDefaultValidationTimeout returns the configured validation timeout
func GetDefaultValidationTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultValidationTimeout
}

// GetDefaultSecurityCheckTimeout returns the configured security check timeout
func GetDefaultSecurityCheckTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultSecurityCheckTimeout
}

// GetDefaultAuditWriteTimeout returns the configured audit write timeout
func GetDefaultAuditWriteTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultAuditWriteTimeout
}

// GetDefaultCryptoOperationTimeout returns the configured crypto operation timeout
func GetDefaultCryptoOperationTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultCryptoOperationTimeout
}

// GetDefaultIOTimeout returns the configured I/O timeout
func GetDefaultIOTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultIOTimeout
}

// GetDefaultTCPKeepAlive returns the configured TCP keep-alive interval
func GetDefaultTCPKeepAlive() time.Duration {
	return timeconfig.GetConfig().DefaultTCPKeepAlive
}

// GetDefaultIdleConnectionTimeout returns the configured idle connection timeout
func GetDefaultIdleConnectionTimeout() time.Duration {
	return timeconfig.GetConfig().DefaultIdleConnectionTimeout
}

// GetDefaultCleanupWorkerInterval returns the configured cleanup worker interval
func GetDefaultCleanupWorkerInterval() time.Duration {
	return timeconfig.GetConfig().DefaultCleanupWorkerInterval
}

// GetDefaultExpiredCleanupInterval returns the configured expired cleanup interval
func GetDefaultExpiredCleanupInterval() time.Duration {
	return timeconfig.GetConfig().DefaultExpiredCleanupInterval
}

// GetDefaultMaintenanceInterval returns the configured maintenance interval
func GetDefaultMaintenanceInterval() time.Duration {
	return timeconfig.GetConfig().DefaultMaintenanceInterval
}

// GetDefaultErrorCountWindow returns the configured error count window
func GetDefaultErrorCountWindow() time.Duration {
	return timeconfig.GetConfig().DefaultErrorCountWindow
}

// GetDefaultErrorResetInterval returns the configured error reset interval
func GetDefaultErrorResetInterval() time.Duration {
	return timeconfig.GetConfig().DefaultErrorResetInterval
}

// GetDefaultAlertCooldown returns the configured alert cooldown
func GetDefaultAlertCooldown() time.Duration {
	return timeconfig.GetConfig().DefaultAlertCooldown
}

// GetDefaultDuplicateAlertWindow returns the configured duplicate alert window
func GetDefaultDuplicateAlertWindow() time.Duration {
	return timeconfig.GetConfig().DefaultDuplicateAlertWindow
}

// GetDefaultGCMonitoringInterval returns the configured GC monitoring interval
func GetDefaultGCMonitoringInterval() time.Duration {
	return timeconfig.GetConfig().DefaultGCMonitoringInterval
}

// GetDefaultResourceSampleInterval returns the configured resource sample interval
func GetDefaultResourceSampleInterval() time.Duration {
	return timeconfig.GetConfig().DefaultResourceSampleInterval
}

// GetDefaultMemoryMonitoringInterval returns the configured memory monitoring interval
func GetDefaultMemoryMonitoringInterval() time.Duration {
	return timeconfig.GetConfig().DefaultMemoryMonitoringInterval
}

// GetDefaultPerformanceMetricsInterval returns the configured performance metrics interval
func GetDefaultPerformanceMetricsInterval() time.Duration {
	return timeconfig.GetConfig().DefaultPerformanceMetricsInterval
}

// GetDefaultMetricsRetention returns the configured metrics retention
func GetDefaultMetricsRetention() time.Duration {
	return timeconfig.GetConfig().DefaultMetricsRetention
}

// GetDefaultAuditLogRetention returns the configured audit log retention
func GetDefaultAuditLogRetention() time.Duration {
	return timeconfig.GetConfig().DefaultAuditLogRetention
}

// GetDefaultTraceRetention returns the configured trace retention
func GetDefaultTraceRetention() time.Duration {
	return timeconfig.GetConfig().DefaultTraceRetention
}

// GetDefaultCPUProfileInterval returns the configured CPU profile interval
func GetDefaultCPUProfileInterval() time.Duration {
	return timeconfig.GetConfig().DefaultCPUProfileInterval
}

// GetDefaultMemoryProfileInterval returns the configured memory profile interval
func GetDefaultMemoryProfileInterval() time.Duration {
	return timeconfig.GetConfig().DefaultMemoryProfileInterval
}
