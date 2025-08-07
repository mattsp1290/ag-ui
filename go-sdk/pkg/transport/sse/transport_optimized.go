package sse

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"unsafe"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
)

// Pre-allocated string constants to avoid allocations
const (
	prefixData  = "data:"
	prefixEvent = "event:"
	prefixID    = "id:"
	prefixRetry = "retry:"
)

// String intern pool for event types
var eventTypeIntern = &sync.Map{}

// ByteSlicePool manages pooled byte slices for parsing
var byteSlicePool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 1024)
		return &b
	},
}

// getPooledByteSlice gets a byte slice from the pool
func getPooledByteSlice() *[]byte {
	return byteSlicePool.Get().(*[]byte)
}

// putPooledByteSlice returns a byte slice to the pool
func putPooledByteSlice(b *[]byte) {
	*b = (*b)[:0]
	byteSlicePool.Put(b)
}

// stringToBytes converts string to byte slice without allocation
func stringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}

// bytesToString converts byte slice to string without allocation
func bytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// internEventType interns an event type string to reduce allocations
func internEventType(eventType string) string {
	if v, ok := eventTypeIntern.Load(eventType); ok {
		return v.(string)
	}
	eventTypeIntern.Store(eventType, eventType)
	return eventType
}

// optimizedParseSSEEvent parses SSE events with minimal allocations
func (t *SSETransport) optimizedParseSSEEvent(eventType, data, id string, retry int) (events.Event, error) {
	if data == "" {
		return nil, nil
	}

	// Use pooled decoder to avoid allocations
	dec := decoderPool.Get().(*json.Decoder)
	defer func() {
		// Reset decoder for reuse
		dec = json.NewDecoder(strings.NewReader(""))
		decoderPool.Put(dec)
	}()

	dec = json.NewDecoder(strings.NewReader(data))

	// Pre-allocate map with estimated capacity
	eventData := make(map[string]interface{}, 8)
	if err := dec.Decode(&eventData); err != nil {
		return nil, messages.NewConversionError("json", "event", eventType,
			"failed to parse event data: "+err.Error())
	}

	// Intern event type to reduce string allocations
	if eventType == "" {
		if typeField, ok := eventData["type"].(string); ok {
			eventType = internEventType(typeField)
		} else {
			eventType = "unknown"
		}
	} else {
		eventType = internEventType(eventType)
	}

	return t.createOptimizedEvent(eventType, eventData)
}

// Decoder pool to avoid creating new decoders
var decoderPool = sync.Pool{
	New: func() interface{} {
		return json.NewDecoder(strings.NewReader(""))
	},
}

// Pre-allocated event type constants
var (
	eventTypeTextMessageStart   = string(events.EventTypeTextMessageStart)
	eventTypeTextMessageContent = string(events.EventTypeTextMessageContent)
	eventTypeTextMessageEnd     = string(events.EventTypeTextMessageEnd)
	eventTypeToolCallStart      = string(events.EventTypeToolCallStart)
	eventTypeToolCallArgs       = string(events.EventTypeToolCallArgs)
	eventTypeToolCallEnd        = string(events.EventTypeToolCallEnd)
	eventTypeStateSnapshot      = string(events.EventTypeStateSnapshot)
	eventTypeStateDelta         = string(events.EventTypeStateDelta)
	eventTypeMessagesSnapshot   = string(events.EventTypeMessagesSnapshot)
	eventTypeRunStarted         = string(events.EventTypeRunStarted)
	eventTypeRunFinished        = string(events.EventTypeRunFinished)
	eventTypeRunError           = string(events.EventTypeRunError)
	eventTypeStepStarted        = string(events.EventTypeStepStarted)
	eventTypeStepFinished       = string(events.EventTypeStepFinished)
	eventTypeRaw                = string(events.EventTypeRaw)
	eventTypeCustom             = string(events.EventTypeCustom)
)

