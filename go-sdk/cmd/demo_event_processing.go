package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

func main() {
	fmt.Println("=== AG-UI Go SDK Event Processing Demo ===")
	
	// Create and configure agent
	agent := client.NewBaseAgent("demo-agent", "Demo agent for event processing")
	
	config := client.DefaultAgentConfig()
	config.Name = "demo-agent"
	config.Capabilities.Streaming = true
	
	ctx := context.Background()
	
	// Initialize and start agent
	if err := agent.Initialize(ctx, config); err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}
	
	if err := agent.Start(ctx); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}
	
	fmt.Printf("Agent '%s' initialized and started successfully\n", agent.Name())
	fmt.Printf("Agent capabilities: %+v\n", agent.Capabilities())
	fmt.Println()
	
	// Demonstrate different event types
	testEvents := []struct {
		name  string
		event events.Event
	}{
		{
			name:  "Text Message Start",
			event: events.NewTextMessageStartEvent("msg-001", events.WithRole("user")),
		},
		{
			name:  "Text Message Content",
			event: events.NewTextMessageContentEvent("msg-001", "Hello, can you help me?"),
		},
		{
			name:  "Text Message End", 
			event: events.NewTextMessageEndEvent("msg-001"),
		},
		{
			name:  "Tool Call Start",
			event: events.NewToolCallStartEvent("tool-001", "calculator"),
		},
		{
			name:  "Tool Call Args",
			event: events.NewToolCallArgsEvent("tool-001", `{"operation": "add", "a": 5, "b": 3}`),
		},
		{
			name:  "Tool Call End",
			event: events.NewToolCallEndEvent("tool-001"),
		},
		{
			name: "State Snapshot",
			event: events.NewStateSnapshotEvent(map[string]interface{}{
				"user_id":      "user-123",
				"session_id":   "session-456", 
				"context":      "math_assistance",
				"last_result":  8,
			}),
		},
		{
			name: "State Delta",
			event: events.NewStateDeltaEvent([]events.JSONPatchOperation{
				{
					Op:    "replace",
					Path:  "/last_result",
					Value: 15,
				},
				{
					Op:    "add",
					Path:  "/calculation_count",
					Value: 1,
				},
			}),
		},
		{
			name:  "Custom Health Check",
			event: events.NewCustomEvent("health_check"),
		},
		{
			name:  "Custom Metrics Request",
			event: events.NewCustomEvent("metrics_request"),
		},
		{
			name:  "Run Started",
			event: events.NewRunStartedEvent("thread-001", "run-001"),
		},
		{
			name:  "Step Started",
			event: events.NewStepStartedEvent("processing_user_request"),
		},
		{
			name:  "Step Finished",
			event: events.NewStepFinishedEvent("processing_user_request"),
		},
		{
			name:  "Run Finished",
			event: events.NewRunFinishedEvent("thread-001", "run-001"),
		},
	}
	
	fmt.Println("Processing events:")
	fmt.Println(strings.Repeat("-", 60))
	
	for i, test := range testEvents {
		fmt.Printf("%d. Processing: %s\n", i+1, test.name)
		fmt.Printf("   Event Type: %s\n", test.event.Type())
		
		// Process the event
		start := time.Now()
		responseEvents, err := agent.ProcessEvent(ctx, test.event)
		duration := time.Since(start)
		
		if err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
		} else {
			fmt.Printf("   ✅ Success (took %v)\n", duration)
			fmt.Printf("   📤 Response events: %d\n", len(responseEvents))
			
			for j, respEvent := range responseEvents {
				fmt.Printf("      %d. Type: %s\n", j+1, respEvent.Type())
				
				// Show some details based on event type
				switch respEvent.Type() {
				case events.EventTypeTextMessageContent:
					if msgEvent, ok := respEvent.(*events.TextMessageContentEvent); ok {
						fmt.Printf("         Content: %s\n", msgEvent.Delta)
					}
				case events.EventTypeCustom:
					if customEvent, ok := respEvent.(*events.CustomEvent); ok {
						fmt.Printf("         Name: %s\n", customEvent.Name)
						if customEvent.Value != nil {
							fmt.Printf("         Has Value: Yes\n")
						}
					}
				case events.EventTypeRunError:
					if errorEvent, ok := respEvent.(*events.RunErrorEvent); ok {
						fmt.Printf("         Message: %s\n", errorEvent.Message)
						if errorEvent.Code != nil {
							fmt.Printf("         Code: %s\n", *errorEvent.Code)
						}
					}
				}
			}
		}
		fmt.Println()
	}
	
	// Show final metrics
	health := agent.Health()
	fmt.Println("Final Agent Health & Metrics:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Status: %s\n", health.Status)
	fmt.Printf("Last Check: %s\n", health.LastCheck.Format("2006-01-02 15:04:05"))
	
	if details, ok := health.Details["events_processed"].(int64); ok {
		fmt.Printf("Events Processed: %d\n", details)
	}
	if details, ok := health.Details["error_count"].(int64); ok {
		fmt.Printf("Error Count: %d\n", details)
	}
	if details, ok := health.Details["uptime"].(string); ok {
		fmt.Printf("Uptime: %s\n", details)
	}
	
	// Test error handling
	fmt.Println("\nTesting Error Handling:")
	fmt.Println(strings.Repeat("-", 30))
	
	// Test with invalid event
	invalidEvent := events.NewTextMessageStartEvent("") // Empty message ID
	_, err := agent.ProcessEvent(ctx, invalidEvent)
	if err != nil {
		fmt.Printf("✅ Validation error correctly caught: %v\n", err)
	}
	
	// Test context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	
	validEvent := events.NewTextMessageStartEvent("msg-002")
	_, err = agent.ProcessEvent(cancelCtx, validEvent)
	if err != nil {
		fmt.Printf("✅ Context cancellation correctly handled: %v\n", err)
	}
	
	// Stop the agent
	if err := agent.Stop(ctx); err != nil {
		log.Printf("Warning: Failed to stop agent cleanly: %v", err)
	} else {
		fmt.Printf("\n✅ Agent stopped successfully\n")
	}
	
	fmt.Println("\n=== Demo completed successfully ===")
}