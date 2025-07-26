package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/internal/timeconfig"
)

// PerformanceConfig contains configuration for performance optimizations
type PerformanceConfig struct {
	// MaxConcurrentConnections is the maximum number of concurrent connections
	MaxConcurrentConnections int

	// MessageBatchSize is the number of messages to batch together
	MessageBatchSize int

	// MessageBatchTimeout is the timeout for message batching
	MessageBatchTimeout time.Duration

	// BufferPoolSize is the size of the buffer pool
	BufferPoolSize int

	// MaxBufferSize is the maximum size of individual buffers
	MaxBufferSize int

	// EnableZeroCopy enables zero-copy operations where possible
	EnableZeroCopy bool

	// EnableMemoryPooling enables memory pool management
	EnableMemoryPooling bool

	// EnableProfiling enables CPU and memory profiling
	EnableProfiling bool

	// ProfilingInterval is the interval for profiling snapshots
	ProfilingInterval time.Duration

	// MaxLatency is the maximum acceptable latency
	MaxLatency time.Duration

	// MaxMemoryUsage is the maximum memory usage for the given connection count
	MaxMemoryUsage int64

	// EnableMetrics enables performance metrics collection
	EnableMetrics bool

	// MetricsInterval is the interval for metrics collection
	MetricsInterval time.Duration

	// MessageSerializerType defines the serialization method to use
	MessageSerializerType SerializerType

	// Logger is the logger instance
	Logger *zap.Logger
}

// SerializerType defines different serialization methods
type SerializerType int

const (
	// JSONSerializer uses standard JSON serialization
	JSONSerializer SerializerType = iota
	// OptimizedJSONSerializer uses optimized JSON serialization
	OptimizedJSONSerializer
	// ProtobufSerializer uses Protocol Buffers serialization
	ProtobufSerializer
)

// String returns the string representation of the serializer type
func (s SerializerType) String() string {
	switch s {
	case JSONSerializer:
		return "json"
	case OptimizedJSONSerializer:
		return "optimized_json"
	case ProtobufSerializer:
		return "protobuf"
	default:
		return "unknown"
	}
}

// DefaultPerformanceConfig returns a default performance configuration
// Uses configurable timeouts that adapt to test/production environments
func DefaultPerformanceConfig() *PerformanceConfig {
	config := timeconfig.GetConfig()
	return &PerformanceConfig{
		MaxConcurrentConnections: 1000,
		MessageBatchSize:         10,
		MessageBatchTimeout:      config.DefaultMessageBatchTimeout,
		BufferPoolSize:           1000,
		MaxBufferSize:            64 * 1024, // 64KB
		EnableZeroCopy:           true,
		EnableMemoryPooling:      true,
		EnableProfiling:          false,
		ProfilingInterval:        config.DefaultProfilingInterval,
		MaxLatency:               config.DefaultMaxLatency,
		MaxMemoryUsage:           80 * 1024 * 1024, // 80MB
		EnableMetrics:            true,
		MetricsInterval:          config.DefaultMetricsInterval,
		MessageSerializerType:    OptimizedJSONSerializer,
		Logger:                   zap.NewNop(),
	}
}

// HighConcurrencyPerformanceConfig returns a performance configuration optimized for high concurrency testing
// Uses configurable timeouts that adapt to test/production environments
func HighConcurrencyPerformanceConfig() *PerformanceConfig {
	config := timeconfig.GetConfig()
	return &PerformanceConfig{
		MaxConcurrentConnections: 50000,     // Much higher concurrency limit
		MessageBatchSize:         100,       // Larger batches for better throughput  
		MessageBatchTimeout:      config.DefaultMessageBatchTimeout, // Use configurable timeout
		BufferPoolSize:           10000,     // More buffers for high concurrency
		MaxBufferSize:            64 * 1024, // 64KB
		EnableZeroCopy:           true,
		EnableMemoryPooling:      true,
		EnableProfiling:          false,
		ProfilingInterval:        config.DefaultProfilingInterval,
		MaxLatency:               config.DefaultMaxLatency, // Use configurable latency
		MaxMemoryUsage:           500 * 1024 * 1024, // 500MB for high concurrency
		EnableMetrics:            false,     // Disable metrics to reduce overhead
		MetricsInterval:          config.DefaultMetricsInterval,
		MessageSerializerType:    OptimizedJSONSerializer,
		Logger:                   zap.NewNop(),
	}
}

