package worker

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// WorkerManager manages goroutine lifecycle with proper context cancellation,
// panic recovery, and resource cleanup
type WorkerManager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	logger          *zap.Logger
	workerCount     int64
	maxWorkers      int64
	shutdownTimeout time.Duration
	panicHandler    PanicHandler
	metrics         *WorkerMetrics
	mu              sync.RWMutex
	workers         map[string]*WorkerInfo
	stopOnce        sync.Once
}

// WorkerInfo tracks information about a running worker
type WorkerInfo struct {
	ID        string
	Name      string
	StartTime time.Time
	Context   context.Context
	Cancel    context.CancelFunc
}

// WorkerMetrics tracks worker-related metrics
type WorkerMetrics struct {
	WorkersCreated   int64
	WorkersCompleted int64
	WorkersFailed    int64
	PanicsRecovered  int64
	WorkersActive    int64
}

// PanicHandler defines how panics should be handled
type PanicHandler interface {
	HandlePanic(workerID string, panicValue interface{}, stackTrace []byte)
}

// DefaultPanicHandler provides default panic handling with structured logging
type DefaultPanicHandler struct {
	logger *zap.Logger
}

// NewDefaultPanicHandler creates a new default panic handler
func NewDefaultPanicHandler(logger *zap.Logger) *DefaultPanicHandler {
	return &DefaultPanicHandler{logger: logger}
}

// HandlePanic logs panic information with structured fields
func (h *DefaultPanicHandler) HandlePanic(workerID string, panicValue interface{}, stackTrace []byte) {
	h.logger.Error("Worker panic recovered",
		zap.String("worker_id", workerID),
		zap.Any("panic_value", panicValue),
		zap.String("stack_trace", string(stackTrace)),
		zap.Time("timestamp", time.Now()),
	)
}

// WorkerConfig holds configuration for the worker manager
type WorkerConfig struct {
	MaxWorkers      int64
	ShutdownTimeout time.Duration
	Logger          *zap.Logger
	PanicHandler    PanicHandler
}

// DefaultWorkerConfig returns a default configuration
func DefaultWorkerConfig() *WorkerConfig {
	logger, _ := zap.NewProduction()
	return &WorkerConfig{
		MaxWorkers:      int64(runtime.NumCPU() * 10), // Allow 10x CPU cores
		ShutdownTimeout: 30 * time.Second,
		Logger:          logger,
		PanicHandler:    NewDefaultPanicHandler(logger),
	}
}

