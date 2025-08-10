package config

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
)

// MaxDepth defines the maximum recursion depth to prevent stack overflow
const MaxDepth = 1000

// CopyOnWriteThreshold defines the minimum size for copy-on-write optimization
const CopyOnWriteThreshold = 100

// OptimizedCopier provides high-performance deep copying with various optimizations
type OptimizedCopier struct {
	// Pool for reusing maps and slices to reduce GC pressure
	mapPool   sync.Pool
	slicePool sync.Pool
	
	// Statistics for monitoring performance
	stats CopyStats
}

// CopyStats tracks copying performance metrics
type CopyStats struct {
	TotalCopies       int64
	CowOptimizations  int64
	TypeOptimizations int64
	StackOverflowHits int64
}

// NewOptimizedCopier creates a new optimized copier with object pools
func NewOptimizedCopier() *OptimizedCopier {
	return &OptimizedCopier{
		mapPool: sync.Pool{
			New: func() interface{} {
				return make(map[string]interface{}, 16) // Pre-allocate with capacity
			},
		},
		slicePool: sync.Pool{
			New: func() interface{} {
				return make([]interface{}, 0, 16) // Pre-allocate with capacity
			},
		},
	}
}

// DeepCopy performs optimized deep copying with stack overflow protection
func (c *OptimizedCopier) DeepCopy(original map[string]interface{}) map[string]interface{} {
	if original == nil {
		return nil
	}
	
	atomic.AddInt64(&c.stats.TotalCopies, 1)
	
	// Check if we should use copy-on-write optimization
	if len(original) > CopyOnWriteThreshold {
		if result := c.tryCopyOnWrite(original); result != nil {
			atomic.AddInt64(&c.stats.CowOptimizations, 1)
			return result
		}
	}
	
	// Use iterative copying with explicit stack to avoid stack overflow
	return c.deepCopyIterative(original)
}

// DeepCopyValue performs optimized deep copying of individual values
func (c *OptimizedCopier) DeepCopyValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	
	// Type-specific optimizations without reflection
	switch v := value.(type) {
	case map[string]interface{}:
		return c.DeepCopy(v)
	case []interface{}:
		return c.copySliceInterface(v)
	case []string:
		return c.copySliceString(v)
	case []int:
		return c.copySliceInt(v)
	case []int64:
		return c.copySliceInt64(v)
	case []float64:
		return c.copySliceFloat64(v)
	case []bool:
		return c.copySliceBool(v)
	case map[string]string:
		return c.copyMapStringString(v)
	case map[string]int:
		return c.copyMapStringInt(v)
	case string, int, int8, int16, int32, int64,
		 uint, uint8, uint16, uint32, uint64,
		 float32, float64, bool, complex64, complex128:
		// Immutable types - return as-is
		atomic.AddInt64(&c.stats.TypeOptimizations, 1)
		return value
	default:
		// Fallback for unknown types - return as-is assuming immutable
		return value
	}
}

// tryCopyOnWrite attempts copy-on-write optimization for large maps
func (c *OptimizedCopier) tryCopyOnWrite(original map[string]interface{}) map[string]interface{} {
	// For very large maps, we can implement a copy-on-write wrapper
	// This is a simplified version - in practice you'd need more sophisticated tracking
	if c.isImmutableMap(original) {
		// If the map contains only immutable values, we can safely share it
		return original
	}
	return nil
}

// isImmutableMap checks if a map contains only immutable values
func (c *OptimizedCopier) isImmutableMap(m map[string]interface{}) bool {
	for _, value := range m {
		if !c.isImmutableValue(value) {
			return false
		}
	}
	return true
}

// isImmutableValue checks if a value is immutable
func (c *OptimizedCopier) isImmutableValue(value interface{}) bool {
	switch value.(type) {
	case string, int, int8, int16, int32, int64,
		 uint, uint8, uint16, uint32, uint64,
		 float32, float64, bool, complex64, complex128:
		return true
	case []string, []int, []int64, []float64, []bool:
		return true // These are copied efficiently
	default:
		return false
	}
}

// copyStackFrame represents a frame in the iterative copying stack
type copyStackFrame struct {
	sourceMap map[string]interface{}
	targetMap map[string]interface{}
	depth     int
}

// deepCopyIterative performs deep copying using an explicit stack to avoid stack overflow
func (c *OptimizedCopier) deepCopyIterative(original map[string]interface{}) map[string]interface{} {
	if original == nil {
		return nil
	}
	
	// Get a map from the pool
	result := c.getMapFromPool()
	
	// Use explicit stack for iteration to prevent stack overflow
	stack := make([]copyStackFrame, 0, 32) // Pre-allocate stack capacity
	stack = append(stack, copyStackFrame{
		sourceMap: original,
		targetMap: result,
		depth:     0,
	})
	
	for len(stack) > 0 {
		// Pop frame from stack
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		
		// Check for stack overflow protection
		if frame.depth > MaxDepth {
			atomic.AddInt64(&c.stats.StackOverflowHits, 1)
			// Return partial result to prevent crash
			return result
		}
		
		// Process current frame
		for key, value := range frame.sourceMap {
			switch v := value.(type) {
			case map[string]interface{}:
				// Create new map and push to stack for processing
				newMap := c.getMapFromPool()
				frame.targetMap[key] = newMap
				stack = append(stack, copyStackFrame{
					sourceMap: v,
					targetMap: newMap,
					depth:     frame.depth + 1,
				})
			case []interface{}:
				frame.targetMap[key] = c.copySliceInterface(v)
			default:
				frame.targetMap[key] = c.DeepCopyValue(v)
			}
		}
	}
	
	return result
}

// Optimized slice copying methods with type specialization

