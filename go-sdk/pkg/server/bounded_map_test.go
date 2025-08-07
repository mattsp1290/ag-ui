package server

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestBoundedMap_Basic(t *testing.T) {
	config := BoundedMapConfig{
		MaxSize:        5,
		EnableTimeouts: false,
	}

	bm := NewBoundedMap[string, int](config)

	// Test basic set/get operations
	bm.Set("key1", 1)
	bm.Set("key2", 2)
	bm.Set("key3", 3)

	if val, ok := bm.Get("key1"); !ok || val != 1 {
		t.Errorf("Expected key1=1, got %v, %v", val, ok)
	}

	if val, ok := bm.Get("key2"); !ok || val != 2 {
		t.Errorf("Expected key2=2, got %v, %v", val, ok)
	}

	if val, ok := bm.Get("key3"); !ok || val != 3 {
		t.Errorf("Expected key3=3, got %v, %v", val, ok)
	}

	// Test non-existent key
	if _, ok := bm.Get("nonexistent"); ok {
		t.Error("Expected nonexistent key to return false")
	}

	// Test length
	if bm.Len() != 3 {
		t.Errorf("Expected length 3, got %d", bm.Len())
	}
}

func TestBoundedMap_LRUEviction(t *testing.T) {
	config := BoundedMapConfig{
		MaxSize:        3,
		EnableTimeouts: false,
	}

	bm := NewBoundedMap[string, int](config)

	// Fill the map
	bm.Set("key1", 1)
	bm.Set("key2", 2)
	bm.Set("key3", 3)

	// Access key1 to make it most recently used
	bm.Get("key1")

	// Add a new key, which should evict key2 (least recently used)
	bm.Set("key4", 4)

	// key1 and key3 should still exist, key2 should be evicted
	if _, ok := bm.Get("key1"); !ok {
		t.Error("key1 should still exist")
	}
	if _, ok := bm.Get("key3"); !ok {
		t.Error("key3 should still exist")
	}
	if _, ok := bm.Get("key4"); !ok {
		t.Error("key4 should exist")
	}
	if _, ok := bm.Get("key2"); ok {
		t.Error("key2 should have been evicted")
	}

	// Verify stats
	stats := bm.Stats()
	if stats.Evictions != 1 {
		t.Errorf("Expected 1 eviction, got %d", stats.Evictions)
	}
}

func TestBoundedMap_TTLExpiration(t *testing.T) {
	config := BoundedMapConfig{
		MaxSize:        10,
		EnableTimeouts: true,
		TTL:            100 * time.Millisecond,
	}

	bm := NewBoundedMap[string, int](config)

	// Set a key
	bm.Set("key1", 1)

	// Should be retrievable immediately
	if val, ok := bm.Get("key1"); !ok || val != 1 {
		t.Errorf("Expected key1=1, got %v, %v", val, ok)
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	if _, ok := bm.Get("key1"); ok {
		t.Error("key1 should have expired")
	}

	// Verify stats show timeout
	stats := bm.Stats()
	if stats.Timeouts != 1 {
		t.Errorf("Expected 1 timeout, got %d", stats.Timeouts)
	}
}

func TestBoundedMap_GetOrSet(t *testing.T) {
	config := BoundedMapConfig{
		MaxSize:        5,
		EnableTimeouts: false,
	}

	bm := NewBoundedMap[string, int](config)

	// First call should set the value
	val := bm.GetOrSet("key1", func() int { return 42 })
	if val != 42 {
		t.Errorf("Expected 42, got %d", val)
	}

	// Second call should return existing value
	val = bm.GetOrSet("key1", func() int { return 100 })
	if val != 42 {
		t.Errorf("Expected 42 (existing value), got %d", val)
	}

	if bm.Len() != 1 {
		t.Errorf("Expected length 1, got %d", bm.Len())
	}
}

func TestBoundedMap_MemoryExhaustionProtection(t *testing.T) {
	config := BoundedMapConfig{
		MaxSize:        1000,
		EnableTimeouts: false,
	}

	bm := NewBoundedMap[string, int](config)

	// Try to add many more entries than the limit
	for i := 0; i < 2000; i++ {
		bm.Set(fmt.Sprintf("key%d", i), i)
	}

	// Should never exceed max size
	if bm.Len() > config.MaxSize {
		t.Errorf("Map size %d exceeded maximum %d", bm.Len(), config.MaxSize)
	}

	// Should have evictions
	stats := bm.Stats()
	if stats.Evictions == 0 {
		t.Error("Expected evictions to occur")
	}

	// Most recently added keys should still exist
	if _, ok := bm.Get("key1999"); !ok {
		t.Error("Most recent key should still exist")
	}

	// Oldest keys should be evicted
	if _, ok := bm.Get("key0"); ok {
		t.Error("Oldest key should have been evicted")
	}
}

func TestBoundedMap_ConcurrentAccess(t *testing.T) {
	config := BoundedMapConfig{
		MaxSize:        1000,
		EnableTimeouts: false,
	}

	bm := NewBoundedMap[string, int](config)

	var wg sync.WaitGroup
	numGoroutines := 10
	itemsPerGoroutine := 100

	// Concurrent writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				bm.Set(key, id*itemsPerGoroutine+j)
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				bm.Get(key) // Don't care about the result, just testing safety
			}
		}(i)
	}

	wg.Wait()

	// Should not exceed max size even under concurrent access
	if bm.Len() > config.MaxSize {
		t.Errorf("Map size %d exceeded maximum %d", bm.Len(), config.MaxSize)
	}
}

