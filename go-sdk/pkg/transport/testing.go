package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// MockTransport is a highly configurable mock implementation of the Transport interface
type MockTransport struct {
	mu sync.RWMutex

	// Configuration
	connectBehavior    func(ctx context.Context) error
	sendBehavior       func(ctx context.Context, event TransportEvent) error
	closeBehavior      func(ctx context.Context) error
	
	// State
	connected      atomic.Bool
	closed         atomic.Bool
	eventChan      chan events.Event
	errorChan      chan error
	stats          TransportStats
	config         Config
	
	// Call tracking
	calls          map[string][]interface{}
	callCount      map[string]int
	
	// Event recording
	sentEvents     []TransportEvent
	receivedEvents []events.Event
}

// NewMockTransport creates a new mock transport with default behavior
func NewMockTransport() *MockTransport {
	return &MockTransport{
		eventChan:  make(chan events.Event, 100),
		errorChan:  make(chan error, 100),
		calls:      make(map[string][]interface{}),
		callCount:  make(map[string]int),
		config: &BaseConfig{
			Type:           "mock",
			Endpoint:       "mock://test",
			Timeout:        30 * time.Second,
			MaxMessageSize: 1024 * 1024,
		},
	}
}

// Connect implements Transport.Connect
func (m *MockTransport) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordCall("Connect", ctx)
	
	if m.connectBehavior != nil {
		if err := m.connectBehavior(ctx); err != nil {
			return err
		}
	}
	
	if m.connected.Load() {
		return ErrAlreadyConnected
	}
	
	m.connected.Store(true)
	m.stats.ConnectedAt = time.Now()
	return nil
}

// Send implements Transport.Send
func (m *MockTransport) Send(ctx context.Context, event TransportEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordCall("Send", ctx, event)
	
	if !m.connected.Load() {
		return ErrNotConnected
	}
	
	if m.sendBehavior != nil {
		return m.sendBehavior(ctx, event)
	}
	
	m.sentEvents = append(m.sentEvents, event)
	m.stats.EventsSent++
	m.stats.LastEventSentAt = time.Now()
	
	return nil
}

// Receive implements Transport.Receive
func (m *MockTransport) Receive() <-chan events.Event {
	return m.eventChan
}

// Errors implements Transport.Errors
func (m *MockTransport) Errors() <-chan error {
	return m.errorChan
}

// Close implements Transport.Close
func (m *MockTransport) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordCall("Close", ctx)
	
	if m.closeBehavior != nil {
		return m.closeBehavior(ctx)
	}
	
	if !m.connected.Load() {
		return nil
	}
	
	// Check if already closed to prevent double-close panic
	if m.closed.CompareAndSwap(false, true) {
		m.connected.Store(false)
		close(m.eventChan)
		close(m.errorChan)
	}
	
	return nil
}

// IsConnected implements Transport.IsConnected
func (m *MockTransport) IsConnected() bool {
	return m.connected.Load()
}

// Config implements Transport.Config
func (m *MockTransport) Config() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Stats implements Transport.Stats
func (m *MockTransport) Stats() TransportStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := m.stats
	if m.connected.Load() && !stats.ConnectedAt.IsZero() {
		stats.Uptime = time.Since(stats.ConnectedAt)
	}
	
	return stats
}

// Test helper methods

// SetConnectBehavior sets custom behavior for Connect calls
func (m *MockTransport) SetConnectBehavior(fn func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectBehavior = fn
}

// SetSendBehavior sets custom behavior for Send calls
func (m *MockTransport) SetSendBehavior(fn func(ctx context.Context, event TransportEvent) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendBehavior = fn
}

// SetCloseBehavior sets custom behavior for Close calls
func (m *MockTransport) SetCloseBehavior(fn func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeBehavior = fn
}

