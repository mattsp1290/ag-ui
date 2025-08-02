package providers

import (
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
)

// TestAnthropicSimpleConversion tests basic message conversion to Anthropic format
func TestAnthropicSimpleConversion(t *testing.T) {
	converter := NewAnthropicConverter()

	msgs := messages.MessageList{
		messages.NewSystemMessage("You are a helpful assistant."),
		messages.NewUserMessage("Hello!"),
		messages.NewAssistantMessage("Hi there! How can I help you?"),
	}

	result, err := converter.ToProviderFormat(msgs)
	if err != nil {
		t.Fatalf("Failed to convert messages: %v", err)
	}

	request, ok := result.(AnthropicRequest)
	if !ok {
		t.Fatalf("Expected AnthropicRequest, got %T", result)
	}

	// Check system prompt
	if request.System != "You are a helpful assistant." {
		t.Errorf("Expected system prompt, got %s", request.System)
	}

	// Check conversation messages
	if len(request.Messages) != 2 {
		t.Errorf("Expected 2 conversation messages, got %d", len(request.Messages))
	}

	// Check roles
	expectedRoles := []string{"user", "assistant"}
	for i, msg := range request.Messages {
		if msg.Role != expectedRoles[i] {
			t.Errorf("Message %d: expected role %s, got %s", i, expectedRoles[i], msg.Role)
		}
	}
}

// TestAnthropicSystemMessageHandling tests system message processing
func TestAnthropicSystemMessageHandling(t *testing.T) {
	converter := NewAnthropicConverter()

	// Test combining multiple system messages
	msgs := messages.MessageList{
		messages.NewSystemMessage("First system instruction."),
		messages.NewSystemMessage("Second system instruction."),
		messages.NewUserMessage("Hello!"),
	}

	result, err := converter.ToProviderFormat(msgs)
	if err != nil {
		t.Fatalf("Failed to convert messages: %v", err)
	}

	request, ok := result.(AnthropicRequest)
	if !ok {
		t.Fatalf("Expected AnthropicRequest, got %T", result)
	}

	// System messages should be combined
	expectedSystem := "First system instruction.\n\nSecond system instruction."
	if request.System != expectedSystem {
		t.Errorf("Expected combined system prompt, got %s", request.System)
	}

	// Only user message should remain in conversation
	if len(request.Messages) != 1 {
		t.Errorf("Expected 1 conversation message, got %d", len(request.Messages))
	}
}

// TestAnthropicDeveloperMessageConversion tests developer message handling
func TestAnthropicDeveloperMessageConversion(t *testing.T) {
	converter := NewAnthropicConverter()

	// Create developer message
	developerMsg := messages.NewMessage(messages.RoleDeveloper, "Debug information: connection established")
	msgs := messages.MessageList{
		messages.NewUserMessage("Hello"),
		developerMsg,
		messages.NewAssistantMessage("Hi there!"),
	}

	result, err := converter.ToProviderFormat(msgs)
	if err != nil {
		t.Fatalf("Failed to convert messages: %v", err)
	}

	request, ok := result.(AnthropicRequest)
	if !ok {
		t.Fatalf("Expected AnthropicRequest, got %T", result)
	}

	// Check that developer message is converted to assistant message with special format
	found := false
	for _, msg := range request.Messages {
		if msg.Role == "assistant" && len(msg.Content) > 0 && msg.Content[0].Type == "text" && msg.Content[0].Text != nil {
			textContent := *msg.Content[0].Text
			if contains(textContent, "Debug information") {
				found = true
				// Should be prefixed with [Developer Message]
				if !contains(textContent, "[Developer Message]") {
					t.Error("Developer message should be prefixed with [Developer Message]")
				}
				break
			}
		}
	}

	if !found {
		t.Error("Developer message not found in converted format")
	}
}

// TestAnthropicFromProviderFormat tests conversion from Anthropic format back to messages
func TestAnthropicFromProviderFormat(t *testing.T) {
	converter := NewAnthropicConverter()

	// Create Anthropic response
	textContent := "Hello! How can I help you today?"
	response := AnthropicResponse{
		Content: []AnthropicContent{
			{Type: "text", Text: &textContent},
		},
		Role: "assistant",
	}

	msgs, err := converter.FromProviderFormat(response)
	if err != nil {
		t.Fatalf("Failed to convert from provider format: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	msg := msgs[0]
	if msg.GetRole() != messages.RoleAssistant {
		t.Errorf("Expected assistant role, got %s", msg.GetRole())
	}

	if content := msg.GetContent(); content == nil || *content != "Hello! How can I help you today?" {
		var contentStr string
		if content == nil {
			contentStr = "<nil>"
		} else {
			contentStr = *content
		}
		t.Errorf("Unexpected content: %s", contentStr)
	}
}

// TestAnthropicErrorHandling tests error cases
func TestAnthropicErrorHandling(t *testing.T) {
	converter := NewAnthropicConverter()

	// Test with nil input
	result, err := converter.ToProviderFormat(nil)
	if err == nil {
		t.Error("Expected error for nil input")
	}
	if result != nil {
		t.Error("Expected nil result for error case")
	}

	// Test with empty messages
	emptyMsgs := messages.MessageList{}
	result, err = converter.ToProviderFormat(emptyMsgs)
	if err == nil {
		t.Error("Expected error for empty messages")
	}

	// Test FromProviderFormat with invalid input
	msgs, err := converter.FromProviderFormat("invalid")
	if err == nil {
		t.Error("Expected error for invalid input type")
	}
	if msgs != nil {
		t.Error("Expected nil messages for error case")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findInString(s, substr)
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}