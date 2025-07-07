package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ValidationSeverity defines the severity level of validation errors
type ValidationSeverity int

const (
	ValidationSeverityError ValidationSeverity = iota
	ValidationSeverityWarning
	ValidationSeverityInfo
)

func (s ValidationSeverity) String() string {
	switch s {
	case ValidationSeverityError:
		return "ERROR"
	case ValidationSeverityWarning:
		return "WARNING"
	case ValidationSeverityInfo:
		return "INFO"
	default:
		return "UNKNOWN"
	}
}

// ValidationError represents a validation error with context
type ValidationError struct {
	RuleID      string                 `json:"rule_id"`
	EventID     string                 `json:"event_id,omitempty"`
	EventType   EventType              `json:"event_type"`
	Message     string                 `json:"message"`
	Severity    ValidationSeverity     `json:"severity"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Severity, e.RuleID, e.Message)
}

// ValidationResult represents the result of validation
type ValidationResult struct {
	IsValid     bool                `json:"is_valid"`
	Errors      []*ValidationError  `json:"errors,omitempty"`
	Warnings    []*ValidationError  `json:"warnings,omitempty"`
	Information []*ValidationError  `json:"information,omitempty"`
	EventCount  int                 `json:"event_count"`
	Duration    time.Duration       `json:"duration"`
	Timestamp   time.Time           `json:"timestamp"`
}

// HasErrors returns true if there are any validation errors
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if there are any validation warnings
func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// AddError adds a validation error to the result
func (r *ValidationResult) AddError(err *ValidationError) {
	r.Errors = append(r.Errors, err)
	r.IsValid = false
}

// AddWarning adds a validation warning to the result
func (r *ValidationResult) AddWarning(warning *ValidationError) {
	r.Warnings = append(r.Warnings, warning)
}

// AddInfo adds a validation info to the result
func (r *ValidationResult) AddInfo(info *ValidationError) {
	r.Information = append(r.Information, info)
}

// ValidationRule defines an interface for validation rules
type ValidationRule interface {
	// ID returns the unique identifier for this rule
	ID() string
	
	// Description returns a human-readable description of the rule
	Description() string
	
	// Validate validates an event against this rule
	Validate(event Event, context *ValidationContext) *ValidationResult
	
	// IsEnabled returns whether this rule is enabled
	IsEnabled() bool
	
	// SetEnabled enables or disables this rule
	SetEnabled(enabled bool)
	
	// GetSeverity returns the severity level for violations of this rule
	GetSeverity() ValidationSeverity
	
	// SetSeverity sets the severity level for violations of this rule
	SetSeverity(severity ValidationSeverity)
}

// BaseValidationRule provides common functionality for validation rules
type BaseValidationRule struct {
	id          string
	description string
	enabled     bool
	severity    ValidationSeverity
	mutex       sync.RWMutex
}

// NewBaseValidationRule creates a new base validation rule
func NewBaseValidationRule(id, description string, severity ValidationSeverity) *BaseValidationRule {
	return &BaseValidationRule{
		id:          id,
		description: description,
		enabled:     true,
		severity:    severity,
	}
}

// ID returns the rule ID
func (r *BaseValidationRule) ID() string {
	return r.id
}

// Description returns the rule description
func (r *BaseValidationRule) Description() string {
	return r.description
}

// IsEnabled returns whether the rule is enabled
func (r *BaseValidationRule) IsEnabled() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.enabled
}

// SetEnabled enables or disables the rule
func (r *BaseValidationRule) SetEnabled(enabled bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.enabled = enabled
}

// GetSeverity returns the rule severity
func (r *BaseValidationRule) GetSeverity() ValidationSeverity {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.severity
}

// SetSeverity sets the rule severity
func (r *BaseValidationRule) SetSeverity(severity ValidationSeverity) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.severity = severity
}

// CreateError creates a validation error for this rule
func (r *BaseValidationRule) CreateError(event Event, message string, context map[string]interface{}, suggestions []string) *ValidationError {
	eventID := ""
	// BaseEvent doesn't have EventID field, so we'll leave it empty for now
	// Individual event types can override this if they have IDs

	return &ValidationError{
		RuleID:      r.id,
		EventID:     eventID,
		EventType:   event.Type(),
		Message:     message,
		Severity:    r.severity,
		Context:     context,
		Suggestions: suggestions,
		Timestamp:   time.Now(),
	}
}

// EventPhase represents the current phase of event processing
type EventPhase int

const (
	PhaseInit EventPhase = iota
	PhaseRunning
	PhaseFinished
	PhaseError
)

func (p EventPhase) String() string {
	switch p {
	case PhaseInit:
		return "INIT"
	case PhaseRunning:
		return "RUNNING"
	case PhaseFinished:
		return "FINISHED"
	case PhaseError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// RunState tracks the state of a specific run
type RunState struct {
	RunID     string    `json:"run_id"`
	ThreadID  string    `json:"thread_id"`
	StartTime time.Time `json:"start_time"`
	Phase     EventPhase `json:"phase"`
	StepCount int       `json:"step_count"`
}

// MessageState tracks the state of a specific message
type MessageState struct {
	MessageID    string    `json:"message_id"`
	ParentMsgID  string    `json:"parent_msg_id,omitempty"`
	StartTime    time.Time `json:"start_time"`
	ContentCount int       `json:"content_count"`
	IsActive     bool      `json:"is_active"`
}

// ToolState tracks the state of a specific tool call
type ToolState struct {
	ToolCallID   string    `json:"tool_call_id"`
	ParentMsgID  string    `json:"parent_msg_id,omitempty"`
	ToolName     string    `json:"tool_name"`
	StartTime    time.Time `json:"start_time"`
	ArgsCount    int       `json:"args_count"`
	IsActive     bool      `json:"is_active"`
}

// ValidationState tracks the current state of validation
type ValidationState struct {
	CurrentPhase     EventPhase                `json:"current_phase"`
	ActiveRuns       map[string]*RunState      `json:"active_runs"`
	FinishedRuns     map[string]*RunState      `json:"finished_runs"`
	ActiveMessages   map[string]*MessageState  `json:"active_messages"`
	FinishedMessages map[string]*MessageState  `json:"finished_messages"`
	ActiveTools      map[string]*ToolState     `json:"active_tools"`
	FinishedTools    map[string]*ToolState     `json:"finished_tools"`
	ActiveSteps      map[string]bool           `json:"active_steps"`
	EventCount       int                       `json:"event_count"`
	LastEventTime    time.Time                 `json:"last_event_time"`
	StartTime        time.Time                 `json:"start_time"`
	
	// Thread safety
	mutex sync.RWMutex
}

// NewValidationState creates a new validation state
func NewValidationState() *ValidationState {
	return &ValidationState{
		CurrentPhase:     PhaseInit,
		ActiveRuns:       make(map[string]*RunState),
		FinishedRuns:     make(map[string]*RunState),
		ActiveMessages:   make(map[string]*MessageState),
		FinishedMessages: make(map[string]*MessageState),
		ActiveTools:      make(map[string]*ToolState),
		FinishedTools:    make(map[string]*ToolState),
		ActiveSteps:      make(map[string]bool),
		StartTime:        time.Now(),
	}
}

// CleanupFinishedItems removes finished runs, messages, and tools older than the specified time.
// This prevents memory leaks in long-running applications by removing old state data.
func (s *ValidationState) CleanupFinishedItems(olderThan time.Time) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// Cleanup finished runs
	for id, run := range s.FinishedRuns {
		if run.StartTime.Before(olderThan) {
			delete(s.FinishedRuns, id)
		}
	}
	
	// Cleanup finished messages
	for id, msg := range s.FinishedMessages {
		if msg.StartTime.Before(olderThan) {
			delete(s.FinishedMessages, id)
		}
	}
	
	// Cleanup finished tools
	for id, tool := range s.FinishedTools {
		if tool.StartTime.Before(olderThan) {
			delete(s.FinishedTools, id)
		}
	}
}

// GetMemoryStats returns statistics about the current memory usage of the validation state.
func (s *ValidationState) GetMemoryStats() map[string]int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return map[string]int{
		"active_runs":       len(s.ActiveRuns),
		"finished_runs":     len(s.FinishedRuns),
		"active_messages":   len(s.ActiveMessages),
		"finished_messages": len(s.FinishedMessages),
		"active_tools":      len(s.ActiveTools),
		"finished_tools":    len(s.FinishedTools),
		"active_steps":      len(s.ActiveSteps),
		"total_finished":    len(s.FinishedRuns) + len(s.FinishedMessages) + len(s.FinishedTools),
	}
}

// ValidationContext provides context for validation operations
type ValidationContext struct {
	State         *ValidationState   `json:"state"`
	EventSequence []Event            `json:"event_sequence,omitempty"`
	CurrentEvent  Event              `json:"current_event,omitempty"`
	EventIndex    int                `json:"event_index"`
	Config        *ValidationConfig  `json:"config,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Context       context.Context    `json:"-"` // Context for cancellation
}

