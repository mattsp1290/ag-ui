package encoding

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"
)

// PoolMetrics tracks pooling statistics
type PoolMetrics struct {
	Gets   int64 // Number of objects retrieved from pool
	Puts   int64 // Number of objects returned to pool
	News   int64 // Number of new objects created
	Resets int64 // Number of objects reset
}

// Pool interface for object pooling
type Pool[T any] interface {
	Get() T
	Put(obj T)
	Metrics() PoolMetrics
	Reset()
}

// BufferPool manages a pool of bytes.Buffer instances
type BufferPool struct {
	pool          sync.Pool
	metrics       PoolMetrics
	maxSize       int   // Maximum buffer size to keep in pool
	maxBuffers    int32 // Maximum number of buffers to allocate
	activeBuffers int32 // Current number of active buffers
	secureZero    bool  // Enable secure zeroing for sensitive data
	mu            sync.RWMutex
}

// NewBufferPool creates a new buffer pool with secure zeroing enabled by default
func NewBufferPool(maxSize int) *BufferPool {
	return NewBufferPoolWithOptions(maxSize, 1000, true) // Default max 1000 buffers, secure
}

// NewBufferPoolWithCapacity creates a new buffer pool with a capacity limit
func NewBufferPoolWithCapacity(maxSize int, maxBuffers int32) *BufferPool {
	return NewBufferPoolWithOptions(maxSize, maxBuffers, false)
}

