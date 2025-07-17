package websocket

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2"
	"go.uber.org/zap"

	"github.com/ag-ui/go-sdk/pkg/core"
)

// ConnectionPool manages a pool of WebSocket connections with load balancing
type ConnectionPool struct {
	// Configuration
	config *PoolConfig

	// Connection management
	connections map[string]*Connection
	connMutex   sync.RWMutex

	// Load balancing
	roundRobinIndex int64
	connectionKeys  []string
	keysMutex       sync.RWMutex

	// Health monitoring
	healthChecker *HealthChecker

	// Statistics
	stats *PoolStats

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Event handlers
	onConnectionStateChange func(connID string, state ConnectionState)
	onHealthChange          func(connID string, healthy bool)
	onMessage               func(data []byte)
	handlersMutex           sync.RWMutex
}

// PoolConfig contains configuration for the connection pool
type PoolConfig struct {
	// MinConnections is the minimum number of connections to maintain
	MinConnections int

	// MaxConnections is the maximum number of connections allowed
	MaxConnections int

	// ConnectionTimeout is the timeout for establishing connections
	ConnectionTimeout time.Duration

	// HealthCheckInterval is the interval for health checks
	HealthCheckInterval time.Duration

	// IdleTimeout is the timeout for idle connections
	IdleTimeout time.Duration

	// MaxIdleConnections is the maximum number of idle connections
	MaxIdleConnections int

	// LoadBalancingStrategy defines how connections are selected
	LoadBalancingStrategy LoadBalancingStrategy

	// ConnectionTemplate is the template configuration for new connections
	ConnectionTemplate *ConnectionConfig

	// URLs are the WebSocket URLs to connect to
	URLs []string

	// Logger is the logger instance
	Logger *zap.Logger
}

// LoadBalancingStrategy defines different load balancing strategies
type LoadBalancingStrategy int

const (
	// RoundRobin distributes requests evenly across connections
	RoundRobin LoadBalancingStrategy = iota
	// LeastConnections selects the connection with the fewest active requests
	LeastConnections
	// HealthBased selects the healthiest connection
	HealthBased
	// Random selects a random connection
	Random
)

// String returns the string representation of the load balancing strategy
func (s LoadBalancingStrategy) String() string {
	switch s {
	case RoundRobin:
		return "round_robin"
	case LeastConnections:
		return "least_connections"
	case HealthBased:
		return "health_based"
	case Random:
		return "random"
	default:
		return "unknown"
	}
}

// PoolStats tracks connection pool statistics
type PoolStats struct {
	TotalConnections     int64
	ActiveConnections    int64
	IdleConnections      int64
	HealthyConnections   int64
	UnhealthyConnections int64
	TotalRequests        int64
	FailedRequests       int64
	TotalBytesReceived   int64
	TotalBytesSent       int64
	AverageResponseTime  time.Duration
	mutex                sync.RWMutex
}

