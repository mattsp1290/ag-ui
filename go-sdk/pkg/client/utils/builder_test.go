package utils

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// MockClient provides a test implementation of the client.Client interface  
type MockClient struct {
	client.Client
}

func (m *MockClient) Name() string { return "mock-client" }

// createMockEventHandler creates a mock event handler function
func createMockEventHandler(name string) events.EventHandler {
	return func(ctx context.Context, event events.BusEvent) error {
		// Mock handler implementation - just log that it was called
		_ = event
		return nil
	}
}

// MockMessage for testing
type MockMessage struct {
	id        string
	role      messages.MessageRole
	content   string
	timestamp time.Time
	name      string
}

func (m *MockMessage) GetID() string                     { return m.id }
func (m *MockMessage) GetRole() messages.MessageRole    { return m.role }
func (m *MockMessage) GetContent() *string              { return &m.content }
func (m *MockMessage) GetName() *string                 { if m.name == "" { return nil }; return &m.name }
func (m *MockMessage) GetTimestamp() time.Time          { return m.timestamp }
func (m *MockMessage) GetMetadata() *messages.MessageMetadata { return &messages.MessageMetadata{} }
func (m *MockMessage) SetTimestamp(t time.Time)         { m.timestamp = t }
func (m *MockMessage) SetMetadata(meta map[string]interface{}) { }
func (m *MockMessage) Validate() error                  { return nil }
func (m *MockMessage) ToJSON() ([]byte, error)         { return nil, nil }

// mockEventFilter for testing
type mockEventFilter struct {
	name string
}

func (f *mockEventFilter) Apply(event events.Event) bool { return true }
func (f *mockEventFilter) Name() string                  { return f.name }

func TestNewFluentClient(t *testing.T) {
	mockClient := (*client.Client)(nil) // Use nil for now
	fluentClient := NewFluentClient(mockClient)

	if fluentClient == nil {
		t.Fatal("NewFluentClient returned nil")
	}
	if fluentClient.client != mockClient {
		t.Error("FluentClient client not set correctly")
	}
	if fluentClient.utils == nil {
		t.Error("FluentClient utils not initialized")
	}
	if fluentClient.context == nil {
		t.Error("FluentClient context not initialized")
	}
	if fluentClient.utils.Agent == nil {
		t.Error("AgentUtils not initialized")
	}
	if fluentClient.utils.State == nil {
		t.Error("StateUtils not initialized")
	}
	if fluentClient.utils.Message == nil {
		t.Error("MessageUtils not initialized")
	}
	if fluentClient.utils.Event == nil {
		t.Error("EventUtils not initialized")
	}
	if fluentClient.utils.Diagnostics == nil {
		t.Error("DiagnosticsUtils not initialized")
	}
	if fluentClient.utils.Common == nil {
		t.Error("CommonUtils not initialized")
	}
}

func TestFluentClient_WithContext(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)

	ctx := context.WithValue(context.Background(), "test", "value")
	result := fluentClient.WithContext(ctx)

	if result != fluentClient {
		t.Error("WithContext should return same instance")
	}
	if fluentClient.context != ctx {
		t.Error("Context not set correctly")
	}
}

func TestFluentClient_NewAgent(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)

	builder := fluentClient.NewAgent("test-agent")

	if builder == nil {
		t.Fatal("NewAgent returned nil")
	}
	if builder.name != "test-agent" {
		t.Errorf("Expected name 'test-agent', got %s", builder.name)
	}
	if builder.client != fluentClient {
		t.Error("Builder client not set correctly")
	}
	if builder.config == nil {
		t.Error("Builder config not initialized")
	}
}

func TestFluentClient_Messages(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)

	msgs := []messages.Message{
		&MockMessage{id: "1", role: messages.RoleUser, content: "Hello"},
	}
	builder := fluentClient.Messages(msgs)

	if builder == nil {
		t.Fatal("Messages returned nil")
	}
	if len(builder.messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(builder.messages))
	}
	if builder.client != fluentClient {
		t.Error("Builder client not set correctly")
	}
}

