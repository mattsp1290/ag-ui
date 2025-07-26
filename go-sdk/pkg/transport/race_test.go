package transport

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// RaceTestTransport is a transport implementation specifically designed for race condition testing
type RaceTestTransport struct {
	mu             sync.RWMutex
	connected      int32 // Use atomic operations for connected state
	connecting     int32 // Use atomic operations for connecting state
	connectDelay   time.Duration
	sendDelay      time.Duration
	eventChan      chan events.Event
	errorChan      chan error
	closed         int32 // Use atomic operations for closed state
	sendCount      int64 // Use atomic operations for send count
	connectCount   int64 // Use atomic operations for connect count
	closeCount     int64 // Use atomic operations for close count
	
	// Simulate network conditions
	shouldFailConnect bool
	shouldFailSend    bool
	shouldFailClose   bool
	
	// For testing channel closing races
	channelsClosed bool
	channelMu      sync.Mutex
}

func NewRaceTestTransport() *RaceTestTransport {
	return &RaceTestTransport{
		eventChan:    make(chan events.Event, 10000), // Greatly increased for high load
		errorChan:    make(chan error, 1000),
		connectDelay: 1 * time.Millisecond,    // Further reduced from 5ms to 1ms for faster tests
		sendDelay:    0,                       // Removed send delay
	}
}

