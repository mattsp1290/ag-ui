package events

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2"
)

// PerformanceMode defines the validation performance mode
type PerformanceMode int

const (
	// FastMode prioritizes speed over thoroughness
	FastMode PerformanceMode = iota
	// BalancedMode balances speed and thoroughness
	BalancedMode
	// ThoroughMode prioritizes thoroughness over speed
	ThoroughMode
)

// PerformanceConfig holds performance tuning parameters
type PerformanceConfig struct {
	Mode            PerformanceMode
	CacheTTL        time.Duration
	CacheSize       int
	WorkerPoolSize  int
	BatchSize       int
	MaxConcurrency  int
	EnableHotPath   bool
	EnableAsync     bool
	MemoryPoolSize  int
	ResourceMonitor bool
	MonitorInterval time.Duration
}

// DefaultPerformanceConfig returns a balanced performance configuration
func DefaultPerformanceConfig() *PerformanceConfig {
	return &PerformanceConfig{
		Mode:            BalancedMode,
		CacheTTL:        5 * time.Minute,
		CacheSize:       10000,
		WorkerPoolSize:  runtime.NumCPU() * 2,
		BatchSize:       100,
		MaxConcurrency:  runtime.NumCPU() * 4,
		EnableHotPath:   true,
		EnableAsync:     true,
		MemoryPoolSize:  1000,
		ResourceMonitor: true,
		MonitorInterval: 30 * time.Second,
	}
}

// CachedValidationResult represents a cached validation result
type CachedValidationResult struct {
	Valid     bool
	Errors    []error
	Timestamp time.Time
	HitCount  uint64
}

// ValidationResultCache implements an LRU cache for validation results
type ValidationResultCache struct {
	cache     *lru.Cache[string, *CachedValidationResult]
	ttl       time.Duration
	mu        sync.RWMutex
	hits      uint64
	misses    uint64
	evictions uint64
}

// NewValidationResultCache creates a new validation result cache
func NewValidationResultCache(size int, ttl time.Duration) (*ValidationResultCache, error) {
	cache, err := lru.New[string, *CachedValidationResult](size)
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	}

	return &ValidationResultCache{
		cache: cache,
		ttl:   ttl,
	}, nil
}

// Get retrieves a validation result from the cache
func (c *ValidationResultCache) Get(key string) (*CachedValidationResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result, ok := c.cache.Get(key)
	if !ok {
		atomic.AddUint64(&c.misses, 1)
		return nil, false
	}

	// Check TTL
	if time.Since(result.Timestamp) > c.ttl {
		c.cache.Remove(key)
		atomic.AddUint64(&c.misses, 1)
		return nil, false
	}

	atomic.AddUint64(&result.HitCount, 1)
	atomic.AddUint64(&c.hits, 1)
	return result, true
}

// Set stores a validation result in the cache
func (c *ValidationResultCache) Set(key string, valid bool, errors []error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	evicted := c.cache.Add(key, &CachedValidationResult{
		Valid:     valid,
		Errors:    errors,
		Timestamp: time.Now(),
		HitCount:  0,
	})

	if evicted {
		atomic.AddUint64(&c.evictions, 1)
	}
}

// Stats returns cache statistics
func (c *ValidationResultCache) Stats() (hits, misses, evictions uint64) {
	return atomic.LoadUint64(&c.hits),
		atomic.LoadUint64(&c.misses),
		atomic.LoadUint64(&c.evictions)
}

// Clear empties the cache
func (c *ValidationResultCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Purge()
}

// WorkerPool manages a pool of workers for parallel validation
type WorkerPool struct {
	workers    int
	jobQueue   chan ValidationJob
	resultChan chan ValidationJobResult
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	stopOnce   sync.Once
}

// ValidationJob represents a validation task
type ValidationJob struct {
	ID      string
	Event   Event
	Rules   []ValidationRule
	Context context.Context
}

