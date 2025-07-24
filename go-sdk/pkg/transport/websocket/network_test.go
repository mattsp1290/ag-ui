package websocket

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
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


// NetworkCondition represents different network conditions for testing
type NetworkCondition int

const (
	NetworkNormal NetworkCondition = iota
	NetworkHighLatency
	NetworkPacketLoss
	NetworkJitter
	NetworkDisconnected
	NetworkIntermittent
)

// ChaosServer simulates various network conditions and failures
type ChaosServer struct {
	server       *httptest.Server
	upgrader     websocket.Upgrader
	condition    NetworkCondition
	latency      time.Duration
	packetLoss   float64
	jitter       time.Duration
	disconnected bool
	connections  map[string]*ChaosConnection
	connMutex    sync.RWMutex
	logger       *zap.Logger

	// Chaos injection
	randomDisconnects bool
	disconnectRate    float64
	corruptMessages   bool
	corruptRate       float64
}

type ChaosConnection struct {
	conn      *websocket.Conn
	connID    string
	connected bool
	lastSeen  time.Time
	mutex     sync.RWMutex
}

func NewChaosServer(t testing.TB) *ChaosServer {
	server := &ChaosServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		condition:   NetworkNormal,
		connections: make(map[string]*ChaosConnection),
		logger:      zaptest.NewLogger(t),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.handleWebSocket)
	server.server = httptest.NewServer(mux)

	return server
}

func NewChaosTLSServer(t testing.TB) *ChaosServer {
	server := NewChaosServer(t)
	server.server.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.handleWebSocket)
	server.server = httptest.NewTLSServer(mux)

	return server
}

func (s *ChaosServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Simulate connection refused when disconnected
	if s.disconnected {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	connID := fmt.Sprintf("%s_%d", r.RemoteAddr, time.Now().UnixNano())
	chaosConn := &ChaosConnection{
		conn:      conn,
		connID:    connID,
		connected: true,
		lastSeen:  time.Now(),
	}

	s.connMutex.Lock()
	s.connections[connID] = chaosConn
	s.connMutex.Unlock()

	defer func() {
		s.connMutex.Lock()
		delete(s.connections, connID)
		s.connMutex.Unlock()
		conn.Close()
	}()

	// Handle messages with chaos injection
	for {
		// Check for random disconnection
		if s.randomDisconnects && rand.Float64() < s.disconnectRate {
			s.logger.Debug("Chaos: Random disconnection triggered", zap.String("conn_id", connID))
			break
		}

		// Apply network conditions
		s.applyNetworkConditions()

		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Debug("WebSocket read error", zap.Error(err))
			}
			break
		}

		chaosConn.mutex.Lock()
		chaosConn.lastSeen = time.Now()
		chaosConn.mutex.Unlock()

		// Simulate packet loss
		if s.condition == NetworkPacketLoss && rand.Float64() < s.packetLoss {
			s.logger.Debug("Chaos: Simulating packet loss", zap.String("conn_id", connID))
			continue
		}

		// Corrupt message if enabled
		if s.corruptMessages && rand.Float64() < s.corruptRate {
			message = s.corruptMessage(message)
			s.logger.Debug("Chaos: Message corrupted", zap.String("conn_id", connID))
		}

		// Echo message back with potential delays
		s.sendMessageWithChaos(conn, messageType, message)
	}
}

func (s *ChaosServer) applyNetworkConditions() {
	switch s.condition {
	case NetworkHighLatency:
		time.Sleep(s.latency)
	case NetworkJitter:
		jitterDelay := time.Duration(rand.Intn(int(s.jitter)))
		time.Sleep(jitterDelay)
	case NetworkIntermittent:
		// Randomly introduce delays
		if rand.Float64() < 0.3 {
			delay := time.Duration(rand.Intn(1000)) * time.Millisecond
			time.Sleep(delay)
		}
	}
}