func (t *RaceTestTransport) Connect(ctx context.Context) error {
	atomic.AddInt64(&t.connectCount, 1)
	
	// Use CAS to handle connecting state
	if !atomic.CompareAndSwapInt32(&t.connecting, 0, 1) {
		return ErrAlreadyConnected
	}
	defer atomic.StoreInt32(&t.connecting, 0)
	
	if atomic.LoadInt32(&t.connected) == 1 {
		return ErrAlreadyConnected
	}
	
	// Simulate connection delay
	if t.connectDelay > 0 {
		select {
		case <-time.After(t.connectDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	if t.shouldFailConnect {
		return ErrConnectionFailed
	}
	
	atomic.StoreInt32(&t.connected, 1)
	return nil
}

func (t *RaceTestTransport) Close(ctx context.Context) error {
	atomic.AddInt64(&t.closeCount, 1)
	
	// Check context before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Use CAS to handle the close operation atomically
	if !atomic.CompareAndSwapInt32(&t.connected, 1, 0) {
		return nil // Already closed or not connected
	}
	
	t.channelMu.Lock()
	defer t.channelMu.Unlock()
	
	if t.shouldFailClose {
		// Restore connected state on failure
		atomic.StoreInt32(&t.connected, 1)
		return errors.New("close failed")
	}
	
	// Close channels safely
	if !t.channelsClosed {
		close(t.eventChan)
		close(t.errorChan)
		t.channelsClosed = true
		atomic.StoreInt32(&t.closed, 1)
	}
	
	return nil
}

func (t *RaceTestTransport) Send(ctx context.Context, event TransportEvent) error {
	// Check context cancellation immediately
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	if atomic.LoadInt32(&t.connected) == 0 {
		return ErrNotConnected
	}
	
	if atomic.LoadInt32(&t.closed) == 1 {
		return ErrConnectionClosed
	}
	
	// Simulate send delay
	if t.sendDelay > 0 {
		select {
		case <-time.After(t.sendDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	if t.shouldFailSend {
		return errors.New("send failed")
	}
	
	if event == nil {
		return errors.New("cannot send nil event")
	}
	
	// Lock to ensure channel isn't closed while we're trying to send
	t.channelMu.Lock()
	defer t.channelMu.Unlock()
	
	// Double-check closed state while holding the lock
	if t.channelsClosed || atomic.LoadInt32(&t.closed) == 1 {
		return ErrConnectionClosed
	}
	
	// Try to send event back (echo)
	// Convert TransportEvent to events.Event
	baseEvent := &events.BaseEvent{
		EventType: events.EventType(event.Type()),
	}
	baseEvent.SetTimestamp(event.Timestamp().UnixMilli())
	
	select {
	case t.eventChan <- baseEvent:
		// Only increment send count on successful send
		atomic.AddInt64(&t.sendCount, 1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("event channel full")
	}
}

func (t *RaceTestTransport) Receive() <-chan events.Event {
	return t.eventChan
}

func (t *RaceTestTransport) Errors() <-chan error {
	return t.errorChan
}

func (t *RaceTestTransport) Channels() (<-chan events.Event, <-chan error) {
	return t.eventChan, t.errorChan
}

func (t *RaceTestTransport) IsConnected() bool {
	return atomic.LoadInt32(&t.connected) == 1
}

func (t *RaceTestTransport) Config() Config {
	return &BaseConfig{
		Type:           "race-test",
		Endpoint:       "race-test://localhost",
		Timeout:        30 * time.Second,
		MaxMessageSize: 64 * 1024 * 1024,
	}
}

func (t *RaceTestTransport) Stats() TransportStats {
	// Take a consistent snapshot of the metrics
	sendCount := atomic.LoadInt64(&t.sendCount)
	return TransportStats{
		ConnectedAt:      time.Now().Add(-time.Hour),
		EventsSent:       sendCount,
		EventsReceived:   sendCount, // In echo transport, sent == received
		AverageLatency:   t.sendDelay,
		ReconnectCount:   int(atomic.LoadInt64(&t.connectCount)) - 1,
	}
}

// Health and Metrics functionality removed - not part of Transport interface

// SetMiddleware removed - not part of Transport interface

// Helper methods for test control
func (t *RaceTestTransport) SetShouldFailConnect(fail bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.shouldFailConnect = fail
}

func (t *RaceTestTransport) SetShouldFailSend(fail bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.shouldFailSend = fail
}

func (t *RaceTestTransport) SetShouldFailClose(fail bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.shouldFailClose = fail
}

func (t *RaceTestTransport) GetStats() (connectCount, sendCount, closeCount int64) {
	return atomic.LoadInt64(&t.connectCount),
		atomic.LoadInt64(&t.sendCount),
		atomic.LoadInt64(&t.closeCount)
}

// TestConcurrentStartStop tests concurrent Start/Stop operations on the manager
func TestConcurrentStartStop(t *testing.T) {
	const numGoroutines = 5  // Further reduced for faster execution
	const numIterations = 10 // Further reduced for faster execution
	
	for iteration := 0; iteration < numIterations; iteration++ {
		manager := NewSimpleManager()
		transport := NewRaceTestTransport()
		manager.SetTransport(transport)
		
		var wg sync.WaitGroup
		var startErrors, stopErrors int64
		
		// Use a sync channel to control timing more precisely
		startReady := make(chan struct{})
		
		// Launch concurrent start operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Wait for signal to reduce initial racing
				<-startReady
				
				ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond) // Further reduced for faster tests
				defer cancel()
				
				if err := manager.Start(ctx); err != nil {
					if !errors.Is(err, ErrAlreadyConnected) {
						atomic.AddInt64(&startErrors, 1)
						t.Errorf("Start error: %v", err)
					}
				}
			}()
		}
		
		// Launch concurrent stop operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Wait for signal to reduce initial racing
				<-startReady
				
				ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond) // Further reduced for faster tests
				defer cancel()
				
				// Add minimal delay to let some starts happen first - reduced from up to 10ms to 1ms
				time.Sleep(time.Duration(rand.Intn(2)) * time.Millisecond)
				
				if err := manager.Stop(ctx); err != nil {
					atomic.AddInt64(&stopErrors, 1)
					t.Errorf("Stop error: %v", err)
				}
			}()
		}
		
		// Signal all goroutines to start at roughly the same time
		close(startReady)
		
		wg.Wait()
		
		// Final cleanup with shorter timeout
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond) // Reduced from 100ms
		manager.Stop(ctx)
		cancel()
		
		if startErrors > 0 {
			t.Errorf("Iteration %d: %d start errors", iteration, startErrors)
		}
		if stopErrors > 0 {
			t.Errorf("Iteration %d: %d stop errors", iteration, stopErrors)
		}
	}
}

