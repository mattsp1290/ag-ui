package protobuf

import (
	"encoding/binary"
	"fmt"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// ProtobufDecoder implements the Decoder interface for Protocol Buffer decoding
type ProtobufDecoder struct {
	options *encoding.DecodingOptions
}

// NewProtobufDecoder creates a new ProtobufDecoder with optional configuration
func NewProtobufDecoder(options *encoding.DecodingOptions) *ProtobufDecoder {
	if options == nil {
		options = &encoding.DecodingOptions{}
	}
	return &ProtobufDecoder{
		options: options,
	}
}

// Decode decodes a single event from protobuf binary format
func (d *ProtobufDecoder) Decode(data []byte) (events.Event, error) {
	if len(data) == 0 {
		return nil, &encoding.DecodingError{
			Format:  "protobuf",
			Message: "cannot decode empty data",
		}
	}

	// Check size limit
	if d.options.MaxSize > 0 && int64(len(data)) > d.options.MaxSize {
		return nil, &encoding.DecodingError{
			Format:  "protobuf",
			Data:    data,
			Message: fmt.Sprintf("input size %d exceeds maximum %d", len(data), d.options.MaxSize),
		}
	}

	// Unmarshal protobuf
	var pbEvent generated.Event
	if err := proto.Unmarshal(data, &pbEvent); err != nil {
		return nil, &encoding.DecodingError{
			Format:  "protobuf",
			Data:    data,
			Message: "failed to unmarshal protobuf",
			Cause:   err,
		}
	}

	// Convert to internal event type
	event, err := protobufToEvent(&pbEvent)
	if err != nil {
		return nil, &encoding.DecodingError{
			Format:  "protobuf",
			Data:    data,
			Message: "failed to convert protobuf to event",
			Cause:   err,
		}
	}

	// Validate event if requested
	if d.options.ValidateEvents {
		if err := event.Validate(); err != nil {
			return nil, &encoding.DecodingError{
				Format:  "protobuf",
				Data:    data,
				Message: "event validation failed",
				Cause:   err,
			}
		}
	}

	return event, nil
}

// DecodeMultiple decodes multiple events from length-prefixed format
func (d *ProtobufDecoder) DecodeMultiple(data []byte) ([]events.Event, error) {
	if len(data) == 0 {
		return nil, &encoding.DecodingError{
			Format:  "protobuf",
			Message: "cannot decode empty data",
		}
	}

	// Check size limit
	if d.options.MaxSize > 0 && int64(len(data)) > d.options.MaxSize {
		return nil, &encoding.DecodingError{
			Format:  "protobuf",
			Data:    data,
			Message: fmt.Sprintf("input size %d exceeds maximum %d", len(data), d.options.MaxSize),
		}
	}

	// Check if it's length-prefixed format
	if len(data) >= 4 {
		// Try to read as length-prefixed format
		offset := 0
		count := binary.BigEndian.Uint32(data[offset:])
		offset += 4
		
		// Sanity check
		if count > 0 && count < 100000 { // reasonable upper limit
			result := make([]events.Event, 0, count)
			
			for i := uint32(0); i < count; i++ {
				if offset+4 > len(data) {
					break // Not enough data, fall back to single event
				}
				
				length := binary.BigEndian.Uint32(data[offset:])
				offset += 4
				
				if offset+int(length) > len(data) {
					break // Not enough data, fall back to single event
				}
				
				event, err := d.Decode(data[offset : offset+int(length)])
				if err != nil {
					break // Decoding failed, fall back to single event
				}
				
				result = append(result, event)
				offset += int(length)
			}
			
			// If we successfully decoded all events, return them
			if len(result) == int(count) {
				return result, nil
			}
		}
	}

	// Fall back to trying as single event
	event, err := d.Decode(data)
	if err != nil {
		return nil, err
	}
	return []events.Event{event}, nil
}

// ContentType returns the MIME type for protobuf
func (d *ProtobufDecoder) ContentType() string {
	return "application/x-protobuf"
}

// CanStream indicates that protobuf supports streaming
func (d *ProtobufDecoder) CanStream() bool {
	return true
}

// protobufToEvent converts a protobuf Event to internal event type
func protobufToEvent(pbEvent *generated.Event) (events.Event, error) {
	if pbEvent == nil {
		return nil, fmt.Errorf("nil protobuf event")
	}

	switch evt := pbEvent.Event.(type) {
	case *generated.Event_TextMessageStart:
		return protobufToTextMessageStart(evt.TextMessageStart)
	case *generated.Event_TextMessageContent:
		return protobufToTextMessageContent(evt.TextMessageContent)
	case *generated.Event_TextMessageEnd:
		return protobufToTextMessageEnd(evt.TextMessageEnd)
	case *generated.Event_ToolCallStart:
		return protobufToToolCallStart(evt.ToolCallStart)
	case *generated.Event_ToolCallArgs:
		return protobufToToolCallArgs(evt.ToolCallArgs)
	case *generated.Event_ToolCallEnd:
		return protobufToToolCallEnd(evt.ToolCallEnd)
	case *generated.Event_StateSnapshot:
		return protobufToStateSnapshot(evt.StateSnapshot)
	case *generated.Event_StateDelta:
		return protobufToStateDelta(evt.StateDelta)
	case *generated.Event_MessagesSnapshot:
		return protobufToMessagesSnapshot(evt.MessagesSnapshot)
	case *generated.Event_Raw:
		return protobufToRaw(evt.Raw)
	case *generated.Event_Custom:
		return protobufToCustom(evt.Custom)
	case *generated.Event_RunStarted:
		return protobufToRunStarted(evt.RunStarted)
	case *generated.Event_RunFinished:
		return protobufToRunFinished(evt.RunFinished)
	case *generated.Event_RunError:
		return protobufToRunError(evt.RunError)
	case *generated.Event_StepStarted:
		return protobufToStepStarted(evt.StepStarted)
	case *generated.Event_StepFinished:
		return protobufToStepFinished(evt.StepFinished)
	default:
		return nil, fmt.Errorf("unknown event type: %T", evt)
	}
}

// Event conversion functions

func protobufToTextMessageStart(pb *generated.TextMessageStartEvent) (*events.TextMessageStartEvent, error) {
	return &events.TextMessageStartEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		MessageID: pb.MessageId,
		Role:      pb.Role,
	}, nil
}

