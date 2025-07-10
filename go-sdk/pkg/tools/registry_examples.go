package tools

import (
	"context"
	"time"
)

// Example usage of enhanced registry features

// ExampleBasicRegistryUsage demonstrates basic registry operations
func ExampleBasicRegistryUsage() {
	// Create a registry with default configuration
	registry := NewRegistry()
	
	// Create a simple tool
	tool := &Tool{
		ID:          "example-tool",
		Name:        "Example Tool",
		Description: "A simple example tool",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"message": {
					Type:        "string",
					Description: "Message to process",
				},
			},
			Required: []string{"message"},
		},
		Executor: &DefaultExecutor{},
	}
	
	// Register the tool
	registry.Register(tool)
	
	// Retrieve and use the tool
	retrievedTool, _ := registry.Get("example-tool")
	_ = retrievedTool
}

// ExampleConflictResolution demonstrates conflict resolution strategies
func ExampleConflictResolution() {
	// Create registry with version-based conflict resolution
	config := DefaultRegistryConfig()
	config.ConflictResolutionStrategy = ConflictStrategyVersionBased
	registry := NewRegistryWithConfig(config)
	
	// Register first version
	tool1 := &Tool{
		ID:      "my-tool",
		Name:    "My Tool",
		Version: "1.0.0",
		Schema:  &ToolSchema{Type: "object"},
		Executor: &DefaultExecutor{},
	}
	registry.Register(tool1)
	
	// Register newer version (will replace the old one)
	tool2 := &Tool{
		ID:      "my-tool",
		Name:    "My Tool",
		Version: "2.0.0",
		Schema:  &ToolSchema{Type: "object"},
		Executor: &DefaultExecutor{},
	}
	registry.Register(tool2) // No error, version 2.0.0 replaces 1.0.0
}

// ExampleCustomConflictResolver demonstrates custom conflict resolution
func ExampleCustomConflictResolver() {
	registry := NewRegistry()
	
	// Add custom conflict resolver that merges descriptions
	registry.AddConflictResolver(func(existing, new *Tool) (*Tool, error) {
		if existing.ID == new.ID {
			merged := existing.Clone()
			merged.Description = existing.Description + " (updated: " + new.Description + ")"
			return merged, nil
		}
		return nil, nil // Let other resolvers handle
	})
	
	// Register tools with same ID
	tool1 := &Tool{
		ID:          "merge-tool",
		Name:        "Merge Tool",
		Description: "Original description",
		Version:     "1.0.0",
		Schema:      &ToolSchema{Type: "object"},
		Executor:    &DefaultExecutor{},
	}
	registry.Register(tool1)
	
	tool2 := &Tool{
		ID:          "merge-tool",
		Name:        "Merge Tool",
		Description: "New description",
		Version:     "1.0.0",
		Schema:      &ToolSchema{Type: "object"},
		Executor:    &DefaultExecutor{},
	}
	registry.Register(tool2) // Descriptions will be merged
}

// ExampleVersionMigration demonstrates version migration with custom handlers
func ExampleVersionMigration() {
	config := DefaultRegistryConfig()
	config.EnableVersionMigration = true
	config.ConflictResolutionStrategy = ConflictStrategyVersionBased
	registry := NewRegistryWithConfig(config)
	
	// Add migration handler for version 1.0.0 -> 2.0.0
	registry.AddMigrationHandler("1.0.0", func(ctx context.Context, oldTool, newTool *Tool) error {
		// Custom migration logic here
		// For example, migrate data, update configurations, etc.
		return nil
	})
	
	// Register old version
	oldTool := &Tool{
		ID:      "migrating-tool",
		Version: "1.0.0",
		Schema:  &ToolSchema{Type: "object"},
		Executor: &DefaultExecutor{},
	}
	registry.Register(oldTool)
	
	// Register new version (migration handler will be called)
	newTool := &Tool{
		ID:      "migrating-tool",
		Version: "2.0.0",
		Schema:  &ToolSchema{Type: "object"},
		Executor: &DefaultExecutor{},
	}
	registry.Register(newTool)
}