func TestFluentClient_EventStream(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")

	builder := fluentClient.EventStream(agent)

	if builder == nil {
		t.Fatal("EventStream returned nil")
	}
	if builder.agent != agent {
		t.Error("Builder agent not set correctly")
	}
	if builder.client != fluentClient {
		t.Error("Builder client not set correctly")
	}
}

func TestFluentClient_State(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")

	builder := fluentClient.State(agent)

	if builder == nil {
		t.Fatal("State returned nil")
	}
	if builder.agent != agent {
		t.Error("Builder agent not set correctly")
	}
	if builder.client != fluentClient {
		t.Error("Builder client not set correctly")
	}
}

func TestFluentClient_ErrorHandling(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)

	// Initially no errors
	if fluentClient.HasErrors() {
		t.Error("Should not have errors initially")
	}
	if len(fluentClient.GetErrors()) != 0 {
		t.Error("Should have zero errors initially")
	}

	// Add error
	fluentClient.addError(errors.NewValidationError("test", "test error"))
	
	if !fluentClient.HasErrors() {
		t.Error("Should have errors after adding one")
	}
	if len(fluentClient.GetErrors()) != 1 {
		t.Error("Should have one error")
	}
}

// AgentBuilder Tests

func TestAgentBuilder_WithConfig(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	builder := fluentClient.NewAgent("test-agent")

	config := &client.AgentConfig{
		Name: "test-agent",
		Description: "Test agent config",
	}

	result := builder.WithConfig(config)

	if result != builder {
		t.Error("WithConfig should return same builder instance")
	}
	if builder.config != config {
		t.Error("Config not set correctly")
	}
	if len(builder.errors) != 0 {
		t.Error("Should not have errors for valid config")
	}
}

func TestAgentBuilder_WithConfig_Nil(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	builder := fluentClient.NewAgent("test-agent")

	result := builder.WithConfig(nil)

	if result != builder {
		t.Error("WithConfig should return same builder instance")
	}
	if len(builder.errors) != 1 {
		t.Error("Should have error for nil config")
	}
}

func TestAgentBuilder_WithTools(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	builder := fluentClient.NewAgent("test-agent")

	result := builder.WithTools("tool1", "tool2", "tool3")

	if result != builder {
		t.Error("WithTools should return same builder instance")
	}
	tools := builder.GetTools()
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}
	expectedTools := []string{"tool1", "tool2", "tool3"}
	for i, tool := range expectedTools {
		if tools[i] != tool {
			t.Errorf("Expected tool %s at index %d, got %s", tool, i, tools[i])
		}
	}

	// Test chaining multiple calls
	result = builder.WithTools("tool4", "tool5")
	tools = builder.GetTools()
	if len(tools) != 5 {
		t.Errorf("Expected 5 tools after chaining, got %d", len(tools))
	}
}

func TestAgentBuilder_WithState(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	builder := fluentClient.NewAgent("test-agent")

	state := map[string]interface{}{"key": "value"}
	result := builder.WithState(state)

	if result != builder {
		t.Error("WithState should return same builder instance")
	}
	// Cannot compare maps directly, check if state was set
	if builder.state == nil {
		t.Error("State not set correctly")
	}
}

func TestAgentBuilder_WithEventHandler(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	builder := fluentClient.NewAgent("test-agent")

	handler := createMockEventHandler("test-handler")
	result := builder.WithEventHandler(handler)

	if result != builder {
		t.Error("WithEventHandler should return same builder instance")
	}
	if len(builder.handlers) != 1 {
		t.Error("Handler not added")
	}
	// Check handler was added by verifying length and name
	if len(builder.handlers) == 0 || builder.handlers[0] == nil {
		t.Error("Handler not added correctly")
	}
	if len(builder.errors) != 0 {
		t.Error("Should not have errors for valid handler")
	}
}

