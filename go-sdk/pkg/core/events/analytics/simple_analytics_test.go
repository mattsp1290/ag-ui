package analytics

import (
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
)

// MockEvent implements the events.Event interface for testing
type MockEvent struct {
	eventType   events.EventType
	timestamp   *int64
	baseEvent   *events.BaseEvent
}

func (m *MockEvent) Type() events.EventType {
	return m.eventType
}

func (m *MockEvent) Timestamp() *int64 {
	return m.timestamp
}

func (m *MockEvent) SetTimestamp(timestamp int64) {
	m.timestamp = &timestamp
}

func (m *MockEvent) Validate() error {
	return nil
}

func (m *MockEvent) ToJSON() ([]byte, error) {
	return []byte(`{}`), nil
}

func (m *MockEvent) ToProtobuf() (*generated.Event, error) {
	return &generated.Event{}, nil
}

func (m *MockEvent) GetBaseEvent() *events.BaseEvent {
	return m.baseEvent
}

// Helper function to create a mock event
func createMockEvent(eventType events.EventType) events.Event {
	now := time.Now().UnixMilli()
	return &MockEvent{
		eventType: eventType,
		timestamp: &now,
		baseEvent: &events.BaseEvent{
			EventType:   eventType,
			TimestampMs: &now,
		},
	}
}

func TestSimpleAnalyticsEngine_Creation(t *testing.T) {
	engine := NewSimpleAnalyticsEngine(nil)
	
	if engine == nil {
		t.Fatal("Expected analytics engine to be created")
	}
	
	if engine.config == nil {
		t.Error("Expected config to be initialized")
	}
	
	if engine.buffer == nil {
		t.Error("Expected buffer to be initialized")
	}
	
	if engine.metrics == nil {
		t.Error("Expected metrics to be initialized")
	}
	
	if engine.patterns == nil {
		t.Error("Expected patterns map to be initialized")
	}
}

func TestSimpleAnalyticsEngine_AnalyzeEvent(t *testing.T) {
	engine := NewSimpleAnalyticsEngine(nil)
	
	// Create a test event
	event := createMockEvent(events.EventTypeTextMessageStart)
	
	// Analyze the event
	result, err := engine.AnalyzeEvent(event)
	if err != nil {
		t.Fatalf("Expected analyze event to succeed, got error: %v", err)
	}
	
	if result == nil {
		t.Fatal("Expected result to be returned")
	}
	
	if result.EventType != events.EventTypeTextMessageStart {
		t.Errorf("Expected event type %s, got %s", events.EventTypeTextMessageStart, result.EventType)
	}
	
	if result.ProcessingTime <= 0 {
		t.Error("Expected processing time to be positive")
	}
	
	// Check metrics were updated
	metrics := engine.GetMetrics()
	if metrics.EventsProcessed != 1 {
		t.Errorf("Expected 1 event processed, got %d", metrics.EventsProcessed)
	}
}

func TestSimpleAnalyticsEngine_PatternDetection(t *testing.T) {
	config := &SimpleAnalyticsConfig{
		BufferSize:      100,
		AnalysisWindow:  1 * time.Minute,
		MinPatternCount: 3,
	}
	engine := NewSimpleAnalyticsEngine(config)
	
	// Add multiple events of the same type
	eventType := events.EventTypeTextMessageStart
	for i := 0; i < 5; i++ {
		event := createMockEvent(eventType)
		result, err := engine.AnalyzeEvent(event)
		if err != nil {
			t.Fatalf("Failed to analyze event %d: %v", i, err)
		}
		
		// After the 3rd event, we should detect a pattern
		if i >= 2 && len(result.PatternsFound) == 0 {
			t.Errorf("Expected pattern to be detected after event %d", i+1)
		}
	}
	
	// Check patterns were recorded
	patterns := engine.GetPatterns()
	if len(patterns) == 0 {
		t.Error("Expected patterns to be recorded")
	}
	
	pattern, exists := patterns[string(eventType)]
	if !exists {
		t.Errorf("Expected pattern for event type %s", eventType)
	}
	
	if pattern.Count != 5 {
		t.Errorf("Expected pattern count 5, got %d", pattern.Count)
	}
}

func TestSimpleAnalyticsEngine_AnomalyDetection(t *testing.T) {
	engine := NewSimpleAnalyticsEngine(nil)
	
	// Add many events of one type to establish baseline
	commonType := events.EventTypeTextMessageContent
	for i := 0; i < 50; i++ {
		event := createMockEvent(commonType)
		_, err := engine.AnalyzeEvent(event)
		if err != nil {
			t.Fatalf("Failed to analyze common event %d: %v", i, err)
		}
	}
	
	// Add a rare event type - should be anomalous due to low frequency
	rareEvent := createMockEvent(events.EventTypeRunError)
	result, err := engine.AnalyzeEvent(rareEvent)
	if err != nil {
		t.Fatalf("Failed to analyze rare event: %v", err)
	}
	
	// The rare event should be detected as an anomaly
	// With 1 rare event out of 51 total, frequency = 1/51 ≈ 0.0196 < 0.05
	if !result.IsAnomaly {
		t.Errorf("Expected rare event to be detected as anomaly, frequency should be ~%.3f", 1.0/51.0)
	}
	
	if result.AnomalyScore <= 0 {
		t.Errorf("Expected positive anomaly score for rare event, got %.3f", result.AnomalyScore)
	}
}

