package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/auth"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/observability"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/ratelimit"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/security"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/transform"
)

// MiddlewareManager manages middleware chains and configurations
type MiddlewareManager struct {
	registry       MiddlewareRegistry
	chains         map[string]*MiddlewareChain
	defaultChain   string
	configWatcher  *ConfigWatcher
	mu             sync.RWMutex
}

// NewMiddlewareManager creates a new middleware manager
func NewMiddlewareManager() *MiddlewareManager {
	manager := &MiddlewareManager{
		registry:     NewDefaultMiddlewareRegistry(),
		chains:       make(map[string]*MiddlewareChain),
		defaultChain: "default",
	}

	// Create default chain
	defaultHandler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{
			ID:         req.ID,
			StatusCode: 200,
			Body:       map[string]interface{}{"message": "OK"},
			Timestamp:  time.Now(),
		}, nil
	}

	manager.chains["default"] = NewMiddlewareChain(defaultHandler)
	
	return manager
}

// LoadConfiguration loads middleware configuration from file
func (mm *MiddlewareManager) LoadConfiguration(configPath string) error {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config MiddlewareConfiguration
	
	// Determine file format
	switch {
	case strings.HasSuffix(configPath, ".yaml") || strings.HasSuffix(configPath, ".yml"):
		err = yaml.Unmarshal(data, &config)
	case strings.HasSuffix(configPath, ".json"):
		err = json.Unmarshal(data, &config)
	default:
		return fmt.Errorf("unsupported config file format")
	}

	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return mm.ApplyConfiguration(&config)
}

// ApplyConfiguration applies middleware configuration
func (mm *MiddlewareManager) ApplyConfiguration(config *MiddlewareConfiguration) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Clear existing chains
	mm.chains = make(map[string]*MiddlewareChain)

	// Create chains from configuration
	for _, chainConfig := range config.Chains {
		chain, err := mm.createChainFromConfig(chainConfig)
		if err != nil {
			return fmt.Errorf("failed to create chain %s: %w", chainConfig.Name, err)
		}
		mm.chains[chainConfig.Name] = chain
	}

	// Set default chain
	if config.DefaultChain != "" {
		mm.defaultChain = config.DefaultChain
	}

	return nil
}

// CreateChain creates a new middleware chain
func (mm *MiddlewareManager) CreateChain(name string, handler Handler) *MiddlewareChain {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	chain := NewMiddlewareChain(handler)
	mm.chains[name] = chain
	return chain
}

// GetChain returns a middleware chain by name
func (mm *MiddlewareManager) GetChain(name string) *MiddlewareChain {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if chain, exists := mm.chains[name]; exists {
		return chain
	}

	// Return default chain if requested chain doesn't exist
	return mm.chains[mm.defaultChain]
}

// GetDefaultChain returns the default middleware chain
func (mm *MiddlewareManager) GetDefaultChain() *MiddlewareChain {
	return mm.GetChain(mm.defaultChain)
}

// SetDefaultChain sets the default middleware chain
func (mm *MiddlewareManager) SetDefaultChain(name string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if _, exists := mm.chains[name]; !exists {
		return fmt.Errorf("chain %s does not exist", name)
	}

	mm.defaultChain = name
	return nil
}

// ListChains returns all chain names
func (mm *MiddlewareManager) ListChains() []string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	names := make([]string, 0, len(mm.chains))
	for name := range mm.chains {
		names = append(names, name)
	}
	return names
}

// RemoveChain removes a middleware chain
func (mm *MiddlewareManager) RemoveChain(name string) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if name == mm.defaultChain {
		return false // Cannot remove default chain
	}

	if _, exists := mm.chains[name]; exists {
		delete(mm.chains, name)
		return true
	}
	return false
}

// GetRegistry returns the middleware registry
func (mm *MiddlewareManager) GetRegistry() MiddlewareRegistry {
	return mm.registry
}

// WatchConfiguration watches for configuration file changes
func (mm *MiddlewareManager) WatchConfiguration(configPath string) error {
	if mm.configWatcher != nil {
		mm.configWatcher.Stop()
	}

	watcher, err := NewConfigWatcher(configPath, func() {
		if err := mm.LoadConfiguration(configPath); err != nil {
			fmt.Printf("Failed to reload middleware configuration: %v\n", err)
		} else {
			fmt.Printf("Middleware configuration reloaded successfully\n")
		}
	})

	if err != nil {
		return fmt.Errorf("failed to create config watcher: %w", err)
	}

	mm.configWatcher = watcher
	return nil
}

