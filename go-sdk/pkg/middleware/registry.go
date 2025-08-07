package middleware

import (
	"fmt"
	"sort"
	"sync"
)

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

	// Advanced resilience middleware factories
	dmr.Register("resilience", NewResilienceMiddlewareFactory())
	dmr.Register("circuit_breaker", NewResilienceMiddlewareFactory())
	dmr.Register("retry", NewResilienceMiddlewareFactory())
}
