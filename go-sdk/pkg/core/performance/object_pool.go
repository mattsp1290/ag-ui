package performance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ObjectPoolConfig contains configuration for object pools
type ObjectPoolConfig struct {
	// Pool sizing
	InitialSize     int           // Initial number of objects to pre-allocate
	MaxSize         int           // Maximum number of objects in pool (0 = unlimited)
	MaxIdleTime     time.Duration // Maximum time an object can stay idle
	CleanupInterval time.Duration // Interval for cleanup operations

	// Performance tuning
	PreallocationSize int     // Size for preallocated objects (e.g., buffer size)
	GrowthFactor      float64 // Factor by which pool grows (default 1.5)
	ShrinkThreshold   float64 // Utilization threshold below which pool shrinks

	// Monitoring
	EnableMetrics   bool          // Enable metrics collection
	MetricsInterval time.Duration // Interval for metrics collection
}

// DefaultObjectPoolConfig returns default object pool configuration
func DefaultObjectPoolConfig() *ObjectPoolConfig {
	return &ObjectPoolConfig{
		InitialSize:       10,
		MaxSize:           1000,
		MaxIdleTime:       5 * time.Minute,
		CleanupInterval:   1 * time.Minute,
		PreallocationSize: 1024,
		GrowthFactor:      1.5,
		ShrinkThreshold:   0.3,
		EnableMetrics:     true,
		MetricsInterval:   30 * time.Second,
	}
}

