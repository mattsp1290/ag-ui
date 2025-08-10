package state

import (
	"encoding/json"
	"sync"
)

// Item represents a simple item in the state
type Item struct {
	ID    string `json:"id"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

// State represents the shared state structure
type State struct {
	Version int64  `json:"version"`
	Counter int    `json:"counter"`
	Items   []Item `json:"items"`
}

// NewState creates a new initial state
func NewState() *State {
	return &State{
		Version: 1,
		Counter: 0,
		Items:   make([]Item, 0),
	}
}

// Clone creates a deep copy of the state
func (s *State) Clone() *State {
	if s == nil {
		return nil
	}

	// Clone items slice
	items := make([]Item, len(s.Items))
	copy(items, s.Items)

	return &State{
		Version: s.Version,
		Counter: s.Counter,
		Items:   items,
	}
}

// ToJSON returns the state as canonical JSON bytes for deterministic comparisons
func (s *State) ToJSON() ([]byte, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// StateSnapshot represents a full state snapshot event
type StateSnapshot struct {
	Type    string `json:"type"`
	Version int64  `json:"version"`
	Data    *State `json:"data"`
}

// NewStateSnapshot creates a new state snapshot event
func NewStateSnapshot(state *State) *StateSnapshot {
	return &StateSnapshot{
		Type:    "STATE_SNAPSHOT",
		Version: state.Version,
		Data:    state,
	}
}

// StateDelta represents a state change using RFC 6902 patch
type StateDelta struct {
	Type    string                   `json:"type"`
	Version int64                    `json:"version"`
	Patch   []map[string]interface{} `json:"patch"`
}

// NewStateDelta creates a new state delta event
func NewStateDelta(version int64, patch []map[string]interface{}) *StateDelta {
	return &StateDelta{
		Type:    "STATE_DELTA",
		Version: version,
		Patch:   patch,
	}
}

// StateEventWrapper is a generic wrapper for both snapshot and delta events
type StateEventWrapper struct {
	mu    sync.RWMutex
	event interface{}
}

// NewStateEventWrapper creates a new wrapper for state events
func NewStateEventWrapper(event interface{}) *StateEventWrapper {
	return &StateEventWrapper{
		event: event,
	}
}

// GetEvent returns the wrapped event in a thread-safe manner
func (w *StateEventWrapper) GetEvent() interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.event
}

// SetEvent sets the wrapped event in a thread-safe manner
func (w *StateEventWrapper) SetEvent(event interface{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.event = event
}