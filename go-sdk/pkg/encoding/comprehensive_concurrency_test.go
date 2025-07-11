package encoding_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/encoding/json"
	"github.com/ag-ui/go-sdk/pkg/encoding/negotiation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentRegistryOperations tests concurrent registry operations
func TestConcurrentRegistryOperations(t *testing.T) {
	const numGoroutines = 100
	const numOperations = 50
	
	registry := encoding.NewFormatRegistry()
	
	// Pre-register some formats
	require.NoError(t, registry.RegisterFormat(encoding.JSONFormatInfo()))
	require.NoError(t, registry.RegisterFormat(encoding.ProtobufFormatInfo()))
	
	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines*numOperations)
	
	// Test concurrent format registration
	t.Run("ConcurrentRegistration", func(t *testing.T) {
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					mimeType := fmt.Sprintf("application/test-%d-%d", id, j)
					info := encoding.NewFormatInfo(fmt.Sprintf("Test %d-%d", id, j), mimeType)
					
					err := registry.RegisterFormat(info)
					if err != nil {
						errorChan <- fmt.Errorf("goroutine %d: failed to register format: %w", id, err)
					}
				}
			}(i)
		}
		
		wg.Wait()
		close(errorChan)
		
		// Check for errors
		for err := range errorChan {
			t.Errorf("Registration error: %v", err)
		}
	})
	
	// Test concurrent format lookup
	t.Run("ConcurrentLookup", func(t *testing.T) {
		errorChan = make(chan error, numGoroutines*numOperations)
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					// Test different lookup operations
					operations := []func() error{
						func() error {
							_, err := registry.GetFormat("application/json")
							return err
						},
						func() error {
							if !registry.SupportsFormat("application/json") {
								return fmt.Errorf("format not supported")
							}
							return nil
						},
						func() error {
							formats := registry.ListFormats()
							if len(formats) == 0 {
								return fmt.Errorf("no formats found")
							}
							return nil
						},
						func() error {
							if !registry.SupportsEncoding("application/json") {
								return fmt.Errorf("encoding not supported")
							}
							return nil
						},
						func() error {
							if !registry.SupportsDecoding("application/json") {
								return fmt.Errorf("decoding not supported")
							}
							return nil
						},
					}
					
					op := operations[j%len(operations)]
					if err := op(); err != nil {
						errorChan <- fmt.Errorf("goroutine %d: lookup failed: %w", id, err)
					}
				}
			}(i)
		}
		
		wg.Wait()
		close(errorChan)
		
		for err := range errorChan {
			t.Errorf("Lookup error: %v", err)
		}
	})
}

// TestConcurrentEncodingDecoding tests concurrent encoding/decoding operations
func TestConcurrentEncodingDecoding(t *testing.T) {
	const numGoroutines = 50
	const numOperations = 100
	
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	// Create shared encoder/decoder
	encoder, err := registry.GetEncoder(ctx, "application/json", nil)
	require.NoError(t, err)
	
	decoder, err := registry.GetDecoder(ctx, "application/json", nil)
	require.NoError(t, err)
	
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	
	// Test concurrent encoding
	t.Run("ConcurrentEncoding", func(t *testing.T) {
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					event := events.NewTextMessageContentEvent(
						fmt.Sprintf("msg-%d-%d", id, j),
						fmt.Sprintf("content-%d-%d", id, j),
					)
					
					_, err := encoder.Encode(ctx, event)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						t.Errorf("Encoding failed: %v", err)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}(i)
		}
		
		wg.Wait()
		
		t.Logf("Encoding results: %d successes, %d errors", successCount, errorCount)
		assert.Equal(t, int64(numGoroutines*numOperations), successCount)
		assert.Equal(t, int64(0), errorCount)
	})
	
	// Reset counters
	atomic.StoreInt64(&successCount, 0)
	atomic.StoreInt64(&errorCount, 0)
	
	// Test concurrent decoding
	t.Run("ConcurrentDecoding", func(t *testing.T) {
		// Pre-encode some test data
		testEvent := events.NewTextMessageContentEvent("test-msg", "test content")
		testData, err := encoder.Encode(ctx, testEvent)
		require.NoError(t, err)
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					_, err := decoder.Decode(ctx, testData)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						t.Errorf("Decoding failed: %v", err)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}(i)
		}
		
		wg.Wait()
		
		t.Logf("Decoding results: %d successes, %d errors", successCount, errorCount)
		assert.Equal(t, int64(numGoroutines*numOperations), successCount)
		assert.Equal(t, int64(0), errorCount)
	})
}

