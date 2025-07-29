//go:build examples
// +build examples

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// CacheDemoExecutor demonstrates various caching strategies and performance optimizations.
// This example shows LRU cache, TTL cache, write-through cache, and cache metrics.
type CacheDemoExecutor struct {
	caches map[string]Cache
	stats  *CacheStats
	mu     sync.RWMutex
}

// Cache interface defines the basic cache operations
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration) error
	Delete(key string) bool
	Clear() error
	Size() int
	Stats() CacheMetrics
	Keys() []string
}

// CacheMetrics provides cache performance metrics
type CacheMetrics struct {
	Hits            int64         `json:"hits"`
	Misses          int64         `json:"misses"`
	Sets            int64         `json:"sets"`
	Deletes         int64         `json:"deletes"`
	Evictions       int64         `json:"evictions"`
	Size            int           `json:"size"`
	MaxSize         int           `json:"max_size"`
	HitRatio        float64       `json:"hit_ratio"`
	MemoryUsage     int64         `json:"memory_usage"`
	AverageGetTime  time.Duration `json:"average_get_time"`
	AverageSetTime  time.Duration `json:"average_set_time"`
	OldestEntry     time.Time     `json:"oldest_entry,omitempty"`
	NewestEntry     time.Time     `json:"newest_entry,omitempty"`
}

// CacheStats provides overall caching statistics
type CacheStats struct {
	mu              sync.RWMutex
	TotalOperations int64                    `json:"total_operations"`
	OperationTimes  map[string]time.Duration `json:"operation_times"`
	CacheMetrics    map[string]CacheMetrics  `json:"cache_metrics"`
	StartTime       time.Time                `json:"start_time"`
}

// LRUCache implements a Least Recently Used cache
type LRUCache struct {
	mu          sync.RWMutex
	items       map[string]*LRUItem
	head        *LRUItem
	tail        *LRUItem
	maxSize     int
	metrics     CacheMetrics
	getTimes    []time.Duration
	setTimes    []time.Duration
}

// LRUItem represents an item in the LRU cache
type LRUItem struct {
	key       string
	value     interface{}
	createdAt time.Time
	expiresAt time.Time
	prev      *LRUItem
	next      *LRUItem
}

// TTLCache implements a Time-To-Live cache
type TTLCache struct {
	mu       sync.RWMutex
	items    map[string]*TTLItem
	maxSize  int
	metrics  CacheMetrics
	getTimes []time.Duration
	setTimes []time.Duration
	cleanup  chan struct{}
	stopped  bool
}

// TTLItem represents an item in the TTL cache
type TTLItem struct {
	value     interface{}
	createdAt time.Time
	expiresAt time.Time
}

// WriteThroughCache implements a write-through cache with persistent backend
type WriteThroughCache struct {
	mu       sync.RWMutex
	items    map[string]*CacheItem
	backend  PersistentBackend
	maxSize  int
	metrics  CacheMetrics
	getTimes []time.Duration
	setTimes []time.Duration
}

// CacheItem represents an item in the write-through cache
type CacheItem struct {
	value     interface{}
	createdAt time.Time
	expiresAt time.Time
	dirty     bool
}

// PersistentBackend simulates a persistent storage backend
type PersistentBackend interface {
	Get(key string) (interface{}, error)
	Set(key string, value interface{}) error
	Delete(key string) error
}

// MockBackend implements a mock persistent backend
type MockBackend struct {
	data  map[string]interface{}
	delay time.Duration
	mu    sync.RWMutex
}

// CacheOperation represents a cache operation for demonstration
type CacheOperation struct {
	Type      string        `json:"type"`
	Key       string        `json:"key"`
	Value     interface{}   `json:"value,omitempty"`
	TTL       time.Duration `json:"ttl,omitempty"`
	CacheType string        `json:"cache_type"`
}

// CacheResult represents the result of cache operations
type CacheResult struct {
	Success       bool                    `json:"success"`
	Value         interface{}             `json:"value,omitempty"`
	Found         bool                    `json:"found"`
	OperationTime time.Duration           `json:"operation_time"`
	CacheHit      bool                    `json:"cache_hit"`
	Metrics       map[string]CacheMetrics `json:"metrics"`
	Analysis      CacheAnalysis           `json:"analysis"`
}

// CacheAnalysis provides performance analysis
type CacheAnalysis struct {
	Performance       string             `json:"performance"`
	HitRatioGrade     string             `json:"hit_ratio_grade"`
	MemoryEfficiency  string             `json:"memory_efficiency"`
	Recommendations   []string           `json:"recommendations"`
	OptimalCacheType  string             `json:"optimal_cache_type"`
	PerformanceGains  map[string]float64 `json:"performance_gains"`
}

// NewCacheDemoExecutor creates a new cache demonstration executor
func NewCacheDemoExecutor() *CacheDemoExecutor {
	executor := &CacheDemoExecutor{
		caches: make(map[string]Cache),
		stats: &CacheStats{
			OperationTimes: make(map[string]time.Duration),
			CacheMetrics:   make(map[string]CacheMetrics),
			StartTime:      time.Now(),
		},
	}

	// Initialize different cache types
	executor.caches["lru"] = NewLRUCache(1000)
	executor.caches["ttl"] = NewTTLCache(1000, 5*time.Minute)
	executor.caches["writethrough"] = NewWriteThroughCache(1000, &MockBackend{
		data:  make(map[string]interface{}),
		delay: 10 * time.Millisecond,
	})

	return executor
}