func TestAgentBuilder_WithEventHandler_Nil(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	builder := fluentClient.NewAgent("test-agent")

	result := builder.WithEventHandler(nil)

	if result != builder {
		t.Error("WithEventHandler should return same builder instance")
	}
	if len(builder.errors) != 1 {
		t.Error("Should have error for nil handler")
	}
}

func TestAgentBuilder_WithRetryPolicy(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	builder := fluentClient.NewAgent("test-agent")

	policy := &RetryPolicy{
		MaxAttempts: 3,
		Delay:       1 * time.Second,
		BackoffType: BackoffTypeExponential,
		MaxDelay:    30 * time.Second,
	}
	result := builder.WithRetryPolicy(policy)

	if result != builder {
		t.Error("WithRetryPolicy should return same builder instance")
	}
	if builder.retryPolicy != policy {
		t.Error("Retry policy not set correctly")
	}
	if len(builder.errors) != 0 {
		t.Error("Should not have errors for valid policy")
	}
}

func TestAgentBuilder_WithRetryPolicy_Nil(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	builder := fluentClient.NewAgent("test-agent")

	result := builder.WithRetryPolicy(nil)

	if result != builder {
		t.Error("WithRetryPolicy should return same builder instance")
	}
	if len(builder.errors) != 1 {
		t.Error("Should have error for nil policy")
	}
}

func TestAgentBuilder_Build(t *testing.T) {
	t.Run("ValidBuild", func(t *testing.T) {
		mockClient := (*client.Client)(nil)
		fluentClient := NewFluentClient(mockClient)
		builder := fluentClient.NewAgent("valid-agent")

		config := &client.AgentConfig{Name: "valid-agent"}
		builder.WithConfig(config)

		// Note: Build method has placeholder implementation
		_, err := builder.Build()
		if err == nil {
			t.Error("Expected error due to placeholder implementation")
		}
	})

	t.Run("EmptyName", func(t *testing.T) {
		mockClient := (*client.Client)(nil)
		fluentClient := NewFluentClient(mockClient)
		builder := fluentClient.NewAgent("")

		_, err := builder.Build()
		if err == nil {
			t.Error("Expected error for empty agent name")
		}
	})

	t.Run("WithErrors", func(t *testing.T) {
		mockClient := (*client.Client)(nil)
		fluentClient := NewFluentClient(mockClient)
		builder := fluentClient.NewAgent("test-agent")

		// Add error by passing nil config
		builder.WithConfig(nil)

		_, err := builder.Build()
		if err == nil {
			t.Error("Expected error when builder has accumulated errors")
		}
	})
}

func TestAgentBuilder_Clone(t *testing.T) {
	t.Run("ValidSource", func(t *testing.T) {
		mockClient := (*client.Client)(nil)
		fluentClient := NewFluentClient(mockClient)
		builder := fluentClient.NewAgent("clone-agent")

		sourceAgent := NewMockAgent("source-agent", "Source agent")
		
		// Note: Clone method has placeholder implementation
		_, err := builder.Clone(sourceAgent)
		if err == nil {
			t.Error("Expected error due to placeholder implementation")
		}
	})

	t.Run("NilSource", func(t *testing.T) {
		mockClient := (*client.Client)(nil)
		fluentClient := NewFluentClient(mockClient)
		builder := fluentClient.NewAgent("clone-agent")

		_, err := builder.Clone(nil)
		if err == nil {
			t.Error("Expected error for nil source agent")
		}
	})
}

// MessageBuilder Tests

func TestMessageBuilder_FilterByType(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	msgs := []messages.Message{
		&MockMessage{id: "1", role: messages.RoleUser, content: "Hello"},
	}
	builder := fluentClient.Messages(msgs)

	result := builder.FilterByType("text", "json")

	if result != builder {
		t.Error("FilterByType should return same builder instance")
	}
	if len(builder.filters) != 1 {
		t.Error("Filter not added")
	}
}

