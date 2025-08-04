package sse

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventStreamResourceCleanup tests basic resource cleanup without complex dependencies
func TestEventStreamResourceCleanup(t *testing.T) {
	// Record initial goroutine count
	runtime.GC()
	time.Sleep(10 * time.Millisecond) // Allow GC to complete
	initialGoroutines := runtime.NumGoroutine()

	// Create a simple event stream configuration
	config := &StreamConfig{
		EventBufferSize:     50,
		ChunkBufferSize:     25,
		MaxChunkSize:        512,
		FlushInterval:       20 * time.Millisecond,
		BatchEnabled:        false, // Disable batching for simplicity
		BatchSize:           10,    // Must be > 0 to avoid divide by zero
		MaxConcurrentEvents: 10,
		BackpressureTimeout: 500 * time.Millisecond,
		DrainTimeout:       200 * time.Millisecond,
		SequenceEnabled:    false, // Disable sequencing for simplicity
		WorkerCount:        2,
		EnableMetrics:      true,
		MetricsInterval:    50 * time.Millisecond,
	}
	
	stream, err := NewEventStream(config)
	require.NoError(t, err)
	
	err = stream.Start()
	require.NoError(t, err)
	
	// Let stream run and create goroutines
	time.Sleep(100 * time.Millisecond)
	
	// Check that goroutines increased (indicating workers are running)
	runtime.GC()
	runningGoroutines := runtime.NumGoroutine()
	assert.Greater(t, runningGoroutines, initialGoroutines, 
		"Should have more goroutines when stream is running (initial: %d, running: %d)", 
		initialGoroutines, runningGoroutines)
	
	// Close stream
	err = stream.Close()
	assert.NoError(t, err)
	
	// Wait for cleanup
	time.Sleep(150 * time.Millisecond)
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	
	finalGoroutines := runtime.NumGoroutine()
	
	// Verify goroutines are cleaned up (allow small buffer for test goroutines)
	assert.LessOrEqual(t, finalGoroutines, initialGoroutines+2, 
		"Goroutines should be cleaned up after closing stream (initial: %d, final: %d)", 
		initialGoroutines, finalGoroutines)
}

// TestEventStreamLifecycle tests start/stop lifecycle
func TestEventStreamLifecycle(t *testing.T) {
	config := &StreamConfig{
		EventBufferSize:     20,
		ChunkBufferSize:     10,
		MaxChunkSize:        128,
		FlushInterval:       10 * time.Millisecond,
		BatchEnabled:        false,
		BatchSize:           5, // Must be > 0
		MaxConcurrentEvents: 5,
		BackpressureTimeout: 200 * time.Millisecond,
		DrainTimeout:       100 * time.Millisecond,
		SequenceEnabled:    false,
		WorkerCount:        1,
		EnableMetrics:      false, // Disable metrics for simplicity
	}
	
	stream, err := NewEventStream(config)
	require.NoError(t, err)
	
	// Test that stream is not started initially
	assert.False(t, stream.isStarted(), "Stream should not be started initially")
	assert.False(t, stream.isClosed(), "Stream should not be closed initially")
	
	// Start stream
	err = stream.Start()
	require.NoError(t, err)
	assert.True(t, stream.isStarted(), "Stream should be started after Start()")
	
	// Test that starting again fails
	err = stream.Start()
	assert.Error(t, err, "Starting already started stream should fail")
	
	// Close stream
	err = stream.Close()
	assert.NoError(t, err)
	assert.True(t, stream.isClosed(), "Stream should be closed after Close()")
	
	// Test that closing again succeeds (idempotent)
	err = stream.Close()
	assert.NoError(t, err, "Closing already closed stream should be idempotent")
}

