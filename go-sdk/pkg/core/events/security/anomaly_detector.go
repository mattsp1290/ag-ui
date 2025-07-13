package security

import (
	"math"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// Anomaly represents a detected anomaly
type Anomaly struct {
	Type      string                 `json:"type"`
	Score     float64                `json:"score"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details"`
}

// AnomalyDetector detects anomalous patterns in events
type AnomalyDetector struct {
	config          *SecurityConfig
	eventHistory    *CircularBuffer
	patternAnalyzer *PatternAnalyzer
	statisticsCache *StatisticsCache
	mutex           sync.RWMutex
}

// CircularBuffer stores recent events for analysis
type CircularBuffer struct {
	events    []EventRecord
	capacity  int
	position  int
	mutex     sync.RWMutex
}

// EventRecord stores event data for anomaly detection
type EventRecord struct {
	Event     events.Event
	Timestamp time.Time
	Source    string
	Size      int
}

// PatternAnalyzer analyzes patterns in events
type PatternAnalyzer struct {
	patterns      map[string]*Pattern
	patternWindow time.Duration
	mutex         sync.RWMutex
}

// Pattern represents an event pattern
type Pattern struct {
	Name       string
	Count      int
	LastSeen   time.Time
	Frequency  float64
	IsAnomaly  bool
}

// StatisticsCache caches statistical calculations
type StatisticsCache struct {
	mean              float64
	stdDev            float64
	lastCalculated    time.Time
	calculationWindow time.Duration
	mutex             sync.RWMutex
}

// NewAnomalyDetector creates a new anomaly detector
func NewAnomalyDetector(config *SecurityConfig) *AnomalyDetector {
	return &AnomalyDetector{
		config: config,
		eventHistory: &CircularBuffer{
			events:   make([]EventRecord, 1000),
			capacity: 1000,
		},
		patternAnalyzer: &PatternAnalyzer{
			patterns:      make(map[string]*Pattern),
			patternWindow: config.AnomalyWindowSize,
		},
		statisticsCache: &StatisticsCache{
			calculationWindow: 5 * time.Minute,
		},
	}
}

// DetectAnomaly checks if an event represents an anomaly
func (ad *AnomalyDetector) DetectAnomaly(event events.Event, context *events.ValidationContext) *Anomaly {
	if !ad.config.EnableAnomalyDetection {
		return nil
	}
	
	ad.mutex.Lock()
	defer ad.mutex.Unlock()
	
	// Record event
	record := EventRecord{
		Event:     event,
		Timestamp: time.Now(),
		Source:    ad.extractSource(context),
		Size:      ad.calculateEventSize(event),
	}
	ad.eventHistory.Add(record)
	
	// Update patterns
	ad.patternAnalyzer.UpdatePattern(event)
	
	// Check various anomaly types
	if anomaly := ad.checkFrequencyAnomaly(event); anomaly != nil {
		return anomaly
	}
	
	if anomaly := ad.checkSizeAnomaly(record); anomaly != nil {
		return anomaly
	}
	
	if anomaly := ad.checkPatternAnomaly(event); anomaly != nil {
		return anomaly
	}
	
	if anomaly := ad.checkBurstAnomaly(); anomaly != nil {
		return anomaly
	}
	
	return nil
}

// checkFrequencyAnomaly checks for unusual event frequency
func (ad *AnomalyDetector) checkFrequencyAnomaly(event events.Event) *Anomaly {
	recentEvents := ad.eventHistory.GetRecent(100)
	eventTypeCount := 0
	
	for _, record := range recentEvents {
		if record.Event != nil && record.Event.Type() == event.Type() {
			eventTypeCount++
		}
	}
	
	// Calculate z-score
	stats := ad.statisticsCache.GetStats()
	if stats.stdDev > 0 {
		zScore := math.Abs(float64(eventTypeCount)-stats.mean) / stats.stdDev
		
		if zScore > ad.config.AnomalyThreshold {
			return &Anomaly{
				Type:      "frequency_anomaly",
				Score:     zScore,
				Timestamp: time.Now(),
				Details: map[string]interface{}{
					"event_type":  event.Type(),
					"count":       eventTypeCount,
					"mean":        stats.mean,
					"std_dev":     stats.stdDev,
					"z_score":     zScore,
				},
			}
		}
	}
	
	return nil
}

// checkSizeAnomaly checks for unusual event sizes
func (ad *AnomalyDetector) checkSizeAnomaly(record EventRecord) *Anomaly {
	recentEvents := ad.eventHistory.GetRecent(100)
	
	// Calculate average size
	totalSize := 0
	count := 0
	for _, r := range recentEvents {
		if r.Event != nil {
			totalSize += r.Size
			count++
		}
	}
	
	if count == 0 {
		return nil
	}
	
	avgSize := float64(totalSize) / float64(count)
	
	// Check if current size is anomalous
	if float64(record.Size) > avgSize*3 {
		return &Anomaly{
			Type:      "size_anomaly",
			Score:     float64(record.Size) / avgSize,
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"event_size": record.Size,
				"avg_size":   avgSize,
				"ratio":      float64(record.Size) / avgSize,
			},
		}
	}
	
	return nil
}