// PerformanceManager manages performance optimizations for WebSocket transport
type PerformanceManager struct {
	config *PerformanceConfig

	// Buffer pool for message handling
	bufferPool *BufferPool

	// Message batcher for batching outgoing messages
	messageBatcher *MessageBatcher

	// Connection pool manager
	connectionPoolManager *ConnectionPoolManager

	// Serializer factory
	serializerFactory *SerializerFactory

	// Metrics collector
	metricsCollector *MetricsCollector

	// Profiler for performance analysis
	profiler *Profiler

	// Memory manager for efficient memory usage
	memoryManager *MemoryManager

	// Rate limiter for connection management
	rateLimiter *rate.Limiter

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPerformanceManager creates a new performance manager
func NewPerformanceManager(config *PerformanceConfig) (*PerformanceManager, error) {
	if config == nil {
		config = DefaultPerformanceConfig()
	}

	pm := &PerformanceManager{
		config: config,
	}

	// Initialize buffer pool
	pm.bufferPool = NewBufferPool(config.BufferPoolSize, config.MaxBufferSize)

	// Initialize message batcher
	pm.messageBatcher = NewMessageBatcher(config.MessageBatchSize, config.MessageBatchTimeout)

	// Initialize connection pool manager
	pm.connectionPoolManager = NewConnectionPoolManager(config.MaxConcurrentConnections)

	// Initialize serializer factory
	pm.serializerFactory = NewSerializerFactory(config.MessageSerializerType)

	// Initialize metrics collector
	if config.EnableMetrics {
		pm.metricsCollector = NewMetricsCollector(config.MetricsInterval)
	}

	// Initialize profiler
	if config.EnableProfiling {
		pm.profiler = NewProfiler(config.ProfilingInterval)
	}

	// Initialize memory manager
	if config.EnableMemoryPooling {
		pm.memoryManager = NewMemoryManager(config.MaxMemoryUsage)
	}

	// Initialize rate limiter
	pm.rateLimiter = rate.NewLimiter(rate.Limit(config.MaxConcurrentConnections), config.MaxConcurrentConnections)

	return pm, nil
}

// Start initializes the performance manager
func (pm *PerformanceManager) Start(ctx context.Context) error {
	pm.config.Logger.Info("Starting performance manager")

	// Create a derived context that we can cancel
	pm.ctx, pm.cancel = context.WithCancel(ctx)

	// Use internal context for all goroutines so they can be properly cancelled in Stop()
	// Start message batcher
	pm.wg.Add(1)
	go pm.messageBatcher.Start(pm.ctx, &pm.wg)

	// Start metrics collector with internal context
	if pm.metricsCollector != nil {
		pm.wg.Add(1)
		go pm.metricsCollector.Start(pm.ctx, &pm.wg)
	}

	// Start profiler with internal context
	if pm.profiler != nil {
		pm.wg.Add(1)
		go pm.profiler.Start(pm.ctx, &pm.wg)
	}

	// Start memory manager with internal context
	if pm.memoryManager != nil {
		pm.wg.Add(1)
		go pm.memoryManager.Start(pm.ctx, &pm.wg)
	}

	pm.config.Logger.Info("Performance manager started")
	return nil
}

// Stop gracefully shuts down the performance manager
func (pm *PerformanceManager) Stop() error {
	pm.config.Logger.Info("Stopping performance manager")

	// Cancel context first
	pm.cancel()

	// Close message batcher to unblock any goroutines waiting on channels
	if pm.messageBatcher != nil {
		pm.messageBatcher.Close()
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		pm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished successfully
		pm.config.Logger.Info("Performance manager stopped")
		return nil
	case <-time.After(200 * time.Millisecond):
		// Reduced timeout for faster test completion while preserving reliability
		pm.config.Logger.Warn("Performance manager stop timeout")
		return fmt.Errorf("timeout waiting for performance manager to stop")
	}
}

// OptimizeMessage optimizes a message for transmission
func (pm *PerformanceManager) OptimizeMessage(event events.Event) ([]byte, error) {
	start := time.Now()

	// Get serializer
	serializer := pm.serializerFactory.GetSerializer()

	// Serialize message
	data, err := serializer.Serialize(event)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize message: %w", err)
	}

	// Track serialization time
	if pm.metricsCollector != nil {
		pm.metricsCollector.TrackSerializationTime(time.Since(start))
	}

	return data, nil
}

// BatchMessage adds a message to the batch for optimized transmission
func (pm *PerformanceManager) BatchMessage(data []byte) error {
	if !pm.rateLimiter.Allow() {
		return fmt.Errorf("websocket performance manager: rate limit exceeded for message batching")
	}

	return pm.messageBatcher.AddMessage(data)
}

// GetBuffer gets a buffer from the pool
func (pm *PerformanceManager) GetBuffer() []byte {
	return pm.bufferPool.Get()
}

// PutBuffer returns a buffer to the pool
func (pm *PerformanceManager) PutBuffer(buf []byte) {
	pm.bufferPool.Put(buf)
}

// GetConnectionSlot acquires a connection slot
func (pm *PerformanceManager) GetConnectionSlot(ctx context.Context) (*ConnectionSlot, error) {
	return pm.connectionPoolManager.AcquireSlot(ctx)
}

// ReleaseConnectionSlot releases a connection slot
func (pm *PerformanceManager) ReleaseConnectionSlot(slot *ConnectionSlot) {
	pm.connectionPoolManager.ReleaseSlot(slot)
}

// GetMetrics returns performance metrics
func (pm *PerformanceManager) GetMetrics() *PerformanceMetrics {
	if pm.metricsCollector == nil {
		return nil
	}
	return pm.metricsCollector.GetMetrics()
}

// GetMemoryUsage returns current memory usage
func (pm *PerformanceManager) GetMemoryUsage() int64 {
	if pm.memoryManager == nil {
		return 0
	}
	return pm.memoryManager.GetCurrentUsage()
}

// BufferPool manages a pool of reusable buffers
type BufferPool struct {
	pool    sync.Pool
	maxSize int
	stats   struct {
		// Cache line padded to prevent false sharing
		gets     int64
		_        [56]byte // Cache line padding
		puts     int64
		_        [56]byte // Cache line padding
		creates  int64
		_        [56]byte // Cache line padding
		maxUsage int64
		_        [56]byte // Cache line padding
	}
}

