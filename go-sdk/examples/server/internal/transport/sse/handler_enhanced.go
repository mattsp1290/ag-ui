package sse

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/encoding"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/valyala/fasthttp"
)

// EnhancedSSEHandler provides an SSE handler with proper event encoding integration
type EnhancedSSEHandler struct {
	config    *HandlerConfig
	sseWriter *encoding.SSEWriter
	logger    *slog.Logger
}

// NewEnhancedSSEHandler creates a new enhanced SSE handler with encoding support
func NewEnhancedSSEHandler(cfg *config.Config) *EnhancedSSEHandler {
	handlerConfig := DefaultHandlerConfig()

	// Use config values if available
	if cfg.SSEKeepAlive > 0 {
		handlerConfig.KeepaliveInterval = cfg.SSEKeepAlive
	}

	logger := slog.Default()
	sseWriter := encoding.NewSSEWriter().WithLogger(logger)

	return &EnhancedSSEHandler{
		config:    handlerConfig,
		sseWriter: sseWriter,
		logger:    logger,
	}
}

// BuildEnhancedSSEHandler creates a Fiber v3 handler with enhanced event encoding
func BuildEnhancedSSEHandler(cfg *config.Config) fiber.Handler {
	handler := NewEnhancedSSEHandler(cfg)

	return func(c fiber.Ctx) error {
		startTime := time.Now()

		// Extract correlation ID from query parameters
		cid := c.Query("cid", "")
		requestID := c.Locals("requestid")
		if requestID == nil {
			requestID = "unknown"
		}

		// Get negotiated content type for event payloads
		eventContentType := encoding.GetEventContentType(c)

		// Structured logging context
		logCtx := []any{
			"request_id", requestID,
			"route", c.Route().Path,
			"cid", cid,
			"event_content_type", eventContentType,
		}

		handler.logger.Info("Enhanced SSE connection initiated", logCtx...)

		// Set SSE headers as required by specification
		c.Set("Content-Type", "text/event-stream; charset=utf-8")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Headers", "Cache-Control")

		// Get client context for cancellation detection
		ctx := c.RequestCtx()

		handler.logger.Info("Enhanced SSE connection established",
			append(logCtx, "latency_ms", time.Since(startTime).Milliseconds())...)

		// Start enhanced streaming loop
		return handler.handleEnhancedSSEStream(ctx, c, logCtx, cid, eventContentType)
	}
}

// handleEnhancedSSEStream manages the SSE streaming loop with proper event encoding
func (h *EnhancedSSEHandler) handleEnhancedSSEStream(reqCtx *fasthttp.RequestCtx, c fiber.Ctx, logCtx []any, cid, eventContentType string) error {
	return c.SendStreamWriter(func(w *bufio.Writer) {
		ctx := context.Background() // Create context for encoding operations

		// Send initial connection event using proper encoding
		connectionEvent := &encoding.CustomEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventTypeCustom,
			},
		}
		connectionEvent.SetData(map[string]interface{}{
			"type":         "connection",
			"timestamp":    time.Now().Format(time.RFC3339),
			"cid":          cid,
			"content_type": eventContentType,
		})
		connectionEvent.SetTimestamp(time.Now().UnixMilli())

		// Write initial connection event with proper encoding and validation
		if err := h.sseWriter.WriteEventWithType(ctx, w, connectionEvent, "connection"); err != nil {
			logCtx = append(logCtx, "error", err, "event_type", "connection")
			h.handleWriteError(err, logCtx)
			return
		}

		h.logger.Debug("Initial connection event sent",
			append(logCtx, "event_type", "connection")...)

		keepaliveTicker := time.NewTicker(h.config.KeepaliveInterval)
		defer keepaliveTicker.Stop()

		// Counter for demonstration purposes
		eventCounter := 0

		for {
			select {
			case <-reqCtx.Done():
				// Client disconnected or context cancelled
				h.logger.Info("Enhanced SSE connection closed",
					append(logCtx, "reason", "context_cancelled")...)
				return

			case <-keepaliveTicker.C:
				// Send keepalive event with proper encoding
				eventCounter++

				keepaliveEvent := &encoding.CustomEvent{
					BaseEvent: events.BaseEvent{
						EventType: events.EventTypeCustom,
					},
				}
				keepaliveEvent.SetData(map[string]interface{}{
					"type":      "keepalive",
					"timestamp": time.Now().Format(time.RFC3339),
					"sequence":  eventCounter,
					"cid":       cid,
				})
				keepaliveEvent.SetTimestamp(time.Now().UnixMilli())

				// Validate and write keepalive event
				if err := h.sseWriter.WriteEventWithType(ctx, w, keepaliveEvent, "keepalive"); err != nil {
					logCtx = append(logCtx, "error", err, "event_type", "keepalive")
					h.handleWriteError(err, logCtx)
					return
				}

				if h.config.EnableDebugLogging {
					h.logger.Debug("Keepalive event sent",
						append(logCtx, "event_type", "keepalive", "sequence", eventCounter)...)
				}

			case <-time.After(100 * time.Millisecond):
				// Periodic check for context cancellation
				if reqCtx.Err() != nil {
					h.logger.Info("Enhanced SSE connection closed via periodic check",
						append(logCtx, "reason", "periodic_check_detected_cancellation")...)
					return
				}

				// Send sample AG-UI events for demonstration
				if eventCounter > 0 && eventCounter%10 == 0 {
					sampleEvent := h.createSampleAGUIEvent(eventCounter, cid)

					if err := h.sseWriter.WriteEvent(ctx, w, sampleEvent); err != nil {
						logCtx = append(logCtx, "error", err, "event_type", sampleEvent.Type())
						h.handleWriteError(err, logCtx)
						return
					}

					if h.config.EnableDebugLogging {
						h.logger.Debug("Sample AG-UI event sent",
							append(logCtx, "event_type", sampleEvent.Type(), "counter", eventCounter)...)
					}
				}
			}
		}
	})
}