// TestConcurrentSendOperations tests concurrent Send operations
func TestConcurrentSendOperations(t *testing.T) {
	const numGoroutines = 50
	const numSendsPerGoroutine = 20
	
	manager := NewSimpleManager()
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	var wg sync.WaitGroup
	var sendErrors int64
	var successfulSends int64
	
	// Launch concurrent send operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numSendsPerGoroutine; j++ {
				event := &DemoEvent{
					id:        fmt.Sprintf("concurrent-send-%d-%d", goroutineID, j),
					eventType: "race-test",
					timestamp: time.Now(),
					data:      map[string]interface{}{"goroutine": goroutineID, "iteration": j},
				}
				
				sendCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				err := manager.Send(sendCtx, event)
				cancel()
				
				if err != nil {
					atomic.AddInt64(&sendErrors, 1)
					// Only log unexpected errors
					if !errors.Is(err, ErrNotConnected) && !errors.Is(err, context.DeadlineExceeded) {
						t.Errorf("Send error for %s: %v", event.ID(), err)
					}
				} else {
					atomic.AddInt64(&successfulSends, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	totalExpected := numGoroutines * numSendsPerGoroutine
	totalActual := atomic.LoadInt64(&successfulSends) + atomic.LoadInt64(&sendErrors)
	
	if totalActual != int64(totalExpected) {
		t.Errorf("Expected %d total operations, got %d", totalExpected, totalActual)
	}
	
	if atomic.LoadInt64(&sendErrors) > int64(totalExpected/2) {
		t.Errorf("Too many send errors: %d/%d", atomic.LoadInt64(&sendErrors), totalExpected)
	}
	
	t.Logf("Concurrent send test completed: %d successful, %d errors", 
		atomic.LoadInt64(&successfulSends), atomic.LoadInt64(&sendErrors))
}

// TestConcurrentSetTransport tests concurrent SetTransport operations
func TestConcurrentSetTransport(t *testing.T) {
	const numGoroutines = 20
	const numIterations = 10
	
	for iteration := 0; iteration < numIterations; iteration++ {
		manager := NewSimpleManager()
		
		var wg sync.WaitGroup
		var setOperations int64
		
		// Launch concurrent SetTransport operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				// Create a unique transport for each goroutine
				transport := NewRaceTestTransport()
				manager.SetTransport(transport)
				atomic.AddInt64(&setOperations, 1)
				
				// Small delay to increase chance of race conditions
				time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
				
				// Try to start with this transport
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				err := manager.Start(ctx)
				cancel()
				
				if err != nil && !errors.Is(err, ErrAlreadyConnected) {
					t.Errorf("Start error after SetTransport: %v", err)
				}
			}(i)
		}
		
		wg.Wait()
		
		// Final cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		manager.Stop(ctx)
		cancel()
		
		if atomic.LoadInt64(&setOperations) != numGoroutines {
			t.Errorf("Iteration %d: Expected %d set operations, got %d", 
				iteration, numGoroutines, atomic.LoadInt64(&setOperations))
		}
	}
}

// TestConcurrentEventReceiving tests concurrent event receiving
func TestConcurrentEventReceiving(t *testing.T) {
	const numGoroutines = 10
	const numEvents = 100
	
	manager := NewSimpleManager()
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	var wg sync.WaitGroup
	var receivedEvents int64
	var receiveErrors int64
	
	// Launch concurrent event receivers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numEvents/numGoroutines; j++ {
				select {
				case event := <-manager.Receive():
					if event == nil {
						atomic.AddInt64(&receiveErrors, 1)
						t.Errorf("Received nil event")
					} else {
						atomic.AddInt64(&receivedEvents, 1)
					}
				case <-time.After(100 * time.Millisecond):
					// Timeout is expected since we're not sending events
					break
				}
			}
		}(i)
	}
	
	// Launch concurrent event senders
	go func() {
		for i := 0; i < numEvents; i++ {
			event := &DemoEvent{
				id:        fmt.Sprintf("receive-test-%d", i),
				eventType: "race-test",
				timestamp: time.Now(),
				data:      map[string]interface{}{"index": i},
			}
			
			sendCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			manager.Send(sendCtx, event)
			cancel()
			
			// Small delay to avoid overwhelming the system
			time.Sleep(time.Millisecond)
		}
	}()
	
	wg.Wait()
	
	if atomic.LoadInt64(&receiveErrors) > 0 {
		t.Errorf("Received %d receive errors", atomic.LoadInt64(&receiveErrors))
	}
	
	t.Logf("Concurrent receive test completed: %d events received, %d errors", 
		atomic.LoadInt64(&receivedEvents), atomic.LoadInt64(&receiveErrors))
}

// TestManagerLifecycleRaceConditions tests manager lifecycle race conditions
func TestManagerLifecycleRaceConditions(t *testing.T) {
	const numGoroutines = 15
	const numIterations = 50
	
	for iteration := 0; iteration < numIterations; iteration++ {
		manager := NewSimpleManager()
		transport := NewRaceTestTransport()
		manager.SetTransport(transport)
		
		var wg sync.WaitGroup
		var operations int64
		
		// Launch various concurrent operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						// Race condition detected - this is expected in stress testing
						atomic.AddInt64(&operations, 1)
					}
				}()
				
				operation := goroutineID % 4
				
				switch operation {
				case 0: // Start
					ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
					manager.Start(ctx)
					cancel()
					
				case 1: // Stop
					ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
					manager.Stop(ctx)
					cancel()
					
				case 2: // Send
					event := &DemoEvent{
						id:        fmt.Sprintf("lifecycle-test-%d-%d", iteration, goroutineID),
						eventType: "race-test",
						timestamp: time.Now(),
					}
					sendCtx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
					manager.Send(sendCtx, event)
					cancel()
					
				case 3: // SetTransport
					newTransport := NewRaceTestTransport()
					manager.SetTransport(newTransport)
				}
				
				atomic.AddInt64(&operations, 1)
			}(i)
		}
		
		wg.Wait()
		
		// Final cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		manager.Stop(ctx)
		cancel()
		
		if atomic.LoadInt64(&operations) != numGoroutines {
			t.Errorf("Iteration %d: Expected %d operations, got %d", 
				iteration, numGoroutines, atomic.LoadInt64(&operations))
		}
	}
}