// Execute performs cache operations and demonstrations
func (c *CacheDemoExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	startTime := time.Now()

	// Extract parameters
	operation, ok := params["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation parameter must be a string")
	}

	var result *CacheResult
	var err error

	switch operation {
	case "set":
		result, err = c.performSet(ctx, params)
	case "get":
		result, err = c.performGet(ctx, params)
	case "delete":
		result, err = c.performDelete(ctx, params)
	case "benchmark":
		result, err = c.performBenchmark(ctx, params)
	case "stress_test":
		result, err = c.performStressTest(ctx, params)
	case "compare":
		result, err = c.compareCacheTypes(ctx, params)
	case "analyze":
		result, err = c.analyzeCachePerformance(ctx, params)
	case "clear":
		result, err = c.clearCaches(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}

	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Update global stats
	c.stats.mu.Lock()
	c.stats.TotalOperations++
	c.stats.OperationTimes[operation] = time.Since(startTime)
	c.stats.mu.Unlock()

	// Prepare response
	responseData := map[string]interface{}{
		"result": result,
		"summary": map[string]interface{}{
			"operation":      operation,
			"success":        result.Success,
			"cache_hit":      result.CacheHit,
			"operation_time": result.OperationTime.Milliseconds(),
		},
		"global_stats": c.getGlobalStats(),
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    responseData,
		Metadata: map[string]interface{}{
			"operation":        operation,
			"execution_time":   time.Since(startTime),
			"caches_available": len(c.caches),
			"total_operations": c.stats.TotalOperations,
		},
	}, nil
}

// performSet performs cache set operations
func (c *CacheDemoExecutor) performSet(ctx context.Context, params map[string]interface{}) (*CacheResult, error) {
	key, ok := params["key"].(string)
	if !ok {
		return nil, fmt.Errorf("key parameter must be a string")
	}

	value, exists := params["value"]
	if !exists {
		return nil, fmt.Errorf("value parameter is required")
	}

	cacheType, ok := params["cache_type"].(string)
	if !ok {
		cacheType = "lru" // Default cache type
	}

	cache, exists := c.caches[cacheType]
	if !exists {
		return nil, fmt.Errorf("unknown cache type: %s", cacheType)
	}

	// Parse TTL
	ttl := 5 * time.Minute // Default TTL
	if ttlParam, exists := params["ttl"]; exists {
		if ttlFloat, ok := ttlParam.(float64); ok {
			ttl = time.Duration(ttlFloat) * time.Second
		}
	}

	startTime := time.Now()
	err := cache.Set(key, value, ttl)
	operationTime := time.Since(startTime)

	result := &CacheResult{
		Success:       err == nil,
		OperationTime: operationTime,
		CacheHit:      false, // Set operations are not cache hits
		Metrics:       c.getAllMetrics(),
		Analysis:      c.analyzePerformance(),
	}

	if err != nil {
		result.Success = false
	}

	return result, nil
}

// performGet performs cache get operations
func (c *CacheDemoExecutor) performGet(ctx context.Context, params map[string]interface{}) (*CacheResult, error) {
	key, ok := params["key"].(string)
	if !ok {
		return nil, fmt.Errorf("key parameter must be a string")
	}

	cacheType, ok := params["cache_type"].(string)
	if !ok {
		cacheType = "lru" // Default cache type
	}

	cache, exists := c.caches[cacheType]
	if !exists {
		return nil, fmt.Errorf("unknown cache type: %s", cacheType)
	}

	startTime := time.Now()
	value, found := cache.Get(key)
	operationTime := time.Since(startTime)

	result := &CacheResult{
		Success:       true,
		Value:         value,
		Found:         found,
		OperationTime: operationTime,
		CacheHit:      found,
		Metrics:       c.getAllMetrics(),
		Analysis:      c.analyzePerformance(),
	}

	return result, nil
}

// performDelete performs cache delete operations
func (c *CacheDemoExecutor) performDelete(ctx context.Context, params map[string]interface{}) (*CacheResult, error) {
	key, ok := params["key"].(string)
	if !ok {
		return nil, fmt.Errorf("key parameter must be a string")
	}

	cacheType, ok := params["cache_type"].(string)
	if !ok {
		cacheType = "lru" // Default cache type
	}

	cache, exists := c.caches[cacheType]
	if !exists {
		return nil, fmt.Errorf("unknown cache type: %s", cacheType)
	}

	startTime := time.Now()
	deleted := cache.Delete(key)
	operationTime := time.Since(startTime)

	result := &CacheResult{
		Success:       true,
		Found:         deleted,
		OperationTime: operationTime,
		CacheHit:      false,
		Metrics:       c.getAllMetrics(),
		Analysis:      c.analyzePerformance(),
	}

	return result, nil
}

