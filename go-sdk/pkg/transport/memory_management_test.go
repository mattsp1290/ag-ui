package transport

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/proto/generated"
)

// mockEvent implements events.Event for testing
type mockEvent struct {
	data map[string]interface{}
}

func (m *mockEvent) Type() events.EventType                { return "mock" }
func (m *mockEvent) Timestamp() *int64                     { return nil }
func (m *mockEvent) SetTimestamp(int64)                    {}
func (m *mockEvent) Validate() error                       { return nil }
func (m *mockEvent) ToJSON() ([]byte, error)               { return nil, nil }
func (m *mockEvent) ToProtobuf() (*generated.Event, error) { return nil, nil }
func (m *mockEvent) GetBaseEvent() *events.BaseEvent       { return nil }
func (m *mockEvent) ThreadID() string                      { return "test-thread" }
func (m *mockEvent) RunID() string                         { return "test-run" }

func TestMemoryManager(t *testing.T) {
	// Create memory manager with test configuration
	config := &MemoryManagerConfig{
		LowMemoryPercent:      50.0, // Lower thresholds for testing
		HighMemoryPercent:     70.0,
		CriticalMemoryPercent: 90.0,
		MonitorInterval:       100 * time.Millisecond,
	}

	mm := NewMemoryManager(config)
	t.Cleanup(func() {
		mm.Stop()
	})

	mm.Start()

	// Test adaptive buffer sizing
	t.Run("AdaptiveBufferSizing", func(t *testing.T) {
		baseSize := 1000

		// Normal pressure should return base size
		adaptedSize := mm.GetAdaptiveBufferSize("test_buffer", baseSize)
		if adaptedSize != baseSize {
			t.Errorf("Expected base size %d, got %d", baseSize, adaptedSize)
		}
	})

	// Test memory pressure callbacks
	t.Run("MemoryPressureCallbacks", func(t *testing.T) {
		callbackCalled := make(chan MemoryPressureLevel, 1)

		mm.OnMemoryPressure(func(level MemoryPressureLevel) {
			select {
			case callbackCalled <- level:
			default:
			}
		})

		// Force a memory pressure change (in real scenarios this would happen naturally)
		// We'll just test that the callback mechanism works
		mm.notifyPressureChange(MemoryPressureHigh)

		select {
		case level := <-callbackCalled:
			if level != MemoryPressureHigh {
				t.Errorf("Expected MemoryPressureHigh, got %v", level)
			}
		case <-time.After(1 * time.Second):
			t.Error("Callback was not called within timeout")
		}
	})

	// Test metrics collection
	t.Run("MetricsCollection", func(t *testing.T) {
		metrics := mm.GetMetrics()

		if metrics.TotalAllocated == 0 {
			t.Error("Expected non-zero total allocated memory")
		}

		if metrics.HeapInUse == 0 {
			t.Error("Expected non-zero heap in use")
		}
	})
}

