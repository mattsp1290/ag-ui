// Package server provides example usage of the AG-UI server framework.
// This file demonstrates how to use the core server framework to create
// a production-ready AG-UI compatible server with agents and endpoints.
package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// ExampleAgent demonstrates a simple agent implementation.
type ExampleAgent struct {
	name        string
	description string
}

// NewExampleAgent creates a new example agent.
func NewExampleAgent(name, description string) *ExampleAgent {
	return &ExampleAgent{
		name:        name,
		description: description,
	}
}

// HandleEvent processes incoming events for the example agent.
func (a *ExampleAgent) HandleEvent(ctx context.Context, event any) ([]any, error) {
	// Simple echo response
	return []any{fmt.Sprintf("Echo from %s: %v", a.name, event)}, nil
}

// Name returns the agent's identifier.
func (a *ExampleAgent) Name() string {
	return a.name
}

// Description returns the agent's description.
func (a *ExampleAgent) Description() string {
	return a.description
}

// ExampleCustomHandler demonstrates a custom request handler.
type ExampleCustomHandler struct {
	message string
}

// Handle processes incoming requests.
func (h *ExampleCustomHandler) Handle(ctx context.Context, req *Request, resp ResponseWriter) error {
	response := map[string]interface{}{
		"message":   h.message,
		"path":      req.URL.Path,
		"method":    req.Method,
		"timestamp": time.Now().Unix(),
	}
	return resp.WriteJSON(response)
}

// Pattern returns the URL pattern this handler matches.
func (h *ExampleCustomHandler) Pattern() string {
	return "/api/custom"
}

// Methods returns the HTTP methods this handler supports.
func (h *ExampleCustomHandler) Methods() []string {
	return []string{"GET", "POST"}
}

// ExampleUsage demonstrates how to set up and use the server framework.
func ExampleUsage() error {
	// Create framework configuration
	config := DefaultFrameworkConfig()
	config.Name = "My AG-UI Server"
	config.HTTP.Port = 8081
	config.HTTP.Host = "localhost"

	// Create framework instance
	framework := NewFramework()

	// Initialize the framework
	ctx := context.Background()
	if err := framework.Initialize(ctx, config); err != nil {
		return errors.WithOperation("initialize", "framework", err)
	}

	// Create and register agents
	echoAgent := NewExampleAgent("echo-agent", "A simple echo agent")
	if err := framework.RegisterAgent(echoAgent); err != nil {
		return errors.WithOperation("register", "echo_agent", err)
	}

	processingAgent := NewExampleAgent("processing-agent", "A data processing agent")
	if err := framework.RegisterAgent(processingAgent); err != nil {
		return errors.WithOperation("register", "processing_agent", err)
	}

	// Register custom handlers
	customHandler := &ExampleCustomHandler{message: "Hello from custom handler!"}
	if err := framework.RegisterHandler("/api/custom", customHandler); err != nil {
		return errors.WithOperation("register", "custom_handler", err)
	}

	// Start the framework
	if err := framework.Start(ctx); err != nil {
		return errors.WithOperation("start", "framework", err)
	}

	log.Printf("Server started successfully!")
	log.Printf("Framework status: %+v", framework.GetStatus())
	log.Printf("Registered agents: %v", framework.ListAgents())

	// Wait for framework to be ready
	time.Sleep(100 * time.Millisecond)

	// Perform health check
	healthResult := framework.HealthCheck(ctx)
	log.Printf("Health check result: %+v", healthResult)

	// In a real application, you would run this until shutdown signal
	// For this example, we'll stop after a short time
	time.Sleep(1 * time.Second)

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := framework.Stop(shutdownCtx); err != nil {
		return errors.WithOperation("stop", "framework", err)
	}

	if err := framework.Shutdown(shutdownCtx); err != nil {
		return errors.WithOperation("shutdown", "framework", err)
	}

	log.Printf("Server shutdown completed successfully!")
	return nil
}

// ExampleConfigurationOptions demonstrates different configuration options.
func ExampleConfigurationOptions() *FrameworkConfig {
	config := DefaultFrameworkConfig()

	// Basic server settings
	config.Name = "Production AG-UI Server"
	config.Version = "2.0.0"
	config.Description = "Production-ready AG-UI server with full features"

	// HTTP configuration
	config.HTTP.Host = "0.0.0.0"
	config.HTTP.Port = 443
	config.HTTP.ReadTimeout = 60 * time.Second
	config.HTTP.WriteTimeout = 60 * time.Second
	config.HTTP.IdleTimeout = 300 * time.Second

	// Enable TLS
	config.HTTP.TLS.Enabled = true
	config.HTTP.TLS.CertFile = "/etc/ssl/certs/server.crt"
	config.HTTP.TLS.KeyFile = "/etc/ssl/private/server.key"

	// CORS configuration
	config.HTTP.CORS.Enabled = true
	config.HTTP.CORS.AllowOrigins = []string{"https://myapp.com", "https://admin.myapp.com"}
	config.HTTP.CORS.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.HTTP.CORS.AllowHeaders = []string{"Content-Type", "Authorization", "X-Requested-With"}

	// Agent management
	config.Agents.MaxAgents = 50
	config.Agents.DiscoveryEnabled = true
	config.Agents.DiscoveryInterval = 60 * time.Second
	config.Agents.HealthCheckEnabled = true
	config.Agents.HealthCheckTimeout = 30 * time.Second

	// Enable all middleware
	config.Middleware.EnableLogging = true
	config.Middleware.EnableMetrics = true
	config.Middleware.EnableRateLimit = true
	config.Middleware.EnableAuth = true
	config.Middleware.EnableCompression = true

	// Security settings
	config.Security.EnableHTTPS = true
	config.Security.AllowedOrigins = []string{"https://myapp.com"}
	config.Security.RequiredHeaders = []string{"Authorization"}
	config.Security.RateLimitPerMin = 500
	config.Security.MaxRequestSize = 5 * 1024 * 1024 // 5MB

	// Performance tuning
	config.Performance.MaxConcurrentRequests = 500
	config.Performance.RequestTimeout = 60 * time.Second
	config.Performance.WorkerPoolSize = 20
	config.Performance.EnableProfiling = true

	// Health check configuration
	config.HealthCheck.Enabled = true
	config.HealthCheck.Interval = 30 * time.Second
	config.HealthCheck.Timeout = 10 * time.Second
	config.HealthCheck.FailureThreshold = 3

	// Logging configuration
	config.Logging.Level = "info"
	config.Logging.Format = "json"
	config.Logging.OutputFile = "/var/log/ag-ui-server.log"

	return config
}

// RunExampleServer demonstrates how to run the example server.
// This is useful for testing and development purposes.
func RunExampleServer() {
	if err := ExampleUsage(); err != nil {
		log.Fatalf("Example server failed: %v", err)
	}
}