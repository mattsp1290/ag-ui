package cache

import (
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	_ "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// CacheStrategyTestSuite provides comprehensive testing for cache strategies
type CacheStrategyTestSuite struct {
	suite.Suite
}

func (suite *CacheStrategyTestSuite) TestTTLStrategy() {
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
	
	// Test ShouldCache
	suite.True(strategy.ShouldCache(key, result))
	
	// Test initial TTL
	ttl := strategy.GetTTL(key)
	suite.Equal(5*time.Minute, ttl)
	
	// Test hit tracking
	entry := &ValidationCacheEntry{
		Key:            key,
		AccessCount:    1,
		LastAccessedAt: time.Now(),
		ExpiresAt:      time.Now().Add(5 * time.Minute),
	}
	
	strategy.OnHit(key, entry)
	
	// TTL should increase after hits
	newTTL := strategy.GetTTL(key)
	suite.Greater(newTTL, ttl)
	
	// Test score calculation
	score := strategy.Score(entry)
	suite.Greater(score, 0.0)
}

func (suite *CacheStrategyTestSuite) TestTTLStrategyAdaptive() {
	strategy := NewTTLStrategy(5 * time.Minute)
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	entry := &ValidationCacheEntry{
		Key:            key,
		AccessCount:    1,
		LastAccessedAt: time.Now(),
		ExpiresAt:      time.Now().Add(5 * time.Minute),
	}
	
	// Multiple hits should increase TTL
	for i := 0; i < 5; i++ {
		strategy.OnHit(key, entry)
	}
	
	adaptiveTTL := strategy.GetTTL(key)
	suite.Greater(adaptiveTTL, 5*time.Minute)
	suite.LessOrEqual(adaptiveTTL, strategy.MaxTTL)
}

func (suite *CacheStrategyTestSuite) TestLFUStrategy() {
	strategy := NewLFUStrategy()
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	result := ValidationResult{
		Valid:          true,
		ValidationTime: 10 * time.Millisecond,
		EventSize:      1024,
	}
	
	// Test ShouldCache
	suite.True(strategy.ShouldCache(key, result))
	
	// Test fixed TTL
	ttl := strategy.GetTTL(key)
	suite.Equal(30*time.Minute, ttl)
	
	// Test miss tracking
	strategy.OnMiss(key)
	
	// Test hit tracking
	entry := &ValidationCacheEntry{
		Key:            key,
		AccessCount:    1,
		LastAccessedAt: time.Now(),
		ExpiresAt:      time.Now().Add(30 * time.Minute),
	}
	
	initialScore := strategy.Score(entry)
	
	// Multiple hits should increase frequency
	for i := 0; i < 5; i++ {
		strategy.OnHit(key, entry)
	}
	
	newScore := strategy.Score(entry)
	suite.Greater(newScore, initialScore)
}

func (suite *CacheStrategyTestSuite) TestLFUStrategyDecay() {
	strategy := NewLFUStrategy()
	strategy.DecayInterval = 100 * time.Millisecond // Fast decay for testing
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	// Add some hits
	entry := &ValidationCacheEntry{
		Key:            key,
		AccessCount:    1,
		LastAccessedAt: time.Now(),
		ExpiresAt:      time.Now().Add(30 * time.Minute),
	}
	
	for i := 0; i < 10; i++ {
		strategy.OnHit(key, entry)
	}
	
	initialScore := strategy.Score(entry)
	
	// Wait for decay interval to pass
	time.Sleep(150 * time.Millisecond)
	
	// Manually trigger decay without adding a hit
	strategy.TriggerDecay()
	
	decayedScore := strategy.Score(entry)
	suite.Less(decayedScore, initialScore)
}

func (suite *CacheStrategyTestSuite) TestAdaptiveStrategy() {
	baseStrategy := NewTTLStrategy(5 * time.Minute)
	adaptiveStrategy := NewAdaptiveStrategy(baseStrategy, 1024*1024) // 1MB limit
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	// Test normal conditions
	smallResult := ValidationResult{
		Valid:          true,
		ValidationTime: 10 * time.Millisecond,
		EventSize:      1024, // 1KB
	}
	
	suite.True(adaptiveStrategy.ShouldCache(key, smallResult))
	
	// Test memory pressure
	adaptiveStrategy.UpdateSystemMetrics(900*1024, 0.5) // 900KB memory, low load
	
	largeResult := ValidationResult{
		Valid:          true,
		ValidationTime: 10 * time.Millisecond,
		EventSize:      20 * 1024, // 20KB - too large under memory pressure
	}
	
	suite.False(adaptiveStrategy.ShouldCache(key, largeResult))
	
	// Test high system load
	adaptiveStrategy.UpdateSystemMetrics(500*1024, 0.9) // Lower memory, high load
	
	fastResult := ValidationResult{
		Valid:          true,
		ValidationTime: 5 * time.Millisecond, // Fast validation
		EventSize:      1024,
	}
	
	suite.False(adaptiveStrategy.ShouldCache(key, fastResult))
	
	slowResult := ValidationResult{
		Valid:          true,
		ValidationTime: 50 * time.Millisecond, // Slow validation
		EventSize:      1024,
	}
	
	suite.True(adaptiveStrategy.ShouldCache(key, slowResult))
}

func (suite *CacheStrategyTestSuite) TestAdaptiveStrategyTTLAdjustment() {
	baseStrategy := NewTTLStrategy(5 * time.Minute)
	adaptiveStrategy := NewAdaptiveStrategy(baseStrategy, 1024*1024)
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	// Normal conditions
	baseTTL := adaptiveStrategy.GetTTL(key)
	suite.Equal(5*time.Minute, baseTTL)
	
	// High memory pressure
	adaptiveStrategy.UpdateSystemMetrics(950*1024, 0.5) // 95% memory usage
	
	reducedTTL := adaptiveStrategy.GetTTL(key)
	suite.Less(reducedTTL, baseTTL)
	suite.Equal(baseTTL/2, reducedTTL)
}

func (suite *CacheStrategyTestSuite) TestPredictiveStrategy() {
	baseStrategy := NewTTLStrategy(5 * time.Minute)
	predictiveStrategy := NewPredictiveStrategy(baseStrategy)
	predictiveStrategy.MinPatternCount = 2 // Lower for testing
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	// Test without pattern
	baseTTL := predictiveStrategy.GetTTL(key)
	suite.Equal(5*time.Minute, baseTTL)
	
	// Create access pattern
	now := time.Now()
	predictiveStrategy.updatePattern(key.EventHash, now)
	predictiveStrategy.updatePattern(key.EventHash, now.Add(10*time.Minute))
	predictiveStrategy.updatePattern(key.EventHash, now.Add(20*time.Minute))
	
	// Should predict next access and extend TTL
	predictiveTTL := predictiveStrategy.GetTTL(key)
	suite.Greater(predictiveTTL, baseTTL)
	
	// Test score with prediction
	entry := &ValidationCacheEntry{
		Key:            key,
		AccessCount:    1,
		LastAccessedAt: now,
		ExpiresAt:      now.Add(5 * time.Minute),
	}
	
	score := predictiveStrategy.Score(entry)
	suite.Greater(score, 0.0)
}

func (suite *CacheStrategyTestSuite) TestPredictiveStrategyPatternDetection() {
	baseStrategy := NewTTLStrategy(5 * time.Minute)
	predictiveStrategy := NewPredictiveStrategy(baseStrategy)
	predictiveStrategy.MinPatternCount = 3
	
	eventHash := "hash123"
	
	// Create regular pattern (every 10 minutes)
	now := time.Now()
	accessTimes := []time.Time{
		now,
		now.Add(10 * time.Minute),
		now.Add(20 * time.Minute),
		now.Add(30 * time.Minute),
	}
	
	for _, accessTime := range accessTimes {
		predictiveStrategy.updatePattern(eventHash, accessTime)
	}
	
	// Check pattern
	pattern := predictiveStrategy.accessPatterns[eventHash]
	suite.NotNil(pattern)
	suite.Greater(pattern.Confidence, 0.5)
	suite.True(pattern.PredictedNext.After(now.Add(30*time.Minute)))
}

func (suite *CacheStrategyTestSuite) TestCompositeStrategy() {
	strategies := []CacheStrategy{
		NewTTLStrategy(5 * time.Minute),
		NewLFUStrategy(),
	}
	weights := []float64{0.7, 0.3}
	
	compositeStrategy, err := NewCompositeStrategy(strategies, weights)
	suite.NoError(err)
	suite.NotNil(compositeStrategy)
	
	// Test weight normalization
	expectedWeights := []float64{0.7, 0.3}
	suite.Equal(expectedWeights, compositeStrategy.Weights)
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	result := ValidationResult{
		Valid:          true,
		ValidationTime: 10 * time.Millisecond,
		EventSize:      1024,
	}
	
	// Test ShouldCache (all strategies must agree)
	suite.True(compositeStrategy.ShouldCache(key, result))
	
	// Test weighted TTL
	ttl := compositeStrategy.GetTTL(key)
	expectedTTL := time.Duration(float64(5*time.Minute)*0.7 + float64(30*time.Minute)*0.3)
	suite.Equal(expectedTTL, ttl)
	
	// Test score calculation
	entry := &ValidationCacheEntry{
		Key:            key,
		AccessCount:    1,
		LastAccessedAt: time.Now(),
		ExpiresAt:      time.Now().Add(5 * time.Minute),
	}
	
	score := compositeStrategy.Score(entry)
	suite.Greater(score, 0.0)
}

func (suite *CacheStrategyTestSuite) TestCompositeStrategyError() {
	strategies := []CacheStrategy{
		NewTTLStrategy(5 * time.Minute),
		NewLFUStrategy(),
	}
	weights := []float64{0.7} // Mismatched length
	
	_, err := NewCompositeStrategy(strategies, weights)
	suite.Error(err)
}

func (suite *CacheStrategyTestSuite) TestCompositeStrategyAllMustAgree() {
	// Create a strategy that always says no to caching
	noCache := &TestCacheStrategy{shouldCache: false}
	yesCache := &TestCacheStrategy{shouldCache: true}
	
	strategies := []CacheStrategy{noCache, yesCache}
	weights := []float64{0.5, 0.5}
	
	compositeStrategy, err := NewCompositeStrategy(strategies, weights)
	suite.NoError(err)
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	result := ValidationResult{
		Valid:          true,
		ValidationTime: 10 * time.Millisecond,
		EventSize:      1024,
	}
	
	// Should not cache because one strategy says no
	suite.False(compositeStrategy.ShouldCache(key, result))
}

// TestCacheStrategy is a mock strategy for testing
type TestCacheStrategy struct {
	shouldCache bool
	ttl         time.Duration
}

func (s *TestCacheStrategy) ShouldCache(key ValidationCacheKey, result ValidationResult) bool {
	return s.shouldCache
}

func (s *TestCacheStrategy) GetTTL(key ValidationCacheKey) time.Duration {
	if s.ttl == 0 {
		return 5 * time.Minute
	}
	return s.ttl
}

func (s *TestCacheStrategy) OnHit(key ValidationCacheKey, entry *ValidationCacheEntry) {
	// No-op
}

func (s *TestCacheStrategy) OnMiss(key ValidationCacheKey) {
	// No-op
}

func (s *TestCacheStrategy) Score(entry *ValidationCacheEntry) float64 {
	return 0.5
}

func TestCacheStrategyTestSuite(t *testing.T) {
	suite.Run(t, new(CacheStrategyTestSuite))
}

// Benchmark tests for cache strategies
func BenchmarkTTLStrategy(b *testing.B) {
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
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.ShouldCache(key, result)
		strategy.GetTTL(key)
	}
}

