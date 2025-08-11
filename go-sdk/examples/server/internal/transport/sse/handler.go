package sse

import (
	"bufio"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"

	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
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
	return c.SendStreamWriter(func(w *bufio.Writer) {
		if !writeInitialConnection(w, logger, logCtx, cid) {
			return
		}
		streamLoop(reqCtx, w, config, logger, logCtx, cid)
	})
}

func writeInitialConnection(w *bufio.Writer, logger *slog.Logger, logCtx []any, cid string) bool {
	initialEvent := fmt.Sprintf("data: {\"type\":\"connection\",\"timestamp\":\"%s\",\"cid\":\"%s\"}\n\n",
		time.Now().Format(time.RFC3339), cid)
	if _, err := w.WriteString(initialEvent); err != nil {
		logWriteError(logger, logCtx, err, "connection", "Initial connection write failed due to connection closure", "Failed to write initial connection event")
		return false
	}
	if err := w.Flush(); err != nil {
		logWriteError(logger, logCtx, err, "connection", "Initial connection flush failed due to connection closure", "Failed to flush initial connection event")
		return false
	}
	return true
}

func streamLoop(reqCtx *fasthttp.RequestCtx, w *bufio.Writer, config *HandlerConfig, logger *slog.Logger, logCtx []any, cid string) {
	keepaliveTicker := time.NewTicker(config.KeepaliveInterval)
	defer keepaliveTicker.Stop()
	eventCounter := 0
	for {
		select {
		case <-reqCtx.Done():
			logger.Info("SSE connection closed", append(logCtx, "reason", "context_canceled")...)
			return
		case <-keepaliveTicker.C:
			eventCounter++
			if !writeAndFlush(w, logger, logCtx, fmt.Sprintf("event: keepalive\ndata: {\"type\":\"keepalive\",\"timestamp\":\"%s\",\"sequence\":%d,\"cid\":\"%s\"}\n\n", time.Now().Format(time.RFC3339), eventCounter, cid), "keepalive", "Keepalive write failed due to connection closure", "Failed to write keepalive event") {
				return
			}
			if config.EnableDebugLogging {
				logger.Debug("Keepalive sent", append(logCtx, "event_type", "keepalive", "sequence", eventCounter)...)
			}
		case <-time.After(100 * time.Millisecond):
			if reqCtx.Err() != nil {
				logger.Info("SSE connection closed via periodic check", append(logCtx, "reason", "periodic_check_detected_cancellation")...)
				return
			}
			if eventCounter > 0 && eventCounter%10 == 0 {
				if !writeAndFlush(w, logger, logCtx, fmt.Sprintf("data: {\"type\":\"sample\",\"message\":\"Sample event #%d\",\"timestamp\":\"%s\",\"cid\":\"%s\"}\n\n", eventCounter/10, time.Now().Format(time.RFC3339), cid), "sample", "Sample event write failed due to connection closure", "Failed to write sample event") {
					return
				}
				if config.EnableDebugLogging {
					logger.Debug("Sample event sent", append(logCtx, "event_type", "sample")...)
				}
			}
		}
	}
}

func writeAndFlush(w *bufio.Writer, logger *slog.Logger, logCtx []any, payload string, eventType string, debugMsg string, errMsg string) bool {
	if _, err := w.WriteString(payload); err != nil {
		logWriteError(logger, logCtx, err, eventType, debugMsg, errMsg)
		return false
	}
	if err := w.Flush(); err != nil {
		logWriteError(logger, logCtx, err, eventType, strings.Replace(debugMsg, "write", "flush", 1), strings.Replace(errMsg, "write", "flush", 1))
		return false
	}
	return true
}

func logWriteError(logger *slog.Logger, logCtx []any, err error, eventType, debugMsg, errMsg string) {
	logCtx = append(logCtx, "error", err, "event_type", eventType)
	if strings.Contains(err.Error(), "connection closed") || strings.Contains(err.Error(), "broken pipe") {
		logger.Debug(debugMsg, logCtx...)
	} else {
		logger.Error(errMsg, logCtx...)
	}
}

// GetConnectionCount returns the current number of active SSE connections
// This is a placeholder for future connection tracking
func GetConnectionCount() int {
	// TODO: Implement connection counting if needed
	return 0
}