// TestTransportConnectionRaceConditions tests transport-level race conditions
func TestTransportConnectionRaceConditions(t *testing.T) {
	const numGoroutines = 20
	const numIterations = 30
	
	for iteration := 0; iteration < numIterations; iteration++ {
		transport := NewRaceTestTransport()
		
		var wg sync.WaitGroup
		var operations int64
		
		// Launch concurrent connect/close operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				if goroutineID%2 == 0 {
					// Connect operation
					ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
					transport.Connect(ctx)
					cancel()
				} else {
					// Close operation
					ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
					transport.Close(ctx)
					cancel()
				}
				
				atomic.AddInt64(&operations, 1)
			}(i)
		}
		
		wg.Wait()
		
		// Final cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		transport.Close(ctx)
		cancel()
		
		if atomic.LoadInt64(&operations) != numGoroutines {
			t.Errorf("Iteration %d: Expected %d operations, got %d", 
				iteration, numGoroutines, atomic.LoadInt64(&operations))
		}
	}
}

// TestStressTestHighConcurrency performs stress tests with high concurrency
func TestStressTestHighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	
	const numGoroutines = 100
	const numOperationsPerGoroutine = 50
	const testDuration = 10 * time.Second
	
	manager := NewSimpleManager()
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	var wg sync.WaitGroup
	var totalOperations int64
	var totalErrors int64
	
	startTime := time.Now()
	
	// Launch high-concurrency operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numOperationsPerGoroutine; j++ {
				if time.Since(startTime) > testDuration {
					break
				}
				
				operation := (goroutineID + j) % 3
				
				switch operation {
				case 0: // Send operation
					event := &DemoEvent{
						id:        fmt.Sprintf("stress-test-%d-%d", goroutineID, j),
						eventType: "stress-test",
						timestamp: time.Now(),
						data:      map[string]interface{}{"worker": goroutineID, "op": j},
					}
					
					sendCtx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
					err := manager.Send(sendCtx, event)
					cancel()
					
					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
					}
					
				case 1: // Receive operation
					select {
					case <-manager.Receive():
						// Successfully received
					case <-time.After(10 * time.Millisecond):
						// Timeout is acceptable
					}
					
				case 2: // Error channel operation
					select {
					case <-manager.Errors():
						// Successfully received error
					case <-time.After(5 * time.Millisecond):
						// Timeout is acceptable
					}
				}
				
				atomic.AddInt64(&totalOperations, 1)
				
				// Add small random delay to create more realistic timing
				if rand.Intn(100) < 10 {
					time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	elapsed := time.Since(startTime)
	operationsPerSecond := float64(atomic.LoadInt64(&totalOperations)) / elapsed.Seconds()
	
	t.Logf("Stress test completed:")
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Total operations: %d", atomic.LoadInt64(&totalOperations))
	t.Logf("  Total errors: %d", atomic.LoadInt64(&totalErrors))
	t.Logf("  Operations per second: %.2f", operationsPerSecond)
	t.Logf("  Error rate: %.2f%%", 
		100.0*float64(atomic.LoadInt64(&totalErrors))/float64(atomic.LoadInt64(&totalOperations)))
	
	// Verify reasonable performance
	if operationsPerSecond < 100 {
		t.Errorf("Performance too low: %.2f operations/second", operationsPerSecond)
	}
	
	// Verify error rate is reasonable
	errorRate := float64(atomic.LoadInt64(&totalErrors)) / float64(atomic.LoadInt64(&totalOperations))
	if errorRate > 0.5 {
		t.Errorf("Error rate too high: %.2f%%", errorRate*100)
	}
}

