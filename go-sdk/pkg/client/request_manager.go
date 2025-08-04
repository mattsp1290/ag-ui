// Package client provides request/response management utilities for the ag-ui Go SDK.
package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"

	pkgerrors "github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

var (
	// Request manager errors
	ErrRequestCancelled     = fmt.Errorf("request cancelled")
	ErrRequestTimeout       = fmt.Errorf("request timeout")
	ErrInvalidURL          = fmt.Errorf("invalid URL")
	ErrInvalidMethod       = fmt.Errorf("invalid HTTP method")
	ErrSecurityViolation   = fmt.Errorf("security validation failed")
	ErrRateLimitExceeded   = fmt.Errorf("rate limit exceeded")
	ErrCircuitBreakerOpen  = fmt.Errorf("circuit breaker open")
)

// RequestManager handles HTTP request construction, validation, and response processing
// with comprehensive error handling, metrics, and middleware support.
type RequestManager struct {
	httpClient       *http.Client
	config           RequestManagerConfig
	middleware       []Middleware
	metrics          *RequestMetrics
	tracer           trace.Tracer
	logger           *logrus.Logger
	circuitBreaker   *CircuitBreaker
	rateLimiter      *rate.Limiter
	securityHeaders  map[string]string
	correlationMap   sync.Map // map[string]*RequestCorrelation
	requestCounter   int64
	mu               sync.RWMutex
}

// RequestManagerConfig contains configuration options for the RequestManager.
type RequestManagerConfig struct {
	// Timeout for requests (default: 30 seconds)
	Timeout time.Duration

	// Maximum number of idle connections
	MaxIdleConns int

	// Maximum idle connections per host
	MaxIdleConnsPerHost int

	// Keep-alive timeout
	KeepAlive time.Duration

	// TLS handshake timeout
	TLSHandshakeTimeout time.Duration

	// Response header timeout
	ResponseHeaderTimeout time.Duration

	// Expect continue timeout
	ExpectContinueTimeout time.Duration

	// Maximum response size (default: 100MB)
	MaxResponseSize int64

	// Rate limit requests per second (0 = no limit)
	RateLimit float64

	// Rate limit burst size
	RateLimitBurst int

	// Circuit breaker configuration
	CircuitBreakerConfig CircuitBreakerConfig

	// Security headers to validate
	RequiredSecurityHeaders []string

	// Allowed content types
	AllowedContentTypes []string

	// User agent string
	UserAgent string

	// Enable request/response logging
	EnableLogging bool

	// Enable metrics collection
	EnableMetrics bool

	// Enable tracing
	EnableTracing bool

	// Custom headers to add to all requests
	DefaultHeaders map[string]string
}

// Note: CircuitBreakerConfig is defined in resilience.go with more comprehensive fields

// RequestCorrelation tracks request metadata and timing.
type RequestCorrelation struct {
	ID           string
	Method       string
	URL          string
	StartTime    time.Time
	Headers      http.Header
	UserAgent    string
	RemoteAddr   string
	TraceSpan    trace.Span
	Context      context.Context
	Retries      int
	Metadata     map[string]interface{}
}

// RequestMetrics tracks request/response metrics.
type RequestMetrics struct {
	requestCounter       metric.Int64Counter
	responseCounter      metric.Int64Counter
	requestDuration      metric.Float64Histogram
	requestSize          metric.Int64Histogram
	responseSize         metric.Int64Histogram
	activeRequests       metric.Int64UpDownCounter
	circuitBreakerState  metric.Int64Gauge
	rateLimitHits        metric.Int64Counter
	securityViolations   metric.Int64Counter
}

// Middleware defines the interface for request/response middleware.
type Middleware interface {
	// ProcessRequest is called before sending the request
	ProcessRequest(ctx context.Context, req *http.Request, correlation *RequestCorrelation) error

	// ProcessResponse is called after receiving the response
	ProcessResponse(ctx context.Context, resp *http.Response, correlation *RequestCorrelation) error

	// HandleError is called when an error occurs during request processing
	HandleError(ctx context.Context, err error, correlation *RequestCorrelation) error
}

// CircuitBreaker implements a circuit breaker pattern for request reliability.
// Note: CircuitBreaker and CircuitBreakerState are defined in resilience.go with more comprehensive implementation