// Stop stops the middleware manager and cleans up resources
func (mm *MiddlewareManager) Stop() error {
	if mm.configWatcher != nil {
		mm.configWatcher.Stop()
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Clean up middleware chains
	for _, chain := range mm.chains {
		// Perform any necessary cleanup
		chain.Clear()
	}

	return nil
}

// GetMetrics returns middleware metrics
func (mm *MiddlewareManager) GetMetrics() map[string]interface{} {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	metrics := make(map[string]interface{})
	
	for name, chain := range mm.chains {
		chainMetrics := map[string]interface{}{
			"middleware_count": len(chain.ListMiddleware()),
			"middleware_list":  chain.ListMiddleware(),
		}
		metrics[name] = chainMetrics
	}

	return metrics
}

// createChainFromConfig creates a middleware chain from configuration
func (mm *MiddlewareManager) createChainFromConfig(config ChainConfiguration) (*MiddlewareChain, error) {
	// Create handler
	var handler Handler
	if config.Handler.Type != "" {
		h, err := mm.createHandlerFromConfig(config.Handler)
		if err != nil {
			return nil, fmt.Errorf("failed to create handler: %w", err)
		}
		handler = h
	} else {
		// Default handler
		handler = func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       map[string]interface{}{"message": "OK"},
				Timestamp:  time.Now(),
			}, nil
		}
	}

	chain := NewMiddlewareChain(handler)

	// Add middleware to chain
	for _, middlewareConfig := range config.Middleware {
		middleware, err := mm.registry.Create(&middlewareConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create middleware %s: %w", middlewareConfig.Name, err)
		}

		if middleware != nil && middleware.Enabled() {
			chain.Add(middleware)
		}
	}

	return chain, nil
}

// createHandlerFromConfig creates a handler from configuration
func (mm *MiddlewareManager) createHandlerFromConfig(config HandlerConfiguration) (Handler, error) {
	switch config.Type {
	case "echo":
		return func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: 200,
				Body:       req.Body,
				Headers:    req.Headers,
				Timestamp:  time.Now(),
			}, nil
		}, nil

	case "status":
		statusCode := 200
		if code, ok := config.Config["status_code"].(int); ok {
			statusCode = code
		}

		message := "OK"
		if msg, ok := config.Config["message"].(string); ok {
			message = msg
		}

		return func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				ID:         req.ID,
				StatusCode: statusCode,
				Body:       map[string]interface{}{"message": message},
				Timestamp:  time.Now(),
			}, nil
		}, nil

	default:
		return nil, fmt.Errorf("unknown handler type: %s", config.Type)
	}
}

// DefaultMiddlewareRegistry provides default middleware factory implementations
type DefaultMiddlewareRegistry struct {
	factories map[string]MiddlewareFactory
	mu        sync.RWMutex
}

// NewDefaultMiddlewareRegistry creates a new default middleware registry
func NewDefaultMiddlewareRegistry() *DefaultMiddlewareRegistry {
	registry := &DefaultMiddlewareRegistry{
		factories: make(map[string]MiddlewareFactory),
	}

	// Register default middleware factories
	registry.registerDefaultFactories()
	
	return registry
}

// Register registers a middleware factory
func (dmr *DefaultMiddlewareRegistry) Register(middlewareType string, factory MiddlewareFactory) error {
	dmr.mu.Lock()
	defer dmr.mu.Unlock()

	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}

	dmr.factories[middlewareType] = factory
	return nil
}

// Unregister removes a middleware factory
func (dmr *DefaultMiddlewareRegistry) Unregister(middlewareType string) error {
	dmr.mu.Lock()
	defer dmr.mu.Unlock()

	if _, exists := dmr.factories[middlewareType]; exists {
		delete(dmr.factories, middlewareType)
		return nil
	}

	return fmt.Errorf("middleware type %s not found", middlewareType)
}

// Create creates a middleware instance from configuration
func (dmr *DefaultMiddlewareRegistry) Create(config *MiddlewareConfig) (Middleware, error) {
	dmr.mu.RLock()
	factory, exists := dmr.factories[config.Type]
	dmr.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown middleware type: %s", config.Type)
	}

	return factory.Create(config)
}

// ListTypes returns all registered middleware types
func (dmr *DefaultMiddlewareRegistry) ListTypes() []string {
	dmr.mu.RLock()
	defer dmr.mu.RUnlock()

	types := make([]string, 0, len(dmr.factories))
	for middlewareType := range dmr.factories {
		types = append(types, middlewareType)
	}

	sort.Strings(types)
	return types
}