func protobufToTextMessageContent(pb *generated.TextMessageContentEvent) (*events.TextMessageContentEvent, error) {
	return &events.TextMessageContentEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		MessageID: pb.MessageId,
		Delta:     pb.Delta,
	}, nil
}

func protobufToTextMessageEnd(pb *generated.TextMessageEndEvent) (*events.TextMessageEndEvent, error) {
	return &events.TextMessageEndEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		MessageID: pb.MessageId,
	}, nil
}

func protobufToToolCallStart(pb *generated.ToolCallStartEvent) (*events.ToolCallStartEvent, error) {
	return &events.ToolCallStartEvent{
		BaseEvent:       protobufToBaseEvent(pb.BaseEvent),
		ToolCallID:      pb.ToolCallId,
		ToolCallName:    pb.ToolCallName,
		ParentMessageID: pb.ParentMessageId,
	}, nil
}

func protobufToToolCallArgs(pb *generated.ToolCallArgsEvent) (*events.ToolCallArgsEvent, error) {
	return &events.ToolCallArgsEvent{
		BaseEvent:  protobufToBaseEvent(pb.BaseEvent),
		ToolCallID: pb.ToolCallId,
		Delta:      pb.Delta,
	}, nil
}

func protobufToToolCallEnd(pb *generated.ToolCallEndEvent) (*events.ToolCallEndEvent, error) {
	return &events.ToolCallEndEvent{
		BaseEvent:  protobufToBaseEvent(pb.BaseEvent),
		ToolCallID: pb.ToolCallId,
	}, nil
}

