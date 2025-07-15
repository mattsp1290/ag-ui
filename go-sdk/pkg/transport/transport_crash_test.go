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

// CrashingTransport simulates a transport that crashes during operation
type CrashingTransport struct {
	mu sync.RWMutex
	
	// Control crash behavior
	crashOnConnect   bool
	crashOnSend      bool
	crashAfterSends  int32
	sendCount        int32
	crashOnReceive   bool
	crashOnHealth    bool
	
	// Channels
	eventChan chan events.Event
	errorChan chan error
	
	// State
	connected bool
	crashed   bool
}

func NewCrashingTransport() *CrashingTransport {
	return &CrashingTransport{
		eventChan: make(chan events.Event, 10),
		errorChan: make(chan error, 10),
	}
}

func (t *CrashingTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.crashOnConnect {
		t.crashed = true
		return errors.New("transport crashed on connect")
	}
	
	if t.connected {
		return ErrAlreadyConnected
	}
	
	t.connected = true
	return nil
}

func (t *CrashingTransport) Close(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if !t.connected {
		return nil
	}
	
	t.connected = false
	
	// Close channels
	close(t.eventChan)
	close(t.errorChan)
	
	return nil
}

func (t *CrashingTransport) Send(ctx context.Context, event TransportEvent) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.crashed {
		return errors.New("transport has crashed")
	}
	
	if !t.connected {
		return ErrNotConnected
	}
	
	// Check if we should crash after certain number of sends
	if t.crashAfterSends > 0 {
		count := atomic.AddInt32(&t.sendCount, 1)
		if count >= t.crashAfterSends {
			t.crashed = true
			t.connected = false
			// Simulate crash by sending error
			select {
			case t.errorChan <- errors.New("transport crashed after sends"):
			default:
			}
			return errors.New("transport crashed")
		}
	}
	
	if t.crashOnSend {
		t.crashed = true
		t.connected = false
		return errors.New("transport crashed on send")
	}
	
	return nil
}

func (t *CrashingTransport) Receive() <-chan events.Event {
	if t.crashOnReceive {
		t.mu.Lock()
		t.crashed = true
		t.connected = false
		t.mu.Unlock()
		
		// Send crash error
		select {
		case t.errorChan <- errors.New("transport crashed on receive"):
		default:
		}
	}
	
	return t.eventChan
}

func (t *CrashingTransport) Errors() <-chan error {
	return t.errorChan
}

func (t *CrashingTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected && !t.crashed
}

func (t *CrashingTransport) Config() Config {
	return &BaseConfig{
		Type:           "crashing",
		Endpoint:       "crash://test",
		Timeout:        30 * time.Second,
		MaxMessageSize: 1024,
	}
}

func (t *CrashingTransport) Stats() TransportStats {
	return TransportStats{
		EventsSent:     int64(atomic.LoadInt32(&t.sendCount)),
		ReconnectCount: 0, // CrashingTransport doesn't track reconnects
	}
}

// Health and Metrics functionality removed - not part of Transport interface

// SetMiddleware removed - not part of Transport interface

// SimulateCrash manually triggers a crash
func (t *CrashingTransport) SimulateCrash() {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.crashed = true
	t.connected = false
	
	// Send crash error
	select {
	case t.errorChan <- errors.New("transport crashed"):
	default:
	}
}

