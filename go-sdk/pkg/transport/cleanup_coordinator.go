package transport

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CleanupCoordinator coordinates cleanup across multiple components to prevent deadlocks
type CleanupCoordinator struct {
	mu       sync.RWMutex
	tracker  *CleanupTracker
	groups   map[string]*CleanupGroup
	sequence []string // Ordered list of group IDs for cleanup

	// Coordination state
	state       int32 // atomic access to CoordinatorState
	cleanupOnce sync.Once
	cleanupDone chan struct{}
	cleanupErr  error

	// Configuration
	config CoordinatorConfig
}

// CoordinatorState represents the state of the cleanup coordinator
type CoordinatorState int32

const (
	CoordinatorStateActive CoordinatorState = iota
	CoordinatorStateShuttingDown
	CoordinatorStateStopped
)

// CoordinatorConfig configures the cleanup coordinator
type CoordinatorConfig struct {
	// MaxParallelCleanups limits concurrent cleanup operations
	MaxParallelCleanups int

	// GroupTimeout is the timeout for each cleanup group
	GroupTimeout time.Duration

	// EnableDeadlockDetection enables deadlock detection
	EnableDeadlockDetection bool

	// Logger for coordinator events
	Logger Logger
}

// DefaultCoordinatorConfig returns default coordinator configuration
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		MaxParallelCleanups:     4,
		GroupTimeout:            10 * time.Second,
		EnableDeadlockDetection: true,
		Logger:                  nil,
	}
}

// CleanupGroup represents a group of related cleanup operations
type CleanupGroup struct {
	ID           string
	Name         string
	Priority     int      // Lower values = higher priority
	Dependencies []string // IDs of groups that must complete first
	Operations   []CleanupOperation

	// State tracking
	state     int32 // atomic access to GroupState
	startTime time.Time
	endTime   time.Time
	errors    []error
	mu        sync.Mutex
}

// GroupState represents the state of a cleanup group
type GroupState int32

const (
	GroupStatePending GroupState = iota
	GroupStateRunning
	GroupStateComplete
	GroupStateFailed
)

// CleanupOperation represents a single cleanup operation
type CleanupOperation struct {
	Name        string
	Description string
	Timeout     time.Duration
	Func        func(context.Context) error

	// Options
	ContinueOnError bool // Continue with next operation even if this fails
	Retryable       bool // Can retry if fails
	MaxRetries      int  // Maximum retry attempts
}

// NewCleanupCoordinator creates a new cleanup coordinator
func NewCleanupCoordinator(config CoordinatorConfig) *CleanupCoordinator {
	tracker := NewCleanupTracker(CleanupConfig{
		MaxCleanupDuration: 60 * time.Second,
		PhaseTimeout:       10 * time.Second,
		EnableStackTrace:   config.EnableDeadlockDetection,
		Logger:             config.Logger,
	})

	return &CleanupCoordinator{
		tracker:     tracker,
		groups:      make(map[string]*CleanupGroup),
		cleanupDone: make(chan struct{}),
		config:      config,
	}
}

