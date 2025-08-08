package utils

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
)

// FluentClient provides a fluent API for common AG-UI operations.
type FluentClient struct {
	client  *client.Client
	utils   *ClientUtils
	errors  []error
	context context.Context
}

// AgentBuilder provides a fluent API for agent configuration and creation.
type AgentBuilder struct {
	client      *FluentClient
	name        string
	config      *client.AgentConfig
	tools       []string
	state       interface{}
	handlers    []events.EventHandler
	retryPolicy *RetryPolicy
	errors      []error
	mu          sync.Mutex // Protects tools slice for concurrent access
}

// MessageBuilder provides a fluent API for message operations.
type MessageBuilder struct {
	client      *FluentClient
	messages    []messages.Message
	filters     []MessageFilter
	transformer string
	format      string
	errors      []error
}

// EventStreamBuilder provides a fluent API for event stream operations.
type EventStreamBuilder struct {
	client    *FluentClient
	agent     client.Agent
	filters   []EventFilter
	windows   []WindowConfig
	processor string
	handler   EventHandler
	errors    []error
}

// StateBuilder provides a fluent API for state operations.
type StateBuilder struct {
	client        *FluentClient
	agent         client.Agent
	compareWith   interface{}
	ignoreFields  []string
	diffOptions   *DiffOptions
	validatorName string
	format        OutputFormat
	errors        []error
}

// RetryPolicy defines retry behavior for operations.
type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts"`
	Delay       time.Duration `json:"delay"`
	BackoffType BackoffType   `json:"backoff_type"`
	MaxDelay    time.Duration `json:"max_delay"`
}

// BackoffType defines the type of backoff strategy.
type BackoffType string

const (
	BackoffTypeFixed       BackoffType = "fixed"
	BackoffTypeExponential BackoffType = "exponential"
	BackoffTypeLinear      BackoffType = "linear"
)

// ClientUtils aggregates all utility classes for easy access.
type ClientUtils struct {
	Agent       *AgentUtils
	State       *StateUtils
	Message     *MessageUtils
	Event       *EventUtils
	Diagnostics *DiagnosticsUtils
	Common      *CommonUtils
}

// NewFluentClient creates a new fluent API client.
func NewFluentClient(client *client.Client) *FluentClient {
	commonUtils := NewCommonUtils()
	utils := &ClientUtils{
		Agent:       NewAgentUtils(),
		State:       NewStateUtils(),
		Message:     NewMessageUtils(),
		Event:       NewEventUtils(),
		Diagnostics: NewDiagnosticsUtils(),
		Common:      commonUtils,
	}

	return &FluentClient{
		client:  client,
		utils:   utils,
		context: context.Background(),
	}
}

// WithContext sets the context for operations.
func (fc *FluentClient) WithContext(ctx context.Context) *FluentClient {
	fc.context = ctx
	return fc
}

// NewAgent starts building a new agent configuration.
func (fc *FluentClient) NewAgent(name string) *AgentBuilder {
	return &AgentBuilder{
		client: fc,
		name:   name,
		config: &client.AgentConfig{},
		tools:  nil, // More efficient than make([]string, 0)
		errors: nil, // More efficient than make([]error, 0)
	}
}

// Messages starts building message operations.
func (fc *FluentClient) Messages(messages []messages.Message) *MessageBuilder {
	return &MessageBuilder{
		client:   fc,
		messages: messages,
		filters:  nil, // More efficient than make([]MessageFilter, 0)
		errors:   nil, // More efficient than make([]error, 0)
	}
}

// EventStream starts building event stream operations.
func (fc *FluentClient) EventStream(agent client.Agent) *EventStreamBuilder {
	return &EventStreamBuilder{
		client:  fc,
		agent:   agent,
		filters: nil, // More efficient than make([]EventFilter, 0)
		windows: nil, // More efficient than make([]WindowConfig, 0)
		errors:  nil, // More efficient than make([]error, 0)
	}
}