// NewBufferPoolWithOptions creates a new buffer pool with full configuration
func NewBufferPoolWithOptions(maxSize int, maxBuffers int32, secureZero bool) *BufferPool {
	bp := &BufferPool{
		maxSize:    maxSize,
		maxBuffers: maxBuffers,
		secureZero: secureZero,
	}
	bp.pool.New = func() interface{} {
		// Check if we're exceeding the buffer limit
		if current := atomic.LoadInt32(&bp.activeBuffers); current >= bp.maxBuffers {
			return nil // Return nil to indicate resource exhaustion
		}
		atomic.AddInt32(&bp.activeBuffers, 1)
		atomic.AddInt64(&bp.metrics.News, 1)
		return &bytes.Buffer{}
	}
	return bp
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() *bytes.Buffer {
	atomic.AddInt64(&bp.metrics.Gets, 1)
	bufInterface := bp.pool.Get()
	if bufInterface == nil {
		// Resource limit exceeded
		return nil
	}
	buf := bufInterface.(*bytes.Buffer)
	// Buffer is already reset in Put(), no need to reset again
	return buf
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	atomic.AddInt64(&bp.metrics.Puts, 1)

	// Don't keep very large buffers in the pool
	if bp.maxSize > 0 && buf.Cap() > bp.maxSize {
		// Decrement active count for oversized buffers that won't be pooled
		atomic.AddInt32(&bp.activeBuffers, -1)
		return
	}

	atomic.AddInt64(&bp.metrics.Resets, 1)

	// Conditionally zero out sensitive data if secure mode is enabled
	if bp.secureZero && buf.Len() > 0 {
		bufBytes := buf.Bytes()
		for i := range bufBytes {
			bufBytes[i] = 0
		}
	}

	// Reset buffer length efficiently
	buf.Reset()
	bp.pool.Put(buf)
}

// PutSecure returns a buffer to the pool with secure zeroing regardless of pool setting
func (bp *BufferPool) PutSecure(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	atomic.AddInt64(&bp.metrics.Puts, 1)

	// Don't keep very large buffers in the pool
	if bp.maxSize > 0 && buf.Cap() > bp.maxSize {
		// Decrement active count for oversized buffers that won't be pooled
		atomic.AddInt32(&bp.activeBuffers, -1)
		return
	}

	atomic.AddInt64(&bp.metrics.Resets, 1)

	// Always zero out contents for sensitive data
	if buf.Len() > 0 {
		bufBytes := buf.Bytes()
		for i := range bufBytes {
			bufBytes[i] = 0
		}
	}

	buf.Reset()
	bp.pool.Put(buf)
}

// Metrics returns pool metrics
func (bp *BufferPool) Metrics() PoolMetrics {
	return PoolMetrics{
		Gets:   atomic.LoadInt64(&bp.metrics.Gets),
		Puts:   atomic.LoadInt64(&bp.metrics.Puts),
		News:   atomic.LoadInt64(&bp.metrics.News),
		Resets: atomic.LoadInt64(&bp.metrics.Resets),
	}
}

// Reset clears the pool
func (bp *BufferPool) Reset() {
	bp.pool = sync.Pool{
		New: func() interface{} {
			// Check if we're exceeding the buffer limit
			if current := atomic.LoadInt32(&bp.activeBuffers); current >= bp.maxBuffers {
				return nil // Return nil to indicate resource exhaustion
			}
			atomic.AddInt32(&bp.activeBuffers, 1)
			atomic.AddInt64(&bp.metrics.News, 1)
			return &bytes.Buffer{}
		},
	}
	atomic.StoreInt64(&bp.metrics.Gets, 0)
	atomic.StoreInt64(&bp.metrics.Puts, 0)
	atomic.StoreInt64(&bp.metrics.News, 0)
	atomic.StoreInt64(&bp.metrics.Resets, 0)
	atomic.StoreInt32(&bp.activeBuffers, 0)
}

// SlicePool manages a pool of byte slices
type SlicePool struct {
	pool         sync.Pool
	metrics      PoolMetrics
	maxSize      int   // Maximum slice size to keep in pool
	maxSlices    int32 // Maximum number of slices to allocate
	activeSlices int32 // Current number of active slices
	secureZero   bool  // Enable secure zeroing for sensitive data
}

// NewSlicePool creates a new slice pool with secure zeroing enabled by default
func NewSlicePool(initialSize, maxSize int) *SlicePool {
	return NewSlicePoolWithOptions(initialSize, maxSize, 1000, true) // Default max 1000 slices, secure
}

// NewSlicePoolWithCapacity creates a new slice pool with a capacity limit
func NewSlicePoolWithCapacity(initialSize, maxSize int, maxSlices int32) *SlicePool {
	return NewSlicePoolWithOptions(initialSize, maxSize, maxSlices, false)
}

// NewSlicePoolWithOptions creates a new slice pool with full configuration
func NewSlicePoolWithOptions(initialSize, maxSize int, maxSlices int32, secureZero bool) *SlicePool {
	sp := &SlicePool{
		maxSize:    maxSize,
		maxSlices:  maxSlices,
		secureZero: secureZero,
	}
	sp.pool.New = func() interface{} {
		// Check if we're exceeding the slice limit
		if current := atomic.LoadInt32(&sp.activeSlices); current >= sp.maxSlices {
			return nil // Return nil to indicate resource exhaustion
		}
		atomic.AddInt32(&sp.activeSlices, 1)
		atomic.AddInt64(&sp.metrics.News, 1)
		return make([]byte, 0, initialSize)
	}
	return sp
}

// Get retrieves a slice from the pool
func (sp *SlicePool) Get() []byte {
	atomic.AddInt64(&sp.metrics.Gets, 1)
	sliceInterface := sp.pool.Get()
	if sliceInterface == nil {
		// Resource limit exceeded
		return nil
	}
	slice := sliceInterface.([]byte)
	return slice[:0] // Reset length but keep capacity
}

// Put returns a slice to the pool
func (sp *SlicePool) Put(slice []byte) {
	if slice == nil {
		return
	}

	atomic.AddInt64(&sp.metrics.Puts, 1)

	// Don't keep very large slices in the pool
	if sp.maxSize > 0 && cap(slice) > sp.maxSize {
		// Decrement active count for oversized slices that won't be pooled
		atomic.AddInt32(&sp.activeSlices, -1)
		return
	}

	atomic.AddInt64(&sp.metrics.Resets, 1)

	// Conditionally zero out sensitive data if secure mode is enabled
	if sp.secureZero && len(slice) > 0 {
		for i := range slice {
			slice[i] = 0
		}
	}

	sp.pool.Put(slice[:0]) // Reset length
}

// PutSecure returns a slice to the pool with secure zeroing regardless of pool setting
func (sp *SlicePool) PutSecure(slice []byte) {
	if slice == nil {
		return
	}

	atomic.AddInt64(&sp.metrics.Puts, 1)

	// Don't keep very large slices in the pool
	if sp.maxSize > 0 && cap(slice) > sp.maxSize {
		// Decrement active count for oversized slices that won't be pooled
		atomic.AddInt32(&sp.activeSlices, -1)
		return
	}

	atomic.AddInt64(&sp.metrics.Resets, 1)

	// Always zero out contents for sensitive data
	if len(slice) > 0 {
		for i := range slice {
			slice[i] = 0
		}
	}

	sp.pool.Put(slice[:0]) // Reset length
}

// Metrics returns pool metrics
func (sp *SlicePool) Metrics() PoolMetrics {
	return PoolMetrics{
		Gets:   atomic.LoadInt64(&sp.metrics.Gets),
		Puts:   atomic.LoadInt64(&sp.metrics.Puts),
		News:   atomic.LoadInt64(&sp.metrics.News),
		Resets: atomic.LoadInt64(&sp.metrics.Resets),
	}
}

// Reset clears the pool
func (sp *SlicePool) Reset() {
	sp.pool = sync.Pool{
		New: func() interface{} {
			// Check if we're exceeding the slice limit
			if current := atomic.LoadInt32(&sp.activeSlices); current >= sp.maxSlices {
				return nil // Return nil to indicate resource exhaustion
			}
			atomic.AddInt32(&sp.activeSlices, 1)
			atomic.AddInt64(&sp.metrics.News, 1)
			return make([]byte, 0, 1024)
		},
	}
	atomic.StoreInt64(&sp.metrics.Gets, 0)
	atomic.StoreInt64(&sp.metrics.Puts, 0)
	atomic.StoreInt64(&sp.metrics.News, 0)
	atomic.StoreInt64(&sp.metrics.Resets, 0)
	atomic.StoreInt32(&sp.activeSlices, 0)
}

// ErrorPool manages a pool of error objects
type ErrorPool struct {
	encodingPool      sync.Pool
	decodingPool      sync.Pool
	operationPool     sync.Pool
	validationPool    sync.Pool
	configurationPool sync.Pool
	resourcePool      sync.Pool
	registryPool      sync.Pool
	metrics           PoolMetrics
}

// NewErrorPool creates a new error pool
func NewErrorPool() *ErrorPool {
	ep := &ErrorPool{}
	ep.encodingPool.New = func() interface{} {
		atomic.AddInt64(&ep.metrics.News, 1)
		return &EncodingError{}
	}
	ep.decodingPool.New = func() interface{} {
		atomic.AddInt64(&ep.metrics.News, 1)
		return &DecodingError{}
	}
	ep.operationPool.New = func() interface{} {
		atomic.AddInt64(&ep.metrics.News, 1)
		return &OperationError{}
	}
	ep.validationPool.New = func() interface{} {
		atomic.AddInt64(&ep.metrics.News, 1)
		return &ValidationError{}
	}
	ep.configurationPool.New = func() interface{} {
		atomic.AddInt64(&ep.metrics.News, 1)
		return &ConfigurationError{}
	}
	ep.resourcePool.New = func() interface{} {
		atomic.AddInt64(&ep.metrics.News, 1)
		return &ResourceError{}
	}
	ep.registryPool.New = func() interface{} {
		atomic.AddInt64(&ep.metrics.News, 1)
		return &RegistryError{}
	}
	return ep
}

// GetEncodingError retrieves an encoding error from the pool
func (ep *ErrorPool) GetEncodingError() *EncodingError {
	atomic.AddInt64(&ep.metrics.Gets, 1)
	err := ep.encodingPool.Get().(*EncodingError)
	err.Reset()
	return err
}

// PutEncodingError returns an encoding error to the pool
func (ep *ErrorPool) PutEncodingError(err *EncodingError) {
	if err == nil {
		return
	}
	atomic.AddInt64(&ep.metrics.Puts, 1)
	atomic.AddInt64(&ep.metrics.Resets, 1)
	err.Reset()
	ep.encodingPool.Put(err)
}

// GetDecodingError retrieves a decoding error from the pool
func (ep *ErrorPool) GetDecodingError() *DecodingError {
	atomic.AddInt64(&ep.metrics.Gets, 1)
	err := ep.decodingPool.Get().(*DecodingError)
	err.Reset()
	return err
}

// PutDecodingError returns a decoding error to the pool
func (ep *ErrorPool) PutDecodingError(err *DecodingError) {
	if err == nil {
		return
	}
	atomic.AddInt64(&ep.metrics.Puts, 1)
	atomic.AddInt64(&ep.metrics.Resets, 1)
	err.Reset()
	ep.decodingPool.Put(err)
}

// GetOperationError retrieves an operation error from the pool
func (ep *ErrorPool) GetOperationError() *OperationError {
	atomic.AddInt64(&ep.metrics.Gets, 1)
	err := ep.operationPool.Get().(*OperationError)
	err.Reset()
	return err
}

// PutOperationError returns an operation error to the pool
func (ep *ErrorPool) PutOperationError(err *OperationError) {
	if err == nil {
		return
	}
	atomic.AddInt64(&ep.metrics.Puts, 1)
	atomic.AddInt64(&ep.metrics.Resets, 1)
	err.Reset()
	ep.operationPool.Put(err)
}

// GetValidationError retrieves a validation error from the pool
func (ep *ErrorPool) GetValidationError() *ValidationError {
	atomic.AddInt64(&ep.metrics.Gets, 1)
	err := ep.validationPool.Get().(*ValidationError)
	err.Reset()
	return err
}

// PutValidationError returns a validation error to the pool
func (ep *ErrorPool) PutValidationError(err *ValidationError) {
	if err == nil {
		return
	}
	atomic.AddInt64(&ep.metrics.Puts, 1)
	atomic.AddInt64(&ep.metrics.Resets, 1)
	err.Reset()
	ep.validationPool.Put(err)
}

// GetConfigurationError retrieves a configuration error from the pool
func (ep *ErrorPool) GetConfigurationError() *ConfigurationError {
	atomic.AddInt64(&ep.metrics.Gets, 1)
	err := ep.configurationPool.Get().(*ConfigurationError)
	err.Reset()
	return err
}

// PutConfigurationError returns a configuration error to the pool
func (ep *ErrorPool) PutConfigurationError(err *ConfigurationError) {
	if err == nil {
		return
	}
	atomic.AddInt64(&ep.metrics.Puts, 1)
	atomic.AddInt64(&ep.metrics.Resets, 1)
	err.Reset()
	ep.configurationPool.Put(err)
}

// GetResourceError retrieves a resource error from the pool
func (ep *ErrorPool) GetResourceError() *ResourceError {
	atomic.AddInt64(&ep.metrics.Gets, 1)
	err := ep.resourcePool.Get().(*ResourceError)
	err.Reset()
	return err
}

// PutResourceError returns a resource error to the pool
func (ep *ErrorPool) PutResourceError(err *ResourceError) {
	if err == nil {
		return
	}
	atomic.AddInt64(&ep.metrics.Puts, 1)
	atomic.AddInt64(&ep.metrics.Resets, 1)
	err.Reset()
	ep.resourcePool.Put(err)
}

// GetRegistryError retrieves a registry error from the pool
func (ep *ErrorPool) GetRegistryError() *RegistryError {
	atomic.AddInt64(&ep.metrics.Gets, 1)
	err := ep.registryPool.Get().(*RegistryError)
	err.Reset()
	return err
}

// PutRegistryError returns a registry error to the pool
func (ep *ErrorPool) PutRegistryError(err *RegistryError) {
	if err == nil {
		return
	}
	atomic.AddInt64(&ep.metrics.Puts, 1)
	atomic.AddInt64(&ep.metrics.Resets, 1)
	err.Reset()
	ep.registryPool.Put(err)
}

// Metrics returns pool metrics
func (ep *ErrorPool) Metrics() PoolMetrics {
	return PoolMetrics{
		Gets:   atomic.LoadInt64(&ep.metrics.Gets),
		Puts:   atomic.LoadInt64(&ep.metrics.Puts),
		News:   atomic.LoadInt64(&ep.metrics.News),
		Resets: atomic.LoadInt64(&ep.metrics.Resets),
	}
}

// Reset clears the pool
func (ep *ErrorPool) Reset() {
	ep.encodingPool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&ep.metrics.News, 1)
			return &EncodingError{}
		},
	}
	ep.decodingPool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&ep.metrics.News, 1)
			return &DecodingError{}
		},
	}
	ep.operationPool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&ep.metrics.News, 1)
			return &OperationError{}
		},
	}
	ep.validationPool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&ep.metrics.News, 1)
			return &ValidationError{}
		},
	}
	ep.configurationPool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&ep.metrics.News, 1)
			return &ConfigurationError{}
		},
	}
	ep.resourcePool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&ep.metrics.News, 1)
			return &ResourceError{}
		},
	}
	ep.registryPool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&ep.metrics.News, 1)
			return &RegistryError{}
		},
	}
	atomic.StoreInt64(&ep.metrics.Gets, 0)
	atomic.StoreInt64(&ep.metrics.Puts, 0)
	atomic.StoreInt64(&ep.metrics.News, 0)
	atomic.StoreInt64(&ep.metrics.Resets, 0)
}

