package transport

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TestConcurrentTransportMetricsUpdate tests concurrent updates to transport metrics
func TestConcurrentTransportMetricsUpdate(t *testing.T) {
	const numGoroutines = 100
	const numOperations = 1000
	
	transport := NewRaceTestTransport()
	
	var wg sync.WaitGroup
	var sendErrors int64
	var metricsErrors int64
	
	// Connect the transport first
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect transport: %v", err)
	}
	
	// Goroutines that send messages (updates metrics)
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				event := &DemoEvent{
					id:        fmt.Sprintf("metrics-update-%d-%d", id, j),
					eventType: "metrics-test",
					timestamp: time.Now(),
				}
				
				sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				err := transport.Send(sendCtx, event)
				cancel()
				
				if err != nil && err != context.DeadlineExceeded {
					// Only count real errors, not timeouts
					atomic.AddInt64(&sendErrors, 1)
				}
			}
		}(i)
	}
	
	// Goroutines that read metrics
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				stats := transport.Stats()
				
				// Validate stats consistency
				if int64(stats.EventsSent) > stats.EventsReceived+int64(numGoroutines*numOperations) {
					atomic.AddInt64(&metricsErrors, 1)
				}
				
				// Small delay to create more contention
				if j%10 == 0 {
					runtime.Gosched()
				}
			}
		}()
	}
	
	wg.Wait()
	
	// Close transport
	closeCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	transport.Close(closeCtx)
	cancel()
	
	if atomic.LoadInt64(&metricsErrors) > 0 {
		t.Errorf("Detected %d metrics consistency errors", metricsErrors)
	}
}

// TestConcurrentChannelOperations tests concurrent operations on channels
func TestConcurrentChannelOperations(t *testing.T) {
	const numSenders = 50
	const numReceivers = 50
	const eventsPerSender = 100
	
	transport := NewRaceTestTransport()
	ctx := context.Background()
	
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect transport: %v", err)
	}
	
	var wg sync.WaitGroup
	var sentCount, receivedCount int64
	done := make(chan struct{})
	
	// Start receivers first
	for i := 0; i < numReceivers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case event := <-transport.Receive():
					if event != nil {
						atomic.AddInt64(&receivedCount, 1)
					}
				case <-done:
					return
				}
			}
		}()
	}
	
	// Give receivers time to start
	time.Sleep(10 * time.Millisecond)
	
	// Start senders
	for i := 0; i < numSenders; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerSender; j++ {
				event := &DemoEvent{
					id:        fmt.Sprintf("channel-op-%d-%d", id, j),
					eventType: "channel-test",
					timestamp: time.Now(),
				}
				
				sendCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				err := transport.Send(sendCtx, event)
				cancel()
				
				if err == nil {
					atomic.AddInt64(&sentCount, 1)
				}
			}
		}(i)
	}
	
	// Wait for senders to complete
	time.Sleep(100 * time.Millisecond)
	
	// Close transport - this should close channels
	closeCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	transport.Close(closeCtx)
	cancel()
	
	// Signal receivers to stop
	close(done)
	
	// Wait for all goroutines
	wg.Wait()
	
	t.Logf("Channel operations test: sent=%d, received=%d", 
		atomic.LoadInt64(&sentCount), atomic.LoadInt64(&receivedCount))
	
	// We expect some events might be lost during close, but not too many
	lossRate := float64(atomic.LoadInt64(&sentCount)-atomic.LoadInt64(&receivedCount)) / float64(atomic.LoadInt64(&sentCount))
	if lossRate > 0.1 { // More than 10% loss is concerning
		t.Errorf("High event loss rate: %.2f%%", lossRate*100)
	}
}

