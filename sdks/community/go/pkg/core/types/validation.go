package types

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ValidateProtocol validates the canonical JSON emitted by this request. It does
// not change the request or make ordinary JSON unmarshaling strict. To check
// required keys before decoding loses their presence, use ValidateRunAgentInputJSON.
func (r RunAgentInput) ValidateProtocol() error {
	return validateProtocolValue(r, ValidateRunAgentInputJSON)
}

// ValidateProtocol validates this message as a protocol producer value.
// ValidateMessageJSON additionally checks required keys in original wire bytes.
func (m Message) ValidateProtocol() error { return validateProtocolValue(m, ValidateMessageJSON) }

// ValidateProtocol validates this content fragment, including its source.
// An empty MIME string is valid; an absent required MIME key is rejected by
// ValidateInputContentJSON when validating original wire bytes.
func (c InputContent) ValidateProtocol() error {
	return validateProtocolValue(c, ValidateInputContentJSON)
}

// ValidateProtocol validates this tool call's function discriminator and payload.
func (c ToolCall) ValidateProtocol() error { return validateProtocolValue(c, validateToolCallJSON) }

// ValidateProtocol validates the protocol JSON emitted by this interrupt.
func (i Interrupt) ValidateProtocol() error { return validateProtocolValue(i, ValidateInterruptJSON) }

func validateProtocolValue(value any, validate func([]byte) error) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return validate(data)
}

type protocolObject map[string]json.RawMessage

func readProtocolObject(data []byte) (protocolObject, error) {
	var object protocolObject
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("expected a JSON object")
	}
	return object, nil
}

