package analytics

import (
	"fmt"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// SimpleAnalyticsEngine provides basic analytics capabilities
type SimpleAnalyticsEngine struct {
	mu       sync.RWMutex
	config   *SimpleAnalyticsConfig
	buffer   *SimpleEventBuffer
	metrics  *SimpleMetrics
	patterns map[string]*SimplePattern
}

// SimpleAnalyticsConfig holds basic configuration
type SimpleAnalyticsConfig struct {
	BufferSize      int           `json:"buffer_size"`
	AnalysisWindow  time.Duration `json:"analysis_window"`
	MinPatternCount int           `json:"min_pattern_count"`
}

// SimpleEventBuffer stores recent events for analysis
type SimpleEventBuffer struct {
	mu      sync.RWMutex
	events  []events.Event
	maxSize int
}

// SimpleMetrics tracks basic analytics metrics
type SimpleMetrics struct {
	EventsProcessed   int64     `json:"events_processed"`
	PatternsDetected  int64     `json:"patterns_detected"`
	AnomaliesDetected int64     `json:"anomalies_detected"`
	LastUpdate        time.Time `json:"last_update"`
}

// SimplePattern represents a basic pattern
type SimplePattern struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	EventType events.EventType `json:"event_type"`
	Count     int              `json:"count"`
	LastSeen  time.Time        `json:"last_seen"`
	Window    time.Duration    `json:"window"`
}

// SimpleAnalyticsResult represents analysis results
type SimpleAnalyticsResult struct {
	EventType      events.EventType `json:"event_type"`
	Timestamp      time.Time        `json:"timestamp"`
	PatternsFound  []string         `json:"patterns_found"`
	IsAnomaly      bool             `json:"is_anomaly"`
	AnomalyScore   float64          `json:"anomaly_score"`
	ProcessingTime time.Duration    `json:"processing_time"`
}

// NewSimpleAnalyticsEngine creates a new simple analytics engine
func NewSimpleAnalyticsEngine(config *SimpleAnalyticsConfig) *SimpleAnalyticsEngine {
	if config == nil {
		config = &SimpleAnalyticsConfig{
			BufferSize:      1000,
			AnalysisWindow:  5 * time.Minute,
			MinPatternCount: 3,
		}
	}

	return &SimpleAnalyticsEngine{
		config:   config,
		buffer:   NewSimpleEventBuffer(config.BufferSize),
		metrics:  &SimpleMetrics{},
		patterns: make(map[string]*SimplePattern),
	}
}

// NewSimpleEventBuffer creates a new event buffer
func NewSimpleEventBuffer(maxSize int) *SimpleEventBuffer {
	return &SimpleEventBuffer{
		events:  make([]events.Event, 0),
		maxSize: maxSize,
	}
}

// Add adds an event to the buffer
func (b *SimpleEventBuffer) Add(event events.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events = append(b.events, event)

	// Keep only recent events
	if len(b.events) > b.maxSize {
		b.events = b.events[len(b.events)-b.maxSize:]
	}
}

// GetRecent returns recent events within the specified duration
func (b *SimpleEventBuffer) GetRecent(duration time.Duration) []events.Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var recent []events.Event

	for _, event := range b.events {
		if ts := event.Timestamp(); ts != nil {
			eventTime := time.Unix(*ts/1000, (*ts%1000)*1000000)
			if eventTime.After(cutoff) {
				recent = append(recent, event)
			}
		}
	}

	return recent
}

// AnalyzeEvent analyzes a single event
func (engine *SimpleAnalyticsEngine) AnalyzeEvent(event events.Event) (*SimpleAnalyticsResult, error) {
	startTime := time.Now()

	engine.mu.Lock()
	defer engine.mu.Unlock()

	// Add to buffer
	engine.buffer.Add(event)

	result := &SimpleAnalyticsResult{
		EventType:      event.Type(),
		Timestamp:      time.Now(),
		PatternsFound:  make([]string, 0),
		ProcessingTime: time.Since(startTime),
	}

	// Simple pattern detection
	patterns := engine.detectSimplePatterns(event)
	result.PatternsFound = patterns

	// Simple anomaly detection
	isAnomaly, score := engine.detectSimpleAnomaly(event)
	result.IsAnomaly = isAnomaly
	result.AnomalyScore = score

	// Update metrics
	engine.updateSimpleMetrics(result)

	result.ProcessingTime = time.Since(startTime)
	return result, nil
}

