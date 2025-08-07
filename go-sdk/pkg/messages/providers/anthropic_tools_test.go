package providers

import (
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
)

// TestAnthropicToolCallConversion tests converting messages with tool calls
func TestAnthropicToolCallConversion(t *testing.T) {
	converter := NewAnthropicConverter()

	toolCalls := []messages.ToolCall{
		{
			ID:       "call_123",
			Type:     "function",
			Function: messages.FunctionCall{Name: "get_weather", Arguments: `{"city": "San Francisco"}`},
		},
		{
			ID:       "call_456",
			Type:     "function",
			Function: messages.FunctionCall{Name: "calculate_tip", Arguments: `{"amount": 50.00, "percentage": 0.18}`},
		},
	}

	assistantMsg := messages.NewAssistantMessage("I'll help you with that.")
	assistantMsg.SetToolCalls(toolCalls)

	msgs := messages.MessageList{
		messages.NewUserMessage("What's the weather and calculate a tip?"),
		assistantMsg,
	}

	result, err := converter.ToProviderFormat(msgs)
	if err != nil {
		t.Fatalf("Failed to convert messages: %v", err)
	}

	request, ok := result.(AnthropicRequest)
	if !ok {
		t.Fatalf("Expected AnthropicRequest, got %T", result)
	}

	// Find the assistant message
	var assistantConv AnthropicMessage
	found := false
	for _, msg := range request.Messages {
		if msg.Role == "assistant" {
			assistantConv = msg
			found = true
			break
		}
	}

	if !found {
		t.Fatal("Assistant message not found in conversion")
	}

	// Check that tool calls are properly converted
	content := assistantConv.Content
	if len(content) == 0 {
		t.Fatal("Expected content to have items")
	}

	// Should have text content and tool use blocks
	textFound := false
	toolUseCount := 0

	for _, item := range content {
		switch item.Type {
		case "text":
			textFound = true
		case "tool_use":
			toolUseCount++
			// Verify tool use structure
			if item.Name == nil {
				t.Error("Tool use should have name field")
			}
			if item.Input == nil {
				t.Error("Tool use should have input field")
			}
		}
	}

	if !textFound {
		t.Error("Text content not found in assistant message")
	}

	if toolUseCount != 2 {
		t.Errorf("Expected 2 tool use blocks, got %d", toolUseCount)
	}
}

// TestAnthropicToolResultConversion tests converting tool result messages
func TestAnthropicToolResultConversion(t *testing.T) {
	converter := NewAnthropicConverter()

	toolMsg := messages.NewToolMessage("The weather in San Francisco is sunny, 72°F", "call_123")

	msgs := messages.MessageList{
		messages.NewUserMessage("What's the weather?"),
		toolMsg,
	}

	result, err := converter.ToProviderFormat(msgs)
	if err != nil {
		t.Fatalf("Failed to convert messages: %v", err)
	}

	request, ok := result.(AnthropicRequest)
	if !ok {
		t.Fatalf("Expected AnthropicRequest, got %T", result)
	}

	// Find the tool result message (should be converted to user message)
	var toolResultMsg AnthropicMessage
	found := false
	for _, msg := range request.Messages {
		if msg.Role == "user" {
			if len(msg.Content) > 0 && msg.Content[0].Type == "tool_result" {
				toolResultMsg = msg
				found = true
				break
			}
		}
	}

	if !found {
		t.Fatal("Tool result message not found in conversion")
	}

	// Verify tool result structure
	if len(toolResultMsg.Content) == 0 {
		t.Fatal("Tool result message should have content")
	}

	firstContent := toolResultMsg.Content[0]
	if firstContent.Type != "tool_result" {
		t.Errorf("Expected type 'tool_result', got %v", firstContent.Type)
	}

	if firstContent.ToolUseID == nil || *firstContent.ToolUseID != "call_123" {
		var toolUseID string
		if firstContent.ToolUseID == nil {
			toolUseID = "<nil>"
		} else {
			toolUseID = *firstContent.ToolUseID
		}
		t.Errorf("Expected tool_use_id 'call_123', got %v", toolUseID)
	}

	if firstContent.Content == nil || *firstContent.Content != "The weather in San Francisco is sunny, 72°F" {
		var content string
		if firstContent.Content == nil {
			content = "<nil>"
		} else {
			content = *firstContent.Content
		}
		t.Errorf("Unexpected tool result content: %v", content)
	}
}

