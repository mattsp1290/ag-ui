package websocket

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestHeartbeatBasicOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping heartbeat test in short mode")
	}
	
	// Setup test WebSocket server
	server := createTestWebSocketServer(t)
	defer server.Close()

	config := DefaultConnectionConfig()
	config.URL = "ws" + strings.TrimPrefix(server.URL, "http")
	config.Logger = zaptest.NewLogger(t)
	config.PingPeriod = 100 * time.Millisecond
	config.PongWait = 200 * time.Millisecond

	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = conn.Connect(ctx)
	require.NoError(t, err)

	heartbeat := conn.heartbeat

	// Start the heartbeat manually (as per design)
	heartbeat.Start(ctx)
	
	// Test state after manual start
	assert.Equal(t, HeartbeatRunning, heartbeat.GetState())
	assert.True(t, heartbeat.IsHealthy()) // Should start healthy

	// Wait for some ping/pong cycles
	time.Sleep(500 * time.Millisecond)

	// Check stats
	stats := heartbeat.GetStats()
	assert.Greater(t, stats.PingsSent, int64(0))
	assert.Greater(t, stats.HealthChecks, int64(0))

	// Stop heartbeat explicitly before closing connection
	heartbeat.Stop()
	assert.Equal(t, HeartbeatStopped, heartbeat.GetState())

	// Close connection
	err = conn.Close()
	require.NoError(t, err)
}

func TestHeartbeatStateTransitions(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("Error closing connection: %v", closeErr)
		}
	}()

	heartbeat := conn.heartbeat

	// Test valid state transitions
	assert.True(t, heartbeat.setState(HeartbeatStarting))
	assert.Equal(t, HeartbeatStarting, heartbeat.GetState())

	assert.True(t, heartbeat.setState(HeartbeatRunning))
	assert.Equal(t, HeartbeatRunning, heartbeat.GetState())

	assert.True(t, heartbeat.setState(HeartbeatStopping))
	assert.Equal(t, HeartbeatStopping, heartbeat.GetState())

	assert.True(t, heartbeat.setState(HeartbeatStopped))
	assert.Equal(t, HeartbeatStopped, heartbeat.GetState())

	// Test invalid state transitions
	assert.False(t, heartbeat.setState(HeartbeatRunning))
	assert.Equal(t, HeartbeatStopped, heartbeat.GetState())
}

func TestHeartbeatHealthMonitoring(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.Logger = zaptest.NewLogger(t)
	config.PingPeriod = 50 * time.Millisecond
	config.PongWait = 1 * time.Second

	conn, err := NewConnection(config)
	require.NoError(t, err)
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("Error closing connection: %v", closeErr)
		}
	}()

	heartbeat := conn.heartbeat

	// Test initial health - should be healthy at start
	assert.True(t, heartbeat.IsHealthy())
	
	// Set lastPongAt to a recent time using nanoseconds consistently
	now := time.Now()
	atomic.StoreInt64(&heartbeat.lastPongAt, now.UnixNano())
	assert.Greater(t, heartbeat.GetConnectionHealth(), float64(0.0))

	// Simulate missed pong by setting lastPongAt to an old time (beyond pong wait)
	oldTime := now.Add(-config.PongWait - 1*time.Second) // Ensure it's beyond pong wait
	atomic.StoreInt64(&heartbeat.lastPongAt, oldTime.UnixNano())

	// Ensure heartbeat is in running state for health check to work
	atomic.StoreInt32(&heartbeat.state, int32(HeartbeatRunning))

	// Check health after missed pong - give it a moment to detect
	time.Sleep(50 * time.Millisecond)
	heartbeat.checkHealth()
	
	// After missed pong beyond timeout, should be unhealthy
	assert.False(t, heartbeat.IsHealthy())
	assert.Equal(t, float64(0.0), heartbeat.GetConnectionHealth())

	// Simulate received pong - this should restore health
	heartbeat.OnPong()
	
	// Should be healthy again after receiving pong
	assert.True(t, heartbeat.IsHealthy())
	assert.Greater(t, heartbeat.GetConnectionHealth(), float64(0.0))
}

