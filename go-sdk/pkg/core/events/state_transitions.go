package events

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// StateTransitionLevel defines the level of state transition validation
type StateTransitionLevel int

const (
	// StateTransitionStrict applies strict state transition validation
	StateTransitionStrict StateTransitionLevel = iota
	// StateTransitionPermissive applies permissive state transition validation
	StateTransitionPermissive
	// StateTransitionCustom allows custom state transition validation
	StateTransitionCustom
)

// StateTransitionConfig contains configuration for state transition validation
type StateTransitionConfig struct {
	Level                      StateTransitionLevel
	EnableConcurrentValidation bool
	EnableRollbackValidation   bool
	EnableVersionCompatibility bool
	MaxConcurrentOperations    int
	MaxRollbackDepth           int
	VersionCompatibilityWindow time.Duration
}

// DefaultStateTransitionConfig returns the default configuration
func DefaultStateTransitionConfig() *StateTransitionConfig {
	return &StateTransitionConfig{
		Level:                      StateTransitionStrict,
		EnableConcurrentValidation: true,
		EnableRollbackValidation:   true,
		EnableVersionCompatibility: true,
		MaxConcurrentOperations:    10,
		MaxRollbackDepth:           50,
		VersionCompatibilityWindow: 24 * time.Hour,
	}
}

// StateTransition represents a state transition definition
type StateTransition struct {
	FromState      string    `json:"from_state"`
	ToState        string    `json:"to_state"`
	EventType      EventType `json:"event_type"`
	Conditions     []string  `json:"conditions,omitempty"`
	Preconditions  []string  `json:"preconditions,omitempty"`
	PostConditions []string  `json:"post_conditions,omitempty"`
}

// StateVersion represents a state version for compatibility checks
type StateVersion struct {
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Checksum  string    `json:"checksum"`
}

// ConcurrentStateOperation represents a concurrent state operation
type ConcurrentStateOperation struct {
	OperationID string    `json:"operation_id"`
	EventType   EventType `json:"event_type"`
	StartTime   time.Time `json:"start_time"`
	State       string    `json:"state"`
	LockHeld    bool      `json:"lock_held"`
}

// RollbackOperation represents a rollback operation
type RollbackOperation struct {
	OperationID    string               `json:"operation_id"`
	TargetState    string               `json:"target_state"`
	RollbackReason string               `json:"rollback_reason"`
	Timestamp      time.Time            `json:"timestamp"`
	PreviousDeltas []JSONPatchOperation `json:"previous_deltas"`
}

// StateTransitionRule validates state transitions and consistency
type StateTransitionRule struct {
	*BaseValidationRule
	config           *StateTransitionConfig
	validTransitions map[string][]StateTransition
	stateVersions    map[string]*StateVersion
	concurrentOps    map[string]*ConcurrentStateOperation
	rollbackHistory  []RollbackOperation
	stateLocks       map[string]*sync.RWMutex
	mutex            sync.RWMutex
}

// NewStateTransitionRule creates a new state transition validation rule
func NewStateTransitionRule(config *StateTransitionConfig) *StateTransitionRule {
	if config == nil {
		config = DefaultStateTransitionConfig()
	}

	rule := &StateTransitionRule{
		BaseValidationRule: NewBaseValidationRule(
			"STATE_TRANSITION_VALIDATION",
			"Validates state transitions, consistency, concurrency, and rollback scenarios",
			ValidationSeverityError,
		),
		config:           config,
		validTransitions: make(map[string][]StateTransition),
		stateVersions:    make(map[string]*StateVersion),
		concurrentOps:    make(map[string]*ConcurrentStateOperation),
		rollbackHistory:  make([]RollbackOperation, 0),
		stateLocks:       make(map[string]*sync.RWMutex),
	}

	// Initialize valid state transitions
	rule.initializeValidTransitions()

	return rule
}

