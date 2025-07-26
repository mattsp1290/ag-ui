package common

import (
	"context"
	"testing"
	"time"
)

// TestTimeouts provides standardized timeout configurations for tests
type TestTimeouts struct {
	// Connection timeouts
	Connect    time.Duration
	Disconnect time.Duration
	
	// Operation timeouts
	Send     time.Duration
	Receive  time.Duration
	Process  time.Duration
	
	// Test-specific timeouts
	Unit        time.Duration
	Integration time.Duration
	Load        time.Duration
	
	// Event timeouts
	EventWait     time.Duration
	EventBatch    time.Duration
	EventSequence time.Duration
}

// DefaultTestTimeouts returns optimized timeouts for unit tests
func DefaultTestTimeouts() TestTimeouts {
	return TestTimeouts{
		// Connection timeouts
		Connect:    2 * time.Second,
		Disconnect: 1 * time.Second,
		
		// Operation timeouts
		Send:    3 * time.Second,  // Increased for HTTP request reliability
		Receive: 3 * time.Second,
		Process: 2 * time.Second,
		
		// Test-specific timeouts
		Unit:        5 * time.Second,
		Integration: 15 * time.Second,
		Load:        30 * time.Second,
		
		// Event timeouts
		EventWait:     500 * time.Millisecond,
		EventBatch:    300 * time.Millisecond,
		EventSequence: 1 * time.Second,
	}
}

// IntegrationTestTimeouts returns timeouts optimized for integration tests
func IntegrationTestTimeouts() TestTimeouts {
	timeouts := DefaultTestTimeouts()
	// Increase timeouts for integration tests which may involve network I/O
	timeouts.Connect = 5 * time.Second
	timeouts.Receive = 8 * time.Second
	timeouts.Process = 5 * time.Second
	timeouts.Integration = 30 * time.Second
	timeouts.EventWait = 1 * time.Second
	return timeouts
}

// LoadTestTimeouts returns timeouts optimized for load tests
func LoadTestTimeouts() TestTimeouts {
	timeouts := DefaultTestTimeouts()
	// Significantly increase timeouts for load tests
	timeouts.Connect = 10 * time.Second
	timeouts.Receive = 15 * time.Second
	timeouts.Process = 10 * time.Second
	timeouts.Load = 60 * time.Second
	timeouts.EventWait = 2 * time.Second
	return timeouts
}

// TestHelper provides common test utilities with consistent timeout handling
type TestHelper struct {
	t        *testing.T
	timeouts TestTimeouts
}

// NewTestHelper creates a new test helper with default timeouts
func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{
		t:        t,
		timeouts: DefaultTestTimeouts(),
	}
}

// NewIntegrationTestHelper creates a test helper optimized for integration tests
func NewIntegrationTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{
		t:        t,
		timeouts: IntegrationTestTimeouts(),
	}
}

// NewLoadTestHelper creates a test helper optimized for load tests
func NewLoadTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{
		t:        t,
		timeouts: LoadTestTimeouts(),
	}
}

// WithCustomTimeouts creates a test helper with custom timeouts
func (th *TestHelper) WithCustomTimeouts(timeouts TestTimeouts) *TestHelper {
	return &TestHelper{
		t:        th.t,
		timeouts: timeouts,
	}
}

// ConnectContext creates a context with connect timeout
func (th *TestHelper) ConnectContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), th.timeouts.Connect)
}

// SendContext creates a context with send timeout
func (th *TestHelper) SendContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), th.timeouts.Send)
}

// ReceiveContext creates a context with receive timeout
func (th *TestHelper) ReceiveContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), th.timeouts.Receive)
}

// ProcessContext creates a context with process timeout
func (th *TestHelper) ProcessContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), th.timeouts.Process)
}

// TestContext creates a context with test-appropriate timeout
func (th *TestHelper) TestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), th.timeouts.Unit)
}

// IntegrationContext creates a context with integration test timeout
func (th *TestHelper) IntegrationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), th.timeouts.Integration)
}

// LoadContext creates a context with load test timeout
func (th *TestHelper) LoadContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), th.timeouts.Load)
}

// WaitForEvent waits for an event with appropriate timeout
func (th *TestHelper) WaitForEvent(eventChan <-chan interface{}) interface{} {
	select {
	case event := <-eventChan:
		return event
	case <-time.After(th.timeouts.EventWait):
		th.t.Fatalf("Timeout waiting for event after %v", th.timeouts.EventWait)
		return nil
	}
}

// WaitForEventWithTimeout waits for an event with custom timeout
func (th *TestHelper) WaitForEventWithTimeout(eventChan <-chan interface{}, timeout time.Duration) interface{} {
	select {
	case event := <-eventChan:
		return event
	case <-time.After(timeout):
		th.t.Fatalf("Timeout waiting for event after %v", timeout)
		return nil
	}
}