// RegisterGroup registers a cleanup group
func (cc *CleanupCoordinator) RegisterGroup(group *CleanupGroup) error {
	if atomic.LoadInt32(&cc.state) != int32(CoordinatorStateActive) {
		return fmt.Errorf("coordinator is not active")
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	if _, exists := cc.groups[group.ID]; exists {
		return fmt.Errorf("group %s already registered", group.ID)
	}

	// Validate dependencies exist
	for _, dep := range group.Dependencies {
		if _, exists := cc.groups[dep]; !exists {
			return fmt.Errorf("dependency %s not found for group %s", dep, group.ID)
		}
	}

	cc.groups[group.ID] = group

	// Rebuild sequence based on dependencies and priority
	cc.rebuildSequence()

	if cc.config.Logger != nil {
		cc.config.Logger.Debug("Cleanup group registered",
			String("id", group.ID),
			String("name", group.Name),
			Int("priority", group.Priority),
			Any("dependencies", group.Dependencies),
			Int("operations", len(group.Operations)))
	}

	return nil
}

// rebuildSequence rebuilds the cleanup sequence based on dependencies and priority
func (cc *CleanupCoordinator) rebuildSequence() {
	// Topological sort with priority consideration
	var sequence []string
	visited := make(map[string]bool)
	tempMarked := make(map[string]bool)

	// Get all groups sorted by priority
	groups := cc.getGroupsSortedByPriority()

	var visit func(id string) error
	visit = func(id string) error {
		if tempMarked[id] {
			// Circular dependency detected
			return fmt.Errorf("circular dependency detected involving group %s", id)
		}

		if visited[id] {
			return nil
		}

		tempMarked[id] = true

		group := cc.groups[id]
		// Visit dependencies first (in reverse order for cleanup)
		for i := len(group.Dependencies) - 1; i >= 0; i-- {
			if err := visit(group.Dependencies[i]); err != nil {
				return err
			}
		}

		tempMarked[id] = false
		visited[id] = true
		sequence = append(sequence, id)

		return nil
	}

	// Visit all groups
	for _, group := range groups {
		if err := visit(group.ID); err != nil {
			if cc.config.Logger != nil {
				cc.config.Logger.Error("Failed to build cleanup sequence",
					Err(err))
			}
			// Fall back to simple priority-based ordering
			sequence = nil
			for _, g := range groups {
				sequence = append(sequence, g.ID)
			}
			break
		}
	}

	// Reverse sequence for cleanup (dependencies cleaned last)
	for i, j := 0, len(sequence)-1; i < j; i, j = i+1, j-1 {
		sequence[i], sequence[j] = sequence[j], sequence[i]
	}

	cc.sequence = sequence
}

// getGroupsSortedByPriority returns groups sorted by priority
func (cc *CleanupCoordinator) getGroupsSortedByPriority() []*CleanupGroup {
	var groups []*CleanupGroup
	for _, group := range cc.groups {
		groups = append(groups, group)
	}

	// Sort by priority (lower value = higher priority)
	for i := 0; i < len(groups); i++ {
		for j := i + 1; j < len(groups); j++ {
			if groups[j].Priority < groups[i].Priority {
				groups[i], groups[j] = groups[j], groups[i]
			}
		}
	}

	return groups
}

// Shutdown initiates coordinated cleanup
func (cc *CleanupCoordinator) Shutdown(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&cc.state, int32(CoordinatorStateActive), int32(CoordinatorStateShuttingDown)) {
		return fmt.Errorf("coordinator is not active")
	}

	var cleanupErr error

	cc.cleanupOnce.Do(func() {
		startTime := time.Now()

		if cc.config.Logger != nil {
			cc.config.Logger.Info("Starting coordinated cleanup",
				Int("groups", len(cc.groups)),
				Any("sequence", cc.sequence))
		}

		// Execute cleanup sequence
		cleanupErr = cc.executeCleanupSequence(ctx)

		// Perform final tracker cleanup
		if err := cc.tracker.Cleanup(ctx); err != nil && cleanupErr == nil {
			cleanupErr = err
		}

		atomic.StoreInt32(&cc.state, int32(CoordinatorStateStopped))
		close(cc.cleanupDone)

		if cc.config.Logger != nil {
			cc.config.Logger.Info("Coordinated cleanup completed",
				Duration("duration", time.Since(startTime)),
				Err(cleanupErr))
		}
	})

	cc.cleanupErr = cleanupErr
	return cleanupErr
}

// executeCleanupSequence executes the cleanup sequence
func (cc *CleanupCoordinator) executeCleanupSequence(ctx context.Context) error {
	cc.mu.RLock()
	sequence := make([]string, len(cc.sequence))
	copy(sequence, cc.sequence)
	cc.mu.RUnlock()

	// Create semaphore for parallel execution control
	sem := make(chan struct{}, cc.config.MaxParallelCleanups)

	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	// Execute groups in sequence, with parallelism where possible
	completedGroups := make(map[string]bool)

	for _, groupID := range sequence {
		cc.mu.RLock()
		group, exists := cc.groups[groupID]
		cc.mu.RUnlock()

		if !exists {
			continue
		}

		// Wait for dependencies to complete
		for _, dep := range group.Dependencies {
			for !completedGroups[dep] {
				select {
				case <-ctx.Done():
					return fmt.Errorf("cleanup cancelled waiting for dependency %s: %w", dep, ctx.Err())
				case <-time.After(100 * time.Millisecond):
					// Check again
				}
			}
		}

		// Execute group cleanup
		wg.Add(1)
		go func(g *CleanupGroup) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("cleanup cancelled for group %s: %w", g.ID, ctx.Err())
				}
				errMu.Unlock()
				return
			}

			// Execute group
			if err := cc.executeGroup(ctx, g); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}

			// Mark as completed
			completedGroups[g.ID] = true
		}(group)

		// Check for cancellation
		select {
		case <-ctx.Done():
			wg.Wait()
			return fmt.Errorf("cleanup sequence cancelled: %w", ctx.Err())
		default:
		}
	}

	// Wait for all groups to complete
	wg.Wait()

	return firstErr
}

