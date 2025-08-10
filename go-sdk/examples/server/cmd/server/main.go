package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
)

func main() {
	// Load configuration with proper precedence: flags > env > defaults
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Set up structured logging
	logLevel := cfg.GetLogLevel()
	slogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(slogger)

	// Log the effective configuration
	cfg.LogSafeConfig(slogger)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "AG-UI Example Server",
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			slogger.Error("Request error", "error", err, "path", c.Path(), "method", c.Method(), "status", code)
			return c.Status(code).JSON(fiber.Map{
				"error":   true,
				"message": err.Error(),
			})
		},
	})

	// Add middleware
	app.Use(recover.New(recover.Config{
		EnableStackTrace: logLevel == slog.LevelDebug,
	}))

	// Add structured request logging
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path} | ${error}\n",
		Output: os.Stdout,
	}))

	// Add CORS middleware if enabled
	if cfg.CORSEnabled {
		app.Use(cors.New(cors.Config{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		}))
	}

	// Basic health check endpoint
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"service": "ag-ui-server",
		})
	})

	// Server info endpoint
	app.Get("/info", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service":      "ag-ui-server",
			"version":      "1.0.0",
			"sse_enabled":  cfg.EnableSSE,
			"cors_enabled": cfg.CORSEnabled,
		})
	})

	// SSE endpoint (if enabled)
	if cfg.EnableSSE {
		app.Get("/events", func(c fiber.Ctx) error {
			c.Set("Content-Type", "text/event-stream")
			c.Set("Cache-Control", "no-cache")
			c.Set("Connection", "keep-alive")
			c.Set("Access-Control-Allow-Origin", "*")
			c.Set("Access-Control-Allow-Headers", "Cache-Control")

			// Send a simple SSE message
			return c.SendString("data: {\"type\": \"connection\", \"message\": \"SSE connection established\"}\n\n")
		})
	}

	// Start server in a goroutine
	serverAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	go func() {
		slogger.Info("Starting server", "address", serverAddr)
		if err := app.Listen(serverAddr); err != nil {
			slogger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	slogger.Info("Server started successfully", "address", serverAddr)

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slogger.Info("Shutting down server...")

	if err := app.ShutdownWithContext(context.Background()); err != nil {
		slogger.Error("Server shutdown error", "error", err)
		os.Exit(1)
	}

	slogger.Info("Server shutdown complete")
}
