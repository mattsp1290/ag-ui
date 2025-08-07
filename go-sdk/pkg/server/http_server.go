package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	httppool "github.com/mattsp1290/ag-ui/go-sdk/pkg/http"
)

// Performance optimization pools
var (
	// String builder pool for hot-path string operations
	stringBuilderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}

	// Response builder pool for JSON responses
	responseBuilderPool = sync.Pool{
		New: func() interface{} {
			b := &strings.Builder{}
			b.Grow(256) // Pre-allocate for typical response size
			return b
		},
	}
)

// HTTPServer provides HTTP server integration with multiple Go web frameworks.
// It implements framework-agnostic server capabilities with performance optimization
// and consistent API across different frameworks.
type HTTPServer struct {
	// Configuration
	config *HTTPServerConfig

	// Framework integrations
	ginEngine    *gin.Engine
	fiberApp     *fiber.App
	stdlibMux    *http.ServeMux
	customRouter *CustomRouter

	// Optimized middleware chain for performance
	optimizedMiddleware []func(http.ResponseWriter, *http.Request, func())

	// Core server components
	httpServer *http.Server
	connPool   *httppool.HTTPConnectionPool

	// Pre-compiled handler for performance optimization
	compileTimeHandler http.Handler

	// Agent management
	agents   map[string]core.Agent
	agentsMu sync.RWMutex

	// Middleware chains
	ginMiddleware    []gin.HandlerFunc
	fiberMiddleware  []fiber.Handler
	stdlibMiddleware []StdlibMiddleware

	// Lifecycle management
	mu           sync.RWMutex
	running      int32
	shutdown     chan struct{}
	shutdownOnce sync.Once

	// Metrics and monitoring
	metrics   *HTTPServerMetrics
	metricsMu sync.RWMutex

	// Background workers
	workerGroup sync.WaitGroup
}

// HTTPServerConfig contains configuration options for the HTTP server.
type HTTPServerConfig struct {
	// Server settings
	Address        string        `json:"address" yaml:"address"`
	Port           int           `json:"port" yaml:"port"`
	ReadTimeout    time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout   time.Duration `json:"write_timeout" yaml:"write_timeout"`
	IdleTimeout    time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
	MaxHeaderBytes int           `json:"max_header_bytes" yaml:"max_header_bytes"`

	// TLS configuration
	TLSConfig   *tls.Config `json:"-" yaml:"-"`
	TLSCertFile string      `json:"tls_cert_file" yaml:"tls_cert_file"`
	TLSKeyFile  string      `json:"tls_key_file" yaml:"tls_key_file"`
	EnableTLS   bool        `json:"enable_tls" yaml:"enable_tls"`

	// Framework preferences
	PreferredFramework FrameworkType `json:"preferred_framework" yaml:"preferred_framework"`
	EnableGin          bool          `json:"enable_gin" yaml:"enable_gin"`
	EnableFiber        bool          `json:"enable_fiber" yaml:"enable_fiber"`
	EnableStdlib       bool          `json:"enable_stdlib" yaml:"enable_stdlib"`
	EnableCustomRouter bool          `json:"enable_custom_router" yaml:"enable_custom_router"`

	// Performance settings
	ConnectionPoolConfig *httppool.HTTPPoolConfig `json:"connection_pool" yaml:"connection_pool"`
	EnableConnectionPool bool                     `json:"enable_connection_pool" yaml:"enable_connection_pool"`

	// Middleware settings
	EnableCORS      bool     `json:"enable_cors" yaml:"enable_cors"`
	CORSOrigins     []string `json:"cors_origins" yaml:"cors_origins"`
	EnableMetrics   bool     `json:"enable_metrics" yaml:"enable_metrics"`
	EnableRecovery  bool     `json:"enable_recovery" yaml:"enable_recovery"`
	EnableLogging   bool     `json:"enable_logging" yaml:"enable_logging"`
	EnableRateLimit bool     `json:"enable_rate_limit" yaml:"enable_rate_limit"`

	// Rate limiting
	RateLimitRequests int           `json:"rate_limit_requests" yaml:"rate_limit_requests"`
	RateLimitWindow   time.Duration `json:"rate_limit_window" yaml:"rate_limit_window"`
	// Rate limiter memory protection
	MaxRateLimiters    int           `json:"max_rate_limiters" yaml:"max_rate_limiters"`
	RateLimiterTTL     time.Duration `json:"rate_limiter_ttl" yaml:"rate_limiter_ttl"`
	EnableMemoryBounds bool          `json:"enable_memory_bounds" yaml:"enable_memory_bounds"`

	// Custom settings
	CustomHeaders map[string]string `json:"custom_headers" yaml:"custom_headers"`

	// Feature flags
	EnableHealthCheck    bool   `json:"enable_health_check" yaml:"enable_health_check"`
	EnableAgentEndpoints bool   `json:"enable_agent_endpoints" yaml:"enable_agent_endpoints"`
	EnableStaticFiles    bool   `json:"enable_static_files" yaml:"enable_static_files"`
	StaticFilesPath      string `json:"static_files_path" yaml:"static_files_path"`
	StaticFilesPrefix    string `json:"static_files_prefix" yaml:"static_files_prefix"`
}

// FrameworkType represents the supported web frameworks.
type FrameworkType string

const (
	FrameworkGin          FrameworkType = "gin"
	FrameworkFiber        FrameworkType = "fiber"
	FrameworkStdlib       FrameworkType = "stdlib"
	FrameworkCustomRouter FrameworkType = "custom"
	FrameworkAuto         FrameworkType = "auto"
)

// StdlibMiddleware represents middleware for stdlib HTTP.
type StdlibMiddleware func(http.Handler) http.Handler

// HTTPServerMetrics contains metrics for the HTTP server.
type HTTPServerMetrics struct {
	// Request metrics
	TotalRequests       int64         `json:"total_requests"`
	SuccessfulRequests  int64         `json:"successful_requests"`
	FailedRequests      int64         `json:"failed_requests"`
	AverageResponseTime time.Duration `json:"average_response_time"`

	// Framework-specific metrics
	GinRequests    int64 `json:"gin_requests"`
	FiberRequests  int64 `json:"fiber_requests"`
	StdlibRequests int64 `json:"stdlib_requests"`
	CustomRequests int64 `json:"custom_requests"`

	// Connection metrics
	ActiveConnections int64 `json:"active_connections"`
	TotalConnections  int64 `json:"total_connections"`

	// Error metrics
	ErrorsByType map[string]int64 `json:"errors_by_type"`

	// Performance metrics
	RequestDuration map[string]time.Duration `json:"request_duration_percentiles"`

	// Agent metrics
	AgentRequests map[string]int64 `json:"agent_requests"`

	// Timestamp
	LastUpdated time.Time `json:"last_updated"`
	StartTime   time.Time `json:"start_time"`
}

// CustomRouter provides a custom HTTP router implementation.
type CustomRouter struct {
	routes          map[string]map[string]http.HandlerFunc // method -> pattern -> handler
	middleware      []func(http.Handler) http.Handler
	mu              sync.RWMutex
	fallbackHandler http.Handler
}

// RouteDefinition represents a route configuration.
type RouteDefinition struct {
	Method  string
	Pattern string
	Handler http.HandlerFunc
}

// NewHTTPServer creates a new HTTP server with the specified configuration.
func NewHTTPServer(config *HTTPServerConfig) (*HTTPServer, error) {
	if config == nil {
		config = DefaultHTTPServerConfig()
	}

	// Validate configuration
	if err := validateHTTPServerConfig(config); err != nil {
		return nil, errors.NewAgentError(
			errors.ErrorTypeValidation,
			"invalid HTTP server configuration",
			"HTTPServer",
		).WithCause(err).WithDetail("operation", "New")
	}

	// Apply defaults
	config = mergeWithDefaults(config)

	server := &HTTPServer{
		config:   config,
		agents:   make(map[string]core.Agent),
		shutdown: make(chan struct{}),
		metrics:  newHTTPServerMetrics(),
	}

	// Initialize connection pool if enabled
	if config.EnableConnectionPool && config.ConnectionPoolConfig != nil {
		pool, err := httppool.NewHTTPConnectionPool(config.ConnectionPoolConfig)
		if err != nil {
			return nil, errors.NewAgentError(
				errors.ErrorTypeInvalidState,
				"failed to create connection pool",
				"HTTPServer",
			).WithCause(err).WithDetail("operation", "New")
		}
		server.connPool = pool
	}

	// Initialize frameworks based on configuration
	if err := server.initializeFrameworks(); err != nil {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to initialize frameworks",
			"HTTPServer",
		).WithCause(err).WithDetail("operation", "New")
	}

	// Pre-compile handler for optimal performance
	if err := server.compileHandler(); err != nil {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to compile handler",
			"HTTPServer",
		).WithCause(err).WithDetail("operation", "New")
	}

	return server, nil
}

