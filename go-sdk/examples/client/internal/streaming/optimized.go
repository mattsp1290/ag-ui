package streaming

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// BufferSize constants for different use cases
const (
	// SmallBuffer for low-latency, small messages (4KB)
	SmallBuffer = 4 * 1024
	// MediumBuffer for typical SSE streams (16KB)
	MediumBuffer = 16 * 1024
	// LargeBuffer for high-throughput scenarios (64KB)
	LargeBuffer = 64 * 1024
	// MaxBuffer for very large messages (256KB)
	MaxBuffer = 256 * 1024
)

// StreamConfig holds configuration for optimized streaming
type StreamConfig struct {
	// BufferSize for the reader (default: MediumBuffer)
	BufferSize int
	// MaxRetries for connection failures (default: 3)
	MaxRetries int
	// RetryDelay base delay for exponential backoff (default: 1s)
	RetryDelay time.Duration
	// MaxRetryDelay maximum delay between retries (default: 30s)
	MaxRetryDelay time.Duration
	// ConnectionTimeout for establishing connection (default: 30s)
	ConnectionTimeout time.Duration
	// ReadTimeout for reading from stream (default: 2m)
	ReadTimeout time.Duration
	// KeepAliveInterval for sending keep-alive pings (default: 30s)
	KeepAliveInterval time.Duration
	// Logger for debug output
	Logger *logrus.Logger
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *StreamConfig {
	return &StreamConfig{
		BufferSize:        MediumBuffer,
		MaxRetries:        3,
		RetryDelay:        time.Second,
		MaxRetryDelay:     30 * time.Second,
		ConnectionTimeout: 30 * time.Second,
		ReadTimeout:       2 * time.Minute,
		KeepAliveInterval: 30 * time.Second,
		Logger:            logrus.New(),
	}
}

// SSEEvent represents a server-sent event
type SSEEvent struct {
	ID    string
	Event string
	Data  []byte
	Retry int
}

// OptimizedStream provides optimized SSE streaming with buffer management
type OptimizedStream struct {
	config     *StreamConfig
	client     *http.Client
	bufferPool *sync.Pool
	mu         sync.RWMutex
	activeConns map[string]*streamConnection
}

// streamConnection represents an active SSE connection
type streamConnection struct {
	id       string
	request  *http.Request
	response *http.Response
	reader   *bufio.Reader
	ctx      context.Context
	cancel   context.CancelFunc
	events   chan SSEEvent
	errors   chan error
}

// NewOptimizedStream creates a new optimized streaming client
func NewOptimizedStream(config *StreamConfig) *OptimizedStream {
	if config == nil {
		config = DefaultConfig()
	}

	// Create buffer pool for efficient memory management
	bufferPool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, config.BufferSize)
		},
	}

	return &OptimizedStream{
		config:      config,
		client:      &http.Client{Timeout: config.ConnectionTimeout},
		bufferPool:  bufferPool,
		activeConns: make(map[string]*streamConnection),
	}
}

// Connect establishes an SSE connection with retry logic
func (s *OptimizedStream) Connect(ctx context.Context, req *http.Request) (<-chan SSEEvent, <-chan error, error) {
	events := make(chan SSEEvent, 100) // Buffered channel for events
	errors := make(chan error, 10)     // Buffered channel for errors

	// Apply retry logic with exponential backoff
	var lastErr error
	retryDelay := s.config.RetryDelay

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			s.config.Logger.WithFields(logrus.Fields{
				"attempt": attempt,
				"delay":   retryDelay,
			}).Info("Retrying SSE connection")
			
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(retryDelay):
			}

			// Exponential backoff
			retryDelay *= 2
			if retryDelay > s.config.MaxRetryDelay {
				retryDelay = s.config.MaxRetryDelay
			}
		}

		// Attempt connection
		resp, err := s.client.Do(req.WithContext(ctx))
		if err != nil {
			lastErr = err
			continue
		}

		// Check response status
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("unexpected status: %d", resp.StatusCode)
			continue
		}

		// Create optimized reader with configured buffer size
		reader := bufio.NewReaderSize(resp.Body, s.config.BufferSize)

		// Create connection context
		connCtx, cancel := context.WithCancel(ctx)

		// Create and store connection
		conn := &streamConnection{
			id:       fmt.Sprintf("conn-%d", time.Now().UnixNano()),
			request:  req,
			response: resp,
			reader:   reader,
			ctx:      connCtx,
			cancel:   cancel,
			events:   events,
			errors:   errors,
		}

		s.mu.Lock()
		s.activeConns[conn.id] = conn
		s.mu.Unlock()

		// Start processing in background
		go s.processStream(conn)

		s.config.Logger.WithFields(logrus.Fields{
			"connection_id": conn.id,
			"buffer_size":   s.config.BufferSize,
		}).Info("SSE connection established")

		return events, errors, nil
	}

	return nil, nil, fmt.Errorf("failed after %d retries: %w", s.config.MaxRetries, lastErr)
}

