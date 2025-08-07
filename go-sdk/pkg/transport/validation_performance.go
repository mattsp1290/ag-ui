package transport

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CachedValidator implements a validator with caching for performance
type CachedValidator struct {
	validator    Validator
	cache        map[string]error
	cacheMutex   sync.RWMutex
	maxCacheSize int
	cacheTTL     time.Duration
	cacheStats   *CacheStats
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	mu           sync.RWMutex
	Hits         uint64
	Misses       uint64
	Size         int
	MaxSize      int
	Evictions    uint64
	TotalOps     uint64
	HitRate      float64
	LastHitTime  time.Time
	LastMissTime time.Time
}

// NewCachedValidator creates a new cached validator
func NewCachedValidator(validator Validator, maxCacheSize int, cacheTTL time.Duration) *CachedValidator {
	return &CachedValidator{
		validator:    validator,
		cache:        make(map[string]error),
		maxCacheSize: maxCacheSize,
		cacheTTL:     cacheTTL,
		cacheStats:   &CacheStats{MaxSize: maxCacheSize},
	}
}

// Validate validates with caching
func (cv *CachedValidator) Validate(ctx context.Context, event TransportEvent) error {
	// Generate cache key from event
	cacheKey := cv.generateCacheKey(event)

	// Check cache first
	cv.cacheMutex.RLock()
	cachedResult, exists := cv.cache[cacheKey]
	cv.cacheMutex.RUnlock()

	if exists {
		cv.updateStats(true)
		return cachedResult
	}

	// Cache miss - validate and cache result
	cv.updateStats(false)
	result := cv.validator.Validate(ctx, event)

	// Cache the result
	cv.cacheMutex.Lock()
	if len(cv.cache) >= cv.maxCacheSize {
		cv.evictOldest()
	}
	cv.cache[cacheKey] = result
	cv.cacheMutex.Unlock()

	return result
}

// ValidateIncoming validates incoming events with caching
func (cv *CachedValidator) ValidateIncoming(ctx context.Context, event TransportEvent) error {
	return cv.validator.ValidateIncoming(ctx, event)
}

// ValidateOutgoing validates outgoing events with caching
func (cv *CachedValidator) ValidateOutgoing(ctx context.Context, event TransportEvent) error {
	return cv.validator.ValidateOutgoing(ctx, event)
}

// generateCacheKey generates a cache key for the event
func (cv *CachedValidator) generateCacheKey(event TransportEvent) string {
	// Use event type and data hash as cache key
	return event.Type() + "_" + cv.hashEventData(event)
}

// hashEventData creates a hash of the event data for caching
func (cv *CachedValidator) hashEventData(event TransportEvent) string {
	// Simple hash based on data structure
	data := event.Data()
	if data == nil {
		return "empty"
	}

	// Create a more deterministic hash
	return fmt.Sprintf("%s_%d", event.Type(), len(data))
}