func TestBoundedMap_Cleanup(t *testing.T) {
	config := BoundedMapConfig{
		MaxSize:        10,
		EnableTimeouts: true,
		TTL:            50 * time.Millisecond,
	}

	bm := NewBoundedMap[string, int](config)

	// Add some entries
	for i := 0; i < 5; i++ {
		bm.Set(fmt.Sprintf("key%d", i), i)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Add one more entry to trigger different timestamps
	bm.Set("new_key", 999)

	// Test that expired entries are automatically removed on access
	expiredKeys := 0
	for i := 0; i < 5; i++ {
		if _, ok := bm.Get(fmt.Sprintf("key%d", i)); !ok {
			expiredKeys++
		}
	}

	// Should have some expired entries
	if expiredKeys == 0 {
		t.Error("Expected some keys to be expired")
	}

	// New entry should still exist
	if _, ok := bm.Get("new_key"); !ok {
		t.Error("New entry should still exist after cleanup")
	}
}

func TestBoundedMap_Stats(t *testing.T) {
	config := BoundedMapConfig{
		MaxSize:        3,
		EnableTimeouts: false,
	}

	bm := NewBoundedMap[string, int](config)

	// Add entries to trigger eviction
	bm.Set("key1", 1)
	bm.Set("key2", 2)
	bm.Set("key3", 3)
	bm.Set("key4", 4) // Should evict key1

	// Test hits and misses
	bm.Get("key2")        // Hit
	bm.Get("key3")        // Hit
	bm.Get("key1")        // Miss (evicted)
	bm.Get("nonexistent") // Miss

	stats := bm.Stats()

	if stats.Size != 3 {
		t.Errorf("Expected size 3, got %d", stats.Size)
	}
	if stats.MaxSize != 3 {
		t.Errorf("Expected max size 3, got %d", stats.MaxSize)
	}
	if stats.Hits != 2 {
		t.Errorf("Expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 2 {
		t.Errorf("Expected 2 misses, got %d", stats.Misses)
	}
	if stats.Evictions != 1 {
		t.Errorf("Expected 1 eviction, got %d", stats.Evictions)
	}
	if stats.HitRate == 0 {
		t.Error("Hit rate should be greater than 0")
	}
}

func TestBoundedMap_Clear(t *testing.T) {
	config := BoundedMapConfig{
		MaxSize:        10,
		EnableTimeouts: false,
	}

	bm := NewBoundedMap[string, int](config)

	// Add some entries
	for i := 0; i < 5; i++ {
		bm.Set(strconv.Itoa(i), i)
	}

	if bm.Len() != 5 {
		t.Errorf("Expected length 5, got %d", bm.Len())
	}

	// Clear the map
	bm.Clear()

	if bm.Len() != 0 {
		t.Errorf("Expected length 0 after clear, got %d", bm.Len())
	}

	// All keys should be gone
	for i := 0; i < 5; i++ {
		if _, ok := bm.Get(strconv.Itoa(i)); ok {
			t.Errorf("Key %d should not exist after clear", i)
		}
	}
}

func TestBoundedMap_Delete(t *testing.T) {
	config := BoundedMapConfig{
		MaxSize:        10,
		EnableTimeouts: false,
	}

	bm := NewBoundedMap[string, int](config)

	// Add some entries
	bm.Set("key1", 1)
	bm.Set("key2", 2)

	// Delete existing key
	if !bm.Delete("key1") {
		t.Error("Delete should return true for existing key")
	}

	// Verify it's gone
	if _, ok := bm.Get("key1"); ok {
		t.Error("key1 should not exist after delete")
	}

	// Delete non-existent key
	if bm.Delete("nonexistent") {
		t.Error("Delete should return false for non-existent key")
	}

	// key2 should still exist
	if _, ok := bm.Get("key2"); !ok {
		t.Error("key2 should still exist")
	}
}

// Benchmark tests to verify performance characteristics
func BenchmarkBoundedMap_Set(b *testing.B) {
	config := BoundedMapConfig{
		MaxSize:        10000,
		EnableTimeouts: false,
	}

	bm := NewBoundedMap[string, int](config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bm.Set(fmt.Sprintf("key%d", i), i)
	}
}

func BenchmarkBoundedMap_Get(b *testing.B) {
	config := BoundedMapConfig{
		MaxSize:        10000,
		EnableTimeouts: false,
	}

	bm := NewBoundedMap[string, int](config)

	// Pre-populate
	for i := 0; i < 1000; i++ {
		bm.Set(fmt.Sprintf("key%d", i), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bm.Get(fmt.Sprintf("key%d", i%1000))
	}
}

func BenchmarkBoundedMap_GetOrSet(b *testing.B) {
	config := BoundedMapConfig{
		MaxSize:        10000,
		EnableTimeouts: false,
	}

	bm := NewBoundedMap[string, int](config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%1000) // Reuse some keys
		bm.GetOrSet(key, func() int { return i })
	}
}
