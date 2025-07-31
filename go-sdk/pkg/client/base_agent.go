package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// BaseAgent provides a common implementation of the Agent interface that can be
// embedded by specific agent implementations.
type BaseAgent struct {
	// Configuration and identity
	config *AgentConfig
	name   string
	desc   string
	
	// Lifecycle management
	status     atomic.Value // AgentStatus
	mu         sync.RWMutex
	startTime  time.Time
	
	// Event processing
	eventStream chan events.Event
	streamMu    sync.RWMutex
	
	// Metrics and monitoring
	metrics      AgentMetrics
	metricsMu    sync.RWMutex
	healthStatus HealthStatus
	healthMu     sync.RWMutex
	
	// Tool execution framework
	toolFramework *ToolExecutionFramework
	toolMu        sync.RWMutex
}

// AgentMetrics contains performance and operational metrics for an agent.
type AgentMetrics struct {
	EventsProcessed       int64         `json:"events_processed"`
	EventsPerSecond       float64       `json:"events_per_second"`
	AverageProcessingTime time.Duration `json:"average_processing_time"`
	ToolsExecuted         int64         `json:"tools_executed"`
	StateUpdates          int64         `json:"state_updates"`
	ErrorCount            int64         `json:"error_count"`
	MemoryUsage           int64         `json:"memory_usage"`
	StartTime             time.Time     `json:"start_time"`
	LastActivity          time.Time     `json:"last_activity"`
}

// NewBaseAgent creates a new base agent with default configuration.
func NewBaseAgent(name, description string) *BaseAgent {
	agent := &BaseAgent{
		name: name,
		desc: description,
		metrics: AgentMetrics{
			StartTime: time.Now(),
		},
		healthStatus: HealthStatus{
			Status:    "uninitialized",
			LastCheck: time.Now(),
			Details:   make(map[string]interface{}),
			Errors:    make([]string, 0),
		},
	}
	
	agent.status.Store(AgentStatusUninitialized)
	return agent
}

// Initialize prepares the agent with the given configuration.
func (a *BaseAgent) Initialize(ctx context.Context, config *AgentConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if a.getStatus() != AgentStatusUninitialized {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			fmt.Sprintf("agent %s is already initialized", a.name),
			a.name,
		)
	}
	
	// Validate configuration
	if err := a.validateConfig(config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	
	// Set configuration with defaults
	a.config = a.mergeWithDefaults(config)
	
	// Initialize event stream
	a.eventStream = make(chan events.Event, a.config.EventProcessing.BufferSize)
	
	// Update status and health
	a.setStatus(AgentStatusInitialized)
	a.updateHealth("initialized", nil)
	
	return nil
}

// Start begins the agent's operation.
func (a *BaseAgent) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	status := a.getStatus()
	if status != AgentStatusInitialized && status != AgentStatusStopped {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			fmt.Sprintf("agent %s cannot be started from status %s", a.name, status),
			a.name,
		)
	}
	
	a.setStatus(AgentStatusStarting)
	
	// Update status and metrics
	a.setStatus(AgentStatusRunning)
	a.startTime = time.Now()
	a.updateHealth("healthy", nil)
	
	return nil
}

// Stop gracefully shuts down the agent.
func (a *BaseAgent) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if a.getStatus() != AgentStatusRunning {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			fmt.Sprintf("agent %s is not running", a.name),
			a.name,
		)
	}
	
	a.setStatus(AgentStatusStopping)
	
	// Cancel all active tool executions
	if a.toolFramework != nil {
		a.toolFramework.CancelAll()
	}
	
	// Close event streams
	if a.eventStream != nil {
		close(a.eventStream)
		a.eventStream = nil
	}
	
	a.setStatus(AgentStatusStopped)
	a.updateHealth("stopped", nil)
	
	return nil
}

// Cleanup releases all resources held by the agent.
func (a *BaseAgent) Cleanup() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Shutdown tool framework
	if a.toolFramework != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		if err := a.toolFramework.Shutdown(ctx); err != nil {
			// Log error but continue cleanup
			fmt.Printf("Warning: tool framework shutdown error: %v\n", err)
		}
		a.toolFramework = nil
	}
	
	// Reset state
	a.config = nil
	
	return nil
}

// ProcessEvent handles a single incoming event and returns response events.
func (a *BaseAgent) ProcessEvent(ctx context.Context, event events.Event) ([]events.Event, error) {
	if a.getStatus() != AgentStatusRunning {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			fmt.Sprintf("agent %s is not running", a.name),
			a.name,
		)
	}
	
	// Update metrics
	a.incrementEventsProcessed()
	startTime := time.Now()
	
	defer func() {
		processingTime := time.Since(startTime)
		a.updateAverageProcessingTime(processingTime)
		a.updateLastActivity()
	}()
	
	// Validate the incoming event
	if err := event.Validate(); err != nil {
		a.incrementErrorCount()
		return nil, errors.NewAgentError(
			errors.ErrorTypeValidation,
			fmt.Sprintf("event validation failed: %v", err),
			a.name,
		)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Process the event based on its type
	responseEvents, err := a.processEventByType(ctx, event)
	if err != nil {
		a.incrementErrorCount()
		return nil, err
	}

	// Emit processed events to stream if streaming is enabled
	if a.config.Capabilities.Streaming && a.eventStream != nil {
		a.emitEventsToStream(responseEvents)
	}

	return responseEvents, nil
}

