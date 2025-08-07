// Package server provides the foundational server framework architecture for AG-UI compatible endpoints.
// This framework implements production-ready lifecycle management, request routing, agent endpoint management,
// and graceful shutdown capabilities with a framework-agnostic design following the established patterns
// from the AG-UI Go SDK.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	pkgerrors "github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport"
)

// ==============================================================================
// CORE FRAMEWORK INTERFACES
// ==============================================================================

// ==============================================================================
// DECOMPOSED FRAMEWORK INTERFACES
// ==============================================================================

// FrameworkLifecycle manages the core lifecycle of the server framework.
type FrameworkLifecycle interface {
	// Initialize prepares the framework with the given configuration.
	Initialize(ctx context.Context, config *FrameworkConfig) error

	// Start begins the framework operation.
	Start(ctx context.Context) error

	// Stop gracefully stops the framework.
	Stop(ctx context.Context) error

	// Shutdown performs a complete shutdown and cleanup.
	Shutdown(ctx context.Context) error
}

// AgentRegistry manages agent registration and retrieval.
type AgentRegistry interface {
	// RegisterAgent registers an agent with the framework.
	RegisterAgent(agent core.Agent) error

	// UnregisterAgent removes an agent from the framework.
	UnregisterAgent(name string) error

	// GetAgent retrieves a registered agent by name.
	GetAgent(name string) (core.Agent, bool)

	// ListAgents returns information about all registered agents.
	ListAgents() []AgentInfo
}

// RouteRegistry manages request routing and middleware.
type RouteRegistry interface {
	// RegisterHandler registers a request handler for a specific pattern.
	RegisterHandler(pattern string, handler RequestHandler) error

	// RegisterMiddleware registers middleware with the framework.
	RegisterMiddleware(middleware Middleware) error
}

// FrameworkStatusProvider provides status and health information.
type FrameworkStatusProvider interface {
	// IsRunning returns true if the framework is currently running.
	IsRunning() bool

	// GetStatus returns the current framework status.
	GetStatus() FrameworkStatus

	// HealthCheck performs a comprehensive health check.
	HealthCheck(ctx context.Context) HealthCheckResult
}

// ==============================================================================
// COMPOSED INTERFACES FOR SPECIFIC USE CASES
// ==============================================================================

// MinimalFramework provides only lifecycle and status operations.
// Useful for basic server implementations that don't need agents or routing.
type MinimalFramework interface {
	FrameworkLifecycle
	FrameworkStatusProvider
}

// AgentFramework provides lifecycle, agent management, and status.
// Useful for agent-only servers without HTTP routing.
type AgentFramework interface {
	FrameworkLifecycle
	AgentRegistry
	FrameworkStatusProvider
}

// RoutingFramework provides lifecycle, routing, and status.
// Useful for HTTP-only servers without agent management.
type RoutingFramework interface {
	FrameworkLifecycle
	RouteRegistry
	FrameworkStatusProvider
}

// ServerFramework defines the complete interface for the AG-UI server framework.
// It composes all focused interfaces following the Interface Segregation Principle.
// Use this for full-featured server implementations.
type ServerFramework interface {
	FrameworkLifecycle
	AgentRegistry
	RouteRegistry
	FrameworkStatusProvider
}

// ==============================================================================
// DECOMPOSED REQUEST HANDLING INTERFACES
// ==============================================================================

// RequestProcessor handles the core request processing logic.
type RequestProcessor interface {
	// Handle processes an incoming request and returns a response
	Handle(ctx context.Context, req *Request, resp ResponseWriter) error
}

// RouteDescriptor provides routing information for handlers.
type RouteDescriptor interface {
	// Pattern returns the URL pattern this handler matches
	Pattern() string

	// Methods returns the HTTP methods this handler supports
	Methods() []string
}

// RequestHandler defines the interface for handling incoming requests.
// Composed of focused interfaces following the Interface Segregation Principle.
type RequestHandler interface {
	RequestProcessor
	RouteDescriptor
}

// MiddlewareProcessor handles the core middleware processing logic.
type MiddlewareProcessor interface {
	// Process intercepts and optionally modifies requests/responses
	Process(ctx context.Context, req *Request, resp ResponseWriter, next NextHandler) error
}

// MiddlewareDescriptor provides metadata about middleware.
type MiddlewareDescriptor interface {
	// Name returns the middleware name for identification
	Name() string

	// Priority returns the middleware priority (higher values execute first)
	Priority() int
}

// Middleware defines the interface for request/response middleware.
// Composed of focused interfaces following the Interface Segregation Principle.
type Middleware interface {
	MiddlewareProcessor
	MiddlewareDescriptor
}

// NextHandler represents the next handler in the middleware chain.
type NextHandler func(ctx context.Context, req *Request, resp ResponseWriter) error

// BasicResponseWriter provides core HTTP response writing functionality.
type BasicResponseWriter interface {
	// Header returns the response headers
	Header() http.Header

	// WriteHeader writes the HTTP status code
	WriteHeader(statusCode int)

	// Write writes response data
	Write(data []byte) (int, error)
}

