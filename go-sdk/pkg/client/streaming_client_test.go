package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// MockSSEServer creates a test SSE server
func MockSSEServer(events []string, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		for i, event := range events {
			fmt.Fprintf(w, "%s\n\n", event)
			flusher.Flush()
			
			if delay > 0 && i < len(events)-1 {
				time.Sleep(delay)
			}
		}
	}))
}

func TestSSEClient_BasicConnection(t *testing.T) {
	events := []string{
		"data: hello",
		"data: world",
	}
	
	server := MockSSEServer(events, 10*time.Millisecond)
	defer server.Close()

	config := SSEClientConfig{
		URL:              server.URL,
		EventBufferSize:  10,
		InitialBackoff:   100 * time.Millisecond,
		MaxBackoff:       time.Second,
	}

	client, err := NewSSEClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSE client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Verify connection state
	if client.State() != SSEStateConnected {
		t.Errorf("Expected state %v, got %v", SSEStateConnected, client.State())
	}

	// Collect events
	var receivedEvents []*SSEEvent
	eventChan := client.Events()

	done := make(chan bool)
	go func() {
		for event := range eventChan {
			receivedEvents = append(receivedEvents, event)
			if len(receivedEvents) >= len(events) {
				done <- true
				return
			}
		}
	}()

	select {
	case <-done:
		// Success
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for events")
	}

	if len(receivedEvents) != len(events) {
		t.Errorf("Expected %d events, got %d", len(events), len(receivedEvents))
	}

	// Verify event data
	expectedData := []string{"hello", "world"}
	for i, event := range receivedEvents {
		if event.Data != expectedData[i] {
			t.Errorf("Event %d: expected data %q, got %q", i, expectedData[i], event.Data)
		}
	}
}

func TestSSEClient_EventParsing(t *testing.T) {
	events := []string{
		"id: 1\nevent: test\ndata: {\"message\": \"hello\"}\nretry: 5000",
		"id: 2\nevent: custom\ndata: line1\ndata: line2",
		"data: simple event",
	}

	server := MockSSEServer(events, 5*time.Millisecond)
	defer server.Close()

	config := SSEClientConfig{
		URL:             server.URL,
		EventBufferSize: 10,
	}

	client, err := NewSSEClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSE client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	var receivedEvents []*SSEEvent
	eventChan := client.Events()

	// Collect events with timeout
	timeout := time.After(2 * time.Second)
	for len(receivedEvents) < 3 {
		select {
		case event := <-eventChan:
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			break
		}
	}

	if len(receivedEvents) < 3 {
		t.Fatalf("Expected at least 3 events, got %d", len(receivedEvents))
	}

	// Verify first event
	event1 := receivedEvents[0]
	if event1.ID != "1" {
		t.Errorf("Event 1: expected ID '1', got '%s'", event1.ID)
	}
	if event1.Event != "test" {
		t.Errorf("Event 1: expected event 'test', got '%s'", event1.Event)
	}
	if event1.Data != `{"message": "hello"}` {
		t.Errorf("Event 1: expected data %q, got %q", `{"message": "hello"}`, event1.Data)
	}
	if event1.Retry == nil || *event1.Retry != 5*time.Second {
		t.Errorf("Event 1: expected retry 5s, got %v", event1.Retry)
	}

	// Verify second event (multiline data)
	event2 := receivedEvents[1]
	if event2.ID != "2" {
		t.Errorf("Event 2: expected ID '2', got '%s'", event2.ID)
	}
	if event2.Event != "custom" {
		t.Errorf("Event 2: expected event 'custom', got '%s'", event2.Event)
	}
	if event2.Data != "line1\nline2" {
		t.Errorf("Event 2: expected multiline data, got %q", event2.Data)
	}

	// Verify third event (simple)
	event3 := receivedEvents[2]
	if event3.Data != "simple event" {
		t.Errorf("Event 3: expected data 'simple event', got %q", event3.Data)
	}
}