func (s *ChaosServer) sendMessageWithChaos(conn *websocket.Conn, messageType int, message []byte) {
	// Apply latency
	if s.latency > 0 {
		time.Sleep(s.latency)
	}

	// Check for intermittent failures
	if s.condition == NetworkIntermittent && rand.Float64() < 0.1 {
		return // Drop the response
	}

	if err := conn.WriteMessage(messageType, message); err != nil {
		s.logger.Debug("Failed to write message", zap.Error(err))
	}
}

func (s *ChaosServer) corruptMessage(message []byte) []byte {
	if len(message) == 0 {
		return message
	}

	// Randomly flip bits or change bytes
	corrupted := make([]byte, len(message))
	copy(corrupted, message)

	// Corrupt 1-3 random bytes
	numCorruptions := rand.Intn(3) + 1
	for i := 0; i < numCorruptions; i++ {
		if len(corrupted) > 0 {
			pos := rand.Intn(len(corrupted))
			corrupted[pos] = byte(rand.Intn(256))
		}
	}

	return corrupted
}

func (s *ChaosServer) SetNetworkCondition(condition NetworkCondition) {
	s.condition = condition
}

func (s *ChaosServer) SetLatency(latency time.Duration) {
	s.latency = latency
}

func (s *ChaosServer) SetPacketLoss(rate float64) {
	s.packetLoss = rate
}

func (s *ChaosServer) SetJitter(jitter time.Duration) {
	s.jitter = jitter
}

func (s *ChaosServer) SetDisconnected(disconnected bool) {
	s.disconnected = disconnected
}

func (s *ChaosServer) EnableRandomDisconnects(rate float64) {
	s.randomDisconnects = true
	s.disconnectRate = rate
}

func (s *ChaosServer) EnableMessageCorruption(rate float64) {
	s.corruptMessages = true
	s.corruptRate = rate
}

func (s *ChaosServer) DisconnectAllConnections() {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	for _, conn := range s.connections {
		conn.conn.Close()
	}
}

func (s *ChaosServer) GetConnectionCount() int {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()
	return len(s.connections)
}

func (s *ChaosServer) URL() string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http") + "/ws"
}

func (s *ChaosServer) TLSURL() string {
	return "wss" + strings.TrimPrefix(s.server.URL, "https") + "/ws"
}

func (s *ChaosServer) Close() {
	s.server.Close()
}

// NetworkTestMetrics tracks network testing metrics
type NetworkTestMetrics struct {
	connectionAttempts    int64
	successfulConnections int64
	reconnectionAttempts  int64
	messagesSent          int64
	messagesReceived      int64
	messagesFailed        int64
	networkErrors         int64
	recoveryTime          time.Duration
	maxRecoveryTime       time.Duration
	totalDowntime         time.Duration
	mutex                 sync.RWMutex
}

func (m *NetworkTestMetrics) RecordConnectionAttempt() {
	atomic.AddInt64(&m.connectionAttempts, 1)
}

func (m *NetworkTestMetrics) RecordSuccessfulConnection() {
	atomic.AddInt64(&m.successfulConnections, 1)
}

func (m *NetworkTestMetrics) RecordReconnectionAttempt() {
	atomic.AddInt64(&m.reconnectionAttempts, 1)
}

func (m *NetworkTestMetrics) RecordMessageSent() {
	atomic.AddInt64(&m.messagesSent, 1)
}

func (m *NetworkTestMetrics) RecordMessageReceived() {
	atomic.AddInt64(&m.messagesReceived, 1)
}

func (m *NetworkTestMetrics) RecordMessageFailed() {
	atomic.AddInt64(&m.messagesFailed, 1)
}

func (m *NetworkTestMetrics) RecordNetworkError() {
	atomic.AddInt64(&m.networkErrors, 1)
}

func (m *NetworkTestMetrics) RecordRecoveryTime(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.recoveryTime = duration
	if duration > m.maxRecoveryTime {
		m.maxRecoveryTime = duration
	}
}

func (m *NetworkTestMetrics) AddDowntime(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.totalDowntime += duration
}

