package sse

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// TestEventStreamCleanupTimeout tests that EventStream.Close() doesn't hang indefinitely
func TestEventStreamCleanupTimeout(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 4
	config.DrainTimeout = 1 * time.Second // Short timeout for test
	config.EnableMetrics = true
	config.BatchEnabled = true

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	// Start the stream
	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}

	// Send some events to create work
	testEvent := events.NewRunStartedEvent("test-thread", "test-run")
	for i := 0; i < 10; i++ {
		_ = stream.SendEvent(testEvent) // Ignore errors for test
	}

	// Let workers process for a bit
	time.Sleep(50 * time.Millisecond)

	// Close should complete within reasonable time
	start := time.Now()
	err = stream.Close()
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Close should complete quickly (within drain timeout + some buffer)
	maxExpectedDuration := config.DrainTimeout + 500*time.Millisecond
	if duration > maxExpectedDuration {
		t.Errorf("Close() took too long: %v (expected < %v)", duration, maxExpectedDuration)
	}

	t.Logf("Close() completed in %v", duration)
}

// TestFlowControllerReleaseUnderShutdown tests FlowController.Release() behavior during shutdown
func TestFlowControllerReleaseUnderShutdown(t *testing.T) {
	fc := NewFlowController(2, 100*time.Millisecond, 1*time.Second)
	ctx := context.Background()

	// Acquire all slots
	err := fc.Acquire(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire first slot: %v", err)
	}

	err = fc.Acquire(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire second slot: %v", err)
	}

	// Drain the channel manually to simulate shutdown scenario
	fc.Drain()

	// Release should not hang even when channel is empty
	start := time.Now()
	fc.Release()
	fc.Release()
	duration := time.Since(start)

	if duration > 100*time.Millisecond {
		t.Errorf("Release() took too long under shutdown: %v", duration)
	}

	t.Logf("Release() under shutdown completed in %v", duration)
}

// TestEventProcessorShutdownDrain tests that event processors drain properly during shutdown
func TestEventProcessorShutdownDrain(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 2
	config.DrainTimeout = 500 * time.Millisecond
	config.EnableMetrics = false
	config.BatchEnabled = false

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}

	// Fill the event channel
	testEvent := events.NewRunStartedEvent("test-thread", "test-run")
	for i := 0; i < 50; i++ {
		select {
		case stream.eventChan <- testEvent:
		default:
			break // Channel full
		}
	}

	// Close should drain events and complete
	start := time.Now()
	err = stream.Close()
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Should complete within drain timeout + buffer
	if duration > config.DrainTimeout+200*time.Millisecond {
		t.Errorf("Close() took too long draining events: %v", duration)
	}

	t.Logf("Event drain completed in %v", duration)
}

// TestConcurrentCloseOperations tests that multiple Close() calls are safe
func TestConcurrentCloseOperations(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 2
	config.DrainTimeout = 1 * time.Second

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}

	// Call Close() concurrently from multiple goroutines
	const numClosers = 5
	errChan := make(chan error, numClosers)

	for i := 0; i < numClosers; i++ {
		go func() {
			errChan <- stream.Close()
		}()
	}

	// Collect results
	for i := 0; i < numClosers; i++ {
		err := <-errChan
		if err != nil {
			t.Errorf("Concurrent Close() call %d returned error: %v", i, err)
		}
	}

	t.Log("Concurrent Close() operations completed successfully")
}

// TestGoroutineLeakDetection tests for goroutine leaks during EventStream lifecycle
func TestGoroutineLeakDetection(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutines: %d", initialGoroutines)

	config := DefaultStreamConfig()
	config.WorkerCount = 4
	config.DrainTimeout = 1 * time.Second
	config.EnableMetrics = true

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}

	// Do some work
	testEvent := events.NewRunStartedEvent("test-thread", "test-run")
	for i := 0; i < 20; i++ {
		_ = stream.SendEvent(testEvent)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Close and verify cleanup
	err = stream.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Allow cleanup to complete
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	t.Logf("Final goroutines: %d (diff: %d)", finalGoroutines, finalGoroutines-initialGoroutines)

	// Allow for some variance but detect significant leaks
	if finalGoroutines > initialGoroutines+3 {
		t.Errorf("Potential goroutine leak: initial=%d, final=%d", 
			initialGoroutines, finalGoroutines)
	}
}

// TestBatchProcessorShutdown tests batch processor shutdown behavior
func TestBatchProcessorShutdown(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.BatchEnabled = true
	config.BatchSize = 10
	config.BatchTimeout = 100 * time.Millisecond
	config.DrainTimeout = 500 * time.Millisecond

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}

	// Add events to create partial batches
	testEvent := events.NewRunStartedEvent("test-thread", "test-run")
	for i := 0; i < 5; i++ { // Less than batch size
		_ = stream.SendEvent(testEvent)
	}

	// Close should handle partial batches gracefully
	start := time.Now()
	err = stream.Close()
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	if duration > config.DrainTimeout+200*time.Millisecond {
		t.Errorf("Batch processor shutdown took too long: %v", duration)
	}

	t.Logf("Batch processor shutdown completed in %v", duration)
}