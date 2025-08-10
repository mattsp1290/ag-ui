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
)

// AgenticGenerativeUIHandler creates a Fiber handler for the agentic generative UI route
func AgenticGenerativeUIHandler(cfg *config.Config) fiber.Handler {
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

		logger.Info("Agentic generative UI SSE connection established", logCtx...)

		// Get request context for cancellation
		ctx := c.RequestCtx()

		// Start streaming
		return c.SendStreamWriter(func(w *bufio.Writer) {
			if err := streamAgenticGenerativeUIEvents(ctx, w, sseWriter, cfg, logger, logCtx); err != nil {
				logger.Error("Error streaming agentic generative UI events", append(logCtx, "error", err)...)
			}
		})
	}
}

// streamAgenticGenerativeUIEvents implements the deterministic state update sequence
func streamAgenticGenerativeUIEvents(reqCtx context.Context, w *bufio.Writer, sseWriter *encoding.SSEWriter, cfg *config.Config, logger *slog.Logger, logCtx []any) error {
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

	// Initialize state with 10 steps
	initialState := map[string]interface{}{
		"steps": []map[string]interface{}{
			{"description": "Step 1", "status": "pending"},
			{"description": "Step 2", "status": "pending"},
			{"description": "Step 3", "status": "pending"},
			{"description": "Step 4", "status": "pending"},
			{"description": "Step 5", "status": "pending"},
			{"description": "Step 6", "status": "pending"},
			{"description": "Step 7", "status": "pending"},
			{"description": "Step 8", "status": "pending"},
			{"description": "Step 9", "status": "pending"},
			{"description": "Step 10", "status": "pending"},
		},
	}

	// Send initial state snapshot
	stateSnapshot := events.NewStateSnapshotEvent(initialState)
	if err := sseWriter.WriteEvent(ctx, w, stateSnapshot); err != nil {
		return fmt.Errorf("failed to write STATE_SNAPSHOT event: %w", err)
	}

	// Brief pause after initial snapshot
	time.Sleep(200 * time.Millisecond)

	// Check for cancellation
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected after initial snapshot", append(logCtx, "reason", "context_cancelled")...)
		return nil
	}

	// Update each step with delta events
	for i := 0; i < 10; i++ {
		// Check for cancellation before each step
		if err := reqCtx.Err(); err != nil {
			logger.Debug("Client disconnected during step updates", append(logCtx, "step", i+1, "reason", "context_cancelled")...)
			return nil
		}

		// Create JSON patch operation to update step status
		delta := []events.JSONPatchOperation{
			{
				Op:    "replace",
				Path:  fmt.Sprintf("/steps/%d/status", i),
				Value: "completed",
			},
		}

		// Send state delta event
		stateDelta := events.NewStateDeltaEvent(delta)
		if err := sseWriter.WriteEvent(ctx, w, stateDelta); err != nil {
			return fmt.Errorf("failed to write STATE_DELTA event for step %d: %w", i+1, err)
		}

		// Add configurable pacing between updates (default ~250ms)
		updateDelay := 250 * time.Millisecond
		if cfg.StreamingChunkDelay > 0 {
			updateDelay = cfg.StreamingChunkDelay
		}
		time.Sleep(updateDelay)
	}

	// Check for cancellation before final snapshot
	if err := reqCtx.Err(); err != nil {
		logger.Debug("Client disconnected before final snapshot", append(logCtx, "reason", "context_cancelled")...)
		return nil
	}

	// Send final state snapshot with all steps completed
	finalState := map[string]interface{}{
		"steps": []map[string]interface{}{
			{"description": "Step 1", "status": "completed"},
			{"description": "Step 2", "status": "completed"},
			{"description": "Step 3", "status": "completed"},
			{"description": "Step 4", "status": "completed"},
			{"description": "Step 5", "status": "completed"},
			{"description": "Step 6", "status": "completed"},
			{"description": "Step 7", "status": "completed"},
			{"description": "Step 8", "status": "completed"},
			{"description": "Step 9", "status": "completed"},
			{"description": "Step 10", "status": "completed"},
		},
	}

	finalSnapshot := events.NewStateSnapshotEvent(finalState)
	if err := sseWriter.WriteEvent(ctx, w, finalSnapshot); err != nil {
		return fmt.Errorf("failed to write final STATE_SNAPSHOT event: %w", err)
	}

	// Send RUN_FINISHED event
	runFinished := events.NewRunFinishedEvent(threadID, runID)
	if err := sseWriter.WriteEvent(ctx, w, runFinished); err != nil {
		return fmt.Errorf("failed to write RUN_FINISHED event: %w", err)
	}

	logger.Info("Agentic generative UI event sequence completed successfully", logCtx...)
	return nil
}