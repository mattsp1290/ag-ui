package client

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// Simple test to verify event processing logic without protobuf dependencies
func TestEventProcessingCore(t *testing.T) {
	// Create and initialize agent
	agent := NewBaseAgent("test-agent", "Test agent for event processing")
	
	config := DefaultAgentConfig()
	config.Name = "test-agent"
	
	ctx := context.Background()
	if err := agent.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}
	
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	
	// Test different event types
	testCases := []struct {
		name          string
		event         events.Event
		expectedType  events.EventType
		expectedCount int
	}{
		{
			name:          "TextMessageStart",
			event:         events.NewTextMessageStartEvent("msg-123"),
			expectedType:  events.EventTypeTextMessageContent,
			expectedCount: 1,
		},
		{
			name:          "TextMessageContent", 
			event:         events.NewTextMessageContentEvent("msg-123", "Hello"),
			expectedType:  events.EventTypeTextMessageContent,
			expectedCount: 1,
		},
		{
			name:          "TextMessageEnd",
			event:         events.NewTextMessageEndEvent("msg-123"),
			expectedType:  events.EventTypeTextMessageEnd,
			expectedCount: 1,
		},
		{
			name:          "ToolCallStart",
			event:         events.NewToolCallStartEvent("tool-123", "unknown-tool"),
			expectedType:  events.EventTypeRunError, // Should return error for unknown tool
			expectedCount: 1,
		},
		{
			name: "StateSnapshot",
			event: events.NewStateSnapshotEvent(map[string]interface{}{
				"key": "value",
			}),
			expectedType:  events.EventTypeStateDelta,
			expectedCount: 1,
		},
		{
			name: "CustomHealthCheck",
			event: events.NewCustomEvent("health_check"),
			expectedType:  events.EventTypeCustom,
			expectedCount: 1,
		},
	}
	
	initialEventsProcessed := agent.getEventsProcessed()
	
	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			responseEvents, err := agent.ProcessEvent(ctx, tc.event)
			
			if err != nil {
				t.Fatalf("Failed to process %s event: %v", tc.name, err)
			}
			
			if len(responseEvents) != tc.expectedCount {
				t.Errorf("Expected %d response events for %s, got %d", 
					tc.expectedCount, tc.name, len(responseEvents))
			}
			
			if len(responseEvents) > 0 && responseEvents[0].Type() != tc.expectedType {
				t.Errorf("Expected %s response type for %s, got %s", 
					tc.expectedType, tc.name, responseEvents[0].Type())
			}
			
			// Verify events processed count increased
			expectedProcessed := initialEventsProcessed + int64(i+1)
			if agent.getEventsProcessed() != expectedProcessed {
				t.Errorf("Expected %d events processed after %s, got %d", 
					expectedProcessed, tc.name, agent.getEventsProcessed())
			}
		})
	}
}

func TestEventValidation(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test agent")
	
	config := DefaultAgentConfig()
	config.Name = "test-agent"
	
	ctx := context.Background()
	if err := agent.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}
	
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	
	// Test with invalid event (empty message ID)
	invalidEvent := events.NewTextMessageStartEvent("")
	
	_, err := agent.ProcessEvent(ctx, invalidEvent)
	if err == nil {
		t.Error("Expected validation error for invalid event")
	}
	
	// Verify it's a validation error
	agentErr, ok := err.(*errors.AgentError)
	if !ok {
		t.Errorf("Expected AgentError, got %T", err)
	} else if agentErr.Type != errors.ErrorTypeValidation {
		t.Errorf("Expected validation error, got %s", agentErr.Type)
	}
	
	// Error count should be incremented
	if agent.getErrorCount() != 1 {
		t.Errorf("Expected 1 error counted, got %d", agent.getErrorCount())
	}
}

func TestMetricsUpdating(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test agent")
	
	config := DefaultAgentConfig()
	config.Name = "test-agent"
	
	ctx := context.Background()
	if err := agent.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}
	
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	
	initialProcessed := agent.getEventsProcessed()
	initialTools := atomic.LoadInt64(&agent.metrics.ToolsExecuted)
	initialStates := atomic.LoadInt64(&agent.metrics.StateUpdates)
	
	// Process a tool call (should increment tools executed)
	toolEvent := events.NewToolCallStartEvent("tool-123", "test-tool")
	_, err := agent.ProcessEvent(ctx, toolEvent)
	if err != nil {
		t.Fatalf("Failed to process tool event: %v", err)
	}
	
	// Process a state snapshot (should increment state updates)
	stateEvent := events.NewStateSnapshotEvent(map[string]interface{}{"key": "value"})
	_, err = agent.ProcessEvent(ctx, stateEvent)
	if err != nil {
		t.Fatalf("Failed to process state event: %v", err)
	}
	
	// Verify metrics were updated
	if agent.getEventsProcessed() != initialProcessed+2 {
		t.Errorf("Expected %d events processed, got %d", 
			initialProcessed+2, agent.getEventsProcessed())
	}
	
	if atomic.LoadInt64(&agent.metrics.ToolsExecuted) != initialTools+1 {
		t.Errorf("Expected %d tools executed, got %d", 
			initialTools+1, atomic.LoadInt64(&agent.metrics.ToolsExecuted))
	}
	
	if atomic.LoadInt64(&agent.metrics.StateUpdates) != initialStates+1 {
		t.Errorf("Expected %d state updates, got %d", 
			initialStates+1, atomic.LoadInt64(&agent.metrics.StateUpdates))
	}
	
	// Verify processing time is set
	if agent.metrics.AverageProcessingTime == 0 {
		t.Error("Expected average processing time to be set")
	}
}

