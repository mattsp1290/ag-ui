package errors

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// PanicRecoveryOptions configures panic recovery behavior
type PanicRecoveryOptions struct {
	// RecoveryHandler is called when a panic is recovered
	RecoveryHandler func(panicValue interface{}, stackTrace []byte, context *EnhancedErrorContext)
	
	// ShouldRecover determines if a panic should be recovered based on its value
	ShouldRecover func(panicValue interface{}) bool
	
	// IncludeStackTrace includes full stack trace in error details
	IncludeStackTrace bool
	
	// MaxStackDepth limits the stack trace depth
	MaxStackDepth int
	
	// Component identifies the component for error reporting
	Component string
	
	// NodeID identifies the node for distributed error tracking
	NodeID string
}

// DefaultPanicRecoveryOptions returns default panic recovery options
func DefaultPanicRecoveryOptions() *PanicRecoveryOptions {
	return &PanicRecoveryOptions{
		RecoveryHandler:   defaultRecoveryHandler,
		ShouldRecover:     defaultShouldRecover,
		IncludeStackTrace: true,
		MaxStackDepth:     32,
		Component:         "unknown",
	}
}

// WithRecovery wraps a function with panic recovery and enhanced error context
func WithRecovery(ctx context.Context, operationName string, options *PanicRecoveryOptions, fn func() error) error {
	if options == nil {
		options = DefaultPanicRecoveryOptions()
	}
	
	// Get or create enhanced error context
	eec, _ := GetOrCreateEnhancedContext(ctx, operationName)
	eec.WithComponent(options.Component).WithNodeID(options.NodeID)
	
	var panicErr error
	
	// Capture performance metrics
	startTime := time.Now()
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	startMem := memStats.Alloc
	
	defer func() {
		if r := recover(); r != nil {
			// Check if we should recover this panic
			if options.ShouldRecover != nil && !options.ShouldRecover(r) {
				// Re-panic if we shouldn't recover
				panic(r)
			}
			
			// Capture final memory usage
			runtime.ReadMemStats(&memStats)
			endMem := memStats.Alloc
			
			// Create performance metrics
			perfMetrics := &PerformanceMetrics{
				StartTime:   startTime,
				Duration:    time.Since(startTime),
				MemoryUsage: endMem - startMem,
			}
			
			eec.WithPerformanceMetrics(perfMetrics)
			
			// Capture stack trace
			stackTrace := debug.Stack()
			
			// Create enhanced panic error
			panicErr = eec.createPanicError(r, stackTrace, options)
			
			// Call recovery handler if provided
			if options.RecoveryHandler != nil {
				options.RecoveryHandler(r, stackTrace, eec)
			}
		}
	}()
	
	// Execute the function with enhanced context
	err := fn()
	
	// If there was a panic, return the panic error
	if panicErr != nil {
		return panicErr
	}
	
	// If the function returned an error, enhance it with context
	if err != nil {
		return eec.RecordEnhancedError(err, "OPERATION_ERROR")
	}
	
	return nil
}

// WithRecoveryResult wraps a function that returns a result with panic recovery
func WithRecoveryResult(ctx context.Context, operationName string, options *PanicRecoveryOptions, fn func() (interface{}, error)) (interface{}, error) {
	var result interface{}
	var fnErr error
	
	err := WithRecovery(ctx, operationName, options, func() error {
		result, fnErr = fn()
		return fnErr
	})
	
	if err != nil {
		return nil, err
	}
	
	return result, nil
}

// SafeGoroutine starts a goroutine with panic recovery
func SafeGoroutine(ctx context.Context, operationName string, options *PanicRecoveryOptions, fn func()) {
	go func() {
		WithRecovery(ctx, operationName, options, func() error {
			fn()
			return nil
		})
	}()
}

// SafeGoroutineWithChannel starts a goroutine with panic recovery and error reporting via channel
func SafeGoroutineWithChannel(ctx context.Context, operationName string, options *PanicRecoveryOptions, errorChan chan<- error, fn func() error) {
	go func() {
		defer close(errorChan)
		
		err := WithRecovery(ctx, operationName, options, fn)
		if err != nil {
			select {
			case errorChan <- err:
			case <-ctx.Done():
			}
		}
	}()
}