// ValidationMetrics tracks performance metrics for validation
type ValidationMetrics struct {
	EventsProcessed     int64         `json:"events_processed"`
	ValidationDuration  time.Duration `json:"validation_duration"`
	AverageEventLatency time.Duration `json:"average_event_latency"`
	ErrorCount          int64         `json:"error_count"`
	WarningCount        int64         `json:"warning_count"`
	RuleExecutionTimes  map[string]time.Duration `json:"rule_execution_times"`
	StartTime           time.Time     `json:"start_time"`
	
	// Thread safety
	mutex sync.RWMutex
}

// NewValidationMetrics creates new validation metrics
func NewValidationMetrics() *ValidationMetrics {
	return &ValidationMetrics{
		RuleExecutionTimes: make(map[string]time.Duration),
		StartTime:          time.Now(),
	}
}

// RecordEvent records processing of an event
func (m *ValidationMetrics) RecordEvent(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.EventsProcessed++
	m.ValidationDuration += duration
	
	if m.EventsProcessed > 0 {
		m.AverageEventLatency = m.ValidationDuration / time.Duration(m.EventsProcessed)
	}
}

// RecordError records a validation error
func (m *ValidationMetrics) RecordError() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.ErrorCount++
}

// RecordWarning records a validation warning
func (m *ValidationMetrics) RecordWarning() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.WarningCount++
}

// RecordRuleExecution records execution time for a specific rule
func (m *ValidationMetrics) RecordRuleExecution(ruleID string, duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Store the latest execution time instead of accumulating
	// This prevents unbounded memory growth
	m.RuleExecutionTimes[ruleID] = duration
}

// EventValidator provides comprehensive event validation
type EventValidator struct {
	rules              []ValidationRule
	state              *ValidationState
	metrics            *ValidationMetrics
	config             *ValidationConfig
	tracer             trace.Tracer
	parallelValidator  *ParallelValidator
	parallelConfig     *ParallelValidationConfig
	mutex              sync.RWMutex
}

