package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

)

// AdvancedMockTransport provides advanced testing capabilities with state machine,
// network simulation, and detailed behavior control
type AdvancedMockTransport struct {
	*MockTransport
	
	// State machine
	state           atomic.Value // ConnectionState
	stateHistory    []ConnectionState
	stateCallbacks  []ConnectionHandler
	
	// Network simulation
	latency         time.Duration
	jitter          time.Duration
	packetLoss      float64
	bandwidth       int64 // bytes per second
	
	// Behavior control
	middleware      []Middleware
	eventFilter     EventFilter
	serializer      Serializer
	compressor      Compressor
	
	// Metrics
	metrics         *TransportMetrics
	healthChecker   *MockHealthChecker
}

// NewAdvancedMockTransport creates a new advanced mock transport
func NewAdvancedMockTransport() *AdvancedMockTransport {
	amt := &AdvancedMockTransport{
		MockTransport: NewMockTransport(),
		metrics:       NewTransportMetrics(),
		healthChecker: NewMockHealthChecker(),
	}
	amt.state.Store(StateDisconnected)
	return amt
}

// Connect with state machine support
func (t *AdvancedMockTransport) Connect(ctx context.Context) error {
	t.setState(StateConnecting)
	
	// Simulate network latency
	if t.latency > 0 {
		select {
		case <-time.After(t.latency + t.getJitter()):
		case <-ctx.Done():
			t.setState(StateDisconnected)
			return ctx.Err()
		}
	}
	
	err := t.MockTransport.Connect(ctx)
	if err != nil {
		t.setState(StateError)
		return err
	}
	
	t.setState(StateConnected)
	return nil
}

