// +build ignore

package transport

import (
	"context"
	"fmt"
	"log"
	"time"
)

// DemoBackpressure demonstrates the backpressure functionality
func DemoBackpressure() {
	fmt.Println("=== Backpressure Demo ===")
	
	// Demo 1: Drop Oldest Strategy
	fmt.Println("\n1. Drop Oldest Strategy:")
	demoDropOldest()
	
	// Demo 2: Drop Newest Strategy
	fmt.Println("\n2. Drop Newest Strategy:")
	demoDropNewest()
	
	// Demo 3: Block with Timeout Strategy
	fmt.Println("\n3. Block with Timeout Strategy:")
	demoBlockTimeout()
	
	// Demo 4: Manager Integration
	fmt.Println("\n4. Manager Integration:")
	demoManagerIntegration()
}

func demoDropOldest() {
	config := BackpressureConfig{
		Strategy:      BackpressureDropOldest,
		BufferSize:    3,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  time.Second,
		EnableMetrics: true,
	}
	
	handler := NewBackpressureHandler(config)
	defer handler.Stop()
	
	fmt.Printf("  - Buffer size: %d\n", config.BufferSize)
	
	// Send more events than buffer can hold
	for i := 1; i <= 5; i++ {
		// Use type-safe data event creation
		dataEvent := CreateDataEvent(fmt.Sprintf("event-%d", i), 
			[]byte(fmt.Sprintf("backpressure test data %d", i)),
			func(data *DataEventData) {
				data.ContentType = "text/plain"
				data.SequenceNumber = uint64(i)
			},
		)
		
		event := Event{
			Event: NewTransportEventAdapter(dataEvent),
		}
		
		err := handler.SendEvent(event)
		if err != nil {
			fmt.Printf("  - Error sending event %d: %v\n", i, err)
		} else {
			fmt.Printf("  - Sent event %d\n", i)
		}
	}
	
	// Read available events
	fmt.Printf("  - Available events: ")
	for {
		select {
		case event := <-handler.EventChan():
			fmt.Printf("%s ", event.Event.ID())
		case <-time.After(100 * time.Millisecond):
			fmt.Println()
			break
		}
	}
	
	// Show metrics
	metrics := handler.GetMetrics()
	fmt.Printf("  - Events dropped: %d\n", metrics.EventsDropped)
}

func demoDropNewest() {
	config := BackpressureConfig{
		Strategy:      BackpressureDropNewest,
		BufferSize:    3,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  time.Second,
		EnableMetrics: true,
	}
	
	handler := NewBackpressureHandler(config)
	defer handler.Stop()
	
	fmt.Printf("  - Buffer size: %d\n", config.BufferSize)
	
	// Send more events than buffer can hold
	for i := 1; i <= 5; i++ {
		// Use type-safe data event creation
		dataEvent := CreateDataEvent(fmt.Sprintf("event-%d", i), 
			[]byte(fmt.Sprintf("backpressure test data %d", i)),
			func(data *DataEventData) {
				data.ContentType = "text/plain"
				data.SequenceNumber = uint64(i)
			},
		)
		
		event := Event{
			Event: NewTransportEventAdapter(dataEvent),
		}
		
		err := handler.SendEvent(event)
		if err != nil {
			fmt.Printf("  - Error sending event %d: %v\n", i, err)
		} else {
			fmt.Printf("  - Sent event %d\n", i)
		}
	}
	
	// Read available events
	fmt.Printf("  - Available events: ")
	for {
		select {
		case event := <-handler.EventChan():
			fmt.Printf("%s ", event.Event.ID())
		case <-time.After(100 * time.Millisecond):
			fmt.Println()
			break
		}
	}
	
	// Show metrics
	metrics := handler.GetMetrics()
	fmt.Printf("  - Events dropped: %d\n", metrics.EventsDropped)
}

