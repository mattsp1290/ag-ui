package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/stretchr/testify/assert"
	_ "github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Test basic cache validator creation and operations
func TestCacheValidator_BasicOperations(t *testing.T) {
	config := DefaultCacheValidatorConfig()
	config.L2Enabled = false // Disable L2 for basic test
	
	cv, err := NewCacheValidator(config)
	if err != nil {
		t.Fatalf("Failed to create cache validator: %v", err)
	}
	defer cv.Shutdown(context.Background())
	
	ctx := context.Background()
	
	// Test with a real event
	event := events.NewRunStartedEvent("thread-123", "run-456")
	
	// First validation should work
	err = cv.ValidateEvent(ctx, event)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	
	// Second validation should also work (cached)
	err = cv.ValidateEvent(ctx, event)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	
	// Verify stats are being collected
	stats := cv.GetStats()
	if stats.TotalHits+stats.TotalMisses == 0 {
		t.Error("Expected some cache activity")
	}
}

func TestTTLStrategy(t *testing.T) {
	strategy := NewTTLStrategy(5 * time.Minute)
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	result := ValidationResult{
		Valid:          true,
		ValidationTime: 10 * time.Millisecond,
		EventSize:      1024,
	}
	
	// Should cache all results
	if !strategy.ShouldCache(key, result) {
		t.Error("Expected strategy to cache result")
	}
	
	// Initial TTL
	ttl := strategy.GetTTL(key)
	if ttl != 5*time.Minute {
		t.Errorf("Expected 5 minute TTL, got: %v", ttl)
	}
}

func TestMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector(DefaultMetricsConfig())
	defer collector.Shutdown(context.Background())
	
	// Record some operations
	collector.RecordHit(L1Cache, 1*time.Millisecond)
	collector.RecordMiss(5*time.Millisecond)
	collector.UpdateSize(50, 100)
	
	report := collector.GetReport()
	
	if report.BasicMetrics.Hits != 1 {
		t.Errorf("Expected 1 hit, got: %d", report.BasicMetrics.Hits)
	}
	
	if report.BasicMetrics.Misses != 1 {
		t.Errorf("Expected 1 miss, got: %d", report.BasicMetrics.Misses)
	}
	
	if report.SizeMetrics.CurrentSize != 50 {
		t.Errorf("Expected current size 50, got: %d", report.SizeMetrics.CurrentSize)
	}
	
	if report.SizeMetrics.MaxSize != 100 {
		t.Errorf("Expected max size 100, got: %d", report.SizeMetrics.MaxSize)
	}
}

// CacheTestSuite provides a test suite for comprehensive cache testing
type CacheTestSuite struct {
	suite.Suite
	cacheValidator *CacheValidator
	mockL2Cache    *MockDistributedCache
	ctx            context.Context
	cancel         context.CancelFunc
}

// MockDistributedCache implements DistributedCache interface for testing
type MockDistributedCache struct {
	mu    sync.RWMutex
	data  map[string][]byte
	ttls  map[string]time.Time
	error bool
}

func NewMockDistributedCache() *MockDistributedCache {
	return &MockDistributedCache{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Time),
	}
}

func (m *MockDistributedCache) Get(ctx context.Context, key string) ([]byte, error) {
	if m.error {
		return nil, assert.AnError
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Check TTL
	if expires, exists := m.ttls[key]; exists && time.Now().After(expires) {
		return nil, assert.AnError
	}
	
	data, exists := m.data[key]
	if !exists {
		return nil, assert.AnError
	}
	
	return data, nil
}

func (m *MockDistributedCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if m.error {
		return assert.AnError
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.data[key] = value
	m.ttls[key] = time.Now().Add(ttl)
	return nil
}

func (m *MockDistributedCache) Delete(ctx context.Context, key string) error {
	if m.error {
		return assert.AnError
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.data, key)
	delete(m.ttls, key)
	return nil
}

func (m *MockDistributedCache) Exists(ctx context.Context, key string) (bool, error) {
	if m.error {
		return false, assert.AnError
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	_, exists := m.data[key]
	return exists, nil
}

func (m *MockDistributedCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if m.error {
		return 0, assert.AnError
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	expires, exists := m.ttls[key]
	if !exists {
		return 0, assert.AnError
	}
	
	return time.Until(expires), nil
}

func (m *MockDistributedCache) Scan(ctx context.Context, pattern string) ([]string, error) {
	if m.error {
		return nil, assert.AnError
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	keys := make([]string, 0)
	for key := range m.data {
		// Simple pattern matching - just check if pattern is contained
		if pattern == "*" || contains(key, pattern) {
			keys = append(keys, key)
		}
	}
	
	return keys, nil
}

func (m *MockDistributedCache) SetError(error bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.error = error
}

func (m *MockDistributedCache) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string][]byte)
	m.ttls = make(map[string]time.Time)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || 
			(len(s) > len(substr) && 
				(s[:len(substr)] == substr || 
					s[len(s)-len(substr):] == substr || 
					(len(s) > len(substr) && s[1:len(substr)+1] == substr))))
}

func (suite *CacheTestSuite) SetupTest() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.mockL2Cache = NewMockDistributedCache()
	
	config := DefaultCacheValidatorConfig()
	config.L2Cache = suite.mockL2Cache
	config.L2Enabled = true
	config.L1Size = 100
	config.L1TTL = 1 * time.Minute
	config.L2TTL = 5 * time.Minute
	
	var err error
	suite.cacheValidator, err = NewCacheValidator(config)
	suite.Require().NoError(err)
}

