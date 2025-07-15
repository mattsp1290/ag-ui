package transport

import (
	"errors"
	"testing"
	"time"
)

// Mock event implementation for testing
type mockEventData struct {
	ID      string
	Message string
}

func (m mockEventData) Validate() error {
	if m.ID == "" {
		return NewValidationError("id is required", nil)
	}
	if m.Message == "" {
		return NewValidationError("message is required", nil)
	}
	return nil
}

func (m mockEventData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"id":      m.ID,
		"message": m.Message,
	}
}

// Mock typed event implementation
type mockTypedEvent[T EventData] struct {
	id        string
	eventType string
	timestamp time.Time
	data      T
}

func (m mockTypedEvent[T]) ID() string {
	return m.id
}

func (m mockTypedEvent[T]) Type() string {
	return m.eventType
}

func (m mockTypedEvent[T]) Timestamp() time.Time {
	return m.timestamp
}

func (m mockTypedEvent[T]) TypedData() T {
	return m.data
}

func (m mockTypedEvent[T]) Data() map[string]interface{} {
	return m.data.ToMap()
}

func TestBatchEventValidation(t *testing.T) {
	now := time.Now()
	mockEvents := []TypedTransportEvent[mockEventData]{
		&mockTypedEvent[mockEventData]{
			id:        "event1",
			eventType: "test",
			timestamp: now,
			data:      mockEventData{ID: "1", Message: "test1"},
		},
		&mockTypedEvent[mockEventData]{
			id:        "event2",
			eventType: "test",
			timestamp: now,
			data:      mockEventData{ID: "2", Message: "test2"},
		},
	}

	tests := []struct {
		name      string
		batch     BatchEvent[mockEventData]
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid batch event",
			batch: BatchEvent[mockEventData]{
				BatchID:      "batch-001",
				Events:       mockEvents,
				BatchSize:    2,
				MaxBatchSize: 10,
				CreatedAt:    now,
				Status:       BatchStatusPending,
				SuccessCount: 0,
				FailureCount: 0,
			},
			expectErr: false,
		},
		{
			name: "missing batch ID",
			batch: BatchEvent[mockEventData]{
				Events:       mockEvents,
				BatchSize:    2,
				MaxBatchSize: 10,
				CreatedAt:    now,
				Status:       BatchStatusPending,
			},
			expectErr: true,
			errMsg:    "batch_id is required",
		},
		{
			name: "empty events",
			batch: BatchEvent[mockEventData]{
				BatchID:      "batch-002",
				Events:       []TypedTransportEvent[mockEventData]{},
				BatchSize:    0,
				MaxBatchSize: 10,
				CreatedAt:    now,
				Status:       BatchStatusPending,
			},
			expectErr: true,
			errMsg:    "events cannot be empty",
		},
		{
			name: "batch size mismatch",
			batch: BatchEvent[mockEventData]{
				BatchID:      "batch-003",
				Events:       mockEvents,
				BatchSize:    5, // Wrong size
				MaxBatchSize: 10,
				CreatedAt:    now,
				Status:       BatchStatusPending,
			},
			expectErr: true,
			errMsg:    "batch_size must match events length",
		},
		{
			name: "exceeds max batch size",
			batch: BatchEvent[mockEventData]{
				BatchID:      "batch-004",
				Events:       mockEvents,
				BatchSize:    2,
				MaxBatchSize: 1, // Too small
				CreatedAt:    now,
				Status:       BatchStatusPending,
			},
			expectErr: true,
			errMsg:    "batch_size exceeds max_batch_size",
		},
		{
			name: "missing created at",
			batch: BatchEvent[mockEventData]{
				BatchID:      "batch-005",
				Events:       mockEvents,
				BatchSize:    2,
				MaxBatchSize: 10,
				Status:       BatchStatusPending,
			},
			expectErr: true,
			errMsg:    "created_at is required",
		},
		{
			name: "missing status",
			batch: BatchEvent[mockEventData]{
				BatchID:      "batch-006",
				Events:       mockEvents,
				BatchSize:    2,
				MaxBatchSize: 10,
				CreatedAt:    now,
			},
			expectErr: true,
			errMsg:    "status is required",
		},
		{
			name: "invalid success/failure count",
			batch: BatchEvent[mockEventData]{
				BatchID:      "batch-007",
				Events:       mockEvents,
				BatchSize:    2,
				MaxBatchSize: 10,
				CreatedAt:    now,
				Status:       BatchStatusCompleted,
				SuccessCount: 2,
				FailureCount: 1, // Total exceeds batch size
			},
			expectErr: true,
			errMsg:    "success_count + failure_count cannot exceed batch_size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.batch.Validate()
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q but got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBatchEventToMap(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(5 * time.Second)

	batch := BatchEvent[mockEventData]{
		BatchID: "batch-001",
		Events: []TypedTransportEvent[mockEventData]{
			&mockTypedEvent[mockEventData]{
				id:        "event1",
				eventType: "test",
				timestamp: now,
				data:      mockEventData{ID: "1", Message: "test1"},
			},
		},
		BatchSize:          1,
		MaxBatchSize:       10,
		CreatedAt:          now,
		CompletedAt:        &completedAt,
		ProcessingDuration: 5 * time.Second,
		Status:             BatchStatusCompleted,
		Errors:             []error{errors.New("test error")},
		SuccessCount:       0,
		FailureCount:       1,
		Metadata:           map[string]interface{}{"key": "value"},
		ProcessorID:        "processor-1",
		Priority:           1,
		RetryAttempt:       1,
		MaxRetries:         3,
	}

	result := batch.ToMap()

	// Verify required fields
	if result["batch_id"] != "batch-001" {
		t.Errorf("expected batch_id to be 'batch-001', got %v", result["batch_id"])
	}
	if result["batch_size"] != 1 {
		t.Errorf("expected batch_size to be 1, got %v", result["batch_size"])
	}
	if result["status"] != BatchStatusCompleted {
		t.Errorf("expected status to be %s, got %v", BatchStatusCompleted, result["status"])
	}

	// Verify optional fields
	if result["processor_id"] != "processor-1" {
		t.Errorf("expected processor_id to be 'processor-1', got %v", result["processor_id"])
	}
	if result["priority"] != 1 {
		t.Errorf("expected priority to be 1, got %v", result["priority"])
	}
	if result["retry_attempt"] != 1 {
		t.Errorf("expected retry_attempt to be 1, got %v", result["retry_attempt"])
	}
}

func TestSequencedEventValidation(t *testing.T) {
	now := time.Now()
	mockEvent := &mockTypedEvent[mockEventData]{
		id:        "event1",
		eventType: "test",
		timestamp: now,
		data:      mockEventData{ID: "1", Message: "test1"},
	}

	totalInSeq := uint64(10)

	tests := []struct {
		name      string
		sequenced SequencedEvent[mockEventData]
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid sequenced event",
			sequenced: SequencedEvent[mockEventData]{
				SequenceID:      "seq-001",
				SequenceNumber:  5,
				TotalInSequence: &totalInSeq,
				Event:           mockEvent,
				ChecksumCurrent: "checksum123",
				IsFirst:         false,
				IsLast:          false,
				CreatedAt:       now,
			},
			expectErr: false,
		},
		{
			name: "missing sequence ID",
			sequenced: SequencedEvent[mockEventData]{
				SequenceNumber:  1,
				Event:           mockEvent,
				ChecksumCurrent: "checksum123",
				CreatedAt:       now,
			},
			expectErr: true,
			errMsg:    "sequence_id is required",
		},
		{
			name: "zero sequence number",
			sequenced: SequencedEvent[mockEventData]{
				SequenceID:      "seq-002",
				SequenceNumber:  0,
				Event:           mockEvent,
				ChecksumCurrent: "checksum123",
				CreatedAt:       now,
			},
			expectErr: true,
			errMsg:    "sequence_number must be greater than 0",
		},
		{
			name: "missing event",
			sequenced: SequencedEvent[mockEventData]{
				SequenceID:      "seq-003",
				SequenceNumber:  1,
				ChecksumCurrent: "checksum123",
				CreatedAt:       now,
			},
			expectErr: true,
			errMsg:    "event is required",
		},
		{
			name: "missing checksum",
			sequenced: SequencedEvent[mockEventData]{
				SequenceID:     "seq-004",
				SequenceNumber: 1,
				Event:          mockEvent,
				CreatedAt:      now,
			},
			expectErr: true,
			errMsg:    "checksum_current is required",
		},
		{
			name: "sequence number exceeds total",
			sequenced: SequencedEvent[mockEventData]{
				SequenceID:      "seq-005",
				SequenceNumber:  11,
				TotalInSequence: &totalInSeq,
				Event:           mockEvent,
				ChecksumCurrent: "checksum123",
				CreatedAt:       now,
			},
			expectErr: true,
			errMsg:    "sequence_number cannot exceed total_in_sequence",
		},
		{
			name: "first event not sequence number 1",
			sequenced: SequencedEvent[mockEventData]{
				SequenceID:      "seq-006",
				SequenceNumber:  2,
				Event:           mockEvent,
				ChecksumCurrent: "checksum123",
				IsFirst:         true,
				CreatedAt:       now,
			},
			expectErr: true,
			errMsg:    "first event must have sequence_number 1",
		},
		{
			name: "last event sequence number mismatch",
			sequenced: SequencedEvent[mockEventData]{
				SequenceID:      "seq-007",
				SequenceNumber:  5,
				TotalInSequence: &totalInSeq,
				Event:           mockEvent,
				ChecksumCurrent: "checksum123",
				IsLast:          true,
				CreatedAt:       now,
			},
			expectErr: true,
			errMsg:    "last event sequence_number must match total_in_sequence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sequenced.Validate()
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q but got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSequencedEventToMap(t *testing.T) {
	now := time.Now()
	totalInSeq := uint64(10)
	prevSeqNum := uint64(4)
	nextSeqNum := uint64(6)

	sequenced := SequencedEvent[mockEventData]{
		SequenceID:      "seq-001",
		SequenceNumber:  5,
		TotalInSequence: &totalInSeq,
		Event: &mockTypedEvent[mockEventData]{
			id:        "event1",
			eventType: "test",
			timestamp: now,
			data:      mockEventData{ID: "1", Message: "test1"},
		},
		PreviousSequenceNumber:     &prevSeqNum,
		NextExpectedSequenceNumber: &nextSeqNum,
		IsFirst:                    false,
		IsLast:                     false,
		ChecksumPrevious:           "prev-checksum",
		ChecksumCurrent:            "curr-checksum",
		Dependencies:               []uint64{1, 2, 3},
		PartitionKey:               "partition-1",
		OrderingKey:                "order-1",
		Timeout:                    5 * time.Minute,
		CreatedAt:                  now,
	}

	result := sequenced.ToMap()

	// Verify required fields
	if result["sequence_id"] != "seq-001" {
		t.Errorf("expected sequence_id to be 'seq-001', got %v", result["sequence_id"])
	}
	if result["sequence_number"] != uint64(5) {
		t.Errorf("expected sequence_number to be 5, got %v", result["sequence_number"])
	}

	// Verify optional fields
	if result["total_in_sequence"] != uint64(10) {
		t.Errorf("expected total_in_sequence to be 10, got %v", result["total_in_sequence"])
	}
	if result["partition_key"] != "partition-1" {
		t.Errorf("expected partition_key to be 'partition-1', got %v", result["partition_key"])
	}
}

func TestConditionalEventValidation(t *testing.T) {
	mockEvent := &mockTypedEvent[mockEventData]{
		id:        "event1",
		eventType: "test",
		timestamp: time.Now(),
		data:      mockEventData{ID: "1", Message: "test1"},
	}

	tests := []struct {
		name      string
		cond      ConditionalEvent[mockEventData]
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid conditional event",
			cond: ConditionalEvent[mockEventData]{
				ConditionID: "cond-001",
				Event:       mockEvent,
				Condition: &EventCondition{
					Type:       "field_match",
					Expression: "id == '1'",
				},
			},
			expectErr: false,
		},
		{
			name: "missing condition ID",
			cond: ConditionalEvent[mockEventData]{
				Event: mockEvent,
				Condition: &EventCondition{
					Type:       "field_match",
					Expression: "id == '1'",
				},
			},
			expectErr: true,
			errMsg:    "condition_id is required",
		},
		{
			name: "missing event",
			cond: ConditionalEvent[mockEventData]{
				ConditionID: "cond-002",
				Condition: &EventCondition{
					Type:       "field_match",
					Expression: "id == '1'",
				},
			},
			expectErr: true,
			errMsg:    "event is required",
		},
		{
			name: "missing condition",
			cond: ConditionalEvent[mockEventData]{
				ConditionID: "cond-003",
				Event:       mockEvent,
			},
			expectErr: true,
			errMsg:    "condition is required",
		},
		{
			name: "missing condition type",
			cond: ConditionalEvent[mockEventData]{
				ConditionID: "cond-004",
				Event:       mockEvent,
				Condition: &EventCondition{
					Expression: "id == '1'",
				},
			},
			expectErr: true,
			errMsg:    "condition.type is required",
		},
		{
			name: "missing condition expression",
			cond: ConditionalEvent[mockEventData]{
				ConditionID: "cond-005",
				Event:       mockEvent,
				Condition: &EventCondition{
					Type: "field_match",
				},
			},
			expectErr: true,
			errMsg:    "condition.expression is required",
		},
		{
			name: "invalid retry policy - negative max retries",
			cond: ConditionalEvent[mockEventData]{
				ConditionID: "cond-006",
				Event:       mockEvent,
				Condition: &EventCondition{
					Type:       "field_match",
					Expression: "id == '1'",
				},
				RetryPolicy: &EventRetryPolicy{
					MaxRetries:        -1,
					BackoffMultiplier: 2.0,
				},
			},
			expectErr: true,
			errMsg:    "retry_policy.max_retries cannot be negative",
		},
		{
			name: "invalid retry policy - backoff multiplier < 1.0",
			cond: ConditionalEvent[mockEventData]{
				ConditionID: "cond-007",
				Event:       mockEvent,
				Condition: &EventCondition{
					Type:       "field_match",
					Expression: "id == '1'",
				},
				RetryPolicy: &EventRetryPolicy{
					MaxRetries:        3,
					BackoffMultiplier: 0.5,
				},
			},
			expectErr: true,
			errMsg:    "retry_policy.backoff_multiplier must be >= 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cond.Validate()
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q but got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestConditionalEventToMap(t *testing.T) {
	now := time.Now()
	isCondMet := true
	minCount := 5
	maxCount := 10

	cond := ConditionalEvent[mockEventData]{
		ConditionID: "cond-001",
		Event: &mockTypedEvent[mockEventData]{
			id:        "event1",
			eventType: "test",
			timestamp: now,
			data:      mockEventData{ID: "1", Message: "test1"},
		},
		Condition: &EventCondition{
			Type:          "field_match",
			Expression:    "id == '1'",
			Operator:      "eq",
			ExpectedValue: "1",
			FieldPath:     "id",
			TimeWindow:    5 * time.Minute,
			EventTypes:    []string{"test"},
			MinCount:      &minCount,
			MaxCount:      &maxCount,
			Parameters:    map[string]interface{}{"param1": "value1"},
		},
		IsConditionMet:        &isCondMet,
		EvaluatedAt:           &now,
		EvaluationContext:     map[string]interface{}{"ctx": "value"},
		AlternativeAction:     ConditionalActionDelay,
		MaxEvaluationAttempts: 3,
		EvaluationAttempts:    1,
		TimeoutAt:             &now,
		Dependencies:          []string{"dep1", "dep2"},
		Priority:              1,
		RetryPolicy: &EventRetryPolicy{
			MaxRetries:        3,
			InitialDelay:      1 * time.Second,
			MaxDelay:          30 * time.Second,
			BackoffMultiplier: 2.0,
			Jitter:            true,
		},
	}

	result := cond.ToMap()

	// Verify required fields
	if result["condition_id"] != "cond-001" {
		t.Errorf("expected condition_id to be 'cond-001', got %v", result["condition_id"])
	}
	if result["evaluation_attempts"] != 1 {
		t.Errorf("expected evaluation_attempts to be 1, got %v", result["evaluation_attempts"])
	}

	// Verify condition conversion
	condMap, ok := result["condition"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected condition to be a map")
	}
	if condMap["type"] != "field_match" {
		t.Errorf("expected condition.type to be 'field_match', got %v", condMap["type"])
	}
	if condMap["operator"] != "eq" {
		t.Errorf("expected condition.operator to be 'eq', got %v", condMap["operator"])
	}

	// Verify optional fields
	if result["is_condition_met"] != true {
		t.Errorf("expected is_condition_met to be true, got %v", result["is_condition_met"])
	}
	if result["alternative_action"] != ConditionalActionDelay {
		t.Errorf("expected alternative_action to be %s, got %v", ConditionalActionDelay, result["alternative_action"])
	}
}

func TestTimedEventValidation(t *testing.T) {
	now := time.Now()
	delay := 5 * time.Second
	scheduledAt := now.Add(delay)

	mockEvent := &mockTypedEvent[mockEventData]{
		id:        "event1",
		eventType: "test",
		timestamp: now,
		data:      mockEventData{ID: "1", Message: "test1"},
	}

	tests := []struct {
		name      string
		timed     TimedEvent[mockEventData]
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid timed event",
			timed: TimedEvent[mockEventData]{
				TimerID:     "timer-001",
				Event:       mockEvent,
				ScheduledAt: scheduledAt,
				CreatedAt:   now,
				Delay:       delay,
			},
			expectErr: false,
		},
		{
			name: "missing timer ID",
			timed: TimedEvent[mockEventData]{
				Event:       mockEvent,
				ScheduledAt: scheduledAt,
				CreatedAt:   now,
				Delay:       delay,
			},
			expectErr: true,
			errMsg:    "timer_id is required",
		},
		{
			name: "missing event",
			timed: TimedEvent[mockEventData]{
				TimerID:     "timer-002",
				ScheduledAt: scheduledAt,
				CreatedAt:   now,
				Delay:       delay,
			},
			expectErr: true,
			errMsg:    "event is required",
		},
		{
			name: "missing scheduled at",
			timed: TimedEvent[mockEventData]{
				TimerID:   "timer-003",
				Event:     mockEvent,
				CreatedAt: now,
				Delay:     delay,
			},
			expectErr: true,
			errMsg:    "scheduled_at is required",
		},
		{
			name: "missing created at",
			timed: TimedEvent[mockEventData]{
				TimerID:     "timer-004",
				Event:       mockEvent,
				ScheduledAt: scheduledAt,
				Delay:       delay,
			},
			expectErr: true,
			errMsg:    "created_at is required",
		},
		{
			name: "negative delay",
			timed: TimedEvent[mockEventData]{
				TimerID:     "timer-005",
				Event:       mockEvent,
				ScheduledAt: now.Add(-5 * time.Second),
				CreatedAt:   now,
				Delay:       -5 * time.Second,
			},
			expectErr: true,
			errMsg:    "delay cannot be negative",
		},
		{
			name: "scheduled time mismatch",
			timed: TimedEvent[mockEventData]{
				TimerID:     "timer-006",
				Event:       mockEvent,
				ScheduledAt: now.Add(10 * time.Second), // Wrong scheduled time
				CreatedAt:   now,
				Delay:       5 * time.Second,
			},
			expectErr: true,
			errMsg:    "scheduled_at must equal created_at + delay",
		},
		{
			name: "recurring event without pattern",
			timed: TimedEvent[mockEventData]{
				TimerID:     "timer-007",
				Event:       mockEvent,
				ScheduledAt: scheduledAt,
				CreatedAt:   now,
				Delay:       delay,
				IsRecurring: true,
			},
			expectErr: true,
			errMsg:    "recurrence_pattern is required for recurring events",
		},
		{
			name: "occurrence count exceeds max",
			timed: TimedEvent[mockEventData]{
				TimerID:         "timer-008",
				Event:           mockEvent,
				ScheduledAt:     scheduledAt,
				CreatedAt:       now,
				Delay:           delay,
				MaxOccurrences:  5,
				OccurrenceCount: 6,
			},
			expectErr: true,
			errMsg:    "occurrence_count cannot exceed max_occurrences",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.timed.Validate()
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q but got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestTimedEventToMap(t *testing.T) {
	now := time.Now()
	delay := 5 * time.Second
	scheduledAt := now.Add(delay)
	processedAt := now.Add(10 * time.Second)
	expiresAt := now.Add(1 * time.Hour)
	nextScheduledAt := now.Add(1 * time.Hour)

	timed := TimedEvent[mockEventData]{
		TimerID: "timer-001",
		Event: &mockTypedEvent[mockEventData]{
			id:        "event1",
			eventType: "test",
			timestamp: now,
			data:      mockEventData{ID: "1", Message: "test1"},
		},
		ScheduledAt:       scheduledAt,
		CreatedAt:         now,
		ProcessedAt:       &processedAt,
		Delay:             delay,
		ActualDelay:       10 * time.Second,
		MaxDelay:          30 * time.Second,
		IsExpired:         false,
		ExpiresAt:         &expiresAt,
		IsRecurring:       true,
		RecurrencePattern: "0 * * * *",
		NextScheduledAt:   &nextScheduledAt,
		MaxOccurrences:    10,
		OccurrenceCount:   1,
		TimeZone:          "UTC",
		Priority:          1,
		OnExpiry:          ExpiryActionLog,
	}

	result := timed.ToMap()

	// Verify required fields
	if result["timer_id"] != "timer-001" {
		t.Errorf("expected timer_id to be 'timer-001', got %v", result["timer_id"])
	}
	if result["is_expired"] != false {
		t.Errorf("expected is_expired to be false, got %v", result["is_expired"])
	}
	if result["is_recurring"] != true {
		t.Errorf("expected is_recurring to be true, got %v", result["is_recurring"])
	}

	// Verify optional fields
	if result["recurrence_pattern"] != "0 * * * *" {
		t.Errorf("expected recurrence_pattern to be '0 * * * *', got %v", result["recurrence_pattern"])
	}
	if result["time_zone"] != "UTC" {
		t.Errorf("expected time_zone to be 'UTC', got %v", result["time_zone"])
	}
	if result["on_expiry"] != ExpiryActionLog {
		t.Errorf("expected on_expiry to be %s, got %v", ExpiryActionLog, result["on_expiry"])
	}
}

func TestContextualEventValidation(t *testing.T) {
	now := time.Now()
	mockEvent := &mockTypedEvent[mockEventData]{
		id:        "event1",
		eventType: "test",
		timestamp: now,
		data:      mockEventData{ID: "1", Message: "test1"},
	}

	tests := []struct {
		name      string
		ctx       ContextualEvent[mockEventData]
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid contextual event",
			ctx: ContextualEvent[mockEventData]{
				ContextID: "ctx-001",
				Event:     mockEvent,
				Context: &EventContext{
					Timestamp:   now,
					Source:      "test-service",
					Environment: "production",
				},
			},
			expectErr: false,
		},
		{
			name: "missing context ID",
			ctx: ContextualEvent[mockEventData]{
				Event: mockEvent,
				Context: &EventContext{
					Timestamp:   now,
					Source:      "test-service",
					Environment: "production",
				},
			},
			expectErr: true,
			errMsg:    "context_id is required",
		},
		{
			name: "missing event",
			ctx: ContextualEvent[mockEventData]{
				ContextID: "ctx-002",
				Context: &EventContext{
					Timestamp:   now,
					Source:      "test-service",
					Environment: "production",
				},
			},
			expectErr: true,
			errMsg:    "event is required",
		},
		{
			name: "missing context",
			ctx: ContextualEvent[mockEventData]{
				ContextID: "ctx-003",
				Event:     mockEvent,
			},
			expectErr: true,
			errMsg:    "context is required",
		},
		{
			name: "missing context timestamp",
			ctx: ContextualEvent[mockEventData]{
				ContextID: "ctx-004",
				Event:     mockEvent,
				Context: &EventContext{
					Source:      "test-service",
					Environment: "production",
				},
			},
			expectErr: true,
			errMsg:    "context.timestamp is required",
		},
		{
			name: "missing context source",
			ctx: ContextualEvent[mockEventData]{
				ContextID: "ctx-005",
				Event:     mockEvent,
				Context: &EventContext{
					Timestamp:   now,
					Environment: "production",
				},
			},
			expectErr: true,
			errMsg:    "context.source is required",
		},
		{
			name: "missing context environment",
			ctx: ContextualEvent[mockEventData]{
				ContextID: "ctx-006",
				Event:     mockEvent,
				Context: &EventContext{
					Timestamp: now,
					Source:    "test-service",
				},
			},
			expectErr: true,
			errMsg:    "context.environment is required",
		},
		{
			name: "invalid user context - missing user ID",
			ctx: ContextualEvent[mockEventData]{
				ContextID: "ctx-007",
				Event:     mockEvent,
				Context: &EventContext{
					Timestamp:   now,
					Source:      "test-service",
					Environment: "production",
				},
				UserContext: &UserContext{
					UserType: "human",
				},
			},
			expectErr: true,
			errMsg:    "user_context.user_id is required when user_context is provided",
		},
		{
			name: "invalid request context - missing request ID",
			ctx: ContextualEvent[mockEventData]{
				ContextID: "ctx-008",
				Event:     mockEvent,
				Context: &EventContext{
					Timestamp:   now,
					Source:      "test-service",
					Environment: "production",
				},
				RequestContext: &RequestContext{
					Method: "GET",
				},
			},
			expectErr: true,
			errMsg:    "request_context.request_id is required when request_context is provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q but got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestContextualEventToMap(t *testing.T) {
	now := time.Now()

	ctx := ContextualEvent[mockEventData]{
		ContextID: "ctx-001",
		Event: &mockTypedEvent[mockEventData]{
			id:        "event1",
			eventType: "test",
			timestamp: now,
			data:      mockEventData{ID: "1", Message: "test1"},
		},
		Context: &EventContext{
			Timestamp:       now,
			Version:         "1.0",
			Source:          "test-service",
			SourceVersion:   "v1.2.3",
			Environment:     "production",
			Region:          "us-east-1",
			TenantID:        "tenant-123",
			ServiceName:     "api-gateway",
			ServiceInstance: "api-gateway-1",
			ProcessID:       12345,
			ThreadID:        "thread-1",
		},
		CorrelationID: "corr-123",
		CausationID:   "cause-456",
		TraceID:       "trace-789",
		SpanID:        "span-abc",
		BusinessContext: map[string]interface{}{
			"order_id": "order-123",
		},
		TechnicalContext: map[string]interface{}{
			"cpu_usage": 45.5,
		},
		UserContext: &UserContext{
			UserID:      "user-123",
			UserType:    "human",
			SessionID:   "session-456",
			Roles:       []string{"admin", "user"},
			Permissions: []string{"read", "write"},
			Groups:      []string{"engineering"},
			ClientInfo: map[string]string{
				"app_version": "2.0.1",
			},
			Preferences: map[string]interface{}{
				"theme": "dark",
			},
		},
		RequestContext: &RequestContext{
			RequestID:     "req-123",
			Method:        "POST",
			URL:           "/api/v1/events",
			Headers:       map[string]string{"X-Request-ID": "req-123"},
			QueryParams:   map[string]string{"filter": "active"},
			UserAgent:     "TestClient/1.0",
			ClientIP:      "192.168.1.100",
			ContentType:   "application/json",
			ContentLength: 1024,
			Referrer:      "https://example.com",
		},
		EnvironmentContext: &EnvironmentContext{
			Hostname:        "host-1",
			Platform:        "linux",
			Architecture:    "amd64",
			RuntimeVersion:  "go1.21",
			ContainerID:     "container-123",
			PodName:         "api-pod-1",
			Namespace:       "production",
			NodeName:        "node-1",
			EnvironmentVars: map[string]string{"ENV": "prod"},
		},
		SecurityContext: &SecurityContext{
			AuthenticationMethod:  "oauth2",
			TokenType:             "bearer",
			TokenHash:             "hash-123",
			ClientCertificateHash: "cert-hash",
			TLSVersion:            "TLS1.3",
			CipherSuite:           "TLS_AES_256_GCM_SHA384",
			IsEncrypted:           true,
			SecurityHeaders:       map[string]string{"X-Auth": "Bearer token"},
		},
		Tags: map[string]string{
			"env": "prod",
			"app": "api",
		},
		Annotations: map[string]string{
			"note": "test event",
		},
	}

	result := ctx.ToMap()

	// Verify required fields
	if result["context_id"] != "ctx-001" {
		t.Errorf("expected context_id to be 'ctx-001', got %v", result["context_id"])
	}

	// Verify context conversion
	ctxMap, ok := result["context"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected context to be a map")
	}
	if ctxMap["source"] != "test-service" {
		t.Errorf("expected context.source to be 'test-service', got %v", ctxMap["source"])
	}
	if ctxMap["environment"] != "production" {
		t.Errorf("expected context.environment to be 'production', got %v", ctxMap["environment"])
	}

	// Verify optional fields
	if result["correlation_id"] != "corr-123" {
		t.Errorf("expected correlation_id to be 'corr-123', got %v", result["correlation_id"])
	}
	if result["trace_id"] != "trace-789" {
		t.Errorf("expected trace_id to be 'trace-789', got %v", result["trace_id"])
	}

	// Verify user context
	userCtx, ok := result["user_context"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user_context to be a map")
	}
	if userCtx["user_id"] != "user-123" {
		t.Errorf("expected user_context.user_id to be 'user-123', got %v", userCtx["user_id"])
	}

	// Verify tags
	tags, ok := result["tags"].(map[string]string)
	if !ok {
		t.Fatalf("expected tags to be a map[string]string")
	}
	if tags["env"] != "prod" {
		t.Errorf("expected tags['env'] to be 'prod', got %v", tags["env"])
	}
}

func TestCompositeEventConstructors(t *testing.T) {
	now := time.Now()

	// Test BatchEvent constructor
	batchData := BatchEvent[mockEventData]{
		BatchID:   "batch-001",
		Events:    []TypedTransportEvent[mockEventData]{},
		BatchSize: 0,
		Status:    BatchStatusPending,
		CreatedAt: now,
	}
	batchEvent := NewBatchEvent("batch-event-1", batchData)
	if batchEvent.ID() != "batch-event-1" {
		t.Errorf("expected batch event ID to be 'batch-event-1', got %v", batchEvent.ID())
	}
	if batchEvent.Type() != EventTypeBatch {
		t.Errorf("expected batch event type to be %s, got %v", EventTypeBatch, batchEvent.Type())
	}

	// Test SequencedEvent constructor
	seqData := SequencedEvent[mockEventData]{
		SequenceID:      "seq-001",
		SequenceNumber:  1,
		Event:           nil,
		ChecksumCurrent: "checksum",
		CreatedAt:       now,
	}
	seqEvent := NewSequencedEvent("seq-event-1", seqData)
	if seqEvent.ID() != "seq-event-1" {
		t.Errorf("expected sequenced event ID to be 'seq-event-1', got %v", seqEvent.ID())
	}
	if seqEvent.Type() != EventTypeSequenced {
		t.Errorf("expected sequenced event type to be %s, got %v", EventTypeSequenced, seqEvent.Type())
	}

	// Test ConditionalEvent constructor
	condData := ConditionalEvent[mockEventData]{
		ConditionID: "cond-001",
		Event:       nil,
		Condition:   &EventCondition{Type: "test", Expression: "true"},
	}
	condEvent := NewConditionalEvent("cond-event-1", condData)
	if condEvent.ID() != "cond-event-1" {
		t.Errorf("expected conditional event ID to be 'cond-event-1', got %v", condEvent.ID())
	}
	if condEvent.Type() != EventTypeConditional {
		t.Errorf("expected conditional event type to be %s, got %v", EventTypeConditional, condEvent.Type())
	}

	// Test TimedEvent constructor
	timedData := TimedEvent[mockEventData]{
		TimerID:     "timer-001",
		Event:       nil,
		ScheduledAt: now,
		CreatedAt:   now,
		Delay:       0,
	}
	timedEvent := NewTimedEvent("timed-event-1", timedData)
	if timedEvent.ID() != "timed-event-1" {
		t.Errorf("expected timed event ID to be 'timed-event-1', got %v", timedEvent.ID())
	}
	if timedEvent.Type() != EventTypeTimed {
		t.Errorf("expected timed event type to be %s, got %v", EventTypeTimed, timedEvent.Type())
	}

	// Test ContextualEvent constructor
	ctxData := ContextualEvent[mockEventData]{
		ContextID: "ctx-001",
		Event:     nil,
		Context: &EventContext{
			Timestamp:   now,
			Source:      "test",
			Environment: "test",
		},
	}
	ctxEvent := NewContextualEvent("ctx-event-1", ctxData)
	if ctxEvent.ID() != "ctx-event-1" {
		t.Errorf("expected contextual event ID to be 'ctx-event-1', got %v", ctxEvent.ID())
	}
	if ctxEvent.Type() != EventTypeContextual {
		t.Errorf("expected contextual event type to be %s, got %v", EventTypeContextual, ctxEvent.Type())
	}
}