// SimulateEvent simulates receiving an event
func (m *MockTransport) SimulateEvent(event events.Event) error {
	if m.closed.Load() {
		return errors.New("transport is closed")
	}
	
	select {
	case m.eventChan <- event:
		m.mu.Lock()
		m.receivedEvents = append(m.receivedEvents, event)
		m.stats.EventsReceived++
		m.stats.LastEventRecvAt = time.Now()
		m.mu.Unlock()
		return nil
	default:
		return errors.New("event channel full")
	}
}

// SimulateError simulates an error
func (m *MockTransport) SimulateError(err error) error {
	if m.closed.Load() {
		return errors.New("transport is closed")
	}
	
	select {
	case m.errorChan <- err:
		m.mu.Lock()
		m.stats.ErrorCount++
		m.stats.LastError = err
		m.mu.Unlock()
		return nil
	default:
		return errors.New("error channel full")
	}
}

// GetSentEvents returns all events that were sent
func (m *MockTransport) GetSentEvents() []TransportEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	events := make([]TransportEvent, len(m.sentEvents))
	copy(events, m.sentEvents)
	return events
}

// GetCallCount returns the number of times a method was called
func (m *MockTransport) GetCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount[method]
}

// WasCalled returns true if the method was called at least once
func (m *MockTransport) WasCalled(method string) bool {
	return m.GetCallCount(method) > 0
}

// Reset resets the mock state
func (m *MockTransport) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check if channels were closed before resetting the flag
	wasClosed := m.closed.Load()
	
	m.connected.Store(false)
	m.closed.Store(false)
	
	// Create new channels if they were closed
	if wasClosed {
		m.eventChan = make(chan events.Event, 100)
		m.errorChan = make(chan error, 100)
	}
	
	m.calls = make(map[string][]interface{})
	m.callCount = make(map[string]int)
	m.sentEvents = nil
	m.receivedEvents = nil
	m.stats = TransportStats{}
}

func (m *MockTransport) recordCall(method string, args ...interface{}) {
	m.calls[method] = append(m.calls[method], args)
	m.callCount[method]++
}

// MockManager is a mock implementation of a transport manager
type MockManager struct {
	mu sync.RWMutex
	
	transport       Transport
	running         atomic.Bool
	startBehavior   func(ctx context.Context) error
	stopBehavior    func(ctx context.Context) error
	sendBehavior    func(ctx context.Context, event TransportEvent) error
	
	calls           map[string][]interface{}
	sentEvents      []TransportEvent
}

// NewMockManager creates a new mock manager
func NewMockManager() *MockManager {
	return &MockManager{
		calls: make(map[string][]interface{}),
	}
}

// SetTransport sets the transport
func (m *MockManager) SetTransport(transport Transport) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transport = transport
}

// Start starts the manager
func (m *MockManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordCall("Start", ctx)
	
	if m.startBehavior != nil {
		return m.startBehavior(ctx)
	}
	
	if m.running.Load() {
		return ErrAlreadyConnected
	}
	
	m.running.Store(true)
	return nil
}

// Stop stops the manager
func (m *MockManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordCall("Stop", ctx)
	
	if m.stopBehavior != nil {
		return m.stopBehavior(ctx)
	}
	
	m.running.Store(false)
	return nil
}

// Send sends an event
func (m *MockManager) Send(ctx context.Context, event TransportEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordCall("Send", ctx, event)
	
	if m.sendBehavior != nil {
		return m.sendBehavior(ctx, event)
	}
	
	if !m.running.Load() {
		return ErrNotConnected
	}
	
	m.sentEvents = append(m.sentEvents, event)
	return nil
}

// IsRunning returns true if the manager is running
func (m *MockManager) IsRunning() bool {
	return m.running.Load()
}

func (m *MockManager) recordCall(method string, args ...interface{}) {
	m.calls[method] = append(m.calls[method], args)
}

// MockEventHandler is a mock implementation of EventHandler
type MockEventHandler struct {
	mu          sync.Mutex
	handledEvents []events.Event
	behavior      func(ctx context.Context, event events.Event) error
}

