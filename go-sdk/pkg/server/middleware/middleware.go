// Package middleware provides a comprehensive middleware framework for HTTP servers
// with multiple middleware implementations for authentication, CORS, logging,
// metrics, and rate limiting.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Middleware is the core interface that all middleware must implement.
// It follows the standard HTTP middleware pattern for framework-agnostic usage.
type Middleware interface {
	// Handler wraps an HTTP handler with middleware functionality
	Handler(next http.Handler) http.Handler

	// Name returns the middleware name for debugging and logging
	Name() string

	// Priority returns the middleware priority for ordering (higher = earlier)
	Priority() int

	// Config returns the middleware configuration
	Config() interface{}

	// Cleanup performs any necessary cleanup when the middleware is destroyed
	Cleanup() error
}

// MiddlewareFunc is a function type that implements the Middleware interface
type MiddlewareFunc func(http.Handler) http.Handler

// Handler implements the Middleware interface for MiddlewareFunc
func (mf MiddlewareFunc) Handler(next http.Handler) http.Handler {
	return mf(next)
}

// Name returns a default name for function middleware
func (mf MiddlewareFunc) Name() string {
	return "func-middleware"
}

// Priority returns default priority for function middleware
func (mf MiddlewareFunc) Priority() int {
	return 0
}

// Config returns nil for function middleware
func (mf MiddlewareFunc) Config() interface{} {
	return nil
}

// Cleanup does nothing for function middleware
func (mf MiddlewareFunc) Cleanup() error {
	return nil
}

// Chain represents a chain of middleware that can be executed in sequence
type Chain struct {
	middlewares []Middleware
	mu          sync.RWMutex
	compiled    http.Handler
	dirty       bool
	logger      *zap.Logger
}

// NewChain creates a new middleware chain
func NewChain(logger *zap.Logger) *Chain {
	if logger == nil {
		logger = zap.NewNop()
	}

	c := &Chain{
		middlewares: make([]Middleware, 0),
		logger:      logger,
		dirty:       true,
	}
	// Explicitly initialize the mutex to ensure proper state
	c.mu = sync.RWMutex{}
	return c
}

// Use adds middleware to the chain
func (c *Chain) Use(middleware ...Middleware) *Chain {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.middlewares = append(c.middlewares, middleware...)
	c.dirty = true

	c.logger.Debug("Added middleware to chain",
		zap.Int("middleware_count", len(middleware)),
		zap.Int("total_count", len(c.middlewares)),
	)

	return c
}

// Remove removes middleware from the chain by name
func (c *Chain) Remove(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, mw := range c.middlewares {
		if mw.Name() == name {
			c.middlewares = append(c.middlewares[:i], c.middlewares[i+1:]...)
			c.dirty = true

			c.logger.Debug("Removed middleware from chain",
				zap.String("middleware_name", name),
				zap.Int("remaining_count", len(c.middlewares)),
			)

			return true
		}
	}

	return false
}

// Clear removes all middleware from the chain
func (c *Chain) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Cleanup existing middleware
	for _, mw := range c.middlewares {
		if err := mw.Cleanup(); err != nil {
			c.logger.Warn("Error cleaning up middleware",
				zap.String("middleware_name", mw.Name()),
				zap.Error(err),
			)
		}
	}

	c.middlewares = make([]Middleware, 0)
	c.compiled = nil
	c.dirty = true

	c.logger.Debug("Cleared all middleware from chain")
}

// List returns a copy of the middleware list
func (c *Chain) List() []Middleware {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Middleware, len(c.middlewares))
	copy(result, c.middlewares)
	return result
}

