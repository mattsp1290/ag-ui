package sse

import (
	"testing"
	"time"
)

func TestConnectionState(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{ConnectionStateDisconnected, "disconnected"},
		{ConnectionStateConnecting, "connecting"},
		{ConnectionStateConnected, "connected"},
		{ConnectionStateReconnecting, "reconnecting"},
		{ConnectionStateError, "error"},
		{ConnectionStateClosed, "closed"},
	}

	for _, test := range tests {
		if test.state.String() != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, test.state.String())
		}
	}
}

func TestReconnectionPolicy(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	policy := DefaultReconnectionPolicy()

	if !policy.Enabled {
		t.Error("Expected reconnection to be enabled by default")
	}

	if policy.MaxAttempts != 10 {
		t.Errorf("Expected max attempts to be 10, got %d", policy.MaxAttempts)
	}

	if policy.BackoffMultiplier != 2.0 {
		t.Errorf("Expected backoff multiplier to be 2.0, got %f", policy.BackoffMultiplier)
	}
}

func TestHeartbeatConfig(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	config := DefaultHeartbeatConfig()

	if !config.Enabled {
		t.Error("Expected heartbeat to be enabled by default")
	}

	if config.Interval != 30*time.Second {
		t.Errorf("Expected interval to be 30s, got %v", config.Interval)
	}

	if config.MaxMissed != 5 {
		t.Errorf("Expected max missed to be 5, got %d", config.MaxMissed)
	}
}

func TestConnectionMetrics(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	metrics := NewConnectionMetrics()

	// Test initial values
	if metrics.ConnectAttempts.Load() != 0 {
		t.Error("Expected initial connect attempts to be 0")
	}

	// Test recording metrics
	metrics.RecordConnectAttempt()
	if metrics.ConnectAttempts.Load() != 1 {
		t.Error("Expected connect attempts to be 1 after recording")
	}

	duration := 100 * time.Millisecond
	metrics.RecordConnectSuccess(duration)
	if metrics.ConnectSuccesses.Load() != 1 {
		t.Error("Expected connect successes to be 1 after recording")
	}

	// Test success rate calculation
	rate := metrics.GetConnectSuccessRate()
	if rate != 100.0 {
		t.Errorf("Expected success rate to be 100%%, got %f", rate)
	}
}

func TestNewConnection(t *testing.T) {
	config := DefaultConfig()

	conn, err := NewConnection(config, nil)
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}

	if conn.State() != ConnectionStateDisconnected {
		t.Errorf("Expected initial state to be disconnected, got %s", conn.State().String())
	}

	if conn.ID() == "" {
		t.Error("Expected connection to have an ID")
	}

	// Test state change
	conn.setState(ConnectionStateConnecting)
	if conn.State() != ConnectionStateConnecting {
		t.Errorf("Expected state to be connecting after setState, got %s", conn.State().String())
	}

	// Clean up
	conn.Close()
	if conn.State() != ConnectionStateClosed {
		t.Errorf("Expected state to be closed after Close(), got %s", conn.State().String())
	}
}

func TestConnectionLifecycle(t *testing.T) {
	config := DefaultConfig()

	conn, err := NewConnection(config, nil)
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}
	defer conn.Close()

	// Test initial state
	if !conn.IsAlive() {
		t.Error("Expected new connection to be alive")
	}

	if conn.IsConnected() {
		t.Error("Expected new connection to not be connected initially")
	}

	// Test closing
	conn.Close()
	if conn.IsAlive() {
		t.Error("Expected closed connection to not be alive")
	}
}

func TestReconnectDelayCalculation(t *testing.T) {
	config := DefaultConfig()

	conn, err := NewConnection(config, nil)
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}
	defer conn.Close()

	// Test delay calculation
	delay1 := conn.calculateReconnectDelay(1)
	delay2 := conn.calculateReconnectDelay(2)
	delay3 := conn.calculateReconnectDelay(3)

	// Should be increasing with exponential backoff
	if delay1 >= delay2 {
		t.Errorf("Expected delay to increase: %v >= %v", delay1, delay2)
	}

	if delay2 >= delay3 {
		t.Errorf("Expected delay to increase: %v >= %v", delay2, delay3)
	}

	// Should not exceed max delay
	maxDelay := conn.reconnectPolicy.MaxDelay
	delayMax := conn.calculateReconnectDelay(20) // Large attempt number
	if delayMax > maxDelay*2 {                   // Allow some jitter
		t.Errorf("Expected delay to not exceed max delay (with jitter): %v > %v", delayMax, maxDelay*2)
	}
}

