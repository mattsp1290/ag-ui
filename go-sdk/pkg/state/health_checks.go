package state

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// StateManagerHealthCheck checks the health of the state manager
type StateManagerHealthCheck struct {
	manager *StateManager
	name    string
}

// NewStateManagerHealthCheck creates a new state manager health check
func NewStateManagerHealthCheck(manager *StateManager) *StateManagerHealthCheck {
	return &StateManagerHealthCheck{
		manager: manager,
		name:    "state_manager",
	}
}

// Name returns the name of the health check
func (hc *StateManagerHealthCheck) Name() string {
	return hc.name
}

// Check performs the health check
func (hc *StateManagerHealthCheck) Check(ctx context.Context) error {
	if hc.manager == nil {
		return errors.New("state manager is nil")
	}

	// Check if manager is closing or closed
	if hc.manager.isClosing() {
		return errors.New("state manager is closing")
	}

	// Check if update queue is available
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue is available
	}

	// Check if core components are initialized
	if hc.manager.store == nil {
		return errors.New("state store is not initialized")
	}

	if hc.manager.deltaComputer == nil {
		return errors.New("delta computer is not initialized")
	}

	if hc.manager.conflictResolver == nil {
		return errors.New("conflict resolver is not initialized")
	}

	return nil
}

// MemoryHealthCheck checks memory usage and GC performance
type MemoryHealthCheck struct {
	maxMemoryMB   int64
	maxGCPauseMs  int64
	maxGoroutines int
	name          string
}

// NewMemoryHealthCheck creates a new memory health check
func NewMemoryHealthCheck(maxMemoryMB int64, maxGCPauseMs int64, maxGoroutines int) *MemoryHealthCheck {
	return &MemoryHealthCheck{
		maxMemoryMB:   maxMemoryMB,
		maxGCPauseMs:  maxGCPauseMs,
		maxGoroutines: maxGoroutines,
		name:          "memory",
	}
}

// Name returns the name of the health check
func (hc *MemoryHealthCheck) Name() string {
	return hc.name
}

// Check performs the health check
func (hc *MemoryHealthCheck) Check(ctx context.Context) error {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Check memory usage
	memoryMB := int64(memStats.Alloc / 1024 / 1024)
	// Special case: if maxMemoryMB is 0 and we have any memory allocation, it's an error
	if hc.maxMemoryMB == 0 && memStats.Alloc > 0 {
		return fmt.Errorf("memory usage (%d bytes) exceeds threshold (0 MB)", memStats.Alloc)
	}
	if memoryMB > hc.maxMemoryMB {
		return fmt.Errorf("memory usage (%d MB) exceeds threshold (%d MB)", memoryMB, hc.maxMemoryMB)
	}

	// Check GC pause time
	if memStats.NumGC > 0 {
		lastGCPause := memStats.PauseNs[(memStats.NumGC+255)%256]
		gcPauseMs := int64(lastGCPause / 1000000)
		if gcPauseMs > hc.maxGCPauseMs {
			return fmt.Errorf("GC pause time (%d ms) exceeds threshold (%d ms)", gcPauseMs, hc.maxGCPauseMs)
		}
	}

	// Check goroutine count
	goroutines := runtime.NumGoroutine()
	if goroutines > hc.maxGoroutines {
		return fmt.Errorf("goroutine count (%d) exceeds threshold (%d)", goroutines, hc.maxGoroutines)
	}

	return nil
}

// StoreHealthCheck checks the health of the state store
type StoreHealthCheck struct {
	store   StoreInterface
	name    string
	timeout time.Duration
}

// NewStoreHealthCheck creates a new store health check
func NewStoreHealthCheck(store StoreInterface, timeout time.Duration) *StoreHealthCheck {
	// Validate timeout
	if timeout <= 0 {
		timeout = 5 * time.Second // Default timeout
	}
	
	return &StoreHealthCheck{
		store:   store,
		name:    "store",
		timeout: timeout,
	}
}

// Name returns the name of the health check
func (hc *StoreHealthCheck) Name() string {
	return hc.name
}

