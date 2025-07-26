// Package transport benchmarks provide comprehensive performance testing for transport operations.
// This file contains benchmarks for:
// - Send operations (various payload sizes, batch operations)
// - Receive operations (sequential and burst patterns)
// - Manager lifecycle operations (creation, startup, shutdown)
// - Concurrent operations (multiple goroutines)
// - Memory allocation patterns
// - Error conditions and handling
// - Backpressure scenarios
// - Feature comparisons (with/without logging, validation, etc.)
//
// Usage:
//   go test -bench=. -benchmem                    # Run all benchmarks
//   go test -bench=BenchmarkSend -benchmem        # Run send benchmarks
//   go test -bench=BenchmarkConcurrent -benchmem  # Run concurrent benchmarks
//
// See ExampleBenchmark() for more usage examples and profiling options.
package transport

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
	
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// BenchmarkEvent implements TransportEvent for benchmarking
// Deprecated: Use typed events with CreateDataEvent, CreateConnectionEvent, etc.
type BenchmarkEvent struct {
	id        string
	eventType string
	data      map[string]interface{}
	timestamp time.Time
}

func (e *BenchmarkEvent) ID() string                      { return e.id }
func (e *BenchmarkEvent) Type() string                    { return e.eventType }
func (e *BenchmarkEvent) Timestamp() time.Time            { return e.timestamp }
func (e *BenchmarkEvent) Data() map[string]interface{}    { return e.data }


// BenchmarkMockTransport implements Transport interface for benchmarking
type BenchmarkMockTransport struct {
	connected      bool
	eventChan      chan events.Event
	errorChan      chan error
	capabilities   Capabilities
	metrics        Metrics
	middleware     []Middleware
	sendDelay      time.Duration
	receiveDelay   time.Duration
	enableMetrics  bool
	messagesSent   uint64
	messagesReceived uint64
	mu             sync.RWMutex
}

func NewBenchmarkMockTransport(bufferSize int) *BenchmarkMockTransport {
	// Use simplified capabilities
	caps := Capabilities{
		Streaming:        true,
		Bidirectional:    true,
		MaxMessageSize:   1024 * 1024,
		ProtocolVersion:  "1.0",
	}
	
	return &BenchmarkMockTransport{
		eventChan:    make(chan events.Event, bufferSize),
		errorChan:    make(chan error, bufferSize),
		capabilities: caps,
		metrics: Metrics{
			ConnectionUptime: 0,
			AverageLatency:   10 * time.Millisecond,
		},
		enableMetrics: true,
	}
}

func (t *BenchmarkMockTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Check context before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	t.connected = true
	t.metrics.ConnectionUptime = time.Since(time.Now())
	return nil
}

func (t *BenchmarkMockTransport) Close(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Check context before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	if t.connected {
		t.connected = false
		close(t.eventChan)
		close(t.errorChan)
	}
	return nil
}

