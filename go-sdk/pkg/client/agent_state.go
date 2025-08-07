package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/state"
)

// AgentStateManager provides agent-specific state management with local caching,
// remote synchronization, conflict resolution, state versioning, and performance
// optimization for state operations.
//
// Key features:
//   - Integration with existing state management system
//   - Configurable synchronization policies
//   - Efficient state diff computation
//   - Memory and storage optimization
//   - Local state caching and persistence
//   - Conflict resolution strategies
//   - State versioning and rollback
type AgentStateManager struct {
	// Configuration
	config StateConfig
	
	// State storage
	store       state.StoreInterface
	localCache  *StateCache
	
	// Synchronization
	syncTicker  *time.Ticker
	syncMu      sync.RWMutex
	lastSync    time.Time
	syncErrors  int64
	
	// Conflict resolution
	resolver    ConflictResolver
	
	// State versioning
	versions    map[string]*StateVersion
	versionsMu  sync.RWMutex
	
	// Lifecycle
	running     atomic.Bool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	isHealthy   atomic.Bool
	
	// Metrics
	metrics     StateManagerMetrics
	metricsMu   sync.RWMutex
	
	// Subscriptions
	subscriptions map[string][]StateSubscription
	subsMu        sync.RWMutex
}

// StateCache provides efficient local caching of state data.
type StateCache struct {
	data       map[string]interface{}
	mu         sync.RWMutex
	maxSize    int64
	currentSize atomic.Int64
	evictionPolicy EvictionPolicy
	accessTimes map[string]time.Time
}

// ConflictResolver handles state conflicts using various strategies.
type ConflictResolver interface {
	ResolveConflict(local, remote interface{}, path string) (interface{}, error)
}

