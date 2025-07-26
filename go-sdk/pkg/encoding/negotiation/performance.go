package negotiation

import (
	"sync"
	"time"
)

// PerformanceMetrics contains performance data for a content type
type PerformanceMetrics struct {
	// EncodingTime is the average time to encode
	EncodingTime time.Duration
	// DecodingTime is the average time to decode
	DecodingTime time.Duration
	// PayloadSize is the average encoded payload size
	PayloadSize int64
	// SuccessRate is the success rate (0.0 to 1.0)
	SuccessRate float64
	// Throughput is the bytes per second throughput
	Throughput float64
	// MemoryUsage is the average memory usage in bytes
	MemoryUsage int64
	// CPUUsage is the average CPU usage percentage
	CPUUsage float64
	// LastUpdated is when metrics were last updated
	LastUpdated time.Time
}

// PerformanceTracker tracks performance metrics for content types
type PerformanceTracker struct {
	metrics map[string]*aggregatedMetrics
	mu      sync.RWMutex
}

// aggregatedMetrics stores aggregated performance data
type aggregatedMetrics struct {
	samples      int64
	totalEncTime time.Duration
	totalDecTime time.Duration
	totalSize    int64
	successCount int64
	failureCount int64
	totalMemory  int64
	totalCPU     float64
	lastUpdated  time.Time
	
	// Moving averages for better adaptability
	recentEncTimes []time.Duration
	recentDecTimes []time.Duration
	recentSizes    []int64
	windowSize     int
}

// NewPerformanceTracker creates a new performance tracker
func NewPerformanceTracker() *PerformanceTracker {
	return &PerformanceTracker{
		metrics: make(map[string]*aggregatedMetrics),
	}
}

// UpdateMetrics updates performance metrics for a content type
func (pt *PerformanceTracker) UpdateMetrics(contentType string, metrics PerformanceMetrics) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	agg, exists := pt.metrics[contentType]
	if !exists {
		agg = &aggregatedMetrics{
			windowSize:     100, // Keep last 100 samples for moving average
			recentEncTimes: make([]time.Duration, 0, 100),
			recentDecTimes: make([]time.Duration, 0, 100),
			recentSizes:    make([]int64, 0, 100),
		}
		pt.metrics[contentType] = agg
	}

	// Update aggregated metrics
	agg.samples++
	agg.totalEncTime += metrics.EncodingTime
	agg.totalDecTime += metrics.DecodingTime
	agg.totalSize += metrics.PayloadSize
	agg.totalMemory += metrics.MemoryUsage
	agg.totalCPU += metrics.CPUUsage
	agg.lastUpdated = time.Now()

	// Update success/failure counts
	if metrics.SuccessRate >= 1.0 {
		agg.successCount++
	} else if metrics.SuccessRate <= 0.0 {
		agg.failureCount++
	} else {
		// Partial success - weighted update
		agg.successCount += int64(metrics.SuccessRate * 100)
		agg.failureCount += int64((1 - metrics.SuccessRate) * 100)
	}

	// Update moving averages
	agg.updateMovingAverages(metrics)
}

// updateMovingAverages updates the moving average windows
func (agg *aggregatedMetrics) updateMovingAverages(metrics PerformanceMetrics) {
	// Add to recent times
	agg.recentEncTimes = append(agg.recentEncTimes, metrics.EncodingTime)
	if len(agg.recentEncTimes) > agg.windowSize {
		agg.recentEncTimes = agg.recentEncTimes[1:]
	}

	agg.recentDecTimes = append(agg.recentDecTimes, metrics.DecodingTime)
	if len(agg.recentDecTimes) > agg.windowSize {
		agg.recentDecTimes = agg.recentDecTimes[1:]
	}

	agg.recentSizes = append(agg.recentSizes, metrics.PayloadSize)
	if len(agg.recentSizes) > agg.windowSize {
		agg.recentSizes = agg.recentSizes[1:]
	}
}

