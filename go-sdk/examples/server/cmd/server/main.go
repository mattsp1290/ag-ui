package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/sirupsen/logrus"

	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/encoding"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/logging"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/middleware"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/transport/sse"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/routes"
)

func main() {
	// Load configuration with proper precedence: flags > env > defaults
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Set up structured logging with logrus
	logger, err := logging.Init(logging.Config{
		Level:        cfg.LogLevel,
		EnableCaller: cfg.LogLevel == "debug",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Log the effective configuration
	logger.WithFields(logrus.Fields{
		"host":                  cfg.Host,
		"port":                  cfg.Port,
		"log_level":             cfg.LogLevel,
		"enable_sse":            cfg.EnableSSE,
		"read_timeout":          cfg.ReadTimeout,
		"write_timeout":         cfg.WriteTimeout,
		"sse_keepalive":         cfg.SSEKeepAlive,
		"cors_enabled":          cfg.CORSEnabled,
		"streaming_chunk_delay": cfg.StreamingChunkDelay,
	}).Info("Server configuration loaded")

	// Create Fiber app with updated configuration
	app := fiber.New(fiber.Config{
		AppName:      "AG-UI Example Server",
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}

			// Use request logger if available, otherwise use main logger
			entry := middleware.GetLogger(c)
			entry.WithFields(logrus.Fields{
				"error":  err.Error(),
				"status": code,
			}).Error("Request error")

			return c.Status(code).JSON(fiber.Map{
				"error":   true,
				"message": err.Error(),
			})
		},
	})

	// Configure middleware stack in proper order
	app.Use(requestid.New())
	app.Use(middleware.RequestContext(logger))
	app.Use(middleware.Recovery())
	app.Use(middleware.AccessLog())

	// Add CORS middleware if enabled
	if cfg.CORSEnabled {
		app.Use(cors.New(cors.Config{
			AllowOrigins:     cfg.CORSAllowedOrigins,
			AllowMethods:     []string{"GET", "POST", "HEAD", "PUT", "DELETE", "PATCH", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
			AllowCredentials: false,
		}))
	}

	// Add content negotiation middleware for all routes
	app.Use(encoding.ContentNegotiationMiddleware(encoding.ContentNegotiationConfig{
		DefaultContentType: "application/json",
		SupportedTypes:     []string{"application/json", "application/vnd.ag-ui+json"},
		EnableLogging:      cfg.LogLevel == "debug",
	}))

	// Health check endpoint
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

	// Basic info route (from main branch)
	app.Get("/", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "AG-UI Go Example Server is running!",
			"path":    c.Path(),
			"method":  c.Method(),
			"headers": c.GetReqHeaders(),
		})
	})

	// SSE endpoints (if enabled)
	if cfg.EnableSSE {
		// Legacy simple SSE endpoint for backward compatibility
		app.Get("/events", func(c fiber.Ctx) error {
			c.Set("Content-Type", "text/event-stream")
			c.Set("Cache-Control", "no-cache")
			c.Set("Connection", "keep-alive")
			c.Set("Access-Control-Allow-Origin", "*")
			c.Set("Access-Control-Allow-Headers", "Cache-Control")

			// Send a simple SSE message
			return c.SendString("data: {\"type\": \"connection\", \"message\": \"SSE connection established\"}\n\n")
		})

		// Enhanced SSE transport endpoint with proper event encoding and validation
		app.Get("/examples/_internal/stream", sse.BuildEnhancedSSEHandler(cfg))

		// Keep the original handler for backwards compatibility
		app.Get("/examples/_internal/stream/legacy", sse.BuildSSEHandler(cfg))

		// Agentic chat route with deterministic event sequence
		app.Get("/examples/agentic-chat", routes.AgenticChatHandler(cfg))

		// Human-in-the-loop route with conditional branching
		app.Post("/human_in_the_loop", routes.HumanInTheLoopHandler(cfg))

		// Agentic generative UI route with state updates
		app.Get("/examples/agentic-generative-ui", routes.AgenticGenerativeUIHandler(cfg))

		// Tool-based generative UI route with tool call demonstration
		app.Get("/examples/tool-based-generative-ui", routes.ToolBasedGenerativeUIHandler(cfg))

		// Shared state routes
		app.Get("/examples/shared-state", routes.SharedStateHandler(cfg))
		app.Post("/examples/shared-state/update", routes.SharedStateUpdateHandler(cfg))

		// Predictive state updates route
		app.Get("/examples/state/predictive", routes.PredictiveStateHandler(cfg))
	}

	// Start server in a goroutine
	serverAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	go func() {
		logger.WithField("address", serverAddr).Info("Starting server")
		if err := app.Listen(serverAddr); err != nil {
			logger.WithError(err).Error("Server failed to start")
			os.Exit(1)
		}
	}()

	logger.WithField("address", serverAddr).Info("Server started successfully")

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		logger.WithError(err).Error("Server shutdown error")
		os.Exit(1)
	}

	logger.Info("Server shutdown complete")
}