// NewRequestManager creates a new RequestManager with the specified configuration.
func NewRequestManager(config RequestManagerConfig) (*RequestManager, error) {
	// Set default values
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 100
	}
	if config.MaxIdleConnsPerHost == 0 {
		config.MaxIdleConnsPerHost = 10
	}
	if config.KeepAlive == 0 {
		config.KeepAlive = 30 * time.Second
	}
	if config.TLSHandshakeTimeout == 0 {
		config.TLSHandshakeTimeout = 10 * time.Second
	}
	if config.ResponseHeaderTimeout == 0 {
		config.ResponseHeaderTimeout = 10 * time.Second
	}
	if config.ExpectContinueTimeout == 0 {
		config.ExpectContinueTimeout = 1 * time.Second
	}
	if config.MaxResponseSize == 0 {
		config.MaxResponseSize = 100 * 1024 * 1024 // 100MB
	}
	if config.UserAgent == "" {
		config.UserAgent = "ag-ui-go-sdk/1.0.0"
	}
	if config.CircuitBreakerConfig.FailureThreshold == 0 {
		config.CircuitBreakerConfig.FailureThreshold = 5
	}
	if config.CircuitBreakerConfig.Timeout == 0 {
		config.CircuitBreakerConfig.Timeout = 60 * time.Second
	}
	if config.CircuitBreakerConfig.HalfOpenMaxCalls == 0 {
		config.CircuitBreakerConfig.HalfOpenMaxCalls = 10
	}

	// Create HTTP transport with custom configuration
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.Timeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Create HTTP client
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	// Create logger
	logger := logrus.New()
	if config.EnableLogging {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.WarnLevel)
	}

	// Create tracer
	var tracer trace.Tracer
	if config.EnableTracing {
		tracer = otel.Tracer("ag-ui-request-manager")
	}

	// Create circuit breaker
	circuitBreaker := &CircuitBreaker{
		config: config.CircuitBreakerConfig,
		state:  CircuitBreakerClosed,
	}

	// Create rate limiter
	var rateLimiter *rate.Limiter
	if config.RateLimit > 0 {
		rateLimiter = rate.NewLimiter(rate.Limit(config.RateLimit), config.RateLimitBurst)
	}

	// Initialize security headers
	securityHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	}

	rm := &RequestManager{
		httpClient:      httpClient,
		config:          config,
		middleware:      make([]Middleware, 0),
		logger:          logger,
		tracer:          tracer,
		circuitBreaker:  circuitBreaker,
		rateLimiter:     rateLimiter,
		securityHeaders: securityHeaders,
	}

	// Initialize metrics if enabled
	if config.EnableMetrics {
		metrics, err := rm.initializeMetrics()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
		rm.metrics = metrics
	}

	return rm, nil
}

