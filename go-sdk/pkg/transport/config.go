package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BaseConfig provides common configuration fields for all transport types.
type BaseConfig struct {
	// Transport type (e.g., "websocket", "http", "grpc")
	Type string `json:"type"`

	// Endpoint URL or address
	Endpoint string `json:"endpoint"`

	// Connection timeout
	Timeout time.Duration `json:"timeout"`

	// Custom headers
	Headers map[string]string `json:"headers,omitempty"`

	// TLS configuration for secure connections
	TLS *tls.Config `json:"-"`

	// EnableCompression enables data compression
	EnableCompression bool `json:"enable_compression"`

	// MaxMessageSize sets the maximum message size in bytes
	MaxMessageSize int64 `json:"max_message_size"`

	// ReadBufferSize sets the read buffer size
	ReadBufferSize int `json:"read_buffer_size"`

	// WriteBufferSize sets the write buffer size
	WriteBufferSize int `json:"write_buffer_size"`

	// KeepAlive configuration
	KeepAlive *KeepAliveConfig `json:"keep_alive,omitempty"`

	// Retry configuration
	Retry *RetryConfig `json:"retry,omitempty"`

	// Circuit breaker configuration
	CircuitBreaker *CircuitBreakerConfig `json:"circuit_breaker,omitempty"`

	// Authentication configuration
	Auth *AuthConfig `json:"auth,omitempty"`

	// Metrics configuration
	Metrics *MetricsConfig `json:"metrics,omitempty"`

	// Custom metadata
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Validate validates the base configuration.
func (c *BaseConfig) Validate() error {
	if c.Type == "" {
		return fmt.Errorf("transport type is required")
	}

	if c.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}

	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}

	if c.MaxMessageSize <= 0 {
		c.MaxMessageSize = 64 * 1024 * 1024 // 64MB default
	}

	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = 4096
	}

	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = 4096
	}

	// Validate endpoint URL
	if _, err := url.Parse(c.Endpoint); err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	// Validate configurations
	if c.KeepAlive != nil {
		if err := c.KeepAlive.Validate(); err != nil {
			return fmt.Errorf("invalid keep-alive config: %w", err)
		}
	}

	if c.Retry != nil {
		if err := c.Retry.Validate(); err != nil {
			return fmt.Errorf("invalid retry config: %w", err)
		}
	}

	if c.CircuitBreaker != nil {
		if err := c.CircuitBreaker.Validate(); err != nil {
			return fmt.Errorf("invalid circuit breaker config: %w", err)
		}
	}

	if c.Auth != nil {
		if err := c.Auth.Validate(); err != nil {
			return fmt.Errorf("invalid auth config: %w", err)
		}
	}

	if c.Metrics != nil {
		if err := c.Metrics.Validate(); err != nil {
			return fmt.Errorf("invalid metrics config: %w", err)
		}
	}

	return nil
}

// Clone creates a deep copy of the base configuration.
func (c *BaseConfig) Clone() Config {
	clone := &BaseConfig{
		Type:              c.Type,
		Endpoint:          c.Endpoint,
		Timeout:           c.Timeout,
		EnableCompression: c.EnableCompression,
		MaxMessageSize:    c.MaxMessageSize,
		ReadBufferSize:    c.ReadBufferSize,
		WriteBufferSize:   c.WriteBufferSize,
	}

	// Deep copy headers
	if c.Headers != nil {
		clone.Headers = make(map[string]string, len(c.Headers))
		for k, v := range c.Headers {
			clone.Headers[k] = v
		}
	}

	// Deep copy TLS config
	if c.TLS != nil {
		clone.TLS = c.TLS.Clone()
	}

	// Deep copy configurations
	if c.KeepAlive != nil {
		clone.KeepAlive = c.KeepAlive.Clone()
	}

	if c.Retry != nil {
		clone.Retry = c.Retry.Clone()
	}

	if c.CircuitBreaker != nil {
		clone.CircuitBreaker = c.CircuitBreaker.Clone()
	}

	if c.Auth != nil {
		clone.Auth = c.Auth.Clone()
	}

	if c.Metrics != nil {
		clone.Metrics = c.Metrics.Clone()
	}

	// Deep copy metadata
	if c.Metadata != nil {
		clone.Metadata = make(map[string]any, len(c.Metadata))
		for k, v := range c.Metadata {
			clone.Metadata[k] = v
		}
	}

	return clone
}

