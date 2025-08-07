package sse

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport/common"
)

// EventStream manages efficient event streaming for HTTP SSE transport
type EventStream struct {
	// Configuration
	config      *StreamConfig
	compression CompressionType

	// Event processing
	eventChan  chan events.Event
	batchChan  chan *EventBatch
	outputChan chan *StreamChunk
	errorChan  chan error

	// Flow control and backpressure
	flowController *FlowController
	sequencer      *EventSequencer

	// Buffer management
	bufferPool  *BufferPool
	chunkBuffer *ChunkBuffer

	// Performance monitoring
	metrics *StreamMetrics

	// Lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started int32
	closed  int32

	// Synchronization
	mu sync.RWMutex
}

// StreamConfig defines configuration for the event stream
type StreamConfig struct {
	// Buffering settings
	EventBufferSize int           `json:"event_buffer_size"`
	ChunkBufferSize int           `json:"chunk_buffer_size"`
	MaxChunkSize    int           `json:"max_chunk_size"`
	FlushInterval   time.Duration `json:"flush_interval"`

	// Batching settings
	BatchEnabled bool          `json:"batch_enabled"`
	BatchSize    int           `json:"batch_size"`
	BatchTimeout time.Duration `json:"batch_timeout"`
	MaxBatchSize int           `json:"max_batch_size"`

	// Compression settings
	CompressionEnabled bool            `json:"compression_enabled"`
	CompressionType    CompressionType `json:"compression_type"`
	CompressionLevel   int             `json:"compression_level"`
	MinCompressionSize int             `json:"min_compression_size"`

	// Flow control settings
	MaxConcurrentEvents int           `json:"max_concurrent_events"`
	BackpressureTimeout time.Duration `json:"backpressure_timeout"`
	DrainTimeout        time.Duration `json:"drain_timeout"`

	// Sequencing settings
	SequenceEnabled  bool `json:"sequence_enabled"`
	OrderingRequired bool `json:"ordering_required"`
	OutOfOrderBuffer int  `json:"out_of_order_buffer"`

	// Performance settings
	WorkerCount     int           `json:"worker_count"`
	EnableMetrics   bool          `json:"enable_metrics"`
	MetricsInterval time.Duration `json:"metrics_interval"`
}

// CompressionType defines supported compression algorithms
type CompressionType string

const (
	CompressionNone    CompressionType = "none"
	CompressionGzip    CompressionType = "gzip"
	CompressionDeflate CompressionType = "deflate"
)

// EventBatch represents a batch of events for processing
type EventBatch struct {
	Events    []events.Event `json:"events"`
	Timestamp time.Time      `json:"timestamp"`
	BatchID   string         `json:"batch_id"`
	Size      int            `json:"size"`
}

// StreamChunk represents a processed chunk ready for transmission
type StreamChunk struct {
	Data        []byte    `json:"data"`
	EventType   string    `json:"event_type"`
	EventID     string    `json:"event_id"`
	Retry       *int      `json:"retry,omitempty"`
	Compressed  bool      `json:"compressed"`
	SequenceNum uint64    `json:"sequence_num"`
	ChunkIndex  int       `json:"chunk_index"`
	TotalChunks int       `json:"total_chunks"`
	Timestamp   time.Time `json:"timestamp"`
}

// FlowController manages backpressure and flow control
type FlowController struct {
	maxConcurrent  int32
	current        int32
	backpressureCh chan struct{}
	timeout        time.Duration
	drainTimeout   time.Duration
	metrics        *FlowMetrics
	mu             sync.RWMutex
}

// FlowMetrics tracks flow control statistics
type FlowMetrics struct {
	EventsProcessed    uint64 `json:"events_processed"`
	EventsDropped      uint64 `json:"events_dropped"`
	BackpressureEvents uint64 `json:"backpressure_events"`
	AverageWaitTime    int64  `json:"average_wait_time_ns"`
	MaxWaitTime        int64  `json:"max_wait_time_ns"`
	CurrentConcurrent  int32  `json:"current_concurrent"`
}

// EventSequencer manages event ordering and sequencing
type EventSequencer struct {
	enabled          bool
	orderingRequired bool
	nextSequence     uint64
	sequenceBuffer   map[uint64]*SequencedEvent
	bufferSize       int
	timeout          time.Duration
	outputChan       chan *SequencedEvent
	metrics          *SequenceMetrics
	mu               sync.RWMutex
}

// SequencedEvent wraps an event with sequence information
type SequencedEvent struct {
	Event       events.Event `json:"event"`
	SequenceNum uint64       `json:"sequence_num"`
	Timestamp   time.Time    `json:"timestamp"`
	Retries     int          `json:"retries"`
}

// SequenceMetrics tracks sequencing statistics
type SequenceMetrics struct {
	EventsSequenced   uint64  `json:"events_sequenced"`
	OutOfOrderEvents  uint64  `json:"out_of_order_events"`
	DroppedEvents     uint64  `json:"dropped_events"`
	BufferUtilization float64 `json:"buffer_utilization"`
	AverageDelay      int64   `json:"average_delay_ns"`
	MaxDelay          int64   `json:"max_delay_ns"`
}

// BufferPool manages reusable buffers for efficient memory usage
type BufferPool struct {
	pool sync.Pool
	size int
}

// ChunkBuffer manages chunking of large events
type ChunkBuffer struct {
	maxChunkSize int
	buffer       *bytes.Buffer
	chunks       []*StreamChunk
	currentChunk int
	mu           sync.RWMutex
}

