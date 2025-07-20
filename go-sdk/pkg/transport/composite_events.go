package transport

import (
	"fmt"
	"time"
)

// Composite event types that combine multiple events or add advanced functionality
// These provide sophisticated event handling patterns for complex scenarios

// CompositeEvent represents a transport event that contains multiple child events
type CompositeEvent struct {
	BaseEvent SimpleTransportEvent
	Events    []TransportEvent
}

// ID returns the composite event ID
func (e *CompositeEvent) ID() string {
	return e.BaseEvent.ID()
}

// Type returns the composite event type
func (e *CompositeEvent) Type() string {
	return e.BaseEvent.Type()
}

// Timestamp returns the composite event timestamp
func (e *CompositeEvent) Timestamp() time.Time {
	return e.BaseEvent.Timestamp()
}

// Data returns the composite event data including child events
func (e *CompositeEvent) Data() map[string]interface{} {
	data := e.BaseEvent.Data()
	if data == nil {
		data = make(map[string]interface{})
	}
	
	// Add child event count
	data["event_count"] = len(e.Events)
	
	// Add child event IDs
	childIDs := make([]string, len(e.Events))
	for i, event := range e.Events {
		childIDs[i] = event.ID()
	}
	data["child_event_ids"] = childIDs
	
	return data
}

// BatchEvent represents a collection of events processed together
type BatchEvent[T EventData] struct {
	// BatchID is the unique identifier for this batch
	BatchID string `json:"batch_id"`
	
	// Events contains the events in this batch
	Events []TypedTransportEvent[T] `json:"events"`
	
	// BatchSize is the number of events in this batch
	BatchSize int `json:"batch_size"`
	
	// MaxBatchSize is the maximum allowed batch size
	MaxBatchSize int `json:"max_batch_size"`
	
	// CreatedAt when the batch was created
	CreatedAt time.Time `json:"created_at"`
	
	// CompletedAt when the batch processing completed (nil if not completed)
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	
	// ProcessingDuration how long the batch took to process
	ProcessingDuration time.Duration `json:"processing_duration,omitempty"`
	
	// Status indicates the batch status (pending, processing, completed, failed)
	Status string `json:"status"`
	
	// Errors contains any processing errors
	Errors []error `json:"errors,omitempty"`
	
	// SuccessCount number of successfully processed events
	SuccessCount int `json:"success_count"`
	
	// FailureCount number of failed events
	FailureCount int `json:"failure_count"`
	
	// Metadata contains additional batch metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	
	// ProcessorID identifies the processor that handled this batch
	ProcessorID string `json:"processor_id,omitempty"`
	
	// Priority indicates batch processing priority
	Priority int `json:"priority,omitempty"`
	
	// RetryAttempt for failed batch retries
	RetryAttempt int `json:"retry_attempt,omitempty"`
	
	// MaxRetries maximum number of retry attempts
	MaxRetries int `json:"max_retries,omitempty"`
}

// Validate ensures the batch event data is valid
func (b BatchEvent[T]) Validate() error {
	if b.BatchID == "" {
		return NewValidationError("batch_id is required", nil)
	}
	if len(b.Events) == 0 {
		return NewValidationError("events cannot be empty", nil)
	}
	if b.BatchSize != len(b.Events) {
		return NewValidationError("batch_size must match events length", nil)
	}
	if b.MaxBatchSize > 0 && b.BatchSize > b.MaxBatchSize {
		return NewValidationError("batch_size exceeds max_batch_size", nil)
	}
	if b.CreatedAt.IsZero() {
		return NewValidationError("created_at is required", nil)
	}
	if b.Status == "" {
		return NewValidationError("status is required", nil)
	}
	if b.SuccessCount+b.FailureCount > b.BatchSize {
		return NewValidationError("success_count + failure_count cannot exceed batch_size", nil)
	}
	
	// Validate individual events
	for i, event := range b.Events {
		if err := event.TypedData().Validate(); err != nil {
			return fmt.Errorf("event[%d] validation failed: %w", i, err)
		}
	}
	
	return nil
}

// ToMap converts the batch event data to a map for backward compatibility
func (b BatchEvent[T]) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["batch_id"] = b.BatchID
	result["batch_size"] = b.BatchSize
	result["max_batch_size"] = b.MaxBatchSize
	result["created_at"] = b.CreatedAt
	result["status"] = b.Status
	result["success_count"] = b.SuccessCount
	result["failure_count"] = b.FailureCount
	
	// Convert events to maps
	eventMaps := make([]map[string]interface{}, len(b.Events))
	for i, event := range b.Events {
		eventMaps[i] = event.Data()
	}
	result["events"] = eventMaps
	
	if b.CompletedAt != nil {
		result["completed_at"] = b.CompletedAt
	}
	if b.ProcessingDuration > 0 {
		result["processing_duration"] = b.ProcessingDuration.String()
	}
	if len(b.Errors) > 0 {
		errorStrings := make([]string, len(b.Errors))
		for i, err := range b.Errors {
			errorStrings[i] = err.Error()
		}
		result["errors"] = errorStrings
	}
	if b.Metadata != nil {
		result["metadata"] = b.Metadata
	}
	if b.ProcessorID != "" {
		result["processor_id"] = b.ProcessorID
	}
	if b.Priority != 0 {
		result["priority"] = b.Priority
	}
	if b.RetryAttempt > 0 {
		result["retry_attempt"] = b.RetryAttempt
	}
	if b.MaxRetries > 0 {
		result["max_retries"] = b.MaxRetries
	}
	
	return result
}