// NewBufferPool creates a new buffer pool
func NewBufferPool(poolSize, maxSize int) *BufferPool {
	bp := &BufferPool{
		maxSize: maxSize,
	}

	bp.pool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&bp.stats.creates, 1)
			return make([]byte, 0, maxSize)
		},
	}

	// Pre-populate the pool
	for i := 0; i < poolSize; i++ {
		bp.pool.Put(make([]byte, 0, maxSize))
	}

	return bp
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() []byte {
	atomic.AddInt64(&bp.stats.gets, 1)
	buf := bp.pool.Get().([]byte)
	return buf[:0] // Reset length but keep capacity
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(buf []byte) {
	if cap(buf) > bp.maxSize {
		return // Don't put oversized buffers back
	}

	atomic.AddInt64(&bp.stats.puts, 1)
	bp.pool.Put(buf)
}

// GetStats returns buffer pool statistics
func (bp *BufferPool) GetStats() map[string]int64 {
	return map[string]int64{
		"gets":     atomic.LoadInt64(&bp.stats.gets),
		"puts":     atomic.LoadInt64(&bp.stats.puts),
		"creates":  atomic.LoadInt64(&bp.stats.creates),
		"maxUsage": atomic.LoadInt64(&bp.stats.maxUsage),
	}
}

// MessageBatcher batches messages for optimized transmission
type MessageBatcher struct {
	batchSize    int
	batchTimeout time.Duration
	messages     chan []byte
	batches      chan [][]byte
	stats        struct {
		// Cache line padded to prevent false sharing
		messagesIn     int64
		_              [56]byte // Cache line padding
		batchesOut     int64
		_              [56]byte // Cache line padding
		avgBatchSize   float64 // Protected by mutex, no padding needed
		droppedBatches int64
		_              [56]byte // Cache line padding
	}
	mutex  sync.RWMutex // Protect avgBatchSize field
	closed atomic.Bool  // Track if batcher is closed
}

// NewMessageBatcher creates a new message batcher
func NewMessageBatcher(batchSize int, batchTimeout time.Duration) *MessageBatcher {
	return &MessageBatcher{
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		messages:     make(chan []byte, batchSize*100), // Increased for high throughput
		batches:      make(chan [][]byte, 1000), // Increased for high throughput
	}
}

// Start starts the message batcher
func (mb *MessageBatcher) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	batch := make([][]byte, 0, mb.batchSize)
	timer := time.NewTimer(mb.batchTimeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			// Flush remaining messages with context-aware sending
			if len(batch) > 0 {
				mb.flushBatchWithContext(ctx, batch)
			}
			return

		case msg, ok := <-mb.messages:
			if !ok {
				// Channel closed, flush remaining and exit
				if len(batch) > 0 {
					mb.flushBatchWithContext(ctx, batch)
				}
				return
			}
			batch = append(batch, msg)
			atomic.AddInt64(&mb.stats.messagesIn, 1)

			if len(batch) >= mb.batchSize {
				mb.flushBatchWithContext(ctx, batch)
				batch = make([][]byte, 0, mb.batchSize)
				timer.Reset(mb.batchTimeout)
			}

		case <-timer.C:
			if len(batch) > 0 {
				mb.flushBatchWithContext(ctx, batch)
				batch = make([][]byte, 0, mb.batchSize)
			}
			timer.Reset(mb.batchTimeout)
		}
	}
}

// AddMessage adds a message to the batch
func (mb *MessageBatcher) AddMessage(data []byte) error {
	if mb.closed.Load() {
		return errors.New("message batcher is closed")
	}
	select {
	case mb.messages <- data:
		return nil
	default:
		return fmt.Errorf("websocket performance manager: message queue is full")
	}
}

// GetBatch gets a batch of messages
func (mb *MessageBatcher) GetBatch() [][]byte {
	select {
	case batch := <-mb.batches:
		return batch
	default:
		return nil
	}
}

// Close closes the message batcher and its channels
func (mb *MessageBatcher) Close() {
	if mb.closed.CompareAndSwap(false, true) {
		close(mb.messages)
		close(mb.batches)
	}
}

// flushBatch sends a batch of messages
func (mb *MessageBatcher) flushBatch(batch [][]byte) {
	mb.flushBatchWithContext(context.Background(), batch)
}

func (mb *MessageBatcher) flushBatchWithContext(ctx context.Context, batch [][]byte) {
	if len(batch) == 0 {
		return
	}

	// Check if batcher is closed before attempting to send
	if mb.closed.Load() {
		atomic.AddInt64(&mb.stats.droppedBatches, 1)
		return
	}

	batchCopy := make([][]byte, len(batch))
	copy(batchCopy, batch)

	// Use defer recover to handle potential panic from sending on closed channel
	defer func() {
		if r := recover(); r != nil {
			// Channel was closed, count as dropped batch
			atomic.AddInt64(&mb.stats.droppedBatches, 1)
		}
	}()

	select {
	case mb.batches <- batchCopy:
		atomic.AddInt64(&mb.stats.batchesOut, 1)
		// Update average batch size with proper locking
		mb.mutex.Lock()
		currentAvg := mb.stats.avgBatchSize
		newAvg := (currentAvg*float64(mb.stats.batchesOut-1) + float64(len(batch))) / float64(mb.stats.batchesOut)
		mb.stats.avgBatchSize = newAvg
		mb.mutex.Unlock()
	case <-ctx.Done():
		// Context cancelled, don't block on channel send
		atomic.AddInt64(&mb.stats.droppedBatches, 1)
		return
	case <-time.After(100 * time.Millisecond):
		// Timeout to prevent indefinite blocking - drop the batch
		atomic.AddInt64(&mb.stats.droppedBatches, 1)
		return
	default:
		// Batch queue full, drop batch
		atomic.AddInt64(&mb.stats.droppedBatches, 1)
	}
}