// State starts building state operations.
func (fc *FluentClient) State(agent client.Agent) *StateBuilder {
	return &StateBuilder{
		client: fc,
		agent:  agent,
		errors: nil, // More efficient than make([]error, 0)
	}
}

// GetUtils returns the utility classes for direct access.
func (fc *FluentClient) GetUtils() *ClientUtils {
	return fc.utils
}

// HasErrors returns true if any errors have been accumulated.
func (fc *FluentClient) HasErrors() bool {
	return len(fc.errors) > 0
}

// GetErrors returns all accumulated errors.
func (fc *FluentClient) GetErrors() []error {
	return fc.errors
}

// AgentBuilder methods

// WithConfig sets the agent configuration.
func (ab *AgentBuilder) WithConfig(config *client.AgentConfig) *AgentBuilder {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	if config == nil {
		ab.errors = append(ab.errors, errors.NewValidationError("config", "config cannot be nil"))
	} else {
		ab.config = config
	}
	return ab
}

// WithTools adds tools to the agent.
func (ab *AgentBuilder) WithTools(tools ...string) *AgentBuilder {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	ab.tools = append(ab.tools, tools...)
	return ab
}

// GetTools returns a copy of the tools slice (thread-safe).
func (ab *AgentBuilder) GetTools() []string {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	// Return a copy to prevent external modification
	result := make([]string, len(ab.tools))
	copy(result, ab.tools)
	return result
}

// WithState sets the initial state for the agent.
func (ab *AgentBuilder) WithState(state interface{}) *AgentBuilder {
	ab.state = state
	return ab
}

// WithEventHandler adds an event handler to the agent.
func (ab *AgentBuilder) WithEventHandler(handler events.EventHandler) *AgentBuilder {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	if handler == nil {
		ab.errors = append(ab.errors, errors.NewValidationError("handler", "handler cannot be nil"))
	} else {
		ab.handlers = append(ab.handlers, handler)
	}
	return ab
}

// WithRetryPolicy sets the retry policy for the agent.
func (ab *AgentBuilder) WithRetryPolicy(policy *RetryPolicy) *AgentBuilder {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	if policy == nil {
		ab.errors = append(ab.errors, errors.NewValidationError("policy", "policy cannot be nil"))
	} else {
		ab.retryPolicy = policy
	}
	return ab
}

// Build creates the agent with the configured settings.
func (ab *AgentBuilder) Build() (client.Agent, error) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	
	// Return accumulated errors if any
	if len(ab.errors) > 0 {
		return nil, errors.NewValidationError("build", "agent build failed")
	}

	// Validate required fields using common utilities
	if err := ab.client.utils.Common.ValidateNotEmpty(ab.name, "name"); err != nil {
		return nil, err
	}

	// This is a placeholder implementation - in practice, you would use
	// your agent factory to create the agent with the specified configuration
	return nil, errors.NewOperationError("Build", "agent", fmt.Errorf("agent building not implemented - requires agent factory"))
}

// Clone creates a clone of an existing agent with new configuration.
func (ab *AgentBuilder) Clone(sourceAgent client.Agent) (client.Agent, error) {
	if sourceAgent == nil {
		return nil, errors.NewValidationError("sourceAgent", "source agent cannot be nil")
	}

	// Use AgentUtils to clone the agent
	clonedAgent, err := ab.client.utils.Agent.Clone(sourceAgent)
	if err != nil {
		return nil, err
	}

	// Apply any additional configuration to the cloned agent
	// This would involve updating the cloned agent's configuration
	// with the settings from this builder

	return clonedAgent, nil
}

// MessageBuilder methods

// FilterByType filters messages by content type.
func (mb *MessageBuilder) FilterByType(types ...string) *MessageBuilder {
	filter := &MessageTypeFilter{
		allowedTypes: types,
		name:         "type_filter",
	}
	mb.filters = append(mb.filters, filter)
	return mb
}

