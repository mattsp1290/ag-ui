package encoding_test

import (
	"context"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
	"github.com/stretchr/testify/assert"
)

// TestSimpleContextCancellation tests basic context cancellation functionality
func TestSimpleContextCancellation(t *testing.T) {
	t.Run("JSONEncoderContextCancellation", func(t *testing.T) {
		encoder := json.NewJSONEncoder(nil)

		// Test immediate cancellation
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		event := events.NewTextMessageStartEvent("msg-1", events.WithRole("test"))
		_, err := encoder.Encode(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cancelled")
	})

	t.Run("JSONEncoderTimeoutCancellation", func(t *testing.T) {
		encoder := json.NewJSONEncoder(nil)

		// Test timeout cancellation
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Create many events to increase processing time
		eventList := make([]events.Event, 1000)
		for i := 0; i < 1000; i++ {
			eventList[i] = events.NewTextMessageStartEvent("msg-1", events.WithRole("test"))
		}

		time.Sleep(2 * time.Millisecond) // Ensure context times out

		_, err := encoder.EncodeMultiple(ctx, eventList)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cancelled")
	})
}
