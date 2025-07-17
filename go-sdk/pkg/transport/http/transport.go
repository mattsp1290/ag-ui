package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	// Using local stubs for testing
)

// EventValidator defines the interface for event validation
type EventValidator interface {
	ValidateEvent(ctx context.Context, event Event) *ValidationResult
}

// StandardEventValidator is a basic implementation of EventValidator
type StandardEventValidator struct{}

// ValidateEvent validates an event using basic rules
func (v *StandardEventValidator) ValidateEvent(ctx context.Context, event Event) *ValidationResult {
	result := &ValidationResult{
		IsValid:   true,
		Timestamp: time.Now(),
	}

	// Basic validation - check if event is nil
	if event == nil {
		result.AddError(&ValidationError{
			RuleID:    "null_event",
			EventType: "",
			Message:   "event cannot be nil",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		})
		return result
	}

	// Check event type
	if event.Type() == "" {
		result.AddError(&ValidationError{
			RuleID:    "empty_event_type",
			EventType: event.Type(),
			Message:   "event type cannot be empty",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		})
	}

	return result
}

// TransportConfig holds the configuration for HTTP transport
type TransportConfig struct {
	// BaseURL is the base URL for the HTTP endpoint
	BaseURL string

	// RequestTimeout is the timeout for HTTP requests
	RequestTimeout time.Duration

	// MaxEventSize is the maximum size of an event in bytes
	MaxEventSize int64

	// MaxBatchSize is the maximum number of events in a batch
	MaxBatchSize int

	// EnableEventValidation enables event validation before sending
	EnableEventValidation bool

	// EventValidator is the validator to use for events
	EventValidator EventValidator

	// Logger is the logger instance
	Logger *zap.Logger

	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// RetryBackoff is the base backoff duration for retries
	RetryBackoff time.Duration

	// TLSConfig is the TLS configuration for HTTPS connections
	TLSConfig *tls.Config

	// HTTPClient is a custom HTTP client (optional)
	HTTPClient *http.Client

	// AuthToken is the bearer token for authentication
	AuthToken string

	// Headers are custom headers to send with requests
	Headers map[string]string

	// EnableCircuitBreaker enables circuit breaker pattern
	EnableCircuitBreaker bool

	// CircuitBreakerThreshold is the failure threshold for circuit breaker
	CircuitBreakerThreshold int

	// CircuitBreakerTimeout is the timeout for circuit breaker
	CircuitBreakerTimeout time.Duration

	// EnableCompression enables gzip compression
	EnableCompression bool

	// RequestMiddleware are middleware functions for requests
	RequestMiddleware []RequestMiddleware

	// ResponseMiddleware are middleware functions for responses
	ResponseMiddleware []ResponseMiddleware

	// EnableMetrics enables metrics collection
	EnableMetrics bool
}

// RequestMiddleware is a function that can modify HTTP requests
type RequestMiddleware func(*http.Request) error

// ResponseMiddleware is a function that can process HTTP responses
type ResponseMiddleware func(*http.Response) error

// HTTPTransport implements the Transport interface for HTTP
type HTTPTransport struct {
	config         *TransportConfig
	client         *http.Client
	logger         *zap.Logger
	stats          *TransportStats
	metrics        *TransportMetrics
	mu             sync.RWMutex
	connected      bool
	startTime      time.Time
	activeRequests int64
	circuitBreaker *CircuitBreaker
}

// TransportStats holds transport statistics with mutex
type TransportStats struct {
	EventsSent       int64
	EventsReceived   int64
	EventsFailed     int64
	BytesTransferred int64
	AverageLatency   time.Duration
	mu               sync.RWMutex
}

// TransportMetrics holds detailed metrics
type TransportMetrics struct {
	TotalRequests              int64
	SuccessfulRequests         int64
	FailedRequests             int64
	TotalBytesSent             int64
	TotalBytesReceived         int64
	AverageRequestDuration     time.Duration
	RequestDurationPercentiles map[string]time.Duration
	ErrorsByType               map[string]int64
	mu                         sync.RWMutex
}

// CircuitBreaker implements a simple circuit breaker
type CircuitBreaker struct {
	threshold     int
	timeout       time.Duration
	failures      int32
	lastFailTime  time.Time
	state         int32 // 0: closed, 1: open, 2: half-open
	mu            sync.Mutex
}