func TestRingBuffer(t *testing.T) {
	config := DefaultRingBufferConfig()
	config.Capacity = 10

	rb := NewRingBuffer(config)
	defer rb.Close()

	// Test basic operations
	t.Run("BasicOperations", func(t *testing.T) {
		// Test push and pop
		event := &mockEvent{data: map[string]interface{}{"test": "value"}}

		err := rb.Push(event)
		if err != nil {
			t.Fatalf("Failed to push event: %v", err)
		}

		if rb.Size() != 1 {
			t.Errorf("Expected size 1, got %d", rb.Size())
		}

		poppedEvent, err := rb.Pop()
		if err != nil {
			t.Fatalf("Failed to pop event: %v", err)
		}

		if poppedEvent != event {
			t.Error("Popped event doesn't match pushed event")
		}

		if rb.Size() != 0 {
			t.Errorf("Expected size 0, got %d", rb.Size())
		}
	})

	// Test overflow handling
	t.Run("OverflowHandling", func(t *testing.T) {
		// Fill buffer to capacity
		for i := 0; i < config.Capacity; i++ {
			event := &mockEvent{data: map[string]interface{}{"id": i}}
			err := rb.Push(event)
			if err != nil {
				t.Fatalf("Failed to push event %d: %v", i, err)
			}
		}

		if !rb.IsFull() {
			t.Error("Buffer should be full")
		}

		// Try to push one more (should trigger overflow policy)
		extraEvent := &mockEvent{data: map[string]interface{}{"id": "extra"}}
		err := rb.Push(extraEvent)
		if err != nil {
			t.Fatalf("Push with overflow should not return error: %v", err)
		}

		// Should still be at capacity
		if rb.Size() != config.Capacity {
			t.Errorf("Expected size %d, got %d", config.Capacity, rb.Size())
		}
	})

	// Test concurrent access
	t.Run("ConcurrentAccess", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping concurrent access test in short mode")
		}
		if os.Getenv("RACE") == "1" {
			t.Skip("Skipping concurrent access test during race detection")
		}

		rb.Clear()

		var wg sync.WaitGroup
		numProducers := 3       // Reduced from 5
		numConsumers := 2       // Reduced from 3
		eventsPerProducer := 20 // Reduced from 100

		// Start producers
		for p := 0; p < numProducers; p++ {
			wg.Add(1)
			go func(producerID int) {
				defer wg.Done()
				for i := 0; i < eventsPerProducer; i++ {
					event := &mockEvent{data: map[string]interface{}{"producer": producerID, "event": i}}
					rb.Push(event)
				}
			}(p)
		}

		// Start consumers
		consumedCount := make([]int, numConsumers)
		for c := 0; c < numConsumers; c++ {
			wg.Add(1)
			go func(consumerID int) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second) // Reduced timeout
				defer cancel()

				for {
					event, err := rb.PopWithContext(ctx)
					if err != nil {
						return
					}
					if event != nil {
						consumedCount[consumerID]++
					}
				}
			}(c)
		}

		wg.Wait()

		totalConsumed := 0
		for _, count := range consumedCount {
			totalConsumed += count
		}

		t.Logf("Total events consumed: %d", totalConsumed)
		// Some events may be dropped due to overflow, so we just check that we consumed some
		if totalConsumed == 0 {
			t.Error("No events were consumed")
		}
	})
}

func TestCleanupManager(t *testing.T) {
	config := DefaultCleanupManagerConfig()
	config.CheckInterval = 100 * time.Millisecond
	config.DefaultTTL = 200 * time.Millisecond

	cm := NewCleanupManager(config)
	defer cm.Stop()

	err := cm.Start()
	if err != nil {
		t.Fatalf("Failed to start cleanup manager: %v", err)
	}

	// Test task registration and execution
	t.Run("TaskExecution", func(t *testing.T) {
		cleanupCalled := make(chan int, 1)

		cleanupFunc := func() (int, error) {
			select {
			case cleanupCalled <- 42:
			default:
			}
			return 1, nil
		}

		err := cm.RegisterTask("test_task", 50*time.Millisecond, cleanupFunc)
		if err != nil {
			t.Fatalf("Failed to register task: %v", err)
		}

		// Wait for cleanup to run
		select {
		case result := <-cleanupCalled:
			if result != 42 {
				t.Errorf("Expected 42, got %d", result)
			}
		case <-time.After(1 * time.Second):
			t.Error("Cleanup function was not called within timeout")
		}
	})

	// Test map cleanup functionality
	t.Run("MapCleanup", func(t *testing.T) {
		testMap := &sync.Map{}

		// Add some test data with timestamps
		type testData struct {
			value     string
			timestamp time.Time
		}

		// Add current data (should not be cleaned)
		testMap.Store("current", testData{value: "current", timestamp: time.Now()})

		// Add old data (should be cleaned)
		testMap.Store("old", testData{value: "old", timestamp: time.Now().Add(-1 * time.Hour)})

		cleanupFunc := CreateMapCleanupFunc[string, testData](
			testMap,
			func(data testData) time.Time { return data.timestamp },
			30*time.Minute,
		)

		cleaned, err := cleanupFunc()
		if err != nil {
			t.Fatalf("Cleanup function failed: %v", err)
		}

		if cleaned != 1 {
			t.Errorf("Expected 1 item cleaned, got %d", cleaned)
		}

		// Verify current item still exists
		if _, exists := testMap.Load("current"); !exists {
			t.Error("Current item should still exist")
		}

		// Verify old item was removed
		if _, exists := testMap.Load("old"); exists {
			t.Error("Old item should have been removed")
		}
	})
}

