package cache

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// CacheStrategy defines the interface for cache strategies
type CacheStrategy interface {
	// ShouldCache determines if a validation result should be cached
	ShouldCache(key ValidationCacheKey, result ValidationResult) bool
	
	// GetTTL returns the TTL for a cache entry
	GetTTL(key ValidationCacheKey) time.Duration
	
	// OnHit is called when a cache hit occurs
	OnHit(key ValidationCacheKey, entry *ValidationCacheEntry)
	
	// OnMiss is called when a cache miss occurs
	OnMiss(key ValidationCacheKey)
	
	// Score returns a score for cache entry prioritization
	Score(entry *ValidationCacheEntry) float64
}

// ValidationResult represents a validation result
type ValidationResult struct {
	Valid          bool
	Errors         []error
	ValidationTime time.Duration
	EventSize      int
}

// TTLStrategy implements time-based caching
type TTLStrategy struct {
	DefaultTTL     time.Duration
	MaxTTL         time.Duration
	MinTTL         time.Duration
	TTLMultiplier  float64
	mu             sync.RWMutex
	hitCounts      map[string]uint64
}

// NewTTLStrategy creates a new TTL-based strategy
func NewTTLStrategy(defaultTTL time.Duration) *TTLStrategy {
	return &TTLStrategy{
		DefaultTTL:    defaultTTL,
		MaxTTL:        24 * time.Hour,
		MinTTL:        1 * time.Minute,
		TTLMultiplier: 1.5,
		hitCounts:     make(map[string]uint64),
	}
}

func (s *TTLStrategy) ShouldCache(key ValidationCacheKey, result ValidationResult) bool {
	// Always cache validation results
	return true
}

func (s *TTLStrategy) GetTTL(key ValidationCacheKey) time.Duration {
	s.mu.RLock()
	hits := s.hitCounts[key.EventHash]
	s.mu.RUnlock()
	
	// Adaptive TTL based on hit count
	ttl := s.DefaultTTL
	if hits > 0 {
		// Increase TTL for frequently accessed items
		multiplier := math.Min(float64(hits)*s.TTLMultiplier, 10.0)
		ttl = time.Duration(float64(s.DefaultTTL) * multiplier)
	}
	
	// Clamp to min/max
	if ttl < s.MinTTL {
		ttl = s.MinTTL
	} else if ttl > s.MaxTTL {
		ttl = s.MaxTTL
	}
	
	return ttl
}

func (s *TTLStrategy) OnHit(key ValidationCacheKey, entry *ValidationCacheEntry) {
	s.mu.Lock()
	s.hitCounts[key.EventHash]++
	s.mu.Unlock()
}

func (s *TTLStrategy) OnMiss(key ValidationCacheKey) {
	// No action on miss for TTL strategy
}

func (s *TTLStrategy) Score(entry *ValidationCacheEntry) float64 {
	// Score based on remaining TTL and access count
	remainingTTL := time.Until(entry.ExpiresAt)
	ttlScore := float64(remainingTTL) / float64(s.MaxTTL)
	accessScore := math.Log10(float64(entry.AccessCount + 1))
	
	return ttlScore*0.3 + accessScore*0.7
}

// LFUStrategy implements Least Frequently Used caching
type LFUStrategy struct {
	MinFrequency   uint64
	DecayRate      float64
	DecayInterval  time.Duration
	mu             sync.RWMutex
	frequencies    map[string]float64
	lastDecay      time.Time
}

// NewLFUStrategy creates a new LFU-based strategy
func NewLFUStrategy() *LFUStrategy {
	return &LFUStrategy{
		MinFrequency:  1,
		DecayRate:     0.95,
		DecayInterval: 1 * time.Hour,
		frequencies:   make(map[string]float64),
		lastDecay:     time.Now(),
	}
}

func (s *LFUStrategy) ShouldCache(key ValidationCacheKey, result ValidationResult) bool {
	// Cache everything initially
	return true
}

func (s *LFUStrategy) GetTTL(key ValidationCacheKey) time.Duration {
	// Fixed TTL for LFU
	return 30 * time.Minute
}

func (s *LFUStrategy) OnHit(key ValidationCacheKey, entry *ValidationCacheEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Apply decay if needed
	s.applyDecay()
	
	// Increment frequency
	s.frequencies[key.EventHash]++
}

func (s *LFUStrategy) OnMiss(key ValidationCacheKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Initialize frequency for new items
	s.frequencies[key.EventHash] = float64(s.MinFrequency)
}

func (s *LFUStrategy) Score(entry *ValidationCacheEntry) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	freq, exists := s.frequencies[entry.Key.EventHash]
	if !exists {
		return 0
	}
	
	// Consider both frequency and recency
	recencyScore := 1.0 / (1.0 + time.Since(entry.LastAccessedAt).Hours())
	
	return freq*0.7 + recencyScore*0.3
}