// StreamMetrics tracks overall stream performance
type StreamMetrics struct {
	// Event statistics
	TotalEvents      uint64  `json:"total_events"`
	EventsPerSecond  float64 `json:"events_per_second"`
	EventsProcessed  uint64  `json:"events_processed"`
	EventsDropped    uint64  `json:"events_dropped"`
	EventsCompressed uint64  `json:"events_compressed"`

	// Batch statistics
	TotalBatches        uint64  `json:"total_batches"`
	AverageBatchSize    float64 `json:"average_batch_size"`
	BatchProcessingTime int64   `json:"batch_processing_time_ns"`

	// Compression statistics
	CompressionRatio float64 `json:"compression_ratio"`
	CompressionTime  int64   `json:"compression_time_ns"`
	BytesSaved       uint64  `json:"bytes_saved"`

	// Performance statistics
	AverageLatency int64  `json:"average_latency_ns"`
	MaxLatency     int64  `json:"max_latency_ns"`
	ThroughputBps  uint64 `json:"throughput_bps"`
	MemoryUsage    uint64 `json:"memory_usage_bytes"`

	// Error statistics
	ProcessingErrors  uint64 `json:"processing_errors"`
	CompressionErrors uint64 `json:"compression_errors"`
	SequencingErrors  uint64 `json:"sequencing_errors"`

	// Flow control statistics
	FlowControl *FlowMetrics     `json:"flow_control"`
	Sequencing  *SequenceMetrics `json:"sequencing"`

	// Timing
	StartTime     time.Time `json:"start_time"`
	LastEventTime time.Time `json:"last_event_time"`

	// Synchronization
	mu sync.RWMutex
}

// DefaultStreamConfig returns a default stream configuration
func DefaultStreamConfig() *StreamConfig {
	return &StreamConfig{
		EventBufferSize:     5000,                  // Increased buffer size for higher throughput
		ChunkBufferSize:     500,                   // Increased chunk buffer
		MaxChunkSize:        64 * 1024,             // 64KB
		FlushInterval:       10 * time.Millisecond, // Reduced flush interval for faster processing
		BatchEnabled:        true,
		BatchSize:           100,                   // Increased batch size for better throughput
		BatchTimeout:        10 * time.Millisecond, // Much faster batching for low latency
		MaxBatchSize:        1000,                  // Increased max batch size
		CompressionEnabled:  true,
		CompressionType:     CompressionGzip,
		CompressionLevel:    1,   // Fastest compression level for performance
		MinCompressionSize:  512, // Lower threshold for compression
		MaxConcurrentEvents: 500, // Higher concurrency for better throughput
		BackpressureTimeout: 5 * time.Second,
		DrainTimeout:        30 * time.Second,
		SequenceEnabled:     false, // Disable sequencing for performance gain
		OrderingRequired:    false,
		OutOfOrderBuffer:    1000,
		WorkerCount:         8, // More workers for higher throughput
		EnableMetrics:       true,
		MetricsInterval:     30 * time.Second,
	}
}