// processEventByType handles event processing based on the event type
func (a *BaseAgent) processEventByType(ctx context.Context, event events.Event) ([]events.Event, error) {
	switch event.Type() {
	case events.EventTypeTextMessageStart:
		return a.processTextMessageStart(ctx, event)
	case events.EventTypeTextMessageContent:
		return a.processTextMessageContent(ctx, event)
	case events.EventTypeTextMessageEnd:
		return a.processTextMessageEnd(ctx, event)
	case events.EventTypeToolCallStart:
		return a.processToolCallStart(ctx, event)
	case events.EventTypeToolCallArgs:
		return a.processToolCallArgs(ctx, event)
	case events.EventTypeToolCallEnd:
		return a.processToolCallEnd(ctx, event)
	case events.EventTypeStateSnapshot:
		return a.processStateSnapshot(ctx, event)
	case events.EventTypeStateDelta:
		return a.processStateDelta(ctx, event)
	case events.EventTypeMessagesSnapshot:
		return a.processMessagesSnapshot(ctx, event)
	case events.EventTypeRaw:
		return a.processRawEvent(ctx, event)
	case events.EventTypeCustom:
		return a.processCustomEvent(ctx, event)
	case events.EventTypeRunStarted:
		return a.processRunStarted(ctx, event)
	case events.EventTypeRunFinished:
		return a.processRunFinished(ctx, event)
	case events.EventTypeRunError:
		return a.processRunError(ctx, event)
	case events.EventTypeStepStarted:
		return a.processStepStarted(ctx, event)
	case events.EventTypeStepFinished:
		return a.processStepFinished(ctx, event)
	default:
		return nil, errors.NewAgentError(
			errors.ErrorTypeUnsupported,
			fmt.Sprintf("unsupported event type: %s", event.Type()),
			a.name,
		)
	}
}

// processTextMessageStart handles TEXT_MESSAGE_START events
func (a *BaseAgent) processTextMessageStart(ctx context.Context, event events.Event) ([]events.Event, error) {
	msgEvent, ok := event.(*events.TextMessageStartEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to TextMessageStartEvent",
			a.name,
		)
	}

	// Generate a response acknowledging the message start
	responseEvent := events.NewTextMessageContentEvent(
		msgEvent.MessageID,
		fmt.Sprintf("Processing message %s from agent %s", msgEvent.MessageID, a.name),
	)

	return []events.Event{responseEvent}, nil
}

// processTextMessageContent handles TEXT_MESSAGE_CONTENT events
func (a *BaseAgent) processTextMessageContent(ctx context.Context, event events.Event) ([]events.Event, error) {
	msgEvent, ok := event.(*events.TextMessageContentEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to TextMessageContentEvent",
			a.name,
		)
	}

	// Echo the content with agent processing indication
	responseEvent := events.NewTextMessageContentEvent(
		msgEvent.MessageID,
		fmt.Sprintf("[%s processed]: %s", a.name, msgEvent.Delta),
	)

	return []events.Event{responseEvent}, nil
}

// processTextMessageEnd handles TEXT_MESSAGE_END events
func (a *BaseAgent) processTextMessageEnd(ctx context.Context, event events.Event) ([]events.Event, error) {
	msgEvent, ok := event.(*events.TextMessageEndEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to TextMessageEndEvent",
			a.name,
		)
	}

	// Generate a completion confirmation
	responseEvent := events.NewTextMessageEndEvent(msgEvent.MessageID)

	return []events.Event{responseEvent}, nil
}

// processToolCallStart handles TOOL_CALL_START events
func (a *BaseAgent) processToolCallStart(ctx context.Context, event events.Event) ([]events.Event, error) {
	toolEvent, ok := event.(*events.ToolCallStartEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to ToolCallStartEvent",
			a.name,
		)
	}

	a.incrementToolsExecuted()

	// Check if the tool is available
	availableTools := a.ListTools()
	toolFound := false
	for _, tool := range availableTools {
		if tool.Name == toolEvent.ToolCallName {
			toolFound = true
			break
		}
	}

	if !toolFound {
		// Return error event for unknown tool
		errorEvent := events.NewRunErrorEvent(
			fmt.Sprintf("Tool '%s' not found", toolEvent.ToolCallName),
			events.WithErrorCode("TOOL_NOT_FOUND"),
		)
		return []events.Event{errorEvent}, nil
	}

	// Acknowledge tool call start
	responseEvent := events.NewToolCallArgsEvent(
		toolEvent.ToolCallID,
		fmt.Sprintf("Starting execution of tool: %s", toolEvent.ToolCallName),
	)

	return []events.Event{responseEvent}, nil
}

