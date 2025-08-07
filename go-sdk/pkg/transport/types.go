package transport

import (
	"time"
)

// CompressionType constants
const (
	CompressionGzip = "gzip"
	CompressionNone = "none"
)

// SecurityFeature constants
const (
	SecurityTLS  = "tls"
	SecurityNone = "none"
)

// Capabilities describes basic transport characteristics.
// Simplified from the previous complex capability system.
type Capabilities struct {
	// Streaming indicates if the transport supports streaming
	Streaming bool

	// Bidirectional indicates if the transport supports bidirectional communication
	Bidirectional bool

	// MaxMessageSize is the maximum message size supported (0 for unlimited)
	MaxMessageSize int64

	// ProtocolVersion is the version of the transport protocol
	ProtocolVersion string

	// Features is a map of feature names to their values
	Features map[string]interface{}
}

// Metrics contains performance metrics for a transport.
type Metrics struct {
	// ConnectionUptime is how long the connection has been established
	ConnectionUptime time.Duration

	// MessagesSent is the total number of messages sent
	MessagesSent uint64

	// MessagesReceived is the total number of messages received
	MessagesReceived uint64

	// BytesSent is the total number of bytes sent
	BytesSent uint64

	// BytesReceived is the total number of bytes received
	BytesReceived uint64

	// ErrorCount is the total number of errors encountered
	ErrorCount uint64

	// AverageLatency is the average message latency
	AverageLatency time.Duration

	// CurrentThroughput is the current throughput in messages per second
	CurrentThroughput float64

	// ReconnectCount is the number of reconnection attempts
	ReconnectCount uint64

	// LastError contains the last error encountered
	LastError error

	// LastErrorTime is when the last error occurred
	LastErrorTime time.Time
}

// ReconnectStrategy defines how reconnection should be handled.
type ReconnectStrategy struct {
	// MaxAttempts is the maximum number of reconnection attempts (0 for infinite)
	MaxAttempts int

	// InitialDelay is the initial delay between reconnection attempts
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between reconnection attempts
	MaxDelay time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64

	// Jitter adds randomness to reconnection delays
	Jitter bool
}
