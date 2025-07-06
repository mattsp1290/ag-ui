package state

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// StateEventHandler handles state-related events from the AG-UI protocol
type StateEventHandler struct {
	store         *StateStore
	deltaComputer *DeltaComputer
	metrics       *StateMetrics
	mu            sync.RWMutex
	
	// Event processing configuration
	batchSize     int
	batchTimeout  time.Duration
	pendingDeltas []events.JSONPatchOperation
	batchTimer    *time.Timer
	
	// Callbacks for state changes
	onSnapshot    func(*events.StateSnapshotEvent) error
	onDelta       func(*events.StateDeltaEvent) error
	onStateChange func(StateChange)
}

// StateEventHandlerOption is a configuration option for StateEventHandler
type StateEventHandlerOption func(*StateEventHandler)

// WithBatchSize sets the batch size for delta events
func WithBatchSize(size int) StateEventHandlerOption {
	return func(h *StateEventHandler) {
		h.batchSize = size
	}
}

// WithBatchTimeout sets the timeout for batching delta events
func WithBatchTimeout(timeout time.Duration) StateEventHandlerOption {
	return func(h *StateEventHandler) {
		h.batchTimeout = timeout
	}
}

// WithSnapshotCallback sets the callback for snapshot events
func WithSnapshotCallback(fn func(*events.StateSnapshotEvent) error) StateEventHandlerOption {
	return func(h *StateEventHandler) {
		h.onSnapshot = fn
	}
}

// WithDeltaCallback sets the callback for delta events
func WithDeltaCallback(fn func(*events.StateDeltaEvent) error) StateEventHandlerOption {
	return func(h *StateEventHandler) {
		h.onDelta = fn
	}
}

// WithStateChangeCallback sets the callback for state changes
func WithStateChangeCallback(fn func(StateChange)) StateEventHandlerOption {
	return func(h *StateEventHandler) {
		h.onStateChange = fn
	}
}

// NewStateEventHandler creates a new state event handler
func NewStateEventHandler(store *StateStore, options ...StateEventHandlerOption) *StateEventHandler {
	handler := &StateEventHandler{
		store:         store,
		deltaComputer: NewDeltaComputer(DefaultDeltaOptions()),
		metrics:       NewStateMetrics(),
		batchSize:     100,
		batchTimeout:  100 * time.Millisecond,
		pendingDeltas: make([]events.JSONPatchOperation, 0),
	}
	
	// Apply options
	for _, opt := range options {
		opt(handler)
	}
	
	// Subscribe to state changes if callback is set
	if handler.onStateChange != nil {
		store.Subscribe("/", handler.onStateChange)
	}
	
	return handler
}

// HandleStateSnapshot processes a state snapshot event
func (h *StateEventHandler) HandleStateSnapshot(event *events.StateSnapshotEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Start metrics
	startTime := time.Now()
	defer func() {
		h.metrics.RecordEventProcessing("snapshot", time.Since(startTime))
	}()
	
	// Validate event
	if err := h.validateSnapshotEvent(event); err != nil {
		h.metrics.IncrementErrors("snapshot_validation")
		return fmt.Errorf("invalid snapshot event: %w", err)
	}
	
	// Cancel any pending batch processing
	if h.batchTimer != nil {
		h.batchTimer.Stop()
		h.pendingDeltas = h.pendingDeltas[:0]
	}
	
	// Create a state snapshot for backup
	currentSnapshot, err := h.store.CreateSnapshot()
	if err != nil {
		h.metrics.IncrementErrors("snapshot_backup")
		return fmt.Errorf("failed to create backup snapshot: %w", err)
	}
	
	// Apply the snapshot
	if err := h.applySnapshot(event.Snapshot); err != nil {
		// Restore from backup on failure
		if restoreErr := h.store.RestoreSnapshot(currentSnapshot); restoreErr != nil {
			h.metrics.IncrementErrors("snapshot_restore")
			return fmt.Errorf("failed to apply snapshot and restore failed: apply=%w, restore=%w", err, restoreErr)
		}
		h.metrics.IncrementErrors("snapshot_apply")
		return fmt.Errorf("failed to apply snapshot: %w", err)
	}
	
	// Call custom callback if set
	if h.onSnapshot != nil {
		if err := h.onSnapshot(event); err != nil {
			h.metrics.IncrementErrors("snapshot_callback")
			return fmt.Errorf("snapshot callback failed: %w", err)
		}
	}
	
	h.metrics.IncrementEvents("snapshot")
	return nil
}

// HandleStateDelta processes a state delta event
func (h *StateEventHandler) HandleStateDelta(event *events.StateDeltaEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Start metrics
	startTime := time.Now()
	defer func() {
		h.metrics.RecordEventProcessing("delta", time.Since(startTime))
	}()
	
	// Validate event
	if err := h.validateDeltaEvent(event); err != nil {
		h.metrics.IncrementErrors("delta_validation")
		return fmt.Errorf("invalid delta event: %w", err)
	}
	
	// Add to pending deltas for batching
	h.pendingDeltas = append(h.pendingDeltas, event.Delta...)
	
	// Check if we should process the batch
	if len(h.pendingDeltas) >= h.batchSize {
		return h.processPendingDeltas()
	}
	
	// Set up batch timer if not already running
	if h.batchTimer == nil {
		h.batchTimer = time.AfterFunc(h.batchTimeout, func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.processPendingDeltas()
		})
	}
	
	return nil
}

