# Storage Backends for AG-UI State Management

This document describes the pluggable storage backend system for the AG-UI state management library.

## Overview

The state management system supports multiple storage backends:

- **File Backend**: Persistent storage using the local filesystem (production-ready)
- **Redis Backend**: High-performance in-memory storage with persistence (mock implementation provided)
- **PostgreSQL Backend**: Relational database storage with ACID guarantees (mock implementation provided)

## Architecture

The storage system follows a pluggable architecture with the following components:

### Core Interfaces

```go
type StorageBackend interface {
    // State operations
    GetState(ctx context.Context, stateID string) (map[string]interface{}, error)
    SetState(ctx context.Context, stateID string, state map[string]interface{}) error
    DeleteState(ctx context.Context, stateID string) error
    
    // Version operations
    GetVersion(ctx context.Context, stateID string, versionID string) (*StateVersion, error)
    SaveVersion(ctx context.Context, stateID string, version *StateVersion) error
    GetVersionHistory(ctx context.Context, stateID string, limit int) ([]*StateVersion, error)
    
    // Snapshot operations
    GetSnapshot(ctx context.Context, stateID string, snapshotID string) (*StateSnapshot, error)
    SaveSnapshot(ctx context.Context, stateID string, snapshot *StateSnapshot) error
    ListSnapshots(ctx context.Context, stateID string) ([]*StateSnapshot, error)
    
    // Transaction operations
    BeginTransaction(ctx context.Context) (Transaction, error)
    
    // Housekeeping
    Close() error
    Ping(ctx context.Context) error
    Stats() map[string]interface{}
}

type Transaction interface {
    SetState(ctx context.Context, stateID string, state map[string]interface{}) error
    SaveVersion(ctx context.Context, stateID string, version *StateVersion) error
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}
```

### Integration with StateStore

The `PersistentStateStore` wraps the in-memory `StateStore` and provides:

- Automatic persistence to storage backends
- Synchronous and asynchronous persistence modes
- Transaction support across memory and storage
- Health checking and statistics
- Graceful shutdown and error handling

## File Backend (Production Ready)

The file backend provides persistent storage using the local filesystem with the following features:

### Features

- **Sharding**: Distributes files across multiple directories for better performance
- **Backups**: Automatic backup creation before updates
- **Transactions**: Atomic file operations
- **JSON Storage**: Human-readable state files
- **Error Recovery**: Handles filesystem errors gracefully

### Configuration

```go
config := &StorageConfig{
    Type: StorageBackendFile,
    FileOptions: &FileOptions{
        BaseDir:         "/var/lib/ag-ui/state",
        EnableSharding:  true,
        ShardCount:      16,
        FileMode:        0644,
        EnableBackups:   true,
        BackupCount:     5,
    },
    ReadTimeout:  time.Second,
    WriteTimeout: 2 * time.Second,
}
```

### Usage Example

```go
// Create file-based persistent store
store, err := NewPersistentStateStore(config, nil)
if err != nil {
    log.Fatal(err)
}
defer store.Close()

// Use like a regular StateStore
err = store.Set("/user/name", "John Doe")
if err != nil {
    log.Fatal(err)
}

// Create persistent snapshots
snapshot, err := store.CreatePersistentSnapshot()
if err != nil {
    log.Fatal(err)
}

// Get snapshots from storage
snapshots, err := store.GetPersistentSnapshots("default")
if err != nil {
    log.Fatal(err)
}
```

### Directory Structure

```
/var/lib/ag-ui/state/
├── states/
│   ├── shard_0/
│   │   └── state1.json
│   ├── shard_1/
│   │   └── state2.json
│   └── ...
├── versions/
│   ├── shard_0/
│   │   ├── version1.json
│   │   └── version2.json
│   └── ...
└── snapshots/
    ├── shard_0/
    │   ├── snapshot1.json
    │   └── snapshot2.json
    └── ...
```

## Redis Backend (Mock Implementation)

The Redis backend provides high-performance in-memory storage. Currently, a mock implementation is provided for development and testing.

### Features (Mock)

- Thread-safe in-memory storage using `sync.Map`
- Compatible API with Redis backend
- Suitable for development and testing

### Production Implementation

To use a real Redis backend, you would need to:

1. Add Redis dependencies:
```bash
go get github.com/go-redis/redis/v8
```

2. Implement the actual Redis backend in `storage.go`
3. Update the factory function to use the real implementation

### Configuration

```go
config := &StorageConfig{
    Type:          StorageBackendRedis,
    ConnectionURL: "localhost:6379",
    RedisOptions: &RedisOptions{
        PoolSize:     10,
        MinIdleConns: 5,
        MaxRetries:   3,
        Password:     "",
        DB:           0,
        KeyPrefix:    "app:state:",
    },
    ConnectTimeout: 5 * time.Second,
    ReadTimeout:    2 * time.Second,
    WriteTimeout:   2 * time.Second,
}
```

## PostgreSQL Backend (Mock Implementation)

The PostgreSQL backend provides ACID-compliant relational database storage. Currently, a mock implementation is provided.

### Features (Mock)

- Thread-safe in-memory storage
- Compatible API with PostgreSQL backend
- Suitable for development and testing

### Production Implementation

To use a real PostgreSQL backend, you would need to:

1. Add PostgreSQL dependencies:
```bash
go get github.com/lib/pq
```

2. Implement the actual PostgreSQL backend in `storage.go`
3. Set up database schema and tables

### Configuration

