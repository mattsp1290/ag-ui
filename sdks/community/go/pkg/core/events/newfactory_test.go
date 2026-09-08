package events

import (
	"reflect"
	"testing"
)

func TestNewEventForTypeAllProtocolEvents(t *testing.T) {
	tests := []struct {
		eventType EventType
		want      Event
	}{
		{EventTypeTextMessageStart, &TextMessageStartEvent{}},
		{EventTypeTextMessageContent, &TextMessageContentEvent{}},
		{EventTypeTextMessageEnd, &TextMessageEndEvent{}},
		{EventTypeTextMessageChunk, &TextMessageChunkEvent{}},
		{EventTypeToolCallStart, &ToolCallStartEvent{}},
		{EventTypeToolCallArgs, &ToolCallArgsEvent{}},
		{EventTypeToolCallEnd, &ToolCallEndEvent{}},
		{EventTypeToolCallChunk, &ToolCallChunkEvent{}},
		{EventTypeToolCallResult, &ToolCallResultEvent{}},
		{EventTypeStateSnapshot, &StateSnapshotEvent{}},
		{EventTypeStateDelta, &StateDeltaEvent{}},
		{EventTypeMessagesSnapshot, &MessagesSnapshotEvent{}},
		{EventTypeActivitySnapshot, &ActivitySnapshotEvent{}},
		{EventTypeActivityDelta, &ActivityDeltaEvent{}},
		{EventTypeRaw, &RawEvent{}},
		{EventTypeCustom, &CustomEvent{}},
		{EventTypeRunStarted, &RunStartedEvent{}},
		{EventTypeRunFinished, &RunFinishedEvent{}},
		{EventTypeRunError, &RunErrorEvent{}},
		{EventTypeStepStarted, &StepStartedEvent{}},
		{EventTypeStepFinished, &StepFinishedEvent{}},
		{EventTypeThinkingStart, &ThinkingStartEvent{}},
		{EventTypeThinkingEnd, &ThinkingEndEvent{}},
		{EventTypeThinkingTextMessageStart, &ThinkingTextMessageStartEvent{}},
		{EventTypeThinkingTextMessageContent, &ThinkingTextMessageContentEvent{}},
		{EventTypeThinkingTextMessageEnd, &ThinkingTextMessageEndEvent{}},
		{EventTypeReasoningStart, &ReasoningStartEvent{}},
		{EventTypeReasoningMessageStart, &ReasoningMessageStartEvent{}},
		{EventTypeReasoningMessageContent, &ReasoningMessageContentEvent{}},
		{EventTypeReasoningMessageEnd, &ReasoningMessageEndEvent{}},
		{EventTypeReasoningMessageChunk, &ReasoningMessageChunkEvent{}},
		{EventTypeReasoningEnd, &ReasoningEndEvent{}},
		{EventTypeReasoningEncryptedValue, &ReasoningEncryptedValueEvent{}},
		{EventTypeSubagentStarted, &SubagentStartedEvent{}},
		{EventTypeSubagentFinished, &SubagentFinishedEvent{}},
		{EventTypeSubagentError, &SubagentErrorEvent{}},
	}

	for _, test := range tests {
		t.Run(string(test.eventType), func(t *testing.T) {
			first, err := NewEventForType(test.eventType)
			if err != nil {
				t.Fatalf("NewEventForType() error = %v", err)
			}
			second, err := NewEventForType(test.eventType)
			if err != nil {
				t.Fatalf("second NewEventForType() error = %v", err)
			}
			if reflect.TypeOf(first) != reflect.TypeOf(test.want) {
				t.Fatalf("NewEventForType() type = %T, want %T", first, test.want)
			}
			if first.Type() != test.eventType || first.GetBaseEvent() == nil {
				t.Fatalf("factory base = %#v, type = %q", first.GetBaseEvent(), first.Type())
			}
			if first.Timestamp() != nil {
				t.Fatalf("factory synthesized timestamp %v", *first.Timestamp())
			}
			if first.GetBaseEvent() == second.GetBaseEvent() {
				t.Fatal("factory reused BaseEvent allocation")
			}
		})
	}
}

func TestNewEventForTypeUnknown(t *testing.T) {
	event, err := NewEventForType(EventTypeUnknown)
	if err == nil || event != nil {
		t.Fatalf("NewEventForType(UNKNOWN) = (%#v, %v), want nil error result", event, err)
	}
	if isValidEventType(EventTypeUnknown) {
		t.Fatal("UNKNOWN must not be a valid protocol event type")
	}
}

func TestEventDecoderUsesFactoryAndPreservesDiscriminatorPolicy(t *testing.T) {
	decoder := NewEventDecoder(nil)

	tests := []struct {
		name        string
		eventName   EventType
		data        string
		want        Event
		wantType    EventType
		wantNilBase bool
	}{
		{"tool call chunk", EventTypeToolCallChunk, `{"type":"TOOL_CALL_CHUNK"}`, &ToolCallChunkEvent{}, EventTypeToolCallChunk, false},
		{"subagent started", EventTypeSubagentStarted, `{"type":"SUBAGENT_STARTED"}`, &SubagentStartedEvent{}, EventTypeSubagentStarted, false},
		{"subagent finished", EventTypeSubagentFinished, `{"type":"SUBAGENT_FINISHED"}`, &SubagentFinishedEvent{}, EventTypeSubagentFinished, false},
		{"subagent error", EventTypeSubagentError, `{"type":"SUBAGENT_ERROR"}`, &SubagentErrorEvent{}, EventTypeSubagentError, false},
		{"missing payload type keeps nil base", EventTypeTextMessageStart, `{"messageId":"m"}`, &TextMessageStartEvent{}, "", true},
		{"mismatched payload type is retained", EventTypeTextMessageStart, `{"type":"TEXT_MESSAGE_END","messageId":"m"}`, &TextMessageStartEvent{}, EventTypeTextMessageEnd, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, err := decoder.DecodeEvent(string(test.eventName), []byte(test.data))
			if err != nil {
				t.Fatalf("DecodeEvent() error = %v", err)
			}
			if reflect.TypeOf(event) != reflect.TypeOf(test.want) {
				t.Fatalf("DecodeEvent() = %T, want %T", event, test.want)
			}
			if test.wantNilBase {
				if event.GetBaseEvent() != nil {
					t.Fatalf("DecodeEvent() base = %#v, want nil", event.GetBaseEvent())
				}
				return
			}
			if event.Type() != test.wantType {
				t.Fatalf("DecodeEvent() = %T type %q, want %T type %q", event, event.Type(), test.want, test.wantType)
			}
		})
	}
}

func TestNilBaseEventValidateReturnsError(t *testing.T) {
	var base *BaseEvent
	if err := base.Validate(); err == nil {
		t.Fatal("nil BaseEvent.Validate() returned nil")
	}
}
