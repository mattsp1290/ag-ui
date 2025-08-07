package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EnhancedMiddlewareManager extends the basic MiddlewareManager with advanced capabilities
type EnhancedMiddlewareManager struct {
	*MiddlewareManager
	asyncChains        map[string]*AsyncMiddlewareChain
	dependencyChains   map[string]*DependencyAwareMiddlewareChain
	dependencyManager  *DependencyManager
	asyncPools         map[string]*AsyncMiddlewarePool
	batchProcessors    map[string]*AsyncBatchProcessor
	resilienceStats    map[string]*ResilienceStats
	performanceMetrics *PerformanceMetrics
	mu                 sync.RWMutex
}

// PerformanceMetrics tracks performance statistics
type PerformanceMetrics struct {
	TotalRequests       uint64
	SuccessfulRequests  uint64
	FailedRequests      uint64
	AverageLatency      time.Duration
	P95Latency          time.Duration
	P99Latency          time.Duration
	ThroughputPerSecond float64
	mu                  sync.RWMutex
}

// NewEnhancedMiddlewareManager creates a new enhanced middleware manager
func NewEnhancedMiddlewareManager() *EnhancedMiddlewareManager {
	return &EnhancedMiddlewareManager{
		MiddlewareManager:  NewMiddlewareManager(),
		asyncChains:        make(map[string]*AsyncMiddlewareChain),
		dependencyChains:   make(map[string]*DependencyAwareMiddlewareChain),
		dependencyManager:  NewDependencyManager(),
		asyncPools:         make(map[string]*AsyncMiddlewarePool),
		batchProcessors:    make(map[string]*AsyncBatchProcessor),
		resilienceStats:    make(map[string]*ResilienceStats),
		performanceMetrics: &PerformanceMetrics{},
	}
}

// CreateAsyncChain creates a new async middleware chain
func (emm *EnhancedMiddlewareManager) CreateAsyncChain(name string, handler Handler, maxConcurrency int, timeout time.Duration) *AsyncMiddlewareChain {
	emm.mu.Lock()
	defer emm.mu.Unlock()

	chain := NewAsyncMiddlewareChain(handler, maxConcurrency, timeout)
	emm.asyncChains[name] = chain
	return chain
}

// CreateDependencyAwareChain creates a new dependency-aware middleware chain
func (emm *EnhancedMiddlewareManager) CreateDependencyAwareChain(name string, handler Handler) *DependencyAwareMiddlewareChain {
	emm.mu.Lock()
	defer emm.mu.Unlock()

	chain := emm.dependencyManager.CreateChain(name, handler)
	emm.dependencyChains[name] = chain
	return chain
}

// GetAsyncChain returns an async middleware chain by name
func (emm *EnhancedMiddlewareManager) GetAsyncChain(name string) *AsyncMiddlewareChain {
	emm.mu.RLock()
	defer emm.mu.RUnlock()

	return emm.asyncChains[name]
}

// GetDependencyAwareChain returns a dependency-aware middleware chain by name
func (emm *EnhancedMiddlewareManager) GetDependencyAwareChain(name string) *DependencyAwareMiddlewareChain {
	emm.mu.RLock()
	defer emm.mu.RUnlock()

	return emm.dependencyChains[name]
}

// ProcessAsync processes a request asynchronously
func (emm *EnhancedMiddlewareManager) ProcessAsync(ctx context.Context, chainName string, req *Request) <-chan *MiddlewareResult {
	resultChan := make(chan *MiddlewareResult, 1)

	chain := emm.GetAsyncChain(chainName)
	if chain == nil {
		resultChan <- &MiddlewareResult{
			Response: nil,
			Error:    fmt.Errorf("async chain %s not found", chainName),
		}
		close(resultChan)
		return resultChan
	}

	return chain.ProcessAsync(ctx, req)
}

