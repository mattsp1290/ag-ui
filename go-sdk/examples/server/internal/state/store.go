package state

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	mattbaird "github.com/mattbaird/jsonpatch"
)

// UpdateFunc is a function type for state updates
type UpdateFunc func(*State)

// StateChange represents a state change notification
type StateChange struct {
	OldState *State
	NewState *State
	Patch    []map[string]interface{}
	Version  int64
}

// Watcher represents a subscription to state changes
type Watcher struct {
	ch     chan *StateDelta
	ctx    context.Context
	cancel context.CancelFunc
	id     string
	mu     sync.Mutex
	closed bool
}

// NewWatcher creates a new state watcher
func NewWatcher(ctx context.Context, id string) *Watcher {
	childCtx, cancel := context.WithCancel(ctx)
	return &Watcher{
		ch:     make(chan *StateDelta, 100), // Buffered channel to prevent blocking
		ctx:    childCtx,
		cancel: cancel,
		id:     id,
	}
}

// Channel returns the watcher's channel for receiving state deltas
func (w *Watcher) Channel() <-chan *StateDelta {
	return w.ch
}

// Context returns the watcher's context
func (w *Watcher) Context() context.Context {
	return w.ctx
}

// Close closes the watcher and cleans up resources
func (w *Watcher) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.closed {
		w.closed = true
		w.cancel()
		close(w.ch)
	}
}

// Send sends a delta to the watcher (non-blocking)
func (w *Watcher) Send(delta *StateDelta) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return false
	}

	select {
	case w.ch <- delta:
		return true
	default:
		// Channel is full, skip this delta to prevent blocking
		slog.Warn("Watcher channel full, skipping delta", "watcher_id", w.id, "version", delta.Version)
		return false
	}
}

// Store manages the shared state with concurrency safety
type Store struct {
	mu        sync.RWMutex
	state     *State
	watchers  map[string]*Watcher
	watcherMu sync.RWMutex
	logger    *slog.Logger
}

// NewStore creates a new Store with initial state
func NewStore() *Store {
	return &Store{
		state:    NewState(),
		watchers: make(map[string]*Watcher),
		logger:   slog.Default(),
	}
}

// WithLogger sets a custom logger for the store
func (s *Store) WithLogger(logger *slog.Logger) *Store {
	s.logger = logger
	return s
}

// Snapshot returns a copy of the current state
func (s *Store) Snapshot() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Clone()
}

// Update applies the given update function and generates/validates patches
func (s *Store) Update(updateFn UpdateFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clone current state for comparison
	oldState := s.state.Clone()
	oldJSON, err := oldState.ToJSON()
	if err != nil {
		s.logger.Error("Failed to serialize old state", "error", err)
		return fmt.Errorf("failed to serialize old state: %w", err)
	}

	// Apply the update to a cloned state
	newState := oldState.Clone()
	updateFn(newState)

	// Increment version
	newState.Version = oldState.Version + 1

	// Generate new state JSON
	newJSON, err := newState.ToJSON()
	if err != nil {
		s.logger.Error("Failed to serialize new state", "error", err)
		return fmt.Errorf("failed to serialize new state: %w", err)
	}

	// Generate RFC 6902 patch
	patchOps, err := mattbaird.CreatePatch(oldJSON, newJSON)
	if err != nil {
		s.logger.Error("Failed to create JSON patch", "error", err)
		return fmt.Errorf("failed to create JSON patch: %w", err)
	}

	// Convert patch operations to map format for JSON serialization
	patchMaps := make([]map[string]interface{}, len(patchOps))
	for i, op := range patchOps {
		patchMaps[i] = map[string]interface{}{
			"op":   op.Operation,
			"path": op.Path,
		}
		if op.Value != nil {
			patchMaps[i]["value"] = op.Value
		}
	}

	// Validate patch by applying it back to the old state
	if err := s.validatePatch(oldJSON, newJSON, patchMaps); err != nil {
		s.logger.Error("Patch validation failed", "error", err)
		return fmt.Errorf("patch validation failed: %w", err)
	}

	// Only broadcast if there are actual changes
	if len(patchOps) > 0 {
		// Commit the new state
		s.state = newState

		s.logger.Debug("State updated",
			"old_version", oldState.Version,
			"new_version", newState.Version,
			"patch_operations", len(patchOps))

		// Broadcast the change to watchers
		delta := NewStateDelta(newState.Version, patchMaps)
		s.broadcastDelta(delta)
	} else {
		s.logger.Debug("No changes detected in state update", "version", oldState.Version)
	}

	return nil
}

