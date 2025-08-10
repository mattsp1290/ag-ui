package sse

import (
	"bufio"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
	"github.com/valyala/fasthttp"
)

// HandlerConfig contains configuration options for the SSE handler
type HandlerConfig struct {
	// KeepaliveInterval defines how often to send keepalive frames
	KeepaliveInterval time.Duration
	// EnableDebugLogging enables verbose debug logging
	EnableDebugLogging bool
	// MaxConnections limits concurrent SSE connections (0 = unlimited)
	MaxConnections int
	// ConnectionTimeout for idle connections
	ConnectionTimeout time.Duration
}

// DefaultHandlerConfig returns a sensible default configuration
func DefaultHandlerConfig() *HandlerConfig {
	return &HandlerConfig{
		KeepaliveInterval:  15 * time.Second,
		EnableDebugLogging: false,
		MaxConnections:     100,
		ConnectionTimeout:  5 * time.Minute,
	}
}

// BuildSSEHandler creates a Fiber v3 handler for Server-Sent Events transport
func BuildSSEHandler(cfg *config.Config) fiber.Handler {
	handlerConfig := DefaultHandlerConfig()

	// Use config values if available
	if cfg.SSEKeepAlive > 0 {
		handlerConfig.KeepaliveInterval = cfg.SSEKeepAlive
	}

	logger := slog.Default()

	return func(c fiber.Ctx) error {
		startTime := time.Now()

		// Extract correlation ID from query parameters
		cid := c.Query("cid", "")
		requestID := c.Locals("requestid")
		if requestID == nil {
			requestID = "unknown"
		}

		// Structured logging context
		logCtx := []any{
			"request_id", requestID,
			"route", c.Route().Path,
			"cid", cid,
		}

		logger.Info("SSE connection initiated", logCtx...)

		// Set SSE headers as required by specification
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Headers", "Cache-Control")

		// Get client context for cancellation detection
		ctx := c.RequestCtx()

		logger.Info("SSE connection established", append(logCtx, "latency_ms", time.Since(startTime).Milliseconds())...)

		// Start streaming loop which will send initial connection event first
		return handleSSEStream(ctx, c, handlerConfig, logger, logCtx, cid)
	}
}