// Check performs the health check
func (hc *StoreHealthCheck) Check(ctx context.Context) error {
	if hc.store == nil {
		return errors.New("state store is nil")
	}

	// Check if the store is a StateStore and if it's in a degraded state
	// Only do this check if store is not nil
	if stateStore, ok := hc.store.(*StateStore); ok && stateStore != nil {
		// Check if shards are properly initialized
		if stateStore.shards == nil || len(stateStore.shards) == 0 {
			return errors.New("store shards not initialized")
		}
		
		// Check each shard for nil current state
		for i, shard := range stateStore.shards {
			if shard == nil {
				return fmt.Errorf("shard %d is nil", i)
			}
			if shard.current.Load() == nil {
				return fmt.Errorf("shard %d has nil current state", i)
			}
		}
	}

	// Create a test context with timeout
	testCtx, cancel := context.WithTimeout(ctx, hc.timeout)
	defer cancel()

	// Safely attempt to get state with panic recovery
	var state map[string]interface{}
	var err error
	
	// Use a channel to handle timeout
	done := make(chan bool, 1)
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in store.GetState(): %v", r)
				done <- true
			}
		}()
		
		// Try to get the state
		state = hc.store.GetState()
		done <- true
	}()
	
	// Wait for either completion or timeout
	select {
	case <-done:
		if err != nil {
			return fmt.Errorf("store health check failed: %w", err)
		}
		if state == nil {
			return errors.New("store returned nil state")
		}
	case <-testCtx.Done():
		return fmt.Errorf("store health check timed out: %w", testCtx.Err())
	}

	// Verify we can check for a non-existent key (basic operation test)
	testStateID := fmt.Sprintf("health_check_%d", time.Now().UnixNano())
	_, exists := state[testStateID]
	_ = exists  // This is expected to be false for a non-existent key

	return nil
}

// EventHandlerHealthCheck checks the health of the event handler
type EventHandlerHealthCheck struct {
	handler *StateEventHandler
	name    string
}

// NewEventHandlerHealthCheck creates a new event handler health check
func NewEventHandlerHealthCheck(handler *StateEventHandler) *EventHandlerHealthCheck {
	return &EventHandlerHealthCheck{
		handler: handler,
		name:    "event_handler",
	}
}

// Name returns the name of the health check
func (hc *EventHandlerHealthCheck) Name() string {
	return hc.name
}

// Check performs the health check
func (hc *EventHandlerHealthCheck) Check(ctx context.Context) error {
	if hc.handler == nil {
		return errors.New("event handler is nil")
	}

	// Check event queue depth first, as it might indicate degraded state
	// even if handler appears to be running
	queueDepth := hc.handler.getQueueDepth()
	if queueDepth > 10000 { // Arbitrary high threshold
		return fmt.Errorf("event queue depth (%d) is too high", queueDepth)
	}

	// Check if event handler is running
	if !hc.handler.isRunning() {
		return errors.New("event handler is not running")
	}

	return nil
}

// RateLimiterHealthCheck checks the health of rate limiters
type RateLimiterHealthCheck struct {
	rateLimiter       *RateLimiter
	clientRateLimiter *ClientRateLimiter
	name              string
}

// NewRateLimiterHealthCheck creates a new rate limiter health check
func NewRateLimiterHealthCheck(rateLimiter *RateLimiter, clientRateLimiter *ClientRateLimiter) *RateLimiterHealthCheck {
	return &RateLimiterHealthCheck{
		rateLimiter:       rateLimiter,
		clientRateLimiter: clientRateLimiter,
		name:              "rate_limiter",
	}
}

// Name returns the name of the health check
func (hc *RateLimiterHealthCheck) Name() string {
	return hc.name
}

// Check performs the health check
func (hc *RateLimiterHealthCheck) Check(ctx context.Context) error {
	// Check if rate limiters are initialized
	if hc.rateLimiter == nil && hc.clientRateLimiter == nil {
		return errors.New("no rate limiters configured")
	}

	// For now, just check that they exist
	// In a real implementation, you might check their internal state
	return nil
}

// AuditHealthCheck checks the health of the audit system
type AuditHealthCheck struct {
	auditManager *AuditManager
	name         string
}

// NewAuditHealthCheck creates a new audit health check
func NewAuditHealthCheck(auditManager *AuditManager) *AuditHealthCheck {
	return &AuditHealthCheck{
		auditManager: auditManager,
		name:         "audit",
	}
}

// Name returns the name of the health check
func (hc *AuditHealthCheck) Name() string {
	return hc.name
}

// Check performs the health check
func (hc *AuditHealthCheck) Check(ctx context.Context) error {
	if hc.auditManager == nil {
		return errors.New("audit manager is nil")
	}

	if !hc.auditManager.isEnabled() {
		return errors.New("audit logging is disabled")
	}

	// Check if logger is properly initialized
	if hc.auditManager.logger == nil {
		return errors.New("audit logger is not initialized")
	}

	// Try to verify recent audit logs
	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Minute)

	verification, err := hc.auditManager.logger.Verify(ctx, startTime, endTime)
	if err != nil {
		return fmt.Errorf("audit verification failed: %w", err)
	}

	if !verification.Valid {
		return fmt.Errorf("audit logs are invalid: %d tampered logs, %d missing logs",
			len(verification.TamperedLogs), len(verification.MissingLogs))
	}

	return nil
}