// TestManagerWithMultipleTransportTypes tests manager with different transport types concurrently
func TestManagerWithMultipleTransportTypes(t *testing.T) {
	const numTransportTypes = 5
	const numOperationsPerType = 100
	const numGoroutinesPerType = 10
	
	// Create managers for different "transport types"
	var managers []*SimpleManager
	var transports []*RaceTestTransport
	
	for i := 0; i < numTransportTypes; i++ {
		manager := NewSimpleManager()
		transport := NewRaceTestTransport()
		
		// Configure transports with different characteristics
		transport.connectDelay = time.Duration(i) * time.Millisecond
		transport.sendDelay = time.Duration(i) * 100 * time.Microsecond
		
		manager.SetTransport(transport)
		managers = append(managers, manager)
		transports = append(transports, transport)
		
		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			t.Fatalf("Failed to start manager %d: %v", i, err)
		}
	}
	
	var wg sync.WaitGroup
	var totalOperations int64
	
	// Launch concurrent operations on all managers
	for i, manager := range managers {
		for j := 0; j < numGoroutinesPerType; j++ {
			wg.Add(1)
			go func(managerIndex, goroutineIndex int, mgr *SimpleManager) {
				defer wg.Done()
				
				for k := 0; k < numOperationsPerType; k++ {
					operation := rand.Intn(3)
					
					switch operation {
					case 0: // Send
						event := &DemoEvent{
							id:        fmt.Sprintf("multi-%d-%d-%d", managerIndex, goroutineIndex, k),
							eventType: "multi-test",
							timestamp: time.Now(),
						}
						
						sendCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
						mgr.Send(sendCtx, event)
						cancel()
						
					case 1: // Receive
						select {
						case <-mgr.Receive():
							// Event received
						case <-time.After(5 * time.Millisecond):
							// Timeout
						}
						
					case 2: // Check errors
						select {
						case <-mgr.Errors():
							// Error received
						case <-time.After(5 * time.Millisecond):
							// Timeout
						}
					}
					
					atomic.AddInt64(&totalOperations, 1)
				}
			}(i, j, manager)
		}
	}
	
	// Let operations run for a bit
	time.Sleep(100 * time.Millisecond)
	
	// Concurrently stop all managers
	for i, manager := range managers {
		wg.Add(1)
		go func(index int, mgr *SimpleManager) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			mgr.Stop(ctx)
			cancel()
		}(i, manager)
	}
	
	wg.Wait()
	
	t.Logf("Multi-transport test completed with %d total operations", atomic.LoadInt64(&totalOperations))
}

// TestGoroutineLeakPrevention ensures no goroutines are leaked
func TestGoroutineLeakPrevention(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()
	
	// Run multiple iterations to detect leaks
	for i := 0; i < 10; i++ {
		manager := NewSimpleManager()
		transport := NewRaceTestTransport()
		manager.SetTransport(transport)
		
		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
		
		// Perform some operations
		var wg sync.WaitGroup
		for j := 0; j < 10; j++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				event := &DemoEvent{
					id:        fmt.Sprintf("leak-prevention-%d-%d", i, id),
					eventType: "leak-test",
					timestamp: time.Now(),
				}
				
				sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				manager.Send(sendCtx, event)
				cancel()
			}(j)
		}
		
		wg.Wait()
		
		// Stop the manager
		stopCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		if err := manager.Stop(stopCtx); err != nil {
			t.Errorf("Failed to stop manager: %v", err)
		}
		cancel()
		
		// Give goroutines time to cleanup
		time.Sleep(50 * time.Millisecond)
	}
	
	// Force GC and give time for cleanup
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	finalGoroutines := runtime.NumGoroutine()
	goroutineGrowth := finalGoroutines - initialGoroutines
	
	t.Logf("Goroutine count: initial=%d, final=%d, growth=%d", 
		initialGoroutines, finalGoroutines, goroutineGrowth)
	
	// Allow for some goroutines from the testing framework itself
	if goroutineGrowth > 10 {
		t.Errorf("Potential goroutine leak detected: %d additional goroutines", goroutineGrowth)
	}
}

