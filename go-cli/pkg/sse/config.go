package sse

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// MetricsMode represents the metrics reporting mode
type MetricsMode string

const (
	MetricsModeOff   MetricsMode = "off"
	MetricsModeLog   MetricsMode = "log"
	MetricsModeJSON  MetricsMode = "json"
)

// CLIConfig holds CLI configuration for SSE client
type CLIConfig struct {
	// Connection settings
	URL            string
	Headers        map[string]string
	Timeout        time.Duration
	
	// Metrics settings
	MetricsMode     MetricsMode
	MetricsInterval time.Duration
	
	// Reconnection settings
	EnableReconnect   bool
	MaxReconnectTries int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	
	// Logging
	LogLevel  string
	LogFormat string
	
	// Advanced
	BufferSize int
}

// RegisterFlags registers SSE-related command-line flags
func RegisterFlags(fs *flag.FlagSet) *CLIConfig {
	config := &CLIConfig{
		Headers: make(map[string]string),
	}
	
	// Connection flags
	fs.StringVar(&config.URL, "sse-url", "", "SSE endpoint URL")
	fs.DurationVar(&config.Timeout, "sse-timeout", 30*time.Second, "Connection timeout")
	
	// Metrics flags
	metricsMode := fs.String("metrics", "off", "Metrics mode: off|log|json")
	fs.DurationVar(&config.MetricsInterval, "metrics-interval", 5*time.Second, "Metrics reporting interval")
	
	// Reconnection flags
	fs.BoolVar(&config.EnableReconnect, "sse-reconnect", true, "Enable automatic reconnection")
	fs.IntVar(&config.MaxReconnectTries, "sse-max-reconnect", 0, "Maximum reconnection attempts (0=unlimited)")
	fs.DurationVar(&config.InitialBackoff, "sse-initial-backoff", time.Second, "Initial reconnection backoff")
	fs.DurationVar(&config.MaxBackoff, "sse-max-backoff", 30*time.Second, "Maximum reconnection backoff")
	
	// Logging flags
	fs.StringVar(&config.LogLevel, "log-level", "info", "Log level: debug|info|warn|error")
	fs.StringVar(&config.LogFormat, "log-format", "text", "Log format: text|json")
	
	// Advanced flags
	fs.IntVar(&config.BufferSize, "sse-buffer-size", 100, "Event buffer size")
	
	// Custom headers flag (comma-separated key=value pairs)
	headersStr := fs.String("sse-headers", "", "Custom headers (format: key1=value1,key2=value2)")
	
	// Parse headers after flag parsing
	fs.Parse(os.Args[1:])
	
	// Parse metrics mode
	switch strings.ToLower(*metricsMode) {
	case "off":
		config.MetricsMode = MetricsModeOff
	case "log":
		config.MetricsMode = MetricsModeLog
	case "json":
		config.MetricsMode = MetricsModeJSON
	default:
		fmt.Fprintf(os.Stderr, "Invalid metrics mode: %s, using 'off'\n", *metricsMode)
		config.MetricsMode = MetricsModeOff
	}
	
	// Parse headers
	if *headersStr != "" {
		pairs := strings.Split(*headersStr, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				config.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}
	
	return config
}

// LoadFromEnv loads configuration from environment variables
func (c *CLIConfig) LoadFromEnv() {
	// Override with environment variables if set
	if url := os.Getenv("AG_UI_SSE_URL"); url != "" {
		c.URL = url
	}
	
	if metricsMode := os.Getenv("AG_UI_METRICS"); metricsMode != "" {
		switch strings.ToLower(metricsMode) {
		case "off":
			c.MetricsMode = MetricsModeOff
		case "log":
			c.MetricsMode = MetricsModeLog
		case "json":
			c.MetricsMode = MetricsModeJSON
		}
	}
	
	if interval := os.Getenv("AG_UI_METRICS_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			c.MetricsInterval = d
		}
	}
	
	if logLevel := os.Getenv("AG_UI_LOG_LEVEL"); logLevel != "" {
		c.LogLevel = logLevel
	}
	
	if logFormat := os.Getenv("AG_UI_LOG_FORMAT"); logFormat != "" {
		c.LogFormat = logFormat
	}
	
	// Parse headers from environment
	if headers := os.Getenv("AG_UI_SSE_HEADERS"); headers != "" {
		pairs := strings.Split(headers, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				c.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}
}

// Validate validates the configuration
func (c *CLIConfig) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("SSE URL is required")
	}
	
	if c.MetricsInterval < time.Second {
		return fmt.Errorf("metrics interval must be at least 1 second")
	}
	
	if c.BufferSize < 1 {
		return fmt.Errorf("buffer size must be at least 1")
	}
	
	if c.InitialBackoff < time.Millisecond*100 {
		return fmt.Errorf("initial backoff must be at least 100ms")
	}
	
	if c.MaxBackoff < c.InitialBackoff {
		return fmt.Errorf("max backoff must be >= initial backoff")
	}
	
	return nil
}