// processToolCallArgs handles TOOL_CALL_ARGS events
func (a *BaseAgent) processToolCallArgs(ctx context.Context, event events.Event) ([]events.Event, error) {
	toolEvent, ok := event.(*events.ToolCallArgsEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to ToolCallArgsEvent",
			a.name,
		)
	}

	// Process tool arguments (in base implementation, just echo)
	responseEvent := events.NewToolCallArgsEvent(
		toolEvent.ToolCallID,
		fmt.Sprintf("Processed args: %s", toolEvent.Delta),
	)

	return []events.Event{responseEvent}, nil
}

// processToolCallEnd handles TOOL_CALL_END events
func (a *BaseAgent) processToolCallEnd(ctx context.Context, event events.Event) ([]events.Event, error) {
	toolEvent, ok := event.(*events.ToolCallEndEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to ToolCallEndEvent",
			a.name,
		)
	}

	// Complete the tool call
	responseEvent := events.NewToolCallEndEvent(toolEvent.ToolCallID)

	return []events.Event{responseEvent}, nil
}

// processStateSnapshot handles STATE_SNAPSHOT events
func (a *BaseAgent) processStateSnapshot(ctx context.Context, event events.Event) ([]events.Event, error) {
	stateEvent, ok := event.(*events.StateSnapshotEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to StateSnapshotEvent",
			a.name,
		)
	}

	// Update agent state with snapshot
	currentState, err := a.GetState(ctx)
	if err != nil {
		return nil, err
	}

	// Merge or replace state based on snapshot
	updatedState := map[string]interface{}{
		"agent_state": currentState,
		"snapshot":    stateEvent.Snapshot,
		"updated_at":  time.Now(),
	}

	// Generate state delta response
	deltaOps := []events.JSONPatchOperation{
		{
			Op:    "replace",
			Path:  "/last_snapshot",
			Value: stateEvent.Snapshot,
		},
		{
			Op:    "replace",
			Path:  "/updated_at",
			Value: time.Now().Unix(),
		},
	}

	responseEvent := events.NewStateDeltaEvent(deltaOps)
	a.incrementStateUpdates()

	// Store updated state for future reference
	_ = updatedState // In a real implementation, this would update persistent state

	return []events.Event{responseEvent}, nil
}

// processStateDelta handles STATE_DELTA events
func (a *BaseAgent) processStateDelta(ctx context.Context, event events.Event) ([]events.Event, error) {
	deltaEvent, ok := event.(*events.StateDeltaEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to StateDeltaEvent",
			a.name,
		)
	}

	a.incrementStateUpdates()

	// Apply delta operations to current state
	for _, op := range deltaEvent.Delta {
		switch op.Op {
		case "add", "replace":
			// Apply the operation (simplified implementation)
			_ = op // In real implementation, would apply JSON patch operations
		case "remove":
			// Remove the specified path
			_ = op
		case "move", "copy":
			// Move or copy operations
			_ = op
		}
	}

	// Acknowledge state delta processing
	responseEvent := events.NewStateSnapshotEvent(map[string]interface{}{
		"status":           "delta_applied",
		"operations_count": len(deltaEvent.Delta),
		"processed_at":     time.Now(),
	})

	return []events.Event{responseEvent}, nil
}

// processMessagesSnapshot handles MESSAGES_SNAPSHOT events
func (a *BaseAgent) processMessagesSnapshot(ctx context.Context, event events.Event) ([]events.Event, error) {
	msgEvent, ok := event.(*events.MessagesSnapshotEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to MessagesSnapshotEvent",
			a.name,
		)
	}

	// Process message snapshot
	messageCount := len(msgEvent.Messages)
	
	// Generate summary response
	responseEvent := events.NewCustomEvent(
		"message_snapshot_processed",
		events.WithValue(map[string]interface{}{
			"message_count": messageCount,
			"processed_by":  a.name,
			"processed_at":  time.Now(),
		}),
	)

	return []events.Event{responseEvent}, nil
}

// processRawEvent handles RAW events
func (a *BaseAgent) processRawEvent(ctx context.Context, event events.Event) ([]events.Event, error) {
	rawEvent, ok := event.(*events.RawEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to RawEvent",
			a.name,
		)
	}

	// Pass through raw events with processing metadata
	responseEvent := events.NewRawEvent(
		map[string]interface{}{
			"original_event": rawEvent.Event,
			"processed_by":   a.name,
			"processed_at":   time.Now(),
		},
		events.WithSource(fmt.Sprintf("agent:%s", a.name)),
	)

	return []events.Event{responseEvent}, nil
}