// TestAnthropicComplexToolFlow tests a complete tool call flow
func TestAnthropicComplexToolFlow(t *testing.T) {
	converter := NewAnthropicConverter()

	// Tool call message
	toolCall := messages.ToolCall{
		ID:       "call_weather",
		Type:     "function",
		Function: messages.FunctionCall{Name: "get_weather", Arguments: `{"city": "Tokyo"}`},
	}

	assistantMsg := messages.NewAssistantMessage("Let me check the weather for you.")
	assistantMsg.SetToolCalls([]messages.ToolCall{toolCall})

	// Tool result message
	toolResult := messages.NewToolMessage("Tokyo: Cloudy, 18°C, humidity 65%", "call_weather")

	// Follow-up assistant message
	finalMsg := messages.NewAssistantMessage("The weather in Tokyo is currently cloudy with a temperature of 18°C.")

	msgs := messages.MessageList{
		messages.NewUserMessage("What's the weather like in Tokyo?"),
		assistantMsg,
		toolResult,
		finalMsg,
	}

	result, err := converter.ToProviderFormat(msgs)
	if err != nil {
		t.Fatalf("Failed to convert messages: %v", err)
	}

	request, ok := result.(AnthropicRequest)
	if !ok {
		t.Fatalf("Expected AnthropicRequest, got %T", result)
	}

	// Should have 4 messages: user, assistant (with tool), user (with tool result), assistant (final)
	if len(request.Messages) != 4 {
		t.Errorf("Expected 4 messages, got %d", len(request.Messages))
	}

	// Verify sequence: user, assistant, user, assistant
	expectedRoles := []string{"user", "assistant", "user", "assistant"}
	for i, msg := range request.Messages {
		if msg.Role != expectedRoles[i] {
			t.Errorf("Message %d: expected role %s, got %s", i, expectedRoles[i], msg.Role)
		}
	}

	// Verify assistant message has tool use
	assistantWithTool := request.Messages[1]
	if len(assistantWithTool.Content) == 0 {
		t.Fatal("Assistant message with tool should have content")
	}

	hasToolUse := false
	for _, item := range assistantWithTool.Content {
		if item.Type == "tool_use" {
			hasToolUse = true
			break
		}
	}

	if !hasToolUse {
		t.Error("Assistant message should contain tool use")
	}

	// Verify tool result message
	toolResultUser := request.Messages[2]
	if toolResultUser.Role != "user" {
		t.Error("Tool result should be converted to user message")
	}
}

// TestAnthropicInvalidToolHandling tests error handling for invalid tool data
func TestAnthropicInvalidToolHandling(t *testing.T) {
	converter := NewAnthropicConverter()

	// Create tool call with invalid JSON arguments
	toolCall := messages.ToolCall{
		ID:       "call_invalid",
		Type:     "function",
		Function: messages.FunctionCall{Name: "test_func", Arguments: `{"invalid": json}`},
	}

	assistantMsg := messages.NewAssistantMessage("Processing...")
	assistantMsg.SetToolCalls([]messages.ToolCall{toolCall})

	msgs := messages.MessageList{
		messages.NewUserMessage("Test"),
		assistantMsg,
	}

	// Should handle gracefully (either fix the JSON or provide error indication)
	result, err := converter.ToProviderFormat(msgs)
	if err != nil {
		// If it errors, that's acceptable for invalid JSON
		t.Logf("Expected error for invalid JSON: %v", err)
		return
	}

	// If it doesn't error, verify the result is reasonable
	request, ok := result.(AnthropicRequest)
	if !ok {
		t.Fatalf("Expected AnthropicRequest, got %T", result)
	}

	if len(request.Messages) == 0 {
		t.Error("Should have at least some messages even with invalid tool JSON")
	}
}