// SequencedEvent represents an event with sequence ordering
type SequencedEvent[T EventData] struct {
	// SequenceID identifies the sequence this event belongs to
	SequenceID string `json:"sequence_id"`
	
	// SequenceNumber is the position in the sequence (1-based)
	SequenceNumber uint64 `json:"sequence_number"`
	
	// TotalInSequence is the total number of events in this sequence (if known)
	TotalInSequence *uint64 `json:"total_in_sequence,omitempty"`
	
	// Event is the wrapped event data
	Event TypedTransportEvent[T] `json:"event"`
	
	// PreviousSequenceNumber for gap detection
	PreviousSequenceNumber *uint64 `json:"previous_sequence_number,omitempty"`
	
	// NextExpectedSequenceNumber for ordering validation
	NextExpectedSequenceNumber *uint64 `json:"next_expected_sequence_number,omitempty"`
	
	// IsLast indicates if this is the last event in the sequence
	IsLast bool `json:"is_last"`
	
	// IsFirst indicates if this is the first event in the sequence
	IsFirst bool `json:"is_first"`
	
	// ChecksumPrevious for integrity checking with previous event
	ChecksumPrevious string `json:"checksum_previous,omitempty"`
	
	// ChecksumCurrent for integrity checking
	ChecksumCurrent string `json:"checksum_current"`
	
	// Dependencies lists sequence numbers this event depends on
	Dependencies []uint64 `json:"dependencies,omitempty"`
	
	// PartitionKey for partitioned sequences
	PartitionKey string `json:"partition_key,omitempty"`
	
	// OrderingKey for sub-sequence ordering
	OrderingKey string `json:"ordering_key,omitempty"`
	
	// Timeout for sequence completion
	Timeout time.Duration `json:"timeout,omitempty"`
	
	// CreatedAt when this sequenced event was created
	CreatedAt time.Time `json:"created_at"`
}

// Validate ensures the sequenced event data is valid
func (s SequencedEvent[T]) Validate() error {
	if s.SequenceID == "" {
		return NewValidationError("sequence_id is required", nil)
	}
	if s.SequenceNumber == 0 {
		return NewValidationError("sequence_number must be greater than 0", nil)
	}
	if s.Event == nil {
		return NewValidationError("event is required", nil)
	}
	if s.ChecksumCurrent == "" {
		return NewValidationError("checksum_current is required", nil)
	}
	if s.CreatedAt.IsZero() {
		return NewValidationError("created_at is required", nil)
	}
	
	// Validate wrapped event
	if err := s.Event.TypedData().Validate(); err != nil {
		return fmt.Errorf("wrapped event validation failed: %w", err)
	}
	
	// Validate sequence consistency
	if s.TotalInSequence != nil && s.SequenceNumber > *s.TotalInSequence {
		return NewValidationError("sequence_number cannot exceed total_in_sequence", nil)
	}
	
	if s.IsFirst && s.SequenceNumber != 1 {
		return NewValidationError("first event must have sequence_number 1", nil)
	}
	
	if s.IsLast && s.TotalInSequence != nil && s.SequenceNumber != *s.TotalInSequence {
		return NewValidationError("last event sequence_number must match total_in_sequence", nil)
	}
	
	return nil
}

// ToMap converts the sequenced event data to a map for backward compatibility
func (s SequencedEvent[T]) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["sequence_id"] = s.SequenceID
	result["sequence_number"] = s.SequenceNumber
	result["event"] = s.Event.Data()
	result["is_last"] = s.IsLast
	result["is_first"] = s.IsFirst
	result["checksum_current"] = s.ChecksumCurrent
	result["created_at"] = s.CreatedAt
	
	if s.TotalInSequence != nil {
		result["total_in_sequence"] = *s.TotalInSequence
	}
	if s.PreviousSequenceNumber != nil {
		result["previous_sequence_number"] = *s.PreviousSequenceNumber
	}
	if s.NextExpectedSequenceNumber != nil {
		result["next_expected_sequence_number"] = *s.NextExpectedSequenceNumber
	}
	if s.ChecksumPrevious != "" {
		result["checksum_previous"] = s.ChecksumPrevious
	}
	if len(s.Dependencies) > 0 {
		result["dependencies"] = s.Dependencies
	}
	if s.PartitionKey != "" {
		result["partition_key"] = s.PartitionKey
	}
	if s.OrderingKey != "" {
		result["ordering_key"] = s.OrderingKey
	}
	if s.Timeout > 0 {
		result["timeout"] = s.Timeout.String()
	}
	
	return result
}

