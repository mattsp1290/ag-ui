package state

import "time"

// Default configuration constants for state management
const (
	// History and Storage Limits
	DefaultMaxHistorySize         = 100  // Maximum number of state versions to retain
	DefaultMaxHistorySizeSharding = 1000 // Maximum history size for sharded operations
	DefaultMaxContexts            = 1000 // Default maximum number of active contexts
	DefaultShardCount             = 16   // Default number of shards for better concurrency

	// Buffer Sizes
	DefaultEventBufferSize  = 1000 // Default size for event processing buffer
	DefaultUpdateQueueSize  = 200  // Update queue size (BatchSize * 2)
	DefaultErrorChannelSize = 100  // Buffer size for error propagation channel
	BufferPoolSize          = 1024 // Buffer pool default size in bytes

	// Batch Processing
	DefaultBatchSize    = 100  // Default batch size for processing operations
	DefaultMaxBatchSize = 1000 // Maximum allowed batch size

	// Timeouts and Intervals
	DefaultRetryDelay              = 100 * time.Millisecond // Default retry delay
	DefaultEventRetryBackoff       = time.Second            // Event processing retry backoff
	DefaultBatchTimeout            = 100 * time.Millisecond // Batch processing timeout
	DefaultPerformanceBatchTimeout = 10 * time.Millisecond  // Performance optimizer batch timeout
	DefaultMetricsInterval         = 30 * time.Second       // Metrics collection interval
	DefaultCheckpointInterval      = 5 * time.Minute        // Automatic checkpoint creation interval
	DefaultContextTTL              = 1 * time.Hour          // Default context time-to-live
	DefaultCleanupInterval         = 15 * time.Minute       // Context cleanup interval
	DefaultSubscriptionTTL         = 1 * time.Hour          // Subscription time-to-live
	DefaultSubscriptionCleanup     = 10 * time.Minute       // Subscription cleanup interval
	DefaultShutdownTimeout         = 30 * time.Second       // Manager shutdown timeout
	DefaultUpdateTimeout           = 30 * time.Second       // Default update operation timeout
	DefaultShutdownGracePeriod     = 10 * time.Millisecond  // Grace period for shutdown
	DefaultCacheTTL                = 5 * time.Minute        // Default cache time-to-live

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
	DefaultGCMonitoringInterval       = 100 * time.Millisecond // GC monitoring frequency
	DefaultResourceSampleInterval     = 10 * time.Second       // Resource monitoring interval
	DefaultMemoryMonitoringInterval   = 5 * time.Second        // Memory monitoring frequency
	DefaultPerformanceMetricsInterval = 30 * time.Second       // Performance metrics collection
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
	DefaultHealthCheckInterval = 30 * time.Second // Health check frequency
	DefaultHealthCheckTimeout  = 5 * time.Second  // Health check timeout

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
	DefaultStorageTimeout = 5 * time.Second // Storage operation timeout
	DefaultStorageRetries = 3               // Storage operation retries
	DefaultStorageMaxKeys = 1000000         // Maximum storage keys

	// Transaction Settings
	DefaultTransactionTimeout = 30 * time.Second // Transaction timeout
	DefaultMaxTransactionOps  = 1000             // Maximum operations per transaction
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
	DefaultCleanupWorkerInterval  = time.Minute     // Cleanup worker run frequency
	DefaultExpiredCleanupInterval = 5 * time.Minute // Expired entries cleanup
	DefaultMaintenanceInterval    = 1 * time.Hour   // General maintenance interval

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
	DefaultTestTimeout = 30 * time.Second      // Test operation timeout
	DefaultTestRetries = 3                     // Test retry attempts
	DefaultMockLatency = 10 * time.Millisecond // Mock operation latency

	// Debug Settings
	DefaultDebugSampleRate = 1.0  // 100% debug sampling in dev mode
	DefaultVerboseLogging  = true // Enable verbose logging in development
)