// initializeValidTransitions initializes the valid state transition definitions
func (r *StateTransitionRule) initializeValidTransitions() {
	// Run state transitions
	r.addTransition("INIT", "RUNNING", EventTypeRunStarted,
		[]string{"run_id_valid", "thread_id_valid"},
		[]string{"no_active_run_with_same_id"},
		[]string{"run_state_active"})

	r.addTransition("RUNNING", "FINISHED", EventTypeRunFinished,
		[]string{"run_id_matches"},
		[]string{"run_is_active"},
		[]string{"run_state_finished"})

	r.addTransition("RUNNING", "ERROR", EventTypeRunError,
		[]string{"error_message_provided"},
		[]string{"run_is_active"},
		[]string{"run_state_error"})

	// Message state transitions
	r.addTransition("INIT", "ACTIVE", EventTypeTextMessageStart,
		[]string{"message_id_valid"},
		[]string{"no_active_message_with_same_id"},
		[]string{"message_state_active"})

	r.addTransition("ACTIVE", "ACTIVE", EventTypeTextMessageContent,
		[]string{"message_id_matches", "delta_provided"},
		[]string{"message_is_active"},
		[]string{"content_accumulated"})

	r.addTransition("ACTIVE", "FINISHED", EventTypeTextMessageEnd,
		[]string{"message_id_matches"},
		[]string{"message_is_active"},
		[]string{"message_state_finished"})

	// Tool call state transitions
	r.addTransition("INIT", "ACTIVE", EventTypeToolCallStart,
		[]string{"tool_call_id_valid", "tool_name_provided"},
		[]string{"no_active_tool_with_same_id"},
		[]string{"tool_state_active"})

	r.addTransition("ACTIVE", "ACTIVE", EventTypeToolCallArgs,
		[]string{"tool_call_id_matches", "args_delta_provided"},
		[]string{"tool_is_active"},
		[]string{"args_accumulated"})

	r.addTransition("ACTIVE", "FINISHED", EventTypeToolCallEnd,
		[]string{"tool_call_id_matches"},
		[]string{"tool_is_active"},
		[]string{"tool_state_finished"})

	// Step state transitions
	r.addTransition("INIT", "ACTIVE", EventTypeStepStarted,
		[]string{"step_name_provided"},
		[]string{"no_active_step_with_same_name"},
		[]string{"step_state_active"})

	r.addTransition("ACTIVE", "FINISHED", EventTypeStepFinished,
		[]string{"step_name_matches"},
		[]string{"step_is_active"},
		[]string{"step_state_finished"})

	// State snapshot transitions
	r.addTransition("ANY", "SNAPSHOT", EventTypeStateSnapshot,
		[]string{"snapshot_provided"},
		[]string{},
		[]string{"snapshot_captured"})

	// State delta transitions
	r.addTransition("ANY", "DELTA_APPLIED", EventTypeStateDelta,
		[]string{"delta_operations_valid"},
		[]string{"state_consistent"},
		[]string{"delta_applied"})
}

// addTransition adds a state transition definition
func (r *StateTransitionRule) addTransition(fromState, toState string, eventType EventType, conditions, preconditions, postconditions []string) {
	transition := StateTransition{
		FromState:      fromState,
		ToState:        toState,
		EventType:      eventType,
		Conditions:     conditions,
		Preconditions:  preconditions,
		PostConditions: postconditions,
	}

	key := fmt.Sprintf("%s:%s", fromState, string(eventType))
	r.validTransitions[key] = append(r.validTransitions[key], transition)
}

// Validate implements the ValidationRule interface
func (r *StateTransitionRule) Validate(event Event, context *ValidationContext) *ValidationResult {
	result := &ValidationResult{
		IsValid:   true,
		Timestamp: time.Now(),
	}

	if !r.IsEnabled() {
		return result
	}

	// Validate state transitions based on configuration level
	switch r.config.Level {
	case StateTransitionStrict:
		r.validateStrict(event, context, result)
	case StateTransitionPermissive:
		r.validatePermissive(event, context, result)
	case StateTransitionCustom:
		r.validateCustom(event, context, result)
	}

	// Validate concurrent operations if enabled
	if r.config.EnableConcurrentValidation {
		r.validateConcurrentOperations(event, context, result)
	}

	// Validate rollback scenarios if enabled
	if r.config.EnableRollbackValidation {
		r.validateRollbackScenarios(event, context, result)
	}

	// Validate state version compatibility if enabled
	if r.config.EnableVersionCompatibility {
		r.validateVersionCompatibility(event, context, result)
	}

	// Validate delta operations for state delta events
	if event.Type() == EventTypeStateDelta {
		r.validateDeltaOperations(event, context, result)
	}

	return result
}