func (t *BenchmarkMockTransport) Send(ctx context.Context, event TransportEvent) error {
	if t.sendDelay > 0 {
		select {
		case <-time.After(t.sendDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if !t.connected {
		return ErrNotConnected
	}
	
	// Simulate sending by incrementing counter
	t.messagesSent++
	if t.enableMetrics {
		t.metrics.BytesSent += uint64(len(event.ID()) + len(event.Type()))
	}
	
	return nil
}

func (t *BenchmarkMockTransport) Receive() <-chan events.Event {
	return t.eventChan
}

func (t *BenchmarkMockTransport) Errors() <-chan error {
	return t.errorChan
}

func (t *BenchmarkMockTransport) Channels() (<-chan events.Event, <-chan error) {
	return t.eventChan, t.errorChan
}

func (t *BenchmarkMockTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// Capabilities, Health and Metrics functionality removed - not part of Transport interface

// SetMiddleware removed - not part of Transport interface

// Config returns the transport configuration
func (t *BenchmarkMockTransport) Config() Config {
	return &BaseConfig{
		Type:     "mock",
		Endpoint: "mock://localhost",
	}
}

// Stats returns transport statistics
func (t *BenchmarkMockTransport) Stats() TransportStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return TransportStats{
		EventsSent:     int64(t.messagesSent),
		EventsReceived: int64(t.messagesReceived),
		BytesSent:      0,
		BytesReceived:  0,
		ConnectedAt:    time.Now(),
	}
}

// Generate test events of various sizes using type-safe APIs
func generateEvent(id string, payloadSize int) TransportEvent {
	// Create payload of specified size
	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	
	// Create benchmark event
	return &BenchmarkEvent{
		id:        id,
		eventType: "benchmark",
		data: map[string]interface{}{
			"payload": payload,
			"size":    payloadSize,
		},
		timestamp: time.Now(),
	}
}

// Benchmark send operations with different payload sizes
func BenchmarkSend_SmallPayload(b *testing.B) {
	benchmarkSend(b, 100, false) // 100 bytes
}

func BenchmarkSend_MediumPayload(b *testing.B) {
	benchmarkSend(b, 1024, false) // 1KB
}

func BenchmarkSend_LargePayload(b *testing.B) {
	benchmarkSend(b, 10*1024, false) // 10KB
}

func BenchmarkSend_VeryLargePayload(b *testing.B) {
	benchmarkSend(b, 100*1024, false) // 100KB
}

func BenchmarkSend_WithMetrics(b *testing.B) {
	benchmarkSend(b, 1024, true) // 1KB with metrics
}

func benchmarkSend(b *testing.B, payloadSize int, enableMetrics bool) {
	transport := NewBenchmarkMockTransport(1000)
	transport.enableMetrics = enableMetrics
	
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		b.Fatal(err)
	}
	defer transport.Close(ctx)
	
	event := generateEvent("benchmark", payloadSize)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		if err := transport.Send(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark batch send operations
func BenchmarkSendBatch_10Events(b *testing.B) {
	benchmarkSendBatch(b, 10, 1024)
}

func BenchmarkSendBatch_100Events(b *testing.B) {
	benchmarkSendBatch(b, 100, 1024)
}

func BenchmarkSendBatch_1000Events(b *testing.B) {
	benchmarkSendBatch(b, 1000, 1024)
}

func benchmarkSendBatch(b *testing.B, batchSize, payloadSize int) {
	transport := NewBenchmarkMockTransport(batchSize * 2)
	
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		b.Fatal(err)
	}
	defer transport.Close(ctx)
	
	// Pre-generate events
	events := make([]TransportEvent, batchSize)
	for i := 0; i < batchSize; i++ {
		events[i] = generateEvent(fmt.Sprintf("batch-%d", i), payloadSize)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		for _, event := range events {
			if err := transport.Send(ctx, event); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// Benchmark receive operations
func BenchmarkReceive_Sequential(b *testing.B) {
	benchmarkReceive(b, 1, 1024)
}

func BenchmarkReceive_Burst(b *testing.B) {
	benchmarkReceive(b, 100, 1024)
}

func benchmarkReceive(b *testing.B, burstSize, payloadSize int) {
	transport := NewBenchmarkMockTransport(burstSize * 2)
	
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		b.Fatal(err)
	}
	defer transport.Close(ctx)
	
	// Pre-populate events
	go func() {
		for i := 0; i < b.N*burstSize; i++ {
			// Create a mock core event for benchmarking
			timestamp := time.Now().UnixMilli()
			event := &BackpressureMockCoreEvent{
				BaseEvent: &events.BaseEvent{
					EventType:   events.EventTypeCustom,
					TimestampMs: &timestamp,
				},
				id: fmt.Sprintf("receive-%d", i),
			}
			select {
			case transport.eventChan <- event:
			case <-time.After(time.Second):
				return
			}
		}
	}()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	received := 0
	for i := 0; i < b.N; i++ {
		for j := 0; j < burstSize; j++ {
			select {
			case <-transport.Receive():
				received++
			case <-time.After(time.Second):
				b.Fatalf("Timeout waiting for event, received %d/%d", received, b.N*burstSize)
			}
		}
	}
}

// Benchmark manager lifecycle operations
func BenchmarkManagerLifecycle_Create(b *testing.B) {
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		manager := NewSimpleManager()
		_ = manager
	}
}

func BenchmarkManagerLifecycle_StartStop(b *testing.B) {
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		manager := NewSimpleManager()
		transport := NewBenchmarkMockTransport(100)
		manager.SetTransport(transport)
		
		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			b.Fatal(err)
		}
		if err := manager.Stop(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkManagerLifecycle_SetTransport(b *testing.B) {
	manager := NewSimpleManager()
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		transport := NewBenchmarkMockTransport(100)
		manager.SetTransport(transport)
	}
}

// Benchmark concurrent operations
func BenchmarkConcurrentSend_2Goroutines(b *testing.B) {
	benchmarkConcurrentSend(b, 2, 1024)
}

func BenchmarkConcurrentSend_10Goroutines(b *testing.B) {
	benchmarkConcurrentSend(b, 10, 1024)
}

func BenchmarkConcurrentSend_100Goroutines(b *testing.B) {
	benchmarkConcurrentSend(b, 100, 1024)
}

func benchmarkConcurrentSend(b *testing.B, numGoroutines, payloadSize int) {
	transport := NewBenchmarkMockTransport(numGoroutines * 100)
	
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		b.Fatal(err)
	}
	defer transport.Close(ctx)
	
	event := generateEvent("concurrent", payloadSize)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var wg sync.WaitGroup
	sendsPerGoroutine := b.N / numGoroutines
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < sendsPerGoroutine; j++ {
				if err := transport.Send(ctx, event); err != nil {
					b.Errorf("Send failed: %v", err)
				}
			}
		}()
	}
	
	wg.Wait()
}

func BenchmarkConcurrentReceive_2Goroutines(b *testing.B) {
	benchmarkConcurrentReceive(b, 2, 1024)
}

func BenchmarkConcurrentReceive_10Goroutines(b *testing.B) {
	benchmarkConcurrentReceive(b, 10, 1024)
}

func benchmarkConcurrentReceive(b *testing.B, numGoroutines, payloadSize int) {
	// Create a simple concurrent event processing test
	b.ResetTimer()
	b.ReportAllocs()
	
	var wg sync.WaitGroup
	eventChan := make(chan events.Event, numGoroutines*10)
	
	// Create events for processing
	testEvents := make([]events.Event, b.N)
	for i := 0; i < b.N; i++ {
		timestamp := time.Now().UnixMilli()
		testEvents[i] = &BackpressureMockCoreEvent{
			BaseEvent: &events.BaseEvent{
				EventType:   events.EventTypeCustom,
				TimestampMs: &timestamp,
			},
			id: fmt.Sprintf("concurrent-%d", i),
		}
	}
	
	// Start producer
	go func() {
		for _, event := range testEvents {
			eventChan <- event
		}
		close(eventChan)
	}()
	
	// Start consumers
	eventsPerGoroutine := b.N / numGoroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count := 0
			for event := range eventChan {
				_ = event // Process event
				count++
				if count >= eventsPerGoroutine {
					return
				}
			}
		}()
	}
	
	wg.Wait()
}

