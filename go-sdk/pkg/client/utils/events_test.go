package utils

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/proto/generated"
)

// Type aliases for cleaner code
type Event = events.Event
type EventType = events.EventType

// MockEvent for testing
type MockEvent struct {
	id        string
	eventType EventType
	timestamp *int64
	data      map[string]interface{}
	mu        sync.RWMutex
}

func NewMockEvent(id string, eventType EventType) *MockEvent {
	now := time.Now().UnixMilli()
	return &MockEvent{
		id:        id,
		eventType: eventType,
		timestamp: &now,
		data:      make(map[string]interface{}),
	}
}

func (e *MockEvent) ID() string                           { return e.id }
func (e *MockEvent) Type() EventType               { return e.eventType }
func (e *MockEvent) Timestamp() *int64                    { return e.timestamp }
func (e *MockEvent) Data() map[string]interface{} { 
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make(map[string]interface{})
	for k, v := range e.data {
		result[k] = v
	}
	return result
}

func (e *MockEvent) SetData(key string, value interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.data[key] = value
}

func (e *MockEvent) SetTimestamp(timestamp int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.timestamp = &timestamp
}

func (e *MockEvent) ThreadID() string { return "" }
func (e *MockEvent) RunID() string { return "" }
func (e *MockEvent) Validate() error { return nil }
func (e *MockEvent) ToJSON() ([]byte, error) { return nil, nil }
func (e *MockEvent) ToProtobuf() (*generated.Event, error) { return nil, nil }
func (e *MockEvent) GetBaseEvent() *events.BaseEvent {
	return &events.BaseEvent{
		EventType:   e.eventType,
		TimestampMs: e.timestamp,
	}
}

// Mock event types
const (
	MockEventTypeTest    EventType = "test"
	MockEventTypeProcess EventType = "process" 
	MockEventTypeError   EventType = "error"
	MockEventTypeCustom  EventType = "custom"
)

func TestNewEventUtils(t *testing.T) {
	utils := NewEventUtils()
	
	if utils == nil {
		t.Fatal("NewEventUtils returned nil")
	}
	if utils.processors == nil {
		t.Error("processors map not initialized")
	}
	if utils.metrics == nil {
		t.Error("metrics not initialized")
	}
	if utils.metrics.ProcessorMetrics == nil {
		t.Error("processor metrics map not initialized")
	}
}

func TestEventUtils_CreateProcessor(t *testing.T) {
	utils := NewEventUtils()

	t.Run("ValidProcessor", func(t *testing.T) {
		processor := utils.CreateProcessor("test-processor", 100)
		
		if processor == nil {
			t.Fatal("CreateProcessor returned nil")
		}
		if processor.name != "test-processor" {
			t.Errorf("Expected name 'test-processor', got %s", processor.name)
		}
		if processor.bufferSize != 100 {
			t.Errorf("Expected buffer size 100, got %d", processor.bufferSize)
		}
		if cap(processor.input) != 100 {
			t.Errorf("Expected input channel capacity 100, got %d", cap(processor.input))
		}
		if cap(processor.output) != 100 {
			t.Errorf("Expected output channel capacity 100, got %d", cap(processor.output))
		}
	})

	t.Run("ProcessorRegistered", func(t *testing.T) {
		processorName := "registered-processor"
		processor := utils.CreateProcessor(processorName, 50)
		
		// Check if processor is registered
		utils.processorsMu.RLock()
		registered, exists := utils.processors[processorName]
		utils.processorsMu.RUnlock()
		
		if !exists {
			t.Error("Processor not registered in utils map")
		}
		if registered != processor {
			t.Error("Wrong processor registered")
		}
	})

	t.Run("ConcurrentCreation", func(t *testing.T) {
		var wg sync.WaitGroup
		numProcessors := 10
		
		for i := 0; i < numProcessors; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				name := fmt.Sprintf("concurrent-processor-%d", id)
				processor := utils.CreateProcessor(name, 25)
				if processor == nil {
					t.Errorf("Failed to create processor %d", id)
				}
			}(i)
		}
		
		wg.Wait()
		
		// Verify all processors were created
		utils.processorsMu.RLock()
		count := len(utils.processors)
		utils.processorsMu.RUnlock()
		
		expectedCount := numProcessors + 2 // +2 from previous tests
		if count != expectedCount {
			t.Errorf("Expected %d processors, got %d", expectedCount, count)
		}
	})
}