// ConditionalEvent represents an event with conditional logic
type ConditionalEvent[T EventData] struct {
	// ConditionID identifies the condition to evaluate
	ConditionID string `json:"condition_id"`
	
	// Event is the wrapped event that may be processed conditionally
	Event TypedTransportEvent[T] `json:"event"`
	
	// Condition defines the condition to evaluate
	Condition *EventCondition `json:"condition"`
	
	// IsConditionMet indicates if the condition has been evaluated and met
	IsConditionMet *bool `json:"is_condition_met,omitempty"`
	
	// EvaluatedAt when the condition was last evaluated
	EvaluatedAt *time.Time `json:"evaluated_at,omitempty"`
	
	// EvaluationContext contains context used for condition evaluation
	EvaluationContext map[string]interface{} `json:"evaluation_context,omitempty"`
	
	// AlternativeAction defines what to do if condition is not met
	AlternativeAction string `json:"alternative_action,omitempty"`
	
	// MaxEvaluationAttempts limits how many times condition can be evaluated
	MaxEvaluationAttempts int `json:"max_evaluation_attempts,omitempty"`
	
	// EvaluationAttempts tracks how many times condition has been evaluated
	EvaluationAttempts int `json:"evaluation_attempts"`
	
	// TimeoutAt when the conditional event expires
	TimeoutAt *time.Time `json:"timeout_at,omitempty"`
	
	// Dependencies lists other conditions this condition depends on
	Dependencies []string `json:"dependencies,omitempty"`
	
	// Priority for condition evaluation ordering
	Priority int `json:"priority,omitempty"`
	
	// RetryPolicy for condition evaluation failures
	RetryPolicy *EventRetryPolicy `json:"retry_policy,omitempty"`
}

// EventCondition defines a condition for conditional events
type EventCondition struct {
	// Type indicates the condition type (field_match, event_count, time_window, etc.)
	Type string `json:"type"`
	
	// Expression is the condition expression (varies by type)
	Expression string `json:"expression"`
	
	// Parameters contains condition-specific parameters
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	
	// Operator for comparison conditions (eq, ne, gt, lt, gte, lte, in, not_in, etc.)
	Operator string `json:"operator,omitempty"`
	
	// ExpectedValue for comparison conditions
	ExpectedValue interface{} `json:"expected_value,omitempty"`
	
	// FieldPath for field-based conditions
	FieldPath string `json:"field_path,omitempty"`
	
	// TimeWindow for time-based conditions
	TimeWindow time.Duration `json:"time_window,omitempty"`
	
	// EventTypes for event-type-based conditions
	EventTypes []string `json:"event_types,omitempty"`
	
	// MinCount for count-based conditions
	MinCount *int `json:"min_count,omitempty"`
	
	// MaxCount for count-based conditions
	MaxCount *int `json:"max_count,omitempty"`
}

// EventRetryPolicy defines retry behavior for conditional events
type EventRetryPolicy struct {
	// MaxRetries maximum number of retry attempts
	MaxRetries int `json:"max_retries"`
	
	// InitialDelay before first retry
	InitialDelay time.Duration `json:"initial_delay"`
	
	// MaxDelay maximum delay between retries
	MaxDelay time.Duration `json:"max_delay"`
	
	// BackoffMultiplier for exponential backoff
	BackoffMultiplier float64 `json:"backoff_multiplier"`
	
	// Jitter adds randomness to delay
	Jitter bool `json:"jitter"`
}

// Validate ensures the conditional event data is valid
func (c ConditionalEvent[T]) Validate() error {
	if c.ConditionID == "" {
		return NewValidationError("condition_id is required", nil)
	}
	if c.Event == nil {
		return NewValidationError("event is required", nil)
	}
	if c.Condition == nil {
		return NewValidationError("condition is required", nil)
	}
	
	// Validate wrapped event
	if err := c.Event.TypedData().Validate(); err != nil {
		return fmt.Errorf("wrapped event validation failed: %w", err)
	}
	
	// Validate condition
	if c.Condition.Type == "" {
		return NewValidationError("condition.type is required", nil)
	}
	if c.Condition.Expression == "" {
		return NewValidationError("condition.expression is required", nil)
	}
	
	// Validate retry policy if present
	if c.RetryPolicy != nil {
		if c.RetryPolicy.MaxRetries < 0 {
			return NewValidationError("retry_policy.max_retries cannot be negative", nil)
		}
		if c.RetryPolicy.BackoffMultiplier < 1.0 {
			return NewValidationError("retry_policy.backoff_multiplier must be >= 1.0", nil)
		}
	}
	
	return nil
}

