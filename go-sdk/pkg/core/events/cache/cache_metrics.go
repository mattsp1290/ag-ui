package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector collects and aggregates cache metrics
type MetricsCollector struct {
	// Basic counters
	hits          uint64
	misses        uint64
	evictions     uint64
	expirations   uint64
	
	// Level-specific counters
	l1Hits        uint64
	l1Misses      uint64
	l2Hits        uint64
	l2Misses      uint64
	
	// Latency tracking
	hitLatencies  *LatencyTracker
	missLatencies *LatencyTracker
	setLatencies  *LatencyTracker
	
	// Size tracking
	currentSize   int64
	maxSize       int64
	totalBytes    int64
	
	// Time series data
	timeSeries    *TimeSeriesData
	
	// Histogram data
	sizeHistogram *Histogram
	ttlHistogram  *Histogram
	
	// Advanced metrics
	compressionStats *CompressionStats
	memoryStats      *MemoryStats
	
	// Configuration
	config        *MetricsConfig
	
	// Synchronization
	mu            sync.RWMutex
	shutdownCh    chan struct{}
	wg            sync.WaitGroup
}

// MetricsConfig contains configuration for metrics collection
type MetricsConfig struct {
	EnableDetailedLatency   bool
	EnableTimeSeries        bool
	EnableHistograms        bool
	EnableMemoryProfiling   bool
	TimeSeriesWindow        time.Duration
	HistogramBuckets        int
	ReportingInterval       time.Duration
	PercentilesToTrack      []float64
}

// DefaultMetricsConfig returns default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		EnableDetailedLatency:   true,
		EnableTimeSeries:        true,
		EnableHistograms:        true,
		EnableMemoryProfiling:   true,
		TimeSeriesWindow:        1 * time.Hour,
		HistogramBuckets:        20,
		ReportingInterval:       1 * time.Minute,
		PercentilesToTrack:      []float64{0.5, 0.75, 0.9, 0.95, 0.99},
	}
}

// LatencyTracker tracks operation latencies
type LatencyTracker struct {
	samples       []time.Duration
	maxSamples    int
	sum           time.Duration
	count         uint64
	min           time.Duration
	max           time.Duration
	mu            sync.Mutex
}

// TimeSeriesData maintains time series metrics
type TimeSeriesData struct {
	points        []*MetricPoint
	window        time.Duration
	resolution    time.Duration
	mu            sync.RWMutex
}

// MetricPoint represents a point in time series
type MetricPoint struct {
	Timestamp    time.Time
	HitRate      float64
	MissRate     float64
	EvictionRate float64
	AvgLatency   time.Duration
	Size         int64
	MemoryUsage  int64
}

// Histogram maintains histogram data
type Histogram struct {
	buckets      []uint64
	boundaries   []float64
	count        uint64
	sum          float64
	mu           sync.Mutex
}

// CompressionStats tracks compression performance
type CompressionStats struct {
	CompressedBytes   uint64
	UncompressedBytes uint64
	CompressionTime   time.Duration
	DecompressionTime time.Duration
	CompressionCount  uint64
	mu                sync.Mutex
}

// MemoryStats tracks memory usage
type MemoryStats struct {
	HeapAlloc      uint64
	HeapInUse      uint64
	StackInUse     uint64
	NumGC          uint32
	GCPauseTotal   time.Duration
	LastGC         time.Time
	mu             sync.RWMutex
}

// CacheMetricsReport represents a comprehensive metrics report
type CacheMetricsReport struct {
	Timestamp         time.Time                 `json:"timestamp"`
	BasicMetrics      *BasicMetrics             `json:"basic_metrics"`
	LatencyMetrics    *LatencyMetrics           `json:"latency_metrics"`
	SizeMetrics       *SizeMetrics              `json:"size_metrics"`
	PerformanceMetrics *PerformanceMetrics      `json:"performance_metrics"`
	HealthMetrics     *HealthMetrics            `json:"health_metrics"`
	Recommendations   []string                  `json:"recommendations"`
}

// BasicMetrics contains basic cache metrics
type BasicMetrics struct {
	Hits          uint64  `json:"hits"`
	Misses        uint64  `json:"misses"`
	HitRate       float64 `json:"hit_rate"`
	Evictions     uint64  `json:"evictions"`
	Expirations   uint64  `json:"expirations"`
	L1HitRate     float64 `json:"l1_hit_rate"`
	L2HitRate     float64 `json:"l2_hit_rate"`
}

