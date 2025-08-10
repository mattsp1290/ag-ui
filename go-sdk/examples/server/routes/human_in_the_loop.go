package routes

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/encoding"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
)

// RunAgentInput represents the input structure for the human-in-the-loop endpoint
type RunAgentInput struct {
	Messages []map[string]interface{} `json:"messages"`
}

// parseMessages converts the raw message maps to message objects
func (r *RunAgentInput) parseMessages() ([]messages.Message, error) {
	var result []messages.Message
	for _, msgData := range r.Messages {
		role, ok := msgData["role"].(string)
		if !ok {
			return nil, fmt.Errorf("message missing role field")
		}
		
		content := ""
		if c, ok := msgData["content"].(string); ok {
			content = c
		}
		
		msg := messages.NewMessage(messages.MessageRole(role), content)
		result = append(result, msg)
	}
	return result, nil
}

// HumanInTheLoopHandler creates a Fiber handler for the human-in-the-loop route
func HumanInTheLoopHandler(cfg *config.Config) fiber.Handler {
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
		var input RunAgentInput
		if err := c.Bind().JSON(&input); err != nil {
			logger.Error("Failed to parse request body", append(logCtx, "error", err)...)
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		if len(input.Messages) == 0 {
			logger.Error("No messages provided in request", logCtx...)
			return c.Status(400).JSON(fiber.Map{
				"error": "Messages array cannot be empty",
			})
		}

		// Parse messages
		parsedMessages, err := input.parseMessages()
		if err != nil {
			logger.Error("Failed to parse messages", append(logCtx, "error", err)...)
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid message format",
			})
		}

		// Set SSE headers after validation
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Headers", "Cache-Control")

		logger.Info("Human-in-the-loop SSE connection established", logCtx...)

		// Get request context for cancellation
		ctx := c.RequestCtx()

		// Start streaming
		return c.SendStreamWriter(func(w *bufio.Writer) {
			if err := streamHumanInTheLoopEvents(ctx, w, sseWriter, parsedMessages, cfg, logger, logCtx); err != nil {
				logger.Error("Error streaming human-in-the-loop events", append(logCtx, "error", err)...)
			}
		})
	}
}

// streamHumanInTheLoopEvents implements the branching event sequence
func streamHumanInTheLoopEvents(reqCtx context.Context, w *bufio.Writer, sseWriter *encoding.SSEWriter, parsedMessages []messages.Message, cfg *config.Config, logger *slog.Logger, logCtx []any) error {
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

	// Get last message to determine branching logic
	lastMessage := parsedMessages[len(parsedMessages)-1]
	lastMessageRole := lastMessage.GetRole()

	// Branch based on last message role
	if lastMessageRole == messages.RoleTool {
		// Assistant text branch - respond to tool result
		if err := streamAssistantTextResponse(reqCtx, w, sseWriter, logger, logCtx); err != nil {
			return err
		}
	} else {
		// Tool call branch - call generate_task_steps
		if err := streamGenerateTaskSteps(reqCtx, w, sseWriter, cfg.StreamingChunkDelay, logger, logCtx); err != nil {
			return err
		}
	}

	// Send RUN_FINISHED event
	runFinished := events.NewRunFinishedEvent(threadID, runID)
	if err := sseWriter.WriteEvent(ctx, w, runFinished); err != nil {
		return fmt.Errorf("failed to write RUN_FINISHED event: %w", err)
	}

	logger.Info("Human-in-the-loop event sequence completed successfully", logCtx...)
	return nil
}

// streamAssistantTextResponse handles the assistant text response branch
func streamAssistantTextResponse(reqCtx context.Context, w *bufio.Writer, sseWriter *encoding.SSEWriter, logger *slog.Logger, logCtx []any) error {
	messageID := events.GenerateMessageID()
	ctx := context.Background()

	// Send TEXT_MESSAGE_START event
	msgStart := events.NewTextMessageStartEvent(messageID, events.WithRole("assistant"))
	if err := sseWriter.WriteEvent(ctx, w, msgStart); err != nil {
		return fmt.Errorf("failed to write TEXT_MESSAGE_START event: %w", err)
	}

	// Check for cancellation
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected during message start", append(logCtx, "reason", "context_cancelled")...)
		return nil
	}

	// Send assistant response content
	responseContent := "Thank you for using the tool. Based on the results, I can see that the task has been completed successfully. Is there anything else you'd like me to help you with?"
	
	msgContent := events.NewTextMessageContentEvent(messageID, responseContent)
	if err := sseWriter.WriteEvent(ctx, w, msgContent); err != nil {
		return fmt.Errorf("failed to write TEXT_MESSAGE_CONTENT event: %w", err)
	}

	// Check for cancellation
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected during message content", append(logCtx, "reason", "context_cancelled")...)
		return nil
	}

	// Send TEXT_MESSAGE_END event
	msgEnd := events.NewTextMessageEndEvent(messageID)
	if err := sseWriter.WriteEvent(ctx, w, msgEnd); err != nil {
		return fmt.Errorf("failed to write TEXT_MESSAGE_END event: %w", err)
	}

	return nil
}