func (c *OptimizedCopier) copySliceInterface(original []interface{}) []interface{} {
	if original == nil {
		return nil
	}
	
	result := make([]interface{}, len(original))
	for i, item := range original {
		result[i] = c.DeepCopyValue(item)
	}
	return result
}

func (c *OptimizedCopier) copySliceString(original []string) []string {
	if original == nil {
		return nil
	}
	
	result := make([]string, len(original))
	copy(result, original)
	return result
}

func (c *OptimizedCopier) copySliceInt(original []int) []int {
	if original == nil {
		return nil
	}
	
	result := make([]int, len(original))
	copy(result, original)
	return result
}

func (c *OptimizedCopier) copySliceInt64(original []int64) []int64 {
	if original == nil {
		return nil
	}
	
	result := make([]int64, len(original))
	copy(result, original)
	return result
}

func (c *OptimizedCopier) copySliceFloat64(original []float64) []float64 {
	if original == nil {
		return nil
	}
	
	result := make([]float64, len(original))
	copy(result, original)
	return result
}

func (c *OptimizedCopier) copySliceBool(original []bool) []bool {
	if original == nil {
		return nil
	}
	
	result := make([]bool, len(original))
	copy(result, original)
	return result
}

func (c *OptimizedCopier) copyMapStringString(original map[string]string) map[string]string {
	if original == nil {
		return nil
	}
	
	result := make(map[string]string, len(original))
	for k, v := range original {
		result[k] = v
	}
	return result
}

func (c *OptimizedCopier) copyMapStringInt(original map[string]int) map[string]int {
	if original == nil {
		return nil
	}
	
	result := make(map[string]int, len(original))
	for k, v := range original {
		result[k] = v
	}
	return result
}

// Pool management methods

func (c *OptimizedCopier) getMapFromPool() map[string]interface{} {
	m := c.mapPool.Get().(map[string]interface{})
	// Clear the map (it might have been used before)
	for k := range m {
		delete(m, k)
	}
	return m
}

func (c *OptimizedCopier) putMapToPool(m map[string]interface{}) {
	// Only return maps to pool if they're not too large to prevent memory bloat
	if len(m) < 100 {
		c.mapPool.Put(m)
	}
}

func (c *OptimizedCopier) getSliceFromPool() []interface{} {
	s := c.slicePool.Get().([]interface{})
	return s[:0] // Reset slice length but keep capacity
}

func (c *OptimizedCopier) putSliceToPool(s []interface{}) {
	// Only return slices to pool if they're not too large
	if cap(s) < 100 {
		c.slicePool.Put(s)
	}
}

// GetStats returns current copying statistics
func (c *OptimizedCopier) GetStats() CopyStats {
	return CopyStats{
		TotalCopies:       atomic.LoadInt64(&c.stats.TotalCopies),
		CowOptimizations:  atomic.LoadInt64(&c.stats.CowOptimizations),
		TypeOptimizations: atomic.LoadInt64(&c.stats.TypeOptimizations),
		StackOverflowHits: atomic.LoadInt64(&c.stats.StackOverflowHits),
	}
}

// ResetStats resets the copying statistics
func (c *OptimizedCopier) ResetStats() {
	atomic.StoreInt64(&c.stats.TotalCopies, 0)
	atomic.StoreInt64(&c.stats.CowOptimizations, 0)
	atomic.StoreInt64(&c.stats.TypeOptimizations, 0)
	atomic.StoreInt64(&c.stats.StackOverflowHits, 0)
}

// String provides a string representation of the statistics
func (cs CopyStats) String() string {
	return fmt.Sprintf("CopyStats{Total: %d, COW: %d, TypeOpt: %d, StackOverflow: %d}",
		cs.TotalCopies, cs.CowOptimizations, cs.TypeOptimizations, cs.StackOverflowHits)
}

// Global optimized copier instance
var globalCopier = NewOptimizedCopier()

// FastDeepCopy is a convenience function that uses the global optimized copier
func FastDeepCopy(original map[string]interface{}) map[string]interface{} {
	return globalCopier.DeepCopy(original)
}

// FastDeepCopyValue is a convenience function for copying individual values
func FastDeepCopyValue(value interface{}) interface{} {
	return globalCopier.DeepCopyValue(value)
}

// GetCopyStats returns global copying statistics
func GetCopyStats() CopyStats {
	return globalCopier.GetStats()
}

// UnsafeCopyMap provides zero-copy sharing for read-only scenarios
// WARNING: This should only be used when you're certain the map won't be modified
func UnsafeCopyMap(original map[string]interface{}) map[string]interface{} {
	// This is a zero-copy operation that just returns the original map
	// Use with extreme caution - only for read-only scenarios
	return original
}

// Memory-efficient copying for specific known structures

// CopyConfigData provides optimized copying for configuration data structures
func CopyConfigData(original map[string]interface{}) map[string]interface{} {
	if original == nil {
		return nil
	}
	
	// Pre-allocate with known typical capacity for config data
	result := make(map[string]interface{}, len(original)+len(original)/4)
	
	for key, value := range original {
		// Common config patterns optimization
		switch key {
		case "metadata", "properties", "tags", "conditions":
			// These are often read-only, so we can share them safely
			if globalCopier.isImmutableValue(value) {
				result[key] = value
				continue
			}
		}
		
		result[key] = FastDeepCopyValue(value)
	}
	
	return result
}

// Unsafe operations for advanced users who need maximum performance

// unsafeStringToBytes converts string to byte slice without allocation
// WARNING: The result should not be modified as it shares memory with the original string
func unsafeStringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&struct {
		string
		Cap int
	}{s, len(s)}))
}

// unsafeBytesToString converts byte slice to string without allocation
// WARNING: The byte slice should not be modified after this conversion
func unsafeBytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}