// NewEventValidator creates a new event validator with the specified configuration.
// If config is nil, DefaultValidationConfig() is used which applies strict validation.
//
// The validator is thread-safe and can be used concurrently. It validates events
// according to AG-UI protocol requirements and provides detailed error reporting.
//
// Example - Basic Usage:
//
//	validator := NewEventValidator(nil) // Uses defaults
//	result := validator.ValidateEvent(ctx, event)
//	if result.HasErrors() {
//	    for _, err := range result.Errors {
//	        log.Printf("Validation error: %s", err.Message)
//	    }
//	}
//
// Example - Custom Configuration:
//
//	config := &ValidationConfig{
//	    Level: ValidationPermissive,
//	    SkipTimestampValidation: true,
//	}
//	validator := NewEventValidator(config)
//
// Example - Sequence Validation:
//
//	result := validator.ValidateSequence(ctx, []Event{
//	    &RunStartedEvent{RunID: "run-1", ThreadID: "thread-1"},
//	    &TextMessageStartEvent{MessageID: "msg-1"},
//	    &TextMessageEndEvent{MessageID: "msg-1"},
//	    &RunFinishedEvent{RunID: "run-1"},
//	})
func NewEventValidator(config *ValidationConfig) *EventValidator {
	if config == nil {
		config = DefaultValidationConfig()
	}
	
	validator := &EventValidator{
		rules:              make([]ValidationRule, 0),
		state:              NewValidationState(),
		metrics:            NewValidationMetrics(),
		config:             config,
		tracer:             otel.Tracer("ag-ui/events/validation"),
		parallelValidator:  NewParallelValidator(DefaultParallelValidationConfig()),
		parallelConfig:     DefaultParallelValidationConfig(),
	}
	
	// Add default rules
	validator.AddDefaultRules()
	
	return validator
}

// AddRule adds a validation rule to the validator.
// The rule will be executed for all subsequent validation operations.
// Rules are executed in the order they were added.
//
// Example:
//
//	customRule := &MyCustomRule{
//	    BaseValidationRule: BaseValidationRule{
//	        id:          "CUSTOM_BUSINESS_LOGIC",
//	        description: "Validates custom business requirements",
//	        severity:    ValidationSeverityError,
//	        enabled:     true,
//	    },
//	}
//	validator.AddRule(customRule)
func (v *EventValidator) AddRule(rule ValidationRule) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.rules = append(v.rules, rule)
}

// RemoveRule removes a validation rule by ID
func (v *EventValidator) RemoveRule(ruleID string) bool {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	
	for i, rule := range v.rules {
		if rule.ID() == ruleID {
			v.rules = append(v.rules[:i], v.rules[i+1:]...)
			return true
		}
	}
	return false
}

// GetRule gets a validation rule by ID
func (v *EventValidator) GetRule(ruleID string) ValidationRule {
	v.mutex.RLock()
	defer v.mutex.RUnlock()
	
	for _, rule := range v.rules {
		if rule.ID() == ruleID {
			return rule
		}
	}
	return nil
}

// GetRules returns all validation rules
func (v *EventValidator) GetRules() []ValidationRule {
	v.mutex.RLock()
	defer v.mutex.RUnlock()
	
	rules := make([]ValidationRule, len(v.rules))
	copy(rules, v.rules)
	return rules
}

