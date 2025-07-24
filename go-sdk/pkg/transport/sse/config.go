package sse

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap/zapcore"
)

// ComprehensiveConfig represents the complete configuration for HTTP SSE transport
type ComprehensiveConfig struct {
	// Connection Configuration
	Connection ConnectionConfig `json:"connection" yaml:"connection"`

	// Retry Configuration
	Retry RetryConfig `json:"retry" yaml:"retry"`

	// Security Configuration
	Security SecurityConfig `json:"security" yaml:"security"`

	// Performance Configuration
	Performance PerformanceConfig `json:"performance" yaml:"performance"`

	// Monitoring Configuration
	Monitoring MonitoringConfig `json:"monitoring" yaml:"monitoring"`

	// Environment-specific settings
	Environment Environment `json:"environment" yaml:"environment"`

	// Feature flags
	Features FeatureFlags `json:"features" yaml:"features"`
}

// ConnectionConfig defines connection-related settings
type ConnectionConfig struct {
	// Base URL for the SSE endpoint
	BaseURL string `json:"base_url" yaml:"base_url"`

	// Endpoint path for SSE connections
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Connection timeout
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout"`

	// Read timeout for receiving events
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout"`

	// Write timeout for sending requests
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`

	// Keep-alive settings
	KeepAlive KeepAliveConfig `json:"keep_alive" yaml:"keep_alive"`

	// TLS configuration
	TLS TLSConfig `json:"tls" yaml:"tls"`

	// HTTP client configuration
	HTTPClient HTTPClientConfig `json:"http_client" yaml:"http_client"`

	// Connection pool settings
	ConnectionPool ConnectionPoolConfig `json:"connection_pool" yaml:"connection_pool"`
}

// KeepAliveConfig defines TCP keep-alive settings
type KeepAliveConfig struct {
	// Enable TCP keep-alive
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Keep-alive interval
	Interval time.Duration `json:"interval" yaml:"interval"`

	// Idle timeout before sending keep-alive probes
	IdleTimeout time.Duration `json:"idle_timeout" yaml:"idle_timeout"`

	// Number of keep-alive probes before connection is considered dead
	ProbeCount int `json:"probe_count" yaml:"probe_count"`
}

// TLSConfig defines TLS/SSL settings
type TLSConfig struct {
	// Enable TLS
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Skip certificate verification (insecure)
	InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`

	// Certificate file path
	CertFile string `json:"cert_file" yaml:"cert_file"`

	// Private key file path
	KeyFile string `json:"key_file" yaml:"key_file"`

	// CA certificate file path
	CAFile string `json:"ca_file" yaml:"ca_file"`

	// Server name for certificate verification
	ServerName string `json:"server_name" yaml:"server_name"`

	// Minimum TLS version
	MinVersion uint16 `json:"min_version" yaml:"min_version"`

	// Maximum TLS version
	MaxVersion uint16 `json:"max_version" yaml:"max_version"`

	// Cipher suites
	CipherSuites []uint16 `json:"cipher_suites" yaml:"cipher_suites"`
}

// HTTPClientConfig defines HTTP client settings
type HTTPClientConfig struct {
	// User agent string
	UserAgent string `json:"user_agent" yaml:"user_agent"`

	// Custom headers
	Headers map[string]string `json:"headers" yaml:"headers"`

	// HTTP proxy URL
	ProxyURL string `json:"proxy_url" yaml:"proxy_url"`

	// Disable HTTP/2
	DisableHTTP2 bool `json:"disable_http2" yaml:"disable_http2"`

	// Maximum idle connections
	MaxIdleConns int `json:"max_idle_conns" yaml:"max_idle_conns"`

	// Maximum idle connections per host
	MaxIdleConnsPerHost int `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`

	// Maximum connections per host
	MaxConnsPerHost int `json:"max_conns_per_host" yaml:"max_conns_per_host"`

	// Idle connection timeout
	IdleConnTimeout time.Duration `json:"idle_conn_timeout" yaml:"idle_conn_timeout"`

	// Response header timeout
	ResponseHeaderTimeout time.Duration `json:"response_header_timeout" yaml:"response_header_timeout"`

	// Expect continue timeout
	ExpectContinueTimeout time.Duration `json:"expect_continue_timeout" yaml:"expect_continue_timeout"`
}

// ConnectionPoolConfig defines connection pooling settings
type ConnectionPoolConfig struct {
	// Maximum number of connections in the pool
	MaxConnections int `json:"max_connections" yaml:"max_connections"`

	// Maximum number of idle connections
	MaxIdleConnections int `json:"max_idle_connections" yaml:"max_idle_connections"`

	// Connection lifetime
	ConnectionLifetime time.Duration `json:"connection_lifetime" yaml:"connection_lifetime"`

	// Connection idle timeout
	IdleTimeout time.Duration `json:"idle_timeout" yaml:"idle_timeout"`

	// Health check interval
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
}

// RetryConfig defines retry behavior
type RetryConfig struct {
	// Enable retry mechanism
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Maximum number of retries
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// Initial retry delay
	InitialDelay time.Duration `json:"initial_delay" yaml:"initial_delay"`

	// Maximum retry delay
	MaxDelay time.Duration `json:"max_delay" yaml:"max_delay"`

	// Backoff strategy
	BackoffStrategy BackoffStrategy `json:"backoff_strategy" yaml:"backoff_strategy"`

	// Backoff multiplier (for exponential backoff)
	BackoffMultiplier float64 `json:"backoff_multiplier" yaml:"backoff_multiplier"`

	// Jitter factor (0.0 to 1.0)
	JitterFactor float64 `json:"jitter_factor" yaml:"jitter_factor"`

	// Retry on specific HTTP status codes
	RetryOnStatusCodes []int `json:"retry_on_status_codes" yaml:"retry_on_status_codes"`

	// Retry on specific errors
	RetryOnErrors []string `json:"retry_on_errors" yaml:"retry_on_errors"`

	// Circuit breaker settings
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
}

// BackoffStrategy defines the retry backoff strategy
type BackoffStrategy string

const (
	BackoffStrategyFixed       BackoffStrategy = "fixed"
	BackoffStrategyLinear      BackoffStrategy = "linear"
	BackoffStrategyExponential BackoffStrategy = "exponential"
)

// CircuitBreakerConfig defines circuit breaker settings
type CircuitBreakerConfig struct {
	// Enable circuit breaker
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Failure threshold before opening the circuit
	FailureThreshold int `json:"failure_threshold" yaml:"failure_threshold"`

	// Success threshold for closing the circuit
	SuccessThreshold int `json:"success_threshold" yaml:"success_threshold"`

	// Timeout before attempting to close the circuit
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// Maximum number of requests allowed in half-open state
	MaxRequests int `json:"max_requests" yaml:"max_requests"`
}

// SecurityConfig defines security settings
type SecurityConfig struct {
	// Authentication configuration
	Auth AuthConfig `json:"auth" yaml:"auth"`

	// CORS configuration
	CORS CORSConfig `json:"cors" yaml:"cors"`

	// Rate limiting configuration
	RateLimit RateLimitConfig `json:"rate_limit" yaml:"rate_limit"`

	// Input validation settings
	Validation ValidationConfig `json:"validation" yaml:"validation"`

	// Request signing configuration
	RequestSigning RequestSigningConfig `json:"request_signing" yaml:"request_signing"`
}

// AuthConfig defines authentication settings
type AuthConfig struct {
	// Authentication type
	Type AuthType `json:"type" yaml:"type"`

	// Bearer token
	BearerToken string `json:"bearer_token" yaml:"bearer_token"`

	// API key
	APIKey string `json:"api_key" yaml:"api_key"`

	// API key header name
	APIKeyHeader string `json:"api_key_header" yaml:"api_key_header"`

	// Basic authentication
	BasicAuth BasicAuthConfig `json:"basic_auth" yaml:"basic_auth"`

	// OAuth2 configuration
	OAuth2 OAuth2Config `json:"oauth2" yaml:"oauth2"`

	// JWT configuration
	JWT JWTConfig `json:"jwt" yaml:"jwt"`
}

// AuthType defines the authentication type
type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeBearer AuthType = "bearer"
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeBasic  AuthType = "basic"
	AuthTypeOAuth2 AuthType = "oauth2"
	AuthTypeJWT    AuthType = "jwt"
)

// BasicAuthConfig defines basic authentication settings
type BasicAuthConfig struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

// OAuth2Config defines OAuth2 settings
type OAuth2Config struct {
	ClientID     string   `json:"client_id" yaml:"client_id"`
	ClientSecret string   `json:"client_secret" yaml:"client_secret"`
	TokenURL     string   `json:"token_url" yaml:"token_url"`
	Scopes       []string `json:"scopes" yaml:"scopes"`
}

// JWTConfig defines JWT settings
type JWTConfig struct {
	// JWT token
	Token string `json:"token" yaml:"token"`

	// JWT signing key
	SigningKey string `json:"signing_key" yaml:"signing_key"`

	// JWT algorithm
	Algorithm string `json:"algorithm" yaml:"algorithm"`

	// JWT expiration
	Expiration time.Duration `json:"expiration" yaml:"expiration"`
}

// CORSConfig defines CORS settings
type CORSConfig struct {
	// Enable CORS
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Allowed origins
	AllowedOrigins []string `json:"allowed_origins" yaml:"allowed_origins"`

	// Allowed methods
	AllowedMethods []string `json:"allowed_methods" yaml:"allowed_methods"`

	// Allowed headers
	AllowedHeaders []string `json:"allowed_headers" yaml:"allowed_headers"`

	// Exposed headers
	ExposedHeaders []string `json:"exposed_headers" yaml:"exposed_headers"`

	// Allow credentials
	AllowCredentials bool `json:"allow_credentials" yaml:"allow_credentials"`

	// Max age
	MaxAge time.Duration `json:"max_age" yaml:"max_age"`
}

