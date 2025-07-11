package streaming

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/errors"
)

// StreamState represents the current state of a stream
type StreamState int32

const (
	StreamStateIdle StreamState = iota
	StreamStateActive
	StreamStatePaused
	StreamStateClosed
	StreamStateError
)

// StreamConfig holds configuration for stream management
type StreamConfig struct {
	// BufferSize is the size of the internal buffer
	BufferSize int

	// MaxConcurrency limits concurrent operations
	MaxConcurrency int

	// FlushInterval determines how often to flush buffers
	FlushInterval time.Duration

	// BackpressureThreshold triggers backpressure handling
	BackpressureThreshold int

	// EnableMetrics enables metric collection
	EnableMetrics bool

	// OnBackpressure is called when backpressure is detected
	OnBackpressure func(pending int)

	// OnError is called when an error occurs
	OnError func(error)
}

// DefaultStreamConfig returns a default configuration
func DefaultStreamConfig() *StreamConfig {
	return &StreamConfig{
		BufferSize:            8192,
		MaxConcurrency:        10,
		FlushInterval:         100 * time.Millisecond,
		BackpressureThreshold: 1000,
		EnableMetrics:         true,
	}
}

// StreamManager coordinates streaming operations
type StreamManager struct {
	config  *StreamConfig
	encoder encoding.StreamEncoder
	decoder encoding.StreamDecoder
	state   atomic.Int32
	metrics *StreamMetrics

	// Stream lifecycle
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	startOnce  sync.Once
	closeOnce  sync.Once

	// Buffer management
	writeBuffer chan writeRequest
	readBuffer  chan events.Event
	errors      chan error

	// Flow control
	flowController *FlowController
	bufferPool     sync.Pool

	// Synchronization
	mu sync.RWMutex
}

type writeRequest struct {
	event events.Event
	done  chan error
}

// NewStreamManager creates a new stream manager
func NewStreamManager(encoder encoding.StreamEncoder, decoder encoding.StreamDecoder, config *StreamConfig) *StreamManager {
	if config == nil {
		config = DefaultStreamConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	sm := &StreamManager{
		config:         config,
		encoder:        encoder,
		decoder:        decoder,
		ctx:            ctx,
		cancel:         cancel,
		writeBuffer:    make(chan writeRequest, config.BufferSize),
		readBuffer:     make(chan events.Event, config.BufferSize),
		errors:         make(chan error, 10),
		flowController: NewFlowController(config.BackpressureThreshold),
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, config.BufferSize)
			},
		},
	}

	if config.EnableMetrics {
		sm.metrics = NewStreamMetrics()
	}

	sm.state.Store(int32(StreamStateIdle))
	return sm
}

// Start initializes the stream manager
func (sm *StreamManager) Start() error {
	// Check if already started or closed
	currentState := StreamState(sm.state.Load())
	if currentState != StreamStateIdle {
		return errors.NewStreamingError("STREAM_MANAGER_INVALID_STATE", "stream manager already started or closed")
	}
	
	var startErr error
	sm.startOnce.Do(func() {
		if !sm.state.CompareAndSwap(int32(StreamStateIdle), int32(StreamStateActive)) {
			startErr = errors.NewStreamingError("STREAM_MANAGER_INVALID_STATE", "stream manager already started or closed")
			return
		}

		// Start worker goroutines
		sm.wg.Add(2)
		go sm.errorHandler()
		go sm.metricsCollector()
	})

	return startErr
}

// Stop gracefully shuts down the stream manager
func (sm *StreamManager) Stop() error {
	var closeErr error
	sm.closeOnce.Do(func() {
		// Set state to closing
		sm.state.Store(int32(StreamStateClosed))

		// Cancel context to stop all operations
		sm.cancel()

		// Close channels
		close(sm.writeBuffer)
		close(sm.readBuffer)
		close(sm.errors)

		// Wait for all goroutines to finish
		sm.wg.Wait()

		if sm.metrics != nil {
			sm.metrics.Close()
		}
	})

	return closeErr
}