// ProcessWithDependencies processes a request with dependency resolution
func (emm *EnhancedMiddlewareManager) ProcessWithDependencies(ctx context.Context, chainName string, req *Request) (*Response, error) {
	chain := emm.GetDependencyAwareChain(chainName)
	if chain == nil {
		return nil, fmt.Errorf("dependency-aware chain %s not found", chainName)
	}

	startTime := time.Now()
	response, err := chain.Process(ctx, req)

	// Update performance metrics
	emm.updatePerformanceMetrics(time.Since(startTime), err == nil)

	return response, err
}

// ProcessBatch processes multiple requests concurrently
func (emm *EnhancedMiddlewareManager) ProcessBatch(ctx context.Context, chainName string, requests []*Request) ([]*MiddlewareResult, error) {
	processor := emm.getBatchProcessor(chainName)
	if processor == nil {
		return nil, fmt.Errorf("batch processor for chain %s not found", chainName)
	}

	return processor.ProcessBatch(ctx, requests)
}

// getBatchProcessor gets or creates a batch processor for the chain
func (emm *EnhancedMiddlewareManager) getBatchProcessor(chainName string) *AsyncBatchProcessor {
	emm.mu.RLock()
	if processor, exists := emm.batchProcessors[chainName]; exists {
		emm.mu.RUnlock()
		return processor
	}
	emm.mu.RUnlock()

	emm.mu.Lock()
	defer emm.mu.Unlock()

	// Double-check pattern
	if processor, exists := emm.batchProcessors[chainName]; exists {
		return processor
	}

	// Create processor if async chain exists
	if asyncChain, exists := emm.asyncChains[chainName]; exists {
		processor := NewAsyncBatchProcessor(asyncChain, 10) // Default batch size
		emm.batchProcessors[chainName] = processor
		return processor
	}

	return nil
}

// UpdatePerformanceMetrics updates performance metrics
func (emm *EnhancedMiddlewareManager) updatePerformanceMetrics(duration time.Duration, success bool) {
	emm.performanceMetrics.mu.Lock()
	defer emm.performanceMetrics.mu.Unlock()

	emm.performanceMetrics.TotalRequests++
	if success {
		emm.performanceMetrics.SuccessfulRequests++
	} else {
		emm.performanceMetrics.FailedRequests++
	}

	// Simple moving average for latency (could be enhanced with more sophisticated algorithms)
	if emm.performanceMetrics.TotalRequests == 1 {
		emm.performanceMetrics.AverageLatency = duration
	} else {
		// Exponential moving average
		alpha := 0.1
		emm.performanceMetrics.AverageLatency = time.Duration(
			alpha*float64(duration) + (1-alpha)*float64(emm.performanceMetrics.AverageLatency),
		)
	}
}

// AddResilienceMiddleware adds a resilience middleware to a chain
func (emm *EnhancedMiddlewareManager) AddResilienceMiddleware(chainName string, middleware *ResilienceMiddleware) error {
	emm.mu.Lock()
	defer emm.mu.Unlock()

	// Add to regular chain if it exists
	if chain := emm.GetChain(chainName); chain != nil {
		chain.Add(middleware)
	}

	// Add to async chain if it exists
	if asyncChain, exists := emm.asyncChains[chainName]; exists {
		asyncChain.Add(middleware)
	}

	// Add to dependency-aware chain if it exists
	if depChain, exists := emm.dependencyChains[chainName]; exists {
		// For dependency chains, we need to specify dependencies
		return depChain.AddMiddlewareWithDependencies(
			middleware,
			[]string{}, // No dependencies by default
			false,      // Not optional
			&AlwaysDependencyCondition{},
		)
	}

	return nil
}

