package transport

import (
	"time"
)

// EventData is the base interface that all typed event data must implement.
// This provides a common contract for event data validation and serialization.
type EventData interface {
	// Validate ensures the event data is valid
	Validate() error
	
	// ToMap converts the event data to a map[string]interface{} for backward compatibility
	ToMap() map[string]interface{}
}

// TypedTransportEvent is a generic interface for type-safe transport events.
// It provides compile-time type safety for event data while maintaining
// backward compatibility with the existing TransportEvent interface.
type TypedTransportEvent[T EventData] interface {
	// ID returns the unique identifier for this event
	ID() string
	
	// Type returns the event type
	Type() string
	
	// Timestamp returns when the event was created
	Timestamp() time.Time
	
	// TypedData returns the strongly-typed event data
	TypedData() T
	
	// Data returns the event data as a map for backward compatibility
	// Deprecated: Use TypedData() for type-safe access
	Data() map[string]interface{}
}

// ConnectionEventData represents connection-related event data
type ConnectionEventData struct {
	// Status indicates the connection status (connected, disconnected, reconnecting, etc.)
	Status string `json:"status"`
	
	// RemoteAddress is the address of the remote endpoint
	RemoteAddress string `json:"remote_address,omitempty"`
	
	// LocalAddress is the local address used for the connection
	LocalAddress string `json:"local_address,omitempty"`
	
	// Protocol is the protocol used (http, websocket, grpc, etc.)
	Protocol string `json:"protocol,omitempty"`
	
	// Version is the protocol version
	Version string `json:"version,omitempty"`
	
	// Capabilities are the negotiated capabilities
	Capabilities map[string]interface{} `json:"capabilities,omitempty"`
	
	// Error contains error information if the connection failed
	Error string `json:"error,omitempty"`
	
	// AttemptNumber for reconnection events
	AttemptNumber int `json:"attempt_number,omitempty"`
}

// Validate ensures the connection event data is valid
func (c ConnectionEventData) Validate() error {
	if c.Status == "" {
		return NewValidationError("status is required", nil)
	}
	return nil
}

// ToMap converts the connection event data to a map for backward compatibility
func (c ConnectionEventData) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["status"] = c.Status
	
	if c.RemoteAddress != "" {
		result["remote_address"] = c.RemoteAddress
	}
	if c.LocalAddress != "" {
		result["local_address"] = c.LocalAddress
	}
	if c.Protocol != "" {
		result["protocol"] = c.Protocol
	}
	if c.Version != "" {
		result["version"] = c.Version
	}
	if c.Capabilities != nil {
		result["capabilities"] = c.Capabilities
	}
	if c.Error != "" {
		result["error"] = c.Error
	}
	if c.AttemptNumber > 0 {
		result["attempt_number"] = c.AttemptNumber
	}
	
	return result
}

// DataEventData represents message/data-related event data
type DataEventData struct {
	// Content is the actual data payload
	Content []byte `json:"content"`
	
	// ContentType indicates the MIME type or format of the content
	ContentType string `json:"content_type,omitempty"`
	
	// Encoding indicates the encoding used (utf-8, base64, etc.)
	Encoding string `json:"encoding,omitempty"`
	
	// Size is the size of the content in bytes
	Size int64 `json:"size"`
	
	// Checksum for data integrity verification
	Checksum string `json:"checksum,omitempty"`
	
	// Compressed indicates if the content is compressed
	Compressed bool `json:"compressed,omitempty"`
	
	// StreamID if this data belongs to a specific stream
	StreamID string `json:"stream_id,omitempty"`
	
	// SequenceNumber for ordered delivery
	SequenceNumber uint64 `json:"sequence_number,omitempty"`
}

// Validate ensures the data event data is valid
func (d DataEventData) Validate() error {
	if d.Content == nil {
		return NewValidationError("content is required", nil)
	}
	if d.Size < 0 {
		return NewValidationError("size cannot be negative", nil)
	}
	if d.Size != int64(len(d.Content)) {
		return NewValidationError("size does not match content length", nil)
	}
	return nil
}

// ToMap converts the data event data to a map for backward compatibility
func (d DataEventData) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["content"] = d.Content
	result["size"] = d.Size
	
	if d.ContentType != "" {
		result["content_type"] = d.ContentType
	}
	if d.Encoding != "" {
		result["encoding"] = d.Encoding
	}
	if d.Checksum != "" {
		result["checksum"] = d.Checksum
	}
	result["compressed"] = d.Compressed
	if d.StreamID != "" {
		result["stream_id"] = d.StreamID
	}
	if d.SequenceNumber > 0 {
		result["sequence_number"] = d.SequenceNumber
	}
	
	return result
}

