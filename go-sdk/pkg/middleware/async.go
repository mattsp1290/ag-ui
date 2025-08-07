package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AsyncMiddlewareChain manages asynchronous middleware execution
type AsyncMiddlewareChain struct {
	*MiddlewareChain
	maxConcurrency int
	timeout        time.Duration
	semaphore      chan struct{}
	wg             sync.WaitGroup
	mu             sync.RWMutex
	shutdown       bool
}

// NewAsyncMiddlewareChain creates a new asynchronous middleware chain
func NewAsyncMiddlewareChain(handler Handler, maxConcurrency int, timeout time.Duration) *AsyncMiddlewareChain {
	if maxConcurrency <= 0 {
		maxConcurrency = 10 // Default max concurrency
	}
	if timeout <= 0 {
		timeout = 30 * time.Second // Default timeout
	}

	return &AsyncMiddlewareChain{
		MiddlewareChain: NewMiddlewareChain(handler),
		maxConcurrency:  maxConcurrency,
		timeout:         timeout,
		semaphore:       make(chan struct{}, maxConcurrency),
	}
}

// ProcessAsync executes the middleware chain asynchronously
func (ac *AsyncMiddlewareChain) ProcessAsync(ctx context.Context, req *Request) <-chan *MiddlewareResult {
	resultChan := make(chan *MiddlewareResult, 1)

	// Check if shutdown has started
	ac.mu.RLock()
	if ac.shutdown {
		ac.mu.RUnlock()
		// Return error result immediately
		go func() {
			defer close(resultChan)
			resultChan <- &MiddlewareResult{
				Error: fmt.Errorf("middleware chain is shutting down"),
			}
		}()
		return resultChan
	}

	// Add to wait group while holding read lock
	ac.wg.Add(1)
	ac.mu.RUnlock()

	go func() {
		defer ac.wg.Done()
		defer close(resultChan)

		// Apply timeout to the context
		ctx, cancel := context.WithTimeout(ctx, ac.timeout)
		defer cancel()

		// Acquire semaphore to limit concurrency
		select {
		case ac.semaphore <- struct{}{}:
			defer func() { <-ac.semaphore }()
		case <-ctx.Done():
			resultChan <- &MiddlewareResult{
				Response: nil,
				Error:    fmt.Errorf("async middleware timeout: %w", ctx.Err()),
			}
			return
		}

		// Execute the middleware chain
		response, err := ac.executeAsyncChain(ctx, req, 0)
		resultChan <- &MiddlewareResult{
			Response: response,
			Error:    err,
		}
	}()

	return resultChan
}

// executeAsyncChain recursively executes async middleware
func (ac *AsyncMiddlewareChain) executeAsyncChain(ctx context.Context, req *Request, index int) (*Response, error) {
	// If we've processed all middleware, call the final handler
	if index >= len(ac.middlewares) {
		if ac.handler == nil {
			return &Response{
				ID:         req.ID,
				StatusCode: 404,
				Error:      fmt.Errorf("no handler configured"),
				Timestamp:  time.Now(),
			}, nil
		}

		// Handle final handler execution with context timeout
		startTime := time.Now()
		type result struct {
			resp *Response
			err  error
		}

		resultChan := make(chan result, 1)
		go func() {
			resp, err := ac.handler(ctx, req)
			if resp != nil && resp.Duration == 0 {
				resp.Duration = time.Since(startTime)
			}
			resultChan <- result{resp: resp, err: err}
		}()

		select {
		case res := <-resultChan:
			return res.resp, res.err
		case <-ctx.Done():
			return nil, fmt.Errorf("handler timeout: %w", ctx.Err())
		}
	}

	middleware := ac.middlewares[index]

	// Skip disabled middleware
	if !middleware.Enabled() {
		return ac.executeAsyncChain(ctx, req, index+1)
	}

	// Check if middleware implements AsyncMiddleware
	if asyncMiddleware, ok := middleware.(AsyncMiddleware); ok {
		// Create the next handler for async middleware
		next := func(ctx context.Context, req *Request) (*Response, error) {
			return ac.executeAsyncChain(ctx, req, index+1)
		}

		// Execute async middleware
		resultChan := asyncMiddleware.ProcessAsync(ctx, req, next)
		select {
		case result := <-resultChan:
			if result != nil {
				return result.Response, result.Error
			}
			return nil, fmt.Errorf("async middleware returned nil result")
		case <-ctx.Done():
			return nil, fmt.Errorf("async middleware timeout: %w", ctx.Err())
		}
	}

	// Fallback to synchronous execution for regular middleware
	next := func(ctx context.Context, req *Request) (*Response, error) {
		return ac.executeAsyncChain(ctx, req, index+1)
	}

	startTime := time.Now()
	resp, err := middleware.Process(ctx, req, next)
	if resp != nil && resp.Duration == 0 {
		resp.Duration = time.Since(startTime)
	}

	return resp, err
}

// Wait waits for all async operations to complete
func (ac *AsyncMiddlewareChain) Wait() {
	ac.wg.Wait()
}

// Shutdown gracefully shuts down the async chain
func (ac *AsyncMiddlewareChain) Shutdown(ctx context.Context) error {
	// Set shutdown flag to prevent new operations
	ac.mu.Lock()
	ac.shutdown = true
	ac.mu.Unlock()

	// Wait for all operations to complete or context to timeout
	done := make(chan struct{})
	go func() {
		ac.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("async chain shutdown timeout: %w", ctx.Err())
	}
}

