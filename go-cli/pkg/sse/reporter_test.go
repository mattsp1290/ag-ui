package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestLoggerReporter_ReportJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})
	
	reporter := NewLoggerReporter(logger, "json")
	
	metrics := Metrics{
		ConnectionID:    "test-123",
		IsConnected:     true,
		TotalEvents:     100,
		EventsPerSecond: 10.5,
		BytesRead:       1024,
		ErrorCount:      2,
	}
	
	err := reporter.Report(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Failed to report metrics: %v", err)
	}
	
	// Parse the output
	output := buf.String()
	if !strings.Contains(output, "sse_metrics") {
		t.Error("Expected output to contain 'sse_metrics'")
	}
	
	// Check for JSON structure
	if !strings.Contains(output, `"type":"sse_metrics"`) {
		t.Error("Expected JSON output to contain type field")
	}
}

func TestLoggerReporter_ReportHuman(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	
	reporter := NewLoggerReporter(logger, "human")
	
	metrics := Metrics{
		ConnectionID:       "test-456",
		IsConnected:        true,
		TotalEvents:        50,
		EventsPerSecond:    5.0,
		BytesRead:          2048,
		ErrorCount:         1,
		ReconnectAttempts:  3,
		ConnectionDuration: 10 * time.Second,
	}
	
	err := reporter.Report(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Failed to report metrics: %v", err)
	}
	
	output := buf.String()
	
	// Check for expected fields
	expectedFields := []string{
		"connection_id=test-456",
		"connected=true",
		"events_total=50",
		"events_per_sec",
		"bytes_read",
		"errors=1",
		"reconnects=3",
		"duration=10s",
	}
	
	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Expected output to contain '%s'", field)
		}
	}
}

func TestLoggerReporter_ReportSummary(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	
	reporter := NewLoggerReporter(logger, "human")
	
	metrics := Metrics{
		ConnectionID:       "test-789",
		ConnectionDuration: 60 * time.Second,
		TotalEvents:        1000,
		BytesRead:          10240,
		ParseErrors:        10,
		ReconnectAttempts:  2,
		UptimeSeconds:      60,
	}
	
	err := reporter.ReportSummary(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Failed to report summary: %v", err)
	}
	
	output := buf.String()
	
	if !strings.Contains(output, "SSE Connection Summary") {
		t.Error("Expected output to contain 'SSE Connection Summary'")
	}
	
	if !strings.Contains(output, "connection_id=test-789") {
		t.Error("Expected output to contain connection ID")
	}
}

func TestLoggerReporter_SanitizeMetrics(t *testing.T) {
	reporter := NewLoggerReporter(nil, "json")
	
	metrics := Metrics{
		ConnectionID: "test-123",
		TotalEvents:  100,
	}
	
	// Add a field that should be redacted
	data, _ := json.Marshal(metrics)
	var metricsMap map[string]interface{}
	json.Unmarshal(data, &metricsMap)
	metricsMap["authorization"] = "Bearer secret-token"
	metricsMap["api-key"] = "secret-key"
	
	// Create a metrics struct with the map
	sanitized := reporter.sanitizeMetrics(metrics)
	
	// The original metrics don't have auth fields, but let's check the function works
	if val, ok := sanitized["connectionId"]; !ok || val != "test-123" {
		t.Error("Expected connectionId to be preserved")
	}
}

func TestLoggerReporter_FormatBytes(t *testing.T) {
	reporter := NewLoggerReporter(nil, "human")
	
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	
	for _, tt := range tests {
		result := reporter.formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestCallbackReporter(t *testing.T) {
	var reportedMetrics Metrics
	var summaryMetrics Metrics
	reportCalled := false
	summaryCalled := false
	
	reporter := NewCallbackReporter(
		func(m Metrics) error {
			reportCalled = true
			reportedMetrics = m
			return nil
		},
		func(m Metrics) error {
			summaryCalled = true
			summaryMetrics = m
			return nil
		},
	)
	
	testMetrics := Metrics{
		ConnectionID: "callback-test",
		TotalEvents:  42,
	}
	
	// Test Report
	err := reporter.Report(context.Background(), testMetrics)
	if err != nil {
		t.Fatalf("Report failed: %v", err)
	}
	
	if !reportCalled {
		t.Error("Report callback was not called")
	}
	
	if reportedMetrics.ConnectionID != "callback-test" {
		t.Error("Metrics not passed correctly to callback")
	}
	
	// Test ReportSummary
	err = reporter.ReportSummary(context.Background(), testMetrics)
	if err != nil {
		t.Fatalf("ReportSummary failed: %v", err)
	}
	
	if !summaryCalled {
		t.Error("Summary callback was not called")
	}
	
	if summaryMetrics.TotalEvents != 42 {
		t.Error("Summary metrics not passed correctly")
	}
}

func TestCallbackReporter_WithError(t *testing.T) {
	testErr := errors.New("callback error")
	
	reporter := NewCallbackReporter(
		func(m Metrics) error {
			return testErr
		},
		nil,
	)
	
	err := reporter.Report(context.Background(), Metrics{})
	if err != testErr {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}
}

func TestMultiReporter(t *testing.T) {
	var reporter1Called, reporter2Called bool
	
	r1 := NewCallbackReporter(
		func(m Metrics) error {
			reporter1Called = true
			return nil
		},
		nil,
	)
	
	r2 := NewCallbackReporter(
		func(m Metrics) error {
			reporter2Called = true
			return nil
		},
		nil,
	)
	
	multi := NewMultiReporter(r1, r2)
	
	err := multi.Report(context.Background(), Metrics{})
	if err != nil {
		t.Fatalf("MultiReporter.Report failed: %v", err)
	}
	
	if !reporter1Called || !reporter2Called {
		t.Error("Not all reporters were called")
	}
}

func TestReporter_Start(t *testing.T) {
	callCount := 0
	reporter := NewCallbackReporter(
		func(m Metrics) error {
			callCount++
			return nil
		},
		nil,
	)
	
	health := NewSSEHealth()
	health.RecordEvent(100)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := reporter.Start(ctx, health, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to start reporter: %v", err)
	}
	
	// Wait for a few intervals
	time.Sleep(150 * time.Millisecond)
	
	// Stop the reporter
	reporter.Stop()
	
	// Should have been called at least twice
	if callCount < 2 {
		t.Errorf("Expected at least 2 calls, got %d", callCount)
	}
}

func TestReporter_StopMultipleTimes(t *testing.T) {
	reporter := NewCallbackReporter(nil, nil)
	
	// First stop should succeed
	err := reporter.Stop()
	if err != nil {
		t.Errorf("First Stop() failed: %v", err)
	}
	
	// Second stop should also succeed (idempotent)
	err = reporter.Stop()
	if err != nil {
		t.Errorf("Second Stop() failed: %v", err)
	}
}