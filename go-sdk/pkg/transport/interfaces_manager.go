package transport

import (
	"context"
	
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// TransportManager manages transport instances
type TransportManager interface {
	// AddTransport adds a transport to the registry.
	AddTransport(name string, transport Transport) error

	// RemoveTransport removes a transport from the registry.
	RemoveTransport(name string) error

	// GetTransport retrieves a transport by name.
	GetTransport(name string) (Transport, error)

	// GetActiveTransports returns all active transports.
	GetActiveTransports() map[string]Transport
}

// EventRouter routes events to appropriate transports
type EventRouter interface {
	// SendEvent sends an event using the best available transport.
	SendEvent(ctx context.Context, event TransportEvent) error

	// SendEventToTransport sends an event to a specific transport.
	SendEventToTransport(ctx context.Context, transportName string, event TransportEvent) error
}

// EventAggregator aggregates events from multiple sources
type EventAggregator interface {
	// ReceiveEvents returns a channel that receives events from all transports.
	ReceiveEvents(ctx context.Context) (<-chan events.Event, error)
}

// LoadBalancerSetter allows setting load balancing strategy
type LoadBalancerSetter interface {
	// SetLoadBalancer sets the load balancing strategy.
	SetLoadBalancer(balancer LoadBalancer)
}

// ManagerStatsProvider provides aggregated statistics
type ManagerStatsProvider interface {
	// GetStats returns aggregated statistics from all transports.
	GetStats() map[string]TransportStats
}

// TransportMultiManager manages multiple transport instances
type TransportMultiManager interface {
	TransportManager
	EventRouter
	EventAggregator
	LoadBalancerSetter
	ManagerStatsProvider
	
	// Close closes all managed transports.
	Close() error
}

// LoadBalancer represents a load balancing strategy for multiple transports.
type LoadBalancer interface {
	// SelectTransport selects a transport for sending an event.
	SelectTransport(transports map[string]Transport, event TransportEvent) (string, error)

	// UpdateStats updates the load balancer with transport statistics.
	UpdateStats(transportName string, stats TransportStats)

	// Name returns the load balancer name.
	Name() string
}