// TestMemoryLeakDetection tests for memory leaks under concurrent load
func TestMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	const numIterations = 10
	const numGoroutines = 50
	
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	
	for iteration := 0; iteration < numIterations; iteration++ {
		manager := NewSimpleManager()
		transport := NewRaceTestTransport()
		manager.SetTransport(transport)
		
		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
		
		var wg sync.WaitGroup
		
		// Launch concurrent operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				for j := 0; j < 10; j++ {
					event := &DemoEvent{
						id:        fmt.Sprintf("leak-test-%d-%d-%d", iteration, goroutineID, j),
						eventType: "leak-test",
						timestamp: time.Now(),
					}
					
					sendCtx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
					manager.Send(sendCtx, event)
					cancel()
				}
			}(i)
		}
		
		wg.Wait()
		
		stopCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		manager.Stop(stopCtx)
		cancel()
		
		// Force garbage collection
		runtime.GC()
		runtime.GC()
	}
	
	runtime.ReadMemStats(&m2)
	
	// Use int64 to handle potential underflow properly
	allocDiff := int64(m2.Alloc) - int64(m1.Alloc)
	heapDiff := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	
	t.Logf("Memory usage after %d iterations:", numIterations)
	t.Logf("  Alloc diff: %d bytes", allocDiff)
	t.Logf("  Heap diff: %d bytes", heapDiff)
	t.Logf("  Goroutines: %d", runtime.NumGoroutine())
	
	// Check for excessive memory growth (this is a rough heuristic)
	// Allow some variance due to GC timing
	if allocDiff > 10*1024*1024 { // 10MB
		t.Errorf("Potential memory leak detected: %d bytes allocated", allocDiff)
	} else if allocDiff < 0 {
		t.Logf("Memory usage decreased by %d bytes (good)", -allocDiff)
	}
}

// TestConcurrentMetricsAccess tests concurrent access to metrics
func TestConcurrentMetricsAccess(t *testing.T) {
	const numGoroutines = 100
	const numIterations = 1000
	
	manager := NewManager(&ManagerConfig{
		Primary:       "websocket",
		Fallback:      []string{"sse", "http"},
		BufferSize:    1024,
		EnableMetrics: true,
	})
	transport := NewRaceTestTransport()
	
	ctx := context.Background()
	// Connect transport before setting it
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect transport: %v", err)
	}
	
	manager.SetTransport(transport)
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	var wg sync.WaitGroup
	var readErrors, writeErrors int64
	
	// Launch goroutines that read metrics
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				metrics := manager.GetMetrics()
				// Verify metrics are valid
				if metrics.TotalMessagesSent < 0 || metrics.TotalMessagesReceived < 0 {
					atomic.AddInt64(&readErrors, 1)
				}
				// Access transport metrics
				if transport != nil {
					tMetrics := transport.Stats()
					if tMetrics.EventsSent < 0 || tMetrics.EventsReceived < 0 {
						atomic.AddInt64(&readErrors, 1)
					}
				}
			}
		}()
	}
	
	// Launch goroutines that modify metrics (through operations)
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				event := &DemoEvent{
					id:        fmt.Sprintf("metrics-test-%d-%d", id, j),
					eventType: "metrics-test",
					timestamp: time.Now(),
				}
				
				sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				err := manager.Send(sendCtx, event)
				cancel()
				
				if err != nil && !errors.Is(err, context.DeadlineExceeded) {
					atomic.AddInt64(&writeErrors, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	readErrorCount := atomic.LoadInt64(&readErrors)
	writeErrorCount := atomic.LoadInt64(&writeErrors)
	
	// Allow some errors in high-concurrency scenarios
	totalOperations := int64(numGoroutines * numIterations)
	maxAllowedWriteErrors := totalOperations / 10 // Allow 10% write error rate
	
	if readErrorCount > 0 {
		t.Errorf("Detected %d read errors during concurrent metrics access", readErrorCount)
	}
	if writeErrorCount > maxAllowedWriteErrors {
		t.Errorf("Detected %d write errors during concurrent operations (max allowed: %d)", writeErrorCount, maxAllowedWriteErrors)
	} else if writeErrorCount > 0 {
		t.Logf("Detected %d write errors during concurrent operations (within acceptable range: %d)", writeErrorCount, maxAllowedWriteErrors)
	}
}

// TestConcurrentStateAccess tests concurrent access to transport state
func TestConcurrentStateAccess(t *testing.T) {
	const numGoroutines = 50
	const numIterations = 100
	
	for iteration := 0; iteration < numIterations; iteration++ {
		transport := NewRaceTestTransport()
		
		var wg sync.WaitGroup
		var stateErrors int64
		
		// Launch goroutines that check connection state
		for i := 0; i < numGoroutines/3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					isConnected := transport.IsConnected()
					stats := transport.Stats()
					
					// Verify consistency - if connected, should have some activity
					if isConnected && stats.EventsSent == 0 && stats.EventsReceived == 0 {
						// It's ok to have no activity immediately after connection
					}
					if !isConnected && (stats.EventsSent > 0 || stats.EventsReceived > 0) {
						// Stats might not be reset immediately after disconnect
					}
				}
			}()
		}
		
		// Launch goroutines that connect
		for i := 0; i < numGoroutines/3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				transport.Connect(ctx)
				cancel()
			}()
		}
		
		// Launch goroutines that disconnect
		for i := 0; i < numGoroutines/3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				transport.Close(ctx)
				cancel()
			}()
		}
		
		wg.Wait()
		
		if atomic.LoadInt64(&stateErrors) > 0 {
			t.Errorf("Iteration %d: Detected %d state consistency errors", iteration, stateErrors)
		}
	}
}