// Poolable interface for objects that can be pooled
type Poolable interface {
	Reset()
}

// Reset methods for error types
func (e *EncodingError) Reset() {
	e.Format = ""
	e.Event = nil
	e.Message = ""
	e.Cause = nil
}

func (e *DecodingError) Reset() {
	e.Format = ""
	e.Data = nil
	e.Message = ""
	e.Cause = nil
}

// Global pools for common objects
var (
	// Buffer pools with different size limits and capacity limits (secure by default)
	smallBufferPool  = NewBufferPoolWithOptions(4096, 500, true)   // 4KB max, 500 buffers, secure
	mediumBufferPool = NewBufferPoolWithOptions(65536, 200, true)  // 64KB max, 200 buffers, secure
	largeBufferPool  = NewBufferPoolWithOptions(1048576, 50, true) // 1MB max, 50 buffers, secure

	// Slice pools for different sizes with capacity limits (secure by default)
	smallSlicePool  = NewSlicePoolWithOptions(1024, 4096, 500, true)    // 1KB initial, 4KB max, 500 slices, secure
	mediumSlicePool = NewSlicePoolWithOptions(4096, 65536, 200, true)   // 4KB initial, 64KB max, 200 slices, secure
	largeSlicePool  = NewSlicePoolWithOptions(16384, 1048576, 50, true) // 16KB initial, 1MB max, 50 slices, secure

	// Error pool
	errorPool = NewErrorPool()
)