func TestSSEClient_Reconnection(t *testing.T) {
	connectionCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectionCount++
		count := connectionCount
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// First connection: send one event then close
		if count == 1 {
			fmt.Fprintf(w, "data: connection1\n\n")
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
			return // Close connection
		}

		// Second connection: send another event
		if count == 2 {
			fmt.Fprintf(w, "data: connection2\n\n")
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	var reconnectCalled int32 // Changed to int32 for atomic operations
	config := SSEClientConfig{
		URL:                  server.URL,
		InitialBackoff:       50 * time.Millisecond,
		MaxBackoff:           200 * time.Millisecond,
		MaxReconnectAttempts: 2,
		OnReconnect: func(attempt int) {
			atomic.StoreInt32(&reconnectCalled, 1) // Atomic write
		},
	}

	client, err := NewSSEClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSE client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	var receivedEvents []*SSEEvent
	eventChan := client.Events()

	// Collect events from both connections
	timeout := time.After(5 * time.Second)
	for len(receivedEvents) < 2 {
		select {
		case event := <-eventChan:
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			break
		}
	}

	if atomic.LoadInt32(&reconnectCalled) == 0 {
		t.Error("Expected reconnect callback to be called")
	}

	if len(receivedEvents) < 2 {
		t.Errorf("Expected at least 2 events from reconnection, got %d", len(receivedEvents))
	}

	mu.Lock()
	finalConnectionCount := connectionCount
	mu.Unlock()

	if finalConnectionCount < 2 {
		t.Errorf("Expected at least 2 connections, got %d", finalConnectionCount)
	}
}

func TestSSEClient_FlowControl(t *testing.T) {
	// Create many events to trigger flow control
	var events []string
	for i := 0; i < 50; i++ {
		events = append(events, fmt.Sprintf("data: event_%d", i))
	}

	server := MockSSEServer(events, 0) // No delay between events
	defer server.Close()

	config := SSEClientConfig{
		URL:                   server.URL,
		EventBufferSize:       0,  // No channel buffering to force immediate backpressure
		FlowControlEnabled:    true,
		FlowControlThreshold:  0.5, // 50%
	}

	client, err := NewSSEClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSE client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	eventChan := client.Events()

	// Slow consumer to trigger backpressure
	var receivedCount int32 // Changed to int32 for atomic operations
	slowConsumer := func() {
		for range eventChan {
			atomic.AddInt32(&receivedCount, 1) // Atomic increment
			time.Sleep(50 * time.Millisecond) // Very slow processing to force backpressure
		}
	}

	go slowConsumer()

	// Wait for backpressure to activate
	time.Sleep(500 * time.Millisecond)

	// Check that flow control is working by verifying slower consumption than production
	// With 50 events sent immediately and slow consumer (50ms per event), 
	// the consumer should be behind if flow control is working
	expectedMaxReceived := int32(10) // In 500ms, slow consumer (50ms each) can process ~10 events
	currentCount := atomic.LoadInt32(&receivedCount) // Atomic read
	
	if currentCount > expectedMaxReceived {
		t.Errorf("Expected flow control to limit events, but received %d events (expected <= %d)", currentCount, expectedMaxReceived)
	}
	
	// Check that some events are being processed (not completely blocked)
	if currentCount == 0 {
		t.Error("Expected some events to be received with flow control")
	}

	// Wait for more events to be processed
	time.Sleep(2 * time.Second)

	finalCount := atomic.LoadInt32(&receivedCount) // Atomic read
	if finalCount == 0 {
		t.Error("Expected some events to be received despite backpressure")
	}
}

func TestSSEClient_EventFiltering(t *testing.T) {
	events := []string{
		"event: allowed\ndata: should receive",
		"event: blocked\ndata: should not receive", 
		"event: allowed\ndata: should receive 2",
	}

	server := MockSSEServer(events, 10*time.Millisecond)
	defer server.Close()

	config := SSEClientConfig{
		URL: server.URL,
		EventFilter: func(eventType string) bool {
			return eventType == "allowed"
		},
	}

	client, err := NewSSEClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSE client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	var receivedEvents []*SSEEvent
	eventChan := client.Events()

	timeout := time.After(2 * time.Second)
	for {
		select {
		case event := <-eventChan:
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			goto done
		}
	}

done:
	// Should only receive the "allowed" events
	if len(receivedEvents) != 2 {
		t.Errorf("Expected 2 filtered events, got %d", len(receivedEvents))
	}

	for _, event := range receivedEvents {
		if event.Event != "allowed" {
			t.Errorf("Expected only 'allowed' events, got event type: %s", event.Event)
		}
	}
}

func TestSSEClient_LastEventID(t *testing.T) {
	events := []string{
		"id: event1\ndata: first",
		"id: event2\ndata: second",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		lastEventID := r.Header.Get("Last-Event-ID")
		
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// If resuming from event1, only send event2
		if lastEventID == "event1" {
			fmt.Fprintf(w, "id: event2\ndata: second\n\n")
		} else {
			// Send all events
			for _, event := range events {
				fmt.Fprintf(w, "%s\n\n", event)
				flusher.Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}
	}))
	defer server.Close()

	// First connection
	config := SSEClientConfig{
		URL: server.URL,
	}

	client, err := NewSSEClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSE client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Receive first event
	eventChan := client.Events()
	event := <-eventChan
	if event.ID != "event1" {
		t.Errorf("Expected first event ID 'event1', got '%s'", event.ID)
	}

	lastEventID := client.LastEventID()
	if lastEventID != "event1" {
		t.Errorf("Expected last event ID 'event1', got '%s'", lastEventID)
	}

	client.Close()

	// Second connection with last event ID
	config.LastEventID = lastEventID
	client2, err := NewSSEClient(config)
	if err != nil {
		t.Fatalf("Failed to create second SSE client: %v", err)
	}
	defer client2.Close()

	err = client2.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect second client: %v", err)
	}

	// Should only receive event2
	eventChan2 := client2.Events()
	select {
	case event2 := <-eventChan2:
		if event2.ID != "event2" {
			t.Errorf("Expected resumed event ID 'event2', got '%s'", event2.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for resumed event")
	}
}

func TestSSEClient_TLSConfig(t *testing.T) {
	// Create HTTPS test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: secure connection\n\n")
	}))
	defer server.Close()

	config := SSEClientConfig{
		URL:           server.URL,
		SkipTLSVerify: true, // For test server with self-signed cert
	}

	client, err := NewSSEClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSE client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect to HTTPS server: %v", err)
	}

	// Verify secure connection works
	eventChan := client.Events()
	select {
	case event := <-eventChan:
		if event.Data != "secure connection" {
			t.Errorf("Expected secure connection data, got: %s", event.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for secure connection event")
	}
}

func TestSSEClient_ConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      SSEClientConfig
		expectError bool
	}{
		{
			name:        "empty URL",
			config:      SSEClientConfig{},
			expectError: true,
		},
		{
			name: "invalid URL format",
			config: SSEClientConfig{
				URL: "not-a-url",
			},
			expectError: true,
		},
		{
			name: "invalid URL scheme",
			config: SSEClientConfig{
				URL: "ftp://example.com",
			},
			expectError: true,
		},
		{
			name: "valid HTTP URL",
			config: SSEClientConfig{
				URL: "http://example.com/events",
			},
			expectError: false,
		},
		{
			name: "valid HTTPS URL",
			config: SSEClientConfig{
				URL: "https://example.com/events",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSSEClient(tt.config)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestSSEClient_EventConversion(t *testing.T) {
	// Test SSE to AG-UI event conversion
	sseEvent := &SSEEvent{
		ID:        "test-id",
		Event:     "TEXT_MESSAGE_CONTENT",
		Data:      `{"message": "hello"}`,
		Timestamp: time.Now(),
		Sequence:  1,
		Headers:   map[string]string{"custom": "value"},
	}

	agEvent, err := ConvertSSEToEvent(sseEvent)
	if err != nil {
		t.Fatalf("Failed to convert SSE to AG-UI event: %v", err)
	}

	if agEvent.Type() != events.EventType("TEXT_MESSAGE_CONTENT") {
		t.Errorf("Expected event type TEXT_MESSAGE_CONTENT, got %s", agEvent.Type())
	}

	// Test AG-UI to SSE event conversion
	baseEvent := events.NewBaseEvent(events.EventTypeTextMessageStart)
	baseEvent.RawEvent = map[string]interface{}{
		"message": "test",
	}

	convertedSSE, err := ConvertEventToSSE(baseEvent)
	if err != nil {
		t.Fatalf("Failed to convert AG-UI to SSE event: %v", err)
	}

	if convertedSSE.Event != string(events.EventTypeTextMessageStart) {
		t.Errorf("Expected event type %s, got %s", events.EventTypeTextMessageStart, convertedSSE.Event)
	}

	if convertedSSE.Data == "" {
		t.Error("Expected non-empty data field")
	}
}

func TestSSEClient_ConcurrentAccess(t *testing.T) {
	events := []string{
		"data: concurrent test",
	}

	server := MockSSEServer(events, 10*time.Millisecond)
	defer server.Close()

	config := SSEClientConfig{
		URL: server.URL,
	}

	client, err := NewSSEClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSE client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Test concurrent access to client methods
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			// These should be safe to call concurrently
			_ = client.State()
			_ = client.LastEventID()
			_ = client.ReconnectCount()
			_ = client.BufferLength()
			_ = client.IsBackpressureActive()
		}()
	}

	wg.Wait()
}

func TestSSEClient_HealthCheck(t *testing.T) {
	// Server that stops sending data after first event
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "data: initial event\n\n")
		flusher.Flush()

		// Simulate server becoming unresponsive
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	var disconnectCalled int32 // Changed to int32 for atomic operations
	config := SSEClientConfig{
		URL:                 server.URL,
		HealthCheckInterval: 100 * time.Millisecond,
		OnDisconnect: func(err error) {
			atomic.StoreInt32(&disconnectCalled, 1) // Atomic write
		},
	}

	client, err := NewSSEClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSE client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Receive initial event
	eventChan := client.Events()
	select {
	case <-eventChan:
		// Got initial event
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for initial event")
	}

	// Wait for health check to detect inactive connection
	time.Sleep(500 * time.Millisecond)

	if atomic.LoadInt32(&disconnectCalled) == 0 { // Atomic read
		t.Error("Expected disconnect callback to be called due to health check failure")
	}
}

// BenchmarkSSEClient_EventThroughput benchmarks event processing throughput
func BenchmarkSSEClient_EventThroughput(b *testing.B) {
	// Create server with many events
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		for i := 0; i < b.N; i++ {
			fmt.Fprintf(w, "id: %d\ndata: benchmark event %d\n\n", i, i)
			if i%100 == 0 {
				flusher.Flush()
			}
		}
		flusher.Flush()
	}))
	defer server.Close()

	config := SSEClientConfig{
		URL:             server.URL,
		EventBufferSize: 10000,
	}

	client, err := NewSSEClient(config)
	if err != nil {
		b.Fatalf("Failed to create SSE client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}

	b.ResetTimer()
	
	eventChan := client.Events()
	eventCount := 0

	for eventCount < b.N {
		select {
		case <-eventChan:
			eventCount++
		case <-ctx.Done():
			b.Fatalf("Timeout after receiving %d/%d events", eventCount, b.N)
		}
	}
}