// GetType returns the transport type.
func (c *BaseConfig) GetType() string {
	return c.Type
}

// GetEndpoint returns the endpoint URL.
func (c *BaseConfig) GetEndpoint() string {
	return c.Endpoint
}

// GetTimeout returns the connection timeout.
func (c *BaseConfig) GetTimeout() time.Duration {
	return c.Timeout
}

// GetHeaders returns the custom headers.
func (c *BaseConfig) GetHeaders() map[string]string {
	return c.Headers
}

// IsSecure returns true if the connection uses TLS.
func (c *BaseConfig) IsSecure() bool {
	if c.TLS != nil {
		return true
	}
	
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return false
	}
	
	return u.Scheme == "wss" || u.Scheme == "https"
}

// WebSocketConfig contains configuration specific to WebSocket transport.
type WebSocketConfig struct {
	*BaseConfig

	// Subprotocols specifies the WebSocket subprotocols to negotiate
	Subprotocols []string `json:"subprotocols,omitempty"`

	// Origin specifies the Origin header value
	Origin string `json:"origin,omitempty"`

	// PingInterval specifies the interval between ping frames
	PingInterval time.Duration `json:"ping_interval"`

	// PongTimeout specifies the timeout for pong responses
	PongTimeout time.Duration `json:"pong_timeout"`

	// CloseTimeout specifies the timeout for close handshake
	CloseTimeout time.Duration `json:"close_timeout"`

	// HandshakeTimeout specifies the timeout for WebSocket handshake
	HandshakeTimeout time.Duration `json:"handshake_timeout"`

	// EnablePingPong enables ping-pong keep-alive mechanism
	EnablePingPong bool `json:"enable_ping_pong"`

	// EnableAutoReconnect enables automatic reconnection on connection loss
	EnableAutoReconnect bool `json:"enable_auto_reconnect"`

	// ReconnectInterval specifies the interval between reconnection attempts
	ReconnectInterval time.Duration `json:"reconnect_interval"`

	// MaxReconnectAttempts specifies the maximum number of reconnection attempts
	MaxReconnectAttempts int `json:"max_reconnect_attempts"`

	// MessageQueue configuration for buffering messages during reconnection
	MessageQueue *MessageQueueConfig `json:"message_queue,omitempty"`

	// Dialer specifies a custom dialer for WebSocket connections
	Dialer *net.Dialer `json:"-"`

	// NetDial allows custom network dialing
	NetDial func(network, addr string) (net.Conn, error) `json:"-"`

	// NetDialContext allows custom network dialing with context
	NetDialContext func(ctx context.Context, network, addr string) (net.Conn, error) `json:"-"`

	// CheckOrigin allows custom origin validation
	CheckOrigin func(r *http.Request) bool `json:"-"`

	// EnableBinaryFrames enables support for binary WebSocket frames
	EnableBinaryFrames bool `json:"enable_binary_frames"`

	// CompressionLevel sets the compression level for per-message deflate
	CompressionLevel int `json:"compression_level"`

	// CompressionThreshold sets the minimum message size for compression
	CompressionThreshold int `json:"compression_threshold"`
}

// NewWebSocketConfig creates a new WebSocket configuration with defaults.
func NewWebSocketConfig(endpoint string) *WebSocketConfig {
	return &WebSocketConfig{
		BaseConfig: &BaseConfig{
			Type:              "websocket",
			Endpoint:          endpoint,
			Timeout:           30 * time.Second,
			EnableCompression: true,
			MaxMessageSize:    64 * 1024 * 1024, // 64MB
			ReadBufferSize:    4096,
			WriteBufferSize:   4096,
		},
		PingInterval:            30 * time.Second,
		PongTimeout:             10 * time.Second,
		CloseTimeout:            10 * time.Second,
		HandshakeTimeout:        10 * time.Second,
		EnablePingPong:          true,
		EnableAutoReconnect:     true,
		ReconnectInterval:       5 * time.Second,
		MaxReconnectAttempts:    10,
		EnableBinaryFrames:      true,
		CompressionLevel:        6,
		CompressionThreshold:    1024,
	}
}