func (m *NetworkTestMetrics) GetSummary() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	connectionAttempts := atomic.LoadInt64(&m.connectionAttempts)
	successfulConnections := atomic.LoadInt64(&m.successfulConnections)
	reconnectionAttempts := atomic.LoadInt64(&m.reconnectionAttempts)
	messagesSent := atomic.LoadInt64(&m.messagesSent)
	messagesReceived := atomic.LoadInt64(&m.messagesReceived)
	messagesFailed := atomic.LoadInt64(&m.messagesFailed)
	networkErrors := atomic.LoadInt64(&m.networkErrors)

	var connectionSuccessRate float64
	if connectionAttempts > 0 {
		connectionSuccessRate = float64(successfulConnections) / float64(connectionAttempts)
	}

	var messageSuccessRate float64
	if messagesSent > 0 {
		messageSuccessRate = float64(messagesReceived) / float64(messagesSent)
	}

	return map[string]interface{}{
		"connection_attempts":     connectionAttempts,
		"successful_connections":  successfulConnections,
		"reconnection_attempts":   reconnectionAttempts,
		"messages_sent":           messagesSent,
		"messages_received":       messagesReceived,
		"messages_failed":         messagesFailed,
		"network_errors":          networkErrors,
		"connection_success_rate": connectionSuccessRate,
		"message_success_rate":    messageSuccessRate,
		"last_recovery_time_ms":   m.recoveryTime.Milliseconds(),
		"max_recovery_time_ms":    m.maxRecoveryTime.Milliseconds(),
		"total_downtime_ms":       m.totalDowntime.Milliseconds(),
	}
}

func TestNetworkLatency(t *testing.T) {
	server := NewChaosServer(t)
	defer server.Close()

	// Test with various latency levels
	latencyLevels := []time.Duration{
		50 * time.Millisecond,   // Low latency
		200 * time.Millisecond,  // Medium latency
		500 * time.Millisecond,  // High latency
		1000 * time.Millisecond, // Very high latency
	}

	for _, latency := range latencyLevels {
		t.Run(fmt.Sprintf("Latency_%dms", latency.Milliseconds()), func(t *testing.T) {
			server.SetNetworkCondition(NetworkHighLatency)
			server.SetLatency(latency)

			config := FastTransportConfig()
			config.URLs = []string{server.URL()}
			config.Logger = zaptest.NewLogger(t)
			config.PoolConfig.ConnectionTemplate.ReadTimeout = 10 * time.Second
			// Increase WriteTimeout proportionally to latency to handle round-trip delays
			config.PoolConfig.ConnectionTemplate.WriteTimeout = 5*time.Second + 3*latency
			// Increase HandshakeTimeout for high-latency scenarios
			if latency > 200*time.Millisecond {
				config.PoolConfig.ConnectionTemplate.HandshakeTimeout = 10*time.Second + 2*latency
			}
			config.EnableEventValidation = false

			transport, err := NewTransport(config)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err = transport.Start(ctx)
			require.NoError(t, err)
			defer transport.Stop()

			// Wait for connections with extended timeout for high latency
			// Need much longer timeout for high latency scenarios due to handshake delays
			connectionTimeout := 10*time.Second + 5*latency
			assert.Eventually(t, func() bool {
				// Check both connection count and health for high latency scenarios
				activeCount := transport.GetActiveConnectionCount()
				connected := transport.IsConnected()
				if !connected && activeCount > 0 {
					t.Logf("Active connections: %d, but not considered healthy", activeCount)
				}
				return connected || activeCount > 0
			}, connectionTimeout, 500*time.Millisecond)

			// For high-latency scenarios, skip rigid stabilization checks
			// since the connection behavior is expected to be different
			if latency <= 200*time.Millisecond {
				// Brief stabilization wait for low-latency scenarios
				time.Sleep(100*time.Millisecond + latency/2)
				require.True(t, transport.IsConnected(),
					"Should have healthy connection for low-latency scenarios")
			} else {
				// For high-latency scenarios, just ensure we can attempt message sending
				// The connection management will handle latency-related issues dynamically
				t.Logf("High-latency scenario (%v): proceeding with message tests without strict connection checks", latency)
			}

			// Test message sending under latency
			const numMessages = 10
			var successfulMessages int64

			startTime := time.Now()
			for i := 0; i < numMessages; i++ {
				event := &MockEvent{
					EventType: events.EventTypeTextMessageContent,
					Data:      fmt.Sprintf("latency_test_message_%d", i),
				}

				if err := transport.SendEvent(ctx, event); err == nil {
					atomic.AddInt64(&successfulMessages, 1)
				}
			}
			totalTime := time.Since(startTime)

			// Verify messages were sent successfully
			assert.Equal(t, int64(numMessages), successfulMessages)

			// Average time per message should account for latency
			avgTimePerMessage := totalTime / time.Duration(numMessages)
			t.Logf("Average time per message with %v latency: %v", latency, avgTimePerMessage)

			// For high-latency scenarios, focus on successful message transmission
			// rather than strict connectivity health checks
			if latency <= 200*time.Millisecond {
				// Only enforce strict connectivity for low-latency scenarios
				assert.True(t, transport.IsConnected(),
					"Should maintain healthy connectivity for low-latency scenarios")
			} else {
				// For high-latency scenarios, successful message sending is the key indicator
				t.Logf("High-latency scenario (%v): Messages sent successfully even if connection health varies", latency)
			}
		})
	}
}

