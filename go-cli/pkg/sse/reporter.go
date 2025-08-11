package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// MetricsReporter defines the interface for reporting SSE metrics
type MetricsReporter interface {
	// Report sends metrics to the reporter
	Report(ctx context.Context, metrics Metrics) error
	
	// ReportSummary sends a final summary when the connection closes
	ReportSummary(ctx context.Context, metrics Metrics) error
	
	// Start begins periodic reporting (if applicable)
	Start(ctx context.Context, health *SSEHealth, interval time.Duration) error
	
	// Stop stops the reporter
	Stop() error
}

// LoggerReporter reports metrics using logrus
type LoggerReporter struct {
	logger     *logrus.Logger
	format     string // "human" or "json"
	level      logrus.Level
	stopChan   chan struct{}
	stopped    bool
	redactKeys []string // Keys to redact from output
}

// NewLoggerReporter creates a new logger-based reporter
func NewLoggerReporter(logger *logrus.Logger, format string) *LoggerReporter {
	if logger == nil {
		logger = logrus.New()
	}
	
	if format != "json" && format != "human" {
		format = "human"
	}
	
	return &LoggerReporter{
		logger:   logger,
		format:   format,
		level:    logrus.InfoLevel,
		stopChan: make(chan struct{}),
		redactKeys: []string{
			"authorization",
			"api-key",
			"secret",
			"token",
			"password",
		},
	}
}

// Report logs the current metrics
func (r *LoggerReporter) Report(ctx context.Context, metrics Metrics) error {
	if r.format == "json" {
		return r.reportJSON(metrics)
	}
	return r.reportHuman(metrics)
}

// reportJSON outputs metrics in JSON format
func (r *LoggerReporter) reportJSON(metrics Metrics) error {
	// Create a sanitized version of metrics
	sanitized := r.sanitizeMetrics(metrics)
	
	// Add metadata
	output := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"type":      "sse_metrics",
		"metrics":   sanitized,
	}
	
	// Log as structured JSON
	r.logger.WithFields(logrus.Fields{
		"timestamp": output["timestamp"],
		"type":      output["type"],
		"metrics":   output["metrics"],
	}).Info("SSE Metrics")
	
	return nil
}

// reportHuman outputs metrics in human-readable format
func (r *LoggerReporter) reportHuman(metrics Metrics) error {
	fields := logrus.Fields{
		"connection_id": metrics.ConnectionID,
		"connected":     metrics.IsConnected,
		"events_total":  metrics.TotalEvents,
		"events_per_sec": fmt.Sprintf("%.2f", metrics.EventsPerSecond),
		"bytes_read":    r.formatBytes(metrics.BytesRead),
		"errors":        metrics.ErrorCount,
	}
	
	if metrics.ReconnectAttempts > 0 {
		fields["reconnects"] = metrics.ReconnectAttempts
	}
	
	if metrics.ConnectionDuration > 0 {
		fields["duration"] = metrics.ConnectionDuration.String()
	}
	
	r.logger.WithFields(fields).Info("SSE Metrics")
	return nil
}

// ReportSummary logs a final summary
func (r *LoggerReporter) ReportSummary(ctx context.Context, metrics Metrics) error {
	avgRate := metrics.GetAverageEventRate()
	errorRate := metrics.GetErrorRate()
	bytesPerEvent := metrics.GetBytesPerEvent()
	
	if r.format == "json" {
		summary := map[string]interface{}{
			"timestamp":          time.Now().Unix(),
			"type":              "sse_summary",
			"connection_id":     metrics.ConnectionID,
			"duration_seconds":  metrics.ConnectionDuration.Seconds(),
			"total_events":      metrics.TotalEvents,
			"total_bytes":       metrics.BytesRead,
			"avg_events_per_sec": avgRate,
			"error_rate_percent": errorRate,
			"bytes_per_event":   bytesPerEvent,
			"reconnect_attempts": metrics.ReconnectAttempts,
			"parse_errors":      metrics.ParseErrors,
		}
		
		data, err := json.Marshal(summary)
		if err != nil {
			return fmt.Errorf("failed to marshal summary: %w", err)
		}
		
		r.logger.WithField("sse_summary", true).Info(string(data))
	} else {
		r.logger.WithFields(logrus.Fields{
			"connection_id":     metrics.ConnectionID,
			"duration":         metrics.ConnectionDuration.String(),
			"total_events":     metrics.TotalEvents,
			"total_bytes":      r.formatBytes(metrics.BytesRead),
			"avg_rate":         fmt.Sprintf("%.2f events/sec", avgRate),
			"error_rate":       fmt.Sprintf("%.2f%%", errorRate),
			"bytes_per_event":  fmt.Sprintf("%.0f", bytesPerEvent),
			"reconnects":       metrics.ReconnectAttempts,
			"parse_errors":     metrics.ParseErrors,
		}).Info("SSE Connection Summary")
	}
	
	return nil
}