// ToMap converts the conditional event data to a map for backward compatibility
func (c ConditionalEvent[T]) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["condition_id"] = c.ConditionID
	result["event"] = c.Event.Data()
	result["evaluation_attempts"] = c.EvaluationAttempts
	
	// Convert condition
	conditionMap := map[string]interface{}{
		"type":       c.Condition.Type,
		"expression": c.Condition.Expression,
	}
	if c.Condition.Parameters != nil {
		conditionMap["parameters"] = c.Condition.Parameters
	}
	if c.Condition.Operator != "" {
		conditionMap["operator"] = c.Condition.Operator
	}
	if c.Condition.ExpectedValue != nil {
		conditionMap["expected_value"] = c.Condition.ExpectedValue
	}
	if c.Condition.FieldPath != "" {
		conditionMap["field_path"] = c.Condition.FieldPath
	}
	if c.Condition.TimeWindow > 0 {
		conditionMap["time_window"] = c.Condition.TimeWindow.String()
	}
	if len(c.Condition.EventTypes) > 0 {
		conditionMap["event_types"] = c.Condition.EventTypes
	}
	if c.Condition.MinCount != nil {
		conditionMap["min_count"] = *c.Condition.MinCount
	}
	if c.Condition.MaxCount != nil {
		conditionMap["max_count"] = *c.Condition.MaxCount
	}
	result["condition"] = conditionMap
	
	if c.IsConditionMet != nil {
		result["is_condition_met"] = *c.IsConditionMet
	}
	if c.EvaluatedAt != nil {
		result["evaluated_at"] = c.EvaluatedAt
	}
	if c.EvaluationContext != nil {
		result["evaluation_context"] = c.EvaluationContext
	}
	if c.AlternativeAction != "" {
		result["alternative_action"] = c.AlternativeAction
	}
	if c.MaxEvaluationAttempts > 0 {
		result["max_evaluation_attempts"] = c.MaxEvaluationAttempts
	}
	if c.TimeoutAt != nil {
		result["timeout_at"] = c.TimeoutAt
	}
	if len(c.Dependencies) > 0 {
		result["dependencies"] = c.Dependencies
	}
	if c.Priority != 0 {
		result["priority"] = c.Priority
	}
	if c.RetryPolicy != nil {
		retryMap := map[string]interface{}{
			"max_retries":        c.RetryPolicy.MaxRetries,
			"initial_delay":      c.RetryPolicy.InitialDelay.String(),
			"max_delay":          c.RetryPolicy.MaxDelay.String(),
			"backoff_multiplier": c.RetryPolicy.BackoffMultiplier,
			"jitter":             c.RetryPolicy.Jitter,
		}
		result["retry_policy"] = retryMap
	}
	
	return result
}

// TimedEvent represents an event with timing constraints
type TimedEvent[T EventData] struct {
	// TimerID identifies the timer associated with this event
	TimerID string `json:"timer_id"`
	
	// Event is the wrapped event
	Event TypedTransportEvent[T] `json:"event"`
	
	// ScheduledAt when the event should be processed
	ScheduledAt time.Time `json:"scheduled_at"`
	
	// CreatedAt when the timed event was created
	CreatedAt time.Time `json:"created_at"`
	
	// ProcessedAt when the event was actually processed (nil if not yet processed)
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	
	// Delay intended delay from creation to processing
	Delay time.Duration `json:"delay"`
	
	// ActualDelay actual delay experienced
	ActualDelay time.Duration `json:"actual_delay,omitempty"`
	
	// MaxDelay maximum allowed delay before event expires
	MaxDelay time.Duration `json:"max_delay,omitempty"`
	
	// IsExpired indicates if the event has expired
	IsExpired bool `json:"is_expired"`
	
	// ExpiresAt when the event expires
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	
	// IsRecurring indicates if this is a recurring event
	IsRecurring bool `json:"is_recurring"`
	
	// RecurrencePattern for recurring events (cron-like syntax)
	RecurrencePattern string `json:"recurrence_pattern,omitempty"`
	
	// NextScheduledAt when the next occurrence is scheduled (for recurring events)
	NextScheduledAt *time.Time `json:"next_scheduled_at,omitempty"`
	
	// MaxOccurrences limits recurring events (0 = unlimited)
	MaxOccurrences int `json:"max_occurrences,omitempty"`
	
	// OccurrenceCount tracks how many times this event has occurred
	OccurrenceCount int `json:"occurrence_count"`
	
	// TimeZone for schedule calculations
	TimeZone string `json:"time_zone,omitempty"`
	
	// Priority for timing queue ordering
	Priority int `json:"priority,omitempty"`
	
	// OnExpiry action to take when event expires (discard, log, alert, etc.)
	OnExpiry string `json:"on_expiry,omitempty"`
}