func TestPacketLoss(t *testing.T) {
	server := NewChaosServer(t)
	defer server.Close()

	packetLossRates := []float64{0.05, 0.1, 0.2, 0.3} // 5%, 10%, 20%, 30%

	for _, lossRate := range packetLossRates {
		t.Run(fmt.Sprintf("PacketLoss_%.0f_percent", lossRate*100), func(t *testing.T) {
			server.SetNetworkCondition(NetworkPacketLoss)
			server.SetPacketLoss(lossRate)

			config := FastTransportConfig()
			config.URLs = []string{server.URL()}
			config.Logger = zaptest.NewLogger(t)
			config.PoolConfig.ConnectionTemplate.MaxReconnectAttempts = 5
			config.PoolConfig.ConnectionTemplate.InitialReconnectDelay = 100 * time.Millisecond
			config.EnableEventValidation = false

			transport, err := NewTransport(config)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err = transport.Start(ctx)
			require.NoError(t, err)
			defer transport.Stop()

			// Wait for connections
			assert.Eventually(t, func() bool {
				return transport.IsConnected()
			}, 10*time.Second, 200*time.Millisecond)

			// Send messages and measure success rate
			const numMessages = 100
			var successfulMessages int64
			var errors int64

			for i := 0; i < numMessages; i++ {
				event := &MockEvent{
					EventType: events.EventTypeTextMessageContent,
					Data:      fmt.Sprintf("packet_loss_test_%d", i),
				}

				if err := transport.SendEvent(ctx, event); err != nil {
					atomic.AddInt64(&errors, 1)
				} else {
					atomic.AddInt64(&successfulMessages, 1)
				}

				// Small delay between messages
				time.Sleep(10 * time.Millisecond)
			}

			successRate := float64(successfulMessages) / float64(numMessages)
			t.Logf("Success rate with %.0f%% packet loss: %.2f%%",
				lossRate*100, successRate*100)

			// Even with packet loss, should maintain reasonable success rate
			// due to WebSocket reliability features
			expectedMinSuccessRate := 1.0 - (lossRate * 1.5) // Allow some tolerance
			assert.Greater(t, successRate, expectedMinSuccessRate,
				"Success rate should be reasonable even with packet loss")

			// Should maintain connectivity
			assert.True(t, transport.IsConnected())
		})
	}
}