// createSampleAGUIEvent creates a sample AG-UI event for demonstration
func (h *EnhancedSSEHandler) createSampleAGUIEvent(counter int, cid string) events.Event {
	// Rotate through different AG-UI event types for demonstration
	eventTypes := []events.EventType{
		events.EventTypeTextMessageStart,
		events.EventTypeTextMessageContent,
		events.EventTypeTextMessageEnd,
		events.EventTypeRunStarted,
		events.EventTypeRunFinished,
	}

	eventType := eventTypes[counter%len(eventTypes)]

	// Create appropriate event based on type
	switch eventType {
	case events.EventTypeTextMessageStart:
		return &SampleTextMessageStartEvent{
			BaseEvent: events.BaseEvent{
				EventType:   eventType,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			Content: "Starting message stream",
			CID:     cid,
		}
	case events.EventTypeTextMessageContent:
		return &SampleTextMessageContentEvent{
			BaseEvent: events.BaseEvent{
				EventType:   eventType,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			Content: fmt.Sprintf("Message content chunk #%d", counter),
			CID:     cid,
		}
	case events.EventTypeTextMessageEnd:
		return &SampleTextMessageEndEvent{
			BaseEvent: events.BaseEvent{
				EventType:   eventType,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			Content: "Message stream ended",
			CID:     cid,
		}
	default:
		return &SampleRunEvent{
			BaseEvent: events.BaseEvent{
				EventType:   eventType,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			RunData: map[string]interface{}{
				"run_id":  fmt.Sprintf("run_%d", counter),
				"status":  "running",
				"counter": counter,
				"cid":     cid,
			},
		}
	}
}

// handleWriteError handles SSE write errors consistently
func (h *EnhancedSSEHandler) handleWriteError(err error, logCtx []any) {
	logCtx = append(logCtx, "error", err)

	// Log connection closed errors as debug instead of error since they're expected
	if strings.Contains(err.Error(), "connection closed") ||
		strings.Contains(err.Error(), "broken pipe") ||
		strings.Contains(err.Error(), "SSE write failed") {
		h.logger.Debug("SSE write failed due to connection closure", logCtx...)
	} else {
		h.logger.Error("SSE write failed", logCtx...)
	}
}

// Helper function to create int64 pointer
func timePtr(t int64) *int64 {
	return &t
}

// Sample event implementations for demonstration
type SampleTextMessageStartEvent struct {
	events.BaseEvent
	Content string `json:"content"`
	CID     string `json:"cid"`
}

func (e *SampleTextMessageStartEvent) ThreadID() string { return e.CID }
func (e *SampleTextMessageStartEvent) RunID() string    { return "" }
func (e *SampleTextMessageStartEvent) Validate() error {
	if e.Content == "" {
		return fmt.Errorf("content cannot be empty")
	}
	return nil
}

type SampleTextMessageContentEvent struct {
	events.BaseEvent
	Content string `json:"content"`
	CID     string `json:"cid"`
}

func (e *SampleTextMessageContentEvent) ThreadID() string { return e.CID }
func (e *SampleTextMessageContentEvent) RunID() string    { return "" }
func (e *SampleTextMessageContentEvent) Validate() error {
	if e.Content == "" {
		return fmt.Errorf("content cannot be empty")
	}
	return nil
}

type SampleTextMessageEndEvent struct {
	events.BaseEvent
	Content string `json:"content"`
	CID     string `json:"cid"`
}

func (e *SampleTextMessageEndEvent) ThreadID() string { return e.CID }
func (e *SampleTextMessageEndEvent) RunID() string    { return "" }
func (e *SampleTextMessageEndEvent) Validate() error  { return nil }

type SampleRunEvent struct {
	events.BaseEvent
	RunData map[string]interface{} `json:"run_data"`
}

func (e *SampleRunEvent) ThreadID() string { return "" }
func (e *SampleRunEvent) RunID() string {
	if id, ok := e.RunData["run_id"].(string); ok {
		return id
	}
	return ""
}
func (e *SampleRunEvent) Validate() error { return nil }
