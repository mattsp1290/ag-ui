package routes

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"

	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/encoding"
)

// ToolBasedGenerativeUIInput represents the input structure for the tool-based generative UI endpoint
type ToolBasedGenerativeUIInput struct {
	ThreadID       string                   `json:"thread_id"`
	RunID          string                   `json:"run_id"`
	State          interface{}              `json:"state"`
	Messages       []map[string]interface{} `json:"messages"`
	Tools          []interface{}            `json:"tools"`
	Context        []interface{}            `json:"context"`
	ForwardedProps interface{}              `json:"forwarded_props"`
}

// ToolBasedGenerativeUIHandler creates a Fiber handler for the tool-based generative UI route
func ToolBasedGenerativeUIHandler(cfg *config.Config) fiber.Handler {
	logger := slog.Default()
	sseWriter := encoding.NewSSEWriter().WithLogger(logger)

	return func(c fiber.Ctx) error {
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

		// Parse request body first before setting headers
		var input ToolBasedGenerativeUIInput
		if err := c.Bind().JSON(&input); err != nil {
			logger.Error("Failed to parse request body", append(logCtx, "error", err)...)
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		// Set SSE headers after validation
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Headers", "Cache-Control")

		logger.Info("Tool-based generative UI SSE connection established", logCtx...)

		// Get request context for cancellation
		ctx := c.RequestCtx()

		// Start streaming
		return c.SendStreamWriter(func(w *bufio.Writer) {
			if err := streamToolBasedGenerativeUIEvents(ctx, w, sseWriter, &input, cfg, logger, logCtx); err != nil {
				logger.Error("Error streaming tool-based generative UI events", append(logCtx, "error", err)...)
			}
		})
	}
}

// streamToolBasedGenerativeUIEvents implements the tool-based generative UI event sequence
func streamToolBasedGenerativeUIEvents(reqCtx context.Context, w *bufio.Writer, sseWriter *encoding.SSEWriter, input *ToolBasedGenerativeUIInput, _ *config.Config, logger *slog.Logger, logCtx []any) error {
	// Use IDs from input or generate new ones if not provided
	threadID := input.ThreadID
	if threadID == "" {
		threadID = events.GenerateThreadID()
	}
	runID := input.RunID
	if runID == "" {
		runID = events.GenerateRunID()
	}

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

	// Small delay to simulate processing time
	time.Sleep(100 * time.Millisecond)

	// Check if last message was a tool result
	var lastMessage map[string]interface{}
	if len(input.Messages) > 0 {
		lastMessage = input.Messages[len(input.Messages)-1]
	}

	var newMessage events.Message
	messageID := events.GenerateMessageID()

	// Determine what type of message to send
	if lastMessage != nil && lastMessage["role"] == "tool" {
		// Send text message for tool result
		content := "Haiku created"
		newMessage = events.Message{
			ID:      messageID,
			Role:    "assistant",
			Content: &content,
		}
	} else {
		// Send tool call message
		toolCallID := events.GenerateToolCallID()

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
		newMessage = events.Message{
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
	}

	// Convert input messages to events.Message format and append new message
	allMessages := make([]events.Message, 0, len(input.Messages)+1)

	// Convert input messages to the expected format
	for _, msg := range input.Messages {
		eventMsg := events.Message{
			Role: "",
		}

		// Extract fields from the map
		if id, ok := msg["id"].(string); ok {
			eventMsg.ID = id
		}
		if role, ok := msg["role"].(string); ok {
			eventMsg.Role = role
		}
		if content, ok := msg["content"].(string); ok {
			eventMsg.Content = &content
		}

		// Handle tool calls if present
		if toolCalls, ok := msg["tool_calls"].([]interface{}); ok {
			eventMsg.ToolCalls = make([]events.ToolCall, 0, len(toolCalls))
			for _, tc := range toolCalls {
				if tcMap, ok := tc.(map[string]interface{}); ok {
					toolCall := events.ToolCall{}
					if id, ok := tcMap["id"].(string); ok {
						toolCall.ID = id
					}
					if tcType, ok := tcMap["type"].(string); ok {
						toolCall.Type = tcType
					}
					if function, ok := tcMap["function"].(map[string]interface{}); ok {
						if name, ok := function["name"].(string); ok {
							toolCall.Function.Name = name
						}
						if args, ok := function["arguments"].(string); ok {
							toolCall.Function.Arguments = args
						}
					}
					eventMsg.ToolCalls = append(eventMsg.ToolCalls, toolCall)
				}
			}
		}

		allMessages = append(allMessages, eventMsg)
	}

	// Add the new message
	allMessages = append(allMessages, newMessage)

	// Check for cancellation before sending messages
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected before messages snapshot", append(logCtx, "reason", "context_canceled")...)
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
		logger.Debug("Client disconnected before RUN_FINISHED", append(logCtx, "reason", "context_canceled")...)
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