// States for circuit breaker
const (
	CircuitClosed   = int32(0)
	CircuitOpen     = int32(1)
	CircuitHalfOpen = int32(2)
)

// ShouldAllow checks if the request should be allowed
func (cb *CircuitBreaker) ShouldAllow() bool {
	if cb == nil {
		return true
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := atomic.LoadInt32(&cb.state)

	switch state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailTime) > cb.timeout {
			atomic.StoreInt32(&cb.state, CircuitHalfOpen)
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	if cb == nil {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.StoreInt32(&cb.failures, 0)
	atomic.StoreInt32(&cb.state, CircuitClosed)
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	if cb == nil {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	failures := atomic.AddInt32(&cb.failures, 1)
	cb.lastFailTime = time.Now()

	if int(failures) >= cb.threshold {
		atomic.StoreInt32(&cb.state, CircuitOpen)
	}
}

// DefaultTransportConfig returns the default HTTP transport configuration
func DefaultTransportConfig() *TransportConfig {
	return &TransportConfig{
		BaseURL:               "http://localhost:8080",
		RequestTimeout:        30 * time.Second,
		MaxEventSize:          10 * 1024 * 1024, // 10MB
		MaxBatchSize:          100,
		EnableEventValidation: true,
		EventValidator:        &StandardEventValidator{},
		Logger:                zap.NewNop(),
		MaxRetries:            3,
		RetryBackoff:          1 * time.Second,
		EnableMetrics:         true,
	}
}

// NewTransport creates a new HTTP transport
func NewTransport(config *TransportConfig) (*HTTPTransport, error) {
	if config == nil {
		config = DefaultTransportConfig()
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL must be provided")
	}

	// Validate URL
	if _, err := url.Parse(config.BaseURL); err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	if config.Logger == nil {
		config.Logger = zap.NewNop()
	}

	// Create HTTP client if not provided
	client := config.HTTPClient
	if client == nil {
		tr := &http.Transport{
			TLSClientConfig: config.TLSConfig,
			MaxIdleConns:    100,
			IdleConnTimeout: 90 * time.Second,
		}
		client = &http.Client{
			Transport: tr,
			Timeout:   config.RequestTimeout,
		}
	}

	t := &HTTPTransport{
		config: config,
		client: client,
		logger: config.Logger,
		stats:  &TransportStats{},
		metrics: &TransportMetrics{
			RequestDurationPercentiles: make(map[string]time.Duration),
			ErrorsByType:               make(map[string]int64),
		},
	}

	if config.EnableCircuitBreaker {
		t.circuitBreaker = &CircuitBreaker{
			threshold: config.CircuitBreakerThreshold,
			timeout:   config.CircuitBreakerTimeout,
		}
	}

	return t, nil
}

// Start starts the HTTP transport
func (t *HTTPTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	// Test connection with a health check
	if err := t.performHealthCheck(ctx); err != nil {
		t.logger.Warn("Health check failed during start", zap.Error(err))
		// Don't fail start, as server might become available later
	}

	t.connected = true
	t.startTime = time.Now()
	t.logger.Info("HTTP transport started", zap.String("baseURL", t.config.BaseURL))

	return nil
}

// Stop stops the HTTP transport
func (t *HTTPTransport) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	// Wait for active requests to complete
	for atomic.LoadInt64(&t.activeRequests) > 0 {
		time.Sleep(10 * time.Millisecond)
	}

	t.connected = false
	t.logger.Info("HTTP transport stopped")

	return nil
}

// IsConnected returns whether the transport is connected
func (t *HTTPTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// SendEvent sends a single event
func (t *HTTPTransport) SendEvent(ctx context.Context, event Event) error {
	if !t.IsConnected() {
		return fmt.Errorf("transport is not connected")
	}

	// Check context
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	// Validate event if enabled
	if t.config.EnableEventValidation && t.config.EventValidator != nil {
		result := t.config.EventValidator.ValidateEvent(ctx, event)
		if !result.IsValid {
			t.updateStats(0, 0, 1)
			return fmt.Errorf("event validation failed: %v", result.Errors)
		}
	}

	// Serialize event
	data, err := event.ToJSON()
	if err != nil {
		t.updateStats(0, 0, 1)
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Check event size
	if int64(len(data)) > t.config.MaxEventSize {
		t.updateStats(0, 0, 1)
		return fmt.Errorf("event size %d exceeds maximum %d", len(data), t.config.MaxEventSize)
	}

	// Send with retry
	return t.sendWithRetry(ctx, "/events", data)
}

// SendBatch sends multiple events in a batch
func (t *HTTPTransport) SendBatch(ctx context.Context, events []Event) error {
	if !t.IsConnected() {
		return fmt.Errorf("transport is not connected")
	}

	if len(events) == 0 {
		return fmt.Errorf("empty batch")
	}

	if len(events) > t.config.MaxBatchSize {
		return fmt.Errorf("batch size %d exceeds maximum %d", len(events), t.config.MaxBatchSize)
	}

	// Validate and serialize events
	var batch []json.RawMessage
	totalSize := int64(0)

	for _, event := range events {
		if t.config.EnableEventValidation && t.config.EventValidator != nil {
			result := t.config.EventValidator.ValidateEvent(ctx, event)
			if !result.IsValid {
				return fmt.Errorf("event validation failed: %v", result.Errors)
			}
		}

		data, err := event.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize event: %w", err)
		}

		totalSize += int64(len(data))
		if totalSize > t.config.MaxEventSize {
			return fmt.Errorf("batch size exceeds maximum")
		}

		batch = append(batch, json.RawMessage(data))
	}

	// Serialize batch
	batchData, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to serialize batch: %w", err)
	}

	// Send with retry
	return t.sendWithRetry(ctx, "/batch", batchData)
}

// Ping performs a health check
func (t *HTTPTransport) Ping(ctx context.Context) error {
	return t.performHealthCheck(ctx)
}

// GetStats returns transport statistics
func (t *HTTPTransport) GetStats() PublicTransportStats {
	t.stats.mu.RLock()
	defer t.stats.mu.RUnlock()

	return PublicTransportStats{
		EventsSent:       t.stats.EventsSent,
		EventsReceived:   t.stats.EventsReceived,
		EventsFailed:     t.stats.EventsFailed,
		BytesTransferred: t.stats.BytesTransferred,
		AverageLatency:   t.stats.AverageLatency,
	}
}

// GetDetailedStatus returns detailed status information
func (t *HTTPTransport) GetDetailedStatus() map[string]interface{} {
	stats := t.GetStats()
	
	return map[string]interface{}{
		"transport_stats": stats,
		"http_client": map[string]interface{}{
			"timeout":        t.client.Timeout,
			"max_idle_conns": 100, // Default value
		},
		"active_requests": atomic.LoadInt64(&t.activeRequests),
		"configuration": map[string]interface{}{
			"base_url":       t.config.BaseURL,
			"max_retries":    t.config.MaxRetries,
			"retry_backoff":  t.config.RetryBackoff,
			"max_event_size": t.config.MaxEventSize,
		},
	}
}

// GetMetrics returns transport metrics
func (t *HTTPTransport) GetMetrics() *TransportMetrics {
	if !t.config.EnableMetrics {
		return nil
	}

	t.metrics.mu.RLock()
	defer t.metrics.mu.RUnlock()

	// Create a copy to avoid race conditions
	metricsCopy := &TransportMetrics{
		TotalRequests:              t.metrics.TotalRequests,
		SuccessfulRequests:         t.metrics.SuccessfulRequests,
		FailedRequests:             t.metrics.FailedRequests,
		TotalBytesSent:             t.metrics.TotalBytesSent,
		TotalBytesReceived:         t.metrics.TotalBytesReceived,
		AverageRequestDuration:     t.metrics.AverageRequestDuration,
		RequestDurationPercentiles: make(map[string]time.Duration),
		ErrorsByType:               make(map[string]int64),
	}

	// Copy maps
	for k, v := range t.metrics.RequestDurationPercentiles {
		metricsCopy.RequestDurationPercentiles[k] = v
	}
	for k, v := range t.metrics.ErrorsByType {
		metricsCopy.ErrorsByType[k] = v
	}

	// Add default percentiles if not present
	if _, ok := metricsCopy.RequestDurationPercentiles["p50"]; !ok {
		metricsCopy.RequestDurationPercentiles["p50"] = t.metrics.AverageRequestDuration
	}
	if _, ok := metricsCopy.RequestDurationPercentiles["p95"]; !ok {
		metricsCopy.RequestDurationPercentiles["p95"] = t.metrics.AverageRequestDuration * 2
	}
	if _, ok := metricsCopy.RequestDurationPercentiles["p99"]; !ok {
		metricsCopy.RequestDurationPercentiles["p99"] = t.metrics.AverageRequestDuration * 3
	}

	return metricsCopy
}

// Helper methods

func (t *HTTPTransport) sendWithRetry(ctx context.Context, endpoint string, data []byte) error {
	var lastErr error
	
	for attempt := 0; attempt <= t.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := t.config.RetryBackoff * time.Duration(1<<(attempt-1))
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := t.sendRequest(ctx, endpoint, data)
		if err == nil {
			return nil
		}

		lastErr = err
		t.logger.Warn("Request failed, retrying",
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.Int("maxRetries", t.config.MaxRetries))
	}

	return fmt.Errorf("failed after %d retries: %w", t.config.MaxRetries, lastErr)
}

func (t *HTTPTransport) sendRequest(ctx context.Context, endpoint string, data []byte) error {
	// Check circuit breaker
	if !t.circuitBreaker.ShouldAllow() {
		return fmt.Errorf("circuit breaker open")
	}

	atomic.AddInt64(&t.activeRequests, 1)
	defer atomic.AddInt64(&t.activeRequests, -1)

	start := time.Now()

	// Create request
	url := t.config.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.updateStats(0, 0, 1)
		t.circuitBreaker.RecordFailure()
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if t.config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+t.config.AuthToken)
	}
	for k, v := range t.config.Headers {
		req.Header.Set(k, v)
	}

	// Apply request middleware
	for _, mw := range t.config.RequestMiddleware {
		if err := mw(req); err != nil {
			return fmt.Errorf("request middleware error: %w", err)
		}
	}

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		t.updateStats(0, 0, 1)
		t.recordError("connection_refused")
		t.circuitBreaker.RecordFailure()
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Apply response middleware
	for _, mw := range t.config.ResponseMiddleware {
		if err := mw(resp); err != nil {
			return fmt.Errorf("response middleware error: %w", err)
		}
	}

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.updateStats(0, 0, 1)
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode >= 400 {
		t.updateStats(0, 0, 1)
		t.circuitBreaker.RecordFailure()
		if resp.StatusCode >= 500 {
			return fmt.Errorf("server error: status %d, body: %s", resp.StatusCode, string(respBody))
		}
		return fmt.Errorf("client error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	// Update stats
	duration := time.Since(start)
	t.updateStats(int64(len(data)), int64(len(respBody)), 0)
	t.updateLatency(duration)
	t.circuitBreaker.RecordSuccess()

	return nil
}

func (t *HTTPTransport) performHealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.config.BaseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	return nil
}