// Compile compiles the middleware chain into a single handler
func (c *Chain) Compile() http.Handler {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dirty && c.compiled != nil {
		return c.compiled
	}

	// Sort middleware by priority (higher priority first)
	sortedMiddleware := make([]Middleware, len(c.middlewares))
	copy(sortedMiddleware, c.middlewares)

	// Simple bubble sort by priority
	for i := 0; i < len(sortedMiddleware)-1; i++ {
		for j := 0; j < len(sortedMiddleware)-i-1; j++ {
			if sortedMiddleware[j].Priority() < sortedMiddleware[j+1].Priority() {
				sortedMiddleware[j], sortedMiddleware[j+1] = sortedMiddleware[j+1], sortedMiddleware[j]
			}
		}
	}

	// Build the chain from the end - use local variable to avoid multiple writes to c.compiled
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default handler if no final handler is set
		w.WriteHeader(http.StatusNotFound)
	})

	// Apply middleware in reverse order (last middleware wraps first)
	for i := len(sortedMiddleware) - 1; i >= 0; i-- {
		mw := sortedMiddleware[i]
		handler = mw.Handler(handler)

		c.logger.Debug("Applied middleware to chain",
			zap.String("middleware_name", mw.Name()),
			zap.Int("priority", mw.Priority()),
		)
	}

	// Assign the completed handler chain only once
	c.compiled = handler
	c.dirty = false
	return c.compiled
}

// Handler returns a handler that executes the middleware chain followed by the final handler
func (c *Chain) Handler(final http.Handler) http.Handler {
	if final == nil {
		final = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Sort middleware by priority (higher priority first)
	sortedMiddleware := make([]Middleware, len(c.middlewares))
	copy(sortedMiddleware, c.middlewares)

	// Simple bubble sort by priority
	for i := 0; i < len(sortedMiddleware)-1; i++ {
		for j := 0; j < len(sortedMiddleware)-i-1; j++ {
			if sortedMiddleware[j].Priority() < sortedMiddleware[j+1].Priority() {
				sortedMiddleware[j], sortedMiddleware[j+1] = sortedMiddleware[j+1], sortedMiddleware[j]
			}
		}
	}

	// Build the chain from the end
	handler := final

	// Apply middleware in reverse order (last middleware wraps first)
	for i := len(sortedMiddleware) - 1; i >= 0; i-- {
		mw := sortedMiddleware[i]
		handler = mw.Handler(handler)
	}

	return handler
}

// Cleanup cleans up all middleware in the chain
func (c *Chain) Cleanup() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errors []error

	for _, mw := range c.middlewares {
		if err := mw.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("cleanup error for %s: %w", mw.Name(), err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple cleanup errors: %v", errors)
	}

	return nil
}

// Manager manages multiple middleware chains and provides lifecycle management
type Manager struct {
	chains map[string]*Chain
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewManager creates a new middleware manager
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Manager{
		chains: make(map[string]*Chain),
		logger: logger,
	}
}

// CreateChain creates a new named middleware chain
func (m *Manager) CreateChain(name string) *Chain {
	m.mu.Lock()
	defer m.mu.Unlock()

	chain := NewChain(m.logger)
	m.chains[name] = chain

	m.logger.Debug("Created middleware chain",
		zap.String("chain_name", name),
	)

	return chain
}

// GetChain retrieves a middleware chain by name
func (m *Manager) GetChain(name string) (*Chain, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chain, exists := m.chains[name]
	return chain, exists
}

// RemoveChain removes a middleware chain by name
func (m *Manager) RemoveChain(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	chain, exists := m.chains[name]
	if !exists {
		return false
	}

	// Cleanup the chain
	if err := chain.Cleanup(); err != nil {
		m.logger.Warn("Error cleaning up chain",
			zap.String("chain_name", name),
			zap.Error(err),
		)
	}

	delete(m.chains, name)

	m.logger.Debug("Removed middleware chain",
		zap.String("chain_name", name),
	)

	return true
}

// ListChains returns a list of all chain names
func (m *Manager) ListChains() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.chains))
	for name := range m.chains {
		names = append(names, name)
	}

	return names
}

// Cleanup cleans up all chains
func (m *Manager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	for name, chain := range m.chains {
		if err := chain.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("cleanup error for chain %s: %w", name, err))
		}
	}

	m.chains = make(map[string]*Chain)

	if len(errors) > 0 {
		return fmt.Errorf("multiple cleanup errors: %v", errors)
	}

	return nil
}

// Common middleware context keys
type contextKey string

const (
	// RequestIDKey is the context key for request ID
	RequestIDKey contextKey = "request_id"

	// UserIDKey is the context key for user ID
	UserIDKey contextKey = "user_id"

	// AuthContextKey is the context key for authentication context
	AuthContextKey contextKey = "auth_context"

	// MetricsKey is the context key for metrics data
	MetricsKey contextKey = "metrics"

	// LoggerKey is the context key for request-scoped logger
	LoggerKey contextKey = "logger"

	// StartTimeKey is the context key for request start time
	StartTimeKey contextKey = "start_time"
)