// ToClientConfig converts CLI config to SSE client config
func (c *CLIConfig) ToClientConfig() (ClientConfig, error) {
	if err := c.Validate(); err != nil {
		return ClientConfig{}, err
	}
	
	// Set up logger
	logger := logrus.New()
	
	// Set log level
	level, err := logrus.ParseLevel(c.LogLevel)
	if err != nil {
		logger.Warnf("Invalid log level %s, using info", c.LogLevel)
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)
	
	// Set log format
	if c.LogFormat == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}
	
	// Create reporter based on metrics mode
	var reporter MetricsReporter
	if c.MetricsMode != MetricsModeOff {
		format := "human"
		if c.MetricsMode == MetricsModeJSON {
			format = "json"
		}
		reporter = NewLoggerReporter(logger, format)
	}
	
	return ClientConfig{
		URL:                  c.URL,
		Headers:              c.Headers,
		EnableReconnect:      c.EnableReconnect,
		InitialBackoff:       c.InitialBackoff,
		MaxBackoff:           c.MaxBackoff,
		BackoffMultiplier:    2.0,
		MaxReconnectAttempts: c.MaxReconnectTries,
		ConnectTimeout:       c.Timeout,
		ReadTimeout:          c.Timeout,
		BufferSize:           c.BufferSize,
		EnableMetrics:        c.MetricsMode != MetricsModeOff,
		MetricsReporter:      reporter,
		MetricsInterval:      c.MetricsInterval,
		Logger:               logger,
		
		// Callbacks for logging
		OnConnect: func(connID string) {
			logger.WithField("connection_id", connID).Info("Connected to SSE stream")
		},
		OnDisconnect: func(err error) {
			if err != nil {
				logger.WithError(err).Warn("Disconnected from SSE stream")
			} else {
				logger.Info("Disconnected from SSE stream")
			}
		},
		OnReconnect: func(attempt int) {
			logger.WithField("attempt", attempt).Info("Reconnection attempt")
		},
		OnError: func(err error) {
			logger.WithError(err).Error("SSE error")
		},
	}, nil
}

// PrintUsage prints usage information for SSE flags
func PrintUsage() {
	fmt.Println("SSE Client Flags:")
	fmt.Println("  --sse-url string           SSE endpoint URL (required)")
	fmt.Println("  --metrics string           Metrics mode: off|log|json (default: off)")
	fmt.Println("  --metrics-interval duration  Metrics reporting interval (default: 5s)")
	fmt.Println("  --sse-timeout duration     Connection timeout (default: 30s)")
	fmt.Println("  --sse-reconnect            Enable automatic reconnection (default: true)")
	fmt.Println("  --sse-max-reconnect int    Maximum reconnection attempts, 0=unlimited (default: 0)")
	fmt.Println("  --sse-initial-backoff duration  Initial reconnection backoff (default: 1s)")
	fmt.Println("  --sse-max-backoff duration     Maximum reconnection backoff (default: 30s)")
	fmt.Println("  --sse-headers string       Custom headers (format: key1=value1,key2=value2)")
	fmt.Println("  --sse-buffer-size int      Event buffer size (default: 100)")
	fmt.Println("  --log-level string         Log level: debug|info|warn|error (default: info)")
	fmt.Println("  --log-format string        Log format: text|json (default: text)")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  AG_UI_SSE_URL              SSE endpoint URL")
	fmt.Println("  AG_UI_METRICS              Metrics mode")
	fmt.Println("  AG_UI_METRICS_INTERVAL     Metrics reporting interval")
	fmt.Println("  AG_UI_SSE_HEADERS          Custom headers")
	fmt.Println("  AG_UI_LOG_LEVEL            Log level")
	fmt.Println("  AG_UI_LOG_FORMAT           Log format")
}