// Send with network simulation
func (t *AdvancedMockTransport) Send(ctx context.Context, event TransportEvent) error {
	// Simulate packet loss
	if t.shouldDropPacket() {
		t.metrics.RecordDroppedEvent()
		return errors.New("packet dropped due to network simulation")
	}
	
	// Apply middleware
	processedEvent := event
	for _, mw := range t.middleware {
		var err error
		processedEvent, err = mw.ProcessOutgoing(ctx, processedEvent)
		if err != nil {
			return err
		}
	}
	
	// Simulate bandwidth limitation
	if t.bandwidth > 0 {
		size := t.estimateEventSize(processedEvent)
		delay := time.Duration(float64(size) / float64(t.bandwidth) * float64(time.Second))
		
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	// Simulate network latency
	if t.latency > 0 {
		select {
		case <-time.After(t.latency + t.getJitter()):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	return t.MockTransport.Send(ctx, processedEvent)
}

// Close with state machine support
func (t *AdvancedMockTransport) Close(ctx context.Context) error {
	t.setState(StateClosing)
	
	err := t.MockTransport.Close(ctx)
	if err != nil {
		t.setState(StateError)
		return err
	}
	
	t.setState(StateClosed)
	return nil
}

// GetState returns the current connection state
func (t *AdvancedMockTransport) GetState() ConnectionState {
	return t.state.Load().(ConnectionState)
}

// SetNetworkConditions configures network simulation parameters
func (t *AdvancedMockTransport) SetNetworkConditions(latency, jitter time.Duration, packetLoss float64, bandwidth int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.latency = latency
	t.jitter = jitter
	t.packetLoss = packetLoss
	t.bandwidth = bandwidth
}

// AddMiddleware adds middleware to the transport
func (t *AdvancedMockTransport) AddMiddleware(mw Middleware) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.middleware = append(t.middleware, mw)
}

// SetEventFilter sets an event filter
func (t *AdvancedMockTransport) SetEventFilter(filter EventFilter) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.eventFilter = filter
}

func (t *AdvancedMockTransport) setState(state ConnectionState) {
	oldState := t.GetState()
	t.state.Store(state)
	
	t.mu.Lock()
	t.stateHistory = append(t.stateHistory, state)
	callbacks := append([]ConnectionHandler{}, t.stateCallbacks...)
	t.mu.Unlock()
	
	// Notify callbacks
	var err error
	if state == StateError {
		err = errors.New("connection error")
	}
	
	for _, cb := range callbacks {
		cb(state, err)
	}
	
	// Update metrics
	if oldState != state {
		t.metrics.RecordStateChange(oldState, state)
	}
}

func (t *AdvancedMockTransport) shouldDropPacket() bool {
	if t.packetLoss <= 0 {
		return false
	}
	
	// Use a simple counter-based approach for consistent packet loss
	// Every Nth packet is dropped where N = 1/packetLoss
	if t.packetLoss >= 1.0 {
		return true // Drop all packets if loss is 100% or more
	}
	
	// Initialize metrics if needed (no lock required for atomic operations)
	if t.metrics == nil {
		t.mu.Lock()
		if t.metrics == nil {
			t.metrics = NewTransportMetrics()
		}
		t.mu.Unlock()
	}
	
	// Use atomic counter for thread safety (no lock required)
	packetCount := atomic.AddInt64(&t.metrics.eventsSent, 1)
	dropInterval := int64(1.0 / t.packetLoss)
	return (packetCount % dropInterval) == 0
}

func (t *AdvancedMockTransport) getJitter() time.Duration {
	if t.jitter == 0 {
		return 0
	}
	// Simple jitter simulation: +/- 50% of configured jitter
	base := int64(t.jitter)
	variance := base / 2
	jitterNs := base - variance + (time.Now().UnixNano() % (2 * variance))
	return time.Duration(jitterNs)
}

func (t *AdvancedMockTransport) estimateEventSize(event TransportEvent) int64 {
	// Simple size estimation
	size := int64(len(event.ID()) + len(event.Type()))
	
	if data := event.Data(); data != nil {
		for k, v := range data {
			size += int64(len(k))
			if str, ok := v.(string); ok {
				size += int64(len(str))
			} else {
				size += 64 // Rough estimate for other types
			}
		}
	}
	
	return size
}

// TransportMetrics tracks detailed transport metrics
type TransportMetrics struct {
	mu sync.RWMutex
	
	// Event metrics - cache line padded to prevent false sharing
	eventsSent        int64
	_                 [56]byte // Cache line padding
	eventsReceived    int64
	_                 [56]byte // Cache line padding
	eventsDropped     int64
	_                 [56]byte // Cache line padding
	eventsFiltered    int64
	_                 [56]byte // Cache line padding
	
	// Byte metrics - cache line padded to prevent false sharing
	bytesSent         int64
	_                 [56]byte // Cache line padding
	bytesReceived     int64
	_                 [56]byte // Cache line padding
	
	// Latency metrics
	latencySamples    []time.Duration
	minLatency        time.Duration
	maxLatency        time.Duration
	
	// State metrics
	stateChanges      map[string]int64
	connectionTime    time.Duration
	lastConnectedAt   time.Time
	
	// Error metrics
	errorsByType      map[string]int64
}

// NewTransportMetrics creates new transport metrics
func NewTransportMetrics() *TransportMetrics {
	return &TransportMetrics{
		stateChanges: make(map[string]int64),
		errorsByType: make(map[string]int64),
	}
}

// RecordDroppedEvent records a dropped event
func (m *TransportMetrics) RecordDroppedEvent() {
	atomic.AddInt64(&m.eventsDropped, 1)
}

// RecordStateChange records a state transition
func (m *TransportMetrics) RecordStateChange(from, to ConnectionState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := fmt.Sprintf("%s->%s", from, to)
	m.stateChanges[key]++
	
	if to == StateConnected {
		m.lastConnectedAt = time.Now()
	} else if from == StateConnected && !m.lastConnectedAt.IsZero() {
		m.connectionTime += time.Since(m.lastConnectedAt)
	}
}

// GetSummary returns a metrics summary
func (m *TransportMetrics) GetSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	summary := map[string]interface{}{
		"events_sent":     atomic.LoadInt64(&m.eventsSent),
		"events_received": atomic.LoadInt64(&m.eventsReceived),
		"events_dropped":  atomic.LoadInt64(&m.eventsDropped),
		"events_filtered": atomic.LoadInt64(&m.eventsFiltered),
		"bytes_sent":      atomic.LoadInt64(&m.bytesSent),
		"bytes_received":  atomic.LoadInt64(&m.bytesReceived),
		"state_changes":   m.stateChanges,
		"errors_by_type":  m.errorsByType,
	}
	
	if len(m.latencySamples) > 0 {
		var total time.Duration
		for _, l := range m.latencySamples {
			total += l
		}
		summary["avg_latency"] = total / time.Duration(len(m.latencySamples))
		summary["min_latency"] = m.minLatency
		summary["max_latency"] = m.maxLatency
	}
	
	return summary
}

// MockHealthChecker provides mock health checking functionality
type MockHealthChecker struct {
	mu         sync.RWMutex
	healthy    bool
	lastCheck  time.Time
	checkError error
	metadata   map[string]any
}

// NewMockHealthChecker creates a new mock health checker
func NewMockHealthChecker() *MockHealthChecker {
	return &MockHealthChecker{
		healthy:  true,
		metadata: make(map[string]any),
	}
}

// CheckHealth performs a health check
func (h *MockHealthChecker) CheckHealth(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.lastCheck = time.Now()
	
	if h.checkError != nil {
		h.healthy = false
		return h.checkError
	}
	
	h.healthy = true
	return nil
}

// IsHealthy returns the health status
func (h *MockHealthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.healthy
}

