package transport

import (
	"context"
	
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// Middleware represents transport middleware for intercepting and modifying events.
type Middleware interface {
	// ProcessOutgoing processes outgoing events before they are sent.
	ProcessOutgoing(ctx context.Context, event TransportEvent) (TransportEvent, error)

	// ProcessIncoming processes incoming events before they are delivered.
	ProcessIncoming(ctx context.Context, event events.Event) (events.Event, error)

	// Name returns the middleware name for logging and debugging.
	Name() string
	
	// Wrap wraps a transport with this middleware.
	Wrap(transport Transport) Transport
}

// MiddlewareChain represents a chain of middleware processors.
type MiddlewareChain interface {
	// Add adds middleware to the chain.
	Add(middleware Middleware)

	// ProcessOutgoing processes an outgoing event through the middleware chain.
	ProcessOutgoing(ctx context.Context, event TransportEvent) (TransportEvent, error)

	// ProcessIncoming processes an incoming event through the middleware chain.
	ProcessIncoming(ctx context.Context, event events.Event) (events.Event, error)

	// Clear removes all middleware from the chain.
	Clear()
}

// EventFilter represents a filter for events based on type or other criteria.
type EventFilter interface {
	// ShouldProcess returns true if the event should be processed.
	ShouldProcess(event events.Event) bool

	// Priority returns the filter priority (higher values are processed first).
	Priority() int

	// Name returns the filter name for logging and debugging.
	Name() string
}