func TestEventUtils_GetProcessor(t *testing.T) {
	utils := NewEventUtils()
	
	t.Run("ExistingProcessor", func(t *testing.T) {
		originalProcessor := utils.CreateProcessor("get-test", 50)
		
		retrievedProcessor, err := utils.GetProcessor("get-test")
		if err != nil {
			t.Fatalf("GetProcessor failed: %v", err)
		}
		if retrievedProcessor != originalProcessor {
			t.Error("Retrieved processor is not the same as original")
		}
	})
	
	t.Run("NonExistentProcessor", func(t *testing.T) {
		_, err := utils.GetProcessor("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent processor")
		}
	})
}

func TestEventUtils_StartStopProcessor(t *testing.T) {
	utils := NewEventUtils()
	
	t.Run("StartValidProcessor", func(t *testing.T) {
		utils.CreateProcessor("start-test", 50)
		
		err := utils.StartProcessor("start-test")
		if err != nil {
			t.Fatalf("StartProcessor failed: %v", err)
		}
		
		processor, _ := utils.GetProcessor("start-test")
		if !processor.isRunning.Load() {
			t.Error("Processor should be running")
		}
		
		// Stop it
		err = utils.StopProcessor("start-test")
		if err != nil {
			t.Fatalf("StopProcessor failed: %v", err)
		}
		
		if processor.isRunning.Load() {
			t.Error("Processor should be stopped")
		}
	})
	
	t.Run("StartNonExistentProcessor", func(t *testing.T) {
		err := utils.StartProcessor("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent processor")
		}
	})
	
	t.Run("StopNonExistentProcessor", func(t *testing.T) {
		err := utils.StopProcessor("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent processor")
		}
	})
}

func TestEventProcessor_AddFilter(t *testing.T) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("filter-test", 50)
	
	filter := NewEventTypeFilter(MockEventTypeTest)
	result := processor.AddFilter(filter)
	
	if result != processor {
		t.Error("AddFilter should return same processor instance")
	}
	if len(processor.filters) != 1 {
		t.Error("Filter not added")
	}
	if processor.filters[0] != filter {
		t.Error("Wrong filter added")
	}
}

func TestEventProcessor_AddHandler(t *testing.T) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("handler-test", 50)
	
	handler := NewLoggingHandler()
	result := processor.AddHandler(handler)
	
	if result != processor {
		t.Error("AddHandler should return same processor instance")
	}
	if len(processor.handlers) != 1 {
		t.Error("Handler not added")
	}
	if processor.handlers[0] != handler {
		t.Error("Wrong handler added")
	}
}

func TestEventProcessor_AddWindow(t *testing.T) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("window-test", 50)
	
	window := WindowConfig{
		Type:     WindowTypeTime,
		Duration: 1 * time.Second,
	}
	result := processor.AddWindow(window)
	
	if result != processor {
		t.Error("AddWindow should return same processor instance")
	}
	if len(processor.windows) != 1 {
		t.Error("Window not added")
	}
	if processor.windows[0].Type != WindowTypeTime {
		t.Error("Wrong window added")
	}
}

func TestEventProcessor_SetBatchConfig(t *testing.T) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("batch-test", 50)
	
	result := processor.SetBatchConfig(25, 2*time.Second)
	
	if result != processor {
		t.Error("SetBatchConfig should return same processor instance")
	}
	if processor.batchSize != 25 {
		t.Errorf("Expected batch size 25, got %d", processor.batchSize)
	}
	if processor.batchTimeout != 2*time.Second {
		t.Errorf("Expected batch timeout 2s, got %v", processor.batchTimeout)
	}
}