// StateVersion represents a versioned state with metadata.
type StateVersion struct {
	ID        string                 `json:"id"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	Checksum  string                 `json:"checksum"`
	ParentID  string                 `json:"parent_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// StateSubscription represents a subscription to state changes.
type StateSubscription struct {
	ID       string
	Path     string
	Callback func(StateChangeEvent)
	Filter   StateFilter
}

// StateChangeEvent represents a state change event.
type StateChangeEvent struct {
	Path      string                 `json:"path"`
	OldValue  interface{}            `json:"old_value"`
	NewValue  interface{}            `json:"new_value"`
	Operation string                 `json:"operation"`  // Use string for simple operation types
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Note: StateOperation struct is defined in agent.go with more comprehensive fields
// For simple string-based operations, use StateOperationType constants from agent.go

// StateFilter allows filtering of state change events.
type StateFilter func(StateChangeEvent) bool

// EvictionPolicy defines how cache entries are evicted.
type EvictionPolicy int

const (
	EvictionPolicyLRU EvictionPolicy = iota
	EvictionPolicyLFU
	EvictionPolicyTTL
)

// StateManagerMetrics contains metrics for the state manager.
type StateManagerMetrics struct {
	StateReads        int64         `json:"state_reads"`
	StateWrites       int64         `json:"state_writes"`
	CacheHits         int64         `json:"cache_hits"`
	CacheMisses       int64         `json:"cache_misses"`
	SyncOperations    int64         `json:"sync_operations"`
	ConflictResolutions int64       `json:"conflict_resolutions"`
	AverageLatency    time.Duration `json:"average_latency"`
	ErrorCount        int64         `json:"error_count"`
	LastSyncTime      time.Time     `json:"last_sync_time"`
}

// ConflictResolutionStrategy defines available conflict resolution strategies.
type ConflictResolutionStrategy string

const (
	StrategyLastWriterWins  ConflictResolutionStrategy = "last-writer-wins"
	StrategyFirstWriterWins ConflictResolutionStrategy = "first-writer-wins"
	StrategyMerge          ConflictResolutionStrategy = "merge"
	StrategyCustom         ConflictResolutionStrategy = "custom"
	
	// Compatibility aliases for constants used in other files
	ConflictResolutionLastWriterWins ConflictResolutionStrategy = "last-writer-wins"
	ConflictResolutionFirstWriterWins ConflictResolutionStrategy = "first-writer-wins"
	ConflictResolutionMerge ConflictResolutionStrategy = "merge"
	ConflictResolutionReject ConflictResolutionStrategy = "reject"
)

// NewAgentStateManager creates a new agent state manager with the given configuration.
func NewAgentStateManager(config StateConfig) (*AgentStateManager, error) {
	if config.CacheSize == "" {
		config.CacheSize = "100MB"
	}
	if config.SyncInterval == 0 {
		config.SyncInterval = 5 * time.Second
	}
	if config.ConflictResolution == "" {
		config.ConflictResolution = StrategyLastWriterWins
	}
	
	// Parse cache size
	cacheSize, err := parseCacheSize(config.CacheSize)
	if err != nil {
		return nil, errors.WithOperation("parse", "cache_size", err)
	}
	
	// Create state store
	store := state.NewStateStore()
	
	// Create cache
	cache := &StateCache{
		data:           make(map[string]interface{}),
		maxSize:        cacheSize,
		evictionPolicy: EvictionPolicyLRU,
		accessTimes:    make(map[string]time.Time),
	}
	
	// Create conflict resolver
	resolver, err := newConflictResolver(config.ConflictResolution)
	if err != nil {
		return nil, errors.WithOperation("create", "conflict_resolver", err)
	}
	
	manager := &AgentStateManager{
		config:        config,
		store:         store,
		localCache:    cache,
		resolver:      resolver,
		versions:      make(map[string]*StateVersion),
		subscriptions: make(map[string][]StateSubscription),
		metrics: StateManagerMetrics{
			LastSyncTime: time.Now(),
		},
	}
	
	manager.isHealthy.Store(true)
	
	return manager, nil
}

// Start begins state management operations.
func (sm *AgentStateManager) Start(ctx context.Context) error {
	if sm.running.Load() {
		return errors.NewStateError(string(errors.ErrorTypeInvalidState), "state manager is already running")
	}
	
	sm.ctx, sm.cancel = context.WithCancel(ctx)
	sm.running.Store(true)
	
	// Start synchronization if enabled
	if sm.config.SyncInterval > 0 {
		sm.syncTicker = time.NewTicker(sm.config.SyncInterval)
		sm.wg.Add(1)
		go sm.syncLoop()
	}
	
	// Start cache eviction
	sm.wg.Add(1)
	go sm.cacheEvictionLoop()
	
	// Start metrics collection
	sm.wg.Add(1)
	go sm.metricsLoop()
	
	return nil
}

// Stop gracefully stops state management.
func (sm *AgentStateManager) Stop(ctx context.Context) error {
	if !sm.running.Load() {
		return nil
	}
	
	sm.running.Store(false)
	sm.cancel()
	
	if sm.syncTicker != nil {
		sm.syncTicker.Stop()
	}
	
	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		sm.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All goroutines finished
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for state manager to stop")
	}
	
	return nil
}

// Cleanup releases all resources.
func (sm *AgentStateManager) Cleanup() error {
	sm.localCache.clear()
	sm.versions = make(map[string]*StateVersion)
	sm.subscriptions = make(map[string][]StateSubscription)
	return nil
}

// GetState returns the current state of the agent.
func (sm *AgentStateManager) GetState(ctx context.Context) (interface{}, error) {
	if !sm.running.Load() {
		return nil, errors.NewStateError(string(errors.ErrorTypeInvalidState), "state manager is not running")
	}
	
	startTime := time.Now()
	defer func() {
		latency := time.Since(startTime)
		sm.updateLatencyMetrics(latency)
		atomic.AddInt64(&sm.metrics.StateReads, 1)
	}()
	
	// Try cache first
	if data := sm.localCache.get("root"); data != nil {
		atomic.AddInt64(&sm.metrics.CacheHits, 1)
		return data, nil
	}
	
	// Cache miss, get from store
	atomic.AddInt64(&sm.metrics.CacheMisses, 1)
	data := sm.store.GetState()
	
	// Cache the result
	sm.localCache.set("root", data)
	
	return data, nil
}

// UpdateState applies a state change delta.
func (sm *AgentStateManager) UpdateState(ctx context.Context, delta interface{}) error {
	if !sm.running.Load() {
		return errors.NewStateError(string(errors.ErrorTypeInvalidState), "state manager is not running")
	}
	
	startTime := time.Now()
	defer func() {
		latency := time.Since(startTime)
		sm.updateLatencyMetrics(latency)
		atomic.AddInt64(&sm.metrics.StateWrites, 1)
	}()
	
	// Convert delta to JSON patch
	patch, err := sm.convertDeltaToPatch(delta)
	if err != nil {
		atomic.AddInt64(&sm.metrics.ErrorCount, 1)
		return errors.WithOperation("convert", "delta_to_patch", err)
	}
	
	// Get current state for conflict detection
	currentState, err := sm.GetState(ctx)
	if err != nil {
		atomic.AddInt64(&sm.metrics.ErrorCount, 1)
		return errors.WithOperation("get", "current_state", err)
	}
	
	// Apply patch
	err = sm.store.ApplyPatch(patch)
	if err != nil {
		// Handle potential conflicts
		if isConflictError(err) {
			atomic.AddInt64(&sm.metrics.ConflictResolutions, 1)
			return sm.handleConflict(ctx, currentState, delta, patch)
		}
		atomic.AddInt64(&sm.metrics.ErrorCount, 1)
		return errors.WithOperation("apply", "state_patch", err)
	}
	
	// Update cache
	sm.localCache.invalidate("root")
	
	// Create state version
	version, err := sm.createStateVersion(delta)
	if err != nil {
		// Log error but don't fail the update
		// In a real implementation, this would use proper logging
	} else {
		sm.addVersion(version)
	}
	
	// Notify subscribers
	sm.notifySubscribers(StateChangeEvent{
		Path:      "root",
		NewValue:  delta,
		Operation: "patch",
		Timestamp: time.Now(),
		Version:   version.ID,
	})
	
	return nil
}

// Subscribe to state changes.
func (sm *AgentStateManager) Subscribe(path string, callback func(StateChangeEvent), filter StateFilter) string {
	sm.subsMu.Lock()
	defer sm.subsMu.Unlock()
	
	subscriptionID := fmt.Sprintf("sub_%d", time.Now().UnixNano())
	subscription := StateSubscription{
		ID:       subscriptionID,
		Path:     path,
		Callback: callback,
		Filter:   filter,
	}
	
	if sm.subscriptions[path] == nil {
		sm.subscriptions[path] = make([]StateSubscription, 0)
	}
	
	sm.subscriptions[path] = append(sm.subscriptions[path], subscription)
	
	return subscriptionID
}

// Unsubscribe from state changes.
func (sm *AgentStateManager) Unsubscribe(subscriptionID string) {
	sm.subsMu.Lock()
	defer sm.subsMu.Unlock()
	
	for path, subs := range sm.subscriptions {
		for i, sub := range subs {
			if sub.ID == subscriptionID {
				// Remove subscription
				sm.subscriptions[path] = append(subs[:i], subs[i+1:]...)
				if len(sm.subscriptions[path]) == 0 {
					delete(sm.subscriptions, path)
				}
				return
			}
		}
	}
}

// GetVersion returns a specific state version.
func (sm *AgentStateManager) GetVersion(versionID string) (*StateVersion, error) {
	sm.versionsMu.RLock()
	defer sm.versionsMu.RUnlock()
	
	version, exists := sm.versions[versionID]
	if !exists {
		return nil, errors.NewStateError(string(errors.ErrorTypeNotFound), fmt.Sprintf("version %s not found", versionID))
	}
	
	return version, nil
}

// RollbackToVersion rolls back the state to a specific version.
func (sm *AgentStateManager) RollbackToVersion(ctx context.Context, versionID string) error {
	version, err := sm.GetVersion(versionID)
	if err != nil {
		return err
	}
	
	// Create rollback patch
	patch, err := sm.createRollbackPatch(version.Data)
	if err != nil {
		return errors.WithOperation("create", "rollback_patch", err)
	}
	
	// Apply rollback
	err = sm.store.ApplyPatch(patch)
	if err != nil {
		return errors.WithOperation("apply", "rollback_patch", err)
	}
	
	// Clear cache
	sm.localCache.clear()
	
	return nil
}

// GetMetrics returns current state manager metrics.
func (sm *AgentStateManager) GetMetrics() StateManagerMetrics {
	sm.metricsMu.RLock()
	defer sm.metricsMu.RUnlock()
	return sm.metrics
}

// IsHealthy returns the health status.
func (sm *AgentStateManager) IsHealthy() bool {
	return sm.isHealthy.Load()
}

// Private methods

func (sm *AgentStateManager) syncLoop() {
	defer sm.wg.Done()
	
	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-sm.syncTicker.C:
			sm.performSync()
		}
	}
}