func TestEventProcessingContextCancellation(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test agent")
	
	config := DefaultAgentConfig()
	config.Name = "test-agent"
	
	baseCtx := context.Background()
	if err := agent.Initialize(baseCtx, config); err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}
	
	if err := agent.Start(baseCtx); err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	
	// Create cancelled context
	cancelledCtx, cancel := context.WithCancel(baseCtx)
	cancel()
	
	event := events.NewTextMessageStartEvent("msg-123")
	
	_, err := agent.ProcessEvent(cancelledCtx, event)
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
	
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestAgentNotRunning(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test agent")
	
	config := DefaultAgentConfig()
	config.Name = "test-agent"
	
	ctx := context.Background()
	if err := agent.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}
	
	// Don't start the agent
	
	event := events.NewTextMessageStartEvent("msg-123")
	
	_, err := agent.ProcessEvent(ctx, event)
	if err == nil {
		t.Error("Expected error when agent is not running")
	}
	
	agentErr, ok := err.(*errors.AgentError)
	if !ok {
		t.Errorf("Expected AgentError, got %T", err)
	} else if agentErr.Type != errors.ErrorTypeInvalidState {
		t.Errorf("Expected invalid state error, got %s", agentErr.Type)
	}
}

func TestCustomEventTypes(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test agent")
	
	config := DefaultAgentConfig()
	config.Name = "test-agent"
	
	ctx := context.Background()
	if err := agent.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}
	
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	
	// Test metrics request
	metricsEvent := events.NewCustomEvent("metrics_request")
	responseEvents, err := agent.ProcessEvent(ctx, metricsEvent)
	if err != nil {
		t.Fatalf("Failed to process metrics request: %v", err)
	}
	
	if len(responseEvents) != 1 {
		t.Fatalf("Expected 1 response event, got %d", len(responseEvents))
	}
	
	customResponse, ok := responseEvents[0].(*events.CustomEvent)
	if !ok {
		t.Fatalf("Expected CustomEvent response, got %T", responseEvents[0])
	}
	
	if customResponse.Name != "metrics_response" {
		t.Errorf("Expected 'metrics_response', got '%s'", customResponse.Name)
	}
	
	// Verify metrics data is present
	metricsData, ok := customResponse.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected metrics data to be map[string]interface{}, got %T", customResponse.Value)
	}
	
	expectedKeys := []string{"events_processed", "error_count", "tools_executed", "state_updates", "average_processing_time"}
	for _, key := range expectedKeys {
		if _, exists := metricsData[key]; !exists {
			t.Errorf("Expected metrics key '%s' not found", key)
		}
	}
}

func TestProcessEventStreaming(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test agent")
	
	config := DefaultAgentConfig()
	config.Name = "test-agent"
	config.Capabilities.Streaming = true
	
	ctx := context.Background()
	if err := agent.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}
	
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	
	// Get the event stream
	stream, err := agent.StreamEvents(ctx)
	if err != nil {
		t.Fatalf("Failed to get event stream: %v", err)
	}
	
	// Process an event
	event := events.NewTextMessageStartEvent("msg-123")
	responseEvents, err := agent.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to process event: %v", err)
	}
	
	// Verify response events were also sent to stream
	select {
	case streamEvent := <-stream:
		if streamEvent.Type() != responseEvents[0].Type() {
			t.Errorf("Stream event type %s doesn't match response event type %s", 
				streamEvent.Type(), responseEvents[0].Type())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected event in stream, but got timeout")
	}
}

// Benchmark event processing performance
func BenchmarkProcessEvent(b *testing.B) {
	agent := NewBaseAgent("bench-agent", "Benchmark agent")
	
	config := DefaultAgentConfig()
	config.Name = "bench-agent"
	
	ctx := context.Background()
	if err := agent.Initialize(ctx, config); err != nil {
		b.Fatalf("Failed to initialize agent: %v", err)
	}
	
	if err := agent.Start(ctx); err != nil {
		b.Fatalf("Failed to start agent: %v", err)
	}
	
	event := events.NewTextMessageStartEvent(fmt.Sprintf("msg-%d", time.Now().UnixNano()))
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := agent.ProcessEvent(ctx, event)
		if err != nil {
			b.Fatalf("Failed to process event: %v", err)
		}
	}
}