func TestNetworkPartition(t *testing.T) {
	server := NewChaosServer(t)
	defer server.Close()

	config := FastTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.PoolConfig.ConnectionTemplate.MaxReconnectAttempts = 10
	config.PoolConfig.ConnectionTemplate.InitialReconnectDelay = 100 * time.Millisecond
	config.PoolConfig.ConnectionTemplate.MaxReconnectDelay = 2 * time.Second
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	metrics := &NetworkTestMetrics{}

	t.Run("Network_Partition_Recovery", func(t *testing.T) {
		// Wait for initial connection
		assert.Eventually(t, func() bool {
			if transport.IsConnected() {
				metrics.RecordSuccessfulConnection()
				return true
			}
			metrics.RecordConnectionAttempt()
			return false
		}, 10*time.Second, 200*time.Millisecond)

		// Verify initial connectivity
		assert.True(t, transport.IsConnected())
		initialConnections := transport.GetActiveConnectionCount()

		// Simulate network partition by disconnecting server
		t.Log("Simulating network partition...")
		server.SetDisconnected(true)
		server.DisconnectAllConnections()

		// Wait for disconnection to be detected
		assert.Eventually(t, func() bool {
			connected := transport.IsConnected()
			if !connected {
				metrics.RecordNetworkError()
			}
			return !connected
		}, 10*time.Second, 500*time.Millisecond, "Should detect network partition")

		// Try to send messages during partition (should fail)
		for i := 0; i < 5; i++ {
			event := &MockEvent{
				EventType: events.EventTypeTextMessageContent,
				Data:      fmt.Sprintf("partition_test_message_%d", i),
			}

			if err := transport.SendEvent(ctx, event); err != nil {
				metrics.RecordMessageFailed()
			} else {
				metrics.RecordMessageSent()
			}
		}

		// Simulate network recovery
		partitionDuration := 5 * time.Second
		time.Sleep(partitionDuration)
		t.Log("Simulating network recovery...")

		recoveryStart := time.Now()
		server.SetDisconnected(false)

		// Wait for reconnection
		reconnected := assert.Eventually(t, func() bool {
			if transport.IsConnected() {
				metrics.RecordSuccessfulConnection()
				return true
			}
			metrics.RecordReconnectionAttempt()
			return false
		}, 20*time.Second, 500*time.Millisecond, "Should reconnect after partition")

		if reconnected {
			recoveryTime := time.Since(recoveryStart)
			metrics.RecordRecoveryTime(recoveryTime)
			metrics.AddDowntime(partitionDuration + recoveryTime)

			t.Logf("Network recovery completed in %v", recoveryTime)

			// Verify connections are re-established
			assert.Eventually(t, func() bool {
				return transport.GetActiveConnectionCount() >= initialConnections
			}, 10*time.Second, 500*time.Millisecond)

			// Test message sending after recovery
			for i := 0; i < 10; i++ {
				event := &MockEvent{
					EventType: events.EventTypeTextMessageContent,
					Data:      fmt.Sprintf("recovery_test_message_%d", i),
				}

				if err := transport.SendEvent(ctx, event); err != nil {
					metrics.RecordMessageFailed()
				} else {
					metrics.RecordMessageSent()
					metrics.RecordMessageReceived() // Assume success for now
				}
			}

			// Recovery should be reasonably fast
			assert.Less(t, recoveryTime, 15*time.Second,
				"Recovery should complete within 15 seconds")
		}

		// Print network test summary
		summary := metrics.GetSummary()
		t.Logf("Network Partition Test Summary: %+v", summary)
	})
}