// ValidationJobResult represents the result of a validation job
type ValidationJobResult struct {
	ID     string
	Valid  bool
	Errors []error
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		workers:    workers,
		jobQueue:   make(chan ValidationJob, queueSize),
		resultChan: make(chan ValidationJobResult, queueSize),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start initializes and starts the worker pool
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() {
	wp.stopOnce.Do(func() {
		close(wp.jobQueue)
		wp.cancel()
		wp.wg.Wait()
		close(wp.resultChan)
	})
}

// Submit adds a validation job to the queue
func (wp *WorkerPool) Submit(job ValidationJob) error {
	select {
	case wp.jobQueue <- job:
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	default:
		return fmt.Errorf("job queue is full")
	}
}

// Results returns the result channel
func (wp *WorkerPool) Results() <-chan ValidationJobResult {
	return wp.resultChan
}

// worker processes validation jobs
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	for {
		select {
		case job, ok := <-wp.jobQueue:
			if !ok {
				return
			}
			wp.processJob(job)
		case <-wp.ctx.Done():
			return
		}
	}
}

// processJob executes a validation job
func (wp *WorkerPool) processJob(job ValidationJob) {
	var errors []error
	valid := true

	// Create validation context for the job
	validationContext := &ValidationContext{
		CurrentEvent: job.Event,
		State:        NewValidationState(),
	}

	for _, rule := range job.Rules {
		result := rule.Validate(job.Event, validationContext)
		if result.HasErrors() {
			for _, err := range result.Errors {
				errors = append(errors, err)
			}
			valid = false
		}
	}

	select {
	case wp.resultChan <- ValidationJobResult{
		ID:     job.ID,
		Valid:  valid,
		Errors: errors,
	}:
	case <-job.Context.Done():
		// Job was cancelled
	}
}

// MemoryPool manages reusable objects to reduce allocations
type MemoryPool struct {
	errorSlicePool sync.Pool
	bufferPool     sync.Pool
}

// NewMemoryPool creates a new memory pool
func NewMemoryPool(size int) *MemoryPool {
	return &MemoryPool{
		errorSlicePool: sync.Pool{
			New: func() interface{} {
				return make([]error, 0, 10)
			},
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 1024)
			},
		},
	}
}

// GetErrorSlice retrieves an error slice from the pool
func (mp *MemoryPool) GetErrorSlice() []error {
	return mp.errorSlicePool.Get().([]error)[:0]
}

// PutErrorSlice returns an error slice to the pool
func (mp *MemoryPool) PutErrorSlice(errors []error) {
	mp.errorSlicePool.Put(errors)
}

// GetBuffer retrieves a byte buffer from the pool
func (mp *MemoryPool) GetBuffer() []byte {
	return mp.bufferPool.Get().([]byte)[:0]
}

// PutBuffer returns a byte buffer to the pool
func (mp *MemoryPool) PutBuffer(buf []byte) {
	mp.bufferPool.Put(buf)
}

// HotPathOptimizer tracks and optimizes frequently used validation paths
type HotPathOptimizer struct {
	paths     map[string]*HotPath
	mu        sync.RWMutex
	threshold uint64
}

// HotPath represents a frequently used validation path
type HotPath struct {
	Key       string
	Count     uint64
	LastUsed  time.Time
	Optimized bool
	FastPath  func(Event) error
}

// NewHotPathOptimizer creates a new hot path optimizer
func NewHotPathOptimizer(threshold uint64) *HotPathOptimizer {
	return &HotPathOptimizer{
		paths:     make(map[string]*HotPath),
		threshold: threshold,
	}
}

// Track records usage of a validation path
func (hpo *HotPathOptimizer) Track(key string) {
	hpo.mu.Lock()
	defer hpo.mu.Unlock()

	path, exists := hpo.paths[key]
	if !exists {
		path = &HotPath{
			Key:      key,
			Count:    0,
			LastUsed: time.Now(),
		}
		hpo.paths[key] = path
	}

	atomic.AddUint64(&path.Count, 1)
	path.LastUsed = time.Now()

	// Check if optimization is needed
	if path.Count >= hpo.threshold && !path.Optimized {
		hpo.optimize(path)
	}
}

