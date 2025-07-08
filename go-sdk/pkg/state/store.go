package state

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// StateChange represents a change to the state
type StateChange struct {
	Path      string
	OldValue  interface{}
	NewValue  interface{}
	Operation string
	Timestamp time.Time
}

// ImmutableState represents an immutable version of the state with reference counting
type ImmutableState struct {
	version int64
	data    map[string]interface{}
	refs    int32
}

// StateView provides safe read-only access to state data
type StateView struct {
	data    map[string]interface{}
	cleanup func()
}

// StateVersion represents a version of the state with history tracking
type StateVersion struct {
	ID        string                 // Unique identifier
	Timestamp time.Time              // When this version was created
	State     map[string]interface{} // Complete state at this version
	Delta     JSONPatch              // Changes from previous version
	Metadata  map[string]interface{} // Additional metadata
	ParentID  string                 // Parent version ID
}

// StateSnapshot represents a point-in-time snapshot of the state
type StateSnapshot struct {
	ID        string                 // Unique identifier
	Timestamp time.Time              // When the snapshot was created
	State     map[string]interface{} // Complete state
	Version   string                 // Version ID this snapshot represents
	Metadata  map[string]interface{} // Additional metadata
}

// StateTransaction represents an atomic transaction
type StateTransaction struct {
	store     *StateStore
	patches   JSONPatch
	snapshot  map[string]interface{}
	committed bool
	mu        sync.Mutex
}

// SubscriptionCallback is the function signature for state change subscriptions
type SubscriptionCallback func(StateChange)

// subscription represents an active subscription
type subscription struct {
	id           string
	path         string
	callback     SubscriptionCallback
	lastAccessed time.Time // Track access time for cleanup
	created      time.Time // Track creation time
}

// StateStore provides versioned state management with history and transactions
type StateStore struct {
	// Sharded state implementation for fine-grained locking
	shards     []*stateShard // 16 shards for better concurrency
	shardCount uint32        // Number of shards (power of 2)

	// Global operations locks
	transactionsMu sync.RWMutex // Lock for transaction management
	historyMu      sync.Mutex   // Lock for history operations

	version       int64
	history       []*StateVersion
	maxHistory    int
	subscriptions sync.Map // Using sync.Map to reduce lock contention
	transactions  map[string]*StateTransaction

	// Subscription management
	subscriptionTTL time.Duration
	lastCleanup     time.Time
	cleanupInterval time.Duration

	// Error handling
	errorHandler func(error)
	logger       Logger
}

// stateShard represents a single shard with its own lock
type stateShard struct {
	mu      sync.RWMutex
	current atomic.Value // *ImmutableState - atomic for lock-free reads
}

// NewStateStore creates a new state store instance
func NewStateStore(options ...StateStoreOption) *StateStore {
	store := &StateStore{
		shardCount:      DefaultShardCount, // Default to 16 shards for better concurrency
		version:         0,
		history:         make([]*StateVersion, 0),
		maxHistory:      DefaultMaxHistorySizeSharding, // Default max history for sharded operations
		transactions:    make(map[string]*StateTransaction),
		subscriptionTTL: DefaultSubscriptionTTL,     // Default TTL for subscriptions
		cleanupInterval: DefaultSubscriptionCleanup, // Default cleanup interval
		lastCleanup:     time.Now(),
		logger:          DefaultLogger(),
		errorHandler:    nil, // Will be set after initialization
	}

	// Apply options
	for _, opt := range options {
		opt(store)
	}

	// Set default error handler after store is initialized
	if store.errorHandler == nil {
		store.errorHandler = func(err error) {
			if store.logger != nil {
				store.logger.Error("state store error", Err(err))
			}
		}
	}

	// Initialize shards
	store.shards = make([]*stateShard, store.shardCount)
	for i := uint32(0); i < store.shardCount; i++ {
		shard := &stateShard{}
		// Initialize each shard with empty immutable state
		initialState := &ImmutableState{
			version: 0,
			data:    make(map[string]interface{}),
			refs:    0,
		}
		shard.current.Store(initialState)
		store.shards[i] = shard
	}

	// Create initial version
	store.createVersion(nil, nil)

	return store
}