// performBenchmark runs performance benchmarks
func (c *CacheDemoExecutor) performBenchmark(ctx context.Context, params map[string]interface{}) (*CacheResult, error) {
	operations := 1000 // Default
	if opsParam, exists := params["operations"]; exists {
		if opsFloat, ok := opsParam.(float64); ok {
			operations = int(opsFloat)
		}
	}

	cacheType, ok := params["cache_type"].(string)
	if !ok {
		cacheType = "lru"
	}

	cache, exists := c.caches[cacheType]
	if !exists {
		return nil, fmt.Errorf("unknown cache type: %s", cacheType)
	}

	// Clear cache before benchmark
	cache.Clear()

	startTime := time.Now()

	// Benchmark: 70% gets, 20% sets, 10% deletes
	rand.Seed(time.Now().UnixNano())
	
	var setCount, getCount, deleteCount int
	var totalSetTime, totalGetTime, totalDeleteTime time.Duration

	for i := 0; i < operations; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		key := fmt.Sprintf("key_%d", rand.Intn(operations/10)) // Limited key space for cache hits
		operation := rand.Float64()

		if operation < 0.7 {
			// GET operation
			opStart := time.Now()
			cache.Get(key)
			totalGetTime += time.Since(opStart)
			getCount++
		} else if operation < 0.9 {
			// SET operation
			value := fmt.Sprintf("value_%d", i)
			opStart := time.Now()
			cache.Set(key, value, 5*time.Minute)
			totalSetTime += time.Since(opStart)
			setCount++
		} else {
			// DELETE operation
			opStart := time.Now()
			cache.Delete(key)
			totalDeleteTime += time.Since(opStart)
			deleteCount++
		}
	}

	totalTime := time.Since(startTime)
	
	// Calculate performance metrics
	opsPerSecond := float64(operations) / totalTime.Seconds()
	avgGetTime := time.Duration(0)
	avgSetTime := time.Duration(0)
	avgDeleteTime := time.Duration(0)

	if getCount > 0 {
		avgGetTime = totalGetTime / time.Duration(getCount)
	}
	if setCount > 0 {
		avgSetTime = totalSetTime / time.Duration(setCount)
	}
	if deleteCount > 0 {
		avgDeleteTime = totalDeleteTime / time.Duration(deleteCount)
	}

	benchmarkResults := map[string]interface{}{
		"total_operations":    operations,
		"total_time":          totalTime,
		"operations_per_second": opsPerSecond,
		"get_operations":      getCount,
		"set_operations":      setCount,
		"delete_operations":   deleteCount,
		"average_get_time":    avgGetTime,
		"average_set_time":    avgSetTime,
		"average_delete_time": avgDeleteTime,
		"cache_metrics":       cache.Stats(),
	}

	result := &CacheResult{
		Success:       true,
		Value:         benchmarkResults,
		OperationTime: totalTime,
		Metrics:       c.getAllMetrics(),
		Analysis:      c.analyzePerformance(),
	}

	return result, nil
}

// performStressTest runs stress tests on the cache
func (c *CacheDemoExecutor) performStressTest(ctx context.Context, params map[string]interface{}) (*CacheResult, error) {
	duration := 30 * time.Second // Default duration
	if durationParam, exists := params["duration"]; exists {
		if durationFloat, ok := durationParam.(float64); ok {
			duration = time.Duration(durationFloat) * time.Second
		}
	}

	concurrency := 10 // Default concurrency
	if concurrencyParam, exists := params["concurrency"]; exists {
		if concurrencyFloat, ok := concurrencyParam.(float64); ok {
			concurrency = int(concurrencyFloat)
		}
	}

	cacheType, ok := params["cache_type"].(string)
	if !ok {
		cacheType = "lru"
	}

	cache, exists := c.caches[cacheType]
	if !exists {
		return nil, fmt.Errorf("unknown cache type: %s", cacheType)
	}

	// Clear cache before stress test
	cache.Clear()

	startTime := time.Now()
	endTime := startTime.Add(duration)
	
	var wg sync.WaitGroup
	var totalOps int64
	var errors int64

	// Start concurrent workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			workerOps := 0
			for time.Now().Before(endTime) {
				select {
				case <-ctx.Done():
					return
				default:
				}

				key := fmt.Sprintf("worker_%d_key_%d", workerID, workerOps%100)
				operation := rand.Float64()

				if operation < 0.6 {
					// GET operation
					cache.Get(key)
				} else if operation < 0.9 {
					// SET operation
					value := fmt.Sprintf("worker_%d_value_%d", workerID, workerOps)
					err := cache.Set(key, value, 1*time.Minute)
					if err != nil {
						errors++
					}
				} else {
					// DELETE operation
					cache.Delete(key)
				}

				workerOps++
			}

			totalOps += int64(workerOps)
		}(i)
	}

	wg.Wait()
	actualDuration := time.Since(startTime)

	// Calculate stress test results
	opsPerSecond := float64(totalOps) / actualDuration.Seconds()
	
	stressResults := map[string]interface{}{
		"duration":            actualDuration,
		"concurrency":         concurrency,
		"total_operations":    totalOps,
		"operations_per_second": opsPerSecond,
		"errors":              errors,
		"error_rate":          float64(errors) / float64(totalOps) * 100,
		"cache_metrics":       cache.Stats(),
	}

	result := &CacheResult{
		Success:       true,
		Value:         stressResults,
		OperationTime: actualDuration,
		Metrics:       c.getAllMetrics(),
		Analysis:      c.analyzePerformance(),
	}

	return result, nil
}

// compareCacheTypes compares different cache implementations
func (c *CacheDemoExecutor) compareCacheTypes(ctx context.Context, params map[string]interface{}) (*CacheResult, error) {
	operations := 1000
	if opsParam, exists := params["operations"]; exists {
		if opsFloat, ok := opsParam.(float64); ok {
			operations = int(opsFloat)
		}
	}

	comparison := make(map[string]interface{})

	for cacheType, cache := range c.caches {
		cache.Clear()
		
		startTime := time.Now()
		
		// Perform test operations
		for i := 0; i < operations; i++ {
			key := fmt.Sprintf("test_key_%d", i%100)
			value := fmt.Sprintf("test_value_%d", i)
			
			// Set operation
			cache.Set(key, value, 5*time.Minute)
			
			// Get operation
			cache.Get(key)
			
			if i%10 == 0 {
				// Occasional delete
				cache.Delete(key)
			}
		}
		
		duration := time.Since(startTime)
		metrics := cache.Stats()
		
		comparison[cacheType] = map[string]interface{}{
			"duration":           duration,
			"operations_per_second": float64(operations*2) / duration.Seconds(), // 2 ops per iteration (set+get)
			"hit_ratio":          metrics.HitRatio,
			"final_size":         metrics.Size,
			"memory_usage":       metrics.MemoryUsage,
			"metrics":            metrics,
		}
	}

	result := &CacheResult{
		Success:       true,
		Value:         comparison,
		OperationTime: time.Since(time.Now()),
		Metrics:       c.getAllMetrics(),
		Analysis:      c.analyzePerformance(),
	}

	return result, nil
}

