package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	pkgerrors "github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// Common client errors
var (
	ErrClientClosed = errors.New("client is closed")
	ErrAgentNotFound = errors.New("agent not found")
	ErrAgentAlreadyRegistered = errors.New("agent already registered")
)

// Client represents a connection to an AG-UI server.
type Client struct {
	// baseURL is the base URL of the AG-UI server
	baseURL *url.URL

	// config stores the client configuration
	config Config

	// closed indicates if the client has been closed
	closed bool

	// agents stores registered agents by name
	agents map[string]Agent

	// agentsMux protects the agents map for concurrent access
	agentsMux sync.RWMutex
}

// Config contains configuration options for the client.
type Config struct {
	// BaseURL is the base URL of the AG-UI server
	BaseURL string

	// Timeout for requests (default: 30 seconds)
	Timeout time.Duration

	// Authentication configuration
	AuthToken string

	// Retry configuration
	MaxRetries int

	// User agent string
	UserAgent string
}

// New creates a new AG-UI client with the specified configuration.
func New(config Config) (*Client, error) {
	if config.BaseURL == "" {
		return nil, pkgerrors.NewConfigurationErrorWithField("BaseURL", "base URL cannot be empty", config.BaseURL)
	}

	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, pkgerrors.NewConfigurationErrorWithField("BaseURL", "invalid base URL", config.BaseURL).WithCause(err)
	}

	// Set default values
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.UserAgent == "" {
		config.UserAgent = "ag-ui-go-sdk/1.0.0"
	}

	return &Client{
		baseURL: baseURL,
		config:  config,
		closed:  false,
		agents:  make(map[string]Agent),
	}, nil
}

// SendEvent sends an event to the specified agent and returns the response.
// If the agent is registered locally, it will process the event directly.
// Otherwise, it will send the event over the network (when transport layer is implemented).
func (c *Client) SendEvent(ctx context.Context, agentName string, event any) (responses []any, err error) {
	if c.closed {
		return nil, pkgerrors.NewOperationError("SendEvent", "client", ErrClientClosed)
	}

	defer func() {
		if err != nil {
			err = pkgerrors.WrapWithContext(err, "SendEvent", agentName)
		}
	}()

	if agentName == "" {
		return nil, pkgerrors.NewValidationErrorWithField("agentName", "required", "agent name cannot be empty", agentName)
	}

	if event == nil {
		return nil, pkgerrors.NewValidationErrorWithField("event", "required", "event cannot be nil", event)
	}

	// Validate event type if it has a type field
	if err := c.validateEventType(event); err != nil {
		return nil, err
	}

	// Use context with timeout
	if ctx == nil {
		ctx = context.Background()
	}
	
	timeoutCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	// Check if agent is registered locally
	c.agentsMux.RLock()
	_, exists := c.agents[agentName]
	c.agentsMux.RUnlock()

	if exists {
		// Send to registered agent directly
		return c.SendEventToAgent(timeoutCtx, agentName, event)
	}

	// Implementation placeholder - to be implemented when transport layer is added
	select {
	case <-timeoutCtx.Done():
		return nil, pkgerrors.NewTimeoutErrorWithOperation("SendEvent", c.config.Timeout, c.config.Timeout)
	default:
		return nil, pkgerrors.NewNotImplementedError("SendEvent", "transport layer")
	}
}

// Stream opens a streaming connection to the specified agent.
func (c *Client) Stream(ctx context.Context, agentName string) (eventChan <-chan any, err error) {
	if c.closed {
		return nil, pkgerrors.NewOperationError("Stream", "client", ErrClientClosed)
	}

	defer func() {
		if err != nil {
			err = pkgerrors.WrapWithContext(err, "Stream", agentName)
		}
	}()

	if agentName == "" {
		return nil, pkgerrors.NewValidationErrorWithField("agentName", "required", "agent name cannot be empty", agentName)
	}

	// Use context with timeout
	if ctx == nil {
		ctx = context.Background()
	}
	
	timeoutCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	// Implementation placeholder - to be implemented when transport layer is added
	select {
	case <-timeoutCtx.Done():
		return nil, pkgerrors.NewTimeoutErrorWithOperation("Stream", c.config.Timeout, c.config.Timeout)
	default:
		return nil, pkgerrors.NewNotImplementedError("Stream", "transport layer")
	}
}

// Close closes the client and releases any resources.
func (c *Client) Close() error {
	if c.closed {
		return ErrClientClosed
	}
	
	c.closed = true
	// Resource cleanup will be implemented when transport layer is added
	return nil
}

// validateEventType validates that the event has a valid type if it contains type information.
func (c *Client) validateEventType(event any) error {
	// For now, we accept any event type since this is for basic validation
	// Proper event type validation will be implemented when protobuf events are integrated
	if event == nil {
		return pkgerrors.NewValidationErrorWithField("event", "required", "event cannot be nil", event)
	}
	return nil
}

// IsClosed returns true if the client has been closed
func (c *Client) IsClosed() bool {
	return c.closed
}

// GetConfig returns a copy of the client configuration
func (c *Client) GetConfig() Config {
	return c.config
}

