package transport

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// DeadlockDetector monitors for potential deadlocks in cleanup operations
type DeadlockDetector struct {
	mu             sync.RWMutex
	resources      map[string]*DeadlockResource
	waitGraph      map[string][]string // Resource ID -> waiting for Resource IDs
	detectionTimer *time.Timer
	config         DeadlockConfig
	
	// Detection state
	detecting      bool
	lastDetection  time.Time
	deadlockCount  int64
	
	// Callback for when deadlock is detected
	onDeadlock func(DeadlockInfo)
}

// DeadlockConfig configures deadlock detection
type DeadlockConfig struct {
	// DetectionInterval is how often to check for deadlocks
	DetectionInterval time.Duration
	
	// WaitTimeout is the maximum time to wait for a resource
	WaitTimeout time.Duration
	
	// EnableStackTrace captures stack traces for debugging
	EnableStackTrace bool
	
	// Logger for deadlock events
	Logger Logger
}

// DefaultDeadlockConfig returns default deadlock detection configuration
func DefaultDeadlockConfig() DeadlockConfig {
	return DeadlockConfig{
		DetectionInterval: 5 * time.Second,
		WaitTimeout:       30 * time.Second,
		EnableStackTrace:  true,
		Logger:            nil,
	}
}

// DeadlockResource represents a resource that can be involved in deadlocks
type DeadlockResource struct {
	ID          string
	Type        ResourceType
	Owner       string // Goroutine ID that owns this resource
	Waiters     []string // Goroutine IDs waiting for this resource
	AcquiredAt  time.Time
	StackTrace  string // Stack trace of acquisition
	
	// Synchronization
	mu      sync.RWMutex
	acquired bool
}

// DeadlockInfo contains information about a detected deadlock
type DeadlockInfo struct {
	Cycle       []string // Resource IDs in the deadlock cycle
	Resources   map[string]*DeadlockResource
	DetectedAt  time.Time
	Resolution  string // How the deadlock was resolved
	StackTraces map[string]string // Stack traces of involved goroutines
}

// NewDeadlockDetector creates a new deadlock detector
func NewDeadlockDetector(config DeadlockConfig) *DeadlockDetector {
	dd := &DeadlockDetector{
		resources:  make(map[string]*DeadlockResource),
		waitGraph:  make(map[string][]string),
		config:     config,
	}
	
	// Start detection timer
	dd.detectionTimer = time.NewTimer(config.DetectionInterval)
	go dd.detectionLoop()
	
	return dd
}

// RegisterResource registers a resource for deadlock tracking
func (dd *DeadlockDetector) RegisterResource(id string, resourceType ResourceType) {
	dd.mu.Lock()
	defer dd.mu.Unlock()
	
	resource := &DeadlockResource{
		ID:   id,
		Type: resourceType,
	}
	
	// Capture stack trace if enabled
	if dd.config.EnableStackTrace {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		resource.StackTrace = string(buf[:n])
	}
	
	dd.resources[id] = resource
	
	if dd.config.Logger != nil {
		dd.config.Logger.Debug("Resource registered for deadlock detection",
			String("id", id),
			Any("type", resourceType))
	}
}

// UnregisterResource removes a resource from deadlock tracking
func (dd *DeadlockDetector) UnregisterResource(id string) {
	dd.mu.Lock()
	defer dd.mu.Unlock()
	
	delete(dd.resources, id)
	delete(dd.waitGraph, id)
	
	// Remove from wait graph
	for resourceID, waitList := range dd.waitGraph {
		newWaitList := make([]string, 0, len(waitList))
		for _, waitID := range waitList {
			if waitID != id {
				newWaitList = append(newWaitList, waitID)
			}
		}
		dd.waitGraph[resourceID] = newWaitList
	}
	
	if dd.config.Logger != nil {
		dd.config.Logger.Debug("Resource unregistered from deadlock detection", String("id", id))
	}
}

// AcquireResource records that a resource has been acquired
func (dd *DeadlockDetector) AcquireResource(resourceID string, ownerID string) {
	dd.mu.Lock()
	defer dd.mu.Unlock()
	
	resource, exists := dd.resources[resourceID]
	if !exists {
		return
	}
	
	resource.mu.Lock()
	resource.acquired = true
	resource.Owner = ownerID
	resource.AcquiredAt = time.Now()
	
	// Capture stack trace if enabled
	if dd.config.EnableStackTrace {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		resource.StackTrace = string(buf[:n])
	}
	
	resource.mu.Unlock()
	
	// Remove from wait graph
	delete(dd.waitGraph, resourceID)
	
	if dd.config.Logger != nil {
		dd.config.Logger.Debug("Resource acquired",
			String("resource", resourceID),
			String("owner", ownerID))
	}
}

// ReleaseResource records that a resource has been released
func (dd *DeadlockDetector) ReleaseResource(resourceID string) {
	dd.mu.Lock()
	defer dd.mu.Unlock()
	
	resource, exists := dd.resources[resourceID]
	if !exists {
		return
	}
	
	resource.mu.Lock()
	resource.acquired = false
	resource.Owner = ""
	resource.AcquiredAt = time.Time{}
	resource.StackTrace = ""
	resource.mu.Unlock()
	
	if dd.config.Logger != nil {
		dd.config.Logger.Debug("Resource released", String("resource", resourceID))
	}
}