func TestEventProcessor_StartStop(t *testing.T) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("lifecycle-test", 50)
	
	t.Run("StartProcessor", func(t *testing.T) {
		err := processor.Start()
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		
		if !processor.isRunning.Load() {
			t.Error("Processor should be running")
		}
	})
	
	t.Run("DoubleStart", func(t *testing.T) {
		err := processor.Start()
		if err == nil {
			t.Error("Expected error for double start")
		}
	})
	
	t.Run("StopProcessor", func(t *testing.T) {
		err := processor.Stop()
		if err != nil {
			t.Fatalf("Stop failed: %v", err)
		}
		
		if processor.isRunning.Load() {
			t.Error("Processor should not be running")
		}
	})
	
	t.Run("DoubleStop", func(t *testing.T) {
		err := processor.Stop()
		if err == nil {
			t.Error("Expected error for double stop")
		}
	})
}

func TestEventProcessor_SendEvent(t *testing.T) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("send-test", 50)
	
	event := NewMockEvent("test-event-1", MockEventTypeTest)
	
	t.Run("SendToStoppedProcessor", func(t *testing.T) {
		err := processor.SendEvent(event)
		if err == nil {
			t.Error("Expected error when sending to stopped processor")
		}
	})
	
	t.Run("SendToRunningProcessor", func(t *testing.T) {
		processor.Start()
		defer processor.Stop()
		
		err := processor.SendEvent(event)
		if err != nil {
			t.Errorf("SendEvent failed: %v", err)
		}
	})
	
	t.Run("SendToFullBuffer", func(t *testing.T) {
		smallProcessor := utils.CreateProcessor("small-buffer", 1)
		smallProcessor.Start()
		defer smallProcessor.Stop()
		
		// Fill the buffer
		event1 := NewMockEvent("event-1", MockEventTypeTest)
		err := smallProcessor.SendEvent(event1)
		if err != nil {
			t.Errorf("First SendEvent failed: %v", err)
		}
		
		// This should fail because buffer is full and no processing is happening
		event2 := NewMockEvent("event-2", MockEventTypeTest) 
		err = smallProcessor.SendEvent(event2)
		if err == nil {
			t.Error("Expected error when buffer is full")
		}
	})
}

func TestEventProcessor_EventProcessing(t *testing.T) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("processing-test", 100)
	
	// Add a simple logging handler
	handler := NewLoggingHandler()
	processor.AddHandler(handler)
	
	// Start processing
	err := processor.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer processor.Stop()
	
	// Send some events
	numEvents := 10
	for i := 0; i < numEvents; i++ {
		event := NewMockEvent(fmt.Sprintf("event-%d", i), MockEventTypeTest)
		err := processor.SendEvent(event)
		if err != nil {
			t.Errorf("SendEvent %d failed: %v", i, err)
		}
	}
	
	// Give time for processing
	time.Sleep(100 * time.Millisecond)
	
	// Check metrics
	metrics := processor.GetMetrics()
	if metrics.EventsProcessed < int64(numEvents) {
		t.Errorf("Expected at least %d events processed, got %d", numEvents, metrics.EventsProcessed)
	}
}

func TestEventProcessor_Filtering(t *testing.T) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("filter-test", 100)
	
	// Add filter that only allows test events
	filter := NewEventTypeFilter(MockEventTypeTest)
	processor.AddFilter(filter)
	
	handler := NewLoggingHandler()
	processor.AddHandler(handler)
	
	err := processor.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer processor.Stop()
	
	// Send mixed event types
	testEvent := NewMockEvent("test-event", MockEventTypeTest)
	errorEvent := NewMockEvent("error-event", MockEventTypeError)
	
	processor.SendEvent(testEvent)
	processor.SendEvent(errorEvent)
	
	// Give time for processing
	time.Sleep(100 * time.Millisecond)
	
	metrics := processor.GetMetrics()
	
	// Should have processed 1 event (test) and filtered 1 (error)
	if metrics.EventsProcessed != 1 {
		t.Errorf("Expected 1 processed event, got %d", metrics.EventsProcessed)
	}
	if metrics.EventsFiltered != 1 {
		t.Errorf("Expected 1 filtered event, got %d", metrics.EventsFiltered)
	}
}