// RateLimitConfig defines rate limiting settings
type RateLimitConfig struct {
	// Enable rate limiting
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Requests per second
	RequestsPerSecond int `json:"requests_per_second" yaml:"requests_per_second"`

	// Burst size
	BurstSize int `json:"burst_size" yaml:"burst_size"`

	// Rate limit per client
	PerClient RateLimitPerClientConfig `json:"per_client" yaml:"per_client"`

	// Rate limit per endpoint
	PerEndpoint map[string]RateLimitEndpointConfig `json:"per_endpoint" yaml:"per_endpoint"`
}

// RateLimitPerClientConfig defines per-client rate limiting
type RateLimitPerClientConfig struct {
	// Enable per-client rate limiting
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Requests per second per client
	RequestsPerSecond int `json:"requests_per_second" yaml:"requests_per_second"`

	// Burst size per client
	BurstSize int `json:"burst_size" yaml:"burst_size"`

	// Client identification method
	IdentificationMethod string `json:"identification_method" yaml:"identification_method"`
}

// RateLimitEndpointConfig defines per-endpoint rate limiting
type RateLimitEndpointConfig struct {
	RequestsPerSecond int `json:"requests_per_second" yaml:"requests_per_second"`
	BurstSize         int `json:"burst_size" yaml:"burst_size"`
}

// ValidationConfig defines input validation settings
type ValidationConfig struct {
	// Enable input validation
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Maximum request size
	MaxRequestSize int64 `json:"max_request_size" yaml:"max_request_size"`

	// Maximum header size
	MaxHeaderSize int64 `json:"max_header_size" yaml:"max_header_size"`

	// Allowed content types
	AllowedContentTypes []string `json:"allowed_content_types" yaml:"allowed_content_types"`

	// Request timeout
	RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout"`

	// Validate JSON schema
	ValidateJSONSchema bool `json:"validate_json_schema" yaml:"validate_json_schema"`

	// JSON schema file path
	JSONSchemaFile string `json:"json_schema_file" yaml:"json_schema_file"`
}

// RequestSigningConfig defines request signing settings
type RequestSigningConfig struct {
	// Enable request signing
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Signing algorithm
	Algorithm string `json:"algorithm" yaml:"algorithm"`

	// Signing key
	SigningKey string `json:"signing_key" yaml:"signing_key"`

	// Headers to include in signature
	SignedHeaders []string `json:"signed_headers" yaml:"signed_headers"`

	// Signature header name
	SignatureHeader string `json:"signature_header" yaml:"signature_header"`

	// Timestamp header name
	TimestampHeader string `json:"timestamp_header" yaml:"timestamp_header"`

	// Maximum timestamp skew
	MaxTimestampSkew time.Duration `json:"max_timestamp_skew" yaml:"max_timestamp_skew"`
}

// PerformanceConfig defines performance settings
type PerformanceConfig struct {
	// Buffer configuration
	Buffering BufferingConfig `json:"buffering" yaml:"buffering"`

	// Compression configuration
	Compression CompressionConfig `json:"compression" yaml:"compression"`

	// Batching configuration
	Batching BatchingConfig `json:"batching" yaml:"batching"`

	// Caching configuration
	Caching CachingConfig `json:"caching" yaml:"caching"`

	// Connection tuning
	ConnectionTuning ConnectionTuningConfig `json:"connection_tuning" yaml:"connection_tuning"`
}

// BufferingConfig defines buffering settings
type BufferingConfig struct {
	// Enable buffering
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Read buffer size
	ReadBufferSize int `json:"read_buffer_size" yaml:"read_buffer_size"`

	// Write buffer size
	WriteBufferSize int `json:"write_buffer_size" yaml:"write_buffer_size"`

	// Event buffer size
	EventBufferSize int `json:"event_buffer_size" yaml:"event_buffer_size"`

	// Buffer flush interval
	FlushInterval time.Duration `json:"flush_interval" yaml:"flush_interval"`

	// Maximum buffer size
	MaxBufferSize int `json:"max_buffer_size" yaml:"max_buffer_size"`
}

// CompressionConfig defines compression settings
type CompressionConfig struct {
	// Enable compression
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Compression algorithm
	Algorithm CompressionAlgorithm `json:"algorithm" yaml:"algorithm"`

	// Compression level
	Level int `json:"level" yaml:"level"`

	// Minimum size for compression
	MinSize int `json:"min_size" yaml:"min_size"`

	// Content types to compress
	ContentTypes []string `json:"content_types" yaml:"content_types"`
}

// CompressionAlgorithm defines compression algorithms
type CompressionAlgorithm string

const (
	CompressionAlgorithmGzip    CompressionAlgorithm = "gzip"
	CompressionAlgorithmDeflate CompressionAlgorithm = "deflate"
	CompressionAlgorithmBrotli  CompressionAlgorithm = "brotli"
)

// BatchingConfig defines batching settings
type BatchingConfig struct {
	// Enable batching
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Batch size
	BatchSize int `json:"batch_size" yaml:"batch_size"`

	// Batch timeout
	BatchTimeout time.Duration `json:"batch_timeout" yaml:"batch_timeout"`

	// Maximum batch size
	MaxBatchSize int `json:"max_batch_size" yaml:"max_batch_size"`

	// Batch compression
	Compression bool `json:"compression" yaml:"compression"`
}

// CachingConfig defines caching settings
type CachingConfig struct {
	// Enable caching
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Cache size
	CacheSize int `json:"cache_size" yaml:"cache_size"`

	// Cache TTL
	TTL time.Duration `json:"ttl" yaml:"ttl"`

	// Cache key prefix
	KeyPrefix string `json:"key_prefix" yaml:"key_prefix"`

	// Cache eviction policy
	EvictionPolicy EvictionPolicy `json:"eviction_policy" yaml:"eviction_policy"`
}

// EvictionPolicy defines cache eviction policies
type EvictionPolicy string

const (
	EvictionPolicyLRU  EvictionPolicy = "lru"
	EvictionPolicyLFU  EvictionPolicy = "lfu"
	EvictionPolicyFIFO EvictionPolicy = "fifo"
	EvictionPolicyTTL  EvictionPolicy = "ttl"
)

// ConnectionTuningConfig defines connection tuning settings
type ConnectionTuningConfig struct {
	// TCP no delay
	TCPNoDelay bool `json:"tcp_no_delay" yaml:"tcp_no_delay"`

	// TCP keep alive
	TCPKeepAlive bool `json:"tcp_keep_alive" yaml:"tcp_keep_alive"`

	// Socket linger timeout
	SocketLinger int `json:"socket_linger" yaml:"socket_linger"`

	// Receive buffer size
	ReceiveBufferSize int `json:"receive_buffer_size" yaml:"receive_buffer_size"`

	// Send buffer size
	SendBufferSize int `json:"send_buffer_size" yaml:"send_buffer_size"`
}

// MonitoringConfig defines monitoring settings
type MonitoringConfig struct {
	// Enable monitoring
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Metrics configuration
	Metrics MetricsConfig `json:"metrics" yaml:"metrics"`

	// Logging configuration
	Logging LoggingConfig `json:"logging" yaml:"logging"`

	// Health checks configuration
	HealthChecks HealthChecksConfig `json:"health_checks" yaml:"health_checks"`

	// Tracing configuration
	Tracing TracingConfig `json:"tracing" yaml:"tracing"`

	// Alerting configuration
	Alerting AlertingConfig `json:"alerting" yaml:"alerting"`
}

// MetricsConfig defines metrics collection settings
type MetricsConfig struct {
	// Enable metrics collection
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Metrics endpoint
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Collection interval
	Interval time.Duration `json:"interval" yaml:"interval"`

	// Prometheus configuration
	Prometheus PrometheusConfig `json:"prometheus" yaml:"prometheus"`

	// Custom metrics
	Custom map[string]interface{} `json:"custom" yaml:"custom"`
}

// PrometheusConfig defines Prometheus-specific settings
type PrometheusConfig struct {
	// Enable Prometheus metrics
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Metrics namespace
	Namespace string `json:"namespace" yaml:"namespace"`

	// Metrics subsystem
	Subsystem string `json:"subsystem" yaml:"subsystem"`

	// Labels to add to all metrics
	Labels map[string]string `json:"labels" yaml:"labels"`

	// Custom registry for metrics (optional, uses default if nil)
	Registry *prometheus.Registry `json:"-" yaml:"-"`
}

// LoggingConfig defines logging settings
type LoggingConfig struct {
	// Enable logging
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Log level
	Level zapcore.Level `json:"level" yaml:"level"`

	// Log format
	Format string `json:"format" yaml:"format"`

	// Log output
	Output []string `json:"output" yaml:"output"`

	// Structured logging
	Structured bool `json:"structured" yaml:"structured"`

	// Log sampling
	Sampling LogSamplingConfig `json:"sampling" yaml:"sampling"`

	// Log rotation
	Rotation LogRotationConfig `json:"rotation" yaml:"rotation"`
}

// LogSamplingConfig defines log sampling settings
type LogSamplingConfig struct {
	// Enable sampling
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Initial sampling rate
	Initial int `json:"initial" yaml:"initial"`

	// Thereafter sampling rate
	Thereafter int `json:"thereafter" yaml:"thereafter"`
}

