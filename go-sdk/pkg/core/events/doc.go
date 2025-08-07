// Package events provides a comprehensive type-safe event system for the AG-UI Go SDK.
//
// This package implements the complete AG-UI (Agent-User Interaction) protocol with
// compile-time type safety, automatic validation, and seamless serialization. The
// event system enables real-time streaming, bidirectional state synchronization, and
// human-in-the-loop collaboration between AI agents and front-end applications.
//
// # Event Types
//
// The package implements all 16 AG-UI event types:
//
// Run Lifecycle Events:
//   - RUN_STARTED: Agent execution initiation
//   - RUN_FINISHED: Successful agent execution completion
//   - RUN_ERROR: Agent execution error termination
//   - STEP_STARTED: Individual step initiation
//   - STEP_FINISHED: Individual step completion
//
// Message Events:
//   - TEXT_MESSAGE_START: Text message stream initiation
//   - TEXT_MESSAGE_CONTENT: Streaming text message content
//   - TEXT_MESSAGE_END: Text message stream completion
//
// Tool Events:
//   - TOOL_CALL_START: Tool invocation initiation
//   - TOOL_CALL_ARGS: Tool arguments specification
//   - TOOL_CALL_END: Tool execution completion
//
// State Events:
//   - STATE_SNAPSHOT: Complete state snapshot
//   - STATE_DELTA: Incremental state changes using JSON Patch (RFC 6902)
//   - MESSAGES_SNAPSHOT: Complete message history
//
// Custom Events:
//   - RAW: Raw data pass-through
//   - CUSTOM: Custom event types for extensibility
//
// # Basic Usage
//
//	import "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
//
//	// Create type-safe events
//	runEvent := events.NewRunStartedEvent("thread-123", "run-456")
//	if err := runEvent.Validate(); err != nil {
//		log.Fatal("Invalid event:", err)
//	}
//
//	// Create streaming message content
//	msgStart := events.NewTextMessageStartEvent("msg-1",
//		events.WithMessageRole("user"))
//	msgContent := events.NewTextMessageContentEvent("msg-1", "Hello, ")
//	msgContent2 := events.NewTextMessageContentEvent("msg-1", "world!")
//	msgEnd := events.NewTextMessageEndEvent("msg-1")
//
//	// All events have automatic validation
//	events := []events.Event{runEvent, msgStart, msgContent, msgContent2, msgEnd}
//	for _, event := range events {
//		if err := event.Validate(); err != nil {
//			log.Printf("Event validation failed: %v", err)
//		}
//	}
//
// # Event Creation with Options
//
// Events support various options for flexible configuration:
//
//	// Text message with role
//	msgEvent := events.NewTextMessageStartEvent("msg-1",
//		events.WithMessageRole("assistant"))
//
//	// Tool call with parent message
//	toolEvent := events.NewToolCallStartEvent("tool-1", "get_weather",
//		events.WithParentMessage("msg-1"))
//
//	// Run error with code
//	errorEvent := events.NewRunErrorEvent("Connection failed",
//		events.WithErrorCode("NETWORK_ERROR"),
//		events.WithRunID("run-123"))
//
// # Automatic ID Generation
//
// The package provides utilities for automatic ID generation:
//
//	// Generate IDs manually
//	runID := events.GenerateRunID()
//	messageID := events.GenerateMessageID()
//	toolCallID := events.GenerateToolCallID()
//
//	// Use auto-generation options
//	runEvent := events.NewRunStartedEventWithOptions("", "",
//		events.WithAutoRunID(),
//		events.WithAutoThreadID())
//
//	msgEvent := events.NewTextMessageStartEvent("",
//		events.WithAutoMessageID(),
//		events.WithMessageRole("user"))
//
// # Fluent Event Builder
//
// For complex event construction, use the fluent builder pattern:
//
//	// Simple event with builder
//	event, err := events.NewEventBuilder().
//		RunStarted().
//		WithThreadID("thread-123").
//		WithRunID("run-456").
//		Build()
//
//	// Complex event with auto-generation
//	complexEvent, err := events.NewEventBuilder().
//		TextMessageStart().
//		WithRole("assistant").
//		WithAutoGenerateIDs().
//		Build()
//
//	// State delta with multiple operations
//	stateEvent, err := events.NewEventBuilder().
//		StateDelta().
//		AddDeltaOperation("add", "/counter", 42).
//		AddDeltaOperation("replace", "/status", "active").
//		Build()
//
// # Event Validation System
//
// The package provides a comprehensive validation system to ensure AG-UI protocol compliance:
//
//	import "context"
//
//	// Create a validator with default configuration (strict mode)
//	validator := events.NewEventValidator(nil)
//
//	// Validate a single event
//	event := events.NewRunStartedEvent("thread-1", "run-1")
//	result := validator.ValidateEvent(context.Background(), event)
//
//	if result.HasErrors() {
//		for _, err := range result.Errors {
//			log.Printf("[%s] %s: %s", err.Severity, err.RuleID, err.Message)
//			// Use err.Suggestions for remediation hints
//		}
//	}
//
//	// Validate an event sequence
//	sequence := []events.Event{
//		events.NewRunStartedEvent("thread-1", "run-1"),
//		events.NewTextMessageStartEvent("msg-1", events.WithRole("user")),
//		events.NewTextMessageContentEvent("msg-1", "Hello"),
//		events.NewTextMessageEndEvent("msg-1"),
//		events.NewRunFinishedEvent("thread-1", "run-1"),
//	}
//
//	seqResult := validator.ValidateSequence(context.Background(), sequence)
//	if !seqResult.IsValid {
//		log.Printf("Sequence validation failed with %d errors", len(seqResult.Errors))
//	}
//
// # Validation Levels
//
// The package provides preset configurations for common use cases:
//
//	// Production - enforces all AG-UI protocol rules strictly
//	prodValidator := events.NewEventValidator(events.ProductionValidationConfig())
//
//	// Development - validates protocol but lenient with IDs/timestamps
//	devValidator := events.NewEventValidator(events.DevelopmentValidationConfig())
//
//	// Testing - allows out-of-order events for unit tests
//	testValidator := events.NewEventValidator(events.TestingValidationConfig())
//
//	// Permissive - minimal checks for prototyping
//	permissiveValidator := events.NewEventValidator(events.PermissiveValidationConfig())
//
//	// Custom validation configuration for specific needs
//	config := &events.ValidationConfig{
//		Level: events.ValidationStrict,
//		SkipTimestampValidation: true,  // Skip timestamp checks
//		SkipSequenceValidation: false,  // Enforce sequence rules
//		AllowEmptyIDs: false,           // Require all IDs
//		CustomValidators: []events.CustomValidator{
//			myBusinessLogicValidator,
//		},
//	}
//	customValidator := events.NewEventValidator(config)
//
// # Validation Rules
//
// The validator enforces these AG-UI protocol rules:
//
// Run Lifecycle Rules:
//   - RUN_STARTED must be the first event in any sequence
//   - No events allowed after RUN_FINISHED except RUN_ERROR
//   - Each run must have exactly one RUN_STARTED event
//   - RUN_FINISHED or RUN_ERROR must conclude every run
//
// Message Rules:
//   - TEXT_MESSAGE_START must precede TEXT_MESSAGE_CONTENT
//   - TEXT_MESSAGE_CONTENT must be followed by TEXT_MESSAGE_END
//   - Message IDs must be consistent across start/content/end events
//
// Tool Call Rules:
//   - TOOL_CALL_START must precede TOOL_CALL_ARGS
//   - TOOL_CALL_ARGS must be followed by TOOL_CALL_END
//   - Tool call IDs must be consistent across start/args/end events
//
// # Custom Validation Rules
//
//	// Implement the ValidationRule interface for custom rules
//	type MyCustomRule struct {
//		events.BaseValidationRule
//	}
//
//	func (r *MyCustomRule) Validate(event events.Event, ctx *events.ValidationContext) *events.ValidationResult {
//		result := &events.ValidationResult{IsValid: true}
//
//		// Your validation logic here
//		if event.Type() == events.EventTypeRunStarted {
//			runEvent := event.(*events.RunStartedEvent)
//			if !strings.HasPrefix(runEvent.RunID, "run-") {
//				result.AddError(&events.ValidationError{
//					RuleID:  r.ID(),
//					Message: "Run ID must start with 'run-'",
//					Suggestions: []string{"Use format: run-<uuid>"},
//				})
//			}
//		}
//
//		return result
//	}
//
//	// Add custom rule to validator
//	validator.AddRule(&MyCustomRule{
//		BaseValidationRule: events.BaseValidationRule{
//			id: "CUSTOM_RUN_ID_FORMAT",
//			description: "Validates run ID format",
//			severity: events.ValidationSeverityError,
//			enabled: true,
//		},
//	})
//
// # Serialization
//
// Events support JSON and Protocol Buffer serialization:
//
//	// JSON serialization
//	jsonData, err := event.ToJSON()
//	if err != nil {
//		log.Fatal("JSON serialization failed:", err)
//	}
//
//	// Deserialize from JSON
//	parsedEvent, err := events.EventFromJSON(jsonData)
//	if err != nil {
//		log.Fatal("JSON deserialization failed:", err)
//	}
//
//	// Protocol Buffer serialization
//	pbEvent, err := event.ToProtobuf()
//	if err != nil {
//		log.Fatal("Protobuf serialization failed:", err)
//	}
//
//	// Deserialize from Protocol Buffer
//	parsedEvent, err = events.EventFromProtobuf(pbEvent)
//	if err != nil {
//		log.Fatal("Protobuf deserialization failed:", err)
//	}
//
// # State Management
//
// State events support snapshots and incremental updates:
//
//	// Complete state snapshot
//	state := map[string]any{
//		"counter": 42,
//		"status":  "active",
//		"data":    []string{"item1", "item2"},
//	}
//	snapshotEvent := events.NewStateSnapshotEvent(state)
//
//	// Incremental state changes using JSON Patch
//	deltaOps := []events.JSONPatchOperation{
//		events.JSONPatchOperation{Op: "add", Path: "/newField", Value: "newValue"},
//		events.JSONPatchOperation{Op: "replace", Path: "/counter", Value: 43},
//		events.JSONPatchOperation{Op: "remove", Path: "/data/0"},
//	}
//	deltaEvent := events.NewStateDeltaEvent(deltaOps)
//
//	// Messages snapshot
//	messages := []events.Message{
//		{
//			ID:      "msg-1",
//			Role:    "user",
//			Content: stringPtr("Hello, assistant!"),
//		},
//		{
//			ID:   "msg-2",
//			Role: "assistant",
//			ToolCalls: []events.ToolCall{
//				{
//					ID:   "tool-1",
//					Type: "function",
//					Function: events.Function{
//						Name:      "get_weather",
//						Arguments: `{"location": "San Francisco"}`,
//					},
//				},
//			},
//		},
//	}
//	messagesEvent := events.NewMessagesSnapshotEvent(messages)
//
// # Custom Events
//
// For application-specific events, use custom or raw events:
//
//	// Custom event with structured data
//	customEvent := events.NewCustomEvent("user-action",
//		events.WithValue(map[string]any{
//			"action": "click",
//			"target": "submit-button",
//			"timestamp": time.Now().Unix(),
//		}))
//
//	// Raw event for pass-through data
//	rawEvent := events.NewRawEvent(externalEventData,
//		events.WithSource("external-system"))
//
// # Sequence Validation
//
// The package provides comprehensive sequence validation:
//
//	// Create a complete interaction sequence
//	sequence := []events.Event{
//		events.NewRunStartedEvent("thread-1", "run-1"),
//		events.NewStepStartedEvent("planning"),
//		events.NewTextMessageStartEvent("msg-1", events.WithRole("user")),
//		events.NewTextMessageContentEvent("msg-1", "What's the weather?"),
//		events.NewTextMessageEndEvent("msg-1"),
//		events.NewToolCallStartEvent("tool-1", "get_weather",
//			events.WithParentMessageID("msg-1")),
//		events.NewToolCallArgsEvent("tool-1", `{"location": "SF"}`),
//		events.NewToolCallEndEvent("tool-1"),
//		events.NewStepFinishedEvent("planning"),
//		events.NewRunFinishedEvent("thread-1", "run-1"),
//	}
//
//	// Validate the sequence
//	if err := events.ValidateSequence(sequence); err != nil {
//		log.Fatal("Invalid sequence:", err)
//	}
//
// # Performance Considerations
//
// The package is optimized for high-frequency event creation and validation:
//
//	// Benchmark event creation performance
//	func BenchmarkEventCreation(b *testing.B) {
//		for i := 0; i < b.N; i++ {
//			_ = events.NewRunStartedEvent("thread-123", "run-456")
//		}
//	}
//
//	// Use object pools for high-frequency scenarios
//	var eventPool = sync.Pool{
//		New: func() interface{} {
//			return events.NewEventBuilder()
//		},
//	}
//
//	func createEventOptimized() events.Event {
//		builder := eventPool.Get().(*events.EventBuilder)
//		defer eventPool.Put(builder)
//
//		event, _ := builder.RunStarted().
//			WithThreadID("thread-123").
//			WithRunID("run-456").
//			Build()
//		return event
//	}
//
// # Integration Examples
//
// # HTTP Server Integration
//
//	func handleEventStream(w http.ResponseWriter, r *http.Request) {
//		w.Header().Set("Content-Type", "application/json")
//		w.Header().Set("Cache-Control", "no-cache")
//		w.Header().Set("Connection", "keep-alive")
//
//		// Create event sequence
//		events := []events.Event{
//			events.NewRunStartedEvent("thread-1", "run-1"),
//			events.NewTextMessageStartEvent("msg-1", events.WithRole("assistant")),
//		}
//
//		for _, event := range events {
//			jsonData, _ := event.ToJSON()
//			fmt.Fprintf(w, "data: %s\n\n", jsonData)
//			if f, ok := w.(http.Flusher); ok {
//				f.Flush()
//			}
//		}
//	}
//
// # WebSocket Integration
//
//	func handleWebSocket(conn *websocket.Conn) {
//		for {
//			// Read event from client
//			_, message, err := conn.ReadMessage()
//			if err != nil {
//				break
//			}
//
//			// Parse event
//			event, err := events.EventFromJSON(message)
//			if err != nil {
//				continue
//			}
//
//			// Validate event
//			if err := event.Validate(); err != nil {
//				continue
//			}
//
//			// Process and respond
//			response := events.NewTextMessageStartEvent("response-1",
//				events.WithRole("assistant"))
//			responseData, _ := response.ToJSON()
//			conn.WriteMessage(websocket.TextMessage, responseData)
//		}
//	}
//
// # Error Handling Best Practices
//
//	func processEvent(data []byte) error {
//		// Parse event with error handling
//		event, err := events.EventFromJSON(data)
//		if err != nil {
//			return fmt.Errorf("failed to parse event: %w", err)
//		}
//
//		// Validate with context
//		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//		defer cancel()
//
//		if err := events.ValidateEventWithContext(ctx, event); err != nil {
//			return fmt.Errorf("event validation failed: %w", err)
//		}
//
//		// Handle different event types
//		switch event.Type() {
//		case events.EventTypeRunStarted:
//			return handleRunStarted(event.(*events.RunStartedEvent))
//		case events.EventTypeTextMessageStart:
//			return handleMessageStart(event.(*events.TextMessageStartEvent))
//		default:
//			return fmt.Errorf("unsupported event type: %s", event.Type())
//		}
//	}
//
// # Testing Utilities
//
// The package provides utilities for testing:
//
//	func TestEventSequence(t *testing.T) {
//		// Create test sequence
//		sequence := []events.Event{
//			events.NewRunStartedEvent("test-thread", "test-run"),
//			events.NewRunFinishedEvent("test-thread", "test-run"),
//		}
//
//		// Validate sequence
//		assert.NoError(t, events.ValidateSequence(sequence))
//
//		// Test JSON round-trip
//		for _, event := range sequence {
//			jsonData, err := event.ToJSON()
//			assert.NoError(t, err)
//
//			parsed, err := events.EventFromJSON(jsonData)
//			assert.NoError(t, err)
//			assert.Equal(t, event.Type(), parsed.Type())
//		}
//	}
//
// # Cross-SDK Compatibility
//
// Events are designed for cross-SDK compatibility with TypeScript and Python:
//
//	// JSON field names match TypeScript/Python conventions
//	event := events.NewRunStartedEvent("thread-123", "run-456")
//	jsonData, _ := event.ToJSON()
//	// Output: {"type":"RUN_STARTED","timestamp":1672531200000,"threadId":"thread-123","runId":"run-456"}
//
//	// Protocol Buffer schemas are shared across SDKs
//	pbEvent, _ := event.ToProtobuf()
//	// Can be consumed by TypeScript/Python clients
//
// # Helper Functions
//
// Utility functions for common patterns:
//
//	func stringPtr(s string) *string {
//		return &s
//	}
//
//	func createMessageSequence(messageID, content string) []events.Event {
//		return []events.Event{
//			events.NewTextMessageStartEvent(messageID, events.WithRole("user")),
//			events.NewTextMessageContentEvent(messageID, content),
//			events.NewTextMessageEndEvent(messageID),
//		}
//	}
//
//	func createToolCallSequence(toolCallID, toolName, args string) []events.Event {
//		return []events.Event{
//			events.NewToolCallStartEvent(toolCallID, toolName),
//			events.NewToolCallArgsEvent(toolCallID, args),
//			events.NewToolCallEndEvent(toolCallID),
//		}
//	}
//
// # Memory Management
//
// For long-running applications, the validator provides automatic cleanup:
//
//	// Create validator with cleanup for production use
//	validator := events.NewEventValidator(events.ProductionValidationConfig())
//
//	// Start automatic cleanup routine
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	// Clean up items older than 24 hours every hour
//	validator.StartCleanupRoutine(ctx, time.Hour, 24*time.Hour)
//
//	// Monitor memory usage
//	stats := validator.GetState().GetMemoryStats()
//	log.Printf("Active runs: %d, Finished runs: %d",
//		stats["active_runs"], stats["finished_runs"])
//
//	// Manual cleanup if needed
//	validator.GetState().CleanupFinishedItems(time.Now().Add(-12*time.Hour))
package events