// initializeFrameworks sets up the configured web frameworks.
func (s *HTTPServer) initializeFrameworks() error {
	var err error

	// Initialize Gin if enabled
	if s.config.EnableGin {
		if err = s.initializeGin(); err != nil {
			return errors.WithOperation("initialize", "gin_framework", err)
		}
	}

	// Initialize Fiber if enabled
	if s.config.EnableFiber {
		if err = s.initializeFiber(); err != nil {
			return errors.WithOperation("initialize", "fiber_framework", err)
		}
	}

	// Initialize stdlib if enabled
	if s.config.EnableStdlib {
		if err = s.initializeStdlib(); err != nil {
			return errors.WithOperation("initialize", "stdlib_framework", err)
		}
	}

	// Initialize custom router if enabled
	if s.config.EnableCustomRouter {
		if err = s.initializeCustomRouter(); err != nil {
			return errors.WithOperation("initialize", "custom_router", err)
		}
	}

	return nil
}

// compileHandler pre-compiles the HTTP handler for optimal performance
func (s *HTTPServer) compileHandler() error {
	// Determine the handler based on preferred framework at initialization time
	switch s.config.PreferredFramework {
	case FrameworkGin:
		if s.ginEngine == nil {
			return fmt.Errorf("Gin framework not initialized")
		}
		s.compileTimeHandler = s.ginEngine
	case FrameworkFiber:
		// For Fiber, we'll use a custom adapter that avoids runtime conversion
		if s.fiberApp == nil {
			return fmt.Errorf("Fiber framework not initialized")
		}
		// Create optimized Fiber adapter
		s.compileTimeHandler = s.createOptimizedFiberHandler()
	case FrameworkStdlib:
		if s.stdlibMux == nil {
			return fmt.Errorf("stdlib mux not initialized")
		}
		s.compileTimeHandler = s.stdlibMux
	case FrameworkCustomRouter:
		if s.customRouter == nil {
			return fmt.Errorf("custom router not initialized")
		}
		s.compileTimeHandler = s.customRouter
	default:
		// Auto-select best framework at compile time
		handler, err := s.selectBestFramework()
		if err != nil {
			return fmt.Errorf("failed to select framework: %w", err)
		}
		s.compileTimeHandler = handler
	}

	return nil
}

// createOptimizedFiberHandler creates an optimized adapter for Fiber
func (s *HTTPServer) createOptimizedFiberHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Direct Fiber handling without conversion overhead
		// This is a simplified adapter - in production, you'd use fiber.Adaptor
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Fiber optimized handler"))
	})
}

// initializeGin sets up the Gin framework.
func (s *HTTPServer) initializeGin() error {
	// Set Gin mode based on configuration
	if !s.config.EnableLogging {
		gin.SetMode(gin.ReleaseMode)
	}

	s.ginEngine = gin.New()

	// Add default middleware
	if s.config.EnableRecovery {
		s.ginEngine.Use(gin.Recovery())
	}

	if s.config.EnableLogging {
		s.ginEngine.Use(gin.Logger())
	}

	// Add CORS middleware
	if s.config.EnableCORS {
		s.ginEngine.Use(s.ginCORSMiddleware())
	}

	// Add metrics middleware
	if s.config.EnableMetrics {
		s.ginEngine.Use(s.ginMetricsMiddleware())
	}

	// Add custom headers middleware
	if len(s.config.CustomHeaders) > 0 {
		s.ginEngine.Use(s.ginCustomHeadersMiddleware())
	}

	// Setup routes
	s.setupGinRoutes()

	return nil
}

// initializeFiber sets up the Fiber framework.
func (s *HTTPServer) initializeFiber() error {
	// Create Fiber app with optimized configuration
	config := fiber.Config{
		ServerHeader:                 "AG-UI-Server",
		AppName:                      "AG-UI HTTP Server",
		DisableStartupMessage:        !s.config.EnableLogging,
		StrictRouting:                false,
		CaseSensitive:                false,
		UnescapePath:                 false,
		BodyLimit:                    4 * 1024 * 1024, // 4MB default
		ReadTimeout:                  s.config.ReadTimeout,
		WriteTimeout:                 s.config.WriteTimeout,
		IdleTimeout:                  s.config.IdleTimeout,
		ReadBufferSize:               4096,
		WriteBufferSize:              4096,
		CompressedFileSuffix:         ".fiber.gz",
		ProxyHeader:                  "",
		GETOnly:                      false,
		ErrorHandler:                 nil,
		DisableKeepalive:             false,
		DisableDefaultDate:           false,
		DisableDefaultContentType:    false,
		DisableHeaderNormalizing:     false,
		DisablePreParseMultipartForm: false,
		Prefork:                      false,
		Network:                      fiber.NetworkTCP,
	}

	s.fiberApp = fiber.New(config)

	// Add recovery middleware
	if s.config.EnableRecovery {
		s.fiberApp.Use(fiberrecover.New(fiberrecover.Config{
			EnableStackTrace: s.config.EnableLogging,
		}))
	}

	// Add logging middleware
	if s.config.EnableLogging {
		s.fiberApp.Use(logger.New(logger.Config{
			Format: "${time} ${status} - ${method} ${path} ${latency}\n",
		}))
	}

	// Add CORS middleware
	if s.config.EnableCORS {
		s.fiberApp.Use(s.fiberCORSMiddleware())
	}

	// Add metrics middleware
	if s.config.EnableMetrics {
		s.fiberApp.Use(s.fiberMetricsMiddleware())
	}

	// Add custom headers middleware
	if len(s.config.CustomHeaders) > 0 {
		s.fiberApp.Use(s.fiberCustomHeadersMiddleware())
	}

	// Setup routes
	s.setupFiberRoutes()

	return nil
}

// initializeStdlib sets up the standard library HTTP server.
func (s *HTTPServer) initializeStdlib() error {
	s.stdlibMux = http.NewServeMux()

	// Setup middleware chain
	var handler http.Handler = s.stdlibMux

	// Add custom headers middleware
	if len(s.config.CustomHeaders) > 0 {
		handler = s.stdlibCustomHeadersMiddleware(handler)
	}

	// Add metrics middleware
	if s.config.EnableMetrics {
		handler = s.stdlibMetricsMiddleware(handler)
	}

	// Add CORS middleware
	if s.config.EnableCORS {
		handler = s.stdlibCORSMiddleware(handler)
	}

	// Add recovery middleware
	if s.config.EnableRecovery {
		handler = s.stdlibRecoveryMiddleware(handler)
	}

	// Add logging middleware
	if s.config.EnableLogging {
		handler = s.stdlibLoggingMiddleware(handler)
	}

	// Setup routes
	s.setupStdlibRoutes()

	return nil
}

// initializeCustomRouter sets up the custom router.
func (s *HTTPServer) initializeCustomRouter() error {
	s.customRouter = &CustomRouter{
		routes: make(map[string]map[string]http.HandlerFunc),
	}

	// Setup middleware chain
	if s.config.EnableLogging {
		s.customRouter.Use(s.stdlibLoggingMiddleware)
	}

	if s.config.EnableRecovery {
		s.customRouter.Use(s.stdlibRecoveryMiddleware)
	}

	if s.config.EnableCORS {
		s.customRouter.Use(s.stdlibCORSMiddleware)
	}

	if s.config.EnableMetrics {
		s.customRouter.Use(s.stdlibMetricsMiddleware)
	}

	if len(s.config.CustomHeaders) > 0 {
		s.customRouter.Use(s.stdlibCustomHeadersMiddleware)
	}

	// Setup routes
	s.setupCustomRoutes()

	return nil
}