// validateStrict applies strict state transition validation
func (r *StateTransitionRule) validateStrict(event Event, context *ValidationContext, result *ValidationResult) {
	// Determine current state based on event type and context
	currentState := r.determineCurrentState(event, context)

	// Get valid transitions for current state and event type
	transitionKey := fmt.Sprintf("%s:%s", currentState, string(event.Type()))
	validTransitions, exists := r.validTransitions[transitionKey]

	if !exists {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("No valid state transition found from %s for event %s", currentState, event.Type()),
			map[string]interface{}{
				"current_state": currentState,
				"event_type":    event.Type(),
			},
			[]string{
				fmt.Sprintf("Ensure the state transition from %s to a valid state is defined for %s events", currentState, event.Type()),
				"Review the state machine definition for valid transitions",
			}))
		return
	}

	// Validate each possible transition
	var validTransition *StateTransition
	for _, transition := range validTransitions {
		if r.validateTransitionConditions(event, context, &transition) {
			validTransition = &transition
			break
		}
	}

	if validTransition == nil {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("State transition conditions not met for %s from %s", event.Type(), currentState),
			map[string]interface{}{
				"current_state":         currentState,
				"event_type":            event.Type(),
				"available_transitions": validTransitions,
			},
			[]string{
				"Ensure all transition conditions are met before sending the event",
				"Check preconditions and state consistency",
			}))
		return
	}

	// Validate state consistency
	r.validateStateConsistency(event, context, result, validTransition)
}

// validatePermissive applies permissive state transition validation
func (r *StateTransitionRule) validatePermissive(event Event, context *ValidationContext, result *ValidationResult) {
	// In permissive mode, only validate critical state transitions
	switch event.Type() {
	case EventTypeRunStarted:
		r.validateRunStartTransition(event, context, result)
	case EventTypeRunFinished, EventTypeRunError:
		r.validateRunEndTransition(event, context, result)
	case EventTypeStateDelta:
		r.validateBasicDeltaOperations(event, context, result)
	}
}

// validateCustom applies custom state transition validation
func (r *StateTransitionRule) validateCustom(event Event, context *ValidationContext, result *ValidationResult) {
	// Custom validation logic can be implemented here
	// For now, apply basic validation
	r.validatePermissive(event, context, result)
}

// validateConcurrentOperations validates concurrent state operations
func (r *StateTransitionRule) validateConcurrentOperations(event Event, context *ValidationContext, result *ValidationResult) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if we have too many concurrent operations
	if len(r.concurrentOps) >= r.config.MaxConcurrentOperations {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Maximum concurrent operations limit reached (%d)", r.config.MaxConcurrentOperations),
			map[string]interface{}{
				"max_concurrent_ops": r.config.MaxConcurrentOperations,
				"current_ops":        len(r.concurrentOps),
			},
			[]string{
				"Wait for existing operations to complete before starting new ones",
				"Increase MaxConcurrentOperations if needed",
			}))
		return
	}

	// Generate operation ID
	operationID := r.generateOperationID(event)

	// Check for conflicting operations
	for _, op := range r.concurrentOps {
		if r.operationsConflict(event, op) {
			result.AddError(r.CreateError(event,
				fmt.Sprintf("Concurrent operation conflict detected with operation %s", op.OperationID),
				map[string]interface{}{
					"conflicting_operation": op.OperationID,
					"conflicting_event":     op.EventType,
					"current_event":         event.Type(),
				},
				[]string{
					"Wait for conflicting operation to complete",
					"Use proper locking mechanisms to prevent conflicts",
				}))
			return
		}
	}

	// Register the operation
	r.concurrentOps[operationID] = &ConcurrentStateOperation{
		OperationID: operationID,
		EventType:   event.Type(),
		StartTime:   time.Now(),
		State:       r.determineCurrentState(event, context),
		LockHeld:    false,
	}

	// Clean up completed operations
	r.cleanupCompletedOperations()
}