// registerDefaultFactories registers the default middleware factories
func (dmr *DefaultMiddlewareRegistry) registerDefaultFactories() {
	// Authentication middleware factories
	dmr.Register("jwt_auth", &JWTMiddlewareFactory{})
	dmr.Register("api_key_auth", &APIKeyMiddlewareFactory{})
	dmr.Register("basic_auth", &BasicAuthMiddlewareFactory{})
	dmr.Register("oauth2_auth", &OAuth2MiddlewareFactory{})

	// Observability middleware factories
	dmr.Register("logging", &LoggingMiddlewareFactory{})
	dmr.Register("metrics", &MetricsMiddlewareFactory{})
	dmr.Register("correlation_id", &CorrelationIDMiddlewareFactory{})

	// Rate limiting middleware factories
	dmr.Register("rate_limit", &RateLimitMiddlewareFactory{})
	dmr.Register("distributed_rate_limit", &DistributedRateLimitMiddlewareFactory{})

	// Transformation middleware factories
	dmr.Register("transformation", &TransformationMiddlewareFactory{})

	// Security middleware factories
	dmr.Register("security", &SecurityMiddlewareFactory{})
}

// Middleware factory implementations

// JWTMiddlewareFactory creates JWT authentication middleware
type JWTMiddlewareFactory struct{}

func (f *JWTMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	jwtConfig := &auth.JWTConfig{
		Algorithm:  "HS256",
		Expiration: 24 * time.Hour,
	}

	if secret, ok := config.Config["secret"].(string); ok {
		jwtConfig.Secret = secret
	}

	if alg, ok := config.Config["algorithm"].(string); ok {
		jwtConfig.Algorithm = alg
	}

	if issuer, ok := config.Config["issuer"].(string); ok {
		jwtConfig.Issuer = issuer
	}

	middleware, err := auth.NewJWTMiddleware(jwtConfig, nil, nil)
	if err != nil {
		return nil, err
	}

	err = middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}

	return NewAuthMiddlewareAdapter(middleware), nil
}

func (f *JWTMiddlewareFactory) SupportedTypes() []string {
	return []string{"jwt_auth"}
}

// APIKeyMiddlewareFactory creates API key authentication middleware
type APIKeyMiddlewareFactory struct{}

func (f *APIKeyMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	apiKeyConfig := &auth.APIKeyConfig{
		HeaderName:   "X-API-Key",
		CacheTimeout: 5 * time.Minute,
	}

	if header, ok := config.Config["header_name"].(string); ok {
		apiKeyConfig.HeaderName = header
	}

	if endpoint, ok := config.Config["validation_endpoint"].(string); ok {
		apiKeyConfig.ValidationEndpoint = endpoint
	}

	middleware := auth.NewAPIKeyMiddleware(apiKeyConfig, nil, nil)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewAuthMiddlewareAdapter(middleware), nil
}

func (f *APIKeyMiddlewareFactory) SupportedTypes() []string {
	return []string{"api_key_auth"}
}

// BasicAuthMiddlewareFactory creates basic authentication middleware
type BasicAuthMiddlewareFactory struct{}

func (f *BasicAuthMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	basicConfig := &auth.BasicAuthConfig{
		Realm:              "Restricted Area",
		HashAlgorithm:      "bcrypt",
	}

	if realm, ok := config.Config["realm"].(string); ok {
		basicConfig.Realm = realm
	}

	if hash, ok := config.Config["hash_algorithm"].(string); ok {
		basicConfig.HashAlgorithm = hash
	}

	middleware := auth.NewBasicAuthMiddleware(basicConfig, nil, nil, nil)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewAuthMiddlewareAdapter(middleware), nil
}

func (f *BasicAuthMiddlewareFactory) SupportedTypes() []string {
	return []string{"basic_auth"}
}

// OAuth2MiddlewareFactory creates OAuth2 authentication middleware
type OAuth2MiddlewareFactory struct{}

func (f *OAuth2MiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	oauth2Config := &auth.OAuth2Config{}

	if clientID, ok := config.Config["client_id"].(string); ok {
		oauth2Config.ClientID = clientID
	}

	if clientSecret, ok := config.Config["client_secret"].(string); ok {
		oauth2Config.ClientSecret = clientSecret
	}

	if authURL, ok := config.Config["auth_url"].(string); ok {
		oauth2Config.AuthURL = authURL
	}

	if tokenURL, ok := config.Config["token_url"].(string); ok {
		oauth2Config.TokenURL = tokenURL
	}

	middleware := auth.NewOAuth2Middleware(oauth2Config, nil, nil)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewAuthMiddlewareAdapter(middleware), nil
}