// Validate validates the WebSocket configuration.
func (c *WebSocketConfig) Validate() error {
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	if c.Type != "websocket" {
		return fmt.Errorf("invalid transport type for WebSocket config: %s", c.Type)
	}

	// Validate WebSocket-specific URL scheme
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if u.Scheme != "ws" && u.Scheme != "wss" {
		return fmt.Errorf("WebSocket endpoint must use 'ws' or 'wss' scheme, got: %s", u.Scheme)
	}

	if c.PingInterval <= 0 {
		c.PingInterval = 30 * time.Second
	}

	if c.PongTimeout <= 0 {
		c.PongTimeout = 10 * time.Second
	}

	if c.CloseTimeout <= 0 {
		c.CloseTimeout = 10 * time.Second
	}

	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = 10 * time.Second
	}

	if c.ReconnectInterval <= 0 {
		c.ReconnectInterval = 5 * time.Second
	}

	if c.MaxReconnectAttempts < 0 {
		c.MaxReconnectAttempts = 10
	}

	if c.CompressionLevel < 0 || c.CompressionLevel > 9 {
		c.CompressionLevel = 6
	}

	if c.CompressionThreshold < 0 {
		c.CompressionThreshold = 1024
	}

	if c.MessageQueue != nil {
		if err := c.MessageQueue.Validate(); err != nil {
			return fmt.Errorf("invalid message queue config: %w", err)
		}
	}

	return nil
}

// Clone creates a deep copy of the WebSocket configuration.
func (c *WebSocketConfig) Clone() Config {
	clone := &WebSocketConfig{
		BaseConfig:               c.BaseConfig.Clone().(*BaseConfig),
		Subprotocols:            make([]string, len(c.Subprotocols)),
		Origin:                  c.Origin,
		PingInterval:            c.PingInterval,
		PongTimeout:             c.PongTimeout,
		CloseTimeout:            c.CloseTimeout,
		HandshakeTimeout:        c.HandshakeTimeout,
		EnablePingPong:          c.EnablePingPong,
		EnableAutoReconnect:     c.EnableAutoReconnect,
		ReconnectInterval:       c.ReconnectInterval,
		MaxReconnectAttempts:    c.MaxReconnectAttempts,
		EnableBinaryFrames:      c.EnableBinaryFrames,
		CompressionLevel:        c.CompressionLevel,
		CompressionThreshold:    c.CompressionThreshold,
		Dialer:                  c.Dialer,
		NetDial:                 c.NetDial,
		NetDialContext:          c.NetDialContext,
		CheckOrigin:             c.CheckOrigin,
	}

	copy(clone.Subprotocols, c.Subprotocols)

	if c.MessageQueue != nil {
		clone.MessageQueue = c.MessageQueue.Clone()
	}

	return clone
}

// HTTPConfig contains configuration specific to HTTP transport.
type HTTPConfig struct {
	*BaseConfig

	// Method specifies the HTTP method (GET, POST, etc.)
	Method string `json:"method"`

	// UserAgent specifies the User-Agent header
	UserAgent string `json:"user_agent"`

	// MaxIdleConns specifies the maximum number of idle connections
	MaxIdleConns int `json:"max_idle_conns"`

	// MaxIdleConnsPerHost specifies the maximum number of idle connections per host
	MaxIdleConnsPerHost int `json:"max_idle_conns_per_host"`

	// MaxConnsPerHost specifies the maximum number of connections per host
	MaxConnsPerHost int `json:"max_conns_per_host"`

	// IdleConnTimeout specifies the timeout for idle connections
	IdleConnTimeout time.Duration `json:"idle_conn_timeout"`

	// ResponseHeaderTimeout specifies the timeout for response headers
	ResponseHeaderTimeout time.Duration `json:"response_header_timeout"`

	// ExpectContinueTimeout specifies the timeout for Expect: 100-continue
	ExpectContinueTimeout time.Duration `json:"expect_continue_timeout"`

	// DisableKeepAlives disables HTTP keep-alives
	DisableKeepAlives bool `json:"disable_keep_alives"`

	// DisableCompression disables HTTP compression
	DisableCompression bool `json:"disable_compression"`

	// Transport allows custom HTTP transport
	Transport *http.Transport `json:"-"`

	// Client allows custom HTTP client
	Client *http.Client `json:"-"`

	// EnableStreaming enables HTTP/2 streaming
	EnableStreaming bool `json:"enable_streaming"`

	// StreamingBufferSize sets the buffer size for streaming
	StreamingBufferSize int `json:"streaming_buffer_size"`
}