// TestConcurrentStreamingOperations tests concurrent streaming operations
func TestConcurrentStreamingOperations(t *testing.T) {
	const numGoroutines = 20
	const eventsPerStream = 10
	
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	
	t.Run("ConcurrentStreaming", func(t *testing.T) {
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Create stream encoder/decoder for this goroutine
				encoder, err := registry.GetStreamEncoder(ctx, "application/json", nil)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					t.Errorf("Failed to get stream encoder: %v", err)
					return
				}
				
				decoder, err := registry.GetStreamDecoder(ctx, "application/json", nil)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					t.Errorf("Failed to get stream decoder: %v", err)
					return
				}
				
				// Create events for this stream
				testEvents := make([]events.Event, eventsPerStream)
				for j := 0; j < eventsPerStream; j++ {
					testEvents[j] = events.NewTextMessageContentEvent(
						fmt.Sprintf("msg-%d-%d", id, j),
						fmt.Sprintf("content-%d-%d", id, j),
					)
				}
				
				// Test streaming
				var buf bytes.Buffer
				eventChan := make(chan events.Event, eventsPerStream)
				
				// Start encoding
				go func() {
					for _, event := range testEvents {
						eventChan <- event
					}
					close(eventChan)
				}()
				
				err = encoder.EncodeStream(ctx, eventChan, &buf)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					t.Errorf("Stream encoding failed: %v", err)
					return
				}
				
				// Test decoding
				reader := bytes.NewReader(buf.Bytes())
				decodedChan := make(chan events.Event, eventsPerStream)
				
				go func() {
					err := decoder.DecodeStream(ctx, reader, decodedChan)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						t.Errorf("Stream decoding failed: %v", err)
					}
				}()
				
				// Count decoded events
				decodedCount := 0
				for range decodedChan {
					decodedCount++
				}
				
				if decodedCount == eventsPerStream {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&errorCount, 1)
					t.Errorf("Expected %d events, got %d", eventsPerStream, decodedCount)
				}
			}(i)
		}
		
		wg.Wait()
		
		t.Logf("Streaming results: %d successes, %d errors", successCount, errorCount)
		assert.Equal(t, int64(numGoroutines), successCount)
		assert.Equal(t, int64(0), errorCount)
	})
}

// TestConcurrentPoolOperations tests concurrent pool operations
func TestConcurrentPoolOperations(t *testing.T) {
	const numGoroutines = 100
	const numOperations = 50
	
	var wg sync.WaitGroup
	
	t.Run("ConcurrentBufferPool", func(t *testing.T) {
		var getCount, putCount int64
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					// Get buffer
					buf := encoding.GetBuffer(1024)
					atomic.AddInt64(&getCount, 1)
					
					// Use buffer
					buf.WriteString(fmt.Sprintf("test-%d-%d", id, j))
					
					// Put back
					encoding.PutBuffer(buf)
					atomic.AddInt64(&putCount, 1)
				}
			}(i)
		}
		
		wg.Wait()
		
		expectedOps := int64(numGoroutines * numOperations)
		assert.Equal(t, expectedOps, getCount)
		assert.Equal(t, expectedOps, putCount)
	})
	
	t.Run("ConcurrentSlicePool", func(t *testing.T) {
		var getCount, putCount int64
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					// Get slice
					slice := encoding.GetSlice(1024)
					atomic.AddInt64(&getCount, 1)
					
					// Use slice
					slice = append(slice, []byte(fmt.Sprintf("test-%d-%d", id, j))...)
					
					// Put back
					encoding.PutSlice(slice)
					atomic.AddInt64(&putCount, 1)
				}
			}(i)
		}
		
		wg.Wait()
		
		expectedOps := int64(numGoroutines * numOperations)
		assert.Equal(t, expectedOps, getCount)
		assert.Equal(t, expectedOps, putCount)
	})
	
	t.Run("ConcurrentErrorPool", func(t *testing.T) {
		var getCount, putCount int64
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					// Get encoding error
					encErr := encoding.GetEncodingError()
					atomic.AddInt64(&getCount, 1)
					
					// Use error
					encErr.Message = fmt.Sprintf("test-%d-%d", id, j)
					
					// Put back
					encoding.PutEncodingError(encErr)
					atomic.AddInt64(&putCount, 1)
					
					// Get decoding error
					decErr := encoding.GetDecodingError()
					atomic.AddInt64(&getCount, 1)
					
					// Use error
					decErr.Message = fmt.Sprintf("test-%d-%d", id, j)
					
					// Put back
					encoding.PutDecodingError(decErr)
					atomic.AddInt64(&putCount, 1)
				}
			}(i)
		}
		
		wg.Wait()
		
		expectedOps := int64(numGoroutines * numOperations * 2) // 2 error types
		assert.Equal(t, expectedOps, getCount)
		assert.Equal(t, expectedOps, putCount)
	})
}