// SetErrorHandler sets the error handler for the store
func (s *StateStore) SetErrorHandler(handler func(error)) {
	if handler != nil {
		s.errorHandler = handler
	}
}

// SetLogger sets the logger for the store
func (s *StateStore) SetLogger(logger Logger) {
	if logger != nil {
		s.logger = logger
	}
}

// StateStoreOption is a configuration option for StateStore
type StateStoreOption func(*StateStore)

// WithMaxHistory sets the maximum number of history entries to keep
func WithMaxHistory(max int) StateStoreOption {
	return func(s *StateStore) {
		s.maxHistory = max
	}
}

// WithShardCount sets the number of shards (must be power of 2)
func WithShardCount(count uint32) StateStoreOption {
	return func(s *StateStore) {
		// Ensure it's a power of 2
		if count > 0 && (count&(count-1)) == 0 {
			s.shardCount = count
		}
	}
}

// WithLogger sets the logger for the store
func WithLogger(logger Logger) StateStoreOption {
	return func(s *StateStore) {
		s.logger = logger
	}
}

// getShardIndex returns the shard index for a given path using FNV hash
func (s *StateStore) getShardIndex(path string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(path))
	// Use bitwise AND with (shardCount - 1) for fast modulo when shardCount is power of 2
	return h.Sum32() & (s.shardCount - 1)
}

// getShardForPath returns the shard responsible for the given path
func (s *StateStore) getShardForPath(path string) *stateShard {
	index := s.getShardIndex(path)
	return s.shards[index]
}

// Get retrieves a value at the specified path
func (s *StateStore) Get(path string) (interface{}, error) {
	if path == "" || path == "/" {
		// For root path, we need to merge data from all shards
		return s.getAllShardsData(), nil
	}

	// For nested paths, we need to find the correct shard based on the top-level key
	// Parse the path to get the top-level key
	tokens := parseJSONPointer(path)
	if len(tokens) == 0 {
		// Path doesn't start with '/', might be a stateID - fallback to original behavior
		shard := s.getShardForPath(path)
		state := shard.current.Load().(*ImmutableState)
		atomic.AddInt32(&state.refs, 1)
		defer atomic.AddInt32(&state.refs, -1)

		value, err := getValueAtPath(state.data, path)
		if err != nil {
			return nil, fmt.Errorf("failed to get value at path %s: %w", path, err)
		}

		return deepCopy(value), nil
	}

	// Use the top-level key to determine the shard
	topLevelPath := "/" + tokens[0]
	shard := s.getShardForPath(topLevelPath)

	// Lock-free read using atomic value from the specific shard
	state := shard.current.Load().(*ImmutableState)
	atomic.AddInt32(&state.refs, 1)
	defer atomic.AddInt32(&state.refs, -1)

	value, err := getValueAtPath(state.data, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get value at path %s: %w", path, err)
	}

	return deepCopy(value), nil
}

// getAllShardsData merges data from all shards for root path access
func (s *StateStore) getAllShardsData() map[string]interface{} {
	merged := make(map[string]interface{})

	// Collect data from all shards
	for _, shard := range s.shards {
		state := shard.current.Load().(*ImmutableState)
		atomic.AddInt32(&state.refs, 1)

		// Merge shard data into result
		for k, v := range state.data {
			merged[k] = deepCopy(v)
		}

		atomic.AddInt32(&state.refs, -1)
	}

	return merged
}