// GetMetrics returns current performance metrics for a content type
func (pt *PerformanceTracker) GetMetrics(contentType string) (PerformanceMetrics, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	agg, exists := pt.metrics[contentType]
	if !exists || agg.samples == 0 {
		return PerformanceMetrics{}, false
	}

	// Calculate current metrics
	metrics := PerformanceMetrics{
		EncodingTime: agg.getAverageEncTime(),
		DecodingTime: agg.getAverageDecTime(),
		PayloadSize:  agg.getAverageSize(),
		SuccessRate:  agg.getSuccessRate(),
		MemoryUsage:  agg.totalMemory / agg.samples,
		CPUUsage:     agg.totalCPU / float64(agg.samples),
		LastUpdated:  agg.lastUpdated,
	}

	// Calculate throughput
	totalTime := metrics.EncodingTime + metrics.DecodingTime
	if totalTime > 0 {
		metrics.Throughput = float64(metrics.PayloadSize) / totalTime.Seconds()
	}

	return metrics, true
}

// getAverageEncTime calculates average encoding time with preference for recent samples
func (agg *aggregatedMetrics) getAverageEncTime() time.Duration {
	if len(agg.recentEncTimes) > 10 {
		// Use recent average if we have enough samples
		var total time.Duration
		for _, t := range agg.recentEncTimes {
			total += t
		}
		return total / time.Duration(len(agg.recentEncTimes))
	}
	// Fall back to overall average
	if agg.samples > 0 {
		return agg.totalEncTime / time.Duration(agg.samples)
	}
	return 0
}

// getAverageDecTime calculates average decoding time with preference for recent samples
func (agg *aggregatedMetrics) getAverageDecTime() time.Duration {
	if len(agg.recentDecTimes) > 10 {
		// Use recent average if we have enough samples
		var total time.Duration
		for _, t := range agg.recentDecTimes {
			total += t
		}
		return total / time.Duration(len(agg.recentDecTimes))
	}
	// Fall back to overall average
	if agg.samples > 0 {
		return agg.totalDecTime / time.Duration(agg.samples)
	}
	return 0
}

// getAverageSize calculates average payload size with preference for recent samples
func (agg *aggregatedMetrics) getAverageSize() int64 {
	if len(agg.recentSizes) > 10 {
		// Use recent average if we have enough samples
		var total int64
		for _, s := range agg.recentSizes {
			total += s
		}
		return total / int64(len(agg.recentSizes))
	}
	// Fall back to overall average
	if agg.samples > 0 {
		return agg.totalSize / agg.samples
	}
	return 0
}

// getSuccessRate calculates the success rate
func (agg *aggregatedMetrics) getSuccessRate() float64 {
	total := agg.successCount + agg.failureCount
	if total == 0 {
		return 1.0 // Assume success if no data
	}
	return float64(agg.successCount) / float64(total)
}

// GetScore returns a performance score for a content type (0.0 to 1.0)
func (pt *PerformanceTracker) GetScore(contentType string) float64 {
	metrics, exists := pt.GetMetrics(contentType)
	if !exists {
		return 0.5 // Default score for unknown types
	}

	// Calculate composite score based on multiple factors
	score := 0.0

	// Success rate (40% weight)
	score += metrics.SuccessRate * 0.4

	// Speed score (30% weight) - faster is better
	speedScore := calculateSpeedScore(metrics.EncodingTime + metrics.DecodingTime)
	score += speedScore * 0.3

	// Size efficiency score (20% weight) - smaller is better
	sizeScore := calculateSizeScore(metrics.PayloadSize)
	score += sizeScore * 0.2

	// Resource usage score (10% weight) - lower is better
	resourceScore := calculateResourceScore(metrics.MemoryUsage, metrics.CPUUsage)
	score += resourceScore * 0.1

	return score
}