func (f *OAuth2MiddlewareFactory) SupportedTypes() []string {
	return []string{"oauth2_auth"}
}

// LoggingMiddlewareFactory creates logging middleware
type LoggingMiddlewareFactory struct{}

func (f *LoggingMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	loggingConfig := &observability.LoggingConfig{
		Level:             observability.LogLevelInfo,
		Format:            observability.LogFormatJSON,
		EnableCorrelation: true,
	}

	if level, ok := config.Config["level"].(string); ok {
		switch strings.ToUpper(level) {
		case "DEBUG":
			loggingConfig.Level = observability.LogLevelDebug
		case "INFO":
			loggingConfig.Level = observability.LogLevelInfo
		case "WARN":
			loggingConfig.Level = observability.LogLevelWarn
		case "ERROR":
			loggingConfig.Level = observability.LogLevelError
		}
	}

	middleware := observability.NewLoggingMiddleware(loggingConfig)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewObservabilityMiddlewareAdapter(middleware), nil
}

func (f *LoggingMiddlewareFactory) SupportedTypes() []string {
	return []string{"logging"}
}

// MetricsMiddlewareFactory creates metrics middleware
type MetricsMiddlewareFactory struct{}

func (f *MetricsMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	metricsConfig := &observability.MetricsConfig{
		EnableRequestCount:    true,
		EnableRequestDuration: true,
		EnableActiveRequests:  true,
	}

	middleware := observability.NewMetricsMiddleware(metricsConfig, nil)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewObservabilityMiddlewareAdapter(middleware), nil
}

func (f *MetricsMiddlewareFactory) SupportedTypes() []string {
	return []string{"metrics"}
}

// CorrelationIDMiddlewareFactory creates correlation ID middleware
type CorrelationIDMiddlewareFactory struct{}

func (f *CorrelationIDMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	headerName := "X-Correlation-ID"
	if header, ok := config.Config["header_name"].(string); ok {
		headerName = header
	}

	middleware := observability.NewCorrelationIDMiddleware(headerName)
	err := middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewObservabilityMiddlewareAdapter(middleware), nil
}

func (f *CorrelationIDMiddlewareFactory) SupportedTypes() []string {
	return []string{"correlation_id"}
}

// RateLimitMiddlewareFactory creates rate limiting middleware
type RateLimitMiddlewareFactory struct{}

func (f *RateLimitMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	rateLimitConfig := &ratelimit.RateLimitConfig{
		Algorithm:       ratelimit.AlgorithmTokenBucket,
		RequestsPerUnit: 100,
		Unit:            time.Minute,
		Burst:           10,
		KeyGenerator:    "ip",
	}

	if alg, ok := config.Config["algorithm"].(string); ok {
		rateLimitConfig.Algorithm = ratelimit.RateLimitAlgorithm(alg)
	}

	if requests, ok := config.Config["requests_per_unit"].(int); ok {
		rateLimitConfig.RequestsPerUnit = int64(requests)
	}

	if unit, ok := config.Config["unit"].(string); ok {
		if duration, err := time.ParseDuration(unit); err == nil {
			rateLimitConfig.Unit = duration
		}
	}

	middleware, err := ratelimit.NewRateLimitMiddleware(rateLimitConfig)
	if err != nil {
		return nil, err
	}

	err = middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewRateLimitMiddlewareAdapter(middleware), nil
}

func (f *RateLimitMiddlewareFactory) SupportedTypes() []string {
	return []string{"rate_limit"}
}

// DistributedRateLimitMiddlewareFactory creates distributed rate limiting middleware
type DistributedRateLimitMiddlewareFactory struct{}

func (f *DistributedRateLimitMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	rateLimitConfig := &ratelimit.RateLimitConfig{
		Algorithm:       ratelimit.AlgorithmTokenBucket,
		RequestsPerUnit: 100,
		Unit:            time.Minute,
		Burst:           10,
		KeyGenerator:    "ip",
		Distributed:     true,
	}

	if redisURL, ok := config.Config["redis_url"].(string); ok {
		rateLimitConfig.RedisURL = redisURL
	}

	// Use mock Redis client for now
	redisClient := ratelimit.NewMockRedisClient()

	middleware, err := ratelimit.NewDistributedRateLimitMiddleware(rateLimitConfig, redisClient)
	if err != nil {
		return nil, err
	}

	err = middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewRateLimitMiddlewareAdapter(middleware), nil
}