// Set updates a value at the specified path
func (s *StateStore) Set(path string, value interface{}) error {
	// Handle root path differently as it affects all shards
	if path == "" || path == "/" {
		return s.setRootPath(value)
	}

	// For nested paths, we need to find the correct shard based on the top-level key
	// Parse the path to get the top-level key
	tokens := parseJSONPointer(path)
	if len(tokens) == 0 {
		// Path doesn't start with '/', might be a stateID - fallback to original behavior
		shard := s.getShardForPath(path)
		shard.mu.Lock()
		defer shard.mu.Unlock()

		currentState := shard.current.Load().(*ImmutableState)

		// Create a patch for this operation
		patch := JSONPatch{
			JSONPatchOperation{
				Op:    JSONPatchOpReplace,
				Path:  path,
				Value: value,
			},
		}

		// Check if path exists, if not use add operation
		if _, err := getValueAtPath(currentState.data, path); err != nil {
			patch[0].Op = JSONPatchOpAdd
		}

		return s.applyPatchToShard(shard, patch)
	}

	// Use the top-level key to determine the shard
	topLevelPath := "/" + tokens[0]
	shard := s.getShardForPath(topLevelPath)

	// Lock only the specific shard
	shard.mu.Lock()
	defer shard.mu.Unlock()

	currentState := shard.current.Load().(*ImmutableState)

	// Create a patch for this operation
	patch := JSONPatch{
		JSONPatchOperation{
			Op:    JSONPatchOpReplace,
			Path:  path,
			Value: value,
		},
	}

	// Check if path exists, if not use add operation
	if _, err := getValueAtPath(currentState.data, path); err != nil {
		patch[0].Op = JSONPatchOpAdd
	}

	return s.applyPatchToShard(shard, patch)
}

// setRootPath handles setting the root path which affects all shards
func (s *StateStore) setRootPath(value interface{}) error {
	// Need to lock all shards for root path update
	for _, shard := range s.shards {
		shard.mu.Lock()
		defer shard.mu.Unlock()
	}

	// Clear all shards and redistribute data
	if m, ok := value.(map[string]interface{}); ok {
		// First, clear all shards
		for _, shard := range s.shards {
			newState := &ImmutableState{
				version: atomic.LoadInt64(&s.version) + 1,
				data:    make(map[string]interface{}),
				refs:    0,
			}
			shard.current.Store(newState)
		}

		// Create a map to hold new data for each shard
		shardData := make([]map[string]interface{}, s.shardCount)
		for i := range shardData {
			shardData[i] = make(map[string]interface{})
		}

		// Distribute the data
		for k, v := range m {
			path := "/" + k
			shardIdx := s.getShardIndex(path)
			shardData[shardIdx][k] = v
		}

		// Update each shard with its new data
		newVersion := atomic.AddInt64(&s.version, 1)
		for i, shard := range s.shards {
			newState := &ImmutableState{
				version: newVersion,
				data:    shardData[i],
				refs:    0,
			}
			shard.current.Store(newState)
		}
	}

	patch := JSONPatch{JSONPatchOperation{Op: JSONPatchOpReplace, Path: "/", Value: value}}
	// Use pre-computed state to avoid deadlock
	if m, ok := value.(map[string]interface{}); ok {
		s.createVersionWithState(patch, deepCopy(m).(map[string]interface{}), nil)
	} else {
		s.createVersionWithState(patch, map[string]interface{}{"": value}, nil)
	}

	// Notify subscribers
	changes := []StateChange{{
		Path:      "/",
		Operation: string(JSONPatchOpReplace),
		NewValue:  value,
		Timestamp: time.Now(),
	}}
	s.notifySubscribers(changes)

	return nil
}

// ensureParentPaths ensures all parent paths exist as objects
// Note: This is now used only for validation, actual path creation happens in COW
func (s *StateStore) ensureParentPaths(path string, data map[string]interface{}) error {
	if path == "" || path == "/" {
		return nil
	}

	tokens := parseJSONPointer(path)
	var current interface{} = data

	for i := 0; i < len(tokens)-1; i++ {
		token := tokens[i]

		switch c := current.(type) {
		case map[string]interface{}:
			if val, exists := c[token]; exists {
				current = val
			} else {
				// Path doesn't exist yet, which is okay for COW
				return nil
			}
		default:
			// Current path exists but is not an object
			pathSoFar := "/" + strings.Join(tokens[:i+1], "/")
			return fmt.Errorf("cannot create path %s: parent is not an object", pathSoFar)
		}
	}

	return nil
}

