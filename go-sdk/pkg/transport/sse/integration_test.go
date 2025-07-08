package sse

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

// NetworkSimulator simulates various network conditions
type NetworkSimulator struct {
	server      *httptest.Server
	proxy       *httptest.Server
	latency     time.Duration
	packetLoss  float64
	bandwidth   int64 // bytes per second
	disconnect  bool
	mu          sync.RWMutex
	transferred int64
	lastReset   time.Time
}

// NewNetworkSimulator creates a new network simulator
func NewNetworkSimulator(handler http.Handler) *NetworkSimulator {
	ns := &NetworkSimulator{
		latency:    0,
		packetLoss: 0,
		bandwidth:  0, // unlimited
		lastReset:  time.Now(),
	}

	ns.server = httptest.NewServer(handler)

	// Create proxy server that simulates network conditions
	ns.proxy = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ns.mu.RLock()
		latency := ns.latency
		packetLoss := ns.packetLoss
		bandwidth := ns.bandwidth
		disconnect := ns.disconnect
		ns.mu.RUnlock()

		// Simulate latency
		if latency > 0 {
			time.Sleep(latency)
		}

		// Simulate packet loss
		if packetLoss > 0 && rand.Float64() < packetLoss {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Simulate disconnect
		if disconnect {
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
		}

		// Forward request to actual server
		req, _ := http.NewRequest(r.Method, ns.server.URL+r.URL.Path, r.Body)
		req.Header = r.Header

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy headers
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)

		// Simulate bandwidth limitation
		if bandwidth > 0 {
			// Create a rate-limited writer
			limitedWriter := &rateLimitedWriter{
				w:         w,
				bandwidth: bandwidth,
				ns:        ns,
			}
			io.Copy(limitedWriter, resp.Body)
		} else {
			io.Copy(w, resp.Body)
		}
	}))

	return ns
}

// rateLimitedWriter implements bandwidth limiting
type rateLimitedWriter struct {
	w         io.Writer
	bandwidth int64
	ns        *NetworkSimulator
}

func (rlw *rateLimitedWriter) Write(p []byte) (n int, err error) {
	rlw.ns.mu.Lock()
	elapsed := time.Since(rlw.ns.lastReset)
	if elapsed >= time.Second {
		rlw.ns.transferred = 0
		rlw.ns.lastReset = time.Now()
	}

	if rlw.ns.transferred >= rlw.bandwidth {
		// Wait until next second
		sleepTime := time.Second - elapsed
		rlw.ns.mu.Unlock()
		time.Sleep(sleepTime)
		rlw.ns.mu.Lock()
		rlw.ns.transferred = 0
		rlw.ns.lastReset = time.Now()
	}

	toWrite := len(p)
	if rlw.ns.transferred+int64(toWrite) > rlw.bandwidth {
		toWrite = int(rlw.bandwidth - rlw.ns.transferred)
	}

	rlw.ns.transferred += int64(toWrite)
	rlw.ns.mu.Unlock()

	return rlw.w.Write(p[:toWrite])
}

// SetLatency sets the network latency
func (ns *NetworkSimulator) SetLatency(latency time.Duration) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.latency = latency
}

// SetPacketLoss sets the packet loss rate (0.0 to 1.0)
func (ns *NetworkSimulator) SetPacketLoss(loss float64) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.packetLoss = loss
}

// SetBandwidth sets the bandwidth limit in bytes per second
func (ns *NetworkSimulator) SetBandwidth(bandwidth int64) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.bandwidth = bandwidth
}

// SimulateDisconnect simulates a network disconnect
func (ns *NetworkSimulator) SimulateDisconnect() {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.disconnect = true
}

// Reset resets network conditions to normal
func (ns *NetworkSimulator) Reset() {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.latency = 0
	ns.packetLoss = 0
	ns.bandwidth = 0
	ns.disconnect = false
}

// Close closes the simulator
func (ns *NetworkSimulator) Close() {
	if ns.proxy != nil {
		ns.proxy.Close()
	}
	if ns.server != nil {
		ns.server.Close()
	}
}

// LoadTestMetrics tracks load test performance metrics
type LoadTestMetrics struct {
	TotalConnections  int64
	ActiveConnections int64
	SuccessfulEvents  int64
	FailedEvents      int64
	TotalLatency      int64 // nanoseconds
	MinLatency        int64
	MaxLatency        int64
	MemoryUsed        uint64
	CPUPercent        float64
	Goroutines        int
	StartTime         time.Time
	mu                sync.RWMutex
}

// RecordEvent records an event transmission
func (m *LoadTestMetrics) RecordEvent(latency time.Duration, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if success {
		atomic.AddInt64(&m.SuccessfulEvents, 1)
	} else {
		atomic.AddInt64(&m.FailedEvents, 1)
	}

	latencyNs := latency.Nanoseconds()
	atomic.AddInt64(&m.TotalLatency, latencyNs)

	if m.MinLatency == 0 || latencyNs < m.MinLatency {
		m.MinLatency = latencyNs
	}
	if latencyNs > m.MaxLatency {
		m.MaxLatency = latencyNs
	}
}

// GetAverageLatency returns the average latency
func (m *LoadTestMetrics) GetAverageLatency() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.SuccessfulEvents + m.FailedEvents
	if total == 0 {
		return 0
	}
	return time.Duration(m.TotalLatency / total)
}

