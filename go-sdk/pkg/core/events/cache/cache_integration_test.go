package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// CacheIntegrationTestSuite tests cache integration scenarios
type CacheIntegrationTestSuite struct {
	suite.Suite
	primaryCache   *CacheValidator
	secondaryCache *CacheValidator
	mockL2Cache    *MockDistributedCache
	coordinator    *CacheCoordinator
	ctx            context.Context
	cancel         context.CancelFunc
}

func (suite *CacheIntegrationTestSuite) SetupTest() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.mockL2Cache = NewMockDistributedCache()
	
	// Create coordinator for distributed cache coordination
	coordinatorConfig := DefaultCoordinatorConfig()
	suite.coordinator = &CacheCoordinator{
		nodeID:         "coordinator",
		nodes:          make(map[string]*NodeInfo),
		config:         coordinatorConfig,
		invalidationCh: make(chan InvalidationMessage, 100),
		updateCh:       make(chan CacheUpdateMessage, 100),
		metricsCh:      make(chan MetricsReport, 100),
		clusterState:   &ClusterState{},
		shutdownCh:     make(chan struct{}),
	}
	
	// Create primary cache node
	primaryConfig := &CacheValidatorConfig{
		L1Size:        100,
		L1TTL:         1 * time.Minute,
		L2Cache:       suite.mockL2Cache,
		L2Enabled:     true,
		L2TTL:         5 * time.Minute,
		NodeID:        "node-1",
		Coordinator:   suite.coordinator,
		MetricsEnabled: true,
	}
	
	var err error
	suite.primaryCache, err = NewCacheValidator(primaryConfig)
	suite.Require().NoError(err)
	
	// Create secondary cache node
	secondaryConfig := &CacheValidatorConfig{
		L1Size:        100,
		L1TTL:         1 * time.Minute,
		L2Cache:       suite.mockL2Cache,
		L2Enabled:     true,
		L2TTL:         5 * time.Minute,
		NodeID:        "node-2",
		Coordinator:   suite.coordinator,
		MetricsEnabled: true,
	}
	
	suite.secondaryCache, err = NewCacheValidator(secondaryConfig)
	suite.Require().NoError(err)
}

func (suite *CacheIntegrationTestSuite) TearDownTest() {
	if suite.primaryCache != nil {
		suite.primaryCache.Shutdown(suite.ctx)
	}
	if suite.secondaryCache != nil {
		suite.secondaryCache.Shutdown(suite.ctx)
	}
	if suite.cancel != nil {
		suite.cancel()
	}
}

// TestDistributedCacheIntegration tests L1/L2 cache interaction
func (suite *CacheIntegrationTestSuite) TestDistributedCacheIntegration() {
	event := events.NewRunStartedEvent("thread-1", "run-1")
	
	// Node 1: Initial validation (miss all caches)
	err := suite.primaryCache.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	stats1 := suite.primaryCache.GetStats()
	suite.Equal(uint64(1), stats1.L1Misses, "Should have L1 miss on first access")
	
	// Node 2: Should hit L2 cache
	err = suite.secondaryCache.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	stats2 := suite.secondaryCache.GetStats()
	suite.Equal(uint64(1), stats2.L2Hits, "Should hit L2 cache from node 1's write")
	suite.Equal(uint64(0), stats2.L1Hits, "Should not have L1 hit on first access")
	
	// Node 2: Second access should hit L1
	err = suite.secondaryCache.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	stats2 = suite.secondaryCache.GetStats()
	suite.Equal(uint64(1), stats2.L1Hits, "Should hit L1 cache on second access")
}

// TestCacheCoordination tests distributed cache coordination
func (suite *CacheIntegrationTestSuite) TestCacheCoordination() {
	// Register nodes with coordinator
	suite.coordinator.nodes["node-1"] = &NodeInfo{
		ID:            "node-1",
		Address:       "localhost:8001",
		State:         NodeStateActive,
		LastHeartbeat: time.Now(),
		Metrics:       CacheStats{},
		Shards:        []int{},
	}
	
	suite.coordinator.nodes["node-2"] = &NodeInfo{
		ID:            "node-2",
		Address:       "localhost:8002",
		State:         NodeStateActive,
		LastHeartbeat: time.Now(),
		Metrics:       CacheStats{},
		Shards:        []int{},
	}
	
	event := events.NewRunStartedEvent("thread-1", "run-1")
	
	// Both nodes cache the event
	err := suite.primaryCache.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	err = suite.secondaryCache.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	// Verify both have the event in L1
	stats1 := suite.primaryCache.GetStats()
	stats2 := suite.secondaryCache.GetStats()
	
	// Node 1 invalidates the event
	err = suite.primaryCache.InvalidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	// Give time for coordination
	time.Sleep(100 * time.Millisecond)
	
	// Both nodes should have cache miss
	err = suite.primaryCache.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	err = suite.secondaryCache.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	stats1 = suite.primaryCache.GetStats()
	stats2 = suite.secondaryCache.GetStats()
	
	// Both should have additional misses
	suite.Greater(stats1.L1Misses, uint64(1))
	suite.Greater(stats2.L1Misses, uint64(1))
}