// ValidateEvent validates a single event against all enabled validation rules.
// It returns a ValidationResult containing any errors, warnings, or informational messages.
//
// The validator maintains internal state tracking for proper event sequence validation.
// Events are only added to state if validation passes without errors.
//
// Context is used for cancellation support (future enhancement).
//
// Example:
//
//	event := &RunStartedEvent{
//	    BaseEvent: &BaseEvent{
//	        EventType: EventTypeRunStarted,
//	        TimestampMs: timePtr(time.Now().UnixMilli()),
//	    },
//	    RunID:    "run-123",
//	    ThreadID: "thread-456",
//	}
//	
//	result := validator.ValidateEvent(ctx, event)
//	if result.HasErrors() {
//	    // Handle validation errors
//	    for _, err := range result.Errors {
//	        log.Printf("[%s] %s: %s", err.Severity, err.RuleID, err.Message)
//	        // Use err.Suggestions for helpful remediation hints
//	    }
//	} else if result.HasWarnings() {
//	    // Log warnings but continue processing
//	    for _, warn := range result.Warnings {
//	        log.Printf("Warning: %s", warn.Message)
//	    }
//	}
func (v *EventValidator) ValidateEvent(ctx context.Context, event Event) *ValidationResult {
	// Start distributed tracing span
	ctx, span := v.tracer.Start(ctx, "event_validation",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	start := time.Now()
	defer func() {
		duration := time.Since(start)
		v.metrics.RecordEvent(duration)
		
		// Record tracing attributes
		span.SetAttributes(
			attribute.Int64("validation.duration_ms", duration.Milliseconds()),
			attribute.Int("validation.event_count", 1),
		)
	}()

	result := &ValidationResult{
		IsValid:   true,
		Errors:    make([]*ValidationError, 0),
		Warnings:  make([]*ValidationError, 0),
		EventCount: 1,
		Timestamp: time.Now(),
	}

	// Add event attributes to span if event is not nil
	if event != nil {
		span.SetAttributes(
			attribute.String("event.type", string(event.Type())),
		)
		// Add event ID if available through type assertion
		if eventWithID, ok := event.(interface{ GetEventID() string }); ok {
			if eventID := eventWithID.GetEventID(); eventID != "" {
				span.SetAttributes(attribute.String("event.id", eventID))
			}
		}
	}

	// Check context before starting
	select {
	case <-ctx.Done():
		result.IsValid = false
		validationError := &ValidationError{
			RuleID:    "CONTEXT_CANCELLED",
			Message:   "Validation cancelled by context",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		}
		result.AddError(validationError)
		result.Duration = time.Since(start)
		
		// Record error in span
		span.RecordError(fmt.Errorf("validation cancelled by context"))
		span.SetStatus(codes.Error, "Validation cancelled")
		span.AddEvent("validation.cancelled")
		
		return result
	default:
	}

	if event == nil {
		validationError := &ValidationError{
			RuleID:    "NULL_EVENT",
			Message:   "Event cannot be nil",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		}
		result.AddError(validationError)
		
		// Record error in span
		span.RecordError(fmt.Errorf("event cannot be nil"))
		span.SetStatus(codes.Error, "Null event")
		span.AddEvent("validation.null_event")
		
		return result
	}

	// Create a snapshot of the current state for validation to prevent race conditions
	// This ensures validation rules read from a consistent state snapshot
	stateSnapshot := v.createStateSnapshot()
	
	// Create validation context with the state snapshot
	validationContext := &ValidationContext{
		State:        stateSnapshot,
		CurrentEvent: event,
		EventIndex:   0,
		Config:       v.config,
		Metadata:     make(map[string]interface{}),
		Context:      ctx,
	}

	// Apply validation rules
	v.mutex.RLock()
	rules := make([]ValidationRule, len(v.rules))
	copy(rules, v.rules)
	v.mutex.RUnlock()

	span.SetAttributes(attribute.Int("validation.rules_count", len(rules)))

	for _, rule := range rules {
		if !rule.IsEnabled() {
			continue
		}

		// Create child span for rule execution
		ruleCtx, ruleSpan := v.tracer.Start(ctx, "rule_validation",
			trace.WithSpanKind(trace.SpanKindInternal),
		)
		ruleSpan.SetAttributes(
			attribute.String("validation.rule.id", rule.ID()),
			attribute.String("validation.rule.description", rule.Description()),
			attribute.String("validation.rule.severity", rule.GetSeverity().String()),
		)

		ruleStart := time.Now()
		ruleResult := rule.Validate(event, validationContext)
		ruleDuration := time.Since(ruleStart)
		
		// Record rule execution metrics and tracing
		v.metrics.RecordRuleExecution(rule.ID(), ruleDuration)
		ruleSpan.SetAttributes(
			attribute.Int64("validation.rule.duration_ms", ruleDuration.Milliseconds()),
		)

		if ruleResult != nil {
			// Add errors
			errorCount := len(ruleResult.Errors)
			warningCount := len(ruleResult.Warnings)
			
			for _, err := range ruleResult.Errors {
				result.AddError(err)
				v.metrics.RecordError()
				
				// Record error event in rule span
				ruleSpan.AddEvent("validation.rule.error", trace.WithAttributes(
					attribute.String("error.message", err.Message),
					attribute.String("error.rule_id", err.RuleID),
				))
			}
			
			// Add warnings
			for _, warning := range ruleResult.Warnings {
				result.AddWarning(warning)
				v.metrics.RecordWarning()
				
				// Record warning event in rule span
				ruleSpan.AddEvent("validation.rule.warning", trace.WithAttributes(
					attribute.String("warning.message", warning.Message),
					attribute.String("warning.rule_id", warning.RuleID),
				))
			}
			
			// Add information
			for _, info := range ruleResult.Information {
				result.AddInfo(info)
				
				// Record info event in rule span
				ruleSpan.AddEvent("validation.rule.info", trace.WithAttributes(
					attribute.String("info.message", info.Message),
					attribute.String("info.rule_id", info.RuleID),
				))
			}
			
			// Set rule span status and attributes
			ruleSpan.SetAttributes(
				attribute.Int("validation.rule.error_count", errorCount),
				attribute.Int("validation.rule.warning_count", warningCount),
			)
			
			if errorCount > 0 {
				ruleSpan.SetStatus(codes.Error, fmt.Sprintf("Rule validation failed with %d errors", errorCount))
			} else {
				ruleSpan.SetStatus(codes.Ok, "Rule validation completed")
			}
		} else {
			ruleSpan.SetStatus(codes.Ok, "Rule validation completed")
		}
		
		ruleSpan.End()
		_ = ruleCtx // Mark as used
	}

	// Update state only after successful validation
	if result.IsValid {
		v.updateState(event)
	}

	result.Duration = time.Since(start)
	
	// Set final span attributes and status
	span.SetAttributes(
		attribute.Bool("validation.result.is_valid", result.IsValid),
		attribute.Int("validation.result.error_count", len(result.Errors)),
		attribute.Int("validation.result.warning_count", len(result.Warnings)),
		attribute.Int("validation.result.info_count", len(result.Information)),
	)
	
	if result.IsValid {
		span.SetStatus(codes.Ok, "Event validation completed successfully")
	} else {
		span.SetStatus(codes.Error, fmt.Sprintf("Event validation failed with %d errors", len(result.Errors)))
		span.AddEvent("validation.failed", trace.WithAttributes(
			attribute.Int("error_count", len(result.Errors)),
		))
	}
	
	return result
}

// ValidateSequence validates a sequence of events in order, ensuring they follow
// AG-UI protocol requirements for event ordering and lifecycle management.
//
// This method is thread-safe and creates an isolated validator instance for each
// validation call. This ensures that concurrent ValidateSequence calls don't 
// interfere with each other. Each event is validated in order, and validation 
// stops at the first error unless config allows continuation.
//
// Example - Complete Run Lifecycle:
//
//	events := []Event{
//	    &RunStartedEvent{RunID: "run-1", ThreadID: "thread-1"},
//	    &TextMessageStartEvent{MessageID: "msg-1", ParentMsgID: ""},
//	    &TextMessageContentEvent{MessageID: "msg-1", Delta: "Hello"},
//	    &TextMessageContentEvent{MessageID: "msg-1", Delta: " world!"},
//	    &TextMessageEndEvent{MessageID: "msg-1"},
//	    &RunFinishedEvent{RunID: "run-1"},
//	}
//	
//	result := validator.ValidateSequence(ctx, events)
//	if !result.IsValid {
//	    log.Printf("Sequence validation failed with %d errors", len(result.Errors))
//	}
//
// Example - Tool Call Sequence:
//
//	events := []Event{
//	    &RunStartedEvent{RunID: "run-1", ThreadID: "thread-1"},
//	    &ToolCallStartEvent{ToolCallID: "tool-1", ToolName: "search"},
//	    &ToolCallArgsEvent{ToolCallID: "tool-1", ArgsChunk: `{"query":`},
//	    &ToolCallArgsEvent{ToolCallID: "tool-1", ArgsChunk: `"weather"}`},
//	    &ToolCallEndEvent{ToolCallID: "tool-1"},
//	    &RunFinishedEvent{RunID: "run-1"},
//	}
func (v *EventValidator) ValidateSequence(ctx context.Context, events []Event) *ValidationResult {
	// Start distributed tracing span for sequence validation
	ctx, span := v.tracer.Start(ctx, "sequence_validation",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	start := time.Now()
	
	result := &ValidationResult{
		IsValid:    true,
		Errors:     make([]*ValidationError, 0),
		Warnings:   make([]*ValidationError, 0),
		EventCount: len(events),
		Timestamp:  time.Now(),
	}

	// Add sequence attributes to span
	span.SetAttributes(
		attribute.Int("validation.sequence.event_count", len(events)),
	)

	// Check context before starting
	select {
	case <-ctx.Done():
		result.IsValid = false
		validationError := &ValidationError{
			RuleID:    "CONTEXT_CANCELLED",
			Message:   "Validation cancelled by context",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		}
		result.AddError(validationError)
		result.Duration = time.Since(start)
		
		// Record error in span
		span.RecordError(fmt.Errorf("sequence validation cancelled by context"))
		span.SetStatus(codes.Error, "Sequence validation cancelled")
		span.AddEvent("validation.sequence.cancelled")
		
		return result
	default:
	}

	if len(events) == 0 {
		result.Duration = time.Since(start)
		span.SetStatus(codes.Ok, "Empty sequence validation completed")
		span.AddEvent("validation.sequence.empty")
		return result
	}

	// Create an isolated validator for this sequence validation to ensure thread safety
	// This prevents concurrent ValidateSequence calls from interfering with each other
	isolatedValidator := &EventValidator{
		rules:   v.GetRules(),             // Get a copy of rules
		state:   NewValidationState(),     // Fresh state for this validation
		metrics: NewValidationMetrics(),   // Fresh metrics for this validation
		config:  v.config,                 // Config is read-only, safe to share
		mutex:   sync.RWMutex{},          // New mutex for the isolated validator
	}

	// Create validation context for sequence using the isolated validator's state
	validationContext := &ValidationContext{
		State:         isolatedValidator.state,
		EventSequence: events,
		Config:        isolatedValidator.config,
		Metadata:      make(map[string]interface{}),
		Context:       ctx,
	}

	// Validate each event in sequence
	for i, event := range events {
		// Check context periodically during long sequences
		if i > 0 && i%DefaultBatchCheckInterval == 0 {
			select {
			case <-ctx.Done():
				result.IsValid = false
				validationError := &ValidationError{
					RuleID:    "CONTEXT_CANCELLED",
					Message:   fmt.Sprintf("Validation cancelled after %d events", i),
					Severity:  ValidationSeverityError,
					Timestamp: time.Now(),
				}
				result.AddError(validationError)
				result.Duration = time.Since(start)
				
				// Record cancellation in span
				span.RecordError(fmt.Errorf("sequence validation cancelled after %d events", i))
				span.SetStatus(codes.Error, "Sequence validation cancelled during processing")
				span.AddEvent("validation.sequence.cancelled_during_processing", trace.WithAttributes(
					attribute.Int("events_processed", i),
				))
				
				return result
			default:
			}
		}

		// Create child span for individual event validation
		eventCtx, eventSpan := v.tracer.Start(ctx, "sequence_event_validation",
			trace.WithSpanKind(trace.SpanKindInternal),
		)
		eventSpan.SetAttributes(
			attribute.Int("validation.sequence.event_index", i),
		)
		
		if event != nil {
			eventSpan.SetAttributes(
				attribute.String("event.type", string(event.Type())),
			)
		}

		validationContext.CurrentEvent = event
		validationContext.EventIndex = i
		
		// Validate the event using the isolated validator's context
		eventResult := isolatedValidator.validateEventWithContext(eventCtx, event, validationContext)
		
		// Record event validation result in span
		eventSpan.SetAttributes(
			attribute.Bool("validation.event.is_valid", eventResult.IsValid),
			attribute.Int("validation.event.error_count", len(eventResult.Errors)),
			attribute.Int("validation.event.warning_count", len(eventResult.Warnings)),
		)
		
		if eventResult.IsValid {
			eventSpan.SetStatus(codes.Ok, "Event validation completed")
		} else {
			eventSpan.SetStatus(codes.Error, fmt.Sprintf("Event validation failed with %d errors", len(eventResult.Errors)))
		}
		
		// Merge results
		for _, err := range eventResult.Errors {
			result.AddError(err)
		}
		for _, warning := range eventResult.Warnings {
			result.AddWarning(warning)
		}
		for _, info := range eventResult.Information {
			result.AddInfo(info)
		}
		
		eventSpan.End()
		_ = eventCtx // Mark as used
	}

	result.Duration = time.Since(start)
	
	// Set final sequence span attributes and status
	span.SetAttributes(
		attribute.Bool("validation.sequence.is_valid", result.IsValid),
		attribute.Int("validation.sequence.error_count", len(result.Errors)),
		attribute.Int("validation.sequence.warning_count", len(result.Warnings)),
		attribute.Int("validation.sequence.info_count", len(result.Information)),
		attribute.Int64("validation.sequence.duration_ms", result.Duration.Milliseconds()),
	)
	
	if result.IsValid {
		span.SetStatus(codes.Ok, "Sequence validation completed successfully")
		span.AddEvent("validation.sequence.completed", trace.WithAttributes(
			attribute.Int("events_processed", len(events)),
		))
	} else {
		span.SetStatus(codes.Error, fmt.Sprintf("Sequence validation failed with %d errors", len(result.Errors)))
		span.AddEvent("validation.sequence.failed", trace.WithAttributes(
			attribute.Int("error_count", len(result.Errors)),
			attribute.Int("events_processed", len(events)),
		))
	}
	
	return result
}

// validateEventWithContext validates an event with a specific validation context
func (v *EventValidator) validateEventWithContext(ctx context.Context, event Event, validationContext *ValidationContext) *ValidationResult {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		v.metrics.RecordEvent(duration)
	}()

	result := &ValidationResult{
		IsValid:   true,
		Errors:    make([]*ValidationError, 0),
		Warnings:  make([]*ValidationError, 0),
		EventCount: 1,
		Timestamp: time.Now(),
	}

	if event == nil {
		result.AddError(&ValidationError{
			RuleID:    "NULL_EVENT",
			Message:   "Event cannot be nil",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		})
		return result
	}

	// Apply validation rules using the provided context (validate before updating state)
	v.mutex.RLock()
	rules := make([]ValidationRule, len(v.rules))
	copy(rules, v.rules)
	v.mutex.RUnlock()

	for _, rule := range rules {
		if !rule.IsEnabled() {
			continue
		}

		ruleStart := time.Now()
		ruleResult := rule.Validate(event, validationContext)
		ruleDuration := time.Since(ruleStart)
		
		v.metrics.RecordRuleExecution(rule.ID(), ruleDuration)

		if ruleResult != nil {
			// Add errors
			for _, err := range ruleResult.Errors {
				result.AddError(err)
				v.metrics.RecordError()
			}
			
			// Add warnings
			for _, warning := range ruleResult.Warnings {
				result.AddWarning(warning)
				v.metrics.RecordWarning()
			}
			
			// Add information
			for _, info := range ruleResult.Information {
				result.AddInfo(info)
			}
		}
	}

	// Update state only after successful validation
	if result.IsValid {
		v.updateState(event)
	}

	result.Duration = time.Since(start)
	return result
}

// GetState returns the current validation state
func (v *EventValidator) GetState() *ValidationState {
	v.state.mutex.RLock()
	defer v.state.mutex.RUnlock()
	
	// Return a copy to prevent external modification
	stateCopy := &ValidationState{
		CurrentPhase:     v.state.CurrentPhase,
		ActiveRuns:       make(map[string]*RunState),
		FinishedRuns:     make(map[string]*RunState),
		ActiveMessages:   make(map[string]*MessageState),
		FinishedMessages: make(map[string]*MessageState),
		ActiveTools:      make(map[string]*ToolState),
		FinishedTools:    make(map[string]*ToolState),
		ActiveSteps:      make(map[string]bool),
		EventCount:       v.state.EventCount,
		LastEventTime:    v.state.LastEventTime,
		StartTime:        v.state.StartTime,
	}
	
	// Deep copy maps
	for k, v := range v.state.ActiveRuns {
		runCopy := *v
		stateCopy.ActiveRuns[k] = &runCopy
	}
	for k, v := range v.state.FinishedRuns {
		runCopy := *v
		stateCopy.FinishedRuns[k] = &runCopy
	}
	for k, v := range v.state.ActiveMessages {
		msgCopy := *v
		stateCopy.ActiveMessages[k] = &msgCopy
	}
	for k, v := range v.state.FinishedMessages {
		msgCopy := *v
		stateCopy.FinishedMessages[k] = &msgCopy
	}
	for k, v := range v.state.ActiveTools {
		toolCopy := *v
		stateCopy.ActiveTools[k] = &toolCopy
	}
	for k, v := range v.state.FinishedTools {
		toolCopy := *v
		stateCopy.FinishedTools[k] = &toolCopy
	}
	for k, v := range v.state.ActiveSteps {
		stateCopy.ActiveSteps[k] = v
	}
	
	return stateCopy
}

// GetMetrics returns the validation metrics
func (v *EventValidator) GetMetrics() *ValidationMetrics {
	v.metrics.mutex.RLock()
	defer v.metrics.mutex.RUnlock()
	
	// Return a copy to prevent external modification
	metricsCopy := &ValidationMetrics{
		EventsProcessed:     v.metrics.EventsProcessed,
		ValidationDuration:  v.metrics.ValidationDuration,
		AverageEventLatency: v.metrics.AverageEventLatency,
		ErrorCount:          v.metrics.ErrorCount,
		WarningCount:        v.metrics.WarningCount,
		RuleExecutionTimes:  make(map[string]time.Duration),
		StartTime:           v.metrics.StartTime,
	}
	
	// Deep copy the map
	for k, v := range v.metrics.RuleExecutionTimes {
		metricsCopy.RuleExecutionTimes[k] = v
	}
	
	return metricsCopy
}

// Reset resets the validator state
func (v *EventValidator) Reset() {
	v.state = NewValidationState()
	v.metrics = NewValidationMetrics()
}

// StartCleanupRoutine starts a background goroutine that periodically cleans up old finished items.
// This prevents memory leaks in long-running applications. The cleanup runs at the specified interval
// and removes items older than the retention period.
//
// Example:
//
//	validator := NewEventValidator(config)
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	
//	// Clean up items older than 24 hours every hour
//	validator.StartCleanupRoutine(ctx, time.Hour, 24*time.Hour)
func (v *EventValidator) StartCleanupRoutine(ctx context.Context, interval time.Duration, retentionPeriod time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cutoff := time.Now().Add(-retentionPeriod)
				v.state.CleanupFinishedItems(cutoff)
				
				// Also cleanup old metrics data
				v.cleanupMetrics(cutoff)
			}
		}
	}()
}