// executeGroup executes all operations in a cleanup group
func (cc *CleanupCoordinator) executeGroup(ctx context.Context, group *CleanupGroup) error {
	atomic.StoreInt32(&group.state, int32(GroupStateRunning))
	group.startTime = time.Now()

	if cc.config.Logger != nil {
		cc.config.Logger.Info("Executing cleanup group",
			String("id", group.ID),
			String("name", group.Name),
			Int("operations", len(group.Operations)))
	}

	// Create group context with timeout
	groupCtx, cancel := context.WithTimeout(ctx, cc.config.GroupTimeout)
	defer cancel()

	var firstErr error

	for i, op := range group.Operations {
		select {
		case <-groupCtx.Done():
			err := fmt.Errorf("group %s cancelled during operation %d: %w", group.ID, i, groupCtx.Err())
			group.mu.Lock()
			group.errors = append(group.errors, err)
			group.mu.Unlock()
			atomic.StoreInt32(&group.state, int32(GroupStateFailed))
			return err
		default:
		}

		// Execute operation with retries if configured
		err := cc.executeOperation(groupCtx, op)

		if err != nil {
			group.mu.Lock()
			group.errors = append(group.errors, err)
			group.mu.Unlock()

			if firstErr == nil {
				firstErr = err
			}

			if !op.ContinueOnError {
				atomic.StoreInt32(&group.state, int32(GroupStateFailed))
				return fmt.Errorf("cleanup operation %s failed in group %s: %w", op.Name, group.ID, err)
			}
		}
	}

	group.endTime = time.Now()

	if firstErr != nil {
		atomic.StoreInt32(&group.state, int32(GroupStateFailed))
	} else {
		atomic.StoreInt32(&group.state, int32(GroupStateComplete))
	}

	if cc.config.Logger != nil {
		cc.config.Logger.Info("Cleanup group completed",
			String("id", group.ID),
			String("name", group.Name),
			Duration("duration", group.endTime.Sub(group.startTime)),
			Int("errors", len(group.errors)))
	}

	return firstErr
}

// executeOperation executes a single cleanup operation with retry logic
func (cc *CleanupCoordinator) executeOperation(ctx context.Context, op CleanupOperation) error {
	opTimeout := op.Timeout
	if opTimeout == 0 {
		opTimeout = 5 * time.Second
	}

	maxRetries := op.MaxRetries
	if !op.Retryable {
		maxRetries = 0
	}

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff for retries
			backoff := time.Duration(attempt) * 100 * time.Millisecond
			if backoff > time.Second {
				backoff = time.Second
			}

			select {
			case <-ctx.Done():
				return fmt.Errorf("operation %s cancelled during retry: %w", op.Name, ctx.Err())
			case <-time.After(backoff):
			}

			if cc.config.Logger != nil {
				cc.config.Logger.Debug("Retrying cleanup operation",
					String("name", op.Name),
					Int("attempt", attempt+1),
					Int("max_retries", maxRetries))
			}
		}

		// Create operation context with timeout
		opCtx, cancel := context.WithTimeout(ctx, opTimeout)

		// Execute operation
		done := make(chan error, 1)
		go func() {
			done <- op.Func(opCtx)
		}()

		select {
		case err := <-done:
			cancel()
			if err == nil {
				return nil
			}
			lastErr = err

		case <-opCtx.Done():
			cancel()
			lastErr = fmt.Errorf("operation %s timeout after %v: %w", op.Name, opTimeout, opCtx.Err())
		}
	}

	return fmt.Errorf("operation %s failed after %d attempts: %w", op.Name, maxRetries+1, lastErr)
}

// Wait waits for cleanup to complete
func (cc *CleanupCoordinator) Wait() error {
	<-cc.cleanupDone
	return cc.cleanupErr
}

// GetGroupStatus returns the status of a cleanup group
func (cc *CleanupCoordinator) GetGroupStatus(groupID string) (GroupStatus, error) {
	cc.mu.RLock()
	group, exists := cc.groups[groupID]
	cc.mu.RUnlock()

	if !exists {
		return GroupStatus{}, fmt.Errorf("group %s not found", groupID)
	}

	group.mu.Lock()
	errors := make([]error, len(group.errors))
	copy(errors, group.errors)
	group.mu.Unlock()

	return GroupStatus{
		ID:        group.ID,
		Name:      group.Name,
		State:     GroupState(atomic.LoadInt32(&group.state)),
		StartTime: group.startTime,
		EndTime:   group.endTime,
		Duration:  group.endTime.Sub(group.startTime),
		Errors:    errors,
	}, nil
}

// GroupStatus represents the status of a cleanup group
type GroupStatus struct {
	ID        string
	Name      string
	State     GroupState
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Errors    []error
}

// GetStatus returns the overall coordinator status
func (cc *CleanupCoordinator) GetStatus() CoordinatorStatus {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	groupStatuses := make(map[string]GroupStatus)
	for id := range cc.groups {
		if status, err := cc.GetGroupStatus(id); err == nil {
			groupStatuses[id] = status
		}
	}

	return CoordinatorStatus{
		State:        CoordinatorState(atomic.LoadInt32(&cc.state)),
		Groups:       groupStatuses,
		Sequence:     cc.sequence,
		TrackerStats: cc.tracker.GetStats(),
	}
}

// CoordinatorStatus represents the overall coordinator status
type CoordinatorStatus struct {
	State        CoordinatorState
	Groups       map[string]GroupStatus
	Sequence     []string
	TrackerStats CleanupStats
}

// String returns string representation of group state
func (s GroupState) String() string {
	switch s {
	case GroupStatePending:
		return "pending"
	case GroupStateRunning:
		return "running"
	case GroupStateComplete:
		return "complete"
	case GroupStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// String returns string representation of coordinator state
func (s CoordinatorState) String() string {
	switch s {
	case CoordinatorStateActive:
		return "active"
	case CoordinatorStateShuttingDown:
		return "shutting_down"
	case CoordinatorStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}