func TestHeartbeatPongHandling(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("Error closing connection: %v", closeErr)
		}
	}()

	heartbeat := conn.heartbeat

	// Test initial stats to ensure we have a baseline
	initialStats := heartbeat.GetStats()
	assert.Equal(t, int64(0), initialStats.PongsReceived)
	
	// Get initial pong time as baseline
	initialPongTime := heartbeat.GetLastPongTime()
	
	// Wait a small amount to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Test pong handling - OnPong() should update the timestamp and stats
	heartbeat.OnPong()

	// Get the updated pong time
	newPongTime := heartbeat.GetLastPongTime()

	// Check that pong time was updated (should be different)
	assert.True(t, newPongTime.After(initialPongTime) || newPongTime.Equal(initialPongTime.Add(time.Nanosecond)), 
		"New pong time (%v) should be after or very close to initial (%v)", newPongTime, initialPongTime)
	
	// Should be healthy after receiving pong
	assert.True(t, heartbeat.IsHealthy())
	assert.Equal(t, int32(0), heartbeat.GetMissedPongCount())

	// Check stats - should show one pong received
	stats := heartbeat.GetStats()
	assert.Equal(t, int64(1), stats.PongsReceived)

	// Test that calling OnPong() again increments the count
	heartbeat.OnPong()
	secondStats := heartbeat.GetStats()
	assert.Equal(t, int64(2), secondStats.PongsReceived)
	assert.True(t, heartbeat.IsHealthy())
}

func TestHeartbeatRTTCalculation(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("Error closing connection: %v", closeErr)
		}
	}()

	heartbeat := conn.heartbeat

	// Simulate ping/pong sequence
	heartbeat.stats.LastPingAt = time.Now()
	time.Sleep(10 * time.Millisecond)
	heartbeat.OnPong()

	stats := heartbeat.GetStats()
	assert.Greater(t, stats.AverageRTT, time.Duration(0))
	assert.Greater(t, stats.MaxRTT, time.Duration(0))
	assert.Greater(t, stats.MinRTT, time.Duration(0))
	assert.Equal(t, stats.MinRTT, stats.MaxRTT) // Only one measurement

	// Add another measurement
	heartbeat.stats.LastPingAt = time.Now()
	time.Sleep(20 * time.Millisecond)
	heartbeat.OnPong()

	stats = heartbeat.GetStats()
	assert.Greater(t, stats.MaxRTT, stats.MinRTT)
}

func TestHeartbeatConfigurationUpdates(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("Error closing connection: %v", closeErr)
		}
	}()

	heartbeat := conn.heartbeat

	// Test initial configuration
	assert.Equal(t, config.PingPeriod, heartbeat.GetPingPeriod())
	assert.Equal(t, config.PongWait, heartbeat.GetPongWait())

	// Update configuration
	newPingPeriod := 60 * time.Second
	newPongWait := 65 * time.Second

	heartbeat.SetPingPeriod(newPingPeriod)
	heartbeat.SetPongWait(newPongWait)

	assert.Equal(t, newPingPeriod, heartbeat.GetPingPeriod())
	assert.Equal(t, newPongWait, heartbeat.GetPongWait())
}

func TestHeartbeatDetailedStatus(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	heartbeat := conn.heartbeat

	// Get detailed status
	status := heartbeat.GetDetailedHealthStatus()

	// Check required fields
	assert.Contains(t, status, "is_healthy")
	assert.Contains(t, status, "health_score")
	assert.Contains(t, status, "state")
	assert.Contains(t, status, "last_ping_at")
	assert.Contains(t, status, "last_pong_at")
	assert.Contains(t, status, "time_since_last_pong")
	assert.Contains(t, status, "missed_pongs")
	assert.Contains(t, status, "ping_period")
	assert.Contains(t, status, "pong_wait")
	assert.Contains(t, status, "total_pings_sent")
	assert.Contains(t, status, "total_pongs_received")
	assert.Contains(t, status, "total_missed_pongs")
	assert.Contains(t, status, "health_checks")
	assert.Contains(t, status, "unhealthy_periods")
	assert.Contains(t, status, "average_rtt")
	assert.Contains(t, status, "min_rtt")
	assert.Contains(t, status, "max_rtt")

	// Check types
	assert.IsType(t, true, status["is_healthy"])
	assert.IsType(t, float64(0), status["health_score"])
	assert.IsType(t, "", status["state"])
	assert.IsType(t, time.Time{}, status["last_ping_at"])
	assert.IsType(t, time.Time{}, status["last_pong_at"])
	assert.IsType(t, time.Duration(0), status["time_since_last_pong"])
	assert.IsType(t, int32(0), status["missed_pongs"])
	assert.IsType(t, time.Duration(0), status["ping_period"])
	assert.IsType(t, time.Duration(0), status["pong_wait"])
	assert.IsType(t, int64(0), status["total_pings_sent"])
	assert.IsType(t, int64(0), status["total_pongs_received"])
	assert.IsType(t, int64(0), status["total_missed_pongs"])
	assert.IsType(t, int64(0), status["health_checks"])
	assert.IsType(t, int64(0), status["unhealthy_periods"])
	assert.IsType(t, time.Duration(0), status["average_rtt"])
	assert.IsType(t, time.Duration(0), status["min_rtt"])
	assert.IsType(t, time.Duration(0), status["max_rtt"])
}