// createOptimizedEvent creates events using string comparison instead of reflection
func (t *SSETransport) createOptimizedEvent(eventType string, data map[string]interface{}) (events.Event, error) {
	// Use string comparison instead of type conversion for better performance
	switch eventType {
	case eventTypeTextMessageStart:
		return t.parseTextMessageStartEvent(data)
	case eventTypeTextMessageContent:
		return t.parseTextMessageContentEvent(data)
	case eventTypeTextMessageEnd:
		return t.parseTextMessageEndEvent(data)
	case eventTypeToolCallStart:
		return t.parseToolCallStartEvent(data)
	case eventTypeToolCallArgs:
		return t.parseToolCallArgsEvent(data)
	case eventTypeToolCallEnd:
		return t.parseToolCallEndEvent(data)
	case eventTypeStateSnapshot:
		return t.parseStateSnapshotEvent(data)
	case eventTypeStateDelta:
		return t.parseStateDeltaEventOptimized(data)
	case eventTypeMessagesSnapshot:
		return t.parseMessagesSnapshotEventOptimized(data)
	case eventTypeRunStarted:
		return t.parseRunStartedEvent(data)
	case eventTypeRunFinished:
		return t.parseRunFinishedEvent(data)
	case eventTypeRunError:
		return t.parseRunErrorEvent(data)
	case eventTypeStepStarted:
		return t.parseStepStartedEvent(data)
	case eventTypeStepFinished:
		return t.parseStepFinishedEvent(data)
	case eventTypeRaw:
		return t.parseRawEvent(data)
	case eventTypeCustom:
		return t.parseCustomEvent(data)
	default:
		return t.parseUnknownEvent(eventType, data)
	}
}

