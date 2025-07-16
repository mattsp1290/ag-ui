package messages

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// TypedMessage is a generic interface for typed messages
type TypedMessage[T any] interface {
	Message
	GetTypedData() T
	SetTypedData(data T)
}

// TypedMessageList represents a collection of typed messages
type TypedMessageList[T any] []TypedMessage[T]

// Validate validates all messages in the typed list
func (tml TypedMessageList[T]) Validate() error {
	for i, msg := range tml {
		if err := msg.Validate(); err != nil {
			return fmt.Errorf("invalid message at index %d: %w", i, err)
		}
	}
	return nil
}

// ToJSON serializes the typed message list to JSON
func (tml TypedMessageList[T]) ToJSON() ([]byte, error) {
	return json.Marshal(tml)
}

// BaseTypedMessage contains common fields for all typed message types
type BaseTypedMessage[T any] struct {
	BaseMessage
	TypedData T `json:"typedData,omitempty"`
}

// GetTypedData returns the typed data
func (m *BaseTypedMessage[T]) GetTypedData() T {
	return m.TypedData
}

// SetTypedData sets the typed data
func (m *BaseTypedMessage[T]) SetTypedData(data T) {
	m.TypedData = data
}

// TypedUserMessage represents a typed message from a user
type TypedUserMessage[T any] struct {
	BaseTypedMessage[T]
}

// NewTypedUserMessage creates a new typed user message
func NewTypedUserMessage[T any](content string, data T) *TypedUserMessage[T] {
	msg := &TypedUserMessage[T]{
		BaseTypedMessage: BaseTypedMessage[T]{
			BaseMessage: BaseMessage{
				Role:    RoleUser,
				Content: &content,
			},
			TypedData: data,
		},
	}
	msg.ensureID()
	msg.ensureMetadata()
	return msg
}

// Accept implements the Visitable interface
func (m *TypedUserMessage[T]) Accept(v MessageVisitor) error {
	// For now, delegate to the base user message visitor
	userMsg := &UserMessage{BaseMessage: m.BaseMessage}
	return v.VisitUser(userMsg)
}

// Validate validates the typed user message
func (m *TypedUserMessage[T]) Validate() error {
	if err := m.Role.Validate(); err != nil {
		return err
	}
	if m.Content == nil || *m.Content == "" {
		return NewValidationError("user message content is required",
			ValidationViolation{
				Field:   "content",
				Message: "content is required",
				Value:   nil,
			})
	}
	return nil
}

// TypedAssistantMessage represents a typed message from an AI assistant
type TypedAssistantMessage[T any] struct {
	BaseTypedMessage[T]
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
}

// NewTypedAssistantMessage creates a new typed assistant message
func NewTypedAssistantMessage[T any](content string, data T) *TypedAssistantMessage[T] {
	msg := &TypedAssistantMessage[T]{
		BaseTypedMessage: BaseTypedMessage[T]{
			BaseMessage: BaseMessage{
				Role:    RoleAssistant,
				Content: &content,
			},
			TypedData: data,
		},
	}
	msg.ensureID()
	msg.ensureMetadata()
	return msg
}

// NewTypedAssistantMessageWithTools creates a new typed assistant message with tool calls
func NewTypedAssistantMessageWithTools[T any](toolCalls []ToolCall, data T) *TypedAssistantMessage[T] {
	msg := &TypedAssistantMessage[T]{
		BaseTypedMessage: BaseTypedMessage[T]{
			BaseMessage: BaseMessage{
				Role: RoleAssistant,
			},
			TypedData: data,
		},
		ToolCalls: toolCalls,
	}
	msg.ensureID()
	msg.ensureMetadata()
	return msg
}

// Accept implements the Visitable interface
func (m *TypedAssistantMessage[T]) Accept(v MessageVisitor) error {
	// For now, delegate to the base assistant message visitor
	assistantMsg := &AssistantMessage{
		BaseMessage: m.BaseMessage,
		ToolCalls:   m.ToolCalls,
	}
	return v.VisitAssistant(assistantMsg)
}

