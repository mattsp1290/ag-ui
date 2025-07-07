// Package main demonstrates the use of different storage backends for state persistence
// including Redis, PostgreSQL, and File-based storage with production configurations.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ag-ui/go-sdk/pkg/state"
)

// Example application state
type AppState struct {
	Users      map[string]User      `json:"users"`
	Sessions   map[string]Session   `json:"sessions"`
	Metrics    MetricsData          `json:"metrics"`
	LastUpdate time.Time            `json:"lastUpdate"`
}

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	Settings  map[string]interface{} `json:"settings"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Metadata  map[string]string `json:"metadata"`
}

type MetricsData struct {
	ActiveUsers   int     `json:"activeUsers"`
	RequestCount  int64   `json:"requestCount"`
	ResponseTime  float64 `json:"responseTime"`
	ErrorRate     float64 `json:"errorRate"`
}

func main() {
	ctx := context.Background()
	
	// SECURITY NOTE: This example uses environment variables for database credentials.
	// Never hardcode credentials in your source code.
	// Set POSTGRES_CONN_STRING and other connection env vars before running in production.
	
	// Demonstrate different storage backends
	fmt.Println("=== Storage Backend Examples ===\n")
	
	// 1. File-based storage (great for development and small deployments)
	demonstrateFileStorage(ctx)
	
	// 2. Redis storage (ideal for high-performance distributed systems)
	demonstrateRedisStorage(ctx)
	
	// 3. PostgreSQL storage (best for durability and complex queries)
	demonstratePostgreSQLStorage(ctx)
	
	// 4. Hybrid storage with fallback
	demonstrateHybridStorage(ctx)
	
	// 5. Performance comparison
	compareStoragePerformance(ctx)
}

func demonstrateFileStorage(ctx context.Context) {
	fmt.Println("1. File-Based Storage Example")
	fmt.Println("-----------------------------")
	
	// Configure file storage
	storageConfig := &state.StorageConfig{
		Type:              state.StorageTypeFile,
		ConnectionURL:     "./state_data",
		MaxConnections:    10,
		ConnectionTimeout: 5 * time.Second,
		RetryPolicy: state.RetryPolicy{
			MaxRetries:     3,
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     5 * time.Second,
			Multiplier:     2.0,
		},
		Compression: state.CompressionConfig{
			Enabled:       true,
			Algorithm:     "gzip",
			Level:         6,
			MinSizeBytes:  1024, // Only compress if data > 1KB
		},
		Encryption: state.EncryptionConfig{
			Enabled:   true,
			Algorithm: "AES-256-GCM",
			KeyID:     "file-storage-key-1",
		},
	}
	
	// Create storage backend
	logger := state.NewLogger(state.LogLevelInfo)
	storage, err := state.NewFileBackend(storageConfig, logger)
	if err != nil {
		log.Printf("Failed to create file storage: %v", err)
		return
	}
	defer storage.Close()
	
	// Create state store with file backend
	store := state.NewStateStore(
		state.WithStorageBackend(storage),
		state.WithMaxHistory(50),
		state.WithLogger(logger),
	)
	
	// Initialize application state
	appState := createSampleAppState()
	
	// Save state
	if err := saveAppState(ctx, store, storage, appState); err != nil {
		log.Printf("Failed to save state: %v", err)
		return
	}
	
	// Simulate updates
	for i := 0; i < 5; i++ {
		// Update metrics
		appState.Metrics.RequestCount += int64(100 + i*50)
		appState.Metrics.ResponseTime = 15.5 + float64(i)*2.3
		appState.LastUpdate = time.Now()
		
		// Save updated state
		if err := saveAppState(ctx, store, storage, appState); err != nil {
			log.Printf("Failed to save updated state: %v", err)
			continue
		}
		
		fmt.Printf("  Update %d: Saved state (requests: %d, response time: %.2fms)\n", 
			i+1, appState.Metrics.RequestCount, appState.Metrics.ResponseTime)
		
		time.Sleep(500 * time.Millisecond)
	}
	
	// Load state from storage
	loadedState, err := loadAppState(ctx, storage, "app-state")
	if err != nil {
		log.Printf("Failed to load state: %v", err)
		return
	}
	
	fmt.Printf("\n  Loaded state from file storage:\n")
	fmt.Printf("    Active users: %d\n", loadedState.Metrics.ActiveUsers)
	fmt.Printf("    Total requests: %d\n", loadedState.Metrics.RequestCount)
	fmt.Printf("    Last update: %s\n\n", loadedState.LastUpdate.Format(time.RFC3339))
}

func demonstrateRedisStorage(ctx context.Context) {
	fmt.Println("2. Redis Storage Example")
	fmt.Println("------------------------")
	
	// Configure Redis storage
	storageConfig := &state.StorageConfig{
		Type:              state.StorageTypeRedis,
		ConnectionURL:     "redis://localhost:6379/0",
		MaxConnections:    100,
		ConnectionTimeout: 10 * time.Second,
		RetryPolicy: state.RetryPolicy{
			MaxRetries:     5,
			InitialBackoff: 50 * time.Millisecond,
			MaxBackoff:     2 * time.Second,
			Multiplier:     1.5,
		},
		Compression: state.CompressionConfig{
			Enabled:       true,
			Algorithm:     "snappy", // Fast compression for Redis
			Level:         1,
			MinSizeBytes:  512,
		},
		CacheTTL: 5 * time.Minute,
		Options: map[string]interface{}{
			"pool_timeout":     30 * time.Second,
			"idle_timeout":     10 * time.Minute,
			"max_idle_conns":   50,
			"enable_pipelining": true,
		},
	}
	
	// Create Redis storage backend (using mock for demo)
	logger := state.NewLogger(state.LogLevelInfo)
	storage, err := state.NewMockRedisBackend(storageConfig, logger)
	if err != nil {
		log.Printf("Failed to create Redis storage: %v", err)
		return
	}
	defer storage.Close()
	
	// Create state store with Redis backend
	store := state.NewStateStore(
		state.WithStorageBackend(storage),
		state.WithMaxHistory(100),
		state.WithSnapshotInterval(30*time.Second),
		state.WithLogger(logger),
	)
	
	// Demonstrate high-frequency updates (typical for Redis)
	fmt.Println("  Simulating high-frequency updates...")
	
	appState := createSampleAppState()
	updateCount := 0
	
	// Simulate real-time metrics updates
	ticker := time.NewTicker(100 * time.Millisecond)
	done := time.After(2 * time.Second)
	
	for {
		select {
		case <-ticker.C:
			// Update real-time metrics
			appState.Metrics.ActiveUsers = 100 + updateCount%50
			appState.Metrics.RequestCount += int64(10 + updateCount%20)
			appState.Metrics.ResponseTime = 10.0 + float64(updateCount%10)*0.5
			appState.Metrics.ErrorRate = float64(updateCount%5) * 0.01
			appState.LastUpdate = time.Now()
			
			// Save to Redis
			if err := saveAppState(ctx, store, storage, appState); err != nil {
				log.Printf("Failed to save state: %v", err)
				continue
			}
			
			updateCount++
			if updateCount%10 == 0 {
				fmt.Printf("    Processed %d updates (active users: %d, error rate: %.2f%%)\n",
					updateCount, appState.Metrics.ActiveUsers, appState.Metrics.ErrorRate*100)
			}
			
		case <-done:
			ticker.Stop()
			fmt.Printf("  Completed %d high-frequency updates\n\n", updateCount)
			return
		}
	}
}

func demonstratePostgreSQLStorage(ctx context.Context) {
	fmt.Println("3. PostgreSQL Storage Example")
	fmt.Println("-----------------------------")
	
	// Configure PostgreSQL storage
	// Use environment variable for connection string, with a safe default without credentials
	connStr := os.Getenv("POSTGRES_CONN_STRING")
	if connStr == "" {
		connStr = "postgres://localhost:5432/statedb?sslmode=disable"
		fmt.Println("NOTE: Using default PostgreSQL connection string. Set POSTGRES_CONN_STRING env var for production use.")
	}
	
	storageConfig := &state.StorageConfig{
		Type:              state.StorageTypePostgreSQL,
		ConnectionURL:     connStr,
		MaxConnections:    50,
		ConnectionTimeout: 30 * time.Second,
		RetryPolicy: state.RetryPolicy{
			MaxRetries:     3,
			InitialBackoff: 200 * time.Millisecond,
			MaxBackoff:     10 * time.Second,
			Multiplier:     2.0,
		},
		Compression: state.CompressionConfig{
			Enabled:       true,
			Algorithm:     "zstd", // Better compression for storage
			Level:         3,
			MinSizeBytes:  2048,
		},
		Encryption: state.EncryptionConfig{
			Enabled:   true,
			Algorithm: "AES-256-GCM",
			KeyID:     "postgres-key-1",
		},
		Options: map[string]interface{}{
			"schema":           "state_management",
			"table_prefix":     "agui_",
			"enable_indexing":  true,
			"vacuum_interval":  24 * time.Hour,
			"retention_days":   90,
		},
	}
	
	// Create PostgreSQL storage backend (using mock for demo)
	logger := state.NewLogger(state.LogLevelInfo)
	storage, err := state.NewMockPostgreSQLBackend(storageConfig, logger)
	if err != nil {
		log.Printf("Failed to create PostgreSQL storage: %v", err)
		return
	}
	defer storage.Close()
	
	// Create state store with PostgreSQL backend
	store := state.NewStateStore(
		state.WithStorageBackend(storage),
		state.WithMaxHistory(1000), // PostgreSQL can handle more history
		state.WithAuditLog(true),
		state.WithLogger(logger),
	)
	
	// Demonstrate complex state with versioning
	fmt.Println("  Creating versioned state with audit trail...")
	
	appState := createSampleAppState()
	
	// Create multiple versions with different users
	users := []string{"alice", "bob", "charlie", "david"}
	for i, username := range users {
		// Add new user
		userID := fmt.Sprintf("user-%d", i+1000)
		appState.Users[userID] = User{
			ID:        userID,
			Username:  username,
			Email:     fmt.Sprintf("%s@example.com", username),
			CreatedAt: time.Now(),
			Settings: map[string]interface{}{
				"theme":         "dark",
				"notifications": true,
				"language":      "en",
			},
		}
		
		// Create session
		sessionID := fmt.Sprintf("session-%d", i+2000)
		appState.Sessions[sessionID] = Session{
			ID:        sessionID,
			UserID:    userID,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Metadata: map[string]string{
				"ip":        fmt.Sprintf("192.168.1.%d", 100+i),
				"userAgent": "Mozilla/5.0",
			},
		}
		
		// Save version
		if err := saveAppState(ctx, store, storage, appState); err != nil {
			log.Printf("Failed to save state: %v", err)
			continue
		}
		
		fmt.Printf("    Version %d: Added user '%s' with session\n", i+1, username)
		time.Sleep(200 * time.Millisecond)
	}
	
	// Query historical data
	fmt.Println("\n  Querying version history...")
	versions, err := storage.GetVersionHistory(ctx, "app-state", 10)
	if err != nil {
		log.Printf("Failed to get version history: %v", err)
		return
	}
	
	fmt.Printf("  Found %d versions in PostgreSQL\n", len(versions))
	for i, version := range versions {
		fmt.Printf("    Version %s: %s (size: %d bytes)\n",
			version.ID[:8], version.Timestamp.Format("15:04:05"), len(version.Data))
	}
	
	fmt.Println()
}

func demonstrateHybridStorage(ctx context.Context) {
	fmt.Println("4. Hybrid Storage with Fallback")
	fmt.Println("--------------------------------")
	
	// Configure primary storage (Redis for speed)
	primaryConfig := &state.StorageConfig{
		Type:              state.StorageTypeRedis,
		ConnectionURL:     "redis://localhost:6379/0",
		MaxConnections:    50,
		ConnectionTimeout: 5 * time.Second,
	}
	
	// Configure fallback storage (PostgreSQL for durability)
	fallbackConnStr := os.Getenv("POSTGRES_FALLBACK_CONN_STRING")
	if fallbackConnStr == "" {
		fallbackConnStr = os.Getenv("POSTGRES_CONN_STRING") // Try primary connection string
		if fallbackConnStr == "" {
			fallbackConnStr = "postgres://localhost:5432/statedb"
		}
	}
	
	fallbackConfig := &state.StorageConfig{
		Type:              state.StorageTypePostgreSQL,
		ConnectionURL:     fallbackConnStr,
		MaxConnections:    20,
		ConnectionTimeout: 10 * time.Second,
	}
	
	// Create hybrid storage
	logger := state.NewLogger(state.LogLevelInfo)
	hybridStorage, err := state.NewHybridStorage(primaryConfig, fallbackConfig, logger)
	if err != nil {
		log.Printf("Failed to create hybrid storage: %v", err)
		return
	}
	defer hybridStorage.Close()
	
	// Create state store with hybrid backend
	store := state.NewStateStore(
		state.WithStorageBackend(hybridStorage),
		state.WithMaxHistory(200),
		state.WithLogger(logger),
	)
	
	fmt.Println("  Testing failover scenarios...")
	
	// Simulate primary storage operations
	appState := createSampleAppState()
	
	// Normal operation (uses primary)
	fmt.Println("  1. Normal operation - writing to primary (Redis)")
	if err := saveAppState(ctx, store, hybridStorage, appState); err != nil {
		log.Printf("Failed to save state: %v", err)
	}
	
	// Simulate primary failure
	fmt.Println("  2. Simulating primary storage failure...")
	hybridStorage.SimulatePrimaryFailure(true)
	
	// Operations should fallback to secondary
	fmt.Println("  3. Automatic fallback to secondary (PostgreSQL)")
	appState.Metrics.ErrorRate = 0.05
	if err := saveAppState(ctx, store, hybridStorage, appState); err != nil {
		log.Printf("Failed to save state during fallback: %v", err)
	}
	
	// Restore primary
	fmt.Println("  4. Primary storage recovered")
	hybridStorage.SimulatePrimaryFailure(false)
	
	// Sync data back to primary
	fmt.Println("  5. Syncing data back to primary...")
	if err := hybridStorage.SyncToPrimary(ctx); err != nil {
		log.Printf("Failed to sync to primary: %v", err)
	}
	
	fmt.Println("  Hybrid storage demonstration complete\n")
}

func compareStoragePerformance(ctx context.Context) {
	fmt.Println("5. Storage Performance Comparison")
	fmt.Println("---------------------------------")
	
	// Create test data
	testState := createLargeTestState(1000) // 1000 users
	
	// Test each storage backend
	backends := []struct {
		name   string
		config *state.StorageConfig
	}{
		{
			name: "File Storage",
			config: &state.StorageConfig{
				Type:          state.StorageTypeFile,
				ConnectionURL: "./perf_test",
			},
		},
		{
			name: "Redis Storage",
			config: &state.StorageConfig{
				Type:          state.StorageTypeRedis,
				ConnectionURL: "redis://localhost:6379/0",
			},
		},
		{
			name: "PostgreSQL Storage",
			config: &state.StorageConfig{
				Type:          state.StorageTypePostgreSQL,
				ConnectionURL: "postgres://localhost:5432/statedb",
			},
		},
	}
	
	fmt.Println("  Running performance benchmarks...")
	fmt.Println("  Test data: 1000 users, ~500KB state size")
	fmt.Println()
	
	for _, backend := range backends {
		fmt.Printf("  %s:\n", backend.name)
		
		// Create storage (using mocks for demo)
		logger := state.NewLogger(state.LogLevelError) // Quiet for benchmarks
		var storage state.StorageBackend
		var err error
		
		switch backend.config.Type {
		case state.StorageTypeFile:
			storage, err = state.NewFileBackend(backend.config, logger)
		case state.StorageTypeRedis:
			storage, err = state.NewMockRedisBackend(backend.config, logger)
		case state.StorageTypePostgreSQL:
			storage, err = state.NewMockPostgreSQLBackend(backend.config, logger)
		}
		
		if err != nil {
			fmt.Printf("    Failed to create backend: %v\n", err)
			continue
		}
		
		// Benchmark write performance
		writeStart := time.Now()
		for i := 0; i < 10; i++ {
			if err := storage.SetState(ctx, "perf-test", testState); err != nil {
				fmt.Printf("    Write error: %v\n", err)
				break
			}
		}
		writeDuration := time.Since(writeStart)
		writeOpsPerSec := float64(10) / writeDuration.Seconds()
		
		// Benchmark read performance
		readStart := time.Now()
		for i := 0; i < 100; i++ {
			if _, err := storage.GetState(ctx, "perf-test"); err != nil {
				fmt.Printf("    Read error: %v\n", err)
				break
			}
		}
		readDuration := time.Since(readStart)
		readOpsPerSec := float64(100) / readDuration.Seconds()
		
		fmt.Printf("    Write performance: %.2f ops/sec (%.2fms avg)\n", 
			writeOpsPerSec, float64(writeDuration.Milliseconds())/10)
		fmt.Printf("    Read performance:  %.2f ops/sec (%.2fms avg)\n", 
			readOpsPerSec, float64(readDuration.Milliseconds())/100)
		fmt.Println()
		
		storage.Close()
	}
	
	fmt.Println("  Performance comparison complete")
}

// Helper functions

func createSampleAppState() *AppState {
	return &AppState{
		Users:    make(map[string]User),
		Sessions: make(map[string]Session),
		Metrics: MetricsData{
			ActiveUsers:   42,
			RequestCount:  1000,
			ResponseTime:  15.5,
			ErrorRate:     0.02,
		},
		LastUpdate: time.Now(),
	}
}

func createLargeTestState(userCount int) map[string]interface{} {
	users := make(map[string]interface{})
	for i := 0; i < userCount; i++ {
		userID := fmt.Sprintf("user-%d", i)
		users[userID] = map[string]interface{}{
			"id":       userID,
			"username": fmt.Sprintf("user%d", i),
			"email":    fmt.Sprintf("user%d@example.com", i),
			"settings": map[string]interface{}{
				"theme":         "dark",
				"language":      "en",
				"notifications": true,
				"preferences":   make(map[string]interface{}),
			},
		}
	}
	
	return map[string]interface{}{
		"users":      users,
		"sessions":   make(map[string]interface{}),
		"metrics":    make(map[string]interface{}),
		"lastUpdate": time.Now(),
	}
}

func saveAppState(ctx context.Context, store *state.StateStore, storage state.StorageBackend, appState *AppState) error {
	// Convert to map
	data, err := json.Marshal(appState)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	
	var stateMap map[string]interface{}
	if err := json.Unmarshal(data, &stateMap); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}
	
	// Save to storage backend
	if err := storage.SetState(ctx, "app-state", stateMap); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}
	
	// Also update in-memory store
	for key, value := range stateMap {
		if err := store.Set("/"+key, value); err != nil {
			return fmt.Errorf("failed to update store: %w", err)
		}
	}
	
	return nil
}

func loadAppState(ctx context.Context, storage state.StorageBackend, stateID string) (*AppState, error) {
	// Load from storage
	stateMap, err := storage.GetState(ctx, stateID)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	
	// Convert to AppState
	data, err := json.Marshal(stateMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state: %w", err)
	}
	
	var appState AppState
	if err := json.Unmarshal(data, &appState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}
	
	return &appState, nil
}