// JSONResponseWriter provides JSON response writing capabilities.
type JSONResponseWriter interface {
	// WriteJSON writes a JSON response
	WriteJSON(data interface{}) error
}

// EventResponseWriter provides AG-UI event response writing capabilities.
type EventResponseWriter interface {
	// WriteEvent writes an AG-UI event response
	WriteEvent(event events.Event) error
}

// ResponseWriter defines the interface for writing HTTP responses.
// Composed of focused interfaces following the Interface Segregation Principle.
type ResponseWriter interface {
	BasicResponseWriter
	JSONResponseWriter
	EventResponseWriter
}

// ==============================================================================
// CONFIGURATION AND DATA STRUCTURES
// ==============================================================================

// FrameworkConfig contains configuration for the server framework.
type FrameworkConfig struct {
	// Server identification
	Name        string `json:"name" yaml:"name"`
	Version     string `json:"version" yaml:"version"`
	Description string `json:"description" yaml:"description"`

	// HTTP server configuration
	HTTP HTTPConfig `json:"http" yaml:"http"`

	// Agent management configuration
	Agents AgentManagerConfig `json:"agents" yaml:"agents"`

	// Middleware configuration
	Middleware MiddlewareConfig `json:"middleware" yaml:"middleware"`

	// Transport configuration
	Transport TransportConfig `json:"transport" yaml:"transport"`

	// Encoding configuration
	Encoding EncodingConfig `json:"encoding" yaml:"encoding"`

	// Health check configuration
	HealthCheck HealthCheckConfig `json:"health_check" yaml:"health_check"`

	// Security configuration
	Security FrameworkSecurityConfig `json:"security" yaml:"security"`

	// Logging configuration
	Logging LoggingConfig `json:"logging" yaml:"logging"`

	// Performance configuration
	Performance PerformanceConfig `json:"performance" yaml:"performance"`
}

// HTTPConfig contains HTTP server-specific configuration.
type HTTPConfig struct {
	Host         string        `json:"host" yaml:"host"`
	Port         int           `json:"port" yaml:"port"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout" yaml:"idle_timeout"`

	// TLS configuration
	TLS TLSConfig `json:"tls" yaml:"tls"`

	// CORS configuration
	CORS FrameworkCORSConfig `json:"cors" yaml:"cors"`
}

// TLSConfig contains TLS configuration.
type TLSConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	CertFile string `json:"cert_file" yaml:"cert_file"`
	KeyFile  string `json:"key_file" yaml:"key_file"`
}

// FrameworkCORSConfig contains CORS configuration for the framework.
type FrameworkCORSConfig struct {
	Enabled      bool     `json:"enabled" yaml:"enabled"`
	AllowOrigins []string `json:"allow_origins" yaml:"allow_origins"`
	AllowMethods []string `json:"allow_methods" yaml:"allow_methods"`
	AllowHeaders []string `json:"allow_headers" yaml:"allow_headers"`
}

// AgentManagerConfig contains agent management configuration.
type AgentManagerConfig struct {
	MaxAgents          int           `json:"max_agents" yaml:"max_agents"`
	DiscoveryEnabled   bool          `json:"discovery_enabled" yaml:"discovery_enabled"`
	DiscoveryInterval  time.Duration `json:"discovery_interval" yaml:"discovery_interval"`
	HealthCheckEnabled bool          `json:"health_check_enabled" yaml:"health_check_enabled"`
	HealthCheckTimeout time.Duration `json:"health_check_timeout" yaml:"health_check_timeout"`
}

// MiddlewareConfig contains middleware configuration.
type MiddlewareConfig struct {
	EnableLogging     bool `json:"enable_logging" yaml:"enable_logging"`
	EnableMetrics     bool `json:"enable_metrics" yaml:"enable_metrics"`
	EnableRateLimit   bool `json:"enable_rate_limit" yaml:"enable_rate_limit"`
	EnableAuth        bool `json:"enable_auth" yaml:"enable_auth"`
	EnableCompression bool `json:"enable_compression" yaml:"enable_compression"`
}

// TransportConfig contains transport configuration.
type TransportConfig struct {
	DefaultType string                 `json:"default_type" yaml:"default_type"`
	Transports  map[string]interface{} `json:"transports" yaml:"transports"`
}

// EncodingConfig contains encoding configuration.
type EncodingConfig struct {
	DefaultFormat string                 `json:"default_format" yaml:"default_format"`
	Formats       map[string]interface{} `json:"formats" yaml:"formats"`
}

// HealthCheckConfig contains health check configuration.
type HealthCheckConfig struct {
	Enabled          bool          `json:"enabled" yaml:"enabled"`
	Interval         time.Duration `json:"interval" yaml:"interval"`
	Timeout          time.Duration `json:"timeout" yaml:"timeout"`
	FailureThreshold int           `json:"failure_threshold" yaml:"failure_threshold"`
}

// FrameworkSecurityConfig contains security configuration for the framework.
type FrameworkSecurityConfig struct {
	EnableHTTPS     bool     `json:"enable_https" yaml:"enable_https"`
	AllowedOrigins  []string `json:"allowed_origins" yaml:"allowed_origins"`
	RequiredHeaders []string `json:"required_headers" yaml:"required_headers"`
	RateLimitPerMin int      `json:"rate_limit_per_min" yaml:"rate_limit_per_min"`
	MaxRequestSize  int64    `json:"max_request_size" yaml:"max_request_size"`
}

