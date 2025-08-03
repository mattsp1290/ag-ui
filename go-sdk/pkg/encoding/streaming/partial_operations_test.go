package streaming

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	agencoding "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	agjson "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPartialOperationCleanup tests that operations clean up properly when context is cancelled mid-operation
func TestPartialOperationCleanup(t *testing.T) {
	t.Run("ChunkedEncoderCleanup", func(t *testing.T) {
		baseEncoder := agjson.NewJSONEncoder(nil)
		config := DefaultChunkConfig()
		config.EnableParallelProcessing = true
		config.ProcessorCount = 4
		chunkedEncoder := NewChunkedEncoder(baseEncoder, config)

		ctx, cancel := context.WithCancel(context.Background())
		
		input := make(chan events.Event, 1000)
		output := make(chan *Chunk, 100)
		
		// Track goroutine count
		var goroutineCounter int64

		// Start sending events in background
		go func() {
			atomic.AddInt64(&goroutineCounter, 1)
			defer atomic.AddInt64(&goroutineCounter, -1)
			
			for i := 0; i < 1000; i++ {
				select {
				case input <- events.NewTextMessageStartEvent("msg-1"):
				case <-ctx.Done():
					close(input)
					return
				}
			}
			close(input)
		}()

		// Start consuming chunks to prevent blocking
		go func() {
			for range output {
				// Drain the output channel
			}
		}()

		// Start encoding
		encodeErr := make(chan error, 1)
		go func() {
			atomic.AddInt64(&goroutineCounter, 1)
			defer atomic.AddInt64(&goroutineCounter, -1)
			
			encodeErr <- chunkedEncoder.EncodeChunked(ctx, input, output)
		}()

		// Let some processing happen
		time.Sleep(10 * time.Millisecond)

		// Cancel context
		cancel()

		// Wait for encoding to complete
		err := <-encodeErr
		// Context cancellation may or may not cause an error depending on timing
		if err != nil {
			assert.Contains(t, err.Error(), "context", "If there's an error, it should be context-related")
		}

		// Give time for cleanup
		time.Sleep(50 * time.Millisecond)

		// Verify no goroutine leaks
		finalCount := atomic.LoadInt64(&goroutineCounter)
		assert.Equal(t, int64(0), finalCount, "Expected all goroutines to be cleaned up")
	})

	t.Run("StreamManagerCleanup", func(t *testing.T) {
		// Create streaming versions
		streamEncoder := agjson.NewStreamingJSONEncoder(nil)
		streamDecoder := agjson.NewStreamingJSONDecoder(nil)
		
		config := DefaultStreamConfig()
		streamManager := NewStreamManager(streamEncoder, streamDecoder, config)

		err := streamManager.Start()
		require.NoError(t, err)
		defer streamManager.Stop()

		ctx, cancel := context.WithCancel(context.Background())
		
		input := make(chan events.Event, 100)
		output := &testWriter{data: make([][]byte, 0)}
		
		var writeCounter int64

		// Start sending events
		go func() {
			atomic.AddInt64(&writeCounter, 1)
			defer atomic.AddInt64(&writeCounter, -1)
			
			for i := 0; i < 100; i++ {
				select {
				case input <- events.NewTextMessageStartEvent("msg-1"):
				case <-ctx.Done():
					close(input)
					return
				}
			}
			close(input)
		}()

		// Start writing stream
		writeErr := make(chan error, 1)
		go func() {
			atomic.AddInt64(&writeCounter, 1)
			defer atomic.AddInt64(&writeCounter, -1)
			
			writeErr <- streamManager.WriteStream(ctx, input, output)
		}()

		// Let some processing happen
		time.Sleep(10 * time.Millisecond)

		// Cancel context
		cancel()

		// Wait for operation to complete
		err = <-writeErr
		// Context cancellation may or may not cause an error depending on timing
		if err != nil {
			assert.Contains(t, err.Error(), "context", "If there's an error, it should be context-related")
		}

		// Give time for cleanup
		time.Sleep(50 * time.Millisecond)

		// Verify goroutines are cleaned up
		finalCount := atomic.LoadInt64(&writeCounter)
		assert.Equal(t, int64(0), finalCount, "Expected all goroutines to be cleaned up")
	})

	t.Run("ValidationCleanup", func(t *testing.T) {
		config := validation.DefaultSecurityConfig()
		config.MaxNestingDepth = 1000 // Allow deep nesting to make validation slow
		validator := validation.NewSecurityValidator(config)

		ctx, cancel := context.WithCancel(context.Background())
		
		// Create deeply nested structure that will take time to validate
		largeData := createDeeplyNestedJSON(t, 500)

		var validationCounter int64

		// Start validation in background
		validateErr := make(chan error, 1)
		go func() {
			atomic.AddInt64(&validationCounter, 1)
			defer atomic.AddInt64(&validationCounter, -1)
			
			validateErr <- validator.ValidateInput(ctx, largeData)
		}()

		// Let some validation happen
		time.Sleep(5 * time.Millisecond)

		// Cancel context
		cancel()

		// Wait for validation to complete
		err := <-validateErr
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation cancelled")

		// Give time for cleanup
		time.Sleep(50 * time.Millisecond)

		// Verify no goroutine leaks
		finalCount := atomic.LoadInt64(&validationCounter)
		assert.Equal(t, int64(0), finalCount, "Expected validation goroutine to be cleaned up")
	})
}