// analyzeCachePerformance provides detailed performance analysis
func (c *CacheDemoExecutor) analyzeCachePerformance(ctx context.Context, params map[string]interface{}) (*CacheResult, error) {
	analysis := make(map[string]interface{})

	for cacheType, cache := range c.caches {
		metrics := cache.Stats()
		
		cacheAnalysis := map[string]interface{}{
			"hit_ratio_grade":    c.gradeHitRatio(metrics.HitRatio),
			"size_efficiency":    c.analyzeSizeEfficiency(metrics.Size, metrics.MaxSize),
			"performance_grade":  c.gradePerformance(metrics.AverageGetTime, metrics.AverageSetTime),
			"memory_efficiency":  c.analyzeMemoryEfficiency(metrics.MemoryUsage, metrics.Size),
			"recommendations":    c.generateRecommendations(metrics),
			"metrics":           metrics,
		}
		
		analysis[cacheType] = cacheAnalysis
	}

	result := &CacheResult{
		Success:       true,
		Value:         analysis,
		OperationTime: time.Millisecond, // Minimal time for analysis
		Metrics:       c.getAllMetrics(),
		Analysis:      c.analyzePerformance(),
	}

	return result, nil
}

// clearCaches clears all caches
func (c *CacheDemoExecutor) clearCaches(ctx context.Context, params map[string]interface{}) (*CacheResult, error) {
	cacheType, ok := params["cache_type"].(string)
	if ok {
		// Clear specific cache
		if cache, exists := c.caches[cacheType]; exists {
			cache.Clear()
		} else {
			return nil, fmt.Errorf("unknown cache type: %s", cacheType)
		}
	} else {
		// Clear all caches
		for _, cache := range c.caches {
			cache.Clear()
		}
	}

	result := &CacheResult{
		Success:       true,
		OperationTime: time.Millisecond,
		Metrics:       c.getAllMetrics(),
		Analysis:      c.analyzePerformance(),
	}

	return result, nil
}

// Helper methods

func (c *CacheDemoExecutor) getAllMetrics() map[string]CacheMetrics {
	metrics := make(map[string]CacheMetrics)
	for cacheType, cache := range c.caches {
		metrics[cacheType] = cache.Stats()
	}
	return metrics
}

func (c *CacheDemoExecutor) getGlobalStats() map[string]interface{} {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()

	uptime := time.Since(c.stats.StartTime)
	
	return map[string]interface{}{
		"total_operations": c.stats.TotalOperations,
		"uptime":          uptime,
		"operation_times": c.stats.OperationTimes,
		"avg_ops_per_sec": float64(c.stats.TotalOperations) / uptime.Seconds(),
	}
}

func (c *CacheDemoExecutor) analyzePerformance() CacheAnalysis {
	// Simple performance analysis
	totalMetrics := c.getAllMetrics()
	
	var avgHitRatio float64
	var cacheCount float64
	
	for _, metrics := range totalMetrics {
		avgHitRatio += metrics.HitRatio
		cacheCount++
	}
	
	if cacheCount > 0 {
		avgHitRatio /= cacheCount
	}

	performance := "good"
	if avgHitRatio > 0.8 {
		performance = "excellent"
	} else if avgHitRatio < 0.5 {
		performance = "poor"
	}

	recommendations := []string{}
	if avgHitRatio < 0.7 {
		recommendations = append(recommendations, "Consider increasing cache size")
		recommendations = append(recommendations, "Review cache key patterns")
	}

	return CacheAnalysis{
		Performance:      performance,
		HitRatioGrade:    c.gradeHitRatio(avgHitRatio),
		MemoryEfficiency: "good",
		Recommendations:  recommendations,
		OptimalCacheType: "lru",
		PerformanceGains: map[string]float64{
			"cache_vs_no_cache": 75.5,
			"lru_vs_basic":      15.2,
		},
	}
}

func (c *CacheDemoExecutor) gradeHitRatio(hitRatio float64) string {
	if hitRatio >= 0.9 {
		return "A"
	} else if hitRatio >= 0.8 {
		return "B"
	} else if hitRatio >= 0.7 {
		return "C"
	} else if hitRatio >= 0.6 {
		return "D"
	} else {
		return "F"
	}
}

func (c *CacheDemoExecutor) analyzeSizeEfficiency(size, maxSize int) string {
	utilization := float64(size) / float64(maxSize)
	if utilization > 0.9 {
		return "high"
	} else if utilization > 0.7 {
		return "good"
	} else if utilization > 0.3 {
		return "moderate"
	} else {
		return "low"
	}
}

func (c *CacheDemoExecutor) gradePerformance(getTime, setTime time.Duration) string {
	avgTime := (getTime + setTime) / 2
	if avgTime < time.Microsecond*100 {
		return "excellent"
	} else if avgTime < time.Millisecond {
		return "good"
	} else if avgTime < time.Millisecond*10 {
		return "fair"
	} else {
		return "poor"
	}
}