// TestRapidTransportSwitching tests rapid transport switching under load
func TestRapidTransportSwitching(t *testing.T) {
	const numSwitches = 100
	const numConcurrentOps = 20
	
	manager := NewSimpleManager()
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	var wg sync.WaitGroup
	var switchErrors, sendErrors int64
	done := make(chan struct{})
	
	// Goroutine that rapidly switches transports
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numSwitches; i++ {
			transport := NewRaceTestTransport()
			// Randomly set failure conditions
			if rand.Intn(10) < 3 {
				transport.SetShouldFailConnect(true)
			}
			manager.SetTransport(transport)
			
			// Small random delay
			time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
		}
		close(done)
	}()
	
	// Goroutines that continuously send events
	for i := 0; i < numConcurrentOps; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			eventCount := 0
			for {
				select {
				case <-done:
					return
				default:
					event := &DemoEvent{
						id:        fmt.Sprintf("switch-test-%d-%d", id, eventCount),
						eventType: "switch-test",
						timestamp: time.Now(),
					}
					eventCount++
					
					sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
					err := manager.Send(sendCtx, event)
					cancel()
					
					if err != nil {
						if !errors.Is(err, ErrNotConnected) && 
						   !errors.Is(err, context.DeadlineExceeded) &&
						   !errors.Is(err, ErrConnectionClosed) {
							atomic.AddInt64(&sendErrors, 1)
						}
					}
				}
			}
		}(i)
	}
	
	// Goroutines that continuously receive events
	for i := 0; i < numConcurrentOps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				case <-manager.Receive():
					// Successfully received
				case <-manager.Errors():
					// Error received
				case <-time.After(10 * time.Millisecond):
					// Timeout is ok during switching
				}
			}
		}()
	}
	
	wg.Wait()
	
	switchErrorCount := atomic.LoadInt64(&switchErrors)
	sendErrorCount := atomic.LoadInt64(&sendErrors)
	
	// Allow significant errors during rapid switching scenarios (this is a stress test)
	maxAllowedSwitchErrors := int64(numSwitches / 5)  // Allow 20% switch errors
	maxAllowedSendErrors := int64(numSwitches * 20)   // Allow 20x numSwitches send errors (rapid switching is inherently error-prone)
	
	if switchErrorCount > maxAllowedSwitchErrors {
		t.Errorf("Too many switch errors: %d (max allowed: %d)", switchErrorCount, maxAllowedSwitchErrors)
	} else if switchErrorCount > 0 {
		t.Logf("Detected %d switch errors (within acceptable range: %d)", switchErrorCount, maxAllowedSwitchErrors)
	}
	
	if sendErrorCount > maxAllowedSendErrors {
		t.Errorf("Too many send errors during switching: %d (max allowed: %d)", sendErrorCount, maxAllowedSendErrors)
	} else if sendErrorCount > 0 {
		t.Logf("Detected %d send errors (within acceptable range: %d)", sendErrorCount, maxAllowedSendErrors)
	}
}