// cleanupMetrics removes old rule execution time data
func (v *EventValidator) cleanupMetrics(olderThan time.Time) {
	v.metrics.mutex.Lock()
	defer v.metrics.mutex.Unlock()
	
	// For metrics, we'll keep a rolling window of execution times
	// This is a simple implementation - in production you might want
	// a more sophisticated approach like keeping only the last N entries
	if len(v.metrics.RuleExecutionTimes) > 1000 {
		// Clear and start fresh if we have too many entries
		v.metrics.RuleExecutionTimes = make(map[string]time.Duration)
	}
}

// createStateSnapshot creates a read-only snapshot of the current validation state.
// This is used to prevent race conditions during validation by ensuring rules
// read from a consistent state snapshot rather than the mutable state.
func (v *EventValidator) createStateSnapshot() *ValidationState {
	v.state.mutex.RLock()
	defer v.state.mutex.RUnlock()
	
	// Create a new state with copies of the data
	snapshot := &ValidationState{
		CurrentPhase:     v.state.CurrentPhase,
		EventCount:       v.state.EventCount,
		LastEventTime:    v.state.LastEventTime,
		StartTime:        v.state.StartTime,
		ActiveRuns:       make(map[string]*RunState),
		FinishedRuns:     make(map[string]*RunState),
		ActiveMessages:   make(map[string]*MessageState),
		FinishedMessages: make(map[string]*MessageState),
		ActiveTools:      make(map[string]*ToolState),
		FinishedTools:    make(map[string]*ToolState),
		ActiveSteps:      make(map[string]bool),
	}
	
	// Deep copy active runs
	for k, v := range v.state.ActiveRuns {
		runCopy := *v
		snapshot.ActiveRuns[k] = &runCopy
	}
	
	// Deep copy finished runs
	for k, v := range v.state.FinishedRuns {
		runCopy := *v
		snapshot.FinishedRuns[k] = &runCopy
	}
	
	// Deep copy active messages
	for k, v := range v.state.ActiveMessages {
		msgCopy := *v
		snapshot.ActiveMessages[k] = &msgCopy
	}
	
	// Deep copy finished messages
	for k, v := range v.state.FinishedMessages {
		msgCopy := *v
		snapshot.FinishedMessages[k] = &msgCopy
	}
	
	// Deep copy active tools
	for k, v := range v.state.ActiveTools {
		toolCopy := *v
		snapshot.ActiveTools[k] = &toolCopy
	}
	
	// Deep copy finished tools
	for k, v := range v.state.FinishedTools {
		toolCopy := *v
		snapshot.FinishedTools[k] = &toolCopy
	}
	
	// Copy active steps
	for k, v := range v.state.ActiveSteps {
		snapshot.ActiveSteps[k] = v
	}
	
	return snapshot
}