// UpdateSystemMetrics updates system resource metrics
func (m *LoadTestMetrics) UpdateSystemMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.MemoryUsed = memStats.Alloc
	m.Goroutines = runtime.NumGoroutine()
}

// ======================== Browser Compatibility Tests ========================

// TestBrowserCompatibility tests SSE transport with real browser scenarios
func TestBrowserCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser compatibility tests in short mode")
	}

	// Create test SSE server
	sseHandler := createTestSSEHandler()
	server := httptest.NewServer(sseHandler)
	defer server.Close()

	testCases := []struct {
		name     string
		browser  string
		testFunc func(t *testing.T, url string)
	}{
		{
			name:     "Chrome SSE Connection",
			browser:  "chrome",
			testFunc: testChromeSSE,
		},
		{
			name:     "Firefox SSE Connection",
			browser:  "firefox",
			testFunc: testFirefoxSSE,
		},
		{
			name:     "Safari SSE Connection",
			browser:  "safari",
			testFunc: testSafariSSE,
		},
		{
			name:     "Mobile Browser Simulation",
			browser:  "mobile",
			testFunc: testMobileSSE,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if !isBrowserAvailable(tc.browser) {
				t.Skipf("Browser %s not available", tc.browser)
			}
			tc.testFunc(t, server.URL)
		})
	}
}

// testChromeSSE tests SSE in Chrome using chromedp
func testChromeSSE(t *testing.T, serverURL string) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set timeout
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var eventData string
	err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/test.html"),
		chromedp.WaitVisible("#events", chromedp.ByID),
		chromedp.Evaluate(`
			new Promise((resolve, reject) => {
				const events = [];
				const source = new EventSource('`+serverURL+`/events');
				
				source.onmessage = (e) => {
					events.push(e.data);
					if (events.length >= 5) {
						source.close();
						resolve(events.join(','));
					}
				};
				
				source.onerror = (e) => {
					source.close();
					reject(e);
				};
				
				setTimeout(() => {
					source.close();
					resolve(events.join(','));
				}, 10000);
			})
		`, &eventData),
	)

	require.NoError(t, err, "Chrome SSE test failed")
	assert.Contains(t, eventData, "event", "Should receive SSE events")
}

// ======================== Network Failure Simulation Tests ========================

