package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	
	"github.com/sirupsen/logrus"
)

// ErrorHandler provides centralized error handling for the CLI
type ErrorHandler struct {
	logger     *logrus.Logger
	output     io.Writer
	jsonOutput bool
	verbose    bool
	noColor    bool
	
	// Error handling configuration
	maxRetries     int
	retryDelay     time.Duration
	retryMultiplier float64
	
	// Statistics
	errorCount     int
	warningCount   int
	retryCount     int
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *logrus.Logger, output io.Writer) *ErrorHandler {
	return &ErrorHandler{
		logger:          logger,
		output:          output,
		maxRetries:      3,
		retryDelay:      time.Second,
		retryMultiplier: 2.0,
	}
}

// SetJSONOutput enables JSON output mode
func (h *ErrorHandler) SetJSONOutput(enabled bool) {
	h.jsonOutput = enabled
}

// SetVerbose enables verbose error output
func (h *ErrorHandler) SetVerbose(enabled bool) {
	h.verbose = enabled
}

// SetNoColor disables colored output
func (h *ErrorHandler) SetNoColor(enabled bool) {
	h.noColor = enabled
}

// SetMaxRetries sets the maximum number of retries
func (h *ErrorHandler) SetMaxRetries(max int) {
	h.maxRetries = max
}

// HandleError processes an error and displays it appropriately
func (h *ErrorHandler) HandleError(err error, context string) {
	if err == nil {
		return
	}
	
	h.errorCount++
	
	// Convert to ToolError if not already
	toolErr := h.categorizeError(err, context)
	
	// Log the error
	h.logError(toolErr)
	
	// Display the error
	if h.jsonOutput {
		h.displayErrorJSON(toolErr)
	} else {
		h.displayErrorPretty(toolErr)
	}
}

// HandleWarning processes a warning message
func (h *ErrorHandler) HandleWarning(message string, details string) {
	h.warningCount++
	
	if h.jsonOutput {
		warning := map[string]interface{}{
			"type":    "warning",
			"message": message,
		}
		if details != "" {
			warning["details"] = details
		}
		data, _ := json.Marshal(warning)
		fmt.Fprintln(h.output, string(data))
	} else {
		if h.noColor {
			fmt.Fprintf(h.output, "⚠️  Warning: %s\n", message)
		} else {
			fmt.Fprintf(h.output, "\033[33m⚠️  Warning: %s\033[0m\n", message)
		}
		if details != "" && h.verbose {
			fmt.Fprintf(h.output, "   %s\n", details)
		}
	}
}

// categorizeError converts a generic error to a ToolError with appropriate category
func (h *ErrorHandler) categorizeError(err error, context string) *ToolError {
	// Check if already a ToolError
	if toolErr, ok := err.(*ToolError); ok {
		return toolErr
	}
	
	errMsg := err.Error()
	errLower := strings.ToLower(errMsg)
	
	// Categorize based on error message patterns
	switch {
	case strings.Contains(errLower, "connection") || 
	     strings.Contains(errLower, "network") ||
	     strings.Contains(errLower, "dial"):
		return NewNetworkError(errMsg, true)
		
	case strings.Contains(errLower, "timeout"):
		return NewTimeoutError(context, "unknown")
		
	case strings.Contains(errLower, "validation") ||
	     strings.Contains(errLower, "invalid") ||
	     strings.Contains(errLower, "schema"):
		return NewValidationError("", errMsg)
		
	case strings.Contains(errLower, "permission") ||
	     strings.Contains(errLower, "unauthorized") ||
	     strings.Contains(errLower, "forbidden"):
		return NewPermissionError(errMsg)
		
	case strings.Contains(errLower, "server") ||
	     strings.Contains(errLower, "500") ||
	     strings.Contains(errLower, "502") ||
	     strings.Contains(errLower, "503"):
		return NewServerError(500, errMsg)
		
	default:
		return &ToolError{
			Category:  CategoryUnknown,
			Severity:  SeverityError,
			Message:   errMsg,
			Operation: context,
			Retryable: false,
			Original:  err,
		}
	}
}

// logError logs the error with appropriate level
func (h *ErrorHandler) logError(err *ToolError) {
	fields := logrus.Fields{
		"category": err.Category,
		"severity": err.Severity,
	}
	
	if err.ToolName != "" {
		fields["tool"] = err.ToolName
	}
	if err.Operation != "" {
		fields["operation"] = err.Operation
	}
	if err.Code != "" {
		fields["code"] = err.Code
	}
	
	entry := h.logger.WithFields(fields)
	
	switch err.Severity {
	case SeverityCritical:
		entry.Error(err.Message)
	case SeverityError:
		entry.Error(err.Message)
	case SeverityWarning:
		entry.Warn(err.Message)
	case SeverityInfo:
		entry.Info(err.Message)
	}
}

// displayErrorPretty displays the error in a user-friendly format
func (h *ErrorHandler) displayErrorPretty(err *ToolError) {
	emoji := GetErrorEmoji(err.Category)
	
	// Build error message
	var sb strings.Builder
	
	// Main error line
	if h.noColor {
		sb.WriteString(fmt.Sprintf("\n%s Error", emoji))
	} else {
		color := GetSeverityColor(err.Severity)
		sb.WriteString(fmt.Sprintf("\n%s%s Error\033[0m", color, emoji))
	}
	
	if err.ToolName != "" {
		sb.WriteString(fmt.Sprintf(" in tool '%s'", err.ToolName))
	}
	
	sb.WriteString(fmt.Sprintf(": %s\n", err.Message))
	
	// Details if verbose
	if h.verbose && err.Details != "" {
		sb.WriteString(fmt.Sprintf("   Details: %s\n", err.Details))
	}
	
	// Suggestion
	if err.Suggestion != "" {
		if h.noColor {
			sb.WriteString(fmt.Sprintf("   💡 %s\n", err.Suggestion))
		} else {
			sb.WriteString(fmt.Sprintf("   \033[36m💡 %s\033[0m\n", err.Suggestion))
		}
	}
	
	// Retry information
	if err.Retryable {
		if h.noColor {
			sb.WriteString("   🔄 This error may be retried\n")
		} else {
			sb.WriteString("   \033[33m🔄 This error may be retried\033[0m\n")
		}
	}
	
	fmt.Fprint(h.output, sb.String())
}

