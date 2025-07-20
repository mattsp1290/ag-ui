package transport

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// CleanupPhase represents phases of cleanup process
type CleanupPhase int

const (
	// CleanupPhaseNone indicates no cleanup in progress
	CleanupPhaseNone CleanupPhase = iota
	// CleanupPhaseInitiated indicates cleanup has been initiated
	CleanupPhaseInitiated
	// CleanupPhaseStoppingWorkers indicates stopping worker goroutines
	CleanupPhaseStoppingWorkers
	// CleanupPhaseClosingConnections indicates closing network connections
	CleanupPhaseClosingConnections
	// CleanupPhaseReleasingResources indicates releasing memory and file resources
	CleanupPhaseReleasingResources
	// CleanupPhaseFinalizing indicates final cleanup steps
	CleanupPhaseFinalizing
	// CleanupPhaseComplete indicates cleanup is complete
	CleanupPhaseComplete
)

// String returns string representation of cleanup phase
func (p CleanupPhase) String() string {
	switch p {
	case CleanupPhaseNone:
		return "none"
	case CleanupPhaseInitiated:
		return "initiated"
	case CleanupPhaseStoppingWorkers:
		return "stopping_workers"
	case CleanupPhaseClosingConnections:
		return "closing_connections"
	case CleanupPhaseReleasingResources:
		return "releasing_resources"
	case CleanupPhaseFinalizing:
		return "finalizing"
	case CleanupPhaseComplete:
		return "complete"
	default:
		return "unknown"
	}
}

// ResourceType represents different types of resources to track
type ResourceType string

const (
	ResourceTypeGoroutine   ResourceType = "goroutine"
	ResourceTypeConnection  ResourceType = "connection"
	ResourceTypeChannel     ResourceType = "channel"
	ResourceTypeTimer       ResourceType = "timer"
	ResourceTypeFile        ResourceType = "file"
	ResourceTypeMemory      ResourceType = "memory"
	ResourceTypeHandler     ResourceType = "handler"
	ResourceTypeSubscription ResourceType = "subscription"
)

// TrackedResource represents a tracked resource
type TrackedResource struct {
	ID           string
	Type         ResourceType
	Description  string
	CreatedAt    time.Time
	CleanedAt    time.Time
	Cleaned      bool
	CleanupFunc  func() error
	Dependencies []string // IDs of resources that must be cleaned first
	StackTrace   string   // For debugging resource leaks
}

// CleanupTracker tracks resources and ensures proper cleanup order
type CleanupTracker struct {
	mu        sync.RWMutex
	resources map[string]*TrackedResource
	phase     int32 // atomic access to CleanupPhase
	
	// Cleanup statistics
	stats struct {
		totalTracked      int64
		totalCleaned      int64
		cleanupErrors     int64
		cleanupStartTime  time.Time
		cleanupEndTime    time.Time
		phaseStartTimes   map[CleanupPhase]time.Time
		phaseDurations    map[CleanupPhase]time.Duration
	}
	
	// Cleanup configuration
	config CleanupConfig
	
	// Cleanup coordination
	cleanupOnce sync.Once
	cleanupDone chan struct{}
	cleanupErr  error
}

// CleanupConfig configures cleanup behavior
type CleanupConfig struct {
	// MaxCleanupDuration is the maximum time allowed for cleanup
	MaxCleanupDuration time.Duration
	
	// PhaseTimeout is the timeout for each cleanup phase
	PhaseTimeout time.Duration
	
	// EnableStackTrace enables stack trace capture for debugging
	EnableStackTrace bool
	
	// Logger for cleanup events
	Logger Logger
}

// DefaultCleanupConfig returns default cleanup configuration
func DefaultCleanupConfig() CleanupConfig {
	return CleanupConfig{
		MaxCleanupDuration: 30 * time.Second,
		PhaseTimeout:       5 * time.Second,
		EnableStackTrace:   false,
		Logger:            nil,
	}
}