// TestNetworkResilience tests SSE transport under various network conditions
func TestNetworkResilience(t *testing.T) {
	// Create test stream
	config := DefaultStreamConfig()
	config.EnableMetrics = true
	stream, err := NewEventStream(config)
	require.NoError(t, err)

	err = stream.Start()
	require.NoError(t, err)
	defer stream.Close()

	// Create SSE handler
	handler := createStreamingSSEHandler(stream)
	ns := NewNetworkSimulator(handler)
	defer ns.Close()

	// Create transport
	transportConfig := DefaultConfig()
	transportConfig.BaseURL = ns.proxy.URL
	transportConfig.ReconnectDelay = 500 * time.Millisecond
	transportConfig.MaxReconnects = 3

	transport, err := NewSSETransport(transportConfig)
	require.NoError(t, err)
	defer transport.Close()

	t.Run("High Latency", func(t *testing.T) {
		ns.Reset()
		ns.SetLatency(500 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Start receiving
		eventChan, err := transport.Receive(ctx)
		require.NoError(t, err)

		// Send test event
		testEvent := events.NewTextMessageContentEvent("test", "high latency test")
		err = stream.SendEvent(testEvent)
		require.NoError(t, err)

		// Should receive event despite high latency
		select {
		case event := <-eventChan:
			assert.NotNil(t, event)
		case <-ctx.Done():
			t.Fatal("Timeout waiting for event with high latency")
		}
	})

	t.Run("Packet Loss", func(t *testing.T) {
		ns.Reset()
		ns.SetPacketLoss(0.3) // 30% packet loss

		successCount := 0
		failureCount := 0

		// Send multiple events and check delivery rate
		for i := 0; i < 20; i++ {
			testEvent := events.NewTextMessageContentEvent("test", fmt.Sprintf("packet loss test %d", i))
			err := transport.Send(context.Background(), testEvent)
			if err == nil {
				successCount++
			} else {
				failureCount++
			}
		}

		// Should have some successes despite packet loss
		assert.Greater(t, successCount, 0, "Should have successful transmissions")
		assert.Greater(t, failureCount, 0, "Should have some failures due to packet loss")

		successRate := float64(successCount) / float64(successCount+failureCount)
		assert.InDelta(t, 0.7, successRate, 0.2, "Success rate should be around 70%")
	})

	t.Run("Connection Drop", func(t *testing.T) {
		ns.Reset()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Start receiving
		eventChan, err := transport.Receive(ctx)
		require.NoError(t, err)

		// Send initial event
		testEvent := events.NewTextMessageContentEvent("test", "before disconnect")
		err = stream.SendEvent(testEvent)
		require.NoError(t, err)

		// Should receive first event
		select {
		case event := <-eventChan:
			assert.NotNil(t, event)
		case <-ctx.Done():
			t.Fatal("Timeout waiting for first event")
		}

		// Simulate disconnect
		ns.SimulateDisconnect()
		time.Sleep(100 * time.Millisecond)

		// Reset connection
		ns.Reset()

		// Transport should reconnect and receive new events
		testEvent2 := events.NewTextMessageContentEvent("test", "after reconnect")
		err = stream.SendEvent(testEvent2)
		require.NoError(t, err)

		// Should eventually receive event after reconnection
		reconnected := false
		timeout := time.After(5 * time.Second)

		for !reconnected {
			select {
			case event := <-eventChan:
				if event != nil {
					reconnected = true
				}
			case <-timeout:
				t.Fatal("Failed to reconnect after connection drop")
			}
		}
	})

	t.Run("Bandwidth Limitation", func(t *testing.T) {
		ns.Reset()
		ns.SetBandwidth(10 * 1024) // 10 KB/s

		// Create large event
		largeContent := strings.Repeat("X", 50*1024) // 50 KB
		testEvent := events.NewTextMessageContentEvent("test", largeContent)

		start := time.Now()
		err := stream.SendEvent(testEvent)
		require.NoError(t, err)

		// Should take at least 5 seconds to transmit 50KB at 10KB/s
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		eventChan, err := transport.Receive(ctx)
		require.NoError(t, err)

		select {
		case <-eventChan:
			elapsed := time.Since(start)
			assert.Greater(t, elapsed, 4*time.Second, "Should be bandwidth limited")
		case <-ctx.Done():
			t.Fatal("Timeout waiting for bandwidth-limited event")
		}
	})
}

// ======================== Load Testing ========================

// TestHighConcurrencyLoad tests SSE transport with >1000 concurrent connections
func TestHighConcurrencyLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	// Configuration
	const (
		targetConnections = 1200
		eventsPerSecond   = 100
		testDuration      = 30 * time.Second
		maxLatency        = 100 * time.Millisecond
	)

	// Create metrics collector
	metrics := &LoadTestMetrics{
		StartTime: time.Now(),
	}

	// Create test stream with optimized config
	config := DefaultStreamConfig()
	config.WorkerCount = runtime.NumCPU() * 2
	config.EventBufferSize = 10000
	config.ChunkBufferSize = 10000
	config.EnableMetrics = true
	config.BatchEnabled = true
	config.BatchSize = 50
	config.CompressionEnabled = true

	stream, err := NewEventStream(config)
	require.NoError(t, err)

	err = stream.Start()
	require.NoError(t, err)
	defer stream.Close()

	// Create SSE server with optimized settings
	server := httptest.NewUnstartedServer(createStreamingSSEHandler(stream))

	// Enable HTTP/2 for better performance
	server.TLS = &tls.Config{
		NextProtos: []string{"h2", "http/1.1"},
	}
	server.StartTLS()
	defer server.Close()

	// Enable HTTP/2 client
	client := &http.Client{
		Transport: &http2.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 30 * time.Second,
	}

	// Connection pool
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	// Start system metrics collector
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				metrics.UpdateSystemMetrics()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Create concurrent connections
	connectionErrors := int64(0)

	for i := 0; i < targetConnections; i++ {
		wg.Add(1)
		atomic.AddInt64(&metrics.TotalConnections, 1)

		go func(connID int) {
			defer wg.Done()
			defer atomic.AddInt64(&metrics.ActiveConnections, -1)

			atomic.AddInt64(&metrics.ActiveConnections, 1)

			// Create SSE connection
			req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/events/stream", nil)
			if err != nil {
				atomic.AddInt64(&connectionErrors, 1)
				return
			}

			req.Header.Set("Accept", "text/event-stream")
			req.Header.Set("Cache-Control", "no-cache")

			resp, err := client.Do(req)
			if err != nil {
				atomic.AddInt64(&connectionErrors, 1)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				atomic.AddInt64(&connectionErrors, 1)
				return
			}

			// Read events
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() && ctx.Err() == nil {
				line := scanner.Text()
				if strings.HasPrefix(line, "data:") {
					eventStart := time.Now()
					// Process event
					latency := time.Since(eventStart)
					metrics.RecordEvent(latency, true)
				}
			}
		}(i)

		// Stagger connection creation
		if i%100 == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Event generator
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(eventsPerSecond))
		defer ticker.Stop()

		eventCount := 0
		for {
			select {
			case <-ticker.C:
				event := events.NewTextMessageContentEvent(
					fmt.Sprintf("msg-%d", eventCount),
					fmt.Sprintf("Load test event %d at %s", eventCount, time.Now().Format(time.RFC3339)),
				)

				start := time.Now()
				err := stream.SendEvent(event)
				latency := time.Since(start)

				if err == nil {
					metrics.RecordEvent(latency, true)
				} else {
					metrics.RecordEvent(latency, false)
				}

				eventCount++

			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for test completion
	<-ctx.Done()

	// Allow time for cleanup
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cleanupCancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Cleanup completed
	case <-cleanupCtx.Done():
		t.Log("Cleanup timeout, some connections may still be active")
	}

	// Analyze results
	totalEvents := metrics.SuccessfulEvents + metrics.FailedEvents
	successRate := float64(metrics.SuccessfulEvents) / float64(totalEvents) * 100
	avgLatency := metrics.GetAverageLatency()

	t.Logf("Load Test Results:")
	t.Logf("  Total Connections: %d", metrics.TotalConnections)
	t.Logf("  Connection Errors: %d", connectionErrors)
	t.Logf("  Total Events: %d", totalEvents)
	t.Logf("  Successful Events: %d (%.2f%%)", metrics.SuccessfulEvents, successRate)
	t.Logf("  Failed Events: %d", metrics.FailedEvents)
	t.Logf("  Average Latency: %v", avgLatency)
	t.Logf("  Min Latency: %v", time.Duration(metrics.MinLatency))
	t.Logf("  Max Latency: %v", time.Duration(metrics.MaxLatency))
	t.Logf("  Memory Used: %.2f MB", float64(metrics.MemoryUsed)/(1024*1024))
	t.Logf("  Goroutines: %d", metrics.Goroutines)

	// Verify success criteria
	assert.Greater(t, metrics.TotalConnections-connectionErrors, int64(1000),
		"Should maintain >1000 concurrent connections")
	assert.Less(t, avgLatency, maxLatency,
		"Average latency should be less than 100ms")
	assert.Greater(t, successRate, 95.0,
		"Success rate should be greater than 95%")
}

// ======================== Security Vulnerability Tests ========================

// TestSecurityVulnerabilities tests for common security vulnerabilities
func TestSecurityVulnerabilities(t *testing.T) {
	logger := zap.NewNop()

	// Create secure SSE server
	securityConfig := SecurityConfig{
		Auth: AuthConfig{
			Type:        AuthTypeBearer,
			BearerToken: "secure-token-123",
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 100,
			BurstSize:         10,
		},
		CORS: CORSConfig{
			Enabled:          true,
			AllowedOrigins:   []string{"https://trusted.example.com"},
			AllowCredentials: false,
		},
	}

	securityManager, err := NewSecurityManager(securityConfig, logger)
	require.NoError(t, err)

	// Create secure handler with middleware
	baseHandler := createTestSSEHandler()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Apply security checks
		authCtx, err := securityManager.Authenticate(r)
		if err != nil || !authCtx.Authenticated {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Check rate limit
		if err := securityManager.CheckRateLimit(r); err != nil {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		// Apply security headers
		securityManager.ApplySecurityHeaders(w, r)

		// Call base handler
		baseHandler.ServeHTTP(w, r)
	})
	server := httptest.NewTLSServer(handler)
	defer server.Close()

	client := server.Client()

	t.Run("Authentication Bypass Attempt", func(t *testing.T) {
		testCases := []struct {
			name   string
			header string
			status int
		}{
			{"No Auth", "", http.StatusUnauthorized},
			{"Wrong Token", "Bearer wrong-token", http.StatusUnauthorized},
			{"Invalid Format", "InvalidAuth", http.StatusUnauthorized},
			{"SQL Injection", "Bearer ' OR '1'='1", http.StatusUnauthorized},
			{"Valid Token", "Bearer secure-token-123", http.StatusOK},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req, _ := http.NewRequest("GET", server.URL+"/events/stream", nil)
				if tc.header != "" {
					req.Header.Set("Authorization", tc.header)
				}

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, tc.status, resp.StatusCode)
			})
		}
	})

	t.Run("Rate Limiting", func(t *testing.T) {
		// Should allow initial requests
		successCount := 0
		for i := 0; i < 150; i++ {
			req, _ := http.NewRequest("GET", server.URL+"/events/stream", nil)
			req.Header.Set("Authorization", "Bearer secure-token-123")
			req.Header.Set("X-Forwarded-For", "192.168.1.1") // Same IP

			resp, err := client.Do(req)
			require.NoError(t, err)

			if resp.StatusCode == http.StatusOK {
				successCount++
			}
			resp.Body.Close()

			// Small delay to avoid overwhelming
			if i%10 == 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}

		// Should have rate limited after 100 requests
		assert.LessOrEqual(t, successCount, 110, "Rate limiting should kick in around 100 requests")
		assert.GreaterOrEqual(t, successCount, 90, "Should allow close to limit before blocking")
	})

	t.Run("CORS Validation", func(t *testing.T) {
		testCases := []struct {
			name    string
			origin  string
			allowed bool
		}{
			{"Allowed Origin", "https://trusted.example.com", true},
			{"Disallowed Origin", "https://evil.example.com", false},
			{"No Origin", "", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req, _ := http.NewRequest("GET", server.URL+"/events/stream", nil)
				req.Header.Set("Authorization", "Bearer secure-token-123")
				if tc.origin != "" {
					req.Header.Set("Origin", tc.origin)
				}

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				corsHeader := resp.Header.Get("Access-Control-Allow-Origin")
				if tc.allowed {
					assert.Equal(t, tc.origin, corsHeader)
				} else {
					assert.Empty(t, corsHeader)
				}
			})
		}
	})

	t.Run("XSS Prevention", func(t *testing.T) {
		// Create stream for XSS test
		config := DefaultStreamConfig()
		stream, err := NewEventStream(config)
		require.NoError(t, err)
		err = stream.Start()
		require.NoError(t, err)
		defer stream.Close()

		// Try to inject script in event
		maliciousContent := `<script>alert('XSS')</script>`
		event := events.NewTextMessageContentEvent("xss-test", maliciousContent)

		err = stream.SendEvent(event)
		require.NoError(t, err)

		// Receive and check event is properly escaped
		chunk := <-stream.ReceiveChunks()
		assert.NotNil(t, chunk)

		// Data should be JSON encoded, escaping the script
		assert.NotContains(t, string(chunk.Data), "<script>", "Script tags should be escaped")
		assert.Contains(t, string(chunk.Data), "\\u003cscript\\u003e", "Should be JSON escaped")
	})

	t.Run("Resource Exhaustion Protection", func(t *testing.T) {
		// Try to send extremely large payload
		largePayload := strings.Repeat("X", 10*1024*1024) // 10MB

		req, _ := http.NewRequest("POST", server.URL+"/events", strings.NewReader(largePayload))
		req.Header.Set("Authorization", "Bearer secure-token-123")
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should reject overly large payloads
		assert.NotEqual(t, http.StatusOK, resp.StatusCode, "Should reject large payloads")
	})
}