// processPendingDeltas processes all pending delta operations
func (h *StateEventHandler) processPendingDeltas() error {
	if len(h.pendingDeltas) == 0 {
		return nil
	}
	
	// Convert to internal patch format
	patch := make(JSONPatch, len(h.pendingDeltas))
	for i, op := range h.pendingDeltas {
		patch[i] = JSONPatchOperation{
			Op:    JSONPatchOp(op.Op),
			Path:  op.Path,
			Value: op.Value,
			From:  op.From,
		}
	}
	
	// Apply the batch
	if err := h.store.ApplyPatch(patch); err != nil {
		h.metrics.IncrementErrors("delta_apply")
		return fmt.Errorf("failed to apply delta batch: %w", err)
	}
	
	// Call custom callback if set
	if h.onDelta != nil {
		deltaEvent := &events.StateDeltaEvent{
			BaseEvent: events.NewBaseEvent(events.EventTypeStateDelta),
			Delta:     h.pendingDeltas,
		}
		if err := h.onDelta(deltaEvent); err != nil {
			h.metrics.IncrementErrors("delta_callback")
			return fmt.Errorf("delta callback failed: %w", err)
		}
	}
	
	// Clear pending deltas
	h.pendingDeltas = h.pendingDeltas[:0]
	h.batchTimer = nil
	
	h.metrics.IncrementEvents("delta")
	return nil
}

// validateSnapshotEvent validates a snapshot event
func (h *StateEventHandler) validateSnapshotEvent(event *events.StateSnapshotEvent) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}
	
	// Use the event's built-in validation
	return event.Validate()
}

// validateDeltaEvent validates a delta event
func (h *StateEventHandler) validateDeltaEvent(event *events.StateDeltaEvent) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}
	
	// Use the event's built-in validation
	return event.Validate()
}

// applySnapshot applies a snapshot to the state store
func (h *StateEventHandler) applySnapshot(snapshot interface{}) error {
	// Convert snapshot to JSON for normalization
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}
	
	// Import the snapshot into the store
	return h.store.Import(data)
}

// StateEventGenerator generates state events from the state store
type StateEventGenerator struct {
	store         *StateStore
	deltaComputer *DeltaComputer
	lastSnapshot  map[string]interface{}
	mu            sync.RWMutex
}

// NewStateEventGenerator creates a new state event generator
func NewStateEventGenerator(store *StateStore) *StateEventGenerator {
	return &StateEventGenerator{
		store:         store,
		deltaComputer: NewDeltaComputer(DefaultDeltaOptions()),
		lastSnapshot:  make(map[string]interface{}),
	}
}

// GenerateSnapshot generates a state snapshot event
func (g *StateEventGenerator) GenerateSnapshot() (*events.StateSnapshotEvent, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	// Get current state
	state := g.store.GetState()
	
	// Update last snapshot
	g.lastSnapshot = state
	
	// Create snapshot event
	event := events.NewStateSnapshotEvent(state)
	
	return event, nil
}

// GenerateDelta generates a state delta event between old and new states
func (g *StateEventGenerator) GenerateDelta(oldState, newState interface{}) (*events.StateDeltaEvent, error) {
	// Compute delta using the delta computer
	patch, err := g.deltaComputer.ComputeDelta(oldState, newState)
	if err != nil {
		return nil, fmt.Errorf("failed to compute delta: %w", err)
	}
	
	// Convert to event format
	eventOps := make([]events.JSONPatchOperation, len(patch))
	for i, op := range patch {
		eventOps[i] = events.JSONPatchOperation{
			Op:    string(op.Op),
			Path:  op.Path,
			Value: op.Value,
			From:  op.From,
		}
	}
	
	// Create delta event
	event := events.NewStateDeltaEvent(eventOps)
	
	return event, nil
}

// GenerateDeltaFromCurrent generates a delta from the last snapshot to current state
func (g *StateEventGenerator) GenerateDeltaFromCurrent() (*events.StateDeltaEvent, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	// Get current state
	currentState := g.store.GetState()
	
	// Generate delta from last snapshot
	event, err := g.GenerateDelta(g.lastSnapshot, currentState)
	if err != nil {
		return nil, err
	}
	
	// Update last snapshot
	g.lastSnapshot = currentState
	
	return event, nil
}

// StateMetrics tracks metrics for state event processing
type StateMetrics struct {
	mu               sync.RWMutex
	eventsProcessed  map[string]int64
	errors           map[string]int64
	processingTimes  map[string][]time.Duration
	lastUpdate       time.Time
}

// NewStateMetrics creates a new metrics tracker
func NewStateMetrics() *StateMetrics {
	return &StateMetrics{
		eventsProcessed: make(map[string]int64),
		errors:          make(map[string]int64),
		processingTimes: make(map[string][]time.Duration),
		lastUpdate:      time.Now(),
	}
}

