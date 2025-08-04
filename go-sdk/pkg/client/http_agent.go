package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"golang.org/x/net/http2"
)

// Common error conditions for HttpAgent
const (
	// HTTP configuration errors
	ErrHTTPConfigNil               = "config_nil"
	ErrMaxIdleConnsInvalid         = "max_idle_conns_invalid"
	ErrMaxIdleConnsPerHostInvalid  = "max_idle_conns_per_host_invalid"
	ErrMaxConnsPerHostInvalid      = "max_conns_per_host_invalid"
	ErrDialTimeoutInvalid          = "dial_timeout_invalid"
	ErrRequestTimeoutInvalid       = "request_timeout_invalid"
	ErrMaxResponseBodySizeInvalid  = "max_response_body_size_invalid"
)

// HttpAgent provides HTTP-specific functionality by embedding BaseAgent.
// It implements the Agent interface through BaseAgent and adds HTTP client
// management, connection pooling, and protocol support for HTTP/1.1 and HTTP/2.
type HttpAgent struct {
	*BaseAgent
	
	// HTTP-specific configuration
	httpConfig *HttpConfig
	
	// HTTP client management
	client     *http.Client
	transport  *http.Transport
	h2Transport *http2.Transport
	
	// Connection management
	connPool     *ConnectionPool
	connPoolMu   sync.RWMutex
	activeConns  int64
	maxConns     int64
	
	// Protocol configuration
	protocolVersion HttpProtocolVersion
	tlsConfig      *tls.Config
	
	// Request/response tracking
	requestCount   int64
	responseCount  int64
	errorCount     int64
	
	// Lifecycle management
	httpMu        sync.RWMutex
	shutdownCtx   context.Context
	shutdownCancel context.CancelFunc
}

// HttpConfig contains HTTP-specific configuration options.
type HttpConfig struct {
	// Protocol settings
	ProtocolVersion    HttpProtocolVersion `json:"protocol_version" yaml:"protocol_version"`
	EnableHTTP2        bool                `json:"enable_http2" yaml:"enable_http2"`
	ForceHTTP2         bool                `json:"force_http2" yaml:"force_http2"`
	
	// Connection settings
	MaxIdleConns        int           `json:"max_idle_conns" yaml:"max_idle_conns"`
	MaxIdleConnsPerHost int           `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`
	MaxConnsPerHost     int           `json:"max_conns_per_host" yaml:"max_conns_per_host"`
	IdleConnTimeout     time.Duration `json:"idle_conn_timeout" yaml:"idle_conn_timeout"`
	KeepAlive           time.Duration `json:"keep_alive" yaml:"keep_alive"`
	
	// Timeout settings
	DialTimeout         time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	RequestTimeout      time.Duration `json:"request_timeout" yaml:"request_timeout"`
	ResponseTimeout     time.Duration `json:"response_timeout" yaml:"response_timeout"`
	TLSHandshakeTimeout time.Duration `json:"tls_handshake_timeout" yaml:"tls_handshake_timeout"`
	
	// TLS settings
	TLSConfig          *tls.Config `json:"-" yaml:"-"` // Not serializable
	InsecureSkipVerify bool        `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	
	// Advanced settings
	DisableCompression   bool   `json:"disable_compression" yaml:"disable_compression"`
	DisableKeepAlives   bool   `json:"disable_keep_alives" yaml:"disable_keep_alives"`
	UserAgent           string `json:"user_agent" yaml:"user_agent"`
	MaxResponseBodySize int64  `json:"max_response_body_size" yaml:"max_response_body_size"`
	
	// Circuit breaker settings
	EnableCircuitBreaker bool          `json:"enable_circuit_breaker" yaml:"enable_circuit_breaker"`
	CircuitBreakerConfig *CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
}

// HttpProtocolVersion represents the HTTP protocol version to use.
type HttpProtocolVersion string

const (
	HttpProtocolVersionAuto HttpProtocolVersion = "auto"
	HttpProtocolVersion1_1  HttpProtocolVersion = "1.1"
	HttpProtocolVersion2    HttpProtocolVersion = "2"
)

// Note: CircuitBreakerConfig is defined in resilience.go with more comprehensive fields

// Note: ConnectionPool and ConnectionEntry are defined in http_transport.go with more comprehensive fields

// HttpMetrics contains HTTP-specific metrics.
type HttpMetrics struct {
	RequestCount       int64         `json:"request_count"`
	ResponseCount      int64         `json:"response_count"`
	ErrorCount         int64         `json:"error_count"`
	ActiveConnections  int64         `json:"active_connections"`
	PooledConnections  int64         `json:"pooled_connections"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	TotalBytesReceived int64         `json:"total_bytes_received"`
	TotalBytesSent     int64         `json:"total_bytes_sent"`
}

