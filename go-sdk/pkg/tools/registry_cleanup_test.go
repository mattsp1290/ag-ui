package tools

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistryToolLRUEviction tests LRU eviction when MaxTools is reached
func TestRegistryToolLRUEviction(t *testing.T) {
	config := &RegistryConfig{
		MaxTools:      5,  // Small limit to test eviction
		EnableToolLRU: true,
		ToolTTL:       0, // Disable TTL for this test
		EnableBackgroundToolCleanup: false,
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	// Register more tools than the limit
	for i := 0; i < 10; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("tool-%d", i),
			Name:        fmt.Sprintf("Tool %d", i),
			Description: fmt.Sprintf("Test tool %d", i),
			Version:     "1.0.0",
			Executor:    &testExecutor{},
		}
		err := registry.Register(tool)
		require.NoError(t, err)
	}

	// Verify only MaxTools are kept
	assert.Equal(t, config.MaxTools, registry.Count(), "Registry should not exceed MaxTools")

	// Verify oldest tools were evicted (LRU)
	for i := 0; i < 5; i++ {
		_, err := registry.Get(fmt.Sprintf("tool-%d", i))
		assert.Error(t, err, "Oldest tools should have been evicted")
	}

	// Verify newest tools are still present
	for i := 5; i < 10; i++ {
		tool, err := registry.Get(fmt.Sprintf("tool-%d", i))
		assert.NoError(t, err, "Newest tools should still be present")
		assert.NotNil(t, tool)
	}
}

// TestRegistryToolTTLCleanup tests TTL-based cleanup of expired tools
func TestRegistryToolTTLCleanup(t *testing.T) {
	config := &RegistryConfig{
		MaxTools:                     100,
		EnableToolLRU:                false,
		ToolTTL:                      100 * time.Millisecond, // Very short TTL
		ToolCleanupInterval:          50 * time.Millisecond,  // Frequent cleanup
		EnableBackgroundToolCleanup: false, // Manual cleanup for testing
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	// Register some tools
	for i := 0; i < 5; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("tool-%d", i),
			Name:        fmt.Sprintf("Tool %d", i),
			Description: fmt.Sprintf("Test tool %d", i),
			Version:     "1.0.0",
			Executor:    &testExecutor{},
		}
		err := registry.Register(tool)
		require.NoError(t, err)
	}

	// Verify all tools are present initially
	assert.Equal(t, 5, registry.Count())

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Run manual cleanup
	cleaned, err := registry.CleanupExpiredTools()
	require.NoError(t, err)
	assert.Equal(t, 5, cleaned, "All tools should be cleaned up due to TTL expiry")

	// Verify tools are gone
	assert.Equal(t, 0, registry.Count())
	for i := 0; i < 5; i++ {
		_, err := registry.Get(fmt.Sprintf("tool-%d", i))
		assert.Error(t, err, "Tools should be removed after TTL cleanup")
	}
}

// TestRegistryAccessTimeCleanup tests cleanup based on last access time
func TestRegistryAccessTimeCleanup(t *testing.T) {
	config := &RegistryConfig{
		MaxTools:                     100,
		EnableToolLRU:                false,
		ToolTTL:                      0, // Disable TTL
		EnableBackgroundToolCleanup: false,
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	// Register some tools
	for i := 0; i < 5; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("tool-%d", i),
			Name:        fmt.Sprintf("Tool %d", i),
			Description: fmt.Sprintf("Test tool %d", i),
			Version:     "1.0.0",
			Executor:    &testExecutor{},
		}
		err := registry.Register(tool)
		require.NoError(t, err)
	}

	// Access some tools to update their access time
	time.Sleep(50 * time.Millisecond)
	_, _ = registry.Get("tool-2")
	_, _ = registry.Get("tool-3")

	// Wait a bit more
	time.Sleep(50 * time.Millisecond)

	// Clean up tools not accessed in the last 75ms
	cleaned, err := registry.CleanupByAccessTime(75 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 3, cleaned, "Tools 0, 1, and 4 should be cleaned up")

	// Verify correct tools remain
	assert.Equal(t, 2, registry.Count())
	
	// Tools 2 and 3 should still be present (they were accessed recently)
	_, err = registry.Get("tool-2")
	assert.NoError(t, err)
	_, err = registry.Get("tool-3")
	assert.NoError(t, err)

	// Tools 0, 1, and 4 should be gone
	_, err = registry.Get("tool-0")
	assert.Error(t, err)
	_, err = registry.Get("tool-1")
	assert.Error(t, err)
	_, err = registry.Get("tool-4")
	assert.Error(t, err)
}