// NewCleanupTracker creates a new cleanup tracker
func NewCleanupTracker(config CleanupConfig) *CleanupTracker {
	ct := &CleanupTracker{
		resources:   make(map[string]*TrackedResource),
		config:      config,
		cleanupDone: make(chan struct{}),
	}
	
	ct.stats.phaseStartTimes = make(map[CleanupPhase]time.Time)
	ct.stats.phaseDurations = make(map[CleanupPhase]time.Duration)
	
	return ct
}

// Track registers a resource for cleanup tracking
func (ct *CleanupTracker) Track(id string, resourceType ResourceType, description string, cleanupFunc func() error, dependencies ...string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	resource := &TrackedResource{
		ID:           id,
		Type:         resourceType,
		Description:  description,
		CreatedAt:    time.Now(),
		CleanupFunc:  cleanupFunc,
		Dependencies: dependencies,
	}
	
	// Capture stack trace if enabled
	if ct.config.EnableStackTrace {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		resource.StackTrace = string(buf[:n])
	}
	
	ct.resources[id] = resource
	atomic.AddInt64(&ct.stats.totalTracked, 1)
	
	if ct.config.Logger != nil {
		ct.config.Logger.Debug("Resource tracked",
			String("id", id),
			String("type", string(resourceType)),
			String("description", description),
			Any("dependencies", dependencies))
	}
}

// Untrack removes a resource from tracking (used when resource is cleaned up normally)
func (ct *CleanupTracker) Untrack(id string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	if resource, exists := ct.resources[id]; exists {
		resource.Cleaned = true
		resource.CleanedAt = time.Now()
		delete(ct.resources, id)
		atomic.AddInt64(&ct.stats.totalCleaned, 1)
		
		if ct.config.Logger != nil {
			ct.config.Logger.Debug("Resource untracked",
				String("id", id),
				String("type", string(resource.Type)),
				Duration("lifetime", time.Since(resource.CreatedAt)))
		}
	}
}

// GetPhase returns the current cleanup phase
func (ct *CleanupTracker) GetPhase() CleanupPhase {
	return CleanupPhase(atomic.LoadInt32(&ct.phase))
}

// setPhase sets the current cleanup phase
func (ct *CleanupTracker) setPhase(phase CleanupPhase) {
	oldPhase := CleanupPhase(atomic.SwapInt32(&ct.phase, int32(phase)))
	
	// Record phase timing
	now := time.Now()
	ct.stats.phaseStartTimes[phase] = now
	if oldPhase != CleanupPhaseNone {
		if startTime, exists := ct.stats.phaseStartTimes[oldPhase]; exists {
			ct.stats.phaseDurations[oldPhase] = now.Sub(startTime)
		}
	}
	
	if ct.config.Logger != nil {
		ct.config.Logger.Info("Cleanup phase changed",
			String("from", oldPhase.String()),
			String("to", phase.String()))
	}
}

// Cleanup performs ordered cleanup of all tracked resources
func (ct *CleanupTracker) Cleanup(ctx context.Context) error {
	var cleanupErr error
	
	ct.cleanupOnce.Do(func() {
		ct.stats.cleanupStartTime = time.Now()
		ct.setPhase(CleanupPhaseInitiated)
		
		// Create cleanup context with timeout
		cleanupCtx, cancel := context.WithTimeout(ctx, ct.config.MaxCleanupDuration)
		defer cancel()
		
		// Perform cleanup in phases
		cleanupErr = ct.performPhaseCleanup(cleanupCtx)
		
		ct.stats.cleanupEndTime = time.Now()
		ct.setPhase(CleanupPhaseComplete)
		close(ct.cleanupDone)
		
		if ct.config.Logger != nil {
			ct.config.Logger.Info("Cleanup completed",
				Duration("duration", ct.stats.cleanupEndTime.Sub(ct.stats.cleanupStartTime)),
				Int64("cleaned", atomic.LoadInt64(&ct.stats.totalCleaned)),
				Int64("errors", atomic.LoadInt64(&ct.stats.cleanupErrors)),
				Err(cleanupErr))
		}
	})
	
	ct.cleanupErr = cleanupErr
	return cleanupErr
}

