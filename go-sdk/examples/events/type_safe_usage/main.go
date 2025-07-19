// Package main demonstrates type-safe event usage with the AG-UI Go SDK.
//
// This example shows:
// - Creating type-safe events using builders and constructors
// - Event validation and error handling
// - Automatic ID generation
// - Event serialization (JSON and protobuf)
// - Complex event scenarios (streaming, state management)
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

func main() {
	fmt.Println("=== AG-UI Type-Safe Event System Example ===")
	fmt.Println("Demonstrates: Type-safe event creation, validation, serialization, and best practices")
	fmt.Println()

	// 1. Basic Event Creation
	fmt.Println("=== 1. Basic Event Creation ===")
	demonstrateBasicEvents()

	// 2. Event Builder Pattern
	fmt.Println("\n=== 2. Event Builder Pattern ===")
	demonstrateEventBuilder()

	// 3. Auto ID Generation
	fmt.Println("\n=== 3. Auto ID Generation ===")
	demonstrateAutoIDGeneration()

	// 4. Complex Events
	fmt.Println("\n=== 4. Complex Events (State and Messages) ===")
	demonstrateComplexEvents()

	// 5. Event Streaming Simulation
	fmt.Println("\n=== 5. Event Streaming Simulation ===")
	demonstrateEventStreaming()

	// 6. Custom and Raw Events
	fmt.Println("\n=== 6. Custom and Raw Events ===")
	demonstrateCustomEvents()

	// 7. Event Validation
	fmt.Println("\n=== 7. Event Validation ===")
	demonstrateEventValidation()

	// 8. Event Serialization
	fmt.Println("\n=== 8. Event Serialization ===")
	demonstrateEventSerialization()

	// 9. Error Handling
	fmt.Println("\n=== 9. Error Handling ===")
	demonstrateErrorHandling()

	// 10. Best Practices
	fmt.Println("\n=== 10. Best Practices ===")
	demonstrateBestPractices()

	fmt.Println("\n=== Example Completed Successfully! ===")
}

func demonstrateBasicEvents() {
	// Create run lifecycle events
	runStarted := events.NewRunStartedEvent("thread-123", "run-456")
	fmt.Printf("Run Started: %+v\n", runStarted)

	runFinished := events.NewRunFinishedEvent("thread-123", "run-456")
	fmt.Printf("Run Finished: %+v\n", runFinished)

	runError := events.NewRunErrorEvent("Run failed due to timeout",
		events.WithErrorCode("TIMEOUT"),
		events.WithRunID("run-456"),
	)
	fmt.Printf("Run Error: %+v\n", runError)

	// Create step events
	stepStarted := events.NewStepStartedEvent("data-processing")
	fmt.Printf("Step Started: %+v\n", stepStarted)

	stepFinished := events.NewStepFinishedEvent("data-processing")
	fmt.Printf("Step Finished: %+v\n", stepFinished)

	// Create message events
	msgStart := events.NewTextMessageStartEvent("msg-789")
	fmt.Printf("Message Start: %+v\n", msgStart)

	msgContent := events.NewTextMessageContentEvent("msg-789", "Hello, world!")
	fmt.Printf("Message Content: %+v\n", msgContent)

	msgEnd := events.NewTextMessageEndEvent("msg-789")
	fmt.Printf("Message End: %+v\n", msgEnd)

	// Create tool events
	toolStart := events.NewToolCallStartEvent("tool-123", "calculator")
	fmt.Printf("Tool Start: %+v\n", toolStart)

	toolArgs := events.NewToolCallArgsEvent("tool-123", `{"operation": "add", "a": 5, "b": 3}`)
	fmt.Printf("Tool Args: %+v\n", toolArgs)

	toolEnd := events.NewToolCallEndEvent("tool-123")
	fmt.Printf("Tool End: %+v\n", toolEnd)
}