// getTypeString returns a string representation of the type
func getTypeString(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int32, int64, float32, float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

// evictOldest removes the oldest cache entry
func (cv *CachedValidator) evictOldest() {
	// Simple eviction - remove first entry
	// In a real implementation, you'd track access times
	for key := range cv.cache {
		delete(cv.cache, key)
		cv.cacheStats.mu.Lock()
		cv.cacheStats.Evictions++
		cv.cacheStats.mu.Unlock()
		break
	}
}

// updateStats updates cache statistics
func (cv *CachedValidator) updateStats(hit bool) {
	cv.cacheStats.mu.Lock()
	defer cv.cacheStats.mu.Unlock()

	cv.cacheStats.TotalOps++
	if hit {
		cv.cacheStats.Hits++
		cv.cacheStats.LastHitTime = time.Now()
	} else {
		cv.cacheStats.Misses++
		cv.cacheStats.LastMissTime = time.Now()
	}

	if cv.cacheStats.TotalOps > 0 {
		cv.cacheStats.HitRate = float64(cv.cacheStats.Hits) / float64(cv.cacheStats.TotalOps)
	}

	cv.cacheStats.Size = len(cv.cache)
}

// GetCacheStats returns cache statistics
func (cv *CachedValidator) GetCacheStats() CacheStats {
	cv.cacheStats.mu.RLock()
	defer cv.cacheStats.mu.RUnlock()
	return CacheStats{
		Hits:         cv.cacheStats.Hits,
		Misses:       cv.cacheStats.Misses,
		Size:         cv.cacheStats.Size,
		MaxSize:      cv.cacheStats.MaxSize,
		Evictions:    cv.cacheStats.Evictions,
		TotalOps:     cv.cacheStats.TotalOps,
		HitRate:      cv.cacheStats.HitRate,
		LastHitTime:  cv.cacheStats.LastHitTime,
		LastMissTime: cv.cacheStats.LastMissTime,
	}
}

// ClearCache clears the validation cache
func (cv *CachedValidator) ClearCache() {
	cv.cacheMutex.Lock()
	defer cv.cacheMutex.Unlock()

	cv.cache = make(map[string]error)
	cv.cacheStats.mu.Lock()
	cv.cacheStats.Size = 0
	cv.cacheStats.mu.Unlock()
}

// FastValidator implements a lightweight validator for high-throughput scenarios
type FastValidator struct {
	config                *ValidationConfig
	enabledRules          []string
	maxMessageSize        int64
	requiredFields        []string
	allowedTypes          map[string]bool
	skipComplexValidation bool
}

// NewFastValidator creates a new fast validator
func NewFastValidator(config *ValidationConfig) *FastValidator {
	if config == nil {
		config = &ValidationConfig{
			Enabled:        true,
			MaxMessageSize: 1024 * 1024, // 1MB
			RequiredFields: []string{"id", "type"},
		}
	}

	allowedTypes := make(map[string]bool)
	for _, t := range config.AllowedEventTypes {
		allowedTypes[t] = true
	}

	return &FastValidator{
		config:                config,
		maxMessageSize:        config.MaxMessageSize,
		requiredFields:        config.RequiredFields,
		allowedTypes:          allowedTypes,
		skipComplexValidation: true,
	}
}

// Validate performs fast validation
func (fv *FastValidator) Validate(ctx context.Context, event TransportEvent) error {
	if !fv.config.Enabled {
		return nil
	}

	// Fast path validations only

	// 1. Check event type
	if len(fv.allowedTypes) > 0 && !fv.allowedTypes[event.Type()] {
		return NewValidationError("event type not allowed", nil)
	}

	// 2. Check required fields (fast)
	data := event.Data()
	for _, field := range fv.requiredFields {
		if _, exists := data[field]; !exists {
			return NewValidationError("missing required field: "+field, nil)
		}
	}

	// 3. Rough size check (without serialization)
	if fv.maxMessageSize > 0 {
		estimatedSize := fv.estimateSize(data)
		if estimatedSize > fv.maxMessageSize {
			return NewValidationError("estimated message size exceeds limit", nil)
		}
	}

	return nil
}

// ValidateIncoming validates incoming events
func (fv *FastValidator) ValidateIncoming(ctx context.Context, event TransportEvent) error {
	if fv.config.SkipValidationOnIncoming {
		return nil
	}
	return fv.Validate(ctx, event)
}

// ValidateOutgoing validates outgoing events
func (fv *FastValidator) ValidateOutgoing(ctx context.Context, event TransportEvent) error {
	if fv.config.SkipValidationOnOutgoing {
		return nil
	}
	return fv.Validate(ctx, event)
}

// estimateSize estimates the serialized size without actual serialization
func (fv *FastValidator) estimateSize(data map[string]interface{}) int64 {
	var size int64
	for key, value := range data {
		size += int64(len(key)) + fv.estimateValueSize(value)
	}
	return size
}

// estimateValueSize estimates the size of a value
func (fv *FastValidator) estimateValueSize(value interface{}) int64 {
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case int, int32, int64, float32, float64:
		return 8 // Approximate
	case bool:
		return 1
	case []interface{}:
		var size int64
		for _, item := range v {
			size += fv.estimateValueSize(item)
		}
		return size
	case map[string]interface{}:
		var size int64
		for key, val := range v {
			size += int64(len(key)) + fv.estimateValueSize(val)
		}
		return size
	default:
		return 100 // Conservative estimate
	}
}