// performPhaseCleanup executes cleanup in proper order
func (ct *CleanupTracker) performPhaseCleanup(ctx context.Context) error {
	phases := []struct {
		phase CleanupPhase
		types []ResourceType
	}{
		{
			phase: CleanupPhaseStoppingWorkers,
			types: []ResourceType{ResourceTypeGoroutine, ResourceTypeTimer},
		},
		{
			phase: CleanupPhaseClosingConnections,
			types: []ResourceType{ResourceTypeConnection, ResourceTypeSubscription},
		},
		{
			phase: CleanupPhaseReleasingResources,
			types: []ResourceType{ResourceTypeChannel, ResourceTypeHandler, ResourceTypeFile, ResourceTypeMemory},
		},
	}
	
	var firstErr error
	
	for _, p := range phases {
		select {
		case <-ctx.Done():
			return fmt.Errorf("cleanup cancelled during phase %s: %w", p.phase, ctx.Err())
		default:
		}
		
		ct.setPhase(p.phase)
		
		// Create phase context with timeout
		phaseCtx, cancel := context.WithTimeout(ctx, ct.config.PhaseTimeout)
		
		// Clean resources of specified types
		if err := ct.cleanResourcesByType(phaseCtx, p.types...); err != nil && firstErr == nil {
			firstErr = err
		}
		
		cancel()
	}
	
	// Final cleanup phase
	ct.setPhase(CleanupPhaseFinalizing)
	
	// Clean any remaining resources
	if err := ct.cleanRemainingResources(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	
	return firstErr
}

// cleanResourcesByType cleans resources of specified types
func (ct *CleanupTracker) cleanResourcesByType(ctx context.Context, types ...ResourceType) error {
	ct.mu.Lock()
	
	// Build list of resources to clean
	var toClean []*TrackedResource
	typeSet := make(map[ResourceType]bool)
	for _, t := range types {
		typeSet[t] = true
	}
	
	for _, resource := range ct.resources {
		if typeSet[resource.Type] && !resource.Cleaned {
			toClean = append(toClean, resource)
		}
	}
	
	ct.mu.Unlock()
	
	// Sort by dependencies
	toClean = ct.sortByDependencies(toClean)
	
	// Clean resources
	var firstErr error
	for _, resource := range toClean {
		select {
		case <-ctx.Done():
			return fmt.Errorf("cleanup cancelled while cleaning %s: %w", resource.ID, ctx.Err())
		default:
		}
		
		if err := ct.cleanResource(resource); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			atomic.AddInt64(&ct.stats.cleanupErrors, 1)
		}
	}
	
	return firstErr
}

// cleanRemainingResources cleans any remaining tracked resources
func (ct *CleanupTracker) cleanRemainingResources(ctx context.Context) error {
	ct.mu.Lock()
	
	var remaining []*TrackedResource
	for _, resource := range ct.resources {
		if !resource.Cleaned {
			remaining = append(remaining, resource)
		}
	}
	
	ct.mu.Unlock()
	
	if len(remaining) == 0 {
		return nil
	}
	
	if ct.config.Logger != nil {
		ct.config.Logger.Warn("Cleaning remaining resources",
			Int("count", len(remaining)))
	}
	
	// Sort by dependencies
	remaining = ct.sortByDependencies(remaining)
	
	var firstErr error
	for _, resource := range remaining {
		select {
		case <-ctx.Done():
			return fmt.Errorf("cleanup cancelled while cleaning remaining resource %s: %w", resource.ID, ctx.Err())
		default:
		}
		
		if err := ct.cleanResource(resource); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			atomic.AddInt64(&ct.stats.cleanupErrors, 1)
		}
	}
	
	return firstErr
}