// ObjectPool provides high-performance object pooling
type ObjectPool struct {
	config    *ObjectPoolConfig
	pool      sync.Pool
	factory   ObjectFactory
	validator ObjectValidator

	// Metrics
	metrics *PoolMetrics

	// Lifecycle management
	objects map[interface{}]*PooledObject
	mutex   sync.RWMutex

	// Cleanup
	cleanupTicker *time.Ticker
	metricsTicker *time.Ticker
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// ObjectFactory creates new objects
type ObjectFactory interface {
	CreateObject() interface{}
	ResetObject(obj interface{}) error
	ValidateObject(obj interface{}) bool
}

// ObjectValidator validates objects
type ObjectValidator interface {
	IsValid(obj interface{}) bool
	ShouldDiscard(obj interface{}) bool
}

// PooledObject wraps an object with pool metadata
type PooledObject struct {
	Object     interface{}
	CreatedAt  time.Time
	LastUsedAt time.Time
	UseCount   int64
	InUse      bool
	pool       *ObjectPool
	mutex      sync.RWMutex
}

// NewPooledObject creates a new pooled object
func NewPooledObject(obj interface{}, pool *ObjectPool) *PooledObject {
	return &PooledObject{
		Object:     obj,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		UseCount:   0,
		InUse:      false,
		pool:       pool,
	}
}

// Use marks the object as in use
func (po *PooledObject) Use() {
	po.mutex.Lock()
	defer po.mutex.Unlock()

	po.InUse = true
	po.LastUsedAt = time.Now()
	po.UseCount++
}

// Release returns the object to the pool
func (po *PooledObject) Release() {
	po.mutex.Lock()
	defer po.mutex.Unlock()

	po.InUse = false
	po.LastUsedAt = time.Now()

	// Return to pool
	po.pool.Put(po.Object)
}

// IsExpired checks if the object has expired
func (po *PooledObject) IsExpired(maxIdleTime time.Duration) bool {
	po.mutex.RLock()
	defer po.mutex.RUnlock()

	return maxIdleTime > 0 && !po.InUse && time.Since(po.LastUsedAt) > maxIdleTime
}

// PoolMetrics tracks pool performance metrics
type PoolMetrics struct {
	// Usage statistics
	TotalGets        int64
	TotalPuts        int64
	TotalCreated     int64
	TotalDiscarded   int64
	TotalValidations int64

	// Performance metrics
	AvgGetTime    time.Duration
	AvgPutTime    time.Duration
	AvgCreateTime time.Duration

	// Pool state
	CurrentSize    int32
	MaxSizeReached int32
	CleanupRuns    int64

	// Utilization
	HitRate         float64
	UtilizationRate float64

	// Memory stats
	MemoryUsage     int64
	MemoryAllocated int64

	mutex sync.RWMutex
}

// NewObjectPool creates a new object pool
func NewObjectPool(config *ObjectPoolConfig, factory ObjectFactory, validator ObjectValidator) *ObjectPool {
	if config == nil {
		config = DefaultObjectPoolConfig()
	}

	pool := &ObjectPool{
		config:    config,
		factory:   factory,
		validator: validator,
		metrics:   &PoolMetrics{},
		objects:   make(map[interface{}]*PooledObject),
		stopCh:    make(chan struct{}),
	}

	// Initialize sync.Pool
	pool.pool = sync.Pool{
		New: func() interface{} {
			return pool.createObject()
		},
	}

	// Pre-allocate initial objects
	pool.preallocate()

	// Start background workers
	if config.CleanupInterval > 0 {
		pool.cleanupTicker = time.NewTicker(config.CleanupInterval)
		pool.wg.Add(1)
		go pool.cleanupWorker()
	}

	if config.EnableMetrics && config.MetricsInterval > 0 {
		pool.metricsTicker = time.NewTicker(config.MetricsInterval)
		pool.wg.Add(1)
		go pool.metricsWorker()
	}

	return pool
}

// Get retrieves an object from the pool
func (p *ObjectPool) Get() interface{} {
	start := time.Now()

	// Get from pool
	obj := p.pool.Get()

	// Track usage
	p.mutex.Lock()
	if pooledObj, exists := p.objects[obj]; exists {
		pooledObj.Use()
	} else {
		// New object, wrap it
		pooledObj := NewPooledObject(obj, p)
		pooledObj.Use()
		p.objects[obj] = pooledObj
	}
	p.mutex.Unlock()

	// Update metrics
	p.updateGetMetrics(time.Since(start))

	return obj
}

// Put returns an object to the pool
func (p *ObjectPool) Put(obj interface{}) {
	if obj == nil {
		return
	}

	start := time.Now()

	// Validate object if validator is provided
	if p.validator != nil {
		if !p.validator.IsValid(obj) {
			p.discardObject(obj)
			return
		}

		if p.validator.ShouldDiscard(obj) {
			p.discardObject(obj)
			return
		}
	}

	// Reset object if factory supports it
	if p.factory != nil {
		if err := p.factory.ResetObject(obj); err != nil {
			p.discardObject(obj)
			return
		}
	}

	// Update pooled object metadata
	p.mutex.Lock()
	if pooledObj, exists := p.objects[obj]; exists {
		pooledObj.Release()
	}
	p.mutex.Unlock()

	// Check pool size limit
	if p.config.MaxSize > 0 {
		currentSize := atomic.LoadInt32(&p.metrics.CurrentSize)
		if int(currentSize) >= p.config.MaxSize {
			p.discardObject(obj)
			return
		}
	}

	// Return to pool
	p.pool.Put(obj)

	// Update metrics
	p.updatePutMetrics(time.Since(start))
}

// preallocate creates initial objects
func (p *ObjectPool) preallocate() {
	for i := 0; i < p.config.InitialSize; i++ {
		obj := p.createObject()
		p.pool.Put(obj)
	}
}

// createObject creates a new object
func (p *ObjectPool) createObject() interface{} {
	start := time.Now()

	var obj interface{}
	if p.factory != nil {
		obj = p.factory.CreateObject()
	} else {
		// Default object creation (empty interface)
		obj = make(map[string]interface{})
	}

	// Update metrics
	p.updateCreateMetrics(time.Since(start))
	atomic.AddInt32(&p.metrics.CurrentSize, 1)

	return obj
}

// discardObject removes an object from the pool
func (p *ObjectPool) discardObject(obj interface{}) {
	p.mutex.Lock()
	delete(p.objects, obj)
	p.mutex.Unlock()

	atomic.AddInt32(&p.metrics.CurrentSize, -1)
	atomic.AddInt64(&p.metrics.TotalDiscarded, 1)
}

// cleanupWorker performs periodic cleanup
func (p *ObjectPool) cleanupWorker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.cleanupTicker.C:
			p.performCleanup()
		case <-p.stopCh:
			return
		}
	}
}