// NewEventStream creates a new event streaming instance
func NewEventStream(config *StreamConfig) (*EventStream, error) {
	if config == nil {
		config = DefaultStreamConfig()
	}

	if err := validateStreamConfig(config); err != nil {
		return nil, fmt.Errorf("invalid stream config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	stream := &EventStream{
		config:      config,
		compression: config.CompressionType,
		eventChan:   make(chan events.Event, config.EventBufferSize),
		batchChan:   make(chan *EventBatch, config.EventBufferSize/config.BatchSize+1),
		outputChan:  make(chan *StreamChunk, config.ChunkBufferSize),
		errorChan:   make(chan error, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Initialize components
	stream.flowController = NewFlowController(config.MaxConcurrentEvents,
		config.BackpressureTimeout, config.DrainTimeout)
	stream.sequencer = NewEventSequencer(config.SequenceEnabled,
		config.OrderingRequired, config.OutOfOrderBuffer)
	stream.bufferPool = NewBufferPool(config.MaxChunkSize)
	stream.chunkBuffer = NewChunkBuffer(config.MaxChunkSize)

	if config.EnableMetrics {
		stream.metrics = NewStreamMetrics()
	}

	return stream, nil
}

// Start begins the event streaming process
func (s *EventStream) Start() error {
	if !atomic.CompareAndSwapInt32(&s.started, 0, 1) {
		return messages.NewStreamingError("stream", 0, "stream already started")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isClosed() {
		return messages.NewStreamingError("stream", 0, "stream is closed")
	}

	// Start processing workers
	for i := 0; i < s.config.WorkerCount; i++ {
		s.wg.Add(1)
		go s.eventProcessor(i)
	}

	// Start batching processor if enabled
	if s.config.BatchEnabled {
		s.wg.Add(1)
		go s.batchProcessor()
	}

	// Start chunk processor
	s.wg.Add(1)
	go s.chunkProcessor()

	// Start flow controller
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.flowController.Run(s.ctx)
	}()

	// Start sequencer if enabled
	if s.config.SequenceEnabled {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.sequencer.Run(s.ctx)
		}()
	}

	// Start metrics collector if enabled
	if s.config.EnableMetrics && s.metrics != nil {
		s.wg.Add(1)
		go s.metricsCollector()
	}

	if s.metrics != nil {
		s.metrics.StartTime = time.Now()
	}

	return nil
}

// SendEvent sends an event through the streaming pipeline
func (s *EventStream) SendEvent(event events.Event) error {
	if s.isClosed() {
		return messages.NewStreamingError("stream", 0, "stream is closed")
	}

	if !s.isStarted() {
		return messages.NewStreamingError("stream", 0, "stream not started")
	}

	if event == nil {
		return fmt.Errorf("event cannot be nil: %w", common.NewValidationError("event", "required", "event must not be nil", nil))
	}

	// Validate event
	if err := event.Validate(); err != nil {
		return fmt.Errorf("event validation failed: %w", err)
	}

	// Apply flow control
	if err := s.flowController.Acquire(s.ctx); err != nil {
		if s.metrics != nil {
			atomic.AddUint64(&s.metrics.EventsDropped, 1)
		}
		return fmt.Errorf("flow control rejected event: %w", err)
	}

	// Send event to processing pipeline
	select {
	case s.eventChan <- event:
		if s.metrics != nil {
			atomic.AddUint64(&s.metrics.TotalEvents, 1)

			// Update time fields under mutex
			s.metrics.mu.Lock()
			s.metrics.LastEventTime = time.Now()
			s.metrics.mu.Unlock()
		}
		return nil
	case <-s.ctx.Done():
		s.flowController.Release()
		return messages.NewStreamingError("stream", 0, "stream context cancelled")
	case <-time.After(s.config.BackpressureTimeout):
		s.flowController.Release()
		if s.metrics != nil {
			atomic.AddUint64(&s.metrics.EventsDropped, 1)
		}
		return messages.NewStreamingError("stream", 0, "backpressure timeout exceeded")
	}
}

// ReceiveChunks returns a channel for receiving processed stream chunks
func (s *EventStream) ReceiveChunks() <-chan *StreamChunk {
	return s.outputChan
}

// GetErrorChannel returns the error channel for monitoring stream errors
func (s *EventStream) GetErrorChannel() <-chan error {
	return s.errorChan
}

// GetMetrics returns current stream metrics
func (s *EventStream) GetMetrics() *StreamMetrics {
	if s.metrics == nil {
		return nil
	}

	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	// Create a copy to avoid data races, using atomic operations for atomically updated fields
	metrics := StreamMetrics{
		TotalEvents:         atomic.LoadUint64(&s.metrics.TotalEvents),
		EventsPerSecond:     s.metrics.EventsPerSecond,
		EventsProcessed:     atomic.LoadUint64(&s.metrics.EventsProcessed),
		EventsDropped:       atomic.LoadUint64(&s.metrics.EventsDropped),
		EventsCompressed:    atomic.LoadUint64(&s.metrics.EventsCompressed),
		TotalBatches:        atomic.LoadUint64(&s.metrics.TotalBatches),
		AverageBatchSize:    s.metrics.AverageBatchSize,
		BatchProcessingTime: atomic.LoadInt64(&s.metrics.BatchProcessingTime),
		CompressionRatio:    s.metrics.CompressionRatio,
		CompressionTime:     atomic.LoadInt64(&s.metrics.CompressionTime),
		BytesSaved:          atomic.LoadUint64(&s.metrics.BytesSaved),
		AverageLatency:      atomic.LoadInt64(&s.metrics.AverageLatency),
		MaxLatency:          atomic.LoadInt64(&s.metrics.MaxLatency),
		ThroughputBps:       atomic.LoadUint64(&s.metrics.ThroughputBps),
		MemoryUsage:         atomic.LoadUint64(&s.metrics.MemoryUsage),
		ProcessingErrors:    atomic.LoadUint64(&s.metrics.ProcessingErrors),
		CompressionErrors:   atomic.LoadUint64(&s.metrics.CompressionErrors),
		SequencingErrors:    atomic.LoadUint64(&s.metrics.SequencingErrors),
		StartTime:           s.metrics.StartTime,
		LastEventTime:       s.metrics.LastEventTime,
	}

	if s.metrics.FlowControl != nil {
		flowMetrics := *s.metrics.FlowControl
		metrics.FlowControl = &flowMetrics
	}
	if s.metrics.Sequencing != nil {
		seqMetrics := *s.metrics.Sequencing
		metrics.Sequencing = &seqMetrics
	}

	return &metrics
}

// Close gracefully shuts down the event stream
func (s *EventStream) Close() error {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return nil // Already closed
	}

	// Cancel context to signal shutdown to all workers
	s.cancel()

	// Give workers a short time to detect context cancellation before closing channels
	time.Sleep(10 * time.Millisecond)

	// Close input channels to unblock workers
	s.mu.Lock()
	s.closeChannelSafely(s.eventChan)
	if s.batchChan != nil {
		s.closeChannelSafely(s.batchChan)
	}
	s.mu.Unlock()

	// Wait for all workers to finish with timeout protection
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Ignore panics during shutdown
			}
		}()
		s.wg.Wait()
		close(done)
	}()

	// Use a reasonable timeout for worker shutdown
	drainTimeout := s.config.DrainTimeout
	if drainTimeout <= 0 {
		drainTimeout = 5 * time.Second // Fallback timeout
	}

	select {
	case <-done:
		// Clean shutdown - workers finished gracefully
	case <-time.After(drainTimeout):
		// Force shutdown after timeout - this is expected for stuck workers
		// Workers may still be running but we proceed with cleanup
	}

	// Drain the flow controller to prevent any lingering state
	if s.flowController != nil {
		s.flowController.Drain()
	}

	// Now safe to close output channels after workers have stopped or timed out
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close output channels safely
	s.closeChannelSafely(s.outputChan)
	s.closeChannelSafely(s.errorChan)

	return nil
}