// TestRegistryBackgroundCleanup tests background TTL cleanup
func TestRegistryBackgroundCleanup(t *testing.T) {
	config := &RegistryConfig{
		MaxTools:                     100,
		EnableToolLRU:                false,
		ToolTTL:                      100 * time.Millisecond, // Short TTL
		ToolCleanupInterval:          50 * time.Millisecond,  // Frequent cleanup
		EnableBackgroundToolCleanup: true, // Enable background cleanup
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	// Register some tools
	for i := 0; i < 3; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("tool-%d", i),
			Name:        fmt.Sprintf("Tool %d", i),
			Description: fmt.Sprintf("Test tool %d", i),
			Version:     "1.0.0",
			Executor:    &testExecutor{},
		}
		err := registry.Register(tool)
		require.NoError(t, err)
	}

	// Verify tools are present initially
	assert.Equal(t, 3, registry.Count())

	// Wait for background cleanup to remove expired tools
	// Wait longer than TTL + cleanup interval
	time.Sleep(200 * time.Millisecond)

	// Verify tools have been cleaned up by background process
	assert.Equal(t, 0, registry.Count(), "Background cleanup should have removed all expired tools")
}

// TestRegistryLRUAccessPattern tests that LRU correctly tracks access patterns
func TestRegistryLRUAccessPattern(t *testing.T) {
	config := &RegistryConfig{
		MaxTools:      3,  // Small limit
		EnableToolLRU: true,
		ToolTTL:       0, // Disable TTL
		EnableBackgroundToolCleanup: false,
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	// Register 3 tools (at limit)
	tools := []string{"tool-1", "tool-2", "tool-3"}
	for _, toolID := range tools {
		tool := &Tool{
			ID:       toolID,
			Name:     toolID,
			Description: "Test tool",
			Version:  "1.0.0",
			Executor: &testExecutor{},
		}
		err := registry.Register(tool)
		require.NoError(t, err)
	}

	// Access tool-1 to make it most recently used
	_, err := registry.Get("tool-1")
	require.NoError(t, err)

	// Register a new tool, should evict tool-2 (least recently used)
	newTool := &Tool{
		ID:       "tool-4",
		Name:     "tool-4",
		Description: "New test tool",
		Version:  "1.0.0",
		Executor: &testExecutor{},
	}
	err = registry.Register(newTool)
	require.NoError(t, err)

	// Verify tool-1, tool-3, and tool-4 are present
	_, err = registry.Get("tool-1")
	assert.NoError(t, err, "tool-1 should still be present (recently accessed)")
	_, err = registry.Get("tool-3")
	assert.NoError(t, err, "tool-3 should still be present")
	_, err = registry.Get("tool-4")
	assert.NoError(t, err, "tool-4 should be present (newly added)")

	// Verify tool-2 was evicted
	_, err = registry.Get("tool-2")
	assert.Error(t, err, "tool-2 should have been evicted (least recently used)")
}

// TestRegistryConcurrentCleanup tests cleanup mechanisms under concurrent access
func TestRegistryConcurrentCleanup(t *testing.T) {
	config := &RegistryConfig{
		MaxTools:                     50,
		EnableToolLRU:                true,
		ToolTTL:                      200 * time.Millisecond,
		ToolCleanupInterval:          100 * time.Millisecond,
		EnableBackgroundToolCleanup: true,
		MaxConcurrentRegistrations:  10,
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	var wg sync.WaitGroup
	numGoroutines := 10
	toolsPerGoroutine := 20

	// Concurrent registration and access
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			// Register tools
			for j := 0; j < toolsPerGoroutine; j++ {
				tool := &Tool{
					ID:          fmt.Sprintf("tool-%d-%d", goroutineID, j),
					Name:        fmt.Sprintf("Tool %d-%d", goroutineID, j),
					Description: "Concurrent test tool",
					Version:     "1.0.0",
					Executor:    &testExecutor{},
				}
				_ = registry.Register(tool) // Ignore errors due to eviction/concurrency
			}
			
			// Random access to tools
			for k := 0; k < 10; k++ {
				toolID := fmt.Sprintf("tool-%d-%d", goroutineID, k%toolsPerGoroutine)
				_, _ = registry.Get(toolID) // Ignore errors
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify registry is still in a consistent state
	assert.LessOrEqual(t, registry.Count(), config.MaxTools, "Registry should not exceed MaxTools")

	// Wait for background cleanup to process
	time.Sleep(300 * time.Millisecond)

	// Verify cleanup stats are reasonable
	stats := registry.GetToolsCleanupStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "total_tools")
	assert.Contains(t, stats, "lru_enabled")
	assert.True(t, stats["lru_enabled"].(bool))
}

// TestRegistryClearAllTools tests clearing all tools
func TestRegistryClearAllTools(t *testing.T) {
	registry := NewRegistry()
	defer registry.CloseToolsCleanup()

	// Register some tools
	for i := 0; i < 5; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("tool-%d", i),
			Name:        fmt.Sprintf("Tool %d", i),
			Description: "Test tool",
			Version:     "1.0.0",
			Executor:    &testExecutor{},
		}
		err := registry.Register(tool)
		require.NoError(t, err)
	}

	// Verify tools are present
	assert.Equal(t, 5, registry.Count())

	// Clear all tools
	err := registry.ClearAllTools()
	require.NoError(t, err)

	// Verify all tools are gone
	assert.Equal(t, 0, registry.Count())

	// Verify specific tools are gone
	for i := 0; i < 5; i++ {
		_, err := registry.Get(fmt.Sprintf("tool-%d", i))
		assert.Error(t, err)
	}

	// Verify memory usage is reset
	usage := registry.GetResourceUsage()
	assert.Equal(t, int64(0), usage["memory_usage"].(int64))
}

