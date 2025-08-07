package transport

import (
	"testing"
)

func TestCreateDataEventBasic(t *testing.T) {
	// Test basic creation
	content := []byte("test data")
	event := CreateDataEvent("test-1", content)

	if event.ID() != "test-1" {
		t.Errorf("expected ID 'test-1', got %s", event.ID())
	}

	if event.Type() != "data" {
		t.Errorf("expected type 'data', got %s", event.Type())
	}

	data := event.TypedData()
	if string(data.Content) != "test data" {
		t.Errorf("expected content 'test data', got %s", string(data.Content))
	}

	if data.Size != 9 {
		t.Errorf("expected size 9, got %d", data.Size)
	}
}

func TestCreateDataEventWithOptions(t *testing.T) {
	// Test with options
	content := []byte("test data")
	event := CreateDataEvent("test-2", content,
		WithContentType("application/json"),
		WithStreamID("stream-123"),
		WithSequenceNumber(42),
	)

	data := event.TypedData()
	if data.ContentType != "application/json" {
		t.Errorf("expected content type 'application/json', got %s", data.ContentType)
	}

	if data.StreamID != "stream-123" {
		t.Errorf("expected stream ID 'stream-123', got %s", data.StreamID)
	}

	if data.SequenceNumber != 42 {
		t.Errorf("expected sequence number 42, got %d", data.SequenceNumber)
	}
}

func TestCreateDataEventWithBuilder(t *testing.T) {
	// Test with builder function
	content := []byte("test data")
	event := CreateDataEvent("test-3", content,
		func(data *DataEventData) {
			data.ContentType = "text/plain"
			data.Encoding = "utf-8"
			data.Compressed = true
		},
	)

	data := event.TypedData()
	if data.ContentType != "text/plain" {
		t.Errorf("expected content type 'text/plain', got %s", data.ContentType)
	}

	if data.Encoding != "utf-8" {
		t.Errorf("expected encoding 'utf-8', got %s", data.Encoding)
	}

	if !data.Compressed {
		t.Error("expected compressed to be true")
	}
}

func TestCreateConnectionEventBasic(t *testing.T) {
	// Test basic creation
	event := CreateConnectionEvent("conn-1", "connected")

	if event.ID() != "conn-1" {
		t.Errorf("expected ID 'conn-1', got %s", event.ID())
	}

	if event.Type() != "connection" {
		t.Errorf("expected type 'connection', got %s", event.Type())
	}

	data := event.TypedData()
	if data.Status != "connected" {
		t.Errorf("expected status 'connected', got %s", data.Status)
	}
}

func TestCreateConnectionEventWithOptions(t *testing.T) {
	// Test with options
	event := CreateConnectionEvent("conn-2", "connected",
		WithRemoteAddress("example.com:8080"),
		WithProtocol("websocket"),
		WithVersion("1.0"),
	)

	data := event.TypedData()
	if data.RemoteAddress != "example.com:8080" {
		t.Errorf("expected remote address 'example.com:8080', got %s", data.RemoteAddress)
	}

	if data.Protocol != "websocket" {
		t.Errorf("expected protocol 'websocket', got %s", data.Protocol)
	}

	if data.Version != "1.0" {
		t.Errorf("expected version '1.0', got %s", data.Version)
	}
}

func TestCreateConnectionEventWithBuilder(t *testing.T) {
	// Test with builder function
	event := CreateConnectionEvent("conn-3", "disconnected",
		func(data *ConnectionEventData) {
			data.RemoteAddress = "localhost:3000"
			data.Error = "connection timeout"
			data.AttemptNumber = 3
		},
	)

	data := event.TypedData()
	if data.RemoteAddress != "localhost:3000" {
		t.Errorf("expected remote address 'localhost:3000', got %s", data.RemoteAddress)
	}

	if data.Error != "connection timeout" {
		t.Errorf("expected error 'connection timeout', got %s", data.Error)
	}

	if data.AttemptNumber != 3 {
		t.Errorf("expected attempt number 3, got %d", data.AttemptNumber)
	}
}

func TestCreateErrorEvent(t *testing.T) {
	// Test error event creation
	event := CreateErrorEvent("err-1", "test error",
		WithErrorCode("E001"),
		WithErrorSeverity("error"),
		WithRetryable(true),
	)

	if event.ID() != "err-1" {
		t.Errorf("expected ID 'err-1', got %s", event.ID())
	}

	if event.Type() != "error" {
		t.Errorf("expected type 'error', got %s", event.Type())
	}

	data := event.TypedData()
	if data.Message != "test error" {
		t.Errorf("expected message 'test error', got %s", data.Message)
	}

	if data.Code != "E001" {
		t.Errorf("expected code 'E001', got %s", data.Code)
	}

	if data.Severity != "error" {
		t.Errorf("expected severity 'error', got %s", data.Severity)
	}

	if !data.Retryable {
		t.Error("expected retryable to be true")
	}
}

func TestAdaptToLegacyEvent(t *testing.T) {
	// Test adapter function
	typedEvent := CreateDataEvent("adapt-1", []byte("test"),
		WithContentType("text/plain"),
	)

	legacyEvent := AdaptToLegacyEvent(typedEvent)

	if legacyEvent.ID() != "adapt-1" {
		t.Errorf("expected ID 'adapt-1', got %s", legacyEvent.ID())
	}

	if legacyEvent.Type() != "data" {
		t.Errorf("expected type 'data', got %s", legacyEvent.Type())
	}

	dataMap := legacyEvent.Data()
	if string(dataMap["content"].([]byte)) != "test" {
		t.Errorf("expected content 'test', got %v", dataMap["content"])
	}

	if dataMap["content_type"] != "text/plain" {
		t.Errorf("expected content_type 'text/plain', got %v", dataMap["content_type"])
	}
}

func TestTryGetTypedEvent(t *testing.T) {
	// Test retrieving typed event from adapter
	originalEvent := CreateDataEvent("retrieve-1", []byte("test"))
	adapter := AdaptToLegacyEvent(originalEvent)

	// Try to get it back
	retrieved := TryGetTypedEvent[DataEventData](adapter)
	if retrieved == nil {
		t.Fatal("expected to retrieve typed event")
	}

	if retrieved.ID() != "retrieve-1" {
		t.Errorf("expected ID 'retrieve-1', got %s", retrieved.ID())
	}

	data := retrieved.TypedData()
	if string(data.Content) != "test" {
		t.Errorf("expected content 'test', got %s", string(data.Content))
	}
}

func TestMixedOptionsAndBuilder(t *testing.T) {
	// Test mixing options and builder
	event := CreateDataEvent("mixed-1", []byte("test"),
		WithContentType("application/json"),
		func(data *DataEventData) {
			data.StreamID = "stream-456"
			data.Compressed = true
		},
		WithSequenceNumber(100),
	)

	data := event.TypedData()
	if data.ContentType != "application/json" {
		t.Errorf("expected content type 'application/json', got %s", data.ContentType)
	}

	if data.StreamID != "stream-456" {
		t.Errorf("expected stream ID 'stream-456', got %s", data.StreamID)
	}

	if !data.Compressed {
		t.Error("expected compressed to be true")
	}

	if data.SequenceNumber != 100 {
		t.Errorf("expected sequence number 100, got %d", data.SequenceNumber)
	}
}