// NewMockEventHandler creates a new mock event handler
func NewMockEventHandler() *MockEventHandler {
	return &MockEventHandler{}
}

// Handle handles an event
func (h *MockEventHandler) Handle(ctx context.Context, event events.Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.handledEvents = append(h.handledEvents, event)
	
	if h.behavior != nil {
		return h.behavior(ctx, event)
	}
	
	return nil
}

// GetHandledEvents returns all handled events
func (h *MockEventHandler) GetHandledEvents() []events.Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	events := make([]events.Event, len(h.handledEvents))
	copy(events, h.handledEvents)
	return events
}

// SetBehavior sets custom behavior for handling events
func (h *MockEventHandler) SetBehavior(fn func(ctx context.Context, event events.Event) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.behavior = fn
}

// Test Event Helpers

// TestEvent is a simple implementation of TransportEvent for testing
type TestEvent struct {
	id        string
	eventType string
	timestamp time.Time
	data      map[string]interface{}
}

// NewTestEvent creates a new test event
func NewTestEvent(id, eventType string) *TestEvent {
	return &TestEvent{
		id:        id,
		eventType: eventType,
		timestamp: time.Now(),
		data:      make(map[string]interface{}),
	}
}

// NewTestEventWithData creates a new test event with data
func NewTestEventWithData(id, eventType string, data map[string]interface{}) *TestEvent {
	return &TestEvent{
		id:        id,
		eventType: eventType,
		timestamp: time.Now(),
		data:      data,
	}
}

func (e *TestEvent) ID() string                    { return e.id }
func (e *TestEvent) Type() string                  { return e.eventType }
func (e *TestEvent) Timestamp() time.Time          { return e.timestamp }
func (e *TestEvent) Data() map[string]interface{}  { return e.data }

// Test Data Generators

// GenerateTestEvents generates a slice of test events
func GenerateTestEvents(count int, prefix string) []TransportEvent {
	events := make([]TransportEvent, count)
	for i := 0; i < count; i++ {
		events[i] = NewTestEvent(
			fmt.Sprintf("%s-%d", prefix, i),
			"test.event",
		)
	}
	return events
}

// GenerateTestEventsWithDelay generates test events with a delay between each
func GenerateTestEventsWithDelay(count int, prefix string, delay time.Duration) []TransportEvent {
	events := make([]TransportEvent, count)
	for i := 0; i < count; i++ {
		events[i] = NewTestEvent(
			fmt.Sprintf("%s-%d", prefix, i),
			"test.event",
		)
		if i < count-1 {
			time.Sleep(delay)
		}
	}
	return events
}

// Test Assertion Helpers

// AssertEventReceived asserts that an event is received within the timeout
func AssertEventReceived(t *testing.T, eventChan <-chan events.Event, timeout time.Duration) events.Event {
	t.Helper()
	
	select {
	case event := <-eventChan:
		if event == nil {
			t.Fatal("Received nil event")
		}
		return event
	case <-time.After(timeout):
		t.Fatal("Timeout waiting for event")
		return nil
	}
}

// AssertNoEvent asserts that no event is received within the timeout
func AssertNoEvent(t *testing.T, eventChan <-chan events.Event, timeout time.Duration) {
	t.Helper()
	
	select {
	case event := <-eventChan:
		t.Fatalf("Unexpected event received: %v", event)
	case <-time.After(timeout):
		// Expected
	}
}

// AssertErrorReceived asserts that an error is received within the timeout
func AssertErrorReceived(t *testing.T, errorChan <-chan error, timeout time.Duration) error {
	t.Helper()
	
	select {
	case err := <-errorChan:
		if err == nil {
			t.Fatal("Received nil error")
		}
		return err
	case <-time.After(timeout):
		t.Fatal("Timeout waiting for error")
		return nil
	}
}

