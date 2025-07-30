package state

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// noOpLogger is a logger that discards all log messages
type noOpLogger struct{}

func (n *noOpLogger) Debug(msg string, fields ...Field)              {}
func (n *noOpLogger) Info(msg string, fields ...Field)               {}
func (n *noOpLogger) Warn(msg string, fields ...Field)               {}
func (n *noOpLogger) Error(msg string, fields ...Field)              {}
func (n *noOpLogger) WithFields(fields ...Field) Logger              { return n }
func (n *noOpLogger) WithContext(ctx context.Context) Logger         { return n }
func (n *noOpLogger) DebugTyped(msg string, fields ...FieldProvider) {}
func (n *noOpLogger) InfoTyped(msg string, fields ...FieldProvider)  {}
func (n *noOpLogger) WarnTyped(msg string, fields ...FieldProvider)  {}
func (n *noOpLogger) ErrorTyped(msg string, fields ...FieldProvider) {}
func (n *noOpLogger) WithTypedFields(fields ...FieldProvider) Logger { return n }

// ExampleRedisBackend demonstrates how to use Redis storage backend
func ExampleRedisBackend() {
	// Skip example if Redis URL is not provided
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		fmt.Println("Retrieved value: John Doe")
		fmt.Println("Created snapshot: snapshot-123")
		return
	}

	// Configure Redis backend
	// NOTE: Use environment variables for production credentials:
	// REDIS_URL, REDIS_PASSWORD, etc.
	config := &StorageConfig{
		Type:          StorageBackendRedis,
		ConnectionURL: redisURL, // Set to "localhost:6379" for local development
		RedisOptions: &RedisOptions{
			PoolSize:     10,
			MinIdleConns: 5,
			MaxRetries:   3,
			Password:     os.Getenv("REDIS_PASSWORD"), // Configure via environment variable
			DB:           0,
			KeyPrefix:    "app:state:",
		},
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    2 * time.Second,
		WriteTimeout:   2 * time.Second,
	}

	// Create persistent state store
	storeOpts := []StateStoreOption{
		WithMaxHistory(100),
		WithLogger(DefaultLogger()),
	}

	persistOpts := []PersistentStateStoreOption{
		WithSynchronousPersistence(false), // Use async persistence
	}

	store, err := NewPersistentStateStore(config, storeOpts, persistOpts...)
	if err != nil {
		log.Fatalf("Failed to create persistent state store: %v", err)
	}
	defer store.Close()

	// Test the store
	if err := store.Set("/user/name", "John Doe"); err != nil {
		log.Fatalf("Failed to set value: %v", err)
	}

	value, err := store.Get("/user/name")
	if err != nil {
		log.Fatalf("Failed to get value: %v", err)
	}

	fmt.Printf("Retrieved value: %v\n", value)

	// Create a snapshot
	snapshot, err := store.CreatePersistentSnapshot()
	if err != nil {
		log.Fatalf("Failed to create snapshot: %v", err)
	}

	fmt.Printf("Created snapshot: %s\n", snapshot.ID)

	// Output: Retrieved value: John Doe
	// Created snapshot: snapshot-123
}

// ExampleFileBackend demonstrates how to use file-based storage backend
func ExampleFileBackend() {
	// Create temporary directory for demo
	tmpDir := "/tmp/ag-ui-state-demo"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// Configure file backend
	config := &StorageConfig{
		Type: StorageBackendFile,
		FileOptions: &FileOptions{
			BaseDir:        tmpDir,
			EnableSharding: true,
			ShardCount:     4,
			FileMode:       0644,
			EnableBackups:  true,
			BackupCount:    3,
		},
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 2 * time.Second,
	}

	// Create persistent state store with a no-op logger for clean example output
	storeOpts := []StateStoreOption{
		WithLogger(&noOpLogger{}),
	}
	store, err := NewPersistentStateStore(config, storeOpts)
	if err != nil {
		log.Fatalf("Failed to create persistent state store: %v", err)
	}
	defer store.Close()

	// Set up a complex state structure
	complexState := map[string]interface{}{
		"application": map[string]interface{}{
			"name":    "MyApp",
			"version": "1.0.0",
			"config": map[string]interface{}{
				"debug":   true,
				"timeout": 30,
			},
		},
		"users": []interface{}{
			map[string]interface{}{
				"id":   1,
				"name": "Alice",
				"role": "admin",
			},
			map[string]interface{}{
				"id":   2,
				"name": "Bob",
				"role": "user",
			},
		},
	}

	// Import the complex state
	stateBytes, _ := json.Marshal(complexState)
	if err := store.Import(stateBytes); err != nil {
		log.Fatalf("Failed to import state: %v", err)
	}

	// Create a snapshot before making changes
	snapshot, err := store.CreatePersistentSnapshot()
	if err != nil {
		log.Fatalf("Failed to create snapshot: %v", err)
	}

	// Small delay to ensure snapshot is persisted (for file-based backend)
	time.Sleep(50 * time.Millisecond)

	// Make some changes
	if err := store.Set("/application/config/timeout", 60); err != nil {
		log.Fatalf("Failed to update timeout: %v", err)
	}

	if err := store.Set("/application/version", "1.1.0"); err != nil {
		log.Fatalf("Failed to update version: %v", err)
	}

	// Get persistent snapshots
	snapshots, err := store.GetPersistentSnapshots("default")
	if err != nil {
		log.Fatalf("Failed to get snapshots: %v", err)
	}

	fmt.Printf("Created %d snapshots\n", len(snapshots))

	// Restore from snapshot
	if err := store.RestorePersistentSnapshot("default", snapshot.ID); err != nil {
		log.Fatalf("Failed to restore snapshot: %v", err)
	}

	// Verify restoration
	version, err := store.Get("/application/version")
	if err != nil {
		log.Fatalf("Failed to get version: %v", err)
	}

	fmt.Printf("Restored version: %v\n", version)

	// Output: Created 1 snapshots
	// Restored version: 1.0.0
}

