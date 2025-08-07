package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

func main() {
	// Example SSE endpoint - replace with your actual endpoint
	sseURL := "https://api.example.com/events"
	if len(os.Args) > 1 {
		sseURL = os.Args[1]
	}

	fmt.Printf("Connecting to SSE endpoint: %s\n", sseURL)

	// Configure the SSE client
	config := client.SSEClientConfig{
		URL: sseURL,

		// Connection settings
		InitialBackoff:       time.Second,
		MaxBackoff:           30 * time.Second,
		BackoffMultiplier:    2.0,
		MaxReconnectAttempts: 0, // Unlimited reconnection attempts

		// Buffer and flow control
		EventBufferSize:      1000,
		FlowControlEnabled:   true,
		FlowControlThreshold: 0.8,

		// Timeouts
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        10 * time.Second,
		HealthCheckInterval: 30 * time.Second,

		// Headers
		Headers: map[string]string{
			"Authorization": "Bearer your-token-here",
			"User-Agent":    "AG-UI SSE Example/1.0",
		},

		// TLS configuration
		TLSConfig: &tls.Config{
			InsecureSkipVerify: false, // Set to true only for testing
		},

		// Event filtering - only process certain event types
		EventFilter: func(eventType string) bool {
			allowedTypes := map[string]bool{
				"TEXT_MESSAGE_START":   true,
				"TEXT_MESSAGE_CONTENT": true,
				"TEXT_MESSAGE_END":     true,
				"TOOL_CALL_START":      true,
				"TOOL_CALL_END":        true,
				"STATE_SNAPSHOT":       true,
				"custom":               true,
			}
			return allowedTypes[eventType] || eventType == ""
		},

		// Callback functions
		OnConnect: func() {
			fmt.Println("✅ Connected to SSE stream")
		},

		OnDisconnect: func(err error) {
			fmt.Printf("❌ Disconnected from SSE stream: %v\n", err)
		},

		OnReconnect: func(attempt int) {
			fmt.Printf("🔄 Reconnection attempt #%d\n", attempt)
		},

		OnError: func(err error) {
			fmt.Printf("⚠️  SSE Error: %v\n", err)
		},
	}

	// Create the SSE client
	sseClient, err := client.NewSSEClient(config)
	if err != nil {
		log.Fatalf("Failed to create SSE client: %v", err)
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Shutting down...")
		cancel()
	}()

	// Connect to the SSE stream
	if err := sseClient.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to SSE stream: %v", err)
	}

	// Start processing events
	go processEvents(sseClient, ctx)

	// Start monitoring client status
	go monitorClientStatus(sseClient, ctx)

	// Demo: Create a simple HTTP server to show how to convert events
	go startDemoServer(sseClient)

	fmt.Println("📡 SSE client is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-ctx.Done()

	// Clean shutdown
	fmt.Println("🧹 Cleaning up...")
	if err := sseClient.Close(); err != nil {
		fmt.Printf("Error closing SSE client: %v\n", err)
	}

	fmt.Println("👋 Goodbye!")
}

// processEvents handles incoming SSE events
func processEvents(sseClient *client.SSEClient, ctx context.Context) {
	eventChan := sseClient.Events()
	eventCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				fmt.Println("📻 Event channel closed")
				return
			}

			eventCount++
			processSSEEvent(event, eventCount)
		}
	}
}

// processSSEEvent processes a single SSE event
func processSSEEvent(event *client.SSEEvent, count int) {
	fmt.Printf("\n📨 Event #%d received:\n", count)
	fmt.Printf("  ID: %s\n", event.ID)
	fmt.Printf("  Type: %s\n", event.Event)
	fmt.Printf("  Sequence: %d\n", event.Sequence)
	fmt.Printf("  Timestamp: %s\n", event.Timestamp.Format(time.RFC3339))

	// Show data (truncated if too long)
	data := event.Data
	if len(data) > 200 {
		data = data[:200] + "..."
	}
	fmt.Printf("  Data: %s\n", data)

	// Show custom headers if any
	if len(event.Headers) > 0 {
		fmt.Printf("  Headers: %v\n", event.Headers)
	}

	// Convert to AG-UI event and demonstrate usage
	if agEvent, err := client.ConvertSSEToEvent(event); err == nil {
		fmt.Printf("  AG-UI Event Type: %s\n", agEvent.Type())

		// Process based on event type
		switch agEvent.Type() {
		case events.EventTypeTextMessageStart:
			fmt.Printf("  🚀 Text message started\n")
		case events.EventTypeTextMessageContent:
			fmt.Printf("  📝 Text message content received\n")
		case events.EventTypeTextMessageEnd:
			fmt.Printf("  ✅ Text message completed\n")
		case events.EventTypeToolCallStart:
			fmt.Printf("  🔧 Tool call started\n")
		case events.EventTypeToolCallEnd:
			fmt.Printf("  ✅ Tool call completed\n")
		case events.EventTypeStateSnapshot:
			fmt.Printf("  📸 State snapshot received\n")
		default:
			fmt.Printf("  ❓ Unknown/Custom event type\n")
		}
	} else {
		fmt.Printf("  ⚠️  Failed to convert to AG-UI event: %v\n", err)
	}
}

// monitorClientStatus periodically reports client status
func monitorClientStatus(sseClient *client.SSEClient, ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reportClientStatus(sseClient)
		}
	}
}

// reportClientStatus prints current client status
func reportClientStatus(sseClient *client.SSEClient) {
	state := sseClient.State()
	lastEventID := sseClient.LastEventID()
	reconnectCount := sseClient.ReconnectCount()
	bufferLength := sseClient.BufferLength()
	backpressureActive := sseClient.IsBackpressureActive()

	fmt.Printf("\n📊 SSE Client Status:\n")
	fmt.Printf("  State: %s\n", state)
	fmt.Printf("  Last Event ID: %s\n", lastEventID)
	fmt.Printf("  Reconnection Count: %d\n", reconnectCount)
	fmt.Printf("  Buffer Length: %d\n", bufferLength)
	fmt.Printf("  Backpressure Active: %t\n", backpressureActive)
}

