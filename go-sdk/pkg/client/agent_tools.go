package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// AgentToolManager provides seamless tool integration for agents with tool discovery,
// registration, execution with parameter validation, streaming responses, result caching,
// security sandboxing, and comprehensive error handling.
//
// Key features:
//   - Integration with existing tool system
//   - Asynchronous tool execution
//   - Resource limits and timeout handling
//   - Tool discovery and registration
//   - Streaming tool responses
//   - Security and sandboxing for tool execution
//   - Tool result caching and optimization
type AgentToolManager struct {
	// Configuration
	config ToolsConfig

	// Tool registry
	registry  *tools.Registry
	toolCache *ToolCache

	// Execution management
	executor   *ToolExecutor
	executions map[string]*ToolExecution
	execMu     sync.RWMutex

	// Security and sandboxing
	sandbox *ToolSandbox

	// Resource management
	semaphore chan struct{} // For limiting concurrent executions

	// Metrics and monitoring
	metrics   ToolManagerMetrics
	metricsMu sync.RWMutex

	// Lifecycle
	running   atomic.Bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	isHealthy atomic.Bool
}

// ToolCache provides efficient caching of tool results.
type ToolCache struct {
	cache         map[string]*CachedResult
	mu            sync.RWMutex
	maxSize       int
	ttl           time.Duration
	cleanupTicker *time.Ticker
}

// CachedResult represents a cached tool execution result.
type CachedResult struct {
	Result    interface{} `json:"result"`
	Error     string      `json:"error,omitempty"`
	Cached    time.Time   `json:"cached"`
	Hits      int64       `json:"hits"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// ToolExecutor manages tool execution with sandboxing and resource limits.
type ToolExecutor struct {
	timeout        time.Duration
	sandbox        *ToolSandbox
	resourceLimits ResourceLimits
}

// ToolExecution represents an active tool execution.
type ToolExecution struct {
	ID         string                 `json:"id"`
	ToolName   string                 `json:"tool_name"`
	Parameters interface{}            `json:"parameters"`
	StartTime  time.Time              `json:"start_time"`
	Status     ExecutionStatus        `json:"status"`
	Result     interface{}            `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Duration   time.Duration          `json:"duration"`
	Context    context.Context        `json:"-"`
	Cancel     context.CancelFunc     `json:"-"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ExecutionStatus represents the status of a tool execution.
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
	ExecutionStatusTimeout   ExecutionStatus = "timeout"
)

// ToolSandbox provides security sandboxing for tool execution.
type ToolSandbox struct {
	enabled        bool
	allowedPaths   []string
	blockedPaths   []string
	networkPolicy  NetworkPolicy
	resourceLimits ResourceLimits
}

// NetworkPolicy defines network access restrictions.
type NetworkPolicy struct {
	AllowOutbound bool     `json:"allow_outbound"`
	AllowedHosts  []string `json:"allowed_hosts"`
	BlockedHosts  []string `json:"blocked_hosts"`
	AllowedPorts  []int    `json:"allowed_ports"`
}

// ResourceLimits defines resource usage limits for tool execution.
type ResourceLimits struct {
	MaxMemory    int64         `json:"max_memory"`     // In bytes
	MaxCPUTime   time.Duration `json:"max_cpu_time"`   // CPU time limit
	MaxDiskSpace int64         `json:"max_disk_space"` // In bytes
	MaxNetworkIO int64         `json:"max_network_io"` // In bytes
}

// ToolManagerMetrics contains metrics for the tool manager.
type ToolManagerMetrics struct {
	ToolsExecuted         int64         `json:"tools_executed"`
	SuccessfulExecutions  int64         `json:"successful_executions"`
	FailedExecutions      int64         `json:"failed_executions"`
	CachedExecutions      int64         `json:"cached_executions"`
	AverageExecutionTime  time.Duration `json:"average_execution_time"`
	ConcurrentExecutions  int32         `json:"concurrent_executions"`
	SandboxViolations     int64         `json:"sandbox_violations"`
	ResourceLimitExceeded int64         `json:"resource_limit_exceeded"`
	LastExecutionTime     time.Time     `json:"last_execution_time"`
}

// StreamingResult represents a streaming tool execution result.
type StreamingResult struct {
	ExecutionID string      `json:"execution_id"`
	Chunk       interface{} `json:"chunk"`
	IsComplete  bool        `json:"is_complete"`
	Error       string      `json:"error,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
}

