package internal

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
)

// CallbackJob represents a job to be executed by the callback pool
type CallbackJob struct {
	fn func()
}

// CallbackPool manages a pool of worker goroutines for executing small callback operations
// to reduce the overhead of creating individual goroutines for each callback
type CallbackPool struct {
	workers    int
	jobQueue   chan CallbackJob
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	started    int32  // use atomic operations for thread-safe access
	startOnce  sync.Once
	stopOnce   sync.Once
}

// NewCallbackPool creates a new callback pool with the specified number of workers
// If workers is 0, it defaults to runtime.NumCPU()
func NewCallbackPool(workers int) *CallbackPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	return &CallbackPool{
		workers:  workers,
		jobQueue: make(chan CallbackJob, workers*2), // Buffer size = workers * 2
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start initializes and starts the worker goroutines
func (cp *CallbackPool) Start() {
	cp.startOnce.Do(func() {
		atomic.StoreInt32(&cp.started, 1)
		for i := 0; i < cp.workers; i++ {
			cp.wg.Add(1)
			go cp.worker()
		}
	})
}

// worker is the main worker goroutine that processes jobs from the queue
func (cp *CallbackPool) worker() {
	defer cp.wg.Done()
	
	for {
		select {
		case <-cp.ctx.Done():
			return
		case job, ok := <-cp.jobQueue:
			if !ok {
				return
			}
			
			// Execute the job with panic recovery
			func() {
				defer func() {
					if r := recover(); r != nil {
						// Log panic but don't crash the worker
						// In a real implementation, you might want to use a proper logger
						_ = r // Suppress unused variable warning
					}
				}()
				job.fn()
			}()
		}
	}
}

// Submit attempts to queue a job for execution by the worker pool
// If the pool is not started, it starts automatically
// If the queue is full, it falls back to executing the function directly
// to avoid blocking the caller
func (cp *CallbackPool) Submit(fn func()) {
	if fn == nil {
		return
	}

	// Auto-start the pool if not already started
	if atomic.LoadInt32(&cp.started) == 0 {
		cp.Start()
	}

	job := CallbackJob{fn: fn}
	
	select {
	case cp.jobQueue <- job:
		// Successfully queued
	default:
		// Queue is full, execute directly to avoid blocking
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Recover from panics in direct execution
					_ = r
				}
			}()
			fn()
		}()
	}
}

// Stop gracefully shuts down the callback pool
// It cancels the context and waits for all workers to finish
func (cp *CallbackPool) Stop() {
	cp.stopOnce.Do(func() {
		cp.cancel()
		close(cp.jobQueue)
		cp.wg.Wait()
	})
}

// WorkerCount returns the number of workers in the pool
func (cp *CallbackPool) WorkerCount() int {
	return cp.workers
}

// QueueCapacity returns the capacity of the job queue
func (cp *CallbackPool) QueueCapacity() int {
	return cap(cp.jobQueue)
}

// QueueLength returns the current number of jobs in the queue
func (cp *CallbackPool) QueueLength() int {
	return len(cp.jobQueue)
}

// IsStarted returns true if the pool has been started
func (cp *CallbackPool) IsStarted() bool {
	return atomic.LoadInt32(&cp.started) == 1
}