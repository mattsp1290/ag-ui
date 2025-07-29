package providers

import (
	"fmt"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
)

// Converter defines the interface for converting messages to/from provider formats
// Deprecated: Use TypedConverter for type safety
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

// TypedConverter defines the type-safe interface for converting messages to/from provider formats.
// TRequest represents the type of the provider request format.
// TResponse represents the type of the provider response format.
// TMessageData represents the type of message-specific data (metadata, content, tool arguments, etc.)
type TypedConverter[TRequest, TResponse, TMessageData any] interface {
	// ToProviderFormat converts AG-UI messages to strongly-typed provider-specific format
	ToProviderFormat(messages.TypedMessageList[TMessageData]) (TRequest, error)

	// FromProviderFormat converts strongly-typed provider-specific format to AG-UI messages
	FromProviderFormat(TResponse) (messages.TypedMessageList[TMessageData], error)

	// GetProviderName returns the name of the provider
	GetProviderName() string

	// SupportsStreaming indicates if the provider supports streaming
	SupportsStreaming() bool
	
	// ValidateRequest validates a provider request before sending
	ValidateRequest(TRequest) error
	
	// ValidateResponse validates a provider response after receiving
	ValidateResponse(TResponse) error
}

// TypedStreamingConverter extends TypedConverter with streaming capabilities.
// TStreamEvent represents the type of streaming events.
type TypedStreamingConverter[TRequest, TResponse, TMessageData, TStreamEvent any] interface {
	TypedConverter[TRequest, TResponse, TMessageData]
	
	// ProcessStreamEvent processes a streaming event and returns the updated message
	ProcessStreamEvent(state TypedStreamingState[TMessageData], event TStreamEvent) (*messages.TypedAssistantMessage[TMessageData], error)
	
	// CreateStreamingState creates a new streaming state for message reconstruction
	CreateStreamingState() TypedStreamingState[TMessageData]
}

// TypedStreamingState maintains state for type-safe streaming message reconstruction
type TypedStreamingState[T any] interface {
	// Reset clears the streaming state for reuse
	Reset()
	
	// GetCurrentMessage returns the current message being built
	GetCurrentMessage() *messages.TypedAssistantMessage[T]
	
	// Size returns the current size of the streaming state
	Size() int
	
	// Cleanup releases resources held by the streaming state
	Cleanup()
}

// Registry manages provider converters
type Registry struct {
	converters map[string]Converter
}

// TypedRegistry manages type-safe provider converters.
// T represents the common message data type across all converters in this registry.
type TypedRegistry[T any] struct {
	converters map[string]interface{} // Stores various TypedConverter implementations
}

// NewTypedRegistry creates a new type-safe converter registry
func NewTypedRegistry[T any]() *TypedRegistry[T] {
	return &TypedRegistry[T]{
		converters: make(map[string]interface{}),
	}
}

// RegisterTypedConverter registers a new type-safe converter
func (tr *TypedRegistry[T]) RegisterTypedConverter(
	converter interface{},
) error {
	// Type assert to get provider name
	type providerNamer interface {
		GetProviderName() string
	}
	
	pn, ok := converter.(providerNamer)
	if !ok {
		return fmt.Errorf("converter does not implement GetProviderName method")
	}
	
	name := pn.GetProviderName()
	if _, exists := tr.converters[name]; exists {
		return fmt.Errorf("typed converter for provider %s already registered", name)
	}
	tr.converters[name] = converter
	return nil
}

// GetTypedConverter retrieves a type-safe converter by provider name
func (tr *TypedRegistry[T]) GetTypedConverter(
	providerName string,
) (interface{}, error) {
	conv, exists := tr.converters[providerName]
	if !exists {
		return nil, fmt.Errorf("no typed converter found for provider %s", providerName)
	}
	
	return conv, nil
}

