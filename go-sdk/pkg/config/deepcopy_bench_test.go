package config

import (
	"fmt"
	"testing"
	"time"
)

// Test data generation helpers

func generateSimpleMap(size int) map[string]interface{} {
	m := make(map[string]interface{}, size)
	for i := 0; i < size; i++ {
		m[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	return m
}

func generateNestedMap(depth, breadth int) map[string]interface{} {
	if depth == 0 {
		return generateSimpleMap(breadth)
	}
	
	m := make(map[string]interface{}, breadth)
	for i := 0; i < breadth; i++ {
		m[fmt.Sprintf("level_%d_key_%d", depth, i)] = generateNestedMap(depth-1, breadth)
	}
	return m
}

func generateMixedTypeMap(size int) map[string]interface{} {
	m := make(map[string]interface{}, size)
	
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("key_%d", i)
		switch i % 7 {
		case 0:
			m[key] = fmt.Sprintf("string_value_%d", i)
		case 1:
			m[key] = i
		case 2:
			m[key] = int64(i)
		case 3:
			m[key] = float64(i) * 1.5
		case 4:
			m[key] = i%2 == 0
		case 5:
			m[key] = []string{fmt.Sprintf("item_%d_1", i), fmt.Sprintf("item_%d_2", i)}
		case 6:
			m[key] = []int{i, i + 1, i + 2}
		}
	}
	
	// Add some nested maps
	for i := 0; i < size/10; i++ {
		m[fmt.Sprintf("nested_%d", i)] = generateSimpleMap(5)
	}
	
	return m
}

func generateLargeSliceMap(sliceSize int) map[string]interface{} {
	m := make(map[string]interface{})
	
	// Large interface slice
	largeSlice := make([]interface{}, sliceSize)
	for i := 0; i < sliceSize; i++ {
		if i%3 == 0 {
			largeSlice[i] = map[string]interface{}{
				"id":   i,
				"name": fmt.Sprintf("item_%d", i),
			}
		} else {
			largeSlice[i] = fmt.Sprintf("item_%d", i)
		}
	}
	m["large_slice"] = largeSlice
	
	// Large string slice
	stringSlice := make([]string, sliceSize)
	for i := 0; i < sliceSize; i++ {
		stringSlice[i] = fmt.Sprintf("string_item_%d", i)
	}
	m["string_slice"] = stringSlice
	
	return m
}

// Benchmark the old reflection-based approach
func BenchmarkOldDeepCopy_Small(b *testing.B) {
	data := generateSimpleMap(10)
	merger := NewMerger(MergeStrategyDeepMerge).(*MergerImpl)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = merger.deepCopyValue(data)
	}
}

func BenchmarkOldDeepCopy_Medium(b *testing.B) {
	data := generateSimpleMap(100)
	merger := NewMerger(MergeStrategyDeepMerge).(*MergerImpl)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = merger.deepCopyValue(data)
	}
}

func BenchmarkOldDeepCopy_Large(b *testing.B) {
	data := generateSimpleMap(1000)
	merger := NewMerger(MergeStrategyDeepMerge).(*MergerImpl)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = merger.deepCopyValue(data)
	}
}

func BenchmarkOldDeepCopy_Nested(b *testing.B) {
	data := generateNestedMap(5, 5) // 5 levels deep, 5 items per level
	merger := NewMerger(MergeStrategyDeepMerge).(*MergerImpl)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = merger.deepCopyValue(data)
	}
}

func BenchmarkOldDeepCopy_MixedTypes(b *testing.B) {
	data := generateMixedTypeMap(100)
	merger := NewMerger(MergeStrategyDeepMerge).(*MergerImpl)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = merger.deepCopyValue(data)
	}
}

// Benchmark the new optimized approach
func BenchmarkOptimizedDeepCopy_Small(b *testing.B) {
	data := generateSimpleMap(10)
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = copier.DeepCopy(data)
	}
}

func BenchmarkOptimizedDeepCopy_Medium(b *testing.B) {
	data := generateSimpleMap(100)
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = copier.DeepCopy(data)
	}
}

func BenchmarkOptimizedDeepCopy_Large(b *testing.B) {
	data := generateSimpleMap(1000)
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = copier.DeepCopy(data)
	}
}

func BenchmarkOptimizedDeepCopy_Nested(b *testing.B) {
	data := generateNestedMap(5, 5) // 5 levels deep, 5 items per level
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = copier.DeepCopy(data)
	}
}

func BenchmarkOptimizedDeepCopy_MixedTypes(b *testing.B) {
	data := generateMixedTypeMap(100)
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = copier.DeepCopy(data)
	}
}

// Benchmark convenience function
func BenchmarkFastDeepCopy_Medium(b *testing.B) {
	data := generateSimpleMap(100)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = FastDeepCopy(data)
	}
}

// Benchmark specific scenarios
func BenchmarkOptimizedDeepCopy_LargeSlices(b *testing.B) {
	data := generateLargeSliceMap(1000)
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = copier.DeepCopy(data)
	}
}