// Stop is an alias for Close for compatibility
func (s *EventStream) Stop() error {
	return s.Close()
}

// closeChannelSafely closes a channel safely using reflection-free approach
func (s *EventStream) closeChannelSafely(ch interface{}) {
	defer func() {
		if r := recover(); r != nil {
			// Channel was already closed, ignore panic
		}
	}()

	switch c := ch.(type) {
	case chan events.Event:
		close(c)
	case chan *StreamChunk:
		close(c)
	case chan error:
		close(c)
	case chan *EventBatch:
		close(c)
	}
}

// eventProcessor processes individual events
func (s *EventStream) eventProcessor(workerID int) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash
			if !s.isClosed() {
				s.handleError(fmt.Errorf("worker %d panicked: %v", workerID, r))
			}
		}
	}()

	for {
		select {
		case event, ok := <-s.eventChan:
			if !ok {
				return // Channel closed
			}

			startTime := time.Now()

			if err := s.processEvent(event, workerID); err != nil {
				if !s.isClosed() {
					s.handleError(fmt.Errorf("worker %d: event processing failed: %w", workerID, err))
					if s.metrics != nil {
						atomic.AddUint64(&s.metrics.ProcessingErrors, 1)
					}
				}
			}

			// Release flow control
			s.flowController.Release()

			// Update metrics
			if s.metrics != nil {
				latency := time.Since(startTime).Nanoseconds()
				atomic.AddUint64(&s.metrics.EventsProcessed, 1)

				s.metrics.mu.Lock()
				if latency > s.metrics.MaxLatency {
					s.metrics.MaxLatency = latency
				}

				// Update average latency using exponential moving average
				alpha := 0.1
				s.metrics.AverageLatency = int64(float64(s.metrics.AverageLatency)*(1-alpha) + float64(latency)*alpha)
				s.metrics.mu.Unlock()
			}

		case <-s.ctx.Done():
			// Context cancelled, drain any remaining events in the channel with timeout
			drainCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			for {
				select {
				case <-s.eventChan:
					// Release flow control for drained events
					s.flowController.Release()
				case <-drainCtx.Done():
					// Timeout reached during drain, exit to prevent hanging
					return
				default:
					// No more events to drain
					return
				}
			}

		case <-time.After(100 * time.Millisecond):
			// Periodic check for context cancellation and to prevent blocking
			if s.isClosed() {
				return
			}
		}
	}
}

// processEvent processes a single event
func (s *EventStream) processEvent(event events.Event, workerID int) error {
	// Add sequence number if sequencing is enabled
	var sequencedEvent *SequencedEvent
	if s.config.SequenceEnabled {
		sequencedEvent = s.sequencer.AddEvent(event)
		if sequencedEvent == nil {
			return fmt.Errorf("event rejected by sequencer")
		}
	}

	// Handle batching
	if s.config.BatchEnabled {
		return s.addToBatch(event)
	}

	// Process event directly
	return s.processEventDirect(event, sequencedEvent)
}

// processEventDirect processes an event without batching
func (s *EventStream) processEventDirect(event events.Event, seqEvent *SequencedEvent) error {
	// Serialize event
	data, err := event.ToJSON()
	if err != nil {
		return fmt.Errorf("event serialization failed: %w", err)
	}

	// Apply compression if enabled and size threshold met
	compressed := false
	if s.config.CompressionEnabled && len(data) >= s.config.MinCompressionSize {
		compressedData, err := s.compressData(data)
		if err != nil {
			if s.metrics != nil {
				atomic.AddUint64(&s.metrics.CompressionErrors, 1)
			}
		} else {
			data = compressedData
			compressed = true
			if s.metrics != nil {
				atomic.AddUint64(&s.metrics.EventsCompressed, 1)
				atomic.AddUint64(&s.metrics.BytesSaved, uint64(len(data)-len(compressedData)))
			}
		}
	}

	// Create chunks if data exceeds chunk size
	return s.createChunks(data, event, seqEvent, compressed)
}

// addToBatch adds an event to the current batch
func (s *EventStream) addToBatch(event events.Event) error {
	// Check if stream is closed
	if s.isClosed() {
		return fmt.Errorf("stream is closed")
	}

	// This would typically involve accumulating events in a batch
	// For now, create a single-event batch
	batch := &EventBatch{
		Events:    []events.Event{event},
		Timestamp: time.Now(),
		BatchID:   fmt.Sprintf("batch-%d", time.Now().UnixNano()),
		Size:      1,
	}

	// Acquire read lock to safely access batchChan during the entire operation
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check again if closed after acquiring lock
	if s.isClosed() || s.batchChan == nil {
		return fmt.Errorf("stream is closed")
	}

	select {
	case s.batchChan <- batch:
		return nil
	case <-s.ctx.Done():
		return fmt.Errorf("context cancelled")
	case <-time.After(s.config.BatchTimeout):
		// Check if context is done before timing out
		select {
		case <-s.ctx.Done():
			return fmt.Errorf("context cancelled")
		default:
			return fmt.Errorf("batch timeout exceeded")
		}
	}
}