// updateState updates the validation state based on the event
func (v *EventValidator) updateState(event Event) {
	v.state.mutex.Lock()
	defer v.state.mutex.Unlock()
	
	v.state.EventCount++
	v.state.LastEventTime = time.Now()
	
	switch event.Type() {
	case EventTypeRunStarted:
		if runEvent, ok := event.(*RunStartedEvent); ok {
			v.state.CurrentPhase = PhaseRunning
			v.state.ActiveRuns[runEvent.RunID] = &RunState{
				RunID:     runEvent.RunID,
				ThreadID:  runEvent.ThreadID,
				StartTime: time.Now(),
				Phase:     PhaseRunning,
			}
		}
		
	case EventTypeRunFinished:
		if runEvent, ok := event.(*RunFinishedEvent); ok {
			v.state.CurrentPhase = PhaseFinished
			if runState, exists := v.state.ActiveRuns[runEvent.RunID]; exists {
				runState.Phase = PhaseFinished
				v.state.FinishedRuns[runEvent.RunID] = runState
				delete(v.state.ActiveRuns, runEvent.RunID)
			}
		}
		
	case EventTypeRunError:
		if runEvent, ok := event.(*RunErrorEvent); ok {
			v.state.CurrentPhase = PhaseError
			if runState, exists := v.state.ActiveRuns[runEvent.RunID]; exists {
				runState.Phase = PhaseError
				v.state.FinishedRuns[runEvent.RunID] = runState
				delete(v.state.ActiveRuns, runEvent.RunID)
			}
		}
		
	case EventTypeStepStarted:
		if stepEvent, ok := event.(*StepStartedEvent); ok {
			v.state.ActiveSteps[stepEvent.StepName] = true
			// Update step count for active runs
			for _, runState := range v.state.ActiveRuns {
				runState.StepCount++
			}
		}
		
	case EventTypeStepFinished:
		if stepEvent, ok := event.(*StepFinishedEvent); ok {
			delete(v.state.ActiveSteps, stepEvent.StepName)
		}
		
	case EventTypeTextMessageStart:
		if msgEvent, ok := event.(*TextMessageStartEvent); ok {
			parentMsgID := ""
			// TextMessageStartEvent doesn't have ParentMessageID field
			v.state.ActiveMessages[msgEvent.MessageID] = &MessageState{
				MessageID:    msgEvent.MessageID,
				ParentMsgID:  parentMsgID,
				StartTime:    time.Now(),
				ContentCount: 0,
				IsActive:     true,
			}
		}
		
	case EventTypeTextMessageContent:
		if msgEvent, ok := event.(*TextMessageContentEvent); ok {
			if msgState, exists := v.state.ActiveMessages[msgEvent.MessageID]; exists {
				msgState.ContentCount++
			}
		}
		
	case EventTypeTextMessageEnd:
		if msgEvent, ok := event.(*TextMessageEndEvent); ok {
			if msgState, exists := v.state.ActiveMessages[msgEvent.MessageID]; exists {
				msgState.IsActive = false
				v.state.FinishedMessages[msgEvent.MessageID] = msgState
				delete(v.state.ActiveMessages, msgEvent.MessageID)
			}
		}
		
	case EventTypeToolCallStart:
		if toolEvent, ok := event.(*ToolCallStartEvent); ok {
			parentMsgID := ""
			if toolEvent.ParentMessageID != nil {
				parentMsgID = *toolEvent.ParentMessageID
			}
			v.state.ActiveTools[toolEvent.ToolCallID] = &ToolState{
				ToolCallID:  toolEvent.ToolCallID,
				ParentMsgID: parentMsgID,
				ToolName:    toolEvent.ToolCallName,
				StartTime:   time.Now(),
				ArgsCount:   0,
				IsActive:    true,
			}
		}
		
	case EventTypeToolCallArgs:
		if toolEvent, ok := event.(*ToolCallArgsEvent); ok {
			if toolState, exists := v.state.ActiveTools[toolEvent.ToolCallID]; exists {
				toolState.ArgsCount++
			}
		}
		
	case EventTypeToolCallEnd:
		if toolEvent, ok := event.(*ToolCallEndEvent); ok {
			if toolState, exists := v.state.ActiveTools[toolEvent.ToolCallID]; exists {
				toolState.IsActive = false
				v.state.FinishedTools[toolEvent.ToolCallID] = toolState
				delete(v.state.ActiveTools, toolEvent.ToolCallID)
			}
		}
	}
}