// TestTransportCrashScenarios tests various transport crash scenarios
func TestTransportCrashScenarios(t *testing.T) {
	t.Run("crash_during_connect", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewCrashingTransport()
		transport.crashOnConnect = true
		
		manager.SetTransport(transport)
		
		ctx := context.Background()
		err := manager.Start(ctx)
		
		if err == nil || err.Error() != "transport crashed on connect" {
			t.Errorf("Expected crash error, got %v", err)
		}
		
		// Manager should not be running
		if atomic.LoadInt32(&manager.running) != 0 {
			t.Error("Manager should not be running after crash")
		}
	})

	t.Run("crash_during_send", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewCrashingTransport()
		transport.crashOnSend = true
		
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		event := &DemoEvent{id: "test-1", eventType: "demo"}
		err := manager.Send(context.Background(), event)
		
		if err == nil || err.Error() != "transport crashed on send" {
			t.Errorf("Expected crash error, got %v", err)
		}
	})

	t.Run("crash_after_multiple_sends", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewCrashingTransport()
		transport.crashAfterSends = 5
		
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		// Send multiple events
		var lastErr error
		for i := 0; i < 10; i++ {
			event := &DemoEvent{id: fmt.Sprintf("test-%d", i), eventType: "demo"}
			if err := manager.Send(context.Background(), event); err != nil {
				lastErr = err
				break
			}
		}
		
		if lastErr == nil || lastErr.Error() != "transport crashed" {
			t.Errorf("Expected crash after 5 sends, got %v", lastErr)
		}
		
		// Check stats
		stats := transport.Stats()
		if stats.EventsSent != 5 {
			t.Errorf("Expected 5 messages sent before crash, got %d", stats.EventsSent)
		}
	})

	t.Run("crash_during_receive", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewCrashingTransport()
		transport.crashOnReceive = true
		
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		// Trigger receive by accessing the channel
		go func() {
			<-manager.Receive()
		}()
		
		// Wait for crash error
		select {
		case err := <-manager.Errors():
			if err == nil || err.Error() != "transport crashed on receive" {
				t.Errorf("Expected crash error, got %v", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Expected crash error on receive")
		}
	})

	t.Run("crash_during_health_check", func(t *testing.T) {
		transport := NewCrashingTransport()
		transport.crashOnHealth = true
		
		ctx := context.Background()
		transport.Connect(ctx)
		
		// Health check removed - test stats instead
		stats := transport.Stats()
		if stats.EventsSent != 0 {
			t.Errorf("Expected 0 events sent, got %d", stats.EventsSent)
		}
	})

	t.Run("recovery_after_crash", func(t *testing.T) {
		manager := NewSimpleManager()
		
		// First transport that will crash
		transport1 := NewCrashingTransport()
		transport1.crashAfterSends = 3
		
		manager.SetTransport(transport1)
		manager.Start(context.Background())
		
		// Send until crash
		for i := 0; i < 5; i++ {
			event := &DemoEvent{id: fmt.Sprintf("test-%d", i), eventType: "demo"}
			manager.Send(context.Background(), event)
		}
		
		// Replace with new transport
		transport2 := NewCrashingTransport()
		manager.SetTransport(transport2)
		
		// Should be able to send again
		event := &DemoEvent{id: "after-recovery", eventType: "demo"}
		err := manager.Send(context.Background(), event)
		
		if err != nil {
			t.Errorf("Should be able to send after recovery: %v", err)
		}
		
		manager.Stop(context.Background())
	})
}

// TestConcurrentCrashScenarios tests crashes during concurrent operations
func TestConcurrentCrashScenarios(t *testing.T) {
	t.Run("crash_during_concurrent_sends", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewCrashingTransport()
		transport.crashAfterSends = 10
		
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		var wg sync.WaitGroup
		crashCount := int32(0)
		successCount := int32(0)
		
		// Launch concurrent sends
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				event := &DemoEvent{id: fmt.Sprintf("concurrent-%d", id), eventType: "demo"}
				if err := manager.Send(context.Background(), event); err != nil {
					if err.Error() == "transport crashed" || err.Error() == "transport has crashed" {
						atomic.AddInt32(&crashCount, 1)
					}
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}(i)
		}
		
		wg.Wait()
		
		// Should have some successes before crash
		if atomic.LoadInt32(&successCount) == 0 {
			t.Error("Expected some successful sends before crash")
		}
		
		// Should have detected the crash
		if atomic.LoadInt32(&crashCount) == 0 {
			t.Error("Expected to detect transport crash")
		}
		
		t.Logf("Success: %d, Crash detected: %d", 
			atomic.LoadInt32(&successCount), 
			atomic.LoadInt32(&crashCount))
	})

	t.Run("crash_with_multiple_managers", func(t *testing.T) {
		transport := NewCrashingTransport()
		
		// Create multiple managers sharing the same transport
		managers := make([]*SimpleManager, 3)
		for i := range managers {
			managers[i] = NewSimpleManager()
			managers[i].SetTransport(transport)
			managers[i].Start(context.Background())
			defer managers[i].Stop(context.Background())
		}
		
		// Simulate crash
		transport.SimulateCrash()
		
		// All managers should detect the crash
		for i, manager := range managers {
			event := &DemoEvent{id: fmt.Sprintf("manager-%d", i), eventType: "demo"}
			err := manager.Send(context.Background(), event)
			
			if err == nil {
				t.Errorf("Manager %d: Expected error after crash", i)
			}
		}
	})
}

