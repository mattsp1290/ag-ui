package events

import (
	"context"
	"testing"
)

func TestValidatorPermissiveMode(t *testing.T) {
	ctx := context.Background()
	validator := NewValidator(PermissiveValidationConfig())

	tests := []struct {
		name      string
		event     Event
		wantError bool
		errorMsg  string
	}{
		// Run events
		{
			name: "RunStartedEvent with empty IDs",
			event: &RunStartedEvent{
				BaseEvent:     NewBaseEvent(EventTypeRunStarted),
				ThreadIDValue: "",
				RunIDValue:    "",
			},
			wantError: false, // AllowEmptyIDs is true in permissive mode
		},
		{
			name: "RunFinishedEvent with empty IDs",
			event: &RunFinishedEvent{
				BaseEvent:     NewBaseEvent(EventTypeRunFinished),
				ThreadIDValue: "",
				RunIDValue:    "",
			},
			wantError: false,
		},
		{
			name: "RunErrorEvent with empty message",
			event: &RunErrorEvent{
				BaseEvent: NewBaseEvent(EventTypeRunError),
				Message:   "",
			},
			wantError: true,
			errorMsg:  "RunErrorEvent validation failed: message field is required",
		},
		// Step events
		{
			name: "StepStartedEvent with empty stepName",
			event: &StepStartedEvent{
				BaseEvent: NewBaseEvent(EventTypeStepStarted),
				StepName:  "",
			},
			wantError: true,
			errorMsg:  "StepStartedEvent validation failed: stepName field is required",
		},
		{
			name: "StepFinishedEvent with empty stepName",
			event: &StepFinishedEvent{
				BaseEvent: NewBaseEvent(EventTypeStepFinished),
				StepName:  "",
			},
			wantError: true,
			errorMsg:  "StepFinishedEvent validation failed: stepName field is required",
		},
		// Message events
		{
			name: "TextMessageContentEvent with empty delta",
			event: &TextMessageContentEvent{
				BaseEvent: NewBaseEvent(EventTypeTextMessageContent),
				MessageID: "msg-123",
				Delta:     "",
			},
			wantError: true,
			errorMsg:  "TextMessageContentEvent validation failed: delta field is required",
		},
		{
			name: "TextMessageEndEvent with empty messageId",
			event: &TextMessageEndEvent{
				BaseEvent: NewBaseEvent(EventTypeTextMessageEnd),
				MessageID: "",
			},
			wantError: false, // AllowEmptyIDs is true
		},
		// Tool events
		{
			name: "ToolCallArgsEvent with empty delta",
			event: &ToolCallArgsEvent{
				BaseEvent:  NewBaseEvent(EventTypeToolCallArgs),
				ToolCallID: "tool-123",
				Delta:      "",
			},
			wantError: true,
			errorMsg:  "ToolCallArgsEvent validation failed: delta field is required",
		},
		{
			name: "ToolCallEndEvent with empty toolCallId",
			event: &ToolCallEndEvent{
				BaseEvent:  NewBaseEvent(EventTypeToolCallEnd),
				ToolCallID: "",
			},
			wantError: false, // AllowEmptyIDs is true
		},
		// State events
		{
			name: "StateSnapshotEvent with nil snapshot",
			event: &StateSnapshotEvent{
				BaseEvent: NewBaseEvent(EventTypeStateSnapshot),
				Snapshot:  nil,
			},
			wantError: true,
			errorMsg:  "StateSnapshotEvent validation failed: snapshot field is required",
		},
		{
			name: "StateDeltaEvent with empty delta",
			event: &StateDeltaEvent{
				BaseEvent: NewBaseEvent(EventTypeStateDelta),
				Delta:     []JSONPatchOperation{},
			},
			wantError: true,
			errorMsg:  "StateDeltaEvent validation failed: delta field must contain at least one operation",
		},
		{
			name: "MessagesSnapshotEvent with invalid message",
			event: &MessagesSnapshotEvent{
				BaseEvent: NewBaseEvent(EventTypeMessagesSnapshot),
				Messages: []Message{
					{
						ID:   "msg-123",
						Role: "", // Missing role
					},
				},
			},
			wantError: true,
			errorMsg:  "MessagesSnapshotEvent validation failed: message[0].role field is required",
		},
		// Custom events
		{
			name: "RawEvent with nil event",
			event: &RawEvent{
				BaseEvent: NewBaseEvent(EventTypeRaw),
				Event:     nil,
			},
			wantError: true,
			errorMsg:  "RawEvent validation failed: event field is required",
		},
		{
			name: "CustomEvent with empty name",
			event: &CustomEvent{
				BaseEvent: NewBaseEvent(EventTypeCustom),
				Name:      "",
			},
			wantError: true,
			errorMsg:  "CustomEvent validation failed: name field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateEvent(ctx, tt.event)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateEvent() error = nil, want error containing %q", tt.errorMsg)
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("ValidateEvent() error = %q, want %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateEvent() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidatorStrictMode(t *testing.T) {
	ctx := context.Background()
	validator := NewValidator(DefaultValidationConfig())

	tests := []struct {
		name      string
		event     Event
		wantError bool
	}{
		{
			name: "RunStartedEvent with empty IDs in strict mode",
			event: &RunStartedEvent{
				BaseEvent:     NewBaseEvent(EventTypeRunStarted),
				ThreadIDValue: "",
				RunIDValue:    "",
			},
			wantError: true, // Strict mode should fail
		},
		{
			name: "Valid RunStartedEvent in strict mode",
			event: &RunStartedEvent{
				BaseEvent:     NewBaseEvent(EventTypeRunStarted),
				ThreadIDValue: "thread-123",
				RunIDValue:    "run-456",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateEvent(ctx, tt.event)
			if tt.wantError && err == nil {
				t.Errorf("ValidateEvent() error = nil, want error")
			} else if !tt.wantError && err != nil {
				t.Errorf("ValidateEvent() unexpected error = %v", err)
			}
		})
	}
}
