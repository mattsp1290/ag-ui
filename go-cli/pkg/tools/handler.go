package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ToolCallHandler manages the tool call request/response lifecycle
type ToolCallHandler struct {
	registry *ToolRegistry
	client   *http.Client
	logger   *logrus.Logger
	config   ToolHandlerConfig
}

// ToolHandlerConfig holds configuration for the tool handler
type ToolHandlerConfig struct {
	// ServerURL is the base URL of the AG-UI server
	ServerURL string
	
	// Endpoint to post tool results to (e.g., "/tool_based_generative_ui")
	Endpoint string
	
	// Headers to include in requests
	Headers map[string]string
	
	// Timeout for HTTP requests
	Timeout time.Duration
	
	// Interactive mode for prompting user for arguments
	Interactive bool
	
	// ToolArgs for non-interactive mode (JSON string)
	ToolArgs string
	
	// RetryAttempts for network failures
	RetryAttempts int
	
	// RetryDelay between attempts
	RetryDelay time.Duration
}

// NewToolCallHandler creates a new tool call handler
func NewToolCallHandler(registry *ToolRegistry, config ToolHandlerConfig, logger *logrus.Logger) *ToolCallHandler {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}
	if logger == nil {
		logger = logrus.New()
	}
	
	client := &http.Client{
		Timeout: config.Timeout,
	}
	
	return &ToolCallHandler{
		registry: registry,
		client:   client,
		logger:   logger,
		config:   config,
	}
}

// HandleToolCallRequest processes a tool call from an assistant message
func (h *ToolCallHandler) HandleToolCallRequest(ctx context.Context, toolCall *ToolCall, threadID, runID string, messages []interface{}) error {
	h.logger.WithFields(logrus.Fields{
		"tool_call_id": toolCall.ID,
		"tool_name":    toolCall.Function.Name,
		"thread_id":    threadID,
		"run_id":       runID,
	}).Info("Handling tool call request")
	
	// Get tool schema from registry
	tool, exists := h.registry.Get(toolCall.Function.Name)
	if !exists {
		return fmt.Errorf("tool '%s' not found in registry", toolCall.Function.Name)
	}
	
	// Parse and validate arguments
	args, err := h.parseAndValidateArgs(toolCall, tool)
	if err != nil {
		return fmt.Errorf("failed to validate arguments: %w", err)
	}
	
	// Execute the tool (this would be implemented based on the specific tool)
	result, err := h.executeTool(ctx, tool, args)
	if err != nil {
		return fmt.Errorf("tool execution failed: %w", err)
	}
	
	// Create tool result message
	toolResultMsg := h.createToolResultMessage(toolCall.ID, result)
	
	// Append to messages array
	updatedMessages := append(messages, toolResultMsg)
	
	// Submit back to server
	return h.submitToolResult(ctx, threadID, runID, updatedMessages)
}

// parseAndValidateArgs parses and validates tool arguments
func (h *ToolCallHandler) parseAndValidateArgs(toolCall *ToolCall, tool *Tool) (map[string]interface{}, error) {
	var args map[string]interface{}
	
	// Parse arguments from tool call
	if toolCall.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
		}
	} else {
		args = make(map[string]interface{})
	}
	
	// In non-interactive mode with tool args provided
	if !h.config.Interactive && h.config.ToolArgs != "" {
		var providedArgs map[string]interface{}
		if err := json.Unmarshal([]byte(h.config.ToolArgs), &providedArgs); err != nil {
			return nil, fmt.Errorf("failed to parse provided tool args: %w", err)
		}
		// Merge provided args
		for k, v := range providedArgs {
			args[k] = v
		}
	}
	
	// Validate against schema
	if tool.Parameters != nil {
		validator := NewValidator(tool.Parameters)
		if err := validator.Validate(args); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
	}
	
	h.logger.WithField("args", args).Debug("Tool arguments validated")
	
	return args, nil
}

