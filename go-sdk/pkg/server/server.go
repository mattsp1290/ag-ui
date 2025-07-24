package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/ag-ui/go-sdk/pkg/core"
)

// Server represents an AG-UI server that can host multiple agents.
type Server struct {
	config *Config
	agents map[string]core.Agent
	mu     sync.RWMutex

	// TODO: Add transport handlers, middleware stack, and connection management
}

// Config contains configuration options for the server.
type Config struct {
	// Address is the server listen address (e.g., ":8080")
	Address string

	// TODO: Add TLS configuration, CORS settings, middleware options, etc.
}

// Validate validates the server configuration
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("server config cannot be nil")
	}
	
	if c.Address == "" {
		return fmt.Errorf("server address cannot be empty")
	}
	
	// Validate address format - should be in the format ":port" or "host:port"
	if !strings.HasPrefix(c.Address, ":") && !strings.Contains(c.Address, ":") {
		return fmt.Errorf("server address must be in format ':port' or 'host:port', got %q", c.Address)
	}
	
	return nil
}

// New creates a new AG-UI server with the specified configuration.
func New(config Config) (*Server, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid server configuration: %w", err)
	}
	
	return &Server{
		config: &config,
		agents: make(map[string]core.Agent),
	}, nil
}

// RegisterAgent registers an agent with the server under the specified name.
func (s *Server) RegisterAgent(name string, agent core.Agent) error {
	if s == nil {
		return fmt.Errorf("server cannot be nil")
	}
	if name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}
	if agent == nil {
		return fmt.Errorf("agent cannot be nil")
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.agents == nil {
		s.agents = make(map[string]core.Agent)
	}
	
	// Check for duplicate registration
	if _, exists := s.agents[name]; exists {
		return fmt.Errorf("agent with name %q is already registered", name)
	}
	
	s.agents[name] = agent
	return nil
}

// UnregisterAgent removes an agent from the server.
func (s *Server) UnregisterAgent(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agents, name)
}

// GetAgent retrieves a registered agent by name.
func (s *Server) GetAgent(name string) (core.Agent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, exists := s.agents[name]
	return agent, exists
}

// ListenAndServe starts the server and listens for incoming connections.
func (s *Server) ListenAndServe() error {
	// TODO: Implement HTTP server setup with AG-UI protocol handlers
	http.HandleFunc("/ag-ui", s.handleAGUI)

	fmt.Printf("Starting AG-UI server on %s\n", s.config.Address)
	return http.ListenAndServe(s.config.Address, nil)
}

// handleAGUI handles incoming AG-UI protocol requests.
func (s *Server) handleAGUI(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement protocol handling, event routing, and response generation
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte("AG-UI protocol handler not implemented")) // Ignore error on response write
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	// TODO: Implement graceful shutdown
	return fmt.Errorf("not implemented")
}