func demonstrateEventBuilder() {
	// Simple event with builder
	event1, err := events.NewEventBuilder().
		RunStarted().
		WithThreadID("thread-456").
		WithRunID("run-789").
		WithCurrentTimestamp().
		Build()

	if err != nil {
		log.Printf("Failed to build event: %v", err)
		return
	}
	fmt.Printf("Builder Event 1: %+v\n", event1)

	// Complex message event with builder
	event2, err := events.NewEventBuilder().
		TextMessageContent().
		WithMessageID("msg-builder-123").
		WithDelta("This message was created with the builder pattern.").
		Build()

	if err != nil {
		log.Printf("Failed to build event: %v", err)
		return
	}
	fmt.Printf("Builder Event 2: %+v\n", event2)

	// Tool event with builder
	event3, err := events.NewEventBuilder().
		ToolCallStart().
		WithToolCallID("tool-builder-456").
		WithToolCallName("file_processor").
		WithParentMessageID("msg-parent-789").
		Build()

	if err != nil {
		log.Printf("Failed to build event: %v", err)
		return
	}
	fmt.Printf("Builder Event 3: %+v\n", event3)

	// State delta with builder
	event4, err := events.NewEventBuilder().
		StateDelta().
		AddDeltaOperation("replace", "/status", "processing").
		AddDeltaOperation("add", "/progress", 0.5).
		AddDeltaOperation("remove", "/errors", nil).
		Build()

	if err != nil {
		log.Printf("Failed to build event: %v", err)
		return
	}
	fmt.Printf("Builder Event 4: %+v\n", event4)
}

func demonstrateAutoIDGeneration() {
	// Using builder with auto-generation
	event1, err := events.NewEventBuilder().
		RunStarted().
		WithAutoGenerateIDs().
		Build()

	if err != nil {
		log.Printf("Failed to build event with auto IDs: %v", err)
		return
	}
	
	runEvent := event1.(*events.RunStartedEvent)
	fmt.Printf("Auto-generated Thread ID: %s\n", runEvent.ThreadID)
	fmt.Printf("Auto-generated Run ID: %s\n", runEvent.RunID)

	// Using constructor options
	event2 := events.NewRunStartedEventWithOptions("", "",
		events.WithAutoThreadID(),
		events.WithAutoRunID(),
	)
	fmt.Printf("Option-generated Thread ID: %s\n", event2.ThreadID)
	fmt.Printf("Option-generated Run ID: %s\n", event2.RunID)

	// Manual ID generation
	threadID := events.GenerateThreadID()
	runID := events.GenerateRunID()
	messageID := events.GenerateMessageID()
	toolCallID := events.GenerateToolCallID()
	stepID := events.GenerateStepID()

	fmt.Printf("Manual IDs:\n")
	fmt.Printf("  Thread ID: %s\n", threadID)
	fmt.Printf("  Run ID: %s\n", runID)
	fmt.Printf("  Message ID: %s\n", messageID)
	fmt.Printf("  Tool Call ID: %s\n", toolCallID)
	fmt.Printf("  Step ID: %s\n", stepID)
}

func demonstrateComplexEvents() {
	// State snapshot event
	appState := map[string]interface{}{
		"userId":    "user-123",
		"sessionId": "session-456",
		"data": map[string]interface{}{
			"currentPage": "dashboard",
			"filters": map[string]interface{}{
				"dateRange": "last-7-days",
				"category":  "analytics",
			},
			"preferences": map[string]interface{}{
				"theme":    "dark",
				"language": "en",
			},
		},
		"metadata": map[string]interface{}{
			"version":     "1.2.3",
			"timestamp":   time.Now().Unix(),
			"environment": "production",
		},
	}

	snapshotEvent := events.NewStateSnapshotEvent(appState)
	fmt.Printf("State Snapshot: %+v\n", snapshotEvent)

	// State delta event with multiple operations
	deltaOps := []events.JSONPatchOperation{
		{
			Op:    "replace",
			Path:  "/data/currentPage",
			Value: "settings",
		},
		{
			Op:    "add",
			Path:  "/data/filters/status",
			Value: "active",
		},
		{
			Op:    "remove",
			Path:  "/data/filters/category",
		},
		{
			Op:   "move",
			Path: "/data/preferences/theme",
			From: "/data/preferences/oldTheme",
		},
	}

	deltaEvent := events.NewStateDeltaEvent(deltaOps)
	fmt.Printf("State Delta: %+v\n", deltaEvent)

	// Messages snapshot event
	messages := []events.Message{
		{
			ID:      "msg-1",
			Role:    "user",
			Content: stringPtr("Can you help me analyze this data?"),
		},
		{
			ID:      "msg-2",
			Role:    "assistant",
			Content: stringPtr("Of course! I'd be happy to help you analyze your data. What type of data are you working with?"),
		},
		{
			ID:      "msg-3",
			Role:    "user",
			Content: stringPtr("I have sales data from the last quarter."),
		},
	}

	messagesEvent := events.NewMessagesSnapshotEvent(messages)
	fmt.Printf("Messages Snapshot: %+v\n", messagesEvent)
}

