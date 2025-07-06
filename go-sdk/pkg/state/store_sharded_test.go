package state

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestShardedStateStore_ConcurrentAccess tests concurrent access to different shards
func TestShardedStateStore_ConcurrentAccess(t *testing.T) {
	store := NewStateStore(WithShardCount(16))
	
	// Number of concurrent operations
	numGoroutines := 50
	numOperations := 100
	
	// Track successful operations
	var successCount int64
	var wg sync.WaitGroup
	
	// Launch concurrent goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Generate paths that will hash to different shards
				path := fmt.Sprintf("/data/routine%d/item%d", routineID, j)
				value := map[string]interface{}{
					"routine": routineID,
					"item":    j,
					"data":    fmt.Sprintf("value-%d-%d", routineID, j),
				}
				
				// Set value
				if err := store.Set(path, value); err != nil {
					// Don't use t.Errorf in goroutines, it's not safe
					continue
				}
				
				// Get value
				retrieved, err := store.Get(path)
				if err != nil {
					// Don't use t.Errorf in goroutines, it's not safe
					continue
				}
				
				// Verify value
				if retrievedMap, ok := retrieved.(map[string]interface{}); ok {
					if retrievedMap["routine"] == routineID && retrievedMap["item"] == j {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedTotal := int64(numGoroutines * numOperations)
	if successCount != expectedTotal {
		t.Errorf("Expected %d successful operations, got %d", expectedTotal, successCount)
	}
}

// TestShardedStateStore_ShardDistribution verifies even distribution across shards
func TestShardedStateStore_ShardDistribution(t *testing.T) {
	store := NewStateStore(WithShardCount(16))
	
	// Track which shard each path maps to
	shardCounts := make(map[uint32]int)
	numPaths := 10000
	
	for i := 0; i < numPaths; i++ {
		path := fmt.Sprintf("/test/path/%d", i)
		shardIdx := store.getShardIndex(path)
		shardCounts[shardIdx]++
	}
	
	// Verify all shards are used
	if len(shardCounts) != int(store.shardCount) {
		t.Errorf("Expected %d shards to be used, but only %d were used", store.shardCount, len(shardCounts))
	}
	
	// Check distribution (should be roughly even)
	expectedPerShard := numPaths / int(store.shardCount)
	tolerance := float64(expectedPerShard) * 0.2 // 20% tolerance
	
	for shard, count := range shardCounts {
		if float64(count) < float64(expectedPerShard)-tolerance || 
		   float64(count) > float64(expectedPerShard)+tolerance {
			t.Errorf("Shard %d has %d items, expected around %d (±%.0f)", 
				shard, count, expectedPerShard, tolerance)
		}
	}
}

// TestShardedStateStore_RootPathOperations tests operations on root path
func TestShardedStateStore_RootPathOperations(t *testing.T) {
	store := NewStateStore(WithShardCount(16))
	
	// Set multiple values across different shards
	testData := map[string]interface{}{
		"users":    map[string]interface{}{"count": 100},
		"products": map[string]interface{}{"count": 200},
		"orders":   map[string]interface{}{"count": 300},
		"config":   map[string]interface{}{"version": "1.0"},
	}
	
	// Set each key individually (will go to different shards)
	for key, value := range testData {
		if err := store.Set("/"+key, value); err != nil {
			t.Fatalf("Failed to set /%s: %v", key, err)
		}
	}
	
	// Get root path should return all data
	rootData, err := store.Get("/")
	if err != nil {
		t.Fatalf("Failed to get root path: %v", err)
	}
	
	rootMap, ok := rootData.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map for root path, got %T", rootData)
	}
	
	// Verify all data is present
	for key, expectedValue := range testData {
		actualValue, exists := rootMap[key]
		if !exists {
			t.Errorf("Key %s not found in root data", key)
			continue
		}
		
		// Compare as JSON for deep equality
		expectedJSON, _ := json.Marshal(expectedValue)
		actualJSON, _ := json.Marshal(actualValue)
		if string(expectedJSON) != string(actualJSON) {
			t.Errorf("Value mismatch for key %s: expected %s, got %s", 
				key, expectedJSON, actualJSON)
		}
	}
}

// TestShardedStateStore_TransactionAcrossShards tests transactions spanning multiple shards
func TestShardedStateStore_TransactionAcrossShards(t *testing.T) {
	store := NewStateStore(WithShardCount(16))
	
	// Start transaction
	tx := store.Begin()
	
	// Apply patches that will affect different shards
	patches := JSONPatch{
		{Op: JSONPatchOpAdd, Path: "/user1", Value: map[string]interface{}{"name": "Alice"}},
		{Op: JSONPatchOpAdd, Path: "/user2", Value: map[string]interface{}{"name": "Bob"}},
		{Op: JSONPatchOpAdd, Path: "/user3", Value: map[string]interface{}{"name": "Charlie"}},
		{Op: JSONPatchOpAdd, Path: "/config/setting1", Value: "value1"},
		{Op: JSONPatchOpAdd, Path: "/data/item1", Value: "data1"},
	}
	
	if err := tx.Apply(patches); err != nil {
		t.Fatalf("Failed to apply patches to transaction: %v", err)
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}
	
	// Verify all changes were applied
	verifyPaths := map[string]interface{}{
		"/user1":          map[string]interface{}{"name": "Alice"},
		"/user2":          map[string]interface{}{"name": "Bob"},
		"/user3":          map[string]interface{}{"name": "Charlie"},
		"/config/setting1": "value1",
		"/data/item1":      "data1",
	}
	
	for path, expectedValue := range verifyPaths {
		actualValue, err := store.Get(path)
		if err != nil {
			t.Errorf("Failed to get %s after transaction: %v", path, err)
			continue
		}
		
		// Compare as JSON for deep equality
		expectedJSON, _ := json.Marshal(expectedValue)
		actualJSON, _ := json.Marshal(actualValue)
		if string(expectedJSON) != string(actualJSON) {
			t.Errorf("Value mismatch for path %s: expected %s, got %s",
				path, expectedJSON, actualJSON)
		}
	}
}

// BenchmarkShardedStateStore_ConcurrentWrites benchmarks concurrent write performance
func BenchmarkShardedStateStore_ConcurrentWrites(b *testing.B) {
	configs := []struct {
		name       string
		shardCount uint32
	}{
		{"1shard", 1},
		{"4shards", 4},
		{"8shards", 8},
		{"16shards", 16},
		{"32shards", 32},
	}
	
	for _, config := range configs {
		b.Run(config.name, func(b *testing.B) {
			store := NewStateStore(WithShardCount(config.shardCount))
			
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					path := fmt.Sprintf("/bench/item%d", i)
					value := map[string]interface{}{
						"id":   i,
						"data": fmt.Sprintf("value-%d", i),
					}
					if err := store.Set(path, value); err != nil {
						b.Errorf("Set failed: %v", err)
					}
					i++
				}
			})
		})
	}
}