func TestConnectionPool(t *testing.T) {
	config := DefaultConfig()

	pool, err := NewConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Test initial state
	stats := pool.GetPoolStats()
	if stats["total_connections"].(int64) != 0 {
		t.Error("Expected initial pool to have 0 connections")
	}

	healthyCount := pool.GetHealthyConnectionCount()
	if healthyCount != 0 {
		t.Errorf("Expected 0 healthy connections initially, got %d", healthyCount)
	}
}

func TestConnectionInfo(t *testing.T) {
	config := DefaultConfig()

	conn, err := NewConnection(config, nil)
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}
	defer conn.Close()

	info := conn.GetConnectionInfo()

	// Check required fields
	if info["id"] == "" {
		t.Error("Expected connection info to include ID")
	}

	if info["state"] != "disconnected" {
		t.Errorf("Expected initial state to be disconnected, got %s", info["state"])
	}

	if info["reconnect_attempts"].(int32) != 0 {
		t.Error("Expected initial reconnect attempts to be 0")
	}

	if info["uptime"].(time.Duration) != 0 {
		t.Error("Expected initial uptime to be 0")
	}
}

func TestPoolMetrics(t *testing.T) {
	config := DefaultConfig()

	pool, err := NewConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Test initial metrics
	if pool.poolMetrics.TotalConnections.Load() != 0 {
		t.Error("Expected initial total connections to be 0")
	}

	if pool.poolMetrics.ActiveConnections.Load() != 0 {
		t.Error("Expected initial active connections to be 0")
	}

	stats := pool.GetPoolStats()
	if stats["pool_utilization"].(float64) != 0.0 {
		t.Error("Expected initial pool utilization to be 0.0")
	}
}

func TestConnectionChannels(t *testing.T) {
	config := DefaultConfig()

	conn, err := NewConnection(config, nil)
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}
	defer conn.Close()

	// Test channels are accessible
	eventChan := conn.ReadEvents()
	if eventChan == nil {
		t.Error("Expected event channel to be available")
	}

	errorChan := conn.ReadErrors()
	if errorChan == nil {
		t.Error("Expected error channel to be available")
	}

	stateChan := conn.ReadStateChanges()
	if stateChan == nil {
		t.Error("Expected state change channel to be available")
	}

	// Test state change notification
	go func() {
		conn.setState(ConnectionStateConnecting)
	}()

	select {
	case state := <-stateChan:
		if state != ConnectionStateConnecting {
			t.Errorf("Expected state change to be connecting, got %s", state.String())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive state change notification")
	}
}

func TestHeartbeatMetrics(t *testing.T) {
	metrics := NewConnectionMetrics()

	// Test heartbeat recording
	metrics.RecordHeartbeat(true)
	if metrics.HeartbeatsSent.Load() != 1 {
		t.Error("Expected heartbeats sent to be 1")
	}

	if metrics.HeartbeatsSuccess.Load() != 1 {
		t.Error("Expected heartbeat successes to be 1")
	}

	metrics.RecordHeartbeat(false)
	if metrics.HeartbeatsSent.Load() != 2 {
		t.Error("Expected heartbeats sent to be 2")
	}

	if metrics.HeartbeatsFailed.Load() != 1 {
		t.Error("Expected heartbeat failures to be 1")
	}

	// Test success rate
	rate := metrics.GetHeartbeatSuccessRate()
	if rate != 50.0 {
		t.Errorf("Expected heartbeat success rate to be 50%%, got %f", rate)
	}
}

func TestConnectionStates(t *testing.T) {
	config := DefaultConfig()

	conn, err := NewConnection(config, nil)
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}
	defer conn.Close()

	// Test all state transitions
	states := []ConnectionState{
		ConnectionStateConnecting,
		ConnectionStateConnected,
		ConnectionStateReconnecting,
		ConnectionStateError,
		ConnectionStateDisconnected,
	}

	for _, state := range states {
		conn.setState(state)
		if conn.State() != state {
			t.Errorf("Failed to set state to %s", state.String())
		}
	}

	// Test final close
	conn.setState(ConnectionStateClosed)
	if conn.State() != ConnectionStateClosed {
		t.Error("Failed to set state to closed")
	}
}
