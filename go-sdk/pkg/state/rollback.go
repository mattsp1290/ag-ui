package state

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

// RollbackManager manages state rollback operations.
type RollbackManager interface {
	// RollbackToVersion rolls back state to a specific version
	RollbackToVersion(versionID string) error

	// RollbackToTimestamp rolls back state to a specific point in time
	RollbackToTimestamp(timestamp time.Time) error

	// CreateMarker creates a named rollback point
	CreateMarker(name string) error

	// RollbackToMarker rolls back to a named marker
	RollbackToMarker(name string) error

	// ListMarkers returns all available markers
	ListMarkers() ([]RollbackMarker, error)

	// DeleteMarker removes a rollback marker
	DeleteMarker(name string) error

	// GetRollbackHistory returns the history of rollback operations
	GetRollbackHistory() ([]RollbackOperation, error)

	// CanRollback checks if rollback is possible to a specific version
	CanRollback(versionID string) (bool, error)
}

// RollbackMarker represents a named rollback point.
type RollbackMarker struct {
	// Name is the unique identifier for this marker
	Name string `json:"name"`

	// VersionID is the state version this marker points to
	VersionID string `json:"versionId"`

	// Description provides context for this marker
	Description string `json:"description,omitempty"`

	// CreatedAt is when this marker was created
	CreatedAt time.Time `json:"createdAt"`

	// Metadata contains additional marker information
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// RollbackOperation represents a completed rollback operation.
type RollbackOperation struct {
	// ID is the unique identifier for this operation
	ID string `json:"id"`

	// Type indicates the rollback type (version, timestamp, marker)
	Type string `json:"type"`

	// FromVersion is the version before rollback
	FromVersion string `json:"fromVersion"`

	// ToVersion is the version after rollback
	ToVersion string `json:"toVersion"`

	// Target is what was rolled back to (version ID, timestamp, or marker name)
	Target string `json:"target"`

	// Timestamp is when the rollback occurred
	Timestamp time.Time `json:"timestamp"`

	// Success indicates if the rollback succeeded
	Success bool `json:"success"`

	// Error contains any error message if rollback failed
	Error string `json:"error,omitempty"`

	// Metadata contains additional operation information
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// RollbackStrategy defines how rollback operations are performed.
type RollbackStrategy interface {
	// Execute performs the rollback operation
	Execute(store StoreInterface, targetVersion *StateVersion) error

	// Validate checks if the rollback can be performed safely
	Validate(store StoreInterface, targetVersion *StateVersion) error

	// Name returns the strategy name
	Name() string
}

// StateRollback implements RollbackManager with various rollback strategies.
type StateRollback struct {
	store      StoreInterface
	validator  StateValidator
	strategy   RollbackStrategy
	markers    map[string]*RollbackMarker
	history    []*RollbackOperation
	maxHistory int
	mu         sync.RWMutex
}

// NewStateRollback creates a new rollback manager.
func NewStateRollback(store StoreInterface, options ...RollbackOption) *StateRollback {
	rollback := &StateRollback{
		store:      store,
		strategy:   NewSafeRollbackStrategy(),
		markers:    make(map[string]*RollbackMarker),
		history:    make([]*RollbackOperation, 0),
		maxHistory: 100,
	}

	// Apply options
	for _, opt := range options {
		opt(rollback)
	}

	return rollback
}

// RollbackOption is a configuration option for StateRollback.
type RollbackOption func(*StateRollback)

// WithValidator sets the state validator for rollback validation.
func WithValidator(validator StateValidator) RollbackOption {
	return func(r *StateRollback) {
		r.validator = validator
	}
}

// WithStrategy sets the rollback strategy.
func WithStrategy(strategy RollbackStrategy) RollbackOption {
	return func(r *StateRollback) {
		r.strategy = strategy
	}
}

// WithRollbackMaxHistory sets the maximum rollback history to maintain.
func WithRollbackMaxHistory(max int) RollbackOption {
	return func(r *StateRollback) {
		r.maxHistory = max
	}
}

// RollbackToVersion rolls back state to a specific version.
func (r *StateRollback) RollbackToVersion(versionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find target version
	targetVersion, err := r.findVersion(versionID)
	if err != nil {
		return fmt.Errorf("failed to find version %s: %w", versionID, err)
	}

	// Get current version
	currentVersion := r.getCurrentVersion()

	// Validate rollback
	if r.validator != nil {
		result, err := r.validator.Validate(targetVersion.State)
		if err != nil {
			return fmt.Errorf("failed to validate target state: %w", err)
		}
		if !result.Valid {
			return fmt.Errorf("target state validation failed: %v", result.Errors)
		}
	}

	// Validate with strategy
	if err := r.strategy.Validate(r.store, targetVersion); err != nil {
		return fmt.Errorf("rollback validation failed: %w", err)
	}

	// Execute rollback
	if err := r.strategy.Execute(r.store, targetVersion); err != nil {
		r.recordOperation("version", currentVersion.ID, "", versionID, false, err.Error())
		return fmt.Errorf("rollback execution failed: %w", err)
	}

	// Record successful operation
	newVersion := r.getCurrentVersion()
	r.recordOperation("version", currentVersion.ID, newVersion.ID, versionID, true, "")

	return nil
}

// RollbackToTimestamp rolls back state to a specific point in time.
func (r *StateRollback) RollbackToTimestamp(timestamp time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find version at or before timestamp
	targetVersion, err := r.findVersionByTimestamp(timestamp)
	if err != nil {
		return fmt.Errorf("failed to find version at timestamp %v: %w", timestamp, err)
	}

	// Get current version
	currentVersion := r.getCurrentVersion()

	// Validate and execute rollback
	if r.validator != nil {
		result, err := r.validator.Validate(targetVersion.State)
		if err != nil {
			return fmt.Errorf("failed to validate target state: %w", err)
		}
		if !result.Valid {
			return fmt.Errorf("target state validation failed: %v", result.Errors)
		}
	}

	if err := r.strategy.Validate(r.store, targetVersion); err != nil {
		return fmt.Errorf("rollback validation failed: %w", err)
	}

	if err := r.strategy.Execute(r.store, targetVersion); err != nil {
		r.recordOperation("timestamp", currentVersion.ID, "", timestamp.String(), false, err.Error())
		return fmt.Errorf("rollback execution failed: %w", err)
	}

	// Record successful operation
	newVersion := r.getCurrentVersion()
	r.recordOperation("timestamp", currentVersion.ID, newVersion.ID, timestamp.String(), true, "")

	return nil
}

// CreateMarker creates a named rollback point.
func (r *StateRollback) CreateMarker(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if name == "" {
		return fmt.Errorf("marker name cannot be empty")
	}

	if _, exists := r.markers[name]; exists {
		return fmt.Errorf("marker %s already exists", name)
	}

	currentVersion := r.getCurrentVersion()
	if currentVersion == nil {
		return fmt.Errorf("no current version available")
	}

	marker := &RollbackMarker{
		Name:      name,
		VersionID: currentVersion.ID,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	r.markers[name] = marker
	return nil
}

// RollbackToMarker rolls back to a named marker.
func (r *StateRollback) RollbackToMarker(name string) error {
	r.mu.RLock()
	marker, exists := r.markers[name]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("marker %s not found", name)
	}

	// Use RollbackToVersion with the marker's version
	err := r.RollbackToVersion(marker.VersionID)
	if err != nil {
		return fmt.Errorf("failed to rollback to marker %s: %w", name, err)
	}

	// Update operation history to note it was a marker rollback
	r.mu.Lock()
	if len(r.history) > 0 {
		r.history[len(r.history)-1].Type = "marker"
		r.history[len(r.history)-1].Target = name
	}
	r.mu.Unlock()

	return nil
}

// ListMarkers returns all available markers.
func (r *StateRollback) ListMarkers() ([]RollbackMarker, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	markers := make([]RollbackMarker, 0, len(r.markers))
	for _, marker := range r.markers {
		markers = append(markers, *marker)
	}

	return markers, nil
}

// DeleteMarker removes a rollback marker.
func (r *StateRollback) DeleteMarker(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.markers[name]; !exists {
		return fmt.Errorf("marker %s not found", name)
	}

	delete(r.markers, name)
	return nil
}

// GetRollbackHistory returns the history of rollback operations.
func (r *StateRollback) GetRollbackHistory() ([]RollbackOperation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	history := make([]RollbackOperation, len(r.history))
	for i, op := range r.history {
		history[i] = *op
	}

	return history, nil
}

// CanRollback checks if rollback is possible to a specific version.
func (r *StateRollback) CanRollback(versionID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find target version
	targetVersion, err := r.findVersion(versionID)
	if err != nil {
		return false, err
	}

	// Validate with strategy
	if err := r.strategy.Validate(r.store, targetVersion); err != nil {
		return false, nil
	}

	// Validate state if validator is available
	if r.validator != nil {
		result, err := r.validator.Validate(targetVersion.State)
		if err != nil {
			return false, err
		}
		return result.Valid, nil
	}

	return true, nil
}

// Helper methods

// findVersion finds a version by ID in the store's history.
func (r *StateRollback) findVersion(versionID string) (*StateVersion, error) {
	history, err := r.store.GetHistory()
	if err != nil {
		return nil, err
	}

	for _, version := range history {
		if version.ID == versionID {
			return version, nil
		}
	}

	return nil, fmt.Errorf("version %s not found", versionID)
}

// findVersionByTimestamp finds the latest version at or before the given timestamp.
func (r *StateRollback) findVersionByTimestamp(timestamp time.Time) (*StateVersion, error) {
	history, err := r.store.GetHistory()
	if err != nil {
		return nil, err
	}

	var targetVersion *StateVersion
	for _, version := range history {
		if version.Timestamp.Before(timestamp) || version.Timestamp.Equal(timestamp) {
			targetVersion = version
		} else {
			break
		}
	}

	if targetVersion == nil {
		return nil, fmt.Errorf("no version found at or before %v", timestamp)
	}

	return targetVersion, nil
}

// getCurrentVersion gets the current version from the store.
func (r *StateRollback) getCurrentVersion() *StateVersion {
	history, err := r.store.GetHistory()
	if err != nil || len(history) == 0 {
		return nil
	}
	return history[len(history)-1]
}

// recordOperation records a rollback operation in history.
func (r *StateRollback) recordOperation(opType, fromVersion, toVersion, target string, success bool, errorMsg string) {
	id, _ := generateID()

	operation := &RollbackOperation{
		ID:          id,
		Type:        opType,
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		Target:      target,
		Timestamp:   time.Now(),
		Success:     success,
		Error:       errorMsg,
		Metadata:    make(map[string]interface{}),
	}

	r.history = append(r.history, operation)

	// Trim history if needed
	if len(r.history) > r.maxHistory {
		r.history = r.history[len(r.history)-r.maxHistory:]
	}
}

// Rollback Strategies

// SafeRollbackStrategy implements a safe rollback approach.
type SafeRollbackStrategy struct {
	createSnapshot bool
}

// NewSafeRollbackStrategy creates a new safe rollback strategy.
func NewSafeRollbackStrategy() RollbackStrategy {
	return &SafeRollbackStrategy{
		createSnapshot: true,
	}
}

// Name returns the strategy name.
func (s *SafeRollbackStrategy) Name() string {
	return "safe"
}

// Validate checks if the rollback can be performed safely.
func (s *SafeRollbackStrategy) Validate(store StoreInterface, targetVersion *StateVersion) error {
	if store == nil {
		return fmt.Errorf("store cannot be nil")
	}
	if targetVersion == nil {
		return fmt.Errorf("target version cannot be nil")
	}

	// Check if target state is valid
	if targetVersion.State == nil {
		return fmt.Errorf("target version has no state")
	}

	return nil
}

// Execute performs the rollback operation.
func (s *SafeRollbackStrategy) Execute(store StoreInterface, targetVersion *StateVersion) error {
	// Create a snapshot before rollback if enabled
	if s.createSnapshot {
		_, err := store.CreateSnapshot()
		if err != nil {
			return fmt.Errorf("failed to create pre-rollback snapshot: %w", err)
		}
	}

	// Create a patch to transform current state to target state
	currentState := store.GetState()
	patch := createTransformPatch(currentState, targetVersion.State)

	// Apply the patch
	if err := store.ApplyPatch(patch); err != nil {
		return fmt.Errorf("failed to apply rollback patch: %w", err)
	}

	return nil
}

// FastRollbackStrategy implements a fast rollback without safety checks.
type FastRollbackStrategy struct{}

// NewFastRollbackStrategy creates a new fast rollback strategy.
func NewFastRollbackStrategy() RollbackStrategy {
	return &FastRollbackStrategy{}
}

// Name returns the strategy name.
func (f *FastRollbackStrategy) Name() string {
	return "fast"
}

// Validate performs minimal validation.
func (f *FastRollbackStrategy) Validate(store StoreInterface, targetVersion *StateVersion) error {
	if store == nil || targetVersion == nil {
		return fmt.Errorf("invalid parameters")
	}
	return nil
}

// Execute performs direct state replacement.
func (f *FastRollbackStrategy) Execute(store StoreInterface, targetVersion *StateVersion) error {
	// Direct state replacement using a single patch
	patch := JSONPatch{
		JSONPatchOperation{
			Op:    JSONPatchOpReplace,
			Path:  "/",
			Value: targetVersion.State,
		},
	}

	return store.ApplyPatch(patch)
}

// IncrementalRollbackStrategy applies changes incrementally.
type IncrementalRollbackStrategy struct {
	batchSize int
}

// NewIncrementalRollbackStrategy creates a new incremental rollback strategy.
func NewIncrementalRollbackStrategy(batchSize int) RollbackStrategy {
	return &IncrementalRollbackStrategy{
		batchSize: batchSize,
	}
}

// Name returns the strategy name.
func (i *IncrementalRollbackStrategy) Name() string {
	return "incremental"
}

// Validate checks if incremental rollback is possible.
func (i *IncrementalRollbackStrategy) Validate(store StoreInterface, targetVersion *StateVersion) error {
	if store == nil || targetVersion == nil {
		return fmt.Errorf("invalid parameters")
	}
	if i.batchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}
	return nil
}

// Execute performs incremental rollback.
func (i *IncrementalRollbackStrategy) Execute(store StoreInterface, targetVersion *StateVersion) error {
	currentState := store.GetState()
	patches := createDetailedTransformPatches(currentState, targetVersion.State)

	// Apply patches in batches
	for j := 0; j < len(patches); j += i.batchSize {
		end := j + i.batchSize
		if end > len(patches) {
			end = len(patches)
		}

		batch := patches[j:end]
		if err := store.ApplyPatch(batch); err != nil {
			return fmt.Errorf("failed to apply batch %d: %w", j/i.batchSize, err)
		}
	}

	return nil
}

// Helper functions for creating patches

// createTransformPatch creates a simple patch to transform one state to another.
func createTransformPatch(from, to map[string]interface{}) JSONPatch {
	// This is a simplified implementation
	// A full implementation would calculate minimal patches
	return JSONPatch{
		JSONPatchOperation{
			Op:    JSONPatchOpReplace,
			Path:  "/",
			Value: to,
		},
	}
}

// createDetailedTransformPatches creates detailed patches for incremental updates.
func createDetailedTransformPatches(from, to map[string]interface{}) JSONPatch {
	patches := JSONPatch{}

	// Remove keys that don't exist in target
	for key := range from {
		if _, exists := to[key]; !exists {
			patches = append(patches, JSONPatchOperation{
				Op:   JSONPatchOpRemove,
				Path: "/" + key,
			})
		}
	}

	// Add or update keys from target
	for key, value := range to {
		if existingValue, exists := from[key]; exists {
			if !reflect.DeepEqual(existingValue, value) {
				patches = append(patches, JSONPatchOperation{
					Op:    JSONPatchOpReplace,
					Path:  "/" + key,
					Value: value,
				})
			}
		} else {
			patches = append(patches, JSONPatchOperation{
				Op:    JSONPatchOpAdd,
				Path:  "/" + key,
				Value: value,
			})
		}
	}

	return patches
}
