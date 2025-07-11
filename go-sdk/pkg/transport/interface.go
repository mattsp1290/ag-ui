package transport

import (
	"context"
	"time"
)

// TransportEvent represents an event that can be sent through a transport.
// This is a simplified interface that doesn't depend on generics.
type TransportEvent interface {
	// ID returns the unique identifier for this event
	ID() string
	
	// Type returns the event type
	Type() string
	
	// Timestamp returns when the event was created
	Timestamp() time.Time
	
	// Data returns the event data as a map
	Data() map[string]interface{}
}

// Transport defines the interface for all transport implementations.
// It provides a unified abstraction for different transport mechanisms
// such as HTTP/SSE, WebSocket, HTTP, and gRPC.
type Transport interface {
	// Connect establishes a connection to the remote endpoint.
	// It should handle initial handshake and capability negotiation.
	Connect(ctx context.Context) error

	// Close gracefully shuts down the transport connection.
	// It should clean up all resources and notify any active listeners.
	// The context allows for timeout control during graceful shutdown.
	Close(ctx context.Context) error

	// Send transmits an event through the transport.
	// It should handle serialization and any transport-specific encoding.
	Send(ctx context.Context, event TransportEvent) error

	// Receive returns a channel for receiving events from the transport.
	// The channel should be closed when the transport is closed or encounters an error.
	Receive() <-chan Event

	// Errors returns a channel for receiving transport-specific errors.
	// This allows for asynchronous error handling.
	Errors() <-chan error

	// IsConnected returns whether the transport is currently connected.
	IsConnected() bool

	// Capabilities returns the capabilities supported by this transport.
	Capabilities() Capabilities

	// Health performs a health check on the transport connection.
	// It returns nil if the transport is healthy, or an error describing the issue.
	Health(ctx context.Context) error

	// Metrics returns performance metrics for the transport.
	Metrics() Metrics

	// SetMiddleware configures the middleware chain for this transport.
	SetMiddleware(middleware ...Middleware)
}

// Event represents an event received from the transport.
// It wraps the transport event with transport-specific metadata.
type Event struct {
	Event     TransportEvent
	Metadata  EventMetadata
	Timestamp time.Time
}

// EventMetadata contains transport-specific metadata for an event.
type EventMetadata struct {
	// TransportID identifies which transport this event came from
	TransportID string

	// Headers contains any transport-specific headers
	Headers map[string]string

	// Size is the size of the event in bytes as transmitted
	Size int64

	// Latency is the time taken to receive the event
	Latency time.Duration

	// Compressed indicates if the event was compressed during transport
	Compressed bool
}

// StreamTransport extends Transport with streaming-specific operations.
type StreamTransport interface {
	Transport

	// CreateStream creates a new stream within the transport.
	CreateStream(ctx context.Context, streamID string) (Stream, error)

	// AcceptStream accepts incoming streams from the remote endpoint.
	AcceptStream(ctx context.Context) (Stream, error)
}

// Stream represents a single stream within a transport.
type Stream interface {
	// ID returns the unique identifier for this stream
	ID() string

	// Read reads data from the stream
	Read(p []byte) (n int, err error)

	// Write writes data to the stream
	Write(p []byte) (n int, err error)

	// Close closes the stream gracefully
	// The context allows for timeout control during graceful shutdown.
	Close(ctx context.Context) error

	// SendEvent sends an event on this stream
	SendEvent(ctx context.Context, event TransportEvent) error

	// ReceiveEvent receives an event from this stream
	ReceiveEvent(ctx context.Context) (TransportEvent, error)

	// Reset forcefully resets the stream
	Reset() error
}

// ReconnectableTransport extends Transport with reconnection capabilities.
type ReconnectableTransport interface {
	Transport

	// Reconnect attempts to reconnect the transport.
	Reconnect(ctx context.Context) error

	// SetReconnectStrategy configures the reconnection strategy.
	SetReconnectStrategy(strategy ReconnectStrategy)

	// OnReconnect registers a callback for reconnection events.
	OnReconnect(callback func(attemptNumber int, err error))
}


// Middleware defines the interface for transport middleware.
type Middleware interface {
	// Wrap wraps a transport with middleware functionality
	Wrap(transport Transport) Transport
}

// MiddlewareFunc is a function type that implements Middleware
type MiddlewareFunc func(Transport) Transport

// Wrap implements the Middleware interface for MiddlewareFunc
func (f MiddlewareFunc) Wrap(t Transport) Transport {
	return f(t)
}