// Benchmark memory allocation patterns
func BenchmarkMemoryAllocation_EventCreation(b *testing.B) {
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		event := generateEvent(fmt.Sprintf("alloc-%d", i), 1024)
		_ = event
	}
}

func BenchmarkMemoryAllocation_EventWrapping(b *testing.B) {
	baseEvent := generateEvent("base", 1024)
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// Simple allocation test
		event := generateEvent(fmt.Sprintf("wrap-%d", i), 1024)
		_ = event
		_ = baseEvent
	}
}

func BenchmarkMemoryAllocation_ChannelOperations(b *testing.B) {
	eventChan := make(chan events.Event, 1000)
	timestamp := time.Now().UnixMilli()
	event := &BackpressureMockCoreEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeCustom,
			TimestampMs: &timestamp,
		},
		id: "channel-test",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		eventChan <- event
		<-eventChan
	}
}

// Benchmark error conditions
func BenchmarkError_NotConnected(b *testing.B) {
	transport := NewBenchmarkMockTransport(100)
	event := generateEvent("error", 1024)
	ctx := context.Background()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		err := transport.Send(ctx, event)
		if err != ErrNotConnected {
			b.Fatalf("Expected ErrNotConnected, got %v", err)
		}
	}
}

func BenchmarkError_ContextCanceled(b *testing.B) {
	transport := NewBenchmarkMockTransport(100)
	event := generateEvent("canceled", 1024)
	
	if err := transport.Connect(context.Background()); err != nil {
		b.Fatal(err)
	}
	defer transport.Close(context.Background())
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		err := transport.Send(ctx, event)
		_ = err // Error expected
	}
}

// Benchmark backpressure scenarios
func BenchmarkBackpressure_DropOldest(b *testing.B) {
	benchmarkBackpressure(b, BackpressureDropOldest, 10)
}

func BenchmarkBackpressure_DropNewest(b *testing.B) {
	benchmarkBackpressure(b, BackpressureDropNewest, 10)
}

func BenchmarkBackpressure_Block(b *testing.B) {
	benchmarkBackpressure(b, BackpressureBlock, 10)
}