func TestMessageBuilder_FilterByRole(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	msgs := []messages.Message{
		&MockMessage{id: "1", role: messages.RoleUser, content: "Hello"},
	}
	builder := fluentClient.Messages(msgs)

	result := builder.FilterByRole(messages.RoleUser, messages.RoleAssistant)

	if result != builder {
		t.Error("FilterByRole should return same builder instance")
	}
	if len(builder.filters) != 1 {
		t.Error("Filter not added")
	}
}

func TestMessageBuilder_FilterByContent(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	msgs := []messages.Message{
		&MockMessage{id: "1", role: messages.RoleUser, content: "Hello"},
	}
	builder := fluentClient.Messages(msgs)

	result := builder.FilterByContent("test.*pattern")

	if result != builder {
		t.Error("FilterByContent should return same builder instance")
	}
	// Should have an error due to placeholder implementation
	if len(builder.errors) != 1 {
		t.Error("Expected error due to placeholder implementation")
	}
}

func TestMessageBuilder_TransformWith(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	msgs := []messages.Message{
		&MockMessage{id: "1", role: messages.RoleUser, content: "Hello"},
	}
	builder := fluentClient.Messages(msgs)

	result := builder.TransformWith("json_transformer")

	if result != builder {
		t.Error("TransformWith should return same builder instance")
	}
	if builder.transformer != "json_transformer" {
		t.Error("Transformer not set correctly")
	}
}

func TestMessageBuilder_ExportToJSON(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	msgs := []messages.Message{
		&MockMessage{id: "1", role: messages.RoleUser, content: "Hello"},
	}
	builder := fluentClient.Messages(msgs)

	data, err := builder.ExportToJSON()
	if err != nil {
		t.Fatalf("ExportToJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Exported data should not be empty")
	}
}

func TestMessageBuilder_ExportToCSV(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	msgs := []messages.Message{
		&MockMessage{id: "1", role: messages.RoleUser, content: "Hello"},
	}
	builder := fluentClient.Messages(msgs)

	data, err := builder.ExportToCSV()
	if err != nil {
		t.Fatalf("ExportToCSV failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Exported data should not be empty")
	}
}

func TestMessageBuilder_GetStatistics(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	msgs := []messages.Message{
		&MockMessage{id: "1", role: messages.RoleUser, content: "Hello"},
		&MockMessage{id: "2", role: messages.RoleAssistant, content: "Hi there"},
	}
	builder := fluentClient.Messages(msgs)

	stats, err := builder.GetStatistics()
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}
	if stats == nil {
		t.Error("Statistics should not be nil")
	}
}

// EventStreamBuilder Tests

func TestEventStreamBuilder_Filter(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")
	builder := fluentClient.EventStream(agent)

	// Create a mock filter
	filter := &mockEventFilter{name: "test-filter"}

	result := builder.Filter(filter)

	if result != builder {
		t.Error("Filter should return same builder instance")
	}
}

func TestEventStreamBuilder_Filter_Nil(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")
	builder := fluentClient.EventStream(agent)

	result := builder.Filter(nil)

	if result != builder {
		t.Error("Filter should return same builder instance")
	}
	if len(builder.errors) != 1 {
		t.Error("Should have error for nil filter")
	}
}

func TestEventStreamBuilder_Window(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")
	builder := fluentClient.EventStream(agent)

	result := builder.Window(WindowTypeTime, 100, 5*time.Second)

	if result != builder {
		t.Error("Window should return same builder instance")
	}
	if len(builder.windows) != 1 {
		t.Error("Window not added")
	}
}

func TestEventStreamBuilder_Process(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")
	builder := fluentClient.EventStream(agent)

	result := builder.Process("test-processor")

	if result != builder {
		t.Error("Process should return same builder instance")
	}
	if builder.processor != "test-processor" {
		t.Error("Processor not set correctly")
	}
}

// StateBuilder Tests

