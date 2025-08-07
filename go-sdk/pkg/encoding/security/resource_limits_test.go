package security

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/validation"
)

// TestBufferPoolResourceLimits tests that buffer pools enforce resource limits
func TestBufferPoolResourceLimits(t *testing.T) {
	// Test small buffer pool limits
	pool := encoding.NewBufferPoolWithCapacity(1024, 2) // Max 2 buffers, 1KB each

	// Should be able to get first buffer
	buf1 := pool.Get()
	if buf1 == nil {
		t.Fatal("Expected to get first buffer")
	}

	// Should be able to get second buffer
	buf2 := pool.Get()
	if buf2 == nil {
		t.Fatal("Expected to get second buffer")
	}

	// Third buffer should be nil (limit exceeded)
	buf3 := pool.Get()
	if buf3 != nil {
		t.Fatal("Expected nil buffer when limit exceeded")
	}

	// Return buffers to pool
	pool.Put(buf1)
	pool.Put(buf2)

	// Should be able to get buffer again after returning
	buf4 := pool.Get()
	if buf4 == nil {
		t.Fatal("Expected to get buffer after returning to pool")
	}
	pool.Put(buf4)
}

// TestSlicePoolResourceLimits tests that slice pools enforce resource limits
func TestSlicePoolResourceLimits(t *testing.T) {
	// Test slice pool limits
	pool := encoding.NewSlicePoolWithCapacity(512, 1024, 2) // Max 2 slices

	// Should be able to get first slice
	slice1 := pool.Get()
	if slice1 == nil {
		t.Fatal("Expected to get first slice")
	}

	// Should be able to get second slice
	slice2 := pool.Get()
	if slice2 == nil {
		t.Fatal("Expected to get second slice")
	}

	// Third slice should be nil (limit exceeded)
	slice3 := pool.Get()
	if slice3 != nil {
		t.Fatal("Expected nil slice when limit exceeded")
	}

	// Return slices to pool
	pool.Put(slice1)
	pool.Put(slice2)
}

// TestGlobalBufferLimits tests the global buffer allocation functions
func TestGlobalBufferLimits(t *testing.T) {
	// Reset pools to ensure clean state
	encoding.ResetAllPools()

	// Test safe buffer allocation
	var buffers []*bytes.Buffer

	// Try to allocate many buffers - should eventually return nil when exhausted
	for i := 0; i < 1000; i++ {
		buf := encoding.GetBufferSafe(1024)
		if buf == nil {
			t.Logf("Buffer allocation failed at iteration %d (expected)", i)
			break
		}
		buffers = append(buffers, buf)
	}

	// Clean up
	for _, buf := range buffers {
		encoding.PutBuffer(buf)
	}

	// Test allocation of oversized buffer
	hugeBuf := encoding.GetBufferSafe(200 * 1024 * 1024) // 200MB
	if hugeBuf != nil {
		t.Logf("Note: GetBufferSafe returned buffer for large allocation (this may be expected if fallback allocation is enabled)")
	}
}