// Validate validates the typed assistant message
func (m *TypedAssistantMessage[T]) Validate() error {
	if err := m.Role.Validate(); err != nil {
		return err
	}
	// Content is optional for assistant messages (can have only tool calls)
	if m.Content == nil && len(m.ToolCalls) == 0 {
		return fmt.Errorf("assistant message must have either content or tool calls")
	}
	// Validate tool calls if present
	for i, tc := range m.ToolCalls {
		if tc.ID == "" {
			return fmt.Errorf("tool call at index %d missing ID", i)
		}
		if tc.Type != "function" {
			return fmt.Errorf("tool call at index %d has invalid type: %s", i, tc.Type)
		}
		if tc.Function.Name == "" {
			return fmt.Errorf("tool call at index %d missing function name", i)
		}
	}
	return nil
}

// ToJSON serializes the typed assistant message to JSON
func (m *TypedAssistantMessage[T]) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// TypedSystemMessage represents a typed system-level message
type TypedSystemMessage[T any] struct {
	BaseTypedMessage[T]
}

// NewTypedSystemMessage creates a new typed system message
func NewTypedSystemMessage[T any](content string, data T) *TypedSystemMessage[T] {
	msg := &TypedSystemMessage[T]{
		BaseTypedMessage: BaseTypedMessage[T]{
			BaseMessage: BaseMessage{
				Role:    RoleSystem,
				Content: &content,
			},
			TypedData: data,
		},
	}
	msg.ensureID()
	msg.ensureMetadata()
	return msg
}

// Accept implements the Visitable interface
func (m *TypedSystemMessage[T]) Accept(v MessageVisitor) error {
	// For now, delegate to the base system message visitor
	systemMsg := &SystemMessage{BaseMessage: m.BaseMessage}
	return v.VisitSystem(systemMsg)
}

// Validate validates the typed system message
func (m *TypedSystemMessage[T]) Validate() error {
	if err := m.Role.Validate(); err != nil {
		return err
	}
	if m.Content == nil || *m.Content == "" {
		return fmt.Errorf("system message content is required")
	}
	return nil
}

// TypedToolMessage represents a typed tool execution result
type TypedToolMessage[T any] struct {
	BaseTypedMessage[T]
	ToolCallID string `json:"toolCallId"`
}

// NewTypedToolMessage creates a new typed tool message
func NewTypedToolMessage[T any](content string, toolCallID string, data T) *TypedToolMessage[T] {
	msg := &TypedToolMessage[T]{
		BaseTypedMessage: BaseTypedMessage[T]{
			BaseMessage: BaseMessage{
				Role:    RoleTool,
				Content: &content,
			},
			TypedData: data,
		},
		ToolCallID: toolCallID,
	}
	msg.ensureID()
	msg.ensureMetadata()
	return msg
}

// Accept implements the Visitable interface
func (m *TypedToolMessage[T]) Accept(v MessageVisitor) error {
	// For now, delegate to the base tool message visitor
	toolMsg := &ToolMessage{
		BaseMessage: m.BaseMessage,
		ToolCallID:  m.ToolCallID,
	}
	return v.VisitTool(toolMsg)
}

// Validate validates the typed tool message
func (m *TypedToolMessage[T]) Validate() error {
	if err := m.Role.Validate(); err != nil {
		return err
	}
	if m.Content == nil || *m.Content == "" {
		return fmt.Errorf("tool message content is required")
	}
	if m.ToolCallID == "" {
		return fmt.Errorf("tool message toolCallId is required")
	}
	return nil
}

// TypedDeveloperMessage represents a typed developer/debug message
type TypedDeveloperMessage[T any] struct {
	BaseTypedMessage[T]
}

// NewTypedDeveloperMessage creates a new typed developer message
func NewTypedDeveloperMessage[T any](content string, data T) *TypedDeveloperMessage[T] {
	msg := &TypedDeveloperMessage[T]{
		BaseTypedMessage: BaseTypedMessage[T]{
			BaseMessage: BaseMessage{
				Role:    RoleDeveloper,
				Content: &content,
			},
			TypedData: data,
		},
	}
	msg.ensureID()
	msg.ensureMetadata()
	return msg
}