// TestRegistryCleanupStats tests cleanup statistics
func TestRegistryCleanupStats(t *testing.T) {
	config := &RegistryConfig{
		MaxTools:      10,
		EnableToolLRU: true,
		ToolTTL:       1 * time.Hour, // Long TTL
		EnableBackgroundToolCleanup: false,
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	// Register some tools
	for i := 0; i < 5; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("tool-%d", i),
			Name:        fmt.Sprintf("Tool %d", i),
			Description: "Test tool",
			Version:     "1.0.0",
			Executor:    &testExecutor{},
		}
		err := registry.Register(tool)
		require.NoError(t, err)
	}

	// Access some tools to vary access counts
	_, _ = registry.Get("tool-0")
	_, _ = registry.Get("tool-0") // tool-0 accessed twice
	_, _ = registry.Get("tool-1") // tool-1 accessed once

	// Get cleanup stats
	stats := registry.GetToolsCleanupStats()
	require.NotNil(t, stats)

	// Verify expected stats
	assert.Equal(t, 5, stats["total_tools"])
	assert.Equal(t, 5, stats["lru_list_length"])
	assert.Equal(t, 5, stats["lru_index_length"])
	assert.Equal(t, 3600.0, stats["ttl_seconds"]) // 1 hour in seconds
	assert.True(t, stats["lru_enabled"].(bool))
	assert.False(t, stats["cleanup_enabled"].(bool))

	// Verify access count statistics
	assert.Contains(t, stats, "total_access_count")
	assert.Contains(t, stats, "average_access_count")
	assert.Equal(t, int64(8), stats["total_access_count"]) // 5 initial + 2 for tool-0 + 1 for tool-1
	assert.Equal(t, 1.6, stats["average_access_count"])   // 8/5 = 1.6

	// Verify timestamp fields are present
	assert.Contains(t, stats, "oldest_created")
	assert.Contains(t, stats, "newest_created")
	assert.Contains(t, stats, "oldest_access")
	assert.Contains(t, stats, "newest_access")
}