// NewHttpAgent creates a new HTTP agent with the specified configuration.
func NewHttpAgent(name, description string, httpConfig *HttpConfig) (*HttpAgent, error) {
	if httpConfig == nil {
		httpConfig = DefaultHttpConfig()
	}
	
	// Validate HTTP configuration
	if err := validateHttpConfig(httpConfig); err != nil {
		return nil, errors.NewAgentError(
			errors.ErrorTypeValidation,
			"invalid HTTP configuration",
			"HttpAgent",
		).WithCause(err).WithDetail("operation", "NewHttpAgent")
	}
	
	// Create base agent
	baseAgent := NewBaseAgent(name, description)
	
	// Create shutdown context
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	
	// Create HTTP agent
	httpAgent := &HttpAgent{
		BaseAgent:       baseAgent,
		httpConfig:      httpConfig,
		maxConns:        int64(httpConfig.MaxConnsPerHost),
		protocolVersion: httpConfig.ProtocolVersion,
		shutdownCtx:     shutdownCtx,
		shutdownCancel:  shutdownCancel,
	}
	
	// Initialize connection pool
	httpAgent.connPool = NewConnectionPool(httpConfig.MaxIdleConns, httpConfig.IdleConnTimeout)
	
	return httpAgent, nil
}

// Initialize prepares the HTTP agent with the given configuration.
func (h *HttpAgent) Initialize(ctx context.Context, config *AgentConfig) error {
	// Initialize base agent first
	if err := h.BaseAgent.Initialize(ctx, config); err != nil {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"base agent initialization failed",
			h.Name(),
		).WithCause(err).WithDetail("operation", "Initialize")
	}
	
	h.httpMu.Lock()
	defer h.httpMu.Unlock()
	
	// Setup HTTP transport
	if err := h.setupHttpTransport(); err != nil {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"HTTP transport setup failed",
			h.Name(),
		).WithCause(err).WithDetail("operation", "Initialize")
	}
	
	// Setup HTTP client
	if err := h.setupHttpClient(); err != nil {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"HTTP client setup failed",
			h.Name(),
		).WithCause(err).WithDetail("operation", "Initialize")
	}
	
	// Setup TLS configuration if needed
	if err := h.setupTLSConfig(); err != nil {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"TLS configuration setup failed",
			h.Name(),
		).WithCause(err).WithDetail("operation", "Initialize")
	}
	
	return nil
}

// Start begins the HTTP agent's operation.
func (h *HttpAgent) Start(ctx context.Context) error {
	// Start base agent first
	if err := h.BaseAgent.Start(ctx); err != nil {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"base agent start failed",
			h.Name(),
		).WithCause(err).WithDetail("operation", "Start")
	}
	
	h.httpMu.Lock()
	defer h.httpMu.Unlock()
	
	// Start connection pool maintenance
	if h.connPool != nil {
		go h.connPool.maintainConnections(h.shutdownCtx)
	}
	
	// Start metrics collection
	go h.collectMetrics(h.shutdownCtx)
	
	return nil
}

// Stop gracefully shuts down the HTTP agent.
func (h *HttpAgent) Stop(ctx context.Context) error {
	h.httpMu.Lock()
	
	// Cancel shutdown context to stop background goroutines
	if h.shutdownCancel != nil {
		h.shutdownCancel()
	}
	
	// Close connection pool
	if h.connPool != nil {
		h.connPool.Close()
	}
	
	// Close idle connections in transport
	if h.transport != nil {
		h.transport.CloseIdleConnections()
	}
	
	h.httpMu.Unlock()
	
	// Stop base agent
	if err := h.BaseAgent.Stop(ctx); err != nil {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"base agent stop failed",
			h.Name(),
		).WithCause(err).WithDetail("operation", "Stop")
	}
	
	return nil
}

