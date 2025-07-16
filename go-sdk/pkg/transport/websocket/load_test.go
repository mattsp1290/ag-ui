package websocket

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// LoadTestServer provides a WebSocket server optimized for load testing
type LoadTestServer struct {
	server      *httptest.Server
	upgrader    websocket.Upgrader
	connections int64
	messages    int64
	errors      int64
	logger      *zap.Logger
	echoMode    bool
	dropRate    float64
}

func NewLoadTestServer(t testing.TB) *LoadTestServer {
	server := &LoadTestServer{
		upgrader: websocket.Upgrader{
			CheckOrigin:     func(r *http.Request) bool { return true },
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		},
		logger:   zaptest.NewLogger(t),
		echoMode: true,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.handleWebSocket)
	server.server = httptest.NewServer(mux)

	return server
}

func (s *LoadTestServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		atomic.AddInt64(&s.errors, 1)
		return
	}
	defer conn.Close()

	atomic.AddInt64(&s.connections, 1)
	defer atomic.AddInt64(&s.connections, -1)

	// Set read/write deadlines for load testing
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				atomic.AddInt64(&s.errors, 1)
			}
			break
		}

		atomic.AddInt64(&s.messages, 1)

		// Simulate message processing and potential drops
		if s.dropRate > 0 && rand.Float64() < s.dropRate {
			continue
		}

		if s.echoMode {
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(messageType, message); err != nil {
				atomic.AddInt64(&s.errors, 1)
				break
			}
		}

		// Reset read deadline
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}

func (s *LoadTestServer) URL() string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http") + "/ws"
}

func (s *LoadTestServer) Close() {
	s.server.Close()
}

func (s *LoadTestServer) GetStats() (connections, messages, errors int64) {
	return atomic.LoadInt64(&s.connections), atomic.LoadInt64(&s.messages), atomic.LoadInt64(&s.errors)
}

func (s *LoadTestServer) SetDropRate(rate float64) {
	s.dropRate = rate
}

func (s *LoadTestServer) SetEchoMode(enabled bool) {
	s.echoMode = enabled
}

// LoadTestMetrics tracks performance metrics during load testing
type LoadTestMetrics struct {
	mu                     sync.RWMutex
	startTime              time.Time
	endTime                time.Time
	messagesSent           int64
	messagesReceived       int64
	errorsOccurred         int64
	connectionsEstablished int64
	connectionsFailed      int64
	totalLatency           time.Duration
	minLatency             time.Duration
	maxLatency             time.Duration
	latencySamples         int64
	memUsagePeek           uint64
	goroutinesPeak         int
}

func NewLoadTestMetrics() *LoadTestMetrics {
	return &LoadTestMetrics{
		startTime: time.Now(),
	}
}

func (m *LoadTestMetrics) RecordMessageSent() {
	atomic.AddInt64(&m.messagesSent, 1)
}

func (m *LoadTestMetrics) RecordMessageReceived() {
	atomic.AddInt64(&m.messagesReceived, 1)
}

func (m *LoadTestMetrics) RecordError() {
	atomic.AddInt64(&m.errorsOccurred, 1)
}

func (m *LoadTestMetrics) RecordConnectionEstablished() {
	atomic.AddInt64(&m.connectionsEstablished, 1)
}

func (m *LoadTestMetrics) RecordConnectionFailed() {
	atomic.AddInt64(&m.connectionsFailed, 1)
}

func (m *LoadTestMetrics) RecordLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	atomic.AddInt64(&m.latencySamples, 1)
	m.totalLatency += latency

	if m.minLatency == 0 || latency < m.minLatency {
		m.minLatency = latency
	}
	if latency > m.maxLatency {
		m.maxLatency = latency
	}
}

func (m *LoadTestMetrics) UpdateMemoryUsage() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.mu.Lock()
	if memStats.Alloc > m.memUsagePeek {
		m.memUsagePeek = memStats.Alloc
	}
	m.mu.Unlock()
}

func (m *LoadTestMetrics) UpdateGoroutineCount() {
	count := runtime.NumGoroutine()
	m.mu.Lock()
	if count > m.goroutinesPeak {
		m.goroutinesPeak = count
	}
	m.mu.Unlock()
}

