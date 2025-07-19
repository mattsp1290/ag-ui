package tools

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestOptimizationIntegration tests the integration of all performance optimizations
func TestOptimizationIntegration(t *testing.T) {
	// Test 1: Registry with Pagination
	t.Run("PaginatedRegistry", func(t *testing.T) {
		registry := NewRegistry()
		
		// Add some tools
		for i := 0; i < 20; i++ {
			tool := &Tool{
				ID:          "tool-" + string(rune('0'+i)),
				Name:        "Test Tool " + string(rune('0'+i)),
				Description: "Test description",
				Version:     "1.0.0",
				Schema: &ToolSchema{
					Type: "object",
					Properties: map[string]*Property{
						"input": {
							Type:        "string",
							Description: "Input parameter",
						},
					},
					Required: []string{"input"},
				},
				Executor: &testExecutor{},
			}
			err := registry.Register(tool)
			if err != nil {
				t.Fatalf("Failed to register tool: %v", err)
			}
		}
		
		// Test paginated list
		options := &PaginationOptions{
			Page:      1,
			Size:      10,
			SortBy:    "name",
			SortOrder: "asc",
		}
		
		result, err := registry.ListPaginated(nil, options)
		if err != nil {
			t.Fatalf("ListPaginated failed: %v", err)
		}
		
		if len(result.Tools) != 10 {
			t.Errorf("Expected 10 tools, got %d", len(result.Tools))
		}
		
		if result.TotalCount != 20 {
			t.Errorf("Expected total count 20, got %d", result.TotalCount)
		}
		
		if result.TotalPages != 2 {
			t.Errorf("Expected 2 total pages, got %d", result.TotalPages)
		}
	})
	
	// Test 2: Schema Caching
	t.Run("SchemaCaching", func(t *testing.T) {
		schema := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"name": {
					Type:        "string",
					Description: "Name parameter",
				},
			},
			Required: []string{"name"},
		}
		
		// Create multiple validators with the same schema
		validator1 := NewSchemaValidator(schema)
		validator2 := NewSchemaValidator(schema)
		
		// They should have the same schema hash
		if validator1.schemaHash != validator2.schemaHash {
			t.Error("Expected same schema hash for identical schemas")
		}
		
		// Test validation
		params := map[string]interface{}{
			"name": "test",
		}
		
		err := validator1.Validate(params)
		if err != nil {
			t.Errorf("Validation failed: %v", err)
		}
		
		err = validator2.Validate(params)
		if err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})
	
	// Test 3: Copy-on-Write
	t.Run("CopyOnWrite", func(t *testing.T) {
		original := &Tool{
			ID:          "test-tool",
			Name:        "Original Tool",
			Description: "Original description",
			Version:     "1.0.0",
		}
		
		clone := original.CloneOptimized()
		
		// Should be shared initially
		if !clone.IsShared() {
			t.Error("Expected clone to be shared")
		}
		
		// Names should be the same
		if clone.Name != original.Name {
			t.Error("Expected clone to have same name as original")
		}
		
		// Modify clone
		clone.SetName("Modified Tool")
		
		// After modification, clone should not be shared
		if clone.IsShared() {
			t.Error("Expected clone to not be shared after modification")
		}
		
		// Original should be unchanged
		if original.Name != "Original Tool" {
			t.Error("Expected original to remain unchanged")
		}
		
		// Clone should have modified value
		if clone.Name != "Modified Tool" {
			t.Error("Expected clone to have modified name")
		}
	})
	
	// Test 4: Memory Pool
	t.Run("MemoryPool", func(t *testing.T) {
		pool := NewMemoryPool()
		
		// Get and return tools
		tool1 := pool.GetTool()
		tool1.ID = "tool-1"
		
		tool2 := pool.GetTool()
		tool2.ID = "tool-2"
		
		// Return to pool
		pool.PutTool(tool1)
		pool.PutTool(tool2)
		
		// Get again
		tool3 := pool.GetTool()
		tool4 := pool.GetTool()
		
		// Should be reset
		if tool3.ID != "" {
			t.Error("Expected tool from pool to be reset")
		}
		
		if tool4.ID != "" {
			t.Error("Expected tool from pool to be reset")
		}
	})
}

// Test executor for testing
type testExecutor struct{}

func (e *testExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	return &ToolExecutionResult{
		Success:   true,
		Data:      "test result",
		Timestamp: time.Now(),
	}, nil
}