// NewHTTPConfig creates a new HTTP configuration with defaults.
func NewHTTPConfig(endpoint string) *HTTPConfig {
	return &HTTPConfig{
		BaseConfig: &BaseConfig{
			Type:              "http",
			Endpoint:          endpoint,
			Timeout:           30 * time.Second,
			EnableCompression: true,
			MaxMessageSize:    64 * 1024 * 1024, // 64MB
			ReadBufferSize:    4096,
			WriteBufferSize:   4096,
		},
		Method:                    "POST",
		UserAgent:                "ag-ui-go-sdk/1.0",
		MaxIdleConns:             100,
		MaxIdleConnsPerHost:      2,
		MaxConnsPerHost:          0,
		IdleConnTimeout:          90 * time.Second,
		ResponseHeaderTimeout:    30 * time.Second,
		ExpectContinueTimeout:    1 * time.Second,
		DisableKeepAlives:        false,
		DisableCompression:       false,
		EnableStreaming:          false,
		StreamingBufferSize:      8192,
	}
}

// Validate validates the HTTP configuration.
func (c *HTTPConfig) Validate() error {
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	if c.Type != "http" {
		return fmt.Errorf("invalid transport type for HTTP config: %s", c.Type)
	}

	// Validate HTTP-specific URL scheme
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("HTTP endpoint must use 'http' or 'https' scheme, got: %s", u.Scheme)
	}

	if c.Method == "" {
		c.Method = "POST"
	}

	validMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	methodValid := false
	for _, method := range validMethods {
		if strings.EqualFold(c.Method, method) {
			c.Method = method
			methodValid = true
			break
		}
	}

	if !methodValid {
		return fmt.Errorf("invalid HTTP method: %s", c.Method)
	}

	if c.MaxIdleConns <= 0 {
		c.MaxIdleConns = 100
	}

	if c.MaxIdleConnsPerHost <= 0 {
		c.MaxIdleConnsPerHost = 2
	}

	if c.IdleConnTimeout <= 0 {
		c.IdleConnTimeout = 90 * time.Second
	}

	if c.ResponseHeaderTimeout <= 0 {
		c.ResponseHeaderTimeout = 30 * time.Second
	}

	if c.ExpectContinueTimeout <= 0 {
		c.ExpectContinueTimeout = 1 * time.Second
	}

	if c.StreamingBufferSize <= 0 {
		c.StreamingBufferSize = 8192
	}

	return nil
}

// Clone creates a deep copy of the HTTP configuration.
func (c *HTTPConfig) Clone() Config {
	clone := &HTTPConfig{
		BaseConfig:               c.BaseConfig.Clone().(*BaseConfig),
		Method:                   c.Method,
		UserAgent:                c.UserAgent,
		MaxIdleConns:             c.MaxIdleConns,
		MaxIdleConnsPerHost:      c.MaxIdleConnsPerHost,
		MaxConnsPerHost:          c.MaxConnsPerHost,
		IdleConnTimeout:          c.IdleConnTimeout,
		ResponseHeaderTimeout:    c.ResponseHeaderTimeout,
		ExpectContinueTimeout:    c.ExpectContinueTimeout,
		DisableKeepAlives:        c.DisableKeepAlives,
		DisableCompression:       c.DisableCompression,
		EnableStreaming:          c.EnableStreaming,
		StreamingBufferSize:      c.StreamingBufferSize,
		Transport:                c.Transport,
		Client:                   c.Client,
	}

	return clone
}

// KeepAliveConfig contains keep-alive configuration.
type KeepAliveConfig struct {
	// Enabled enables keep-alive
	Enabled bool `json:"enabled"`

	// Interval specifies the keep-alive interval
	Interval time.Duration `json:"interval"`

	// Timeout specifies the keep-alive timeout
	Timeout time.Duration `json:"timeout"`

	// MaxRetries specifies the maximum number of keep-alive retries
	MaxRetries int `json:"max_retries"`
}

// Validate validates the keep-alive configuration.
func (c *KeepAliveConfig) Validate() error {
	if c.Enabled {
		if c.Interval <= 0 {
			c.Interval = 30 * time.Second
		}

		if c.Timeout <= 0 {
			c.Timeout = 10 * time.Second
		}

		if c.MaxRetries < 0 {
			c.MaxRetries = 3
		}
	}

	return nil
}

