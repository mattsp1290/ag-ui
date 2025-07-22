package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/ag-ui/go-sdk/pkg/core"
	agerrors "github.com/ag-ui/go-sdk/pkg/errors"
)

// Client represents a connection to an AG-UI server.
type Client struct {
	// baseURL is the base URL of the AG-UI server
	baseURL *url.URL

	// TODO: Add transport layer, connection pool, and other client state
}

// Config contains configuration options for the client.
type Config struct {
	// BaseURL is the base URL of the AG-UI server
	BaseURL string

	// TODO: Add authentication, timeout, retry configuration, etc.
}

// New creates a new AG-UI client with the specified configuration.
func New(config Config) (*Client, error) {
	if config.BaseURL == "" {
		return nil, &core.ConfigError{
			Field: "BaseURL",
			Value: config.BaseURL,
			Err:   errors.New(agerrors.MsgBaseURLCannotBeEmpty),
		}
	}

	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, &core.ConfigError{
			Field: "BaseURL",
			Value: config.BaseURL,
			Err:   fmt.Errorf("invalid base URL: %w", err),
		}
	}

	return &Client{
		baseURL: baseURL,
	}, nil
}

// SendEvent sends an event to the specified agent and returns the response.
func (c *Client) SendEvent(ctx context.Context, agentName string, event any) (responses []any, err error) {
	// TODO: Use ctx for request cancellation and timeout when transport layer is implemented
	_ = ctx // Acknowledge ctx parameter is intentionally unused for now

	defer func() {
		if err != nil {
			err = fmt.Errorf("SendEvent failed for agent %s: %w", agentName, err)
		}
	}()

	if agentName == "" {
		return nil, &core.ConfigError{
			Field: "agentName",
			Value: agentName,
			Err:   errors.New(agerrors.MsgAgentNameCannotBeEmpty),
		}
	}

	if event == nil {
		return nil, &core.ConfigError{
			Field: "event",
			Value: event,
			Err:   errors.New(agerrors.MsgEventCannotBeNil),
		}
	}

	// Validate event type if it has a type field
	if err := validateEventType(event); err != nil {
		return nil, err
	}

	// TODO: Implement event sending via transport layer (Issue #123)
	return nil, core.ErrNotImplemented
}

// Stream opens a streaming connection to the specified agent.
func (c *Client) Stream(ctx context.Context, agentName string) (eventChan <-chan any, err error) {
	// TODO: Use ctx for connection cancellation and timeout when streaming is implemented
	_ = ctx // Acknowledge ctx parameter is intentionally unused for now

	defer func() {
		if err != nil {
			err = fmt.Errorf("Stream failed for agent %s: %w", agentName, err)
		}
	}()

	if agentName == "" {
		return nil, &core.ConfigError{
			Field: "agentName",
			Value: agentName,
			Err:   errors.New(agerrors.MsgAgentNameCannotBeEmpty),
		}
	}

	// TODO: Implement streaming connection (Issue #124)
	return nil, core.ErrNotImplemented
}

// Close closes the client and releases any resources.
func (c *Client) Close() error {
	// TODO: Implement resource cleanup
	return nil
}

// validateEventType validates that the event has a valid type if it contains type information.
func validateEventType(event any) error {
	// For now, we accept any event type since this is for basic validation
	// TODO: Add proper event type validation once protobuf events are integrated
	if event == nil {
		return &core.ConfigError{
			Field: "event",
			Value: event,
			Err:   errors.New(agerrors.MsgEventCannotBeNil),
		}
	}
	return nil
}