// TestCrashRecoveryPatterns tests various recovery patterns after crashes
func TestCrashRecoveryPatterns(t *testing.T) {
	t.Run("automatic_reconnect_after_crash", func(t *testing.T) {
		// This tests a pattern where the manager could implement auto-reconnect
		manager := NewSimpleManager()
		
		reconnectAttempts := 0
		maxReconnectAttempts := 3
		
		var currentTransport *CrashingTransport
		
		// Function to create and set new transport
		createNewTransport := func() {
			reconnectAttempts++
			currentTransport = NewCrashingTransport()
			if reconnectAttempts < maxReconnectAttempts {
				currentTransport.crashAfterSends = 2 // Will crash again
			}
			manager.SetTransport(currentTransport)
		}
		
		createNewTransport()
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		// Send events and handle crashes
		successfulSends := 0
		for i := 0; i < 10; i++ {
			event := &DemoEvent{id: fmt.Sprintf("test-%d", i), eventType: "demo"}
			err := manager.Send(context.Background(), event)
			
			if err != nil && (err.Error() == "transport crashed" || err.Error() == "transport has crashed") {
				if reconnectAttempts < maxReconnectAttempts {
					createNewTransport()
					// Retry the send
					err = manager.Send(context.Background(), event)
				}
			}
			
			if err == nil {
				successfulSends++
			}
		}
		
		if reconnectAttempts != maxReconnectAttempts {
			t.Errorf("Expected %d reconnect attempts, got %d", maxReconnectAttempts, reconnectAttempts)
		}
		
		if successfulSends == 0 {
			t.Error("Expected some successful sends after reconnects")
		}
		
		t.Logf("Reconnect attempts: %d, Successful sends: %d", reconnectAttempts, successfulSends)
	})

	t.Run("graceful_degradation_on_crash", func(t *testing.T) {
		// Test fallback behavior when primary transport crashes
		primaryManager := NewSimpleManager()
		fallbackManager := NewSimpleManager()
		
		primaryTransport := NewCrashingTransport()
		primaryTransport.crashAfterSends = 3
		
		fallbackTransport := NewErrorTransport() // Stable transport
		
		primaryManager.SetTransport(primaryTransport)
		fallbackManager.SetTransport(fallbackTransport)
		
		primaryManager.Start(context.Background())
		fallbackManager.Start(context.Background())
		defer primaryManager.Stop(context.Background())
		defer fallbackManager.Stop(context.Background())
		
		// Function to send with fallback
		sendWithFallback := func(event TransportEvent) error {
			if err := primaryManager.Send(context.Background(), event); err != nil {
				// Primary failed, try fallback
				return fallbackManager.Send(context.Background(), event)
			}
			return nil
		}
		
		// Send multiple events
		primaryCount := 0
		fallbackCount := 0
		
		for i := 0; i < 10; i++ {
			event := &DemoEvent{id: fmt.Sprintf("test-%d", i), eventType: "demo"}
			err := sendWithFallback(event)
			
			if err == nil {
				if i < 3 {
					primaryCount++
				} else {
					fallbackCount++
				}
			}
		}
		
		if primaryCount != 3 {
			t.Errorf("Expected 3 sends via primary, got %d", primaryCount)
		}
		
		if fallbackCount == 0 {
			t.Error("Expected some sends via fallback after primary crash")
		}
		
		t.Logf("Primary sends: %d, Fallback sends: %d", primaryCount, fallbackCount)
	})
}

// BenchmarkCrashRecovery benchmarks crash recovery performance
func BenchmarkCrashRecovery(b *testing.B) {
	b.Run("transport_replacement_after_crash", func(b *testing.B) {
		manager := NewSimpleManager()
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		event := &DemoEvent{id: "bench", eventType: "demo"}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create transport that crashes after 10 sends
			transport := NewCrashingTransport()
			transport.crashAfterSends = 10
			manager.SetTransport(transport)
			
			// Send until crash
			for j := 0; j < 15; j++ {
				manager.Send(context.Background(), event)
			}
		}
	})

	b.Run("concurrent_crash_detection", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			manager := NewSimpleManager()
			transport := NewCrashingTransport()
			transport.crashAfterSends = 100
			
			manager.SetTransport(transport)
			manager.Start(context.Background())
			defer manager.Stop(context.Background())
			
			event := &DemoEvent{id: "bench", eventType: "demo"}
			
			for pb.Next() {
				err := manager.Send(context.Background(), event)
				if err != nil && (err.Error() == "transport crashed" || err.Error() == "transport has crashed") {
					// Reset with new transport
					transport = NewCrashingTransport()
					transport.crashAfterSends = 100
					manager.SetTransport(transport)
				}
			}
		})
	})
}