// performCleanup removes expired objects
func (p *ObjectPool) performCleanup() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	expiredObjects := make([]interface{}, 0)

	for obj, pooledObj := range p.objects {
		if pooledObj.IsExpired(p.config.MaxIdleTime) {
			expiredObjects = append(expiredObjects, obj)
		}
	}

	// Remove expired objects
	for _, obj := range expiredObjects {
		delete(p.objects, obj)
		atomic.AddInt32(&p.metrics.CurrentSize, -1)
		atomic.AddInt64(&p.metrics.TotalDiscarded, 1)
	}

	atomic.AddInt64(&p.metrics.CleanupRuns, 1)
}

// metricsWorker collects and updates metrics
func (p *ObjectPool) metricsWorker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.metricsTicker.C:
			p.updateMetrics()
		case <-p.stopCh:
			return
		}
	}
}

// updateMetrics updates pool metrics
func (p *ObjectPool) updateMetrics() {
	p.metrics.mutex.Lock()
	defer p.metrics.mutex.Unlock()

	// Calculate hit rate
	totalGets := atomic.LoadInt64(&p.metrics.TotalGets)
	totalCreated := atomic.LoadInt64(&p.metrics.TotalCreated)
	if totalGets > 0 {
		p.metrics.HitRate = 1.0 - (float64(totalCreated) / float64(totalGets))
	}

	// Calculate utilization rate
	currentSize := atomic.LoadInt32(&p.metrics.CurrentSize)
	if p.config.MaxSize > 0 {
		p.metrics.UtilizationRate = float64(currentSize) / float64(p.config.MaxSize)
	}

	// Update memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	p.metrics.MemoryUsage = int64(memStats.Alloc)
	p.metrics.MemoryAllocated = int64(memStats.TotalAlloc)
}

// updateGetMetrics updates get operation metrics
func (p *ObjectPool) updateGetMetrics(duration time.Duration) {
	atomic.AddInt64(&p.metrics.TotalGets, 1)

	p.metrics.mutex.Lock()
	defer p.metrics.mutex.Unlock()

	// Update average get time
	gets := atomic.LoadInt64(&p.metrics.TotalGets)
	if gets > 1 {
		p.metrics.AvgGetTime = time.Duration(
			(int64(p.metrics.AvgGetTime)*gets + int64(duration)) / (gets + 1),
		)
	} else {
		p.metrics.AvgGetTime = duration
	}
}

// updatePutMetrics updates put operation metrics
func (p *ObjectPool) updatePutMetrics(duration time.Duration) {
	atomic.AddInt64(&p.metrics.TotalPuts, 1)

	p.metrics.mutex.Lock()
	defer p.metrics.mutex.Unlock()

	// Update average put time
	puts := atomic.LoadInt64(&p.metrics.TotalPuts)
	if puts > 1 {
		p.metrics.AvgPutTime = time.Duration(
			(int64(p.metrics.AvgPutTime)*puts + int64(duration)) / (puts + 1),
		)
	} else {
		p.metrics.AvgPutTime = duration
	}
}

// updateCreateMetrics updates create operation metrics
func (p *ObjectPool) updateCreateMetrics(duration time.Duration) {
	atomic.AddInt64(&p.metrics.TotalCreated, 1)

	p.metrics.mutex.Lock()
	defer p.metrics.mutex.Unlock()

	// Update average create time
	creates := atomic.LoadInt64(&p.metrics.TotalCreated)
	if creates > 1 {
		p.metrics.AvgCreateTime = time.Duration(
			(int64(p.metrics.AvgCreateTime)*creates + int64(duration)) / (creates + 1),
		)
	} else {
		p.metrics.AvgCreateTime = duration
	}
}