// FilterByRole filters messages by role.
func (mb *MessageBuilder) FilterByRole(roles ...messages.MessageRole) *MessageBuilder {
	filter := &RoleFilter{
		allowedRoles: roles,
		name:         "role_filter",
	}
	mb.filters = append(mb.filters, filter)
	return mb
}

// FilterByContent filters messages by content pattern.
func (mb *MessageBuilder) FilterByContent(pattern string) *MessageBuilder {
	// This would create a content filter - simplified implementation
	mb.errors = append(mb.errors, fmt.Errorf("content filter with pattern '%s' - implementation pending", pattern))
	return mb
}

// TransformWith applies a transformer to the messages.
func (mb *MessageBuilder) TransformWith(transformerName string) *MessageBuilder {
	mb.transformer = transformerName
	return mb
}

// ExportToJSON exports the filtered messages to JSON format.
func (mb *MessageBuilder) ExportToJSON() ([]byte, error) {
	if len(mb.errors) > 0 {
		return nil, errors.NewValidationError("export", "message export failed")
	}

	// Apply filters
	filteredMessages := mb.messages
	for _, filter := range mb.filters {
		filteredMessages = filter.Apply(filteredMessages)
	}

	// Apply transformer if specified
	if mb.transformer != "" {
		transformedMessages, err := mb.client.utils.Message.Transform(filteredMessages, mb.transformer)
		if err != nil {
			return nil, err
		}
		filteredMessages = transformedMessages
	}

	// Export to JSON
	return mb.client.utils.Message.Export(filteredMessages, "json")
}

// ExportToCSV exports the filtered messages to CSV format.
func (mb *MessageBuilder) ExportToCSV() ([]byte, error) {
	if len(mb.errors) > 0 {
		return nil, errors.NewValidationError("export", "message export failed")
	}

	// Apply filters and transformations (similar to JSON export)
	filteredMessages := mb.messages
	for _, filter := range mb.filters {
		filteredMessages = filter.Apply(filteredMessages)
	}

	return mb.client.utils.Message.Export(filteredMessages, "csv")
}

// GetStatistics returns statistics about the filtered messages.
func (mb *MessageBuilder) GetStatistics() (*MessageStats, error) {
	if len(mb.errors) > 0 {
		return nil, errors.NewValidationError("statistics", "message statistics failed")
	}

	// Apply filters
	filteredMessages := mb.messages
	for _, filter := range mb.filters {
		filteredMessages = filter.Apply(filteredMessages)
	}

	return mb.client.utils.Message.Statistics(filteredMessages), nil
}

// EventStreamBuilder methods

// Filter adds an event filter.
func (esb *EventStreamBuilder) Filter(filter EventFilter) *EventStreamBuilder {
	if filter != nil {
		esb.filters = append(esb.filters, filter)
	} else {
		esb.errors = append(esb.errors, errors.NewValidationError("filter", "filter cannot be nil"))
	}
	return esb
}

// Window adds a windowing configuration.
func (esb *EventStreamBuilder) Window(windowType WindowType, size int, duration time.Duration) *EventStreamBuilder {
	config := WindowConfig{
		Type:     windowType,
		Size:     size,
		Duration: duration,
	}
	esb.windows = append(esb.windows, config)
	return esb
}

// Process sets the processor for the event stream.
func (esb *EventStreamBuilder) Process(processorName string) *EventStreamBuilder {
	esb.processor = processorName
	return esb
}