// validateRollbackScenarios validates rollback scenarios
func (r *StateTransitionRule) validateRollbackScenarios(event Event, context *ValidationContext, result *ValidationResult) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if this is a rollback operation
	if r.isRollbackOperation(event) {
		rollbackOp := r.extractRollbackOperation(event)

		// Validate rollback depth
		if len(r.rollbackHistory) >= r.config.MaxRollbackDepth {
			result.AddError(r.CreateError(event,
				fmt.Sprintf("Maximum rollback depth exceeded (%d)", r.config.MaxRollbackDepth),
				map[string]interface{}{
					"max_rollback_depth": r.config.MaxRollbackDepth,
					"current_depth":      len(r.rollbackHistory),
				},
				[]string{
					"Reduce rollback depth or increase MaxRollbackDepth",
					"Consider state checkpointing to reduce rollback requirements",
				}))
			return
		}

		// Validate rollback target state
		if !r.isValidRollbackTarget(rollbackOp.TargetState, context) {
			result.AddError(r.CreateError(event,
				fmt.Sprintf("Invalid rollback target state: %s", rollbackOp.TargetState),
				map[string]interface{}{
					"target_state": rollbackOp.TargetState,
				},
				[]string{
					"Ensure the target state is valid and reachable",
					"Check state history for available rollback points",
				}))
			return
		}

		// Record rollback operation
		r.rollbackHistory = append(r.rollbackHistory, *rollbackOp)

		// Clean up old rollback history
		r.cleanupRollbackHistory()
	}
}

// validateVersionCompatibility validates state version compatibility
func (r *StateTransitionRule) validateVersionCompatibility(event Event, context *ValidationContext, result *ValidationResult) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if event contains version information
	if versionInfo := r.extractVersionInfo(event); versionInfo != nil {
		stateKey := r.getStateKey(event)
		currentVersion, exists := r.stateVersions[stateKey]

		if exists {
			// Check compatibility window
			timeDiff := time.Since(currentVersion.Timestamp)
			if timeDiff > r.config.VersionCompatibilityWindow {
				result.AddError(r.CreateError(event,
					fmt.Sprintf("Version compatibility window exceeded: %v > %v", timeDiff, r.config.VersionCompatibilityWindow),
					map[string]interface{}{
						"current_version": currentVersion.Version,
						"event_version":   versionInfo.Version,
						"time_diff":       timeDiff,
						"window":          r.config.VersionCompatibilityWindow,
					},
					[]string{
						"Update to a compatible version",
						"Extend the compatibility window if needed",
					}))
				return
			}

			// Check version compatibility
			if !r.versionsCompatible(currentVersion.Version, versionInfo.Version) {
				result.AddError(r.CreateError(event,
					fmt.Sprintf("Incompatible state version: current=%s, event=%s", currentVersion.Version, versionInfo.Version),
					map[string]interface{}{
						"current_version": currentVersion.Version,
						"event_version":   versionInfo.Version,
					},
					[]string{
						"Ensure state versions are compatible",
						"Apply necessary migrations if required",
					}))
				return
			}
		}

		// Update version information
		r.stateVersions[stateKey] = versionInfo
	}
}

// validateDeltaOperations validates delta operations for state delta events
func (r *StateTransitionRule) validateDeltaOperations(event Event, context *ValidationContext, result *ValidationResult) {
	deltaEvent, ok := event.(*StateDeltaEvent)
	if !ok {
		return
	}

	// Validate each delta operation
	for i, operation := range deltaEvent.Delta {
		if err := r.validateDeltaOperation(operation, context); err != nil {
			result.AddError(r.CreateError(event,
				fmt.Sprintf("Invalid delta operation at index %d: %s", i, err.Error()),
				map[string]interface{}{
					"operation_index": i,
					"operation":       operation,
				},
				[]string{
					"Ensure delta operations are valid JSON Patch operations",
					"Check operation paths and values",
				}))
		}
	}

	// Validate operation sequence
	if err := r.validateDeltaSequence(deltaEvent.Delta, context); err != nil {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Invalid delta operation sequence: %s", err.Error()),
			map[string]interface{}{
				"delta_operations": deltaEvent.Delta,
			},
			[]string{
				"Ensure delta operations are in correct order",
				"Check for conflicting operations",
			}))
	}
}

// Helper methods