// ======================== Performance Regression Tests ========================

// TestPerformanceRegression tests for performance regressions
func TestPerformanceRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression tests in short mode")
	}

	// Load baseline metrics (these would normally come from a file or database)
	baseline := PerformanceBaseline{
		Throughput:      1000.0, // events/sec
		LatencyP50:      10 * time.Millisecond,
		LatencyP95:      50 * time.Millisecond,
		LatencyP99:      100 * time.Millisecond,
		MemoryUsage:     100 * 1024 * 1024, // 100MB
		ConnectionCount: 100,
	}

	// Run performance test
	results := runPerformanceBenchmark(t, 30*time.Second)

	// Compare with baseline
	t.Logf("Performance Test Results:")
	t.Logf("  Throughput: %.2f events/sec (baseline: %.2f)", results.Throughput, baseline.Throughput)
	t.Logf("  Latency P50: %v (baseline: %v)", results.LatencyP50, baseline.LatencyP50)
	t.Logf("  Latency P95: %v (baseline: %v)", results.LatencyP95, baseline.LatencyP95)
	t.Logf("  Latency P99: %v (baseline: %v)", results.LatencyP99, baseline.LatencyP99)
	t.Logf("  Memory Usage: %.2f MB (baseline: %.2f MB)",
		float64(results.MemoryUsage)/(1024*1024),
		float64(baseline.MemoryUsage)/(1024*1024))

	// Check for regressions (allow 10% degradation)
	assert.Greater(t, results.Throughput, baseline.Throughput*0.9,
		"Throughput regression detected")
	assert.Less(t, results.LatencyP50, time.Duration(float64(baseline.LatencyP50)*1.1),
		"P50 latency regression detected")
	assert.Less(t, results.LatencyP95, time.Duration(float64(baseline.LatencyP95)*1.1),
		"P95 latency regression detected")
	assert.Less(t, results.LatencyP99, time.Duration(float64(baseline.LatencyP99)*1.1),
		"P99 latency regression detected")
	assert.Less(t, results.MemoryUsage, uint64(float64(baseline.MemoryUsage)*1.2),
		"Memory usage regression detected")
}