// AssertNoError asserts that no error is received within the timeout
func AssertNoError(t *testing.T, errorChan <-chan error, timeout time.Duration) {
	t.Helper()
	
	select {
	case err := <-errorChan:
		t.Fatalf("Unexpected error received: %v", err)
	case <-time.After(timeout):
		// Expected
	}
}

// AssertTransportConnected asserts that a transport is connected
func AssertTransportConnected(t *testing.T, transport Transport) {
	t.Helper()
	
	if !transport.IsConnected() {
		t.Fatal("Transport is not connected")
	}
}

// AssertTransportNotConnected asserts that a transport is not connected
func AssertTransportNotConnected(t *testing.T, transport Transport) {
	t.Helper()
	
	if transport.IsConnected() {
		t.Fatal("Transport is connected when it should not be")
	}
}

// Timeout Helpers

// WithTimeout runs a function with a timeout
func WithTimeout(t *testing.T, timeout time.Duration, fn func(ctx context.Context)) {
	t.Helper()
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	done := make(chan struct{})
	go func() {
		fn(ctx)
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-ctx.Done():
		t.Fatal("Test timed out")
	}
}

// WithTimeoutExpected runs a function with a timeout but does not fail the test on timeout.
// This is useful when testing timeout behavior where a timeout is the expected outcome.
// The function will be called with a context that has the specified timeout.
// The test can check ctx.Err() to verify if the timeout occurred.
func WithTimeoutExpected(t *testing.T, timeout time.Duration, fn func(ctx context.Context)) {
	t.Helper()
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	done := make(chan struct{})
	go func() {
		fn(ctx)
		close(done)
	}()
	
	select {
	case <-done:
		// Function completed (either successfully or with an error)
	case <-ctx.Done():
		// Timeout occurred - this is allowed in this helper
		// Wait a bit more for the function to complete processing the timeout
		select {
		case <-done:
			// Function finished processing
		case <-time.After(100 * time.Millisecond):
			// Give up waiting
		}
	}
}

// WaitForCondition waits for a condition to be true
func WaitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	
	t.Fatal("Condition not met within timeout")
}

// Error Simulation Utilities

// ErrorSimulator provides utilities for simulating various error conditions
type ErrorSimulator struct {
	mu              sync.RWMutex
	errorPatterns   map[string]error
	errorFrequency  map[string]int
	callCounts      map[string]int
}

// NewErrorSimulator creates a new error simulator
func NewErrorSimulator() *ErrorSimulator {
	return &ErrorSimulator{
		errorPatterns:  make(map[string]error),
		errorFrequency: make(map[string]int),
		callCounts:     make(map[string]int),
	}
}

// SetError sets an error to be returned for a specific operation
func (s *ErrorSimulator) SetError(operation string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorPatterns[operation] = err
}

// SetErrorFrequency sets how often an error should occur (every N calls)
func (s *ErrorSimulator) SetErrorFrequency(operation string, frequency int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorFrequency[operation] = frequency
}

// ShouldError returns whether an error should be simulated for this call
func (s *ErrorSimulator) ShouldError(operation string) (error, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.callCounts[operation]++
	
	if err, ok := s.errorPatterns[operation]; ok {
		if freq, hasFreq := s.errorFrequency[operation]; hasFreq {
			if s.callCounts[operation]%freq == 0 {
				return err, true
			}
		} else {
			return err, true
		}
	}
	
	return nil, false
}

// Test Fixtures

// TestConfig provides common test configurations
type TestConfig struct {
	DefaultTimeout     time.Duration
	EventChannelSize   int
	ErrorChannelSize   int
	MaxMessageSize     int
}

// DefaultTestConfig returns the default test configuration
func DefaultTestConfig() TestConfig {
	return TestConfig{
		DefaultTimeout:   100 * time.Millisecond,
		EventChannelSize: 100,
		ErrorChannelSize: 100,
		MaxMessageSize:   1024 * 1024,
	}
}