// ensureParentPathsCOW ensures all parent paths exist as objects in the COW data
func (s *StateStore) ensureParentPathsCOW(path string, data map[string]interface{}) error {
	if path == "" || path == "/" {
		return nil
	}

	tokens := parseJSONPointer(path)
	var current interface{} = data

	for i := 0; i < len(tokens)-1; i++ {
		token := tokens[i]

		switch c := current.(type) {
		case map[string]interface{}:
			if val, exists := c[token]; exists {
				current = val
			} else {
				// Create the path
				c[token] = make(map[string]interface{})
				current = c[token]
			}
		default:
			// Current path exists but is not an object
			pathSoFar := "/" + strings.Join(tokens[:i+1], "/")
			return fmt.Errorf("cannot create path %s: parent is not an object", pathSoFar)
		}
	}

	return nil
}

// Delete removes a value at the specified path
func (s *StateStore) Delete(path string) error {
	if path == "" || path == "/" {
		return fmt.Errorf("cannot delete root")
	}

	// For nested paths, we need to find the correct shard based on the top-level key
	// Parse the path to get the top-level key
	tokens := parseJSONPointer(path)
	var shard *stateShard

	if len(tokens) == 0 {
		// Path doesn't start with '/', might be a stateID - fallback to original behavior
		shard = s.getShardForPath(path)
	} else {
		// Use the top-level key to determine the shard
		topLevelPath := "/" + tokens[0]
		shard = s.getShardForPath(topLevelPath)
	}

	// Lock only the specific shard
	shard.mu.Lock()
	defer shard.mu.Unlock()

	patch := JSONPatch{
		JSONPatchOperation{
			Op:   JSONPatchOpRemove,
			Path: path,
		},
	}

	return s.applyPatchToShard(shard, patch)
}

// ApplyPatch applies a JSON Patch to the state
func (s *StateStore) ApplyPatch(patch JSONPatch) error {
	// Group patches by shard to minimize lock contention
	patchesByShard := make(map[uint32]JSONPatch)
	globalPatches := JSONPatch{}

	for _, op := range patch {
		if op.Path == "" || op.Path == "/" {
			globalPatches = append(globalPatches, op)
		} else {
			// For nested paths, we need to find the correct shard based on the top-level key
			tokens := parseJSONPointer(op.Path)
			var shardIdx uint32

			if len(tokens) == 0 {
				// Path doesn't start with '/', might be a stateID - fallback to original behavior
				shardIdx = s.getShardIndex(op.Path)
			} else {
				// Use the top-level key to determine the shard
				topLevelPath := "/" + tokens[0]
				shardIdx = s.getShardIndex(topLevelPath)
			}

			patchesByShard[shardIdx] = append(patchesByShard[shardIdx], op)
		}
	}

	// Apply global patches first (requires all shards locked)
	if len(globalPatches) > 0 {
		for _, shard := range s.shards {
			shard.mu.Lock()
			defer shard.mu.Unlock()
		}

		for _, op := range globalPatches {
			if err := s.applyGlobalPatch(op); err != nil {
				return err
			}
		}
	}

	// Apply shard-specific patches
	for shardIdx, shardPatches := range patchesByShard {
		shard := s.shards[shardIdx]
		shard.mu.Lock()
		if err := s.applyPatchToShard(shard, shardPatches); err != nil {
			shard.mu.Unlock()
			return err
		}
		shard.mu.Unlock()
	}

	return nil
}

// applyPatchToShard applies a patch to a specific shard (caller must hold lock)
func (s *StateStore) applyPatchToShard(shard *stateShard, patch JSONPatch) error {
	// Validate patch
	if err := patch.Validate(); err != nil {
		return fmt.Errorf("invalid patch: %w", err)
	}

	currentState := shard.current.Load().(*ImmutableState)

	// Create a shallow copy of current state data (copy-on-write)
	newData := make(map[string]interface{}, len(currentState.data))
	for k, v := range currentState.data {
		newData[k] = v
	}

	// For add operations, ensure parent paths exist
	for _, op := range patch {
		if op.Op == JSONPatchOpAdd && op.Path != "" && op.Path != "/" {
			if err := s.ensureParentPathsCOW(op.Path, newData); err != nil {
				return err
			}
		}
	}

	// Apply patch to copy
	newState, err := patch.Apply(newData)
	if err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	// Convert to map if needed
	newStateMap, ok := newState.(map[string]interface{})
	if !ok {
		// If the result is not a map, wrap it
		newStateMap = map[string]interface{}{
			"": newState,
		}
	}

	// Detect changes before creating new state
	changes := s.detectChanges(currentState.data, newStateMap, patch)

	// Create new immutable state
	newImmutableState := &ImmutableState{
		version: atomic.AddInt64(&s.version, 1),
		data:    newStateMap,
		refs:    0,
	}

	// Atomically update shard state
	shard.current.Store(newImmutableState)

	// Notify subscribers
	s.notifySubscribers(changes)

	// Note: Version creation is skipped for per-shard operations to avoid deadlock.
	// The sharded architecture trades off complete version history for better concurrency.

	return nil
}