// LogRotationConfig defines log rotation settings
type LogRotationConfig struct {
	// Enable rotation
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Maximum file size
	MaxSize int `json:"max_size" yaml:"max_size"`

	// Maximum number of files
	MaxFiles int `json:"max_files" yaml:"max_files"`

	// Maximum age
	MaxAge time.Duration `json:"max_age" yaml:"max_age"`

	// Compress rotated files
	Compress bool `json:"compress" yaml:"compress"`
}

// HealthChecksConfig defines health check settings
type HealthChecksConfig struct {
	// Enable health checks
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Health check interval
	Interval time.Duration `json:"interval" yaml:"interval"`

	// Health check timeout
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// Health check endpoint
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Custom health checks
	Custom []HealthCheckConfig `json:"custom" yaml:"custom"`
}

// HealthCheckConfig defines a custom health check
type HealthCheckConfig struct {
	Name     string            `json:"name" yaml:"name"`
	Endpoint string            `json:"endpoint" yaml:"endpoint"`
	Method   string            `json:"method" yaml:"method"`
	Timeout  time.Duration     `json:"timeout" yaml:"timeout"`
	Headers  map[string]string `json:"headers" yaml:"headers"`
}

// TracingConfig defines distributed tracing settings
type TracingConfig struct {
	// Enable tracing
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Tracing provider
	Provider string `json:"provider" yaml:"provider"`

	// Service name
	ServiceName string `json:"service_name" yaml:"service_name"`

	// Sampling rate
	SamplingRate float64 `json:"sampling_rate" yaml:"sampling_rate"`

	// Jaeger configuration
	Jaeger JaegerConfig `json:"jaeger" yaml:"jaeger"`

	// Zipkin configuration
	Zipkin ZipkinConfig `json:"zipkin" yaml:"zipkin"`
}

// JaegerConfig defines Jaeger-specific settings
type JaegerConfig struct {
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	Agent    string `json:"agent" yaml:"agent"`
}

// ZipkinConfig defines Zipkin-specific settings
type ZipkinConfig struct {
	Endpoint string `json:"endpoint" yaml:"endpoint"`
}

// AlertingConfig defines alerting settings
type AlertingConfig struct {
	// Enable alerting
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Alert thresholds
	Thresholds AlertThresholds `json:"thresholds" yaml:"thresholds"`

	// Alert channels
	Channels []AlertChannel `json:"channels" yaml:"channels"`
}

// AlertThresholds defines various alert thresholds
type AlertThresholds struct {
	// Error rate threshold (percentage)
	ErrorRate float64 `json:"error_rate" yaml:"error_rate"`

	// Latency threshold (milliseconds)
	Latency float64 `json:"latency" yaml:"latency"`

	// Memory usage threshold (percentage)
	MemoryUsage float64 `json:"memory_usage" yaml:"memory_usage"`

	// CPU usage threshold (percentage)
	CPUUsage float64 `json:"cpu_usage" yaml:"cpu_usage"`

	// Connection count threshold
	ConnectionCount int `json:"connection_count" yaml:"connection_count"`
}

// AlertChannel defines an alert notification channel
type AlertChannel struct {
	Type   string                 `json:"type" yaml:"type"`
	Config map[string]interface{} `json:"config" yaml:"config"`
}

// Environment defines environment-specific settings
type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentStaging     Environment = "staging"
	EnvironmentProduction  Environment = "production"
)

// FeatureFlags defines feature toggles
type FeatureFlags struct {
	// Enable experimental features
	ExperimentalFeatures bool `json:"experimental_features" yaml:"experimental_features"`

	// Enable debug mode
	DebugMode bool `json:"debug_mode" yaml:"debug_mode"`

	// Enable performance profiling
	PerformanceProfiling bool `json:"performance_profiling" yaml:"performance_profiling"`

	// Enable detailed metrics
	DetailedMetrics bool `json:"detailed_metrics" yaml:"detailed_metrics"`

	// Enable request tracing
	RequestTracing bool `json:"request_tracing" yaml:"request_tracing"`
}

// ConfigBuilder provides a fluent interface for building configuration
type ConfigBuilder struct {
	config ComprehensiveConfig
}

// NewConfigBuilder creates a new configuration builder
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: DefaultComprehensiveConfig(),
	}
}

// WithConnection sets connection configuration
func (b *ConfigBuilder) WithConnection(connection ConnectionConfig) *ConfigBuilder {
	b.config.Connection = connection
	return b
}

// WithBaseURL sets the base URL
func (b *ConfigBuilder) WithBaseURL(baseURL string) *ConfigBuilder {
	b.config.Connection.BaseURL = baseURL
	return b
}

// WithEndpoint sets the endpoint path
func (b *ConfigBuilder) WithEndpoint(endpoint string) *ConfigBuilder {
	b.config.Connection.Endpoint = endpoint
	return b
}

// WithRetry sets retry configuration
func (b *ConfigBuilder) WithRetry(retry RetryConfig) *ConfigBuilder {
	b.config.Retry = retry
	return b
}

// WithMaxRetries sets the maximum number of retries
func (b *ConfigBuilder) WithMaxRetries(maxRetries int) *ConfigBuilder {
	b.config.Retry.MaxRetries = maxRetries
	return b
}

// WithSecurity sets security configuration
func (b *ConfigBuilder) WithSecurity(security SecurityConfig) *ConfigBuilder {
	b.config.Security = security
	return b
}

// WithAuth sets authentication configuration
func (b *ConfigBuilder) WithAuth(auth AuthConfig) *ConfigBuilder {
	b.config.Security.Auth = auth
	return b
}

// WithBearerToken sets bearer token authentication
func (b *ConfigBuilder) WithBearerToken(token string) *ConfigBuilder {
	b.config.Security.Auth.Type = AuthTypeBearer
	b.config.Security.Auth.BearerToken = token
	return b
}

// WithAPIKey sets API key authentication
func (b *ConfigBuilder) WithAPIKey(key, header string) *ConfigBuilder {
	b.config.Security.Auth.Type = AuthTypeAPIKey
	b.config.Security.Auth.APIKey = key
	b.config.Security.Auth.APIKeyHeader = header
	return b
}

// WithPerformance sets performance configuration
func (b *ConfigBuilder) WithPerformance(performance PerformanceConfig) *ConfigBuilder {
	b.config.Performance = performance
	return b
}

// WithCompression enables compression
func (b *ConfigBuilder) WithCompression(algorithm CompressionAlgorithm, level int) *ConfigBuilder {
	b.config.Performance.Compression.Enabled = true
	b.config.Performance.Compression.Algorithm = algorithm
	b.config.Performance.Compression.Level = level
	return b
}

// WithMonitoring sets monitoring configuration
func (b *ConfigBuilder) WithMonitoring(monitoring MonitoringConfig) *ConfigBuilder {
	b.config.Monitoring = monitoring
	return b
}

// WithMetrics enables metrics collection
func (b *ConfigBuilder) WithMetrics(enabled bool, interval time.Duration) *ConfigBuilder {
	b.config.Monitoring.Metrics.Enabled = enabled
	b.config.Monitoring.Metrics.Interval = interval
	return b
}

// WithEnvironment sets the environment
func (b *ConfigBuilder) WithEnvironment(env Environment) *ConfigBuilder {
	b.config.Environment = env
	return b
}

// WithFeatureFlags sets feature flags
func (b *ConfigBuilder) WithFeatureFlags(flags FeatureFlags) *ConfigBuilder {
	b.config.Features = flags
	return b
}

// Build returns the configured ComprehensiveConfig
func (b *ConfigBuilder) Build() ComprehensiveConfig {
	return b.config
}

// ConfigLoader provides methods to load configuration from various sources
type ConfigLoader struct{}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{}
}

// LoadFromFile loads configuration from a JSON file
func (l *ConfigLoader) LoadFromFile(filename string) (ComprehensiveConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return ComprehensiveConfig{}, &core.ConfigError{
			Field: "file",
			Value: filename,
			Err:   fmt.Errorf("failed to open config file: %w", err),
		}
	}
	defer file.Close()

	return l.LoadFromReader(file)
}

// LoadFromReader loads configuration from a reader
func (l *ConfigLoader) LoadFromReader(reader io.Reader) (ComprehensiveConfig, error) {
	var config ComprehensiveConfig

	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&config); err != nil {
		return ComprehensiveConfig{}, &core.ConfigError{
			Field: "json",
			Value: nil,
			Err:   fmt.Errorf("failed to decode config: %w", err),
		}
	}

	return config, nil
}