// GetHealthStatus returns detailed health status
func (h *MockHealthChecker) GetHealthStatus() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	status := HealthStatus{
		Healthy:   h.healthy,
		Timestamp: h.lastCheck,
		Metadata:  h.metadata,
	}
	
	if h.checkError != nil {
		status.Error = h.checkError.Error()
	}
	
	return status
}

// SetHealthy sets the health status
func (h *MockHealthChecker) SetHealthy(healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.healthy = healthy
}

// SetCheckError sets the error to return on health checks
func (h *MockHealthChecker) SetCheckError(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkError = err
}

// ScenarioTransport provides pre-configured transport behaviors for common test scenarios
type ScenarioTransport struct {
	*AdvancedMockTransport
	ctx        context.Context
	cancel     context.CancelFunc
	shutdownWG sync.WaitGroup
}

// NewScenarioTransport creates a new scenario transport
func NewScenarioTransport(scenario string) *ScenarioTransport {
	ctx, cancel := context.WithCancel(context.Background())
	st := &ScenarioTransport{
		AdvancedMockTransport: NewAdvancedMockTransport(),
		ctx:                   ctx,
		cancel:                cancel,
	}
	
	switch scenario {
	case "flaky-network":
		st.SetNetworkConditions(
			100*time.Millisecond,  // latency
			50*time.Millisecond,   // jitter
			0.05,                  // 5% packet loss
			1024*1024,            // 1MB/s bandwidth
		)
		
	case "slow-connection":
		st.SetNetworkConditions(
			500*time.Millisecond,  // high latency
			100*time.Millisecond,  // high jitter
			0,                     // no packet loss
			56*1024,              // 56KB/s (dial-up speed)
		)
		
	case "unreliable":
		st.SetNetworkConditions(
			50*time.Millisecond,   // moderate latency
			20*time.Millisecond,   // some jitter
			0.2,                   // 20% packet loss
			1024*1024,            // 1MB/s
		)
		
	case "perfect":
		st.SetNetworkConditions(
			0,                    // no latency
			0,                    // no jitter
			0,                    // no packet loss
			0,                    // unlimited bandwidth
		)
		
	case "disconnecting":
		// Disconnect after 5 sends
		sendCount := 0
		st.SetSendBehavior(func(ctx context.Context, event TransportEvent) error {
			sendCount++
			if sendCount >= 5 {
				st.setState(StateDisconnected)
				return ErrConnectionClosed
			}
			return nil
		})
		
	case "reconnecting":
		// Simulate periodic disconnections with proper lifecycle management
		st.shutdownWG.Add(1)
		go func() {
			defer st.shutdownWG.Done()
			
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			
			for {
				select {
				case <-ticker.C:
					if st.GetState() == StateConnected {
						st.setState(StateReconnecting)
						
						// Use context-aware sleep for reconnection delay
						select {
						case <-time.After(500 * time.Millisecond):
							st.setState(StateConnected)
						case <-st.ctx.Done():
							// Shutdown requested during reconnection delay
							return
						}
					}
				case <-st.ctx.Done():
					// Shutdown requested
					return
				}
			}
		}()
	}
	
	return st
}

// Close shuts down the ScenarioTransport and waits for all goroutines to finish
func (st *ScenarioTransport) Close(ctx context.Context) error {
	// Signal shutdown to all goroutines
	st.cancel()
	
	// Wait for all background goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		st.shutdownWG.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All goroutines finished successfully
	case <-ctx.Done():
		// Timeout while waiting for goroutines - this is a leak warning
		return fmt.Errorf("timeout waiting for scenario transport goroutines to finish: %w", ctx.Err())
	case <-time.After(5 * time.Second):
		// Default timeout to prevent hanging tests
		return fmt.Errorf("scenario transport goroutines failed to finish within 5 seconds")
	}
	
	// Call parent Close method
	return st.AdvancedMockTransport.Close(ctx)
}

// ChaosTransport introduces random failures and delays for chaos testing
type ChaosTransport struct {
	*AdvancedMockTransport
	
	// Chaos configuration
	errorRate      float64
	delayRange     [2]time.Duration
	possibleErrors []error
	
	// Counters for deterministic error simulation
	connectCount int64
	sendCount    int64
}

// NewChaosTransport creates a new chaos transport
func NewChaosTransport(errorRate float64) *ChaosTransport {
	ct := &ChaosTransport{
		AdvancedMockTransport: NewAdvancedMockTransport(),
		errorRate:            errorRate,
		delayRange:           [2]time.Duration{0, 100 * time.Millisecond},
		possibleErrors: []error{
			ErrConnectionClosed,
			ErrTimeout,
			ErrMessageTooLarge,
			errors.New("random chaos error"),
		},
	}
	
	// Don't set send/connect behavior to avoid recursion - override the methods instead
	
	return ct
}