func TestSimpleEventBuffer_Operations(t *testing.T) {
	buffer := NewSimpleEventBuffer(5)
	
	// Test empty buffer
	recent := buffer.GetRecent(1 * time.Hour)
	if len(recent) != 0 {
		t.Errorf("Expected 0 recent events, got %d", len(recent))
	}
	
	// Add events
	testEvents := make([]events.Event, 3)
	for i := 0; i < 3; i++ {
		testEvents[i] = createMockEvent(events.EventTypeTextMessageStart)
		buffer.Add(testEvents[i])
	}
	
	// Check recent events
	recent = buffer.GetRecent(1 * time.Hour)
	if len(recent) != 3 {
		t.Errorf("Expected 3 recent events, got %d", len(recent))
	}
	
	// Test buffer overflow
	for i := 3; i < 8; i++ {
		event := createMockEvent(events.EventTypeTextMessageEnd)
		buffer.Add(event)
	}
	
	recent = buffer.GetRecent(1 * time.Hour)
	if len(recent) > 5 {
		t.Errorf("Expected at most 5 events due to buffer limit, got %d", len(recent))
	}
}

func TestSimpleAnalyticsEngine_GetRecentEvents(t *testing.T) {
	engine := NewSimpleAnalyticsEngine(nil)
	
	// Add some events
	for i := 0; i < 5; i++ {
		event := createMockEvent(events.EventTypeTextMessageStart)
		_, err := engine.AnalyzeEvent(event)
		if err != nil {
			t.Fatalf("Failed to analyze event %d: %v", i, err)
		}
	}
	
	// Get recent events
	recent := engine.GetRecentEvents(1 * time.Hour)
	if len(recent) != 5 {
		t.Errorf("Expected 5 recent events, got %d", len(recent))
	}
}

func TestSimpleAnalyticsEngine_Reset(t *testing.T) {
	engine := NewSimpleAnalyticsEngine(nil)
	
	// Add some events and patterns
	for i := 0; i < 5; i++ {
		event := createMockEvent(events.EventTypeTextMessageStart)
		_, err := engine.AnalyzeEvent(event)
		if err != nil {
			t.Fatalf("Failed to analyze event %d: %v", i, err)
		}
	}
	
	// Check that data exists
	metrics := engine.GetMetrics()
	if metrics.EventsProcessed == 0 {
		t.Error("Expected events to be processed before reset")
	}
	
	patterns := engine.GetPatterns()
	if len(patterns) == 0 {
		t.Error("Expected patterns to exist before reset")
	}
	
	// Reset the engine
	engine.Reset()
	
	// Check that data is cleared
	metrics = engine.GetMetrics()
	if metrics.EventsProcessed != 0 {
		t.Error("Expected events processed to be reset to 0")
	}
	
	patterns = engine.GetPatterns()
	if len(patterns) != 0 {
		t.Error("Expected patterns to be cleared after reset")
	}
	
	recent := engine.GetRecentEvents(1 * time.Hour)
	if len(recent) != 0 {
		t.Error("Expected event buffer to be cleared after reset")
	}
}

func TestSimpleAnalyticsEngine_ConfigValidation(t *testing.T) {
	config := &SimpleAnalyticsConfig{
		BufferSize:      50,
		AnalysisWindow:  2 * time.Minute,
		MinPatternCount: 5,
	}
	
	engine := NewSimpleAnalyticsEngine(config)
	
	if engine.config.BufferSize != 50 {
		t.Errorf("Expected buffer size 50, got %d", engine.config.BufferSize)
	}
	
	if engine.config.AnalysisWindow != 2*time.Minute {
		t.Errorf("Expected analysis window 2m, got %v", engine.config.AnalysisWindow)
	}
	
	if engine.config.MinPatternCount != 5 {
		t.Errorf("Expected min pattern count 5, got %d", engine.config.MinPatternCount)
	}
}

// Benchmark test
func BenchmarkSimpleAnalyticsEngine_AnalyzeEvent(b *testing.B) {
	engine := NewSimpleAnalyticsEngine(nil)
	event := createMockEvent(events.EventTypeTextMessageStart)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.AnalyzeEvent(event)
		if err != nil {
			b.Fatalf("Failed to analyze event: %v", err)
		}
	}
}

func BenchmarkSimpleEventBuffer_Add(b *testing.B) {
	buffer := NewSimpleEventBuffer(1000)
	event := createMockEvent(events.EventTypeTextMessageStart)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer.Add(event)
	}
}