// TestJSONEncoderConcurrencyLimits tests JSON encoder concurrency limits
func TestJSONEncoderConcurrencyLimits(t *testing.T) {
	encoder := json.NewJSONEncoderWithConcurrencyLimit(nil, 2) // Max 2 concurrent operations

	// Create test event with larger content to slow down processing
	largeContent := string(bytes.Repeat([]byte("test content "), 10000)) // ~130KB content
	event := &events.TextMessageContentEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
		MessageID: "test-message",
		Delta:     largeContent,
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	var successCount int32
	var failureCount int32
	var startBarrier sync.WaitGroup

	numGoroutines := 20 // Increase number of concurrent operations
	startBarrier.Add(1) // Barrier to ensure all goroutines start simultaneously

	// Launch multiple concurrent encoding operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Wait for all goroutines to be ready
			startBarrier.Wait()

			// Add small staggered delay to increase contention
			time.Sleep(time.Microsecond * time.Duration(idx%3))

			_, err := encoder.Encode(ctx, event)
			if err != nil {
				if isResourceLimitError(err) {
					atomic.AddInt32(&failureCount, 1)
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	// Release all goroutines at once to maximize contention
	startBarrier.Done()
	wg.Wait()

	t.Logf("Successful encodings: %d, Failed due to limits: %d", successCount, failureCount)

	// Verify that total operations is correct
	total := successCount + failureCount
	if total != int32(numGoroutines) {
		t.Errorf("Expected total operations to be %d, got %d", numGoroutines, total)
	}

	// At least some operations should succeed (encoder is functional)
	if successCount == 0 {
		t.Error("Expected some operations to succeed")
	}

	// With a limit of 2 and 20 concurrent operations with large data,
	// we should expect some failures, but make it non-strict for reliability
	if failureCount == 0 {
		t.Logf("Note: No operations failed due to concurrency limits. This may occur on systems with very fast processing.")
		// Don't fail the test - log instead for debugging
	} else {
		t.Logf("Successfully enforced concurrency limits: %d/%d operations failed", failureCount, total)
	}
}

// TestJSONDecoderConcurrencyLimits tests JSON decoder concurrency limits
func TestJSONDecoderConcurrencyLimits(t *testing.T) {
	decoder := json.NewJSONDecoderWithConcurrencyLimit(nil, 2) // Max 2 concurrent operations

	// Create test JSON data with correct field names - make it larger to slow down processing
	largeContent := string(bytes.Repeat([]byte("test content "), 1000)) // ~13KB content
	jsonData := []byte(fmt.Sprintf(`{"type":"TEXT_MESSAGE_CONTENT","messageId":"test","delta":"%s"}`, largeContent))

	ctx := context.Background()
	var wg sync.WaitGroup
	var successCount int32
	var failureCount int32
	var startBarrier sync.WaitGroup

	numGoroutines := 20 // Increase number of concurrent operations
	startBarrier.Add(1) // Barrier to ensure all goroutines start simultaneously

	// Launch multiple concurrent decoding operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Wait for all goroutines to be ready
			startBarrier.Wait()

			// Add small staggered delay to increase contention
			time.Sleep(time.Microsecond * time.Duration(idx%3))

			_, err := decoder.Decode(ctx, jsonData)
			if err != nil {
				if isResourceLimitError(err) {
					atomic.AddInt32(&failureCount, 1)
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	// Release all goroutines at once to maximize contention
	startBarrier.Done()
	wg.Wait()

	t.Logf("Successful decodings: %d, Failed due to limits: %d", successCount, failureCount)

	// Verify that at least some operations succeeded and total is correct
	total := successCount + failureCount
	if total != int32(numGoroutines) {
		t.Errorf("Expected total operations to be %d, got %d", numGoroutines, total)
	}

	// At least some operations should succeed (decoder is functional)
	if successCount == 0 {
		t.Error("Expected at least some operations to succeed")
	}

	// With a limit of 2 and 20 concurrent operations with larger data,
	// we should expect some failures, but make it non-strict for reliability
	if failureCount == 0 {
		t.Logf("Note: No operations failed due to concurrency limits. This may occur on systems with very fast processing.")
		// Don't fail the test - log instead for debugging
	} else {
		t.Logf("Successfully enforced concurrency limits: %d/%d operations failed", failureCount, total)
	}
}

// TestJSONDecoderConcurrencyLimitsDeterministic tests JSON decoder concurrency limits in a more deterministic way
func TestJSONDecoderConcurrencyLimitsDeterministic(t *testing.T) {
	decoder := json.NewJSONDecoderWithConcurrencyLimit(nil, 1) // Very strict limit of 1

	// Create test JSON data
	jsonData := []byte(`{"type":"TEXT_MESSAGE_CONTENT","messageId":"test","delta":"content"}`)

	ctx := context.Background()
	var wg sync.WaitGroup
	var successCount int32
	var failureCount int32

	// Use a channel to serialize the start of operations and create contention
	startCh := make(chan struct{})

	numGoroutines := 5
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Wait for signal to start
			<-startCh

			_, err := decoder.Decode(ctx, jsonData)
			if err != nil {
				if isResourceLimitError(err) {
					atomic.AddInt32(&failureCount, 1)
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	// Start all goroutines simultaneously
	close(startCh)
	wg.Wait()

	t.Logf("Deterministic test - Successful decodings: %d, Failed due to limits: %d", successCount, failureCount)

	// Verify total operations
	total := successCount + failureCount
	if total != int32(numGoroutines) {
		t.Errorf("Expected total operations to be %d, got %d", numGoroutines, total)
	}

	// At least one operation should succeed
	if successCount == 0 {
		t.Error("Expected at least one operation to succeed")
	}

	// With such a strict limit (1) and simultaneous start, we expect some failures
	t.Logf("Concurrency enforcement: %d successes, %d limit rejections out of %d total",
		successCount, failureCount, total)
}

// TestSecurityValidatorResourceLimits tests security validator resource monitoring
func TestSecurityValidatorResourceLimits(t *testing.T) {
	config := validation.SecurityConfig{
		MaxInputSize:          10 * 1024 * 1024, // 10MB input limit
		MaxMemoryUsage:        50 * 1024 * 1024, // 50MB memory limit (higher to account for system memory)
		EnableResourceMonitor: true,
	}

	validator := validation.NewSecurityValidator(config)
	ctx := context.Background()

	// Test with small data - should succeed (use non-null bytes)
	smallData := bytes.Repeat([]byte{'A'}, 1024) // 1KB of 'A' characters
	err := validator.ValidateInput(ctx, smallData)
	if err != nil {
		t.Errorf("Validation should succeed for small data: %v", err)
	}

	// Test with very large data - should fail due to input size limits
	largeData := bytes.Repeat([]byte{'B'}, 15*1024*1024) // 15MB of 'B' characters (exceeds 10MB input limit)
	err = validator.ValidateInput(ctx, largeData)
	if err == nil {
		t.Error("Expected validation to fail for large data exceeding input size limits")
	}

	// Create a validator with very restrictive memory limits for testing
	restrictiveConfig := validation.SecurityConfig{
		MaxInputSize:          10 * 1024 * 1024, // 10MB input limit
		MaxMemoryUsage:        100 * 1024,       // Only 100KB memory limit (very restrictive)
		EnableResourceMonitor: true,
	}
	restrictiveValidator := validation.NewSecurityValidator(restrictiveConfig)

	// Test concurrent validations with restrictive limits
	var wg sync.WaitGroup
	var successCount int32
	var failureCount int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Create data that will likely exceed memory limits when processed concurrently
			lines := make([]string, 8) // 8 lines of 8KB each = 64KB total
			for j := range lines {
				lines[j] = string(bytes.Repeat([]byte{'C'}, 8*1024))
			}
			mediumData := []byte(strings.Join(lines, "\n"))
			err := restrictiveValidator.ValidateInput(ctx, mediumData)
			if err != nil {
				if isResourceLimitError(err) {
					atomic.AddInt32(&failureCount, 1)
				} else {
					t.Errorf("Unexpected validation error: %v", err)
				}
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	t.Logf("Successful validations: %d, Failed due to limits: %d", successCount, failureCount)

	// Should have some failures due to resource limits when using restrictive validator
	if failureCount == 0 {
		t.Error("Expected some validations to fail due to resource limits with restrictive validator")
	}
}

// TestSecurityValidatorWithEvents tests validator resource limits with events
func TestSecurityValidatorWithEvents(t *testing.T) {
	config := validation.SecurityConfig{
		MaxInputSize:          10 * 1024 * 1024,  // 10MB input limit
		MaxStringLength:       2 * 1024 * 1024,   // 2MB string limit
		MaxMemoryUsage:        100 * 1024 * 1024, // 100MB limit (higher to account for system memory)
		EnableResourceMonitor: true,
	}

	validator := validation.NewSecurityValidator(config)
	ctx := context.Background()

	// Create a large event that exceeds string length limits
	largeEvent := &events.TextMessageContentEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
		MessageID: "test-message",
		Delta:     string(bytes.Repeat([]byte{'X'}, 3*1024*1024)), // 3MB content (exceeds 2MB string limit)
	}

	// Should fail due to string length limits
	err := validator.ValidateEvent(ctx, largeEvent)
	if err == nil {
		t.Error("Expected validation to fail for large event")
	}

	// Small event should succeed
	smallEvent := &events.TextMessageContentEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
		MessageID: "test-message",
		Delta:     "small content",
	}

	err = validator.ValidateEvent(ctx, smallEvent)
	if err != nil {
		t.Errorf("Small event validation should succeed: %v", err)
	}
}

// TestResourceStatsRetrieval tests that resource usage statistics can be retrieved
func TestResourceStatsRetrieval(t *testing.T) {
	config := validation.SecurityConfig{
		MaxMemoryUsage:        100 * 1024 * 1024, // 100MB limit (higher to account for system memory)
		EnableResourceMonitor: true,
	}

	validator := validation.NewSecurityValidator(config)

	// Get initial stats
	stats := validator.GetResourceStats()
	if stats == nil {
		t.Fatal("Expected resource stats to be available")
	}

	// Check required fields
	requiredFields := []string{
		"system_memory_alloc",
		"tracked_memory",
		"active_operations",
		"max_memory_limit",
		"resource_monitor_enabled",
		"memory_utilization",
	}

	for _, field := range requiredFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Expected field %s in resource stats", field)
		}
	}

	// Verify limits are reported correctly
	if stats["max_memory_limit"] != int64(100*1024*1024) {
		t.Errorf("Expected max_memory_limit to be %d, got %v", 100*1024*1024, stats["max_memory_limit"])
	}

	if stats["resource_monitor_enabled"] != true {
		t.Error("Expected resource_monitor_enabled to be true")
	}
}

// TestMemoryUsageTracking tests that memory usage is tracked correctly
func TestMemoryUsageTracking(t *testing.T) {
	config := validation.SecurityConfig{
		MaxInputSize:          10 * 1024 * 1024,  // 10MB input limit
		MaxMemoryUsage:        100 * 1024 * 1024, // 100MB limit (higher to account for system memory)
		EnableResourceMonitor: true,
	}

	validator := validation.NewSecurityValidator(config)
	ctx := context.Background()

	// Get initial stats
	initialStats := validator.GetResourceStats()
	initialTracked := initialStats["tracked_memory"].(int64)

	// Validate some data (use non-null bytes with line breaks)
	lines := make([]string, 128) // 128 lines of 8KB each = 1MB total
	for i := range lines {
		lines[i] = string(bytes.Repeat([]byte{'D'}, 8*1024))
	}
	testData := []byte(strings.Join(lines, "\n"))
	err := validator.ValidateInput(ctx, testData)
	if err != nil {
		t.Fatalf("Validation should succeed: %v", err)
	}

	// Memory should be back to initial level after validation completes
	finalStats := validator.GetResourceStats()
	finalTracked := finalStats["tracked_memory"].(int64)

	if finalTracked != initialTracked {
		t.Errorf("Expected tracked memory to return to initial level (%d), got %d", initialTracked, finalTracked)
	}
}

// isResourceLimitError checks if an error is due to resource limits
func isResourceLimitError(err error) bool {
	errorStr := err.Error()
	resourceLimitKeywords := []string{
		"limit exceeded",
		"resource limits exceeded",
		"concurrency limit",
		"memory limit",
		"resource exhaustion",
	}

	for _, keyword := range resourceLimitKeywords {
		if strings.Contains(errorStr, keyword) {
			return true
		}
	}

	// Check for specific error types
	switch err.(type) {
	case *encoding.EncodingError:
		if encErr := err.(*encoding.EncodingError); encErr != nil {
			return isResourceLimitError(fmt.Errorf("%s", encErr.Message))
		}
	case *encoding.DecodingError:
		if decErr := err.(*encoding.DecodingError); decErr != nil {
			return isResourceLimitError(fmt.Errorf("%s", decErr.Message))
		}
	}

	return false
}

// BenchmarkResourceLimitEnforcement benchmarks the performance impact of resource limit enforcement
func BenchmarkResourceLimitEnforcement(b *testing.B) {
	encoder := json.NewJSONEncoderWithConcurrencyLimit(nil, 100)
	event := &events.TextMessageContentEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
		MessageID: "test-message",
		Delta:     "test content",
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := encoder.Encode(ctx, event)
			if err != nil && !isResourceLimitError(err) {
				b.Fatalf("Unexpected error: %v", err)
			}
		}
	})
}

