// Package middleware provides comprehensive middleware and interceptor system
// for request/response processing, authentication, authorization, logging,
// metrics collection, rate limiting, and custom transformations.
//
// The middleware system enables pluggable cross-cutting concerns for both
// client and server components without cluttering core business logic.
package middleware

import (
	"context"
	"errors"
	"time"
)

// Request represents a generic request that can be processed by middleware
type Request struct {
	ID        string                 `json:"id"`
	Method    string                 `json:"method"`
	Path      string                 `json:"path"`
	Headers   map[string]string      `json:"headers"`
	Body      interface{}            `json:"body"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

// Response represents a generic response that can be processed by middleware
type Response struct {
	ID         string                 `json:"id"`
	StatusCode int                    `json:"status_code"`
	Headers    map[string]string      `json:"headers"`
	Body       interface{}            `json:"body"`
	Error      error                  `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata"`
	Timestamp  time.Time              `json:"timestamp"`
	Duration   time.Duration          `json:"duration"`
}

// NextHandler represents the next middleware or final handler in the chain
type NextHandler func(ctx context.Context, req *Request) (*Response, error)

// Middleware represents the core middleware interface that all middleware must implement
type Middleware interface {
	// Name returns the middleware name for identification and debugging
	Name() string

	// Process processes a request through this middleware, calling next handler
	Process(ctx context.Context, req *Request, next NextHandler) (*Response, error)

	// Configure configures the middleware with provided parameters
	Configure(config map[string]interface{}) error

	// Enabled returns whether this middleware is currently enabled
	Enabled() bool

	// Priority returns the middleware priority (higher values execute first)
	Priority() int
}

// MiddlewareChain manages and executes a chain of middleware
type MiddlewareChain struct {
	middlewares []Middleware
	handler     Handler
}

// Handler represents the final handler at the end of middleware chain
type Handler func(ctx context.Context, req *Request) (*Response, error)

// NewMiddlewareChain creates a new middleware chain with the given final handler
func NewMiddlewareChain(handler Handler) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make([]Middleware, 0),
		handler:     handler,
	}
}