// BenchmarkShardedStateStore_ConcurrentReads benchmarks concurrent read performance
func BenchmarkShardedStateStore_ConcurrentReads(b *testing.B) {
	store := NewStateStore(WithShardCount(16))
	
	// Pre-populate with data
	numItems := 10000
	for i := 0; i < numItems; i++ {
		path := fmt.Sprintf("/bench/item%d", i)
		value := map[string]interface{}{
			"id":   i,
			"data": fmt.Sprintf("value-%d", i),
		}
		store.Set(path, value)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			path := fmt.Sprintf("/bench/item%d", i%numItems)
			if _, err := store.Get(path); err != nil {
				b.Errorf("Get failed: %v", err)
			}
			i++
		}
	})
}

// TestShardedStateStore_LockContentionReduction measures lock contention improvement
func TestShardedStateStore_LockContentionReduction(t *testing.T) {
	// Skip if short testing
	if testing.Short() {
		t.Skip("Skipping lock contention test in short mode")
	}
	
	// Test with different shard counts
	shardCounts := []uint32{1, 16}
	
	for _, shardCount := range shardCounts {
		t.Run(fmt.Sprintf("%d_shards", shardCount), func(t *testing.T) {
			store := NewStateStore(WithShardCount(shardCount))
			
			numGoroutines := 50
			numOperations := 1000
			start := time.Now()
			
			var wg sync.WaitGroup
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for j := 0; j < numOperations; j++ {
						path := fmt.Sprintf("/worker%d/item%d", id, j)
						store.Set(path, map[string]interface{}{"value": j})
					}
				}(i)
			}
			
			wg.Wait()
			duration := time.Since(start)
			
			opsPerSecond := float64(numGoroutines*numOperations) / duration.Seconds()
			t.Logf("Shard count %d: %v total, %.0f ops/sec", shardCount, duration, opsPerSecond)
		})
	}
}