// NewAgentToolManager creates a new agent tool manager with the given configuration.
func NewAgentToolManager(config ToolsConfig) (*AgentToolManager, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10
	}

	// Create tool registry
	registry := tools.NewRegistry()

	// Create tool cache
	cache := &ToolCache{
		cache:   make(map[string]*CachedResult),
		maxSize: 1000, // Configurable
		ttl:     5 * time.Minute,
	}

	// Create executor
	executor := &ToolExecutor{
		timeout: config.Timeout,
		resourceLimits: ResourceLimits{
			MaxMemory:    100 * 1024 * 1024, // 100MB
			MaxCPUTime:   config.Timeout,
			MaxDiskSpace: 50 * 1024 * 1024, // 50MB
			MaxNetworkIO: 10 * 1024 * 1024, // 10MB
		},
	}

	// Create sandbox if enabled
	var sandbox *ToolSandbox
	if config.EnableSandboxing {
		sandbox = &ToolSandbox{
			enabled: true,
			networkPolicy: NetworkPolicy{
				AllowOutbound: false,
				AllowedHosts:  []string{},
				BlockedHosts:  []string{"localhost", "127.0.0.1"},
				AllowedPorts:  []int{80, 443},
			},
			resourceLimits: executor.resourceLimits,
		}
		executor.sandbox = sandbox
	}

	manager := &AgentToolManager{
		config:     config,
		registry:   registry,
		toolCache:  cache,
		executor:   executor,
		executions: make(map[string]*ToolExecution),
		sandbox:    sandbox,
		semaphore:  make(chan struct{}, config.MaxConcurrent),
		metrics: ToolManagerMetrics{
			LastExecutionTime: time.Now(),
		},
	}

	manager.isHealthy.Store(true)

	return manager, nil
}

// Start begins tool management operations.
func (tm *AgentToolManager) Start(ctx context.Context) error {
	if tm.running.Load() {
		return errors.NewAgentError(errors.ErrorTypeInvalidState, "tool manager is already running", "tool_manager")
	}

	tm.ctx, tm.cancel = context.WithCancel(ctx)
	tm.running.Store(true)

	// Start cache cleanup if caching is enabled
	if tm.config.EnableCaching {
		tm.toolCache.cleanupTicker = time.NewTicker(1 * time.Minute)
		tm.wg.Add(1)
		go tm.cacheCleanupLoop()
	}

	// Start metrics collection
	tm.wg.Add(1)
	go tm.metricsLoop()

	// Start execution monitoring
	tm.wg.Add(1)
	go tm.executionMonitorLoop()

	// Register built-in tools
	if err := tm.registerBuiltinTools(); err != nil {
		return fmt.Errorf("failed to register builtin tools: %w", err)
	}

	return nil
}

// Stop gracefully stops tool management.
func (tm *AgentToolManager) Stop(ctx context.Context) error {
	if !tm.running.Load() {
		return nil
	}

	tm.running.Store(false)
	tm.cancel()

	// Cancel all active executions
	tm.execMu.Lock()
	for _, execution := range tm.executions {
		if execution.Cancel != nil {
			execution.Cancel()
		}
	}
	tm.execMu.Unlock()

	// Stop cache cleanup
	if tm.toolCache.cleanupTicker != nil {
		tm.toolCache.cleanupTicker.Stop()
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		tm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for tool manager to stop")
	}

	return nil
}

// Cleanup releases all resources.
func (tm *AgentToolManager) Cleanup() error {
	tm.toolCache.clear()
	tm.executions = make(map[string]*ToolExecution)
	return nil
}