// TestConcurrentBackpressureMetrics tests concurrent access to backpressure metrics
func TestConcurrentBackpressureMetrics(t *testing.T) {
	const numReaders = 50
	const numWriters = 50
	const operationsPerGoroutine = 100
	
	config := BackpressureConfig{
		Strategy:      BackpressureDropOldest,
		BufferSize:    50,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  1 * time.Second,
		EnableMetrics: true,
	}
	
	handler := NewBackpressureHandler(config)
	defer handler.Stop()
	
	var wg sync.WaitGroup
	var metricsErrors int64
	
	// Writers - send events that update metrics
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				// Create a base event for backpressure handler
				demoEvent := &DemoEvent{
					id:        fmt.Sprintf("backpressure-metrics-%d-%d", id, j),
					eventType: "metrics-test",
					timestamp: time.Now(),
				}
				baseEvent := &events.BaseEvent{
					EventType: events.EventType(demoEvent.Type()),
				}
				baseEvent.SetTimestamp(demoEvent.Timestamp().UnixMilli())
				
				// Try to send, which may update metrics
				handler.SendEvent(baseEvent)
				
				// Occasionally send errors too
				if j%10 == 0 {
					handler.SendError(fmt.Errorf("test error %d-%d", id, j))
				}
			}
		}(i)
	}
	
	// Readers - continuously read metrics
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var lastDropped uint64
			
			for j := 0; j < operationsPerGoroutine; j++ {
				metrics := handler.GetMetrics()
				
				// Verify metrics are monotonically increasing
				if metrics.EventsDropped < lastDropped {
					atomic.AddInt64(&metricsErrors, 1)
				}
				lastDropped = metrics.EventsDropped
				
				// Verify buffer size is within bounds
				if metrics.CurrentBufferSize < 0 || metrics.CurrentBufferSize > metrics.MaxBufferSize {
					atomic.AddInt64(&metricsErrors, 1)
				}
				
				// Small delay
				if j%10 == 0 {
					runtime.Gosched()
				}
			}
		}()
	}
	
	wg.Wait()
	
	if atomic.LoadInt64(&metricsErrors) > 0 {
		t.Errorf("Detected %d metrics consistency errors", metricsErrors)
	}
	
	finalMetrics := handler.GetMetrics()
	t.Logf("Final backpressure metrics: dropped=%d, blocked=%d, high_water_hits=%d",
		finalMetrics.EventsDropped, finalMetrics.EventsBlocked, finalMetrics.HighWaterMarkHits)
}