func BenchmarkLFUStrategy(b *testing.B) {
	strategy := NewLFUStrategy()
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	result := ValidationResult{
		Valid:          true,
		ValidationTime: 10 * time.Millisecond,
		EventSize:      1024,
	}
	
	entry := &ValidationCacheEntry{
		Key:            key,
		AccessCount:    1,
		LastAccessedAt: time.Now(),
		ExpiresAt:      time.Now().Add(30 * time.Minute),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.ShouldCache(key, result)
		strategy.GetTTL(key)
		strategy.Score(entry)
	}
}

func BenchmarkAdaptiveStrategy(b *testing.B) {
	baseStrategy := NewTTLStrategy(5 * time.Minute)
	adaptiveStrategy := NewAdaptiveStrategy(baseStrategy, 1024*1024)
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	result := ValidationResult{
		Valid:          true,
		ValidationTime: 10 * time.Millisecond,
		EventSize:      1024,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adaptiveStrategy.ShouldCache(key, result)
		adaptiveStrategy.GetTTL(key)
	}
}

func BenchmarkCompositeStrategy(b *testing.B) {
	strategies := []CacheStrategy{
		NewTTLStrategy(5 * time.Minute),
		NewLFUStrategy(),
	}
	weights := []float64{0.7, 0.3}
	
	compositeStrategy, _ := NewCompositeStrategy(strategies, weights)
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	result := ValidationResult{
		Valid:          true,
		ValidationTime: 10 * time.Millisecond,
		EventSize:      1024,
	}
	
	entry := &ValidationCacheEntry{
		Key:            key,
		AccessCount:    1,
		LastAccessedAt: time.Now(),
		ExpiresAt:      time.Now().Add(5 * time.Minute),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compositeStrategy.ShouldCache(key, result)
		compositeStrategy.GetTTL(key)
		compositeStrategy.Score(entry)
	}
}