package events

import (
	"fmt"
	"time"
)

// EventDataType interface ensures all event data types implement required methods
type EventDataType interface {
	// Validate ensures the event data is valid
	Validate() error
	
	// ToMap converts the event data to a map[string]interface{} for backward compatibility
	ToMap() map[string]interface{}
	
	// FromMap populates the event data from a map[string]interface{} for backward compatibility
	FromMap(data map[string]interface{}) error
	
	// DataType returns the type identifier for this data
	DataType() string
}

// MessageEventData represents typed data for message events
type MessageEventData struct {
	MessageID string  `json:"messageId"`
	Role      *string `json:"role,omitempty"`
	Delta     string  `json:"delta,omitempty"`
}

// Validate ensures the message event data is valid
func (m MessageEventData) Validate() error {
	if m.MessageID == "" {
		return fmt.Errorf("messageId field is required")
	}
	return nil
}

// ToMap converts the message event data to a map for backward compatibility
func (m MessageEventData) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"messageId": m.MessageID,
	}
	if m.Role != nil {
		result["role"] = *m.Role
	}
	if m.Delta != "" {
		result["delta"] = m.Delta
	}
	return result
}

// FromMap populates the message event data from a map for backward compatibility
func (m *MessageEventData) FromMap(data map[string]interface{}) error {
	if messageID, ok := data["messageId"].(string); ok {
		m.MessageID = messageID
	} else {
		return fmt.Errorf("messageId field is required and must be a string")
	}
	
	if role, ok := data["role"].(string); ok {
		m.Role = &role
	}
	
	if delta, ok := data["delta"].(string); ok {
		m.Delta = delta
	}
	
	return nil
}

// DataType returns the type identifier for this data
func (m MessageEventData) DataType() string {
	return "message"
}

// ToolCallEventData represents typed data for tool call events
type ToolCallEventData struct {
	ToolCallID      string  `json:"toolCallId"`
	ToolCallName    string  `json:"toolCallName,omitempty"`
	ParentMessageID *string `json:"parentMessageId,omitempty"`
	Delta           string  `json:"delta,omitempty"`
}

// Validate ensures the tool call event data is valid
func (t ToolCallEventData) Validate() error {
	if t.ToolCallID == "" {
		return fmt.Errorf("toolCallId field is required")
	}
	return nil
}

// ToMap converts the tool call event data to a map for backward compatibility
func (t ToolCallEventData) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"toolCallId": t.ToolCallID,
	}
	if t.ToolCallName != "" {
		result["toolCallName"] = t.ToolCallName
	}
	if t.ParentMessageID != nil {
		result["parentMessageId"] = *t.ParentMessageID
	}
	if t.Delta != "" {
		result["delta"] = t.Delta
	}
	return result
}

// FromMap populates the tool call event data from a map for backward compatibility
func (t *ToolCallEventData) FromMap(data map[string]interface{}) error {
	if toolCallID, ok := data["toolCallId"].(string); ok {
		t.ToolCallID = toolCallID
	} else {
		return fmt.Errorf("toolCallId field is required and must be a string")
	}
	
	if toolCallName, ok := data["toolCallName"].(string); ok {
		t.ToolCallName = toolCallName
	}
	
	if parentMessageID, ok := data["parentMessageId"].(string); ok {
		t.ParentMessageID = &parentMessageID
	}
	
	if delta, ok := data["delta"].(string); ok {
		t.Delta = delta
	}
	
	return nil
}

// DataType returns the type identifier for this data
func (t ToolCallEventData) DataType() string {
	return "toolCall"
}

// RunEventData represents typed data for run events
type RunEventData struct {
	RunID    string  `json:"runId"`
	ThreadID string  `json:"threadId,omitempty"`
	Message  string  `json:"message,omitempty"`
	Code     *string `json:"code,omitempty"`
}

// Validate ensures the run event data is valid
func (r RunEventData) Validate() error {
	if r.RunID == "" {
		return fmt.Errorf("runId field is required")
	}
	return nil
}

// ToMap converts the run event data to a map for backward compatibility
func (r RunEventData) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"runId": r.RunID,
	}
	if r.ThreadID != "" {
		result["threadId"] = r.ThreadID
	}
	if r.Message != "" {
		result["message"] = r.Message
	}
	if r.Code != nil {
		result["code"] = *r.Code
	}
	return result
}

// FromMap populates the run event data from a map for backward compatibility
func (r *RunEventData) FromMap(data map[string]interface{}) error {
	if runID, ok := data["runId"].(string); ok {
		r.RunID = runID
	} else {
		return fmt.Errorf("runId field is required and must be a string")
	}
	
	if threadID, ok := data["threadId"].(string); ok {
		r.ThreadID = threadID
	}
	
	if message, ok := data["message"].(string); ok {
		r.Message = message
	}
	
	if code, ok := data["code"].(string); ok {
		r.Code = &code
	}
	
	return nil
}