func TestStateBuilder_Compare(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")
	builder := fluentClient.State(agent)

	previousState := map[string]interface{}{"key": "value"}
	result := builder.Compare(previousState)

	if result != builder {
		t.Error("Compare should return same builder instance")
	}
	// Cannot compare maps directly, check if state was set
	if builder.compareWith == nil {
		t.Error("Previous state not set correctly")
	}
}

func TestStateBuilder_IgnoreFields(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")
	builder := fluentClient.State(agent)

	result := builder.IgnoreFields("timestamp", "version")

	if result != builder {
		t.Error("IgnoreFields should return same builder instance")
	}
	if len(builder.ignoreFields) != 2 {
		t.Error("Ignore fields not set correctly")
	}
}

func TestStateBuilder_WithDiffOptions(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")
	builder := fluentClient.State(agent)

	options := &DiffOptions{
		MaxDepth: 5,
		ComparisonMode: ComparisonModeShallow,
	}
	result := builder.WithDiffOptions(options)

	if result != builder {
		t.Error("WithDiffOptions should return same builder instance")
	}
	if builder.diffOptions != options {
		t.Error("Diff options not set correctly")
	}
	if len(builder.errors) != 0 {
		t.Error("Should not have errors for valid options")
	}
}

func TestStateBuilder_WithDiffOptions_Nil(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")
	builder := fluentClient.State(agent)

	result := builder.WithDiffOptions(nil)

	if result != builder {
		t.Error("WithDiffOptions should return same builder instance")
	}
	if len(builder.errors) != 1 {
		t.Error("Should have error for nil options")
	}
}

func TestStateBuilder_ValidateWith(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")
	builder := fluentClient.State(agent)

	result := builder.ValidateWith("schema-validator")

	if result != builder {
		t.Error("ValidateWith should return same builder instance")
	}
	if builder.validatorName != "schema-validator" {
		t.Error("Validator name not set correctly")
	}
}

// Helper function tests

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy == nil {
		t.Fatal("DefaultRetryPolicy returned nil")
	}
	if policy.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts 3, got %d", policy.MaxAttempts)
	}
	if policy.Delay != 1*time.Second {
		t.Errorf("Expected Delay 1s, got %v", policy.Delay)
	}
	if policy.BackoffType != BackoffTypeExponential {
		t.Errorf("Expected BackoffType exponential, got %v", policy.BackoffType)
	}
	if policy.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay 30s, got %v", policy.MaxDelay)
	}
}

func TestCreateTimeWindow(t *testing.T) {
	duration := 5 * time.Second
	window := CreateTimeWindow(duration)

	if window.Type != WindowTypeTime {
		t.Errorf("Expected WindowType time, got %v", window.Type)
	}
	if window.Duration != duration {
		t.Errorf("Expected Duration %v, got %v", duration, window.Duration)
	}
}

func TestCreateCountWindow(t *testing.T) {
	size := 100
	window := CreateCountWindow(size)

	if window.Type != WindowTypeCount {
		t.Errorf("Expected WindowType count, got %v", window.Type)
	}
	if window.Size != size {
		t.Errorf("Expected Size %d, got %d", size, window.Size)
	}
}

func TestCreateSlidingWindow(t *testing.T) {
	duration := 10 * time.Second
	overlap := 2 * time.Second
	window := CreateSlidingWindow(duration, overlap)

	if window.Type != WindowTypeSliding {
		t.Errorf("Expected WindowType sliding, got %v", window.Type)
	}
	if window.Duration != duration {
		t.Errorf("Expected Duration %v, got %v", duration, window.Duration)
	}
	if window.Overlap != overlap {
		t.Errorf("Expected Overlap %v, got %v", overlap, window.Overlap)
	}
}

// Factory method tests

func TestNewDefaultAgentBuilder(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)

	builder := fluentClient.NewDefaultAgentBuilder("default-agent")

	if builder == nil {
		t.Fatal("NewDefaultAgentBuilder returned nil")
	}
	if builder.name != "default-agent" {
		t.Errorf("Expected name 'default-agent', got %s", builder.name)
	}
	if builder.retryPolicy == nil {
		t.Error("Retry policy not set")
	}
}

