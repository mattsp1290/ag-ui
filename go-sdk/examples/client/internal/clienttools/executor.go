package clienttools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ToolFunc is the function signature for client-side tools
type ToolFunc func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// Tool represents a client-side tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Execute     ToolFunc               `json:"-"`
	// Store the original parameter definition for validation
	ParamDef    *ToolParameters        `json:"-"`
}

// Executor manages client-side tool execution
type Executor struct {
	mu       sync.RWMutex
	tools    map[string]*Tool
	metrics  *ExecutionMetrics
	maxRetry int
	timeout  time.Duration
}

// ExecutionMetrics tracks tool execution statistics
type ExecutionMetrics struct {
	mu         sync.RWMutex
	executions map[string]*ToolMetrics
}

// ToolMetrics contains metrics for a specific tool
type ToolMetrics struct {
	TotalCalls      int64
	SuccessfulCalls int64
	FailedCalls     int64
	TotalDuration   time.Duration
	LastExecution   time.Time
}

// ErrorType categorizes different types of errors
type ErrorType string

const (
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeExecution  ErrorType = "execution"
	ErrorTypeTimeout    ErrorType = "timeout"
	ErrorTypeNotFound   ErrorType = "not_found"
	ErrorTypeRetryable  ErrorType = "retryable"
)