// TestConcurrentFactoryOperations tests concurrent factory operations
func TestConcurrentFactoryOperations(t *testing.T) {
	const numGoroutines = 50
	const numOperations = 20
	
	ctx := context.Background()
	
	t.Run("ConcurrentPooledFactory", func(t *testing.T) {
		factory := encoding.NewPooledCodecFactory()
		
		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					// Create codec
					codec, err := factory.CreateCodec(ctx, "application/json", nil, nil)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					
					// Use encoder
					event := events.NewTextMessageContentEvent(
						fmt.Sprintf("msg-%d-%d", id, j),
						fmt.Sprintf("content-%d-%d", id, j),
					)
					
					_, err = codec.Encode(ctx, event)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
					
					// Release codec
					if releasable, ok := codec.(encoding.ReleasableEncoder); ok {
						releasable.Release()
					}
				}
			}(i)
		}
		
		wg.Wait()
		
		t.Logf("Factory results: %d successes, %d errors", successCount, errorCount)
		assert.Equal(t, int64(numGoroutines*numOperations), successCount)
		assert.Equal(t, int64(0), errorCount)
	})
}

// TestConcurrentNegotiation tests concurrent content negotiation
func TestConcurrentNegotiation(t *testing.T) {
	const numGoroutines = 100
	const numOperations = 50
	
	negotiator := negotiation.NewContentNegotiator("application/json")
	
	acceptHeaders := []string{
		"application/json",
		"application/x-protobuf,application/json;q=0.8",
		"text/html,application/xhtml+xml,application/xml;q=0.9,application/json;q=0.8,*/*;q=0.7",
		"*/*",
	}
	
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				header := acceptHeaders[j%len(acceptHeaders)]
				
				_, err := negotiator.Negotiate(header)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Negotiation results: %d successes, %d errors", successCount, errorCount)
	assert.Equal(t, int64(numGoroutines*numOperations), successCount)
	assert.Equal(t, int64(0), errorCount)
}

// TestMemoryLeakDetection tests for memory leaks in concurrent scenarios
func TestMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	const numIterations = 1000
	const numGoroutines = 10
	
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	// Force GC and get initial memory stats
	runtime.GC()
	var initialStats runtime.MemStats
	runtime.ReadMemStats(&initialStats)
	
	var wg sync.WaitGroup
	
	for iteration := 0; iteration < numIterations; iteration++ {
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Create encoder
				encoder, err := registry.GetEncoder(ctx, "application/json", nil)
				if err != nil {
					t.Errorf("Failed to get encoder: %v", err)
					return
				}
				
				// Use encoder
				event := events.NewTextMessageContentEvent("msg", "content")
				_, err = encoder.Encode(ctx, event)
				if err != nil {
					t.Errorf("Encoding failed: %v", err)
				}
			}(i)
		}
		
		wg.Wait()
		
		// Periodic GC to help detect leaks
		if iteration%100 == 0 {
			runtime.GC()
		}
	}
	
	// Final GC and memory check
	runtime.GC()
	var finalStats runtime.MemStats
	runtime.ReadMemStats(&finalStats)
	
	// Check for significant memory increase
	memoryIncrease := finalStats.Alloc - initialStats.Alloc
	t.Logf("Memory increase: %d bytes", memoryIncrease)
	
	// Allow some memory increase, but not too much
	maxAllowedIncrease := uint64(10 * 1024 * 1024) // 10MB
	if memoryIncrease > maxAllowedIncrease {
		t.Errorf("Potential memory leak detected: memory increased by %d bytes", memoryIncrease)
	}
}