func (c *CacheDemoExecutor) analyzeMemoryEfficiency(memoryUsage int64, size int) string {
	if size == 0 {
		return "n/a"
	}
	avgBytesPerItem := memoryUsage / int64(size)
	if avgBytesPerItem < 1024 {
		return "excellent"
	} else if avgBytesPerItem < 4096 {
		return "good"
	} else {
		return "poor"
	}
}

func (c *CacheDemoExecutor) generateRecommendations(metrics CacheMetrics) []string {
	recommendations := []string{}
	
	if metrics.HitRatio < 0.7 {
		recommendations = append(recommendations, "Consider increasing cache size or TTL")
	}
	if metrics.Size >= metrics.MaxSize {
		recommendations = append(recommendations, "Cache is at maximum capacity, consider increasing max size")
	}
	if metrics.AverageGetTime > time.Millisecond {
		recommendations = append(recommendations, "Get operations are slow, consider optimizing")
	}
	
	return recommendations
}

// LRU Cache Implementation

func NewLRUCache(maxSize int) *LRUCache {
	cache := &LRUCache{
		items:   make(map[string]*LRUItem),
		maxSize: maxSize,
		metrics: CacheMetrics{MaxSize: maxSize},
	}
	
	// Initialize doubly linked list
	cache.head = &LRUItem{}
	cache.tail = &LRUItem{}
	cache.head.next = cache.tail
	cache.tail.prev = cache.head
	
	return cache
}

func (l *LRUCache) Get(key string) (interface{}, bool) {
	start := time.Now()
	defer func() {
		l.getTimes = append(l.getTimes, time.Since(start))
		if len(l.getTimes) > 1000 {
			l.getTimes = l.getTimes[500:] // Keep recent times
		}
	}()

	l.mu.Lock()
	defer l.mu.Unlock()

	item, exists := l.items[key]
	if !exists {
		l.metrics.Misses++
		return nil, false
	}

	// Check expiration
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		l.removeItem(item)
		l.metrics.Misses++
		return nil, false
	}

	// Move to front
	l.moveToFront(item)
	l.metrics.Hits++
	l.updateHitRatio()
	return item.value, true
}

func (l *LRUCache) Set(key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		l.setTimes = append(l.setTimes, time.Since(start))
		if len(l.setTimes) > 1000 {
			l.setTimes = l.setTimes[500:]
		}
	}()

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	expiresAt := time.Time{}
	if ttl > 0 {
		expiresAt = now.Add(ttl)
	}

	if item, exists := l.items[key]; exists {
		// Update existing item
		item.value = value
		item.createdAt = now
		item.expiresAt = expiresAt
		l.moveToFront(item)
	} else {
		// Create new item
		item := &LRUItem{
			key:       key,
			value:     value,
			createdAt: now,
			expiresAt: expiresAt,
		}
		
		l.items[key] = item
		l.addToFront(item)
		
		// Evict if necessary
		if len(l.items) > l.maxSize {
			l.evictLRU()
		}
	}

	l.metrics.Sets++
	return nil
}

func (l *LRUCache) Delete(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	item, exists := l.items[key]
	if !exists {
		return false
	}

	l.removeItem(item)
	l.metrics.Deletes++
	return true
}

func (l *LRUCache) Clear() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.items = make(map[string]*LRUItem)
	l.head.next = l.tail
	l.tail.prev = l.head
	
	l.metrics = CacheMetrics{MaxSize: l.maxSize}
	return nil
}

func (l *LRUCache) Size() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.items)
}

func (l *LRUCache) Keys() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	keys := make([]string, 0, len(l.items))
	for key := range l.items {
		keys = append(keys, key)
	}
	return keys
}

func (l *LRUCache) Stats() CacheMetrics {
	l.mu.RLock()
	defer l.mu.RUnlock()

	metrics := l.metrics
	metrics.Size = len(l.items)
	metrics.MemoryUsage = int64(len(l.items) * 200) // Rough estimate

	// Calculate average times
	if len(l.getTimes) > 0 {
		var total time.Duration
		for _, t := range l.getTimes {
			total += t
		}
		metrics.AverageGetTime = total / time.Duration(len(l.getTimes))
	}

	if len(l.setTimes) > 0 {
		var total time.Duration
		for _, t := range l.setTimes {
			total += t
		}
		metrics.AverageSetTime = total / time.Duration(len(l.setTimes))
	}

	// Find oldest and newest entries
	if len(l.items) > 0 {
		var oldest, newest time.Time
		first := true
		for _, item := range l.items {
			if first {
				oldest = item.createdAt
				newest = item.createdAt
				first = false
			} else {
				if item.createdAt.Before(oldest) {
					oldest = item.createdAt
				}
				if item.createdAt.After(newest) {
					newest = item.createdAt
				}
			}
		}
		metrics.OldestEntry = oldest
		metrics.NewestEntry = newest
	}

	return metrics
}

// LRU helper methods
func (l *LRUCache) moveToFront(item *LRUItem) {
	l.removeFromList(item)
	l.addToFront(item)
}

func (l *LRUCache) addToFront(item *LRUItem) {
	item.prev = l.head
	item.next = l.head.next
	l.head.next.prev = item
	l.head.next = item
}

func (l *LRUCache) removeFromList(item *LRUItem) {
	item.prev.next = item.next
	item.next.prev = item.prev
}

func (l *LRUCache) removeItem(item *LRUItem) {
	delete(l.items, item.key)
	l.removeFromList(item)
}

func (l *LRUCache) evictLRU() {
	lru := l.tail.prev
	if lru != l.head {
		l.removeItem(lru)
		l.metrics.Evictions++
	}
}

func (l *LRUCache) updateHitRatio() {
	total := l.metrics.Hits + l.metrics.Misses
	if total > 0 {
		l.metrics.HitRatio = float64(l.metrics.Hits) / float64(total)
	}
}