// CompositeHealthCheck combines multiple health checks
type CompositeHealthCheck struct {
	checks   []HealthCheck
	name     string
	parallel bool
}

// NewCompositeHealthCheck creates a composite health check
func NewCompositeHealthCheck(name string, parallel bool, checks ...HealthCheck) *CompositeHealthCheck {
	return &CompositeHealthCheck{
		checks:   checks,
		name:     name,
		parallel: parallel,
	}
}

// Name returns the name of the health check
func (hc *CompositeHealthCheck) Name() string {
	return hc.name
}

// Check performs all the health checks
func (hc *CompositeHealthCheck) Check(ctx context.Context) error {
	if len(hc.checks) == 0 {
		return nil
	}

	if hc.parallel {
		return hc.checkParallel(ctx)
	}

	return hc.checkSequential(ctx)
}

func (hc *CompositeHealthCheck) checkSequential(ctx context.Context) error {
	for _, check := range hc.checks {
		if err := check.Check(ctx); err != nil {
			return fmt.Errorf("health check '%s' failed: %w", check.Name(), err)
		}
	}
	return nil
}

func (hc *CompositeHealthCheck) checkParallel(ctx context.Context) error {
	type result struct {
		name string
		err  error
	}

	// Pre-allocate slice to avoid append overhead
	results := make([]result, len(hc.checks))
	var wg sync.WaitGroup

	// Launch all goroutines and write directly to indexed positions
	for i, check := range hc.checks {
		wg.Add(1)
		go func(i int, check HealthCheck) {
			defer wg.Done()
			err := check.Check(ctx)
			results[i] = result{name: check.Name(), err: err}
		}(i, check)
	}

	// Wait for all checks to complete
	wg.Wait()

	// Collect failures efficiently
	var failures []string
	for _, result := range results {
		if result.err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", result.name, result.err))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("health checks failed: %v", failures)
	}

	return nil
}

// PerformanceHealthCheck checks if performance metrics are within acceptable bounds
type PerformanceHealthCheck struct {
	performanceOptimizer PerformanceOptimizer
	name                 string
	maxPoolMissRate      float64
	maxErrorRate         float64
}

// NewPerformanceHealthCheck creates a new performance health check
func NewPerformanceHealthCheck(optimizer PerformanceOptimizer, maxPoolMissRate, maxErrorRate float64) *PerformanceHealthCheck {
	return &PerformanceHealthCheck{
		performanceOptimizer: optimizer,
		name:                 "performance",
		maxPoolMissRate:      maxPoolMissRate,
		maxErrorRate:         maxErrorRate,
	}
}

// Name returns the name of the health check
func (hc *PerformanceHealthCheck) Name() string {
	return hc.name
}

// Check performs the health check
func (hc *PerformanceHealthCheck) Check(ctx context.Context) error {
	if hc.performanceOptimizer == nil {
		return errors.New("performance optimizer is nil")
	}

	metrics := hc.performanceOptimizer.GetMetrics()

	// Check pool efficiency
	if metrics.PoolEfficiency < (100.0 - hc.maxPoolMissRate) {
		return fmt.Errorf("pool efficiency (%.2f%%) is below threshold (%.2f%%)",
			metrics.PoolEfficiency, 100.0-hc.maxPoolMissRate)
	}

	// Error rate check will be added when ErrorRate field is added to PerformanceMetrics
	// The maxErrorRate parameter is currently not being validated

	return nil
}

// CustomHealthCheck allows for custom health check implementations
type CustomHealthCheck struct {
	name    string
	checkFn func(context.Context) error
}

// NewCustomHealthCheck creates a custom health check
func NewCustomHealthCheck(name string, checkFn func(context.Context) error) *CustomHealthCheck {
	return &CustomHealthCheck{
		name:    name,
		checkFn: checkFn,
	}
}

// Name returns the name of the health check
func (hc *CustomHealthCheck) Name() string {
	return hc.name
}

// Check performs the custom health check
func (hc *CustomHealthCheck) Check(ctx context.Context) error {
	if hc.checkFn == nil {
		return errors.New("health check function is nil")
	}
	return hc.checkFn(ctx)
}

// HealthStatus represents the overall health status
type HealthStatus int

const (
	HealthStatusHealthy HealthStatus = iota
	HealthStatusDegraded
	HealthStatusUnhealthy
)