// GetResilienceStats returns resilience statistics for all chains
func (emm *EnhancedMiddlewareManager) GetResilienceStats() map[string]ResilienceStats {
	emm.mu.RLock()
	defer emm.mu.RUnlock()

	stats := make(map[string]ResilienceStats)

	// Collect stats from regular chains
	for chainName := range emm.chains {
		if chain := emm.GetChain(chainName); chain != nil {
			for _, middlewareName := range chain.ListMiddleware() {
				if middleware := chain.GetMiddleware(middlewareName); middleware != nil {
					if resilienceMiddleware, ok := middleware.(*ResilienceMiddleware); ok {
						stats[fmt.Sprintf("%s:%s", chainName, middlewareName)] = resilienceMiddleware.GetCircuitBreakerStats()
					}
				}
			}
		}
	}

	// Collect stats from async chains
	for chainName, asyncChain := range emm.asyncChains {
		if asyncChain != nil {
			for _, middlewareName := range asyncChain.ListMiddleware() {
				if middleware := asyncChain.GetMiddleware(middlewareName); middleware != nil {
					if resilienceMiddleware, ok := middleware.(*ResilienceMiddleware); ok {
						stats[fmt.Sprintf("async:%s:%s", chainName, middlewareName)] = resilienceMiddleware.GetCircuitBreakerStats()
					}
				}
			}
		}
	}

	// Collect stats from dependency chains
	for chainName, depChain := range emm.dependencyChains {
		if depChain != nil && depChain.dependencyGraph != nil {
			// Walk through all nodes in the dependency graph
			for middlewareName, node := range depChain.dependencyGraph.nodes {
				if resilienceMiddleware, ok := node.Middleware.(*ResilienceMiddleware); ok {
					stats[fmt.Sprintf("dep:%s:%s", chainName, middlewareName)] = resilienceMiddleware.GetCircuitBreakerStats()
				}
			}
		}
	}

	return stats
}

// GetPerformanceMetrics returns current performance metrics
func (emm *EnhancedMiddlewareManager) GetPerformanceMetrics() PerformanceMetrics {
	emm.performanceMetrics.mu.RLock()
	defer emm.performanceMetrics.mu.RUnlock()

	// Create a copy without the mutex to avoid copying lock value
	return PerformanceMetrics{
		TotalRequests:       emm.performanceMetrics.TotalRequests,
		SuccessfulRequests:  emm.performanceMetrics.SuccessfulRequests,
		FailedRequests:      emm.performanceMetrics.FailedRequests,
		AverageLatency:      emm.performanceMetrics.AverageLatency,
		P95Latency:          emm.performanceMetrics.P95Latency,
		P99Latency:          emm.performanceMetrics.P99Latency,
		ThroughputPerSecond: emm.performanceMetrics.ThroughputPerSecond,
		// Note: deliberately omitting mu field to avoid copying the lock
	}
}

// ValidateAllDependencies validates dependencies across all chains
func (emm *EnhancedMiddlewareManager) ValidateAllDependencies() map[string][]error {
	return emm.dependencyManager.ValidateAllChains()
}

// GetDependencyReport returns a comprehensive dependency report
func (emm *EnhancedMiddlewareManager) GetDependencyReport() DependencyReport {
	return emm.dependencyManager.GetDependencyReport()
}

// GetAsyncStats returns statistics for all async chains
func (emm *EnhancedMiddlewareManager) GetAsyncStats() map[string]AsyncConcurrencyStats {
	emm.mu.RLock()
	defer emm.mu.RUnlock()

	stats := make(map[string]AsyncConcurrencyStats)
	for name, chain := range emm.asyncChains {
		stats[name] = chain.GetConcurrencyStats()
	}

	return stats
}

// Shutdown gracefully shuts down all advanced middleware components
func (emm *EnhancedMiddlewareManager) Shutdown(ctx context.Context) error {
	emm.mu.Lock()
	defer emm.mu.Unlock()

	// Shutdown async chains
	for name, chain := range emm.asyncChains {
		if err := chain.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown async chain %s: %w", name, err)
		}
	}

	// Close async pools
	for _, pool := range emm.asyncPools {
		pool.Close()
	}

	// Shutdown the base manager
	return emm.MiddlewareManager.Stop()
}