// ExecuteTool executes a tool with the given name and parameters.
func (tm *AgentToolManager) ExecuteTool(ctx context.Context, name string, params interface{}) (interface{}, error) {
	if !tm.running.Load() {
		return nil, errors.NewAgentError(errors.ErrorTypeInvalidState, "tool manager is not running", "tool_manager")
	}

	// Check if tool exists
	tool, err := tm.registry.Get(name)
	if err != nil {
		return nil, errors.NewAgentError(errors.ErrorTypeNotFound, fmt.Sprintf("tool %s not found", name), "tool_manager")
	}

	// Check cache if enabled
	if tm.config.EnableCaching {
		if cached := tm.getCachedResult(name, params); cached != nil {
			atomic.AddInt64(&tm.metrics.CachedExecutions, 1)
			if cached.Error != "" {
				return nil, fmt.Errorf("cached error: %s", cached.Error)
			}
			return cached.Result, nil
		}
	}

	// Acquire semaphore for concurrency control
	select {
	case tm.semaphore <- struct{}{}:
		defer func() { <-tm.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Update concurrent execution count
	atomic.AddInt32(&tm.metrics.ConcurrentExecutions, 1)
	defer atomic.AddInt32(&tm.metrics.ConcurrentExecutions, -1)

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, tm.config.Timeout)
	defer cancel()

	// Create execution record
	execution := &ToolExecution{
		ID:         fmt.Sprintf("exec_%d", time.Now().UnixNano()),
		ToolName:   name,
		Parameters: params,
		StartTime:  time.Now(),
		Status:     ExecutionStatusPending,
		Context:    execCtx,
		Cancel:     cancel,
		Metadata:   make(map[string]interface{}),
	}

	// Register execution
	tm.execMu.Lock()
	tm.executions[execution.ID] = execution
	tm.execMu.Unlock()

	// Clean up execution record when done
	defer func() {
		tm.execMu.Lock()
		delete(tm.executions, execution.ID)
		tm.execMu.Unlock()
	}()

	// Convert tool to ReadOnlyTool for safe access
	readOnlyTool := tools.NewReadOnlyTool(tool)

	// Execute tool
	result, err := tm.executeToolWithSandbox(execCtx, readOnlyTool, params, execution)

	// Update execution record
	execution.Duration = time.Since(execution.StartTime)
	if err != nil {
		execution.Status = ExecutionStatusFailed
		execution.Error = err.Error()
		atomic.AddInt64(&tm.metrics.FailedExecutions, 1)
	} else {
		execution.Status = ExecutionStatusCompleted
		execution.Result = result
		atomic.AddInt64(&tm.metrics.SuccessfulExecutions, 1)
	}

	// Update metrics
	atomic.AddInt64(&tm.metrics.ToolsExecuted, 1)
	tm.updateAverageExecutionTime(execution.Duration)

	// Cache result if enabled
	if tm.config.EnableCaching && err == nil {
		tm.cacheResult(name, params, result, err)
	}

	tm.metricsMu.Lock()
	tm.metrics.LastExecutionTime = time.Now()
	tm.metricsMu.Unlock()

	return result, err
}

// ExecuteToolStreaming executes a tool with streaming results.
func (tm *AgentToolManager) ExecuteToolStreaming(ctx context.Context, name string, params interface{}) (<-chan StreamingResult, error) {
	// Create result channel
	resultChan := make(chan StreamingResult, 100)

	// Start execution in goroutine
	go func() {
		defer close(resultChan)

		// Execute tool normally first
		result, err := tm.ExecuteTool(ctx, name, params)

		if err != nil {
			resultChan <- StreamingResult{
				ExecutionID: fmt.Sprintf("stream_%d", time.Now().UnixNano()),
				Error:       err.Error(),
				IsComplete:  true,
				Timestamp:   time.Now(),
			}
			return
		}

		// For non-streaming tools, send the complete result
		resultChan <- StreamingResult{
			ExecutionID: fmt.Sprintf("stream_%d", time.Now().UnixNano()),
			Chunk:       result,
			IsComplete:  true,
			Timestamp:   time.Now(),
		}
	}()

	return resultChan, nil
}

// ListTools returns a list of available tools.
func (tm *AgentToolManager) ListTools() []ToolDefinition {
	allTools, _ := tm.registry.List(nil)
	definitions := make([]ToolDefinition, len(allTools))

	for i, tool := range allTools {
		readOnlyTool := tools.NewReadOnlyTool(tool)
		definitions[i] = ToolDefinition{
			Name:        readOnlyTool.GetID(),
			Description: readOnlyTool.GetDescription(),
			Schema:      readOnlyTool.GetSchema(),
			Capabilities: map[string]interface{}{
				"version":   readOnlyTool.GetVersion(),
				"cacheable": readOnlyTool.GetCapabilities() != nil && readOnlyTool.GetCapabilities().Cacheable,
				"streaming": readOnlyTool.GetCapabilities() != nil && readOnlyTool.GetCapabilities().Streaming,
				"async":     readOnlyTool.GetCapabilities() != nil && readOnlyTool.GetCapabilities().Async,
			},
		}
	}

	return definitions
}

// RegisterTool registers a new tool with the manager.
func (tm *AgentToolManager) RegisterTool(tool *tools.Tool) error {
	return tm.registry.Register(tool)
}

// UnregisterTool removes a tool from the manager.
func (tm *AgentToolManager) UnregisterTool(name string) error {
	return tm.registry.Unregister(name)
}

// GetExecution returns information about a specific execution.
func (tm *AgentToolManager) GetExecution(executionID string) (*ToolExecution, error) {
	tm.execMu.RLock()
	defer tm.execMu.RUnlock()

	execution, exists := tm.executions[executionID]
	if !exists {
		return nil, errors.NewAgentError(errors.ErrorTypeNotFound, fmt.Sprintf("execution %s not found", executionID), "tool_manager")
	}

	return execution, nil
}

// CancelExecution cancels a running execution.
func (tm *AgentToolManager) CancelExecution(executionID string) error {
	tm.execMu.RLock()
	defer tm.execMu.RUnlock()

	execution, exists := tm.executions[executionID]
	if !exists {
		return errors.NewAgentError(errors.ErrorTypeNotFound, fmt.Sprintf("execution %s not found", executionID), "tool_manager")
	}

	if execution.Cancel != nil {
		execution.Cancel()
		execution.Status = ExecutionStatusCancelled
	}

	return nil
}

// GetMetrics returns current tool manager metrics.
func (tm *AgentToolManager) GetMetrics() ToolManagerMetrics {
	tm.metricsMu.RLock()
	defer tm.metricsMu.RUnlock()
	return tm.metrics
}

// IsHealthy returns the health status.
func (tm *AgentToolManager) IsHealthy() bool {
	return tm.isHealthy.Load()
}

// Private methods

func (tm *AgentToolManager) executeToolWithSandbox(ctx context.Context, tool tools.ReadOnlyTool, params interface{}, execution *ToolExecution) (interface{}, error) {
	execution.Status = ExecutionStatusRunning

	// Validate parameters
	if err := tm.validateParameters(tool, params); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// Apply sandbox if enabled
	if tm.sandbox != nil && tm.sandbox.enabled {
		if err := tm.applySandbox(ctx, execution); err != nil {
			atomic.AddInt64(&tm.metrics.SandboxViolations, 1)
			return nil, fmt.Errorf("sandbox violation: %w", err)
		}
	}

	// Execute the tool
	executor := tool.GetExecutor()
	if executor == nil {
		return nil, errors.NewAgentError(errors.ErrorTypeValidation, fmt.Sprintf("tool %s has no executor", tool.GetID()), "tool_manager")
	}

	// Monitor resource usage
	resourceMonitor := tm.startResourceMonitoring(ctx, execution)
	defer resourceMonitor.Stop()

	// Execute with timeout
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return nil, errors.NewAgentError(errors.ErrorTypeValidation, "params must be a map[string]interface{}", "tool_manager")
	}
	result, err := executor.Execute(ctx, paramsMap)
	if err != nil {
		// Check if it's a timeout error
		if ctx.Err() == context.DeadlineExceeded {
			execution.Status = ExecutionStatusTimeout
			return nil, errors.NewAgentError(errors.ErrorTypeTimeout, fmt.Sprintf("tool %s execution timed out", tool.GetID()), "tool_manager")
		}
		return nil, err
	}

	return result, nil
}