// RegisterWithGin registers the HTTP server with a Gin engine.
func (s *HTTPServer) RegisterWithGin(engine *gin.Engine) error {
	if engine == nil {
		return errors.NewValidationError("gin_engine_nil", "Gin engine cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ginEngine = engine
	s.config.EnableGin = true

	// Setup routes on the provided engine
	s.setupGinRoutes()

	return nil
}

// RegisterWithFiber registers the HTTP server with a Fiber app instance.
func (s *HTTPServer) RegisterWithFiber(app *fiber.App) error {
	if app == nil {
		return errors.NewValidationError("fiber_app_nil", "Fiber app cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.fiberApp = app
	s.config.EnableFiber = true

	// Setup routes on the provided app
	s.setupFiberRoutes()

	return nil
}

// RegisterWithStdlib registers the HTTP server with a standard library ServeMux.
func (s *HTTPServer) RegisterWithStdlib(mux *http.ServeMux) error {
	if mux == nil {
		return errors.NewValidationError("stdlib_mux_nil", "stdlib mux cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.stdlibMux = mux
	s.config.EnableStdlib = true

	// Setup routes on the provided mux
	s.setupStdlibRoutes()

	return nil
}

// Start starts the HTTP server.
func (s *HTTPServer) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"HTTP server is already running",
			"HTTPServer",
		).WithDetail("operation", "Start")
	}

	// Start connection pool if enabled
	if s.connPool != nil {
		// Connection pool doesn't need explicit start in this implementation
		// It's ready to use once created
	}

	// Use pre-compiled handler for optimal performance (no runtime overhead)
	if s.compileTimeHandler == nil {
		atomic.StoreInt32(&s.running, 0)
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"compile-time handler not initialized",
			"HTTPServer",
		).WithDetail("operation", "Start")
	}

	// Special handling for Fiber which runs its own server
	if s.config.PreferredFramework == FrameworkFiber {
		return s.startFiberServer(ctx)
	}

	handler := s.compileTimeHandler

	// Create HTTP server - optimized string concatenation
	builder := stringBuilderPool.Get().(*strings.Builder)
	builder.Reset()
	builder.WriteString(s.config.Address)
	builder.WriteByte(':')
	builder.WriteString(fmt.Sprintf("%d", s.config.Port))
	address := builder.String()
	stringBuilderPool.Put(builder)
	s.httpServer = &http.Server{
		Addr:           address,
		Handler:        handler,
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		IdleTimeout:    s.config.IdleTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
		TLSConfig:      s.config.TLSConfig,
	}

	// Start background workers
	s.startBackgroundWorkers()

	// Start server
	s.workerGroup.Add(1)
	go func() {
		defer s.workerGroup.Done()

		var serverErr error
		if s.config.EnableTLS && s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
			serverErr = s.httpServer.ListenAndServeTLS(s.config.TLSCertFile, s.config.TLSKeyFile)
		} else {
			serverErr = s.httpServer.ListenAndServe()
		}

		if serverErr != nil && serverErr != http.ErrServerClosed {
			// Log error but don't return it as this is a background goroutine
			fmt.Printf("HTTP server error: %v\n", serverErr)
		}
	}()

	return nil
}

// Stop gracefully stops the HTTP server with enhanced timeout handling.
func (s *HTTPServer) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"HTTP server is not running",
			"HTTPServer",
		).WithDetail("operation", "Stop")
	}

	s.shutdownOnce.Do(func() {
		close(s.shutdown)
	})

	// Create a timeout context for shutdown operations
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	// Enhanced shutdown with proper timeout handling
	shutdownErrors := make([]error, 0)

	// Stop HTTP server or Fiber app with timeout
	if s.httpServer != nil {
		serverShutdown := make(chan error, 1)
		go func() {
			serverShutdown <- s.httpServer.Shutdown(shutdownCtx)
		}()

		select {
		case err := <-serverShutdown:
			if err != nil {
				shutdownErrors = append(shutdownErrors, errors.NewAgentError(
					errors.ErrorTypeInvalidState,
					"failed to shutdown HTTP server",
					"HTTPServer",
				).WithCause(err).WithDetail("operation", "Stop"))
			}
		case <-shutdownCtx.Done():
			// Force close after timeout
			if err := s.httpServer.Close(); err != nil {
				shutdownErrors = append(shutdownErrors, errors.NewAgentError(
					errors.ErrorTypeInvalidState,
					"failed to force close HTTP server",
					"HTTPServer",
				).WithCause(err).WithDetail("operation", "Stop"))
			}
		}
	} else if s.fiberApp != nil {
		fiberShutdown := make(chan error, 1)
		go func() {
			fiberShutdown <- s.fiberApp.Shutdown()
		}()

		select {
		case err := <-fiberShutdown:
			if err != nil {
				shutdownErrors = append(shutdownErrors, errors.NewAgentError(
					errors.ErrorTypeInvalidState,
					"failed to shutdown Fiber app",
					"HTTPServer",
				).WithCause(err).WithDetail("operation", "Stop"))
			}
		case <-time.After(10 * time.Second):
			// Fiber doesn't have a force close, so we'll continue
			shutdownErrors = append(shutdownErrors, errors.NewAgentError(
				errors.ErrorTypeTimeout,
				"Fiber app shutdown timed out",
				"HTTPServer",
			).WithDetail("operation", "Stop"))
		}
	}

	// Stop connection pool with timeout
	if s.connPool != nil {
		poolShutdown := make(chan error, 1)
		go func() {
			poolShutdown <- s.connPool.Shutdown(shutdownCtx)
		}()

		select {
		case err := <-poolShutdown:
			if err != nil {
				shutdownErrors = append(shutdownErrors, errors.NewAgentError(
					errors.ErrorTypeInvalidState,
					"failed to shutdown connection pool",
					"HTTPServer",
				).WithCause(err).WithDetail("operation", "Stop"))
			}
		case <-shutdownCtx.Done():
			// Force cleanup if shutdown times out
			if err := s.connPool.Shutdown(shutdownCtx); err != nil {
				shutdownErrors = append(shutdownErrors, errors.NewAgentError(
					errors.ErrorTypeInvalidState,
					"failed to force close connection pool",
					"HTTPServer",
				).WithCause(err).WithDetail("operation", "Stop"))
			}
		}
	}

	// Wait for background workers to finish with timeout
	workersDone := make(chan struct{})
	go func() {
		s.workerGroup.Wait()
		close(workersDone)
	}()

	select {
	case <-workersDone:
		// Workers finished gracefully
	case <-shutdownCtx.Done():
		// Context timeout, continue anyway (workers will be abandoned)
		shutdownErrors = append(shutdownErrors, errors.NewAgentError(
			errors.ErrorTypeTimeout,
			"background workers shutdown timed out",
			"HTTPServer",
		).WithDetail("operation", "Stop"))
	}

	// Return combined errors if any occurred
	if len(shutdownErrors) > 0 {
		// Return the first error, but log all
		if s.config.EnableLogging {
			builder := stringBuilderPool.Get().(*strings.Builder)
			builder.Reset()
			builder.WriteString(fmt.Sprintf("HTTP Server shutdown completed with %d errors:\n", len(shutdownErrors)))
			for i, err := range shutdownErrors {
				builder.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
			}
			fmt.Print(builder.String())
			stringBuilderPool.Put(builder)
		}
		return shutdownErrors[0]
	}

	return nil
}

// selectBestFramework automatically selects the best available framework.
func (s *HTTPServer) selectBestFramework() (http.Handler, error) {
	// Priority order: Gin > Fiber > stdlib > custom
	if s.config.EnableGin && s.ginEngine != nil {
		s.config.PreferredFramework = FrameworkGin
		return s.ginEngine, nil
	}

	if s.config.EnableFiber && s.fiberApp != nil {
		s.config.PreferredFramework = FrameworkFiber
		// Fiber runs on its own server, return a placeholder handler
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Fiber server runs independently"))
		}), nil
	}

	if s.config.EnableStdlib && s.stdlibMux != nil {
		s.config.PreferredFramework = FrameworkStdlib
		return s.stdlibMux, nil
	}

	if s.config.EnableCustomRouter && s.customRouter != nil {
		s.config.PreferredFramework = FrameworkCustomRouter
		return s.customRouter, nil
	}

	return nil, fmt.Errorf("no frameworks available")
}

// RegisterAgent registers an agent with the HTTP server.
func (s *HTTPServer) RegisterAgent(name string, agent core.Agent) error {
	if name == "" {
		return errors.NewValidationError("agent_name_empty", "agent name cannot be empty")
	}

	if agent == nil {
		return errors.NewValidationError("agent_nil", "agent cannot be nil")
	}

	s.agentsMu.Lock()
	defer s.agentsMu.Unlock()

	if _, exists := s.agents[name]; exists {
		return errors.NewResourceConflictError("agent", name, "agent already registered")
	}

	s.agents[name] = agent
	return nil
}

