package streaming

import (
	"context"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testStreamEncoder implements StreamEncoder for goroutine leak testing
type testStreamEncoder struct{}

func (m *testStreamEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return nil, nil
}

func (m *testStreamEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return nil, nil
}

func (m *testStreamEncoder) ContentType() string {
	return "test"
}

func (m *testStreamEncoder) CanStream() bool {
	return true
}

func (m *testStreamEncoder) SupportsStreaming() bool {
	return true
}

func (m *testStreamEncoder) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return nil
}

func (m *testStreamEncoder) StartStream(ctx context.Context, writer io.Writer) error {
	return nil
}

func (m *testStreamEncoder) WriteEvent(ctx context.Context, event events.Event) error {
	return nil
}

func (m *testStreamEncoder) EndStream(ctx context.Context) error {
	return nil
}

// testStreamDecoder implements StreamDecoder for goroutine leak testing
type testStreamDecoder struct{}

func (m *testStreamDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return nil, nil
}

func (m *testStreamDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return nil, nil
}

func (m *testStreamDecoder) ContentType() string {
	return "test"
}

func (m *testStreamDecoder) CanStream() bool {
	return true
}

func (m *testStreamDecoder) SupportsStreaming() bool {
	return true
}

func (m *testStreamDecoder) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return nil
}

func (m *testStreamDecoder) StartStream(ctx context.Context, reader io.Reader) error {
	return nil
}

func (m *testStreamDecoder) ReadEvent(ctx context.Context) (events.Event, error) {
	return nil, nil
}

func (m *testStreamDecoder) EndStream(ctx context.Context) error {
	return nil
}

func TestStreamManagerNoGoroutineLeak(t *testing.T) {
	// Get initial goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	initialCount := runtime.NumGoroutine()

	// Create and start multiple stream managers
	for i := 0; i < 5; i++ {
		config := DefaultStreamConfig()
		config.EnableMetrics = true // Ensure metrics are enabled to test the leak

		encoder := &testStreamEncoder{}
		decoder := &testStreamDecoder{}
		
		sm := NewStreamManager(encoder, decoder, config)
		
		// Start the stream manager
		err := sm.Start()
		require.NoError(t, err)
		
		// Let it run for a bit
		time.Sleep(50 * time.Millisecond)
		
		// Stop the stream manager
		err = sm.Stop()
		require.NoError(t, err)
	}

	// Wait for goroutines to clean up
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check final goroutine count
	finalCount := runtime.NumGoroutine()
	
	// Allow for some variance in goroutine count (test framework, etc.)
	// but it should not grow significantly
	assert.LessOrEqual(t, finalCount, initialCount+2, 
		"Goroutine leak detected: initial=%d, final=%d", initialCount, finalCount)
}

func TestStreamMetricsNoGoroutineLeak(t *testing.T) {
	// Get initial goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	initialCount := runtime.NumGoroutine()

	// Create and close multiple metrics instances
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		
		// Create metrics with context
		metrics := NewStreamMetricsWithContext(ctx)
		
		// Simulate some work
		for j := 0; j < 100; j++ {
			ts := time.Now().UnixMilli()
			event := &mockEvent{
				EventType:   events.EventTypeTextMessageStart,
				TimestampMs: &ts,
			}
			metrics.RecordEvent(event)
		}
		
		// Cancel context and close metrics
		cancel()
		metrics.Close()
	}

	// Wait for goroutines to clean up
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check final goroutine count
	finalCount := runtime.NumGoroutine()
	
	// Should be approximately the same as initial count
	assert.LessOrEqual(t, finalCount, initialCount+2,
		"Goroutine leak in metrics: initial=%d, final=%d", initialCount, finalCount)
}

func TestStreamManagerContextCancellation(t *testing.T) {
	config := DefaultStreamConfig()
	config.EnableMetrics = true

	encoder := &mockStreamEncoder{}
	decoder := &mockStreamDecoder{}
	
	sm := NewStreamManager(encoder, decoder, config)
	
	// Start the stream manager
	err := sm.Start()
	require.NoError(t, err)
	
	// Get initial goroutine count after start
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runningCount := runtime.NumGoroutine()
	
	// Stop should cancel all contexts
	err = sm.Stop()
	require.NoError(t, err)
	
	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	// Verify goroutines are cleaned up
	finalCount := runtime.NumGoroutine()
	assert.Less(t, finalCount, runningCount,
		"Goroutines not cleaned up after Stop: running=%d, final=%d", runningCount, finalCount)
}

func TestMetricsParentContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Get initial count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	initialCount := runtime.NumGoroutine()
	
	// Create metrics with parent context
	metrics := NewStreamMetricsWithContext(ctx)
	
	// Let it run
	time.Sleep(100 * time.Millisecond)
	
	// Cancel parent context (should stop metrics goroutine)
	cancel()
	
	// Don't call Close() - the context cancellation should be enough
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	// Check that goroutine was cleaned up
	finalCount := runtime.NumGoroutine()
	assert.LessOrEqual(t, finalCount, initialCount+1,
		"Metrics goroutine not cleaned up by context cancellation: initial=%d, final=%d", 
		initialCount, finalCount)
	
	// Calling Close() should be safe even after context cancellation
	metrics.Close()
}