// LoadFromEnv loads configuration from environment variables
func (l *ConfigLoader) LoadFromEnv() ComprehensiveConfig {
	config := DefaultComprehensiveConfig()

	// Connection configuration
	if val := os.Getenv("SSE_BASE_URL"); val != "" {
		config.Connection.BaseURL = val
	}
	if val := os.Getenv("SSE_ENDPOINT"); val != "" {
		config.Connection.Endpoint = val
	}
	if val := os.Getenv("SSE_CONNECT_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.Connection.ConnectTimeout = duration
		}
	}
	if val := os.Getenv("SSE_READ_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.Connection.ReadTimeout = duration
		}
	}
	if val := os.Getenv("SSE_WRITE_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.Connection.WriteTimeout = duration
		}
	}

	// TLS configuration
	if val := os.Getenv("SSE_TLS_ENABLED"); val != "" {
		config.Connection.TLS.Enabled = val == "true"
	}
	if val := os.Getenv("SSE_TLS_INSECURE_SKIP_VERIFY"); val != "" {
		config.Connection.TLS.InsecureSkipVerify = val == "true"
	}
	if val := os.Getenv("SSE_TLS_CERT_FILE"); val != "" {
		config.Connection.TLS.CertFile = val
	}
	if val := os.Getenv("SSE_TLS_KEY_FILE"); val != "" {
		config.Connection.TLS.KeyFile = val
	}
	if val := os.Getenv("SSE_TLS_CA_FILE"); val != "" {
		config.Connection.TLS.CAFile = val
	}

	// Retry configuration
	if val := os.Getenv("SSE_RETRY_ENABLED"); val != "" {
		config.Retry.Enabled = val == "true"
	}
	if val := os.Getenv("SSE_RETRY_MAX_RETRIES"); val != "" {
		if retries, err := strconv.Atoi(val); err == nil {
			config.Retry.MaxRetries = retries
		}
	}
	if val := os.Getenv("SSE_RETRY_INITIAL_DELAY"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.Retry.InitialDelay = duration
		}
	}
	if val := os.Getenv("SSE_RETRY_MAX_DELAY"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.Retry.MaxDelay = duration
		}
	}
	if val := os.Getenv("SSE_RETRY_BACKOFF_STRATEGY"); val != "" {
		config.Retry.BackoffStrategy = BackoffStrategy(val)
	}

	// Authentication configuration
	if val := os.Getenv("SSE_AUTH_TYPE"); val != "" {
		config.Security.Auth.Type = AuthType(val)
	}
	if val := os.Getenv("SSE_AUTH_BEARER_TOKEN"); val != "" {
		config.Security.Auth.BearerToken = val
	}
	if val := os.Getenv("SSE_AUTH_API_KEY"); val != "" {
		config.Security.Auth.APIKey = val
	}
	if val := os.Getenv("SSE_AUTH_API_KEY_HEADER"); val != "" {
		config.Security.Auth.APIKeyHeader = val
	}
	if val := os.Getenv("SSE_AUTH_BASIC_USERNAME"); val != "" {
		config.Security.Auth.BasicAuth.Username = val
	}
	if val := os.Getenv("SSE_AUTH_BASIC_PASSWORD"); val != "" {
		config.Security.Auth.BasicAuth.Password = val
	}

	// Rate limiting configuration
	if val := os.Getenv("SSE_RATE_LIMIT_ENABLED"); val != "" {
		config.Security.RateLimit.Enabled = val == "true"
	}
	if val := os.Getenv("SSE_RATE_LIMIT_REQUESTS_PER_SECOND"); val != "" {
		if rps, err := strconv.Atoi(val); err == nil {
			config.Security.RateLimit.RequestsPerSecond = rps
		}
	}
	if val := os.Getenv("SSE_RATE_LIMIT_BURST_SIZE"); val != "" {
		if burst, err := strconv.Atoi(val); err == nil {
			config.Security.RateLimit.BurstSize = burst
		}
	}

	// Performance configuration
	if val := os.Getenv("SSE_COMPRESSION_ENABLED"); val != "" {
		config.Performance.Compression.Enabled = val == "true"
	}
	if val := os.Getenv("SSE_COMPRESSION_ALGORITHM"); val != "" {
		config.Performance.Compression.Algorithm = CompressionAlgorithm(val)
	}
	if val := os.Getenv("SSE_COMPRESSION_LEVEL"); val != "" {
		if level, err := strconv.Atoi(val); err == nil {
			config.Performance.Compression.Level = level
		}
	}

	// Buffering configuration
	if val := os.Getenv("SSE_BUFFERING_ENABLED"); val != "" {
		config.Performance.Buffering.Enabled = val == "true"
	}
	if val := os.Getenv("SSE_BUFFERING_READ_BUFFER_SIZE"); val != "" {
		if size, err := strconv.Atoi(val); err == nil {
			config.Performance.Buffering.ReadBufferSize = size
		}
	}
	if val := os.Getenv("SSE_BUFFERING_WRITE_BUFFER_SIZE"); val != "" {
		if size, err := strconv.Atoi(val); err == nil {
			config.Performance.Buffering.WriteBufferSize = size
		}
	}

	// Monitoring configuration
	if val := os.Getenv("SSE_MONITORING_ENABLED"); val != "" {
		config.Monitoring.Enabled = val == "true"
	}
	if val := os.Getenv("SSE_MONITORING_METRICS_ENABLED"); val != "" {
		config.Monitoring.Metrics.Enabled = val == "true"
	}
	if val := os.Getenv("SSE_MONITORING_METRICS_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.Monitoring.Metrics.Interval = duration
		}
	}
	if val := os.Getenv("SSE_MONITORING_PROMETHEUS_ENABLED"); val != "" {
		config.Monitoring.Metrics.Prometheus.Enabled = val == "true"
	}
	if val := os.Getenv("SSE_MONITORING_PROMETHEUS_NAMESPACE"); val != "" {
		config.Monitoring.Metrics.Prometheus.Namespace = val
	}
	if val := os.Getenv("SSE_MONITORING_PROMETHEUS_SUBSYSTEM"); val != "" {
		config.Monitoring.Metrics.Prometheus.Subsystem = val
	}

	// Logging configuration
	if val := os.Getenv("SSE_LOGGING_ENABLED"); val != "" {
		config.Monitoring.Logging.Enabled = val == "true"
	}
	if val := os.Getenv("SSE_LOGGING_LEVEL"); val != "" {
		if level, err := zapcore.ParseLevel(val); err == nil {
			config.Monitoring.Logging.Level = level
		}
	}
	if val := os.Getenv("SSE_LOGGING_FORMAT"); val != "" {
		config.Monitoring.Logging.Format = val
	}
	if val := os.Getenv("SSE_LOGGING_STRUCTURED"); val != "" {
		config.Monitoring.Logging.Structured = val == "true"
	}

	// Environment
	if val := os.Getenv("SSE_ENVIRONMENT"); val != "" {
		config.Environment = Environment(val)
	}

	// Feature flags
	if val := os.Getenv("SSE_FEATURE_DEBUG_MODE"); val != "" {
		config.Features.DebugMode = val == "true"
	}
	if val := os.Getenv("SSE_FEATURE_EXPERIMENTAL_FEATURES"); val != "" {
		config.Features.ExperimentalFeatures = val == "true"
	}
	if val := os.Getenv("SSE_FEATURE_PERFORMANCE_PROFILING"); val != "" {
		config.Features.PerformanceProfiling = val == "true"
	}

	return config
}