// Validate ensures the timed event data is valid
func (t TimedEvent[T]) Validate() error {
	if t.TimerID == "" {
		return NewValidationError("timer_id is required", nil)
	}
	if t.Event == nil {
		return NewValidationError("event is required", nil)
	}
	if t.ScheduledAt.IsZero() {
		return NewValidationError("scheduled_at is required", nil)
	}
	if t.CreatedAt.IsZero() {
		return NewValidationError("created_at is required", nil)
	}
	if t.Delay < 0 {
		return NewValidationError("delay cannot be negative", nil)
	}
	
	// Validate wrapped event
	if err := t.Event.TypedData().Validate(); err != nil {
		return fmt.Errorf("wrapped event validation failed: %w", err)
	}
	
	// Validate scheduling consistency
	expectedScheduledAt := t.CreatedAt.Add(t.Delay)
	if !t.ScheduledAt.Equal(expectedScheduledAt) {
		return NewValidationError("scheduled_at must equal created_at + delay", nil)
	}
	
	// Validate recurring event constraints
	if t.IsRecurring && t.RecurrencePattern == "" {
		return NewValidationError("recurrence_pattern is required for recurring events", nil)
	}
	
	if t.MaxOccurrences > 0 && t.OccurrenceCount > t.MaxOccurrences {
		return NewValidationError("occurrence_count cannot exceed max_occurrences", nil)
	}
	
	return nil
}

// ToMap converts the timed event data to a map for backward compatibility
func (t TimedEvent[T]) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["timer_id"] = t.TimerID
	result["event"] = t.Event.Data()
	result["scheduled_at"] = t.ScheduledAt
	result["created_at"] = t.CreatedAt
	result["delay"] = t.Delay.String()
	result["is_expired"] = t.IsExpired
	result["is_recurring"] = t.IsRecurring
	result["occurrence_count"] = t.OccurrenceCount
	
	if t.ProcessedAt != nil {
		result["processed_at"] = t.ProcessedAt
	}
	if t.ActualDelay > 0 {
		result["actual_delay"] = t.ActualDelay.String()
	}
	if t.MaxDelay > 0 {
		result["max_delay"] = t.MaxDelay.String()
	}
	if t.ExpiresAt != nil {
		result["expires_at"] = t.ExpiresAt
	}
	if t.RecurrencePattern != "" {
		result["recurrence_pattern"] = t.RecurrencePattern
	}
	if t.NextScheduledAt != nil {
		result["next_scheduled_at"] = t.NextScheduledAt
	}
	if t.MaxOccurrences > 0 {
		result["max_occurrences"] = t.MaxOccurrences
	}
	if t.TimeZone != "" {
		result["time_zone"] = t.TimeZone
	}
	if t.Priority != 0 {
		result["priority"] = t.Priority
	}
	if t.OnExpiry != "" {
		result["on_expiry"] = t.OnExpiry
	}
	
	return result
}

// ContextualEvent represents an event with rich context data
type ContextualEvent[T EventData] struct {
	// ContextID identifies the context
	ContextID string `json:"context_id"`
	
	// Event is the wrapped event
	Event TypedTransportEvent[T] `json:"event"`
	
	// Context contains rich contextual information
	Context *EventContext `json:"context"`
	
	// CorrelationID for event correlation across systems
	CorrelationID string `json:"correlation_id,omitempty"`
	
	// CausationID identifies the event that caused this one
	CausationID string `json:"causation_id,omitempty"`
	
	// TraceID for distributed tracing
	TraceID string `json:"trace_id,omitempty"`
	
	// SpanID for distributed tracing
	SpanID string `json:"span_id,omitempty"`
	
	// BusinessContext contains business-specific context
	BusinessContext map[string]interface{} `json:"business_context,omitempty"`
	
	// TechnicalContext contains technical context
	TechnicalContext map[string]interface{} `json:"technical_context,omitempty"`
	
	// UserContext contains user-specific context
	UserContext *UserContext `json:"user_context,omitempty"`
	
	// RequestContext contains request-specific context
	RequestContext *RequestContext `json:"request_context,omitempty"`
	
	// EnvironmentContext contains environment information
	EnvironmentContext *EnvironmentContext `json:"environment_context,omitempty"`
	
	// SecurityContext contains security-related context
	SecurityContext *SecurityContext `json:"security_context,omitempty"`
	
	// Tags for event categorization and filtering
	Tags map[string]string `json:"tags,omitempty"`
	
	// Annotations for additional metadata
	Annotations map[string]string `json:"annotations,omitempty"`
}