func TestEventProcessor_BatchProcessing(t *testing.T) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("batch-test", 100)
	
	// Set small batch size and timeout
	processor.SetBatchConfig(3, 50*time.Millisecond)
	
	handler := NewLoggingHandler()
	processor.AddHandler(handler)
	
	err := processor.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer processor.Stop()
	
	// Send events that should trigger batch processing
	for i := 0; i < 5; i++ {
		event := NewMockEvent(fmt.Sprintf("batch-event-%d", i), MockEventTypeTest)
		processor.SendEvent(event)
	}
	
	// Give time for batch processing
	time.Sleep(150 * time.Millisecond)
	
	metrics := processor.GetMetrics()
	if metrics.EventsProcessed != 5 {
		t.Errorf("Expected 5 processed events, got %d", metrics.EventsProcessed)
	}
}

func TestEventReplay(t *testing.T) {
	utils := NewEventUtils()
	
	// Create some test events
	events := []Event{
		NewMockEvent("replay-1", MockEventTypeTest),
		NewMockEvent("replay-2", MockEventTypeProcess),
		NewMockEvent("replay-3", MockEventTypeError),
	}
	
	replay := utils.CreateEventReplay(events)
	if replay == nil {
		t.Fatal("CreateEventReplay returned nil")
	}
	
	t.Run("BasicReplay", func(t *testing.T) {
		subscriber := replay.Subscribe()
		
		err := replay.Play()
		if err != nil {
			t.Fatalf("Play failed: %v", err)
		}
		
		// Collect replayed events
		var replayed []Event
		timeout := time.After(1 * time.Second)
		
		for i := 0; i < len(events); i++ {
			select {
			case event := <-subscriber:
				replayed = append(replayed, event)
			case <-timeout:
				t.Fatal("Timeout waiting for replayed events")
			}
		}
		
		if len(replayed) != len(events) {
			t.Errorf("Expected %d replayed events, got %d", len(events), len(replayed))
		}
		
		// Verify event order
		for i, event := range replayed {
			if event.GetBaseEvent().ID() != events[i].GetBaseEvent().ID() {
				t.Errorf("Event %d order mismatch: expected %s, got %s", i, events[i].GetBaseEvent().ID(), event.GetBaseEvent().ID())
			}
		}
		
		replay.Stop()
	})
	
	t.Run("PauseResume", func(t *testing.T) {
		newReplay := utils.CreateEventReplay(events)
		subscriber := newReplay.Subscribe()
		
		err := newReplay.Play()
		if err != nil {
			t.Fatalf("Play failed: %v", err)
		}
		
		// Pause after a short time
		time.Sleep(10 * time.Millisecond)
		err = newReplay.Pause()
		if err != nil {
			t.Errorf("Pause failed: %v", err)
		}
		
		// Should not receive more events while paused
		select {
		case <-subscriber:
			// This is okay - might have received events before pause
		case <-time.After(50 * time.Millisecond):
			// This is expected while paused
		}
		
		newReplay.Stop()
	})
	
	t.Run("SetSpeed", func(t *testing.T) {
		err := replay.SetSpeed(2.0)
		if err != nil {
			t.Errorf("SetSpeed failed: %v", err)
		}
		
		err = replay.SetSpeed(0)
		if err == nil {
			t.Error("Expected error for zero speed")
		}
		
		err = replay.SetSpeed(-1)
		if err == nil {
			t.Error("Expected error for negative speed")
		}
	})
}