// ListProviders returns a list of all registered provider names
func (tr *TypedRegistry[T]) ListProviders() []string {
	providers := make([]string, 0, len(tr.converters))
	for name := range tr.converters {
		providers = append(providers, name)
	}
	return providers
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

// DefaultTypedRegistry is the global type-safe converter registry for common message data
var DefaultTypedRegistry = NewTypedRegistry[map[string]interface{}]()

// RegisterTyped registers a type-safe converter with the default registry
func RegisterTyped[TRequest, TResponse any](
	converter TypedConverter[TRequest, TResponse, map[string]interface{}],
) error {
	return DefaultTypedRegistry.RegisterTypedConverter(converter)
}

// GetTyped retrieves a type-safe converter from the default registry
func GetTyped[TRequest, TResponse any](
	providerName string,
) (TypedConverter[TRequest, TResponse, map[string]interface{}], error) {
	conv, err := DefaultTypedRegistry.GetTypedConverter(providerName)
	if err != nil {
		return nil, err
	}
	
	// Type assert to the specific converter type
	typedConv, ok := conv.(TypedConverter[TRequest, TResponse, map[string]interface{}])
	if !ok {
		return nil, fmt.Errorf("converter for provider %s is not of the expected type", providerName)
	}
	
	return typedConv, nil
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

// TypedBaseConverter provides common functionality for type-safe converters.
// T represents the message data type.
type TypedBaseConverter[T any] struct {
	options        TypedConversionOptions[T]
	validateData   func(T) error
	sanitizeData   func(T) (T, error)
	transformData  func(T) (T, error)
}

// TypedConversionOptions provides type-safe options for message conversion
type TypedConversionOptions[T any] struct {
	// Legacy options
	ConversionOptions
	
	// Type-specific options
	ValidateCustomData   func(T) error
	SanitizeCustomData   func(T) (T, error)
	TransformCustomData  func(T) (T, error)
	
	// Schema validation
	DataSchema interface{} // JSON Schema or other validation schema
	
	// Performance options
	EnableDataCaching    bool
	EnableDataDedup      bool
	MaxDataCacheSize     int
}

// NewTypedBaseConverter creates a new type-safe base converter with default options
func NewTypedBaseConverter[T any]() *TypedBaseConverter[T] {
	return &TypedBaseConverter[T]{
		options: TypedConversionOptions[T]{
			ConversionOptions: ConversionOptions{
				IncludeSystemMessages:    true,
				MergeConsecutiveMessages: false,
			},
		},
	}
}

// SetTypedOptions sets the type-safe conversion options
func (tbc *TypedBaseConverter[T]) SetTypedOptions(options TypedConversionOptions[T]) {
	tbc.options = options
	tbc.validateData = options.ValidateCustomData
	tbc.sanitizeData = options.SanitizeCustomData
	tbc.transformData = options.TransformCustomData
}

// PreprocessTypedMessages applies common preprocessing to typed messages
func (tbc *TypedBaseConverter[T]) PreprocessTypedMessages(msgList messages.TypedMessageList[T]) messages.TypedMessageList[T] {
	processed := make(messages.TypedMessageList[T], 0, len(msgList))

	// Filter system messages if needed
	for _, msg := range msgList {
		if msg.GetRole() == messages.RoleSystem && !tbc.options.IncludeSystemMessages {
			continue
		}
		processed = append(processed, msg)
	}

	// Merge consecutive messages if enabled
	if tbc.options.MergeConsecutiveMessages {
		processed = tbc.mergeConsecutiveTypedMessages(processed)
	}

	return processed
}

// mergeConsecutiveTypedMessages merges consecutive typed messages from the same role
func (tbc *TypedBaseConverter[T]) mergeConsecutiveTypedMessages(msgList messages.TypedMessageList[T]) messages.TypedMessageList[T] {
	if len(msgList) <= 1 {
		return msgList
	}

	merged := make(messages.TypedMessageList[T], 0, len(msgList))
	current := msgList[0]

	for i := 1; i < len(msgList); i++ {
		next := msgList[i]

		// Check if we can merge (same role, both have content, not tool messages)
		if current.GetRole() == next.GetRole() &&
			current.GetRole() != messages.RoleTool &&
			current.GetContent() != nil && next.GetContent() != nil {
			
			// Merge the content
			mergedContent := *current.GetContent() + "\n\n" + *next.GetContent()

			// Create a new typed message with merged content based on role
			// For now, we'll keep the current message and update its content
			// This preserves the typed data while merging the content
			switch msg := current.(type) {
			case *messages.TypedUserMessage[T]:
				current = messages.NewTypedUserMessage(mergedContent, msg.GetTypedData())
			case *messages.TypedAssistantMessage[T]:
				current = messages.NewTypedAssistantMessage(mergedContent, msg.GetTypedData())
			case *messages.TypedSystemMessage[T]:
				current = messages.NewTypedSystemMessage(mergedContent, msg.GetTypedData())
			case *messages.TypedDeveloperMessage[T]:
				current = messages.NewTypedDeveloperMessage(mergedContent, msg.GetTypedData())
			default:
				// If we can't handle the type, just add current and move on
				merged = append(merged, current)
				current = next
				continue
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

// ValidateTypedData validates custom data using the configured validator
func (tbc *TypedBaseConverter[T]) ValidateTypedData(data T) error {
	if tbc.validateData != nil {
		return tbc.validateData(data)
	}
	return nil
}

// SanitizeTypedData sanitizes custom data using the configured sanitizer
func (tbc *TypedBaseConverter[T]) SanitizeTypedData(data T) (T, error) {
	if tbc.sanitizeData != nil {
		return tbc.sanitizeData(data)
	}
	return data, nil
}

// TransformTypedData transforms custom data using the configured transformer
func (tbc *TypedBaseConverter[T]) TransformTypedData(data T) (T, error) {
	if tbc.transformData != nil {
		return tbc.transformData(data)
	}
	return data, nil
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

// ValidateTypedMessages validates that typed messages are in a valid format for conversion
func ValidateTypedMessages[T any](msgList messages.TypedMessageList[T], opts ...TypedConversionValidationOptions[T]) error {
	if len(msgList) == 0 {
		return messages.ErrEmptyMessageList()
	}

	// Apply default options
	var options TypedConversionValidationOptions[T]
	if len(opts) > 0 {
		options = opts[0]
	} else {
		options = TypedConversionValidationOptions[T]{
			ConversionValidationOptions: ConversionValidationOptions{
				AllowStandaloneToolMessages: false,
			},
		}
	}

	// Validate each message
	if err := msgList.Validate(); err != nil {
		return err
	}

	// Type-specific validation
	if options.ValidateMessageData != nil {
		for i, msg := range msgList {
			// Extract typed data from message and validate
			// Note: This is a placeholder - actual implementation would need
			// to extract the typed data based on message type
			_ = i
			_ = msg
		}
	}

	// Additional validation rules for typed messages
	// Similar to legacy validation but with type safety

	return nil
}

// TypedConversionValidationOptions provides options for validating typed message conversion
type TypedConversionValidationOptions[T any] struct {
	// Legacy validation options
	ConversionValidationOptions
	
	// Type-specific validation
	ValidateMessageData func(T) error
	ValidateToolArgs    func(T) error
	ValidateMetadata    func(T) error
	
	// Schema validation
	DataSchema     interface{}
	ArgsSchema     interface{}
	MetadataSchema interface{}
}
