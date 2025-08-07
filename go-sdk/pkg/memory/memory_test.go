package memory

import (
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
func (m *mockEvent) ThreadID() string                      { return "" }
func (m *mockEvent) RunID() string                         { return "" }
func (m *mockEvent) Validate() error                       { return nil }
func (m *mockEvent) ToJSON() ([]byte, error)               { return nil, nil }
func (m *mockEvent) ToProtobuf() (*generated.Event, error) { return nil, nil }
func (m *mockEvent) GetBaseEvent() *events.BaseEvent       { return nil }

func TestMemoryManager(t *testing.T) {
	config := &MemoryManagerConfig{
		LowMemoryPercent:      50.0,
		HighMemoryPercent:     70.0,
		CriticalMemoryPercent: 90.0,
		MonitorInterval:       100 * time.Millisecond,
	}

	mm := NewMemoryManager(config)
	defer mm.Stop()

	mm.Start()

	t.Run("AdaptiveBufferSizing", func(t *testing.T) {
		baseSize := 1000
		adaptedSize := mm.GetAdaptiveBufferSize("test_buffer", baseSize)

		if adaptedSize <= 0 {
			t.Errorf("Expected positive buffer size, got %d", adaptedSize)
		}
	})

	t.Run("MemoryPressureCallbacks", func(t *testing.T) {
		callbackCalled := make(chan MemoryPressureLevel, 1)

		mm.OnMemoryPressure(func(level MemoryPressureLevel) {
			select {
			case callbackCalled <- level:
			default:
			}
		})

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

	t.Run("MetricsCollection", func(t *testing.T) {
		metrics := mm.GetMetrics()

		if metrics.TotalAllocated == 0 {
			t.Error("Expected non-zero total allocated memory")
		}
	})
}

func TestRingBuffer(t *testing.T) {
	config := DefaultRingBufferConfig()
	config.Capacity = 10

	rb := NewRingBuffer(config)
	defer rb.Close()

	t.Run("BasicOperations", func(t *testing.T) {
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

	t.Run("OverflowHandling", func(t *testing.T) {
		rb.Clear()

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

	t.Run("ConcurrentAccess", func(t *testing.T) {
		rb.Clear()

		var wg sync.WaitGroup
		numProducers := 5
		eventsPerProducer := 100

		// Start producers
		for p := 0; p < numProducers; p++ {
			wg.Add(1)
			go func(producerID int) {
				defer wg.Done()
				for i := 0; i < eventsPerProducer; i++ {
					event := &mockEvent{data: map[string]interface{}{"producer": producerID, "event": i}}
					rb.Push(event) // Will handle overflow according to policy
				}
			}(p)
		}

		wg.Wait()

		// Now consume all available events
		consumed := 0
		for {
			if event, ok := rb.TryPop(); ok && event != nil {
				consumed++
			} else {
				break
			}
		}

		t.Logf("Total events consumed: %d", consumed)
		if consumed == 0 {
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

	t.Run("MapCleanup", func(t *testing.T) {
		testMap := &sync.Map{}

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

	t.Run("BackpressureBehavior", func(t *testing.T) {
		// Clear any existing events
		for {
			select {
			case <-handler.EventChan():
			default:
				goto fillBuffer
			}
		}

	fillBuffer:
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

	t.Run("BasicOperations", func(t *testing.T) {
		slice.Append("item1")
		slice.Append("item2")
		slice.Append("item3")

		if slice.Len() != 3 {
			t.Errorf("Expected length 3, got %d", slice.Len())
		}

		item, exists := slice.Get(1)
		if !exists || item != "item2" {
			t.Errorf("Expected 'item2', got %v (exists: %v)", item, exists)
		}

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

	t.Run("ConcurrentAccess", func(t *testing.T) {
		slice.Clear()

		var wg sync.WaitGroup
		numGoroutines := 10
		itemsPerGoroutine := 100

		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for i := 0; i < itemsPerGoroutine; i++ {
					slice.Append(goroutineID*itemsPerGoroutine + i)
				}
			}(g)
		}

		wg.Wait()

		expectedLength := numGoroutines * itemsPerGoroutine
		if slice.Len() != expectedLength {
			t.Errorf("Expected length %d, got %d", expectedLength, slice.Len())
		}
	})

	t.Run("FunctionalOperations", func(t *testing.T) {
		slice.Clear()
		slice.Append(1)
		slice.Append(2)
		slice.Append(3)
		slice.Append(4)

		evenSlice := slice.Filter(func(item interface{}) bool {
			return item.(int)%2 == 0
		})

		if evenSlice.Len() != 2 {
			t.Errorf("Expected 2 even numbers, got %d", evenSlice.Len())
		}

		hasOdd := slice.Any(func(item interface{}) bool {
			return item.(int)%2 == 1
		})

		if !hasOdd {
			t.Error("Expected to find odd numbers")
		}

		allPositive := slice.All(func(item interface{}) bool {
			return item.(int) > 0
		})

		if !allPositive {
			t.Error("Expected all numbers to be positive")
		}
	})
}

func BenchmarkMemoryManager(b *testing.B) {
	mm := NewMemoryManager(nil)
	defer mm.Stop()
	mm.Start()

	b.Run("GetAdaptiveBufferSize", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mm.GetAdaptiveBufferSize("test", 1000)
		}
	})

	b.Run("GetMemoryPressureLevel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mm.GetMemoryPressureLevel()
		}
	})

	b.Run("GetMetrics", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mm.GetMetrics()
		}
	})
}

func BenchmarkRingBuffer(b *testing.B) {
	rb := NewRingBuffer(DefaultRingBufferConfig())
	defer rb.Close()

	event := &mockEvent{data: map[string]interface{}{"test": "value"}}

	b.Run("Push", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rb.Push(event)
		}
	})

	b.Run("Pop", func(b *testing.B) {
		// Pre-fill buffer
		for i := 0; i < 1000; i++ {
			rb.Push(event)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rb.TryPop()
		}
	})
}

func BenchmarkSlice(b *testing.B) {
	slice := NewSlice()

	b.Run("Append", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			slice.Append(i)
		}
	})

	b.Run("Get", func(b *testing.B) {
		// Pre-populate
		for i := 0; i < 1000; i++ {
			slice.Append(i)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			slice.Get(i % 1000)
		}
	})
}

func TestMemoryPressureSimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory pressure test in short mode")
	}

	mm := NewMemoryManager(&MemoryManagerConfig{
		LowMemoryPercent:      30.0, // Lower thresholds for testing
		HighMemoryPercent:     50.0,
		CriticalMemoryPercent: 70.0,
		MonitorInterval:       50 * time.Millisecond,
	})
	defer mm.Stop()
	mm.Start()

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

		// Allocate memory in chunks
		for i := 0; i < 50; i++ {
			chunk := make([]byte, 512*1024) // 512KB chunks
			memoryHog = append(memoryHog, chunk)

			if i%5 == 0 {
				runtime.GC()
				time.Sleep(10 * time.Millisecond)
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
