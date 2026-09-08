package events

import "fmt"

var eventConstructors = map[EventType]func(*BaseEvent) Event{
	EventTypeTextMessageStart:           func(b *BaseEvent) Event { return &TextMessageStartEvent{BaseEvent: b} },
	EventTypeTextMessageContent:         func(b *BaseEvent) Event { return &TextMessageContentEvent{BaseEvent: b} },
	EventTypeTextMessageEnd:             func(b *BaseEvent) Event { return &TextMessageEndEvent{BaseEvent: b} },
	EventTypeTextMessageChunk:           func(b *BaseEvent) Event { return &TextMessageChunkEvent{BaseEvent: b} },
	EventTypeToolCallStart:              func(b *BaseEvent) Event { return &ToolCallStartEvent{BaseEvent: b} },
	EventTypeToolCallArgs:               func(b *BaseEvent) Event { return &ToolCallArgsEvent{BaseEvent: b} },
	EventTypeToolCallEnd:                func(b *BaseEvent) Event { return &ToolCallEndEvent{BaseEvent: b} },
	EventTypeToolCallChunk:              func(b *BaseEvent) Event { return &ToolCallChunkEvent{BaseEvent: b} },
	EventTypeToolCallResult:             func(b *BaseEvent) Event { return &ToolCallResultEvent{BaseEvent: b} },
	EventTypeStateSnapshot:              func(b *BaseEvent) Event { return &StateSnapshotEvent{BaseEvent: b} },
	EventTypeStateDelta:                 func(b *BaseEvent) Event { return &StateDeltaEvent{BaseEvent: b} },
	EventTypeMessagesSnapshot:           func(b *BaseEvent) Event { return &MessagesSnapshotEvent{BaseEvent: b} },
	EventTypeActivitySnapshot:           func(b *BaseEvent) Event { return &ActivitySnapshotEvent{BaseEvent: b} },
	EventTypeActivityDelta:              func(b *BaseEvent) Event { return &ActivityDeltaEvent{BaseEvent: b} },
	EventTypeRaw:                        func(b *BaseEvent) Event { return &RawEvent{BaseEvent: b} },
	EventTypeCustom:                     func(b *BaseEvent) Event { return &CustomEvent{BaseEvent: b} },
	EventTypeRunStarted:                 func(b *BaseEvent) Event { return &RunStartedEvent{BaseEvent: b} },
	EventTypeRunFinished:                func(b *BaseEvent) Event { return &RunFinishedEvent{BaseEvent: b} },
	EventTypeRunError:                   func(b *BaseEvent) Event { return &RunErrorEvent{BaseEvent: b} },
	EventTypeStepStarted:                func(b *BaseEvent) Event { return &StepStartedEvent{BaseEvent: b} },
	EventTypeStepFinished:               func(b *BaseEvent) Event { return &StepFinishedEvent{BaseEvent: b} },
	EventTypeThinkingStart:              func(b *BaseEvent) Event { return &ThinkingStartEvent{BaseEvent: b} },
	EventTypeThinkingEnd:                func(b *BaseEvent) Event { return &ThinkingEndEvent{BaseEvent: b} },
	EventTypeThinkingTextMessageStart:   func(b *BaseEvent) Event { return &ThinkingTextMessageStartEvent{BaseEvent: b} },
	EventTypeThinkingTextMessageContent: func(b *BaseEvent) Event { return &ThinkingTextMessageContentEvent{BaseEvent: b} },
	EventTypeThinkingTextMessageEnd:     func(b *BaseEvent) Event { return &ThinkingTextMessageEndEvent{BaseEvent: b} },
	EventTypeReasoningStart:             func(b *BaseEvent) Event { return &ReasoningStartEvent{BaseEvent: b} },
	EventTypeReasoningMessageStart:      func(b *BaseEvent) Event { return &ReasoningMessageStartEvent{BaseEvent: b} },
	EventTypeReasoningMessageContent:    func(b *BaseEvent) Event { return &ReasoningMessageContentEvent{BaseEvent: b} },
	EventTypeReasoningMessageEnd:        func(b *BaseEvent) Event { return &ReasoningMessageEndEvent{BaseEvent: b} },
	EventTypeReasoningMessageChunk:      func(b *BaseEvent) Event { return &ReasoningMessageChunkEvent{BaseEvent: b} },
	EventTypeReasoningEnd:               func(b *BaseEvent) Event { return &ReasoningEndEvent{BaseEvent: b} },
	EventTypeReasoningEncryptedValue:    func(b *BaseEvent) Event { return &ReasoningEncryptedValueEvent{BaseEvent: b} },
	EventTypeSubagentStarted:            func(b *BaseEvent) Event { return &SubagentStartedEvent{BaseEvent: b} },
	EventTypeSubagentFinished:           func(b *BaseEvent) Event { return &SubagentFinishedEvent{BaseEvent: b} },
	EventTypeSubagentError:              func(b *BaseEvent) Event { return &SubagentErrorEvent{BaseEvent: b} },
}

// NewEventForType returns a fresh concrete event for a protocol event type.
func NewEventForType(eventType EventType) (Event, error) {
	constructor, ok := eventConstructors[eventType]
	if !ok {
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
	// Decoding must not synthesize optional wire fields such as timestamp.
	return constructor(&BaseEvent{EventType: eventType}), nil
}