// DefaultComprehensiveConfig returns the default comprehensive configuration
func DefaultComprehensiveConfig() ComprehensiveConfig {
	return ComprehensiveConfig{
		Connection: ConnectionConfig{
			BaseURL:        "http://localhost:8080",
			Endpoint:       "/events",
			ConnectTimeout: 30 * time.Second,
			ReadTimeout:    60 * time.Second,
			WriteTimeout:   30 * time.Second,
			KeepAlive: KeepAliveConfig{
				Enabled:     true,
				Interval:    30 * time.Second,
				IdleTimeout: 90 * time.Second,
				ProbeCount:  9,
			},
			TLS: TLSConfig{
				Enabled:            false,
				InsecureSkipVerify: false,
				MinVersion:         tls.VersionTLS12,
				MaxVersion:         tls.VersionTLS13,
			},
			HTTPClient: HTTPClientConfig{
				UserAgent:             "ag-ui-go-sdk/1.0",
				Headers:               make(map[string]string),
				DisableHTTP2:          false,
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   10,
				MaxConnsPerHost:       0,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
			ConnectionPool: ConnectionPoolConfig{
				MaxConnections:      100,
				MaxIdleConnections:  10,
				ConnectionLifetime:  30 * time.Minute,
				IdleTimeout:         90 * time.Second,
				HealthCheckInterval: 30 * time.Second,
			},
		},
		Retry: RetryConfig{
			Enabled:            true,
			MaxRetries:         3,
			InitialDelay:       100 * time.Millisecond,
			MaxDelay:           30 * time.Second,
			BackoffStrategy:    BackoffStrategyExponential,
			BackoffMultiplier:  2.0,
			JitterFactor:       0.1,
			RetryOnStatusCodes: []int{500, 502, 503, 504},
			RetryOnErrors:      []string{"connection", "timeout", "network"},
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 5,
				SuccessThreshold: 3,
				Timeout:          60 * time.Second,
				MaxRequests:      10,
			},
		},
		Security: SecurityConfig{
			Auth: AuthConfig{
				Type:         AuthTypeNone,
				APIKeyHeader: "X-API-Key",
			},
			CORS: CORSConfig{
				Enabled:          false,
				AllowedOrigins:   []string{"https://localhost:3000", "https://localhost:8080"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With", "X-CSRF-Token"},
				ExposedHeaders:   []string{},
				AllowCredentials: true,
				MaxAge:           24 * time.Hour,
			},
			RateLimit: RateLimitConfig{
				Enabled:           false,
				RequestsPerSecond: 100,
				BurstSize:         200,
				PerClient: RateLimitPerClientConfig{
					Enabled:              false,
					RequestsPerSecond:    10,
					BurstSize:            20,
					IdentificationMethod: "ip",
				},
				PerEndpoint: make(map[string]RateLimitEndpointConfig),
			},
			Validation: ValidationConfig{
				Enabled:             true,
				MaxRequestSize:      10 * 1024 * 1024, // 10MB
				MaxHeaderSize:       1024 * 1024,      // 1MB
				AllowedContentTypes: []string{"application/json", "text/plain"},
				RequestTimeout:      30 * time.Second,
				ValidateJSONSchema:  false,
			},
			RequestSigning: RequestSigningConfig{
				Enabled:          false,
				Algorithm:        "HMAC-SHA256",
				SignedHeaders:    []string{"host", "date", "content-type"},
				SignatureHeader:  "X-Signature",
				TimestampHeader:  "X-Timestamp",
				MaxTimestampSkew: 5 * time.Minute,
			},
		},
		Performance: PerformanceConfig{
			Buffering: BufferingConfig{
				Enabled:         true,
				ReadBufferSize:  8192,
				WriteBufferSize: 8192,
				EventBufferSize: 1000,
				FlushInterval:   100 * time.Millisecond,
				MaxBufferSize:   1024 * 1024, // 1MB
			},
			Compression: CompressionConfig{
				Enabled:      false,
				Algorithm:    CompressionAlgorithmGzip,
				Level:        6,
				MinSize:      1024,
				ContentTypes: []string{"application/json", "text/plain"},
			},
			Batching: BatchingConfig{
				Enabled:      false,
				BatchSize:    100,
				BatchTimeout: 100 * time.Millisecond,
				MaxBatchSize: 1000,
				Compression:  false,
			},
			Caching: CachingConfig{
				Enabled:        false,
				CacheSize:      1000,
				TTL:            5 * time.Minute,
				KeyPrefix:      "sse:",
				EvictionPolicy: EvictionPolicyLRU,
			},
			ConnectionTuning: ConnectionTuningConfig{
				TCPNoDelay:        true,
				TCPKeepAlive:      true,
				SocketLinger:      -1,
				ReceiveBufferSize: 65536,
				SendBufferSize:    65536,
			},
		},
		Monitoring: MonitoringConfig{
			Enabled: true,
			Metrics: MetricsConfig{
				Enabled:  true,
				Endpoint: "/metrics",
				Interval: 30 * time.Second,
				Prometheus: PrometheusConfig{
					Enabled:   true,
					Namespace: "sse",
					Subsystem: "transport",
					Labels:    make(map[string]string),
				},
				Custom: make(map[string]interface{}),
			},
			Logging: LoggingConfig{
				Enabled:    true,
				Level:      zapcore.InfoLevel,
				Format:     "json",
				Output:     []string{"stdout"},
				Structured: true,
				Sampling: LogSamplingConfig{
					Enabled:    true,
					Initial:    100,
					Thereafter: 100,
				},
				Rotation: LogRotationConfig{
					Enabled:  false,
					MaxSize:  100, // MB
					MaxFiles: 10,
					MaxAge:   30 * 24 * time.Hour,
					Compress: true,
				},
			},
			HealthChecks: HealthChecksConfig{
				Enabled:  true,
				Interval: 30 * time.Second,
				Timeout:  5 * time.Second,
				Endpoint: "/health",
				Custom:   []HealthCheckConfig{},
			},
			Tracing: TracingConfig{
				Enabled:      false,
				Provider:     "jaeger",
				ServiceName:  "sse-transport",
				SamplingRate: 0.1,
				Jaeger: JaegerConfig{
					Endpoint: "http://localhost:14268/api/traces",
					Agent:    "localhost:6831",
				},
				Zipkin: ZipkinConfig{
					Endpoint: "http://localhost:9411/api/v2/spans",
				},
			},
			Alerting: AlertingConfig{
				Enabled: false,
				Thresholds: AlertThresholds{
					ErrorRate:       5.0,
					Latency:         1000,
					MemoryUsage:     80,
					CPUUsage:        80,
					ConnectionCount: 1000,
				},
				Channels: []AlertChannel{},
			},
		},
		Environment: EnvironmentDevelopment,
		Features: FeatureFlags{
			ExperimentalFeatures: false,
			DebugMode:            false,
			PerformanceProfiling: false,
			DetailedMetrics:      false,
			RequestTracing:       false,
		},
	}
}

// DevelopmentConfig returns a development-optimized configuration
func DevelopmentConfig() ComprehensiveConfig {
	config := DefaultComprehensiveConfig()
	config.Environment = EnvironmentDevelopment
	config.Features.DebugMode = true
	config.Features.DetailedMetrics = true
	config.Features.RequestTracing = true
	config.Monitoring.Logging.Level = zapcore.DebugLevel
	config.Monitoring.Logging.Format = "console"
	config.Monitoring.Tracing.Enabled = true
	config.Monitoring.Tracing.SamplingRate = 1.0
	config.Retry.MaxRetries = 1
	config.Connection.ConnectTimeout = 5 * time.Second
	config.Connection.ReadTimeout = 10 * time.Second
	config.Connection.WriteTimeout = 5 * time.Second
	return config
}

// ProductionConfig returns a production-optimized configuration
func ProductionConfig() ComprehensiveConfig {
	config := DefaultComprehensiveConfig()
	config.Environment = EnvironmentProduction
	config.Features.DebugMode = false
	config.Features.DetailedMetrics = false
	config.Features.RequestTracing = false
	config.Monitoring.Logging.Level = zapcore.InfoLevel
	config.Monitoring.Logging.Format = "json"
	config.Monitoring.Logging.Structured = true
	config.Monitoring.Tracing.Enabled = true
	config.Monitoring.Tracing.SamplingRate = 0.01
	config.Monitoring.Alerting.Enabled = true
	config.Performance.Compression.Enabled = true
	config.Performance.Caching.Enabled = true
	config.Security.RateLimit.Enabled = true
	config.Security.Validation.Enabled = true
	config.Retry.CircuitBreaker.Enabled = true
	return config
}

// StagingConfig returns a staging-optimized configuration
func StagingConfig() ComprehensiveConfig {
	config := ProductionConfig()
	config.Environment = EnvironmentStaging
	config.Features.DebugMode = true
	config.Features.DetailedMetrics = true
	config.Monitoring.Logging.Level = zapcore.DebugLevel
	config.Monitoring.Tracing.SamplingRate = 0.1
	return config
}

// Validate validates the configuration
func (c *ComprehensiveConfig) Validate() error {
	// Validate connection configuration
	if err := c.validateConnection(); err != nil {
		return err
	}

	// Validate retry configuration
	if err := c.validateRetry(); err != nil {
		return err
	}

	// Validate security configuration
	if err := c.validateSecurity(); err != nil {
		return err
	}

	// Validate performance configuration
	if err := c.validatePerformance(); err != nil {
		return err
	}

	// Validate monitoring configuration
	if err := c.validateMonitoring(); err != nil {
		return err
	}

	return nil
}

// validateConnection validates connection configuration
func (c *ComprehensiveConfig) validateConnection() error {
	// Validate base URL
	if c.Connection.BaseURL == "" {
		return &core.ConfigError{
			Field: "connection.base_url",
			Value: c.Connection.BaseURL,
			Err:   errors.New("base URL is required"),
		}
	}

	// Parse and validate base URL
	if _, err := url.Parse(c.Connection.BaseURL); err != nil {
		return &core.ConfigError{
			Field: "connection.base_url",
			Value: c.Connection.BaseURL,
			Err:   fmt.Errorf("invalid base URL: %w", err),
		}
	}

	// Validate timeouts
	if c.Connection.ConnectTimeout <= 0 {
		return &core.ConfigError{
			Field: "connection.connect_timeout",
			Value: c.Connection.ConnectTimeout,
			Err:   errors.New("connect timeout must be positive"),
		}
	}

	if c.Connection.ReadTimeout <= 0 {
		return &core.ConfigError{
			Field: "connection.read_timeout",
			Value: c.Connection.ReadTimeout,
			Err:   errors.New("read timeout must be positive"),
		}
	}

	if c.Connection.WriteTimeout <= 0 {
		return &core.ConfigError{
			Field: "connection.write_timeout",
			Value: c.Connection.WriteTimeout,
			Err:   errors.New("write timeout must be positive"),
		}
	}

	// Validate keep-alive configuration
	if c.Connection.KeepAlive.Enabled {
		if c.Connection.KeepAlive.Interval <= 0 {
			return &core.ConfigError{
				Field: "connection.keep_alive.interval",
				Value: c.Connection.KeepAlive.Interval,
				Err:   errors.New("keep-alive interval must be positive"),
			}
		}

		if c.Connection.KeepAlive.IdleTimeout <= 0 {
			return &core.ConfigError{
				Field: "connection.keep_alive.idle_timeout",
				Value: c.Connection.KeepAlive.IdleTimeout,
				Err:   errors.New("keep-alive idle timeout must be positive"),
			}
		}

		if c.Connection.KeepAlive.ProbeCount <= 0 {
			return &core.ConfigError{
				Field: "connection.keep_alive.probe_count",
				Value: c.Connection.KeepAlive.ProbeCount,
				Err:   errors.New("keep-alive probe count must be positive"),
			}
		}
	}

	// Validate TLS configuration
	if c.Connection.TLS.Enabled {
		if c.Connection.TLS.MinVersion > c.Connection.TLS.MaxVersion {
			return &core.ConfigError{
				Field: "connection.tls.min_version",
				Value: c.Connection.TLS.MinVersion,
				Err:   errors.New("TLS min version cannot be greater than max version"),
			}
		}
	}

	// Validate HTTP client configuration
	if c.Connection.HTTPClient.MaxIdleConns < 0 {
		return &core.ConfigError{
			Field: "connection.http_client.max_idle_conns",
			Value: c.Connection.HTTPClient.MaxIdleConns,
			Err:   errors.New("max idle connections cannot be negative"),
		}
	}

	if c.Connection.HTTPClient.MaxIdleConnsPerHost < 0 {
		return &core.ConfigError{
			Field: "connection.http_client.max_idle_conns_per_host",
			Value: c.Connection.HTTPClient.MaxIdleConnsPerHost,
			Err:   errors.New("max idle connections per host cannot be negative"),
		}
	}

	// Validate connection pool configuration
	if c.Connection.ConnectionPool.MaxConnections <= 0 {
		return &core.ConfigError{
			Field: "connection.connection_pool.max_connections",
			Value: c.Connection.ConnectionPool.MaxConnections,
			Err:   errors.New("max connections must be positive"),
		}
	}

	if c.Connection.ConnectionPool.MaxIdleConnections < 0 {
		return &core.ConfigError{
			Field: "connection.connection_pool.max_idle_connections",
			Value: c.Connection.ConnectionPool.MaxIdleConnections,
			Err:   errors.New("max idle connections cannot be negative"),
		}
	}

	if c.Connection.ConnectionPool.MaxIdleConnections > c.Connection.ConnectionPool.MaxConnections {
		return &core.ConfigError{
			Field: "connection.connection_pool.max_idle_connections",
			Value: c.Connection.ConnectionPool.MaxIdleConnections,
			Err:   errors.New("max idle connections cannot exceed max connections"),
		}
	}

	return nil
}

// validateRetry validates retry configuration
func (c *ComprehensiveConfig) validateRetry() error {
	if !c.Retry.Enabled {
		return nil
	}

	if c.Retry.MaxRetries < 0 {
		return &core.ConfigError{
			Field: "retry.max_retries",
			Value: c.Retry.MaxRetries,
			Err:   errors.New("max retries cannot be negative"),
		}
	}

	if c.Retry.InitialDelay <= 0 {
		return &core.ConfigError{
			Field: "retry.initial_delay",
			Value: c.Retry.InitialDelay,
			Err:   errors.New("initial delay must be positive"),
		}
	}

	if c.Retry.MaxDelay <= 0 {
		return &core.ConfigError{
			Field: "retry.max_delay",
			Value: c.Retry.MaxDelay,
			Err:   errors.New("max delay must be positive"),
		}
	}

	if c.Retry.InitialDelay > c.Retry.MaxDelay {
		return &core.ConfigError{
			Field: "retry.initial_delay",
			Value: c.Retry.InitialDelay,
			Err:   errors.New("initial delay cannot exceed max delay"),
		}
	}

	// Validate backoff strategy
	switch c.Retry.BackoffStrategy {
	case BackoffStrategyFixed, BackoffStrategyLinear, BackoffStrategyExponential:
		// Valid strategies
	default:
		return &core.ConfigError{
			Field: "retry.backoff_strategy",
			Value: c.Retry.BackoffStrategy,
			Err:   errors.New("invalid backoff strategy"),
		}
	}

	if c.Retry.BackoffMultiplier <= 0 {
		return &core.ConfigError{
			Field: "retry.backoff_multiplier",
			Value: c.Retry.BackoffMultiplier,
			Err:   errors.New("backoff multiplier must be positive"),
		}
	}

	if c.Retry.JitterFactor < 0 || c.Retry.JitterFactor > 1 {
		return &core.ConfigError{
			Field: "retry.jitter_factor",
			Value: c.Retry.JitterFactor,
			Err:   errors.New("jitter factor must be between 0 and 1"),
		}
	}

	// Validate circuit breaker configuration
	if c.Retry.CircuitBreaker.Enabled {
		if c.Retry.CircuitBreaker.FailureThreshold <= 0 {
			return &core.ConfigError{
				Field: "retry.circuit_breaker.failure_threshold",
				Value: c.Retry.CircuitBreaker.FailureThreshold,
				Err:   errors.New("failure threshold must be positive"),
			}
		}

		if c.Retry.CircuitBreaker.SuccessThreshold <= 0 {
			return &core.ConfigError{
				Field: "retry.circuit_breaker.success_threshold",
				Value: c.Retry.CircuitBreaker.SuccessThreshold,
				Err:   errors.New("success threshold must be positive"),
			}
		}

		if c.Retry.CircuitBreaker.Timeout <= 0 {
			return &core.ConfigError{
				Field: "retry.circuit_breaker.timeout",
				Value: c.Retry.CircuitBreaker.Timeout,
				Err:   errors.New("circuit breaker timeout must be positive"),
			}
		}

		if c.Retry.CircuitBreaker.MaxRequests <= 0 {
			return &core.ConfigError{
				Field: "retry.circuit_breaker.max_requests",
				Value: c.Retry.CircuitBreaker.MaxRequests,
				Err:   errors.New("max requests must be positive"),
			}
		}
	}

	return nil
}

// validateSecurity validates security configuration
func (c *ComprehensiveConfig) validateSecurity() error {
	// Validate authentication configuration
	switch c.Security.Auth.Type {
	case AuthTypeNone:
		// No validation needed
	case AuthTypeBearer:
		if c.Security.Auth.BearerToken == "" {
			return &core.ConfigError{
				Field: "security.auth.bearer_token",
				Value: c.Security.Auth.BearerToken,
				Err:   errors.New("bearer token is required for bearer authentication"),
			}
		}
	case AuthTypeAPIKey:
		if c.Security.Auth.APIKey == "" {
			return &core.ConfigError{
				Field: "security.auth.api_key",
				Value: c.Security.Auth.APIKey,
				Err:   errors.New("API key is required for API key authentication"),
			}
		}
		if c.Security.Auth.APIKeyHeader == "" {
			return &core.ConfigError{
				Field: "security.auth.api_key_header",
				Value: c.Security.Auth.APIKeyHeader,
				Err:   errors.New("API key header is required for API key authentication"),
			}
		}
	case AuthTypeBasic:
		if c.Security.Auth.BasicAuth.Username == "" {
			return &core.ConfigError{
				Field: "security.auth.basic_auth.username",
				Value: c.Security.Auth.BasicAuth.Username,
				Err:   errors.New("username is required for basic authentication"),
			}
		}
		if c.Security.Auth.BasicAuth.Password == "" {
			return &core.ConfigError{
				Field: "security.auth.basic_auth.password",
				Value: c.Security.Auth.BasicAuth.Password,
				Err:   errors.New("password is required for basic authentication"),
			}
		}
	case AuthTypeOAuth2:
		if c.Security.Auth.OAuth2.ClientID == "" {
			return &core.ConfigError{
				Field: "security.auth.oauth2.client_id",
				Value: c.Security.Auth.OAuth2.ClientID,
				Err:   errors.New("client ID is required for OAuth2 authentication"),
			}
		}
		if c.Security.Auth.OAuth2.ClientSecret == "" {
			return &core.ConfigError{
				Field: "security.auth.oauth2.client_secret",
				Value: c.Security.Auth.OAuth2.ClientSecret,
				Err:   errors.New("client secret is required for OAuth2 authentication"),
			}
		}
		if c.Security.Auth.OAuth2.TokenURL == "" {
			return &core.ConfigError{
				Field: "security.auth.oauth2.token_url",
				Value: c.Security.Auth.OAuth2.TokenURL,
				Err:   errors.New("token URL is required for OAuth2 authentication"),
			}
		}
	case AuthTypeJWT:
		if c.Security.Auth.JWT.Token == "" && c.Security.Auth.JWT.SigningKey == "" {
			return &core.ConfigError{
				Field: "security.auth.jwt",
				Value: nil,
				Err:   errors.New("either token or signing key is required for JWT authentication"),
			}
		}
	default:
		return &core.ConfigError{
			Field: "security.auth.type",
			Value: c.Security.Auth.Type,
			Err:   errors.New("invalid authentication type"),
		}
	}

	// Validate rate limiting configuration
	if c.Security.RateLimit.Enabled {
		if c.Security.RateLimit.RequestsPerSecond <= 0 {
			return &core.ConfigError{
				Field: "security.rate_limit.requests_per_second",
				Value: c.Security.RateLimit.RequestsPerSecond,
				Err:   errors.New("requests per second must be positive"),
			}
		}

		if c.Security.RateLimit.BurstSize <= 0 {
			return &core.ConfigError{
				Field: "security.rate_limit.burst_size",
				Value: c.Security.RateLimit.BurstSize,
				Err:   errors.New("burst size must be positive"),
			}
		}

		if c.Security.RateLimit.PerClient.Enabled {
			if c.Security.RateLimit.PerClient.RequestsPerSecond <= 0 {
				return &core.ConfigError{
					Field: "security.rate_limit.per_client.requests_per_second",
					Value: c.Security.RateLimit.PerClient.RequestsPerSecond,
					Err:   errors.New("per-client requests per second must be positive"),
				}
			}

			if c.Security.RateLimit.PerClient.BurstSize <= 0 {
				return &core.ConfigError{
					Field: "security.rate_limit.per_client.burst_size",
					Value: c.Security.RateLimit.PerClient.BurstSize,
					Err:   errors.New("per-client burst size must be positive"),
				}
			}
		}
	}

	// Validate validation configuration
	if c.Security.Validation.Enabled {
		if c.Security.Validation.MaxRequestSize <= 0 {
			return &core.ConfigError{
				Field: "security.validation.max_request_size",
				Value: c.Security.Validation.MaxRequestSize,
				Err:   errors.New("max request size must be positive"),
			}
		}

		if c.Security.Validation.MaxHeaderSize <= 0 {
			return &core.ConfigError{
				Field: "security.validation.max_header_size",
				Value: c.Security.Validation.MaxHeaderSize,
				Err:   errors.New("max header size must be positive"),
			}
		}

		if c.Security.Validation.RequestTimeout <= 0 {
			return &core.ConfigError{
				Field: "security.validation.request_timeout",
				Value: c.Security.Validation.RequestTimeout,
				Err:   errors.New("request timeout must be positive"),
			}
		}
	}

	return nil
}

// validatePerformance validates performance configuration
func (c *ComprehensiveConfig) validatePerformance() error {
	// Validate buffering configuration
	if c.Performance.Buffering.Enabled {
		if c.Performance.Buffering.ReadBufferSize <= 0 {
			return &core.ConfigError{
				Field: "performance.buffering.read_buffer_size",
				Value: c.Performance.Buffering.ReadBufferSize,
				Err:   errors.New("read buffer size must be positive"),
			}
		}

		if c.Performance.Buffering.WriteBufferSize <= 0 {
			return &core.ConfigError{
				Field: "performance.buffering.write_buffer_size",
				Value: c.Performance.Buffering.WriteBufferSize,
				Err:   errors.New("write buffer size must be positive"),
			}
		}

		if c.Performance.Buffering.EventBufferSize <= 0 {
			return &core.ConfigError{
				Field: "performance.buffering.event_buffer_size",
				Value: c.Performance.Buffering.EventBufferSize,
				Err:   errors.New("event buffer size must be positive"),
			}
		}

		if c.Performance.Buffering.MaxBufferSize <= 0 {
			return &core.ConfigError{
				Field: "performance.buffering.max_buffer_size",
				Value: c.Performance.Buffering.MaxBufferSize,
				Err:   errors.New("max buffer size must be positive"),
			}
		}

		if c.Performance.Buffering.FlushInterval <= 0 {
			return &core.ConfigError{
				Field: "performance.buffering.flush_interval",
				Value: c.Performance.Buffering.FlushInterval,
				Err:   errors.New("flush interval must be positive"),
			}
		}
	}

	// Validate compression configuration
	if c.Performance.Compression.Enabled {
		// Validate compression algorithm
		switch c.Performance.Compression.Algorithm {
		case CompressionAlgorithmGzip, CompressionAlgorithmDeflate, CompressionAlgorithmBrotli:
			// Valid algorithms
		default:
			return &core.ConfigError{
				Field: "performance.compression.algorithm",
				Value: c.Performance.Compression.Algorithm,
				Err:   errors.New("invalid compression algorithm"),
			}
		}

		if c.Performance.Compression.Level < 0 || c.Performance.Compression.Level > 9 {
			return &core.ConfigError{
				Field: "performance.compression.level",
				Value: c.Performance.Compression.Level,
				Err:   errors.New("compression level must be between 0 and 9"),
			}
		}

		if c.Performance.Compression.MinSize < 0 {
			return &core.ConfigError{
				Field: "performance.compression.min_size",
				Value: c.Performance.Compression.MinSize,
				Err:   errors.New("compression min size cannot be negative"),
			}
		}
	}

	// Validate batching configuration
	if c.Performance.Batching.Enabled {
		if c.Performance.Batching.BatchSize <= 0 {
			return &core.ConfigError{
				Field: "performance.batching.batch_size",
				Value: c.Performance.Batching.BatchSize,
				Err:   errors.New("batch size must be positive"),
			}
		}

		if c.Performance.Batching.MaxBatchSize <= 0 {
			return &core.ConfigError{
				Field: "performance.batching.max_batch_size",
				Value: c.Performance.Batching.MaxBatchSize,
				Err:   errors.New("max batch size must be positive"),
			}
		}

		if c.Performance.Batching.BatchSize > c.Performance.Batching.MaxBatchSize {
			return &core.ConfigError{
				Field: "performance.batching.batch_size",
				Value: c.Performance.Batching.BatchSize,
				Err:   errors.New("batch size cannot exceed max batch size"),
			}
		}

		if c.Performance.Batching.BatchTimeout <= 0 {
			return &core.ConfigError{
				Field: "performance.batching.batch_timeout",
				Value: c.Performance.Batching.BatchTimeout,
				Err:   errors.New("batch timeout must be positive"),
			}
		}
	}

	// Validate caching configuration
	if c.Performance.Caching.Enabled {
		if c.Performance.Caching.CacheSize <= 0 {
			return &core.ConfigError{
				Field: "performance.caching.cache_size",
				Value: c.Performance.Caching.CacheSize,
				Err:   errors.New("cache size must be positive"),
			}
		}

		if c.Performance.Caching.TTL <= 0 {
			return &core.ConfigError{
				Field: "performance.caching.ttl",
				Value: c.Performance.Caching.TTL,
				Err:   errors.New("cache TTL must be positive"),
			}
		}

		// Validate eviction policy
		switch c.Performance.Caching.EvictionPolicy {
		case EvictionPolicyLRU, EvictionPolicyLFU, EvictionPolicyFIFO, EvictionPolicyTTL:
			// Valid policies
		default:
			return &core.ConfigError{
				Field: "performance.caching.eviction_policy",
				Value: c.Performance.Caching.EvictionPolicy,
				Err:   errors.New("invalid eviction policy"),
			}
		}
	}

	return nil
}

// validateMonitoring validates monitoring configuration
func (c *ComprehensiveConfig) validateMonitoring() error {
	if !c.Monitoring.Enabled {
		return nil
	}

	// Validate metrics configuration
	if c.Monitoring.Metrics.Enabled {
		if c.Monitoring.Metrics.Interval <= 0 {
			return &core.ConfigError{
				Field: "monitoring.metrics.interval",
				Value: c.Monitoring.Metrics.Interval,
				Err:   errors.New("metrics interval must be positive"),
			}
		}
	}

	// Validate health checks configuration
	if c.Monitoring.HealthChecks.Enabled {
		if c.Monitoring.HealthChecks.Interval <= 0 {
			return &core.ConfigError{
				Field: "monitoring.health_checks.interval",
				Value: c.Monitoring.HealthChecks.Interval,
				Err:   errors.New("health check interval must be positive"),
			}
		}

		if c.Monitoring.HealthChecks.Timeout <= 0 {
			return &core.ConfigError{
				Field: "monitoring.health_checks.timeout",
				Value: c.Monitoring.HealthChecks.Timeout,
				Err:   errors.New("health check timeout must be positive"),
			}
		}

		// Validate custom health checks
		for i, check := range c.Monitoring.HealthChecks.Custom {
			if check.Name == "" {
				return &core.ConfigError{
					Field: fmt.Sprintf("monitoring.health_checks.custom[%d].name", i),
					Value: check.Name,
					Err:   errors.New("health check name is required"),
				}
			}

			if check.Endpoint == "" {
				return &core.ConfigError{
					Field: fmt.Sprintf("monitoring.health_checks.custom[%d].endpoint", i),
					Value: check.Endpoint,
					Err:   errors.New("health check endpoint is required"),
				}
			}

			if check.Timeout <= 0 {
				return &core.ConfigError{
					Field: fmt.Sprintf("monitoring.health_checks.custom[%d].timeout", i),
					Value: check.Timeout,
					Err:   errors.New("health check timeout must be positive"),
				}
			}
		}
	}

	// Validate tracing configuration
	if c.Monitoring.Tracing.Enabled {
		if c.Monitoring.Tracing.ServiceName == "" {
			return &core.ConfigError{
				Field: "monitoring.tracing.service_name",
				Value: c.Monitoring.Tracing.ServiceName,
				Err:   errors.New("service name is required for tracing"),
			}
		}

		if c.Monitoring.Tracing.SamplingRate < 0 || c.Monitoring.Tracing.SamplingRate > 1 {
			return &core.ConfigError{
				Field: "monitoring.tracing.sampling_rate",
				Value: c.Monitoring.Tracing.SamplingRate,
				Err:   errors.New("sampling rate must be between 0 and 1"),
			}
		}
	}

	// Validate alerting configuration
	if c.Monitoring.Alerting.Enabled {
		if c.Monitoring.Alerting.Thresholds.ErrorRate < 0 || c.Monitoring.Alerting.Thresholds.ErrorRate > 100 {
			return &core.ConfigError{
				Field: "monitoring.alerting.thresholds.error_rate",
				Value: c.Monitoring.Alerting.Thresholds.ErrorRate,
				Err:   errors.New("error rate threshold must be between 0 and 100"),
			}
		}

		if c.Monitoring.Alerting.Thresholds.Latency < 0 {
			return &core.ConfigError{
				Field: "monitoring.alerting.thresholds.latency",
				Value: c.Monitoring.Alerting.Thresholds.Latency,
				Err:   errors.New("latency threshold cannot be negative"),
			}
		}

		if c.Monitoring.Alerting.Thresholds.MemoryUsage < 0 || c.Monitoring.Alerting.Thresholds.MemoryUsage > 100 {
			return &core.ConfigError{
				Field: "monitoring.alerting.thresholds.memory_usage",
				Value: c.Monitoring.Alerting.Thresholds.MemoryUsage,
				Err:   errors.New("memory usage threshold must be between 0 and 100"),
			}
		}

		if c.Monitoring.Alerting.Thresholds.CPUUsage < 0 || c.Monitoring.Alerting.Thresholds.CPUUsage > 100 {
			return &core.ConfigError{
				Field: "monitoring.alerting.thresholds.cpu_usage",
				Value: c.Monitoring.Alerting.Thresholds.CPUUsage,
				Err:   errors.New("CPU usage threshold must be between 0 and 100"),
			}
		}

		if c.Monitoring.Alerting.Thresholds.ConnectionCount < 0 {
			return &core.ConfigError{
				Field: "monitoring.alerting.thresholds.connection_count",
				Value: c.Monitoring.Alerting.Thresholds.ConnectionCount,
				Err:   errors.New("connection count threshold cannot be negative"),
			}
		}
	}

	return nil
}

// SaveToFile saves the configuration to a JSON file
func (c *ComprehensiveConfig) SaveToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return &core.ConfigError{
			Field: "file",
			Value: filename,
			Err:   fmt.Errorf("failed to create config file: %w", err),
		}
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(c); err != nil {
		return &core.ConfigError{
			Field: "json",
			Value: nil,
			Err:   fmt.Errorf("failed to encode config: %w", err),
		}
	}

	return nil
}