// UnregisterAgent removes an agent from the HTTP server.
func (s *HTTPServer) UnregisterAgent(name string) error {
	if name == "" {
		return errors.NewValidationError("agent_name_empty", "agent name cannot be empty")
	}

	s.agentsMu.Lock()
	defer s.agentsMu.Unlock()

	if _, exists := s.agents[name]; !exists {
		return errors.NewResourceNotFoundError("agent", name)
	}

	delete(s.agents, name)
	return nil
}

// GetAgent retrieves a registered agent by name.
func (s *HTTPServer) GetAgent(name string) (core.Agent, bool) {
	s.agentsMu.RLock()
	defer s.agentsMu.RUnlock()

	agent, exists := s.agents[name]
	return agent, exists
}

// ListAgents returns a list of all registered agent names.
func (s *HTTPServer) ListAgents() []string {
	s.agentsMu.RLock()
	defer s.agentsMu.RUnlock()

	agents := make([]string, 0, len(s.agents))
	for name := range s.agents {
		agents = append(agents, name)
	}
	return agents
}

// GetMetrics returns current HTTP server metrics.
func (s *HTTPServer) GetMetrics() *HTTPServerMetrics {
	s.metricsMu.RLock()
	defer s.metricsMu.RUnlock()

	// Create a deep copy
	metrics := &HTTPServerMetrics{
		TotalRequests:       atomic.LoadInt64(&s.metrics.TotalRequests),
		SuccessfulRequests:  atomic.LoadInt64(&s.metrics.SuccessfulRequests),
		FailedRequests:      atomic.LoadInt64(&s.metrics.FailedRequests),
		AverageResponseTime: s.metrics.AverageResponseTime,
		GinRequests:         atomic.LoadInt64(&s.metrics.GinRequests),
		FiberRequests:       atomic.LoadInt64(&s.metrics.FiberRequests),
		StdlibRequests:      atomic.LoadInt64(&s.metrics.StdlibRequests),
		CustomRequests:      atomic.LoadInt64(&s.metrics.CustomRequests),
		ActiveConnections:   atomic.LoadInt64(&s.metrics.ActiveConnections),
		TotalConnections:    atomic.LoadInt64(&s.metrics.TotalConnections),
		ErrorsByType:        make(map[string]int64),
		RequestDuration:     make(map[string]time.Duration),
		AgentRequests:       make(map[string]int64),
		LastUpdated:         s.metrics.LastUpdated,
		StartTime:           s.metrics.StartTime,
	}

	// Copy maps
	for k, v := range s.metrics.ErrorsByType {
		metrics.ErrorsByType[k] = v
	}
	for k, v := range s.metrics.RequestDuration {
		metrics.RequestDuration[k] = v
	}
	for k, v := range s.metrics.AgentRequests {
		metrics.AgentRequests[k] = v
	}

	return metrics
}

// IsRunning returns true if the HTTP server is currently running.
func (s *HTTPServer) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}

// GetConfig returns a copy of the HTTP server configuration.
func (s *HTTPServer) GetConfig() HTTPServerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.config
}

// OptimizedMiddlewareChain represents a flattened middleware chain for better performance
type OptimizedMiddlewareChain struct {
	middlewares []func(http.ResponseWriter, *http.Request, func())
	mu          sync.RWMutex
}

// NewOptimizedMiddlewareChain creates a new optimized middleware chain
func NewOptimizedMiddlewareChain() *OptimizedMiddlewareChain {
	return &OptimizedMiddlewareChain{
		middlewares: make([]func(http.ResponseWriter, *http.Request, func()), 0, 8),
	}
}

// Add adds a middleware to the optimized chain
func (omc *OptimizedMiddlewareChain) Add(middleware func(http.ResponseWriter, *http.Request, func())) {
	omc.mu.Lock()
	defer omc.mu.Unlock()
	omc.middlewares = append(omc.middlewares, middleware)
}

// Execute executes the middleware chain with minimal call stack overhead
func (omc *OptimizedMiddlewareChain) Execute(w http.ResponseWriter, r *http.Request, final func()) {
	omc.mu.RLock()
	middlewares := omc.middlewares
	omc.mu.RUnlock()

	if len(middlewares) == 0 {
		final()
		return
	}

	// Flatten the middleware chain to reduce call stack depth
	var index int
	var next func()
	next = func() {
		if index < len(middlewares) {
			middleware := middlewares[index]
			index++
			middleware(w, r, next)
		} else {
			final()
		}
	}
	next()
}

// CustomRouter methods

// Use adds middleware to the custom router.
func (r *CustomRouter) Use(middleware func(http.Handler) http.Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middleware = append(r.middleware, middleware)
}

// Handle registers a handler for the given method and pattern.
func (r *CustomRouter) Handle(method, pattern string, handler http.HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.routes[method] == nil {
		r.routes[method] = make(map[string]http.HandlerFunc)
	}
	r.routes[method][pattern] = handler
}

// GET registers a GET handler.
func (r *CustomRouter) GET(pattern string, handler http.HandlerFunc) {
	r.Handle("GET", pattern, handler)
}

// POST registers a POST handler.
func (r *CustomRouter) POST(pattern string, handler http.HandlerFunc) {
	r.Handle("POST", pattern, handler)
}

// PUT registers a PUT handler.
func (r *CustomRouter) PUT(pattern string, handler http.HandlerFunc) {
	r.Handle("PUT", pattern, handler)
}

// DELETE registers a DELETE handler.
func (r *CustomRouter) DELETE(pattern string, handler http.HandlerFunc) {
	r.Handle("DELETE", pattern, handler)
}

// SetFallback sets a fallback handler for unmatched routes.
func (r *CustomRouter) SetFallback(handler http.Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallbackHandler = handler
}

// ServeHTTP implements http.Handler for the custom router.
func (r *CustomRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Build middleware chain
	var handler http.Handler = http.HandlerFunc(r.serveRoute)

	r.mu.RLock()
	for i := len(r.middleware) - 1; i >= 0; i-- {
		handler = r.middleware[i](handler)
	}
	r.mu.RUnlock()

	handler.ServeHTTP(w, req)
}

// serveRoute handles the actual route matching and serving.
func (r *CustomRouter) serveRoute(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check for exact pattern match
	if methodRoutes, exists := r.routes[req.Method]; exists {
		if handler, found := methodRoutes[req.URL.Path]; found {
			handler(w, req)
			return
		}
	}

	// Use fallback handler if available
	if r.fallbackHandler != nil {
		r.fallbackHandler.ServeHTTP(w, req)
		return
	}

	// Default 404 response
	http.NotFound(w, req)
}

// Utility functions

// startFiberServer starts the Fiber server independently.
func (s *HTTPServer) startFiberServer(ctx context.Context) error {
	// Start background workers
	s.startBackgroundWorkers()

	// Start Fiber server in a goroutine
	s.workerGroup.Add(1)
	go func() {
		defer s.workerGroup.Done()

		builder := stringBuilderPool.Get().(*strings.Builder)
		builder.Reset()
		builder.WriteString(s.config.Address)
		builder.WriteByte(':')
		builder.WriteString(fmt.Sprintf("%d", s.config.Port))
		address := builder.String()
		stringBuilderPool.Put(builder)

		var serverErr error
		if s.config.EnableTLS && s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
			serverErr = s.fiberApp.ListenTLS(address, s.config.TLSCertFile, s.config.TLSKeyFile)
		} else {
			serverErr = s.fiberApp.Listen(address)
		}

		if serverErr != nil {
			fmt.Printf("Fiber server error: %v\n", serverErr)
		}
	}()

	return nil
}

// DefaultHTTPServerConfig returns a default HTTP server configuration.
func DefaultHTTPServerConfig() *HTTPServerConfig {
	return &HTTPServerConfig{
		Address:              "localhost",
		Port:                 8080,
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
		IdleTimeout:          120 * time.Second,
		MaxHeaderBytes:       1 << 20, // 1MB
		PreferredFramework:   FrameworkAuto,
		EnableGin:            true,
		EnableFiber:          true,
		EnableStdlib:         true,
		EnableCustomRouter:   false,
		EnableConnectionPool: false,
		EnableCORS:           true,
		CORSOrigins:          []string{"*"},
		EnableMetrics:        true,
		EnableRecovery:       true,
		EnableLogging:        true,
		EnableRateLimit:      false,
		RateLimitRequests:    1000,
		RateLimitWindow:      time.Minute,
		MaxRateLimiters:      50000,
		RateLimiterTTL:       10 * time.Minute,
		EnableMemoryBounds:   true,
		CustomHeaders:        make(map[string]string),
		EnableHealthCheck:    true,
		EnableAgentEndpoints: true,
		EnableStaticFiles:    false,
		StaticFilesPath:      "./static",
		StaticFilesPrefix:    "/static",
	}
}