func BenchmarkOptimizedDeepCopy_DeepNesting(b *testing.B) {
	data := generateNestedMap(10, 3) // 10 levels deep, 3 items per level
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = copier.DeepCopy(data)
	}
}

// Memory efficiency tests
func BenchmarkMemoryEfficiency_Repeated(b *testing.B) {
	data := generateMixedTypeMap(50)
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		result := copier.DeepCopy(data)
		_ = result // Prevent optimization
		// The pool should reuse memory across iterations
	}
}

// Type-specific optimizations
func BenchmarkTypeSpecific_StringSlice(b *testing.B) {
	data := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		data[i] = fmt.Sprintf("item_%d", i)
	}
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = copier.copySliceString(data)
	}
}

func BenchmarkTypeSpecific_IntSlice(b *testing.B) {
	data := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		data[i] = i
	}
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = copier.copySliceInt(data)
	}
}

func BenchmarkTypeSpecific_MapStringString(b *testing.B) {
	data := make(map[string]string, 100)
	for i := 0; i < 100; i++ {
		data[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = copier.copyMapStringString(data)
	}
}

// Copy-on-write benchmarks
func BenchmarkCopyOnWrite_ImmutableMap(b *testing.B) {
	// Create a map with only immutable values
	data := make(map[string]interface{}, 200)
	for i := 0; i < 200; i++ {
		switch i % 3 {
		case 0:
			data[fmt.Sprintf("str_%d", i)] = fmt.Sprintf("value_%d", i)
		case 1:
			data[fmt.Sprintf("int_%d", i)] = i
		case 2:
			data[fmt.Sprintf("float_%d", i)] = float64(i) * 1.5
		}
	}
	
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = copier.DeepCopy(data)
	}
}

// Stack overflow protection benchmark
func BenchmarkStackOverflowProtection(b *testing.B) {
	// Create a very deeply nested structure
	data := generateNestedMap(MaxDepth+10, 2) // Exceed max depth
	copier := NewOptimizedCopier()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		result := copier.DeepCopy(data)
		_ = result
		
		// Check if stack overflow was hit
		stats := copier.GetStats()
		if stats.StackOverflowHits > 0 {
			b.Logf("Stack overflow protection triggered %d times", stats.StackOverflowHits)
			copier.ResetStats() // Reset for next iteration
		}
	}
}

// Comparative benchmark across different data sizes
func BenchmarkComparative_OldVsNew(b *testing.B) {
	sizes := []int{10, 50, 100, 500, 1000}
	
	for _, size := range sizes {
		data := generateMixedTypeMap(size)
		merger := NewMerger(MergeStrategyDeepMerge).(*MergerImpl)
		copier := NewOptimizedCopier()
		
		b.Run(fmt.Sprintf("Old_Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = merger.deepCopyValue(data)
			}
		})
		
		b.Run(fmt.Sprintf("New_Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = copier.DeepCopy(data)
			}
		})
	}
}

// Real-world configuration copying benchmark
func BenchmarkConfigCopy_Realistic(b *testing.B) {
	// Simulate realistic configuration data
	data := map[string]interface{}{
		"app": map[string]interface{}{
			"name":    "test-app",
			"version": "1.0.0",
			"port":    8080,
			"debug":   true,
		},
		"database": map[string]interface{}{
			"host":     "localhost",
			"port":     5432,
			"name":     "testdb",
			"ssl":      false,
			"pool_size": 10,
			"timeouts": map[string]interface{}{
				"connect": "30s",
				"read":    "10s",
				"write":   "5s",
			},
		},
		"cache": map[string]interface{}{
			"type": "redis",
			"endpoints": []string{
				"redis-1:6379",
				"redis-2:6379",
				"redis-3:6379",
			},
			"config": map[string]interface{}{
				"max_retries": 3,
				"timeout":     "1s",
			},
		},
		"logging": map[string]interface{}{
			"level": "info",
			"outputs": []string{"stdout", "file"},
			"file": map[string]interface{}{
				"path":       "/var/log/app.log",
				"max_size":   "100MB",
				"max_backups": 10,
			},
		},
		"features": map[string]interface{}{
			"feature_flags": map[string]bool{
				"new_ui":       true,
				"beta_feature": false,
				"analytics":    true,
			},
			"limits": map[string]int{
				"max_users":    1000,
				"max_requests": 10000,
				"rate_limit":   100,
			},
		},
		"metadata": map[string]interface{}{
			"created_at":   time.Now().Format(time.RFC3339),
			"updated_at":   time.Now().Format(time.RFC3339),
			"environment":  "development",
			"region":       "us-east-1",
		},
	}
	
	b.Run("Old", func(b *testing.B) {
		merger := NewMerger(MergeStrategyDeepMerge).(*MergerImpl)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = merger.deepCopyValue(data)
		}
	})
	
	b.Run("New", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = FastDeepCopy(data)
		}
	})
	
	b.Run("ConfigSpecific", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = CopyConfigData(data)
		}
	})
}

// Thread safety benchmark
func BenchmarkConcurrentCopy(b *testing.B) {
	data := generateMixedTypeMap(100)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = FastDeepCopy(data)
		}
	})
}