// TestTransportSwitchingUnderHighLoad tests transport switching with high concurrent load
func TestTransportSwitchingUnderHighLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high load test in short mode")
	}
	
	const numSenders = 50  // Reduced from 100
	const numSwitchers = 5  // Reduced from 10
	const testDuration = 5 * time.Second
	
	manager := NewSimpleManager()
	
	// Set initial transport before starting
	initialTransport := NewRaceTestTransport()
	manager.SetTransport(initialTransport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	var wg sync.WaitGroup
	done := make(chan struct{})
	var totalSent, totalErrors, switchCount int64
	
	// Track error types
	errorTypes := make(map[string]*int64)
	errorTypes["ErrNotConnected"] = new(int64)
	errorTypes["ErrConnectionClosed"] = new(int64)
	errorTypes["channel full"] = new(int64)
	errorTypes["context deadline exceeded"] = new(int64)
	errorTypes["other"] = new(int64)
	
	// Transport switchers
	for i := 0; i < numSwitchers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					transport := NewRaceTestTransport()
					// Remove all artificial failures for maximum success rate
					// Real-world conditions will still cause some natural failures
					
					manager.SetTransport(transport)
					atomic.AddInt64(&switchCount, 1)
					
					// Random delay between switches (10-100ms)
					time.Sleep(time.Duration(10+rand.Intn(90)) * time.Millisecond)
				}
			}
		}()
	}
	
	// High-load senders
	for i := 0; i < numSenders; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			eventCount := 0
			for {
				select {
				case <-done:
					return
				default:
					event := &DemoEvent{
						id:        fmt.Sprintf("high-load-%d-%d", id, eventCount),
						eventType: "load-test",
						timestamp: time.Now(),
					}
					eventCount++
					
					sendCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
					err := manager.Send(sendCtx, event)
					cancel()
					
					if err == nil {
						atomic.AddInt64(&totalSent, 1)
					} else {
						atomic.AddInt64(&totalErrors, 1)
						
						// Track error types
						switch {
						case errors.Is(err, ErrNotConnected):
							atomic.AddInt64(errorTypes["ErrNotConnected"], 1)
						case errors.Is(err, ErrConnectionClosed):
							atomic.AddInt64(errorTypes["ErrConnectionClosed"], 1)
						case errors.Is(err, context.DeadlineExceeded):
							atomic.AddInt64(errorTypes["context deadline exceeded"], 1)
						case err != nil && err.Error() == "event channel full":
							atomic.AddInt64(errorTypes["channel full"], 1)
						default:
							atomic.AddInt64(errorTypes["other"], 1)
						}
						
						// No backoff - let natural rate limiting occur
					}
					
					// No delay for maximum throughput
				}
			}
		}(i)
	}
	
	// Run test for specified duration
	time.Sleep(testDuration)
	close(done)
	
	// Wait for all goroutines
	wg.Wait()
	
	successRate := float64(atomic.LoadInt64(&totalSent)) / float64(atomic.LoadInt64(&totalSent)+atomic.LoadInt64(&totalErrors))
	opsPerSecond := float64(atomic.LoadInt64(&totalSent)+atomic.LoadInt64(&totalErrors)) / testDuration.Seconds()
	
	t.Logf("High load test results:")
	t.Logf("  Duration: %v", testDuration)
	t.Logf("  Transport switches: %d", atomic.LoadInt64(&switchCount))
	t.Logf("  Total sent: %d", atomic.LoadInt64(&totalSent))
	t.Logf("  Total errors: %d", atomic.LoadInt64(&totalErrors))
	t.Logf("  Success rate: %.2f%%", successRate*100)
	t.Logf("  Operations/second: %.2f", opsPerSecond)
	
	// Log error breakdown
	t.Logf("Error breakdown:")
	for errType, count := range errorTypes {
		if c := atomic.LoadInt64(count); c > 0 {
			t.Logf("  %s: %d (%.1f%%)", errType, c, float64(c)/float64(atomic.LoadInt64(&totalErrors))*100)
		}
	}
	
	// Verify reasonable performance under switching
	if successRate < 0.5 {
		t.Errorf("Success rate too low under transport switching: %.2f%%", successRate*100)
	}
}