// processCustomEvent handles CUSTOM events
func (a *BaseAgent) processCustomEvent(ctx context.Context, event events.Event) ([]events.Event, error) {
	customEvent, ok := event.(*events.CustomEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to CustomEvent",
			a.name,
		)
	}

	// Process custom event based on name
	switch customEvent.Name {
	case "health_check":
		health := a.Health()
		responseEvent := events.NewCustomEvent(
			"health_response",
			events.WithValue(health),
		)
		return []events.Event{responseEvent}, nil

	case "metrics_request":
		metrics := map[string]interface{}{
			"events_processed":         a.getEventsProcessed(),
			"error_count":             a.getErrorCount(),
			"tools_executed":          atomic.LoadInt64(&a.metrics.ToolsExecuted),
			"state_updates":           atomic.LoadInt64(&a.metrics.StateUpdates),
			"average_processing_time": a.metrics.AverageProcessingTime.String(),
		}
		responseEvent := events.NewCustomEvent(
			"metrics_response",
			events.WithValue(metrics),
		)
		return []events.Event{responseEvent}, nil

	default:
		// Echo custom event with processing info
		responseEvent := events.NewCustomEvent(
			fmt.Sprintf("%s_processed", customEvent.Name),
			events.WithValue(map[string]interface{}{
				"original_value": customEvent.Value,
				"processed_by":   a.name,
				"processed_at":   time.Now(),
			}),
		)
		return []events.Event{responseEvent}, nil
	}
}

// processRunStarted handles RUN_STARTED events
func (a *BaseAgent) processRunStarted(ctx context.Context, event events.Event) ([]events.Event, error) {
	runEvent, ok := event.(*events.RunStartedEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to RunStartedEvent",
			a.name,
		)
	}

	// Acknowledge run start
	responseEvent := events.NewStepStartedEvent(
		fmt.Sprintf("processing_run_%s", runEvent.RunID()),
	)

	return []events.Event{responseEvent}, nil
}

// processRunFinished handles RUN_FINISHED events
func (a *BaseAgent) processRunFinished(ctx context.Context, event events.Event) ([]events.Event, error) {
	runEvent, ok := event.(*events.RunFinishedEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to RunFinishedEvent",
			a.name,
		)
	}

	// Acknowledge run completion
	responseEvent := events.NewStepFinishedEvent(
		fmt.Sprintf("processing_run_%s", runEvent.RunID()),
	)

	return []events.Event{responseEvent}, nil
}

// processRunError handles RUN_ERROR events
func (a *BaseAgent) processRunError(ctx context.Context, event events.Event) ([]events.Event, error) {
	errorEvent, ok := event.(*events.RunErrorEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to RunErrorEvent",
			a.name,
		)
	}

	a.incrementErrorCount()

	// Log error and generate recovery response
	responseEvent := events.NewCustomEvent(
		"error_handled",
		events.WithValue(map[string]interface{}{
			"original_error": errorEvent.Message,
			"error_code":     errorEvent.Code,
			"run_id":         errorEvent.RunID(),
			"handled_by":     a.name,
			"handled_at":     time.Now(),
		}),
	)

	return []events.Event{responseEvent}, nil
}

// processStepStarted handles STEP_STARTED events
func (a *BaseAgent) processStepStarted(ctx context.Context, event events.Event) ([]events.Event, error) {
	stepEvent, ok := event.(*events.StepStartedEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to StepStartedEvent",
			a.name,
		)
	}

	// Acknowledge step start and begin processing
	responseEvent := events.NewStepStartedEvent(
		fmt.Sprintf("%s_processing", stepEvent.StepName),
	)

	return []events.Event{responseEvent}, nil
}

// processStepFinished handles STEP_FINISHED events
func (a *BaseAgent) processStepFinished(ctx context.Context, event events.Event) ([]events.Event, error) {
	stepEvent, ok := event.(*events.StepFinishedEvent)
	if !ok {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"failed to cast event to StepFinishedEvent",
			a.name,
		)
	}

	// Complete step processing
	responseEvent := events.NewStepFinishedEvent(
		fmt.Sprintf("%s_processing", stepEvent.StepName),
	)

	return []events.Event{responseEvent}, nil
}

// emitEventsToStream sends events to the agent's event stream
func (a *BaseAgent) emitEventsToStream(events []events.Event) {
	a.streamMu.RLock()
	defer a.streamMu.RUnlock()

	if a.eventStream == nil {
		return
	}

	for _, event := range events {
		select {
		case a.eventStream <- event:
			// Successfully sent event
		default:
			// Channel is full, skip this event to avoid blocking
			// In production, you might want to log this or handle it differently
		}
	}
}

// StreamEvents returns a channel for receiving events from the agent.
func (a *BaseAgent) StreamEvents(ctx context.Context) (<-chan events.Event, error) {
	if a.getStatus() != AgentStatusRunning {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			fmt.Sprintf("agent %s is not running", a.name),
			a.name,
		)
	}
	
	if !a.config.Capabilities.Streaming {
		return nil, errors.NewAgentError(
			errors.ErrorTypeUnsupported,
			fmt.Sprintf("agent %s does not support streaming", a.name),
			a.name,
		)
	}
	
	return a.eventStream, nil
}

