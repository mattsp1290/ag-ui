package sse

import (
	"errors"
	"testing"
	"time"
)

func TestSSEHealth_RecordConnect(t *testing.T) {
	h := NewSSEHealth()
	
	connID := "test-conn-123"
	h.RecordConnect(connID)
	
	metrics := h.GetMetrics()
	
	if metrics.ConnectionID != connID {
		t.Errorf("Expected connection ID %s, got %s", connID, metrics.ConnectionID)
	}
	
	if !metrics.IsConnected {
		t.Error("Expected IsConnected to be true")
	}
	
	if metrics.ConnectedAt.IsZero() {
		t.Error("Expected ConnectedAt to be set")
	}
	
	if !metrics.DisconnectedAt.IsZero() {
		t.Error("Expected DisconnectedAt to be zero")
	}
}

func TestSSEHealth_RecordDisconnect(t *testing.T) {
	h := NewSSEHealth()
	
	// Connect first
	h.RecordConnect("test-conn")
	time.Sleep(10 * time.Millisecond)
	
	// Then disconnect
	testErr := errors.New("test disconnection")
	h.RecordDisconnect(testErr)
	
	metrics := h.GetMetrics()
	
	if metrics.IsConnected {
		t.Error("Expected IsConnected to be false")
	}
	
	if metrics.DisconnectedAt.IsZero() {
		t.Error("Expected DisconnectedAt to be set")
	}
	
	if metrics.ConnectionDuration <= 0 {
		t.Error("Expected positive ConnectionDuration")
	}
	
	if metrics.ErrorCount != 1 {
		t.Errorf("Expected ErrorCount to be 1, got %d", metrics.ErrorCount)
	}
}

func TestSSEHealth_RecordEvent(t *testing.T) {
	h := NewSSEHealth()
	
	// Record multiple events
	eventSizes := []int{100, 200, 150}
	for _, size := range eventSizes {
		h.RecordEvent(size)
	}
	
	metrics := h.GetMetrics()
	
	if metrics.FramesRead != 3 {
		t.Errorf("Expected FramesRead to be 3, got %d", metrics.FramesRead)
	}
	
	if metrics.TotalEvents != 3 {
		t.Errorf("Expected TotalEvents to be 3, got %d", metrics.TotalEvents)
	}
	
	expectedBytes := uint64(450)
	if metrics.BytesRead != expectedBytes {
		t.Errorf("Expected BytesRead to be %d, got %d", expectedBytes, metrics.BytesRead)
	}
	
	if metrics.LastEventAt.IsZero() {
		t.Error("Expected LastEventAt to be set")
	}
}

func TestSSEHealth_RecordParseError(t *testing.T) {
	h := NewSSEHealth()
	
	// Record parse errors
	for i := 0; i < 5; i++ {
		h.RecordParseError(errors.New("parse error"))
	}
	
	metrics := h.GetMetrics()
	
	if metrics.ParseErrors != 5 {
		t.Errorf("Expected ParseErrors to be 5, got %d", metrics.ParseErrors)
	}
	
	if metrics.ErrorCount != 5 {
		t.Errorf("Expected ErrorCount to be 5, got %d", metrics.ErrorCount)
	}
	
	if metrics.LastError == nil {
		t.Error("Expected LastError to be set")
	}
}

func TestSSEHealth_RecordReconnectAttempt(t *testing.T) {
	h := NewSSEHealth()
	
	// Record reconnect attempts
	for i := 0; i < 3; i++ {
		h.RecordReconnectAttempt()
	}
	
	metrics := h.GetMetrics()
	
	if metrics.ReconnectAttempts != 3 {
		t.Errorf("Expected ReconnectAttempts to be 3, got %d", metrics.ReconnectAttempts)
	}
}

func TestSSEHealth_GetEventsPerSecond(t *testing.T) {
	h := NewSSEHealth()
	
	// Record events at a known rate
	start := time.Now()
	for i := 0; i < 10; i++ {
		h.RecordEvent(100)
		time.Sleep(100 * time.Millisecond)
	}
	elapsed := time.Since(start)
	
	rate := h.GetEventsPerSecond()
	expectedRate := float64(10) / elapsed.Seconds()
	
	// Allow for some variance due to timing
	tolerance := 2.0
	if rate < expectedRate-tolerance || rate > expectedRate+tolerance {
		t.Errorf("Expected rate around %.2f events/sec, got %.2f", expectedRate, rate)
	}
}