// batchProcessor processes event batches
func (s *EventStream) batchProcessor() {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash
			if !s.isClosed() {
				s.handleError(fmt.Errorf("batch processor panicked: %v", r))
			}
		}
	}()

	ticker := time.NewTicker(s.config.BatchTimeout)
	defer ticker.Stop()

	var currentBatch *EventBatch

	for {
		select {
		case batch, ok := <-s.batchChan:
			if !ok {
				if currentBatch != nil {
					s.processBatch(currentBatch)
				}
				return
			}

			if currentBatch == nil {
				currentBatch = batch
			} else {
				// Merge batches
				currentBatch.Events = append(currentBatch.Events, batch.Events...)
				currentBatch.Size += batch.Size
			}

			// Process batch if it reaches max size
			if currentBatch.Size >= s.config.BatchSize {
				if err := s.processBatch(currentBatch); err != nil && !s.isClosed() {
					s.handleError(fmt.Errorf("batch processing failed: %w", err))
				}
				currentBatch = nil
			}

		case <-ticker.C:
			if currentBatch != nil && currentBatch.Size > 0 {
				if err := s.processBatch(currentBatch); err != nil && !s.isClosed() {
					s.handleError(fmt.Errorf("batch processing failed: %w", err))
				}
				currentBatch = nil
			}

		case <-s.ctx.Done():
			ticker.Stop()
			if currentBatch != nil {
				// Try to process final batch but don't block on errors
				s.processBatch(currentBatch)
			}
			// Drain remaining batches with timeout to prevent hanging
			drainCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			for {
				select {
				case batch, ok := <-s.batchChan:
					if !ok {
						return
					}
					// Process remaining batches quickly
					if batch != nil {
						s.processBatch(batch)
					}
				case <-drainCtx.Done():
					// Timeout reached during drain, exit to prevent hanging
					return
				default:
					return
				}
			}
		}
	}
}

// processBatch processes a complete batch of events
func (s *EventStream) processBatch(batch *EventBatch) error {
	startTime := time.Now()

	// Serialize batch
	data, err := json.Marshal(batch.Events)
	if err != nil {
		return fmt.Errorf("batch serialization failed: %w", err)
	}

	// Apply compression if enabled
	compressed := false
	if s.config.CompressionEnabled && len(data) >= s.config.MinCompressionSize {
		compressedData, err := s.compressData(data)
		if err != nil {
			if s.metrics != nil {
				atomic.AddUint64(&s.metrics.CompressionErrors, 1)
			}
		} else {
			data = compressedData
			compressed = true
			if s.metrics != nil {
				atomic.AddUint64(&s.metrics.EventsCompressed, uint64(len(batch.Events)))
			}
		}
	}

	// Create batch chunk
	chunk := &StreamChunk{
		Data:        data,
		EventType:   "batch",
		EventID:     batch.BatchID,
		Compressed:  compressed,
		ChunkIndex:  0,
		TotalChunks: 1,
		Timestamp:   batch.Timestamp,
	}

	if s.config.SequenceEnabled {
		chunk.SequenceNum = s.sequencer.GetNextSequence()
	}

	// Send chunk to output with safety checks
	if err := s.sendChunkSafely(chunk); err != nil {
		return err
	}

	if s.metrics != nil {
		atomic.AddUint64(&s.metrics.TotalBatches, 1)
		processingTime := time.Since(startTime).Nanoseconds()
		atomic.StoreInt64(&s.metrics.BatchProcessingTime, processingTime)
	}
	return nil
}

// chunkProcessor handles chunk creation and management
func (s *EventStream) chunkProcessor() {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash
			if !s.isClosed() {
				s.handleError(fmt.Errorf("chunk processor panicked: %v", r))
			}
		}
	}()

	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Flush any pending chunks
			s.chunkBuffer.FlushSafely(s.outputChan, s.ctx)

		case <-s.ctx.Done():
			ticker.Stop()
			// Final flush with context that may be cancelled
			s.chunkBuffer.FlushSafely(s.outputChan, context.Background())
			return
		}
	}
}

// createChunks creates stream chunks from event data
func (s *EventStream) createChunks(data []byte, event events.Event, seqEvent *SequencedEvent, compressed bool) error {
	if len(data) <= s.config.MaxChunkSize {
		// Single chunk
		chunk := &StreamChunk{
			Data:        data,
			EventType:   string(event.Type()),
			Compressed:  compressed,
			ChunkIndex:  0,
			TotalChunks: 1,
			Timestamp:   time.Now(),
		}

		if event.Timestamp() != nil {
			chunk.EventID = fmt.Sprintf("%d", *event.Timestamp())
		}

		if seqEvent != nil {
			chunk.SequenceNum = seqEvent.SequenceNum
		}

		return s.sendChunkSafely(chunk)
	}

	// Multiple chunks needed
	totalChunks := (len(data) + s.config.MaxChunkSize - 1) / s.config.MaxChunkSize
	eventID := fmt.Sprintf("event-%d", time.Now().UnixNano())
	if event.Timestamp() != nil {
		eventID = fmt.Sprintf("%d", *event.Timestamp())
	}

	for i := 0; i < totalChunks; i++ {
		start := i * s.config.MaxChunkSize
		end := start + s.config.MaxChunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := &StreamChunk{
			Data:        data[start:end],
			EventType:   string(event.Type()),
			EventID:     eventID,
			Compressed:  compressed,
			ChunkIndex:  i,
			TotalChunks: totalChunks,
			Timestamp:   time.Now(),
		}

		if seqEvent != nil {
			chunk.SequenceNum = seqEvent.SequenceNum
		}

		if err := s.sendChunkSafely(chunk); err != nil {
			return err
		}
	}

	return nil
}

