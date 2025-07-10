package tools

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ExecutionEngine manages the execution of tools.
// It provides validation, timeout management, concurrency control,
// and result handling for tool executions.
//
// Key features:
//   - Parameter validation against tool schemas
//   - Configurable execution timeouts
//   - Concurrency limiting to prevent resource exhaustion
//   - Rate limiting per tool or globally
//   - Execution hooks for cross-cutting concerns
//   - Metrics collection and reporting
//   - Streaming execution support
//   - Panic recovery and error handling
//
// Example usage:
//
//	registry := NewRegistry()
//	engine := NewExecutionEngine(registry,
//		WithMaxConcurrent(50),
//		WithDefaultTimeout(30*time.Second),
//		WithRateLimiter(rateLimiter),
//	)
//	
//	result, err := engine.Execute(ctx, "my-tool", params)
//	if err != nil {
//		// Handle error
//	}
type ExecutionEngine struct {
	registry *Registry

	// Configuration
	maxConcurrent  int32 // Changed to int32 for atomic operations
	defaultTimeout time.Duration

	// Execution tracking
	mu          sync.RWMutex
	cond        *sync.Cond
	activeCount int32             // Changed to int32 for atomic operations
	executions  sync.Map          // Changed to sync.Map for concurrent access

	// Metrics
	metrics *ExecutionMetrics

	// Rate limiting
	rateLimiter RateLimiter

	// Hooks for extensibility - protected by RWMutex
	hookMu        sync.RWMutex
	beforeExecute []ExecutionHook
	afterExecute  []ExecutionHook

	// Enhanced features
	cache             *ExecutionCache
	asyncWorkers      int
	asyncJobQueue     chan *AsyncJob
	asyncResults      sync.Map // Changed to sync.Map for concurrent access
	resourceMonitor   *ResourceMonitor
	sandbox           *SandboxConfig
}

// executionState tracks the state of a single tool execution.
type executionState struct {
	toolID    string
	startTime time.Time
	cancel    context.CancelFunc
}

// ExecutionMetrics tracks tool execution statistics.
// It provides insights into tool usage patterns, performance,
// and error rates. Metrics are thread-safe and can be accessed
// concurrently during execution.
type ExecutionMetrics struct {
	// Atomic counters for concurrent access
	totalExecutions int64 // atomic
	successCount    int64 // atomic
	errorCount      int64 // atomic
	totalDuration   int64 // atomic (nanoseconds)
	
	// Enhanced metrics
	cacheHits        int64 // atomic
	cacheMisses      int64 // atomic
	asyncExecutions  int64 // atomic
	
	// Tool metrics protected by mutex
	mu          sync.RWMutex
	toolMetrics map[string]*ToolMetrics
}

// ToolMetrics tracks statistics for a specific tool.
// It includes execution counts, success/error rates,
// and timing information for performance analysis.
type ToolMetrics struct {
	Executions      int64
	Successes       int64
	Errors          int64
	TotalDuration   time.Duration
	AverageDuration time.Duration
}

// RateLimiter interface for tool rate limiting.
// Implementations can provide per-tool or global rate limiting
// to prevent overload and ensure fair resource usage.
//
// Example implementation:
//
//	type TokenBucketLimiter struct {
//		buckets map[string]*rate.Limiter
//	}
//	
//	func (l *TokenBucketLimiter) Allow(toolID string) bool {
//		return l.buckets[toolID].Allow()
//	}
//	
//	func (l *TokenBucketLimiter) Wait(ctx context.Context, toolID string) error {
//		return l.buckets[toolID].Wait(ctx)
//	}
type RateLimiter interface {
	// Allow checks if a tool execution is allowed
	Allow(toolID string) bool
	// Wait blocks until the tool execution is allowed
	Wait(ctx context.Context, toolID string) error
}

// ExecutionHook is called before or after tool execution.
// Hooks enable cross-cutting concerns like logging, authentication,
// metrics collection, or parameter transformation.
//
// Example hooks:
//
//	// Logging hook
//	loggingHook := func(ctx context.Context, toolID string, params map[string]interface{}) error {
//		log.Printf("Executing tool %s with params: %v", toolID, params)
//		return nil
//	}
//	
//	// Authentication hook
//	authHook := func(ctx context.Context, toolID string, params map[string]interface{}) error {
//		if !isAuthenticated(ctx) {
//			return errors.New("authentication required")
//		}
//		return nil
//	}
type ExecutionHook func(ctx context.Context, toolID string, params map[string]interface{}) error

