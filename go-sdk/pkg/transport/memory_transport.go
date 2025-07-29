package transport

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// MemoryConfig is the configuration for memory transport
type MemoryConfig struct {
	BufferSize int
}

// Validate validates the configuration
func (c *MemoryConfig) Validate() error {
	if c.BufferSize <= 0 {
		return fmt.Errorf("buffer size must be positive: got %d", c.BufferSize)
	}
	return nil
}

// Clone creates a deep copy of the configuration
func (c *MemoryConfig) Clone() Config {
	return &MemoryConfig{
		BufferSize: c.BufferSize,
	}
}

// GetType returns the transport type
func (c *MemoryConfig) GetType() string {
	return "memory"
}

// GetEndpoint returns empty for memory transport
func (c *MemoryConfig) GetEndpoint() string {
	return ""
}

// GetTimeout returns the connection timeout
func (c *MemoryConfig) GetTimeout() time.Duration {
	return 5 * time.Second
}

// GetHeaders returns empty headers for memory transport
func (c *MemoryConfig) GetHeaders() map[string]string {
	return nil
}

// IsSecure returns true as memory transport is always secure
func (c *MemoryConfig) IsSecure() bool {
	return true
}

// MemoryTransport is an in-memory transport implementation for testing
type MemoryTransport struct {
	mu            sync.RWMutex
	connected     bool
	eventChan     chan events.Event
	errorChan     chan error
	bufferSize    int
	closeOnce     sync.Once
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewMemoryTransport creates a new in-memory transport
func NewMemoryTransport(bufferSize int) *MemoryTransport {
	ctx, cancel := context.WithCancel(context.Background())
	return &MemoryTransport{
		bufferSize: bufferSize,
		eventChan:  make(chan events.Event, bufferSize),
		errorChan:  make(chan error, bufferSize),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Connect establishes the connection
func (t *MemoryTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return ErrAlreadyConnected
	}

	t.connected = true
	return nil
}

// Send sends an event through the transport
func (t *MemoryTransport) Send(ctx context.Context, event TransportEvent) error {
	t.mu.RLock()
	connected := t.connected
	t.mu.RUnlock()

	if !connected {
		return ErrNotConnected
	}

	// Convert TransportEvent to events.Event
	baseEvent := &events.BaseEvent{
		EventType: events.EventType(event.Type()),
	}
	baseEvent.SetTimestamp(event.Timestamp().UnixMilli())

	select {
	case t.eventChan <- baseEvent:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.ctx.Done():
		return fmt.Errorf("transport closed: %w", t.ctx.Err())
	default:
		// Buffer full - backpressure error
		err := fmt.Errorf("transport buffer full (size: %d): %w", t.bufferSize, ErrBackpressureActive)
		select {
		case t.errorChan <- err:
		default:
		}
		return err
	}
}

// Receive returns the channel for receiving events
func (t *MemoryTransport) Receive() <-chan events.Event {
	return t.eventChan
}

// Errors returns the channel for receiving errors
func (t *MemoryTransport) Errors() <-chan error {
	return t.errorChan
}

// Channels returns both event and error channels together
func (t *MemoryTransport) Channels() (<-chan events.Event, <-chan error) {
	return t.eventChan, t.errorChan
}

// Close closes the transport
func (t *MemoryTransport) Close(ctx context.Context) error {
	var closeErr error
	t.closeOnce.Do(func() {
		t.mu.Lock()
		t.connected = false
		t.mu.Unlock()

		t.cancel()
		close(t.eventChan)
		close(t.errorChan)
	})
	return closeErr
}

// IsConnected returns whether the transport is connected
func (t *MemoryTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// Type returns the transport type
func (t *MemoryTransport) Type() string {
	return "memory"
}

// Config returns the transport configuration
func (t *MemoryTransport) Config() Config {
	return &MemoryConfig{
		BufferSize: t.bufferSize,
	}
}

// Context returns the transport context
func (t *MemoryTransport) Context() context.Context {
	return t.ctx
}

// Stats returns transport statistics
func (t *MemoryTransport) Stats() TransportStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return TransportStats{
		ConnectedAt:      time.Now(),
		ReconnectCount:   0,
		LastError:        nil,
		Uptime:           time.Since(time.Now()),
		EventsSent:       0, // Would need counters
		EventsReceived:   0,
		BytesSent:        0,
		BytesReceived:    0,
		AverageLatency:   0,
		ErrorCount:       0,
		LastEventSentAt:  time.Time{},
		LastEventRecvAt:  time.Time{},
	}
}

// SetOption sets a transport option
func (t *MemoryTransport) SetOption(key string, value interface{}) error {
	// Memory transport doesn't support runtime options
	return fmt.Errorf("option %s not supported: %w", key, ErrUnsupportedCapability)
}

// GetOption gets a transport option
func (t *MemoryTransport) GetOption(key string) (interface{}, error) {
	switch key {
	case "buffer_size":
		return t.bufferSize, nil
	default:
		return nil, fmt.Errorf("option %s not found: %w", key, ErrUnsupportedCapability)
	}
}