// Clone creates a deep copy of the keep-alive configuration.
func (c *KeepAliveConfig) Clone() *KeepAliveConfig {
	return &KeepAliveConfig{
		Enabled:    c.Enabled,
		Interval:   c.Interval,
		Timeout:    c.Timeout,
		MaxRetries: c.MaxRetries,
	}
}

// RetryConfig contains retry configuration.
type RetryConfig struct {
	// Enabled enables retry logic
	Enabled bool `json:"enabled"`

	// MaxAttempts specifies the maximum number of retry attempts
	MaxAttempts int `json:"max_attempts"`

	// InitialDelay specifies the initial delay between retries
	InitialDelay time.Duration `json:"initial_delay"`

	// MaxDelay specifies the maximum delay between retries
	MaxDelay time.Duration `json:"max_delay"`

	// Multiplier specifies the backoff multiplier
	Multiplier float64 `json:"multiplier"`

	// Jitter enables random jitter in retry delays
	Jitter bool `json:"jitter"`

	// RetryableErrors specifies which errors should trigger a retry
	RetryableErrors []string `json:"retryable_errors,omitempty"`
}

// Validate validates the retry configuration.
func (c *RetryConfig) Validate() error {
	if c.Enabled {
		if c.MaxAttempts <= 0 {
			c.MaxAttempts = 3
		}

		if c.InitialDelay <= 0 {
			c.InitialDelay = 1 * time.Second
		}

		if c.MaxDelay <= 0 {
			c.MaxDelay = 60 * time.Second
		}

		if c.Multiplier <= 0 {
			c.Multiplier = 2.0
		}

		if c.InitialDelay > c.MaxDelay {
			return fmt.Errorf("initial delay cannot be greater than max delay")
		}
	}

	return nil
}

// Clone creates a deep copy of the retry configuration.
func (c *RetryConfig) Clone() *RetryConfig {
	clone := &RetryConfig{
		Enabled:      c.Enabled,
		MaxAttempts:  c.MaxAttempts,
		InitialDelay: c.InitialDelay,
		MaxDelay:     c.MaxDelay,
		Multiplier:   c.Multiplier,
		Jitter:       c.Jitter,
	}

	if c.RetryableErrors != nil {
		clone.RetryableErrors = make([]string, len(c.RetryableErrors))
		copy(clone.RetryableErrors, c.RetryableErrors)
	}

	return clone
}

// CircuitBreakerConfig contains circuit breaker configuration.
type CircuitBreakerConfig struct {
	// Enabled enables circuit breaker
	Enabled bool `json:"enabled"`

	// FailureThreshold specifies the failure threshold to open the circuit
	FailureThreshold int `json:"failure_threshold"`

	// RecoveryTimeout specifies the recovery timeout
	RecoveryTimeout time.Duration `json:"recovery_timeout"`

	// HalfOpenMaxCalls specifies the maximum calls in half-open state
	HalfOpenMaxCalls int `json:"half_open_max_calls"`

	// MinCalls specifies the minimum calls before circuit evaluation
	MinCalls int `json:"min_calls"`

	// SlidingWindowSize specifies the sliding window size
	SlidingWindowSize int `json:"sliding_window_size"`

	// FailureRateThreshold specifies the failure rate threshold (percentage)
	FailureRateThreshold float64 `json:"failure_rate_threshold"`
}

// Validate validates the circuit breaker configuration.
func (c *CircuitBreakerConfig) Validate() error {
	if c.Enabled {
		if c.FailureThreshold <= 0 {
			c.FailureThreshold = 5
		}

		if c.RecoveryTimeout <= 0 {
			c.RecoveryTimeout = 30 * time.Second
		}

		if c.HalfOpenMaxCalls <= 0 {
			c.HalfOpenMaxCalls = 10
		}

		if c.MinCalls <= 0 {
			c.MinCalls = 20
		}

		if c.SlidingWindowSize <= 0 {
			c.SlidingWindowSize = 100
		}

		if c.FailureRateThreshold <= 0 || c.FailureRateThreshold > 100 {
			c.FailureRateThreshold = 50.0
		}
	}

	return nil
}

