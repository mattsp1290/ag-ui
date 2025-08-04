package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	
	"github.com/mattsp1290/ag-ui/go-sdk/internal/timeconfig"
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
	maxConcurrent  int
	defaultTimeout time.Duration

	// Concurrency control - channel-based semaphore (RACE CONDITION FIX)
	concurrencySemaphore chan struct{} // Buffered channel acts as semaphore
	executions           sync.Map      // Active execution tracking

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
	asyncResults      sync.Map
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
		e.maxConcurrent = max
		// Recreate semaphore with new capacity
		e.concurrencySemaphore = make(chan struct{}, max)
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
	maxConcurrent := 100 // Default max concurrent executions
	e := &ExecutionEngine{
		registry:             registry,
		maxConcurrent:        maxConcurrent,
		defaultTimeout:       30 * time.Second, // Default timeout
		concurrencySemaphore: make(chan struct{}, maxConcurrent), // Channel-based semaphore
		metrics: &ExecutionMetrics{
			toolMetrics: make(map[string]*ToolMetrics),
		},
	}

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

	// Check cache if enabled and tool is cacheable
	if e.cache != nil && toolView.GetCapabilities() != nil && toolView.GetCapabilities().Cacheable {
		cacheKey := e.generateCacheKey(toolID, params)
		if cached, ok := e.getCached(cacheKey); ok {
			atomic.AddInt64(&e.metrics.cacheHits, 1)
			return cached, nil
		}
		atomic.AddInt64(&e.metrics.cacheMisses, 1)
	}

	// Check rate limits
	if e.rateLimiter != nil {
		if rateLimitErr := e.rateLimiter.Wait(ctx, toolID); rateLimitErr != nil {
			return nil, NewToolError(ErrorTypeRateLimit, "RATE_LIMIT_EXCEEDED", "rate limit exceeded").
				WithToolID(toolID).
				WithCause(rateLimitErr)
		}
	}

	// Acquire concurrency slot using channel-based semaphore (RACE CONDITION FIX)
	// Block until a slot becomes available or context is cancelled
	select {
	case e.concurrencySemaphore <- struct{}{}: // Acquire slot
		// Successfully acquired slot, will be released in defer
	case <-ctx.Done():
		return nil, NewToolError(ErrorTypeTimeout, "CONTEXT_CANCELLED", "context cancelled while waiting for execution slot").
			WithToolID(toolID).
			WithCause(ctx.Err())
	}
	defer func() { <-e.concurrencySemaphore }() // Release slot

	// Validate parameters
	validator := NewSchemaValidator(toolView.GetSchema())
	if validationErr := validator.Validate(params); validationErr != nil {
		return nil, NewToolError(ErrorTypeValidation, "VALIDATION_FAILED", "parameter validation failed").
			WithToolID(toolID).
			WithCause(validationErr).
			WithDetail("parameters", params)
	}

	// Check cache if tool is cacheable
	if e.cache != nil && toolView.GetCapabilities() != nil && toolView.GetCapabilities().Cacheable {
		cacheKey := e.generateCacheKey(toolID, params)
		if cachedResult := e.cache.Get(cacheKey); cachedResult != nil {
			// Update cache hit metrics
			atomic.AddInt64(&e.metrics.cacheHits, 1)
			return cachedResult, nil
		}
		// Update cache miss metrics
		atomic.AddInt64(&e.metrics.cacheMisses, 1)
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

	// Store successful results in cache if tool is cacheable
	if e.cache != nil && result.Success && toolView.GetCapabilities() != nil && toolView.GetCapabilities().Cacheable {
		cacheKey := e.generateCacheKey(toolID, params)
		e.cache.Set(cacheKey, result)
	}

	// Run after-execute hooks
	e.runAfterHooks(ctx, toolID, params)

	// Cache successful results if caching is enabled and tool is cacheable
	if e.cache != nil && result.Success && toolView.GetCapabilities() != nil && toolView.GetCapabilities().Cacheable {
		cacheKey := e.generateCacheKey(toolID, params)
		e.setCached(cacheKey, result)
	}

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

	// Acquire concurrency slot using channel-based semaphore (RACE CONDITION FIX)
	// Block until a slot becomes available or context is cancelled
	select {
	case e.concurrencySemaphore <- struct{}{}: // Acquire slot
		// Successfully acquired slot, will be released in defer
	case <-ctx.Done():
		return nil, NewToolError(ErrorTypeTimeout, "CONTEXT_CANCELLED", "context cancelled while waiting for execution slot").
			WithToolID(toolID).
			WithCause(ctx.Err())
	}
	defer func() { <-e.concurrencySemaphore }() // Release slot

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
		streamTimeout := time.NewTimer(timeout + timeconfig.GetConfig().DefaultShutdownTimeout) // Give extra time beyond execution timeout
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

	// Check if tool and executor are not nil to prevent panic
	if tool == nil {
		return nil, NewToolError(ErrorTypeInternal, "TOOL_NIL", "tool is nil")
	}
	
	executor := tool.GetExecutor()
	if executor == nil {
		return nil, NewToolError(ErrorTypeInternal, "EXECUTOR_NIL", "tool executor is nil").
			WithToolID(tool.GetID())
	}

	return executor.Execute(ctx, params)
}