// DataType returns the type identifier for this data
func (r RunEventData) DataType() string {
	return "run"
}

// StepEventData represents typed data for step events
type StepEventData struct {
	StepName string `json:"stepName"`
}

// Validate ensures the step event data is valid
func (s StepEventData) Validate() error {
	if s.StepName == "" {
		return fmt.Errorf("stepName field is required")
	}
	return nil
}

// ToMap converts the step event data to a map for backward compatibility
func (s StepEventData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"stepName": s.StepName,
	}
}

// FromMap populates the step event data from a map for backward compatibility
func (s *StepEventData) FromMap(data map[string]interface{}) error {
	if stepName, ok := data["stepName"].(string); ok {
		s.StepName = stepName
	} else {
		return fmt.Errorf("stepName field is required and must be a string")
	}
	return nil
}

// DataType returns the type identifier for this data
func (s StepEventData) DataType() string {
	return "step"
}

// StateSnapshotEventData represents typed data for state snapshot events
type StateSnapshotEventData struct {
	Snapshot interface{} `json:"snapshot"`
}

// Validate ensures the state snapshot event data is valid
func (s StateSnapshotEventData) Validate() error {
	if s.Snapshot == nil {
		return fmt.Errorf("snapshot field is required")
	}
	return nil
}

// ToMap converts the state snapshot event data to a map for backward compatibility
func (s StateSnapshotEventData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"snapshot": s.Snapshot,
	}
}

// FromMap populates the state snapshot event data from a map for backward compatibility
func (s *StateSnapshotEventData) FromMap(data map[string]interface{}) error {
	s.Snapshot = data["snapshot"]
	if s.Snapshot == nil {
		return fmt.Errorf("snapshot field is required")
	}
	return nil
}

// DataType returns the type identifier for this data
func (s StateSnapshotEventData) DataType() string {
	return "stateSnapshot"
}

// StateDeltaEventData represents typed data for state delta events
type StateDeltaEventData struct {
	Delta []JSONPatchOperation `json:"delta"`
}

// Validate ensures the state delta event data is valid
func (s StateDeltaEventData) Validate() error {
	if len(s.Delta) == 0 {
		return fmt.Errorf("delta field must contain at least one operation")
	}
	
	// Validate each operation
	for i, op := range s.Delta {
		if err := validateJSONPatchOperation(op); err != nil {
			return fmt.Errorf("invalid operation at index %d: %w", i, err)
		}
	}
	
	return nil
}

// ToMap converts the state delta event data to a map for backward compatibility
func (s StateDeltaEventData) ToMap() map[string]interface{} {
	deltaSlice := make([]interface{}, len(s.Delta))
	for i, op := range s.Delta {
		deltaSlice[i] = map[string]interface{}{
			"op":    op.Op,
			"path":  op.Path,
			"value": op.Value,
			"from":  op.From,
		}
	}
	return map[string]interface{}{
		"delta": deltaSlice,
	}
}

// FromMap populates the state delta event data from a map for backward compatibility
func (s *StateDeltaEventData) FromMap(data map[string]interface{}) error {
	deltaRaw, ok := data["delta"].([]interface{})
	if !ok {
		return fmt.Errorf("delta field is required and must be an array")
	}
	
	s.Delta = make([]JSONPatchOperation, len(deltaRaw))
	for i, opRaw := range deltaRaw {
		opMap, ok := opRaw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("delta operation at index %d must be an object", i)
		}
		
		op := JSONPatchOperation{}
		if opStr, ok := opMap["op"].(string); ok {
			op.Op = opStr
		}
		if pathStr, ok := opMap["path"].(string); ok {
			op.Path = pathStr
		}
		op.Value = opMap["value"]
		if fromStr, ok := opMap["from"].(string); ok {
			op.From = fromStr
		}
		
		s.Delta[i] = op
	}
	
	return nil
}

// DataType returns the type identifier for this data
func (s StateDeltaEventData) DataType() string {
	return "stateDelta"
}

// MessagesSnapshotEventData represents typed data for messages snapshot events
type MessagesSnapshotEventData struct {
	Messages []Message `json:"messages"`
}

// Validate ensures the messages snapshot event data is valid
func (m MessagesSnapshotEventData) Validate() error {
	for i, msg := range m.Messages {
		if err := validateMessage(msg); err != nil {
			return fmt.Errorf("invalid message at index %d: %w", i, err)
		}
	}
	return nil
}

