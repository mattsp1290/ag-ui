package errors

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// RecoveryStrategy defines how to handle errors
type RecoveryStrategy string

const (
	// StrategyRetry attempts to retry the operation
	StrategyRetry RecoveryStrategy = "retry"
	
	// StrategySkip skips the current operation and continues
	StrategySkip RecoveryStrategy = "skip"
	
	// StrategyAbort stops all operations
	StrategyAbort RecoveryStrategy = "abort"
	
	// StrategyPrompt asks the user what to do
	StrategyPrompt RecoveryStrategy = "prompt"
	
	// StrategyFallback tries an alternative approach
	StrategyFallback RecoveryStrategy = "fallback"
)

// RecoveryPolicy defines the error recovery policy
type RecoveryPolicy struct {
	// Default strategy for all errors
	DefaultStrategy RecoveryStrategy
	
	// Category-specific strategies
	CategoryStrategies map[ErrorCategory]RecoveryStrategy
	
	// Interactive mode
	Interactive bool
	
	// Maximum retries before falling back to next strategy
	MaxRetries int
	
	// Retry delay configuration
	RetryDelay      time.Duration
	RetryMultiplier float64
	
	// Fallback function for custom recovery
	FallbackFunc func(error) error
}

// NewRecoveryPolicy creates a default recovery policy
func NewRecoveryPolicy() *RecoveryPolicy {
	return &RecoveryPolicy{
		DefaultStrategy: StrategyPrompt,
		CategoryStrategies: map[ErrorCategory]RecoveryStrategy{
			CategoryNetwork:    StrategyRetry,
			CategoryTimeout:    StrategyRetry,
			CategoryServer:     StrategyRetry,
			CategoryValidation: StrategyAbort,
			CategoryPermission: StrategyAbort,
			CategoryTool:       StrategyPrompt,
		},
		Interactive:     true,
		MaxRetries:      3,
		RetryDelay:      time.Second,
		RetryMultiplier: 2.0,
	}
}

// GetStrategy returns the recovery strategy for an error
func (p *RecoveryPolicy) GetStrategy(err *ToolError) RecoveryStrategy {
	// Check if interactive mode is disabled
	if !p.Interactive && p.DefaultStrategy == StrategyPrompt {
		// Fall back to abort if prompting is not possible
		return StrategyAbort
	}
	
	// Check for category-specific strategy
	if strategy, ok := p.CategoryStrategies[err.Category]; ok {
		return strategy
	}
	
	return p.DefaultStrategy
}