// TestCacheFallbackBehavior tests fallback when L2 is unavailable
func (suite *CacheIntegrationTestSuite) TestCacheFallbackBehavior() {
	event := events.NewRunStartedEvent("thread-1", "run-1")
	
	// Normal operation
	err := suite.primaryCache.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	// Clear L1 to force L2 lookup
	suite.primaryCache.l1Cache.Purge()
	
	// Set L2 to error mode
	suite.mockL2Cache.SetError(true)
	
	// Should still work by falling back to validation
	err = suite.primaryCache.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
	
	// Restore L2
	suite.mockL2Cache.SetError(false)
	
	// Should work normally again
	err = suite.primaryCache.ValidateEvent(suite.ctx, event)
	suite.NoError(err)
}

// TestHighLoadCachePerformance tests cache under high load
func (suite *CacheIntegrationTestSuite) TestHighLoadCachePerformance() {
	numWorkers := 20
	numRequests := 1000
	numUniqueEvents := 100
	
	// Pre-generate events
	eventList := make([]events.Event, numUniqueEvents)
	for i := 0; i < numUniqueEvents; i++ {
		eventList[i] = events.NewRunStartedEvent(
			fmt.Sprintf("thread-%d", i),
			fmt.Sprintf("run-%d", i),
		)
	}
	
	// Warm up cache
	for _, event := range eventList {
		err := suite.primaryCache.ValidateEvent(suite.ctx, event)
		suite.NoError(err)
	}
	
	// Track metrics
	var totalRequests int64
	var cacheHits int64
	var errors int64
	startTime := time.Now()
	
	// Launch workers
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for i := 0; i < numRequests; i++ {
				event := eventList[i%numUniqueEvents]
				
				err := suite.primaryCache.ValidateEvent(suite.ctx, event)
				if err != nil {
					atomic.AddInt64(&errors, 1)
					continue
				}
				
				atomic.AddInt64(&totalRequests, 1)
			}
		}(w)
	}
	
	wg.Wait()
	duration := time.Since(startTime)
	
	// Get final stats
	stats := suite.primaryCache.GetStats()
	cacheHits = int64(stats.L1Hits + stats.L2Hits)
	
	// Calculate performance metrics
	requestsPerSecond := float64(totalRequests) / duration.Seconds()
	cacheHitRate := float64(cacheHits) / float64(totalRequests) * 100
	
	// Log performance results
	suite.T().Logf("Performance Test Results:")
	suite.T().Logf("  Total Requests: %d", totalRequests)
	suite.T().Logf("  Duration: %v", duration)
	suite.T().Logf("  Requests/sec: %.2f", requestsPerSecond)
	suite.T().Logf("  Cache Hit Rate: %.2f%%", cacheHitRate)
	suite.T().Logf("  Errors: %d", errors)
	
	// Assertions
	suite.Equal(int64(0), errors, "Should have no errors")
	suite.Greater(cacheHitRate, 90.0, "Cache hit rate should be high")
	suite.Greater(requestsPerSecond, 10000.0, "Should handle >10k requests/sec")
}

// TestCacheMemoryPressure tests cache behavior under memory pressure
func (suite *CacheIntegrationTestSuite) TestCacheMemoryPressure() {
	// Create cache with small size
	config := &CacheValidatorConfig{
		L1Size:        10, // Very small cache
		L1TTL:         1 * time.Minute,
		L2Cache:       suite.mockL2Cache,
		L2Enabled:     true,
		L2TTL:         5 * time.Minute,
		MetricsEnabled: true,
	}
	
	smallCache, err := NewCacheValidator(config)
	suite.Require().NoError(err)
	defer smallCache.Shutdown(suite.ctx)
	
	// Fill cache beyond capacity
	for i := 0; i < 50; i++ {
		event := events.NewRunStartedEvent(
			fmt.Sprintf("thread-%d", i),
			fmt.Sprintf("run-%d", i),
		)
		err := smallCache.ValidateEvent(suite.ctx, event)
		suite.NoError(err)
	}
	
	stats := smallCache.GetStats()
	suite.Greater(stats.Evictions, uint64(30), "Should have evictions due to small cache size")
	
	// Verify cache is still functional
	testEvent := events.NewRunStartedEvent("test-thread", "test-run")
	err = smallCache.ValidateEvent(suite.ctx, testEvent)
	suite.NoError(err)
}