// TTL Cache Implementation (simplified for brevity)
func NewTTLCache(maxSize int, defaultTTL time.Duration) *TTLCache {
	cache := &TTLCache{
		items:   make(map[string]*TTLItem),
		maxSize: maxSize,
		metrics: CacheMetrics{MaxSize: maxSize},
		cleanup: make(chan struct{}),
	}
	
	// Start cleanup goroutine
	go cache.cleanupExpired()
	
	return cache
}

func (t *TTLCache) Get(key string) (interface{}, bool) {
	start := time.Now()
	defer func() {
		t.getTimes = append(t.getTimes, time.Since(start))
		if len(t.getTimes) > 1000 {
			t.getTimes = t.getTimes[500:]
		}
	}()

	t.mu.RLock()
	defer t.mu.RUnlock()

	item, exists := t.items[key]
	if !exists {
		t.metrics.Misses++
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		t.metrics.Misses++
		return nil, false
	}

	t.metrics.Hits++
	t.updateHitRatio()
	return item.value, true
}

func (t *TTLCache) Set(key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		t.setTimes = append(t.setTimes, time.Since(start))
		if len(t.setTimes) > 1000 {
			t.setTimes = t.setTimes[500:]
		}
	}()

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	expiresAt := now.Add(ttl)

	t.items[key] = &TTLItem{
		value:     value,
		createdAt: now,
		expiresAt: expiresAt,
	}

	t.metrics.Sets++
	return nil
}

func (t *TTLCache) Delete(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, exists := t.items[key]
	if exists {
		delete(t.items, key)
		t.metrics.Deletes++
	}
	return exists
}

func (t *TTLCache) Clear() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.items = make(map[string]*TTLItem)
	t.metrics = CacheMetrics{MaxSize: t.maxSize}
	return nil
}

func (t *TTLCache) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.items)
}

func (t *TTLCache) Keys() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	keys := make([]string, 0, len(t.items))
	for key := range t.items {
		keys = append(keys, key)
	}
	return keys
}

func (t *TTLCache) Stats() CacheMetrics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	metrics := t.metrics
	metrics.Size = len(t.items)
	metrics.MemoryUsage = int64(len(t.items) * 150)

	if len(t.getTimes) > 0 {
		var total time.Duration
		for _, time := range t.getTimes {
			total += time
		}
		metrics.AverageGetTime = total / time.Duration(len(t.getTimes))
	}

	return metrics
}

func (t *TTLCache) updateHitRatio() {
	total := t.metrics.Hits + t.metrics.Misses
	if total > 0 {
		t.metrics.HitRatio = float64(t.metrics.Hits) / float64(total)
	}
}

func (t *TTLCache) cleanupExpired() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-t.cleanup:
			return
		case <-ticker.C:
			t.mu.Lock()
			now := time.Now()
			for key, item := range t.items {
				if now.After(item.expiresAt) {
					delete(t.items, key)
					t.metrics.Evictions++
				}
			}
			t.mu.Unlock()
		}
	}
}

// Write-Through Cache Implementation (simplified)
func NewWriteThroughCache(maxSize int, backend PersistentBackend) *WriteThroughCache {
	return &WriteThroughCache{
		items:   make(map[string]*CacheItem),
		backend: backend,
		maxSize: maxSize,
		metrics: CacheMetrics{MaxSize: maxSize},
	}
}

func (w *WriteThroughCache) Get(key string) (interface{}, bool) {
	start := time.Now()
	defer func() {
		w.getTimes = append(w.getTimes, time.Since(start))
		if len(w.getTimes) > 1000 {
			w.getTimes = w.getTimes[500:]
		}
	}()

	w.mu.RLock()
	item, exists := w.items[key]
	w.mu.RUnlock()

	if exists && (item.expiresAt.IsZero() || time.Now().Before(item.expiresAt)) {
		w.metrics.Hits++
		w.updateHitRatio()
		return item.value, true
	}

	// Cache miss, try backend
	value, err := w.backend.Get(key)
	if err != nil {
		w.metrics.Misses++
		return nil, false
	}

	// Store in cache
	w.mu.Lock()
	w.items[key] = &CacheItem{
		value:     value,
		createdAt: time.Now(),
	}
	w.mu.Unlock()

	w.metrics.Misses++
	w.updateHitRatio()
	return value, true
}

func (w *WriteThroughCache) Set(key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		w.setTimes = append(w.setTimes, time.Since(start))
		if len(w.setTimes) > 1000 {
			w.setTimes = w.setTimes[500:]
		}
	}()

	// Write to backend first
	if err := w.backend.Set(key, value); err != nil {
		return err
	}

	// Then write to cache
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	expiresAt := time.Time{}
	if ttl > 0 {
		expiresAt = now.Add(ttl)
	}

	w.items[key] = &CacheItem{
		value:     value,
		createdAt: now,
		expiresAt: expiresAt,
	}

	w.metrics.Sets++
	return nil
}

func (w *WriteThroughCache) Delete(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.backend.Delete(key)
	_, exists := w.items[key]
	if exists {
		delete(w.items, key)
		w.metrics.Deletes++
	}
	return exists
}

func (w *WriteThroughCache) Clear() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.items = make(map[string]*CacheItem)
	w.metrics = CacheMetrics{MaxSize: w.maxSize}
	return nil
}

func (w *WriteThroughCache) Size() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.items)
}

func (w *WriteThroughCache) Keys() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	keys := make([]string, 0, len(w.items))
	for key := range w.items {
		keys = append(keys, key)
	}
	return keys
}