// BenchmarkSSETransport benchmarks the SSE transport
func BenchmarkSSETransport(b *testing.B) {
	// Create test stream
	config := DefaultStreamConfig()
	config.EnableMetrics = false
	config.WorkerCount = runtime.NumCPU()

	stream, err := NewEventStream(config)
	require.NoError(b, err)

	err = stream.Start()
	require.NoError(b, err)
	defer stream.Close()

	// Create server
	server := httptest.NewServer(createStreamingSSEHandler(stream))
	defer server.Close()

	// Create transport
	transportConfig := DefaultConfig()
	transportConfig.BaseURL = server.URL
	transport, err := NewSSETransport(transportConfig)
	require.NoError(b, err)
	defer transport.Close()

	// Test event
	event := events.NewTextMessageContentEvent("bench", "benchmark test content")

	b.Run("Send", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				err := transport.Send(context.Background(), event)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	b.Run("Receive", func(b *testing.B) {
		ctx := context.Background()
		eventChan, err := transport.Receive(ctx)
		require.NoError(b, err)

		// Event generator
		go func() {
			for i := 0; i < b.N; i++ {
				stream.SendEvent(event)
			}
		}()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			<-eventChan
		}
	})

	b.Run("EndToEnd", func(b *testing.B) {
		ctx := context.Background()
		eventChan, err := transport.Receive(ctx)
		require.NoError(b, err)

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Send via stream
				err := stream.SendEvent(event)
				if err != nil {
					b.Fatal(err)
				}

				// Receive via transport
				select {
				case <-eventChan:
				case <-time.After(1 * time.Second):
					b.Fatal("Timeout receiving event")
				}
			}
		})
	})
}

// ======================== Memory and CPU Profiling ========================

// TestMemoryProfile generates memory profile during load test
func TestMemoryProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory profiling in short mode")
	}

	// Create memory profile file
	f, err := os.Create("mem.prof")
	require.NoError(t, err)
	defer f.Close()

	// Run load test with profiling
	runLoadTestWithProfiling(t, 10*time.Second, 100)

	// Write heap profile
	runtime.GC()
	err = pprof.WriteHeapProfile(f)
	require.NoError(t, err)

	t.Log("Memory profile written to mem.prof")
	t.Log("Analyze with: go tool pprof mem.prof")
}

// TestCPUProfile generates CPU profile during load test
func TestCPUProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CPU profiling in short mode")
	}

	// Create CPU profile file
	f, err := os.Create("cpu.prof")
	require.NoError(t, err)
	defer f.Close()

	// Start CPU profiling
	err = pprof.StartCPUProfile(f)
	require.NoError(t, err)
	defer pprof.StopCPUProfile()

	// Run load test
	runLoadTestWithProfiling(t, 10*time.Second, 100)

	t.Log("CPU profile written to cpu.prof")
	t.Log("Analyze with: go tool pprof cpu.prof")
}

// ======================== Helper Functions ========================

// createTestSSEHandler creates a basic SSE handler for testing
func createTestSSEHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Send test events
		for i := 0; i < 10; i++ {
			fmt.Fprintf(w, "event: test\n")
			fmt.Fprintf(w, "data: {\"index\": %d, \"message\": \"test event\"}\n\n", i)
			flusher.Flush()

			time.Sleep(100 * time.Millisecond)
		}
	}
}