// WaitForEvents waits for multiple events with batch timeout
func (th *TestHelper) WaitForEvents(eventChan <-chan interface{}, count int) []interface{} {
	events := make([]interface{}, 0, count)
	timeout := time.After(th.timeouts.EventBatch * time.Duration(count))
	
	for i := 0; i < count; i++ {
		select {
		case event := <-eventChan:
			events = append(events, event)
		case <-timeout:
			th.t.Fatalf("Timeout waiting for event %d/%d after %v", i+1, count, th.timeouts.EventBatch*time.Duration(count))
			return events
		}
	}
	
	return events
}

// WaitForEventSequence waits for a sequence of events with appropriate timeout
func (th *TestHelper) WaitForEventSequence(eventChan <-chan interface{}, count int) []interface{} {
	events := make([]interface{}, 0, count)
	deadline := time.After(th.timeouts.EventSequence)
	
	for i := 0; i < count; i++ {
		select {
		case event := <-eventChan:
			events = append(events, event)
		case <-deadline:
			th.t.Fatalf("Timeout waiting for event sequence after %v (received %d/%d events)", 
				th.timeouts.EventSequence, len(events), count)
			return events
		}
	}
	
	return events
}

// ExpectEvent waits for an event and fails the test if timeout occurs
func (th *TestHelper) ExpectEvent(eventChan <-chan interface{}, description string) interface{} {
	select {
	case event := <-eventChan:
		return event
	case <-time.After(th.timeouts.EventWait):
		th.t.Fatalf("Expected %s but timed out after %v", description, th.timeouts.EventWait)
		return nil
	}
}

// ExpectNoEvent waits for a short period and fails if an event is received
func (th *TestHelper) ExpectNoEvent(eventChan <-chan interface{}, description string) {
	select {
	case event := <-eventChan:
		th.t.Fatalf("Expected no %s but received: %v", description, event)
	case <-time.After(100 * time.Millisecond): // Short wait to ensure no events
		// Expected behavior - no event received
	}
}

// WaitWithCleanup waits for an operation to complete with cleanup on timeout
func (th *TestHelper) WaitWithCleanup(operation func() error, cleanup func(), timeout time.Duration, description string) {
	done := make(chan error, 1)
	
	go func() {
		done <- operation()
	}()
	
	select {
	case err := <-done:
		if err != nil {
			th.t.Fatalf("%s failed: %v", description, err)
		}
	case <-time.After(timeout):
		if cleanup != nil {
			cleanup()
		}
		th.t.Fatalf("%s timed out after %v", description, timeout)
	}
}

// AssertEventually asserts that a condition becomes true within the timeout
func (th *TestHelper) AssertEventually(condition func() bool, timeout time.Duration, interval time.Duration, description string) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		if condition() {
			return
		}
		
		select {
		case <-ticker.C:
			continue
		case <-deadline:
			th.t.Fatalf("Condition '%s' was not met within %v", description, timeout)
		}
	}
}

// RetrySoon retries an operation with a short delay
func (th *TestHelper) RetrySoon(operation func() bool, description string) {
	th.AssertEventually(operation, th.timeouts.EventWait, 10*time.Millisecond, description)
}

// RetryMedium retries an operation with medium intervals
func (th *TestHelper) RetryMedium(operation func() bool, description string) {
	th.AssertEventually(operation, th.timeouts.Process, 50*time.Millisecond, description)
}

// RetryLong retries an operation with longer intervals
func (th *TestHelper) RetryLong(operation func() bool, description string) {
	th.AssertEventually(operation, th.timeouts.Integration, 100*time.Millisecond, description)
}

// Cleanup runs cleanup functions with a timeout
func (th *TestHelper) Cleanup(cleanup func()) {
	done := make(chan struct{})
	
	go func() {
		defer close(done)
		cleanup()
	}()
	
	select {
	case <-done:
		// Cleanup completed
	case <-time.After(th.timeouts.Disconnect):
		th.t.Logf("Cleanup took longer than %v", th.timeouts.Disconnect)
	}
}

// SleepShort sleeps for a short, consistent duration
func (th *TestHelper) SleepShort() {
	time.Sleep(50 * time.Millisecond)
}

// SleepMedium sleeps for a medium, consistent duration
func (th *TestHelper) SleepMedium() {
	time.Sleep(100 * time.Millisecond)
}

// SleepLong sleeps for a longer, consistent duration
func (th *TestHelper) SleepLong() {
	time.Sleep(200 * time.Millisecond)
}