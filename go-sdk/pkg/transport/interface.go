package transport

import (
	"context"
	"time"
)

// TransportEvent represents an event that can be sent through a transport.
// This is a simplified interface that doesn't depend on generics.
// 
// Deprecated: Use TypedTransportEvent[T] for type-safe event handling.
// This interface is maintained for backward compatibility.
type TransportEvent interface {
	// ID returns the unique identifier for this event
	ID() string
	
	// Type returns the event type
	Type() string
	
	// Timestamp returns when the event was created
	Timestamp() time.Time
	
	// Data returns the event data as a map
	// Deprecated: Use TypedTransportEvent[T].TypedData() for type-safe access.
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

// Event conversion utilities and helpers for backward compatibility

// AdaptToLegacyEvent converts any TypedTransportEvent to a legacy TransportEvent
// This is useful for gradual migration from typed to legacy events.
func AdaptToLegacyEvent[T EventData](typedEvent TypedTransportEvent[T]) TransportEvent {
	return NewTransportEventAdapter(typedEvent)
}

// TryGetTypedEvent attempts to extract a TypedTransportEvent from a TransportEvent
// Returns the typed event if successful, nil otherwise.
// This is useful when working with mixed event types during migration.
func TryGetTypedEvent[T EventData](event TransportEvent) TypedTransportEvent[T] {
	if adapter, ok := event.(*TransportEventAdapter); ok {
		if typedEvent, ok := adapter.GetTypedEvent(); ok {
			if te, ok := typedEvent.(TypedTransportEvent[T]); ok {
				return te
			}
		}
	}
	return nil
}

// ConvertToTypedEvent attempts to convert a legacy TransportEvent to a TypedTransportEvent
// using the provided constructor function. The constructor should create the typed data
// from the legacy event's Data() map.
func ConvertToTypedEvent[T EventData](
	event TransportEvent, 
	constructor func(map[string]interface{}) (T, error),
) (TypedTransportEvent[T], error) {
	return ToTypedEvent(event, constructor)
}

// CreateConnectionEvent creates a typed connection event
func CreateConnectionEvent(id, status string, opts ...ConnectionEventOption) TypedTransportEvent[ConnectionEventData] {
	data := ConnectionEventData{
		Status: status,
	}
	
	// Apply options
	for _, opt := range opts {
		opt(&data)
	}
	
	return NewTypedEvent(id, "connection", data)
}

// CreateDataEvent creates a typed data event
func CreateDataEvent(id string, content []byte, opts ...DataEventOption) TypedTransportEvent[DataEventData] {
	data := DataEventData{
		Content: content,
		Size:    int64(len(content)),
	}
	
	// Apply options
	for _, opt := range opts {
		opt(&data)
	}
	
	return NewTypedEvent(id, "data", data)
}

// CreateErrorEvent creates a typed error event
func CreateErrorEvent(id, message string, opts ...ErrorEventOption) TypedTransportEvent[ErrorEventData] {
	data := ErrorEventData{
		Message: message,
	}
	
	// Apply options
	for _, opt := range opts {
		opt(&data)
	}
	
	return NewTypedEvent(id, "error", data)
}

// CreateStreamEvent creates a typed stream event
func CreateStreamEvent(id, streamID, action string, opts ...StreamEventOption) TypedTransportEvent[StreamEventData] {
	data := StreamEventData{
		StreamID: streamID,
		Action:   action,
	}
	
	// Apply options
	for _, opt := range opts {
		opt(&data)
	}
	
	return NewTypedEvent(id, "stream", data)
}

// CreateMetricsEvent creates a typed metrics event
func CreateMetricsEvent(id, metricName string, value float64, opts ...MetricsEventOption) TypedTransportEvent[MetricsEventData] {
	data := MetricsEventData{
		MetricName: metricName,
		Value:      value,
	}
	
	// Apply options
	for _, opt := range opts {
		opt(&data)
	}
	
	return NewTypedEvent(id, "metrics", data)
}

// Option types for configuring event data

// ConnectionEventOption configures ConnectionEventData
type ConnectionEventOption func(*ConnectionEventData)

// WithRemoteAddress sets the remote address for connection events
func WithRemoteAddress(address string) ConnectionEventOption {
	return func(data *ConnectionEventData) {
		data.RemoteAddress = address
	}
}

// WithLocalAddress sets the local address for connection events
func WithLocalAddress(address string) ConnectionEventOption {
	return func(data *ConnectionEventData) {
		data.LocalAddress = address
	}
}

// WithProtocol sets the protocol for connection events
func WithProtocol(protocol string) ConnectionEventOption {
	return func(data *ConnectionEventData) {
		data.Protocol = protocol
	}
}

// WithProtocolVersion sets the protocol version for connection events
func WithProtocolVersion(version string) ConnectionEventOption {
	return func(data *ConnectionEventData) {
		data.Version = version
	}
}

// WithConnectionCapabilities sets the capabilities for connection events
func WithConnectionCapabilities(capabilities map[string]interface{}) ConnectionEventOption {
	return func(data *ConnectionEventData) {
		data.Capabilities = capabilities
	}
}

// WithConnectionError sets the error for connection events
func WithConnectionError(err string) ConnectionEventOption {
	return func(data *ConnectionEventData) {
		data.Error = err
	}
}

// WithAttemptNumber sets the attempt number for connection events
func WithAttemptNumber(attempt int) ConnectionEventOption {
	return func(data *ConnectionEventData) {
		data.AttemptNumber = attempt
	}
}

// DataEventOption configures DataEventData
type DataEventOption func(*DataEventData)

// WithContentType sets the content type for data events
func WithContentType(contentType string) DataEventOption {
	return func(data *DataEventData) {
		data.ContentType = contentType
	}
}

// WithEncoding sets the encoding for data events
func WithEncoding(encoding string) DataEventOption {
	return func(data *DataEventData) {
		data.Encoding = encoding
	}
}

// WithChecksum sets the checksum for data events
func WithChecksum(checksum string) DataEventOption {
	return func(data *DataEventData) {
		data.Checksum = checksum
	}
}

// WithCompressed sets the compressed flag for data events
func WithCompressed(compressed bool) DataEventOption {
	return func(data *DataEventData) {
		data.Compressed = compressed
	}
}

// WithStreamID sets the stream ID for data events
func WithStreamID(streamID string) DataEventOption {
	return func(data *DataEventData) {
		data.StreamID = streamID
	}
}

// WithSequenceNumber sets the sequence number for data events
func WithSequenceNumber(seqNum uint64) DataEventOption {
	return func(data *DataEventData) {
		data.SequenceNumber = seqNum
	}
}

// ErrorEventOption configures ErrorEventData
type ErrorEventOption func(*ErrorEventData)

// WithErrorCode sets the error code for error events
func WithErrorCode(code string) ErrorEventOption {
	return func(data *ErrorEventData) {
		data.Code = code
	}
}

// WithErrorSeverity sets the severity for error events
func WithErrorSeverity(severity string) ErrorEventOption {
	return func(data *ErrorEventData) {
		data.Severity = severity
	}
}

// WithErrorCategory sets the category for error events
func WithErrorCategory(category string) ErrorEventOption {
	return func(data *ErrorEventData) {
		data.Category = category
	}
}

// WithRetryable sets the retryable flag for error events
func WithRetryable(retryable bool) ErrorEventOption {
	return func(data *ErrorEventData) {
		data.Retryable = retryable
	}
}

// WithErrorDetails sets the details for error events
func WithErrorDetails(details map[string]interface{}) ErrorEventOption {
	return func(data *ErrorEventData) {
		data.Details = details
	}
}

// WithStackTrace sets the stack trace for error events
func WithStackTrace(stackTrace string) ErrorEventOption {
	return func(data *ErrorEventData) {
		data.StackTrace = stackTrace
	}
}

// WithRequestID sets the request ID for error events
func WithRequestID(requestID string) ErrorEventOption {
	return func(data *ErrorEventData) {
		data.RequestID = requestID
	}
}

// StreamEventOption configures StreamEventData
type StreamEventOption func(*StreamEventData)

// WithDirection sets the direction for stream events
func WithDirection(direction string) StreamEventOption {
	return func(data *StreamEventData) {
		data.Direction = direction
	}
}

// WithPriority sets the priority for stream events
func WithPriority(priority int) StreamEventOption {
	return func(data *StreamEventData) {
		data.Priority = priority
	}
}

// WithWindowSize sets the window size for stream events
func WithWindowSize(windowSize uint32) StreamEventOption {
	return func(data *StreamEventData) {
		data.WindowSize = windowSize
	}
}

// WithStreamState sets the state for stream events
func WithStreamState(state string) StreamEventOption {
	return func(data *StreamEventData) {
		data.State = state
	}
}

// WithReason sets the reason for stream events
func WithReason(reason string) StreamEventOption {
	return func(data *StreamEventData) {
		data.Reason = reason
	}
}

// WithStreamHeaders sets the headers for stream events
func WithStreamHeaders(headers map[string]string) StreamEventOption {
	return func(data *StreamEventData) {
		data.Headers = headers
	}
}

// MetricsEventOption configures MetricsEventData
type MetricsEventOption func(*MetricsEventData)

// WithUnit sets the unit for metrics events
func WithUnit(unit string) MetricsEventOption {
	return func(data *MetricsEventData) {
		data.Unit = unit
	}
}

// WithTags sets the tags for metrics events
func WithTags(tags map[string]string) MetricsEventOption {
	return func(data *MetricsEventData) {
		data.Tags = tags
	}
}

// WithLabels sets the labels for metrics events
func WithLabels(labels map[string]string) MetricsEventOption {
	return func(data *MetricsEventData) {
		data.Labels = labels
	}
}

// WithSampleRate sets the sample rate for metrics events
func WithSampleRate(sampleRate float64) MetricsEventOption {
	return func(data *MetricsEventData) {
		data.SampleRate = sampleRate
	}
}

// WithInterval sets the interval for metrics events
func WithInterval(interval time.Duration) MetricsEventOption {
	return func(data *MetricsEventData) {
		data.Interval = interval
	}
}