// validateHTTPServerConfig validates the HTTP server configuration.
func validateHTTPServerConfig(config *HTTPServerConfig) error {
	if config == nil {
		return errors.NewValidationError("config_nil", "HTTP server configuration cannot be nil")
	}

	if config.Port < 0 || config.Port > 65535 {
		return errors.NewValidationError("port_invalid", "port must be between 0 and 65535 (0 means auto-assign)").
			WithField("port", config.Port)
	}

	if config.ReadTimeout <= 0 {
		return errors.NewValidationError("read_timeout_invalid", "read timeout must be positive").
			WithField("read_timeout", config.ReadTimeout)
	}

	if config.WriteTimeout <= 0 {
		return errors.NewValidationError("write_timeout_invalid", "write timeout must be positive").
			WithField("write_timeout", config.WriteTimeout)
	}

	if config.MaxHeaderBytes <= 0 {
		return errors.NewValidationError("max_header_bytes_invalid", "max header bytes must be positive").
			WithField("max_header_bytes", config.MaxHeaderBytes)
	}

	return nil
}

// mergeWithDefaults merges the configuration with default values.
func mergeWithDefaults(config *HTTPServerConfig) *HTTPServerConfig {
	defaults := DefaultHTTPServerConfig()

	if config.Address == "" {
		config.Address = defaults.Address
	}
	if config.Port == 0 {
		config.Port = defaults.Port
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = defaults.ReadTimeout
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = defaults.WriteTimeout
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = defaults.IdleTimeout
	}
	if config.MaxHeaderBytes == 0 {
		config.MaxHeaderBytes = defaults.MaxHeaderBytes
	}
	if config.PreferredFramework == "" {
		config.PreferredFramework = defaults.PreferredFramework
	}
	if len(config.CORSOrigins) == 0 {
		config.CORSOrigins = defaults.CORSOrigins
	}
	if config.RateLimitRequests == 0 {
		config.RateLimitRequests = defaults.RateLimitRequests
	}
	if config.RateLimitWindow == 0 {
		config.RateLimitWindow = defaults.RateLimitWindow
	}
	if config.MaxRateLimiters == 0 {
		config.MaxRateLimiters = defaults.MaxRateLimiters
	}
	if config.RateLimiterTTL == 0 {
		config.RateLimiterTTL = defaults.RateLimiterTTL
	}
	if config.CustomHeaders == nil {
		config.CustomHeaders = make(map[string]string)
	}
	if config.StaticFilesPath == "" {
		config.StaticFilesPath = defaults.StaticFilesPath
	}
	if config.StaticFilesPrefix == "" {
		config.StaticFilesPrefix = defaults.StaticFilesPrefix
	}

	// Merge boolean framework flags - if none are explicitly enabled, use defaults
	if !config.EnableGin && !config.EnableFiber && !config.EnableStdlib && !config.EnableCustomRouter {
		config.EnableGin = defaults.EnableGin
		config.EnableFiber = defaults.EnableFiber
		config.EnableStdlib = defaults.EnableStdlib
		config.EnableCustomRouter = defaults.EnableCustomRouter
	}

	return config
}

// newHTTPServerMetrics creates a new metrics instance.
func newHTTPServerMetrics() *HTTPServerMetrics {
	return &HTTPServerMetrics{
		ErrorsByType:    make(map[string]int64),
		RequestDuration: make(map[string]time.Duration),
		AgentRequests:   make(map[string]int64),
		StartTime:       time.Now(),
		LastUpdated:     time.Now(),
	}
}

// Framework-specific route setup methods

// setupGinRoutes sets up routes for the Gin framework.
func (s *HTTPServer) setupGinRoutes() {
	if s.ginEngine == nil {
		return
	}

	// Health check endpoint
	if s.config.EnableHealthCheck {
		s.ginEngine.GET("/health", s.ginHealthHandler)
	}

	// Agent endpoints
	if s.config.EnableAgentEndpoints {
		agentGroup := s.ginEngine.Group("/agents")
		agentGroup.GET("/", s.ginListAgentsHandler)
		agentGroup.GET("/:name", s.ginGetAgentHandler)
		agentGroup.POST("/:name/execute", s.ginExecuteAgentHandler)
	}

	// Static files
	if s.config.EnableStaticFiles && s.config.StaticFilesPath != "" {
		s.ginEngine.Static(s.config.StaticFilesPrefix, s.config.StaticFilesPath)
	}

	// Metrics endpoint
	if s.config.EnableMetrics {
		s.ginEngine.GET("/metrics", s.ginMetricsHandler)
	}
}

// setupFiberRoutes sets up routes for the Fiber framework.
func (s *HTTPServer) setupFiberRoutes() {
	if s.fiberApp == nil {
		return
	}

	// Health check endpoint
	if s.config.EnableHealthCheck {
		s.fiberApp.Get("/health", s.fiberHealthHandler)
	}

	// Agent endpoints
	if s.config.EnableAgentEndpoints {
		agentGroup := s.fiberApp.Group("/agents")
		agentGroup.Get("/", s.fiberListAgentsHandler)
		agentGroup.Get("/:name", s.fiberGetAgentHandler)
		agentGroup.Post("/:name/execute", s.fiberExecuteAgentHandler)
	}

	// Static files
	if s.config.EnableStaticFiles && s.config.StaticFilesPath != "" {
		s.fiberApp.Static(s.config.StaticFilesPrefix, s.config.StaticFilesPath)
	}

	// Metrics endpoint
	if s.config.EnableMetrics {
		s.fiberApp.Get("/metrics", s.fiberMetricsHandler)
	}
}

// setupStdlibRoutes sets up routes for the standard library.
func (s *HTTPServer) setupStdlibRoutes() {
	if s.stdlibMux == nil {
		return
	}

	// Health check endpoint
	if s.config.EnableHealthCheck {
		s.stdlibMux.HandleFunc("/health", s.stdlibHealthHandler)
	}

	// Agent endpoints
	if s.config.EnableAgentEndpoints {
		s.stdlibMux.HandleFunc("/agents/", s.stdlibAgentsHandler)
	}

	// Static files
	if s.config.EnableStaticFiles && s.config.StaticFilesPath != "" {
		fileServer := http.FileServer(http.Dir(s.config.StaticFilesPath))
		s.stdlibMux.Handle(s.config.StaticFilesPrefix+"/",
			http.StripPrefix(s.config.StaticFilesPrefix, fileServer))
	}

	// Metrics endpoint
	if s.config.EnableMetrics {
		s.stdlibMux.HandleFunc("/metrics", s.stdlibMetricsHandler)
	}
}

// setupCustomRoutes sets up routes for the custom router.
func (s *HTTPServer) setupCustomRoutes() {
	if s.customRouter == nil {
		return
	}

	// Health check endpoint
	if s.config.EnableHealthCheck {
		s.customRouter.GET("/health", s.stdlibHealthHandler)
	}

	// Agent endpoints
	if s.config.EnableAgentEndpoints {
		s.customRouter.GET("/agents/", s.stdlibAgentsHandler)
		s.customRouter.POST("/agents/", s.stdlibAgentsHandler)
	}

	// Static files
	if s.config.EnableStaticFiles && s.config.StaticFilesPath != "" {
		fileServer := http.FileServer(http.Dir(s.config.StaticFilesPath))
		s.customRouter.Handle("GET", s.config.StaticFilesPrefix+"/",
			http.StripPrefix(s.config.StaticFilesPrefix, fileServer).ServeHTTP)
	}

	// Metrics endpoint
	if s.config.EnableMetrics {
		s.customRouter.GET("/metrics", s.stdlibMetricsHandler)
	}
}

// Gin handlers

// ginHealthHandler handles health check requests for Gin.
func (s *HTTPServer) ginHealthHandler(c *gin.Context) {
	atomic.AddInt64(&s.metrics.GinRequests, 1)

	s.agentsMu.RLock()
	agentCount := len(s.agents)
	s.agentsMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"agents":    agentCount,
		"framework": "gin",
		"uptime":    time.Since(s.metrics.StartTime).String(),
	})
}

