// Package main demonstrates standalone SSE client usage
// This example works independently of the main client package to avoid compilation issues
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"
)

// Simplified SSE client demo that doesn't depend on the full AG-UI package
func main() {
	fmt.Println("🚀 SSE Streaming Client Demo")
	fmt.Println("This demo shows the core SSE functionality without external dependencies")

	// Create a demo SSE server
	server := createDemoSSEServer()
	defer server.Close()

	fmt.Printf("📡 Demo server started at: %s\n", server.URL)

	// Demonstrate SSE client functionality
	demonstrateSSEClient(server.URL)
}

// createDemoSSEServer creates a test SSE server that sends sample events
func createDemoSSEServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Send demo events
		events := []string{
			"id: 1\nevent: welcome\ndata: {\"message\": \"Welcome to SSE demo!\"}",
			"id: 2\nevent: notification\ndata: {\"type\": \"info\", \"text\": \"This is a notification\"}",
			"id: 3\ndata: Simple message without event type",
			"id: 4\nevent: multiline\ndata: Line 1\ndata: Line 2\ndata: Line 3",
			"id: 5\nevent: json\ndata: {\"users\": [\"alice\", \"bob\"], \"count\": 2}",
			"retry: 5000",
			"id: 6\nevent: heartbeat\ndata: {\"timestamp\": \"" + time.Now().Format(time.RFC3339) + "\"}",
		}

		fmt.Printf("📤 Sending %d demo events\n", len(events))

		for i, event := range events {
			fmt.Fprintf(w, "%s\n\n", event)
			flusher.Flush()

			// Add delay between events to simulate real-time data
			if i < len(events)-1 {
				time.Sleep(500 * time.Millisecond)
			}
		}

		fmt.Println("✅ All demo events sent")
	}))
}

// demonstrateSSEClient demonstrates SSE client functionality
func demonstrateSSEClient(serverURL string) {
	fmt.Println("🔌 Connecting to SSE stream...")

	// Create context with timeout for cancellation support
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create HTTP client for SSE
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", serverURL, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	// Set SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Unexpected status: %d", resp.StatusCode)
	}

	fmt.Println("✅ Connected successfully!")
	fmt.Println("📨 Processing events:")

	// Parse SSE events manually to demonstrate the protocol
	parseSSEEvents(ctx, resp)
}

// parseSSEEvents manually parses SSE events to demonstrate the protocol
func parseSSEEvents(ctx context.Context, resp *http.Response) {
	scanner := bufio.NewScanner(resp.Body)

	var currentEvent SSEEvent
	eventCount := 0

	for scanner.Scan() {
		// Check context cancellation during long-running loop
		select {
		case <-ctx.Done():
			fmt.Printf("⏰ Context cancelled, stopping event processing: %v\n", ctx.Err())
			return
		default:
		}

		line := scanner.Text()

		// Empty line indicates end of event
		if line == "" {
			if currentEvent.hasData() {
				eventCount++
				currentEvent.display(eventCount)
				currentEvent = SSEEvent{} // Reset for next event
			}
			continue
		}

		// Parse event line
		currentEvent.parseLine(line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("❌ Scanner error: %v\n", err)
	}

	fmt.Printf("🎉 Processing complete! Received %d events\n", eventCount)
}

// SSEEvent represents a parsed Server-Sent Event
type SSEEvent struct {
	ID    string
	Event string
	Data  string
	Retry string
}

// hasData checks if the event has any data
func (e *SSEEvent) hasData() bool {
	return e.Data != "" || e.ID != "" || e.Event != "" || e.Retry != ""
}

// parseLine parses a single line of an SSE event
func (e *SSEEvent) parseLine(line string) {
	// Handle comments
	if line[0] == ':' {
		return
	}

	// Find field separator
	colonIndex := -1
	for i, char := range line {
		if char == ':' {
			colonIndex = i
			break
		}
	}

	var field, value string
	if colonIndex == -1 {
		field = line
		value = ""
	} else {
		field = line[:colonIndex]
		value = line[colonIndex+1:]
		// Remove leading space from value
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}
	}

	// Set field value
	switch field {
	case "id":
		e.ID = value
	case "event":
		e.Event = value
	case "data":
		if e.Data != "" {
			e.Data += "\n"
		}
		e.Data += value
	case "retry":
		e.Retry = value
	}
}

// display prints the event in a formatted way
func (e *SSEEvent) display(count int) {
	fmt.Printf("\n📋 Event #%d:\n", count)

	if e.ID != "" {
		fmt.Printf("   ID: %s\n", e.ID)
	}

	if e.Event != "" {
		fmt.Printf("   Type: %s\n", e.Event)
	} else {
		fmt.Printf("   Type: (default)\n")
	}

	if e.Data != "" {
		// Format multiline data
		if len(e.Data) > 100 {
			fmt.Printf("   Data: %s...\n", e.Data[:100])
		} else {
			fmt.Printf("   Data: %s\n", e.Data)
		}
	}

	if e.Retry != "" {
		fmt.Printf("   Retry: %s ms\n", e.Retry)
	}

	// Demonstrate event type handling
	e.handleEventType()
}

// handleEventType demonstrates how different event types might be handled
func (e *SSEEvent) handleEventType() {
	switch e.Event {
	case "welcome":
		fmt.Printf("   🎉 Welcome event received!\n")
	case "notification":
		fmt.Printf("   🔔 Notification event received!\n")
	case "multiline":
		fmt.Printf("   📝 Multiline event received!\n")
	case "json":
		fmt.Printf("   📊 JSON data event received!\n")
	case "heartbeat":
		fmt.Printf("   💓 Heartbeat event received!\n")
	case "":
		fmt.Printf("   📄 Default event type\n")
	default:
		fmt.Printf("   ❓ Unknown event type: %s\n", e.Event)
	}
}

// Additional demo functionality
func init() {
	fmt.Println("🔧 SSE Client Demo Initialized")
	fmt.Println("This demonstrates the core concepts of Server-Sent Events:")
	fmt.Println("  • Event ID tracking")
	fmt.Println("  • Event type handling")
	fmt.Println("  • Multiline data parsing")
	fmt.Println("  • Retry interval processing")
	fmt.Println("  • Connection management")
	fmt.Println()
}
