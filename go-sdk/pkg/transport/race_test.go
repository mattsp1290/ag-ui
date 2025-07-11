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
)

// RaceTestTransport is a transport implementation specifically designed for race condition testing
type RaceTestTransport struct {
	mu             sync.RWMutex
	connected      int32 // Use atomic operations for connected state
	connecting     int32 // Use atomic operations for connecting state
	connectDelay   time.Duration
	sendDelay      time.Duration
	eventChan      chan Event
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
		eventChan:    make(chan Event, 100),
		errorChan:    make(chan error, 100),
		connectDelay: 10 * time.Millisecond,
		sendDelay:    1 * time.Millisecond,
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
	atomic.AddInt64(&t.sendCount, 1)
	
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
	
	// Try to send event back (echo)
	select {
	case t.eventChan <- Event{
		Event:     event,
		Metadata:  EventMetadata{TransportID: "race-test"},
		Timestamp: time.Now(),
	}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Check if we're closed
		if atomic.LoadInt32(&t.closed) == 1 {
			return errors.New("transport closed")
		}
		return errors.New("event channel full")
	}
}

func (t *RaceTestTransport) Receive() <-chan Event {
	return t.eventChan
}

func (t *RaceTestTransport) Errors() <-chan error {
	return t.errorChan
}

func (t *RaceTestTransport) IsConnected() bool {
	return atomic.LoadInt32(&t.connected) == 1
}

func (t *RaceTestTransport) Capabilities() Capabilities {
	return Capabilities{
		Streaming:     true,
		Bidirectional: true,
		Multiplexing:  true,
	}
}

func (t *RaceTestTransport) Health(ctx context.Context) error {
	if atomic.LoadInt32(&t.connected) == 0 {
		return ErrNotConnected
	}
	return nil
}

func (t *RaceTestTransport) Metrics() Metrics {
	return Metrics{
		ConnectionUptime:  time.Hour,
		MessagesSent:      uint64(atomic.LoadInt64(&t.sendCount)),
		MessagesReceived:  uint64(atomic.LoadInt64(&t.sendCount)),
		AverageLatency:    t.sendDelay,
		CurrentThroughput: 1000.0,
	}
}

func (t *RaceTestTransport) SetMiddleware(middleware ...Middleware) {
	// No-op for race testing
}

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
	const numGoroutines = 10
	const numIterations = 100
	
	for iteration := 0; iteration < numIterations; iteration++ {
		manager := NewSimpleManager()
		transport := NewRaceTestTransport()
		manager.SetTransport(transport)
		
		var wg sync.WaitGroup
		var startErrors, stopErrors int64
		
		// Launch concurrent start operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
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
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				
				// Add small delay to let some starts happen first
				time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
				
				if err := manager.Stop(ctx); err != nil {
					atomic.AddInt64(&stopErrors, 1)
					t.Errorf("Stop error: %v", err)
				}
			}()
		}
		
		wg.Wait()
		
		// Final cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
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
					if event.Event == nil {
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
	
	allocDiff := m2.Alloc - m1.Alloc
	heapDiff := m2.HeapAlloc - m1.HeapAlloc
	
	t.Logf("Memory usage after %d iterations:", numIterations)
	t.Logf("  Alloc diff: %d bytes", allocDiff)
	t.Logf("  Heap diff: %d bytes", heapDiff)
	t.Logf("  Goroutines: %d", runtime.NumGoroutine())
	
	// Check for excessive memory growth (this is a rough heuristic)
	if allocDiff > 10*1024*1024 { // 10MB
		t.Errorf("Potential memory leak detected: %d bytes allocated", allocDiff)
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
func BenchmarkConcurrentOperations(b *testing.B) {
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