func (tm *AgentToolManager) validateParameters(tool tools.ReadOnlyTool, params interface{}) error {
	schema := tool.GetSchema()
	if schema == nil {
		return nil // No schema validation required
	}

	// Convert params to map[string]interface{}
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("params must be a map[string]interface{}")
	}

	// Create validator and validate parameters against schema
	validator := tools.NewSchemaValidator(schema)
	return validator.Validate(paramsMap)
}

func (tm *AgentToolManager) applySandbox(ctx context.Context, execution *ToolExecution) error {
	// Apply sandbox restrictions
	// This is a simplified implementation
	if tm.sandbox.networkPolicy.AllowOutbound == false {
		execution.Metadata["network_restricted"] = true
	}

	return nil
}

func (tm *AgentToolManager) startResourceMonitoring(ctx context.Context, execution *ToolExecution) *ResourceMonitor {
	monitor := &ResourceMonitor{
		limits: tm.executor.resourceLimits,
		ctx:    ctx,
	}

	go monitor.Start(execution)
	return monitor
}

func (tm *AgentToolManager) registerBuiltinTools() error {
	// Register built-in tools
	// This would typically register common tools like HTTP, file operations, etc.
	return nil
}

func (tm *AgentToolManager) getCachedResult(toolName string, params interface{}) *CachedResult {
	cacheKey := tm.generateCacheKey(toolName, params)
	return tm.toolCache.get(cacheKey)
}

