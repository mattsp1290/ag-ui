package tools_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistryEnhancedFeatures tests the enhanced registry functionality
func TestRegistryEnhancedFeatures(t *testing.T) {
	t.Run("ConflictResolution", func(t *testing.T) {
		testConflictResolution(t)
	})
	
	t.Run("VersionMigration", func(t *testing.T) {
		testVersionMigration(t)
	})
	
	t.Run("CategoryTree", func(t *testing.T) {
		testCategoryTree(t)
	})
	
	t.Run("DynamicLoading", func(t *testing.T) {
		testDynamicLoading(t)
	})
	
	t.Run("HotReloading", func(t *testing.T) {
		testHotReloading(t)
	})
	
	t.Run("DependencyGraph", func(t *testing.T) {
		testDependencyGraph(t)
	})
}

// testConflictResolution tests conflict resolution strategies
func testConflictResolution(t *testing.T) {
	t.Run("ErrorStrategy", func(t *testing.T) {
		config := tools.DefaultRegistryConfig()
		config.ConflictResolutionStrategy = tools.ConflictStrategyError
		registry := tools.NewRegistryWithConfig(config)
		
		tool1 := createTestTool("test1", "TestTool")
		tool2 := createTestTool("test1", "TestTool") // Same ID
		
		require.NoError(t, registry.Register(tool1))
		err := registry.Register(tool2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
	
	t.Run("OverwriteStrategy", func(t *testing.T) {
		config := tools.DefaultRegistryConfig()
		config.ConflictResolutionStrategy = tools.ConflictStrategyOverwrite
		registry := tools.NewRegistryWithConfig(config)
		
		tool1 := createTestTool("test1", "TestTool")
		tool1.Description = "Original"
		tool2 := createTestTool("test1", "TestTool") // Same ID
		tool2.Description = "Overwritten"
		
		require.NoError(t, registry.Register(tool1))
		require.NoError(t, registry.Register(tool2))
		
		retrieved, err := registry.Get("test1")
		require.NoError(t, err)
		assert.Equal(t, "Overwritten", retrieved.Description)
	})
	
	t.Run("VersionBasedStrategy", func(t *testing.T) {
		config := tools.DefaultRegistryConfig()
		config.ConflictResolutionStrategy = tools.ConflictStrategyVersionBased
		registry := tools.NewRegistryWithConfig(config)
		
		tool1 := createTestTool("test1", "TestTool")
		tool1.Version = "1.0.0"
		tool2 := createTestTool("test1", "TestTool") // Same ID
		tool2.Version = "2.0.0"
		tool2.Description = "Newer version"
		
		require.NoError(t, registry.Register(tool1))
		require.NoError(t, registry.Register(tool2))
		
		retrieved, err := registry.Get("test1")
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", retrieved.Version)
		assert.Equal(t, "Newer version", retrieved.Description)
	})
	
	t.Run("CustomResolver", func(t *testing.T) {
		registry := tools.NewRegistry()
		
		// Add custom resolver that merges tools
		registry.AddConflictResolver(func(existing, new *tools.Tool) (*tools.Tool, error) {
			merged := existing.Clone()
			merged.Description = fmt.Sprintf("%s + %s", existing.Description, new.Description)
			return merged, nil
		})
		
		tool1 := createTestTool("test1", "TestTool")
		tool1.Description = "First"
		tool2 := createTestTool("test1", "TestTool") // Same ID
		tool2.Description = "Second"
		
		require.NoError(t, registry.Register(tool1))
		require.NoError(t, registry.Register(tool2))
		
		retrieved, err := registry.Get("test1")
		require.NoError(t, err)
		assert.Equal(t, "First + Second", retrieved.Description)
	})
}

// testVersionMigration tests version migration functionality
func testVersionMigration(t *testing.T) {
	t.Run("BasicMigration", func(t *testing.T) {
		config := tools.DefaultRegistryConfig()
		config.EnableVersionMigration = true
		config.ConflictResolutionStrategy = tools.ConflictStrategyVersionBased
		registry := tools.NewRegistryWithConfig(config)
		
		tool1 := createTestTool("test1", "TestTool")
		tool1.Version = "1.0.0"
		tool2 := createTestTool("test1", "TestTool") // Same ID
		tool2.Version = "2.0.0"
		
		require.NoError(t, registry.Register(tool1))
		require.NoError(t, registry.Register(tool2))
		
		retrieved, err := registry.Get("test1")
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", retrieved.Version)
	})
	
	t.Run("CustomMigrationHandler", func(t *testing.T) {
		config := tools.DefaultRegistryConfig()
		config.EnableVersionMigration = true
		config.ConflictResolutionStrategy = tools.ConflictStrategyVersionBased
		registry := tools.NewRegistryWithConfig(config)
		
		migrationCalled := false
		registry.AddMigrationHandler("1.0.0", func(ctx context.Context, oldTool, newTool *tools.Tool) error {
			migrationCalled = true
			return nil
		})
		
		tool1 := createTestTool("test1", "TestTool")
		tool1.Version = "1.0.0"
		tool2 := createTestTool("test1", "TestTool") // Same ID
		tool2.Version = "2.0.0"
		
		require.NoError(t, registry.Register(tool1))
		require.NoError(t, registry.Register(tool2))
		
		assert.True(t, migrationCalled)
	})
	
	t.Run("MigrationCompatibilityCheck", func(t *testing.T) {
		config := tools.DefaultRegistryConfig()
		config.EnableVersionMigration = true
		config.ConflictResolutionStrategy = tools.ConflictStrategyVersionBased
		registry := tools.NewRegistryWithConfig(config)
		
		tool1 := createTestTool("test1", "TestTool")
		tool1.Version = "1.0.0"
		tool1.Schema.Properties["param1"] = &tools.Property{Type: "string"}
		tool1.Schema.Properties["param2"] = &tools.Property{Type: "string"}
		tool1.Schema.Required = []string{"param1", "param2"}
		
		tool2 := createTestTool("test1", "TestTool") // Same ID
		tool2.Version = "2.0.0"
		tool2.Schema.Properties["param1"] = &tools.Property{Type: "string"}
		tool2.Schema.Required = []string{"param1"} // Missing param2
		
		require.NoError(t, registry.Register(tool1))
		err := registry.Register(tool2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required parameter")
	})
}

// testCategoryTree tests hierarchical categorization
func testCategoryTree(t *testing.T) {
	t.Run("BasicCategoryOperations", func(t *testing.T) {
		registry := tools.NewRegistry()
		
		tool1 := createTestToolWithMetadata("test1", "TestTool1", []string{"system/file"}, nil)
		tool2 := createTestToolWithMetadata("test2", "TestTool2", []string{"system/network"}, nil)
		tool3 := createTestToolWithMetadata("test3", "TestTool3", []string{"user/ui"}, nil)
		
		require.NoError(t, registry.Register(tool1))
		require.NoError(t, registry.Register(tool2))
		require.NoError(t, registry.Register(tool3))
		
		// Test category retrieval
		systemTools, err := registry.GetByCategory("system/file")
		require.NoError(t, err)
		assert.Len(t, systemTools, 1)
		assert.Equal(t, "test1", systemTools[0].ID)
		
		// Test category tree
		tree := registry.GetCategoryTree()
		assert.NotNil(t, tree)
		
		categories := tree.GetAllCategories()
		assert.Contains(t, categories, "system/file")
		assert.Contains(t, categories, "system/network")
		assert.Contains(t, categories, "user/ui")
	})
	
	t.Run("CategoryTreeHierarchy", func(t *testing.T) {
		tree := tools.NewCategoryTree()
		
		metadata := map[string]interface{}{
			"description": "System tools",
		}
		
		require.NoError(t, tree.AddCategory("system/file/io", metadata))
		require.NoError(t, tree.AddTool("system/file/io", "tool1"))
		
		tools := tree.GetToolsInCategory("system/file/io")
		assert.Contains(t, tools, "tool1")
		
		node := tree.GetCategoryNode("system/file/io")
		assert.NotNil(t, node)
		assert.Equal(t, "io", node.Name)
		assert.Equal(t, "system/file/io", node.Path)
	})
}

// testDynamicLoading tests dynamic tool loading
func testDynamicLoading(t *testing.T) {
	t.Run("LoadFromFile", func(t *testing.T) {
		registry := tools.NewRegistry()
		
		// Create a temporary file with tool definitions
		tools := []*tools.Tool{
			createTestTool("test1", "TestTool1"),
			createTestTool("test2", "TestTool2"),
		}
		
		tempFile, err := ioutil.TempFile("", "tools_*.json")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		
		// Write tools to file
		err = writeToolsToFile(tools, tempFile.Name())
		require.NoError(t, err)
		
		// Load tools from file
		err = registry.LoadFromFile(context.Background(), tempFile.Name())
		require.NoError(t, err)
		
		// Verify tools were loaded
		assert.Equal(t, 2, registry.Count())
		
		tool1, err := registry.Get("test1")
		require.NoError(t, err)
		assert.Equal(t, "TestTool1", tool1.Name)
	})
	
	t.Run("LoadFromURL", func(t *testing.T) {
		registry := tools.NewRegistry()
		
		// Create a test HTTP server
		tools := []*tools.Tool{
			createTestTool("test1", "TestTool1"),
			createTestTool("test2", "TestTool2"),
		}
		
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			writeToolsToWriter(tools, w)
		}))
		defer server.Close()
		
		// Load tools from URL
		err := registry.LoadFromURL(context.Background(), server.URL)
		require.NoError(t, err)
		
		// Verify tools were loaded
		assert.Equal(t, 2, registry.Count())
		
		tool1, err := registry.Get("test1")
		require.NoError(t, err)
		assert.Equal(t, "TestTool1", tool1.Name)
	})
}

// testHotReloading tests hot reloading functionality
func testHotReloading(t *testing.T) {
	t.Run("BasicHotReloading", func(t *testing.T) {
		config := tools.DefaultRegistryConfig()
		config.EnableHotReloading = true
		config.HotReloadInterval = 100 * time.Millisecond
		registry := tools.NewRegistryWithConfig(config)
		
		// Create a temporary file
		tempFile, err := ioutil.TempFile("", "tools_*.json")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		
		// Write initial tools
		initialTools := []*tools.Tool{
			createTestTool("test1", "TestTool1"),
		}
		err = writeToolsToFile(initialTools, tempFile.Name())
		require.NoError(t, err)
		
		// Load initial tools
		err = registry.LoadFromFile(context.Background(), tempFile.Name())
		require.NoError(t, err)
		
		// Start watching the file
		err = registry.WatchFile(tempFile.Name())
		require.NoError(t, err)
		
		// Verify initial tool count
		assert.Equal(t, 1, registry.Count())
		
		// Clear registry first to simulate reloading
		registry.Clear()
		
		// Update the file with new tools
		updatedTools := []*tools.Tool{
			createTestTool("test1", "TestTool1"),
			createTestTool("test2", "TestTool2"),
		}
		err = writeToolsToFile(updatedTools, tempFile.Name())
		require.NoError(t, err)
		
		// Wait for hot reload
		time.Sleep(200 * time.Millisecond)
		
		// Verify tools were reloaded
		assert.Equal(t, 2, registry.Count())
		
		// Stop watching
		err = registry.StopWatching(tempFile.Name())
		require.NoError(t, err)
	})
}

// testDependencyGraph tests dependency management
func testDependencyGraph(t *testing.T) {
	t.Run("BasicDependencyResolution", func(t *testing.T) {
		registry := tools.NewRegistry()
		
		// Create tools with dependencies
		tool1 := createTestTool("tool1", "Tool1")
		tool2 := createTestToolWithMetadata("tool2", "Tool2", nil, []string{"tool1"})
		tool3 := createTestToolWithMetadata("tool3", "Tool3", nil, []string{"tool2"})
		
		require.NoError(t, registry.Register(tool1))
		require.NoError(t, registry.Register(tool2))
		require.NoError(t, registry.Register(tool3))
		
		// Test dependency resolution
		deps, err := registry.GetDependenciesWithConstraints("tool3")
		require.NoError(t, err)
		assert.Len(t, deps, 2) // tool1 and tool2
		
		// Verify dependency IDs
		depIDs := make(map[string]bool)
		for _, dep := range deps {
			depIDs[dep.ID] = true
		}
		assert.True(t, depIDs["tool1"])
		assert.True(t, depIDs["tool2"])
	})
	
	t.Run("VersionConstraints", func(t *testing.T) {
		registry := tools.NewRegistry()
		
		// Create tools with version constraints
		tool1 := createTestTool("tool1", "Tool1")
		tool1.Version = "1.0.0"
		
		tool2 := createTestTool("tool2", "Tool2")
		tool2.Version = "2.0.0"
		
		require.NoError(t, registry.Register(tool1))
		require.NoError(t, registry.Register(tool2))
		
		// Add dependency with version constraint
		dg := tools.NewDependencyGraph()
		
		err := dg.AddDependency("tool2", "tool1", ">=1.0.0", false)
		require.NoError(t, err)
		
		// Test dependency resolution with constraints
		deps, err := dg.ResolveDependencies("tool2", map[string]*tools.Tool{
			"tool1": tool1,
			"tool2": tool2,
		})
		require.NoError(t, err)
		assert.Len(t, deps, 1)
		assert.Equal(t, "tool1", deps[0].ID)
	})
	
	t.Run("CircularDependencyDetection", func(t *testing.T) {
		dg := tools.NewDependencyGraph()
		
		// Create circular dependency: tool1 -> tool2 -> tool1
		require.NoError(t, dg.AddDependency("tool1", "tool2", "", false))
		require.NoError(t, dg.AddDependency("tool2", "tool1", "", false))
		
		// Check for circular dependencies
		hasCircular := dg.HasCircularDependencies()
		assert.True(t, hasCircular)
	})
}

// Helper functions for testing

func writeToolsToFile(tools []*tools.Tool, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	return writeToolsToWriter(tools, file)
}

func writeToolsToWriter(tools []*tools.Tool, writer interface {
	Write([]byte) (int, error)
}) error {
	// Simple JSON encoding for testing
	var jsonData strings.Builder
	jsonData.WriteString("[")
	
	for i, tool := range tools {
		if i > 0 {
			jsonData.WriteString(",")
		}
		
		jsonData.WriteString(fmt.Sprintf(`{
			"id": "%s",
			"name": "%s",
			"description": "%s",
			"version": "%s",
			"schema": {
				"type": "object",
				"properties": {}
			}
		}`, tool.ID, tool.Name, tool.Description, tool.Version))
	}
	
	jsonData.WriteString("]")
	
	_, err := writer.Write([]byte(jsonData.String()))
	return err
}

// TestRegistryThreadSafety tests thread safety of the enhanced registry
func TestRegistryThreadSafety(t *testing.T) {
	config := &tools.RegistryConfig{
		MaxTools:                   2000, // Allow more tools
		MaxConcurrentRegistrations: 200, // Allow more concurrent registrations
		EnableToolLRU:              false, // Disable LRU for this test
	}
	registry := tools.NewRegistryWithConfig(config)
	
	const numGoroutines = 100
	const numOperations = 10
	
	var wg sync.WaitGroup
	
	// Test concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				tool := createTestTool(fmt.Sprintf("tool_%d_%d", id, j), fmt.Sprintf("Tool %d %d", id, j))
				err := registry.Register(tool)
				assert.NoError(t, err, "Registration should succeed with proper configuration")
			}
		}(i)
	}
	
	// Test concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				registry.Count()
				registry.ListAll()
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify final state
	actualCount := registry.Count()
	expectedCount := numGoroutines * numOperations
	assert.Equal(t, expectedCount, actualCount, "All tools should be registered with proper configuration")
}