// WaitForResource records that a goroutine is waiting for a resource
func (dd *DeadlockDetector) WaitForResource(resourceID string, waiterID string, waitingFor []string) {
	dd.mu.Lock()
	defer dd.mu.Unlock()
	
	resource, exists := dd.resources[resourceID]
	if !exists {
		return
	}
	
	resource.mu.Lock()
	// Add waiter if not already present
	found := false
	for _, w := range resource.Waiters {
		if w == waiterID {
			found = true
			break
		}
	}
	if !found {
		resource.Waiters = append(resource.Waiters, waiterID)
	}
	resource.mu.Unlock()
	
	// Update wait graph
	dd.waitGraph[waiterID] = waitingFor
	
	if dd.config.Logger != nil {
		dd.config.Logger.Debug("Resource wait recorded",
			String("resource", resourceID),
			String("waiter", waiterID),
			Any("waiting_for", waitingFor))
	}
}

// StopWaitingForResource records that a goroutine is no longer waiting
func (dd *DeadlockDetector) StopWaitingForResource(resourceID string, waiterID string) {
	dd.mu.Lock()
	defer dd.mu.Unlock()
	
	resource, exists := dd.resources[resourceID]
	if !exists {
		return
	}
	
	resource.mu.Lock()
	// Remove waiter
	newWaiters := make([]string, 0, len(resource.Waiters))
	for _, w := range resource.Waiters {
		if w != waiterID {
			newWaiters = append(newWaiters, w)
		}
	}
	resource.Waiters = newWaiters
	resource.mu.Unlock()
	
	// Remove from wait graph
	delete(dd.waitGraph, waiterID)
	
	if dd.config.Logger != nil {
		dd.config.Logger.Debug("Resource wait stopped",
			String("resource", resourceID),
			String("waiter", waiterID))
	}
}

// detectionLoop runs the deadlock detection algorithm periodically
func (dd *DeadlockDetector) detectionLoop() {
	for {
		select {
		case <-dd.detectionTimer.C:
			if dd.detecting {
				dd.detectDeadlocks()
			}
			dd.detectionTimer.Reset(dd.config.DetectionInterval)
		}
	}
}

// Start starts deadlock detection
func (dd *DeadlockDetector) Start() {
	dd.mu.Lock()
	defer dd.mu.Unlock()
	
	dd.detecting = true
	
	if dd.config.Logger != nil {
		dd.config.Logger.Info("Deadlock detection started")
	}
}

// Stop stops deadlock detection
func (dd *DeadlockDetector) Stop() {
	dd.mu.Lock()
	defer dd.mu.Unlock()
	
	dd.detecting = false
	
	if dd.detectionTimer != nil {
		dd.detectionTimer.Stop()
	}
	
	if dd.config.Logger != nil {
		dd.config.Logger.Info("Deadlock detection stopped")
	}
}

// detectDeadlocks performs cycle detection in the wait graph
func (dd *DeadlockDetector) detectDeadlocks() {
	dd.mu.RLock()
	defer dd.mu.RUnlock()
	
	// Create a copy of the wait graph for analysis
	waitGraph := make(map[string][]string)
	for k, v := range dd.waitGraph {
		waitGraph[k] = make([]string, len(v))
		copy(waitGraph[k], v)
	}
	
	// Find cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	
	for node := range waitGraph {
		if !visited[node] {
			if cycle := dd.findCycle(node, waitGraph, visited, recStack, []string{}); len(cycle) > 0 {
				dd.handleDeadlock(cycle)
			}
		}
	}
}

