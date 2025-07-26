package transport

import (
	"context"
	"io"
	
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// ReadWriter combines io.Reader and io.Writer for raw data transport.
type ReadWriter interface {
	io.Reader
	io.Writer
	io.Closer
}

// EventBus provides event bus capabilities for decoupled communication.
type EventBus interface {
	// Subscribe subscribes to events of a specific type.
	Subscribe(eventType string, handler EventHandler) error

	// Unsubscribe removes a subscription.
	Unsubscribe(eventType string, handler EventHandler) error

	// Publish publishes an event to all subscribers.
	Publish(ctx context.Context, eventType string, event events.Event) error

	// Close closes the event bus.
	Close() error
}