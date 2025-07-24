package tools

import (
	"fmt"
	"testing"
	"time"
)

// createMinimalBenchmarkSchema creates a minimal valid schema for benchmark test tools
func createMinimalBenchmarkSchema() *ToolSchema {
	return &ToolSchema{
		Type: "object",
		Properties: map[string]*Property{
			"input": {
				Type:        "string",
				Description: "Test input parameter",
			},
		},
	}
}

// BenchmarkRegistryWithCleanup benchmarks registry operations with cleanup enabled
func BenchmarkRegistryWithCleanup(b *testing.B) {
	config := &RegistryConfig{
		MaxTools:                     1000,
		EnableToolLRU:                true,
		ToolTTL:                      1 * time.Hour, // Long TTL to avoid cleanup during benchmark
		ToolCleanupInterval:          30 * time.Minute,
		EnableBackgroundToolCleanup: false, // Disable background cleanup for consistent benchmarks
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	b.Run("Register", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tool := &Tool{
				ID:          fmt.Sprintf("bench-tool-%d", i),
				Name:        fmt.Sprintf("Benchmark Tool %d", i),
				Description: "Benchmark test tool",
				Version:     "1.0.0",
				Schema:      createMinimalBenchmarkSchema(),
				Executor:    &testExecutor{},
			}
			_ = registry.Register(tool) // Ignore errors due to eviction
		}
	})

	// Pre-populate registry for get benchmarks
	for i := 0; i < 500; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("get-tool-%d", i),
			Name:        fmt.Sprintf("Get Tool %d", i),
			Description: "Get benchmark test tool",
			Version:     "1.0.0",
			Schema:      createMinimalBenchmarkSchema(),
			Executor:    &testExecutor{},
		}
		_ = registry.Register(tool)
	}

	b.Run("Get", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			toolID := fmt.Sprintf("get-tool-%d", i%500)
			_, _ = registry.Get(toolID) // Ignore errors
		}
	})

	b.Run("List", func(b *testing.B) {
		filter := &ToolFilter{
			Tags: []string{"benchmark"},
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = registry.List(filter)
		}
	})

	b.Run("Count", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.Count()
		}
	})
}

// BenchmarkRegistryWithoutCleanup benchmarks registry operations without cleanup for comparison
func BenchmarkRegistryWithoutCleanup(b *testing.B) {
	config := &RegistryConfig{
		MaxTools:                     0, // Unlimited
		EnableToolLRU:                false,
		ToolTTL:                      0, // No TTL
		EnableBackgroundToolCleanup: false,
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	b.Run("Register", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tool := &Tool{
				ID:          fmt.Sprintf("bench-tool-no-cleanup-%d", i),
				Name:        fmt.Sprintf("Benchmark Tool No Cleanup %d", i),
				Description: "Benchmark test tool without cleanup",
				Version:     "1.0.0",
				Schema:      createMinimalBenchmarkSchema(),
				Executor:    &testExecutor{},
			}
			_ = registry.Register(tool)
		}
	})

	// Pre-populate registry for get benchmarks
	for i := 0; i < 500; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("get-tool-no-cleanup-%d", i),
			Name:        fmt.Sprintf("Get Tool No Cleanup %d", i),
			Description: "Get benchmark test tool without cleanup",
			Version:     "1.0.0",
			Schema:      createMinimalBenchmarkSchema(),
			Executor:    &testExecutor{},
		}
		_ = registry.Register(tool)
	}

	b.Run("Get", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			toolID := fmt.Sprintf("get-tool-no-cleanup-%d", i%500)
			_, _ = registry.Get(toolID) // Ignore errors
		}
	})

	b.Run("List", func(b *testing.B) {
		filter := &ToolFilter{
			Tags: []string{"benchmark"},
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = registry.List(filter)
		}
	})

	b.Run("Count", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.Count()
		}
	})
}

