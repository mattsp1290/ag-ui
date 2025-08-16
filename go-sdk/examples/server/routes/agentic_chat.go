package routes

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"

	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/encoding"
)

// AgenticChatHandler creates a Fiber handler for the agentic chat route
func AgenticChatHandler(_ *config.Config) fiber.Handler {
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

		logger.Info("Agentic chat SSE connection established", logCtx...)

		// Get request context for cancellation
		ctx := c.RequestCtx()

		// Start streaming
		return c.SendStreamWriter(func(w *bufio.Writer) {
			if err := streamAgenticChatEvents(ctx, w, sseWriter, logger, logCtx); err != nil {
				logger.Error("Error streaming agentic chat events", append(logCtx, "error", err)...)
			}
		})
	}
}

// streamAgenticChatEvents implements the deterministic event sequence
func streamAgenticChatEvents(reqCtx context.Context, w *bufio.Writer, sseWriter *encoding.SSEWriter, logger *slog.Logger, logCtx []any) error {
	// Generate IDs for this session
	threadID := events.GenerateThreadID()
	runID := events.GenerateRunID()
	messageID := events.GenerateMessageID()
	toolCallID := events.GenerateToolCallID()

	// Create a wrapped context for our operations
	ctx := context.Background()

	// Send RUN_STARTED event
	runStarted := events.NewRunStartedEvent(threadID, runID)
	if err := sseWriter.WriteEvent(ctx, w, runStarted); err != nil {
		return fmt.Errorf("failed to write RUN_STARTED event: %w", err)
	}

	// Check for cancellation
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected during RUN_STARTED", append(logCtx, "reason", "context_canceled")...)
		return nil
	}

	// Send TEXT_MESSAGE_START event
	msgStart := events.NewTextMessageStartEvent(messageID, events.WithRole("assistant"))
	if err := sseWriter.WriteEvent(ctx, w, msgStart); err != nil {
		return fmt.Errorf("failed to write TEXT_MESSAGE_START event: %w", err)
	}

	// Send initial message content
	msgContent := events.NewTextMessageContentEvent(messageID, "I'll help you with that task. Let me use a tool to get the information you need.")
	if err := sseWriter.WriteEvent(ctx, w, msgContent); err != nil {
		return fmt.Errorf("failed to write TEXT_MESSAGE_CONTENT event: %w", err)
	}

	// Check for cancellation
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected during message content", append(logCtx, "reason", "context_canceled")...)
		return nil
	}

	// Send TEXT_MESSAGE_END event
	msgEnd := events.NewTextMessageEndEvent(messageID)
	if err := sseWriter.WriteEvent(ctx, w, msgEnd); err != nil {
		return fmt.Errorf("failed to write TEXT_MESSAGE_END event: %w", err)
	}

	// Brief pause before tool call
	time.Sleep(300 * time.Millisecond)

	// Check for cancellation
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected before tool call", append(logCtx, "reason", "context_canceled")...)
		return nil
	}

	// Send TOOL_CALL_START event
	toolStart := events.NewToolCallStartEvent(toolCallID, "get_weather")
	if err := sseWriter.WriteEvent(ctx, w, toolStart); err != nil {
		return fmt.Errorf("failed to write TOOL_CALL_START event: %w", err)
	}

	// Send TOOL_CALL_ARGS event with JSON arguments
	toolArgs := events.NewToolCallArgsEvent(toolCallID, `{"location": "San Francisco", "unit": "celsius"}`)
	if err := sseWriter.WriteEvent(ctx, w, toolArgs); err != nil {
		return fmt.Errorf("failed to write TOOL_CALL_ARGS event: %w", err)
	}

	// Brief pause to simulate tool execution
	time.Sleep(500 * time.Millisecond)

	// Check for cancellation
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected during tool execution", append(logCtx, "reason", "context_canceled")...)
		return nil
	}

	// Send TOOL_CALL_END event
	toolEnd := events.NewToolCallEndEvent(toolCallID)
	if err := sseWriter.WriteEvent(ctx, w, toolEnd); err != nil {
		return fmt.Errorf("failed to write TOOL_CALL_END event: %w", err)
	}

	// Send final message with the result
	finalMessageID := events.GenerateMessageID()

	finalMsgStart := events.NewTextMessageStartEvent(finalMessageID, events.WithRole("assistant"))
	if err := sseWriter.WriteEvent(ctx, w, finalMsgStart); err != nil {
		return fmt.Errorf("failed to write final TEXT_MESSAGE_START event: %w", err)
	}

	// Stream the final response with some pauses
	finalParts := []string{
		"Based on the weather data, ",
		"the current temperature in San Francisco is 22°C ",
		"with partly cloudy skies. ",
		"It's a pleasant day for outdoor activities! ✓",
	}

	for _, part := range finalParts {
		// Check for cancellation before each part
		if err := reqCtx.Err(); err != nil {
			logger.Debug("Client disconnected during final message", append(logCtx, "reason", "context_canceled")...)
			return nil
		}

		finalMsgContent := events.NewTextMessageContentEvent(finalMessageID, part)
		if err := sseWriter.WriteEvent(ctx, w, finalMsgContent); err != nil {
			return fmt.Errorf("failed to write final TEXT_MESSAGE_CONTENT event: %w", err)
		}

		time.Sleep(200 * time.Millisecond) // Small delay between chunks
	}

	// Send final TEXT_MESSAGE_END event
	finalMsgEnd := events.NewTextMessageEndEvent(finalMessageID)
	if err := sseWriter.WriteEvent(ctx, w, finalMsgEnd); err != nil {
		return fmt.Errorf("failed to write final TEXT_MESSAGE_END event: %w", err)
	}

	// Send RUN_FINISHED event
	runFinished := events.NewRunFinishedEvent(threadID, runID)
	if err := sseWriter.WriteEvent(ctx, w, runFinished); err != nil {
		return fmt.Errorf("failed to write RUN_FINISHED event: %w", err)
	}

	logger.Info("Agentic chat event sequence completed successfully", logCtx...)
	return nil
}