// NewWorkerManager creates a new worker manager
func NewWorkerManager(config *WorkerConfig) *WorkerManager {
	if config == nil {
		config = DefaultWorkerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerManager{
		ctx:             ctx,
		cancel:          cancel,
		logger:          config.Logger,
		maxWorkers:      config.MaxWorkers,
		shutdownTimeout: config.ShutdownTimeout,
		panicHandler:    config.PanicHandler,
		metrics:         &WorkerMetrics{},
		workers:         make(map[string]*WorkerInfo),
	}
}

// WorkerFunc represents a function that can be executed as a worker
type WorkerFunc func(ctx context.Context) error

// WorkerOptions provides options for worker execution
type WorkerOptions struct {
	Name            string
	Timeout         time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	SkipPanicRecovery bool
}

// StartWorker starts a new worker with proper lifecycle management
func (wm *WorkerManager) StartWorker(name string, fn WorkerFunc, opts *WorkerOptions) (string, error) {
	if opts == nil {
		opts = &WorkerOptions{
			Name:       name,
			MaxRetries: 0,
			RetryDelay: time.Second,
		}
	}

	// Check if we've exceeded max workers
	if atomic.LoadInt64(&wm.workerCount) >= wm.maxWorkers {
		return "", fmt.Errorf("maximum number of workers (%d) exceeded", wm.maxWorkers)
	}

	// Generate unique worker ID
	workerID := fmt.Sprintf("%s-%d-%d", name, time.Now().UnixNano(), atomic.AddInt64(&wm.workerCount, 1))

	// Create worker context
	workerCtx, workerCancel := context.WithCancel(wm.ctx)
	if opts.Timeout > 0 {
		workerCtx, workerCancel = context.WithTimeout(wm.ctx, opts.Timeout)
	}

	// Register worker
	wm.mu.Lock()
	wm.workers[workerID] = &WorkerInfo{
		ID:        workerID,
		Name:      name,
		StartTime: time.Now(),
		Context:   workerCtx,
		Cancel:    workerCancel,
	}
	wm.mu.Unlock()

	// Update metrics
	atomic.AddInt64(&wm.metrics.WorkersCreated, 1)
	atomic.AddInt64(&wm.metrics.WorkersActive, 1)

	// Start worker goroutine
	wm.wg.Add(1)
	go wm.runWorker(workerID, fn, opts, workerCtx, workerCancel)

	wm.logger.Debug("Worker started",
		zap.String("worker_id", workerID),
		zap.String("worker_name", name),
		zap.Duration("timeout", opts.Timeout),
		zap.Int("max_retries", opts.MaxRetries),
	)

	return workerID, nil
}

// runWorker executes the worker function with proper panic recovery
func (wm *WorkerManager) runWorker(workerID string, fn WorkerFunc, opts *WorkerOptions, ctx context.Context, cancel context.CancelFunc) {
	defer wm.wg.Done()
	defer cancel()
	defer func() {
		// Update metrics
		atomic.AddInt64(&wm.metrics.WorkersActive, -1)
		atomic.AddInt64(&wm.metrics.WorkersCompleted, 1)
		
		// Remove worker from registry
		wm.mu.Lock()
		delete(wm.workers, workerID)
		wm.mu.Unlock()
	}()

	// Set up panic recovery if not skipped
	if !opts.SkipPanicRecovery {
		defer func() {
			if r := recover(); r != nil {
				atomic.AddInt64(&wm.metrics.PanicsRecovered, 1)
				atomic.AddInt64(&wm.metrics.WorkersFailed, 1)
				
				// Capture stack trace
				stackTrace := debug.Stack()
				
				// Handle panic through configured handler
				if wm.panicHandler != nil {
					wm.panicHandler.HandlePanic(workerID, r, stackTrace)
				}
				
				wm.logger.Error("Worker panic recovered",
					zap.String("worker_id", workerID),
					zap.String("worker_name", opts.Name),
					zap.Any("panic_value", r),
					zap.String("stack_trace", string(stackTrace)),
				)
			}
		}()
	}

	// Execute worker function with retries
	var lastErr error
	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			wm.logger.Debug("Worker cancelled",
				zap.String("worker_id", workerID),
				zap.String("worker_name", opts.Name),
				zap.Error(ctx.Err()),
			)
			return
		default:
		}

		// Execute the function
		err := fn(ctx)
		if err == nil {
			// Success
			wm.logger.Debug("Worker completed successfully",
				zap.String("worker_id", workerID),
				zap.String("worker_name", opts.Name),
				zap.Int("attempts", attempt+1),
			)
			return
		}

		lastErr = err
		
		// If this is the last attempt, don't retry
		if attempt == opts.MaxRetries {
			break
		}

		// Log retry attempt
		wm.logger.Warn("Worker failed, retrying",
			zap.String("worker_id", workerID),
			zap.String("worker_name", opts.Name),
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", opts.MaxRetries),
			zap.Duration("retry_delay", opts.RetryDelay),
		)

		// Wait before retrying
		select {
		case <-time.After(opts.RetryDelay):
		case <-ctx.Done():
			wm.logger.Debug("Worker cancelled during retry wait",
				zap.String("worker_id", workerID),
				zap.String("worker_name", opts.Name),
				zap.Error(ctx.Err()),
			)
			return
		}
	}

	// All attempts failed
	atomic.AddInt64(&wm.metrics.WorkersFailed, 1)
	wm.logger.Error("Worker failed after all retry attempts",
		zap.String("worker_id", workerID),
		zap.String("worker_name", opts.Name),
		zap.Error(lastErr),
		zap.Int("max_retries", opts.MaxRetries),
	)
}

// CancelWorker cancels a specific worker
func (wm *WorkerManager) CancelWorker(workerID string) error {
	wm.mu.RLock()
	worker, exists := wm.workers[workerID]
	wm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	worker.Cancel()
	wm.logger.Debug("Worker cancelled",
		zap.String("worker_id", workerID),
		zap.String("worker_name", worker.Name),
	)

	return nil
}

// GetWorkerInfo returns information about a specific worker
func (wm *WorkerManager) GetWorkerInfo(workerID string) (*WorkerInfo, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	worker, exists := wm.workers[workerID]
	if !exists {
		return nil, fmt.Errorf("worker %s not found", workerID)
	}

	// Return a copy to avoid race conditions
	return &WorkerInfo{
		ID:        worker.ID,
		Name:      worker.Name,
		StartTime: worker.StartTime,
		Context:   worker.Context,
		Cancel:    worker.Cancel,
	}, nil
}