// RegisterAgent registers an agent with the client for local processing.
func (c *Client) RegisterAgent(agent Agent) error {
	if c.closed {
		return pkgerrors.NewOperationError("RegisterAgent", "client", ErrClientClosed)
	}

	if agent == nil {
		return pkgerrors.NewValidationErrorWithField("agent", "required", "agent cannot be nil", agent)
	}

	agentName := agent.Name()
	if agentName == "" {
		return pkgerrors.NewValidationErrorWithField("agent.Name()", "required", "agent name cannot be empty", agentName)
	}

	c.agentsMux.Lock()
	defer c.agentsMux.Unlock()

	if _, exists := c.agents[agentName]; exists {
		return pkgerrors.NewOperationError("RegisterAgent", agentName, ErrAgentAlreadyRegistered)
	}

	c.agents[agentName] = agent
	return nil
}

// UnregisterAgent removes an agent from the client.
func (c *Client) UnregisterAgent(name string) error {
	if c.closed {
		return pkgerrors.NewOperationError("UnregisterAgent", "client", ErrClientClosed)
	}

	if name == "" {
		return pkgerrors.NewValidationErrorWithField("name", "required", "agent name cannot be empty", name)
	}

	c.agentsMux.Lock()
	defer c.agentsMux.Unlock()

	if _, exists := c.agents[name]; !exists {
		return pkgerrors.NewOperationError("UnregisterAgent", name, ErrAgentNotFound)
	}

	delete(c.agents, name)
	return nil
}

// GetAgent retrieves a registered agent by name.
func (c *Client) GetAgent(name string) (Agent, error) {
	if c.closed {
		return nil, pkgerrors.NewOperationError("GetAgent", "client", ErrClientClosed)
	}

	if name == "" {
		return nil, pkgerrors.NewValidationErrorWithField("name", "required", "agent name cannot be empty", name)
	}

	c.agentsMux.RLock()
	defer c.agentsMux.RUnlock()

	agent, exists := c.agents[name]
	if !exists {
		return nil, pkgerrors.NewOperationError("GetAgent", name, ErrAgentNotFound)
	}

	return agent, nil
}

// ListAgents returns a list of names of all registered agents.
func (c *Client) ListAgents() []string {
	c.agentsMux.RLock()
	defer c.agentsMux.RUnlock()

	names := make([]string, 0, len(c.agents))
	for name := range c.agents {
		names = append(names, name)
	}
	return names
}

// SendEventToAgent sends an event to a specific registered agent and returns the response.
// This method bridges the type differences between Client (uses `any`) and Agent (uses `events.Event`).
func (c *Client) SendEventToAgent(ctx context.Context, agentName string, event any) ([]any, error) {
	if c.closed {
		return nil, pkgerrors.NewOperationError("SendEventToAgent", "client", ErrClientClosed)
	}

	if agentName == "" {
		return nil, pkgerrors.NewValidationErrorWithField("agentName", "required", "agent name cannot be empty", agentName)
	}

	if event == nil {
		return nil, pkgerrors.NewValidationErrorWithField("event", "required", "event cannot be nil", event)
	}

	c.agentsMux.RLock()
	agent, exists := c.agents[agentName]
	c.agentsMux.RUnlock()

	if !exists {
		return nil, pkgerrors.NewOperationError("SendEventToAgent", agentName, ErrAgentNotFound)
	}

	// Convert the event from `any` to `events.Event`
	eventObj, err := c.convertToEvent(event)
	if err != nil {
		return nil, pkgerrors.WrapWithContext(err, "SendEventToAgent", agentName)
	}

	// Process the event with the agent
	responseEvents, err := agent.ProcessEvent(ctx, eventObj)
	if err != nil {
		return nil, pkgerrors.WrapWithContext(err, "SendEventToAgent", agentName)
	}

	// Convert response events back to `any` type
	responses := make([]any, len(responseEvents))
	for i, responseEvent := range responseEvents {
		responses[i] = responseEvent
	}

	return responses, nil
}

// convertToEvent converts an `any` value to an `events.Event`.
// This method handles the type bridging between the Client API and Agent API.
func (c *Client) convertToEvent(event any) (events.Event, error) {
	// If the event is already an events.Event, return it directly
	if eventObj, ok := event.(events.Event); ok {
		return eventObj, nil
	}

	// If the event is a map or struct with type information, try to create a proper Event
	if eventMap, ok := event.(map[string]interface{}); ok {
		return c.createEventFromMap(eventMap)
	}

	// As a fallback, create a custom event with the raw data
	return c.createCustomEvent(event)
}

// createEventFromMap creates an events.Event from a map with type information.
func (c *Client) createEventFromMap(eventMap map[string]interface{}) (events.Event, error) {
	// Extract the event type from the map
	eventTypeStr, ok := eventMap["type"].(string)
	if !ok {
		return nil, fmt.Errorf("event map missing or invalid 'type' field")
	}

	eventType := events.EventType(eventTypeStr)

	// Create a base event with the specified type
	baseEvent := events.NewBaseEvent(eventType)

	// Set timestamp if provided
	if timestamp, ok := eventMap["timestamp"]; ok {
		if timestampInt, ok := timestamp.(int64); ok {
			baseEvent.SetTimestamp(timestampInt)
		} else if timestampFloat, ok := timestamp.(float64); ok {
			baseEvent.SetTimestamp(int64(timestampFloat))
		}
	}

	// Set raw event data
	if data, ok := eventMap["data"]; ok {
		baseEvent.RawEvent = data
	} else {
		// Use the entire map as raw event data
		baseEvent.RawEvent = eventMap
	}

	return baseEvent, nil
}

// createCustomEvent creates a custom events.Event for arbitrary data.
func (c *Client) createCustomEvent(event any) (events.Event, error) {
	baseEvent := events.NewBaseEvent(events.EventTypeCustom)
	baseEvent.RawEvent = event
	return baseEvent, nil
}