// GetMetrics returns pool metrics
func (p *ObjectPool) GetMetrics() *PoolMetrics {
	p.metrics.mutex.RLock()
	defer p.metrics.mutex.RUnlock()

	return &PoolMetrics{
		TotalGets:        atomic.LoadInt64(&p.metrics.TotalGets),
		TotalPuts:        atomic.LoadInt64(&p.metrics.TotalPuts),
		TotalCreated:     atomic.LoadInt64(&p.metrics.TotalCreated),
		TotalDiscarded:   atomic.LoadInt64(&p.metrics.TotalDiscarded),
		TotalValidations: atomic.LoadInt64(&p.metrics.TotalValidations),
		AvgGetTime:       p.metrics.AvgGetTime,
		AvgPutTime:       p.metrics.AvgPutTime,
		AvgCreateTime:    p.metrics.AvgCreateTime,
		CurrentSize:      atomic.LoadInt32(&p.metrics.CurrentSize),
		MaxSizeReached:   atomic.LoadInt32(&p.metrics.MaxSizeReached),
		CleanupRuns:      atomic.LoadInt64(&p.metrics.CleanupRuns),
		HitRate:          p.metrics.HitRate,
		UtilizationRate:  p.metrics.UtilizationRate,
		MemoryUsage:      p.metrics.MemoryUsage,
		MemoryAllocated:  p.metrics.MemoryAllocated,
	}
}

// GetPoolInfo returns pool information
func (p *ObjectPool) GetPoolInfo() map[string]interface{} {
	metrics := p.GetMetrics()

	return map[string]interface{}{
		"current_size":     metrics.CurrentSize,
		"max_size":         p.config.MaxSize,
		"initial_size":     p.config.InitialSize,
		"hit_rate":         metrics.HitRate,
		"utilization_rate": metrics.UtilizationRate,
		"total_gets":       metrics.TotalGets,
		"total_puts":       metrics.TotalPuts,
		"total_created":    metrics.TotalCreated,
		"total_discarded":  metrics.TotalDiscarded,
		"avg_get_time":     metrics.AvgGetTime,
		"avg_put_time":     metrics.AvgPutTime,
		"avg_create_time":  metrics.AvgCreateTime,
		"cleanup_runs":     metrics.CleanupRuns,
		"memory_usage":     metrics.MemoryUsage,
		"memory_allocated": metrics.MemoryAllocated,
	}
}

// Close closes the object pool
func (p *ObjectPool) Close() error {
	// Stop background workers
	close(p.stopCh)

	// Stop tickers
	if p.cleanupTicker != nil {
		p.cleanupTicker.Stop()
	}
	if p.metricsTicker != nil {
		p.metricsTicker.Stop()
	}

	// Wait for workers to finish
	p.wg.Wait()

	// Clear objects
	p.mutex.Lock()
	p.objects = make(map[interface{}]*PooledObject)
	p.mutex.Unlock()

	return nil
}

// Specialized Object Factories

// BufferFactory creates byte buffers
type BufferFactory struct {
	InitialSize int
	MaxSize     int
}

// NewBufferFactory creates a new buffer factory
func NewBufferFactory(initialSize, maxSize int) *BufferFactory {
	return &BufferFactory{
		InitialSize: initialSize,
		MaxSize:     maxSize,
	}
}

// CreateObject creates a new buffer
func (f *BufferFactory) CreateObject() interface{} {
	return bytes.NewBuffer(make([]byte, 0, f.InitialSize))
}

// ResetObject resets a buffer
func (f *BufferFactory) ResetObject(obj interface{}) error {
	if buffer, ok := obj.(*bytes.Buffer); ok {
		buffer.Reset()
		return nil
	}
	return fmt.Errorf("object is not a *bytes.Buffer")
}

// ValidateObject validates a buffer
func (f *BufferFactory) ValidateObject(obj interface{}) bool {
	if buffer, ok := obj.(*bytes.Buffer); ok {
		return buffer.Cap() <= f.MaxSize
	}
	return false
}

// SliceFactory creates byte slices
type SliceFactory struct {
	InitialSize int
	MaxSize     int
}