// ExecutionError provides detailed error information
type ExecutionError struct {
	Type       ErrorType              `json:"type"`
	Message    string                 `json:"message"`
	ToolName   string                 `json:"tool_name"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Suggestion string                 `json:"suggestion,omitempty"`
	Retryable  bool                   `json:"retryable"`
	Wrapped    error                  `json:"-"`
}

func (e *ExecutionError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("%s: %s (suggestion: %s)", e.Type, e.Message, e.Suggestion)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// ExecutionResult contains the result of a tool execution
type ExecutionResult struct {
	ToolName  string                 `json:"tool_name"`
	Success   bool                   `json:"success"`
	Result    interface{}            `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	ErrorType ErrorType              `json:"error_type,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewExecutor creates a new client-side tool executor
func NewExecutor() *Executor {
	return &Executor{
		tools: make(map[string]*Tool),
		metrics: &ExecutionMetrics{
			executions: make(map[string]*ToolMetrics),
		},
		maxRetry: 3,
		timeout:  30 * time.Second,
	}
}

// RegisterTool registers a client-side tool
func (e *Executor) RegisterTool(tool *Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if tool.Execute == nil {
		return fmt.Errorf("tool execute function cannot be nil")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.tools[tool.Name]; exists {
		return fmt.Errorf("tool %s already registered", tool.Name)
	}

	e.tools[tool.Name] = tool
	return nil
}

// UnregisterTool removes a tool from the registry
func (e *Executor) UnregisterTool(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.tools[name]; !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	delete(e.tools, name)
	return nil
}

// Execute runs a client-side tool with validation and enhanced error handling
func (e *Executor) Execute(ctx context.Context, toolName string, params interface{}) (*ExecutionResult, error) {
	e.mu.RLock()
	tool, exists := e.tools[toolName]
	e.mu.RUnlock()

	if !exists {
		execErr := &ExecutionError{
			Type:       ErrorTypeNotFound,
			Message:    fmt.Sprintf("tool '%s' not found", toolName),
			ToolName:   toolName,
			Suggestion: "Check available tools with 'tools list' command",
			Retryable:  false,
		}
		return e.buildErrorResult(toolName, execErr, 0), execErr
	}

	// Convert params to map if necessary
	paramMap, err := e.normalizeParams(params)
	if err != nil {
		execErr := &ExecutionError{
			Type:       ErrorTypeValidation,
			Message:    fmt.Sprintf("failed to parse parameters: %v", err),
			ToolName:   toolName,
			Suggestion: "Ensure parameters are valid JSON format",
			Retryable:  false,
		}
		return e.buildErrorResult(toolName, execErr, 0), execErr
	}

	// Validate parameters if schema is available
	if tool.ParamDef != nil {
		if err := ValidateToolArguments(toolName, tool.ParamDef, paramMap); err != nil {
			execErr := &ExecutionError{
				Type:       ErrorTypeValidation,
				Message:    err.Error(),
				ToolName:   toolName,
				Parameters: paramMap,
				Suggestion: SuggestCorrection(err),
				Retryable:  false,
			}
			return e.buildErrorResult(toolName, execErr, 0), execErr
		}
	}

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Record start time
	startTime := time.Now()

	// Execute the tool with enhanced error handling
	result, execErr := e.executeWithEnhancedRetry(execCtx, tool, paramMap)

	// Calculate duration
	duration := time.Since(startTime)

	// Update metrics
	e.updateMetrics(toolName, execErr == nil, duration)

	// Build execution result
	execResult := &ExecutionResult{
		ToolName:  toolName,
		Success:   execErr == nil,
		Duration:  duration,
		Timestamp: startTime,
		Metadata: map[string]interface{}{
			"parameters": paramMap,
			"retries":    e.maxRetry,
		},
	}

	if execErr != nil {
		execResult.Error = execErr.Error()
		execResult.ErrorType = execErr.Type
		if execErr.Suggestion != "" {
			if execResult.Metadata == nil {
				execResult.Metadata = make(map[string]interface{})
			}
			execResult.Metadata["suggestion"] = execErr.Suggestion
		}
	} else {
		execResult.Result = result
	}

	// Return nil error if successful to maintain backwards compatibility
	if execErr != nil {
		return execResult, execErr
	}
	return execResult, nil
}

// executeWithEnhancedRetry executes a tool with enhanced retry logic and error categorization
func (e *Executor) executeWithEnhancedRetry(ctx context.Context, tool *Tool, params map[string]interface{}) (interface{}, *ExecutionError) {
	var lastErr *ExecutionError
	attemptCount := 0
	
	for i := 0; i < e.maxRetry; i++ {
		attemptCount++
		
		select {
		case <-ctx.Done():
			// Context cancelled or timeout
			if ctx.Err() == context.DeadlineExceeded {
				return nil, &ExecutionError{
					Type:       ErrorTypeTimeout,
					Message:    fmt.Sprintf("tool execution timed out after %v", e.timeout),
					ToolName:   tool.Name,
					Parameters: params,
					Suggestion: "Consider increasing timeout or simplifying the operation",
					Retryable:  true,
				}
			}
			return nil, &ExecutionError{
				Type:       ErrorTypeExecution,
				Message:    "execution cancelled",
				ToolName:   tool.Name,
				Parameters: params,
				Retryable:  false,
			}
		default:
			result, err := tool.Execute(ctx, params)
			if err == nil {
				return result, nil
			}
			
			// Categorize the error
			execErr := e.categorizeError(err, tool.Name, params)
			lastErr = execErr
			
			// Don't retry if not retryable or context is cancelled
			if !execErr.Retryable || ctx.Err() != nil {
				return nil, execErr
			}
			
			// Exponential backoff for retryable errors
			if i < e.maxRetry-1 {
				backoff := time.Duration(1<<uint(i)) * 100 * time.Millisecond
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return nil, &ExecutionError{
						Type:       ErrorTypeTimeout,
						Message:    fmt.Sprintf("timed out during retry backoff"),
						ToolName:   tool.Name,
						Parameters: params,
						Retryable:  false,
					}
				}
			}
		}
	}
	
	// Add retry information to the final error
	if lastErr != nil {
		lastErr.Message = fmt.Sprintf("%s (failed after %d attempts)", lastErr.Message, attemptCount)
	}
	
	return nil, lastErr
}

// categorizeError analyzes an error and categorizes it
func (e *Executor) categorizeError(err error, toolName string, params map[string]interface{}) *ExecutionError {
	errMsg := err.Error()
	
	// Check for common error patterns
	switch {
	case strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "access denied"):
		return &ExecutionError{
			Type:       ErrorTypeExecution,
			Message:    err.Error(),
			ToolName:   toolName,
			Parameters: params,
			Suggestion: "Check file permissions and access rights",
			Retryable:  false,
			Wrapped:    err,
		}
	case strings.Contains(errMsg, "no such file") || strings.Contains(errMsg, "not found"):
		return &ExecutionError{
			Type:       ErrorTypeExecution,
			Message:    err.Error(),
			ToolName:   toolName,
			Parameters: params,
			Suggestion: "Verify the file or resource exists",
			Retryable:  false,
			Wrapped:    err,
		}
	case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "timed out"):
		return &ExecutionError{
			Type:       ErrorTypeTimeout,
			Message:    err.Error(),
			ToolName:   toolName,
			Parameters: params,
			Suggestion: "The operation took too long. Try with a smaller dataset or increase timeout",
			Retryable:  true,
			Wrapped:    err,
		}
	case strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "network"):
		return &ExecutionError{
			Type:       ErrorTypeRetryable,
			Message:    err.Error(),
			ToolName:   toolName,
			Parameters: params,
			Suggestion: "Network issue detected. Check connectivity",
			Retryable:  true,
			Wrapped:    err,
		}
	default:
		// Default to execution error
		return &ExecutionError{
			Type:       ErrorTypeExecution,
			Message:    err.Error(),
			ToolName:   toolName,
			Parameters: params,
			Suggestion: "Review the error message and adjust parameters accordingly",
			Retryable:  false,
			Wrapped:    err,
		}
	}
}

// buildErrorResult creates an ExecutionResult for an error
func (e *Executor) buildErrorResult(toolName string, err *ExecutionError, duration time.Duration) *ExecutionResult {
	return &ExecutionResult{
		ToolName:  toolName,
		Success:   false,
		Error:     err.Error(),
		ErrorType: err.Type,
		Duration:  duration,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"error_type": err.Type,
			"retryable":  err.Retryable,
			"suggestion": err.Suggestion,
		},
	}
}

// normalizeParams converts various param types to map[string]interface{}
func (e *Executor) normalizeParams(params interface{}) (map[string]interface{}, error) {
	if params == nil {
		return make(map[string]interface{}), nil
	}

	switch p := params.(type) {
	case map[string]interface{}:
		return p, nil
	case string:
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(p), &result); err != nil {
			return nil, err
		}
		return result, nil
	default:
		// Try JSON marshaling/unmarshaling to convert
		data, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
}

// updateMetrics updates execution metrics for a tool
func (e *Executor) updateMetrics(toolName string, success bool, duration time.Duration) {
	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()

	metrics, exists := e.metrics.executions[toolName]
	if !exists {
		metrics = &ToolMetrics{}
		e.metrics.executions[toolName] = metrics
	}

	metrics.TotalCalls++
	if success {
		metrics.SuccessfulCalls++
	} else {
		metrics.FailedCalls++
	}
	metrics.TotalDuration += duration
	metrics.LastExecution = time.Now()
}

// GetTool returns a tool by name
func (e *Executor) GetTool(name string) (*Tool, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	tool, exists := e.tools[name]
	return tool, exists
}

// ListTools returns all registered tools
func (e *Executor) ListTools() []*Tool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tools := make([]*Tool, 0, len(e.tools))
	for _, tool := range e.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetMetrics returns execution metrics for a tool
func (e *Executor) GetMetrics(toolName string) (*ToolMetrics, bool) {
	e.metrics.mu.RLock()
	defer e.metrics.mu.RUnlock()
	metrics, exists := e.metrics.executions[toolName]
	return metrics, exists
}

// SetTimeout sets the execution timeout
func (e *Executor) SetTimeout(timeout time.Duration) {
	e.timeout = timeout
}

// SetMaxRetry sets the maximum number of retries
func (e *Executor) SetMaxRetry(maxRetry int) {
	e.maxRetry = maxRetry
}