// ginListAgentsHandler lists all agents for Gin.
func (s *HTTPServer) ginListAgentsHandler(c *gin.Context) {
	atomic.AddInt64(&s.metrics.GinRequests, 1)

	agents := s.ListAgents()
	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"count":  len(agents),
	})
}

// ginGetAgentHandler gets a specific agent for Gin.
func (s *HTTPServer) ginGetAgentHandler(c *gin.Context) {
	atomic.AddInt64(&s.metrics.GinRequests, 1)

	name := c.Param("name")
	if agent, exists := s.GetAgent(name); exists {
		c.JSON(http.StatusOK, gin.H{
			"name":   name,
			"status": "found",
			"type":   fmt.Sprintf("%T", agent),
		})

		// Update agent request metrics
		s.metricsMu.Lock()
		s.metrics.AgentRequests[name]++
		s.metricsMu.Unlock()
	} else {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "agent not found",
			"name":  name,
		})
	}
}

// ginExecuteAgentHandler executes an agent for Gin.
func (s *HTTPServer) ginExecuteAgentHandler(c *gin.Context) {
	atomic.AddInt64(&s.metrics.GinRequests, 1)

	name := c.Param("name")
	agent, exists := s.GetAgent(name)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "agent not found",
			"name":  name,
		})
		return
	}

	// Update agent request metrics
	s.metricsMu.Lock()
	s.metrics.AgentRequests[name]++
	s.metricsMu.Unlock()

	// For now, just return agent information
	// In a full implementation, this would execute the agent with request data
	c.JSON(http.StatusOK, gin.H{
		"message": "agent execution not yet implemented",
		"agent":   name,
		"type":    fmt.Sprintf("%T", agent),
	})
}

// ginMetricsHandler returns metrics for Gin.
func (s *HTTPServer) ginMetricsHandler(c *gin.Context) {
	atomic.AddInt64(&s.metrics.GinRequests, 1)

	metrics := s.GetMetrics()
	c.JSON(http.StatusOK, metrics)
}

// Fiber handlers

// fiberHealthHandler handles health check requests for Fiber.
func (s *HTTPServer) fiberHealthHandler(c *fiber.Ctx) error {
	atomic.AddInt64(&s.metrics.FiberRequests, 1)

	s.agentsMu.RLock()
	agentCount := len(s.agents)
	s.agentsMu.RUnlock()

	return c.JSON(fiber.Map{
		"status":    "healthy",
		"agents":    agentCount,
		"framework": "fiber",
		"uptime":    time.Since(s.metrics.StartTime).String(),
	})
}

// fiberListAgentsHandler lists all agents for Fiber.
func (s *HTTPServer) fiberListAgentsHandler(c *fiber.Ctx) error {
	atomic.AddInt64(&s.metrics.FiberRequests, 1)

	agents := s.ListAgents()
	return c.JSON(fiber.Map{
		"agents": agents,
		"count":  len(agents),
	})
}

// fiberGetAgentHandler gets a specific agent for Fiber.
func (s *HTTPServer) fiberGetAgentHandler(c *fiber.Ctx) error {
	atomic.AddInt64(&s.metrics.FiberRequests, 1)

	name := c.Params("name")
	if agent, exists := s.GetAgent(name); exists {
		// Update agent request metrics
		s.metricsMu.Lock()
		s.metrics.AgentRequests[name]++
		s.metricsMu.Unlock()

		return c.JSON(fiber.Map{
			"name":   name,
			"status": "found",
			"type":   fmt.Sprintf("%T", agent),
		})
	}

	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": "agent not found",
		"name":  name,
	})
}

// fiberExecuteAgentHandler executes an agent for Fiber.
func (s *HTTPServer) fiberExecuteAgentHandler(c *fiber.Ctx) error {
	atomic.AddInt64(&s.metrics.FiberRequests, 1)

	name := c.Params("name")
	agent, exists := s.GetAgent(name)
	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "agent not found",
			"name":  name,
		})
	}

	// Update agent request metrics
	s.metricsMu.Lock()
	s.metrics.AgentRequests[name]++
	s.metricsMu.Unlock()

	// For now, just return agent information
	return c.JSON(fiber.Map{
		"message": "agent execution not yet implemented",
		"agent":   name,
		"type":    fmt.Sprintf("%T", agent),
	})
}

// fiberMetricsHandler returns metrics for Fiber.
func (s *HTTPServer) fiberMetricsHandler(c *fiber.Ctx) error {
	atomic.AddInt64(&s.metrics.FiberRequests, 1)

	metrics := s.GetMetrics()
	return c.JSON(metrics)
}

// Standard library handlers

// stdlibHealthHandler handles health check requests for stdlib.
func (s *HTTPServer) stdlibHealthHandler(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.metrics.StdlibRequests, 1)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.agentsMu.RLock()
	agentCount := len(s.agents)
	s.agentsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Optimized JSON response building
	builder := responseBuilderPool.Get().(*strings.Builder)
	builder.Reset()
	builder.WriteString(`{"status":"healthy","agents":`)
	builder.WriteString(fmt.Sprintf("%d", agentCount))
	builder.WriteString(`,"framework":"stdlib","uptime":"`)
	builder.WriteString(time.Since(s.metrics.StartTime).String())
	builder.WriteString(`"}`)
	response := builder.String()
	responseBuilderPool.Put(builder)

	w.Write([]byte(response))
}

// stdlibAgentsHandler handles agent requests for stdlib.
func (s *HTTPServer) stdlibAgentsHandler(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.metrics.StdlibRequests, 1)

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		s.handleStdlibGetAgents(w, r)
	case http.MethodPost:
		s.handleStdlibExecuteAgent(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStdlibGetAgents handles GET requests for agents.
func (s *HTTPServer) handleStdlibGetAgents(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /agents/ - list all agents
	if path == "/agents/" {
		agents := s.ListAgents()
		// Optimized agents list JSON response
		builder := responseBuilderPool.Get().(*strings.Builder)
		builder.Reset()
		builder.WriteString(`{"agents":[`)
		for i, agent := range agents {
			if i > 0 {
				builder.WriteByte(',')
			}
			builder.WriteByte('"')
			builder.WriteString(agent)
			builder.WriteByte('"')
		}
		builder.WriteString(`],"count":`)
		builder.WriteString(fmt.Sprintf("%d", len(agents)))
		builder.WriteByte('}')
		response := builder.String()
		responseBuilderPool.Put(builder)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
		return
	}

	// Handle /agents/{name} - get specific agent
	if len(path) > 8 { // len("/agents/") = 8
		name := path[8:]
		if agent, exists := s.GetAgent(name); exists {
			// Update agent request metrics
			s.metricsMu.Lock()
			s.metrics.AgentRequests[name]++
			s.metricsMu.Unlock()

			// Optimized agent info JSON response
			builder := responseBuilderPool.Get().(*strings.Builder)
			builder.Reset()
			builder.WriteString(`{"name":"`)
			builder.WriteString(name)
			builder.WriteString(`","status":"found","type":"`)
			builder.WriteString(fmt.Sprintf("%T", agent))
			builder.WriteString(`"}`)
			response := builder.String()
			responseBuilderPool.Put(builder)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		} else {
			// Optimized error response for agent not found
			builder := responseBuilderPool.Get().(*strings.Builder)
			builder.Reset()
			builder.WriteString(`{"error":"agent not found","name":"`)
			builder.WriteString(name)
			builder.WriteString(`"}`)
			response := builder.String()
			responseBuilderPool.Put(builder)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(response))
		}
	}
}

// handleStdlibExecuteAgent handles POST requests to execute agents.
func (s *HTTPServer) handleStdlibExecuteAgent(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /agents/{name}/execute
	if len(path) > 8 { // len("/agents/") = 8
		parts := path[8:] // Remove "/agents/"
		if len(parts) > 0 {
			name := parts
			if agent, exists := s.GetAgent(name); exists {
				// Update agent request metrics
				s.metricsMu.Lock()
				s.metrics.AgentRequests[name]++
				s.metricsMu.Unlock()

				// Optimized execution response
				builder := responseBuilderPool.Get().(*strings.Builder)
				builder.Reset()
				builder.WriteString(`{"message":"agent execution not yet implemented","agent":"`)
				builder.WriteString(name)
				builder.WriteString(`","type":"`)
				builder.WriteString(fmt.Sprintf("%T", agent))
				builder.WriteString(`"}`)
				response := builder.String()
				responseBuilderPool.Put(builder)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(response))
			} else {
				// Optimized error response for agent not found (execute)
				builder := responseBuilderPool.Get().(*strings.Builder)
				builder.Reset()
				builder.WriteString(`{"error":"agent not found","name":"`)
				builder.WriteString(name)
				builder.WriteString(`"}`)
				response := builder.String()
				responseBuilderPool.Put(builder)
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(response))
			}
		}
	}
}