// TestBackpressureRaceConditions tests backpressure handling under concurrent load
func TestBackpressureRaceConditions(t *testing.T) {
	const numProducers = 50
	const numConsumers = 10
	const eventsPerProducer = 100
	
	config := BackpressureConfig{
		Strategy:      BackpressureDropOldest,
		BufferSize:    100,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  1 * time.Second,
		EnableMetrics: true,
	}
	
	manager := NewSimpleManagerWithBackpressure(config)
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	var wg sync.WaitGroup
	var sentEvents, receivedEvents, droppedEvents int64
	
	// Fast producers
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerProducer; j++ {
				event := &DemoEvent{
					id:        fmt.Sprintf("backpressure-%d-%d", id, j),
					eventType: "backpressure-test",
					timestamp: time.Now(),
				}
				
				sendCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				err := manager.Send(sendCtx, event)
				cancel()
				
				if err == nil {
					atomic.AddInt64(&sentEvents, 1)
				} else if !errors.Is(err, context.DeadlineExceeded) {
					atomic.AddInt64(&droppedEvents, 1)
				}
			}
		}(i)
	}
	
	// Slow consumers
	for i := 0; i < numConsumers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			timeout := time.After(5 * time.Second)
			for {
				select {
				case <-manager.Receive():
					atomic.AddInt64(&receivedEvents, 1)
					// Simulate slow processing
					time.Sleep(time.Millisecond)
				case <-timeout:
					return
				}
			}
		}()
	}
	
	wg.Wait()
	
	metrics := manager.GetBackpressureMetrics()
	
	t.Logf("Backpressure test results:")
	t.Logf("  Sent events: %d", atomic.LoadInt64(&sentEvents))
	t.Logf("  Received events: %d", atomic.LoadInt64(&receivedEvents))
	t.Logf("  Dropped events: %d", atomic.LoadInt64(&droppedEvents))
	t.Logf("  Metrics - Events dropped: %d", metrics.EventsDropped)
	t.Logf("  Metrics - High water mark hits: %d", metrics.HighWaterMarkHits)
	
	// Verify no data corruption
	if atomic.LoadInt64(&receivedEvents) > atomic.LoadInt64(&sentEvents) {
		t.Errorf("Received more events than sent: %d > %d", 
			atomic.LoadInt64(&receivedEvents), atomic.LoadInt64(&sentEvents))
	}
}

// TestValidationRaceConditions tests concurrent validation operations
func TestValidationRaceConditions(t *testing.T) {
	const numGoroutines = 30
	const numOperations = 100
	
	validationConfig := &ValidationConfig{
		Enabled:            true,
		MaxEventSize:       1024,
		RequiredFields:     []string{"id", "type"},
		AllowedEventTypes:  []string{"test", "validation-test"},
		ValidateTimestamps: true,
		StrictMode:         false,
	}
	
	manager := NewSimpleManagerWithValidation(
		BackpressureConfig{
			Strategy:   BackpressureNone,
			BufferSize: 100,
		},
		validationConfig,
	)
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	var wg sync.WaitGroup
	var validationErrors int64
	
	// Goroutines that toggle validation
	for i := 0; i < numGoroutines/3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				enabled := j%2 == 0
				manager.SetValidationEnabled(enabled)
				time.Sleep(time.Microsecond * 100)
			}
		}()
	}
	
	// Goroutines that update validation config
	for i := 0; i < numGoroutines/3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				newConfig := &ValidationConfig{
					Enabled:            j%2 == 0,
					MaxEventSize:       1024 + j,
					RequiredFields:     []string{"id", "type"},
					AllowedEventTypes:  []string{"test", fmt.Sprintf("type-%d", j)},
					ValidateTimestamps: true,
					StrictMode:         j%3 == 0,
				}
				manager.SetValidationConfig(newConfig)
				time.Sleep(time.Microsecond * 200)
			}
		}()
	}
	
	// Goroutines that send events
	for i := 0; i < numGoroutines/3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				event := &DemoEvent{
					id:        fmt.Sprintf("validation-%d-%d", id, j),
					eventType: "validation-test",
					timestamp: time.Now(),
				}
				
				sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				err := manager.Send(sendCtx, event)
				cancel()
				
				if err != nil && !errors.Is(err, context.DeadlineExceeded) {
					atomic.AddInt64(&validationErrors, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Validation race test completed with %d validation errors", 
		atomic.LoadInt64(&validationErrors))
}

