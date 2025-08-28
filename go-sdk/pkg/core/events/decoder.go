package events

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
)

// EventDecoder handles decoding of SSE events to Go SDK event types
type EventDecoder struct {
	logger *logrus.Logger
}

// NewEventDecoder creates a new event decoder
func NewEventDecoder(logger *logrus.Logger) *EventDecoder {
	if logger == nil {
		logger = logrus.New()
	}
	return &EventDecoder{logger: logger}
}

// DecodeEvent decodes a raw SSE event into the appropriate Go SDK event type
func (ed *EventDecoder) DecodeEvent(eventName string, data []byte) (Event, error) {
	eventType := EventType(eventName)

	// Check if this is a valid event type
	if !isValidEventType(eventType) {
		ed.logger.WithField("event", eventName).Warn("Unknown event type")
		return nil, fmt.Errorf("unknown event type: %s", eventName)
	}

	// Decode based on event type
	switch eventType {
	case EventTypeRunStarted:
		var evt RunStartedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode RUN_STARTED: %w", err)
		}
		return &evt, nil

	case EventTypeRunFinished:
		var evt RunFinishedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode RUN_FINISHED: %w", err)
		}
		return &evt, nil

	case EventTypeRunError:
		var evt RunErrorEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode RUN_ERROR: %w", err)
		}
		return &evt, nil

	case EventTypeTextMessageStart:
		var evt TextMessageStartEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TEXT_MESSAGE_START: %w", err)
		}
		return &evt, nil

	case EventTypeTextMessageContent:
		var evt TextMessageContentEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TEXT_MESSAGE_CONTENT: %w", err)
		}
		return &evt, nil

	case EventTypeTextMessageEnd:
		var evt TextMessageEndEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TEXT_MESSAGE_END: %w", err)
		}
		return &evt, nil

	case EventTypeToolCallStart:
		var evt ToolCallStartEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TOOL_CALL_START: %w", err)
		}
		return &evt, nil

	case EventTypeToolCallArgs:
		var evt ToolCallArgsEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TOOL_CALL_ARGS: %w", err)
		}
		return &evt, nil

	case EventTypeToolCallEnd:
		var evt ToolCallEndEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TOOL_CALL_END: %w", err)
		}
		return &evt, nil

	case EventTypeToolCallResult:
		var evt ToolCallResultEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TOOL_CALL_RESULT: %w", err)
		}
		return &evt, nil

	case EventTypeStateSnapshot:
		var evt StateSnapshotEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode STATE_SNAPSHOT: %w", err)
		}
		return &evt, nil

	case EventTypeStateDelta:
		var evt StateDeltaEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode STATE_DELTA: %w", err)
		}
		return &evt, nil

	case EventTypeMessagesSnapshot:
		var evt MessagesSnapshotEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode MESSAGES_SNAPSHOT: %w", err)
		}
		return &evt, nil

	case EventTypeStepStarted:
		var evt StepStartedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode STEP_STARTED: %w", err)
		}
		return &evt, nil

	case EventTypeStepFinished:
		var evt StepFinishedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode STEP_FINISHED: %w", err)
		}
		return &evt, nil

	case EventTypeThinkingStart:
		var evt ThinkingStartEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode THINKING_START: %w", err)
		}
		return &evt, nil

	case EventTypeThinkingEnd:
		var evt ThinkingEndEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode THINKING_END: %w", err)
		}
		return &evt, nil

	case EventTypeThinkingTextMessageStart:
		var evt ThinkingTextMessageStartEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode THINKING_TEXT_MESSAGE_START: %w", err)
		}
		return &evt, nil

	case EventTypeThinkingTextMessageContent:
		var evt ThinkingTextMessageContentEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode THINKING_TEXT_MESSAGE_CONTENT: %w", err)
		}
		return &evt, nil

	case EventTypeThinkingTextMessageEnd:
		var evt ThinkingTextMessageEndEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode THINKING_TEXT_MESSAGE_END: %w", err)
		}
		return &evt, nil

	case EventTypeCustom:
		var evt CustomEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode CUSTOM: %w", err)
		}
		return &evt, nil

	case EventTypeRaw:
		var evt RawEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode RAW: %w", err)
		}
		return &evt, nil

	default:
		// For any other event types, return a raw event
		source := string(eventType)
		return &RawEvent{
			BaseEvent: &BaseEvent{
				EventType: eventType,
			},
			Event:  json.RawMessage(data),
			Source: &source,
		}, nil
	}
}

// isValidEventType checks if an event type is valid
//func isValidEventType(eventType EventType) bool {
//	switch eventType {
//	case EventTypeTextMessageStart,
//		EventTypeTextMessageContent,
//		EventTypeTextMessageEnd,
//		EventTypeTextMessageChunk,
//		EventTypeToolCallStart,
//		EventTypeToolCallArgs,
//		EventTypeToolCallEnd,
//		EventTypeToolCallChunk,
//		EventTypeToolCallResult,
//		EventTypeStateSnapshot,
//		EventTypeStateDelta,
//		EventTypeMessagesSnapshot,
//		EventTypeRaw,
//		EventTypeCustom,
//		EventTypeRunStarted,
//		EventTypeRunFinished,
//		EventTypeRunError,
//		EventTypeStepStarted,
//		EventTypeStepFinished,
//		EventTypeThinkingStart,
//		EventTypeThinkingEnd,
//		EventTypeThinkingTextMessageStart,
//		EventTypeThinkingTextMessageContent,
//		EventTypeThinkingTextMessageEnd:
//		return true
//	default:
//		return false
//	}
//}
//