// Clone creates a deep copy of the circuit breaker configuration.
func (c *CircuitBreakerConfig) Clone() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Enabled:              c.Enabled,
		FailureThreshold:     c.FailureThreshold,
		RecoveryTimeout:      c.RecoveryTimeout,
		HalfOpenMaxCalls:     c.HalfOpenMaxCalls,
		MinCalls:             c.MinCalls,
		SlidingWindowSize:    c.SlidingWindowSize,
		FailureRateThreshold: c.FailureRateThreshold,
	}
}

// AuthConfig contains authentication configuration.
type AuthConfig struct {
	// Type specifies the authentication type
	Type string `json:"type"`

	// Token specifies the authentication token
	Token string `json:"token,omitempty"`

	// Username specifies the username for basic auth
	Username string `json:"username,omitempty"`

	// Password specifies the password for basic auth
	Password string `json:"password,omitempty"`

	// APIKey specifies the API key
	APIKey string `json:"api_key,omitempty"`

	// APISecret specifies the API secret
	APISecret string `json:"api_secret,omitempty"`

	// TokenURL specifies the token endpoint URL
	TokenURL string `json:"token_url,omitempty"`

	// RefreshURL specifies the refresh endpoint URL
	RefreshURL string `json:"refresh_url,omitempty"`

	// RefreshToken specifies the refresh token
	RefreshToken string `json:"refresh_token,omitempty"`

	// ClientID specifies the OAuth client ID
	ClientID string `json:"client_id,omitempty"`

	// ClientSecret specifies the OAuth client secret
	ClientSecret string `json:"client_secret,omitempty"`

	// Scopes specifies the OAuth scopes
	Scopes []string `json:"scopes,omitempty"`

	// ExpiresAt specifies when the token expires
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// CustomHeaders specifies custom authentication headers
	CustomHeaders map[string]string `json:"custom_headers,omitempty"`
}

// Validate validates the authentication configuration.
func (c *AuthConfig) Validate() error {
	if c.Type == "" {
		return fmt.Errorf("authentication type is required")
	}

	switch c.Type {
	case "bearer":
		if c.Token == "" {
			return fmt.Errorf("token is required for bearer authentication")
		}
	case "basic":
		if c.Username == "" || c.Password == "" {
			return fmt.Errorf("username and password are required for basic authentication")
		}
	case "api_key":
		if c.APIKey == "" {
			return fmt.Errorf("API key is required for API key authentication")
		}
	case "oauth2":
		if c.ClientID == "" || c.ClientSecret == "" {
			return fmt.Errorf("client ID and secret are required for OAuth2 authentication")
		}
		if c.TokenURL == "" {
			return fmt.Errorf("token URL is required for OAuth2 authentication")
		}
	case "custom":
		if len(c.CustomHeaders) == 0 {
			return fmt.Errorf("custom headers are required for custom authentication")
		}
	default:
		return fmt.Errorf("unsupported authentication type: %s", c.Type)
	}

	return nil
}

// Clone creates a deep copy of the authentication configuration.
func (c *AuthConfig) Clone() *AuthConfig {
	clone := &AuthConfig{
		Type:         c.Type,
		Token:        c.Token,
		Username:     c.Username,
		Password:     c.Password,
		APIKey:       c.APIKey,
		APISecret:    c.APISecret,
		TokenURL:     c.TokenURL,
		RefreshURL:   c.RefreshURL,
		RefreshToken: c.RefreshToken,
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		ExpiresAt:    c.ExpiresAt,
	}

	if c.Scopes != nil {
		clone.Scopes = make([]string, len(c.Scopes))
		copy(clone.Scopes, c.Scopes)
	}

	if c.CustomHeaders != nil {
		clone.CustomHeaders = make(map[string]string, len(c.CustomHeaders))
		for k, v := range c.CustomHeaders {
			clone.CustomHeaders[k] = v
		}
	}

	return clone
}

// MetricsConfig contains metrics configuration.
type MetricsConfig struct {
	// Enabled enables metrics collection
	Enabled bool `json:"enabled"`

	// Namespace specifies the metrics namespace
	Namespace string `json:"namespace"`

	// Labels specifies additional labels for metrics
	Labels map[string]string `json:"labels,omitempty"`

	// CollectionInterval specifies the metrics collection interval
	CollectionInterval time.Duration `json:"collection_interval"`

	// ExportInterval specifies the metrics export interval
	ExportInterval time.Duration `json:"export_interval"`

	// Endpoint specifies the metrics endpoint
	Endpoint string `json:"endpoint,omitempty"`

	// EnableDetailedMetrics enables detailed metrics collection
	EnableDetailedMetrics bool `json:"enable_detailed_metrics"`

	// EnableHistograms enables histogram metrics
	EnableHistograms bool `json:"enable_histograms"`

	// HistogramBuckets specifies histogram bucket boundaries
	HistogramBuckets []float64 `json:"histogram_buckets,omitempty"`
}