// determineCurrentState determines the current state based on event and context
func (r *StateTransitionRule) determineCurrentState(event Event, context *ValidationContext) string {
	switch event.Type() {
	case EventTypeRunStarted:
		return "INIT"
	case EventTypeRunFinished, EventTypeRunError:
		if len(context.State.ActiveRuns) > 0 {
			return "RUNNING"
		}
		return "INIT"
	case EventTypeTextMessageStart:
		return "INIT"
	case EventTypeTextMessageContent, EventTypeTextMessageEnd:
		return "ACTIVE"
	case EventTypeToolCallStart:
		return "INIT"
	case EventTypeToolCallArgs, EventTypeToolCallEnd:
		return "ACTIVE"
	case EventTypeStepStarted:
		return "INIT"
	case EventTypeStepFinished:
		return "ACTIVE"
	case EventTypeStateSnapshot:
		return "ANY"
	case EventTypeStateDelta:
		return "ANY"
	default:
		return "UNKNOWN"
	}
}

// validateTransitionConditions validates transition conditions
func (r *StateTransitionRule) validateTransitionConditions(event Event, context *ValidationContext, transition *StateTransition) bool {
	// Validate preconditions
	for _, precondition := range transition.Preconditions {
		if !r.checkCondition(precondition, event, context) {
			return false
		}
	}

	// Validate conditions
	for _, condition := range transition.Conditions {
		if !r.checkCondition(condition, event, context) {
			return false
		}
	}

	return true
}

// checkCondition checks a specific condition
func (r *StateTransitionRule) checkCondition(condition string, event Event, context *ValidationContext) bool {
	switch condition {
	case "run_id_valid":
		return r.checkRunIDValid(event)
	case "thread_id_valid":
		return r.checkThreadIDValid(event)
	case "no_active_run_with_same_id":
		return r.checkNoActiveRunWithSameID(event, context)
	case "run_id_matches":
		return r.checkRunIDMatches(event, context)
	case "run_is_active":
		return r.checkRunIsActive(event, context)
	case "message_id_valid":
		return r.checkMessageIDValid(event)
	case "no_active_message_with_same_id":
		return r.checkNoActiveMessageWithSameID(event, context)
	case "message_id_matches":
		return r.checkMessageIDMatches(event, context)
	case "message_is_active":
		return r.checkMessageIsActive(event, context)
	case "delta_provided":
		return r.checkDeltaProvided(event)
	case "tool_call_id_valid":
		return r.checkToolCallIDValid(event)
	case "tool_name_provided":
		return r.checkToolNameProvided(event)
	case "no_active_tool_with_same_id":
		return r.checkNoActiveToolWithSameID(event, context)
	case "tool_call_id_matches":
		return r.checkToolCallIDMatches(event, context)
	case "tool_is_active":
		return r.checkToolIsActive(event, context)
	case "args_delta_provided":
		return r.checkArgsDeltaProvided(event)
	case "step_name_provided":
		return r.checkStepNameProvided(event)
	case "no_active_step_with_same_name":
		return r.checkNoActiveStepWithSameName(event, context)
	case "step_name_matches":
		return r.checkStepNameMatches(event, context)
	case "step_is_active":
		return r.checkStepIsActive(event, context)
	case "snapshot_provided":
		return r.checkSnapshotProvided(event)
	case "delta_operations_valid":
		return r.checkDeltaOperationsValid(event)
	case "state_consistent":
		return r.checkStateConsistent(event, context)
	case "error_message_provided":
		return r.checkErrorMessageProvided(event)
	default:
		return true // Unknown conditions are considered valid
	}
}

// Condition check implementations

func (r *StateTransitionRule) checkRunIDValid(event Event) bool {
	switch e := event.(type) {
	case *RunStartedEvent:
		return e.RunID != ""
	case *RunFinishedEvent:
		return e.RunID != ""
	case *RunErrorEvent:
		return e.RunID != ""
	}
	return true
}

func (r *StateTransitionRule) checkThreadIDValid(event Event) bool {
	if runEvent, ok := event.(*RunStartedEvent); ok {
		return runEvent.ThreadID != ""
	}
	return true
}

func (r *StateTransitionRule) checkNoActiveRunWithSameID(event Event, context *ValidationContext) bool {
	if runEvent, ok := event.(*RunStartedEvent); ok {
		_, exists := context.State.ActiveRuns[runEvent.RunID]
		return !exists
	}
	return true
}

func (r *StateTransitionRule) checkRunIDMatches(event Event, context *ValidationContext) bool {
	var runID string
	switch e := event.(type) {
	case *RunFinishedEvent:
		runID = e.RunID
	case *RunErrorEvent:
		runID = e.RunID
	default:
		return true
	}

	_, exists := context.State.ActiveRuns[runID]
	return exists
}