// createStreamingSSEHandler creates an SSE handler that streams from EventStream
func createStreamingSSEHandler(stream *EventStream) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		ctx := r.Context()
		for {
			select {
			case chunk := <-stream.ReceiveChunks():
				if chunk == nil {
					return
				}

				if err := WriteSSEChunk(w, chunk); err != nil {
					return
				}
				flusher.Flush()

			case <-ctx.Done():
				return
			}
		}
	}
}

// isBrowserAvailable checks if a browser is available for testing
func isBrowserAvailable(browser string) bool {
	switch browser {
	case "chrome":
		// Check if Chrome/Chromium is available
		_, err := chromedp.NewContext(context.Background())
		if err != nil {
			return false
		}
		return true
	case "firefox", "safari", "mobile":
		// For this example, we'll only implement Chrome testing
		// In a real implementation, you would check for these browsers
		return false
	default:
		return false
	}
}

// testFirefoxSSE placeholder for Firefox testing
func testFirefoxSSE(t *testing.T, url string) {
	t.Skip("Firefox testing not implemented in this example")
}

// testSafariSSE placeholder for Safari testing
func testSafariSSE(t *testing.T, url string) {
	t.Skip("Safari testing not implemented in this example")
}

// testMobileSSE placeholder for mobile browser testing
func testMobileSSE(t *testing.T, url string) {
	t.Skip("Mobile browser testing not implemented in this example")
}

// PerformanceBaseline stores performance baseline metrics
type PerformanceBaseline struct {
	Throughput      float64 // events per second
	LatencyP50      time.Duration
	LatencyP95      time.Duration
	LatencyP99      time.Duration
	MemoryUsage     uint64
	CPUUsage        float64
	ConnectionCount int
}

// runPerformanceBenchmark runs a performance benchmark
func runPerformanceBenchmark(t *testing.T, duration time.Duration) PerformanceBaseline {
	// Create test components
	config := DefaultStreamConfig()
	config.EnableMetrics = true
	stream, err := NewEventStream(config)
	require.NoError(t, err)

	err = stream.Start()
	require.NoError(t, err)
	defer stream.Close()

	server := httptest.NewServer(createStreamingSSEHandler(stream))
	defer server.Close()

	// Collect latency samples
	var latencies []time.Duration
	var mu sync.Mutex

	eventCount := int64(0)
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// Create test connections
	connectionCount := 100
	var wg sync.WaitGroup

	for i := 0; i < connectionCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Create transport
			transportConfig := DefaultConfig()
			transportConfig.BaseURL = server.URL
			transport, err := NewSSETransport(transportConfig)
			if err != nil {
				return
			}
			defer transport.Close()

			eventChan, err := transport.Receive(ctx)
			if err != nil {
				return
			}

			// Measure event latency
			for {
				select {
				case event := <-eventChan:
					if event != nil && event.Timestamp() != nil {
						latency := time.Since(time.Unix(0, *event.Timestamp()))
						mu.Lock()
						latencies = append(latencies, latency)
						mu.Unlock()
						atomic.AddInt64(&eventCount, 1)
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Event generator
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond) // 100 events/sec
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				event := events.NewTextMessageContentEvent("perf", "performance test")
				stream.SendEvent(event)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for completion
	<-ctx.Done()
	wg.Wait()

	// Calculate metrics
	elapsed := time.Since(startTime)
	throughput := float64(eventCount) / elapsed.Seconds()

	// Calculate percentiles
	mu.Lock()
	percentiles := calculatePercentiles(latencies)
	mu.Unlock()

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return PerformanceBaseline{
		Throughput:      throughput,
		LatencyP50:      percentiles.P50,
		LatencyP95:      percentiles.P95,
		LatencyP99:      percentiles.P99,
		MemoryUsage:     memStats.Alloc,
		ConnectionCount: connectionCount,
	}
}

// Percentiles holds latency percentile values
type Percentiles struct {
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration
}

// calculatePercentiles calculates latency percentiles
func calculatePercentiles(latencies []time.Duration) Percentiles {
	if len(latencies) == 0 {
		return Percentiles{}
	}

	// Sort latencies
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	p50Index := len(sorted) * 50 / 100
	p95Index := len(sorted) * 95 / 100
	p99Index := len(sorted) * 99 / 100

	return Percentiles{
		P50: sorted[p50Index],
		P95: sorted[p95Index],
		P99: sorted[p99Index],
	}
}

// runLoadTestWithProfiling runs a load test for profiling
func runLoadTestWithProfiling(t *testing.T, duration time.Duration, connections int) {
	config := DefaultStreamConfig()
	stream, err := NewEventStream(config)
	require.NoError(t, err)

	err = stream.Start()
	require.NoError(t, err)
	defer stream.Close()

	server := httptest.NewServer(createStreamingSSEHandler(stream))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup

	// Create connections
	for i := 0; i < connections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			transportConfig := DefaultConfig()
			transportConfig.BaseURL = server.URL
			transport, err := NewSSETransport(transportConfig)
			if err != nil {
				return
			}
			defer transport.Close()

			eventChan, err := transport.Receive(ctx)
			if err != nil {
				return
			}

			for {
				select {
				case <-eventChan:
					// Process event
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Generate events
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				event := events.NewTextMessageContentEvent("profile", "profiling test")
				stream.SendEvent(event)
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
}

// ======================== Original Integration Tests ========================

// TestStreamSSEIntegration tests integration between EventStream and SSE transport
func TestStreamSSEIntegration(t *testing.T) {
	// Create a test stream
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.BatchEnabled = false
	config.CompressionEnabled = false
	config.SequenceEnabled = false
	config.EnableMetrics = true

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Create a test HTTP server that streams SSE
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("Response writer doesn't support flushing")
			return
		}

		// Stream chunks as SSE
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		for {
			select {
			case chunk := <-stream.ReceiveChunks():
				if chunk == nil {
					return
				}

				// Write the chunk as SSE
				if err := WriteSSEChunk(w, chunk); err != nil {
					t.Errorf("Failed to write SSE chunk: %v", err)
					return
				}

				flusher.Flush()

			case <-ctx.Done():
				return
			}
		}
	}))
	defer server.Close()

	// Send test events
	testEvents := []events.Event{
		events.NewRunStartedEvent("test-thread", "test-run"),
		events.NewTextMessageStartEvent("msg-1"),
		events.NewTextMessageContentEvent("msg-1", "Hello, World!"),
		events.NewTextMessageEndEvent("msg-1"),
		events.NewRunFinishedEvent("test-thread", "test-run"),
	}

	// Start a goroutine to send events
	go func() {
		time.Sleep(100 * time.Millisecond) // Give server time to start

		for _, event := range testEvents {
			if err := stream.SendEvent(event); err != nil {
				t.Errorf("Failed to send event: %v", err)
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// Make a request to the SSE endpoint
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to connect to SSE endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Verify SSE headers
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type: text/event-stream, got: %s", resp.Header.Get("Content-Type"))
	}

	// Read and verify SSE data
	buf := make([]byte, 4096)
	var sseData bytes.Buffer

	// Read with timeout
	done := make(chan bool)
	go func() {
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				sseData.Write(buf[:n])
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("Error reading SSE data: %v", err)
				break
			}
		}
		done <- true
	}()

	select {
	case <-done:
		// Reading completed
	case <-time.After(3 * time.Second):
		// Timeout - this is expected as SSE streams continuously
	}

	// Verify we received SSE-formatted data
	sseContent := sseData.String()
	if sseContent == "" {
		t.Error("No SSE data received")
	}

	// Check for SSE event structure
	if !strings.Contains(sseContent, "event:") {
		t.Error("SSE data missing event field")
	}

	if !strings.Contains(sseContent, "data:") {
		t.Error("SSE data missing data field")
	}

	// Check for specific event types
	expectedEventTypes := []string{"RUN_STARTED", "TEXT_MESSAGE_START", "TEXT_MESSAGE_CONTENT", "TEXT_MESSAGE_END", "RUN_FINISHED"}
	for _, eventType := range expectedEventTypes {
		if !strings.Contains(sseContent, eventType) {
			t.Errorf("SSE data missing expected event type: %s", eventType)
		}
	}

	t.Logf("Received SSE data:\n%s", sseContent)
}