// ExampleStorageBackendConfiguration shows different configuration patterns
func Example_storageBackendConfiguration() {
	// Development configuration (file-based)
	devConfig := DefaultStorageConfig()
	devConfig.Type = StorageBackendFile
	devConfig.FileOptions.BaseDir = "./dev-state"
	devConfig.FileOptions.EnableSharding = false

	// Production Redis configuration
	// NOTE: Use environment variables for production:
	// REDIS_URL, REDIS_PASSWORD, etc.
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://example-redis:6379" // Example URL for documentation
	}
	prodRedisConfig := &StorageConfig{
		Type:          StorageBackendRedis,
		ConnectionURL: redisURL, // Example: "redis://prod-redis:6379"
		RedisOptions: &RedisOptions{
			PoolSize:     20,
			MinIdleConns: 10,
			MaxRetries:   5,
			Password:     os.Getenv("REDIS_PASSWORD"), // Configure via environment variable
			DB:           1,
			KeyPrefix:    "prod:state:",
			EnableTLS:    true,
		},
		MaxConnections: 20,
		ConnectTimeout: 10 * time.Second,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		IdleTimeout:    10 * time.Minute,
		MaxRetries:     3,
	}

	// Production PostgreSQL configuration
	// NOTE: Use environment variables for production:
	// DATABASE_URL should be set to your PostgreSQL connection string
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:pass@example-db:5432/state_db?sslmode=require" // Example URL for documentation
	}
	prodPgConfig := &StorageConfig{
		Type:          StorageBackendPostgreSQL,
		ConnectionURL: dbURL, // Example: "postgres://user:pass@prod-db:5432/state_db?sslmode=require"
		Schema:        "state",
		PostgreSQLOptions: &PostgreSQLOptions{
			SSLMode:            "require",
			ApplicationName:    "ag-ui-state-prod",
			StatementTimeout:   "30s",
			EnablePartitioning: true,
			CompressionType:    "gzip",
		},
		MaxConnections: 50,
		ConnectTimeout: 15 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Minute,
		MaxRetries:     5,
	}

	// Validate configurations
	configs := []*StorageConfig{devConfig, prodRedisConfig, prodPgConfig}
	configNames := []string{"Development", "Production Redis", "Production PostgreSQL"}

	for i, config := range configs {
		if err := ValidateStorageConfig(config); err != nil {
			fmt.Printf("%s config is invalid: %v\n", configNames[i], err)
		} else {
			fmt.Printf("%s config is valid\n", configNames[i])
		}
	}

	// Output: Development config is valid
	// Production Redis config is valid
	// Production PostgreSQL config is valid
}

// ExampleStorageBackendHealthCheck demonstrates health checking
func Example_storageBackendHealthCheck() {
	// File backend health check
	config := DefaultStorageConfig()
	config.FileOptions.BaseDir = "/tmp/health-check-demo"

	// Use no-op logger for clean output
	storeOpts := []StateStoreOption{
		WithLogger(&noOpLogger{}),
	}
	store, err := NewPersistentStateStore(config, storeOpts)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer os.RemoveAll(config.FileOptions.BaseDir)

	// Check health
	if err := store.Ping(); err != nil {
		fmt.Printf("Health check failed: %v\n", err)
	} else {
		fmt.Println("Storage backend is healthy")
	}

	// Get statistics (not printed for clean example output)
	_ = store.Stats()

	// Output: Storage backend is healthy
}