// streamGenerateTaskSteps handles the tool call branch with incremental JSON streaming
func streamGenerateTaskSteps(reqCtx context.Context, w *bufio.Writer, sseWriter *encoding.SSEWriter, chunkDelay time.Duration, logger *slog.Logger, logCtx []any) error {
	toolCallID := events.GenerateToolCallID()
	ctx := context.Background()

	// Send TOOL_CALL_START event
	toolStart := events.NewToolCallStartEvent(toolCallID, "generate_task_steps")
	if err := sseWriter.WriteEvent(ctx, w, toolStart); err != nil {
		return fmt.Errorf("failed to write TOOL_CALL_START event: %w", err)
	}

	// Check for cancellation
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected during tool start", append(logCtx, "reason", "context_cancelled")...)
		return nil
	}

	// Stream incremental JSON args for the steps array
	if err := streamToolCallArgs(reqCtx, w, sseWriter, toolCallID, chunkDelay, logger, logCtx); err != nil {
		return err
	}

	// Send TOOL_CALL_END event
	toolEnd := events.NewToolCallEndEvent(toolCallID)
	if err := sseWriter.WriteEvent(ctx, w, toolEnd); err != nil {
		return fmt.Errorf("failed to write TOOL_CALL_END event: %w", err)
	}

	return nil
}

// streamToolCallArgs streams incremental JSON arguments that build the steps array
func streamToolCallArgs(reqCtx context.Context, w *bufio.Writer, sseWriter *encoding.SSEWriter, toolCallID string, chunkDelay time.Duration, logger *slog.Logger, logCtx []any) error {
	ctx := context.Background()

	// Define the steps that will be built incrementally
	steps := []string{
		"Analyze the user's request to understand the requirements",
		"Break down the task into manageable components",
		"Research any necessary background information or context",
		"Identify potential challenges or obstacles",
		"Create a step-by-step implementation plan",
		"Gather required resources and tools",
		"Set up the development environment if needed",
		"Implement the core functionality",
		"Test the implementation thoroughly",
		"Document the solution and provide examples",
	}

	// Start with opening bracket and "steps" key
	openingChunk := `{"steps":[`
	toolArgs := events.NewToolCallArgsEvent(toolCallID, openingChunk)
	if err := sseWriter.WriteEvent(ctx, w, toolArgs); err != nil {
		return fmt.Errorf("failed to write opening TOOL_CALL_ARGS event: %w", err)
	}

	// Check for cancellation and add pacing
	time.Sleep(chunkDelay)
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected during opening args", append(logCtx, "reason", "context_cancelled")...)
		return nil
	}

	// Stream each step with proper JSON formatting
	for i, step := range steps {
		// Create JSON for this step
		stepJSON := fmt.Sprintf(`{"step":%d,"description":"%s"}`, i+1, step)
		
		// Add comma if not the first step
		if i > 0 {
			stepJSON = "," + stepJSON
		}

		// Send the step chunk
		stepArgs := events.NewToolCallArgsEvent(toolCallID, stepJSON)
		if err := sseWriter.WriteEvent(ctx, w, stepArgs); err != nil {
			return fmt.Errorf("failed to write step %d TOOL_CALL_ARGS event: %w", i+1, err)
		}

		// Check for cancellation and add pacing
		time.Sleep(chunkDelay)
		if err := reqCtx.Err(); err != nil {
			logger.Debug("Client disconnected during step args", append(logCtx, "step", i+1, "reason", "context_cancelled")...)
			return nil
		}
	}

	// Send closing bracket
	closingChunk := `]}`
	closingArgs := events.NewToolCallArgsEvent(toolCallID, closingChunk)
	if err := sseWriter.WriteEvent(ctx, w, closingArgs); err != nil {
		return fmt.Errorf("failed to write closing TOOL_CALL_ARGS event: %w", err)
	}

	return nil
}