```go
config := &StorageConfig{
    Type:          StorageBackendPostgreSQL,
    ConnectionURL: os.Getenv("DATABASE_URL"), // Example: "postgres://user:password@localhost/dbname?sslmode=disable"
    Schema:        "public",
    PostgreSQLOptions: &PostgreSQLOptions{
        SSLMode:         "disable",
        ApplicationName: "ag-ui-state",
    },
    MaxConnections: 10,
    ConnectTimeout: 10 * time.Second,
    ReadTimeout:    5 * time.Second,
    WriteTimeout:   5 * time.Second,
}
```

## Persistence Modes

### Synchronous Persistence

In synchronous mode, all state changes are immediately persisted to storage before returning.

```go
store, err := NewPersistentStateStore(config, nil,
    WithSynchronousPersistence(true))
```

**Pros:**
- Guaranteed consistency
- No data loss risk
- Immediate error feedback

**Cons:**
- Higher latency
- Blocking operations

### Asynchronous Persistence

In asynchronous mode, state changes are queued for background persistence.

```go
store, err := NewPersistentStateStore(config, nil,
    WithSynchronousPersistence(false))
```

**Pros:**
- Lower latency
- Non-blocking operations
- Better throughput

**Cons:**
- Eventual consistency
- Potential data loss on system failure
- Delayed error reporting

## Transactions

The storage backends support transactions for atomic operations:

```go
// Begin a transaction
tx, err := store.BeginPersistentTransaction()
if err != nil {
    log.Fatal(err)
}

// Apply multiple operations
patch := JSONPatch{
    {Op: JSONPatchOpAdd, Path: "/config/timeout", Value: 30},
    {Op: JSONPatchOpAdd, Path: "/config/retries", Value: 3},
}

if err := tx.Apply(patch); err != nil {
    tx.Rollback()
    log.Fatal(err)
}

// Commit the transaction
if err := tx.Commit(); err != nil {
    log.Fatal(err)
}
```

## Health Checking and Monitoring

### Health Checks

```go
// Check storage backend health
if err := store.Ping(); err != nil {
    log.Printf("Storage backend unhealthy: %v", err)
}
```

### Statistics

```go
// Get storage statistics
stats := store.Stats()
fmt.Printf("Backend type: %s\n", stats["backend_type"])
fmt.Printf("Memory version: %v\n", stats["memory_version"])
fmt.Printf("Sync queue size: %v\n", stats["sync_queue_size"])
```

## Error Handling

The storage backends provide comprehensive error handling:

- Connection errors
- Timeout errors
- Validation errors
- Transaction errors
- File system errors

Errors are logged and can be handled through error callbacks:

```go
store.SetErrorHandler(func(err error) {
    log.Printf("Storage error: %v", err)
    // Handle error (e.g., send alerts, retry logic)
})
```

## Best Practices

### Configuration

1. **Choose appropriate timeouts** based on your performance requirements
2. **Enable sharding** for file backend with high write volumes
3. **Configure connection pooling** for database backends
4. **Set up monitoring** and health checks

### Performance

1. **Use asynchronous persistence** for high-throughput applications
2. **Batch operations** when possible
3. **Monitor queue sizes** and adjust buffer sizes accordingly
4. **Consider backup strategies** for production deployments

### Security

1. **Use secure connection strings** with proper authentication
2. **Set appropriate file permissions** for file backend
3. **Enable SSL/TLS** for network-based backends
4. **Validate configurations** before deployment

## Migration Between Backends

You can migrate data between different storage backends:

```go
// Export from source backend
sourceStore, _ := NewPersistentStateStore(sourceConfig, nil)
data, err := sourceStore.Export()
if err != nil {
    log.Fatal(err)
}
sourceStore.Close()

// Import to destination backend
destStore, _ := NewPersistentStateStore(destConfig, nil)
err = destStore.Import(data)
if err != nil {
    log.Fatal(err)
}
destStore.Close()
```

## Extending the System

To add a new storage backend:

1. Implement the `StorageBackend` interface
2. Implement the `Transaction` interface
3. Add configuration options
4. Update the factory function
5. Add tests and documentation

Example skeleton:

```go
type MyCustomBackend struct {
    config *StorageConfig
    logger Logger
}

func NewMyCustomBackend(config *StorageConfig, logger Logger) (*MyCustomBackend, error) {
    // Initialize your backend
    return &MyCustomBackend{config: config, logger: logger}, nil
}

func (m *MyCustomBackend) GetState(ctx context.Context, stateID string) (map[string]interface{}, error) {
    // Implement state retrieval
    return nil, nil
}

// Implement other required methods...
```

## Troubleshooting

### Common Issues

1. **Permission denied**: Check file permissions and directory access
2. **Connection refused**: Verify connection URLs and network connectivity
3. **Timeout errors**: Adjust timeout configurations
4. **Queue full**: Increase buffer sizes or switch to synchronous mode
5. **Disk space**: Monitor available disk space for file backend

### Debugging

Enable debug logging to troubleshoot issues:

```go
logger := NewLogger(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

store, err := NewPersistentStateStore(config, nil,
    WithPersistentLogger(logger))
```

## Conclusion

The pluggable storage backend system provides flexibility and scalability for the AG-UI state management system. Choose the appropriate backend based on your requirements:

- **File Backend**: For single-node deployments with persistence requirements
- **Redis Backend**: For high-performance distributed systems
- **PostgreSQL Backend**: For applications requiring ACID guarantees and complex queries

The system is designed to be extensible, allowing you to implement custom backends as needed.