func (s *LFUStrategy) applyDecay() {
	if time.Since(s.lastDecay) < s.DecayInterval {
		return
	}
	
	// Apply exponential decay to all frequencies
	for key, freq := range s.frequencies {
		s.frequencies[key] = freq * s.DecayRate
		if s.frequencies[key] < float64(s.MinFrequency) {
			s.frequencies[key] = float64(s.MinFrequency)
		}
	}
	
	s.lastDecay = time.Now()
}

// AdaptiveStrategy adjusts caching based on system conditions
type AdaptiveStrategy struct {
	BaseStrategy   CacheStrategy
	MemoryLimit    int64
	LoadThreshold  float64
	mu             sync.RWMutex
	currentMemory  int64
	systemLoad     float64
}

// NewAdaptiveStrategy creates a new adaptive strategy
func NewAdaptiveStrategy(baseStrategy CacheStrategy, memoryLimit int64) *AdaptiveStrategy {
	return &AdaptiveStrategy{
		BaseStrategy:  baseStrategy,
		MemoryLimit:   memoryLimit,
		LoadThreshold: 0.8,
	}
}

func (s *AdaptiveStrategy) ShouldCache(key ValidationCacheKey, result ValidationResult) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Check memory constraints
	if s.currentMemory >= int64(float64(s.MemoryLimit)*s.LoadThreshold) {
		// Only cache small, frequently accessed items when memory is tight
		if result.EventSize > 1024*10 { // 10KB threshold
			return false
		}
	}
	
	// Check system load
	if s.systemLoad > s.LoadThreshold {
		// Be more selective during high load
		if result.ValidationTime < 10*time.Millisecond {
			return false // Don't cache fast validations
		}
	}
	
	return s.BaseStrategy.ShouldCache(key, result)
}

func (s *AdaptiveStrategy) GetTTL(key ValidationCacheKey) time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	baseTTL := s.BaseStrategy.GetTTL(key)
	
	// Reduce TTL under memory pressure
	if s.currentMemory >= int64(float64(s.MemoryLimit)*0.9) {
		baseTTL = baseTTL / 2
	}
	
	return baseTTL
}

func (s *AdaptiveStrategy) OnHit(key ValidationCacheKey, entry *ValidationCacheEntry) {
	s.BaseStrategy.OnHit(key, entry)
}

func (s *AdaptiveStrategy) OnMiss(key ValidationCacheKey) {
	s.BaseStrategy.OnMiss(key)
}

func (s *AdaptiveStrategy) Score(entry *ValidationCacheEntry) float64 {
	baseScore := s.BaseStrategy.Score(entry)
	
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Adjust score based on system conditions
	if s.systemLoad > s.LoadThreshold {
		// Prefer smaller entries during high load
		sizeScore := 1.0 / (1.0 + float64(entry.Metadata["event_size"].(int))/1024.0)
		baseScore = baseScore*0.7 + sizeScore*0.3
	}
	
	return baseScore
}

func (s *AdaptiveStrategy) UpdateSystemMetrics(memory int64, load float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.currentMemory = memory
	s.systemLoad = load
}

// PredictiveStrategy uses patterns to predict cache needs
type PredictiveStrategy struct {
	BaseStrategy    CacheStrategy
	PatternWindow   time.Duration
	MinPatternCount int
	mu              sync.RWMutex
	accessPatterns  map[string]*AccessPattern
}

// AccessPattern tracks access patterns for prediction
type AccessPattern struct {
	LastAccess   time.Time
	AccessTimes  []time.Time
	PredictedNext time.Time
	Confidence   float64
}

// NewPredictiveStrategy creates a new predictive strategy
func NewPredictiveStrategy(baseStrategy CacheStrategy) *PredictiveStrategy {
	return &PredictiveStrategy{
		BaseStrategy:    baseStrategy,
		PatternWindow:   24 * time.Hour,
		MinPatternCount: 3,
		accessPatterns:  make(map[string]*AccessPattern),
	}
}

func (s *PredictiveStrategy) ShouldCache(key ValidationCacheKey, result ValidationResult) bool {
	return s.BaseStrategy.ShouldCache(key, result)
}

func (s *PredictiveStrategy) GetTTL(key ValidationCacheKey) time.Duration {
	s.mu.RLock()
	pattern, exists := s.accessPatterns[key.EventHash]
	s.mu.RUnlock()
	
	if !exists || pattern.Confidence < 0.7 {
		return s.BaseStrategy.GetTTL(key)
	}
	
	// Extend TTL to cover predicted next access
	timeUntilNext := time.Until(pattern.PredictedNext)
	if timeUntilNext > 0 {
		// Add buffer
		return timeUntilNext + 5*time.Minute
	}
	
	return s.BaseStrategy.GetTTL(key)
}

func (s *PredictiveStrategy) OnHit(key ValidationCacheKey, entry *ValidationCacheEntry) {
	s.BaseStrategy.OnHit(key, entry)
	s.updatePattern(key.EventHash, time.Now())
}