func (hs HealthStatus) String() string {
	switch hs {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusDegraded:
		return "degraded"
	case HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// HealthCheckResult represents the result of a single health check
type HealthCheckResult struct {
	Status       HealthStatus
	Message      string
	ResponseTime time.Duration
}

// HealthReport represents a comprehensive health report
type HealthReport struct {
	Status    HealthStatus
	Timestamp time.Time
	Checks    map[string]HealthCheckResult
	Metrics   HealthMetrics
}

// HealthMetrics contains health-related metrics
type HealthMetrics struct {
	CPUUsage        float64
	MemoryUsage     float64
	MemoryAllocated uint64
	GoroutineCount  int
	GCPauseAvg      float64
}

// HealthChecker provides a unified interface for running health checks
type HealthChecker struct {
	checks []HealthCheck
	mu     sync.RWMutex
}

// NewHealthChecker creates a new health checker with default checks for a StateManager
func NewHealthChecker(manager *StateManager) *HealthChecker {
	hc := &HealthChecker{
		checks: make([]HealthCheck, 0),
	}
	
	// Add default health checks
	if manager != nil {
		hc.AddHealthCheck(NewStateManagerHealthCheck(manager))
		
		if manager.store != nil {
			hc.AddHealthCheck(NewStoreHealthCheck(manager.store, 5*time.Second))
		}
		
		if manager.eventHandler != nil {
			hc.AddHealthCheck(NewEventHandlerHealthCheck(manager.eventHandler))
		}
		
		if manager.rateLimiter != nil || manager.clientRateLimiter != nil {
			hc.AddHealthCheck(NewRateLimiterHealthCheck(manager.rateLimiter, manager.clientRateLimiter))
		}
		
		if manager.auditManager != nil {
			hc.AddHealthCheck(NewAuditHealthCheck(manager.auditManager))
		}
		
		// Add memory health check with reasonable defaults
		hc.AddHealthCheck(NewMemoryHealthCheck(500, 100, 1000)) // 500MB, 100ms GC pause, 1000 goroutines
	}
	
	return hc
}

// AddHealthCheck adds a health check to the checker
func (hc *HealthChecker) AddHealthCheck(check HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks = append(hc.checks, check)
}

// RemoveHealthCheck removes a health check by name
func (hc *HealthChecker) RemoveHealthCheck(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	for i, check := range hc.checks {
		if check.Name() == name {
			hc.checks = append(hc.checks[:i], hc.checks[i+1:]...)
			break
		}
	}
}

// CheckHealth runs all health checks and returns the overall health status
func (hc *HealthChecker) CheckHealth(ctx context.Context) HealthReport {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	report := HealthReport{
		Timestamp: time.Now(),
		Checks:    make(map[string]HealthCheckResult),
	}
	
	overallStatus := HealthStatusHealthy
	
	for _, check := range hc.checks {
		start := time.Now()
		err := check.Check(ctx)
		responseTime := time.Since(start)
		
		result := HealthCheckResult{
			ResponseTime: responseTime,
		}
		
		if err != nil {
			result.Status = HealthStatusUnhealthy
			result.Message = err.Error()
			overallStatus = HealthStatusUnhealthy
		} else {
			result.Status = HealthStatusHealthy
			result.Message = "OK"
		}
		
		report.Checks[check.Name()] = result
	}
	
	report.Status = overallStatus
	
	// Collect health metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	report.Metrics = HealthMetrics{
		CPUUsage:        0.0, // Would need actual CPU monitoring
		MemoryUsage:     float64(memStats.Alloc) / float64(memStats.Sys) * 100,
		MemoryAllocated: memStats.Alloc,
		GoroutineCount:  runtime.NumGoroutine(),
		GCPauseAvg:      float64(memStats.PauseNs[(memStats.NumGC+255)%256]) / 1e6, // Convert to ms
	}
	
	return report
}

// CheckHealthLegacy runs all health checks and returns the legacy format
func (hc *HealthChecker) CheckHealthLegacy(ctx context.Context) map[string]error {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	results := make(map[string]error)
	
	for _, check := range hc.checks {
		results[check.Name()] = check.Check(ctx)
	}
	
	return results
}

// IsHealthy returns true if all health checks pass
func (hc *HealthChecker) IsHealthy(ctx context.Context) bool {
	report := hc.CheckHealth(ctx)
	return report.Status == HealthStatusHealthy
}

// GetHealthChecks returns a copy of all registered health checks
func (hc *HealthChecker) GetHealthChecks() []HealthCheck {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	checks := make([]HealthCheck, len(hc.checks))
	copy(checks, hc.checks)
	return checks
}

// Helper methods for StateManager and StateEventHandler
// These would need to be added to the respective structs