// WriteStream handles writing events to a stream
func (sm *StreamManager) WriteStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	if sm.GetState() != StreamStateActive {
		return errors.NewStreamingError("STREAM_MANAGER_NOT_ACTIVE", "stream manager not active")
	}

	// Start encoder stream
	if err := sm.encoder.StartStream(ctx, output); err != nil {
		return errors.NewStreamingError("ENCODER_STREAM_START_FAILED", "failed to start encoder stream").WithCause(err)
	}
	defer sm.encoder.EndStream(ctx)

	// Start write worker
	sm.wg.Add(1)
	go sm.writeWorker(output)

	// Process input events
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-sm.ctx.Done():
			return sm.ctx.Err()
		case event, ok := <-input:
			if !ok {
				return nil // Input channel closed
			}

			// Check backpressure
			if sm.flowController.ShouldThrottle() {
				if sm.config.OnBackpressure != nil {
					sm.config.OnBackpressure(len(sm.writeBuffer))
				}
				// Apply backpressure by blocking
				time.Sleep(10 * time.Millisecond)
			}

			// Send write request
			req := writeRequest{
				event: event,
				done:  make(chan error, 1),
			}

			select {
			case sm.writeBuffer <- req:
				// Wait for write completion
				if err := <-req.done; err != nil {
					sm.reportError(err)
					return err
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// ReadStream handles reading events from a stream
func (sm *StreamManager) ReadStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	if sm.GetState() != StreamStateActive {
		return errors.NewStreamingError("STREAM_MANAGER_NOT_ACTIVE", "stream manager not active")
	}

	// Start decoder stream
	if err := sm.decoder.StartStream(ctx, input); err != nil {
		return errors.NewStreamingError("DECODER_STREAM_START_FAILED", "failed to start decoder stream").WithCause(err)
	}
	defer sm.decoder.EndStream(ctx)

	// Start read worker
	sm.wg.Add(1)
	go sm.readWorker(input)

	// Process read events
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-sm.ctx.Done():
			return sm.ctx.Err()
		case event, ok := <-sm.readBuffer:
			if !ok {
				return nil // Read buffer closed
			}

			// Send to output
			select {
			case output <- event:
				if sm.metrics != nil {
					sm.metrics.RecordEvent(event)
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// writeWorker handles write operations
func (sm *StreamManager) writeWorker(output io.Writer) {
	defer sm.wg.Done()

	flushTicker := time.NewTicker(sm.config.FlushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case req, ok := <-sm.writeBuffer:
			if !ok {
				return // Channel closed
			}

			// Write event
			err := sm.encoder.WriteEvent(context.Background(), req.event)
			if err != nil {
				req.done <- err
				sm.reportError(err)
				continue
			}

			// Update metrics
			if sm.metrics != nil {
				sm.metrics.RecordEvent(req.event)
			}

			// Update flow control
			sm.flowController.RecordWrite()

			req.done <- nil

		case <-flushTicker.C:
			// Periodic flush if needed
			if flusher, ok := output.(interface{ Flush() error }); ok {
				if err := flusher.Flush(); err != nil {
					sm.reportError(errors.NewStreamingError("FLUSH_ERROR", "flush error").WithCause(err))
				}
			}
		}
	}
}

// readWorker handles read operations
func (sm *StreamManager) readWorker(input io.Reader) {
	defer sm.wg.Done()
	defer close(sm.readBuffer)

	for {
		select {
		case <-sm.ctx.Done():
			return
		default:
			// Read event
			event, err := sm.decoder.ReadEvent(context.Background())
			if err != nil {
				if err == io.EOF {
					return // Normal end of stream
				}
				sm.reportError(err)
				return
			}

			// Update flow control
			sm.flowController.RecordRead()

			// Send to buffer
			select {
			case sm.readBuffer <- event:
			case <-sm.ctx.Done():
				return
			}
		}
	}
}

// errorHandler processes errors
func (sm *StreamManager) errorHandler() {
	defer sm.wg.Done()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case err, ok := <-sm.errors:
			if !ok {
				return
			}

			if sm.config.OnError != nil {
				sm.config.OnError(err)
			}

			// Update state on critical errors
			if isCriticalError(err) {
				sm.state.Store(int32(StreamStateError))
			}
		}
	}
}

// metricsCollector collects metrics periodically
func (sm *StreamManager) metricsCollector() {
	defer sm.wg.Done()

	if sm.metrics == nil {
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.metrics.UpdateBufferSize(len(sm.writeBuffer), len(sm.readBuffer))
			sm.metrics.UpdateFlowControl(sm.flowController.GetMetrics())
		}
	}
}

// GetState returns the current stream state
func (sm *StreamManager) GetState() StreamState {
	return StreamState(sm.state.Load())
}

// GetMetrics returns current metrics
func (sm *StreamManager) GetMetrics() *StreamMetrics {
	return sm.metrics
}

// Pause pauses the stream
func (sm *StreamManager) Pause() error {
	if !sm.state.CompareAndSwap(int32(StreamStateActive), int32(StreamStatePaused)) {
		return errors.NewStreamingError("PAUSE_INVALID_STATE", "cannot pause: stream not active")
	}
	return nil
}

// Resume resumes the stream
func (sm *StreamManager) Resume() error {
	if !sm.state.CompareAndSwap(int32(StreamStatePaused), int32(StreamStateActive)) {
		return errors.NewStreamingError("RESUME_INVALID_STATE", "cannot resume: stream not paused")
	}
	return nil
}

// reportError reports an error
func (sm *StreamManager) reportError(err error) {
	select {
	case sm.errors <- err:
	default:
		// Error channel full, drop error
	}
}

// isCriticalError determines if an error is critical
func isCriticalError(err error) bool {
	// Define critical error conditions
	return false
}

// String returns a string representation of the stream state
func (s StreamState) String() string {
	switch s {
	case StreamStateIdle:
		return "idle"
	case StreamStateActive:
		return "active"
	case StreamStatePaused:
		return "paused"
	case StreamStateClosed:
		return "closed"
	case StreamStateError:
		return "error"
	default:
		return "unknown"
	}
}