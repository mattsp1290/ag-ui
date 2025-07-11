package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// CompressionType represents supported compression algorithms.
type CompressionType string

const (
	CompressionNone   CompressionType = "none"
	CompressionGzip   CompressionType = "gzip"
	CompressionZstd   CompressionType = "zstd"
	CompressionSnappy CompressionType = "snappy"
	CompressionBrotli CompressionType = "brotli"
)

// CapabilityRequirements defines what capabilities are required or preferred
type CapabilityRequirements struct {
	Required  []string `json:"required"`
	Preferred []string `json:"preferred"`
}

// Config represents the main transport configuration
type Config struct {
	// Primary transport type to use
	Primary string `yaml:"primary" json:"primary" validate:"required"`

	// Fallback transport types in order of preference
	Fallback []string `yaml:"fallback" json:"fallback"`

	// Selection strategy configuration
	Selection SelectionConfig `yaml:"selection" json:"selection"`

	// Capability requirements
	Capabilities CapabilityConfig `yaml:"capabilities" json:"capabilities"`

	// Performance thresholds
	Performance PerformanceConfig `yaml:"performance" json:"performance"`

	// Transport-specific configurations
	Transports map[string]interface{} `yaml:"transports" json:"transports"`

	// Global settings
	Global GlobalConfig `yaml:"global" json:"global"`
}

// SelectionConfig configures transport selection behavior
type SelectionConfig struct {
	// Strategy for selecting transports: "performance", "capability", "manual"
	Strategy string `yaml:"strategy" json:"strategy" default:"performance"`

	// Health check interval
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval" default:"30s"`

	// Failover threshold (number of failures before switching)
	FailoverThreshold int `yaml:"failover_threshold" json:"failover_threshold" default:"3"`

	// Retry configuration
	Retry RetryConfig `yaml:"retry" json:"retry"`

	// Load balancing strategy
	LoadBalancing LoadBalancingConfig `yaml:"load_balancing" json:"load_balancing"`
}

// CapabilityConfig defines capability requirements
type CapabilityConfig struct {
	// Required capabilities
	Required []string `yaml:"required" json:"required"`

	// Preferred capabilities
	Preferred []string `yaml:"preferred" json:"preferred"`

	// Minimum requirements
	MinRequirements CapabilityRequirements `yaml:"min_requirements" json:"min_requirements"`
}

// PerformanceConfig defines performance thresholds
type PerformanceConfig struct {
	// Latency threshold in milliseconds
	LatencyThreshold int64 `yaml:"latency_threshold" json:"latency_threshold" default:"100"`

	// Throughput threshold in messages per second
	ThroughputThreshold int64 `yaml:"throughput_threshold" json:"throughput_threshold" default:"1000"`

	// Connection timeout
	ConnectionTimeout time.Duration `yaml:"connection_timeout" json:"connection_timeout" default:"30s"`

	// Read timeout
	ReadTimeout time.Duration `yaml:"read_timeout" json:"read_timeout" default:"30s"`

	// Write timeout
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout" default:"30s"`

	// Maximum concurrent connections
	MaxConcurrentConnections int `yaml:"max_concurrent_connections" json:"max_concurrent_connections" default:"100"`
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	// Maximum number of retries
	MaxAttempts int `yaml:"max_attempts" json:"max_attempts" default:"3"`

	// Initial delay between retries
	InitialDelay time.Duration `yaml:"initial_delay" json:"initial_delay" default:"1s"`

	// Maximum delay between retries
	MaxDelay time.Duration `yaml:"max_delay" json:"max_delay" default:"30s"`

	// Backoff multiplier
	BackoffMultiplier float64 `yaml:"backoff_multiplier" json:"backoff_multiplier" default:"2.0"`

	// Whether to use jitter
	Jitter bool `yaml:"jitter" json:"jitter" default:"true"`
}

// LoadBalancingConfig configures load balancing
type LoadBalancingConfig struct {
	// Strategy: "round_robin", "least_connections", "random", "weighted"
	Strategy string `yaml:"strategy" json:"strategy" default:"round_robin"`

	// Weights for weighted load balancing
	Weights map[string]int `yaml:"weights" json:"weights"`

	// Health check configuration
	HealthCheck HealthCheckConfig `yaml:"health_check" json:"health_check"`
}