// handleSSEStream manages the SSE streaming loop with keepalive and cancellation
func handleSSEStream(reqCtx *fasthttp.RequestCtx, c fiber.Ctx, config *HandlerConfig, logger *slog.Logger, logCtx []any, cid string) error {
	// Use SendStreamWriter for proper streaming
	return c.SendStreamWriter(func(w *bufio.Writer) {
		// Send initial connection event first
		initialEvent := fmt.Sprintf("data: {\"type\":\"connection\",\"timestamp\":\"%s\",\"cid\":\"%s\"}\n\n",
			time.Now().Format(time.RFC3339), cid)

		if _, err := w.WriteString(initialEvent); err != nil {
			logCtx = append(logCtx, "error", err, "event_type", "connection")
			// Log connection closed errors as debug instead of error since they're expected
			if strings.Contains(err.Error(), "connection closed") || strings.Contains(err.Error(), "broken pipe") {
				logger.Debug("Initial connection write failed due to connection closure", logCtx...)
			} else {
				logger.Error("Failed to write initial connection event", logCtx...)
			}
			return
		}

		if err := w.Flush(); err != nil {
			logCtx = append(logCtx, "error", err, "event_type", "connection")
			// Log connection closed errors as debug instead of error since they're expected
			if strings.Contains(err.Error(), "connection closed") || strings.Contains(err.Error(), "broken pipe") {
				logger.Debug("Initial connection flush failed due to connection closure", logCtx...)
			} else {
				logger.Error("Failed to flush initial connection event", logCtx...)
			}
			return
		}

		keepaliveTicker := time.NewTicker(config.KeepaliveInterval)
		defer keepaliveTicker.Stop()

		// Counter for demonstration purposes
		eventCounter := 0

		for {
			select {
			case <-reqCtx.Done():
				// Client disconnected or context cancelled
				logCtx = append(logCtx, "reason", "context_cancelled")
				logger.Info("SSE connection closed", logCtx...)
				return

			case <-keepaliveTicker.C:
				// Send keepalive frame
				eventCounter++

				keepaliveEvent := fmt.Sprintf("event: keepalive\ndata: {\"type\":\"keepalive\",\"timestamp\":\"%s\",\"sequence\":%d,\"cid\":\"%s\"}\n\n",
					time.Now().Format(time.RFC3339), eventCounter, cid)

				if _, err := w.WriteString(keepaliveEvent); err != nil {
					logCtx = append(logCtx, "error", err, "event_type", "keepalive")
					// Log connection closed errors as debug instead of error since they're expected
					if strings.Contains(err.Error(), "connection closed") || strings.Contains(err.Error(), "broken pipe") {
						logger.Debug("Keepalive write failed due to connection closure", logCtx...)
					} else {
						logger.Error("Failed to write keepalive event", logCtx...)
					}
					return
				}

				// Flush immediately after write to ensure delivery
				if err := w.Flush(); err != nil {
					logCtx = append(logCtx, "error", err, "event_type", "keepalive")
					// Log connection closed errors as debug instead of error since they're expected
					if strings.Contains(err.Error(), "connection closed") || strings.Contains(err.Error(), "broken pipe") {
						logger.Debug("Keepalive flush failed due to connection closure", logCtx...)
					} else {
						logger.Error("Failed to flush keepalive event", logCtx...)
					}
					return
				}

				if config.EnableDebugLogging {
					logger.Debug("Keepalive sent", append(logCtx, "event_type", "keepalive", "sequence", eventCounter)...)
				}

			case <-time.After(100 * time.Millisecond):
				// Periodic check for context cancellation to ensure we detect disconnects quickly
				// This provides a tight bound for cancellation detection as required by spec (< 100ms)
				if reqCtx.Err() != nil {
					logCtx = append(logCtx, "reason", "periodic_check_detected_cancellation")
					logger.Info("SSE connection closed via periodic check", logCtx...)
					return
				}

				// Optional: Send a sample data event occasionally for testing
				if eventCounter > 0 && eventCounter%10 == 0 {
					sampleEvent := fmt.Sprintf("data: {\"type\":\"sample\",\"message\":\"Sample event #%d\",\"timestamp\":\"%s\",\"cid\":\"%s\"}\n\n",
						eventCounter/10, time.Now().Format(time.RFC3339), cid)

					if _, err := w.WriteString(sampleEvent); err != nil {
						logCtx = append(logCtx, "error", err, "event_type", "sample")
						// Log connection closed errors as debug instead of error since they're expected
						if strings.Contains(err.Error(), "connection closed") || strings.Contains(err.Error(), "broken pipe") {
							logger.Debug("Sample event write failed due to connection closure", logCtx...)
						} else {
							logger.Error("Failed to write sample event", logCtx...)
						}
						return
					}

					if err := w.Flush(); err != nil {
						logCtx = append(logCtx, "error", err, "event_type", "sample")
						// Log connection closed errors as debug instead of error since they're expected
						if strings.Contains(err.Error(), "connection closed") || strings.Contains(err.Error(), "broken pipe") {
							logger.Debug("Sample event flush failed due to connection closure", logCtx...)
						} else {
							logger.Error("Failed to flush sample event", logCtx...)
						}
						return
					}

					if config.EnableDebugLogging {
						logger.Debug("Sample event sent", append(logCtx, "event_type", "sample")...)
					}
				}
			}
		}
	})
}

// GetConnectionCount returns the current number of active SSE connections
// This is a placeholder for future connection tracking
func GetConnectionCount() int {
	// TODO: Implement connection counting if needed
	return 0
}