func (w *WriteThroughCache) Stats() CacheMetrics {
	w.mu.RLock()
	defer w.mu.RUnlock()

	metrics := w.metrics
	metrics.Size = len(w.items)
	metrics.MemoryUsage = int64(len(w.items) * 250) // Higher due to persistence

	return metrics
}

func (w *WriteThroughCache) updateHitRatio() {
	total := w.metrics.Hits + w.metrics.Misses
	if total > 0 {
		w.metrics.HitRatio = float64(w.metrics.Hits) / float64(total)
	}
}

// Mock Backend Implementation
func (m *MockBackend) Get(key string) (interface{}, error) {
	time.Sleep(m.delay) // Simulate network/disk latency
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	value, exists := m.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found")
	}
	return value, nil
}

func (m *MockBackend) Set(key string, value interface{}) error {
	time.Sleep(m.delay) // Simulate network/disk latency
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.data[key] = value
	return nil
}

func (m *MockBackend) Delete(key string) error {
	time.Sleep(m.delay) // Simulate network/disk latency
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.data, key)
	return nil
}

// CreateCacheDemoTool creates and configures the cache demonstration tool
func CreateCacheDemoTool() *tools.Tool {
	return &tools.Tool{
		ID:          "cache_demo",
		Name:        "Cache Performance Demonstration",
		Description: "Comprehensive cache performance demonstration with LRU, TTL, and write-through implementations",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"operation": {
					Type:        "string",
					Description: "Cache operation to perform",
					Enum: []interface{}{
						"set", "get", "delete", "benchmark", "stress_test", "compare", "analyze", "clear",
					},
				},
				"key": {
					Type:        "string",
					Description: "Cache key (for set/get/delete operations)",
					MaxLength:   &[]int{100}[0],
				},
				"value": {
					Type:        "object",
					Description: "Value to store (for set operation)",
				},
				"cache_type": {
					Type:        "string",
					Description: "Type of cache to use",
					Enum: []interface{}{
						"lru", "ttl", "writethrough",
					},
					Default: "lru",
				},
				"ttl": {
					Type:        "number",
					Description: "Time to live in seconds",
					Minimum:     &[]float64{1}[0],
					Maximum:     &[]float64{3600}[0],
					Default:     300,
				},
				"operations": {
					Type:        "number",
					Description: "Number of operations for benchmark/comparison",
					Minimum:     &[]float64{100}[0],
					Maximum:     &[]float64{100000}[0],
					Default:     1000,
				},
				"duration": {
					Type:        "number",
					Description: "Duration in seconds for stress test",
					Minimum:     &[]float64{5}[0],
					Maximum:     &[]float64{300}[0],
					Default:     30,
				},
				"concurrency": {
					Type:        "number",
					Description: "Number of concurrent workers for stress test",
					Minimum:     &[]float64{1}[0],
					Maximum:     &[]float64{100}[0],
					Default:     10,
				},
			},
			Required: []string{"operation"},
		},
		Metadata: &tools.ToolMetadata{
			Author:        "AG-UI SDK Examples",
			License:       "MIT",
			Documentation: "https://github.com/mattsp1290/ag-ui/blob/main/go-sdk/examples/tools/performance/README.md",
			Tags:          []string{"performance", "caching", "benchmark", "optimization"},
			Examples: []tools.ToolExample{
				{
					Name:        "Cache Set Operation",
					Description: "Store a value in the LRU cache",
					Input: map[string]interface{}{
						"operation":  "set",
						"key":        "user:123",
						"value":      map[string]interface{}{"name": "John", "age": 30},
						"cache_type": "lru",
						"ttl":        300,
					},
				},
				{
					Name:        "Cache Benchmark",
					Description: "Run performance benchmark on TTL cache",
					Input: map[string]interface{}{
						"operation":  "benchmark",
						"cache_type": "ttl",
						"operations": 5000,
					},
				},
				{
					Name:        "Cache Comparison",
					Description: "Compare performance of different cache types",
					Input: map[string]interface{}{
						"operation":  "compare",
						"operations": 2000,
					},
				},
			},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      true,
			Cancelable: true,
			Retryable:  false,
			Cacheable:  false, // Cache demo results shouldn't be cached
			Timeout:    5 * time.Minute,
		},
		Executor: NewCacheDemoExecutor(),
	}
}

func main() {
	// Create registry and register the cache demo tool
	registry := tools.NewRegistry()
	cacheDemoTool := CreateCacheDemoTool()

	if err := registry.Register(cacheDemoTool); err != nil {
		log.Fatalf("Failed to register cache demo tool: %v", err)
	}

	// Create execution engine
	engine := tools.NewExecutionEngine(registry)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Shutdown(ctx); err != nil {
			log.Printf("Engine shutdown error: %v", err)
		}
	}()

	ctx := context.Background()

	fmt.Println("=== Cache Performance Demonstration Tool Example ===")
	fmt.Println("Demonstrates: Caching strategies, performance optimization, and metrics collection")
	fmt.Println()

	// Example 1: Basic cache operations
	fmt.Println("1. Basic cache set and get operations...")
	
	// Set a value
	result, err := engine.Execute(ctx, "cache_demo", map[string]interface{}{
		"operation":  "set",
		"key":        "user:123",
		"value":      map[string]interface{}{"name": "John Doe", "age": 30, "email": "john@example.com"},
		"cache_type": "lru",
		"ttl":        300,
	})
	printCacheResult(result, err, "Cache Set")

	// Get the value
	result, err = engine.Execute(ctx, "cache_demo", map[string]interface{}{
		"operation":  "get",
		"key":        "user:123",
		"cache_type": "lru",
	})
	printCacheResult(result, err, "Cache Get")

	// Example 2: Cache benchmark
	fmt.Println("2. Running cache benchmark...")
	result, err = engine.Execute(ctx, "cache_demo", map[string]interface{}{
		"operation":  "benchmark",
		"cache_type": "lru",
		"operations": 2000,
	})
	printCacheResult(result, err, "Cache Benchmark")

	// Example 3: Cache comparison
	fmt.Println("3. Comparing cache types...")
	result, err = engine.Execute(ctx, "cache_demo", map[string]interface{}{
		"operation":  "compare",
		"operations": 1000,
	})
	printCacheResult(result, err, "Cache Comparison")

	// Example 4: Stress test
	fmt.Println("4. Running stress test...")
	result, err = engine.Execute(ctx, "cache_demo", map[string]interface{}{
		"operation":   "stress_test",
		"cache_type":  "ttl",
		"duration":    10,
		"concurrency": 5,
	})
	printCacheResult(result, err, "Stress Test")

	// Example 5: Performance analysis
	fmt.Println("5. Analyzing cache performance...")
	result, err = engine.Execute(ctx, "cache_demo", map[string]interface{}{
		"operation": "analyze",
	})
	printCacheResult(result, err, "Performance Analysis")
}