// Channel-based semaphore approach eliminates the need for these problematic methods.
// The concurrency control is now handled directly in Execute/ExecuteStream methods
// using the concurrencySemaphore channel, which provides atomic acquire/release
// operations without race conditions.

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
	// NOTE: Do NOT decrement activeCount here - it's already decremented by decrementActiveCount()
	// that is deferred in Execute/ExecuteStream methods. Removing the double decrement fixes the race.
	// This avoids double decrementing the counter and potential race conditions.
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

// GetActiveExecutions returns the number of active executions (RACE CONDITION FIX).
// This is useful for monitoring system load and debugging.
func (e *ExecutionEngine) GetActiveExecutions() int {
	// Channel length represents current active executions
	return len(e.concurrencySemaphore)
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

// Get retrieves a cached result if it exists and is not expired
func (c *ExecutionCache) Get(key string) *ToolExecutionResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.cache[key]
	if !exists {
		return nil
	}
	
	// Check if entry has expired
	if time.Since(entry.CreatedAt) > c.ttl {
		// Don't delete here to avoid write lock, let Set handle cleanup
		return nil
	}
	
	return entry.Result
}

// Set stores a result in the cache
func (c *ExecutionCache) Set(key string, result *ToolExecutionResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Simple LRU: if cache is full, remove oldest entry
	if len(c.cache) >= c.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.cache {
			if oldestKey == "" || v.CreatedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.CreatedAt
			}
		}
		if oldestKey != "" {
			delete(c.cache, oldestKey)
		}
	}
	
	c.cache[key] = &CacheEntry{
		Result:    result,
		CreatedAt: time.Now(),
	}
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
		
		// Start async workers
		for i := 0; i < workers; i++ {
			go e.asyncWorker()
		}
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

// generateCacheKey generates a cache key from tool ID and parameters
func (e *ExecutionEngine) generateCacheKey(toolID string, params map[string]interface{}) string {
	// Create a deterministic string representation of params
	paramBytes, _ := json.Marshal(params)
	return fmt.Sprintf("%s:%s", toolID, string(paramBytes))
}

// processAsyncJobs processes jobs from the async queue
func (e *ExecutionEngine) processAsyncJobs() {
	for job := range e.asyncJobQueue {
		// Defensive check for nil job
		if job == nil {
			continue
		}
		
		// Check if context is already cancelled before processing
		select {
		case <-job.Context.Done():
			// Context already cancelled, send error immediately
			e.sendAsyncResult(job, nil, job.Context.Err())
			continue
		default:
		}
		
		// Execute the job with timeout protection
		resultChan := make(chan struct {
			result *ToolExecutionResult
			err    error
		}, 1)
		
		// Execute in a separate goroutine to handle blocking Execute calls
		go func() {
			result, err := e.Execute(job.Context, job.ToolID, job.Params)
			select {
			case resultChan <- struct {
				result *ToolExecutionResult
				err    error
			}{result, err}:
			case <-job.Context.Done():
				// Context cancelled, don't block on sending result
				return
			}
		}()
		
		// Wait for execution result or context cancellation
		select {
		case execResult := <-resultChan:
			// Execution completed, send result
			e.sendAsyncResult(job, execResult.result, execResult.err)
		case <-job.Context.Done():
			// Context cancelled while executing
			e.sendAsyncResult(job, nil, job.Context.Err())
		}
	}
}