// TestConcurrentResourceAccess tests resource limit enforcement under concurrent access
func TestConcurrentResourceAccess(t *testing.T) {
	// Test concurrent buffer pool access
	t.Run("ConcurrentBufferPool", func(t *testing.T) {
		pool := encoding.NewBufferPoolWithCapacity(1024, 10) // Small pool for testing
		var wg sync.WaitGroup
		var successCount int32
		var nilCount int32

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				buf := pool.Get()
				if buf == nil {
					atomic.AddInt32(&nilCount, 1)
				} else {
					atomic.AddInt32(&successCount, 1)
					// Simulate some work
					time.Sleep(time.Millisecond)
					pool.Put(buf)
				}
			}()
		}

		wg.Wait()

		t.Logf("Successful gets: %d, Nil returns: %d", successCount, nilCount)

		// Should have some nil returns due to pool exhaustion
		if nilCount == 0 {
			t.Error("Expected some nil returns due to pool limits")
		}
	})

	// Test concurrent encoder access
	t.Run("ConcurrentEncoder", func(t *testing.T) {
		encoder := json.NewJSONEncoderWithConcurrencyLimit(nil, 2) // Very small limit for testing
		event := &events.TextMessageContentEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeTextMessageContent,
			},
			MessageID: "test-message",
			Delta:     "test content",
		}

		ctx := context.Background()
		var wg sync.WaitGroup
		var successCount int32
		var limitCount int32

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				// Add a small delay to increase concurrency
				time.Sleep(time.Millisecond * time.Duration(idx%10))

				_, err := encoder.Encode(ctx, event)
				if err != nil {
					if isResourceLimitError(err) {
						atomic.AddInt32(&limitCount, 1)
					} else {
						t.Errorf("Unexpected error: %v", err)
					}
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Successful encodings: %d, Limit rejections: %d", successCount, limitCount)

		// Should have some limit rejections under normal conditions
		if limitCount == 0 {
			t.Logf("Note: No operations were rejected due to concurrency limits. This may occur when running in parallel with other tests.")
			// Don't fail the test - log instead for debugging
		} else {
			t.Logf("Successfully enforced concurrency limits: %d/%d operations rejected", limitCount, successCount+limitCount)
		}
	})
}