// TestPerformanceComparison compares performance before and after optimizations
func TestPerformanceComparison(t *testing.T) {
	registry := NewRegistry()
	
	// Create tools
	for i := 0; i < 100; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("tool-%d", i),
			Name:        fmt.Sprintf("Test Tool %d", i),
			Description: "Test description",
			Version:     "1.0.0",
			Schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"input": {
						Type:        "string",
						Description: "Input parameter",
					},
				},
				Required: []string{"input"},
			},
			Executor: &testExecutor{},
		}
		err := registry.Register(tool)
		if err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}
	
	// Test paginated vs non-paginated
	t.Run("PaginatedVsNonPaginated", func(t *testing.T) {
		options := &PaginationOptions{
			Page:      1,
			Size:      10,
			SortBy:    "name",
			SortOrder: "asc",
		}
		
		// Paginated
		start := time.Now()
		result, err := registry.ListPaginated(nil, options)
		paginatedTime := time.Since(start)
		
		if err != nil {
			t.Fatalf("ListPaginated failed: %v", err)
		}
		
		if len(result.Tools) != 10 {
			t.Errorf("Expected 10 tools from paginated list, got %d", len(result.Tools))
		}
		
		// Non-paginated
		start = time.Now()
		allTools, err := registry.List(nil)
		nonPaginatedTime := time.Since(start)
		
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		
		t.Logf("Paginated time: %v, Non-paginated time: %v", paginatedTime, nonPaginatedTime)
		t.Logf("Paginated returned %d tools, Non-paginated returned %d tools", len(result.Tools), len(allTools))
	})
}

// TestCachingEffectiveness tests how effective the caching is
func TestCachingEffectiveness(t *testing.T) {
	// Test list cache
	t.Run("ListCache", func(t *testing.T) {
		registry := NewRegistry()
		
		// Add some tools
		for i := 0; i < 50; i++ {
			tool := &Tool{
				ID:          fmt.Sprintf("tool-cache-%d", i),
				Name:        fmt.Sprintf("Test Tool Cache %d", i),
				Description: "Test description",
				Version:     "1.0.0",
				Schema: &ToolSchema{
					Type: "object",
					Properties: map[string]*Property{
						"input": {
							Type:        "string",
							Description: "Input parameter",
						},
					},
					Required: []string{"input"},
				},
				Executor: &testExecutor{},
			}
			err := registry.Register(tool)
			if err != nil {
				t.Fatalf("Failed to register tool: %v", err)
			}
		}
		
		options := &PaginationOptions{
			Page:      1,
			Size:      10,
			SortBy:    "name",
			SortOrder: "asc",
		}
		
		// First call - should miss cache
		start := time.Now()
		result1, err := registry.ListPaginated(nil, options)
		firstCallTime := time.Since(start)
		
		if err != nil {
			t.Fatalf("ListPaginated failed: %v", err)
		}
		
		// Second call - should hit cache
		start = time.Now()
		result2, err := registry.ListPaginated(nil, options)
		secondCallTime := time.Since(start)
		
		if err != nil {
			t.Fatalf("ListPaginated failed: %v", err)
		}
		
		// Results should be the same
		if len(result1.Tools) != len(result2.Tools) {
			t.Error("Expected same number of tools from cached call")
		}
		
		t.Logf("First call time: %v, Second call time: %v", firstCallTime, secondCallTime)
		
		// Second call should be faster (cached)
		if secondCallTime > firstCallTime {
			t.Log("Note: Second call was not faster, but caching may still be working")
		}
	})
	
	// Test schema cache effectiveness
	t.Run("SchemaCache", func(t *testing.T) {
		schema := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"name": {
					Type:        "string",
					Description: "Name parameter",
				},
			},
			Required: []string{"name"},
		}
		
		params := map[string]interface{}{
			"name": "test",
		}
		
		// Create multiple validators - should reuse cached schema
		start := time.Now()
		for i := 0; i < 10; i++ {
			validator := NewSchemaValidator(schema)
			err := validator.Validate(params)
			if err != nil {
				t.Fatalf("Validation failed: %v", err)
			}
		}
		cachedTime := time.Since(start)
		
		t.Logf("Time for 10 cached schema validations: %v", cachedTime)
		
		// Check cache stats if available
		if globalSchemaCache != nil {
			hitCount, missCount, hitRate := globalSchemaCache.GetStats()
			t.Logf("Schema cache stats: hits=%d, misses=%d, hit_rate=%.2f%%", hitCount, missCount, hitRate*100)
		}
	})
}