// NewSliceFactory creates a new slice factory
func NewSliceFactory(initialSize, maxSize int) *SliceFactory {
	return &SliceFactory{
		InitialSize: initialSize,
		MaxSize:     maxSize,
	}
}

// CreateObject creates a new slice
func (f *SliceFactory) CreateObject() interface{} {
	return make([]byte, 0, f.InitialSize)
}

// ResetObject resets a slice
func (f *SliceFactory) ResetObject(obj interface{}) error {
	if slice, ok := obj.([]byte); ok {
		obj = slice[:0]
		return nil
	}
	return fmt.Errorf("object is not a []byte")
}

// ValidateObject validates a slice
func (f *SliceFactory) ValidateObject(obj interface{}) bool {
	if slice, ok := obj.([]byte); ok {
		return cap(slice) <= f.MaxSize
	}
	return false
}

// JSONEncoderFactory creates JSON encoders
type JSONEncoderFactory struct {
	BufferPool *ObjectPool
}

// NewJSONEncoderFactory creates a new JSON encoder factory
func NewJSONEncoderFactory(bufferPool *ObjectPool) *JSONEncoderFactory {
	return &JSONEncoderFactory{
		BufferPool: bufferPool,
	}
}

// CreateObject creates a new JSON encoder
func (f *JSONEncoderFactory) CreateObject() interface{} {
	var buffer *bytes.Buffer
	if f.BufferPool != nil {
		buffer = f.BufferPool.Get().(*bytes.Buffer)
	} else {
		buffer = bytes.NewBuffer(make([]byte, 0, 1024))
	}

	return json.NewEncoder(buffer)
}

// ResetObject resets a JSON encoder
func (f *JSONEncoderFactory) ResetObject(obj interface{}) error {
	// JSON encoders can't be easily reset, so we don't reuse them
	return fmt.Errorf("JSON encoders cannot be reset")
}

// ValidateObject validates a JSON encoder
func (f *JSONEncoderFactory) ValidateObject(obj interface{}) bool {
	_, ok := obj.(*json.Encoder)
	return ok
}

// SimpleValidator provides basic object validation
type SimpleValidator struct {
	MaxAge time.Duration
}

// NewSimpleValidator creates a new simple validator
func NewSimpleValidator(maxAge time.Duration) *SimpleValidator {
	return &SimpleValidator{
		MaxAge: maxAge,
	}
}

// IsValid checks if an object is valid
func (v *SimpleValidator) IsValid(obj interface{}) bool {
	return obj != nil
}

// ShouldDiscard checks if an object should be discarded
func (v *SimpleValidator) ShouldDiscard(obj interface{}) bool {
	// Simple validation - could be extended with more sophisticated logic
	return false
}

// Global object pools for common types
var (
	// BufferPool for byte buffers
	BufferPool *ObjectPool

	// SlicePool for byte slices
	SlicePool *ObjectPool

	// StringBuilderPool for string builders
	StringBuilderPool *ObjectPool
)

// init initializes global object pools
func init() {
	// Initialize buffer pool
	bufferFactory := NewBufferFactory(1024, 1024*1024) // 1KB initial, 1MB max
	BufferPool = NewObjectPool(DefaultObjectPoolConfig(), bufferFactory, nil)

	// Initialize slice pool
	sliceFactory := NewSliceFactory(1024, 1024*1024) // 1KB initial, 1MB max
	SlicePool = NewObjectPool(DefaultObjectPoolConfig(), sliceFactory, nil)
}

// GetBuffer gets a buffer from the global pool
func GetBuffer() *bytes.Buffer {
	return BufferPool.Get().(*bytes.Buffer)
}

// PutBuffer returns a buffer to the global pool
func PutBuffer(buffer *bytes.Buffer) {
	BufferPool.Put(buffer)
}

// GetSlice gets a slice from the global pool
func GetSlice() []byte {
	return SlicePool.Get().([]byte)
}

// PutSlice returns a slice to the global pool
func PutSlice(slice []byte) {
	SlicePool.Put(slice)
}