// Health check for the enhanced middleware manager
func (emm *EnhancedMiddlewareManager) HealthCheck() MiddlewareHealthStatus {
	emm.mu.RLock()
	defer emm.mu.RUnlock()

	status := MiddlewareHealthStatus{
		Healthy:          true,
		RegularChains:    len(emm.chains),
		AsyncChains:      len(emm.asyncChains),
		DependencyChains: len(emm.dependencyChains),
		ActiveMiddleware: 0,
		DependencyErrors: emm.ValidateAllDependencies(),
		Timestamp:        time.Now(),
	}

	// Count active middleware
	for _, chain := range emm.chains {
		status.ActiveMiddleware += len(chain.ListMiddleware())
	}

	// Check if there are any dependency errors
	if len(status.DependencyErrors) > 0 {
		status.Healthy = false
	}

	return status
}

// MiddlewareHealthStatus represents the health status of the middleware system
type MiddlewareHealthStatus struct {
	Healthy          bool
	RegularChains    int
	AsyncChains      int
	DependencyChains int
	ActiveMiddleware int
	DependencyErrors map[string][]error
	Timestamp        time.Time
}

// EnhancedMiddlewareConfiguration extends the basic configuration with advanced features
type EnhancedMiddlewareConfiguration struct {
	*MiddlewareConfiguration
	AsyncChains       []AsyncChainConfiguration      `json:"async_chains" yaml:"async_chains"`
	DependencyChains  []DependencyChainConfiguration `json:"dependency_chains" yaml:"dependency_chains"`
	ResilienceConfig  ResilienceConfiguration        `json:"resilience" yaml:"resilience"`
	PerformanceConfig PerformanceConfiguration       `json:"performance" yaml:"performance"`
}

// AsyncChainConfiguration represents configuration for async chains
type AsyncChainConfiguration struct {
	Name           string               `json:"name" yaml:"name"`
	MaxConcurrency int                  `json:"max_concurrency" yaml:"max_concurrency"`
	Timeout        string               `json:"timeout" yaml:"timeout"`
	Handler        HandlerConfiguration `json:"handler" yaml:"handler"`
	Middleware     []MiddlewareConfig   `json:"middleware" yaml:"middleware"`
}

// DependencyChainConfiguration represents configuration for dependency-aware chains
type DependencyChainConfiguration struct {
	Name       string                       `json:"name" yaml:"name"`
	Handler    HandlerConfiguration         `json:"handler" yaml:"handler"`
	Middleware []DependencyMiddlewareConfig `json:"middleware" yaml:"middleware"`
}

// DependencyMiddlewareConfig extends MiddlewareConfig with dependency information
type DependencyMiddlewareConfig struct {
	MiddlewareConfig
	Dependencies []string `json:"dependencies" yaml:"dependencies"`
	Optional     bool     `json:"optional" yaml:"optional"`
	Condition    string   `json:"condition" yaml:"condition"`
}

// ResilienceConfiguration contains global resilience settings
type ResilienceConfiguration struct {
	DefaultCircuitBreaker CircuitBreakerConfiguration `json:"default_circuit_breaker" yaml:"default_circuit_breaker"`
	DefaultRetry          RetryConfiguration          `json:"default_retry" yaml:"default_retry"`
	GlobalRateLimit       RateLimitConfiguration      `json:"global_rate_limit" yaml:"global_rate_limit"`
}

// CircuitBreakerConfiguration represents circuit breaker configuration
type CircuitBreakerConfiguration struct {
	MaxFailures      int    `json:"max_failures" yaml:"max_failures"`
	ResetTimeout     string `json:"reset_timeout" yaml:"reset_timeout"`
	HalfOpenMaxCalls int    `json:"half_open_max_calls" yaml:"half_open_max_calls"`
	SuccessThreshold int    `json:"success_threshold" yaml:"success_threshold"`
	Timeout          string `json:"timeout" yaml:"timeout"`
}

// RetryConfiguration represents retry configuration
type RetryConfiguration struct {
	MaxAttempts     int      `json:"max_attempts" yaml:"max_attempts"`
	InitialDelay    string   `json:"initial_delay" yaml:"initial_delay"`
	MaxDelay        string   `json:"max_delay" yaml:"max_delay"`
	BackoffFactor   float64  `json:"backoff_factor" yaml:"backoff_factor"`
	RetryableErrors []string `json:"retryable_errors" yaml:"retryable_errors"`
}