// sendChunkSafely sends a chunk with proper error handling
func (s *EventStream) sendChunkSafely(chunk *StreamChunk) error {
	select {
	case s.outputChan <- chunk:
		return nil
	case <-s.ctx.Done():
		return fmt.Errorf("context cancelled")
	case <-time.After(5 * time.Second):
		// Prevent deadlock with timeout
		if s.isClosed() {
			return fmt.Errorf("stream closed")
		}
		return fmt.Errorf("timeout sending chunk")
	}
}

// compressData compresses data using the configured compression algorithm
func (s *EventStream) compressData(data []byte) ([]byte, error) {
	startTime := time.Now()

	var buf bytes.Buffer
	var writer io.WriteCloser
	var err error

	switch s.compression {
	case CompressionGzip:
		if s.config.CompressionLevel > 0 {
			writer, err = gzip.NewWriterLevel(&buf, s.config.CompressionLevel)
		} else {
			writer = gzip.NewWriter(&buf)
		}
	case CompressionDeflate:
		if s.config.CompressionLevel > 0 {
			writer, err = flate.NewWriter(&buf, s.config.CompressionLevel)
		} else {
			writer, err = flate.NewWriter(&buf, flate.DefaultCompression)
		}
	default:
		return data, fmt.Errorf("unsupported compression type: %s", s.compression)
	}

	if err != nil {
		return data, fmt.Errorf("compression writer creation failed: %w", err)
	}

	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return data, fmt.Errorf("compression write failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return data, fmt.Errorf("compression close failed: %w", err)
	}

	if s.metrics != nil {
		compressionTime := time.Since(startTime).Nanoseconds()
		atomic.StoreInt64(&s.metrics.CompressionTime, compressionTime)

		s.metrics.mu.Lock()
		if len(data) > 0 {
			s.metrics.CompressionRatio = float64(buf.Len()) / float64(len(data))
		}
		s.metrics.mu.Unlock()
	}

	return buf.Bytes(), nil
}

// metricsCollector periodically collects and updates metrics
func (s *EventStream) metricsCollector() {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash
			if !s.isClosed() {
				s.handleError(fmt.Errorf("metrics collector panicked: %v", r))
			}
		}
	}()

	ticker := time.NewTicker(s.config.MetricsInterval)
	defer ticker.Stop()

	lastEventCount := uint64(0)
	lastTime := time.Now()

	for {
		select {
		case <-ticker.C:
			if s.metrics != nil && !s.isClosed() {
				s.metrics.mu.Lock()

				// Calculate events per second
				currentEvents := atomic.LoadUint64(&s.metrics.TotalEvents)
				currentTime := time.Now()

				if elapsed := currentTime.Sub(lastTime).Seconds(); elapsed > 0 {
					s.metrics.EventsPerSecond = float64(currentEvents-lastEventCount) / elapsed
				}

				lastEventCount = currentEvents
				lastTime = currentTime

				// Update flow control metrics
				if s.flowController != nil && s.flowController.metrics != nil {
					s.metrics.FlowControl = &FlowMetrics{
						EventsProcessed:    s.flowController.metrics.EventsProcessed,
						EventsDropped:      s.flowController.metrics.EventsDropped,
						BackpressureEvents: s.flowController.metrics.BackpressureEvents,
						AverageWaitTime:    atomic.LoadInt64(&s.flowController.metrics.AverageWaitTime),
						MaxWaitTime:        atomic.LoadInt64(&s.flowController.metrics.MaxWaitTime),
						CurrentConcurrent:  atomic.LoadInt32(&s.flowController.metrics.CurrentConcurrent),
					}
				}

				// Update sequencing metrics
				if s.sequencer != nil && s.sequencer.metrics != nil {
					s.metrics.Sequencing = s.sequencer.GetMetrics()
				}

				s.metrics.mu.Unlock()
			}

		case <-s.ctx.Done():
			ticker.Stop()
			return
		}
	}
}

// handleError handles stream errors safely
func (s *EventStream) handleError(err error) {
	if s.isClosed() {
		return // Don't send errors after close
	}

	select {
	case s.errorChan <- err:
	default:
		// Error channel full, drop error to prevent blocking
	}
}

// isStarted checks if the stream has been started
func (s *EventStream) isStarted() bool {
	return atomic.LoadInt32(&s.started) == 1
}

// isClosed checks if the stream has been closed
func (s *EventStream) isClosed() bool {
	return atomic.LoadInt32(&s.closed) == 1
}

// validateStreamConfig validates the stream configuration
func validateStreamConfig(config *StreamConfig) error {
	if config.EventBufferSize <= 0 {
		return fmt.Errorf("event buffer size must be positive")
	}

	if config.ChunkBufferSize <= 0 {
		return fmt.Errorf("chunk buffer size must be positive")
	}

	if config.MaxChunkSize <= 0 {
		return fmt.Errorf("max chunk size must be positive")
	}

	if config.BatchEnabled {
		if config.BatchSize <= 0 {
			return fmt.Errorf("batch size must be positive")
		}

		if config.MaxBatchSize <= 0 {
			return fmt.Errorf("max batch size must be positive")
		}

		if config.BatchSize > config.MaxBatchSize {
			return fmt.Errorf("batch size cannot exceed max batch size")
		}

		if config.BatchTimeout <= 0 {
			return fmt.Errorf("batch timeout must be positive")
		}
	}

	if config.MaxConcurrentEvents <= 0 {
		return fmt.Errorf("max concurrent events must be positive")
	}

	if config.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be positive")
	}

	if config.CompressionEnabled {
		switch config.CompressionType {
		case CompressionGzip, CompressionDeflate:
			// Valid compression types
		case CompressionNone:
			config.CompressionEnabled = false
		default:
			return fmt.Errorf("invalid compression type: %s", config.CompressionType)
		}

		if config.CompressionLevel < 0 || config.CompressionLevel > 9 {
			return fmt.Errorf("compression level must be between 0 and 9")
		}
	}

	return nil
}