// EventContext represents comprehensive event context
type EventContext struct {
	// Timestamp when the context was captured
	Timestamp time.Time `json:"timestamp"`
	
	// Version of the context schema
	Version string `json:"version"`
	
	// Source system or component that generated the event
	Source string `json:"source"`
	
	// SourceVersion version of the source system
	SourceVersion string `json:"source_version,omitempty"`
	
	// Environment (development, staging, production, etc.)
	Environment string `json:"environment"`
	
	// Region or datacenter location
	Region string `json:"region,omitempty"`
	
	// Tenant or organization identifier
	TenantID string `json:"tenant_id,omitempty"`
	
	// ServiceName of the originating service
	ServiceName string `json:"service_name,omitempty"`
	
	// ServiceInstance identifier
	ServiceInstance string `json:"service_instance,omitempty"`
	
	// ProcessID of the generating process
	ProcessID int `json:"process_id,omitempty"`
	
	// ThreadID of the generating thread
	ThreadID string `json:"thread_id,omitempty"`
}

// UserContext represents user-specific context
type UserContext struct {
	// UserID identifies the user
	UserID string `json:"user_id"`
	
	// UserType indicates the type of user (human, service, etc.)
	UserType string `json:"user_type,omitempty"`
	
	// SessionID identifies the user session
	SessionID string `json:"session_id,omitempty"`
	
	// Roles assigned to the user
	Roles []string `json:"roles,omitempty"`
	
	// Permissions granted to the user
	Permissions []string `json:"permissions,omitempty"`
	
	// Groups the user belongs to
	Groups []string `json:"groups,omitempty"`
	
	// ClientInfo about the user's client
	ClientInfo map[string]string `json:"client_info,omitempty"`
	
	// Preferences user preferences relevant to the event
	Preferences map[string]interface{} `json:"preferences,omitempty"`
}

// RequestContext represents request-specific context
type RequestContext struct {
	// RequestID identifies the request
	RequestID string `json:"request_id"`
	
	// Method HTTP method or operation type
	Method string `json:"method,omitempty"`
	
	// URL or endpoint
	URL string `json:"url,omitempty"`
	
	// Headers relevant headers
	Headers map[string]string `json:"headers,omitempty"`
	
	// QueryParams query parameters
	QueryParams map[string]string `json:"query_params,omitempty"`
	
	// UserAgent client user agent
	UserAgent string `json:"user_agent,omitempty"`
	
	// ClientIP client IP address
	ClientIP string `json:"client_ip,omitempty"`
	
	// ContentType request content type
	ContentType string `json:"content_type,omitempty"`
	
	// ContentLength request content length
	ContentLength int64 `json:"content_length,omitempty"`
	
	// Referrer request referrer
	Referrer string `json:"referrer,omitempty"`
}

// EnvironmentContext represents environment information
type EnvironmentContext struct {
	// Hostname of the machine
	Hostname string `json:"hostname,omitempty"`
	
	// Platform (linux, windows, darwin, etc.)
	Platform string `json:"platform,omitempty"`
	
	// Architecture (amd64, arm64, etc.)
	Architecture string `json:"architecture,omitempty"`
	
	// RuntimeVersion (Go version, etc.)
	RuntimeVersion string `json:"runtime_version,omitempty"`
	
	// ContainerID if running in container
	ContainerID string `json:"container_id,omitempty"`
	
	// PodName if running in Kubernetes
	PodName string `json:"pod_name,omitempty"`
	
	// Namespace Kubernetes namespace
	Namespace string `json:"namespace,omitempty"`
	
	// NodeName Kubernetes node
	NodeName string `json:"node_name,omitempty"`
	
	// Environment variables relevant to the event
	EnvironmentVars map[string]string `json:"environment_vars,omitempty"`
}

// SecurityContext represents security-related context
type SecurityContext struct {
	// AuthenticationMethod used
	AuthenticationMethod string `json:"authentication_method,omitempty"`
	
	// TokenType (bearer, api_key, etc.)
	TokenType string `json:"token_type,omitempty"`
	
	// TokenHash hashed token for audit purposes
	TokenHash string `json:"token_hash,omitempty"`
	
	// ClientCertificateHash if using client certificates
	ClientCertificateHash string `json:"client_certificate_hash,omitempty"`
	
	// TLSVersion used for the connection
	TLSVersion string `json:"tls_version,omitempty"`
	
	// CipherSuite used
	CipherSuite string `json:"cipher_suite,omitempty"`
	
	// IsEncrypted indicates if the communication is encrypted
	IsEncrypted bool `json:"is_encrypted"`
	
	// SecurityHeaders relevant security headers
	SecurityHeaders map[string]string `json:"security_headers,omitempty"`
}