// cleanResource cleans a single resource
func (ct *CleanupTracker) cleanResource(resource *TrackedResource) error {
	if resource.Cleaned {
		return nil
	}
	
	if ct.config.Logger != nil {
		ct.config.Logger.Debug("Cleaning resource",
			String("id", resource.ID),
			String("type", string(resource.Type)),
			String("description", resource.Description))
	}
	
	var err error
	if resource.CleanupFunc != nil {
		// Add timeout protection for cleanup function
		done := make(chan error, 1)
		go func() {
			done <- resource.CleanupFunc()
		}()
		
		select {
		case err = <-done:
		case <-time.After(5 * time.Second):
			err = fmt.Errorf("cleanup timeout for resource %s", resource.ID)
		}
	}
	
	ct.mu.Lock()
	resource.Cleaned = true
	resource.CleanedAt = time.Now()
	delete(ct.resources, resource.ID)
	ct.mu.Unlock()
	
	atomic.AddInt64(&ct.stats.totalCleaned, 1)
	
	if err != nil {
		if ct.config.Logger != nil {
			ct.config.Logger.Error("Resource cleanup failed",
				String("id", resource.ID),
				String("type", string(resource.Type)),
				Err(err))
		}
		return fmt.Errorf("failed to clean %s resource %s: %w", resource.Type, resource.ID, err)
	}
	
	return nil
}

// sortByDependencies sorts resources to respect dependency order
func (ct *CleanupTracker) sortByDependencies(resources []*TrackedResource) []*TrackedResource {
	// Simple topological sort - resources with no dependencies first
	var sorted []*TrackedResource
	processed := make(map[string]bool)
	
	// First pass - resources with no dependencies
	for _, resource := range resources {
		if len(resource.Dependencies) == 0 {
			sorted = append(sorted, resource)
			processed[resource.ID] = true
		}
	}
	
	// Subsequent passes - resources whose dependencies are processed
	changed := true
	for changed {
		changed = false
		for _, resource := range resources {
			if processed[resource.ID] {
				continue
			}
			
			allDepsProcessed := true
			for _, dep := range resource.Dependencies {
				if !processed[dep] {
					allDepsProcessed = false
					break
				}
			}
			
			if allDepsProcessed {
				sorted = append(sorted, resource)
				processed[resource.ID] = true
				changed = true
			}
		}
	}
	
	// Add any remaining resources (circular dependencies)
	for _, resource := range resources {
		if !processed[resource.ID] {
			sorted = append(sorted, resource)
		}
	}
	
	return sorted
}

// Wait waits for cleanup to complete
func (ct *CleanupTracker) Wait() error {
	<-ct.cleanupDone
	return ct.cleanupErr
}

// GetStats returns cleanup statistics
func (ct *CleanupTracker) GetStats() CleanupStats {
	return CleanupStats{
		TotalTracked:     atomic.LoadInt64(&ct.stats.totalTracked),
		TotalCleaned:     atomic.LoadInt64(&ct.stats.totalCleaned),
		CleanupErrors:    atomic.LoadInt64(&ct.stats.cleanupErrors),
		CleanupDuration:  ct.stats.cleanupEndTime.Sub(ct.stats.cleanupStartTime),
		PhaseDurations:   ct.stats.phaseDurations,
		CurrentPhase:     ct.GetPhase(),
		ResourcesLeaked:  ct.getLeakedResources(),
	}
}

// getLeakedResources returns resources that were not cleaned
func (ct *CleanupTracker) getLeakedResources() []LeakedResource {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	var leaked []LeakedResource
	for _, resource := range ct.resources {
		if !resource.Cleaned {
			leaked = append(leaked, LeakedResource{
				ID:          resource.ID,
				Type:        resource.Type,
				Description: resource.Description,
				CreatedAt:   resource.CreatedAt,
				StackTrace:  resource.StackTrace,
			})
		}
	}
	
	return leaked
}

// CleanupStats contains cleanup statistics
type CleanupStats struct {
	TotalTracked    int64
	TotalCleaned    int64
	CleanupErrors   int64
	CleanupDuration time.Duration
	PhaseDurations  map[CleanupPhase]time.Duration
	CurrentPhase    CleanupPhase
	ResourcesLeaked []LeakedResource
}

// LeakedResource represents a resource that was not properly cleaned
type LeakedResource struct {
	ID          string
	Type        ResourceType
	Description string
	CreatedAt   time.Time
	StackTrace  string
}