// Clone creates a deep copy of the configuration
func (c *ComprehensiveConfig) Clone() ComprehensiveConfig {
	// This is a simplified clone implementation
	// In a real implementation, you would want to properly deep copy all nested structures
	clone := *c

	// Deep copy slices and maps
	clone.Security.CORS.AllowedOrigins = make([]string, len(c.Security.CORS.AllowedOrigins))
	copy(clone.Security.CORS.AllowedOrigins, c.Security.CORS.AllowedOrigins)

	clone.Security.CORS.AllowedMethods = make([]string, len(c.Security.CORS.AllowedMethods))
	copy(clone.Security.CORS.AllowedMethods, c.Security.CORS.AllowedMethods)

	clone.Security.CORS.AllowedHeaders = make([]string, len(c.Security.CORS.AllowedHeaders))
	copy(clone.Security.CORS.AllowedHeaders, c.Security.CORS.AllowedHeaders)

	clone.Connection.HTTPClient.Headers = make(map[string]string)
	for k, v := range c.Connection.HTTPClient.Headers {
		clone.Connection.HTTPClient.Headers[k] = v
	}

	return clone
}

// Merge merges another configuration into this one
func (c *ComprehensiveConfig) Merge(other ComprehensiveConfig) {
	// This is a simplified merge implementation
	// In a real implementation, you would want to properly merge all nested structures

	// Override non-zero values
	if other.Connection.BaseURL != "" {
		c.Connection.BaseURL = other.Connection.BaseURL
	}
	if other.Connection.Endpoint != "" {
		c.Connection.Endpoint = other.Connection.Endpoint
	}
	if other.Connection.ConnectTimeout != 0 {
		c.Connection.ConnectTimeout = other.Connection.ConnectTimeout
	}
	if other.Connection.ReadTimeout != 0 {
		c.Connection.ReadTimeout = other.Connection.ReadTimeout
	}
	if other.Connection.WriteTimeout != 0 {
		c.Connection.WriteTimeout = other.Connection.WriteTimeout
	}

	// Merge authentication
	if other.Security.Auth.Type != AuthTypeNone {
		c.Security.Auth = other.Security.Auth
	}

	// Merge feature flags
	if other.Features.DebugMode {
		c.Features.DebugMode = other.Features.DebugMode
	}
	if other.Features.ExperimentalFeatures {
		c.Features.ExperimentalFeatures = other.Features.ExperimentalFeatures
	}
	if other.Features.PerformanceProfiling {
		c.Features.PerformanceProfiling = other.Features.PerformanceProfiling
	}
	if other.Features.DetailedMetrics {
		c.Features.DetailedMetrics = other.Features.DetailedMetrics
	}
	if other.Features.RequestTracing {
		c.Features.RequestTracing = other.Features.RequestTracing
	}
}

