package messages

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestHistoryCompaction tests history compaction functionality
func TestHistoryCompaction(t *testing.T) {
	// Create history with size limit
	options := HistoryOptions{
		MaxMessages:      5,
		CompactThreshold: 3,
	}
	h := NewHistory(options)

	// Add more messages than the limit
	for i := 0; i < 8; i++ {
		msg := NewUserMessage(fmt.Sprintf("Message %d", i+1))
		if err := h.Add(msg); err != nil {
			t.Errorf("Failed to add message %d: %v", i, err)
		}
	}

	// Should only keep the last 5 messages
	if size := h.Size(); size != 5 {
		t.Errorf("Expected size 5 after compaction, got %d", size)
	}

	// Verify correct messages are kept (should be messages 4-8)
	all := h.GetAll()
	for i, msg := range all {
		expectedContent := fmt.Sprintf("Message %d", i+4)
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
}

// TestHistoryAgeLimit tests automatic cleanup of old messages
func TestHistoryAgeLimit(t *testing.T) {
	// Create history with age limit
	options := HistoryOptions{
		MaxAge:           100 * time.Millisecond,
		CompactThreshold: 3,
		EnableIndexing:   true, // Required for Get() method to work
	}
	h := NewHistory(options)

	// Add messages
	for i := 0; i < 3; i++ {
		msg := NewUserMessage(fmt.Sprintf("Message %d", i+1))
		if err := h.Add(msg); err != nil {
			t.Errorf("Failed to add message %d: %v", i, err)
		}
		time.Sleep(30 * time.Millisecond) // Space out message creation
	}

	initialSize := h.Size()
	if initialSize != 3 {
		t.Errorf("Expected initial size 3, got %d", initialSize)
	}

	// Wait for messages to age
	time.Sleep(200 * time.Millisecond)

	// Add a new message (this triggers lazy cleanup of aged messages)
	newMsg := NewUserMessage("New message")
	if err := h.Add(newMsg); err != nil {
		t.Errorf("Failed to add new message: %v", err)
	}

	// Should have fewer messages now (age-based cleanup is triggered on Add)
	currentSize := h.Size()
	if currentSize >= initialSize {
		t.Errorf("Expected size reduction due to age limit, got %d (was %d)", currentSize, initialSize)
	}

	// New message should still be there
	retrieved, err := h.Get(newMsg.GetID())
	if err != nil {
		t.Errorf("Failed to retrieve new message: %v", err)
	}
	if retrieved.GetID() != newMsg.GetID() {
		t.Error("New message should still be accessible")
	}
}

// TestHistoryClear tests clearing all messages
func TestHistoryClear(t *testing.T) {
	h := NewHistory()

	// Add messages
	for i := 0; i < 5; i++ {
		msg := NewUserMessage(fmt.Sprintf("Message %d", i+1))
		if err := h.Add(msg); err != nil {
			t.Errorf("Failed to add message %d: %v", i, err)
		}
	}

	if size := h.Size(); size != 5 {
		t.Errorf("Expected size 5 before clear, got %d", size)
	}

	// Clear history
	h.Clear()

	// Should be empty now
	if size := h.Size(); size != 0 {
		t.Errorf("Expected size 0 after clear, got %d", size)
	}

	// Should be able to add new messages
	newMsg := NewUserMessage("New message after clear")
	if err := h.Add(newMsg); err != nil {
		t.Errorf("Failed to add message after clear: %v", err)
	}

	if size := h.Size(); size != 1 {
		t.Errorf("Expected size 1 after adding to cleared history, got %d", size)
	}
}

// TestHistorySnapshot tests creating snapshots of history
func TestHistorySnapshot(t *testing.T) {
	h := NewHistory()

	// Add messages
	messages := []Message{
		NewUserMessage("Message 1"),
		NewAssistantMessage("Message 2"),
		NewUserMessage("Message 3"),
	}

	if err := h.AddBatch(messages); err != nil {
		t.Errorf("Failed to add batch: %v", err)
	}

	// Create snapshot
	snapshot := h.Snapshot()

	// Verify snapshot content
	if len(snapshot.Messages) != 3 {
		t.Errorf("Expected 3 messages in snapshot, got %d", len(snapshot.Messages))
	}

	if snapshot.Timestamp.IsZero() {
		t.Error("Snapshot should have creation timestamp")
	}

	// Add more messages to original history
	newMsg := NewUserMessage("New message")
	if err := h.Add(newMsg); err != nil {
		t.Errorf("Failed to add new message: %v", err)
	}

	// Original history should have 4 messages
	if size := h.Size(); size != 4 {
		t.Errorf("Expected size 4 in original history, got %d", size)
	}

	// Snapshot should still have 3 messages (immutable)
	if len(snapshot.Messages) != 3 {
		t.Errorf("Snapshot should be immutable, still expected 3 messages, got %d", len(snapshot.Messages))
	}

	// Restore from snapshot (manually add messages from snapshot)
	restoredHistory := NewHistory()
	if err := restoredHistory.AddBatch(snapshot.Messages); err != nil {
		t.Errorf("Failed to restore from snapshot: %v", err)
	}

	if size := restoredHistory.Size(); size != 3 {
		t.Errorf("Expected size 3 in restored history, got %d", size)
	}

	// Verify restored content matches original
	originalMessages := snapshot.Messages
	restoredMessages := restoredHistory.GetAll()
	for i, msg := range restoredMessages {
		content1 := msg.GetContent()
		content2 := originalMessages[i].GetContent()
		if (content1 == nil && content2 != nil) || (content1 != nil && content2 == nil) ||
			(content1 != nil && content2 != nil && *content1 != *content2) {
			t.Errorf("Restored message %d content mismatch", i)
		}
	}
}

// TestHistoryThreadSafety tests concurrent access to history
func TestHistoryThreadSafety(t *testing.T) {
	h := NewHistory()

	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 10

	// Launch multiple goroutines to add messages concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := NewUserMessage(fmt.Sprintf("Routine %d Message %d", routineID, j+1))
				if err := h.Add(msg); err != nil {
					t.Errorf("Failed to add message from routine %d: %v", routineID, err)
				}
			}
		}(i)
	}

	// Launch goroutines to read messages concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				_ = h.GetAll() // Just ensure no race conditions
				_ = h.Size()
				_, _ = h.GetByRole(RoleUser)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	expectedSize := numGoroutines * messagesPerGoroutine
	if size := h.Size(); size != expectedSize {
		t.Errorf("Expected size %d after concurrent operations, got %d", expectedSize, size)
	}

	// Verify all messages are accessible
	all := h.GetAll()
	if len(all) != expectedSize {
		t.Errorf("Expected %d messages in GetAll(), got %d", expectedSize, len(all))
	}
}