// applyGlobalPatch applies a patch that affects the global state
func (s *StateStore) applyGlobalPatch(op JSONPatchOperation) error {
	// This is called with all shards already locked
	switch op.Op {
	case JSONPatchOpReplace:
		if m, ok := op.Value.(map[string]interface{}); ok {
			// Clear all shards and redistribute
			for _, shard := range s.shards {
				newState := &ImmutableState{
					version: atomic.LoadInt64(&s.version) + 1,
					data:    make(map[string]interface{}),
					refs:    0,
				}
				shard.current.Store(newState)
			}

			// Create a map to hold new data for each shard
			shardData := make([]map[string]interface{}, s.shardCount)
			for i := range shardData {
				shardData[i] = make(map[string]interface{})
			}

			// Distribute the data
			for k, v := range m {
				path := "/" + k
				shardIdx := s.getShardIndex(path)
				shardData[shardIdx][k] = v
			}

			// Update each shard with its new data
			newVersion := atomic.AddInt64(&s.version, 1)
			for i, shard := range s.shards {
				newState := &ImmutableState{
					version: newVersion,
					data:    shardData[i],
					refs:    0,
				}
				shard.current.Store(newState)
			}
		}
	default:
		return fmt.Errorf("unsupported global operation: %s", op.Op)
	}

	return nil
}

// applyPatchInternalCOW applies a patch using copy-on-write semantics
// Deprecated: This is kept for backward compatibility, use shard-specific methods
func (s *StateStore) applyPatchInternalCOW(patch JSONPatch) error {
	return s.ApplyPatch(patch)
}

// CreateSnapshot creates a snapshot of the current state
func (s *StateStore) CreateSnapshot() (*StateSnapshot, error) {
	// Get data from all shards
	stateCopy := s.getAllShardsData()

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate snapshot ID: %w", err)
	}

	s.historyMu.Lock()
	currentVersion := ""
	if len(s.history) > 0 {
		currentVersion = s.history[len(s.history)-1].ID
	}
	s.historyMu.Unlock()

	snapshot := &StateSnapshot{
		ID:        id,
		Timestamp: time.Now(),
		State:     stateCopy,
		Version:   currentVersion,
		Metadata:  make(map[string]interface{}),
	}

	return snapshot, nil
}

// RestoreSnapshot restores the state from a snapshot
func (s *StateStore) RestoreSnapshot(snapshot *StateSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot cannot be nil")
	}

	// Lock all shards for snapshot restore
	for _, shard := range s.shards {
		shard.mu.Lock()
		defer shard.mu.Unlock()
	}

	currentState := s.getAllShardsData()

	// Create patch to transform current state to snapshot state
	_ = s.createRestorePatch(currentState, snapshot.State)

	// Apply the restore patch
	return s.applyGlobalPatch(JSONPatchOperation{
		Op:    JSONPatchOpReplace,
		Path:  "/",
		Value: snapshot.State,
	})
}

// GetHistory returns the state history
func (s *StateStore) GetHistory() ([]*StateVersion, error) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	// Return a copy of history
	historyCopy := make([]*StateVersion, len(s.history))
	for i, v := range s.history {
		historyCopy[i] = &StateVersion{
			ID:        v.ID,
			Timestamp: v.Timestamp,
			State:     s.deepCopyState(v.State),
			Delta:     v.Delta,
			Metadata:  v.Metadata,
			ParentID:  v.ParentID,
		}
	}

	return historyCopy, nil
}

