package transport

import (
	"time"
)

// ConnectionEventOption is a functional option for configuring ConnectionEventData
type ConnectionEventOption func(*ConnectionEventData)

// WithRemoteAddress sets the remote address for a connection event
func WithRemoteAddress(address string) ConnectionEventOption {
	return func(c *ConnectionEventData) {
		c.RemoteAddress = address
	}
}

// WithLocalAddress sets the local address for a connection event
func WithLocalAddress(address string) ConnectionEventOption {
	return func(c *ConnectionEventData) {
		c.LocalAddress = address
	}
}

// WithProtocol sets the protocol for a connection event
func WithProtocol(protocol string) ConnectionEventOption {
	return func(c *ConnectionEventData) {
		c.Protocol = protocol
	}
}

// WithVersion sets the protocol version for a connection event
func WithVersion(version string) ConnectionEventOption {
	return func(c *ConnectionEventData) {
		c.Version = version
	}
}

// WithCapabilities sets the capabilities for a connection event
func WithCapabilities(capabilities ConnectionCapabilities) ConnectionEventOption {
	return func(c *ConnectionEventData) {
		c.Capabilities = capabilities
	}
}

// WithConnectionError sets the error for a connection event
func WithConnectionError(err string) ConnectionEventOption {
	return func(c *ConnectionEventData) {
		c.Error = err
	}
}

// WithAttemptNumber sets the attempt number for a connection event
func WithAttemptNumber(attempt int) ConnectionEventOption {
	return func(c *ConnectionEventData) {
		c.AttemptNumber = attempt
	}
}

// CreateConnectionEvent creates a new connection event with the given options
// It supports both functional options and a builder function as the last parameter
func CreateConnectionEvent(id string, status string, options ...interface{}) TypedTransportEvent[ConnectionEventData] {
	data := ConnectionEventData{
		Status: status,
	}
	
	// Apply options - support both ConnectionEventOption and builder functions
	for _, opt := range options {
		switch v := opt.(type) {
		case ConnectionEventOption:
			v(&data)
		case func(*ConnectionEventData):
			v(&data)
		}
	}
	
	return NewTypedEvent(id, "connection", data)
}

// DataEventOption is a functional option for configuring DataEventData
type DataEventOption func(*DataEventData)

// WithContentType sets the content type for a data event
func WithContentType(contentType string) DataEventOption {
	return func(d *DataEventData) {
		d.ContentType = contentType
	}
}

// WithEncoding sets the encoding for a data event
func WithEncoding(encoding string) DataEventOption {
	return func(d *DataEventData) {
		d.Encoding = encoding
	}
}

// WithChecksum sets the checksum for a data event
func WithChecksum(checksum string) DataEventOption {
	return func(d *DataEventData) {
		d.Checksum = checksum
	}
}

// WithCompressed sets whether the data is compressed
func WithCompressed(compressed bool) DataEventOption {
	return func(d *DataEventData) {
		d.Compressed = compressed
	}
}

// WithStreamID sets the stream ID for a data event
func WithStreamID(streamID string) DataEventOption {
	return func(d *DataEventData) {
		d.StreamID = streamID
	}
}

// WithSequenceNumber sets the sequence number for ordered delivery
func WithSequenceNumber(seq uint64) DataEventOption {
	return func(d *DataEventData) {
		d.SequenceNumber = seq
	}
}

// CreateDataEvent creates a new data event with the given content and options
// It supports both functional options and a builder function as the last parameter
func CreateDataEvent(id string, content []byte, options ...interface{}) TypedTransportEvent[DataEventData] {
	data := DataEventData{
		Content: content,
		Size:    int64(len(content)),
	}
	
	// Apply options - support both DataEventOption and builder functions
	for _, opt := range options {
		switch v := opt.(type) {
		case DataEventOption:
			v(&data)
		case func(*DataEventData):
			v(&data)
		}
	}
	
	return NewTypedEvent(id, "data", data)
}

// ErrorEventOption is a functional option for configuring ErrorEventData
type ErrorEventOption func(*ErrorEventData)

// WithErrorCode sets the error code
func WithErrorCode(code string) ErrorEventOption {
	return func(e *ErrorEventData) {
		e.Code = code
	}
}

// WithErrorSeverity sets the error severity
func WithErrorSeverity(severity string) ErrorEventOption {
	return func(e *ErrorEventData) {
		e.Severity = severity
	}
}

// WithErrorCategory sets the error category
func WithErrorCategory(category string) ErrorEventOption {
	return func(e *ErrorEventData) {
		e.Category = category
	}
}

// WithRetryable sets whether the error is retryable
func WithRetryable(retryable bool) ErrorEventOption {
	return func(e *ErrorEventData) {
		e.Retryable = retryable
	}
}

// WithErrorDetails sets additional error details
func WithErrorDetails(details ErrorDetails) ErrorEventOption {
	return func(e *ErrorEventData) {
		e.Details = details
	}
}

// WithStackTrace sets the stack trace (use sparingly, not in production)
func WithStackTrace(stackTrace string) ErrorEventOption {
	return func(e *ErrorEventData) {
		e.StackTrace = stackTrace
	}
}

// WithRequestID sets the request ID for correlation
func WithRequestID(requestID string) ErrorEventOption {
	return func(e *ErrorEventData) {
		e.RequestID = requestID
	}
}