// Subscribe subscribes to the processed event stream.
func (esb *EventStreamBuilder) Subscribe(handler EventHandler) (<-chan events.Event, error) {
	if len(esb.errors) > 0 {
		return nil, errors.NewValidationError("subscribe", "event stream subscription failed")
	}

	if esb.agent == nil {
		return nil, errors.NewValidationError("agent", "agent is required for event stream")
	}

	// Create or get event processor
	processorName := esb.processor
	if processorName == "" {
		processorName = fmt.Sprintf("stream_%s_%d", esb.agent.Name(), time.Now().Unix())
	}

	processor := esb.client.utils.Event.CreateProcessor(processorName, 1000)

	// Add filters
	for _, filter := range esb.filters {
		processor.AddFilter(filter)
	}

	// Add windows
	for _, window := range esb.windows {
		processor.AddWindow(window)
	}

	// Add handler
	if handler != nil {
		processor.AddHandler(handler)
	}

	// Start processor
	err := processor.Start()
	if err != nil {
		return nil, err
	}

	return processor.Output(), nil
}

// StateBuilder methods

// Compare sets the state to compare with.
func (sb *StateBuilder) Compare(previousState interface{}) *StateBuilder {
	sb.compareWith = previousState
	return sb
}

// IgnoreFields sets fields to ignore during comparison.
func (sb *StateBuilder) IgnoreFields(fields ...string) *StateBuilder {
	sb.ignoreFields = append(sb.ignoreFields, fields...)
	return sb
}

// WithDiffOptions sets diff options for comparison.
func (sb *StateBuilder) WithDiffOptions(options *DiffOptions) *StateBuilder {
	if options != nil {
		sb.diffOptions = options
	} else {
		sb.errors = append(sb.errors, errors.NewValidationError("options", "diff options cannot be nil"))
	}
	return sb
}

// ValidateWith sets the validator to use for state validation.
func (sb *StateBuilder) ValidateWith(validatorName string) *StateBuilder {
	sb.validatorName = validatorName
	return sb
}

// GenerateDiff generates a diff between current and previous state.
func (sb *StateBuilder) GenerateDiff() (*StateDiff, error) {
	if len(sb.errors) > 0 {
		return nil, errors.NewValidationError("diff", "state diff generation failed")
	}

	if sb.agent == nil {
		return nil, errors.NewValidationError("agent", "agent is required for state diff")
	}

	// Get current state
	stateManager, ok := sb.agent.(client.StateManager)
	if !ok {
		return nil, errors.NewOperationError("GenerateDiff", "state",
			fmt.Errorf("agent does not support state management"))
	}

	currentState, err := stateManager.GetState(sb.client.context)
	if err != nil {
		return nil, err
	}

	if sb.compareWith == nil {
		return nil, errors.NewValidationError("compareWith", "previous state is required for diff")
	}

	// Set up diff options
	options := sb.diffOptions
	if options == nil {
		options = &DiffOptions{
			IgnoreFields:   sb.ignoreFields,
			MaxDepth:       10,
			ComparisonMode: ComparisonModeDeep,
			OutputFormat:   OutputFormatJSON,
		}
	} else {
		// Merge ignore fields
		options.IgnoreFields = append(options.IgnoreFields, sb.ignoreFields...)
	}

	return sb.client.utils.State.Diff(currentState, sb.compareWith, options)
}

// AsJSON exports the state as JSON.
func (sb *StateBuilder) AsJSON() ([]byte, error) {
	if len(sb.errors) > 0 {
		return nil, errors.NewValidationError("export", "state export failed")
	}

	if sb.agent == nil {
		return nil, errors.NewValidationError("agent", "agent is required for state export")
	}

	// Get current state
	stateManager, ok := sb.agent.(client.StateManager)
	if !ok {
		return nil, errors.NewOperationError("AsJSON", "state",
			fmt.Errorf("agent does not support state management"))
	}

	state, err := stateManager.GetState(sb.client.context)
	if err != nil {
		return nil, err
	}

	return sb.client.utils.State.Export(state, OutputFormatJSON)
}