func TestEventFilters(t *testing.T) {
	t.Run("EventTypeFilter", func(t *testing.T) {
		filter := NewEventTypeFilter(MockEventTypeTest, MockEventTypeProcess)
		
		testEvent := NewMockEvent("test", MockEventTypeTest)
		processEvent := NewMockEvent("process", MockEventTypeProcess)
		errorEvent := NewMockEvent("error", MockEventTypeError)
		
		if !filter.Apply(testEvent) {
			t.Error("Test event should pass filter")
		}
		if !filter.Apply(processEvent) {
			t.Error("Process event should pass filter")
		}
		if filter.Apply(errorEvent) {
			t.Error("Error event should not pass filter")
		}
	})
	
	t.Run("TimeRangeEventFilter", func(t *testing.T) {
		now := time.Now()
		start := now.Add(-1 * time.Hour)
		end := now.Add(1 * time.Hour)
		
		filter := NewTimeRangeEventFilter(start, end)
		
		// Create event with timestamp in range
		recentTime := now.UnixMilli()
		recentEvent := &MockEvent{
			id:        "recent",
			eventType: MockEventTypeTest,
			timestamp: &recentTime,
		}
		
		// Create event with timestamp out of range
		oldTime := now.Add(-2 * time.Hour).UnixMilli()
		oldEvent := &MockEvent{
			id:        "old",
			eventType: MockEventTypeTest,
			timestamp: &oldTime,
		}
		
		if !filter.Apply(recentEvent) {
			t.Error("Recent event should pass time filter")
		}
		if filter.Apply(oldEvent) {
			t.Error("Old event should not pass time filter")
		}
	})
	
	t.Run("ContentEventFilter", func(t *testing.T) {
		filter := NewContentEventFilter(func(event Event) bool {
			// Cast to MockEvent to access Data method
			if mockEvent, ok := event.(*MockEvent); ok {
				data := mockEvent.Data()
				content, exists := data["content"]
				if !exists {
					return false
				}
				str, ok := content.(string)
				return ok && strings.Contains(str, "important")
			}
			return false
		})
		
		importantEvent := NewMockEvent("important", MockEventTypeTest)
		importantEvent.SetData("content", "this is important")
		
		normalEvent := NewMockEvent("normal", MockEventTypeTest)
		normalEvent.SetData("content", "this is normal")
		
		if !filter.Apply(importantEvent) {
			t.Error("Important event should pass content filter")
		}
		if filter.Apply(normalEvent) {
			t.Error("Normal event should not pass content filter")
		}
	})
}

func TestEventHandlers(t *testing.T) {
	t.Run("LoggingHandler", func(t *testing.T) {
		handler := NewLoggingHandler()
		event := NewMockEvent("log-test", MockEventTypeTest)
		
		result, err := handler.Handle(context.Background(), event)
		if err != nil {
			t.Errorf("LoggingHandler failed: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("Expected 1 result event, got %d", len(result))
		}
		if result[0] != event {
			t.Error("LoggingHandler should return same event")
		}
	})
	
	t.Run("MetricsHandler", func(t *testing.T) {
		metrics := &EventMetrics{
			TotalProcessed: 0,
		}
		handler := NewMetricsHandler(metrics)
		event := NewMockEvent("metrics-test", MockEventTypeTest)
		
		initialCount := metrics.TotalProcessed
		result, err := handler.Handle(context.Background(), event)
		if err != nil {
			t.Errorf("MetricsHandler failed: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("Expected 1 result event, got %d", len(result))
		}
		if metrics.TotalProcessed != initialCount+1 {
			t.Error("MetricsHandler should increment processed count")
		}
	})
	
	t.Run("TransformHandler", func(t *testing.T) {
		transformer := func(event Event) ([]Event, error) {
			// Create two events from one
			event1 := NewMockEvent(event.GetBaseEvent().ID()+"-1", event.Type())
			event2 := NewMockEvent(event.GetBaseEvent().ID()+"-2", event.Type())
			return []Event{event1, event2}, nil
		}
		
		handler := NewTransformHandler(transformer)
		event := NewMockEvent("transform-test", MockEventTypeTest)
		
		result, err := handler.Handle(context.Background(), event)
		if err != nil {
			t.Errorf("TransformHandler failed: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("Expected 2 result events, got %d", len(result))
		}
	})
}

// Concurrency and race condition tests

func TestEventProcessor_ConcurrentAccess(t *testing.T) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("concurrent-test", 1000)
	
	handler := NewLoggingHandler()
	processor.AddHandler(handler)
	
	err := processor.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer processor.Stop()
	
	var wg sync.WaitGroup
	numRoutines := 10
	eventsPerRoutine := 50
	
	// Concurrent event sending
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerRoutine; j++ {
				event := NewMockEvent(fmt.Sprintf("event-%d-%d", routineID, j), MockEventTypeTest)
				processor.SendEvent(event)
			}
		}(i)
	}
	
	// Concurrent metrics reading
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				processor.GetMetrics()
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}
	
	wg.Wait()
	
	// Give time for all events to be processed
	time.Sleep(200 * time.Millisecond)
	
	metrics := processor.GetMetrics()
	expectedEvents := int64(numRoutines * eventsPerRoutine)
	
	if metrics.EventsProcessed < expectedEvents {
		t.Errorf("Expected at least %d processed events, got %d", expectedEvents, metrics.EventsProcessed)
	}
}