// Helper functions for context management

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// SetRequestID sets request ID in context
func SetRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}

// SetUserID sets user ID in context
func SetUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// GetLogger extracts logger from context
func GetLogger(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(LoggerKey).(*zap.Logger); ok {
		return logger
	}
	return zap.NewNop()
}

// SetLogger sets logger in context
func SetLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, logger)
}

// GetStartTime extracts request start time from context
func GetStartTime(ctx context.Context) time.Time {
	if t, ok := ctx.Value(StartTimeKey).(time.Time); ok {
		return t
	}
	return time.Time{}
}

// SetStartTime sets request start time in context
func SetStartTime(ctx context.Context, startTime time.Time) context.Context {
	return context.WithValue(ctx, StartTimeKey, startTime)
}

// ResponseWriter wraps http.ResponseWriter to capture response data
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

// NewResponseWriter creates a new response writer wrapper
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader captures the status code
func (rw *ResponseWriter) WriteHeader(code int) {
	if rw.statusCode == http.StatusOK {
		rw.statusCode = code
	}
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size
func (rw *ResponseWriter) Write(data []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(data)
	rw.written += int64(n)
	return n, err
}

// Status returns the response status code
func (rw *ResponseWriter) Status() int {
	return rw.statusCode
}

// Written returns the number of bytes written
func (rw *ResponseWriter) Written() int64 {
	return rw.written
}

// Common middleware configuration structures

// BaseConfig contains common configuration for all middleware
type BaseConfig struct {
	// Enabled indicates if the middleware is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Priority sets the middleware execution priority
	Priority int `json:"priority" yaml:"priority"`

	// Name is the middleware name
	Name string `json:"name" yaml:"name"`

	// Debug enables debug logging
	Debug bool `json:"debug" yaml:"debug"`
}

// ValidateBaseConfig validates common middleware configuration
func ValidateBaseConfig(config *BaseConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	if config.Name == "" {
		return fmt.Errorf("middleware name cannot be empty")
	}

	return nil
}

// ConditionalMiddleware wraps middleware with conditional execution
type ConditionalMiddleware struct {
	middleware Middleware
	condition  func(*http.Request) bool
	logger     *zap.Logger
}

// NewConditionalMiddleware creates middleware that only executes when condition is true
func NewConditionalMiddleware(middleware Middleware, condition func(*http.Request) bool, logger *zap.Logger) *ConditionalMiddleware {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &ConditionalMiddleware{
		middleware: middleware,
		condition:  condition,
		logger:     logger,
	}
}

// Handler implements the Middleware interface
func (cm *ConditionalMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cm.condition(r) {
			cm.middleware.Handler(next).ServeHTTP(w, r)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

// Name returns the wrapped middleware name
func (cm *ConditionalMiddleware) Name() string {
	return "conditional-" + cm.middleware.Name()
}

// Priority returns the wrapped middleware priority
func (cm *ConditionalMiddleware) Priority() int {
	return cm.middleware.Priority()
}

// Config returns the wrapped middleware config
func (cm *ConditionalMiddleware) Config() interface{} {
	return cm.middleware.Config()
}

// Cleanup cleans up the wrapped middleware
func (cm *ConditionalMiddleware) Cleanup() error {
	return cm.middleware.Cleanup()
}

// Utility functions for common middleware patterns

// GenerateRequestID generates a unique request ID
func GenerateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

// GetClientIP extracts client IP from request
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Take the first IP in the list
		if idx := len(forwarded); idx != -1 {
			for i, char := range forwarded {
				if char == ',' {
					return forwarded[:i]
				}
			}
		}
		return forwarded
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// IsWebSocket checks if the request is a WebSocket upgrade request
func IsWebSocket(r *http.Request) bool {
	return r.Header.Get("Upgrade") == "websocket" &&
		r.Header.Get("Connection") == "Upgrade"
}

// IsAJAXRequest checks if the request is an AJAX request
func IsAJAXRequest(r *http.Request) bool {
	return r.Header.Get("X-Requested-With") == "XMLHttpRequest"
}