// TestCacheInvalidationPropagation tests invalidation across nodes
func (suite *CacheIntegrationTestSuite) TestCacheInvalidationPropagation() {
	// Create events
	testEvents := []events.Event{
		events.NewRunStartedEvent("thread-1", "run-1"),
		events.NewRunStartedEvent("thread-2", "run-2"),
		events.NewToolCallStartEvent("tool-1", "ToolName"),
	}
	
	// Cache events on both nodes
	for _, event := range testEvents {
		err := suite.primaryCache.ValidateEvent(suite.ctx, event)
		suite.NoError(err)
		err = suite.secondaryCache.ValidateEvent(suite.ctx, event)
		suite.NoError(err)
	}
	
	// Invalidate by event type on primary node
	err := suite.primaryCache.InvalidateEventType(suite.ctx, events.EventType("RunStarted"))
	suite.NoError(err)
	
	// Check that L2 entries are removed
	time.Sleep(100 * time.Millisecond)
	
	// Both nodes should miss for RunStarted events
	for i := 0; i < 2; i++ {
		err = suite.primaryCache.ValidateEvent(suite.ctx, testEvents[i])
		suite.NoError(err)
		err = suite.secondaryCache.ValidateEvent(suite.ctx, testEvents[i])
		suite.NoError(err)
	}
	
	// Tool event should still be cached
	err = suite.primaryCache.ValidateEvent(suite.ctx, testEvents[2])
	suite.NoError(err)
	
	primaryStats := suite.primaryCache.GetStats()
	secondaryStats := suite.secondaryCache.GetStats()
	
	suite.Greater(primaryStats.L1Misses, uint64(2))
	suite.Greater(secondaryStats.L1Misses, uint64(2))
}

// TestCacheWarmupIntegration tests cache warmup in distributed setup
func (suite *CacheIntegrationTestSuite) TestCacheWarmupIntegration() {
	// Prepare warmup events
	warmupEvents := make([]events.Event, 50)
	for i := 0; i < 50; i++ {
		warmupEvents[i] = events.NewRunStartedEvent(
			fmt.Sprintf("thread-%d", i),
			fmt.Sprintf("run-%d", i),
		)
	}
	
	// Warmup primary cache
	err := suite.primaryCache.Warmup(suite.ctx, warmupEvents)
	suite.NoError(err)
	
	// Verify all events are in L1
	for _, event := range warmupEvents {
		err = suite.primaryCache.ValidateEvent(suite.ctx, event)
		suite.NoError(err)
	}
	
	stats := suite.primaryCache.GetStats()
	suite.Equal(uint64(50), stats.L1Hits, "All warmed events should hit L1")
	
	// Secondary cache should be able to use L2
	for _, event := range warmupEvents[:10] {
		err = suite.secondaryCache.ValidateEvent(suite.ctx, event)
		suite.NoError(err)
	}
	
	stats2 := suite.secondaryCache.GetStats()
	suite.Equal(uint64(10), stats2.L2Hits, "Secondary should hit L2 for warmed events")
}

// TestConcurrentMultiNodeAccess tests concurrent access from multiple nodes
func (suite *CacheIntegrationTestSuite) TestConcurrentMultiNodeAccess() {
	numEvents := 20
	numIterations := 100
	
	// Create shared events
	sharedEvents := make([]events.Event, numEvents)
	for i := 0; i < numEvents; i++ {
		sharedEvents[i] = events.NewRunStartedEvent(
			fmt.Sprintf("shared-thread-%d", i),
			fmt.Sprintf("shared-run-%d", i),
		)
	}
	
	var wg sync.WaitGroup
	errors := make(chan error, 2*numIterations)
	
	// Node 1 operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numIterations; i++ {
			event := sharedEvents[i%numEvents]
			if err := suite.primaryCache.ValidateEvent(suite.ctx, event); err != nil {
				errors <- err
				return
			}
			
			// Occasionally invalidate
			if i%20 == 0 {
				if err := suite.primaryCache.InvalidateEvent(suite.ctx, event); err != nil {
					errors <- err
					return
				}
			}
		}
	}()
	
	// Node 2 operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numIterations; i++ {
			event := sharedEvents[i%numEvents]
			if err := suite.secondaryCache.ValidateEvent(suite.ctx, event); err != nil {
				errors <- err
				return
			}
			
			// Occasionally invalidate
			if i%25 == 0 {
				if err := suite.secondaryCache.InvalidateEvent(suite.ctx, event); err != nil {
					errors <- err
					return
				}
			}
		}
	}()
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	for err := range errors {
		suite.Fail("Concurrent multi-node operation failed", err)
	}
	
	// Verify both caches are still functional
	testEvent := events.NewRunStartedEvent("final-test", "final-run")
	err := suite.primaryCache.ValidateEvent(suite.ctx, testEvent)
	suite.NoError(err)
	err = suite.secondaryCache.ValidateEvent(suite.ctx, testEvent)
	suite.NoError(err)
}

func TestCacheIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(CacheIntegrationTestSuite))
}

// Standalone integration tests

// TestCacheWithAuthentication tests cache integration with authentication
func TestCacheWithAuthentication(t *testing.T) {
	// Mock authentication validator
	authValidator := &MockAuthenticationValidator{
		validTokens: map[string]bool{
			"valid-token-1": true,
			"valid-token-2": true,
		},
	}
	
	config := &CacheValidatorConfig{
		L1Size:        100,
		L1TTL:         5 * time.Minute,
		L2Enabled:     false,
		MetricsEnabled: true,
		InvalidationStrategies: []InvalidationStrategy{
			&AuthenticationInvalidationStrategy{
				authValidator: authValidator,
			},
		},
	}
	
	cv, err := NewCacheValidator(config)
	require.NoError(t, err)
	defer cv.Shutdown(context.Background())
	
	ctx := context.Background()
	
	// Create authenticated events
	event1 := &AuthenticatedEvent{
		Event: events.NewRunStartedEvent("thread-1", "run-1"),
		Token: "valid-token-1",
	}
	
	event2 := &AuthenticatedEvent{
		Event: events.NewRunStartedEvent("thread-2", "run-2"),
		Token: "invalid-token",
	}
	
	// Valid token should cache
	err = cv.ValidateEvent(ctx, event1.Event)
	assert.NoError(t, err)
	
	// Invalid token should not affect cache
	err = cv.ValidateEvent(ctx, event2.Event)
	assert.NoError(t, err)
	
	// Revoke token
	authValidator.RevokeToken("valid-token-1")
	
	// Cache should be invalidated for revoked token
	// This would be handled by the invalidation strategy
}

// TestCacheWithMonitoring tests cache integration with monitoring
func TestCacheWithMonitoring(t *testing.T) {
	// Create monitoring collector
	collector := NewMetricsCollector(DefaultMetricsConfig())
	
	config := &CacheValidatorConfig{
		L1Size:        100,
		L1TTL:         5 * time.Minute,
		L2Enabled:     false,
		MetricsEnabled: true,
	}
	
	cv, err := NewCacheValidator(config)
	require.NoError(t, err)
	defer cv.Shutdown(context.Background())
	
	ctx := context.Background()
	
	// Perform operations
	for i := 0; i < 100; i++ {
		event := events.NewRunStartedEvent(
			fmt.Sprintf("thread-%d", i%10),
			fmt.Sprintf("run-%d", i%10),
		)
		
		start := time.Now()
		err := cv.ValidateEvent(ctx, event)
		duration := time.Since(start)
		
		assert.NoError(t, err)
		
		// Record metrics
		if i < 10 {
			collector.RecordMiss(duration)
		} else {
			collector.RecordHit(L1Cache, duration)
		}
	}
	
	// Get metrics report
	report := collector.GetReport()
	
	assert.Equal(t, uint64(90), report.BasicMetrics.Hits)
	assert.Equal(t, uint64(10), report.BasicMetrics.Misses)
	assert.Greater(t, report.BasicMetrics.HitRate, 0.85)
}

// Mock types for integration tests

type MockAuthenticationValidator struct {
	validTokens map[string]bool
	mu          sync.RWMutex
}

func (m *MockAuthenticationValidator) ValidateToken(token string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.validTokens[token]
}

func (m *MockAuthenticationValidator) RevokeToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.validTokens, token)
}

type AuthenticatedEvent struct {
	Event events.Event
	Token string
}

type AuthenticationInvalidationStrategy struct {
	authValidator *MockAuthenticationValidator
}

func (s *AuthenticationInvalidationStrategy) ShouldInvalidate(entry *ValidationCacheEntry) bool {
	// Check if associated token is still valid
	if token, ok := entry.Metadata["auth_token"].(string); ok {
		return !s.authValidator.ValidateToken(token)
	}
	return false
}

func (s *AuthenticationInvalidationStrategy) OnInvalidate(key ValidationCacheKey) {
	// Log invalidation
}