package client

import (
	"context"
	"errors"
	"net/url"
	"time"

	pkgerrors "github.com/ag-ui/go-sdk/pkg/errors"
)

// Common client errors
var (
	ErrClientClosed = errors.New("client is closed")
)

// Client represents a connection to an AG-UI server.
type Client struct {
	// baseURL is the base URL of the AG-UI server
	baseURL *url.URL

	// config stores the client configuration
	config Config

	// closed indicates if the client has been closed
	closed bool
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
	}, nil
}

// SendEvent sends an event to the specified agent and returns the response.
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