// TestRaceConditions tests for race conditions using race detector
func TestRaceConditions(t *testing.T) {
	const numGoroutines = 50
	const numOperations = 100
	
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	// Shared resources
	sharedData := make(map[string][]byte)
	var sharedMutex sync.RWMutex
	
	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Get encoder
				encoder, err := registry.GetEncoder(ctx, "application/json", nil)
				if err != nil {
					t.Errorf("Failed to get encoder: %v", err)
					continue
				}
				
				// Create event
				event := events.NewTextMessageContentEvent(
					fmt.Sprintf("msg-%d-%d", id, j),
					fmt.Sprintf("content-%d-%d", id, j),
				)
				
				// Encode
				data, err := encoder.Encode(ctx, event)
				if err != nil {
					t.Errorf("Encoding failed: %v", err)
					continue
				}
				
				// Store in shared map
				key := fmt.Sprintf("key-%d-%d", id, j)
				sharedMutex.Lock()
				sharedData[key] = data
				sharedMutex.Unlock()
				
				// Read from shared map
				sharedMutex.RLock()
				_, exists := sharedData[key]
				sharedMutex.RUnlock()
				
				if !exists {
					t.Errorf("Data not found in shared map")
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify final state
	sharedMutex.RLock()
	dataCount := len(sharedData)
	sharedMutex.RUnlock()
	
	expectedCount := numGoroutines * numOperations
	assert.Equal(t, expectedCount, dataCount)
}

// TestDeadlockDetection tests for potential deadlocks
func TestDeadlockDetection(t *testing.T) {
	const numGoroutines = 10
	const timeout = 5 * time.Second
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	registry := encoding.GetGlobalRegistry()
	
	// Create multiple resources that could potentially deadlock
	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Try to acquire multiple resources in different orders
			resources := []string{
				"application/json",
				"application/x-protobuf",
			}
			
			// Randomize order to increase chance of deadlock
			rand.Shuffle(len(resources), func(i, j int) {
				resources[i], resources[j] = resources[j], resources[i]
			})
			
			for _, mimeType := range resources {
				encoder, err := registry.GetEncoder(ctx, mimeType, nil)
				if err != nil {
					t.Errorf("Failed to get encoder for %s: %v", mimeType, err)
					return
				}
				
				// Use encoder
				event := events.NewTextMessageContentEvent("msg", "content")
				_, err = encoder.Encode(ctx, event)
				if err != nil {
					t.Errorf("Encoding failed: %v", err)
				}
			}
		}(i)
	}
	
	// Wait with timeout
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()
	
	select {
	case <-done:
		t.Log("No deadlock detected")
	case <-ctx.Done():
		t.Fatal("Potential deadlock detected: test timed out")
	}
}

// TestStressTest runs a comprehensive stress test
func TestStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	
	const duration = 10 * time.Second
	const numGoroutines = 20
	
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	registry := encoding.GetGlobalRegistry()
	
	var wg sync.WaitGroup
	var totalOps int64
	var errorCount int64
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			localOps := int64(0)
			
			for {
				select {
				case <-ctx.Done():
					atomic.AddInt64(&totalOps, localOps)
					return
				default:
					// Perform random operations
					operations := []func() error{
						func() error {
							encoder, err := registry.GetEncoder(ctx, "application/json", nil)
							if err != nil {
								return err
							}
							event := events.NewTextMessageContentEvent("msg", "content")
							_, err = encoder.Encode(ctx, event)
							return err
						},
						func() error {
							decoder, err := registry.GetDecoder(ctx, "application/json", nil)
							if err != nil {
								return err
							}
							data := []byte(`{"type":"TEXT_MESSAGE_CONTENT","messageId":"msg","delta":"content","timestamp":1234567890}`)
							_, err = decoder.Decode(ctx, data)
							return err
						},
						func() error {
							buf := encoding.GetBuffer(1024)
							defer encoding.PutBuffer(buf)
							buf.WriteString("test")
							return nil
						},
						func() error {
							slice := encoding.GetSlice(1024)
							defer encoding.PutSlice(slice)
							slice = append(slice, []byte("test")...)
							return nil
						},
					}
					
					op := operations[rand.Intn(len(operations))]
					if err := op(); err != nil {
						atomic.AddInt64(&errorCount, 1)
					}
					
					localOps++
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Stress test completed: %d operations, %d errors", totalOps, errorCount)
	
	// Verify we performed a reasonable number of operations
	assert.Greater(t, totalOps, int64(1000), "Should have performed at least 1000 operations")
	
	// Error rate should be low
	errorRate := float64(errorCount) / float64(totalOps) * 100
	assert.Less(t, errorRate, 5.0, "Error rate should be less than 5%")
}

// TestGoroutineLeaks tests for goroutine leaks
func TestGoroutineLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping goroutine leak test in short mode")
	}
	
	initialGoroutines := runtime.NumGoroutine()
	
	const numIterations = 100
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	for i := 0; i < numIterations; i++ {
		// Create stream encoder
		encoder, err := registry.GetStreamEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Use streaming operations
		var buf bytes.Buffer
		eventChan := make(chan events.Event, 1)
		
		// Start and immediately cancel
		go func() {
			encoder.EncodeStream(ctx, eventChan, &buf)
		}()
		
		// Send event and close
		eventChan <- events.NewTextMessageContentEvent("msg", "content")
		close(eventChan)
		
		// Give goroutine time to finish
		time.Sleep(1 * time.Millisecond)
	}
	
	// Wait for goroutines to finish
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	
	finalGoroutines := runtime.NumGoroutine()
	
	// Allow some variance, but not too much
	maxAllowedIncrease := 10
	if finalGoroutines > initialGoroutines+maxAllowedIncrease {
		t.Errorf("Potential goroutine leak: started with %d, ended with %d", initialGoroutines, finalGoroutines)
	}
}

// init ensures tests have some randomness
func init() {
	rand.Seed(time.Now().UnixNano())
}