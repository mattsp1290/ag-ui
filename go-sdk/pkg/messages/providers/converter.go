package providers

import (
	"fmt"

	"github.com/ag-ui/go-sdk/pkg/messages"
)

// Converter defines the interface for converting messages to/from provider formats
type Converter interface {
	// ToProviderFormat converts AG-UI messages to provider-specific format
	ToProviderFormat(messages.MessageList) (interface{}, error)

	// FromProviderFormat converts provider-specific format to AG-UI messages
	FromProviderFormat(interface{}) (messages.MessageList, error)

	// GetProviderName returns the name of the provider
	GetProviderName() string

	// SupportsStreaming indicates if the provider supports streaming
	SupportsStreaming() bool
}

// Registry manages provider converters
type Registry struct {
	converters map[string]Converter
}

// NewRegistry creates a new converter registry
func NewRegistry() *Registry {
	return &Registry{
		converters: make(map[string]Converter),
	}
}

// Register registers a new converter
func (r *Registry) Register(converter Converter) error {
	name := converter.GetProviderName()
	if _, exists := r.converters[name]; exists {
		return fmt.Errorf("converter for provider %s already registered", name)
	}
	r.converters[name] = converter
	return nil
}

// Get retrieves a converter by provider name
func (r *Registry) Get(providerName string) (Converter, error) {
	converter, exists := r.converters[providerName]
	if !exists {
		return nil, fmt.Errorf("no converter found for provider %s", providerName)
	}
	return converter, nil
}

// ListProviders returns a list of all registered provider names
func (r *Registry) ListProviders() []string {
	providers := make([]string, 0, len(r.converters))
	for name := range r.converters {
		providers = append(providers, name)
	}
	return providers
}

// DefaultRegistry is the global converter registry
var DefaultRegistry = NewRegistry()

// Register registers a converter with the default registry
func Register(converter Converter) error {
	return DefaultRegistry.Register(converter)
}

// Get retrieves a converter from the default registry
func Get(providerName string) (Converter, error) {
	return DefaultRegistry.Get(providerName)
}

// ConversionOptions provides options for message conversion
type ConversionOptions struct {
	// MaxTokens limits the total tokens in the conversation
	MaxTokens int

	// TruncateStrategy defines how to handle token limits
	TruncateStrategy TruncateStrategy

	// IncludeSystemMessages indicates whether to include system messages
	IncludeSystemMessages bool

	// MergeConsecutiveMessages indicates whether to merge consecutive messages from the same role
	MergeConsecutiveMessages bool
}

// TruncateStrategy defines how to handle message truncation
type TruncateStrategy int

const (
	// TruncateOldest removes the oldest messages first
	TruncateOldest TruncateStrategy = iota

	// TruncateMiddle removes messages from the middle of the conversation
	TruncateMiddle

	// TruncateSystemFirst removes system messages before user/assistant messages
	TruncateSystemFirst
)

// BaseConverter provides common functionality for converters
type BaseConverter struct {
	options ConversionOptions
}

// NewBaseConverter creates a new base converter with default options
func NewBaseConverter() *BaseConverter {
	return &BaseConverter{
		options: ConversionOptions{
			IncludeSystemMessages:    true,
			MergeConsecutiveMessages: false,
		},
	}
}

// SetOptions sets the conversion options
func (c *BaseConverter) SetOptions(options ConversionOptions) {
	c.options = options
}

// PreprocessMessages applies common preprocessing to messages
func (c *BaseConverter) PreprocessMessages(msgList messages.MessageList) messages.MessageList {
	processed := make(messages.MessageList, 0, len(msgList))

	// Filter system messages if needed
	for _, msg := range msgList {
		if msg.GetRole() == messages.RoleSystem && !c.options.IncludeSystemMessages {
			continue
		}
		processed = append(processed, msg)
	}

	// Merge consecutive messages if enabled
	if c.options.MergeConsecutiveMessages {
		processed = c.mergeConsecutiveMessages(processed)
	}

	return processed
}

// mergeConsecutiveMessages merges consecutive messages from the same role
func (c *BaseConverter) mergeConsecutiveMessages(msgList messages.MessageList) messages.MessageList {
	if len(msgList) <= 1 {
		return msgList
	}

	merged := make(messages.MessageList, 0, len(msgList))
	current := msgList[0]

	for i := 1; i < len(msgList); i++ {
		next := msgList[i]

		// Check if we can merge
		if current.GetRole() == next.GetRole() &&
			current.GetRole() != messages.RoleTool && // Don't merge tool messages
			current.GetContent() != nil && next.GetContent() != nil {
			// Merge the content
			mergedContent := *current.GetContent() + "\n\n" + *next.GetContent()

			// Create a new message with merged content
			switch current.GetRole() {
			case messages.RoleUser:
				current = messages.NewUserMessage(mergedContent)
			case messages.RoleAssistant:
				current = messages.NewAssistantMessage(mergedContent)
			case messages.RoleSystem:
				current = messages.NewSystemMessage(mergedContent)
			case messages.RoleDeveloper:
				current = messages.NewDeveloperMessage(mergedContent)
			}
		} else {
			// Can't merge, add current and move to next
			merged = append(merged, current)
			current = next
		}
	}

	// Don't forget the last message
	merged = append(merged, current)

	return merged
}

// ConversionValidationOptions provides options for message validation during conversion
type ConversionValidationOptions struct {
	AllowStandaloneToolMessages bool
}

// ValidateMessages validates that messages are in a valid format for conversion
func ValidateMessages(msgList messages.MessageList, opts ...ConversionValidationOptions) error {
	if len(msgList) == 0 {
		return messages.ErrEmptyMessageList()
	}

	// Apply default options
	options := ConversionValidationOptions{
		AllowStandaloneToolMessages: false,
	}
	if len(opts) > 0 {
		options = opts[0]
	}

	// Validate each message
	if err := msgList.Validate(); err != nil {
		return err
	}

	// Additional validation rules
	// 1. Tool messages must follow assistant messages with tool calls (unless allowed standalone)
	var lastAssistantWithTools *messages.AssistantMessage

	for i, msg := range msgList {
		switch m := msg.(type) {
		case *messages.ToolMessage:
			if !options.AllowStandaloneToolMessages && lastAssistantWithTools == nil {
				return messages.NewValidationError(
					fmt.Sprintf("tool message at index %d has no preceding assistant message with tool calls", i),
					messages.ValidationViolation{
						Field:   "toolMessage",
						Message: "requires preceding assistant message with tool calls",
						Value:   i,
					})
			}

			// Verify the tool call ID exists (only if we have a preceding assistant message)
			if lastAssistantWithTools != nil {
				found := false
				for _, tc := range lastAssistantWithTools.ToolCalls {
					if tc.ID == m.ToolCallID {
						found = true
						break
					}
				}
				if !found {
					return messages.ErrMissingToolCallReference(m.ToolCallID, i)
				}
			}

		case *messages.AssistantMessage:
			if len(m.ToolCalls) > 0 {
				lastAssistantWithTools = m
			}
		}
	}

	return nil
}