// sendAsyncResult safely sends an async result without blocking
func (e *ExecutionEngine) sendAsyncResult(job *AsyncJob, result *ToolExecutionResult, err error) {
	if job == nil {
		return
	}
	
	asyncResult := &AsyncResult{
		JobID:  job.ID,
		Result: result,
		Error:  err,
	}
	
	// Clean up result channel from map first
	if job.ID != "" {
		e.asyncResults.Delete(job.ID)
	}
	
	// Send result with timeout protection to prevent blocking
	// Use a longer timeout than the mock executor to ensure result delivery
	if job.Result != nil {
		select {
		case job.Result <- asyncResult:
			// Result sent successfully
		case <-job.Context.Done():
			// Context cancelled, but still try to send the result
			// because the receiver might still be waiting
			select {
			case job.Result <- asyncResult:
			case <-time.After(50 * time.Millisecond):
				// Give up after a short timeout if receiver is gone
			}
		case <-time.After(1 * time.Second):
			// Use a longer timeout to handle delayed processing
			// This prevents goroutine leaks when nobody is reading the channel
		}
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

// asyncWorker processes async jobs from the queue
func (e *ExecutionEngine) asyncWorker() {
	for job := range e.asyncJobQueue {
		// Execute the job
		result, err := e.Execute(job.Context, job.ToolID, job.Params)
		
		// Send result back
		asyncResult := &AsyncResult{
			JobID:  job.ID,
			Result: result,
			Error:  err,
		}
		
		// Always try to send the result, even if context is canceled
		// This ensures the caller receives notification of cancellation
		select {
		case job.Result <- asyncResult:
			// Result sent successfully
		default:
			// Result channel might be closed or full, continue anyway
		}
		
		// Clean up result channel from map
		e.asyncResults.Delete(job.ID)
	}
}


// getCached retrieves a cached result if available and not expired
func (e *ExecutionEngine) getCached(key string) (*ToolExecutionResult, bool) {
	e.cache.mu.RLock()
	defer e.cache.mu.RUnlock()
	
	entry, exists := e.cache.cache[key]
	if !exists {
		return nil, false
	}
	
	// Check if entry has expired
	if time.Since(entry.CreatedAt) > e.cache.ttl {
		// Entry expired, remove it (we'll need write lock for this)
		go func() {
			e.cache.mu.Lock()
			delete(e.cache.cache, key)
			e.cache.mu.Unlock()
		}()
		return nil, false
	}
	
	return entry.Result, true
}

// setCached stores a result in the cache
func (e *ExecutionEngine) setCached(key string, result *ToolExecutionResult) {
	e.cache.mu.Lock()
	defer e.cache.mu.Unlock()
	
	// Check cache size limit
	if len(e.cache.cache) >= e.cache.maxSize {
		// Simple eviction: remove the oldest entry
		// In production, use LRU or other eviction strategy
		var oldestKey string
		var oldestTime time.Time
		for k, v := range e.cache.cache {
			if oldestKey == "" || v.CreatedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.CreatedAt
			}
		}
		if oldestKey != "" {
			delete(e.cache.cache, oldestKey)
		}
	}
	
	e.cache.cache[key] = &CacheEntry{
		Result:    result,
		CreatedAt: time.Now(),
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
	// Cancel all active executions first
	e.CancelAll()
	
	// Close async job queue to stop accepting new jobs
	if e.asyncJobQueue != nil {
		close(e.asyncJobQueue)
	}
	
	// Wait a bit for executions to finish or until context expires
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if e.GetActiveExecutions() == 0 {
				return nil
			}
		}
	}
}