// GetBuffer returns a buffer from the appropriate pool based on expected size
// Returns nil if resource limits are exceeded
func GetBuffer(expectedSize int) *bytes.Buffer {
	switch {
	case expectedSize <= 4096:
		return smallBufferPool.Get()
	case expectedSize <= 65536:
		return mediumBufferPool.Get()
	default:
		return largeBufferPool.Get()
	}
}

// GetBufferSafe returns a buffer from the appropriate pool or creates a new one if pool is exhausted
func GetBufferSafe(expectedSize int) *bytes.Buffer {
	buf := GetBuffer(expectedSize)
	if buf == nil {
		// Pool exhausted, create a new buffer but don't exceed reasonable limits
		if expectedSize > 100*1024*1024 { // 100MB limit
			return nil
		}
		return &bytes.Buffer{}
	}
	return buf
}

// PutBuffer returns a buffer to the appropriate pool
func PutBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	// The individual pool's Put method will handle zeroing
	switch {
	case buf.Cap() <= 4096:
		smallBufferPool.Put(buf)
	case buf.Cap() <= 65536:
		mediumBufferPool.Put(buf)
	default:
		largeBufferPool.Put(buf)
	}
}

// PutBufferSecure returns a buffer to the appropriate pool with secure zeroing
func PutBufferSecure(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	switch {
	case buf.Cap() <= 4096:
		smallBufferPool.PutSecure(buf)
	case buf.Cap() <= 65536:
		mediumBufferPool.PutSecure(buf)
	default:
		largeBufferPool.PutSecure(buf)
	}
}