// NewFlowController creates a new flow controller
func NewFlowController(maxConcurrent int, timeout, drainTimeout time.Duration) *FlowController {
	fc := &FlowController{
		maxConcurrent:  int32(maxConcurrent),
		backpressureCh: make(chan struct{}, maxConcurrent),
		timeout:        timeout,
		drainTimeout:   drainTimeout,
		metrics:        &FlowMetrics{},
	}
	return fc
}

// Drain safely drains the flow controller during shutdown
func (fc *FlowController) Drain() {
	// Drain the backpressure channel with timeout to prevent hanging
	drainCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	for {
		select {
		case <-fc.backpressureCh:
			// Drain slot from channel
			atomic.AddInt32(&fc.current, -1)
		case <-drainCtx.Done():
			// Timeout reached, force reset the counter
			atomic.StoreInt32(&fc.current, 0)
			atomic.StoreInt32(&fc.metrics.CurrentConcurrent, 0)
			return
		default:
			// No more slots to drain
			return
		}
	}
}

// Acquire acquires a flow control slot
func (fc *FlowController) Acquire(ctx context.Context) error {
	start := time.Now()

	select {
	case fc.backpressureCh <- struct{}{}:
		current := atomic.AddInt32(&fc.current, 1)
		atomic.StoreInt32(&fc.metrics.CurrentConcurrent, current)

		waitTime := time.Since(start).Nanoseconds()
		atomic.AddInt64(&fc.metrics.AverageWaitTime, waitTime)

		if waitTime > atomic.LoadInt64(&fc.metrics.MaxWaitTime) {
			atomic.StoreInt64(&fc.metrics.MaxWaitTime, waitTime)
		}

		return nil
	case <-ctx.Done():
		atomic.AddUint64(&fc.metrics.EventsDropped, 1)
		return ctx.Err()
	case <-time.After(fc.timeout):
		// Check if context is done before timing out
		select {
		case <-ctx.Done():
			atomic.AddUint64(&fc.metrics.EventsDropped, 1)
			return ctx.Err()
		default:
			atomic.AddUint64(&fc.metrics.BackpressureEvents, 1)
			return fmt.Errorf("flow control timeout")
		}
	}
}

// Release releases a flow control slot
func (fc *FlowController) Release() {
	// Use atomic operations to ensure we only release if we actually have something to release
	current := atomic.LoadInt32(&fc.current)
	if current <= 0 {
		return // Nothing to release
	}

	select {
	case <-fc.backpressureCh:
		newCurrent := atomic.AddInt32(&fc.current, -1)
		atomic.StoreInt32(&fc.metrics.CurrentConcurrent, newCurrent)
		atomic.AddUint64(&fc.metrics.EventsProcessed, 1)
	default:
		// Channel is empty or closed, but we still need to decrement the counter
		// This can happen during shutdown when the channel is drained
		if atomic.CompareAndSwapInt32(&fc.current, current, current-1) {
			atomic.StoreInt32(&fc.metrics.CurrentConcurrent, current-1)
			atomic.AddUint64(&fc.metrics.EventsProcessed, 1)
		}
	}
}

// Run runs the flow controller
func (fc *FlowController) Run(ctx context.Context) {
	// FlowController doesn't need a background goroutine,
	// it's just used for synchronization
	// This method exists for interface compatibility
	// Just wait for context cancellation and return
	select {
	case <-ctx.Done():
		return
	}
}

// NewEventSequencer creates a new event sequencer
func NewEventSequencer(enabled, orderingRequired bool, bufferSize int) *EventSequencer {
	return &EventSequencer{
		enabled:          enabled,
		orderingRequired: orderingRequired,
		sequenceBuffer:   make(map[uint64]*SequencedEvent),
		bufferSize:       bufferSize,
		timeout:          5 * time.Second,
		outputChan:       make(chan *SequencedEvent, bufferSize),
		metrics:          &SequenceMetrics{},
	}
}

// AddEvent adds an event to the sequencer
func (es *EventSequencer) AddEvent(event events.Event) *SequencedEvent {
	if !es.enabled {
		return &SequencedEvent{
			Event:       event,
			SequenceNum: 0,
			Timestamp:   time.Now(),
		}
	}

	es.mu.Lock()
	defer es.mu.Unlock()

	sequenceNum := atomic.AddUint64(&es.nextSequence, 1)
	seqEvent := &SequencedEvent{
		Event:       event,
		SequenceNum: sequenceNum,
		Timestamp:   time.Now(),
	}

	if es.orderingRequired {
		es.sequenceBuffer[sequenceNum] = seqEvent
		atomic.AddUint64(&es.metrics.EventsSequenced, 1)
		return nil // Will be processed in order by Run()
	}

	atomic.AddUint64(&es.metrics.EventsSequenced, 1)
	return seqEvent
}

// GetNextSequence returns the next sequence number
func (es *EventSequencer) GetNextSequence() uint64 {
	return atomic.AddUint64(&es.nextSequence, 1)
}

// GetMetrics returns sequencing metrics
func (es *EventSequencer) GetMetrics() *SequenceMetrics {
	es.mu.RLock()
	defer es.mu.RUnlock()

	metrics := *es.metrics
	metrics.BufferUtilization = float64(len(es.sequenceBuffer)) / float64(es.bufferSize)
	return &metrics
}