// initializeMetrics initializes OpenTelemetry metrics for the request manager.
func (rm *RequestManager) initializeMetrics() (*RequestMetrics, error) {
	meter := otel.Meter("ag-ui-request-manager")

	requestCounter, err := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	responseCounter, err := meter.Int64Counter(
		"http_responses_total",
		metric.WithDescription("Total number of HTTP responses"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
	)
	if err != nil {
		return nil, err
	}

	requestSize, err := meter.Int64Histogram(
		"http_request_size_bytes",
		metric.WithDescription("HTTP request size in bytes"),
	)
	if err != nil {
		return nil, err
	}

	responseSize, err := meter.Int64Histogram(
		"http_response_size_bytes",
		metric.WithDescription("HTTP response size in bytes"),
	)
	if err != nil {
		return nil, err
	}

	activeRequests, err := meter.Int64UpDownCounter(
		"http_active_requests",
		metric.WithDescription("Number of active HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	circuitBreakerState, err := meter.Int64Gauge(
		"circuit_breaker_state",
		metric.WithDescription("Circuit breaker state (0=closed, 1=open, 2=half-open)"),
	)
	if err != nil {
		return nil, err
	}

	rateLimitHits, err := meter.Int64Counter(
		"rate_limit_hits_total",
		metric.WithDescription("Total number of rate limit hits"),
	)
	if err != nil {
		return nil, err
	}

	securityViolations, err := meter.Int64Counter(
		"security_violations_total",
		metric.WithDescription("Total number of security violations"),
	)
	if err != nil {
		return nil, err
	}

	return &RequestMetrics{
		requestCounter:       requestCounter,
		responseCounter:      responseCounter,
		requestDuration:      requestDuration,
		requestSize:          requestSize,
		responseSize:         responseSize,
		activeRequests:       activeRequests,
		circuitBreakerState:  circuitBreakerState,
		rateLimitHits:        rateLimitHits,
		securityViolations:   securityViolations,
	}, nil
}

// AddMiddleware adds a middleware to the request manager.
func (rm *RequestManager) AddMiddleware(middleware Middleware) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.middleware = append(rm.middleware, middleware)
}

// Do executes an HTTP request with comprehensive error handling and middleware processing.
func (rm *RequestManager) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Check circuit breaker
	if !rm.circuitBreaker.AllowRequest() {
		if rm.metrics != nil {
			rm.metrics.circuitBreakerState.Record(ctx, int64(rm.circuitBreaker.GetState()))
		}
		return nil, pkgerrors.WrapWithContext(fmt.Errorf("circuit breaker open"), "Do", "circuit breaker")
	}

	// Check rate limit
	if rm.rateLimiter != nil && !rm.rateLimiter.Allow() {
		if rm.metrics != nil {
			rm.metrics.rateLimitHits.Add(ctx, 1)
		}
		return nil, pkgerrors.NewOperationError("Do", "rate limit", ErrRateLimitExceeded)
	}

	// Create request correlation
	correlation := rm.createCorrelation(ctx, req)
	defer rm.cleanupCorrelation(correlation)

	// Start tracing span
	if rm.tracer != nil {
		var span trace.Span
		ctx, span = rm.tracer.Start(ctx, "http_request",
			trace.WithAttributes(
				attribute.String("http.method", req.Method),
				attribute.String("http.url", req.URL.String()),
				attribute.String("correlation.id", correlation.ID),
			),
		)
		correlation.TraceSpan = span
		correlation.Context = ctx
		defer span.End()
	}

	// Record active request metric
	if rm.metrics != nil {
		rm.metrics.activeRequests.Add(ctx, 1)
		defer rm.metrics.activeRequests.Add(ctx, -1)
	}

	// Validate request
	if err := rm.validateRequest(ctx, req, correlation); err != nil {
		rm.recordError(ctx, err, correlation)
		return nil, err
	}

	// Apply security headers
	rm.applySecurityHeaders(req)

	// Apply default headers
	rm.applyDefaultHeaders(req)

	// Process request middleware
	for _, middleware := range rm.middleware {
		if err := middleware.ProcessRequest(ctx, req, correlation); err != nil {
			rm.recordError(ctx, err, correlation)
			return nil, pkgerrors.WrapWithContext(err, "Do", "middleware")
		}
	}

	// Record request metrics
	if rm.metrics != nil {
		rm.metrics.requestCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("method", req.Method),
				attribute.String("host", req.URL.Host),
			),
		)

		if req.ContentLength > 0 {
			rm.metrics.requestSize.Record(ctx, req.ContentLength)
		}
	}

	// Execute request
	startTime := time.Now()
	resp, err := rm.httpClient.Do(req)
	duration := time.Since(startTime)

	// Record duration metric
	if rm.metrics != nil {
		rm.metrics.requestDuration.Record(ctx, duration.Seconds())
	}

	if err != nil {
		// Record circuit breaker failure
		rm.circuitBreaker.RecordFailure()
		rm.recordError(ctx, err, correlation)

		// Process error middleware
		for _, middleware := range rm.middleware {
			if handledErr := middleware.HandleError(ctx, err, correlation); handledErr != nil {
				rm.logger.WithError(handledErr).Warn("Middleware error handling failed")
			}
		}

		return nil, pkgerrors.WrapWithContext(err, "Do", "http request")
	}

	// Record circuit breaker success
	rm.circuitBreaker.RecordSuccess()

	// Validate response
	if err := rm.validateResponse(ctx, resp, correlation); err != nil {
		resp.Body.Close()
		rm.recordError(ctx, err, correlation)
		return nil, err
	}

	// Record response metrics
	if rm.metrics != nil {
		rm.metrics.responseCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("method", req.Method),
				attribute.String("status_code", strconv.Itoa(resp.StatusCode)),
				attribute.String("host", req.URL.Host),
			),
		)

		if resp.ContentLength > 0 {
			rm.metrics.responseSize.Record(ctx, resp.ContentLength)
		}
	}

	// Process response middleware
	for _, middleware := range rm.middleware {
		if err := middleware.ProcessResponse(ctx, resp, correlation); err != nil {
			resp.Body.Close()
			rm.recordError(ctx, err, correlation)
			return nil, pkgerrors.WrapWithContext(err, "Do", "middleware")
		}
	}

	// Log successful request
	if rm.config.EnableLogging {
		rm.logger.WithFields(logrus.Fields{
			"correlation_id": correlation.ID,
			"method":         req.Method,
			"url":            req.URL.String(),
			"status_code":    resp.StatusCode,
			"duration_ms":    duration.Milliseconds(),
		}).Info("HTTP request completed")
	}

	return resp, nil
}

