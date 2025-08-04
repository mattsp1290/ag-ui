package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"golang.org/x/net/http2"
)

// HTTPTransportConfig holds the configuration for HTTP transport
type HTTPTransportConfig struct {
	// Core connection settings
	BaseURL        string            `json:"base_url" yaml:"base_url"`
	Headers        map[string]string `json:"headers" yaml:"headers"`
	AuthToken      string            `json:"auth_token" yaml:"auth_token"`
	UserAgent      string            `json:"user_agent" yaml:"user_agent"`
	
	// Protocol settings
	ForceHTTP1     bool `json:"force_http1" yaml:"force_http1"`
	EnableHTTP2    bool `json:"enable_http2" yaml:"enable_http2"`
	
	// Connection pooling
	MaxIdleConns        int           `json:"max_idle_conns" yaml:"max_idle_conns"`
	MaxIdleConnsPerHost int           `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`
	MaxConnsPerHost     int           `json:"max_conns_per_host" yaml:"max_conns_per_host"`
	IdleConnTimeout     time.Duration `json:"idle_conn_timeout" yaml:"idle_conn_timeout"`
	
	// Timeouts
	ConnectTimeout      time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
	RequestTimeout      time.Duration `json:"request_timeout" yaml:"request_timeout"`
	ResponseTimeout     time.Duration `json:"response_timeout" yaml:"response_timeout"`
	KeepAliveTimeout    time.Duration `json:"keep_alive_timeout" yaml:"keep_alive_timeout"`
	TLSHandshakeTimeout time.Duration `json:"tls_handshake_timeout" yaml:"tls_handshake_timeout"`
	
	// Retry and resilience
	MaxRetries         int           `json:"max_retries" yaml:"max_retries"`
	BaseRetryDelay     time.Duration `json:"base_retry_delay" yaml:"base_retry_delay"`
	MaxRetryDelay      time.Duration `json:"max_retry_delay" yaml:"max_retry_delay"`
	RetryJitter        bool          `json:"retry_jitter" yaml:"retry_jitter"`
	CircuitBreakerConfig *CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
	
	// Security
	TLSConfig           *tls.Config `json:"-" yaml:"-"`
	InsecureSkipVerify  bool        `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	SecurityHeaders     map[string]string `json:"security_headers" yaml:"security_headers"`
	
	// Performance
	EnableCompression bool `json:"enable_compression" yaml:"enable_compression"`
	CompressionLevel  int  `json:"compression_level" yaml:"compression_level"`
	BufferSize        int  `json:"buffer_size" yaml:"buffer_size"`
	
	// Monitoring
	EnableMetrics bool `json:"enable_metrics" yaml:"enable_metrics"`
	MetricsPrefix string `json:"metrics_prefix" yaml:"metrics_prefix"`
}

// Note: CircuitBreakerConfig is defined in resilience.go with more comprehensive fields

// HTTPTransport implements a high-performance HTTP transport layer
type HTTPTransport struct {
	config    *HTTPTransportConfig
	client    *http.Client
	transport *http.Transport
	
	// Connection management
	connectionPool    *ConnectionPool
	keepAliveManager  *KeepAliveManager
	
	// Resilience
	circuitBreaker    *CircuitBreaker
	retryPolicy       *RetryPolicy
	
	// Performance monitoring
	metrics           *HTTPMetrics
	performanceStats  *PerformanceStats
	
	// Lifecycle
	mu                sync.RWMutex
	started           bool
	shutdown          chan struct{}
	wg                sync.WaitGroup
	
	// Context management
	ctx               context.Context
	cancel            context.CancelFunc
}

// ConnectionPool manages HTTP connection pooling
type ConnectionPool struct {
	maxIdle     int
	maxPerHost  int
	maxTotal    int
	idleTimeout time.Duration
	
	mu          sync.RWMutex
	active      map[string]*connectionEntry
	idle        map[string][]*connectionEntry
	stats       ConnectionPoolStats
	
	// Lifecycle management
	shutdown    chan struct{}
	done        chan struct{}
}

type connectionEntry struct {
	conn       net.Conn
	lastUsed   time.Time
	requests   int64
	created    time.Time
}

type ConnectionPoolStats struct {
	TotalConnections  int64         `json:"total_connections"`
	ActiveConnections int64         `json:"active_connections"`
	IdleConnections   int64         `json:"idle_connections"`
	ConnectionsReused int64         `json:"connections_reused"`
	ConnectionsCreated int64        `json:"connections_created"`
	AverageLifetime   time.Duration `json:"average_lifetime"`
}

// NewConnectionPool creates a new connection pool with the specified configuration
func NewConnectionPool(maxIdle int, idleTimeout time.Duration) *ConnectionPool {
	return &ConnectionPool{
		maxIdle:     maxIdle,
		maxPerHost:  10, // Default value
		maxTotal:    50, // Default value
		idleTimeout: idleTimeout,
		active:      make(map[string]*connectionEntry),
		idle:        make(map[string][]*connectionEntry),
		shutdown:    make(chan struct{}),
		done:        make(chan struct{}),
	}
}

// Size returns the total number of connections in the pool
func (cp *ConnectionPool) Size() int64 {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	
	return cp.stats.TotalConnections
}

// maintainConnections starts the connection maintenance goroutine
func (cp *ConnectionPool) maintainConnections(ctx context.Context) {
	defer close(cp.done)
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-cp.shutdown:
			return
		case <-ticker.C:
			cp.cleanup()
		}
	}
}

// Close gracefully shuts down the connection pool
func (cp *ConnectionPool) Close() {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	// Signal shutdown
	select {
	case <-cp.shutdown:
		// Already closed
		return
	default:
		close(cp.shutdown)
	}
	
	// Close all active connections
	for _, conn := range cp.active {
		if conn.conn != nil {
			conn.conn.Close()
		}
	}
	
	// Close all idle connections
	for _, connections := range cp.idle {
		for _, conn := range connections {
			if conn.conn != nil {
				conn.conn.Close()
			}
		}
	}
	
	// Clear maps
	cp.active = make(map[string]*connectionEntry)
	cp.idle = make(map[string][]*connectionEntry)
	
	// Wait for maintenance goroutine to finish
	<-cp.done
}

// KeepAliveManager handles connection keep-alive
type KeepAliveManager struct {
	enabled     bool
	interval    time.Duration
	timeout     time.Duration
	maxRequests int
	
	mu          sync.RWMutex
	connections map[string]*keepAliveConn
	ticker      *time.Ticker
	shutdown    chan struct{}
}

type keepAliveConn struct {
	conn         net.Conn
	lastPing     time.Time
	requestCount int
	healthy      bool
}

// Note: CircuitBreaker and CircuitBreakerState are defined in resilience.go with more comprehensive implementation

// RetryPolicy implements smart retry logic
type RetryPolicy struct {
	maxRetries    int
	baseDelay     time.Duration
	maxDelay      time.Duration
	enableJitter  bool
	backoffFactor float64
}

// HTTPMetrics collects detailed HTTP metrics
type HTTPMetrics struct {
	// Request metrics
	TotalRequests        int64         `json:"total_requests"`
	SuccessfulRequests   int64         `json:"successful_requests"`
	FailedRequests       int64         `json:"failed_requests"`
	
	// Timing metrics
	TotalDuration        time.Duration `json:"total_duration"`
	AverageDuration      time.Duration `json:"average_duration"`
	MinDuration          time.Duration `json:"min_duration"`
	MaxDuration          time.Duration `json:"max_duration"`
	
	// Data transfer
	BytesSent            int64         `json:"bytes_sent"`
	BytesReceived        int64         `json:"bytes_received"`
	
	// Protocol statistics
	HTTP1Requests        int64         `json:"http1_requests"`
	HTTP2Requests        int64         `json:"http2_requests"`
	
	// Error breakdown
	ConnectionErrors     int64         `json:"connection_errors"`
	TimeoutErrors        int64         `json:"timeout_errors"`
	ProtocolErrors       int64         `json:"protocol_errors"`
	
	// Compression stats
	CompressionRatio     float64       `json:"compression_ratio"`
	CompressedRequests   int64         `json:"compressed_requests"`
	
	mu                   sync.RWMutex
	lastReset           time.Time
}

// PerformanceStats tracks performance characteristics
type PerformanceStats struct {
	// Connection performance
	ConnectionSetupTime  time.Duration `json:"connection_setup_time"`
	TLSHandshakeTime     time.Duration `json:"tls_handshake_time"`
	DNSLookupTime        time.Duration `json:"dns_lookup_time"`
	
	// Request performance
	TimeToFirstByte      time.Duration `json:"time_to_first_byte"`
	ContentDownloadTime  time.Duration `json:"content_download_time"`
	
	// Throughput
	RequestsPerSecond    float64       `json:"requests_per_second"`
	BytesPerSecond       float64       `json:"bytes_per_second"`
	
	// Resource utilization
	GoroutineCount       int           `json:"goroutine_count"`
	MemoryUsage          int64         `json:"memory_usage"`
	
	mu                   sync.RWMutex
	windowStart          time.Time
	windowRequests       int64
	windowBytes          int64
}

// DefaultHTTPTransportConfig returns a sensible default configuration
func DefaultHTTPTransportConfig() *HTTPTransportConfig {
	return &HTTPTransportConfig{
		BaseURL:             "",
		Headers:             make(map[string]string),
		UserAgent:           "ag-ui-http-transport/1.0",
		
		EnableHTTP2:         true,
		ForceHTTP1:          false,
		
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     90 * time.Second,
		
		ConnectTimeout:      10 * time.Second,
		RequestTimeout:      30 * time.Second,
		ResponseTimeout:     30 * time.Second,
		KeepAliveTimeout:    30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		
		MaxRetries:          3,
		BaseRetryDelay:      100 * time.Millisecond,
		MaxRetryDelay:       30 * time.Second,
		RetryJitter:         true,
		
		CircuitBreakerConfig: &CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          30 * time.Second,
			HalfOpenMaxCalls: 3,
			FailureRateThreshold: 0.5,
			MinimumRequestThreshold: 10,
		},
		
		InsecureSkipVerify: false,
		SecurityHeaders: map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"X-XSS-Protection":       "1; mode=block",
		},
		
		EnableCompression: true,
		CompressionLevel:  6,
		BufferSize:        32 * 1024,
		
		EnableMetrics:     true,
		MetricsPrefix:     "http_transport",
	}
}

// NewHTTPTransport creates a new HTTP transport instance
func NewHTTPTransport(config *HTTPTransportConfig) (*HTTPTransport, error) {
	if config == nil {
		config = DefaultHTTPTransportConfig()
	}
	
	if err := validateConfig(config); err != nil {
		return nil, errors.NewAgentError(
			errors.ErrorTypeValidation,
			"invalid configuration",
			"HTTPTransport",
		).WithCause(err).WithDetail("operation", "NewHTTPTransport")
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	transport := &HTTPTransport{
		config:   config,
		shutdown: make(chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
	}
	
	if err := transport.initialize(); err != nil {
		cancel()
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to initialize transport",
			"HTTPTransport",
		).WithCause(err).WithDetail("operation", "NewHTTPTransport")
	}
	
	return transport, nil
}

// initialize sets up all transport components
func (h *HTTPTransport) initialize() error {
	// Initialize metrics
	h.metrics = &HTTPMetrics{
		lastReset:   time.Now(),
		MinDuration: time.Hour, // Will be updated with actual values
	}
	
	h.performanceStats = &PerformanceStats{
		windowStart: time.Now(),
	}
	
	// Initialize connection pool
	h.connectionPool = &ConnectionPool{
		maxIdle:     h.config.MaxIdleConns,
		maxPerHost:  h.config.MaxIdleConnsPerHost,
		maxTotal:    h.config.MaxConnsPerHost,
		idleTimeout: h.config.IdleConnTimeout,
		active:      make(map[string]*connectionEntry),
		idle:        make(map[string][]*connectionEntry),
	}
	
	// Initialize keep-alive manager
	h.keepAliveManager = &KeepAliveManager{
		enabled:     true,
		interval:    h.config.KeepAliveTimeout / 3,
		timeout:     h.config.KeepAliveTimeout,
		maxRequests: 1000,
		connections: make(map[string]*keepAliveConn),
		shutdown:    make(chan struct{}),
	}
	
	// Initialize circuit breaker
	if h.config.CircuitBreakerConfig.Enabled {
		h.circuitBreaker = NewCircuitBreaker(*h.config.CircuitBreakerConfig)
	}
	
	// Initialize retry policy
	h.retryPolicy = &RetryPolicy{
		maxRetries:    h.config.MaxRetries,
		baseDelay:     h.config.BaseRetryDelay,
		maxDelay:      h.config.MaxRetryDelay,
		enableJitter:  h.config.RetryJitter,
		backoffFactor: 2.0,
	}
	
	// Create HTTP transport
	if err := h.createHTTPTransport(); err != nil {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to create HTTP transport",
			"HTTPTransport",
		).WithCause(err).WithDetail("operation", "initialize")
	}
	
	// Create HTTP client
	h.client = &http.Client{
		Transport: h.transport,
		Timeout:   h.config.RequestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	
	return nil
}

// createHTTPTransport sets up the underlying HTTP transport with optimizations
func (h *HTTPTransport) createHTTPTransport() error {
	// Create base transport
	transport := &http.Transport{
		// Connection pooling
		MaxIdleConns:        h.config.MaxIdleConns,
		MaxIdleConnsPerHost: h.config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     h.config.MaxConnsPerHost,
		IdleConnTimeout:     h.config.IdleConnTimeout,
		
		// Timeouts
		DialContext: (&net.Dialer{
			Timeout:   h.config.ConnectTimeout,
			KeepAlive: h.config.KeepAliveTimeout,
		}).DialContext,
		TLSHandshakeTimeout:   h.config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: h.config.ResponseTimeout,
		
		// Compression
		DisableCompression: !h.config.EnableCompression,
		
		// Keep-alive
		DisableKeepAlives: false,
		
		// TLS configuration
		TLSClientConfig: h.createTLSConfig(),
		
		// Protocol selection
		ForceAttemptHTTP2: h.config.EnableHTTP2 && !h.config.ForceHTTP1,
	}
	
	// Configure HTTP/2 if enabled
	if h.config.EnableHTTP2 && !h.config.ForceHTTP1 {
		if err := http2.ConfigureTransport(transport); err != nil {
			return errors.NewAgentError(
				errors.ErrorTypeInvalidState,
				"failed to configure HTTP/2",
				"HTTPTransport",
			).WithCause(err).WithDetail("operation", "createHTTPTransport")
		}
	}
	
	h.transport = transport
	return nil
}

// createTLSConfig creates optimized TLS configuration
func (h *HTTPTransport) createTLSConfig() *tls.Config {
	if h.config.TLSConfig != nil {
		return h.config.TLSConfig.Clone()
	}
	
	tlsConfig := &tls.Config{
		InsecureSkipVerify: h.config.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}
	
	return tlsConfig
}

// Start initializes and starts the HTTP transport
func (h *HTTPTransport) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.started {
		return nil
	}
	
	// Start keep-alive manager
	h.wg.Add(1)
	go h.keepAliveManager.start(&h.wg)
	
	// Start metrics collection
	if h.config.EnableMetrics {
		h.wg.Add(1)
		go h.startMetricsCollection(&h.wg)
	}
	
	// Start connection pool cleanup
	h.wg.Add(1)
	go h.startConnectionPoolCleanup(&h.wg)
	
	h.started = true
	return nil
}

// Stop gracefully shuts down the HTTP transport
func (h *HTTPTransport) Stop(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.started {
		return nil
	}
	
	// Signal shutdown
	close(h.shutdown)
	h.cancel()
	
	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Clean shutdown
	case <-ctx.Done():
		// Timeout - force close
		return ctx.Err()
	}
	
	// Close idle connections
	h.transport.CloseIdleConnections()
	
	h.started = false
	return nil
}

// SendRequest sends an HTTP request with full feature support
func (h *HTTPTransport) SendRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	if !h.started {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"transport not started",
			"HTTPTransport",
		).WithDetail("operation", "SendRequest")
	}
	
	// Check circuit breaker
	if h.circuitBreaker != nil && !h.circuitBreaker.Allow() {
		return nil, errors.NewAgentError(
			errors.ErrorTypeRateLimit,
			"circuit breaker open",
			"HTTPTransport",
		).WithDetail("operation", "SendRequest")
	}
	
	// Build URL
	targetURL, err := h.buildURL(path)
	if err != nil {
		return nil, errors.NewValidationError("invalid_url", "invalid URL").
			WithField("path", path).WithCause(err).WithDetail("operation", "SendRequest")
	}
	
	// Create request with retry logic
	var response *http.Response
	var lastErr error
	
	for attempt := 0; attempt <= h.config.MaxRetries; attempt++ {
		// Apply backoff delay for retries
		if attempt > 0 {
			delay := h.retryPolicy.calculateDelay(attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		
		// Create and send request
		response, lastErr = h.sendSingleRequest(ctx, method, targetURL, body)
		
		// Check if request was successful
		if lastErr == nil && response.StatusCode < 500 {
			// Success or client error (don't retry client errors)
			if h.circuitBreaker != nil {
				h.circuitBreaker.RecordSuccess()
			}
			h.recordMetrics(true, time.Since(time.Now()), int64(len(body)))
			return response, nil
		}
		
		// Record failure
		if h.circuitBreaker != nil {
			h.circuitBreaker.RecordFailure()
		}
		
		// Check if we should retry
		if !h.shouldRetry(attempt, lastErr, response) {
			break
		}
		
		// Close response body if exists
		if response != nil && response.Body != nil {
			response.Body.Close()
		}
	}
	
	h.recordMetrics(false, time.Since(time.Now()), int64(len(body)))
	return response, lastErr
}

// sendSingleRequest performs a single HTTP request
func (h *HTTPTransport) sendSingleRequest(ctx context.Context, method, url string, body []byte) (*http.Response, error) {
	startTime := time.Now()
	
	// Prepare request body
	var bodyReader io.Reader
	var contentLength int64
	
	if body != nil {
		if h.config.EnableCompression && len(body) > 1024 {
			// Compress large bodies
			compressed, err := h.compressBody(body)
			if err == nil && len(compressed) < len(body) {
				bodyReader = bytes.NewReader(compressed)
				contentLength = int64(len(compressed))
			} else {
				bodyReader = bytes.NewReader(body)
				contentLength = int64(len(body))
			}
		} else {
			bodyReader = bytes.NewReader(body)
			contentLength = int64(len(body))
		}
	}
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, errors.NewAgentError(
			errors.ErrorTypeValidation,
			"failed to create request",
			"HTTPTransport",
		).WithCause(err).WithDetail("operation", "sendSingleRequest").WithDetail("method", method)
	}
	
	// Set headers
	h.setRequestHeaders(req, contentLength)
	
	// Send request
	response, err := h.client.Do(req)
	if err != nil {
		h.recordError(err)
		return nil, errors.NewAgentError(
			errors.ErrorTypeExternal,
			"request failed",
			"HTTPTransport",
		).WithCause(err).WithDetail("operation", "sendSingleRequest").WithDetail("method", method)
	}
	
	// Record performance metrics
	duration := time.Since(startTime)
	h.recordPerformanceMetrics(duration, response)
	
	return response, nil
}

// setRequestHeaders sets all necessary headers for the request
func (h *HTTPTransport) setRequestHeaders(req *http.Request, contentLength int64) {
	// Standard headers
	req.Header.Set("User-Agent", h.config.UserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	
	if contentLength > 0 {
		req.Header.Set("Content-Length", fmt.Sprintf("%d", contentLength))
	}
	
	// Compression headers
	if h.config.EnableCompression {
		req.Header.Set("Accept-Encoding", "gzip, deflate")
		if contentLength > 1024 {
			req.Header.Set("Content-Encoding", "gzip")
		}
	}
	
	// Security headers
	for key, value := range h.config.SecurityHeaders {
		req.Header.Set(key, value)
	}
	
	// Custom headers
	for key, value := range h.config.Headers {
		req.Header.Set(key, value)
	}
	
	// Authentication
	if h.config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.config.AuthToken)
	}
	
	// Keep-alive headers
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Keep-Alive", fmt.Sprintf("timeout=%d", int(h.config.KeepAliveTimeout.Seconds())))
}

// compressBody compresses request body using gzip
func (h *HTTPTransport) compressBody(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, h.config.CompressionLevel)
	if err != nil {
		return nil, err
	}
	
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, err
	}
	
	if err := writer.Close(); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}

// buildURL constructs the full URL from base URL and path
func (h *HTTPTransport) buildURL(path string) (string, error) {
	baseURL, err := url.Parse(h.config.BaseURL)
	if err != nil {
		return "", err
	}
	
	pathURL, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	
	return baseURL.ResolveReference(pathURL).String(), nil
}

// shouldRetry determines if a request should be retried
func (h *HTTPTransport) shouldRetry(attempt int, err error, response *http.Response) bool {
	if attempt >= h.config.MaxRetries {
		return false
	}
	
	// Retry on network errors
	if err != nil {
		return true
	}
	
	// Retry on server errors (5xx)
	if response != nil && response.StatusCode >= 500 {
		return true
	}
	
	// Retry on specific 4xx errors
	if response != nil {
		switch response.StatusCode {
		case http.StatusTooManyRequests, http.StatusRequestTimeout:
			return true
		}
	}
	
	return false
}

// Circuit breaker implementation
func (cb *CircuitBreaker) Allow() bool {
	if cb == nil {
		return true
	}
	
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	state := cb.state
	
	switch state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if recovery timeout has passed
		if time.Since(cb.lastFailTime) > cb.config.Timeout {
			cb.state = CircuitBreakerHalfOpen
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return cb.requests < int64(cb.config.SuccessThreshold)
	}
	
	return false
}

// Note: CircuitBreaker methods RecordSuccess and RecordFailure are implemented in resilience.go

// Retry policy implementation
func (rp *RetryPolicy) calculateDelay(attempt int) time.Duration {
	delay := rp.baseDelay
	
	// Exponential backoff
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * rp.backoffFactor)
		if delay > rp.maxDelay {
			delay = rp.maxDelay
			break
		}
	}
	
	// Add jitter
	if rp.enableJitter {
		jitter := time.Duration(rand.Float64() * float64(delay) * 0.1)
		delay += jitter
	}
	
	return delay
}

// Keep-alive manager implementation
func (kam *KeepAliveManager) start(wg *sync.WaitGroup) {
	defer wg.Done()
	
	if !kam.enabled {
		return
	}
	
	kam.ticker = time.NewTicker(kam.interval)
	defer kam.ticker.Stop()
	
	for {
		select {
		case <-kam.ticker.C:
			kam.performHealthChecks()
		case <-kam.shutdown:
			return
		}
	}
}

func (kam *KeepAliveManager) performHealthChecks() {
	kam.mu.RLock()
	connections := make([]*keepAliveConn, 0, len(kam.connections))
	for _, conn := range kam.connections {
		connections = append(connections, conn)
	}
	kam.mu.RUnlock()
	
	for _, conn := range connections {
		if time.Since(conn.lastPing) > kam.timeout {
			kam.closeConnection(conn)
		}
	}
}

func (kam *KeepAliveManager) closeConnection(conn *keepAliveConn) {
	if conn.conn != nil {
		conn.conn.Close()
	}
	conn.healthy = false
}

// Metrics collection
func (h *HTTPTransport) startMetricsCollection(wg *sync.WaitGroup) {
	defer wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			h.updatePerformanceStats()
		case <-h.shutdown:
			return
		}
	}
}

func (h *HTTPTransport) recordMetrics(success bool, duration time.Duration, bytes int64) {
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()
	
	h.metrics.TotalRequests++
	h.metrics.TotalDuration += duration
	h.metrics.AverageDuration = h.metrics.TotalDuration / time.Duration(h.metrics.TotalRequests)
	
	if duration < h.metrics.MinDuration {
		h.metrics.MinDuration = duration
	}
	if duration > h.metrics.MaxDuration {
		h.metrics.MaxDuration = duration
	}
	
	if success {
		h.metrics.SuccessfulRequests++
		h.metrics.BytesSent += bytes
	} else {
		h.metrics.FailedRequests++
	}
}

func (h *HTTPTransport) recordError(err error) {
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()
	
	if err == nil {
		return
	}
	
	// Classify error types
	errorStr := err.Error()
	switch {
	case strings.Contains(errorStr, "timeout"):
		h.metrics.TimeoutErrors++
	case strings.Contains(errorStr, "connection"):
		h.metrics.ConnectionErrors++
	default:
		h.metrics.ProtocolErrors++
	}
}

func (h *HTTPTransport) recordPerformanceMetrics(duration time.Duration, response *http.Response) {
	h.performanceStats.mu.Lock()
	defer h.performanceStats.mu.Unlock()
	
	// Update window stats
	h.performanceStats.windowRequests++
	
	// Check protocol version
	if response != nil {
		if response.ProtoMajor == 2 {
			h.metrics.HTTP2Requests++
		} else {
			h.metrics.HTTP1Requests++
		}
	}
	
	// Update resource utilization
	h.performanceStats.GoroutineCount = runtime.NumGoroutine()
	
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	h.performanceStats.MemoryUsage = int64(memStats.Alloc)
}

func (h *HTTPTransport) updatePerformanceStats() {
	h.performanceStats.mu.Lock()
	defer h.performanceStats.mu.Unlock()
	
	now := time.Now()
	elapsed := now.Sub(h.performanceStats.windowStart)
	
	if elapsed > 0 {
		h.performanceStats.RequestsPerSecond = float64(h.performanceStats.windowRequests) / elapsed.Seconds()
		h.performanceStats.BytesPerSecond = float64(h.performanceStats.windowBytes) / elapsed.Seconds()
	}
	
	// Reset window
	h.performanceStats.windowStart = now
	h.performanceStats.windowRequests = 0
	h.performanceStats.windowBytes = 0
}

// Connection pool cleanup
func (h *HTTPTransport) startConnectionPoolCleanup(wg *sync.WaitGroup) {
	defer wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			h.connectionPool.cleanup()
		case <-h.shutdown:
			return
		}
	}
}

func (cp *ConnectionPool) cleanup() {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	now := time.Now()
	
	// Clean up idle connections
	for host, connections := range cp.idle {
		var alive []*connectionEntry
		for _, conn := range connections {
			if now.Sub(conn.lastUsed) < cp.idleTimeout {
				alive = append(alive, conn)
			} else {
				if conn.conn != nil {
					conn.conn.Close()
				}
			}
		}
		
		if len(alive) > 0 {
			cp.idle[host] = alive
		} else {
			delete(cp.idle, host)
		}
	}
}

// Public API methods
func (h *HTTPTransport) GetMetrics() *HTTPMetrics {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	metrics := *h.metrics
	return &metrics
}

func (h *HTTPTransport) GetPerformanceStats() *PerformanceStats {
	h.performanceStats.mu.RLock()
	defer h.performanceStats.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	stats := *h.performanceStats
	return &stats
}

func (h *HTTPTransport) GetConnectionPoolStats() ConnectionPoolStats {
	h.connectionPool.mu.RLock()
	defer h.connectionPool.mu.RUnlock()
	
	return h.connectionPool.stats
}

func (h *HTTPTransport) IsHealthy() bool {
	if !h.started {
		return false
	}
	
	// Check circuit breaker state
	if h.circuitBreaker != nil {
		state := h.circuitBreaker.GetState()
		if state == CircuitBreakerOpen {
			return false
		}
	}
	
	return true
}

func (h *HTTPTransport) GetProtocolVersion() string {
	if h.config.ForceHTTP1 {
		return "HTTP/1.1"
	}
	if h.config.EnableHTTP2 {
		return "HTTP/2"
	}
	return "HTTP/1.1"
}

// Configuration validation
func validateConfig(config *HTTPTransportConfig) error {
	if config.BaseURL == "" {
		return errors.NewValidationError("base_url_required", "base URL is required").
			WithField("BaseURL", config.BaseURL)
	}
	
	if _, err := url.Parse(config.BaseURL); err != nil {
		return errors.NewValidationError("invalid_base_url", "invalid base URL").
			WithField("BaseURL", config.BaseURL).WithCause(err)
	}
	
	if config.MaxRetries < 0 {
		return errors.NewValidationError("max_retries_invalid", "max retries cannot be negative").
			WithField("MaxRetries", config.MaxRetries)
	}
	
	if config.BaseRetryDelay <= 0 {
		return errors.NewValidationError("base_retry_delay_invalid", "base retry delay must be positive").
			WithField("BaseRetryDelay", config.BaseRetryDelay.String())
	}
	
	if config.MaxRetryDelay < config.BaseRetryDelay {
		return errors.NewValidationError("max_retry_delay_invalid", "max retry delay must be >= base retry delay").
			WithField("MaxRetryDelay", config.MaxRetryDelay.String()).
			WithField("BaseRetryDelay", config.BaseRetryDelay.String())
	}
	
	if config.ConnectTimeout <= 0 {
		return errors.NewValidationError("connect_timeout_invalid", "connect timeout must be positive").
			WithField("ConnectTimeout", config.ConnectTimeout.String())
	}
	
	if config.RequestTimeout <= 0 {
		return errors.NewValidationError("request_timeout_invalid", "request timeout must be positive").
			WithField("RequestTimeout", config.RequestTimeout.String())
	}
	
	return nil
}