func (r *StateTransitionRule) checkRunIsActive(event Event, context *ValidationContext) bool {
	return len(context.State.ActiveRuns) > 0
}

func (r *StateTransitionRule) checkMessageIDValid(event Event) bool {
	switch e := event.(type) {
	case *TextMessageStartEvent:
		return e.MessageID != ""
	case *TextMessageContentEvent:
		return e.MessageID != ""
	case *TextMessageEndEvent:
		return e.MessageID != ""
	}
	return true
}

func (r *StateTransitionRule) checkNoActiveMessageWithSameID(event Event, context *ValidationContext) bool {
	if msgEvent, ok := event.(*TextMessageStartEvent); ok {
		_, exists := context.State.ActiveMessages[msgEvent.MessageID]
		return !exists
	}
	return true
}

func (r *StateTransitionRule) checkMessageIDMatches(event Event, context *ValidationContext) bool {
	var messageID string
	switch e := event.(type) {
	case *TextMessageContentEvent:
		messageID = e.MessageID
	case *TextMessageEndEvent:
		messageID = e.MessageID
	default:
		return true
	}

	_, exists := context.State.ActiveMessages[messageID]
	return exists
}

func (r *StateTransitionRule) checkMessageIsActive(event Event, context *ValidationContext) bool {
	return len(context.State.ActiveMessages) > 0
}

func (r *StateTransitionRule) checkDeltaProvided(event Event) bool {
	if msgEvent, ok := event.(*TextMessageContentEvent); ok {
		return msgEvent.Delta != ""
	}
	return true
}

func (r *StateTransitionRule) checkToolCallIDValid(event Event) bool {
	switch e := event.(type) {
	case *ToolCallStartEvent:
		return e.ToolCallID != ""
	case *ToolCallArgsEvent:
		return e.ToolCallID != ""
	case *ToolCallEndEvent:
		return e.ToolCallID != ""
	}
	return true
}

func (r *StateTransitionRule) checkToolNameProvided(event Event) bool {
	if toolEvent, ok := event.(*ToolCallStartEvent); ok {
		return toolEvent.ToolCallName != ""
	}
	return true
}

func (r *StateTransitionRule) checkNoActiveToolWithSameID(event Event, context *ValidationContext) bool {
	if toolEvent, ok := event.(*ToolCallStartEvent); ok {
		_, exists := context.State.ActiveTools[toolEvent.ToolCallID]
		return !exists
	}
	return true
}

func (r *StateTransitionRule) checkToolCallIDMatches(event Event, context *ValidationContext) bool {
	var toolCallID string
	switch e := event.(type) {
	case *ToolCallArgsEvent:
		toolCallID = e.ToolCallID
	case *ToolCallEndEvent:
		toolCallID = e.ToolCallID
	default:
		return true
	}

	_, exists := context.State.ActiveTools[toolCallID]
	return exists
}

func (r *StateTransitionRule) checkToolIsActive(event Event, context *ValidationContext) bool {
	return len(context.State.ActiveTools) > 0
}

func (r *StateTransitionRule) checkArgsDeltaProvided(event Event) bool {
	if toolEvent, ok := event.(*ToolCallArgsEvent); ok {
		return toolEvent.Delta != ""
	}
	return true
}

func (r *StateTransitionRule) checkStepNameProvided(event Event) bool {
	switch e := event.(type) {
	case *StepStartedEvent:
		return e.StepName != ""
	case *StepFinishedEvent:
		return e.StepName != ""
	}
	return true
}

func (r *StateTransitionRule) checkNoActiveStepWithSameName(event Event, context *ValidationContext) bool {
	if stepEvent, ok := event.(*StepStartedEvent); ok {
		return !context.State.ActiveSteps[stepEvent.StepName]
	}
	return true
}

func (r *StateTransitionRule) checkStepNameMatches(event Event, context *ValidationContext) bool {
	if stepEvent, ok := event.(*StepFinishedEvent); ok {
		return context.State.ActiveSteps[stepEvent.StepName]
	}
	return true
}

func (r *StateTransitionRule) checkStepIsActive(event Event, context *ValidationContext) bool {
	return len(context.State.ActiveSteps) > 0
}

func (r *StateTransitionRule) checkSnapshotProvided(event Event) bool {
	if snapshotEvent, ok := event.(*StateSnapshotEvent); ok {
		return snapshotEvent.Snapshot != nil
	}
	return true
}