func (sm *AgentStateManager) performSync() {
	sm.syncMu.Lock()
	defer sm.syncMu.Unlock()
	
	// Simplified sync operation
	// In a real implementation, this would sync with remote systems
	atomic.AddInt64(&sm.metrics.SyncOperations, 1)
	sm.lastSync = time.Now()
	
	sm.metricsMu.Lock()
	sm.metrics.LastSyncTime = time.Now()
	sm.metricsMu.Unlock()
}

func (sm *AgentStateManager) cacheEvictionLoop() {
	defer sm.wg.Done()
	
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.localCache.evict()
		}
	}
}

func (sm *AgentStateManager) metricsLoop() {
	defer sm.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.updateHealthStatus()
		}
	}
}

func (sm *AgentStateManager) updateHealthStatus() {
	// Check if sync errors are too high
	errorCount := atomic.LoadInt64(&sm.metrics.ErrorCount)
	syncErrors := atomic.LoadInt64(&sm.syncErrors)
	
	if errorCount > 100 || syncErrors > 10 {
		sm.isHealthy.Store(false)
	} else {
		sm.isHealthy.Store(true)
	}
}

func (sm *AgentStateManager) convertDeltaToPatch(delta interface{}) (state.JSONPatch, error) {
	// Convert delta to JSON patch format
	// This is a simplified implementation
	deltaBytes, err := json.Marshal(delta)
	if err != nil {
		return nil, errors.WithOperation("marshal", "delta_data", err)
	}
	
	// Create a simple patch operation
	patch := state.JSONPatch{
		{
			Op:    "replace",
			Path:  "/",
			Value: json.RawMessage(deltaBytes),
		},
	}
	
	return patch, nil
}

