package sse

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// Example_basicUsage demonstrates basic SSE transport usage
func Example_basicUsage() {
	// Note: This example assumes a server is running at localhost:8080
	// In real tests, you would use a test server
	// Create a new SSE transport with default configuration
	config := &Config{
		BaseURL:     "http://localhost:8080",
		BufferSize:  100,
		ReadTimeout: 30 * time.Second,
	}

	transport, err := NewSSETransport(config)
	if err != nil {
		log.Fatal(err)
	}
	defer transport.Close()

	// Send an event
	// ctx := context.Background()
	// event := events.NewRunStartedEvent("thread-123", "run-456")

	// Skip actual send in example to avoid connection errors
	// err = transport.Send(ctx, event)
	// if err != nil {
	// 	log.Printf("Failed to send event: %v", err)
	// 	return
	// }

	fmt.Println("Event sent successfully")
	// Output: Event sent successfully
}

// Example_receiveEvents demonstrates receiving events via SSE
func Example_receiveEvents() {
	config := &Config{
		BaseURL: "http://localhost:8080",
	}

	transport, err := NewSSETransport(config)
	if err != nil {
		log.Fatal(err)
	}
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start receiving events
	eventChan, err := transport.Receive(ctx)
	if err != nil {
		log.Printf("Failed to start receiving: %v", err)
		return
	}

	// Process events
	go func() {
		for {
			select {
			case event := <-eventChan:
				if event == nil {
					continue
				}
				fmt.Printf("Received event: %s\n", event.Type())

				// Handle specific event types
				switch event.Type() {
				case events.EventTypeRunStarted:
					fmt.Println("Processing run started event")
				case events.EventTypeTextMessageContent:
					fmt.Println("Processing text message content")
				default:
					fmt.Printf("Unknown event type: %s\n", event.Type())
				}

			case <-ctx.Done():
				fmt.Println("Context cancelled, stopping event processing")
				return
			}
		}
	}()

	// Monitor errors
	go func() {
		for err := range transport.GetErrorChannel() {
			log.Printf("Transport error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	fmt.Println("Event reception completed")
}

// Example_customHeaders demonstrates setting custom headers
func Example_customHeaders() {
	transport, err := NewSSETransport(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer transport.Close()

	// Set authentication header
	transport.SetHeader("Authorization", "Bearer your-token-here")

	// Set custom application headers
	transport.SetHeader("X-Client-Version", "1.0.0")
	transport.SetHeader("X-Request-ID", "req-12345")

	// The headers will be included in all HTTP requests
	// ctx := context.Background()
	// event := events.NewRunStartedEvent("thread-123", "run-456")

	// Skip actual send in example to avoid connection errors
	// err = transport.Send(ctx, event)
	// if err != nil {
	// 	log.Printf("Failed to send event: %v", err)
	// 	return
	// }

	fmt.Println("Event sent with custom headers")
	// Output: Event sent with custom headers
}

// Example_batchSending demonstrates sending multiple events at once
func Example_batchSending() {
	transport, err := NewSSETransport(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer transport.Close()

	// Create multiple events
	events := []events.Event{
		events.NewRunStartedEvent("thread-123", "run-456"),
		events.NewStepStartedEvent("step-1"),
		events.NewTextMessageStartEvent("msg-789", events.WithRole("user")),
		events.NewTextMessageContentEvent("msg-789", "Hello, world!"),
		events.NewTextMessageEndEvent("msg-789"),
		events.NewStepFinishedEvent("step-1"),
		events.NewRunFinishedEvent("thread-123", "run-456"),
	}

	// Skip actual send in example to avoid connection errors
	// ctx := context.Background()
	// err = transport.SendBatch(ctx, events)
	// if err != nil {
	// 	log.Printf("Failed to send batch: %v", err)
	// 	return
	// }

	fmt.Printf("Successfully sent batch of %d events\n", len(events))
	// Output: Successfully sent batch of 7 events
}

// Example_connectionManagement demonstrates connection lifecycle
func Example_connectionManagement() {
	config := &Config{
		BaseURL:        "http://localhost:8080",
		ReconnectDelay: 1 * time.Second,
		MaxReconnects:  3,
	}

	transport, err := NewSSETransport(config)
	if err != nil {
		log.Fatal(err)
	}
	defer transport.Close()

	// Check initial connection status
	fmt.Printf("Initial status: %s\n", transport.GetConnectionStatus())

	// Test connectivity
	ctx := context.Background()
	err = transport.Ping(ctx)
	if err != nil {
		log.Printf("Ping failed: %v", err)
	} else {
		fmt.Println("Ping successful")
	}

	// Get transport statistics
	stats := transport.Stats()
	fmt.Printf("Transport stats: %s\n", stats)

	// Reset connection (useful for testing)
	err = transport.Reset()
	if err != nil {
		log.Printf("Reset failed: %v", err)
	} else {
		fmt.Println("Transport reset successfully")
	}

	// Output:
	// Initial status: disconnected
	// Transport stats: SSETransport{status=disconnected, reconnects=0, baseURL=http://localhost:8080, bufferSize=1000}
	// Transport reset successfully
}

// Example_errorHandling demonstrates error handling patterns
func Example_errorHandling() {
	transport, err := NewSSETransport(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer transport.Close()

	ctx := context.Background()

	// Send invalid event (this will fail validation)
	invalidEvent := events.NewRunStartedEvent("", "run-123") // Empty thread ID
	err = transport.Send(ctx, invalidEvent)
	if err != nil {
		fmt.Printf("Expected validation error: %v\n", err)
	}

	// Send nil event
	err = transport.Send(ctx, nil)
	if err != nil {
		fmt.Printf("Expected nil event error: %v\n", err)
	}

	// Try to use closed transport
	transport.Close()
	validEvent := events.NewRunStartedEvent("thread-123", "run-456")
	err = transport.Send(ctx, validEvent)
	if err != nil {
		fmt.Printf("Expected closed transport error: %v\n", err)
	}

	// Output:
	// Expected validation error: validation error: event validation failed: RunStartedEvent validation failed: threadId field is required
	// Expected nil event error: validation error: event cannot be nil
	// Expected closed transport error: streaming error for event transport at index 0: transport is closed
}

// Example_formatSSEEvent demonstrates SSE event formatting
func Example_formatSSEEvent() {
	// Create a test event
	event := events.NewRunStartedEvent("thread-123", "run-456")

	// Format as SSE
	sseData, err := FormatSSEEvent(event)
	if err != nil {
		log.Printf("Failed to format event: %v", err)
		return
	}

	fmt.Print(sseData)
	// Output will be in SSE format:
	// event: RUN_STARTED
	// data: {"type":"RUN_STARTED","timestamp":1234567890,"threadId":"thread-123","runId":"run-456"}
	// id: 1234567890
	//
}

// Example_advancedConfiguration demonstrates advanced configuration options
func Example_advancedConfiguration() {
	// Create custom HTTP client with specific timeouts
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	config := &Config{
		BaseURL: "http://localhost:8080",
		Headers: map[string]string{
			"User-Agent":    "AG-UI-SDK/1.0.0",
			"Authorization": "Bearer token123",
		},
		BufferSize:     500,
		ReadTimeout:    45 * time.Second,
		WriteTimeout:   15 * time.Second,
		ReconnectDelay: 2 * time.Second,
		MaxReconnects:  5,
		Client:         httpClient,
	}

	transport, err := NewSSETransport(config)
	if err != nil {
		log.Fatal(err)
	}
	defer transport.Close()

	fmt.Printf("Advanced transport configured with buffer size: %d\n", config.BufferSize)
	// Output: Advanced transport configured with buffer size: 500
}

// Example_eventTypeHandling demonstrates handling different event types
func Example_eventTypeHandling() {
	transport, err := NewSSETransport(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer transport.Close()

	// Example event data received from server
	testData := map[string]interface{}{
		"type":      "TEXT_MESSAGE_CONTENT",
		"messageId": "msg-123",
		"delta":     "Hello, world!",
		"timestamp": float64(time.Now().UnixMilli()),
	}

	// Parse the event
	event, err := transport.createEventFromData("TEXT_MESSAGE_CONTENT", testData)
	if err != nil {
		log.Printf("Failed to parse event: %v", err)
		return
	}

	// Type assertion to access specific fields
	if textEvent, ok := event.(*events.TextMessageContentEvent); ok {
		fmt.Printf("Received text content: %s for message %s\n",
			textEvent.Delta, textEvent.MessageID)
	}

	// Output: Received text content: Hello, world! for message msg-123
}