// LoggingConfig contains logging configuration.
type LoggingConfig struct {
	Level      string `json:"level" yaml:"level"`
	Format     string `json:"format" yaml:"format"`
	OutputFile string `json:"output_file" yaml:"output_file"`
}

// PerformanceConfig contains performance tuning configuration.
type PerformanceConfig struct {
	MaxConcurrentRequests int           `json:"max_concurrent_requests" yaml:"max_concurrent_requests"`
	RequestTimeout        time.Duration `json:"request_timeout" yaml:"request_timeout"`
	WorkerPoolSize        int           `json:"worker_pool_size" yaml:"worker_pool_size"`
	EnableProfiling       bool          `json:"enable_profiling" yaml:"enable_profiling"`
}

// Request represents an incoming HTTP request with AG-UI extensions.
type Request struct {
	*http.Request

	// AG-UI specific fields
	AgentName string                 `json:"agent_name,omitempty"`
	Event     events.Event           `json:"event,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// AgentInfo provides information about a registered agent.
type AgentInfo struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Status       string                 `json:"status"`
	Capabilities []string               `json:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	LastSeen     time.Time              `json:"last_seen"`
}

// FrameworkStatus represents the current status of the framework.
type FrameworkStatus struct {
	State        FrameworkState         `json:"state"`
	StartTime    time.Time              `json:"start_time"`
	Uptime       time.Duration          `json:"uptime"`
	AgentCount   int                    `json:"agent_count"`
	RequestCount int64                  `json:"request_count"`
	ErrorCount   int64                  `json:"error_count"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// FrameworkState represents the lifecycle state of the framework.
type FrameworkState int32

const (
	StateUninitialized FrameworkState = iota
	StateInitialized
	StateStarting
	StateRunning
	StateStopping
	StateStopped
	StateShutdown
	StateError
)

// String returns the string representation of the framework state.
func (s FrameworkState) String() string {
	switch s {
	case StateUninitialized:
		return "uninitialized"
	case StateInitialized:
		return "initialized"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateShutdown:
		return "shutdown"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// HealthCheckResult represents the result of a health check.
type HealthCheckResult struct {
	Healthy   bool                   `json:"healthy"`
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
	Checks    map[string]CheckResult `json:"checks"`
	Errors    []string               `json:"errors,omitempty"`
}

// CheckResult represents the result of an individual health check.
type CheckResult struct {
	Name     string        `json:"name"`
	Healthy  bool          `json:"healthy"`
	Duration time.Duration `json:"duration"`
	Message  string        `json:"message,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// ==============================================================================
// FRAMEWORK IMPLEMENTATION
// ==============================================================================

// BaseFramework provides the core implementation of the ServerFramework interface.
type BaseFramework struct {
	// Configuration
	config *FrameworkConfig

	// State management
	state     int32 // atomic access
	startTime time.Time
	mu        sync.RWMutex

	// Agent management
	agents   map[string]core.Agent
	agentsMu sync.RWMutex

	// Request handling
	handlers    map[string]RequestHandler
	middlewares []Middleware
	handlersMu  sync.RWMutex

	// HTTP server
	httpServer *http.Server
	serverMux  *http.ServeMux

	// Transport and encoding
	transportManager transport.TransportManager
	encodingRegistry encoding.CodecFactory

	// Metrics
	requestCount int64
	errorCount   int64

	// Lifecycle channels
	stopCh     chan struct{}
	shutdownCh chan struct{}
	doneCh     chan struct{}
}

// NewFramework creates a new server framework instance.
func NewFramework() *BaseFramework {
	return &BaseFramework{
		agents:      make(map[string]core.Agent),
		handlers:    make(map[string]RequestHandler),
		middlewares: make([]Middleware, 0),
		serverMux:   http.NewServeMux(),
		stopCh:      make(chan struct{}),
		shutdownCh:  make(chan struct{}),
		doneCh:      make(chan struct{}),
	}
}

// Initialize prepares the framework with the given configuration.
func (f *BaseFramework) Initialize(ctx context.Context, config *FrameworkConfig) error {
	if !f.compareAndSwapState(StateUninitialized, StateInitialized) {
		return pkgerrors.NewStateError("invalid_state", "framework already initialized").
			WithTransition("uninitialized -> initialized")
	}

	if config == nil {
		config = DefaultFrameworkConfig()
	}

	if err := f.validateConfig(config); err != nil {
		atomic.StoreInt32(&f.state, int32(StateError))
		return pkgerrors.NewValidationError("config_invalid", "invalid configuration").WithCause(err)
	}

	f.mu.Lock()
	f.config = config
	f.mu.Unlock()

	// Initialize components
	if err := f.initializeComponents(ctx); err != nil {
		atomic.StoreInt32(&f.state, int32(StateError))
		return pkgerrors.NewStateError("init_failed", "failed to initialize components").WithCause(err)
	}

	// Register default handlers
	if err := f.registerDefaultHandlers(); err != nil {
		atomic.StoreInt32(&f.state, int32(StateError))
		return pkgerrors.NewStateError("handlers_failed", "failed to register default handlers").WithCause(err)
	}

	// Register default middleware
	if err := f.registerDefaultMiddleware(); err != nil {
		atomic.StoreInt32(&f.state, int32(StateError))
		return pkgerrors.NewStateError("middleware_failed", "failed to register default middleware").WithCause(err)
	}

	return nil
}

// Start begins the framework operation.
func (f *BaseFramework) Start(ctx context.Context) error {
	if !f.compareAndSwapState(StateInitialized, StateStarting) &&
		!f.compareAndSwapState(StateStopped, StateStarting) {
		return pkgerrors.NewStateError("invalid_state", "framework not in a startable state").
			WithTransition("initialized|stopped -> starting")
	}

	f.mu.Lock()
	f.startTime = time.Now()
	f.mu.Unlock()

	// Create HTTP server
	if err := f.createHTTPServer(); err != nil {
		atomic.StoreInt32(&f.state, int32(StateError))
		return pkgerrors.NewStateError("server_create_failed", "failed to create HTTP server").WithCause(err)
	}

	// Start HTTP server in a goroutine
	go f.runHTTPServer()

	// Wait for server to be ready or context cancellation
	select {
	case <-ctx.Done():
		atomic.StoreInt32(&f.state, int32(StateError))
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		// Give server time to start
	}

	if !f.compareAndSwapState(StateStarting, StateRunning) {
		return pkgerrors.NewStateError("state_transition_failed", "failed to transition to running state")
	}

	return nil
}

// Stop gracefully stops the framework.
func (f *BaseFramework) Stop(ctx context.Context) error {
	if !f.compareAndSwapState(StateRunning, StateStopping) {
		// If already stopped or in stopping state, return success
		currentState := FrameworkState(atomic.LoadInt32(&f.state))
		if currentState == StateStopped || currentState == StateStopping {
			return nil
		}
		return pkgerrors.NewStateError("invalid_state", "framework not running").
			WithTransition("running -> stopping")
	}

	// Signal stop (only close if not already closed)
	select {
	case <-f.stopCh:
		// Already closed
	default:
		close(f.stopCh)
	}

	// Stop HTTP server
	if f.httpServer != nil {
		if err := f.httpServer.Shutdown(ctx); err != nil {
			return pkgerrors.NewStateError("server_stop_failed", "failed to stop HTTP server").WithCause(err)
		}
	}

	// Stop transport manager
	if f.transportManager != nil {
		if err := f.transportManager.Close(ctx); err != nil {
			return pkgerrors.NewStateError("transport_stop_failed", "failed to stop transport manager").WithCause(err)
		}
	}

	if !f.compareAndSwapState(StateStopping, StateStopped) {
		return pkgerrors.NewStateError("state_transition_failed", "failed to transition to stopped state")
	}

	return nil
}

// Shutdown performs a complete shutdown and cleanup.
func (f *BaseFramework) Shutdown(ctx context.Context) error {
	currentState := FrameworkState(atomic.LoadInt32(&f.state))

	// If running, stop first
	if currentState == StateRunning {
		if err := f.Stop(ctx); err != nil {
			return err
		}
	}

	if !f.compareAndSwapState(StateStopped, StateShutdown) &&
		!f.compareAndSwapState(StateError, StateShutdown) {
		return pkgerrors.NewStateError("invalid_state", "framework not in a shutdownable state").
			WithTransition("stopped|error -> shutdown")
	}

	// Signal shutdown (only close if not already closed)
	select {
	case <-f.shutdownCh:
		// Already closed
	default:
		close(f.shutdownCh)
	}

	// Cleanup resources
	f.cleanup()

	// Signal completion (only close if not already closed)
	select {
	case <-f.doneCh:
		// Already closed
	default:
		close(f.doneCh)
	}

	return nil
}

// RegisterAgent registers an agent with the framework.
func (f *BaseFramework) RegisterAgent(agent core.Agent) error {
	if agent == nil {
		return pkgerrors.NewValidationError("agent_required", "agent cannot be nil")
	}

	name := agent.Name()
	if name == "" {
		return pkgerrors.NewValidationError("agent_name_required", "agent name cannot be empty")
	}

	f.agentsMu.Lock()
	defer f.agentsMu.Unlock()

	if _, exists := f.agents[name]; exists {
		return pkgerrors.NewConflictError("agent_exists", "agent already registered").
			WithResource("agent", name)
	}

	// Check max agents limit
	f.mu.RLock()
	maxAgents := f.config.Agents.MaxAgents
	f.mu.RUnlock()

	if maxAgents > 0 && len(f.agents) >= maxAgents {
		return pkgerrors.NewStateError("max_agents_reached", "maximum number of agents reached").
			WithDetail("max_agents", maxAgents).
			WithDetail("current_count", len(f.agents))
	}

	f.agents[name] = agent
	return nil
}

// UnregisterAgent removes an agent from the framework.
func (f *BaseFramework) UnregisterAgent(name string) error {
	if name == "" {
		return pkgerrors.NewValidationError("agent_name_required", "agent name cannot be empty")
	}

	f.agentsMu.Lock()
	defer f.agentsMu.Unlock()

	if _, exists := f.agents[name]; !exists {
		return pkgerrors.NewStateError("agent_not_found", "agent not registered").
			WithDetail("agent_name", name)
	}

	delete(f.agents, name)
	return nil
}

// GetAgent retrieves a registered agent by name.
func (f *BaseFramework) GetAgent(name string) (core.Agent, bool) {
	f.agentsMu.RLock()
	defer f.agentsMu.RUnlock()

	agent, exists := f.agents[name]
	return agent, exists
}

// ListAgents returns information about all registered agents.
func (f *BaseFramework) ListAgents() []AgentInfo {
	f.agentsMu.RLock()
	defer f.agentsMu.RUnlock()

	agents := make([]AgentInfo, 0, len(f.agents))
	for name, agent := range f.agents {
		info := AgentInfo{
			Name:        name,
			Description: agent.Description(),
			Status:      "active", // TODO: implement actual status tracking
			LastSeen:    time.Now(),
		}
		agents = append(agents, info)
	}

	return agents
}

// RegisterHandler registers a request handler for a specific pattern.
func (f *BaseFramework) RegisterHandler(pattern string, handler RequestHandler) error {
	if pattern == "" {
		return pkgerrors.NewValidationError("pattern_required", "handler pattern cannot be empty")
	}
	if handler == nil {
		return pkgerrors.NewValidationError("handler_required", "handler cannot be nil")
	}

	f.handlersMu.Lock()
	defer f.handlersMu.Unlock()

	if _, exists := f.handlers[pattern]; exists {
		return pkgerrors.NewConflictError("handler_exists", "handler already registered for pattern").
			WithResource("pattern", pattern)
	}

	f.handlers[pattern] = handler

	// Register with HTTP mux
	f.serverMux.HandleFunc(pattern, f.createHTTPHandler(handler))

	return nil
}

// RegisterMiddleware registers middleware with the framework.
func (f *BaseFramework) RegisterMiddleware(middleware Middleware) error {
	if middleware == nil {
		return pkgerrors.NewValidationError("middleware_required", "middleware cannot be nil")
	}

	f.handlersMu.Lock()
	defer f.handlersMu.Unlock()

	f.middlewares = append(f.middlewares, middleware)

	// Sort middlewares by priority (higher priority first)
	sort.Slice(f.middlewares, func(i, j int) bool {
		return f.middlewares[i].Priority() > f.middlewares[j].Priority()
	})

	return nil
}

// IsRunning returns true if the framework is currently running.
func (f *BaseFramework) IsRunning() bool {
	return FrameworkState(atomic.LoadInt32(&f.state)) == StateRunning
}

// GetStatus returns the current framework status.
func (f *BaseFramework) GetStatus() FrameworkStatus {
	f.mu.RLock()
	startTime := f.startTime
	f.mu.RUnlock()

	f.agentsMu.RLock()
	agentCount := len(f.agents)
	f.agentsMu.RUnlock()

	return FrameworkStatus{
		State:        FrameworkState(atomic.LoadInt32(&f.state)),
		StartTime:    startTime,
		Uptime:       time.Since(startTime),
		AgentCount:   agentCount,
		RequestCount: atomic.LoadInt64(&f.requestCount),
		ErrorCount:   atomic.LoadInt64(&f.errorCount),
	}
}

// HealthCheck performs a comprehensive health check.
func (f *BaseFramework) HealthCheck(ctx context.Context) HealthCheckResult {
	start := time.Now()
	checks := make(map[string]CheckResult)
	errors := make([]string, 0)
	healthy := true

	// Check framework state
	state := FrameworkState(atomic.LoadInt32(&f.state))
	checks["framework_state"] = CheckResult{
		Name:     "framework_state",
		Healthy:  state == StateRunning,
		Duration: time.Since(start),
		Message:  fmt.Sprintf("Framework state: %s", state.String()),
	}
	if state != StateRunning {
		healthy = false
		errors = append(errors, fmt.Sprintf("Framework not running: %s", state.String()))
	}

	// Check HTTP server
	if f.httpServer != nil {
		checks["http_server"] = CheckResult{
			Name:     "http_server",
			Healthy:  true,
			Duration: time.Since(start),
			Message:  "HTTP server is running",
		}
	} else {
		checks["http_server"] = CheckResult{
			Name:     "http_server",
			Healthy:  false,
			Duration: time.Since(start),
			Error:    "HTTP server not initialized",
		}
		healthy = false
		errors = append(errors, "HTTP server not initialized")
	}

	// Check agents
	f.agentsMu.RLock()
	agentCount := len(f.agents)
	f.agentsMu.RUnlock()

	checks["agents"] = CheckResult{
		Name:     "agents",
		Healthy:  true,
		Duration: time.Since(start),
		Message:  fmt.Sprintf("Registered agents: %d", agentCount),
	}

	return HealthCheckResult{
		Healthy: healthy,
		Status: func() string {
			if healthy {
				return "healthy"
			} else {
				return "unhealthy"
			}
		}(),
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Checks:    checks,
		Errors:    errors,
	}
}

// ==============================================================================
// PRIVATE HELPER METHODS
// ==============================================================================

// compareAndSwapState atomically compares and swaps the framework state.
func (f *BaseFramework) compareAndSwapState(old, new FrameworkState) bool {
	return atomic.CompareAndSwapInt32(&f.state, int32(old), int32(new))
}

// validateConfig validates the framework configuration.
func (f *BaseFramework) validateConfig(config *FrameworkConfig) error {
	if config.Name == "" {
		return fmt.Errorf("framework name cannot be empty")
	}

	if config.HTTP.Port < 0 || config.HTTP.Port > 65535 {
		return fmt.Errorf("invalid HTTP port: %d (must be 0-65535, 0 means auto-assign)", config.HTTP.Port)
	}

	if config.HTTP.ReadTimeout < 0 {
		return fmt.Errorf("invalid read timeout: %v", config.HTTP.ReadTimeout)
	}

	if config.HTTP.WriteTimeout < 0 {
		return fmt.Errorf("invalid write timeout: %v", config.HTTP.WriteTimeout)
	}

	return nil
}

// initializeComponents initializes framework components.
func (f *BaseFramework) initializeComponents(ctx context.Context) error {
	// Initialize transport manager if needed
	// TODO: Implement transport manager initialization

	// Initialize encoding registry if needed
	// TODO: Implement encoding registry initialization

	return nil
}

// registerDefaultHandlers registers the default framework handlers.
func (f *BaseFramework) registerDefaultHandlers() error {
	// Register health check handler
	healthHandler := &HealthCheckHandler{framework: f}
	if err := f.RegisterHandler("/health", healthHandler); err != nil {
		return err
	}

	// Register agents handler
	agentsHandler := &AgentsHandler{framework: f}
	if err := f.RegisterHandler("/agents", agentsHandler); err != nil {
		return err
	}

	// Register status handler
	statusHandler := &StatusHandler{framework: f}
	if err := f.RegisterHandler("/status", statusHandler); err != nil {
		return err
	}

	return nil
}

// registerDefaultMiddleware registers the default framework middleware.
func (f *BaseFramework) registerDefaultMiddleware() error {
	f.mu.RLock()
	config := f.config
	f.mu.RUnlock()

	// Register logging middleware
	if config.Middleware.EnableLogging {
		loggingMiddleware := &LoggingMiddleware{}
		if err := f.RegisterMiddleware(loggingMiddleware); err != nil {
			return err
		}
	}

	// Register metrics middleware
	if config.Middleware.EnableMetrics {
		metricsMiddleware := &MetricsMiddleware{framework: f}
		if err := f.RegisterMiddleware(metricsMiddleware); err != nil {
			return err
		}
	}

	// Register CORS middleware
	if config.HTTP.CORS.Enabled {
		corsMiddleware := &CORSMiddleware{config: config.HTTP.CORS}
		if err := f.RegisterMiddleware(corsMiddleware); err != nil {
			return err
		}
	}

	return nil
}

// createHTTPServer creates and configures the HTTP server.
func (f *BaseFramework) createHTTPServer() error {
	f.mu.RLock()
	config := f.config
	f.mu.RUnlock()

	address := fmt.Sprintf("%s:%d", config.HTTP.Host, config.HTTP.Port)

	f.httpServer = &http.Server{
		Addr:         address,
		Handler:      f.serverMux,
		ReadTimeout:  config.HTTP.ReadTimeout,
		WriteTimeout: config.HTTP.WriteTimeout,
		IdleTimeout:  config.HTTP.IdleTimeout,
	}

	return nil
}

// runHTTPServer starts the HTTP server.
func (f *BaseFramework) runHTTPServer() {
	f.mu.RLock()
	config := f.config
	f.mu.RUnlock()

	var err error
	if config.HTTP.TLS.Enabled {
		err = f.httpServer.ListenAndServeTLS(config.HTTP.TLS.CertFile, config.HTTP.TLS.KeyFile)
	} else {
		err = f.httpServer.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		atomic.StoreInt32(&f.state, int32(StateError))
	}
}

// createHTTPHandler creates an HTTP handler that wraps a RequestHandler with middleware.
func (f *BaseFramework) createHTTPHandler(handler RequestHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Increment request counter
		atomic.AddInt64(&f.requestCount, 1)

		// Create AG-UI request wrapper
		req := &Request{Request: r}

		// Create response writer wrapper
		respWriter := &HTTPResponseWriter{ResponseWriter: w}

		// Build middleware chain
		next := func(ctx context.Context, req *Request, resp ResponseWriter) error {
			return handler.Handle(ctx, req, resp)
		}

		// Apply middleware in reverse order (since we're building the chain)
		f.handlersMu.RLock()
		middlewares := make([]Middleware, len(f.middlewares))
		copy(middlewares, f.middlewares)
		f.handlersMu.RUnlock()

		for i := len(middlewares) - 1; i >= 0; i-- {
			middleware := middlewares[i]
			nextHandler := next
			next = func(ctx context.Context, req *Request, resp ResponseWriter) error {
				return middleware.Process(ctx, req, resp, nextHandler)
			}
		}

		// Execute the handler chain
		if err := next(r.Context(), req, respWriter); err != nil {
			atomic.AddInt64(&f.errorCount, 1)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// cleanup performs resource cleanup.
func (f *BaseFramework) cleanup() {
	// Clear agents
	f.agentsMu.Lock()
	f.agents = make(map[string]core.Agent)
	f.agentsMu.Unlock()

	// Clear handlers
	f.handlersMu.Lock()
	f.handlers = make(map[string]RequestHandler)
	f.middlewares = make([]Middleware, 0)
	f.handlersMu.Unlock()
}

// ==============================================================================
// DEFAULT HANDLERS
// ==============================================================================

// HealthCheckHandler handles health check requests.
type HealthCheckHandler struct {
	framework *BaseFramework
}

// Handle processes health check requests.
func (h *HealthCheckHandler) Handle(ctx context.Context, req *Request, resp ResponseWriter) error {
	result := h.framework.HealthCheck(ctx)
	return resp.WriteJSON(result)
}

// Pattern returns the URL pattern for this handler.
func (h *HealthCheckHandler) Pattern() string {
	return "/health"
}

// Methods returns the HTTP methods this handler supports.
func (h *HealthCheckHandler) Methods() []string {
	return []string{"GET"}
}

// AgentsHandler handles agent listing requests.
type AgentsHandler struct {
	framework *BaseFramework
}

// Handle processes agent listing requests.
func (h *AgentsHandler) Handle(ctx context.Context, req *Request, resp ResponseWriter) error {
	agents := h.framework.ListAgents()
	return resp.WriteJSON(map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	})
}

// Pattern returns the URL pattern for this handler.
func (h *AgentsHandler) Pattern() string {
	return "/agents"
}

// Methods returns the HTTP methods this handler supports.
func (h *AgentsHandler) Methods() []string {
	return []string{"GET"}
}

// StatusHandler handles framework status requests.
type StatusHandler struct {
	framework *BaseFramework
}

// Handle processes status requests.
func (h *StatusHandler) Handle(ctx context.Context, req *Request, resp ResponseWriter) error {
	status := h.framework.GetStatus()
	return resp.WriteJSON(status)
}

// Pattern returns the URL pattern for this handler.
func (h *StatusHandler) Pattern() string {
	return "/status"
}

// Methods returns the HTTP methods this handler supports.
func (h *StatusHandler) Methods() []string {
	return []string{"GET"}
}

// ==============================================================================
// DEFAULT MIDDLEWARE
// ==============================================================================

// LoggingMiddleware provides request/response logging.
type LoggingMiddleware struct{}

// Process logs request details and calls the next handler.
func (m *LoggingMiddleware) Process(ctx context.Context, req *Request, resp ResponseWriter, next NextHandler) error {
	start := time.Now()

	// Log request
	fmt.Printf("[%s] %s %s - Started\n", start.Format("2006-01-02 15:04:05"), req.Method, req.URL.Path)

	err := next(ctx, req, resp)

	// Log response
	duration := time.Since(start)
	status := "SUCCESS"
	if err != nil {
		status = "ERROR"
	}

	fmt.Printf("[%s] %s %s - %s (%v)\n",
		time.Now().Format("2006-01-02 15:04:05"),
		req.Method,
		req.URL.Path,
		status,
		duration)

	return err
}

// Name returns the middleware name.
func (m *LoggingMiddleware) Name() string {
	return "logging"
}

// Priority returns the middleware priority.
func (m *LoggingMiddleware) Priority() int {
	return 100 // High priority to log everything
}

// MetricsMiddleware provides request metrics collection.
type MetricsMiddleware struct {
	framework *BaseFramework
}

// Process collects metrics and calls the next handler.
func (m *MetricsMiddleware) Process(ctx context.Context, req *Request, resp ResponseWriter, next NextHandler) error {
	start := time.Now()

	err := next(ctx, req, resp)

	// Update metrics
	atomic.AddInt64(&m.framework.requestCount, 1)
	if err != nil {
		atomic.AddInt64(&m.framework.errorCount, 1)
	}

	// TODO: Add more detailed metrics collection (response time, status codes, etc.)
	_ = start // Placeholder until we implement detailed metrics

	return err
}

// Name returns the middleware name.
func (m *MetricsMiddleware) Name() string {
	return "metrics"
}

// Priority returns the middleware priority.
func (m *MetricsMiddleware) Priority() int {
	return 90 // High priority for accurate metrics
}

// CORSMiddleware provides CORS support.
type CORSMiddleware struct {
	config FrameworkCORSConfig
}

// Process adds CORS headers and handles preflight requests.
func (m *CORSMiddleware) Process(ctx context.Context, req *Request, resp ResponseWriter, next NextHandler) error {
	// Add CORS headers
	headers := resp.Header()

	if len(m.config.AllowOrigins) > 0 {
		origin := req.Header.Get("Origin")
		for _, allowedOrigin := range m.config.AllowOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				headers.Set("Access-Control-Allow-Origin", allowedOrigin)
				break
			}
		}
	}

	if len(m.config.AllowMethods) > 0 {
		methods := ""
		for i, method := range m.config.AllowMethods {
			if i > 0 {
				methods += ", "
			}
			methods += method
		}
		headers.Set("Access-Control-Allow-Methods", methods)
	}

	if len(m.config.AllowHeaders) > 0 {
		headersStr := ""
		for i, header := range m.config.AllowHeaders {
			if i > 0 {
				headersStr += ", "
			}
			headersStr += header
		}
		headers.Set("Access-Control-Allow-Headers", headersStr)
	}

	// Handle preflight requests
	if req.Method == "OPTIONS" {
		resp.WriteHeader(http.StatusOK)
		return nil
	}

	return next(ctx, req, resp)
}