// Cleanup releases all HTTP-specific resources.
func (h *HttpAgent) Cleanup() error {
	h.httpMu.Lock()
	defer h.httpMu.Unlock()
	
	// Cancel shutdown context
	if h.shutdownCancel != nil {
		h.shutdownCancel()
	}
	
	// Cleanup HTTP client and transport
	if h.transport != nil {
		h.transport.CloseIdleConnections()
		h.transport = nil
	}
	h.client = nil
	h.h2Transport = nil
	h.tlsConfig = nil
	
	// Close connection pool
	if h.connPool != nil {
		h.connPool.Close()
		h.connPool = nil
	}
	
	// Cleanup base agent
	return h.BaseAgent.Cleanup()
}

// HttpClient returns the configured HTTP client.
func (h *HttpAgent) HttpClient() *http.Client {
	h.httpMu.RLock()
	defer h.httpMu.RUnlock()
	return h.client
}

// GetHttpMetrics returns HTTP-specific metrics.
func (h *HttpAgent) GetHttpMetrics() *HttpMetrics {
	return &HttpMetrics{
		RequestCount:      atomic.LoadInt64(&h.requestCount),
		ResponseCount:     atomic.LoadInt64(&h.responseCount),
		ErrorCount:        atomic.LoadInt64(&h.errorCount),
		ActiveConnections: atomic.LoadInt64(&h.activeConns),
		PooledConnections: h.connPool.Size(),
	}
}

// SendRequest sends an HTTP request using the configured client.
func (h *HttpAgent) SendRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	if h.getStatus() != AgentStatusRunning {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			fmt.Sprintf("HTTP agent %s is not running", h.Name()),
			h.Name(),
		)
	}
	
	atomic.AddInt64(&h.requestCount, 1)
	atomic.AddInt64(&h.activeConns, 1)
	defer atomic.AddInt64(&h.activeConns, -1)
	
	h.httpMu.RLock()
	client := h.client
	h.httpMu.RUnlock()
	
	if client == nil {
		atomic.AddInt64(&h.errorCount, 1)
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"HTTP client not initialized",
			h.Name(),
		)
	}
	
	// Set timeout if configured
	if h.httpConfig.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.httpConfig.RequestTimeout)
		defer cancel()
	}
	
	// Send request
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		atomic.AddInt64(&h.errorCount, 1)
		return nil, errors.NewAgentError(
			errors.ErrorTypeExternal,
			fmt.Sprintf("HTTP request failed: %v", err),
			h.Name(),
		)
	}
	
	atomic.AddInt64(&h.responseCount, 1)
	return resp, nil
}

// setupHttpTransport configures the HTTP transport based on protocol version.
func (h *HttpAgent) setupHttpTransport() error {
	config := h.httpConfig
	
	// Create base transport
	h.transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     config.MaxConnsPerHost,
		IdleConnTimeout:     config.IdleConnTimeout,
		TLSHandshakeTimeout: config.TLSHandshakeTimeout,
		
		DisableCompression: config.DisableCompression,
		DisableKeepAlives:  config.DisableKeepAlives,
	}
	
	// Configure HTTP/2 support
	if config.EnableHTTP2 {
		if config.ForceHTTP2 {
			// Force HTTP/2 only
			h.h2Transport = &http2.Transport{
				TLSClientConfig: h.tlsConfig,
			}
		} else {
			// Enable HTTP/2 upgrade
			if err := http2.ConfigureTransport(h.transport); err != nil {
				return errors.NewAgentError(
					errors.ErrorTypeInvalidState,
					"failed to configure HTTP/2 transport",
					h.Name(),
				).WithCause(err).WithDetail("operation", "setupHttpTransport")
			}
		}
	}
	
	return nil
}