func (tm *AgentToolManager) cacheResult(toolName string, params interface{}, result interface{}, err error) {
	cacheKey := tm.generateCacheKey(toolName, params)

	cached := &CachedResult{
		Result:    result,
		Cached:    time.Now(),
		ExpiresAt: time.Now().Add(tm.toolCache.ttl),
	}

	if err != nil {
		cached.Error = err.Error()
	}

	tm.toolCache.set(cacheKey, cached)
}

func (tm *AgentToolManager) generateCacheKey(toolName string, params interface{}) string {
	// Generate cache key from tool name and parameters
	paramsBytes, _ := json.Marshal(params)
	return fmt.Sprintf("%s:%s", toolName, string(paramsBytes))
}

func (tm *AgentToolManager) updateAverageExecutionTime(duration time.Duration) {
	tm.metricsMu.Lock()
	defer tm.metricsMu.Unlock()

	if tm.metrics.AverageExecutionTime == 0 {
		tm.metrics.AverageExecutionTime = duration
	} else {
		tm.metrics.AverageExecutionTime = (tm.metrics.AverageExecutionTime + duration) / 2
	}
}

// Background loops

func (tm *AgentToolManager) cacheCleanupLoop() {
	defer tm.wg.Done()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-tm.toolCache.cleanupTicker.C:
			tm.toolCache.cleanup()
		}
	}
}

func (tm *AgentToolManager) metricsLoop() {
	defer tm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.updateHealthStatus()
		}
	}
}

func (tm *AgentToolManager) executionMonitorLoop() {
	defer tm.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.monitorExecutions()
		}
	}
}

func (tm *AgentToolManager) updateHealthStatus() {
	failedExecutions := atomic.LoadInt64(&tm.metrics.FailedExecutions)
	totalExecutions := atomic.LoadInt64(&tm.metrics.ToolsExecuted)

	if totalExecutions > 0 {
		failureRate := float64(failedExecutions) / float64(totalExecutions)
		if failureRate > 0.5 { // 50% failure rate
			tm.isHealthy.Store(false)
		} else {
			tm.isHealthy.Store(true)
		}
	}
}

func (tm *AgentToolManager) monitorExecutions() {
	tm.execMu.RLock()
	defer tm.execMu.RUnlock()

	now := time.Now()
	for _, execution := range tm.executions {
		if execution.Status == ExecutionStatusRunning {
			// Check for long-running executions
			if now.Sub(execution.StartTime) > tm.config.Timeout*2 {
				// This execution is taking too long
				if execution.Cancel != nil {
					execution.Cancel()
				}
			}
		}
	}
}

// ToolCache methods

func (tc *ToolCache) get(key string) *CachedResult {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	result, exists := tc.cache[key]
	if !exists {
		return nil
	}

	// Check expiration
	if time.Now().After(result.ExpiresAt) {
		// Expired, remove it
		delete(tc.cache, key)
		return nil
	}

	// Update hit count
	atomic.AddInt64(&result.Hits, 1)
	return result
}

func (tc *ToolCache) set(key string, result *CachedResult) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.cache[key] = result

	// Check if cache is full
	if len(tc.cache) > tc.maxSize {
		tc.evictOldest()
	}
}

func (tc *ToolCache) cleanup() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	now := time.Now()
	for key, result := range tc.cache {
		if now.After(result.ExpiresAt) {
			delete(tc.cache, key)
		}
	}
}

func (tc *ToolCache) clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.cache = make(map[string]*CachedResult)
}

func (tc *ToolCache) evictOldest() {
	// Find oldest entry
	var oldestKey string
	var oldestTime time.Time

	for key, result := range tc.cache {
		if oldestTime.IsZero() || result.Cached.Before(oldestTime) {
			oldestKey = key
			oldestTime = result.Cached
		}
	}

	if oldestKey != "" {
		delete(tc.cache, oldestKey)
	}
}

// ResourceMonitor monitors resource usage during tool execution.
type ResourceMonitor struct {
	limits ResourceLimits
	ctx    context.Context
}

func (rm *ResourceMonitor) Start(execution *ToolExecution) {
	// Monitor resource usage
	// This is a simplified implementation
	// In a real implementation, this would monitor CPU, memory, disk, and network usage
}

func (rm *ResourceMonitor) Stop() {
	// Stop monitoring
}