// BenchmarkRegistryCleanupOperations benchmarks the cleanup operations themselves
func BenchmarkRegistryCleanupOperations(b *testing.B) {
	config := &RegistryConfig{
		MaxTools:                     10000,
		EnableToolLRU:                true,
		ToolTTL:                      1 * time.Millisecond, // Very short TTL for cleanup testing
		EnableBackgroundToolCleanup: false,
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	// Pre-populate registry with tools that will expire
	for i := 0; i < 1000; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("cleanup-tool-%d", i),
			Name:        fmt.Sprintf("Cleanup Tool %d", i),
			Description: "Cleanup benchmark test tool",
			Version:     "1.0.0",
			Schema:      createMinimalBenchmarkSchema(),
			Executor:    &testExecutor{},
		}
		_ = registry.Register(tool)
	}

	// Wait for tools to expire
	time.Sleep(5 * time.Millisecond)

	b.Run("CleanupExpiredTools", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Re-populate between iterations since cleanup removes tools
			if i > 0 {
				for j := 0; j < 100; j++ {
					tool := &Tool{
						ID:          fmt.Sprintf("cleanup-tool-%d-%d", i, j),
						Name:        fmt.Sprintf("Cleanup Tool %d-%d", i, j),
						Description: "Cleanup benchmark test tool",
						Version:     "1.0.0",
						Schema:      createMinimalBenchmarkSchema(),
						Executor:    &testExecutor{},
					}
					_ = registry.Register(tool)
				}
				time.Sleep(5 * time.Millisecond) // Let them expire
			}
			
			_, _ = registry.CleanupExpiredTools()
		}
	})

	// Re-populate for access time cleanup benchmark
	for i := 0; i < 1000; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("access-cleanup-tool-%d", i),
			Name:        fmt.Sprintf("Access Cleanup Tool %d", i),
			Description: "Access cleanup benchmark test tool",
			Version:     "1.0.0",
			Schema:      createMinimalBenchmarkSchema(),
			Executor:    &testExecutor{},
		}
		_ = registry.Register(tool)
	}

	time.Sleep(5 * time.Millisecond) // Age the tools

	b.Run("CleanupByAccessTime", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Re-populate between iterations
			if i > 0 {
				for j := 0; j < 100; j++ {
					tool := &Tool{
						ID:          fmt.Sprintf("access-cleanup-tool-%d-%d", i, j),
						Name:        fmt.Sprintf("Access Cleanup Tool %d-%d", i, j),
						Description: "Access cleanup benchmark test tool",
						Version:     "1.0.0",
						Schema:      createMinimalBenchmarkSchema(),
						Executor:    &testExecutor{},
					}
					_ = registry.Register(tool)
				}
				time.Sleep(5 * time.Millisecond) // Age them
			}
			
			_, _ = registry.CleanupByAccessTime(1 * time.Millisecond)
		}
	})

	b.Run("ClearAllTools", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Re-populate between iterations
			if i > 0 {
				for j := 0; j < 100; j++ {
					tool := &Tool{
						ID:          fmt.Sprintf("clear-tool-%d-%d", i, j),
						Name:        fmt.Sprintf("Clear Tool %d-%d", i, j),
						Description: "Clear benchmark test tool",
						Version:     "1.0.0",
						Schema:      createMinimalBenchmarkSchema(),
						Executor:    &testExecutor{},
					}
					_ = registry.Register(tool)
				}
			}
			
			_ = registry.ClearAllTools()
		}
	})
}

// BenchmarkRegistryLRUEviction benchmarks LRU eviction performance
func BenchmarkRegistryLRUEviction(b *testing.B) {
	config := &RegistryConfig{
		MaxTools:      100, // Small limit to force frequent evictions
		EnableToolLRU: true,
		ToolTTL:       0, // No TTL
		EnableBackgroundToolCleanup: false,
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("lru-tool-%d", i),
			Name:        fmt.Sprintf("LRU Tool %d", i),
			Description: "LRU benchmark test tool",
			Version:     "1.0.0",
			Schema:      createMinimalBenchmarkSchema(),
			Executor:    &testExecutor{},
		}
		_ = registry.Register(tool) // Will trigger evictions after first 100
	}
}

// BenchmarkRegistryMemoryUsage benchmarks memory usage tracking
func BenchmarkRegistryMemoryUsage(b *testing.B) {
	registry := NewRegistry()
	defer registry.CloseToolsCleanup()

	// Pre-populate with some tools
	for i := 0; i < 100; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("memory-tool-%d", i),
			Name:        fmt.Sprintf("Memory Tool %d", i),
			Description: "Memory usage benchmark test tool with longer description for more realistic memory usage",
			Version:     "1.0.0",
			Schema:      createMinimalBenchmarkSchema(),
			Executor:    &testExecutor{},
			Metadata: &ToolMetadata{
				Author:        "Benchmark Author",
				Documentation: "Detailed metadata for memory usage testing",
				Tags:          []string{"benchmark", "memory", "test", "performance"},
			},
		}
		_ = registry.Register(tool)
	}

	b.Run("GetResourceUsage", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.GetResourceUsage()
		}
	})

	b.Run("GetToolsCleanupStats", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.GetToolsCleanupStats()
		}
	})
}

// BenchmarkRegistryConcurrentOperations benchmarks concurrent operations with cleanup
func BenchmarkRegistryConcurrentOperations(b *testing.B) {
	config := &RegistryConfig{
		MaxTools:                     1000,
		EnableToolLRU:                true,
		ToolTTL:                      1 * time.Hour, // Long TTL
		EnableBackgroundToolCleanup: false,
		MaxConcurrentRegistrations:  10,
	}

	registry := NewRegistryWithConfig(config)
	defer registry.CloseToolsCleanup()

	// Pre-populate for get operations
	for i := 0; i < 500; i++ {
		tool := &Tool{
			ID:          fmt.Sprintf("concurrent-tool-%d", i),
			Name:        fmt.Sprintf("Concurrent Tool %d", i),
			Description: "Concurrent benchmark test tool",
			Version:     "1.0.0",
			Schema:      createMinimalBenchmarkSchema(),
			Executor:    &testExecutor{},
		}
		_ = registry.Register(tool)
	}

	b.Run("ConcurrentRegister", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				tool := &Tool{
					ID:          fmt.Sprintf("parallel-tool-%d", i),
					Name:        fmt.Sprintf("Parallel Tool %d", i),
					Description: "Parallel benchmark test tool",
					Version:     "1.0.0",
					Schema:      createMinimalBenchmarkSchema(),
					Executor:    &testExecutor{},
				}
				_ = registry.Register(tool)
				i++
			}
		})
	})

	b.Run("ConcurrentGet", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				toolID := fmt.Sprintf("concurrent-tool-%d", i%500)
				_, _ = registry.Get(toolID)
				i++
			}
		})
	})

	b.Run("ConcurrentList", func(b *testing.B) {
		filter := &ToolFilter{
			Tags: []string{"benchmark"},
		}
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = registry.List(filter)
			}
		})
	})
}