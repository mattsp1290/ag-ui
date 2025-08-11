package sse_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/examples/client/internal/sse"
	"github.com/sirupsen/logrus"
)

// ExampleReconnectingClient demonstrates using the SSE client with automatic reconnection
func ExampleReconnectingClient() {
	// Configure the basic SSE client
	config := sse.Config{
		Endpoint:       "https://api.example.com/events",
		APIKey:         "your-api-key",
		AuthHeader:     "Authorization",
		ConnectTimeout: 30 * time.Second,
		ReadTimeout:    5 * time.Minute,
		BufferSize:     100,
		Logger:         logrus.New(),
	}

	// Configure reconnection behavior
	reconnectConfig := sse.ReconnectionConfig{
		Enabled:           true,
		InitialDelay:      250 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.2,
		MaxRetries:        0,                      // Unlimited retries
		MaxElapsedTime:    24 * time.Hour,         // Give up after 24 hours
		ResetInterval:     60 * time.Second,       // Reset backoff after 60s of stable connection
		IdleTimeout:       5 * time.Minute,        // Reconnect if no data for 5 minutes
	}

	// Create client with reconnection support
	client := sse.NewReconnectingClient(config, reconnectConfig)
	defer client.Close()

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start streaming with automatic reconnection
	frames, errors, err := client.StreamWithReconnect(sse.StreamOptions{
		Context: ctx,
		Payload: map[string]interface{}{
			"filter": "important-events",
		},
	})

	if err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}

	// Process events
	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				// Stream ended
				fmt.Println("Stream closed")
				return
			}
			
			// Process the SSE frame
			fmt.Printf("Received: %s\n", frame.Data)

		case err, ok := <-errors:
			if !ok {
				// Error channel closed
				return
			}
			
			// Handle non-retryable errors
			fmt.Printf("Stream error: %v\n", err)
			return

		case <-time.After(10 * time.Minute):
			// Timeout - stop streaming
			fmt.Println("Timeout reached, stopping stream")
			cancel()
			return
		}
	}
}

// ExampleReconnectingClient_withMetrics demonstrates monitoring reconnection statistics
func ExampleReconnectingClient_withMetrics() {
	config := sse.Config{
		Endpoint: "https://api.example.com/events",
		APIKey:   "your-api-key",
		Logger:   logrus.New(),
	}

	reconnectConfig := sse.DefaultReconnectionConfig()
	client := sse.NewReconnectingClient(config, reconnectConfig)
	defer client.Close()

	ctx := context.Background()
	frames, errors, _ := client.StreamWithReconnect(sse.StreamOptions{
		Context: ctx,
	})

	// Monitor connection statistics
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			stats := client.GetStats()
			fmt.Printf("Connection stats: %+v\n", stats)
		}
	}()

	// Process events
	for {
		select {
		case frame := <-frames:
			if frame.Data != nil {
				fmt.Printf("Event at %s: %s\n", frame.Timestamp, frame.Data)
			}
		case err := <-errors:
			if err != nil {
				log.Printf("Error: %v", err)
				return
			}
		}
	}
}

// ExampleReconnectingClient_customBackoff demonstrates custom backoff configuration
func ExampleReconnectingClient_customBackoff() {
	config := sse.Config{
		Endpoint: "https://api.example.com/events",
		APIKey:   "your-api-key",
	}

	// Aggressive reconnection for critical real-time data
	reconnectConfig := sse.ReconnectionConfig{
		Enabled:           true,
		InitialDelay:      100 * time.Millisecond,  // Start fast
		MaxDelay:          5 * time.Second,          // Cap at 5 seconds
		BackoffMultiplier: 1.5,                      // Slower growth
		JitterFactor:      0.3,                      // More jitter to spread load
		MaxRetries:        100,                      // Limit attempts
		MaxElapsedTime:    10 * time.Minute,         // Give up after 10 minutes
		ResetInterval:     30 * time.Second,         // Quick reset
		IdleTimeout:       30 * time.Second,         // Detect disconnects quickly
	}

	client := sse.NewReconnectingClient(config, reconnectConfig)
	defer client.Close()

	// Use the client...
	ctx := context.Background()
	frames, errors, err := client.StreamWithReconnect(sse.StreamOptions{
		Context: ctx,
	})

	if err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}

	// Process frames and errors...
	_ = frames
	_ = errors
}

// ExampleReconnectingClient_withLastEventID demonstrates using Last-Event-ID for resumption
func ExampleReconnectingClient_withLastEventID() {
	config := sse.Config{
		Endpoint: "https://api.example.com/events",
		APIKey:   "your-api-key",
	}

	reconnectConfig := sse.DefaultReconnectionConfig()
	client := sse.NewReconnectingClient(config, reconnectConfig)
	defer client.Close()

	// Track last event ID for resumption
	var lastEventID string

	ctx := context.Background()
	frames, errors, _ := client.StreamWithReconnect(sse.StreamOptions{
		Context: ctx,
	})

	for {
		select {
		case frame := <-frames:
			// Extract event ID from frame if available
			// This would depend on your event format
			if eventID := extractEventID(frame.Data); eventID != "" {
				lastEventID = eventID
				client.SetLastEventID(lastEventID)
			}
			
			fmt.Printf("Received event (ID: %s): %s\n", lastEventID, frame.Data)

		case err := <-errors:
			if err != nil {
				log.Printf("Error: %v", err)
				return
			}
		}
	}
}

// Helper function to extract event ID (implementation depends on your protocol)
func extractEventID(data []byte) string {
	// This is a placeholder - actual implementation would parse the event
	// to extract the ID field if present
	return ""
}