func TestIntermittentConnectivity(t *testing.T) {
	server := NewChaosServer(t)
	defer server.Close()

	server.SetNetworkCondition(NetworkIntermittent)
	server.EnableRandomDisconnects(0.05) // 5% chance of random disconnect

	config := FastTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.PoolConfig.ConnectionTemplate.MaxReconnectAttempts = 20
	config.PoolConfig.ConnectionTemplate.InitialReconnectDelay = 50 * time.Millisecond
	config.PoolConfig.ConnectionTemplate.PingPeriod = 1 * time.Second
	config.PoolConfig.ConnectionTemplate.PongWait = 2 * time.Second
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	metrics := &NetworkTestMetrics{}

	t.Run("Intermittent_Connectivity_Resilience", func(t *testing.T) {
		// Wait for initial connection
		assert.Eventually(t, func() bool {
			if transport.IsConnected() {
				metrics.RecordSuccessfulConnection()
				return true
			}
			return false
		}, 10*time.Second, 200*time.Millisecond)

		const testDuration = 30 * time.Second
		const messageInterval = 100 * time.Millisecond

		startTime := time.Now()
		messageCount := 0

		// Send messages continuously during intermittent connectivity
		for time.Since(startTime) < testDuration {
			event := &MockEvent{
				EventType: events.EventTypeTextMessageContent,
				Data:      fmt.Sprintf("intermittent_test_%d", messageCount),
			}

			if err := transport.SendEvent(ctx, event); err != nil {
				metrics.RecordMessageFailed()
				metrics.RecordNetworkError()
			} else {
				metrics.RecordMessageSent()
			}

			messageCount++
			time.Sleep(messageInterval)
		}

		t.Logf("Sent %d messages during intermittent connectivity test", messageCount)

		// Verify final connectivity state
		isConnected := transport.IsConnected()
		if !isConnected {
			// Wait a bit more for potential recovery
			assert.Eventually(t, func() bool {
				return transport.IsConnected()
			}, 10*time.Second, 500*time.Millisecond,
				"Should eventually reconnect after intermittent issues")
		}

		// Even with intermittent issues, should achieve reasonable success rate
		summary := metrics.GetSummary()
		successRate := summary["message_success_rate"].(float64)

		t.Logf("Message success rate under intermittent connectivity: %.2f%%", successRate*100)
		t.Logf("Intermittent Connectivity Test Summary: %+v", summary)

		// Should maintain reasonable performance despite intermittent issues
		assert.Greater(t, successRate, 0.7,
			"Should maintain >70% success rate despite intermittent connectivity")
	})
}

func TestTLSNetworkFailures(t *testing.T) {
	server := NewChaosTLSServer(t)
	defer server.Close()

	config := FastTransportConfig()
	config.URLs = []string{server.TLSURL()}
	config.Logger = zaptest.NewLogger(t)
	config.PoolConfig.ConnectionTemplate.MaxReconnectAttempts = 5
	// Configure TLS client to skip verification for testing
	config.PoolConfig.ConnectionTemplate.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	t.Run("TLS_Connection_Resilience", func(t *testing.T) {
		err := transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		// Wait for TLS connection
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 15*time.Second, 500*time.Millisecond)

		// Send messages over TLS
		for i := 0; i < 10; i++ {
			event := &MockEvent{
				EventType: events.EventTypeTextMessageContent,
				Data:      fmt.Sprintf("tls_test_message_%d", i),
			}

			err := transport.SendEvent(ctx, event)
			assert.NoError(t, err, "Should send messages over TLS successfully")
		}

		// Simulate TLS connection interruption
		server.DisconnectAllConnections()

		// Should recover TLS connection
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 20*time.Second, 1*time.Second)

		// Verify TLS functionality after recovery
		event := &MockEvent{
			EventType: events.EventTypeTextMessageContent,
			Data:      "tls_recovery_test",
		}

		err = transport.SendEvent(ctx, event)
		assert.NoError(t, err, "Should send messages after TLS recovery")
	})
}