func demonstrateEventStreaming() {
	fmt.Println("Simulating streaming message content...")

	messageID := events.GenerateMessageID()
	fullMessage := "This is a simulated streaming response that demonstrates how content can be sent in real-time chunks to create a smooth user experience."

	// Start message
	startEvent := events.NewTextMessageStartEvent(messageID)
	fmt.Printf("Stream Start: %+v\n", startEvent)

	// Stream content in chunks
	chunkSize := 20
	for i := 0; i < len(fullMessage); i += chunkSize {
		end := i + chunkSize
		if end > len(fullMessage) {
			end = len(fullMessage)
		}

		chunk := fullMessage[i:end]
		contentEvent := events.NewTextMessageContentEvent(messageID, chunk)
		
		fmt.Printf("Stream Chunk %d: %+v\n", (i/chunkSize)+1, contentEvent)
		
		// Simulate streaming delay
		time.Sleep(100 * time.Millisecond)
	}

	// End message
	endEvent := events.NewTextMessageEndEvent(messageID)
	fmt.Printf("Stream End: %+v\n", endEvent)

	fmt.Println("Streaming simulation completed!")
}

func demonstrateCustomEvents() {
	// Custom event for application-specific data
	customEvent := events.NewCustomEvent("user_file_upload")
	fmt.Printf("Custom Event: %+v\n", customEvent)

	// Raw event for external system integration
	externalSystemEvent := struct {
		Type      string                 `json:"type"`
		Source    string                 `json:"source"`
		Data      map[string]interface{} `json:"data"`
		Timestamp int64                  `json:"timestamp"`
		Version   string                 `json:"version"`
	}{
		Type:   "external.notification",
		Source: "payment-service",
		Data: map[string]interface{}{
			"transaction_id": "txn-789",
			"amount":         99.99,
			"currency":       "USD",
			"status":         "completed",
		},
		Timestamp: time.Now().Unix(),
		Version:   "2.1.0",
	}

	rawEvent := events.NewRawEvent(externalSystemEvent)
	fmt.Printf("Raw Event: %+v\n", rawEvent)
}

func demonstrateEventValidation() {
	fmt.Println("Testing event validation...")

	// Valid event
	validEvent := events.NewRunStartedEvent("thread-123", "run-456")
	if err := validEvent.Validate(); err != nil {
		fmt.Printf("Unexpected validation error: %v\n", err)
	} else {
		fmt.Println("✓ Valid event passed validation")
	}

	// Invalid event - missing required fields
	invalidEvent := &events.RunStartedEvent{
		BaseEvent: events.NewBaseEvent(events.EventTypeRunStarted),
		ThreadID:  "", // Missing required field
		RunID:     "run-456",
	}

	if err := invalidEvent.Validate(); err != nil {
		fmt.Printf("✓ Invalid event correctly failed validation: %v\n", err)
	} else {
		fmt.Println("✗ Invalid event incorrectly passed validation")
	}

	// Invalid event - missing message content
	invalidMessageEvent := &events.RunErrorEvent{
		BaseEvent: events.NewBaseEvent(events.EventTypeRunError),
		Message:   "", // Missing required field
		RunID:     "run-456",
	}

	if err := invalidMessageEvent.Validate(); err != nil {
		fmt.Printf("✓ Invalid message event correctly failed validation: %v\n", err)
	} else {
		fmt.Println("✗ Invalid message event incorrectly passed validation")
	}

	// Test builder validation
	fmt.Println("Testing builder validation...")
	_, err := events.NewEventBuilder().
		// No event type set
		WithRunID("run-123").
		Build()

	if err != nil {
		fmt.Printf("✓ Builder correctly failed without event type: %v\n", err)
	} else {
		fmt.Println("✗ Builder incorrectly succeeded without event type")
	}
}

func demonstrateEventSerialization() {
	// Create a complex event
	event := events.NewRunStartedEvent("thread-789", "run-012")

	// JSON serialization
	jsonData, err := event.ToJSON()
	if err != nil {
		log.Printf("Failed to serialize to JSON: %v", err)
		return
	}
	fmt.Printf("JSON Serialization: %s\n", string(jsonData))

	// Protobuf serialization
	pbEvent, err := event.ToProtobuf()
	if err != nil {
		log.Printf("Failed to serialize to protobuf: %v", err)
		return
	}
	fmt.Printf("Protobuf Type: %T\n", pbEvent.Event)
	fmt.Printf("Protobuf Event: %+v\n", pbEvent)

	// Serialize a more complex event
	deltaEvent, err := events.NewEventBuilder().
		StateDelta().
		AddDeltaOperation("replace", "/status", "active").
		AddDeltaOperation("add", "/metadata/updatedAt", time.Now().Unix()).
		Build()

	if err != nil {
		log.Printf("Failed to build delta event: %v", err)
		return
	}

	deltaJSON, err := deltaEvent.ToJSON()
	if err != nil {
		log.Printf("Failed to serialize delta to JSON: %v", err)
		return
	}
	fmt.Printf("Delta JSON: %s\n", string(deltaJSON))
}

