package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	
	"github.com/sirupsen/logrus"
)

// ToolCallRequest represents a tool invocation request
type ToolCallRequest struct {
	ToolCallID string                 `json:"toolCallId"`
	ToolName   string                 `json:"toolName"`
	Arguments  map[string]interface{} `json:"arguments"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCallResponse represents a tool invocation response
type ToolCallResponse struct {
	ToolCallID string      `json:"toolCallId"`
	ToolName   string      `json:"toolName"`
	Result     interface{} `json:"result,omitempty"`
	Error      *ToolError  `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ToolHandler handles tool invocations with retry logic
type ToolHandler struct {
	retryManager *RetryManager
	logger       *logrus.Logger
	
	// Track active tool calls for idempotency
	activeCalls map[string]*ToolCallState
	mu          sync.RWMutex
	
	// Metrics
	totalCalls      int64
	successfulCalls int64
	failedCalls     int64
	retriedCalls    int64
	
	// Callbacks for tool execution (to be implemented by the client)
	executor ToolExecutor
}

// ToolCallState tracks the state of an active tool call
type ToolCallState struct {
	Request       *ToolCallRequest
	Response      *ToolCallResponse
	StartTime     time.Time
	EndTime       *time.Time
	AttemptCount  int
	IsComplete    bool
	IsCancelled   bool
}

// ToolExecutor defines the interface for executing tool calls
type ToolExecutor interface {
	Execute(ctx context.Context, request *ToolCallRequest) (*ToolCallResponse, error)
}

// NewToolHandler creates a new tool handler
func NewToolHandler(config RetryConfig, executor ToolExecutor) *ToolHandler {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}
	
	return &ToolHandler{
		retryManager: NewRetryManager(config),
		logger:       config.Logger,
		activeCalls:  make(map[string]*ToolCallState),
		executor:     executor,
	}
}

// HandleToolCall handles a tool call request with retry logic
func (h *ToolHandler) HandleToolCall(ctx context.Context, request *ToolCallRequest) (*ToolCallResponse, error) {
	h.mu.Lock()
	
	// Check for duplicate/idempotent request
	if state, exists := h.activeCalls[request.ToolCallID]; exists {
		h.mu.Unlock()
		
		// If call is complete, return cached response
		if state.IsComplete {
			h.logger.WithFields(logrus.Fields{
				"toolCallId": request.ToolCallID,
				"toolName":   request.ToolName,
			}).Info("Returning cached response for idempotent request")
			return state.Response, nil
		}
		
		// If call is still in progress, wait for it
		h.logger.WithFields(logrus.Fields{
			"toolCallId": request.ToolCallID,
			"toolName":   request.ToolName,
		}).Info("Tool call already in progress, waiting...")
		
		// Wait for completion with timeout
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		
		return h.waitForCompletion(waitCtx, request.ToolCallID)
	}
	
	// Create new state
	state := &ToolCallState{
		Request:      request,
		StartTime:    time.Now(),
		AttemptCount: 0,
	}
	h.activeCalls[request.ToolCallID] = state
	h.totalCalls++
	h.mu.Unlock()
	
	// Execute with retry logic
	response, err := h.executeWithRetry(ctx, request)
	
	// Update state
	h.mu.Lock()
	now := time.Now()
	state.EndTime = &now
	state.IsComplete = true
	state.Response = response
	
	if err != nil {
		h.failedCalls++
	} else {
		h.successfulCalls++
	}
	
	if state.AttemptCount > 1 {
		h.retriedCalls++
	}
	h.mu.Unlock()
	
	return response, err
}