// LatencyMetrics contains latency-related metrics
type LatencyMetrics struct {
	AvgHitLatency    time.Duration            `json:"avg_hit_latency"`
	AvgMissLatency   time.Duration            `json:"avg_miss_latency"`
	AvgSetLatency    time.Duration            `json:"avg_set_latency"`
	HitPercentiles   map[string]time.Duration `json:"hit_percentiles"`
	MissPercentiles  map[string]time.Duration `json:"miss_percentiles"`
}

// SizeMetrics contains size-related metrics
type SizeMetrics struct {
	CurrentSize        int64   `json:"current_size"`
	MaxSize            int64   `json:"max_size"`
	Utilization        float64 `json:"utilization"`
	AvgEntrySize       int64   `json:"avg_entry_size"`
	TotalBytes         int64   `json:"total_bytes"`
	CompressionRatio   float64 `json:"compression_ratio"`
}

// PerformanceMetrics contains performance metrics
type PerformanceMetrics struct {
	ThroughputPerSec   float64       `json:"throughput_per_sec"`
	OperationsPerSec   float64       `json:"operations_per_sec"`
	CacheMissesPerSec  float64       `json:"cache_misses_per_sec"`
	EvictionsPerSec    float64       `json:"evictions_per_sec"`
	ResponseTime       time.Duration `json:"response_time"`
}