// ValidationPool manages a pool of validators for concurrent validation
type ValidationPool struct {
	validators chan Validator
	factory    func() Validator
	maxSize    int
	created    int
	mu         sync.Mutex
}

// NewValidationPool creates a new validation pool
func NewValidationPool(maxSize int, factory func() Validator) *ValidationPool {
	return &ValidationPool{
		validators: make(chan Validator, maxSize),
		factory:    factory,
		maxSize:    maxSize,
	}
}

// Get gets a validator from the pool
func (vp *ValidationPool) Get() Validator {
	select {
	case validator := <-vp.validators:
		return validator
	default:
		vp.mu.Lock()
		if vp.created < vp.maxSize {
			vp.created++
			vp.mu.Unlock()
			return vp.factory()
		}
		vp.mu.Unlock()
		// Pool is full, create a new one anyway
		return vp.factory()
	}
}

// Put returns a validator to the pool
func (vp *ValidationPool) Put(validator Validator) {
	select {
	case vp.validators <- validator:
	default:
		// Pool is full, discard the validator
	}
}

// BatchValidator validates multiple events in batches
type BatchValidator struct {
	validator Validator
	batchSize int
}

// NewBatchValidator creates a new batch validator
func NewBatchValidator(validator Validator, batchSize int) *BatchValidator {
	return &BatchValidator{
		validator: validator,
		batchSize: batchSize,
	}
}

// ValidateBatch validates a batch of events
func (bv *BatchValidator) ValidateBatch(ctx context.Context, events []TransportEvent) []error {
	errors := make([]error, len(events))

	// Process in batches to avoid overwhelming the system
	for i := 0; i < len(events); i += bv.batchSize {
		end := i + bv.batchSize
		if end > len(events) {
			end = len(events)
		}

		// Validate batch
		for j := i; j < end; j++ {
			errors[j] = bv.validator.Validate(ctx, events[j])
		}

		// Check for context cancellation
		if ctx.Err() != nil {
			return errors
		}
	}

	return errors
}

// AsynchronousValidator validates events asynchronously
type AsynchronousValidator struct {
	validator Validator
	workers   int
	queue     chan validationTask
	results   chan validationResult
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

type validationTask struct {
	event TransportEvent
	id    string
}

type validationResult struct {
	id    string
	error error
}

// NewAsynchronousValidator creates a new asynchronous validator
func NewAsynchronousValidator(validator Validator, workers int, queueSize int) *AsynchronousValidator {
	ctx, cancel := context.WithCancel(context.Background())

	av := &AsynchronousValidator{
		validator: validator,
		workers:   workers,
		queue:     make(chan validationTask, queueSize),
		results:   make(chan validationResult, queueSize),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		av.wg.Add(1)
		go av.worker()
	}

	return av
}

// worker processes validation tasks
func (av *AsynchronousValidator) worker() {
	defer av.wg.Done()

	for {
		select {
		case task := <-av.queue:
			result := validationResult{
				id:    task.id,
				error: av.validator.Validate(av.ctx, task.event),
			}

			select {
			case av.results <- result:
			case <-av.ctx.Done():
				return
			}
		case <-av.ctx.Done():
			return
		}
	}
}

// ValidateAsync validates an event asynchronously
func (av *AsynchronousValidator) ValidateAsync(event TransportEvent, id string) error {
	task := validationTask{
		event: event,
		id:    id,
	}

	select {
	case av.queue <- task:
		return nil
	case <-av.ctx.Done():
		return av.ctx.Err()
	default:
		return NewValidationError("validation queue full", nil)
	}
}

// GetResult gets a validation result
func (av *AsynchronousValidator) GetResult() (string, error, bool) {
	select {
	case result := <-av.results:
		return result.id, result.error, true
	case <-av.ctx.Done():
		return "", av.ctx.Err(), false
	default:
		return "", nil, false
	}
}

// Close closes the asynchronous validator
func (av *AsynchronousValidator) Close() {
	av.cancel()
	close(av.queue)
	av.wg.Wait()
	close(av.results)
}