// parseStateDeltaEventOptimized parses state delta with pre-allocated slices
func (t *SSETransport) parseStateDeltaEventOptimized(data map[string]interface{}) (events.Event, error) {
	deltaData, ok := data["delta"].([]interface{})
	if !ok {
		return nil, messages.NewConversionError("json", "event", "STATE_DELTA",
			"delta field is required and must be an array")
	}

	// Pre-allocate slice with exact capacity
	delta := make([]events.JSONPatchOperation, 0, len(deltaData))

	for _, opData := range deltaData {
		opMap, ok := opData.(map[string]interface{})
		if !ok {
			return nil, messages.NewConversionError("json", "event", "STATE_DELTA",
				"delta operations must be objects")
		}

		op := events.JSONPatchOperation{}

		// Use type assertions instead of reflection
		if opType, ok := opMap["op"].(string); ok {
			op.Op = internString(opType)
		}

		if path, ok := opMap["path"].(string); ok {
			op.Path = path
		}

		if value, ok := opMap["value"]; ok {
			op.Value = value
		}

		if from, ok := opMap["from"].(string); ok {
			op.From = from
		}

		delta = append(delta, op)
	}

	event := events.NewStateDeltaEvent(delta)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

// parseMessagesSnapshotEventOptimized parses messages with pre-allocated slices
func (t *SSETransport) parseMessagesSnapshotEventOptimized(data map[string]interface{}) (events.Event, error) {
	messagesData, ok := data["messages"].([]interface{})
	if !ok {
		return nil, messages.NewConversionError("json", "event", "MESSAGES_SNAPSHOT",
			"messages field is required and must be an array")
	}

	// Pre-allocate with exact capacity
	messagesList := make([]events.Message, 0, len(messagesData))

	for _, msgData := range messagesData {
		msgMap, ok := msgData.(map[string]interface{})
		if !ok {
			return nil, messages.NewConversionError("json", "event", "MESSAGES_SNAPSHOT",
				"messages must be objects")
		}

		msg := events.Message{}

		// Direct type assertions
		if id, ok := msgMap["id"].(string); ok {
			msg.ID = id
		}

		if role, ok := msgMap["role"].(string); ok {
			msg.Role = internString(role)
		}

		if content, ok := msgMap["content"].(string); ok {
			msg.Content = &content
		}

		if name, ok := msgMap["name"].(string); ok {
			msg.Name = &name
		}

		if toolCallID, ok := msgMap["toolCallId"].(string); ok {
			msg.ToolCallID = &toolCallID
		}

		// Parse tool calls with pre-allocation
		if toolCallsData, ok := msgMap["toolCalls"].([]interface{}); ok {
			msg.ToolCalls = make([]events.ToolCall, 0, len(toolCallsData))

			for _, tcData := range toolCallsData {
				tcMap, ok := tcData.(map[string]interface{})
				if !ok {
					continue
				}

				toolCall := events.ToolCall{}

				if id, ok := tcMap["id"].(string); ok {
					toolCall.ID = id
				}

				if tcType, ok := tcMap["type"].(string); ok {
					toolCall.Type = internString(tcType)
				}

				if functionData, ok := tcMap["function"].(map[string]interface{}); ok {
					if name, ok := functionData["name"].(string); ok {
						toolCall.Function.Name = name
					}

					if args, ok := functionData["arguments"].(string); ok {
						toolCall.Function.Arguments = args
					}
				}

				msg.ToolCalls = append(msg.ToolCalls, toolCall)
			}
		}

		messagesList = append(messagesList, msg)
	}

	event := events.NewMessagesSnapshotEvent(messagesList)

	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

// optimizedReadEvent reads events with reduced allocations
func (t *SSETransport) optimizedReadEvent() (events.Event, error) {
	if t.reader == nil {
		return nil, messages.NewStreamingError("transport", 0, "no active connection")
	}

	// Use pooled buffers
	buf := getPooledByteSlice()
	defer putPooledByteSlice(buf)

	var eventType, data, id string
	var retry int

	for {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			return nil, messages.NewStreamingError("transport", 0, "failed to read line: "+err.Error())
		}

		// Trim in-place without allocation
		line = strings.TrimRight(line, "\n\r")

		if line == "" {
			if data != "" {
				return t.optimizedParseSSEEvent(eventType, data, id, retry)
			}
			continue
		}

		// Use switch for better performance than multiple HasPrefix calls
		switch {
		case len(line) > 5 && line[:5] == prefixData:
			// Avoid string concatenation
			if data == "" {
				data = line[5:]
			} else {
				// Use builder for multiple concatenations
				var b strings.Builder
				b.Grow(len(data) + len(line) - 4)
				b.WriteString(data)
				b.WriteString(line[5:])
				data = b.String()
			}
			if len(data) > 0 && data[0] == ' ' {
				data = data[1:]
			}

		case len(line) > 6 && line[:6] == prefixEvent:
			eventType = strings.TrimSpace(line[6:])

		case len(line) > 3 && line[:3] == prefixID:
			id = strings.TrimSpace(line[3:])

		case len(line) > 6 && line[:6] == prefixRetry:
			// Parse retry value efficiently
			retryStr := strings.TrimSpace(line[6:])
			// Simple integer parsing without time.ParseDuration
			retry = 0
			for i := 0; i < len(retryStr); i++ {
				if retryStr[i] >= '0' && retryStr[i] <= '9' {
					retry = retry*10 + int(retryStr[i]-'0')
				}
			}
		}
	}
}

// internString interns a string to reduce allocations
func internString(s string) string {
	if v, ok := eventTypeIntern.Load(s); ok {
		return v.(string)
	}
	eventTypeIntern.Store(s, s)
	return s
}

// FormatSSEEventOptimized formats events with minimal allocations
func FormatSSEEventOptimized(event events.Event) (string, error) {
	if event == nil {
		return "", messages.NewValidationError("event must not be nil")
	}

	eventData, err := event.ToJSON()
	if err != nil {
		return "", messages.NewConversionError("event", "json", string(event.Type()), err.Error())
	}

	// Pre-calculate size to avoid reallocations
	eventType := event.Type()
	size := 7 + len(eventType) + 7 + len(eventData) + 2 // "event: " + type + "\ndata: " + data + "\n\n"

	if event.Timestamp() != nil {
		size += 20 // Approximate size for "id: <timestamp>\n"
	}

	// Use bytes.Buffer with pre-allocated capacity
	var buf bytes.Buffer
	buf.Grow(size)

	buf.WriteString("event: ")
	buf.WriteString(string(eventType))
	buf.WriteByte('\n')

	buf.WriteString("data: ")
	buf.Write(eventData)
	buf.WriteByte('\n')

	if ts := event.Timestamp(); ts != nil {
		buf.WriteString("id: ")
		// Fast integer to string conversion
		buf.Write(appendInt(nil, *ts))
		buf.WriteByte('\n')
	}

	buf.WriteByte('\n')

	return buf.String(), nil
}

// appendInt appends an int64 to a byte slice (faster than strconv.FormatInt)
func appendInt(dst []byte, i int64) []byte {
	if i < 0 {
		dst = append(dst, '-')
		i = -i
	}

	var buf [20]byte
	j := len(buf)

	for {
		j--
		buf[j] = byte('0' + i%10)
		i /= 10
		if i == 0 {
			break
		}
	}

	return append(dst, buf[j:]...)
}