func (t *HTTPTransport) updateStats(bytesSent, bytesReceived, failed int64) {
	t.stats.mu.Lock()
	defer t.stats.mu.Unlock()

	if failed > 0 {
		t.stats.EventsFailed += failed
	} else {
		t.stats.EventsSent++
		t.stats.BytesTransferred += bytesSent + bytesReceived
	}

	if t.config.EnableMetrics {
		t.metrics.mu.Lock()
		defer t.metrics.mu.Unlock()

		t.metrics.TotalRequests++
		if failed > 0 {
			t.metrics.FailedRequests++
		} else {
			t.metrics.SuccessfulRequests++
			t.metrics.TotalBytesSent += bytesSent
			t.metrics.TotalBytesReceived += bytesReceived
		}
	}
}

func (t *HTTPTransport) updateLatency(duration time.Duration) {
	t.stats.mu.Lock()
	defer t.stats.mu.Unlock()

	// Simple moving average
	if t.stats.AverageLatency == 0 {
		t.stats.AverageLatency = duration
	} else {
		t.stats.AverageLatency = (t.stats.AverageLatency + duration) / 2
	}

	if t.config.EnableMetrics {
		t.metrics.mu.Lock()
		defer t.metrics.mu.Unlock()

		if t.metrics.AverageRequestDuration == 0 {
			t.metrics.AverageRequestDuration = duration
		} else {
			t.metrics.AverageRequestDuration = (t.metrics.AverageRequestDuration + duration) / 2
		}
	}
}

func (t *HTTPTransport) recordError(errorType string) {
	if t.config.EnableMetrics {
		t.metrics.mu.Lock()
		defer t.metrics.mu.Unlock()

		t.metrics.ErrorsByType[errorType]++
	}
}