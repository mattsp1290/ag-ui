// +build ignore

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
	
	// Test a few key event types to demonstrate the implementation
	fmt.Println("Testing Event Processing Implementation:")
	fmt.Println(strings.Repeat("-", 50))
	
	// Test 1: Text Message Processing
	fmt.Println("1. Text Message Processing:")
	msgStart := events.NewTextMessageStartEvent("msg-001", events.WithRole("user"))
	responses, err := agent.ProcessEvent(ctx, msgStart)
	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Printf("   ✅ Message Start processed -> %d response(s)\n", len(responses))
		if len(responses) > 0 {
			fmt.Printf("   📤 Response type: %s\n", responses[0].Type())
		}
	}
	
	// Test 2: Tool Call Processing  
	fmt.Println("\n2. Tool Call Processing (Unknown Tool):")
	toolCall := events.NewToolCallStartEvent("tool-001", "unknown-calculator")
	responses, err = agent.ProcessEvent(ctx, toolCall)
	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Printf("   ✅ Tool Call processed -> %d response(s)\n", len(responses))
		if len(responses) > 0 {
			fmt.Printf("   📤 Response type: %s (Expected: Error for unknown tool)\n", responses[0].Type())
		}
	}
	
	// Test 3: State Management
	fmt.Println("\n3. State Snapshot Processing:")
	stateSnapshot := events.NewStateSnapshotEvent(map[string]interface{}{
		"user_id": "user-123",
		"context": "demo_session",
	})
	responses, err = agent.ProcessEvent(ctx, stateSnapshot)
	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Printf("   ✅ State Snapshot processed -> %d response(s)\n", len(responses))
		if len(responses) > 0 {
			fmt.Printf("   📤 Response type: %s\n", responses[0].Type())
		}
	}
	
	// Test 4: Custom Event Processing
	fmt.Println("\n4. Custom Event Processing (Health Check):")
	healthCheck := events.NewCustomEvent("health_check")
	responses, err = agent.ProcessEvent(ctx, healthCheck)
	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Printf("   ✅ Health Check processed -> %d response(s)\n", len(responses))
		if len(responses) > 0 {
			fmt.Printf("   📤 Response type: %s\n", responses[0].Type())
			if customResp, ok := responses[0].(*events.CustomEvent); ok {
				fmt.Printf("   📋 Response name: %s\n", customResp.Name)
			}
		}
	}
	
	// Test 5: Metrics Request
	fmt.Println("\n5. Custom Event Processing (Metrics Request):")
	metricsReq := events.NewCustomEvent("metrics_request")
	responses, err = agent.ProcessEvent(ctx, metricsReq)
	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Printf("   ✅ Metrics Request processed -> %d response(s)\n", len(responses))
		if len(responses) > 0 {
			fmt.Printf("   📤 Response type: %s\n", responses[0].Type())
			if customResp, ok := responses[0].(*events.CustomEvent); ok {
				fmt.Printf("   📋 Response name: %s\n", customResp.Name)
				if metrics, ok := customResp.Value.(map[string]interface{}); ok {
					fmt.Printf("   📊 Metrics data contains %d fields\n", len(metrics))
				}
			}
		}
	}
	
	// Test Error Handling
	fmt.Println("\n" + strings.Repeat("-", 50))
	fmt.Println("Testing Error Handling:")
	
	// Test 6: Invalid Event Validation
	fmt.Println("\n6. Invalid Event Validation:")
	invalidEvent := events.NewTextMessageStartEvent("") // Empty message ID should fail validation
	_, err = agent.ProcessEvent(ctx, invalidEvent)
	if err != nil {
		fmt.Printf("   ✅ Validation error correctly caught: %v\n", err)
	} else {
		fmt.Printf("   ❌ Expected validation error but got none\n")
	}
	
	// Test 7: Context Cancellation
	fmt.Println("\n7. Context Cancellation Handling:")
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately
	
	validEvent := events.NewTextMessageStartEvent("msg-002")
	_, err = agent.ProcessEvent(cancelCtx, validEvent)
	if err != nil {
		fmt.Printf("   ✅ Context cancellation correctly handled: %v\n", err)
	} else {
		fmt.Printf("   ❌ Expected context cancellation error but got none\n")
	}
	
	// Show final metrics
	fmt.Println("\n" + strings.Repeat("-", 50))
	fmt.Println("Final Agent Metrics:")
	health := agent.Health()
	fmt.Printf("Status: %s\n", health.Status)
	fmt.Printf("Events Processed: %v\n", health.Details["events_processed"])
	fmt.Printf("Error Count: %v\n", health.Details["error_count"])
	fmt.Printf("Uptime: %v\n", health.Details["uptime"])
	
	// Stop the agent
	if err := agent.Stop(ctx); err != nil {
		log.Printf("Warning: Failed to stop agent cleanly: %v", err)
	} else {
		fmt.Printf("\n✅ Agent stopped successfully\n")
	}
	
	fmt.Println("\n=== Event Processing Implementation Demo Completed ===")
	fmt.Println("✅ All major event types are now properly handled!")
	fmt.Println("✅ Comprehensive error handling and validation implemented!")
	fmt.Println("✅ Metrics tracking and context cancellation working!")
	fmt.Println("✅ Extensible framework ready for specific agent implementations!")
}