// GetFastPath retrieves an optimized validation function if available
func (hpo *HotPathOptimizer) GetFastPath(key string) (func(Event) error, bool) {
	hpo.mu.RLock()
	defer hpo.mu.RUnlock()

	path, exists := hpo.paths[key]
	if !exists || !path.Optimized || path.FastPath == nil {
		return nil, false
	}

	return path.FastPath, true
}

// optimize creates an optimized validation function for a hot path
func (hpo *HotPathOptimizer) optimize(path *HotPath) {
	// Create optimized validation function based on path characteristics
	// This is a simplified example - real optimization would analyze the path
	path.FastPath = func(e Event) error {
		// Fast validation logic
		if e == nil {
			return fmt.Errorf("event is nil")
		}
		if e.Type() == "" {
			return fmt.Errorf("event type is empty")
		}
		return nil
	}
	path.Optimized = true
}

// BatchProcessor handles batch validation of events
type BatchProcessor struct {
	batchSize int
	timeout   time.Duration
	processor func([]Event) []ValidationJobResult
	input     chan Event
	output    chan ValidationJobResult
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(batchSize int, timeout time.Duration, processor func([]Event) []ValidationJobResult) *BatchProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	return &BatchProcessor{
		batchSize: batchSize,
		timeout:   timeout,
		processor: processor,
		input:     make(chan Event, batchSize*2),
		output:    make(chan ValidationJobResult, batchSize*2),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins batch processing
func (bp *BatchProcessor) Start() {
	bp.wg.Add(1)
	go bp.processBatches()
}

// Stop gracefully shuts down the batch processor
func (bp *BatchProcessor) Stop() {
	close(bp.input)
	bp.cancel()
	bp.wg.Wait()
	close(bp.output)
}

// Submit adds an event to the batch queue
func (bp *BatchProcessor) Submit(event Event) error {
	select {
	case bp.input <- event:
		return nil
	case <-bp.ctx.Done():
		return fmt.Errorf("batch processor is shutting down")
	}
}

// Results returns the result channel
func (bp *BatchProcessor) Results() <-chan ValidationJobResult {
	return bp.output
}

// processBatches handles batch processing logic
func (bp *BatchProcessor) processBatches() {
	defer bp.wg.Done()

	batch := make([]Event, 0, bp.batchSize)
	timer := time.NewTimer(bp.timeout)
	defer timer.Stop()

	for {
		select {
		case event, ok := <-bp.input:
			if !ok {
				// Process remaining batch
				if len(batch) > 0 {
					bp.processBatch(batch)
				}
				return
			}

			batch = append(batch, event)
			if len(batch) >= bp.batchSize {
				bp.processBatch(batch)
				batch = batch[:0]
				timer.Reset(bp.timeout)
			}

		case <-timer.C:
			if len(batch) > 0 {
				bp.processBatch(batch)
				batch = batch[:0]
			}
			timer.Reset(bp.timeout)

		case <-bp.ctx.Done():
			return
		}
	}
}

// processBatch executes validation on a batch of events
func (bp *BatchProcessor) processBatch(batch []Event) {
	results := bp.processor(batch)
	for _, result := range results {
		select {
		case bp.output <- result:
		case <-bp.ctx.Done():
			return
		}
	}
}

// AsyncValidator provides asynchronous validation capabilities
type AsyncValidator struct {
	validator   func(Event) error
	concurrency int
	sem         chan struct{}
	wg          sync.WaitGroup
}

// NewAsyncValidator creates a new async validator
func NewAsyncValidator(validator func(Event) error, concurrency int) *AsyncValidator {
	return &AsyncValidator{
		validator:   validator,
		concurrency: concurrency,
		sem:         make(chan struct{}, concurrency),
	}
}

// ValidateAsync performs asynchronous validation
func (av *AsyncValidator) ValidateAsync(ctx context.Context, event Event) <-chan error {
	result := make(chan error, 1)

	av.wg.Add(1)
	go func() {
		defer av.wg.Done()

		// Acquire semaphore
		select {
		case av.sem <- struct{}{}:
			defer func() { <-av.sem }()
		case <-ctx.Done():
			result <- ctx.Err()
			close(result)
			return
		}

		// Perform validation
		err := av.validator(event)
		select {
		case result <- err:
		case <-ctx.Done():
		}
		close(result)
	}()

	return result
}

// Wait waits for all async validations to complete
func (av *AsyncValidator) Wait() {
	av.wg.Wait()
}

// ResourceMonitor tracks resource utilization
type ResourceMonitor struct {
	interval       time.Duration
	stopCh         chan struct{}
	metrics        *ResourceMetrics
	mu             sync.RWMutex
	alertThreshold float64
	alertFunc      func(string)
	stopOnce       sync.Once
}

// ResourceMetrics holds resource utilization metrics
type ResourceMetrics struct {
	CPUUsage       float64
	MemoryUsage    uint64
	GoroutineCount int
	HeapAlloc      uint64
	HeapObjects    uint64
	LastUpdate     time.Time
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(interval time.Duration, alertThreshold float64, alertFunc func(string)) *ResourceMonitor {
	return &ResourceMonitor{
		interval:       interval,
		stopCh:         make(chan struct{}),
		metrics:        &ResourceMetrics{},
		alertThreshold: alertThreshold,
		alertFunc:      alertFunc,
	}
}

// Start begins resource monitoring
func (rm *ResourceMonitor) Start() {
	go rm.monitor()
}

// Stop halts resource monitoring
func (rm *ResourceMonitor) Stop() {
	rm.stopOnce.Do(func() {
		close(rm.stopCh)
	})
}

// GetMetrics returns current resource metrics
func (rm *ResourceMonitor) GetMetrics() ResourceMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return *rm.metrics
}

// monitor continuously tracks resource usage
func (rm *ResourceMonitor) monitor() {
	ticker := time.NewTicker(rm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rm.updateMetrics()
		case <-rm.stopCh:
			return
		}
	}
}

// updateMetrics updates resource utilization metrics
func (rm *ResourceMonitor) updateMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	rm.mu.Lock()
	rm.metrics.MemoryUsage = m.Alloc
	rm.metrics.GoroutineCount = runtime.NumGoroutine()
	rm.metrics.HeapAlloc = m.HeapAlloc
	rm.metrics.HeapObjects = m.HeapObjects
	rm.metrics.LastUpdate = time.Now()
	rm.mu.Unlock()

	// Check thresholds and alert if necessary
	if float64(m.Alloc) > rm.alertThreshold*1024*1024*1024 { // Convert GB to bytes
		if rm.alertFunc != nil {
			rm.alertFunc(fmt.Sprintf("High memory usage: %d MB", m.Alloc/1024/1024))
		}
	}
}