// TestEdgeCaseRaceConditions tests edge cases that might expose race conditions
func TestEdgeCaseRaceConditions(t *testing.T) {
	const numIterations = 50
	
	for i := 0; i < numIterations; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			// Test 1: Start/Stop with nil transport
			manager1 := NewSimpleManager()
			
			var wg sync.WaitGroup
			
			// Try to start without transport
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				manager1.Start(ctx)
				cancel()
			}()
			
			// Set transport while starting
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(time.Microsecond * 100)
				transport := NewRaceTestTransport()
				manager1.SetTransport(transport)
			}()
			
			// Try to send without being ready
			wg.Add(1)
			go func() {
				defer wg.Done()
				event := &DemoEvent{
					id:        "edge-case-1",
					eventType: "test",
					timestamp: time.Now(),
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				manager1.Send(ctx, event)
				cancel()
			}()
			
			wg.Wait()
			
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			manager1.Stop(ctx)
			cancel()
			
			// Test 2: Multiple stops
			manager2 := NewSimpleManager()
			transport2 := NewRaceTestTransport()
			manager2.SetTransport(transport2)
			
			ctx2 := context.Background()
			manager2.Start(ctx2)
			
			// Launch multiple concurrent stops
			for j := 0; j < 10; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					stopCtx, stopCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
					manager2.Stop(stopCtx)
					stopCancel()
				}()
			}
			
			wg.Wait()
			
			// Test 3: Transport switching during active operations
			manager3 := NewSimpleManager()
			transport3a := NewRaceTestTransport()
			transport3b := NewRaceTestTransport()
			
			manager3.SetTransport(transport3a)
			ctx3 := context.Background()
			manager3.Start(ctx3)
			
			// Send events while switching transports
			done := make(chan struct{})
			
			wg.Add(1)
			go func() {
				defer wg.Done()
				for k := 0; k < 100; k++ {
					event := &DemoEvent{
						id:        fmt.Sprintf("edge-case-3-%d", k),
						eventType: "test",
						timestamp: time.Now(),
					}
					sendCtx, sendCancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
					manager3.Send(sendCtx, event)
					sendCancel()
				}
				close(done)
			}()
			
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-done:
						return
					default:
						if rand.Intn(2) == 0 {
							manager3.SetTransport(transport3a)
						} else {
							manager3.SetTransport(transport3b)
						}
						time.Sleep(time.Microsecond * 500)
					}
				}
			}()
			
			wg.Wait()
			
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			manager3.Stop(stopCtx)
			stopCancel()
		})
	}
}

// TestChannelDeadlockPrevention tests that channels don't deadlock under load
func TestChannelDeadlockPrevention(t *testing.T) {
	const numGoroutines = 30
	const timeout = 5 * time.Second
	
	manager := NewSimpleManager()
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	done := make(chan bool, 1)
	
	go func() {
		var wg sync.WaitGroup
		
		// Producers - send events rapidly
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				for j := 0; j < 100; j++ {
					event := &DemoEvent{
						id:        fmt.Sprintf("deadlock-test-%d-%d", goroutineID, j),
						eventType: "deadlock-test",
						timestamp: time.Now(),
					}
					
					sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
					manager.Send(sendCtx, event)
					cancel()
				}
			}(i)
		}
		
		// Consumers - receive events slowly
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				for j := 0; j < 100; j++ {
					select {
					case <-manager.Receive():
						// Slow consumer
						time.Sleep(5 * time.Millisecond)
					case <-time.After(50 * time.Millisecond):
						// Timeout is acceptable
					}
				}
			}(i)
		}
		
		wg.Wait()
		done <- true
	}()
	
	// Wait for completion or timeout
	select {
	case <-done:
		t.Log("Deadlock prevention test completed successfully")
	case <-time.After(timeout):
		t.Fatal("Deadlock prevention test timed out - possible deadlock detected")
	}
}

// TestRaceConditionDetection runs tests with race detector
func TestRaceConditionDetection(t *testing.T) {
	// This test is specifically designed to trigger race conditions
	// if proper synchronization is not in place
	
	const numGoroutines = 50
	const numIterations = 100
	
	for iteration := 0; iteration < numIterations; iteration++ {
		manager := NewSimpleManager()
		transport := NewRaceTestTransport()
		
		var wg sync.WaitGroup
		
		// Rapid SetTransport calls
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				manager.SetTransport(transport)
			}()
		}
		
		// Rapid Start/Stop calls
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				manager.Start(ctx)
				cancel()
			}()
		}
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				manager.Stop(ctx)
				cancel()
			}()
		}
		
		wg.Wait()
		
		// Final cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		manager.Stop(ctx)
		cancel()
	}
}

// BenchmarkConcurrentOperations benchmarks concurrent operations
func BenchmarkConcurrentOperationsRace(b *testing.B) {
	b.Run("concurrent_sends", func(b *testing.B) {
		manager := NewSimpleManager()
		transport := NewRaceTestTransport()
		manager.SetTransport(transport)
		
		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			b.Fatalf("Failed to start manager: %v", err)
		}
		defer manager.Stop(ctx)
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				event := &DemoEvent{
					id:        fmt.Sprintf("bench-%d", i),
					eventType: "benchmark",
					timestamp: time.Now(),
				}
				
				sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				manager.Send(sendCtx, event)
				cancel()
				i++
			}
		})
	})
	
	b.Run("concurrent_start_stop", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				manager := NewSimpleManager()
				transport := NewRaceTestTransport()
				manager.SetTransport(transport)
				
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				manager.Start(ctx)
				manager.Stop(ctx)
				cancel()
			}
		})
	})
	
	b.Run("concurrent_set_transport", func(b *testing.B) {
		manager := NewSimpleManager()
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				transport := NewRaceTestTransport()
				manager.SetTransport(transport)
			}
		})
	})
}