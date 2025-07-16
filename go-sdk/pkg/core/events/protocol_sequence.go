package events

import (
	"fmt"
	"time"
)

// ProtocolSequenceRule validates AG-UI protocol sequence compliance
type ProtocolSequenceRule struct {
	*BaseValidationRule
	requiredSequence map[EventType][]EventType
}

// NewProtocolSequenceRule creates a new protocol sequence validation rule
func NewProtocolSequenceRule() *ProtocolSequenceRule {
	return &ProtocolSequenceRule{
		BaseValidationRule: NewBaseValidationRule(
			"PROTOCOL_SEQUENCE_COMPLIANCE",
			"Validates strict AG-UI protocol sequence compliance",
			ValidationSeverityError,
		),
		requiredSequence: map[EventType][]EventType{
			// Run events must be properly ordered
			EventTypeRunStarted:  {},
			EventTypeRunFinished: {EventTypeRunStarted},
			EventTypeRunError:    {EventTypeRunStarted},

			// Message events must follow start -> content -> end pattern
			EventTypeTextMessageStart:   {EventTypeRunStarted},
			EventTypeTextMessageContent: {EventTypeTextMessageStart},
			EventTypeTextMessageEnd:     {EventTypeTextMessageStart, EventTypeTextMessageContent},

			// Tool call events must follow start -> args -> end pattern
			EventTypeToolCallStart: {EventTypeRunStarted},
			EventTypeToolCallArgs:  {EventTypeToolCallStart},
			EventTypeToolCallEnd:   {EventTypeToolCallStart, EventTypeToolCallArgs},

			// Step events must be within a run
			EventTypeStepStarted:  {EventTypeRunStarted},
			EventTypeStepFinished: {EventTypeStepStarted},
		},
	}
}

// Validate implements the ValidationRule interface
func (r *ProtocolSequenceRule) Validate(event Event, context *ValidationContext) *ValidationResult {
	result := &ValidationResult{
		IsValid:   true,
		Timestamp: time.Now(),
	}

	if !r.IsEnabled() {
		return result
	}

	// Skip validation if sequence validation is disabled
	if context.Config != nil && context.Config.SkipSequenceValidation {
		return result
	}

	// Check if event type requires prerequisite events
	requiredEvents, exists := r.requiredSequence[event.Type()]
	if !exists {
		return result // No requirements for this event type
	}

	// For empty requirements, the event is always valid
	if len(requiredEvents) == 0 {
		return result
	}

	// Check if any required event has occurred in the sequence
	if context.EventSequence != nil {
		r.validateSequenceRequirements(event, context, result, requiredEvents)
	} else {
		// For single event validation, check state
		r.validateStateRequirements(event, context, result, requiredEvents)
	}

	return result
}

// validateSequenceRequirements validates requirements against event sequence
func (r *ProtocolSequenceRule) validateSequenceRequirements(event Event, context *ValidationContext, result *ValidationResult, requiredEvents []EventType) {
	// Check if any required event type exists in the sequence before current event
	found := false
	for i := 0; i < context.EventIndex; i++ {
		for _, requiredType := range requiredEvents {
			if context.EventSequence[i].Type() == requiredType {
				// Validate that the required event matches the context (same run, message, tool call, etc.)
				if r.validateEventContext(event, context.EventSequence[i]) {
					found = true
					break
				}
			}
		}
		if found {
			break
		}
	}

	if !found {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Event %s requires one of %v to occur first", event.Type(), requiredEvents),
			map[string]interface{}{
				"event_type":      event.Type(),
				"required_events": requiredEvents,
				"sequence_index":  context.EventIndex,
			},
			[]string{fmt.Sprintf("Send one of %v events before %s", requiredEvents, event.Type())}))
	}
}