// TestRegistryConfigUpdate tests updating cleanup configuration
func TestRegistryConfigUpdate(t *testing.T) {
	initialConfig := &RegistryConfig{
		MaxTools:                     10,
		EnableToolLRU:                true,
		ToolTTL:                      1 * time.Hour,
		ToolCleanupInterval:          30 * time.Minute,
		EnableBackgroundToolCleanup: false,
	}

	registry := NewRegistryWithConfig(initialConfig)
	defer registry.CloseToolsCleanup()

	// Register a tool
	tool := &Tool{
		ID:       "test-tool",
		Name:     "Test Tool",
		Description: "Test tool",
		Version:  "1.0.0",
		Executor: &testExecutor{},
	}
	err := registry.Register(tool)
	require.NoError(t, err)

	// Update configuration
	newConfig := &RegistryConfig{
		MaxTools:                     5,   // Smaller limit
		EnableToolLRU:                true,
		ToolTTL:                      10 * time.Minute, // Shorter TTL
		ToolCleanupInterval:          5 * time.Minute,  // More frequent cleanup
		EnableBackgroundToolCleanup: true,             // Enable background cleanup
	}

	err = registry.UpdateToolsConfig(newConfig)
	require.NoError(t, err)

	// Verify configuration was updated
	stats := registry.GetToolsCleanupStats()
	assert.Equal(t, 600.0, stats["ttl_seconds"])   // 10 minutes
	assert.Equal(t, 300.0, stats["cleanup_interval"]) // 5 minutes
	assert.True(t, stats["cleanup_enabled"].(bool))

	// Tool should still be present
	retrievedTool, err := registry.Get("test-tool")
	require.NoError(t, err)
	assert.Equal(t, "test-tool", retrievedTool.ID)
}

// TestRegistryMemoryTracking tests memory usage tracking with cleanup
func TestRegistryMemoryTracking(t *testing.T) {
	config := &RegistryConfig{
		MaxTools:                     5,
		EnableToolLRU:                true,
		ToolTTL:                      0, // Disable TTL
		EnableBackgroundToolCleanup: false,
		MaxMemoryUsage:               10 * 1024, // 10KB limit
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	// Register tools and track memory usage
	initialUsage := registry.GetResourceUsage()
	initialMemory := initialUsage["memory_usage"].(int64)

	// Register some tools
	for i := 0; i < 3; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("tool-%d", i),
			Name:        fmt.Sprintf("Tool %d", i),
			Description: fmt.Sprintf("Test tool %d with some longer description to use more memory", i),
			Version:     "1.0.0",
			Executor:    &testExecutor{},
			Metadata: &ToolMetadata{
				Author:        "Test Author",
				Documentation: "Detailed description for memory testing",
				Tags:          []string{"test", "memory", "tracking"},
			},
		}
		err := registry.Register(tool)
		require.NoError(t, err)
	}

	// Verify memory usage increased
	afterRegistration := registry.GetResourceUsage()
	afterMemory := afterRegistration["memory_usage"].(int64)
	assert.Greater(t, afterMemory, initialMemory, "Memory usage should increase after registering tools")

	// Register more tools to trigger LRU eviction
	for i := 3; i < 8; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("tool-%d", i),
			Name:        fmt.Sprintf("Tool %d", i),
			Description: "Test tool",
			Version:     "1.0.0",
			Executor:    &testExecutor{},
		}
		err := registry.Register(tool)
		require.NoError(t, err)
	}

	// Verify tool count is bounded
	assert.Equal(t, config.MaxTools, registry.Count())

	// Clear all tools and verify memory is reset
	err := registry.ClearAllTools()
	require.NoError(t, err)

	finalUsage := registry.GetResourceUsage()
	finalMemory := finalUsage["memory_usage"].(int64)
	assert.Equal(t, int64(0), finalMemory, "Memory usage should be reset after clearing all tools")
}