// GetSlice returns a slice from the appropriate pool based on expected size
// Returns nil if resource limits are exceeded
func GetSlice(expectedSize int) []byte {
	switch {
	case expectedSize <= 4096:
		return smallSlicePool.Get()
	case expectedSize <= 65536:
		return mediumSlicePool.Get()
	default:
		return largeSlicePool.Get()
	}
}

// GetSliceSafe returns a slice from the appropriate pool or creates a new one if pool is exhausted
func GetSliceSafe(expectedSize int) []byte {
	slice := GetSlice(expectedSize)
	if slice == nil {
		// Pool exhausted, create a new slice but don't exceed reasonable limits
		if expectedSize > 100*1024*1024 { // 100MB limit
			return nil
		}
		return make([]byte, 0, expectedSize)
	}
	return slice
}

// PutSlice returns a slice to the appropriate pool
func PutSlice(slice []byte) {
	if slice == nil {
		return
	}

	// The individual pool's Put method will handle zeroing
	switch {
	case cap(slice) <= 4096:
		smallSlicePool.Put(slice)
	case cap(slice) <= 65536:
		mediumSlicePool.Put(slice)
	default:
		largeSlicePool.Put(slice)
	}
}

// PutSliceSecure returns a slice to the appropriate pool with secure zeroing
func PutSliceSecure(slice []byte) {
	if slice == nil {
		return
	}

	switch {
	case cap(slice) <= 4096:
		smallSlicePool.PutSecure(slice)
	case cap(slice) <= 65536:
		mediumSlicePool.PutSecure(slice)
	default:
		largeSlicePool.PutSecure(slice)
	}
}

