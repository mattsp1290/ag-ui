package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// MiddlewareManager manages middleware chains and configurations
type MiddlewareManager struct {
	registry      MiddlewareRegistry
	chains        map[string]*MiddlewareChain
	defaultChain  string
	configWatcher *ConfigWatcher
	mu            sync.RWMutex
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
	data, err := os.ReadFile(configPath)
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