func demonstrateErrorHandling() {
	fmt.Println("Demonstrating error handling patterns...")

	// Graceful event creation with error handling
	event, err := createEventSafely(func() (events.Event, error) {
		return events.NewEventBuilder().
			TextMessageContent().
			WithMessageID("msg-safe-123").
			WithDelta("Safe event creation").
			Build()
	})

	if err != nil {
		fmt.Printf("Failed to create event safely: %v\n", err)
	} else {
		fmt.Printf("✓ Successfully created event safely: %+v\n", event)
	}

	// Error recovery example
	event2, err := createEventWithRecovery()
	if err != nil {
		fmt.Printf("Failed to create event with recovery: %v\n", err)
	} else {
		fmt.Printf("✓ Successfully created event with recovery: %+v\n", event2)
	}

	// Validation error handling
	if err := validateEventSafely(nil); err != nil {
		fmt.Printf("✓ Handled nil event validation gracefully: %v\n", err)
	}
}

func demonstrateBestPractices() {
	fmt.Println("Demonstrating best practices...")

	// 1. Use specific event types instead of generic ones
	fmt.Println("1. Using specific event types:")
	specificEvent := events.NewToolCallStartEvent("tool-123", "calculator")
	fmt.Printf("   Specific: %+v\n", specificEvent)

	// Instead of:
	genericEvent := events.NewCustomEvent("tool_action")
	fmt.Printf("   Generic: %+v\n", genericEvent)

	// 2. Include contextual information
	fmt.Println("2. Including contextual information:")
	contextualEvent := events.NewRunStartedEventWithOptions("", "",
		events.WithAutoThreadID(),
		events.WithAutoRunID(),
	)
	fmt.Printf("   With context: %+v\n", contextualEvent)

	// 3. Validate events before sending
	fmt.Println("3. Always validate events:")
	if err := contextualEvent.Validate(); err != nil {
		fmt.Printf("   Validation failed: %v\n", err)
	} else {
		fmt.Printf("   ✓ Event is valid\n")
	}

	// 4. Use auto-generation for unique IDs
	fmt.Println("4. Using auto-generation for IDs:")
	autoEvent, _ := events.NewEventBuilder().
		RunStarted().
		WithAutoGenerateIDs().
		Build()
	autoRunEvent := autoEvent.(*events.RunStartedEvent)
	fmt.Printf("   Auto Thread ID: %s\n", autoRunEvent.ThreadID)
	fmt.Printf("   Auto Run ID: %s\n", autoRunEvent.RunID)

	// 5. Batch related events
	fmt.Println("5. Batching related events:")
	batchEvents := createEventBatch("session-123")
	fmt.Printf("   Created batch of %d events\n", len(batchEvents))
	for i, event := range batchEvents {
		fmt.Printf("   Event %d: %T\n", i+1, event)
	}
}

// Helper functions

func stringPtr(s string) *string {
	return &s
}

func createEventSafely(creator func() (events.Event, error)) (events.Event, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic during event creation: %v", r)
		}
	}()

	event, err := creator()
	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	// Always validate
	if err := event.Validate(); err != nil {
		return nil, fmt.Errorf("event validation failed: %w", err)
	}

	return event, nil
}

func createEventWithRecovery() (events.Event, error) {
	// Simulate a function that might panic
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic: %v", r)
		}
	}()

	return events.NewEventBuilder().
		StepStarted().
		WithStepName("recovery-step").
		Build()
}

func validateEventSafely(event events.Event) error {
	if event == nil {
		return fmt.Errorf("cannot validate nil event")
	}

	return event.Validate()
}

func createEventBatch(sessionID string) []events.Event {
	var batch []events.Event

	// Start session
	startEvent := events.NewRunStartedEventWithOptions("", "",
		events.WithAutoThreadID(),
		events.WithAutoRunID(),
	)
	batch = append(batch, startEvent)

	// Start step
	stepEvent := events.NewStepStartedEventWithOptions("",
		events.WithAutoStepName(),
	)
	batch = append(batch, stepEvent)

	// Message events
	msgStart := events.NewTextMessageStartEvent(events.GenerateMessageID())
	batch = append(batch, msgStart)

	msgContent := events.NewTextMessageContentEvent(msgStart.MessageID, "Processing your request...")
	batch = append(batch, msgContent)

	msgEnd := events.NewTextMessageEndEvent(msgStart.MessageID)
	batch = append(batch, msgEnd)

	return batch
}