// TestRegistryPerformance tests performance of the enhanced registry
func TestRegistryPerformance(t *testing.T) {
	registry := tools.NewRegistry()
	
	// Register a large number of tools
	const numTools = 1000
	start := time.Now()
	
	for i := 0; i < numTools; i++ {
		tool := createTestTool(fmt.Sprintf("tool_%d", i), fmt.Sprintf("Tool %d", i))
		require.NoError(t, registry.Register(tool))
	}
	
	registrationTime := time.Since(start)
	t.Logf("Registered %d tools in %v", numTools, registrationTime)
	
	// Test lookup performance
	start = time.Now()
	for i := 0; i < numTools; i++ {
		_, err := registry.Get(fmt.Sprintf("tool_%d", i))
		require.NoError(t, err)
	}
	
	lookupTime := time.Since(start)
	t.Logf("Looked up %d tools in %v", numTools, lookupTime)
	
	// Test listing performance
	start = time.Now()
	tools, err := registry.ListAll()
	require.NoError(t, err)
	assert.Len(t, tools, numTools)
	
	listingTime := time.Since(start)
	t.Logf("Listed %d tools in %v", numTools, listingTime)
	
	// Performance assertions (these may need adjustment based on hardware)
	assert.Less(t, registrationTime, 100*time.Millisecond)
	assert.Less(t, lookupTime, 50*time.Millisecond)
	assert.Less(t, listingTime, 50*time.Millisecond)
}