// Validate validates the metrics configuration.
func (c *MetricsConfig) Validate() error {
	if c.Enabled {
		if c.Namespace == "" {
			c.Namespace = "ag_ui_transport"
		}

		if c.CollectionInterval <= 0 {
			c.CollectionInterval = 10 * time.Second
		}

		if c.ExportInterval <= 0 {
			c.ExportInterval = 60 * time.Second
		}

		if c.EnableHistograms && len(c.HistogramBuckets) == 0 {
			// Default histogram buckets for latency measurements
			c.HistogramBuckets = []float64{
				0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
			}
		}
	}

	return nil
}

// Clone creates a deep copy of the metrics configuration.
func (c *MetricsConfig) Clone() *MetricsConfig {
	clone := &MetricsConfig{
		Enabled:               c.Enabled,
		Namespace:             c.Namespace,
		CollectionInterval:    c.CollectionInterval,
		ExportInterval:        c.ExportInterval,
		Endpoint:              c.Endpoint,
		EnableDetailedMetrics: c.EnableDetailedMetrics,
		EnableHistograms:      c.EnableHistograms,
	}

	if c.Labels != nil {
		clone.Labels = make(map[string]string, len(c.Labels))
		for k, v := range c.Labels {
			clone.Labels[k] = v
		}
	}

	if c.HistogramBuckets != nil {
		clone.HistogramBuckets = make([]float64, len(c.HistogramBuckets))
		copy(clone.HistogramBuckets, c.HistogramBuckets)
	}

	return clone
}

// MessageQueueConfig contains message queue configuration.
type MessageQueueConfig struct {
	// Enabled enables message queuing
	Enabled bool `json:"enabled"`

	// MaxSize specifies the maximum queue size
	MaxSize int `json:"max_size"`

	// MaxMemory specifies the maximum memory usage for the queue
	MaxMemory int64 `json:"max_memory"`

	// PersistToDisk enables persisting messages to disk
	PersistToDisk bool `json:"persist_to_disk"`

	// PersistenceDir specifies the directory for persisted messages
	PersistenceDir string `json:"persistence_dir,omitempty"`

	// FlushInterval specifies the flush interval for disk persistence
	FlushInterval time.Duration `json:"flush_interval"`

	// DropPolicy specifies the policy for dropping messages when queue is full
	DropPolicy string `json:"drop_policy"`

	// Priority enables priority queuing
	Priority bool `json:"priority"`

	// Compression enables message compression in the queue
	Compression bool `json:"compression"`

	// Deduplication enables message deduplication
	Deduplication bool `json:"deduplication"`

	// TTL specifies the time-to-live for messages
	TTL time.Duration `json:"ttl"`
}

// Validate validates the message queue configuration.
func (c *MessageQueueConfig) Validate() error {
	if c.Enabled {
		if c.MaxSize <= 0 {
			c.MaxSize = 10000
		}

		if c.MaxMemory <= 0 {
			c.MaxMemory = 64 * 1024 * 1024 // 64MB
		}

		if c.PersistToDisk && c.PersistenceDir == "" {
			return fmt.Errorf("persistence directory is required when persist_to_disk is enabled")
		}

		if c.FlushInterval <= 0 {
			c.FlushInterval = 5 * time.Second
		}

		if c.DropPolicy == "" {
			c.DropPolicy = "oldest"
		}

		validDropPolicies := []string{"oldest", "newest", "random", "priority"}
		policyValid := false
		for _, policy := range validDropPolicies {
			if c.DropPolicy == policy {
				policyValid = true
				break
			}
		}

		if !policyValid {
			return fmt.Errorf("invalid drop policy: %s", c.DropPolicy)
		}

		if c.TTL <= 0 {
			c.TTL = 24 * time.Hour
		}
	}

	return nil
}

