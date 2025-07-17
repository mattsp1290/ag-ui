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
		return nil // Silently drop
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
	// Simple random packet loss simulation
	return time.Now().UnixNano()%100 < int64(t.packetLoss*100)
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
	
	// Event metrics
	eventsSent        int64
	eventsReceived    int64
	eventsDropped     int64
	eventsFiltered    int64
	
	// Byte metrics
	bytesSent         int64
	bytesReceived     int64
	
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
}

// NewScenarioTransport creates a new scenario transport
func NewScenarioTransport(scenario string) *ScenarioTransport {
	st := &ScenarioTransport{
		AdvancedMockTransport: NewAdvancedMockTransport(),
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
		// Simulate periodic disconnections
		go func() {
			for {
				time.Sleep(2 * time.Second)
				if st.GetState() == StateConnected {
					st.setState(StateReconnecting)
					time.Sleep(500 * time.Millisecond)
					st.setState(StateConnected)
				}
			}
		}()
	}
	
	return st
}

// ChaosTransport introduces random failures and delays for chaos testing
type ChaosTransport struct {
	*AdvancedMockTransport
	
	// Chaos configuration
	errorRate      float64
	delayRange     [2]time.Duration
	possibleErrors []error
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
	
	// Override send behavior with chaos
	ct.SetSendBehavior(ct.chaosSend)
	ct.SetConnectBehavior(ct.chaosConnect)
	
	return ct
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
	if ct.shouldError() {
		return ct.randomError()
	}
	
	return nil
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
	if ct.shouldError() {
		return ErrConnectionFailed
	}
	
	return nil
}

func (ct *ChaosTransport) shouldError() bool {
	return time.Now().UnixNano()%100 < int64(ct.errorRate*100)
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