// setupHttpClient creates and configures the HTTP client.
func (h *HttpAgent) setupHttpClient() error {
	config := h.httpConfig
	
	// Choose transport based on protocol configuration
	var transport http.RoundTripper
	switch {
	case config.ForceHTTP2 && h.h2Transport != nil:
		transport = h.h2Transport
	default:
		transport = h.transport
	}
	
	h.client = &http.Client{
		Transport: transport,
		Timeout:   config.RequestTimeout,
	}
	
	return nil
}

// setupTLSConfig configures TLS settings.
func (h *HttpAgent) setupTLSConfig() error {
	config := h.httpConfig
	
	if config.TLSConfig != nil {
		h.tlsConfig = config.TLSConfig.Clone()
	} else {
		h.tlsConfig = &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		}
	}
	
	// Apply TLS config to transport
	if h.transport != nil {
		h.transport.TLSClientConfig = h.tlsConfig
	}
	
	if h.h2Transport != nil {
		h.h2Transport.TLSClientConfig = h.tlsConfig
	}
	
	return nil
}

// collectMetrics periodically collects and updates HTTP metrics.
func (h *HttpAgent) collectMetrics(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Update connection pool metrics
			if h.connPool != nil {
				// Connection pool metrics are updated in real-time
			}
		}
	}
}

// DefaultHttpConfig returns a default HTTP configuration.
func DefaultHttpConfig() *HttpConfig {
	return &HttpConfig{
		ProtocolVersion:       HttpProtocolVersionAuto,
		EnableHTTP2:          true,
		ForceHTTP2:           false,
		MaxIdleConns:         100,
		MaxIdleConnsPerHost:  10,
		MaxConnsPerHost:      50,
		IdleConnTimeout:      90 * time.Second,
		KeepAlive:            30 * time.Second,
		DialTimeout:          30 * time.Second,
		RequestTimeout:       60 * time.Second,
		ResponseTimeout:      30 * time.Second,
		TLSHandshakeTimeout:  10 * time.Second,
		InsecureSkipVerify:   false,
		DisableCompression:   false,
		DisableKeepAlives:    false,
		UserAgent:           "AG-UI-HttpAgent/1.0",
		MaxResponseBodySize:  10 * 1024 * 1024, // 10MB
		EnableCircuitBreaker: false,
		CircuitBreakerConfig: &CircuitBreakerConfig{
			Enabled:          false,
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          60 * time.Second,
			HalfOpenMaxCalls: 3,
			FailureRateThreshold: 0.5,
			MinimumRequestThreshold: 10,
		},
	}
}

// validateHttpConfig validates the HTTP configuration.
func validateHttpConfig(config *HttpConfig) error {
	if config == nil {
		return errors.NewValidationError(ErrHTTPConfigNil, "HTTP configuration cannot be nil")
	}
	
	if config.MaxIdleConns <= 0 {
		return errors.NewValidationError(ErrMaxIdleConnsInvalid, "max idle connections must be positive").WithField("MaxIdleConns", config.MaxIdleConns)
	}
	
	if config.MaxIdleConnsPerHost <= 0 {
		return errors.NewValidationError(ErrMaxIdleConnsPerHostInvalid, "max idle connections per host must be positive").WithField("MaxIdleConnsPerHost", config.MaxIdleConnsPerHost)
	}
	
	if config.MaxConnsPerHost <= 0 {
		return errors.NewValidationError(ErrMaxConnsPerHostInvalid, "max connections per host must be positive").WithField("MaxConnsPerHost", config.MaxConnsPerHost)
	}
	
	if config.DialTimeout <= 0 {
		return errors.NewValidationError(ErrDialTimeoutInvalid, "dial timeout must be positive").WithField("DialTimeout", config.DialTimeout.String())
	}
	
	if config.RequestTimeout <= 0 {
		return errors.NewValidationError(ErrRequestTimeoutInvalid, "request timeout must be positive").WithField("RequestTimeout", config.RequestTimeout.String())
	}
	
	if config.MaxResponseBodySize <= 0 {
		return errors.NewValidationError(ErrMaxResponseBodySizeInvalid, "max response body size must be positive").WithField("MaxResponseBodySize", config.MaxResponseBodySize)
	}
	
	return nil
}

// Note: ConnectionPool methods are implemented in http_transport.go