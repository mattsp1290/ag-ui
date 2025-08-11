package routes

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/state"
)

// Global store instance - in production this might be dependency-injected
var sharedStore = state.NewStore().WithLogger(slog.Default())

// SharedStateHandler creates the SSE endpoint for shared state streaming
func SharedStateHandler(_ *config.Config) fiber.Handler {
	logger := slog.Default()

	return func(c fiber.Ctx) error {
		// Extract correlation ID and other parameters
		cid := c.Query("cid", "")
		requestID := c.Locals("requestid")
		if requestID == nil {
			requestID = "unknown"
		}

		// Check for demo flag
		enableDemo := c.Query("demo", "") == "true"

		// Structured logging context
		logCtx := []any{
			"request_id", requestID,
			"route", c.Route().Path,
			"cid", cid,
			"demo", enableDemo,
		}

		logger.Info("Shared state SSE connection initiated", logCtx...)

		// Set SSE headers
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Headers", "Cache-Control")

		// Get client context for cancellation detection
		ctx := c.RequestCtx()

		// Create context for this connection
		connCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start streaming
		return c.SendStreamWriter(func(w *bufio.Writer) {
			// Send initial STATE_SNAPSHOT
			snapshot := state.NewStateSnapshot(sharedStore.Snapshot())
			if err := writeSSEEvent(w, snapshot, ""); err != nil {
				logger.Error("Failed to write initial snapshot", append(logCtx, "error", err)...)
				return
			}

			// Create watcher for state changes
			watcher, err := sharedStore.Watch(connCtx)
			if err != nil {
				logger.Error("Failed to create state watcher", append(logCtx, "error", err)...)
				return
			}
			defer watcher.Close()

			// Start demo goroutine if requested
			if enableDemo {
				go runDemo(connCtx, logger, logCtx)
			}

			// Set up periodic keepalive
			keepaliveTicker := time.NewTicker(15 * time.Second)
			defer keepaliveTicker.Stop()

			eventCounter := 0

			for {
				select {
				case <-ctx.Done():
					// Client disconnected
					logger.Info("Shared state SSE connection closed", append(logCtx, "reason", "client_disconnected")...)
					return

				case <-connCtx.Done():
					// Context canceled
					logger.Info("Shared state SSE connection closed", append(logCtx, "reason", "context_canceled")...)
					return

				case delta := <-watcher.Channel():
					// Received state delta
					if delta == nil {
						// Channel closed
						logger.Info("State watcher channel closed", logCtx...)
						return
					}

					if err := writeSSEEvent(w, delta, ""); err != nil {
						if strings.Contains(err.Error(), "connection closed") || strings.Contains(err.Error(), "broken pipe") {
							logger.Debug("Delta write failed due to connection closure", append(logCtx, "error", err)...)
						} else {
							logger.Error("Failed to write state delta", append(logCtx, "error", err)...)
						}
						return
					}

				case <-keepaliveTicker.C:
					// Send keepalive
					eventCounter++
					keepalive := map[string]interface{}{
						"type":      "keepalive",
						"timestamp": time.Now().Format(time.RFC3339),
						"sequence":  eventCounter,
						"cid":       cid,
					}

					if err := writeSSEEvent(w, keepalive, "keepalive"); err != nil {
						if strings.Contains(err.Error(), "connection closed") || strings.Contains(err.Error(), "broken pipe") {
							logger.Debug("Keepalive write failed due to connection closure", append(logCtx, "error", err)...)
						} else {
							logger.Error("Failed to write keepalive", append(logCtx, "error", err)...)
						}
						return
					}

				case <-time.After(100 * time.Millisecond):
					// Periodic check for context cancellation
					select {
					case <-ctx.Done():
						logger.Info("Shared state SSE connection closed via periodic check", logCtx...)
						return
					default:
						// Continue
					}
				}
			}
		})
	}
}