func benchmarkBackpressure(b *testing.B, strategy BackpressureStrategy, bufferSize int) {
	config := BackpressureConfig{
		Strategy:      strategy,
		BufferSize:    bufferSize,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  1 * time.Millisecond, // Very short timeout to avoid deadlock
		EnableMetrics: false, // Disable metrics for pure performance
	}
	
	handler := NewBackpressureHandler(config)
	defer handler.Stop()
	
	timestamp := time.Now().UnixMilli()
	event := &BackpressureMockCoreEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeCustom,
			TimestampMs: &timestamp,
		},
		id: "backpressure-test",
	}
	
	// Start a consumer to drain the channel for blocking strategies
	if strategy == BackpressureBlock || strategy == BackpressureBlockWithTimeout {
		go func() {
			for {
				select {
				case <-handler.EventChan():
					// Drain events
				case <-time.After(10 * time.Millisecond):
					return
				}
			}
		}()
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		handler.SendEvent(event)
	}
}

// Benchmark with and without features
func BenchmarkWithoutLogging(b *testing.B) {
	benchmarkWithFeatures(b, false, false)
}

func BenchmarkWithLogging(b *testing.B) {
	benchmarkWithFeatures(b, true, false)
}

func BenchmarkWithoutValidation(b *testing.B) {
	benchmarkWithFeatures(b, false, false)
}

func BenchmarkWithValidation(b *testing.B) {
	benchmarkWithFeatures(b, false, true)
}

func BenchmarkWithAllFeatures(b *testing.B) {
	benchmarkWithFeatures(b, true, true)
}

func benchmarkWithFeatures(b *testing.B, enableLogging, enableValidation bool) {
	transport := NewBenchmarkMockTransport(1000)
	transport.enableMetrics = true
	
	// Simulate logging overhead
	if enableLogging {
		transport.sendDelay = 1 * time.Microsecond
	}
	
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		b.Fatal(err)
	}
	defer transport.Close(ctx)
	
	event := generateEvent("features", 1024)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// Simulate validation overhead
		if enableValidation {
			if event.ID() == "" || event.Type() == "" {
				b.Fatal("Invalid event")
			}
		}
		
		if err := transport.Send(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark comparative scenarios
func BenchmarkComparative_SimpleManager(b *testing.B) {
	manager := NewSimpleManager()
	transport := NewBenchmarkMockTransport(1000)
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		b.Fatal(err)
	}
	defer manager.Stop(ctx)
	
	event := generateEvent("simple", 1024)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		if err := manager.Send(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkComparative_DirectTransport(b *testing.B) {
	transport := NewBenchmarkMockTransport(1000)
	
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		b.Fatal(err)
	}
	defer transport.Close(ctx)
	
	event := generateEvent("direct", 1024)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		if err := transport.Send(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark memory usage
func BenchmarkMemoryUsageTransport(b *testing.B) {
	transport := NewBenchmarkMockTransport(1000)
	ctx := context.Background()
	transport.Connect(ctx)
	defer transport.Close(ctx)
	
	event := generateEvent("memory", 1024)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		transport.Send(ctx, event)
	}
}

// Example benchmark usage and results formatting
func ExampleBenchmark() {
	// Run specific benchmark:
	// go test -bench=BenchmarkSend_SmallPayload -benchmem
	
	// Run all benchmarks:
	// go test -bench=. -benchmem
	
	// Run with CPU profiling:
	// go test -bench=BenchmarkSend_SmallPayload -cpuprofile=cpu.prof
	
	// Run with memory profiling:
	// go test -bench=BenchmarkSend_SmallPayload -memprofile=mem.prof
	
	// Compare benchmarks:
	// go test -bench=BenchmarkSend -benchmem -count=5 > old.txt
	// go test -bench=BenchmarkSend -benchmem -count=5 > new.txt
	// benchcmp old.txt new.txt
	
	// Run benchmarks by category:
	// go test -bench="BenchmarkSend_" -benchmem          # Send operations
	// go test -bench="BenchmarkConcurrent" -benchmem      # Concurrent operations
	// go test -bench="BenchmarkBackpressure" -benchmem    # Backpressure scenarios
	// go test -bench="BenchmarkManagerLifecycle" -benchmem # Manager operations
	// go test -bench="BenchmarkError" -benchmem           # Error conditions
	// go test -bench="BenchmarkWith" -benchmem            # Feature comparisons
}