// ListWorkers returns information about all active workers
func (wm *WorkerManager) ListWorkers() []*WorkerInfo {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workers := make([]*WorkerInfo, 0, len(wm.workers))
	for _, worker := range wm.workers {
		workers = append(workers, &WorkerInfo{
			ID:        worker.ID,
			Name:      worker.Name,
			StartTime: worker.StartTime,
			Context:   worker.Context,
			Cancel:    worker.Cancel,
		})
	}

	return workers
}

// GetMetrics returns current worker metrics
func (wm *WorkerManager) GetMetrics() WorkerMetrics {
	return WorkerMetrics{
		WorkersCreated:   atomic.LoadInt64(&wm.metrics.WorkersCreated),
		WorkersCompleted: atomic.LoadInt64(&wm.metrics.WorkersCompleted),
		WorkersFailed:    atomic.LoadInt64(&wm.metrics.WorkersFailed),
		PanicsRecovered:  atomic.LoadInt64(&wm.metrics.PanicsRecovered),
		WorkersActive:    atomic.LoadInt64(&wm.metrics.WorkersActive),
	}
}

// Stop gracefully shuts down the worker manager
func (wm *WorkerManager) Stop() error {
	var stopErr error
	
	wm.stopOnce.Do(func() {
		wm.logger.Info("Stopping worker manager",
			zap.Int64("active_workers", atomic.LoadInt64(&wm.metrics.WorkersActive)),
		)

		// Cancel all workers
		wm.cancel()

		// Wait for all workers to complete with timeout
		done := make(chan struct{})
		go func() {
			wm.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			wm.logger.Info("All workers stopped gracefully")
		case <-time.After(wm.shutdownTimeout):
			stopErr = fmt.Errorf("timeout waiting for workers to stop after %v", wm.shutdownTimeout)
			wm.logger.Error("Timeout waiting for workers to stop",
				zap.Duration("timeout", wm.shutdownTimeout),
				zap.Int64("remaining_workers", atomic.LoadInt64(&wm.metrics.WorkersActive)),
			)
		}
	})

	return stopErr
}

// Context returns the worker manager's context
func (wm *WorkerManager) Context() context.Context {
	return wm.ctx
}

// IsRunning returns true if the worker manager is running
func (wm *WorkerManager) IsRunning() bool {
	select {
	case <-wm.ctx.Done():
		return false
	default:
		return true
	}
}

// StartBackgroundWorker is a convenience method for starting long-running background workers
func (wm *WorkerManager) StartBackgroundWorker(name string, fn WorkerFunc) (string, error) {
	opts := &WorkerOptions{
		Name:       name,
		MaxRetries: 3,
		RetryDelay: 5 * time.Second,
	}
	return wm.StartWorker(name, fn, opts)
}

// StartTimedWorker is a convenience method for starting workers with a timeout
func (wm *WorkerManager) StartTimedWorker(name string, timeout time.Duration, fn WorkerFunc) (string, error) {
	opts := &WorkerOptions{
		Name:       name,
		Timeout:    timeout,
		MaxRetries: 1,
		RetryDelay: time.Second,
	}
	return wm.StartWorker(name, fn, opts)
}

// StartOneOffWorker is a convenience method for starting workers that should run once
func (wm *WorkerManager) StartOneOffWorker(name string, fn WorkerFunc) (string, error) {
	opts := &WorkerOptions{
		Name:       name,
		MaxRetries: 0,
	}
	return wm.StartWorker(name, fn, opts)
}

// Health check functionality
func (wm *WorkerManager) HealthCheck() error {
	if !wm.IsRunning() {
		return fmt.Errorf("worker manager is not running")
	}

	metrics := wm.GetMetrics()
	
	// Check for excessive failures
	if metrics.WorkersCreated > 0 {
		failureRate := float64(metrics.WorkersFailed) / float64(metrics.WorkersCreated)
		if failureRate > 0.5 {
			return fmt.Errorf("high worker failure rate: %.2f%%", failureRate*100)
		}
	}

	// Check for excessive panics
	if metrics.PanicsRecovered > 10 {
		return fmt.Errorf("excessive panic recoveries: %d", metrics.PanicsRecovered)
	}

	return nil
}