// DefaultPoolConfig returns a default configuration for the connection pool
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MinConnections:        2,
		MaxConnections:        10,
		ConnectionTimeout:     30 * time.Second,
		HealthCheckInterval:   10 * time.Second,
		IdleTimeout:           5 * time.Minute,
		MaxIdleConnections:    5,
		LoadBalancingStrategy: RoundRobin,
		ConnectionTemplate:    DefaultConnectionConfig(),
		Logger:                zap.NewNop(),
	}
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *PoolConfig) (*ConnectionPool, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}

	if len(config.URLs) == 0 {
		return nil, &core.ConfigError{
			Field: "URLs",
			Value: config.URLs,
			Err:   errors.New("at least one WebSocket URL must be provided"),
		}
	}

	if config.MinConnections < 1 {
		return nil, &core.ConfigError{
			Field: "MinConnections",
			Value: config.MinConnections,
			Err:   errors.New("minimum connections must be at least 1"),
		}
	}

	if config.MaxConnections < config.MinConnections {
		return nil, &core.ConfigError{
			Field: "MaxConnections",
			Value: config.MaxConnections,
			Err:   errors.New("maximum connections must be greater than or equal to minimum connections"),
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &ConnectionPool{
		config:      config,
		connections: make(map[string]*Connection),
		ctx:         ctx,
		cancel:      cancel,
		stats:       &PoolStats{},
	}

	// Initialize health checker
	pool.healthChecker = NewHealthChecker(pool, config.HealthCheckInterval)

	return pool, nil
}

// Start initializes the connection pool and establishes minimum connections
func (p *ConnectionPool) Start(ctx context.Context) error {
	p.config.Logger.Info("Starting connection pool",
		zap.Int("min_connections", p.config.MinConnections),
		zap.Int("max_connections", p.config.MaxConnections),
		zap.Int("urls", len(p.config.URLs)))

	// Start health checker
	p.wg.Add(1)
	go p.healthChecker.Start(ctx, &p.wg)

	// Establish minimum connections
	for i := 0; i < p.config.MinConnections; i++ {
		if err := p.createConnection(ctx); err != nil {
			p.config.Logger.Error("Failed to create minimum connection",
				zap.Int("index", i),
				zap.Error(err))
			// Continue trying to create other connections
		}
	}

	p.config.Logger.Info("Connection pool started",
		zap.Int("active_connections", p.GetActiveConnectionCount()))

	return nil
}

// Stop gracefully shuts down the connection pool
func (p *ConnectionPool) Stop() error {
	p.config.Logger.Info("Stopping connection pool")

	// Close all connections first to prevent goroutine leaks
	p.connMutex.Lock()
	for id, conn := range p.connections {
		p.config.Logger.Debug("Closing connection", zap.String("id", id))
		if err := conn.Close(); err != nil {
			p.config.Logger.Error("Error closing connection",
				zap.String("id", id),
				zap.Error(err))
		}
	}
	p.connections = make(map[string]*Connection)
	p.connMutex.Unlock()

	// Cancel context to stop all goroutines after connections are closed
	p.cancel()

	// Wait for all goroutines to finish
	p.wg.Wait()

	p.config.Logger.Info("Connection pool stopped")
	return nil
}

// GetConnection returns a connection based on the load balancing strategy
func (p *ConnectionPool) GetConnection(ctx context.Context) (*Connection, error) {
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	if len(p.connections) == 0 {
		return nil, errors.New("no connections available")
	}

	// Get healthy connections
	healthyConns := p.getHealthyConnections()
	if len(healthyConns) == 0 {
		return nil, errors.New("no healthy connections available")
	}

	// Select connection based on strategy
	var selectedConn *Connection
	switch p.config.LoadBalancingStrategy {
	case RoundRobin:
		selectedConn = p.selectRoundRobin(healthyConns)
	case LeastConnections:
		selectedConn = p.selectLeastConnections(healthyConns)
	case HealthBased:
		selectedConn = p.selectHealthBased(healthyConns)
	case Random:
		selectedConn = p.selectRandom(healthyConns)
	default:
		selectedConn = p.selectRoundRobin(healthyConns)
	}

	if selectedConn == nil {
		return nil, errors.New("failed to select connection")
	}

	return selectedConn, nil
}

// SendMessage sends a message through the pool using load balancing
func (p *ConnectionPool) SendMessage(ctx context.Context, message []byte) error {
	start := time.Now()

	// Update stats
	p.stats.mutex.Lock()
	p.stats.TotalRequests++
	p.stats.mutex.Unlock()

	conn, err := p.GetConnection(ctx)
	if err != nil {
		p.stats.mutex.Lock()
		p.stats.FailedRequests++
		p.stats.mutex.Unlock()
		return fmt.Errorf("failed to get connection: %w", err)
	}

	err = conn.SendMessage(ctx, message)
	if err != nil {
		p.stats.mutex.Lock()
		p.stats.FailedRequests++
		p.stats.mutex.Unlock()
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Update response time statistics
	responseTime := time.Since(start)
	p.stats.mutex.Lock()
	if p.stats.AverageResponseTime == 0 {
		p.stats.AverageResponseTime = responseTime
	} else {
		// Use exponential moving average
		p.stats.AverageResponseTime = time.Duration(
			float64(p.stats.AverageResponseTime)*0.9 + float64(responseTime)*0.1,
		)
	}
	p.stats.TotalBytesSent += int64(len(message))
	p.stats.mutex.Unlock()

	return nil
}

// GetActiveConnectionCount returns the number of active connections
func (p *ConnectionPool) GetActiveConnectionCount() int {
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	count := 0
	for _, conn := range p.connections {
		if conn.IsConnected() {
			count++
		}
	}

	return count
}

// GetHealthyConnectionCount returns the number of healthy connections
func (p *ConnectionPool) GetHealthyConnectionCount() int {
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	count := 0
	for _, conn := range p.connections {
		if conn.IsConnected() && conn.heartbeat.IsHealthy() {
			count++
		}
	}

	return count
}

// GetStats returns a copy of the pool statistics
func (p *ConnectionPool) Stats() PoolStats {
	p.stats.mutex.Lock()
	defer p.stats.mutex.Unlock()

	// Update real-time stats - need to read connections safely
	p.connMutex.RLock()
	totalConnections := int64(len(p.connections))
	p.connMutex.RUnlock()

	p.stats.TotalConnections = totalConnections
	p.stats.ActiveConnections = int64(p.GetActiveConnectionCount())
	p.stats.HealthyConnections = int64(p.GetHealthyConnectionCount())
	p.stats.UnhealthyConnections = p.stats.TotalConnections - p.stats.HealthyConnections

	return *p.stats
}

// SetOnConnectionStateChange sets the connection state change handler
func (p *ConnectionPool) SetOnConnectionStateChange(handler func(connID string, state ConnectionState)) {
	p.handlersMutex.Lock()
	p.onConnectionStateChange = handler
	p.handlersMutex.Unlock()
}

// SetOnHealthChange sets the health change handler
func (p *ConnectionPool) SetOnHealthChange(handler func(connID string, healthy bool)) {
	p.handlersMutex.Lock()
	p.onHealthChange = handler
	p.handlersMutex.Unlock()
}

// SetMessageHandler sets the message handler for all connections
func (p *ConnectionPool) SetMessageHandler(handler func(data []byte)) {
	p.handlersMutex.Lock()
	p.onMessage = handler
	p.handlersMutex.Unlock()

	// Update existing connections
	p.connMutex.RLock()
	for _, conn := range p.connections {
		conn.SetOnMessage(handler)
	}
	p.connMutex.RUnlock()
}

// createConnection creates a new connection and adds it to the pool
func (p *ConnectionPool) createConnection(ctx context.Context) error {
	// Select URL using round-robin
	urlIndex := int(atomic.AddInt64(&p.roundRobinIndex, 1)-1) % len(p.config.URLs)
	url := p.config.URLs[urlIndex]

	// Create connection configuration
	connConfig := *p.config.ConnectionTemplate
	connConfig.URL = url
	connConfig.Logger = p.config.Logger

	// Create connection
	conn, err := NewConnection(&connConfig)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	// Generate connection ID
	connID := fmt.Sprintf("conn_%d_%s", time.Now().Unix(), url)

	// Set up event handlers
	conn.SetOnConnect(func() {
		p.handlersMutex.RLock()
		handler := p.onConnectionStateChange
		p.handlersMutex.RUnlock()

		if handler != nil {
			handler(connID, StateConnected)
		}
	})

	conn.SetOnDisconnect(func(err error) {
		p.handlersMutex.RLock()
		handler := p.onConnectionStateChange
		p.handlersMutex.RUnlock()

		if handler != nil {
			handler(connID, StateDisconnected)
		}
	})

	// Set message handler if available
	p.handlersMutex.RLock()
	messageHandler := p.onMessage
	p.handlersMutex.RUnlock()

	if messageHandler != nil {
		conn.SetOnMessage(messageHandler)
	}

	// Connect with timeout
	connectCtx, cancel := context.WithTimeout(ctx, p.config.ConnectionTimeout)
	defer cancel()

	if err := conn.Connect(connectCtx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Start auto-reconnect
	conn.StartAutoReconnect(p.ctx)

	// Add to pool
	p.connMutex.Lock()
	p.connections[connID] = conn
	p.updateConnectionKeys()
	p.connMutex.Unlock()

	p.config.Logger.Info("Created connection",
		zap.String("id", connID),
		zap.String("url", url))

	return nil
}

// removeConnection removes a connection from the pool
func (p *ConnectionPool) removeConnection(connID string) {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if conn, exists := p.connections[connID]; exists {
		conn.Close()
		delete(p.connections, connID)
		p.updateConnectionKeys()

		p.config.Logger.Info("Removed connection",
			zap.String("id", connID))
	}
}

// updateConnectionKeys updates the connection keys for load balancing
func (p *ConnectionPool) updateConnectionKeys() {
	p.keysMutex.Lock()
	defer p.keysMutex.Unlock()

	p.connectionKeys = make([]string, 0, len(p.connections))
	for id := range p.connections {
		p.connectionKeys = append(p.connectionKeys, id)
	}
}

// getHealthyConnections returns a list of healthy connections
func (p *ConnectionPool) getHealthyConnections() []*Connection {
	var healthy []*Connection
	for _, conn := range p.connections {
		if conn.IsConnected() && conn.heartbeat.IsHealthy() {
			healthy = append(healthy, conn)
		}
	}
	return healthy
}

// selectRoundRobin selects a connection using round-robin
func (p *ConnectionPool) selectRoundRobin(connections []*Connection) *Connection {
	if len(connections) == 0 {
		return nil
	}

	index := int(atomic.AddInt64(&p.roundRobinIndex, 1)-1) % len(connections)
	return connections[index]
}

// selectLeastConnections selects the connection with the fewest active requests
func (p *ConnectionPool) selectLeastConnections(connections []*Connection) *Connection {
	if len(connections) == 0 {
		return nil
	}

	var bestConn *Connection
	var minLoad int64 = int64(^uint64(0) >> 1) // Max int64

	for _, conn := range connections {
		// Calculate connection load based on multiple factors
		metrics := conn.GetMetrics()
		
		// Primary metric: pending messages in channel (approximate active requests)
		pendingMessages := int64(len(conn.messageCh))
		
		// Secondary metric: message rate difference (sent vs received)
		// Higher difference indicates more outbound load
		messageRate := metrics.MessagesSent - metrics.MessagesReceived
		if messageRate < 0 {
			messageRate = 0 // Only count positive outbound load
		}
		
		// Tertiary metric: error rate (higher errors = higher load/instability)
		errorRate := metrics.Errors
		
		// Calculate composite load score
		load := pendingMessages*100 + messageRate*10 + errorRate*5
		
		// Select connection with lowest load
		if load < minLoad {
			minLoad = load
			bestConn = conn
		}
	}

	// Fallback to first connection if no best connection found
	if bestConn == nil {
		return connections[0]
	}

	return bestConn
}

// selectHealthBased selects the healthiest connection
func (p *ConnectionPool) selectHealthBased(connections []*Connection) *Connection {
	if len(connections) == 0 {
		return nil
	}

	var bestConn *Connection
	var bestHealth float64

	for _, conn := range connections {
		health := conn.heartbeat.GetConnectionHealth()
		if health > bestHealth {
			bestHealth = health
			bestConn = conn
		}
	}

	return bestConn
}

// selectRandom selects a random connection
func (p *ConnectionPool) selectRandom(connections []*Connection) *Connection {
	if len(connections) == 0 {
		return nil
	}

	// Use current time as seed for simple randomness
	index := int(time.Now().UnixNano()) % len(connections)
	return connections[index]
}

// HealthChecker monitors connection health and manages pool size
type HealthChecker struct {
	pool     *ConnectionPool
	interval time.Duration
	cache    *lru.Cache[string, bool]
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(pool *ConnectionPool, interval time.Duration) *HealthChecker {
	cache, _ := lru.New[string, bool](100)
	return &HealthChecker{
		pool:     pool,
		interval: interval,
		cache:    cache,
	}
}

// Start begins health checking
func (h *HealthChecker) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.checkHealth()
		}
	}
}

// checkHealth performs health checks on all connections
func (h *HealthChecker) checkHealth() {
	h.pool.connMutex.RLock()
	connections := make(map[string]*Connection)
	for id, conn := range h.pool.connections {
		connections[id] = conn
	}
	h.pool.connMutex.RUnlock()

	var unhealthyConnections []string

	for id, conn := range connections {
		healthy := conn.IsConnected() && conn.heartbeat.IsHealthy()

		// Check if health status changed
		if previousHealth, exists := h.cache.Get(id); exists {
			if previousHealth != healthy {
				h.pool.handlersMutex.RLock()
				handler := h.pool.onHealthChange
				h.pool.handlersMutex.RUnlock()

				if handler != nil {
					handler(id, healthy)
				}
			}
		}

		h.cache.Add(id, healthy)

		if !healthy {
			unhealthyConnections = append(unhealthyConnections, id)
		}
	}

	// Remove persistently unhealthy connections
	for _, id := range unhealthyConnections {
		if conn, exists := connections[id]; exists {
			// Remove if connection has been unhealthy for too long
			if time.Since(conn.GetLastConnected()) > h.pool.config.IdleTimeout {
				h.pool.removeConnection(id)
			}
		}
	}

	// Scale up if needed
	healthyCount := h.pool.GetHealthyConnectionCount()
	if healthyCount < h.pool.config.MinConnections {
		needed := h.pool.config.MinConnections - healthyCount
		for i := 0; i < needed; i++ {
			// Check connection count with proper locking
			h.pool.connMutex.RLock()
			currentConnCount := len(h.pool.connections)
			h.pool.connMutex.RUnlock()

			if currentConnCount >= h.pool.config.MaxConnections {
				break
			}
			if err := h.pool.createConnection(context.Background()); err != nil {
				h.pool.config.Logger.Error("Failed to create replacement connection",
					zap.Error(err))
			}
		}
	}
}

// GetDetailedStatus returns detailed status of all connections
func (p *ConnectionPool) GetDetailedStatus() map[string]interface{} {
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	connections := make(map[string]interface{})
	for id, conn := range p.connections {
		connections[id] = map[string]interface{}{
			"url":                conn.GetURL(),
			"state":              conn.State().String(),
			"is_connected":       conn.IsConnected(),
			"last_connected":     conn.GetLastConnected(),
			"reconnect_attempts": conn.GetReconnectAttempts(),
			"last_error":         conn.LastError(),
			"metrics":            conn.GetMetrics(),
			"heartbeat":          conn.heartbeat.GetDetailedHealthStatus(),
		}
	}

	return map[string]interface{}{
		"total_connections":   len(p.connections),
		"active_connections":  p.GetActiveConnectionCount(),
		"healthy_connections": p.GetHealthyConnectionCount(),
		"load_balancing":      p.config.LoadBalancingStrategy.String(),
		"stats":               p.Stats(),
		"connections":         connections,
	}
}