// ErrorEventData represents error-related event data
type ErrorEventData struct {
	// Message is the error message
	Message string `json:"message"`
	
	// Code is an error code for programmatic handling
	Code string `json:"code,omitempty"`
	
	// Severity indicates the error severity (fatal, error, warning, info)
	Severity string `json:"severity,omitempty"`
	
	// Category categorizes the error (network, protocol, validation, etc.)
	Category string `json:"category,omitempty"`
	
	// Retryable indicates if the operation can be retried
	Retryable bool `json:"retryable"`
	
	// Details contains additional error context
	Details map[string]interface{} `json:"details,omitempty"`
	
	// StackTrace for debugging (should be omitted in production)
	StackTrace string `json:"stack_trace,omitempty"`
	
	// RequestID for request correlation
	RequestID string `json:"request_id,omitempty"`
}

// Validate ensures the error event data is valid
func (e ErrorEventData) Validate() error {
	if e.Message == "" {
		return NewValidationError("message is required", nil)
	}
	return nil
}

// ToMap converts the error event data to a map for backward compatibility
func (e ErrorEventData) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["message"] = e.Message
	result["retryable"] = e.Retryable
	
	if e.Code != "" {
		result["code"] = e.Code
	}
	if e.Severity != "" {
		result["severity"] = e.Severity
	}
	if e.Category != "" {
		result["category"] = e.Category
	}
	if e.Details != nil {
		result["details"] = e.Details
	}
	if e.StackTrace != "" {
		result["stack_trace"] = e.StackTrace
	}
	if e.RequestID != "" {
		result["request_id"] = e.RequestID
	}
	
	return result
}

// StreamEventData represents stream-related event data
type StreamEventData struct {
	// StreamID is the unique identifier for the stream
	StreamID string `json:"stream_id"`
	
	// Action indicates the stream action (create, close, reset, etc.)
	Action string `json:"action"`
	
	// Direction indicates data flow direction (inbound, outbound, bidirectional)
	Direction string `json:"direction,omitempty"`
	
	// Priority for stream prioritization
	Priority int `json:"priority,omitempty"`
	
	// WindowSize for flow control
	WindowSize uint32 `json:"window_size,omitempty"`
	
	// State indicates the current stream state
	State string `json:"state,omitempty"`
	
	// Reason provides additional context for the action
	Reason string `json:"reason,omitempty"`
	
	// Headers contains stream-specific headers
	Headers map[string]string `json:"headers,omitempty"`
}

// Validate ensures the stream event data is valid
func (s StreamEventData) Validate() error {
	if s.StreamID == "" {
		return NewValidationError("stream_id is required", nil)
	}
	if s.Action == "" {
		return NewValidationError("action is required", nil)
	}
	return nil
}

// ToMap converts the stream event data to a map for backward compatibility
func (s StreamEventData) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["stream_id"] = s.StreamID
	result["action"] = s.Action
	
	if s.Direction != "" {
		result["direction"] = s.Direction
	}
	if s.Priority != 0 {
		result["priority"] = s.Priority
	}
	if s.WindowSize > 0 {
		result["window_size"] = s.WindowSize
	}
	if s.State != "" {
		result["state"] = s.State
	}
	if s.Reason != "" {
		result["reason"] = s.Reason
	}
	if s.Headers != nil {
		result["headers"] = s.Headers
	}
	
	return result
}

// MetricsEventData represents metrics-related event data
type MetricsEventData struct {
	// MetricName is the name of the metric
	MetricName string `json:"metric_name"`
	
	// Value is the metric value
	Value float64 `json:"value"`
	
	// Unit indicates the unit of measurement
	Unit string `json:"unit,omitempty"`
	
	// Tags for metric categorization and filtering
	Tags map[string]string `json:"tags,omitempty"`
	
	// Labels for additional metric metadata (same as tags, different naming convention)
	Labels map[string]string `json:"labels,omitempty"`
	
	// SampleRate for sampled metrics
	SampleRate float64 `json:"sample_rate,omitempty"`
	
	// Interval indicates the measurement interval
	Interval time.Duration `json:"interval,omitempty"`
}

// Validate ensures the metrics event data is valid
func (m MetricsEventData) Validate() error {
	if m.MetricName == "" {
		return NewValidationError("metric_name is required", nil)
	}
	if m.SampleRate < 0 || m.SampleRate > 1 {
		return NewValidationError("sample_rate must be between 0 and 1", nil)
	}
	return nil
}