func (o protocolObject) stringValue(name string, required bool) (string, error) {
	raw, ok := o[name]
	if !ok {
		if required {
			return "", fmt.Errorf("%s is required", name)
		}
		return "", nil
	}
	var value string
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func (o protocolObject) strings(required bool, names ...string) error {
	for _, name := range names {
		if _, err := o.stringValue(name, required); err != nil {
			return err
		}
	}
	return nil
}

func (o protocolObject) object(name string, required bool) (protocolObject, error) {
	raw, ok := o[name]
	if !ok {
		if required {
			return nil, fmt.Errorf("%s is required", name)
		}
		return nil, nil
	}
	value, err := readProtocolObject(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return value, nil
}

func (o protocolObject) array(name string, required bool, validate func([]byte) error) error {
	raw, ok := o[name]
	if !ok {
		if required {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return fmt.Errorf("%s must be an array", name)
	}
	if entries == nil {
		return fmt.Errorf("%s must be an array", name)
	}
	for index, entry := range entries {
		if err := validate(entry); err != nil {
			return fmt.Errorf("%s[%d]: %w", name, index, err)
		}
	}
	return nil
}

// ValidateInputContentJSON checks a canonical camelCase content payload without
// mutating or decoding it into a producer value. Unknown extension keys are
// ignored. Unlike ordinary UnmarshalJSON, this opt-in check enforces presence.
func ValidateInputContentJSON(data []byte) error {
	object, err := readProtocolObject(data)
	if err != nil {
		return err
	}
	kind, err := object.stringValue("type", true)
	if err != nil {
		return err
	}
	switch kind {
	case InputContentTypeText:
		return object.strings(true, "text")
	case InputContentTypeBinary:
		if err := object.strings(true, "mimeType"); err != nil {
			return err
		}
		if err := object.strings(false, "id", "url", "data", "filename"); err != nil {
			return err
		}
		for _, key := range []string{"id", "url", "data"} {
			value, _ := object.stringValue(key, false)
			if value != "" {
				return nil
			}
		}
		return fmt.Errorf("binary content requires at least one of id, url, or data")
	case InputContentTypeImage, InputContentTypeAudio, InputContentTypeVideo, InputContentTypeDocument:
		source, err := object.object("source", true)
		if err != nil {
			return err
		}
		sourceType, err := source.stringValue("type", true)
		if err != nil {
			return fmt.Errorf("source: %w", err)
		}
		if sourceType != InputContentSourceTypeData && sourceType != InputContentSourceTypeURL {
			return fmt.Errorf("unknown source type %q", sourceType)
		}
		if err := source.strings(true, "value"); err != nil {
			return fmt.Errorf("source: %w", err)
		}
		_, err = source.stringValue("mimeType", sourceType == InputContentSourceTypeData)
		return err
	default:
		return fmt.Errorf("unknown content type %q", kind)
	}
}

func validateToolCallJSON(data []byte) error {
	object, err := readProtocolObject(data)
	if err != nil {
		return err
	}
	if err := object.strings(true, "id"); err != nil {
		return err
	}
	kind, err := object.stringValue("type", true)
	if err != nil {
		return err
	}
	if kind != ToolCallTypeFunction {
		return fmt.Errorf("unknown tool call type %q", kind)
	}
	function, err := object.object("function", true)
	if err != nil {
		return err
	}
	if err := function.strings(true, "name", "arguments"); err != nil {
		return fmt.Errorf("function: %w", err)
	}
	if err := object.strings(false, "encryptedValue"); err != nil {
		return err
	}
	_, err = object.object("metadata", false)
	return err
}

// ValidateMessageJSON validates one canonical camelCase message, including
// nested content and tool calls. Ordinary Message.UnmarshalJSON stays permissive.
func ValidateMessageJSON(data []byte) error {
	object, err := readProtocolObject(data)
	if err != nil {
		return err
	}
	if err := object.strings(true, "id"); err != nil {
		return err
	}
	role, err := object.stringValue("role", true)
	if err != nil {
		return err
	}
	if err := object.strings(false, "subagentRunId"); err != nil {
		return err
	}
	if _, err := object.object("metadata", false); err != nil {
		return err
	}
	if Role(role) != RoleActivity {
		if err := object.strings(false, "encryptedValue"); err != nil {
			return err
		}
	}
	switch Role(role) {
	case RoleDeveloper, RoleSystem, RoleAssistant, RoleUser:
		if err := object.strings(false, "name"); err != nil {
			return err
		}
	}
	switch Role(role) {
	case RoleDeveloper, RoleSystem, RoleReasoning:
		return object.strings(true, "content")
	case RoleAssistant:
		if err := object.strings(false, "content"); err != nil {
			return err
		}
		return object.array("toolCalls", false, validateToolCallJSON)
	case RoleUser:
		if _, err := object.stringValue("content", true); err == nil {
			return nil
		}
		return object.array("content", true, ValidateInputContentJSON)
	case RoleTool:
		if err := object.strings(true, "content", "toolCallId"); err != nil {
			return err
		}
		return object.strings(false, "error")
	case RoleActivity:
		if err := object.strings(true, "activityType"); err != nil {
			return err
		}
		_, err := object.object("content", true)
		return err
	default:
		return fmt.Errorf("unknown message role %q", role)
	}
}

// ValidateInterruptJSON validates required interrupt keys before a Go string
// loses the distinction between omission and an explicitly empty string.
func ValidateInterruptJSON(data []byte) error {
	object, err := readProtocolObject(data)
	if err != nil {
		return err
	}
	if err := object.strings(true, "id", "reason"); err != nil {
		return err
	}
	if err := object.strings(false, "message", "toolCallId", "expiresAt", "subagentRunId"); err != nil {
		return err
	}
	if _, err := object.object("responseSchema", false); err != nil {
		return err
	}
	_, err = object.object("metadata", false)
	return err
}

func validateResumeJSON(data []byte) error {
	object, err := readProtocolObject(data)
	if err != nil {
		return err
	}
	if err := object.strings(true, "interruptId"); err != nil {
		return err
	}
	status, err := object.stringValue("status", true)
	if err != nil {
		return err
	}
	if status != string(ResumeStatusResolved) && status != string(ResumeStatusCancelled) {
		return fmt.Errorf("unknown resume status %q", status)
	}
	_, err = object.object("metadata", false)
	return err
}

func validateToolJSON(data []byte) error {
	object, err := readProtocolObject(data)
	if err != nil {
		return err
	}
	if err := object.strings(true, "name", "description"); err != nil {
		return err
	}
	_, err = object.object("metadata", false)
	return err
}

func validateContextJSON(data []byte) error {
	object, err := readProtocolObject(data)
	if err != nil {
		return err
	}
	return object.strings(true, "description", "value")
}

// ValidateRunAgentInputJSON validates canonical camelCase request bytes. It
// accepts arbitrary JSON state/payload values and unknown extension keys; it
// requires schema keys without imposing nonempty strings or URL/MIME policies.
// Snake-case aliases remain supported by ordinary UnmarshalJSON. Callers using
// aliases can decode first and call ValidateProtocol to validate emitted values.
func ValidateRunAgentInputJSON(data []byte) error {
	object, err := readProtocolObject(data)
	if err != nil {
		return err
	}
	if err := object.strings(true, "threadId", "runId"); err != nil {
		return err
	}
	if err := object.strings(false, "parentRunId"); err != nil {
		return err
	}
	if _, ok := object["forwardedProps"]; !ok {
		return fmt.Errorf("forwardedProps is required")
	}
	if err := object.array("messages", true, ValidateMessageJSON); err != nil {
		return err
	}
	if err := object.array("tools", true, validateToolJSON); err != nil {
		return err
	}
	if err := object.array("context", true, validateContextJSON); err != nil {
		return err
	}
	return object.array("resume", false, validateResumeJSON)
}