// createCorrelation creates a new request correlation with unique ID and metadata.
func (rm *RequestManager) createCorrelation(ctx context.Context, req *http.Request) *RequestCorrelation {
	id := uuid.New().String()
	requestNum := atomic.AddInt64(&rm.requestCounter, 1)

	correlation := &RequestCorrelation{
		ID:         id,
		Method:     req.Method,
		URL:        req.URL.String(),
		StartTime:  time.Now(),
		Headers:    req.Header.Clone(),
		UserAgent:  req.UserAgent(),
		RemoteAddr: req.RemoteAddr,
		Context:    ctx,
		Metadata: map[string]interface{}{
			"request_number": requestNum,
		},
	}

	// Store correlation for tracking
	rm.correlationMap.Store(id, correlation)

	return correlation
}

// cleanupCorrelation removes the correlation from tracking.
func (rm *RequestManager) cleanupCorrelation(correlation *RequestCorrelation) {
	rm.correlationMap.Delete(correlation.ID)
	if correlation.TraceSpan != nil {
		correlation.TraceSpan.End()
	}
}

// validateRequest validates the HTTP request for security and correctness.
func (rm *RequestManager) validateRequest(ctx context.Context, req *http.Request, correlation *RequestCorrelation) error {
	// Validate HTTP method
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	methodValid := false
	for _, method := range validMethods {
		if req.Method == method {
			methodValid = true
			break
		}
	}
	if !methodValid {
		return pkgerrors.NewValidationErrorWithField("method", "invalid", "unsupported HTTP method", req.Method)
	}

	// Validate URL
	if req.URL == nil {
		return pkgerrors.NewValidationErrorWithField("url", "required", "URL cannot be nil", nil)
	}

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return pkgerrors.NewValidationErrorWithField("url.scheme", "invalid", "only HTTP and HTTPS schemes are supported", req.URL.Scheme)
	}

	// Validate host
	if req.URL.Host == "" {
		return pkgerrors.NewValidationErrorWithField("url.host", "required", "host cannot be empty", req.URL.Host)
	}

	// Validate content type if body is present
	if req.Body != nil && req.Header.Get("Content-Type") == "" {
		return pkgerrors.NewValidationErrorWithField("content-type", "required", "Content-Type header required for requests with body", nil)
	}

	// Validate allowed content types
	if len(rm.config.AllowedContentTypes) > 0 && req.Body != nil {
		contentType := req.Header.Get("Content-Type")
		allowed := false
		for _, allowedType := range rm.config.AllowedContentTypes {
			if strings.HasPrefix(contentType, allowedType) {
				allowed = true
				break
			}
		}
		if !allowed {
			if rm.metrics != nil {
				rm.metrics.securityViolations.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("violation_type", "content_type"),
						attribute.String("content_type", contentType),
					),
				)
			}
			return pkgerrors.NewSecurityError("validateRequest", "content type not allowed")
		}
	}

	// Validate user agent
	if req.UserAgent() == "" {
		req.Header.Set("User-Agent", rm.config.UserAgent)
	}

	return nil
}