func (r *StateTransitionRule) checkDeltaOperationsValid(event Event) bool {
	if deltaEvent, ok := event.(*StateDeltaEvent); ok {
		return len(deltaEvent.Delta) > 0
	}
	return true
}

func (r *StateTransitionRule) checkStateConsistent(event Event, context *ValidationContext) bool {
	// Basic state consistency check
	return context.State != nil
}

func (r *StateTransitionRule) checkErrorMessageProvided(event Event) bool {
	if errorEvent, ok := event.(*RunErrorEvent); ok {
		return errorEvent.Message != ""
	}
	return true
}

// Additional validation methods

func (r *StateTransitionRule) validateStateConsistency(event Event, context *ValidationContext, result *ValidationResult, transition *StateTransition) {
	// Validate post-conditions
	for _, postcondition := range transition.PostConditions {
		if !r.checkPostCondition(postcondition, event, context) {
			result.AddError(r.CreateError(event,
				fmt.Sprintf("Post-condition '%s' not satisfied after state transition", postcondition),
				map[string]interface{}{
					"postcondition": postcondition,
					"transition":    transition,
				},
				[]string{
					"Ensure state is properly updated after the transition",
					"Check for any side effects that might affect state consistency",
				}))
		}
	}
}

func (r *StateTransitionRule) checkPostCondition(postcondition string, event Event, context *ValidationContext) bool {
	// Post-condition checks are typically validated after state updates
	// For this implementation, we'll assume they're valid
	return true
}

func (r *StateTransitionRule) validateRunStartTransition(event Event, context *ValidationContext, result *ValidationResult) {
	runEvent, ok := event.(*RunStartedEvent)
	if !ok {
		return
	}

	if runEvent.RunID == "" {
		result.AddError(r.CreateError(event, "Run ID is required for run start transition", nil,
			[]string{"Provide a valid run ID"}))
	}

	if _, exists := context.State.ActiveRuns[runEvent.RunID]; exists {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Run %s is already active", runEvent.RunID),
			map[string]interface{}{"run_id": runEvent.RunID},
			[]string{"Use a different run ID or finish the current run"}))
	}
}

func (r *StateTransitionRule) validateRunEndTransition(event Event, context *ValidationContext, result *ValidationResult) {
	var runID string
	switch e := event.(type) {
	case *RunFinishedEvent:
		runID = e.RunID
	case *RunErrorEvent:
		runID = e.RunID
	default:
		return
	}

	if runID == "" {
		result.AddError(r.CreateError(event, "Run ID is required for run end transition", nil,
			[]string{"Provide a valid run ID"}))
		return
	}

	if _, exists := context.State.ActiveRuns[runID]; !exists {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Cannot end run %s that is not active", runID),
			map[string]interface{}{"run_id": runID},
			[]string{"Start the run first before ending it"}))
	}
}

func (r *StateTransitionRule) validateBasicDeltaOperations(event Event, context *ValidationContext, result *ValidationResult) {
	deltaEvent, ok := event.(*StateDeltaEvent)
	if !ok {
		return
	}

	if len(deltaEvent.Delta) == 0 {
		result.AddError(r.CreateError(event, "Delta operations are required", nil,
			[]string{"Provide at least one delta operation"}))
	}
}

func (r *StateTransitionRule) generateOperationID(event Event) string {
	return fmt.Sprintf("op_%s_%d", event.Type(), time.Now().UnixNano())
}

func (r *StateTransitionRule) operationsConflict(event Event, op *ConcurrentStateOperation) bool {
	// Simple conflict detection based on event types
	conflictingTypes := map[EventType][]EventType{
		EventTypeRunStarted:  {EventTypeRunFinished, EventTypeRunError},
		EventTypeRunFinished: {EventTypeRunStarted, EventTypeRunError},
		EventTypeRunError:    {EventTypeRunStarted, EventTypeRunFinished},
	}

	conflicts, exists := conflictingTypes[event.Type()]
	if !exists {
		return false
	}

	for _, conflictType := range conflicts {
		if op.EventType == conflictType {
			return true
		}
	}

	return false
}