func (suite *CacheTestSuite) TearDownTest() {
	if suite.cacheValidator != nil {
		suite.cacheValidator.Shutdown(suite.ctx)
	}
	if suite.cancel != nil {
		suite.cancel()
	}
}

func (suite *CacheTestSuite) TestCacheValidation() {
	event := events.NewRunStartedEvent("thread-123", "run-456")
	
	// First validation should miss cache
	err := suite.cacheValidator.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	stats := suite.cacheValidator.GetStats()
	suite.Equal(uint64(1), stats.L1Misses)
	suite.Equal(uint64(0), stats.L1Hits)
	
	// Second validation should hit L1 cache
	err = suite.cacheValidator.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	stats = suite.cacheValidator.GetStats()
	suite.Equal(uint64(1), stats.L1Hits)
}

func (suite *CacheTestSuite) TestL2CachePromotion() {
	// Clear L1 cache to test L2 promotion
	suite.cacheValidator.l1Cache.Purge()
	
	event := events.NewRunStartedEvent("thread-123", "run-456")
	
	// First validation populates both caches
	err := suite.cacheValidator.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	// Clear L1 cache
	suite.cacheValidator.l1Cache.Purge()
	
	// Second validation should hit L2 and promote to L1
	err = suite.cacheValidator.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	stats := suite.cacheValidator.GetStats()
	suite.Equal(uint64(1), stats.L2Hits)
}

func (suite *CacheTestSuite) TestCacheInvalidation() {
	event := events.NewRunStartedEvent("thread-123", "run-456")
	
	// Populate cache
	err := suite.cacheValidator.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	// Invalidate the event
	err = suite.cacheValidator.InvalidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	// Next validation should miss cache
	err = suite.cacheValidator.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	stats := suite.cacheValidator.GetStats()
	suite.Equal(uint64(2), stats.L1Misses) // Initial miss + post-invalidation miss
}

func (suite *CacheTestSuite) TestCacheInvalidationByEventType() {
	event1 := events.NewRunStartedEvent("thread-123", "run-456")
	event2 := events.NewRunStartedEvent("thread-124", "run-457")
	event3 := events.NewToolCallStartEvent("tool-123", "ToolName")
	
	// Populate cache with different events
	err := suite.cacheValidator.ValidateEvent(suite.ctx, event1)
	suite.NoError(err)
	err = suite.cacheValidator.ValidateEvent(suite.ctx, event2)
	suite.NoError(err)
	err = suite.cacheValidator.ValidateEvent(suite.ctx, event3)
	suite.NoError(err)
	
	// Invalidate all RunStarted events
	err = suite.cacheValidator.InvalidateEventType(suite.ctx, events.EventTypeRunStarted)
	suite.NoError(err)
	
	// RunStarted events should miss cache
	err = suite.cacheValidator.ValidateEvent(suite.ctx, event1)
	suite.NoError(err)
	err = suite.cacheValidator.ValidateEvent(suite.ctx, event2)
	suite.NoError(err)
	
	// ToolCallStarted event should still hit cache
	err = suite.cacheValidator.ValidateEvent(suite.ctx, event3)
	suite.NoError(err)
	
	stats := suite.cacheValidator.GetStats()
	suite.Equal(uint64(1), stats.L1Hits) // Only the tool call event
}