// GetState returns the current state of the agent.
func (a *BaseAgent) GetState(ctx context.Context) (interface{}, error) {
	// Simplified implementation
	return map[string]interface{}{
		"status": a.getStatus(),
		"name":   a.name,
	}, nil
}

// UpdateState applies a state change delta to the agent's state.
func (a *BaseAgent) UpdateState(ctx context.Context, delta interface{}) error {
	a.incrementStateUpdates()
	return nil
}

// ExecuteTool executes a tool with the given name and parameters.
func (a *BaseAgent) ExecuteTool(ctx context.Context, name string, params interface{}) (interface{}, error) {
	a.incrementToolsExecuted()
	
	// Check if agent is running
	if a.getStatus() != AgentStatusRunning {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			fmt.Sprintf("agent %s is not running", a.name),
			a.name,
		)
	}
	
	// Get the tool execution framework
	framework := a.getToolExecutionFramework()
	if framework == nil {
		return nil, errors.NewAgentError(
			errors.ErrorTypeNotFound,
			"tool execution framework not initialized",
			a.name,
		)
	}
	
	// Convert params to map[string]interface{} if needed
	var paramsMap map[string]interface{}
	switch p := params.(type) {
	case map[string]interface{}:
		paramsMap = p
	case nil:
		paramsMap = make(map[string]interface{})
	default:
		// Try to marshal and unmarshal to convert to map
		data, err := json.Marshal(params)
		if err != nil {
			return nil, errors.NewAgentError(
				errors.ErrorTypeValidation,
				fmt.Sprintf("failed to serialize parameters: %v", err),
				a.name,
			)
		}
		if err := json.Unmarshal(data, &paramsMap); err != nil {
			return nil, errors.NewAgentError(
				errors.ErrorTypeValidation,
				fmt.Sprintf("failed to deserialize parameters: %v", err),
				a.name,
			)
		}
	}
	
	// Create execution context with timeout from tools config
	timeout := a.config.Tools.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second // default timeout
	}
	
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Execute the tool
	result, err := framework.Execute(execCtx, name, paramsMap)
	if err != nil {
		a.incrementErrorCount()
		return nil, errors.NewAgentError(
			errors.ErrorTypeValidation,
			fmt.Sprintf("tool execution failed: %v", err),
			a.name,
		)
	}
	
	// Return the result data
	if result != nil && result.Success {
		return result.Data, nil
	}
	
	// Tool execution failed
	if result != nil && result.Error != "" {
		a.incrementErrorCount()
		return nil, errors.NewAgentError(
			errors.ErrorTypeValidation,
			fmt.Sprintf("tool execution error: %s", result.Error),
			a.name,
		)
	}
	
	// Unknown error
	a.incrementErrorCount()
	return nil, errors.NewAgentError(
		errors.ErrorTypeValidation,
		"tool execution failed with unknown error",
		a.name,
	)
}

// ListTools returns a list of tools available to this agent.
func (a *BaseAgent) ListTools() []ToolDefinition {
	framework := a.getToolExecutionFramework()
	if framework == nil {
		return []ToolDefinition{}
	}
	
	registry := framework.GetRegistry()
	if registry == nil {
		return []ToolDefinition{}
	}
	
	// Get all tools from registry
	allTools, err := registry.ListAll()
	if err != nil {
		return []ToolDefinition{}
	}
	toolDefs := make([]ToolDefinition, 0, len(allTools))
	
	for _, tool := range allTools {
		toolView := tools.NewReadOnlyTool(tool)
		toolDef := ToolDefinition{
			Name:        toolView.GetName(),
			Description: toolView.GetDescription(),
			Schema:      toolView.GetSchema(),
		}
		
		// Add capabilities if available
		if capabilities := toolView.GetCapabilities(); capabilities != nil {
			toolDef.Capabilities = map[string]interface{}{
				"streaming":  capabilities.Streaming,
				"async":      capabilities.Async,
				"cancelable": capabilities.Cancelable,
				"retryable":  capabilities.Retryable,
				"cacheable":  capabilities.Cacheable,
				"timeout":    capabilities.Timeout.String(),
			}
			if capabilities.RateLimit > 0 {
				toolDef.Capabilities["rateLimit"] = capabilities.RateLimit
			}
		}
		
		toolDefs = append(toolDefs, toolDef)
	}
	
	return toolDefs
}

// Name returns the unique identifier for this agent instance.
func (a *BaseAgent) Name() string {
	return a.name
}

// Description returns a human-readable description of the agent's purpose.
func (a *BaseAgent) Description() string {
	return a.desc
}

