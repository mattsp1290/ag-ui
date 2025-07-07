package state

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
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
	bufferPool      sync.Pool // Buffer pool for compression/decompression

	// Metrics
	allocations   atomic.Int64
	poolHits      atomic.Int64
	poolMisses    atomic.Int64
	gcPauses      atomic.Int64
	lastGCPause   atomic.Int64
	memoryUsage   atomic.Int64
	connections   atomic.Int64
	bytesRead     atomic.Int64
	bytesWritten  atomic.Int64
	cacheHits     atomic.Int64
	cacheMisses   atomic.Int64

	// Configuration
	enablePooling      bool
	enableBatching     bool
	enableCompression  bool
	enableLazyLoading  bool
	enableSharding     bool
	batchSize         int
	batchTimeout      time.Duration
	compressionLevel  int
	maxConcurrency    int
	maxMemoryUsage    int64
	shardCount        int

	// Batch processing
	batchQueue    chan batchItem
	batchWorkers  sync.WaitGroup
	stopBatch     chan struct{}

	// Rate limiting
	rateLimiter   *RateLimiter
	maxOpsPerSec  int

	// Connection pooling
	connectionPool  *ConnectionPool

	// State sharding
	stateShards     []*StateShard

	// Lazy loading cache
	lazyCache       *LazyCache

	// Memory optimizer
	memoryOptimizer *MemoryOptimizer

	// Concurrent access optimizer
	concurrentOptimizer *ConcurrentOptimizer
	
	// Context and lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// PerformanceOptions configures the performance optimizer
type PerformanceOptions struct {
	EnablePooling      bool
	EnableBatching     bool
	EnableCompression  bool
	EnableLazyLoading  bool
	EnableSharding     bool
	BatchSize         int
	BatchTimeout      time.Duration
	CompressionLevel  int
	MaxConcurrency    int
	MaxOpsPerSecond   int
	MaxMemoryUsage    int64
	ShardCount        int
	ConnectionPoolSize int
	LazyCacheSize     int
	CacheExpiryTime   time.Duration
}