// Validate ensures the contextual event data is valid
func (c ContextualEvent[T]) Validate() error {
	if c.ContextID == "" {
		return NewValidationError("context_id is required", nil)
	}
	if c.Event == nil {
		return NewValidationError("event is required", nil)
	}
	if c.Context == nil {
		return NewValidationError("context is required", nil)
	}
	
	// Validate wrapped event
	if err := c.Event.TypedData().Validate(); err != nil {
		return fmt.Errorf("wrapped event validation failed: %w", err)
	}
	
	// Validate context
	if c.Context.Timestamp.IsZero() {
		return NewValidationError("context.timestamp is required", nil)
	}
	if c.Context.Source == "" {
		return NewValidationError("context.source is required", nil)
	}
	if c.Context.Environment == "" {
		return NewValidationError("context.environment is required", nil)
	}
	
	// Validate user context if present
	if c.UserContext != nil && c.UserContext.UserID == "" {
		return NewValidationError("user_context.user_id is required when user_context is provided", nil)
	}
	
	// Validate request context if present
	if c.RequestContext != nil && c.RequestContext.RequestID == "" {
		return NewValidationError("request_context.request_id is required when request_context is provided", nil)
	}
	
	return nil
}

// ToMap converts the contextual event data to a map for backward compatibility
func (c ContextualEvent[T]) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["context_id"] = c.ContextID
	result["event"] = c.Event.Data()
	
	// Convert context
	contextMap := map[string]interface{}{
		"timestamp":   c.Context.Timestamp,
		"source":      c.Context.Source,
		"environment": c.Context.Environment,
	}
	if c.Context.Version != "" {
		contextMap["version"] = c.Context.Version
	}
	if c.Context.SourceVersion != "" {
		contextMap["source_version"] = c.Context.SourceVersion
	}
	if c.Context.Region != "" {
		contextMap["region"] = c.Context.Region
	}
	if c.Context.TenantID != "" {
		contextMap["tenant_id"] = c.Context.TenantID
	}
	if c.Context.ServiceName != "" {
		contextMap["service_name"] = c.Context.ServiceName
	}
	if c.Context.ServiceInstance != "" {
		contextMap["service_instance"] = c.Context.ServiceInstance
	}
	if c.Context.ProcessID > 0 {
		contextMap["process_id"] = c.Context.ProcessID
	}
	if c.Context.ThreadID != "" {
		contextMap["thread_id"] = c.Context.ThreadID
	}
	result["context"] = contextMap
	
	if c.CorrelationID != "" {
		result["correlation_id"] = c.CorrelationID
	}
	if c.CausationID != "" {
		result["causation_id"] = c.CausationID
	}
	if c.TraceID != "" {
		result["trace_id"] = c.TraceID
	}
	if c.SpanID != "" {
		result["span_id"] = c.SpanID
	}
	if c.BusinessContext != nil {
		result["business_context"] = c.BusinessContext
	}
	if c.TechnicalContext != nil {
		result["technical_context"] = c.TechnicalContext
	}
	
	// Convert user context
	if c.UserContext != nil {
		userMap := map[string]interface{}{
			"user_id": c.UserContext.UserID,
		}
		if c.UserContext.UserType != "" {
			userMap["user_type"] = c.UserContext.UserType
		}
		if c.UserContext.SessionID != "" {
			userMap["session_id"] = c.UserContext.SessionID
		}
		if len(c.UserContext.Roles) > 0 {
			userMap["roles"] = c.UserContext.Roles
		}
		if len(c.UserContext.Permissions) > 0 {
			userMap["permissions"] = c.UserContext.Permissions
		}
		if len(c.UserContext.Groups) > 0 {
			userMap["groups"] = c.UserContext.Groups
		}
		if c.UserContext.ClientInfo != nil {
			userMap["client_info"] = c.UserContext.ClientInfo
		}
		if c.UserContext.Preferences != nil {
			userMap["preferences"] = c.UserContext.Preferences
		}
		result["user_context"] = userMap
	}
	
	// Convert request context
	if c.RequestContext != nil {
		requestMap := map[string]interface{}{
			"request_id": c.RequestContext.RequestID,
		}
		if c.RequestContext.Method != "" {
			requestMap["method"] = c.RequestContext.Method
		}
		if c.RequestContext.URL != "" {
			requestMap["url"] = c.RequestContext.URL
		}
		if c.RequestContext.Headers != nil {
			requestMap["headers"] = c.RequestContext.Headers
		}
		if c.RequestContext.QueryParams != nil {
			requestMap["query_params"] = c.RequestContext.QueryParams
		}
		if c.RequestContext.UserAgent != "" {
			requestMap["user_agent"] = c.RequestContext.UserAgent
		}
		if c.RequestContext.ClientIP != "" {
			requestMap["client_ip"] = c.RequestContext.ClientIP
		}
		if c.RequestContext.ContentType != "" {
			requestMap["content_type"] = c.RequestContext.ContentType
		}
		if c.RequestContext.ContentLength > 0 {
			requestMap["content_length"] = c.RequestContext.ContentLength
		}
		if c.RequestContext.Referrer != "" {
			requestMap["referrer"] = c.RequestContext.Referrer
		}
		result["request_context"] = requestMap
	}
	
	// Convert environment context
	if c.EnvironmentContext != nil {
		envMap := make(map[string]interface{})
		if c.EnvironmentContext.Hostname != "" {
			envMap["hostname"] = c.EnvironmentContext.Hostname
		}
		if c.EnvironmentContext.Platform != "" {
			envMap["platform"] = c.EnvironmentContext.Platform
		}
		if c.EnvironmentContext.Architecture != "" {
			envMap["architecture"] = c.EnvironmentContext.Architecture
		}
		if c.EnvironmentContext.RuntimeVersion != "" {
			envMap["runtime_version"] = c.EnvironmentContext.RuntimeVersion
		}
		if c.EnvironmentContext.ContainerID != "" {
			envMap["container_id"] = c.EnvironmentContext.ContainerID
		}
		if c.EnvironmentContext.PodName != "" {
			envMap["pod_name"] = c.EnvironmentContext.PodName
		}
		if c.EnvironmentContext.Namespace != "" {
			envMap["namespace"] = c.EnvironmentContext.Namespace
		}
		if c.EnvironmentContext.NodeName != "" {
			envMap["node_name"] = c.EnvironmentContext.NodeName
		}
		if c.EnvironmentContext.EnvironmentVars != nil {
			envMap["environment_vars"] = c.EnvironmentContext.EnvironmentVars
		}
		if len(envMap) > 0 {
			result["environment_context"] = envMap
		}
	}
	
	// Convert security context
	if c.SecurityContext != nil {
		secMap := make(map[string]interface{})
		secMap["is_encrypted"] = c.SecurityContext.IsEncrypted
		if c.SecurityContext.AuthenticationMethod != "" {
			secMap["authentication_method"] = c.SecurityContext.AuthenticationMethod
		}
		if c.SecurityContext.TokenType != "" {
			secMap["token_type"] = c.SecurityContext.TokenType
		}
		if c.SecurityContext.TokenHash != "" {
			secMap["token_hash"] = c.SecurityContext.TokenHash
		}
		if c.SecurityContext.ClientCertificateHash != "" {
			secMap["client_certificate_hash"] = c.SecurityContext.ClientCertificateHash
		}
		if c.SecurityContext.TLSVersion != "" {
			secMap["tls_version"] = c.SecurityContext.TLSVersion
		}
		if c.SecurityContext.CipherSuite != "" {
			secMap["cipher_suite"] = c.SecurityContext.CipherSuite
		}
		if c.SecurityContext.SecurityHeaders != nil {
			secMap["security_headers"] = c.SecurityContext.SecurityHeaders
		}
		result["security_context"] = secMap
	}
	
	if c.Tags != nil {
		result["tags"] = c.Tags
	}
	if c.Annotations != nil {
		result["annotations"] = c.Annotations
	}
	
	return result
}