// Name returns the middleware name.
func (m *CORSMiddleware) Name() string {
	return "cors"
}

// Priority returns the middleware priority.
func (m *CORSMiddleware) Priority() int {
	return 80 // High priority to set headers early
}

// ==============================================================================
// RESPONSE WRITER IMPLEMENTATION
// ==============================================================================

// HTTPResponseWriter implements ResponseWriter for HTTP responses.
type HTTPResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader writes the HTTP status code.
func (w *HTTPResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// WriteJSON writes a JSON response.
func (w *HTTPResponseWriter) WriteJSON(data interface{}) error {
	w.Header().Set("Content-Type", "application/json")

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ") // Pretty print JSON

	if err := encoder.Encode(data); err != nil {
		return pkgerrors.NewEncodingError("json_encode_failed", "failed to encode JSON response").
			WithCause(err).
			WithOperation("encode").
			WithFormat("json")
	}

	return nil
}

// WriteEvent writes an AG-UI event response.
func (w *HTTPResponseWriter) WriteEvent(event events.Event) error {
	if event == nil {
		return pkgerrors.NewValidationError("event_required", "event cannot be nil")
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-AG-UI-Event-Type", string(event.Type()))

	// Get timestamp
	timestamp := int64(0)
	if ts := event.Timestamp(); ts != nil {
		timestamp = *ts
	}

	// Convert event to JSON using the event's ToJSON method
	if jsonData, err := event.ToJSON(); err == nil {
		w.Write(jsonData)
		return nil
	}

	// Fallback: create basic event structure
	eventData := map[string]interface{}{
		"type":      event.Type(),
		"timestamp": timestamp,
		"thread_id": event.ThreadID(),
		"run_id":    event.RunID(),
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(eventData); err != nil {
		return pkgerrors.NewEncodingError("event_encode_failed", "failed to encode event response").
			WithCause(err).
			WithOperation("encode").
			WithFormat("json")
	}

	return nil
}

// ==============================================================================
// UTILITY FUNCTIONS
// ==============================================================================

// NewFrameworkFromConfig creates a new framework instance with the given configuration.
func NewFrameworkFromConfig(config *FrameworkConfig) (*BaseFramework, error) {
	framework := NewFramework()

	if err := framework.Initialize(context.Background(), config); err != nil {
		return nil, err
	}

	return framework, nil
}

// ==============================================================================
// DEFAULT CONFIGURATION FUNCTION
// ==============================================================================

// DefaultFrameworkConfig returns a default framework configuration.
func DefaultFrameworkConfig() *FrameworkConfig {
	return &FrameworkConfig{
		Name:        "ag-ui-server",
		Version:     "1.0.0",
		Description: "AG-UI Server Framework",
		HTTP: HTTPConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
			TLS: TLSConfig{
				Enabled: false,
			},
			CORS: FrameworkCORSConfig{
				Enabled:      true,
				AllowOrigins: []string{"*"},
				AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowHeaders: []string{"Content-Type", "Authorization"},
			},
		},
		Agents: AgentManagerConfig{
			MaxAgents:          100,
			DiscoveryEnabled:   true,
			DiscoveryInterval:  30 * time.Second,
			HealthCheckEnabled: true,
			HealthCheckTimeout: 10 * time.Second,
		},
		Middleware: MiddlewareConfig{
			EnableLogging:     true,
			EnableMetrics:     true,
			EnableRateLimit:   false,
			EnableAuth:        false,
			EnableCompression: true,
		},
		Transport: TransportConfig{
			DefaultType: "http",
			Transports:  make(map[string]interface{}),
		},
		Encoding: EncodingConfig{
			DefaultFormat: "application/json",
			Formats:       make(map[string]interface{}),
		},
		HealthCheck: HealthCheckConfig{
			Enabled:          true,
			Interval:         30 * time.Second,
			Timeout:          10 * time.Second,
			FailureThreshold: 3,
		},
		Security: FrameworkSecurityConfig{
			EnableHTTPS:     false,
			AllowedOrigins:  []string{"*"},
			RequiredHeaders: []string{},
			RateLimitPerMin: 1000,
			MaxRequestSize:  10 * 1024 * 1024, // 10MB
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Performance: PerformanceConfig{
			MaxConcurrentRequests: 1000,
			RequestTimeout:        30 * time.Second,
			WorkerPoolSize:        10,
			EnableProfiling:       false,
		},
	}
}