// GetStats returns batcher statistics
func (mb *MessageBatcher) GetStats() map[string]interface{} {
	mb.mutex.RLock()
	avgBatchSize := mb.stats.avgBatchSize
	mb.mutex.RUnlock()

	return map[string]interface{}{
		"messages_in":    atomic.LoadInt64(&mb.stats.messagesIn),
		"batches_out":    atomic.LoadInt64(&mb.stats.batchesOut),
		"avg_batch_size": avgBatchSize,
	}
}

// ConnectionPoolManager manages connection slots for optimal resource usage
type ConnectionPoolManager struct {
	maxConnections int
	slots          chan *ConnectionSlot
	activeSlots    map[string]*ConnectionSlot
	mutex          sync.RWMutex
	stats          struct {
		// Cache line padded to prevent false sharing
		slotsAcquired int64
		_             [56]byte // Cache line padding
		slotsReleased int64
		_             [56]byte // Cache line padding
		maxUsage      int64
		_             [56]byte // Cache line padding
	}
}

// ConnectionSlot represents a connection slot
type ConnectionSlot struct {
	ID         string
	AcquiredAt time.Time
	InUse      bool
	mutex      sync.RWMutex
}

// NewConnectionPoolManager creates a new connection pool manager
func NewConnectionPoolManager(maxConnections int) *ConnectionPoolManager {
	cpm := &ConnectionPoolManager{
		maxConnections: maxConnections,
		slots:          make(chan *ConnectionSlot, maxConnections),
		activeSlots:    make(map[string]*ConnectionSlot),
	}

	// Pre-populate slots
	for i := 0; i < maxConnections; i++ {
		slot := &ConnectionSlot{
			ID: fmt.Sprintf("slot_%d", i),
		}
		cpm.slots <- slot
	}

	return cpm
}