// TestFixture encapsulates common test setup
type TestFixture struct {
	Transport *MockTransport
	Manager   *MockManager
	Handler   *MockEventHandler
	Config    TestConfig
	Ctx       context.Context
	Cancel    context.CancelFunc
}

// NewTestFixture creates a new test fixture
func NewTestFixture(t *testing.T) *TestFixture {
	ctx, cancel := context.WithCancel(context.Background())
	
	fixture := &TestFixture{
		Transport: NewMockTransport(),
		Manager:   NewMockManager(),
		Handler:   NewMockEventHandler(),
		Config:    DefaultTestConfig(),
		Ctx:       ctx,
		Cancel:    cancel,
	}
	
	// Wire up the manager and transport
	fixture.Manager.SetTransport(fixture.Transport)
	
	// Cleanup
	t.Cleanup(func() {
		cancel()
	})
	
	return fixture
}

// ConnectTransport connects the transport with error handling
func (f *TestFixture) ConnectTransport(t *testing.T) {
	t.Helper()
	
	ctx, cancel := context.WithTimeout(f.Ctx, f.Config.DefaultTimeout)
	defer cancel()
	
	if err := f.Transport.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect transport: %v", err)
	}
}

// StartManager starts the manager with error handling
func (f *TestFixture) StartManager(t *testing.T) {
	t.Helper()
	
	ctx, cancel := context.WithTimeout(f.Ctx, f.Config.DefaultTimeout)
	defer cancel()
	
	if err := f.Manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
}

// SendEvent sends an event through the transport
func (f *TestFixture) SendEvent(t *testing.T, event TransportEvent) {
	t.Helper()
	
	ctx, cancel := context.WithTimeout(f.Ctx, f.Config.DefaultTimeout)
	defer cancel()
	
	if err := f.Transport.Send(ctx, event); err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}
}

// Concurrent Testing Helpers

// ConcurrentTest helps with concurrent testing scenarios
type ConcurrentTest struct {
	wg        sync.WaitGroup
	errors    chan error
	done      chan struct{}
}

// NewConcurrentTest creates a new concurrent test helper
func NewConcurrentTest() *ConcurrentTest {
	return &ConcurrentTest{
		errors: make(chan error, 100),
		done:   make(chan struct{}),
	}
}

// Run runs a function concurrently N times
func (ct *ConcurrentTest) Run(count int, fn func(id int) error) {
	ct.wg.Add(count)
	
	for i := 0; i < count; i++ {
		go func(id int) {
			defer ct.wg.Done()
			
			if err := fn(id); err != nil {
				select {
				case ct.errors <- err:
				default:
					// Error channel full
				}
			}
		}(i)
	}
}

// Wait waits for all concurrent operations to complete
func (ct *ConcurrentTest) Wait() []error {
	ct.wg.Wait()
	close(ct.done)
	close(ct.errors)
	
	var errs []error
	for err := range ct.errors {
		errs = append(errs, err)
	}
	
	return errs
}

// Benchmark Helpers

// BenchmarkTransport runs standard transport benchmarks
func BenchmarkTransport(b *testing.B, transport Transport) {
	ctx := context.Background()
	
	// Connect
	if err := transport.Connect(ctx); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer transport.Close(ctx)
	
	// Create test event
	event := NewTestEvent("bench-1", "benchmark")
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		if err := transport.Send(ctx, event); err != nil {
			b.Fatalf("Failed to send: %v", err)
		}
	}
}

// BenchmarkConcurrentSend benchmarks concurrent send operations
func BenchmarkConcurrentSend(b *testing.B, transport Transport, concurrency int) {
	ctx := context.Background()
	
	// Connect
	if err := transport.Connect(ctx); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer transport.Close(ctx)
	
	b.ResetTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		event := NewTestEvent("bench-concurrent", "benchmark")
		for pb.Next() {
			if err := transport.Send(ctx, event); err != nil {
				b.Fatalf("Failed to send: %v", err)
			}
		}
	})
}