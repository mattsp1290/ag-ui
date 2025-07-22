package messages

import (
	"fmt"
	"testing"
)

// TestHistoryBasicOperations tests basic add/retrieve operations
func TestHistoryBasicOperations(t *testing.T) {
	h := NewHistory()

	msg1 := NewUserMessage("First message")
	msg2 := NewAssistantMessage("Second message")

	// Add messages
	if err := h.Add(msg1); err != nil {
		t.Errorf("Failed to add first message: %v", err)
	}
	if err := h.Add(msg2); err != nil {
		t.Errorf("Failed to add second message: %v", err)
	}

	// Check size
	if size := h.Size(); size != 2 {
		t.Errorf("Expected size 2, got %d", size)
	}

	// Retrieve by ID
	retrieved, err := h.Get(msg1.GetID())
	if err != nil {
		t.Errorf("Failed to get message: %v", err)
	}
	if retrieved.GetID() != msg1.GetID() {
		t.Error("Retrieved wrong message")
	}

	// Get all messages
	all := h.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(all))
	}
}

// TestHistoryDuplicatePrevention tests duplicate message prevention
func TestHistoryDuplicatePrevention(t *testing.T) {
	h := NewHistory()

	msg := NewUserMessage("Test message")
	if err := h.Add(msg); err != nil {
		t.Errorf("Failed to add message: %v", err)
	}

	// Try to add the same message again
	if err := h.Add(msg); err == nil {
		t.Error("Should not allow duplicate messages")
	}

	if size := h.Size(); size != 1 {
		t.Errorf("Expected size 1, got %d", size)
	}
}

// TestHistoryBatchOperations tests batch message operations
func TestHistoryBatchOperations(t *testing.T) {
	h := NewHistory()

	messages := []Message{
		NewUserMessage("Message 1"),
		NewAssistantMessage("Message 2"),
		NewUserMessage("Message 3"),
	}

	if err := h.AddBatch(messages); err != nil {
		t.Errorf("Failed to add batch: %v", err)
	}

	if size := h.Size(); size != 3 {
		t.Errorf("Expected size 3, got %d", size)
	}

	// Verify order is maintained
	all := h.GetAll()
	for i, msg := range all {
		content1 := msg.GetContent()
		content2 := messages[i].GetContent()
		if (content1 == nil && content2 != nil) || (content1 != nil && content2 == nil) || 
		   (content1 != nil && content2 != nil && *content1 != *content2) {
			t.Errorf("Message %d content mismatch", i)
		}
	}
}

// TestHistoryRangeQueries tests message range queries
func TestHistoryRangeQueries(t *testing.T) {
	h := NewHistory()

	// Add multiple messages
	messages := make([]Message, 5)
	for i := 0; i < 5; i++ {
		messages[i] = NewUserMessage(fmt.Sprintf("Message %d", i+1))
		if err := h.Add(messages[i]); err != nil {
			t.Errorf("Failed to add message %d: %v", i, err)
		}
	}

	// Get messages by range
	rangeMessages, err := h.GetRange(1, 3)
	if err != nil {
		t.Errorf("Failed to get range: %v", err)
	}

	if len(rangeMessages) != 2 {
		t.Errorf("Expected 2 messages in range, got %d", len(rangeMessages))
	}

	// Verify correct messages in range
	for i, msg := range rangeMessages {
		expectedContent := fmt.Sprintf("Message %d", i+2) // Offset by 1 due to range start
		content := msg.GetContent()
		if content == nil || *content != expectedContent {
			var actualContent string
			if content == nil {
				actualContent = "<nil>"
			} else {
				actualContent = *content
			}
			t.Errorf("Expected content '%s', got '%s'", expectedContent, actualContent)
		}
	}

	// Test invalid range
	_, err = h.GetRange(10, 15)
	if err == nil {
		t.Error("Should error on invalid range")
	}
}

// TestHistoryLastMessages tests getting last N messages
func TestHistoryLastMessages(t *testing.T) {
	h := NewHistory()

	// Add multiple messages
	for i := 0; i < 10; i++ {
		msg := NewUserMessage(fmt.Sprintf("Message %d", i+1))
		if err := h.Add(msg); err != nil {
			t.Errorf("Failed to add message %d: %v", i, err)
		}
	}

	// Get last 5 messages
	lastMessages := h.GetLast(5)
	if len(lastMessages) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(lastMessages))
	}

	// Verify correct messages (should be messages 6-10)
	for i, msg := range lastMessages {
		expectedContent := fmt.Sprintf("Message %d", i+6)
		content := msg.GetContent()
		if content == nil || *content != expectedContent {
			var actualContent string
			if content == nil {
				actualContent = "<nil>"
			} else {
				actualContent = *content
			}
			t.Errorf("Expected content '%s', got '%s'", expectedContent, actualContent)
		}
	}

	// Test getting more messages than available
	allMessages := h.GetLast(20)
	if len(allMessages) != 10 {
		t.Errorf("Expected 10 messages (all available), got %d", len(allMessages))
	}

	// Test getting zero messages
	zeroMessages := h.GetLast(0)
	if len(zeroMessages) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(zeroMessages))
	}
}