func TestEventReplay_ConcurrentSubscribers(t *testing.T) {
	utils := NewEventUtils()
	
	// Create test events
	numEvents := 20
	events := make([]Event, numEvents)
	for i := 0; i < numEvents; i++ {
		events[i] = NewMockEvent(fmt.Sprintf("concurrent-event-%d", i), MockEventTypeTest)
	}
	
	replay := utils.CreateEventReplay(events)
	
	var wg sync.WaitGroup
	var subscribeWg sync.WaitGroup
	numSubscribers := 5
	results := make([][]Event, numSubscribers)
	
	// Create multiple subscribers
	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		subscribeWg.Add(1)
		go func(subscriberID int) {
			defer wg.Done()
			
			subscriber := replay.Subscribe()
			subscribeWg.Done() // Signal that subscription is complete
			var received []Event
			
			timeout := time.After(2 * time.Second)
			for {
				select {
				case event, ok := <-subscriber:
					if !ok {
						results[subscriberID] = received
						return
					}
					received = append(received, event)
					if len(received) >= numEvents {
						results[subscriberID] = received
						return
					}
				case <-timeout:
					results[subscriberID] = received
					return
				}
			}
		}(i)
	}
	
	// Wait for all subscribers to be registered before starting replay
	subscribeWg.Wait()
	
	// Start replay
	err := replay.Play()
	if err != nil {
		t.Fatalf("Play failed: %v", err)
	}
	
	wg.Wait()
	replay.Stop()
	
	// Verify all subscribers received events
	for i, result := range results {
		if len(result) == 0 {
			t.Errorf("Subscriber %d received no events", i)
		}
	}
}

// Memory leak tests

func TestMemoryLeak_EventProcessor(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	utils := NewEventUtils()
	
	// Create and destroy many processors
	for i := 0; i < 100; i++ {
		processor := utils.CreateProcessor(fmt.Sprintf("leak-test-%d", i), 50)
		
		// Start processor
		processor.Start()
		
		// Send some events
		for j := 0; j < 10; j++ {
			event := NewMockEvent(fmt.Sprintf("leak-event-%d-%d", i, j), MockEventTypeTest)
			processor.SendEvent(event)
		}
		
		// Stop processor quickly
		time.Sleep(10 * time.Millisecond)
		processor.Stop()
	}
	
	// Force garbage collection
	runtime.GC()
	
	// Check that we don't have excessive number of processors
	utils.processorsMu.RLock()
	processorCount := len(utils.processors)
	utils.processorsMu.RUnlock()
	
	if processorCount > 150 { // Allow some leeway
		t.Errorf("Possible memory leak: %d processors still registered", processorCount)
	}
}

func TestMemoryLeak_EventReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	utils := NewEventUtils()
	
	// Create many replays with subscribers
	for i := 0; i < 50; i++ {
		events := []Event{
			NewMockEvent(fmt.Sprintf("replay-leak-%d-1", i), MockEventTypeTest),
			NewMockEvent(fmt.Sprintf("replay-leak-%d-2", i), MockEventTypeTest),
		}
		
		replay := utils.CreateEventReplay(events)
		
		// Create multiple subscribers
		subscribers := make([]<-chan Event, 3)
		for j := 0; j < 3; j++ {
			subscribers[j] = replay.Subscribe()
		}
		
		// Start and quickly stop
		replay.Play()
		time.Sleep(5 * time.Millisecond)
		replay.Stop()
		
		// Try to drain subscribers
		for _, sub := range subscribers {
			select {
			case <-sub:
			default:
			}
		}
	}
	
	runtime.GC()
	// This test mainly ensures we don't panic or create obvious leaks
}

// Channel safety tests