// validateResponse validates the HTTP response for security and correctness.
func (rm *RequestManager) validateResponse(ctx context.Context, resp *http.Response, correlation *RequestCorrelation) error {
	// Check for required security headers
	for _, header := range rm.config.RequiredSecurityHeaders {
		if resp.Header.Get(header) == "" {
			if rm.metrics != nil {
				rm.metrics.securityViolations.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("violation_type", "missing_security_header"),
						attribute.String("header", header),
					),
				)
			}
			rm.logger.WithFields(logrus.Fields{
				"correlation_id": correlation.ID,
				"missing_header": header,
			}).Warn("Missing required security header")
		}
	}

	// Validate content length
	if resp.ContentLength > rm.config.MaxResponseSize {
		return pkgerrors.NewValidationErrorWithField("content-length", "too_large", 
			fmt.Sprintf("response size %d exceeds maximum %d", resp.ContentLength, rm.config.MaxResponseSize), 
			resp.ContentLength)
	}

	// Wrap response body to limit size
	if resp.Body != nil {
		resp.Body = &limitedResponseBody{
			ReadCloser: resp.Body,
			maxSize:    rm.config.MaxResponseSize,
			bytesRead:  0,
		}
	}

	return nil
}

// applySecurityHeaders applies security headers to the request.
func (rm *RequestManager) applySecurityHeaders(req *http.Request) {
	for header, value := range rm.securityHeaders {
		if req.Header.Get(header) == "" {
			req.Header.Set(header, value)
		}
	}
}

// applyDefaultHeaders applies default headers to the request.
func (rm *RequestManager) applyDefaultHeaders(req *http.Request) {
	for header, value := range rm.config.DefaultHeaders {
		if req.Header.Get(header) == "" {
			req.Header.Set(header, value)
		}
	}

	// Always set User-Agent if not present
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", rm.config.UserAgent)
	}
}

// recordError records error metrics and logs.
func (rm *RequestManager) recordError(ctx context.Context, err error, correlation *RequestCorrelation) {
	if rm.config.EnableLogging {
		rm.logger.WithError(err).WithFields(logrus.Fields{
			"correlation_id": correlation.ID,
			"method":         correlation.Method,
			"url":            correlation.URL,
			"duration_ms":    time.Since(correlation.StartTime).Milliseconds(),
		}).Error("HTTP request failed")
	}

	if correlation.TraceSpan != nil {
		correlation.TraceSpan.RecordError(err)
		correlation.TraceSpan.SetAttributes(attribute.Bool("error", true))
	}
}

// GetActiveCorrelations returns all active request correlations.
func (rm *RequestManager) GetActiveCorrelations() map[string]*RequestCorrelation {
	correlations := make(map[string]*RequestCorrelation)
	rm.correlationMap.Range(func(key, value interface{}) bool {
		if id, ok := key.(string); ok {
			if correlation, ok := value.(*RequestCorrelation); ok {
				correlations[id] = correlation
			}
		}
		return true
	})
	return correlations
}

// GetCircuitBreakerState returns the current circuit breaker state.
func (rm *RequestManager) GetCircuitBreakerState() CircuitBreakerState {
	return rm.circuitBreaker.GetState()
}

// Close gracefully shuts down the request manager.
func (rm *RequestManager) Close() error {
	rm.httpClient.CloseIdleConnections()
	return nil
}

// Note: CircuitBreaker methods are implemented in resilience.go

// limitedResponseBody wraps an io.ReadCloser to limit the number of bytes read.
type limitedResponseBody struct {
	io.ReadCloser
	maxSize   int64
	bytesRead int64
}

// Read implements io.Reader with size limiting.
func (r *limitedResponseBody) Read(p []byte) (n int, err error) {
	if r.bytesRead >= r.maxSize {
		return 0, fmt.Errorf("response size limit exceeded: %d bytes", r.maxSize)
	}

	remaining := r.maxSize - r.bytesRead
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}

	n, err = r.ReadCloser.Read(p)
	r.bytesRead += int64(n)

	return n, err
}

// GetCorrelationMapStats returns statistics about the correlation map
func (rm *RequestManager) GetCorrelationMapStats() map[string]interface{} {
	stats := map[string]interface{}{
		"type": "sync.Map",
	}
	
	// Count entries by iterating over the map
	count := 0
	rm.correlationMap.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	
	stats["count"] = count
	return stats
}