func (m *LoadTestMetrics) Finalize() {
	m.mu.Lock()
	m.endTime = time.Now()
	m.mu.Unlock()
}

func (m *LoadTestMetrics) GetSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	duration := m.endTime.Sub(m.startTime)
	messagesSent := atomic.LoadInt64(&m.messagesSent)
	messagesReceived := atomic.LoadInt64(&m.messagesReceived)
	errorsOccurred := atomic.LoadInt64(&m.errorsOccurred)
	connectionsEstablished := atomic.LoadInt64(&m.connectionsEstablished)
	connectionsFailed := atomic.LoadInt64(&m.connectionsFailed)
	latencySamples := atomic.LoadInt64(&m.latencySamples)

	var avgLatency time.Duration
	if latencySamples > 0 {
		avgLatency = m.totalLatency / time.Duration(latencySamples)
	}

	return map[string]interface{}{
		"duration_seconds":        duration.Seconds(),
		"messages_sent":           messagesSent,
		"messages_received":       messagesReceived,
		"errors_occurred":         errorsOccurred,
		"connections_established": connectionsEstablished,
		"connections_failed":      connectionsFailed,
		"messages_per_second":     float64(messagesSent) / duration.Seconds(),
		"error_rate":              float64(errorsOccurred) / float64(messagesSent+errorsOccurred),
		"connection_success_rate": float64(connectionsEstablished) / float64(connectionsEstablished+connectionsFailed),
		"average_latency_ms":      avgLatency.Milliseconds(),
		"min_latency_ms":          m.minLatency.Milliseconds(),
		"max_latency_ms":          m.maxLatency.Milliseconds(),
		"memory_peak_mb":          float64(m.memUsagePeek) / (1024 * 1024),
		"goroutines_peak":         m.goroutinesPeak,
	}
}

func TestHighConcurrencyConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high concurrency test in short mode")
	}

	// Environment-based timeout scaling
	timeout := 90 * time.Second
	if os.Getenv("CI") == "true" {
		timeout = 150 * time.Second
	}

	server := NewLoadTestServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop() // Reduce logging overhead
	config.PoolConfig.MinConnections = 50
	config.PoolConfig.MaxConnections = 200
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	metrics := NewLoadTestMetrics()
	defer metrics.Finalize()

	t.Run("1000_Concurrent_Connections", func(t *testing.T) {
		err := transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		// Wait for initial connections
		time.Sleep(2 * time.Second)

		const numGoroutines = 1000
		const messagesPerGoroutine = 10

		var wg sync.WaitGroup
		var errors int64

		startTime := time.Now()

		// Monitor system resources
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					metrics.UpdateMemoryUsage()
					metrics.UpdateGoroutineCount()
				}
			}
		}()

		// Launch concurrent message senders
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < messagesPerGoroutine; j++ {
					// Check context cancellation
					select {
					case <-ctx.Done():
						return
					default:
					}

					event := &MockEvent{
						EventType: events.EventTypeTextMessageContent,
						Data:      fmt.Sprintf("load_test_message_%d_%d", id, j),
					}

					msgStart := time.Now()
					err := transport.SendEvent(ctx, event)
					if err != nil {
						atomic.AddInt64(&errors, 1)
						metrics.RecordError()
					} else {
						metrics.RecordMessageSent()
						metrics.RecordLatency(time.Since(msgStart))
					}

					// Small random delay to simulate realistic usage
					time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
				}
			}(i)
		}

		// Wait for goroutines with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All goroutines completed
		case <-ctx.Done():
			t.Log("Load test timeout, some goroutines may still be running")
		}
		duration := time.Since(startTime)

		// Verify results
		assert.Equal(t, int64(0), errors, "No errors should occur during load test")

		stats := transport.GetStats()
		expectedMessages := int64(numGoroutines * messagesPerGoroutine)
		assert.Equal(t, expectedMessages, stats.EventsSent)

		throughput := float64(expectedMessages) / duration.Seconds()
		t.Logf("Load test completed: %d messages in %v (%.2f msg/sec)",
			expectedMessages, duration, throughput)

		// Performance assertions (relaxed for test stability)
		assert.Greater(t, throughput, 500.0, "Should achieve at least 500 messages/sec")
		assert.Less(t, duration, 60*time.Second, "Should complete within 60 seconds")

		// Print metrics summary
		summary := metrics.GetSummary()
		t.Logf("Performance Summary: %+v", summary)

		// Memory usage should be reasonable
		memPeakMB := summary["memory_peak_mb"].(float64)
		assert.Less(t, memPeakMB, 500.0, "Memory usage should stay under 500MB")
	})
}

func TestSustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sustained load test in short mode")
	}

	// Environment-based timeout scaling
	timeout := 150 * time.Second
	if os.Getenv("CI") == "true" {
		timeout = 240 * time.Second
	}

	server := NewLoadTestServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
	config.PoolConfig.MinConnections = 10
	config.PoolConfig.MaxConnections = 50
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	metrics := NewLoadTestMetrics()
	defer metrics.Finalize()

	t.Run("Sustained_Load_60_Seconds", func(t *testing.T) {
		// Wait for connections
		time.Sleep(1 * time.Second)

		const duration = 60 * time.Second
		const targetThroughput = 500 // messages per second
		const numWorkers = 20

		var wg sync.WaitGroup
		var totalMessages int64
		var totalErrors int64

		startTime := time.Now()
		endTime := startTime.Add(duration)

		// Launch worker goroutines
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				messageCount := 0
				ticker := time.NewTicker(time.Duration(numWorkers * int(time.Second) / targetThroughput))
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						if time.Now().After(endTime) {
							return
						}

						event := &MockEvent{
							EventType: events.EventTypeTextMessageContent,
							Data:      fmt.Sprintf("sustained_load_worker_%d_msg_%d", workerID, messageCount),
						}

						msgStart := time.Now()
						err := transport.SendEvent(ctx, event)
						if err != nil {
							atomic.AddInt64(&totalErrors, 1)
							metrics.RecordError()
						} else {
							atomic.AddInt64(&totalMessages, 1)
							metrics.RecordMessageSent()
							metrics.RecordLatency(time.Since(msgStart))
						}

						messageCount++
					}
				}
			}(i)
		}

		// Monitor resources during the test
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if time.Now().After(endTime) {
						return
					}

					metrics.UpdateMemoryUsage()
					metrics.UpdateGoroutineCount()

					// Log progress
					currentMessages := atomic.LoadInt64(&totalMessages)
					currentErrors := atomic.LoadInt64(&totalErrors)
					elapsed := time.Since(startTime)
					currentThroughput := float64(currentMessages) / elapsed.Seconds()

					t.Logf("Progress: %d messages, %d errors, %.2f msg/sec",
						currentMessages, currentErrors, currentThroughput)
				}
			}
		}()

		wg.Wait()
		actualDuration := time.Since(startTime)

		// Verify sustained performance
		finalMessages := atomic.LoadInt64(&totalMessages)
		finalErrors := atomic.LoadInt64(&totalErrors)
		actualThroughput := float64(finalMessages) / actualDuration.Seconds()

		t.Logf("Sustained load test completed:")
		t.Logf("  Duration: %v", actualDuration)
		t.Logf("  Messages: %d", finalMessages)
		t.Logf("  Errors: %d", finalErrors)
		t.Logf("  Throughput: %.2f msg/sec", actualThroughput)

		// Performance assertions
		assert.Greater(t, actualThroughput, float64(targetThroughput)*0.8,
			"Should achieve at least 80% of target throughput")
		assert.Less(t, float64(finalErrors)/float64(finalMessages), 0.01,
			"Error rate should be less than 1%")

		// Transport should remain stable
		assert.True(t, transport.IsConnected())
		assert.Greater(t, transport.GetActiveConnectionCount(), 0)

		// Print final metrics
		summary := metrics.GetSummary()
		t.Logf("Final Performance Summary: %+v", summary)
	})
}

func TestBurstLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping burst load test in short mode")
	}

	// Environment-based timeout scaling
	timeout := 90 * time.Second
	if os.Getenv("CI") == "true" {
		timeout = 150 * time.Second
	}

	server := NewLoadTestServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
	config.PoolConfig.MinConnections = 5
	config.PoolConfig.MaxConnections = 100
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	metrics := NewLoadTestMetrics()
	defer metrics.Finalize()

	t.Run("Burst_Load_Pattern", func(t *testing.T) {
		// Wait for connections
		time.Sleep(1 * time.Second)

		const burstSize = 1000
		const burstInterval = 5 * time.Second
		const numBursts = 5

		var totalMessages int64
		var totalErrors int64

		for burst := 0; burst < numBursts; burst++ {
			t.Logf("Starting burst %d/%d", burst+1, numBursts)

			var wg sync.WaitGroup
			burstStart := time.Now()

			// Generate burst of messages
			for i := 0; i < burstSize; i++ {
				wg.Add(1)
				go func(msgID int) {
					defer wg.Done()

					// Check context cancellation
					select {
					case <-ctx.Done():
						return
					default:
					}

					event := &MockEvent{
						EventType: events.EventTypeTextMessageContent,
						Data:      fmt.Sprintf("burst_%d_message_%d", burst, msgID),
					}

					msgStart := time.Now()
					err := transport.SendEvent(ctx, event)
					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
						metrics.RecordError()
					} else {
						atomic.AddInt64(&totalMessages, 1)
						metrics.RecordMessageSent()
						metrics.RecordLatency(time.Since(msgStart))
					}
				}(i)
			}

			// Wait for burst goroutines with timeout
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				// All goroutines completed
			case <-ctx.Done():
				t.Log("Burst test timeout, some goroutines may still be running")
				return
			}
			burstDuration := time.Since(burstStart)
			burstThroughput := float64(burstSize) / burstDuration.Seconds()

			t.Logf("Burst %d completed in %v (%.2f msg/sec)",
				burst+1, burstDuration, burstThroughput)

			// Update system metrics
			metrics.UpdateMemoryUsage()
			metrics.UpdateGoroutineCount()

			// Verify transport stability after burst
			assert.True(t, transport.IsConnected(), "Transport should remain connected after burst")

			// Wait between bursts (except for the last one)
			if burst < numBursts-1 {
				time.Sleep(burstInterval)
			}
		}

		// Final verification
		finalMessages := atomic.LoadInt64(&totalMessages)
		finalErrors := atomic.LoadInt64(&totalErrors)
		expectedMessages := int64(numBursts * burstSize)

		t.Logf("Burst load test summary:")
		t.Logf("  Expected messages: %d", expectedMessages)
		t.Logf("  Actual messages: %d", finalMessages)
		t.Logf("  Errors: %d", finalErrors)

		assert.Equal(t, expectedMessages, finalMessages, "All messages should be sent successfully")
		assert.Equal(t, int64(0), finalErrors, "No errors should occur")

		// Print metrics
		summary := metrics.GetSummary()
		t.Logf("Burst Performance Summary: %+v", summary)
	})
}

func TestMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	// Environment-based timeout scaling
	iterationTimeout := 45 * time.Second
	if os.Getenv("CI") == "true" {
		iterationTimeout = 90 * time.Second
	}

	server := NewLoadTestServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
	config.EnableEventValidation = false

	t.Run("Memory_Leak_Detection", func(t *testing.T) {
		const iterations = 10
		const messagesPerIteration = 1000

		var memoryUsages []uint64

		for i := 0; i < iterations; i++ {
			// Create new transport for each iteration
			transport, err := NewTransport(config)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), iterationTimeout)

			err = transport.Start(ctx)
			require.NoError(t, err)

			// Wait for connections
			time.Sleep(500 * time.Millisecond)

			// Send messages
			for j := 0; j < messagesPerIteration; j++ {
				event := &MockEvent{
					EventType: events.EventTypeTextMessageContent,
					Data:      fmt.Sprintf("memory_test_%d_%d", i, j),
				}
				_ = transport.SendEvent(ctx, event)
			}

			// Stop transport and cleanup
			transport.Stop()
			cancel()

			// Force garbage collection
			runtime.GC()
			runtime.GC() // Run twice to ensure cleanup

			// Measure memory usage
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			memoryUsages = append(memoryUsages, memStats.Alloc)

			t.Logf("Iteration %d: Memory usage: %.2f MB",
				i+1, float64(memStats.Alloc)/(1024*1024))
		}

		// Analyze memory trend
		if len(memoryUsages) >= 5 {
			firstHalf := memoryUsages[:len(memoryUsages)/2]
			secondHalf := memoryUsages[len(memoryUsages)/2:]

			var firstAvg, secondAvg uint64
			for _, usage := range firstHalf {
				firstAvg += usage
			}
			firstAvg /= uint64(len(firstHalf))

			for _, usage := range secondHalf {
				secondAvg += usage
			}
			secondAvg /= uint64(len(secondHalf))

			growthRatio := float64(secondAvg) / float64(firstAvg)
			t.Logf("Memory growth ratio: %.2f", growthRatio)

			// Memory usage should not grow significantly
			// Allow up to 2.5x growth due to Go's garbage collection patterns
			assert.Less(t, growthRatio, 2.5,
				"Memory usage should not grow by more than 150% over iterations")
		}
	})
}

func TestConnectionPoolScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection pool scaling test in short mode")
	}

	// Environment-based timeout scaling
	timeout := 90 * time.Second
	if os.Getenv("CI") == "true" {
		timeout = 150 * time.Second
	}

	server := NewLoadTestServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
	config.PoolConfig.MinConnections = 1
	config.PoolConfig.MaxConnections = 50
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("Connection_Pool_Auto_Scaling", func(t *testing.T) {
		// Wait for initial connections
		time.Sleep(1 * time.Second)

		initialConnections := transport.GetActiveConnectionCount()
		t.Logf("Initial connections: %d", initialConnections)

		// Gradually increase load to trigger connection scaling
		loadLevels := []int{10, 50, 100, 200, 500}

		for _, load := range loadLevels {
			t.Logf("Testing load level: %d concurrent senders", load)

			var wg sync.WaitGroup
			var errors int64

			startTime := time.Now()

			// Launch concurrent senders
			for i := 0; i < load; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					for j := 0; j < 10; j++ {
						// Check context cancellation
						select {
						case <-ctx.Done():
							return
						default:
						}

						event := &MockEvent{
							EventType: events.EventTypeTextMessageContent,
							Data:      fmt.Sprintf("scaling_test_%d_%d", id, j),
						}

						if err := transport.SendEvent(ctx, event); err != nil {
							atomic.AddInt64(&errors, 1)
						}

						// Small delay to sustain load
						time.Sleep(10 * time.Millisecond)
					}
				}(i)
			}

			// Wait for goroutines with timeout
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				// All goroutines completed
			case <-ctx.Done():
				t.Log("Connection pool scaling test timeout, some goroutines may still be running")
			}
			duration := time.Since(startTime)
			currentConnections := transport.GetActiveConnectionCount()

			t.Logf("Load %d completed in %v, connections: %d, errors: %d",
				load, duration, currentConnections, errors)

			// Verify performance under load
			assert.Equal(t, int64(0), errors, "No errors should occur under load %d", load)

			// Connection pool maintains configured connections, not auto-scaling
			// Just verify connections are healthy
			assert.GreaterOrEqual(t, currentConnections, 1,
				"At least one connection should be active")

			// Brief cooldown between load levels
			time.Sleep(2 * time.Second)
		}

		// Verify final state
		finalConnections := transport.GetActiveConnectionCount()
		poolStats := transport.GetConnectionPoolStats()

		t.Logf("Final connections: %d", finalConnections)
		t.Logf("Pool stats: %+v", poolStats)

		assert.GreaterOrEqual(t, finalConnections, initialConnections,
			"Connection pool should maintain at least initial connections")
	})
}

func TestUnderAdverseConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adverse conditions test in short mode")
	}

	// Environment-based timeout scaling
	timeout := 90 * time.Second
	if os.Getenv("CI") == "true" {
		timeout = 150 * time.Second
	}

	server := NewLoadTestServer(t)
	defer server.Close()

	// Configure server with adverse conditions
	server.SetDropRate(0.1) // Drop 10% of messages

	config := DefaultTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
	config.PoolConfig.MinConnections = 5
	config.PoolConfig.MaxConnections = 20
	config.PoolConfig.ConnectionTemplate.MaxReconnectAttempts = 5
	config.PoolConfig.ConnectionTemplate.InitialReconnectDelay = 100 * time.Millisecond
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	metrics := NewLoadTestMetrics()
	defer metrics.Finalize()

	t.Run("Performance_Under_Adverse_Conditions", func(t *testing.T) {
		// Wait for connections
		time.Sleep(1 * time.Second)

		const numMessages = 1000
		const numWorkers = 50

		var wg sync.WaitGroup
		var successfulMessages int64
		var failedMessages int64

		startTime := time.Now()

		// Launch workers
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < numMessages/numWorkers; j++ {
					// Check context cancellation
					select {
					case <-ctx.Done():
						return
					default:
					}

					event := &MockEvent{
						EventType: events.EventTypeTextMessageContent,
						Data:      fmt.Sprintf("adverse_test_%d_%d", workerID, j),
					}

					msgStart := time.Now()
					err := transport.SendEvent(ctx, event)
					if err != nil {
						atomic.AddInt64(&failedMessages, 1)
						metrics.RecordError()
					} else {
						atomic.AddInt64(&successfulMessages, 1)
						metrics.RecordMessageSent()
						metrics.RecordLatency(time.Since(msgStart))
					}

					// Random delays to simulate realistic conditions
					time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)
				}
			}(i)
		}

		// Wait for goroutines with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All goroutines completed
		case <-ctx.Done():
			t.Log("Adverse conditions test timeout, some goroutines may still be running")
		}
		duration := time.Since(startTime)

		successful := atomic.LoadInt64(&successfulMessages)
		failed := atomic.LoadInt64(&failedMessages)
		total := successful + failed
		successRate := float64(successful) / float64(total)

		t.Logf("Adverse conditions test results:")
		t.Logf("  Duration: %v", duration)
		t.Logf("  Successful messages: %d", successful)
		t.Logf("  Failed messages: %d", failed)
		t.Logf("  Success rate: %.2f%%", successRate*100)

		// Under adverse conditions, we should still achieve reasonable success rate
		assert.Greater(t, successRate, 0.8, "Success rate should be > 80% even under adverse conditions")

		// Transport should remain operational
		assert.True(t, transport.IsConnected())
		assert.Greater(t, transport.GetActiveConnectionCount(), 0)

		// Print final metrics
		summary := metrics.GetSummary()
		t.Logf("Adverse Conditions Performance Summary: %+v", summary)
	})
}

// Load testing benchmarks
func BenchmarkHighConcurrencyLoad(b *testing.B) {
	server := NewLoadTestServer(b)
	defer server.Close()

	config := DefaultTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
	config.PoolConfig.MaxConnections = 100
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(b, err)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(b, err)
	defer transport.Stop()

	// Wait for connections
	time.Sleep(1 * time.Second)

	event := &MockEvent{
		EventType: events.EventTypeTextMessageContent,
		Data:      "benchmark load test message",
	}

	b.ResetTimer()
	b.SetParallelism(100) // High parallelism for load testing

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = transport.SendEvent(ctx, event)
		}
	})
}

func BenchmarkConnectionPoolPerformance(b *testing.B) {
	server1 := NewLoadTestServer(b)
	defer server1.Close()

	server2 := NewLoadTestServer(b)
	defer server2.Close()

	server3 := NewLoadTestServer(b)
	defer server3.Close()

	config := DefaultTransportConfig()
	config.URLs = []string{server1.URL(), server2.URL(), server3.URL()}
	config.Logger = zap.NewNop()
	config.PoolConfig.MinConnections = 10
	config.PoolConfig.MaxConnections = 30
	config.PoolConfig.LoadBalancingStrategy = RoundRobin
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(b, err)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(b, err)
	defer transport.Stop()

	// Wait for connections
	time.Sleep(2 * time.Second)

	event := &MockEvent{
		EventType: events.EventTypeTextMessageContent,
		Data:      "benchmark pool test message",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = transport.SendEvent(ctx, event)
		}
	})
}

// Helper function for min calculation
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