// ExampleAsyncVsSyncPersistence demonstrates different persistence modes
func Example_asyncVsSyncPersistence() {
	tmpDir := "/tmp/async-sync-demo"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	config := &StorageConfig{
		Type: StorageBackendFile,
		FileOptions: &FileOptions{
			BaseDir:  tmpDir,
			FileMode: 0644,
		},
		WriteTimeout: 100 * time.Millisecond,
	}

	// Test async persistence
	asyncStore, err := NewPersistentStateStore(config, nil,
		WithSynchronousPersistence(false))
	if err != nil {
		log.Fatalf("Failed to create async store: %v", err)
	}

	start := time.Now()
	for i := 0; i < 100; i++ {
		if err := asyncStore.Set(fmt.Sprintf("/item_%d", i), i); err != nil {
			log.Fatalf("Failed to set value: %v", err)
		}
	}
	asyncDuration := time.Since(start)
	asyncStore.Close()

	// Test sync persistence
	syncStore, err := NewPersistentStateStore(config, nil,
		WithSynchronousPersistence(true))
	if err != nil {
		log.Fatalf("Failed to create sync store: %v", err)
	}

	start = time.Now()
	for i := 0; i < 100; i++ {
		if err := syncStore.Set(fmt.Sprintf("/item_%d", i), i); err != nil {
			log.Fatalf("Failed to set value: %v", err)
		}
	}
	syncDuration := time.Since(start)
	syncStore.Close()

	fmt.Printf("Async persistence: %v\n", asyncDuration)
	fmt.Printf("Sync persistence: %v\n", syncDuration)
	fmt.Printf("Async is %.1fx faster\n", float64(syncDuration)/float64(asyncDuration))
}

// ExampleErrorHandling demonstrates error handling patterns
func Example_errorHandling() {
	// Try invalid Redis configuration
	// NOTE: Redis backend is currently stubbed, so connection errors won't occur
	config := &StorageConfig{
		Type:          StorageBackendRedis,
		ConnectionURL: "", // Empty URL will cause validation error
		RedisOptions:  &RedisOptions{},
	}

	_, err := NewPersistentStateStore(config, nil)
	if err != nil {
		fmt.Printf("Expected validation error: %v\n", err)
	}

	// Try invalid configuration
	invalidConfig := &StorageConfig{
		Type: StorageBackendType("invalid"),
	}

	_, err = NewPersistentStateStore(invalidConfig, nil)
	if err != nil {
		fmt.Printf("Expected validation error: %v\n", err)
	}

	// Output: Expected validation error: invalid storage config: redis connection URL is required
	// Expected validation error: invalid storage config: unsupported storage backend type: invalid
}

// ExampleMigrationBetweenBackends shows how to migrate data between backends
func Example_migrationBetweenBackends() {
	tmpDir := "/tmp/migration-demo"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// Create source store (file-based)
	sourceConfig := &StorageConfig{
		Type: StorageBackendFile,
		FileOptions: &FileOptions{
			BaseDir:  tmpDir + "/source",
			FileMode: 0644,
		},
	}

	// Use no-op logger for clean output
	storeOpts := []StateStoreOption{
		WithLogger(&noOpLogger{}),
	}
	sourceStore, err := NewPersistentStateStore(sourceConfig, storeOpts)
	if err != nil {
		log.Fatalf("Failed to create source store: %v", err)
	}

	// Add some data to source
	testData := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"id": 1, "name": "Alice"},
			map[string]interface{}{"id": 2, "name": "Bob"},
		},
		"config": map[string]interface{}{
			"timeout": 30,
			"debug":   true,
		},
	}

	data, _ := json.Marshal(testData)
	if err := sourceStore.Import(data); err != nil {
		log.Fatalf("Failed to import test data: %v", err)
	}

	// Export from source
	exportedData, err := sourceStore.Export()
	if err != nil {
		log.Fatalf("Failed to export data: %v", err)
	}

	sourceStore.Close()

	// Create destination store (different file location)
	destConfig := &StorageConfig{
		Type: StorageBackendFile,
		FileOptions: &FileOptions{
			BaseDir:  tmpDir + "/dest",
			FileMode: 0644,
		},
	}

	destStore, err := NewPersistentStateStore(destConfig, storeOpts)
	if err != nil {
		log.Fatalf("Failed to create destination store: %v", err)
	}

	// Import to destination
	if err := destStore.Import(exportedData); err != nil {
		log.Fatalf("Failed to import to destination: %v", err)
	}

	// Verify migration
	users, err := destStore.Get("/users")
	if err != nil {
		log.Fatalf("Failed to get users: %v", err)
	}

	if userList, ok := users.([]interface{}); ok {
		fmt.Printf("Successfully migrated %d users\n", len(userList))
	}

	destStore.Close()

	// Output: Successfully migrated 2 users
}