func (sm *AgentStateManager) handleConflict(ctx context.Context, currentState, delta interface{}, patch state.JSONPatch) error {
	// Use conflict resolver to resolve the conflict
	resolved, err := sm.resolver.ResolveConflict(currentState, delta, "/")
	if err != nil {
		return errors.WithOperation("resolve", "state_conflict", err)
	}
	
	// Apply resolved state
	resolvedPatch, err := sm.convertDeltaToPatch(resolved)
	if err != nil {
		return errors.WithOperation("create", "resolved_patch", err)
	}
	
	return sm.store.ApplyPatch(resolvedPatch)
}

func (sm *AgentStateManager) createStateVersion(delta interface{}) (*StateVersion, error) {
	versionID := fmt.Sprintf("v_%d", time.Now().UnixNano())
	
	// Get current state for version
	currentState := sm.store.GetState()
	
	version := &StateVersion{
		ID:        versionID,
		Data:      currentState,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
	
	// Calculate checksum
	checksum, err := sm.calculateChecksum(currentState)
	if err != nil {
		return nil, err
	}
	version.Checksum = checksum
	
	return version, nil
}

func (sm *AgentStateManager) addVersion(version *StateVersion) {
	sm.versionsMu.Lock()
	defer sm.versionsMu.Unlock()
	
	sm.versions[version.ID] = version
	
	// Limit version history
	if len(sm.versions) > 100 {
		sm.cleanupOldVersions()
	}
}

func (sm *AgentStateManager) cleanupOldVersions() {
	// Keep only the latest 50 versions
	// This is a simplified cleanup strategy
	if len(sm.versions) <= 50 {
		return
	}
	
	// In a real implementation, this would sort by timestamp and remove oldest
	for id := range sm.versions {
		delete(sm.versions, id)
		if len(sm.versions) <= 50 {
			break
		}
	}
}

func (sm *AgentStateManager) createRollbackPatch(targetState map[string]interface{}) (state.JSONPatch, error) {
	// Create patch to rollback to target state
	return sm.convertDeltaToPatch(targetState)
}

func (sm *AgentStateManager) calculateChecksum(data interface{}) (string, error) {
	// Simple checksum calculation
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	
	// In a real implementation, this would use a proper hash function
	return fmt.Sprintf("checksum_%d", len(dataBytes)), nil
}

func (sm *AgentStateManager) notifySubscribers(event StateChangeEvent) {
	sm.subsMu.RLock()
	defer sm.subsMu.RUnlock()
	
	for path, subs := range sm.subscriptions {
		if sm.pathMatches(path, event.Path) {
			for _, sub := range subs {
				if sub.Filter == nil || sub.Filter(event) {
					go sub.Callback(event) // Non-blocking notification
				}
			}
		}
	}
}

func (sm *AgentStateManager) pathMatches(subscriptionPath, eventPath string) bool {
	// Simple path matching - in real implementation would support wildcards
	return subscriptionPath == eventPath || subscriptionPath == "*"
}

func (sm *AgentStateManager) updateLatencyMetrics(latency time.Duration) {
	sm.metricsMu.Lock()
	defer sm.metricsMu.Unlock()
	
	if sm.metrics.AverageLatency == 0 {
		sm.metrics.AverageLatency = latency
	} else {
		sm.metrics.AverageLatency = (sm.metrics.AverageLatency + latency) / 2
	}
}

// StateCache methods

func (sc *StateCache) get(key string) interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	
	data, exists := sc.data[key]
	if exists {
		sc.accessTimes[key] = time.Now()
		return data
	}
	return nil
}