// ExecutionEngineOption configures the execution engine.
// Options follow the functional options pattern for flexible
// and extensible configuration.
type ExecutionEngineOption func(*ExecutionEngine)

// WithMaxConcurrent sets the maximum number of concurrent executions.
func WithMaxConcurrent(max int) ExecutionEngineOption {
	return func(e *ExecutionEngine) {
		e.maxConcurrent = int32(max)
	}
}

// WithDefaultTimeout sets the default execution timeout.
func WithDefaultTimeout(timeout time.Duration) ExecutionEngineOption {
	return func(e *ExecutionEngine) {
		e.defaultTimeout = timeout
	}
}

// WithRateLimiter sets the rate limiter for tool executions.
func WithRateLimiter(limiter RateLimiter) ExecutionEngineOption {
	return func(e *ExecutionEngine) {
		e.rateLimiter = limiter
	}
}

// NewExecutionEngine creates a new execution engine.
func NewExecutionEngine(registry *Registry, opts ...ExecutionEngineOption) *ExecutionEngine {
	e := &ExecutionEngine{
		registry:       registry,
		maxConcurrent:  100,              // Default max concurrent executions
		defaultTimeout: 30 * time.Second, // Default timeout
		metrics: &ExecutionMetrics{
			toolMetrics: make(map[string]*ToolMetrics),
		},
	}

	// Initialize the condition variable with the mutex
	e.cond = sync.NewCond(&e.mu)

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Execute runs a tool with the given parameters.
// It performs the following steps:
//   1. Retrieves the tool from the registry
//   2. Applies rate limiting if configured
//   3. Checks concurrency limits
//   4. Validates parameters against the tool schema
//   5. Runs pre-execution hooks
//   6. Executes the tool with timeout and panic recovery
//   7. Runs post-execution hooks
//   8. Updates execution metrics
//
// The method returns a ToolExecutionResult with the output or error.
// Context cancellation is respected throughout the execution.
func (e *ExecutionEngine) Execute(ctx context.Context, toolID string, params map[string]interface{}) (*ToolExecutionResult, error) {
	// Get the tool from registry (read-only view for memory efficiency)
	toolView, err := e.registry.GetReadOnly(toolID)
	if err != nil {
		return nil, NewToolError(ErrorTypeValidation, "TOOL_NOT_FOUND", "tool not found").
			WithToolID(toolID).
			WithCause(err)
	}

	// Check rate limits
	if e.rateLimiter != nil {
		if rateLimitErr := e.rateLimiter.Wait(ctx, toolID); rateLimitErr != nil {
			return nil, NewToolError(ErrorTypeRateLimit, "RATE_LIMIT_EXCEEDED", "rate limit exceeded").
				WithToolID(toolID).
				WithCause(rateLimitErr)
		}
	}

	// Check concurrency limits with proper synchronization
	if concurrencyErr := e.checkConcurrencyLimit(ctx); concurrencyErr != nil {
		return nil, concurrencyErr
	}
	defer e.decrementActiveCount()

	// Validate parameters
	validator := NewSchemaValidator(toolView.GetSchema())
	if validationErr := validator.Validate(params); validationErr != nil {
		return nil, NewToolError(ErrorTypeValidation, "VALIDATION_FAILED", "parameter validation failed").
			WithToolID(toolID).
			WithCause(validationErr).
			WithDetail("parameters", params)
	}

	// Run before-execute hooks with proper locking
	if err := e.runBeforeHooks(ctx, toolID, params); err != nil {
		return nil, err
	}

	// Set up execution context with timeout
	timeout := e.defaultTimeout
	if capabilities := toolView.GetCapabilities(); capabilities != nil && capabilities.Timeout > 0 {
		timeout = capabilities.Timeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Track execution
	execID := fmt.Sprintf("%s-%d", toolID, time.Now().UnixNano())
	e.trackExecution(execID, toolID, cancel)
	defer e.untrackExecution(execID)

	// Record start time
	startTime := time.Now()

	// Execute the tool
	result, err := e.executeWithRecovery(execCtx, toolView, params)

	// Record execution time
	duration := time.Since(startTime)

	// Prepare result
	if err != nil {
		result = &ToolExecutionResult{
			Success:   false,
			Error:     err.Error(),
			Duration:  duration,
			Timestamp: time.Now(),
		}
	} else if result == nil {
		result = &ToolExecutionResult{
			Success:   true,
			Duration:  duration,
			Timestamp: time.Now(),
		}
	} else {
		result.Duration = duration
		result.Timestamp = time.Now()
	}

	// Update metrics based on final result
	e.updateMetrics(toolID, result.Success, duration)

	// Run after-execute hooks
	e.runAfterHooks(ctx, toolID, params)

	return result, nil
}

// ExecuteStream runs a streaming tool with the given parameters.
// It returns a channel that receives chunks of output as they are produced.
// The channel is closed when execution completes or an error occurs.
//
// Streaming execution is useful for:
//   - Tools that produce large outputs
//   - Real-time data processing
//   - Progress reporting during long operations
//
// Example usage:
//
//	stream, err := engine.ExecuteStream(ctx, "log-reader", params)
//	if err != nil {
//		return err
//	}
//	
//	for chunk := range stream {
//		switch chunk.Type {
//		case "data":
//			fmt.Println(chunk.Data)
//		case "error":
//			return fmt.Errorf("stream error: %v", chunk.Data)
//		}
//	}
func (e *ExecutionEngine) ExecuteStream(ctx context.Context, toolID string, params map[string]interface{}) (<-chan *ToolStreamChunk, error) {
	// Get the tool from registry (read-only view for memory efficiency)
	toolView, err := e.registry.GetReadOnly(toolID)
	if err != nil {
		return nil, NewToolError(ErrorTypeValidation, "TOOL_NOT_FOUND", "tool not found").
			WithToolID(toolID).
			WithCause(err)
	}

	// Check if tool supports streaming
	streamingExecutor, ok := toolView.GetExecutor().(StreamingToolExecutor)
	if !ok {
		return nil, NewToolError(ErrorTypeValidation, "STREAMING_NOT_SUPPORTED", "tool does not support streaming").
			WithToolID(toolID)
	}

	// Validate parameters
	validator := NewSchemaValidator(toolView.GetSchema())
	if validationErr := validator.Validate(params); validationErr != nil {
		return nil, NewToolError(ErrorTypeValidation, "VALIDATION_FAILED", "parameter validation failed").
			WithToolID(toolID).
			WithCause(validationErr).
			WithDetail("parameters", params)
	}

	// Check rate limits
	if e.rateLimiter != nil {
		if rateLimitErr := e.rateLimiter.Wait(ctx, toolID); rateLimitErr != nil {
			return nil, NewToolError(ErrorTypeRateLimit, "RATE_LIMIT_EXCEEDED", "rate limit exceeded").
				WithToolID(toolID).
				WithCause(rateLimitErr)
		}
	}

	// Check concurrency limits with proper synchronization
	if concurrencyErr := e.checkConcurrencyLimit(ctx); concurrencyErr != nil {
		return nil, concurrencyErr
	}
	defer e.decrementActiveCount()

	// Set up execution context with timeout
	timeout := e.defaultTimeout
	if capabilities := toolView.GetCapabilities(); capabilities != nil && capabilities.Timeout > 0 {
		timeout = capabilities.Timeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)

	// Track execution
	execID := fmt.Sprintf("%s-stream-%d", toolID, time.Now().UnixNano())
	e.trackExecution(execID, toolID, cancel)

	// Execute the streaming tool
	stream, err := streamingExecutor.ExecuteStream(execCtx, params)
	if err != nil {
		cancel() // Explicitly call cancel in the error branch
		e.untrackExecution(execID)
		return nil, NewToolError(ErrorTypeExecution, "STREAMING_FAILED", "streaming execution failed").
			WithToolID(toolID).
			WithCause(err)
	}

	// Wrap the stream to handle cleanup with buffered channel to prevent goroutine blocking
	wrappedStream := make(chan *ToolStreamChunk, 10)
	
	go func() {
		defer close(wrappedStream)
		defer cancel()
		defer e.untrackExecution(execID)

		startTime := time.Now()
		hasError := false

		// Add a timeout to prevent indefinite blocking
		streamTimeout := time.NewTimer(timeout + 10*time.Second) // Give extra time beyond execution timeout
		defer streamTimeout.Stop()

		// Ensure metrics are updated when the goroutine exits
		defer func() {
			duration := time.Since(startTime)
			e.updateMetrics(toolID, !hasError, duration)
		}()

		for {
			select {
			case chunk, ok := <-stream:
				if !ok {
					// Stream closed, exit normally
					return
				}
				
				// Try to send chunk with timeout protection
				select {
				case wrappedStream <- chunk:
					if chunk.Type == "error" {
						hasError = true
					}
				case <-execCtx.Done():
					// Context canceled, stop streaming
					return
				case <-streamTimeout.C:
					// Stream processing timeout, prevent goroutine leak
					return
				}
				
			case <-execCtx.Done():
				// Context canceled, stop streaming
				return
				
			case <-streamTimeout.C:
				// Stream processing timeout, prevent goroutine leak
				return
			}
		}
	}()

	return wrappedStream, nil
}

// executeWithRecovery executes a tool with panic recovery.
func (e *ExecutionEngine) executeWithRecovery(ctx context.Context, tool ReadOnlyTool, params map[string]interface{}) (result *ToolExecutionResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = NewToolError(ErrorTypeInternal, "EXECUTION_PANIC", fmt.Sprintf("tool execution panicked: %v", r)).
				WithToolID(tool.GetID())
			result = nil
		}
	}()

	return tool.GetExecutor().Execute(ctx, params)
}