// ToMap converts the metrics event data to a map for backward compatibility
func (m MetricsEventData) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["metric_name"] = m.MetricName
	result["value"] = m.Value
	
	if m.Unit != "" {
		result["unit"] = m.Unit
	}
	if m.Tags != nil {
		result["tags"] = m.Tags
	}
	if m.Labels != nil {
		result["labels"] = m.Labels
	}
	if m.SampleRate > 0 {
		result["sample_rate"] = m.SampleRate
	}
	if m.Interval > 0 {
		result["interval"] = m.Interval.String()
	}
	
	return result
}

// typedEventImpl is a concrete implementation of TypedTransportEvent
type typedEventImpl[T EventData] struct {
	id        string
	eventType string
	timestamp time.Time
	data      T
}

// NewTypedEvent creates a new typed transport event
func NewTypedEvent[T EventData](id, eventType string, data T) TypedTransportEvent[T] {
	return &typedEventImpl[T]{
		id:        id,
		eventType: eventType,
		timestamp: time.Now(),
		data:      data,
	}
}

// ID returns the unique identifier for this event
func (e *typedEventImpl[T]) ID() string {
	return e.id
}

// Type returns the event type
func (e *typedEventImpl[T]) Type() string {
	return e.eventType
}

// Timestamp returns when the event was created
func (e *typedEventImpl[T]) Timestamp() time.Time {
	return e.timestamp
}

// TypedData returns the strongly-typed event data
func (e *typedEventImpl[T]) TypedData() T {
	return e.data
}

// Data returns the event data as a map for backward compatibility
func (e *typedEventImpl[T]) Data() map[string]interface{} {
	return e.data.ToMap()
}

// TransportEventAdapter wraps a TypedTransportEvent to implement the legacy TransportEvent interface
type TransportEventAdapter struct {
	typedEvent interface{} // We use interface{} here to avoid generic constraints in the struct
	id         string
	eventType  string
	timestamp  time.Time
	dataMap    map[string]interface{}
}

// NewTransportEventAdapter creates an adapter from any TypedTransportEvent to TransportEvent
func NewTransportEventAdapter[T EventData](typedEvent TypedTransportEvent[T]) TransportEvent {
	return &TransportEventAdapter{
		typedEvent: typedEvent,
		id:         typedEvent.ID(),
		eventType:  typedEvent.Type(),
		timestamp:  typedEvent.Timestamp(),
		dataMap:    typedEvent.Data(),
	}
}

// ID returns the unique identifier for this event
func (a *TransportEventAdapter) ID() string {
	return a.id
}

// Type returns the event type
func (a *TransportEventAdapter) Type() string {
	return a.eventType
}

// Timestamp returns when the event was created
func (a *TransportEventAdapter) Timestamp() time.Time {
	return a.timestamp
}

// Data returns the event data as a map
func (a *TransportEventAdapter) Data() map[string]interface{} {
	return a.dataMap
}

// GetTypedEvent attempts to retrieve the original typed event
// Returns the typed event and true if successful, nil and false otherwise
func (a *TransportEventAdapter) GetTypedEvent() (interface{}, bool) {
	return a.typedEvent, a.typedEvent != nil
}

// legacyEventImpl is a simple implementation of TransportEvent for backward compatibility
type legacyEventImpl struct {
	id        string
	eventType string
	timestamp time.Time
	data      map[string]interface{}
}

// NewLegacyEvent creates a new legacy transport event
func NewLegacyEvent(id, eventType string, data map[string]interface{}) TransportEvent {
	return &legacyEventImpl{
		id:        id,
		eventType: eventType,
		timestamp: time.Now(),
		data:      data,
	}
}

// ID returns the unique identifier for this event
func (e *legacyEventImpl) ID() string {
	return e.id
}

// Type returns the event type
func (e *legacyEventImpl) Type() string {
	return e.eventType
}

// Timestamp returns when the event was created
func (e *legacyEventImpl) Timestamp() time.Time {
	return e.timestamp
}

// Data returns the event data as a map
func (e *legacyEventImpl) Data() map[string]interface{} {
	// Return a copy to prevent external modification
	result := make(map[string]interface{})
	for k, v := range e.data {
		result[k] = v
	}
	return result
}

// ToTypedEvent attempts to convert a legacy TransportEvent to a TypedTransportEvent
// This is a convenience function for migration scenarios
func ToTypedEvent[T EventData](event TransportEvent, constructor func(map[string]interface{}) (T, error)) (TypedTransportEvent[T], error) {
	data, err := constructor(event.Data())
	if err != nil {
		return nil, err
	}
	
	return &typedEventImpl[T]{
		id:        event.ID(),
		eventType: event.Type(),
		timestamp: event.Timestamp(),
		data:      data,
	}, nil
}