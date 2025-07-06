package state

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// PerformanceOptimizer provides performance optimization for state operations
type PerformanceOptimizer struct {
	// Object pools for reducing allocations
	patchPool       sync.Pool
	stateChangePool sync.Pool
	eventPool       sync.Pool

	// Metrics
	allocations   atomic.Int64
	poolHits      atomic.Int64
	poolMisses    atomic.Int64
	gcPauses      atomic.Int64
	lastGCPause   atomic.Int64

	// Configuration
	enablePooling      bool
	enableBatching     bool
	enableCompression  bool
	batchSize         int
	batchTimeout      time.Duration
	compressionLevel  int
	maxConcurrency    int

	// Batch processing
	batchQueue    chan batchItem
	batchWorkers  sync.WaitGroup
	stopBatch     chan struct{}

	// Rate limiting
	rateLimiter   *RateLimiter
	maxOpsPerSec  int
}

// PerformanceOptions configures the performance optimizer
type PerformanceOptions struct {
	EnablePooling      bool
	EnableBatching     bool
	EnableCompression  bool
	BatchSize         int
	BatchTimeout      time.Duration
	CompressionLevel  int
	MaxConcurrency    int
	MaxOpsPerSecond   int
}

// DefaultPerformanceOptions returns default performance options
func DefaultPerformanceOptions() PerformanceOptions {
	return PerformanceOptions{
		EnablePooling:     true,
		EnableBatching:    true,
		EnableCompression: false,
		BatchSize:        100,
		BatchTimeout:     10 * time.Millisecond,
		CompressionLevel: 6,
		MaxConcurrency:   runtime.NumCPU() * 2,
		MaxOpsPerSecond:  10000,
	}
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer(opts PerformanceOptions) *PerformanceOptimizer {
	po := &PerformanceOptimizer{
		enablePooling:     opts.EnablePooling,
		enableBatching:    opts.EnableBatching,
		enableCompression: opts.EnableCompression,
		batchSize:        opts.BatchSize,
		batchTimeout:     opts.BatchTimeout,
		compressionLevel: opts.CompressionLevel,
		maxConcurrency:   opts.MaxConcurrency,
		maxOpsPerSec:     opts.MaxOpsPerSecond,
		batchQueue:       make(chan batchItem, opts.BatchSize*10),
		stopBatch:        make(chan struct{}),
	}

	// Initialize object pools
	po.patchPool = sync.Pool{
		New: func() interface{} {
			po.poolMisses.Add(1)
			return &JSONPatchOperation{}
		},
	}

	po.stateChangePool = sync.Pool{
		New: func() interface{} {
			po.poolMisses.Add(1)
			return &StateChange{}
		},
	}

	po.eventPool = sync.Pool{
		New: func() interface{} {
			po.poolMisses.Add(1)
			return &StateEvent{}
		},
	}

	// Initialize rate limiter
	if opts.MaxOpsPerSecond > 0 {
		po.rateLimiter = NewRateLimiter(opts.MaxOpsPerSecond)
	}

	// Start batch workers if enabled
	if opts.EnableBatching {
		po.startBatchWorkers()
	}

	// Start GC monitoring
	go po.monitorGC()

	return po
}

// GetPatchOperation gets a patch operation from the pool
func (po *PerformanceOptimizer) GetPatchOperation() *JSONPatchOperation {
	if !po.enablePooling {
		return &JSONPatchOperation{}
	}
	
	po.poolHits.Add(1)
	return po.patchPool.Get().(*JSONPatchOperation)
}

// PutPatchOperation returns a patch operation to the pool
func (po *PerformanceOptimizer) PutPatchOperation(op *JSONPatchOperation) {
	if !po.enablePooling {
		return
	}
	
	// Reset the operation
	op.Op = ""
	op.Path = ""
	op.Value = nil
	op.From = ""
	
	po.patchPool.Put(op)
}

// GetStateChange gets a state change from the pool
func (po *PerformanceOptimizer) GetStateChange() *StateChange {
	if !po.enablePooling {
		return &StateChange{}
	}
	
	po.poolHits.Add(1)
	return po.stateChangePool.Get().(*StateChange)
}

// PutStateChange returns a state change to the pool
func (po *PerformanceOptimizer) PutStateChange(sc *StateChange) {
	if !po.enablePooling {
		return
	}
	
	// Reset the state change
	sc.Path = ""
	sc.OldValue = nil
	sc.NewValue = nil
	sc.Operation = ""
	sc.Timestamp = time.Time{}
	
	po.stateChangePool.Put(sc)
}

// StateEvent represents a state event for pooling
type StateEvent struct {
	Type      string
	Path      string
	Value     interface{}
	Timestamp time.Time
}

// GetStateEvent gets a state event from the pool
func (po *PerformanceOptimizer) GetStateEvent() *StateEvent {
	if !po.enablePooling {
		return &StateEvent{}
	}
	
	po.poolHits.Add(1)
	return po.eventPool.Get().(*StateEvent)
}

// PutStateEvent returns a state event to the pool
func (po *PerformanceOptimizer) PutStateEvent(se *StateEvent) {
	if !po.enablePooling {
		return
	}
	
	// Reset the event
	se.Type = ""
	se.Path = ""
	se.Value = nil
	se.Timestamp = time.Time{}
	
	po.eventPool.Put(se)
}

// batchItem represents an item in the batch queue
type batchItem struct {
	operation func() error
	result    chan error
}

// startBatchWorkers starts the batch processing workers
func (po *PerformanceOptimizer) startBatchWorkers() {
	for i := 0; i < po.maxConcurrency; i++ {
		po.batchWorkers.Add(1)
		go po.batchWorker()
	}
}

// batchWorker processes items from the batch queue
func (po *PerformanceOptimizer) batchWorker() {
	defer po.batchWorkers.Done()
	
	batch := make([]batchItem, 0, po.batchSize)
	timer := time.NewTimer(po.batchTimeout)
	timer.Stop()
	
	for {
		select {
		case item := <-po.batchQueue:
			batch = append(batch, item)
			
			if len(batch) == 1 {
				timer.Reset(po.batchTimeout)
			}
			
			if len(batch) >= po.batchSize {
				po.processBatch(batch)
				batch = batch[:0]
				timer.Stop()
			}
			
		case <-timer.C:
			if len(batch) > 0 {
				po.processBatch(batch)
				batch = batch[:0]
			}
			
		case <-po.stopBatch:
			if len(batch) > 0 {
				po.processBatch(batch)
			}
			return
		}
	}
}

// processBatch processes a batch of operations
func (po *PerformanceOptimizer) processBatch(batch []batchItem) {
	// Process all operations in the batch
	for _, item := range batch {
		err := item.operation()
		if item.result != nil {
			item.result <- err
		}
	}
}

// BatchOperation submits an operation for batch processing
func (po *PerformanceOptimizer) BatchOperation(ctx context.Context, operation func() error) error {
	if !po.enableBatching {
		return operation()
	}
	
	// Check rate limit
	if po.rateLimiter != nil {
		if err := po.rateLimiter.Wait(ctx); err != nil {
			return err
		}
	}
	
	result := make(chan error, 1)
	select {
	case po.batchQueue <- batchItem{operation: operation, result: result}:
		select {
		case err := <-result:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

// monitorGC monitors garbage collection pauses
func (po *PerformanceOptimizer) monitorGC() {
	var lastNumGC uint32
	var memStats runtime.MemStats
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for range ticker.C {
		runtime.ReadMemStats(&memStats)
		
		if memStats.NumGC > lastNumGC {
			po.gcPauses.Add(int64(memStats.NumGC - lastNumGC))
			po.lastGCPause.Store(int64(memStats.PauseNs[(memStats.NumGC+255)%256]))
			lastNumGC = memStats.NumGC
		}
		
		po.allocations.Store(int64(memStats.Mallocs))
	}
}

// GetMetrics returns performance metrics
func (po *PerformanceOptimizer) GetMetrics() PerformanceMetrics {
	return PerformanceMetrics{
		Allocations:      po.allocations.Load(),
		PoolHits:        po.poolHits.Load(),
		PoolMisses:      po.poolMisses.Load(),
		GCPauses:        po.gcPauses.Load(),
		LastGCPauseNs:   po.lastGCPause.Load(),
		PoolEfficiency:  po.calculatePoolEfficiency(),
	}
}

// calculatePoolEfficiency calculates the pool hit rate
func (po *PerformanceOptimizer) calculatePoolEfficiency() float64 {
	hits := float64(po.poolHits.Load())
	misses := float64(po.poolMisses.Load())
	total := hits + misses
	
	if total == 0 {
		return 0
	}
	
	return hits / total * 100
}

// Stop stops the performance optimizer
func (po *PerformanceOptimizer) Stop() {
	if po.enableBatching {
		close(po.stopBatch)
		po.batchWorkers.Wait()
	}
}

// PerformanceMetrics contains performance metrics
type PerformanceMetrics struct {
	Allocations     int64
	PoolHits       int64
	PoolMisses     int64
	GCPauses       int64
	LastGCPauseNs  int64
	PoolEfficiency float64
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	rate       int
	bucket     chan struct{}
	ticker     *time.Ticker
	stop       chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(ratePerSecond int) *RateLimiter {
	rl := &RateLimiter{
		rate:   ratePerSecond,
		bucket: make(chan struct{}, ratePerSecond),
		ticker: time.NewTicker(time.Second / time.Duration(ratePerSecond)),
		stop:   make(chan struct{}),
	}
	
	// Fill the bucket initially
	for i := 0; i < ratePerSecond; i++ {
		rl.bucket <- struct{}{}
	}
	
	// Start the token generator
	go rl.generate()
	
	return rl
}

// generate generates tokens at the specified rate
func (rl *RateLimiter) generate() {
	for {
		select {
		case <-rl.ticker.C:
			select {
			case rl.bucket <- struct{}{}:
				// Token added
			default:
				// Bucket full, discard token
			}
		case <-rl.stop:
			rl.ticker.Stop()
			return
		}
	}
}

// Wait waits for a token or until context is done
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.bucket:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	close(rl.stop)
}

// OptimizedDelta represents an optimized delta with compression
type OptimizedDelta struct {
	Operations []JSONPatchOperation
	Compressed bool
	Size       int
}

// CompressDelta compresses a JSON patch for network transmission
func CompressDelta(patch JSONPatch) (*OptimizedDelta, error) {
	// For now, just return the patch as-is
	// In a real implementation, you could use gzip or other compression
	return &OptimizedDelta{
		Operations: patch,
		Compressed: false,
		Size:      len(patch),
	}, nil
}

// DecompressDelta decompresses an optimized delta
func DecompressDelta(delta *OptimizedDelta) (JSONPatch, error) {
	if !delta.Compressed {
		return delta.Operations, nil
	}
	
	// In a real implementation, decompress the data
	return delta.Operations, nil
}