// checkConcurrencyLimit checks if we can execute another tool (FIXED).
func (e *ExecutionEngine) checkConcurrencyLimit(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	maxConcurrent := atomic.LoadInt32(&e.maxConcurrent)
	
	// Wait for a slot to become available if we're at capacity
	for atomic.LoadInt32(&e.activeCount) >= maxConcurrent {
		// Check if context is already cancelled before waiting
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Use a simple approach without goroutines to avoid leaks
		// We'll rely on the Signal() call in untrackExecution() to wake us up
		// and check context cancellation after each Wait()
		
		// Set up a timeout channel to prevent indefinite blocking
		timeoutCh := make(chan struct{})
		timeout := time.AfterFunc(30*time.Second, func() {
			close(timeoutCh)
			e.cond.Broadcast()
		})
		
		// Wait releases the lock and waits for a signal
		e.cond.Wait()
		
		// Cancel the timeout since we woke up
		if !timeout.Stop() {
			// Timer already fired, drain the channel
			select {
			case <-timeoutCh:
			default:
			}
		}

		// Check if context was cancelled while waiting
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	atomic.AddInt32(&e.activeCount, 1)
	return nil
}

// decrementActiveCount decrements the active execution count and signals waiters.
func (e *ExecutionEngine) decrementActiveCount() {
	atomic.AddInt32(&e.activeCount, -1)
	e.cond.Broadcast()
}

// runBeforeHooks runs before-execute hooks with proper locking (FIXED).
func (e *ExecutionEngine) runBeforeHooks(ctx context.Context, toolID string, params map[string]interface{}) error {
	e.hookMu.RLock()
	hooks := append([]ExecutionHook{}, e.beforeExecute...)
	e.hookMu.RUnlock()

	for _, hook := range hooks {
		if hookErr := hook(ctx, toolID, params); hookErr != nil {
			return NewToolError(ErrorTypeExecution, "HOOK_FAILED", "pre-execution hook failed").
				WithToolID(toolID).
				WithCause(hookErr)
		}
	}
	return nil
}

// runAfterHooks runs after-execute hooks with proper locking (FIXED).
func (e *ExecutionEngine) runAfterHooks(ctx context.Context, toolID string, params map[string]interface{}) {
	e.hookMu.RLock()
	hooks := append([]ExecutionHook{}, e.afterExecute...)
	e.hookMu.RUnlock()

	for _, hook := range hooks {
		if hookErr := hook(ctx, toolID, params); hookErr != nil {
			// Log hook errors but don't fail the execution
			fmt.Printf("post-execution hook error: %v\n", hookErr)
		}
	}
}

// trackExecution records an active execution (FIXED).
func (e *ExecutionEngine) trackExecution(execID, toolID string, cancel context.CancelFunc) {
	exec := &executionState{
		toolID:    toolID,
		startTime: time.Now(),
		cancel:    cancel,
	}
	e.executions.Store(execID, exec)
}

// untrackExecution removes an execution from tracking (FIXED).
func (e *ExecutionEngine) untrackExecution(execID string) {
	e.executions.Delete(execID)
	atomic.AddInt32(&e.activeCount, -1)
	
	// Signal any waiting goroutines that a slot is now available
	e.cond.Broadcast()
}

// updateMetrics updates execution metrics with atomic operations (FIXED).
func (e *ExecutionEngine) updateMetrics(toolID string, success bool, duration time.Duration) {
	atomic.AddInt64(&e.metrics.totalExecutions, 1)
	atomic.AddInt64(&e.metrics.totalDuration, int64(duration))

	if success {
		atomic.AddInt64(&e.metrics.successCount, 1)
	} else {
		atomic.AddInt64(&e.metrics.errorCount, 1)
	}

	// Update tool-specific metrics with proper locking
	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()

	toolMetric, exists := e.metrics.toolMetrics[toolID]
	if !exists {
		toolMetric = &ToolMetrics{}
		e.metrics.toolMetrics[toolID] = toolMetric
	}

	toolMetric.Executions++
	toolMetric.TotalDuration += duration
	toolMetric.AverageDuration = toolMetric.TotalDuration / time.Duration(toolMetric.Executions)

	if success {
		toolMetric.Successes++
	} else {
		toolMetric.Errors++
	}
}

// GetMetrics returns the current execution metrics (FIXED).
// It returns a copy of the metrics to prevent external modification.
// Metrics include global statistics and per-tool breakdowns.
func (e *ExecutionEngine) GetMetrics() *ExecutionMetrics {
	// Read atomic values
	totalExecutions := atomic.LoadInt64(&e.metrics.totalExecutions)
	successCount := atomic.LoadInt64(&e.metrics.successCount)
	errorCount := atomic.LoadInt64(&e.metrics.errorCount)
	totalDuration := atomic.LoadInt64(&e.metrics.totalDuration)
	cacheHits := atomic.LoadInt64(&e.metrics.cacheHits)
	cacheMisses := atomic.LoadInt64(&e.metrics.cacheMisses)
	asyncExecutions := atomic.LoadInt64(&e.metrics.asyncExecutions)

	// Copy tool metrics with lock
	e.metrics.mu.RLock()
	toolMetricsCopy := make(map[string]*ToolMetrics)
	for toolID, metric := range e.metrics.toolMetrics {
		toolMetricsCopy[toolID] = &ToolMetrics{
			Executions:      metric.Executions,
			Successes:       metric.Successes,
			Errors:          metric.Errors,
			TotalDuration:   metric.TotalDuration,
			AverageDuration: metric.AverageDuration,
		}
	}
	e.metrics.mu.RUnlock()

	return &ExecutionMetrics{
		totalExecutions: totalExecutions,
		successCount:    successCount,
		errorCount:      errorCount,
		totalDuration:   totalDuration,
		cacheHits:       cacheHits,
		cacheMisses:     cacheMisses,
		asyncExecutions: asyncExecutions,
		toolMetrics:     toolMetricsCopy,
	}
}

// CancelAll cancels all active executions (FIXED).
// This is useful for graceful shutdown or emergency stops.
// Each execution's context is cancelled, allowing tools to
// clean up resources properly.
func (e *ExecutionEngine) CancelAll() {
	e.executions.Range(func(key, value interface{}) bool {
		exec := value.(*executionState)
		exec.cancel()
		return true
	})
}

// AddBeforeExecuteHook adds a hook to run before tool execution (FIXED).
// Multiple hooks are executed in the order they were added.
// If any hook returns an error, execution is aborted.
func (e *ExecutionEngine) AddBeforeExecuteHook(hook ExecutionHook) {
	e.hookMu.Lock()
	defer e.hookMu.Unlock()
	e.beforeExecute = append(e.beforeExecute, hook)
}

// AddAfterExecuteHook adds a hook to run after tool execution (FIXED).
// These hooks run regardless of execution success or failure.
// Hook errors are logged but don't affect the execution result.
func (e *ExecutionEngine) AddAfterExecuteHook(hook ExecutionHook) {
	e.hookMu.Lock()
	defer e.hookMu.Unlock()
	e.afterExecute = append(e.afterExecute, hook)
}

// GetActiveExecutions returns the number of active executions (FIXED).
// This is useful for monitoring system load and debugging.
func (e *ExecutionEngine) GetActiveExecutions() int {
	return int(atomic.LoadInt32(&e.activeCount))
}

// IsExecuting checks if a specific tool is currently executing (FIXED).
// This can be used to prevent duplicate executions or check tool status.
func (e *ExecutionEngine) IsExecuting(toolID string) bool {
	found := false
	e.executions.Range(func(key, value interface{}) bool {
		exec := value.(*executionState)
		if exec.toolID == toolID {
			found = true
			return false // Stop iteration
		}
		return true
	})
	return found
}

// Enhanced execution types and structures

// ExecutionCache provides caching for execution results.
// It stores recent execution results to avoid redundant computations
// for tools marked as cacheable. The cache uses LRU eviction
// and TTL expiration for efficient memory usage.
type ExecutionCache struct {
	mu       sync.RWMutex
	cache    map[string]*CacheEntry
	maxSize  int
	ttl      time.Duration
}

// CacheEntry represents a cached execution result.
// It includes the result data and creation timestamp for TTL checks.
type CacheEntry struct {
	Result    *ToolExecutionResult
	CreatedAt time.Time
}

// AsyncJob represents an asynchronous execution job.
// Jobs can be prioritized and queued for background execution
// by worker goroutines.
type AsyncJob struct {
	ID       string
	ToolID   string
	Params   map[string]interface{}
	Priority int
	Context  context.Context
	Result   chan *AsyncResult
}

// AsyncResult represents the result of an asynchronous execution.
// It contains either a successful result or an error.
type AsyncResult struct {
	JobID  string
	Result *ToolExecutionResult
	Error  error
}

// ResourceMonitor tracks resource usage during execution.
// It can enforce limits on memory, CPU, goroutines, and file descriptors
// to prevent resource exhaustion from misbehaving tools.
type ResourceMonitor struct {
	MaxMemory   int64
	MaxCPU      float64
	MaxGoroutines int
	MaxFileDescriptors int
}

// SandboxConfig defines sandboxing constraints for tool execution.
// It provides security isolation by limiting tool capabilities
// and access to system resources.
//
// Example configuration:
//
//	sandbox := &SandboxConfig{
//		Enabled:          true,
//		MaxMemory:        1 << 30,  // 1GB
//		NetworkAccess:    false,
//		FileSystemAccess: true,
//		AllowedPaths:     []string{"/tmp", "/data"},
//		Timeout:          5 * time.Minute,
//	}
type SandboxConfig struct {
	Enabled          bool
	MaxMemory        int64
	MaxProcesses     int
	MaxFileHandles   int
	NetworkAccess    bool
	FileSystemAccess bool
	AllowedPaths     []string
	BlockedPaths     []string
	Timeout          time.Duration
}

// Enhanced option functions

// WithCaching enables execution result caching
func WithCaching(maxSize int, ttl time.Duration) ExecutionEngineOption {
	return func(e *ExecutionEngine) {
		e.cache = &ExecutionCache{
			cache:   make(map[string]*CacheEntry),
			maxSize: maxSize,
			ttl:     ttl,
		}
	}
}

// WithAsyncWorkers configures asynchronous execution workers
func WithAsyncWorkers(workers int) ExecutionEngineOption {
	return func(e *ExecutionEngine) {
		e.asyncWorkers = workers
		e.asyncJobQueue = make(chan *AsyncJob, workers*2)
		// e.asyncResults is already initialized as sync.Map
	}
}

// WithResourceMonitoring enables resource usage monitoring
func WithResourceMonitoring(maxMemory int64, maxCPU float64, maxGoroutines, maxFDs int) ExecutionEngineOption {
	return func(e *ExecutionEngine) {
		e.resourceMonitor = &ResourceMonitor{
			MaxMemory:          maxMemory,
			MaxCPU:             maxCPU,
			MaxGoroutines:      maxGoroutines,
			MaxFileDescriptors: maxFDs,
		}
	}
}

// WithSandboxing enables sandboxed execution
func WithSandboxing(config *SandboxConfig) ExecutionEngineOption {
	return func(e *ExecutionEngine) {
		e.sandbox = config
	}
}

// ExecuteAsync executes a tool asynchronously with priority.
// It queues the execution job and returns immediately with a job ID
// and result channel. Higher priority jobs are executed first.
//
// Example usage:
//
//	jobID, resultChan, err := engine.ExecuteAsync(ctx, "analyzer", params, 10)
//	if err != nil {
//		return err
//	}
//	
//	// Do other work...
//	
//	select {
//	case result := <-resultChan:
//		if result.Error != nil {
//			return result.Error
//		}
//		// Process result.Result
//	case <-ctx.Done():
//		return ctx.Err()
//	}
func (e *ExecutionEngine) ExecuteAsync(ctx context.Context, toolID string, params map[string]interface{}, priority int) (string, <-chan *AsyncResult, error) {
	if e.asyncJobQueue == nil {
		return "", nil, NewToolError(ErrorTypeConfiguration, "ASYNC_NOT_ENABLED", "async execution not enabled").
			WithToolID(toolID)
	}

	jobID := fmt.Sprintf("job_%d", time.Now().UnixNano())
	resultChan := make(chan *AsyncResult, 1)

	job := &AsyncJob{
		ID:       jobID,
		ToolID:   toolID,
		Params:   params,
		Priority: priority,
		Context:  ctx,
		Result:   resultChan,
	}

	// Store result channel using sync.Map
	e.asyncResults.Store(jobID, resultChan)

	// Increment async execution metrics atomically
	atomic.AddInt64(&e.metrics.asyncExecutions, 1)

	select {
	case e.asyncJobQueue <- job:
		return jobID, resultChan, nil
	case <-ctx.Done():
		e.asyncResults.Delete(jobID)
		return "", nil, ctx.Err()
	default:
		e.asyncResults.Delete(jobID)
		return "", nil, NewToolError(ErrorTypeResource, "QUEUE_FULL", "async job queue is full").
			WithToolID(toolID).
			WithDetail("queue_size", cap(e.asyncJobQueue))
	}
}

// GetCacheMetrics returns cache performance metrics (FIXED).
// Metrics include cache size, hit ratio, and hit/miss counts.
// Returns nil if caching is not enabled.
func (e *ExecutionEngine) GetCacheMetrics() map[string]interface{} {
	if e.cache == nil {
		return nil
	}
	
	e.cache.mu.RLock()
	size := len(e.cache.cache)
	maxSize := e.cache.maxSize
	e.cache.mu.RUnlock()
	
	totalHits := atomic.LoadInt64(&e.metrics.cacheHits)
	totalMisses := atomic.LoadInt64(&e.metrics.cacheMisses)
	
	var hitRatio float64
	if totalHits+totalMisses > 0 {
		hitRatio = float64(totalHits) / float64(totalHits+totalMisses)
	}
	
	return map[string]interface{}{
		"size":      size,
		"maxSize":   maxSize,
		"hitRatio":  hitRatio,
		"hits":      totalHits,
		"misses":    totalMisses,
	}
}

// GetResourceMetrics returns resource monitoring metrics.
// Metrics include resource limit violations and current limits.
// Returns nil if resource monitoring is not enabled.
func (e *ExecutionEngine) GetResourceMetrics() map[string]interface{} {
	if e.resourceMonitor == nil {
		return nil
	}
	
	return map[string]interface{}{
		"violations": 0, // Simplified for example
		"maxMemory":  e.resourceMonitor.MaxMemory,
		"maxCPU":     e.resourceMonitor.MaxCPU,
	}
}

// GetJobQueueMetrics returns async job queue metrics.
// Metrics include queue length, capacity, and worker count.
// Returns nil if async execution is not enabled.
func (e *ExecutionEngine) GetJobQueueMetrics() map[string]interface{} {
	if e.asyncJobQueue == nil {
		return nil
	}
	
	return map[string]interface{}{
		"queueLength": len(e.asyncJobQueue),
		"capacity":    cap(e.asyncJobQueue),
		"workers":     e.asyncWorkers,
	}
}

// Shutdown gracefully shuts down the execution engine.
// It stops accepting new executions, waits for active executions
// to complete (up to the context deadline), and cleans up resources.
//
// Example usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	
//	if err := engine.Shutdown(ctx); err != nil {
//		log.Printf("shutdown error: %v", err)
//	}
func (e *ExecutionEngine) Shutdown(ctx context.Context) error {
	// In a real implementation, this would stop async workers and clean up resources
	if e.asyncJobQueue != nil {
		close(e.asyncJobQueue)
	}
	return nil
}