// Run runs the event sequencer
func (es *EventSequencer) Run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash
		}
	}()

	if !es.orderingRequired {
		// If ordering is not required, just wait for context cancellation
		select {
		case <-ctx.Done():
			return
		}
	}

	ticker := time.NewTicker(es.timeout)
	defer ticker.Stop()

	expectedSeq := uint64(1)

	for {
		select {
		case <-ticker.C:
			es.mu.Lock()

			// Process events in order
			for {
				if seqEvent, exists := es.sequenceBuffer[expectedSeq]; exists {
					delete(es.sequenceBuffer, expectedSeq)

					select {
					case es.outputChan <- seqEvent:
						expectedSeq++
					case <-ctx.Done():
						// Context cancelled, put back and exit
						es.sequenceBuffer[expectedSeq] = seqEvent
						es.mu.Unlock()
						ticker.Stop()
						return
					default:
						// Output channel full, put back and try later
						es.sequenceBuffer[expectedSeq] = seqEvent
						break
					}
				} else {
					break
				}
			}

			// Clean up old events
			for seq, seqEvent := range es.sequenceBuffer {
				if time.Since(seqEvent.Timestamp) > es.timeout {
					delete(es.sequenceBuffer, seq)
					atomic.AddUint64(&es.metrics.DroppedEvents, 1)
				}
			}

			es.mu.Unlock()

		case <-ctx.Done():
			ticker.Stop()
			// Clean up on shutdown
			es.mu.Lock()
			for seq := range es.sequenceBuffer {
				delete(es.sequenceBuffer, seq)
			}
			es.mu.Unlock()
			return
		}
	}
}

// NewBufferPool creates a new buffer pool
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		size: size,
		pool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, size))
			},
		},
	}
}

// Get gets a buffer from the pool
func (bp *BufferPool) Get() *bytes.Buffer {
	return bp.pool.Get().(*bytes.Buffer)
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(buf *bytes.Buffer) {
	buf.Reset()
	bp.pool.Put(buf)
}

// NewChunkBuffer creates a new chunk buffer
func NewChunkBuffer(maxChunkSize int) *ChunkBuffer {
	return &ChunkBuffer{
		maxChunkSize: maxChunkSize,
		buffer:       bytes.NewBuffer(make([]byte, 0, maxChunkSize)),
		chunks:       make([]*StreamChunk, 0),
	}
}

// AddChunk adds a chunk to the buffer
func (cb *ChunkBuffer) AddChunk(chunk *StreamChunk) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.chunks = append(cb.chunks, chunk)
}

// Flush flushes pending chunks to the output channel
func (cb *ChunkBuffer) Flush(outputChan chan<- *StreamChunk) {
	cb.FlushSafely(outputChan, context.Background())
}

// FlushSafely flushes pending chunks with context cancellation support
func (cb *ChunkBuffer) FlushSafely(outputChan chan<- *StreamChunk, ctx context.Context) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	for _, chunk := range cb.chunks {
		select {
		case outputChan <- chunk:
			// Successfully sent
		case <-ctx.Done():
			// Context cancelled, stop flushing
			return
		default:
			// Output channel full, skip this chunk
		}
	}

	cb.chunks = cb.chunks[:0] // Reset slice
}

// NewStreamMetrics creates a new stream metrics instance
func NewStreamMetrics() *StreamMetrics {
	return &StreamMetrics{
		StartTime: time.Now(),
	}
}

// FormatSSEChunk formats a stream chunk as SSE data
func FormatSSEChunk(chunk *StreamChunk) (string, error) {
	if chunk == nil {
		return "", fmt.Errorf("chunk cannot be nil")
	}

	var sse strings.Builder

	// Event type
	if chunk.EventType != "" {
		sse.WriteString(fmt.Sprintf("event: %s\n", chunk.EventType))
	}

	// Event ID
	if chunk.EventID != "" {
		eventID := chunk.EventID
		if chunk.TotalChunks > 1 {
			eventID = fmt.Sprintf("%s-%d", chunk.EventID, chunk.ChunkIndex)
		}
		sse.WriteString(fmt.Sprintf("id: %s\n", eventID))
	}

	// Retry
	if chunk.Retry != nil {
		sse.WriteString(fmt.Sprintf("retry: %d\n", *chunk.Retry))
	}

	// Data
	if chunk.Compressed {
		// For compressed data, we need to base64 encode
		encodedData := base64.StdEncoding.EncodeToString(chunk.Data)
		sse.WriteString(fmt.Sprintf("data: {\"compressed\":true,\"data\":\"%s\"}\n", encodedData))
	} else {
		sse.WriteString(fmt.Sprintf("data: %s\n", string(chunk.Data)))
	}

	// Chunking metadata
	if chunk.TotalChunks > 1 {
		metadata := map[string]interface{}{
			"chunk_index":  chunk.ChunkIndex,
			"total_chunks": chunk.TotalChunks,
			"sequence_num": chunk.SequenceNum,
		}
		metadataBytes, _ := json.Marshal(metadata)
		sse.WriteString(fmt.Sprintf("data: %s\n", string(metadataBytes)))
	}

	sse.WriteString("\n")

	return sse.String(), nil
}

// WriteSSEChunk writes a stream chunk as SSE to a writer
func WriteSSEChunk(w io.Writer, chunk *StreamChunk) error {
	sseData, err := FormatSSEChunk(chunk)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(sseData))
	return err
}