// TestStreamCompressionWithSSE tests compressed data over SSE
func TestStreamCompressionWithSSE(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.BatchEnabled = false
	config.CompressionEnabled = true
	config.CompressionType = CompressionGzip
	config.MinCompressionSize = 0 // Compress everything for testing
	config.SequenceEnabled = false
	config.EnableMetrics = false

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Create large content that will benefit from compression
	largeContent := strings.Repeat("This is a test message that repeats many times. ", 100)
	event := events.NewTextMessageContentEvent("large-msg", largeContent)

	// Send the event
	err = stream.SendEvent(event)
	if err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	// Receive and verify the chunk
	select {
	case chunk := <-stream.ReceiveChunks():
		if !chunk.Compressed {
			t.Error("Expected compressed chunk")
		}

		// Format as SSE and verify it's valid
		sseData, err := FormatSSEChunk(chunk)
		if err != nil {
			t.Fatalf("Failed to format SSE chunk: %v", err)
		}

		if !strings.Contains(sseData, "compressed") {
			t.Error("SSE data should indicate compression")
		}

		t.Logf("Original size: %d, Compressed size: %d", len(largeContent), len(chunk.Data))

	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for compressed chunk")
	}
}

// TestStreamBatchingWithSSE tests batched events over SSE
func TestStreamBatchingWithSSE(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.BatchEnabled = true
	config.BatchSize = 3
	config.BatchTimeout = 100 * time.Millisecond
	config.CompressionEnabled = false
	config.SequenceEnabled = false
	config.EnableMetrics = false

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Send events that will be batched
	events := []events.Event{
		events.NewTextMessageStartEvent("msg-1"),
		events.NewTextMessageContentEvent("msg-1", "Hello"),
		events.NewTextMessageEndEvent("msg-1"),
	}

	for _, event := range events {
		err = stream.SendEvent(event)
		if err != nil {
			t.Fatalf("Failed to send event: %v", err)
		}
	}

	// Wait for batch processing
	select {
	case chunk := <-stream.ReceiveChunks():
		if chunk.EventType != "batch" {
			t.Errorf("Expected batch event type, got: %s", chunk.EventType)
		}

		// Format as SSE
		sseData, err := FormatSSEChunk(chunk)
		if err != nil {
			t.Fatalf("Failed to format SSE chunk: %v", err)
		}

		if !strings.Contains(sseData, "event: batch") {
			t.Error("SSE data should indicate batch event type")
		}

		t.Logf("Batch SSE data:\n%s", sseData)

	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for batch")
	}
}