// Accept implements the Visitable interface
func (m *TypedDeveloperMessage[T]) Accept(v MessageVisitor) error {
	// For now, delegate to the base developer message visitor
	devMsg := &DeveloperMessage{BaseMessage: m.BaseMessage}
	return v.VisitDeveloper(devMsg)
}

// Validate validates the typed developer message
func (m *TypedDeveloperMessage[T]) Validate() error {
	if err := m.Role.Validate(); err != nil {
		return err
	}
	if m.Content == nil || *m.Content == "" {
		return fmt.Errorf("developer message content is required")
	}
	return nil
}

// TypedConversation represents a conversation thread with typed messages
type TypedConversation[T any] struct {
	ID       string
	Messages TypedMessageList[T]
	Options  ConversationOptions
}

// NewTypedConversation creates a new typed conversation
func NewTypedConversation[T any](options ...ConversationOptions) *TypedConversation[T] {
	opts := ConversationOptions{
		MaxMessages:            1000,
		PreserveSystemMessages: true,
	}
	if len(options) > 0 {
		opts = options[0]
	}

	return &TypedConversation[T]{
		ID:       uuid.New().String(),
		Messages: make(TypedMessageList[T], 0),
		Options:  opts,
	}
}

// AddMessage adds a typed message to the conversation
func (c *TypedConversation[T]) AddMessage(msg TypedMessage[T]) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	c.Messages = append(c.Messages, msg)

	// Apply message limits if needed
	if c.Options.MaxMessages > 0 && len(c.Messages) > c.Options.MaxMessages {
		c.pruneMessages()
	}

	return nil
}

// pruneMessages removes old messages while preserving system messages if configured
func (c *TypedConversation[T]) pruneMessages() {
	if !c.Options.PreserveSystemMessages {
		// Simple case: just keep the last N messages
		startIdx := len(c.Messages) - c.Options.MaxMessages
		c.Messages = c.Messages[startIdx:]
		return
	}

	// Complex case: preserve system messages
	var systemMessages []TypedMessage[T]
	var otherMessages []TypedMessage[T]

	for _, msg := range c.Messages {
		if msg.GetRole() == RoleSystem {
			systemMessages = append(systemMessages, msg)
		} else {
			otherMessages = append(otherMessages, msg)
		}
	}

	// Calculate how many non-system messages we can keep
	nonSystemSlots := c.Options.MaxMessages - len(systemMessages)
	if nonSystemSlots < 0 {
		nonSystemSlots = 0
	}

	// Keep the most recent non-system messages
	if len(otherMessages) > nonSystemSlots {
		startIdx := len(otherMessages) - nonSystemSlots
		otherMessages = otherMessages[startIdx:]
	}

	// Rebuild the message list
	c.Messages = make(TypedMessageList[T], 0, len(systemMessages)+len(otherMessages))
	c.Messages = append(c.Messages, systemMessages...)
	c.Messages = append(c.Messages, otherMessages...)
}

// GetMessagesByRole returns all messages with the specified role
func (c *TypedConversation[T]) GetMessagesByRole(role MessageRole) TypedMessageList[T] {
	var filtered TypedMessageList[T]
	for _, msg := range c.Messages {
		if msg.GetRole() == role {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// GetLastMessage returns the last message in the conversation
func (c *TypedConversation[T]) GetLastMessage() TypedMessage[T] {
	if len(c.Messages) == 0 {
		return nil
	}
	return c.Messages[len(c.Messages)-1]
}

// GetLastUserMessage returns the last user message in the conversation
func (c *TypedConversation[T]) GetLastUserMessage() TypedMessage[T] {
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if c.Messages[i].GetRole() == RoleUser {
			return c.Messages[i]
		}
	}
	return nil
}

// GetLastAssistantMessage returns the last assistant message in the conversation
func (c *TypedConversation[T]) GetLastAssistantMessage() TypedMessage[T] {
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if c.Messages[i].GetRole() == RoleAssistant {
			return c.Messages[i]
		}
	}
	return nil
}