package encoding

import (
	"testing"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TestOptimalBufferSizing tests the optimal buffer sizing function
func TestOptimalBufferSizing(t *testing.T) {
	testCases := []struct {
		eventType    events.EventType
		expectedSize int
		description  string
	}{
		{events.EventTypeTextMessageStart, SmallEventBufferSize, "TextMessageStart should use small buffer"},
		{events.EventTypeTextMessageContent, MediumEventBufferSize, "TextMessageContent should use medium buffer"},
		{events.EventTypeToolCallArgs, LargeEventBufferSize, "ToolCallArgs should use large buffer"},
		{events.EventTypeStateSnapshot, VeryLargeEventBufferSize, "StateSnapshot should use very large buffer"},
		{events.EventTypeUnknown, DefaultEventBufferSize, "Unknown event should use default buffer"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			size := GetOptimalBufferSize(tc.eventType)
			if size != tc.expectedSize {
				t.Errorf("Expected size %d, got %d for event type %s", tc.expectedSize, size, tc.eventType)
			}
		})
	}
}

// TestBufferPoolBasics tests basic buffer pool functionality
func TestBufferPoolBasics(t *testing.T) {
	// Test getting and returning buffers
	buf1 := GetBuffer(1024)
	if buf1 == nil {
		t.Fatal("GetBuffer returned nil")
	}

	buf2 := GetBuffer(1024)
	if buf2 == nil {
		t.Fatal("GetBuffer returned nil")
	}

	// Buffers should be different instances
	if buf1 == buf2 {
		t.Error("GetBuffer returned same buffer instance")
	}

	// Test writing to buffer
	buf1.WriteString("test data")
	if buf1.String() != "test data" {
		t.Error("Buffer write failed")
	}

	// Return buffers to pool
	PutBuffer(buf1)
	PutBuffer(buf2)

	// Test getting buffer again (should be reset)
	buf3 := GetBuffer(1024)
	if buf3.Len() != 0 {
		t.Error("Buffer was not reset when returned from pool")
	}

	PutBuffer(buf3)
}

// TestBufferPoolSizes tests different buffer pool sizes
func TestBufferPoolSizes(t *testing.T) {
	testCases := []struct {
		size        int
		description string
	}{
		{100, "Small buffer"},
		{1024, "Small buffer"},
		{8192, "Medium buffer"},
		{32768, "Medium buffer"},
		{131072, "Large buffer"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			buf := GetBuffer(tc.size)
			if buf == nil {
				t.Fatal("GetBuffer returned nil")
			}

			// Write some data
			buf.WriteString("test")
			if buf.String() != "test" {
				t.Error("Buffer write failed")
			}

			// Return buffer
			PutBuffer(buf)
		})
	}
}

// TestGetOptimalBufferSizeForEvent tests event-specific buffer sizing
func TestGetOptimalBufferSizeForEvent(t *testing.T) {
	// Create test events
	textEvent := &events.TextMessageContentEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
		MessageID: "test-id",
		Delta:     "short text",
	}

	size := GetOptimalBufferSizeForEvent(textEvent)
	if size < MediumEventBufferSize {
		t.Errorf("Expected at least %d, got %d", MediumEventBufferSize, size)
	}

	// Test with nil event
	size = GetOptimalBufferSizeForEvent(nil)
	if size != DefaultEventBufferSize {
		t.Errorf("Expected %d for nil event, got %d", DefaultEventBufferSize, size)
	}
}

// TestGetOptimalBufferSizeForMultiple tests multiple events buffer sizing
func TestGetOptimalBufferSizeForMultiple(t *testing.T) {
	eventsSlice := []events.Event{
		&events.TextMessageStartEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeTextMessageStart,
			},
			MessageID: "test-id",
			Role:      &[]string{"user"}[0],
		},
		&events.TextMessageContentEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeTextMessageContent,
			},
			MessageID: "test-id",
			Delta:     "content",
		},
	}

	size := GetOptimalBufferSizeForMultiple(eventsSlice)
	expectedMinSize := SmallEventBufferSize + MediumEventBufferSize + 50*2 // overhead
	if size < expectedMinSize {
		t.Errorf("Expected at least %d, got %d", expectedMinSize, size)
	}

	// Test with empty slice
	size = GetOptimalBufferSizeForMultiple([]events.Event{})
	if size != DefaultEventBufferSize {
		t.Errorf("Expected %d for empty slice, got %d", DefaultEventBufferSize, size)
	}
}

// TestMaxFunction tests the max utility function
func TestMaxFunction(t *testing.T) {
	testCases := []struct {
		a, b, expected int
	}{
		{5, 10, 10},
		{10, 5, 10},
		{7, 7, 7},
		{0, 1, 1},
		{-1, 0, 0},
	}

	for _, tc := range testCases {
		result := max(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("max(%d, %d) = %d, expected %d", tc.a, tc.b, result, tc.expected)
		}
	}
}