// Event type constants for composite events
const (
	EventTypeBatch       = "batch"
	EventTypeSequenced   = "sequenced"
	EventTypeConditional = "conditional"
	EventTypeTimed       = "timed"
	EventTypeContextual  = "contextual"
)

// Convenience constructors for composite events

// NewBatchEvent creates a new batch event
func NewBatchEvent[T EventData](id string, data BatchEvent[T]) TypedTransportEvent[BatchEvent[T]] {
	return NewTypedEvent(id, EventTypeBatch, data)
}

// NewSequencedEvent creates a new sequenced event
func NewSequencedEvent[T EventData](id string, data SequencedEvent[T]) TypedTransportEvent[SequencedEvent[T]] {
	return NewTypedEvent(id, EventTypeSequenced, data)
}

// NewConditionalEvent creates a new conditional event
func NewConditionalEvent[T EventData](id string, data ConditionalEvent[T]) TypedTransportEvent[ConditionalEvent[T]] {
	return NewTypedEvent(id, EventTypeConditional, data)
}

// NewTimedEvent creates a new timed event
func NewTimedEvent[T EventData](id string, data TimedEvent[T]) TypedTransportEvent[TimedEvent[T]] {
	return NewTypedEvent(id, EventTypeTimed, data)
}

// NewContextualEvent creates a new contextual event
func NewContextualEvent[T EventData](id string, data ContextualEvent[T]) TypedTransportEvent[ContextualEvent[T]] {
	return NewTypedEvent(id, EventTypeContextual, data)
}

// Helper functions for composite event management

// BatchStatus constants
const (
	BatchStatusPending    = "pending"
	BatchStatusProcessing = "processing"
	BatchStatusCompleted  = "completed"
	BatchStatusFailed     = "failed"
	BatchStatusCancelled  = "cancelled"
)

// ConditionalAction constants
const (
	ConditionalActionDiscard  = "discard"
	ConditionalActionDelay    = "delay"
	ConditionalActionLog      = "log"
	ConditionalActionAlert    = "alert"
	ConditionalActionFallback = "fallback"
)

// ExpiryAction constants
const (
	ExpiryActionDiscard = "discard"
	ExpiryActionLog     = "log"
	ExpiryActionAlert   = "alert"
	ExpiryActionRetry   = "retry"
)