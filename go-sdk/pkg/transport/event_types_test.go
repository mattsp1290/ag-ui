package transport

import (
	"testing"
	"time"
)

func TestTypedEventDataValidation(t *testing.T) {
	tests := []struct {
		name      string
		eventData EventData
		expectErr bool
	}{
		{
			name: "valid connection event",
			eventData: ConnectionEventData{
				Status:        "connected",
				RemoteAddress: "example.com:8080",
				Protocol:      "websocket",
			},
			expectErr: false,
		},
		{
			name: "invalid connection event - no status",
			eventData: ConnectionEventData{
				RemoteAddress: "example.com:8080",
				Protocol:      "websocket",
			},
			expectErr: true,
		},
		{
			name: "valid data event",
			eventData: DataEventData{
				Content:     []byte("hello world"),
				Size:        11,
				ContentType: "text/plain",
			},
			expectErr: false,
		},
		{
			name: "invalid data event - nil content",
			eventData: DataEventData{
				Content: nil,
				Size:    0,
			},
			expectErr: true,
		},
		{
			name: "invalid data event - size mismatch",
			eventData: DataEventData{
				Content: []byte("hello"),
				Size:    10, // Wrong size
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.eventData.Validate()
			if tt.expectErr && err == nil {
				t.Errorf("expected validation error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestTypedTransportEventInterface(t *testing.T) {
	// Create a typed event
	data := ConnectionEventData{
		Status:        "connected",
		RemoteAddress: "example.com:8080",
		Protocol:      "websocket",
	}
	
	typedEvent := NewTypedEvent("test-id", "connection", data)
	
	// Test the interface methods
	if typedEvent.ID() != "test-id" {
		t.Errorf("expected ID 'test-id', got %s", typedEvent.ID())
	}
	
	if typedEvent.Type() != "connection" {
		t.Errorf("expected type 'connection', got %s", typedEvent.Type())
	}
	
	if typedEvent.Timestamp().IsZero() {
		t.Error("expected non-zero timestamp")
	}
	
	// Test typed data access
	retrievedData := typedEvent.TypedData()
	if retrievedData.Status != "connected" {
		t.Errorf("expected status 'connected', got %s", retrievedData.Status)
	}
	
	// Test backward compatibility - Data() method
	dataMap := typedEvent.Data()
	if dataMap["status"] != "connected" {
		t.Errorf("expected status 'connected' in data map, got %v", dataMap["status"])
	}
}

func TestEventConversion(t *testing.T) {
	// Create a typed event
	data := DataEventData{
		Content:     []byte("test content"),
		Size:        12,
		ContentType: "text/plain",
	}
	
	typedEvent := NewTypedEvent("test-id", "data", data)
	
	// Convert to legacy event
	legacyEvent := AdaptToLegacyEvent(typedEvent)
	
	// Verify the legacy event works
	if legacyEvent.ID() != "test-id" {
		t.Errorf("expected ID 'test-id', got %s", legacyEvent.ID())
	}
	
	if legacyEvent.Type() != "data" {
		t.Errorf("expected type 'data', got %s", legacyEvent.Type())
	}
	
	dataMap := legacyEvent.Data()
	if string(dataMap["content"].([]byte)) != "test content" {
		t.Errorf("expected content 'test content', got %v", dataMap["content"])
	}
	
	// Try to convert back to typed event
	retrievedTypedEvent := TryGetTypedEvent[DataEventData](legacyEvent)
	if retrievedTypedEvent == nil {
		t.Error("expected to retrieve typed event from legacy event")
	} else {
		retrievedData := retrievedTypedEvent.TypedData()
		if string(retrievedData.Content) != "test content" {
			t.Errorf("expected content 'test content', got %s", string(retrievedData.Content))
		}
	}
}

func TestEventCreators(t *testing.T) {
	// Test connection event creator
	connEvent := CreateConnectionEvent("conn-1", "connected",
		WithRemoteAddress("example.com:8080"),
		WithProtocol("websocket"),
	)
	
	data := connEvent.TypedData()
	if data.Status != "connected" {
		t.Errorf("expected status 'connected', got %s", data.Status)
	}
	if data.RemoteAddress != "example.com:8080" {
		t.Errorf("expected remote address 'example.com:8080', got %s", data.RemoteAddress)
	}
	if data.Protocol != "websocket" {
		t.Errorf("expected protocol 'websocket', got %s", data.Protocol)
	}
	
	// Test data event creator
	content := []byte("test data")
	dataEvent := CreateDataEvent("data-1", content,
		WithContentType("application/json"),
		WithStreamID("stream-123"),
	)
	
	dataEventData := dataEvent.TypedData()
	if string(dataEventData.Content) != "test data" {
		t.Errorf("expected content 'test data', got %s", string(dataEventData.Content))
	}
	if dataEventData.ContentType != "application/json" {
		t.Errorf("expected content type 'application/json', got %s", dataEventData.ContentType)
	}
	if dataEventData.StreamID != "stream-123" {
		t.Errorf("expected stream ID 'stream-123', got %s", dataEventData.StreamID)
	}
	
	// Test error event creator
	errorEvent := CreateErrorEvent("error-1", "test error",
		WithErrorCode("E001"),
		WithErrorSeverity("error"),
		WithRetryable(true),
	)
	
	errorData := errorEvent.TypedData()
	if errorData.Message != "test error" {
		t.Errorf("expected message 'test error', got %s", errorData.Message)
	}
	if errorData.Code != "E001" {
		t.Errorf("expected code 'E001', got %s", errorData.Code)
	}
	if errorData.Severity != "error" {
		t.Errorf("expected severity 'error', got %s", errorData.Severity)
	}
	if !errorData.Retryable {
		t.Error("expected retryable to be true")
	}
	
	// Test metrics event creator
	metricsEvent := CreateMetricsEvent("metrics-1", "cpu_usage", 75.5,
		WithUnit("percent"),
		WithTags(map[string]string{"host": "server1"}),
		WithInterval(time.Minute),
	)
	
	metricsData := metricsEvent.TypedData()
	if metricsData.MetricName != "cpu_usage" {
		t.Errorf("expected metric name 'cpu_usage', got %s", metricsData.MetricName)
	}
	if metricsData.Value != 75.5 {
		t.Errorf("expected value 75.5, got %f", metricsData.Value)
	}
	if metricsData.Unit != "percent" {
		t.Errorf("expected unit 'percent', got %s", metricsData.Unit)
	}
	if metricsData.Interval != time.Minute {
		t.Errorf("expected interval 1m, got %v", metricsData.Interval)
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Create a legacy event using the old interface
	legacyEvent := NewLegacyEvent("legacy-1", "test", map[string]interface{}{
		"message": "hello world",
		"count":   42,
	})
	
	// Verify it works with the TransportEvent interface
	var event TransportEvent = legacyEvent
	if event.ID() != "legacy-1" {
		t.Errorf("expected ID 'legacy-1', got %s", event.ID())
	}
	
	data := event.Data()
	if data["message"] != "hello world" {
		t.Errorf("expected message 'hello world', got %v", data["message"])
	}
	if data["count"] != 42 {
		t.Errorf("expected count 42, got %v", data["count"])
	}
	
	// Create a typed event and use it as a legacy event
	typedData := ConnectionEventData{
		Status:   "connected",
		Protocol: "http",
	}
	typedEvent := NewTypedEvent("typed-1", "connection", typedData)
	
	// Convert to legacy and use as TransportEvent
	var legacyFromTyped TransportEvent = AdaptToLegacyEvent(typedEvent)
	if legacyFromTyped.ID() != "typed-1" {
		t.Errorf("expected ID 'typed-1', got %s", legacyFromTyped.ID())
	}
	
	adaptedData := legacyFromTyped.Data()
	if adaptedData["status"] != "connected" {
		t.Errorf("expected status 'connected', got %v", adaptedData["status"])
	}
}