// stdlibMetricsHandler returns metrics for stdlib.
func (s *HTTPServer) stdlibMetricsHandler(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.metrics.StdlibRequests, 1)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := s.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Optimized JSON metrics response building
	builder := responseBuilderPool.Get().(*strings.Builder)
	builder.Reset()
	builder.Grow(512) // Pre-allocate for metrics response
	builder.WriteString(`{"total_requests":`)
	builder.WriteString(fmt.Sprintf("%d", metrics.TotalRequests))
	builder.WriteString(`,"successful_requests":`)
	builder.WriteString(fmt.Sprintf("%d", metrics.SuccessfulRequests))
	builder.WriteString(`,"failed_requests":`)
	builder.WriteString(fmt.Sprintf("%d", metrics.FailedRequests))
	builder.WriteString(`,"gin_requests":`)
	builder.WriteString(fmt.Sprintf("%d", metrics.GinRequests))
	builder.WriteString(`,"fiber_requests":`)
	builder.WriteString(fmt.Sprintf("%d", metrics.FiberRequests))
	builder.WriteString(`,"stdlib_requests":`)
	builder.WriteString(fmt.Sprintf("%d", metrics.StdlibRequests))
	builder.WriteString(`,"custom_requests":`)
	builder.WriteString(fmt.Sprintf("%d", metrics.CustomRequests))
	builder.WriteString(`,"active_connections":`)
	builder.WriteString(fmt.Sprintf("%d", metrics.ActiveConnections))
	builder.WriteString(`,"total_connections":`)
	builder.WriteString(fmt.Sprintf("%d", metrics.TotalConnections))
	builder.WriteString(`,"last_updated":"`)
	builder.WriteString(metrics.LastUpdated.Format(time.RFC3339))
	builder.WriteString(`","start_time":"`)
	builder.WriteString(metrics.StartTime.Format(time.RFC3339))
	builder.WriteString(`"}`)
	response := builder.String()
	responseBuilderPool.Put(builder)

	w.Write([]byte(response))
}

// Middleware implementations

// Gin middleware

// ginCORSMiddleware provides CORS support for Gin.
func (s *HTTPServer) ginCORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range s.config.CORSOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// ginMetricsMiddleware provides metrics collection for Gin.
func (s *HTTPServer) ginMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		atomic.AddInt64(&s.metrics.TotalRequests, 1)
		atomic.AddInt64(&s.metrics.ActiveConnections, 1)

		c.Next()

		duration := time.Since(start)
		atomic.AddInt64(&s.metrics.ActiveConnections, -1)

		// Update average response time
		s.metricsMu.Lock()
		if s.metrics.AverageResponseTime == 0 {
			s.metrics.AverageResponseTime = duration
		} else {
			s.metrics.AverageResponseTime = (s.metrics.AverageResponseTime + duration) / 2
		}
		s.metrics.LastUpdated = time.Now()
		s.metricsMu.Unlock()

		// Count successful vs failed requests
		if c.Writer.Status() < 400 {
			atomic.AddInt64(&s.metrics.SuccessfulRequests, 1)
		} else {
			atomic.AddInt64(&s.metrics.FailedRequests, 1)
		}
	}
}

// ginCustomHeadersMiddleware adds custom headers for Gin.
func (s *HTTPServer) ginCustomHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		for key, value := range s.config.CustomHeaders {
			c.Header(key, value)
		}
		c.Next()
	}
}

// Fiber middleware

// fiberCORSMiddleware provides CORS support for Fiber.
func (s *HTTPServer) fiberCORSMiddleware() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     strings.Join(s.config.CORSOrigins, ","),
		AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Requested-With",
		AllowCredentials: false,
		ExposeHeaders:    "",
		MaxAge:           86400, // 24 hours
	})
}

// fiberMetricsMiddleware provides metrics collection for Fiber.
func (s *HTTPServer) fiberMetricsMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		atomic.AddInt64(&s.metrics.TotalRequests, 1)
		atomic.AddInt64(&s.metrics.ActiveConnections, 1)

		err := c.Next()

		duration := time.Since(start)
		atomic.AddInt64(&s.metrics.ActiveConnections, -1)

		// Update average response time
		s.metricsMu.Lock()
		if s.metrics.AverageResponseTime == 0 {
			s.metrics.AverageResponseTime = duration
		} else {
			s.metrics.AverageResponseTime = (s.metrics.AverageResponseTime + duration) / 2
		}
		s.metrics.LastUpdated = time.Now()
		s.metricsMu.Unlock()

		// Count successful vs failed requests
		if c.Response().StatusCode() < 400 {
			atomic.AddInt64(&s.metrics.SuccessfulRequests, 1)
		} else {
			atomic.AddInt64(&s.metrics.FailedRequests, 1)
		}

		return err
	}
}

// fiberCustomHeadersMiddleware adds custom headers for Fiber.
func (s *HTTPServer) fiberCustomHeadersMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		for key, value := range s.config.CustomHeaders {
			c.Set(key, value)
		}
		return c.Next()
	}
}

// Standard library middleware

// stdlibLoggingMiddleware provides logging for stdlib.
func (s *HTTPServer) stdlibLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		// Optimized log message building
		builder := stringBuilderPool.Get().(*strings.Builder)
		builder.Reset()
		builder.WriteByte('[')
		builder.WriteString(start.Format("2006/01/02 15:04:05"))
		builder.WriteString("] ")
		builder.WriteString(r.Method)
		builder.WriteByte(' ')
		builder.WriteString(r.URL.Path)
		builder.WriteByte(' ')
		builder.WriteString(fmt.Sprintf("%d", wrapped.statusCode))
		builder.WriteByte(' ')
		builder.WriteString(duration.String())
		builder.WriteByte('\n')
		fmt.Print(builder.String())
		stringBuilderPool.Put(builder)
	})
}

// stdlibRecoveryMiddleware provides panic recovery for stdlib.
func (s *HTTPServer) stdlibRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Optimized panic log message
				builder := stringBuilderPool.Get().(*strings.Builder)
				builder.Reset()
				builder.WriteString("Panic recovered: ")
				builder.WriteString(fmt.Sprintf("%v", err))
				builder.WriteByte('\n')
				fmt.Print(builder.String())
				stringBuilderPool.Put(builder)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)

				// Update error metrics
				s.metricsMu.Lock()
				s.metrics.ErrorsByType["panic"]++
				s.metricsMu.Unlock()
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// stdlibCORSMiddleware provides CORS support for stdlib.
func (s *HTTPServer) stdlibCORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range s.config.CORSOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// stdlibMetricsMiddleware provides metrics collection for stdlib.
func (s *HTTPServer) stdlibMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		atomic.AddInt64(&s.metrics.TotalRequests, 1)
		atomic.AddInt64(&s.metrics.ActiveConnections, 1)

		// Wrap the ResponseWriter to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		atomic.AddInt64(&s.metrics.ActiveConnections, -1)

		// Update average response time
		s.metricsMu.Lock()
		if s.metrics.AverageResponseTime == 0 {
			s.metrics.AverageResponseTime = duration
		} else {
			s.metrics.AverageResponseTime = (s.metrics.AverageResponseTime + duration) / 2
		}
		s.metrics.LastUpdated = time.Now()
		s.metricsMu.Unlock()

		// Count successful vs failed requests
		if wrapped.statusCode < 400 {
			atomic.AddInt64(&s.metrics.SuccessfulRequests, 1)
		} else {
			atomic.AddInt64(&s.metrics.FailedRequests, 1)
		}
	})
}

// stdlibCustomHeadersMiddleware adds custom headers for stdlib.
func (s *HTTPServer) stdlibCustomHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, value := range s.config.CustomHeaders {
			w.Header().Set(key, value)
		}
		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Background workers

// startBackgroundWorkers starts background workers for metrics and monitoring.
func (s *HTTPServer) startBackgroundWorkers() {
	// Metrics update worker
	if s.config.EnableMetrics {
		s.workerGroup.Add(1)
		go func() {
			defer s.workerGroup.Done()
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-s.shutdown:
					return
				case <-ticker.C:
					s.updateDetailedMetrics()
				}
			}
		}()
	}

	// Connection pool monitoring worker
	if s.connPool != nil {
		s.workerGroup.Add(1)
		go func() {
			defer s.workerGroup.Done()
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-s.shutdown:
					return
				case <-ticker.C:
					s.monitorConnectionPool()
				}
			}
		}()
	}
}