func (s *PredictiveStrategy) OnMiss(key ValidationCacheKey) {
	s.BaseStrategy.OnMiss(key)
	s.updatePattern(key.EventHash, time.Now())
}

func (s *PredictiveStrategy) Score(entry *ValidationCacheEntry) float64 {
	baseScore := s.BaseStrategy.Score(entry)
	
	s.mu.RLock()
	pattern, exists := s.accessPatterns[entry.Key.EventHash]
	s.mu.RUnlock()
	
	if !exists {
		return baseScore
	}
	
	// Boost score if predicted to be accessed soon
	if pattern.Confidence > 0.7 && time.Until(pattern.PredictedNext) < 30*time.Minute {
		predictiveBoost := pattern.Confidence
		return baseScore*0.6 + predictiveBoost*0.4
	}
	
	return baseScore
}

func (s *PredictiveStrategy) updatePattern(eventHash string, accessTime time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	pattern, exists := s.accessPatterns[eventHash]
	if !exists {
		pattern = &AccessPattern{
			AccessTimes: make([]time.Time, 0),
		}
		s.accessPatterns[eventHash] = pattern
	}
	
	// Add access time
	pattern.LastAccess = accessTime
	pattern.AccessTimes = append(pattern.AccessTimes, accessTime)
	
	// Keep only recent accesses
	cutoff := time.Now().Add(-s.PatternWindow)
	filtered := make([]time.Time, 0)
	for _, t := range pattern.AccessTimes {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	pattern.AccessTimes = filtered
	
	// Predict next access if we have enough data
	if len(pattern.AccessTimes) >= s.MinPatternCount {
		s.predictNextAccess(pattern)
	}
}

func (s *PredictiveStrategy) predictNextAccess(pattern *AccessPattern) {
	// Simple prediction based on average interval
	if len(pattern.AccessTimes) < 2 {
		return
	}
	
	var totalInterval time.Duration
	for i := 1; i < len(pattern.AccessTimes); i++ {
		interval := pattern.AccessTimes[i].Sub(pattern.AccessTimes[i-1])
		totalInterval += interval
	}
	
	avgInterval := totalInterval / time.Duration(len(pattern.AccessTimes)-1)
	
	// Calculate confidence based on interval consistency
	var variance float64
	avgSeconds := avgInterval.Seconds()
	for i := 1; i < len(pattern.AccessTimes); i++ {
		interval := pattern.AccessTimes[i].Sub(pattern.AccessTimes[i-1]).Seconds()
		variance += math.Pow(interval-avgSeconds, 2)
	}
	variance /= float64(len(pattern.AccessTimes) - 1)
	stdDev := math.Sqrt(variance)
	
	// Confidence inversely proportional to coefficient of variation
	coeffOfVariation := stdDev / avgSeconds
	pattern.Confidence = math.Max(0, 1.0-coeffOfVariation)
	
	// Predict next access
	pattern.PredictedNext = pattern.LastAccess.Add(avgInterval)
}

// CompositeStrategy combines multiple strategies
type CompositeStrategy struct {
	Strategies []CacheStrategy
	Weights    []float64
}

// NewCompositeStrategy creates a new composite strategy
func NewCompositeStrategy(strategies []CacheStrategy, weights []float64) (*CompositeStrategy, error) {
	if len(strategies) != len(weights) {
		return nil, fmt.Errorf("strategies and weights must have the same length")
	}
	
	// Normalize weights
	var sum float64
	for _, w := range weights {
		sum += w
	}
	
	normalizedWeights := make([]float64, len(weights))
	for i, w := range weights {
		normalizedWeights[i] = w / sum
	}
	
	return &CompositeStrategy{
		Strategies: strategies,
		Weights:    normalizedWeights,
	}, nil
}

func (s *CompositeStrategy) ShouldCache(key ValidationCacheKey, result ValidationResult) bool {
	// All strategies must agree to cache
	for _, strategy := range s.Strategies {
		if !strategy.ShouldCache(key, result) {
			return false
		}
	}
	return true
}

func (s *CompositeStrategy) GetTTL(key ValidationCacheKey) time.Duration {
	// Weighted average of TTLs
	var weightedTTL time.Duration
	for i, strategy := range s.Strategies {
		ttl := strategy.GetTTL(key)
		weightedTTL += time.Duration(float64(ttl) * s.Weights[i])
	}
	return weightedTTL
}

func (s *CompositeStrategy) OnHit(key ValidationCacheKey, entry *ValidationCacheEntry) {
	for _, strategy := range s.Strategies {
		strategy.OnHit(key, entry)
	}
}

func (s *CompositeStrategy) OnMiss(key ValidationCacheKey) {
	for _, strategy := range s.Strategies {
		strategy.OnMiss(key)
	}
}

func (s *CompositeStrategy) Score(entry *ValidationCacheEntry) float64 {
	// Weighted average of scores
	var weightedScore float64
	for i, strategy := range s.Strategies {
		score := strategy.Score(entry)
		weightedScore += score * s.Weights[i]
	}
	return weightedScore
}