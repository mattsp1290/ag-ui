// Package examples demonstrates how to use bounded maps to prevent memory leaks
// in the httpagent2 codebase
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/internal"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
	httppool "github.com/mattsp1290/ag-ui/go-sdk/pkg/http"
)

func main() {
	fmt.Println("=== Bounded Maps Implementation Examples ===")

	// Example 1: Basic bounded map usage
	demonstrateBoundedMapBasics()

	// Example 2: Connection pool with bounded server pools
	demonstrateConnectionPoolWithBoundedMaps()

	// Example 3: Request manager with bounded correlations
	demonstrateRequestManagerWithBoundedCorrelations()

	// Example 4: Config manager with bounded caches
	demonstrateConfigManagerWithBoundedCaches()

	// Example 5: Monitoring and metrics
	demonstrateMetricsAndMonitoring()
}

// demonstrateBoundedMapBasics shows basic bounded map functionality
func demonstrateBoundedMapBasics() {
	fmt.Println("\n--- Basic Bounded Map Usage ---")

	// Create a bounded map with LRU eviction and TTL
	boundedMap := internal.NewBoundedMapOptions().
		WithMaxSize(1000).
		WithTTL(30 * time.Minute).
		WithCleanupInterval(5 * time.Minute).
		WithMetrics(true).
		WithEvictionCallback(func(key, value interface{}, reason internal.EvictionReason) {
			fmt.Printf("Evicted key=%v, reason=%s\n", key, reason)
		}).
		Build()

	defer boundedMap.Close()

	// Add some data
	boundedMap.Set("user:123", map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	})

	// Retrieve data
	if value, exists := boundedMap.Get("user:123"); exists {
		fmt.Printf("Retrieved: %+v\n", value)
	}

	// Show statistics
	stats := boundedMap.GetStats()
	fmt.Printf("Map stats: %+v\n", stats)
}

// demonstrateConnectionPoolWithBoundedMaps shows bounded connection pools
func demonstrateConnectionPoolWithBoundedMaps() {
	fmt.Println("\n--- Connection Pool with Bounded Maps ---")

	// Create HTTP pool config with bounded pool settings
	config := httppool.DefaultHTTPPoolConfig()
	config.BoundedPool = httppool.BoundedPoolConfig{
		MaxServerPools:      500,              // Maximum number of server pools
		ServerPoolTTL:       30 * time.Minute, // TTL for unused pools
		PoolCleanupInterval: 5 * time.Minute,  // Cleanup interval
		EnablePoolMetrics:   true,
	}

	// Create connection pool
	pool, err := httppool.NewHTTPConnectionPool(config)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Shutdown(context.Background())

	// Add some servers
	servers := []string{
		"http://api1.example.com",
		"http://api2.example.com",
		"http://api3.example.com",
	}

	for _, server := range servers {
		if err := pool.AddServer(server, 1); err != nil {
			log.Printf("Failed to add server %s: %v", server, err)
		}
	}

	fmt.Printf("Connection pool created with %d servers\n", len(servers))

	// Get connection pool metrics
	metrics := pool.GetMetrics()
	fmt.Printf("Pool metrics: TotalConnections=%d, ActiveConnections=%d\n",
		metrics.TotalConnections, metrics.ActiveConnections)
}

// demonstrateRequestManagerWithBoundedCorrelations shows bounded request correlations
func demonstrateRequestManagerWithBoundedCorrelations() {
	fmt.Println("\n--- Request Manager with Standard Correlations ---")

	// Create request manager config (without bounded correlations for now)
	config := client.RequestManagerConfig{
		Timeout:       30 * time.Second,
		MaxIdleConns:  100,
		EnableMetrics: true,
		// Note: BoundedCorrelation feature is planned for future implementation
	}

	// Create request manager
	rm, err := client.NewRequestManager(config)
	if err != nil {
		log.Fatalf("Failed to create request manager: %v", err)
	}
	defer rm.Close()

	fmt.Println("Request manager created with standard correlations")

	// Get correlation map stats (using existing API)
	stats := rm.GetCorrelationMapStats()
	fmt.Printf("Correlation map stats: %+v\n", stats)
}

// demonstrateConfigManagerWithBoundedCaches shows bounded configuration caches
func demonstrateConfigManagerWithBoundedCaches() {
	fmt.Println("\n--- Config Manager Example (Bounded Implementation Planned) ---")

	// Note: This demonstrates the planned API for bounded configuration caches
	// The actual implementation is not yet available in the client package

	fmt.Println("Bounded configuration cache management is planned for future implementation.")
	fmt.Println("It will include:")
	fmt.Println("  - Maximum cached configurations limit")
	fmt.Println("  - TTL-based cache expiration")
	fmt.Println("  - Bounded watchers and listeners")
	fmt.Println("  - Automatic cleanup and metrics")

	// For now, show what the configuration would look like:
	fmt.Println("\nPlanned configuration structure:")
	fmt.Println("  MaxCachedConfigs: 1000")
	fmt.Println("  ConfigCacheTTL: 1 hour")
	fmt.Println("  MaxWatchers: 500")
	fmt.Println("  WatcherTTL: 2 hours")
	fmt.Println("  CleanupInterval: 15 minutes")
}

// demonstrateMetricsAndMonitoring shows how to monitor bounded maps
func demonstrateMetricsAndMonitoring() {
	fmt.Println("\n--- Metrics and Monitoring ---")

	// Create a bounded map with metrics enabled
	boundedMap := internal.NewBoundedMapOptions().
		WithMaxSize(100).
		WithTTL(1 * time.Minute).
		WithMetrics(true).
		WithMetricsPrefix("demo_map").
		Build()

	defer boundedMap.Close()

	// Simulate some activity
	for i := 0; i < 150; i++ { // Exceed max size to trigger evictions
		boundedMap.Set(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
	}

	// Wait for some operations
	time.Sleep(100 * time.Millisecond)

	// Get statistics
	stats := boundedMap.GetStats()
	fmt.Printf("Final map stats:\n")
	for key, value := range stats {
		fmt.Printf("  %s: %v\n", key, value)
	}

	// Show current size vs max size
	fmt.Printf("Current size: %d, Max size: %d\n", boundedMap.Len(), stats["max_size"])

	// Demonstrate key listing
	keys := boundedMap.Keys()
	fmt.Printf("Number of keys: %d\n", len(keys))
	if len(keys) > 0 {
		fmt.Printf("First few keys: %v\n", keys[:min(5, len(keys))])
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
