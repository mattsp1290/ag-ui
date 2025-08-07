package events

import (
	"context"
	"fmt"
	"time"
)

// TypedValidationError represents a validation error with typed event context
type TypedValidationError[T EventDataType] struct {
	RuleID      string                 `json:"rule_id"`
	EventID     string                 `json:"event_id,omitempty"`
	EventType   EventType              `json:"event_type"`
	Message     string                 `json:"message"`
	Severity    ValidationSeverity     `json:"severity"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	EventData   T                      `json:"event_data,omitempty"`
}

func (e *TypedValidationError[T]) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Severity, e.RuleID, e.Message)
}

// TypedValidationResult represents the result of typed event validation
type TypedValidationResult[T EventDataType] struct {
	IsValid     bool                       `json:"is_valid"`
	Errors      []*TypedValidationError[T] `json:"errors,omitempty"`
	Warnings    []*TypedValidationError[T] `json:"warnings,omitempty"`
	Information []*TypedValidationError[T] `json:"information,omitempty"`
	EventCount  int                        `json:"event_count"`
	Duration    time.Duration              `json:"duration"`
	Timestamp   time.Time                  `json:"timestamp"`
}

// HasErrors returns true if there are any validation errors
func (r *TypedValidationResult[T]) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if there are any validation warnings
func (r *TypedValidationResult[T]) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// AddError adds a validation error to the result
func (r *TypedValidationResult[T]) AddError(err *TypedValidationError[T]) {
	r.Errors = append(r.Errors, err)
	r.IsValid = false
}

// AddWarning adds a validation warning to the result
func (r *TypedValidationResult[T]) AddWarning(warning *TypedValidationError[T]) {
	r.Warnings = append(r.Warnings, warning)
}

// AddInfo adds a validation info to the result
func (r *TypedValidationResult[T]) AddInfo(info *TypedValidationError[T]) {
	r.Information = append(r.Information, info)
}

// ToLegacyResult converts to legacy ValidationResult
func (r *TypedValidationResult[T]) ToLegacyResult() *ValidationResult {
	result := &ValidationResult{
		IsValid:    r.IsValid,
		EventCount: r.EventCount,
		Duration:   r.Duration,
		Timestamp:  r.Timestamp,
	}

	// Convert errors
	for _, err := range r.Errors {
		legacyErr := &ValidationError{
			RuleID:      err.RuleID,
			EventID:     err.EventID,
			EventType:   err.EventType,
			Message:     err.Message,
			Severity:    err.Severity,
			Context:     err.Context,
			Suggestions: err.Suggestions,
			Timestamp:   err.Timestamp,
		}
		result.Errors = append(result.Errors, legacyErr)
	}

	// Convert warnings
	for _, warning := range r.Warnings {
		legacyWarning := &ValidationError{
			RuleID:      warning.RuleID,
			EventID:     warning.EventID,
			EventType:   warning.EventType,
			Message:     warning.Message,
			Severity:    warning.Severity,
			Context:     warning.Context,
			Suggestions: warning.Suggestions,
			Timestamp:   warning.Timestamp,
		}
		result.Warnings = append(result.Warnings, legacyWarning)
	}

	// Convert information
	for _, info := range r.Information {
		legacyInfo := &ValidationError{
			RuleID:      info.RuleID,
			EventID:     info.EventID,
			EventType:   info.EventType,
			Message:     info.Message,
			Severity:    info.Severity,
			Context:     info.Context,
			Suggestions: info.Suggestions,
			Timestamp:   info.Timestamp,
		}
		result.Information = append(result.Information, legacyInfo)
	}

	return result
}

// TypedValidationRule defines an interface for typed validation rules
type TypedValidationRule[T EventDataType] interface {
	// ID returns the unique identifier for this rule
	ID() string

	// Description returns a human-readable description of the rule
	Description() string

	// Validate validates a typed event against this rule
	Validate(event TypedEvent[T], context *TypedValidationContext[T]) *TypedValidationResult[T]

	// IsEnabled returns whether this rule is enabled
	IsEnabled() bool

	// SetEnabled enables or disables this rule
	SetEnabled(enabled bool)

	// GetSeverity returns the severity level for violations of this rule
	GetSeverity() ValidationSeverity

	// SetSeverity sets the severity level for violations of this rule
	SetSeverity(severity ValidationSeverity)

	// SupportedEventTypes returns the event types this rule can validate
	SupportedEventTypes() []EventType
}

// TypedBaseValidationRule provides common functionality for typed validation rules
type TypedBaseValidationRule[T EventDataType] struct {
	*BaseValidationRule
	supportedTypes []EventType
}

// NewTypedBaseValidationRule creates a new typed base validation rule
func NewTypedBaseValidationRule[T EventDataType](
	id, description string,
	severity ValidationSeverity,
	supportedTypes []EventType,
) *TypedBaseValidationRule[T] {
	return &TypedBaseValidationRule[T]{
		BaseValidationRule: NewBaseValidationRule(id, description, severity),
		supportedTypes:     supportedTypes,
	}
}

// SupportedEventTypes returns the event types this rule can validate
func (r *TypedBaseValidationRule[T]) SupportedEventTypes() []EventType {
	return r.supportedTypes
}

// CreateTypedError creates a typed validation error for this rule
func (r *TypedBaseValidationRule[T]) CreateTypedError(
	event TypedEvent[T],
	message string,
	context map[string]interface{},
	suggestions []string,
) *TypedValidationError[T] {
	eventID := ""
	// Try to extract ID if the event has one
	if event != nil {
		eventID = event.ID()
	}

	return &TypedValidationError[T]{
		RuleID:      r.ID(),
		EventID:     eventID,
		EventType:   event.Type(),
		Message:     message,
		Severity:    r.GetSeverity(),
		Context:     context,
		Suggestions: suggestions,
		Timestamp:   time.Now(),
		EventData:   event.TypedData(),
	}
}

// TypedValidationContext provides context for typed validation operations
type TypedValidationContext[T EventDataType] struct {
	State         *ValidationState       `json:"state"`
	EventSequence []TypedEvent[T]        `json:"event_sequence,omitempty"`
	CurrentEvent  TypedEvent[T]          `json:"current_event,omitempty"`
	EventIndex    int                    `json:"event_index"`
	Config        *ValidationConfig      `json:"config,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ToLegacyContext converts to legacy ValidationContext
func (c *TypedValidationContext[T]) ToLegacyContext() *ValidationContext {
	legacyContext := &ValidationContext{
		State:      c.State,
		EventIndex: c.EventIndex,
		Config:     c.Config,
		Metadata:   c.Metadata,
	}

	// Convert current event if present
	if c.CurrentEvent != nil {
		legacyContext.CurrentEvent = c.CurrentEvent.ToLegacyEvent()
	}

	// Convert event sequence
	if len(c.EventSequence) > 0 {
		legacyContext.EventSequence = make([]Event, len(c.EventSequence))
		for i, typedEvent := range c.EventSequence {
			legacyContext.EventSequence[i] = typedEvent.ToLegacyEvent()
		}
	}

	return legacyContext
}

// TypedEventValidator provides comprehensive typed event validation
type TypedEventValidator[T EventDataType] struct {
	rules   []TypedValidationRule[T]
	state   *ValidationState
	metrics *ValidationMetrics
	config  *ValidationConfig
	adapter *EventAdapter
}

// NewTypedEventValidator creates a new typed event validator
func NewTypedEventValidator[T EventDataType](config *ValidationConfig) *TypedEventValidator[T] {
	if config == nil {
		config = DefaultValidationConfig()
	}

	return &TypedEventValidator[T]{
		rules:   make([]TypedValidationRule[T], 0),
		state:   NewValidationState(),
		metrics: NewValidationMetrics(),
		config:  config,
		adapter: &EventAdapter{},
	}
}

// AddRule adds a typed validation rule to the validator
func (v *TypedEventValidator[T]) AddRule(rule TypedValidationRule[T]) {
	v.rules = append(v.rules, rule)
}

// RemoveRule removes a typed validation rule by ID
func (v *TypedEventValidator[T]) RemoveRule(ruleID string) bool {
	for i, rule := range v.rules {
		if rule.ID() == ruleID {
			v.rules = append(v.rules[:i], v.rules[i+1:]...)
			return true
		}
	}
	return false
}

// GetRule gets a typed validation rule by ID
func (v *TypedEventValidator[T]) GetRule(ruleID string) TypedValidationRule[T] {
	for _, rule := range v.rules {
		if rule.ID() == ruleID {
			return rule
		}
	}
	return nil
}

// ValidateTypedEvent validates a single typed event
func (v *TypedEventValidator[T]) ValidateTypedEvent(
	ctx context.Context,
	event TypedEvent[T],
) *TypedValidationResult[T] {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		v.metrics.RecordEvent(duration)
	}()

	result := &TypedValidationResult[T]{
		IsValid:    true,
		Errors:     make([]*TypedValidationError[T], 0),
		Warnings:   make([]*TypedValidationError[T], 0),
		EventCount: 1,
		Timestamp:  time.Now(),
	}

	// Check context before starting
	select {
	case <-ctx.Done():
		result.IsValid = false
		result.AddError(&TypedValidationError[T]{
			RuleID:    "CONTEXT_CANCELLED",
			Message:   "Validation cancelled by context",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		})
		result.Duration = time.Since(start)
		return result
	default:
	}

	if event == nil {
		result.AddError(&TypedValidationError[T]{
			RuleID:    "NULL_EVENT",
			Message:   "Event cannot be nil",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		})
		return result
	}

	// Create validation context
	validationContext := &TypedValidationContext[T]{
		State:        v.state,
		CurrentEvent: event,
		EventIndex:   0,
		Config:       v.config,
		Metadata:     make(map[string]interface{}),
	}

	// Apply validation rules
	for _, rule := range v.rules {
		if !rule.IsEnabled() {
			continue
		}

		// Check if rule supports this event type
		supportedTypes := rule.SupportedEventTypes()
		if len(supportedTypes) > 0 {
			supported := false
			for _, supportedType := range supportedTypes {
				if supportedType == event.Type() {
					supported = true
					break
				}
			}
			if !supported {
				continue
			}
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
		// Convert to legacy event and update state using existing logic
		legacyEvent := event.ToLegacyEvent()
		v.updateTypedState(legacyEvent)
	}

	result.Duration = time.Since(start)
	return result
}

// ValidateTypedSequence validates a sequence of typed events
func (v *TypedEventValidator[T]) ValidateTypedSequence(
	ctx context.Context,
	events []TypedEvent[T],
) *TypedValidationResult[T] {
	start := time.Now()

	result := &TypedValidationResult[T]{
		IsValid:    true,
		Errors:     make([]*TypedValidationError[T], 0),
		Warnings:   make([]*TypedValidationError[T], 0),
		EventCount: len(events),
		Timestamp:  time.Now(),
	}

	if len(events) == 0 {
		result.Duration = time.Since(start)
		return result
	}

	// Create validation context for sequence
	validationContext := &TypedValidationContext[T]{
		State:         v.state,
		EventSequence: events,
		Config:        v.config,
		Metadata:      make(map[string]interface{}),
	}

	// Validate each event in sequence
	for i, event := range events {
		// Check context periodically during long sequences
		if i > 0 && i%DefaultBatchCheckInterval == 0 {
			select {
			case <-ctx.Done():
				result.IsValid = false
				result.AddError(&TypedValidationError[T]{
					RuleID:    "CONTEXT_CANCELLED",
					Message:   fmt.Sprintf("Validation cancelled after %d events", i),
					Severity:  ValidationSeverityError,
					Timestamp: time.Now(),
				})
				result.Duration = time.Since(start)
				return result
			default:
			}
		}

		validationContext.CurrentEvent = event
		validationContext.EventIndex = i

		// Validate the event
		eventResult := v.validateTypedEventWithContext(ctx, event, validationContext)

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
	}

	result.Duration = time.Since(start)
	return result
}

// validateTypedEventWithContext validates a typed event with a specific validation context
func (v *TypedEventValidator[T]) validateTypedEventWithContext(
	ctx context.Context,
	event TypedEvent[T],
	validationContext *TypedValidationContext[T],
) *TypedValidationResult[T] {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		v.metrics.RecordEvent(duration)
	}()

	result := &TypedValidationResult[T]{
		IsValid:    true,
		Errors:     make([]*TypedValidationError[T], 0),
		Warnings:   make([]*TypedValidationError[T], 0),
		EventCount: 1,
		Timestamp:  time.Now(),
	}

	if event == nil {
		result.AddError(&TypedValidationError[T]{
			RuleID:    "NULL_EVENT",
			Message:   "Event cannot be nil",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		})
		return result
	}

	// Apply validation rules using the provided context
	for _, rule := range v.rules {
		if !rule.IsEnabled() {
			continue
		}

		// Check if rule supports this event type
		supportedTypes := rule.SupportedEventTypes()
		if len(supportedTypes) > 0 {
			supported := false
			for _, supportedType := range supportedTypes {
				if supportedType == event.Type() {
					supported = true
					break
				}
			}
			if !supported {
				continue
			}
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
		legacyEvent := event.ToLegacyEvent()
		v.updateTypedState(legacyEvent)
	}

	result.Duration = time.Since(start)
	return result
}

// updateTypedState updates the validation state using legacy event logic
func (v *TypedEventValidator[T]) updateTypedState(legacyEvent Event) {
	// Delegate to existing updateState logic from validator.go
	// This maintains compatibility with existing state management
	v.state.mutex.Lock()
	defer v.state.mutex.Unlock()

	v.state.EventCount++
	v.state.LastEventTime = time.Now()

	// Use the same logic as the legacy validator's updateState method
	// This ensures consistency between typed and legacy validation
	switch legacyEvent.Type() {
	case EventTypeRunStarted:
		if runEvent, ok := legacyEvent.(*RunStartedEvent); ok {
			v.state.CurrentPhase = PhaseRunning
			v.state.ActiveRuns[runEvent.RunID()] = &RunState{
				RunID:     runEvent.RunID(),
				ThreadID:  runEvent.ThreadID(),
				StartTime: time.Now(),
				Phase:     PhaseRunning,
			}
		}

	case EventTypeRunFinished:
		if runEvent, ok := legacyEvent.(*RunFinishedEvent); ok {
			v.state.CurrentPhase = PhaseFinished
			if runState, exists := v.state.ActiveRuns[runEvent.RunID()]; exists {
				runState.Phase = PhaseFinished
				v.state.FinishedRuns[runEvent.RunID()] = runState
				delete(v.state.ActiveRuns, runEvent.RunID())
			}
		}

	case EventTypeRunError:
		if runEvent, ok := legacyEvent.(*RunErrorEvent); ok {
			v.state.CurrentPhase = PhaseError
			if runState, exists := v.state.ActiveRuns[runEvent.RunID()]; exists {
				runState.Phase = PhaseError
				v.state.FinishedRuns[runEvent.RunID()] = runState
				delete(v.state.ActiveRuns, runEvent.RunID())
			}
		}

		// Add other event types as needed...
		// This follows the same pattern as the legacy updateState method
	}
}

// ToLegacyValidator converts to a legacy EventValidator for backward compatibility
func (v *TypedEventValidator[T]) ToLegacyValidator() *EventValidator {
	legacyValidator := &EventValidator{
		rules:   make([]ValidationRule, 0),
		state:   v.state,
		metrics: v.metrics,
		config:  v.config,
	}

	// Convert typed rules to legacy rules (this would require a wrapper)
	// For now, we'll just return the validator with shared state
	return legacyValidator
}

// ValidateLegacyEvent validates a legacy event using typed validation when possible
func (v *TypedEventValidator[T]) ValidateLegacyEvent(
	ctx context.Context,
	legacyEvent Event,
) *ValidationResult {
	// Try to convert to typed event
	typedEventInterface, err := v.adapter.ToTypedEvent(legacyEvent)
	if err != nil {
		// Fall back to basic validation
		result := &ValidationResult{
			IsValid: false,
			Errors: []*ValidationError{{
				RuleID:    "CONVERSION_ERROR",
				Message:   fmt.Sprintf("Failed to convert to typed event: %v", err),
				Severity:  ValidationSeverityError,
				Timestamp: time.Now(),
			}},
			EventCount: 1,
			Timestamp:  time.Now(),
		}
		return result
	}

	// Check if it's the correct type
	if typedEvent, ok := typedEventInterface.(TypedEvent[T]); ok {
		typedResult := v.ValidateTypedEvent(ctx, typedEvent)
		return typedResult.ToLegacyResult()
	}

	// If types don't match, return a basic validation
	result := &ValidationResult{
		IsValid: false,
		Errors: []*ValidationError{{
			RuleID:    "TYPE_MISMATCH",
			Message:   "Event type does not match validator type parameter",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		}},
		EventCount: 1,
		Timestamp:  time.Now(),
	}
	return result
}