// Connect with chaos behavior
func (ct *ChaosTransport) Connect(ctx context.Context) error {
	return ct.chaosConnect(ctx)
}

// Send with chaos behavior
func (ct *ChaosTransport) Send(ctx context.Context, event TransportEvent) error {
	return ct.chaosSend(ctx, event)
}

func (ct *ChaosTransport) chaosSend(ctx context.Context, event TransportEvent) error {
	// Random delay
	delay := ct.randomDelay()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	// Random error
	if ct.shouldSendError() {
		return ct.randomError()
	}
	
	// If no error, call the base mock transport send method (avoid AdvancedMockTransport to prevent loops)
	return ct.MockTransport.Send(ctx, event)
}

func (ct *ChaosTransport) chaosConnect(ctx context.Context) error {
	// Random delay
	delay := ct.randomDelay()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	// Random error
	if ct.shouldConnectError() {
		return ErrConnectionFailed
	}
	
	// If no error, call the base mock transport connect method (avoid AdvancedMockTransport to prevent loops)
	return ct.MockTransport.Connect(ctx)
}

func (ct *ChaosTransport) shouldConnectError() bool {
	if ct.errorRate <= 0 {
		return false
	}
	if ct.errorRate >= 1.0 {
		return true
	}
	
	// Use deterministic counter-based error simulation for consistent results
	count := atomic.AddInt64(&ct.connectCount, 1)
	errorInterval := int64(1.0 / ct.errorRate)
	return (count % errorInterval) == 0
}

func (ct *ChaosTransport) shouldSendError() bool {
	if ct.errorRate <= 0 {
		return false
	}
	if ct.errorRate >= 1.0 {
		return true
	}
	
	// Use deterministic counter-based error simulation for consistent results
	count := atomic.AddInt64(&ct.sendCount, 1)
	errorInterval := int64(1.0 / ct.errorRate)
	return (count % errorInterval) == 0
}

func (ct *ChaosTransport) randomDelay() time.Duration {
	if ct.delayRange[0] >= ct.delayRange[1] {
		return ct.delayRange[0]
	}
	
	diff := int64(ct.delayRange[1] - ct.delayRange[0])
	delay := int64(ct.delayRange[0]) + (time.Now().UnixNano() % diff)
	return time.Duration(delay)
}

func (ct *ChaosTransport) randomError() error {
	if len(ct.possibleErrors) == 0 {
		return errors.New("chaos error")
	}
	
	idx := int(time.Now().UnixNano() % int64(len(ct.possibleErrors)))
	return ct.possibleErrors[idx]
}

// RecordingTransport records all operations for detailed analysis
type RecordingTransport struct {
	Transport
	
	mu         sync.RWMutex
	operations []Operation
	recording  bool
}

// Operation represents a recorded transport operation
type Operation struct {
	Type      string
	Timestamp time.Time
	Duration  time.Duration
	Args      []interface{}
	Result    interface{}
	Error     error
}

// NewRecordingTransport creates a new recording transport
func NewRecordingTransport(wrapped Transport) *RecordingTransport {
	return &RecordingTransport{
		Transport: wrapped,
		recording: true,
	}
}

// Connect records the connect operation
func (rt *RecordingTransport) Connect(ctx context.Context) error {
	start := time.Now()
	err := rt.Transport.Connect(ctx)
	rt.recordOperation("Connect", start, []interface{}{ctx}, nil, err)
	return err
}

// Send records the send operation
func (rt *RecordingTransport) Send(ctx context.Context, event TransportEvent) error {
	start := time.Now()
	err := rt.Transport.Send(ctx, event)
	rt.recordOperation("Send", start, []interface{}{ctx, event}, nil, err)
	return err
}

// Close records the close operation
func (rt *RecordingTransport) Close(ctx context.Context) error {
	start := time.Now()
	err := rt.Transport.Close(ctx)
	rt.recordOperation("Close", start, []interface{}{ctx}, nil, err)
	return err
}

// GetOperations returns all recorded operations
func (rt *RecordingTransport) GetOperations() []Operation {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	ops := make([]Operation, len(rt.operations))
	copy(ops, rt.operations)
	return ops
}

// Clear clears recorded operations
func (rt *RecordingTransport) Clear() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.operations = nil
}

// StartRecording starts recording operations
func (rt *RecordingTransport) StartRecording() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.recording = true
}

// StopRecording stops recording operations
func (rt *RecordingTransport) StopRecording() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.recording = false
}

func (rt *RecordingTransport) recordOperation(opType string, start time.Time, args []interface{}, result interface{}, err error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	if !rt.recording {
		return
	}
	
	op := Operation{
		Type:      opType,
		Timestamp: start,
		Duration:  time.Since(start),
		Args:      args,
		Result:    result,
		Error:     err,
	}
	
	rt.operations = append(rt.operations, op)
}