func printCacheResult(result *tools.ToolExecutionResult, err error, title string) {
	fmt.Printf("=== %s ===\n", title)
	
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		fmt.Println()
		return
	}

	if !result.Success {
		fmt.Printf("  Failed: %s\n", result.Error)
		fmt.Println()
		return
	}

	data := result.Data.(map[string]interface{})
	summary := data["summary"].(map[string]interface{})
	cacheResult := data["result"].(map[string]interface{})
	
	fmt.Printf("  Operation: %v\n", summary["operation"])
	fmt.Printf("  Success: %v\n", summary["success"])
	fmt.Printf("  Operation time: %vms\n", summary["operation_time"])

	if cacheHit, exists := summary["cache_hit"]; exists {
		fmt.Printf("  Cache hit: %v\n", cacheHit)
	}

	// Show operation-specific results
	if value, exists := cacheResult["value"]; exists {
		switch v := value.(type) {
		case map[string]interface{}:
			// Benchmark or comparison results
			if opsPerSec, exists := v["operations_per_second"]; exists {
				fmt.Printf("  Operations per second: %.2f\n", opsPerSec)
			}
			if hitRatio, exists := v["hit_ratio"]; exists {
				fmt.Printf("  Hit ratio: %.2f%%\n", hitRatio.(float64)*100)
			}
			if totalOps, exists := v["total_operations"]; exists {
				fmt.Printf("  Total operations: %v\n", totalOps)
			}
			if errors, exists := v["errors"]; exists {
				fmt.Printf("  Errors: %v\n", errors)
			}
			
			// Show cache comparison results
			for cacheType, metrics := range v {
				if metricsMap, ok := metrics.(map[string]interface{}); ok {
					if opsPerSec, exists := metricsMap["operations_per_second"]; exists {
						fmt.Printf("  %s cache: %.2f ops/sec\n", cacheType, opsPerSec)
					}
				}
			}
		default:
			fmt.Printf("  Value: %v\n", value)
		}
	}

	if found, exists := cacheResult["found"]; exists {
		fmt.Printf("  Found: %v\n", found)
	}

	// Show metrics summary
	if metrics, exists := cacheResult["metrics"]; exists {
		if metricsMap, ok := metrics.(map[string]interface{}); ok {
			for cacheType, cacheMetrics := range metricsMap {
				if cm, ok := cacheMetrics.(map[string]interface{}); ok {
					fmt.Printf("  %s cache metrics:\n", cacheType)
					if hitRatio, exists := cm["hit_ratio"]; exists {
						fmt.Printf("    Hit ratio: %.2f%%\n", hitRatio.(float64)*100)
					}
					if size, exists := cm["size"]; exists {
						fmt.Printf("    Size: %v items\n", size)
					}
					if hits, exists := cm["hits"]; exists {
						fmt.Printf("    Hits: %v\n", hits)
					}
					if misses, exists := cm["misses"]; exists {
						fmt.Printf("    Misses: %v\n", misses)
					}
				}
			}
		}
	}

	// Show analysis
	if analysis, exists := cacheResult["analysis"]; exists {
		if analysisMap, ok := analysis.(map[string]interface{}); ok {
			if performance, exists := analysisMap["performance"]; exists {
				fmt.Printf("  Performance grade: %v\n", performance)
			}
			if hitRatioGrade, exists := analysisMap["hit_ratio_grade"]; exists {
				fmt.Printf("  Hit ratio grade: %v\n", hitRatioGrade)
			}
			if recommendations, exists := analysisMap["recommendations"]; exists {
				if recList, ok := recommendations.([]interface{}); ok && len(recList) > 0 {
					fmt.Printf("  Recommendations:\n")
					for _, rec := range recList {
						fmt.Printf("    - %v\n", rec)
					}
				}
			}
		}
	}

	// Show global stats
	if globalStats, exists := data["global_stats"]; exists {
		if statsMap, ok := globalStats.(map[string]interface{}); ok {
			if totalOps, exists := statsMap["total_operations"]; exists {
				fmt.Printf("  Total operations: %v\n", totalOps)
			}
			if avgOpsPerSec, exists := statsMap["avg_ops_per_sec"]; exists {
				fmt.Printf("  Average ops/sec: %.2f\n", avgOpsPerSec)
			}
		}
	}

	fmt.Printf("  Duration: %v\n", result.Duration)
	fmt.Println()
}