// executeWithRetry executes a tool call with retry logic
func (h *ToolHandler) executeWithRetry(ctx context.Context, request *ToolCallRequest) (*ToolCallResponse, error) {
	attemptCount := 0
	startTime := time.Now()
	
	for {
		attemptCount++
		
		// Update attempt count
		h.mu.Lock()
		if state, exists := h.activeCalls[request.ToolCallID]; exists {
			state.AttemptCount = attemptCount
		}
		h.mu.Unlock()
		
		// Create attempt context with timeout
		attemptCtx := ctx
		if h.retryManager.config.PerAttemptTimeout > 0 {
			var cancel context.CancelFunc
			attemptCtx, cancel = context.WithTimeout(ctx, h.retryManager.config.PerAttemptTimeout)
			defer cancel()
		}
		
		// Execute the tool call
		h.logger.WithFields(logrus.Fields{
			"toolCallId":   request.ToolCallID,
			"toolName":     request.ToolName,
			"attemptCount": attemptCount,
		}).Info("Executing tool call")
		
		response, err := h.executor.Execute(attemptCtx, request)
		
		if err == nil {
			// Success
			response.Duration = time.Since(startTime)
			h.retryManager.RecordSuccess(request.ToolCallID)
			
			h.logger.WithFields(logrus.Fields{
				"toolCallId":   request.ToolCallID,
				"toolName":     request.ToolName,
				"attemptCount": attemptCount,
				"duration":     response.Duration,
			}).Info("Tool call succeeded")
			
			return response, nil
		}
		
		// Check if we should retry
		shouldRetry, delay, retryErr := h.retryManager.ShouldRetry(ctx, request.ToolCallID, err)
		
		if !shouldRetry {
			// No retry, return error
			toolErr, ok := retryErr.(*ToolError)
			if !ok {
				toolErr = h.retryManager.classifier.Classify(retryErr, request.ToolName, request.ToolCallID)
			}
			
			response = &ToolCallResponse{
				ToolCallID: request.ToolCallID,
				ToolName:   request.ToolName,
				Error:      toolErr,
				Duration:   time.Since(startTime),
			}
			
			h.logger.WithFields(logrus.Fields{
				"toolCallId":   request.ToolCallID,
				"toolName":     request.ToolName,
				"attemptCount": attemptCount,
				"error":        toolErr.Error(),
			}).Error("Tool call failed, no retry")
			
			return response, toolErr
		}
		
		// Wait before retry
		h.logger.WithFields(logrus.Fields{
			"toolCallId":   request.ToolCallID,
			"toolName":     request.ToolName,
			"attemptCount": attemptCount,
			"delay":        delay,
		}).Info("Waiting before retry")
		
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			// Context cancelled
			return nil, ctx.Err()
		}
	}
}

// waitForCompletion waits for a tool call to complete
func (h *ToolHandler) waitForCompletion(ctx context.Context, toolCallID string) (*ToolCallResponse, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			h.mu.RLock()
			state, exists := h.activeCalls[toolCallID]
			if !exists {
				h.mu.RUnlock()
				return nil, fmt.Errorf("tool call %s not found", toolCallID)
			}
			
			if state.IsComplete {
				response := state.Response
				h.mu.RUnlock()
				return response, nil
			}
			
			if state.IsCancelled {
				h.mu.RUnlock()
				return nil, fmt.Errorf("tool call %s was cancelled", toolCallID)
			}
			h.mu.RUnlock()
			
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// CancelToolCall cancels an active tool call
func (h *ToolHandler) CancelToolCall(toolCallID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	state, exists := h.activeCalls[toolCallID]
	if !exists {
		return fmt.Errorf("tool call %s not found", toolCallID)
	}
	
	if state.IsComplete {
		return fmt.Errorf("tool call %s already complete", toolCallID)
	}
	
	state.IsCancelled = true
	
	h.logger.WithFields(logrus.Fields{
		"toolCallId": toolCallID,
		"toolName":   state.Request.ToolName,
	}).Info("Tool call cancelled")
	
	return nil
}

// GetMetrics returns current handler metrics
func (h *ToolHandler) GetMetrics() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	successRate := float64(0)
	if h.totalCalls > 0 {
		successRate = float64(h.successfulCalls) / float64(h.totalCalls) * 100
	}
	
	retryRate := float64(0)
	if h.totalCalls > 0 {
		retryRate = float64(h.retriedCalls) / float64(h.totalCalls) * 100
	}
	
	activeCalls := 0
	for _, state := range h.activeCalls {
		if !state.IsComplete && !state.IsCancelled {
			activeCalls++
		}
	}
	
	return map[string]interface{}{
		"totalCalls":      h.totalCalls,
		"successfulCalls": h.successfulCalls,
		"failedCalls":     h.failedCalls,
		"retriedCalls":    h.retriedCalls,
		"activeCalls":     activeCalls,
		"successRate":     fmt.Sprintf("%.2f%%", successRate),
		"retryRate":       fmt.Sprintf("%.2f%%", retryRate),
		"retryMetrics":    h.retryManager.Metrics(),
	}
}

// ToJSON converts metrics to JSON
func (h *ToolHandler) MetricsJSON(pretty bool) ([]byte, error) {
	metrics := h.GetMetrics()
	if pretty {
		return json.MarshalIndent(metrics, "", "  ")
	}
	return json.Marshal(metrics)
}

// Reset clears all handler state
func (h *ToolHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.activeCalls = make(map[string]*ToolCallState)
	h.totalCalls = 0
	h.successfulCalls = 0
	h.failedCalls = 0
	h.retriedCalls = 0
	h.retryManager.Reset()
	
	h.logger.Info("Tool handler state reset")
}