// Clone creates a deep copy of the message queue configuration.
func (c *MessageQueueConfig) Clone() *MessageQueueConfig {
	return &MessageQueueConfig{
		Enabled:        c.Enabled,
		MaxSize:        c.MaxSize,
		MaxMemory:      c.MaxMemory,
		PersistToDisk:  c.PersistToDisk,
		PersistenceDir: c.PersistenceDir,
		FlushInterval:  c.FlushInterval,
		DropPolicy:     c.DropPolicy,
		Priority:       c.Priority,
		Compression:    c.Compression,
		Deduplication:  c.Deduplication,
		TTL:            c.TTL,
	}
}

// ConfigBuilder provides a fluent interface for building transport configurations.
type ConfigBuilder struct {
	config Config
}

// NewConfigBuilder creates a new configuration builder.
func NewConfigBuilder(transportType string) *ConfigBuilder {
	var config Config
	switch transportType {
	case "websocket":
		config = NewWebSocketConfig("")
	case "http":
		config = NewHTTPConfig("")
	default:
		config = &BaseConfig{Type: transportType}
	}
	
	return &ConfigBuilder{config: config}
}

// WithEndpoint sets the endpoint URL.
func (b *ConfigBuilder) WithEndpoint(endpoint string) *ConfigBuilder {
	switch c := b.config.(type) {
	case *WebSocketConfig:
		c.BaseConfig.Endpoint = endpoint
	case *HTTPConfig:
		c.BaseConfig.Endpoint = endpoint
	case *BaseConfig:
		c.Endpoint = endpoint
	}
	return b
}

// WithTimeout sets the connection timeout.
func (b *ConfigBuilder) WithTimeout(timeout time.Duration) *ConfigBuilder {
	switch c := b.config.(type) {
	case *WebSocketConfig:
		c.BaseConfig.Timeout = timeout
	case *HTTPConfig:
		c.BaseConfig.Timeout = timeout
	case *BaseConfig:
		c.Timeout = timeout
	}
	return b
}

// WithHeaders sets custom headers.
func (b *ConfigBuilder) WithHeaders(headers map[string]string) *ConfigBuilder {
	switch c := b.config.(type) {
	case *WebSocketConfig:
		c.BaseConfig.Headers = headers
	case *HTTPConfig:
		c.BaseConfig.Headers = headers
	case *BaseConfig:
		c.Headers = headers
	}
	return b
}

// WithTLS sets TLS configuration.
func (b *ConfigBuilder) WithTLS(tls *tls.Config) *ConfigBuilder {
	switch c := b.config.(type) {
	case *WebSocketConfig:
		c.BaseConfig.TLS = tls
	case *HTTPConfig:
		c.BaseConfig.TLS = tls
	case *BaseConfig:
		c.TLS = tls
	}
	return b
}

// WithAuth sets authentication configuration.
func (b *ConfigBuilder) WithAuth(auth *AuthConfig) *ConfigBuilder {
	switch c := b.config.(type) {
	case *WebSocketConfig:
		c.BaseConfig.Auth = auth
	case *HTTPConfig:
		c.BaseConfig.Auth = auth
	case *BaseConfig:
		c.Auth = auth
	}
	return b
}

// WithRetry sets retry configuration.
func (b *ConfigBuilder) WithRetry(retry *RetryConfig) *ConfigBuilder {
	switch c := b.config.(type) {
	case *WebSocketConfig:
		c.BaseConfig.Retry = retry
	case *HTTPConfig:
		c.BaseConfig.Retry = retry
	case *BaseConfig:
		c.Retry = retry
	}
	return b
}

// WithMetrics sets metrics configuration.
func (b *ConfigBuilder) WithMetrics(metrics *MetricsConfig) *ConfigBuilder {
	switch c := b.config.(type) {
	case *WebSocketConfig:
		c.BaseConfig.Metrics = metrics
	case *HTTPConfig:
		c.BaseConfig.Metrics = metrics
	case *BaseConfig:
		c.Metrics = metrics
	}
	return b
}

// Build builds and validates the configuration.
func (b *ConfigBuilder) Build() (Config, error) {
	if err := b.config.Validate(); err != nil {
		return nil, err
	}
	return b.config, nil
}

// MustBuild builds the configuration and panics if validation fails.
func (b *ConfigBuilder) MustBuild() Config {
	config, err := b.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build config: %v", err))
	}
	return config
}