// Subscribe registers a callback for state changes at the specified path
func (s *StateStore) Subscribe(path string, callback SubscriptionCallback) func() {
	id, _ := generateID()
	now := time.Now()
	sub := &subscription{
		id:           id,
		path:         path,
		callback:     callback,
		lastAccessed: now,
		created:      now,
	}

	s.subscriptions.Store(id, sub)

	// Trigger cleanup if needed
	s.maybeCleanupSubscriptions()

	// Return unsubscribe function
	return func() {
		s.subscriptions.Delete(id)
	}
}

// Begin starts a new transaction
func (s *StateStore) Begin() *StateTransaction {
	// Get snapshot from all shards
	snapshot := s.getAllShardsData()

	id, _ := generateID()
	tx := &StateTransaction{
		store:    s,
		patches:  make(JSONPatch, 0),
		snapshot: snapshot,
	}

	s.transactionsMu.Lock()
	s.transactions[id] = tx
	s.transactionsMu.Unlock()

	return tx
}

// Transaction methods

// Apply adds a patch to the transaction
func (tx *StateTransaction) Apply(patch JSONPatch) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.committed {
		return fmt.Errorf("transaction already committed")
	}

	// Validate patch
	if err := patch.Validate(); err != nil {
		return fmt.Errorf("invalid patch: %w", err)
	}

	// Apply to transaction snapshot to validate
	newSnapshot, err := patch.Apply(tx.snapshot)
	if err != nil {
		return fmt.Errorf("failed to apply patch to transaction: %w", err)
	}

	// Update snapshot
	if snapMap, ok := newSnapshot.(map[string]interface{}); ok {
		tx.snapshot = snapMap
	} else {
		tx.snapshot = map[string]interface{}{"": newSnapshot}
	}

	// Add to patches
	tx.patches = append(tx.patches, patch...)

	return nil
}

// Commit commits the transaction
func (tx *StateTransaction) Commit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.committed {
		return fmt.Errorf("transaction already committed")
	}

	tx.committed = true

	// Apply all patches to the store
	if len(tx.patches) > 0 {
		return tx.store.ApplyPatch(tx.patches)
	}

	return nil
}

// Rollback discards the transaction
func (tx *StateTransaction) Rollback() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.committed {
		return fmt.Errorf("transaction already committed")
	}

	tx.committed = true
	tx.patches = nil
	tx.snapshot = nil

	return nil
}

// Helper methods

// deepCopyState creates a deep copy of the state map
func (s *StateStore) deepCopyState(state map[string]interface{}) map[string]interface{} {
	return deepCopy(state).(map[string]interface{})
}

// createVersion creates a new version entry
func (s *StateStore) createVersion(delta JSONPatch, metadata map[string]interface{}) {
	id, _ := generateID()

	// Get current state from all shards
	stateCopy := s.getAllShardsData()

	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	parentID := ""
	if len(s.history) > 0 {
		parentID = s.history[len(s.history)-1].ID
	}

	version := &StateVersion{
		ID:        id,
		Timestamp: time.Now(),
		State:     stateCopy,
		Delta:     delta,
		Metadata:  metadata,
		ParentID:  parentID,
	}

	s.history = append(s.history, version)

	// Atomic history trimming to prevent race conditions
	if len(s.history) > s.maxHistory {
		// More efficient trimming using copy to avoid memory leaks
		copy(s.history, s.history[len(s.history)-s.maxHistory:])
		s.history = s.history[:s.maxHistory]
	}
}

// createVersionWithState creates a new version entry with pre-computed state
func (s *StateStore) createVersionWithState(delta JSONPatch, state map[string]interface{}, metadata map[string]interface{}) {
	id, _ := generateID()

	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	parentID := ""
	if len(s.history) > 0 {
		parentID = s.history[len(s.history)-1].ID
	}

	version := &StateVersion{
		ID:        id,
		Timestamp: time.Now(),
		State:     state,
		Delta:     delta,
		Metadata:  metadata,
		ParentID:  parentID,
	}

	s.history = append(s.history, version)

	// Atomic history trimming to prevent race conditions
	if len(s.history) > s.maxHistory {
		// More efficient trimming using copy to avoid memory leaks
		copy(s.history, s.history[len(s.history)-s.maxHistory:])
		s.history = s.history[:s.maxHistory]
	}
}