// ExampleCategoryTree demonstrates hierarchical categorization
func ExampleCategoryTree() {
	registry := NewRegistry()
	
	// Create tools with hierarchical categories
	systemTool := &Tool{
		ID:      "file-reader",
		Name:    "File Reader",
		Version: "1.0.0",
		Metadata: &ToolMetadata{
			Tags: []string{"system/file", "io"},
		},
		Schema:   &ToolSchema{Type: "object"},
		Executor: &DefaultExecutor{},
	}
	
	uiTool := &Tool{
		ID:      "button-creator",
		Name:    "Button Creator",
		Version: "1.0.0",
		Metadata: &ToolMetadata{
			Tags: []string{"ui/components", "frontend"},
		},
		Schema:   &ToolSchema{Type: "object"},
		Executor: &DefaultExecutor{},
	}
	
	registry.Register(systemTool)
	registry.Register(uiTool)
	
	// Retrieve tools by category
	systemTools, _ := registry.GetByCategory("system/file")
	uiTools, _ := registry.GetByCategory("ui/components")
	
	_ = systemTools
	_ = uiTools
	
	// Access category tree
	categoryTree := registry.GetCategoryTree()
	allCategories := categoryTree.GetAllCategories()
	_ = allCategories
}

// ExampleDynamicLoading demonstrates loading tools from files and URLs
func ExampleDynamicLoading() {
	registry := NewRegistry()
	
	// Load tools from a JSON file
	ctx := context.Background()
	registry.LoadFromFile(ctx, "tools.json")
	
	// Load tools from a URL
	registry.LoadFromURL(ctx, "https://example.com/tools.json")
}

// ExampleHotReloading demonstrates hot reloading of tools
func ExampleHotReloading() {
	config := DefaultRegistryConfig()
	config.EnableHotReloading = true
	config.HotReloadInterval = 10 * time.Second
	registry := NewRegistryWithConfig(config)
	
	// Start watching a file for changes
	registry.WatchFile("tools.json")
	
	// Registry will automatically reload tools when the file changes
	
	// Stop watching when done
	registry.StopWatching("tools.json")
}

// ExampleDependencyManagement demonstrates dependency resolution with constraints
func ExampleDependencyManagement() {
	registry := NewRegistry()
	
	// Create tools with dependencies
	baseTool := &Tool{
		ID:       "base-tool",
		Name:     "Base Tool",
		Version:  "1.0.0",
		Schema:   &ToolSchema{Type: "object"},
		Executor: &DefaultExecutor{},
	}
	
	dependentTool := &Tool{
		ID:      "dependent-tool",
		Name:    "Dependent Tool",
		Version: "1.0.0",
		Metadata: &ToolMetadata{
			Dependencies: []string{"base-tool"},
		},
		Schema:   &ToolSchema{Type: "object"},
		Executor: &DefaultExecutor{},
	}
	
	registry.Register(baseTool)
	registry.Register(dependentTool)
	
	// Resolve dependencies with version constraints
	dependencies, _ := registry.GetDependenciesWithConstraints("dependent-tool")
	_ = dependencies
	
	// Add custom dependency constraints
	graph := registry.GetCategoryTree() // Access internal dependency graph in real usage
	_ = graph
}

// ExampleAdvancedConfiguration demonstrates advanced registry configuration
func ExampleAdvancedConfiguration() {
	config := &RegistryConfig{
		EnableHotReloading:         true,
		HotReloadInterval:          5 * time.Second,
		MaxDependencyDepth:         20,
		ConflictResolutionStrategy: ConflictStrategyPriorityBased,
		EnableVersionMigration:     true,
		MigrationTimeout:           60 * time.Second,
		LoadingTimeout:             30 * time.Second,
		EnableCaching:              true,
		CacheExpiration:            10 * time.Minute,
	}
	
	registry := NewRegistryWithConfig(config)
	
	// Use the configured registry
	_ = registry
	
	// Update configuration at runtime
	newConfig := DefaultRegistryConfig()
	newConfig.EnableHotReloading = false
	registry.SetConfig(newConfig)
}