// IncrementEvents increments the event counter
func (m *StateMetrics) IncrementEvents(eventType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventsProcessed[eventType]++
	m.lastUpdate = time.Now()
}

// IncrementErrors increments the error counter
func (m *StateMetrics) IncrementErrors(errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[errorType]++
	m.lastUpdate = time.Now()
}

// RecordEventProcessing records the processing time for an event
func (m *StateMetrics) RecordEventProcessing(eventType string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Keep only last 1000 samples
	if len(m.processingTimes[eventType]) >= 1000 {
		m.processingTimes[eventType] = m.processingTimes[eventType][1:]
	}
	
	m.processingTimes[eventType] = append(m.processingTimes[eventType], duration)
	m.lastUpdate = time.Now()
}

// GetStats returns current metrics statistics
func (m *StateMetrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := map[string]interface{}{
		"events_processed": m.eventsProcessed,
		"errors":          m.errors,
		"last_update":     m.lastUpdate,
	}
	
	// Calculate average processing times
	avgTimes := make(map[string]float64)
	for eventType, times := range m.processingTimes {
		if len(times) > 0 {
			var total time.Duration
			for _, t := range times {
				total += t
			}
			avgTimes[eventType] = float64(total) / float64(len(times)) / float64(time.Millisecond)
		}
	}
	stats["avg_processing_times_ms"] = avgTimes
	
	return stats
}

// Reset resets all metrics
func (m *StateMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.eventsProcessed = make(map[string]int64)
	m.errors = make(map[string]int64)
	m.processingTimes = make(map[string][]time.Duration)
	m.lastUpdate = time.Now()
}

// StateEventStream provides real-time streaming of state changes as events
type StateEventStream struct {
	store      *StateStore
	generator  *StateEventGenerator
	handlers   []func(events.Event) error
	mu         sync.RWMutex
	stopCh     chan struct{}
	interval   time.Duration
	deltaOnly  bool
}

// StateEventStreamOption is a configuration option for StateEventStream
type StateEventStreamOption func(*StateEventStream)

// WithStreamInterval sets the interval for generating events
func WithStreamInterval(interval time.Duration) StateEventStreamOption {
	return func(s *StateEventStream) {
		s.interval = interval
	}
}

// WithDeltaOnly configures the stream to only emit delta events
func WithDeltaOnly(deltaOnly bool) StateEventStreamOption {
	return func(s *StateEventStream) {
		s.deltaOnly = deltaOnly
	}
}

// NewStateEventStream creates a new state event stream
func NewStateEventStream(store *StateStore, generator *StateEventGenerator, options ...StateEventStreamOption) *StateEventStream {
	stream := &StateEventStream{
		store:     store,
		generator: generator,
		handlers:  make([]func(events.Event) error, 0),
		stopCh:    make(chan struct{}),
		interval:  100 * time.Millisecond,
		deltaOnly: false,
	}
	
	// Apply options
	for _, opt := range options {
		opt(stream)
	}
	
	return stream
}

// Subscribe adds a handler for state events
func (s *StateEventStream) Subscribe(handler func(events.Event) error) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.handlers = append(s.handlers, handler)
	idx := len(s.handlers) - 1
	
	// Return unsubscribe function
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.handlers = append(s.handlers[:idx], s.handlers[idx+1:]...)
	}
}

// Start begins streaming state changes
func (s *StateEventStream) Start() error {
	// Send initial snapshot unless deltaOnly is set
	if !s.deltaOnly {
		snapshot, err := s.generator.GenerateSnapshot()
		if err != nil {
			return fmt.Errorf("failed to generate initial snapshot: %w", err)
		}
		s.emit(snapshot)
	}
	
	// Start change detection loop
	go s.streamLoop()
	
	return nil
}

// Stop stops the event stream
func (s *StateEventStream) Stop() {
	close(s.stopCh)
}

// streamLoop continuously monitors for state changes
func (s *StateEventStream) streamLoop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	
	for {
		// Check if stream is stopped before processing
		select {
		case <-s.stopCh:
			// Stream stopped, exit immediately
			return
		default:
			// Continue processing
		}

		select {
		case <-ticker.C:
			// Generate delta from last known state
			delta, err := s.generator.GenerateDeltaFromCurrent()
			if err != nil {
				// Log error but continue streaming
				continue
			}
			
			// Only emit if there are actual changes
			if len(delta.Delta) > 0 {
				s.emit(delta)
			}
			
		case <-s.stopCh:
			return
		}
	}
}

// emit sends an event to all subscribers
func (s *StateEventStream) emit(event events.Event) {
	s.mu.RLock()
	handlers := make([]func(events.Event) error, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.RUnlock()
	
	for _, handler := range handlers {
		// Call handlers in separate goroutines to prevent blocking
		go func(h func(events.Event) error) {
			if err := h(event); err != nil {
				// Log error but continue with other handlers
			}
		}(handler)
	}
}