// validateStateRequirements validates requirements against current state
func (r *ProtocolSequenceRule) validateStateRequirements(event Event, context *ValidationContext, result *ValidationResult, requiredEvents []EventType) {
	// Check state to see if required preconditions are met
	switch event.Type() {
	case EventTypeRunFinished, EventTypeRunError:
		// Check if any run is active
		if len(context.State.ActiveRuns) == 0 {
			result.AddError(r.CreateError(event,
				"Cannot finish or error a run when no runs are active",
				map[string]interface{}{"active_runs": len(context.State.ActiveRuns)},
				[]string{"Start a run with RUN_STARTED before finishing or erroring"}))
		}

	case EventTypeTextMessageContent, EventTypeTextMessageEnd:
		// Check if any message is active
		if len(context.State.ActiveMessages) == 0 {
			result.AddError(r.CreateError(event,
				"Cannot send message content or end when no messages are active",
				map[string]interface{}{"active_messages": len(context.State.ActiveMessages)},
				[]string{"Start a message with TEXT_MESSAGE_START before sending content or ending"}))
		}

	case EventTypeToolCallArgs, EventTypeToolCallEnd:
		// Check if any tool call is active
		if len(context.State.ActiveTools) == 0 {
			result.AddError(r.CreateError(event,
				"Cannot send tool call args or end when no tool calls are active",
				map[string]interface{}{"active_tools": len(context.State.ActiveTools)},
				[]string{"Start a tool call with TOOL_CALL_START before sending args or ending"}))
		}

	case EventTypeStepFinished:
		// Check if any step is active
		if len(context.State.ActiveSteps) == 0 {
			result.AddError(r.CreateError(event,
				"Cannot finish a step when no steps are active",
				map[string]interface{}{"active_steps": len(context.State.ActiveSteps)},
				[]string{"Start a step with STEP_STARTED before finishing"}))
		}
	}
}

// validateEventContext checks if two events share the same context (run ID, message ID, etc.)
func (r *ProtocolSequenceRule) validateEventContext(currentEvent, previousEvent Event) bool {
	// For run events, they should be from the same run
	if currentRunEvent, ok := currentEvent.(*RunFinishedEvent); ok {
		if prevRunEvent, ok := previousEvent.(*RunStartedEvent); ok {
			return currentRunEvent.RunID() == prevRunEvent.RunID()
		}
	}

	if currentRunEvent, ok := currentEvent.(*RunErrorEvent); ok {
		if prevRunEvent, ok := previousEvent.(*RunStartedEvent); ok {
			return currentRunEvent.RunID() == prevRunEvent.RunID()
		}
	}

	// For message events, they should be from the same message
	if currentMsgEvent, ok := currentEvent.(*TextMessageContentEvent); ok {
		if prevMsgEvent, ok := previousEvent.(*TextMessageStartEvent); ok {
			return currentMsgEvent.MessageID == prevMsgEvent.MessageID
		}
	}

	if currentMsgEvent, ok := currentEvent.(*TextMessageEndEvent); ok {
		if prevMsgEvent, ok := previousEvent.(*TextMessageStartEvent); ok {
			return currentMsgEvent.MessageID == prevMsgEvent.MessageID
		}
		if prevMsgEvent, ok := previousEvent.(*TextMessageContentEvent); ok {
			return currentMsgEvent.MessageID == prevMsgEvent.MessageID
		}
	}

	// For tool call events, they should be from the same tool call
	if currentToolEvent, ok := currentEvent.(*ToolCallArgsEvent); ok {
		if prevToolEvent, ok := previousEvent.(*ToolCallStartEvent); ok {
			return currentToolEvent.ToolCallID == prevToolEvent.ToolCallID
		}
	}

	if currentToolEvent, ok := currentEvent.(*ToolCallEndEvent); ok {
		if prevToolEvent, ok := previousEvent.(*ToolCallStartEvent); ok {
			return currentToolEvent.ToolCallID == prevToolEvent.ToolCallID
		}
		if prevToolEvent, ok := previousEvent.(*ToolCallArgsEvent); ok {
			return currentToolEvent.ToolCallID == prevToolEvent.ToolCallID
		}
	}

	// For step events, they should be from the same step
	if currentStepEvent, ok := currentEvent.(*StepFinishedEvent); ok {
		if prevStepEvent, ok := previousEvent.(*StepStartedEvent); ok {
			return currentStepEvent.StepName == prevStepEvent.StepName
		}
	}

	// Default case - events are contextually related if we reach here
	return true
}
