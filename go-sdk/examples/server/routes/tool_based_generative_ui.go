package routes

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/encoding"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// ToolBasedGenerativeUIHandler creates a Fiber handler for the tool-based generative UI route
func ToolBasedGenerativeUIHandler(cfg *config.Config) fiber.Handler {
	logger := slog.Default()
	sseWriter := encoding.NewSSEWriter().WithLogger(logger)

	return func(c fiber.Ctx) error {
		// Set SSE headers
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Headers", "Cache-Control")

		// Extract request metadata
		requestID := c.Locals("requestid")
		if requestID == nil {
			requestID = "unknown"
		}

		logCtx := []any{
			"request_id", requestID,
			"route", c.Route().Path,
			"method", c.Method(),
		}

		logger.Info("Tool-based generative UI SSE connection established", logCtx...)

		// Get request context for cancellation
		ctx := c.RequestCtx()

		// Start streaming
		return c.SendStreamWriter(func(w *bufio.Writer) {
			if err := streamToolBasedGenerativeUIEvents(ctx, w, sseWriter, cfg, logger, logCtx); err != nil {
				logger.Error("Error streaming tool-based generative UI events", append(logCtx, "error", err)...)
			}
		})
	}
}

// streamToolBasedGenerativeUIEvents implements the tool-based generative UI event sequence
func streamToolBasedGenerativeUIEvents(reqCtx context.Context, w *bufio.Writer, sseWriter *encoding.SSEWriter, cfg *config.Config, logger *slog.Logger, logCtx []any) error {
	// Generate IDs for this session
	threadID := events.GenerateThreadID()
	runID := events.GenerateRunID()

	// Create a wrapped context for our operations
	ctx := context.Background()

	// Send RUN_STARTED event
	runStarted := events.NewRunStartedEvent(threadID, runID)
	if err := sseWriter.WriteEvent(ctx, w, runStarted); err != nil {
		return fmt.Errorf("failed to write RUN_STARTED event: %w", err)
	}

	// Check for cancellation
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected during RUN_STARTED", append(logCtx, "reason", "context_cancelled")...)
		return nil
	}

	// Small delay to simulate processing time
	time.Sleep(100 * time.Millisecond)

	// For this implementation, we'll assume we're sending a tool call message
	// (similar to the Python reference where it checks the last message but we'll start with tool call)
	toolCallID := events.GenerateToolCallID()
	messageID := events.GenerateMessageID()

	// Prepare haiku arguments (matching Python reference)
	haikuArgs := map[string]interface{}{
		"japanese": []string{"エーアイの", "橋つなぐ道", "コパキット"},
		"english": []string{
			"From AI's realm",
			"A bridge-road linking us—",
			"CopilotKit.",
		},
	}

	// Marshal haiku arguments to JSON string
	haikuArgsJSON, err := json.Marshal(haikuArgs)
	if err != nil {
		return fmt.Errorf("failed to marshal haiku arguments: %w", err)
	}

	// Create new assistant message with tool call
	newMessage := events.Message{
		ID:   messageID,
		Role: "assistant",
		ToolCalls: []events.ToolCall{
			{
				ID:   toolCallID,
				Type: "function",
				Function: events.Function{
					Name:      "generate_haiku",
					Arguments: string(haikuArgsJSON),
				},
			},
		},
	}

	// For this example, we'll start with just the new message
	// In a real implementation, this would include input messages + the new message
	allMessages := []events.Message{newMessage}

	// Check for cancellation before sending messages
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected before messages snapshot", append(logCtx, "reason", "context_cancelled")...)
		return nil
	}

	// Send messages snapshot event
	messagesSnapshot := events.NewMessagesSnapshotEvent(allMessages)
	if err := sseWriter.WriteEvent(ctx, w, messagesSnapshot); err != nil {
		return fmt.Errorf("failed to write MESSAGES_SNAPSHOT event: %w", err)
	}

	// Small delay before finishing
	time.Sleep(100 * time.Millisecond)

	// Check for cancellation before final event
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected before RUN_FINISHED", append(logCtx, "reason", "context_cancelled")...)
		return nil
	}

	// Send RUN_FINISHED event
	runFinished := events.NewRunFinishedEvent(threadID, runID)
	if err := sseWriter.WriteEvent(ctx, w, runFinished); err != nil {
		return fmt.Errorf("failed to write RUN_FINISHED event: %w", err)
	}

	logger.Info("Tool-based generative UI event sequence completed successfully", logCtx...)
	return nil
}