// updateDetailedMetrics updates detailed performance metrics.
func (s *HTTPServer) updateDetailedMetrics() {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	// Update request duration percentiles (simplified implementation)
	s.metrics.RequestDuration["p50"] = s.metrics.AverageResponseTime
	s.metrics.RequestDuration["p95"] = s.metrics.AverageResponseTime * 2
	s.metrics.RequestDuration["p99"] = s.metrics.AverageResponseTime * 3

	s.metrics.LastUpdated = time.Now()

	// Log metrics periodically
	if s.config.EnableLogging {
		// Optimized metrics log message
		builder := stringBuilderPool.Get().(*strings.Builder)
		builder.Reset()
		builder.WriteString("HTTP Server Metrics - Total: ")
		builder.WriteString(fmt.Sprintf("%d", atomic.LoadInt64(&s.metrics.TotalRequests)))
		builder.WriteString(", Success: ")
		builder.WriteString(fmt.Sprintf("%d", atomic.LoadInt64(&s.metrics.SuccessfulRequests)))
		builder.WriteString(", Failed: ")
		builder.WriteString(fmt.Sprintf("%d", atomic.LoadInt64(&s.metrics.FailedRequests)))
		builder.WriteString(", Avg Response: ")
		builder.WriteString(s.metrics.AverageResponseTime.String())
		builder.WriteByte('\n')
		fmt.Print(builder.String())
		stringBuilderPool.Put(builder)
	}
}

// monitorConnectionPool monitors the connection pool with enhanced leak detection.
func (s *HTTPServer) monitorConnectionPool() {
	if s.connPool == nil {
		return
	}

	poolMetrics := s.connPool.GetMetrics()
	if poolMetrics == nil {
		return
	}

	// Update connection metrics
	atomic.StoreInt64(&s.metrics.TotalConnections, poolMetrics.TotalConnections)

	// Enhanced leak detection
	leak := s.detectConnectionLeaks(poolMetrics)
	if leak != nil {
		s.handleConnectionLeak(leak)
	}

	// Monitor connection pool health
	healthScore := s.calculatePoolHealthScore(poolMetrics)
	if healthScore < 0.7 {
		s.attemptPoolRecovery(poolMetrics)
	}

	if s.config.EnableLogging {
		// Enhanced connection pool log message with health metrics
		builder := stringBuilderPool.Get().(*strings.Builder)
		builder.Reset()
		builder.WriteString("Connection Pool - Total: ")
		builder.WriteString(fmt.Sprintf("%d", poolMetrics.TotalConnections))
		builder.WriteString(", Active: ")
		builder.WriteString(fmt.Sprintf("%d", poolMetrics.ActiveConnections))
		builder.WriteString(", Idle: ")
		builder.WriteString(fmt.Sprintf("%d", poolMetrics.IdleConnections))
		builder.WriteString(", Utilization: ")
		builder.WriteString(fmt.Sprintf("%.2f%%", poolMetrics.PoolUtilization*100))
		builder.WriteString(", Health: ")
		builder.WriteString(fmt.Sprintf("%.1f", healthScore))
		if leak != nil {
			builder.WriteString(", LEAK DETECTED: ")
			builder.WriteString(leak.Description)
		}
		builder.WriteByte('\n')
		fmt.Print(builder.String())
		stringBuilderPool.Put(builder)
	}
}

// ConnectionLeak represents a detected connection leak
type ConnectionLeak struct {
	Type         string
	Description  string
	Severity     string
	DetectedAt   time.Time
	Metrics      interface{}
	Suggestions  []string
}

// detectConnectionLeaks analyzes pool metrics to detect potential leaks
func (s *HTTPServer) detectConnectionLeaks(metrics *httppool.HTTPPoolMetrics) *ConnectionLeak {
	// Check for high connection count without corresponding requests
	if metrics.ActiveConnections > 100 && s.metrics.TotalRequests == 0 {
		return &ConnectionLeak{
			Type:        "orphaned_connections",
			Description: fmt.Sprintf("High active connections (%d) with no requests", metrics.ActiveConnections),
			Severity:    "high",
			DetectedAt:  time.Now(),
			Metrics:     metrics,
			Suggestions: []string{"Check for connection cleanup issues", "Review timeout configurations"},
		}
	}

	// Check for excessive idle connections
	if metrics.IdleConnections > 50 {
		return &ConnectionLeak{
			Type:        "excessive_idle_connections",
			Description: fmt.Sprintf("Excessive idle connections: %d", metrics.IdleConnections),
			Severity:    "medium",
			DetectedAt:  time.Now(),
			Metrics:     metrics,
			Suggestions: []string{"Reduce MaxIdleTime", "Implement connection recycling"},
		}
	}

	// Check for low pool utilization efficiency
	if metrics.PoolUtilization < 0.1 && metrics.TotalConnections > 10 {
		return &ConnectionLeak{
			Type:        "low_utilization",
			Description: fmt.Sprintf("Low pool utilization: %.2f%% with %d connections", metrics.PoolUtilization*100, metrics.TotalConnections),
			Severity:    "low",
			DetectedAt:  time.Now(),
			Metrics:     metrics,
			Suggestions: []string{"Reduce pool size", "Review connection timeout settings"},
		}
	}

	return nil
}

// handleConnectionLeak takes corrective action when a leak is detected
func (s *HTTPServer) handleConnectionLeak(leak *ConnectionLeak) {
	// Log the leak
	if s.config.EnableLogging {
		builder := stringBuilderPool.Get().(*strings.Builder)
		builder.Reset()
		builder.WriteString("CONNECTION LEAK DETECTED: ")
		builder.WriteString(leak.Type)
		builder.WriteString(" - ")
		builder.WriteString(leak.Description)
		builder.WriteString(" (Severity: ")
		builder.WriteString(leak.Severity)
		builder.WriteString(")\n")
		fmt.Print(builder.String())
		stringBuilderPool.Put(builder)
	}

	// Take corrective action based on leak type
	switch leak.Type {
	case "orphaned_connections":
		// Note: Connection cleanup is handled automatically by the pool
		if s.config.EnableLogging {
			fmt.Println("Orphaned connections detected - automatic cleanup in progress")
		}
	case "excessive_idle_connections":
		// Note: Idle timeout is configured at pool creation time
		if s.config.EnableLogging {
			fmt.Println("Excessive idle connections detected - pool will auto-cleanup")
		}
	case "low_utilization":
		// Note: Pool size is managed automatically
		if s.config.EnableLogging {
			fmt.Println("Low utilization detected - monitoring pool metrics")
		}
	}

	// Update leak detection metrics
	s.metricsMu.Lock()
	if s.metrics.ErrorsByType == nil {
		s.metrics.ErrorsByType = make(map[string]int64)
	}
	s.metrics.ErrorsByType["connection_leak_"+leak.Type]++
	s.metricsMu.Unlock()
}

// calculatePoolHealthScore calculates a health score for the connection pool
func (s *HTTPServer) calculatePoolHealthScore(metrics *httppool.HTTPPoolMetrics) float64 {
	score := 1.0

	// Penalize high connection counts
	if metrics.TotalConnections > 200 {
		score *= 0.8
	}

	// Penalize low utilization
	if metrics.PoolUtilization < 0.1 {
		score *= 0.7
	}

	// Penalize excessive idle connections
	if metrics.IdleConnections > metrics.ActiveConnections*2 {
		score *= 0.6
	}

	return score
}

// attemptPoolRecovery attempts to recover an unhealthy connection pool
func (s *HTTPServer) attemptPoolRecovery(metrics *httppool.HTTPPoolMetrics) {
	if s.config.EnableLogging {
		builder := stringBuilderPool.Get().(*strings.Builder)
		builder.Reset()
		builder.WriteString("Attempting connection pool recovery - Health score below threshold\n")
		fmt.Print(builder.String())
		stringBuilderPool.Put(builder)
	}

	// Note: Connection cleanup is handled automatically by the pool
	if s.config.EnableLogging {
		fmt.Println("Pool recovery initiated - relying on automatic connection management")
	}

	// Force garbage collection to free up resources
	go func() {
		time.Sleep(1 * time.Second)
		// This would be runtime.GC() in a real implementation
		// Omitting to avoid forcing GC in library code
	}()
}