func TestChannelSafety_EventProcessor(t *testing.T) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("channel-safety", 100)
	
	// Start and stop processor multiple times rapidly
	for i := 0; i < 10; i++ {
		err := processor.Start()
		if err != nil && i == 0 {
			t.Fatalf("Start failed: %v", err)
		}
		
		// Send an event
		if processor.isRunning.Load() {
			event := NewMockEvent(fmt.Sprintf("safety-event-%d", i), MockEventTypeTest)
			processor.SendEvent(event)
		}
		
		time.Sleep(10 * time.Millisecond)
		
		err = processor.Stop()
		if err != nil {
			t.Logf("Stop failed on iteration %d: %v", i, err)
		}
	}
}

func TestChannelSafety_EventReplay(t *testing.T) {
	utils := NewEventUtils()
	
	events := []Event{
		NewMockEvent("safety-1", MockEventTypeTest),
		NewMockEvent("safety-2", MockEventTypeTest),
	}
	
	replay := utils.CreateEventReplay(events)
	
	// Create subscriber and immediately start/stop multiple times
	subscriber := replay.Subscribe()
	
	for i := 0; i < 5; i++ {
		replay.Play()
		time.Sleep(5 * time.Millisecond)
		replay.Stop()
		
		// Try to read from subscriber without blocking
		select {
		case <-subscriber:
		default:
		}
	}
}

// Performance regression tests

func TestPerformanceRegression_EventThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}
	
	utils := NewEventUtils()
	processor := utils.CreateProcessor("throughput-test", 10000)
	
	// Add minimal handler to avoid bottlenecks
	handler := NewLoggingHandler()
	processor.AddHandler(handler)
	
	err := processor.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer processor.Stop()
	
	numEvents := 1000
	start := time.Now()
	
	// Send events as fast as possible
	for i := 0; i < numEvents; i++ {
		event := NewMockEvent(fmt.Sprintf("perf-event-%d", i), MockEventTypeTest)
		err := processor.SendEvent(event)
		if err != nil {
			t.Errorf("SendEvent %d failed: %v", i, err)
		}
	}
	
	sendTime := time.Since(start)
	
	// Wait for processing to complete
	time.Sleep(500 * time.Millisecond)
	
	totalTime := time.Since(start)
	metrics := processor.GetMetrics()
	
	eventsPerSec := float64(numEvents) / sendTime.Seconds()
	processedPerSec := float64(metrics.EventsProcessed) / totalTime.Seconds()
	
	t.Logf("Event throughput: %.0f events/sec sent, %.0f events/sec processed", eventsPerSec, processedPerSec)
	
	// Performance expectations
	if eventsPerSec < 1000 {
		t.Errorf("Event sending too slow: %.0f events/sec", eventsPerSec)
	}
	if metrics.EventsProcessed < int64(float64(numEvents)*0.9) { // Allow 10% loss
		t.Errorf("Too many events lost: %d processed out of %d sent", metrics.EventsProcessed, numEvents)
	}
}

// Benchmark tests

func BenchmarkEventProcessor_SendEvent(b *testing.B) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("bench-send", 10000)
	processor.Start()
	defer processor.Stop()
	
	event := NewMockEvent("bench-event", MockEventTypeTest)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.SendEvent(event)
	}
}

func BenchmarkEventProcessor_ProcessEvent(b *testing.B) {
	utils := NewEventUtils()
	processor := utils.CreateProcessor("bench-process", 1000)
	
	handler := NewLoggingHandler()
	processor.AddHandler(handler)
	
	processor.Start()
	defer processor.Stop()
	
	events := make([]Event, b.N)
	for i := 0; i < b.N; i++ {
		events[i] = NewMockEvent(fmt.Sprintf("bench-event-%d", i), MockEventTypeTest)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.SendEvent(events[i])
	}
}

func BenchmarkEventFilter_Apply(b *testing.B) {
	filter := NewEventTypeFilter(MockEventTypeTest, MockEventTypeProcess)
	event := NewMockEvent("bench-filter", MockEventTypeTest)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter.Apply(event)
	}
}

func BenchmarkEventReplay_Subscribe(b *testing.B) {
	utils := NewEventUtils()
	
	events := make([]Event, 100)
	for i := 0; i < 100; i++ {
		events[i] = NewMockEvent(fmt.Sprintf("bench-replay-%d", i), MockEventTypeTest)
	}
	
	replay := utils.CreateEventReplay(events)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		subscriber := replay.Subscribe()
		_ = subscriber
	}
}