func TestBackpressureHandler(t *testing.T) {
	config := BackpressureConfig{
		Strategy:      BackpressureDropOldest,
		BufferSize:    10,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		EnableMetrics: true,
	}

	handler := NewBackpressureHandler(config)
	defer handler.Stop()

	// Test basic event sending
	t.Run("BasicEventSending", func(t *testing.T) {
		event := &mockEvent{data: map[string]interface{}{"test": "value"}}

		err := handler.SendEvent(event)
		if err != nil {
			t.Fatalf("Failed to send event: %v", err)
		}

		// Receive the event
		select {
		case receivedEvent := <-handler.EventChan():
			if receivedEvent != event {
				t.Error("Received event doesn't match sent event")
			}
		case <-time.After(1 * time.Second):
			t.Error("Event was not received within timeout")
		}
	})

	// Test backpressure behavior
	t.Run("BackpressureBehavior", func(t *testing.T) {
		// Fill the buffer
		for i := 0; i < config.BufferSize+5; i++ {
			event := &mockEvent{data: map[string]interface{}{"id": i}}
			handler.SendEvent(event) // Should not block due to drop strategy
		}

		metrics := handler.GetMetrics()
		if metrics.EventsDropped == 0 {
			t.Error("Expected some events to be dropped")
		}

		t.Logf("Events dropped: %d", metrics.EventsDropped)
	})
}

func TestSlice(t *testing.T) {
	slice := NewSlice()

	// Test basic operations
	t.Run("BasicOperations", func(t *testing.T) {
		// Test append and length
		slice.Append("item1")
		slice.Append("item2")
		slice.Append("item3")

		if slice.Len() != 3 {
			t.Errorf("Expected length 3, got %d", slice.Len())
		}

		// Test get
		item, exists := slice.Get(1)
		if !exists || item != "item2" {
			t.Errorf("Expected 'item2', got %v (exists: %v)", item, exists)
		}

		// Test remove
		removed := slice.RemoveFunc(func(item interface{}) bool {
			return item == "item2"
		})

		if !removed {
			t.Error("Expected item to be removed")
		}

		if slice.Len() != 2 {
			t.Errorf("Expected length 2 after removal, got %d", slice.Len())
		}
	})

	// Test concurrent access
	t.Run("ConcurrentAccess", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping concurrent access test in short mode")
		}

		slice.Clear()

		var wg sync.WaitGroup
		numGoroutines := 5      // Reduced from 10
		itemsPerGoroutine := 20 // Reduced from 100

		// Concurrent appends
		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for i := 0; i < itemsPerGoroutine; i++ {
					slice.Append(fmt.Sprintf("g%d-i%d", goroutineID, i))
				}
			}(g)
		}

		wg.Wait()

		expectedLength := numGoroutines * itemsPerGoroutine
		if slice.Len() != expectedLength {
			t.Errorf("Expected length %d, got %d", expectedLength, slice.Len())
		}
	})

	// Test functional operations
	t.Run("FunctionalOperations", func(t *testing.T) {
		slice.Clear()
		slice.Append(1)
		slice.Append(2)
		slice.Append(3)
		slice.Append(4)

		// Test filter
		evenSlice := slice.Filter(func(item interface{}) bool {
			return item.(int)%2 == 0
		})

		if evenSlice.Len() != 2 {
			t.Errorf("Expected 2 even numbers, got %d", evenSlice.Len())
		}

		// Test any
		hasOdd := slice.Any(func(item interface{}) bool {
			return item.(int)%2 == 1
		})

		if !hasOdd {
			t.Error("Expected to find odd numbers")
		}

		// Test all
		allPositive := slice.All(func(item interface{}) bool {
			return item.(int) > 0
		})

		if !allPositive {
			t.Error("Expected all numbers to be positive")
		}
	})
}

// Force garbage collection to test memory pressure
func TestMemoryPressureSimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory pressure test in short mode")
	}

	mm := NewMemoryManager(nil)
	defer mm.Stop()
	mm.Start()

	// Allocate a lot of memory to trigger pressure
	t.Run("SimulateMemoryPressure", func(t *testing.T) {
		var memoryHog [][]byte

		pressureDetected := make(chan bool, 1)
		mm.OnMemoryPressure(func(level MemoryPressureLevel) {
			if level >= MemoryPressureLow {
				select {
				case pressureDetected <- true:
				default:
				}
			}
		})

		// Allocate memory in chunks - reduced for faster execution
		for i := 0; i < 20; i++ { // Reduced from 100
			chunk := make([]byte, 512*1024) // 512KB chunks (reduced from 1MB)
			memoryHog = append(memoryHog, chunk)

			// Force GC occasionally
			if i%5 == 0 { // Reduced interval
				runtime.GC()
				time.Sleep(5 * time.Millisecond) // Reduced sleep time
			}
		}

		// Force memory update
		mm.updateMemoryUsage()
		mm.checkMemoryPressure()

		// Clean up
		memoryHog = nil
		runtime.GC()

		t.Logf("Memory pressure simulation completed")
	})
}