// GetConcurrencyStats returns concurrency statistics
func (ac *AsyncMiddlewareChain) GetConcurrencyStats() AsyncConcurrencyStats {
	return AsyncConcurrencyStats{
		MaxConcurrency:   ac.maxConcurrency,
		CurrentActive:    ac.maxConcurrency - len(ac.semaphore),
		QueuedOperations: len(ac.semaphore),
		Timeout:          ac.timeout,
	}
}

// AsyncConcurrencyStats contains statistics about async operations
type AsyncConcurrencyStats struct {
	MaxConcurrency   int
	CurrentActive    int
	QueuedOperations int
	Timeout          time.Duration
}

// AsyncMiddlewareWrapper wraps regular middleware to make it async-compatible
type AsyncMiddlewareWrapper struct {
	Middleware
}

// NewAsyncMiddlewareWrapper creates a wrapper that makes regular middleware async-compatible
func NewAsyncMiddlewareWrapper(middleware Middleware) AsyncMiddleware {
	return &AsyncMiddlewareWrapper{
		Middleware: middleware,
	}
}

// ProcessAsync implements AsyncMiddleware by wrapping synchronous Process call
func (amw *AsyncMiddlewareWrapper) ProcessAsync(ctx context.Context, req *Request, next NextHandler) <-chan *MiddlewareResult {
	resultChan := make(chan *MiddlewareResult, 1)

	go func() {
		defer close(resultChan)

		response, err := amw.Middleware.Process(ctx, req, next)
		resultChan <- &MiddlewareResult{
			Response: response,
			Error:    err,
		}
	}()

	return resultChan
}

// AsyncMiddlewarePool manages a pool of async middleware instances for high-throughput scenarios
type AsyncMiddlewarePool struct {
	factory func() AsyncMiddleware
	pool    chan AsyncMiddleware
	maxSize int
	created int
	mu      sync.Mutex
}

// NewAsyncMiddlewarePool creates a new pool of async middleware instances
func NewAsyncMiddlewarePool(maxSize int, factory func() AsyncMiddleware) *AsyncMiddlewarePool {
	if maxSize <= 0 {
		maxSize = 10
	}

	return &AsyncMiddlewarePool{
		factory: factory,
		pool:    make(chan AsyncMiddleware, maxSize),
		maxSize: maxSize,
	}
}

// Get retrieves an async middleware instance from the pool
func (amp *AsyncMiddlewarePool) Get() AsyncMiddleware {
	select {
	case middleware := <-amp.pool:
		return middleware
	default:
		amp.mu.Lock()
		if amp.created < amp.maxSize {
			amp.created++
			amp.mu.Unlock()
			return amp.factory()
		}
		amp.mu.Unlock()

		// Block waiting for an available instance with timeout
		select {
		case middleware := <-amp.pool:
			return middleware
		case <-time.After(1 * time.Second):
			// Timeout waiting for instance, create a temporary one
			return amp.factory()
		}
	}
}

// Put returns an async middleware instance to the pool
func (amp *AsyncMiddlewarePool) Put(middleware AsyncMiddleware) {
	select {
	case amp.pool <- middleware:
	default:
		// Pool is full, discard the instance
	}
}

// Close closes the pool and cleans up resources
func (amp *AsyncMiddlewarePool) Close() {
	close(amp.pool)
	for middleware := range amp.pool {
		if lifecycle, ok := middleware.(MiddlewareLifecycle); ok {
			_ = lifecycle.Shutdown(context.Background())
		}
	}
}

// AsyncBatchProcessor processes multiple requests concurrently
type AsyncBatchProcessor struct {
	chain          *AsyncMiddlewareChain
	batchSize      int
	processingTime time.Duration
}

// NewAsyncBatchProcessor creates a new batch processor for async middleware
func NewAsyncBatchProcessor(chain *AsyncMiddlewareChain, batchSize int) *AsyncBatchProcessor {
	if batchSize <= 0 {
		batchSize = 10
	}

	return &AsyncBatchProcessor{
		chain:     chain,
		batchSize: batchSize,
	}
}

// ProcessBatch processes multiple requests concurrently
func (abp *AsyncBatchProcessor) ProcessBatch(ctx context.Context, requests []*Request) ([]*MiddlewareResult, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	results := make([]*MiddlewareResult, len(requests))
	var wg sync.WaitGroup

	// Process requests in batches
	for i := 0; i < len(requests); i += abp.batchSize {
		end := i + abp.batchSize
		if end > len(requests) {
			end = len(requests)
		}

		batchRequests := requests[i:end]
		wg.Add(len(batchRequests))

		for j, req := range batchRequests {
			go func(index int, request *Request) {
				defer wg.Done()

				resultChan := abp.chain.ProcessAsync(ctx, request)
				select {
				case result := <-resultChan:
					results[i+index] = result
				case <-ctx.Done():
					results[i+index] = &MiddlewareResult{
						Response: nil,
						Error:    fmt.Errorf("batch processing timeout: %w", ctx.Err()),
					}
				}
			}(j, req)
		}
	}

	// Wait for all requests to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return results, nil
	case <-ctx.Done():
		return results, fmt.Errorf("batch processing timeout: %w", ctx.Err())
	}
}