// ToMap converts the messages snapshot event data to a map for backward compatibility
func (m MessagesSnapshotEventData) ToMap() map[string]interface{} {
	messagesSlice := make([]interface{}, len(m.Messages))
	for i, msg := range m.Messages {
		msgMap := map[string]interface{}{
			"id":   msg.ID,
			"role": msg.Role,
		}
		if msg.Content != nil {
			msgMap["content"] = *msg.Content
		}
		if msg.Name != nil {
			msgMap["name"] = *msg.Name
		}
		if msg.ToolCallID != nil {
			msgMap["toolCallId"] = *msg.ToolCallID
		}
		if len(msg.ToolCalls) > 0 {
			toolCallsSlice := make([]interface{}, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				toolCallsSlice[j] = map[string]interface{}{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				}
			}
			msgMap["toolCalls"] = toolCallsSlice
		}
		messagesSlice[i] = msgMap
	}
	return map[string]interface{}{
		"messages": messagesSlice,
	}
}

// FromMap populates the messages snapshot event data from a map for backward compatibility
func (m *MessagesSnapshotEventData) FromMap(data map[string]interface{}) error {
	messagesRaw, ok := data["messages"].([]interface{})
	if !ok {
		return fmt.Errorf("messages field is required and must be an array")
	}
	
	m.Messages = make([]Message, len(messagesRaw))
	for i, msgRaw := range messagesRaw {
		msgMap, ok := msgRaw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("message at index %d must be an object", i)
		}
		
		msg := Message{}
		if id, ok := msgMap["id"].(string); ok {
			msg.ID = id
		}
		if role, ok := msgMap["role"].(string); ok {
			msg.Role = role
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
		
		if toolCallsRaw, ok := msgMap["toolCalls"].([]interface{}); ok {
			msg.ToolCalls = make([]ToolCall, len(toolCallsRaw))
			for j, tcRaw := range toolCallsRaw {
				tcMap, ok := tcRaw.(map[string]interface{})
				if !ok {
					continue
				}
				
				tc := ToolCall{}
				if id, ok := tcMap["id"].(string); ok {
					tc.ID = id
				}
				if tcType, ok := tcMap["type"].(string); ok {
					tc.Type = tcType
				}
				if funcRaw, ok := tcMap["function"].(map[string]interface{}); ok {
					if name, ok := funcRaw["name"].(string); ok {
						tc.Function.Name = name
					}
					if args, ok := funcRaw["arguments"].(string); ok {
						tc.Function.Arguments = args
					}
				}
				msg.ToolCalls[j] = tc
			}
		}
		
		m.Messages[i] = msg
	}
	
	return nil
}

// DataType returns the type identifier for this data
func (m MessagesSnapshotEventData) DataType() string {
	return "messagesSnapshot"
}

// RawEventData represents typed data for raw events
type RawEventData struct {
	Event  interface{} `json:"event"`
	Source *string     `json:"source,omitempty"`
}

// Validate ensures the raw event data is valid
func (r RawEventData) Validate() error {
	if r.Event == nil {
		return fmt.Errorf("event field is required")
	}
	return nil
}

// ToMap converts the raw event data to a map for backward compatibility
func (r RawEventData) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"event": r.Event,
	}
	if r.Source != nil {
		result["source"] = *r.Source
	}
	return result
}

// FromMap populates the raw event data from a map for backward compatibility
func (r *RawEventData) FromMap(data map[string]interface{}) error {
	r.Event = data["event"]
	if r.Event == nil {
		return fmt.Errorf("event field is required")
	}
	
	if source, ok := data["source"].(string); ok {
		r.Source = &source
	}
	
	return nil
}

// DataType returns the type identifier for this data
func (r RawEventData) DataType() string {
	return "raw"
}

// CustomEventData represents typed data for custom events
type CustomEventData struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value,omitempty"`
}

// Validate ensures the custom event data is valid
func (c CustomEventData) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name field is required")
	}
	return nil
}

// ToMap converts the custom event data to a map for backward compatibility
func (c CustomEventData) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"name": c.Name,
	}
	if c.Value != nil {
		result["value"] = c.Value
	}
	return result
}

// FromMap populates the custom event data from a map for backward compatibility
func (c *CustomEventData) FromMap(data map[string]interface{}) error {
	if name, ok := data["name"].(string); ok {
		c.Name = name
	} else {
		return fmt.Errorf("name field is required and must be a string")
	}
	
	c.Value = data["value"]
	
	return nil
}

// DataType returns the type identifier for this data
func (c CustomEventData) DataType() string {
	return "custom"
}