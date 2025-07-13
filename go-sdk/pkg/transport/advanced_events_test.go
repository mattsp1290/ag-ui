package transport

import (
	"testing"
	"time"
)

func TestAdvancedEventTypes(t *testing.T) {
	t.Run("MessageEventData", func(t *testing.T) {
		data := &MessageEventData{
			Content:     "Test message",
			Role:        "user",
			Model:       "gpt-4",
			MessageID:   "msg-123",
			ThreadID:    "thread-456",
			Temperature: 0.7,
			MaxTokens:   150,
			TokenUsage: &TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			Metadata: map[string]string{
				"source": "test",
			},
			ProcessingTime: 100 * time.Millisecond,
		}
		
		// Test validation
		if err := data.Validate(); err != nil {
			t.Errorf("Valid MessageEventData failed validation: %v", err)
		}
		
		// Test invalid data
		invalidData := &MessageEventData{
			Content: "",
			Role:    "invalid",
		}
		if err := invalidData.Validate(); err == nil {
			t.Error("Invalid MessageEventData passed validation")
		}
		
		// Test ToMap conversion
		dataMap := data.ToMap()
		if dataMap["content"] != "Test message" {
			t.Error("ToMap conversion failed for content")
		}
		if dataMap["role"] != "user" {
			t.Error("ToMap conversion failed for role")
		}
	})
	
	t.Run("SecurityEventData", func(t *testing.T) {
		data := &SecurityEventData{
			EventType:   SecurityEventLogin,
			Severity:    SecuritySeverityInfo,
			Actor:       "user-123",
			Target:      "api",
			ThreatLevel: ThreatLevelNone,
			Permissions: []string{"read", "write"},
		}
		
		if err := data.Validate(); err != nil {
			t.Errorf("Valid SecurityEventData failed validation: %v", err)
		}
		
		// Test invalid data
		invalidData := &SecurityEventData{
			EventType:   "invalid-type",
			Severity:    "invalid-severity",
			ThreatLevel: "invalid-threat",
		}
		if err := invalidData.Validate(); err == nil {
			t.Error("Invalid SecurityEventData passed validation")
		}
		
		// Test ToMap
		dataMap := data.ToMap()
		if dataMap["event_type"] != string(SecurityEventLogin) {
			t.Error("ToMap conversion failed for event_type")
		}
	})
	
	t.Run("PerformanceEventData", func(t *testing.T) {
		data := &PerformanceEventData{
			MetricName:  "api_latency",
			Value:       125.5,
			Unit:        "ms",
			MetricType:  MetricTypeTimer,
			Component:   "api",
			Trend:       TrendStable,
			AlertLevel:  AlertLevelNone,
			Percentile:  95.0,
		}
		
		if err := data.Validate(); err != nil {
			t.Errorf("Valid PerformanceEventData failed validation: %v", err)
		}
		
		// Test invalid percentile
		invalidData := &PerformanceEventData{
			MetricName: "test",
			Value:      1.0,
			Unit:       "ms",
			MetricType: MetricTypeTimer,
			Percentile: 150.0, // Invalid: > 100
			Trend:      TrendStable,
			AlertLevel: AlertLevelNone,
		}
		if err := invalidData.Validate(); err == nil {
			t.Error("Invalid PerformanceEventData passed validation")
		}
	})
	
	t.Run("SystemEventData", func(t *testing.T) {
		data := &SystemEventData{
			EventType:    SystemEventStartup,
			Component:    "test-service",
			CurrentState: "running",
			Resources: ResourceUsage{
				CPU:    50.5,
				Memory: 1024,
			},
			Dependencies: []string{"db", "cache"},
		}
		
		if err := data.Validate(); err != nil {
			t.Errorf("Valid SystemEventData failed validation: %v", err)
		}
		
		// Test ToMap with resources
		dataMap := data.ToMap()
		if dataMap["component"] != "test-service" {
			t.Error("ToMap conversion failed for component")
		}
		if dataMap["resources"] == nil {
			t.Error("ToMap conversion failed to include resources")
		}
	})
	
	t.Run("ConfigurationEventData", func(t *testing.T) {
		data := &ConfigurationEventData{
			Key:       "api.timeout",
			NewValue:  "30s",
			ValueType: "duration",
			Impact:    ConfigImpactLow,
		}
		
		if err := data.Validate(); err != nil {
			t.Errorf("Valid ConfigurationEventData failed validation: %v", err)
		}
		
		// Test invalid impact
		invalidData := &ConfigurationEventData{
			Key:       "test",
			NewValue:  "value",
			ValueType: "string",
			Impact:    "invalid-impact",
		}
		if err := invalidData.Validate(); err == nil {
			t.Error("Invalid ConfigurationEventData passed validation")
		}
	})
	
	t.Run("StateChangeEventData", func(t *testing.T) {
		data := &StateChangeEventData{
			FromState:  "idle",
			ToState:    "processing",
			EntityID:   "entity-123",
			EntityType: "processor",
			Duration:   5 * time.Second,
			Automatic:  true,
		}
		
		if err := data.Validate(); err != nil {
			t.Errorf("Valid StateChangeEventData failed validation: %v", err)
		}
		
		// Test ToMap with duration
		dataMap := data.ToMap()
		if dataMap["duration"] != "5s" {
			t.Error("ToMap conversion failed for duration")
		}
	})
}

func TestAdvancedEventCreation(t *testing.T) {
	t.Run("CreateMessageEvent", func(t *testing.T) {
		data := &MessageEventData{
			Content: "Hello",
			Role:    "user",
		}
		event := CreateMessageEvent("msg-1", data)
		
		if event.ID() != "msg-1" {
			t.Errorf("Expected ID msg-1, got %s", event.ID())
		}
		if event.Type() != "message" {
			t.Errorf("Expected type message, got %s", event.Type())
		}
		if event.TypedData().Content != "Hello" {
			t.Error("Event data not preserved correctly")
		}
	})
	
	t.Run("CreateSecurityEvent", func(t *testing.T) {
		data := &SecurityEventData{
			EventType:   SecurityEventLogin,
			Severity:    SecuritySeverityInfo,
			ThreatLevel: ThreatLevelNone,
		}
		event := CreateSecurityEvent("sec-1", data)
		
		if event.Type() != "security" {
			t.Errorf("Expected type security, got %s", event.Type())
		}
	})
	
	t.Run("CreatePerformanceEvent", func(t *testing.T) {
		data := &PerformanceEventData{
			MetricName: "test_metric",
			Value:      100.0,
			Unit:       "ms",
			MetricType: MetricTypeGauge,
			Trend:      TrendStable,
			AlertLevel: AlertLevelNone,
		}
		event := CreatePerformanceEvent("perf-1", data)
		
		if event.Type() != "performance" {
			t.Errorf("Expected type performance, got %s", event.Type())
		}
	})
}