// ValidateEventParallel validates a single event using parallel rule execution when possible.
// This method provides improved CPU utilization by executing independent validation rules concurrently.
// 
// Rules are automatically analyzed for dependencies and grouped into:
// - Independent rules: executed in parallel using worker goroutines
// - Dependent rules: executed sequentially to maintain state consistency
//
// The method falls back to sequential execution when:
// - Parallel execution is disabled in configuration
// - The number of rules is below the threshold for parallel execution
// - Context cancellation is requested
//
// Example:
//
//	// Enable parallel validation with custom configuration
//	parallelConfig := &ParallelValidationConfig{
//		MaxGoroutines: 4,
//		EnableParallelExecution: true,
//		MinRulesForParallel: 3,
//		StopOnFirstError: false,
//	}
//	validator.SetParallelConfig(parallelConfig)
//	
//	result := validator.ValidateEventParallel(ctx, event)
//	fmt.Printf("Used %d goroutines, speedup: %.2fx\n", 
//		result.GoroutinesUsed, 
//		float64(result.SequentialExecutionTime)/float64(result.ParallelExecutionTime))
func (v *EventValidator) ValidateEventParallel(ctx context.Context, event Event) *ParallelValidationResult {
	ctx, span := v.tracer.Start(ctx, "validator.ValidateEventParallel")
	defer span.End()

	start := time.Now()
	
	if event == nil {
		span.SetStatus(codes.Error, "event is nil")
		span.RecordError(fmt.Errorf("event cannot be nil"))
		
		result := &ParallelValidationResult{
			ValidationResult: &ValidationResult{
				IsValid:   false,
				Errors:    []*ValidationError{{
					RuleID:    "NULL_EVENT",
					Message:   "Event cannot be nil",
					Severity:  ValidationSeverityError,
					Timestamp: time.Now(),
				}},
				Warnings:   make([]*ValidationError, 0),
				Information: make([]*ValidationError, 0),
				EventCount: 1,
				Timestamp:  time.Now(),
			},
			GoroutinesUsed: 1,
		}
		result.Duration = time.Since(start)
		return result
	}

	// Add tracing attributes
	span.SetAttributes(
		attribute.String("event.type", string(event.Type())),
		attribute.Int("rules.total", len(v.rules)),
	)

	// Create a snapshot of the current state for validation
	stateSnapshot := v.createStateSnapshot()
	
	// Create validation context with the state snapshot
	validationContext := &ValidationContext{
		State:        stateSnapshot,
		CurrentEvent: event,
		EventIndex:   0,
		Config:       v.config,
		Metadata:     make(map[string]interface{}),
		Context:      ctx,
	}

	// Get a copy of rules for thread safety
	v.mutex.RLock()
	rules := make([]ValidationRule, len(v.rules))
	copy(rules, v.rules)
	v.mutex.RUnlock()

	// Execute parallel validation
	result := v.parallelValidator.ValidateEventParallel(ctx, event, rules, validationContext)
	
	// Add tracing attributes for results
	span.SetAttributes(
		attribute.Int("validation.errors", len(result.Errors)),
		attribute.Int("validation.warnings", len(result.Warnings)),
		attribute.Int("rules.parallel", result.RulesExecutedInParallel),
		attribute.Int("rules.sequential", result.RulesExecutedSequentially),
		attribute.Int("goroutines.used", result.GoroutinesUsed),
		attribute.Bool("validation.valid", result.IsValid),
	)

	if !result.IsValid {
		span.SetStatus(codes.Error, "validation failed")
		for _, err := range result.Errors {
			span.RecordError(fmt.Errorf("%s: %s", err.RuleID, err.Message))
		}
	}

	// Update state only after successful validation
	if result.IsValid {
		v.updateState(event)
	}

	return result
}