func TestNewMessageFilterBuilder(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	msgs := []messages.Message{
		&MockMessage{id: "1", role: messages.RoleUser, content: "Hello"},
	}

	builder := fluentClient.NewMessageFilterBuilder(msgs)

	if builder == nil {
		t.Fatal("NewMessageFilterBuilder returned nil")
	}
	if len(builder.messages) != 1 {
		t.Error("Messages not set correctly")
	}
	if len(builder.filters) != 1 {
		t.Error("Default filters not applied")
	}
}

func TestNewEventProcessorBuilder(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	agent := NewMockAgent("test-agent", "Test agent")

	builder := fluentClient.NewEventProcessorBuilder(agent)

	if builder == nil {
		t.Fatal("NewEventProcessorBuilder returned nil")
	}
	if builder.agent != agent {
		t.Error("Agent not set correctly")
	}
	if len(builder.windows) != 1 {
		t.Error("Default window not set")
	}
}

// Concurrency tests

func TestFluentClient_ConcurrentAccess(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)

	var wg sync.WaitGroup
	numRoutines := 10

	// Test concurrent builder creation
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			agentName := fmt.Sprintf("concurrent-agent-%d", id)
			builder := fluentClient.NewAgent(agentName)
			
			if builder == nil {
				t.Errorf("Builder %d is nil", id)
				return
			}
			
			if builder.name != agentName {
				t.Errorf("Builder %d name mismatch", id)
			}
		}(i)
	}

	wg.Wait()
}

func TestAgentBuilder_ConcurrentChaining(t *testing.T) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	builder := fluentClient.NewAgent("concurrent-chain-agent")

	var wg sync.WaitGroup
	numRoutines := 5

	// Test concurrent method chaining
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			tool := fmt.Sprintf("tool-%d", id)
			result := builder.WithTools(tool)
			
			if result != builder {
				t.Errorf("Chaining failed for routine %d", id)
			}
		}(i)
	}

	wg.Wait()

	// Verify all tools were added (order may vary)
	tools := builder.GetTools()
	if len(tools) != numRoutines {
		t.Errorf("Expected %d tools, got %d", numRoutines, len(tools))
	}
}

// Benchmark tests

func BenchmarkFluentClient_NewAgent(b *testing.B) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := fluentClient.NewAgent("bench-agent")
		_ = builder
	}
}

func BenchmarkAgentBuilder_Chaining(b *testing.B) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := fluentClient.NewAgent("bench-chain-agent").
			WithTools("tool1", "tool2").
			WithState(map[string]interface{}{"key": "value"}).
			WithRetryPolicy(DefaultRetryPolicy())
		_ = builder
	}
}

func BenchmarkMessageBuilder_Processing(b *testing.B) {
	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)
	msgs := make([]messages.Message, 100)
	for i := 0; i < 100; i++ {
		msgs[i] = &MockMessage{
			id:      fmt.Sprintf("msg-%d", i),
			role:    messages.RoleUser,
			content: fmt.Sprintf("Message %d content", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := fluentClient.Messages(msgs).
			FilterByRole(messages.RoleUser).
			TransformWith("text")
		_, err := builder.GetStatistics()
		if err != nil {
			b.Fatalf("Statistics failed: %v", err)
		}
	}
}

// Memory leak tests

func TestMemoryLeak_BuilderCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	mockClient := (*client.Client)(nil)
	fluentClient := NewFluentClient(mockClient)

	// Create and discard many builders
	for i := 0; i < 1000; i++ {
		builder := fluentClient.NewAgent(fmt.Sprintf("memory-test-agent-%d", i)).
			WithTools("tool1", "tool2").
			WithState(map[string]interface{}{"test": i})
		_ = builder
	}

	// Force GC
	runtime.GC()
	
	// This test mainly checks that we don't panic or leak goroutines
	// More sophisticated memory profiling would be needed for comprehensive testing
}