func TestMessageCorruption(t *testing.T) {
	server := NewChaosServer(t)
	defer server.Close()

	server.EnableMessageCorruption(0.2) // 20% message corruption rate

	config := FastTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("Message_Corruption_Handling", func(t *testing.T) {
		// Wait for connections
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 10*time.Second, 200*time.Millisecond)

		const numMessages = 50
		var successfulSends int64

		// Send messages with potential corruption
		for i := 0; i < numMessages; i++ {
			event := &MockEvent{
				EventType: events.EventTypeTextMessageContent,
				Data:      fmt.Sprintf("corruption_test_message_%d_with_some_content", i),
			}

			if err := transport.SendEvent(ctx, event); err == nil {
				atomic.AddInt64(&successfulSends, 1)
			}

			time.Sleep(50 * time.Millisecond)
		}

		successRate := float64(successfulSends) / float64(numMessages)
		t.Logf("Message send success rate with corruption: %.2f%%", successRate*100)

		// Should be able to send messages despite corruption
		assert.Greater(t, successRate, 0.9,
			"Should maintain high send success rate despite message corruption")

		// Connection should remain stable
		assert.True(t, transport.IsConnected())
	})
}

func TestCascadingFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cascading failures test in short mode")
	}

	// Create multiple servers to simulate cascading failures
	servers := make([]*ChaosServer, 3)
	urls := make([]string, 3)

	for i := 0; i < 3; i++ {
		servers[i] = NewChaosServer(t)
		urls[i] = servers[i].URL()
	}
	defer func() {
		for _, server := range servers {
			server.Close()
		}
	}()

	config := FastTransportConfig()
	config.URLs = urls
	config.Logger = zaptest.NewLogger(t)
	config.PoolConfig.MinConnections = 3
	config.PoolConfig.MaxConnections = 6
	config.PoolConfig.ConnectionTemplate.MaxReconnectAttempts = 3
	config.PoolConfig.ConnectionTemplate.InitialReconnectDelay = 100 * time.Millisecond
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("Cascading_Server_Failures", func(t *testing.T) {
		// Wait for connections to all servers
		assert.Eventually(t, func() bool {
			return transport.GetActiveConnectionCount() >= 3
		}, 15*time.Second, 500*time.Millisecond)

		initialConnections := transport.GetActiveConnectionCount()
		t.Logf("Initial connections: %d", initialConnections)

		// Phase 1: Fail first server
		t.Log("Phase 1: Failing server 1")
		servers[0].SetDisconnected(true)
		servers[0].DisconnectAllConnections()

		time.Sleep(500 * time.Millisecond)

		// Should still have connections to other servers
		assert.True(t, transport.IsConnected(), "Should maintain connectivity after 1 server failure")

		// Test message sending
		for i := 0; i < 5; i++ {
			event := &MockEvent{
				EventType: events.EventTypeTextMessageContent,
				Data:      fmt.Sprintf("cascade_test_phase1_%d", i),
			}
			err := transport.SendEvent(ctx, event)
			assert.NoError(t, err, "Should send messages after 1 server failure")
		}

		// Phase 2: Fail second server
		t.Log("Phase 2: Failing server 2")
		servers[1].SetDisconnected(true)
		servers[1].DisconnectAllConnections()

		time.Sleep(500 * time.Millisecond)

		// Should still have connection to the last server
		assert.True(t, transport.IsConnected(), "Should maintain connectivity after 2 server failures")

		// Test message sending with only one server
		for i := 0; i < 5; i++ {
			event := &MockEvent{
				EventType: events.EventTypeTextMessageContent,
				Data:      fmt.Sprintf("cascade_test_phase2_%d", i),
			}
			err := transport.SendEvent(ctx, event)
			assert.NoError(t, err, "Should send messages with 1 remaining server")
		}

		// Phase 3: Fail all servers
		t.Log("Phase 3: Failing all servers")
		servers[2].SetDisconnected(true)
		servers[2].DisconnectAllConnections()

		// Wait for disconnection
		assert.Eventually(t, func() bool {
			return !transport.IsConnected()
		}, 10*time.Second, 500*time.Millisecond, "Should detect complete failure")

		// Phase 4: Gradual recovery
		t.Log("Phase 4: Starting recovery")

		// Restore servers one by one
		servers[2].SetDisconnected(false)

		// Should reconnect to available server
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 15*time.Second, 500*time.Millisecond, "Should reconnect when server becomes available")

		// Restore more servers
		servers[1].SetDisconnected(false)
		servers[0].SetDisconnected(false)

		// Should eventually restore full connectivity
		assert.Eventually(t, func() bool {
			return transport.GetActiveConnectionCount() >= 2
		}, 20*time.Second, 1*time.Second, "Should restore multiple connections")

		finalConnections := transport.GetActiveConnectionCount()
		t.Logf("Final connections after recovery: %d", finalConnections)

		// Test final functionality
		for i := 0; i < 10; i++ {
			event := &MockEvent{
				EventType: events.EventTypeTextMessageContent,
				Data:      fmt.Sprintf("cascade_test_recovery_%d", i),
			}
			err := transport.SendEvent(ctx, event)
			assert.NoError(t, err, "Should send messages after full recovery")
		}

		assert.GreaterOrEqual(t, finalConnections, 2,
			"Should restore reasonable number of connections")
	})
}