// RecoveryManager handles error recovery operations
type RecoveryManager struct {
	policy       *RecoveryPolicy
	errorHandler *ErrorHandler
	retryCount   map[string]int
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(policy *RecoveryPolicy, errorHandler *ErrorHandler) *RecoveryManager {
	return &RecoveryManager{
		policy:       policy,
		errorHandler: errorHandler,
		retryCount:   make(map[string]int),
	}
}

// HandleError processes an error and determines recovery action
func (m *RecoveryManager) HandleError(ctx context.Context, err error, operation string) (RecoveryStrategy, error) {
	// Convert to ToolError
	toolErr, ok := err.(*ToolError)
	if !ok {
		toolErr = m.errorHandler.categorizeError(err, operation)
	}
	
	// Display the error
	m.errorHandler.HandleError(toolErr, operation)
	
	// Get recovery strategy
	strategy := m.policy.GetStrategy(toolErr)
	
	// Handle based on strategy
	switch strategy {
	case StrategyRetry:
		if m.canRetry(operation) {
			return StrategyRetry, nil
		}
		// Fall back to next strategy if max retries exceeded
		if m.policy.Interactive {
			return m.promptUser(toolErr)
		}
		return StrategyAbort, toolErr
		
	case StrategyPrompt:
		return m.promptUser(toolErr)
		
	case StrategyFallback:
		if m.policy.FallbackFunc != nil {
			if err := m.policy.FallbackFunc(toolErr); err == nil {
				return StrategyFallback, nil
			}
		}
		return StrategyAbort, toolErr
		
	case StrategySkip:
		m.errorHandler.HandleWarning("Skipping operation due to error", operation)
		return StrategySkip, nil
		
	default:
		return StrategyAbort, toolErr
	}
}

// canRetry checks if the operation can be retried
func (m *RecoveryManager) canRetry(operation string) bool {
	count := m.retryCount[operation]
	if count >= m.policy.MaxRetries {
		return false
	}
	m.retryCount[operation] = count + 1
	return true
}

// promptUser asks the user for recovery action
func (m *RecoveryManager) promptUser(err *ToolError) (RecoveryStrategy, error) {
	fmt.Println("\n─────────────────────────────")
	fmt.Println("An error occurred. What would you like to do?")
	fmt.Println()
	
	options := []string{}
	
	if err.IsRetryable() {
		options = append(options, "[R]etry the operation")
	}
	options = append(options, "[S]kip this operation and continue")
	options = append(options, "[A]bort all operations")
	
	if m.policy.FallbackFunc != nil {
		options = append(options, "[F]allback to alternative method")
	}
	
	for _, opt := range options {
		fmt.Printf("  %s\n", opt)
	}
	
	fmt.Print("\nYour choice: ")
	
	reader := bufio.NewReader(os.Stdin)
	input, readErr := reader.ReadString('\n')
	if readErr != nil {
		return StrategyAbort, readErr
	}
	
	choice := strings.ToLower(strings.TrimSpace(input))
	
	switch choice {
	case "r", "retry":
		if err.IsRetryable() {
			return StrategyRetry, nil
		}
		fmt.Println("This error cannot be retried.")
		return m.promptUser(err)
		
	case "s", "skip":
		return StrategySkip, nil
		
	case "a", "abort":
		return StrategyAbort, err
		
	case "f", "fallback":
		if m.policy.FallbackFunc != nil {
			return StrategyFallback, nil
		}
		fmt.Println("No fallback method available.")
		return m.promptUser(err)
		
	default:
		fmt.Println("Invalid choice. Please try again.")
		return m.promptUser(err)
	}
}

// ExecuteWithRecovery executes an operation with recovery handling
func (m *RecoveryManager) ExecuteWithRecovery(ctx context.Context, operation string, fn func() error) error {
	delay := m.policy.RetryDelay
	
	for {
		// Check context
		if err := ctx.Err(); err != nil {
			return NewToolError(CategoryTimeout, SeverityCritical, "Operation cancelled")
		}
		
		// Execute the function
		err := fn()
		if err == nil {
			// Success - reset retry count
			delete(m.retryCount, operation)
			return nil
		}
		
		// Handle the error and get recovery strategy
		strategy, recoveryErr := m.HandleError(ctx, err, operation)
		
		switch strategy {
		case StrategyRetry:
			// Wait before retry with exponential backoff
			fmt.Printf("Retrying in %v...\n", delay)
			select {
			case <-time.After(delay):
				delay = time.Duration(float64(delay) * m.policy.RetryMultiplier)
			case <-ctx.Done():
				return NewToolError(CategoryTimeout, SeverityCritical, "Operation cancelled during retry")
			}
			continue
			
		case StrategySkip:
			// Skip this operation
			return nil
			
		case StrategyFallback:
			// Try fallback
			if m.policy.FallbackFunc != nil {
				return m.policy.FallbackFunc(err)
			}
			return recoveryErr
			
		case StrategyAbort:
			// Abort all operations
			return recoveryErr
			
		default:
			return recoveryErr
		}
	}
}

// BatchExecutor executes multiple operations with error recovery
type BatchExecutor struct {
	manager      *RecoveryManager
	stopOnError  bool
	results      []BatchResult
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Operation string
	Success   bool
	Error     error
	Skipped   bool
	Duration  time.Duration
}

// NewBatchExecutor creates a new batch executor
func NewBatchExecutor(manager *RecoveryManager, stopOnError bool) *BatchExecutor {
	return &BatchExecutor{
		manager:     manager,
		stopOnError: stopOnError,
		results:     []BatchResult{},
	}
}

// Execute runs a batch of operations
func (b *BatchExecutor) Execute(ctx context.Context, operations []BatchOperation) []BatchResult {
	b.results = make([]BatchResult, 0, len(operations))
	
	for _, op := range operations {
		start := time.Now()
		
		// Check if we should continue
		if b.stopOnError && b.hasErrors() {
			b.results = append(b.results, BatchResult{
				Operation: op.Name,
				Skipped:   true,
				Duration:  0,
			})
			continue
		}
		
		// Execute with recovery
		err := b.manager.ExecuteWithRecovery(ctx, op.Name, op.Func)
		
		result := BatchResult{
			Operation: op.Name,
			Success:   err == nil,
			Error:     err,
			Duration:  time.Since(start),
		}
		
		b.results = append(b.results, result)
		
		// Check if we should stop
		if b.stopOnError && err != nil {
			// Mark remaining operations as skipped
			for i := len(b.results); i < len(operations); i++ {
				b.results = append(b.results, BatchResult{
					Operation: operations[i].Name,
					Skipped:   true,
				})
			}
			break
		}
	}
	
	return b.results
}

// hasErrors checks if any operation has failed
func (b *BatchExecutor) hasErrors() bool {
	for _, result := range b.results {
		if !result.Success && !result.Skipped {
			return true
		}
	}
	return false
}

// GetResults returns the batch execution results
func (b *BatchExecutor) GetResults() []BatchResult {
	return b.results
}

// GetSummary returns a summary of batch execution
func (b *BatchExecutor) GetSummary() map[string]int {
	successful := 0
	failed := 0
	skipped := 0
	
	for _, result := range b.results {
		if result.Skipped {
			skipped++
		} else if result.Success {
			successful++
		} else {
			failed++
		}
	}
	
	return map[string]int{
		"successful": successful,
		"failed":     failed,
		"skipped":    skipped,
		"total":      len(b.results),
	}
}

// BatchOperation represents a single operation in a batch
type BatchOperation struct {
	Name string
	Func func() error
}