// CreateErrorEvent creates a new error event with the given message and options
// It supports both functional options and a builder function as the last parameter
func CreateErrorEvent(id string, message string, options ...interface{}) TypedTransportEvent[ErrorEventData] {
	data := ErrorEventData{
		Message: message,
	}
	
	// Apply options - support both ErrorEventOption and builder functions
	for _, opt := range options {
		switch v := opt.(type) {
		case ErrorEventOption:
			v(&data)
		case func(*ErrorEventData):
			v(&data)
		}
	}
	
	return NewTypedEvent(id, "error", data)
}

// StreamEventOption is a functional option for configuring StreamEventData
type StreamEventOption func(*StreamEventData)

// WithDirection sets the stream direction
func WithDirection(direction string) StreamEventOption {
	return func(s *StreamEventData) {
		s.Direction = direction
	}
}

// WithPriority sets the stream priority
func WithPriority(priority int) StreamEventOption {
	return func(s *StreamEventData) {
		s.Priority = priority
	}
}

// WithWindowSize sets the flow control window size
func WithWindowSize(size uint32) StreamEventOption {
	return func(s *StreamEventData) {
		s.WindowSize = size
	}
}

// WithState sets the stream state
func WithState(state string) StreamEventOption {
	return func(s *StreamEventData) {
		s.State = state
	}
}

// WithReason sets the reason for the stream action
func WithReason(reason string) StreamEventOption {
	return func(s *StreamEventData) {
		s.Reason = reason
	}
}

// WithHeaders sets stream-specific headers
func WithHeaders(headers map[string]string) StreamEventOption {
	return func(s *StreamEventData) {
		s.Headers = headers
	}
}

// CreateStreamEvent creates a new stream event with the given stream ID and action
func CreateStreamEvent(id string, streamID string, action string, options ...StreamEventOption) TypedTransportEvent[StreamEventData] {
	data := StreamEventData{
		StreamID: streamID,
		Action:   action,
	}
	
	// Apply all options
	for _, opt := range options {
		opt(&data)
	}
	
	return NewTypedEvent(id, "stream", data)
}

// MetricsEventOption is a functional option for configuring MetricsEventData
type MetricsEventOption func(*MetricsEventData)

// WithUnit sets the unit of measurement
func WithUnit(unit string) MetricsEventOption {
	return func(m *MetricsEventData) {
		m.Unit = unit
	}
}

// WithTags sets metric tags
func WithTags(tags map[string]string) MetricsEventOption {
	return func(m *MetricsEventData) {
		m.Tags = tags
	}
}

// WithLabels sets metric labels (alternative to tags)
func WithLabels(labels map[string]string) MetricsEventOption {
	return func(m *MetricsEventData) {
		m.Labels = labels
	}
}

// WithSampleRate sets the sample rate for sampled metrics
func WithSampleRate(rate float64) MetricsEventOption {
	return func(m *MetricsEventData) {
		m.SampleRate = rate
	}
}

// WithInterval sets the measurement interval
func WithInterval(interval time.Duration) MetricsEventOption {
	return func(m *MetricsEventData) {
		m.Interval = interval
	}
}

// CreateMetricsEvent creates a new metrics event with the given name and value
func CreateMetricsEvent(id string, metricName string, value float64, options ...MetricsEventOption) TypedTransportEvent[MetricsEventData] {
	data := MetricsEventData{
		MetricName: metricName,
		Value:      value,
	}
	
	// Apply all options
	for _, opt := range options {
		opt(&data)
	}
	
	return NewTypedEvent(id, "metrics", data)
}

// Adapter functions for backward compatibility

// AdaptToLegacyEvent converts a TypedTransportEvent to a TransportEvent
// This allows typed events to be used with legacy interfaces
func AdaptToLegacyEvent[T EventData](typedEvent TypedTransportEvent[T]) TransportEvent {
	return NewTransportEventAdapter(typedEvent)
}

// TryGetTypedEvent attempts to extract a typed event from a TransportEvent
// Returns nil if the event is not a typed event or doesn't match the expected type
func TryGetTypedEvent[T EventData](event TransportEvent) TypedTransportEvent[T] {
	// Check if it's a TransportEventAdapter
	if adapter, ok := event.(*TransportEventAdapter); ok {
		if typedEvent, ok := adapter.GetTypedEvent(); ok {
			if te, ok := typedEvent.(TypedTransportEvent[T]); ok {
				return te
			}
		}
	}
	
	// Check if it's already a typed event
	if te, ok := event.(TypedTransportEvent[T]); ok {
		return te
	}
	
	return nil
}

// Alternative syntax for CreateDataEvent that accepts a function to configure the data
// This provides flexibility for complex data initialization
func CreateDataEventWithBuilder(id string, content []byte, builder func(*DataEventData)) TypedTransportEvent[DataEventData] {
	data := DataEventData{
		Content: content,
		Size:    int64(len(content)),
	}
	
	if builder != nil {
		builder(&data)
	}
	
	return NewTypedEvent(id, "data", data)
}

// Alternative syntax for CreateConnectionEvent that accepts a function to configure the data
// This provides flexibility for complex connection initialization
func CreateConnectionEventWithBuilder(id string, status string, builder func(*ConnectionEventData)) TypedTransportEvent[ConnectionEventData] {
	data := ConnectionEventData{
		Status: status,
	}
	
	if builder != nil {
		builder(&data)
	}
	
	return NewTypedEvent(id, "connection", data)
}