// DefaultPerformanceOptions returns default performance options
func DefaultPerformanceOptions() PerformanceOptions {
	return PerformanceOptions{
		EnablePooling:      true,
		EnableBatching:     true,
		EnableCompression:  false,
		EnableLazyLoading:  true,
		EnableSharding:     true,
		BatchSize:          DefaultBatchSize,
		BatchTimeout:       DefaultPerformanceBatchTimeout,
		CompressionLevel:   DefaultCompressionLevel,
		MaxConcurrency:     runtime.NumCPU() * 2,
		MaxOpsPerSecond:    DefaultMaxOpsPerSecond,
		MaxMemoryUsage:     DefaultMaxMemoryUsage, // 100MB
		ShardCount:         DefaultShardCount,
		ConnectionPoolSize: DefaultConnectionPoolSize,
		LazyCacheSize:      DefaultLazyCacheSize,
		CacheExpiryTime:    DefaultLazyCacheExpiryTime,
	}
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer(opts PerformanceOptions) *PerformanceOptimizer {
	ctx, cancel := context.WithCancel(context.Background())
	
	po := &PerformanceOptimizer{
		enablePooling:     opts.EnablePooling,
		enableBatching:    opts.EnableBatching,
		enableCompression: opts.EnableCompression,
		enableLazyLoading: opts.EnableLazyLoading,
		enableSharding:    opts.EnableSharding,
		batchSize:        opts.BatchSize,
		batchTimeout:     opts.BatchTimeout,
		compressionLevel: opts.CompressionLevel,
		maxConcurrency:   opts.MaxConcurrency,
		maxOpsPerSec:     opts.MaxOpsPerSecond,
		maxMemoryUsage:   opts.MaxMemoryUsage,
		shardCount:       opts.ShardCount,
		batchQueue:       make(chan batchItem, opts.BatchSize*DefaultTaskQueueMultiplier*5), // 10x batch size
		stopBatch:        make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
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

	po.bufferPool = sync.Pool{
		New: func() interface{} {
			po.poolMisses.Add(1)
			return bytes.NewBuffer(make([]byte, 0, BufferPoolSize))
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

	// Initialize connection pool with default factory
	po.connectionPool = NewConnectionPoolWithDefault(opts.ConnectionPoolSize)

	// Initialize state shards if enabled
	if opts.EnableSharding {
		po.stateShards = make([]*StateShard, opts.ShardCount)
		for i := 0; i < opts.ShardCount; i++ {
			po.stateShards[i] = NewStateShard()
		}
	}

	// Initialize lazy cache if enabled
	if opts.EnableLazyLoading {
		po.lazyCache = NewLazyCache(opts.LazyCacheSize, opts.CacheExpiryTime)
	}

	// Initialize memory optimizer
	po.memoryOptimizer = NewMemoryOptimizer(opts.MaxMemoryUsage)

	// Initialize concurrent access optimizer
	po.concurrentOptimizer = NewConcurrentOptimizer(opts.MaxConcurrency)

	// Start monitoring goroutines
	po.wg.Add(2)
	go po.monitorGC()
	go po.monitorMemory()

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
	defer po.wg.Done()
	
	var lastNumGC uint32
	var memStats runtime.MemStats
	
	ticker := time.NewTicker(DefaultGCMonitoringInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			runtime.ReadMemStats(&memStats)
			
			if memStats.NumGC > lastNumGC {
				po.gcPauses.Add(int64(memStats.NumGC - lastNumGC))
				po.lastGCPause.Store(int64(memStats.PauseNs[(memStats.NumGC+255)%256]))
				lastNumGC = memStats.NumGC
			}
			
			po.allocations.Store(int64(memStats.Mallocs))
		case <-po.ctx.Done():
			return
		}
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
		MemoryUsage:     po.memoryUsage.Load(),
		Connections:     po.connections.Load(),
		BytesRead:       po.bytesRead.Load(),
		BytesWritten:    po.bytesWritten.Load(),
		CacheHits:       po.cacheHits.Load(),
		CacheMisses:     po.cacheMisses.Load(),
		CacheHitRate:    po.calculateCacheHitRate(),
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

// calculateCacheHitRate calculates the cache hit rate
func (po *PerformanceOptimizer) calculateCacheHitRate() float64 {
	hits := float64(po.cacheHits.Load())
	misses := float64(po.cacheMisses.Load())
	total := hits + misses
	
	if total == 0 {
		return 0
	}
	
	return hits / total * 100
}

// Stop stops the performance optimizer
func (po *PerformanceOptimizer) Stop() {
	// Cancel context to stop monitoring goroutines
	po.cancel()
	
	// Wait for all goroutines to finish
	po.wg.Wait()
	
	if po.enableBatching {
		close(po.stopBatch)
		po.batchWorkers.Wait()
	}
	
	// Stop rate limiter
	if po.rateLimiter != nil {
		po.rateLimiter.Stop()
	}
	
	// Close connection pool
	if po.connectionPool != nil {
		po.connectionPool.Close()
	}
	
	// Shutdown concurrent optimizer
	if po.concurrentOptimizer != nil {
		po.concurrentOptimizer.Shutdown()
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
	MemoryUsage    int64
	Connections    int64
	BytesRead      int64
	BytesWritten   int64
	CacheHits      int64
	CacheMisses    int64
	CacheHitRate   float64
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
	// Convert patch to JSON
	jsonData, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch: %w", err)
	}

	// Compress the JSON data
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(jsonData); err != nil {
		return nil, fmt.Errorf("failed to compress patch: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close compression writer: %w", err)
	}

	compressed := buf.Bytes()
	
	return &OptimizedDelta{
		Operations: patch,
		Compressed: true,
		Size:       len(compressed),
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

// ConnectionFactory creates new connections
type ConnectionFactory func() Connection

// ConnectionPool manages a pool of connections to storage backends
type ConnectionPool struct {
	connections chan Connection
	mu          sync.RWMutex
	size        int
	created     int
	maxSize     int
	factory     ConnectionFactory
}

// Connection represents a connection to a storage backend
type Connection interface {
	Close() error
	IsValid() bool
	LastUsed() time.Time
}

// NewConnectionPool creates a new connection pool with a factory function
func NewConnectionPool(size int, factory ConnectionFactory) *ConnectionPool {
	return &ConnectionPool{
		connections: make(chan Connection, size),
		maxSize:     size,
		factory:     factory,
	}
}

// NewConnectionPoolWithDefault creates a new connection pool with a default factory (for backward compatibility)
func NewConnectionPoolWithDefault(size int) *ConnectionPool {
	// Default factory returns nil - this should be overridden in production
	return NewConnectionPool(size, func() Connection {
		return nil // Production code should provide a real factory
	})
}

// Get retrieves a connection from the pool
func (cp *ConnectionPool) Get() (Connection, error) {
	select {
	case conn := <-cp.connections:
		if conn.IsValid() {
			return conn, nil
		}
		// Connection is invalid, decrement count and create new one
		cp.mu.Lock()
		cp.created--
		cp.mu.Unlock()
	default:
		// No connections available
	}
	
	// Try to create a new connection
	cp.mu.Lock()
	if cp.created < cp.maxSize {
		cp.created++
		cp.mu.Unlock()
		// Create new connection using factory
		if cp.factory == nil {
			return nil, fmt.Errorf("no connection factory configured")
		}
		conn := cp.factory()
		if conn == nil {
			cp.mu.Lock()
			cp.created--
			cp.mu.Unlock()
			return nil, fmt.Errorf("connection factory returned nil")
		}
		return conn, nil
	}
	cp.mu.Unlock()
	return nil, fmt.Errorf("connection pool exhausted")
}

// Put returns a connection to the pool
func (cp *ConnectionPool) Put(conn Connection) {
	if conn == nil || !conn.IsValid() {
		cp.mu.Lock()
		cp.created--
		cp.mu.Unlock()
		return
	}
	
	select {
	case cp.connections <- conn:
		// Connection returned to pool
	default:
		// Pool is full, close the connection
		conn.Close()
		cp.mu.Lock()
		cp.created--
		cp.mu.Unlock()
	}
}

// Close closes all connections in the pool
func (cp *ConnectionPool) Close() {
	close(cp.connections)
	for conn := range cp.connections {
		conn.Close()
	}
}


// StateShard represents a shard of state data for better distribution
type StateShard struct {
	mu        sync.RWMutex
	data      map[string]interface{}
	version   int64
	size      int64
	lastAccess time.Time
}

// NewStateShard creates a new state shard
func NewStateShard() *StateShard {
	return &StateShard{
		data:       make(map[string]interface{}),
		version:    0,
		size:       0,
		lastAccess: time.Now(),
	}
}

// Get retrieves a value from the shard
func (ss *StateShard) Get(key string) (interface{}, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	
	ss.lastAccess = time.Now()
	value, exists := ss.data[key]
	return value, exists
}

// Set stores a value in the shard
func (ss *StateShard) Set(key string, value interface{}) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	
	// Estimate size change
	oldSize := ss.estimateSize(ss.data[key])
	newSize := ss.estimateSize(value)
	
	ss.data[key] = value
	ss.version++
	ss.size += newSize - oldSize
	ss.lastAccess = time.Now()
}

// Delete removes a value from the shard
func (ss *StateShard) Delete(key string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	
	if value, exists := ss.data[key]; exists {
		delete(ss.data, key)
		ss.version++
		ss.size -= ss.estimateSize(value)
		ss.lastAccess = time.Now()
	}
}

// estimateSize estimates the memory size of a value
func (ss *StateShard) estimateSize(value interface{}) int64 {
	if value == nil {
		return 0
	}
	
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case map[string]interface{}:
		size := int64(0)
		for k, val := range v {
			size += int64(len(k)) + ss.estimateSize(val)
		}
		return size
	case []interface{}:
		size := int64(0)
		for _, val := range v {
			size += ss.estimateSize(val)
		}
		return size
	default:
		// Rough estimate for other types
		return 64
	}
}

// GetShardForKey returns the shard index for a given key
func (po *PerformanceOptimizer) GetShardForKey(key string) int {
	if !po.enableSharding || len(po.stateShards) == 0 {
		return 0
	}
	
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32()) % len(po.stateShards)
}

// LazyCache implements lazy loading with TTL cache
type LazyCache struct {
	cache      sync.Map
	size       int
	maxSize    int
	ttl        time.Duration
	hits       atomic.Int64
	misses     atomic.Int64
	mu         sync.RWMutex
	keys       []string // For LRU eviction
}

// CacheEntry represents a cached entry
type CacheEntry struct {
	value    interface{}
	expires  time.Time
	accessed time.Time
}

// NewLazyCache creates a new lazy cache
func NewLazyCache(maxSize int, ttl time.Duration) *LazyCache {
	lc := &LazyCache{
		maxSize: maxSize,
		ttl:     ttl,
		keys:    make([]string, 0, maxSize),
	}
	
	// Start cleanup goroutine
	go lc.cleanup()
	
	return lc
}

// Get retrieves a value from the cache
func (lc *LazyCache) Get(key string) (interface{}, bool) {
	if val, ok := lc.cache.Load(key); ok {
		entry := val.(*CacheEntry)
		if time.Now().Before(entry.expires) {
			entry.accessed = time.Now()
			lc.hits.Add(1)
			return entry.value, true
		}
		// Entry expired, remove it
		lc.cache.Delete(key)
		lc.removeKey(key)
	}
	
	lc.misses.Add(1)
	return nil, false
}

// Set stores a value in the cache
func (lc *LazyCache) Set(key string, value interface{}) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	// Check if we need to evict
	if lc.size >= lc.maxSize {
		lc.evictLRU()
	}
	
	entry := &CacheEntry{
		value:    value,
		expires:  time.Now().Add(lc.ttl),
		accessed: time.Now(),
	}
	
	lc.cache.Store(key, entry)
	lc.keys = append(lc.keys, key)
	lc.size++
}

// evictLRU evicts the least recently used entry
func (lc *LazyCache) evictLRU() {
	if len(lc.keys) == 0 {
		return
	}
	
	var oldestKey string
	var oldestTime time.Time
	
	for _, key := range lc.keys {
		if val, ok := lc.cache.Load(key); ok {
			entry := val.(*CacheEntry)
			if oldestKey == "" || entry.accessed.Before(oldestTime) {
				oldestKey = key
				oldestTime = entry.accessed
			}
		}
	}
	
	if oldestKey != "" {
		lc.cache.Delete(oldestKey)
		lc.removeKey(oldestKey)
		lc.size--
	}
}

// removeKey removes a key from the keys slice
func (lc *LazyCache) removeKey(key string) {
	for i, k := range lc.keys {
		if k == key {
			lc.keys = append(lc.keys[:i], lc.keys[i+1:]...)
			break
		}
	}
}

// cleanup removes expired entries
func (lc *LazyCache) cleanup() {
	ticker := time.NewTicker(DefaultCleanupWorkerInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		now := time.Now()
		lc.cache.Range(func(key, value interface{}) bool {
			entry := value.(*CacheEntry)
			if now.After(entry.expires) {
				lc.cache.Delete(key)
				lc.mu.Lock()
				lc.removeKey(key.(string))
				lc.size--
				lc.mu.Unlock()
			}
			return true
		})
	}
}

// GetStats returns cache statistics
func (lc *LazyCache) GetStats() (hits, misses int64, hitRate float64) {
	h := lc.hits.Load()
	m := lc.misses.Load()
	total := h + m
	
	if total > 0 {
		hitRate = float64(h) / float64(total) * 100
	}
	
	return h, m, hitRate
}

// MemoryOptimizer manages memory usage and garbage collection
type MemoryOptimizer struct {
	maxMemory     int64
	currentMemory atomic.Int64
	gcThreshold   int64
	lastGC        time.Time
	mu            sync.RWMutex
}

// NewMemoryOptimizer creates a new memory optimizer
func NewMemoryOptimizer(maxMemory int64) *MemoryOptimizer {
	return &MemoryOptimizer{
		maxMemory:   maxMemory,
		gcThreshold: maxMemory / 2, // Trigger GC at 50% memory usage
		lastGC:      time.Now(),
	}
}

// CheckMemoryUsage checks if memory usage is within limits
func (mo *MemoryOptimizer) CheckMemoryUsage() bool {
	current := mo.currentMemory.Load()
	return current < mo.maxMemory
}

// UpdateMemoryUsage updates the current memory usage
func (mo *MemoryOptimizer) UpdateMemoryUsage(delta int64) {
	mo.currentMemory.Add(delta)
	
	if mo.currentMemory.Load() > mo.gcThreshold {
		mo.maybeRunGC()
	}
}

// maybeRunGC runs garbage collection if needed
func (mo *MemoryOptimizer) maybeRunGC() {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	
	if time.Since(mo.lastGC) > DefaultGCInterval {
		runtime.GC()
		mo.lastGC = time.Now()
	}
}

// GetMemoryUsage returns current memory usage
func (mo *MemoryOptimizer) GetMemoryUsage() int64 {
	return mo.currentMemory.Load()
}

// ConcurrentOptimizer manages concurrent access patterns
type ConcurrentOptimizer struct {
	maxConcurrency int
	activeTasks    atomic.Int64
	taskQueue      chan func()
	workers        sync.WaitGroup
	shutdown       chan struct{}
}

// NewConcurrentOptimizer creates a new concurrent optimizer
func NewConcurrentOptimizer(maxConcurrency int) *ConcurrentOptimizer {
	co := &ConcurrentOptimizer{
		maxConcurrency: maxConcurrency,
		taskQueue:      make(chan func(), maxConcurrency*DefaultTaskQueueMultiplier),
		shutdown:       make(chan struct{}),
	}
	
	// Start worker goroutines
	for i := 0; i < maxConcurrency; i++ {
		co.workers.Add(1)
		go co.worker()
	}
	
	return co
}

// worker processes tasks from the queue
func (co *ConcurrentOptimizer) worker() {
	defer co.workers.Done()
	
	for {
		select {
		case task := <-co.taskQueue:
			co.activeTasks.Add(1)
			task()
			co.activeTasks.Add(-1)
		case <-co.shutdown:
			return
		}
	}
}

// Execute executes a task, potentially queuing it if at capacity
func (co *ConcurrentOptimizer) Execute(task func()) bool {
	select {
	case co.taskQueue <- task:
		return true
	default:
		// Queue is full, reject the task
		return false
	}
}

// GetActiveTasks returns the number of active tasks
func (co *ConcurrentOptimizer) GetActiveTasks() int64 {
	return co.activeTasks.Load()
}

// Shutdown shuts down the concurrent optimizer
func (co *ConcurrentOptimizer) Shutdown() {
	close(co.shutdown)
	co.workers.Wait()
}

// GetBuffer gets a buffer from the pool
func (po *PerformanceOptimizer) GetBuffer() *bytes.Buffer {
	if !po.enablePooling {
		return bytes.NewBuffer(make([]byte, 0, BufferPoolSize))
	}
	
	po.poolHits.Add(1)
	return po.bufferPool.Get().(*bytes.Buffer)
}

// PutBuffer returns a buffer to the pool
func (po *PerformanceOptimizer) PutBuffer(buf *bytes.Buffer) {
	if !po.enablePooling {
		return
	}
	
	buf.Reset()
	po.bufferPool.Put(buf)
}

// OptimizeForLargeState optimizes performance for large state sizes
func (po *PerformanceOptimizer) OptimizeForLargeState(stateSize int64) {
	if stateSize > DefaultMaxMemoryUsage { // 100MB
		// Enable all optimizations for large states
		po.enableCompression = true
		po.enableSharding = true
		po.enableLazyLoading = true
		
		// Adjust batch size for large states
		po.batchSize = int(math.Min(float64(po.batchSize*2), float64(DefaultMaxBatchSize)))
		
		// Trigger memory optimization
		if po.memoryOptimizer != nil {
			po.memoryOptimizer.maybeRunGC()
		}
	}
}

// monitorMemory monitors memory usage and triggers optimizations
func (po *PerformanceOptimizer) monitorMemory() {
	defer po.wg.Done()
	
	ticker := time.NewTicker(DefaultMemoryMonitoringInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			
			po.memoryUsage.Store(int64(memStats.Alloc))
			
			// Update memory optimizer
			if po.memoryOptimizer != nil {
				po.memoryOptimizer.currentMemory.Store(int64(memStats.Alloc))
			}
			
			// Trigger optimizations if memory usage is high
			if memStats.Alloc > DefaultCompressionThreshold { // 50MB
				po.OptimizeForLargeState(int64(memStats.Alloc))
			}
		case <-po.ctx.Done():
			return
		}
	}
}

// GetEnhancedMetrics returns enhanced performance metrics
func (po *PerformanceOptimizer) GetEnhancedMetrics() PerformanceMetrics {
	metrics := po.GetMetrics()
	
	// Add cache metrics if lazy cache is enabled
	if po.lazyCache != nil {
		cacheHits, cacheMisses, cacheHitRate := po.lazyCache.GetStats()
		metrics.CacheHits = cacheHits
		metrics.CacheMisses = cacheMisses
		metrics.CacheHitRate = cacheHitRate
	}
	
	// Add memory metrics
	metrics.MemoryUsage = po.memoryUsage.Load()
	
	// Add connection metrics
	metrics.Connections = po.connections.Load()
	
	// Add I/O metrics
	metrics.BytesRead = po.bytesRead.Load()
	metrics.BytesWritten = po.bytesWritten.Load()
	
	return metrics
}

// ProcessLargeStateUpdate processes large state updates efficiently
func (po *PerformanceOptimizer) ProcessLargeStateUpdate(ctx context.Context, update func() error) error {
	// Use concurrent optimizer for large updates
	if po.concurrentOptimizer != nil {
		done := make(chan error, 1)
		
		if po.concurrentOptimizer.Execute(func() {
			done <- update()
		}) {
			select {
			case err := <-done:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			// Fall back to direct execution if queue is full
			return update()
		}
	}
	
	return update()
}

// LazyLoadState loads state data lazily for better performance
func (po *PerformanceOptimizer) LazyLoadState(key string, loader func() (interface{}, error)) (interface{}, error) {
	if !po.enableLazyLoading || po.lazyCache == nil {
		return loader()
	}
	
	// Check cache first
	if value, found := po.lazyCache.Get(key); found {
		return value, nil
	}
	
	// Load from source
	value, err := loader()
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	po.lazyCache.Set(key, value)
	return value, nil
}

// ShardedGet retrieves data from the appropriate shard
func (po *PerformanceOptimizer) ShardedGet(key string) (interface{}, bool) {
	if !po.enableSharding || len(po.stateShards) == 0 {
		return nil, false
	}
	
	shardIndex := po.GetShardForKey(key)
	return po.stateShards[shardIndex].Get(key)
}

// ShardedSet stores data in the appropriate shard
func (po *PerformanceOptimizer) ShardedSet(key string, value interface{}) {
	if !po.enableSharding || len(po.stateShards) == 0 {
		return
	}
	
	shardIndex := po.GetShardForKey(key)
	po.stateShards[shardIndex].Set(key, value)
}

// CompressData compresses data using gzip
func (po *PerformanceOptimizer) CompressData(data []byte) ([]byte, error) {
	if !po.enableCompression {
		return data, nil
	}
	
	buf := po.GetBuffer()
	defer po.PutBuffer(buf)
	
	writer := gzip.NewWriter(buf)
	defer writer.Close()
	
	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("failed to compress data: %w", err)
	}
	
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close compression writer: %w", err)
	}
	
	po.bytesWritten.Add(int64(len(data)))
	return buf.Bytes(), nil
}

// DecompressData decompresses gzip data
func (po *PerformanceOptimizer) DecompressData(data []byte) ([]byte, error) {
	if !po.enableCompression {
		return data, nil
	}
	
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create decompression reader: %w", err)
	}
	defer reader.Close()
	
	buf := po.GetBuffer()
	defer po.PutBuffer(buf)
	
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}
	
	po.bytesRead.Add(int64(len(data)))
	return buf.Bytes(), nil
}