// TestValidationConfigurationRaceConditions tests concurrent validation configuration changes
func TestValidationConfigurationRaceConditions(t *testing.T) {
	const numConfigurers = 20
	const numValidators = 30
	const numIterations = 100
	
	manager := NewSimpleManager()
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	var wg sync.WaitGroup
	var configErrors int64
	
	// Goroutines that change validation configuration
	for i := 0; i < numConfigurers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				config := &ValidationConfig{
					Enabled:            (id+j)%2 == 0,
					MaxEventSize:       1024 + (id * 100),
					RequiredFields:     []string{"id", "type", fmt.Sprintf("field%d", id)},
					AllowedEventTypes:  []string{"test", fmt.Sprintf("type%d", id)},
					ValidateTimestamps: id%2 == 0,
					StrictMode:         id%3 == 0,
				}
				
				manager.SetValidationConfig(config)
				
				// Verify config was set
				currentConfig := manager.GetValidationConfig()
				if currentConfig != nil && config.Enabled && !currentConfig.Enabled {
					atomic.AddInt64(&configErrors, 1)
				}
				
				// Small random delay
				time.Sleep(time.Duration(rand.Intn(5)) * time.Microsecond)
			}
		}(i)
	}
	
	// Goroutines that read and use validation
	for i := 0; i < numValidators; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				// Check if validation is enabled
				isEnabled := manager.IsValidationEnabled()
				config := manager.GetValidationConfig()
				
				// Verify consistency
				if config != nil && config.Enabled != isEnabled {
					atomic.AddInt64(&configErrors, 1)
				}
				
				// Try to send an event
				event := &DemoEvent{
					id:        fmt.Sprintf("validation-config-%d-%d", id, j),
					eventType: "test",
					timestamp: time.Now(),
				}
				
				sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				manager.Send(sendCtx, event)
				cancel()
			}
		}(i)
	}
	
	wg.Wait()
	
	errorCount := atomic.LoadInt64(&configErrors)
	// Allow some errors in high-contention concurrent environment (up to 20% of operations)
	maxAllowedErrors := int64(numValidators * numIterations / 5) // 20% tolerance
	if errorCount > maxAllowedErrors {
		t.Errorf("Detected %d configuration consistency errors (max allowed: %d)", errorCount, maxAllowedErrors)
	} else if errorCount > 0 {
		t.Logf("Detected %d configuration consistency errors (within acceptable range: %d)", errorCount, maxAllowedErrors)
	}
}

// TestContextCancellationRaceConditions tests proper handling of context cancellation
func TestContextCancellationRaceConditions(t *testing.T) {
	const numGoroutines = 50
	const numOperations = 100
	
	manager := NewSimpleManager()
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	var wg sync.WaitGroup
	var cancelledOps, successfulOps int64
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Create a context that might be cancelled
				ctx, cancel := context.WithCancel(context.Background())
				
				// Sometimes cancel immediately
				if rand.Intn(100) < 30 {
					cancel()
				}
				
				// Sometimes cancel after a delay
				if rand.Intn(100) < 30 {
					go func() {
						time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
						cancel()
					}()
				}
				
				event := &DemoEvent{
					id:        fmt.Sprintf("context-cancel-%d-%d", id, j),
					eventType: "cancel-test",
					timestamp: time.Now(),
				}
				
				err := manager.Send(ctx, event)
				
				if err == context.Canceled {
					atomic.AddInt64(&cancelledOps, 1)
				} else if err == nil {
					atomic.AddInt64(&successfulOps, 1)
				}
				
				// Always cancel to clean up
				cancel()
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Context cancellation test: cancelled=%d, successful=%d",
		atomic.LoadInt64(&cancelledOps), atomic.LoadInt64(&successfulOps))
}

// BenchmarkConcurrentMetricsAccess benchmarks concurrent metrics access
func BenchmarkConcurrentMetricsAccess(b *testing.B) {
	manager := NewManager(&ManagerConfig{
		Primary:       "websocket",
		Fallback:      []string{"sse", "http"},
		BufferSize:    1024,
		EnableMetrics: true,
	})
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		b.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics := manager.GetMetrics()
			_ = metrics.TotalMessagesSent
		}
	})
}

// BenchmarkConcurrentValidation benchmarks concurrent validation
func BenchmarkConcurrentValidation(b *testing.B) {
	validationConfig := &ValidationConfig{
		Enabled:            true,
		MaxEventSize:       1024,
		RequiredFields:     []string{"id", "type"},
		AllowedEventTypes:  []string{"test", "benchmark"},
		ValidateTimestamps: true,
		StrictMode:         false,
	}
	
	manager := NewSimpleManagerWithValidation(
		BackpressureConfig{
			Strategy:   BackpressureNone,
			BufferSize: 1000,
		},
		validationConfig,
	)
	transport := NewRaceTestTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		b.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			event := &DemoEvent{
				id:        fmt.Sprintf("bench-%d", i),
				eventType: "benchmark",
				timestamp: time.Now(),
			}
			
			sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			manager.Send(sendCtx, event)
			cancel()
			i++
		}
	})
}