// startDemoServer starts a simple HTTP server to demonstrate event conversion
func startDemoServer(sseClient *client.SSEClient) {
	mux := http.NewServeMux()

	// Endpoint to get client status
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		status := map[string]interface{}{
			"state":              sseClient.State().String(),
			"lastEventID":        sseClient.LastEventID(),
			"reconnectCount":     sseClient.ReconnectCount(),
			"bufferLength":       sseClient.BufferLength(),
			"backpressureActive": sseClient.IsBackpressureActive(),
		}

		fmt.Fprintf(w, `{
			"state": "%s",
			"lastEventID": "%s",
			"reconnectCount": %d,
			"bufferLength": %d,
			"backpressureActive": %t
		}`,
			status["state"],
			status["lastEventID"],
			status["reconnectCount"],
			status["bufferLength"],
			status["backpressureActive"])
	})

	// Endpoint to create and send a sample AG-UI event
	mux.HandleFunc("/send-sample-event", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Create a sample AG-UI event
		sampleEvent := events.NewBaseEvent(events.EventTypeTextMessageContent)
		sampleEvent.RawEvent = map[string]interface{}{
			"message": "Hello from demo server!",
			"sender":  "demo",
		}

		// Convert to SSE format
		sseEvent, err := client.ConvertEventToSSE(sampleEvent)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to convert event: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"message": "Sample event created",
			"sseEvent": {
				"id": "%s",
				"event": "%s",
				"data": "%s",
				"timestamp": "%s"
			}
		}`,
			sseEvent.ID,
			sseEvent.Event,
			sseEvent.Data,
			sseEvent.Timestamp.Format(time.RFC3339))
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "ok", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	fmt.Println("🌐 Demo HTTP server starting on :8080")
	fmt.Println("  GET  /status - Client status")
	fmt.Println("  POST /send-sample-event - Create sample event")
	fmt.Println("  GET  /health - Health check")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Printf("Demo server error: %v\n", err)
	}
}

// Example functions for advanced usage

// ExampleWithCustomEventHandler demonstrates custom event handling
func ExampleWithCustomEventHandler() {
	config := client.SSEClientConfig{
		URL: "https://api.example.com/events",
		EventFilter: func(eventType string) bool {
			// Only process message and tool events
			return eventType == "TEXT_MESSAGE_START" ||
				eventType == "TEXT_MESSAGE_CONTENT" ||
				eventType == "TEXT_MESSAGE_END" ||
				eventType == "TOOL_CALL_START" ||
				eventType == "TOOL_CALL_END"
		},
	}

	sseClient, err := client.NewSSEClient(config)
	if err != nil {
		log.Fatal(err)
	}
	defer sseClient.Close()

	ctx := context.Background()
	if err := sseClient.Connect(ctx); err != nil {
		log.Fatal(err)
	}

	// Custom event processor
	for event := range sseClient.Events() {
		switch event.Event {
		case "TEXT_MESSAGE_START":
			handleMessageStart(event)
		case "TEXT_MESSAGE_CONTENT":
			handleMessageContent(event)
		case "TEXT_MESSAGE_END":
			handleMessageEnd(event)
		case "TOOL_CALL_START":
			handleToolCallStart(event)
		case "TOOL_CALL_END":
			handleToolCallEnd(event)
		}
	}
}

func handleMessageStart(event *client.SSEEvent) {
	fmt.Printf("📝 Message started: %s\n", event.Data)
}

func handleMessageContent(event *client.SSEEvent) {
	fmt.Printf("📄 Message content: %s\n", event.Data)
}

func handleMessageEnd(event *client.SSEEvent) {
	fmt.Printf("✅ Message ended: %s\n", event.Data)
}

func handleToolCallStart(event *client.SSEEvent) {
	fmt.Printf("🔧 Tool call started: %s\n", event.Data)
}

func handleToolCallEnd(event *client.SSEEvent) {
	fmt.Printf("🔧 Tool call ended: %s\n", event.Data)
}

// ExampleWithResilience demonstrates robust error handling and reconnection
func ExampleWithResilience() {
	maxRetries := 5
	currentRetry := 0

	config := client.SSEClientConfig{
		URL:                  "https://api.example.com/events",
		InitialBackoff:       time.Second,
		MaxBackoff:           30 * time.Second,
		MaxReconnectAttempts: maxRetries,
		OnError: func(err error) {
			log.Printf("SSE Error: %v", err)
		},
		OnReconnect: func(attempt int) {
			currentRetry = attempt
			log.Printf("Reconnection attempt %d/%d", attempt, maxRetries)
		},
		OnDisconnect: func(err error) {
			log.Printf("Disconnected: %v", err)
			if currentRetry >= maxRetries {
				log.Fatal("Max reconnection attempts reached")
			}
		},
	}

	sseClient, err := client.NewSSEClient(config)
	if err != nil {
		log.Fatal(err)
	}
	defer sseClient.Close()

	// Retry connection logic
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := sseClient.Connect(ctx)
		cancel()

		if err == nil {
			break // Connected successfully
		}

		log.Printf("Connection failed: %v", err)
		time.Sleep(time.Second * time.Duration(currentRetry+1))
		currentRetry++

		if currentRetry >= maxRetries {
			log.Fatal("Failed to connect after maximum retries")
		}
	}

	// Process events...
	for event := range sseClient.Events() {
		log.Printf("Received event: %s", event.Event)
	}
}
