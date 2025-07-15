package transport

import (
	"context"
	"io"
	"time"
	
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TransportEvent represents a transport event with metadata.
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


// Transport represents the core transport interface for bidirectional communication
// with agents and front-end applications in the AG-UI system.
type Transport interface {
	// Connect establishes a connection to the remote endpoint.
	// Returns an error if the connection cannot be established.
	Connect(ctx context.Context) error

	// Send sends an event to the remote endpoint.
	// The event parameter can be any type that implements the Event interface.
	Send(ctx context.Context, event TransportEvent) error

	// Receive returns a channel that receives events from the remote endpoint.
	// The channel is closed when the transport is closed or an error occurs.
	Receive() <-chan events.Event

	// Errors returns a channel that receives errors from the transport.
	// The channel is closed when the transport is closed.
	Errors() <-chan error

	// Close closes the transport and releases any associated resources.
	// This method should be idempotent and safe to call multiple times.
	Close(ctx context.Context) error

	// IsConnected returns true if the transport is currently connected.
	IsConnected() bool

	// Config returns the transport's configuration.
	Config() Config

	// Stats returns transport statistics and metrics.
	Stats() TransportStats
}

// StreamingTransport extends Transport with streaming-specific capabilities
// for real-time bidirectional communication.
type StreamingTransport interface {
	Transport

	// StartStreaming begins streaming events in both directions.
	// Returns channels for sending and receiving events, along with an error channel.
	StartStreaming(ctx context.Context) (send chan<- TransportEvent, receive <-chan events.Event, errors <-chan error, err error)

	// SendBatch sends multiple events in a single batch operation.
	// This can be more efficient than sending events individually.
	SendBatch(ctx context.Context, events []TransportEvent) error

	// SetEventHandler sets a callback function to handle received events.
	// This provides an alternative to using the receive channel from StartStreaming.
	SetEventHandler(handler EventHandler)

	// GetStreamingStats returns streaming-specific statistics.
	GetStreamingStats() StreamingStats
}

// ReliableTransport extends Transport with reliability features like
// acknowledgments, retries, and ordered delivery.
type ReliableTransport interface {
	Transport

	// SendEventWithAck sends an event and waits for acknowledgment.
	// Returns an error if the event is not acknowledged within the timeout.
	SendEventWithAck(ctx context.Context, event TransportEvent, timeout time.Duration) error

	// SetAckHandler sets a callback for handling acknowledgments.
	SetAckHandler(handler AckHandler)

	// GetReliabilityStats returns reliability-specific statistics.
	GetReliabilityStats() ReliabilityStats
}

// EventHandler is a callback function for handling received events.
type EventHandler func(ctx context.Context, event events.Event) error

// AckHandler is a callback function for handling acknowledgments.
type AckHandler func(ctx context.Context, eventID string, success bool) error

// TransportStats contains general transport statistics.
type TransportStats struct {
	// Connection statistics
	ConnectedAt    time.Time     `json:"connected_at"`
	ReconnectCount int           `json:"reconnect_count"`
	LastError      error         `json:"last_error,omitempty"`
	Uptime         time.Duration `json:"uptime"`

	// Event statistics
	EventsSent       int64         `json:"events_sent"`
	EventsReceived   int64         `json:"events_received"`
	BytesSent        int64         `json:"bytes_sent"`
	BytesReceived    int64         `json:"bytes_received"`
	AverageLatency   time.Duration `json:"average_latency"`
	ErrorCount       int64         `json:"error_count"`
	LastEventSentAt  time.Time     `json:"last_event_sent_at"`
	LastEventRecvAt  time.Time     `json:"last_event_recv_at"`
}

// StreamingStats contains streaming-specific statistics.
type StreamingStats struct {
	TransportStats

	// Streaming-specific metrics
	StreamsActive        int           `json:"streams_active"`
	StreamsTotal         int           `json:"streams_total"`
	BufferUtilization    float64       `json:"buffer_utilization"`
	BackpressureEvents   int64         `json:"backpressure_events"`
	DroppedEvents        int64         `json:"dropped_events"`
	AverageEventSize     int64         `json:"average_event_size"`
	ThroughputEventsPerSec float64     `json:"throughput_events_per_sec"`
	ThroughputBytesPerSec  float64     `json:"throughput_bytes_per_sec"`
}

// ReliabilityStats contains reliability-specific statistics.
type ReliabilityStats struct {
	TransportStats

	// Reliability-specific metrics
	EventsAcknowledged     int64         `json:"events_acknowledged"`
	EventsUnacknowledged   int64         `json:"events_unacknowledged"`
	EventsRetried          int64         `json:"events_retried"`
	EventsTimedOut         int64         `json:"events_timed_out"`
	AverageAckTime         time.Duration `json:"average_ack_time"`
	DuplicateEvents        int64         `json:"duplicate_events"`
	OutOfOrderEvents       int64         `json:"out_of_order_events"`
	RedeliveryRate         float64       `json:"redelivery_rate"`
}

// Config represents the interface for transport configuration.
type Config interface {
	// Validate validates the configuration.
	Validate() error

	// Clone creates a deep copy of the configuration.
	Clone() Config

	// GetType returns the transport type (e.g., "websocket", "http", "grpc").
	GetType() string

	// GetEndpoint returns the endpoint URL or address.
	GetEndpoint() string

	// GetTimeout returns the connection timeout.
	GetTimeout() time.Duration

	// GetHeaders returns custom headers for the transport.
	GetHeaders() map[string]string

	// IsSecure returns true if the transport uses secure connections.
	IsSecure() bool
}

// ConnectionState represents the current state of a transport connection.
type ConnectionState int

const (
	// StateDisconnected indicates the transport is not connected.
	StateDisconnected ConnectionState = iota
	// StateConnecting indicates the transport is attempting to connect.
	StateConnecting
	// StateConnected indicates the transport is connected and ready.
	StateConnected
	// StateReconnecting indicates the transport is attempting to reconnect.
	StateReconnecting
	// StateClosing indicates the transport is closing the connection.
	StateClosing
	// StateClosed indicates the transport is closed.
	StateClosed
	// StateError indicates the transport is in an error state.
	StateError
)

// String returns the string representation of the connection state.
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosing:
		return "closing"
	case StateClosed:
		return "closed"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// ConnectionCallback is called when the connection state changes.
type ConnectionCallback func(state ConnectionState, err error)

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

// TransportManager manages multiple transport instances and provides
// load balancing, failover, and connection pooling capabilities.
type TransportManager interface {
	// AddTransport adds a transport to the manager.
	AddTransport(name string, transport Transport) error

	// RemoveTransport removes a transport from the manager.
	RemoveTransport(name string) error

	// GetTransport retrieves a transport by name.
	GetTransport(name string) (Transport, error)

	// GetActiveTransports returns all active transports.
	GetActiveTransports() map[string]Transport

	// SendEvent sends an event using the best available transport.
	SendEvent(ctx context.Context, event TransportEvent) error

	// SendEventToTransport sends an event to a specific transport.
	SendEventToTransport(ctx context.Context, transportName string, event TransportEvent) error

	// ReceiveEvents returns a channel that receives events from all transports.
	ReceiveEvents(ctx context.Context) (<-chan events.Event, error)

	// SetLoadBalancer sets the load balancing strategy.
	SetLoadBalancer(balancer LoadBalancer)

	// Close closes all managed transports.
	Close() error

	// GetStats returns aggregated statistics from all transports.
	GetStats() map[string]TransportStats
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

// Serializer handles serialization and deserialization of events for transport.
type Serializer interface {
	// Serialize converts an event to bytes for transport.
	Serialize(event TransportEvent) ([]byte, error)

	// Deserialize converts bytes back to an event.
	Deserialize(data []byte) (events.Event, error)

	// ContentType returns the content type for the serialized data.
	ContentType() string

	// SupportedTypes returns the types that this serializer can handle.
	SupportedTypes() []string
}

// Compressor handles compression and decompression of serialized data.
type Compressor interface {
	// Compress compresses the input data.
	Compress(data []byte) ([]byte, error)

	// Decompress decompresses the input data.
	Decompress(data []byte) ([]byte, error)

	// Algorithm returns the compression algorithm name.
	Algorithm() string

	// CompressionRatio returns the achieved compression ratio.
	CompressionRatio() float64
}

// HealthChecker provides health check capabilities for transports.
type HealthChecker interface {
	// CheckHealth performs a health check on the transport.
	CheckHealth(ctx context.Context) error

	// IsHealthy returns true if the transport is healthy.
	IsHealthy() bool

	// GetHealthStatus returns detailed health status information.
	GetHealthStatus() HealthStatus
}

// HealthStatus represents the health status of a transport.
type HealthStatus struct {
	Healthy    bool          `json:"healthy"`
	Timestamp  time.Time     `json:"timestamp"`
	Latency    time.Duration `json:"latency"`
	Error      string        `json:"error,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// MetricsCollector collects and reports transport metrics.
type MetricsCollector interface {
	// RecordEvent records an event metric.
	RecordEvent(eventType string, size int64, latency time.Duration)

	// RecordError records an error metric.
	RecordError(errorType string, err error)

	// RecordConnection records a connection metric.
	RecordConnection(connected bool, duration time.Duration)

	// GetMetrics returns collected metrics.
	GetMetrics() map[string]any

	// Reset resets all collected metrics.
	Reset()
}

// AuthProvider handles authentication for transport connections.
type AuthProvider interface {
	// GetCredentials returns authentication credentials.
	GetCredentials(ctx context.Context) (map[string]string, error)

	// RefreshCredentials refreshes authentication credentials.
	RefreshCredentials(ctx context.Context) error

	// IsValid returns true if the credentials are valid.
	IsValid() bool

	// ExpiresAt returns when the credentials expire.
	ExpiresAt() time.Time
}

// RetryPolicy defines retry behavior for failed operations.
type RetryPolicy interface {
	// ShouldRetry returns true if the operation should be retried.
	ShouldRetry(attempt int, err error) bool

	// NextDelay returns the delay before the next retry attempt.
	NextDelay(attempt int) time.Duration

	// MaxAttempts returns the maximum number of retry attempts.
	MaxAttempts() int

	// Reset resets the retry policy state.
	Reset()
}

// CircuitBreaker provides circuit breaker functionality for transport operations.
type CircuitBreaker interface {
	// Execute executes an operation with circuit breaker protection.
	Execute(ctx context.Context, operation func() error) error

	// IsOpen returns true if the circuit breaker is open.
	IsOpen() bool

	// Reset resets the circuit breaker state.
	Reset()

	// GetState returns the current circuit breaker state.
	GetState() CircuitBreakerState
}

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	// CircuitClosed indicates the circuit breaker is closed (normal operation).
	CircuitClosed CircuitBreakerState = iota
	// CircuitOpen indicates the circuit breaker is open (rejecting requests).
	CircuitOpen
	// CircuitHalfOpen indicates the circuit breaker is half-open (testing).
	CircuitHalfOpen
)

// String returns the string representation of the circuit breaker state.
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

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

// TransportEventType represents different types of transport events.
type TransportEventType string

const (
	// EventTypeConnected indicates a successful connection.
	EventTypeConnected TransportEventType = "connected"
	// EventTypeDisconnected indicates a disconnection.
	EventTypeDisconnected TransportEventType = "disconnected"
	// EventTypeReconnecting indicates a reconnection attempt.
	EventTypeReconnecting TransportEventType = "reconnecting"
	// EventTypeError indicates an error occurred.
	EventTypeError TransportEventType = "error"
	// EventTypeEventSent indicates an event was sent.
	EventTypeEventSent TransportEventType = "event_sent"
	// EventTypeEventReceived indicates an event was received.
	EventTypeEventReceived TransportEventType = "event_received"
	// EventTypeStatsUpdated indicates transport statistics were updated.
	EventTypeStatsUpdated TransportEventType = "stats_updated"
)

// TransportEventImpl represents a transport-related event.
type TransportEventImpl struct {
	Type      TransportEventType `json:"type"`
	Timestamp time.Time          `json:"timestamp"`
	Transport string             `json:"transport"`
	Data      any                `json:"data,omitempty"`
	Error     error              `json:"error,omitempty"`
}

// NewTransportEvent creates a new transport event.
func NewTransportEvent(eventType TransportEventType, transport string, data any) *TransportEventImpl {
	return &TransportEventImpl{
		Type:      eventType,
		Timestamp: time.Now(),
		Transport: transport,
		Data:      data,
	}
}

// NewTransportErrorEvent creates a new transport error event.
func NewTransportErrorEvent(transport string, err error) *TransportEventImpl {
	return &TransportEventImpl{
		Type:      EventTypeError,
		Timestamp: time.Now(),
		Transport: transport,
		Error:     err,
	}
}