// AcquireSlot acquires a connection slot
func (cpm *ConnectionPoolManager) AcquireSlot(ctx context.Context) (*ConnectionSlot, error) {
	select {
	case slot := <-cpm.slots:
		slot.mutex.Lock()
		slot.AcquiredAt = time.Now()
		slot.InUse = true
		slot.mutex.Unlock()

		cpm.mutex.Lock()
		cpm.activeSlots[slot.ID] = slot
		cpm.mutex.Unlock()

		atomic.AddInt64(&cpm.stats.slotsAcquired, 1)
		if acquired := atomic.LoadInt64(&cpm.stats.slotsAcquired); acquired > atomic.LoadInt64(&cpm.stats.maxUsage) {
			atomic.StoreInt64(&cpm.stats.maxUsage, acquired)
		}

		return slot, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ReleaseSlot releases a connection slot
func (cpm *ConnectionPoolManager) ReleaseSlot(slot *ConnectionSlot) {
	if slot == nil {
		return
	}

	slot.mutex.Lock()
	slot.InUse = false
	slot.mutex.Unlock()

	cpm.mutex.Lock()
	delete(cpm.activeSlots, slot.ID)
	cpm.mutex.Unlock()

	select {
	case cpm.slots <- slot:
		atomic.AddInt64(&cpm.stats.slotsReleased, 1)
	default:
		// Slot channel full, shouldn't happen
	}
}

// GetStats returns connection pool manager statistics
func (cpm *ConnectionPoolManager) GetStats() map[string]int64 {
	cpm.mutex.RLock()
	activeSlots := int64(len(cpm.activeSlots))
	cpm.mutex.RUnlock()

	return map[string]int64{
		"slots_acquired": atomic.LoadInt64(&cpm.stats.slotsAcquired),
		"slots_released": atomic.LoadInt64(&cpm.stats.slotsReleased),
		"max_usage":      atomic.LoadInt64(&cpm.stats.maxUsage),
		"active_slots":   activeSlots,
	}
}

// MessageSerializer defines interface for message serialization
type MessageSerializer interface {
	Serialize(event events.Event) ([]byte, error)
	Deserialize(data []byte) (events.Event, error)
}

// SerializerFactory creates message serializers
type SerializerFactory struct {
	serializerType SerializerType
	pool           sync.Pool
}

// NewSerializerFactory creates a new serializer factory
func NewSerializerFactory(serializerType SerializerType) *SerializerFactory {
	sf := &SerializerFactory{
		serializerType: serializerType,
	}

	sf.pool = sync.Pool{
		New: func() interface{} {
			switch serializerType {
			case OptimizedJSONSerializer:
				return &PerfOptimizedJSONSerializer{}
			case ProtobufSerializer:
				return &PerfProtobufSerializer{}
			default:
				return &PerfJSONSerializer{}
			}
		},
	}

	return sf
}

// GetSerializer gets a serializer from the pool
func (sf *SerializerFactory) GetSerializer() MessageSerializer {
	return sf.pool.Get().(MessageSerializer)
}

// PutSerializer returns a serializer to the pool
func (sf *SerializerFactory) PutSerializer(serializer MessageSerializer) {
	sf.pool.Put(serializer)
}

// PerfJSONSerializer implements standard JSON serialization
type PerfJSONSerializer struct{}

// Serialize serializes an event to JSON
func (js *PerfJSONSerializer) Serialize(event events.Event) ([]byte, error) {
	return event.ToJSON()
}

// Deserialize deserializes JSON to an event
func (js *PerfJSONSerializer) Deserialize(data []byte) (events.Event, error) {
	// This would need proper event type detection and parsing
	return nil, fmt.Errorf("PerfJSONSerializer.Deserialize: method not yet implemented")
}

// PerfOptimizedJSONSerializer implements optimized JSON serialization
type PerfOptimizedJSONSerializer struct {
	buffer []byte
}

// Serialize serializes an event to optimized JSON
func (ojs *PerfOptimizedJSONSerializer) Serialize(event events.Event) ([]byte, error) {
	// Use zero-copy JSON serialization where possible
	if ojs.buffer == nil {
		ojs.buffer = make([]byte, 0, 1024)
	}

	// Reset buffer
	ojs.buffer = ojs.buffer[:0]

	// Use event's ToJSON method for proper serialization
	// Use the event's ToJSON method for proper serialization
	data, err := event.ToJSON()
	if err != nil {
		return nil, err
	}

	// Copy to our buffer to reuse memory
	if len(data) <= cap(ojs.buffer) {
		ojs.buffer = append(ojs.buffer, data...)
		return ojs.buffer, nil
	}

	return data, nil
}

// Deserialize deserializes optimized JSON to an event
func (ojs *PerfOptimizedJSONSerializer) Deserialize(data []byte) (events.Event, error) {
	// This would need proper event type detection and parsing
	return nil, fmt.Errorf("PerfOptimizedJSONSerializer.Deserialize: method not yet implemented")
}

// PerfProtobufSerializer implements Protocol Buffers serialization
type PerfProtobufSerializer struct{}

// Serialize serializes an event to Protocol Buffers
func (ps *PerfProtobufSerializer) Serialize(event events.Event) ([]byte, error) {
	// Convert to protobuf and serialize
	pb, err := event.ToProtobuf()
	if err != nil {
		return nil, err
	}

	// This would use protobuf marshaling in a real implementation
	return json.Marshal(pb) // Placeholder
}

// Deserialize deserializes Protocol Buffers to an event
func (ps *PerfProtobufSerializer) Deserialize(data []byte) (events.Event, error) {
	// This would need proper protobuf parsing
	return nil, fmt.Errorf("PerfProtobufSerializer.Deserialize: method not yet implemented")
}

// ZeroCopyBuffer implements zero-copy buffer operations
type ZeroCopyBuffer struct {
	data   []byte
	offset int
}

// NewZeroCopyBuffer creates a new zero-copy buffer
func NewZeroCopyBuffer(data []byte) *ZeroCopyBuffer {
	return &ZeroCopyBuffer{
		data: data,
	}
}

// Bytes returns the buffer data using zero-copy
func (zcb *ZeroCopyBuffer) Bytes() []byte {
	return zcb.data[zcb.offset:]
}

// String returns the buffer data as a string using zero-copy
func (zcb *ZeroCopyBuffer) String() string {
	data := zcb.data[zcb.offset:]
	if len(data) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(data), len(data))
}

// Advance advances the buffer offset
func (zcb *ZeroCopyBuffer) Advance(n int) {
	zcb.offset += n
	if zcb.offset > len(zcb.data) {
		zcb.offset = len(zcb.data)
	}
}

// Reset resets the buffer
func (zcb *ZeroCopyBuffer) Reset() {
	zcb.offset = 0
}

// Len returns the remaining length
func (zcb *ZeroCopyBuffer) Len() int {
	return len(zcb.data) - zcb.offset
}

// MetricsCollector collects and tracks performance metrics
type MetricsCollector struct {
	interval time.Duration
	metrics  *PerformanceMetrics
	mutex    sync.RWMutex
}

// PerformanceMetrics contains various performance metrics
type PerformanceMetrics struct {
	// Connection metrics
	ActiveConnections    int64
	TotalConnections     int64
	ConnectionsPerSecond float64
	AvgConnectionTime    time.Duration

	// Message metrics
	MessagesPerSecond float64
	MessagesSent      int64
	MessagesReceived  int64
	MessagesFailures  int64
	AvgMessageSize    float64

	// Latency metrics
	AvgLatency time.Duration
	MinLatency time.Duration
	MaxLatency time.Duration
	P95Latency time.Duration
	P99Latency time.Duration

	// Throughput metrics
	BytesPerSecond     float64
	TotalBytesSent     int64
	TotalBytesReceived int64

	// Memory metrics
	MemoryUsage     int64
	BufferPoolUsage int64
	GCPauses        int64
	AvgGCPause      time.Duration

	// Serialization metrics
	SerializationTime     time.Duration
	SerializationPerSec   float64
	SerializationFailures int64

	// Error metrics
	ErrorRate        float64
	TotalErrors      int64
	ConnectionErrors int64
	TimeoutErrors    int64

	// System metrics
	CPUUsage       float64
	GoroutineCount int64
	HeapSize       int64
	StackSize      int64

	// Timestamps
	StartTime  time.Time
	LastUpdate time.Time
	Uptime     time.Duration
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		interval: interval,
		metrics: &PerformanceMetrics{
			StartTime:  time.Now(),
			LastUpdate: time.Now(),
		},
	}
}

