# AG-UI Event Encoder/Decoder Compatibility

This document describes the JSON encoding/decoding compatibility between the Go SDK and the Server Starter (tool-based UI) SSE payloads.

## Overview

The Go SDK event types have been validated for roundtrip compatibility with the Server Starter protocol. All events can be:
1. Decoded from server JSON into Go structs
2. Re-encoded back to JSON with identical structure
3. Validated for field presence, casing, and values

## Compatibility Status ✅

All event types have been tested and verified for full compatibility:

### Message Events
- ✅ TEXT_MESSAGE_START
- ✅ TEXT_MESSAGE_CONTENT
- ✅ TEXT_MESSAGE_END
- ✅ TEXT_MESSAGE_CHUNK

### Tool Call Events
- ✅ TOOL_CALL_START
- ✅ TOOL_CALL_ARGS
- ✅ TOOL_CALL_END
- ✅ TOOL_CALL_CHUNK
- ✅ TOOL_CALL_RESULT

### Run/Step Events
- ✅ RUN_STARTED
- ✅ RUN_FINISHED
- ✅ RUN_ERROR
- ✅ STEP_STARTED
- ✅ STEP_FINISHED

### State Events
- ✅ STATE_SNAPSHOT
- ✅ STATE_DELTA (JSON Patch RFC 6902)
- ✅ MESSAGES_SNAPSHOT

### Thinking Events
- ✅ THINKING_START
- ✅ THINKING_END
- ✅ THINKING_TEXT_MESSAGE_START
- ✅ THINKING_TEXT_MESSAGE_CONTENT
- ✅ THINKING_TEXT_MESSAGE_END

### Other Events
- ✅ CUSTOM
- ✅ RAW

## Field Naming Convention

All JSON fields use **camelCase** naming to match the Server Starter protocol:

| Go Field | JSON Field | Example |
|----------|------------|---------|
| MessageID | messageId | "messageId": "msg-123" |
| ToolCallID | toolCallId | "toolCallId": "tool-456" |
| ThreadIDValue | threadId | "threadId": "thread-789" |
| RunIDValue | runId | "runId": "run-abc" |
| ParentMessageID | parentMessageId | "parentMessageId": "parent-def" |
| ToolCallName | toolCallName | "toolCallName": "generate_haiku" |
| StepName | stepName | "stepName": "data_processing" |

## Optional Fields

Optional fields are properly handled with `omitempty` tags:
- When nil/empty, fields are omitted from JSON output
- Server can send events with or without optional fields
- Decoder handles missing optional fields gracefully

Examples of optional fields:
- `role` in TEXT_MESSAGE_START
- `parentMessageId` in TOOL_CALL_START and TOOL_CALL_CHUNK
- `timestamp` in all events
- `result` in RUN_FINISHED

## Recent Fixes

### Version 1.0 (Current)
1. **TOOL_CALL_CHUNK**: Added missing `parentMessageId` field
2. **RUN_FINISHED**: Added optional `result` field for completion data

## Testing

### Roundtrip Tests
Located in `roundtrip_test.go`, these tests verify:
- JSON → Go struct decoding
- Go struct → JSON encoding
- Field name casing (camelCase)
- Optional field handling
- Value preservation

### Test Fixtures
Test fixtures in `testdata/fixtures/` contain:
- Representative samples from Server Starter
- Both minimal and complete event examples
- Edge cases and validation scenarios

### Helper Utilities
`json_helpers.go` provides utilities for:
- Semantic JSON comparison (ignoring field order)
- CamelCase validation
- JSON normalization
- Optional field handling

## Usage Examples

### Decoding Server Events
```go
// Decode SSE event from server
var event TextMessageStartEvent
err := json.Unmarshal(sseData, &event)
if err != nil {
    return err
}

// Validate the event
if err := event.Validate(); err != nil {
    return err
}
```

### Encoding Events for Transmission
```go
// Create an event
event := NewToolCallStartEvent("tool-123", "generate_haiku",
    WithParentMessageID("msg-456"))

// Encode to JSON
jsonData, err := event.ToJSON()
if err != nil {
    return err
}

// Send as SSE
fmt.Fprintf(w, "data: %s\n\n", jsonData)
```

## Protocol Deviations

Currently, there are **no intentional deviations** from the Server Starter protocol. The Go SDK implements full compatibility with:
- Field naming (camelCase)
- Optional field handling
- Event type constants
- Timestamp format (Unix milliseconds)

## Future Considerations

1. **Streaming Optimization**: Consider adding streaming-specific encoders for better performance
2. **Validation Modes**: Add strict vs lenient validation modes for different use cases
3. **Schema Evolution**: Maintain backward compatibility as the protocol evolves

## Verification

To verify encoder/decoder compatibility:

```bash
# Run roundtrip tests
go test ./pkg/core/events -run TestRoundtripCompatibility -v

# Run field naming tests
go test ./pkg/core/events -run TestFieldNamingConventions -v

# Run optional field tests
go test ./pkg/core/events -run TestOptionalFieldHandling -v

# Run all event tests
go test ./pkg/core/events -v
```

All tests should pass, confirming full compatibility with the Server Starter protocol.