// PerformanceOptimizer orchestrates all performance optimizations
type PerformanceOptimizer struct {
	config          *PerformanceConfig
	cache           *ValidationResultCache
	workerPool      *WorkerPool
	memoryPool      *MemoryPool
	hotPathOpt      *HotPathOptimizer
	batchProcessor  *BatchProcessor
	resourceMonitor *ResourceMonitor
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer(config *PerformanceConfig) (*PerformanceOptimizer, error) {
	cache, err := NewValidationResultCache(config.CacheSize, config.CacheTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	po := &PerformanceOptimizer{
		config:     config,
		cache:      cache,
		workerPool: NewWorkerPool(config.WorkerPoolSize, config.WorkerPoolSize*10),
		memoryPool: NewMemoryPool(config.MemoryPoolSize),
		hotPathOpt: NewHotPathOptimizer(100), // Optimize after 100 uses
	}

	if config.ResourceMonitor {
		po.resourceMonitor = NewResourceMonitor(
			config.MonitorInterval,
			2.0, // Alert at 2GB memory usage
			func(msg string) {
				// Log or handle alerts
				fmt.Printf("Resource Alert: %s\n", msg)
			},
		)
	}

	return po, nil
}

// Start initializes all performance components
func (po *PerformanceOptimizer) Start() {
	po.workerPool.Start()
	if po.resourceMonitor != nil {
		po.resourceMonitor.Start()
	}
}

// Stop gracefully shuts down all components
func (po *PerformanceOptimizer) Stop() {
	po.workerPool.Stop()
	if po.resourceMonitor != nil {
		po.resourceMonitor.Stop()
	}
}

// OptimizeValidation applies performance optimizations to validation
func (po *PerformanceOptimizer) OptimizeValidation(event Event, rules []ValidationRule) error {
	// Generate cache key
	cacheKey := po.generateCacheKey(event, rules)

	// Check cache first
	if result, ok := po.cache.Get(cacheKey); ok {
		if !result.Valid {
			return fmt.Errorf("cached validation errors: %v", result.Errors)
		}
		return nil
	}

	// Track hot path
	po.hotPathOpt.Track(string(event.Type()))

	// Check for optimized fast path
	if fastPath, ok := po.hotPathOpt.GetFastPath(string(event.Type())); ok && po.config.Mode == FastMode {
		err := fastPath(event)
		po.cache.Set(cacheKey, err == nil, []error{err})
		return err
	}

	// Use worker pool for parallel validation
	if po.config.Mode != FastMode && len(rules) > 5 {
		return po.parallelValidation(event, rules, cacheKey)
	}

	// Sequential validation for small rule sets
	return po.sequentialValidation(event, rules, cacheKey)
}

// generateCacheKey creates a cache key for validation results
func (po *PerformanceOptimizer) generateCacheKey(event Event, rules []ValidationRule) string {
	// Simplified key generation - in production, use a proper hash
	// Since Event is an interface, we need to extract identifying information
	eventType := event.Type()
	// Generate a unique identifier based on event type and timestamp
	timestamp := ""
	if ts := event.Timestamp(); ts != nil {
		timestamp = fmt.Sprintf("%d", *ts)
	}
	return fmt.Sprintf("%s:%s:%d", eventType, timestamp, len(rules))
}

// parallelValidation performs validation using the worker pool
func (po *PerformanceOptimizer) parallelValidation(event Event, rules []ValidationRule, cacheKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Generate a unique ID for this job
	jobID := fmt.Sprintf("%s-%d", event.Type(), time.Now().UnixNano())

	job := ValidationJob{
		ID:      jobID,
		Event:   event,
		Rules:   rules,
		Context: ctx,
	}

	if err := po.workerPool.Submit(job); err != nil {
		return fmt.Errorf("failed to submit validation job: %w", err)
	}

	select {
	case result := <-po.workerPool.Results():
		po.cache.Set(cacheKey, result.Valid, result.Errors)
		if !result.Valid {
			return fmt.Errorf("validation failed: %v", result.Errors)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("validation timeout")
	}
}

// sequentialValidation performs validation sequentially
func (po *PerformanceOptimizer) sequentialValidation(event Event, rules []ValidationRule, cacheKey string) error {
	errors := po.memoryPool.GetErrorSlice()
	defer po.memoryPool.PutErrorSlice(errors)

	// Create validation context
	validationContext := &ValidationContext{
		CurrentEvent: event,
		State:        NewValidationState(),
	}

	valid := true
	for _, rule := range rules {
		result := rule.Validate(event, validationContext)
		if result.HasErrors() {
			for _, err := range result.Errors {
				errors = append(errors, err)
			}
			valid = false
			if po.config.Mode == FastMode {
				// Fast mode: fail on first error
				break
			}
		}
	}

	po.cache.Set(cacheKey, valid, errors)

	if !valid {
		return fmt.Errorf("validation failed: %v", errors)
	}
	return nil
}