// HealthCheckConfig configures health checking
type HealthCheckConfig struct {
	// Whether health checking is enabled
	Enabled bool `yaml:"enabled" json:"enabled" default:"true"`

	// Health check interval
	Interval time.Duration `yaml:"interval" json:"interval" default:"10s"`

	// Health check timeout
	Timeout time.Duration `yaml:"timeout" json:"timeout" default:"5s"`

	// Number of consecutive failures before marking unhealthy
	FailureThreshold int `yaml:"failure_threshold" json:"failure_threshold" default:"3"`

	// Number of consecutive successes before marking healthy
	SuccessThreshold int `yaml:"success_threshold" json:"success_threshold" default:"1"`
}

// GlobalConfig contains global transport settings
type GlobalConfig struct {
	// Whether to enable metrics collection
	EnableMetrics bool `yaml:"enable_metrics" json:"enable_metrics" default:"true"`

	// Whether to enable tracing
	EnableTracing bool `yaml:"enable_tracing" json:"enable_tracing" default:"false"`

	// Log level
	LogLevel string `yaml:"log_level" json:"log_level" default:"info"`

	// Maximum message size
	MaxMessageSize int64 `yaml:"max_message_size" json:"max_message_size" default:"1048576"` // 1MB

	// Buffer sizes
	BufferSize int `yaml:"buffer_size" json:"buffer_size" default:"1024"`

	// Compression settings
	Compression CompressionConfig `yaml:"compression" json:"compression"`

	// TLS settings
	TLS TLSConfig `yaml:"tls" json:"tls"`

	// Backpressure settings
	Backpressure BackpressureConfig `yaml:"backpressure" json:"backpressure"`
}

// CompressionConfig configures compression settings
type CompressionConfig struct {
	// Whether compression is enabled
	Enabled bool `yaml:"enabled" json:"enabled" default:"false"`

	// Compression type
	Type CompressionType `yaml:"type" json:"type" default:"gzip"`

	// Compression level (1-9)
	Level int `yaml:"level" json:"level" default:"6"`

	// Minimum size threshold for compression
	MinSize int64 `yaml:"min_size" json:"min_size" default:"1024"`
}

// TLSConfig configures TLS settings
type TLSConfig struct {
	// Whether TLS is enabled
	Enabled bool `yaml:"enabled" json:"enabled" default:"false"`

	// Certificate file path
	CertFile string `yaml:"cert_file" json:"cert_file"`

	// Key file path
	KeyFile string `yaml:"key_file" json:"key_file"`

	// CA certificate file path
	CAFile string `yaml:"ca_file" json:"ca_file"`

	// Whether to verify certificates
	VerifyCerts bool `yaml:"verify_certs" json:"verify_certs" default:"true"`

	// Server name for certificate verification
	ServerName string `yaml:"server_name" json:"server_name"`
}

// BackpressureConfig configures backpressure handling
type BackpressureConfig struct {
	// Strategy defines the backpressure strategy to use
	Strategy string `yaml:"strategy" json:"strategy" default:"none"`
	
	// BufferSize is the size of the event buffer
	BufferSize int `yaml:"buffer_size" json:"buffer_size" default:"1024"`
	
	// HighWaterMark is the percentage of buffer fullness that triggers backpressure
	HighWaterMark float64 `yaml:"high_water_mark" json:"high_water_mark" default:"0.8"`
	
	// LowWaterMark is the percentage of buffer fullness that releases backpressure
	LowWaterMark float64 `yaml:"low_water_mark" json:"low_water_mark" default:"0.2"`
	
	// BlockTimeout is the maximum time to block when using block_timeout strategy
	BlockTimeout time.Duration `yaml:"block_timeout" json:"block_timeout" default:"5s"`
	
	// EnableMetrics enables backpressure metrics collection
	EnableMetrics bool `yaml:"enable_metrics" json:"enable_metrics" default:"true"`
}

// ConfigManager manages transport configuration
type ConfigManager struct {
	config    *Config
	defaults  *Config
	validators map[string]Validator
}

// Validator validates configuration values
type Validator interface {
	Validate(value interface{}) error
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		config:     &Config{},
		defaults:   getDefaultConfig(),
		validators: make(map[string]Validator),
	}
}

// LoadFromFile loads configuration from a file
func (cm *ConfigManager) LoadFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	ext := filepath.Ext(filename)
	switch ext {
	case ".yaml", ".yml":
		return cm.LoadFromYAML(data)
	case ".json":
		return cm.LoadFromJSON(data)
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}
}