// findCycle uses DFS to find cycles in the wait graph
func (dd *DeadlockDetector) findCycle(node string, graph map[string][]string, visited, recStack map[string]bool, path []string) []string {
	visited[node] = true
	recStack[node] = true
	path = append(path, node)
	
	for _, neighbor := range graph[node] {
		if !visited[neighbor] {
			if cycle := dd.findCycle(neighbor, graph, visited, recStack, path); len(cycle) > 0 {
				return cycle
			}
		} else if recStack[neighbor] {
			// Found a cycle, extract it
			cycleStart := -1
			for i, n := range path {
				if n == neighbor {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				return path[cycleStart:]
			}
		}
	}
	
	recStack[node] = false
	return nil
}

// handleDeadlock handles a detected deadlock
func (dd *DeadlockDetector) handleDeadlock(cycle []string) {
	dd.deadlockCount++
	
	// Build deadlock info
	deadlockInfo := DeadlockInfo{
		Cycle:       cycle,
		Resources:   make(map[string]*DeadlockResource),
		DetectedAt:  time.Now(),
		StackTraces: make(map[string]string),
	}
	
	// Collect resource information
	for _, resourceID := range cycle {
		if resource, exists := dd.resources[resourceID]; exists {
			resourceCopy := *resource
			deadlockInfo.Resources[resourceID] = &resourceCopy
			
			if resource.StackTrace != "" {
				deadlockInfo.StackTraces[resourceID] = resource.StackTrace
			}
		}
	}
	
	if dd.config.Logger != nil {
		dd.config.Logger.Error("Deadlock detected",
			Any("cycle", cycle),
			Int64("deadlock_count", dd.deadlockCount),
			Int("resources", len(deadlockInfo.Resources)))
	}
	
	// Call deadlock handler if set
	if dd.onDeadlock != nil {
		dd.onDeadlock(deadlockInfo)
	}
}

// SetDeadlockHandler sets the callback for when deadlocks are detected
func (dd *DeadlockDetector) SetDeadlockHandler(handler func(DeadlockInfo)) {
	dd.mu.Lock()
	defer dd.mu.Unlock()
	
	dd.onDeadlock = handler
}

// GetDeadlockCount returns the number of deadlocks detected
func (dd *DeadlockDetector) GetDeadlockCount() int64 {
	dd.mu.RLock()
	defer dd.mu.RUnlock()
	
	return dd.deadlockCount
}

// GetWaitGraph returns a copy of the current wait graph
func (dd *DeadlockDetector) GetWaitGraph() map[string][]string {
	dd.mu.RLock()
	defer dd.mu.RUnlock()
	
	graph := make(map[string][]string)
	for k, v := range dd.waitGraph {
		graph[k] = make([]string, len(v))
		copy(graph[k], v)
	}
	
	return graph
}

// GetResourceInfo returns information about a specific resource
func (dd *DeadlockDetector) GetResourceInfo(resourceID string) (*DeadlockResource, bool) {
	dd.mu.RLock()
	defer dd.mu.RUnlock()
	
	resource, exists := dd.resources[resourceID]
	if !exists {
		return nil, false
	}
	
	// Return a copy
	resourceCopy := *resource
	return &resourceCopy, true
}

// GetAllResources returns information about all tracked resources
func (dd *DeadlockDetector) GetAllResources() map[string]*DeadlockResource {
	dd.mu.RLock()
	defer dd.mu.RUnlock()
	
	resources := make(map[string]*DeadlockResource)
	for k, v := range dd.resources {
		resourceCopy := *v
		resources[k] = &resourceCopy
	}
	
	return resources
}

// GenerateReport generates a detailed report of the current state
func (dd *DeadlockDetector) GenerateReport() string {
	dd.mu.RLock()
	defer dd.mu.RUnlock()
	
	var report strings.Builder
	
	report.WriteString("Deadlock Detection Report\n")
	report.WriteString("========================\n\n")
	
	report.WriteString(fmt.Sprintf("Detection Status: %v\n", dd.detecting))
	report.WriteString(fmt.Sprintf("Deadlocks Detected: %d\n", dd.deadlockCount))
	report.WriteString(fmt.Sprintf("Resources Tracked: %d\n", len(dd.resources)))
	report.WriteString(fmt.Sprintf("Wait Graph Entries: %d\n\n", len(dd.waitGraph)))
	
	// Resource information
	if len(dd.resources) > 0 {
		report.WriteString("Resources:\n")
		report.WriteString("----------\n")
		
		// Sort resources by ID for consistent output
		var resourceIDs []string
		for id := range dd.resources {
			resourceIDs = append(resourceIDs, id)
		}
		sort.Strings(resourceIDs)
		
		for _, id := range resourceIDs {
			resource := dd.resources[id]
			resource.mu.RLock()
			
			report.WriteString(fmt.Sprintf("  %s (%s):\n", id, resource.Type))
			report.WriteString(fmt.Sprintf("    Acquired: %v\n", resource.acquired))
			if resource.Owner != "" {
				report.WriteString(fmt.Sprintf("    Owner: %s\n", resource.Owner))
				report.WriteString(fmt.Sprintf("    Acquired At: %v\n", resource.AcquiredAt))
			}
			if len(resource.Waiters) > 0 {
				report.WriteString(fmt.Sprintf("    Waiters: %v\n", resource.Waiters))
			}
			
			resource.mu.RUnlock()
			report.WriteString("\n")
		}
	}
	
	// Wait graph
	if len(dd.waitGraph) > 0 {
		report.WriteString("Wait Graph:\n")
		report.WriteString("-----------\n")
		
		// Sort wait graph entries
		var waiters []string
		for waiter := range dd.waitGraph {
			waiters = append(waiters, waiter)
		}
		sort.Strings(waiters)
		
		for _, waiter := range waiters {
			waitingFor := dd.waitGraph[waiter]
			if len(waitingFor) > 0 {
				report.WriteString(fmt.Sprintf("  %s -> %v\n", waiter, waitingFor))
			}
		}
	}
	
	return report.String()
}

// String returns a string representation of the deadlock info
func (di DeadlockInfo) String() string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("Deadlock detected at %v\n", di.DetectedAt))
	sb.WriteString(fmt.Sprintf("Cycle: %v\n", di.Cycle))
	sb.WriteString(fmt.Sprintf("Resources involved: %d\n", len(di.Resources)))
	
	if di.Resolution != "" {
		sb.WriteString(fmt.Sprintf("Resolution: %s\n", di.Resolution))
	}
	
	return sb.String()
}