func (suite *CacheTestSuite) TestConcurrentCacheOperations() {
	const numGoroutines = 10
	const numOperations = 100
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				event := events.NewRunStartedEvent(
					fmt.Sprintf("thread-%d", goroutineID),
					fmt.Sprintf("run-%d", j),
				)
				
				if err := suite.cacheValidator.ValidateEvent(suite.ctx, event); err != nil {
					errors <- err
					return
				}
				
				// Randomly invalidate some events
				if j%10 == 0 {
					if err := suite.cacheValidator.InvalidateEvent(suite.ctx, event); err != nil {
						errors <- err
						return
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	for err := range errors {
		suite.Fail("Concurrent operation failed", err)
	}
	
	stats := suite.cacheValidator.GetStats()
	suite.Greater(stats.TotalHits+stats.TotalMisses, uint64(0))
}

func (suite *CacheTestSuite) TestCacheWarmup() {
	events := []events.Event{
		events.NewRunStartedEvent("thread-1", "run-1"),
		events.NewRunStartedEvent("thread-2", "run-2"),
		events.NewToolCallStartEvent("tool-1", "ToolName"),
	}
	
	err := suite.cacheValidator.Warmup(suite.ctx, events)
	suite.NoError(err)
	
	// All events should now hit cache
	for _, event := range events {
		err := suite.cacheValidator.ValidateEvent(suite.ctx, event)
		suite.NoError(err)
	}
	
	stats := suite.cacheValidator.GetStats()
	suite.Equal(uint64(len(events)), stats.L1Hits)
}

func (suite *CacheTestSuite) TestCacheSequenceValidation() {
	events := []events.Event{
		events.NewRunStartedEvent("thread-1", "run-1"),
		events.NewToolCallStartEvent("tool-1", "ToolName"),
		events.NewToolCallEndEvent("tool-1"),
		events.NewRunFinishedEvent("thread-1", "run-1"),
	}
	
	err := suite.cacheValidator.ValidateSequence(suite.ctx, events)
	suite.NoError(err)
	
	// Individual events should be cached
	for _, event := range events {
		err := suite.cacheValidator.ValidateEvent(suite.ctx, event)
		suite.NoError(err)
	}
	
	stats := suite.cacheValidator.GetStats()
	suite.Greater(stats.L1Hits, uint64(0))
}

func (suite *CacheTestSuite) TestCacheExpirationWorker() {
	// Create cache with very short TTL
	config := DefaultCacheValidatorConfig()
	config.L1TTL = 100 * time.Millisecond
	config.L2Enabled = false
	
	cv, err := NewCacheValidator(config)
	suite.Require().NoError(err)
	defer cv.Shutdown(suite.ctx)
	
	event := events.NewRunStartedEvent("thread-123", "run-456")
	
	// Populate cache
	err = cv.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	// Wait for expiration
	time.Sleep(200 * time.Millisecond)
	
	// Should miss cache after expiration
	err = cv.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	stats := cv.GetStats()
	suite.Equal(uint64(2), stats.L1Misses) // Initial miss + post-expiration miss
}

func (suite *CacheTestSuite) TestL2CacheErrors() {
	// Set L2 cache to return errors
	suite.mockL2Cache.SetError(true)
	
	event := events.NewRunStartedEvent("thread-123", "run-456")
	
	// Should still work with L1 cache only
	err := suite.cacheValidator.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	// Second call should hit L1 cache
	err = suite.cacheValidator.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	stats := suite.cacheValidator.GetStats()
	suite.Equal(uint64(1), stats.L1Hits)
	suite.Equal(uint64(0), stats.L2Hits)
}

func (suite *CacheTestSuite) TestCacheKeyGeneration() {
	event1 := events.NewRunStartedEvent("thread-123", "run-456")
	event2 := events.NewRunStartedEvent("thread-123", "run-457") // Different run
	event3 := events.NewRunStartedEvent("thread-124", "run-456") // Different thread
	
	key1, err := suite.cacheValidator.generateCacheKey(event1)
	suite.NoError(err)
	
	key2, err := suite.cacheValidator.generateCacheKey(event2)
	suite.NoError(err)
	
	key3, err := suite.cacheValidator.generateCacheKey(event3)
	suite.NoError(err)
	
	// All keys should be different
	suite.NotEqual(key1.EventHash, key2.EventHash)
	suite.NotEqual(key1.EventHash, key3.EventHash)
	suite.NotEqual(key2.EventHash, key3.EventHash)
	
	// Same event should generate same key
	key4, err := suite.cacheValidator.generateCacheKey(event1)
	suite.NoError(err)
	suite.Equal(key1.EventHash, key4.EventHash)
}

func TestCacheTestSuite(t *testing.T) {
	suite.Run(t, new(CacheTestSuite))
}

// Benchmark tests for cache performance
func BenchmarkCacheValidation(b *testing.B) {
	config := DefaultCacheValidatorConfig()
	config.L2Enabled = false
	
	cv, err := NewCacheValidator(config)
	if err != nil {
		b.Fatal(err)
	}
	defer cv.Shutdown(context.Background())
	
	event := events.NewRunStartedEvent("thread-123", "run-456")
	ctx := context.Background()
	
	// Warm up cache
	cv.ValidateEvent(ctx, event)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cv.ValidateEvent(ctx, event)
	}
}

func BenchmarkCacheConcurrentValidation(b *testing.B) {
	config := DefaultCacheValidatorConfig()
	config.L2Enabled = false
	
	cv, err := NewCacheValidator(config)
	if err != nil {
		b.Fatal(err)
	}
	defer cv.Shutdown(context.Background())
	
	event := events.NewRunStartedEvent("thread-123", "run-456")
	ctx := context.Background()
	
	// Warm up cache
	cv.ValidateEvent(ctx, event)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cv.ValidateEvent(ctx, event)
		}
	})
}

func BenchmarkCacheKeyGeneration(b *testing.B) {
	config := DefaultCacheValidatorConfig()
	cv, err := NewCacheValidator(config)
	if err != nil {
		b.Fatal(err)
	}
	defer cv.Shutdown(context.Background())
	
	event := events.NewRunStartedEvent("thread-123", "run-456")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cv.generateCacheKey(event)
	}
}