// LoadFromYAML loads configuration from YAML data
func (cm *ConfigManager) LoadFromYAML(data []byte) error {
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse YAML config: %w", err)
	}

	cm.config = &config
	return cm.applyDefaults()
}

// LoadFromJSON loads configuration from JSON data
func (cm *ConfigManager) LoadFromJSON(data []byte) error {
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse JSON config: %w", err)
	}

	cm.config = &config
	return cm.applyDefaults()
}

// LoadFromEnvironment loads configuration from environment variables
func (cm *ConfigManager) LoadFromEnvironment() error {
	config := &Config{}
	
	// Load main config fields
	if primary := os.Getenv("AG_UI_TRANSPORT_PRIMARY"); primary != "" {
		config.Primary = primary
	}
	
	if fallback := os.Getenv("AG_UI_TRANSPORT_FALLBACK"); fallback != "" {
		config.Fallback = strings.Split(fallback, ",")
	}

	// Load selection config
	if strategy := os.Getenv("AG_UI_TRANSPORT_SELECTION_STRATEGY"); strategy != "" {
		config.Selection.Strategy = strategy
	}

	if interval := os.Getenv("AG_UI_TRANSPORT_HEALTH_CHECK_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			config.Selection.HealthCheckInterval = d
		}
	}

	if threshold := os.Getenv("AG_UI_TRANSPORT_FAILOVER_THRESHOLD"); threshold != "" {
		if t, err := strconv.Atoi(threshold); err == nil {
			config.Selection.FailoverThreshold = t
		}
	}

	// Load capability config
	if required := os.Getenv("AG_UI_TRANSPORT_CAPABILITIES_REQUIRED"); required != "" {
		config.Capabilities.Required = strings.Split(required, ",")
	}

	if preferred := os.Getenv("AG_UI_TRANSPORT_CAPABILITIES_PREFERRED"); preferred != "" {
		config.Capabilities.Preferred = strings.Split(preferred, ",")
	}

	cm.config = config
	return cm.applyDefaults()
}

// GetConfig returns the current configuration
func (cm *ConfigManager) GetConfig() *Config {
	return cm.config
}

// SetConfig sets the configuration
func (cm *ConfigManager) SetConfig(config *Config) error {
	cm.config = config
	return cm.applyDefaults()
}

// Validate validates the configuration
func (cm *ConfigManager) Validate() error {
	if cm.config == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate primary transport
	if cm.config.Primary == "" {
		return fmt.Errorf("primary transport must be specified")
	}

	// Validate selection strategy
	validStrategies := []string{"performance", "capability", "manual"}
	if !contains(validStrategies, cm.config.Selection.Strategy) {
		return fmt.Errorf("invalid selection strategy: %s", cm.config.Selection.Strategy)
	}

	// Validate performance thresholds
	if cm.config.Performance.LatencyThreshold < 0 {
		return fmt.Errorf("latency threshold must be non-negative")
	}

	if cm.config.Performance.ThroughputThreshold < 0 {
		return fmt.Errorf("throughput threshold must be non-negative")
	}

	// Validate retry configuration
	if cm.config.Selection.Retry.MaxAttempts < 0 {
		return fmt.Errorf("max retry attempts must be non-negative")
	}

	if cm.config.Selection.Retry.BackoffMultiplier < 1.0 {
		return fmt.Errorf("backoff multiplier must be >= 1.0")
	}

	// Validate load balancing
	validLBStrategies := []string{"round_robin", "least_connections", "random", "weighted"}
	if !contains(validLBStrategies, cm.config.Selection.LoadBalancing.Strategy) {
		return fmt.Errorf("invalid load balancing strategy: %s", cm.config.Selection.LoadBalancing.Strategy)
	}

	// Validate custom validators
	for field, validator := range cm.validators {
		if err := validator.Validate(cm.getFieldValue(field)); err != nil {
			return fmt.Errorf("validation failed for field %s: %w", field, err)
		}
	}

	return nil
}

// AddValidator adds a custom validator for a field
func (cm *ConfigManager) AddValidator(field string, validator Validator) {
	cm.validators[field] = validator
}

// applyDefaults applies default values to the configuration
func (cm *ConfigManager) applyDefaults() error {
	if cm.config == nil {
		cm.config = &Config{}
	}

	// Apply defaults using reflection
	return cm.applyDefaultsRecursive(reflect.ValueOf(cm.config).Elem(), reflect.ValueOf(cm.defaults).Elem())
}