// TestEventStreamConfigValidation tests configuration validation
func TestEventStreamConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		configFunc  func() *StreamConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "ValidConfig",
			configFunc: func() *StreamConfig {
				return &StreamConfig{
					EventBufferSize:     10,
					ChunkBufferSize:     5,
					MaxChunkSize:        64,
					FlushInterval:       10 * time.Millisecond,
					BatchEnabled:        false,
					BatchSize:           2, // Must be > 0
					MaxConcurrentEvents: 2,
					BackpressureTimeout: 100 * time.Millisecond,
					DrainTimeout:       50 * time.Millisecond,
					SequenceEnabled:    false,
					WorkerCount:        1,
					EnableMetrics:      false,
				}
			},
			expectError: false,
		},
		{
			name: "ZeroEventBufferSize",
			configFunc: func() *StreamConfig {
				return &StreamConfig{
					EventBufferSize: 0, // Invalid
					ChunkBufferSize: 5,
					MaxChunkSize:    64,
					BatchSize:       1, // Must be > 0
					WorkerCount:     1,
				}
			},
			expectError: true,
			errorMsg:    "event buffer size must be positive",
		},
		{
			name: "NegativeChunkBufferSize",
			configFunc: func() *StreamConfig {
				return &StreamConfig{
					EventBufferSize: 10,
					ChunkBufferSize: -1, // Invalid
					MaxChunkSize:    64,
					BatchSize:       1, // Must be > 0
					WorkerCount:     1,
				}
			},
			expectError: true,
			errorMsg:    "chunk buffer size must be positive",
		},
		{
			name: "ZeroMaxConcurrentEvents",
			configFunc: func() *StreamConfig {
				return &StreamConfig{
					EventBufferSize:     10,
					ChunkBufferSize:     5,
					MaxChunkSize:        64,
					BatchSize:           1, // Must be > 0
					MaxConcurrentEvents: 0, // Invalid
					WorkerCount:         1, // Valid
				}
			},
			expectError: true,
			errorMsg:    "max concurrent events must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.configFunc()
			
			stream, err := NewEventStream(config)
			
			if tt.expectError {
				assert.Error(t, err, "Expected validation error")
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg, "Error should contain expected message")
				}
				assert.Nil(t, stream, "Stream should be nil when validation fails")
			} else {
				assert.NoError(t, err, "Should not have validation error")
				assert.NotNil(t, stream, "Stream should be created successfully")
				if stream != nil {
					stream.Close() // Clean up
				}
			}
		})
	}
}

// TestEventStreamConcurrentAccess tests concurrent access safety
func TestEventStreamConcurrentAccess(t *testing.T) {
	config := &StreamConfig{
		EventBufferSize:     100,
		ChunkBufferSize:     50,
		MaxChunkSize:        256,
		FlushInterval:       20 * time.Millisecond,
		BatchEnabled:        false,
		BatchSize:           10, // Must be > 0
		MaxConcurrentEvents: 20,
		BackpressureTimeout: 300 * time.Millisecond,
		DrainTimeout:       150 * time.Millisecond,
		SequenceEnabled:    false,
		WorkerCount:        3,
		EnableMetrics:      true,
		MetricsInterval:    40 * time.Millisecond,
	}
	
	stream, err := NewEventStream(config)
	require.NoError(t, err)
	
	err = stream.Start()
	require.NoError(t, err)
	
	// Test concurrent access to metrics
	var wg sync.WaitGroup
	numGoroutines := 5
	operationsPerGoroutine := 20
	
	// Concurrent metric access
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			
			for i := 0; i < operationsPerGoroutine; i++ {
				metrics := stream.GetMetrics()
				if metrics != nil {
					// Access various metric fields to test thread safety
					_ = metrics.TotalEvents
					_ = metrics.EventsProcessed
					_ = metrics.EventsDropped
					_ = metrics.StartTime
				}
				time.Sleep(1 * time.Millisecond)
			}
		}(g)
	}
	
	// Concurrent chunk channel access
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			
			chunkChan := stream.ReceiveChunks()
			for i := 0; i < operationsPerGoroutine/4; i++ {
				select {
				case <-chunkChan:
					// Got a chunk, which is fine
				case <-time.After(10 * time.Millisecond):
					// Timeout is also fine
				}
			}
		}(g)
	}
	
	// Concurrent error channel access
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			
			errorChan := stream.GetErrorChannel()
			for i := 0; i < operationsPerGoroutine/4; i++ {
				select {
				case <-errorChan:
					// Got an error, which is fine
				case <-time.After(10 * time.Millisecond):
					// Timeout is also fine
				}
			}
		}(g)
	}
	
	wg.Wait()
	
	// Verify stream is still functional
	metrics := stream.GetMetrics()
	assert.NotNil(t, metrics, "Should still be able to get metrics after concurrent access")
	
	// Clean up
	err = stream.Close()
	assert.NoError(t, err)
}