// HealthMetrics contains cache health indicators
type HealthMetrics struct {
	HealthScore        float64              `json:"health_score"`
	MemoryPressure     float64              `json:"memory_pressure"`
	EvictionPressure   float64              `json:"eviction_pressure"`
	FragmentationRatio float64              `json:"fragmentation_ratio"`
	ErrorRate          float64              `json:"error_rate"`
	Issues             []string             `json:"issues"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(config *MetricsConfig) *MetricsCollector {
	if config == nil {
		config = DefaultMetricsConfig()
	}
	
	mc := &MetricsCollector{
		config:           config,
		hitLatencies:     NewLatencyTracker(10000),
		missLatencies:    NewLatencyTracker(10000),
		setLatencies:     NewLatencyTracker(10000),
		compressionStats: &CompressionStats{},
		memoryStats:      &MemoryStats{},
		shutdownCh:       make(chan struct{}),
	}
	
	if config.EnableTimeSeries {
		mc.timeSeries = NewTimeSeriesData(config.TimeSeriesWindow, 1*time.Minute)
	}
	
	if config.EnableHistograms {
		mc.sizeHistogram = NewHistogram(config.HistogramBuckets, 0, 10*1024*1024) // 0-10MB
		mc.ttlHistogram = NewHistogram(config.HistogramBuckets, 0, 3600)        // 0-1hour
	}
	
	// Start background workers
	mc.wg.Add(1)
	go mc.aggregationWorker()
	
	if config.EnableMemoryProfiling {
		mc.wg.Add(1)
		go mc.memoryProfilingWorker()
	}
	
	return mc
}

// RecordHit records a cache hit
func (mc *MetricsCollector) RecordHit(level CacheLevel, latency time.Duration) {
	atomic.AddUint64(&mc.hits, 1)
	
	switch level {
	case L1Cache:
		atomic.AddUint64(&mc.l1Hits, 1)
	case L2Cache:
		atomic.AddUint64(&mc.l2Hits, 1)
	}
	
	if mc.config != nil && mc.config.EnableDetailedLatency && mc.hitLatencies != nil {
		mc.hitLatencies.Record(latency)
	}
}

// RecordMiss records a cache miss
func (mc *MetricsCollector) RecordMiss(latency time.Duration) {
	atomic.AddUint64(&mc.misses, 1)
	atomic.AddUint64(&mc.l1Misses, 1)
	
	if mc.config != nil && mc.config.EnableDetailedLatency && mc.missLatencies != nil {
		mc.missLatencies.Record(latency)
	}
}

// RecordSet records a cache set operation
func (mc *MetricsCollector) RecordSet(latency time.Duration, size int64) {
	if mc.config != nil && mc.config.EnableDetailedLatency && mc.setLatencies != nil {
		mc.setLatencies.Record(latency)
	}
	
	atomic.AddInt64(&mc.totalBytes, size)
	
	if mc.config != nil && mc.config.EnableHistograms && mc.sizeHistogram != nil {
		mc.sizeHistogram.Record(float64(size))
	}
}

// RecordEviction records a cache eviction
func (mc *MetricsCollector) RecordEviction() {
	atomic.AddUint64(&mc.evictions, 1)
}

// RecordExpiration records a cache expiration
func (mc *MetricsCollector) RecordExpiration() {
	atomic.AddUint64(&mc.expirations, 1)
}

// RecordCompression records compression stats
func (mc *MetricsCollector) RecordCompression(uncompressed, compressed uint64, duration time.Duration) {
	mc.compressionStats.mu.Lock()
	defer mc.compressionStats.mu.Unlock()
	
	mc.compressionStats.UncompressedBytes += uncompressed
	mc.compressionStats.CompressedBytes += compressed
	mc.compressionStats.CompressionTime += duration
	mc.compressionStats.CompressionCount++
}

// UpdateSize updates the current cache size
func (mc *MetricsCollector) UpdateSize(current, max int64) {
	atomic.StoreInt64(&mc.currentSize, current)
	atomic.StoreInt64(&mc.maxSize, max)
}

// GetReport generates a comprehensive metrics report
func (mc *MetricsCollector) GetReport() *CacheMetricsReport {
	report := &CacheMetricsReport{
		Timestamp:          time.Now(),
		BasicMetrics:       mc.getBasicMetrics(),
		LatencyMetrics:     mc.getLatencyMetrics(),
		SizeMetrics:        mc.getSizeMetrics(),
		PerformanceMetrics: mc.getPerformanceMetrics(),
		HealthMetrics:      mc.getHealthMetrics(),
		Recommendations:    mc.generateRecommendations(),
	}
	
	return report
}

// GetTimeSeries returns time series data
func (mc *MetricsCollector) GetTimeSeries(duration time.Duration) []*MetricPoint {
	if mc.timeSeries == nil {
		return nil
	}
	
	return mc.timeSeries.GetPoints(duration)
}

// Shutdown shuts down the metrics collector
func (mc *MetricsCollector) Shutdown(ctx context.Context) error {
	close(mc.shutdownCh)
	
	done := make(chan struct{})
	go func() {
		mc.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Private methods

func (mc *MetricsCollector) getBasicMetrics() *BasicMetrics {
	hits := atomic.LoadUint64(&mc.hits)
	misses := atomic.LoadUint64(&mc.misses)
	total := hits + misses
	
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	
	l1Hits := atomic.LoadUint64(&mc.l1Hits)
	l1Misses := atomic.LoadUint64(&mc.l1Misses)
	l1Total := l1Hits + l1Misses
	
	l1HitRate := float64(0)
	if l1Total > 0 {
		l1HitRate = float64(l1Hits) / float64(l1Total)
	}
	
	l2Hits := atomic.LoadUint64(&mc.l2Hits)
	l2Total := l2Hits + (l1Misses - l2Hits) // L2 misses = L1 misses - L2 hits
	
	l2HitRate := float64(0)
	if l2Total > 0 {
		l2HitRate = float64(l2Hits) / float64(l2Total)
	}
	
	return &BasicMetrics{
		Hits:        hits,
		Misses:      misses,
		HitRate:     hitRate,
		Evictions:   atomic.LoadUint64(&mc.evictions),
		Expirations: atomic.LoadUint64(&mc.expirations),
		L1HitRate:   l1HitRate,
		L2HitRate:   l2HitRate,
	}
}

func (mc *MetricsCollector) getLatencyMetrics() *LatencyMetrics {
	metrics := &LatencyMetrics{
		HitPercentiles:  make(map[string]time.Duration),
		MissPercentiles: make(map[string]time.Duration),
	}
	
	if mc.config.EnableDetailedLatency && mc.hitLatencies != nil && mc.missLatencies != nil && mc.setLatencies != nil {
		metrics.AvgHitLatency = mc.hitLatencies.Average()
		metrics.AvgMissLatency = mc.missLatencies.Average()
		metrics.AvgSetLatency = mc.setLatencies.Average()
		
		// Calculate percentiles
		if mc.config.PercentilesToTrack != nil {
			for _, p := range mc.config.PercentilesToTrack {
				pStr := fmt.Sprintf("p%.0f", p*100)
				metrics.HitPercentiles[pStr] = mc.hitLatencies.Percentile(p)
				metrics.MissPercentiles[pStr] = mc.missLatencies.Percentile(p)
			}
		}
	}
	
	return metrics
}

func (mc *MetricsCollector) getSizeMetrics() *SizeMetrics {
	currentSize := atomic.LoadInt64(&mc.currentSize)
	maxSize := atomic.LoadInt64(&mc.maxSize)
	
	utilization := float64(0)
	if maxSize > 0 {
		utilization = float64(currentSize) / float64(maxSize)
	}
	
	avgEntrySize := int64(0)
	if currentSize > 0 {
		totalBytes := atomic.LoadInt64(&mc.totalBytes)
		avgEntrySize = totalBytes / currentSize
	}
	
	compressionRatio := float64(1.0)
	mc.compressionStats.mu.Lock()
	if mc.compressionStats.UncompressedBytes > 0 {
		compressionRatio = float64(mc.compressionStats.CompressedBytes) / float64(mc.compressionStats.UncompressedBytes)
	}
	mc.compressionStats.mu.Unlock()
	
	return &SizeMetrics{
		CurrentSize:      currentSize,
		MaxSize:          maxSize,
		Utilization:      utilization,
		AvgEntrySize:     avgEntrySize,
		TotalBytes:       atomic.LoadInt64(&mc.totalBytes),
		CompressionRatio: compressionRatio,
	}
}

func (mc *MetricsCollector) getPerformanceMetrics() *PerformanceMetrics {
	responseTime := time.Duration(0)
	if mc.hitLatencies != nil {
		responseTime = mc.hitLatencies.Average()
	}
	
	// TODO: Calculate actual rates based on time windows
	return &PerformanceMetrics{
		ThroughputPerSec:  0,
		OperationsPerSec:  0,
		CacheMissesPerSec: 0,
		EvictionsPerSec:   0,
		ResponseTime:      responseTime,
	}
}

func (mc *MetricsCollector) getHealthMetrics() *HealthMetrics {
	health := &HealthMetrics{
		Issues: make([]string, 0),
	}
	
	// Calculate health score
	basicMetrics := mc.getBasicMetrics()
	sizeMetrics := mc.getSizeMetrics()
	
	// Base score starts at 100
	score := float64(100)
	
	// Penalize low hit rate
	if basicMetrics.HitRate < 0.5 {
		score -= 20
		health.Issues = append(health.Issues, "Low hit rate")
	} else if basicMetrics.HitRate < 0.7 {
		score -= 10
		health.Issues = append(health.Issues, "Suboptimal hit rate")
	}
	
	// Check memory pressure
	health.MemoryPressure = sizeMetrics.Utilization
	if sizeMetrics.Utilization > 0.9 {
		score -= 15
		health.Issues = append(health.Issues, "High memory pressure")
	}
	
	// Check eviction pressure
	evictions := atomic.LoadUint64(&mc.evictions)
	hits := atomic.LoadUint64(&mc.hits)
	if hits > 0 {
		health.EvictionPressure = float64(evictions) / float64(hits)
		if health.EvictionPressure > 0.1 {
			score -= 10
			health.Issues = append(health.Issues, "High eviction rate")
		}
	}
	
	// TODO: Calculate fragmentation ratio
	health.FragmentationRatio = 0
	
	// TODO: Calculate error rate
	health.ErrorRate = 0
	
	health.HealthScore = math.Max(0, score)
	
	return health
}

func (mc *MetricsCollector) generateRecommendations() []string {
	recommendations := make([]string, 0)
	
	basicMetrics := mc.getBasicMetrics()
	sizeMetrics := mc.getSizeMetrics()
	latencyMetrics := mc.getLatencyMetrics()
	
	// Hit rate recommendations
	if basicMetrics.HitRate < 0.5 {
		recommendations = append(recommendations, "Consider increasing cache size to improve hit rate")
		recommendations = append(recommendations, "Review cache key patterns and access patterns")
	}
	
	// Size recommendations
	if sizeMetrics.Utilization > 0.9 {
		recommendations = append(recommendations, "Cache is near capacity, consider increasing max size")
	}
	
	// Eviction recommendations
	evictionRate := float64(basicMetrics.Evictions) / float64(basicMetrics.Hits+basicMetrics.Misses)
	if evictionRate > 0.1 {
		recommendations = append(recommendations, "High eviction rate detected, consider adjusting eviction policy")
	}
	
	// Latency recommendations
	if latencyMetrics.AvgMissLatency > 100*time.Millisecond {
		recommendations = append(recommendations, "High miss latency, consider implementing prefetching")
	}
	
	// L1/L2 recommendations
	if basicMetrics.L1HitRate < 0.8 && basicMetrics.L2HitRate > 0.5 {
		recommendations = append(recommendations, "Consider increasing L1 cache size for better performance")
	}
	
	return recommendations
}

// Background workers

func (mc *MetricsCollector) aggregationWorker() {
	defer mc.wg.Done()
	
	ticker := time.NewTicker(mc.config.ReportingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-mc.shutdownCh:
			return
		case <-ticker.C:
			mc.aggregateMetrics()
		}
	}
}

func (mc *MetricsCollector) memoryProfilingWorker() {
	defer mc.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-mc.shutdownCh:
			return
		case <-ticker.C:
			mc.updateMemoryStats()
		}
	}
}

func (mc *MetricsCollector) aggregateMetrics() {
	if mc.timeSeries == nil {
		return
	}
	
	// Create metric point
	point := &MetricPoint{
		Timestamp:  time.Now(),
		HitRate:    mc.getBasicMetrics().HitRate,
		Size:       atomic.LoadInt64(&mc.currentSize),
	}
	
	// Only add average latency if hitLatencies is initialized
	if mc.hitLatencies != nil {
		point.AvgLatency = mc.hitLatencies.Average()
	}
	
	// TODO: Add more metrics to time series
	
	mc.timeSeries.AddPoint(point)
}

func (mc *MetricsCollector) updateMemoryStats() {
	// TODO: Implement memory profiling
}

// LatencyTracker implementation

// NewLatencyTracker creates a new latency tracker
func NewLatencyTracker(maxSamples int) *LatencyTracker {
	return &LatencyTracker{
		samples:    make([]time.Duration, 0, maxSamples),
		maxSamples: maxSamples,
		min:        time.Duration(math.MaxInt64),
	}
}

// Record records a latency sample
func (lt *LatencyTracker) Record(latency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	
	lt.samples = append(lt.samples, latency)
	if len(lt.samples) > lt.maxSamples {
		lt.samples = lt.samples[1:]
	}
	
	lt.sum += latency
	lt.count++
	
	if latency < lt.min {
		lt.min = latency
	}
	if latency > lt.max {
		lt.max = latency
	}
}

// Average returns the average latency
func (lt *LatencyTracker) Average() time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	
	if lt.count == 0 {
		return 0
	}
	
	return lt.sum / time.Duration(lt.count)
}

// Percentile returns the percentile latency
func (lt *LatencyTracker) Percentile(p float64) time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	
	if len(lt.samples) == 0 {
		return 0
	}
	
	// Copy and sort samples
	sorted := make([]time.Duration, len(lt.samples))
	copy(sorted, lt.samples)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	
	index := int(float64(len(sorted)-1) * p)
	return sorted[index]
}

// TimeSeriesData implementation

// NewTimeSeriesData creates new time series data
func NewTimeSeriesData(window, resolution time.Duration) *TimeSeriesData {
	return &TimeSeriesData{
		points:     make([]*MetricPoint, 0),
		window:     window,
		resolution: resolution,
	}
}

// AddPoint adds a metric point
func (ts *TimeSeriesData) AddPoint(point *MetricPoint) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	
	ts.points = append(ts.points, point)
	
	// Remove old points
	cutoff := time.Now().Add(-ts.window)
	newPoints := make([]*MetricPoint, 0)
	for _, p := range ts.points {
		if p.Timestamp.After(cutoff) {
			newPoints = append(newPoints, p)
		}
	}
	ts.points = newPoints
}

// GetPoints returns points within duration
func (ts *TimeSeriesData) GetPoints(duration time.Duration) []*MetricPoint {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	cutoff := time.Now().Add(-duration)
	result := make([]*MetricPoint, 0)
	
	for _, p := range ts.points {
		if p.Timestamp.After(cutoff) {
			result = append(result, p)
		}
	}
	
	return result
}

// Histogram implementation

// NewHistogram creates a new histogram
func NewHistogram(buckets int, min, max float64) *Histogram {
	h := &Histogram{
		buckets:    make([]uint64, buckets),
		boundaries: make([]float64, buckets+1),
	}
	
	// Create bucket boundaries
	step := (max - min) / float64(buckets)
	for i := 0; i <= buckets; i++ {
		h.boundaries[i] = min + float64(i)*step
	}
	
	return h
}

// Record records a value in the histogram
func (h *Histogram) Record(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Find appropriate bucket
	bucket := 0
	for i := 1; i < len(h.boundaries); i++ {
		if value < h.boundaries[i] {
			bucket = i - 1
			break
		}
	}
	
	// Handle values beyond max
	if bucket == 0 && value >= h.boundaries[len(h.boundaries)-1] {
		bucket = len(h.buckets) - 1
	}
	
	h.buckets[bucket]++
	h.count++
	h.sum += value
}

// ToJSON converts histogram to JSON
func (h *Histogram) ToJSON() ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	average := float64(0)
	if h.count > 0 {
		average = h.sum / float64(h.count)
	}
	
	data := map[string]interface{}{
		"buckets":    h.buckets,
		"boundaries": h.boundaries,
		"count":      h.count,
		"sum":        h.sum,
		"average":    average,
	}
	
	return json.Marshal(data)
}