// detectChanges detects what changed between two states
func (s *StateStore) detectChanges(oldState, newState map[string]interface{}, patch JSONPatch) []StateChange {
	changes := make([]StateChange, 0)
	timestamp := time.Now()

	for _, op := range patch {
		change := StateChange{
			Path:      op.Path,
			Operation: string(op.Op),
			Timestamp: timestamp,
		}

		// Get old value
		if oldVal, err := getValueAtPath(oldState, op.Path); err == nil {
			change.OldValue = oldVal
		}

		// Get new value
		if op.Op != JSONPatchOpRemove {
			if newVal, err := getValueAtPath(newState, op.Path); err == nil {
				change.NewValue = newVal
			}
		}

		changes = append(changes, change)
	}

	return changes
}

// notifySubscribers notifies all relevant subscribers of changes
func (s *StateStore) notifySubscribers(changes []StateChange) {
	// Collect notifications without holding locks
	var notifications []func()

	for _, change := range changes {
		s.subscriptions.Range(func(key, value interface{}) bool {
			sub := value.(*subscription)
			if s.pathMatches(sub.path, change.Path) {
				// Update last accessed time
				sub.lastAccessed = time.Now()

				// Capture variables for closure
				cb := sub.callback
				ch := change

				notifications = append(notifications, func() {
					defer func() {
						if r := recover(); r != nil {
							// Report panic through error handler
							if s.errorHandler != nil {
								s.errorHandler(fmt.Errorf("subscriber callback panic: %v", r))
							}
						}
					}()

					// Additional safety check - ensure callback is not nil
					if cb != nil {
						cb(ch)
					} else if s.errorHandler != nil {
						s.errorHandler(fmt.Errorf("nil callback encountered for subscription %s", sub.id))
					}
				})
			}
			return true
		})
	}

	// Execute all notifications asynchronously
	for _, notify := range notifications {
		go notify()
	}
}

// pathMatches checks if a subscription path matches a change path
func (s *StateStore) pathMatches(subPath, changePath string) bool {
	// Exact match
	if subPath == changePath {
		return true
	}

	// Root path matches everything
	if subPath == "/" || subPath == "" {
		return true
	}

	// Wildcard match (e.g., /users/* matches /users/123)
	if strings.HasSuffix(subPath, "/*") {
		prefix := strings.TrimSuffix(subPath, "/*")
		return strings.HasPrefix(changePath, prefix+"/")
	}

	// Parent path match (e.g., /users matches /users/123)
	return strings.HasPrefix(changePath, subPath+"/")
}

// createRestorePatch creates a patch to transform oldState to newState
func (s *StateStore) createRestorePatch(oldState, newState map[string]interface{}) JSONPatch {
	// This is a simplified implementation
	// A full implementation would calculate minimal patches
	return JSONPatch{
		JSONPatchOperation{
			Op:    JSONPatchOpReplace,
			Path:  "/",
			Value: newState,
		},
	}
}