// Start begins periodic reporting
func (r *LoggerReporter) Start(ctx context.Context, health *SSEHealth, interval time.Duration) error {
	if r.stopped {
		return fmt.Errorf("reporter already stopped")
	}
	
	ticker := time.NewTicker(interval)
	
	go func() {
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.stopChan:
				return
			case <-ticker.C:
				metrics := health.GetMetrics()
				if err := r.Report(ctx, metrics); err != nil {
					r.logger.WithError(err).Warn("Failed to report metrics")
				}
			}
		}
	}()
	
	return nil
}

// Stop stops the reporter
func (r *LoggerReporter) Stop() error {
	if !r.stopped {
		close(r.stopChan)
		r.stopped = true
	}
	return nil
}

// sanitizeMetrics removes sensitive information from metrics
func (r *LoggerReporter) sanitizeMetrics(metrics Metrics) map[string]interface{} {
	// Convert to map for easier manipulation
	data, _ := json.Marshal(metrics)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	
	// Redact sensitive fields
	for key := range result {
		for _, redactKey := range r.redactKeys {
			if strings.Contains(strings.ToLower(key), redactKey) {
				result[key] = "[REDACTED]"
			}
		}
	}
	
	return result
}

// formatBytes formats bytes into human-readable format
func (r *LoggerReporter) formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// CallbackReporter reports metrics via callback functions
type CallbackReporter struct {
	onReport      func(metrics Metrics) error
	onSummary     func(metrics Metrics) error
	stopChan      chan struct{}
	stopped       bool
}

// NewCallbackReporter creates a new callback-based reporter
func NewCallbackReporter(onReport, onSummary func(metrics Metrics) error) *CallbackReporter {
	return &CallbackReporter{
		onReport:  onReport,
		onSummary: onSummary,
		stopChan:  make(chan struct{}),
	}
}

// Report calls the report callback
func (r *CallbackReporter) Report(ctx context.Context, metrics Metrics) error {
	if r.onReport != nil {
		return r.onReport(metrics)
	}
	return nil
}

// ReportSummary calls the summary callback
func (r *CallbackReporter) ReportSummary(ctx context.Context, metrics Metrics) error {
	if r.onSummary != nil {
		return r.onSummary(metrics)
	}
	return nil
}

// Start begins periodic reporting
func (r *CallbackReporter) Start(ctx context.Context, health *SSEHealth, interval time.Duration) error {
	if r.stopped {
		return fmt.Errorf("reporter already stopped")
	}
	
	if r.onReport == nil {
		return nil // Nothing to do
	}
	
	ticker := time.NewTicker(interval)
	
	go func() {
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.stopChan:
				return
			case <-ticker.C:
				metrics := health.GetMetrics()
				if err := r.Report(ctx, metrics); err != nil {
					// Log error if possible, but don't stop
					fmt.Printf("Callback reporter error: %v\n", err)
				}
			}
		}
	}()
	
	return nil
}

// Stop stops the reporter
func (r *CallbackReporter) Stop() error {
	if !r.stopped {
		close(r.stopChan)
		r.stopped = true
	}
	return nil
}

// MultiReporter combines multiple reporters
type MultiReporter struct {
	reporters []MetricsReporter
}

// NewMultiReporter creates a reporter that sends to multiple destinations
func NewMultiReporter(reporters ...MetricsReporter) *MultiReporter {
	return &MultiReporter{
		reporters: reporters,
	}
}

// Report sends metrics to all reporters
func (m *MultiReporter) Report(ctx context.Context, metrics Metrics) error {
	var errs []error
	for _, r := range m.reporters {
		if err := r.Report(ctx, metrics); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("multi-reporter errors: %v", errs)
	}
	return nil
}

// ReportSummary sends summary to all reporters
func (m *MultiReporter) ReportSummary(ctx context.Context, metrics Metrics) error {
	var errs []error
	for _, r := range m.reporters {
		if err := r.ReportSummary(ctx, metrics); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("multi-reporter summary errors: %v", errs)
	}
	return nil
}

// Start starts all reporters
func (m *MultiReporter) Start(ctx context.Context, health *SSEHealth, interval time.Duration) error {
	for _, r := range m.reporters {
		if err := r.Start(ctx, health, interval); err != nil {
			return fmt.Errorf("failed to start reporter: %w", err)
		}
	}
	return nil
}

// Stop stops all reporters
func (m *MultiReporter) Stop() error {
	var errs []error
	for _, r := range m.reporters {
		if err := r.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("multi-reporter stop errors: %v", errs)
	}
	return nil
}