// validatePatch applies the patch to the old JSON and verifies it matches the new JSON
func (s *Store) validatePatch(oldJSON, newJSON []byte, patchMaps []map[string]interface{}) error {
	// Convert patch maps to JSON patch format for evanphx library
	patchBytes, err := json.Marshal(patchMaps)
	if err != nil {
		return fmt.Errorf("failed to marshal patch for validation: %w", err)
	}

	// Create patch using evanphx library
	patch, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		return fmt.Errorf("failed to decode patch for validation: %w", err)
	}

	// Apply the patch to the old JSON
	appliedJSON, err := patch.Apply(oldJSON)
	if err != nil {
		return fmt.Errorf("failed to apply patch for validation: %w", err)
	}

	// Compare the applied result with the expected new JSON
	// We need to normalize both JSONs for comparison (e.g., handle field ordering)
	var applied, expected interface{}

	if err := json.Unmarshal(appliedJSON, &applied); err != nil {
		return fmt.Errorf("failed to unmarshal applied JSON: %w", err)
	}

	if err := json.Unmarshal(newJSON, &expected); err != nil {
		return fmt.Errorf("failed to unmarshal expected JSON: %w", err)
	}

	// Re-marshal both to ensure consistent formatting
	appliedNormalized, marshalAppliedErr := json.Marshal(applied)
	if marshalAppliedErr != nil {
		return fmt.Errorf("failed to marshal applied JSON: %w", marshalAppliedErr)
	}

	expectedNormalized, marshalExpectedErr := json.Marshal(expected)
	if marshalExpectedErr != nil {
		return fmt.Errorf("failed to marshal expected JSON: %w", marshalExpectedErr)
	}

	// Compare normalized JSON strings
	if string(appliedNormalized) != string(expectedNormalized) {
		s.logger.Error("Patch validation failed",
			"applied", string(appliedNormalized),
			"expected", string(expectedNormalized))
		return fmt.Errorf("patch application does not match expected result")
	}

	return nil
}

// Watch creates a new watcher for state changes
func (s *Store) Watch(ctx context.Context) (*Watcher, error) {
	watcherID := fmt.Sprintf("watcher_%d", time.Now().UnixNano())
	watcher := NewWatcher(ctx, watcherID)

	s.watcherMu.Lock()
	s.watchers[watcherID] = watcher
	s.watcherMu.Unlock()

	// Clean up the watcher when its context is canceled
	go func() {
		<-watcher.Context().Done()
		s.removeWatcher(watcherID)
	}()

	s.logger.Debug("New watcher registered", "watcher_id", watcherID)
	return watcher, nil
}

// removeWatcher removes a watcher from the store
func (s *Store) removeWatcher(watcherID string) {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()

	if watcher, exists := s.watchers[watcherID]; exists {
		watcher.Close()
		delete(s.watchers, watcherID)
		s.logger.Debug("Watcher removed", "watcher_id", watcherID)
	}
}

// broadcastDelta sends a state delta to all watchers
func (s *Store) broadcastDelta(delta *StateDelta) {
	s.watcherMu.RLock()
	defer s.watcherMu.RUnlock()

	successCount := 0
	for _, watcher := range s.watchers {
		if watcher.Send(delta) {
			successCount++
		}
	}

	s.logger.Debug("Broadcasted delta to watchers",
		"version", delta.Version,
		"total_watchers", len(s.watchers),
		"successful_sends", successCount)
}

// GetWatcherCount returns the number of active watchers
func (s *Store) GetWatcherCount() int {
	s.watcherMu.RLock()
	defer s.watcherMu.RUnlock()
	return len(s.watchers)
}

// Close closes the store and all watchers
func (s *Store) Close() {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()

	for _, watcher := range s.watchers {
		watcher.Close()
	}
	s.watchers = make(map[string]*Watcher)
	s.logger.Debug("Store closed, all watchers removed")
}