// executeTool executes the tool with the given arguments
func (h *ToolCallHandler) executeTool(ctx context.Context, tool *Tool, args map[string]interface{}) (string, error) {
	// This is a placeholder - actual tool execution would depend on the tool type
	// For now, we'll return a structured JSON response
	
	result := map[string]interface{}{
		"status": "success",
		"tool":   tool.Name,
		"args":   args,
		"output": fmt.Sprintf("Tool '%s' executed successfully", tool.Name),
	}
	
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool result: %w", err)
	}
	
	return string(resultJSON), nil
}

// createToolResultMessage creates a tool result message
func (h *ToolCallHandler) createToolResultMessage(toolCallID, content string) map[string]interface{} {
	return map[string]interface{}{
		"id":         uuid.New().String(),
		"role":       "tool",
		"content":    content,
		"toolCallId": toolCallID, // camelCase per spec
	}
}

// submitToolResult submits the tool result back to the server
func (h *ToolCallHandler) submitToolResult(ctx context.Context, threadID, runID string, messages []interface{}) error {
	// Construct request body
	requestBody := map[string]interface{}{
		"thread_id":       threadID,
		"run_id":          runID,
		"messages":        messages,
		"state":           map[string]interface{}{},
		"tools":           []interface{}{},
		"context":         []interface{}{},
		"forwarded_props": map[string]interface{}{},
	}
	
	bodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}
	
	url := strings.TrimSuffix(h.config.ServerURL, "/") + h.config.Endpoint
	
	// Retry logic
	var lastErr error
	for attempt := 0; attempt < h.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(h.config.RetryDelay):
			}
		}
		
		// Create request
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
		if err != nil {
			lastErr = err
			continue
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		for k, v := range h.config.Headers {
			req.Header.Set(k, v)
		}
		
		// Send request
		resp, err := h.client.Do(req)
		if err != nil {
			lastErr = err
			h.logger.WithError(err).WithField("attempt", attempt+1).Warn("Request failed, retrying")
			continue
		}
		
		// Check response
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
			h.logger.WithError(lastErr).WithField("attempt", attempt+1).Warn("Bad status code, retrying")
			continue
		}
		
		// Success - the SSE stream will be handled by the main client
		resp.Body.Close()
		h.logger.Info("Tool result submitted successfully")
		return nil
	}
	
	return fmt.Errorf("failed after %d attempts: %w", h.config.RetryAttempts, lastErr)
}

// ProcessMessagesSnapshot processes a MESSAGES_SNAPSHOT event to detect tool calls
func (h *ToolCallHandler) ProcessMessagesSnapshot(ctx context.Context, messages []interface{}, threadID, runID string) error {
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		
		// Check if this is an assistant message with tool calls
		if role, ok := msgMap["role"].(string); ok && role == "assistant" {
			if toolCalls, ok := msgMap["toolCalls"].([]interface{}); ok && len(toolCalls) > 0 {
				h.logger.WithField("tool_calls_count", len(toolCalls)).Info("Detected tool calls in assistant message")
				
				// Process each tool call
				for _, tc := range toolCalls {
					tcMap, ok := tc.(map[string]interface{})
					if !ok {
						continue
					}
					
					toolCall := &ToolCall{}
					
					// Parse tool call fields
					if id, ok := tcMap["id"].(string); ok {
						toolCall.ID = id
					}
					if typ, ok := tcMap["type"].(string); ok {
						toolCall.Type = typ
					}
					if fn, ok := tcMap["function"].(map[string]interface{}); ok {
						if name, ok := fn["name"].(string); ok {
							toolCall.Function.Name = name
						}
						if args, ok := fn["arguments"].(string); ok {
							toolCall.Function.Arguments = args
						}
					}
					
					// Handle the tool call
					if err := h.HandleToolCallRequest(ctx, toolCall, threadID, runID, messages); err != nil {
						h.logger.WithError(err).WithField("tool_call_id", toolCall.ID).Error("Failed to handle tool call")
						// Continue processing other tool calls
					}
				}
				
				// We only process the first assistant message with tool calls
				break
			}
		}
	}
	
	return nil
}