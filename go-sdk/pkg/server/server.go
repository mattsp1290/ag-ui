package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core"
	pkgerrors "github.com/ag-ui/go-sdk/pkg/errors"
)

// Server represents an AG-UI server that can host multiple agents.
type Server struct {
	config *Config
	agents map[string]core.Agent
	mu     sync.RWMutex

	// HTTP server instance
	httpServer *http.Server

	// Server state
	running bool
	closed  bool
}

// Config contains configuration options for the server.
type Config struct {
	// Address is the server listen address (e.g., ":8080")
	Address string

	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the next request
	IdleTimeout time.Duration

	// MaxHeaderBytes controls the maximum number of bytes the server will read
	MaxHeaderBytes int

	// TLS configuration
	TLSCertFile string
	TLSKeyFile  string

	// CORS settings
	CORSEnabled      bool
	CORSAllowOrigins []string
	CORSAllowMethods []string
	CORSAllowHeaders []string

	// Middleware options
	EnableLogging    bool
	EnableMetrics    bool
	EnableRecovery   bool
	EnableRateLimit  bool
	RateLimitRequest int
	RateLimitWindow  time.Duration
}

// New creates a new AG-UI server with the specified configuration.
func New(config Config) *Server {
	// Set default values
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 10 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Second
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 120 * time.Second
	}
	if config.MaxHeaderBytes == 0 {
		config.MaxHeaderBytes = 1 << 20 // 1MB
	}
	if config.RateLimitRequest == 0 {
		config.RateLimitRequest = 1000
	}
	if config.RateLimitWindow == 0 {
		config.RateLimitWindow = time.Minute
	}

	return &Server{
		config: &config,
		agents: make(map[string]core.Agent),
	}
}

// RegisterAgent registers an agent with the server under the specified name.
func (s *Server) RegisterAgent(name string, agent core.Agent) error {
	if name == "" {
		return pkgerrors.NewValidationErrorWithField("name", "required", "agent name cannot be empty", name)
	}
	if agent == nil {
		return pkgerrors.NewValidationErrorWithField("agent", "required", "agent cannot be nil", agent)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return pkgerrors.NewOperationError("RegisterAgent", "server", errors.New("server is closed"))
	}
	
	if _, exists := s.agents[name]; exists {
		return pkgerrors.NewResourceConflictError("agent", name, "agent already registered")
	}
	
	s.agents[name] = agent
	return nil
}

// UnregisterAgent removes an agent from the server.
func (s *Server) UnregisterAgent(name string) error {
	if name == "" {
		return pkgerrors.NewValidationErrorWithField("name", "required", "agent name cannot be empty", name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return pkgerrors.NewOperationError("UnregisterAgent", "server", errors.New("server is closed"))
	}
	
	if _, exists := s.agents[name]; !exists {
		return pkgerrors.NewResourceNotFoundError("agent", name)
	}
	
	delete(s.agents, name)
	return nil
}

// GetAgent retrieves a registered agent by name.
func (s *Server) GetAgent(name string) (core.Agent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, exists := s.agents[name]
	return agent, exists
}

// ListAgents returns a list of all registered agent names.
func (s *Server) ListAgents() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	agents := make([]string, 0, len(s.agents))
	for name := range s.agents {
		agents = append(agents, name)
	}
	return agents
}

// ListenAndServe starts the server and listens for incoming connections.
func (s *Server) ListenAndServe() error {
	if s.config.Address == "" {
		return pkgerrors.NewConfigurationErrorWithField("Address", "server address cannot be empty", s.config.Address)
	}
	
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return pkgerrors.NewOperationError("ListenAndServe", "server", errors.New("server is already running"))
	}
	if s.closed {
		s.mu.Unlock()
		return pkgerrors.NewOperationError("ListenAndServe", "server", errors.New("server is closed"))
	}
	s.running = true
	s.mu.Unlock()

	// Set up HTTP server with AG-UI protocol handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/ag-ui", s.handleAGUI)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/agents", s.handleAgents)

	s.httpServer = &http.Server{
		Addr:           s.config.Address,
		Handler:        mux,
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		IdleTimeout:    s.config.IdleTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
	}

	fmt.Printf("Starting AG-UI server on %s\n", s.config.Address)
	
	// Start server with or without TLS
	if s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
		return s.httpServer.ListenAndServeTLS(s.config.TLSCertFile, s.config.TLSKeyFile)
	}
	return s.httpServer.ListenAndServe()
}

// handleAGUI handles incoming AG-UI protocol requests.
func (s *Server) handleAGUI(w http.ResponseWriter, r *http.Request) {
	// Protocol handling, event routing, and response generation to be implemented
	// when the AG-UI protocol specification is finalized
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte(`{"error": "AG-UI protocol handler not implemented", "status": "not_implemented"}`))
}

// handleHealth handles health check requests.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	s.mu.RLock()
	agentCount := len(s.agents)
	running := s.running
	closed := s.closed
	s.mu.RUnlock()
	
	status := "healthy"
	if closed {
		status = "closed"
	} else if !running {
		status = "not_running"
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := fmt.Sprintf(`{"status": "%s", "agents": %d, "running": %t}`, status, agentCount, running)
	_, _ = w.Write([]byte(response))
}

// handleAgents handles agent listing requests.
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	agents := s.ListAgents()
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := fmt.Sprintf(`{"agents": %q}`, agents)
	_, _ = w.Write([]byte(response))
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return fmt.Errorf("server is already closed")
	}
	
	s.closed = true
	s.running = false
	
	// Shutdown HTTP server if it exists
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return pkgerrors.WrapWithContext(err, "Shutdown", "httpServer")
		}
	}
	
	// Clear agents
	s.agents = make(map[string]core.Agent)
	
	return nil
}

// IsRunning returns true if the server is currently running.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// IsClosed returns true if the server has been closed.
func (s *Server) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

// GetConfig returns a copy of the server configuration.
func (s *Server) GetConfig() Config {
	return *s.config
}