func demoBlockTimeout() {
	config := BackpressureConfig{
		Strategy:      BackpressureBlockWithTimeout,
		BufferSize:    2,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  200 * time.Millisecond,
		EnableMetrics: true,
	}
	
	handler := NewBackpressureHandler(config)
	defer handler.Stop()
	
	fmt.Printf("  - Buffer size: %d, timeout: %v\n", config.BufferSize, config.BlockTimeout)
	
	// Fill buffer
	for i := 1; i <= 2; i++ {
		// Use type-safe data event creation
		dataEvent := CreateDataEvent(fmt.Sprintf("event-%d", i), 
			[]byte(fmt.Sprintf("block timeout test data %d", i)),
			func(data *DataEventData) {
				data.ContentType = "text/plain"
				data.SequenceNumber = uint64(i)
			},
		)
		
		event := Event{
			Event: NewTransportEventAdapter(dataEvent),
		}
		
		err := handler.SendEvent(event)
		if err != nil {
			fmt.Printf("  - Error sending event %d: %v\n", i, err)
		} else {
			fmt.Printf("  - Sent event %d\n", i)
		}
	}
	
	// This should timeout
	start := time.Now()
	// Use type-safe error event for timeout test
	errorEvent := CreateErrorEvent("timeout-event", "timeout test message",
		func(data *ErrorEventData) {
			data.Code = "TIMEOUT_TEST"
			data.Severity = "warning"
			data.Category = "backpressure"
			data.Retryable = true
		},
	)
	
	event := Event{
		Event: NewTransportEventAdapter(errorEvent),
	}
	
	err := handler.SendEvent(event)
	elapsed := time.Since(start)
	
	if err != nil {
		fmt.Printf("  - Event timed out after %v: %v\n", elapsed, err)
	} else {
		fmt.Printf("  - Unexpected success after %v\n", elapsed)
	}
	
	// Show metrics
	metrics := handler.GetMetrics()
	fmt.Printf("  - Events blocked: %d\n", metrics.EventsBlocked)
}

func demoManagerIntegration() {
	config := BackpressureConfig{
		Strategy:      BackpressureDropOldest,
		BufferSize:    3,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  time.Second,
		EnableMetrics: true,
	}
	
	manager := NewSimpleManagerWithBackpressure(config)
	defer manager.Stop(context.Background())
	
	fmt.Printf("  - Created manager with backpressure strategy: %s\n", config.Strategy)
	
	// Check that channels are properly wired
	eventChan := manager.Receive()
	errorChan := manager.Errors()
	
	if eventChan != nil {
		fmt.Printf("  - Event channel available: %T\n", eventChan)
	}
	
	if errorChan != nil {
		fmt.Printf("  - Error channel available: %T\n", errorChan)
	}
	
	// Get backpressure metrics
	metrics := manager.GetBackpressureMetrics()
	fmt.Printf("  - Max buffer size: %d\n", metrics.MaxBufferSize)
	fmt.Printf("  - Current buffer size: %d\n", metrics.CurrentBufferSize)
	fmt.Printf("  - Events dropped: %d\n", metrics.EventsDropped)
	
	fmt.Printf("  - Manager integration successful!\n")
}

// Example usage function
func ExampleBackpressureUsage() {
	// Simple usage with drop oldest strategy
	manager := NewSimpleManagerWithBackpressure(BackpressureConfig{
		Strategy:      BackpressureDropOldest,
		BufferSize:    1000,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  5 * time.Second,
		EnableMetrics: true,
	})
	
	// Start the manager
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		log.Printf("Failed to start manager: %v", err)
		return
	}
	defer manager.Stop(ctx)
	
	// Monitor backpressure metrics
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				metrics := manager.GetBackpressureMetrics()
				if metrics.EventsDropped > 0 {
					log.Printf("Backpressure active: %d events dropped", metrics.EventsDropped)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	
	// Use the manager normally
	eventChan := manager.Receive()
	go func() {
		for event := range eventChan {
			// Process events
			log.Printf("Processing event: %s", event.Event.ID())
		}
	}()
	
	// The manager will now handle backpressure automatically
	fmt.Println("Manager is running with backpressure handling...")
}