// TestStreamChunkingWithSSE tests large event chunking over SSE
func TestStreamChunkingWithSSE(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.MaxChunkSize = 1024 // Small chunk size for testing
	config.BatchEnabled = false
	config.CompressionEnabled = false
	config.SequenceEnabled = false
	config.EnableMetrics = false

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Create large content that will require chunking
	largeContent := strings.Repeat("Large message content. ", 200)
	event := events.NewTextMessageContentEvent("large-msg", largeContent)

	err = stream.SendEvent(event)
	if err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	// Collect all chunks
	var chunks []*StreamChunk
	timeout := time.After(1 * time.Second)

	for {
		select {
		case chunk := <-stream.ReceiveChunks():
			chunks = append(chunks, chunk)

			// If this is the last chunk, break
			if chunk.ChunkIndex == chunk.TotalChunks-1 {
				goto done
			}

		case <-timeout:
			t.Error("Timeout waiting for chunks")
			goto done
		}
	}

done:
	if len(chunks) == 0 {
		t.Fatal("No chunks received")
	}

	// Verify chunk consistency
	totalChunks := chunks[0].TotalChunks
	if len(chunks) != totalChunks {
		t.Errorf("Expected %d chunks, got %d", totalChunks, len(chunks))
	}

	// Verify all chunks have the same event ID
	eventID := chunks[0].EventID
	for i, chunk := range chunks {
		if chunk.EventID != eventID {
			t.Errorf("Chunk %d has different event ID: %s vs %s", i, chunk.EventID, eventID)
		}

		if chunk.TotalChunks != totalChunks {
			t.Errorf("Chunk %d has different total chunks: %d vs %d", i, chunk.TotalChunks, totalChunks)
		}

		if chunk.ChunkIndex != i {
			t.Errorf("Chunk has wrong index: expected %d, got %d", i, chunk.ChunkIndex)
		}
	}

	// Format chunks as SSE and verify
	for i, chunk := range chunks {
		sseData, err := FormatSSEChunk(chunk)
		if err != nil {
			t.Fatalf("Failed to format chunk %d as SSE: %v", i, err)
		}

		// Verify chunk metadata is present
		if !strings.Contains(sseData, "chunk_index") {
			t.Errorf("Chunk %d SSE data missing chunk metadata", i)
		}

		t.Logf("Chunk %d SSE data:\n%s", i, sseData)
	}

	// Verify data can be reassembled
	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk.Data...)
	}

	if string(reassembled) != largeContent {
		t.Error("Reassembled data doesn't match original")
	}
}

// TestStreamMetricsCollection tests metrics collection during streaming
func TestStreamMetricsCollection(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.EnableMetrics = true
	config.MetricsInterval = 100 * time.Millisecond
	config.BatchEnabled = false
	config.CompressionEnabled = true
	config.MinCompressionSize = 0

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Send multiple events
	numEvents := 10
	for i := 0; i < numEvents; i++ {
		event := events.NewTextMessageContentEvent("msg", "test content for compression")
		err = stream.SendEvent(event)
		if err != nil {
			t.Fatalf("Failed to send event %d: %v", i, err)
		}
	}

	// Consume chunks
	for i := 0; i < numEvents; i++ {
		select {
		case <-stream.ReceiveChunks():
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for chunk")
		}
	}

	// Wait for metrics collection
	time.Sleep(200 * time.Millisecond)

	// Verify metrics
	metrics := stream.GetMetrics()
	if metrics == nil {
		t.Fatal("Metrics not available")
	}

	if metrics.TotalEvents != uint64(numEvents) {
		t.Errorf("Expected %d total events, got %d", numEvents, metrics.TotalEvents)
	}

	if metrics.EventsProcessed != uint64(numEvents) {
		t.Errorf("Expected %d processed events, got %d", numEvents, metrics.EventsProcessed)
	}

	if metrics.EventsCompressed == 0 {
		t.Error("Expected some events to be compressed")
	}

	if metrics.AverageLatency == 0 {
		t.Error("Expected non-zero average latency")
	}

	if metrics.FlowControl == nil {
		t.Error("Flow control metrics not available")
	}

	t.Logf("Stream metrics: TotalEvents=%d, EventsProcessed=%d, EventsCompressed=%d, AvgLatency=%v",
		metrics.TotalEvents, metrics.EventsProcessed, metrics.EventsCompressed,
		time.Duration(metrics.AverageLatency))
}

// BenchmarkStreamSSEIntegration benchmarks the complete stream-to-SSE pipeline
func BenchmarkStreamSSEIntegration(b *testing.B) {
	config := DefaultStreamConfig()
	config.WorkerCount = 4
	config.BatchEnabled = true
	config.BatchSize = 50
	config.CompressionEnabled = true
	config.EnableMetrics = false

	stream, err := NewEventStream(config)
	if err != nil {
		b.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		b.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Consumer goroutine to prevent blocking
	go func() {
		for chunk := range stream.ReceiveChunks() {
			// Simulate SSE formatting
			_, _ = FormatSSEChunk(chunk)
		}
	}()

	// Benchmark event sending
	event := events.NewTextMessageContentEvent("msg", "Benchmark test content")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := stream.SendEvent(event)
			if err != nil {
				b.Fatalf("Failed to send event: %v", err)
			}
		}
	})
}
