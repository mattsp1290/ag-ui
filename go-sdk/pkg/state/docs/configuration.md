# AG-UI State Management Configuration Guide

This guide covers all configuration options for the AG-UI state management system, including setup, tuning, and best practices for different deployment scenarios.

## Table of Contents

1. [Quick Start Configuration](#quick-start-configuration)
2. [Manager Options](#manager-options)
3. [Storage Configuration](#storage-configuration)
4. [Validation Configuration](#validation-configuration)
5. [Performance Configuration](#performance-configuration)
6. [Monitoring Configuration](#monitoring-configuration)
7. [Security Configuration](#security-configuration)
8. [Environment-Specific Configurations](#environment-specific-configurations)
9. [Advanced Configuration](#advanced-configuration)
10. [Configuration Examples](#configuration-examples)

## Quick Start Configuration

### Basic Setup

```go
package main

import (
    "log"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/state"
)

func main() {
    // Use default configuration
    options := state.DefaultManagerOptions()
    
    manager, err := state.NewStateManager(options)
    if err != nil {
        log.Fatal("Failed to create state manager:", err)
    }
    defer manager.Close()
    
    // Your application logic here
}
```

### Minimal Custom Configuration

```go
// Minimal custom configuration
options := state.ManagerOptions{
    MaxHistorySize:   50,              // Keep 50 versions
    EnableCaching:    true,            // Enable caching
    CacheSize:        1000,            // Cache 1000 states
    ConflictStrategy: state.LastWriteWins, // Simple conflict resolution
    StrictMode:       false,           // Relaxed validation
}

manager, err := state.NewStateManager(options)
```

## Manager Options

### Complete Options Structure

```go
type ManagerOptions struct {
    // Storage configuration
    MaxHistorySize int                    // Maximum versions to keep (default: 100)
    EnableCaching  bool                   // Enable in-memory caching (default: true)
    
    // Conflict resolution configuration
    ConflictStrategy ConflictResolutionStrategy // How to handle conflicts (default: LastWriteWins)
    MaxRetries       int                        // Max retry attempts (default: 3)
    RetryDelay       time.Duration             // Delay between retries (default: 100ms)
    
    // Validation configuration
    ValidationRules []ValidationRule // Custom validation rules
    StrictMode      bool            // Fail on validation errors (default: true)
    
    // Rollback configuration
    MaxCheckpoints       int           // Max checkpoints to keep (default: 10)
    CheckpointInterval   time.Duration // Auto checkpoint interval (default: 5m)
    AutoCheckpoint       bool          // Enable auto checkpoints (default: true)
    CompressCheckpoints  bool          // Compress checkpoints (default: true)
    
    // Event handling configuration
    EventBufferSize      int           // Event buffer size (default: 1000)
    ProcessingWorkers    int           // Number of workers (default: 4)
    EventRetryBackoff    time.Duration // Event retry backoff (default: 1s)
    
    // Performance configuration
    CacheSize          int           // Cache size limit (default: 1000)
    CacheTTL           time.Duration // Cache TTL (default: 5m)
    EnableCompression  bool          // Enable compression (default: true)
    EnableBatching     bool          // Enable batching (default: true)
    BatchSize          int           // Batch size (default: 100)
    BatchTimeout       time.Duration // Batch timeout (default: 100ms)
    
    // Monitoring configuration
    EnableMetrics      bool          // Enable metrics (default: true)
    MetricsInterval    time.Duration // Metrics interval (default: 30s)
    EnableTracing      bool          // Enable tracing (default: false)
    
    // Audit configuration
    EnableAudit        bool          // Enable audit logging (default: true)
    AuditLogger        AuditLogger   // Custom audit logger
    
    // Storage backend configuration
    StorageBackend     string        // Backend type ("file", "redis", "postgres")
    StorageConfig      *StorageConfig // Backend-specific configuration
    
    // Security configuration
    SecurityConfig     *SecurityConfig // Security settings
    
    // Advanced configuration
    AdvancedConfig     *AdvancedConfig // Advanced settings
}
```

### Default Values

```go
func DefaultManagerOptions() ManagerOptions {
    return ManagerOptions{
        // Storage
        MaxHistorySize:       100,
        EnableCaching:        true,
        
        // Conflict resolution
        ConflictStrategy:     LastWriteWins,
        MaxRetries:           3,
        RetryDelay:           100 * time.Millisecond,
        
        // Validation
        StrictMode:           true,
        
        // Rollback
        MaxCheckpoints:       10,
        CheckpointInterval:   5 * time.Minute,
        AutoCheckpoint:       true,
        CompressCheckpoints:  true,
        
        // Events
        EventBufferSize:      1000,
        ProcessingWorkers:    4,
        EventRetryBackoff:    time.Second,
        
        // Performance
        CacheSize:            1000,
        CacheTTL:             5 * time.Minute,
        EnableCompression:    true,
        EnableBatching:       true,
        BatchSize:            100,
        BatchTimeout:         100 * time.Millisecond,
        
        // Monitoring
        EnableMetrics:        true,
        MetricsInterval:      30 * time.Second,
        EnableTracing:        false,
        
        // Audit
        EnableAudit:          true,
        AuditLogger:          nil, // Default JSON logger
        
        // Storage
        StorageBackend:       "file",
        StorageConfig:        DefaultStorageConfig(),
    }
}
```

## Storage Configuration

### Storage Backend Options

#### File Storage (Default)

```go
storageConfig := &state.StorageConfig{
    // Basic settings
    Path:        "/var/lib/agui/state",  // Storage directory
    Compression: true,                   // Enable compression
    
    // Performance settings
    WriteTimeout:    5 * time.Second,    // Write timeout
    ReadTimeout:     3 * time.Second,    // Read timeout
    SyncWrites:      true,               // Sync writes to disk
    
    // Backup settings
    BackupEnabled:   true,               // Enable backups
    BackupInterval:  1 * time.Hour,      // Backup interval
    BackupRetention: 7 * 24 * time.Hour, // Keep backups for 7 days
    
    // Maintenance settings
    CompactionEnabled:  true,            // Enable file compaction
    CompactionInterval: 24 * time.Hour,  // Compact daily
    
    // Advanced settings
    MaxFileSize:     100 * 1024 * 1024,  // 100MB max file size
    DirectoryPerms:  0755,               // Directory permissions
    FilePerms:       0644,               // File permissions
}

options := state.DefaultManagerOptions()
options.StorageBackend = "file"
options.StorageConfig = storageConfig
```

#### Redis Storage

```go
storageConfig := &state.StorageConfig{
    // Connection settings
    ConnectionURL:    os.Getenv("REDIS_URL"), // Example: "redis://localhost:6379"
    Database:         0,                 // Redis database number
    Password:         os.Getenv("REDIS_PASSWORD"), // Redis password from environment
    
    // Pool settings
    PoolSize:         10,                // Connection pool size
    MinIdleConns:     5,                 // Minimum idle connections
    MaxConnAge:       30 * time.Minute,  // Max connection age
    PoolTimeout:      4 * time.Second,   // Pool timeout
    IdleTimeout:      5 * time.Minute,   // Idle timeout
    
    // Operation settings
    ReadTimeout:      3 * time.Second,   // Read timeout
    WriteTimeout:     3 * time.Second,   // Write timeout
    MaxRetries:       3,                 // Max retry attempts
    MinRetryBackoff:  8 * time.Millisecond,
    MaxRetryBackoff:  512 * time.Millisecond,
    
    // Persistence settings
    EnablePersistence: true,             // Enable Redis persistence
    
    // Clustering settings (if using Redis Cluster)
    ClusterEnabled:   false,             // Enable cluster mode
    ClusterNodes:     []string{},        // Cluster node addresses
    
    // Advanced settings
    KeyPrefix:        "agui:state:",     // Key prefix
    Compression:      true,              // Enable compression
    CompressionLevel: 6,                 // Compression level (1-9)
}

options.StorageBackend = "redis"
options.StorageConfig = storageConfig
```

#### PostgreSQL Storage

```go
storageConfig := &state.StorageConfig{
    // Connection settings
    ConnectionURL:    os.Getenv("DATABASE_URL"), // Example: "postgres://user:pass@localhost:5432/statedb"
    
    // Pool settings
    MaxOpenConns:     25,                // Max open connections
    MaxIdleConns:     5,                 // Max idle connections
    ConnMaxLifetime:  5 * time.Minute,   // Connection max lifetime
    ConnMaxIdleTime:  30 * time.Second,  // Connection max idle time
    
    // Operation settings
    QueryTimeout:     10 * time.Second,  // Query timeout
    TxTimeout:        30 * time.Second,  // Transaction timeout
    
    // Schema settings
    SchemaName:       "public",          // Schema name
    TablePrefix:      "agui_",           // Table prefix
    
    // Performance settings
    BulkInsertSize:   1000,              // Bulk insert batch size
    EnablePreparedStmts: true,           // Enable prepared statements
    
    // Advanced settings
    SSLMode:          "prefer",          // SSL mode
    Compression:      true,              // Enable compression
    CompressionLevel: 6,                 // Compression level
}

options.StorageBackend = "postgres"
options.StorageConfig = storageConfig
```

### Storage Configuration Examples

#### High Performance Setup

```go
// Optimized for high throughput
storageConfig := &state.StorageConfig{
    // Use Redis for high performance
    ConnectionURL:    os.Getenv("REDIS_URL"), // Example: "redis://localhost:6379"
    PoolSize:         20,                // Large pool
    MinIdleConns:     10,                // Keep connections warm
    
    // Disable compression for speed
    Compression:      false,
    
    // Fast timeouts
    ReadTimeout:      1 * time.Second,
    WriteTimeout:     2 * time.Second,
}

options.StorageBackend = "redis"
options.StorageConfig = storageConfig
options.EnableBatching = true
options.BatchSize = 500
options.BatchTimeout = 50 * time.Millisecond
```

#### Durability-First Setup

```go
// Optimized for data durability
storageConfig := &state.StorageConfig{
    // Use PostgreSQL for ACID guarantees
    ConnectionURL:    os.Getenv("DATABASE_URL"), // Example: "postgres://user:pass@localhost:5432/statedb"
    MaxOpenConns:     10,
    TxTimeout:        60 * time.Second,
    
    // Enable all durability features
    EnablePersistence: true,
    BackupEnabled:    true,
    BackupInterval:   30 * time.Minute,
    
    // High compression for storage efficiency
    Compression:      true,
    CompressionLevel: 9,
}

options.StorageBackend = "postgres"
options.StorageConfig = storageConfig
options.AutoCheckpoint = true
options.CheckpointInterval = 1 * time.Minute
```

## Validation Configuration

### Built-in Validation Rules

```go
options := state.DefaultManagerOptions()

// Add validation rules
options.ValidationRules = []state.ValidationRule{
    // Required fields
    state.RequiredFields("id", "type", "name"),
    
    // Type validation
    state.TypeValidation("age", "number"),
    state.TypeValidation("email", "string"),
    state.TypeValidation("active", "boolean"),
    
    // Range validation
    state.RangeValidation("age", 0, 150),
    state.RangeValidation("score", 0.0, 100.0),
    
    // Pattern validation
    state.PatternValidation("email", `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
    state.PatternValidation("phone", `^\+?[1-9]\d{1,14}$`),
    
    // Length validation
    state.LengthValidation("name", 1, 100),
    state.LengthValidation("description", 0, 500),
    
    // Enum validation
    state.EnumValidation("status", []string{"active", "inactive", "pending"}),
    state.EnumValidation("role", []string{"admin", "user", "guest"}),
}

// Validation mode
options.StrictMode = true  // Fail on validation errors
```

### Custom Validation Rules

```go
// Custom business logic validation
type BusinessLogicRule struct {
    name string
}

func (r *BusinessLogicRule) Validate(state *state.State) error {
    // Example: Premium users must have email
    if userType, ok := state.Data["type"].(string); ok && userType == "premium" {
        if email, ok := state.Data["email"].(string); !ok || email == "" {
            return errors.New("premium users must have email")
        }
    }
    
    // Example: Age must be consistent with birth date
    if age, ok := state.Data["age"].(float64); ok {
        if birthDate, ok := state.Data["birth_date"].(string); ok {
            if calculatedAge := calculateAge(birthDate); int(age) != calculatedAge {
                return errors.New("age inconsistent with birth date")
            }
        }
    }
    
    return nil
}

func (r *BusinessLogicRule) Description() string {
    return "Business logic validation"
}

// Add to validation rules
options.ValidationRules = append(options.ValidationRules, &BusinessLogicRule{
    name: "business-logic",
})
```

### Conditional Validation

```go
// Conditional validation based on state
options.ValidationRules = append(options.ValidationRules, 
    state.ConditionalRule(
        func(state *state.State) bool {
            // Only validate address for users with type "customer"
            userType, _ := state.Data["type"].(string)
            return userType == "customer"
        },
        state.RequiredFields("address", "city", "country"),
    ),
)
```

## Performance Configuration

### Caching Configuration

```go
options := state.DefaultManagerOptions()

// Cache settings
options.EnableCaching = true
options.CacheSize = 10000              // Cache 10,000 states
options.CacheTTL = 10 * time.Minute    // Cache for 10 minutes

// Cache behavior
options.CachePolicy = state.CachePolicy{
    MaxSize:        10000,
    TTL:           10 * time.Minute,
    EvictionPolicy: "lru",              // LRU eviction
    WarmupEnabled:  true,               // Warm cache on startup
    WarmupSize:     1000,               // Warm 1000 most recent states
}
```

### Batching Configuration

```go
// Batch processing settings
options.EnableBatching = true
options.BatchSize = 200                 // Process 200 operations per batch
options.BatchTimeout = 50 * time.Millisecond // Flush after 50ms

// Advanced batching
options.BatchConfig = state.BatchConfig{
    MaxSize:        200,
    MaxTimeout:     50 * time.Millisecond,
    MinSize:        10,                 // Minimum batch size
    MaxWaitTime:    100 * time.Millisecond, // Max wait for batch
    PriorityQueues: true,               // Enable priority queues
}
```

### Compression Configuration

```go
// Compression settings
options.EnableCompression = true
options.CompressionConfig = state.CompressionConfig{
    Algorithm:     "gzip",              // Compression algorithm
    Level:         6,                   // Compression level (1-9)
    MinSize:       1024,                // Only compress if > 1KB
    CompressTypes: []string{"json", "text"}, // Compress these types
}
```

### Concurrency Configuration

```go
// Concurrency settings
options.ConcurrencyConfig = state.ConcurrencyConfig{
    MaxConcurrentOps:    100,           // Max concurrent operations
    MaxConcurrentReads:  50,            // Max concurrent reads
    MaxConcurrentWrites: 20,            // Max concurrent writes
    ReadWriteRatio:      4,             // Read:Write ratio (4:1)
    DeadlockTimeout:     5 * time.Second, // Deadlock detection timeout
}
```

## Monitoring Configuration

### Metrics Configuration

```go
options := state.DefaultManagerOptions()

// Enable metrics collection
options.EnableMetrics = true
options.MetricsInterval = 30 * time.Second

// Prometheus configuration
options.MonitoringConfig = state.MonitoringConfig{
    EnablePrometheus:     true,
    PrometheusNamespace:  "agui",
    PrometheusSubsystem:  "state",
    MetricsEnabled:       true,
    MetricsInterval:      30 * time.Second,
    
    // Custom metrics
    CustomMetrics: map[string]state.MetricDefinition{
        "custom_counter": {
            Type: "counter",
            Help: "Custom counter metric",
            Labels: []string{"operation", "status"},
        },
        "custom_histogram": {
            Type: "histogram",
            Help: "Custom histogram metric",
            Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
        },
    },
}
```

### Logging Configuration

```go
// Logging configuration
options.MonitoringConfig.LogLevel = zapcore.InfoLevel
options.MonitoringConfig.LogFormat = "json"        // "json" or "console"
options.MonitoringConfig.StructuredLogging = true
options.MonitoringConfig.LogSampling = true        // Enable log sampling

// Custom log output
logFile, _ := os.OpenFile("/var/log/agui-state.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
options.MonitoringConfig.LogOutput = logFile
```

### Tracing Configuration

```go
// Distributed tracing
options.EnableTracing = true
options.MonitoringConfig.EnableTracing = true
options.MonitoringConfig.TracingServiceName = "agui-state-manager"
options.MonitoringConfig.TracingProvider = "jaeger"
options.MonitoringConfig.TraceSampleRate = 0.1     // Sample 10% of traces
```

### Health Check Configuration

```go
// Health check settings
options.MonitoringConfig.EnableHealthChecks = true
options.MonitoringConfig.HealthCheckInterval = 30 * time.Second
options.MonitoringConfig.HealthCheckTimeout = 5 * time.Second

// Custom health checks
options.MonitoringConfig.CustomHealthChecks = map[string]state.HealthCheckFunc{
    "custom_check": func() error {
        // Custom health check logic
        return nil
    },
}
```

## Security Configuration

### Authentication Configuration

```go
options := state.DefaultManagerOptions()

// Security settings
options.SecurityConfig = &state.SecurityConfig{
    // Authentication
    AuthEnabled:        true,
    AuthProvider:       "jwt",           // JWT authentication
    AuthTokenHeader:    "Authorization", // Token header name
    AuthTokenPrefix:    "Bearer ",       // Token prefix
    
    // Authorization
    AuthzEnabled:       true,
    AuthzProvider:      "rbac",          // RBAC authorization
    DefaultPermissions: []string{"read"}, // Default permissions
    
    // Context validation
    ValidateContext:    true,
    RequireUserID:      true,
    AllowAnonymous:     false,
    
    // Rate limiting
    RateLimitEnabled:   true,
    RateLimitPerUser:   100,             // 100 ops per minute per user
    RateLimitGlobal:    1000,            // 1000 ops per minute globally
    RateLimitWindow:    time.Minute,     // Rate limit window
}
```

### Data Protection Configuration

```go
// Data protection settings
options.SecurityConfig.DataProtection = state.DataProtectionConfig{
    // Encryption
    EncryptionEnabled:  true,
    EncryptionKey:      "your-32-char-encryption-key!!",
    EncryptionAlgorithm: "AES-256-GCM",
    
    // PII protection
    PIIDetectionEnabled: true,
    PIIFields:          []string{"email", "phone", "ssn"},
    PIIMaskingEnabled:  true,
    PIIMaskingChar:     "*",
    
    // Data sanitization
    SanitizeAuditLogs: true,
    SanitizeEvents:    true,
    SanitizeMetrics:   true,
    
    // Secure deletion
    SecureDeleteEnabled: true,
    SecureDeletePasses:  3,
}
```

### Audit Configuration

```go
// Audit logging
options.EnableAudit = true
options.AuditConfig = state.AuditConfig{
    // Basic settings
    Enabled:           true,
    LogLevel:          "info",
    LogFormat:         "json",
    
    // Storage
    StorageType:       "file",
    StoragePath:       "/var/log/agui-audit.log",
    RotationEnabled:   true,
    RotationSize:      100 * 1024 * 1024, // 100MB
    RetentionDays:     90,                  // Keep 90 days
    
    // Content
    LogOperations:     true,
    LogDataChanges:    true,
    LogAuthentication: true,
    LogAuthorization:  true,
    LogErrors:         true,
    
    // Filtering
    ExcludeOperations: []string{"health_check"},
    ExcludeUsers:      []string{"system"},
    IncludeDataFields: []string{"id", "type", "user_id"},
}
```

## Environment-Specific Configurations

### Development Environment

```go
func DevelopmentConfig() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Storage - use in-memory for development
    options.StorageBackend = "memory"
    options.StorageConfig = &state.StorageConfig{
        MaxMemorySize: 100 * 1024 * 1024, // 100MB
    }
    
    // Relaxed validation
    options.StrictMode = false
    
    // Verbose logging
    options.MonitoringConfig.LogLevel = zapcore.DebugLevel
    options.MonitoringConfig.LogFormat = "console"
    
    // Disabled security for development
    options.SecurityConfig = &state.SecurityConfig{
        AuthEnabled:     false,
        AuthzEnabled:    false,
        AllowAnonymous:  true,
    }
    
    // Small cache for development
    options.CacheSize = 100
    
    return options
}
```

### Production Environment

```go
func ProductionConfig() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Storage - use PostgreSQL for production
    options.StorageBackend = "postgres"
    options.StorageConfig = &state.StorageConfig{
        ConnectionURL:    os.Getenv("DATABASE_URL"),
        MaxOpenConns:     25,
        MaxIdleConns:     5,
        ConnMaxLifetime:  5 * time.Minute,
        BackupEnabled:    true,
        BackupInterval:   1 * time.Hour,
    }
    
    // Strict validation
    options.StrictMode = true
    
    // Production logging
    options.MonitoringConfig.LogLevel = zapcore.InfoLevel
    options.MonitoringConfig.LogFormat = "json"
    options.MonitoringConfig.StructuredLogging = true
    
    // Full security
    options.SecurityConfig = &state.SecurityConfig{
        AuthEnabled:        true,
        AuthzEnabled:       true,
        ValidateContext:    true,
        RequireUserID:      true,
        RateLimitEnabled:   true,
        EncryptionEnabled:  true,
        PIIDetectionEnabled: true,
    }
    
    // Large cache for production
    options.CacheSize = 10000
    
    // Enable all monitoring
    options.EnableMetrics = true
    options.EnableTracing = true
    options.EnableAudit = true
    
    return options
}
```

### Testing Environment

```go
func TestingConfig() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Storage - use memory for tests
    options.StorageBackend = "memory"
    
    // Fast operations for tests
    options.CacheSize = 10
    options.BatchSize = 10
    options.BatchTimeout = 10 * time.Millisecond
    
    // Minimal history for tests
    options.MaxHistorySize = 5
    options.MaxCheckpoints = 3
    
    // Disabled monitoring for tests
    options.EnableMetrics = false
    options.EnableTracing = false
    options.EnableAudit = false
    
    // Minimal logging
    options.MonitoringConfig.LogLevel = zapcore.WarnLevel
    
    return options
}
```

## Advanced Configuration

### Custom Configuration Provider

```go
// Configuration provider interface
type ConfigProvider interface {
    GetConfig(key string) (interface{}, error)
    WatchConfig(key string, callback func(interface{})) error
}

// File-based configuration provider
type FileConfigProvider struct {
    configPath string
}

func (p *FileConfigProvider) GetConfig(key string) (interface{}, error) {
    // Load configuration from file
    return nil, nil
}

// Usage
configProvider := &FileConfigProvider{configPath: "/etc/agui/config.yaml"}
options := state.LoadOptionsFromProvider(configProvider)
```

### Dynamic Configuration

```go
// Dynamic configuration updates
manager, err := state.NewStateManager(options)
if err != nil {
    log.Fatal(err)
}

// Update configuration at runtime
newConfig := state.RuntimeConfig{
    CacheSize:        2000,
    BatchSize:        150,
    MetricsInterval:  60 * time.Second,
}

err = manager.UpdateConfig(newConfig)
if err != nil {
    log.Printf("Failed to update config: %v", err)
}
```

### Plugin Configuration

```go
// Plugin configuration
options.PluginConfig = state.PluginConfig{
    LoadPath:     "/usr/lib/agui/plugins",
    EnabledPlugins: []string{"custom-validator", "metrics-exporter"},
    PluginSettings: map[string]interface{}{
        "custom-validator": map[string]interface{}{
            "strict": true,
            "rules": []string{"email", "phone"},
        },
        "metrics-exporter": map[string]interface{}{
            "endpoint": "http://prometheus:9090",
            "interval": "30s",
        },
    },
}
```

## Configuration Examples

### High-Availability Setup

```go
func HighAvailabilityConfig() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Clustered Redis storage
    options.StorageBackend = "redis"
    options.StorageConfig = &state.StorageConfig{
        ClusterEnabled: true,
        ClusterNodes: []string{
            "redis-1:6379",
            "redis-2:6379",
            "redis-3:6379",
        },
        MaxRetries:      5,
        EnablePersistence: true,
    }
    
    // Aggressive caching
    options.CacheSize = 50000
    options.CacheTTL = 15 * time.Minute
    
    // Multiple event workers
    options.ProcessingWorkers = 8
    options.EventBufferSize = 5000
    
    // Frequent checkpoints
    options.AutoCheckpoint = true
    options.CheckpointInterval = 30 * time.Second
    
    // Comprehensive monitoring
    options.EnableMetrics = true
    options.EnableTracing = true
    options.EnableAudit = true
    
    return options
}
```

### Microservices Configuration

```go
func MicroservicesConfig(serviceName string) state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Service-specific storage path
    options.StorageBackend = "file"
    options.StorageConfig = &state.StorageConfig{
        Path: fmt.Sprintf("/var/lib/agui/%s", serviceName),
    }
    
    // Service-specific metrics
    options.MonitoringConfig.PrometheusSubsystem = serviceName
    options.MonitoringConfig.TracingServiceName = serviceName
    
    // Service-specific validation
    options.ValidationRules = loadServiceValidationRules(serviceName)
    
    // Service-specific security
    options.SecurityConfig.AuthProvider = "service-mesh"
    
    return options
}
```

### Edge Computing Configuration

```go
func EdgeConfig() state.ManagerOptions {
    options := state.DefaultManagerOptions()
    
    // Local file storage for edge
    options.StorageBackend = "file"
    options.StorageConfig = &state.StorageConfig{
        Path:        "/opt/agui/edge-state",
        Compression: true,
        SyncWrites:  false, // Async writes for performance
    }
    
    // Small cache for edge devices
    options.CacheSize = 100
    
    // Minimal history
    options.MaxHistorySize = 10
    options.MaxCheckpoints = 3
    
    // Reduced monitoring
    options.EnableMetrics = false
    options.EnableTracing = false
    options.MonitoringConfig.LogLevel = zapcore.WarnLevel
    
    // Offline capability
    options.OfflineConfig = state.OfflineConfig{
        EnableOfflineMode: true,
        SyncInterval:     5 * time.Minute,
        ConflictStrategy: state.LastWriteWins,
    }
    
    return options
}
```

This configuration guide provides comprehensive coverage of all configuration options available in the AG-UI state management system, with practical examples for different deployment scenarios and use cases.