func TestSSEHealth_Reset(t *testing.T) {
	h := NewSSEHealth()
	
	// Set up some data
	h.RecordConnect("test-conn")
	h.RecordEvent(100)
	h.RecordParseError(errors.New("test"))
	h.RecordReconnectAttempt()
	
	// Reset
	h.Reset()
	
	metrics := h.GetMetrics()
	
	// Check all counters are reset
	if metrics.BytesRead != 0 {
		t.Errorf("Expected BytesRead to be 0, got %d", metrics.BytesRead)
	}
	
	if metrics.FramesRead != 0 {
		t.Errorf("Expected FramesRead to be 0, got %d", metrics.FramesRead)
	}
	
	if metrics.ParseErrors != 0 {
		t.Errorf("Expected ParseErrors to be 0, got %d", metrics.ParseErrors)
	}
	
	if metrics.ReconnectAttempts != 0 {
		t.Errorf("Expected ReconnectAttempts to be 0, got %d", metrics.ReconnectAttempts)
	}
	
	if metrics.IsConnected {
		t.Error("Expected IsConnected to be false")
	}
	
	if metrics.ConnectionID != "" {
		t.Errorf("Expected ConnectionID to be empty, got %s", metrics.ConnectionID)
	}
}

func TestMetrics_GetAverageEventRate(t *testing.T) {
	metrics := Metrics{
		TotalEvents:   100,
		UptimeSeconds: 10,
	}
	
	avgRate := metrics.GetAverageEventRate()
	expected := 10.0
	
	if avgRate != expected {
		t.Errorf("Expected average rate %.2f, got %.2f", expected, avgRate)
	}
	
	// Test with zero uptime
	metrics.UptimeSeconds = 0
	avgRate = metrics.GetAverageEventRate()
	if avgRate != 0 {
		t.Errorf("Expected average rate 0 for zero uptime, got %.2f", avgRate)
	}
}

func TestMetrics_GetErrorRate(t *testing.T) {
	metrics := Metrics{
		TotalEvents: 95,
		ParseErrors: 5,
	}
	
	errorRate := metrics.GetErrorRate()
	expected := 5.0
	
	if errorRate != expected {
		t.Errorf("Expected error rate %.2f%%, got %.2f%%", expected, errorRate)
	}
	
	// Test with no events
	metrics.TotalEvents = 0
	metrics.ParseErrors = 0
	errorRate = metrics.GetErrorRate()
	if errorRate != 0 {
		t.Errorf("Expected error rate 0%% for no events, got %.2f%%", errorRate)
	}
}

func TestMetrics_GetBytesPerEvent(t *testing.T) {
	metrics := Metrics{
		BytesRead:  1000,
		FramesRead: 10,
	}
	
	bytesPerEvent := metrics.GetBytesPerEvent()
	expected := 100.0
	
	if bytesPerEvent != expected {
		t.Errorf("Expected %.0f bytes per event, got %.0f", expected, bytesPerEvent)
	}
	
	// Test with no frames
	metrics.FramesRead = 0
	bytesPerEvent = metrics.GetBytesPerEvent()
	if bytesPerEvent != 0 {
		t.Errorf("Expected 0 bytes per event for no frames, got %.0f", bytesPerEvent)
	}
}

func TestSSEHealth_ConcurrentAccess(t *testing.T) {
	h := NewSSEHealth()
	
	// Simulate concurrent access
	done := make(chan bool)
	
	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				h.RecordEvent(100)
				if j%10 == 0 {
					h.RecordParseError(errors.New("test"))
				}
				if j%20 == 0 {
					h.RecordReconnectAttempt()
				}
			}
			done <- true
		}(i)
	}
	
	// Reader goroutines
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = h.GetMetrics()
				_ = h.GetEventsPerSecond()
				time.Sleep(time.Microsecond)
			}
			done <- true
		}()
	}
	
	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}
	
	// Verify final metrics are consistent
	metrics := h.GetMetrics()
	
	if metrics.TotalEvents != 1000 {
		t.Errorf("Expected TotalEvents to be 1000, got %d", metrics.TotalEvents)
	}
	
	if metrics.ParseErrors != 100 {
		t.Errorf("Expected ParseErrors to be 100, got %d", metrics.ParseErrors)
	}
	
	if metrics.ReconnectAttempts != 50 {
		t.Errorf("Expected ReconnectAttempts to be 50, got %d", metrics.ReconnectAttempts)
	}
}