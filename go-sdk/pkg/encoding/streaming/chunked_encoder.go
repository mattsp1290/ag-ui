package streaming

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

// ChunkConfig holds configuration for chunked encoding
type ChunkConfig struct {
	// MaxChunkSize is the maximum size of a chunk in bytes
	MaxChunkSize int

	// MaxEventsPerChunk is the maximum number of events per chunk
	MaxEventsPerChunk int

	// CompressionThreshold triggers compression for chunks above this size
	CompressionThreshold int

	// EnableParallelProcessing allows parallel chunk processing
	EnableParallelProcessing bool

	// ProcessorCount is the number of parallel processors
	ProcessorCount int
}

// DefaultChunkConfig returns default chunk configuration
func DefaultChunkConfig() *ChunkConfig {
	return &ChunkConfig{
		MaxChunkSize:             1024 * 1024, // 1MB
		MaxEventsPerChunk:        1000,
		CompressionThreshold:     10 * 1024, // 10KB
		EnableParallelProcessing: true,
		ProcessorCount:           4,
	}
}

// ChunkHeader contains metadata about a chunk
type ChunkHeader struct {
	ChunkID      string
	SequenceNum  int64
	EventCount   int
	ByteSize     int
	Compressed   bool
	Checksum     uint32
}

// Chunk represents a chunk of events
type Chunk struct {
	Header ChunkHeader
	Events []events.Event
	Data   []byte
}

// ChunkedEncoder breaks large event sequences into manageable chunks
type ChunkedEncoder struct {
	config         *ChunkConfig
	baseEncoder    encoding.Encoder
	chunkSequence  atomic.Int64
	processedCount atomic.Int64
	totalBytes     atomic.Int64
	
	// Progress tracking
	progressCallbacks []func(processed, total int64)
	progressMu        sync.RWMutex
	
	// Chunk processing
	chunkPool sync.Pool
	workers   sync.WaitGroup
	
	// Metrics
	metrics *ChunkMetrics
}

// ChunkMetrics tracks chunking metrics
type ChunkMetrics struct {
	ChunksCreated   atomic.Int64
	EventsProcessed atomic.Int64
	BytesProcessed  atomic.Int64
	CompressionRate atomic.Uint64 // stored as percentage * 100
}

// NewChunkedEncoder creates a new chunked encoder
func NewChunkedEncoder(baseEncoder encoding.Encoder, config *ChunkConfig) *ChunkedEncoder {
	if config == nil {
		config = DefaultChunkConfig()
	}

	return &ChunkedEncoder{
		config:      config,
		baseEncoder: baseEncoder,
		metrics:     &ChunkMetrics{},
		chunkPool: sync.Pool{
			New: func() interface{} {
				return &Chunk{
					Events: make([]events.Event, 0, config.MaxEventsPerChunk),
				}
			},
		},
	}
}

// EncodeChunked encodes a stream of events into chunks
func (ce *ChunkedEncoder) EncodeChunked(ctx context.Context, input <-chan events.Event, output chan<- *Chunk) error {
	defer close(output)

	if ce.config.EnableParallelProcessing {
		return ce.encodeParallel(ctx, input, output)
	}
	return ce.encodeSequential(ctx, input, output)
}

// encodeSequential processes events sequentially
func (ce *ChunkedEncoder) encodeSequential(ctx context.Context, input <-chan events.Event, output chan<- *Chunk) error {
	currentChunk := ce.getChunk()
	currentSize := 0

	flushChunk := func() error {
		if len(currentChunk.Events) == 0 {
			return nil
		}

		// Encode chunk
		encoded, err := ce.encodeChunk(currentChunk)
		if err != nil {
			return err
		}

		// Send chunk
		select {
		case output <- encoded:
			ce.updateProgress()
		case <-ctx.Done():
			return ctx.Err()
		}

		// Reset for next chunk
		ce.putChunk(currentChunk)
		currentChunk = ce.getChunk()
		currentSize = 0
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-input:
			if !ok {
				// Input closed, flush final chunk
				return flushChunk()
			}

			// Estimate event size
			eventSize := ce.estimateEventSize(event)

			// Check if we need to flush
			if ce.shouldFlush(currentChunk, currentSize, eventSize) {
				if err := flushChunk(); err != nil {
					return err
				}
			}

			// Add event to chunk
			currentChunk.Events = append(currentChunk.Events, event)
			currentSize += eventSize
			ce.processedCount.Add(1)
		}
	}
}