func protobufToStateSnapshot(pb *generated.StateSnapshotEvent) (*events.StateSnapshotEvent, error) {
	var snapshot interface{}
	if pb.Snapshot != nil {
		snapshot = structValueToInterface(pb.Snapshot)
	}

	return &events.StateSnapshotEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		Snapshot:  snapshot,
	}, nil
}

func protobufToStateDelta(pb *generated.StateDeltaEvent) (*events.StateDeltaEvent, error) {
	operations := make([]events.JSONPatchOperation, 0, len(pb.Delta))
	for _, p := range pb.Delta {
		operation := events.JSONPatchOperation{
			Op:   convertPatchOp(p.Op),
			Path: p.Path,
		}
		if p.Value != nil {
			operation.Value = structValueToInterface(p.Value)
		}
		if p.From != nil && *p.From != "" {
			operation.From = *p.From
		}
		operations = append(operations, operation)
	}

	return &events.StateDeltaEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		Delta:     operations,
	}, nil
}

func protobufToMessagesSnapshot(pb *generated.MessagesSnapshotEvent) (*events.MessagesSnapshotEvent, error) {
	messages := make([]events.Message, 0, len(pb.Messages))
	for _, msg := range pb.Messages {
		if msg != nil {
			messages = append(messages, protobufToMessage(msg))
		}
	}

	return &events.MessagesSnapshotEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		Messages:  messages,
	}, nil
}

func protobufToRaw(pb *generated.RawEvent) (*events.RawEvent, error) {
	var data interface{}
	if pb.Event != nil {
		data = structValueToInterface(pb.Event)
	}
	
	return &events.RawEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		Event:     data,
		Source:    pb.Source,
	}, nil
}

func protobufToCustom(pb *generated.CustomEvent) (*events.CustomEvent, error) {
	var value interface{}
	if pb.Value != nil {
		value = structValueToInterface(pb.Value)
	}

	return &events.CustomEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		Name:      pb.Name,
		Value:     value,
	}, nil
}

func protobufToRunStarted(pb *generated.RunStartedEvent) (*events.RunStartedEvent, error) {
	return &events.RunStartedEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		ThreadID:  pb.ThreadId,
		RunID:     pb.RunId,
	}, nil
}

func protobufToRunFinished(pb *generated.RunFinishedEvent) (*events.RunFinishedEvent, error) {
	return &events.RunFinishedEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		ThreadID:  pb.ThreadId,
		RunID:     pb.RunId,
	}, nil
}

func protobufToRunError(pb *generated.RunErrorEvent) (*events.RunErrorEvent, error) {
	return &events.RunErrorEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		Code:      pb.Code,
		Message:   pb.Message,
	}, nil
}

func protobufToStepStarted(pb *generated.StepStartedEvent) (*events.StepStartedEvent, error) {
	return &events.StepStartedEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		StepName:  pb.StepName,
	}, nil
}

func protobufToStepFinished(pb *generated.StepFinishedEvent) (*events.StepFinishedEvent, error) {
	return &events.StepFinishedEvent{
		BaseEvent: protobufToBaseEvent(pb.BaseEvent),
		StepName:  pb.StepName,
	}, nil
}

// Helper functions

func protobufToBaseEvent(pb *generated.BaseEvent) *events.BaseEvent {
	if pb == nil {
		return &events.BaseEvent{}
	}

	baseEvent := &events.BaseEvent{
		EventType: protobufToEventType(pb.Type),
	}
	
	if pb.Timestamp != nil {
		baseEvent.TimestampMs = pb.Timestamp
	}
	
	// RawEvent field mapping if needed
	if pb.RawEvent != nil {
		baseEvent.RawEvent = structValueToInterface(pb.RawEvent)
	}
	
	return baseEvent
}