// generateID generates a unique identifier
func generateID() (string, error) {
	bytes := make([]byte, RandomIDBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GetVersion returns the current version number
func (s *StateStore) GetVersion() int64 {
	return atomic.LoadInt64(&s.version)
}

// GetState returns a copy of the complete current state
func (s *StateStore) GetState() map[string]interface{} {
	// Get data from all shards
	return s.getAllShardsData()
}

// GetStateView returns a read-only view of the current state
// This is more efficient than GetState as it avoids deep copying
func (s *StateStore) GetStateView() *StateView {
	// For sharded implementation, we need to merge data from all shards
	// This is less efficient than single-shard access but maintains compatibility
	merged := s.getAllShardsData()

	return &StateView{
		data: merged,
		cleanup: func() {
			// No cleanup needed for merged data
		},
	}
}

// Data returns the underlying map for read-only access
func (v *StateView) Data() map[string]interface{} {
	return v.data
}

// Cleanup releases the reference to the immutable state
func (v *StateView) Cleanup() {
	if v.cleanup != nil {
		v.cleanup()
		v.cleanup = nil
	}
}

// Clear removes all state and history
func (s *StateStore) Clear() {
	// Lock all shards
	for _, shard := range s.shards {
		shard.mu.Lock()
		defer shard.mu.Unlock()
	}

	// Clear all shards
	for _, shard := range s.shards {
		newState := &ImmutableState{
			version: 0,
			data:    make(map[string]interface{}),
			refs:    0,
		}
		shard.current.Store(newState)
	}

	atomic.StoreInt64(&s.version, 0)

	// Clear history under separate lock
	s.historyMu.Lock()
	s.history = make([]*StateVersion, 0)
	s.historyMu.Unlock()

	// Skip version creation for Clear to avoid issues
}

// Export exports the current state as JSON
func (s *StateStore) Export() ([]byte, error) {
	// Get data from all shards
	merged := s.getAllShardsData()
	return json.MarshalIndent(merged, "", "  ")
}

// Import imports state from JSON
func (s *StateStore) Import(data []byte) error {
	var newStateData map[string]interface{}
	if err := json.Unmarshal(data, &newStateData); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Lock all shards for import
	for _, shard := range s.shards {
		shard.mu.Lock()
		defer shard.mu.Unlock()
	}

	// Clear all shards first
	for _, shard := range s.shards {
		newState := &ImmutableState{
			version: atomic.AddInt64(&s.version, 1),
			data:    make(map[string]interface{}),
			refs:    0,
		}
		shard.current.Store(newState)
	}

	// Create a map to hold new data for each shard
	shardData := make([]map[string]interface{}, s.shardCount)
	for i := range shardData {
		shardData[i] = make(map[string]interface{})
	}

	// Distribute the data
	for k, v := range newStateData {
		path := "/" + k
		shardIdx := s.getShardIndex(path)
		shardData[shardIdx][k] = v
	}

	// Update each shard with its new data
	for i, shard := range s.shards {
		state := shard.current.Load().(*ImmutableState)
		newState := &ImmutableState{
			version: state.version + 1,
			data:    shardData[i],
			refs:    0,
		}
		shard.current.Store(newState)
	}

	// Create version entry with pre-computed state
	patch := JSONPatch{JSONPatchOperation{Op: JSONPatchOpReplace, Path: "/", Value: newStateData}}
	s.createVersionWithState(patch, deepCopy(newStateData).(map[string]interface{}), nil)

	return nil
}

// maybeCleanupSubscriptions performs cleanup if enough time has passed
func (s *StateStore) maybeCleanupSubscriptions() {
	now := time.Now()
	if now.Sub(s.lastCleanup) < s.cleanupInterval {
		return
	}

	s.lastCleanup = now
	go s.cleanupExpiredSubscriptions()
}

// cleanupExpiredSubscriptions removes expired subscriptions
func (s *StateStore) cleanupExpiredSubscriptions() {
	cutoff := time.Now().Add(-s.subscriptionTTL)

	s.subscriptions.Range(func(key, value interface{}) bool {
		sub := value.(*subscription)
		if sub.lastAccessed.Before(cutoff) {
			s.subscriptions.Delete(key)
		}
		return true
	})
}

// WithSubscriptionTTL sets the subscription time-to-live
func WithSubscriptionTTL(ttl time.Duration) StateStoreOption {
	return func(s *StateStore) {
		s.subscriptionTTL = ttl
	}
}

// WithCleanupInterval sets the cleanup interval
func WithCleanupInterval(interval time.Duration) StateStoreOption {
	return func(s *StateStore) {
		s.cleanupInterval = interval
	}
}

// collectGarbage performs garbage collection on old immutable states
// This is automatically called periodically or can be called manually
func (s *StateStore) collectGarbage() {
	// Since we're using atomic.Value, old states will be GC'd automatically
	// when they have zero references. This method is here for future
	// optimizations if needed.
}

// GetReferenceCount returns the current reference count for the state
// This is mainly for debugging and testing
func (s *StateStore) GetReferenceCount() int32 {
	// Return total reference count across all shards
	var totalRefs int32
	for _, shard := range s.shards {
		state := shard.current.Load().(*ImmutableState)
		totalRefs += atomic.LoadInt32(&state.refs)
	}
	return totalRefs
}