// applyDefaultsRecursive recursively applies default values
func (cm *ConfigManager) applyDefaultsRecursive(configValue, defaultValue reflect.Value) error {
	for i := 0; i < configValue.NumField(); i++ {
		configField := configValue.Field(i)
		defaultField := defaultValue.Field(i)
		structField := configValue.Type().Field(i)

		// Check if field has a default tag
		if defaultTag := structField.Tag.Get("default"); defaultTag != "" {
			if cm.isZeroValue(configField) {
				if err := cm.setDefaultValue(configField, defaultTag); err != nil {
					return fmt.Errorf("failed to set default for field %s: %w", structField.Name, err)
				}
			}
		}

		// Recursively apply defaults to nested structs
		if configField.Kind() == reflect.Struct && configField.CanSet() {
			if err := cm.applyDefaultsRecursive(configField, defaultField); err != nil {
				return err
			}
		}
	}

	return nil
}

// isZeroValue checks if a value is the zero value for its type
func (cm *ConfigManager) isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0.0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Slice:
		return v.Len() == 0
	case reflect.Map:
		return v.Len() == 0
	default:
		return false
	}
}

// setDefaultValue sets a default value based on the default tag
func (cm *ConfigManager) setDefaultValue(field reflect.Value, defaultValue string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(defaultValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if defaultValue == "" {
			return nil
		}
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			if d, err := time.ParseDuration(defaultValue); err == nil {
				field.SetInt(int64(d))
			} else {
				return fmt.Errorf("invalid duration: %s", defaultValue)
			}
		} else {
			if i, err := strconv.ParseInt(defaultValue, 10, 64); err == nil {
				field.SetInt(i)
			} else {
				return fmt.Errorf("invalid integer: %s", defaultValue)
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if u, err := strconv.ParseUint(defaultValue, 10, 64); err == nil {
			field.SetUint(u)
		} else {
			return fmt.Errorf("invalid unsigned integer: %s", defaultValue)
		}
	case reflect.Float32, reflect.Float64:
		if f, err := strconv.ParseFloat(defaultValue, 64); err == nil {
			field.SetFloat(f)
		} else {
			return fmt.Errorf("invalid float: %s", defaultValue)
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(defaultValue); err == nil {
			field.SetBool(b)
		} else {
			return fmt.Errorf("invalid boolean: %s", defaultValue)
		}
	}

	return nil
}

// getFieldValue gets the value of a field by name
func (cm *ConfigManager) getFieldValue(fieldName string) interface{} {
	v := reflect.ValueOf(cm.config).Elem()
	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}
	return field.Interface()
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() *Config {
	return &Config{
		Primary:  "websocket",
		Fallback: []string{"sse", "http"},
		Selection: SelectionConfig{
			Strategy:            "performance",
			HealthCheckInterval: 30 * time.Second,
			FailoverThreshold:   3,
			Retry: RetryConfig{
				MaxAttempts:       3,
				InitialDelay:      1 * time.Second,
				MaxDelay:          30 * time.Second,
				BackoffMultiplier: 2.0,
				Jitter:            true,
			},
			LoadBalancing: LoadBalancingConfig{
				Strategy: "round_robin",
				HealthCheck: HealthCheckConfig{
					Enabled:          true,
					Interval:         10 * time.Second,
					Timeout:          5 * time.Second,
					FailureThreshold: 3,
					SuccessThreshold: 1,
				},
			},
		},
		Performance: PerformanceConfig{
			LatencyThreshold:         100,
			ThroughputThreshold:      1000,
			ConnectionTimeout:        30 * time.Second,
			ReadTimeout:              30 * time.Second,
			WriteTimeout:             30 * time.Second,
			MaxConcurrentConnections: 100,
		},
		Global: GlobalConfig{
			EnableMetrics:  true,
			EnableTracing:  false,
			LogLevel:       "info",
			MaxMessageSize: 1024 * 1024, // 1MB
			BufferSize:     1024,
			Compression: CompressionConfig{
				Enabled: false,
				Type:    CompressionGzip,
				Level:   6,
				MinSize: 1024,
			},
			TLS: TLSConfig{
				Enabled:     false,
				VerifyCerts: true,
			},
			Backpressure: BackpressureConfig{
				Strategy:      "none",
				BufferSize:    1024,
				HighWaterMark: 0.8,
				LowWaterMark:  0.2,
				BlockTimeout:  5 * time.Second,
				EnableMetrics: true,
			},
		},
	}
}

// contains checks if a slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}