// RateLimitConfiguration represents rate limit configuration
type RateLimitConfiguration struct {
	TokensPerSecond int `json:"tokens_per_second" yaml:"tokens_per_second"`
	BucketSize      int `json:"bucket_size" yaml:"bucket_size"`
}

// PerformanceConfiguration contains performance monitoring settings
type PerformanceConfiguration struct {
	EnableMetrics   bool   `json:"enable_metrics" yaml:"enable_metrics"`
	MetricsInterval string `json:"metrics_interval" yaml:"metrics_interval"`
	LatencyBuckets  []int  `json:"latency_buckets" yaml:"latency_buckets"`
}

// ApplyEnhancedConfiguration applies an enhanced middleware configuration
func (emm *EnhancedMiddlewareManager) ApplyEnhancedConfiguration(config *EnhancedMiddlewareConfiguration) error {
	// Apply base configuration first
	if err := emm.MiddlewareManager.ApplyConfiguration(config.MiddlewareConfiguration); err != nil {
		return fmt.Errorf("failed to apply base configuration: %w", err)
	}

	emm.mu.Lock()
	defer emm.mu.Unlock()

	// Apply async chain configurations
	for _, asyncConfig := range config.AsyncChains {
		timeout, err := time.ParseDuration(asyncConfig.Timeout)
		if err != nil {
			timeout = 30 * time.Second
		}

		handler, err := emm.createHandlerFromConfig(asyncConfig.Handler)
		if err != nil {
			return fmt.Errorf("failed to create handler for async chain %s: %w", asyncConfig.Name, err)
		}

		chain := NewAsyncMiddlewareChain(handler, asyncConfig.MaxConcurrency, timeout)

		// Add middleware to chain
		for _, middlewareConfig := range asyncConfig.Middleware {
			middleware, err := emm.registry.Create(&middlewareConfig)
			if err != nil {
				return fmt.Errorf("failed to create middleware %s for async chain: %w", middlewareConfig.Name, err)
			}
			if middleware != nil && middleware.Enabled() {
				chain.Add(middleware)
			}
		}

		emm.asyncChains[asyncConfig.Name] = chain
	}

	// Apply dependency chain configurations
	for _, depConfig := range config.DependencyChains {
		handler, err := emm.createHandlerFromConfig(depConfig.Handler)
		if err != nil {
			return fmt.Errorf("failed to create handler for dependency chain %s: %w", depConfig.Name, err)
		}

		chain := emm.dependencyManager.CreateChain(depConfig.Name, handler)

		// Add middleware with dependencies
		for _, middlewareConfig := range depConfig.Middleware {
			middleware, err := emm.registry.Create(&middlewareConfig.MiddlewareConfig)
			if err != nil {
				return fmt.Errorf("failed to create middleware %s for dependency chain: %w", middlewareConfig.Name, err)
			}

			if middleware != nil && middleware.Enabled() {
				var condition DependencyCondition = &AlwaysDependencyCondition{}

				// Parse condition if specified
				switch middlewareConfig.Condition {
				case "path":
					// This would need more configuration, simplified for now
					condition = &PathBasedDependencyCondition{PathPatterns: []string{"*"}}
				}

				if err := chain.AddMiddlewareWithDependencies(
					middleware,
					middlewareConfig.Dependencies,
					middlewareConfig.Optional,
					condition,
				); err != nil {
					return fmt.Errorf("failed to add middleware %s to dependency chain: %w", middlewareConfig.Name, err)
				}
			}
		}

		emm.dependencyChains[depConfig.Name] = chain
	}

	return nil
}

// Helper method to create handlers (reused from base manager)
func (emm *EnhancedMiddlewareManager) createHandlerFromConfig(config HandlerConfiguration) (Handler, error) {
	// This would be the same logic as in the base MiddlewareManager
	// For now, return a simple implementation
	return func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{
			ID:         req.ID,
			StatusCode: 200,
			Body:       map[string]interface{}{"message": "OK"},
			Timestamp:  time.Now(),
		}, nil
	}, nil
}
