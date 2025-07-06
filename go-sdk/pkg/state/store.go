package state

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
	// Separate locks for different operations to reduce contention
	stateMu           sync.RWMutex     // Lock for state operations
	transactionsMu    sync.RWMutex     // Lock for transaction management
	historyMu         sync.Mutex       // Lock for history operations
	
	state             map[string]interface{}
	version           int64
	history           []*StateVersion
	maxHistory        int
	subscriptions     sync.Map // Using sync.Map to reduce lock contention
	transactions      map[string]*StateTransaction
	
	// Subscription management
	subscriptionTTL   time.Duration
	lastCleanup       time.Time
	cleanupInterval   time.Duration
}

// NewStateStore creates a new state store instance
func NewStateStore(options ...StateStoreOption) *StateStore {
	store := &StateStore{
		state:             make(map[string]interface{}),
		version:           0,
		history:           make([]*StateVersion, 0),
		maxHistory:        1000, // Default max history
		transactions:      make(map[string]*StateTransaction),
		subscriptionTTL:   1 * time.Hour,   // Default TTL for subscriptions
		cleanupInterval:   10 * time.Minute, // Default cleanup interval
		lastCleanup:       time.Now(),
	}

	// Apply options
	for _, opt := range options {
		opt(store)
	}

	// Create initial version
	store.createVersion(nil, nil)

	return store
}

// StateStoreOption is a configuration option for StateStore
type StateStoreOption func(*StateStore)

// WithMaxHistory sets the maximum number of history entries to keep
func WithMaxHistory(max int) StateStoreOption {
	return func(s *StateStore) {
		s.maxHistory = max
	}
}

// Get retrieves a value at the specified path
func (s *StateStore) Get(path string) (interface{}, error) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()

	if path == "" || path == "/" {
		return s.deepCopyState(s.state), nil
	}

	value, err := getValueAtPath(s.state, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get value at path %s: %w", path, err)
	}

	return deepCopy(value), nil
}

// Set updates a value at the specified path
func (s *StateStore) Set(path string, value interface{}) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	// Handle root path
	if path == "" || path == "/" {
		oldState := s.deepCopyState(s.state)
		s.state = make(map[string]interface{})
		if m, ok := value.(map[string]interface{}); ok {
			s.state = m
		} else {
			s.state[""] = value
		}
		s.version++
		patch := JSONPatch{{Op: JSONPatchOpReplace, Path: "/", Value: value}}
		s.createVersion(patch, nil)
		
		// Notify subscribers
		changes := s.detectChanges(oldState, s.state, patch)
		s.notifySubscribers(changes)
		return nil
	}

	// Ensure parent paths exist
	if err := s.ensureParentPaths(path); err != nil {
		return err
	}

	// Create a patch for this operation
	patch := JSONPatch{
		{
			Op:    JSONPatchOpReplace,
			Path:  path,
			Value: value,
		},
	}

	// Check if path exists, if not use add operation
	if _, err := getValueAtPath(s.state, path); err != nil {
		patch[0].Op = JSONPatchOpAdd
	}

	return s.applyPatchInternal(patch)
}

// ensureParentPaths ensures all parent paths exist as objects
func (s *StateStore) ensureParentPaths(path string) error {
	if path == "" || path == "/" {
		return nil
	}

	tokens := parseJSONPointer(path)
	var current interface{} = s.state

	for i := 0; i < len(tokens)-1; i++ {
		token := tokens[i]
		
		switch c := current.(type) {
		case map[string]interface{}:
			if _, exists := c[token]; !exists {
				c[token] = make(map[string]interface{})
			}
			current = c[token]
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
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if path == "" || path == "/" {
		return fmt.Errorf("cannot delete root")
	}

	patch := JSONPatch{
		{
			Op:   JSONPatchOpRemove,
			Path: path,
		},
	}

	return s.applyPatchInternal(patch)
}

// ApplyPatch applies a JSON Patch to the state
func (s *StateStore) ApplyPatch(patch JSONPatch) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	return s.applyPatchInternal(patch)
}

// applyPatchInternal applies a patch without locking (internal use)
func (s *StateStore) applyPatchInternal(patch JSONPatch) error {
	// Validate patch
	if err := patch.Validate(); err != nil {
		return fmt.Errorf("invalid patch: %w", err)
	}

	// Create a copy of current state
	stateCopy := s.deepCopyState(s.state)

	// Apply patch to copy
	newState, err := patch.Apply(stateCopy)
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

	// Notify subscribers before updating state
	changes := s.detectChanges(s.state, newStateMap, patch)

	// Update state
	s.state = newStateMap
	s.version++

	// Create new version
	s.createVersion(patch, nil)

	// Notify subscribers
	s.notifySubscribers(changes)

	return nil
}

// CreateSnapshot creates a snapshot of the current state
func (s *StateStore) CreateSnapshot() (*StateSnapshot, error) {
	s.stateMu.RLock()
	stateCopy := s.deepCopyState(s.state)
	s.stateMu.RUnlock()
	
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

	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	// Create patch to transform current state to snapshot state
	patch := s.createRestorePatch(s.state, snapshot.State)

	// Apply the restore patch
	return s.applyPatchInternal(patch)
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
	s.stateMu.RLock()
	snapshot := s.deepCopyState(s.state)
	s.stateMu.RUnlock()

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
	
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	
	parentID := ""
	if len(s.history) > 0 {
		parentID = s.history[len(s.history)-1].ID
	}

	version := &StateVersion{
		ID:        id,
		Timestamp: time.Now(),
		State:     s.deepCopyState(s.state),
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
							// Log panic but don't crash - subscriber callbacks should not crash the system
							fmt.Printf("[StateStore] Subscriber callback panic: %v\n", r)
						}
					}()
					
					// Additional safety check - ensure callback is not nil
					if cb != nil {
						cb(ch)
					} else {
						fmt.Printf("[StateStore] Warning: nil callback encountered\n")
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
		{
			Op:    JSONPatchOpReplace,
			Path:  "/",
			Value: newState,
		},
	}
}

// generateID generates a unique identifier
func generateID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GetVersion returns the current version number
func (s *StateStore) GetVersion() int64 {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.version
}

// GetState returns a copy of the complete current state
func (s *StateStore) GetState() map[string]interface{} {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.deepCopyState(s.state)
}

// Clear removes all state and history
func (s *StateStore) Clear() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	s.state = make(map[string]interface{})
	s.version = 0
	
	// Clear history under separate lock
	s.historyMu.Lock()
	s.history = make([]*StateVersion, 0)
	s.historyMu.Unlock()
	
	s.createVersion(nil, nil)
}

// Export exports the current state as JSON
func (s *StateStore) Export() ([]byte, error) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()

	return json.MarshalIndent(s.state, "", "  ")
}

// Import imports state from JSON
func (s *StateStore) Import(data []byte) error {
	var newState map[string]interface{}
	if err := json.Unmarshal(data, &newState); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	// Create patch to transform current state to imported state
	patch := s.createRestorePatch(s.state, newState)
	return s.applyPatchInternal(patch)
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