// checkPatternAnomaly checks for unusual patterns
func (ad *AnomalyDetector) checkPatternAnomaly(event events.Event) *Anomaly {
	pattern := ad.patternAnalyzer.GetPattern(string(event.Type()))
	if pattern != nil && pattern.IsAnomaly {
		return &Anomaly{
			Type:      "pattern_anomaly",
			Score:     0.8,
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"pattern":   pattern.Name,
				"frequency": pattern.Frequency,
				"last_seen": pattern.LastSeen,
			},
		}
	}
	
	return nil
}

// checkBurstAnomaly checks for burst patterns
func (ad *AnomalyDetector) checkBurstAnomaly() *Anomaly {
	recentEvents := ad.eventHistory.GetRecent(10)
	if len(recentEvents) < 10 {
		return nil
	}
	
	// Check time difference between events
	timeDiffs := make([]float64, 0, len(recentEvents)-1)
	for i := 1; i < len(recentEvents); i++ {
		diff := recentEvents[i].Timestamp.Sub(recentEvents[i-1].Timestamp).Seconds()
		timeDiffs = append(timeDiffs, diff)
	}
	
	// Calculate average time difference
	avgDiff := 0.0
	for _, diff := range timeDiffs {
		avgDiff += diff
	}
	avgDiff /= float64(len(timeDiffs))
	
	// Check for burst (very small time differences)
	if avgDiff < 0.1 { // Less than 100ms average
		return &Anomaly{
			Type:      "burst_anomaly",
			Score:     1.0 / avgDiff,
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"avg_interval_ms": avgDiff * 1000,
				"event_count":     len(recentEvents),
			},
		}
	}
	
	return nil
}

// UpdateConfig updates the anomaly detector configuration
func (ad *AnomalyDetector) UpdateConfig(config *SecurityConfig) {
	ad.mutex.Lock()
	defer ad.mutex.Unlock()
	
	ad.config = config
	ad.patternAnalyzer.patternWindow = config.AnomalyWindowSize
}

// extractSource extracts source information from context
func (ad *AnomalyDetector) extractSource(context *events.ValidationContext) string {
	if context == nil || context.Metadata == nil {
		return "unknown"
	}
	
	if source, ok := context.Metadata["source"].(string); ok {
		return source
	}
	
	return "unknown"
}

// calculateEventSize calculates the size of an event
func (ad *AnomalyDetector) calculateEventSize(event events.Event) int {
	// Simplified size calculation
	switch e := event.(type) {
	case *events.TextMessageContentEvent:
		return len(e.Delta)
	case *events.ToolCallArgsEvent:
		return len(e.Delta)
	case *events.RunErrorEvent:
		return len(e.Message)
	default:
		return 0
	}
}

// CircularBuffer methods

func (cb *CircularBuffer) Add(record EventRecord) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.events[cb.position] = record
	cb.position = (cb.position + 1) % cb.capacity
}

func (cb *CircularBuffer) GetRecent(count int) []EventRecord {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	
	if count > cb.capacity {
		count = cb.capacity
	}
	
	result := make([]EventRecord, 0, count)
	
	// Start from the most recent position and go backwards
	for i := 0; i < count; i++ {
		idx := (cb.position - 1 - i + cb.capacity) % cb.capacity
		if cb.events[idx].Event != nil {
			result = append(result, cb.events[idx])
		}
	}
	
	return result
}

// PatternAnalyzer methods

func (pa *PatternAnalyzer) UpdatePattern(event events.Event) {
	pa.mutex.Lock()
	defer pa.mutex.Unlock()
	
	patternKey := string(event.Type())
	
	if pattern, exists := pa.patterns[patternKey]; exists {
		pattern.Count++
		pattern.LastSeen = time.Now()
		
		// Update frequency
		elapsed := time.Since(pattern.LastSeen)
		if elapsed.Minutes() > 0 {
			pattern.Frequency = float64(pattern.Count) / elapsed.Minutes()
		}
		
		// Simple anomaly detection - if frequency suddenly increases
		if pattern.Frequency > 100 { // More than 100 per minute
			pattern.IsAnomaly = true
		}
	} else {
		pa.patterns[patternKey] = &Pattern{
			Name:      patternKey,
			Count:     1,
			LastSeen:  time.Now(),
			Frequency: 0,
			IsAnomaly: false,
		}
	}
	
	// Clean old patterns
	pa.cleanOldPatterns()
}

func (pa *PatternAnalyzer) GetPattern(key string) *Pattern {
	pa.mutex.RLock()
	defer pa.mutex.RUnlock()
	
	return pa.patterns[key]
}

func (pa *PatternAnalyzer) cleanOldPatterns() {
	cutoff := time.Now().Add(-pa.patternWindow)
	
	for key, pattern := range pa.patterns {
		if pattern.LastSeen.Before(cutoff) {
			delete(pa.patterns, key)
		}
	}
}

// StatisticsCache methods

type Statistics struct {
	mean   float64
	stdDev float64
}

func (sc *StatisticsCache) GetStats() Statistics {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()
	
	// Recalculate if cache is old
	if time.Since(sc.lastCalculated) > sc.calculationWindow {
		// In real implementation, recalculate from event history
		sc.mean = 10.0    // Placeholder
		sc.stdDev = 3.0   // Placeholder
		sc.lastCalculated = time.Now()
	}
	
	return Statistics{
		mean:   sc.mean,
		stdDev: sc.stdDev,
	}
}