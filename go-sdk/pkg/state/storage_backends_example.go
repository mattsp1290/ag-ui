package state

import (
	"fmt"
	"log"
	"os"
	"time"
)

// ExamplePostgreSQLBackend demonstrates how to use PostgreSQL storage backend
func ExamplePostgreSQLBackend() {
	// Get connection string from environment variable with safe default
	connStr := os.Getenv("POSTGRES_CONN_STRING")
	if connStr == "" {
		// Safe default without credentials
		connStr = "postgres://localhost:5432/statedb"
		log.Println("Warning: Using default PostgreSQL connection string. Set POSTGRES_CONN_STRING environment variable for production use.")
	}

	// Configure PostgreSQL backend
	config := &StorageConfig{
		Type:          StorageBackendPostgreSQL,
		ConnectionURL: connStr,
		Schema:        "public",
		PostgreSQLOptions: &PostgreSQLOptions{
			SSLMode:         "disable",
			ApplicationName: "ag-ui-state-example",
		},
		MaxConnections: 10,
		ConnectTimeout: 10 * time.Second,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
	}

	// Create persistent state store
	storeOpts := []StateStoreOption{
		WithMaxHistory(50),
		WithShardCount(8),
	}

	persistOpts := []PersistentStateStoreOption{
		WithSynchronousPersistence(true), // Use sync persistence for critical data
	}

	store, err := NewPersistentStateStore(config, storeOpts, persistOpts...)
	if err != nil {
		log.Fatalf("Failed to create persistent state store: %v", err)
	}
	defer store.Close()

	// Use transactions for atomic operations
	tx, err := store.BeginPersistentTransaction()
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}

	// Apply multiple operations in a transaction
	patch := JSONPatch{
		{Op: JSONPatchOpAdd, Path: "/config/timeout", Value: 30},
		{Op: JSONPatchOpAdd, Path: "/config/retries", Value: 3},
		{Op: JSONPatchOpAdd, Path: "/config/debug", Value: true},
	}

	if err := tx.Apply(patch); err != nil {
		tx.Rollback()
		log.Fatalf("Failed to apply patch: %v", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify the changes
	state := store.GetState()
	fmt.Printf("Final state: %+v\n", state)
}

// ExamplePostgreSQLWithCustomConfig shows PostgreSQL configuration with environment variables
func ExamplePostgreSQLWithCustomConfig() {
	// Get connection string from environment variable
	connStr := os.Getenv("POSTGRES_CONN_STRING")
	if connStr == "" {
		connStr = "postgres://localhost:5432/statedb"
	}

	// Get other configuration from environment
	sslMode := os.Getenv("POSTGRES_SSL_MODE")
	if sslMode == "" {
		sslMode = "disable"
	}

	appName := os.Getenv("APP_NAME")
	if appName == "" {
		appName = "ag-ui-state"
	}

	config := &StorageConfig{
		Type:          StorageBackendPostgreSQL,
		ConnectionURL: connStr,
		Schema:        "public",
		PostgreSQLOptions: &PostgreSQLOptions{
			SSLMode:         sslMode,
			ApplicationName: appName,
		},
		MaxConnections: 10,
		ConnectTimeout: 10 * time.Second,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
	}

	store, err := NewPersistentStateStore(config, nil)
	if err != nil {
		log.Printf("Failed to create store: %v", err)
		return
	}
	defer store.Close()

	// Example usage
	if err := store.Set("/app/version", "1.0.0"); err != nil {
		log.Printf("Failed to set value: %v", err)
		return
	}

	fmt.Println("Successfully connected and stored data")
}

// ExampleProductionPostgreSQLSetup demonstrates production-ready PostgreSQL setup
func ExampleProductionPostgreSQLSetup() {
	// In production, always use environment variables for sensitive configuration
	connStr := os.Getenv("POSTGRES_CONN_STRING")
	if connStr == "" {
		log.Fatal("POSTGRES_CONN_STRING environment variable must be set")
	}

	// Production configuration with all security features enabled
	prodPgConfig := &StorageConfig{
		Type:          StorageBackendPostgreSQL,
		ConnectionURL: connStr, // Uses environment variable
		Schema:        "state",
		PostgreSQLOptions: &PostgreSQLOptions{
			SSLMode:            "require", // Always use SSL in production
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

	// Validate configuration before use
	if err := ValidateStorageConfig(prodPgConfig); err != nil {
		log.Fatalf("Invalid storage configuration: %v", err)
	}

	store, err := NewPersistentStateStore(prodPgConfig, nil,
		WithSynchronousPersistence(true))
	if err != nil {
		log.Fatalf("Failed to create persistent state store: %v", err)
	}
	defer store.Close()

	// Health check
	if err := store.Ping(); err != nil {
		log.Fatalf("Storage backend health check failed: %v", err)
	}

	fmt.Println("Production PostgreSQL storage backend initialized successfully")
}

// ExampleRedisBackendWithEnv demonstrates Redis configuration with environment variables
func ExampleRedisBackendWithEnv() {
	// Get Redis connection URL from environment
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
		log.Println("Warning: Using default Redis URL. Set REDIS_URL environment variable for production use.")
	}

	// Get Redis password from environment (optional)
	redisPassword := os.Getenv("REDIS_PASSWORD")

	// Configure Redis backend
	config := &StorageConfig{
		Type:          StorageBackendRedis,
		ConnectionURL: redisURL,
		RedisOptions: &RedisOptions{
			PoolSize:     10,
			MinIdleConns: 5,
			MaxRetries:   3,
			Password:     redisPassword, // From environment variable
			DB:           0,
			KeyPrefix:    "app:state:",
		},
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    2 * time.Second,
		WriteTimeout:   2 * time.Second,
	}

	// Create persistent state store
	store, err := NewPersistentStateStore(config, nil)
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
}

// ExampleStorageBackendMigration shows how to migrate between backends using environment configuration
func ExampleStorageBackendMigration() {
	// Source backend configuration from environment
	sourceType := os.Getenv("MIGRATION_SOURCE_TYPE")
	if sourceType == "" {
		sourceType = "file"
	}

	// Destination backend configuration from environment
	destConnStr := os.Getenv("MIGRATION_DEST_POSTGRES")
	if destConnStr == "" {
		destConnStr = "postgres://localhost:5432/statedb"
		log.Println("Warning: Using default destination PostgreSQL. Set MIGRATION_DEST_POSTGRES for production.")
	}

	// Create source store based on type
	var sourceConfig *StorageConfig
	switch sourceType {
	case "file":
		sourceConfig = &StorageConfig{
			Type: StorageBackendFile,
			FileOptions: &FileOptions{
				BaseDir:  "/tmp/state-migration-source",
				FileMode: 0600,
			},
		}
	case "redis":
		redisURL := os.Getenv("MIGRATION_SOURCE_REDIS")
		if redisURL == "" {
			redisURL = "localhost:6379"
		}
		sourceConfig = &StorageConfig{
			Type:          StorageBackendRedis,
			ConnectionURL: redisURL,
			RedisOptions:  &RedisOptions{},
		}
	default:
		log.Fatalf("Unsupported source type: %s", sourceType)
	}

	sourceStore, err := NewPersistentStateStore(sourceConfig, nil)
	if err != nil {
		log.Fatalf("Failed to create source store: %v", err)
	}
	defer sourceStore.Close()

	// Export data from source
	exportedData, err := sourceStore.Export()
	if err != nil {
		log.Fatalf("Failed to export data: %v", err)
	}

	// Create destination PostgreSQL store
	destConfig := &StorageConfig{
		Type:          StorageBackendPostgreSQL,
		ConnectionURL: destConnStr,
		Schema:        "migrated",
		PostgreSQLOptions: &PostgreSQLOptions{
			SSLMode:         "disable",
			ApplicationName: "state-migration",
		},
	}

	destStore, err := NewPersistentStateStore(destConfig, nil)
	if err != nil {
		log.Fatalf("Failed to create destination store: %v", err)
	}
	defer destStore.Close()

	// Import to destination
	if err := destStore.Import(exportedData); err != nil {
		log.Fatalf("Failed to import to destination: %v", err)
	}

	fmt.Println("Migration completed successfully")
}