// calculateSpeedScore converts time duration to a 0-1 score
func calculateSpeedScore(duration time.Duration) float64 {
	// Convert to milliseconds
	ms := duration.Milliseconds()
	
	// Score mapping:
	// 0-10ms: 1.0
	// 10-50ms: 0.8-1.0
	// 50-100ms: 0.6-0.8
	// 100-500ms: 0.3-0.6
	// >500ms: 0.0-0.3
	
	switch {
	case ms <= 10:
		return 1.0
	case ms <= 50:
		return 1.0 - (float64(ms-10)/40)*0.2
	case ms <= 100:
		return 0.8 - (float64(ms-50)/50)*0.2
	case ms <= 500:
		return 0.6 - (float64(ms-100)/400)*0.3
	default:
		// Asymptotic approach to 0
		return 0.3 / (1 + float64(ms-500)/1000)
	}
}

// calculateSizeScore converts payload size to a 0-1 score
func calculateSizeScore(size int64) float64 {
	// Score mapping:
	// 0-1KB: 1.0
	// 1-10KB: 0.8-1.0
	// 10-100KB: 0.6-0.8
	// 100KB-1MB: 0.3-0.6
	// >1MB: 0.0-0.3
	
	kb := float64(size) / 1024
	
	switch {
	case kb <= 1:
		return 1.0
	case kb <= 10:
		return 1.0 - (kb-1)/9*0.2
	case kb <= 100:
		return 0.8 - (kb-10)/90*0.2
	case kb <= 1024:
		return 0.6 - (kb-100)/924*0.3
	default:
		// Asymptotic approach to 0
		return 0.3 / (1 + (kb-1024)/1024)
	}
}

// calculateResourceScore converts resource usage to a 0-1 score
func calculateResourceScore(memory int64, cpu float64) float64 {
	// Memory score (50% of resource score)
	memoryMB := float64(memory) / (1024 * 1024)
	memoryScore := 1.0
	switch {
	case memoryMB <= 10:
		memoryScore = 1.0
	case memoryMB <= 50:
		memoryScore = 1.0 - (memoryMB-10)/40*0.3
	case memoryMB <= 100:
		memoryScore = 0.7 - (memoryMB-50)/50*0.3
	default:
		memoryScore = 0.4 / (1 + (memoryMB-100)/100)
	}

	// CPU score (50% of resource score)
	cpuScore := 1.0
	switch {
	case cpu <= 10:
		cpuScore = 1.0
	case cpu <= 30:
		cpuScore = 1.0 - (cpu-10)/20*0.3
	case cpu <= 60:
		cpuScore = 0.7 - (cpu-30)/30*0.3
	default:
		cpuScore = 0.4 / (1 + (cpu-60)/40)
	}

	return (memoryScore + cpuScore) / 2
}

// Reset clears all performance metrics
func (pt *PerformanceTracker) Reset() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.metrics = make(map[string]*aggregatedMetrics)
}

// GetAllScores returns performance scores for all tracked content types
func (pt *PerformanceTracker) GetAllScores() map[string]float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	scores := make(map[string]float64)
	for contentType := range pt.metrics {
		scores[contentType] = pt.GetScore(contentType)
	}
	return scores
}

// Benchmark runs a benchmark for a content type
func (pt *PerformanceTracker) Benchmark(contentType string, benchFunc func() error) error {
	start := time.Now()
	
	// Run the benchmark function
	err := benchFunc()
	
	elapsed := time.Since(start)
	
	// Update metrics based on result
	metrics := PerformanceMetrics{
		EncodingTime: elapsed / 2, // Assume half time for encoding
		DecodingTime: elapsed / 2, // Assume half time for decoding
		SuccessRate:  1.0,
	}
	
	if err != nil {
		metrics.SuccessRate = 0.0
	}
	
	pt.UpdateMetrics(contentType, metrics)
	return err
}

// GetRecommendation returns a recommendation for the best performing format
func (pt *PerformanceTracker) GetRecommendation() (string, float64) {
	scores := pt.GetAllScores()
	
	var bestType string
	var bestScore float64
	
	for contentType, score := range scores {
		if score > bestScore {
			bestScore = score
			bestType = contentType
		}
	}
	
	return bestType, bestScore
}