func protobufToEventType(pbType generated.EventType) events.EventType {
	switch pbType {
	case generated.EventType_TEXT_MESSAGE_START:
		return events.EventTypeTextMessageStart
	case generated.EventType_TEXT_MESSAGE_CONTENT:
		return events.EventTypeTextMessageContent
	case generated.EventType_TEXT_MESSAGE_END:
		return events.EventTypeTextMessageEnd
	case generated.EventType_TOOL_CALL_START:
		return events.EventTypeToolCallStart
	case generated.EventType_TOOL_CALL_ARGS:
		return events.EventTypeToolCallArgs
	case generated.EventType_TOOL_CALL_END:
		return events.EventTypeToolCallEnd
	case generated.EventType_STATE_SNAPSHOT:
		return events.EventTypeStateSnapshot
	case generated.EventType_STATE_DELTA:
		return events.EventTypeStateDelta
	case generated.EventType_MESSAGES_SNAPSHOT:
		return events.EventTypeMessagesSnapshot
	case generated.EventType_RAW:
		return events.EventTypeRaw
	case generated.EventType_CUSTOM:
		return events.EventTypeCustom
	case generated.EventType_RUN_STARTED:
		return events.EventTypeRunStarted
	case generated.EventType_RUN_FINISHED:
		return events.EventTypeRunFinished
	case generated.EventType_RUN_ERROR:
		return events.EventTypeRunError
	case generated.EventType_STEP_STARTED:
		return events.EventTypeStepStarted
	case generated.EventType_STEP_FINISHED:
		return events.EventTypeStepFinished
	default:
		return events.EventTypeUnknown
	}
}

// protobufToMetadata function removed as metadata is not used in BaseEvent

// protobufToCacheControl function removed as CacheControl is not defined in the protobuf schema

// convertPatchOp converts protobuf patch operation type to string
func convertPatchOp(op generated.JsonPatchOperationType) string {
	switch op {
	case generated.JsonPatchOperationType_ADD:
		return "add"
	case generated.JsonPatchOperationType_REMOVE:
		return "remove"
	case generated.JsonPatchOperationType_REPLACE:
		return "replace"
	case generated.JsonPatchOperationType_MOVE:
		return "move"
	case generated.JsonPatchOperationType_COPY:
		return "copy"
	case generated.JsonPatchOperationType_TEST:
		return "test"
	default:
		return "unknown"
	}
}

func structValueToInterface(v *structpb.Value) interface{} {
	if v == nil {
		return nil
	}
	
	switch val := v.Kind.(type) {
	case *structpb.Value_NullValue:
		return nil
	case *structpb.Value_NumberValue:
		return val.NumberValue
	case *structpb.Value_StringValue:
		return val.StringValue
	case *structpb.Value_BoolValue:
		return val.BoolValue
	case *structpb.Value_StructValue:
		m := make(map[string]interface{})
		if val.StructValue != nil {
			for k, v := range val.StructValue.Fields {
				m[k] = structValueToInterface(v)
			}
		}
		return m
	case *structpb.Value_ListValue:
		if val.ListValue == nil {
			return []interface{}{}
		}
		list := make([]interface{}, 0, len(val.ListValue.Values))
		for _, v := range val.ListValue.Values {
			list = append(list, structValueToInterface(v))
		}
		return list
	default:
		return nil
	}
}

func protobufToMessage(msg *generated.Message) events.Message {
	if msg == nil {
		return events.Message{}
	}
	
	message := events.Message{
		ID:   msg.Id,
		Role: msg.Role,
	}
	
	if msg.Content != nil {
		message.Content = msg.Content
	}
	
	if msg.Name != nil {
		message.Name = msg.Name
	}
	
	if msg.ToolCallId != nil {
		message.ToolCallID = msg.ToolCallId
	}
	
	if len(msg.ToolCalls) > 0 {
		toolCalls := make([]events.ToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			toolCall := events.ToolCall{
				ID:   tc.Id,
				Type: tc.Type,
			}
			if tc.Function != nil {
				toolCall.Function = events.Function{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				}
			}
			toolCalls = append(toolCalls, toolCall)
		}
		message.ToolCalls = toolCalls
	}
	
	return message
}