// Capabilities returns information about what this agent can do.
func (a *BaseAgent) Capabilities() AgentCapabilities {
	if a.config == nil {
		return AgentCapabilities{}
	}
	return a.config.Capabilities
}

// Health returns the current health status of the agent.
func (a *BaseAgent) Health() HealthStatus {
	a.healthMu.RLock()
	defer a.healthMu.RUnlock()
	
	// Update health details with current metrics
	health := a.healthStatus
	health.Details["status"] = a.getStatus()
	health.Details["uptime"] = time.Since(a.startTime).String()
	health.Details["events_processed"] = a.getEventsProcessed()
	health.Details["error_count"] = a.getErrorCount()
	
	return health
}

// Helper methods

func (a *BaseAgent) getStatus() AgentStatus {
	return a.status.Load().(AgentStatus)
}

func (a *BaseAgent) setStatus(status AgentStatus) {
	a.status.Store(status)
}

func (a *BaseAgent) updateHealth(status string, errors []string) {
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	
	a.healthStatus.Status = status
	a.healthStatus.LastCheck = time.Now()
	if errors != nil {
		a.healthStatus.Errors = errors
	}
}

func (a *BaseAgent) incrementEventsProcessed() {
	atomic.AddInt64(&a.metrics.EventsProcessed, 1)
}

func (a *BaseAgent) incrementErrorCount() {
	atomic.AddInt64(&a.metrics.ErrorCount, 1)
}

func (a *BaseAgent) incrementStateUpdates() {
	atomic.AddInt64(&a.metrics.StateUpdates, 1)
}

func (a *BaseAgent) incrementToolsExecuted() {
	atomic.AddInt64(&a.metrics.ToolsExecuted, 1)
}

func (a *BaseAgent) getEventsProcessed() int64 {
	return atomic.LoadInt64(&a.metrics.EventsProcessed)
}

func (a *BaseAgent) getErrorCount() int64 {
	return atomic.LoadInt64(&a.metrics.ErrorCount)
}

func (a *BaseAgent) updateAverageProcessingTime(duration time.Duration) {
	a.metricsMu.Lock()
	defer a.metricsMu.Unlock()
	
	if a.metrics.AverageProcessingTime == 0 {
		a.metrics.AverageProcessingTime = duration
	} else {
		a.metrics.AverageProcessingTime = (a.metrics.AverageProcessingTime + duration) / 2
	}
}

func (a *BaseAgent) updateLastActivity() {
	a.metricsMu.Lock()
	defer a.metricsMu.Unlock()
	a.metrics.LastActivity = time.Now()
}

func (a *BaseAgent) validateConfig(config *AgentConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	
	if config.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	
	if config.EventProcessing.BufferSize <= 0 {
		return fmt.Errorf("event processing buffer size must be positive")
	}
	
	if config.EventProcessing.BatchSize <= 0 {
		return fmt.Errorf("event processing batch size must be positive")
	}
	
	if config.Tools.MaxConcurrent <= 0 {
		return fmt.Errorf("tool max concurrent must be positive")
	}
	
	if config.History.MaxMessages <= 0 {
		return fmt.Errorf("history max messages must be positive")
	}
	
	return nil
}

func (a *BaseAgent) mergeWithDefaults(config *AgentConfig) *AgentConfig {
	defaults := DefaultAgentConfig()
	
	// Merge configuration with defaults
	merged := *config
	
	if merged.EventProcessing.BufferSize == 0 {
		merged.EventProcessing.BufferSize = defaults.EventProcessing.BufferSize
	}
	
	if merged.EventProcessing.BatchSize == 0 {
		merged.EventProcessing.BatchSize = defaults.EventProcessing.BatchSize
	}
	
	if merged.EventProcessing.Timeout == 0 {
		merged.EventProcessing.Timeout = defaults.EventProcessing.Timeout
	}
	
	if merged.State.SyncInterval == 0 {
		merged.State.SyncInterval = defaults.State.SyncInterval
	}
	
	if merged.State.CacheSize == "" {
		merged.State.CacheSize = defaults.State.CacheSize
	}
	
	if merged.Tools.Timeout == 0 {
		merged.Tools.Timeout = defaults.Tools.Timeout
	}
	
	if merged.Tools.MaxConcurrent == 0 {
		merged.Tools.MaxConcurrent = defaults.Tools.MaxConcurrent
	}
	
	if merged.History.MaxMessages == 0 {
		merged.History.MaxMessages = defaults.History.MaxMessages
	}
	
	if merged.History.Retention == 0 {
		merged.History.Retention = defaults.History.Retention
	}
	
	if merged.Custom == nil {
		merged.Custom = make(map[string]interface{})
	}
	
	return &merged
}

// ToolExecutionFramework manages tool execution for the base agent.
// It provides a complete tool execution system with validation, timeout management,
// concurrency control, and error handling.
type ToolExecutionFramework struct {
	registry        *tools.Registry
	executionEngine *tools.ExecutionEngine
	config          *ToolsConfig
	mu              sync.RWMutex
	
	// Custom tool registry for agent-specific tools
	customTools map[string]*tools.Tool
	customMu    sync.RWMutex
}