// String returns a string representation of the configuration
func (c *ComprehensiveConfig) String() string {
	data, _ := json.MarshalIndent(c, "", "  ")
	return string(data)
}

// GetHTTPClient returns an HTTP client configured according to the configuration
func (c *ComprehensiveConfig) GetHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        c.Connection.HTTPClient.MaxIdleConns,
		MaxIdleConnsPerHost: c.Connection.HTTPClient.MaxIdleConnsPerHost,
		MaxConnsPerHost:     c.Connection.HTTPClient.MaxConnsPerHost,
		IdleConnTimeout:     c.Connection.HTTPClient.IdleConnTimeout,
		DisableKeepAlives:   !c.Connection.KeepAlive.Enabled,
	}

	// Configure TLS with secure defaults
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}
	
	if c.Connection.TLS.Enabled {
		// Override defaults with configuration values
		if c.Connection.TLS.InsecureSkipVerify {
			tlsConfig.InsecureSkipVerify = true
		}
		if c.Connection.TLS.ServerName != "" {
			tlsConfig.ServerName = c.Connection.TLS.ServerName
		}
		if c.Connection.TLS.MinVersion != 0 {
			tlsConfig.MinVersion = c.Connection.TLS.MinVersion
		}
		if c.Connection.TLS.MaxVersion != 0 {
			tlsConfig.MaxVersion = c.Connection.TLS.MaxVersion
		}
		if len(c.Connection.TLS.CipherSuites) > 0 {
			tlsConfig.CipherSuites = c.Connection.TLS.CipherSuites
		}
	}
	
	transport.TLSClientConfig = tlsConfig

	// Configure proxy
	if c.Connection.HTTPClient.ProxyURL != "" {
		if proxyURL, err := url.Parse(c.Connection.HTTPClient.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	// Configure HTTP/2
	if c.Connection.HTTPClient.DisableHTTP2 {
		transport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   c.Connection.ConnectTimeout,
	}
}

// GetContext returns a context with the configured timeout
func (c *ComprehensiveConfig) GetContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.Connection.ConnectTimeout)
}