// SetParallelConfig updates the parallel validation configuration
func (v *EventValidator) SetParallelConfig(config *ParallelValidationConfig) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	
	if config != nil {
		v.parallelConfig = config
		v.parallelValidator.UpdateConfig(config)
	}
}

// GetParallelConfig returns the current parallel validation configuration
func (v *EventValidator) GetParallelConfig() *ParallelValidationConfig {
	v.mutex.RLock()
	defer v.mutex.RUnlock()
	
	// Return a copy to prevent external modification
	configCopy := *v.parallelConfig
	return &configCopy
}

// GetParallelMetrics returns metrics about parallel validation performance
func (v *EventValidator) GetParallelMetrics() *ParallelValidationMetrics {
	return v.parallelValidator.GetMetrics()
}

// EnableParallelValidation enables or disables parallel validation
func (v *EventValidator) EnableParallelValidation(enabled bool) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	
	v.parallelConfig.EnableParallelExecution = enabled
	v.parallelValidator.UpdateConfig(v.parallelConfig)
}

// IsParallelValidationEnabled returns whether parallel validation is currently enabled
func (v *EventValidator) IsParallelValidationEnabled() bool {
	v.mutex.RLock()
	defer v.mutex.RUnlock()
	
	return v.parallelConfig.EnableParallelExecution
}

// SetMaxGoroutines sets the maximum number of goroutines for parallel validation
func (v *EventValidator) SetMaxGoroutines(maxGoroutines int) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	
	v.parallelConfig.MaxGoroutines = maxGoroutines
	v.parallelValidator.UpdateConfig(v.parallelConfig)
}

// GetMaxGoroutines returns the current maximum number of goroutines for parallel validation
func (v *EventValidator) GetMaxGoroutines() int {
	v.mutex.RLock()
	defer v.mutex.RUnlock()
	
	return v.parallelConfig.MaxGoroutines
}