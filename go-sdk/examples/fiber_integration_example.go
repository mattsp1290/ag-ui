package main

import (
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/server"
)

// ExampleFiberIntegration demonstrates how to use the HTTP server with Fiber framework.
func main() {
	// Example 1: Using HTTP server with automatic Fiber initialization
	log.Println("Example 1: Automatic Fiber initialization")

	config := server.DefaultHTTPServerConfig()
	config.Port = 8080
	config.EnableFiber = true
	config.EnableGin = false // Disable Gin to prefer Fiber
	config.PreferredFramework = server.FrameworkFiber

	httpServer, err := server.NewHTTPServer(config)
	if err != nil {
		log.Fatalf("Failed to create HTTP server: %v", err)
	}

	ctx := context.Background()
	if err := httpServer.Start(ctx); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}

	log.Println("HTTP Server with Fiber started on :8080")
	log.Println("Available endpoints:")
	log.Println("  GET /health - Health check")
	log.Println("  GET /agents - List agents")
	log.Println("  GET /metrics - Server metrics")

	// Run for a short time
	time.Sleep(2 * time.Second)

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Stop(shutdownCtx); err != nil {
		log.Printf("Error stopping HTTP server: %v", err)
	}

	log.Println("HTTP Server stopped")

	// Example 2: Using HTTP server with external Fiber app
	log.Println("\nExample 2: External Fiber app integration")

	// Create custom Fiber app
	app := fiber.New(fiber.Config{
		AppName:               "Custom AG-UI Fiber App",
		DisableStartupMessage: false,
	})

	// Add custom routes to Fiber app
	app.Get("/custom", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Custom Fiber route",
			"path":    c.Path(),
			"method":  c.Method(),
		})
	})

	// Create HTTP server configuration
	config2 := server.DefaultHTTPServerConfig()
	config2.Port = 8081

	httpServer2, err := server.NewHTTPServer(config2)
	if err != nil {
		log.Fatalf("Failed to create second HTTP server: %v", err)
	}

	// Register the custom Fiber app with HTTP server
	if err := httpServer2.RegisterWithFiber(app); err != nil {
		log.Fatalf("Failed to register Fiber app: %v", err)
	}

	if err := httpServer2.Start(ctx); err != nil {
		log.Fatalf("Failed to start second HTTP server: %v", err)
	}

	log.Println("HTTP Server with custom Fiber app started on :8081")
	log.Println("Available endpoints:")
	log.Println("  GET /health - Health check")
	log.Println("  GET /agents - List agents")
	log.Println("  GET /metrics - Server metrics")
	log.Println("  GET /custom - Custom Fiber route")

	// Run for a short time
	time.Sleep(2 * time.Second)

	// Graceful shutdown
	if err := httpServer2.Stop(shutdownCtx); err != nil {
		log.Printf("Error stopping second HTTP server: %v", err)
	}

	log.Println("Custom Fiber HTTP Server stopped")
	log.Println("Fiber integration examples completed successfully!")
}

// SetupFiberServer demonstrates the recommended way to setup a Fiber server
// as mentioned in the task requirements.
func SetupFiberServer() *fiber.App {
	app := fiber.New()
	agServer, err := server.NewHTTPServer(server.DefaultHTTPServerConfig())
	if err != nil {
		log.Fatalf("Failed to create AG server: %v", err)
	}

	if err := agServer.RegisterWithFiber(app); err != nil {
		log.Fatalf("Failed to register with Fiber: %v", err)
	}

	return app
}

// Note: This example shows the basic integration patterns.
// The Fiber framework provides high-performance HTTP server capabilities
// with built-in middleware, routing, and optimizations specifically
// designed for Go web applications.