func (f *DistributedRateLimitMiddlewareFactory) SupportedTypes() []string {
	return []string{"distributed_rate_limit"}
}

// TransformationMiddlewareFactory creates transformation middleware
type TransformationMiddlewareFactory struct{}

func (f *TransformationMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	transformConfig := &transform.TransformationConfig{
		DefaultPipeline: "default",
		Pipelines: []transform.PipelineConfig{
			{
				Name:    "default",
				Enabled: true,
				Transformers: []transform.TransformerConfig{
					{
						Type:    "sanitization",
						Name:    "default_sanitization",
						Enabled: true,
						Config: map[string]interface{}{
							"sensitive_fields": []string{"password", "token", "secret"},
							"replacement":      "[REDACTED]",
						},
					},
				},
			},
		},
	}

	middleware, err := transform.NewTransformationMiddleware(transformConfig)
	if err != nil {
		return nil, err
	}

	err = middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewTransformMiddlewareAdapter(middleware), nil
}

func (f *TransformationMiddlewareFactory) SupportedTypes() []string {
	return []string{"transformation"}
}

// SecurityMiddlewareFactory creates security middleware
type SecurityMiddlewareFactory struct{}

func (f *SecurityMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	securityConfig := &security.SecurityConfig{
		CORS: &security.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		},
		Headers: &security.SecurityHeadersConfig{
			Enabled:                 true,
			XFrameOptions:          "DENY",
			XContentTypeOptions:    "nosniff",
			XXSSProtection:         "1; mode=block",
		},
		ThreatDetection: &security.ThreatDetectionConfig{
			Enabled:      true,
			SQLInjection: true,
			XSSDetection: true,
			LogThreats:   true,
		},
	}

	middleware, err := security.NewSecurityMiddleware(securityConfig)
	if err != nil {
		return nil, err
	}

	err = middleware.Configure(config.Config)
	if err != nil {
		return nil, err
	}
	return NewSecurityMiddlewareAdapter(middleware), nil
}

func (f *SecurityMiddlewareFactory) SupportedTypes() []string {
	return []string{"security"}
}

// Configuration structures

// MiddlewareConfiguration represents the complete middleware configuration
type MiddlewareConfiguration struct {
	DefaultChain string                 `json:"default_chain" yaml:"default_chain"`
	Chains       []ChainConfiguration   `json:"chains" yaml:"chains"`
	Global       map[string]interface{} `json:"global" yaml:"global"`
}

// ChainConfiguration represents a middleware chain configuration
type ChainConfiguration struct {
	Name        string                 `json:"name" yaml:"name"`
	Enabled     bool                   `json:"enabled" yaml:"enabled"`
	Handler     HandlerConfiguration   `json:"handler" yaml:"handler"`
	Middleware  []MiddlewareConfig     `json:"middleware" yaml:"middleware"`
	Conditions  map[string]interface{} `json:"conditions" yaml:"conditions"`
}

// HandlerConfiguration represents a handler configuration
type HandlerConfiguration struct {
	Type   string                 `json:"type" yaml:"type"`
	Config map[string]interface{} `json:"config" yaml:"config"`
}

// ConfigWatcher watches for configuration file changes
type ConfigWatcher struct {
	filePath    string
	callback    func()
	stopChannel chan bool
	stopped     bool
	mu          sync.Mutex
}

// NewConfigWatcher creates a new configuration file watcher
func NewConfigWatcher(filePath string, callback func()) (*ConfigWatcher, error) {
	watcher := &ConfigWatcher{
		filePath:    filePath,
		callback:    callback,
		stopChannel: make(chan bool, 1),
	}

	go watcher.watch()
	return watcher, nil
}

// watch monitors the configuration file for changes
func (cw *ConfigWatcher) watch() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastModTime time.Time
	if info, err := os.Stat(cw.filePath); err == nil {
		lastModTime = info.ModTime()
	}

	for {
		select {
		case <-cw.stopChannel:
			return
		case <-ticker.C:
			if info, err := os.Stat(cw.filePath); err == nil {
				if info.ModTime().After(lastModTime) {
					lastModTime = info.ModTime()
					// Wait a bit to ensure file write is complete
					time.Sleep(100 * time.Millisecond)
					cw.callback()
				}
			}
		}
	}
}

// Stop stops the configuration watcher
func (cw *ConfigWatcher) Stop() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if !cw.stopped {
		cw.stopped = true
		cw.stopChannel <- true
		close(cw.stopChannel)
	}
}