// displayErrorJSON displays the error in JSON format
func (h *ErrorHandler) displayErrorJSON(err *ToolError) {
	errorData := map[string]interface{}{
		"type":      "error",
		"category":  err.Category,
		"severity":  err.Severity,
		"message":   err.Message,
		"retryable": err.Retryable,
	}
	
	if err.ToolName != "" {
		errorData["tool_name"] = err.ToolName
	}
	if err.Operation != "" {
		errorData["operation"] = err.Operation
	}
	if err.Code != "" {
		errorData["code"] = err.Code
	}
	if err.Details != "" {
		errorData["details"] = err.Details
	}
	if err.Suggestion != "" {
		errorData["suggestion"] = err.Suggestion
	}
	
	data, _ := json.Marshal(errorData)
	fmt.Fprintln(h.output, string(data))
}

// ExecuteWithRetry executes a function with retry logic
func (h *ErrorHandler) ExecuteWithRetry(ctx context.Context, operation string, fn func() error) error {
	var lastErr error
	delay := h.retryDelay
	
	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		// Check context
		if err := ctx.Err(); err != nil {
			return NewToolError(CategoryTimeout, SeverityCritical, "Operation cancelled")
		}
		
		// Execute the function
		err := fn()
		if err == nil {
			return nil // Success
		}
		
		lastErr = err
		
		// Check if error is retryable
		toolErr := h.categorizeError(err, operation)
		if !toolErr.IsRetryable() || attempt == h.maxRetries {
			return toolErr
		}
		
		h.retryCount++
		
		// Display retry message
		if !h.jsonOutput {
			if h.noColor {
				fmt.Fprintf(h.output, "   🔄 Retrying %s (attempt %d/%d) in %v...\n", 
					operation, attempt+1, h.maxRetries, delay)
			} else {
				fmt.Fprintf(h.output, "   \033[33m🔄 Retrying %s (attempt %d/%d) in %v...\033[0m\n", 
					operation, attempt+1, h.maxRetries, delay)
			}
		}
		
		// Wait before retry
		select {
		case <-time.After(delay):
			delay = time.Duration(float64(delay) * h.retryMultiplier)
		case <-ctx.Done():
			return NewToolError(CategoryTimeout, SeverityCritical, "Operation cancelled during retry")
		}
	}
	
	return h.categorizeError(lastErr, operation)
}

// HandleHTTPError processes HTTP response errors
func (h *ErrorHandler) HandleHTTPError(resp *http.Response, operation string) *ToolError {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	
	// Try to read error body
	body, _ := io.ReadAll(resp.Body)
	message := string(body)
	
	// Try to parse as JSON error
	var jsonError map[string]interface{}
	if err := json.Unmarshal(body, &jsonError); err == nil {
		if msg, ok := jsonError["message"].(string); ok {
			message = msg
		} else if errMsg, ok := jsonError["error"].(string); ok {
			message = errMsg
		}
	}
	
	if message == "" {
		message = resp.Status
	}
	
	return NewServerError(resp.StatusCode, message).WithDetails(operation)
}

// GetStatistics returns error handling statistics
func (h *ErrorHandler) GetStatistics() map[string]int {
	return map[string]int{
		"errors":   h.errorCount,
		"warnings": h.warningCount,
		"retries":  h.retryCount,
	}
}

// DisplaySummary displays a summary of errors and warnings
func (h *ErrorHandler) DisplaySummary() {
	if h.errorCount == 0 && h.warningCount == 0 {
		return
	}
	
	if h.jsonOutput {
		summary := map[string]interface{}{
			"type":       "summary",
			"errors":     h.errorCount,
			"warnings":   h.warningCount,
			"retries":    h.retryCount,
		}
		data, _ := json.Marshal(summary)
		fmt.Fprintln(h.output, string(data))
	} else {
		fmt.Fprintln(h.output, "\n─────────────────────────────")
		if h.errorCount > 0 {
			if h.noColor {
				fmt.Fprintf(h.output, "❌ %d error(s) occurred\n", h.errorCount)
			} else {
				fmt.Fprintf(h.output, "\033[31m❌ %d error(s) occurred\033[0m\n", h.errorCount)
			}
		}
		if h.warningCount > 0 {
			if h.noColor {
				fmt.Fprintf(h.output, "⚠️  %d warning(s) encountered\n", h.warningCount)
			} else {
				fmt.Fprintf(h.output, "\033[33m⚠️  %d warning(s) encountered\033[0m\n", h.warningCount)
			}
		}
		if h.retryCount > 0 {
			fmt.Fprintf(h.output, "🔄 %d retry attempt(s) made\n", h.retryCount)
		}
	}
}

// Reset resets the error handler statistics
func (h *ErrorHandler) Reset() {
	h.errorCount = 0
	h.warningCount = 0
	h.retryCount = 0
}