// Start starts the metrics collector
func (mc *MetricsCollector) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(mc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mc.collectMetrics()
		}
	}
}

// collectMetrics collects current metrics
func (mc *MetricsCollector) collectMetrics() {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	now := time.Now()
	mc.metrics.LastUpdate = now
	mc.metrics.Uptime = now.Sub(mc.metrics.StartTime)

	// Collect memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	mc.metrics.MemoryUsage = int64(memStats.Alloc)
	mc.metrics.HeapSize = int64(memStats.HeapAlloc)
	mc.metrics.StackSize = int64(memStats.StackInuse)
	mc.metrics.GoroutineCount = int64(runtime.NumGoroutine())

	// Calculate CPU usage (simplified)
	mc.metrics.CPUUsage = float64(runtime.NumCPU()) * 100.0 / float64(runtime.GOMAXPROCS(0))

	// GC metrics
	mc.metrics.GCPauses = int64(memStats.NumGC)
	if memStats.NumGC > 0 {
		mc.metrics.AvgGCPause = time.Duration(memStats.PauseTotalNs / uint64(memStats.NumGC))
	}
}

// TrackConnectionTime tracks connection establishment time
func (mc *MetricsCollector) TrackConnectionTime(duration time.Duration) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.metrics.TotalConnections++
	if mc.metrics.AvgConnectionTime == 0 {
		mc.metrics.AvgConnectionTime = duration
	} else {
		mc.metrics.AvgConnectionTime = time.Duration(
			float64(mc.metrics.AvgConnectionTime)*0.9 + float64(duration)*0.1,
		)
	}
}

// TrackMessageLatency tracks message processing latency
func (mc *MetricsCollector) TrackMessageLatency(latency time.Duration) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if mc.metrics.MinLatency == 0 || latency < mc.metrics.MinLatency {
		mc.metrics.MinLatency = latency
	}
	if latency > mc.metrics.MaxLatency {
		mc.metrics.MaxLatency = latency
	}

	if mc.metrics.AvgLatency == 0 {
		mc.metrics.AvgLatency = latency
	} else {
		mc.metrics.AvgLatency = time.Duration(
			float64(mc.metrics.AvgLatency)*0.9 + float64(latency)*0.1,
		)
	}
}

// TrackSerializationTime tracks serialization time
func (mc *MetricsCollector) TrackSerializationTime(duration time.Duration) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if mc.metrics.SerializationTime == 0 {
		mc.metrics.SerializationTime = duration
	} else {
		mc.metrics.SerializationTime = time.Duration(
			float64(mc.metrics.SerializationTime)*0.9 + float64(duration)*0.1,
		)
	}
}

// TrackMessageSize tracks message size
func (mc *MetricsCollector) TrackMessageSize(size int) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if mc.metrics.AvgMessageSize == 0 {
		mc.metrics.AvgMessageSize = float64(size)
	} else {
		mc.metrics.AvgMessageSize = mc.metrics.AvgMessageSize*0.9 + float64(size)*0.1
	}
}

// TrackError tracks an error
func (mc *MetricsCollector) TrackError(errorType string) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.metrics.TotalErrors++
	switch errorType {
	case "connection":
		mc.metrics.ConnectionErrors++
	case "timeout":
		mc.metrics.TimeoutErrors++
	}

	// Calculate error rate
	totalOps := mc.metrics.MessagesSent + mc.metrics.MessagesReceived + mc.metrics.TotalConnections
	if totalOps > 0 {
		mc.metrics.ErrorRate = float64(mc.metrics.TotalErrors) / float64(totalOps) * 100
	}
}

// GetMetrics returns a copy of current metrics
func (mc *MetricsCollector) GetMetrics() *PerformanceMetrics {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	// Create a copy to avoid race conditions
	metrics := *mc.metrics
	return &metrics
}

// Profiler handles CPU and memory profiling
type Profiler struct {
	interval    time.Duration
	cpuProfile  *pprof.Profile
	memProfile  *pprof.Profile
	enabled     bool
	profileData map[string]interface{}
	mutex       sync.RWMutex
}

// NewProfiler creates a new profiler
func NewProfiler(interval time.Duration) *Profiler {
	return &Profiler{
		interval:    interval,
		enabled:     true,
		profileData: make(map[string]interface{}),
	}
}

// Start starts the profiler
func (p *Profiler) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if p.enabled {
				p.collectProfiles()
			}
		}
	}
}

// collectProfiles collects profiling data
func (p *Profiler) collectProfiles() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Collect CPU profile
	cpuProfile := pprof.Lookup("goroutine")
	if cpuProfile != nil {
		p.profileData["cpu_profile"] = cpuProfile
	}

	// Collect memory profile
	memProfile := pprof.Lookup("heap")
	if memProfile != nil {
		p.profileData["memory_profile"] = memProfile
	}

	// Collect goroutine profile
	goroutineProfile := pprof.Lookup("goroutine")
	if goroutineProfile != nil {
		p.profileData["goroutine_profile"] = goroutineProfile
	}

	p.profileData["timestamp"] = time.Now()
}

