package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
	
	"github.com/ag-ui/go-sdk/examples/client/internal/ui"
	"github.com/sirupsen/logrus"
)

// StreamIntegration provides integration between SSE events and tool handling
type StreamIntegration struct {
	handler  *ToolHandler
	renderer *ui.Renderer
	logger   *logrus.Logger
	
	// Exit code management
	exitCode     int
	shouldExit   bool
}

// NewStreamIntegration creates a new stream integration
func NewStreamIntegration(config RetryConfig, renderer *ui.Renderer) *StreamIntegration {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}
	
	// Create a simple executor for now (will be replaced with actual implementation)
	executor := &SimpleToolExecutor{
		logger: config.Logger,
	}
	
	return &StreamIntegration{
		handler:  NewToolHandler(config, executor),
		renderer: renderer,
		logger:   config.Logger,
	}
}

// HandleSSEEvent processes an SSE event and handles tool-related events
func (si *StreamIntegration) HandleSSEEvent(eventType string, data json.RawMessage) error {
	switch eventType {
	case "TOOL_CALL_REQUESTED":
		return si.handleToolCallRequested(data)
	case "TOOL_ERROR":
		return si.renderer.HandleToolError(eventType, data)
	case "TOOL_RETRY":
		return si.renderer.HandleToolRetry(eventType, data)
	default:
		// Pass through to renderer for non-tool events
		return si.renderer.HandleEvent(eventType, data)
	}
}

// handleToolCallRequested handles a tool call request event
func (si *StreamIntegration) handleToolCallRequested(data json.RawMessage) error {
	var request ToolCallRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return fmt.Errorf("failed to unmarshal TOOL_CALL_REQUESTED: %w", err)
	}
	
	// Create context with timeout
	ctx := context.Background()
	if si.handler.retryManager.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, si.handler.retryManager.config.Timeout)
		defer cancel()
	}
	
	// Handle the tool call with retry logic
	response, err := si.handler.HandleToolCall(ctx, &request)
	
	if err != nil {
		// Tool call failed after all retries
		toolErr, ok := err.(*ToolError)
		if !ok {
			toolErr = si.handler.retryManager.classifier.Classify(err, request.ToolName, request.ToolCallID)
		}
		
		// Render error
		errorData, _ := json.Marshal(toolErr)
		si.renderer.HandleToolError("TOOL_ERROR", errorData)
		
		// Set exit code based on error type
		si.updateExitCode(toolErr)
		
		// Check if we should abort
		if si.handler.retryManager.config.OnError == RetryPolicyAbort {
			si.shouldExit = true
			si.logger.WithFields(logrus.Fields{
				"toolCallId": request.ToolCallID,
				"toolName":   request.ToolName,
				"error":      toolErr.Error(),
			}).Error("Aborting due to tool error")
		}
	} else {
		// Tool call succeeded
		resultData, _ := json.Marshal(map[string]interface{}{
			"toolCallId": response.ToolCallID,
			"result":     response.Result,
		})
		si.renderer.HandleEvent("TOOL_CALL_RESULT", resultData)
	}
	
	return nil
}

// updateExitCode updates the exit code based on error type
func (si *StreamIntegration) updateExitCode(err *ToolError) {
	// Map error codes to exit codes
	switch err.Code {
	case ErrorCodeTimeout:
		si.exitCode = 124 // Standard timeout exit code
	case ErrorCodePermission:
		si.exitCode = 77 // Permission denied
	case ErrorCodeNotFound:
		si.exitCode = 127 // Command not found
	case ErrorCodeValidation, ErrorCodeInvalidInput:
		si.exitCode = 22 // Invalid argument
	case ErrorCodeNetwork:
		si.exitCode = 101 // Network unreachable
	case ErrorCodeInternal:
		si.exitCode = 70 // Internal software error
	default:
		si.exitCode = 1 // General error
	}
}

// GetExitCode returns the current exit code
func (si *StreamIntegration) GetExitCode() int {
	if si.exitCode == 0 && si.handler.failedCalls > 0 {
		return 1 // Default error code if any calls failed
	}
	return si.exitCode
}

// ShouldExit returns whether the stream should exit
func (si *StreamIntegration) ShouldExit() bool {
	return si.shouldExit
}

// GetMetrics returns current metrics
func (si *StreamIntegration) GetMetrics() map[string]interface{} {
	return si.handler.GetMetrics()
}

// PrintMetrics prints metrics to the logger
func (si *StreamIntegration) PrintMetrics() {
	metrics := si.GetMetrics()
	
	si.logger.WithFields(logrus.Fields{
		"totalCalls":      metrics["totalCalls"],
		"successfulCalls": metrics["successfulCalls"],
		"failedCalls":     metrics["failedCalls"],
		"retriedCalls":    metrics["retriedCalls"],
		"successRate":     metrics["successRate"],
		"retryRate":       metrics["retryRate"],
	}).Info("Tool call metrics")
}

// CleanExit performs a clean exit with proper terminal state
func (si *StreamIntegration) CleanExit() {
	// Clear any pending output
	fmt.Fprintln(os.Stdout)
	
	// Print final metrics if there were tool calls
	if si.handler.totalCalls > 0 {
		si.PrintMetrics()
	}
	
	// Set exit code
	exitCode := si.GetExitCode()
	if exitCode != 0 {
		si.logger.WithField("exitCode", exitCode).Info("Exiting with error")
		os.Exit(exitCode)
	}
}

// SimpleToolExecutor is a placeholder executor for testing
type SimpleToolExecutor struct {
	logger *logrus.Logger
}

// Execute simulates tool execution
func (e *SimpleToolExecutor) Execute(ctx context.Context, request *ToolCallRequest) (*ToolCallResponse, error) {
	// Simulate processing time
	select {
	case <-time.After(100 * time.Millisecond):
		// Success case (for now, always succeed)
		return &ToolCallResponse{
			ToolCallID: request.ToolCallID,
			ToolName:   request.ToolName,
			Result: map[string]interface{}{
				"status":  "success",
				"message": fmt.Sprintf("Tool %s executed successfully", request.ToolName),
			},
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}