// SharedStateUpdateHandler handles POST requests to update shared state
func SharedStateUpdateHandler(_ *config.Config) fiber.Handler {
	logger := slog.Default()

	return func(c fiber.Ctx) error {
		// Parse request body
		var updateRequest map[string]interface{}
		if err := json.Unmarshal(c.Body(), &updateRequest); err != nil {
			logger.Error("Failed to parse update request", "error", err)
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid JSON body",
			})
		}

		op, ok := updateRequest["op"].(string)
		if !ok {
			return c.Status(400).JSON(fiber.Map{
				"error": "Missing 'op' field",
			})
		}

		// Apply the requested operation
		err := sharedStore.Update(func(s *state.State) {
			switch op {
			case "increment_counter":
				s.Counter++
				logger.Debug("Counter incremented", "new_value", s.Counter)

			case "decrement_counter":
				s.Counter--
				logger.Debug("Counter decremented", "new_value", s.Counter)

			case "reset_counter":
				s.Counter = 0
				logger.Debug("Counter reset")

			case "add_item":
				// Extract item data
				if itemData, ok := updateRequest["value"].(map[string]interface{}); ok {
					item := state.Item{
						ID:    fmt.Sprintf("item_%d", time.Now().UnixNano()),
						Value: getString(itemData, "value"),
						Type:  getString(itemData, "type"),
					}
					s.Items = append(s.Items, item)
					logger.Debug("Item added", "item_id", item.ID, "value", item.Value)
				}

			case "clear_items":
				s.Items = make([]state.Item, 0)
				logger.Debug("Items cleared")

			default:
				// Unknown operation - log but don't error, just ignore
				logger.Warn("Unknown operation requested", "operation", op)
			}
		})

		if err != nil {
			logger.Error("Failed to update state", "error", err, "operation", op)
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to update state",
			})
		}

		// Return current state info
		snapshot := sharedStore.Snapshot()
		return c.JSON(fiber.Map{
			"success":   true,
			"operation": op,
			"state": fiber.Map{
				"version":     snapshot.Version,
				"counter":     snapshot.Counter,
				"items_count": len(snapshot.Items),
				"watchers":    sharedStore.GetWatcherCount(),
			},
		})
	}
}

// writeSSEEvent writes an event in SSE format
func writeSSEEvent(w *bufio.Writer, data interface{}, eventType string) error {
	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	// Write event type if specified
	if eventType != "" {
		if _, err := w.WriteString(fmt.Sprintf("event: %s\n", eventType)); err != nil {
			return fmt.Errorf("failed to write event type: %w", err)
		}
	}

	// Escape newlines in JSON data for SSE format
	escapedData := strings.ReplaceAll(string(jsonData), "\n", "\\n")
	escapedData = strings.ReplaceAll(escapedData, "\r", "\\r")

	// Write data line
	if _, err := w.WriteString(fmt.Sprintf("data: %s\n\n", escapedData)); err != nil {
		return fmt.Errorf("failed to write event data: %w", err)
	}

	// Flush immediately
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush event data: %w", err)
	}

	return nil
}

// runDemo performs some automated state changes for demonstration
func runDemo(ctx context.Context, logger *slog.Logger, logCtx []any) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	operations := []string{"increment_counter", "add_item", "increment_counter", "add_item", "decrement_counter"}
	opIndex := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if opIndex >= len(operations) {
				return // Demo complete
			}

			op := operations[opIndex]
			err := sharedStore.Update(func(s *state.State) {
				switch op {
				case "increment_counter":
					s.Counter++
				case "decrement_counter":
					s.Counter--
				case "add_item":
					item := state.Item{
						ID:    fmt.Sprintf("demo_item_%d", time.Now().UnixNano()),
						Value: fmt.Sprintf("Demo item #%d", opIndex),
						Type:  "demo",
					}
					s.Items = append(s.Items, item)
				}
			})

			if err != nil {
				logger.Error("Demo operation failed", append(logCtx, "error", err, "operation", op)...)
			} else {
				logger.Debug("Demo operation completed", append(logCtx, "operation", op)...)
			}

			opIndex++
		}
	}
}

// getString safely extracts a string value from a map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// GetSharedStore returns the global shared store instance
func GetSharedStore() *state.Store {
	return sharedStore
}