// Add adds middleware to the chain, maintaining priority order
func (c *MiddlewareChain) Add(middleware Middleware) {
	if middleware == nil {
		return
	}

	// Insert maintaining priority order (highest priority first)
	inserted := false
	for i, existing := range c.middlewares {
		if middleware.Priority() > existing.Priority() {
			c.middlewares = append(c.middlewares[:i], append([]Middleware{middleware}, c.middlewares[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		c.middlewares = append(c.middlewares, middleware)
	}
}

// Remove removes middleware from the chain by name
func (c *MiddlewareChain) Remove(name string) bool {
	for i, middleware := range c.middlewares {
		if middleware.Name() == name {
			c.middlewares = append(c.middlewares[:i], c.middlewares[i+1:]...)
			return true
		}
	}
	return false
}

// Clear removes all middleware from the chain
func (c *MiddlewareChain) Clear() {
	c.middlewares = c.middlewares[:0]
}

// Process executes the entire middleware chain for a request
func (c *MiddlewareChain) Process(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	// Set request timestamp if not already set
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now()
	}

	// Build the chain of next handlers
	return c.executeChain(ctx, req, 0)
}

// executeChain recursively executes the middleware chain
func (c *MiddlewareChain) executeChain(ctx context.Context, req *Request, index int) (*Response, error) {
	// If we've processed all middleware, call the final handler
	if index >= len(c.middlewares) {
		if c.handler == nil {
			return &Response{
				ID:         req.ID,
				StatusCode: 404,
				Error:      errors.New("no handler configured"),
				Timestamp:  time.Now(),
			}, nil
		}

		startTime := time.Now()
		resp, err := c.handler(ctx, req)
		if resp != nil && resp.Duration == 0 {
			resp.Duration = time.Since(startTime)
		}
		return resp, err
	}

	middleware := c.middlewares[index]

	// Skip disabled middleware
	if !middleware.Enabled() {
		return c.executeChain(ctx, req, index+1)
	}

	// Create the next handler for this middleware
	next := func(ctx context.Context, req *Request) (*Response, error) {
		return c.executeChain(ctx, req, index+1)
	}

	// Execute the middleware
	startTime := time.Now()
	resp, err := middleware.Process(ctx, req, next)
	if resp != nil && resp.Duration == 0 {
		resp.Duration = time.Since(startTime)
	}

	return resp, err
}

// ListMiddleware returns a list of all registered middleware names
func (c *MiddlewareChain) ListMiddleware() []string {
	names := make([]string, len(c.middlewares))
	for i, middleware := range c.middlewares {
		names[i] = middleware.Name()
	}
	return names
}

// GetMiddleware returns middleware by name
func (c *MiddlewareChain) GetMiddleware(name string) Middleware {
	for _, middleware := range c.middlewares {
		if middleware.Name() == name {
			return middleware
		}
	}
	return nil
}

// ConditionalMiddleware represents middleware that executes based on conditions
type ConditionalMiddleware interface {
	Middleware

	// ShouldExecute determines if this middleware should execute for the given request
	ShouldExecute(ctx context.Context, req *Request) bool
}

// AsyncMiddleware represents middleware that can execute asynchronously
type AsyncMiddleware interface {
	Middleware

	// ProcessAsync processes a request asynchronously
	ProcessAsync(ctx context.Context, req *Request, next NextHandler) <-chan *MiddlewareResult
}

// MiddlewareResult represents the result of async middleware processing
type MiddlewareResult struct {
	Response *Response
	Error    error
}

// MiddlewareContext provides context and utilities for middleware execution
type MiddlewareContext struct {
	context.Context

	// StartTime when the request processing started
	StartTime time.Time

	// RequestID unique identifier for this request
	RequestID string

	// Metadata additional context data
	Metadata map[string]interface{}
}

// NewMiddlewareContext creates a new middleware context
func NewMiddlewareContext(ctx context.Context, requestID string) *MiddlewareContext {
	return &MiddlewareContext{
		Context:   ctx,
		StartTime: time.Now(),
		RequestID: requestID,
		Metadata:  make(map[string]interface{}),
	}
}

// SetMetadata sets metadata value
func (mc *MiddlewareContext) SetMetadata(key string, value interface{}) {
	mc.Metadata[key] = value
}

// GetMetadata gets metadata value
func (mc *MiddlewareContext) GetMetadata(key string) (interface{}, bool) {
	value, exists := mc.Metadata[key]
	return value, exists
}

// Elapsed returns time elapsed since request start
func (mc *MiddlewareContext) Elapsed() time.Duration {
	return time.Since(mc.StartTime)
}

// MiddlewareConfig represents configuration for middleware
type MiddlewareConfig struct {
	Name     string                 `json:"name" yaml:"name"`
	Type     string                 `json:"type" yaml:"type"`
	Enabled  bool                   `json:"enabled" yaml:"enabled"`
	Priority int                    `json:"priority" yaml:"priority"`
	Config   map[string]interface{} `json:"config" yaml:"config"`
}

// MiddlewareFactory creates middleware instances from configuration
type MiddlewareFactory interface {
	// Create creates a new middleware instance from configuration
	Create(config *MiddlewareConfig) (Middleware, error)

	// SupportedTypes returns the middleware types this factory can create
	SupportedTypes() []string
}

// MiddlewareRegistry manages middleware factories and creation
type MiddlewareRegistry interface {
	// Register registers a middleware factory for a specific type
	Register(middlewareType string, factory MiddlewareFactory) error

	// Unregister removes a middleware factory
	Unregister(middlewareType string) error

	// Create creates middleware instance from configuration
	Create(config *MiddlewareConfig) (Middleware, error)

	// ListTypes returns all registered middleware types
	ListTypes() []string
}

// ErrorHandler handles middleware errors
type ErrorHandler interface {
	// HandleError processes an error from middleware execution
	HandleError(ctx context.Context, err error, req *Request) (*Response, error)
}

// MiddlewareLifecycle provides lifecycle hooks for middleware
type MiddlewareLifecycle interface {
	// Initialize initializes the middleware (called once at startup)
	Initialize(ctx context.Context) error

	// Shutdown gracefully shuts down the middleware
	Shutdown(ctx context.Context) error
}

// MetricsCollector collects middleware execution metrics
type MetricsCollector interface {
	// RecordDuration records middleware execution duration
	RecordDuration(middlewareName string, duration time.Duration)

	// RecordError records middleware error
	RecordError(middlewareName string, err error)

	// RecordRequest records request processing
	RecordRequest(middlewareName string, req *Request)

	// GetMetrics returns collected metrics
	GetMetrics() map[string]interface{}
}

// Common errors
var (
	ErrMiddlewareNotFound    = errors.New("middleware not found")
	ErrInvalidConfiguration  = errors.New("invalid middleware configuration")
	ErrMiddlewareNotEnabled  = errors.New("middleware not enabled")
	ErrChainExecutionFailed  = errors.New("middleware chain execution failed")
	ErrFactoryNotRegistered  = errors.New("middleware factory not registered")
	ErrInvalidMiddlewareType = errors.New("invalid middleware type")
)