// TestRegistryConfiguration tests configuration management
func TestRegistryConfiguration(t *testing.T) {
	t.Run("DefaultConfiguration", func(t *testing.T) {
		registry := tools.NewRegistry()
		config := registry.GetConfig()
		
		assert.NotNil(t, config)
		assert.False(t, config.EnableHotReloading)
		assert.Equal(t, 30*time.Second, config.HotReloadInterval)
		assert.Equal(t, 10, config.MaxDependencyDepth)
		assert.Equal(t, tools.ConflictStrategyError, config.ConflictResolutionStrategy)
	})
	
	t.Run("CustomConfiguration", func(t *testing.T) {
		config := &tools.RegistryConfig{
			EnableHotReloading:         true,
			HotReloadInterval:          10 * time.Second,
			MaxDependencyDepth:         5,
			ConflictResolutionStrategy: tools.ConflictStrategyOverwrite,
			EnableVersionMigration:     false,
			MigrationTimeout:           60 * time.Second,
			LoadingTimeout:             30 * time.Second,
			EnableCaching:              false,
			CacheExpiration:            10 * time.Minute,
		}
		
		registry := tools.NewRegistryWithConfig(config)
		retrievedConfig := registry.GetConfig()
		
		assert.Equal(t, config.EnableHotReloading, retrievedConfig.EnableHotReloading)
		assert.Equal(t, config.HotReloadInterval, retrievedConfig.HotReloadInterval)
		assert.Equal(t, config.MaxDependencyDepth, retrievedConfig.MaxDependencyDepth)
		assert.Equal(t, config.ConflictResolutionStrategy, retrievedConfig.ConflictResolutionStrategy)
		assert.Equal(t, config.EnableVersionMigration, retrievedConfig.EnableVersionMigration)
	})
	
	t.Run("ConfigurationUpdate", func(t *testing.T) {
		registry := tools.NewRegistry()
		
		newConfig := &tools.RegistryConfig{
			EnableHotReloading:         true,
			HotReloadInterval:          5 * time.Second,
			MaxDependencyDepth:         20,
			ConflictResolutionStrategy: tools.ConflictStrategyVersionBased,
			EnableVersionMigration:     true,
			MigrationTimeout:           45 * time.Second,
			LoadingTimeout:             15 * time.Second,
			EnableCaching:              true,
			CacheExpiration:            15 * time.Minute,
		}
		
		registry.SetConfig(newConfig)
		retrievedConfig := registry.GetConfig()
		
		assert.Equal(t, newConfig.EnableHotReloading, retrievedConfig.EnableHotReloading)
		assert.Equal(t, newConfig.HotReloadInterval, retrievedConfig.HotReloadInterval)
		assert.Equal(t, newConfig.MaxDependencyDepth, retrievedConfig.MaxDependencyDepth)
		assert.Equal(t, newConfig.ConflictResolutionStrategy, retrievedConfig.ConflictResolutionStrategy)
	})
}