// Validate validates the current state.
func (sb *StateBuilder) Validate() (*ValidationResult, error) {
	if len(sb.errors) > 0 {
		return nil, errors.NewValidationError("validate", "state validation failed")
	}

	if sb.agent == nil {
		return nil, errors.NewValidationError("agent", "agent is required for state validation")
	}

	if sb.validatorName == "" {
		return nil, errors.NewValidationError("validator", "validator name is required")
	}

	// Get current state
	stateManager, ok := sb.agent.(client.StateManager)
	if !ok {
		return nil, errors.NewOperationError("Validate", "state",
			fmt.Errorf("agent does not support state management"))
	}

	state, err := stateManager.GetState(sb.client.context)
	if err != nil {
		return nil, err
	}

	return sb.client.utils.State.Validate(state, sb.validatorName)
}

// CreateSnapshot creates a snapshot of the current state.
func (sb *StateBuilder) CreateSnapshot(tags ...string) (*StateSnapshot, error) {
	if len(sb.errors) > 0 {
		return nil, errors.NewValidationError("snapshot", "state snapshot failed")
	}

	if sb.agent == nil {
		return nil, errors.NewValidationError("agent", "agent is required for state snapshot")
	}

	return sb.client.utils.State.CreateSnapshot(sb.agent, tags...)
}

// Helper methods and utilities

// DefaultRetryPolicy returns a default retry policy.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts: 3,
		Delay:       1 * time.Second,
		BackoffType: BackoffTypeExponential,
		MaxDelay:    30 * time.Second,
	}
}

// CreateTimeWindow creates a time-based window configuration.
func CreateTimeWindow(duration time.Duration) WindowConfig {
	return WindowConfig{
		Type:     WindowTypeTime,
		Duration: duration,
	}
}

// CreateCountWindow creates a count-based window configuration.
func CreateCountWindow(size int) WindowConfig {
	return WindowConfig{
		Type: WindowTypeCount,
		Size: size,
	}
}

// CreateSlidingWindow creates a sliding window configuration.
func CreateSlidingWindow(duration, overlap time.Duration) WindowConfig {
	return WindowConfig{
		Type:     WindowTypeSliding,
		Duration: duration,
		Overlap:  overlap,
	}
}

// Validation helpers

func (ab *AgentBuilder) validateName() error {
	if ab.name == "" {
		return errors.NewValidationError("name", "agent name cannot be empty")
	}

	if strings.TrimSpace(ab.name) == "" {
		return errors.NewValidationError("name", "agent name cannot be whitespace only")
	}

	return nil
}

func (ab *AgentBuilder) validateConfig() error {
	if ab.config == nil {
		return errors.NewValidationError("config", "agent config cannot be nil")
	}

	// Additional config validation would go here
	return nil
}

// Error aggregation helpers

func (fc *FluentClient) addError(err error) {
	if err != nil {
		fc.errors = append(fc.errors, err)
	}
}

func (fc *FluentClient) addErrors(errs []error) {
	fc.errors = append(fc.errors, errs...)
}

// Context helpers

func (fc *FluentClient) getContextWithTimeout(timeout time.Duration) context.Context {
	ctx, _ := context.WithTimeout(fc.context, timeout)
	return ctx
}

// Factory methods for common configurations

// NewDefaultAgentBuilder creates an agent builder with default settings.
func (fc *FluentClient) NewDefaultAgentBuilder(name string) *AgentBuilder {
	return fc.NewAgent(name).
		WithRetryPolicy(DefaultRetryPolicy())
}

// NewMessageFilterBuilder creates a message builder with common filters pre-configured.
func (fc *FluentClient) NewMessageFilterBuilder(msgs []messages.Message) *MessageBuilder {
	return fc.Messages(msgs).
		FilterByRole(messages.RoleUser, messages.RoleAssistant)
}

// NewEventProcessorBuilder creates an event stream builder with common settings.
func (fc *FluentClient) NewEventProcessorBuilder(agent client.Agent) *EventStreamBuilder {
	return fc.EventStream(agent).
		Window(WindowTypeTime, 0, 1*time.Minute)
}