// IsProductionEnvironment returns true if the environment is production
func (c *ComprehensiveConfig) IsProductionEnvironment() bool {
	return c.Environment == EnvironmentProduction
}

// IsDevelopmentEnvironment returns true if the environment is development
func (c *ComprehensiveConfig) IsDevelopmentEnvironment() bool {
	return c.Environment == EnvironmentDevelopment
}

// IsStagingEnvironment returns true if the environment is staging
func (c *ComprehensiveConfig) IsStagingEnvironment() bool {
	return c.Environment == EnvironmentStaging
}

// ============================================================================
// Compatibility Layer for existing simple Config struct in transport.go
// ============================================================================

// ToSimpleConfig converts ComprehensiveConfig to the simple Config format used by transport.go
func (c *ComprehensiveConfig) ToSimpleConfig() *Config {
	headers := make(map[string]string)

	// Copy headers from HTTP client config
	for k, v := range c.Connection.HTTPClient.Headers {
		headers[k] = v
	}

	// Add authentication headers
	switch c.Security.Auth.Type {
	case AuthTypeBearer:
		headers["Authorization"] = "Bearer " + c.Security.Auth.BearerToken
	case AuthTypeAPIKey:
		headers[c.Security.Auth.APIKeyHeader] = c.Security.Auth.APIKey
	case AuthTypeBasic:
		// Basic auth is typically handled by the HTTP client
	}

	return &Config{
		BaseURL:        c.Connection.BaseURL + c.Connection.Endpoint,
		Headers:        headers,
		BufferSize:     c.Performance.Buffering.EventBufferSize,
		ReadTimeout:    c.Connection.ReadTimeout,
		WriteTimeout:   c.Connection.WriteTimeout,
		ReconnectDelay: c.Retry.InitialDelay,
		MaxReconnects:  c.Retry.MaxRetries,
		Client:         c.GetHTTPClient(),
	}
}

// FromSimpleConfig creates a ComprehensiveConfig from the simple Config format
func FromSimpleConfig(simpleConfig *Config) *ComprehensiveConfig {
	config := DefaultComprehensiveConfig()

	if simpleConfig == nil {
		return &config
	}

	// Parse base URL and endpoint
	baseURL := simpleConfig.BaseURL
	endpoint := ""
	if strings.Contains(baseURL, "/events") {
		parts := strings.Split(baseURL, "/events")
		if len(parts) >= 2 {
			baseURL = parts[0]
			endpoint = "/events" + strings.Join(parts[1:], "/events")
		}
	}

	config.Connection.BaseURL = baseURL
	config.Connection.Endpoint = endpoint
	config.Connection.ReadTimeout = simpleConfig.ReadTimeout
	config.Connection.WriteTimeout = simpleConfig.WriteTimeout
	config.Connection.HTTPClient.Headers = simpleConfig.Headers
	config.Performance.Buffering.EventBufferSize = simpleConfig.BufferSize
	config.Retry.InitialDelay = simpleConfig.ReconnectDelay
	config.Retry.MaxRetries = simpleConfig.MaxReconnects

	// Extract authentication from headers
	if authHeader, exists := simpleConfig.Headers["Authorization"]; exists {
		if strings.HasPrefix(authHeader, "Bearer ") {
			config.Security.Auth.Type = AuthTypeBearer
			config.Security.Auth.BearerToken = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	// Check for API key in common headers
	for header, value := range simpleConfig.Headers {
		if strings.ToLower(header) == "x-api-key" || strings.ToLower(header) == "api-key" {
			config.Security.Auth.Type = AuthTypeAPIKey
			config.Security.Auth.APIKey = value
			config.Security.Auth.APIKeyHeader = header
			break
		}
	}

	return &config
}

// NewComprehensiveConfigFromSimple creates a new ComprehensiveConfig from a simple Config
// This function provides a migration path for existing code
func NewComprehensiveConfigFromSimple(simpleConfig *Config) *ComprehensiveConfig {
	return FromSimpleConfig(simpleConfig)
}

// DefaultConfigFromComprehensive returns a simple Config based on comprehensive defaults
// This provides an enhanced version of the simple config while maintaining compatibility
func DefaultConfigFromComprehensive() *Config {
	comprehensive := DefaultComprehensiveConfig()
	return comprehensive.ToSimpleConfig()
}