// createPanicError creates an enhanced error from a panic
func (eec *EnhancedErrorContext) createPanicError(panicValue interface{}, stackTrace []byte, options *PanicRecoveryOptions) error {
	// Create the base panic error
	panicErr := &BaseError{
		Code:      "PANIC_RECOVERED",
		Message:   fmt.Sprintf("Panic recovered: %v", panicValue),
		Severity:  SeverityCritical,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}
	
	// Add panic details
	panicErr.Details["panic_value"] = panicValue
	panicErr.Details["panic_type"] = fmt.Sprintf("%T", panicValue)
	
	// Add stack trace if enabled
	if options.IncludeStackTrace {
		panicErr.Details["stack_trace"] = string(stackTrace)
		panicErr.Details["stack_trace_parsed"] = parseStackTrace(stackTrace, options.MaxStackDepth)
	}
	
	// Add enhanced context information
	panicErr.Details["correlation_id"] = string(eec.CorrelationID)
	panicErr.Details["context_id"] = eec.ErrorContext.ID
	panicErr.Details["operation"] = eec.ErrorContext.OperationName
	
	if eec.OperationMetadata != nil {
		panicErr.Details["operation_type"] = eec.OperationMetadata.OperationType
		panicErr.Details["node_id"] = eec.OperationMetadata.NodeID
		panicErr.Details["component"] = eec.OperationMetadata.Component
	}
	
	// Add recovery context
	panicErr.Details["recovery_time"] = time.Now()
	panicErr.Details["goroutine_id"] = getGoroutineID()
	
	// Add actionable guidance specific to panics
	eec.AddActionableGuidance("Check for nil pointer dereferences or array bounds violations")
	eec.AddActionableGuidance("Review recent code changes that might cause runtime panics")
	eec.AddActionableGuidance("Consider adding additional input validation")
	eec.AddActionableGuidance("Monitor memory usage and resource constraints")
	
	// Record the error in the context
	eec.ErrorContext.RecordError(panicErr)
	
	return panicErr
}

// StackFrame represents a single frame in a stack trace
type StackFrame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// parseStackTrace parses a stack trace into structured frames
func parseStackTrace(stackTrace []byte, maxDepth int) []StackFrame {
	// This is a simplified stack trace parser
	// In a production implementation, you might want to use a more sophisticated parser
	frames := make([]StackFrame, 0, maxDepth)
	
	// Parse the stack trace (this is a basic implementation)
	// The actual implementation would need to parse the specific format of Go stack traces
	
	return frames
}

// getGoroutineID returns the current goroutine ID
func getGoroutineID() int {
	// This is a simplified implementation
	// In a production system, you might want to use a more robust method
	return runtime.NumGoroutine()
}

// defaultRecoveryHandler is the default panic recovery handler
func defaultRecoveryHandler(panicValue interface{}, stackTrace []byte, context *EnhancedErrorContext) {
	// Log the panic (in a real implementation, this would use a proper logger)
	fmt.Printf("PANIC RECOVERED: %v\n", panicValue)
	fmt.Printf("Context: %s\n", context.ErrorContext.Summary())
	if len(stackTrace) > 0 {
		fmt.Printf("Stack Trace:\n%s\n", string(stackTrace))
	}
}

// defaultShouldRecover determines if a panic should be recovered
func defaultShouldRecover(panicValue interface{}) bool {
	// By default, recover from all panics except for specific types that should propagate
	switch panicValue.(type) {
	case runtime.Error:
		// Recover from runtime errors like nil pointer dereferences
		return true
	default:
		// Recover from other panics as well
		return true
	}
}

// PanicRecoveryMiddleware creates middleware for panic recovery in distributed systems
type PanicRecoveryMiddleware struct {
	options *PanicRecoveryOptions
}