// GetEncodingError returns an encoding error from the pool
func GetEncodingError() *EncodingError {
	return errorPool.GetEncodingError()
}

// PutEncodingError returns an encoding error to the pool
func PutEncodingError(err *EncodingError) {
	errorPool.PutEncodingError(err)
}

// GetDecodingError returns a decoding error from the pool
func GetDecodingError() *DecodingError {
	return errorPool.GetDecodingError()
}

// PutDecodingError returns a decoding error to the pool
func PutDecodingError(err *DecodingError) {
	errorPool.PutDecodingError(err)
}

// GetOperationError returns an operation error from the pool
func GetOperationError() *OperationError {
	return errorPool.GetOperationError()
}

// PutOperationError returns an operation error to the pool
func PutOperationError(err *OperationError) {
	errorPool.PutOperationError(err)
}

// GetValidationError returns a validation error from the pool
func GetValidationError() *ValidationError {
	return errorPool.GetValidationError()
}

// PutValidationError returns a validation error to the pool
func PutValidationError(err *ValidationError) {
	errorPool.PutValidationError(err)
}

// GetConfigurationError returns a configuration error from the pool
func GetConfigurationError() *ConfigurationError {
	return errorPool.GetConfigurationError()
}

// PutConfigurationError returns a configuration error to the pool
func PutConfigurationError(err *ConfigurationError) {
	errorPool.PutConfigurationError(err)
}

// GetResourceError returns a resource error from the pool
func GetResourceError() *ResourceError {
	return errorPool.GetResourceError()
}

// PutResourceError returns a resource error to the pool
func PutResourceError(err *ResourceError) {
	errorPool.PutResourceError(err)
}

// GetRegistryError returns a registry error from the pool
func GetRegistryError() *RegistryError {
	return errorPool.GetRegistryError()
}

// PutRegistryError returns a registry error to the pool
func PutRegistryError(err *RegistryError) {
	errorPool.PutRegistryError(err)
}

// PoolStats returns statistics for all pools
func PoolStats() map[string]PoolMetrics {
	return map[string]PoolMetrics{
		"small_buffer":  smallBufferPool.Metrics(),
		"medium_buffer": mediumBufferPool.Metrics(),
		"large_buffer":  largeBufferPool.Metrics(),
		"small_slice":   smallSlicePool.Metrics(),
		"medium_slice":  mediumSlicePool.Metrics(),
		"large_slice":   largeSlicePool.Metrics(),
		"error":         errorPool.Metrics(),
	}
}

// ResetAllPools resets all global pools
func ResetAllPools() {
	smallBufferPool.Reset()
	mediumBufferPool.Reset()
	largeBufferPool.Reset()
	smallSlicePool.Reset()
	mediumSlicePool.Reset()
	largeSlicePool.Reset()
	errorPool.Reset()
}

// PoolManager manages lifecycle of pools
type PoolManager struct {
	pools   map[string]interface{}
	metrics map[string]*PoolMetrics
	mu      sync.RWMutex
}

// NewPoolManager creates a new pool manager
func NewPoolManager() *PoolManager {
	return &PoolManager{
		pools:   make(map[string]interface{}),
		metrics: make(map[string]*PoolMetrics),
	}
}

// RegisterPool registers a pool with the manager
func (pm *PoolManager) RegisterPool(name string, pool interface{}) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.pools[name] = pool
}

// GetPool retrieves a pool by name
func (pm *PoolManager) GetPool(name string) interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.pools[name]
}

// GetAllMetrics returns metrics for all registered pools
func (pm *PoolManager) GetAllMetrics() map[string]PoolMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metrics := make(map[string]PoolMetrics)
	for name, pool := range pm.pools {
		if p, ok := pool.(interface{ Metrics() PoolMetrics }); ok {
			metrics[name] = p.Metrics()
		}
	}
	return metrics
}

// StartMonitoring starts periodic monitoring of pools
func (pm *PoolManager) StartMonitoring(interval time.Duration) <-chan map[string]PoolMetrics {
	ch := make(chan map[string]PoolMetrics, 1)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				metrics := pm.GetAllMetrics()
				select {
				case ch <- metrics:
				default:
					// Channel is full, skip this iteration
				}
			}
		}
	}()

	return ch
}