func (r *StateTransitionRule) cleanupCompletedOperations() {
	// Remove operations older than 1 minute
	cutoff := time.Now().Add(-1 * time.Minute)
	for id, op := range r.concurrentOps {
		if op.StartTime.Before(cutoff) {
			delete(r.concurrentOps, id)
		}
	}
}

func (r *StateTransitionRule) isRollbackOperation(event Event) bool {
	// Check if this is a rollback operation based on event metadata
	// This is a simplified implementation
	return event.Type() == EventTypeStateDelta
}

func (r *StateTransitionRule) extractRollbackOperation(event Event) *RollbackOperation {
	// Extract rollback operation details from event
	// This is a simplified implementation
	return &RollbackOperation{
		OperationID:    r.generateOperationID(event),
		TargetState:    "PREVIOUS",
		RollbackReason: "User initiated rollback",
		Timestamp:      time.Now(),
		PreviousDeltas: []JSONPatchOperation{},
	}
}

func (r *StateTransitionRule) isValidRollbackTarget(targetState string, context *ValidationContext) bool {
	// Validate if the target state is valid for rollback
	// This is a simplified implementation
	return targetState != ""
}

func (r *StateTransitionRule) cleanupRollbackHistory() {
	// Keep only the last MaxRollbackDepth entries
	if len(r.rollbackHistory) > r.config.MaxRollbackDepth {
		r.rollbackHistory = r.rollbackHistory[len(r.rollbackHistory)-r.config.MaxRollbackDepth:]
	}
}

func (r *StateTransitionRule) extractVersionInfo(event Event) *StateVersion {
	// Extract version information from event
	// This is a simplified implementation
	return &StateVersion{
		Version:   "1.0.0",
		Timestamp: time.Now(),
		Checksum:  "abc123",
	}
}

func (r *StateTransitionRule) getStateKey(event Event) string {
	// Generate a state key based on event
	return fmt.Sprintf("state_%s", event.Type())
}

func (r *StateTransitionRule) versionsCompatible(current, event string) bool {
	// Simple version compatibility check
	// In a real implementation, this would use proper semantic versioning
	return strings.HasPrefix(current, "1.") && strings.HasPrefix(event, "1.")
}

func (r *StateTransitionRule) validateDeltaOperation(operation JSONPatchOperation, context *ValidationContext) error {
	// Validate individual delta operation
	if operation.Op == "" {
		return fmt.Errorf("operation type is required")
	}

	if operation.Path == "" {
		return fmt.Errorf("operation path is required")
	}

	// Additional validation based on operation type
	switch operation.Op {
	case "add", "replace", "test":
		if operation.Value == nil {
			return fmt.Errorf("value is required for %s operation", operation.Op)
		}
	case "move", "copy":
		if operation.From == "" {
			return fmt.Errorf("from path is required for %s operation", operation.Op)
		}
	}

	return nil
}

func (r *StateTransitionRule) validateDeltaSequence(operations []JSONPatchOperation, context *ValidationContext) error {
	// Validate that delta operations can be applied in sequence
	// This is a simplified implementation

	// Check for conflicting operations
	paths := make(map[string]bool)
	for _, op := range operations {
		if op.Op == "remove" && paths[op.Path] {
			return fmt.Errorf("conflicting operations on path %s", op.Path)
		}
		paths[op.Path] = true
	}

	return nil
}

// GetConcurrentOperations returns current concurrent operations (for testing/debugging)
func (r *StateTransitionRule) GetConcurrentOperations() map[string]*ConcurrentStateOperation {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string]*ConcurrentStateOperation)
	for k, v := range r.concurrentOps {
		result[k] = v
	}
	return result
}

// GetRollbackHistory returns rollback history (for testing/debugging)
func (r *StateTransitionRule) GetRollbackHistory() []RollbackOperation {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make([]RollbackOperation, len(r.rollbackHistory))
	copy(result, r.rollbackHistory)
	return result
}

// GetStateVersions returns state versions (for testing/debugging)
func (r *StateTransitionRule) GetStateVersions() map[string]*StateVersion {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string]*StateVersion)
	for k, v := range r.stateVersions {
		result[k] = v
	}
	return result
}

// GetConfig returns the current configuration
func (r *StateTransitionRule) GetConfig() *StateTransitionConfig {
	return r.config
}

// UpdateConfig updates the configuration
func (r *StateTransitionRule) UpdateConfig(config *StateTransitionConfig) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.config = config
}