// NewPanicRecoveryMiddleware creates a new panic recovery middleware
func NewPanicRecoveryMiddleware(options *PanicRecoveryOptions) *PanicRecoveryMiddleware {
	if options == nil {
		options = DefaultPanicRecoveryOptions()
	}
	
	return &PanicRecoveryMiddleware{
		options: options,
	}
}

// WrapHandler wraps a handler function with panic recovery
func (prm *PanicRecoveryMiddleware) WrapHandler(operationName string, handler func(ctx context.Context) error) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return WithRecovery(ctx, operationName, prm.options, func() error {
			return handler(ctx)
		})
	}
}

// WrapHandlerWithResult wraps a handler function that returns a result with panic recovery
func (prm *PanicRecoveryMiddleware) WrapHandlerWithResult(operationName string, handler func(ctx context.Context) (interface{}, error)) func(ctx context.Context) (interface{}, error) {
	return func(ctx context.Context) (interface{}, error) {
		return WithRecoveryResult(ctx, operationName, prm.options, func() (interface{}, error) {
			return handler(ctx)
		})
	}
}

// CircuitBreakerWithRecovery combines circuit breaker protection with panic recovery
func CircuitBreakerWithRecovery(ctx context.Context, cb CircuitBreaker, operationName string, options *PanicRecoveryOptions, fn func() error) error {
	return cb.Execute(ctx, func() error {
		return WithRecovery(ctx, operationName, options, fn)
	})
}

// CircuitBreakerWithRecoveryResult combines circuit breaker protection with panic recovery for functions returning results
func CircuitBreakerWithRecoveryResult(ctx context.Context, cb CircuitBreaker, operationName string, options *PanicRecoveryOptions, fn func() (interface{}, error)) (interface{}, error) {
	return cb.Call(ctx, func() (interface{}, error) {
		return WithRecoveryResult(ctx, operationName, options, fn)
	})
}

// RecoveryStats tracks statistics about panic recovery
type RecoveryStats struct {
	TotalPanics      int64     `json:"total_panics"`
	RecoveredPanics  int64     `json:"recovered_panics"`
	UnrecoveredPanics int64    `json:"unrecovered_panics"`
	LastPanicTime    time.Time `json:"last_panic_time"`
	PanicsByType     map[string]int64 `json:"panics_by_type"`
}

// RecoveryStatsCollector collects statistics about panic recovery
type RecoveryStatsCollector struct {
	stats *RecoveryStats
	mu    sync.RWMutex
}

// NewRecoveryStatsCollector creates a new recovery stats collector
func NewRecoveryStatsCollector() *RecoveryStatsCollector {
	return &RecoveryStatsCollector{
		stats: &RecoveryStats{
			PanicsByType: make(map[string]int64),
		},
	}
}

// RecordPanic records a panic in the statistics
func (rsc *RecoveryStatsCollector) RecordPanic(panicValue interface{}, recovered bool) {
	rsc.mu.Lock()
	defer rsc.mu.Unlock()
	
	rsc.stats.TotalPanics++
	rsc.stats.LastPanicTime = time.Now()
	
	if recovered {
		rsc.stats.RecoveredPanics++
	} else {
		rsc.stats.UnrecoveredPanics++
	}
	
	panicType := fmt.Sprintf("%T", panicValue)
	rsc.stats.PanicsByType[panicType]++
}

// GetStats returns the current recovery statistics
func (rsc *RecoveryStatsCollector) GetStats() RecoveryStats {
	rsc.mu.RLock()
	defer rsc.mu.RUnlock()
	
	// Create a copy to avoid data races
	statsCopy := *rsc.stats
	statsCopy.PanicsByType = make(map[string]int64)
	for k, v := range rsc.stats.PanicsByType {
		statsCopy.PanicsByType[k] = v
	}
	
	return statsCopy
}

// Global recovery stats collector
var globalRecoveryStatsCollector = NewRecoveryStatsCollector()

// GetGlobalRecoveryStats returns the global recovery statistics
func GetGlobalRecoveryStats() RecoveryStats {
	return globalRecoveryStatsCollector.GetStats()
}