// processStream reads and processes SSE events from the stream
func (s *OptimizedStream) processStream(conn *streamConnection) {
	defer func() {
		// Cleanup on exit
		s.mu.Lock()
		delete(s.activeConns, conn.id)
		s.mu.Unlock()

		conn.response.Body.Close()
		close(conn.events)
		close(conn.errors)
		conn.cancel()

		s.config.Logger.WithField("connection_id", conn.id).Info("SSE connection closed")
	}()

	// Buffer for building events
	var eventBuffer bytes.Buffer
	var currentEvent SSEEvent

	// Read loop with timeout handling
	readTimeout := time.NewTimer(s.config.ReadTimeout)
	defer readTimeout.Stop()

	for {
		select {
		case <-conn.ctx.Done():
			return
		case <-readTimeout.C:
			conn.errors <- fmt.Errorf("read timeout after %v", s.config.ReadTimeout)
			return
		default:
			// Set read deadline (removed - timeouts handled by context)

			// Read line with optimized buffer
			line, err := conn.reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					conn.errors <- fmt.Errorf("read error: %w", err)
				}
				return
			}

			// Reset read timeout
			readTimeout.Reset(s.config.ReadTimeout)

			// Process SSE line
			line = strings.TrimSpace(line)

			// Empty line signals end of event
			if line == "" && eventBuffer.Len() > 0 {
				currentEvent.Data = eventBuffer.Bytes()
				
				select {
				case conn.events <- currentEvent:
					s.config.Logger.WithFields(logrus.Fields{
						"event": currentEvent.Event,
						"size":  len(currentEvent.Data),
					}).Debug("SSE event sent")
				case <-conn.ctx.Done():
					return
				}

				// Reset for next event
				eventBuffer.Reset()
				currentEvent = SSEEvent{}
				continue
			}

			// Skip comments
			if strings.HasPrefix(line, ":") {
				continue
			}

			// Parse SSE fields
			if strings.HasPrefix(line, "id:") {
				currentEvent.ID = strings.TrimSpace(line[3:])
			} else if strings.HasPrefix(line, "event:") {
				currentEvent.Event = strings.TrimSpace(line[6:])
			} else if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(line[5:])
				if eventBuffer.Len() > 0 {
					eventBuffer.WriteByte('\n')
				}
				eventBuffer.WriteString(data)
			} else if strings.HasPrefix(line, "retry:") {
				// Parse retry value (not used in this implementation)
				continue
			}
		}
	}
}

// Close closes all active connections
func (s *OptimizedStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, conn := range s.activeConns {
		conn.cancel()
		conn.response.Body.Close()
	}

	s.activeConns = make(map[string]*streamConnection)
	return nil
}

// GetActiveConnections returns the number of active connections
func (s *OptimizedStream) GetActiveConnections() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.activeConns)
}

// timeoutReader wraps an io.Reader with timeout support
type timeoutReader struct {
	reader  io.ReadCloser
	timeout time.Duration
}

func (r *timeoutReader) Read(p []byte) (int, error) {
	type result struct {
		n   int
		err error
	}

	ch := make(chan result, 1)
	go func() {
		n, err := r.reader.Read(p)
		ch <- result{n, err}
	}()

	select {
	case res := <-ch:
		return res.n, res.err
	case <-time.After(r.timeout):
		return 0, fmt.Errorf("read timeout")
	}
}

func (r *timeoutReader) Close() error {
	if closer, ok := r.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// ParseJSONEvent parses JSON data from an SSE event
func ParseJSONEvent(event SSEEvent, v interface{}) error {
	return json.Unmarshal(event.Data, v)
}

// ConnectionPool manages a pool of SSE connections
type ConnectionPool struct {
	stream      *OptimizedStream
	maxConns    int
	connections sync.Map
	mu          sync.Mutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *StreamConfig, maxConns int) *ConnectionPool {
	return &ConnectionPool{
		stream:   NewOptimizedStream(config),
		maxConns: maxConns,
	}
}

// Get retrieves or creates a connection for the given key
func (p *ConnectionPool) Get(ctx context.Context, key string, req *http.Request) (<-chan SSEEvent, <-chan error, error) {
	// Check if connection exists
	if val, ok := p.connections.Load(key); ok {
		conn := val.(*pooledConnection)
		if conn.isActive() {
			return conn.events, conn.errors, nil
		}
		// Remove stale connection
		p.connections.Delete(key)
	}

	// Check pool limit
	p.mu.Lock()
	count := 0
	p.connections.Range(func(_, _ interface{}) bool {
		count++
		return count < p.maxConns
	})
	p.mu.Unlock()

	if count >= p.maxConns {
		return nil, nil, fmt.Errorf("connection pool limit reached (%d)", p.maxConns)
	}

	// Create new connection
	events, errors, err := p.stream.Connect(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	// Store in pool
	poolConn := &pooledConnection{
		events:    events,
		errors:    errors,
		createdAt: time.Now(),
	}
	p.connections.Store(key, poolConn)

	return events, errors, nil
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	p.connections.Range(func(key, _ interface{}) bool {
		p.connections.Delete(key)
		return true
	})
	return p.stream.Close()
}

// pooledConnection represents a connection in the pool
type pooledConnection struct {
	events    <-chan SSEEvent
	errors    <-chan error
	createdAt time.Time
}

func (c *pooledConnection) isActive() bool {
	// Check if channels are still open
	select {
	case _, ok := <-c.events:
		return ok
	default:
		return true
	}
}