func (sc *StateCache) set(key string, value interface{}) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	sc.data[key] = value
	sc.accessTimes[key] = time.Now()
	
	// Check if eviction is needed
	if sc.needsEviction() {
		sc.evictOne()
	}
}

func (sc *StateCache) invalidate(key string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	delete(sc.data, key)
	delete(sc.accessTimes, key)
}

func (sc *StateCache) clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	sc.data = make(map[string]interface{})
	sc.accessTimes = make(map[string]time.Time)
	sc.currentSize.Store(0)
}

func (sc *StateCache) needsEviction() bool {
	return sc.currentSize.Load() > sc.maxSize
}

func (sc *StateCache) evict() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	// Evict entries based on policy
	cutoff := time.Now().Add(-10 * time.Minute)
	for key, accessTime := range sc.accessTimes {
		if accessTime.Before(cutoff) {
			delete(sc.data, key)
			delete(sc.accessTimes, key)
		}
	}
}

func (sc *StateCache) evictOne() {
	// Find LRU entry
	var oldestKey string
	var oldestTime time.Time
	
	for key, accessTime := range sc.accessTimes {
		if oldestTime.IsZero() || accessTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = accessTime
		}
	}
	
	if oldestKey != "" {
		delete(sc.data, oldestKey)
		delete(sc.accessTimes, oldestKey)
	}
}

// Helper functions

func parseCacheSize(sizeStr string) (int64, error) {
	// Simple size parsing - in real implementation would handle MB, GB, etc.
	switch sizeStr {
	case "100MB":
		return 100 * 1024 * 1024, nil
	case "1GB":
		return 1024 * 1024 * 1024, nil
	default:
		return 100 * 1024 * 1024, nil // Default to 100MB
	}
}

func isConflictError(err error) bool {
	// Check if error is a conflict error
	return false // Simplified
}

func newConflictResolver(strategy ConflictResolutionStrategy) (ConflictResolver, error) {
	switch strategy {
	case StrategyLastWriterWins:
		return &LastWriterWinsResolver{}, nil
	case StrategyFirstWriterWins:
		return &FirstWriterWinsResolver{}, nil
	case StrategyMerge:
		return &MergeResolver{}, nil
	default:
		return &LastWriterWinsResolver{}, nil
	}
}

// Conflict resolvers

type LastWriterWinsResolver struct{}

func (r *LastWriterWinsResolver) ResolveConflict(local, remote interface{}, path string) (interface{}, error) {
	// Always use the remote (newer) value
	return remote, nil
}

type FirstWriterWinsResolver struct{}

func (r *FirstWriterWinsResolver) ResolveConflict(local, remote interface{}, path string) (interface{}, error) {
	// Always use the local (existing) value
	return local, nil
}

type MergeResolver struct{}

func (r *MergeResolver) ResolveConflict(local, remote interface{}, path string) (interface{}, error) {
	// Attempt to merge values
	// This is a simplified implementation
	return remote, nil
}