// encodeParallel processes events in parallel
func (ce *ChunkedEncoder) encodeParallel(ctx context.Context, input <-chan events.Event, output chan<- *Chunk) error {
	// Create worker pool
	workerInput := make(chan *Chunk, ce.config.ProcessorCount)
	workerOutput := make(chan *Chunk, ce.config.ProcessorCount)

	// Start workers
	for i := 0; i < ce.config.ProcessorCount; i++ {
		ce.workers.Add(1)
		go ce.chunkWorker(ctx, workerInput, workerOutput)
	}

	// Start output handler
	outputDone := make(chan struct{})
	go func() {
		defer close(outputDone)
		for chunk := range workerOutput {
			select {
			case output <- chunk:
				ce.updateProgress()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Process input
	currentChunk := ce.getChunk()
	currentSize := 0

	for {
		select {
		case <-ctx.Done():
			close(workerInput)
			ce.workers.Wait()
			close(workerOutput)
			<-outputDone
			return ctx.Err()

		case event, ok := <-input:
			if !ok {
				// Send final chunk if needed
				if len(currentChunk.Events) > 0 {
					select {
					case workerInput <- currentChunk:
					case <-ctx.Done():
						close(workerInput)
						ce.workers.Wait()
						close(workerOutput)
						<-outputDone
						return ctx.Err()
					}
				}
				close(workerInput)
				ce.workers.Wait()
				close(workerOutput)
				<-outputDone
				return nil
			}

			eventSize := ce.estimateEventSize(event)

			if ce.shouldFlush(currentChunk, currentSize, eventSize) {
				select {
				case workerInput <- currentChunk:
					currentChunk = ce.getChunk()
					currentSize = 0
				case <-ctx.Done():
					close(workerInput)
					ce.workers.Wait()
					close(workerOutput)
					<-outputDone
					return ctx.Err()
				}
			}

			currentChunk.Events = append(currentChunk.Events, event)
			currentSize += eventSize
			ce.processedCount.Add(1)
		}
	}
}

// chunkWorker processes chunks
func (ce *ChunkedEncoder) chunkWorker(ctx context.Context, input <-chan *Chunk, output chan<- *Chunk) {
	defer ce.workers.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-input:
			if !ok {
				return
			}

			encoded, err := ce.encodeChunk(chunk)
			if err != nil {
				// Log error and continue
				continue
			}

			select {
			case output <- encoded:
			case <-ctx.Done():
				return
			}
		}
	}
}

// encodeChunk encodes a single chunk
func (ce *ChunkedEncoder) encodeChunk(chunk *Chunk) (*Chunk, error) {
	// Encode events
	data, err := ce.baseEncoder.EncodeMultiple(chunk.Events)
	if err != nil {
		return nil, fmt.Errorf("failed to encode chunk: %w", err)
	}

	// Update header
	chunk.Header = ChunkHeader{
		ChunkID:     fmt.Sprintf("chunk-%d", ce.chunkSequence.Add(1)),
		SequenceNum: ce.chunkSequence.Load(),
		EventCount:  len(chunk.Events),
		ByteSize:    len(data),
		Compressed:  false,
	}

	// Check if compression is needed
	if len(data) > ce.config.CompressionThreshold {
		compressed, err := ce.compressData(data)
		if err == nil && len(compressed) < len(data) {
			chunk.Header.Compressed = true
			compressionRate := uint64((1 - float64(len(compressed))/float64(len(data))) * 10000)
			ce.metrics.CompressionRate.Store(compressionRate)
			data = compressed
		}
	}

	chunk.Data = data
	chunk.Header.Checksum = ce.calculateChecksum(data)

	// Update metrics
	ce.metrics.ChunksCreated.Add(1)
	ce.metrics.EventsProcessed.Add(int64(len(chunk.Events)))
	ce.metrics.BytesProcessed.Add(int64(len(data)))
	ce.totalBytes.Add(int64(len(data)))

	return chunk, nil
}

// shouldFlush determines if a chunk should be flushed
func (ce *ChunkedEncoder) shouldFlush(chunk *Chunk, currentSize, nextEventSize int) bool {
	// Check event count
	if len(chunk.Events) >= ce.config.MaxEventsPerChunk {
		return true
	}

	// Check size limit
	if currentSize+nextEventSize > ce.config.MaxChunkSize {
		return true
	}

	return false
}

// estimateEventSize estimates the size of an event
func (ce *ChunkedEncoder) estimateEventSize(event events.Event) int {
	// Quick estimation based on event type
	// This is a heuristic and can be improved
	switch event.Type() {
	case events.EventTypeTextMessageStart, events.EventTypeTextMessageContent, events.EventTypeTextMessageEnd:
		return 1024 // 1KB average for message events
	case events.EventTypeToolCallStart, events.EventTypeToolCallArgs, events.EventTypeToolCallEnd:
		return 2048 // 2KB average for tool events
	case events.EventTypeStateSnapshot, events.EventTypeStateDelta:
		return 512 // 512B average for state events
	default:
		return 256 // Default estimate
	}
}

// compressData compresses data (placeholder implementation)
func (ce *ChunkedEncoder) compressData(data []byte) ([]byte, error) {
	// TODO: Implement actual compression (gzip, zstd, etc.)
	return data, nil
}

// calculateChecksum calculates checksum for data
func (ce *ChunkedEncoder) calculateChecksum(data []byte) uint32 {
	// Simple checksum implementation
	var sum uint32
	for _, b := range data {
		sum += uint32(b)
	}
	return sum
}

// getChunk gets a chunk from the pool
func (ce *ChunkedEncoder) getChunk() *Chunk {
	chunk := ce.chunkPool.Get().(*Chunk)
	chunk.Events = chunk.Events[:0]
	chunk.Data = nil
	return chunk
}

// putChunk returns a chunk to the pool
func (ce *ChunkedEncoder) putChunk(chunk *Chunk) {
	if cap(chunk.Events) > ce.config.MaxEventsPerChunk*2 {
		// Don't pool oversized chunks
		return
	}
	ce.chunkPool.Put(chunk)
}

// RegisterProgressCallback registers a progress callback
func (ce *ChunkedEncoder) RegisterProgressCallback(callback func(processed, total int64)) {
	ce.progressMu.Lock()
	defer ce.progressMu.Unlock()
	ce.progressCallbacks = append(ce.progressCallbacks, callback)
}

// updateProgress updates progress
func (ce *ChunkedEncoder) updateProgress() {
	processed := ce.processedCount.Load()
	total := ce.totalBytes.Load()

	ce.progressMu.RLock()
	callbacks := ce.progressCallbacks
	ce.progressMu.RUnlock()

	for _, cb := range callbacks {
		cb(processed, total)
	}
}

// GetMetrics returns current metrics
func (ce *ChunkedEncoder) GetMetrics() ChunkMetrics {
	return ChunkMetrics{
		ChunksCreated:   atomic.Int64{},
		EventsProcessed: atomic.Int64{},
		BytesProcessed:  atomic.Int64{},
		CompressionRate: atomic.Uint64{},
	}
}

// Reset resets the encoder state
func (ce *ChunkedEncoder) Reset() {
	ce.chunkSequence.Store(0)
	ce.processedCount.Store(0)
	ce.totalBytes.Store(0)
	ce.metrics = &ChunkMetrics{}
}