// TestSimpleMemoryLeakPrevention tests that repeated create/destroy cycles don't leak memory
func TestSimpleMemoryLeakPrevention(t *testing.T) {
	runtime.GC()
	
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	
	// Run multiple cycles of create/use/destroy
	cycles := 3 // Reduced for faster testing
	for cycle := 0; cycle < cycles; cycle++ {
		config := &StreamConfig{
			EventBufferSize:     20,
			ChunkBufferSize:     10,
			MaxChunkSize:        128,
			FlushInterval:       10 * time.Millisecond,
			BatchEnabled:        false,
			BatchSize:           5, // Must be > 0
			MaxConcurrentEvents: 3,
			BackpressureTimeout: 200 * time.Millisecond,
			DrainTimeout:       100 * time.Millisecond,
			SequenceEnabled:    false,
			WorkerCount:        1,
			EnableMetrics:      true,
			MetricsInterval:    30 * time.Millisecond,
		}
		
		stream, err := NewEventStream(config)
		require.NoError(t, err)
		
		err = stream.Start()
		require.NoError(t, err)
		
		// Let it process briefly
		time.Sleep(30 * time.Millisecond)
		
		// Get some metrics (to ensure all code paths are used)
		metrics := stream.GetMetrics()
		if metrics != nil {
			t.Logf("Cycle %d metrics: StartTime=%v", cycle, metrics.StartTime)
		}
		
		// Close stream
		err = stream.Close()
		require.NoError(t, err)
		
		// Force cleanup
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	
	// Final memory check
	runtime.GC()
	runtime.ReadMemStats(&m2)
	
	// Memory usage should not have grown significantly
	memGrowth := int64(m2.Alloc) - int64(m1.Alloc)
	t.Logf("Memory growth: %d bytes (%.2f MB)", memGrowth, float64(memGrowth)/(1024*1024))
	
	// Allow some growth for legitimate allocations, but not excessive
	maxAllowedGrowthMB := float64(2) // 2MB seems reasonable for test overhead
	actualGrowthMB := float64(memGrowth) / (1024 * 1024)
	
	assert.Less(t, actualGrowthMB, maxAllowedGrowthMB, 
		"Memory growth should be minimal after multiple cycles (%.2f MB > %.2f MB)", 
		actualGrowthMB, maxAllowedGrowthMB)
}

// TestEventStreamShutdownTimeout tests shutdown behavior with timeout
func TestEventStreamShutdownTimeout(t *testing.T) {
	config := &StreamConfig{
		EventBufferSize:     30,
		ChunkBufferSize:     15,
		MaxChunkSize:        64,
		FlushInterval:       5 * time.Millisecond,
		BatchEnabled:        false,
		BatchSize:           3, // Must be > 0
		MaxConcurrentEvents: 5,
		BackpressureTimeout: 100 * time.Millisecond,
		DrainTimeout:       50 * time.Millisecond, // Short timeout for testing
		SequenceEnabled:    false,
		WorkerCount:        2,
		EnableMetrics:      false,
	}
	
	stream, err := NewEventStream(config)
	require.NoError(t, err)
	
	err = stream.Start()
	require.NoError(t, err)
	
	// Let it run briefly
	time.Sleep(30 * time.Millisecond)
	
	// Measure shutdown time
	shutdownStart := time.Now()
	err = stream.Close()
	shutdownDuration := time.Since(shutdownStart)
	
	assert.NoError(t, err, "Shutdown should complete without error")
	
	// Shutdown should complete within reasonable time (allowing for drain timeout + buffer)
	maxExpectedShutdown := config.DrainTimeout + 100*time.Millisecond
	assert.Less(t, shutdownDuration, maxExpectedShutdown, 
		"Shutdown should complete within expected time (took %v, max %v)", 
		shutdownDuration, maxExpectedShutdown)
	
	// Verify stream is properly closed
	assert.True(t, stream.isClosed(), "Stream should be marked as closed")
}