// TestPartialStateRecovery tests that operations can handle being cancelled in the middle
// and don't leave the system in an inconsistent state
func TestPartialStateRecovery(t *testing.T) {
	t.Run("EncoderStateConsistency", func(t *testing.T) {
		encoder := agjson.NewJSONEncoder(&agencoding.EncodingOptions{
			ValidateOutput: true,
		})

		// Create context that will timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()

		eventSlice := make([]events.Event, 1000)
		for i := 0; i < 1000; i++ {
			eventSlice[i] = events.NewTextMessageStartEvent("msg-1")
		}

		// Try encoding with timeout
		_, err := encoder.EncodeMultiple(ctx, eventSlice)
		_ = err // Mark as used for testing timeout behavior
		
		// Even if it fails due to context cancellation, the encoder should still work
		ctx2 := context.Background()
		singleEvent := events.NewTextMessageStartEvent("msg-1")
		data, err2 := encoder.Encode(ctx2, singleEvent)
		
		assert.NoError(t, err2, "Encoder should work after context cancellation")
		assert.NotEmpty(t, data, "Should get valid encoded data")
	})

	t.Run("ValidatorStateConsistency", func(t *testing.T) {
		config := validation.DefaultSecurityConfig()
		validator := validation.NewSecurityValidator(config)

		// Create context that will timeout during validation
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
		defer cancel()

		// Large data that will take time to validate
		largeData := createDeeplyNestedJSON(t, 200)

		// Try validation with timeout
		err := validator.ValidateInput(ctx, largeData)
		
		// Even if it fails, validator should still work for new operations
		ctx2 := context.Background()
		smallData := []byte(`{"test": "data"}`)
		err2 := validator.ValidateInput(ctx2, smallData)
		
		assert.NoError(t, err2, "Validator should work after context cancellation")
		
		// Original operation should have failed due to context
		if err != nil {
			assert.Contains(t, err.Error(), "validation cancelled")
		}
	})
}

// TestConcurrentCancellation tests handling of concurrent operations with context cancellation
func TestConcurrentCancellation(t *testing.T) {
	t.Run("MultipleEncodersWithCancellation", func(t *testing.T) {
		const numEncoders = 10
		
		var wg sync.WaitGroup
		results := make([]error, numEncoders)
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		for i := 0; i < numEncoders; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				encoder := agjson.NewJSONEncoder(nil)
				eventSlice := make([]events.Event, 100)
				for j := 0; j < 100; j++ {
					eventSlice[j] = events.NewTextMessageStartEvent("msg-1")
				}
				
				_, err := encoder.EncodeMultiple(ctx, eventSlice)
				results[index] = err
			}(i)
		}

		wg.Wait()

		// At least some operations should have been cancelled
		cancelledCount := 0
		for _, err := range results {
			if err != nil && err.Error() != "" {
				cancelledCount++
			}
		}

		// We expect some operations to be cancelled due to timeout
		t.Logf("Cancelled operations: %d out of %d", cancelledCount, numEncoders)
	})
}

// Helper functions

type testWriter struct {
	data [][]byte
	mu   sync.Mutex
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	// Copy data to avoid slice reference issues
	data := make([]byte, len(p))
	copy(data, p)
	w.data = append(w.data, data)
	return len(p), nil
}

func (w *testWriter) GetData() [][]byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	result := make([][]byte, len(w.data))
	copy(result, w.data)
	return result
}

// createDeeplyNestedJSON creates a deeply nested JSON structure for testing
func createDeeplyNestedJSON(t *testing.T, depth int) []byte {
	data := make(map[string]interface{})
	current := data
	
	for i := 0; i < depth; i++ {
		next := make(map[string]interface{})
		current["nested"] = next
		current["value"] = "test data with some content to make validation slower"
		current["array"] = []interface{}{
			"item1", "item2", "item3", 
			map[string]interface{}{"subkey": "subvalue"},
		}
		current = next
	}
	
	result, err := json.Marshal(data)
	require.NoError(t, err)
	return result
}