// NewToolExecutionFramework creates a new tool execution framework.
func NewToolExecutionFramework(config *ToolsConfig) *ToolExecutionFramework {
	// Create tool registry
	registry := tools.NewRegistry()
	
	// Register built-in tools
	if err := tools.RegisterBuiltinTools(registry); err != nil {
		// Log error but continue - built-in tools are optional
		fmt.Printf("Warning: failed to register built-in tools: %v\n", err)
	}
	
	// Create execution engine with configuration
	var opts []tools.ExecutionEngineOption
	if config != nil {
		if config.MaxConcurrent > 0 {
			opts = append(opts, tools.WithMaxConcurrent(config.MaxConcurrent))
		}
		if config.Timeout > 0 {
			opts = append(opts, tools.WithDefaultTimeout(config.Timeout))
		}
		if config.EnableCaching {
			// Enable caching with 1000 entries and 1 hour TTL
			opts = append(opts, tools.WithCaching(1000, time.Hour))
		}
	}
	
	executionEngine := tools.NewExecutionEngine(registry, opts...)
	
	return &ToolExecutionFramework{
		registry:        registry,
		executionEngine: executionEngine,
		config:          config,
		customTools:     make(map[string]*tools.Tool),
	}
}

// Execute executes a tool with the given name and parameters.
func (f *ToolExecutionFramework) Execute(ctx context.Context, name string, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	return f.executionEngine.Execute(ctx, name, params)
}

// ExecuteStream executes a streaming tool with the given name and parameters.
func (f *ToolExecutionFramework) ExecuteStream(ctx context.Context, name string, params map[string]interface{}) (<-chan *tools.ToolStreamChunk, error) {
	return f.executionEngine.ExecuteStream(ctx, name, params)
}

// RegisterTool registers a custom tool for this agent.
func (f *ToolExecutionFramework) RegisterTool(tool *tools.Tool) error {
	f.customMu.Lock()
	defer f.customMu.Unlock()
	
	// Validate the tool first
	if err := tool.Validate(); err != nil {
		return fmt.Errorf("tool validation failed: %w", err)
	}
	
	// Register in the main registry
	if err := f.registry.Register(tool); err != nil {
		return fmt.Errorf("failed to register tool in registry: %w", err)
	}
	
	// Store in custom tools map for tracking
	f.customTools[tool.ID] = tool
	
	return nil
}

// UnregisterTool removes a custom tool from this agent.
func (f *ToolExecutionFramework) UnregisterTool(toolID string) error {
	f.customMu.Lock()
	defer f.customMu.Unlock()
	
	// Remove from registry
	if err := f.registry.Unregister(toolID); err != nil {
		return fmt.Errorf("failed to unregister tool from registry: %w", err)
	}
	
	// Remove from custom tools map
	delete(f.customTools, toolID)
	
	return nil
}

// GetTool gets a tool by ID.
func (f *ToolExecutionFramework) GetTool(toolID string) (tools.ReadOnlyTool, error) {
	return f.registry.GetReadOnly(toolID)
}

// ListTools lists all available tools.
func (f *ToolExecutionFramework) ListTools() []tools.ReadOnlyTool {
	allTools, err := f.registry.ListAll()
	if err != nil {
		return []tools.ReadOnlyTool{}
	}
	
	// Convert to ReadOnlyTool views
	readOnlyTools := make([]tools.ReadOnlyTool, 0, len(allTools))
	for _, tool := range allTools {
		readOnlyTools = append(readOnlyTools, tools.NewReadOnlyTool(tool))
	}
	return readOnlyTools
}

// GetRegistry returns the tool registry.
func (f *ToolExecutionFramework) GetRegistry() *tools.Registry {
	return f.registry
}

// GetExecutionEngine returns the execution engine.
func (f *ToolExecutionFramework) GetExecutionEngine() *tools.ExecutionEngine {
	return f.executionEngine
}

// GetMetrics returns execution metrics.
func (f *ToolExecutionFramework) GetMetrics() *tools.ExecutionMetrics {
	return f.executionEngine.GetMetrics()
}

// CancelAll cancels all active tool executions.
func (f *ToolExecutionFramework) CancelAll() {
	f.executionEngine.CancelAll()
}

// Shutdown gracefully shuts down the tool execution framework.
func (f *ToolExecutionFramework) Shutdown(ctx context.Context) error {
	return f.executionEngine.Shutdown(ctx)
}