// GetProfilingData returns current profiling data
func (p *Profiler) GetProfilingData() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	data := make(map[string]interface{})
	for k, v := range p.profileData {
		data[k] = v
	}
	return data
}

// Enable enables profiling
func (p *Profiler) Enable() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.enabled = true
}

// Disable disables profiling
func (p *Profiler) Disable() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.enabled = false
}

// MemoryManager manages memory usage and optimization
type MemoryManager struct {
	maxMemory       int64
	currentUsage    int64
	gcThreshold     int64
	bufferPools     []*BufferPool
	mutex           sync.RWMutex
	currentInterval time.Duration
	lastPressure    float64
	checkNow        chan struct{} // Channel to trigger immediate checks
	stats           struct {
		// Cache line padded to prevent false sharing
		allocations   int64
		_             [56]byte // Cache line padding
		deallocations int64
		_             [56]byte // Cache line padding
		gcTriggers    int64
		_             [56]byte // Cache line padding
		peakUsage     int64
		_             [56]byte // Cache line padding
	}
}

// NewMemoryManager creates a new memory manager
func NewMemoryManager(maxMemory int64) *MemoryManager {
	return &MemoryManager{
		maxMemory:       maxMemory,
		gcThreshold:     maxMemory * 80 / 100, // 80% of max memory
		bufferPools:     make([]*BufferPool, 0),
		currentInterval: 60 * time.Second, // Start with low pressure interval
		lastPressure:    0,
		checkNow:        make(chan struct{}, 10), // Increased buffer to prevent blocking
	}
}

// calculateMemoryPressure calculates the current memory pressure as a percentage
func (mm *MemoryManager) calculateMemoryPressure() float64 {
	if mm.maxMemory == 0 {
		return 0
	}
	return float64(mm.currentUsage) / float64(mm.maxMemory) * 100
}

// getMonitoringInterval returns the appropriate monitoring interval based on memory pressure
func (mm *MemoryManager) getMonitoringInterval(pressure float64) time.Duration {
	switch {
	case pressure >= 95: // Critical pressure
		return 500 * time.Millisecond
	case pressure >= 85: // High pressure
		return 2 * time.Second
	case pressure >= 50: // Medium pressure
		return 15 * time.Second
	default: // Low pressure
		return 60 * time.Second
	}
}

// Start starts the memory manager with dynamic monitoring intervals
func (mm *MemoryManager) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// Create initial ticker with current interval
	ticker := time.NewTicker(mm.currentInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mm.performCheck(&ticker)
		case <-mm.checkNow:
			// Immediate check requested
			mm.performCheck(&ticker)
		}
	}
}

// performCheck performs memory check and updates monitoring interval
func (mm *MemoryManager) performCheck(ticker **time.Ticker) {
	mm.checkMemoryUsage()

	// Calculate new interval based on current pressure
	mm.mutex.RLock()
	pressure := mm.calculateMemoryPressure()
	newInterval := mm.getMonitoringInterval(pressure)
	oldInterval := mm.currentInterval
	mm.mutex.RUnlock()

	// Update interval if it has changed
	if newInterval != oldInterval {
		mm.mutex.Lock()
		mm.currentInterval = newInterval
		mm.lastPressure = pressure
		mm.mutex.Unlock()

		// Reset ticker with new interval
		(*ticker).Stop()
		*ticker = time.NewTicker(newInterval)
	}
}

// checkMemoryUsage checks and manages memory usage
func (mm *MemoryManager) checkMemoryUsage() {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	mm.currentUsage = int64(memStats.Alloc)

	if mm.currentUsage > mm.stats.peakUsage {
		mm.stats.peakUsage = mm.currentUsage
	}

	// Update last pressure reading
	mm.lastPressure = mm.calculateMemoryPressure()

	// Trigger GC if we're approaching the limit
	if mm.currentUsage > mm.gcThreshold {
		runtime.GC()
		mm.stats.gcTriggers++
	}
}

// AllocateBuffer allocates a buffer with memory tracking
func (mm *MemoryManager) AllocateBuffer(size int) []byte {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if mm.currentUsage+int64(size) > mm.maxMemory {
		// Try to trigger GC and recheck system memory
		runtime.GC()
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		
		// Still check against our limit, not system memory
		if mm.currentUsage+int64(size) > mm.maxMemory {
			return nil // Out of memory
		}
	}

	// Update current usage
	// Update our tracked usage
	mm.currentUsage += int64(size)
	atomic.AddInt64(&mm.stats.allocations, 1)
	
	// Update current usage to reflect this allocation
	mm.currentUsage += int64(size)
	
	return make([]byte, size)
}

// DeallocateBuffer deallocates a buffer
func (mm *MemoryManager) DeallocateBuffer(buf []byte) {
	mm.mutex.Lock()
	mm.currentUsage -= int64(len(buf))
	if mm.currentUsage < 0 {
		mm.currentUsage = 0
	}
	mm.mutex.Unlock()
	if buf == nil {
		return
	}
	
	mm.mutex.Lock()
	defer mm.mutex.Unlock()
	
	atomic.AddInt64(&mm.stats.deallocations, 1)
	
	// Update current usage to reflect this deallocation
	mm.currentUsage -= int64(len(buf))
	if mm.currentUsage < 0 {
		mm.currentUsage = 0
	}
	
	// Buffer will be garbage collected automatically
}

