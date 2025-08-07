package messages

import (
	"testing"
	"time"
)

// TestHistoryRoleFiltering tests filtering messages by role
func TestHistoryRoleFiltering(t *testing.T) {
	h := NewHistory()

	// Add mixed role messages
	userMsg1 := NewUserMessage("User message 1")
	assistantMsg1 := NewAssistantMessage("Assistant message 1")
	userMsg2 := NewUserMessage("User message 2")
	assistantMsg2 := NewAssistantMessage("Assistant message 2")
	systemMsg := NewSystemMessage("System message")

	messages := []Message{userMsg1, assistantMsg1, userMsg2, assistantMsg2, systemMsg}
	if err := h.AddBatch(messages); err != nil {
		t.Errorf("Failed to add batch: %v", err)
	}

	// Get user messages
	userMessages, err := h.GetByRole(RoleUser)
	if err != nil {
		t.Errorf("Failed to get user messages: %v", err)
	}
	if len(userMessages) != 2 {
		t.Errorf("Expected 2 user messages, got %d", len(userMessages))
	}

	// Get assistant messages
	assistantMessages, err := h.GetByRole(RoleAssistant)
	if err != nil {
		t.Errorf("Failed to get assistant messages: %v", err)
	}
	if len(assistantMessages) != 2 {
		t.Errorf("Expected 2 assistant messages, got %d", len(assistantMessages))
	}

	// Get system messages
	systemMessages, err := h.GetByRole(RoleSystem)
	if err != nil {
		t.Errorf("Failed to get system messages: %v", err)
	}
	if len(systemMessages) != 1 {
		t.Errorf("Expected 1 system message, got %d", len(systemMessages))
	}

	// Verify content of filtered messages
	for _, msg := range userMessages {
		if msg.GetRole() != RoleUser {
			t.Error("Non-user message in user filter results")
		}
	}
}

// TestHistoryTimestampFiltering tests filtering messages by timestamp
func TestHistoryTimestampFiltering(t *testing.T) {
	h := NewHistory()

	baseTime := time.Now()

	// Add messages with different timestamps
	msg1 := NewUserMessage("Message 1")
	msg1.SetTimestamp(baseTime.Add(-2 * time.Hour))

	msg2 := NewUserMessage("Message 2")
	msg2.SetTimestamp(baseTime.Add(-1 * time.Hour))

	msg3 := NewUserMessage("Message 3")
	msg3.SetTimestamp(baseTime)

	messages := []Message{msg1, msg2, msg3}
	if err := h.AddBatch(messages); err != nil {
		t.Errorf("Failed to add batch: %v", err)
	}

	// Get messages after 1.5 hours ago
	cutoffTime := baseTime.Add(-90 * time.Minute)
	recentMessages, err := h.GetAfter(cutoffTime)
	if err != nil {
		t.Errorf("Failed to get messages after timestamp: %v", err)
	}

	if len(recentMessages) != 2 {
		t.Errorf("Expected 2 recent messages, got %d", len(recentMessages))
	}

	// Verify timestamps
	for _, msg := range recentMessages {
		if msg.GetTimestamp().Before(cutoffTime) {
			t.Error("Message timestamp is before cutoff")
		}
	}

	// Test with future timestamp (should return no messages)
	futureMessages, err := h.GetAfter(baseTime.Add(1 * time.Hour))
	if err != nil {
		t.Errorf("Failed to get future messages: %v", err)
	}
	if len(futureMessages) != 0 {
		t.Errorf("Expected 0 future messages, got %d", len(futureMessages))
	}
}

// TestHistorySearch tests message content search functionality
func TestHistorySearch(t *testing.T) {
	h := NewHistory()

	// Add messages with various content
	messages := []Message{
		NewUserMessage("Hello world"),
		NewAssistantMessage("How can I help you?"),
		NewUserMessage("I need help with golang"),
		NewAssistantMessage("I can help you with Go programming"),
		NewUserMessage("What about Python?"),
		NewSystemMessage("System: User preferences updated"),
	}

	if err := h.AddBatch(messages); err != nil {
		t.Errorf("Failed to add batch: %v", err)
	}

	// Search for "help"
	helpMessages, err := h.Search("help")
	if err != nil {
		t.Errorf("Failed to search for 'help': %v", err)
	}
	if len(helpMessages) != 3 { // "help you", "need help", "help you with"
		t.Errorf("Expected 3 messages containing 'help', got %d", len(helpMessages))
	}

	// Search for case-insensitive "GOLANG"
	golangMessages, err := h.Search("GOLANG")
	if err != nil {
		t.Errorf("Failed to search for 'GOLANG': %v", err)
	}
	if len(golangMessages) != 1 {
		t.Errorf("Expected 1 message containing 'golang', got %d", len(golangMessages))
	}

	// Search for non-existent term
	noMessages, err := h.Search("nonexistent")
	if err != nil {
		t.Errorf("Failed to search for non-existent term: %v", err)
	}
	if len(noMessages) != 0 {
		t.Errorf("Expected 0 messages for non-existent term, got %d", len(noMessages))
	}

	// Search with empty string (should return all messages)
	allMessages, err := h.Search("")
	if err != nil {
		t.Errorf("Failed to search with empty string: %v", err)
	}
	if len(allMessages) != len(messages) {
		t.Errorf("Expected %d messages for empty search, got %d", len(messages), len(allMessages))
	}
}

// TestHistoryContentTypeFiltering tests filtering by content types
func TestHistoryContentTypeFiltering(t *testing.T) {
	h := NewHistory()

	// Add messages with different content types
	textMsg := NewUserMessage("Plain text message")

	// Add structured message (assuming we have this functionality)
	structuredMsg := NewUserMessage("Structured content")
	// Simulate structured content by setting metadata
	structuredMsg.SetMetadata(map[string]interface{}{
		"type": "structured",
		"data": map[string]string{"key": "value"},
	})

	messages := []Message{textMsg, structuredMsg}
	if err := h.AddBatch(messages); err != nil {
		t.Errorf("Failed to add batch: %v", err)
	}

	// Search by content pattern
	textMessages, err := h.Search("text")
	if err != nil {
		t.Errorf("Failed to search for text messages: %v", err)
	}
	if len(textMessages) != 1 {
		t.Errorf("Expected 1 text message, got %d", len(textMessages))
	}

	structuredMessages, err := h.Search("Structured")
	if err != nil {
		t.Errorf("Failed to search for structured messages: %v", err)
	}
	if len(structuredMessages) != 1 {
		t.Errorf("Expected 1 structured message, got %d", len(structuredMessages))
	}
}