// getToolExecutionFramework returns the tool execution framework, initializing it if needed.
func (a *BaseAgent) getToolExecutionFramework() *ToolExecutionFramework {
	a.toolMu.RLock()
	framework := a.toolFramework
	a.toolMu.RUnlock()
	
	if framework != nil {
		return framework
	}
	
	// Initialize framework with double-checked locking
	a.toolMu.Lock()
	defer a.toolMu.Unlock()
	
	// Check again after acquiring write lock
	if a.toolFramework != nil {
		return a.toolFramework
	}
	
	// Initialize with agent's tools config
	var config *ToolsConfig
	if a.config != nil {
		config = &a.config.Tools
	}
	
	a.toolFramework = NewToolExecutionFramework(config)
	return a.toolFramework
}

// RegisterCustomTool registers a custom tool for this agent.
// This allows specific agent implementations to add their own tools.
func (a *BaseAgent) RegisterCustomTool(tool *tools.Tool) error {
	framework := a.getToolExecutionFramework()
	if framework == nil {
		return errors.NewAgentError(
			errors.ErrorTypeNotFound,
			"tool execution framework not initialized",
			a.name,
		)
	}
	
	return framework.RegisterTool(tool)
}

// UnregisterCustomTool removes a custom tool from this agent.
func (a *BaseAgent) UnregisterCustomTool(toolID string) error {
	framework := a.getToolExecutionFramework()
	if framework == nil {
		return errors.NewAgentError(
			errors.ErrorTypeNotFound,
			"tool execution framework not initialized",
			a.name,
		)
	}
	
	return framework.UnregisterTool(toolID)
}

// GetToolMetrics returns tool execution metrics for this agent.
func (a *BaseAgent) GetToolMetrics() *tools.ExecutionMetrics {
	framework := a.getToolExecutionFramework()
	if framework == nil {
		return nil
	}
	
	return framework.GetMetrics()
}

// ExecuteToolAsync executes a tool asynchronously and returns a job ID and result channel.
func (a *BaseAgent) ExecuteToolAsync(ctx context.Context, name string, params interface{}, priority int) (string, <-chan *tools.AsyncResult, error) {
	// Check if agent is running
	if a.getStatus() != AgentStatusRunning {
		return "", nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			fmt.Sprintf("agent %s is not running", a.name),
			a.name,
		)
	}
	
	// Get the tool execution framework
	framework := a.getToolExecutionFramework()
	if framework == nil {
		return "", nil, errors.NewAgentError(
			errors.ErrorTypeNotFound,
			"tool execution framework not initialized",
			a.name,
		)
	}
	
	// Convert params to map[string]interface{} if needed
	var paramsMap map[string]interface{}
	switch p := params.(type) {
	case map[string]interface{}:
		paramsMap = p
	case nil:
		paramsMap = make(map[string]interface{})
	default:
		// Try to marshal and unmarshal to convert to map
		data, err := json.Marshal(params)
		if err != nil {
			return "", nil, errors.NewAgentError(
				errors.ErrorTypeValidation,
				fmt.Sprintf("failed to serialize parameters: %v", err),
				a.name,
			)
		}
		if err := json.Unmarshal(data, &paramsMap); err != nil {
			return "", nil, errors.NewAgentError(
				errors.ErrorTypeValidation,
				fmt.Sprintf("failed to deserialize parameters: %v", err),
				a.name,
			)
		}
	}
	
	// Execute asynchronously using the execution engine
	return framework.GetExecutionEngine().ExecuteAsync(ctx, name, paramsMap, priority)
}

// ExecuteToolStream executes a streaming tool and returns a channel of stream chunks.
func (a *BaseAgent) ExecuteToolStream(ctx context.Context, name string, params interface{}) (<-chan *tools.ToolStreamChunk, error) {
	// Check if agent is running
	if a.getStatus() != AgentStatusRunning {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			fmt.Sprintf("agent %s is not running", a.name),
			a.name,
		)
	}
	
	// Get the tool execution framework
	framework := a.getToolExecutionFramework()
	if framework == nil {
		return nil, errors.NewAgentError(
			errors.ErrorTypeNotFound,
			"tool execution framework not initialized",
			a.name,
		)
	}
	
	// Convert params to map[string]interface{} if needed
	var paramsMap map[string]interface{}
	switch p := params.(type) {
	case map[string]interface{}:
		paramsMap = p
	case nil:
		paramsMap = make(map[string]interface{})
	default:
		// Try to marshal and unmarshal to convert to map
		data, err := json.Marshal(params)
		if err != nil {
			return nil, errors.NewAgentError(
				errors.ErrorTypeValidation,
				fmt.Sprintf("failed to serialize parameters: %v", err),
				a.name,
			)
		}
		if err := json.Unmarshal(data, &paramsMap); err != nil {
			return nil, errors.NewAgentError(
				errors.ErrorTypeValidation,
				fmt.Sprintf("failed to deserialize parameters: %v", err),
				a.name,
			)
		}
	}
	
	// Create execution context with timeout from tools config
	timeout := a.config.Tools.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second // default timeout
	}
	
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Execute the streaming tool
	return framework.ExecuteStream(execCtx, name, paramsMap)
}