func TestHeartbeatConcurrency(t *testing.T) {
	WithResourceControl(t, "TestHeartbeatConcurrency", func() {
		config := DefaultConnectionConfig()
		config.URL = "ws://localhost:8080"
		config.Logger = zaptest.NewLogger(t)

		conn, err := NewConnection(config)
		require.NoError(t, err)

		heartbeat := conn.heartbeat

		// Test concurrent access to heartbeat methods with environment-aware scaling
		var wg sync.WaitGroup
		concurrencyConfig := getConcurrencyConfig("TestHeartbeatConcurrency")
		numGoroutines := concurrencyConfig.NumGoroutines
		iterations := concurrencyConfig.OperationsPerRoutine
		
		t.Logf("TestHeartbeatConcurrency: Using %d goroutines × %d iterations = %d total operations (full_suite=%v)", 
			numGoroutines, iterations, numGoroutines*iterations, isFullTestSuite())

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					// Concurrent reads
					_ = heartbeat.IsHealthy()
					_ = heartbeat.GetState()
					_ = heartbeat.GetStats()
					_ = heartbeat.GetConnectionHealth()
					_ = heartbeat.GetDetailedHealthStatus()

					// Concurrent pong simulation
					heartbeat.OnPong()

					// Concurrent resets
					heartbeat.Reset()
				}
			}()
		}

		wg.Wait()

		// Check that stats are consistent
		stats := heartbeat.GetStats()
		assert.GreaterOrEqual(t, stats.PongsReceived, int64(numGoroutines*iterations))
	}) // Close WithResourceControl
}

func TestHeartbeatReset(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("Error closing connection: %v", closeErr)
		}
	}()

	heartbeat := conn.heartbeat

	// Test reset functionality
	heartbeat.Reset()

	// Reset should not block and should be able to receive the signal
	select {
	case <-heartbeat.resetCh:
		// Reset signal received - this is expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Reset signal was not received within timeout")
	}

	// Test that multiple resets don't block (channel has buffer size 1)
	heartbeat.Reset()
	heartbeat.Reset() // This should not block due to default case in Reset()

	// Verify we can still receive at least one signal
	select {
	case <-heartbeat.resetCh:
		// Reset signal received
	case <-time.After(50 * time.Millisecond):
		// This is okay - the channel might be empty if the default case was used
	}
}

func TestHeartbeatMissedPongHandling(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.Logger = zaptest.NewLogger(t)
	config.PongWait = 100 * time.Millisecond

	conn, err := NewConnection(config)
	require.NoError(t, err)

	heartbeat := conn.heartbeat

	// Set last pong time to long ago
	atomic.StoreInt64(&heartbeat.lastPongAt, time.Now().Add(-200 * time.Millisecond).Unix())
	// Start the heartbeat to set it to running state using atomic operations
	atomic.StoreInt32(&heartbeat.state, int32(HeartbeatRunning))

	// Set last pong time to long ago using atomic for consistency with other operations
	atomic.StoreInt64(&heartbeat.lastPongAt, time.Now().Add(-200*time.Millisecond).UnixNano())

	// Check health - should detect missed pong
	heartbeat.checkHealth()

	assert.False(t, heartbeat.IsHealthy())
	assert.Greater(t, heartbeat.GetMissedPongCount(), int32(0))

	stats := heartbeat.GetStats()
	assert.Greater(t, stats.MissedPongs, int64(0))
	assert.Greater(t, stats.UnhealthyPeriods, int64(0))
}

func TestHeartbeatStopIdempotency(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	heartbeat := conn.heartbeat

	// Stop multiple times should be safe
	heartbeat.Stop()
	heartbeat.Stop()
	heartbeat.Stop()

	assert.Equal(t, HeartbeatStopped, heartbeat.GetState())
}

func TestHeartbeatStatsConsistency(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	heartbeat := conn.heartbeat

	// Generate some stats
	for i := 0; i < 10; i++ {
		heartbeat.OnPong()
		heartbeat.checkHealth()
	}

	// Get stats multiple times and ensure consistency
	stats1 := heartbeat.GetStats()
	stats2 := heartbeat.GetStats()

	assert.Equal(t, stats1.PongsReceived, stats2.PongsReceived)
	assert.Equal(t, stats1.HealthChecks, stats2.HealthChecks)
	assert.Equal(t, stats1.UnhealthyPeriods, stats2.UnhealthyPeriods)
}