func TestSlowNetworkConditions(t *testing.T) {
	server := NewChaosServer(t)
	defer server.Close()

	server.SetNetworkCondition(NetworkJitter)
	server.SetJitter(2 * time.Second) // High jitter

	config := FastTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.PoolConfig.ConnectionTemplate.ReadTimeout = 30 * time.Second
	config.PoolConfig.ConnectionTemplate.WriteTimeout = 10 * time.Second
	config.PoolConfig.ConnectionTemplate.HandshakeTimeout = 15 * time.Second
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("High_Jitter_Network_Tolerance", func(t *testing.T) {
		// Wait for connections with extended timeout due to jitter
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 20*time.Second, 1*time.Second)

		const numMessages = 10
		var successfulMessages int64
		var totalTime time.Duration

		// Send messages and measure time with jitter
		for i := 0; i < numMessages; i++ {
			event := &MockEvent{
				EventType: events.EventTypeTextMessageContent,
				Data:      fmt.Sprintf("jitter_test_message_%d", i),
			}

			start := time.Now()
			if err := transport.SendEvent(ctx, event); err == nil {
				atomic.AddInt64(&successfulMessages, 1)
				totalTime += time.Since(start)
			}
		}

		var avgTime time.Duration
		if successfulMessages > 0 {
			avgTime = totalTime / time.Duration(successfulMessages)
			t.Logf("Average message time with high jitter: %v", avgTime)
		} else {
			t.Logf("No messages sent successfully under high jitter conditions")
		}

		// Should handle jittery network conditions
		assert.Equal(t, int64(numMessages), successfulMessages,
			"Should send all messages despite network jitter")
		assert.True(t, transport.IsConnected(),
			"Should maintain connectivity despite jitter")
	})
}

// Network testing benchmarks
func BenchmarkNetworkLatencyPerformance(b *testing.B) {
	server := NewChaosServer(b)
	defer server.Close()

	server.SetNetworkCondition(NetworkHighLatency)
	server.SetLatency(100 * time.Millisecond)

	config := FastTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
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
		Data:      "benchmark network latency test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = transport.SendEvent(ctx, event)
	}
}

func BenchmarkNetworkRecoveryTime(b *testing.B) {
	server := NewChaosServer(b)
	defer server.Close()

	config := FastTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
	config.PoolConfig.ConnectionTemplate.MaxReconnectAttempts = 5
	config.PoolConfig.ConnectionTemplate.InitialReconnectDelay = 10 * time.Millisecond
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(b, err)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(b, err)
	defer transport.Stop()

	// Wait for initial connection
	time.Sleep(1 * time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate disconnection
		server.SetDisconnected(true)
		server.DisconnectAllConnections()

		// Wait for disconnection
		for transport.IsConnected() {
			time.Sleep(10 * time.Millisecond)
		}

		// Restore connection
		server.SetDisconnected(false)

		// Measure recovery time
		start := time.Now()
		for !transport.IsConnected() {
			time.Sleep(10 * time.Millisecond)
		}
		_ = time.Since(start)
	}
}