// detectSimplePatterns performs basic pattern detection
func (engine *SimpleAnalyticsEngine) detectSimplePatterns(event events.Event) []string {
	var patterns []string

	eventType := event.Type()
	recentEvents := engine.buffer.GetRecent(engine.config.AnalysisWindow)

	// Count events of same type
	count := 0
	for _, recentEvent := range recentEvents {
		if recentEvent.Type() == eventType {
			count++
		}
	}

	// Update or create pattern
	patternID := string(eventType)
	pattern, exists := engine.patterns[patternID]
	if !exists {
		pattern = &SimplePattern{
			ID:        patternID,
			Name:      fmt.Sprintf("Pattern: %s", eventType),
			EventType: eventType,
			Window:    engine.config.AnalysisWindow,
		}
		engine.patterns[patternID] = pattern
	}

	pattern.Count = count
	pattern.LastSeen = time.Now()

	// Check if pattern meets threshold
	if count >= engine.config.MinPatternCount {
		patterns = append(patterns, pattern.Name)
		engine.metrics.PatternsDetected++
	}

	return patterns
}

// detectSimpleAnomaly performs basic anomaly detection
func (engine *SimpleAnalyticsEngine) detectSimpleAnomaly(event events.Event) (bool, float64) {
	eventType := event.Type()
	recentEvents := engine.buffer.GetRecent(engine.config.AnalysisWindow)

	// Count events of same type
	sameTypeCount := 0
	totalCount := len(recentEvents)

	for _, recentEvent := range recentEvents {
		if recentEvent.Type() == eventType {
			sameTypeCount++
		}
	}

	if totalCount == 0 {
		return false, 0.0
	}

	// Calculate frequency ratio
	frequency := float64(sameTypeCount) / float64(totalCount)

	// Simple anomaly detection based on frequency
	// Events that are too rare or too frequent might be anomalies
	var anomalyScore float64
	if frequency < 0.05 { // Very rare (less than 5% of events)
		anomalyScore = 1.0 - frequency*20 // Scale to 0-1
	} else if frequency > 0.8 { // Very frequent (more than 80% of events)
		anomalyScore = frequency
	} else {
		anomalyScore = 0.0
	}

	isAnomaly := anomalyScore > 0.5
	if isAnomaly {
		engine.metrics.AnomaliesDetected++
	}

	return isAnomaly, anomalyScore
}

// updateSimpleMetrics updates internal metrics
func (engine *SimpleAnalyticsEngine) updateSimpleMetrics(result *SimpleAnalyticsResult) {
	engine.metrics.EventsProcessed++
	engine.metrics.LastUpdate = time.Now()
}

// GetMetrics returns current metrics
func (engine *SimpleAnalyticsEngine) GetMetrics() *SimpleMetrics {
	engine.mu.RLock()
	defer engine.mu.RUnlock()

	// Return a copy
	metrics := *engine.metrics
	return &metrics
}

// GetPatterns returns detected patterns
func (engine *SimpleAnalyticsEngine) GetPatterns() map[string]*SimplePattern {
	engine.mu.RLock()
	defer engine.mu.RUnlock()

	// Return a copy
	patterns := make(map[string]*SimplePattern)
	for id, pattern := range engine.patterns {
		patternCopy := *pattern
		patterns[id] = &patternCopy
	}

	return patterns
}

// GetRecentEvents returns recent events from buffer
func (engine *SimpleAnalyticsEngine) GetRecentEvents(duration time.Duration) []events.Event {
	return engine.buffer.GetRecent(duration)
}

// ClearBuffer clears the event buffer
func (engine *SimpleAnalyticsEngine) ClearBuffer() {
	engine.buffer.mu.Lock()
	defer engine.buffer.mu.Unlock()

	engine.buffer.events = engine.buffer.events[:0]
}

// Reset resets all analytics state
func (engine *SimpleAnalyticsEngine) Reset() {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	// Clear buffer without taking engine lock (already held)
	engine.buffer.mu.Lock()
	engine.buffer.events = engine.buffer.events[:0]
	engine.buffer.mu.Unlock()

	engine.patterns = make(map[string]*SimplePattern)
	engine.metrics = &SimpleMetrics{}
}