// GetCurrentUsage returns current memory usage
func (mm *MemoryManager) GetCurrentUsage() int64 {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.currentUsage
}

// GetStats returns memory manager statistics
func (mm *MemoryManager) GetStats() map[string]int64 {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	return map[string]int64{
		"current_usage":           mm.currentUsage,
		"max_memory":              mm.maxMemory,
		"peak_usage":              mm.stats.peakUsage,
		"allocations":             atomic.LoadInt64(&mm.stats.allocations),
		"deallocations":           atomic.LoadInt64(&mm.stats.deallocations),
		"gc_triggers":             atomic.LoadInt64(&mm.stats.gcTriggers),
		"memory_pressure_percent": int64(mm.lastPressure),
		"monitoring_interval_ms":  mm.currentInterval.Milliseconds(),
	}
}

// GetMemoryPressure returns the current memory pressure percentage
func (mm *MemoryManager) GetMemoryPressure() float64 {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.lastPressure
}

// GetMonitoringInterval returns the current monitoring interval
func (mm *MemoryManager) GetMonitoringInterval() time.Duration {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.currentInterval
}

// TriggerCheck triggers an immediate memory check and interval update
func (mm *MemoryManager) TriggerCheck() {
	select {
	case mm.checkNow <- struct{}{}:
		// Triggered successfully
	default:
		// Channel is full, check already pending
	}
}

// PerformanceOptimizer provides high-level optimization methods
type PerformanceOptimizer struct {
	manager *PerformanceManager
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer(manager *PerformanceManager) *PerformanceOptimizer {
	return &PerformanceOptimizer{
		manager: manager,
	}
}

// OptimizeForThroughput optimizes configuration for maximum throughput
func (po *PerformanceOptimizer) OptimizeForThroughput() {
	config := po.manager.config

	// Increase batch size for better throughput
	if config.MessageBatchSize < 50 {
		config.MessageBatchSize = 50
	}

	// Increase batch timeout for better batching
	if config.MessageBatchTimeout < 10*time.Millisecond {
		config.MessageBatchTimeout = 10 * time.Millisecond
	}

	// Use optimized serialization
	config.MessageSerializerType = OptimizedJSONSerializer
}

// OptimizeForLatency optimizes configuration for minimum latency
func (po *PerformanceOptimizer) OptimizeForLatency() {
	config := po.manager.config

	// Reduce batch size for lower latency
	if config.MessageBatchSize > 5 {
		config.MessageBatchSize = 5
	}

	// Reduce batch timeout for immediate sending
	if config.MessageBatchTimeout > 1*time.Millisecond {
		config.MessageBatchTimeout = 1 * time.Millisecond
	}

	// Use fastest serialization
	config.MessageSerializerType = OptimizedJSONSerializer
}

// OptimizeForMemory optimizes configuration for minimum memory usage
func (po *PerformanceOptimizer) OptimizeForMemory() {
	config := po.manager.config

	// Reduce buffer pool size
	if config.BufferPoolSize > 100 {
		config.BufferPoolSize = 100
	}

	// Reduce max buffer size
	if config.MaxBufferSize > 32*1024 {
		config.MaxBufferSize = 32 * 1024
	}

	// Enable aggressive memory pooling
	config.EnableMemoryPooling = true
}

// AdaptiveOptimizer automatically adjusts settings based on current performance
type AdaptiveOptimizer struct {
	manager   *PerformanceManager
	optimizer *PerformanceOptimizer
	enabled   bool
	mutex     sync.RWMutex
}

// NewAdaptiveOptimizer creates a new adaptive optimizer
func NewAdaptiveOptimizer(manager *PerformanceManager) *AdaptiveOptimizer {
	return &AdaptiveOptimizer{
		manager:   manager,
		optimizer: NewPerformanceOptimizer(manager),
		enabled:   true,
	}
}

// Start starts the adaptive optimizer
func (ao *AdaptiveOptimizer) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial adaptation after short delay
	initialTimer := time.NewTimer(100 * time.Millisecond)
	defer initialTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-initialTimer.C:
			if ao.enabled {
				ao.adaptSettings()
			}
		case <-ticker.C:
			if ao.enabled {
				ao.adaptSettings()
			}
		}
	}
}

// adaptSettings adapts settings based on current performance
func (ao *AdaptiveOptimizer) adaptSettings() {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	metrics := ao.manager.GetMetrics()
	if metrics == nil {
		return
	}

	// Adapt based on latency
	if metrics.AvgLatency > ao.manager.config.MaxLatency {
		ao.optimizer.OptimizeForLatency()
	}

	// Adapt based on memory usage
	if metrics.MemoryUsage > ao.manager.config.MaxMemoryUsage*80/100 {
		ao.optimizer.OptimizeForMemory()
	}

	// Adapt based on error rate
	if metrics.ErrorRate > 5.0 { // 5% error rate
		// Reduce load
		ao.manager.config.MessageBatchSize = max(1, ao.manager.config.MessageBatchSize/2)
	}
}

// Enable enables adaptive optimization
func (ao *AdaptiveOptimizer) Enable() {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()
	ao.enabled = true
}

// Disable disables adaptive optimization
func (ao *AdaptiveOptimizer) Disable() {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()
	ao.enabled = false
}

// TriggerAdaptation manually triggers the adaptation process for testing
func (ao *AdaptiveOptimizer) TriggerAdaptation() {
	ao.adaptSettings()
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
